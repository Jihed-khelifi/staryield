package stripepay_test

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
)

func TestProviderImplementsEveryCapability(t *testing.T) {
	f := newFixture(t)
	if f.p.Name() != "stripe" {
		t.Fatalf("name: %s", f.p.Name())
	}
	got := payment.CapabilitiesOf(f.p)
	want := payment.CapCheckout | payment.CapWebhooks | payment.CapVault | payment.CapAttach | payment.CapStoredCharge | payment.CapLookup |
		payment.CapRefund | payment.CapClientConfig | payment.CapInspect | payment.CapRetrieve | payment.CapDisputes | payment.CapAccount
	if got != want {
		t.Fatalf("capabilities: %v, want %v", got.Names(), want.Names())
	}
	if _, ok := any(f.p).(payment.EventFetcher); !ok {
		t.Fatal("the provider must fetch events")
	}
}

func TestCheckoutWithConfirmationTokenSendsTheContract(t *testing.T) {
	f := newFixture(t)
	cus := f.customer()
	req := checkoutRequest("ctoken_ok_visa_4242_FR")
	req.CustomerRef = cus
	req.ReturnURL = "https://staryield.net/en/credits/return?purchase=" + req.PurchaseID.String()
	req.ThreeDSecure = payment.ThreeDSAny
	req.Metadata = map[string]string{"sy_tx_id": "tx-123"}
	res, err := f.p.Checkout(ctx(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != payment.CheckoutSucceeded || !strings.HasPrefix(res.ProviderRef, "pi_") || res.NextAction != nil {
		t.Fatalf("result: %+v", res)
	}
	if res.Card == nil || res.Card.Brand != "visa" || res.Card.Last4 != "4242" || res.Card.Country != "FR" || res.Card.Fingerprint == "" ||
		res.Card.ExpMonth != 12 || res.Card.ExpYear != 2035 {
		t.Fatalf("card: %+v", res.Card)
	}
	if res.Risk == nil || res.Risk.Level != payment.RiskNormal || res.Risk.Score != -1 || res.ThreeDS != "not_supported" {
		t.Fatalf("risk %+v 3ds %q", res.Risk, res.ThreeDS)
	}

	r := f.lastPost("/v1/payment_intents")
	if r.IdempotencyKey() != req.IdempotencyKey {
		t.Fatalf("Idempotency-Key %q, want %q", r.IdempotencyKey(), req.IdempotencyKey)
	}
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer "+testSecretKey) {
		t.Fatal("the secret key must authenticate every request")
	}
	form := map[string]string{"amount": "2500", "currency": "usd", "confirm": "true", "confirmation_token": "ctoken_ok_visa_4242_FR",
		"customer": cus, "description": "250 credits", "return_url": req.ReturnURL,
		"metadata[sy_key]": req.IdempotencyKey, "metadata[sy_purchase_id]": req.PurchaseID.String(), "metadata[sy_user_id]": req.UserID.String(),
		"metadata[sy_tx_id]": "tx-123", "payment_method_options[card][request_three_d_secure]": "any"}
	for k, want := range form {
		if got := r.Form.Get(k); got != want {
			t.Errorf("form %s = %q, want %q", k, got, want)
		}
	}
	if !hasValue(r.Form, "card") || !hasValue(r.Form, "latest_charge") {
		t.Errorf("card must be the only payment method type and latest_charge expanded: %v", r.Form)
	}
	for _, banned := range []string{"receipt_email", "statement_descriptor", "statement_descriptor_suffix", "payment_method"} {
		if _, found := r.Form[banned]; found {
			t.Errorf("%s must never be sent", banned)
		}
	}
}

func hasValue(v url.Values, want string) bool {
	for _, vs := range v {
		for _, x := range vs {
			if x == want {
				return true
			}
		}
	}
	return false
}

func TestCheckoutAcceptsAPaymentMethodAndRefusesOtherTokens(t *testing.T) {
	f := newFixture(t)
	cus := f.customer()
	pm := f.fake.AddPaymentMethod(cus, "ok_visa_4242")
	req := checkoutRequest(pm)
	req.CustomerRef = cus
	if res, err := f.p.Checkout(ctx(), req); err != nil || res.Status != payment.CheckoutSucceeded {
		t.Fatalf("pm checkout: %+v %v", res, err)
	}
	if got := f.lastPost("/v1/payment_intents").Form.Get("payment_method"); got != pm {
		t.Fatalf("payment_method: %q", got)
	}
	f.fake.ResetRequests()
	for _, tok := range []string{"", "tok_visa", "4242424242424242", "sbx_a_visa_4242_12_2035"} {
		if _, err := f.p.Checkout(ctx(), checkoutRequest(tok)); !errors.Is(err, payment.ErrInvalidToken) {
			t.Errorf("token %q: %v", tok, err)
		}
	}
	if len(f.fake.Requests()) != 0 {
		t.Fatal("a token of the wrong shape must be refused without calling Stripe")
	}
	if _, err := f.p.Checkout(ctx(), payment.CheckoutRequest{PaymentToken: "ctoken_ok_visa_4242", Currency: "USD"}); err == nil {
		t.Fatal("a zero amount must be refused")
	}
}

func TestCheckoutDeclineMappingTable(t *testing.T) {
	cases := []struct {
		code string // test token suffix = Stripe decline code
		want string // reason the platform stores
		soft bool
	}{
		{"insufficient_funds", "insufficient_funds", true},
		{"processing_error", "processing_error", true},
		{"try_again_later", "try_again_later", true},
		{"issuer_not_available", "issuer_not_available", true},
		{"reenter_transaction", "reenter_transaction", true},
		{"approve_with_id", "approve_with_id", true},
		{"card_declined", "card_declined", false},
		{"generic_decline", "card_declined", false},
		{"do_not_honor", "do_not_honor", false},
		{"lost_card", "lost_card", false},
		{"stolen_card", "stolen_card", false},
		{"fraudulent", "fraudulent", false},
		{"expired_card", "expired_card", false},
		{"incorrect_cvc", "incorrect_cvc", false},
		{"card_velocity_exceeded", "card_velocity_exceeded", false},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			f := newFixture(t)
			cus := f.customer()
			// Stripe-token checkout: a decline is a failed result carrying the PaymentIntent id.
			res, err := f.p.Checkout(ctx(), checkoutRequest("ctoken_decline_"+tc.code))
			if err != nil || res.Status != payment.CheckoutFailed || res.FailureCode != tc.want || !strings.HasPrefix(res.ProviderRef, "pi_") {
				t.Fatalf("checkout: %+v %v", res, err)
			}
			// Saved-card charge: a decline is a *DeclineError with the softness.
			pm := f.fake.AddPaymentMethod(cus, "decline_"+tc.code)
			_, err = f.p.ChargeStored(ctx(), payment.StoredCharge{CustomerRef: cus, MethodRef: pm, AmountCents: 999, Currency: "USD",
				IdempotencyKey: "sub:" + uuid.NewString(), Reason: payment.ReasonCustomerInitiated})
			d, ok := payment.AsDecline(err)
			if !ok || d.Code != tc.want || d.Soft != tc.soft {
				t.Fatalf("stored charge: %v (%+v)", err, d)
			}
			// The same key replays the same decline (idempotent).
		})
	}
}

