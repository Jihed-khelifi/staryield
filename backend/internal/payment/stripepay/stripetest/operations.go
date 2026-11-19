package stripetest

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ------------------------------------------------------------- refunds

type refund struct {
	id, intent, charge, status, reason string
	amount                             int64
	currency                           string
	metadata                           map[string]string
}

func (s *Server) refundJSON(r *refund) map[string]any {
	return map[string]any{"id": r.id, "object": "refund", "amount": r.amount, "currency": r.currency, "status": r.status,
		"payment_intent": r.intent, "charge": r.charge, "reason": r.reason, "metadata": r.metadata, "created": now()}
}

func (s *Server) createRefund(req Request) (int, []byte, bool) {
	f := req.Form
	pi, found := s.intents[f.Get("payment_intent")]
	if !found {
		return notFound("payment_intent", f.Get("payment_intent"))
	}
	if pi.chargeRef == nil {
		return errorResponse(http.StatusBadRequest, "invalid_request_error", "", "payment_intent", "This PaymentIntent has no charge to refund.")
	}
	ch := pi.chargeRef
	if ch.disputed {
		return errorResponse(http.StatusBadRequest, "invalid_request_error", "charge_disputed", "",
			"Charge "+ch.id+" has been charged back; cannot issue a refund.")
	}
	amount := ch.amount - ch.refunded
	if v := f.Get("amount"); v != "" {
		amount, _ = strconv.ParseInt(v, 10, 64)
	}
	switch {
	case ch.refunded >= ch.amount:
		return errorResponse(http.StatusBadRequest, "invalid_request_error", "charge_already_refunded", "",
			"Charge "+ch.id+" has already been refunded.")
	case amount <= 0 || amount > ch.amount-ch.refunded:
		return errorResponse(http.StatusBadRequest, "invalid_request_error", "", "amount",
			"Refund amount is greater than unrefunded amount on charge "+ch.id+".")
	}
	meta := map[string]string{}
	for k, v := range f {
		if strings.HasPrefix(k, "metadata[") && len(v) > 0 {
			meta[strings.TrimSuffix(strings.TrimPrefix(k, "metadata["), "]")] = v[0]
		}
	}
	r := &refund{id: s.next("re"), intent: pi.id, charge: ch.id, status: "succeeded", reason: f.Get("reason"), amount: amount,
		currency: pi.currency, metadata: meta}
	ch.refunded += amount
	s.refunds[r.id] = r
	s.refundO = append(s.refundO, r.id)
	return ok(s.refundJSON(r))
}

// Refund describes a refund the fake issued.
type Refund struct {
	ID, PaymentIntent, Reason string
	Amount                    int64
	Metadata                  map[string]string
}

// Refunds returns the refunds issued, oldest first.
func (s *Server) Refunds() []Refund {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Refund
	for _, id := range s.refundO {
		r := s.refunds[id]
		out = append(out, Refund{ID: r.id, PaymentIntent: r.intent, Reason: r.reason, Amount: r.amount, Metadata: r.metadata})
	}
	return out
}

// ------------------------------------------------------------ disputes

type dispute struct {
	id, intent, charge, reason, status string
	amount                             int64
	currency                           string
	evidence                           map[string]string
	submitted                          bool
	dueBy                              int64
}

func (s *Server) disputeJSON(d *dispute) map[string]any {
	return map[string]any{"id": d.id, "object": "dispute", "amount": d.amount, "currency": d.currency, "reason": d.reason,
		"status": d.status, "payment_intent": d.intent, "charge": d.charge, "created": now(), "livemode": s.livemode,
		"evidence": d.evidence, "evidence_details": map[string]any{"due_by": d.dueBy, "has_evidence": len(d.evidence) > 0,
			"submission_count": map[bool]int{true: 1, false: 0}[d.submitted], "past_due": false}}
}

// OpenDispute marks the payment's charge as disputed and returns the new
// dispute's id ("dp_…"), status needs_response, evidence due in 14 days.
func (s *Server) OpenDispute(paymentIntentID, reason string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	pi := s.intents[paymentIntentID]
	if pi == nil || pi.chargeRef == nil {
		panic("stripetest: OpenDispute needs a succeeded payment")
	}
	pi.chargeRef.disputed = true
	d := &dispute{id: s.next("dp"), intent: pi.id, charge: pi.chargeRef.id, reason: reason, status: "needs_response",
		amount: pi.amount, currency: pi.currency, evidence: map[string]string{}, dueBy: time.Now().Add(14 * 24 * time.Hour).Unix()}
	s.disputes[d.id] = d
	return d.id
}

