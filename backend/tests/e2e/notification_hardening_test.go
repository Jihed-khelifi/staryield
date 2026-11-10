package e2e

// revisions 4.0: notification hardening (docs/platform-extensions.md 23.6):
// read-state events, push subscriptions that end with the session, the push
// payload id, and idempotent, resumable broadcasts at size.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/push"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

func (e *env) pushCount(userID uuid.UUID) int64 {
	e.t.Helper()
	var n int64
	if err := e.app.DB.Model(&domain.PushSubscription{}).Where("user_id = ?", userID).Count(&n).Error; err != nil {
		e.t.Fatal(err)
	}
	return n
}

func (e *env) pushExists(id uuid.UUID) bool {
	e.t.Helper()
	var n int64
	if err := e.app.DB.Model(&domain.PushSubscription{}).Where("id = ?", id).Count(&n).Error; err != nil {
		e.t.Fatal(err)
	}
	return n > 0
}

// subscribePush registers a browser subscription and returns its id.
func (c *client) subscribePush(endpoint string) uuid.UUID {
	c.e.t.Helper()
	var v service.PushSubscriptionView
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/me/push-subscriptions", browserSubscription(c.e.t, endpoint), &v)
	return v.ID
}

func newEndpoint() string { return "https://fcm.googleapis.com/fcm/send/" + uniq("dev") }

func TestNotificationReadReachesEveryConnectionOfTheUser(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	w1, w2 := c.connect(""), c.connect("")
	other := e.registerClient()
	ows := other.connect("")

	for i := 0; i < 3; i++ {
		e.notify(service.NotificationSpec{Kind: service.NotifyPurchase, Data: map[string]any{"credits": 10 + i}, Link: "/profile"}, c.user.ID)
	}
	var page service.Page[service.NotificationView]
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/notifications", nil, &page)
	if len(page.Items) != 3 {
		t.Fatalf("notifications: %d", len(page.Items))
	}
	w1.drain()
	w2.drain()
	ows.drain()

	first := page.Items[0].ID.String()
	var marked map[string]int64
	c.must(http.StatusOK, http.MethodPost, "/api/v1/me/notifications/read", map[string]any{"ids": []string{first}}, &marked)
	for i, w := range []*ws{w1, w2} {
		ev := w.waitFor("notification.read", 5*time.Second)
		var d struct {
			IDs    []string `json:"ids"`
			All    bool     `json:"all"`
			Unread int64    `json:"unread"`
		}
		if err := json.Unmarshal(ev.Data, &d); err != nil || len(d.IDs) != 1 || d.IDs[0] != first || d.All || d.Unread != 2 || marked["unread"] != 2 {
			t.Fatalf("connection %d: %s (%v)", i+1, ev.Data, err)
		}
		if ev.ID != "" {
			t.Fatalf("notification.read is ephemeral, got stream id %q", ev.ID)
		}
	}

	c.must(http.StatusOK, http.MethodPost, "/api/v1/me/notifications/read", map[string]any{"all": true}, &marked)
	for i, w := range []*ws{w1, w2} {
		ev := w.waitFor("notification.read", 5*time.Second)
		var raw map[string]any
		_ = json.Unmarshal(ev.Data, &raw)
		if raw["all"] != true || raw["unread"] != float64(0) || raw["ids"] != nil {
			t.Fatalf("connection %d all-read: %s", i+1, ev.Data)
		}
	}
	// Another user's connection never hears about it.
	ows.expectNone("notification.read", 300*time.Millisecond)
}

