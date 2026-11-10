// Package oidc is a small, strict OpenID Connect relying party for the
// authorization-code flow with PKCE (revisions 4.0: Sign in with Google,
// docs/platform-extensions.md §23.1).
//
// It discovers the provider's endpoints, builds the authorization URL,
// exchanges the code (client_secret_post) and validates the ID token: RS256
// only, key by kid from the provider's JWKS, issuer, audience, authorized
// party, expiry, nonce, subject and a verified e-mail. The access token is
// discarded and no refresh token is ever requested.
//
// The outbound HTTP client has a 10 s timeout, follows no redirects and
// caps response bodies at 1 MiB. Endpoints named by the discovery document
// must be https (http only for a loopback test issuer).
package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GoogleIssuer is Google's issuer. Its ID tokens carry iss either as this
// URL or as the bare host.
const GoogleIssuer = "https://accounts.google.com"

const (
	// MaxBody caps every provider response.
	MaxBody = 1 << 20
	// HTTPTimeout bounds every provider request.
	HTTPTimeout = 10 * time.Second

	minCache   = 5 * time.Minute
	maxCache   = 24 * time.Hour
	forceEvery = time.Minute
	leeway     = 60 * time.Second
	minRSABits = 2048
)

// Errors returned by providers. Callers map them to sign-in results.
var (
	// ErrUnavailable: the provider could not be reached or answered
	// nonsense (network error, 5xx, oversized body, bad discovery document).
	ErrUnavailable = errors.New("oidc: provider unavailable")
	// ErrInvalidGrant: the provider refused the authorization code.
	ErrInvalidGrant = errors.New("oidc: authorization code rejected")
	// ErrInvalidToken: the ID token failed validation.
	ErrInvalidToken = errors.New("oidc: invalid id token")
	// ErrEmailUnverified: the provider does not vouch for the e-mail.
	ErrEmailUnverified = errors.New("oidc: email not verified")
)

// AuthParams are the per-attempt values of an authorization request.
type AuthParams struct {
	State         string
	Nonce         string
	CodeChallenge string // PKCE S256
	Locale        string // hl=
}

// ExchangeParams are the values needed to redeem an authorization code.
type ExchangeParams struct {
	Code         string
	CodeVerifier string
	// Nonce is the value sent in the authorization request; the ID token
	// must carry exactly it.
	Nonce string
}

// Identity is what the provider vouches for.
type Identity struct {
	// Subject is the provider's stable account id. It, never the e-mail,
	// identifies the person.
	Subject string
	// Email is lower-case and always verified when returned.
	Email string
}

// Provider is an OpenID Connect provider as the sign-in service uses it.
// Tests inject fakes through app.Overrides.
type Provider interface {
	// AuthorizationURL returns the URL to send the browser to.
	AuthorizationURL(ctx context.Context, p AuthParams) (string, error)
	// Exchange redeems the code and returns the validated identity.
	Exchange(ctx context.Context, p ExchangeParams) (*Identity, error)
}

// Config configures a Client.
type Config struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	// AllowLoopbackHTTP accepts http endpoints on loopback hosts (a local
	// mock issuer in tests). Never set it for the real Google.
	AllowLoopbackHTTP bool
	// HTTPClient overrides the hardened default (tests only).
	HTTPClient *http.Client
	// Now overrides the clock (tests).
	Now func() time.Time
}

// NewHTTPClient returns the hardened outbound client: 10 s timeout, no
// redirects, TLS 1.2 or newer.
func NewHTTPClient() *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	return &http.Client{
		Timeout:   HTTPTimeout,
		Transport: tr,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// Client is the standard Provider.
type Client struct {
	cfg Config
	hc  *http.Client
	now func() time.Time

	mu         sync.Mutex
	disc       *discovery
	discExpiry time.Time
	keys       map[string]*rsa.PublicKey
	keysExpiry time.Time
	lastForced time.Time
}

// discovery is the part of the discovery document the client uses.
type discovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

// NewClient returns a Client. HTTPClient defaults to NewHTTPClient; a
// supplied client still has its redirect policy replaced by "follow none".
func NewClient(cfg Config) (*Client, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RedirectURL == "" {
		return nil, errors.New("oidc: client id, secret and redirect URL are required")
	}
	cfg.Issuer = strings.TrimSuffix(cfg.Issuer, "/")
	if err := checkEndpoint(cfg.Issuer, cfg.AllowLoopbackHTTP); err != nil {
		return nil, fmt.Errorf("oidc: issuer: %w", err)
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = NewHTTPClient()
	} else {
		cp := *hc
		cp.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		if cp.Timeout == 0 || cp.Timeout > HTTPTimeout {
			cp.Timeout = HTTPTimeout
		}
		hc = &cp
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Client{cfg: cfg, hc: hc, now: now}, nil
}

// checkEndpoint requires an absolute https URL (http only for loopback when
// allowed) without credentials or fragment.
func checkEndpoint(raw string, allowLoopbackHTTP bool) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" {
		return fmt.Errorf("%q is not an absolute URL", raw)
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		if allowLoopbackHTTP && isLoopback(u.Hostname()) {
			return nil
		}
	}
	return fmt.Errorf("%q must use https", raw)
}

