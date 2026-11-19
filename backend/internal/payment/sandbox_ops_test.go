package payment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// revisions 6.0 (§26.5): the sandbox's card payment operations.

func backdate(p *SandboxProcessor, ref string, by time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.byRef[ref].created = time.Now().Add(-by)
}

func TestSandboxThreeDSecureTokenCheckout(t *testing.T) {
	r, proc := newTestRegistry()
	a := mustProvider[*SandboxA](t, r, "sandbox_a")
	ctx := context.Background()
	res, err := a.Checkout(ctx, CheckoutRequest{PurchaseID: uuid.New(), AmountCents: 999, Currency: "USD", PaymentToken: "sbx_a_visa_3220_12_2035_fr", IdempotencyKey: "k-3ds-ok"})
	if err != nil || res.Status != CheckoutPending || res.NextAction == nil || res.NextAction.Type != "sandbox_3ds" || res.ProviderRef == "" {
		t.Fatalf("3220 must ask for a 3-D Secure step: %+v %v", res, err)
	}
	if n := proc.SuccessfulCharges(); n != 0 {
		t.Fatalf("a charge waiting for the customer is not a success: %d", n)
	}
	// Too early: still waiting. Replays and Lookup keep saying so.
	got, decline, err := a.RetrieveCharge(ctx, res.ProviderRef)
	if err != nil || decline != nil || got.Status != ChargeRequiresAction || got.NextAction == nil {
		t.Fatalf("early retrieve: %+v %v %v", got, decline, err)
	}
	again, _ := a.Checkout(ctx, CheckoutRequest{AmountCents: 999, Currency: "USD", PaymentToken: "sbx_a_visa_3220_12_2035_fr", IdempotencyKey: "k-3ds-ok"})
	if again.Status != CheckoutPending || again.ProviderRef != res.ProviderRef {
		t.Fatalf("replay: %+v", again)
	}
	if lk, _ := a.Lookup(ctx, "k-3ds-ok"); !lk.Found || lk.Result.Status != ChargeRequiresAction {
		t.Fatalf("lookup: %+v", lk)
	}
	backdate(proc, res.ProviderRef, 3*time.Second)
	got, decline, err = a.RetrieveCharge(ctx, res.ProviderRef)
	if err != nil || decline != nil || got.Status != ChargeSucceeded || got.ThreeDS != "authenticated" || got.Card == nil || got.Card.Last4 != "3220" ||
		got.Risk == nil || got.Risk.Level != RiskNormal {
		t.Fatalf("completed: %+v %v %v", got, decline, err)
	}
	if proc.SuccessfulCharges() != 1 {
		t.Fatal("the completed charge counts")
	}
	if done, _ := a.Checkout(ctx, CheckoutRequest{AmountCents: 999, Currency: "USD", PaymentToken: "sbx_a_visa_3220_12_2035_fr", IdempotencyKey: "k-3ds-ok"}); done.Status != CheckoutSucceeded {
		t.Fatalf("replay after completion: %+v", done)
	}
	if _, err := a.Refund(ctx, RefundRequest{ProviderRef: res.ProviderRef, AmountCents: 999, IdempotencyKey: "r1"}); err != nil {
		t.Fatalf("a completed charge can be refunded: %v", err)
	}
}