func TestLogoutRemovesOnlyThePushSubscriptionNamed(t *testing.T) {
	e := newEnv(t, withPush(t))
	c := e.registerClient()
	here, elsewhere, theirs := newEndpoint(), newEndpoint(), newEndpoint()
	hereID, elsewhereID := c.subscribePush(here), c.subscribePush(elsewhere)
	someone := e.registerClient()
	theirsID := someone.subscribePush(theirs)

	// A malformed body is rejected and the session survives.
	if r := c.call(http.MethodPost, "/api/v1/auth/logout", map[string]any{"push_endpoint": here, "surprise": 1}); r.status < 400 || r.status >= 500 {
		t.Fatalf("unknown field accepted: %d %s", r.status, r.body)
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, nil)

	// Logging out with another user's endpoint removes nothing of theirs.
	c.must(http.StatusNoContent, http.MethodPost, "/api/v1/auth/logout", map[string]any{"push_endpoint": theirs}, nil)
	if !e.pushExists(theirsID) || e.pushCount(c.user.ID) != 2 {
		t.Fatal("logout removed a subscription it does not own or removed the user's own")
	}

	c = e.login(c.user.Email, strongPassword)
	c.must(http.StatusNoContent, http.MethodPost, "/api/v1/auth/logout", map[string]any{"push_endpoint": here}, nil)
	if e.pushExists(hereID) || !e.pushExists(elsewhereID) || e.pushCount(c.user.ID) != 1 {
		t.Fatalf("after logout with push_endpoint: here=%v elsewhere=%v", e.pushExists(hereID), e.pushExists(elsewhereID))
	}
	if r := c.call(http.MethodGet, "/api/v1/me", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("session still valid after logout: %d", r.status)
	}
}

func TestLogoutWithoutABodyStillWorksAndKeepsSubscriptions(t *testing.T) {
	e := newEnv(t, withPush(t))
	c := e.registerClient()
	c.subscribePush(newEndpoint())
	c.must(http.StatusNoContent, http.MethodPost, "/api/v1/auth/logout", nil, nil)
	if r := c.call(http.MethodGet, "/api/v1/me", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("session still valid: %d", r.status)
	}
	if e.pushCount(c.user.ID) != 1 {
		t.Fatal("a plain logout must not remove subscriptions (the browser removes its own through push_endpoint)")
	}
	// An empty JSON object is a valid body too.
	c = e.login(c.user.Email, strongPassword)
	c.must(http.StatusNoContent, http.MethodPost, "/api/v1/auth/logout", map[string]any{}, nil)
	if e.pushCount(c.user.ID) != 1 {
		t.Fatal("empty object removed subscriptions")
	}
}

func TestEndingEverySessionRemovesEveryPushSubscription(t *testing.T) {
	e := newEnv(t, withPush(t))
	subscribeTwo := func(c *client) {
		c.subscribePush(newEndpoint())
		c.subscribePush(newEndpoint())
		if e.pushCount(c.user.ID) != 2 {
			t.Fatal("setup")
		}
	}

	t.Run("logout everywhere", func(t *testing.T) {
		c := e.registerClient()
		subscribeTwo(c)
		c.must(http.StatusNoContent, http.MethodPost, "/api/v1/auth/logout-all", nil, nil)
		if n := e.pushCount(c.user.ID); n != 0 {
			t.Fatalf("%d subscriptions left", n)
		}
	})

	t.Run("password change", func(t *testing.T) {
		c := e.registerClient()
		subscribeTwo(c)
		c.must(http.StatusOK, http.MethodPost, "/api/v1/auth/password/change", map[string]any{"current_password": strongPassword, "new_password": "An0ther!Pass#1"}, nil)
		if n := e.pushCount(c.user.ID); n != 0 {
			t.Fatalf("%d subscriptions left", n)
		}
	})

	t.Run("password reset", func(t *testing.T) {
		c := e.registerClient()
		subscribeTwo(c)
		anon := e.client()
		anon.must(http.StatusAccepted, http.MethodPost, "/api/v1/auth/password/forgot", map[string]any{"email": c.user.Email}, nil)
		sent := e.mailsTo(c.user.Email)
		text := sent[len(sent)-1].Text
		token := strings.Fields(text[strings.Index(text, "token=")+6:])[0]
		anon.must(http.StatusNoContent, http.MethodPost, "/api/v1/auth/password/reset", map[string]any{"token": token, "password": "N3w!Passw0rd#2026"}, nil)
		if n := e.pushCount(c.user.ID); n != 0 {
			t.Fatalf("%d subscriptions left", n)
		}
	})

	t.Run("deactivation", func(t *testing.T) {
		c := e.registerClient()
		subscribeTwo(c)
		c.must(http.StatusNoContent, http.MethodPost, "/api/v1/me/deactivate", map[string]any{"password": strongPassword}, nil)
		if n := e.pushCount(c.user.ID); n != 0 {
			t.Fatalf("%d subscriptions left", n)
		}
	})

	t.Run("closure", func(t *testing.T) {
		c := e.registerClient()
		subscribeTwo(c)
		c.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/close", map[string]any{"password": strongPassword, "confirmation": service.ClosureConfirmation}, nil)
		if n := e.pushCount(c.user.ID); n != 0 {
			t.Fatalf("%d subscriptions left", n)
		}
	})

	t.Run("admin ban", func(t *testing.T) {
		admin := e.createAdmin()
		c := e.registerClient()
		subscribeTwo(c)
		keep := e.registerClient()
		keep.subscribePush(newEndpoint())
		admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/users/"+c.user.ID.String(), map[string]any{"status": "banned", "status_reason": "Abuse"}, nil)
		if n := e.pushCount(c.user.ID); n != 0 {
			t.Fatalf("%d subscriptions left after the ban", n)
		}
		if e.pushCount(keep.user.ID) != 1 {
			t.Fatal("a ban removed someone else's subscription")
		}
	})
}

