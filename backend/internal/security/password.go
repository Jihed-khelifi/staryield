// Package security holds cryptographic primitives: Argon2id password hashing,
// JWT access tokens, opaque random tokens and CSRF tokens.
package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// PasswordHasher hashes and verifies passwords with Argon2id (RFC 9106).
type PasswordHasher struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLen     uint32
	keyLen      uint32
	dummy       string
}

// NewPasswordHasher returns a hasher with the given cost parameters.
func NewPasswordHasher(memoryKiB, iterations uint32, parallelism uint8) *PasswordHasher {
	h := &PasswordHasher{memory: memoryKiB, iterations: iterations, parallelism: parallelism, saltLen: 16, keyLen: 32}
	// A real hash used to equalise timing when a login targets an unknown
	// e-mail, so response times do not reveal which accounts exist.
	h.dummy, _ = h.Hash("dummy-password-for-timing-equalisation")
	return h
}

// ErrMismatch is returned when a password does not match its hash.
var ErrMismatch = errors.New("security: password mismatch")

// Hash returns a PHC-formatted Argon2id hash.
func (h *PasswordHasher) Hash(password string) (string, error) {
	salt := make([]byte, h.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("security: salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, h.iterations, h.memory, h.parallelism, h.keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, h.memory, h.iterations, h.parallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify checks password against encoded in constant time. It returns
// needsRehash when the stored parameters are weaker than the current ones.
func (h *PasswordHasher) Verify(password, encoded string) (needsRehash bool, err error) {
	p, salt, key, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}
	other := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.parallelism, uint32(len(key)))
	if subtle.ConstantTimeCompare(key, other) != 1 {
		return false, ErrMismatch
	}
	return p.memory < h.memory || p.iterations < h.iterations || p.parallelism < h.parallelism, nil
}

// VerifyDummy burns the same CPU as a real verification.
func (h *PasswordHasher) VerifyDummy(password string) {
	_, _ = h.Verify(password, h.dummy)
}

type hashParams struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

func decodeHash(encoded string) (hashParams, []byte, []byte, error) {
	var p hashParams
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return p, nil, nil, errors.New("security: unsupported hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return p, nil, nil, errors.New("security: unsupported argon2 version")
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism); err != nil {
		return p, nil, nil, errors.New("security: invalid hash parameters")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return p, nil, nil, errors.New("security: invalid salt")
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) < 16 {
		return p, nil, nil, errors.New("security: invalid key")
	}
	return p, salt, key, nil
}

// Password policy limits. The maximum bounds hashing cost (DoS protection).
const (
	PasswordMinLength = 8
	PasswordMaxLength = 128
)

// commonPasswords rejects the most frequently breached choices that would
// otherwise satisfy the composition rules.
var commonPasswords = map[string]struct{}{
	"password1!": {}, "password123!": {}, "p@ssw0rd": {}, "p@ssword1": {}, "passw0rd!": {},
	"qwerty123!": {}, "welcome1!": {}, "welcome123!": {}, "admin123!": {}, "letmein1!": {},
	"iloveyou1!": {}, "abc12345!": {}, "changeme1!": {}, "summer2024!": {}, "winter2024!": {},
	"staryield1!": {}, "staryield123!": {}, "password!1": {}, "qwerty1!": {}, "aa123456!": {},
}

// ValidatePassword enforces the platform policy: 8–128 characters with at
// least one uppercase letter, one digit and one special character (matching
// the sign-up form), not containing the e-mail local part, and not a common
// password. It returns a user-facing message, or "" when acceptable.
func ValidatePassword(password, email string) string {
	n := utf8.RuneCountInString(password)
	if n < PasswordMinLength {
		return "Use at least 8 characters."
	}
	if n > PasswordMaxLength {
		return "Use at most 128 characters."
	}
	var upper, digit, special bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		case !unicode.IsLetter(r) && !unicode.IsSpace(r):
			special = true
		}
	}
	if !upper || !digit || !special {
		return "Use 8+ characters, an uppercase letter, a number, and a special character."
	}
	lower := strings.ToLower(password)
	if _, ok := commonPasswords[lower]; ok {
		return "This password is too common. Please choose another."
	}
	if at := strings.IndexByte(email, '@'); at >= 4 {
		if strings.Contains(lower, strings.ToLower(email[:at])) {
			return "Your password should not contain your email address."
		}
	}
	return ""
}
