package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/app"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
)

// ------------------------------------------------------------- helpers

// newEnvWithProcessor builds an app instance whose sandbox payment
// providers use proc, so several instances (and the test) share one
// simulated processor — like API replicas talking to one gateway.
func newEnvWithProcessor(t *testing.T, proc *payment.SandboxProcessor, opts ...option) *env {
	t.Helper()
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	cfg.App.Env = config.EnvTest
	cfg.Database.DSN = testDSN
	cfg.Redis.URL = testRedis
	cfg.Redis.KeyPrefix = "e2e:" + uuid.NewString()[:8] + ":"
	cfg.Auth.Argon2Memory, cfg.Auth.Argon2Iterations, cfg.Auth.Argon2Parallelism = 1024, 1, 1
	cfg.Worker.Enabled = false
	cfg.RateLimit.Scale = 1000
	cfg.Security.AllowedOrigins = []string{origin}
	cfg.Security.AdminOrigins = []string{adminOrigin}
	cfg.Log.Level = "error"
	cfg.Analytics.FlushInterval = 50 * time.Millisecond
	// revisions 5.1: production ships with memberships off (§25.1); the suite exercises them
	// unless a test turns the switch off with withMemberships(false).
	cfg.Billing.MembershipsEnabled = true
	for _, o := range opts {
		o(cfg)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("config invalid: %v", err)
	}
	fake := clock.NewFake(time.Now())
	rec := &mail.Recorder{}
	store, advisors, chatImages := storage.NewMemory("https://assets.test"), storage.NewMemory("https://advisors.test"), storage.NewMemory("")
	ctx := context.Background()
	a, err := app.New(ctx, cfg, zap.NewNop(), app.Overrides{Clock: fake, Mailer: rec, Storage: store, AdvisorStorage: advisors,
		ChatStorage: chatImages, PaymentProcessor: proc})
	if err != nil {
		t.Fatalf("app: %v", err)
	}
	a.Start(ctx)
	srv := httptest.NewServer(a.Router)
	e := &env{t: t, app: a, srv: srv, clock: fake, mail: rec, store: store, advisors: advisors, chat: chatImages}
	t.Cleanup(func() {
		srv.CloseClientConnections()
		srv.Close()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		a.Shutdown(sctx)
	})
	return e
}

// billingPlan creates a dedicated plan (service level, no step-up) so tests
// never change the shared seeded Premium Membership.
func (e *env) billingPlan(price int64, edit ...func(*service.PlanVersionInput)) (service.AdminPlanView, uuid.UUID) {
	e.t.Helper()
	in := service.PlanVersionInput{PriceCents: price, Interval: "month"}
	for _, f := range edit {
		f(&in)
	}
	p, err := e.app.Services.Billing.AdminCreatePlan(context.Background(), service.PlanCreateInput{Code: uniq("plan"),
		Name: map[string]string{"en": "Test Membership", "fr": "Abonnement test"}, Version: in})
	if err != nil {
		e.t.Fatalf("create plan: %v", err)
	}
	for _, v := range p.Versions {
		if v.IsCurrent {
			return *p, v.ID
		}
	}
	e.t.Fatal("plan has no current version")
	return *p, uuid.Nil
}

func providerOf(token string) string {
	if strings.HasPrefix(token, "sbx_b_") {
		return "sandbox_b"
	}
	return "sandbox_a"
}

// attachCard saves a sandbox card and returns it.
func (c *client) attachCard(token string) service.PaymentMethodView {
	c.e.t.Helper()
	var m service.PaymentMethodView
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/billing/payment-methods", map[string]any{
		"provider": providerOf(token), "token": token, "idempotency_key": uniq("card-key-")}, &m)
	return m
}

func (c *client) subscribe(versionID, methodID uuid.UUID) service.SubscriptionResult {
	c.e.t.Helper()
	var res service.SubscriptionResult
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/billing/subscriptions", map[string]any{
		"plan_version_id": versionID, "payment_method_id": methodID, "idempotency_key": uniq("sub-key-")}, &res)
	return res
}

