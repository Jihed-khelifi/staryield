package config

import "testing"

// revisions 3.0: the sign-up offer is a site setting; the configuration value
// is only its fallback and defaults to 4 minutes (revisions 5.0: the pricing spec's trial).
func TestSignupFreeMinutesFallbackDefaultsToFour(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Billing.SignupFreeMinutes != 4 {
		t.Fatalf("default fallback: %d", cfg.Billing.SignupFreeMinutes)
	}
}
