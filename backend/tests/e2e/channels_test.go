package e2e

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/push"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

func (e *env) setPrefs(userID uuid.UUID, prefs map[string]any) {
	e.t.Helper()
	if err := e.app.DB.Model(&domain.Profile{}).Where("user_id = ?", userID).Updates(prefs).Error; err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) outbox(userID uuid.UUID) []domain.EmailOutbox {
	e.t.Helper()
	var rows []domain.EmailOutbox
	if err := e.app.DB.Where("user_id = ?", userID).Order("created_at ASC").Find(&rows).Error; err != nil {
		e.t.Fatal(err)
	}
	return rows
}

func sentTo(msgs []mail.Message, to string) []mail.Message {
	var out []mail.Message
	for _, m := range msgs {
		if m.To == to {
			out = append(out, m)
		}
	}
	return out
}

func TestNotificationEmailsFollowPreferences(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()

	// Notification e-mail is on by default and in the user's language.
	e.setPrefs(c.user.ID, map[string]any{})
	if err := e.app.DB.Model(&domain.User{}).Where("id = ?", c.user.ID).Update("locale", "es").Error; err != nil {
		t.Fatal(err)
	}
	e.notify(service.NotificationSpec{Kind: service.NotifyReviewPrompt, Data: map[string]any{"psychic_name": "Ramone"},
		Link: "/chatroom?reader=ramone"}, c.user.ID)
	msgs := sentTo(e.flushMail(), c.user.Email)
	if len(msgs) != 1 || msgs[0].Subject != "¿Qué tal fue tu lectura?" || msgs[0].Category != "notification" {
		t.Fatalf("notification e-mail: %+v", msgs)
	}
	if !strings.Contains(msgs[0].Text, "/es/chatroom?reader=ramone") || !strings.Contains(msgs[0].HTML, "Ramone") {
		t.Fatalf("link or name missing: %s", msgs[0].Text)
	}
	if msgs[0].Headers["List-Unsubscribe"] != "" && !strings.HasPrefix(msgs[0].Headers["List-Unsubscribe"], "<https://") {
		t.Fatalf("list-unsubscribe: %q", msgs[0].Headers["List-Unsubscribe"])
	}

	// E-mail switched off: notification e-mail stops, billing e-mail does not.
	e.mail.Reset()
	e.setPrefs(c.user.ID, map[string]any{"notify_email": false})
	e.notify(service.NotificationSpec{Kind: service.NotifyChatMissed, Data: map[string]any{"client_name": "Nova"}, Link: "/chatroom"}, c.user.ID)
	e.notify(service.NotificationSpec{Kind: service.NotifyCreditsAdjusted, Data: map[string]any{"amount": 40}, Link: "/profile"}, c.user.ID)
	msgs = sentTo(e.flushMail(), c.user.Email)
	if len(msgs) != 1 || msgs[0].Category != "billing" || !strings.Contains(msgs[0].Text, "40") {
		t.Fatalf("with e-mail off: %+v", msgs)
	}

	// Push-only kinds never e-mail; in-app history keeps everything.
	e.mail.Reset()
	e.setPrefs(c.user.ID, map[string]any{"notify_email": true})
	e.notify(service.NotificationSpec{Kind: service.NotifyPurchase, Data: map[string]any{"credits": 600}, Link: "/profile"}, c.user.ID)
	if msgs := sentTo(e.flushMail(), c.user.Email); len(msgs) != 0 {
		t.Fatalf("push-only kind e-mailed: %+v", msgs)
	}
	var page service.Page[service.NotificationView]
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/notifications", nil, &page)
	if len(page.Items) != 4 {
		t.Fatalf("in-app notifications: %d", len(page.Items))
	}

	// Inactive accounts get nothing outside the app.
	e.mail.Reset()
	if err := e.app.DB.Model(&domain.User{}).Where("id = ?", c.user.ID).Update("status", domain.UserBanned).Error; err != nil {
		t.Fatal(err)
	}
	e.notify(service.NotificationSpec{Kind: service.NotifyWithdrawal, Data: map[string]any{"status": "paid", "amount_credits": 10}}, c.user.ID)
	if msgs := sentTo(e.flushMail(), c.user.Email); len(msgs) != 0 {
		t.Fatalf("banned account e-mailed: %+v", msgs)
	}
}

