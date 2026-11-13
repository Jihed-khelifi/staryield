package e2e

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 5.0: revenue analytics (§24.5).
//
// The shared test database holds the data of earlier tests, so this test lives
// in its own year (the fake clock is set to June 2031): every windowed figure
// of the report can then be computed by hand from what this test does. The
// deferred figure is not windowed (it is what is unconsumed at the end of the
// window), so it is checked as "everything else already in the database plus
// what this test left", the "everything else" read straight from the lots.

// pinSignupTrial gives every new account the given number of free minutes (set before any read of the setting).
func (e *env) pinSignupTrial(minutes int) {
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
	set(`{"enabled": true, "free_minutes": ` + string(rune('0'+minutes)) + `}`)
	e.t.Cleanup(func() { set(before) })
}

// processing is the processing estimate of one purchase with the default assumptions (8 % + 30 cents, half-up per purchase).
func processing(price int64) int64 { return (price*800+5000)/10000 + 30 }

func TestRevenueReportMatchesHandComputedFigures(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	e.pinSignupTrial(4)
	// Earlier tests leave sessions open in the shared database. Moving the clock to 2031 would make this test's
	// billing tick bill them (and credit their advisors) inside the window, so they are closed first.
	if err := e.app.DB.Exec(`UPDATE chat_sessions SET status = 'cancelled' WHERE status IN ('requested', 'accepted', 'active', 'disconnected')`).Error; err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2031, 6, 10, 12, 0, 0, 0, time.UTC)
	e.clock.Set(t0)
	ctx := context.Background()
	cfg := e.app.Config

	psychic := e.createPsychic(20)
	admin := e.createAdmin()
	var svc service.ServiceView
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/psychics/"+psychic.user.ID.String()+"/services",
		map[string]any{"name": "Dream Decode", "subtitle": "Symbolic Analysis", "description": "Send your dream.", "price_credits": 100}, &svc)

	// Five sign-ups with free minutes: t1 (buys the 1500-credit pack), t2 (never buys), c1, c2 and t3.
	t1, t2, c1, c2, t3 := e.registerClient(), e.registerClient(), e.registerClient(), e.registerClient(), e.registerClient()
	_ = t2
	e.app.DB.Exec("UPDATE wallets SET free_minutes = 0 WHERE user_id = ?", c1.user.ID) // c1's chat must spend credits
	waitFor(t, "user.registered events", func() bool {
		var n int64
		e.app.DB.Raw(`SELECT count(DISTINCT user_id) FROM analytics_events WHERE name = 'user.registered' AND occurred_at >= '2031-06-01'`).Scan(&n)
		return n == 5
	})

	t1.buyWithCard(1500, t1.attachCard("sbx_a_visa_4242_12_2035"))                 // 3999 cents (the 113-cent pack is declined by the sandbox)
	c1Purchase, _ := c1.buyWithCard(250, c1.attachCard("sbx_a_visa_4242_12_2035")) // 999
	_, c2Tx := c2.buyWithCard(600, c2.attachCard("sbx_a_mastercard_4444_12_2035")) // 1999
	// t3 redeems the welcome offer, then buys again a minute later (a repeat buyer within 30 days).
	var welcomeID uuid.UUID
	var welcomePrice int64
	if err := e.app.DB.Raw("SELECT id, price_cents FROM credit_packs WHERE kind = 'intro' AND active").Row().Scan(&welcomeID, &welcomePrice); err != nil {
		t.Fatal(err)
	}
	// A card no other test used: the welcome offer is one per payment method, and the database is shared.
	t3Card := t3.attachCard(fmt.Sprintf("sbx_a_visa_%04d_11_2037", 1000+rand.IntN(7000)))
	if r := t3.buy(welcomeID, map[string]any{"payment_method_id": t3Card.ID}); r.status != http.StatusCreated {
		t.Fatalf("welcome pack: %d %s", r.status, r.body)
	}
	e.clock.Advance(time.Minute)
	_, t3Tx := t3.buyWithCard(250, t3Card) // 999

	// c1 chats four minutes at 20 credits (80 credits) and orders a 100-credit reading that is delivered:
	// 80 × 999000/250 + 100 × (999000 − 319680)/170 = 319 680 + 399 600 millicents recognized.
	startedSessionOn(t, e, psychic, c1)
	e.tick(time.Minute + time.Second)
	e.tick(time.Minute)
	e.tick(time.Minute)
	var order service.CustomRequestView
	c1.must(http.StatusCreated, http.MethodPost, "/api/v1/custom-requests", map[string]any{"service_id": svc.ID, "idempotency_key": uniq("order-key-")}, &order)
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/custom-requests/"+order.ID.String()+"/accept", nil, nil)
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/custom-requests/"+order.ID.String()+"/deliver", map[string]any{"delivery": "Your answer."}, nil)
	if lot := e.purchaseLot(c1Purchase); lot.CreditsRemaining != 70 || lot.ValueRemainingMillicents != 279_720 {
		t.Fatalf("c1's lot: %+v", lot)
	}
	var advisorNet int64
	e.app.DB.Raw("SELECT COALESCE(sum(net_credits), 0) FROM psychic_earnings WHERE created_at >= '2031-06-01'").Scan(&advisorNet)
	if net, _ := domain.SplitEarnings(100, cfg.Billing.PsychicSharePercent); advisorNet != net {
		t.Fatalf("advisor earnings %d, want only the delivered reading's %d", advisorNet, net)
	}

	gross := int64(3999+999+1999+999) + welcomePrice
	procEstimate := processing(3999) + processing(999) + processing(1999) + processing(welcomePrice) + processing(999)
	advisorCost := advisorNet * cfg.Billing.PayoutCentsPerCredit
	// What the lots of everything else in the database hold (not part of this test's window).
	var baseMilli int64
	e.app.DB.Raw("SELECT COALESCE(sum(value_remaining_millicents), 0) FROM credit_lots WHERE source = 'purchase' AND created_at < '2031-06-01'").Scan(&baseMilli)
	mineMilli := int64(3_999_000) + 279_720 + 1_999_000 + welcomePrice*1000 + 999_000

	const path = "/api/v1/admin/reports/revenue?from=2031-06-01&to=2031-06-30"
	var a service.RevenueReport
	admin.must(http.StatusOK, http.MethodGet, path, nil, &a)
	check := func(name string, got, want int64) {
		t.Helper()
		if got != want {
			t.Errorf("%s: got %d, want %d", name, got, want)
		}
	}
	tot := a.Totals
	check("gross", tot.GrossCents, gross)
	check("purchases", tot.Purchases, 5)
	check("paying clients", tot.PayingClients, 4)
	check("average order", tot.AvgOrderCents, gross/5)
	check("refunds", tot.RefundsCents, 0)
	check("net", tot.NetCents, gross)
	check("recognized", tot.RecognizedCents, 719)
	check("deferred", tot.DeferredCents, (baseMilli+mineMilli)/1000)
	check("advisor cost", tot.AdvisorCostCents, advisorCost)
	check("processing estimate", tot.ProcessingEstimateCents, procEstimate)
	check("contribution", tot.ContributionEstimateCents, gross-advisorCost-procEstimate)
	check("refund rate", tot.RefundRateBps, 0)
	check("disputes", tot.Disputes, 0)
	if a.Interval != "day" || a.Currency != "USD" || len(a.Series) != 30 || a.Assumptions.ProcessingFeeBps != 800 || a.Assumptions.ProcessingFixedCents != 30 {
		t.Fatalf("shape: %s %s %d buckets %+v", a.Interval, a.Currency, len(a.Series), a.Assumptions)
	}
	day := a.Series[9] // 2031-06-10
	if day.Bucket != "2031-06-10T00:00:00Z" || day.GrossCents != gross || day.RefundsCents != 0 || day.NetCents != gross || day.RecognizedCents != 719 {
		t.Fatalf("series day: %+v", day)
	}
	var seriesGross int64
	for i, p := range a.Series {
		seriesGross += p.GrossCents
		if i != 9 && (p.GrossCents != 0 || p.RecognizedCents != 0) {
			t.Fatalf("bucket %s must be empty: %+v", p.Bucket, p)
		}
	}
	check("series gross", seriesGross, gross)
	// Pack mix, by gross (descending).
	wantPacks := map[string][3]int64{ // purchases, gross, share_bps
		"test_1500": {1, 3999, 3999 * 10000 / gross}, "test_250": {2, 1998, 1998 * 10000 / gross}, "test_600": {1, 1999, 1999 * 10000 / gross},
		"welcome_600": {1, welcomePrice, welcomePrice * 10000 / gross},
	}
	if len(a.Packs) != len(wantPacks) {
		t.Fatalf("packs: %+v", a.Packs)
	}
	for i, p := range a.Packs {
		w, ok := wantPacks[p.Code]
		if !ok || p.Purchases != w[0] || p.GrossCents != w[1] || p.ShareBps != w[2] || p.RefundsCents != 0 || p.Name == "" || p.PackID == uuid.Nil {
			t.Errorf("pack %+v, want %v", p, w)
		}
		if i > 0 && a.Packs[i-1].GrossCents < p.GrossCents {
			t.Errorf("packs are not ordered by gross: %+v", a.Packs)
		}
	}
	check("welcome redemptions", a.Welcome.Redemptions, 1)
	check("welcome unverified cards", a.Welcome.UnverifiedCard, 0)
	check("welcome repeat buyers", a.Welcome.RepeatBuyers30d, 1)
	check("welcome repeat rate", a.Welcome.RepeatRateBps, 10000)
	check("trial signups", a.FreeTrial.SignupsWithTrial, 5)
	check("trial purchased", a.FreeTrial.Purchased, 4)
	check("trial conversion", a.FreeTrial.ConversionBps, 8000)
	if rq := a.RefundRequests; rq.Open+rq.Approved+rq.Denied+rq.Credited+rq.Withdrawn != 0 || rq.MedianHoursResolve != nil {
		t.Fatalf("no refund requests yet: %+v", rq)
	}

	// Validation: at most 366 days, day | week | month, a sane range.
	for _, bad := range []string{"?from=2030-01-01&to=2031-06-30", "?from=2031-06-01&to=2031-06-30&interval=hour", "?from=2031-06-30&to=2031-06-01", "?from=nope"} {
		if r := admin.call(http.MethodGet, "/api/v1/admin/reports/revenue"+bad, nil); r.status != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %s", bad, r.status, r.body)
		}
	}
	var month, week service.RevenueReport
	admin.must(http.StatusOK, http.MethodGet, path+"&interval=month", nil, &month)
	admin.must(http.StatusOK, http.MethodGet, path+"&interval=week", nil, &week)
	if len(month.Series) != 1 || month.Series[0].Bucket != "2031-06-01T00:00:00Z" || month.Series[0].GrossCents != gross || month.Series[0].RecognizedCents != 719 {
		t.Fatalf("month series: %+v", month.Series)
	}
	// 2031-06-01 is a Sunday: its week starts Monday 2031-05-26; the purchases of Tuesday 2031-06-10 fall in the week of Monday 06-09.
	if week.Series[0].Bucket != "2031-05-26T00:00:00Z" || len(week.Series) != 6 || week.Series[2].Bucket != "2031-06-09T00:00:00Z" || week.Series[2].GrossCents != gross {
		t.Fatalf("week series: %+v", week.Series)
	}

	// ---- Refund requests and their resolution, hours apart (service level: the admin session would idle out).
	refunds := e.app.Services.Refunds
	create := func(c *client, tx uuid.UUID, reason string) service.RefundRequestView {
		v, _, err := refunds.Create(ctx, c.user.ID, service.CreateRefundInput{TransactionID: tx, Reason: reason, IdempotencyKey: uniq("refund-key-")})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		return *v
	}
	var c1Tx uuid.UUID
	c1Tx = e.uuidOf("SELECT id FROM payment_transactions WHERE credit_purchase_id = ?", c1Purchase)
	reqC1 := create(c1, c1Tx, "unused_credits")
	reqC2 := create(c2, c2Tx, "unused_credits")
	reqT3 := create(t3, t3Tx, "other")
	t1Tx := e.uuidOf("SELECT id FROM payment_transactions WHERE user_id = ? AND kind = 'credit_purchase'", t1.user.ID)
	reqT1 := create(t1, t1Tx, "other")
	if _, err := refunds.Withdraw(ctx, t1.user.ID, reqT1.ID); err != nil {
		t.Fatal(err)
	}
	// c1: refundable = floor(70 × 999 ÷ 250) = 279 cents, approved after 2 h.
	e.clock.Advance(2 * time.Hour)
	if v, err := refunds.Approve(ctx, admin.user.ID, reqC1.ID, service.ApproveRefundInput{AmountCents: 279, Note: "Unused credits, approved."}); err != nil || v.Status != "approved" {
		t.Fatalf("approve: %v %+v", err, v)
	}
	e.clock.Advance(time.Hour) // t3: courtesy credits after 3 h
	if _, err := refunds.Credit(ctx, admin.user.ID, reqT3.ID, service.CreditRefundInput{Credits: 10, Note: "Courtesy credits instead."}); err != nil {
		t.Fatal(err)
	}
	e.clock.Advance(time.Hour) // c2: denied after 4 h
	if _, err := refunds.Deny(ctx, admin.user.ID, reqC2.ID, service.DenyRefundInput{Note: "Not eligible, sorry."}); err != nil {
		t.Fatal(err)
	}
	// One more left open.
	open := create(t3, e.uuidOf("SELECT id FROM payment_transactions WHERE user_id = ? AND kind = 'credit_purchase' ORDER BY created_at LIMIT 1", t3.user.ID), "other")
	_ = open

	rng, interval, err := service.ParseRevenueRange("2031-06-01", "2031-06-30", "day", e.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.app.Services.Revenue.Report(ctx, rng, interval)
	if err != nil {
		t.Fatal(err)
	}
	tb := b.Totals
	check("B gross", tb.GrossCents, gross)
	check("B refunds", tb.RefundsCents, 279)
	check("B net", tb.NetCents, gross-279)
	check("B refund rate", tb.RefundRateBps, 279*10000/gross)
	check("B recognized", tb.RecognizedCents, 719) // a refund claws back a lot; it does not change what chats consumed
	deferredB := (baseMilli + mineMilli - 279_720) / 1000
	check("B deferred", tb.DeferredCents, deferredB)
	check("B contribution", tb.ContributionEstimateCents, gross-279-advisorCost-procEstimate)
	if rq := b.RefundRequests; rq.Open != 1 || rq.Approved != 1 || rq.Denied != 1 || rq.Credited != 1 || rq.Withdrawn != 1 || rq.MedianHoursResolve == nil || *rq.MedianHoursResolve != 3 {
		t.Fatalf("refund requests: %+v median %v", rq, rq.MedianHoursResolve)
	}
	refundedDay := b.Series[9]
	if refundedDay.RefundsCents != 279 || refundedDay.NetCents != gross-279 {
		t.Fatalf("series after the refund: %+v", refundedDay)
	}
	for _, p := range b.Packs {
		want := int64(0)
		if p.Code == "test_250" {
			want = 279
		}
		if p.RefundsCents != want {
			t.Errorf("pack %s refunds %d, want %d", p.Code, p.RefundsCents, want)
		}
	}
	// The overview carries the refund and the net revenue.
	ov, err := e.app.Services.Analytics.Overview(ctx, service.Range{From: time.Date(2031, 6, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2031, 7, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	check("overview revenue", ov.RevenueCents["USD"], gross)
	check("overview refunds", ov.RefundsCents["USD"], 279)
	check("overview net revenue", ov.NetRevenueCents["USD"], gross-279)
	check("overview purchases", ov.Purchases, 5)

	// Deferred revenue at a past instant: value now + what was consumed or clawed back since. Three days later 100 credits of
	// c2's pack are spent; the report as of the end of 2031-06-10 must not change.
	e.clock.Advance(72 * time.Hour)
	if _, err := e.app.Services.Admin.AdjustCredits(ctx, admin.user.ID, c2.user.ID, -100, "spend", uniq("adj-key-")); err != nil {
		t.Fatal(err)
	}
	past, _, err := service.ParseRevenueRange("2031-06-01", "2031-06-10", "day", e.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	c, err := e.app.Services.Revenue.Report(ctx, past, "day")
	if err != nil {
		t.Fatal(err)
	}
	check("C deferred at the end of 06-10", c.Totals.DeferredCents, deferredB)
	check("C refunds", c.Totals.RefundsCents, 279)
	live, err := e.app.Services.Revenue.Report(ctx, rng, "day")
	if err != nil {
		t.Fatal(err)
	}
	spent := int64(100) * 1_999_000 / 600 // value of 100 of c2's 600 credits
	check("live deferred moved by the spend", live.Totals.DeferredCents, (baseMilli+mineMilli-279_720-spent)/1000)
	var liveDirect int64
	e.app.DB.Raw("SELECT floor(COALESCE(sum(value_remaining_millicents), 0) / 1000)::bigint FROM credit_lots WHERE source = 'purchase' AND value_remaining_millicents > 0").Scan(&liveDirect)
	check("live deferred equals the lots", live.Totals.DeferredCents, liveDirect)
}

// waitFor polls until cond holds (analytics events are written by a background flusher).
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(25 * time.Millisecond)
	}
}
