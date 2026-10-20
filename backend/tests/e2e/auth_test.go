package e2e

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

func TestRegisterLoginMeLogout(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	if c.user.Role != "client" || c.user.Wallet == nil || c.user.Wallet.FreeMinutes != 3 {
		t.Fatalf("unexpected user: %+v", c.user)
	}
	if c.user.Profile.BirthDate != "1990-05-17" {
		t.Fatalf("birth date not stored: %q", c.user.Profile.BirthDate)
	}

	var me service.UserView
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, &me)
	if me.Email != c.user.Email {
		t.Fatalf("me mismatch")
	}

	// Logout clears the session; /me is then unauthenticated.
	c.must(http.StatusNoContent, http.MethodPost, "/api/v1/auth/logout", nil, nil)
	if r := c.call(http.MethodGet, "/api/v1/me", nil); r.status != http.StatusUnauthorized || r.errCode(t) != "unauthenticated" {
		t.Fatalf("after logout: %d %s", r.status, r.body)
	}
	// Sign back in.
	e.login(c.user.Email, strongPassword)
}

func TestRegisterValidation(t *testing.T) {
	e := newEnv(t)
	c := e.client()
	r := c.call(http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email": "not-an-email", "password": "weak", "accept_terms": false, "birth_date": time.Now().AddDate(-10, 0, 0).Format("2006-01-02"),
	})
	if r.status != http.StatusUnprocessableEntity || r.errCode(t) != "validation_failed" {
		t.Fatalf("want validation error, got %d %s", r.status, r.body)
	}
	for _, field := range []string{"email", "password", "accept_terms", "birth_date"} {
		if !strings.Contains(string(r.body), `"`+field+`"`) {
			t.Errorf("missing field error for %s: %s", field, r.body)
		}
	}
	// Unknown fields are rejected (mass-assignment protection).
	r = c.call(http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email": uniq("x") + "@example.com", "password": strongPassword, "accept_terms": true, "role": "admin",
	})
	if r.status != http.StatusUnprocessableEntity || !strings.Contains(string(r.body), "role") {
		t.Fatalf("role field must be rejected: %d %s", r.status, r.body)
	}
	// Duplicate e-mail.
	existing := e.registerClient()
	r = c.call(http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email": strings.ToUpper(existing.user.Email), "password": strongPassword, "accept_terms": true,
	})
	if r.status != http.StatusConflict || r.errCode(t) != "email_taken" {
		t.Fatalf("duplicate: %d %s", r.status, r.body)
	}
}

func TestCSRFAndOriginProtection(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()

	// Missing CSRF header on an unsafe request.
	r := c.raw(http.MethodPatch, "/api/v1/me/profile", strings.NewReader(`{"nickname":"x"}`), map[string]string{"Content-Type": "application/json"})
	if r.status != http.StatusForbidden || r.errCode(t) != "csrf_failed" {
		t.Fatalf("missing csrf: %d %s", r.status, r.body)
	}
	// Forged token.
	r = c.raw(http.MethodPatch, "/api/v1/me/profile", strings.NewReader(`{"nickname":"x"}`), map[string]string{
		"Content-Type": "application/json", "X-CSRF-Token": "forged.value"})
	if r.status != http.StatusForbidden {
		t.Fatalf("forged csrf: %d", r.status)
	}
	// Foreign origin is rejected even with a valid token.
	c.ensureCSRF()
	c.origin = "https://evil.example"
	r = c.call(http.MethodPatch, "/api/v1/me/profile", map[string]any{"nickname": "x"})
	if r.status != http.StatusForbidden {
		t.Fatalf("foreign origin: %d", r.status)
	}
	c.origin = origin
	c.must(http.StatusOK, http.MethodPatch, "/api/v1/me/profile", map[string]any{"nickname": "Moon"}, nil)
}

