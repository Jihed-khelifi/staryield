package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testClientID = "client-123.apps.example"
	testSecret   = "s3cret-value"
	testRedirect = "https://api.example.test/api/v1/auth/google/callback"
	testNonce    = "nonce-abc"
)

// mockIssuer is a local OpenID Connect provider with its own RSA key.
type mockIssuer struct {
	t   *testing.T
	srv *httptest.Server
	key *rsa.PrivateKey
	kid string

	mu   sync.Mutex
	now  time.Time
	hits map[string]int
	// knobs
	cacheControl  string
	tokenStatus   int
	tokenBody     func(idToken string) []byte
	tokenForm     url.Values
	discoveryMut  func(m map[string]any)
	discoveryCode int
	jwksBody      func() []byte
	claims        func(c jwt.MapClaims)
	signer        func(c jwt.MapClaims) string // overrides signing
}

func newKey(t *testing.T, bits int) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func newMock(t *testing.T) *mockIssuer {
	t.Helper()
	m := &mockIssuer{t: t, key: newKey(t, 2048), kid: "kid-1", now: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC), hits: map[string]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		m.hits["discovery"]++
		doc := map[string]any{"issuer": m.srv.URL, "authorization_endpoint": m.srv.URL + "/auth", "token_endpoint": m.srv.URL + "/token",
			"jwks_uri": m.srv.URL + "/jwks"}
		if m.discoveryMut != nil {
			m.discoveryMut(doc)
		}
		code, cc := m.discoveryCode, m.cacheControl
		m.mu.Unlock()
		if code != 0 {
			if code >= 300 && code < 400 {
				w.Header().Set("Location", "https://elsewhere.example/")
			}
			w.WriteHeader(code)
			return
		}
		if cc != "" {
			w.Header().Set("Cache-Control", cc)
		}
		_ = json.NewEncoder(w).Encode(doc)
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		m.hits["jwks"]++
		body, cc := m.jwksBody, m.cacheControl
		m.mu.Unlock()
		if cc != "" {
			w.Header().Set("Cache-Control", cc)
		}
		if body != nil {
			_, _ = w.Write(body())
			return
		}
		_, _ = w.Write(m.jwks(m.kid, &m.key.PublicKey))
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		m.mu.Lock()
		m.hits["token"]++
		m.tokenForm = r.PostForm
		status, bodyFn := m.tokenStatus, m.tokenBody
		m.mu.Unlock()
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if status >= 300 && status < 400 {
			w.Header().Set("Location", "https://elsewhere.example/")
		}
		if status != 0 && status != http.StatusOK {
			w.WriteHeader(status)
			if bodyFn != nil {
				_, _ = w.Write(bodyFn(""))
			}
			return
		}
		id := m.sign(nil)
		if bodyFn != nil {
			_, _ = w.Write(bodyFn(id))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at-secret", "token_type": "Bearer", "id_token": id})
	})
	m.srv = httptest.NewServer(mux)
	t.Cleanup(m.srv.Close)
	return m
}

func (m *mockIssuer) jwks(kid string, pub *rsa.PublicKey) []byte {
	b, _ := json.Marshal(map[string]any{"keys": []map[string]any{{"kty": "RSA", "kid": kid, "use": "sig", "alg": "RS256",
		"n": base64.RawURLEncoding.EncodeToString(pub.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1})}}})
	return b
}

func (m *mockIssuer) hit(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.hits[name]
}

func (m *mockIssuer) clock() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.now
}

func (m *mockIssuer) advance(d time.Duration) {
	m.mu.Lock()
	m.now = m.now.Add(d)
	m.mu.Unlock()
}

// baseClaims are the claims of a valid ID token.
func (m *mockIssuer) baseClaims() jwt.MapClaims {
	now := m.clock()
	return jwt.MapClaims{"iss": m.srv.URL, "aud": testClientID, "azp": testClientID, "sub": "1234567890", "nonce": testNonce,
		"email": "Person@Example.com", "email_verified": true, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix()}
}

