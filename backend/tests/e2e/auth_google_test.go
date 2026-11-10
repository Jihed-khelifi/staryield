package e2e

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	"github.com/Jihed-khelifi/staryield/backend/internal/oidc"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
)

// Sign in with Google (revisions 4.0, §23.1), end to end. The provider is a
// fake injected through app.Overrides: it checks that the nonce and the PKCE
// verifier the API sends on exchange belong to the attempt it was asked to
// start; real token validation is covered by the oidc package tests.

type fakeCode struct {
	state string
	id    *oidc.Identity
	err   error
}

type fakeGoogle struct {
	mu       sync.Mutex
	attempts map[string]oidc.AuthParams
	codes    map[string]fakeCode
	n        int
	down     bool
	// mismatches counts exchanges whose nonce/verifier did not match.
	mismatches int
	exchanges  int
}

func newFakeGoogle() *fakeGoogle {
	return &fakeGoogle{attempts: map[string]oidc.AuthParams{}, codes: map[string]fakeCode{}}
}

func (f *fakeGoogle) AuthorizationURL(_ context.Context, p oidc.AuthParams) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return "", oidc.ErrUnavailable
	}
	f.attempts[p.State] = p
	q := url.Values{"response_type": {"code"}, "client_id": {"test-client"}, "scope": {"openid email"}, "state": {p.State},
		"nonce": {p.Nonce}, "code_challenge": {p.CodeChallenge}, "code_challenge_method": {"S256"}, "prompt": {"select_account"}, "hl": {p.Locale}}
	return "https://accounts.example.test/o/oauth2/v2/auth?" + q.Encode(), nil
}

func (f *fakeGoogle) Exchange(_ context.Context, p oidc.ExchangeParams) (*oidc.Identity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.exchanges++
	c, ok := f.codes[p.Code]
	if !ok {
		return nil, oidc.ErrInvalidGrant
	}
	delete(f.codes, p.Code) // codes are single use
	a := f.attempts[c.state]
	if a.Nonce != p.Nonce || a.CodeChallenge != oidc.Challenge(p.CodeVerifier) {
		f.mismatches++
		return nil, oidc.ErrInvalidGrant
	}
	return c.id, c.err
}

// issue hands out an authorization code for an attempt.
func (f *fakeGoogle) issue(state string, id *oidc.Identity, err error) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	code := "code-" + uuid.NewString()
	f.codes[code] = fakeCode{state: state, id: id, err: err}
	return code
}

func (f *fakeGoogle) attempt(state string) oidc.AuthParams {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.attempts[state]
}

func gid(email string) *oidc.Identity {
	return &oidc.Identity{Subject: "sub-" + uuid.NewString(), Email: email}
}

func googleOn(cfg *config.Config) {
	cfg.Google.ClientID = "test-client.apps.example"
	cfg.Google.ClientSecret = "test-secret-value"
	cfg.Google.RedirectURL = "https://api.example.test/api/v1/auth/google/callback"
}

// newGoogleEnv is newEnv with Google sign-in configured and a fake provider.
func newGoogleEnv(t *testing.T, opts ...option) (*env, *fakeGoogle) {
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
	cfg.Log.Level = envOr("STARYIELD_TEST_LOG_LEVEL", "error")
	cfg.Analytics.FlushInterval = 50 * time.Millisecond
	googleOn(cfg)
	for _, o := range opts {
		o(cfg)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("config invalid: %v", err)
	}
	log := zap.NewNop()
	fake := clock.NewFake(time.Now())
	rec := &mail.Recorder{}
	store := storage.NewMemory("https://assets.test")
	advisors := storage.NewMemory("https://advisors.test")
	chatImages := storage.NewMemory("")
	pushes := &fakePush{}
	provider := newFakeGoogle()
	ctx := context.Background()
	a, err := app.New(ctx, cfg, log, app.Overrides{Clock: fake, Mailer: rec, Storage: store, AdvisorStorage: advisors, ChatStorage: chatImages,
		PushSender: pushes, Google: provider})
	if err != nil {
		t.Fatalf("app: %v", err)
	}
	a.Start(ctx)
	srv := httptest.NewServer(a.Router)
	e := &env{t: t, app: a, srv: srv, clock: fake, mail: rec, store: store, advisors: advisors, chat: chatImages, push: pushes}
	t.Cleanup(func() {
		// The e-mail outbox is shared by every test of the package: deliver
		// the security e-mails this test enqueued, so a later test's drain
		// does not find them.
		_ = a.Scheduler.RunJob(context.Background(), app.MailJob)
		srv.CloseClientConnections()
		srv.Close()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		a.Shutdown(sctx)
	})
	return e, provider
}

// gbrowser is a browser that does not follow redirects (the callback answers
// a 303 to the web app, which is not running here).
func (e *env) gbrowser() *client {
	c := e.client()
	c.http.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return c
}

// gcb is a parsed callback answer.
type gcb struct {
	status int
	loc    *url.URL
	result string
	next   string
	header http.Header
	set    []string
}

func parseCallback(r response) gcb {
	out := gcb{status: r.status, header: r.header, set: r.header.Values("Set-Cookie")}
	if loc := r.header.Get("Location"); loc != "" {
		out.loc, _ = url.Parse(loc)
		if out.loc != nil {
			out.result, out.next = out.loc.Query().Get("result"), out.loc.Query().Get("next")
		}
	}
	return out
}

// startGoogle begins an attempt and returns its state.
func (c *client) startGoogle(body map[string]any) (state string, r response) {
	c.e.t.Helper()
	r = c.call(http.MethodPost, "/api/v1/auth/google/start", body)
	if r.status != http.StatusOK {
		return "", r
	}
	var out struct {
		URL string `json:"authorization_url"`
	}
	r.decode(c.e.t, &out)
	u, err := url.Parse(out.URL)
	if err != nil {
		c.e.t.Fatalf("authorization url: %v", err)
	}
	return u.Query().Get("state"), r
}

// callbackGoogle is the browser coming back from Google (a top-level GET
// navigation: no Origin, no CSRF header).
func (c *client) callbackGoogle(q url.Values) gcb {
	c.e.t.Helper()
	saved := c.origin
	c.origin = ""
	defer func() { c.origin = saved }()
	return parseCallback(c.raw(http.MethodGet, "/api/v1/auth/google/callback?"+q.Encode(), nil, nil))
}

// google runs a whole attempt: start, Google authenticates id, callback.
func (c *client) google(f *fakeGoogle, id *oidc.Identity, body map[string]any) gcb {
	c.e.t.Helper()
	state, r := c.startGoogle(body)
	if r.status != http.StatusOK {
		c.e.t.Fatalf("start %v: %d %s", body, r.status, r.body)
	}
	return c.callbackGoogle(url.Values{"code": {f.issue(state, id, nil)}, "state": {state}})
}

func (c *client) mustResult(f *fakeGoogle, id *oidc.Identity, body map[string]any, want string) gcb {
	c.e.t.Helper()
	out := c.google(f, id, body)
	if out.status != http.StatusSeeOther || out.result != want {
		c.e.t.Fatalf("callback for %v: want %s, got %d %s (%v)", body, want, out.status, out.result, out.loc)
	}
	return out
}

// googleAccount signs a new Google-only account up and returns its browser.
func (e *env) googleAccount(f *fakeGoogle, email string) (*client, *oidc.Identity) {
	e.t.Helper()
	c := e.gbrowser()
	id := gid(email)
	c.mustResult(f, id, map[string]any{"intent": "signup", "accept_terms": true}, "signed_up")
	var me service.UserView
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, &me)
	c.user = &me
	return c, id
}

