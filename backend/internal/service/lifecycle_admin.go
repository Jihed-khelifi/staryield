package service

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// Messages of the lifecycle admin endpoints (translated at the API edge).
const (
	msgLifecycleDelay   = "Choose a delay between 1 hour and 90 days."
	msgLifecycleJourney = "Unknown journey."
	msgLifecycleDays    = "Choose between 1 and 30 days."
	msgLifecycleWindow  = "Choose a window between 1 and 90 days."
	msgLifecycleEmpty   = "Change at least one setting."
)

// lifecycleNoteNoVerification: the verification reminder needs the e-mail
// verification feature (SetVerificationReminder).
const lifecycleNoteNoVerification = "verification_unavailable"

const (
	lifecycleDelayMin = 1
	lifecycleDelayMax = 90 * 24
	// lifecycleFunnelMaxDays is the longest sign-up window of the funnel.
	lifecycleFunnelMaxDays = 90
	// LifecycleFunnelDefaultDays is the funnel's window when none is asked for.
	LifecycleFunnelDefaultDays = 30
	// lifecycleFunnelTTL: the funnel is one probe per account of the cohort,
	// so the console reads it from the cache for this long.
	lifecycleFunnelTTL = time.Minute
)

// LifecycleJourney is one journey's setting.
type LifecycleJourney struct {
	Enabled    bool `json:"enabled"`
	DelayHours int  `json:"delay_hours"`
}

// LifecycleSettings is the lifecycle messaging setting (site_settings key
// lifecycle). Journeys always holds every journey.
type LifecycleSettings struct {
	Journeys        map[string]LifecycleJourney `json:"journeys"`
	AttributionDays int                         `json:"attribution_days"`
}

// defaultLifecycle is what the setting resolves to when it cannot be read,
// and what the migration seeds. Win-back starts off: it reaches people who
// left, so the owner turns it on knowingly.
func defaultLifecycle() LifecycleSettings {
	return LifecycleSettings{AttributionDays: 7, Journeys: map[string]LifecycleJourney{
		domain.JourneyVerifyReminder:    {Enabled: true, DelayHours: 24},
		domain.JourneyFirstReading:      {Enabled: true, DelayHours: 24},
		domain.JourneyAfterFirstReading: {Enabled: true, DelayHours: 24},
		domain.JourneyWinBack:           {Enabled: false, DelayHours: 14 * 24},
	}}
}

func lifecycleFrom(st *domain.SiteSetting) LifecycleSettings {
	out := defaultLifecycle()
	if st == nil {
		return out
	}
	if n := mInt(st.Value, "attribution_days", out.AttributionDays); n >= 1 && n <= 30 {
		out.AttributionDays = n
	}
	journeys, _ := st.Value["journeys"].(map[string]any)
	for key, def := range out.Journeys {
		raw, _ := journeys[key].(map[string]any)
		if raw == nil {
			continue
		}
		j := def
		if v, ok := raw["enabled"].(bool); ok {
			j.Enabled = v
		}
		if n := mInt(raw, "delay_hours", def.DelayHours); n >= lifecycleDelayMin && n <= lifecycleDelayMax {
			j.DelayHours = n
		}
		out.Journeys[key] = j
	}
	return out
}

func (s LifecycleSettings) toMap() domain.JSONMap {
	journeys := map[string]any{}
	for key, j := range s.Journeys {
		journeys[key] = map[string]any{"enabled": j.Enabled, "delay_hours": j.DelayHours}
	}
	return domain.JSONMap{"journeys": journeys, "attribution_days": s.AttributionDays}
}

// settings reads the stored setting (database, never stale: the job reads it
// once per run). An unreadable or missing setting resolves to the defaults.
func (s *LifecycleService) settings(ctx context.Context) LifecycleSettings {
	st, err := s.d.Site.Setting(ctx, domain.SiteSettingLifecycle)
	if err != nil {
		warnErr(ctx, "lifecycle: read settings", err)
		return defaultLifecycle()
	}
	return lifecycleFrom(st)
}

