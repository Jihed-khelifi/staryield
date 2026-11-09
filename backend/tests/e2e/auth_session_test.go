package e2e

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
)

// Revisions 3.3 (docs/platform-extensions.md §22.1): sessions stay signed in.

func (c *client) cookie(name, path string) string {
	u, _ := url.Parse(c.e.srv.URL + path)
	for _, ck := range c.http.Jar.Cookies(u) {
		if ck.Name == name {
			return ck.Value
		}
	}
	return ""
}

func (c *client) setCookie(name, value, path string) {
	u, _ := url.Parse(c.e.srv.URL + path)
	c.http.Jar.SetCookies(u, []*http.Cookie{{Name: name, Value: value, Path: path}})
}

func csrfTokenOf(t *testing.T, r response) string {
	t.Helper()
	var out struct {
		Token string `json:"csrf_token"`
	}
	r.decode(t, &out)
	if out.Token == "" || out.Token != r.header.Get("X-CSRF-Token") {
		t.Fatalf("csrf body/header mismatch: %q vs %q", out.Token, r.header.Get("X-CSRF-Token"))
	}
	return out.Token
}

func TestCSRFTokenIsStableUntilSignIn(t *testing.T) {
	e := newEnv(t)
	c := e.client()
	first := csrfTokenOf(t, c.raw(http.MethodGet, "/api/v1/auth/csrf", nil, nil))
	second := csrfTokenOf(t, c.raw(http.MethodGet, "/api/v1/auth/csrf", nil, nil))
	if first != second {
		t.Fatal("GET /auth/csrf must return the same token while the cookie is valid")
	}
	// A forged cookie is replaced, not echoed.
	forged := e.client()
	forged.setCookie("sy_csrf", "x.y", "/")
	if got := csrfTokenOf(t, forged.raw(http.MethodGet, "/api/v1/auth/csrf", nil, nil)); got == "x.y" || got == "" {
		t.Fatalf("invalid cookie must be replaced: %q", got)
	}

	// Sign-in rotates it; refresh keeps it.
	reg := e.registerClient()
	afterLogin := reg.csrf
	if afterLogin == "" {
		t.Fatal("login returned no csrf token")
	}
	var out struct {
		Token string `json:"csrf_token"`
	}
	r := reg.must(http.StatusOK, http.MethodPost, "/api/v1/auth/refresh", nil, &out)
	if out.Token != afterLogin || r.header.Get("X-CSRF-Token") != afterLogin || reg.cookie("sy_csrf", "/") != afterLogin {
		t.Fatalf("refresh must not rotate the csrf token: %q vs %q", out.Token, afterLogin)
	}
	login := e.login(reg.user.Email, reg.pw)
	if login.csrf == "" || login.csrf == afterLogin {
		t.Fatal("login must mint a new csrf token")
	}
	// The same browser logging in again rotates too.
	before := login.csrf
	login.must(http.StatusOK, http.MethodPost, "/api/v1/auth/login", map[string]any{"email": reg.user.Email, "password": reg.pw}, nil)
	if login.csrf == before {
		t.Fatal("repeated login must rotate the csrf token")
	}
}

func TestConcurrentRefreshWithinGraceBothSucceed(t *testing.T) {
	e := newEnv(t)
	a := e.registerClient()
	old := a.cookie("sy_rt", "/api/v1/auth")
	// Second tab: same cookie, refreshed at the same time.
	b := e.client()
	b.setCookie("sy_rt", old, "/api/v1/auth")
	b.csrf = a.csrf
	b.setCookie("sy_csrf", a.csrf, "/")

	a.must(http.StatusOK, http.MethodPost, "/api/v1/auth/refresh", nil, nil)
	e.clock.Advance(90 * time.Second)
	b.must(http.StatusOK, http.MethodPost, "/api/v1/auth/refresh", nil, nil)
	if a.cookie("sy_rt", "/api/v1/auth") == b.cookie("sy_rt", "/api/v1/auth") {
		t.Fatal("each refresh must issue its own token")
	}
	// Both successors keep working.
	a.must(http.StatusOK, http.MethodPost, "/api/v1/auth/refresh", nil, nil)
	b.must(http.StatusOK, http.MethodPost, "/api/v1/auth/refresh", nil, nil)
	a.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, nil)
	b.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, nil)
}

func TestRefreshLostResponseRecovery(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	old := c.cookie("sy_rt", "/api/v1/auth")
	c.must(http.StatusOK, http.MethodPost, "/api/v1/auth/refresh", nil, nil) // the response is "lost":
	lost := c.cookie("sy_rt", "/api/v1/auth")
	c.setCookie("sy_rt", old, "/api/v1/auth") // the browser still holds the old cookie

	e.clock.Advance(5 * time.Minute)
	c.must(http.StatusOK, http.MethodPost, "/api/v1/auth/refresh", nil, nil)
	recovered := c.cookie("sy_rt", "/api/v1/auth")
	if recovered == old || recovered == lost {
		t.Fatal("recovery must issue a new token")
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, nil)

	// The superseded successor is dead and presenting it is reuse.
	other := e.client()
	other.setCookie("sy_rt", lost, "/api/v1/auth")
	if r := other.call(http.MethodPost, "/api/v1/auth/refresh", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("superseded token: %d %s", r.status, r.body)
	}
	if r := c.call(http.MethodPost, "/api/v1/auth/refresh", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("reuse must revoke the family: %d", r.status)
	}
}

