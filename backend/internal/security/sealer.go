package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// sealedPrefix versions the ciphertext format so keys can be rotated later.
const sealedPrefix = "v1:"

// Sealer encrypts small secrets at rest with AES-256-GCM. Each value gets a
// random nonce; an optional associated-data string binds a ciphertext to
// its owner (e.g. the user ID), so values cannot be swapped between rows.
type Sealer struct{ aead cipher.AEAD }

// NewSealer returns a sealer for a 32-byte key.
func NewSealer(key []byte) (*Sealer, error) {
	if len(key) != 32 {
		return nil, errors.New("security: sealer key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Sealer{aead: aead}, nil
}

// Seal encrypts plaintext bound to ad.
func (s *Sealer) Seal(plaintext, ad string) (string, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("security: seal nonce: %w", err)
	}
	out := s.aead.Seal(nonce, nonce, []byte(plaintext), []byte(ad))
	return sealedPrefix + base64.RawStdEncoding.EncodeToString(out), nil
}

// ErrUnseal is returned for tampered, truncated or foreign ciphertexts.
var ErrUnseal = errors.New("security: cannot decrypt value")

// Open decrypts a value produced by Seal with the same ad.
func (s *Sealer) Open(sealed, ad string) (string, error) {
	if !strings.HasPrefix(sealed, sealedPrefix) {
		return "", ErrUnseal
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(sealed, sealedPrefix))
	if err != nil || len(raw) < s.aead.NonceSize()+s.aead.Overhead() {
		return "", ErrUnseal
	}
	nonce, body := raw[:s.aead.NonceSize()], raw[s.aead.NonceSize():]
	plain, err := s.aead.Open(nil, nonce, body, []byte(ad))
	if err != nil {
		return "", ErrUnseal
	}
	return string(plain), nil
}
