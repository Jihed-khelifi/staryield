package config

import "testing"

// revisions 3.0: the sign-up offer is a site setting; the configuration value
// is only its fallback and defaults to 5 minutes.
func TestSignupFreeMinutesFallbackDefaultsToFive(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Billing.SignupFreeMinutes != 5 {
		t.Fatalf("default fallback: %d", cfg.Billing.SignupFreeMinutes)
	}
}