// sign issues an ID token with the current knobs (extra claims applied last).
func (m *mockIssuer) sign(mod func(jwt.MapClaims)) string {
	c := m.baseClaims()
	m.mu.Lock()
	fn, signer := m.claims, m.signer
	m.mu.Unlock()
	if fn != nil {
		fn(c)
	}
	if mod != nil {
		mod(c)
	}
	if signer != nil {
		return signer(c)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, c)
	tok.Header["kid"] = m.kid
	s, err := tok.SignedString(m.key)
	if err != nil {
		m.t.Fatal(err)
	}
	return s
}

func (m *mockIssuer) client(opts ...func(*Config)) *Client {
	m.t.Helper()
	cfg := Config{Issuer: m.srv.URL, ClientID: testClientID, ClientSecret: testSecret, RedirectURL: testRedirect,
		AllowLoopbackHTTP: true, Now: m.clock}
	for _, o := range opts {
		o(&cfg)
	}
	c, err := NewClient(cfg)
	if err != nil {
		m.t.Fatal(err)
	}
	return c
}

func exchange(c *Client) (*Identity, error) {
	return c.Exchange(context.Background(), ExchangeParams{Code: "code-1", CodeVerifier: "verifier-1", Nonce: testNonce})
}

func TestValidFlow(t *testing.T) {
	m := newMock(t)
	c := m.client()

	verifier, err := NewVerifier()
	if err != nil || len(verifier) != 43 {
		t.Fatalf("verifier %q %v", verifier, err)
	}
	raw, err := c.AuthorizationURL(context.Background(), AuthParams{State: "st", Nonce: testNonce, CodeChallenge: Challenge(verifier), Locale: "fr"})
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	q := u.Query()
	if u.Scheme+"://"+u.Host+u.Path != m.srv.URL+"/auth" {
		t.Errorf("authorization endpoint: %s", raw)
	}
	want := map[string]string{"response_type": "code", "client_id": testClientID, "redirect_uri": testRedirect, "scope": "openid email",
		"state": "st", "nonce": testNonce, "code_challenge": Challenge(verifier), "code_challenge_method": "S256",
		"prompt": "select_account", "hl": "fr"}
	for k, v := range want {
		if q.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, q.Get(k), v)
		}
	}
	if q.Has("access_type") || q.Has("include_granted_scopes") {
		t.Errorf("online access only: %s", raw)
	}

	id, err := c.Exchange(context.Background(), ExchangeParams{Code: "the-code", CodeVerifier: verifier, Nonce: testNonce})
	if err != nil {
		t.Fatal(err)
	}
	if id.Subject != "1234567890" || id.Email != "person@example.com" {
		t.Fatalf("identity %+v", id)
	}
	form := m.tokenForm
	for k, v := range map[string]string{"grant_type": "authorization_code", "code": "the-code", "client_id": testClientID,
		"client_secret": testSecret, "redirect_uri": testRedirect, "code_verifier": verifier} {
		if form.Get(k) != v {
			t.Errorf("token form %s = %q, want %q", k, form.Get(k), v)
		}
	}
	if len(form) != 6 {
		t.Errorf("unexpected token form fields: %v", form)
	}
}

