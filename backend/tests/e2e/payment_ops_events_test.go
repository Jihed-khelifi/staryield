package e2e

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 6.0 (§26.5): fraud warnings, refund failures, reviews, detached
// cards, deleted customers, stored charge details and the webhook log.

func fraudWarning(ref string, actionable bool) func(*payment.WebhookEvent) {
	return func(e *payment.WebhookEvent) {
		e.ObjectRef, e.Actionable, e.FraudType, e.ProviderType = ref, actionable, "fraudulent", "radar.early_fraud_warning.created"
	}
}

type warningRow struct {
	ID         uuid.UUID
	Action     string
	Actionable bool
	FraudType  string
	Note       string
}

func (w *opsWorld) warning(ref string) warningRow {
	w.e.t.Helper()
	var r warningRow
	if err := w.e.app.DB.Raw("SELECT id, action, actionable, fraud_type, note FROM payment_fraud_warnings WHERE provider = 'sandbox_a' AND warning_ref = ?", ref).
		Scan(&r).Error; err != nil || r.ID == uuid.Nil {
		w.e.t.Fatalf("warning %s: %+v %v", ref, r, err)
	}
	return r
}

// Without auto-refund a warning is recorded and the team decides: refund or dismiss.
func TestFraudWarningIsRecordedAndAdminsRefundOrDismiss(t *testing.T) {
	w := newOpsWorld(t)
	w.mustSend(w.event("fraud.early_warning", fraudWarning("issfr_manual", true)))
	w.mustSend(w.event("fraud.early_warning", fraudWarning("issfr_manual", true))) // updated: no second record
	fw := w.warning("issfr_manual")
	if fw.Action != "none" || !fw.Actionable || fw.FraudType != "fraudulent" {
		t.Fatalf("warning: %+v", fw)
	}
	if n := w.count("SELECT count(*) FROM payment_fraud_warnings WHERE transaction_id = ?", w.txA); n != 1 {
		t.Fatalf("warnings: %d", n)
	}
	if w.txStatus(w.txA) != "succeeded" {
		t.Fatalf("no refund without the policy: %s", w.txStatus(w.txA))
	}
	if mails := w.supportMail("issfr_manual"); len(mails) != 0 { // the alert names the payment, not the provider's reference
		t.Logf("alerts: %v", mails)
	}
	if !hasAudit(w, "fraud_warning.received") {
		t.Fatal("audit fraud_warning.received missing")
	}

	var list service.Page[service.FraudWarningView]
	w.admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/billing/fraud-warnings?status=none", nil, &list)
	found := false
	for _, it := range list.Items {
		found = found || (it.ID == fw.ID && it.WarningRef == "issfr_manual" && it.TransactionID == w.txA && it.Action == "none")
	}
	if !found {
		t.Fatalf("list: %+v", list.Items)
	}
	if r := w.admin.call(http.MethodGet, "/api/v1/admin/billing/fraud-warnings?status=bogus", nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown status filter: %d", r.status)
	}

	// Refund: the step-up and a note are required; the whole refundable amount goes back, lot only.
	path := "/api/v1/admin/billing/fraud-warnings/" + fw.ID.String()
	w.e.clock.Advance(w.e.app.Config.Admin.StepUpWindow + time.Minute) // the step-up of the sign-in has lapsed
	if r := w.admin.call(http.MethodPost, path+"/refund", map[string]any{"note": "confirmed fraud"}); r.status == http.StatusOK {
		t.Fatal("a fraud refund needs a fresh step-up")
	}
	w.admin.stepUp()
	if r := w.admin.call(http.MethodPost, path+"/refund", map[string]any{"note": " "}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("a note is required: %d", r.status)
	}
	var done service.FraudWarningView
	w.admin.must(http.StatusOK, http.MethodPost, path+"/refund", map[string]any{"note": "confirmed fraud"}, &done)
	if done.Action != "refunded" || done.ActionedAt == nil || done.Note != "confirmed fraud" {
		t.Fatalf("refunded warning: %+v", done)
	}
	if w.txStatus(w.txA) != "refunded" || w.lotRemaining(w.purchaseA) != 0 || w.lotRemaining(w.purchaseB) != 600 || w.bonusRemaining() != 50 {
		t.Fatalf("refund: status %s A %d B %d", w.txStatus(w.txA), w.lotRemaining(w.purchaseA), w.lotRemaining(w.purchaseB))
	}
	w.e.invariant(w.c.user.ID)
	if r := w.admin.call(http.MethodPost, path+"/dismiss", map[string]any{"note": "again"}); r.status != http.StatusConflict {
		t.Fatalf("a handled warning cannot be handled again: %d", r.status)
	}
	if !hasAudit(w, "fraud_warning.refunded") {
		t.Fatal("audit fraud_warning.refunded missing")
	}

	// Dismiss another one.
	w.mustSend(w.event("fraud.early_warning", func(e *payment.WebhookEvent) {
		fraudWarning("issfr_dismiss", true)(e)
		e.ProviderRef = w.refB
	}))
	other := w.warning("issfr_dismiss")
	w.admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/billing/fraud-warnings/"+other.ID.String()+"/dismiss", map[string]any{"note": "customer verified"}, &done)
	if done.Action != "dismissed" || w.txStatus(w.txB) != "succeeded" {
		t.Fatalf("dismissed: %+v / %s", done, w.txStatus(w.txB))
	}
	if r := w.admin.call(http.MethodPost, "/api/v1/admin/billing/fraud-warnings/"+uuid.NewString()+"/dismiss", map[string]any{"note": "x"}); r.status != http.StatusNotFound {
		t.Fatalf("unknown warning: %d", r.status)
	}
}