// LifecycleJourneyStats are a journey's results over one window.
type LifecycleJourneyStats struct {
	Sent           int64            `json:"sent"`
	Skipped        int64            `json:"skipped"`
	SkippedReasons map[string]int64 `json:"skipped_reasons"`
	Converted      int64            `json:"converted"`
	// ConversionRate is converted / sent (0 when nothing was sent).
	ConversionRate float64 `json:"conversion_rate"`
}

// LifecycleJourneyView is a journey as the admin console shows it.
type LifecycleJourneyView struct {
	Key        string `json:"key"`
	Enabled    bool   `json:"enabled"`
	DelayHours int    `json:"delay_hours"`
	// DefaultDelayHours is the delay a fresh installation uses.
	DefaultDelayHours int `json:"default_delay_hours"`
	// Available is false when a feature the journey depends on is not installed
	// (the verification reminder needs e-mail verification): it does not run.
	Available bool `json:"available"`
	// Note says why a journey does not reach anyone right now: "verification_unavailable"
	// (the e-mail verification feature is not installed, the journey does not
	// run) or "referrals_disabled" (the referral program is off: due messages
	// are recorded as skipped). Empty otherwise.
	Note string `json:"note,omitempty"`
	// Promotional journeys reach e-mail and push only for people who opted in
	// to marketing; the others are transactional.
	Promotional bool `json:"promotional"`
	// Channels the journey uses: in_app, email, push.
	Channels []string                         `json:"channels"`
	Stats    map[string]LifecycleJourneyStats `json:"stats"` // keys "7" and "30"
}

// LifecycleView is GET /admin/lifecycle.
type LifecycleView struct {
	Journeys        []LifecycleJourneyView `json:"journeys"`
	AttributionDays int                    `json:"attribution_days"`
	Version         int                    `json:"version"`
	UpdatedAt       time.Time              `json:"updated_at"`
	UpdatedBy       *UserRef               `json:"updated_by,omitempty"`
}

// LifecycleJourneyInput changes a journey; nil fields stay as they are.
type LifecycleJourneyInput struct {
	Enabled    *bool `json:"enabled"`
	DelayHours *int  `json:"delay_hours"`
}

// LifecycleInput is PUT /admin/lifecycle.
type LifecycleInput struct {
	Journeys        map[string]LifecycleJourneyInput `json:"journeys"`
	AttributionDays *int                             `json:"attribution_days"`
	// Version is the version the admin read; a stale one is a 409.
	Version *int `json:"version"`
}

var lifecycleChannels = map[string][]string{
	domain.JourneyVerifyReminder:    {"in_app", "email"},
	domain.JourneyFirstReading:      {"in_app", "email", "push"},
	domain.JourneyAfterFirstReading: {"in_app", "email", "push"},
	domain.JourneyWinBack:           {"in_app", "email", "push"},
}