func TestCheckoutOutcomeErrors(t *testing.T) {
	cases := []struct {
		name  string
		token string
		is    error
	}{
		{"server error", "ctoken_error500", payment.ErrUnknownOutcome},
		{"rate limited", "ctoken_ratelimit", payment.ErrUnknownOutcome},
		{"lost response", "ctoken_timeout_visa_4242", payment.ErrUnknownOutcome},
		{"bad api key", "ctoken_auth", payment.ErrConfiguration},
		{"expired token", "ctoken_expired", payment.ErrInvalidToken},
		{"unknown token", "ctoken_nonsense", payment.ErrInvalidToken},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			res, err := f.p.Checkout(ctx(), checkoutRequest(tc.token))
			if !errors.Is(err, tc.is) {
				t.Fatalf("want %v, got %+v %v", tc.is, res, err)
			}
			for _, e := range f.logs.All() {
				if strings.Contains(e.Message+" "+toString(e.ContextMap()), testSecretKey) {
					t.Fatalf("the API key must never be logged: %+v", e)
				}
			}
		})
	}
}

func toString(m map[string]any) string {
	var b strings.Builder
	for k, v := range m {
		b.WriteString(k)
		b.WriteString("=")
		if s, ok := v.(string); ok {
			b.WriteString(s)
		}
		b.WriteString(" ")
	}
	return b.String()
}

