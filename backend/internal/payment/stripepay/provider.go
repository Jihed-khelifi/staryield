// Package stripepay is the Stripe payment adapter (revisions 6.0, docs/
// platform-extensions.md §26.5). Card details are entered in Stripe's Payment
// Element and never reach Staryield (PCI SAQ-A): the browser sends a
// ConfirmationToken (one-off payment) or a PaymentMethod id (saved card) and
// this package turns them into PaymentIntents.
//
// The adapter implements the optional capability interfaces of package
// payment (checkout, customer vault, saved cards, stored charges, lookup,
// refunds, signed webhooks, disputes, account inspection, event replay). It
// never logs API keys or request bodies.
package stripepay

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/stripe/stripe-go/v87"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
)

// Name is the provider name used in routing rules and stored transactions.
const Name = "stripe"

// Options configure the adapter.
type Options struct {
	// SecretKey is the secret or restricted API key (sk_… / rk_…).
	SecretKey string
	// PublishableKey is the key the browser loads Stripe.js with.
	PublishableKey string
	// WebhookSecret is the signing secret of the webhook endpoint (whsec_…).
	WebhookSecret string
	// APIBase replaces https://api.stripe.com (tests: a fake Stripe).
	APIBase string
	// Timeout bounds every request (default 20 s).
	Timeout time.Duration
	// WebhookTolerance is the accepted age of a webhook signature (default 5 min).
	WebhookTolerance time.Duration
	// MaxNetworkRetries is how often the SDK retries a request that failed in
	// transit; every create call carries an idempotency key, so a retry never
	// charges twice. Nil means one retry; point at 0 to disable.
	MaxNetworkRetries *int64
	// HTTPClient replaces the default client (tests).
	HTTPClient *http.Client
	// Logger receives warnings (API-version mismatches, configuration errors).
	Logger *zap.Logger
	// Now replaces time.Now (token expiry checks).
	Now func() time.Time
}

// Provider is the Stripe provider.
type Provider struct {
	client     *stripe.Client
	publishKey string
	webhookKey string
	tolerance  time.Duration
	livemode   bool
	log        *zap.Logger
	now        func() time.Time

	versionMu sync.Mutex
	versions  map[string]bool // webhook API versions already reported
}

// ErrConfiguration means Stripe refused our credentials (revoked or wrong
// key, missing permission). It is logged as a configuration error; payments
// are refused until the key is fixed.
var ErrConfiguration = payment.ErrConfiguration

// New builds the provider. The keys must be present and of one mode.
func New(o Options) (*Provider, error) {
	if o.SecretKey == "" || o.PublishableKey == "" || o.WebhookSecret == "" {
		return nil, errors.New("stripepay: the secret key, publishable key and webhook secret are required")
	}
	if keyLivemode(o.SecretKey) != keyLivemode(o.PublishableKey) {
		return nil, errors.New("stripepay: the secret and publishable keys must be of the same mode")
	}
	if o.Timeout <= 0 {
		o.Timeout = 20 * time.Second
	}
	if o.WebhookTolerance <= 0 {
		o.WebhookTolerance = 5 * time.Minute
	}
	if o.Logger == nil {
		o.Logger = zap.NewNop()
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	retries := int64(1)
	if o.MaxNetworkRetries != nil {
		retries = *o.MaxNetworkRetries
	}
	httpClient := o.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: o.Timeout}
	}
	cfg := &stripe.BackendConfig{
		HTTPClient:        httpClient,
		MaxNetworkRetries: stripe.Int64(retries),
		// The SDK's own logger prints request details; the adapter logs what
		// matters (without secrets) through zap.
		LeveledLogger: &stripe.LeveledLogger{Level: stripe.LevelNull},
	}
	if o.APIBase != "" {
		cfg.URL = stripe.String(strings.TrimRight(o.APIBase, "/"))
	}
	return &Provider{
		client:     stripe.NewClient(o.SecretKey, stripe.WithBackends(stripe.NewBackendsWithConfig(cfg))),
		publishKey: o.PublishableKey,
		webhookKey: o.WebhookSecret,
		tolerance:  o.WebhookTolerance,
		livemode:   keyLivemode(o.SecretKey),
		log:        o.Logger.With(zap.String("component", "stripe")),
		now:        o.Now,
		versions:   map[string]bool{},
	}, nil
}

// keyLivemode reports whether a Stripe key is a live-mode key.
func keyLivemode(key string) bool { return strings.Contains(key, "_live_") }

// Name implements payment.Named.
func (p *Provider) Name() string { return Name }

// Livemode reports whether the provider runs with live keys.
func (p *Provider) Livemode() bool { return p.livemode }

// PublishableKey is the key the browser loads Stripe.js with.
func (p *Provider) PublishableKey() string { return p.publishKey }

// Compile-time proof of the capabilities (contract table, §26.5).
var (
	_ payment.Named            = (*Provider)(nil)
	_ payment.ClientTokenizer  = (*Provider)(nil)
	_ payment.Checkouter       = (*Provider)(nil)
	_ payment.TokenInspector   = (*Provider)(nil)
	_ payment.CustomerVault    = (*Provider)(nil)
	_ payment.MethodAttacher   = (*Provider)(nil)
	_ payment.StoredCharger    = (*Provider)(nil)
	_ payment.ChargeLookup     = (*Provider)(nil)
	_ payment.ChargeRetriever  = (*Provider)(nil)
	_ payment.Refunder         = (*Provider)(nil)
	_ payment.WebhookVerifier  = (*Provider)(nil)
	_ payment.DisputeManager   = (*Provider)(nil)
	_ payment.AccountInspector = (*Provider)(nil)
	_ payment.EventFetcher     = (*Provider)(nil)
)