// Lifecycle returns the setting with every journey's statistics.
func (s *LifecycleService) Lifecycle(ctx context.Context) (*LifecycleView, error) {
	st, err := s.d.Site.Setting(ctx, domain.SiteSettingLifecycle)
	if err != nil && !isNotFound(err) {
		return nil, mapRepoErr(err, "Setting")
	}
	set := lifecycleFrom(st)
	rows, err := s.d.Repo.Stats(ctx, s.d.Clock.Now(), set.AttributionDays)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	view := &LifecycleView{AttributionDays: set.AttributionDays, Version: 1, UpdatedAt: s.d.Clock.Now()}
	if st != nil {
		view.Version, view.UpdatedAt = st.Version, st.UpdatedAt
		if st.UpdatedBy != nil {
			if names, err := s.d.Site.UserNames(ctx, []uuid.UUID{*st.UpdatedBy}); err == nil {
				if name, ok := names[*st.UpdatedBy]; ok {
					view.UpdatedBy = &UserRef{ID: *st.UpdatedBy, Name: name}
				}
			}
		}
	}
	def := defaultLifecycle()
	referralsOn := s.referralsOn(ctx)
	for _, key := range domain.LifecycleJourneys {
		j := set.Journeys[key]
		v := LifecycleJourneyView{Key: key, Enabled: j.Enabled, DelayHours: j.DelayHours, DefaultDelayHours: def.Journeys[key].DelayHours,
			Available: s.journeyAvailable(key), Promotional: key != domain.JourneyVerifyReminder, Channels: lifecycleChannels[key],
			Stats: map[string]LifecycleJourneyStats{"7": newJourneyStats(), "30": newJourneyStats()}}
		switch {
		case !v.Available:
			v.Note = lifecycleNoteNoVerification
		case key == domain.JourneyAfterFirstReading && !referralsOn:
			v.Note = domain.LifecycleSkipReferralsOff
		}
		view.Journeys = append(view.Journeys, v)
	}
	index := map[string]*LifecycleJourneyView{}
	for i := range view.Journeys {
		index[view.Journeys[i].Key] = &view.Journeys[i]
	}
	for _, r := range rows {
		v := index[r.Journey]
		if v == nil {
			continue
		}
		for window, n := range map[string][2]int64{"7": {r.N7, r.Converted7}, "30": {r.N30, r.Converted30}} {
			st := v.Stats[window]
			if r.Status == domain.LifecycleSent {
				st.Sent += n[0]
				st.Converted += n[1]
			} else if n[0] > 0 {
				st.Skipped += n[0]
				st.SkippedReasons[r.SkipReason] += n[0]
			}
			v.Stats[window] = st
		}
	}
	for _, v := range view.Journeys {
		for window, st := range v.Stats {
			if st.Sent > 0 {
				st.ConversionRate = float64(st.Converted) / float64(st.Sent)
			}
			v.Stats[window] = st
		}
	}
	return view, nil
}

func newJourneyStats() LifecycleJourneyStats {
	return LifecycleJourneyStats{SkippedReasons: map[string]int64{}}
}

// UpdateLifecycle changes the setting. A stale version is a 409
// version_conflict; an unchanged value is not a new version.
func (s *LifecycleService) UpdateLifecycle(ctx context.Context, in LifecycleInput) (*LifecycleView, error) {
	fields := map[string]string{}
	if len(in.Journeys) == 0 && in.AttributionDays == nil {
		fields["journeys"] = msgLifecycleEmpty
	}
	known := map[string]bool{}
	for _, k := range domain.LifecycleJourneys {
		known[k] = true
	}
	keys := make([]string, 0, len(in.Journeys))
	for key := range in.Journeys {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		j := in.Journeys[key]
		switch {
		case !known[key]:
			fields["journeys."+key] = msgLifecycleJourney
		case j.DelayHours != nil && (*j.DelayHours < lifecycleDelayMin || *j.DelayHours > lifecycleDelayMax):
			fields["journeys."+key+".delay_hours"] = msgLifecycleDelay
		}
	}
	if in.AttributionDays != nil && (*in.AttributionDays < 1 || *in.AttributionDays > 30) {
		fields["attribution_days"] = msgLifecycleDays
	}
	if in.Version != nil && *in.Version < 1 {
		fields["version"] = msgSiteWrongType
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	var before, after LifecycleSettings
	changed := false
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		changed = false
		st, err := s.d.Site.SettingForUpdate(ctx, domain.SiteSettingLifecycle)
		if err != nil {
			return err
		}
		before = lifecycleFrom(st)
		if in.Version != nil && *in.Version != st.Version {
			return apperror.VersionConflict(st.Version)
		}
		after = lifecycleFrom(st)
		for key, j := range in.Journeys {
			cur := after.Journeys[key]
			if j.Enabled != nil {
				cur.Enabled = *j.Enabled
			}
			if j.DelayHours != nil {
				cur.DelayHours = *j.DelayHours
			}
			after.Journeys[key] = cur
		}
		if in.AttributionDays != nil {
			after.AttributionDays = *in.AttributionDays
		}
		if sameLifecycle(before, after) {
			return nil
		}
		changed = true
		return s.d.Site.SaveSetting(ctx, domain.SiteSettingLifecycle, after.toMap(), st.Version+1, actor(ctx), s.d.Clock.Now())
	})
	if err != nil {
		return nil, mapRepoErr(err, "Setting")
	}
	if changed {
		recordAudit(ctx, s.d.Audit, s.d.Clock, s.d.IPHashSecret, "lifecycle.settings.updated", "site_setting", domain.SiteSettingLifecycle,
			map[string]any{"before": before.toMap(), "after": after.toMap()})
	}
	return s.Lifecycle(ctx)
}