func isLoopback(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ---------------------------------------------------------------------------
// PKCE and random values
// ---------------------------------------------------------------------------

// RandomToken returns n CSPRNG bytes, base64url-encoded without padding.
func RandomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("oidc: random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// NewVerifier returns a PKCE code verifier: 32 random bytes, 43 characters.
func NewVerifier() (string, error) { return RandomToken(32) }

// Challenge returns the S256 code challenge of a verifier.
func Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

// ttl derives a cache lifetime from Cache-Control, clamped to [5 min, 24 h].
func ttl(h http.Header) time.Duration {
	d := minCache
	for _, part := range strings.Split(h.Get("Cache-Control"), ",") {
		part = strings.ToLower(strings.TrimSpace(part))
		if v, ok := strings.CutPrefix(part, "max-age="); ok {
			if n, err := strconv.ParseInt(strings.Trim(v, `"`), 10, 64); err == nil && n > 0 {
				if n > int64(maxCache/time.Second) {
					return maxCache
				}
				d = time.Duration(n) * time.Second
			}
		}
	}
	return max(d, minCache)
}

// do sends a request and returns the status, headers and a body capped at
// MaxBody (anything larger is ErrUnavailable).
func (c *Client) do(req *http.Request) (int, http.Header, []byte, error) {
	res, err := c.hc.Do(req)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("%w: %s %s: %v", ErrUnavailable, req.Method, req.URL.Host, redactErr(err))
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, MaxBody+1))
	if err != nil {
		return 0, nil, nil, fmt.Errorf("%w: read response: %v", ErrUnavailable, redactErr(err))
	}
	if len(body) > MaxBody {
		return 0, nil, nil, fmt.Errorf("%w: response larger than %d bytes", ErrUnavailable, MaxBody)
	}
	return res.StatusCode, res.Header, body, nil
}

// redactErr drops the URL (which could carry a code) from a transport error.
func redactErr(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

func (c *Client) getJSON(ctx context.Context, rawURL string, out any) (http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	req.Header.Set("Accept", "application/json")
	status, hdr, body, err := c.do(req)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("%w: GET %s answered %d", ErrUnavailable, req.URL.Host, status)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return nil, fmt.Errorf("%w: unreadable document from %s", ErrUnavailable, req.URL.Host)
	}
	return hdr, nil
}

// ---------------------------------------------------------------------------
// Discovery and JWKS
// ---------------------------------------------------------------------------

func (c *Client) discovery(ctx context.Context) (*discovery, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.disc != nil && c.now().Before(c.discExpiry) {
		return c.disc, nil
	}
	var d discovery
	hdr, err := c.getJSON(ctx, c.cfg.Issuer+"/.well-known/openid-configuration", &d)
	if err != nil {
		return nil, err
	}
	if strings.TrimSuffix(d.Issuer, "/") != c.cfg.Issuer {
		return nil, fmt.Errorf("%w: discovery issuer %q does not match", ErrUnavailable, d.Issuer)
	}
	for name, ep := range map[string]string{"authorization_endpoint": d.AuthorizationEndpoint,
		"token_endpoint": d.TokenEndpoint, "jwks_uri": d.JWKSURI} {
		if err := checkEndpoint(ep, c.cfg.AllowLoopbackHTTP); err != nil {
			return nil, fmt.Errorf("%w: discovery %s: %v", ErrUnavailable, name, err)
		}
	}
	c.disc, c.discExpiry = &d, c.now().Add(ttl(hdr))
	return c.disc, nil
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (k jwk) publicKey() (*rsa.PublicKey, bool) {
	if k.Kty != "RSA" || k.Kid == "" || (k.Use != "" && k.Use != "sig") || (k.Alg != "" && k.Alg != "RS256") {
		return nil, false
	}
	n, err1 := base64.RawURLEncoding.DecodeString(strings.TrimRight(k.N, "="))
	e, err2 := base64.RawURLEncoding.DecodeString(strings.TrimRight(k.E, "="))
	if err1 != nil || err2 != nil || len(n) == 0 || len(e) == 0 || len(e) > 4 {
		return nil, false
	}
	exp := new(big.Int).SetBytes(e)
	if !exp.IsInt64() || exp.Int64() < 3 || exp.Int64() > 1<<31-1 {
		return nil, false
	}
	pub := &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(exp.Int64())}
	if pub.N.BitLen() < minRSABits {
		return nil, false
	}
	return pub, true
}

