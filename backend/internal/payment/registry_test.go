package payment

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"
)

func names(ps []Named) []string {
	out := []string{}
	for _, p := range ps {
		out = append(out, p.Name())
	}
	return out
}

func newTestRegistry() (*Registry, *SandboxProcessor) {
	proc := NewSandboxProcessor()
	return NewRegistry("sandbox_a", NewSandboxA(proc, "secret-0123456789abcdef", 5*time.Minute), NewSandboxB(proc)), proc
}

func TestCapabilitiesOfSandboxes(t *testing.T) {
	r, _ := newTestRegistry()
	a, _ := r.Get("sandbox_a")
	b, _ := r.Get("sandbox_b")
	all := CapCheckout | CapWebhooks | CapVault | CapAttach | CapStoredCharge | CapLookup | CapRefund | CapClientConfig | CapInspect
	if got := CapabilitiesOf(a); got != all {
		t.Fatalf("sandbox_a capabilities: %v", got.Names())
	}
	got := CapabilitiesOf(b)
	if !got.Has(CapVault|CapAttach|CapStoredCharge|CapRefund|CapClientConfig|CapInspect) || got.Has(CapCheckout) || got.Has(CapLookup) || got.Has(CapWebhooks) {
		t.Fatalf("sandbox_b capabilities: %v", got.Names())
	}
	if (&Sandbox{}).Name() != "sandbox" || CapabilitiesOf(NewSandbox("x", time.Minute))&CapRefund == 0 {
		t.Fatal("legacy sandbox must support refunds")
	}
	if len(Capability(0).Names()) != 0 {
		t.Fatal("empty capability names")
	}
}