func TestOutboxSealsAndWipesBodies(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	c.must(http.StatusAccepted, http.MethodPost, "/api/v1/auth/password/forgot", map[string]any{"email": c.user.Email}, nil)
	rows := e.outbox(c.user.ID)
	if len(rows) != 1 || rows[0].Status != domain.EmailQueued || rows[0].Category != domain.EmailSecurity {
		t.Fatalf("outbox: %+v", rows)
	}
	local := strings.Split(c.user.Email, "@")[0]
	if strings.Contains(rows[0].ToSealed, local) || strings.Contains(rows[0].PayloadSealed, "reset-password") || len(rows[0].ToHash) != 64 {
		t.Fatal("recipient or link stored in clear")
	}
	e.flushMail()
	rows = e.outbox(c.user.ID)
	if rows[0].Status != domain.EmailSent || rows[0].PayloadSealed != "" || rows[0].ToSealed != "" || rows[0].SentAt == nil || rows[0].Attempts != 1 {
		t.Fatalf("after delivery: %+v", rows[0])
	}
}

func TestOutboxRetriesAndSuppressesBounces(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	e.mail.SetFail(func(mail.Message) (mail.Result, error) {
		return mail.Result{}, &mail.SendError{Status: 429, Code: 10004, Reason: "email.sending.error.throttled", Retryable: true}
	})
	e.notify(service.NotificationSpec{Kind: service.NotifyWithdrawal, Data: map[string]any{"status": "paid", "amount_credits": 2500}}, c.user.ID)
	e.flushMail()
	row := e.outbox(c.user.ID)[0]
	if row.Status != domain.EmailQueued || row.Attempts != 1 || !strings.Contains(row.LastError, "10004") || !row.NextAttemptAt.After(e.clock.Now()) {
		t.Fatalf("after throttling: %+v", row)
	}
	// Recovered provider: the retry is delivered once the backoff elapsed.
	e.mail.SetFail(nil)
	if msgs := e.flushMail(); len(sentTo(msgs, c.user.Email)) != 0 {
		t.Fatal("retried before the backoff elapsed")
	}
	e.clock.Advance(2 * time.Minute)
	if msgs := sentTo(e.flushMail(), c.user.Email); len(msgs) != 1 {
		t.Fatalf("retry not delivered: %+v", msgs)
	}

	// A permanent bounce suppresses the address for later e-mail.
	e.mail.SetFail(func(m mail.Message) (mail.Result, error) { return mail.Result{Suppressed: []string{m.To}}, nil })
	e.notify(service.NotificationSpec{Kind: service.NotifyWithdrawal, Data: map[string]any{"status": "approved", "amount_credits": 2500}}, c.user.ID)
	e.flushMail()
	e.mail.SetFail(nil)
	e.notify(service.NotificationSpec{Kind: service.NotifyWithdrawal, Data: map[string]any{"status": "rejected", "amount_credits": 2500}}, c.user.ID)
	e.flushMail()
	reasons := map[string]domain.EmailOutbox{}
	for _, r := range e.outbox(c.user.ID) {
		if r.Status == domain.EmailSuppressed {
			reasons[r.StatusReason] = r
		}
	}
	bounced, blocked := reasons["permanent_bounce"], reasons["recipient_suppressed"]
	if bounced.ID == uuid.Nil || blocked.ID == uuid.Nil || blocked.Attempts != 0 {
		t.Fatalf("suppression: %+v", reasons)
	}
	var supp domain.EmailSuppression
	if err := e.app.DB.First(&supp, "email_hash = ?", blocked.ToHash).Error; err != nil || supp.Reason != domain.SuppressHardBounce {
		t.Fatalf("suppression row: %+v %v", supp, err)
	}
}

func TestMarketingNeverReachesTheProvider(t *testing.T) {
	e := newEnv(t)
	optedIn, optedOut := e.registerClient(), e.registerClient()
	e.setPrefs(optedIn.user.ID, map[string]any{"notify_marketing": true})
	e.notify(service.NotificationSpec{Kind: service.NotifyBroadcast, Data: map[string]any{
		"title": map[string]any{"en": "New moon tonight"}, "body": map[string]any{"en": "Join us"}}}, optedIn.user.ID, optedOut.user.ID)
	if msgs := e.flushMail(); len(sentTo(msgs, optedIn.user.Email))+len(sentTo(msgs, optedOut.user.Email)) != 0 {
		t.Fatalf("marketing e-mail sent: %+v", msgs)
	}
	if rows := e.outbox(optedIn.user.ID); len(rows) != 1 || rows[0].Status != domain.EmailCancelled || rows[0].StatusReason != "no_marketing_provider" ||
		rows[0].PayloadSealed != "" {
		t.Fatalf("opted-in marketing row: %+v", rows)
	}
	if rows := e.outbox(optedOut.user.ID); len(rows) != 0 {
		t.Fatalf("marketing queued without opt-in: %+v", rows)
	}
}

