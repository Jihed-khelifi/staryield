package e2e

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 6.0 (§26.5, §26.7): the admin console's payment operations pages.

func (w *opsWorld) providerStatus(query string) service.ProviderStatusView {
	w.e.t.Helper()
	var st service.ProviderStatusView
	w.admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/billing/stripe/status"+query, nil, &st)
	return st
}

func hasWarning(st service.ProviderStatusView, part string) bool {
	for _, wn := range st.Warnings {
		if strings.Contains(wn, part) {
			return true
		}
	}
	return false
}

// Stripe is absent here: the page still answers (configured false) with what it can say. A registered
// provider that can report its account is checked against the expected descriptor and webhook endpoint.
func TestProviderStatusPage(t *testing.T) {
	w := newOpsWorld(t)
	base := w.e.srv.URL
	st := w.providerStatus("")
	if st.Configured || st.Account != nil || st.Descriptor.Expected != config.DefaultStatementDescriptor || st.Descriptor.Matches ||
		st.Webhook.ExpectedURL != base+"/api/v1/webhooks/payments/stripe" || !hasWarning(st, "not configured") {
		t.Fatalf("stripe absent: %+v", st)
	}
	if len(st.Webhook.Endpoints) != 0 || st.Webhook.Received24h != 0 {
		t.Fatalf("stripe absent: %+v", st.Webhook)
	}

	st = w.providerStatus("?provider=sandbox_a")
	if !st.Configured || st.Account == nil || !st.Account.ChargesEnabled || !st.Descriptor.Matches || st.Livemode ||
		st.Webhook.ExpectedURL != base+"/api/v1/webhooks/payments/sandbox_a" || len(st.Webhook.MissingEvents) != 18 || !hasWarning(st, "No webhook endpoint") {
		t.Fatalf("sandbox_a: %+v", st)
	}
	// The processor's answers are cached for a minute.
	w.proc.SetAccount(payment.AccountStatus{AccountID: "acct_x", ChargesEnabled: false, PayoutsEnabled: false, StatementDescriptor: "SOMETHING ELSE",
		RequirementsDue: []string{"external_account"}})
	w.proc.SetWebhookEndpoints([]payment.WebhookEndpointInfo{{ID: "we_1", URL: base + "/api/v1/webhooks/payments/sandbox_a", Status: "disabled",
		EnabledEvents: []string{"payment_intent.succeeded", "refund.created"}}})
	if cached := w.providerStatus("?provider=sandbox_a"); cached.Account.AccountID == "acct_x" {
		t.Fatalf("the status must be cached for 60 s: %+v", cached.Account)
	}
	w.e.clock.Advance(61 * time.Second)
	st = w.providerStatus("?provider=sandbox_a")
	if st.Account.AccountID != "acct_x" || st.Descriptor.Matches || st.Descriptor.Actual != "SOMETHING ELSE" || len(st.Webhook.MissingEvents) != 16 ||
		len(st.Webhook.Endpoints) != 1 {
		t.Fatalf("after the cache expired: %+v", st)
	}
	for _, part := range []string{"cannot take charges", "Payouts are not enabled", "information due: external_account", "statement descriptor", "webhook endpoint is disabled"} {
		if !hasWarning(st, part) {
			t.Errorf("missing warning %q in %v", part, st.Warnings)
		}
	}
	// A healthy setup has no warnings; the 24 h counts come from the log.
	w.proc.SetAccount(payment.AccountStatus{AccountID: "acct_ok", ChargesEnabled: true, PayoutsEnabled: true, StatementDescriptor: strings.ToLower(config.DefaultStatementDescriptor)})
	w.proc.SetWebhookEndpoints([]payment.WebhookEndpointInfo{{ID: "we_2", URL: base + "/api/v1/webhooks/payments/sandbox_a/", Status: "enabled", EnabledEvents: []string{"*"}}})
	seen := w.providerStatus("?provider=sandbox_a").Webhook.Received24h // the log is shared by the tests of the package
	w.mustSend(w.event("review.opened"))
	w.mustSend(w.event("review.closed"))
	w.e.clock.Advance(61 * time.Second)
	st = w.providerStatus("?provider=sandbox_a")
	if len(st.Warnings) != 0 || len(st.Webhook.MissingEvents) != 0 || st.Webhook.Received24h != seen+2 || st.Webhook.Failed24h != 0 || st.Webhook.LastEventAt == nil {
		t.Fatalf("healthy: %+v", st)
	}
	// Nothing is cached about a provider that is not registered, and clients cannot read the page.
	if r := w.c.call(http.MethodGet, "/api/v1/admin/billing/stripe/status", nil); r.status == http.StatusOK {
		t.Fatal("the status page is for administrators")
	}
}

