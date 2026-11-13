package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
)

// revisions 5.0: pricing catalog (§24.1). PricingService serves the public
// pricing summary and the signed-in top-up catalog from one snapshot (active
// packs with their computed badges, the pricing settings): an in-process copy
// (10 s) in front of Redis (5 min), invalidated by every pack or settings
// write on all instances (Pub/Sub). With nothing cached and the database
// down it fails (503): prices are never invented.

const (
	// PricingChangedChannel is the Pub/Sub channel (under the key prefix) on
	// which instances announce a pack or pricing-settings change.
	PricingChangedChannel = "pricing:changed"

	pricingCacheKey      = "pricing:v1"
	pricingLocalTTL      = 10 * time.Second
	pricingRedisTTL      = 5 * time.Minute
	pricingRetryAfterErr = 2 * time.Second
)

// Default pricing settings (migration seed, and what an unreadable document
// resolves to).
const (
	defaultRateMin             = 30
	defaultRateMax             = 60
	defaultPopularMin          = 25
	defaultPopularWindowDays   = 90
	defaultProcessingFeeBps    = 800
	defaultProcessingFixedCent = 30
)

// PricingSettings are the admin-controlled pricing parameters.
type PricingSettings struct {
	RateMin              int       `json:"rate_min"`
	RateMax              int       `json:"rate_max"`
	PopularMinPurchases  int       `json:"popular_min_purchases"`
	PopularWindowDays    int       `json:"popular_window_days"`
	ProcessingFeeBps     int       `json:"processing_fee_bps"`
	ProcessingFixedCents int       `json:"processing_fixed_cents"`
	Version              int       `json:"version"`
	UpdatedAt            time.Time `json:"updated_at"`
	UpdatedBy            *UserRef  `json:"updated_by,omitempty"`
}

func defaultPricingSettings(now time.Time) PricingSettings {
	return PricingSettings{RateMin: defaultRateMin, RateMax: defaultRateMax, PopularMinPurchases: defaultPopularMin,
		PopularWindowDays: defaultPopularWindowDays, ProcessingFeeBps: defaultProcessingFeeBps,
		ProcessingFixedCents: defaultProcessingFixedCent, Version: 1, UpdatedAt: now}
}

// pricingSettingsFrom reads the stored document; a missing or out-of-range
// value falls back to its default (never to something more permissive).
func pricingSettingsFrom(st domain.SiteSetting) PricingSettings {
	in := func(key string, def, lo, hi int) int {
		v := mInt(st.Value, key, def)
		if v < lo || v > hi {
			return def
		}
		return v
	}
	out := PricingSettings{
		RateMin:              in("rate_min", defaultRateMin, 1, 10000),
		RateMax:              in("rate_max", defaultRateMax, 1, 10000),
		PopularMinPurchases:  in("popular_min_purchases", defaultPopularMin, 1, 100000),
		PopularWindowDays:    in("popular_window_days", defaultPopularWindowDays, 7, 365),
		ProcessingFeeBps:     in("processing_fee_bps", defaultProcessingFeeBps, 0, 3000),
		ProcessingFixedCents: in("processing_fixed_cents", defaultProcessingFixedCent, 0, 500),
		Version:              st.Version, UpdatedAt: st.UpdatedAt,
	}
	if out.RateMax < out.RateMin {
		out.RateMin, out.RateMax = defaultRateMin, defaultRateMax
	}
	return out
}

func (p PricingSettings) toMap() domain.JSONMap {
	return domain.JSONMap{"rate_min": p.RateMin, "rate_max": p.RateMax, "popular_min_purchases": p.PopularMinPurchases,
		"popular_window_days": p.PopularWindowDays, "processing_fee_bps": p.ProcessingFeeBps,
		"processing_fixed_cents": p.ProcessingFixedCents}
}

// PricingSettingsConfig are the pricing service's configuration values.
type PricingSettingsConfig struct {
	// Currency is the summary's currency (STARYIELD_PAYMENTS_CURRENCY).
	Currency     string
	IPHashSecret string
	// StatementDescriptor is shown with the top-up catalog (revisions 5.1).
	StatementDescriptor string
}

// SignupOfferFunc reads the live sign-up offer (SiteService.SignupOfferState).
type SignupOfferFunc func(ctx context.Context) (enabled bool, minutes int)

