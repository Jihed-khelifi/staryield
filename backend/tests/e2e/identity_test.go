package e2e

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/app"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
)

const (
	uaIPhone = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1"
	uaMac    = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36"
)

// loginAs signs in from a browser with the given User-Agent.
func (e *env) loginAs(email, password, ua string) *client {
	e.t.Helper()
	c := e.client()
	c.ensureCSRF()
	body, _ := json.Marshal(map[string]any{"email": email, "password": password})
	r := c.raw(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body),
		map[string]string{"Content-Type": "application/json", "X-CSRF-Token": c.csrf, "User-Agent": ua})
	if r.status != http.StatusOK {
		e.t.Fatalf("login: %d %s", r.status, r.body)
	}
	var out struct {
		User *service.UserView `json:"user"`
	}
	r.decode(e.t, &out)
	c.user, c.pw = out.User, password
	return c
}

// accessToken returns the access cookie of a browser.
func (c *client) accessToken() string {
	u, _ := url.Parse(c.e.srv.URL)
	for _, ck := range c.http.Jar.Cookies(u) {
		if ck.Name == "sy_at" {
			return ck.Value
		}
	}
	c.e.t.Fatal("no access cookie")
	return ""
}

// bearerStatus calls path with an access token and no cookies.
func (e *env) bearerStatus(token, path string) int {
	return e.client().raw(http.MethodGet, path, nil, map[string]string{"Authorization": "Bearer " + token}).status
}

func (c *client) devices() []service.DeviceView {
	c.e.t.Helper()
	var page service.Page[service.DeviceView]
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/sessions", nil, &page)
	return page.Items
}

func (c *client) currentDevice() service.DeviceView {
	c.e.t.Helper()
	for _, d := range c.devices() {
		if d.Current {
			return d
		}
	}
	c.e.t.Fatal("no current device")
	return service.DeviceView{}
}

// mailTo returns the last e-mail delivered to an address whose text
// contains marker, or fails.
func mailTo(t *testing.T, sent []mail.Message, to, marker string) mail.Message {
	t.Helper()
	for i := len(sent) - 1; i >= 0; i-- {
		if strings.EqualFold(sent[i].To, to) && strings.Contains(sent[i].Text, marker) {
			return sent[i]
		}
	}
	t.Fatalf("no e-mail to %s containing %q among %d", to, marker, len(sent))
	return mail.Message{}
}

func tokenIn(t *testing.T, m mail.Message) string {
	t.Helper()
	i := strings.Index(m.Text, "token=")
	if i < 0 {
		t.Fatalf("no token in %q", m.Text)
	}
	return strings.Fields(m.Text[i+len("token="):])[0]
}

func (e *env) auditCount(action string, target uuid.UUID) int64 {
	var n int64
	e.app.DB.Model(&domain.AuditLog{}).Where("action = ? AND target_id = ?", action, target.String()).Count(&n)
	return n
}

