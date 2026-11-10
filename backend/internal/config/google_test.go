package config

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func googleConfig(t *testing.T, mutate func(*Config)) error {
	t.Helper()
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	mutate(cfg)
	return cfg.Validate()
}

func configured(c *Config) {
	c.Google.ClientID = "id.apps.example"
	c.Google.ClientSecret = "super-secret"
	c.Google.RedirectURL = "https://api.staryield.net/api/v1/auth/google/callback"
}

func TestGoogleDefaultsAndEnabled(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Google.Issuer != "https://accounts.google.com" || cfg.Google.StateTTL != 10*time.Minute || cfg.Google.Enabled() {
		t.Fatalf("defaults: %+v", cfg.Google)
	}
	configured(cfg)
	if !cfg.Google.Enabled() || cfg.Validate() != nil {
		t.Fatalf("a full configuration is valid and enabled: %v", cfg.Validate())
	}
	for name, clear := range map[string]func(*Config){
		"id":     func(c *Config) { c.Google.ClientID = "" },
		"secret": func(c *Config) { c.Google.ClientSecret = "" },
		"redirect": func(c *Config) {
			c.Google.RedirectURL = ""
		},
	} {
		c2, _ := Load("")
		configured(c2)
		clear(c2)
		if c2.Google.Enabled() {
			t.Errorf("without the %s the feature is off", name)
		}
		if err := c2.Validate(); err == nil || !strings.Contains(err.Error(), "STARYIELD_GOOGLE_CLIENT_ID") {
			t.Errorf("a partial configuration (%s) must be refused: %v", name, err)
		}
	}
}

func TestGoogleEnvVariables(t *testing.T) {
	t.Setenv("STARYIELD_GOOGLE_CLIENT_ID", "abc")
	t.Setenv("STARYIELD_GOOGLE_CLIENT_SECRET", "def")
	t.Setenv("STARYIELD_GOOGLE_REDIRECT_URL", "https://api.example.test/cb")
	t.Setenv("STARYIELD_GOOGLE_STATE_TTL", "5m")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Google.Enabled() || cfg.Google.StateTTL != 5*time.Minute || cfg.Google.ClientID != "abc" {
		t.Fatalf("%+v", cfg.Google)
	}
}

func TestGoogleRedirectURLRules(t *testing.T) {
	for _, tc := range []struct {
		name, url, env string
		ok             bool
	}{
		{"https in production", "https://api.staryield.net/api/v1/auth/google/callback", EnvProduction, true},
		{"https in development", "https://api.staryield.net/cb", EnvDevelopment, true},
		{"http localhost in development", "http://localhost:8080/api/v1/auth/google/callback", EnvDevelopment, true},
		{"http loopback ip in test", "http://127.0.0.1:8080/cb", EnvTest, true},
		{"http IPv6 loopback in development", "http://[::1]:8080/cb", EnvDevelopment, true},
		{"http localhost in production", "http://localhost:8080/cb", EnvProduction, false},
		{"http public host in development", "http://api.staryield.net/cb", EnvDevelopment, false},
		{"http public host in production", "http://api.staryield.net/cb", EnvProduction, false},
		{"a host that merely starts with localhost", "http://localhost.evil.example/cb", EnvDevelopment, false},
		{"other scheme", "ftp://api.staryield.net/cb", EnvDevelopment, false},
		{"javascript", "javascript:alert(1)", EnvDevelopment, false},
		{"relative", "/api/v1/auth/google/callback", EnvDevelopment, false},
		{"credentials", "https://user:pw@api.staryield.net/cb", EnvDevelopment, false},
		{"query", "https://api.staryield.net/cb?x=1", EnvDevelopment, false},
		{"fragment", "https://api.staryield.net/cb#x", EnvDevelopment, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := googleConfig(t, func(c *Config) {
				configured(c)
				c.Google.RedirectURL = tc.url
				if tc.env == EnvProduction {
					productionBasics(c)
				}
				c.App.Env = tc.env
			})
			if tc.ok && err != nil {
				t.Fatalf("want valid: %v", err)
			}
			if !tc.ok && (err == nil || !strings.Contains(err.Error(), "STARYIELD_GOOGLE_REDIRECT_URL")) {
				t.Fatalf("want a redirect URL error: %v", err)
			}
		})
	}
}

