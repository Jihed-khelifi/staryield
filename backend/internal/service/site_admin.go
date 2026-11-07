package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 3.0: site settings, admin side (§15.2–§15.4).

// AdminBanner is a banner as the admin console sees it.
type AdminBanner struct {
	ID        uuid.UUID         `json:"id"`
	Text      map[string]string `json:"text"`
	CTALabel  map[string]string `json:"cta_label"`
	Action    BannerAction      `json:"action"`
	Audience  string            `json:"audience"`
	Active    bool              `json:"active"`
	StartsAt  *time.Time        `json:"starts_at,omitempty"`
	EndsAt    *time.Time        `json:"ends_at,omitempty"`
	SortOrder int               `json:"sort_order"`
	Version   int               `json:"version"`
	Status    string            `json:"status"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	UpdatedBy *UserRef          `json:"updated_by,omitempty"`
}

// AdminBanners is GET /admin/site/banners.
type AdminBanners struct {
	Settings BannerSettings `json:"settings"`
	Items    []AdminBanner  `json:"items"`
}

// BannerActionInput is the action of a banner write.
type BannerActionInput struct {
	Type string `json:"type"`
	Href string `json:"href"`
}

// BannerInput creates a banner (POST). Text (en) and action are required.
type BannerInput struct {
	Text     map[string]string  `json:"text"`
	CTALabel map[string]string  `json:"cta_label"`
	Action   *BannerActionInput `json:"action"`
	Audience *string            `json:"audience"`
	Active   *bool              `json:"active"`
	StartsAt *time.Time         `json:"starts_at"`
	EndsAt   *time.Time         `json:"ends_at"`
}

// OptTime is an optional timestamp that distinguishes "absent" from an
// explicit null (which clears the value).
type OptTime struct {
	Set   bool
	Value *time.Time
}

// UnmarshalJSON implements json.Unmarshaler.
func (o *OptTime) UnmarshalJSON(b []byte) error {
	o.Set = true
	if string(b) == "null" {
		o.Value = nil
		return nil
	}
	var t time.Time
	if err := json.Unmarshal(b, &t); err != nil {
		return err
	}
	o.Value = &t
	return nil
}

// BannerPatch updates a banner (PATCH); absent fields are unchanged and a
// text or cta_label object replaces the whole object.
type BannerPatch struct {
	ExpectedVersion *int               `json:"expected_version"`
	Text            map[string]string  `json:"text"`
	CTALabel        map[string]string  `json:"cta_label"`
	Action          *BannerActionInput `json:"action"`
	Audience        *string            `json:"audience"`
	Active          *bool              `json:"active"`
	StartsAt        OptTime            `json:"starts_at"`
	EndsAt          OptTime            `json:"ends_at"`
}

// BannerSettingsPatch is PATCH /admin/site/banners/settings.
type BannerSettingsPatch struct {
	ExpectedVersion *int  `json:"expected_version"`
	Enabled         *bool `json:"enabled"`
	RotationSeconds *int  `json:"rotation_seconds"`
}

func (s *SiteService) adminBanner(b *domain.SiteBanner, now time.Time, names map[uuid.UUID]string) AdminBanner {
	out := AdminBanner{ID: b.ID, Text: map[string]string(b.Text), CTALabel: map[string]string(b.CTALabel),
		Action: BannerAction{Type: b.ActionType, Href: b.ActionHref}, Audience: b.Audience, Active: b.Active,
		StartsAt: b.StartsAt, EndsAt: b.EndsAt, SortOrder: b.SortOrder, Version: b.Version,
		Status: string(b.StatusAt(now)), CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt}
	if out.Text == nil {
		out.Text = map[string]string{}
	}
	if out.CTALabel == nil {
		out.CTALabel = map[string]string{}
	}
	if b.UpdatedBy != nil {
		if name, ok := names[*b.UpdatedBy]; ok {
			out.UpdatedBy = &UserRef{ID: *b.UpdatedBy, Name: name}
		}
	}
	return out
}

func (s *SiteService) names(ctx context.Context, ids ...*uuid.UUID) map[uuid.UUID]string {
	var list []uuid.UUID
	for _, id := range ids {
		if id != nil {
			list = append(list, *id)
		}
	}
	names, err := s.d.Repo.UserNames(ctx, list)
	if err != nil {
		warnErr(ctx, "site: resolve admin names", err)
		return map[uuid.UUID]string{}
	}
	return names
}

// bannerSettings reads the promo bar switch straight from the database.
func (s *SiteService) bannerSettings(ctx context.Context) (BannerSettings, error) {
	st, err := s.d.Repo.Setting(ctx, domain.SiteSettingBanners)
	if err != nil {
		return BannerSettings{}, mapRepoErr(err, "Setting")
	}
	return bannerSettingsFrom(*st), nil
}

// Banners lists every banner (inactive and expired included) with the
// promo bar's settings. It reads the database, never the cache, so the
// versions it returns are the ones writes check.
func (s *SiteService) Banners(ctx context.Context) (*AdminBanners, error) {
	settings, err := s.bannerSettings(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.d.Repo.Banners(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &AdminBanners{Settings: settings, Items: s.adminBanners(ctx, rows)}, nil
}

func (s *SiteService) adminBanners(ctx context.Context, rows []domain.SiteBanner) []AdminBanner {
	var by []*uuid.UUID
	for i := range rows {
		by = append(by, rows[i].UpdatedBy)
	}
	names := s.names(ctx, by...)
	now := s.d.Clock.Now()
	out := make([]AdminBanner, 0, len(rows))
	for i := range rows {
		out = append(out, s.adminBanner(&rows[i], now, names))
	}
	return out
}

// validateAction checks a link action and returns the type and href to store.
func (s *SiteService) validateAction(in *BannerActionInput, fields map[string]string) (typ, href string) {
	if in == nil {
		fields["action.type"] = msgBannerRequired
		return "", ""
	}
	switch in.Type {
	case domain.ActionNone:
		return domain.ActionNone, ""
	case domain.ActionLink:
		h, msg := s.hrefs.check(in.Href)
		if msg != "" {
			fields["action.href"] = msg
			return domain.ActionLink, ""
		}
		return domain.ActionLink, h
	}
	fields["action.type"] = msgBannerActionType
	return "", ""
}

// CreateBanner appends a banner. It fails with 409 limit_reached beyond 10.
func (s *SiteService) CreateBanner(ctx context.Context, in BannerInput) (*AdminBanner, error) {
	fields := map[string]string{}
	text := validateL10n("text", in.Text, domain.BannerTextMax, true, fields)
	cta := validateL10n("cta_label", in.CTALabel, domain.BannerCTAMax, false, fields)
	typ, href := s.validateAction(in.Action, fields)
	audience := domain.AudienceAll
	if in.Audience != nil {
		if audience = *in.Audience; !validAudience(audience) {
			fields["audience"] = msgBannerAudience
		}
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	if !validSchedule(in.StartsAt, in.EndsAt) {
		fields["ends_at"] = msgBannerSchedule
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}

	now := s.d.Clock.Now()
	by := actor(ctx)
	b := &domain.SiteBanner{ID: domain.NewID(), Text: text, CTALabel: cta, ActionType: typ, ActionHref: href, Audience: audience,
		Active: active, StartsAt: utcOrNil(in.StartsAt), EndsAt: utcOrNil(in.EndsAt), Version: 1, CreatedBy: by, UpdatedBy: by,
		CreatedAt: now, UpdatedAt: now}
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		// The banners row serialises creates, deletes and reorders, so the
		// limit cannot be beaten by concurrent requests.
		if _, err := s.d.Repo.SettingForUpdate(ctx, domain.SiteSettingBanners); err != nil {
			return err
		}
		rows, err := s.d.Repo.Banners(ctx)
		if err != nil {
			return err
		}
		if len(rows) >= domain.MaxSiteBanners {
			return apperror.Conflict(apperror.CodeLimitReached, msgBannerLimit).
				WithDetails(map[string]any{"limit": domain.MaxSiteBanners})
		}
		b.SortOrder = 0
		for i := range rows {
			b.SortOrder = max(b.SortOrder, rows[i].SortOrder+1)
		}
		return s.d.Repo.CreateBanner(ctx, b)
	})
	if err != nil {
		return nil, mapRepoErr(err, "Banner")
	}
	s.changed(ctx)
	s.record(ctx, "site_banner.created", "site_banner", b.ID.String(), map[string]any{"audience": audience, "active": active,
		"action": typ, "languages": strings.Join(localesOf(text), ",")})
	out := s.adminBanner(b, now, s.names(ctx, by))
	return &out, nil
}

// UpdateBanner changes a banner under its row lock; expected_version guards
// against lost updates.
func (s *SiteService) UpdateBanner(ctx context.Context, id uuid.UUID, in BannerPatch) (*AdminBanner, error) {
	fields := map[string]string{}
	var text, cta domain.L10n
	if in.Text != nil {
		text = validateL10n("text", in.Text, domain.BannerTextMax, true, fields)
	}
	if in.CTALabel != nil {
		cta = validateL10n("cta_label", in.CTALabel, domain.BannerCTAMax, false, fields)
	}
	var typ, href string
	if in.Action != nil {
		typ, href = s.validateAction(in.Action, fields)
	}
	if in.Audience != nil && !validAudience(*in.Audience) {
		fields["audience"] = msgBannerAudience
	}
	if in.ExpectedVersion != nil && *in.ExpectedVersion < 1 {
		fields["expected_version"] = msgSiteWrongType
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}

	var saved *domain.SiteBanner
	var changed []string
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		changed = nil
		b, err := s.d.Repo.BannerForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if in.ExpectedVersion != nil && *in.ExpectedVersion != b.Version {
			return apperror.VersionConflict(b.Version)
		}
		next := *b
		if in.Text != nil && !equalL10n(text, b.Text) {
			next.Text = text
			changed = append(changed, "text")
		}
		if in.CTALabel != nil && !equalL10n(cta, b.CTALabel) {
			next.CTALabel = cta
			changed = append(changed, "cta_label")
		}
		if in.Action != nil && (typ != b.ActionType || href != b.ActionHref) {
			next.ActionType, next.ActionHref = typ, href
			changed = append(changed, "action")
		}
		if in.Audience != nil && *in.Audience != b.Audience {
			next.Audience = *in.Audience
			changed = append(changed, "audience")
		}
		if in.Active != nil && *in.Active != b.Active {
			next.Active = *in.Active
			changed = append(changed, "active")
		}
		if in.StartsAt.Set && !sameTime(in.StartsAt.Value, b.StartsAt) {
			next.StartsAt = utcOrNil(in.StartsAt.Value)
			changed = append(changed, "starts_at")
		}
		if in.EndsAt.Set && !sameTime(in.EndsAt.Value, b.EndsAt) {
			next.EndsAt = utcOrNil(in.EndsAt.Value)
			changed = append(changed, "ends_at")
		}
		if !validSchedule(next.StartsAt, next.EndsAt) {
			return apperror.Validation(map[string]string{"ends_at": msgBannerSchedule})
		}
		saved = b
		if len(changed) == 0 {
			return nil
		}
		next.Version, next.UpdatedBy, next.UpdatedAt = b.Version+1, actor(ctx), s.d.Clock.Now()
		saved = &next
		return s.d.Repo.SaveBanner(ctx, &next)
	})
	if err != nil {
		return nil, mapRepoErr(err, "Banner")
	}
	if len(changed) > 0 {
		s.changed(ctx)
		s.record(ctx, "site_banner.updated", "site_banner", id.String(), map[string]any{"fields": strings.Join(changed, ",")})
	}
	out := s.adminBanner(saved, s.d.Clock.Now(), s.names(ctx, saved.UpdatedBy))
	return &out, nil
}

// DeleteBanner removes a banner.
func (s *SiteService) DeleteBanner(ctx context.Context, id uuid.UUID) error {
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := s.d.Repo.SettingForUpdate(ctx, domain.SiteSettingBanners); err != nil {
			return err
		}
		return s.d.Repo.DeleteBanner(ctx, id)
	})
	if err != nil {
		return mapRepoErr(err, "Banner")
	}
	s.changed(ctx)
	s.record(ctx, "site_banner.deleted", "site_banner", id.String(), nil)
	return nil
}

// ReorderBanners sets the display order: ids must list every banner
// exactly once.
func (s *SiteService) ReorderBanners(ctx context.Context, ids []uuid.UUID) ([]AdminBanner, error) {
	seen := map[uuid.UUID]bool{}
	for _, id := range ids {
		if seen[id] {
			return nil, apperror.Validation(map[string]string{"ids": msgBannerOrder})
		}
		seen[id] = true
	}
	var rows []domain.SiteBanner
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := s.d.Repo.SettingForUpdate(ctx, domain.SiteSettingBanners); err != nil {
			return err
		}
		current, err := s.d.Repo.Banners(ctx)
		if err != nil {
			return err
		}
		if len(current) != len(ids) {
			return apperror.Validation(map[string]string{"ids": msgBannerOrder})
		}
		for i := range current {
			if !seen[current[i].ID] {
				return apperror.Validation(map[string]string{"ids": msgBannerOrder})
			}
		}
		if err := s.d.Repo.SetBannerOrder(ctx, ids, actor(ctx), s.d.Clock.Now()); err != nil {
			return err
		}
		rows, err = s.d.Repo.Banners(ctx)
		return err
	})
	if err != nil {
		return nil, mapRepoErr(err, "Banner")
	}
	s.changed(ctx)
	strIDs := make([]string, len(ids))
	for i, id := range ids {
		strIDs[i] = id.String()
	}
	s.record(ctx, "site_banner.reordered", "site_banner", "", map[string]any{"ids": strings.Join(strIDs, ",")})
	return s.adminBanners(ctx, rows), nil
}

// UpdateBannerSettings changes the promo bar switch and rotation speed.
func (s *SiteService) UpdateBannerSettings(ctx context.Context, in BannerSettingsPatch) (*BannerSettings, error) {
	fields := map[string]string{}
	if in.RotationSeconds != nil && (*in.RotationSeconds < domain.BannerRotationMin || *in.RotationSeconds > domain.BannerRotationMax) {
		fields["rotation_seconds"] = msgBannerRotation
	}
	if in.ExpectedVersion != nil && *in.ExpectedVersion < 1 {
		fields["expected_version"] = msgSiteWrongType
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	var out BannerSettings
	var changed []string
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		changed = nil
		st, err := s.d.Repo.SettingForUpdate(ctx, domain.SiteSettingBanners)
		if err != nil {
			return err
		}
		cur := bannerSettingsFrom(*st)
		if in.ExpectedVersion != nil && *in.ExpectedVersion != cur.Version {
			return apperror.VersionConflict(cur.Version)
		}
		next := cur
		if in.Enabled != nil && *in.Enabled != cur.Enabled {
			next.Enabled = *in.Enabled
			changed = append(changed, "enabled")
		}
		if in.RotationSeconds != nil && *in.RotationSeconds != cur.RotationSeconds {
			next.RotationSeconds = *in.RotationSeconds
			changed = append(changed, "rotation_seconds")
		}
		out = cur
		if len(changed) == 0 {
			return nil
		}
		next.Version, next.UpdatedAt = cur.Version+1, s.d.Clock.Now()
		out = next
		return s.d.Repo.SaveSetting(ctx, domain.SiteSettingBanners,
			domain.JSONMap{"enabled": next.Enabled, "rotation_seconds": next.RotationSeconds}, next.Version, actor(ctx), next.UpdatedAt)
	})
	if err != nil {
		return nil, mapRepoErr(err, "Setting")
	}
	if len(changed) > 0 {
		s.changed(ctx)
		s.record(ctx, "site_banner.settings_updated", "site_setting", domain.SiteSettingBanners,
			map[string]any{"fields": strings.Join(changed, ",")})
	}
	return &out, nil
}

// ---------------------------------------------------------------------------
// Sign-up offer
// ---------------------------------------------------------------------------

// SignupOfferInput is PUT /admin/site/signup-offer.
type SignupOfferInput struct {
	Enabled         *bool `json:"enabled"`
	FreeMinutes     *int  `json:"free_minutes"`
	ExpectedVersion *int  `json:"expected_version"`
}

func (s *SiteService) signupView(ctx context.Context, st *domain.SiteSetting) *SignupOffer {
	o := signupOfferFrom(*st, s.d.Cfg.SignupFallbackMinutes)
	if st.UpdatedBy != nil {
		if name, ok := s.names(ctx, st.UpdatedBy)[*st.UpdatedBy]; ok {
			o.UpdatedBy = &UserRef{ID: *st.UpdatedBy, Name: name}
		}
	}
	return &o
}

// SignupOffer returns the stored offer (database, never the cache).
func (s *SiteService) SignupOffer(ctx context.Context) (*SignupOffer, error) {
	st, err := s.d.Repo.Setting(ctx, domain.SiteSettingSignupOffer)
	if err != nil {
		return nil, mapRepoErr(err, "Setting")
	}
	return s.signupView(ctx, st), nil
}

// UpdateSignupOffer replaces the offer. Disabling keeps the stored minutes.
func (s *SiteService) UpdateSignupOffer(ctx context.Context, in SignupOfferInput) (*SignupOffer, error) {
	fields := map[string]string{}
	if in.Enabled == nil {
		fields["enabled"] = msgBannerRequired
	}
	if in.FreeMinutes == nil {
		fields["free_minutes"] = msgBannerRequired
	} else if *in.FreeMinutes < domain.SignupFreeMinutesMin || *in.FreeMinutes > domain.SignupFreeMinutesMax {
		fields["free_minutes"] = msgSignupMinutes
	}
	if in.ExpectedVersion != nil && *in.ExpectedVersion < 1 {
		fields["expected_version"] = msgSiteWrongType
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	var before, after SignupOffer
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		st, err := s.d.Repo.SettingForUpdate(ctx, domain.SiteSettingSignupOffer)
		if err != nil {
			return err
		}
		before = signupOfferFrom(*st, s.d.Cfg.SignupFallbackMinutes)
		if in.ExpectedVersion != nil && *in.ExpectedVersion != before.Version {
			return apperror.VersionConflict(before.Version)
		}
		after = before
		after.Enabled, after.FreeMinutes = *in.Enabled, *in.FreeMinutes
		if after.Enabled == before.Enabled && after.FreeMinutes == before.FreeMinutes {
			return nil
		}
		after.Version, after.UpdatedAt = before.Version+1, s.d.Clock.Now()
		return s.d.Repo.SaveSetting(ctx, domain.SiteSettingSignupOffer,
			domain.JSONMap{"enabled": after.Enabled, "free_minutes": after.FreeMinutes}, after.Version, actor(ctx), after.UpdatedAt)
	})
	if err != nil {
		return nil, mapRepoErr(err, "Setting")
	}
	if after.Version != before.Version {
		s.changed(ctx)
		s.record(ctx, "site.signup_offer_updated", "site_setting", domain.SiteSettingSignupOffer, map[string]any{
			"before": map[string]any{"enabled": before.Enabled, "free_minutes": before.FreeMinutes},
			"after":  map[string]any{"enabled": after.Enabled, "free_minutes": after.FreeMinutes}})
	}
	return s.SignupOffer(ctx)
}

// ---------------------------------------------------------------------------
// Free mode
// ---------------------------------------------------------------------------

// FreeModeInput is PUT /admin/site/free-mode.
type FreeModeInput struct {
	Enabled         *bool      `json:"enabled"`
	Confirmation    string     `json:"confirmation"`
	Reason          *string    `json:"reason"`
	EndsAt          *time.Time `json:"ends_at"`
	ExpectedVersion *int       `json:"expected_version"`
}

// FreeModeStats are the sponsored-chat totals since free mode was switched on.
type FreeModeStats struct {
	Since                 *time.Time `json:"since,omitempty"`
	SponsoredSessions     int64      `json:"sponsored_sessions"`
	LiveSponsoredSessions int64      `json:"live_sponsored_sessions"`
	SponsoredMinutes      int64      `json:"sponsored_minutes"`
	SponsoredCredits      int64      `json:"sponsored_credits"`
	AdvisorEarnings       int64      `json:"advisor_earnings_credits"`
}

// FreeModeState is the free-mode document with its statistics.
type FreeModeState struct {
	Enabled    bool          `json:"enabled"`
	Active     bool          `json:"active"`
	EndsAt     *time.Time    `json:"ends_at,omitempty"`
	Reason     string        `json:"reason"`
	EnabledAt  *time.Time    `json:"enabled_at,omitempty"`
	EnabledBy  *UserRef      `json:"enabled_by,omitempty"`
	DisabledAt *time.Time    `json:"disabled_at,omitempty"`
	DisabledBy *UserRef      `json:"disabled_by,omitempty"`
	Version    int           `json:"version"`
	Stats      FreeModeStats `json:"stats"`
}

func (s *SiteService) freeModeState(ctx context.Context, st *domain.SiteSetting) *FreeModeState {
	d := freeModeFrom(*st)
	names := s.names(ctx, d.EnabledBy, d.DisabledBy)
	ref := func(id *uuid.UUID) *UserRef {
		if id == nil {
			return nil
		}
		if name, ok := names[*id]; ok {
			return &UserRef{ID: *id, Name: name}
		}
		return &UserRef{ID: *id}
	}
	return &FreeModeState{Enabled: d.Enabled, Active: d.activeAt(s.d.Clock.Now()), EndsAt: d.EndsAt, Reason: d.Reason,
		EnabledAt: d.EnabledAt, EnabledBy: ref(d.EnabledBy), DisabledAt: d.DisabledAt, DisabledBy: ref(d.DisabledBy),
		Version: d.Version, Stats: s.freeModeStats(ctx, d.EnabledAt)}
}

// freeModeStats sums the sponsored chats since the period began, cached for
// 15 s (changes of the setting drop it).
func (s *SiteService) freeModeStats(ctx context.Context, since *time.Time) FreeModeStats {
	if since == nil {
		return FreeModeStats{}
	}
	if s.d.Cache != nil {
		if raw, ok, err := s.d.Cache.Get(ctx, siteStatsKey); err == nil && ok {
			var cached FreeModeStats
			if json.Unmarshal([]byte(raw), &cached) == nil && cached.Since != nil && cached.Since.Equal(*since) {
				return cached
			}
		}
	}
	out := FreeModeStats{Since: since}
	totals, err := s.d.Repo.SponsoredTotals(ctx, *since)
	if err != nil {
		warnErr(ctx, "site: free mode statistics", err)
		return out
	}
	earn, err := s.d.Repo.SponsoredEarnings(ctx, *since)
	if err != nil {
		warnErr(ctx, "site: free mode earnings", err)
		return out
	}
	out.SponsoredSessions, out.LiveSponsoredSessions = totals.Sessions, totals.Live
	out.SponsoredMinutes, out.SponsoredCredits, out.AdvisorEarnings = totals.Minutes, totals.Credits, earn
	if s.d.Cache != nil {
		if b, err := json.Marshal(out); err == nil {
			warnErr(ctx, "site: cache free mode statistics", s.d.Cache.Set(ctx, siteStatsKey, string(b), siteStatsTTL))
		}
	}
	return out
}

// FreeMode returns the free-mode state (database, never the cache).
func (s *SiteService) FreeMode(ctx context.Context) (*FreeModeState, error) {
	st, err := s.d.Repo.Setting(ctx, domain.SiteSettingFreeMode)
	if err != nil {
		return nil, mapRepoErr(err, "Setting")
	}
	return s.freeModeState(ctx, st), nil
}

// UpdateFreeMode switches free mode on or off. Switching on needs the exact
// confirmation phrase, a reason and (optionally) an end within 90 days. A
// chat requested while it is active stays sponsored after it ends.
func (s *SiteService) UpdateFreeMode(ctx context.Context, in FreeModeInput) (*FreeModeState, error) {
	fields := map[string]string{}
	now := s.d.Clock.Now()
	reason := ""
	if in.Reason != nil {
		reason = cleanBannerLine(*in.Reason)
	}
	switch {
	case in.Enabled == nil:
		fields["enabled"] = msgBannerRequired
	case *in.Enabled:
		if in.Confirmation != domain.FreeModeConfirmation {
			fields["confirmation"] = msgFreeConfirmation
		}
		if n := utf8.RuneCountInString(reason); n < 5 || n > 500 {
			fields["reason"] = msgFreeReason
		}
		if in.EndsAt != nil && !validFreeModeEnd(*in.EndsAt, now) {
			fields["ends_at"] = msgFreeEnds
		}
	default:
		if utf8.RuneCountInString(reason) > 500 {
			fields["reason"] = msgFreeReasonLong
		}
	}
	if in.ExpectedVersion != nil && *in.ExpectedVersion < 1 {
		fields["expected_version"] = msgSiteWrongType
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}

	var before, after freeModeDoc
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		st, err := s.d.Repo.SettingForUpdate(ctx, domain.SiteSettingFreeMode)
		if err != nil {
			return err
		}
		before = freeModeFrom(*st)
		if in.ExpectedVersion != nil && *in.ExpectedVersion != before.Version {
			return apperror.VersionConflict(before.Version)
		}
		after = before
		by := actor(ctx)
		if *in.Enabled {
			after.Enabled, after.Reason = true, reason
			after.EndsAt = utcOrNil(in.EndsAt)
			// A period already running keeps its start; a switched-off or
			// ended one begins a new period.
			if !before.activeAt(now) || before.EnabledAt == nil {
				after.EnabledAt, after.EnabledBy = &now, by
			}
			after.DisabledAt, after.DisabledBy = nil, nil
		} else {
			if !before.Enabled {
				return nil // already off: nothing to change
			}
			after.Enabled = false
			after.DisabledAt, after.DisabledBy = &now, by
		}
		after.Version, after.UpdatedAt = before.Version+1, now
		return s.d.Repo.SaveSetting(ctx, domain.SiteSettingFreeMode, after.toMap(), after.Version, by, now)
	})
	if err != nil {
		return nil, mapRepoErr(err, "Setting")
	}
	if after.Version != before.Version {
		s.changed(ctx)
		action := "site.free_mode_disabled"
		meta := map[string]any{"previous": map[string]any{"enabled": before.Enabled, "active": before.activeAt(now),
			"reason": before.Reason, "ends_at": timeStr(before.EndsAt)}}
		if after.Enabled {
			action = "site.free_mode_enabled"
			meta["reason"], meta["ends_at"] = after.Reason, timeStr(after.EndsAt)
		} else if reason != "" {
			meta["reason"] = reason
		}
		s.record(ctx, action, "site_setting", domain.SiteSettingFreeMode, meta)
	}
	return s.FreeMode(ctx)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func utcOrNil(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func timeStr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// sameTime compares optional instants at database precision.
func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Truncate(domain.TimePrecision).Equal(b.Truncate(domain.TimePrecision))
}

func equalL10n(a, b domain.L10n) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func localesOf(l domain.L10n) []string {
	var out []string
	for _, loc := range domain.SiteLocales {
		if l[loc] != "" {
			out = append(out, loc)
		}
	}
	return out
}