func TestRefreshReuseAfterSuccessorUsed(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	old := c.cookie("sy_rt", "/api/v1/auth")
	c.must(http.StatusOK, http.MethodPost, "/api/v1/auth/refresh", nil, nil) // successor presented below
	c.must(http.StatusOK, http.MethodPost, "/api/v1/auth/refresh", nil, nil) // the successor was used
	other := e.login(c.user.Email, c.pw)                                     // another device of the same user
	e.clock.Advance(5 * time.Minute)

	thief := e.client()
	thief.setCookie("sy_rt", old, "/api/v1/auth")
	if r := thief.call(http.MethodPost, "/api/v1/auth/refresh", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("reuse: %d %s", r.status, r.body)
	}
	if r := c.call(http.MethodPost, "/api/v1/auth/refresh", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("family must be revoked: %d", r.status)
	}
	// Access tokens of the user are revoked as well, including other devices.
	if r := other.call(http.MethodGet, "/api/v1/me", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("other session cookies must be revoked: %d", r.status)
	}
}

// A token rotated moments before a sign-out must not reopen the session,
// neither inside the grace nor inside the recovery window.
func TestSignedOutFamilyIsNeverReopened(t *testing.T) {
	for _, tc := range []struct {
		name    string
		path    string
		advance time.Duration
	}{
		{"logout inside the grace", "/api/v1/auth/logout", 20 * time.Second},
		{"logout-all inside the grace", "/api/v1/auth/logout-all", 20 * time.Second},
		{"logout inside the recovery window", "/api/v1/auth/logout", 5 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			c := e.registerClient()
			old := c.cookie("sy_rt", "/api/v1/auth")
			c.must(http.StatusOK, http.MethodPost, "/api/v1/auth/refresh", nil, nil)
			e.clock.Advance(tc.advance)
			if r := c.call(http.MethodPost, tc.path, nil); r.status >= 300 {
				t.Fatalf("%s: %d %s", tc.path, r.status, r.body)
			}
			thief := e.client()
			thief.setCookie("sy_rt", old, "/api/v1/auth")
			if r := thief.call(http.MethodPost, "/api/v1/auth/refresh", nil); r.status != http.StatusUnauthorized {
				t.Fatalf("a signed-out family must stay closed: %d %s", r.status, r.body)
			}
			if got := thief.cookie("sy_at", "/"); got != "" {
				t.Fatal("no access cookie may be issued")
			}
		})
	}
}

func TestRefreshIsNotLimitedByTheAuthBucket(t *testing.T) {
	e := newEnv(t, func(cfg *config.Config) { cfg.RateLimit.Scale = 1 })
	c := e.registerClient()
	attacker := e.client()
	limited := false
	for i := 0; i < 30; i++ {
		r := attacker.call(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "nobody@example.com", "password": "wrong-password-1"})
		if r.status == http.StatusTooManyRequests {
			limited = true
			break
		}
	}
	if !limited {
		t.Fatal("the auth bucket should be exhausted")
	}
	for i := 0; i < 5; i++ {
		c.must(http.StatusOK, http.MethodPost, "/api/v1/auth/refresh", nil, nil)
	}
}

func TestRefreshRuleLimitsPerCookie(t *testing.T) {
	e := newEnv(t, func(cfg *config.Config) {
		cfg.RateLimit.Scale = 1
		cfg.RateLimit.Refresh = config.Rule{Rate: 1, Period: time.Hour, Burst: 2}
	})
	a, b := e.registerClient(), e.registerClient()
	token := a.cookie("sy_rt", "/api/v1/auth")
	// Presenting the same cookie again and again (a retry loop) is limited.
	for i := 0; i < 2; i++ {
		a.setCookie("sy_rt", token, "/api/v1/auth")
		a.must(http.StatusOK, http.MethodPost, "/api/v1/auth/refresh", nil, nil)
	}
	a.setCookie("sy_rt", token, "/api/v1/auth")
	if r := a.call(http.MethodPost, "/api/v1/auth/refresh", nil); r.status != http.StatusTooManyRequests {
		t.Fatalf("third refresh with one cookie: %d", r.status)
	}
	// Another session from the same IP has its own bucket.
	b.must(http.StatusOK, http.MethodPost, "/api/v1/auth/refresh", nil, nil)
}

func TestStatelessAstrologyPostsSkipCSRF(t *testing.T) {
	e := newEnv(t)
	c := e.client() // no CSRF cookie, no header
	post := func(path string, body string) response {
		return c.raw(http.MethodPost, path, strings.NewReader(body), map[string]string{"Content-Type": "application/json"})
	}
	for path, body := range map[string]string{
		"/api/v1/astrology/natal":      `{"birth_date":"1990-05-01","birth_time":"14:30","place_id":"gn:2988507","tz":"Europe/Paris"}`,
		"/api/v1/astrology/numerology": `{"birth_date":"1987-11-29","as_of":"2026-03-01"}`,
		"/api/v1/astrology/sun-sign":   `{"birth_date":"1990-05-01"}`,
		"/api/v1/astrology/chart":      `{"birth_date":"1990-05-01","birth_time":"14:30","place_id":"gn:2988507"}`,
	} {
		if r := post(path, body); r.status != http.StatusOK {
			t.Errorf("%s without CSRF: %d %s", path, r.status, r.body)
		}
	}
	// Stateful endpoints still need it.
	reg := e.registerClient()
	r := reg.raw(http.MethodPatch, "/api/v1/me/profile", strings.NewReader(`{"nickname":"Moon"}`), map[string]string{"Content-Type": "application/json"})
	if r.status != http.StatusForbidden || r.errCode(t) != "csrf_failed" {
		t.Fatalf("PATCH /me/profile without the header: %d %s", r.status, r.body)
	}
}