// productionBasics satisfies the production checks unrelated to Google.
func productionBasics(c *Config) {
	c.Auth.JWTSecret = strings.Repeat("j", 40)
	c.Security.CSRFSecret = strings.Repeat("c", 40)
	c.Security.CookieSecure = true
	c.Security.AllowedOrigins = []string{"https://staryield.net"}
	c.Security.AdminOrigins = []string{"https://admin.staryield.net"}
	c.Analytics.IPHashSecret = strings.Repeat("h", 32)
	c.Payments.Provider = "disabled"
	c.Payments.WebhookSecret = strings.Repeat("w", 32)
	c.Storage.Driver = "disabled"
	c.Mail.Driver = "noop"
	c.Security.DataEncryptionKey = strings.Repeat("ab", 32)
}

func TestGoogleIssuerRules(t *testing.T) {
	for _, tc := range []struct {
		name, issuer, env string
		ok                bool
	}{
		{"google in production", "https://accounts.google.com", EnvProduction, true},
		{"google in development", "https://accounts.google.com", EnvDevelopment, true},
		{"a mock issuer in test", "http://127.0.0.1:9999", EnvTest, true},
		{"a mock https issuer in development", "https://idp.local.test", EnvDevelopment, true},
		{"a mock issuer in production", "http://127.0.0.1:9999", EnvProduction, false},
		{"a different https issuer in production", "https://idp.example.com", EnvProduction, false},
		{"google with a trailing slash in production", "https://accounts.google.com/", EnvProduction, false},
		{"public http issuer in test", "http://accounts.example.com", EnvTest, false},
		{"empty", "", EnvDevelopment, false},
		{"query", "https://accounts.google.com?x=1", EnvDevelopment, false},
		{"credentials", "https://u:p@accounts.google.com", EnvDevelopment, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := googleConfig(t, func(c *Config) {
				if tc.env == EnvProduction {
					productionBasics(c)
				}
				c.App.Env = tc.env
				c.Google.Issuer = tc.issuer
			})
			if tc.ok && err != nil {
				t.Fatalf("want valid: %v", err)
			}
			if !tc.ok && (err == nil || !strings.Contains(err.Error(), "STARYIELD_GOOGLE_ISSUER")) {
				t.Fatalf("want an issuer error: %v", err)
			}
		})
	}
}

func TestGoogleStateTTLRange(t *testing.T) {
	for ttl, ok := range map[time.Duration]bool{30 * time.Second: false, time.Minute: true, 10 * time.Minute: true, time.Hour: true,
		2 * time.Hour: false, 0: false} {
		err := googleConfig(t, func(c *Config) { c.Google.StateTTL = ttl })
		if (err == nil) != ok {
			t.Errorf("state ttl %v: %v", ttl, err)
		}
	}
}

func TestGoogleSecretIsNeverPrinted(t *testing.T) {
	cfg, _ := Load("")
	configured(cfg)
	for _, s := range []string{fmt.Sprintf("%v", cfg.Google), fmt.Sprintf("%+v", cfg.Google), fmt.Sprintf("%#v", cfg.Google),
		fmt.Sprintf("%v", *cfg), fmt.Sprintf("%+v", *cfg), fmt.Sprint(cfg.Google), cfg.Google.String()} {
		if strings.Contains(s, "super-secret") {
			t.Fatalf("the client secret leaked: %s", s)
		}
		if !strings.Contains(s, "id.apps.example") {
			t.Fatalf("the client id should still show: %s", s)
		}
	}
	if err := googleConfig(t, func(c *Config) { configured(c); c.Google.RedirectURL = "http://api.staryield.net/cb" }); err == nil ||
		strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("validation errors must not echo the secret: %v", err)
	}
}
