package security

import (
	"strings"
	"testing"
	"time"
)

// RFC 6238 appendix B test vectors (SHA-1, seed "12345678901234567890"),
// truncated to 6 digits as authenticator apps use them.
func TestTOTPMatchesRFC6238Vectors(t *testing.T) {
	secret := b32.EncodeToString([]byte("12345678901234567890"))
	for unix, want := range map[int64]string{
		59:          "287082",
		1111111109:  "081804",
		1111111111:  "050471",
		1234567890:  "005924",
		2000000000:  "279037",
		20000000000: "353130",
	} {
		got, err := TOTPCode(secret, TOTPStep(time.Unix(unix, 0)))
		if err != nil || got != want {
			t.Errorf("t=%d: got %s (%v), want %s", unix, got, err, want)
		}
	}
}

func TestVerifyTOTPWindowAndReplay(t *testing.T) {
	secret, err := NewTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	step := TOTPStep(now)
	code, _ := TOTPCode(secret, step)
	if got, ok := VerifyTOTP(secret, code, now, 0); !ok || got != step {
		t.Fatalf("current code rejected: %v %d", ok, got)
	}
	// One step of drift either way is accepted.
	prev, _ := TOTPCode(secret, step-1)
	if _, ok := VerifyTOTP(secret, prev, now, 0); !ok {
		t.Fatal("previous step rejected")
	}
	old, _ := TOTPCode(secret, step-2)
	if _, ok := VerifyTOTP(secret, old, now, 0); ok {
		t.Fatal("stale code accepted")
	}
	// Replay: a step already used is refused.
	if _, ok := VerifyTOTP(secret, code, now, step); ok {
		t.Fatal("replayed code accepted")
	}
	for _, bad := range []string{"", "12345", "1234567", "abcdef", "12 34 5x"} {
		if _, ok := VerifyTOTP(secret, bad, now, 0); ok {
			t.Errorf("malformed code %q accepted", bad)
		}
	}
	spaced := code[:3] + " " + code[3:]
	if _, ok := VerifyTOTP(secret, spaced, now, 0); !ok {
		t.Error("code with a space should be accepted")
	}
}

func TestTOTPURIAndRecoveryCodes(t *testing.T) {
	uri := TOTPURI("Staryield Admin", "ops@staryield.net", "JBSWY3DPEHPK3PXP")
	for _, want := range []string{"otpauth://totp/Staryield%20Admin:ops@staryield.net?", "secret=JBSWY3DPEHPK3PXP", "issuer=Staryield+Admin", "digits=6", "period=30"} {
		if !strings.Contains(uri, want) {
			t.Errorf("uri %q missing %q", uri, want)
		}
	}
	codes, err := NewRecoveryCodes(10)
	if err != nil || len(codes) != 10 {
		t.Fatalf("codes: %v %v", codes, err)
	}
	seen := map[string]bool{}
	for _, c := range codes {
		if len(c) != 11 || c[5] != '-' || seen[c] {
			t.Fatalf("bad or duplicate code %q", c)
		}
		seen[c] = true
		if NormalizeRecoveryCode(strings.ToUpper(strings.ReplaceAll(c, "-", " "))) != c {
			t.Fatalf("normalize %q", c)
		}
	}
	if NormalizeRecoveryCode("short") != "" {
		t.Fatal("short code normalized")
	}
}

func TestSealerRoundTripAndTamper(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	s, err := NewSealer(key)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := s.Seal("JBSWY3DPEHPK3PXP", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sealed, "JBSWY3DPEHPK3PXP") {
		t.Fatal("plaintext visible")
	}
	if got, err := s.Open(sealed, "user-1"); err != nil || got != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("open: %q %v", got, err)
	}
	if _, err := s.Open(sealed, "user-2"); err == nil {
		t.Fatal("ciphertext moved to another owner must fail")
	}
	tampered := sealed[:len(sealed)-2] + "AA"
	if _, err := s.Open(tampered, "user-1"); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
	other, _ := NewSealer([]byte(strings.Repeat("x", 32)))
	if _, err := other.Open(sealed, "user-1"); err == nil {
		t.Fatal("wrong key accepted")
	}
	again, _ := s.Seal("JBSWY3DPEHPK3PXP", "user-1")
	if again == sealed {
		t.Fatal("nonces must differ")
	}
	if _, err := NewSealer([]byte("short")); err == nil {
		t.Fatal("short key accepted")
	}
}