// With efw_auto_refund an actionable warning on an undisputed payment is refunded at once.
func TestFraudWarningAutoRefundOnlyWhenActionableAndUndisputed(t *testing.T) {
	w := newOpsWorld(t)
	w.setPolicies(true, true, true, "automatic")

	// Not actionable: recorded, nothing refunded.
	w.mustSend(w.event("fraud.early_warning", fraudWarning("issfr_na", false)))
	if fw := w.warning("issfr_na"); fw.Action != "none" || fw.Actionable || w.txStatus(w.txA) != "succeeded" {
		t.Fatalf("not actionable: %+v %s", fw, w.txStatus(w.txA))
	}
	// Actionable: the refundable amount goes back (lot clawback, system actor), the warning is marked refunded.
	evt := w.event("fraud.early_warning", fraudWarning("issfr_auto", true))
	w.mustSend(evt)
	w.mustSend(evt) // redelivery: nothing more to refund
	fw := w.warning("issfr_auto")
	if fw.Action != "refunded" || fw.Note != "Refunded automatically." {
		t.Fatalf("auto refund: %+v", fw)
	}
	var refunded int64
	w.e.app.DB.Raw("SELECT refunded_amount_cents FROM payment_transactions WHERE id = ?", w.txA).Scan(&refunded)
	if w.txStatus(w.txA) != "refunded" || refunded != 999 || w.lotRemaining(w.purchaseA) != 0 || w.lotRemaining(w.purchaseB) != 600 {
		t.Fatalf("payment: %s %d, lots A %d B %d", w.txStatus(w.txA), refunded, w.lotRemaining(w.purchaseA), w.lotRemaining(w.purchaseB))
	}
	if n := w.count("SELECT count(*) FROM payment_transactions WHERE parent_transaction_id = ? AND kind = 'refund' AND status = 'succeeded'", w.txA); n != 1 {
		t.Fatalf("refund rows: %d", n)
	}
	if n := w.count("SELECT count(*) FROM audit_logs WHERE action = 'billing.refund' AND target_id = ? AND actor_id IS NULL", w.txA.String()); n != 1 {
		t.Fatalf("the refund is audited with the system as actor: %d", n)
	}
	w.e.invariant(w.c.user.ID)

	// A disputed payment is never refunded automatically.
	w.mustSend(w.event("charge.disputed", func(e *payment.WebhookEvent) {
		openDispute("dp_efw", "needs_response")(e)
		e.ProviderRef = w.refB
	}))
	w.mustSend(w.event("fraud.early_warning", func(e *payment.WebhookEvent) {
		fraudWarning("issfr_disputed", true)(e)
		e.ProviderRef = w.refB
	}))
	if fw := w.warning("issfr_disputed"); fw.Action != "none" || w.txStatus(w.txB) != "disputed" {
		t.Fatalf("disputed payment: %+v %s", fw, w.txStatus(w.txB))
	}
}