func (c *client) subscription() *service.SubscriptionView {
	c.e.t.Helper()
	var out struct {
		Subscription *service.SubscriptionView `json:"subscription"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/subscription", nil, &out)
	return out.Subscription
}

func (c *client) cards() []service.PaymentMethodView {
	c.e.t.Helper()
	var out struct {
		Items []service.PaymentMethodView `json:"items"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/payment-methods", nil, &out)
	return out.Items
}

func (c *client) transactions(query string) service.Page[service.TransactionView] {
	c.e.t.Helper()
	var page service.Page[service.TransactionView]
	c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/transactions"+query, nil, &page)
	return page
}

// renewAt moves the clock to t and runs the scheduler (billing included).
// Weeks pass, so the given clients sign in again (sessions expired).
func (e *env) renewAt(t time.Time, cs ...*client) {
	e.t.Helper()
	e.clock.Set(t)
	e.tick(time.Second)
	for _, c := range cs {
		c.relogin()
	}
}

// relogin signs the client in again with fresh cookies.
func (c *client) relogin() {
	c.e.t.Helper()
	jar, _ := cookiejar.New(nil)
	c.http.Jar, c.csrf = jar, ""
	var out struct {
		User *service.UserView `json:"user"`
	}
	c.must(http.StatusOK, http.MethodPost, "/api/v1/auth/login", map[string]any{"email": c.user.Email, "password": c.pw}, &out)
	c.user = out.User
}

func countNotifications(t *testing.T, c *client, kind string) int {
	t.Helper()
	var page service.Page[service.NotificationView]
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/notifications?limit=100", nil, &page)
	n := 0
	for _, item := range page.Items {
		if item.Kind == kind {
			n++
		}
	}
	return n
}

func signedWebhook(e *env, provider string, evt payment.WebhookEvent, sig string) response {
	body, _ := json.Marshal(evt)
	if sig == "" {
		sig = payment.Sign([]byte(e.app.Config.Payments.WebhookSecret), body, e.clock.Now())
	}
	return e.client().raw(http.MethodPost, "/api/v1/webhooks/payments/"+provider, strings.NewReader(string(body)),
		map[string]string{"Content-Type": "application/json", "X-Staryield-Signature": sig})
}

// --------------------------------------------------------------- tests

func TestMembershipLifecycleGrantsAndDunning(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	plan, version := e.billingPlan(999, func(v *service.PlanVersionInput) {
		v.MonthlyCreditGrant, v.MonthlyFreeMinutes, v.ChatDiscountBps, v.WaitlistPriority = 100, 10, 1000, true
		v.Benefits = map[string][]string{"en": {"100 credits every month"}, "fr": {"100 crédits chaque mois"}}
	})
	c := e.registerClient()
	start := c.wallet()

	// The public catalogue lists the plan, localised.
	var plans struct {
		Items []service.PlanView `json:"items"`
	}
	c.locale = "fr"
	c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/plans", nil, &plans)
	c.locale = ""
	found := false
	for _, p := range plans.Items {
		if p.ID == plan.ID {
			found = p.Name == "Abonnement test" && p.Benefits[0] == "100 crédits chaque mois" && p.ChatDiscountPct == 10 && p.VersionID == version
		}
	}
	if !found {
		t.Fatalf("plan missing or not localised: %+v", plans.Items)
	}
	var cfg payment.ClientConfig
	c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/client-config?purpose=subscription&plan_version_id="+version.String(), nil, &cfg)
	if cfg.Provider != "sandbox_a" || cfg.Type != "sandbox" || cfg.TokenFormat == "" {
		t.Fatalf("client config: %+v", cfg)
	}

	card := c.attachCard("sbx_a_visa_4242_12_2035")
	if !card.IsDefault || card.Brand != "visa" || card.Last4 != "4242" || card.Status != "active" {
		t.Fatalf("card: %+v", card)
	}
	key := uniq("sub-key-")
	var res, replay service.SubscriptionResult
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/billing/subscriptions", map[string]any{"plan_version_id": version,
		"payment_method_id": card.ID, "idempotency_key": key}, &res)
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/billing/subscriptions", map[string]any{"plan_version_id": version,
		"payment_method_id": card.ID, "idempotency_key": key}, &replay)
	sub := res.Subscription
	if sub.Status != "active" || res.Transaction == nil || res.Transaction.Status != "succeeded" || res.Transaction.Kind != "subscription_initial" ||
		sub.RenewsAt == nil || sub.PaymentMethod == nil || sub.PaymentMethod.ID != card.ID || replay.Subscription.ID != sub.ID {
		t.Fatalf("subscribe: %+v / %+v", res, replay)
	}
	if r := c.call(http.MethodPost, "/api/v1/billing/subscriptions", map[string]any{"plan_version_id": version, "payment_method_id": card.ID,
		"idempotency_key": uniq("sub-key-")}); r.status != http.StatusConflict || r.errCode(t) != "subscription_exists" {
		t.Fatalf("second membership: %d %s", r.status, r.body)
	}
	w := c.wallet()
	if w.Balance != start.Balance+100 || w.FreeMinutes != start.FreeMinutes+10 {
		t.Fatalf("first grant: %+v → %+v", start, w)
	}
	bs := e.app.Services.Billing
	if !bs.IsMember(ctx, c.user.ID) || bs.DiscountFor(ctx, c.user.ID) != 1000 || !bs.HasWaitlistPriority(ctx, c.user.ID) {
		t.Fatal("member hooks")
	}
	if got := domain.ApplyDiscount(35, bs.DiscountFor(ctx, c.user.ID)); got != 31 {
		t.Fatalf("discounted rate: %d", got)
	}

	// Renewal 1 → period 2: credits granted again; unused free minutes of
	// the previous grant expire, so the balance does not stack.
	end1 := *sub.CurrentPeriodEnd
	e.tick(time.Minute) // not due yet
	if s := c.subscription(); s.CurrentPeriodEnd == nil || !s.CurrentPeriodEnd.Equal(end1) {
		t.Fatal("renewed before the period ended")
	}
	e.renewAt(end1, c)
	s2 := c.subscription()
	if s2.Status != "active" || !s2.CurrentPeriodStart.Equal(end1) || !s2.CurrentPeriodEnd.After(end1) {
		t.Fatalf("renewal: %+v", s2)
	}
	if w := c.wallet(); w.Balance != start.Balance+200 || w.FreeMinutes != start.FreeMinutes+10 {
		t.Fatalf("second grant: %+v", w)
	}
	waitNotification(t, c, service.NotifySubscriptionRenewed)
	// The member uses 3 granted minutes; at the next renewal the 7 unused
	// expire and 10 are granted (promotional minutes are never taken).
	e.app.DB.Exec("UPDATE wallets SET free_minutes = free_minutes - 3 WHERE user_id = ?", c.user.ID)
	end2 := *s2.CurrentPeriodEnd
	e.renewAt(end2, c)
	if w := c.wallet(); w.FreeMinutes != start.FreeMinutes+10 || w.Balance != start.Balance+300 {
		t.Fatalf("third grant: %+v", w)
	}

	// Soft decline: the member switches to a card that soft-declines.
	soft := c.attachCard("sbx_a_visa_9995_12_2035")
	var upd service.SubscriptionResult
	c.must(http.StatusOK, http.MethodPatch, "/api/v1/billing/subscription", map[string]any{"payment_method_id": soft.ID}, &upd)
	if upd.Transaction != nil || upd.Subscription.PaymentMethod.ID != soft.ID {
		t.Fatalf("switching cards on an active membership must not charge: %+v", upd)
	}
	s3 := c.subscription()
	end3 := *s3.CurrentPeriodEnd
	e.renewAt(end3, c)
	pd := c.subscription()
	if pd.Status != "past_due" || pd.NextRetryAt == nil || !pd.NextRetryAt.Equal(end3.Add(24*time.Hour)) || pd.GraceUntil == nil ||
		!pd.GraceUntil.Equal(end3.Add(7*24*time.Hour)) {
		t.Fatalf("soft decline → past_due: %+v", pd)
	}
	if bs.IsMember(ctx, c.user.ID) || bs.DiscountFor(ctx, c.user.ID) != 0 {
		t.Fatal("past-due members get no benefits")
	}
	waitNotification(t, c, service.NotifySubscriptionPaymentFailed)
	// Retry at +24h declines again; the member then replaces the card and
	// is charged immediately (the anchor does not move).
	e.renewAt(end3.Add(24*time.Hour), c)
	if s := c.subscription(); s.Status != "past_due" || !s.NextRetryAt.Equal(end3.Add(72*time.Hour)) {
		t.Fatalf("second attempt: %+v", s)
	}
	var fixed service.SubscriptionResult
	c.must(http.StatusOK, http.MethodPatch, "/api/v1/billing/subscription", map[string]any{"payment_method_id": card.ID}, &fixed)
	if fixed.Subscription.Status != "active" || fixed.Transaction == nil || fixed.Transaction.Status != "succeeded" ||
		!fixed.Subscription.CurrentPeriodStart.Equal(end3) {
		t.Fatalf("pay now with another card: %+v", fixed)
	}
	if w := c.wallet(); w.Balance != start.Balance+400 {
		t.Fatalf("grant after recovery: %+v", w)
	}

	// Payment history: newest first, paginated without duplicates.
	seen := map[uuid.UUID]bool{}
	kinds := map[string]int{}
	cursor := ""
	for pages := 0; ; pages++ {
		q := "?limit=2"
		if cursor != "" {
			q += "&cursor=" + cursor
		}
		page := c.transactions(q)
		for _, tx := range page.Items {
			if seen[tx.ID] {
				t.Fatalf("duplicate transaction across pages: %s", tx.ID)
			}
			seen[tx.ID] = true
			kinds[tx.Kind+"/"+tx.Status]++
			if tx.Currency != "USD" || tx.AmountCents != 999 || tx.PaymentMethod == nil {
				t.Fatalf("transaction view: %+v", tx)
			}
		}
		if page.NextCursor == "" || pages > 10 {
			break
		}
		cursor = page.NextCursor
	}
	if kinds["subscription_initial/succeeded"] != 1 || kinds["subscription_renewal/succeeded"] != 3 || kinds["subscription_renewal/failed"] != 2 {
		t.Fatalf("history: %v", kinds)
	}
	if r := c.call(http.MethodGet, "/api/v1/billing/transactions?cursor=bogus", nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("bad cursor: %d", r.status)
	}
	// Psychics and anonymous users have no billing.
	if r := e.createPsychic(10).call(http.MethodGet, "/api/v1/billing/subscription", nil); r.status != http.StatusForbidden {
		t.Fatalf("psychic billing: %d", r.status)
	}
	if r := e.client().call(http.MethodGet, "/api/v1/billing/payment-methods", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("anonymous billing: %d", r.status)
	}
}