func TestDailyEmailCap(t *testing.T) {
	e := newEnv(t, func(c *configAlias) { c.Mail.DailyCap = 2 })
	c := e.registerClient()
	for i := 0; i < 3; i++ {
		e.notify(service.NotificationSpec{Kind: service.NotifyChatMissed, Data: map[string]any{"client_name": "Nova"}}, c.user.ID)
	}
	e.flushMail()
	var capped int
	for _, r := range e.outbox(c.user.ID) {
		if r.StatusReason == "daily_cap" {
			capped++
		}
	}
	if capped != 1 {
		t.Fatalf("capped: %d", capped)
	}
	// Security e-mail still goes out.
	anon := e.client()
	anon.must(http.StatusAccepted, http.MethodPost, "/api/v1/auth/password/forgot", map[string]any{"email": c.user.Email}, nil)
	msgs := sentTo(e.flushMail(), c.user.Email)
	if len(msgs) != 3 || msgs[2].Category != "security" {
		t.Fatalf("security e-mail capped: %d messages", len(msgs))
	}
}

func TestPushConfigAndSubscriptions(t *testing.T) {
	e := newEnv(t, withPush(t))
	anon := e.client()
	var cfg service.PushConfigView
	r := anon.must(http.StatusOK, http.MethodGet, "/api/v1/push/config", nil, &cfg)
	if !cfg.Enabled || cfg.PublicKey != e.app.Config.Push.VAPIDPublicKey || !strings.Contains(r.header.Get("Cache-Control"), "public") {
		t.Fatalf("config: %+v %s", cfg, r.header.Get("Cache-Control"))
	}
	if r := anon.call(http.MethodPost, "/api/v1/me/push-subscriptions", browserSubscription(t, "https://fcm.googleapis.com/fcm/send/x")); r.status != http.StatusUnauthorized {
		t.Fatalf("anonymous subscribe: %d", r.status)
	}

	c := e.registerClient()
	endpoint := "https://fcm.googleapis.com/fcm/send/" + uniq("dev")
	sub := browserSubscription(t, endpoint)
	var view service.PushSubscriptionView
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/me/push-subscriptions", sub, &view)
	var row domain.PushSubscription
	if err := e.app.DB.First(&row, "id = ?", view.ID).Error; err != nil {
		t.Fatal(err)
	}
	if row.UserID != c.user.ID || strings.Contains(row.EndpointSealed, "fcm.googleapis.com") || len(row.EndpointHash) != 64 {
		t.Fatalf("stored subscription: %+v", row)
	}
	// Re-subscribing the same browser updates it in place.
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/me/push-subscriptions", browserSubscription(t, endpoint), nil)
	var n int64
	e.app.DB.Model(&domain.PushSubscription{}).Where("user_id = ?", c.user.ID).Count(&n)
	if n != 1 {
		t.Fatalf("duplicate subscription rows: %d", n)
	}

	// Strict validation: scheme, push-service allow-list (SSRF), keys, shape.
	for name, body := range map[string]map[string]any{
		"http":          browserSubscription(t, "http://fcm.googleapis.com/fcm/send/x"),
		"foreign host":  browserSubscription(t, "https://evil.example/fcm/send/x"),
		"look-alike":    browserSubscription(t, "https://fcm.googleapis.com.evil.example/x"),
		"metadata ip":   browserSubscription(t, "https://169.254.169.254/latest/meta-data"),
		"loopback":      browserSubscription(t, "https://127.0.0.1/x"),
		"custom port":   browserSubscription(t, "https://fcm.googleapis.com:8443/x"),
		"bad keys":      {"endpoint": "https://fcm.googleapis.com/fcm/send/k", "keys": map[string]any{"p256dh": "AAAA", "auth": "AAAA"}},
		"unknown field": {"endpoint": endpoint, "keys": map[string]any{"p256dh": "x", "auth": "y"}, "admin": true},
	} {
		if r := c.call(http.MethodPost, "/api/v1/me/push-subscriptions", body); r.status != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %s", name, r.status, r.body)
		}
	}

	// Another account on the same browser takes the subscription over; the
	// first account can no longer remove it.
	other := e.registerClient()
	other.must(http.StatusCreated, http.MethodPost, "/api/v1/me/push-subscriptions", browserSubscription(t, endpoint), nil)
	c.must(http.StatusNoContent, http.MethodDelete, "/api/v1/me/push-subscriptions", map[string]any{"endpoint": endpoint}, nil)
	var owner domain.PushSubscription
	if err := e.app.DB.First(&owner, "endpoint_hash = ?", row.EndpointHash).Error; err != nil || owner.UserID != other.user.ID {
		t.Fatalf("ownership: %+v %v", owner, err)
	}
	// The owner unsubscribes: soft-deleted.
	other.must(http.StatusNoContent, http.MethodDelete, "/api/v1/me/push-subscriptions", map[string]any{"endpoint": endpoint}, nil)
	var gone domain.PushSubscription
	if err := e.app.DB.Unscoped().First(&gone, "endpoint_hash = ?", row.EndpointHash).Error; err != nil || !gone.DeletedAt.Valid {
		t.Fatalf("unsubscribe: %+v %v", gone, err)
	}

	// At most MaxPerUser browsers per account.
	limit := e.app.Config.Push.MaxPerUser
	heavy := e.registerClient()
	for i := 0; i < limit; i++ {
		heavy.must(http.StatusCreated, http.MethodPost, "/api/v1/me/push-subscriptions",
			browserSubscription(t, "https://updates.push.services.mozilla.com/wpush/v2/"+uniq("m")), nil)
	}
	r = heavy.call(http.MethodPost, "/api/v1/me/push-subscriptions", browserSubscription(t, "https://web.push.apple.com/"+uniq("a")))
	if r.status != http.StatusUnprocessableEntity || !strings.Contains(string(r.body), "10") {
		t.Fatalf("limit: %d %s", r.status, r.body)
	}
}

