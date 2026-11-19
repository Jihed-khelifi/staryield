package config

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// revisions 6.0 (§26.5): Stripe settings.

func stripeEnv(t *testing.T, secret, publishable, webhook string) {
	t.Helper()
	t.Setenv("STARYIELD_PAYMENTS_PROVIDERS", "stripe")
	t.Setenv("STARYIELD_PAYMENTS_DEFAULT_PROVIDER", "stripe")
	t.Setenv("STARYIELD_STRIPE_SECRET_KEY", secret)
	t.Setenv("STARYIELD_STRIPE_PUBLISHABLE_KEY", publishable)
	t.Setenv("STARYIELD_STRIPE_WEBHOOK_SECRET", webhook)
}

func TestStripeDefaultsAndVariables(t *testing.T) {
	clearPlatformEnv(t)
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Stripe.Timeout != 20*time.Second || cfg.Stripe.Configured() || cfg.Stripe.AllowTestInProduction || cfg.Stripe.APIBase != "" {
		t.Fatalf("stripe defaults: %+v", cfg.Stripe)
	}
	have := map[string]bool{}
	for _, v := range Variables() {
		have[v.Names[0]] = true
	}
	for _, name := range []string{"SECRET_KEY", "PUBLISHABLE_KEY", "WEBHOOK_SECRET", "API_BASE", "TIMEOUT", "ALLOW_TEST_IN_PRODUCTION"} {
		if !have["STARYIELD_STRIPE_"+name] {
			t.Errorf("STARYIELD_STRIPE_%s is not a setting", name)
		}
	}
}

func TestStripeProviderNeedsAllThreeKeys(t *testing.T) {
	clearPlatformEnv(t)
	t.Setenv("STARYIELD_PAYMENTS_PROVIDERS", "stripe")
	t.Setenv("STARYIELD_PAYMENTS_DEFAULT_PROVIDER", "stripe")
	_, err := Load("")
	if err == nil || !strings.Contains(err.Error(), "STARYIELD_STRIPE_SECRET_KEY, STARYIELD_STRIPE_PUBLISHABLE_KEY and STARYIELD_STRIPE_WEBHOOK_SECRET are required") {
		t.Fatalf("stripe listed without keys: %v", err)
	}
	t.Setenv("STARYIELD_STRIPE_SECRET_KEY", "sk_test_"+strings.Repeat("a", 24))
	if _, err = Load(""); err == nil || !strings.Contains(err.Error(), "are required") {
		t.Fatalf("one key is not enough: %v", err)
	}
	stripeEnv(t, "sk_test_"+strings.Repeat("a", 24), "pk_test_"+strings.Repeat("b", 24), "whsec_"+strings.Repeat("c", 24))
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("complete stripe settings: %v", err)
	}
	if !cfg.Stripe.Configured() || cfg.Stripe.Livemode() {
		t.Fatalf("stripe: %+v", cfg.Stripe)
	}
	// Restricted keys work like secret keys.
	stripeEnv(t, "rk_test_"+strings.Repeat("a", 24), "pk_test_"+strings.Repeat("b", 24), "whsec_"+strings.Repeat("c", 24))
	if _, err := Load(""); err != nil {
		t.Fatalf("restricted key: %v", err)
	}
	// Keys without the provider listed are validated but not required.
	clearPlatformEnv(t)
	t.Setenv("STARYIELD_STRIPE_SECRET_KEY", "sk_test_"+strings.Repeat("a", 24))
	if _, err := Load(""); err != nil {
		t.Fatalf("an unlisted provider needs no keys: %v", err)
	}
}