// The policies card: versioned like the other site settings, step-up, audit, effect on the purchase gate.
func TestPaymentPoliciesAdmin(t *testing.T) {
	w := newOpsWorld(t)
	const path = "/api/v1/admin/billing/policies"
	var cur service.PaymentPoliciesSetting
	w.admin.must(http.StatusOK, http.MethodGet, path, nil, &cur)
	if cur.Version < 1 || !cur.BlockPurchasesOnDispute || !cur.DisputeClawback || cur.EFWAutoRefund || cur.ThreeDSecure != "automatic" {
		t.Fatalf("seeded policies: %+v", cur)
	}
	body := func(block, claw, efw bool, tds string, version int) map[string]any {
		return map[string]any{"block_purchases_on_dispute": block, "dispute_clawback": claw, "efw_auto_refund": efw, "three_d_secure": tds, "expected_version": version}
	}
	w.e.clock.Advance(w.e.app.Config.Admin.StepUpWindow + time.Minute)
	if r := w.admin.call(http.MethodPut, path, body(false, true, true, "any", cur.Version)); r.status == http.StatusOK {
		t.Fatal("changing the policies needs a fresh step-up")
	}
	w.admin.stepUp()
	if r := w.admin.call(http.MethodPut, path, map[string]any{"block_purchases_on_dispute": true}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("incomplete: %d", r.status)
	}
	if r := w.admin.call(http.MethodPut, path, body(true, true, false, "always", cur.Version)); r.status != http.StatusUnprocessableEntity ||
		errField(t, r, "three_d_secure") == "" {
		t.Fatalf("unknown 3-D Secure policy: %d %s", r.status, r.body)
	}
	var out service.PaymentPoliciesSetting
	w.admin.must(http.StatusOK, http.MethodPut, path, body(false, true, true, "any", cur.Version), &out)
	if out.Version != cur.Version+1 || out.BlockPurchasesOnDispute || !out.DisputeClawback || !out.EFWAutoRefund || out.ThreeDSecure != "any" || out.UpdatedBy == nil {
		t.Fatalf("updated: %+v", out)
	}
	if r := w.admin.call(http.MethodPut, path, body(true, true, false, "automatic", cur.Version)); r.status != http.StatusConflict || r.errCode(t) != "version_conflict" {
		t.Fatalf("stale version: %d %s", r.status, r.body)
	}
	if n := w.count("SELECT count(*) FROM audit_logs WHERE action = 'billing.policies.updated'"); n < 1 {
		t.Fatal("audit billing.policies.updated missing")
	}
	// The change is effective at once: blocking is off, so an open dispute no longer pauses purchases.
	w.mustSend(w.event("charge.disputed", openDispute("dp_pol", "needs_response")))
	if r := w.tryBuy(w.c); r.status != http.StatusCreated {
		t.Fatalf("blocking switched off: %d %s", r.status, r.body)
	}
	w.admin.must(http.StatusOK, http.MethodPut, path, body(true, true, false, "automatic", out.Version), &out)
	if r := w.tryBuy(w.c); r.status != http.StatusForbidden {
		t.Fatalf("blocking switched on again: %d", r.status)
	}
	if r := w.c.call(http.MethodGet, path, nil); r.status == http.StatusOK {
		t.Fatal("policies are for administrators")
	}
}

