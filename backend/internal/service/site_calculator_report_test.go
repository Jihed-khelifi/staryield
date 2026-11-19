package service

import (
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 6.0 (§26.4): the e-mail report setting.

func seedCalculatorReport(f *siteFixture, enabled bool, countries ...any) {
	f.repo.settings[domain.SiteSettingCalculatorReport] = domain.SiteSetting{Key: "calculator_report", Version: 1,
		UpdatedAt: time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC),
		Value:     domain.JSONMap{"enabled": enabled, "marketing_prechecked_countries": countries}}
}

func TestCalculatorReportSettingReadAndUnreadable(t *testing.T) {
	f := newSiteFixture(t)
	seedCalculatorReport(f, true, "US", "us", "FR", "bad", 7)
	cur, err := f.svc.CalculatorReportSetting(f.ctx)
	if err != nil || !cur.Enabled || cur.Version != 1 || strings.Join(cur.MarketingPrecheckedCountries, ",") != "US,FR" {
		t.Fatalf("setting %+v %v", cur, err)
	}
	if got := f.svc.CalculatorReport(f.ctx); !got.Enabled || !got.Prechecked("US") || !got.Prechecked("fr") || got.Prechecked("DE") || got.Prechecked("") {
		t.Fatalf("reader %+v", got)
	}
	// Unreadable and nothing cached: the report is off.
	g := newSiteFixture(t)
	seedCalculatorReport(g, true, "US")
	g.repo.fail = true
	if got := g.svc.CalculatorReport(g.ctx); got.Enabled || got.Prechecked("US") {
		t.Fatalf("an unreadable setting must switch the report off: %+v", got)
	}
}

func TestUpdateCalculatorReportValidation(t *testing.T) {
	f := newSiteFixture(t)
	seedCalculatorReport(f, true, "US")
	many := make([]string, 61)
	for i := range many {
		many[i] = string(rune('A'+i/26)) + string(rune('A'+i%26))
	}
	for name, tc := range map[string]struct {
		in    CalculatorReportInput
		field string
	}{
		"nothing":        {CalculatorReportInput{}, "enabled"},
		"no countries":   {CalculatorReportInput{Enabled: ptr(true)}, "marketing_prechecked_countries"},
		"three letters":  {CalculatorReportInput{Enabled: ptr(true), MarketingPrecheckedCountries: &[]string{"USA"}}, "marketing_prechecked_countries"},
		"digits":         {CalculatorReportInput{Enabled: ptr(true), MarketingPrecheckedCountries: &[]string{"U1"}}, "marketing_prechecked_countries"},
		"not a country":  {CalculatorReportInput{Enabled: ptr(true), MarketingPrecheckedCountries: &[]string{"XX"}}, "marketing_prechecked_countries"},
		"empty code":     {CalculatorReportInput{Enabled: ptr(true), MarketingPrecheckedCountries: &[]string{""}}, "marketing_prechecked_countries"},
		"too many":       {CalculatorReportInput{Enabled: ptr(true), MarketingPrecheckedCountries: &many}, "marketing_prechecked_countries"},
		"bad version":    {CalculatorReportInput{Enabled: ptr(true), MarketingPrecheckedCountries: &[]string{"US"}, Version: ptr(0)}, "version"},
		"missing enable": {CalculatorReportInput{MarketingPrecheckedCountries: &[]string{"US"}}, "enabled"},
	} {
		_, err := f.svc.UpdateCalculatorReport(f.ctx, tc.in)
		if got := siteFields(t, err); got[tc.field] == "" {
			t.Errorf("%s: fields %v", name, got)
		}
	}
	if len(f.audit.rows) != 0 {
		t.Fatal("a refused update was audited")
	}
}

func TestUpdateCalculatorReportVersionAuditAndCache(t *testing.T) {
	f := newSiteFixture(t)
	seedCalculatorReport(f, true, "US")
	if !f.svc.CalculatorReport(f.ctx).Enabled { // warm the snapshot
		t.Fatal("enabled")
	}
	cur, _ := f.svc.CalculatorReportSetting(f.ctx)
	countries := []string{" gb ", "ca", "CA", "us"}
	out, err := f.svc.UpdateCalculatorReport(f.ctx, CalculatorReportInput{Enabled: ptr(false), MarketingPrecheckedCountries: &countries, Version: ptr(cur.Version)})
	if err != nil {
		t.Fatal(err)
	}
	if out.Enabled || out.Version != 2 || strings.Join(out.MarketingPrecheckedCountries, ",") != "GB,CA,US" || out.UpdatedBy == nil || out.UpdatedBy.Name != "Ada Admin" {
		t.Fatalf("updated %+v", out)
	}
	if got := f.svc.CalculatorReport(f.ctx); got.Enabled || !got.Prechecked("GB") || got.Prechecked("FR") {
		t.Fatalf("the cache was not dropped: %+v", got)
	}
	if len(f.audit.rows) != 1 || f.audit.rows[0].Action != "site.calculator_report.updated" || f.audit.rows[0].TargetID != "calculator_report" {
		t.Fatalf("audit %+v", f.audit.rows)
	}
	meta := f.audit.rows[0].Metadata
	if before, _ := meta["before"].(map[string]any); before["enabled"] != true {
		t.Fatalf("audit metadata %+v", meta)
	}

	// A stale version is a conflict that names the current one.
	_, err = f.svc.UpdateCalculatorReport(f.ctx, CalculatorReportInput{Enabled: ptr(true), MarketingPrecheckedCountries: &countries, Version: ptr(cur.Version)})
	e, ok := apperror.As(err)
	if !ok || e.Code != apperror.CodeVersionConflict || e.Status != 409 {
		t.Fatalf("stale version: %v", err)
	}
	// An unchanged value is not a new version and is not audited.
	same, err := f.svc.UpdateCalculatorReport(f.ctx, CalculatorReportInput{Enabled: ptr(false), MarketingPrecheckedCountries: &[]string{"GB", "CA", "US"}})
	if err != nil || same.Version != 2 || len(f.audit.rows) != 1 {
		t.Fatalf("unchanged: %+v %v (%d audit rows)", same, err, len(f.audit.rows))
	}
	// Clearing the list is valid.
	cleared, err := f.svc.UpdateCalculatorReport(f.ctx, CalculatorReportInput{Enabled: ptr(true), MarketingPrecheckedCountries: &[]string{}})
	if err != nil || cleared.Version != 3 || len(cleared.MarketingPrecheckedCountries) != 0 || cleared.MarketingPrecheckedCountries == nil {
		t.Fatalf("cleared: %+v %v", cleared, err)
	}
}
