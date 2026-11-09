package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/i18n"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// revisions 3.0: site settings (§15). SiteService owns the runtime site
// configuration: promo banners, the sign-up offer and free mode. Reads are
// served from an in-process copy (10 s) in front of Redis (5 min); every
// admin write drops both and tells the other instances over Pub/Sub.

const (
	// SiteChangedChannel is the Pub/Sub channel (under the key prefix) on
	// which instances announce a site configuration change.
	SiteChangedChannel = "site:config:changed"

	siteCacheKey      = "site:config:v2" // v2: adds off_session (an older copy would read as disabled)
	siteCacheKeyV1    = "site:config:v1" // written by instances that predate off_session; dropped with v2
	siteStatsKey      = "site:free-mode:stats:v1"
	siteLocalTTL      = 10 * time.Second
	siteRedisTTL      = 5 * time.Minute
	siteStatsTTL      = 15 * time.Second
	siteRetryAfterErr = 2 * time.Second
)

// SiteSettings are the site service's configuration values.
type SiteSettings struct {
	// AllowedHosts are the https hosts a banner may link to
	// (STARYIELD_LINKS_ALLOWED_HOSTS).
	AllowedHosts []string
	// SignupFallbackMinutes applies when the offer cannot be read
	// (STARYIELD_BILLING_SIGNUP_FREE_MINUTES).
	SignupFallbackMinutes int
	IPHashSecret          string
}

// SiteDeps are the site service's collaborators.
type SiteDeps struct {
	Tx    database.TxManager
	Repo  repository.SiteRepository
	Cache cache.Store
	Bus   cache.Bus
	Audit repository.AuditRepository
	Clock clock.Clock
	Log   *zap.Logger
	Cfg   SiteSettings
}