func TestRoutingRules(t *testing.T) {
	r, _ := newTestRegistry()
	user := "11111111-1111-1111-1111-111111111111"
	plan := "22222222-2222-2222-2222-222222222222"
	rules := []Rule{
		{ID: "default", Priority: 1000, Enabled: true, Purpose: PurposeAny, Providers: []string{"sandbox_a"}},
		{ID: "vip", Priority: 10, Enabled: true, Purpose: PurposeAny, Providers: []string{"sandbox_b"}, UserIDs: []string{user}},
		{ID: "france", Priority: 20, Enabled: true, Purpose: PurposeAny, Providers: []string{"sandbox_b", "sandbox_a"}, Countries: []string{"FR"}},
		{ID: "eur-subs", Priority: 30, Enabled: true, Purpose: PurposeSubscription, Providers: []string{"sandbox_b"}, Currencies: []string{"EUR"}},
		{ID: "plan", Priority: 40, Enabled: true, Purpose: PurposeSubscription, Providers: []string{"sandbox_b"}, PlanIDs: []string{plan}},
		{ID: "off", Priority: 1, Enabled: false, Purpose: PurposeAny, Providers: []string{"sandbox_b"}},
	}
	cases := []struct {
		name string
		in   RouteInput
		want []string
		rule string
		err  error
	}{
		{"default", RouteInput{UserID: "u", Purpose: PurposeCredits, Currency: "USD"}, []string{"sandbox_a"}, "default", nil},
		{"allow-listed user", RouteInput{UserID: user, Purpose: PurposeCredits}, []string{"sandbox_b"}, "vip", nil},
		{"country FR", RouteInput{UserID: "u", Countries: []string{"FR"}, Purpose: PurposeCredits}, []string{"sandbox_b", "sandbox_a"}, "france", nil},
		{"unknown country skips restricted rules", RouteInput{UserID: "u", Purpose: PurposeCredits}, []string{"sandbox_a"}, "default", nil},
		{"purpose-specific rule", RouteInput{UserID: "u", Currency: "EUR", Purpose: PurposeSubscription}, []string{"sandbox_b"}, "eur-subs", nil},
		{"purpose mismatch", RouteInput{UserID: "u", Currency: "EUR", Purpose: PurposeCredits}, []string{"sandbox_a"}, "default", nil},
		{"plan rule", RouteInput{UserID: "u", PlanID: plan, Purpose: PurposeSubscription}, []string{"sandbox_b"}, "plan", nil},
		{"capability filter keeps order", RouteInput{UserID: "u", Countries: []string{"FR"}, Purpose: PurposeCredits, Needs: CapCheckout},
			[]string{"sandbox_a"}, "france", nil},
		{"rule without capable provider refuses (no fall-through)", RouteInput{UserID: user, Purpose: PurposeCredits, Needs: CapCheckout},
			nil, "vip", ErrNoProvider},
		{"stricter of two countries wins", RouteInput{UserID: "u", Countries: []string{"FR", "US"}, Purpose: PurposeCredits},
			[]string{"sandbox_a"}, "france+default", nil},
	}
	for _, c := range cases {
		got, rule, err := r.Route(c.in, rules)
		if !errors.Is(err, c.err) {
			t.Errorf("%s: err %v want %v", c.name, err, c.err)
			continue
		}
		if c.err == nil && (len(names(got)) != len(c.want) || rule != c.rule) {
			t.Errorf("%s: got %v (%s) want %v (%s)", c.name, names(got), rule, c.want, c.rule)
			continue
		}
		for i := range c.want {
			if names(got)[i] != c.want[i] {
				t.Errorf("%s: order %v want %v", c.name, names(got), c.want)
			}
		}
	}
	// FR allows only sandbox_b and the card's country only sandbox_a: no
	// provider satisfies both, so the payment is refused.
	strict := []Rule{
		{ID: "fr", Priority: 1, Enabled: true, Providers: []string{"sandbox_b"}, Countries: []string{"FR"}},
		{ID: "us", Priority: 2, Enabled: true, Providers: []string{"sandbox_a"}, Countries: []string{"US"}},
	}
	if _, _, err := r.Route(RouteInput{Countries: []string{"FR", "US"}}, strict); !errors.Is(err, ErrNoProvider) {
		t.Fatalf("conflicting countries must refuse: %v", err)
	}
	// Unregistered providers are ignored; an empty registry refuses.
	if got, _, err := r.Route(RouteInput{}, []Rule{{ID: "x", Enabled: true, Providers: []string{"stripe", "sandbox_b"}}}); err != nil || names(got)[0] != "sandbox_b" {
		t.Fatalf("unregistered provider: %v %v", names(got), err)
	}
	if _, _, err := NewRegistry("").Route(RouteInput{}, nil); !errors.Is(err, ErrNoProvider) {
		t.Fatal("empty registry must refuse")
	}
	if _, _, err := NewRegistry("", NewSandboxB(NewSandboxProcessor())).Route(RouteInput{}, nil); !errors.Is(err, ErrNoProvider) {
		t.Fatal("no default and no rule must refuse")
	}
}