func TestMembershipHardDeclineExpiresAfterGrace(t *testing.T) {
	e := newEnv(t)
	_, version := e.billingPlan(999)
	c := e.registerClient()
	good := c.attachCard("sbx_a_visa_4242_12_2035")
	res := c.subscribe(version, good.ID)
	hard := c.attachCard("sbx_a_mastercard_0002_12_2035")
	c.must(http.StatusOK, http.MethodPatch, "/api/v1/billing/subscription", map[string]any{"payment_method_id": hard.ID}, nil)
	end := *res.Subscription.CurrentPeriodEnd
	e.renewAt(end, c)
	s := c.subscription()
	if s.Status != "past_due" || s.NextRetryAt != nil {
		t.Fatalf("hard decline must not retry the same card: %+v", s)
	}
	for _, m := range c.cards() {
		if m.ID == hard.ID && m.Status != "failed" {
			t.Fatalf("hard-declined card status: %+v", m)
		}
	}
	key := fmt.Sprintf("sub:%s:p2:a2", res.Subscription.ID)
	e.renewAt(end.Add(3*24*time.Hour), c)
	var n int64
	e.app.DB.Raw("SELECT count(*) FROM payment_transactions WHERE idempotency_key = ?", key).Scan(&n)
	if n != 0 {
		t.Fatal("no retry may be attempted on a hard-declined card")
	}
	if r := c.call(http.MethodPatch, "/api/v1/billing/subscription", map[string]any{"payment_method_id": hard.ID}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("failed card can't be reused: %d %s", r.status, r.body)
	}
	e.renewAt(end.Add(7*24*time.Hour+time.Second), c)
	if s := c.subscription(); s.Status != "expired" || s.EndedAt == nil {
		t.Fatalf("grace expiry: %+v", s)
	}
	waitNotification(t, c, service.NotifySubscriptionCanceled)
	if e.app.Services.Billing.IsMember(context.Background(), c.user.ID) {
		t.Fatal("expired membership")
	}
	// A new membership can be started with a working card.
	if r := c.subscribe(version, good.ID); r.Subscription.Status != "active" {
		t.Fatalf("new membership: %+v", r)
	}
}

func TestMembershipCancelResumeAndPeriodEnd(t *testing.T) {
	e := newEnv(t)
	_, version := e.billingPlan(1999, func(v *service.PlanVersionInput) { v.MonthlyFreeMinutes = 5 })
	c := e.registerClient()
	card := c.attachCard("sbx_a_visa_4242_12_2035")
	res := c.subscribe(version, card.ID)
	before := c.wallet()
	var out struct {
		Subscription *service.SubscriptionView `json:"subscription"`
	}
	c.must(http.StatusOK, http.MethodPost, "/api/v1/billing/subscription/cancel", map[string]any{"reason": "Trying it out"}, &out)
	if !out.Subscription.CancelAtPeriodEnd || out.Subscription.RenewsAt != nil || out.Subscription.Status != "active" {
		t.Fatalf("cancel at period end: %+v", out.Subscription)
	}
	if r := c.call(http.MethodPost, "/api/v1/billing/subscription/cancel", map[string]any{"at_period_end": false}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("immediate self-cancel of a paid period: %d", r.status)
	}
	c.must(http.StatusOK, http.MethodPost, "/api/v1/billing/subscription/resume", nil, &out)
	if out.Subscription.CancelAtPeriodEnd || out.Subscription.RenewsAt == nil {
		t.Fatalf("resume: %+v", out.Subscription)
	}
	c.must(http.StatusOK, http.MethodPost, "/api/v1/billing/subscription/cancel", nil, &out)
	end := *res.Subscription.CurrentPeriodEnd
	e.renewAt(end, c)
	s := c.subscription()
	if s.Status != "canceled" || s.EndedAt == nil || !s.EndedAt.Equal(end) {
		t.Fatalf("canceled at period end: %+v", s)
	}
	if w := c.wallet(); w.FreeMinutes != before.FreeMinutes-5 {
		t.Fatalf("granted minutes must expire with the membership: %d → %d", before.FreeMinutes, w.FreeMinutes)
	}
	var charges int64
	e.app.DB.Raw("SELECT count(*) FROM payment_transactions WHERE subscription_id = ?", res.Subscription.ID).Scan(&charges)
	if charges != 1 {
		t.Fatalf("a canceled membership must not renew: %d charges", charges)
	}
	if r := c.call(http.MethodPost, "/api/v1/billing/subscription/resume", nil); r.status != http.StatusNotFound {
		t.Fatalf("resume after the end: %d", r.status)
	}
	if r := c.subscribe(version, card.ID); r.Subscription.Status != "active" || r.Subscription.ID == res.Subscription.ID {
		t.Fatalf("subscribe again: %+v", r)
	}
}