// Dispute describes a dispute the fake holds.
type Dispute struct {
	ID, PaymentIntent, Status string
	Evidence                  map[string]string
	Submitted                 bool
}

// DisputeByID returns a dispute.
func (s *Server) DisputeByID(id string) (Dispute, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, found := s.disputes[id]
	if !found {
		return Dispute{}, false
	}
	ev := map[string]string{}
	for k, v := range d.evidence {
		ev[k] = v
	}
	return Dispute{ID: d.id, PaymentIntent: d.intent, Status: d.status, Evidence: ev, Submitted: d.submitted}, true
}

func (s *Server) updateDispute(req Request, id string) (int, []byte, bool) {
	d, found := s.disputes[id]
	if !found {
		return notFound("dispute", id)
	}
	if d.submitted || d.status == "won" || d.status == "lost" {
		return errorResponse(http.StatusBadRequest, "invalid_request_error", "", "", "Evidence for this dispute has already been submitted or the dispute is closed.")
	}
	for k, v := range req.Form {
		if strings.HasPrefix(k, "evidence[") && len(v) > 0 {
			d.evidence[strings.TrimSuffix(strings.TrimPrefix(k, "evidence["), "]")] = v[0]
		}
	}
	if req.Form.Get("submit") == "true" {
		d.submitted = true
		d.status = "under_review"
	}
	return ok(s.disputeJSON(d))
}

func (s *Server) closeDispute(id string) (int, []byte, bool) {
	d, found := s.disputes[id]
	if !found {
		return notFound("dispute", id)
	}
	if d.status == "won" || d.status == "lost" {
		return errorResponse(http.StatusBadRequest, "invalid_request_error", "", "", "This dispute is already closed.")
	}
	d.status = "lost"
	return ok(s.disputeJSON(d))
}

// --------------------------------------------------------------- files

func (s *Server) createFile(req Request) (int, []byte, bool) {
	if req.Upload == nil {
		return errorResponse(http.StatusBadRequest, "invalid_request_error", "parameter_missing", "file", "Missing required param: file.")
	}
	s.files = append(s.files, *req.Upload)
	return ok(map[string]any{"id": s.next("file"), "object": "file", "purpose": req.Upload.Purpose, "filename": req.Upload.Filename,
		"size": req.Upload.Size, "created": now()})
}

// ------------------------------------------------------------- account

// Account is what GET /v1/account describes.
type Account struct {
	ID                  string
	Country             string
	DefaultCurrency     string
	ChargesEnabled      bool
	PayoutsEnabled      bool
	DetailsSubmitted    bool
	StatementDescriptor string
	CurrentlyDue        []string
	PastDue             []string
}

// DefaultAccount is a healthy account whose statement descriptor is the
// platform's default.
func DefaultAccount() Account {
	return Account{ID: "acct_fake", Country: "US", DefaultCurrency: "usd", ChargesEnabled: true, PayoutsEnabled: true,
		DetailsSubmitted: true, StatementDescriptor: "STARYIELD MEDIA LLC", CurrentlyDue: []string{}, PastDue: []string{}}
}

func (a Account) object() map[string]any {
	return map[string]any{"id": a.ID, "object": "account", "country": a.Country, "default_currency": a.DefaultCurrency,
		"charges_enabled": a.ChargesEnabled, "payouts_enabled": a.PayoutsEnabled, "details_submitted": a.DetailsSubmitted,
		"settings":     map[string]any{"payments": map[string]any{"statement_descriptor": a.StatementDescriptor}},
		"requirements": map[string]any{"currently_due": a.CurrentlyDue, "past_due": a.PastDue, "eventually_due": []string{}}}
}

// Endpoint is a webhook endpoint GET /v1/webhook_endpoints lists.
type Endpoint struct {
	ID            string
	URL           string
	Status        string
	EnabledEvents []string
	APIVersion    string
}

func (s *Server) listEndpoints() (int, []byte, bool) {
	data := []any{}
	for _, e := range s.endpoints {
		data = append(data, map[string]any{"id": e.ID, "object": "webhook_endpoint", "url": e.URL, "status": e.Status,
			"enabled_events": e.EnabledEvents, "api_version": e.APIVersion})
	}
	return ok(map[string]any{"object": "list", "url": "/v1/webhook_endpoints", "has_more": false, "data": data})
}