func TestIDTokenValidation(t *testing.T) {
	other := newKey(t, 2048)
	m := newMock(t)
	hs256 := func(c jwt.MapClaims) string {
		// Algorithm confusion: an HMAC token keyed with the public modulus.
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
		tok.Header["kid"] = m.kid
		s, _ := tok.SignedString(m.key.PublicKey.N.Bytes())
		return s
	}
	none := func(c jwt.MapClaims) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodNone, c)
		tok.Header["kid"] = m.kid
		s, _ := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
		return s
	}
	signedBy := func(k *rsa.PrivateKey, kid string) func(jwt.MapClaims) string {
		return func(c jwt.MapClaims) string {
			tok := jwt.NewWithClaims(jwt.SigningMethodRS256, c)
			if kid != "" {
				tok.Header["kid"] = kid
			}
			s, _ := tok.SignedString(k)
			return s
		}
	}
	cases := []struct {
		name   string
		mod    func(jwt.MapClaims)
		signer func(jwt.MapClaims) string
		want   error
	}{
		{name: "wrong audience", mod: func(c jwt.MapClaims) { c["aud"] = "someone-else"; delete(c, "azp") }, want: ErrInvalidToken},
		{name: "audience list without us", mod: func(c jwt.MapClaims) { c["aud"] = []string{"a", "b"} }, want: ErrInvalidToken},
		{name: "several audiences without azp", mod: func(c jwt.MapClaims) { c["aud"] = []string{testClientID, "b"}; delete(c, "azp") }, want: ErrInvalidToken},
		{name: "wrong issuer", mod: func(c jwt.MapClaims) { c["iss"] = "https://evil.example" }, want: ErrInvalidToken},
		{name: "google issuer is not ours", mod: func(c jwt.MapClaims) { c["iss"] = "accounts.google.com" }, want: ErrInvalidToken},
		{name: "missing issuer", mod: func(c jwt.MapClaims) { delete(c, "iss") }, want: ErrInvalidToken},
		{name: "expired", mod: func(c jwt.MapClaims) { c["exp"] = m.clock().Add(-2 * time.Minute).Unix() }, want: ErrInvalidToken},
		{name: "missing exp", mod: func(c jwt.MapClaims) { delete(c, "exp") }, want: ErrInvalidToken},
		{name: "issued in the future", mod: func(c jwt.MapClaims) { c["iat"] = m.clock().Add(5 * time.Minute).Unix() }, want: ErrInvalidToken},
		{name: "missing iat", mod: func(c jwt.MapClaims) { delete(c, "iat") }, want: ErrInvalidToken},
		{name: "wrong azp", mod: func(c jwt.MapClaims) { c["azp"] = "other-client" }, want: ErrInvalidToken},
		{name: "wrong nonce", mod: func(c jwt.MapClaims) { c["nonce"] = "another" }, want: ErrInvalidToken},
		{name: "missing nonce", mod: func(c jwt.MapClaims) { delete(c, "nonce") }, want: ErrInvalidToken},
		{name: "empty subject", mod: func(c jwt.MapClaims) { c["sub"] = "" }, want: ErrInvalidToken},
		{name: "subject too long", mod: func(c jwt.MapClaims) { c["sub"] = strings.Repeat("9", 256) }, want: ErrInvalidToken},
		{name: "missing email", mod: func(c jwt.MapClaims) { delete(c, "email") }, want: ErrInvalidToken},
		{name: "email unverified", mod: func(c jwt.MapClaims) { c["email_verified"] = false }, want: ErrEmailUnverified},
		{name: "email_verified missing", mod: func(c jwt.MapClaims) { delete(c, "email_verified") }, want: ErrEmailUnverified},
		{name: "email_verified string false", mod: func(c jwt.MapClaims) { c["email_verified"] = "false" }, want: ErrEmailUnverified},
		{name: "email_verified number", mod: func(c jwt.MapClaims) { c["email_verified"] = 1 }, want: ErrEmailUnverified},
		{name: "signature by another key", signer: signedBy(other, "kid-1"), want: ErrInvalidToken},
		{name: "unsigned (alg none)", signer: none, want: ErrInvalidToken},
		{name: "HS256 key confusion", signer: hs256, want: ErrInvalidToken},
		{name: "no kid", signer: signedBy(m.key, ""), want: ErrInvalidToken},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m.mu.Lock()
			m.claims, m.signer = tc.mod, tc.signer
			m.mu.Unlock()
			id, err := exchange(m.client())
			if !errors.Is(err, tc.want) || id != nil {
				t.Fatalf("want %v, got %v (%+v)", tc.want, err, id)
			}
		})
	}

	t.Run("a verified e-mail sent as the string true is accepted", func(t *testing.T) {
		m.mu.Lock()
		m.claims, m.signer = func(c jwt.MapClaims) { c["email_verified"] = "true" }, nil
		m.mu.Unlock()
		if id, err := exchange(m.client()); err != nil || id.Email != "person@example.com" {
			t.Fatalf("%v %+v", err, id)
		}
	})
	t.Run("expiry within the 60 s leeway is tolerated", func(t *testing.T) {
		m.mu.Lock()
		m.claims, m.signer = func(c jwt.MapClaims) { c["exp"] = m.clock().Add(-30 * time.Second).Unix() }, nil
		m.mu.Unlock()
		if _, err := exchange(m.client()); err != nil {
			t.Fatalf("leeway: %v", err)
		}
	})
	t.Run("audience list containing us with azp is accepted", func(t *testing.T) {
		m.mu.Lock()
		m.claims, m.signer = func(c jwt.MapClaims) { c["aud"] = []string{"b", testClientID} }, nil
		m.mu.Unlock()
		if _, err := exchange(m.client()); err != nil {
			t.Fatalf("%v", err)
		}
	})
	t.Run("a token for another nonce cannot be replayed", func(t *testing.T) {
		m.mu.Lock()
		m.claims, m.signer = nil, nil
		m.mu.Unlock()
		_, err := m.client().Exchange(context.Background(), ExchangeParams{Code: "c", CodeVerifier: "v", Nonce: "different"})
		if !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("%v", err)
		}
	})
}