func TestDevicesListAndRevokeOne(t *testing.T) {
	e := newEnv(t)
	a := e.registerClient()
	b := e.loginAs(a.user.Email, strongPassword, uaIPhone)
	c := e.loginAs(a.user.Email, strongPassword, uaMac)
	aws, bws := a.connect(""), b.connect("")

	list := a.devices()
	if len(list) != 3 {
		t.Fatalf("devices: %+v", list)
	}
	var current int
	var bDev, cDev service.DeviceView
	for _, d := range list {
		if d.Current {
			current++
		}
		switch d.Label {
		case "Safari on iPhone":
			bDev = d
		case "Chrome on macOS":
			cDev = d
		}
	}
	if current != 1 || bDev.ID == uuid.Nil || bDev.DeviceType != "mobile" || cDev.ID == uuid.Nil || cDev.OS != "macOS" {
		t.Fatalf("device labels: %+v", list)
	}
	aDev := a.currentDevice()
	// Keyset pagination, most recent activity first.
	seenIDs := map[uuid.UUID]bool{}
	next := "/api/v1/me/sessions?limit=2"
	for pages := 0; next != ""; pages++ {
		var page service.Page[service.DeviceView]
		a.must(http.StatusOK, http.MethodGet, next, nil, &page)
		for i, d := range page.Items {
			if seenIDs[d.ID] || (i > 0 && d.LastSeenAt.After(page.Items[i-1].LastSeenAt)) {
				t.Fatalf("page order: %+v", page.Items)
			}
			seenIDs[d.ID] = true
		}
		next = ""
		if page.NextCursor != "" {
			next = "/api/v1/me/sessions?limit=2&cursor=" + url.QueryEscape(page.NextCursor)
		}
		if pages > 3 {
			t.Fatal("pagination does not end")
		}
	}
	if len(seenIDs) != 3 {
		t.Fatalf("paged devices: %d", len(seenIDs))
	}

	// Signing B out from A kills B's access token at once (no expiry wait)
	// and closes only B's realtime connection.
	a.must(http.StatusNoContent, http.MethodDelete, "/api/v1/me/sessions/"+bDev.ID.String(), nil, nil)
	if r := b.call(http.MethodGet, "/api/v1/me", nil); r.status != http.StatusUnauthorized || r.errCode(t) != "session_expired" {
		t.Fatalf("revoked device still authenticated: %d %s", r.status, r.body)
	}
	if code := bws.waitClosed(5 * time.Second); code != realtime.CloseUnauthorized {
		t.Fatalf("revoked device websocket close code %d", code)
	}
	select {
	case <-aws.closed:
		t.Fatal("the other device's websocket was closed")
	case <-time.After(300 * time.Millisecond):
	}
	if r := b.call(http.MethodPost, "/api/v1/auth/refresh", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("revoked device refreshed: %d", r.status)
	}
	var row domain.UserSession
	e.app.DB.First(&row, "id = ?", bDev.ID)
	if row.RevokedAt == nil || row.RevokeReason != "device_signed_out" {
		t.Fatalf("session row: %+v", row)
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, nil)
	a.must(http.StatusNoContent, http.MethodDelete, "/api/v1/me/sessions/"+bDev.ID.String(), nil, nil) // idempotent
	if len(a.devices()) != 2 {
		t.Fatal("revoked device still listed")
	}

	// IDOR: another account can neither see nor revoke these devices.
	x := e.registerClient()
	if r := x.call(http.MethodDelete, "/api/v1/me/sessions/"+aDev.ID.String(), nil); r.status != http.StatusNotFound {
		t.Fatalf("foreign device revoke: %d", r.status)
	}
	if r := x.call(http.MethodDelete, "/api/v1/me/sessions/not-a-uuid", nil); r.status != http.StatusNotFound {
		t.Fatalf("malformed id: %d", r.status)
	}
	if got := x.devices(); len(got) != 1 || got[0].ID == aDev.ID {
		t.Fatalf("foreign devices listed: %+v", got)
	}
	a.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, nil)

	// Sign out everywhere else; the /me/devices alias lists the same rows.
	if r := a.call(http.MethodDelete, "/api/v1/me/sessions", nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("missing scope: %d", r.status)
	}
	var out struct {
		SignedOut int `json:"signed_out"`
	}
	a.must(http.StatusOK, http.MethodDelete, "/api/v1/me/sessions?scope=others", nil, &out)
	if out.SignedOut != 1 || c.call(http.MethodGet, "/api/v1/me", nil).status != http.StatusUnauthorized {
		t.Fatalf("sign out others: %+v", out)
	}
	var alias service.Page[service.DeviceView]
	a.must(http.StatusOK, http.MethodGet, "/api/v1/me/devices", nil, &alias)
	if len(alias.Items) != 1 || alias.Items[0].ID != aDev.ID || !alias.Items[0].Current {
		t.Fatalf("alias: %+v", alias.Items)
	}

	// Signing out the current device clears this browser's session.
	a.must(http.StatusNoContent, http.MethodDelete, "/api/v1/me/sessions/"+aDev.ID.String(), nil, nil)
	if r := a.call(http.MethodGet, "/api/v1/me", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("current device after self-revoke: %d", r.status)
	}

	// Refresh tokens written without a device row (e.g. by the previous
	// release during a rolling deploy) still get one.
	fam := uuid.New()
	now := e.clock.Now()
	if err := e.app.DB.Create(&domain.RefreshToken{ID: uuid.New(), UserID: x.user.ID, FamilyID: fam, TokenHash: security.HashToken(uuid.NewString()),
		ExpiresAt: now.Add(time.Hour), UserAgent: uaMac, CreatedAt: now}).Error; err != nil {
		t.Fatalf("legacy token insert: %v", err)
	}
	var legacy domain.UserSession
	if err := e.app.DB.First(&legacy, "id = ?", fam).Error; err != nil || legacy.UserID != x.user.ID || !legacy.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("device row for a legacy token: %+v %v", legacy, err)
	}
}

func TestLogoutEndsOnlyThisDeviceImmediately(t *testing.T) {
	e := newEnv(t)
	a := e.registerClient()
	b := e.login(a.user.Email, strongPassword)
	aws, bws := a.connect(""), b.connect("")
	token := a.accessToken()
	dev := a.currentDevice()
	if st := e.bearerStatus(token, "/api/v1/me"); st != http.StatusOK {
		t.Fatalf("token before logout: %d", st)
	}
	a.must(http.StatusNoContent, http.MethodPost, "/api/v1/auth/logout", nil, nil)
	// The access token captured before logout no longer works, although it
	// has not expired.
	if st := e.bearerStatus(token, "/api/v1/me"); st != http.StatusUnauthorized {
		t.Fatalf("access token survived logout: %d", st)
	}
	if code := aws.waitClosed(5 * time.Second); code != realtime.CloseUnauthorized {
		t.Fatalf("websocket close code %d", code)
	}
	b.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, nil)
	select {
	case <-bws.closed:
		t.Fatal("logout closed another device's websocket")
	case <-time.After(300 * time.Millisecond):
	}
	var row domain.UserSession
	e.app.DB.First(&row, "id = ?", dev.ID)
	if row.RevokedAt == nil || row.RevokeReason != "logout" {
		t.Fatalf("session row: %+v", row)
	}
}

