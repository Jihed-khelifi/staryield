package stripetest

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
)

// spec is a parsed test token or payment-method spec.
type spec struct {
	behaviour string
	brand     string
	last4     string
	country   string
	decline   string
}

var behaviours = map[string]bool{"ok": true, "3ds": true, "decline": true, "timeout": true, "lost": true, "error500": true,
	"ratelimit": true, "auth": true, "expired": true}

// parseSpec reads "ok_visa_4242_FR", "decline_insufficient_funds"… (the part
// after "ctoken_"); false when the behaviour is unknown.
func parseSpec(s string) (spec, bool) {
	parts := strings.Split(s, "_")
	if len(parts) == 0 || !behaviours[parts[0]] {
		return spec{}, false
	}
	out := spec{behaviour: parts[0], brand: "visa", last4: "4242"}
	switch out.behaviour {
	case "3ds":
		out.last4 = "3155"
	case "decline":
		out.last4 = "0002"
		out.decline = strings.Join(parts[1:], "_")
		if out.decline == "" {
			out.decline = "card_declined"
		}
		return out, true
	}
	if len(parts) >= 3 {
		out.brand, out.last4 = parts[1], parts[2]
	}
	if len(parts) >= 4 {
		out.country = strings.ToUpper(parts[3])
	}
	if out.country == "" {
		out.country = "US"
	}
	return out, true
}

func (sp spec) fingerprint() string {
	sum := sha256.Sum256([]byte("fp|" + sp.brand + "|" + sp.last4))
	return "fp_" + hex.EncodeToString(sum[:8])
}

func (sp spec) card() map[string]any {
	country := sp.country
	if country == "" {
		country = "US"
	}
	return map[string]any{"brand": sp.brand, "last4": sp.last4, "exp_month": 12, "exp_year": 2035, "country": country,
		"fingerprint": sp.fingerprint(), "funding": "credit"}
}

// ------------------------------------------------------- stored state

type customer struct {
	id, email, name string
	metadata        map[string]string
	deleted         bool
}

type method struct {
	id, customer string
	spec         spec
}

type charge struct {
	id          string
	amount      int64
	refunded    int64
	disputed    bool
	threeDS     string // result: not_supported | authenticated | failed
	riskLevel   string
	riskScore   int
	spec        spec
	paymentInt  string
	metadata    map[string]string
	currency    string
	customerRef string
}

type intent struct {
	id, secret   string
	amount       int64
	currency     string
	status       string
	customer     string
	paymentMeth  string
	metadata     map[string]string
	description  string
	chargeRef    *charge
	lastErrCode  string
	lastDecline  string
	needsAction  bool
	returnURL    string
	spec         spec
	confirmToken string
}

type setupIntent struct {
	id, secret, customer string
	usage                string
	confirmed            bool
}

// ------------------------------------------------------------ JSON views

func (s *Server) chargeJSON(c *charge) map[string]any {
	threeDS := map[string]any{"result": c.threeDS}
	if c.threeDS == "" {
		threeDS = nil
	}
	card := c.spec.card()
	card["three_d_secure"] = threeDS
	out := map[string]any{"id": c.id, "object": "charge", "amount": c.amount, "currency": c.currency, "status": "succeeded",
		"paid": true, "captured": true, "disputed": c.disputed, "amount_refunded": c.refunded, "refunded": c.refunded >= c.amount,
		"payment_intent": c.paymentInt, "livemode": s.livemode, "metadata": c.metadata,
		"payment_method_details": map[string]any{"type": "card", "card": card}}
	if c.riskLevel != "" {
		o := map[string]any{"risk_level": c.riskLevel, "type": "authorized", "network_status": "approved_by_network"}
		if c.riskScore > 0 {
			o["risk_score"] = c.riskScore
		}
		out["outcome"] = o
	}
	return out
}

