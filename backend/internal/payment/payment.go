// Package payment abstracts the external payment provider (spec §3: "Buy
// Credits → Payment Provider"). Card data never reaches Staryield servers:
// a real provider tokenises cards in its hosted fields, and only signed
// webhooks or server-to-server confirmations credit a wallet.
package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// CheckoutStatus is the provider's answer to a checkout attempt.
type CheckoutStatus string

// Checkout outcomes.
const (
	CheckoutSucceeded CheckoutStatus = "succeeded"
	CheckoutPending   CheckoutStatus = "pending"
	CheckoutFailed    CheckoutStatus = "failed"
)

// CheckoutRequest describes a charge.
type CheckoutRequest struct {
	PurchaseID  uuid.UUID
	UserID      uuid.UUID
	AmountCents int64
	Currency    string
	Description string
	Email       string
}

// CheckoutResult is the provider response.
type CheckoutResult struct {
	Status      CheckoutStatus
	ProviderRef string
	RedirectURL string
	FailureCode string
}

// WebhookEvent is a verified provider notification.
type WebhookEvent struct {
	Type        string `json:"type"` // payment.succeeded | payment.failed | payment.refunded
	ProviderRef string `json:"provider_ref"`
	PurchaseID  string `json:"purchase_id"`
	FailureCode string `json:"failure_code,omitempty"`
}

// Provider is a payment gateway.
type Provider interface {
	Name() string
	Checkout(ctx context.Context, req CheckoutRequest) (CheckoutResult, error)
	VerifyWebhook(header string, body []byte, now time.Time) (*WebhookEvent, error)
}

// Webhook verification errors.
var (
	ErrBadSignature = errors.New("payment: invalid webhook signature")
	ErrStale        = errors.New("payment: webhook timestamp outside tolerance")
)

// ErrDisabled is returned by checkouts when no payment provider is configured.
var ErrDisabled = errors.New("payment: purchases are disabled")

// Disabled refuses every checkout and webhook. It lets a deployment run
// without a payment provider instead of falling back to the sandbox, which
// would grant credits for free.
type Disabled struct{}

// Name implements Provider.
func (Disabled) Name() string { return "disabled" }

// Checkout implements Provider.
func (Disabled) Checkout(context.Context, CheckoutRequest) (CheckoutResult, error) {
	return CheckoutResult{}, ErrDisabled
}

// VerifyWebhook implements Provider.
func (Disabled) VerifyWebhook(string, []byte, time.Time) (*WebhookEvent, error) {
	return nil, ErrBadSignature
}

// Sandbox approves every checkout immediately. It exists for development
// and automated tests; configuration refuses it in production unless
// explicitly allowed.
type Sandbox struct {
	secret    []byte
	tolerance time.Duration
}

// NewSandbox returns the sandbox provider.
func NewSandbox(webhookSecret string, tolerance time.Duration) *Sandbox {
	return &Sandbox{secret: []byte(webhookSecret), tolerance: tolerance}
}

// Name implements Provider.
func (s *Sandbox) Name() string { return "sandbox" }

// Checkout implements Provider. Amounts ending in 13 cents fail, which lets
// tests exercise the failure path deterministically.
func (s *Sandbox) Checkout(_ context.Context, req CheckoutRequest) (CheckoutResult, error) {
	ref := "sbx_" + security.MustRandomToken(12)
	if req.AmountCents%100 == 13 {
		return CheckoutResult{Status: CheckoutFailed, ProviderRef: ref, FailureCode: "card_declined"}, nil
	}
	return CheckoutResult{Status: CheckoutSucceeded, ProviderRef: ref}, nil
}

// VerifyWebhook implements Provider using the signature scheme
// "t=<unix>,v1=<hex hmac-sha256(secret, t + "." + body)>".
func (s *Sandbox) VerifyWebhook(header string, body []byte, now time.Time) (*WebhookEvent, error) {
	return VerifySignedPayload(s.secret, s.tolerance, header, body, now)
}

// Sign produces a signature header for body at t (tests and tooling).
func Sign(secret []byte, body []byte, t time.Time) string {
	ts := strconv.FormatInt(t.Unix(), 10)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	return "t=" + ts + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

// VerifySignedPayload checks an HMAC signature header with replay protection.
func VerifySignedPayload(secret []byte, tolerance time.Duration, header string, body []byte, now time.Time) (*WebhookEvent, error) {
	var ts int64
	var sigs []string
	for _, part := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch k {
		case "t":
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, ErrBadSignature
			}
			ts = n
		case "v1":
			sigs = append(sigs, v)
		}
	}
	if ts == 0 || len(sigs) == 0 {
		return nil, ErrBadSignature
	}
	at := time.Unix(ts, 0)
	if d := now.Sub(at); d > tolerance || d < -tolerance {
		return nil, ErrStale
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(strconv.FormatInt(ts, 10) + "."))
	mac.Write(body)
	expected := mac.Sum(nil)
	valid := false
	for _, sig := range sigs {
		got, err := hex.DecodeString(sig)
		if err == nil && hmac.Equal(got, expected) {
			valid = true
		}
	}
	if !valid {
		return nil, ErrBadSignature
	}
	var evt WebhookEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return nil, fmt.Errorf("payment: decode webhook: %w", err)
	}
	if evt.Type == "" || evt.ProviderRef == "" {
		return nil, fmt.Errorf("payment: incomplete webhook")
	}
	return &evt, nil
}
