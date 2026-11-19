package stripepay_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment/stripepay/stripetest"
)

func TestRefundSendsTheContractAndMapsErrors(t *testing.T) {
	f := newFixture(t)
	pi := f.paid(2500)
	res, err := f.p.Refund(ctx(), payment.RefundRequest{ProviderRef: pi, AmountCents: 1000, Currency: "USD",
		IdempotencyKey: "refund:tx:1", Reason: "Technical problem during the reading"})
	if err != nil || !strings.HasPrefix(res.ProviderRef, "re_") {
		t.Fatalf("refund: %+v %v", res, err)
	}
	r := f.lastPost("/v1/refunds")
	for k, want := range map[string]string{"payment_intent": pi, "amount": "1000", "reason": "requested_by_customer",
		"metadata[sy_refund_key]": "refund:tx:1", "metadata[sy_reason]": "Technical problem during the reading"} {
		if got := r.Form.Get(k); got != want {
			t.Errorf("refund form %s = %q, want %q", k, got, want)
		}
	}
	if r.IdempotencyKey() != "refund:tx:1" {
		t.Fatalf("Idempotency-Key: %q", r.IdempotencyKey())
	}
	// The same key replays the same refund.
	again, err := f.p.Refund(ctx(), payment.RefundRequest{ProviderRef: pi, AmountCents: 1000, IdempotencyKey: "refund:tx:1", Reason: "Technical problem during the reading"})
	if err != nil || again.ProviderRef != res.ProviderRef || len(f.fake.Refunds()) != 1 {
		t.Fatalf("replay: %+v %v (%d refunds)", again, err, len(f.fake.Refunds()))
	}
	// Reason enum: fraud warnings refund as fraudulent.
	if _, err = f.p.Refund(ctx(), payment.RefundRequest{ProviderRef: pi, AmountCents: 100, IdempotencyKey: "refund:tx:2", Reason: "fraudulent"}); err != nil {
		t.Fatal(err)
	}
	if got := f.lastPost("/v1/refunds").Form.Get("reason"); got != "fraudulent" {
		t.Fatalf("reason: %q", got)
	}
	// More than what is left, then exactly the rest, then nothing is left.
	if _, err = f.p.Refund(ctx(), payment.RefundRequest{ProviderRef: pi, AmountCents: 5000, IdempotencyKey: "refund:tx:3"}); !errors.Is(err, payment.ErrRefundTooLarge) {
		t.Fatalf("too large: %v", err)
	}
	if _, err = f.p.Refund(ctx(), payment.RefundRequest{ProviderRef: pi, AmountCents: 1400, IdempotencyKey: "refund:tx:4"}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.p.Refund(ctx(), payment.RefundRequest{ProviderRef: pi, AmountCents: 1, IdempotencyKey: "refund:tx:5"}); !errors.Is(err, payment.ErrRefundTooLarge) {
		t.Fatalf("fully refunded: %v", err)
	}
	for _, bad := range []payment.RefundRequest{{ProviderRef: "pi_missing", AmountCents: 1}, {ProviderRef: "ch_1", AmountCents: 1}} {
		if _, err = f.p.Refund(ctx(), bad); !errors.Is(err, payment.ErrNotFound) {
			t.Errorf("refund of %q: %v", bad.ProviderRef, err)
		}
	}
	if _, err = f.p.Refund(ctx(), payment.RefundRequest{ProviderRef: pi, AmountCents: 0}); !errors.Is(err, payment.ErrRefundTooLarge) {
		t.Fatalf("zero amount: %v", err)
	}
}

func TestRefundOfADisputedChargeIsRefused(t *testing.T) {
	f := newFixture(t)
	pi := f.paid(2500)
	f.fake.OpenDispute(pi, "fraudulent")
	if _, err := f.p.Refund(ctx(), payment.RefundRequest{ProviderRef: pi, AmountCents: 2500, IdempotencyKey: "refund:d"}); !errors.Is(err, payment.ErrChargeDisputed) {
		t.Fatalf("disputed refund: %v", err)
	}
	if len(f.fake.Refunds()) != 0 {
		t.Fatal("no refund may be issued on a disputed charge")
	}
	f.fake.Fail("POST", "/v1/refunds", 503, 1, "")
	if _, err := f.p.Refund(ctx(), payment.RefundRequest{ProviderRef: pi, AmountCents: 1, IdempotencyKey: "refund:x"}); !errors.Is(err, payment.ErrUnknownOutcome) {
		t.Fatalf("outage: %v", err)
	}
}

func TestDisputeEvidenceUploadSubmitAndAccept(t *testing.T) {
	f := newFixture(t)
	pi := f.paid(2500)
	dp := f.fake.OpenDispute(pi, "product_not_received")
	pdf := []byte("%PDF-1.4 invoice")
	ev := payment.DisputeEvidence{ProductDescription: "Digital credits for live psychic chat, for entertainment purposes only",
		CustomerName: "Ada Lovelace", CustomerEmail: "ada@example.com", BillingCountry: "FR", ServiceDate: "2026-11-19",
		AccessActivityLog: "2026-11-19 reading with Mira, 5 min, 25 credits", RefundPolicyDisclosure: "Accepted at checkout: https://staryield.net/en/refund-policy",
		CancellationRebuttal: "n/a", UncategorizedText: strings.Repeat("é", 25000), ReceiptPDF: pdf, ReceiptFilename: "SY-2026-000001.pdf"}
	// Saving a draft does not submit.
	if err := f.p.SubmitDisputeEvidence(ctx(), dp, ev, false); err != nil {
		t.Fatal(err)
	}
	up := f.fake.Uploads()
	if len(up) != 1 || up[0].Purpose != "dispute_evidence" || up[0].Filename != "SY-2026-000001.pdf" || string(up[0].Content) != string(pdf) {
		t.Fatalf("upload: %+v", up)
	}
	d, _ := f.fake.DisputeByID(dp)
	if d.Submitted || d.Evidence["customer_name"] != "Ada Lovelace" || d.Evidence["customer_email_address"] != "ada@example.com" ||
		d.Evidence["service_date"] != "2026-11-19" || d.Evidence["receipt"] == "" || d.Evidence["product_description"] == "" ||
		d.Evidence["billing_address"] != "FR" || d.Evidence["access_activity_log"] == "" {
		t.Fatalf("evidence: %+v", d)
	}
	if n := len([]rune(d.Evidence["uncategorized_text"])); n != 20000 {
		t.Fatalf("text evidence is clipped to Stripe's limit: %d", n)
	}
	if got := f.lastPost("/v1/disputes/" + dp).Form.Get("submit"); got != "false" {
		t.Fatalf("submit flag: %q", got)
	}
	// Submitting is final.
	if err := f.p.SubmitDisputeEvidence(ctx(), dp, payment.DisputeEvidence{CustomerName: "Ada"}, true); err != nil {
		t.Fatal(err)
	}
	if d, _ = f.fake.DisputeByID(dp); !d.Submitted || d.Status != "under_review" {
		t.Fatalf("after submit: %+v", d)
	}
	if err := f.p.SubmitDisputeEvidence(ctx(), dp, payment.DisputeEvidence{CustomerName: "Again"}, true); err == nil {
		t.Fatal("evidence cannot be edited after it was submitted")
	}
	if err := f.p.SubmitDisputeEvidence(ctx(), "pi_1", ev, false); !errors.Is(err, payment.ErrNotFound) {
		t.Fatalf("not a dispute: %v", err)
	}
	if err := f.p.SubmitDisputeEvidence(ctx(), "dp_missing", payment.DisputeEvidence{CustomerName: "x"}, false); !errors.Is(err, payment.ErrNotFound) {
		t.Fatalf("unknown dispute: %v", err)
	}

	// Accepting concedes the dispute.
	other := f.fake.OpenDispute(f.paid(1000), "fraudulent")
	if err := f.p.AcceptDispute(ctx(), other); err != nil {
		t.Fatal(err)
	}
	if d, _ = f.fake.DisputeByID(other); d.Status != "lost" {
		t.Fatalf("accepted dispute: %+v", d)
	}
	if err := f.p.AcceptDispute(ctx(), other); err == nil {
		t.Fatal("a closed dispute cannot be accepted again")
	}
	if err := f.p.AcceptDispute(ctx(), "x"); !errors.Is(err, payment.ErrNotFound) {
		t.Fatalf("accept a non-dispute: %v", err)
	}
}

func TestAccountStatus(t *testing.T) {
	f := newFixture(t)
	f.fake.SetWebhookEndpoints([]stripetest.Endpoint{{ID: "we_1", URL: "https://api.staryield.net/api/v1/webhooks/payments/stripe",
		Status: "enabled", EnabledEvents: []string{"refund.updated", "payment_intent.succeeded"}, APIVersion: stripetest.APIVersion}})
	acct := stripetest.DefaultAccount()
	acct.PastDue, acct.CurrentlyDue = []string{"company.tax_id"}, []string{"external_account", "company.tax_id"}
	f.fake.SetAccount(acct)
	st, err := f.p.AccountStatus(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if st.AccountID != "acct_fake" || st.Livemode || st.Country != "US" || st.DefaultCurrency != "usd" || !st.ChargesEnabled || !st.PayoutsEnabled ||
		!st.DetailsSubmitted || st.StatementDescriptor != "STARYIELD MEDIA LLC" || st.CheckedAt.IsZero() {
		t.Fatalf("status: %+v", st)
	}
	if len(st.RequirementsDue) != 2 || st.RequirementsDue[0] != "company.tax_id" || st.RequirementsDue[1] != "external_account" {
		t.Fatalf("requirements: %v", st.RequirementsDue)
	}
	if len(st.WebhookEndpoints) != 1 || st.WebhookEndpoints[0].URL == "" || st.WebhookEndpoints[0].Status != "enabled" ||
		st.WebhookEndpoints[0].EnabledEvents[0] != "payment_intent.succeeded" {
		t.Fatalf("endpoints: %+v", st.WebhookEndpoints)
	}
	// A restricted key that may not list webhook endpoints still reports the account.
	f.fake.Fail("GET", "/v1/webhook_endpoints", 403, 1, "")
	st, err = f.p.AccountStatus(ctx())
	if err != nil || len(st.WebhookEndpoints) != 0 || st.AccountID == "" {
		t.Fatalf("restricted key: %+v %v", st, err)
	}
	// An account in trouble.
	bad := stripetest.DefaultAccount()
	bad.ChargesEnabled, bad.PayoutsEnabled = false, false
	f.fake.SetAccount(bad)
	if st, err = f.p.AccountStatus(ctx()); err != nil || st.ChargesEnabled || st.PayoutsEnabled || st.RequirementsDue == nil {
		t.Fatalf("restricted account: %+v %v", st, err)
	}
	f.fake.Fail("GET", "/v1/account", 500, 1, "")
	if _, err = f.p.AccountStatus(ctx()); !errors.Is(err, payment.ErrUnknownOutcome) {
		t.Fatalf("outage: %v", err)
	}
}
