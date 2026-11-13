package e2e

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/migrations"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 5.0: credit lots (docs/platform-extensions.md section 24.3).

// withSignupBonus gives every new account bonus credits.
func withSignupBonus(n int64) option {
	return func(c *config.Config) { c.Billing.SignupBonusCredits = n }
}

// lots returns a user's lots, oldest first.
func (e *env) lots(userID uuid.UUID) []domain.CreditLot {
	e.t.Helper()
	var out []domain.CreditLot
	if err := e.app.DB.Where("user_id = ?", userID).Order("priority, created_at, id").Find(&out).Error; err != nil {
		e.t.Fatal(err)
	}
	return out
}

// lotOf returns the single lot of a source (fails when there is not exactly one).
func (e *env) lotOf(userID uuid.UUID, source domain.LotSource) domain.CreditLot {
	e.t.Helper()
	var found []domain.CreditLot
	for _, l := range e.lots(userID) {
		if l.Source == source {
			found = append(found, l)
		}
	}
	if len(found) != 1 {
		e.t.Fatalf("want one %s lot, got %+v", source, found)
	}
	return found[0]
}

func (e *env) purchaseLot(purchaseID uuid.UUID) domain.CreditLot {
	e.t.Helper()
	var l domain.CreditLot
	if err := e.app.DB.First(&l, "reference_type = 'credit_purchase' AND reference_id = ?", purchaseID).Error; err != nil {
		e.t.Fatalf("purchase lot: %v", err)
	}
	return l
}

// invariant checks Σ credits_remaining = wallet balance and that no lot is
// outside its bounds, for the users given.
func (e *env) invariant(users ...uuid.UUID) {
	e.t.Helper()
	for _, id := range users {
		var balance, remaining int64
		e.app.DB.Raw("SELECT balance FROM wallets WHERE user_id = ?", id).Scan(&balance)
		e.app.DB.Raw("SELECT COALESCE(SUM(credits_remaining), 0) FROM credit_lots WHERE user_id = ?", id).Scan(&remaining)
		if balance != remaining {
			e.t.Fatalf("user %s: balance %d but lots hold %d (%+v)", id, balance, remaining, e.lots(id))
		}
	}
}

// allocationsAddUp checks that every lot's consumed credits and value equal the net of its allocations.
func (e *env) allocationsAddUp(userID uuid.UUID) {
	e.t.Helper()
	for _, l := range e.lots(userID) {
		var credits, value int64
		e.app.DB.Raw("SELECT COALESCE(SUM(credits), 0), COALESCE(SUM(value_millicents), 0) FROM credit_lot_allocations WHERE lot_id = ?", l.ID).
			Row().Scan(&credits, &value)
		if l.CreditsTotal-l.CreditsRemaining != credits || l.PaidCents*1000-l.ValueRemainingMillicents != value {
			e.t.Fatalf("lot %s: used %d/%d but allocations add up to %d/%d", l.ID, l.CreditsTotal-l.CreditsRemaining,
				l.PaidCents*1000-l.ValueRemainingMillicents, credits, value)
		}
	}
}

func (e *env) reconcile() service.LotReconcileReport {
	e.t.Helper()
	rep, err := e.app.Services.Wallet.ReconcileLots(context.Background())
	if err != nil {
		e.t.Fatal(err)
	}
	return rep
}