func TestGoogleIssuerForms(t *testing.T) {
	c := &Client{cfg: Config{Issuer: GoogleIssuer}}
	for iss, want := range map[string]bool{"https://accounts.google.com": true, "accounts.google.com": true,
		"https://accounts.google.com/": false, "http://accounts.google.com": false, "https://evil.example": false, "": false} {
		if got := c.issuerAllowed(iss); got != want {
			t.Errorf("issuer %q: %v want %v", iss, got, want)
		}
	}
	custom := &Client{cfg: Config{Issuer: "http://127.0.0.1:9"}}
	if custom.issuerAllowed("accounts.google.com") || !custom.issuerAllowed("http://127.0.0.1:9") {
		t.Error("a test issuer accepts exactly itself")
	}
}

func TestUnknownKidRefreshesOnceAndIsRateLimited(t *testing.T) {
	m := newMock(t)
	c := m.client()
	if _, err := exchange(c); err != nil {
		t.Fatal(err)
	}
	if m.hit("jwks") != 1 {
		t.Fatalf("jwks hits %d", m.hit("jwks"))
	}

	// The provider rotates its key; the cached set does not know the new kid.
	rotated := newKey(t, 2048)
	m.mu.Lock()
	m.key, m.kid = rotated, "kid-2"
	m.mu.Unlock()
	if _, err := exchange(c); err != nil {
		t.Fatalf("rotation should be picked up by a forced refresh: %v", err)
	}
	if m.hit("jwks") != 2 {
		t.Fatalf("expected one forced refresh, jwks hits %d", m.hit("jwks"))
	}

	// A token naming a key nobody has: one refresh is allowed, then the rate
	// limit holds for a minute.
	m.mu.Lock()
	m.signer = func(cl jwt.MapClaims) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, cl)
		tok.Header["kid"] = "ghost"
		s, _ := tok.SignedString(rotated)
		return s
	}
	m.mu.Unlock()
	for i := 0; i < 3; i++ {
		if _, err := exchange(c); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("ghost kid: %v", err)
		}
	}
	if m.hit("jwks") != 2 {
		t.Fatalf("forced refreshes must be limited to one a minute, jwks hits %d", m.hit("jwks"))
	}
	m.advance(61 * time.Second)
	if _, err := exchange(c); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("ghost kid after a minute: %v", err)
	}
	if m.hit("jwks") != 3 {
		t.Fatalf("one more refresh once a minute passed, jwks hits %d", m.hit("jwks"))
	}
}

func TestDiscoveryAndKeysAreCachedPerCacheControl(t *testing.T) {
	m := newMock(t)
	m.cacheControl = "public, max-age=3600"
	c := m.client()
	for i := 0; i < 3; i++ {
		if _, err := exchange(c); err != nil {
			t.Fatal(err)
		}
	}
	if m.hit("discovery") != 1 || m.hit("jwks") != 1 || m.hit("token") != 3 {
		t.Fatalf("hits discovery=%d jwks=%d token=%d", m.hit("discovery"), m.hit("jwks"), m.hit("token"))
	}
	m.advance(59 * time.Minute)
	if _, err := exchange(c); err != nil || m.hit("jwks") != 1 {
		t.Fatalf("still cached: %v jwks=%d", err, m.hit("jwks"))
	}
	m.advance(2 * time.Minute)
	if _, err := exchange(c); err != nil || m.hit("jwks") != 2 || m.hit("discovery") != 2 {
		t.Fatalf("expired: %v jwks=%d discovery=%d", err, m.hit("jwks"), m.hit("discovery"))
	}
}