func TestPushPayloadCarriesTheNotificationIdAndTransientFailuresKeepTheSubscription(t *testing.T) {
	e := newEnv(t, withPush(t))
	c := e.registerClient()
	c.subscribePush(newEndpoint())
	e.setPrefs(c.user.ID, map[string]any{"notify_push": true})

	e.notify(service.NotificationSpec{Kind: service.NotifyPurchase, Data: map[string]any{"purchase_id": uuid.NewString(), "credits": 50}, Link: "/profile"}, c.user.ID)
	got := e.waitPushes(1)[0]
	n := waitNotification(t, c, service.NotifyPurchase)
	if got.Payload.ID != n.ID.String() || got.Payload.Kind != service.NotifyPurchase {
		t.Fatalf("push payload id %q, notification %s", got.Payload.ID, n.ID)
	}

	// A push service outage (network error, 5xx) is retried and never counts
	// towards removing the subscription.
	e.push.setOutcome(push.Transient)
	before := len(e.push.Sent())
	e.notify(service.NotificationSpec{Kind: service.NotifyReviewReceived, Data: map[string]any{"review_id": uuid.NewString(), "rating": 5}, Link: "/profile"}, c.user.ID)
	e.waitPushes(before + 2) // the first attempt and a retry (backoff 1 s)
	if e.pushCount(c.user.ID) != 1 {
		t.Fatal("transient failure removed the subscription")
	}
	var sub domain.PushSubscription
	if err := e.app.DB.Where("user_id = ?", c.user.ID).First(&sub).Error; err != nil || sub.FailureCount != 0 {
		t.Fatalf("failure counter after transient errors: %+v %v", sub, err)
	}
	// A rejection counts; success resets.
	e.push.setOutcome(push.Failed)
	before = len(e.push.Sent())
	e.notify(service.NotificationSpec{Kind: service.NotifyCreditsAdjusted, Data: map[string]any{"amount": 5}, Link: "/profile"}, c.user.ID)
	e.waitPushes(before + 1)
	eventually2(t, func() bool {
		_ = e.app.DB.Where("user_id = ?", c.user.ID).First(&sub).Error
		return sub.FailureCount == 1
	})
}

func eventually2(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// --- broadcasts at size -------------------------------------------------------

const broadcastUsers = 5000

func (e *env) seedBroadcastClients(n int, tag string) {
	e.t.Helper()
	now := e.clock.Now()
	if err := e.app.DB.Exec(`INSERT INTO users (id, email, password_hash, role, status, display_name, locale, terms_accepted_at, created_at, updated_at)
		SELECT gen_random_uuid(), ? || '-' || i || '@bcast.test', 'x', 'client', 'active', 'B ' || i,
		       (ARRAY['en','fr','es'])[1 + (i % 3)], ?::timestamptz, ?::timestamptz, ?::timestamptz
		FROM generate_series(1, ?::int) i`, tag, now, now, now, n).Error; err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() {
		e.app.DB.Exec(`DELETE FROM users WHERE email LIKE ?`, tag+"-%@bcast.test")
	})
}