// keySet returns the cached JWKS, fetching it when expired or when force is
// set. fetched reports that this call hit the network.
func (c *Client) keySet(ctx context.Context, force bool) (keys map[string]*rsa.PublicKey, fetched bool, err error) {
	d, err := c.discovery(ctx)
	if err != nil {
		return nil, false, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && c.keys != nil && c.now().Before(c.keysExpiry) {
		return c.keys, false, nil
	}
	var doc struct {
		Keys []jwk `json:"keys"`
	}
	hdr, err := c.getJSON(ctx, d.JWKSURI, &doc)
	if err != nil {
		return nil, false, err
	}
	next := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if pub, ok := k.publicKey(); ok {
			next[k.Kid] = pub
		}
	}
	if len(next) == 0 {
		return nil, false, fmt.Errorf("%w: the key set holds no usable RSA key", ErrUnavailable)
	}
	c.keys, c.keysExpiry = next, c.now().Add(ttl(hdr))
	return next, true, nil
}

// key returns the verification key for kid. An unknown kid forces one
// refresh of the key set (the provider may have rotated), at most once a
// minute.
func (c *Client) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	keys, fetched, err := c.keySet(ctx, false)
	if err != nil {
		return nil, err
	}
	if k := keys[kid]; k != nil {
		return k, nil
	}
	if fetched {
		return nil, fmt.Errorf("%w: unknown key id", ErrInvalidToken)
	}
	c.mu.Lock()
	if now := c.now(); now.Sub(c.lastForced) < forceEvery {
		c.mu.Unlock()
		return nil, fmt.Errorf("%w: unknown key id", ErrInvalidToken)
	} else {
		c.lastForced = now
	}
	c.mu.Unlock()
	keys, _, err = c.keySet(ctx, true)
	if err != nil {
		return nil, err
	}
	if k := keys[kid]; k != nil {
		return k, nil
	}
	return nil, fmt.Errorf("%w: unknown key id", ErrInvalidToken)
}

// ---------------------------------------------------------------------------
// Authorization request and code exchange
// ---------------------------------------------------------------------------