// A refund the processor refuses (the cardholder disputed it before we heard) leaves the warning to the team.
func TestFraudWarningAutoRefundRefusedByTheProcessorLeavesItToTheTeam(t *testing.T) {
	w := newOpsWorld(t)
	w.setPolicies(true, true, true, "automatic")
	w.proc.DisputeCharge(w.refA)
	w.mustSend(w.event("fraud.early_warning", fraudWarning("issfr_refused", true)))
	if fw := w.warning("issfr_refused"); fw.Action != "none" || w.txStatus(w.txA) != "succeeded" {
		t.Fatalf("refused refund: %+v %s", fw, w.txStatus(w.txA))
	}
	if n := w.count("SELECT count(*) FROM payment_transactions WHERE parent_transaction_id = ? AND status = 'pending'", w.txA); n != 0 {
		t.Fatalf("pending refund rows: %d", n)
	}
}

// A refund made in the provider's dashboard is booked once; when the processor
// then reports it failed, the refund row fails, the payment's totals follow and
// the team is told, but credits already taken back are not restored.
func TestRefundFailedMarksTheRefundAndAlertsWithoutRestoringCredits(t *testing.T) {
	w := newOpsWorld(t)
	refund := w.event("charge.refunded", func(e *payment.WebhookEvent) {
		e.RefundRef, e.ObjectRef, e.AmountCents, e.RefundStatus = "re_dash_1", "re_dash_1", 999, "pending"
	})
	w.mustSend(refund)
	w.mustSend(refund)
	var refunded int64
	w.e.app.DB.Raw("SELECT refunded_amount_cents FROM payment_transactions WHERE id = ?", w.txA).Scan(&refunded)
	if w.txStatus(w.txA) != "refunded" || refunded != 999 || w.lotRemaining(w.purchaseA) != 0 {
		t.Fatalf("dashboard refund: %s %d lot %d", w.txStatus(w.txA), refunded, w.lotRemaining(w.purchaseA))
	}
	if st := w.text("SELECT status FROM credit_purchases WHERE id = ?", w.purchaseA); st != "refunded" {
		t.Fatalf("purchase status: %s", st)
	}
	balance := w.c.wallet().Balance

	failed := w.event("refund.failed", func(e *payment.WebhookEvent) {
		e.RefundRef, e.ObjectRef, e.RefundStatus = "re_dash_1", "re_dash_1", "failed"
	})
	w.mustSend(failed)
	w.mustSend(w.event("refund.failed", func(e *payment.WebhookEvent) { e.RefundRef, e.ObjectRef = "re_dash_1", "re_dash_1" })) // a second report
	w.e.app.DB.Raw("SELECT refunded_amount_cents FROM payment_transactions WHERE id = ?", w.txA).Scan(&refunded)
	if w.txStatus(w.txA) != "succeeded" || refunded != 0 {
		t.Fatalf("the payment was not refunded after all: %s %d", w.txStatus(w.txA), refunded)
	}
	if st := w.text("SELECT status FROM credit_purchases WHERE id = ?", w.purchaseA); st != "succeeded" {
		t.Fatalf("purchase status: %s", st)
	}
	if st := w.text("SELECT status FROM payment_transactions WHERE provider_ref = 're_dash_1'"); st != "failed" {
		t.Fatalf("refund row: %s", st)
	}
	if got := w.c.wallet().Balance; got != balance {
		t.Fatalf("credits are not restored automatically: %d, was %d", got, balance)
	}
	if n := w.count("SELECT count(*) FROM audit_logs WHERE action = 'refund.failed' AND target_id = ?", w.txA.String()); n != 1 {
		t.Fatalf("audit records: %d", n)
	}
	if mails := w.supportMail(w.txA.String()); len(mails) != 1 || !strings.Contains(mails[0], "refund failed") {
		t.Fatalf("alerts: %v", mails)
	}
	// A refund the platform never booked (unknown reference) is acknowledged and logged as ignored.
	w.mustSend(w.event("refund.failed", func(e *payment.WebhookEvent) { e.RefundRef, e.ObjectRef = "re_unknown", "re_unknown" }))
	if st := w.text("SELECT status FROM webhook_events WHERE provider = 'sandbox_a' AND object_ref = 're_unknown'"); st != "ignored" {
		t.Fatalf("log status: %s", st)
	}
}