func TestPushDisabledWithoutKeys(t *testing.T) {
	e := newEnv(t)
	var cfg service.PushConfigView
	e.client().must(http.StatusOK, http.MethodGet, "/api/v1/push/config", nil, &cfg)
	if cfg.Enabled || cfg.PublicKey != "" {
		t.Fatalf("config: %+v", cfg)
	}
	c := e.registerClient()
	r := c.call(http.MethodPost, "/api/v1/me/push-subscriptions", browserSubscription(t, "https://fcm.googleapis.com/fcm/send/x"))
	if r.status != http.StatusServiceUnavailable || r.errCode(t) != "feature_unavailable" {
		t.Fatalf("subscribe while disabled: %d %s", r.status, r.body)
	}
}

func TestPushDeliveryFollowsPreferencesAndPrunes(t *testing.T) {
	e := newEnv(t, withPush(t))
	c := e.registerClient()
	endpoint := "https://fcm.googleapis.com/fcm/send/" + uniq("dev")
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/me/push-subscriptions", browserSubscription(t, endpoint), nil)

	// Push is off by default: nothing is sent.
	e.notify(service.NotificationSpec{Kind: service.NotifyChatRequested, Data: map[string]any{"client_name": "Nova"}, Link: "/chatroom"}, c.user.ID)
	time.Sleep(100 * time.Millisecond)
	if len(e.push.Sent()) != 0 {
		t.Fatal("push sent although the user did not enable it")
	}

	e.setPrefs(c.user.ID, map[string]any{"notify_push": true})
	if err := e.app.DB.Model(&domain.User{}).Where("id = ?", c.user.ID).Update("locale", "fr").Error; err != nil {
		t.Fatal(err)
	}
	sessionID := uuid.New()
	e.notify(service.NotificationSpec{Kind: service.NotifyChatRequested, Data: map[string]any{"session_id": sessionID, "client_name": "Nova"},
		Link: "/chatroom"}, c.user.ID)
	got := e.waitPushes(1)[0]
	if got.Endpoint != endpoint || got.Payload.Title != "Nouvelle demande de chat" || !strings.Contains(got.Payload.Body, "Nova") ||
		got.Payload.URL != e.app.Config.App.FrontendURL+"/fr/chatroom" || got.Urgency != "high" || got.Payload.Tag != "chat.request:"+sessionID.String() {
		t.Fatalf("push: %+v", got)
	}

	// Marketing push needs the marketing opt-in too.
	e.notify(service.NotificationSpec{Kind: service.NotifyBroadcast, Data: map[string]any{"title": map[string]any{"en": "Hi"}}}, c.user.ID)
	time.Sleep(100 * time.Millisecond)
	if len(e.push.Sent()) != 1 {
		t.Fatal("marketing push without opt-in")
	}

	// An expired subscription (410) is pruned.
	e.push.setOutcome(push.Gone)
	e.notify(service.NotificationSpec{Kind: service.NotifyChatMissed, Data: map[string]any{"client_name": "Nova"}}, c.user.ID)
	e.waitPushes(2)
	deadline := time.Now().Add(3 * time.Second)
	for {
		var n int64
		e.app.DB.Model(&domain.PushSubscription{}).Where("user_id = ?", c.user.ID).Count(&n)
		if n == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("gone subscription not pruned")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestSavedMessageReviveAndSoftDelete(t *testing.T) {
	e := newEnv(t)
	client, _, _, _, s := startedSession(t, e, 10, 250)
	var sent struct {
		Message service.MessageView `json:"message"`
	}
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/messages",
		map[string]any{"body": "Keep this one", "client_msg_id": uniq("keep-")}, &sent)
	path := "/api/v1/chat/saved/" + sent.Message.ID.String()
	list := func() int {
		var saved struct {
			Items []service.MessageView `json:"items"`
		}
		client.must(http.StatusOK, http.MethodGet, "/api/v1/chat/saved", nil, &saved)
		return len(saved.Items)
	}
	client.must(http.StatusNoContent, http.MethodPut, path, nil, nil)
	client.must(http.StatusNoContent, http.MethodPut, path, nil, nil) // idempotent
	if list() != 1 {
		t.Fatal("bookmark missing")
	}
	client.must(http.StatusNoContent, http.MethodDelete, path, nil, nil)
	if list() != 0 {
		t.Fatal("removed bookmark still listed")
	}
	var row domain.SavedMessage
	if err := e.app.DB.Unscoped().First(&row, "user_id = ? AND message_id = ?", client.user.ID, sent.Message.ID).Error; err != nil || !row.DeletedAt.Valid {
		t.Fatalf("soft delete: %+v %v", row, err)
	}
	// Saving again revives the same row as a fresh save.
	e.clock.Advance(time.Minute)
	client.must(http.StatusNoContent, http.MethodPut, path, nil, nil)
	if list() != 1 {
		t.Fatal("revived bookmark not listed")
	}
	var rows []domain.SavedMessage
	e.app.DB.Unscoped().Where("user_id = ?", client.user.ID).Find(&rows)
	if len(rows) != 1 || rows[0].DeletedAt.Valid || !rows[0].CreatedAt.After(row.CreatedAt) {
		t.Fatalf("revive: %+v", rows)
	}
}

func TestTimestampsAndSoftDeletedServices(t *testing.T) {
	e := newEnv(t)
	db := e.app.DB
	// Every mutable table has created_at and updated_at.
	for _, table := range []string{"user_profiles", "wallets", "admin_mfa", "refresh_tokens", "password_reset_tokens", "notifications",
		"chat_attachments", "admin_sessions", "saved_messages", "email_outbox", "email_suppressions", "push_subscriptions"} {
		var cols []string
		if err := db.Table("information_schema.columns").Where("table_schema = 'public' AND table_name = ?", table).
			Pluck("column_name", &cols).Error; err != nil {
			t.Fatal(err)
		}
		have := strings.Join(cols, ",")
		if !strings.Contains(have, "created_at") || !strings.Contains(have, "updated_at") {
			t.Errorf("%s lacks timestamps: %s", table, have)
		}
	}
	// The trigger bumps updated_at when a statement leaves it unchanged, and
	// keeps an application-provided value.
	c := e.registerClient()
	var before domain.Profile
	db.First(&before, "user_id = ?", c.user.ID)
	if err := db.Exec("UPDATE user_profiles SET mood = 'Calm', updated_at = updated_at WHERE user_id = ?", c.user.ID).Error; err != nil {
		t.Fatal(err)
	}
	var after domain.Profile
	db.First(&after, "user_id = ?", c.user.ID)
	if after.UpdatedAt.Before(before.UpdatedAt) {
		t.Fatal("updated_at moved backwards")
	}
	pinned := time.Date(2031, 1, 2, 3, 4, 5, 0, time.UTC)
	db.Exec("UPDATE user_profiles SET mood = 'Bold', updated_at = ? WHERE user_id = ?", pinned, c.user.ID)
	db.First(&after, "user_id = ?", c.user.ID)
	if !after.UpdatedAt.Equal(pinned) {
		t.Fatalf("explicit updated_at overridden: %v", after.UpdatedAt)
	}
	if after.HouseSystem != domain.HousePlacidus || !after.NotifyEmail || after.NotifyPush || after.NotifyMarketing || after.CreatedAt.IsZero() {
		t.Fatalf("profile defaults: %+v", after)
	}

	// Removing a service soft-deletes it: gone from the profile, kept for
	// order history, at most one live custom offering per advisor.
	admin := e.createAdmin()
	p := e.createPsychic(10)
	var svc service.ServiceView
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/psychics/"+p.user.ID.String()+"/services",
		map[string]any{"name": "Candle Ritual", "price_credits": 850}, &svc)
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/services/"+svc.ID.String(), nil, nil)
	var raw domain.PsychicService
	if err := db.Unscoped().First(&raw, "id = ?", svc.ID).Error; err != nil || !raw.DeletedAt.Valid || raw.Kind != domain.ServiceStandard || raw.DeliveryDays != 7 {
		t.Fatalf("soft-deleted service: %+v %v", raw, err)
	}
	var profile struct {
		Services []service.ServiceView `json:"services"`
	}
	e.client().must(http.StatusOK, http.MethodGet, "/api/v1/psychics/"+p.slug(), nil, &profile)
	for _, s := range profile.Services {
		if s.ID == svc.ID {
			t.Fatal("deleted service still listed")
		}
	}
	now := e.clock.Now()
	custom := func() error {
		return db.Create(&domain.PsychicService{ID: domain.NewID(), PsychicID: p.user.ID, Name: "Custom", PriceCredits: 100, Active: true,
			Kind: domain.ServiceCustom, DeliveryDays: 3, CreatedAt: now, UpdatedAt: now}).Error
	}
	if err := custom(); err != nil {
		t.Fatal(err)
	}
	if err := custom(); err == nil {
		t.Fatal("two live custom services for one advisor")
	}
}

