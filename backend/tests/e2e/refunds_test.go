package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 5.0: refund requests (docs/platform-extensions.md §24.4).

// errDetail reads error.details[key] of an error response.
func errDetail(t *testing.T, r response, key string) any {
	t.Helper()
	var body struct {
		Error struct {
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(r.body, &body); err != nil {
		t.Fatalf("decode %s: %v", r.body, err)
	}
	return body.Error.Details[key]
}

// errField reads the message of a validation error's field.
func errField(t *testing.T, r response, name string) string {
	t.Helper()
	fields, _ := errDetail(t, r, "fields").(map[string]any)
	s, _ := fields[name].(string)
	return s
}

// uuidOf runs a query that returns one uuid.
func (e *env) uuidOf(query string, args ...any) uuid.UUID {
	e.t.Helper()
	var id uuid.UUID
	if err := e.app.DB.Raw(query, args...).Row().Scan(&id); err != nil {
		e.t.Fatalf("%s: %v", query, err)
	}
	return id
}

const refundsBase = "/api/v1/billing/refund-requests"
const adminRefunds = "/api/v1/admin/billing/refund-requests"

// refundWorld is a client with a sign-up bonus and two card purchases, the
// first of which is partly used, and an administrator ready to act.
type refundWorld struct {
	e                    *env
	c, admin             *client
	purchaseA, purchaseB uuid.UUID
	txA, txB             uuid.UUID
}

// newRefundWorld buys a 250-credit pack (A, 999 cents) and a 600-credit pack
// (B, 1999 cents) and spends 100 credits: purchased credits go first and the
// oldest lot first, so A has 150 left (refundable 599 cents) and B is untouched.
func newRefundWorld(t *testing.T, opts ...option) *refundWorld {
	t.Helper()
	e := newEnv(t, append([]option{withSignupBonus(50)}, opts...)...)
	e.ensurePacks()
	e.disableSignupTrial()
	w := &refundWorld{e: e, c: e.registerClient(), admin: e.createAdmin()}
	card := w.c.attachCard("sbx_a_visa_4242_12_2035")
	w.purchaseA, w.txA = w.c.buyWithCard(250, card)
	w.purchaseB, w.txB = w.c.buyWithCard(600, card)
	if _, err := e.app.Services.Admin.AdjustCredits(context.Background(), w.admin.user.ID, w.c.user.ID, -100, "use", uniq("adj-key-")); err != nil {
		t.Fatal(err)
	}
	return w
}

func (w *refundWorld) request(tx uuid.UUID, reason, details, key string) response {
	return w.c.call(http.MethodPost, refundsBase, map[string]any{"transaction_id": tx, "reason": reason, "details": details, "idempotency_key": key})
}

func (w *refundWorld) mustRequest(tx uuid.UUID, reason string) service.RefundRequestView {
	w.e.t.Helper()
	var v service.RefundRequestView
	w.c.must(http.StatusCreated, http.MethodPost, refundsBase, map[string]any{"transaction_id": tx, "reason": reason, "details": "please",
		"idempotency_key": uniq("refund-key-")}, &v)
	return v
}

func (w *refundWorld) approve(id uuid.UUID, amount int64) response {
	return w.admin.call(http.MethodPost, adminRefunds+"/"+id.String()+"/approve", map[string]any{"amount_cents": amount, "note": "Unused credits, approved."})
}

func (w *refundWorld) status(id uuid.UUID) string {
	var s string
	w.e.app.DB.Raw("SELECT status FROM refund_requests WHERE id = ?", id).Scan(&s)
	return s
}

func (w *refundWorld) refundRows(tx uuid.UUID) int64 {
	var n int64
	w.e.app.DB.Raw("SELECT count(*) FROM payment_transactions WHERE parent_transaction_id = ? AND kind = 'refund' AND status = 'succeeded'", tx).Scan(&n)
	return n
}

// backdate moves a payment (and so its refund window) into the past.
func (w *refundWorld) backdate(tx uuid.UUID, days int) {
	w.e.t.Helper()
	if err := w.e.app.DB.Exec("UPDATE payment_transactions SET created_at = created_at - make_interval(days => ?) WHERE id = ?", days, tx).Error; err != nil {
		w.e.t.Fatal(err)
	}
}

// Request → admin approves a partly used pack: the money is what the policy says, the clawback is that
// lot's remainder only, the other purchase and the bonus credits are untouched.
func TestRefundRequestApprovedForAPartlyUsedPack(t *testing.T) {
	w := newRefundWorld(t)
	e, c, admin := w.e, w.c, w.admin
	lotA, lotB := e.purchaseLot(w.purchaseA), e.purchaseLot(w.purchaseB)
	if lotA.CreditsRemaining != 150 || lotB.CreditsRemaining != 600 {
		t.Fatalf("setup: A %d B %d", lotA.CreditsRemaining, lotB.CreditsRemaining)
	}
	bonusBefore := e.lotOf(c.user.ID, domain.LotBonus)

	// The quote and the transactions list say what a refund would be, from one batched read per page.
	var q domain.RefundQuote
	c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/refund-quote?transaction_id="+w.txA.String(), nil, &q)
	if q.RefundableCents != 599 || q.CreditsPurchased != 250 || q.CreditsUsed != 100 || q.CreditsRemaining != 150 || q.AmountPaidCents != 999 ||
		q.AlreadyRefundedCent != 0 || !q.WithinWindow || q.TransactionID != w.txA || q.PackName != "Pack 250" || q.Currency != "USD" {
		t.Fatalf("quote: %+v", q)
	}
	if want := q.PurchasedAt.AddDate(0, 0, e.app.Config.Refunds.WindowDays); !q.WindowEndsAt.Equal(want) {
		t.Fatalf("window ends %s, want %s", q.WindowEndsAt, want)
	}
	listed := func() map[uuid.UUID]service.TransactionView {
		var page service.Page[service.TransactionView]
		c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/transactions", nil, &page)
		out := map[uuid.UUID]service.TransactionView{}
		for _, row := range page.Items {
			out[row.ID] = row
		}
		return out
	}
	rows := listed()
	if r := rows[w.txA].Refund; r == nil || r.RefundableCents != 599 || r.CreditsRemaining != 150 || !r.CanRequest || r.Request != nil || !r.WindowEndsAt.Equal(q.WindowEndsAt) {
		t.Fatalf("refund info of A: %+v", r)
	}
	if r := rows[w.txB].Refund; r == nil || r.RefundableCents != 1999 || r.CreditsRemaining != 600 || !r.CanRequest {
		t.Fatalf("refund info of B: %+v", r)
	}

	// Creating: 201, a reference, plain-text details (cleaned, never HTML-interpreted), a snapshot of the quote.
	key := uniq("refund-key-")
	r := w.request(w.txA, "unused_credits", "  I only needed a few\r\nminutes. <b>Thanks</b>\x00 ", key)
	if r.status != http.StatusCreated {
		t.Fatalf("create: %d %s", r.status, r.body)
	}
	var req service.RefundRequestView
	r.decode(t, &req)
	if !domain.RefundReferencePattern.MatchString(req.Reference) || req.Status != "open" || req.Channel != "web" || req.Reason != "unused_credits" ||
		req.TransactionID != w.txA || req.Details != "I only needed a few\nminutes. <b>Thanks</b>" || req.Resolution != nil {
		t.Fatalf("request: %+v", req)
	}
	if req.Quote.RefundableCents != 599 || req.Quote.CreditsRemaining != 150 {
		t.Fatalf("snapshot: %+v", req.Quote)
	}
	// An idempotent retry is 200 with the same request; a new key for the same payment is 409 with the open request's id.
	r = w.request(w.txA, "unused_credits", "retry", key)
	var again service.RefundRequestView
	r.decode(t, &again)
	if r.status != http.StatusOK || again.ID != req.ID || again.Details != req.Details {
		t.Fatalf("replay: %d %s", r.status, r.body)
	}
	r = w.request(w.txA, "unused_credits", "", uniq("refund-key-"))
	if r.status != http.StatusConflict || r.errCode(t) != "refund_request_exists" || errDetail(t, r, "request_id") != req.ID.String() {
		t.Fatalf("duplicate: %d %s", r.status, r.body)
	}
	// The same key cannot be reused for another payment.
	if r := w.request(w.txB, "unused_credits", "", key); r.status != http.StatusUnprocessableEntity || errField(t, r, "idempotency_key") == "" {
		t.Fatalf("key reuse: %d %s", r.status, r.body)
	}
	// The list and the transactions page show it.
	var mine service.Page[service.RefundRequestView]
	c.must(http.StatusOK, http.MethodGet, refundsBase, nil, &mine)
	if len(mine.Items) != 1 || mine.Items[0].ID != req.ID {
		t.Fatalf("my requests: %+v", mine.Items)
	}
	if r := listed()[w.txA].Refund; r == nil || r.CanRequest || r.Request == nil || r.Request.ID != req.ID || r.Request.Status != "open" || r.Request.Reference != req.Reference {
		t.Fatalf("refund info with an open request: %+v", r)
	}
	// The client was told it arrived.
	n := waitNotification(t, c, service.NotifyRefundReceived)
	if n.Data["reference"] != req.Reference || n.Data["request_id"] != req.ID.String() {
		t.Fatalf("received notification: %+v", n.Data)
	}

	// The console: batched list with user and live quote, detail with the user's lots.
	var list service.Page[service.AdminRefundRequestView]
	admin.must(http.StatusOK, http.MethodGet, adminRefunds+"?status=open&user_id="+c.user.ID.String(), nil, &list)
	if len(list.Items) != 1 || list.Items[0].User.Email != c.user.Email || list.Items[0].User.ID != c.user.ID || list.Items[0].LiveQuote.RefundableCents != 599 {
		t.Fatalf("admin list: %+v", list.Items)
	}
	var detail service.AdminRefundRequestDetail
	admin.must(http.StatusOK, http.MethodGet, adminRefunds+"/"+req.ID.String(), nil, &detail)
	if len(detail.Lots) != 3 || detail.ID != req.ID {
		t.Fatalf("detail lots: %+v", detail.Lots)
	}
	if r := admin.call(http.MethodGet, adminRefunds+"?status=bogus", nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("bad status filter: %d", r.status)
	}

	// Approving: too much is 422 on amount_cents and the request stays open; then the refund.
	admin.stepUp()
	if r := w.approve(req.ID, 1000); r.status != http.StatusUnprocessableEntity || errField(t, r, "amount_cents") == "" || w.status(req.ID) != "open" {
		t.Fatalf("over-refund: %d %s", r.status, r.body)
	}
	if r := admin.call(http.MethodPost, adminRefunds+"/"+req.ID.String()+"/approve", map[string]any{"amount_cents": 599, "note": "no"}); r.status != http.StatusUnprocessableEntity || errField(t, r, "note") == "" {
		t.Fatalf("short note: %d %s", r.status, r.body)
	}
	r = w.approve(req.ID, 599)
	if r.status != http.StatusOK {
		t.Fatalf("approve: %d %s", r.status, r.body)
	}
	var done service.AdminRefundRequestView
	r.decode(t, &done)
	if done.Status != "approved" || done.Resolution == nil || *done.Resolution.RefundedCents != 599 || done.Resolution.Note != "Unused credits, approved." ||
		done.LiveQuote.AlreadyRefundedCent != 599 {
		t.Fatalf("approved: %+v", done)
	}
	// Money: the policy's amount, booked once, linked to the request.
	var childID uuid.UUID
	var childAmount int64
	if err := e.app.DB.Raw("SELECT id, amount_cents FROM payment_transactions WHERE parent_transaction_id = ? AND kind = 'refund'", w.txA).Row().Scan(&childID, &childAmount); err != nil {
		t.Fatal(err)
	}
	linked := e.uuidOf("SELECT refund_transaction_id FROM refund_requests WHERE id = ?", req.ID)
	var parentStatus string
	var parentRefunded int64
	e.app.DB.Raw("SELECT status, refunded_amount_cents FROM payment_transactions WHERE id = ?", w.txA).Row().Scan(&parentStatus, &parentRefunded)
	if childAmount != 599 || linked != childID || parentStatus != "partially_refunded" || parentRefunded != 599 || w.refundRows(w.txA) != 1 {
		t.Fatalf("money: child %d linked %v/%v parent %s %d", childAmount, linked, childID, parentStatus, parentRefunded)
	}
	// Credits: only A's remaining credits were taken back.
	if got := e.purchaseLot(w.purchaseA); got.CreditsRemaining != 0 || got.ValueRemainingMillicents != 0 {
		t.Fatalf("lot A: %+v", got)
	}
	if got := e.purchaseLot(w.purchaseB); got.CreditsRemaining != 600 || got.ValueRemainingMillicents != lotB.ValueRemainingMillicents {
		t.Fatalf("lot B touched: %+v", got)
	}
	if got := e.lotOf(c.user.ID, domain.LotBonus); got.CreditsRemaining != bonusBefore.CreditsRemaining {
		t.Fatalf("bonus touched: %+v", got)
	}
	if wal := c.wallet(); wal.Balance != 650 { // 600 (B) + 50 (bonus)
		t.Fatalf("wallet: %+v", wal)
	}
	e.invariant(c.user.ID)
	e.allocationsAddUp(c.user.ID)
	// The client is told, in the app and by e-mail (billing mail is always sent).
	n = waitNotification(t, c, service.NotifyRefundResolved)
	if n.Data["status"] != "approved" || n.Data["refunded_cents"] != float64(599) || n.Data["currency"] != "USD" || n.Data["reference"] != req.Reference {
		t.Fatalf("resolved notification: %+v", n.Data)
	}
	// (revisions 5.1: the e-mail is fanned out right after the in-app row, so poll for it.)
	if m := waitMail(t, e, c.user.Email, "Update on your refund request"); !strings.Contains(m.Text, req.Reference) || !strings.Contains(m.Text, "5.99 USD") || m.Category != "billing" {
		t.Fatalf("e-mail: %+v", m)
	}
	if r := listed()[w.txA].Refund; r == nil || r.Request == nil || r.Request.Status != "approved" || r.RefundableCents != 0 {
		t.Fatalf("refund info after approval: %+v", r)
	}

	// Terminal states never move.
	if r := w.approve(req.ID, 100); r.status != http.StatusConflict || r.errCode(t) != "invalid_state" {
		t.Fatalf("second approval: %d %s", r.status, r.body)
	}
	for path, body := range map[string]map[string]any{"/deny": {"note": "Too late now."}, "/credit": {"credits": 5, "note": "Too late now."}} {
		if r := admin.call(http.MethodPost, adminRefunds+"/"+req.ID.String()+path, body); r.status != http.StatusConflict || r.errCode(t) != "invalid_state" {
			t.Fatalf("%s after approval: %d %s", path, r.status, r.body)
		}
	}
	if r := c.call(http.MethodPost, refundsBase+"/"+req.ID.String()+"/withdraw", nil); r.status != http.StatusConflict || r.errCode(t) != "invalid_state" {
		t.Fatalf("withdraw after approval: %d %s", r.status, r.body)
	}
	if w.refundRows(w.txA) != 1 {
		t.Fatal("a refund was booked twice")
	}
	// Audit trail.
	var actions []string
	e.app.DB.Raw("SELECT action FROM audit_logs WHERE target_type = 'refund_request' AND target_id = ? ORDER BY created_at", req.ID.String()).Scan(&actions)
	if strings.Join(actions, ",") != "refund_request.created,refund_request.approved" {
		t.Fatalf("audit: %v", actions)
	}
	// Webhook-driven refunds: a request stays open and the live quote shows what was already refunded.
	req2 := w.mustRequest(w.txB, "technical_problem")
	if _, err := e.app.Services.Billing.RefundTransaction(context.Background(), w.txB, service.RefundInput{AmountCents: 500, Reason: "Issued from the provider dashboard"}); err != nil {
		t.Fatal(err)
	}
	var live service.AdminRefundRequestDetail
	admin.must(http.StatusOK, http.MethodGet, adminRefunds+"/"+req2.ID.String(), nil, &live)
	if live.Status != "open" || live.LiveQuote.AlreadyRefundedCent != 500 || live.Quote.AlreadyRefundedCent != 0 {
		t.Fatalf("a refund made elsewhere must show in the live quote and leave the request open: %+v / %+v", live.Quote, live.LiveQuote)
	}
}

// Deny, withdraw, courtesy credits.
func TestRefundRequestDenyWithdrawAndCredit(t *testing.T) {
	w := newRefundWorld(t)
	e, c, admin := w.e, w.c, w.admin
	admin.stepUp()

	// Deny: nothing is refunded, the client can ask again.
	first := w.mustRequest(w.txA, "unused_credits")
	if r := admin.call(http.MethodPost, adminRefunds+"/"+first.ID.String()+"/deny", map[string]any{"note": "x"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("short deny note: %d", r.status)
	}
	var denied service.AdminRefundRequestView
	admin.must(http.StatusOK, http.MethodPost, adminRefunds+"/"+first.ID.String()+"/deny", map[string]any{"note": "Credits were used in a completed reading."}, &denied)
	if denied.Status != "denied" || denied.Resolution == nil || denied.Resolution.RefundedCents != nil || denied.Resolution.Note == "" {
		t.Fatalf("denied: %+v", denied)
	}
	if w.refundRows(w.txA) != 0 || e.purchaseLot(w.purchaseA).CreditsRemaining != 150 {
		t.Fatal("a denial must not move money or credits")
	}
	n := waitNotification(t, c, service.NotifyRefundResolved)
	if n.Data["status"] != "denied" {
		t.Fatalf("notification: %+v", n.Data)
	}

	// A new request is possible (here with the Idempotency-Key header; a retry with it is a replay);
	// the client withdraws it; someone else cannot.
	c.ensureCSRF()
	send := func() response {
		b, _ := json.Marshal(map[string]any{"transaction_id": w.txA, "reason": "unused_credits"})
		return c.raw(http.MethodPost, refundsBase, bytes.NewReader(b), map[string]string{"Content-Type": "application/json",
			"X-CSRF-Token": c.csrf, "Idempotency-Key": "header-key-" + w.txA.String()})
	}
	var req service.RefundRequestView
	if r := send(); r.status != http.StatusCreated {
		t.Fatalf("header key: %d %s", r.status, r.body)
	} else {
		r.decode(t, &req)
	}
	var replay service.RefundRequestView
	if r := send(); r.status != http.StatusOK {
		t.Fatalf("header replay: %d %s", r.status, r.body)
	} else if r.decode(t, &replay); replay.ID != req.ID {
		t.Fatalf("replay answered another request: %+v", replay)
	}
	other := e.registerClient()
	if r := other.call(http.MethodPost, refundsBase+"/"+req.ID.String()+"/withdraw", nil); r.status != http.StatusNotFound {
		t.Fatalf("stranger withdraws: %d %s", r.status, r.body)
	}
	if w.status(req.ID) != "open" {
		t.Fatal("a stranger changed the request")
	}
	var withdrawn service.RefundRequestView
	c.must(http.StatusOK, http.MethodPost, refundsBase+"/"+req.ID.String()+"/withdraw", nil, &withdrawn)
	if withdrawn.Status != "withdrawn" {
		t.Fatalf("withdrawn: %+v", withdrawn)
	}
	if r := c.call(http.MethodPost, refundsBase+"/"+req.ID.String()+"/withdraw", nil); r.status != http.StatusConflict || r.errCode(t) != "invalid_state" {
		t.Fatalf("withdraw twice: %d %s", r.status, r.body)
	}
	if r := admin.call(http.MethodPost, adminRefunds+"/"+req.ID.String()+"/deny", map[string]any{"note": "Not needed anymore."}); r.status != http.StatusConflict {
		t.Fatalf("deny after withdrawal: %d", r.status)
	}
	// Another person's request is not found for the list either.
	var others service.Page[service.RefundRequestView]
	other.must(http.StatusOK, http.MethodGet, refundsBase, nil, &others)
	if len(others.Items) != 0 {
		t.Fatalf("a stranger sees requests: %+v", others.Items)
	}

	// Courtesy credits: a bonus lot appears, the request is credited, the grant is single.
	req = w.mustRequest(w.txB, "other")
	lotsBefore := len(e.lots(c.user.ID))
	balanceBefore := c.wallet().Balance
	if r := admin.call(http.MethodPost, adminRefunds+"/"+req.ID.String()+"/credit", map[string]any{"credits": 0, "note": "Courtesy credits."}); r.status != http.StatusUnprocessableEntity || errField(t, r, "credits") == "" {
		t.Fatalf("zero credits: %d %s", r.status, r.body)
	}
	if r := admin.call(http.MethodPost, adminRefunds+"/"+req.ID.String()+"/credit", map[string]any{"credits": 100001, "note": "Courtesy credits."}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("too many credits: %d", r.status)
	}
	var credited service.AdminRefundRequestView
	admin.must(http.StatusOK, http.MethodPost, adminRefunds+"/"+req.ID.String()+"/credit", map[string]any{"credits": 40, "note": "Courtesy credits instead."}, &credited)
	if credited.Status != "credited" || credited.Resolution == nil || *credited.Resolution.CreditsGranted != 40 || credited.Resolution.RefundedCents != nil {
		t.Fatalf("credited: %+v", credited)
	}
	lots := e.lots(c.user.ID)
	var courtesy *domain.CreditLot
	for i := range lots {
		if lots[i].Source == domain.LotBonus && lots[i].Origin == "admin_adjustment" {
			courtesy = &lots[i]
		}
	}
	if len(lots) != lotsBefore+1 || courtesy == nil || courtesy.CreditsTotal != 40 || courtesy.PaidCents != 0 {
		t.Fatalf("courtesy lot: %+v", lots)
	}
	if got := c.wallet().Balance; got != balanceBefore+40 {
		t.Fatalf("balance %d, want %d", got, balanceBefore+40)
	}
	if w.refundRows(w.txB) != 0 {
		t.Fatal("credits are not a refund")
	}
	e.invariant(c.user.ID)
	if r := admin.call(http.MethodPost, adminRefunds+"/"+req.ID.String()+"/credit", map[string]any{"credits": 40, "note": "Courtesy credits again."}); r.status != http.StatusConflict {
		t.Fatalf("second grant: %d", r.status)
	}
	if got := c.wallet().Balance; got != balanceBefore+40 {
		t.Fatalf("a second grant was applied: %d", got)
	}
	var granted int64
	e.app.DB.Raw("SELECT credits_granted FROM refund_requests WHERE id = ?", req.ID).Scan(&granted)
	if granted != 40 {
		t.Fatalf("stored grant: %d", granted)
	}
}

// Eligibility, validation and ownership.
func TestRefundRequestEligibilityAndValidation(t *testing.T) {
	w := newRefundWorld(t)
	e, c := w.e, w.c

	// unused_credits needs the window; other reasons do not.
	w.backdate(w.txA, e.app.Config.Refunds.WindowDays+3)
	r := w.request(w.txA, "unused_credits", "", uniq("k-"))
	if r.status != http.StatusConflict || r.errCode(t) != "refund_not_eligible" || errDetail(t, r, "reason") != "outside_window" {
		t.Fatalf("outside window: %d %s", r.status, r.body)
	}
	var q domain.RefundQuote
	c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/refund-quote?transaction_id="+w.txA.String(), nil, &q)
	if q.WithinWindow {
		t.Fatalf("quote says within the window: %+v", q)
	}
	for _, reason := range []string{"technical_problem", "incorrect_charge", "unauthorized_charge", "reading_concern", "other"} {
		var v service.RefundRequestView
		c.must(http.StatusCreated, http.MethodPost, refundsBase, map[string]any{"transaction_id": w.txA, "reason": reason, "idempotency_key": uniq("k-")}, &v)
		c.must(http.StatusOK, http.MethodPost, refundsBase+"/"+v.ID.String()+"/withdraw", nil, nil)
	}
	// The list of transactions still lets the client ask (technical problems have no deadline).
	var page service.Page[service.TransactionView]
	c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/transactions", nil, &page)
	for _, row := range page.Items {
		if row.ID == w.txA && (row.Refund == nil || !row.Refund.CanRequest) {
			t.Fatalf("A after the window: %+v", row.Refund)
		}
	}

	// Not a credit purchase: a refund row; not paid; fully refunded.
	admin := w.admin
	admin.stepUp()
	if r := w.approve(w.mustRequest(w.txA, "technical_problem").ID, 100); r.status != http.StatusOK {
		t.Fatalf("approve: %d %s", r.status, r.body)
	}
	refundRow := e.uuidOf("SELECT id FROM payment_transactions WHERE parent_transaction_id = ? AND kind = 'refund'", w.txA)
	r = w.request(refundRow, "other", "", uniq("k-"))
	if r.status != http.StatusConflict || errDetail(t, r, "reason") != "not_credit_purchase" {
		t.Fatalf("refund row: %d %s", r.status, r.body)
	}
	if r := c.call(http.MethodGet, "/api/v1/billing/refund-quote?transaction_id="+refundRow.String(), nil); r.status != http.StatusConflict || errDetail(t, r, "reason") != "not_credit_purchase" {
		t.Fatalf("quote of a refund row: %d %s", r.status, r.body)
	}
	e.app.DB.Exec("UPDATE payment_transactions SET status = 'failed' WHERE id = ?", w.txB)
	if r := w.request(w.txB, "other", "", uniq("k-")); r.status != http.StatusConflict || errDetail(t, r, "reason") != "not_paid" {
		t.Fatalf("failed payment: %d %s", r.status, r.body)
	}
	if r := c.call(http.MethodGet, "/api/v1/billing/refund-quote?transaction_id="+w.txB.String(), nil); r.status != http.StatusConflict {
		t.Fatalf("quote of a failed payment: %d", r.status)
	}
	e.app.DB.Exec("UPDATE payment_transactions SET status = 'refunded', refunded_amount_cents = amount_cents WHERE id = ?", w.txB)
	if r := w.request(w.txB, "other", "", uniq("k-")); r.status != http.StatusConflict || errDetail(t, r, "reason") != "fully_refunded" {
		t.Fatalf("refunded payment: %d %s", r.status, r.body)
	}
	var qb domain.RefundQuote
	c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/refund-quote?transaction_id="+w.txB.String(), nil, &qb)
	if qb.RefundableCents != 0 {
		t.Fatalf("a refunded payment refunds nothing more: %+v", qb)
	}
	var rows service.Page[service.TransactionView]
	c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/transactions", nil, &rows)
	for _, row := range rows.Items {
		if row.ID == w.txB && (row.Refund == nil || row.Refund.CanRequest) {
			t.Fatalf("a refunded payment cannot be requested again: %+v", row.Refund)
		}
		if row.Kind == "refund" && row.Refund != nil {
			t.Fatalf("refund rows carry no refund info: %+v", row.Refund)
		}
	}

	// Validation and ownership.
	other := e.registerClient()
	bad := []struct {
		name string
		body map[string]any
		want string
	}{
		{"reason", map[string]any{"transaction_id": w.txA, "reason": "bored", "idempotency_key": uniq("k-")}, "reason"},
		{"no reason", map[string]any{"transaction_id": w.txA, "idempotency_key": uniq("k-")}, "reason"},
		{"details", map[string]any{"transaction_id": w.txA, "reason": "other", "details": strings.Repeat("é", 2001), "idempotency_key": uniq("k-")}, "details"},
		{"key", map[string]any{"transaction_id": w.txA, "reason": "other", "idempotency_key": "short"}, "idempotency_key"},
		{"no payment", map[string]any{"reason": "other", "idempotency_key": uniq("k-")}, "transaction_id"},
	}
	for _, b := range bad {
		if r := c.call(http.MethodPost, refundsBase, b.body); r.status != http.StatusUnprocessableEntity || errField(t, r, b.want) == "" {
			t.Errorf("%s: %d %s", b.name, r.status, r.body)
		}
	}
	if r := c.call(http.MethodPost, refundsBase, map[string]any{"transaction_id": w.txA, "reason": "other", "idempotency_key": uniq("k-"), "amount_cents": 5}); r.status != http.StatusUnprocessableEntity {
		t.Errorf("unknown field: %d", r.status)
	}
	// Exactly 2000 characters of details are fine.
	if r := w.request(w.txA, "other", strings.Repeat("é", 2000), uniq("k-")); r.status != http.StatusCreated {
		t.Errorf("2000 characters: %d %s", r.status, r.body)
	}
	if r := other.call(http.MethodPost, refundsBase, map[string]any{"transaction_id": w.txA, "reason": "other", "idempotency_key": uniq("k-")}); r.status != http.StatusNotFound {
		t.Errorf("someone else's payment: %d", r.status)
	}
	if r := other.call(http.MethodGet, "/api/v1/billing/refund-quote?transaction_id="+w.txA.String(), nil); r.status != http.StatusNotFound {
		t.Errorf("quote of someone else's payment: %d", r.status)
	}
	if r := c.call(http.MethodPost, refundsBase, map[string]any{"transaction_id": uuid.New(), "reason": "other", "idempotency_key": uniq("k-")}); r.status != http.StatusNotFound {
		t.Errorf("unknown payment: %d", r.status)
	}
	if r := c.call(http.MethodGet, "/api/v1/billing/refund-quote", nil); r.status != http.StatusUnprocessableEntity {
		t.Errorf("quote without a payment: %d", r.status)
	}
}

// The kill switch stops in-app requests only.
func TestRefundRequestsKillSwitch(t *testing.T) {
	w := newRefundWorld(t, func(c *config.Config) { c.Refunds.RequestsEnabled = false })
	e, c, admin := w.e, w.c, w.admin
	r := w.request(w.txA, "unused_credits", "", uniq("k-"))
	if r.status != http.StatusServiceUnavailable || r.errCode(t) != "feature_unavailable" {
		t.Fatalf("switched off: %d %s", r.status, r.body)
	}
	// Reading still works, the transactions page says requests are off, and the console can record an e-mailed request.
	c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/refund-quote?transaction_id="+w.txA.String(), nil, nil)
	c.must(http.StatusOK, http.MethodGet, refundsBase, nil, nil)
	var page service.Page[service.TransactionView]
	c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/transactions", nil, &page)
	for _, row := range page.Items {
		if row.Refund != nil && row.Refund.CanRequest {
			t.Fatalf("can_request while switched off: %+v", row.Refund)
		}
	}
	admin.stepUp()
	var rec service.AdminRefundRequestView
	admin.must(http.StatusCreated, http.MethodPost, adminRefunds, map[string]any{"transaction_id": w.txA, "reason": "unused_credits",
		"details": "Received by e-mail.", "channel": "email"}, &rec)
	if rec.Channel != "email" || rec.Status != "open" || rec.User.ID != c.user.ID || rec.LiveQuote.RefundableCents != 599 {
		t.Fatalf("recorded request: %+v", rec)
	}
	// Approval and the other actions are unaffected.
	if r := w.approve(rec.ID, 599); r.status != http.StatusOK {
		t.Fatalf("approve: %d %s", r.status, r.body)
	}
	e.invariant(c.user.ID)
	// An administrator recording a request outside the window is allowed (she decides), but not for a refunded payment.
	w.backdate(w.txB, 40)
	var out service.AdminRefundRequestView
	admin.must(http.StatusCreated, http.MethodPost, adminRefunds, map[string]any{"transaction_id": w.txB, "reason": "unused_credits", "details": "Late e-mail.", "channel": "admin"}, &out)
	if out.Quote.WithinWindow || out.Channel != "admin" {
		t.Fatalf("late request: %+v", out)
	}
	if r := admin.call(http.MethodPost, adminRefunds, map[string]any{"transaction_id": w.txB, "reason": "other", "channel": "web"}); r.status != http.StatusUnprocessableEntity || errField(t, r, "channel") == "" {
		t.Fatalf("web channel by an administrator: %d %s", r.status, r.body)
	}
	if r := admin.call(http.MethodPost, adminRefunds, map[string]any{"transaction_id": w.txB, "reason": "other", "channel": "email"}); r.status != http.StatusConflict || r.errCode(t) != "refund_request_exists" {
		t.Fatalf("duplicate by an administrator: %d %s", r.status, r.body)
	}
}

// Every console write needs a fresh step-up; reading does not.
func TestRefundAdminWritesNeedStepUp(t *testing.T) {
	w := newRefundWorld(t)
	e, admin := w.e, w.admin
	req := w.mustRequest(w.txA, "unused_credits")
	e.clock.Advance(11 * time.Minute)
	id := req.ID.String()
	for _, c := range []struct {
		path string
		body map[string]any
	}{
		{adminRefunds, map[string]any{"transaction_id": w.txB, "reason": "other", "channel": "email"}},
		{adminRefunds + "/" + id + "/approve", map[string]any{"amount_cents": 599, "note": "Approved after review."}},
		{adminRefunds + "/" + id + "/deny", map[string]any{"note": "Denied after review."}},
		{adminRefunds + "/" + id + "/credit", map[string]any{"credits": 10, "note": "Credited after review."}},
	} {
		if r := admin.call(http.MethodPost, c.path, c.body); r.status != http.StatusForbidden || r.errCode(t) != "step_up_required" {
			t.Fatalf("%s without step-up: %d %s", c.path, r.status, r.body)
		}
	}
	if w.status(req.ID) != "open" || w.refundRows(w.txA) != 0 {
		t.Fatal("a write went through without step-up")
	}
	admin.must(http.StatusOK, http.MethodGet, adminRefunds, nil, nil)
	admin.must(http.StatusOK, http.MethodGet, adminRefunds+"/"+id, nil, nil)
	// A client cannot reach the console at all.
	if r := w.c.call(http.MethodGet, adminRefunds, nil); r.status == http.StatusOK {
		t.Fatalf("a client listed the console's requests: %d", r.status)
	}
	if r := w.c.call(http.MethodPost, adminRefunds+"/"+id+"/approve", map[string]any{"amount_cents": 599, "note": "Approved by me."}); r.status == http.StatusOK {
		t.Fatalf("a client approved a refund: %d", r.status)
	}
}

// Two administrators approving the same request: exactly one refund.
func TestRefundApproveConcurrentlyRefundsOnce(t *testing.T) {
	w := newRefundWorld(t)
	e := w.e
	second := e.createAdmin()
	w.admin.stepUp()
	second.stepUp()
	req := w.mustRequest(w.txA, "unused_credits")

	var wg sync.WaitGroup
	statuses := make([]int, 4)
	codes := make([]string, 4)
	for i := range statuses {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a := w.admin
			if i%2 == 1 {
				a = second
			}
			r := a.call(http.MethodPost, adminRefunds+"/"+req.ID.String()+"/approve", map[string]any{"amount_cents": 599, "note": "Approved concurrently."})
			statuses[i] = r.status
			if r.status != http.StatusOK {
				codes[i] = r.errCode(t)
			}
		}()
	}
	wg.Wait()
	ok, conflicts := 0, 0
	for i, s := range statuses {
		switch {
		case s == http.StatusOK:
			ok++
		case s == http.StatusConflict && codes[i] == "invalid_state":
			conflicts++
		default:
			t.Errorf("unexpected answer %d %s", s, codes[i])
		}
	}
	if ok != 1 || conflicts != 3 {
		t.Fatalf("statuses %v codes %v", statuses, codes)
	}
	if w.refundRows(w.txA) != 1 {
		t.Fatalf("refund rows: %d", w.refundRows(w.txA))
	}
	var refunded int64
	e.app.DB.Raw("SELECT refunded_amount_cents FROM payment_transactions WHERE id = ?", w.txA).Scan(&refunded)
	if refunded != 599 || w.status(req.ID) != "approved" {
		t.Fatalf("refunded %d status %s", refunded, w.status(req.ID))
	}
	e.invariant(w.c.user.ID)

	// And two simultaneous submissions for one payment: one request.
	statuses = make([]int, 4)
	for i := range statuses {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i] = w.request(w.txB, "unused_credits", "", uniq("k-")).status
		}()
	}
	wg.Wait()
	created, conflicts := 0, 0
	for _, s := range statuses {
		switch s {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
		default:
			t.Errorf("submission answered %d", s)
		}
	}
	var open int64
	e.app.DB.Raw("SELECT count(*) FROM refund_requests WHERE transaction_id = ? AND status = 'open'", w.txB).Scan(&open)
	if created != 1 || conflicts != 3 || open != 1 {
		t.Fatalf("created %d conflicts %d open %d", created, conflicts, open)
	}
}

