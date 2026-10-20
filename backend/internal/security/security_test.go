package security

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestPasswordHashVerify(t *testing.T) {
	h := NewPasswordHasher(8*1024, 1, 1)
	hash, err := h.Hash("Correct!Horse1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=8192,t=1,p=1$") {
		t.Fatalf("unexpected encoding: %s", hash)
	}
	if again, _ := h.Hash("Correct!Horse1"); again == hash {
		t.Fatal("salts must be random")
	}
	if _, err := h.Verify("Correct!Horse1", hash); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if _, err := h.Verify("wrong", hash); !errors.Is(err, ErrMismatch) {
		t.Fatalf("mismatch: %v", err)
	}
	// Stronger parameters flag the old hash for rehashing.
	stronger := NewPasswordHasher(16*1024, 2, 1)
	if needs, err := stronger.Verify("Correct!Horse1", hash); err != nil || !needs {
		t.Fatalf("rehash: %v %v", needs, err)
	}
	for _, bad := range []string{"", "plain", "$argon2i$v=19$m=1,t=1,p=1$a$b", "$argon2id$v=18$m=1,t=1,p=1$YQ$YQ"} {
		if _, err := h.Verify("x", bad); err == nil {
			t.Errorf("malformed hash %q accepted", bad)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	for pw, ok := range map[string]bool{
		"Str0ng!Passw0rd":         true,
		"short1!A":                true,
		"Sh0rt!":                  false,
		"alllowercase1!":          false,
		"NoDigits!!":              false,
		"NoSpecial123":            false,
		"Password1!":              false, // common
		strings.Repeat("A1!", 50): false,
	} {
		if got := ValidatePassword(pw, "someone@example.com") == ""; got != ok {
			t.Errorf("%q: valid=%v want %v", pw, got, ok)
		}
	}
	if ValidatePassword("Cassandra!2026", "cassandra@example.com") == "" {
		t.Fatal("password containing the e-mail name must be rejected")
	}
}

func TestTokenIssueAndParse(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	iss, err := NewTokenIssuer(strings.Repeat("k", 32), "iss", "aud", 15*time.Minute, clock)
	if err != nil {
		t.Fatal(err)
	}
	user, fam := uuid.New(), uuid.New()
	tok, exp, err := iss.Issue(user, "client", fam, 3)
	if err != nil || exp.Sub(now) != 15*time.Minute {
		t.Fatalf("issue: %v %v", exp, err)
	}
	claims, err := iss.Parse(tok)
	if err != nil || claims.Subject != user.String() || claims.Role != "client" || claims.Version != 3 || claims.FamilyID != fam.String() {
		t.Fatalf("parse: %+v %v", claims, err)
	}

	// Expired.
	now = now.Add(16 * time.Minute)
	if _, err := iss.Parse(tok); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expired: %v", err)
	}
	now = now.Add(-16 * time.Minute)

	// Wrong secret, issuer and audience are rejected.
	other, _ := NewTokenIssuer(strings.Repeat("x", 32), "iss", "aud", time.Minute, clock)
	if _, err := other.Parse(tok); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("wrong secret: %v", err)
	}
	wrongAud, _ := NewTokenIssuer(strings.Repeat("k", 32), "iss", "other", time.Minute, clock)
	if _, err := wrongAud.Parse(tok); err == nil {
		t.Fatal("wrong audience accepted")
	}
	// Algorithm confusion: "none" must never validate.
	unsigned := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{Role: "admin", RegisteredClaims: jwt.RegisteredClaims{
		Subject: user.String(), Issuer: "iss", Audience: jwt.ClaimStrings{"aud"},
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)), IssuedAt: jwt.NewNumericDate(now),
	}})
	none, _ := unsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := iss.Parse(none); err == nil {
		t.Fatal("alg=none accepted")
	}
	if _, err := iss.Parse("garbage"); err == nil {
		t.Fatal("garbage accepted")
	}
	if _, err := NewTokenIssuer("short", "i", "a", time.Minute, nil); err == nil {
		t.Fatal("short secret accepted")
	}
}

func TestCSRF(t *testing.T) {
	c, err := NewCSRF(strings.Repeat("s", 32))
	if err != nil {
		t.Fatal(err)
	}
	tok := c.Issue()
	if !c.Valid(tok) || !c.Match(tok, tok) {
		t.Fatal("fresh token must validate")
	}
	other, _ := NewCSRF(strings.Repeat("o", 32))
	if other.Valid(tok) {
		t.Fatal("token from another secret validated (cookie tossing)")
	}
	if c.Match(tok, c.Issue()) || c.Match("", tok) || c.Match(tok, "") {
		t.Fatal("mismatched header/cookie accepted")
	}
	if c.Valid("abc.def") || c.Valid("no-dot") {
		t.Fatal("malformed token accepted")
	}
}

func TestOpaqueTokens(t *testing.T) {
	a, b := MustRandomToken(32), MustRandomToken(32)
	if a == b || len(a) < 40 {
		t.Fatal("random tokens must be unique and long")
	}
	if HashToken("x") != HashToken("x") || HashToken("x") == HashToken("y") || len(HashToken("x")) != 64 {
		t.Fatal("hash token")
	}
	if Keyed("s", "1.2.3.4", 16) == Keyed("t", "1.2.3.4", 16) || len(Keyed("s", "ip", 16)) != 16 || Keyed("s", "", 16) != "" {
		t.Fatal("keyed hash")
	}
}