// review.opened flags the payment until review.closed; the console shows it.
func TestReviewFlagsThePayment(t *testing.T) {
	w := newOpsWorld(t)
	inReview := func() bool {
		var rows service.Page[service.AdminTransactionRow]
		w.admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/billing/transactions?user_id="+w.c.user.ID.String(), nil, &rows)
		for _, r := range rows.Items {
			if r.ID == w.txA {
				return r.InReview
			}
		}
		t.Fatal("payment not listed")
		return false
	}
	if inReview() {
		t.Fatal("not in review at first")
	}
	w.mustSend(w.event("review.opened"))
	if !inReview() {
		t.Fatal("review.opened must flag the payment")
	}
	w.mustSend(w.event("review.closed"))
	if inReview() {
		t.Fatal("review.closed must clear the flag")
	}
}

// charge.succeeded stores the card country, risk and 3-D Secure result on the payment.
func TestChargeSucceededStoresCardRiskAndThreeDSecure(t *testing.T) {
	w := newOpsWorld(t)
	w.mustSend(w.event("charge.succeeded", func(e *payment.WebhookEvent) {
		e.Card = &payment.CardMeta{Brand: "visa", Last4: "4242", Country: "FR"}
		e.Risk = &payment.RiskInfo{Level: payment.RiskElevated, Score: 61}
		e.ThreeDS = "authenticated"
	}))
	var row struct {
		CardCountry, CardBrand, CardLast4, RiskLevel, ThreeDS string
		RiskScore                                             *int
	}
	w.e.app.DB.Raw("SELECT card_country, card_brand, card_last4, risk_level, risk_score, three_ds FROM payment_transactions WHERE id = ?", w.txA).Scan(&row)
	if row.CardCountry != "FR" || row.CardBrand != "visa" || row.CardLast4 != "4242" || row.RiskLevel != "elevated" || row.RiskScore == nil || *row.RiskScore != 61 ||
		row.ThreeDS != "authenticated" {
		t.Fatalf("stored details: %+v", row)
	}
	var rows service.Page[service.AdminTransactionRow]
	w.admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/billing/transactions?user_id="+w.c.user.ID.String(), nil, &rows)
	for _, r := range rows.Items {
		if r.ID == w.txA && (r.CardCountry != "FR" || r.RiskLevel != "elevated" || r.RiskScore == nil || *r.RiskScore != 61 || r.ThreeDS != "authenticated") {
			t.Fatalf("admin row: %+v", r)
		}
	}
	// Invalid details are dropped instead of failing the event.
	w.mustSend(w.event("charge.succeeded", func(e *payment.WebhookEvent) {
		e.Card, e.Risk = &payment.CardMeta{Country: "France", Last4: "12"}, &payment.RiskInfo{Level: "mystery", Score: 500}
	}))
	w.e.app.DB.Raw("SELECT card_country, risk_level FROM payment_transactions WHERE id = ?", w.txA).Scan(&row)
	if row.CardCountry != "FR" || row.RiskLevel != "unknown" {
		t.Fatalf("after invalid details: %+v", row)
	}
}