func sameLifecycle(a, b LifecycleSettings) bool {
	if a.AttributionDays != b.AttributionDays || len(a.Journeys) != len(b.Journeys) {
		return false
	}
	for key, j := range a.Journeys {
		if b.Journeys[key] != j {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Activation funnel
// ---------------------------------------------------------------------------

// FunnelStep is one step of the activation funnel.
type FunnelStep struct {
	Key string `json:"key"`
	// Count is the clients of the cohort who reached the step; null when the
	// step cannot be counted in this deployment (the referral program is not
	// installed).
	Count *int64 `json:"count"`
	// Rate is Count / the cohort's sign-ups.
	Rate float64 `json:"rate"`
}

// LifecycleFunnelView is GET /admin/lifecycle/funnel.
type LifecycleFunnelView struct {
	Days  int          `json:"days"`
	From  time.Time    `json:"from"`
	To    time.Time    `json:"to"`
	Steps []FunnelStep `json:"steps"`
}

// Funnel counts the sign-up cohort of the last days at each activation step:
// signed up, verified the e-mail, added birth details, completed a first
// reading, invited a friend and completed a second reading. A step counts
// clients who ever reached it (not in order): they are the same accounts at
// every step. One aggregated query, plus the referral program's own count;
// the result is cached for a minute per window. days must be 1..90 (the
// handler applies LifecycleFunnelDefaultDays when the parameter is absent).
func (s *LifecycleService) Funnel(ctx context.Context, days int) (*LifecycleFunnelView, error) {
	if days < 1 || days > lifecycleFunnelMaxDays {
		return nil, apperror.Validation(map[string]string{"days": msgLifecycleWindow})
	}
	key := "lifecycle:funnel:" + strconv.Itoa(days)
	if s.d.Cache != nil {
		if raw, ok, err := s.d.Cache.Get(ctx, key); err == nil && ok {
			var cached LifecycleFunnelView
			if json.Unmarshal([]byte(raw), &cached) == nil {
				return &cached, nil
			}
		}
	}
	out, err := s.funnel(ctx, days)
	if err != nil {
		return nil, err
	}
	if s.d.Cache != nil {
		if raw, err := json.Marshal(out); err == nil {
			warnErr(ctx, "lifecycle: cache funnel", s.d.Cache.Set(ctx, key, string(raw), lifecycleFunnelTTL))
		}
	}
	return out, nil
}

func (s *LifecycleService) funnel(ctx context.Context, days int) (*LifecycleFunnelView, error) {
	to := s.d.Clock.Now()
	from := to.AddDate(0, 0, -days)
	f, err := s.d.Repo.Funnel(ctx, from, to)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var invited *int64
	s.mu.RLock()
	count := s.invitedInCohort
	s.mu.RUnlock()
	if count != nil {
		n, err := count(ctx, from, to)
		if err != nil {
			warnErr(ctx, "lifecycle: count invited cohort", err)
		} else {
			invited = &n
		}
	}
	step := func(key string, n int64) FunnelStep {
		v := n
		return FunnelStep{Key: key, Count: &v, Rate: funnelRatio(n, f.SignedUp)}
	}
	out := &LifecycleFunnelView{Days: days, From: from, To: to, Steps: []FunnelStep{
		step("signed_up", f.SignedUp), step("verified_email", f.Verified), step("birth_details", f.BirthDetails),
		step("first_reading", f.FirstReading)}}
	inv := FunnelStep{Key: "invited_friend", Count: invited}
	if invited != nil {
		inv.Rate = funnelRatio(*invited, f.SignedUp)
	}
	out.Steps = append(out.Steps, inv, step("second_reading", f.SecondReading))
	return out, nil
}

func funnelRatio(n, d int64) float64 {
	if d <= 0 {
		return 0
	}
	return float64(n) / float64(d)
}