// PricingDeps are the pricing service's collaborators.
type PricingDeps struct {
	Tx     database.TxManager
	Repo   repository.PricingRepository
	Site   repository.SiteRepository // the pricing settings document lives in site_settings
	Signup SignupOfferFunc
	Cache  cache.Store
	Bus    cache.Bus
	Audit  repository.AuditRepository
	Clock  clock.Clock
	Log    *zap.Logger
	Cfg    PricingSettingsConfig
}

// snapPack is an active pack as the snapshot holds it.
type snapPack struct {
	ID             uuid.UUID   `json:"id"`
	Code           string      `json:"code"`
	Kind           string      `json:"kind"`
	Name           domain.L10n `json:"name"`
	Tagline        domain.L10n `json:"tagline"`
	Label          domain.L10n `json:"label"`
	Credits        int64       `json:"credits"`
	PriceCents     int64       `json:"price_cents"`
	Currency       string      `json:"currency"`
	Featured       bool        `json:"featured"`
	Reference      bool        `json:"reference"`
	NewClientsOnly bool        `json:"new_clients_only"`
	SortOrder      int         `json:"sort_order"`
	Badges         []PackBadge `json:"badges"`
}

// pricingSnapshot is what Redis and the in-process copy hold: the active
// packs in canonical order with their badges, and the settings.
type pricingSnapshot struct {
	Settings PricingSettings `json:"settings"`
	Packs    []snapPack      `json:"packs"`
}

// PricingPack is a pack as the public site receives it.
type PricingPack struct {
	ID             uuid.UUID   `json:"id"`
	Code           string      `json:"code"`
	Kind           string      `json:"kind"`
	Name           string      `json:"name"`
	Tagline        string      `json:"tagline"`
	Label          string      `json:"label"`
	Credits        int64       `json:"credits"`
	PriceCents     int64       `json:"price_cents"`
	Currency       string      `json:"currency"`
	Featured       bool        `json:"featured"`
	NewClientsOnly bool        `json:"new_clients_only"`
	SortOrder      int         `json:"sort_order"`
	Badges         []PackBadge `json:"badges"`
}

// RateRange is the credits per minute advisors may charge.
type RateRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// PricingReference names the pack savings are measured against.
type PricingReference struct {
	PackID     uuid.UUID `json:"pack_id"`
	Credits    int64     `json:"credits"`
	PriceCents int64     `json:"price_cents"`
}

// FreeTrial is the live sign-up offer (§15.3).
type FreeTrial struct {
	Enabled bool `json:"enabled"`
	Minutes int  `json:"minutes"`
}

// PricingSummary is GET /pricing.
type PricingSummary struct {
	Currency    string            `json:"currency"`
	FreeTrial   FreeTrial         `json:"free_trial"`
	RateRange   RateRange         `json:"rate_range"`
	Reference   *PricingReference `json:"reference,omitempty"`
	Packs       []PricingPack     `json:"packs"`
	GeneratedAt time.Time         `json:"generated_at"`
}

// WelcomeOffer is the account's welcome-offer state in the top-up catalog.
type WelcomeOffer struct {
	Eligible bool       `json:"eligible"`
	PackID   *uuid.UUID `json:"pack_id,omitempty"`
}

// TopUpCatalog is GET /credits/catalog.
type TopUpCatalog struct {
	Packs        []PricingPack `json:"packs"`
	WelcomeOffer WelcomeOffer  `json:"welcome_offer"`
	RateRange    RateRange     `json:"rate_range"`
	Wallet       WalletView    `json:"wallet"`
	// StatementDescriptor is what the buyer's card statement shows
	// (revisions 5.1, §25.3; STARYIELD_PAYMENTS_STATEMENT_DESCRIPTOR). Empty
	// when none is configured (development).
	StatementDescriptor string `json:"statement_descriptor"`
}

// PricingResult is the rendered public summary with its validator.
type PricingResult struct {
	View PricingSummary
	// Body is the JSON the endpoint sends; ETag is a weak validator of its
	// content (generated_at excluded).
	Body []byte
	ETag string
}

type pricingViewMemo struct {
	snap   *pricingSnapshot
	trial  FreeTrial
	second int64
	result *PricingResult
}

type pricingPacksMemo struct {
	snap  *pricingSnapshot
	packs []PricingPack
}

// PricingService serves and administers the pricing catalog.
type PricingService struct {
	d PricingDeps

	mu       sync.RWMutex
	local    *pricingSnapshot
	localExp time.Time // real time; stale copies are kept for outages
	flight   singleflight.Group

	viewMu sync.Mutex
	views  map[string]pricingViewMemo
	packs  map[string]pricingPacksMemo
}