func TestRefreshRotationAndReuseDetection(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()

	refreshCookie := func() *http.Cookie {
		u, _ := url.Parse(e.srv.URL + "/api/v1/auth/refresh")
		for _, ck := range c.http.Jar.Cookies(u) {
			if ck.Name == "sy_rt" {
				return ck
			}
		}
		t.Fatal("no refresh cookie")
		return nil
	}
	old := refreshCookie()
	c.must(http.StatusOK, http.MethodPost, "/api/v1/auth/refresh", nil, nil)
	rotated := refreshCookie()
	if rotated.Value == old.Value {
		t.Fatal("refresh token was not rotated")
	}

	// Replaying the old token inside the grace window is a benign race.
	thief := e.client()
	u, _ := url.Parse(e.srv.URL + "/api/v1/auth/refresh")
	thief.http.Jar.SetCookies(u, []*http.Cookie{{Name: "sy_rt", Value: old.Value, Path: "/api/v1/auth"}})
	if r := thief.call(http.MethodPost, "/api/v1/auth/refresh", nil); r.status != http.StatusConflict || r.errCode(t) != "refresh_conflict" {
		t.Fatalf("replay within grace: %d %s", r.status, r.body)
	}
	// After the grace window, replay means theft: the whole family dies.
	e.clock.Advance(time.Minute)
	if r := thief.call(http.MethodPost, "/api/v1/auth/refresh", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("replay after grace: %d %s", r.status, r.body)
	}
	if r := c.call(http.MethodPost, "/api/v1/auth/refresh", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("family should be revoked, got %d", r.status)
	}
	// Existing access tokens are invalidated too.
	if r := c.call(http.MethodGet, "/api/v1/me", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("access token survived reuse detection: %d", r.status)
	}
}

