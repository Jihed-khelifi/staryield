package payment

import (
	"context"
	"errors"
	"sync"
	"time"
)

// revisions 6.0 (§26.5): the card-payment operations of the sandboxes (test
// support only; see the package comment of sandbox_vault.go for the card
// behaviours).

// sandboxChallengeDelay is how long a 3-D Secure challenge takes: the first
// RetrieveCharge at least this long after the charge completes it.
const sandboxChallengeDelay = 2 * time.Second

// isChallengeCard reports the sandbox cards that ask for a 3-D Secure step.
func isChallengeCard(last4 string) bool { return last4 == "3220" || last4 == "3221" }

// waiting reports a charge whose 3-D Secure step is not completed yet.
func (c *sbxCharge) waiting() bool { return c.challenge && !c.done }

// settle completes a due challenge: 3220 succeeds, 3221 is declined.
func (c *sbxCharge) settle(now time.Time) {
	if !c.waiting() || now.Sub(c.created) < sandboxChallengeDelay {
		return
	}
	c.done = true
	if c.card.Last4 == "3221" {
		c.decline = &DeclineError{Code: "authentication_failed"}
	}
}

// current is the charge's outcome as the processor reports it now.
func (c *sbxCharge) current() (ChargeResult, *DeclineError) {
	card := c.card
	switch {
	case c.decline != nil:
		return ChargeResult{}, c.decline
	case c.waiting():
		return ChargeResult{Status: ChargeRequiresAction, ProviderRef: c.ref, NextAction: &NextAction{Type: "sandbox_3ds"}, Card: &card}, nil
	}
	res := ChargeResult{Status: ChargeSucceeded, ProviderRef: c.ref, NetworkTxnRef: c.networkRef}
	if c.challenge {
		res.Card, res.ThreeDS, res.Risk = &card, "authenticated", &RiskInfo{Level: RiskNormal, Score: 5}
	}
	return res, nil
}

// checkoutResult is current() as a checkout answer.
func (c *sbxCharge) checkoutResult() CheckoutResult {
	res, decline := c.current()
	switch {
	case decline != nil:
		return CheckoutResult{Status: CheckoutFailed, ProviderRef: c.ref, FailureCode: decline.Code}
	case res.Status == ChargeRequiresAction:
		return CheckoutResult{Status: CheckoutPending, ProviderRef: c.ref, NextAction: res.NextAction, Card: res.Card}
	}
	return CheckoutResult{Status: CheckoutSucceeded, ProviderRef: c.ref, Card: res.Card, Risk: res.Risk, ThreeDS: res.ThreeDS}
}

func (s *sandboxCore) retrieveCharge(ctx context.Context, ref string) (ChargeResult, *DeclineError, error) {
	if err := ctx.Err(); err != nil {
		return ChargeResult{}, nil, err
	}
	p := s.proc
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.byRef[ref]
	if !ok {
		return ChargeResult{}, nil, ErrNotFound
	}
	c.settle(time.Now())
	res, decline := c.current()
	return res, decline, nil
}

// sandboxOps is the processor state of disputes, the account and recorded events.
type sandboxOps struct {
	mu        sync.Mutex
	disputes  map[string]*SandboxDispute
	account   AccountStatus
	hasAcct   bool
	endpoints []WebhookEndpointInfo
	events    map[string]WebhookEvent
	seq       int
}

// SandboxDispute is what the sandbox processor holds about a dispute answered
// from the console.
type SandboxDispute struct {
	Evidence  DisputeEvidence
	Submitted bool
	Accepted  bool
}

// ErrEvidenceSubmitted means the evidence of a dispute was already submitted.
var ErrEvidenceSubmitted = errors.New("payment: sandbox: dispute evidence already submitted")

func (o *sandboxOps) recordRefund(refundRef, chargeRef string, amount int64, currency string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.events == nil {
		o.events = map[string]WebhookEvent{}
	}
	id := "evt_" + refundRef
	o.events[id] = WebhookEvent{ID: id, Type: EventChargeRefunded, ProviderType: "refund.created", ProviderRef: chargeRef,
		RefundRef: refundRef, ObjectRef: refundRef, AmountCents: amount, Currency: currency, RefundStatus: "succeeded"}
}

