package config

import (
	"strings"
	"testing"
)

// revisions 5.0: production refuses a minimum age below 18 (§24.6).
func TestProductionRefusesMinimumAgeBelow18(t *testing.T) {
	clearPlatformEnv(t)
	t.Setenv("STARYIELD_APP_ENV", "production")
	setProductionSecrets(t)
	t.Setenv("STARYIELD_SECURITY_COOKIE_SECURE", "true")

	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.MinimumAge != 18 {
		t.Fatalf("default minimum age: %d", cfg.Auth.MinimumAge)
	}
	for _, v := range []string{"17", "13", "0"} {
		t.Setenv("STARYIELD_AUTH_MINIMUM_AGE", v)
		if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "STARYIELD_AUTH_MINIMUM_AGE") {
			t.Fatalf("minimum age %s accepted in production: %v", v, err)
		}
	}
	t.Setenv("STARYIELD_AUTH_MINIMUM_AGE", "21")
	if _, err := Load(""); err != nil {
		t.Fatalf("a higher minimum age must be allowed: %v", err)
	}
}

// Outside production a lower value stays possible (local tests, fixtures).
func TestMinimumAgeBelow18AllowedOutsideProduction(t *testing.T) {
	clearPlatformEnv(t)
	t.Setenv("STARYIELD_AUTH_MINIMUM_AGE", "13")
	if _, err := Load(""); err != nil {
		t.Fatal(err)
	}
}
