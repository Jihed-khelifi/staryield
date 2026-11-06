package config

import (
	"strings"
	"testing"
	"time"
)

func TestBillingProviderSettingsAreValidated(t *testing.T) {
	clearPlatformEnv(t)
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Payments.DefaultProvider != "sandbox_a" || len(cfg.Payments.Providers) != 2 || cfg.Payments.RenewalLease != 5*time.Minute {
		t.Fatalf("billing defaults: %+v", cfg.Payments)
	}
	cfg.Payments.Providers = []string{"sandbox_b", "stripe"}
	cfg.Payments.RenewalLease = 30 * time.Second
	cfg.Payments.MaxPaymentMethods = 0
	err = cfg.Validate()
	for _, want := range []string{`STARYIELD_PAYMENTS_PROVIDERS "stripe" is not supported`, "STARYIELD_PAYMENTS_DEFAULT_PROVIDER must be one of",
		"STARYIELD_PAYMENTS_RENEWAL_LEASE", "STARYIELD_PAYMENTS_MAX_PAYMENT_METHODS"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q in %v", want, err)
		}
	}
}

func TestSandboxBillingProvidersNeedTheProductionOptIn(t *testing.T) {
	clearPlatformEnv(t)
	t.Setenv("STARYIELD_APP_ENV", "production")
	setProductionSecrets(t)
	t.Setenv("STARYIELD_SECURITY_COOKIE_SECURE", "true")
	t.Setenv("STARYIELD_PAYMENTS_PROVIDER", "sandbox")
	_, err := Load("")
	if err == nil || !strings.Contains(err.Error(), "STARYIELD_PAYMENTS_PROVIDERS=sandbox_a is not allowed in production") {
		t.Fatalf("sandbox saved-card providers accepted in production: %v", err)
	}
	t.Setenv("STARYIELD_PAYMENTS_ALLOW_SANDBOX_IN_PRODUCTION", "true")
	if _, err := Load(""); err != nil {
		t.Fatalf("explicit opt-in: %v", err)
	}
}
