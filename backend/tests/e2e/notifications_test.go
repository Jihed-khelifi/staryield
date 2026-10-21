package e2e

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

func TestNotificationsAreDeliveredInRealtimeAndCanBeRead(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	psychic := e.createPsychic(10)
	pws := psychic.goOnline()
	client := e.registerClient()
	client.buyCredits(250)
	cws := client.connect("")

	// The purchase produced a notification for the client.
	n := waitNotification(t, client, service.NotifyPurchase)
	if n.Data["credits"] != float64(250) || n.Link != "/profile" {
		t.Fatalf("purchase notification: %+v", n)
	}

	// A chat request notifies the advisor in real time without blocking.
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": psychic.slug(), "request_key": uniq("req-key-")}, nil)
	ev := pws.waitMatch(service.EventNotificationCreated, service.NotifyChatRequested, 5*time.Second)
	var payload struct {
		Notification service.NotificationView `json:"notification"`
	}
	if err := json.Unmarshal(ev.Data, &payload); err != nil || payload.Notification.Data["client_name"] == "" {
		t.Fatalf("realtime notification: %s", ev.Data)
	}
	var count map[string]int64
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/me/notifications/unread-count", nil, &count)
	if count["count"] < 1 {
		t.Fatalf("unread: %v", count)
	}
	var marked map[string]int64
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/me/notifications/read", map[string]any{"ids": []string{payload.Notification.ID.String()}}, &marked)
	if marked["unread"] != count["count"]-1 {
		t.Fatalf("mark one: %v -> %v", count, marked)
	}
	// A user cannot touch someone else's notifications.
	client.must(http.StatusOK, http.MethodPost, "/api/v1/me/notifications/read", map[string]any{"all": true}, &marked)
	if marked["unread"] != 0 {
		t.Fatalf("mark all: %v", marked)
	}
	var page service.Page[service.NotificationView]
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/me/notifications", nil, &page)
	for _, item := range page.Items {
		if item.ID == payload.Notification.ID && item.ReadAt == nil {
			t.Fatal("notification not marked read")
		}
	}
	if r := client.call(http.MethodPost, "/api/v1/me/notifications/read", map[string]any{}); r.errCode(t) != "validation_failed" {
		t.Fatalf("empty mark: %s", r.body)
	}
	if r := e.client().call(http.MethodGet, "/api/v1/me/notifications", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("anonymous list: %d", r.status)
	}
	_ = cws
}