// The provider refuses (409 refund_refused, the request stays open) or has a refund waiting to be booked
// (a retry with the same amount completes it, another amount is refused).
func TestRefundApproveProviderOutcomes(t *testing.T) {
	proc := payment.NewSandboxProcessor()
	e := newEnvWithProcessor(t, proc, withSignupBonus(50))
	e.ensurePacks()
	e.disableSignupTrial()
	c := e.registerClient()
	admin := e.createAdmin()
	card := c.attachCard("sbx_a_visa_4242_12_2035")
	_, tx := c.buyWithCard(250, card)
	w := &refundWorld{e: e, c: c, admin: admin, txA: tx}
	req := w.mustRequest(tx, "unused_credits")
	admin.stepUp()

	// Definitively refused: the charge is unknown to the provider.
	var ref string
	e.app.DB.Raw("SELECT provider_ref FROM payment_transactions WHERE id = ?", tx).Scan(&ref)
	e.app.DB.Exec("UPDATE payment_transactions SET provider_ref = 'ch_unknown' WHERE id = ?", tx)
	r := w.approve(req.ID, 999)
	if r.status != http.StatusConflict || r.errCode(t) != "refund_refused" {
		t.Fatalf("refused: %d %s", r.status, r.body)
	}
	if w.status(req.ID) != "open" || w.refundRows(tx) != 0 || e.purchaseLot(w.purchaseOf(tx)).CreditsRemaining != 250 {
		t.Fatal("a refused refund must leave the request open and the credits alone")
	}
	e.app.DB.Exec("UPDATE payment_transactions SET provider_ref = ? WHERE id = ?", ref, tx)

	// A refund the provider executed but the platform never booked (crash in between): another amount is refused,
	// the same amount completes it without refunding twice.
	gateway := payment.NewSandboxA(proc, "unused", time.Minute)
	now := e.clock.Now()
	key := "refund:" + tx.String() + ":2" // the failed attempt above used :1
	row := domain.PaymentTransaction{ID: domain.NewID(), UserID: c.user.ID, Kind: domain.TxRefund, Status: domain.TxPending, AmountCents: 400,
		Currency: "USD", Provider: "sandbox_a", ParentTransactionID: &tx, Description: "Refund", IdempotencyKey: key, AttemptNo: 1, CreatedAt: now, UpdatedAt: now}
	purchase := w.purchaseOf(tx)
	row.CreditPurchaseID = &purchase
	if err := e.app.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := gateway.Refund(context.Background(), payment.RefundRequest{ProviderRef: ref, AmountCents: 400, Currency: "USD", IdempotencyKey: key}); err != nil {
		t.Fatal(err)
	}
	if r := w.approve(req.ID, 500); r.status != http.StatusConflict || r.errCode(t) != "invalid_state" || w.status(req.ID) != "open" {
		t.Fatalf("other amount while a refund is pending: %d %s", r.status, r.body)
	}
	r = w.approve(req.ID, 400)
	if r.status != http.StatusOK {
		t.Fatalf("completing the pending refund: %d %s", r.status, r.body)
	}
	if w.refundRows(tx) != 1 || w.status(req.ID) != "approved" {
		t.Fatalf("refund rows %d status %s", w.refundRows(tx), w.status(req.ID))
	}
	if linked := e.uuidOf("SELECT refund_transaction_id FROM refund_requests WHERE id = ?", req.ID); linked != row.ID {
		t.Fatalf("linked %v, the completed refund is %v", linked, row.ID)
	}
	e.invariant(c.user.ID)
}

// purchaseOf finds the purchase behind a payment.
func (w *refundWorld) purchaseOf(tx uuid.UUID) uuid.UUID {
	return w.e.uuidOf("SELECT credit_purchase_id FROM payment_transactions WHERE id = ?", tx)
}

// The refund-request rate limit is 5 an hour per user (a bucket of its own).
func TestRefundRequestsAreRateLimited(t *testing.T) {
	w := newRefundWorld(t, func(c *config.Config) { c.RateLimit.Scale = 1 })
	statuses := map[int]int{}
	for i := 0; i < 8; i++ {
		// Each attempt is a fresh submission for a payment that cannot be requested, so only the bucket decides.
		statuses[w.request(uuid.New(), "other", "", uniq("k-")).status]++
	}
	if statuses[http.StatusTooManyRequests] == 0 || statuses[http.StatusNotFound] != 5 {
		t.Fatalf("answers: %v", statuses)
	}
}