func TestAConfigurationErrorIsLoggedWithoutTheKey(t *testing.T) {
	f := newFixture(t)
	if _, err := f.p.Checkout(ctx(), checkoutRequest("ctoken_auth")); !errors.Is(err, payment.ErrConfiguration) {
		t.Fatal(err)
	}
	logged := false
	for _, e := range f.logs.All() {
		if e.Level.String() == "error" && strings.Contains(e.Message, "configuration error") {
			logged = true
		}
	}
	if !logged {
		t.Fatal("a rejected API key must be logged as a configuration error")
	}
	// A key the fake rejects at the transport level, the way Stripe does.
	bad := newFixture(t, func(o *stripepayOptions) { o.SecretKey = "sk_bad_0123456789abcdef" })
	if _, err := bad.p.Checkout(ctx(), checkoutRequest("ctoken_ok_visa_4242")); !errors.Is(err, payment.ErrConfiguration) {
		t.Fatalf("revoked key: %v", err)
	}
}

func TestALostResponseIsReconciledWithLookupAndReplayedWithTheSameKey(t *testing.T) {
	f := newFixture(t)
	req := checkoutRequest("ctoken_timeout_visa_4242")
	if _, err := f.p.Checkout(ctx(), req); !errors.Is(err, payment.ErrUnknownOutcome) {
		t.Fatalf("lost response: %v", err)
	}
	// The money moved: one succeeded PaymentIntent exists with our key.
	pis := f.fake.Intents()
	if len(pis) != 1 || pis[0].Status != "succeeded" || pis[0].Metadata["sy_key"] != req.IdempotencyKey {
		t.Fatalf("intents: %+v", pis)
	}
	// Stripe's search is eventually consistent: not yet, then found.
	f.fake.SetSearchLag(true)
	if res, err := f.p.Lookup(ctx(), req.IdempotencyKey); err != nil || res.Found {
		t.Fatalf("lookup during the search lag: %+v %v", res, err)
	}
	f.fake.SetSearchLag(false)
	res, err := f.p.Lookup(ctx(), req.IdempotencyKey)
	if err != nil || !res.Found || res.Decline != nil || res.Result.Status != payment.ChargeSucceeded || res.Result.ProviderRef != pis[0].ID {
		t.Fatalf("lookup: %+v %v", res, err)
	}
	if res.Result.Card == nil || res.Result.Card.Last4 != "4242" || res.Result.Risk == nil {
		t.Fatalf("lookup details: %+v", res.Result)
	}
	search := f.fake.RequestsTo("GET", "/v1/payment_intents/search")
	if q := search[len(search)-1].Form.Get("query"); q != "metadata['sy_key']:'"+req.IdempotencyKey+"'" {
		t.Fatalf("search query: %q", q)
	}
	// Replaying the charge with the same key returns the original PaymentIntent, never a second charge.
	replay, err := f.p.Checkout(ctx(), req)
	if err != nil || replay.Status != payment.CheckoutSucceeded || replay.ProviderRef != pis[0].ID {
		t.Fatalf("replay must return the original payment: %+v %v", replay, err)
	}
	if n := len(f.fake.Intents()); n != 1 {
		t.Fatalf("a replayed key must not charge twice: %d payment intents", n)
	}
}