// buyWithCard purchases a pack with a saved card and returns the purchase and its payment transaction.
func (c *client) buyWithCard(credits int64, card service.PaymentMethodView) (purchase uuid.UUID, tx uuid.UUID) {
	c.e.t.Helper()
	var packs struct {
		Items []service.PackView `json:"items"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/credit-packs", nil, &packs)
	for _, p := range packs.Items {
		// Only the test packs from ensurePacks: the canonical catalogue also has
		// a 600-credit Welcome Pack, reserved to new clients (§24.2).
		if p.Credits != credits || !strings.HasPrefix(p.Code, "test_") {
			continue
		}
		var out service.PurchaseView
		c.must(http.StatusCreated, http.MethodPost, "/api/v1/credits/purchases", map[string]any{
			"pack_id": p.ID, "payment_method_id": card.ID, "idempotency_key": uniq("purchase-key-"), "consent": purchaseConsent()}, &out)
		if err := c.e.app.DB.Raw("SELECT id FROM payment_transactions WHERE credit_purchase_id = ?", out.ID).Row().Scan(&tx); err != nil {
			c.e.t.Fatalf("payment transaction of purchase %s: %v", out.ID, err)
		}
		return out.ID, tx
	}
	c.e.t.Fatalf("no pack with %d credits", credits)
	return
}

// disableSignupTrial switches the sign-up free minutes off for the test (restored at its end).
func (e *env) disableSignupTrial() {
	e.t.Helper()
	var before string
	if err := e.app.DB.Raw(`SELECT value::text FROM site_settings WHERE key = 'signup_offer'`).Scan(&before).Error; err != nil {
		e.t.Fatal(err)
	}
	set := func(v string) {
		if err := e.app.DB.Exec(`UPDATE site_settings SET value = ?::jsonb, version = version + 1 WHERE key = 'signup_offer'`, v).Error; err != nil {
			e.t.Errorf("pin signup offer: %v", err)
		}
	}
	set(`{"enabled": false, "free_minutes": 1}`)
	e.t.Cleanup(func() { set(before) })
}

func (e *env) sessionRevenue(id uuid.UUID) int64 {
	var v int64
	e.app.DB.Raw("SELECT revenue_millicents FROM chat_sessions WHERE id = ?", id).Scan(&v)
	return v
}

// Purchase → chat minutes → refund quote → admin refund of a partly used pack.
func TestLotsPurchaseChatQuoteAndAdminRefundClawsBackOnlyThatLot(t *testing.T) {
	e := newEnv(t, withSignupBonus(50))
	e.ensurePacks()
	e.disableSignupTrial()
	psychic := e.createPsychic(20)
	admin := e.createAdmin()

	// Sign-up bonus (50) + a 250-credit pack bought with a card (999 cents).
	c := e.registerClient()
	bonus := e.lotOf(c.user.ID, domain.LotBonus)
	if bonus.CreditsTotal != 50 || bonus.CreditsRemaining != 50 || bonus.PaidCents != 0 || bonus.Origin != "signup_bonus" || bonus.Priority != 2 {
		t.Fatalf("sign-up bonus lot: %+v", bonus)
	}
	card := c.attachCard("sbx_a_visa_4242_12_2035")
	purchaseID, txID := c.buyWithCard(250, card)
	lot := e.purchaseLot(purchaseID)
	if lot.Source != domain.LotPurchase || lot.CreditsTotal != 250 || lot.CreditsRemaining != 250 || lot.PaidCents != 999 ||
		lot.ValueRemainingMillicents != 999_000 || lot.Currency != "USD" || lot.Priority != 0 {
		t.Fatalf("purchase lot: %+v", lot)
	}
	e.invariant(c.user.ID)

	// A chat at 20 credits per minute spends the purchased credits first.
	_, _, cws, pws, s := startedSessionOn(t, e, psychic, c)
	_, _ = cws, pws
	e.tick(time.Minute + time.Second)
	e.tick(time.Minute)
	e.tick(time.Minute) // minutes 2, 3 and 4: 80 credits so far
	lot = e.purchaseLot(purchaseID)
	if lot.CreditsRemaining != 170 || e.lotOf(c.user.ID, domain.LotBonus).CreditsRemaining != 50 {
		t.Fatalf("chat must consume purchased credits first: purchase %d bonus %d", lot.CreditsRemaining, e.lotOf(c.user.ID, domain.LotBonus).CreditsRemaining)
	}
	// The session's revenue basis is exactly the value that left the lot.
	if got, want := e.sessionRevenue(s.ID), 999_000-lot.ValueRemainingMillicents; got != want || got == 0 {
		t.Fatalf("session revenue %d, lot value consumed %d", got, want)
	}
	e.invariant(c.user.ID)
	e.allocationsAddUp(c.user.ID)

	// The refund quote of the Refund Policy: floor(170 × 999 ÷ 250) = 679 cents.
	quote := domain.RefundableCents(lot.CreditsTotal, lot.CreditsRemaining, lot.PaidCents, 0)
	if quote != 679 {
		t.Fatalf("quote %d, want 679", quote)
	}

	// The admin refunds exactly the quote: the money is what the admin typed,
	// the clawback is that lot's remaining credits and nothing else.
	admin.stepUp()
	refund := func(amount int64) service.AdminTransactionRow {
		var row service.AdminTransactionRow
		admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/billing/transactions/"+txID.String()+"/refund",
			map[string]any{"amount_cents": amount, "reason": "Unused credits"}, &row)
		return row
	}
	row := refund(quote)
	if row.RefundedAmountCents != 679 || row.Status != "partially_refunded" {
		t.Fatalf("refund row: %+v", row)
	}
	lot = e.purchaseLot(purchaseID)
	if want := domain.ClawbackCredits(250, 170, 999, 679); want != 170 || lot.CreditsRemaining != 0 || lot.ValueRemainingMillicents != 0 {
		t.Fatalf("clawback %d; lot after refund %+v", want, lot)
	}
	if w := c.wallet(); w.Balance != 50 { // the bonus credits are untouched
		t.Fatalf("wallet after refund: %+v", w)
	}
	if got := e.lotOf(c.user.ID, domain.LotBonus).CreditsRemaining; got != 50 {
		t.Fatalf("bonus lot touched by the refund: %d", got)
	}
	var childAmount int64
	e.app.DB.Raw("SELECT amount_cents FROM payment_transactions WHERE parent_transaction_id = ? AND kind = 'refund'", txID).Scan(&childAmount)
	if childAmount != 679 {
		t.Fatalf("refund transaction amount %d", childAmount)
	}
	e.invariant(c.user.ID)

	// A refund larger than the unused value (a technical problem) books the money and takes only what is left: nothing.
	row = refund(100)
	if row.RefundedAmountCents != 779 {
		t.Fatalf("second refund: %+v", row)
	}
	if got := e.lotOf(c.user.ID, domain.LotBonus).CreditsRemaining; got != 50 || e.purchaseLot(purchaseID).CreditsRemaining != 0 {
		t.Fatal("a refund beyond the unused value must not take other credits")
	}
	if rep := e.reconcile(); rep.Mismatched != 0 {
		t.Fatalf("reconciliation: %+v", rep)
	}
	e.invariant(c.user.ID)
	e.allocationsAddUp(c.user.ID)
}

// startedSessionOn starts a live chat for an existing client and advisor
// (the steps of startedSessionWith, without registering or buying).
func startedSessionOn(t *testing.T, e *env, psychic, client *client) (*client, *client, *ws, *ws, service.SessionView) {
	t.Helper()
	pws := psychic.goOnline()
	cws := client.connect("")
	var requested service.SessionView
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions", map[string]any{
		"psychic_slug": psychic.slug(), "request_key": uniq("req-key-")}, &requested)
	pws.waitFor(realtime.EventSessionRequested, 5*time.Second)
	cws.mustAck(realtime.FrameSessionJoin, map[string]any{"session_id": requested.ID})
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+requested.ID.String()+"/accept", nil, nil)
	pws.mustAck(realtime.FrameSessionJoin, map[string]any{"session_id": requested.ID})
	started := sessionFrom(t, cws.waitFor(realtime.EventSessionStarted, 5*time.Second).Data)
	pws.waitFor(realtime.EventSessionStarted, 5*time.Second)
	if started.Status != "active" || started.BilledMinutes != 1 {
		t.Fatalf("started: %+v", started)
	}
	return client, psychic, cws, pws, started
}

// Purchased credits are spent before membership and bonus credits, whatever
// the order they arrived in; admin adjustments follow the same rules.
func TestLotsConsumptionOrderAndAdminAdjustments(t *testing.T) {
	e := newEnv(t, withSignupBonus(50))
	e.ensurePacks()
	admin := e.createAdmin()
	c := e.registerClient()
	ctx := context.Background()
	adjust := func(amount int64) {
		t.Helper()
		if _, err := e.app.Services.Admin.AdjustCredits(ctx, admin.user.ID, c.user.ID, amount, "test", uniq("adj-key-")); err != nil {
			t.Fatalf("adjust %d: %v", amount, err)
		}
	}
	card := c.attachCard("sbx_a_visa_4242_12_2035")
	purchaseID, _ := c.buyWithCard(250, card)
	adjust(30) // courtesy credits: a second bonus lot
	if lots := e.lots(c.user.ID); len(lots) != 3 {
		t.Fatalf("want signup bonus + purchase + courtesy lots, got %+v", lots)
	}
	e.invariant(c.user.ID)

	adjust(-100) // a negative adjustment spends by priority: purchased credits first
	if got := e.purchaseLot(purchaseID); got.CreditsRemaining != 150 {
		t.Fatalf("purchase lot after -100: %+v", got)
	}
	adjust(-170) // 150 purchased, then 20 from the oldest bonus lot (the sign-up bonus)
	if got := e.purchaseLot(purchaseID); got.CreditsRemaining != 0 || got.ValueRemainingMillicents != 0 {
		t.Fatalf("purchase lot spent: %+v", got)
	}
	var signup, courtesy domain.CreditLot
	for _, l := range e.lots(c.user.ID) {
		if l.Source == domain.LotBonus && l.Origin == "signup_bonus" {
			signup = l
		}
		if l.Source == domain.LotBonus && l.Origin == "admin_adjustment" {
			courtesy = l
		}
	}
	if signup.CreditsRemaining != 30 || courtesy.CreditsRemaining != 30 || courtesy.PaidCents != 0 {
		t.Fatalf("bonus lots: signup %+v courtesy %+v", signup, courtesy)
	}
	if w := c.wallet(); w.Balance != 60 {
		t.Fatalf("balance %d", w.Balance)
	}
	// The adjustment is idempotent: a retry of the same key changes nothing.
	key := uniq("adj-key-")
	for i := 0; i < 2; i++ {
		if _, err := e.app.Services.Admin.AdjustCredits(ctx, admin.user.ID, c.user.ID, -10, "retry", key); err != nil {
			t.Fatal(err)
		}
	}
	if w := c.wallet(); w.Balance != 50 {
		t.Fatalf("a replayed adjustment was applied twice: %d", w.Balance)
	}
	// Overdrawing is refused and leaves the lots alone.
	if _, err := e.app.Services.Admin.AdjustCredits(ctx, admin.user.ID, c.user.ID, -51, "too much", uniq("adj-key-")); err == nil {
		t.Fatal("overdraw accepted")
	}
	e.invariant(c.user.ID)
	e.allocationsAddUp(c.user.ID)
}

// A held custom request takes its credits from the lots by priority and a
// refund returns them to the very same lots, whichever way the order ends.
func TestLotsCustomRequestHoldRefundRestoresSameLots(t *testing.T) {
	e := newEnv(t, withSignupBonus(50))
	e.ensurePacks()
	psychic := e.createPsychic(30)
	admin := e.createAdmin()
	var svc service.ServiceView
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/psychics/"+psychic.user.ID.String()+"/services",
		map[string]any{"name": "Dream Decode", "subtitle": "Symbolic Analysis", "description": "Send your dream.", "price_credits": 280}, &svc)

	c := e.registerClient()
	card := c.attachCard("sbx_a_visa_4242_12_2035")
	purchaseID, _ := c.buyWithCard(250, card)
	order := func() service.CustomRequestView {
		var o service.CustomRequestView
		c.must(http.StatusCreated, http.MethodPost, "/api/v1/custom-requests", map[string]any{"service_id": svc.ID, "idempotency_key": uniq("order-key-")}, &o)
		return o
	}
	requestRevenue := func(id uuid.UUID) int64 {
		var v int64
		e.app.DB.Raw("SELECT revenue_millicents FROM custom_requests WHERE id = ?", id).Scan(&v)
		return v
	}
	before := e.lots(c.user.ID)

	// 280 credits = the whole 250 pack + 30 of the sign-up bonus.
	o := order()
	if p := e.purchaseLot(purchaseID); p.CreditsRemaining != 0 || p.ValueRemainingMillicents != 0 {
		t.Fatalf("hold must spend the purchased lot: %+v", p)
	}
	if b := e.lotOf(c.user.ID, domain.LotBonus); b.CreditsRemaining != 20 {
		t.Fatalf("bonus after hold: %+v", b)
	}
	if got := requestRevenue(o.ID); got != 999_000 {
		t.Fatalf("request revenue %d, want the whole pack's value 999000", got)
	}
	e.invariant(c.user.ID)

	// Cancelling restores exactly: same lots, same value, request revenue back to 0.
	c.must(http.StatusOK, http.MethodPost, "/api/v1/custom-requests/"+o.ID.String()+"/cancel", nil, nil)
	assertSameLots := func(when string) {
		t.Helper()
		after := e.lots(c.user.ID)
		if len(after) != len(before) {
			t.Fatalf("%s: lots %d, want %d (%+v)", when, len(after), len(before), after)
		}
		for i := range before {
			if after[i].ID != before[i].ID || after[i].CreditsRemaining != before[i].CreditsRemaining ||
				after[i].ValueRemainingMillicents != before[i].ValueRemainingMillicents {
				t.Fatalf("%s: lot %s is %d/%d, was %d/%d", when, after[i].ID, after[i].CreditsRemaining, after[i].ValueRemainingMillicents,
					before[i].CreditsRemaining, before[i].ValueRemainingMillicents)
			}
		}
	}
	assertSameLots("after cancel")
	if requestRevenue(o.ID) != 0 {
		t.Fatalf("a refunded hold keeps no revenue: %d", requestRevenue(o.ID))
	}

	// Decline restores the same way (expiry is checked at the end).
	o2 := order()
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/custom-requests/"+o2.ID.String()+"/decline", map[string]any{"reason": "Fully booked"}, nil)
	assertSameLots("after decline")

	// Delivery releases the hold (no lot change, revenue stays); a dispute refund restores the same lots.
	o3 := order()
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/custom-requests/"+o3.ID.String()+"/accept", nil, nil)
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/custom-requests/"+o3.ID.String()+"/deliver", map[string]any{"delivery": "Your answer."}, nil)
	if requestRevenue(o3.ID) != 999_000 || e.purchaseLot(purchaseID).CreditsRemaining != 0 {
		t.Fatal("releasing the hold must leave lots and revenue as they were")
	}
	c.must(http.StatusOK, http.MethodPost, "/api/v1/custom-requests/"+o3.ID.String()+"/dispute", map[string]any{"reason": "Not what I asked for."}, nil)
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/custom-requests/"+o3.ID.String()+"/resolve", map[string]any{"refund": true, "note": "Goodwill"}, nil)
	assertSameLots("after dispute refund")
	if requestRevenue(o3.ID) != 0 {
		t.Fatalf("a refunded delivery keeps no revenue: %d", requestRevenue(o3.ID))
	}
	if w := c.wallet(); w.Balance != 300 || w.Held != 0 {
		t.Fatalf("wallet: %+v", w)
	}
	// Expiry restores the same way.
	order()
	e.clock.Advance(49 * time.Hour)
	e.maintenance()
	assertSameLots("after expiry")
	e.invariant(c.user.ID)
	e.allocationsAddUp(c.user.ID)
	// Nothing was minted: restores found their allocations.
	var legacy int64
	e.app.DB.Raw("SELECT count(*) FROM credit_lots WHERE user_id = ? AND origin = 'legacy_return'", c.user.ID).Scan(&legacy)
	if legacy != 0 {
		t.Fatalf("%d legacy_return lots", legacy)
	}
}

// A membership grant is a lot of its own; refunding the payment reverses only
// that grant's lot and never purchased credits.
func TestLotsMembershipGrantAndReversal(t *testing.T) {
	proc := payment.NewSandboxProcessor()
	e := newEnvWithProcessor(t, proc)
	e.ensurePacks()
	_, version := e.billingPlan(999, func(v *service.PlanVersionInput) { v.MonthlyCreditGrant = 100 })
	admin := e.createAdmin()
	c := e.registerClient()
	card := c.attachCard("sbx_a_visa_4242_12_2035")
	purchaseID, _ := c.buyWithCard(250, card)
	res := c.subscribe(version, card.ID)
	if res.Transaction == nil {
		t.Fatal("no initial transaction")
	}
	grant := e.lotOf(c.user.ID, domain.LotMembership)
	if grant.CreditsTotal != 100 || grant.CreditsRemaining != 100 || grant.PaidCents != 0 || grant.Priority != 1 ||
		grant.ReferenceType != "subscription_grant" || grant.Origin != "subscription_grant" {
		t.Fatalf("membership lot: %+v", grant)
	}
	e.invariant(c.user.ID)

	admin.stepUp()
	refund := func(amount int64) {
		t.Helper()
		admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/billing/transactions/"+res.Transaction.ID.String()+"/refund",
			map[string]any{"amount_cents": amount, "reason": "Goodwill"}, nil)
	}
	refund(500) // 100 × 500 ÷ 999 = 50 credits, from the membership lot only
	if g, p := e.lotOf(c.user.ID, domain.LotMembership), e.purchaseLot(purchaseID); g.CreditsRemaining != 50 || p.CreditsRemaining != 250 {
		t.Fatalf("partial reversal: membership %d purchase %d", g.CreditsRemaining, p.CreditsRemaining)
	}
	e.invariant(c.user.ID)

	// Spend 260 through an admin adjustment: 250 purchased, then 10 membership credits.
	if _, err := e.app.Services.Admin.AdjustCredits(context.Background(), admin.user.ID, c.user.ID, -260, "spend", uniq("adj-key-")); err != nil {
		t.Fatal(err)
	}
	if g := e.lotOf(c.user.ID, domain.LotMembership); g.CreditsRemaining != 40 {
		t.Fatalf("membership after spending: %+v", g)
	}
	refund(499) // 50 credits remain to reverse, but only 40 are left in the lot
	if g := e.lotOf(c.user.ID, domain.LotMembership); g.CreditsRemaining != 0 {
		t.Fatalf("membership after full refund: %+v", g)
	}
	if w := c.wallet(); w.Balance != 0 {
		t.Fatalf("balance %d", w.Balance)
	}
	e.invariant(c.user.ID)
	e.allocationsAddUp(c.user.ID)
}

// Many goroutines buying, charging, restoring and replaying for the same
// users never break Σ remaining = balance, never overspend a lot and never
// write an allocation twice.
func TestLotsConcurrencyKeepsTheInvariant(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	tx := database.NewTxManager(e.app.DB)
	wallet := e.app.Services.Wallet
	users := []uuid.UUID{e.registerClient().user.ID, e.registerClient().user.ID}

	apply := func(m service.Movement) (applied bool, err error) {
		err = tx.WithinTx(ctx, func(ctx context.Context) error {
			_, applied, err = wallet.Apply(ctx, m)
			return err
		})
		return
	}
	// Seed each user with a purchase and a bonus lot.
	for _, u := range users {
		pid := domain.NewID()
		if _, err := apply(service.Movement{UserID: u, Type: domain.LedgerPurchase, Amount: 500, ReferenceType: "credit_purchase", ReferenceID: &pid,
			IdempotencyKey: "purchase:" + pid.String(), Lot: &service.LotSpec{Source: domain.LotPurchase, PaidCents: 1999, Currency: "USD"}}); err != nil {
			t.Fatal(err)
		}
		if _, err := apply(service.Movement{UserID: u, Type: domain.LedgerSignupBonus, Amount: 100, ReferenceType: "user",
			IdempotencyKey: "bonus:" + u.String(), Lot: &service.LotSpec{Source: domain.LotBonus}}); err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	var debits, buys, holds, refunds, replays, insufficient atomic.Int64
	for g := 0; g < 120; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			u := users[g%2]
			key := fmt.Sprintf("conc:%d", g)
			switch g % 6 {
			case 0, 1, 2: // chat minutes (some replayed concurrently)
				m := service.Movement{UserID: u, Type: domain.LedgerChatCharge, Amount: -int64(1 + g%7), ReferenceType: "chat_session", IdempotencyKey: key}
				applied, err := apply(m)
				switch {
				case err == service.ErrInsufficientFunds:
					insufficient.Add(1)
				case err != nil:
					t.Errorf("debit: %v", err)
				case applied:
					debits.Add(1)
				}
				if g%3 == 0 {
					if again, err := apply(m); err != service.ErrInsufficientFunds && (err != nil || again) {
						t.Errorf("replay applied=%v err=%v", again, err)
					} else {
						replays.Add(1)
					}
				}
			case 3: // purchases
				pid := domain.NewID()
				if _, err := apply(service.Movement{UserID: u, Type: domain.LedgerPurchase, Amount: 40, ReferenceType: "credit_purchase", ReferenceID: &pid,
					IdempotencyKey: "purchase:" + pid.String(), Lot: &service.LotSpec{Source: domain.LotPurchase, PaidCents: 133, Currency: "USD"}}); err != nil {
					t.Errorf("purchase: %v", err)
				} else {
					buys.Add(1)
				}
			case 4: // a hold and its refund
				hold := service.Movement{UserID: u, Type: domain.LedgerHold, Amount: -25, HeldAmount: 25, ReferenceType: "custom_request", IdempotencyKey: "hold:" + key}
				if applied, err := apply(hold); err == nil && applied {
					holds.Add(1)
					if _, err := apply(service.Movement{UserID: u, Type: domain.LedgerHoldRefund, Amount: 25, HeldAmount: -25,
						ReferenceType: "custom_request", IdempotencyKey: "refund:" + key, RestoreKey: "hold:" + key}); err != nil {
						t.Errorf("restore: %v", err)
					} else {
						refunds.Add(1)
					}
				} else if err != nil && err != service.ErrInsufficientFunds {
					t.Errorf("hold: %v", err)
				}
			case 5: // courtesy credits
				if _, err := apply(service.Movement{UserID: u, Type: domain.LedgerAdminAdjustment, Amount: 5, ReferenceType: "admin",
					IdempotencyKey: "courtesy:" + key, Lot: &service.LotSpec{Source: domain.LotBonus}}); err != nil {
					t.Errorf("courtesy: %v", err)
				}
			}
		}(g)
	}
	wg.Wait()
	if debits.Load() == 0 || buys.Load() == 0 || holds.Load() == 0 || refunds.Load() != holds.Load() {
		t.Fatalf("scenario did not run: debits %d buys %d holds %d refunds %d", debits.Load(), buys.Load(), holds.Load(), refunds.Load())
	}
	for _, u := range users {
		e.invariant(u)
		e.allocationsAddUp(u)
		var balance, held, ledgerSum, ledgerHeld int64
		e.app.DB.Raw("SELECT balance, held FROM wallets WHERE user_id = ?", u).Row().Scan(&balance, &held)
		e.app.DB.Raw("SELECT COALESCE(SUM(amount), 0), COALESCE(SUM(held_amount), 0) FROM credit_ledger WHERE user_id = ?", u).Row().Scan(&ledgerSum, &ledgerHeld)
		if balance != ledgerSum || held != ledgerHeld || held != 0 {
			t.Fatalf("wallet %d/%d against ledger %d/%d", balance, held, ledgerSum, ledgerHeld)
		}
		// Each ledger entry's allocations add up to its amount (debits) or are within it (restores).
		var bad int64
		e.app.DB.Raw(`SELECT count(*) FROM credit_ledger l
			JOIN (SELECT ledger_entry_id, SUM(credits) AS c FROM credit_lot_allocations WHERE user_id = ? GROUP BY ledger_entry_id) a ON a.ledger_entry_id = l.id
			WHERE (l.amount < 0 AND a.c <> -l.amount) OR (l.amount > 0 AND a.c <> -l.amount)`, u).Scan(&bad)
		if bad != 0 {
			t.Fatalf("%d ledger entries whose allocations do not match their amount", bad)
		}
	}

	// Contention on an almost empty wallet: exactly as many debits succeed as the
	// credits allow, the rest are insufficient funds, and nothing goes negative.
	poor := e.registerClient().user.ID
	if _, err := apply(service.Movement{UserID: poor, Type: domain.LedgerSignupBonus, Amount: 100, ReferenceType: "user",
		IdempotencyKey: "bonus:" + poor.String(), Lot: &service.LotSpec{Source: domain.LotBonus}}); err != nil {
		t.Fatal(err)
	}
	var won, lost atomic.Int64
	for g := 0; g < 80; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			applied, err := apply(service.Movement{UserID: poor, Type: domain.LedgerChatCharge, Amount: -3, ReferenceType: "chat_session",
				IdempotencyKey: fmt.Sprintf("poor:%d", g)})
			switch {
			case err == service.ErrInsufficientFunds:
				lost.Add(1)
			case err != nil || !applied:
				t.Errorf("poor debit: applied=%v err=%v", applied, err)
			default:
				won.Add(1)
			}
		}(g)
	}
	wg.Wait()
	if won.Load() != 33 || lost.Load() != 47 {
		t.Fatalf("33 debits of 3 fit in 100 credits, got %d won, %d lost", won.Load(), lost.Load())
	}
	e.invariant(poor)
	e.allocationsAddUp(poor)
	if rep := e.reconcile(); rep.Wallets == 0 {
		t.Fatalf("reconciliation scanned nothing: %+v", rep)
	}
	t.Logf("debits %d (insufficient %d), purchases %d, holds %d, replays %d", debits.Load(), insufficient.Load(), buys.Load(), holds.Load(), replays.Load())
}

// The reconciliation job reports a wallet whose lots disagree and never
// changes the balance.
func TestLotsReconciliationJobReportsAndNeverRepairs(t *testing.T) {
	e := newEnv(t, withSignupBonus(40))
	c := e.registerClient()
	before := e.reconcile()
	// Credits no lot accounts for: a direct balance edit (what a bug or a manual SQL fix would do).
	if err := e.app.DB.Exec("UPDATE wallets SET balance = balance + 7 WHERE user_id = ?", c.user.ID).Error; err != nil {
		t.Fatal(err)
	}
	after := e.reconcile()
	if after.Mismatched != before.Mismatched+1 || after.Drift != before.Drift+7 {
		t.Fatalf("report before %+v after %+v", before, after)
	}
	if w := c.wallet(); w.Balance != 47 {
		t.Fatalf("the job changed a balance: %d", w.Balance)
	}
	// The scheduler job is registered and runs clean.
	if err := e.app.Scheduler.RunJob(context.Background(), service.LotReconcileJob); err != nil {
		t.Fatal(err)
	}
	// A chat or purchase for the broken wallet still works: the balance is authoritative.
	if _, err := e.app.Services.Admin.AdjustCredits(context.Background(), e.createAdmin().user.ID, c.user.ID, -45, "spend", uniq("adj-key-")); err != nil {
		t.Fatalf("debit on a wallet the lots do not cover: %v", err)
	}
	if w := c.wallet(); w.Balance != 2 {
		t.Fatalf("balance %d", w.Balance)
	}
	if err := e.app.DB.Exec("UPDATE wallets SET balance = 0 WHERE user_id = ?", c.user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if rep := e.reconcile(); rep.Mismatched != before.Mismatched {
		t.Fatalf("after fixing: %+v, started at %+v", rep, before)
	}
}

// The credit_lot_allocations table is append-only, like the ledger.
func TestLotsAllocationsAreAppendOnly(t *testing.T) {
	e := newEnv(t, withSignupBonus(10))
	c := e.registerClient()
	if _, err := e.app.Services.Admin.AdjustCredits(context.Background(), e.createAdmin().user.ID, c.user.ID, -4, "spend", uniq("adj-key-")); err != nil {
		t.Fatal(err)
	}
	if err := e.app.DB.Exec("UPDATE credit_lot_allocations SET credits = credits + 1 WHERE user_id = ?", c.user.ID).Error; err == nil {
		t.Fatal("allocations were updated")
	}
	if err := e.app.DB.Exec("DELETE FROM credit_lot_allocations WHERE user_id = ?", c.user.ID).Error; err == nil {
		t.Fatal("allocations were deleted")
	}
	// The lot constraints hold at the database too.
	lot := e.lotOf(c.user.ID, domain.LotBonus)
	for _, bad := range []string{
		"credits_remaining = credits_total + 1", "credits_remaining = -1", "value_remaining_millicents = 1", "paid_cents = 5",
	} {
		if err := e.app.DB.Exec("UPDATE credit_lots SET "+bad+" WHERE id = ?", lot.ID).Error; err == nil {
			t.Fatalf("lot constraint %q not enforced", bad)
		}
	}
}

// Migration 0025 backfills lots for wallets that predate them: purchases newest
// first against the balance, the rest as one legacy bonus lot, and Σ remaining
// equals the balance for every wallet.
func TestLotsMigrationBackfill(t *testing.T) {
	admin, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := admin.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	name := "staryield_lots_" + uuid.NewString()[:8]
	if err := admin.Exec("CREATE DATABASE " + name).Error; err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() { admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)") })
	u, _ := url.Parse(testDSN)
	u.Path = "/" + name
	e := newEnv(t, func(c *config.Config) { c.Database.DSN = u.String() }, withSignupBonus(50))
	e.ensurePacks()
	adminSvc := e.app.Services.Admin
	ctx := context.Background()
	adminID := e.createAdmin().user.ID

	// A: three purchases (250, 600, 250), part spent. B: bonus only. C: one purchase refunded in full.
	// D: purchase fully spent + signup bonus. E: nothing. F: one purchase and courtesy credits.
	a, b, cc, d, f, empty := e.registerClient(), e.registerClient(), e.registerClient(), e.registerClient(), e.registerClient(), e.registerClient()
	_ = empty
	cardOf := func(c *client) service.PaymentMethodView { return c.attachCard("sbx_a_visa_4242_12_2035") }
	var aPurchases []uuid.UUID
	ca := cardOf(a)
	for _, pack := range []int64{250, 600, 250} {
		id, _ := a.buyWithCard(pack, ca)
		aPurchases = append(aPurchases, id)
		e.clock.Advance(time.Minute)
	}
	if _, err := adminSvc.AdjustCredits(ctx, adminID, a.user.ID, -200, "spend", uniq("adj-key-")); err != nil {
		t.Fatal(err)
	}
	if _, err := adminSvc.AdjustCredits(ctx, adminID, b.user.ID, 10, "courtesy", uniq("adj-key-")); err != nil {
		t.Fatal(err)
	}
	cPurchase, _ := cc.buyWithCard(250, cardOf(cc))
	e.app.DB.Exec("UPDATE credit_purchases SET status = 'refunded' WHERE id = ?", cPurchase) // refunded purchases get no lot
	dPurchase, _ := d.buyWithCard(250, cardOf(d))
	if _, err := adminSvc.AdjustCredits(ctx, adminID, d.user.ID, -250, "spend", uniq("adj-key-")); err != nil {
		t.Fatal(err)
	}
	fPurchase, _ := f.buyWithCard(250, cardOf(f))
	if _, err := adminSvc.AdjustCredits(ctx, adminID, f.user.ID, 100, "courtesy", uniq("adj-key-")); err != nil {
		t.Fatal(err)
	}

	// Rewind to the state before 0025 (the lots and the new columns do not exist).
	for _, sql := range []string{
		"DROP TABLE credit_lot_allocations", "DROP TABLE credit_lots",
		"ALTER TABLE chat_sessions DROP COLUMN revenue_millicents", "ALTER TABLE custom_requests DROP COLUMN revenue_millicents",
		"DELETE FROM schema_migrations WHERE version = '0025_credit_lots'",
	} {
		if err := e.app.DB.Exec(sql).Error; err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	start := time.Now()
	applied, err := migrations.Up(ctx, e.app.DB, zap.NewNop())
	if err != nil || len(applied) != 1 || applied[0] != "0025_credit_lots" {
		t.Fatalf("migrate: %v %v", applied, err)
	}
	t.Logf("backfill of the 0025 migration took %s", time.Since(start))

	// A: balance 50 + 1100 − 200 = 950: newest purchase 250, then 600, then 100 of the oldest. No bonus remainder.
	want := map[uuid.UUID]int64{aPurchases[2]: 250, aPurchases[1]: 600, aPurchases[0]: 100}
	for id, remaining := range want {
		l := e.purchaseLot(id)
		if l.CreditsRemaining != remaining || l.Source != domain.LotPurchase || l.CreditsTotal == 0 || l.LedgerEntryID == nil {
			t.Fatalf("A purchase %s: %+v, want remaining %d", id, l, remaining)
		}
		if exact := remaining * l.PaidCents * 1000 / l.CreditsTotal; l.ValueRemainingMillicents != exact {
			t.Fatalf("A purchase %s value %d, want floor(%d×%d×1000÷%d) = %d", id, l.ValueRemainingMillicents, remaining, l.PaidCents, l.CreditsTotal, exact)
		}
		var key string
		e.app.DB.Raw("SELECT idempotency_key FROM credit_ledger WHERE id = ?", *l.LedgerEntryID).Scan(&key)
		if key != "purchase:"+id.String() {
			t.Fatalf("lot linked to ledger entry %q", key)
		}
	}
	if len(e.lots(a.user.ID)) != 3 {
		t.Fatalf("A lots: %+v", e.lots(a.user.ID))
	}
	// B: no purchases → one legacy bonus lot with the whole balance (50 + 10).
	if bl := e.lots(b.user.ID); len(bl) != 1 || bl[0].Source != domain.LotBonus || bl[0].Origin != "legacy_backfill" || bl[0].CreditsRemaining != 60 ||
		bl[0].LedgerEntryID != nil || bl[0].PaidCents != 0 {
		t.Fatalf("B lots: %+v", bl)
	}
	// C: the refunded purchase gets no lot; the 50 bonus credits are a legacy lot.
	if cl := e.lots(cc.user.ID); len(cl) != 1 || cl[0].Origin != "legacy_backfill" || cl[0].CreditsRemaining != 300 {
		t.Fatalf("C lots: %+v", cl)
	}
	// D: spent purchase, balance 50 → the newest-first rule puts the 50 on the purchase.
	if l := e.purchaseLot(dPurchase); l.CreditsRemaining != 50 || l.CreditsTotal != 250 {
		t.Fatalf("D purchase lot: %+v", l)
	}
	// F: purchase 250 intact, 100 + 50 courtesy and bonus → one legacy lot of 150.
	if l := e.purchaseLot(fPurchase); l.CreditsRemaining != 250 || l.ValueRemainingMillicents != l.PaidCents*1000 {
		t.Fatalf("F purchase lot: %+v", l)
	}
	if l := e.lotOf(f.user.ID, domain.LotBonus); l.CreditsRemaining != 150 || l.Origin != "legacy_backfill" {
		t.Fatalf("F legacy lot: %+v", l)
	}
	// E has a wallet with the sign-up bonus only → legacy lot; nobody has a lot with credits beyond the balance.
	// The invariant holds for every wallet, and the job agrees.
	var mismatched int64
	e.app.DB.Raw(`SELECT count(*) FROM wallets w WHERE w.balance <> COALESCE((SELECT SUM(credits_remaining) FROM credit_lots l WHERE l.user_id = w.user_id), 0)`).Scan(&mismatched)
	if mismatched != 0 {
		t.Fatalf("%d wallets disagree with their lots after the backfill", mismatched)
	}
	if rep := e.reconcile(); rep.Mismatched != 0 {
		t.Fatalf("reconciliation after backfill: %+v", rep)
	}
	// The migrated database serves lot-based movements: a chat-style debit continues from the backfilled lots.
	if _, err := adminSvc.AdjustCredits(ctx, adminID, a.user.ID, -300, "after", uniq("adj-key-")); err != nil {
		t.Fatal(err)
	}
	// Consumption is FIFO inside a source: the oldest purchase's remaining 100, then 200 of the next.
	if g0, g1, g2 := e.purchaseLot(aPurchases[0]), e.purchaseLot(aPurchases[1]), e.purchaseLot(aPurchases[2]); g0.CreditsRemaining != 0 ||
		g1.CreditsRemaining != 400 || g2.CreditsRemaining != 250 {
		t.Fatalf("after spending 300: %d %d %d", g0.CreditsRemaining, g1.CreditsRemaining, g2.CreditsRemaining)
	}
	// Idempotent: running the migrations again does nothing.
	if applied, err := migrations.Up(ctx, e.app.DB, zap.NewNop()); err != nil || len(applied) != 0 {
		t.Fatalf("second run: %v %v", applied, err)
	}
	e.invariant(a.user.ID, b.user.ID, cc.user.ID, d.user.ID, f.user.ID)
}
