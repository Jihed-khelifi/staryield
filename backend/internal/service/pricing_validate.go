package service

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 5.0: pricing catalog (§24.1). Pure validation of pack input and
// pricing settings (no I/O; unit tested on their own).

// Pricing messages (translated at the API edge).
const (
	msgPackCode        = "Use a lowercase code of 2 to 40 letters, digits and underscores."
	msgPackCodeTaken   = "This code is already used by another pack."
	msgPackImmutable   = "This can't be changed after the pack is created."
	msgPackKind        = "Choose standard or intro."
	msgPackNameEN      = "Enter the pack name in English."
	msgPackTagline     = "Use at most 120 characters."
	msgPackCredits     = "Use between 1 and 1,000,000 credits."
	msgPackPrice       = "Use a price between 0.50 and 100,000.00."
	msgPackSortOrder   = "Use a whole number between 0 and 100000."
	msgPackReference   = "The reference pack must be an active standard pack."
	msgPackNewClients  = "Only the welcome offer can be limited to new clients."
	msgPackIntroActive = "Only one welcome pack can be active. Deactivate the current one first."
	msgPackOrder       = "Send every pack exactly once."
	msgPackNothing     = "Nothing to update."
	msgRateRange       = "Use a whole number between 1 and 10000."
	msgRateMaxBelowMin = "The maximum rate must be at least the minimum."
	msgPopularMin      = "Use a whole number between 1 and 100000."
	msgPopularWindow   = "Use a whole number between 7 and 365."
	msgFeeBps          = "Use a whole number between 0 and 3000."
	msgFeeFixed        = "Use a whole number between 0 and 500."
	msgRateBelowRange  = "Use a rate between %d and %d credits."
)

// packLocale cleans one localized pack line: control, bidi and invisible
// format characters stripped, markup refused, length limited. It returns the
// cleaned line and the message to report ("" when valid). An empty result
// means "language not provided".
func packLine(raw string, max int, field string) (string, string) {
	v := cleanBannerLine(raw)
	switch {
	case v == "":
		return "", ""
	case utf8.RuneCountInString(v) > max:
		switch field {
		case "tagline":
			return "", msgPackTagline
		case "label":
			return "", msgBannerCTALength
		}
		return "", fmt.Sprintf("Use between 1 and %d characters.", max)
	case htmlTag.MatchString(v):
		return "", msgBannerHTML
	}
	return v, ""
}

// validatePackL10n cleans localized pack copy like validateL10n does for
// banners: unknown languages are refused, empty ones dropped, en required
// when asked. Field messages are keyed "<field>.<locale>".
func validatePackL10n(field string, in map[string]string, max int, requireEN bool, fields map[string]string) domain.L10n {
	out := domain.L10n{}
	for loc, raw := range in {
		key := field + "." + loc
		if loc != "en" && loc != "fr" && loc != "es" {
			fields[key] = msgBannerLocale
			continue
		}
		v, msg := packLine(raw, max, field)
		switch {
		case msg != "":
			fields[key] = msg
		case v != "":
			out[loc] = v
		}
	}
	if requireEN && out["en"] == "" && fields[field+".en"] == "" {
		fields[field+".en"] = msgPackNameEN
	}
	return out
}

// validPackCode reports a well-formed code.
func validPackCode(code string) bool { return domain.PackCodePattern.MatchString(code) }

// validPackNumbers checks credits and price.
func validPackNumbers(credits, price *int64, creating bool, fields map[string]string) {
	switch {
	case credits == nil:
		if creating {
			fields["credits"] = msgBannerRequired
		}
	case *credits < domain.PackCreditsMin || *credits > domain.PackCreditsMax:
		fields["credits"] = msgPackCredits
	}
	switch {
	case price == nil:
		if creating {
			fields["price_cents"] = msgBannerRequired
		}
	case *price < domain.PackPriceMin || *price > domain.PackPriceMax:
		fields["price_cents"] = msgPackPrice
	}
}

// packRules checks the cross-field rules of a pack after a change.
func packRules(p *domain.CreditPack, fields map[string]string) {
	if p.Reference && (p.Kind != domain.PackStandard || !p.Active) {
		fields["reference"] = msgPackReference
	}
	if p.NewClientsOnly && p.Kind != domain.PackIntro {
		fields["new_clients_only"] = msgPackNewClients
	}
}

// PricingSettingsInput is PUT /admin/pricing/settings (every field required).
type PricingSettingsInput struct {
	RateMin              *int `json:"rate_min"`
	RateMax              *int `json:"rate_max"`
	PopularMinPurchases  *int `json:"popular_min_purchases"`
	PopularWindowDays    *int `json:"popular_window_days"`
	ProcessingFeeBps     *int `json:"processing_fee_bps"`
	ProcessingFixedCents *int `json:"processing_fixed_cents"`
	ExpectedVersion      *int `json:"expected_version"`
}

// validatePricingSettings returns the values to store and the field messages.
func validatePricingSettings(in PricingSettingsInput) (PricingSettings, map[string]string) {
	fields := map[string]string{}
	var out PricingSettings
	num := func(key string, v *int, lo, hi int, msg string) int {
		switch {
		case v == nil:
			fields[key] = msgBannerRequired
		case *v < lo || *v > hi:
			fields[key] = msg
		default:
			return *v
		}
		return 0
	}
	out.RateMin = num("rate_min", in.RateMin, 1, 10000, msgRateRange)
	out.RateMax = num("rate_max", in.RateMax, 1, 10000, msgRateRange)
	if fields["rate_min"] == "" && fields["rate_max"] == "" && out.RateMax < out.RateMin {
		fields["rate_max"] = msgRateMaxBelowMin
	}
	out.PopularMinPurchases = num("popular_min_purchases", in.PopularMinPurchases, 1, 100000, msgPopularMin)
	out.PopularWindowDays = num("popular_window_days", in.PopularWindowDays, 7, 365, msgPopularWindow)
	out.ProcessingFeeBps = num("processing_fee_bps", in.ProcessingFeeBps, 0, 3000, msgFeeBps)
	out.ProcessingFixedCents = num("processing_fixed_cents", in.ProcessingFixedCents, 0, 500, msgFeeFixed)
	if in.ExpectedVersion != nil && *in.ExpectedVersion < 1 {
		fields["expected_version"] = msgSiteWrongType
	}
	return out, fields
}

// rateOutOfRange reports a rate outside the advisor rate range.
func rateOutOfRange(rate int64, min, max int) bool { return rate < int64(min) || rate > int64(max) }

// rateRangeMessage is the 422 text of an advisor rate outside the range.
func rateRangeMessage(min, max int) string {
	return fmt.Sprintf(msgRateBelowRange, min, max)
}

// trimPackCode normalizes a code before validation.
func trimPackCode(s string) string { return strings.TrimSpace(s) }