func TestLookupFindsDeclinesAndPendingPaymentsAndNothingForUnknownKeys(t *testing.T) {
	f := newFixture(t)
	declined := checkoutRequest("ctoken_decline_insufficient_funds")
	if _, err := f.p.Checkout(ctx(), declined); err != nil {
		t.Fatal(err)
	}
	res, err := f.p.Lookup(ctx(), declined.IdempotencyKey)
	if err != nil || !res.Found || res.Decline == nil || res.Decline.Code != "insufficient_funds" || !res.Decline.Soft {
		t.Fatalf("declined lookup: %+v %v", res, err)
	}
	pending := checkoutRequest("ctoken_3ds_visa_3155")
	if _, err := f.p.Checkout(ctx(), pending); err != nil {
		t.Fatal(err)
	}
	res, err = f.p.Lookup(ctx(), pending.IdempotencyKey)
	if err != nil || !res.Found || res.Result.Status != payment.ChargeRequiresAction || res.Result.NextAction == nil {
		t.Fatalf("3-D Secure lookup: %+v %v", res, err)
	}
	if res, err = f.p.Lookup(ctx(), "purchase:unknown"); err != nil || res.Found {
		t.Fatalf("unknown key: %+v %v", res, err)
	}
	if res, err = f.p.Lookup(ctx(), ""); err != nil || res.Found {
		t.Fatalf("empty key: %+v %v", res, err)
	}
	// A key with quotes cannot break out of the search query.
	if _, err = f.p.Lookup(ctx(), `x'\ OR metadata['sy_key']:'y`); err != nil {
		t.Fatal(err)
	}
	q := f.fake.RequestsTo("GET", "/v1/payment_intents/search")
	if got := q[len(q)-1].Form.Get("query"); got != `metadata['sy_key']:'x\'\\ OR metadata[\'sy_key\']:\'y'` {
		t.Fatalf("escaped query: %q", got)
	}
	f.fake.Fail("GET", "/v1/payment_intents/search", 500, 1, "")
	if _, err = f.p.Lookup(ctx(), "purchase:x"); !errors.Is(err, payment.ErrUnknownOutcome) {
		t.Fatalf("search outage: %v", err)
	}
}