// payment_method.detached removes the saved card here too; customer.deleted forgets the customer and its cards.
func TestMethodDetachedAndCustomerDeletedForgetLocalCards(t *testing.T) {
	w := newOpsWorld(t)
	second := w.c.attachCard("sbx_a_mastercard_4444_01_2034")
	if len(w.c.cards()) != 2 {
		t.Fatalf("setup: %d cards", len(w.c.cards()))
	}
	refs := w.proc.MethodRefs()
	if len(refs) != 2 {
		t.Fatalf("processor cards: %v", refs)
	}
	w.mustSend(w.event("method.detached", func(e *payment.WebhookEvent) { e.ObjectRef = refs[1] })) // the second card
	cards := w.c.cards()
	if len(cards) != 1 || cards[0].ID == second.ID {
		t.Fatalf("the detached card must be removed: %+v", cards)
	}
	w.mustSend(w.event("method.detached", func(e *payment.WebhookEvent) { e.ObjectRef = refs[1] })) // again: nothing to remove
	w.mustSend(w.event("method.detached", func(e *payment.WebhookEvent) { e.ObjectRef = "pm_unknown" }))
	if st := w.text("SELECT status FROM webhook_events WHERE provider = 'sandbox_a' AND object_ref = 'pm_unknown'"); st != "ignored" {
		t.Fatalf("an unknown card is logged as ignored: %s", st)
	}
	w.mustSend(w.event("customer.deleted", func(e *payment.WebhookEvent) { e.ObjectRef = w.proc.CustomerRefs()[0] }))
	if cards := w.c.cards(); len(cards) != 0 {
		t.Fatalf("customer.deleted forgets the cards: %+v", cards)
	}
	if n := w.count("SELECT count(*) FROM payment_customers WHERE user_id = ? AND deleted_at IS NULL", w.c.user.ID); n != 0 {
		t.Fatalf("live customers: %d", n)
	}
	// Past purchases keep their records (the payments list is unchanged).
	if n := w.count("SELECT count(*) FROM payment_transactions WHERE user_id = ? AND kind = 'credit_purchase'", w.c.user.ID); n != 2 {
		t.Fatalf("payments: %d", n)
	}
}

// ---- the webhook log ----

func logRow(w *opsWorld, eventID string) (status, errText string, attempts int) {
	w.e.t.Helper()
	var r struct {
		Status   string
		Error    string
		Attempts int
	}
	if err := w.e.app.DB.Raw("SELECT status, error, attempts FROM webhook_events WHERE provider = 'sandbox_a' AND event_id = ?", eventID).Scan(&r).Error; err != nil {
		w.e.t.Fatal(err)
	}
	return r.Status, r.Error, r.Attempts
}