// BannerSettings is the promo bar's own switch.
type BannerSettings struct {
	Enabled         bool      `json:"enabled"`
	RotationSeconds int       `json:"rotation_seconds"`
	Version         int       `json:"version"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// SignupOffer is the free-minutes offer of new accounts.
type SignupOffer struct {
	Enabled     bool      `json:"enabled"`
	FreeMinutes int       `json:"free_minutes"`
	Version     int       `json:"version"`
	UpdatedAt   time.Time `json:"updated_at"`
	UpdatedBy   *UserRef  `json:"updated_by,omitempty"`
}

// OffSessionSettings are the kill switch and per-role daily caps of messages
// outside a live chat (revisions 3.3, §22.3). The zero value is "disabled":
// it is what an unreadable setting resolves to.
type OffSessionSettings struct {
	Enabled           bool `json:"enabled"`
	ClientDailyLimit  int  `json:"client_daily_limit"`
	AdvisorDailyLimit int  `json:"advisor_daily_limit"`
}

// LimitFor returns the cap of a role (the conversation's client or advisor).
func (o OffSessionSettings) LimitFor(asPsychic bool) int {
	if asPsychic {
		return o.AdvisorDailyLimit
	}
	return o.ClientDailyLimit
}

// UserRef names the admin behind a change.
type UserRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// freeModeDoc is the stored free-mode document.
type freeModeDoc struct {
	Enabled    bool       `json:"enabled"`
	Reason     string     `json:"reason"`
	EndsAt     *time.Time `json:"ends_at,omitempty"`
	EnabledAt  *time.Time `json:"enabled_at,omitempty"`
	EnabledBy  *uuid.UUID `json:"enabled_by,omitempty"`
	DisabledAt *time.Time `json:"disabled_at,omitempty"`
	DisabledBy *uuid.UUID `json:"disabled_by,omitempty"`
	Version    int        `json:"version"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// activeAt reports whether free mode applies at now: switched on and its
// end (when set) not reached.
func (d freeModeDoc) activeAt(now time.Time) bool {
	return d.Enabled && (d.EndsAt == nil || now.Before(*d.EndsAt))
}

// siteSnapshot is everything the public config and the billing hooks need.
// It is what Redis and the in-process copy hold; per-locale views are
// derived from it at request time, so schedules and free-mode expiry are
// exact even while the snapshot itself is cached.
type siteSnapshot struct {
	Banners BannerSettings `json:"banners"`
	Signup  SignupOffer    `json:"signup"`
	Free    freeModeDoc    `json:"free"`
	// OffSession is disabled (the zero value) in the defaults served when
	// nothing can be read: never open the feature by accident.
	OffSession OffSessionSettings  `json:"off_session"`
	Items      []domain.SiteBanner `json:"items"`
	Defaults   bool                `json:"defaults,omitempty"`
}

// SiteService serves and administers the site configuration.
type SiteService struct {
	d     SiteDeps
	hrefs hrefChecker

	mu       sync.RWMutex
	local    *siteSnapshot
	localExp time.Time // real time; stale copies are kept for outages
	flight   singleflight.Group

	viewMu sync.Mutex
	views  map[string]siteViewMemo
}

// NewSiteService wires the site service.
func NewSiteService(d SiteDeps) *SiteService {
	if d.Log == nil {
		d.Log = zap.NewNop()
	}
	if d.Cfg.SignupFallbackMinutes < domain.SignupFreeMinutesMin {
		d.Cfg.SignupFallbackMinutes = 5
	}
	return &SiteService{d: d, hrefs: newHrefChecker(d.Cfg.AllowedHosts), views: map[string]siteViewMemo{}}
}

func (s *SiteService) record(ctx context.Context, action, targetType, targetID string, meta map[string]any) {
	if s.d.Audit == nil {
		return
	}
	recordAudit(ctx, s.d.Audit, s.d.Clock, s.d.Cfg.IPHashSecret, action, targetType, targetID, meta)
}

// Run listens for change announcements from other instances until ctx ends.
func (s *SiteService) Run(ctx context.Context) {
	if s.d.Bus == nil {
		return
	}
	s.d.Bus.Subscribe(ctx, SiteChangedChannel, func(string) { s.dropLocal() })
}

func (s *SiteService) dropLocal() {
	s.mu.Lock()
	s.localExp = time.Time{} // keep the copy as the outage fallback, but stale
	s.mu.Unlock()
	s.viewMu.Lock()
	s.views = map[string]siteViewMemo{}
	s.viewMu.Unlock()
}

// changed invalidates every copy after a committed write: this instance's,
// Redis, and the other instances' (Pub/Sub).
func (s *SiteService) changed(ctx context.Context) {
	s.dropLocal()
	if s.d.Cache != nil {
		warnErr(ctx, "site: drop cached config", s.d.Cache.Del(ctx, siteCacheKey, siteCacheKeyV1, siteStatsKey))
	}
	if s.d.Bus != nil {
		warnErr(ctx, "site: announce config change", s.d.Bus.Publish(ctx, SiteChangedChannel, "changed"))
	}
}

// defaults is what is served when nothing can be read: no banners, the
// configured sign-up offer, free mode off.
func (s *SiteService) defaults() *siteSnapshot {
	now := s.d.Clock.Now()
	return &siteSnapshot{
		Banners:  BannerSettings{Enabled: false, RotationSeconds: domain.BannerRotationDefault, Version: 1, UpdatedAt: now},
		Signup:   SignupOffer{Enabled: true, FreeMinutes: s.d.Cfg.SignupFallbackMinutes, Version: 1, UpdatedAt: now},
		Free:     freeModeDoc{Version: 1, UpdatedAt: now},
		Items:    []domain.SiteBanner{},
		Defaults: true,
	}
}

// snapshot returns the current configuration: in-process copy, then Redis,
// then PostgreSQL. When PostgreSQL fails the last good copy (however old)
// is served, else the safe defaults; it never returns an error.
func (s *SiteService) snapshot(ctx context.Context) *siteSnapshot {
	s.mu.RLock()
	snap, exp := s.local, s.localExp
	s.mu.RUnlock()
	if snap != nil && time.Now().Before(exp) {
		return snap
	}
	v, _, _ := s.flight.Do("snapshot", func() (any, error) {
		// A detached context: the first caller's cancellation must not fail
		// the others sharing this load.
		lctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		return s.load(lctx), nil
	})
	return v.(*siteSnapshot)
}

func (s *SiteService) load(ctx context.Context) *siteSnapshot {
	if s.d.Cache != nil {
		if raw, ok, err := s.d.Cache.Get(ctx, siteCacheKey); err == nil && ok {
			var snap siteSnapshot
			if json.Unmarshal([]byte(raw), &snap) == nil {
				s.setLocal(&snap, siteLocalTTL)
				return &snap
			}
		}
	}
	snap, err := s.fromDB(ctx)
	if err != nil {
		warnErr(ctx, "site: load configuration (serving the last good copy)", err)
		s.mu.RLock()
		last := s.local
		s.mu.RUnlock()
		if last == nil {
			last = s.defaults()
		}
		// Retry the database soon, without hammering it.
		s.setLocalKeep(last, siteRetryAfterErr)
		return last
	}
	if s.d.Cache != nil {
		if b, err := json.Marshal(snap); err == nil {
			warnErr(ctx, "site: cache configuration", s.d.Cache.Set(ctx, siteCacheKey, string(b), siteRedisTTL))
		}
	}
	s.setLocal(snap, siteLocalTTL)
	return snap
}

func (s *SiteService) setLocal(snap *siteSnapshot, ttl time.Duration) {
	s.mu.Lock()
	s.local, s.localExp = snap, time.Now().Add(ttl)
	s.mu.Unlock()
}

// setLocalKeep re-arms the copy without changing what it holds (outage).
func (s *SiteService) setLocalKeep(snap *siteSnapshot, ttl time.Duration) { s.setLocal(snap, ttl) }

// fromDB reads the whole configuration.
func (s *SiteService) fromDB(ctx context.Context) (*siteSnapshot, error) {
	settings, err := s.d.Repo.Settings(ctx)
	if err != nil {
		return nil, err
	}
	banners, err := s.d.Repo.Banners(ctx)
	if err != nil {
		return nil, err
	}
	snap := s.defaults()
	snap.Defaults = false
	snap.Items = banners
	if snap.Items == nil {
		snap.Items = []domain.SiteBanner{}
	}
	for _, st := range settings {
		switch st.Key {
		case domain.SiteSettingBanners:
			snap.Banners = bannerSettingsFrom(st)
		case domain.SiteSettingSignupOffer:
			snap.Signup = signupOfferFrom(st, s.d.Cfg.SignupFallbackMinutes)
		case domain.SiteSettingFreeMode:
			snap.Free = freeModeFrom(st)
		case domain.SiteSettingOffSession:
			snap.OffSession = offSessionFrom(st)
		}
	}
	return snap, nil
}

// ---------------------------------------------------------------------------
// Hooks for other services
// ---------------------------------------------------------------------------

// SignupFreeMinutes is the number of free minutes a new account gets: the
// offer's minutes when enabled, else 0; the configured fallback when the
// offer cannot be read at all. At most 10 s stale.
func (s *SiteService) SignupFreeMinutes(ctx context.Context) int {
	snap := s.snapshot(ctx)
	if snap.Defaults {
		return s.d.Cfg.SignupFallbackMinutes
	}
	if !snap.Signup.Enabled {
		return 0
	}
	return snap.Signup.FreeMinutes
}

// FreeModeActive reports whether live chats requested now are free. On an
// outage with nothing cached it answers false: never give service away by
// accident.
func (s *SiteService) FreeModeActive(ctx context.Context) bool {
	return s.snapshot(ctx).Free.activeAt(s.d.Clock.Now())
}

// OffSessionSettings returns the messages-outside-a-live-chat settings
// (at most 10 s stale). When they cannot be read and nothing is cached the
// feature is disabled.
func (s *SiteService) OffSessionSettings(ctx context.Context) OffSessionSettings {
	snap := s.snapshot(ctx)
	if snap.Defaults {
		return OffSessionSettings{}
	}
	return snap.OffSession
}

// ---------------------------------------------------------------------------
// Public configuration (GET /site/config)
// ---------------------------------------------------------------------------

// PublicBanner is a banner as the public site receives it.
type PublicBanner struct {
	ID       string       `json:"id"`
	Text     string       `json:"text"`
	CTALabel string       `json:"cta_label"`
	Action   BannerAction `json:"action"`
	Audience string       `json:"audience"`
}

// BannerAction is where a banner leads.
type BannerAction struct {
	Type string `json:"type"`
	Href string `json:"href"`
}

// SiteConfigView is the public site configuration.
type SiteConfigView struct {
	Banners struct {
		RotationSeconds int            `json:"rotation_seconds"`
		Items           []PublicBanner `json:"items"`
	} `json:"banners"`
	SignupOffer struct {
		Enabled     bool `json:"enabled"`
		FreeMinutes int  `json:"free_minutes"`
	} `json:"signup_offer"`
	FreeMode struct {
		Active bool       `json:"active"`
		EndsAt *time.Time `json:"ends_at,omitempty"`
	} `json:"free_mode"`
	GeneratedAt time.Time `json:"generated_at"`
}

// SiteConfigResult is the rendered public configuration with its validator.
type SiteConfigResult struct {
	View SiteConfigView
	// Body is the JSON the endpoint sends; ETag is a weak validator of its
	// content (generated_at excluded, so it only changes with the content).
	Body []byte
	ETag string
}

type siteViewMemo struct {
	snap   *siteSnapshot
	second int64
	result *SiteConfigResult
}

// Config builds the public configuration for a locale: localized with a
// fallback to English, {free_minutes} replaced, only live banners.
func (s *SiteService) Config(ctx context.Context, locale string) (*SiteConfigResult, error) {
	locale = i18n.Normalize(locale)
	snap := s.snapshot(ctx)
	now := s.d.Clock.Now()

	// Rendered output is reused within a second for the same snapshot, so a
	// burst of requests costs one render.
	s.viewMu.Lock()
	memo, ok := s.views[locale]
	s.viewMu.Unlock()
	if ok && memo.snap == snap && memo.second == now.Unix() {
		return memo.result, nil
	}

	res, err := renderSiteConfig(snap, locale, now)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	s.viewMu.Lock()
	s.views[locale] = siteViewMemo{snap: snap, second: now.Unix(), result: res}
	s.viewMu.Unlock()
	return res, nil
}

// renderSiteConfig is the pure view builder.
func renderSiteConfig(snap *siteSnapshot, locale string, now time.Time) (*SiteConfigResult, error) {
	var v SiteConfigView
	minutes := 0
	if snap.Signup.Enabled {
		minutes = snap.Signup.FreeMinutes
	}
	v.SignupOffer.Enabled, v.SignupOffer.FreeMinutes = snap.Signup.Enabled, minutes
	v.Banners.RotationSeconds = snap.Banners.RotationSeconds
	v.Banners.Items = []PublicBanner{}
	if snap.Banners.Enabled {
		for i := range snap.Items {
			b := &snap.Items[i]
			if len(v.Banners.Items) >= domain.MaxSiteBanners || b.StatusAt(now) != domain.BannerLive {
				continue
			}
			// A banner promising free minutes is not shown while the offer
			// is switched off.
			if !snap.Signup.Enabled && (hasPlaceholder(b.Text) || hasPlaceholder(b.CTALabel)) {
				continue
			}
			text := b.Text.Pick(locale)
			if text == "" {
				continue
			}
			v.Banners.Items = append(v.Banners.Items, PublicBanner{ID: b.ID.String(),
				Text: substituteMinutes(text, minutes), CTALabel: substituteMinutes(b.CTALabel.Pick(locale), minutes),
				Action: BannerAction{Type: b.ActionType, Href: b.ActionHref}, Audience: b.Audience})
		}
	}
	if snap.Free.activeAt(now) {
		v.FreeMode.Active = true
		v.FreeMode.EndsAt = snap.Free.EndsAt
	}
	// The validator covers the content only: hash before stamping the time.
	content, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(content)
	v.GeneratedAt = now.UTC()
	body, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &SiteConfigResult{View: v, Body: body, ETag: `W/"` + hex.EncodeToString(sum[:8]) + `"`}, nil
}

// ETagMatches reports whether an If-None-Match header matches etag (weak
// comparison; "*" matches anything).
func ETagMatches(header, etag string) bool {
	if header == "" {
		return false
	}
	want := strings.TrimPrefix(etag, "W/")
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "*" || strings.TrimPrefix(part, "W/") == want {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Settings documents <-> structs
// ---------------------------------------------------------------------------

func mInt(m domain.JSONMap, key string, def int) int {
	if f, ok := m[key].(float64); ok {
		return int(f)
	}
	return def
}

func mBool(m domain.JSONMap, key string) bool {
	b, _ := m[key].(bool)
	return b
}

func mString(m domain.JSONMap, key string) string {
	s, _ := m[key].(string)
	return s
}

func mTime(m domain.JSONMap, key string) *time.Time {
	if s, ok := m[key].(string); ok && s != "" {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return &t
		}
	}
	return nil
}

func mUUID(m domain.JSONMap, key string) *uuid.UUID {
	if s, ok := m[key].(string); ok {
		if id, err := uuid.Parse(s); err == nil {
			return &id
		}
	}
	return nil
}

func bannerSettingsFrom(st domain.SiteSetting) BannerSettings {
	return BannerSettings{Enabled: mBool(st.Value, "enabled"),
		RotationSeconds: mInt(st.Value, "rotation_seconds", domain.BannerRotationDefault),
		Version:         st.Version, UpdatedAt: st.UpdatedAt}
}

func signupOfferFrom(st domain.SiteSetting, fallback int) SignupOffer {
	return SignupOffer{Enabled: mBool(st.Value, "enabled"), FreeMinutes: mInt(st.Value, "free_minutes", fallback),
		Version: st.Version, UpdatedAt: st.UpdatedAt}
}

// offSessionFrom reads the stored document; anything malformed or out of
// range reads as disabled / clamped, never as more permissive.
func offSessionFrom(st domain.SiteSetting) OffSessionSettings {
	clamp := func(key string) int {
		return min(max(mInt(st.Value, key, 0), domain.OffSessionLimitMin), domain.OffSessionLimitMax)
	}
	return OffSessionSettings{Enabled: mBool(st.Value, "enabled"),
		ClientDailyLimit: clamp("client_daily_limit"), AdvisorDailyLimit: clamp("advisor_daily_limit")}
}

func freeModeFrom(st domain.SiteSetting) freeModeDoc {
	return freeModeDoc{Enabled: mBool(st.Value, "enabled"), Reason: mString(st.Value, "reason"),
		EndsAt: mTime(st.Value, "ends_at"), EnabledAt: mTime(st.Value, "enabled_at"), EnabledBy: mUUID(st.Value, "enabled_by"),
		DisabledAt: mTime(st.Value, "disabled_at"), DisabledBy: mUUID(st.Value, "disabled_by"),
		Version: st.Version, UpdatedAt: st.UpdatedAt}
}

func (d freeModeDoc) toMap() domain.JSONMap {
	m := domain.JSONMap{"enabled": d.Enabled, "reason": d.Reason}
	put := func(key string, t *time.Time) {
		if t != nil {
			m[key] = t.UTC().Format(time.RFC3339Nano)
		}
	}
	put("ends_at", d.EndsAt)
	put("enabled_at", d.EnabledAt)
	put("disabled_at", d.DisabledAt)
	if d.EnabledBy != nil {
		m["enabled_by"] = d.EnabledBy.String()
	}
	if d.DisabledBy != nil {
		m["disabled_by"] = d.DisabledBy.String()
	}
	return m
}

// actor returns the signed-in admin's ID (nil when unknown).
func actor(ctx context.Context) *uuid.UUID {
	p, _ := reqctx.PrincipalFrom(ctx)
	if p.UserID == uuid.Nil {
		return nil
	}
	id := p.UserID
	return &id
}