func TestThreeDSecureNextActionAndRetrieveCharge(t *testing.T) {
	f := newFixture(t)
	req := checkoutRequest("ctoken_3ds_visa_3155")
	req.ThreeDSecure = payment.ThreeDSAutomatic
	res, err := f.p.Checkout(ctx(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != payment.CheckoutPending || res.NextAction == nil || res.NextAction.Type != "use_stripe_sdk" ||
		!strings.HasPrefix(res.NextAction.ClientSecret, res.ProviderRef+"_secret_") {
		t.Fatalf("3-D Secure result: %+v", res)
	}
	if got := f.lastPost("/v1/payment_intents").Form.Get("payment_method_options[card][request_three_d_secure]"); got != "automatic" {
		t.Fatalf("3-D Secure policy: %q", got)
	}
	// While the customer is still authenticating, the charge needs action.
	cur, decline, err := f.p.RetrieveCharge(ctx(), res.ProviderRef)
	if err != nil || decline != nil || cur.Status != payment.ChargeRequiresAction || cur.NextAction == nil {
		t.Fatalf("retrieve while pending: %+v %+v %v", cur, decline, err)
	}
	f.fake.CompleteAction(res.ProviderRef, true)
	cur, decline, err = f.p.RetrieveCharge(ctx(), res.ProviderRef)
	if err != nil || decline != nil || cur.Status != payment.ChargeSucceeded || cur.ThreeDS != "authenticated" || cur.Card == nil || cur.Card.Last4 != "3155" {
		t.Fatalf("retrieve after 3-D Secure: %+v %+v %v", cur, decline, err)
	}
	if r := f.fake.RequestsTo("GET", "/v1/payment_intents/"+res.ProviderRef); !hasValue(r[len(r)-1].Form, "latest_charge") {
		t.Fatal("RetrieveCharge must expand latest_charge")
	}

	// A failed authentication is a decline, not an error.
	failed, err := f.p.Checkout(ctx(), checkoutRequest("ctoken_3ds_visa_3155_DE")) // tokens are single use
	if err != nil {
		t.Fatal(err)
	}
	f.fake.CompleteAction(failed.ProviderRef, false)
	_, decline, err = f.p.RetrieveCharge(ctx(), failed.ProviderRef)
	if err != nil || decline == nil || decline.Code != "authentication_failed" || decline.Soft {
		t.Fatalf("failed 3-D Secure: %+v %v", decline, err)
	}
	if _, _, err = f.p.RetrieveCharge(ctx(), "pi_missing"); !errors.Is(err, payment.ErrNotFound) {
		t.Fatalf("unknown payment: %v", err)
	}
	if _, _, err = f.p.RetrieveCharge(ctx(), "ch_123"); !errors.Is(err, payment.ErrNotFound) {
		t.Fatalf("a charge id is not a payment reference: %v", err)
	}
}

func TestRiskAndNetworkFieldsComeFromTheExpandedCharge(t *testing.T) {
	f := newFixture(t)
	id := f.paid(1500)
	f.fake.SetRisk(id, "elevated", 72)
	res, decline, err := f.p.RetrieveCharge(ctx(), id)
	if err != nil || decline != nil || res.Risk == nil || res.Risk.Level != payment.RiskElevated || res.Risk.Score != 72 {
		t.Fatalf("risk: %+v %+v %v", res.Risk, decline, err)
	}
}

func TestChargeStoredIsCustomerInitiatedOrOffSession(t *testing.T) {
	f := newFixture(t)
	cus := f.customer()
	pm := f.fake.AddPaymentMethod(cus, "ok_mastercard_4444")
	charge := func(reason payment.ChargeReason) (payment.ChargeResult, error) {
		return f.p.ChargeStored(ctx(), payment.StoredCharge{CustomerRef: cus, MethodRef: pm, AmountCents: 999, Currency: "USD",
			IdempotencyKey: "sub:" + uuid.NewString(), Reason: reason, Description: "Premium", ReturnURL: "https://staryield.net/en/credits/return",
			Metadata: map[string]string{"sy_tx_id": "t1"}, ThreeDSecure: payment.ThreeDSAny, StatementDescriptor: "IGNORED"})
	}
	res, err := charge(payment.ReasonCustomerInitiated)
	if err != nil || res.Status != payment.ChargeSucceeded || res.Card == nil || res.Card.Brand != "mastercard" {
		t.Fatalf("customer-initiated: %+v %v", res, err)
	}
	r := f.lastPost("/v1/payment_intents")
	for k, want := range map[string]string{"customer": cus, "payment_method": pm, "confirm": "true", "off_session": "false", "currency": "usd",
		"metadata[sy_tx_id]": "t1", "payment_method_options[card][request_three_d_secure]": "any"} {
		if got := r.Form.Get(k); got != want {
			t.Errorf("form %s = %q, want %q", k, got, want)
		}
	}
	if r.Form.Get("metadata[sy_key]") != r.IdempotencyKey() {
		t.Error("sy_key must equal the idempotency key")
	}
	for _, reason := range []payment.ChargeReason{payment.ReasonRecurring, payment.ReasonUnscheduled} {
		if _, err := charge(reason); err != nil {
			t.Fatal(err)
		}
		if got := f.lastPost("/v1/payment_intents").Form.Get("off_session"); got != "true" {
			t.Errorf("%s must be off session: %q", reason, got)
		}
	}
	// A saved card that needs 3-D Secure.
	pm3 := f.fake.AddPaymentMethod(cus, "3ds_visa_3155")
	res, err = f.p.ChargeStored(ctx(), payment.StoredCharge{CustomerRef: cus, MethodRef: pm3, AmountCents: 999, Currency: "USD",
		IdempotencyKey: "sub:" + uuid.NewString(), Reason: payment.ReasonCustomerInitiated})
	if err != nil || res.Status != payment.ChargeRequiresAction || res.NextAction == nil || res.NextAction.ClientSecret == "" {
		t.Fatalf("3-D Secure saved card: %+v %v", res, err)
	}
	// Another customer's card and missing parameters are refused.
	other := f.customer()
	if _, err = f.p.ChargeStored(ctx(), payment.StoredCharge{CustomerRef: other, MethodRef: pm, AmountCents: 999, Currency: "USD",
		IdempotencyKey: "k"}); err == nil {
		t.Fatal("charging another customer's card must fail")
	}
	if _, err = f.p.ChargeStored(ctx(), payment.StoredCharge{AmountCents: 999, Currency: "USD"}); err == nil {
		t.Fatal("a charge without customer and card must fail")
	}
}

func TestChargeStoredReplaysTheSameKeyWithoutChargingTwice(t *testing.T) {
	f := newFixture(t)
	cus := f.customer()
	pm := f.fake.AddPaymentMethod(cus, "ok_visa_4242")
	req := payment.StoredCharge{CustomerRef: cus, MethodRef: pm, AmountCents: 999, Currency: "USD", IdempotencyKey: "sub:one", Reason: payment.ReasonRecurring}
	first, err := f.p.ChargeStored(ctx(), req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.p.ChargeStored(ctx(), req)
	if err != nil || second.ProviderRef != first.ProviderRef || len(f.fake.Intents()) != 1 {
		t.Fatalf("replay: %+v %v (%d intents)", second, err, len(f.fake.Intents()))
	}
	req.AmountCents = 1999 // same key, other parameters: Stripe refuses
	if _, err = f.p.ChargeStored(ctx(), req); err == nil || errors.Is(err, payment.ErrUnknownOutcome) {
		t.Fatalf("reusing a key with other parameters: %v", err)
	}
}

func TestContextCancellationIsAnUnknownOutcome(t *testing.T) {
	f := newFixture(t)
	c, cancel := context.WithCancel(ctx())
	cancel()
	if _, err := f.p.Checkout(c, checkoutRequest("ctoken_ok_visa_4242")); !errors.Is(err, payment.ErrUnknownOutcome) {
		t.Fatalf("cancelled context: %v", err)
	}
}

func TestALostResponseThatStaysLostIsOnlyFoundByLookup(t *testing.T) {
	f := newFixture(t)
	req := checkoutRequest("ctoken_lost_visa_4242")
	for i := 0; i < 2; i++ { // the original and its retry with the same key
		if _, err := f.p.Checkout(ctx(), req); !errors.Is(err, payment.ErrUnknownOutcome) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	if n := len(f.fake.Intents()); n != 1 {
		t.Fatalf("one payment, however often the key is replayed: %d", n)
	}
	res, err := f.p.Lookup(ctx(), req.IdempotencyKey)
	if err != nil || !res.Found || res.Result.Status != payment.ChargeSucceeded {
		t.Fatalf("lookup: %+v %v", res, err)
	}
}

func TestTheSDKRetriesATransientFailureWithTheSameKey(t *testing.T) {
	one := int64(1)
	f := newFixture(t, func(o *stripepayOptions) { o.MaxNetworkRetries = &one })
	f.fake.Drop("POST", "/v1/payment_intents", 1) // the first attempt never reaches the processor
	req := checkoutRequest("ctoken_ok_visa_4242")
	res, err := f.p.Checkout(ctx(), req)
	if err != nil || res.Status != payment.CheckoutSucceeded {
		t.Fatalf("retried checkout: %+v %v", res, err)
	}
	posts := f.fake.RequestsTo("POST", "/v1/payment_intents")
	if len(posts) != 2 || posts[0].IdempotencyKey() != req.IdempotencyKey || posts[1].IdempotencyKey() != req.IdempotencyKey || len(f.fake.Intents()) != 1 {
		t.Fatalf("%d attempts, %d payments", len(posts), len(f.fake.Intents()))
	}
}
