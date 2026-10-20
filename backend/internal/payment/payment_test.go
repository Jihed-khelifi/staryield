package payment

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWebhookSignatures(t *testing.T) {
	secret := []byte("webhook-secret-0123456789")
	body := []byte(`{"type":"payment.succeeded","provider_ref":"sbx_1"}`)
	now := time.Now()
	s := NewSandbox(string(secret), 5*time.Minute)

	evt, err := s.VerifyWebhook(Sign(secret, body, now), body, now)
	if err != nil || evt.Type != "payment.succeeded" || evt.ProviderRef != "sbx_1" {
		t.Fatalf("valid: %+v %v", evt, err)
	}
	if _, err := s.VerifyWebhook(Sign(secret, body, now), []byte(`{"type":"payment.succeeded","provider_ref":"sbx_2"}`), now); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("tampered body: %v", err)
	}
	if _, err := s.VerifyWebhook(Sign([]byte("other"), body, now), body, now); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("wrong secret: %v", err)
	}
	if _, err := s.VerifyWebhook(Sign(secret, body, now.Add(-10*time.Minute)), body, now); !errors.Is(err, ErrStale) {
		t.Fatalf("replay outside tolerance: %v", err)
	}
	for _, h := range []string{"", "t=abc,v1=00", "v1=00", "t=1"} {
		if _, err := s.VerifyWebhook(h, body, now); err == nil {
			t.Errorf("header %q accepted", h)
		}
	}
	incomplete := []byte(`{"type":""}`)
	if _, err := s.VerifyWebhook(Sign(secret, incomplete, now), incomplete, now); err == nil {
		t.Fatal("incomplete event accepted")
	}
}

func TestSandboxCheckout(t *testing.T) {
	s := NewSandbox("x", time.Minute)
	ok, _ := s.Checkout(context.Background(), CheckoutRequest{AmountCents: 999})
	fail, _ := s.Checkout(context.Background(), CheckoutRequest{AmountCents: 1013})
	if ok.Status != CheckoutSucceeded || ok.ProviderRef == "" || fail.Status != CheckoutFailed || fail.FailureCode == "" {
		t.Fatalf("sandbox: %+v %+v", ok, fail)
	}
}