func (e *env) identities(userID uuid.UUID) []domain.UserIdentity {
	e.t.Helper()
	var rows []domain.UserIdentity
	if err := e.app.DB.Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		e.t.Fatal(err)
	}
	return rows
}

func (e *env) userByEmail(email string) []domain.User {
	e.t.Helper()
	var rows []domain.User
	if err := e.app.DB.Where("email = ?", email).Find(&rows).Error; err != nil {
		e.t.Fatal(err)
	}
	return rows
}

func (c *client) waitNotification(kind string) service.NotificationView {
	c.e.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var page struct {
			Items []service.NotificationView `json:"items"`
		}
		c.must(http.StatusOK, http.MethodGet, "/api/v1/me/notifications", nil, &page)
		for _, n := range page.Items {
			if n.Kind == kind {
				return n
			}
		}
		if time.Now().After(deadline) {
			c.e.t.Fatalf("no %s notification: %+v", kind, page.Items)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (c *client) methods() service.SignInMethodsView {
	c.e.t.Helper()
	var v service.SignInMethodsView
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/sign-in-methods", nil, &v)
	return v
}

func hasSessionCookies(set []string) bool {
	for _, s := range set {
		if (strings.HasPrefix(s, "sy_at=") || strings.HasPrefix(s, "sy_rt=")) && !strings.Contains(s, "Max-Age=0") {
			return true
		}
	}
	return false
}

func cookieCleared(set []string, name string) bool {
	for _, s := range set {
		if strings.HasPrefix(s, name+"=;") || (strings.HasPrefix(s, name+"=") && (strings.Contains(s, "Max-Age=0") || strings.Contains(s, "Max-Age=-1"))) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------

func TestGoogleIsOffUntilConfigured(t *testing.T) {
	e := newEnv(t)
	c := e.gbrowser()
	var site struct {
		Auth struct {
			Google bool `json:"google"`
		} `json:"auth"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/site/config", nil, &site)
	if site.Auth.Google {
		t.Fatal("auth.google must be false without configuration")
	}
	if r := c.call(http.MethodPost, "/api/v1/auth/google/start", map[string]any{"intent": "signin"}); r.status != http.StatusServiceUnavailable ||
		r.errCode(t) != "google_unavailable" {
		t.Fatalf("start: %d %s", r.status, r.body)
	}
	out := c.callbackGoogle(url.Values{"code": {"x"}, "state": {"y"}})
	if out.status != http.StatusSeeOther || out.result != "google_unavailable" {
		t.Fatalf("callback: %+v", out)
	}
	reg := e.registerClient()
	if m := reg.methods(); !m.Password || m.GoogleAvailable || m.Google != nil {
		t.Fatalf("methods: %+v", m)
	}

	e2, _ := newGoogleEnv(t)
	e2.client().must(http.StatusOK, http.MethodGet, "/api/v1/site/config", nil, &site)
	if !site.Auth.Google {
		t.Fatal("auth.google must be true when configured")
	}
	// A partial configuration is refused at start-up.
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Google.ClientID = "only-an-id"
	if err := cfg.Validate(); err == nil {
		t.Fatal("a partial Google configuration must be refused")
	}
}

func TestGoogleSignUpWithTermsCreatesTheAccountThroughTheRegisterPath(t *testing.T) {
	e, f := newGoogleEnv(t)
	c := e.gbrowser()
	email := uniq("gnew") + "@Example.com"
	id := gid(email)
	state, r := c.startGoogle(map[string]any{"intent": "signup", "accept_terms": true, "next": "/profile", "locale": "fr",
		"attribution": map[string]any{"utm_source": "newsletter", "utm_medium": "email"}})
	if r.status != http.StatusOK {
		t.Fatalf("start: %d %s", r.status, r.body)
	}
	if ck := c.cookie("sy_oa", "/api/v1/auth/google/start"); ck == "" {
		t.Fatal("start must set the sy_oa cookie")
	}
	var setOA string
	for _, s := range r.header.Values("Set-Cookie") {
		if strings.HasPrefix(s, "sy_oa=") {
			setOA = s
		}
	}
	for _, want := range []string{"HttpOnly", "SameSite=Lax", "Path=/api/v1/auth", "Max-Age=600"} {
		if !strings.Contains(setOA, want) {
			t.Errorf("sy_oa cookie %q lacks %s", setOA, want)
		}
	}
	a := f.attempt(state)
	if a.Nonce == "" || a.CodeChallenge == "" || len(a.State) < 40 || a.Locale != "fr" {
		t.Fatalf("attempt: %+v", a)
	}

	out := c.callbackGoogle(url.Values{"code": {f.issue(state, id, nil)}, "state": {state}})
	if out.status != http.StatusSeeOther || out.result != "signed_up" || out.next != "/profile" ||
		out.loc.Path != "/fr/auth/google" || out.loc.Host != "localhost:3000" {
		t.Fatalf("callback: %+v", out)
	}
	if out.header.Get("Cache-Control") != "no-store" || out.header.Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("callback headers: %v", out.header)
	}
	if !hasSessionCookies(out.set) || !cookieCleared(out.set, "sy_oa") {
		t.Fatalf("callback cookies: %v", out.set)
	}
	if f.mismatches != 0 {
		t.Fatal("the API sent a nonce or verifier that does not belong to the attempt")
	}

	var me service.UserView
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, &me)
	if me.Email != strings.ToLower(email) || me.Role != "client" || me.Locale != "fr" || me.DisplayName != strings.Split(strings.ToLower(email), "@")[0] ||
		me.Wallet == nil || me.Wallet.FreeMinutes != 5 {
		t.Fatalf("new account: %+v", me)
	}
	var u domain.User
	e.app.DB.First(&u, "id = ?", me.ID)
	if u.PasswordHash != "" || u.HasPassword() || u.EmailVerifiedAt == nil || u.TermsAcceptedAt.IsZero() ||
		u.EmailVerifiedAt.Sub(e.clock.Now()).Abs() > time.Minute {
		t.Fatalf("stored user: %+v", u)
	}
	ids := e.identities(me.ID)
	if len(ids) != 1 || ids[0].Subject != id.Subject || ids[0].Provider != "google" || ids[0].Email != strings.ToLower(email) {
		t.Fatalf("identity: %+v", ids)
	}
	var attr domain.Attribution
	if err := e.app.DB.First(&attr, "user_id = ?", me.ID).Error; err != nil || attr.UTMSource != "newsletter" {
		t.Fatalf("attribution: %+v %v", attr, err)
	}
	if e.auditCount("auth.google.signup", me.ID) != 1 {
		t.Fatal("sign-up not audited")
	}
	if m := c.methods(); m.Password || m.Google == nil || m.Google.Email != strings.ToLower(email) || !m.GoogleAvailable {
		t.Fatalf("methods: %+v", m)
	}
	// The browser holds a usable session: CSRF-protected calls work.
	c.must(http.StatusOK, http.MethodPatch, "/api/v1/me/profile", map[string]any{"nickname": "Moon"}, nil)
	// The access token is real: realtime tickets work.
	c.must(http.StatusOK, http.MethodPost, "/api/v1/realtime/ticket", nil, nil)
}

func TestGoogleSignInOfALinkedAccount(t *testing.T) {
	e, f := newGoogleEnv(t)
	first, id := e.googleAccount(f, uniq("glink")+"@example.com")
	first.must(http.StatusNoContent, http.MethodPost, "/api/v1/auth/logout", nil, nil)

	c := e.gbrowser()
	out := c.mustResult(f, id, map[string]any{"intent": "signin", "next": "/chat"}, "signed_in")
	if out.next != "/chat" || !hasSessionCookies(out.set) {
		t.Fatalf("callback: %+v", out)
	}
	var me service.UserView
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, &me)
	if me.ID != first.user.ID {
		t.Fatalf("signed in as %s, want %s", me.ID, first.user.ID)
	}
	var u domain.User
	e.app.DB.First(&u, "id = ?", me.ID)
	if u.LastLoginAt == nil {
		t.Fatal("last_login_at not recorded")
	}
	// The sign-up intent for a known identity just signs in (no second account).
	d := e.gbrowser()
	d.mustResult(f, id, map[string]any{"intent": "signup", "accept_terms": true}, "signed_in")
	if n := len(e.userByEmail(first.user.Email)); n != 1 {
		t.Fatalf("accounts for the address: %d", n)
	}
	// A hostile next is dropped, a good one kept.
	for _, next := range []string{"//evil.example", "https://evil.example", "/\\evil.example", "/fr/profile", "/%2f%2fevil.example"} {
		out := e.gbrowser().mustResult(f, id, map[string]any{"intent": "signin", "next": next}, "signed_in")
		if out.next != "" || strings.Contains(out.loc.String(), "evil") {
			t.Errorf("next %q survived: %v", next, out.loc)
		}
	}
}

func TestGoogleSignInWithAFlippedIntentOnAnUnknownAccountAsksForConsent(t *testing.T) {
	e, f := newGoogleEnv(t)
	c := e.gbrowser()
	email := uniq("gconsent") + "@example.com"
	id := gid(email)
	out := c.mustResult(f, id, map[string]any{"intent": "signin"}, "signup_consent")
	if hasSessionCookies(out.set) || cookieCleared(out.set, "sy_oa") || len(e.userByEmail(email)) != 0 {
		t.Fatalf("nothing may be created or signed in before consent: %+v", out)
	}
	if r := c.call(http.MethodGet, "/api/v1/me", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("session before consent: %d", r.status)
	}
	var p map[string]any
	c.must(http.StatusOK, http.MethodGet, "/api/v1/auth/google/pending", nil, &p)
	if p["kind"] != "signup_consent" || p["email"] != email {
		t.Fatalf("pending: %v", p)
	}
	// Without the terms the pending sign-up survives.
	if r := c.call(http.MethodPost, "/api/v1/auth/google/complete", map[string]any{"accept_terms": false}); r.status != http.StatusUnprocessableEntity ||
		r.errCode(t) != "validation_failed" || !strings.Contains(string(r.body), "accept_terms") {
		t.Fatalf("complete without terms: %d %s", r.status, r.body)
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/auth/google/pending", nil, &p)
	// Another browser has nothing pending and cannot complete it.
	other := e.gbrowser()
	if r := other.call(http.MethodGet, "/api/v1/auth/google/pending", nil); r.status != http.StatusNotFound || r.errCode(t) != "not_found" {
		t.Fatalf("pending elsewhere: %d %s", r.status, r.body)
	}
	if r := other.call(http.MethodPost, "/api/v1/auth/google/complete", map[string]any{"accept_terms": true}); r.status != http.StatusBadRequest ||
		r.errCode(t) != "google_state_invalid" {
		t.Fatalf("complete elsewhere: %d %s", r.status, r.body)
	}

	var sess struct {
		User   *service.UserView `json:"user"`
		Linked string            `json:"linked"`
	}
	r := c.must(http.StatusCreated, http.MethodPost, "/api/v1/auth/google/complete", map[string]any{"accept_terms": true, "locale": "es",
		"attribution": map[string]any{"utm_source": "consent"}}, &sess)
	if sess.User == nil || sess.User.Email != email || sess.User.Locale != "es" || sess.User.Wallet == nil || sess.User.Wallet.FreeMinutes != 5 ||
		sess.Linked != "" {
		t.Fatalf("complete: %s", r.body)
	}
	var attr domain.Attribution
	if err := e.app.DB.First(&attr, "user_id = ?", sess.User.ID).Error; err != nil || attr.UTMSource != "consent" {
		t.Fatalf("attribution: %+v %v", attr, err)
	}
	if ids := e.identities(sess.User.ID); len(ids) != 1 || ids[0].Subject != id.Subject {
		t.Fatalf("identity: %+v", ids)
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, nil)
	// Replay: the pending record is single use.
	if r := c.call(http.MethodPost, "/api/v1/auth/google/complete", map[string]any{"accept_terms": true}); r.status != http.StatusBadRequest ||
		r.errCode(t) != "google_state_invalid" {
		t.Fatalf("complete replay: %d %s", r.status, r.body)
	}
	if r := c.call(http.MethodGet, "/api/v1/auth/google/pending", nil); r.status != http.StatusNotFound {
		t.Fatalf("pending after complete: %d", r.status)
	}
	if n := len(e.userByEmail(email)); n != 1 {
		t.Fatalf("accounts: %d", n)
	}
}

func TestGoogleSignUpIntentWithoutTermsAsksForThemToo(t *testing.T) {
	e, f := newGoogleEnv(t)
	c := e.gbrowser()
	c.mustResult(f, gid(uniq("gnoterms")+"@example.com"), map[string]any{"intent": "signup", "accept_terms": false}, "signup_consent")
}

func TestGoogleCompleteRegisteredMeanwhileIsEmailTaken(t *testing.T) {
	e, f := newGoogleEnv(t)
	c := e.gbrowser()
	email := uniq("graced") + "@example.com"
	c.mustResult(f, gid(email), map[string]any{"intent": "signin"}, "signup_consent")
	// Someone registers the address with a password before consent is given.
	other := e.client()
	other.must(http.StatusCreated, http.MethodPost, "/api/v1/auth/register", map[string]any{"email": email, "password": strongPassword,
		"accept_terms": true}, nil)
	if r := c.call(http.MethodPost, "/api/v1/auth/google/complete", map[string]any{"accept_terms": true}); r.status != http.StatusConflict ||
		r.errCode(t) != "email_taken" {
		t.Fatalf("complete: %d %s", r.status, r.body)
	}
	if n := len(e.userByEmail(email)); n != 1 {
		t.Fatalf("accounts: %d", n)
	}
}

func TestGoogleExistingEmailMustProveThePasswordOnceToLink(t *testing.T) {
	e, f := newGoogleEnv(t)
	acct := e.registerClient()
	id := gid(strings.ToUpper(acct.user.Email[:1]) + acct.user.Email[1:]) // same address, other case
	c := e.gbrowser()
	// Even the sign-up intent with accepted terms never signs in or creates
	// an account for an address that already has one.
	out := c.mustResult(f, id, map[string]any{"intent": "signup", "accept_terms": true}, "link_password")
	if hasSessionCookies(out.set) || cookieCleared(out.set, "sy_oa") {
		t.Fatalf("callback: %+v", out)
	}
	if r := c.call(http.MethodGet, "/api/v1/me", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("session before the password: %d", r.status)
	}
	if len(e.identities(acct.user.ID)) != 0 || len(e.userByEmail(acct.user.Email)) != 1 {
		t.Fatal("nothing may be linked or created by e-mail match")
	}
	var p map[string]any
	c.must(http.StatusOK, http.MethodGet, "/api/v1/auth/google/pending", nil, &p)
	if p["kind"] != "link_password" || p["email"] != acct.user.Email {
		t.Fatalf("pending: %v", p)
	}

	// A wrong password does not link (and the pending link survives).
	if r := c.call(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": acct.user.Email, "password": "Wr0ng!Passw0rd#"}); r.status != http.StatusUnauthorized {
		t.Fatalf("wrong password: %d", r.status)
	}
	// A sign-in to a different account does not link either.
	bystander := e.registerClient()
	var sess struct {
		User   *service.UserView `json:"user"`
		Linked string            `json:"linked"`
	}
	c.must(http.StatusOK, http.MethodPost, "/api/v1/auth/login", map[string]any{"email": bystander.user.Email, "password": strongPassword}, &sess)
	if sess.Linked != "" || len(e.identities(acct.user.ID)) != 0 || len(e.identities(bystander.user.ID)) != 0 {
		t.Fatalf("a login for another e-mail linked Google: %+v", sess)
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/auth/google/pending", nil, &p)
	c.must(http.StatusNoContent, http.MethodPost, "/api/v1/auth/logout", nil, nil)

	// The right password links, in the same sign-in.
	e.flushMail()
	var res response = c.must(http.StatusOK, http.MethodPost, "/api/v1/auth/login", map[string]any{"email": acct.user.Email, "password": strongPassword}, &sess)
	if sess.Linked != "google" || sess.User == nil || sess.User.ID != acct.user.ID {
		t.Fatalf("login: %s", res.body)
	}
	ids := e.identities(acct.user.ID)
	if len(ids) != 1 || ids[0].Subject != id.Subject {
		t.Fatalf("identity: %+v", ids)
	}
	if !cookieCleared(res.header.Values("Set-Cookie"), "sy_oa") {
		t.Fatalf("sy_oa must be cleared once consumed: %v", res.header.Values("Set-Cookie"))
	}
	if r := c.call(http.MethodGet, "/api/v1/auth/google/pending", nil); r.status != http.StatusNotFound {
		t.Fatalf("pending after link: %d", r.status)
	}
	if m := c.methods(); !m.Password || m.Google == nil {
		t.Fatalf("methods: %+v", m)
	}
	// Audit, in-app notification (masked e-mail) and the security e-mail.
	if e.auditCount("auth.google.linked", acct.user.ID) != 1 {
		t.Fatal("link not audited")
	}
	n := c.waitNotification("account.google_linked")
	if masked, _ := n.Data["email"].(string); masked == "" || masked == acct.user.Email || !strings.Contains(masked, "@") {
		t.Fatalf("notification data: %v", n.Data)
	}
	var added *mail.Message
	for _, m := range e.flushMail() {
		if strings.EqualFold(m.To, acct.user.Email) && strings.Contains(m.Subject, "Google sign-in was added") {
			m := m
			added = &m
		}
	}
	if added == nil || added.Category != "security" || strings.Contains(added.Text, acct.user.Email) {
		t.Fatalf("security e-mail: %+v", added)
	}
	// From now on Google signs in to that account; the next password login
	// carries no link.
	d := e.gbrowser()
	d.mustResult(f, id, map[string]any{"intent": "signin"}, "signed_in")
	var me service.UserView
	d.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, &me)
	if me.ID != acct.user.ID {
		t.Fatalf("signed in as %s", me.ID)
	}
	again := e.client()
	sess.Linked = ""
	again.must(http.StatusOK, http.MethodPost, "/api/v1/auth/login", map[string]any{"email": acct.user.Email, "password": strongPassword}, &sess)
	if sess.Linked != "" {
		t.Fatal("linked twice")
	}
}

func TestGoogleLinkFromSettings(t *testing.T) {
	e, f := newGoogleEnv(t)
	acct := e.registerClient()
	if r := acct.call(http.MethodPost, "/api/v1/auth/google/start", map[string]any{"intent": "link", "next": "/profile/settings"}); r.status != http.StatusOK {
		t.Fatalf("start: %d %s", r.status, r.body)
	}
	// Signed out: linking needs a session. Admin: never.
	if r := e.client().call(http.MethodPost, "/api/v1/auth/google/start", map[string]any{"intent": "link"}); r.status != http.StatusUnauthorized {
		t.Fatalf("anonymous link: %d %s", r.status, r.body)
	}
	_, adminEmail := e.insertAdmin()
	admin := e.login(adminEmail, strongPassword) // an admin signed in on the web app
	if r := admin.call(http.MethodPost, "/api/v1/auth/google/start", map[string]any{"intent": "link"}); r.status != http.StatusForbidden ||
		r.errCode(t) != "google_not_allowed" {
		t.Fatalf("admin link: %d %s", r.status, r.body)
	}
	if r := acct.call(http.MethodPost, "/api/v1/auth/google/start", map[string]any{"intent": "nonsense"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("bad intent: %d", r.status)
	}

	// The signed-in browser links an unrelated Google account.
	b := e.gbrowser()
	b.http.Jar = acct.http.Jar
	b.csrf, b.user = acct.csrf, acct.user
	id := gid(uniq("gother") + "@gmail.com")
	e.flushMail()
	out := b.mustResult(f, id, map[string]any{"intent": "link", "next": "/profile/settings"}, "linked")
	if out.next != "/profile/settings" || hasSessionCookies(out.set) {
		t.Fatalf("callback: %+v", out)
	}
	m := b.methods()
	if !m.Password || m.Google == nil || m.Google.Email != id.Email || m.Google.LinkedAt.IsZero() {
		t.Fatalf("methods: %+v", m)
	}
	n := b.waitNotification("account.google_linked")
	if masked, _ := n.Data["email"].(string); !strings.Contains(masked, "•••@gmail.com") {
		t.Fatalf("masked e-mail: %v", n.Data)
	}
	var added bool
	for _, mm := range e.flushMail() {
		added = added || (strings.EqualFold(mm.To, acct.user.Email) && strings.Contains(mm.Subject, "Google sign-in was added") && mm.Category == "security")
	}
	if !added {
		t.Fatal("no security e-mail for the link")
	}
	// Linking the same Google account again is idempotent; another one is refused.
	b.mustResult(f, id, map[string]any{"intent": "link"}, "linked")
	b.mustResult(f, gid(uniq("gthird")+"@gmail.com"), map[string]any{"intent": "link"}, "google_account_in_use")
	if len(e.identities(acct.user.ID)) != 1 || e.identities(acct.user.ID)[0].Subject != id.Subject {
		t.Fatal("nothing may be replaced")
	}

	// Someone else cannot take that Google account.
	other := e.registerClient()
	ob := e.gbrowser()
	ob.http.Jar = other.http.Jar
	ob.csrf, ob.user = other.csrf, other.user
	ob.mustResult(f, id, map[string]any{"intent": "link"}, "google_account_in_use")
	if len(e.identities(other.user.ID)) != 0 {
		t.Fatal("a Google account was linked to two users")
	}
	// ...nor sign up with it.
	e.gbrowser().mustResult(f, id, map[string]any{"intent": "signup", "accept_terms": true}, "signed_in")
}

func TestGoogleDisconnectNeedsThePassword(t *testing.T) {
	e, f := newGoogleEnv(t)
	acct := e.registerClient()
	b := e.gbrowser()
	b.http.Jar = acct.http.Jar
	b.csrf, b.user = acct.csrf, acct.user
	id := gid(uniq("gdis") + "@gmail.com")
	b.mustResult(f, id, map[string]any{"intent": "link"}, "linked")
	e.flushMail()

	if r := b.call(http.MethodDelete, "/api/v1/me/sign-in-methods/google", map[string]any{"password": "Wr0ng!Passw0rd#"}); r.status != http.StatusUnauthorized ||
		r.errCode(t) != "invalid_credentials" {
		t.Fatalf("wrong password: %d %s", r.status, r.body)
	}
	if r := b.call(http.MethodDelete, "/api/v1/me/sign-in-methods/google", map[string]any{}); r.status != http.StatusUnauthorized {
		t.Fatalf("no password: %d", r.status)
	}
	if b.methods().Google == nil {
		t.Fatal("a refused disconnect removed Google")
	}
	b.must(http.StatusNoContent, http.MethodDelete, "/api/v1/me/sign-in-methods/google", map[string]any{"password": strongPassword}, nil)
	if m := b.methods(); m.Google != nil || !m.Password {
		t.Fatalf("methods: %+v", m)
	}
	if r := b.call(http.MethodDelete, "/api/v1/me/sign-in-methods/google", map[string]any{"password": strongPassword}); r.status != http.StatusNotFound {
		t.Fatalf("second disconnect: %d %s", r.status, r.body)
	}
	b.waitNotification("account.google_unlinked")
	var removed bool
	for _, m := range e.flushMail() {
		removed = removed || (strings.EqualFold(m.To, acct.user.Email) && strings.Contains(m.Subject, "Google sign-in was removed") && m.Category == "security")
	}
	if !removed || e.auditCount("auth.google.unlinked", acct.user.ID) != 1 {
		t.Fatal("unlink not mailed/audited")
	}
	// Google no longer signs in to the account: it is an unknown identity again.
	e.gbrowser().mustResult(f, id, map[string]any{"intent": "signin"}, "signup_consent")
}

func TestGoogleOnlyAccountCannotDisconnectGoogleNorUsePasswordGatedActions(t *testing.T) {
	e, f := newGoogleEnv(t)
	c, id := e.googleAccount(f, uniq("gonly")+"@example.com")

	// The last way in stays.
	if r := c.call(http.MethodDelete, "/api/v1/me/sign-in-methods/google", map[string]any{"password": strongPassword}); r.status != http.StatusConflict ||
		r.errCode(t) != "last_sign_in_method" {
		t.Fatalf("disconnect: %d %s", r.status, r.body)
	}
	// Every password-gated action explains itself instead of failing like a typo.
	for _, tc := range []struct {
		method, path string
		body         map[string]any
	}{
		{http.MethodPost, "/api/v1/auth/password/change", map[string]any{"current_password": strongPassword, "new_password": "N3w!Passw0rd#2026"}},
		{http.MethodPost, "/api/v1/me/email-change", map[string]any{"email": uniq("moved") + "@example.com", "password": strongPassword}},
		{http.MethodPost, "/api/v1/me/deactivate", map[string]any{"password": strongPassword}},
		{http.MethodPost, "/api/v1/me/close", map[string]any{"password": strongPassword, "confirmation": "DELETE"}},
		{http.MethodPost, "/api/v1/me/exports", map[string]any{"password": strongPassword}},
	} {
		if r := c.call(tc.method, tc.path, tc.body); r.status != http.StatusConflict || r.errCode(t) != "password_not_set" {
			t.Errorf("%s: %d %s", tc.path, r.status, r.body)
		}
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, nil) // still signed in

	// Password sign-in fails exactly like a wrong password or an unknown e-mail.
	probe := func(email, password string) response {
		return e.client().call(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": email, "password": password})
	}
	known := probe(c.user.Email, "")
	for _, r := range []response{probe(c.user.Email, strongPassword), probe(c.user.Email, "Wr0ng!Passw0rd#"), probe(uniq("ghost")+"@example.com", strongPassword)} {
		if r.status != http.StatusUnauthorized || r.errCode(t) != "invalid_credentials" {
			t.Fatalf("password login: %d %s", r.status, r.body)
		}
	}
	_ = known
	// ...and counts toward the lockout like a wrong password.
	locked := false
	for i := 0; i < 8 && !locked; i++ {
		locked = probe(c.user.Email, strongPassword).status == http.StatusTooManyRequests
	}
	if !locked {
		t.Fatal("failed password attempts on a Google-only account must still lock it")
	}

	// Password reset sets the first password and unlocks the gated actions.
	e.client().must(http.StatusAccepted, http.MethodPost, "/api/v1/auth/password/forgot", map[string]any{"email": c.user.Email}, nil)
	token := tokenIn(t, mailTo(t, e.flushMail(), c.user.Email, "/reset-password?token="))
	first := "F1rst!Passw0rd#2026"
	e.client().must(http.StatusNoContent, http.MethodPost, "/api/v1/auth/password/reset", map[string]any{"token": token, "password": first}, nil)
	// (The lockout window is real time in Redis; clear it for the next sign-in.)
	e.app.Redis.Del(context.Background(), e.app.Config.Redis.KeyPrefix+"auth:fail:"+security.HashToken(c.user.Email)[:32])
	p := e.login(c.user.Email, first)
	if m := p.methods(); !m.Password || m.Google == nil {
		t.Fatalf("methods after reset: %+v", m)
	}
	p.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/email-change", map[string]any{"email": uniq("moved") + "@example.com", "password": first}, nil)
	p.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/exports", map[string]any{"password": first}, nil)
	// Google still signs in, and can now be disconnected with the new password.
	e.gbrowser().mustResult(f, id, map[string]any{"intent": "signin"}, "signed_in")
	p2 := e.login(c.user.Email, first)
	p2.must(http.StatusNoContent, http.MethodDelete, "/api/v1/me/sign-in-methods/google", map[string]any{"password": first}, nil)
	if m := p2.methods(); m.Google != nil {
		t.Fatalf("methods: %+v", m)
	}
}

func TestGoogleStateRules(t *testing.T) {
	e, f := newGoogleEnv(t)
	email := uniq("gstate") + "@example.com"
	id := gid(email)

	t.Run("a state is single use", func(t *testing.T) {
		c := e.gbrowser()
		state, _ := c.startGoogle(map[string]any{"intent": "signin"})
		code := f.issue(state, id, nil)
		if out := c.callbackGoogle(url.Values{"code": {code}, "state": {state}}); out.result != "signup_consent" {
			t.Fatalf("first: %+v", out)
		}
		// The cookie is still there (consent pending), the state is gone.
		if out := c.callbackGoogle(url.Values{"code": {f.issue(state, id, nil)}, "state": {state}}); out.result != "google_state_invalid" {
			t.Fatalf("replay: %+v", out)
		}
	})
	t.Run("without or with a foreign sy_oa cookie the callback is refused and the attempt survives", func(t *testing.T) {
		victim := e.gbrowser()
		state, _ := victim.startGoogle(map[string]any{"intent": "signin"})
		code := f.issue(state, id, nil)
		// Login CSRF / code injection: another browser delivers the victim's code and state.
		attacker := e.gbrowser()
		if out := attacker.callbackGoogle(url.Values{"code": {code}, "state": {state}}); out.result != "google_state_invalid" || hasSessionCookies(out.set) {
			t.Fatalf("no cookie: %+v", out)
		}
		attacker.setCookie("sy_oa", "attacker-chosen-value", "/api/v1/auth")
		if out := attacker.callbackGoogle(url.Values{"code": {code}, "state": {state}}); out.result != "google_state_invalid" || hasSessionCookies(out.set) {
			t.Fatalf("wrong cookie: %+v", out)
		}
		// The other way round: the attacker's state with the victim's cookie.
		astate, _ := attacker.startGoogle(map[string]any{"intent": "signin"})
		if out := victim.callbackGoogle(url.Values{"code": {f.issue(astate, gid(uniq("evil")+"@example.com"), nil)}, "state": {astate}}); out.result != "google_state_invalid" {
			t.Fatalf("foreign state: %+v", out)
		}
		// The genuine attempt was not burned by the forgeries.
		if out := victim.callbackGoogle(url.Values{"code": {code}, "state": {state}}); out.result != "signup_consent" {
			t.Fatalf("genuine callback: %+v", out)
		}
	})
	t.Run("an expired attempt", func(t *testing.T) {
		c := e.gbrowser()
		state, _ := c.startGoogle(map[string]any{"intent": "signin"})
		e.clock.Advance(11 * time.Minute)
		if out := c.callbackGoogle(url.Values{"code": {f.issue(state, id, nil)}, "state": {state}}); out.result != "google_state_invalid" {
			t.Fatalf("expired: %+v", out)
		}
	})
	t.Run("malformed callbacks", func(t *testing.T) {
		c := e.gbrowser()
		state, _ := c.startGoogle(map[string]any{"intent": "signin"})
		for _, q := range []url.Values{{}, {"code": {"x"}}, {"state": {"nope"}, "code": {"x"}}, {"state": {strings.Repeat("a", 500)}, "code": {"x"}},
			{"state": {state}}} {
			if out := c.callbackGoogle(q); out.status != http.StatusSeeOther || out.result != "google_state_invalid" {
				t.Fatalf("%v: %+v", q, out)
			}
		}
		// The last one (valid state, no code) consumed the attempt; a code for it is useless.
		if out := c.callbackGoogle(url.Values{"code": {f.issue(state, id, nil)}, "state": {state}}); out.result != "google_state_invalid" {
			t.Fatalf("after: %+v", out)
		}
	})
	t.Run("a code that was not issued for the attempt", func(t *testing.T) {
		c := e.gbrowser()
		state, _ := c.startGoogle(map[string]any{"intent": "signin"})
		other, _ := e.gbrowser().startGoogle(map[string]any{"intent": "signin"})
		// The code was bound to another attempt's nonce and verifier.
		if out := c.callbackGoogle(url.Values{"code": {f.issue(other, id, nil)}, "state": {state}}); out.result != "google_state_invalid" {
			t.Fatalf("mismatched code: %+v", out)
		}
	})
	t.Run("the callback is a plain GET: no CSRF header, any Origin", func(t *testing.T) {
		c := e.gbrowser()
		state, _ := c.startGoogle(map[string]any{"intent": "signin"})
		r := c.raw(http.MethodGet, "/api/v1/auth/google/callback?"+url.Values{"code": {f.issue(state, gid(uniq("gorigin")+"@example.com"), nil)}, "state": {state}}.Encode(),
			nil, map[string]string{"Origin": "https://accounts.google.com"})
		if got := parseCallback(r); got.status != http.StatusSeeOther || got.result != "signup_consent" {
			t.Fatalf("callback with a foreign Origin: %d %s", r.status, r.body)
		}
	})
	t.Run("provider-side outcomes", func(t *testing.T) {
		for _, tc := range []struct {
			q    url.Values
			want string
		}{
			{url.Values{"error": {"access_denied"}}, "google_cancelled"},
			{url.Values{"error": {"invalid_request"}}, "google_cancelled"},
			{url.Values{"error": {"temporarily_unavailable"}}, "google_unavailable"},
		} {
			c := e.gbrowser()
			state, _ := c.startGoogle(map[string]any{"intent": "signin"})
			tc.q.Set("state", state)
			if out := c.callbackGoogle(tc.q); out.status != http.StatusSeeOther || out.result != tc.want || !cookieCleared(out.set, "sy_oa") {
				t.Fatalf("%v: %+v", tc.q, out)
			}
		}
		for err, want := range map[error]string{oidc.ErrEmailUnverified: "google_email_unverified", oidc.ErrUnavailable: "google_unavailable",
			oidc.ErrInvalidGrant: "google_state_invalid", oidc.ErrInvalidToken: "google_state_invalid"} {
			c := e.gbrowser()
			state, _ := c.startGoogle(map[string]any{"intent": "signin"})
			if out := c.callbackGoogle(url.Values{"code": {f.issue(state, nil, err)}, "state": {state}}); out.result != want {
				t.Fatalf("%v: %+v", err, out)
			}
		}
		f.mu.Lock()
		f.down = true
		f.mu.Unlock()
		defer func() { f.mu.Lock(); f.down = false; f.mu.Unlock() }()
		if r := e.gbrowser().call(http.MethodPost, "/api/v1/auth/google/start", map[string]any{"intent": "signin"}); r.status != http.StatusServiceUnavailable ||
			r.errCode(t) != "google_unavailable" {
			t.Fatalf("start while Google is down: %d %s", r.status, r.body)
		}
	})
	if f.mismatches != 1 {
		t.Fatalf("only the deliberately mismatched code may fail the nonce/verifier check, got %d", f.mismatches)
	}
}

func TestGoogleRefusesAdministrators(t *testing.T) {
	e, f := newGoogleEnv(t)
	// An admin's address is never linked or signed in through Google.
	_, adminEmail := e.insertAdmin()
	c := e.gbrowser()
	out := c.mustResult(f, gid(adminEmail), map[string]any{"intent": "signin"}, "google_not_allowed")
	if hasSessionCookies(out.set) {
		t.Fatal("an admin session was issued")
	}
	c.mustResult(f, gid(adminEmail), map[string]any{"intent": "signup", "accept_terms": true}, "google_not_allowed")
	if r := c.call(http.MethodGet, "/api/v1/auth/google/pending", nil); r.status != http.StatusNotFound {
		t.Fatalf("pending for an admin: %d", r.status)
	}

	// A user promoted to admin after linking Google can no longer use it.
	acct, id := e.googleAccount(f, uniq("gpromoted")+"@example.com")
	if err := e.app.DB.Model(&domain.User{}).Where("id = ?", acct.user.ID).Update("role", domain.RoleAdmin).Error; err != nil {
		t.Fatal(err)
	}
	e.gbrowser().mustResult(f, id, map[string]any{"intent": "signin"}, "google_not_allowed")
	if m := acct.methods(); m.GoogleAvailable {
		t.Fatalf("an admin is not offered Google: %+v", m)
	}
}

func TestGoogleDeactivatedAndBannedAccounts(t *testing.T) {
	e, f := newGoogleEnv(t)
	acct := e.registerClient()
	b := e.gbrowser()
	b.http.Jar = acct.http.Jar
	b.csrf, b.user = acct.csrf, acct.user
	id := gid(uniq("gdeact") + "@gmail.com")
	b.mustResult(f, id, map[string]any{"intent": "link"}, "linked")
	acct.must(http.StatusNoContent, http.MethodPost, "/api/v1/me/deactivate", map[string]any{"password": strongPassword}, nil)

	c := e.gbrowser()
	out := c.mustResult(f, id, map[string]any{"intent": "signin"}, "reactivation")
	if hasSessionCookies(out.set) || cookieCleared(out.set, "sy_oa") {
		t.Fatalf("callback: %+v", out)
	}
	var p struct {
		Kind  string `json:"kind"`
		Code  string `json:"code"`
		Token string `json:"reactivation_token"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/auth/google/pending", nil, &p)
	if p.Kind != "reactivation" || p.Code != "account_deactivated" || len(p.Token) < 20 {
		t.Fatalf("pending: %+v", p)
	}
	var sess struct {
		User *service.UserView `json:"user"`
	}
	c.must(http.StatusOK, http.MethodPost, "/api/v1/auth/reactivate", map[string]any{"token": p.Token}, &sess)
	if sess.User == nil || sess.User.ID != acct.user.ID || sess.User.Status != "active" {
		t.Fatalf("reactivated: %+v", sess.User)
	}
	e.gbrowser().mustResult(f, id, map[string]any{"intent": "signin"}, "signed_in")

	// A closure that is pending offers to keep the account.
	c2 := e.gbrowser()
	c2.http.Jar = c.http.Jar
	c2.csrf = c.csrf
	c2.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/close", map[string]any{"password": strongPassword, "confirmation": "DELETE"}, nil)
	d := e.gbrowser()
	d.mustResult(f, id, map[string]any{"intent": "signin"}, "reactivation")
	var cp map[string]any
	d.must(http.StatusOK, http.MethodGet, "/api/v1/auth/google/pending", nil, &cp)
	if cp["code"] != "account_closure_pending" || cp["deletion_scheduled_at"] == nil || cp["reactivation_token"] == "" {
		t.Fatalf("closure pending: %v", cp)
	}

	// Banned (admin action): the generic account_disabled screen, no offer.
	if err := e.app.DB.Model(&domain.User{}).Where("id = ?", acct.user.ID).Updates(map[string]any{"status": domain.UserBanned, "deactivated_by": ""}).Error; err != nil {
		t.Fatal(err)
	}
	g := e.gbrowser()
	out = g.mustResult(f, id, map[string]any{"intent": "signin"}, "account_disabled")
	if hasSessionCookies(out.set) {
		t.Fatal("a banned account got a session")
	}
	if r := g.call(http.MethodGet, "/api/v1/auth/google/pending", nil); r.status != http.StatusNotFound {
		t.Fatalf("pending for a banned account: %d", r.status)
	}
}

// The attack the contract exists for: someone pre-registers a victim's
// address with a password they know.
func TestGooglePreHijackedAddressIsNeverLinkedOrSignedInWithoutThePassword(t *testing.T) {
	e, f := newGoogleEnv(t)
	victimEmail := uniq("victim") + "@example.com"
	attacker := e.client()
	var reg struct {
		User *service.UserView `json:"user"`
	}
	attacker.must(http.StatusCreated, http.MethodPost, "/api/v1/auth/register", map[string]any{"email": victimEmail, "password": strongPassword,
		"accept_terms": true}, &reg)
	hijackedID := reg.User.ID

	victim := e.gbrowser()
	vid := gid(victimEmail)
	for _, body := range []map[string]any{{"intent": "signin"}, {"intent": "signup", "accept_terms": true}} {
		out := victim.mustResult(f, vid, body, "link_password")
		if hasSessionCookies(out.set) {
			t.Fatalf("a session was issued for the pre-registered account: %+v", out)
		}
	}
	if r := victim.call(http.MethodGet, "/api/v1/me", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("victim has a session: %d", r.status)
	}
	if len(e.identities(hijackedID)) != 0 || len(e.userByEmail(victimEmail)) != 1 {
		t.Fatal("the Google identity must not be linked, nor a second account created")
	}
	// The victim cannot complete anything without the password...
	if r := victim.call(http.MethodPost, "/api/v1/auth/google/complete", map[string]any{"accept_terms": true}); r.status != http.StatusBadRequest {
		t.Fatalf("complete: %d %s", r.status, r.body)
	}
	// ...and the attacker's own Google attempt (another address) cannot use
	// the victim's pending link by logging in with the password they know.
	attackerGoogle := e.gbrowser()
	attackerGoogle.http.Jar = attacker.http.Jar
	attackerGoogle.mustResult(f, gid(uniq("attackeralt")+"@example.com"), map[string]any{"intent": "signin"}, "signup_consent")
	var sess struct {
		Linked string `json:"linked"`
	}
	attackerGoogle.must(http.StatusOK, http.MethodPost, "/api/v1/auth/login", map[string]any{"email": victimEmail, "password": strongPassword}, &sess)
	if sess.Linked != "" || len(e.identities(hijackedID)) != 0 {
		t.Fatal("the attacker linked a Google account to the pre-registered address")
	}

	// The victim takes the account back with the reset flow (which signs the
	// attacker out), and only then connects Google with the new password.
	victim.must(http.StatusAccepted, http.MethodPost, "/api/v1/auth/password/forgot", map[string]any{"email": victimEmail}, nil)
	token := tokenIn(t, mailTo(t, e.flushMail(), victimEmail, "/reset-password?token="))
	mine := "V1ctim!Passw0rd#2026"
	victim.must(http.StatusNoContent, http.MethodPost, "/api/v1/auth/password/reset", map[string]any{"token": token, "password": mine}, nil)
	if r := attacker.call(http.MethodGet, "/api/v1/me", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("the attacker's session survived the reset: %d", r.status)
	}
	if r := e.client().call(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": victimEmail, "password": strongPassword}); r.status != http.StatusUnauthorized {
		t.Fatalf("the attacker's password still works: %d", r.status)
	}
	victim.mustResult(f, vid, map[string]any{"intent": "signin"}, "link_password")
	victim.must(http.StatusOK, http.MethodPost, "/api/v1/auth/login", map[string]any{"email": victimEmail, "password": mine}, &sess)
	if sess.Linked != "google" || len(e.identities(hijackedID)) != 1 {
		t.Fatalf("link after proving the password: %+v", sess)
	}
}

func TestGoogleConcurrentCallbacksCreateOneAccount(t *testing.T) {
	e, f := newGoogleEnv(t)
	email := uniq("grace") + "@example.com"
	id := gid(email)
	const n = 8
	type attempt struct {
		c     *client
		state string
		code  string
	}
	attempts := make([]attempt, n)
	for i := range attempts {
		c := e.gbrowser()
		state, _ := c.startGoogle(map[string]any{"intent": "signup", "accept_terms": true})
		attempts[i] = attempt{c: c, state: state, code: f.issue(state, id, nil)}
	}
	results := make([]string, n)
	statuses := make([]int, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range attempts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			req, _ := http.NewRequest(http.MethodGet, e.srv.URL+"/api/v1/auth/google/callback?"+url.Values{"code": {attempts[i].code}, "state": {attempts[i].state}}.Encode(), nil)
			res, err := attempts[i].c.http.Do(req)
			if err != nil {
				results[i] = "error: " + err.Error()
				return
			}
			defer res.Body.Close()
			_, _ = io.Copy(io.Discard, res.Body)
			statuses[i] = res.StatusCode
			if loc, err := url.Parse(res.Header.Get("Location")); err == nil {
				results[i] = loc.Query().Get("result")
			}
		}(i)
	}
	close(start)
	wg.Wait()
	t.Logf("callback results: %v", results)
	signedUp := 0
	for i, r := range results {
		if statuses[i] != http.StatusSeeOther || (r != "signed_up" && r != "signed_in") {
			t.Fatalf("callback %d: %d %q", i, statuses[i], r)
		}
		if r == "signed_up" {
			signedUp++
		}
	}
	if signedUp != 1 {
		t.Fatalf("%d callbacks created the account: %v", signedUp, results)
	}
	users := e.userByEmail(email)
	if len(users) != 1 {
		t.Fatalf("accounts: %d", len(users))
	}
	if ids := e.identities(users[0].ID); len(ids) != 1 {
		t.Fatalf("identities: %+v", ids)
	}
	var wallets int64
	e.app.DB.Model(&domain.Wallet{}).Where("user_id = ?", users[0].ID).Count(&wallets)
	if wallets != 1 {
		t.Fatalf("wallets: %d", wallets)
	}
	// Every browser that was told it signed in holds a working session.
	for i, a := range attempts {
		var me service.UserView
		a.c.must(http.StatusOK, http.MethodGet, "/api/v1/me", nil, &me)
		if me.ID != users[0].ID {
			t.Fatalf("browser %d signed in as %s", i, me.ID)
		}
	}

	// The same race through the consent step.
	email2 := uniq("grace2") + "@example.com"
	id2 := gid(email2)
	browsers := make([]*client, n)
	for i := range browsers {
		browsers[i] = e.gbrowser()
		browsers[i].mustResult(f, id2, map[string]any{"intent": "signin"}, "signup_consent")
	}
	codes := make([]int, n)
	var wg2 sync.WaitGroup
	start2 := make(chan struct{})
	for i := range browsers {
		wg2.Add(1)
		go func(i int) {
			defer wg2.Done()
			<-start2
			body := strings.NewReader(`{"accept_terms":true}`)
			req, _ := http.NewRequest(http.MethodPost, e.srv.URL+"/api/v1/auth/google/complete", body)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", origin)
			req.Header.Set("X-CSRF-Token", browsers[i].csrf)
			res, err := browsers[i].http.Do(req)
			if err != nil {
				return
			}
			defer res.Body.Close()
			_, _ = io.Copy(io.Discard, res.Body)
			codes[i] = res.StatusCode
		}(i)
	}
	for i := range browsers {
		browsers[i].ensureCSRF()
	}
	close(start2)
	wg2.Wait()
	for i, s := range codes {
		if s != http.StatusCreated {
			t.Errorf("complete %d: %d", i, s)
		}
	}
	if n := len(e.userByEmail(email2)); n != 1 {
		t.Fatalf("accounts after concurrent consent: %d", n)
	}
}

func TestGoogleErasureFreesTheIdentityAndTheExportAndAdminViewShowSignInMethods(t *testing.T) {
	e, f := newGoogleEnv(t)
	ctx := context.Background()
	acct := e.registerClient()
	b := e.gbrowser()
	b.http.Jar = acct.http.Jar
	b.csrf, b.user = acct.csrf, acct.user
	id := gid(uniq("gexp") + "@gmail.com")
	b.mustResult(f, id, map[string]any{"intent": "link"}, "linked")

	// Admin console: how each account signs in.
	admin := e.createAdmin()
	var view struct {
		Methods []string `json:"sign_in_methods"`
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/users/"+acct.user.ID.String(), nil, &view)
	if strings.Join(view.Methods, ",") != "password,google" {
		t.Fatalf("admin view: %v", view.Methods)
	}
	gonly, _ := e.googleAccount(f, uniq("gonlyview")+"@example.com")
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/users/"+gonly.user.ID.String(), nil, &view)
	if strings.Join(view.Methods, ",") != "google" {
		t.Fatalf("admin view (Google only): %v", view.Methods)
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/users/"+admin.adminID.String(), nil, &view)
	if strings.Join(view.Methods, ",") != "password" {
		t.Fatalf("admin view (admin): %v", view.Methods)
	}

	// Personal-data export: provider, e-mail and date, never the subject.
	var job service.ExportView
	acct.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/exports", map[string]any{"password": strongPassword}, &job)
	if err := e.app.Scheduler.RunJob(ctx, app.ExportJob); err != nil {
		t.Fatal(err)
	}
	var link service.DownloadLink
	acct.must(http.StatusOK, http.MethodPost, "/api/v1/me/exports/"+job.ID.String()+"/download-token", map[string]any{"password": strongPassword}, &link)
	r := acct.raw(http.MethodGet, link.URL, nil, nil)
	zr, err := zip.NewReader(bytes.NewReader(r.body), int64(len(r.body)))
	if err != nil {
		t.Fatalf("export: %d %v", r.status, err)
	}
	var account []byte
	for _, zf := range zr.File {
		zf := zf
		rc, _ := zf.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		if bytes.Contains(data, []byte(id.Subject)) {
			t.Errorf("%s leaks the Google subject", zf.Name)
		}
		if zf.Name == "account.json" {
			account = data
		}
	}
	var doc struct {
		Methods []struct {
			Provider string    `json:"provider"`
			Email    string    `json:"email"`
			LinkedAt time.Time `json:"linked_at"`
		} `json:"sign_in_methods"`
	}
	if err := json.Unmarshal(account, &doc); err != nil || len(doc.Methods) != 1 || doc.Methods[0].Provider != "google" ||
		doc.Methods[0].Email != id.Email || doc.Methods[0].LinkedAt.IsZero() {
		t.Fatalf("account.json sign_in_methods: %v %s", err, account)
	}

	// Closing the account deletes the identity: the Google account is free again.
	acct.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/close", map[string]any{"password": strongPassword, "confirmation": "DELETE"}, nil)
	e.clock.Advance(31 * 24 * time.Hour)
	if err := e.app.Scheduler.RunJob(ctx, app.ClosureJob); err != nil {
		t.Fatal(err)
	}
	if len(e.identities(acct.user.ID)) != 0 {
		t.Fatal("erasure must delete the identity rows")
	}
	e.gbrowser().mustResult(f, id, map[string]any{"intent": "signin"}, "signup_consent")
}

func TestGoogleStartRateLimitIsTheAuthBucket(t *testing.T) {
	e, _ := newGoogleEnv(t, func(cfg *config.Config) { cfg.RateLimit.Scale = 1 })
	c := e.gbrowser()
	limited := false
	for i := 0; i < 80 && !limited; i++ {
		limited = c.call(http.MethodPost, "/api/v1/auth/google/start", map[string]any{"intent": "signin"}).status == http.StatusTooManyRequests
	}
	if !limited {
		t.Fatal("POST /auth/google/start must be rate limited like the other auth endpoints")
	}
	// The callback is limited too, but answers a redirect, never JSON.
	var out gcb
	for i := 0; i < 80 && out.result != "google_unavailable"; i++ {
		out = c.callbackGoogle(url.Values{"code": {"x"}, "state": {"y"}})
		if out.status != http.StatusSeeOther {
			t.Fatalf("callback must always redirect: %+v", out)
		}
	}
	if out.result != "google_unavailable" {
		t.Fatalf("a rate-limited callback must redirect with google_unavailable: %+v", out)
	}
}

// Someone who finishes Google's screen after the owner signed out must not
// be able to attach their Google account to that user.
func TestGoogleLinkNeedsTheStartingDeviceToStaySignedIn(t *testing.T) {
	e, f := newGoogleEnv(t)
	acct := e.registerClient()
	b := e.gbrowser()
	b.http.Jar = acct.http.Jar
	b.csrf, b.user = acct.csrf, acct.user
	state, r := b.startGoogle(map[string]any{"intent": "link"})
	if r.status != http.StatusOK {
		t.Fatalf("start: %d %s", r.status, r.body)
	}
	b.must(http.StatusNoContent, http.MethodPost, "/api/v1/auth/logout", nil, nil)
	attacker := gid(uniq("gattacker") + "@gmail.com")
	out := b.callbackGoogle(url.Values{"code": {f.issue(state, attacker, nil)}, "state": {state}})
	if out.result != "google_state_invalid" || hasSessionCookies(out.set) {
		t.Fatalf("callback after sign-out: %+v", out)
	}
	if len(e.identities(acct.user.ID)) != 0 {
		t.Fatal("a Google account was linked to a signed-out user")
	}
	// The same attempt while still signed in works.
	c := e.login(acct.user.Email, strongPassword)
	cb := e.gbrowser()
	cb.http.Jar = c.http.Jar
	cb.csrf, cb.user = c.csrf, c.user
	cb.mustResult(f, attacker, map[string]any{"intent": "link"}, "linked")
}