// ------------------------------------------------------------- errors

// softDeclines are decline codes that may succeed when retried later; every
// other decline is hard (the customer needs another card).
var softDeclines = map[string]bool{
	"insufficient_funds": true, "processing_error": true, "try_again_later": true,
	"issuer_not_available": true, "reenter_transaction": true, "approve_with_id": true,
}

// declineCode normalises Stripe's code and decline code into the reason the
// rest of the platform stores and shows ("card_declined", "insufficient_funds",
// "expired_card", "authentication_failed"…).
func declineCode(code, decline string) string {
	switch code {
	case "payment_intent_authentication_failure", "authentication_failure":
		return "authentication_failed"
	}
	switch decline {
	case "", "generic_decline":
		if code == "" || code == "card_declined" {
			return "card_declined"
		}
		return code
	}
	return decline
}

// newDecline builds the platform decline for a Stripe card error.
func newDecline(code, decline string) *payment.DeclineError {
	reason := declineCode(code, decline)
	return &payment.DeclineError{Code: reason, Soft: softDeclines[reason] || softDeclines[code]}
}

// mapError turns a failed Stripe call into the error the platform
// understands. op names the operation for logs. Card errors become
// *payment.DeclineError; an expired, used or unknown token or payment method
// becomes payment.ErrInvalidToken; timeouts, connection errors, 5xx and 429
// become payment.ErrUnknownOutcome (retry with the same idempotency key,
// reconcile with Lookup); rejected credentials become ErrConfiguration.
func (p *Provider) mapError(op string, err error) error {
	if err == nil {
		return nil
	}
	var se *stripe.Error
	if !errors.As(err, &se) {
		// Not an answer from Stripe: a timeout, a reset connection or a cancelled
		// context. The request may or may not have been processed.
		return fmt.Errorf("%w: %s: %v", payment.ErrUnknownOutcome, op, redact(err))
	}
	switch {
	case se.HTTPStatusCode == http.StatusUnauthorized || se.Code == stripe.ErrorCodeAPIKeyExpired ||
		(se.HTTPStatusCode == http.StatusForbidden && se.Type != stripe.ErrorTypeCard):
		p.log.Error("stripe: the API key was refused (configuration error)", zap.String("op", op),
			zap.Int("status", se.HTTPStatusCode), zap.String("code", string(se.Code)), zap.String("request_id", se.RequestID))
		return fmt.Errorf("%w: %s: stripe answered %d", ErrConfiguration, op, se.HTTPStatusCode)
	case se.HTTPStatusCode == http.StatusTooManyRequests || se.Code == stripe.ErrorCodeRateLimit ||
		se.Code == stripe.ErrorCodeLockTimeout || se.HTTPStatusCode >= 500 || se.Type == stripe.ErrorTypeAPI:
		return fmt.Errorf("%w: %s: stripe answered %d %s", payment.ErrUnknownOutcome, op, se.HTTPStatusCode, se.Code)
	case se.Code == stripe.ErrorCodeIdempotencyKeyInUse:
		// Another request with the same key is still running.
		return fmt.Errorf("%w: %s: the request is still in progress at stripe", payment.ErrUnknownOutcome, op)
	case se.Type == stripe.ErrorTypeCard:
		return newDecline(string(se.Code), string(se.DeclineCode))
	case se.Code == stripe.ErrorCodeChargeDisputed:
		return payment.ErrChargeDisputed
	case se.Code == stripe.ErrorCodeChargeAlreadyRefunded || (se.Param == "amount" && strings.Contains(op, "refund")):
		return payment.ErrRefundTooLarge
	case se.Code == stripe.ErrorCodeResourceMissing && isTokenParam(se.Param):
		return payment.ErrInvalidToken
	case se.Code == stripe.ErrorCodeResourceMissing || se.HTTPStatusCode == http.StatusNotFound:
		return payment.ErrNotFound
	case se.Code == stripe.ErrorCodeTokenAlreadyUsed || se.Code == stripe.ErrorCodeTokenInUse || se.Code == stripe.ErrorCodeExpiredPaymentMethod ||
		(se.Type == stripe.ErrorTypeInvalidRequest && isTokenParam(se.Param)):
		return payment.ErrInvalidToken
	}
	return fmt.Errorf("stripe: %s: %s %s: %s", op, se.Type, se.Code, safeMessage(se))
}

// isTokenParam reports whether a Stripe error is about the token or payment
// method the browser sent.
func isTokenParam(param string) bool {
	return param == "confirmation_token" || param == "payment_method" || strings.HasPrefix(param, "payment_method_data")
}

// safeMessage is the Stripe error message limited to a log-sized string
// (Stripe's messages never include secrets or full card numbers).
func safeMessage(se *stripe.Error) string {
	m := se.Msg
	if len(m) > 200 {
		m = m[:200]
	}
	return m
}

// redact keeps transport errors short and free of URLs' query strings.
func redact(err error) string {
	m := err.Error()
	if i := strings.Index(m, "?"); i >= 0 {
		m = m[:i]
	}
	if len(m) > 200 {
		m = m[:200]
	}
	return m
}

// withKey sets the idempotency key (Stripe accepts at most 255 characters).
func withKey(params *stripe.Params, key string) {
	if key == "" {
		return
	}
	if len(key) > 255 {
		key = key[:255]
	}
	params.SetIdempotencyKey(key)
}

func timeUnix(sec int64) time.Time { return time.Unix(sec, 0) }
