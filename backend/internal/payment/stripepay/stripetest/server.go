// Package stripetest is a fake Stripe for the adapter's unit tests and the
// end-to-end suite (revisions 6.0, §26.5): an httptest server that implements
// exactly the endpoints package stripepay calls, with deterministic outcomes
// chosen by the test token or payment method it is given. It records every
// request (method, path, headers including Idempotency-Key, form parameters)
// and ships a webhook signer and builders for every event type the platform
// handles (events.go).
//
// # Test tokens
//
// A ConfirmationToken id is "ctoken_" + a spec; a payment method is minted
// with [Server.AddPaymentMethod] from the same kind of spec, or by confirming a
// SetupIntent with [Server.ConfirmSetupIntent]. The spec decides what happens
// when the token or payment method is charged:
//
//	ok[_brand_last4[_CC]]        succeeds        ctoken_ok_visa_4242, ctoken_ok_mastercard_4444_FR
//	3ds[_brand_last4[_CC]]       requires_action (use_stripe_sdk); finish it with
//	                             [Server.CompleteAction]: ctoken_3ds_visa_3155
//	decline_<code>               402 card_error with that decline code:
//	                             ctoken_decline_insufficient_funds, ctoken_decline_card_declined,
//	                             ctoken_decline_lost_card, ctoken_decline_expired_card …
//	timeout[_brand_last4[_CC]]   the payment is created, then the connection is
//	                             dropped before any response (a lost response); a
//	                             retry with the same Idempotency-Key gets the answer
//	lost[_brand_last4[_CC]]      like timeout, but the answer is lost on every retry
//	                             too: only Lookup finds the payment
//	error500                     500 api_error, nothing created
//	ratelimit                    429, nothing created
//	auth                         401 invalid API key
//	expired                      the ConfirmationToken has expired (400 on use)
//
// The brand and last four digits give the card its fingerprint: the same
// brand and last four always mean the same card, whatever the token.
// ctoken_… tokens are single use: a second use is a 400.
//
// # Behaviour
//
// POSTs honour Idempotency-Key like Stripe: the same key and parameters replay
// the first response (header Idempotent-Replayed: true), the same key with other
// parameters is a 400 idempotency_error. PaymentIntent search can be made
// eventually consistent with [Server.SetSearchLag]. Errors and dropped
// connections can be injected for any endpoint with [Server.Fail] and
// [Server.Drop].
package stripetest

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Request is one request the fake received.
type Request struct {
	Method string
	Path   string
	Header http.Header
	// Form holds the form-encoded body (POST) or the query (GET) as Stripe's
	// bracket syntax: "metadata[sy_key]", "expand[0]"…
	Form url.Values
	// Upload describes a multipart file upload (POST /v1/files).
	Upload *Upload
}

// IdempotencyKey is the request's Idempotency-Key header.
func (r Request) IdempotencyKey() string { return r.Header.Get("Idempotency-Key") }

// Upload is a recorded file upload.
type Upload struct {
	Purpose, Filename string
	Size              int
	Content           []byte
}

// Server is the fake Stripe.
type Server struct {
	*httptest.Server

	mu        sync.Mutex
	seq       int
	tag       string   // random per server: ids are unique across fakes (a shared database sees them all)
	customerO []string // creation order
	refundO   []string
	customers map[string]*customer
	methods   map[string]*method
	intents   map[string]*intent
	order     []string // payment intent ids, creation order
	setups    map[string]*setupIntent
	refunds   map[string]*refund
	disputes  map[string]*dispute
	files     []Upload
	events    map[string][]byte
	used      map[string]string // confirmation token → payment intent
	idem      map[string]idemEntry
	requests  []Request
	failures  []*failure
	searchLag bool
	sticky    bool // set by a handler: its dropped response is dropped on replays as well
	account   Account
	endpoints []Endpoint
	livemode  bool
}

type idemEntry struct {
	hash   string
	status int
	body   []byte
	drop   bool // the response is lost on every replay too
}

type failure struct {
	method, prefix string
	status         int
	body           string
	drop           bool
	remaining      int
}

// New starts a fake Stripe. Close it with Close (or t.Cleanup).
func New() *Server {
	s := &Server{tag: randomTag(), customers: map[string]*customer{}, methods: map[string]*method{}, intents: map[string]*intent{},
		setups: map[string]*setupIntent{}, refunds: map[string]*refund{}, disputes: map[string]*dispute{},
		events: map[string][]byte{}, used: map[string]string{}, idem: map[string]idemEntry{},
		account: DefaultAccount()}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

// Livemode makes the fake report livemode=true in its objects (default: test mode).
func (s *Server) Livemode(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.livemode = on
}

// Requests returns every request received so far, oldest first.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

// RequestsTo returns the requests with this method and path (path.Match-free:
// an exact path, or a prefix when it ends in "*").
func (s *Server) RequestsTo(method, path string) []Request {
	var out []Request
	for _, r := range s.Requests() {
		if r.Method == method && pathMatches(path, r.Path) {
			out = append(out, r)
		}
	}
	return out
}

func pathMatches(pattern, path string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(path, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == path
}

// ResetRequests forgets the recorded requests (state is kept).
func (s *Server) ResetRequests() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = nil
}

// SetSearchLag makes PaymentIntent search find nothing, like Stripe's search
// index before it caught up (it is eventually consistent, up to a minute).
func (s *Server) SetSearchLag(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.searchLag = on
}

// Fail answers the next times requests whose method matches and whose path
// starts with prefix with this status and JSON body ("" is a generic
// api_error body).
func (s *Server) Fail(method, prefix string, status, times int, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures = append(s.failures, &failure{method: method, prefix: prefix, status: status, body: body, remaining: times})
}

// Drop closes the connection without answering the next times matching
// requests (before processing them: nothing is created).
func (s *Server) Drop(method, prefix string, times int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures = append(s.failures, &failure{method: method, prefix: prefix, drop: true, remaining: times})
}

// Uploads returns the files uploaded (dispute evidence).
func (s *Server) Uploads() []Upload {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Upload(nil), s.files...)
}

