package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// RandomToken returns n bytes of CSPRNG output, base64url-encoded.
func RandomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("security: random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// MustRandomToken is RandomToken that panics on CSPRNG failure, which is
// unrecoverable.
func MustRandomToken(n int) string {
	t, err := RandomToken(n)
	if err != nil {
		panic(err)
	}
	return t
}

// HashToken returns the hex SHA-256 of an opaque token. Only hashes of
// refresh, reset and ticket tokens are persisted, so a database leak does not
// yield usable credentials.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Keyed returns hex HMAC-SHA256(secret, value), truncated to n hex chars
// (0 = full). Used to pseudonymise IP addresses for analytics and audit.
func Keyed(secret, value string, n int) string {
	if value == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(value))
	out := hex.EncodeToString(mac.Sum(nil))
	if n > 0 && n < len(out) {
		return out[:n]
	}
	return out
}

// Claims are the JWT access-token claims.
type Claims struct {
	Role     string `json:"role"`
	FamilyID string `json:"fam,omitempty"`
	// Version must equal the user's current token_version.
	Version int `json:"ver"`
	jwt.RegisteredClaims
}

// TokenIssuer issues and verifies HS256 access tokens.
type TokenIssuer struct {
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
	now      func() time.Time
}

// NewTokenIssuer builds an issuer. secret must be >= 32 bytes.
func NewTokenIssuer(secret, issuer, audience string, ttl time.Duration, now func() time.Time) (*TokenIssuer, error) {
	if len(secret) < 32 {
		return nil, errors.New("security: jwt secret too short")
	}
	if now == nil {
		now = time.Now
	}
	return &TokenIssuer{secret: []byte(secret), issuer: issuer, audience: audience, ttl: ttl, now: now}, nil
}

// TTL returns the access token lifetime.
func (t *TokenIssuer) TTL() time.Duration { return t.ttl }

// Issue returns a signed access token for userID.
func (t *TokenIssuer) Issue(userID uuid.UUID, role string, familyID uuid.UUID, version int) (string, time.Time, error) {
	now := t.now().UTC()
	exp := now.Add(t.ttl)
	claims := Claims{
		Role:     role,
		FamilyID: familyID.String(),
		Version:  version,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    t.issuer,
			Subject:   userID.String(),
			Audience:  jwt.ClaimStrings{t.audience},
			ExpiresAt: jwt.NewNumericDate(exp),
			NotBefore: jwt.NewNumericDate(now.Add(-5 * time.Second)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("security: sign: %w", err)
	}
	return signed, exp, nil
}

// Token verification errors.
var (
	ErrTokenExpired = errors.New("security: token expired")
	ErrTokenInvalid = errors.New("security: token invalid")
)

// Parse verifies signature, algorithm, issuer, audience and time claims.
func (t *TokenIssuer) Parse(token string) (*Claims, error) {
	var claims Claims
	parsed, err := jwt.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(t.issuer),
		jwt.WithAudience(t.audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(10*time.Second),
		jwt.WithTimeFunc(t.now),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}
	if !parsed.Valid {
		return nil, ErrTokenInvalid
	}
	if _, err := uuid.Parse(claims.Subject); err != nil {
		return nil, ErrTokenInvalid
	}
	return &claims, nil
}

// CSRF issues and verifies signed double-submit tokens: "<nonce>.<mac>".
// The browser echoes the token from a cookie into the X-CSRF-Token header;
// the server checks both match and the MAC is valid, so a value planted by a
// sibling sub-domain (cookie tossing) is rejected unless it was minted here.
type CSRF struct{ secret []byte }

// NewCSRF returns a CSRF helper. secret must be >= 32 bytes.
func NewCSRF(secret string) (*CSRF, error) {
	if len(secret) < 32 {
		return nil, errors.New("security: csrf secret too short")
	}
	return &CSRF{secret: []byte(secret)}, nil
}

// Issue returns a fresh token.
func (c *CSRF) Issue() string {
	nonce := MustRandomToken(24)
	return nonce + "." + c.mac(nonce)
}

// Valid reports whether token is well-formed and authentic.
func (c *CSRF) Valid(token string) bool {
	nonce, mac, ok := strings.Cut(token, ".")
	if !ok || len(nonce) < 16 || len(nonce) > 64 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(mac), []byte(c.mac(nonce))) == 1
}

// Match verifies the header token equals the cookie token and is authentic.
func (c *CSRF) Match(header, cookie string) bool {
	if header == "" || cookie == "" || len(header) > 256 {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(header), []byte(cookie)) != 1 {
		return false
	}
	return c.Valid(header)
}

func (c *CSRF) mac(nonce string) string {
	m := hmac.New(sha256.New, c.secret)
	m.Write([]byte("csrf:" + nonce))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
