package service

import (
	"context"
	"strings"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 6.0 (§26.4): the calculators' e-mail report setting
// (site_settings key calculator_report). The readers are in
// site_revisions6.go; this file is the admin endpoints.

const (
	msgPrecheckCodes = "Use two-letter country codes such as US."
	msgPrecheckCount = "Use at most 60 countries."
)

// CalculatorReportInput is PUT /admin/site/calculator-report.
type CalculatorReportInput struct {
	Enabled                      *bool     `json:"enabled"`
	MarketingPrecheckedCountries *[]string `json:"marketing_prechecked_countries"`
	// Version is the version the admin read; a stale one is a 409.
	Version *int `json:"version"`
}

// CalculatorReportSetting is the setting as the admin console shows it
// (CalculatorReportSetting of §26.4).
type CalculatorReportSetting struct {
	Enabled                      bool      `json:"enabled"`
	MarketingPrecheckedCountries []string  `json:"marketing_prechecked_countries"`
	Version                      int       `json:"version"`
	UpdatedAt                    time.Time `json:"updated_at"`
	UpdatedBy                    *UserRef  `json:"updated_by,omitempty"`
}

func (s *SiteService) calculatorReportView(ctx context.Context, st *domain.SiteSetting) *CalculatorReportSetting {
	v := calculatorReportFrom(*st)
	out := &CalculatorReportSetting{Enabled: v.Enabled, MarketingPrecheckedCountries: v.MarketingPrecheckedCountries,
		Version: st.Version, UpdatedAt: st.UpdatedAt}
	if st.UpdatedBy != nil {
		if name, ok := s.names(ctx, st.UpdatedBy)[*st.UpdatedBy]; ok {
			out.UpdatedBy = &UserRef{ID: *st.UpdatedBy, Name: name}
		}
	}
	return out
}

// CalculatorReportSetting returns the stored setting (database, never the cache).
func (s *SiteService) CalculatorReportSetting(ctx context.Context) (*CalculatorReportSetting, error) {
	st, err := s.d.Repo.Setting(ctx, domain.SiteSettingCalculatorReport)
	if err != nil {
		return nil, mapRepoErr(err, "Setting")
	}
	return s.calculatorReportView(ctx, st), nil
}

// parsePrecheckedCountries validates the admin's list: distinct two-letter
// codes (case-insensitive, stored upper-case), at most MaxPrecheckedCountries.
func parsePrecheckedCountries(in []string) ([]string, string) {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, raw := range in {
		code := strings.ToUpper(strings.TrimSpace(raw))
		if !countryCode.MatchString(code) || code == "XX" || code == "T1" {
			return nil, msgPrecheckCodes
		}
		if !seen[code] {
			seen[code] = true
			out = append(out, code)
		}
	}
	if len(out) > domain.MaxPrecheckedCountries {
		return nil, msgPrecheckCount
	}
	return out, ""
}

// UpdateCalculatorReport replaces the setting. A stale version is a 409
// version_conflict; an unchanged value is not a new version.
func (s *SiteService) UpdateCalculatorReport(ctx context.Context, in CalculatorReportInput) (*CalculatorReportSetting, error) {
	fields := map[string]string{}
	if in.Enabled == nil {
		fields["enabled"] = msgBannerRequired
	}
	var countries []string
	if in.MarketingPrecheckedCountries == nil {
		fields["marketing_prechecked_countries"] = msgBannerRequired
	} else if list, msg := parsePrecheckedCountries(*in.MarketingPrecheckedCountries); msg != "" {
		fields["marketing_prechecked_countries"] = msg
	} else {
		countries = list
	}
	if in.Version != nil && *in.Version < 1 {
		fields["version"] = msgSiteWrongType
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	var before, after CalculatorReportSettings
	changed := false
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		changed = false
		st, err := s.d.Repo.SettingForUpdate(ctx, domain.SiteSettingCalculatorReport)
		if err != nil {
			return err
		}
		before = calculatorReportFrom(*st)
		if in.Version != nil && *in.Version != st.Version {
			return apperror.VersionConflict(st.Version)
		}
		after = CalculatorReportSettings{Enabled: *in.Enabled, MarketingPrecheckedCountries: countries}
		if after.Enabled == before.Enabled && sameCountryList(after.MarketingPrecheckedCountries, before.MarketingPrecheckedCountries) {
			return nil
		}
		changed = true
		return s.d.Repo.SaveSetting(ctx, domain.SiteSettingCalculatorReport, domain.JSONMap{"enabled": after.Enabled,
			"marketing_prechecked_countries": after.MarketingPrecheckedCountries}, st.Version+1, actor(ctx), s.d.Clock.Now())
	})
	if err != nil {
		return nil, mapRepoErr(err, "Setting")
	}
	if changed {
		s.changed(ctx)
		s.record(ctx, "site.calculator_report.updated", "site_setting", domain.SiteSettingCalculatorReport, map[string]any{
			"before": map[string]any{"enabled": before.Enabled, "marketing_prechecked_countries": before.MarketingPrecheckedCountries},
			"after":  map[string]any{"enabled": after.Enabled, "marketing_prechecked_countries": after.MarketingPrecheckedCountries}})
	}
	return s.CalculatorReportSetting(ctx)
}

func sameCountryList(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