func (s *Server) intentJSON(pi *intent, expandCharge bool) map[string]any {
	out := map[string]any{"id": pi.id, "object": "payment_intent", "amount": pi.amount, "currency": pi.currency, "status": pi.status,
		"client_secret": pi.secret, "livemode": s.livemode, "metadata": pi.metadata, "description": pi.description,
		"payment_method_types": []string{"card"}, "customer": nilIfEmpty(pi.customer), "payment_method": nilIfEmpty(pi.paymentMeth),
		"created": now(), "last_payment_error": nil, "next_action": nil, "latest_charge": nil}
	if pi.chargeRef != nil {
		if expandCharge {
			out["latest_charge"] = s.chargeJSON(pi.chargeRef)
		} else {
			out["latest_charge"] = pi.chargeRef.id
		}
	}
	if pi.status == "requires_action" {
		out["next_action"] = map[string]any{"type": "use_stripe_sdk", "use_stripe_sdk": map[string]any{"type": "three_d_secure_redirect",
			"stripe_js": "https://hooks.stripe.com/fake/3ds/" + pi.id}}
	}
	if pi.lastErrCode != "" {
		e := map[string]any{"type": "card_error", "code": pi.lastErrCode, "message": "Your card was declined."}
		if pi.lastDecline != "" {
			e["decline_code"] = pi.lastDecline
		}
		out["last_payment_error"] = e
	}
	return out
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ------------------------------------------------------ payment intents

// declineCodes whose Stripe error code is the decline code itself.
var ownCode = map[string]bool{"expired_card": true, "incorrect_cvc": true, "processing_error": true, "incorrect_number": true}

func (s *Server) createIntent(req Request) (int, []byte, bool) {
	f := req.Form
	amount, _ := strconv.ParseInt(f.Get("amount"), 10, 64)
	currency := f.Get("currency")
	if amount <= 0 || currency == "" {
		return errorResponse(http.StatusBadRequest, "invalid_request_error", "parameter_missing", "amount", "Missing required param: amount.")
	}
	var sp spec
	var tokenID, pmID string
	customerRef := f.Get("customer")
	switch {
	case f.Get("confirmation_token") != "":
		tokenID = f.Get("confirmation_token")
		parsed, ok := s.parseToken(tokenID)
		if !ok {
			return errorResponse(http.StatusNotFound, "invalid_request_error", "resource_missing", "confirmation_token",
				"No such confirmation token: '"+tokenID+"'")
		}
		if parsed.behaviour == "expired" {
			return errorResponse(http.StatusBadRequest, "invalid_request_error", "", "confirmation_token", "The ConfirmationToken has expired.")
		}
		if _, used := s.used[tokenID]; used {
			return errorResponse(http.StatusBadRequest, "invalid_request_error", "", "confirmation_token", "The ConfirmationToken has already been used.")
		}
		sp = parsed
	case f.Get("payment_method") != "":
		pmID = f.Get("payment_method")
		m, ok := s.methods[pmID]
		if !ok {
			return notFound("payment method", pmID)
		}
		if m.customer != customerRef {
			return errorResponse(http.StatusBadRequest, "invalid_request_error", "", "payment_method",
				"The customer does not have a payment method with the ID "+pmID+". The payment method must be attached to the customer.")
		}
		sp = m.spec
	default:
		return errorResponse(http.StatusBadRequest, "invalid_request_error", "parameter_missing", "payment_method", "Missing payment method.")
	}
	if customerRef != "" {
		if c, ok := s.customers[customerRef]; !ok || c.deleted {
			return notFound("customer", customerRef)
		}
	}
	switch sp.behaviour {
	case "error500":
		return errorResponse(http.StatusInternalServerError, "api_error", "", "", "An error occurred with our API.")
	case "ratelimit":
		return errorResponse(http.StatusTooManyRequests, "invalid_request_error", "rate_limit", "", "Too many requests.")
	case "auth":
		return errorResponse(http.StatusUnauthorized, "invalid_request_error", "", "", "Invalid API Key provided: sk_test_***")
	}
	meta := map[string]string{}
	for k, v := range f {
		if strings.HasPrefix(k, "metadata[") && len(v) > 0 {
			meta[strings.TrimSuffix(strings.TrimPrefix(k, "metadata["), "]")] = v[0]
		}
	}
	pi := &intent{id: s.next("pi"), amount: amount, currency: currency, customer: customerRef, paymentMeth: pmID, metadata: meta,
		description: f.Get("description"), returnURL: f.Get("return_url"), spec: sp, confirmToken: tokenID}
	pi.secret = pi.id + "_secret_" + s.tag + strconv.Itoa(s.seq)
	s.intents[pi.id] = pi
	s.order = append(s.order, pi.id)
	if tokenID != "" {
		s.used[tokenID] = pi.id
	}
	expand := wantsExpand(f, "latest_charge")
	switch sp.behaviour {
	case "decline":
		code := "card_declined"
		if ownCode[sp.decline] {
			code = sp.decline
		}
		pi.status, pi.lastErrCode, pi.lastDecline = "requires_payment_method", code, sp.decline
		e := map[string]any{"type": "card_error", "code": code, "decline_code": sp.decline, "message": "Your card was declined.",
			"payment_intent": s.intentJSON(pi, false)}
		return http.StatusPaymentRequired, mustJSON(map[string]any{"error": e}), false
	case "3ds":
		pi.status = "requires_action"
		pi.chargeRef = nil
		return ok(s.intentJSON(pi, expand))
	}
	s.settle(pi, "not_supported")
	status, body, _ := ok(s.intentJSON(pi, expand))
	s.sticky = sp.behaviour == "lost"
	return status, body, sp.behaviour == "timeout" || sp.behaviour == "lost"
}

// settle makes the intent succeed with a charge.
func (s *Server) settle(pi *intent, threeDS string) {
	pi.status = "succeeded"
	pi.lastErrCode, pi.lastDecline = "", ""
	pi.chargeRef = &charge{id: s.next("ch"), amount: pi.amount, currency: pi.currency, spec: pi.spec, paymentInt: pi.id,
		metadata: pi.metadata, threeDS: threeDS, riskLevel: "normal", customerRef: pi.customer}
}

func (s *Server) parseToken(id string) (spec, bool) {
	if !strings.HasPrefix(id, "ctoken_") {
		return spec{}, false
	}
	return parseSpec(strings.TrimPrefix(id, "ctoken_"))
}

func (s *Server) getIntent(req Request, id string) (int, []byte, bool) {
	pi, found := s.intents[id]
	if !found {
		return notFound("payment_intent", id)
	}
	return ok(s.intentJSON(pi, wantsExpand(req.Form, "latest_charge")))
}

func (s *Server) searchIntents(req Request) (int, []byte, bool) {
	list := []any{}
	if !s.searchLag {
		if m := reSearchQuery.FindStringSubmatch(req.Form.Get("query")); m != nil {
			key := strings.NewReplacer(`\\`, `\`, `\'`, `'`).Replace(m[1])
			for _, id := range s.order {
				if pi := s.intents[id]; pi.metadata["sy_key"] == key {
					list = append(list, s.intentJSON(pi, wantsExpand(req.Form, "latest_charge")))
				}
			}
		}
	}
	return ok(map[string]any{"object": "search_result", "url": "/v1/payment_intents/search", "has_more": false, "next_page": nil, "data": list})
}

// ------------------------------------------------------------ test hooks

// CompleteAction finishes a requires_action PaymentIntent the way the browser
// would after the 3-D Secure step: success makes it succeeded (3-D Secure
// "authenticated"), failure puts it back to requires_payment_method with the
// authentication-failure error.
func (s *Server) CompleteAction(paymentIntentID string, success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pi := s.intents[paymentIntentID]
	if pi == nil || pi.status != "requires_action" {
		return
	}
	if success {
		s.settle(pi, "authenticated")
		return
	}
	pi.status, pi.lastErrCode, pi.lastDecline = "requires_payment_method", "payment_intent_authentication_failure", ""
}

// SetRisk sets the Radar outcome of a succeeded payment's charge.
func (s *Server) SetRisk(paymentIntentID, level string, score int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if pi := s.intents[paymentIntentID]; pi != nil && pi.chargeRef != nil {
		pi.chargeRef.riskLevel, pi.chargeRef.riskScore = level, score
	}
}

// Intent describes a PaymentIntent the fake holds.
type Intent struct {
	ID, Status, Currency, Customer, PaymentMethod, ClientSecret string
	Amount                                                      int64
	Metadata                                                    map[string]string
	ChargeID                                                    string
	Refunded                                                    int64
	Disputed                                                    bool
}

func (s *Server) snapshot(pi *intent) Intent {
	out := Intent{ID: pi.id, Status: pi.status, Currency: pi.currency, Customer: pi.customer, PaymentMethod: pi.paymentMeth,
		ClientSecret: pi.secret, Amount: pi.amount, Metadata: map[string]string{}}
	for k, v := range pi.metadata {
		out.Metadata[k] = v
	}
	if pi.chargeRef != nil {
		out.ChargeID, out.Refunded, out.Disputed = pi.chargeRef.id, pi.chargeRef.refunded, pi.chargeRef.disputed
	}
	return out
}

// Intents returns the PaymentIntents created, oldest first.
func (s *Server) Intents() []Intent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Intent, 0, len(s.order))
	for _, id := range s.order {
		out = append(out, s.snapshot(s.intents[id]))
	}
	return out
}

// IntentByKey returns the PaymentIntent created with metadata sy_key = key.
func (s *Server) IntentByKey(key string) (Intent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range s.order {
		if pi := s.intents[id]; pi.metadata["sy_key"] == key {
			return s.snapshot(pi), true
		}
	}
	return Intent{}, false
}

// Intent returns a PaymentIntent by id.
func (s *Server) Intent(id string) (Intent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pi, ok := s.intents[id]
	if !ok {
		return Intent{}, false
	}
	return s.snapshot(pi), true
}
