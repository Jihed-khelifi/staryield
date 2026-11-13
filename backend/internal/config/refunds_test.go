package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// revisions 5.0: refund requests (§24.4).

// policyWindow reads the number of days the English Refund Policy grants
// ("within N calendar days of the original purchase").
func policyWindow(t *testing.T) int {
	t.Helper()
	root := filepath.Join("..", "..", "..", "frontend")
	if _, err := os.Stat(root); err != nil {
		t.Skip("the frontend is not part of this checkout (backend-only build)")
	}
	b, err := os.ReadFile(filepath.Join(root, "src", "content", "legal", "refund", "en.ts"))
	if err != nil {
		t.Fatalf("the English Refund Policy must be readable: %v", err)
	}
	m := regexp.MustCompile(`within (\d+) calendar days`).FindSubmatch(b)
	if m == nil {
		t.Fatal(`the English Refund Policy no longer contains "within N calendar days": update this test together with the policy`)
	}
	n, err := strconv.Atoi(string(m[1]))
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// The default window is the one the published policy promises.
func TestRefundWindowDefaultEqualsThePublishedPolicy(t *testing.T) {
	clearPlatformEnv(t)
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if want := policyWindow(t); cfg.Refunds.WindowDays != want {
		t.Fatalf("STARYIELD_REFUNDS_WINDOW_DAYS defaults to %d but the English Refund Policy says %d calendar days", cfg.Refunds.WindowDays, want)
	}
	if !cfg.Refunds.RequestsEnabled {
		t.Fatal("in-app refund requests must be on by default")
	}
	if r := cfg.RateLimit.RefundRequest; r.Rate != 5 || r.Burst != 5 || r.Period.Hours() != 1 {
		t.Fatalf("refund request rate limit default: %+v", r)
	}
}

func TestRefundWindowIsValidated(t *testing.T) {
	clearPlatformEnv(t)
	for _, v := range []string{"0", "-3", "366", "1000"} {
		t.Setenv("STARYIELD_REFUNDS_WINDOW_DAYS", v)
		if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "STARYIELD_REFUNDS_WINDOW_DAYS") {
			t.Fatalf("window %s accepted: %v", v, err)
		}
	}
	for _, v := range []string{"1", "30", "365"} {
		t.Setenv("STARYIELD_REFUNDS_WINDOW_DAYS", v)
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("window %s refused: %v", v, err)
		}
		if got := strconv.Itoa(cfg.Refunds.WindowDays); got != v {
			t.Fatalf("window %s loaded as %s", v, got)
		}
	}
}

func TestRefundRequestsCanBeSwitchedOff(t *testing.T) {
	clearPlatformEnv(t)
	t.Setenv("STARYIELD_REFUNDS_REQUESTS_ENABLED", "false")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Refunds.RequestsEnabled {
		t.Fatal("the kill switch was ignored")
	}
}