func TestSandboxVaultAndCharges(t *testing.T) {
	ctx := context.Background()
	r, proc := newTestRegistry()
	p, _ := r.Get("sandbox_a")
	a := p.(*SandboxA)
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	cus, err := a.EnsureCustomer(ctx, CustomerRequest{})
	if err != nil || cus == "" {
		t.Fatalf("customer: %q %v", cus, err)
	}
	if again, _ := a.EnsureCustomer(ctx, CustomerRequest{ExistingRef: cus}); again != cus {
		t.Fatal("existing customer must be confirmed")
	}
	// Token validation.
	for _, tok := range []string{"", "sbx_b_visa_4242_12_2030", "sbx_a_visa_42_12_2030", "sbx_a_diners_4242_12_2030", "sbx_a_visa_4242_13_2030",
		"4242424242424242", "sbx_a_visa_4242_12_2030_FRA"} {
		if _, _, err := a.Attach(ctx, AttachRequest{CustomerRef: cus, ClientToken: tok, Now: now}); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("token %q: %v", tok, err)
		}
	}
	if _, _, err := a.Attach(ctx, AttachRequest{CustomerRef: cus, ClientToken: "sbx_a_visa_4242_10_2026", Now: now}); err == nil {
		t.Fatal("expired card attached")
	} else if d, ok := AsDecline(err); !ok || d.Code != "expired_card" {
		t.Fatalf("expired: %v", err)
	}
	if _, _, err := a.Attach(ctx, AttachRequest{CustomerRef: "cus_missing", ClientToken: "sbx_a_visa_4242_12_2030", Now: now}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown customer: %v", err)
	}
	ref, meta, err := a.Attach(ctx, AttachRequest{CustomerRef: cus, ClientToken: "sbx_a_visa_4242_12_2030_fr", IdempotencyKey: "k1", Now: now})
	if err != nil || meta.Brand != "visa" || meta.Last4 != "4242" || meta.ExpMonth != 12 || meta.ExpYear != 2030 || meta.Country != "FR" || meta.Fingerprint == "" {
		t.Fatalf("attach: %v %+v", err, meta)
	}
	if again, _, _ := a.Attach(ctx, AttachRequest{CustomerRef: cus, ClientToken: "sbx_a_visa_4242_12_2030_fr", IdempotencyKey: "k1", Now: now}); again != ref {
		t.Fatal("attach must be idempotent")
	}

	charge := func(key string, amount int64, method string) (ChargeResult, error) {
		return a.ChargeStored(ctx, StoredCharge{CustomerRef: cus, MethodRef: method, AmountCents: amount, Currency: "USD", IdempotencyKey: key,
			Reason: ReasonRecurring})
	}
	ok1, err := charge("sub:1:p1:a1", 999, ref)
	if err != nil || ok1.Status != ChargeSucceeded || ok1.ProviderRef == "" || ok1.NetworkTxnRef == "" {
		t.Fatalf("charge: %+v %v", ok1, err)
	}
	replay, err := charge("sub:1:p1:a1", 999, ref)
	if err != nil || replay.ProviderRef != ok1.ProviderRef || proc.ChargeRequests("sub:1:p1:a1") != 2 || proc.SuccessfulCharges() != 1 {
		t.Fatalf("idempotent replay: %+v %v (requests %d, charges %d)", replay, err, proc.ChargeRequests("sub:1:p1:a1"), proc.SuccessfulCharges())
	}
	if _, err := charge("hard", 1013, ref); err == nil {
		t.Fatal("amount .13 must hard-decline")
	} else if d, _ := AsDecline(err); d == nil || d.Soft {
		t.Fatalf("hard decline: %v", err)
	}
	if _, err := charge("soft", 1014, ref); err == nil {
		t.Fatal("amount .14 must soft-decline")
	} else if d, _ := AsDecline(err); d == nil || !d.Soft {
		t.Fatalf("soft decline: %v", err)
	}
	// Timeout: charged at the provider but the response is lost.
	if _, err := charge("lost", 1015, ref); !errors.Is(err, ErrUnknownOutcome) {
		t.Fatalf("timeout: %v", err)
	}
	res, err := a.Lookup(ctx, "lost")
	if err != nil || !res.Found || res.Result.Status != ChargeSucceeded {
		t.Fatalf("lookup after timeout: %+v %v", res, err)
	}
	if amount, ok := proc.Charged("lost"); !ok || amount != 1015 {
		t.Fatal("lost response must still be charged once")
	}
	if res, _ := a.Lookup(ctx, "never"); res.Found {
		t.Fatal("lookup of an unknown key")
	}
	if res, _ := a.Lookup(ctx, "hard"); !res.Found || res.Decline == nil {
		t.Fatal("lookup of a decline")
	}
	// Card behaviours by number.
	soft, _, _ := a.Attach(ctx, AttachRequest{CustomerRef: cus, ClientToken: "sbx_a_visa_9995_12_2030", Now: now})
	hard, _, _ := a.Attach(ctx, AttachRequest{CustomerRef: cus, ClientToken: "sbx_a_mastercard_0002_12_2030", Now: now})
	if _, err := charge("c-soft", 999, soft); err == nil {
		t.Fatal("9995 must soft-decline")
	}
	if _, err := charge("c-hard", 999, hard); err == nil {
		t.Fatal("0002 must hard-decline")
	}
	// Detached cards cannot be charged.
	if err := a.Detach(ctx, cus, soft); err != nil {
		t.Fatal(err)
	}
	if _, err := charge("detached", 999, soft); err == nil {
		t.Fatal("detached card charged")
	}
	if err := a.Detach(ctx, "other", hard); !errors.Is(err, ErrNotFound) {
		t.Fatal("detach of a foreign card")
	}
	// Refunds.
	if _, err := a.Refund(ctx, RefundRequest{ProviderRef: ok1.ProviderRef, AmountCents: 1000, IdempotencyKey: "r0"}); !errors.Is(err, ErrRefundTooLarge) {
		t.Fatalf("over-refund: %v", err)
	}
	r1, err := a.Refund(ctx, RefundRequest{ProviderRef: ok1.ProviderRef, AmountCents: 500, IdempotencyKey: "r1"})
	if err != nil || r1.ProviderRef == "" {
		t.Fatalf("partial refund: %v", err)
	}
	if again, _ := a.Refund(ctx, RefundRequest{ProviderRef: ok1.ProviderRef, AmountCents: 500, IdempotencyKey: "r1"}); again != r1 {
		t.Fatal("refund must be idempotent")
	}
	if _, err := a.Refund(ctx, RefundRequest{ProviderRef: ok1.ProviderRef, AmountCents: 500, IdempotencyKey: "r2"}); !errors.Is(err, ErrRefundTooLarge) {
		t.Fatal("only 499 cents remain")
	}
	if _, err := a.Refund(ctx, RefundRequest{ProviderRef: "ch_missing", AmountCents: 1}); !errors.Is(err, ErrNotFound) {
		t.Fatal("refund of an unknown charge")
	}
	if err := a.DeleteCustomer(ctx, cus); err != nil {
		t.Fatal(err)
	}
	if _, err := charge("after-delete", 999, ref); err == nil {
		t.Fatal("charging a deleted customer's card")
	}
	cfg, err := a.ClientConfig(ctx, ClientConfigRequest{})
	if err != nil || cfg.Provider != "sandbox_a" || cfg.Type != "sandbox" || len(cfg.TestCards) == 0 {
		t.Fatalf("client config: %+v", cfg)
	}
}

