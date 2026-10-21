package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6238 TOTP is defined over HMAC-SHA1; authenticator apps expect it.
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP parameters (RFC 6238 defaults understood by every authenticator app).
const (
	TOTPDigits = 6
	TOTPPeriod = 30 * time.Second
	// TOTPSkew accepts codes from one step before and after the current one
	// to absorb clock drift between the server and the phone.
	TOTPSkew = 1
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewTOTPSecret returns a random 160-bit secret, base32-encoded.
func NewTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("security: totp secret: %w", err)
	}
	return b32.EncodeToString(buf), nil
}

// TOTPStep returns the time step containing t.
func TOTPStep(t time.Time) int64 { return t.Unix() / int64(TOTPPeriod/time.Second) }

// TOTPCode computes the code for a base32 secret at a time step.
func TOTPCode(secret string, step int64) (string, error) {
	key, err := b32.DecodeString(strings.ToUpper(strings.ReplaceAll(secret, " ", "")))
	if err != nil || len(key) < 10 {
		return "", fmt.Errorf("security: invalid totp secret")
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	bin := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", TOTPDigits, bin%1_000_000), nil
}

// VerifyTOTP checks code against the secret around now and returns the
// matched step. Steps at or before lastUsed are rejected, so a code can only
// be used once (replay protection).
func VerifyTOTP(secret, code string, now time.Time, lastUsed int64) (int64, bool) {
	code = strings.TrimSpace(strings.ReplaceAll(code, " ", ""))
	if len(code) != TOTPDigits {
		return 0, false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	current := TOTPStep(now)
	matched := int64(0)
	ok := false
	// Compare every candidate in constant time and keep scanning, so timing
	// does not reveal which window matched.
	for delta := int64(-TOTPSkew); delta <= TOTPSkew; delta++ {
		step := current + delta
		want, err := TOTPCode(secret, step)
		if err != nil {
			return 0, false
		}
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 && step > lastUsed {
			matched, ok = step, true
		}
	}
	return matched, ok
}

// TOTPURI builds the otpauth:// enrolment URI rendered as a QR code.
func TOTPURI(issuer, account, secret string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(TOTPDigits))
	q.Set("period", fmt.Sprint(int(TOTPPeriod/time.Second)))
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// NewRecoveryCodes returns n single-use recovery codes ("xxxxx-xxxxx",
// lowercase base32, 50 bits of entropy each).
func NewRecoveryCodes(n int) ([]string, error) {
	out := make([]string, 0, n)
	for range n {
		buf := make([]byte, 7)
		if _, err := rand.Read(buf); err != nil {
			return nil, fmt.Errorf("security: recovery code: %w", err)
		}
		s := strings.ToLower(b32.EncodeToString(buf))[:10]
		out = append(out, s[:5]+"-"+s[5:])
	}
	return out, nil
}

// NormalizeRecoveryCode canonicalises user input before hashing.
func NormalizeRecoveryCode(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	c = strings.NewReplacer(" ", "", "-", "").Replace(c)
	if len(c) != 10 {
		return ""
	}
	return c[:5] + "-" + c[5:]
}