func TestMailJobDrainsInBackground(t *testing.T) {
	e := newEnv(t, func(c *configAlias) { c.Worker.Enabled = true; c.Mail.PollInterval = time.Hour })
	c := e.registerClient()
	// The enqueue wakes the worker: delivery happens without waiting for the
	// (hour-long) poll interval.
	e.client().must(http.StatusAccepted, http.MethodPost, "/api/v1/auth/password/forgot", map[string]any{"email": c.user.Email}, nil)
	deadline := time.Now().Add(5 * time.Second)
	for len(sentTo(e.mail.Sent(), c.user.Email)) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("background worker did not deliver the e-mail")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestChatRequestPushTagAndTTL checks the chat request push lives as long as
// the request, and that the "missed" push replaces it on the device.
func TestChatRequestPushTagAndTTL(t *testing.T) {
	e := newEnv(t, withPush(t))
	psychic := e.createPsychic(30)
	psychic.must(http.StatusCreated, http.MethodPost, "/api/v1/me/push-subscriptions",
		browserSubscription(t, "https://fcm.googleapis.com/fcm/send/"+uniq("adv")), nil)
	e.setPrefs(psychic.user.ID, map[string]any{"notify_push": true})
	pws := psychic.goOnline()
	client := e.registerClient()

	var s service.SessionView
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": psychic.slug(), "request_key": uniq("req-key-")}, &s)
	pws.waitFor(realtime.EventSessionRequested, 5*time.Second)
	tag := "chat.request:" + s.ID.String()
	req := e.waitPushes(1)[0]
	if req.Payload.Kind != service.NotifyChatRequested || req.Payload.Tag != tag || req.TTL != e.app.Config.Chat.RequestTimeout || req.Urgency != "high" {
		t.Fatalf("request push: %+v (want ttl %v)", req, e.app.Config.Chat.RequestTimeout)
	}

	e.tick(e.app.Config.Chat.RequestTimeout + time.Second)
	missed := e.waitPushes(2)[1]
	if missed.Payload.Kind != service.NotifyChatMissed || missed.Payload.Tag != tag || missed.Topic != req.Topic || missed.TTL != 0 {
		t.Fatalf("missed push: %+v", missed)
	}
}
