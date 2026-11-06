package config

import (
	"strings"
	"testing"
)

func TestCloudflareMailInProduction(t *testing.T) {
	clearPlatformEnv(t)
	t.Setenv("STARYIELD_APP_ENV", "production")
	setProductionSecrets(t)
	t.Setenv("STARYIELD_SECURITY_COOKIE_SECURE", "true")
	t.Setenv("STARYIELD_MAIL_DRIVER", "cloudflare")
	_, err := Load("")
	if err == nil || !strings.Contains(err.Error(), "STARYIELD_MAIL_CLOUDFLARE_ACCOUNT_ID") || !strings.Contains(err.Error(), "STARYIELD_MAIL_CLOUDFLARE_API_TOKEN") {
		t.Fatalf("cloudflare without credentials: %v", err)
	}
	t.Setenv("STARYIELD_MAIL_CLOUDFLARE_ACCOUNT_ID", "acct")
	t.Setenv("STARYIELD_MAIL_CLOUDFLARE_API_TOKEN", "token")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mail.Driver != "cloudflare" || cfg.Mail.DailyCap != 20 || cfg.Mail.MaxAttempts != 8 {
		t.Fatalf("mail config: %+v", cfg.Mail)
	}
	// Transactional-only provider: marketing must stay off.
	t.Setenv("STARYIELD_MAIL_ALLOW_MARKETING", "true")
	if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "STARYIELD_MAIL_ALLOW_MARKETING") {
		t.Fatalf("marketing through cloudflare accepted: %v", err)
	}
}

func TestMailValidation(t *testing.T) {
	clearPlatformEnv(t)
	cases := map[string]string{
		"STARYIELD_MAIL_DRIVER":              "smtp",
		"STARYIELD_MAIL_FROM":                "not an address",
		"STARYIELD_MAIL_REPLY_TO":            "nope",
		"STARYIELD_MAIL_MAX_ATTEMPTS":        "0",
		"STARYIELD_MAIL_DAILY_CAP":           "0",
		"STARYIELD_MAIL_RETENTION":           "1h",
		"STARYIELD_MAIL_CLOUDFLARE_BASE_URL": "ftp://example.com",
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, value)
			if _, err := Load(""); err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("%s=%s accepted: %v", name, value, err)
			}
		})
	}
}

func TestPushValidation(t *testing.T) {
	clearPlatformEnv(t)
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Push.Active() {
		t.Fatal("push must be inactive without keys")
	}
	t.Setenv("STARYIELD_PUSH_VAPID_PUBLIC_KEY", "only-one-key")
	if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "STARYIELD_PUSH_VAPID_PRIVATE_KEY") {
		t.Fatalf("half a key pair accepted: %v", err)
	}
	t.Setenv("STARYIELD_PUSH_VAPID_PRIVATE_KEY", "other-key")
	t.Setenv("STARYIELD_PUSH_SUBJECT", "support@staryield.net")
	if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "STARYIELD_PUSH_SUBJECT") {
		t.Fatalf("subject without scheme accepted: %v", err)
	}
	t.Setenv("STARYIELD_PUSH_SUBJECT", "mailto:support@staryield.net")
	t.Setenv("STARYIELD_PUSH_MAX_PER_USER", "500")
	if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "STARYIELD_PUSH_MAX_PER_USER") {
		t.Fatalf("absurd per-user limit accepted: %v", err)
	}
}