// NewPricingService wires the pricing service.
func NewPricingService(d PricingDeps) *PricingService {
	if d.Log == nil {
		d.Log = zap.NewNop()
	}
	if d.Cfg.Currency == "" {
		d.Cfg.Currency = "USD"
	}
	if d.Signup == nil {
		d.Signup = func(context.Context) (bool, int) { return false, 0 }
	}
	return &PricingService{d: d, views: map[string]pricingViewMemo{}, packs: map[string]pricingPacksMemo{}}
}

func (s *PricingService) record(ctx context.Context, action, targetType, targetID string, meta map[string]any) {
	if s.d.Audit == nil {
		return
	}
	recordAudit(ctx, s.d.Audit, s.d.Clock, s.d.Cfg.IPHashSecret, action, targetType, targetID, meta)
}

// Run listens for change announcements from other instances until ctx ends.
func (s *PricingService) Run(ctx context.Context) {
	if s.d.Bus == nil {
		return
	}
	s.d.Bus.Subscribe(ctx, PricingChangedChannel, func(string) { s.dropLocal() })
}

func (s *PricingService) dropLocal() {
	s.mu.Lock()
	s.localExp = time.Time{} // keep the copy as the outage fallback, but stale
	s.mu.Unlock()
	s.viewMu.Lock()
	s.views = map[string]pricingViewMemo{}
	s.packs = map[string]pricingPacksMemo{}
	s.viewMu.Unlock()
}

// changed invalidates every copy after a committed write: this instance's,
// Redis, and the other instances' (Pub/Sub).
func (s *PricingService) changed(ctx context.Context) {
	s.dropLocal()
	if s.d.Cache != nil {
		warnErr(ctx, "pricing: drop cached snapshot", s.d.Cache.Del(ctx, pricingCacheKey))
	}
	if s.d.Bus != nil {
		warnErr(ctx, "pricing: announce change", s.d.Bus.Publish(ctx, PricingChangedChannel, "changed"))
	}
}

// snapshot returns the current catalog: in-process copy, then Redis, then
// PostgreSQL. When PostgreSQL fails the last good copy (however old) is
// served; with none the error is a 503.
func (s *PricingService) snapshot(ctx context.Context) (*pricingSnapshot, error) {
	s.mu.RLock()
	snap, exp := s.local, s.localExp
	s.mu.RUnlock()
	if snap != nil && time.Now().Before(exp) {
		return snap, nil
	}
	v, err, _ := s.flight.Do("snapshot", func() (any, error) {
		// A detached context: the first caller's cancellation must not fail
		// the others sharing this load.
		lctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		return s.load(lctx)
	})
	if err != nil {
		return nil, apperror.Unavailable(err)
	}
	return v.(*pricingSnapshot), nil
}

func (s *PricingService) load(ctx context.Context) (*pricingSnapshot, error) {
	if s.d.Cache != nil {
		if raw, ok, err := s.d.Cache.Get(ctx, pricingCacheKey); err == nil && ok {
			var snap pricingSnapshot
			if json.Unmarshal([]byte(raw), &snap) == nil {
				s.setLocal(&snap, pricingLocalTTL)
				return &snap, nil
			}
		}
	}
	snap, err := s.fromDB(ctx)
	if err != nil {
		s.mu.RLock()
		last := s.local
		s.mu.RUnlock()
		if last == nil {
			return nil, err
		}
		warnErr(ctx, "pricing: load catalog (serving the last good copy)", err)
		s.setLocal(last, pricingRetryAfterErr) // retry the database soon, without hammering it
		return last, nil
	}
	if s.d.Cache != nil {
		if b, err := json.Marshal(snap); err == nil {
			warnErr(ctx, "pricing: cache snapshot", s.d.Cache.Set(ctx, pricingCacheKey, string(b), pricingRedisTTL))
		}
	}
	s.setLocal(snap, pricingLocalTTL)
	return snap, nil
}

func (s *PricingService) setLocal(snap *pricingSnapshot, ttl time.Duration) {
	s.mu.Lock()
	s.local, s.localExp = snap, time.Now().Add(ttl)
	s.mu.Unlock()
}