func TestSandboxesShareTheProcessor(t *testing.T) {
	ctx := context.Background()
	proc := NewSandboxProcessor()
	one, two := NewSandboxB(proc), NewSandboxB(proc) // two API replicas
	cus, _ := one.EnsureCustomer(ctx, CustomerRequest{})
	ref, _, err := one.Attach(ctx, AttachRequest{CustomerRef: cus, ClientToken: "sbx_b_visa_4242_12_2030", Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	req := StoredCharge{CustomerRef: cus, MethodRef: ref, AmountCents: 999, Currency: "USD", IdempotencyKey: "shared"}
	a, err1 := one.ChargeStored(ctx, req)
	b, err2 := two.ChargeStored(ctx, req)
	if err1 != nil || err2 != nil || a.ProviderRef != b.ProviderRef || proc.SuccessfulCharges() != 1 {
		t.Fatalf("replicas must share idempotency: %v %v %s %s", err1, err2, a.ProviderRef, b.ProviderRef)
	}
}

func TestSandboxAWebhooksAndCheckout(t *testing.T) {
	ctx := context.Background()
	secret := "webhook-secret-0123456789"
	a := NewSandboxA(NewSandboxProcessor(), secret, 5*time.Minute)
	now := time.Now()
	body, _ := json.Marshal(WebhookEvent{ID: "evt_1", Type: "charge.refunded", ProviderRef: "ch_1", AmountCents: 100})
	h := http.Header{}
	h.Set("X-Staryield-Signature", Sign([]byte(secret), body, now))
	evt, err := a.VerifyWebhookRequest(WebhookRequest{Header: h, Body: body, Now: now})
	if err != nil || evt.ID != "evt_1" || evt.AmountCents != 100 {
		t.Fatalf("webhook: %+v %v", evt, err)
	}
	h.Set("X-Staryield-Signature", Sign([]byte("other"), body, now))
	if _, err := a.VerifyWebhookRequest(WebhookRequest{Header: h, Body: body, Now: now}); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("forged webhook: %v", err)
	}
	// One-off token checkout.
	res, err := a.Checkout(ctx, CheckoutRequest{AmountCents: 999, PaymentToken: "sbx_a_visa_4242_12_2030", IdempotencyKey: "purchase:1"})
	if err != nil || res.Status != CheckoutSucceeded {
		t.Fatalf("token checkout: %+v %v", res, err)
	}
	if again, _ := a.Checkout(ctx, CheckoutRequest{AmountCents: 999, PaymentToken: "sbx_a_visa_4242_12_2030", IdempotencyKey: "purchase:1"}); again.ProviderRef != res.ProviderRef {
		t.Fatal("checkout must be idempotent")
	}
	if res, _ := a.Checkout(ctx, CheckoutRequest{AmountCents: 999, PaymentToken: "sbx_a_visa_0002_12_2030", IdempotencyKey: "purchase:2"}); res.Status != CheckoutFailed {
		t.Fatalf("declined token checkout: %+v", res)
	}
	if _, err := a.Checkout(ctx, CheckoutRequest{AmountCents: 999, PaymentToken: "sbx_b_visa_4242_12_2030"}); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("foreign token: %v", err)
	}
}

