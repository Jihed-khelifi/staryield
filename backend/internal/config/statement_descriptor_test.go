package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// revisions 5.1: statement descriptor (§25.3) and the memberships switch (§25.1).

// policyDescriptor reads the descriptor the English Refund Policy §11 states
// ("Your statement may display purchases as X.").
func policyDescriptor(t *testing.T) string {
	t.Helper()
	root := filepath.Join("..", "..", "..", "frontend")
	if _, err := os.Stat(root); err != nil {
		t.Skip("the frontend is not part of this checkout (backend-only build)")
	}
	b, err := os.ReadFile(filepath.Join(root, "src", "content", "legal", "refund", "en.ts"))
	if err != nil {
		t.Fatalf("the English Refund Policy must be readable: %v", err)
	}
	m := regexp.MustCompile(`Your statement may display purchases as ([^."]+)\.`).FindSubmatch(b)
	if m == nil {
		t.Fatal(`the English Refund Policy no longer contains "Your statement may display purchases as X.": update this test together with the policy`)
	}
	return strings.TrimSpace(string(m[1]))
}

func TestStatementDescriptorDefaultEqualsThePublishedPolicy(t *testing.T) {
	clearPlatformEnv(t)
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if want := policyDescriptor(t); cfg.Payments.StatementDescriptor != want {
		t.Fatalf("STARYIELD_PAYMENTS_STATEMENT_DESCRIPTOR defaults to %q but the English Refund Policy says %q", cfg.Payments.StatementDescriptor, want)
	}
	if !ValidStatementDescriptor(cfg.Payments.StatementDescriptor) {
		t.Fatalf("the default descriptor %q is not valid", cfg.Payments.StatementDescriptor)
	}
}

func TestStatementDescriptorValidation(t *testing.T) {
	cases := map[string]bool{
		"STARYIELD MEDIA LLC":     true,
		"ABCDE":                   true,
		"ABCD":                    false, // too short
		"A VERY LONG DESCRIPTOR!": false,
		"1234567":                 false, // no letter
		"STARYIELD*MEDIA":         false, // '*' is not allowed
		"STARYIELD MÉDIA":         false, // non-ASCII
		"STARYIELD, MEDIA - LLC":  true,
		strings.Repeat("A", 22):   true,
		strings.Repeat("A", 23):   false,
	}
	for in, want := range cases {
		if got := ValidStatementDescriptor(in); got != want {
			t.Errorf("ValidStatementDescriptor(%q) = %v, want %v", in, got, want)
		}
	}
	clearPlatformEnv(t)
	t.Setenv("STARYIELD_PAYMENTS_STATEMENT_DESCRIPTOR", "ab")
	if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "STARYIELD_PAYMENTS_STATEMENT_DESCRIPTOR") {
		t.Fatalf("an invalid descriptor was accepted: %v", err)
	}
	t.Setenv("STARYIELD_PAYMENTS_STATEMENT_DESCRIPTOR", "  MY SHOP LLC  ")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Payments.StatementDescriptor != "MY SHOP LLC" {
		t.Fatalf("descriptor not trimmed: %q", cfg.Payments.StatementDescriptor)
	}
}

func TestProductionRefusesAnEmptyStatementDescriptor(t *testing.T) {
	clearPlatformEnv(t)
	t.Setenv("STARYIELD_APP_ENV", "production")
	setProductionSecrets(t)
	t.Setenv("STARYIELD_SECURITY_COOKIE_SECURE", "true")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("production with the default descriptor: %v", err)
	}
	cfg.Payments.StatementDescriptor = ""
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "STARYIELD_PAYMENTS_STATEMENT_DESCRIPTOR must be set in production") {
		t.Fatalf("an empty descriptor was accepted in production: %v", err)
	}
	cfg.App.Env = "development"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("an empty descriptor is allowed outside production: %v", err)
	}
}

func TestMembershipsAreOffByDefault(t *testing.T) {
	clearPlatformEnv(t)
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Billing.MembershipsEnabled {
		t.Fatal("memberships must be switched off by default")
	}
	t.Setenv("STARYIELD_BILLING_MEMBERSHIPS_ENABLED", "true")
	if cfg, err = Load(""); err != nil || !cfg.Billing.MembershipsEnabled {
		t.Fatalf("memberships switch: %v %+v", err, cfg.Billing)
	}
}

// The default support address is the one the web app prints (frontend/src/config/company.ts).
func TestSupportInboxDefaultEqualsTheWebConfig(t *testing.T) {
	root := filepath.Join("..", "..", "..", "frontend")
	b, err := os.ReadFile(filepath.Join(root, "src", "config", "company.ts"))
	if err != nil {
		t.Skip("the frontend is not part of this checkout (backend-only build)")
	}
	m := regexp.MustCompile(`supportEmail:\s*"([^"]+)"`).FindSubmatch(b)
	if m == nil {
		t.Fatal("company.ts no longer defines supportEmail")
	}
	clearPlatformEnv(t)
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mail.SupportInbox != string(m[1]) {
		t.Fatalf("STARYIELD_MAIL_SUPPORT_INBOX defaults to %q, the web prints %q", cfg.Mail.SupportInbox, m[1])
	}
}
