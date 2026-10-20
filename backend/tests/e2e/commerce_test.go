package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

func TestPurchaseIdempotencyAndFailure(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	c := e.registerClient()
	w := c.connect("")
	var packs struct {
		Items []service.PackView `json:"items"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/credit-packs", nil, &packs)
	var pack250, failing service.PackView
	for _, p := range packs.Items {
		switch {
		case p.Credits == 250:
			pack250 = p
		case p.PriceCents%100 == 13:
			failing = p
		}
	}
	key := uniq("purchase-key-")
	var first, replay service.PurchaseView
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/credits/purchases", map[string]any{"pack_id": pack250.ID, "idempotency_key": key}, &first)
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/credits/purchases", map[string]any{"pack_id": pack250.ID, "idempotency_key": key}, &replay)
	if first.ID != replay.ID || first.Status != "succeeded" || first.Wallet.Balance != 250 {
		t.Fatalf("purchase: %+v / %+v", first, replay)
	}
	if bal := c.wallet().Balance; bal != 250 {
		t.Fatalf("idempotent replay must not double-credit: %d", bal)
	}
	w.waitMatch(realtime.EventWalletUpdated, `"balance":250`, 5*time.Second)

	// Declined card: no credits, clear error code.
	r := c.call(http.MethodPost, "/api/v1/credits/purchases", map[string]any{"pack_id": failing.ID, "idempotency_key": uniq("purchase-key-")})
	if r.status != http.StatusPaymentRequired || r.errCode(t) != "payment_failed" {
		t.Fatalf("declined: %d %s", r.status, r.body)
	}
	if bal := c.wallet().Balance; bal != 250 {
		t.Fatalf("failed payment changed balance: %d", bal)
	}
	var history service.Page[service.PurchaseView]
	c.must(http.StatusOK, http.MethodGet, "/api/v1/credits/purchases", nil, &history)
	if len(history.Items) != 2 || history.Items[0].Status != "failed" {
		t.Fatalf("purchase history: %+v", history.Items)
	}
	// Invalid input.
	if r := c.call(http.MethodPost, "/api/v1/credits/purchases", map[string]any{"pack_id": pack250.ID, "idempotency_key": "x"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("bad key: %d", r.status)
	}
}

func TestPaymentWebhookSignatureAndRefund(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	c := e.registerClient()
	c.buyCredits(600)
	var history service.Page[service.PurchaseView]
	c.must(http.StatusOK, http.MethodGet, "/api/v1/credits/purchases", nil, &history)
	var ref string
	e.app.DB.Raw("SELECT provider_ref FROM credit_purchases WHERE id = ?", history.Items[0].ID).Scan(&ref)

	body, _ := json.Marshal(payment.WebhookEvent{Type: "payment.refunded", ProviderRef: ref})
	anon := e.client()
	send := func(sig string) response {
		return anon.raw(http.MethodPost, "/api/v1/webhooks/payments", strings.NewReader(string(body)),
			map[string]string{"Content-Type": "application/json", "X-Staryield-Signature": sig})
	}
	if r := send("t=1,v1=deadbeef"); r.status != http.StatusBadRequest {
		t.Fatalf("bad signature accepted: %d", r.status)
	}
	stale := payment.Sign([]byte(e.app.Config.Payments.WebhookSecret), body, e.clock.Now().Add(-time.Hour))
	if r := send(stale); r.status != http.StatusBadRequest {
		t.Fatalf("stale signature accepted: %d", r.status)
	}
	valid := payment.Sign([]byte(e.app.Config.Payments.WebhookSecret), body, e.clock.Now())
	if r := send(valid); r.status != http.StatusOK {
		t.Fatalf("valid webhook: %d %s", r.status, r.body)
	}
	if r := send(valid); r.status != http.StatusOK { // replayed delivery is idempotent
		t.Fatalf("replayed webhook: %d", r.status)
	}
	if bal := c.wallet().Balance; bal != 0 {
		t.Fatalf("refund should claw back credits: %d", bal)
	}
}

func TestCustomRequestLifecycle(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	psychic := e.createPsychic(30)
	admin := e.createAdmin()
	pws := psychic.connect("")

	// Admin lists a service for the psychic (BR-09: pricing is admin-only).
	var svc service.ServiceView
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/psychics/"+psychic.user.ID.String()+"/services",
		map[string]any{"name": "Dream Decode", "subtitle": "Symbolic Analysis", "description": "Send your dream.", "price_credits": 200}, &svc)

	client := e.registerClient()
	cws := client.connect("")

	// Not enough credits → 402, nothing held.
	r := client.call(http.MethodPost, "/api/v1/custom-requests", map[string]any{"service_id": svc.ID, "note": "x", "idempotency_key": uniq("order-key-")})
	if r.status != http.StatusPaymentRequired || r.errCode(t) != "insufficient_credits" {
		t.Fatalf("no credits: %d %s", r.status, r.body)
	}
	client.buyCredits(250)

	// Order → credits held (BR-06).
	var order service.CustomRequestView
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/custom-requests", map[string]any{
		"service_id": svc.ID, "note": "I keep dreaming of a lighthouse.", "idempotency_key": uniq("order-key-"),
	}, &order)
	if order.Status != "pending" || order.Wallet.Balance != 50 || order.Wallet.Held != 200 {
		t.Fatalf("order: %+v", order)
	}
	pws.waitFor(realtime.EventCustomRequestCreated, 5*time.Second) // psychic notified

	// Psychic accepts and delivers → credits released, earnings recorded.
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/custom-requests/"+order.ID.String()+"/accept", nil, nil)
	if r := psychic.call(http.MethodPost, "/api/v1/custom-requests/"+order.ID.String()+"/deliver", map[string]any{"delivery": ""}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("empty delivery: %d", r.status)
	}
	var delivered service.CustomRequestView
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/custom-requests/"+order.ID.String()+"/deliver",
		map[string]any{"delivery": "The lighthouse may reflect a wish for guidance."}, &delivered)
	if delivered.Status != "delivered" || delivered.DeliveredAt == nil {
		t.Fatalf("delivered: %+v", delivered)
	}
	cws.waitMatch(realtime.EventCustomRequestUpdated, `"status":"delivered"`, 5*time.Second)
	if w := client.wallet(); w.Balance != 50 || w.Held != 0 {
		t.Fatalf("after release: %+v", w)
	}
	var earnings service.EarningsReport
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/psychic/earnings", nil, &earnings)
	if earnings.Summary.CustomRequests != 1 || earnings.Summary.NetCredits != 120 {
		t.Fatalf("earnings: %+v", earnings.Summary)
	}

	// Client disputes; admin refunds → credits back, earning reversed.
	client.must(http.StatusOK, http.MethodPost, "/api/v1/custom-requests/"+order.ID.String()+"/dispute", map[string]any{"reason": "Not what I asked for."}, nil)
	var resolved service.CustomRequestView
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/custom-requests/"+order.ID.String()+"/resolve", map[string]any{"refund": true, "note": "Goodwill refund"}, &resolved)
	if resolved.Status != "refunded" {
		t.Fatalf("resolved: %+v", resolved)
	}
	if w := client.wallet(); w.Balance != 250 {
		t.Fatalf("refund: %+v", w)
	}
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/psychic/earnings", nil, &earnings)
	if earnings.Summary.NetCredits != 0 {
		t.Fatalf("earning not reversed: %+v", earnings.Summary)
	}

	// Decline → refund (BR-07); cancel → refund; expiry → refund.
	order2 := service.CustomRequestView{}
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/custom-requests", map[string]any{"service_id": svc.ID, "idempotency_key": uniq("order-key-")}, &order2)
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/custom-requests/"+order2.ID.String()+"/decline", map[string]any{"reason": "Fully booked"}, nil)
	order3 := service.CustomRequestView{}
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/custom-requests", map[string]any{"service_id": svc.ID, "idempotency_key": uniq("order-key-")}, &order3)
	client.must(http.StatusOK, http.MethodPost, "/api/v1/custom-requests/"+order3.ID.String()+"/cancel", nil, nil)
	order4 := service.CustomRequestView{}
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/custom-requests", map[string]any{"service_id": svc.ID, "idempotency_key": uniq("order-key-")}, &order4)
	e.clock.Advance(49 * time.Hour)
	e.maintenance()
	// Access tokens expired with the clock jump: refresh like the browser would.
	if r := client.call(http.MethodGet, "/api/v1/wallet", nil); r.errCode(t) != "session_expired" {
		t.Fatalf("expected an expired access token: %s", r.body)
	}
	client.must(http.StatusOK, http.MethodPost, "/api/v1/auth/refresh", nil, nil)
	var got service.CustomRequestView
	client.must(http.StatusOK, http.MethodGet, "/api/v1/custom-requests/"+order4.ID.String(), nil, &got)
	if got.Status != "expired" {
		t.Fatalf("expiry: %+v", got)
	}
	if w := client.wallet(); w.Balance != 250 || w.Held != 0 {
		t.Fatalf("all refunds must restore the balance: %+v", w)
	}
	// Strangers cannot read the order.
	stranger := e.registerClient()
	if r := stranger.call(http.MethodGet, "/api/v1/custom-requests/"+order.ID.String(), nil); r.status != http.StatusNotFound {
		t.Fatalf("stranger read: %d", r.status)
	}
}

func TestAdminConsoleAndReports(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	client, _, _, _, s := startedSession(t, e, 15, 600)
	client.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/end", nil, nil)

	// Users: search + adjust credits (audited).
	var users service.Page[service.AdminUserView]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/users?q="+client.user.Email[:8]+"&role=client", nil, &users)
	if len(users.Items) == 0 {
		t.Fatal("search returned nothing")
	}
	var w service.WalletView
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/users/"+client.user.ID.String()+"/credits",
		map[string]any{"amount": 25, "reason": "Service recovery", "idempotency_key": uniq("adjust-key-")}, &w)
	if w.Balance != 625 {
		t.Fatalf("adjusted balance: %+v", w)
	}
	if r := admin.call(http.MethodPost, "/api/v1/admin/users/"+client.user.ID.String()+"/credits",
		map[string]any{"amount": -100000, "reason": "x", "idempotency_key": uniq("adjust-key-")}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("negative balance allowed: %d", r.status)
	}

	// Credit packs (BR-09) with audit trail.
	var pack service.PackView
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/credit-packs", map[string]any{"name": "Promo", "credits": 100, "price_cents": 499}, &pack)
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/credit-packs/"+pack.ID.String(), map[string]any{"price_cents": 599, "active": false}, &pack)
	if pack.Active || pack.PriceCents != 599 {
		t.Fatalf("pack: %+v", pack)
	}

	// Monitoring: sessions, audited transcript access.
	var sessions service.Page[service.SessionView]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/chat/sessions?status=settled", nil, &sessions)
	if len(sessions.Items) == 0 || sessions.Items[0].CreditsCharged == nil || sessions.Items[0].PsychicEarnings == nil {
		t.Fatalf("admin sessions: %+v", sessions.Items)
	}
	if r := admin.call(http.MethodGet, "/api/v1/admin/chat/sessions/"+s.ID.String()+"/transcript", nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("transcript without reason: %d", r.status)
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/chat/sessions/"+s.ID.String()+"/transcript?reason=Quality+review", nil, nil)
	var audit service.Page[service.AuditView]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/audit-logs", nil, &audit)
	actions := map[string]bool{}
	for _, a := range audit.Items {
		actions[a.Action] = true
	}
	for _, want := range []string{"wallet.adjusted", "pack.created", "pack.updated", "chat.transcript_viewed"} {
		if !actions[want] {
			t.Errorf("audit missing %s: %v", want, actions)
		}
	}
	if err := e.app.DB.Exec("DELETE FROM audit_logs").Error; err == nil {
		t.Fatal("audit log must be append-only")
	}

	// Reports (FR-14).
	from := e.clock.Now().Add(-time.Hour).Format(time.RFC3339)
	to := e.clock.Now().Add(time.Hour).Format(time.RFC3339)
	var overview map[string]any
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/reports/overview?from="+from+"&to="+to, nil, &overview)
	if overview["purchases"].(float64) < 1 || overview["sessions_by_status"].(map[string]any)["settled"].(float64) < 1 {
		t.Fatalf("overview: %v", overview)
	}
	var daily struct {
		Items []map[string]any `json:"items"`
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/reports/daily?from="+from+"&to="+to, nil, &daily)
	if len(daily.Items) == 0 {
		t.Fatal("daily report empty")
	}
}

func TestAnalyticsIngestionAndQueries(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	visitor := e.client()
	anon := "anon_" + strings.ReplaceAll(uniq("v"), "-", "")
	now := e.clock.Now()
	events := []map[string]any{
		{"event_id": newUUID(), "name": "page_view", "anonymous_id": anon, "path": "/en/psychics?token=secret", "occurred_at": now},
		{"event_id": newUUID(), "name": "psychic_profile_viewed", "anonymous_id": anon, "properties": map[string]any{"slug": "ramone", "bad key!": 1}, "occurred_at": now.Add(time.Second)},
		{"event_id": newUUID(), "name": "chat_cta_clicked", "anonymous_id": anon, "occurred_at": now.Add(2 * time.Second)},
		{"event_id": newUUID(), "name": "credits.purchase_succeeded", "anonymous_id": anon}, // forged server event
	}
	var res struct {
		Accepted int `json:"accepted"`
	}
	visitor.must(http.StatusAccepted, http.MethodPost, "/api/v1/analytics/events", map[string]any{"events": events}, &res)
	if res.Accepted != 3 {
		t.Fatalf("accepted %d, want 3 (server-only names rejected)", res.Accepted)
	}
	// Duplicate delivery is ignored (event_id is unique).
	visitor.must(http.StatusAccepted, http.MethodPost, "/api/v1/analytics/events", map[string]any{"events": events[:1]}, nil)
	visitor.must(http.StatusAccepted, http.MethodPost, "/api/v1/client-logs", map[string]any{"level": "error", "message": "TypeError: x is undefined", "code": "render_failed", "url": "https://staryield.net/en/chatroom?token=abc"}, nil)

	// Server-side business events are tracked too.
	e.registerClient()
	time.Sleep(300 * time.Millisecond) // async batch flush
	e.app.Tracker.Stop(context.Background())

	var stored []struct {
		Name string
		Path string
	}
	e.app.DB.Raw("SELECT name, path FROM analytics_events WHERE anonymous_id = ? ORDER BY occurred_at", anon).Scan(&stored)
	if len(stored) != 3 || stored[0].Name != "web.page_view" || strings.Contains(stored[0].Path, "token") {
		t.Fatalf("stored events: %+v", stored)
	}
	var registered int64
	e.app.DB.Raw("SELECT count(*) FROM analytics_events WHERE name = 'user.registered'").Scan(&registered)
	if registered == 0 {
		t.Fatal("server-side events missing")
	}
	from := now.Add(-time.Hour).Format(time.RFC3339)
	to := now.Add(time.Hour).Format(time.RFC3339)
	var funnel struct {
		Items []struct {
			Name   string `json:"name"`
			Actors int64  `json:"actors"`
		} `json:"items"`
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/analytics/funnel?steps=web.page_view,web.psychic_profile_viewed,web.chat_cta_clicked&from="+from+"&to="+to, nil, &funnel)
	if len(funnel.Items) != 3 || funnel.Items[2].Actors < 1 || funnel.Items[0].Actors < funnel.Items[2].Actors {
		t.Fatalf("funnel: %+v", funnel)
	}
	var series struct {
		Items []struct {
			Count int64 `json:"count"`
		} `json:"items"`
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/analytics/timeseries?name=web.page_view&interval=hour&from="+from+"&to="+to, nil, &series)
	if len(series.Items) == 0 {
		t.Fatal("timeseries empty")
	}
	if r := admin.call(http.MethodGet, "/api/v1/admin/analytics/timeseries?name=web.page_view&interval=century", nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("bad interval: %d", r.status)
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/analytics/top?from="+from+"&to="+to, nil, nil)
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/analytics/events?name=web.page_view&from="+from+"&to="+to, nil, nil)
}

func TestPublicAstrologyEndpoints(t *testing.T) {
	e := newEnv(t)
	c := e.client()
	var places struct {
		Items []map[string]any `json:"items"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/astrology/places?q=par", nil, &places)
	if len(places.Items) == 0 {
		t.Fatal("no places")
	}
	var chart service.ChartView
	c.must(http.StatusOK, http.MethodPost, "/api/v1/astrology/chart", map[string]any{"birth_date": "1945-04-30", "birth_time": "15:30", "place_id": "london-gb"}, &chart)
	if chart.SunSign != "Taurus" || chart.LifePath != 8 || !chart.HasLocation {
		t.Fatalf("chart: sun=%s life=%d loc=%v", chart.SunSign, chart.LifePath, chart.HasLocation)
	}
	if r := c.call(http.MethodPost, "/api/v1/astrology/chart", map[string]any{"birth_date": "3000-01-01"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("future date: %d", r.status)
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/astrology/horoscope/aries", nil, nil)
	if r := c.call(http.MethodGet, "/api/v1/astrology/horoscope/dragon", nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown sign: %d", r.status)
	}
	var events service.CosmicEvents
	c.must(http.StatusOK, http.MethodGet, "/api/v1/astrology/events", nil, &events)
	if len(events.Lunar) != 4 {
		t.Fatalf("lunar events: %+v", events)
	}
}

func newUUID() string {
	return strings.ToLower(strings.TrimSpace(uuidString()))
}