func TestPaymentMethodsSecurityLimitsAndIDOR(t *testing.T) {
	e := newEnv(t)
	_, version := e.billingPlan(999)
	c := e.registerClient()
	other := e.registerClient()
	post := func(cl *client, body map[string]any) response {
		return cl.call(http.MethodPost, "/api/v1/billing/payment-methods", body)
	}
	// Card data never reaches the API.
	for _, body := range []map[string]any{
		{"provider": "sandbox_a", "token": "4242424242424242", "idempotency_key": uniq("card-key-")},
		{"provider": "sandbox_a", "token": "sbx_a_visa_4242_12_2035", "number": "4242424242424242", "idempotency_key": uniq("card-key-")},
		{"provider": "sandbox_a", "token": "sbx_a_visa_4242_12_2035", "cvc": "123", "idempotency_key": uniq("card-key-")},
	} {
		if r := post(c, body); r.status != http.StatusUnprocessableEntity {
			t.Fatalf("card data accepted: %v → %d %s", body, r.status, r.body)
		}
	}
	for _, body := range []map[string]any{
		{"provider": "sandbox_a", "token": "sbx_a_visa_4242_12_2035"},                                       // no idempotency key
		{"provider": "stripe", "token": "sbx_a_visa_4242_12_2035", "idempotency_key": uniq("card-key-")},    // unknown provider
		{"provider": "sandbox_a", "token": "sbx_b_visa_4242_12_2035", "idempotency_key": uniq("card-key-")}, // foreign token
		{"provider": "sandbox_a", "token": "sbx_a_visa_4242_01_2020", "idempotency_key": uniq("card-key-")}, // expired
		{"provider": "sandbox_a", "token": "sbx_a_visa_4242_12_2035", "billing_country": "France", "idempotency_key": uniq("card-key-")},
	} {
		if r := post(c, body); r.status != http.StatusUnprocessableEntity {
			t.Fatalf("invalid attach accepted: %v → %d %s", body, r.status, r.body)
		}
	}
	key := uniq("card-key-")
	var first, replay, dup service.PaymentMethodView
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/billing/payment-methods", map[string]any{"provider": "sandbox_a",
		"token": "sbx_a_visa_4242_12_2035", "idempotency_key": key}, &first)
	c.must(http.StatusOK, http.MethodPost, "/api/v1/billing/payment-methods", map[string]any{"provider": "sandbox_a",
		"token": "sbx_a_visa_4242_12_2035", "idempotency_key": key}, &replay)
	c.must(http.StatusOK, http.MethodPost, "/api/v1/billing/payment-methods", map[string]any{"provider": "sandbox_a",
		"token": "sbx_a_visa_4242_12_2035", "idempotency_key": uniq("card-key-")}, &dup)
	if replay.ID != first.ID || dup.ID != first.ID || len(c.cards()) != 1 {
		t.Fatalf("replay/duplicate card: %s %s %s", first.ID, replay.ID, dup.ID)
	}
	second := c.attachCard("sbx_a_mastercard_4444_11_2034")
	if second.IsDefault {
		t.Fatal("only the first card becomes default automatically")
	}
	// Another user's card is invisible.
	for _, r := range []response{
		other.call(http.MethodDelete, "/api/v1/billing/payment-methods/"+first.ID.String(), nil),
		other.call(http.MethodPost, "/api/v1/billing/payment-methods/"+first.ID.String()+"/default", nil),
		other.call(http.MethodPost, "/api/v1/billing/subscriptions", map[string]any{"plan_version_id": version, "payment_method_id": first.ID,
			"idempotency_key": uniq("sub-key-")}),
	} {
		if r.status != http.StatusNotFound {
			t.Fatalf("IDOR: %d %s", r.status, r.body)
		}
	}
	if len(other.cards()) != 0 {
		t.Fatal("cards leaked to another user")
	}
	res := c.subscribe(version, first.ID)
	// Default switch.
	var def service.PaymentMethodView
	c.must(http.StatusOK, http.MethodPost, "/api/v1/billing/payment-methods/"+second.ID.String()+"/default", nil, &def)
	defaults := 0
	for _, m := range c.cards() {
		if m.IsDefault {
			defaults++
			if m.ID != second.ID {
				t.Fatal("wrong default")
			}
		}
	}
	if defaults != 1 {
		t.Fatalf("exactly one default: %d", defaults)
	}
	// Removing the membership's card moves the membership to another card.
	c.must(http.StatusNoContent, http.MethodDelete, "/api/v1/billing/payment-methods/"+first.ID.String(), nil, nil)
	if s := c.subscription(); s.PaymentMethod == nil || s.PaymentMethod.ID != second.ID {
		t.Fatalf("membership card replacement: %+v", s.PaymentMethod)
	}
	// The last card of an auto-renewing membership cannot be removed …
	if r := c.call(http.MethodDelete, "/api/v1/billing/payment-methods/"+second.ID.String(), nil); r.status != http.StatusConflict ||
		r.errCode(t) != "payment_method_required" {
		t.Fatalf("last membership card: %d %s", r.status, r.body)
	}
	// … unless the membership is ending.
	c.must(http.StatusOK, http.MethodPost, "/api/v1/billing/subscription/cancel", nil, nil)
	c.must(http.StatusNoContent, http.MethodDelete, "/api/v1/billing/payment-methods/"+second.ID.String(), nil, nil)
	if r := c.call(http.MethodDelete, "/api/v1/billing/payment-methods/"+second.ID.String(), nil); r.status != http.StatusNotFound {
		t.Fatalf("double delete: %d", r.status)
	}
	_ = res
	// At most five saved cards.
	for i, tok := range []string{"sbx_a_visa_1111_12_2035", "sbx_a_visa_2222_12_2035", "sbx_a_visa_3333_12_2035", "sbx_a_visa_5555_12_2035",
		"sbx_a_visa_6666_12_2035"} {
		if i < 5 {
			c.attachCard(tok)
		}
	}
	if r := post(c, map[string]any{"provider": "sandbox_a", "token": "sbx_a_visa_7777_12_2035", "idempotency_key": uniq("card-key-")}); r.status != http.StatusConflict ||
		r.errCode(t) != "limit_reached" {
		t.Fatalf("sixth card: %d %s", r.status, r.body)
	}
	// Card references are sealed: no provider reference in clear text.
	var refs []string
	e.app.DB.Raw("SELECT provider_method_ref_enc FROM payment_methods WHERE user_id = ?", c.user.ID).Scan(&refs)
	for _, ref := range refs {
		if !strings.HasPrefix(ref, "v1:") || strings.Contains(ref, "pm_sbx") {
			t.Fatalf("provider reference stored in clear: %q", ref)
		}
	}
	var customers []string
	e.app.DB.Raw("SELECT provider_customer_ref_enc FROM payment_customers WHERE user_id = ?", c.user.ID).Scan(&customers)
	if len(customers) != 1 || strings.Contains(customers[0], "cus_sbx") {
		t.Fatalf("customer reference stored in clear: %v", customers)
	}
}