func TestStripeKeysAreCheckedForShapeAndMode(t *testing.T) {
	good := map[string]string{"secret": "sk_test_" + strings.Repeat("a", 24), "pub": "pk_test_" + strings.Repeat("b", 24), "hook": "whsec_" + strings.Repeat("c", 24)}
	cases := []struct {
		name, secret, pub, hook, want string
	}{
		{"publishable key as secret", good["pub"], good["pub"], good["hook"], "STARYIELD_STRIPE_SECRET_KEY must be a secret or restricted key"},
		{"junk secret", "nope", good["pub"], good["hook"], "STARYIELD_STRIPE_SECRET_KEY must be a secret or restricted key"},
		{"secret key as publishable", good["secret"], good["secret"], good["hook"], "STARYIELD_STRIPE_PUBLISHABLE_KEY must be a publishable key"},
		{"mixed modes", "sk_live_" + strings.Repeat("a", 24), good["pub"], good["hook"], "must be of the same mode"},
		{"webhook secret shape", good["secret"], good["pub"], "abcdef", "STARYIELD_STRIPE_WEBHOOK_SECRET must be a webhook signing secret"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearPlatformEnv(t)
			stripeEnv(t, tc.secret, tc.pub, tc.hook)
			if _, err := Load(""); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

func TestStripeProductionRefusesTestKeysAndTheTestAPIBase(t *testing.T) {
	prod := func(t *testing.T) {
		t.Helper()
		clearPlatformEnv(t)
		t.Setenv("STARYIELD_APP_ENV", "production")
		setProductionSecrets(t)
		t.Setenv("STARYIELD_SECURITY_COOKIE_SECURE", "true")
		t.Setenv("STARYIELD_PAYMENTS_PROVIDER", "stripe")
		t.Setenv("STARYIELD_PAYMENTS_PROVIDERS", "stripe")
		t.Setenv("STARYIELD_PAYMENTS_DEFAULT_PROVIDER", "stripe")
	}
	prod(t)
	t.Setenv("STARYIELD_STRIPE_SECRET_KEY", "sk_test_"+strings.Repeat("a", 24))
	t.Setenv("STARYIELD_STRIPE_PUBLISHABLE_KEY", "pk_test_"+strings.Repeat("b", 24))
	t.Setenv("STARYIELD_STRIPE_WEBHOOK_SECRET", "whsec_"+strings.Repeat("c", 24))
	_, err := Load("")
	if err == nil || !strings.Contains(err.Error(), "production refuses them unless STARYIELD_STRIPE_ALLOW_TEST_IN_PRODUCTION=true") {
		t.Fatalf("test keys in production: %v", err)
	}
	t.Setenv("STARYIELD_STRIPE_ALLOW_TEST_IN_PRODUCTION", "true")
	if _, err := Load(""); err != nil {
		t.Fatalf("explicit opt-in: %v", err)
	}

	prod(t)
	t.Setenv("STARYIELD_STRIPE_SECRET_KEY", "sk_live_"+strings.Repeat("a", 24))
	t.Setenv("STARYIELD_STRIPE_PUBLISHABLE_KEY", "pk_live_"+strings.Repeat("b", 24))
	t.Setenv("STARYIELD_STRIPE_WEBHOOK_SECRET", "whsec_"+strings.Repeat("c", 24))
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("live keys in production: %v", err)
	}
	if !cfg.Stripe.Livemode() {
		t.Fatal("live keys must report livemode")
	}
	t.Setenv("STARYIELD_STRIPE_API_BASE", "http://127.0.0.1:9999")
	if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "STARYIELD_STRIPE_API_BASE is for tests only") {
		t.Fatalf("api base in production: %v", err)
	}
}

func TestStripeAPIBaseAndTimeoutAreValidated(t *testing.T) {
	for _, base := range []string{"ftp://x", "http://stripe.example.com", "https://user:pw@api.example.com", "https://a.example.com?x=1", "not a url"} {
		clearPlatformEnv(t)
		t.Setenv("STARYIELD_STRIPE_API_BASE", base)
		if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "STARYIELD_STRIPE_API_BASE") {
			t.Errorf("api base %q accepted: %v", base, err)
		}
	}
	for _, base := range []string{"http://127.0.0.1:12111", "http://localhost:12111", "https://api.stripe.com"} {
		clearPlatformEnv(t)
		t.Setenv("STARYIELD_STRIPE_API_BASE", base)
		if _, err := Load(""); err != nil {
			t.Errorf("api base %q refused: %v", base, err)
		}
	}
	for _, d := range []string{"1s", "2m"} {
		clearPlatformEnv(t)
		t.Setenv("STARYIELD_STRIPE_TIMEOUT", d)
		if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "STARYIELD_STRIPE_TIMEOUT") {
			t.Errorf("timeout %s accepted: %v", d, err)
		}
	}
}

func TestStripeConfigNeverPrintsItsSecrets(t *testing.T) {
	s := StripeConfig{SecretKey: "sk_live_supersecret", PublishableKey: "pk_live_public", WebhookSecret: "whsec_supersecret", Timeout: time.Second}
	for _, out := range []string{fmt.Sprint(s), fmt.Sprintf("%+v", s), fmt.Sprintf("%#v", s), fmt.Sprintf("%v", &s)} {
		if strings.Contains(out, "supersecret") {
			t.Fatalf("secret printed: %s", out)
		}
		if !strings.Contains(out, "pk_live_public") {
			t.Fatalf("the publishable key is not secret: %s", out)
		}
	}
	cfg, _ := Load("")
	cfg.Stripe = s
	if out := fmt.Sprintf("%+v", cfg); strings.Contains(out, "supersecret") {
		t.Fatalf("secret printed through the whole configuration: %s", out)
	}
}

func TestStripePaymentsProviderValueNeedsStripeListed(t *testing.T) {
	clearPlatformEnv(t)
	t.Setenv("STARYIELD_PAYMENTS_PROVIDER", "stripe")
	if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "STARYIELD_PAYMENTS_PROVIDER=stripe requires STARYIELD_PAYMENTS_PROVIDERS to list stripe") {
		t.Fatalf("provider=stripe without the provider listed: %v", err)
	}
	stripeEnv(t, "sk_test_"+strings.Repeat("a", 24), "pk_test_"+strings.Repeat("b", 24), "whsec_"+strings.Repeat("c", 24))
	if _, err := Load(""); err != nil {
		t.Fatalf("provider=stripe: %v", err)
	}
}