// Disputes in the console: list (filters, pages), detail with the evidence draft, evidence (draft, then final) and accept.
func TestAdminDisputesListDetailEvidenceAndAccept(t *testing.T) {
	w := newOpsWorld(t)
	w.mustSend(w.event("charge.succeeded", func(e *payment.WebhookEvent) { e.Card = &payment.CardMeta{Country: "FR"}; e.ThreeDS = "authenticated" }))
	w.mustSend(w.event("charge.disputed", openDispute("dp_adm_a", "needs_response")))
	w.mustSend(w.event("charge.disputed", func(e *payment.WebhookEvent) {
		openDispute("dp_adm_b", "under_review")(e)
		e.ProviderRef = w.refB
	}))
	a, b := w.dispute("dp_adm_a"), w.dispute("dp_adm_b")

	list := func(query string) service.Page[service.DisputeView] {
		var p service.Page[service.DisputeView]
		w.admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/billing/disputes"+query, nil, &p)
		return p
	}
	if p := list("?status=needs_response&user_id=" + w.c.user.ID.String()); len(p.Items) != 1 || p.Items[0].ID != a.ID || p.Items[0].CreditsFrozen != 150 ||
		p.Items[0].Status != "needs_response" || p.Items[0].EvidenceDueBy == nil {
		t.Fatalf("filtered list: %+v", p.Items)
	}
	all := list("?user_id=" + w.c.user.ID.String() + "&limit=1")
	if len(all.Items) != 1 || all.NextCursor == "" || all.Items[0].ID != b.ID {
		t.Fatalf("newest first: %+v", all)
	}
	if next := list("?user_id=" + w.c.user.ID.String() + "&limit=1&cursor=" + all.NextCursor); len(next.Items) != 1 || next.Items[0].ID != a.ID {
		t.Fatalf("second page: %+v", next.Items)
	}
	if r := w.admin.call(http.MethodGet, "/api/v1/admin/billing/disputes?status=bogus", nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown status: %d", r.status)
	}
	if r := w.c.call(http.MethodGet, "/api/v1/admin/billing/disputes", nil); r.status == http.StatusOK {
		t.Fatal("disputes are for administrators")
	}

	// Detail: the payment, the customer and the evidence draft built from what the platform keeps.
	var d service.DisputeDetail
	w.admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/billing/disputes/"+a.ID.String(), nil, &d)
	if d.Transaction == nil || d.Transaction.ID != w.txA || d.Transaction.Dispute == nil || d.Transaction.Dispute.ID != a.ID || d.Transaction.Status != "disputed" ||
		d.User == nil || d.User.Email != w.c.user.Email || d.User.ID != w.c.user.ID {
		t.Fatalf("detail: %+v", d)
	}
	ev := d.EvidenceDraft
	if !strings.Contains(ev.ProductDescription, "Pack 250: 250 digital credits for live psychic chat, for entertainment purposes only") ||
		ev.CustomerEmail != w.c.user.Email || ev.CustomerName != w.c.user.DisplayName || ev.BillingCountry != "FR" || len(ev.ServiceDate) != 10 ||
		!strings.Contains(ev.RefundPolicyDisclosure, "version 2026-11-13") || !strings.Contains(ev.RefundPolicyDisclosure, "/en/refund-policy") ||
		!strings.Contains(ev.UncategorizedText, "3-D Secure: authenticated") || !strings.Contains(ev.AccessActivityLog, "No reading has been started") ||
		!strings.Contains(ev.CancellationRebuttal, "not a subscription") {
		t.Fatalf("draft: %+v", ev)
	}
	if r := w.admin.call(http.MethodGet, "/api/v1/admin/billing/disputes/"+uuid.NewString(), nil); r.status != http.StatusNotFound {
		t.Fatalf("unknown dispute: %d", r.status)
	}

	// Evidence: step-up, validation, a saved draft, then the final submission.
	evidence := "/api/v1/admin/billing/disputes/" + a.ID.String() + "/evidence"
	w.e.clock.Advance(w.e.app.Config.Admin.StepUpWindow + time.Minute)
	if r := w.admin.call(http.MethodPost, evidence, map[string]any{"fields": ev, "submit": false}); r.status == http.StatusOK {
		t.Fatal("evidence needs a fresh step-up")
	}
	w.admin.stepUp()
	bad := ev
	bad.CustomerEmail, bad.ServiceDate = "nope", "yesterday"
	if r := w.admin.call(http.MethodPost, evidence, map[string]any{"fields": bad, "submit": false}); r.status != http.StatusUnprocessableEntity ||
		errField(t, r, "customer_email") == "" || errField(t, r, "service_date") == "" {
		t.Fatalf("invalid evidence: %d %s", r.status, r.body)
	}
	empty := ev
	empty.ProductDescription = ""
	if r := w.admin.call(http.MethodPost, evidence, map[string]any{"fields": empty, "submit": true}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("a submission must describe the product: %d", r.status)
	}
	var saved service.DisputeView
	w.admin.must(http.StatusOK, http.MethodPost, evidence, map[string]any{"fields": ev, "submit": false}, &saved)
	if saved.EvidenceSubmittedAt != nil {
		t.Fatalf("a saved draft is not submitted: %+v", saved)
	}
	if got, ok := w.proc.DisputeEvidence("dp_adm_a"); !ok || got.Submitted || got.Evidence.CustomerEmail != w.c.user.Email ||
		!strings.Contains(got.Evidence.ProductDescription, "digital credits") {
		t.Fatalf("processor draft: %+v %v", got, ok)
	}
	ev.UncategorizedText += " Reviewed by the team."
	w.admin.must(http.StatusOK, http.MethodPost, evidence, map[string]any{"fields": ev, "submit": true}, &saved)
	if saved.EvidenceSubmittedAt == nil {
		t.Fatalf("submitted: %+v", saved)
	}
	if got, _ := w.proc.DisputeEvidence("dp_adm_a"); !got.Submitted || !strings.HasSuffix(got.Evidence.UncategorizedText, "Reviewed by the team.") {
		t.Fatalf("processor evidence: %+v", got)
	}
	if r := w.admin.call(http.MethodPost, evidence, map[string]any{"fields": ev, "submit": true}); r.status != http.StatusConflict {
		t.Fatalf("submitting is final: %d %s", r.status, r.body)
	}
	if !hasAudit(w, "dispute.evidence_submitted") || !hasAudit(w, "dispute.evidence_saved") {
		t.Fatal("evidence audit records missing")
	}
	// A dispute under review takes no evidence; a provider that cannot answer disputes says so.
	if r := w.admin.call(http.MethodPost, "/api/v1/admin/billing/disputes/"+b.ID.String()+"/evidence", map[string]any{"fields": ev, "submit": false}); r.status != http.StatusConflict {
		t.Fatalf("under review: %d", r.status)
	}
	foreign := uuid.New()
	if err := w.e.app.DB.Exec(`INSERT INTO payment_disputes (id, provider, dispute_ref, transaction_id, user_id, amount_cents, currency, status)
		VALUES (?, 'sandbox_b', 'dp_b', ?, ?, 100, 'USD', 'needs_response')`, foreign, w.txA, w.c.user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if r := w.admin.call(http.MethodPost, "/api/v1/admin/billing/disputes/"+foreign.String()+"/evidence", map[string]any{"fields": ev, "submit": false}); r.status != http.StatusConflict {
		t.Fatalf("provider without disputes: %d %s", r.status, r.body)
	}

	// Accept concedes the dispute: it closes as lost (a note is required).
	accept := "/api/v1/admin/billing/disputes/" + b.ID.String() + "/accept"
	if r := w.admin.call(http.MethodPost, accept, map[string]any{"note": ""}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("a note is required: %d", r.status)
	}
	var lost service.DisputeView
	w.admin.must(http.StatusOK, http.MethodPost, accept, map[string]any{"note": "Customer is right, not worth contesting"}, &lost)
	if lost.Status != "lost" || lost.ClosedAt == nil || w.txStatus(w.txB) != "disputed" {
		t.Fatalf("accepted: %+v / %s", lost, w.txStatus(w.txB))
	}
	if got, _ := w.proc.DisputeEvidence("dp_adm_b"); !got.Accepted {
		t.Fatal("the processor was not told")
	}
	if r := w.admin.call(http.MethodPost, accept, map[string]any{"note": "again"}); r.status != http.StatusConflict {
		t.Fatalf("a closed dispute cannot be accepted: %d", r.status)
	}
	if !hasAudit(w, "dispute.accepted") || !hasAudit(w, "dispute.lost") {
		t.Fatal("accept audit records missing")
	}
}

// The usage log lists the readings after the purchase (date, advisor, minutes, credits) and never message text.
func TestDisputeEvidenceUsageLogNeverContainsMessageText(t *testing.T) {
	proc := payment.NewSandboxProcessor()
	e := newEnvWithProcessor(t, proc)
	client, psychic, cws, _, s := startedSession(t, e, 40, 250)
	ack := cws.mustAck(realtime.FrameMessageSend, map[string]any{"session_id": s.ID, "body": "SECRET-QUESTION about my private life", "client_msg_id": "cmsg-evid-1"})
	_ = ack
	admin := e.createAdmin()
	// A card payment from before the reading (inserted: the reading helper buys through the hosted checkout).
	tx := uuid.New()
	ref := "ch_usage_" + uniq("")
	if err := e.app.DB.Exec(`INSERT INTO payment_transactions (id, user_id, kind, status, amount_cents, currency, credits, provider, provider_ref, description, idempotency_key, created_at, updated_at)
		VALUES (?, ?, 'credit_purchase', 'succeeded', 999, 'USD', 250, 'sandbox_a', ?, 'Pack 250', ?, now() - interval '2 hours', now())`, tx, client.user.ID, ref, "manual:"+tx.String()).Error; err != nil {
		t.Fatal(err)
	}
	evt := payment.WebhookEvent{ID: "evt_" + uniq("u"), Type: "charge.disputed", ProviderRef: ref, DisputeRef: "dp_usage", DisputeStatus: "needs_response", AmountCents: 999}
	if r := signedWebhook(e, "sandbox_a", evt, ""); r.status != http.StatusOK {
		t.Fatalf("dispute: %d %s", r.status, r.body)
	}
	id := e.uuidOf("SELECT id FROM payment_disputes WHERE dispute_ref = 'dp_usage'")
	var d service.DisputeDetail
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/billing/disputes/"+id.String(), nil, &d)
	var advisor string
	e.app.DB.Raw("SELECT name FROM psychics WHERE user_id = ?", psychic.user.ID).Scan(&advisor)
	log := d.EvidenceDraft.AccessActivityLog
	if advisor == "" || !strings.Contains(log, "| "+advisor+" |") || !strings.Contains(log, "Readings started after the purchase") {
		t.Fatalf("usage log: %q (advisor %q)", log, advisor)
	}
	for field, text := range map[string]string{"log": log, "uncategorized": d.EvidenceDraft.UncategorizedText, "product": d.EvidenceDraft.ProductDescription} {
		if strings.Contains(text, "SECRET-QUESTION") || strings.Contains(text, "private life") {
			t.Fatalf("message text leaked into %s: %q", field, text)
		}
	}
}

// Payments in the console carry the processor's details and the dispute; Stripe payments link to the Dashboard.
func TestAdminTransactionRowsCarryPaymentOperationsFields(t *testing.T) {
	w := newOpsWorld(t)
	w.mustSend(w.event("charge.succeeded", func(e *payment.WebhookEvent) {
		e.Card, e.Risk, e.ThreeDS = &payment.CardMeta{Country: "DE"}, &payment.RiskInfo{Level: payment.RiskHighest, Score: 90}, "attempted"
	}))
	w.mustSend(w.event("review.opened"))
	w.mustSend(w.event("charge.disputed", openDispute("dp_row", "needs_response")))
	// A Stripe payment (the provider is not registered in this stack): live link, then the test link once the log says test mode.
	stripeTx := uuid.New()
	if err := w.e.app.DB.Exec(`INSERT INTO payment_transactions (id, user_id, kind, status, amount_cents, currency, provider, provider_ref, description, idempotency_key)
		VALUES (?, ?, 'credit_purchase', 'succeeded', 500, 'USD', 'stripe', 'pi_3Abc', 'Stripe pack', ?)`, stripeTx, w.c.user.ID, "manual:"+stripeTx.String()).Error; err != nil {
		t.Fatal(err)
	}
	rows := func() map[uuid.UUID]service.AdminTransactionRow {
		var page service.Page[service.AdminTransactionRow]
		w.admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/billing/transactions?user_id="+w.c.user.ID.String()+"&limit=100", nil, &page)
		out := map[uuid.UUID]service.AdminTransactionRow{}
		for _, r := range page.Items {
			out[r.ID] = r
		}
		return out
	}
	got := rows()
	a := got[w.txA]
	if a.CardCountry != "DE" || a.RiskLevel != "highest" || a.RiskScore == nil || *a.RiskScore != 90 || a.ThreeDS != "attempted" || !a.InReview ||
		a.Dispute == nil || a.Dispute.Status != "needs_response" || a.ProviderURL != "" {
		t.Fatalf("payment A: %+v", a)
	}
	if b := got[w.txB]; b.Dispute != nil || b.InReview || b.CardCountry != "" || b.RiskScore != nil {
		t.Fatalf("payment B must have none of it: %+v", b)
	}
	if s := got[stripeTx]; s.ProviderURL != "https://dashboard.stripe.com/payments/pi_3Abc" {
		t.Fatalf("live link: %q", s.ProviderURL)
	}
	if err := w.e.app.DB.Exec(`INSERT INTO webhook_events (id, provider, event_id, event_type, payload_sha256, status, livemode) VALUES (?, 'stripe', 'evt_t', 'ignored', ?, 'ignored', false)`,
		uuid.New(), strings.Repeat("b", 64)).Error; err != nil {
		t.Fatal(err)
	}
	w.e.clock.Advance(61 * time.Second) // the mode is cached for a minute
	if s := rows()[stripeTx]; s.ProviderURL != "https://dashboard.stripe.com/test/payments/pi_3Abc" {
		t.Fatalf("test link: %q", s.ProviderURL)
	}
}

// fakeInvoicePDFs stands in for the invoices service (service.InvoicePDFs).
type fakeInvoicePDFs struct {
	pdf []byte
	err error
}

func (f fakeInvoicePDFs) PurchaseInvoicePDF(_ context.Context, _ uuid.UUID) ([]byte, string, error) {
	return f.pdf, "invoice-SY-2026-000001.pdf", f.err
}

// The invoice PDF is attached to the evidence only when an invoices service is wired (and has one for the purchase).
func TestDisputeEvidenceAttachesTheInvoiceWhenAnInvoiceServiceIsWired(t *testing.T) {
	w := newOpsWorld(t)
	w.mustSend(w.event("charge.disputed", openDispute("dp_inv", "needs_response")))
	id := w.dispute("dp_inv").ID
	detail := func() service.DisputeDetail {
		var d service.DisputeDetail
		w.admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/billing/disputes/"+id.String(), nil, &d)
		return d
	}
	// The application wires the invoice service (§26.6), so the draft offers the
	// purchase's invoice from the start; a fake makes the attached bytes known.
	if !detail().EvidenceDraft.AttachInvoice {
		t.Fatal("the invoice service is wired: the draft attaches the invoice")
	}
	w.e.app.Services.Billing.WithInvoicePDFs(fakeInvoicePDFs{pdf: []byte("%PDF-1.4 test")})
	ev := detail().EvidenceDraft
	if !ev.AttachInvoice {
		t.Fatal("with an invoice service the draft attaches the invoice")
	}
	w.admin.stepUp()
	path := "/api/v1/admin/billing/disputes/" + id.String() + "/evidence"
	w.admin.must(http.StatusOK, http.MethodPost, path, map[string]any{"fields": ev, "submit": false}, nil)
	if got, _ := w.proc.DisputeEvidence("dp_inv"); string(got.Evidence.ReceiptPDF) != "%PDF-1.4 test" || got.Evidence.ReceiptFilename != "invoice-SY-2026-000001.pdf" {
		t.Fatalf("the receipt: %q %q", got.Evidence.ReceiptPDF, got.Evidence.ReceiptFilename)
	}
	// Unticking the box leaves it out; an invoice that cannot be rendered does not hold the evidence back.
	ev.AttachInvoice = false
	w.admin.must(http.StatusOK, http.MethodPost, path, map[string]any{"fields": ev, "submit": false}, nil)
	if got, _ := w.proc.DisputeEvidence("dp_inv"); len(got.Evidence.ReceiptPDF) != 0 {
		t.Fatalf("unticked: %q", got.Evidence.ReceiptPDF)
	}
	w.e.app.Services.Billing.WithInvoicePDFs(fakeInvoicePDFs{err: context.DeadlineExceeded})
	ev.AttachInvoice = true
	w.admin.must(http.StatusOK, http.MethodPost, path, map[string]any{"fields": ev, "submit": true}, nil)
	if got, _ := w.proc.DisputeEvidence("dp_inv"); !got.Submitted || len(got.Evidence.ReceiptPDF) != 0 {
		t.Fatalf("submitted without the invoice: %+v", got)
	}
}