// SetAccount replaces the account GET /v1/account describes.
func (s *Server) SetAccount(a Account) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.account = a
}

// SetWebhookEndpoints replaces the endpoints GET /v1/webhook_endpoints lists.
func (s *Server) SetWebhookEndpoints(e []Endpoint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.endpoints = append([]Endpoint(nil), e...)
}

// AddEvent registers an event (built with the builders in events.go) so that
// GET /v1/events/{id} returns it.
func (s *Server) AddEvent(payload []byte) {
	var head struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(payload, &head)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[head.ID] = append([]byte(nil), payload...)
}

// next returns the next id of kind prefix ("pi_Xk3f9a1", "cus_Xk3f9a2"…): unique
// across servers, like Stripe's, and as long as a real id.
func (s *Server) next(prefix string) string {
	s.seq++
	return prefix + "_" + s.tag + strconv.Itoa(s.seq)
}

func randomTag() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b) + "Q"
}

// ------------------------------------------------------------ routing

var (
	rePI          = regexp.MustCompile(`^/v1/payment_intents/(pi_[A-Za-z0-9_]+)$`)
	reCustomer    = regexp.MustCompile(`^/v1/customers/([A-Za-z0-9_]+)$`)
	rePM          = regexp.MustCompile(`^/v1/payment_methods/([A-Za-z0-9_]+)$`)
	rePMAction    = regexp.MustCompile(`^/v1/payment_methods/([A-Za-z0-9_]+)/(attach|detach)$`)
	reToken       = regexp.MustCompile(`^/v1/confirmation_tokens/([A-Za-z0-9_\-.:]+)$`)
	reDispute     = regexp.MustCompile(`^/v1/disputes/([A-Za-z0-9_]+)$`)
	reDisputeEnd  = regexp.MustCompile(`^/v1/disputes/([A-Za-z0-9_]+)/close$`)
	reEvent       = regexp.MustCompile(`^/v1/events/([A-Za-z0-9_]+)$`)
	reSearchQuery = regexp.MustCompile(`metadata\['sy_key'\]:'((?:[^'\\]|\\.)*)'`)
)

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	req := Request{Method: r.Method, Path: r.URL.Path, Header: r.Header.Clone()}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err == nil {
			req.Form = r.MultipartForm.Value
			if files := r.MultipartForm.File["file"]; len(files) > 0 {
				f, _ := files[0].Open()
				buf := make([]byte, files[0].Size)
				if f != nil {
					_, _ = f.Read(buf)
					_ = f.Close()
				}
				req.Upload = &Upload{Purpose: r.FormValue("purpose"), Filename: files[0].Filename, Size: int(files[0].Size), Content: buf}
			}
		}
	} else {
		_ = r.ParseForm()
		req.Form = r.Form
	}
	if req.Form == nil {
		req.Form = url.Values{}
	}
	s.mu.Lock()
	s.requests = append(s.requests, req)
	fail := s.matchFailure(r.Method, r.URL.Path)
	s.mu.Unlock()
	w.Header().Set("Request-Id", "req_fake")
	w.Header().Set("Content-Type", "application/json")

	if fail != nil {
		if fail.drop {
			dropConnection(w)
			return
		}
		body := fail.body
		if body == "" {
			body = apiErrorBody("api_error", "", "", "An error occurred with our API.")
		}
		w.WriteHeader(fail.status)
		_, _ = w.Write([]byte(body))
		return
	}
	if strings.HasPrefix(req.Header.Get("Authorization"), "Bearer sk_bad") {
		writeError(w, http.StatusUnauthorized, "invalid_request_error", "", "", "Invalid API Key provided: sk_bad_***")
		return
	}

	status, body, drop := s.route(req)
	if drop {
		dropConnection(w)
		return
	}
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// matchFailure pops an injected failure (mu held).
func (s *Server) matchFailure(method, path string) *failure {
	for _, f := range s.failures {
		if f.remaining > 0 && f.method == method && strings.HasPrefix(path, f.prefix) {
			f.remaining--
			return f
		}
	}
	return nil
}