func TestCreditPurchasesWithSavedCardsAndTokens(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	c := e.registerClient()
	var packs struct {
		Items []service.PackView `json:"items"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/credit-packs", nil, &packs)
	var pack250 service.PackView
	for _, p := range packs.Items {
		if p.Credits == 250 {
			pack250 = p
		}
	}
	card := c.attachCard("sbx_a_visa_4242_12_2035")
	buy := func(extra map[string]any) response {
		body := map[string]any{"pack_id": pack250.ID, "idempotency_key": uniq("purchase-key-"), "consent": purchaseConsent()}
		for k, v := range extra {
			body[k] = v
		}
		return c.call(http.MethodPost, "/api/v1/credits/purchases", body)
	}
	var p service.PurchaseView
	r := buy(map[string]any{"payment_method_id": card.ID})
	if r.status != http.StatusCreated {
		t.Fatalf("saved-card purchase: %d %s", r.status, r.body)
	}
	r.decode(t, &p)
	if p.Status != "succeeded" || p.Provider != "sandbox_a" || p.Wallet.Balance != 250 {
		t.Fatalf("purchase: %+v", p)
	}
	declined := c.attachCard("sbx_a_visa_0002_12_2035")
	if r := buy(map[string]any{"payment_method_id": declined.ID}); r.status != http.StatusPaymentRequired || r.errCode(t) != "payment_failed" {
		t.Fatalf("declined saved card: %d %s", r.status, r.body)
	}
	cards := len(c.cards())
	if r := buy(map[string]any{"payment_token": "sbx_a_visa_4242_12_2035", "provider": "sandbox_a"}); r.status != http.StatusCreated {
		t.Fatalf("one-off token purchase: %d %s", r.status, r.body)
	}
	if len(c.cards()) != cards {
		t.Fatal("a one-off token must not be saved")
	}
	if r := buy(map[string]any{"payment_token": "sbx_a_mastercard_5454_10_2033", "provider": "sandbox_a", "save_method": true}); r.status != http.StatusCreated {
		t.Fatalf("token purchase with save: %d %s", r.status, r.body)
	}
	if len(c.cards()) != cards+1 {
		t.Fatal("save_method must save the card")
	}
	for _, extra := range []map[string]any{
		{"payment_method_id": card.ID, "payment_token": "sbx_a_visa_4242_12_2035", "provider": "sandbox_a"},
		{"payment_token": "4242424242424242", "provider": "sandbox_a"},
		{"payment_token": "sbx_a_visa_4242_12_2035", "provider": "nope"},
	} {
		if r := buy(extra); r.status != http.StatusUnprocessableEntity {
			t.Fatalf("invalid purchase %v: %d %s", extra, r.status, r.body)
		}
	}
	if r := buy(map[string]any{"payment_method_id": uuid.New()}); r.status != http.StatusNotFound {
		t.Fatalf("unknown card: %d", r.status)
	}
	// The legacy hosted checkout still works and every purchase is mirrored
	// into the payment history.
	c.buyCredits(600)
	page := c.transactions("?limit=50")
	kinds := map[string]int{}
	for _, tx := range page.Items {
		if tx.Kind != "credit_purchase" {
			t.Fatalf("unexpected kind: %+v", tx)
		}
		kinds[tx.Status]++
		if tx.Status == "succeeded" && tx.Credits == 0 {
			t.Fatalf("credits missing: %+v", tx)
		}
	}
	if kinds["succeeded"] != 4 || kinds["failed"] != 1 {
		t.Fatalf("purchase history: %v", kinds)
	}
	if w := c.wallet(); w.Balance != 250*3+600 {
		t.Fatalf("balance: %d", w.Balance)
	}
}

func TestAdminBillingConsole(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	plans := "/api/v1/admin/billing/plans"
	body := map[string]any{"code": uniq("vip"), "name": map[string]string{"en": "VIP", "es": "VIP"},
		"version": map[string]any{"price_cents": 999, "interval": "month", "monthly_credit_grant": 200}}
	e.clock.Advance(11 * time.Minute) // the sign-in step-up window has passed
	if r := admin.call(http.MethodPost, plans, body); r.status != http.StatusForbidden || r.errCode(t) != "step_up_required" {
		t.Fatalf("plan creation without step-up: %d %s", r.status, r.body)
	}
	admin.stepUp()
	for _, bad := range []map[string]any{
		{"code": "Bad Code!", "name": map[string]string{"en": "x"}, "version": map[string]any{"price_cents": 999, "interval": "month"}},
		{"code": uniq("ok"), "name": map[string]string{"fr": "x"}, "version": map[string]any{"price_cents": 999, "interval": "month"}},
		{"code": uniq("ok"), "name": map[string]string{"en": "x"}, "version": map[string]any{"price_cents": 1, "interval": "week"}},
	} {
		if r := admin.call(http.MethodPost, plans, bad); r.status != http.StatusUnprocessableEntity {
			t.Fatalf("invalid plan accepted: %v → %d", bad, r.status)
		}
	}
	var plan service.AdminPlanView
	admin.must(http.StatusCreated, http.MethodPost, plans, body, &plan)
	// revisions 5.1: the list also tells the console whether memberships are switched on (§25.1).
	var listed struct {
		Items       []service.AdminPlanView `json:"items"`
		Memberships *bool                   `json:"memberships_enabled"`
	}
	admin.must(http.StatusOK, http.MethodGet, plans, nil, &listed)
	if listed.Memberships == nil || !*listed.Memberships || len(listed.Items) == 0 {
		t.Fatalf("admin plans list: %+v", listed)
	}
	if r := admin.call(http.MethodPost, plans, body); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate code: %d", r.status)
	}
	if len(plan.Versions) != 1 || !plan.Versions[0].IsCurrent || plan.Versions[0].MonthlyCreditGrant != 200 {
		t.Fatalf("plan: %+v", plan)
	}
	v1 := plan.Versions[0].ID

	c := e.registerClient()
	card := c.attachCard("sbx_a_visa_4242_12_2035")
	res := c.subscribe(v1, card.ID)

	// A new price is a new version; the member keeps theirs.
	var updated service.AdminPlanView
	admin.must(http.StatusCreated, http.MethodPost, plans+"/"+plan.ID.String()+"/versions",
		map[string]any{"price_cents": 1999, "interval": "month", "monthly_credit_grant": 300}, &updated)
	if len(updated.Versions) != 2 || updated.Versions[0].Version != 2 || !updated.Versions[0].IsCurrent || updated.Versions[1].IsCurrent ||
		updated.OpenSubscriptions != 1 {
		t.Fatalf("version 2: %+v", updated)
	}
	var count int64
	e.app.DB.Raw("UPDATE subscription_plan_versions SET price_cents = 1 WHERE id = ?", v1).Scan(&count)
	var price int64
	e.app.DB.Raw("SELECT price_cents FROM subscription_plan_versions WHERE id = ?", v1).Scan(&price)
	if price != 999 {
		t.Fatal("plan versions must be immutable in the database")
	}
	e.renewAt(*res.Subscription.CurrentPeriodEnd, c)
	var renewal int64
	e.app.DB.Raw("SELECT amount_cents FROM payment_transactions WHERE subscription_id = ? AND kind = 'subscription_renewal'", res.Subscription.ID).Scan(&renewal)
	if renewal != 999 {
		t.Fatalf("grandfathered renewal charged %d", renewal)
	}
	// A month went by: the console session expired, so sign in again.
	admin = e.createAdmin()
	e.clock.Advance(11 * time.Minute)
	if r := admin.call(http.MethodPatch, plans+"/"+plan.ID.String(), map[string]any{"active": false}); r.errCode(t) != "step_up_required" {
		t.Fatalf("plan edit without step-up: %d %s", r.status, r.body)
	}
	admin.stepUp()
	// Deactivated plans leave the catalogue; plans with members can't be deleted.
	admin.must(http.StatusOK, http.MethodPatch, plans+"/"+plan.ID.String(), map[string]any{"active": false}, nil)
	var public struct {
		Items []service.PlanView `json:"items"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/plans", nil, &public)
	for _, p := range public.Items {
		if p.ID == plan.ID {
			t.Fatal("inactive plan still on sale")
		}
	}
	if r := admin.call(http.MethodDelete, plans+"/"+plan.ID.String(), nil); r.status != http.StatusConflict {
		t.Fatalf("delete plan with members: %d", r.status)
	}

	// Routing: an allow-listed member goes to sandbox_b.
	vip := e.registerClient()
	var rule service.RoutingRuleView
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/billing/routing-rules", map[string]any{"name": "VIP processor",
		"priority": 5, "providers": []string{"sandbox_b"}, "match_user_ids": []string{vip.user.ID.String()}}, &rule)
	var cfg payment.ClientConfig
	vip.must(http.StatusOK, http.MethodGet, "/api/v1/billing/client-config", nil, &cfg)
	if cfg.Provider != "sandbox_b" {
		t.Fatalf("routed client config: %+v", cfg)
	}
	if r := vip.call(http.MethodPost, "/api/v1/billing/payment-methods", map[string]any{"provider": "sandbox_a", "token": "sbx_a_visa_4242_12_2035",
		"idempotency_key": uniq("card-key-")}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("non-routed provider accepted: %d %s", r.status, r.body)
	}
	vipCard := vip.attachCard("sbx_b_visa_4242_12_2035")
	if vipCard.Provider != "sandbox_b" {
		t.Fatalf("vip card: %+v", vipCard)
	}
	var sim service.RouteSimulation
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/billing/routing-rules/simulate", map[string]any{"user_id": vip.user.ID}, &sim)
	if len(sim.Providers) != 1 || sim.Providers[0] != "sandbox_b" || sim.RuleID != rule.ID.String() {
		t.Fatalf("simulate vip: %+v", sim)
	}
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/billing/routing-rules/simulate", map[string]any{"country": "US"}, &sim)
	if len(sim.Providers) == 0 || sim.Providers[0] != "sandbox_a" {
		t.Fatalf("simulate default: %+v", sim)
	}
	if r := admin.call(http.MethodPost, "/api/v1/admin/billing/routing-rules", map[string]any{"name": "x", "providers": []string{"stripe"}}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown provider rule: %d", r.status)
	}
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/billing/routing-rules/"+rule.ID.String(), map[string]any{"enabled": false}, nil)
	vip.must(http.StatusOK, http.MethodGet, "/api/v1/billing/client-config", nil, &cfg)
	if cfg.Provider != "sandbox_a" {
		t.Fatalf("disabled rule still applied: %+v", cfg)
	}
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/billing/routing-rules/"+rule.ID.String(), nil, nil)
	var providers struct {
		Items []service.ProviderView `json:"items"`
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/billing/providers", nil, &providers)
	if len(providers.Items) != 2 || providers.Items[0].Name != "sandbox_a" || len(providers.Items[1].Capabilities) == 0 {
		t.Fatalf("providers: %+v", providers.Items)
	}

	// Listings, refunds with clawback, admin cancellation.
	var subs service.Page[service.AdminSubscriptionRow]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/billing/subscriptions?user_id="+c.user.ID.String(), nil, &subs)
	if len(subs.Items) != 1 || subs.Items[0].UserID != c.user.ID || subs.Items[0].Status != "active" {
		t.Fatalf("admin subscriptions: %+v", subs.Items)
	}
	if r := admin.call(http.MethodGet, "/api/v1/admin/billing/subscriptions?status=bogus", nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("bad status filter: %d", r.status)
	}
	var txs service.Page[service.AdminTransactionRow]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/billing/transactions?kind=subscription_renewal&user_id="+c.user.ID.String(), nil, &txs)
	if len(txs.Items) != 1 || txs.Items[0].Refundable != 999 || txs.Items[0].ProviderRef == "" {
		t.Fatalf("admin transactions: %+v", txs.Items)
	}
	renewalTx := txs.Items[0].ID
	balance := c.wallet().Balance
	refund := func(amount int64) response {
		return admin.call(http.MethodPost, "/api/v1/admin/billing/transactions/"+renewalTx.String()+"/refund", map[string]any{"amount_cents": amount,
			"reason": "Goodwill gesture"})
	}
	admin.stepUp()
	var row service.AdminTransactionRow
	r := refund(500)
	if r.status != http.StatusOK {
		t.Fatalf("partial refund: %d %s", r.status, r.body)
	}
	r.decode(t, &row)
	if row.Status != "partially_refunded" || row.RefundedAmountCents != 500 || row.Refundable != 499 {
		t.Fatalf("partial refund row: %+v", row)
	}
	// 200 granted credits × 500/999 = 100 clawed back.
	if w := c.wallet(); w.Balance != balance-100 {
		t.Fatalf("partial clawback: %d → %d", balance, w.Balance)
	}
	if r := refund(500); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("over-refund: %d", r.status)
	}
	refund(499).decode(t, &row)
	if row.Status != "refunded" || row.Refundable != 0 {
		t.Fatalf("full refund: %+v", row)
	}
	if w := c.wallet(); w.Balance != balance-200 {
		t.Fatalf("full clawback: %d → %d", balance, w.Balance)
	}
	if r := admin.call(http.MethodPost, "/api/v1/admin/billing/subscriptions/"+res.Subscription.ID.String()+"/cancel",
		map[string]any{"immediate": true}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("cancel without reason: %d", r.status)
	}
	var cancelled service.AdminSubscriptionRow
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/billing/subscriptions/"+res.Subscription.ID.String()+"/cancel",
		map[string]any{"immediate": true, "reason": "Chargeback risk"}, &cancelled)
	if cancelled.Status != "canceled" || !strings.HasPrefix(cancelled.CancelReason, "admin:") {
		t.Fatalf("admin cancel: %+v", cancelled)
	}
	waitNotification(t, c, service.NotifySubscriptionCanceled)
	var audited int64
	e.app.DB.Raw("SELECT count(*) FROM audit_logs WHERE action LIKE 'billing.%'").Scan(&audited)
	if audited < 6 {
		t.Fatalf("billing actions must be audited: %d", audited)
	}
	// The client side never reaches admin billing.
	if r := c.call(http.MethodGet, "/api/v1/admin/billing/plans", nil); r.status == http.StatusOK {
		t.Fatal("client reached the admin API")
	}
}