func TestCacheLifetimeIsClamped(t *testing.T) {
	for cc, want := range map[string]time.Duration{
		"":                         5 * time.Minute,
		"no-store":                 5 * time.Minute,
		"max-age=0":                5 * time.Minute,
		"max-age=60":               5 * time.Minute,
		"max-age=1800":             30 * time.Minute,
		"public, max-age=86400":    24 * time.Hour,
		"max-age=99999999999":      24 * time.Hour,
		"max-age=abc":              5 * time.Minute,
		"PUBLIC, Max-Age=7200, x=": 2 * time.Hour,
	} {
		h := http.Header{}
		if cc != "" {
			h.Set("Cache-Control", cc)
		}
		if got := ttl(h); got != want {
			t.Errorf("%q: %v want %v", cc, got, want)
		}
	}
}

func TestOversizedResponsesAreRefused(t *testing.T) {
	huge := []byte(strings.Repeat("a", MaxBody+10))
	t.Run("token endpoint", func(t *testing.T) {
		m := newMock(t)
		m.tokenBody = func(string) []byte { return huge }
		if _, err := exchange(m.client()); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("%v", err)
		}
	})
	t.Run("jwks", func(t *testing.T) {
		m := newMock(t)
		m.jwksBody = func() []byte { return huge }
		if _, err := exchange(m.client()); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("%v", err)
		}
	})
	t.Run("exactly at the cap is read", func(t *testing.T) {
		m := newMock(t)
		pad := strings.Repeat(" ", MaxBody)
		m.tokenBody = func(id string) []byte {
			b, _ := json.Marshal(map[string]any{"id_token": id})
			return append(b, pad[:MaxBody-len(b)]...)
		}
		if _, err := exchange(m.client()); err != nil {
			t.Fatalf("%v", err)
		}
	})
}

func TestRedirectsAreNeverFollowed(t *testing.T) {
	t.Run("discovery", func(t *testing.T) {
		m := newMock(t)
		m.discoveryCode = http.StatusFound
		if _, err := exchange(m.client()); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("%v", err)
		}
	})
	t.Run("token endpoint", func(t *testing.T) {
		m := newMock(t)
		m.tokenStatus = http.StatusFound
		if _, err := exchange(m.client()); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("%v", err)
		}
	})
	t.Run("a custom client cannot re-enable redirects", func(t *testing.T) {
		hops := 0
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hops++ }))
		defer target.Close()
		src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.URL, http.StatusFound)
		}))
		defer src.Close()
		c, err := NewClient(Config{Issuer: src.URL, ClientID: "a", ClientSecret: "b", RedirectURL: testRedirect, AllowLoopbackHTTP: true,
			HTTPClient: &http.Client{Timeout: time.Hour}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.AuthorizationURL(context.Background(), AuthParams{}); !errors.Is(err, ErrUnavailable) || hops != 0 {
			t.Fatalf("err=%v hops=%d", err, hops)
		}
		if c.hc.Timeout != HTTPTimeout {
			t.Fatalf("timeout %v", c.hc.Timeout)
		}
	})
}

func TestInsecureDiscoveryEndpointsAreRefused(t *testing.T) {
	for _, field := range []string{"authorization_endpoint", "token_endpoint", "jwks_uri"} {
		t.Run(field+" over http to a public host", func(t *testing.T) {
			m := newMock(t)
			m.discoveryMut = func(d map[string]any) { d[field] = "http://accounts.evil.example/x" }
			if _, err := exchange(m.client()); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("%v", err)
			}
			if _, err := m.client().AuthorizationURL(context.Background(), AuthParams{}); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("authorization url: %v", err)
			}
		})
	}
	t.Run("loopback http needs the test switch", func(t *testing.T) {
		m := newMock(t)
		if _, err := NewClient(Config{Issuer: m.srv.URL, ClientID: "a", ClientSecret: "b", RedirectURL: testRedirect}); err == nil {
			t.Fatal("an http issuer must be refused by default")
		}
	})
	t.Run("endpoints must be absolute and credential free", func(t *testing.T) {
		for _, ep := range []string{"/relative", "https://user:pw@host.example/x", "ftp://host.example/x", "https://host.example/x#frag", ""} {
			m := newMock(t)
			m.discoveryMut = func(d map[string]any) { d["token_endpoint"] = ep }
			if _, err := exchange(m.client()); !errors.Is(err, ErrUnavailable) {
				t.Errorf("%q: %v", ep, err)
			}
		}
	})
	t.Run("the issuer must match the document", func(t *testing.T) {
		m := newMock(t)
		m.discoveryMut = func(d map[string]any) { d["issuer"] = "https://other.example" }
		if _, err := exchange(m.client()); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("%v", err)
		}
	})
}