func TestWebhookLogRecordsEveryDelivery(t *testing.T) {
	w := newOpsWorld(t)
	processed := w.event("charge.dispute_funds_withdrawn", func(e *payment.WebhookEvent) {
		e.DisputeRef, e.ObjectRef, e.ProviderType, e.Livemode = "dp_log", "dp_log", "charge.dispute.funds_withdrawn", true
	})
	w.mustSend(processed)
	if st, msg, n := logRow(w, processed.ID); st != "processed" || msg != "" || n != 1 {
		t.Fatalf("processed: %s %q %d", st, msg, n)
	}
	var row struct {
		ProviderType string
		ObjectRef    string
		Livemode     bool
		EventType    string
		ProcessedAt  *string
	}
	w.e.app.DB.Raw("SELECT provider_type, object_ref, livemode, event_type, processed_at::text AS processed_at FROM webhook_events WHERE event_id = ?", processed.ID).Scan(&row)
	if row.ProviderType != "charge.dispute.funds_withdrawn" || row.ObjectRef != "dp_log" || !row.Livemode || row.EventType != "charge.dispute_funds_withdrawn" || row.ProcessedAt == nil {
		t.Fatalf("log row: %+v", row)
	}
	// Types the platform does not act on, and payments it does not know, are logged as ignored.
	odd := w.event("ignored")
	w.mustSend(odd)
	if st, _, _ := logRow(w, odd.ID); st != "ignored" {
		t.Fatalf("ignored type: %s", st)
	}
	unknown := w.event("charge.refunded", func(e *payment.WebhookEvent) { e.ProviderRef = "ch_nobody" })
	w.mustSend(unknown)
	if st, msg, _ := logRow(w, unknown.ID); st != "ignored" || !strings.Contains(msg, "no payment") {
		t.Fatalf("unknown payment: %s %q", st, msg)
	}
	// A redelivery of an ignored or processed event is acknowledged without a new attempt.
	w.mustSend(odd)
	if _, _, n := logRow(w, odd.ID); n != 1 {
		t.Fatalf("attempts after a redelivery: %d", n)
	}
	// A forged signature is refused and leaves no trace.
	forged := w.event("charge.refunded")
	if r := signedWebhook(w.e, "sandbox_a", forged, "t=1,v1=00"); r.status != http.StatusBadRequest {
		t.Fatalf("forged: %d", r.status)
	}
	if n := w.count("SELECT count(*) FROM webhook_events WHERE event_id = ?", forged.ID); n != 0 {
		t.Fatalf("a rejected delivery is not logged: %d", n)
	}

	// The console lists them newest first with filters and pages.
	list := func(query string) service.Page[service.WebhookEventRow] {
		var page service.Page[service.WebhookEventRow]
		w.admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/billing/webhook-events"+query, nil, &page)
		return page
	}
	ignored := list("?provider=sandbox_a&status=ignored&limit=100")
	for _, it := range ignored.Items {
		if it.Status != "ignored" || it.Provider != "sandbox_a" {
			t.Fatalf("filter leaked %+v", it)
		}
	}
	if len(ignored.Items) < 2 {
		t.Fatalf("ignored events: %d", len(ignored.Items))
	}
	byType := list("?type=charge.dispute.funds_withdrawn&limit=100")
	if len(byType.Items) != 1 || byType.Items[0].EventID != processed.ID || byType.Items[0].ObjectRef != "dp_log" || !byType.Items[0].Livemode {
		t.Fatalf("by provider type: %+v", byType.Items)
	}
	all := list("?provider=sandbox_a&limit=2")
	if len(all.Items) != 2 || all.NextCursor == "" || !all.Items[0].CreatedAt.After(all.Items[1].CreatedAt) && !all.Items[0].CreatedAt.Equal(all.Items[1].CreatedAt) {
		t.Fatalf("first page: %+v", all)
	}
	next := list("?provider=sandbox_a&limit=2&cursor=" + all.NextCursor)
	if len(next.Items) == 0 || next.Items[0].ID == all.Items[0].ID || next.Items[0].ID == all.Items[1].ID {
		t.Fatalf("second page: %+v", next.Items)
	}
	for _, bad := range []string{"?status=bogus", "?cursor=@@@", "?provider=Not%20Valid"} {
		if r := w.admin.call(http.MethodGet, "/api/v1/admin/billing/webhook-events"+bad, nil); r.status != http.StatusUnprocessableEntity {
			t.Fatalf("%s: %d", bad, r.status)
		}
	}
	if r := w.c.call(http.MethodGet, "/api/v1/admin/billing/webhook-events", nil); r.status == http.StatusOK {
		t.Fatal("the log is for administrators")
	}
}