func (e *env) activeClients() int64 {
	var n int64
	if err := e.app.DB.Raw(`SELECT count(*) FROM users WHERE role = 'client' AND status = 'active'`).Scan(&n).Error; err != nil {
		e.t.Fatal(err)
	}
	return n
}

type broadcastCounts struct{ Rows, Distinct int64 }

func (e *env) broadcastCounts(id uuid.UUID) broadcastCounts {
	var c broadcastCounts
	if err := e.app.DB.Raw(`SELECT count(*) AS rows, count(DISTINCT user_id) AS distinct FROM notifications WHERE broadcast_id = ?`, id).Scan(&c).Error; err != nil {
		e.t.Fatal(err)
	}
	return c
}

func (e *env) recoverBroadcasts() {
	e.t.Helper()
	if err := e.app.Scheduler.RunJob(e.t.Context(), service.BroadcastRecoveryJob); err != nil {
		e.t.Fatalf("broadcast recovery: %v", err)
	}
	e.app.Services.Notifications.Wait()
}

func TestBroadcastToThousandsIsExactIdempotentAndResumable(t *testing.T) {
	if testing.Short() {
		t.Skip("large dataset")
	}
	e := newEnv(t)
	e.seedBroadcastClients(broadcastUsers, "bc"+strings.ReplaceAll(uuid.NewString()[:6], "-", ""))
	admin := e.createAdmin()
	// A listening user proves the realtime event is pipelined to real connections.
	listener := e.registerClient()
	lws := listener.connect("")
	want := e.activeClients()
	if want < broadcastUsers {
		t.Fatalf("seed: %d active clients", want)
	}

	var b service.BroadcastView
	start := time.Now()
	admin.must(http.StatusAccepted, http.MethodPost, "/api/v1/admin/broadcasts", map[string]any{"audience": "clients",
		"title": map[string]string{"en": "Big news"}, "body": map[string]string{"en": "Everyone"}, "link": "/psychics"}, &b)
	lws.waitMatch(service.EventNotificationCreated, b.ID.String(), 20*time.Second)
	e.app.Services.Notifications.Wait()
	t.Logf("broadcast to %d clients took %s", want, time.Since(start).Round(time.Millisecond))

	var detail service.BroadcastView
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/broadcasts/"+b.ID.String(), nil, &detail)
	got := e.broadcastCounts(b.ID)
	if detail.Status != "sent" || int64(detail.Recipients) != want || got.Rows != want || got.Distinct != want {
		t.Fatalf("first run: status=%s recipients=%d rows=%d distinct=%d, want %d", detail.Status, detail.Recipients, got.Rows, got.Distinct, want)
	}

	// Database rule: one notification per broadcast and user.
	var someone uuid.UUID
	if err := e.app.DB.Raw(`SELECT user_id FROM notifications WHERE broadcast_id = ? LIMIT 1`, b.ID).Row().Scan(&someone); err != nil {
		t.Fatal(err)
	}
	err := e.app.DB.Exec(`INSERT INTO notifications (id, user_id, kind, broadcast_id) VALUES (?, ?, 'broadcast', ?)`, uuid.New(), someone, b.ID).Error
	if err == nil {
		t.Fatal("duplicate (broadcast_id, user_id) accepted")
	}

	// Runs again from the start (a crash right before the final status
	// write, or a second instance): nothing is duplicated.
	stale := e.clock.Now().Add(-10 * time.Minute)
	if err := e.app.DB.Exec(`UPDATE broadcasts SET status = 'sending', cursor_user_id = NULL, heartbeat_at = ? WHERE id = ?`, stale, b.ID).Error; err != nil {
		t.Fatal(err)
	}
	e.recoverBroadcasts()
	got = e.broadcastCounts(b.ID)
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/broadcasts/"+b.ID.String(), nil, &detail)
	if detail.Status != "sent" || int64(detail.Recipients) != want || got.Rows != want || got.Distinct != want {
		t.Fatalf("second run: status=%s recipients=%d rows=%d distinct=%d, want %d", detail.Status, detail.Recipients, got.Rows, got.Distinct, want)
	}

	// Resume after an interrupted run: a broadcast stuck in `sending` with
	// its fan-out half done (cursor, notifications up to it) is completed.
	mid := uuid.Nil
	if err := e.app.DB.Raw(`SELECT id FROM users WHERE role = 'client' AND status = 'active' ORDER BY id OFFSET ? LIMIT 1`, want/2).Row().Scan(&mid); err != nil {
		t.Fatal(err)
	}
	b2 := domain.Broadcast{ID: domain.NewID(), Audience: "clients", Title: domain.JSONMap{"en": "Resumed"}, Body: domain.JSONMap{"en": "Half done"},
		Link: "", Status: "sending", CreatedAt: stale, CursorUserID: &mid, HeartbeatAt: &stale, TargetIDs: domain.StringList{}}
	if err := e.app.DB.Create(&b2).Error; err != nil {
		t.Fatal(err)
	}
	if err := e.app.DB.Exec(`INSERT INTO notifications (id, user_id, kind, data, link, broadcast_id, created_at, updated_at)
		SELECT gen_random_uuid(), id, 'broadcast', '{}'::jsonb, '', ?, now(), now() FROM users WHERE role = 'client' AND status = 'active' AND id <= ?`, b2.ID, mid).Error; err != nil {
		t.Fatal(err)
	}
	var already int64
	e.app.DB.Raw(`SELECT count(*) FROM notifications WHERE broadcast_id = ?`, b2.ID).Scan(&already)
	if already == 0 || already >= want {
		t.Fatalf("setup: %d rows already delivered", already)
	}
	e.recoverBroadcasts()
	got = e.broadcastCounts(b2.ID)
	var resumed service.BroadcastView
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/broadcasts/"+b2.ID.String(), nil, &resumed)
	if resumed.Status != "sent" || int64(resumed.Recipients) != want || got.Rows != want || got.Distinct != want {
		t.Fatalf("resumed: status=%s recipients=%d rows=%d distinct=%d, want %d", resumed.Status, resumed.Recipients, got.Rows, got.Distinct, want)
	}

	// A fresh heartbeat is left alone by the recovery job.
	fresh := domain.Broadcast{ID: domain.NewID(), Audience: "clients", Title: domain.JSONMap{"en": "Running"}, Body: domain.JSONMap{"en": "Not stale"},
		Status: "sending", CreatedAt: e.clock.Now(), HeartbeatAt: ptr(e.clock.Now()), TargetIDs: domain.StringList{}}
	if err := e.app.DB.Create(&fresh).Error; err != nil {
		t.Fatal(err)
	}
	e.recoverBroadcasts()
	if c := e.broadcastCounts(fresh.ID); c.Rows != 0 {
		t.Fatalf("recovery resumed a broadcast whose runner is alive: %d rows", c.Rows)
	}
}

