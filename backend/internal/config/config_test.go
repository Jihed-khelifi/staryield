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
	for _, want := range []string{"STARYIELD_AUTH_JWT_SECRET", "STARYIELD_SECURITY_COOKIE_SECURE", "STARYIELD_PAYMENTS_PROVIDER=sandbox",
		"STARYIELD_STORAGE_DRIVER=memory", "STARYIELD_MAIL_DRIVER=log", "https"} {
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
	for _, want := range []string{"'*'", "CIDR", "STARYIELD_BILLING_PSYCHIC_SHARE_PERCENT", "STARYIELD_REALTIME_PING_INTERVAL"} {
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
		"STARYIELD_DATABASE_DSN", "STARYIELD_REDIS_URL", "STARYIELD_APP_ENV", "STARYIELD_HTTP_ADDR", "STARYIELD_APP_INSTANCE_ID",
		"R2_ADVISOR_IMAGE", "R2_ADVISOR_IMAGE_PUBLIC_URL", "R2_CHAT_IMAGES_PRIVATE", "STARYIELD_STORAGE_ADVISOR_BUCKET",
		"STARYIELD_STORAGE_CHAT_BUCKET", "STARYIELD_STORAGE_ADVISOR_PUBLIC_BASE_URL"} {
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
	if err == nil || !strings.Contains(err.Error(), "STARYIELD_AUTH_JWT_SECRET") {
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
	if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "STARYIELD_HTTP_TRUSTED_PROXIES") {
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
	t.Setenv("STARYIELD_SECURITY_DATA_ENCRYPTION_KEY", strings.Repeat("ab", 32))
	t.Setenv("STARYIELD_SECURITY_ADMIN_ORIGINS", "https://admin.staryield.net")
}

func TestR2BucketsComeFromDotEnvNames(t *testing.T) {
	clearPlatformEnv(t)
	t.Setenv("R2_ADVISOR_IMAGE", "staryield-advisors")
	t.Setenv("R2_ADVISOR_IMAGE_PUBLIC_URL", "https://advisorsassets.example.net/")
	t.Setenv("R2_CHAT_IMAGES_PRIVATE", "staryield-chat-private")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storage.AdvisorBucket != "staryield-advisors" || cfg.Storage.ChatBucket != "staryield-chat-private" {
		t.Fatalf("buckets: %q %q", cfg.Storage.AdvisorBucket, cfg.Storage.ChatBucket)
	}
	if cfg.Storage.AdvisorPublicBaseURL != "https://advisorsassets.example.net" {
		t.Fatalf("advisor base url: %q", cfg.Storage.AdvisorPublicBaseURL)
	}
}

func TestDotEnvFileDoesNotOverrideEnvironment(t *testing.T) {
	clearPlatformEnv(t)
	dir := t.TempDir()
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	env := "# comment\nexport R2_CHAT_IMAGES_PRIVATE=\"from-file\"\nR2_ADVISOR_IMAGE='advisors-file'\n"
	if err := os.WriteFile(".env", []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("R2_ADVISOR_IMAGE", "from-env")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Unsetenv("R2_CHAT_IMAGES_PRIVATE") })
	if cfg.Storage.ChatBucket != "from-file" || cfg.Storage.AdvisorBucket != "from-env" {
		t.Fatalf("dotenv: chat=%q advisor=%q", cfg.Storage.ChatBucket, cfg.Storage.AdvisorBucket)
	}
}

func TestEncryptionKeyFormatsAndAdminPolicy(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if k, err := cfg.EncryptionKey(); err != nil || len(k) != 32 {
		t.Fatalf("dev key: %v", err)
	}
	cfg.Security.DataEncryptionKey = "c2hvcnQ="
	if _, err := cfg.EncryptionKey(); err == nil {
		t.Fatal("short key accepted")
	}
	cfg.Security.DataEncryptionKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	if k, err := cfg.EncryptionKey(); err != nil || len(k) != 32 {
		t.Fatalf("base64 key: %v", err)
	}
	cfg.Admin.IdleTimeout = cfg.Admin.SessionTTL * 2
	cfg.Security.AdminOrigins = []string{"*"}
	cfg.Security.AdminIPAllowlist = []string{"not-a-cidr"}
	err = cfg.Validate()
	for _, want := range []string{"STARYIELD_ADMIN_IDLE_TIMEOUT", "STARYIELD_SECURITY_ADMIN_ORIGINS", "STARYIELD_SECURITY_ADMIN_IP_ALLOWLIST"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q error, got %v", want, err)
		}
	}
}

// The production deployment must say whether the key is missing or has the
// wrong length, without echoing the key.
func TestEncryptionKeyErrorsExplainTheFix(t *testing.T) {
	clearPlatformEnv(t)
	t.Setenv("STARYIELD_APP_ENV", "production")
	setProductionSecrets(t)
	t.Setenv("STARYIELD_SECURITY_COOKIE_SECURE", "true")
	cases := map[string]struct{ key, want string }{
		"missing":       {"", "STARYIELD_SECURITY_DATA_ENCRYPTION_KEY is required: 32 random bytes as 64 hex characters (openssl rand -hex 32)"},
		"48-byte hex":   {strings.Repeat("ab", 48), "got 48 bytes (96 hex characters)"},
		"16-byte b64":   {"MDEyMzQ1Njc4OWFiY2RlZg==", "got 16 bytes of base64"},
		"not encoded":   {"correct horse battery staple!", "must be 32 bytes as 64 hex characters or base64 (29 characters given)"},
		"valid hex key": {strings.Repeat("0f", 32), ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("STARYIELD_SECURITY_DATA_ENCRYPTION_KEY", c.key)
			if c.key == "" {
				os.Unsetenv("STARYIELD_SECURITY_DATA_ENCRYPTION_KEY")
			}
			_, err := Load("")
			if c.want == "" {
				if err != nil {
					t.Fatalf("valid key rejected: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want %q, got %v", c.want, err)
			}
			if c.key != "" && strings.Contains(err.Error(), c.key) {
				t.Fatalf("error echoes the key: %v", err)
			}
		})
	}
}