func TestLoginLockoutAndGenericErrors(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.Auth.LoginMaxFailures = 3 })
	victim := e.registerClient()
	attacker := e.client()

	unknown := attacker.call(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": uniq("nobody") + "@example.com", "password": "Wr0ng!pass"})
	wrong := attacker.call(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": victim.user.Email, "password": "Wr0ng!pass"})
	if unknown.status != http.StatusUnauthorized || wrong.status != http.StatusUnauthorized || unknown.errCode(t) != wrong.errCode(t) {
		t.Fatalf("errors must not reveal account existence: %s vs %s", unknown.body, wrong.body)
	}
	for i := 0; i < 2; i++ {
		attacker.call(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": victim.user.Email, "password": "Wr0ng!pass"})
	}
	r := attacker.call(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": victim.user.Email, "password": strongPassword})
	if r.status != http.StatusTooManyRequests || r.errCode(t) != "account_locked" || r.header.Get("Retry-After") == "" {
		t.Fatalf("expected lockout even with the right password: %d %s", r.status, r.body)
	}
}

func TestPasswordResetFlow(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	anon := e.client()

	// Unknown e-mail: same response, no mail.
	anon.must(http.StatusAccepted, http.MethodPost, "/api/v1/auth/password/forgot", map[string]any{"email": uniq("ghost") + "@example.com"}, nil)
	if len(e.mail.Sent()) != 0 {
		t.Fatal("mail sent for unknown account")
	}
	anon.must(http.StatusAccepted, http.MethodPost, "/api/v1/auth/password/forgot", map[string]any{"email": c.user.Email, "locale": "fr"}, nil)
	sent := e.mail.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "/fr/reset-password?token=") {
		t.Fatalf("reset mail: %+v", sent)
	}
	token := strings.TrimSpace(sent[0].Text[strings.Index(sent[0].Text, "token=")+6:])
	token = strings.Fields(token)[0]

	// Weak password rejected; token still usable afterwards.
	if r := anon.call(http.MethodPost, "/api/v1/auth/password/reset", map[string]any{"token": token, "password": "short"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("weak reset password: %d", r.status)
	}
	newPassword := "N3w!Passw0rd#2026"
	anon.must(http.StatusNoContent, http.MethodPost, "/api/v1/auth/password/reset", map[string]any{"token": token, "password": newPassword}, nil)
	// Single use.
	if r := anon.call(http.MethodPost, "/api/v1/auth/password/reset", map[string]any{"token": token, "password": newPassword}); r.errCode(t) != "invalid_token" {
		t.Fatalf("token reuse: %s", r.body)
	}
	// Old sessions are revoked; the new password works.
	if r := c.call(http.MethodGet, "/api/v1/me", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("old session still valid: %d", r.status)
	}
	e.login(c.user.Email, newPassword)
}

func TestChangePasswordKeepsCurrentSession(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	other := e.login(c.user.Email, strongPassword) // second device

	r := c.call(http.MethodPost, "/api/v1/auth/password/change", map[string]any{"current_password": "Wr0ng!pass1", "new_password": "An0ther!Pass#1"})
	if r.status != http.StatusUnprocessableEntity {
		t.Fatalf("wrong current password: %d", r.status)
	}
	c.must(http.StatusOK, http.MethodPost, "/api/v1/auth/password/change", map[string]any{"current_password": strongPassword, "new_password": "An0ther!Pass#1"}, nil)
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, nil)
	if r := other.call(http.MethodGet, "/api/v1/me", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("other device should be signed out: %d", r.status)
	}
}

func TestRBAC(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	p := e.createPsychic(10)
	for _, tc := range []struct {
		who    *client
		method string
		path   string
		want   int
	}{
		{c, http.MethodGet, "/api/v1/admin/users", http.StatusForbidden},
		{p, http.MethodGet, "/api/v1/admin/reports/overview", http.StatusForbidden},
		{c, http.MethodPatch, "/api/v1/psychic/availability", http.StatusForbidden},
		{p, http.MethodPost, "/api/v1/chat/sessions", http.StatusForbidden},
		{p, http.MethodPost, "/api/v1/credits/purchases", http.StatusForbidden},
		{e.client(), http.MethodGet, "/api/v1/wallet", http.StatusUnauthorized},
	} {
		var payload any
		if tc.method != http.MethodGet {
			payload = map[string]any{}
		}
		if r := tc.who.call(tc.method, tc.path, payload); r.status != tc.want {
			t.Errorf("%s %s: want %d got %d (%s)", tc.method, tc.path, tc.want, r.status, r.body)
		}
	}
}

func TestProfileUpdateAndAvatar(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	var me service.UserView
	c.must(http.StatusOK, http.MethodPatch, "/api/v1/me/profile", map[string]any{
		"nickname": "  Luna‮  ", "birth_time": "07:45", "birth_place_id": "paris-fr", "mood": "Calm", "theme": "Light Cream",
		"notify_events": true,
	}, &me)
	if me.Profile.Nickname != "Luna" || me.Profile.BirthPlace == "" || me.Profile.BirthTimezone != "Europe/Paris" || me.Profile.Mood != "Calm" {
		t.Fatalf("profile not updated/sanitised: %+v", me.Profile)
	}
	r := c.call(http.MethodPatch, "/api/v1/me/profile", map[string]any{"mood": "Doomed", "birth_time": "25:99", "birth_place_id": "atlantis"})
	if r.status != http.StatusUnprocessableEntity {
		t.Fatalf("invalid profile accepted: %s", r.body)
	}

	// The full chart now has houses (time + place known).
	var chart service.ChartView
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/chart", nil, &chart)
	if !chart.HasLocation || chart.RisingSign == "" || len(chart.Positions) < 10 || chart.SunSign != "Taurus" {
		t.Fatalf("chart: %+v", chart.Chart)
	}
	var horoscope struct {
		Sign string `json:"sign"`
		Text string `json:"text"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/horoscope", nil, &horoscope)
	if horoscope.Sign != "Taurus" || horoscope.Text == "" {
		t.Fatalf("horoscope: %+v", horoscope)
	}

	// Avatar: non-image content is rejected by sniffing.
	body, ct := multipartFile(t, "avatar", "evil.png", []byte(strings.Repeat("<script>alert(1)</script>", 10)))
	c.ensureCSRF()
	r = c.raw(http.MethodPost, "/api/v1/me/avatar", body, map[string]string{"Content-Type": ct, "X-CSRF-Token": c.csrf})
	if r.status != http.StatusUnsupportedMediaType {
		t.Fatalf("non-image upload: %d %s", r.status, r.body)
	}
	body, ct = multipartFile(t, "avatar", "me.png", pngBytes(t, 640, 480))
	r = c.raw(http.MethodPost, "/api/v1/me/avatar", body, map[string]string{"Content-Type": ct, "X-CSRF-Token": c.csrf})
	if r.status != http.StatusOK {
		t.Fatalf("avatar upload: %d %s", r.status, r.body)
	}
	r.decode(t, &me)
	if !strings.HasPrefix(me.Profile.AvatarURL, "https://assets.test/avatars/") || !strings.HasSuffix(me.Profile.AvatarURL, ".jpg") {
		t.Fatalf("avatar url: %s", me.Profile.AvatarURL)
	}
	key := strings.TrimPrefix(me.Profile.AvatarURL, "https://assets.test/")
	if data, ok := e.store.Get(key); !ok || len(data) == 0 {
		t.Fatal("avatar not stored")
	}
	// Oversized uploads are refused.
	body, ct = multipartFile(t, "avatar", "big.png", make([]byte, 2<<20))
	r = c.raw(http.MethodPost, "/api/v1/me/avatar", body, map[string]string{"Content-Type": ct, "X-CSRF-Token": c.csrf})
	if r.status != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized upload: %d %s", r.status, r.body)
	}
}

func TestSecurityHeadersAndErrors(t *testing.T) {
	e := newEnv(t)
	c := e.client()
	r := c.raw(http.MethodGet, "/api/v1/does-not-exist", nil, map[string]string{"X-Request-ID": "trace-abc-123456"})
	if r.status != http.StatusNotFound || r.errCode(t) != "not_found" {
		t.Fatalf("404: %d %s", r.status, r.body)
	}
	if r.header.Get("X-Request-ID") != "trace-abc-123456" || !strings.Contains(string(r.body), "trace-abc-123456") {
		t.Fatalf("request id not propagated: %v %s", r.header, r.body)
	}
	for _, h := range []string{"X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy", "Referrer-Policy"} {
		if r.header.Get(h) == "" {
			t.Errorf("missing %s", h)
		}
	}
	// Wrong content type.
	c.ensureCSRF()
	r = c.raw(http.MethodPost, "/api/v1/auth/login", strings.NewReader("email=a"), map[string]string{"Content-Type": "application/x-www-form-urlencoded", "X-CSRF-Token": c.csrf})
	if r.status != http.StatusUnsupportedMediaType {
		t.Fatalf("form body: %d", r.status)
	}
	// Oversized JSON body.
	r = c.raw(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"`+strings.Repeat("a", 70<<10)+`"}`), map[string]string{"Content-Type": "application/json", "X-CSRF-Token": c.csrf})
	if r.status != http.StatusRequestEntityTooLarge {
		t.Fatalf("large body: %d %s", r.status, r.body)
	}
	// CORS preflight from an allowed origin.
	r = c.raw(http.MethodOptions, "/api/v1/auth/login", nil, map[string]string{"Access-Control-Request-Method": "POST"})
	if r.header.Get("Access-Control-Allow-Origin") != origin || r.header.Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("cors: %v", r.header)
	}
	c.origin = "https://evil.example"
	r = c.raw(http.MethodOptions, "/api/v1/auth/login", nil, map[string]string{"Access-Control-Request-Method": "POST"})
	if r.header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("cors allowed foreign origin")
	}
}

func TestRateLimitedAuthEndpoints(t *testing.T) {
	e := newEnv(t, func(c *config.Config) {
		c.RateLimit.Scale = 1
		c.RateLimit.Auth = config.Rule{Rate: 3, Period: time.Minute, Burst: 3}
	})
	c := e.client()
	var last response
	for i := 0; i < 5; i++ {
		last = c.call(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "a@example.com", "password": "x"})
	}
	if last.status != http.StatusTooManyRequests || last.errCode(t) != "rate_limited" || last.header.Get("Retry-After") == "" {
		t.Fatalf("expected throttling: %d %s", last.status, last.body)
	}
}