func TestSandboxThreeDSecureFailureAndSavedCard(t *testing.T) {
	r, proc := newTestRegistry()
	a := mustProvider[*SandboxA](t, r, "sandbox_a")
	ctx := context.Background()
	res, _ := a.Checkout(ctx, CheckoutRequest{AmountCents: 500, Currency: "USD", PaymentToken: "sbx_a_visa_3221_12_2035", IdempotencyKey: "k-3ds-fail"})
	if res.Status != CheckoutPending {
		t.Fatalf("3221 also asks first: %+v", res)
	}
	backdate(proc, res.ProviderRef, 3*time.Second)
	_, decline, err := a.RetrieveCharge(ctx, res.ProviderRef)
	if err != nil || decline == nil || decline.Code != "authentication_failed" {
		t.Fatalf("3221 completes as a decline: %v %v", decline, err)
	}
	if refunded := func() error {
		_, err := a.Refund(ctx, RefundRequest{ProviderRef: res.ProviderRef, AmountCents: 500})
		return err
	}(); !errors.Is(refunded, ErrNotFound) {
		t.Fatalf("a declined charge cannot be refunded: %v", refunded)
	}

	// A saved card goes through the same challenge.
	cust, _ := a.EnsureCustomer(ctx, CustomerRequest{UserID: uuid.New()})
	method, _, err := a.Attach(ctx, AttachRequest{CustomerRef: cust, ClientToken: "sbx_a_visa_3220_12_2035", IdempotencyKey: "pm-1", Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	charged, err := a.ChargeStored(ctx, StoredCharge{CustomerRef: cust, MethodRef: method, AmountCents: 700, Currency: "USD", IdempotencyKey: "k-saved"})
	if err != nil || charged.Status != ChargeRequiresAction || charged.NextAction == nil || charged.ProviderRef == "" {
		t.Fatalf("saved 3220 card: %+v %v", charged, err)
	}
	backdate(proc, charged.ProviderRef, 3*time.Second)
	if done, decline, err := a.RetrieveCharge(ctx, charged.ProviderRef); err != nil || decline != nil || done.Status != ChargeSucceeded {
		t.Fatalf("saved card completion: %+v %v %v", done, decline, err)
	}
	if _, _, err := a.RetrieveCharge(ctx, "ch_unknown"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown charge: %v", err)
	}
	// Ordinary cards are untouched: approved at once, no 3-D Secure.
	plain, _ := a.Checkout(ctx, CheckoutRequest{AmountCents: 999, Currency: "USD", PaymentToken: "sbx_a_visa_4242_12_2035", IdempotencyKey: "k-plain"})
	if plain.Status != CheckoutSucceeded || plain.NextAction != nil || plain.Card != nil {
		t.Fatalf("plain checkout changed: %+v", plain)
	}
}

func TestSandboxBAlsoRetrievesCharges(t *testing.T) {
	r, _ := newTestRegistry()
	b := mustProvider[*SandboxB](t, r, "sandbox_b")
	if !CapabilitiesOf(b).Has(CapRetrieve) || CapabilitiesOf(b).Has(CapDisputes) || CapabilitiesOf(b).Has(CapAccount) {
		t.Fatalf("capabilities: %v", CapabilitiesOf(b).Names())
	}
}

func TestSandboxDisputeManagerRecordsEvidenceAndSubmitIsFinal(t *testing.T) {
	r, proc := newTestRegistry()
	a := mustProvider[*SandboxA](t, r, "sandbox_a")
	ctx := context.Background()
	if err := a.SubmitDisputeEvidence(ctx, "dp_1", DisputeEvidence{ProductDescription: "draft"}, false); err != nil {
		t.Fatal(err)
	}
	if d, ok := proc.DisputeEvidence("dp_1"); !ok || d.Submitted || d.Evidence.ProductDescription != "draft" {
		t.Fatalf("saved draft: %+v %v", d, ok)
	}
	if err := a.SubmitDisputeEvidence(ctx, "dp_1", DisputeEvidence{ProductDescription: "final", ReceiptPDF: []byte("%PDF"), ReceiptFilename: "i.pdf"}, true); err != nil {
		t.Fatal(err)
	}
	if err := a.SubmitDisputeEvidence(ctx, "dp_1", DisputeEvidence{ProductDescription: "again"}, true); !errors.Is(err, ErrEvidenceSubmitted) {
		t.Fatalf("submit is final: %v", err)
	}
	if d, _ := proc.DisputeEvidence("dp_1"); !d.Submitted || d.Evidence.ProductDescription != "final" || string(d.Evidence.ReceiptPDF) != "%PDF" {
		t.Fatalf("submitted: %+v", d)
	}
	if err := a.AcceptDispute(ctx, "dp_2"); err != nil {
		t.Fatal(err)
	}
	if d, _ := proc.DisputeEvidence("dp_2"); !d.Accepted {
		t.Fatalf("accepted: %+v", d)
	}
	if err := a.SubmitDisputeEvidence(ctx, "dp_2", DisputeEvidence{}, true); !errors.Is(err, ErrEvidenceSubmitted) {
		t.Fatalf("an accepted dispute takes no evidence: %v", err)
	}
}

func TestSandboxAccountAndEvents(t *testing.T) {
	r, proc := newTestRegistry()
	a := mustProvider[*SandboxA](t, r, "sandbox_a")
	ctx := context.Background()
	st, err := a.AccountStatus(ctx)
	if err != nil || !st.ChargesEnabled || st.Livemode || st.StatementDescriptor == "" || len(st.WebhookEndpoints) != 0 || st.RequirementsDue == nil {
		t.Fatalf("default account: %+v %v", st, err)
	}
	proc.SetAccount(AccountStatus{AccountID: "acct_x", ChargesEnabled: false, StatementDescriptor: "OTHER", RequirementsDue: []string{"external_account"}})
	proc.SetWebhookEndpoints([]WebhookEndpointInfo{{ID: "we_1", URL: "https://api.test/hook", Status: "enabled", EnabledEvents: []string{"*"}}})
	st, _ = a.AccountStatus(ctx)
	if st.AccountID != "acct_x" || st.ChargesEnabled || len(st.WebhookEndpoints) != 1 || st.RequirementsDue[0] != "external_account" {
		t.Fatalf("configured account: %+v", st)
	}

	if _, err := a.FetchEvent(ctx, "evt_none"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown event: %v", err)
	}
	proc.RecordEvent(WebhookEvent{ID: "evt_1", Type: EventChargeDisputed, ProviderRef: "ch_1", DisputeRef: "dp_1"})
	evt, err := a.FetchEvent(ctx, "evt_1")
	if err != nil || evt.DisputeRef != "dp_1" {
		t.Fatalf("recorded event: %+v %v", evt, err)
	}
	// Refunds the processor makes are recorded too (an admin can replay them).
	res, _ := a.Checkout(ctx, CheckoutRequest{AmountCents: 400, Currency: "USD", IdempotencyKey: "k-ev"})
	rf, err := a.Refund(ctx, RefundRequest{ProviderRef: res.ProviderRef, AmountCents: 400, Currency: "USD", IdempotencyKey: "r-ev"})
	if err != nil {
		t.Fatal(err)
	}
	if e, err := a.FetchEvent(ctx, "evt_"+rf.ProviderRef); err != nil || e.Type != EventChargeRefunded || e.RefundRef != rf.ProviderRef || e.AmountCents != 400 {
		t.Fatalf("refund event: %+v %v", e, err)
	}
}

func mustProvider[T any](t *testing.T, r *Registry, name string) T {
	t.Helper()
	p, ok := r.Get(name)
	if !ok {
		t.Fatalf("no provider %s", name)
	}
	v, ok := p.(T)
	if !ok {
		t.Fatalf("provider %s has the wrong type", name)
	}
	return v
}