func TestDeviceActivityIsThrottled(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	dev := c.currentDevice()
	seen := func() time.Time {
		var s domain.UserSession
		e.app.DB.First(&s, "id = ?", dev.ID)
		return s.LastSeenAt
	}
	first := seen()
	e.clock.Advance(20 * time.Second)
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, nil)
	if !seen().Equal(first) {
		t.Fatal("activity written more than once a minute")
	}
	// Once the per-minute marker lapses, the next request records activity.
	e.app.Redis.Del(context.Background(), e.app.Config.Redis.KeyPrefix+"auth:seen:"+dev.ID.String())
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, nil)
	if got := seen(); !got.Equal(e.clock.Now()) {
		t.Fatalf("last seen %v, want %v", got, e.clock.Now())
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	var v service.SettingsView
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/settings", nil, &v)
	if v.Version != 1 || v.Account.Email != c.user.Email || v.Profile.BirthDate != "1990-05-17" || v.Appearance.Locale != "en" ||
		v.Privacy.PolicyVersion == "" || !v.Privacy.Consents["personalization"] || v.Privacy.Consents["analytics"] {
		t.Fatalf("initial settings: %+v", v)
	}
	c.must(http.StatusOK, http.MethodPatch, "/api/v1/me/settings", map[string]any{
		"expected_version": 1,
		"account":          map[string]any{"display_name": "Luna"},
		"profile":          map[string]any{"birth_time": "07:45", "birth_place_id": "paris-fr", "house_system": "whole_sign", "nickname": "Moon"},
		"appearance":       map[string]any{"theme": "Light Cream", "locale": "fr", "time_zone": "Europe/Paris"},
		"notifications":    map[string]any{"email": false, "push": true, "marketing": true, "horoscope": false},
		"privacy":          map[string]any{"share_birth_details_with_advisors": true, "consents": map[string]any{"analytics": true}},
	}, &v)
	check := func(v service.SettingsView) {
		t.Helper()
		if v.Account.DisplayName != "Luna" || v.Profile.BirthTime != "07:45" || v.Profile.BirthPlace != "Paris, France" ||
			v.Profile.BirthTimezone != "Europe/Paris" || v.Profile.HouseSystem != "whole_sign" || v.Appearance.Theme != "Light Cream" ||
			v.Appearance.Locale != "fr" || v.Appearance.TimeZone != "Europe/Paris" || v.Notifications.Email || !v.Notifications.Push ||
			!v.Notifications.Marketing || v.Notifications.Horoscope || !v.Privacy.ShareBirthDetailsWithAdvisors || !v.Privacy.Consents["analytics"] {
			t.Fatalf("settings: %+v", v)
		}
	}
	check(v)
	if v.Version != 2 {
		t.Fatalf("version %d", v.Version)
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/settings", nil, &v)
	check(v)
	var me service.UserView
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, &me)
	if me.Locale != "fr" || me.DisplayName != "Luna" {
		t.Fatalf("/me: %+v", me)
	}

	// A save from a stale tab is refused.
	r := c.call(http.MethodPatch, "/api/v1/me/settings", map[string]any{"expected_version": 1, "account": map[string]any{"display_name": "Old tab"}})
	var conflict struct {
		Error struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	r.decode(t, &conflict)
	if r.status != http.StatusConflict || conflict.Error.Code != "version_conflict" || conflict.Error.Details["current_version"] != float64(2) {
		t.Fatalf("stale save: %d %s", r.status, r.body)
	}
	// Problems are reported per dotted field (translated), and nothing is saved.
	c.locale = "fr"
	r = c.call(http.MethodPatch, "/api/v1/me/settings", map[string]any{"account": map[string]any{"display_name": "Changed"},
		"profile": map[string]any{"birth_time": "25:00"}, "appearance": map[string]any{"time_zone": "Mars/Base"}})
	var invalid localizedError
	r.decode(t, &invalid)
	if r.status != http.StatusUnprocessableEntity || invalid.Error.Details.Fields["profile.birth_time"] == "" ||
		invalid.Error.Details.Fields["appearance.time_zone"] != "Choisissez un fuseau horaire dans la liste." {
		t.Fatalf("validation: %d %s", r.status, r.body)
	}
	c.locale = ""
	if r := c.call(http.MethodPatch, "/api/v1/me/settings", map[string]any{"account": map[string]any{"email": "x@example.com"}}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("e-mail through settings: %d", r.status)
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/settings", nil, &v)
	if v.Account.DisplayName != "Luna" || v.Version != 2 {
		t.Fatalf("rejected patch saved: %+v", v.Account)
	}

	// The PATCH /me/profile alias goes through the same transactional path.
	c.must(http.StatusOK, http.MethodPatch, "/api/v1/me/profile", map[string]any{"nickname": "Star", "mood": "Calm"}, &me)
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/settings", nil, &v)
	if me.Profile.Nickname != "Star" || v.Version != 3 || v.Profile.Mood != "Calm" {
		t.Fatalf("alias: %+v version %d", me.Profile, v.Version)
	}
	// Places from the GeoNames search are accepted with their label.
	c.must(http.StatusOK, http.MethodPatch, "/api/v1/me/settings", map[string]any{"profile": map[string]any{
		"birth_place_id": "gn:2988507", "birth_place_label": "Paris, Île-de-France, France"}}, &v)
	if v.Profile.BirthPlaceID != "gn:2988507" || v.Profile.BirthPlace != "Paris, Île-de-France, France" {
		t.Fatalf("geonames place: %+v", v.Profile)
	}
	var consents service.ConsentsView
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/consents", nil, &consents)
	for _, it := range consents.Items {
		if it.Category == "analytics" && (!it.Granted || !it.Explicit || it.Source != "settings") {
			t.Fatalf("consent from settings: %+v", it)
		}
	}
}

func TestEmailChangeFlow(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	other := e.login(c.user.Email, strongPassword)
	oldEmail, newEmail := c.user.Email, uniq("moved")+"@example.com"

	if r := c.call(http.MethodPost, "/api/v1/me/email-change", map[string]any{"email": newEmail, "password": "Wr0ng!pass1"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("wrong password: %d", r.status)
	}
	c.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/email-change", map[string]any{"email": newEmail, "password": strongPassword}, nil)
	sent := e.flushMail()
	confirm := mailTo(t, sent, newEmail, "/confirm-email?token=")
	notice := mailTo(t, sent, oldEmail, "/cancel-email-change?token=")
	if confirm.Category != "security" || notice.Category != "security" {
		t.Fatalf("categories: %s %s", confirm.Category, notice.Category)
	}
	var v service.SettingsView
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/settings", nil, &v)
	if v.Account.PendingEmail == "" || v.Account.Email != oldEmail {
		t.Fatalf("pending change: %+v", v.Account)
	}

	// Confirming needs no session and ends every session.
	anon := e.client()
	var res service.EmailChangeResult
	token := tokenIn(t, confirm)
	anon.must(http.StatusOK, http.MethodPost, "/api/v1/auth/email-change/confirm", map[string]any{"token": token}, &res)
	if res.Status != "confirmed" {
		t.Fatalf("confirm: %+v", res)
	}
	for _, s := range []*client{c, other} {
		if r := s.call(http.MethodGet, "/api/v1/me", nil); r.status != http.StatusUnauthorized {
			t.Fatalf("session survived the e-mail change: %d", r.status)
		}
	}
	if r := anon.call(http.MethodPost, "/api/v1/auth/email-change/confirm", map[string]any{"token": token}); r.errCode(t) != "invalid_token" {
		t.Fatalf("confirmation reuse: %s", r.body)
	}
	if r := e.client().call(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": oldEmail, "password": strongPassword}); r.status != http.StatusUnauthorized {
		t.Fatalf("old address still signs in: %d", r.status)
	}
	moved := e.login(newEmail, strongPassword)

	// The previous address can undo it: the address is restored, sessions
	// end and a password reset link follows.
	restore := mailTo(t, e.flushMail(), oldEmail, "/cancel-email-change?token=")
	anon.must(http.StatusOK, http.MethodPost, "/api/v1/auth/email-change/cancel", map[string]any{"token": tokenIn(t, restore)}, &res)
	if res.Status != "restored" || moved.call(http.MethodGet, "/api/v1/me", nil).status != http.StatusUnauthorized {
		t.Fatalf("restore: %+v", res)
	}
	reset := mailTo(t, e.flushMail(), oldEmail, "/reset-password?token=")
	newPassword := "N3w!Passw0rd#2026"
	anon.must(http.StatusNoContent, http.MethodPost, "/api/v1/auth/password/reset", map[string]any{"token": tokenIn(t, reset), "password": newPassword}, nil)
	back := e.login(oldEmail, newPassword)

	// An address that belongs to another account: same answer, and the
	// owner of that address is told; no confirmation link exists.
	taken := e.registerClient()
	back.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/email-change", map[string]any{"email": taken.user.Email, "password": newPassword}, nil)
	sent = e.flushMail()
	mailTo(t, sent, taken.user.Email, "another Staryield account")
	for _, m := range sent {
		if strings.Contains(m.Text, "/confirm-email?token=") && strings.EqualFold(m.To, taken.user.Email) {
			t.Fatal("confirmation link sent to an address in use")
		}
	}
	if e.auditCount("email_change.requested", c.user.ID) != 2 || e.auditCount("email_change.confirmed", c.user.ID) != 1 ||
		e.auditCount("email_change.restored", c.user.ID) != 1 {
		t.Fatal("e-mail change audit trail incomplete")
	}
}

func TestWithdrawnPersonalizationHidesSharedNotes(t *testing.T) {
	e := newEnv(t)
	client, ramone, cws, _, first := startedSession(t, e, 10, 250)
	var shared service.NoteView
	ramone.must(http.StatusCreated, http.MethodPost, notesPath(first.ConversationID), map[string]any{"body": "Prefers tarot", "shared": true}, &shared)
	endSession(t, client, first.ID)
	theo := e.createPsychic(10)
	second := sessionBetween(t, client, cws, theo)
	visible := func() int {
		var page service.Page[service.SharedNoteView]
		theo.must(http.StatusOK, http.MethodGet, notesPath(second.ConversationID)+"/shared", nil, &page)
		return len(page.Items)
	}
	if visible() != 1 {
		t.Fatal("shared note not visible by default (personalization defaults to on)")
	}
	var view service.ConsentsView
	client.must(http.StatusOK, http.MethodPut, "/api/v1/me/consents", map[string]any{"consents": map[string]any{"personalization": false}}, &view)
	if visible() != 0 {
		t.Fatal("withdrawn personalization must hide shared notes")
	}
	client.must(http.StatusOK, http.MethodPut, "/api/v1/me/consents", map[string]any{"consents": map[string]any{"personalization": true}, "source": "banner"}, &view)
	if visible() != 1 {
		t.Fatal("re-granted personalization shows shared notes again")
	}
	if r := client.call(http.MethodPut, "/api/v1/me/consents", map[string]any{"consents": map[string]any{"essential": false}}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("essential refused: %d", r.status)
	}

	// The policy is public; the history is append-only.
	var policy service.ConsentPolicyView
	e.client().must(http.StatusOK, http.MethodGet, "/api/v1/consent-policy", nil, &policy)
	if policy.Version == "" || len(policy.Categories) != 4 || !policy.Categories[0].Required {
		t.Fatalf("policy: %+v", policy)
	}
	var events int64
	e.app.DB.Model(&domain.ConsentEvent{}).Where("user_id = ?", client.user.ID).Count(&events)
	if events != 2 {
		t.Fatalf("consent history: %d", events)
	}
	if err := e.app.DB.Exec("UPDATE consent_events SET granted = NOT granted WHERE user_id = ?", client.user.ID).Error; err == nil {
		t.Fatal("consent history must be append-only")
	}
}

func readZip(t *testing.T, body []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("not a zip: %v", err)
	}
	out := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		out[f.Name], _ = io.ReadAll(rc)
		rc.Close()
	}
	return out
}

func TestDataExportFlow(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	client, psychic, cws, _, s := startedSession(t, e, 10, 250)
	cws.mustAck(realtime.FrameMessageSend, map[string]any{"session_id": s.ID, "body": "Hello from the client", "client_msg_id": "m-export-1"})
	var aboutClient service.NoteView
	psychic.must(http.StatusCreated, http.MethodPost, notesPath(s.ConversationID), map[string]any{"body": "Private advisor impression"}, &aboutClient)
	endSession(t, client, s.ID)

	if r := client.call(http.MethodPost, "/api/v1/me/exports", map[string]any{"password": "Wr0ng!pass1"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("export without the password: %d", r.status)
	}
	var job service.ExportView
	client.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/exports", map[string]any{"password": strongPassword}, &job)
	if job.Status != "queued" {
		t.Fatalf("job: %+v", job)
	}
	if r := client.call(http.MethodPost, "/api/v1/me/exports", map[string]any{"password": strongPassword}); r.status != http.StatusConflict || r.errCode(t) != "export_in_progress" {
		t.Fatalf("second export: %d %s", r.status, r.body)
	}
	path := "/api/v1/me/exports/" + job.ID.String()
	if r := client.call(http.MethodPost, path+"/download-token", map[string]any{"password": strongPassword}); r.status != http.StatusConflict {
		t.Fatalf("download before ready: %d", r.status)
	}
	// IDOR: another account sees nothing.
	x := e.registerClient()
	if r := x.call(http.MethodGet, path, nil); r.status != http.StatusNotFound {
		t.Fatalf("foreign export: %d", r.status)
	}
	if r := x.call(http.MethodPost, path+"/download-token", map[string]any{"password": strongPassword}); r.status != http.StatusNotFound {
		t.Fatalf("foreign download token: %d", r.status)
	}

	// The worker builds it.
	if err := e.app.Scheduler.RunJob(ctx, app.ExportJob); err != nil {
		t.Fatal(err)
	}
	client.must(http.StatusOK, http.MethodGet, path, nil, &job)
	if job.Status != "ready" || job.Bytes == 0 || len(job.SHA256) != 64 || job.ExpiresAt == nil ||
		job.ExpiresAt.Sub(e.clock.Now()) != 7*24*time.Hour {
		t.Fatalf("built job: %+v", job)
	}
	var list service.Page[service.ExportView]
	client.must(http.StatusOK, http.MethodGet, "/api/v1/me/exports", nil, &list)
	if len(list.Items) != 1 || list.Items[0].ID != job.ID {
		t.Fatalf("list: %+v", list.Items)
	}
	// In-app and e-mail notifications.
	deadline := time.Now().Add(5 * time.Second)
	for {
		var notes struct {
			Items []service.NotificationView `json:"items"`
		}
		client.must(http.StatusOK, http.MethodGet, "/api/v1/me/notifications", nil, &notes)
		if len(notes.Items) > 0 && notes.Items[0].Kind == service.NotifyExportReady {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no export.ready notification: %+v", notes.Items)
		}
		time.Sleep(20 * time.Millisecond)
	}
	for {
		if sent := e.flushMail(); len(sent) > 0 && strings.Contains(sent[len(sent)-1].Subject, "data export is ready") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no export e-mail")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Single-use download token, streamed as an uncacheable attachment.
	var link service.DownloadLink
	client.must(http.StatusOK, http.MethodPost, path+"/download-token", map[string]any{"password": strongPassword}, &link)
	r := client.raw(http.MethodGet, link.URL, nil, nil)
	if r.status != http.StatusOK || r.header.Get("Content-Type") != "application/zip" || !strings.HasPrefix(r.header.Get("Content-Disposition"), "attachment;") ||
		!strings.Contains(r.header.Get("Cache-Control"), "no-store") {
		t.Fatalf("download: %d %v", r.status, r.header)
	}
	sum := sha256.Sum256(r.body)
	if hex.EncodeToString(sum[:]) != job.SHA256 {
		t.Fatal("downloaded archive does not match its checksum")
	}
	if r := client.raw(http.MethodGet, link.URL, nil, nil); r.errCode(t) != "invalid_token" {
		t.Fatalf("download token reuse: %d %s", r.status, r.body)
	}
	if r := x.raw(http.MethodGet, link.URL, nil, nil); r.status == http.StatusOK {
		t.Fatal("another account downloaded the export")
	}
	files := readZip(t, r.body)
	for _, name := range []string{"README.txt", "manifest.json", "account.json", "devices.json", "consents.json", "chat/messages.json",
		"chat/conversations.json", "credits/ledger.json", "requests.json", "notifications.json"} {
		if _, ok := files[name]; !ok {
			t.Errorf("archive lacks %s", name)
		}
	}
	if !bytes.Contains(files["account.json"], []byte(client.user.Email)) || !bytes.Contains(files["chat/messages.json"], []byte("Hello from the client")) {
		t.Fatal("archive content incomplete")
	}
	for name, data := range files {
		if strings.HasPrefix(name, "advisor/") || bytes.Contains(data, []byte(aboutClient.ID.String())) ||
			bytes.Contains(data, []byte("Private advisor impression")) || bytes.Contains(data, []byte("password_hash")) {
			t.Fatalf("%s leaks a note about the client or a secret", name)
		}
	}

	// At most three exports a day.
	for i := 0; i < 2; i++ {
		client.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/exports", map[string]any{"password": strongPassword}, nil)
		if err := e.app.Scheduler.RunJob(ctx, app.ExportJob); err != nil {
			t.Fatal(err)
		}
	}
	if r := client.call(http.MethodPost, "/api/v1/me/exports", map[string]any{"password": strongPassword}); r.status != http.StatusTooManyRequests ||
		r.errCode(t) != "limit_reached" {
		t.Fatalf("daily limit: %d %s", r.status, r.body)
	}

	// After seven days archives are deleted.
	store := e.app.ExportStore.(*storage.Memory)
	if len(store.Keys()) != 3 {
		t.Fatalf("stored archives: %v", store.Keys())
	}
	e.clock.Advance(7*24*time.Hour + time.Minute)
	e.maintenance()
	client.refresh()
	client.must(http.StatusOK, http.MethodGet, path, nil, &job)
	if job.Status != "expired" || len(store.Keys()) != 0 {
		t.Fatalf("expired export: %+v keys=%v", job, store.Keys())
	}
	for _, action := range []string{"export.requested", "export.ready", "export.download_authorized", "export.downloaded"} {
		if e.auditCount(action, job.ID) == 0 {
			t.Errorf("no %s audit entry", action)
		}
	}

	// The advisor's own export carries the notes they wrote (decrypted).
	var pjob service.ExportView
	psychic.refresh()
	psychic.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/exports", map[string]any{"password": strongPassword}, &pjob)
	if err := e.app.Scheduler.RunJob(ctx, app.ExportJob); err != nil {
		t.Fatal(err)
	}
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/me/exports/"+pjob.ID.String()+"/download-token", map[string]any{"password": strongPassword}, &link)
	r = psychic.raw(http.MethodGet, link.URL, nil, nil)
	if r.status != http.StatusOK {
		t.Fatalf("advisor download: %d %s", r.status, r.body)
	}
	pfiles := readZip(t, r.body)
	if !bytes.Contains(pfiles["advisor/notes_written.json"], []byte("Private advisor impression")) || pfiles["advisor/earnings.json"] == nil ||
		pfiles["advisor/withdrawals.json"] == nil || !bytes.Contains(pfiles["account.json"], []byte(`"advisor_profile"`)) {
		t.Fatalf("advisor archive: %v", keysOf(pfiles))
	}
}

func keysOf(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestDeactivateThenReactivateBySigningIn(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	w := c.connect("")
	if r := c.call(http.MethodPost, "/api/v1/me/deactivate", map[string]any{"password": "Wr0ng!pass1"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("deactivate without the password: %d", r.status)
	}
	c.must(http.StatusNoContent, http.MethodPost, "/api/v1/me/deactivate", map[string]any{"password": strongPassword}, nil)
	if code := w.waitClosed(5 * time.Second); code != realtime.CloseUnauthorized {
		t.Fatalf("websocket close code %d", code)
	}
	if r := c.call(http.MethodGet, "/api/v1/me", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("deactivated account still signed in: %d", r.status)
	}
	mailTo(t, e.flushMail(), c.user.Email, "Sign in at any time to reactivate it.")

	// The right password offers a short-lived reactivation token.
	anon := e.client()
	if r := anon.call(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": c.user.Email, "password": "Wr0ng!pass1"}); r.errCode(t) != "invalid_credentials" {
		t.Fatalf("wrong password: %s", r.body)
	}
	r := anon.call(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": c.user.Email, "password": strongPassword})
	var offer struct {
		Error struct {
			Code    string `json:"code"`
			Details struct {
				Token string `json:"reactivation_token"`
			} `json:"details"`
		} `json:"error"`
	}
	r.decode(t, &offer)
	if r.status != http.StatusForbidden || offer.Error.Code != "account_deactivated" || offer.Error.Details.Token == "" {
		t.Fatalf("login on a deactivated account: %d %s", r.status, r.body)
	}
	var sess struct {
		User *service.UserView `json:"user"`
	}
	anon.must(http.StatusOK, http.MethodPost, "/api/v1/auth/reactivate", map[string]any{"token": offer.Error.Details.Token}, &sess)
	if sess.User == nil || sess.User.Status != "active" {
		t.Fatalf("reactivated: %+v", sess.User)
	}
	anon.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, nil)
	if r := e.client().call(http.MethodPost, "/api/v1/auth/reactivate", map[string]any{"token": offer.Error.Details.Token}); r.errCode(t) != "invalid_token" {
		t.Fatalf("reactivation token reuse: %s", r.body)
	}
	mailTo(t, e.flushMail(), c.user.Email, "reactivated")
	if e.auditCount("user.self_deactivated", c.user.ID) != 1 || e.auditCount("user.reactivated_self", c.user.ID) != 1 {
		t.Fatal("deactivation audit trail incomplete")
	}

	// Staff deactivation offers no way back by signing in.
	e.app.DB.Model(&domain.User{}).Where("id = ?", c.user.ID).Updates(map[string]any{"status": domain.UserDeactivated, "deactivated_by": domain.DeactivatedByAdmin})
	r = e.client().call(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": c.user.Email, "password": strongPassword})
	if r.errCode(t) != "account_disabled" || strings.Contains(string(r.body), "reactivation_token") {
		t.Fatalf("admin-deactivated login: %d %s", r.status, r.body)
	}
}

func TestCloseAccountThenKeepIt(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	if r := c.call(http.MethodPost, "/api/v1/me/close", map[string]any{"password": strongPassword, "confirmation": "delete"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("closure without the confirmation word: %d", r.status)
	}
	var v service.ClosureView
	c.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/close", map[string]any{"password": strongPassword, "confirmation": "DELETE",
		"reason": "Taking a break"}, &v)
	if v.Status != "closure_scheduled" || !v.DeletionScheduledAt.Equal(e.clock.Now().Add(30*24*time.Hour)) {
		t.Fatalf("closure: %+v", v)
	}
	if r := c.call(http.MethodGet, "/api/v1/me", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("closing account still signed in: %d", r.status)
	}
	r := e.client().call(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": c.user.Email, "password": strongPassword})
	if r.status != http.StatusForbidden || r.errCode(t) != "account_closure_pending" || !strings.Contains(string(r.body), "deletion_scheduled_at") {
		t.Fatalf("login during the grace period: %d %s", r.status, r.body)
	}
	keep := mailTo(t, e.flushMail(), c.user.Email, "/keep-account?token=")
	e.client().must(http.StatusOK, http.MethodPost, "/api/v1/auth/closure/cancel", map[string]any{"token": tokenIn(t, keep)}, nil)
	e.login(c.user.Email, strongPassword)
	var u domain.User
	e.app.DB.First(&u, "id = ?", c.user.ID)
	if u.Status != domain.UserActive || u.ClosureScheduledAt != nil || u.DeactivatedBy != "" {
		t.Fatalf("kept account: %+v", u)
	}
	// The erasure job leaves a kept account alone.
	e.clock.Advance(31 * 24 * time.Hour)
	if err := e.app.Scheduler.RunJob(context.Background(), app.ClosureJob); err != nil {
		t.Fatal(err)
	}
	e.app.DB.First(&u, "id = ?", c.user.ID)
	if u.Status != domain.UserActive {
		t.Fatalf("kept account erased: %+v", u)
	}
}

func TestClosedAccountIsErasedAfterGracePeriod(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	c := e.registerClient()
	c.must(http.StatusOK, http.MethodPut, "/api/v1/me/consents", map[string]any{"consents": map[string]any{"analytics": true}}, nil)
	email := c.user.Email
	c.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/close", map[string]any{"password": strongPassword, "confirmation": "DELETE"}, nil)
	e.flushMail()

	e.clock.Advance(29 * 24 * time.Hour)
	if err := e.app.Scheduler.RunJob(ctx, app.ClosureJob); err != nil {
		t.Fatal(err)
	}
	var u domain.User
	e.app.DB.First(&u, "id = ?", c.user.ID)
	if u.Status != domain.UserDeactivated {
		t.Fatalf("erased before the grace period ended: %+v", u)
	}
	e.clock.Advance(24*time.Hour + time.Minute)
	if err := e.app.Scheduler.RunJob(ctx, app.ClosureJob); err != nil {
		t.Fatal(err)
	}
	e.app.DB.First(&u, "id = ?", c.user.ID)
	if u.Status != domain.UserDeleted || u.Email != "deleted+"+u.ID.String()+"@deleted.invalid" || u.DisplayName != "Deleted user" {
		t.Fatalf("not erased: %+v", u)
	}
	for _, table := range []string{"user_sessions", "user_consents", "user_action_tokens", "refresh_tokens"} {
		var n int64
		e.app.DB.Table(table).Where("user_id = ?", c.user.ID).Count(&n)
		if n != 0 {
			t.Errorf("%s keeps %d rows of the erased account", table, n)
		}
	}
	if r := e.client().call(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": email, "password": strongPassword}); r.errCode(t) != "invalid_credentials" {
		t.Fatalf("erased account signs in: %s", r.body)
	}
	mailTo(t, e.flushMail(), email, "personal data erased")
	if e.auditCount("user.erased", c.user.ID) != 1 || e.auditCount("user.closure_scheduled", c.user.ID) != 1 {
		t.Fatal("closure audit trail incomplete")
	}
}

func TestAdvisorClosureIsBlockedByOpenBalances(t *testing.T) {
	e := newEnv(t)
	p := e.createPsychic(10)
	now := e.clock.Now()
	if err := e.app.DB.Create(&domain.Earning{ID: uuid.New(), PsychicID: p.user.ID, SourceType: domain.EarningChatSession, SourceID: uuid.New(),
		EntryKind: domain.EarningKindEarning, GrossCredits: 4000, CommissionCredits: 1000, NetCredits: 3000, SharePercent: 75, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	closeReq := map[string]any{"password": strongPassword, "confirmation": "DELETE"}
	reasons := func() []string {
		t.Helper()
		r := p.call(http.MethodPost, "/api/v1/me/close", closeReq)
		var body struct {
			Error struct {
				Code    string `json:"code"`
				Details struct {
					Reasons []string `json:"reasons"`
				} `json:"details"`
			} `json:"error"`
		}
		r.decode(t, &body)
		if r.status != http.StatusConflict || body.Error.Code != "closure_blocked" {
			t.Fatalf("advisor closure: %d %s", r.status, r.body)
		}
		return body.Error.Details.Reasons
	}
	if got := reasons(); strings.Join(got, ",") != "earnings_balance" {
		t.Fatalf("reasons: %v", got)
	}
	// Once the balance is requested as a payout, the payout blocks instead.
	p.must(http.StatusCreated, http.MethodPost, "/api/v1/psychic/withdrawals", map[string]any{"amount_credits": 3000, "method": "paypal",
		"destination": "advisor@example.com", "idempotency_key": uniq("wd-key-")}, nil)
	if got := reasons(); strings.Join(got, ",") != "pending_withdrawals" {
		t.Fatalf("reasons: %v", got)
	}
	p.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, nil)
	// An advisor with nothing owed can leave.
	free := e.createPsychic(10)
	free.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/close", closeReq, nil)
}

// A staff decision replaces the owner's own pause or closure request: a
// self-deactivated account deactivated by staff cannot be reactivated by
// signing in, and re-activating a closing account cancels its erasure.
func TestStaffDecisionOverridesSelfService(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	paused := e.registerClient()
	paused.must(http.StatusNoContent, http.MethodPost, "/api/v1/me/deactivate", map[string]any{"password": strongPassword}, nil)
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/users/"+paused.user.ID.String(), map[string]any{"status": "deactivated",
		"status_reason": "Under review"}, nil)
	r := e.client().call(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": paused.user.Email, "password": strongPassword})
	if r.errCode(t) != "account_disabled" || strings.Contains(string(r.body), "reactivation_token") {
		t.Fatalf("staff-deactivated account offered reactivation: %d %s", r.status, r.body)
	}

	closing := e.registerClient()
	closing.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/close", map[string]any{"password": strongPassword, "confirmation": "DELETE"}, nil)
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/users/"+closing.user.ID.String(), map[string]any{"status": "active"}, nil)
	e.clock.Advance(31 * 24 * time.Hour)
	if err := e.app.Scheduler.RunJob(context.Background(), app.ClosureJob); err != nil {
		t.Fatal(err)
	}
	var u domain.User
	e.app.DB.First(&u, "id = ?", closing.user.ID)
	if u.Status != domain.UserActive || u.ClosureScheduledAt != nil || u.DeactivatedBy != "" {
		t.Fatalf("re-activated account: %+v", u)
	}
}