func dropConnection(w http.ResponseWriter) {
	if h, ok := w.(http.Hijacker); ok {
		if conn, _, err := h.Hijack(); err == nil {
			_ = conn.Close()
			return
		}
	}
	w.WriteHeader(http.StatusBadGateway)
}

func (s *Server) route(req Request) (status int, body []byte, drop bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, p := req.Method, req.Path
	// Idempotency: replay or conflict, before any state changes.
	if m == http.MethodPost {
		if key := req.IdempotencyKey(); key != "" {
			entry, ok := s.idem[m+" "+p+" "+key]
			if ok {
				if entry.hash != hashForm(req.Form) {
					return errorResponse(http.StatusBadRequest, "idempotency_error", "", "",
						"Keys for idempotent requests can only be used with the same parameters they were first used with.")
				}
				return entry.status, entry.body, entry.drop
			}
		}
	}
	s.sticky = false
	status, body, drop = s.dispatch(req)
	if m == http.MethodPost && status < 500 && status != http.StatusTooManyRequests {
		if key := req.IdempotencyKey(); key != "" {
			s.idem[m+" "+p+" "+key] = idemEntry{hash: hashForm(req.Form), status: status, body: body, drop: drop && s.sticky}
		}
	}
	return status, body, drop
}

func hashForm(f url.Values) string {
	keys := make([]string, 0, len(f))
	for k := range f {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		vals := append([]string(nil), f[k]...)
		sort.Strings(vals)
		fmt.Fprintf(h, "%s=%s&", k, strings.Join(vals, ","))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Server) dispatch(req Request) (int, []byte, bool) {
	m, p := req.Method, req.Path
	switch {
	case m == http.MethodPost && p == "/v1/payment_intents":
		return s.createIntent(req)
	case m == http.MethodGet && p == "/v1/payment_intents/search":
		return s.searchIntents(req)
	case m == http.MethodGet && rePI.MatchString(p):
		return s.getIntent(req, rePI.FindStringSubmatch(p)[1])
	case m == http.MethodPost && p == "/v1/customers":
		return s.createCustomer(req)
	case reCustomer.MatchString(p):
		return s.customerEndpoint(req, reCustomer.FindStringSubmatch(p)[1])
	case m == http.MethodGet && rePM.MatchString(p):
		return s.getMethod(rePM.FindStringSubmatch(p)[1])
	case m == http.MethodPost && rePMAction.MatchString(p):
		g := rePMAction.FindStringSubmatch(p)
		return s.methodAction(req, g[1], g[2])
	case m == http.MethodGet && reToken.MatchString(p):
		return s.getToken(reToken.FindStringSubmatch(p)[1])
	case m == http.MethodPost && p == "/v1/setup_intents":
		return s.createSetupIntent(req)
	case m == http.MethodPost && p == "/v1/refunds":
		return s.createRefund(req)
	case m == http.MethodPost && reDisputeEnd.MatchString(p):
		return s.closeDispute(reDisputeEnd.FindStringSubmatch(p)[1])
	case m == http.MethodPost && reDispute.MatchString(p):
		return s.updateDispute(req, reDispute.FindStringSubmatch(p)[1])
	case m == http.MethodPost && p == "/v1/files":
		return s.createFile(req)
	case m == http.MethodGet && p == "/v1/account":
		return ok(s.account.object())
	case m == http.MethodGet && p == "/v1/webhook_endpoints":
		return s.listEndpoints()
	case m == http.MethodGet && reEvent.MatchString(p):
		id := reEvent.FindStringSubmatch(p)[1]
		if b, found := s.events[id]; found {
			return http.StatusOK, b, false
		}
		return notFound("event", id)
	}
	return errorResponse(http.StatusNotFound, "invalid_request_error", "", "", "Unrecognized request URL ("+m+": "+p+").")
}

// --------------------------------------------------------- responses

func ok(v any) (int, []byte, bool) {
	b, _ := json.Marshal(v)
	return http.StatusOK, b, false
}

func apiErrorBody(typ, code, param, message string) string {
	e := map[string]any{"type": typ, "message": message}
	if code != "" {
		e["code"] = code
	}
	if param != "" {
		e["param"] = param
	}
	b, _ := json.Marshal(map[string]any{"error": e})
	return string(b)
}

func errorResponse(status int, typ, code, param, message string) (int, []byte, bool) {
	return status, []byte(apiErrorBody(typ, code, param, message)), false
}

func writeError(w http.ResponseWriter, status int, typ, code, param, message string) {
	w.WriteHeader(status)
	_, _ = w.Write([]byte(apiErrorBody(typ, code, param, message)))
}

func notFound(kind, id string) (int, []byte, bool) {
	return errorResponse(http.StatusNotFound, "invalid_request_error", "resource_missing", "id", "No such "+kind+": '"+id+"'")
}

func wantsExpand(f url.Values, field string) bool {
	for k, vs := range f {
		if k == "expand" || strings.HasPrefix(k, "expand[") {
			for _, v := range vs {
				if v == field || v == "data."+field {
					return true
				}
			}
		}
	}
	return false
}

func now() int64 { return time.Now().Unix() }
