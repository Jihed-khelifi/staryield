package config

import (
	"strings"
	"testing"
)

func TestLoadDefaultsAndEnv(t *testing.T) {
	t.Setenv("STARYIELD_HTTP_ADDR", ":9999")
	t.Setenv("STARYIELD_SECURITY_ALLOWED_ORIGINS", "https://a.test, https://b.test")
	t.Setenv("STARYIELD_CHAT_GRACE_PERIOD", "2m")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.Addr != ":9999" || len(cfg.Security.AllowedOrigins) != 2 || cfg.Chat.GracePeriod.Minutes() != 2 {
		t.Fatalf("env overrides: %+v %+v", cfg.HTTP, cfg.Security.AllowedOrigins)
	}
	if !strings.HasPrefix(cfg.Auth.JWTSecret, "dev-only") {
		t.Fatal("development secret should be derived")
	}
}

func TestProductionHardening(t *testing.T) {
	t.Setenv("STARYIELD_APP_ENV", "production")
	_, err := Load("")
	if err == nil {
		t.Fatal("production without secrets must fail")
	}
	for _, want := range []string{"jwt_secret", "cookie_secure", "sandbox", "storage.driver memory", "mail.driver log", "https"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in: %v", want, err)
		}
	}
}

func TestValidateRejectsUnsafeValues(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Security.AllowedOrigins = []string{"*"}
	cfg.HTTP.TrustedProxies = []string{"not-a-cidr"}
	cfg.Billing.PsychicSharePercent = 150
	cfg.Realtime.PingInterval = cfg.Realtime.PongWait
	err = cfg.Validate()
	for _, want := range []string{"'*'", "CIDR", "psychic_share_percent", "ping_interval"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q in %v", want, err)
		}
	}
}
