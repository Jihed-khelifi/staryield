package service

import (
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
)

func TestValidateTokenRejectsCardNumbers(t *testing.T) {
	for _, tok := range []string{"4242424242424242", "tok_4242 4242 4242 4242", "x-4242-4242-4242-4242-y", "pm_5555555555554444"} {
		if f := validateToken(tok); f == nil || !strings.Contains(f["token"], "Card numbers") {
			t.Errorf("card number accepted in %q: %v", tok, f)
		}
	}
	for _, tok := range []string{"short", "sbx_a visa", "<script>alert(1)</script>", strings.Repeat("a", 513)} {
		if f := validateToken(tok); f == nil {
			t.Errorf("malformed token accepted: %q", tok)
		}
	}
	for _, tok := range []string{"sbx_a_visa_4242_12_2030", "sbx_b_mastercard_0002_01_2031_fr", "tok_1Nv0Z2LkdIwHu7ixq"} {
		if f := validateToken(tok); f != nil {
			t.Errorf("valid token rejected: %q %v", tok, f)
		}
	}
}

func TestCountriesNormalises(t *testing.T) {
	got := countries("fr", " FR", "", "XX", "T1", "usa", "us", "1A")
	if len(got) != 2 || got[0] != "FR" || got[1] != "US" {
		t.Fatalf("countries: %v", got)
	}
}

func TestPeriodFromKey(t *testing.T) {
	cases := map[string]int{"sub:0190:p1:a1": 1, "sub:abc:p12:a3": 12, "purchase:123": 0, "": 0, "sub:x:px:a1": 0}
	for key, want := range cases {
		if got := periodFromKey(key); got != want {
			t.Errorf("periodFromKey(%q) = %d, want %d", key, got, want)
		}
	}
}

func TestLocalizedValidation(t *testing.T) {
	fields := map[string]string{}
	out := localized("name", map[string]string{"en": " Premium​ ", "fr": "Abonnement", "de": "Mitglied"}, 80, false, true, fields)
	if out["en"] != "Premium" || out["fr"] != "Abonnement" || fields["name.de"] == "" {
		t.Fatalf("localized: %v %v", out, fields)
	}
	fields = map[string]string{}
	localized("name", map[string]string{"fr": "x"}, 80, false, true, fields)
	if fields["name.en"] == "" {
		t.Fatal("English must be required")
	}
	fields = map[string]string{}
	localized("description", map[string]string{"en": strings.Repeat("é", 501)}, 500, true, false, fields)
	if fields["description.en"] == "" {
		t.Fatal("length is counted in characters")
	}
}

func TestValidateVersion(t *testing.T) {
	s := &BillingService{cfg: BillingSettings{Currency: "USD"}}
	fields := map[string]string{}
	v := s.validateVersion(PlanVersionInput{PriceCents: 999, Interval: "month", MonthlyCreditGrant: 100, ChatDiscountBps: 1000,
		MonthlyFreeMinutes: 10, WaitlistPriority: true, Benefits: map[string][]string{"en": {" 100 credits each month "}}}, fields)
	if len(fields) != 0 || v.Currency != "USD" || v.IntervalCount != 1 || v.Benefits["en"][0] != "100 credits each month" {
		t.Fatalf("valid version: %+v %v", v, fields)
	}
	fields = map[string]string{}
	s.validateVersion(PlanVersionInput{PriceCents: 10, Currency: "usdx", Interval: "week", MonthlyCreditGrant: -1, ChatDiscountBps: 6000,
		MonthlyFreeMinutes: 601, Benefits: map[string][]string{"xx": {"a"}, "fr": make([]string, 9)}}, fields)
	for _, k := range []string{"version.price_cents", "version.currency", "version.interval", "version.monthly_credit_grant",
		"version.chat_discount_bps", "version.monthly_free_minutes", "version.benefits.xx", "version.benefits.fr"} {
		if fields[k] == "" {
			t.Errorf("missing error for %s (%v)", k, fields)
		}
	}
	fields = map[string]string{}
	s.validateVersion(PlanVersionInput{PriceCents: 999, Interval: "year", IntervalCount: 4}, fields)
	if fields["version.interval_count"] == "" {
		t.Fatal("yearly interval count is bounded")
	}
}

func TestValidateRoutingRule(t *testing.T) {
	proc := payment.NewSandboxProcessor()
	s := &BillingService{registry: payment.NewRegistry("sandbox_a", payment.NewSandboxA(proc, "x", time.Minute), payment.NewSandboxB(proc)),
		cfg: BillingSettings{Currency: "USD"}}
	fields := map[string]string{}
	name, prio, purpose := " FR cards ", 5, "subscription"
	providers := []string{"sandbox_b", "sandbox_b", "sandbox_a"}
	countriesIn := []string{"fr", "FR", "de"}
	users := []string{"11111111-1111-1111-1111-111111111111"}
	upd := s.validateRule(RoutingRuleInput{Name: &name, Priority: &prio, Purpose: &purpose, Providers: &providers,
		MatchCountries: &countriesIn, MatchUserIDs: &users}, true, fields)
	if len(fields) != 0 {
		t.Fatalf("valid rule rejected: %v", fields)
	}
	if upd["name"] != "FR cards" || len(upd["providers"].(domain.StringList)) != 2 || len(upd["match_countries"].(domain.StringList)) != 2 ||
		upd["enabled"] != true || upd["purpose"] != domain.PurposeSubscription {
		t.Fatalf("normalised rule: %v", upd)
	}
	bad := map[string]string{}
	empty, unknown, badCountries, badUsers := "", "bogus", []string{"FRA"}, []string{"not-a-uuid"}
	bigPrio := 100001
	s.validateRule(RoutingRuleInput{Name: &empty, Priority: &bigPrio, Purpose: &unknown, Providers: &[]string{"stripe"},
		MatchCountries: &badCountries, MatchUserIDs: &badUsers}, true, bad)
	for _, k := range []string{"name", "priority", "purpose", "providers", "match_countries", "match_user_ids"} {
		if bad[k] == "" {
			t.Errorf("missing error for %s (%v)", k, bad)
		}
	}
	// Partial update only touches the given fields.
	enabled := false
	if upd := s.validateRule(RoutingRuleInput{Enabled: &enabled}, false, map[string]string{}); len(upd) != 1 || upd["enabled"] != false {
		t.Fatalf("partial update: %v", upd)
	}
}