func TestPaymentRoutingByTrustedCountry(t *testing.T) {
	e := newEnv(t, func(cfg *config.Config) {
		cfg.HTTP.IPSource = "cloudflare"
		cfg.HTTP.TrustedProxies = []string{"127.0.0.1/32", "::1/128"}
	})
	admin := e.createAdmin()
	admin.stepUp()
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/billing/routing-rules", map[string]any{"name": "France",
		"priority": 1, "providers": []string{"sandbox_b"}, "match_countries": []string{"fr"}}, nil)
	defer e.app.DB.Exec("UPDATE payment_routing_rules SET deleted_at = now() WHERE name = 'France'")
	c := e.registerClient()
	get := func(country string) payment.ClientConfig {
		r := c.raw(http.MethodGet, "/api/v1/billing/client-config", nil, map[string]string{"CF-IPCountry": country})
		var cfg payment.ClientConfig
		r.decode(t, &cfg)
		return cfg
	}
	if cfg := get("FR"); cfg.Provider != "sandbox_b" {
		t.Fatalf("FR edge: %+v", cfg)
	}
	if cfg := get("US"); cfg.Provider != "sandbox_a" {
		t.Fatalf("US edge: %+v", cfg)
	}
	if cfg := get("XX"); cfg.Provider != "sandbox_a" {
		t.Fatalf("unknown edge: %+v", cfg)
	}
	// A French card attached from the US edge must satisfy both countries:
	// FR allows only sandbox_b, so a sandbox_a card issued in France is refused.
	r := c.raw(http.MethodPost, "/api/v1/billing/payment-methods", strings.NewReader(fmt.Sprintf(
		`{"provider":"sandbox_a","token":"sbx_a_visa_4242_12_2035_fr","idempotency_key":"%s"}`, uniq("card-key-"))),
		map[string]string{"Content-Type": "application/json", "CF-IPCountry": "US", "X-CSRF-Token": c.csrfToken()})
	if r.status != http.StatusUnprocessableEntity {
		t.Fatalf("card country must be routed too: %d %s", r.status, r.body)
	}
}

// csrfToken returns the client's CSRF token (raw requests send it).
func (c *client) csrfToken() string {
	c.ensureCSRF()
	return c.csrf
}