// RecordEvent stores a webhook event the way a processor keeps its events, so
// FetchEvent (the admin replay) can return it.
func (p *SandboxProcessor) RecordEvent(evt WebhookEvent) {
	p.ops.mu.Lock()
	defer p.ops.mu.Unlock()
	if p.ops.events == nil {
		p.ops.events = map[string]WebhookEvent{}
	}
	p.ops.events[evt.ID] = evt
}

// DisputeEvidence returns what was sent for a dispute (ok is false when
// nothing was).
func (p *SandboxProcessor) DisputeEvidence(ref string) (SandboxDispute, bool) {
	p.ops.mu.Lock()
	defer p.ops.mu.Unlock()
	d, ok := p.ops.disputes[ref]
	if !ok {
		return SandboxDispute{}, false
	}
	return *d, true
}

// SetAccount sets the fake account sandbox_a reports.
func (p *SandboxProcessor) SetAccount(a AccountStatus) {
	p.ops.mu.Lock()
	defer p.ops.mu.Unlock()
	p.ops.account, p.ops.hasAcct = a, true
}

// SetWebhookEndpoints sets the webhook endpoints the fake account lists.
func (p *SandboxProcessor) SetWebhookEndpoints(e []WebhookEndpointInfo) {
	p.ops.mu.Lock()
	defer p.ops.mu.Unlock()
	p.ops.endpoints = e
}

// SubmitDisputeEvidence implements DisputeManager: the evidence is recorded;
// submitting is final.
func (s *SandboxA) SubmitDisputeEvidence(ctx context.Context, disputeRef string, ev DisputeEvidence, submit bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	o := &s.core.proc.ops
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.disputes == nil {
		o.disputes = map[string]*SandboxDispute{}
	}
	d, ok := o.disputes[disputeRef]
	if !ok {
		d = &SandboxDispute{}
		o.disputes[disputeRef] = d
	}
	if d.Submitted || d.Accepted {
		return ErrEvidenceSubmitted
	}
	d.Evidence, d.Submitted = ev, submit
	return nil
}

// AcceptDispute implements DisputeManager.
func (s *SandboxA) AcceptDispute(ctx context.Context, disputeRef string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	o := &s.core.proc.ops
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.disputes == nil {
		o.disputes = map[string]*SandboxDispute{}
	}
	d, ok := o.disputes[disputeRef]
	if !ok {
		d = &SandboxDispute{}
		o.disputes[disputeRef] = d
	}
	d.Accepted = true
	return nil
}

// AccountStatus implements AccountInspector: the fake account (a healthy test
// account until SetAccount says otherwise).
func (s *SandboxA) AccountStatus(ctx context.Context) (AccountStatus, error) {
	if err := ctx.Err(); err != nil {
		return AccountStatus{}, err
	}
	o := &s.core.proc.ops
	o.mu.Lock()
	defer o.mu.Unlock()
	a := o.account
	if !o.hasAcct {
		a = AccountStatus{AccountID: "acct_sbx_a", Country: "US", DefaultCurrency: "usd", ChargesEnabled: true, PayoutsEnabled: true,
			DetailsSubmitted: true, StatementDescriptor: "STARYIELD MEDIA LLC"}
	}
	a.WebhookEndpoints = append([]WebhookEndpointInfo(nil), o.endpoints...)
	if a.RequirementsDue == nil {
		a.RequirementsDue = []string{}
	}
	a.CheckedAt = time.Now().UTC()
	return a, nil
}

// FetchEvent implements EventFetcher: the events RecordEvent stored and the
// refunds the processor made.
func (s *SandboxA) FetchEvent(ctx context.Context, eventID string) (*WebhookEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	o := &s.core.proc.ops
	o.mu.Lock()
	defer o.mu.Unlock()
	evt, ok := o.events[eventID]
	if !ok {
		return nil, ErrNotFound
	}
	return &evt, nil
}

// DisputeCharge marks a charge as disputed by the cardholder: the processor
// refuses to refund it afterwards (ErrChargeDisputed).
func (p *SandboxProcessor) DisputeCharge(ref string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.byRef[ref]
	if ok {
		c.disputed = true
	}
	return ok
}

// MethodRefs returns the references of the saved cards, oldest first (tests
// build payment_method.detached events from them).
func (p *SandboxProcessor) MethodRefs() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.methodOrder...)
}

// CustomerRefs returns the references of the customers, oldest first.
func (p *SandboxProcessor) CustomerRefs() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.customerOrder...)
}
