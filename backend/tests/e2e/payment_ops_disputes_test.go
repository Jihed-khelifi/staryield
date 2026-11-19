package e2e

import (
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 6.0 (§26.5): disputes through signed provider events.

// A dispute freezes the unused credits of THAT purchase's lot (never other
// purchases or bonus credits), blocks new purchases (403 purchases_blocked,
// localized) and tells the team; a redelivered or repeated event does nothing twice.
func TestDisputeOpenedFreezesTheLotBlocksPurchasesAndIgnoresDuplicates(t *testing.T) {
	w := newOpsWorld(t)
	e, c := w.e, w.c
	if got := c.wallet().Balance; got != 800 {
		t.Fatalf("setup balance: %d", got)
	}
	open := w.event("charge.disputed", openDispute("dp_open", "needs_response"))
	w.mustSend(open)

	d := w.dispute("dp_open")
	if d.Status != "needs_response" || d.CreditsFrozen != 150 || d.CreditsRestored != 0 || d.PriorStatus != "succeeded" || d.Reason != "fraudulent" ||
		d.AmountCents != 999 || d.ActivatedAt == nil || d.ClosedAt != nil {
		t.Fatalf("dispute: %+v", d)
	}
	if got := w.txStatus(w.txA); got != "disputed" {
		t.Fatalf("payment status: %s", got)
	}
	if w.lotRemaining(w.purchaseA) != 0 || w.lotRemaining(w.purchaseB) != 600 || w.bonusRemaining() != 50 {
		t.Fatalf("only lot A may be taken back: A %d B %d bonus %d", w.lotRemaining(w.purchaseA), w.lotRemaining(w.purchaseB), w.bonusRemaining())
	}
	if got := c.wallet().Balance; got != 650 {
		t.Fatalf("balance after the freeze: %d", got)
	}
	if n := w.count("SELECT count(*) FROM credit_ledger WHERE entry_type = 'dispute_reversal' AND user_id = ? AND amount = -150", c.user.ID); n != 1 {
		t.Fatalf("dispute_reversal entries: %d", n)
	}
	e.invariant(c.user.ID)
	e.allocationsAddUp(c.user.ID)
	if !hasAudit(w, "dispute.opened") {
		t.Fatal("audit dispute.opened missing")
	}
	if mails := w.supportMail("dp_open"); len(mails) != 1 || !strings.Contains(mails[0], "dispute opened") {
		t.Fatalf("the support inbox must be told once: %v", mails)
	}

	// Purchases are paused, in the buyer's language; a replay of an earlier purchase is not blocked.
	c.locale = "fr"
	r := w.tryBuy(c)
	if r.status != http.StatusForbidden || r.errCode(t) != "purchases_blocked" || errDetail(t, r, "reason") != "dispute" {
		t.Fatalf("blocked purchase: %d %s", r.status, r.body)
	}
	if !strings.Contains(string(r.body), "litige") {
		t.Fatalf("the message must be localized: %s", r.body)
	}
	c.locale = "es"
	if r := w.tryBuy(c); !strings.Contains(string(r.body), "disputa") {
		t.Fatalf("spanish message: %s", r.body)
	}
	c.locale = ""

	// The same event delivered again: acknowledged, nothing applied twice.
	if r := w.send(open); r.status != http.StatusOK {
		t.Fatalf("redelivery: %d", r.status)
	}
	// Later reports of the same dispute (a new event each) freeze nothing more.
	w.mustSend(w.event("charge.dispute_updated", openDispute("dp_open", "under_review")))
	w.mustSend(w.event("charge.disputed", openDispute("dp_open", "needs_response"))) // stale: cannot reopen
	if d := w.dispute("dp_open"); d.Status != "under_review" || d.CreditsFrozen != 150 {
		t.Fatalf("status only moves forward: %+v", d)
	}
	if n := w.count("SELECT count(*) FROM credit_ledger WHERE entry_type = 'dispute_reversal' AND user_id = ?", c.user.ID); n != 1 {
		t.Fatalf("frozen once: %d entries", n)
	}
	if n := w.count("SELECT count(*) FROM payment_disputes WHERE transaction_id = ?", w.txA); n != 1 {
		t.Fatalf("one dispute row: %d", n)
	}
	if got := c.wallet().Balance; got != 650 {
		t.Fatalf("balance: %d", got)
	}
	// The log row of the redelivered event was applied once.
	var attempts int
	e.app.DB.Raw("SELECT attempts FROM webhook_events WHERE event_id = ?", open.ID).Scan(&attempts)
	if attempts != 1 {
		t.Fatalf("attempts of the first delivery: %d", attempts)
	}
}

// Winning restores the frozen credits into the same lot and the payment's earlier status; purchases resume.
func TestDisputeWonRestoresIntoTheSameLotAndUnblocks(t *testing.T) {
	w := newOpsWorld(t)
	c := w.c
	w.mustSend(w.event("charge.disputed", openDispute("dp_won", "needs_response")))
	if r := w.tryBuy(c); r.status != http.StatusForbidden {
		t.Fatalf("blocked while open: %d", r.status)
	}
	won := w.event("charge.dispute_closed", closedDispute("dp_won", "won"))
	w.mustSend(won)
	w.mustSend(won) // redelivered
	w.mustSend(w.event("charge.dispute_closed", closedDispute("dp_won", "won")))

	d := w.dispute("dp_won")
	if d.Status != "won" || d.CreditsFrozen != 150 || d.CreditsRestored != 150 || d.ClosedAt == nil {
		t.Fatalf("dispute: %+v", d)
	}
	if w.txStatus(w.txA) != "succeeded" {
		t.Fatalf("the earlier status is back: %s", w.txStatus(w.txA))
	}
	if w.lotRemaining(w.purchaseA) != 150 || c.wallet().Balance != 800 {
		t.Fatalf("restored: lot %d balance %d", w.lotRemaining(w.purchaseA), c.wallet().Balance)
	}
	if n := w.count("SELECT count(*) FROM credit_ledger WHERE entry_type = 'dispute_restore' AND user_id = ? AND amount = 150", c.user.ID); n != 1 {
		t.Fatalf("dispute_restore entries: %d", n)
	}
	w.e.invariant(c.user.ID)
	w.e.allocationsAddUp(c.user.ID)
	if !hasAudit(w, "dispute.won") {
		t.Fatal("audit dispute.won missing")
	}
	// A stale update after the close cannot reopen it, and purchases work again.
	w.mustSend(w.event("charge.dispute_updated", openDispute("dp_won", "needs_response")))
	if d := w.dispute("dp_won"); d.Status != "won" || w.txStatus(w.txA) != "succeeded" {
		t.Fatalf("a closed dispute stays closed: %+v", d)
	}
	if r := w.tryBuy(c); r.status != http.StatusCreated {
		t.Fatalf("purchases resume: %d %s", r.status, r.body)
	}
}

// Losing restores nothing and the payment stays disputed; the account is not blocked any more.
func TestDisputeLostKeepsTheCreditsFrozenAndTheTransactionDisputed(t *testing.T) {
	w := newOpsWorld(t)
	w.mustSend(w.event("charge.disputed", openDispute("dp_lost", "needs_response")))
	w.mustSend(w.event("charge.dispute_closed", closedDispute("dp_lost", "lost")))
	d := w.dispute("dp_lost")
	if d.Status != "lost" || d.CreditsFrozen != 150 || d.CreditsRestored != 0 || d.ClosedAt == nil {
		t.Fatalf("dispute: %+v", d)
	}
	if w.txStatus(w.txA) != "disputed" || w.c.wallet().Balance != 650 {
		t.Fatalf("lost: status %s balance %d", w.txStatus(w.txA), w.c.wallet().Balance)
	}
	if !hasAudit(w, "dispute.lost") {
		t.Fatal("audit dispute.lost missing")
	}
	mails := w.supportMail("dp_lost")
	if len(mails) != 2 || !strings.Contains(strings.Join(mails, "|"), "lost") || !strings.Contains(strings.Join(mails, "|"), "opened") {
		t.Fatalf("alerts: %v", mails)
	}
	if r := w.tryBuy(w.c); r.status != http.StatusCreated {
		t.Fatalf("a closed dispute no longer blocks purchases: %d %s", r.status, r.body)
	}
	// A refund of the disputed payment is refused: answer the dispute instead.
	w.admin.stepUp()
	refund := func(tx string) response {
		return w.admin.call(http.MethodPost, "/api/v1/admin/billing/transactions/"+tx+"/refund", map[string]any{"amount_cents": 100, "reason": "customer asked"})
	}
	if r := refund(w.txA.String()); r.status != http.StatusConflict || r.errCode(t) != "refund_disputed" {
		t.Fatalf("refund of a disputed payment: %d %s", r.status, r.body)
	}
	// The processor may know of a dispute before we do: its refusal is the same answer, and no refund row stays pending.
	w.proc.DisputeCharge(w.refB)
	if r := refund(w.txB.String()); r.status != http.StatusConflict || r.errCode(t) != "refund_disputed" {
		t.Fatalf("refund refused by the processor: %d %s", r.status, r.body)
	}
	if n := w.count("SELECT count(*) FROM payment_transactions WHERE parent_transaction_id IN (?, ?) AND status = 'pending'", w.txA, w.txB); n != 0 {
		t.Fatalf("no refund row may stay pending: %d", n)
	}
	if w.txStatus(w.txB) != "succeeded" {
		t.Fatalf("nothing was refunded: %s", w.txStatus(w.txB))
	}
}

// An inquiry freezes nothing until it becomes a dispute.
func TestInquiryFreezesNothingUntilItBecomesADispute(t *testing.T) {
	w := newOpsWorld(t)
	w.mustSend(w.event("charge.disputed", openDispute("dp_inq", "warning_needs_response")))
	d := w.dispute("dp_inq")
	if d.Status != "warning_needs_response" || d.CreditsFrozen != 0 || d.ActivatedAt != nil {
		t.Fatalf("inquiry: %+v", d)
	}
	if w.txStatus(w.txA) != "succeeded" || w.c.wallet().Balance != 800 {
		t.Fatalf("an inquiry changes nothing: %s %d", w.txStatus(w.txA), w.c.wallet().Balance)
	}
	if r := w.tryBuy(w.c); r.status != http.StatusCreated {
		t.Fatalf("an inquiry does not block purchases: %d", r.status)
	}
	// It escalates: the chargeback takes effect now (A still holds its 150, the new purchase has its own lot).
	w.mustSend(w.event("charge.dispute_updated", openDispute("dp_inq", "needs_response")))
	d = w.dispute("dp_inq")
	if d.Status != "needs_response" || d.CreditsFrozen != 150 || d.ActivatedAt == nil || w.txStatus(w.txA) != "disputed" {
		t.Fatalf("escalated: %+v", d)
	}
	// A closed inquiry never had effects to undo.
	w.mustSend(w.event("charge.dispute_closed", closedDispute("dp_inq", "won")))
	if d := w.dispute("dp_inq"); d.CreditsRestored != 150 || w.txStatus(w.txA) != "succeeded" {
		t.Fatalf("closed: %+v", d)
	}
	// An inquiry that simply closes: nothing to undo, nothing frozen.
	w.mustSend(w.event("charge.disputed", openDispute("dp_inq2", "warning_needs_response")))
	w.mustSend(w.event("charge.dispute_closed", closedDispute("dp_inq2", "warning_closed")))
	if d := w.dispute("dp_inq2"); d.Status != "warning_closed" || d.CreditsFrozen != 0 || d.ClosedAt == nil || w.txStatus(w.txA) != "succeeded" {
		t.Fatalf("closed inquiry: %+v", d)
	}
}

// A dispute we never saw open that is already lost still takes effect (out of order delivery).
func TestDisputeClosedLostBeforeItsCreationEventTakesEffectOnce(t *testing.T) {
	w := newOpsWorld(t)
	w.mustSend(w.event("charge.dispute_closed", closedDispute("dp_late", "lost")))
	if d := w.dispute("dp_late"); d.Status != "lost" || d.CreditsFrozen != 150 || w.txStatus(w.txA) != "disputed" {
		t.Fatalf("lost first: %+v", d)
	}
	w.mustSend(w.event("charge.disputed", openDispute("dp_late", "needs_response"))) // the creation event arrives last
	if d := w.dispute("dp_late"); d.Status != "lost" || d.CreditsFrozen != 150 {
		t.Fatalf("a late creation event changes nothing: %+v", d)
	}
	if n := w.count("SELECT count(*) FROM credit_ledger WHERE entry_type = 'dispute_reversal' AND user_id = ?", w.c.user.ID); n != 1 {
		t.Fatalf("reversals: %d", n)
	}
}

// The same dispute reported by many deliveries at once freezes once.
func TestDisputeEventDeliveredTwiceInParallelFreezesOnce(t *testing.T) {
	w := newOpsWorld(t)
	same := w.event("charge.disputed", openDispute("dp_race", "needs_response"))
	var wg sync.WaitGroup
	statuses := make(chan int, 16)
	for i := 0; i < 16; i++ {
		evt := same // the same event id (redeliveries racing) …
		if i%2 == 1 {
			evt = w.event("charge.disputed", openDispute("dp_race", "needs_response")) // … and fresh ids for the same dispute
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses <- w.send(evt).status
		}()
	}
	wg.Wait()
	close(statuses)
	for s := range statuses {
		if s != http.StatusOK {
			t.Fatalf("a racing delivery answered %d", s)
		}
	}
	if d := w.dispute("dp_race"); d.CreditsFrozen != 150 || d.Status != "needs_response" {
		t.Fatalf("dispute: %+v", d)
	}
	if n := w.count("SELECT count(*) FROM credit_ledger WHERE entry_type = 'dispute_reversal' AND user_id = ?", w.c.user.ID); n != 1 {
		t.Fatalf("frozen %d times", n)
	}
	if n := w.count("SELECT count(*) FROM payment_disputes WHERE transaction_id = ?", w.txA); n != 1 {
		t.Fatalf("dispute rows: %d", n)
	}
	if w.c.wallet().Balance != 650 {
		t.Fatalf("balance: %d", w.c.wallet().Balance)
	}
	w.e.invariant(w.c.user.ID)
}

// Policy combinations: clawback off freezes no credits, blocking off lets the buyer buy.
func TestDisputePoliciesClawbackAndBlocking(t *testing.T) {
	w := newOpsWorld(t)
	w.setPolicies(false, false, false, "automatic")
	w.mustSend(w.event("charge.disputed", openDispute("dp_policy", "needs_response")))
	d := w.dispute("dp_policy")
	if d.CreditsFrozen != 0 || w.txStatus(w.txA) != "disputed" || w.c.wallet().Balance != 800 {
		t.Fatalf("clawback off: %+v balance %d", d, w.c.wallet().Balance)
	}
	if r := w.tryBuy(w.c); r.status != http.StatusCreated {
		t.Fatalf("blocking off: %d %s", r.status, r.body)
	}
	// Winning with nothing frozen restores nothing but still puts the status back.
	w.mustSend(w.event("charge.dispute_closed", closedDispute("dp_policy", "won")))
	if d := w.dispute("dp_policy"); d.CreditsRestored != 0 || w.txStatus(w.txA) != "succeeded" {
		t.Fatalf("won: %+v", d)
	}
	// Blocking on, clawback off: purchases are paused but credits stay.
	w.setPolicies(true, false, false, "automatic")
	w.mustSend(w.event("charge.disputed", openDispute("dp_policy2", "needs_response")))
	if r := w.tryBuy(w.c); r.status != http.StatusForbidden || w.dispute("dp_policy2").CreditsFrozen != 0 {
		t.Fatalf("blocking on, clawback off: %d", r.status)
	}
}

// Funds withdrawn and reinstated are recorded on the dispute.
func TestDisputeFundsWithdrawnAndReinstated(t *testing.T) {
	w := newOpsWorld(t)
	w.mustSend(w.event("charge.disputed", openDispute("dp_funds", "needs_response")))
	w.mustSend(w.event("charge.dispute_funds_withdrawn", func(e *payment.WebhookEvent) { e.DisputeRef = "dp_funds" }))
	if !w.dispute("dp_funds").FundsWithdrawn {
		t.Fatal("funds withdrawn not recorded")
	}
	w.mustSend(w.event("charge.dispute_funds_reinstated", func(e *payment.WebhookEvent) { e.DisputeRef = "dp_funds" }))
	if d := w.dispute("dp_funds"); d.FundsWithdrawn || d.Status != "needs_response" {
		t.Fatalf("funds reinstated: %+v", d)
	}
}

// A membership payment that is disputed ends the membership (as before revisions 6.0), without a credit lot to claw back.
func TestDisputeOfAMembershipPaymentCancelsIt(t *testing.T) {
	e := newEnv(t)
	_, version := e.billingPlan(999, func(v *service.PlanVersionInput) { v.MonthlyCreditGrant = 100 })
	c := e.registerClient()
	card := c.attachCard("sbx_a_visa_4242_12_2035")
	res := c.subscribe(version, card.ID)
	var ref string
	e.app.DB.Raw("SELECT provider_ref FROM payment_transactions WHERE subscription_id = ?", res.Subscription.ID).Scan(&ref)
	evt := payment.WebhookEvent{ID: "evt_" + uniq("m"), Type: "charge.disputed", ProviderRef: ref, DisputeRef: uniq("dp_m"), DisputeStatus: "needs_response"}
	if r := signedWebhook(e, "sandbox_a", evt, ""); r.status != http.StatusOK {
		t.Fatalf("dispute webhook: %d %s", r.status, r.body)
	}
	if s := c.subscription(); s.Status != "canceled" {
		t.Fatalf("disputed membership: %+v", s)
	}
	var disputed int64
	e.app.DB.Raw("SELECT count(*) FROM payment_transactions WHERE subscription_id = ? AND status = 'disputed'", res.Subscription.ID).Scan(&disputed)
	if disputed != 1 {
		t.Fatalf("disputed payments: %d", disputed)
	}
	if got := c.wallet().Balance; got < 100 {
		t.Fatalf("the membership grant is not taken back by a dispute: %d", got)
	}
}