// settings reads the pricing document (defaults when it is missing).
func (s *PricingService) settings(ctx context.Context) (PricingSettings, error) {
	st, err := s.d.Site.Setting(ctx, domain.SiteSettingPricing)
	if err != nil {
		if err == repository.ErrNotFound {
			return defaultPricingSettings(s.d.Clock.Now()), nil
		}
		return PricingSettings{}, err
	}
	return pricingSettingsFrom(*st), nil
}

// fromDB builds the snapshot: three small statements (settings, packs and
// the purchase counts of the popularity window).
func (s *PricingService) fromDB(ctx context.Context) (*pricingSnapshot, error) {
	settings, err := s.settings(ctx)
	if err != nil {
		return nil, err
	}
	packs, err := s.d.Repo.Packs(ctx)
	if err != nil {
		return nil, err
	}
	since := s.d.Clock.Now().AddDate(0, 0, -settings.PopularWindowDays)
	counts, err := s.d.Repo.PurchaseCounts(ctx, since)
	if err != nil {
		return nil, err
	}
	return buildSnapshot(settings, packs, counts), nil
}

// buildSnapshot keeps the active packs and computes their badges.
func buildSnapshot(settings PricingSettings, packs []domain.CreditPack, counts []repository.PackCount) *pricingSnapshot {
	byPack := make(map[uuid.UUID]int64, len(counts))
	for _, c := range counts {
		byPack[c.PackID] = c.Purchases
	}
	var active []domain.CreditPack
	var bp []badgePack
	for i := range packs {
		if packs[i].Active {
			active = append(active, packs[i])
			bp = append(bp, badgePackOf(&packs[i]))
		}
	}
	badges := computeBadges(bp, byPack, settings.PopularMinPurchases)
	snap := &pricingSnapshot{Settings: settings, Packs: make([]snapPack, 0, len(active))}
	for i := range active {
		p := &active[i]
		b := badges[p.ID]
		if b == nil {
			b = []PackBadge{}
		}
		snap.Packs = append(snap.Packs, snapPack{ID: p.ID, Code: p.Code, Kind: p.Kind, Name: p.NameL10n, Tagline: p.TaglineL10n,
			Label: p.LabelL10n, Credits: p.Credits, PriceCents: p.PriceCents, Currency: p.Currency, Featured: p.Featured,
			Reference: p.Reference, NewClientsOnly: p.NewClientsOnly, SortOrder: p.SortOrder, Badges: b})
	}
	return snap
}

// RateRange is the advisor rate range (at most 10 s stale). It fails when
// the settings cannot be read and nothing is cached.
func (s *PricingService) RateRange(ctx context.Context) (min, max int, err error) {
	snap, err := s.snapshot(ctx)
	if err != nil {
		return 0, 0, err
	}
	return snap.Settings.RateMin, snap.Settings.RateMax, nil
}

// localizedPacks renders the snapshot's packs for a locale (memoized per
// snapshot and locale; the slice is shared and must not be modified).
func (s *PricingService) localizedPacks(snap *pricingSnapshot, locale string) []PricingPack {
	s.viewMu.Lock()
	memo, ok := s.packs[locale]
	s.viewMu.Unlock()
	if ok && memo.snap == snap {
		return memo.packs
	}
	packs := make([]PricingPack, 0, len(snap.Packs))
	for i := range snap.Packs {
		p := &snap.Packs[i]
		packs = append(packs, PricingPack{ID: p.ID, Code: p.Code, Kind: p.Kind, Name: p.Name.Pick(locale),
			Tagline: p.Tagline.Pick(locale), Label: p.Label.Pick(locale), Credits: p.Credits, PriceCents: p.PriceCents,
			Currency: p.Currency, Featured: p.Featured, NewClientsOnly: p.NewClientsOnly, SortOrder: p.SortOrder, Badges: p.Badges})
	}
	s.viewMu.Lock()
	s.packs[locale] = pricingPacksMemo{snap: snap, packs: packs}
	s.viewMu.Unlock()
	return packs
}

func (s *PricingService) freeTrial(ctx context.Context) FreeTrial {
	enabled, minutes := s.d.Signup(ctx)
	if !enabled {
		return FreeTrial{}
	}
	return FreeTrial{Enabled: true, Minutes: minutes}
}