// revisions 5.0: TokenInspector (§24.2) is an optional capability named
// "inspect"; the sandboxes report the card behind a token with the same
// fingerprint their Attach derives, charging nothing.
func TestTokenInspectorCapability(t *testing.T) {
	r, proc := newTestRegistry()
	a, _ := r.Get("sandbox_a")
	b, _ := r.Get("sandbox_b")
	for _, p := range []Named{a, b} {
		if !CapabilitiesOf(p).Has(CapInspect) {
			t.Fatalf("%s must report the inspect capability: %v", p.Name(), CapabilitiesOf(p).Names())
		}
	}
	if CapabilitiesOf(NewSandbox("x", time.Minute)).Has(CapInspect) {
		t.Fatal("the legacy hosted sandbox cannot inspect tokens")
	}
	names := CapabilitiesOf(a).Names()
	if names[len(names)-1] != "inspect" {
		t.Fatalf("capability names: %v", names)
	}

	ctx := context.Background()
	token := "sbx_a_visa_4242_12_2035_fr"
	inspector := a.(TokenInspector)
	meta, err := inspector.InspectToken(ctx, token)
	if err != nil || meta.Last4 != "4242" || meta.Brand != "visa" || meta.Country != "FR" || meta.Fingerprint == "" {
		t.Fatalf("inspect: %+v %v", meta, err)
	}
	// Same fingerprint as Attach reports for the same card.
	cust, _ := a.(CustomerVault).EnsureCustomer(ctx, CustomerRequest{})
	_, attached, err := a.(MethodAttacher).Attach(ctx, AttachRequest{CustomerRef: cust, ClientToken: token, Now: time.Now()})
	if err != nil || attached.Fingerprint != meta.Fingerprint {
		t.Fatalf("inspect and attach disagree: %q vs %q (%v)", meta.Fingerprint, attached.Fingerprint, err)
	}
	// The same card has the same fingerprint at both sandboxes' own letter; a
	// different card does not.
	other, _ := inspector.InspectToken(ctx, "sbx_a_visa_4444_12_2035")
	if other.Fingerprint == meta.Fingerprint {
		t.Fatal("different cards must have different fingerprints")
	}
	// Inspecting never charges, saves or consumes anything.
	if proc.SuccessfulCharges() != 0 {
		t.Fatal("inspecting a token must not charge")
	}
	// A token of the other sandbox, a malformed one and an expired card.
	if _, err := inspector.InspectToken(ctx, "sbx_b_visa_4242_12_2035"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("foreign token: %v", err)
	}
	if _, err := inspector.InspectToken(ctx, "garbage"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("malformed token: %v", err)
	}
	if _, err := inspector.InspectToken(ctx, "sbx_a_visa_4242_01_2001"); err == nil {
		t.Fatal("an expired card must not be inspectable")
	} else if d, ok := AsDecline(err); !ok || d.Code != "expired_card" {
		t.Fatalf("expired card: %v", err)
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := inspector.InspectToken(cctx, token); err == nil {
		t.Fatal("a cancelled context must be honoured")
	}
	// Routing can require the capability.
	got, _, err := r.Route(RouteInput{Purpose: PurposeCredits, Needs: CapInspect}, nil)
	if err != nil || len(got) != 1 || got[0].Name() != "sandbox_a" {
		t.Fatalf("route by inspect: %v %v", got, err)
	}
}