func TestMembershipLostResponseIsReconciled(t *testing.T) {
	e := newEnv(t)
	// Amounts ending in .15 time out at the sandbox: charged, but the
	// response is lost.
	_, version := e.billingPlan(1015, func(v *service.PlanVersionInput) { v.MonthlyCreditGrant = 50 })
	c := e.registerClient()
	card := c.attachCard("sbx_a_visa_4242_12_2035")
	res := c.subscribe(version, card.ID)
	if res.Subscription.Status != "incomplete" || res.Transaction == nil || res.Transaction.Status != "pending" {
		t.Fatalf("unknown outcome must stay incomplete: %+v", res)
	}
	if e.app.Services.Billing.IsMember(context.Background(), c.user.ID) {
		t.Fatal("incomplete is not a membership")
	}
	e.tick(time.Minute) // lease still running
	if s := c.subscription(); s.Status != "incomplete" {
		t.Fatalf("reconciled before the lease expired: %+v", s)
	}
	e.tick(5 * time.Minute) // lease expired → Lookup finds the charge
	s := c.subscription()
	if s.Status != "active" || s.CurrentPeriodEnd == nil {
		t.Fatalf("reconciled membership: %+v", s)
	}
	if w := c.wallet(); w.Balance != 50 {
		t.Fatalf("grant after reconciliation: %d", w.Balance)
	}
	var charges int64
	e.app.DB.Raw("SELECT count(*) FROM payment_transactions WHERE subscription_id = ? AND status = 'succeeded'", s.ID).Scan(&charges)
	if charges != 1 {
		t.Fatalf("exactly one charge: %d", charges)
	}

	// A provider without Lookup (sandbox_b) is reconciled by replaying the
	// same idempotency key: the provider returns the original charge.
	vip := e.registerClient()
	ids := []string{vip.user.ID.String()}
	name, prio := "Lost-response VIP", 3
	rule, err := e.app.Services.Billing.CreateRoutingRule(context.Background(), service.RoutingRuleInput{Name: &name, Priority: &prio,
		Providers: &[]string{"sandbox_b"}, MatchUserIDs: &ids})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = e.app.Services.Billing.DeleteRoutingRule(context.Background(), rule.ID) }()
	vipCard := vip.attachCard("sbx_b_visa_4242_12_2035")
	pending := vip.subscribe(version, vipCard.ID)
	if pending.Subscription.Status != "incomplete" {
		t.Fatalf("sandbox_b lost response: %+v", pending.Subscription)
	}
	e.tick(6 * time.Minute)
	if s := vip.subscription(); s.Status != "active" {
		t.Fatalf("replayed key must reconcile: %+v", s)
	}
	var vipCharges int64
	e.app.DB.Raw("SELECT count(*) FROM payment_transactions WHERE subscription_id = ? AND status = 'succeeded'", pending.Subscription.ID).Scan(&vipCharges)
	if vipCharges != 1 {
		t.Fatalf("exactly one sandbox_b charge: %d", vipCharges)
	}
}

func TestProviderWebhooksRefundDisputeAndRedelivery(t *testing.T) {
	e := newEnv(t)
	_, version := e.billingPlan(999, func(v *service.PlanVersionInput) { v.MonthlyCreditGrant = 100 })
	c := e.registerClient()
	card := c.attachCard("sbx_a_visa_4242_12_2035")
	res := c.subscribe(version, card.ID)
	var ref string
	e.app.DB.Raw("SELECT provider_ref FROM payment_transactions WHERE subscription_id = ?", res.Subscription.ID).Scan(&ref)
	if ref == "" {
		t.Fatal("no provider reference")
	}
	if r := signedWebhook(e, "sandbox_a", payment.WebhookEvent{ID: "evt_bad", Type: "charge.refunded", ProviderRef: ref}, "t=1,v1=00"); r.status != http.StatusBadRequest {
		t.Fatalf("forged webhook: %d", r.status)
	}
	for _, provider := range []string{"stripe", "sandbox_b"} { // unknown / no webhooks
		if r := signedWebhook(e, provider, payment.WebhookEvent{ID: "evt_x", Type: "charge.refunded", ProviderRef: ref}, ""); r.status != http.StatusBadRequest {
			t.Fatalf("webhook for %s: %d", provider, r.status)
		}
	}
	evt := payment.WebhookEvent{ID: "evt_" + uuid.NewString()[:8], Type: "charge.refunded", ProviderRef: ref, AmountCents: 999}
	if r := signedWebhook(e, "sandbox_a", evt, ""); r.status != http.StatusOK {
		t.Fatalf("refund webhook: %d %s", r.status, r.body)
	}
	if r := signedWebhook(e, "sandbox_a", evt, ""); r.status != http.StatusOK { // redelivery
		t.Fatalf("redelivered webhook: %d", r.status)
	}
	if w := c.wallet(); w.Balance != 0 {
		t.Fatalf("refunded membership credits must be clawed back once: %d", w.Balance)
	}
	var refunds int64
	e.app.DB.Raw("SELECT count(*) FROM payment_transactions WHERE kind = 'refund' AND user_id = ?", c.user.ID).Scan(&refunds)
	if refunds != 1 {
		t.Fatalf("refund rows: %d", refunds)
	}
	var stored string
	e.app.DB.Raw("SELECT payload_sha256 FROM webhook_events WHERE event_id = ?", evt.ID).Scan(&stored)
	if len(stored) != 64 {
		t.Fatal("webhook receipt missing")
	}
	dispute := payment.WebhookEvent{ID: "evt_" + uuid.NewString()[:8], Type: "charge.disputed", ProviderRef: ref}
	if r := signedWebhook(e, "sandbox_a", dispute, ""); r.status != http.StatusOK {
		t.Fatalf("dispute webhook: %d", r.status)
	}
	if s := c.subscription(); s.Status != "canceled" {
		t.Fatalf("disputed membership: %+v", s)
	}
	unknown := payment.WebhookEvent{ID: "evt_" + uuid.NewString()[:8], Type: "charge.succeeded", ProviderRef: "ch_unknown"}
	if r := signedWebhook(e, "sandbox_a", unknown, ""); r.status != http.StatusOK {
		t.Fatalf("unknown charge must be acknowledged: %d", r.status)
	}
}

func TestConcurrentRenewalClaimsAcrossInstances(t *testing.T) {
	proc := payment.NewSandboxProcessor()
	e1 := newEnvWithProcessor(t, proc)
	e2 := newEnvWithProcessor(t, proc)
	e2.clock.Set(e1.clock.Now())
	_, version := e1.billingPlan(999, func(v *service.PlanVersionInput) { v.MonthlyCreditGrant = 10 })
	const members = 12
	subs := make([]service.SubscriptionResult, 0, members)
	clients := make([]*client, 0, members)
	latest := e1.clock.Now()
	for i := 0; i < members; i++ {
		c := e1.registerClient()
		card := c.attachCard("sbx_a_visa_4242_12_2035")
		res := c.subscribe(version, card.ID)
		subs, clients = append(subs, res), append(clients, c)
		if res.Subscription.CurrentPeriodEnd.After(latest) {
			latest = *res.Subscription.CurrentPeriodEnd
		}
	}
	due := latest.Add(time.Second)
	e1.clock.Set(due)
	e2.clock.Set(due)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for _, inst := range []*env{e1, e2, e1, e2} {
		wg.Add(1)
		go func(inst *env) {
			defer wg.Done()
			if _, err := inst.app.Services.Billing.RunDue(context.Background(), 5); err != nil {
				errs <- err
			}
		}(inst)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent run: %v", err)
	}
	// Whatever the claims left, later runs finish (leases protect in-flight rows).
	for i := 0; i < 3; i++ {
		if _, err := e1.app.Services.Billing.RunDue(context.Background(), 50); err != nil {
			t.Fatal(err)
		}
	}
	for i, res := range subs {
		key := fmt.Sprintf("sub:%s:p2:a1", res.Subscription.ID)
		if n := proc.ChargeRequests(key); n != 1 {
			t.Fatalf("member %d: renewal requested %d times", i, n)
		}
		clients[i].relogin()
		if s := clients[i].subscription(); s.Status != "active" || !s.CurrentPeriodStart.Equal(*res.Subscription.CurrentPeriodEnd) {
			t.Fatalf("member %d not renewed once: %+v", i, s)
		}
		if w := clients[i].wallet(); w.Balance != 20 {
			t.Fatalf("member %d grants: %d", i, w.Balance)
		}
	}
	if got := proc.SuccessfulCharges(); got != 2*members {
		t.Fatalf("charges: %d, want %d", got, 2*members)
	}
}