// Summary builds the public pricing summary for a locale. Rendered output
// is reused within a second for the same snapshot and sign-up offer, so a
// burst of requests costs one render.
func (s *PricingService) Summary(ctx context.Context, locale string) (*PricingResult, error) {
	locale = i18n.Normalize(locale)
	snap, err := s.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	trial := s.freeTrial(ctx)
	now := s.d.Clock.Now()
	s.viewMu.Lock()
	memo, ok := s.views[locale]
	s.viewMu.Unlock()
	if ok && memo.snap == snap && memo.trial == trial && memo.second == now.Unix() {
		return memo.result, nil
	}
	res, err := renderPricing(snap, s.localizedPacks(snap, locale), trial, s.d.Cfg.Currency, now)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	s.viewMu.Lock()
	s.views[locale] = pricingViewMemo{snap: snap, trial: trial, second: now.Unix(), result: res}
	s.viewMu.Unlock()
	return res, nil
}

// renderPricing is the pure view builder.
func renderPricing(snap *pricingSnapshot, packs []PricingPack, trial FreeTrial, currency string, now time.Time) (*PricingResult, error) {
	v := PricingSummary{Currency: currency, FreeTrial: trial,
		RateRange: RateRange{Min: snap.Settings.RateMin, Max: snap.Settings.RateMax}, Packs: packs}
	for i := range snap.Packs {
		if p := &snap.Packs[i]; p.Reference && p.Kind == domain.PackStandard {
			v.Reference = &PricingReference{PackID: p.ID, Credits: p.Credits, PriceCents: p.PriceCents}
			break
		}
	}
	if v.Packs == nil {
		v.Packs = []PricingPack{}
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
	return &PricingResult{View: v, Body: body, ETag: `W/"` + hex.EncodeToString(sum[:8]) + `"`}, nil
}

// introPack returns the active intro pack of the snapshot.
func (snap *pricingSnapshot) introPack() *snapPack {
	for i := range snap.Packs {
		if snap.Packs[i].Kind == domain.PackIntro {
			return &snap.Packs[i]
		}
	}
	return nil
}

// OfferEligible applies the welcome-offer rules (§24.2) to an account's
// facts: an active intro pack exists, the account has no reserved or
// redeemed redemption and, when the pack is for new clients only, has never
// paid. The second result is the refusal reason ("" when eligible).
func OfferEligible(introActive, newClientsOnly, hasRedemption, hasPaid bool) (bool, string) {
	switch {
	case !introActive:
		return false, apperror.OfferReasonNoOffer
	case hasRedemption:
		return false, apperror.OfferReasonAlreadyRedeemed
	case newClientsOnly && hasPaid:
		return false, apperror.OfferReasonNotNewClient
	}
	return true, ""
}

// Catalog builds the top-up catalog of a client in one pass: the packs come
// from the cached snapshot and everything about the account (wallet, offer
// eligibility) from a single statement.
func (s *PricingService) Catalog(ctx context.Context, userID uuid.UUID, locale string) (*TopUpCatalog, error) {
	locale = i18n.Normalize(locale)
	snap, err := s.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	st, err := s.d.Repo.CatalogState(ctx, userID)
	if err != nil {
		return nil, mapRepoErr(err, "Wallet")
	}
	all := s.localizedPacks(snap, locale)
	intro := snap.introPack()
	eligible, _ := OfferEligible(intro != nil, intro != nil && intro.NewClientsOnly, st.HasRedemption, st.HasPaid)
	out := &TopUpCatalog{Packs: make([]PricingPack, 0, len(all)), RateRange: RateRange{Min: snap.Settings.RateMin, Max: snap.Settings.RateMax},
		Wallet: WalletView{Balance: st.Balance, Held: st.Held, FreeMinutes: st.FreeMinutes}, StatementDescriptor: s.d.Cfg.StatementDescriptor}
	for i := range all {
		if all[i].Kind == domain.PackIntro && !eligible {
			continue // a redeemed or unavailable offer is absent, never disabled
		}
		if all[i].Kind == domain.PackIntro {
			id := all[i].ID
			out.WelcomeOffer.PackID = &id
		}
		out.Packs = append(out.Packs, all[i])
	}
	out.WelcomeOffer.Eligible = eligible
	// The eligible intro pack comes first (the canonical order already puts
	// it first unless an admin sorted it elsewhere).
	for i := range out.Packs {
		if out.Packs[i].Kind == domain.PackIntro && i > 0 {
			intro := out.Packs[i]
			copy(out.Packs[1:i+1], out.Packs[0:i])
			out.Packs[0] = intro
			break
		}
	}
	return out, nil
}
