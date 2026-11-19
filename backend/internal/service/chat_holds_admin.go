package service

// revisions 6.0 (docs/platform-extensions.md §26.1): the admin console's
// free-trial hold setting, written like the off-session setting (§22.3): a
// versioned PUT, a step-up, an audit record, cache drop and Pub/Sub
// invalidation through changed().

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

const (
	trialHoldStatsKey = "site:trial-hold:stats:v1"
	trialHoldStatsTTL = 60 * time.Second
	// TrialHoldStatsDays is the window of the admin card's figures.
	TrialHoldStatsDays = 30

	msgTrialHoldSeconds = "Use a whole number of seconds between 30 and 300."
	msgTrialHoldGrace   = "Use a whole number of seconds between 0 and 300."
)

// HoldStatsReader reads the hold figures of a window (the chat repository).
type HoldStatsReader interface {
	HoldStatsSince(ctx context.Context, since time.Time) (repository.HoldStats, error)
}

// TrialHoldInput is PUT /admin/site/trial-hold.
type TrialHoldInput struct {
	AfterTrial          *bool `json:"after_trial"`
	AfterPaid           *bool `json:"after_paid"`
	HoldSeconds         *int  `json:"hold_seconds"`
	PaymentGraceSeconds *int  `json:"payment_grace_seconds"`
	Version             *int  `json:"version"`
}

// TrialHoldStats are the figures of the last 30 days. Rates are fractions of
// the holds started (0 when none).
type TrialHoldStats struct {
	WindowDays            int     `json:"window_days"`
	Started               int64   `json:"started"`
	Converted             int64   `json:"converted"`
	Expired               int64   `json:"expired"`
	Revealed              int64   `json:"revealed"`
	ConversionRate        float64 `json:"conversion_rate"`
	RevealRate            float64 `json:"reveal_rate"`
	CreditsFromConversion int64   `json:"credits_from_conversions"`
	CreditsFromReveals    int64   `json:"credits_from_reveals"`
}

// TrialHoldSetting is the setting as the admin console shows it
// (TrialHoldSettings carries version and updated_at).
type TrialHoldSetting struct {
	TrialHoldSettings
	UpdatedBy *UserRef       `json:"updated_by,omitempty"`
	Stats     TrialHoldStats `json:"stats"`
}

func (s *SiteService) trialHoldView(ctx context.Context, st *domain.SiteSetting) *TrialHoldSetting {
	out := &TrialHoldSetting{TrialHoldSettings: trialHoldFrom(*st), Stats: s.trialHoldStats(ctx)}
	if st.UpdatedBy != nil {
		if name, ok := s.names(ctx, st.UpdatedBy)[*st.UpdatedBy]; ok {
			out.UpdatedBy = &UserRef{ID: *st.UpdatedBy, Name: name}
		}
	}
	return out
}

// trialHoldStats sums the last 30 days, cached for 60 s.
func (s *SiteService) trialHoldStats(ctx context.Context) TrialHoldStats {
	if s.d.Cache != nil {
		if raw, ok, err := s.d.Cache.Get(ctx, trialHoldStatsKey); err == nil && ok {
			var cached TrialHoldStats
			if json.Unmarshal([]byte(raw), &cached) == nil && cached.WindowDays == TrialHoldStatsDays {
				return cached
			}
		}
	}
	out := TrialHoldStats{WindowDays: TrialHoldStatsDays}
	if s.d.HoldStats == nil {
		return out
	}
	hs, err := s.d.HoldStats.HoldStatsSince(ctx, s.d.Clock.Now().AddDate(0, 0, -TrialHoldStatsDays))
	if err != nil {
		warnErr(ctx, "site: trial hold statistics", err)
		return out
	}
	out.Started, out.Converted, out.Expired, out.Revealed = hs.Started, hs.Converted, hs.Expired, hs.Revealed
	out.CreditsFromConversion, out.CreditsFromReveals = hs.CreditsFromConversion, hs.CreditsFromReveals
	if hs.Started > 0 {
		out.ConversionRate = float64(hs.Converted) / float64(hs.Started)
		out.RevealRate = float64(hs.Revealed) / float64(hs.Started)
	}
	if s.d.Cache != nil {
		if b, err := json.Marshal(out); err == nil {
			warnErr(ctx, "site: cache trial hold statistics", s.d.Cache.Set(ctx, trialHoldStatsKey, string(b), trialHoldStatsTTL))
		}
	}
	return out
}

