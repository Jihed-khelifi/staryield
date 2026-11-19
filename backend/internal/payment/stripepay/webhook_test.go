package stripepay_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment/stripepay/stripetest"
)

func (f *fixture) verify(payload []byte, header string) (*payment.WebhookEvent, error) {
	h := http.Header{}
	if header != "" {
		h.Set("Stripe-Signature", header)
	}
	return f.p.VerifyWebhookRequest(payment.WebhookRequest{Header: h, Body: payload, Now: f.now})
}

func (f *fixture) signed(payload []byte) (*payment.WebhookEvent, error) {
	return f.verify(payload, stripetest.Sign(testWebhookSecret, payload, f.now))
}

func TestWebhookSignatureIsVerified(t *testing.T) {
	f := newFixture(t)
	payload := stripetest.PaymentIntentSucceeded("evt_1", "pi_1", 2500, "usd", map[string]string{"sy_key": "purchase:1"})
	good := stripetest.Sign(testWebhookSecret, payload, f.now)
	if evt, err := f.verify(payload, good); err != nil || evt.ID != "evt_1" {
		t.Fatalf("valid signature: %+v %v", evt, err)
	}
	// A second v1 signature (secret rotation) is fine as long as one matches.
	rotated := good + ",v1=" + strings.Repeat("0", 64)
	if _, err := f.verify(payload, rotated); err != nil {
		t.Fatalf("rotation: %v", err)
	}
	cases := map[string]struct {
		payload []byte
		header  string
		want    error
	}{
		"no header":       {payload, "", payment.ErrBadSignature},
		"junk header":     {payload, "nonsense", payment.ErrBadSignature},
		"no signature":    {payload, "t=" + good[2:12], payment.ErrBadSignature},
		"wrong secret":    {payload, stripetest.Sign("whsec_other_secret_0123456789", payload, f.now), payment.ErrBadSignature},
		"tampered body":   {[]byte(strings.Replace(string(payload), "2500", "2501", 1)), good, payment.ErrBadSignature},
		"empty body":      {nil, stripetest.Sign(testWebhookSecret, nil, f.now), nil},
		"stale":           {payload, stripetest.Sign(testWebhookSecret, payload, f.now.Add(-6*time.Minute)), payment.ErrStale},
		"from the future": {payload, stripetest.Sign(testWebhookSecret, payload, f.now.Add(6*time.Minute)), payment.ErrStale},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := f.verify(tc.payload, tc.header)
			if tc.want == nil {
				if err == nil {
					t.Fatal("an empty body is not an event")
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
	// Inside the tolerance on both sides.
	for _, d := range []time.Duration{-4 * time.Minute, 4 * time.Minute} {
		if _, err := f.verify(payload, stripetest.Sign(testWebhookSecret, payload, f.now.Add(d))); err != nil {
			t.Errorf("offset %s: %v", d, err)
		}
	}
	// The verifier uses the request's clock, not the machine's.
	old := f.now.Add(-30 * 24 * time.Hour)
	if _, err := f.p.VerifyWebhookRequest(payment.WebhookRequest{Header: http.Header{"Stripe-Signature": {stripetest.Sign(testWebhookSecret, payload, old)}},
		Body: payload, Now: old}); err != nil {
		t.Fatalf("injected clock: %v", err)
	}
	if len(f.fake.Requests()) != 0 {
		t.Fatal("verification must never call the Stripe API")
	}
}

func TestWebhookWithAnotherAPIVersionIsAcceptedAndLoggedOnce(t *testing.T) {
	f := newFixture(t)
	payload := stripetest.UnknownEvent("evt_v", "something.else")
	old := []byte(strings.Replace(string(payload), stripetest.APIVersion, "2020-08-27", 1))
	for i := 0; i < 3; i++ {
		if evt, err := f.signed(old); err != nil || evt.Type != payment.EventIgnored {
			t.Fatalf("old API version: %+v %v", evt, err)
		}
	}
	warnings := 0
	for _, e := range f.logs.All() {
		if e.Level.String() == "warn" && strings.Contains(e.Message, "API version") {
			warnings++
			if !strings.Contains(e.ContextMap()["event_version"].(string), "2020-08-27") {
				t.Fatalf("log fields: %v", e.ContextMap())
			}
		}
	}
	if warnings != 1 {
		t.Fatalf("a version mismatch is logged once, got %d", warnings)
	}
	if _, err := f.signed(payload); err != nil {
		t.Fatal(err)
	}
}

func TestWebhookEventMapping(t *testing.T) {
	f := newFixture(t)
	due := time.Date(2026, 12, 3, 10, 0, 0, 0, time.UTC)
	meta := map[string]string{"sy_key": "purchase:abc", "sy_purchase_id": "11111111-1111-1111-1111-111111111111"}
	dispute := func(status string) stripetest.DisputeObject {
		return stripetest.DisputeObject{ID: "dp_9", PaymentIntent: "pi_9", Amount: 2500, Currency: "usd", Reason: "fraudulent", Status: status, EvidenceDueBy: due}
	}
	cases := []struct {
		name    string
		payload []byte
		check   func(t *testing.T, e *payment.WebhookEvent)
		want    string // normalised type
	}{
		{"payment_intent.succeeded", stripetest.PaymentIntentSucceeded("evt_a", "pi_9", 2500, "usd", meta), func(t *testing.T, e *payment.WebhookEvent) {
			if e.ProviderRef != "pi_9" || e.ObjectRef != "pi_9" || e.IdempotencyKey != "purchase:abc" || e.PurchaseID != meta["sy_purchase_id"] ||
				e.AmountCents != 2500 || e.Currency != "USD" || e.Livemode || e.Card != nil || e.Risk != nil {
				t.Fatalf("%+v", e)
			}
		}, payment.EventChargeSucceeded},
		{"payment_intent.payment_failed", stripetest.PaymentIntentFailed("evt_b", "pi_9", 2500, "usd", meta, "insufficient_funds"), func(t *testing.T, e *payment.WebhookEvent) {
			if e.ProviderRef != "pi_9" || e.FailureCode != "insufficient_funds" || e.IdempotencyKey != "purchase:abc" {
				t.Fatalf("%+v", e)
			}
		}, payment.EventChargeFailed},
		{"payment_intent.payment_failed generic", stripetest.PaymentIntentFailed("evt_b2", "pi_9", 2500, "usd", meta, "generic_decline"), func(t *testing.T, e *payment.WebhookEvent) {
			if e.FailureCode != "card_declined" {
				t.Fatalf("%+v", e)
			}
		}, payment.EventChargeFailed},
		{"payment_intent.canceled", stripetest.PaymentIntentCanceled("evt_c", "pi_9", 2500, "usd", meta), func(t *testing.T, e *payment.WebhookEvent) {
			if e.ProviderRef != "pi_9" || e.FailureCode != "canceled" {
				t.Fatalf("%+v", e)
			}
		}, payment.EventChargeFailed},
		{"payment_intent.processing", stripetest.PaymentIntentProcessing("evt_d", "pi_9", 2500, "usd", meta), func(t *testing.T, e *payment.WebhookEvent) {
			if e.ProviderRef != "pi_9" {
				t.Fatalf("%+v", e)
			}
		}, payment.EventChargeProcessing},
		{"refund.created pending", stripetest.RefundEvent("evt_e", "refund.created", "re_1", "pi_9", 1000, "usd", "pending"), refundCheck("pending"), payment.EventChargeRefunded},
		{"refund.created succeeded", stripetest.RefundEvent("evt_e2", "refund.created", "re_1", "pi_9", 1000, "usd", "succeeded"), refundCheck("succeeded"), payment.EventChargeRefunded},
		{"refund.updated succeeded", stripetest.RefundEvent("evt_f", "refund.updated", "re_1", "pi_9", 1000, "usd", "succeeded"), refundCheck("succeeded"), payment.EventChargeRefunded},
		{"refund.updated pending", stripetest.RefundEvent("evt_f2", "refund.updated", "re_1", "pi_9", 1000, "usd", "pending"), refundCheck("pending"), payment.EventChargeRefunded},
		{"refund.updated failed", stripetest.RefundEvent("evt_g", "refund.updated", "re_1", "pi_9", 1000, "usd", "failed"), refundCheck("failed"), payment.EventRefundFailed},
		{"refund.updated canceled", stripetest.RefundEvent("evt_g2", "refund.updated", "re_1", "pi_9", 1000, "usd", "canceled"), refundCheck("canceled"), payment.EventRefundFailed},
		{"refund.failed", stripetest.RefundEvent("evt_h", "refund.failed", "re_1", "pi_9", 1000, "usd", "failed"), refundCheck("failed"), payment.EventRefundFailed},
		{"refund.updated requires_action", stripetest.RefundEvent("evt_h2", "refund.updated", "re_1", "pi_9", 1000, "usd", "requires_action"), nil, payment.EventIgnored},
		{"charge.dispute.created", stripetest.DisputeEvent("evt_i", "charge.dispute.created", dispute("needs_response")), disputeCheck("needs_response", due), payment.EventChargeDisputed},
		{"charge.dispute.created inquiry", stripetest.DisputeEvent("evt_i2", "charge.dispute.created", dispute("warning_needs_response")), disputeCheck("warning_needs_response", due), payment.EventChargeDisputed},
		{"charge.dispute.updated", stripetest.DisputeEvent("evt_j", "charge.dispute.updated", dispute("under_review")), disputeCheck("under_review", due), payment.EventDisputeUpdated},
		{"charge.dispute.closed won", stripetest.DisputeEvent("evt_k", "charge.dispute.closed", dispute("won")), disputeCheck("won", due), payment.EventDisputeClosed},
		{"charge.dispute.closed lost", stripetest.DisputeEvent("evt_k2", "charge.dispute.closed", dispute("lost")), disputeCheck("lost", due), payment.EventDisputeClosed},
		{"charge.dispute.closed warning_closed", stripetest.DisputeEvent("evt_k3", "charge.dispute.closed", dispute("warning_closed")), disputeCheck("warning_closed", due), payment.EventDisputeClosed},
		{"charge.dispute.funds_withdrawn", stripetest.DisputeEvent("evt_l", "charge.dispute.funds_withdrawn", dispute("needs_response")), disputeCheck("needs_response", due), payment.EventDisputeFundsWithdrawn},
		{"charge.dispute.funds_reinstated", stripetest.DisputeEvent("evt_m", "charge.dispute.funds_reinstated", dispute("won")), disputeCheck("won", due), payment.EventDisputeFundsReinstated},
		{"radar.early_fraud_warning.created", stripetest.FraudWarningEvent("evt_n", "radar.early_fraud_warning.created", "issfr_1", "pi_9", "made_with_stolen_card", true), func(t *testing.T, e *payment.WebhookEvent) {
			if e.ObjectRef != "issfr_1" || e.ProviderRef != "pi_9" || !e.Actionable || e.FraudType != "made_with_stolen_card" {
				t.Fatalf("%+v", e)
			}
		}, payment.EventFraudWarning},
		{"radar.early_fraud_warning.updated", stripetest.FraudWarningEvent("evt_o", "radar.early_fraud_warning.updated", "issfr_1", "pi_9", "fraudulent", false), func(t *testing.T, e *payment.WebhookEvent) {
			if e.Actionable || e.FraudType != "fraudulent" || e.ProviderRef != "pi_9" {
				t.Fatalf("%+v", e)
			}
		}, payment.EventFraudWarning},
		{"payment_method.detached", stripetest.PaymentMethodDetached("evt_p", "pm_5"), func(t *testing.T, e *payment.WebhookEvent) {
			if e.ObjectRef != "pm_5" {
				t.Fatalf("%+v", e)
			}
		}, payment.EventMethodDetached},
		{"customer.deleted", stripetest.CustomerDeleted("evt_q", "cus_5"), func(t *testing.T, e *payment.WebhookEvent) {
			if e.ObjectRef != "cus_5" {
				t.Fatalf("%+v", e)
			}
		}, payment.EventCustomerDeleted},
		{"review.opened", stripetest.ReviewEvent("evt_r", "review.opened", "prv_1", "pi_9", "rule"), func(t *testing.T, e *payment.WebhookEvent) {
			if e.ObjectRef != "prv_1" || e.ProviderRef != "pi_9" {
				t.Fatalf("%+v", e)
			}
		}, payment.EventReviewOpened},
		{"review.closed", stripetest.ReviewEvent("evt_s", "review.closed", "prv_1", "pi_9", "approved"), func(t *testing.T, e *payment.WebhookEvent) {
			if e.ObjectRef != "prv_1" || e.ProviderRef != "pi_9" {
				t.Fatalf("%+v", e)
			}
		}, payment.EventReviewClosed},
		{"anything else", stripetest.UnknownEvent("evt_t", "invoice.paid"), nil, payment.EventIgnored},
	}
	seen := map[string]bool{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, err := f.signed(tc.payload)
			if err != nil {
				t.Fatal(err)
			}
			if e.Type != tc.want || e.ProviderType == "" || e.ID == "" {
				t.Fatalf("type %q (provider type %q, id %q), want %q", e.Type, e.ProviderType, e.ID, tc.want)
			}
			seen[e.ProviderType] = true
			if tc.check != nil {
				tc.check(t, e)
			}
		})
	}
	// Every Stripe event of the contract table is covered.
	for _, name := range []string{"payment_intent.succeeded", "payment_intent.payment_failed", "payment_intent.canceled", "payment_intent.processing",
		"refund.created", "refund.updated", "refund.failed", "charge.dispute.created", "charge.dispute.updated", "charge.dispute.closed",
		"charge.dispute.funds_withdrawn", "charge.dispute.funds_reinstated", "radar.early_fraud_warning.created", "radar.early_fraud_warning.updated",
		"payment_method.detached", "customer.deleted", "review.opened", "review.closed"} {
		if !seen[name] {
			t.Errorf("no mapping test for %s", name)
		}
	}
}

func refundCheck(status string) func(*testing.T, *payment.WebhookEvent) {
	return func(t *testing.T, e *payment.WebhookEvent) {
		t.Helper()
		if e.ProviderRef != "pi_9" || e.RefundRef != "re_1" || e.ObjectRef != "re_1" || e.AmountCents != 1000 || e.RefundStatus != status ||
			e.Currency != "USD" || e.IdempotencyKey != "" {
			t.Fatalf("%+v", e)
		}
	}
}

func disputeCheck(status string, due time.Time) func(*testing.T, *payment.WebhookEvent) {
	return func(t *testing.T, e *payment.WebhookEvent) {
		t.Helper()
		if e.ProviderRef != "pi_9" || e.DisputeRef != "dp_9" || e.ObjectRef != "dp_9" || e.DisputeStatus != status || e.DisputeReason != "fraudulent" ||
			e.AmountCents != 2500 || e.Currency != "USD" || e.EvidenceDueBy == nil || !e.EvidenceDueBy.Equal(due) {
			t.Fatalf("%+v", e)
		}
	}
}

func TestWebhookObjectsAreDecodedLeniently(t *testing.T) {
	f := newFixture(t)
	// Expanded references, unknown fields and nulls do not break decoding.
	payload := stripetest.Build("evt_len", "charge.dispute.created", map[string]any{"id": "dp_1", "object": "dispute", "amount": 100,
		"currency": "eur", "reason": "general", "status": "needs_response", "brand_new_field": map[string]any{"x": 1},
		"payment_intent": map[string]any{"id": "pi_expanded", "object": "payment_intent"}, "evidence_details": nil, "metadata": map[string]any{}})
	e, err := f.signed(payload)
	if err != nil || e.ProviderRef != "pi_expanded" || e.EvidenceDueBy != nil || e.Currency != "EUR" {
		t.Fatalf("%+v %v", e, err)
	}
	// A dispute without a payment intent still decodes (the reference stays empty).
	payload = stripetest.Build("evt_len2", "charge.dispute.created", map[string]any{"id": "dp_2", "payment_intent": nil, "status": "needs_response"})
	if e, err = f.signed(payload); err != nil || e.ProviderRef != "" || e.DisputeRef != "dp_2" {
		t.Fatalf("%+v %v", e, err)
	}
	// A verified but malformed event is an error (Stripe retries), never silently dropped.
	if _, err = f.signed([]byte(`{"id":"evt_bad","object":"event","type":"x","data":{"object":"nope"}}`)); err == nil {
		t.Fatal("a malformed object must fail loudly")
	}
	if _, err = f.signed([]byte(`{"id":"evt_thin","object":"event_notification"}`)); err == nil {
		t.Fatal("a thin event is not supported")
	}
}

func TestFetchEventNormalisesLikeAWebhook(t *testing.T) {
	f := newFixture(t)
	payload := stripetest.DisputeEvent("evt_fetch", "charge.dispute.closed", stripetest.DisputeObject{ID: "dp_7", PaymentIntent: "pi_7", Amount: 900,
		Currency: "usd", Reason: "fraudulent", Status: "won"})
	f.fake.AddEvent(payload)
	fetched, err := f.p.FetchEvent(ctx(), "evt_fetch")
	if err != nil {
		t.Fatal(err)
	}
	hooked, err := f.signed(payload)
	if err != nil {
		t.Fatal(err)
	}
	if *fetched != *hooked && (fetched.Type != hooked.Type || fetched.ProviderRef != hooked.ProviderRef || fetched.DisputeStatus != hooked.DisputeStatus ||
		fetched.DisputeRef != hooked.DisputeRef || fetched.AmountCents != hooked.AmountCents) {
		t.Fatalf("fetched %+v, webhook %+v", fetched, hooked)
	}
	if fetched.Type != payment.EventDisputeClosed || fetched.DisputeStatus != "won" {
		t.Fatalf("%+v", fetched)
	}
	if _, err = f.p.FetchEvent(ctx(), "evt_missing"); !errors.Is(err, payment.ErrNotFound) {
		t.Fatalf("unknown event: %v", err)
	}
	if _, err = f.p.FetchEvent(ctx(), "pi_1"); !errors.Is(err, payment.ErrNotFound) {
		t.Fatalf("not an event id: %v", err)
	}
	f.fake.Fail("GET", "/v1/events/", 500, 1, "")
	if _, err = f.p.FetchEvent(ctx(), "evt_fetch"); !errors.Is(err, payment.ErrUnknownOutcome) {
		t.Fatalf("outage: %v", err)
	}
}