func TestTokenEndpointFailures(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"invalid_grant", http.StatusBadRequest, `{"error":"invalid_grant"}`, ErrInvalidGrant},
		{"invalid_client", http.StatusUnauthorized, `{"error":"invalid_client"}`, ErrInvalidGrant},
		{"server error", http.StatusInternalServerError, ``, ErrUnavailable},
		{"rate limited", http.StatusTooManyRequests, ``, ErrUnavailable},
		{"forbidden", http.StatusForbidden, ``, ErrUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newMock(t)
			m.tokenStatus = tc.status
			m.tokenBody = func(string) []byte { return []byte(tc.body) }
			if _, err := exchange(m.client()); !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
	t.Run("no id_token", func(t *testing.T) {
		m := newMock(t)
		m.tokenBody = func(string) []byte { return []byte(`{"access_token":"x"}`) }
		if _, err := exchange(m.client()); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("%v", err)
		}
	})
	t.Run("empty code or verifier never reaches the provider", func(t *testing.T) {
		m := newMock(t)
		c := m.client()
		for _, p := range []ExchangeParams{{CodeVerifier: "v", Nonce: "n"}, {Code: "c", Nonce: "n"}, {Code: "c", CodeVerifier: "v"},
			{Code: strings.Repeat("x", 3000), CodeVerifier: "v", Nonce: "n"}} {
			if _, err := c.Exchange(context.Background(), p); !errors.Is(err, ErrInvalidGrant) {
				t.Fatalf("%v", err)
			}
		}
		if m.hit("token") != 0 {
			t.Fatal("the token endpoint must not be called")
		}
	})
	t.Run("an unreachable provider is unavailable", func(t *testing.T) {
		m := newMock(t)
		c := m.client()
		m.srv.Close()
		if _, err := exchange(c); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("%v", err)
		}
	})
}

func TestWeakOrUnusableKeysAreIgnored(t *testing.T) {
	t.Run("1024-bit key", func(t *testing.T) {
		m := newMock(t)
		weak := newKey(t, 1024)
		m.jwksBody = func() []byte { return m.jwks(m.kid, &weak.PublicKey) }
		if _, err := exchange(m.client()); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("%v", err)
		}
	})
	t.Run("non-RSA and encryption keys", func(t *testing.T) {
		m := newMock(t)
		m.jwksBody = func() []byte {
			return []byte(`{"keys":[{"kty":"EC","kid":"kid-1","crv":"P-256"},{"kty":"RSA","kid":"kid-1","use":"enc","n":"AQAB","e":"AQAB"}]}`)
		}
		if _, err := exchange(m.client()); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("%v", err)
		}
	})
}

func TestPKCE(t *testing.T) {
	// RFC 7636 appendix B.
	if got := Challenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"); got != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Fatalf("challenge %q", got)
	}
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		v, err := NewVerifier()
		if err != nil || len(v) != 43 || seen[v] || strings.ContainsAny(v, "+/=") {
			t.Fatalf("verifier %q %v", v, err)
		}
		seen[v] = true
	}
}

func TestHardenedHTTPClient(t *testing.T) {
	c := NewHTTPClient()
	if c.Timeout != 10*time.Second || c.CheckRedirect == nil {
		t.Fatalf("%+v", c)
	}
	if err := c.CheckRedirect(nil, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("redirects must not be followed: %v", err)
	}
	if _, err := NewClient(Config{Issuer: "https://accounts.google.com"}); err == nil {
		t.Fatal("credentials are required")
	}
	if _, err := NewClient(Config{Issuer: "http://accounts.google.com", ClientID: "a", ClientSecret: "b", RedirectURL: "x", AllowLoopbackHTTP: true}); err == nil {
		t.Fatal("a public http issuer is refused even with the loopback switch")
	}
	_ = fmt.Sprint()
}
