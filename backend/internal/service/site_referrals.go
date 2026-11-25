package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// Growth: the referral program setting (site_settings key referral_program,
// migration 0036). It travels in the cached site snapshot like the other
// settings; ReferralService reads it through SiteService.ReferralProgram and
// the admin console edits it through the endpoints below.

// ReferralLaunchConfirmation is the phrase an administrator types to launch
// the rewards (enforced server-side).
const ReferralLaunchConfirmation = "LAUNCH"

// CodeReferralLaunched is the 409 of launching twice.
const CodeReferralLaunched apperror.Code = "referral_already_launched"

const (
	msgReferralLaunchConfirm = "Type LAUNCH to confirm."
	msgReferralLaunched      = "The referral rewards were already launched."
)

// ReferralProgram is the program's setting. The zero value (disabled, never
// launched) is what an unreadable setting resolves to: no referral is
// accepted and nothing is granted by accident.
type ReferralProgram struct {
	Enabled               bool       `json:"enabled"`
	RewardMinutes         int        `json:"reward_minutes"`
	MaxRewardsPerReferrer int        `json:"max_rewards_per_referrer"`
	LaunchedAt            *time.Time `json:"launched_at,omitempty"`
	Version               int        `json:"version"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// Launched reports whether the rewards were launched.
func (p ReferralProgram) Launched() bool { return p.LaunchedAt != nil }

// referralProgramFrom reads the stored document; anything malformed or out of
// range is clamped, never read as more generous than the bounds.
func referralProgramFrom(st domain.SiteSetting) ReferralProgram {
	return ReferralProgram{Enabled: mBool(st.Value, "enabled"),
		RewardMinutes: min(max(mInt(st.Value, "reward_minutes", domain.ReferralRewardMinutesDefault),
			domain.ReferralRewardMinutesMin), domain.ReferralRewardMinutesMax),
		MaxRewardsPerReferrer: min(max(mInt(st.Value, "max_rewards_per_referrer", domain.ReferralMaxRewardsDefault),
			domain.ReferralMaxRewardsMin), domain.ReferralMaxRewardsMax),
		LaunchedAt: mTime(st.Value, "launched_at"), Version: st.Version, UpdatedAt: st.UpdatedAt}
}

func (p ReferralProgram) toMap() domain.JSONMap {
	m := domain.JSONMap{"enabled": p.Enabled, "reward_minutes": p.RewardMinutes,
		"max_rewards_per_referrer": p.MaxRewardsPerReferrer, "launched_at": nil}
	if p.LaunchedAt != nil {
		m["launched_at"] = p.LaunchedAt.UTC().Format(time.RFC3339Nano)
	}
	return m
}

// ReferralProgram returns the program setting (at most 10 s stale). When the
// setting cannot be read and nothing is cached it is disabled.
func (s *SiteService) ReferralProgram(ctx context.Context) ReferralProgram {
	snap := s.snapshot(ctx)
	if snap.Referral != nil {
		return *snap.Referral
	}
	if snap.Defaults {
		return ReferralProgram{}
	}
	// A copy cached by an instance that predates the program: read the row.
	st, err := s.d.Repo.Setting(ctx, domain.SiteSettingReferralProgram)
	if err != nil {
		warnErr(ctx, "site: read the referral program", err)
		return ReferralProgram{}
	}
	return referralProgramFrom(*st)
}

// ReferralProgramInput is PUT /admin/site/referral-program. The launch date
// is not editable here (POST /admin/referrals/launch sets it, once).
type ReferralProgramInput struct {
	Enabled               *bool `json:"enabled"`
	RewardMinutes         *int  `json:"reward_minutes"`
	MaxRewardsPerReferrer *int  `json:"max_rewards_per_referrer"`
	// Version is the version the admin read; a stale one is a 409.
	Version *int `json:"version"`
}

// ReferralProgramSetting is the setting as the admin console shows it.
type ReferralProgramSetting struct {
	ReferralProgram
	UpdatedBy *UserRef `json:"updated_by,omitempty"`
}

func (s *SiteService) referralProgramView(ctx context.Context, st *domain.SiteSetting) *ReferralProgramSetting {
	out := &ReferralProgramSetting{ReferralProgram: referralProgramFrom(*st)}
	if st.UpdatedBy != nil {
		if name, ok := s.names(ctx, st.UpdatedBy)[*st.UpdatedBy]; ok {
			out.UpdatedBy = &UserRef{ID: *st.UpdatedBy, Name: name}
		}
	}
	return out
}

// ReferralProgramSetting returns the stored setting (database, never the cache).
func (s *SiteService) ReferralProgramSetting(ctx context.Context) (*ReferralProgramSetting, error) {
	st, err := s.d.Repo.Setting(ctx, domain.SiteSettingReferralProgram)
	if err != nil {
		return nil, mapRepoErr(err, "Setting")
	}
	return s.referralProgramView(ctx, st), nil
}

func referralBoundsMessage(lo, hi int) string {
	return fmt.Sprintf("Use a whole number between %d and %d.", lo, hi)
}

// UpdateReferralProgram replaces the editable fields. A stale version is a
// 409 version_conflict; an unchanged value is not a new version. The launch
// date is preserved.
func (s *SiteService) UpdateReferralProgram(ctx context.Context, in ReferralProgramInput) (*ReferralProgramSetting, error) {
	fields := map[string]string{}
	if in.Enabled == nil {
		fields["enabled"] = msgBannerRequired
	}
	if in.RewardMinutes == nil {
		fields["reward_minutes"] = msgBannerRequired
	} else if *in.RewardMinutes < domain.ReferralRewardMinutesMin || *in.RewardMinutes > domain.ReferralRewardMinutesMax {
		fields["reward_minutes"] = referralBoundsMessage(domain.ReferralRewardMinutesMin, domain.ReferralRewardMinutesMax)
	}
	if in.MaxRewardsPerReferrer == nil {
		fields["max_rewards_per_referrer"] = msgBannerRequired
	} else if *in.MaxRewardsPerReferrer < domain.ReferralMaxRewardsMin || *in.MaxRewardsPerReferrer > domain.ReferralMaxRewardsMax {
		fields["max_rewards_per_referrer"] = referralBoundsMessage(domain.ReferralMaxRewardsMin, domain.ReferralMaxRewardsMax)
	}
	if in.Version != nil && *in.Version < 1 {
		fields["version"] = msgSiteWrongType
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	var before, after ReferralProgram
	changed := false
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		changed = false
		st, err := s.d.Repo.SettingForUpdate(ctx, domain.SiteSettingReferralProgram)
		if err != nil {
			return err
		}
		before = referralProgramFrom(*st)
		if in.Version != nil && *in.Version != st.Version {
			return apperror.VersionConflict(st.Version)
		}
		after = before
		after.Enabled, after.RewardMinutes, after.MaxRewardsPerReferrer = *in.Enabled, *in.RewardMinutes, *in.MaxRewardsPerReferrer
		if after.Enabled == before.Enabled && after.RewardMinutes == before.RewardMinutes &&
			after.MaxRewardsPerReferrer == before.MaxRewardsPerReferrer {
			return nil
		}
		changed = true
		return s.d.Repo.SaveSetting(ctx, domain.SiteSettingReferralProgram, after.toMap(), st.Version+1, actor(ctx), s.d.Clock.Now())
	})
	if err != nil {
		return nil, mapRepoErr(err, "Setting")
	}
	if changed {
		s.changed(ctx)
		s.record(ctx, "site.referral_program.updated", "site_setting", domain.SiteSettingReferralProgram, map[string]any{
			"before": referralAudit(before), "after": referralAudit(after)})
	}
	return s.ReferralProgramSetting(ctx)
}

func referralAudit(p ReferralProgram) map[string]any {
	return map[string]any{"enabled": p.Enabled, "reward_minutes": p.RewardMinutes, "max_rewards_per_referrer": p.MaxRewardsPerReferrer}
}

// LaunchReferralRewards stamps launched_at, once. The typed confirmation is
// enforced here; launching again is a 409. It returns the new setting.
func (s *SiteService) LaunchReferralRewards(ctx context.Context, confirmation string) (*ReferralProgramSetting, error) {
	if confirmation != ReferralLaunchConfirmation {
		return nil, apperror.Validation(map[string]string{"confirmation": msgReferralLaunchConfirm})
	}
	now := s.d.Clock.Now().UTC()
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		st, err := s.d.Repo.SettingForUpdate(ctx, domain.SiteSettingReferralProgram)
		if err != nil {
			return err
		}
		p := referralProgramFrom(*st)
		if p.Launched() {
			return apperror.Conflict(CodeReferralLaunched, msgReferralLaunched)
		}
		p.LaunchedAt = &now
		return s.d.Repo.SaveSetting(ctx, domain.SiteSettingReferralProgram, p.toMap(), st.Version+1, actor(ctx), now)
	})
	if err != nil {
		return nil, mapRepoErr(err, "Setting")
	}
	s.changed(ctx)
	s.record(ctx, "site.referral_program.launched", "site_setting", domain.SiteSettingReferralProgram,
		map[string]any{"launched_at": now.Format(time.RFC3339)})
	return s.ReferralProgramSetting(ctx)
}
