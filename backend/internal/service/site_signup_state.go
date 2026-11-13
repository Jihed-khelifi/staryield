package service

import "context"

// revisions 5.0: the public pricing summary reports the live sign-up offer
// (§24.1 free_trial).

// SignupOfferState is the sign-up offer as new accounts get it: whether it
// is on and its free minutes (0 when off). At most 10 s stale; when the
// settings cannot be read and nothing is cached it is the configured
// fallback, like SignupFreeMinutes.
func (s *SiteService) SignupOfferState(ctx context.Context) (enabled bool, minutes int) {
	snap := s.snapshot(ctx)
	if snap.Defaults {
		return true, s.d.Cfg.SignupFallbackMinutes
	}
	if !snap.Signup.Enabled {
		return false, 0
	}
	return true, snap.Signup.FreeMinutes
}