// AuthorizationURL implements Provider.
func (c *Client) AuthorizationURL(ctx context.Context, p AuthParams) (string, error) {
	d, err := c.discovery(ctx)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(d.AuthorizationEndpoint)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", c.cfg.ClientID)
	q.Set("redirect_uri", c.cfg.RedirectURL)
	q.Set("scope", "openid email")
	q.Set("state", p.State)
	q.Set("nonce", p.Nonce)
	q.Set("code_challenge", p.CodeChallenge)
	q.Set("code_challenge_method", "S256")
	q.Set("prompt", "select_account")
	if p.Locale != "" {
		q.Set("hl", p.Locale)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// Exchange implements Provider.
func (c *Client) Exchange(ctx context.Context, p ExchangeParams) (*Identity, error) {
	if p.Code == "" || len(p.Code) > 2048 || p.CodeVerifier == "" || p.Nonce == "" {
		return nil, fmt.Errorf("%w: missing code, verifier or nonce", ErrInvalidGrant)
	}
	d, err := c.discovery(ctx)
	if err != nil {
		return nil, err
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {p.Code},
		"client_id":     {c.cfg.ClientID},
		"client_secret": {c.cfg.ClientSecret},
		"redirect_uri":  {c.cfg.RedirectURL},
		"code_verifier": {p.CodeVerifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	status, _, body, err := c.do(req)
	if err != nil {
		return nil, err
	}
	var tok struct {
		IDToken string `json:"id_token"`
		Error   string `json:"error"`
	}
	_ = json.Unmarshal(body, &tok)
	switch {
	case status == http.StatusOK:
	case status == http.StatusBadRequest || status == http.StatusUnauthorized:
		// invalid_grant: expired, reused or mismatching code. invalid_client
		// means our own configuration is wrong; both end the attempt.
		return nil, fmt.Errorf("%w: token endpoint answered %d %q", ErrInvalidGrant, status, safeCode(tok.Error))
	default:
		return nil, fmt.Errorf("%w: token endpoint answered %d", ErrUnavailable, status)
	}
	// The access token (if any) was never decoded and is dropped here.
	if tok.IDToken == "" || len(tok.IDToken) > 16<<10 {
		return nil, fmt.Errorf("%w: no id_token in the token response", ErrInvalidToken)
	}
	return c.verify(ctx, tok.IDToken, p.Nonce)
}

// safeCode keeps a provider error code short and printable for logs.
func safeCode(s string) string {
	if len(s) > 40 {
		s = s[:40]
	}
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r > 0x7e {
			return '?'
		}
		return r
	}, s)
}

// ---------------------------------------------------------------------------
// ID token validation
// ---------------------------------------------------------------------------

type idClaims struct {
	jwt.RegisteredClaims
	AuthorizedParty string `json:"azp"`
	Nonce           string `json:"nonce"`
	Email           string `json:"email"`
	EmailVerified   any    `json:"email_verified"`
}

func (c *Client) issuerAllowed(iss string) bool {
	if c.cfg.Issuer == GoogleIssuer {
		return iss == GoogleIssuer || iss == "accounts.google.com"
	}
	return iss == c.cfg.Issuer
}

// verify validates a raw ID token against the expected nonce.
func (c *Client) verify(ctx context.Context, raw, nonce string) (*Identity, error) {
	var claims idClaims
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithLeeway(leeway),
		jwt.WithTimeFunc(c.now),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithAudience(c.cfg.ClientID),
	)
	var keyErr error
	_, err := parser.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != "RS256" {
			return nil, errors.New("unexpected signing algorithm")
		}
		kid, _ := t.Header["kid"].(string)
		if kid == "" || len(kid) > 256 {
			return nil, errors.New("missing key id")
		}
		k, err := c.key(ctx, kid)
		if err != nil {
			keyErr = err
			return nil, err
		}
		return k, nil
	})
	if err != nil {
		// A key fetch that failed because the provider is down is not a bad
		// token: keep that distinction for the caller.
		if keyErr != nil && errors.Is(keyErr, ErrUnavailable) {
			return nil, keyErr
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if !c.issuerAllowed(claims.Issuer) {
		return nil, fmt.Errorf("%w: unexpected issuer", ErrInvalidToken)
	}
	if claims.IssuedAt == nil {
		return nil, fmt.Errorf("%w: missing iat", ErrInvalidToken)
	}
	if claims.AuthorizedParty != "" && claims.AuthorizedParty != c.cfg.ClientID {
		return nil, fmt.Errorf("%w: unexpected authorized party", ErrInvalidToken)
	}
	if len(claims.Audience) > 1 && claims.AuthorizedParty != c.cfg.ClientID {
		return nil, fmt.Errorf("%w: several audiences without azp", ErrInvalidToken)
	}
	if claims.Nonce == "" || nonce == "" || subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(nonce)) != 1 {
		return nil, fmt.Errorf("%w: nonce mismatch", ErrInvalidToken)
	}
	if n := len(claims.Subject); n < 1 || n > 255 {
		return nil, fmt.Errorf("%w: bad subject", ErrInvalidToken)
	}
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	if email == "" || len(email) > 254 {
		return nil, fmt.Errorf("%w: no usable email", ErrInvalidToken)
	}
	if !emailVerified(claims.EmailVerified) {
		return nil, ErrEmailUnverified
	}
	return &Identity{Subject: claims.Subject, Email: email}, nil
}

// emailVerified accepts the boolean true (and Google's legacy string
// "true"); anything else, including absence, is unverified.
func emailVerified(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "true"
	}
	return false
}