func TestAdminBroadcastsToOnlineAndSpecificUsers(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	online := e.registerClient()
	ows := online.connect("")
	defer ows.conn.Close()
	offline := e.registerClient()
	psychic := e.createPsychic(10)
	psychic.goOnline()

	// Validation: English copy is required, links must be app paths.
	if r := admin.call(http.MethodPost, "/api/v1/admin/broadcasts", map[string]any{"audience": "clients", "title": map[string]string{"fr": "Bonjour"}, "body": map[string]string{"en": "x"}}); r.errCode(t) != "validation_failed" {
		t.Fatalf("missing english: %s", r.body)
	}
	if r := admin.call(http.MethodPost, "/api/v1/admin/broadcasts", map[string]any{"audience": "clients", "title": map[string]string{"en": "Hi"}, "body": map[string]string{"en": "x"}, "link": "https://evil.example"}); r.errCode(t) != "validation_failed" {
		t.Fatalf("external link: %s", r.body)
	}

	// Online clients only.
	var b service.BroadcastView
	r := admin.call(http.MethodPost, "/api/v1/admin/broadcasts", map[string]any{"audience": "clients", "online_only": true,
		"title": map[string]string{"en": "Full moon tonight", "fr": "Pleine lune ce soir", "es": "Luna llena esta noche"},
		"body":  map[string]string{"en": "Ask an advisor", "fr": "Demandez à un médium"}, "link": "/psychics"})
	if r.status != http.StatusAccepted {
		t.Fatalf("broadcast: %d %s", r.status, r.body)
	}
	r.decode(t, &b)
	ev := ows.waitMatch(service.EventNotificationCreated, b.ID.String(), 5*time.Second)
	if !strings.Contains(string(ev.Data), "Pleine lune ce soir") {
		t.Fatalf("localized copy missing: %s", ev.Data)
	}
	e.app.Services.Notifications.Wait()
	var detail service.BroadcastView
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/broadcasts/"+b.ID.String(), nil, &detail)
	if detail.Status != "sent" || detail.Recipients < 1 {
		t.Fatalf("broadcast status: %+v", detail)
	}
	if hasNotification(t, offline, b.ID) || hasNotification(t, psychic, b.ID) {
		t.Fatal("offline client or advisor received an online-clients broadcast")
	}

	// Specific users by email.
	var b2 service.BroadcastView
	admin.must(http.StatusAccepted, http.MethodPost, "/api/v1/admin/broadcasts", map[string]any{"audience": "users",
		"emails": []string{offline.user.Email}, "title": map[string]string{"en": "Your account"}, "body": map[string]string{"en": "Hello"}}, &b2)
	e.app.Services.Notifications.Wait()
	if !hasNotification(t, offline, b2.ID) || hasNotification(t, online, b2.ID) {
		t.Fatal("targeted broadcast delivered to the wrong users")
	}
	if r := admin.call(http.MethodPost, "/api/v1/admin/broadcasts", map[string]any{"audience": "users", "emails": []string{"nobody-" + uuid.NewString() + "@example.com"},
		"title": map[string]string{"en": "x"}, "body": map[string]string{"en": "y"}}); r.errCode(t) != "validation_failed" {
		t.Fatalf("no matching recipients: %s", r.body)
	}

	// Everyone: needs step-up once the window has passed.
	e.clock.Advance(11 * time.Minute)
	if r := admin.call(http.MethodPost, "/api/v1/admin/broadcasts", map[string]any{"audience": "all", "title": map[string]string{"en": "x"}, "body": map[string]string{"en": "y"}}); r.errCode(t) != "step_up_required" {
		t.Fatalf("broadcast to all without step-up: %s", r.body)
	}

	var list service.Page[service.BroadcastView]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/broadcasts", nil, &list)
	if len(list.Items) < 2 {
		t.Fatalf("broadcast history: %d", len(list.Items))
	}
	var audit service.Page[service.AuditView]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/audit-logs?action=broadcast.created", nil, &audit)
	if len(audit.Items) < 2 {
		t.Fatalf("broadcast audit: %d", len(audit.Items))
	}
}

func hasNotification(t *testing.T, c *client, broadcastID uuid.UUID) bool {
	t.Helper()
	var page service.Page[service.NotificationView]
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/notifications?limit=100", nil, &page)
	for _, n := range page.Items {
		if n.Kind == service.NotifyBroadcast && n.Data["broadcast_id"] == broadcastID.String() {
			return true
		}
	}
	return false
}

