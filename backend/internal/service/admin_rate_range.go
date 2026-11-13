package service

import (
	"context"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
)

// revisions 5.0: advisor rate range (§24.1). An advisor's rate must lie
// within the admin-controlled range; existing out-of-range advisors are left
// alone and flagged in the admin views (rate_out_of_range).

// RateRangeFunc reads the current advisor rate range (PricingService.RateRange).
type RateRangeFunc func(ctx context.Context) (min, max int, err error)

// SetRateRange enables the advisor rate range rule.
func (s *AdminService) SetRateRange(f RateRangeFunc) { s.rateRange = f }

// checkRate rejects a rate outside the range (422 fields.rate_per_minute).
// Without a configured range the rule is off.
func (s *AdminService) checkRate(ctx context.Context, rate *int64) error {
	if rate == nil || s.rateRange == nil {
		return nil
	}
	lo, hi, err := s.rateRange(ctx)
	if err != nil {
		return err
	}
	if rateOutOfRange(*rate, lo, hi) {
		return apperror.Validation(map[string]string{"rate_per_minute": rateRangeMessage(lo, hi)})
	}
	return nil
}

// flagRates marks the views whose rate is outside the range. When the range
// cannot be read nothing is flagged.
func (s *AdminService) flagRates(ctx context.Context, views ...*AdminPsychicView) {
	if s.rateRange == nil || len(views) == 0 {
		return
	}
	lo, hi, err := s.rateRange(ctx)
	if err != nil {
		warnErr(ctx, "admin: read advisor rate range", err)
		return
	}
	for _, v := range views {
		v.RateOutOfRange = rateOutOfRange(v.RatePerMinute, lo, hi)
	}
}