// AdminTrialHold returns the stored setting (database, never the cache).
func (s *SiteService) AdminTrialHold(ctx context.Context) (*TrialHoldSetting, error) {
	st, err := s.d.Repo.Setting(ctx, domain.SiteSettingTrialHold)
	if err != nil {
		return nil, mapRepoErr(err, "Setting")
	}
	return s.trialHoldView(ctx, st), nil
}

// UpdateTrialHold replaces the setting. A stale version is a 409
// version_conflict; every field is required.
func (s *SiteService) UpdateTrialHold(ctx context.Context, in TrialHoldInput) (*TrialHoldSetting, error) {
	fields := map[string]string{}
	if in.AfterTrial == nil {
		fields["after_trial"] = msgBannerRequired
	}
	if in.AfterPaid == nil {
		fields["after_paid"] = msgBannerRequired
	}
	switch {
	case in.HoldSeconds == nil:
		fields["hold_seconds"] = msgBannerRequired
	case *in.HoldSeconds < domain.TrialHoldSecondsMin || *in.HoldSeconds > domain.TrialHoldSecondsMax:
		fields["hold_seconds"] = msgTrialHoldSeconds
	}
	switch {
	case in.PaymentGraceSeconds == nil:
		fields["payment_grace_seconds"] = msgBannerRequired
	case *in.PaymentGraceSeconds < 0 || *in.PaymentGraceSeconds > domain.TrialHoldPaymentGraceMax:
		fields["payment_grace_seconds"] = msgTrialHoldGrace
	}
	switch {
	case in.Version == nil:
		fields["version"] = msgBannerRequired
	case *in.Version < 1:
		fields["version"] = msgSiteWrongType
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	var before, after TrialHoldSettings
	var changed bool
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		changed = false
		st, err := s.d.Repo.SettingForUpdate(ctx, domain.SiteSettingTrialHold)
		if err != nil {
			return err
		}
		before = trialHoldFrom(*st)
		if *in.Version != st.Version {
			return apperror.VersionConflict(st.Version)
		}
		after = TrialHoldSettings{AfterTrial: *in.AfterTrial, AfterPaid: *in.AfterPaid, HoldSeconds: *in.HoldSeconds,
			PaymentGraceSeconds: *in.PaymentGraceSeconds}
		if after.AfterTrial == before.AfterTrial && after.AfterPaid == before.AfterPaid &&
			after.HoldSeconds == before.HoldSeconds && after.PaymentGraceSeconds == before.PaymentGraceSeconds {
			return nil
		}
		changed = true
		return s.d.Repo.SaveSetting(ctx, domain.SiteSettingTrialHold, domain.JSONMap{"after_trial": after.AfterTrial, "after_paid": after.AfterPaid,
			"hold_seconds": after.HoldSeconds, "payment_grace_seconds": after.PaymentGraceSeconds}, st.Version+1, actor(ctx), s.d.Clock.Now())
	})
	if err != nil {
		return nil, mapRepoErr(err, "Setting")
	}
	if changed {
		s.changed(ctx)
		doc := func(t TrialHoldSettings) map[string]any {
			return map[string]any{"after_trial": t.AfterTrial, "after_paid": t.AfterPaid, "hold_seconds": t.HoldSeconds,
				"payment_grace_seconds": t.PaymentGraceSeconds}
		}
		s.record(ctx, "site.trial_hold.updated", "site_setting", domain.SiteSettingTrialHold, map[string]any{"before": doc(before), "after": doc(after)})
	}
	return s.AdminTrialHold(ctx)
}