func TestPsychicAnalyticsAndWithdrawals(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.Billing.MinWithdrawalCredits = 10 })
	admin := e.createAdmin()
	client, psychic, cws, _, s := startedSession(t, e, 40, 600)
	// Five billed minutes, the first three free (sign-up offer), then end:
	// 2 × 40 = 80 credits gross, the advisor earns 60% = 48.
	for i := 0; i < 4; i++ {
		e.tick(60 * time.Second)
	}
	client.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/end", nil, nil)
	cws.drain()

	from := url.QueryEscape(e.clock.Now().Add(-time.Hour).Format(time.RFC3339))
	to := url.QueryEscape(e.clock.Now().Add(time.Hour).Format(time.RFC3339))
	var a service.PsychicAnalytics
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/psychic/analytics?tz=Europe/Paris&from="+from+"&to="+to, nil, &a)
	if a.Earnings.NetCredits != 48 || a.Earnings.Sessions != 1 || a.Sessions.Completed != 1 || a.Sessions.AcceptanceRate != 1 ||
		a.Clients.Unique != 1 || a.Clients.New != 1 || a.Payout.AvailableCredits != 48 || a.Payout.CentsPerCredit != 2 {
		t.Fatalf("analytics: earnings=%+v sessions=%+v clients=%+v payout=%+v", a.Earnings, a.Sessions, a.Clients, a.Payout)
	}
	if a.Range.TZ != "Europe/Paris" || len(a.Hours.Grid) != 7 || len(a.Hours.Grid[0]) != 24 || len(a.Hours.Best) != 1 || a.Score.Value == 0 {
		t.Fatalf("hours/score: %+v %+v", a.Hours.Best, a.Score)
	}
	peaks := 0
	for _, row := range a.Hours.Levels {
		for _, l := range row {
			if l == "peak" {
				peaks++
			}
		}
	}
	if peaks != 1 || len(a.Clients.Top) != 1 || strings.Contains(a.Clients.Top[0].Name, "@") {
		t.Fatalf("levels/top: peaks=%d top=%+v", peaks, a.Clients.Top)
	}
	if r := psychic.call(http.MethodGet, "/api/v1/psychic/analytics?tz=Not/AZone", nil); r.errCode(t) != "validation_failed" {
		t.Fatalf("bad tz: %s", r.body)
	}
	if r := client.call(http.MethodGet, "/api/v1/psychic/analytics", nil); r.status != http.StatusForbidden {
		t.Fatalf("client analytics: %d", r.status)
	}
	var adminView service.PsychicAnalytics
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/psychics/"+s.Psychic.ID.String()+"/analytics?from="+from+"&to="+to, nil, &adminView)
	if adminView.Earnings.NetCredits != 48 {
		t.Fatalf("admin analytics: %+v", adminView.Earnings)
	}

	// Withdrawals: validation, balance and one open request at a time.
	key := uniq("wd-key-")
	if r := psychic.call(http.MethodPost, "/api/v1/psychic/withdrawals", map[string]any{"amount_credits": 5, "method": "paypal", "destination": "me@example.com", "idempotency_key": key}); r.errCode(t) != "validation_failed" {
		t.Fatalf("below minimum: %s", r.body)
	}
	if r := psychic.call(http.MethodPost, "/api/v1/psychic/withdrawals", map[string]any{"amount_credits": 49, "method": "paypal", "destination": "me@example.com", "idempotency_key": key}); r.errCode(t) != "validation_failed" {
		t.Fatalf("over balance: %s", r.body)
	}
	if r := psychic.call(http.MethodPost, "/api/v1/psychic/withdrawals", map[string]any{"amount_credits": 20, "method": "paypal", "destination": "not an email", "idempotency_key": key}); r.errCode(t) != "validation_failed" {
		t.Fatalf("bad paypal: %s", r.body)
	}
	var w service.WithdrawalView
	psychic.must(http.StatusCreated, http.MethodPost, "/api/v1/psychic/withdrawals", map[string]any{"amount_credits": 30, "method": "bank_transfer",
		"destination": "FR76 3000 6000 0112 3456 7890 189", "idempotency_key": key}, &w)
	if w.Status != "pending" || w.AmountCents != 60 || w.DestinationHint != "•••• 0189" {
		t.Fatalf("withdrawal: %+v", w)
	}
	var same service.WithdrawalView
	psychic.must(http.StatusCreated, http.MethodPost, "/api/v1/psychic/withdrawals", map[string]any{"amount_credits": 30, "method": "bank_transfer",
		"destination": "FR76 3000 6000 0112 3456 7890 189", "idempotency_key": key}, &same)
	if same.ID != w.ID {
		t.Fatal("idempotent retry created a second withdrawal")
	}
	if r := psychic.call(http.MethodPost, "/api/v1/psychic/withdrawals", map[string]any{"amount_credits": 10, "method": "paypal", "destination": "me@example.com", "idempotency_key": uniq("wd-key-")}); r.status != http.StatusConflict && r.errCode(t) != "validation_failed" {
		t.Fatalf("second open withdrawal: %d %s", r.status, r.body)
	}
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/psychic/analytics?from="+from+"&to="+to, nil, &a)
	if a.Payout.AvailableCredits != 18 || a.Payout.ReservedCredits != 30 {
		t.Fatalf("payout after request: %+v", a.Payout)
	}

	// Admin processing: destination reveal and approval need step-up.
	e.clock.Advance(11 * time.Minute)
	if r := admin.call(http.MethodGet, "/api/v1/admin/withdrawals/"+w.ID.String()+"/destination", nil); r.errCode(t) != "step_up_required" {
		t.Fatalf("destination without step-up: %s", r.body)
	}
	admin.stepUp()
	var dest map[string]string
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/withdrawals/"+w.ID.String()+"/destination", nil, &dest)
	if dest["destination"] != "FR76 3000 6000 0112 3456 7890 189" {
		t.Fatalf("destination: %v", dest)
	}
	if r := admin.call(http.MethodPost, "/api/v1/admin/withdrawals/"+w.ID.String()+"/pay", map[string]any{}); r.errCode(t) != "invalid_state" {
		t.Fatalf("pay before approve: %s", r.body)
	}
	var aw service.AdminWithdrawalView
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/withdrawals/"+w.ID.String()+"/approve", map[string]any{"note": "Checked"}, &aw)
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/withdrawals/"+w.ID.String()+"/pay", map[string]any{"reference": "SEPA-123"}, &aw)
	if aw.Status != "paid" || aw.Reference != "SEPA-123" || aw.Psychic.Slug != psychic.slug() {
		t.Fatalf("paid: %+v", aw)
	}
	psychic.refresh() // the clock moved past the access token lifetime
	waitNotification(t, psychic, service.NotifyWithdrawal)
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/psychic/analytics?from="+from+"&to="+to, nil, &a)
	if a.Payout.PaidCredits != 30 || a.Payout.AvailableCredits != 18 {
		t.Fatalf("payout after pay: %+v", a.Payout)
	}

	// Reject returns the credits; cancel works for pending requests.
	var w2 service.WithdrawalView
	psychic.must(http.StatusCreated, http.MethodPost, "/api/v1/psychic/withdrawals", map[string]any{"amount_credits": 10, "method": "paypal",
		"destination": "advisor@example.com", "idempotency_key": uniq("wd-key-")}, &w2)
	if w2.DestinationHint != "a•••@example.com" {
		t.Fatalf("paypal hint: %s", w2.DestinationHint)
	}
	if r := admin.call(http.MethodPost, "/api/v1/admin/withdrawals/"+w2.ID.String()+"/reject", map[string]any{}); r.errCode(t) != "validation_failed" {
		t.Fatalf("reject without note: %s", r.body)
	}
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/withdrawals/"+w2.ID.String()+"/reject", map[string]any{"note": "Account mismatch"}, &aw)
	var w3 service.WithdrawalView
	psychic.must(http.StatusCreated, http.MethodPost, "/api/v1/psychic/withdrawals", map[string]any{"amount_credits": 10, "method": "paypal",
		"destination": "advisor@example.com", "idempotency_key": uniq("wd-key-")}, &w3)
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/psychic/withdrawals/"+w3.ID.String()+"/cancel", nil, &w3)
	if w3.Status != "cancelled" {
		t.Fatalf("cancel: %+v", w3)
	}
	var list service.Page[service.WithdrawalView]
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/psychic/withdrawals", nil, &list)
	if len(list.Items) != 3 {
		t.Fatalf("history: %d", len(list.Items))
	}
	var adminList service.Page[service.AdminWithdrawalView]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/withdrawals?status=paid,rejected", nil, &adminList)
	if len(adminList.Items) < 2 {
		t.Fatalf("admin list: %d", len(adminList.Items))
	}
	// Destinations are never in list payloads.
	b, _ := json.Marshal(adminList)
	if strings.Contains(string(b), "FR76") || strings.Contains(string(b), "advisor@example.com") {
		t.Fatal("payout destination leaked in list")
	}
}
