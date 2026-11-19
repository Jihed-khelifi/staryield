package stripepay_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment/stripepay"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment/stripepay/stripetest"
)

type stripepayOptions = stripepay.Options

const (
	testSecretKey      = "sk_test_unit_secret_key_0123456789"
	testPublishableKey = "pk_test_unit_publishable_0123456789"
	testWebhookSecret  = "whsec_unit_webhook_secret_0123456789"
)

// fixture is an adapter wired to a fresh fake Stripe.
type fixture struct {
	t    *testing.T
	fake *stripetest.Server
	p    *stripepay.Provider
	logs *observer.ObservedLogs
	now  time.Time
	seq  int
}

func newFixture(t *testing.T, mod ...func(*stripepay.Options)) *fixture {
	t.Helper()
	fake := stripetest.New()
	t.Cleanup(fake.Close)
	core, logs := observer.New(zap.DebugLevel)
	f := &fixture{t: t, fake: fake, logs: logs, now: time.Now()}
	zero := int64(0)
	opts := stripepay.Options{SecretKey: testSecretKey, PublishableKey: testPublishableKey, WebhookSecret: testWebhookSecret,
		APIBase: fake.URL, Timeout: 5 * time.Second, WebhookTolerance: 5 * time.Minute, MaxNetworkRetries: &zero,
		Logger: zap.New(core), Now: func() time.Time { return f.now }}
	for _, m := range mod {
		m(&opts)
	}
	p, err := stripepay.New(opts)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	f.p = p
	return f
}

func ctx() context.Context { return context.Background() }

// checkout is a typical credit-purchase request with the given token.
func checkoutRequest(token string) payment.CheckoutRequest {
	id := uuid.New()
	return payment.CheckoutRequest{PurchaseID: id, UserID: uuid.New(), AmountCents: 2500, Currency: "USD", Description: "250 credits",
		PaymentToken: token, IdempotencyKey: "purchase:" + id.String(), StatementDescriptor: "IGNORED"}
}

// customer creates a customer for tests that need one.
func (f *fixture) customer() string {
	f.t.Helper()
	ref, err := f.p.EnsureCustomer(ctx(), payment.CustomerRequest{UserID: uuid.New(), IdempotencyKey: "cus:" + uuid.NewString(),
		Email: "ada@example.com", Name: "Ada"})
	if err != nil {
		f.t.Fatalf("ensure customer: %v", err)
	}
	return ref
}

// paid makes a succeeded payment and returns its PaymentIntent id.
func (f *fixture) paid(amount int64) string {
	f.t.Helper()
	f.seq++ // ConfirmationTokens are single use: every payment gets its own
	req := checkoutRequest(fmt.Sprintf("ctoken_ok_visa_4242_%c%c", 'A'+rune(f.seq/26%26), 'A'+rune(f.seq%26)))
	req.AmountCents = amount
	res, err := f.p.Checkout(ctx(), req)
	if err != nil || res.Status != payment.CheckoutSucceeded {
		f.t.Fatalf("payment: %+v %v", res, err)
	}
	return res.ProviderRef
}

func (f *fixture) lastPost(path string) stripetest.Request {
	f.t.Helper()
	reqs := f.fake.RequestsTo("POST", path)
	if len(reqs) == 0 {
		f.t.Fatalf("no POST %s was sent", path)
	}
	return reqs[len(reqs)-1]
}
