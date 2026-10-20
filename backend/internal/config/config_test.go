package config

import (
	"os"
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

// clearPlatformEnv isolates a test from variables the developer's shell or
// the Makefile may export.
func clearPlatformEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"DATABASE_URL", "REDIS_URL", "APP_ENV", "PORT", "RAILWAY_ENVIRONMENT_ID", "RAILWAY_REPLICA_ID",
		"STARYIELD_DATABASE_DSN", "STARYIELD_REDIS_URL", "STARYIELD_APP_ENV", "STARYIELD_HTTP_ADDR", "STARYIELD_APP_INSTANCE_ID"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

func TestPlatformEnvironmentVariables(t *testing.T) {
	clearPlatformEnv(t)
	t.Setenv("DATABASE_URL", "postgresql://u:p@postgres.railway.internal:5432/railway")
	t.Setenv("REDIS_URL", "redis://default:p@redis.railway.internal:6379")
	t.Setenv("PORT", "3333")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.DSN != "postgresql://u:p@postgres.railway.internal:5432/railway" {
		t.Errorf("DATABASE_URL not used: %q", cfg.Database.DSN)
	}
	if cfg.Redis.URL != "redis://default:p@redis.railway.internal:6379" {
		t.Errorf("REDIS_URL not used: %q", cfg.Redis.URL)
	}
	if cfg.HTTP.Addr != ":3333" {
		t.Errorf("PORT not used: %q", cfg.HTTP.Addr)
	}
}

func TestStaryieldVariablesWinOverPlatformOnes(t *testing.T) {
	clearPlatformEnv(t)
	t.Setenv("DATABASE_URL", "postgres://platform/db")
	t.Setenv("STARYIELD_DATABASE_DSN", "postgres://explicit/db")
	t.Setenv("REDIS_URL", "redis://platform:6379")
	t.Setenv("STARYIELD_REDIS_URL", "redis://explicit:6379")
	t.Setenv("PORT", "3333")
	t.Setenv("STARYIELD_HTTP_ADDR", "127.0.0.1:9000")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.DSN != "postgres://explicit/db" || cfg.Redis.URL != "redis://explicit:6379" || cfg.HTTP.Addr != "127.0.0.1:9000" {
		t.Fatalf("explicit values must win: %q %q %q", cfg.Database.DSN, cfg.Redis.URL, cfg.HTTP.Addr)
	}
}

func TestInvalidPortIsRejected(t *testing.T) {
	clearPlatformEnv(t)
	t.Setenv("PORT", "eighty")
	if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "PORT") {
		t.Fatalf("expected PORT error, got %v", err)
	}
}

func TestRailwayDefaultsToHardenedProduction(t *testing.T) {
	clearPlatformEnv(t)
	t.Setenv("RAILWAY_ENVIRONMENT_ID", "env-123")
	t.Setenv("RAILWAY_REPLICA_ID", "replica-abc")
	// Without secrets the production checks must refuse to boot rather than
	// fall back to development secrets.
	_, err := Load("")
	if err == nil || !strings.Contains(err.Error(), "jwt_secret") {
		t.Fatalf("railway must default to production hardening, got %v", err)
	}

	setProductionSecrets(t)
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.IsProduction() || !cfg.Security.CookieSecure || !cfg.Security.HSTS || cfg.Log.Format != "json" {
		t.Errorf("railway defaults not applied: env=%s secure=%v hsts=%v log=%s", cfg.App.Env, cfg.Security.CookieSecure, cfg.Security.HSTS, cfg.Log.Format)
	}
	if cfg.HTTP.IPSource != "x-real-ip" || len(cfg.HTTP.TrustedProxies) == 0 {
		t.Errorf("railway ip source: %s %v", cfg.HTTP.IPSource, cfg.HTTP.TrustedProxies)
	}
	if cfg.App.InstanceID != "replica-abc" {
		t.Errorf("instance id: %q", cfg.App.InstanceID)
	}

	// An explicit environment still wins.
	t.Setenv("STARYIELD_APP_ENV", "development")
	if cfg, err := Load(""); err != nil || cfg.IsProduction() {
		t.Fatalf("explicit app env must win: %v", err)
	}
}

func TestProductionAcceptsDisabledFeatures(t *testing.T) {
	clearPlatformEnv(t)
	t.Setenv("STARYIELD_APP_ENV", "production")
	setProductionSecrets(t)
	t.Setenv("STARYIELD_SECURITY_COOKIE_SECURE", "true")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storage.Driver != "disabled" || cfg.Payments.Provider != "disabled" {
		t.Fatalf("drivers: %s %s", cfg.Storage.Driver, cfg.Payments.Provider)
	}
}

func TestSpoofableIPSourcesRequireTrustedProxies(t *testing.T) {
	clearPlatformEnv(t)
	t.Setenv("STARYIELD_HTTP_IP_SOURCE", "x-real-ip")
	if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "trusted_proxies") {
		t.Fatalf("expected trusted_proxies error, got %v", err)
	}
}

func setProductionSecrets(t *testing.T) {
	t.Helper()
	t.Setenv("STARYIELD_AUTH_JWT_SECRET", strings.Repeat("j", 48))
	t.Setenv("STARYIELD_SECURITY_CSRF_SECRET", strings.Repeat("c", 48))
	t.Setenv("STARYIELD_ANALYTICS_IP_HASH_SECRET", strings.Repeat("i", 32))
	t.Setenv("STARYIELD_PAYMENTS_WEBHOOK_SECRET", strings.Repeat("w", 32))
	t.Setenv("STARYIELD_SECURITY_ALLOWED_ORIGINS", "https://staryield.net")
	t.Setenv("STARYIELD_STORAGE_DRIVER", "disabled")
	t.Setenv("STARYIELD_PAYMENTS_PROVIDER", "disabled")
	t.Setenv("STARYIELD_MAIL_DRIVER", "noop")
}