func TestNotificationRetentionDeletesInBatches(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	old := e.clock.Now().AddDate(0, 0, -200)
	// 12 000 expired rows (three batches of 5 000) and 5 recent ones.
	if err := e.app.DB.Exec(`INSERT INTO notifications (id, user_id, kind, data, link, created_at, updated_at)
		SELECT gen_random_uuid(), ?, 'purchase.succeeded', '{}'::jsonb, '', ?::timestamptz + (i * interval '1 second'), now() FROM generate_series(1, 12000) i`, c.user.ID, old).Error; err != nil {
		t.Fatal(err)
	}
	if err := e.app.DB.Exec(`INSERT INTO notifications (id, user_id, kind, data, link, created_at, updated_at)
		SELECT gen_random_uuid(), ?, 'purchase.succeeded', '{}'::jsonb, '', ?::timestamptz, now() FROM generate_series(1, 5) i`, c.user.ID, e.clock.Now()).Error; err != nil {
		t.Fatal(err)
	}
	e.maintenance() // runs the "old notifications" cleanup
	var left int64
	e.app.DB.Raw(`SELECT count(*) FROM notifications WHERE user_id = ?`, c.user.ID).Scan(&left)
	if left != 5 {
		t.Fatalf("%d notifications left, want the 5 recent ones", left)
	}
}
