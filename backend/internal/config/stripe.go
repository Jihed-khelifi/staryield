package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// revisions 6.0 (docs/platform-extensions.md §26.5): Stripe card payments.

// StripeProvider is the provider name listed in STARYIELD_PAYMENTS_PROVIDERS.
const StripeProvider = "stripe"

// Stripe request timeout bounds (STARYIELD_STRIPE_TIMEOUT).
const (
	StripeTimeoutMin = 2 * time.Second
	StripeTimeoutMax = time.Minute
)

// StripeConfig configures the Stripe adapter. The provider is registered when
// STARYIELD_PAYMENTS_PROVIDERS lists "stripe"; the three keys are then
// required, of the same mode (live or test).
type StripeConfig struct {
	// SecretKey is the secret or restricted API key (sk_… / rk_…). Never logged.
	SecretKey string `env:"secret_key"`
	// PublishableKey is the key Stripe.js loads with (pk_…); sent to the browser.
	PublishableKey string `env:"publishable_key"`
	// WebhookSecret is the signing secret of the webhook endpoint (whsec_…). Never logged.
	WebhookSecret string `env:"webhook_secret"`
	// APIBase replaces https://api.stripe.com (tests only: a fake Stripe).
	// Refused in production.
	APIBase string `env:"api_base"`
	// Timeout bounds every Stripe request.
	Timeout time.Duration `env:"timeout"`
	// AllowTestInProduction lets a production deployment run with test keys
	// (a dress rehearsal); without it production refuses them.
	AllowTestInProduction bool `env:"allow_test_in_production"`
}

// Configured reports whether every key is set.
func (s StripeConfig) Configured() bool {
	return s.SecretKey != "" && s.PublishableKey != "" && s.WebhookSecret != ""
}

// Livemode reports whether the keys are live-mode keys.
func (s StripeConfig) Livemode() bool { return keyMode(s.SecretKey) == "live" }

// keyMode returns "live" or "test" for a Stripe key ("sk_live_…", "rk_test_…"),
// "" when it has neither shape.
func keyMode(key string) string {
	switch {
	case strings.HasPrefix(key, "sk_live_"), strings.HasPrefix(key, "rk_live_"), strings.HasPrefix(key, "pk_live_"):
		return "live"
	case strings.HasPrefix(key, "sk_test_"), strings.HasPrefix(key, "rk_test_"), strings.HasPrefix(key, "pk_test_"):
		return "test"
	}
	return ""
}

// String redacts the secrets, so the configuration can be printed or logged.
func (s StripeConfig) String() string {
	redact := func(v string) string {
		if v == "" {
			return ""
		}
		return "[redacted]"
	}
	return fmt.Sprintf("{SecretKey:%s PublishableKey:%s WebhookSecret:%s APIBase:%s Timeout:%s AllowTestInProduction:%t}",
		redact(s.SecretKey), s.PublishableKey, redact(s.WebhookSecret), s.APIBase, s.Timeout, s.AllowTestInProduction)
}

// GoString implements fmt.GoStringer (the %#v form) with the same redaction.
func (s StripeConfig) GoString() string { return "config.StripeConfig" + s.String() }

// validateStripe checks the Stripe settings (revisions 6.0, §26.5).
func (c *Config) validateStripe(add func(string, ...any)) {
	e := EnvVar
	s := c.Stripe
	listed := false
	if c.Payments.Provider != "disabled" {
		for _, p := range c.Payments.Providers {
			if p == StripeProvider {
				listed = true
			}
		}
	}
	anySet := s.SecretKey != "" || s.PublishableKey != "" || s.WebhookSecret != ""
	if listed && !s.Configured() {
		add("%s lists %s: %s, %s and %s are required", e("payments.providers"), StripeProvider,
			e("stripe.secret_key"), e("stripe.publishable_key"), e("stripe.webhook_secret"))
	}
	if s.Timeout < StripeTimeoutMin || s.Timeout > StripeTimeoutMax {
		add("%s must be within %s..%s", e("stripe.timeout"), StripeTimeoutMin, StripeTimeoutMax)
	}
	if anySet {
		secretMode, publishableMode := keyMode(s.SecretKey), keyMode(s.PublishableKey)
		if s.SecretKey != "" && (secretMode == "" || strings.HasPrefix(s.SecretKey, "pk_")) {
			add("%s must be a secret or restricted key (sk_live_…, sk_test_…, rk_live_… or rk_test_…)", e("stripe.secret_key"))
		}
		if s.PublishableKey != "" && (publishableMode == "" || !strings.HasPrefix(s.PublishableKey, "pk_")) {
			add("%s must be a publishable key (pk_live_… or pk_test_…)", e("stripe.publishable_key"))
		}
		if s.SecretKey != "" && s.PublishableKey != "" && secretMode != "" && publishableMode != "" && secretMode != publishableMode {
			add("%s and %s must be of the same mode (both live or both test)", e("stripe.secret_key"), e("stripe.publishable_key"))
		}
		if s.WebhookSecret != "" && !strings.HasPrefix(s.WebhookSecret, "whsec_") {
			add("%s must be a webhook signing secret (whsec_…)", e("stripe.webhook_secret"))
		}
		if c.IsProduction() && secretMode == "test" && !s.AllowTestInProduction {
			add("%s are test-mode keys: production refuses them unless %s=true", e("stripe.secret_key"), e("stripe.allow_test_in_production"))
		}
	}
	if s.APIBase != "" {
		u, err := url.Parse(s.APIBase)
		switch {
		case err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "":
			add("%s must be an absolute URL without credentials, query or fragment", e("stripe.api_base"))
		case c.IsProduction():
			add("%s is for tests only and is refused in production", e("stripe.api_base"))
		case u.Scheme != "https" && !(u.Scheme == "http" && isLoopbackHost(u.Hostname())):
			add("%s must be https (http is allowed only for a loopback test server)", e("stripe.api_base"))
		}
	}
}
