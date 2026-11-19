package service

import (
	"context"
	"strings"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
)

// revisions 6.0 (docs/platform-extensions.md §26.0): the three admin
// settings of the round live in site_settings (migration 0031) and travel in
// the cached site snapshot like the others. This file only parses them and
// exposes fail-closed readers; each feature owns its admin endpoints:
//
//   - trial_hold         §26.1 (chat_holds*.go)
//   - payment_policies   §26.5 (billing payment operations)
//   - calculator_report  §26.4 (calculator report e-mails)

// TrialHoldSettings are the free-trial hold settings (§26.1). The zero value
// (both switches off) is what an unreadable setting resolves to: never hold
// a reading by accident.
type TrialHoldSettings struct {
	AfterTrial          bool      `json:"after_trial"`
	AfterPaid           bool      `json:"after_paid"`
	HoldSeconds         int       `json:"hold_seconds"`
	PaymentGraceSeconds int       `json:"payment_grace_seconds"`
	Version             int       `json:"version"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// Applies reports whether a reading whose minute cannot be paid is held:
// trialEnded is true when the free minutes just ran out (no paid minute yet
// in this reading), false when paid credits ran out.
func (t TrialHoldSettings) Applies(trialEnded bool) bool {
	if trialEnded {
		return t.AfterTrial
	}
	return t.AfterPaid
}

// HoldDuration is how long a hold waits for a top-up.
func (t TrialHoldSettings) HoldDuration() time.Duration {
	return time.Duration(min(max(t.HoldSeconds, domain.TrialHoldSecondsMin), domain.TrialHoldSecondsMax)) * time.Second
}

// PaymentGrace is how much longer a hold may wait while a payment the client
// started during it is still processing.
func (t TrialHoldSettings) PaymentGrace() time.Duration {
	return time.Duration(min(max(t.PaymentGraceSeconds, 0), domain.TrialHoldPaymentGraceMax)) * time.Second
}

// PaymentPolicies are the dispute and fraud-warning policies of card
// payments (§26.5). When the setting cannot be read the protective defaults
// apply (defaultPaymentPolicies).
type PaymentPolicies struct {
	BlockPurchasesOnDispute bool      `json:"block_purchases_on_dispute"`
	DisputeClawback         bool      `json:"dispute_clawback"`
	EFWAutoRefund           bool      `json:"efw_auto_refund"`
	ThreeDSecure            string    `json:"three_d_secure"` // payment.ThreeDSAutomatic | payment.ThreeDSAny
	Version                 int       `json:"version"`
	UpdatedAt               time.Time `json:"updated_at"`
}

func defaultPaymentPolicies(now time.Time) PaymentPolicies {
	return PaymentPolicies{BlockPurchasesOnDispute: true, DisputeClawback: true, ThreeDSecure: payment.ThreeDSAutomatic,
		Version: 1, UpdatedAt: now}
}

// CalculatorReportSettings switch the calculators' e-mail report and list
// the countries where its marketing box starts ticked (§26.4). The zero
// value (disabled) is what an unreadable setting resolves to.
type CalculatorReportSettings struct {
	Enabled                      bool      `json:"enabled"`
	MarketingPrecheckedCountries []string  `json:"marketing_prechecked_countries"`
	Version                      int       `json:"version"`
	UpdatedAt                    time.Time `json:"updated_at"`
}

// Prechecked reports whether the marketing box starts ticked for a visitor
// from country (ISO 3166-1 alpha-2). An unknown country never does.
func (c CalculatorReportSettings) Prechecked(country string) bool {
	country = strings.ToUpper(strings.TrimSpace(country))
	if !countryCode.MatchString(country) {
		return false
	}
	for _, cc := range c.MarketingPrecheckedCountries {
		if cc == country {
			return true
		}
	}
	return false
}

func trialHoldFrom(st domain.SiteSetting) TrialHoldSettings {
	return TrialHoldSettings{AfterTrial: mBool(st.Value, "after_trial"), AfterPaid: mBool(st.Value, "after_paid"),
		HoldSeconds:         min(max(mInt(st.Value, "hold_seconds", domain.TrialHoldSecondsDefault), domain.TrialHoldSecondsMin), domain.TrialHoldSecondsMax),
		PaymentGraceSeconds: min(max(mInt(st.Value, "payment_grace_seconds", domain.TrialHoldPaymentGraceDefault), 0), domain.TrialHoldPaymentGraceMax),
		Version:             st.Version, UpdatedAt: st.UpdatedAt}
}

func paymentPoliciesFrom(st domain.SiteSetting) PaymentPolicies {
	threeDS := mString(st.Value, "three_d_secure")
	if threeDS != payment.ThreeDSAny {
		threeDS = payment.ThreeDSAutomatic
	}
	return PaymentPolicies{BlockPurchasesOnDispute: mBool(st.Value, "block_purchases_on_dispute"),
		DisputeClawback: mBool(st.Value, "dispute_clawback"), EFWAutoRefund: mBool(st.Value, "efw_auto_refund"),
		ThreeDSecure: threeDS, Version: st.Version, UpdatedAt: st.UpdatedAt}
}

func calculatorReportFrom(st domain.SiteSetting) CalculatorReportSettings {
	return CalculatorReportSettings{Enabled: mBool(st.Value, "enabled"),
		MarketingPrecheckedCountries: countryList(st.Value["marketing_prechecked_countries"]),
		Version:                      st.Version, UpdatedAt: st.UpdatedAt}
}

// countryList reads a JSON array of country codes, keeping valid,
// upper-cased, distinct codes (at most domain.MaxPrecheckedCountries).
func countryList(v any) []string {
	out := []string{}
	items, _ := v.([]any)
	seen := map[string]bool{}
	for _, it := range items {
		s, _ := it.(string)
		s = strings.ToUpper(strings.TrimSpace(s))
		if countryCode.MatchString(s) && !seen[s] && len(out) < domain.MaxPrecheckedCountries {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// TrialHold returns the hold settings (at most 10 s stale). When they cannot
// be read and nothing is cached, holds are off.
func (s *SiteService) TrialHold(ctx context.Context) TrialHoldSettings {
	snap := s.snapshot(ctx)
	if snap.Defaults {
		return TrialHoldSettings{}
	}
	return snap.TrialHold
}

// PaymentPolicies returns the payment policies (at most 10 s stale). When
// they cannot be read and nothing is cached, the protective defaults apply.
func (s *SiteService) PaymentPolicies(ctx context.Context) PaymentPolicies {
	snap := s.snapshot(ctx)
	if snap.Defaults || snap.PaymentPolicies.ThreeDSecure == "" {
		return defaultPaymentPolicies(s.d.Clock.Now())
	}
	return snap.PaymentPolicies
}

// CalculatorReport returns the e-mail report settings (at most 10 s stale).
// When they cannot be read and nothing is cached, the report is off.
func (s *SiteService) CalculatorReport(ctx context.Context) CalculatorReportSettings {
	snap := s.snapshot(ctx)
	if snap.Defaults {
		return CalculatorReportSettings{}
	}
	return snap.CalculatorReport
}