// A delivery that fails is logged as failed and answered 500 (the provider retries); once the cause is gone the
// retry (or an admin replay, which re-fetches the event from the provider) applies it.
func TestFailedWebhookIsRetriedAndReplayed(t *testing.T) {
	w := newOpsWorld(t)
	db := w.e.app.DB
	for _, q := range []string{
		`CREATE OR REPLACE FUNCTION e2e_fail_dispute() RETURNS trigger AS $$ BEGIN IF NEW.dispute_ref LIKE 'dp_fail_%' THEN RAISE EXCEPTION 'e2e: injected failure'; END IF; RETURN NEW; END $$ LANGUAGE plpgsql`,
		`DROP TRIGGER IF EXISTS e2e_fail_dispute ON payment_disputes`,
		`CREATE TRIGGER e2e_fail_dispute BEFORE INSERT ON payment_disputes FOR EACH ROW EXECUTE FUNCTION e2e_fail_dispute()`,
	} {
		if err := db.Exec(q).Error; err != nil {
			t.Fatal(err)
		}
	}
	dropped := false
	drop := func() {
		if !dropped {
			dropped = true
			db.Exec(`DROP TRIGGER IF EXISTS e2e_fail_dispute ON payment_disputes`)
		}
	}
	t.Cleanup(drop)

	evt := w.event("charge.disputed", openDispute("dp_fail_1", "needs_response"))
	if r := w.send(evt); r.status != http.StatusInternalServerError {
		t.Fatalf("a failing delivery must answer 500 so the provider retries: %d %s", r.status, r.body)
	}
	if st, msg, n := logRow(w, evt.ID); st != "failed" || n != 1 || !strings.Contains(msg, "injected failure") {
		t.Fatalf("failed row: %s %q %d", st, msg, n)
	}
	if r := w.send(evt); r.status != http.StatusInternalServerError { // the retry fails again
		t.Fatalf("retry: %d", r.status)
	}
	if st, _, n := logRow(w, evt.ID); st != "failed" || n != 2 {
		t.Fatalf("after the retry: %s %d", st, n)
	}
	if w.txStatus(w.txA) != "succeeded" || w.c.wallet().Balance != 800 {
		t.Fatal("a failed delivery must change nothing")
	}
	drop()

	// Replay: the console re-fetches the event by its id at the provider and applies it.
	var failed service.WebhookEventRow
	var page service.Page[service.WebhookEventRow]
	w.admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/billing/webhook-events?status=failed&provider=sandbox_a", nil, &page)
	for _, it := range page.Items {
		if it.EventID == evt.ID {
			failed = it
		}
	}
	if failed.ID == uuid.Nil || failed.Attempts != 2 || !strings.Contains(failed.Error, "injected") {
		t.Fatalf("failed rows: %+v", page.Items)
	}
	replay := "/api/v1/admin/billing/webhook-events/" + failed.ID.String() + "/replay"
	w.admin.stepUp()
	if r := w.admin.call(http.MethodPost, replay, nil); r.status != http.StatusConflict {
		t.Fatalf("the provider does not have the event yet: %d %s", r.status, r.body) // not recorded at the processor
	}
	w.proc.RecordEvent(evt)
	var row service.WebhookEventRow
	w.admin.must(http.StatusOK, http.MethodPost, replay, nil, &row)
	if row.Status != "processed" || row.Attempts != 3 || row.Error != "" || row.ProcessedAt == nil {
		t.Fatalf("replayed row: %+v", row)
	}
	if d := w.dispute("dp_fail_1"); d.CreditsFrozen != 150 || w.txStatus(w.txA) != "disputed" {
		t.Fatalf("the replay applied the event: %+v", d)
	}
	if !hasAudit(w, "billing.webhook_replayed") {
		t.Fatal("audit billing.webhook_replayed missing")
	}
	w.admin.must(http.StatusOK, http.MethodPost, replay, nil, &row) // replaying again changes nothing
	if w.c.wallet().Balance != 650 {
		t.Fatalf("a second replay must not freeze again: %d", w.c.wallet().Balance)
	}

	// An event without a provider id cannot be fetched again (409); a provider that is not registered answers 503.
	noID := w.event("ignored", func(e *payment.WebhookEvent) { e.ID = "" })
	w.mustSend(noID)
	var sha string
	db.Raw("SELECT id::text FROM webhook_events WHERE provider = 'sandbox_a' AND event_id LIKE 'sha256:%' ORDER BY created_at DESC LIMIT 1").Scan(&sha)
	if sha == "" {
		t.Fatal("no event without id was logged")
	}
	if r := w.admin.call(http.MethodPost, "/api/v1/admin/billing/webhook-events/"+sha+"/replay", nil); r.status != http.StatusConflict {
		t.Fatalf("event without an id: %d %s", r.status, r.body)
	}
	stripeRow := uuid.New()
	if err := db.Exec(`INSERT INTO webhook_events (id, provider, event_id, event_type, payload_sha256, status) VALUES (?, 'stripe', 'evt_s', 'ignored', ?, 'failed')`,
		stripeRow, strings.Repeat("a", 64)).Error; err != nil {
		t.Fatal(err)
	}
	if r := w.admin.call(http.MethodPost, "/api/v1/admin/billing/webhook-events/"+stripeRow.String()+"/replay", nil); r.status != http.StatusServiceUnavailable ||
		r.errCode(t) != "provider_unavailable" {
		t.Fatalf("unregistered provider: %d %s", r.status, r.body)
	}
	if r := w.admin.call(http.MethodPost, "/api/v1/admin/billing/webhook-events/"+uuid.NewString()+"/replay", nil); r.status != http.StatusNotFound {
		t.Fatalf("unknown row: %d", r.status)
	}
}