func TestAccountClosureEndsMembershipAndForgetsCards(t *testing.T) {
	e := newEnv(t)
	_, version := e.billingPlan(999)
	c := e.registerClient()
	card := c.attachCard("sbx_a_visa_4242_12_2035")
	c.attachCard("sbx_a_mastercard_4444_01_2034")
	res := c.subscribe(version, card.ID)
	for i := 0; i < 2; i++ { // idempotent
		if err := e.app.Services.Billing.CancelForClosure(context.Background(), c.user.ID); err != nil {
			t.Fatalf("closure %d: %v", i, err)
		}
	}
	if s := c.subscription(); s == nil || s.Status != "canceled" {
		t.Fatalf("membership after closure: %+v", s)
	}
	if cards := c.cards(); len(cards) != 0 {
		t.Fatalf("cards after closure: %+v", cards)
	}
	var reason string
	var liveCustomers, liveMethods int64
	e.app.DB.Raw("SELECT cancel_reason FROM subscriptions WHERE id = ?", res.Subscription.ID).Scan(&reason)
	e.app.DB.Raw("SELECT count(*) FROM payment_customers WHERE user_id = ? AND deleted_at IS NULL", c.user.ID).Scan(&liveCustomers)
	e.app.DB.Raw("SELECT count(*) FROM payment_methods WHERE user_id = ? AND deleted_at IS NULL", c.user.ID).Scan(&liveMethods)
	if reason != "account_closed" || liveCustomers != 0 || liveMethods != 0 {
		t.Fatalf("closure state: reason=%q customers=%d methods=%d", reason, liveCustomers, liveMethods)
	}
	// Nothing is ever charged again.
	e.renewAt(res.Subscription.CurrentPeriodEnd.Add(time.Hour))
	var renewals int64
	e.app.DB.Raw("SELECT count(*) FROM payment_transactions WHERE subscription_id = ? AND kind = 'subscription_renewal'",
		res.Subscription.ID).Scan(&renewals)
	if renewals != 0 {
		t.Fatalf("renewals after closure: %d", renewals)
	}
}

func TestRefundRetriesAndProviderEchoesNeverRefundTwice(t *testing.T) {
	proc := payment.NewSandboxProcessor()
	e := newEnvWithProcessor(t, proc)
	_, version := e.billingPlan(999, func(v *service.PlanVersionInput) { v.MonthlyCreditGrant = 100 })
	c := e.registerClient()
	card := c.attachCard("sbx_a_visa_4242_12_2035")
	res := c.subscribe(version, card.ID)
	if res.Transaction == nil {
		t.Fatal("no initial transaction")
	}
	parent := res.Transaction.ID
	var chargeRef string
	e.app.DB.Raw("SELECT provider_ref FROM payment_transactions WHERE id = ?", parent).Scan(&chargeRef)
	gateway := payment.NewSandboxA(proc, "unused", time.Minute)
	// issued simulates a refund the provider executed but the platform never
	// booked (crash between the provider call and the ledger update).
	issued := func(n int, amount int64) string {
		key := fmt.Sprintf("refund:%s:%d", parent, n)
		now := e.clock.Now()
		row := domain.PaymentTransaction{ID: domain.NewID(), UserID: c.user.ID, Kind: domain.TxRefund, Status: domain.TxPending,
			AmountCents: amount, Currency: "USD", Provider: "sandbox_a", SubscriptionID: &res.Subscription.ID,
			ParentTransactionID: &parent, Description: "Refund", IdempotencyKey: key, AttemptNo: 1, CreatedAt: now, UpdatedAt: now}
		if err := e.app.DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
		r, err := gateway.Refund(context.Background(), payment.RefundRequest{ProviderRef: chargeRef, AmountCents: amount,
			Currency: "USD", IdempotencyKey: key})
		if err != nil {
			t.Fatal(err)
		}
		return r.ProviderRef
	}
	admin := e.createAdmin()
	admin.stepUp()
	refund := func(amount int64) response {
		return admin.call(http.MethodPost, "/api/v1/admin/billing/transactions/"+parent.String()+"/refund",
			map[string]any{"amount_cents": amount, "reason": "Duplicate charge"})
	}
	var row service.AdminTransactionRow

	// A retry completes the stranded refund under its own key.
	firstRef := issued(1, 300)
	if r := refund(500); r.status != http.StatusConflict {
		t.Fatalf("other amount while a refund is pending: %d %s", r.status, r.body)
	}
	r := refund(300)
	if r.status != http.StatusOK {
		t.Fatalf("retried refund: %d %s", r.status, r.body)
	}
	r.decode(t, &row)
	if row.RefundedAmountCents != 300 || row.Status != "partially_refunded" {
		t.Fatalf("retried refund row: %+v", row)
	}
	if w := c.wallet(); w.Balance != 70 { // 100 granted × 300/999
		t.Fatalf("clawback after retry: %d", w.Balance)
	}
	// The provider's echo of a refund we booked is recognised.
	echo := payment.WebhookEvent{ID: "evt_" + uuid.NewString()[:8], Type: "charge.refunded", ProviderRef: chargeRef,
		RefundRef: firstRef, AmountCents: 300}
	if r := signedWebhook(e, "sandbox_a", echo, ""); r.status != http.StatusOK {
		t.Fatalf("echo webhook: %d %s", r.status, r.body)
	}
	// An echo that overtakes the booking books the issued refund itself.
	early := payment.WebhookEvent{ID: "evt_" + uuid.NewString()[:8], Type: "charge.refunded", ProviderRef: chargeRef,
		RefundRef: issued(2, 200), AmountCents: 200}
	if r := signedWebhook(e, "sandbox_a", early, ""); r.status != http.StatusOK {
		t.Fatalf("early webhook: %d %s", r.status, r.body)
	}
	var refunded, children int64
	e.app.DB.Raw("SELECT refunded_amount_cents FROM payment_transactions WHERE id = ?", parent).Scan(&refunded)
	e.app.DB.Raw("SELECT count(*) FROM payment_transactions WHERE parent_transaction_id = ?", parent).Scan(&children)
	if refunded != 500 || children != 2 {
		t.Fatalf("booked %d over %d refund rows, want 500 over 2", refunded, children)
	}
	if w := c.wallet(); w.Balance != 50 {
		t.Fatalf("clawback after echo: %d", w.Balance)
	}
	// The provider refunded exactly 500 so far: the remaining 499 still goes through.
	refund(499).decode(t, &row)
	if row.Status != "refunded" || row.RefundedAmountCents != 999 || row.Refundable != 0 {
		t.Fatalf("final refund: %+v", row)
	}
	if w := c.wallet(); w.Balance != 0 {
		t.Fatalf("final clawback: %d", w.Balance)
	}
}
