package service

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// memSiteRepo is an in-memory SiteRepository. fail makes every call fail
// (database outage); loads counts reads of the whole configuration.
type memSiteRepo struct {
	mu       sync.Mutex
	settings map[string]domain.SiteSetting
	banners  []domain.SiteBanner
	names    map[uuid.UUID]string
	fail     bool
	loads    int
	totals   repository.SponsoredTotals
	earnings int64
	statsN   int
	// off-session oversight totals
	offMessages, offSenders, offHits int64
}

var errDown = errors.New("database is down")

func newMemSiteRepo() *memSiteRepo {
	now := time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)
	return &memSiteRepo{names: map[uuid.UUID]string{}, settings: map[string]domain.SiteSetting{
		domain.SiteSettingBanners:     {Key: "banners", Value: domain.JSONMap{"enabled": true, "rotation_seconds": float64(6)}, Version: 1, UpdatedAt: now},
		domain.SiteSettingSignupOffer: {Key: "signup_offer", Value: domain.JSONMap{"enabled": true, "free_minutes": float64(5)}, Version: 1, UpdatedAt: now},
		domain.SiteSettingFreeMode:    {Key: "free_mode", Value: domain.JSONMap{"enabled": false, "reason": ""}, Version: 1, UpdatedAt: now},
		domain.SiteSettingOffSession:  {Key: "off_session_messages", Value: domain.JSONMap{"enabled": true, "client_daily_limit": float64(20), "advisor_daily_limit": float64(20)}, Version: 1, UpdatedAt: now},
	}}
}

// settingsWith builds a document value (tests that store malformed values).
func (r *memSiteRepo) settingsWith(v map[string]any) domain.SiteSetting {
	return domain.SiteSetting{Key: "off_session_messages", Value: domain.JSONMap(v), Version: 1}
}

func (r *memSiteRepo) check() error {
	if r.fail {
		return errDown
	}
	return nil
}

func (r *memSiteRepo) Setting(_ context.Context, key string) (*domain.SiteSetting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.check(); err != nil {
		return nil, err
	}
	s, ok := r.settings[key]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &s, nil
}

func (r *memSiteRepo) Settings(_ context.Context) ([]domain.SiteSetting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.check(); err != nil {
		return nil, err
	}
	r.loads++
	var out []domain.SiteSetting
	for _, s := range r.settings {
		out = append(out, s)
	}
	return out, nil
}

func (r *memSiteRepo) SettingForUpdate(ctx context.Context, key string) (*domain.SiteSetting, error) {
	return r.Setting(ctx, key)
}

func (r *memSiteRepo) SaveSetting(_ context.Context, key string, value domain.JSONMap, version int, by *uuid.UUID, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.check(); err != nil {
		return err
	}
	// Round-trip through JSON like the jsonb column does.
	b, _ := (value).Value()
	var back domain.JSONMap
	_ = back.Scan(b)
	r.settings[key] = domain.SiteSetting{Key: key, Value: back, Version: version, UpdatedBy: by, UpdatedAt: at}
	return nil
}

func (r *memSiteRepo) sorted() []domain.SiteBanner {
	out := slices.Clone(r.banners)
	slices.SortFunc(out, func(a, b domain.SiteBanner) int {
		if a.SortOrder != b.SortOrder {
			return a.SortOrder - b.SortOrder
		}
		return strings.Compare(a.ID.String(), b.ID.String())
	})
	return out
}

func (r *memSiteRepo) Banners(_ context.Context) ([]domain.SiteBanner, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.check(); err != nil {
		return nil, err
	}
	return r.sorted(), nil
}

func (r *memSiteRepo) BannerForUpdate(_ context.Context, id uuid.UUID) (*domain.SiteBanner, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.check(); err != nil {
		return nil, err
	}
	for _, b := range r.banners {
		if b.ID == id {
			return &b, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *memSiteRepo) CreateBanner(_ context.Context, b *domain.SiteBanner) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.check(); err != nil {
		return err
	}
	r.banners = append(r.banners, *b)
	return nil
}

func (r *memSiteRepo) SaveBanner(_ context.Context, b *domain.SiteBanner) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.banners {
		if r.banners[i].ID == b.ID {
			r.banners[i] = *b
			return nil
		}
	}
	return repository.ErrNotFound
}

func (r *memSiteRepo) DeleteBanner(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.banners {
		if r.banners[i].ID == id {
			r.banners = slices.Delete(r.banners, i, i+1)
			return nil
		}
	}
	return repository.ErrNotFound
}

func (r *memSiteRepo) SetBannerOrder(_ context.Context, ids []uuid.UUID, _ *uuid.UUID, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for pos, id := range ids {
		for i := range r.banners {
			if r.banners[i].ID == id {
				r.banners[i].SortOrder = pos
			}
		}
	}
	return nil
}

func (r *memSiteRepo) UserNames(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		if n, ok := r.names[id]; ok {
			out[id] = n
		}
	}
	return out, nil
}

func (r *memSiteRepo) SponsoredTotals(context.Context, time.Time) (repository.SponsoredTotals, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.statsN++
	return r.totals, nil
}

func (r *memSiteRepo) SponsoredEarnings(context.Context, time.Time) (int64, error) {
	return r.earnings, nil
}

func (r *memSiteRepo) OffSessionTotals(context.Context, time.Time) (int64, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.offMessages, r.offSenders, nil
}

func (r *memSiteRepo) OffSessionLimitHits(context.Context, time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.offHits, nil
}

// siteAudit records audit entries.
type siteAudit struct {
	mu   sync.Mutex
	rows []domain.AuditLog
}

func (a *siteAudit) Create(_ context.Context, l *domain.AuditLog) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rows = append(a.rows, *l)
	return nil
}

func (a *siteAudit) List(context.Context, repository.AuditFilter) ([]domain.AuditLog, error) {
	return nil, nil
}

func (a *siteAudit) actions() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []string
	for _, r := range a.rows {
		out = append(out, r.Action)
	}
	return out
}

type siteFixture struct {
	svc   *SiteService
	repo  *memSiteRepo
	audit *siteAudit
	clk   *clock.Fake
	kv    *cache.Memory
	bus   *cache.MemoryBus
	admin uuid.UUID
	ctx   context.Context
}

func newSiteFixture(t *testing.T, hosts ...string) *siteFixture {
	t.Helper()
	clk := clock.NewFake(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	f := &siteFixture{repo: newMemSiteRepo(), audit: &siteAudit{}, clk: clk, kv: cache.NewMemory(nil), bus: cache.NewMemoryBus(), admin: uuid.New()}
	f.repo.names[f.admin] = "Ada Admin"
	f.ctx = reqctx.WithPrincipal(context.Background(), reqctx.Principal{UserID: f.admin, Role: domain.RoleAdmin})
	f.svc = f.instance(hosts...)
	return f
}

// instance builds another service sharing the database, Redis and bus (a
// second API instance).
func (f *siteFixture) instance(hosts ...string) *SiteService {
	return NewSiteService(SiteDeps{Tx: inlineTx{}, Repo: f.repo, Cache: f.kv, Bus: f.bus, Audit: f.audit, Clock: f.clk,
		Cfg: SiteSettings{AllowedHosts: hosts, SignupFallbackMinutes: 4}})
}

func seedBanner(f *siteFixture, order int, text string) domain.SiteBanner {
	b := domain.SiteBanner{ID: uuid.New(), Text: domain.L10n{"en": text}, CTALabel: domain.L10n{}, ActionType: "none", Audience: "all",
		Active: true, SortOrder: order, Version: 1}
	f.repo.banners = append(f.repo.banners, b)
	return b
}

func validBanner(text string) BannerInput {
	return BannerInput{Text: map[string]string{"en": text}, Action: &BannerActionInput{Type: "none"}}
}

func siteFields(t *testing.T, err error) map[string]string {
	t.Helper()
	e, ok := apperror.As(err)
	if !ok || e.Code != apperror.CodeValidation {
		t.Fatalf("want a validation error, got %v", err)
	}
	raw, _ := e.Details["fields"].(map[string]any)
	fields := map[string]string{}
	for k, v := range raw {
		fields[k], _ = v.(string)
	}
	return fields
}

func TestSiteConfigDefaultsAndSeedShape(t *testing.T) {
	f := newSiteFixture(t)
	seedBanner(f, 0, "{free_minutes} MINUTES FREE")
	f.repo.banners[0].CTALabel = domain.L10n{"en": "CLAIM →", "fr": "PROFITER →"}
	f.repo.banners[0].Text["fr"] = "{free_minutes} MINUTES OFFERTES"
	f.repo.banners[0].ActionType, f.repo.banners[0].ActionHref, f.repo.banners[0].Audience = "link", "/signup", "anonymous"

	res, err := f.svc.Config(f.ctx, "en")
	if err != nil {
		t.Fatal(err)
	}
	v := res.View
	if v.SignupOffer.FreeMinutes != 5 || !v.SignupOffer.Enabled || v.FreeMode.Active || v.Banners.RotationSeconds != 6 {
		t.Fatalf("config: %+v", v)
	}
	if len(v.Banners.Items) != 1 || v.Banners.Items[0].Text != "5 MINUTES FREE" || v.Banners.Items[0].CTALabel != "CLAIM →" ||
		v.Banners.Items[0].Action != (BannerAction{Type: "link", Href: "/signup"}) || v.Banners.Items[0].Audience != "anonymous" {
		t.Fatalf("banner: %+v", v.Banners.Items)
	}
	if !strings.HasPrefix(res.ETag, `W/"`) || v.GeneratedAt.IsZero() {
		t.Fatalf("etag %q generated %v", res.ETag, v.GeneratedAt)
	}
	// Locale: fr copy when present; es falls back to English; unknown locales too.
	fr, _ := f.svc.Config(f.ctx, "fr-CA")
	if fr.View.Banners.Items[0].Text != "5 MINUTES OFFERTES" || fr.View.Banners.Items[0].CTALabel != "PROFITER →" {
		t.Fatalf("fr: %+v", fr.View.Banners.Items)
	}
	for _, loc := range []string{"es", "de", ""} {
		got, _ := f.svc.Config(f.ctx, loc)
		if got.View.Banners.Items[0].Text != "5 MINUTES FREE" {
			t.Fatalf("%q fallback: %+v", loc, got.View.Banners.Items)
		}
	}
	// The ETag follows the content, not the clock, and differs per language.
	f.clk.Advance(5 * time.Second)
	again, _ := f.svc.Config(f.ctx, "en")
	if again.ETag != res.ETag || again.View.GeneratedAt.Equal(v.GeneratedAt) {
		t.Fatalf("etag changed with the clock: %q vs %q", again.ETag, res.ETag)
	}
	if fr.ETag == res.ETag {
		t.Fatal("languages share an ETag")
	}
}

func TestSiteConfigHidesBannersAndPlaceholderBannersWithoutOffer(t *testing.T) {
	f := newSiteFixture(t)
	seedBanner(f, 0, "{free_minutes} MINUTES FREE")
	seedBanner(f, 1, "Free horoscopes every day")

	// Bar off: no items at all.
	if _, err := f.svc.UpdateBannerSettings(f.ctx, BannerSettingsPatch{Enabled: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	if res, _ := f.svc.Config(f.ctx, "en"); res.View.Banners.Items == nil || len(res.View.Banners.Items) != 0 || !strings.Contains(string(res.Body), `"items":[]`) {
		t.Fatalf("hidden bar: %s", res.Body)
	}
	if _, err := f.svc.UpdateBannerSettings(f.ctx, BannerSettingsPatch{Enabled: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	// Offer off: free_minutes is 0 and banners promising minutes are not shown.
	if _, err := f.svc.UpdateSignupOffer(f.ctx, SignupOfferInput{Enabled: ptr(false), FreeMinutes: ptr(7)}); err != nil {
		t.Fatal(err)
	}
	res, _ := f.svc.Config(f.ctx, "en")
	if res.View.SignupOffer.Enabled || res.View.SignupOffer.FreeMinutes != 0 || len(res.View.Banners.Items) != 1 ||
		res.View.Banners.Items[0].Text != "Free horoscopes every day" {
		t.Fatalf("offer off: %+v", res.View)
	}
	// The stored minutes are kept for when it is switched back on.
	if o, _ := f.svc.SignupOffer(f.ctx); o.FreeMinutes != 7 || o.Enabled {
		t.Fatalf("stored offer: %+v", o)
	}
}

func TestSiteConfigScheduleAndFreeModeExpiryAreExact(t *testing.T) {
	f := newSiteFixture(t)
	now := f.clk.Now()
	future, past := seedBanner(f, 0, "Later"), seedBanner(f, 1, "Gone")
	live := seedBanner(f, 2, "Now")
	inactive := seedBanner(f, 3, "Off")
	for i := range f.repo.banners {
		switch f.repo.banners[i].ID {
		case future.ID:
			f.repo.banners[i].StartsAt = ptr(now.Add(time.Minute))
		case past.ID:
			f.repo.banners[i].EndsAt = ptr(now.Add(-time.Minute))
		case inactive.ID:
			f.repo.banners[i].Active = false
		}
	}
	res, _ := f.svc.Config(f.ctx, "en")
	if len(res.View.Banners.Items) != 1 || res.View.Banners.Items[0].ID != live.ID.String() {
		t.Fatalf("only the live banner: %+v", res.View.Banners.Items)
	}
	// The snapshot is cached for 10 s, yet a schedule is applied to the
	// request time: after the start time the scheduled banner appears.
	f.clk.Advance(2 * time.Minute)
	res, _ = f.svc.Config(f.ctx, "en")
	if len(res.View.Banners.Items) != 2 || res.View.Banners.Items[0].Text != "Later" {
		t.Fatalf("after the start: %+v", res.View.Banners.Items)
	}
	// Free mode expires on its own, at once.
	ends := f.clk.Now().Add(time.Hour)
	if _, err := f.svc.UpdateFreeMode(f.ctx, FreeModeInput{Enabled: ptr(true), Confirmation: "ENABLE FREE MODE", Reason: ptr("Launch week"), EndsAt: &ends}); err != nil {
		t.Fatal(err)
	}
	res, _ = f.svc.Config(f.ctx, "en")
	if !res.View.FreeMode.Active || res.View.FreeMode.EndsAt == nil || !f.svc.FreeModeActive(f.ctx) {
		t.Fatalf("free mode: %+v", res.View.FreeMode)
	}
	f.clk.Advance(time.Hour)
	res, _ = f.svc.Config(f.ctx, "en")
	if res.View.FreeMode.Active || res.View.FreeMode.EndsAt != nil || f.svc.FreeModeActive(f.ctx) {
		t.Fatalf("free mode after its end: %+v", res.View.FreeMode)
	}
	st, _ := f.svc.FreeMode(f.ctx)
	if !st.Enabled || st.Active {
		t.Fatalf("state after the end: enabled=%v active=%v", st.Enabled, st.Active)
	}
}

func TestSiteCacheServesAndInvalidatesAcrossInstances(t *testing.T) {
	f := newSiteFixture(t)
	seedBanner(f, 0, "First")
	other := f.instance()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go other.Run(ctx)
	go f.svc.Run(ctx)
	time.Sleep(50 * time.Millisecond) // let the subscriptions start

	// Reads are cached: one database load serves many requests and both instances.
	for i := 0; i < 20; i++ {
		if _, err := f.svc.Config(f.ctx, "en"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := other.Config(f.ctx, "en"); err != nil {
		t.Fatal(err)
	}
	if f.repo.loads != 1 {
		t.Fatalf("expected 1 database load (Redis feeds the second instance), got %d", f.repo.loads)
	}
	if got, _ := other.Config(f.ctx, "en"); len(got.View.Banners.Items) != 1 {
		t.Fatal("seed banner missing")
	}
	// A write on one instance is visible on the other without waiting for
	// any TTL: Redis is dropped and the announcement clears its copy.
	if _, err := f.svc.CreateBanner(f.ctx, validBanner("Second")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		got, _ := other.Config(f.ctx, "en")
		if len(got.View.Banners.Items) == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the other instance still serves the old copy: %+v", got.View.Banners.Items)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got, _ := f.svc.Config(f.ctx, "en"); len(got.View.Banners.Items) != 2 {
		t.Fatal("the writing instance serves a stale copy")
	}
}

func TestSiteServesLastGoodCopyThenDefaultsDuringOutage(t *testing.T) {
	f := newSiteFixture(t)
	seedBanner(f, 0, "Still here")
	if res, _ := f.svc.Config(f.ctx, "en"); len(res.View.Banners.Items) != 1 {
		t.Fatal("warm-up")
	}
	// Outage with Redis emptied and the in-process copy expired: the last
	// good copy keeps being served (never an error, never a blank site).
	f.repo.fail = true
	_ = f.kv.Del(f.ctx, siteCacheKey)
	f.svc.dropLocal()
	for i := 0; i < 3; i++ {
		res, err := f.svc.Config(f.ctx, "en")
		if err != nil || len(res.View.Banners.Items) != 1 || res.View.SignupOffer.FreeMinutes != 5 {
			t.Fatalf("outage with a good copy: %v %+v", err, res)
		}
	}
	if f.svc.SignupFreeMinutes(f.ctx) != 5 {
		t.Fatal("signup minutes during the outage")
	}

	// A fresh instance that never saw the database serves safe defaults.
	fresh := newSiteFixture(t)
	fresh.repo.fail = true
	res, err := fresh.svc.Config(fresh.ctx, "en")
	if err != nil {
		t.Fatal(err)
	}
	v := res.View
	if len(v.Banners.Items) != 0 || v.FreeMode.Active || !v.SignupOffer.Enabled || v.SignupOffer.FreeMinutes != 4 {
		t.Fatalf("defaults: %+v", v)
	}
	if fresh.svc.SignupFreeMinutes(fresh.ctx) != 4 || fresh.svc.FreeModeActive(fresh.ctx) {
		t.Fatal("fallback signup minutes / free mode must be the configured fallback and off")
	}
	// Recovery: once the database is back the real configuration returns.
	fresh.repo.fail = false
	seedBanner(fresh, 0, "Back")
	fresh.svc.dropLocal()
	_ = fresh.kv.Del(fresh.ctx, siteCacheKey)
	time.Sleep(10 * time.Millisecond)
	fresh.svc.mu.Lock()
	fresh.svc.localExp = time.Time{}
	fresh.svc.mu.Unlock()
	if res, _ := fresh.svc.Config(fresh.ctx, "en"); len(res.View.Banners.Items) != 1 || res.View.SignupOffer.FreeMinutes != 5 {
		t.Fatalf("recovery: %+v", res.View)
	}
}

func TestSignupFreeMinutesFollowsTheOffer(t *testing.T) {
	f := newSiteFixture(t)
	if got := f.svc.SignupFreeMinutes(f.ctx); got != 5 {
		t.Fatalf("seed: %d", got)
	}
	if _, err := f.svc.UpdateSignupOffer(f.ctx, SignupOfferInput{Enabled: ptr(true), FreeMinutes: ptr(7)}); err != nil {
		t.Fatal(err)
	}
	if got := f.svc.SignupFreeMinutes(f.ctx); got != 7 {
		t.Fatalf("after change: %d", got)
	}
	if _, err := f.svc.UpdateSignupOffer(f.ctx, SignupOfferInput{Enabled: ptr(false), FreeMinutes: ptr(7)}); err != nil {
		t.Fatal(err)
	}
	if got := f.svc.SignupFreeMinutes(f.ctx); got != 0 {
		t.Fatalf("disabled: %d", got)
	}
	if _, err := f.svc.UpdateSignupOffer(f.ctx, SignupOfferInput{Enabled: ptr(true), FreeMinutes: ptr(7)}); err != nil {
		t.Fatal(err)
	}
	if got := f.svc.SignupFreeMinutes(f.ctx); got != 7 {
		t.Fatalf("re-enabled keeps the minutes: %d", got)
	}
}

func TestSignupOfferValidationVersionAndAudit(t *testing.T) {
	f := newSiteFixture(t)
	for _, in := range []SignupOfferInput{{}, {Enabled: ptr(true)}, {FreeMinutes: ptr(5)}, {Enabled: ptr(true), FreeMinutes: ptr(0)},
		{Enabled: ptr(true), FreeMinutes: ptr(61)}, {Enabled: ptr(true), FreeMinutes: ptr(-1)}, {Enabled: ptr(true), FreeMinutes: ptr(5), ExpectedVersion: ptr(0)}} {
		if _, err := f.svc.UpdateSignupOffer(f.ctx, in); !isCode(err, apperror.CodeValidation) {
			t.Errorf("%+v accepted: %v", in, err)
		}
	}
	if fields := siteFields(t, func() error {
		_, err := f.svc.UpdateSignupOffer(f.ctx, SignupOfferInput{Enabled: ptr(true), FreeMinutes: ptr(99)})
		return err
	}()); fields["free_minutes"] != msgSignupMinutes {
		t.Fatalf("fields: %v", fields)
	}
	for _, minutes := range []int{1, 60} {
		if _, err := f.svc.UpdateSignupOffer(f.ctx, SignupOfferInput{Enabled: ptr(true), FreeMinutes: ptr(minutes)}); err != nil {
			t.Fatalf("%d minutes refused: %v", minutes, err)
		}
	}
	// Optimistic concurrency.
	cur, _ := f.svc.SignupOffer(f.ctx)
	if _, err := f.svc.UpdateSignupOffer(f.ctx, SignupOfferInput{Enabled: ptr(true), FreeMinutes: ptr(9), ExpectedVersion: ptr(cur.Version - 1)}); !isCode(err, apperror.CodeVersionConflict) {
		t.Fatalf("stale version: %v", err)
	} else if e, _ := apperror.As(err); e.Details["current_version"] != cur.Version {
		t.Fatalf("details: %v", e.Details)
	}
	out, err := f.svc.UpdateSignupOffer(f.ctx, SignupOfferInput{Enabled: ptr(true), FreeMinutes: ptr(9), ExpectedVersion: ptr(cur.Version)})
	if err != nil || out.Version != cur.Version+1 || out.FreeMinutes != 9 || out.UpdatedBy == nil || out.UpdatedBy.Name != "Ada Admin" {
		t.Fatalf("saved: %+v %v", out, err)
	}
	// An unchanged save neither bumps the version nor writes an audit row.
	n := len(f.audit.actions())
	same, _ := f.svc.UpdateSignupOffer(f.ctx, SignupOfferInput{Enabled: ptr(true), FreeMinutes: ptr(9)})
	if same.Version != out.Version || len(f.audit.actions()) != n {
		t.Fatal("no-op save changed state")
	}
	last := f.audit.rows[len(f.audit.rows)-1]
	if last.Action != "site.signup_offer_updated" || last.ActorID == nil || *last.ActorID != f.admin {
		t.Fatalf("audit: %+v", last)
	}
	if before, _ := last.Metadata["before"].(map[string]any); before["free_minutes"] != 60 {
		t.Fatalf("audit before: %v", last.Metadata)
	}
}

func TestCreateBannerValidationFieldKeys(t *testing.T) {
	f := newSiteFixture(t, "partner.example.com")
	in := BannerInput{
		Text:     map[string]string{"en": "<script>alert(1)</script>", "fr": strings.Repeat("x", 141), "es": "{nope}", "de": "Hallo"},
		CTALabel: map[string]string{"en": strings.Repeat("c", 41)},
		Action:   &BannerActionInput{Type: "link", Href: "javascript:alert(1)"},
		Audience: ptr("everyone"),
		StartsAt: ptr(f.clk.Now().Add(time.Hour)), EndsAt: ptr(f.clk.Now()),
	}
	_, err := f.svc.CreateBanner(f.ctx, in)
	fields := siteFields(t, err)
	for key, want := range map[string]string{"text.en": msgBannerHTML, "text.fr": msgBannerTextLength, "text.es": msgBannerPlaceholder,
		"text.de": msgBannerLocale, "cta_label.en": msgBannerCTALength, "action.href": msgBannerHref, "audience": msgBannerAudience,
		"ends_at": msgBannerSchedule} {
		if fields[key] != want {
			t.Errorf("fields[%q] = %q, want %q (all: %v)", key, fields[key], want, fields)
		}
	}
	for _, c := range []struct {
		name string
		in   BannerInput
		key  string
	}{
		{"no text", BannerInput{Action: &BannerActionInput{Type: "none"}}, "text.en"},
		{"no action", BannerInput{Text: map[string]string{"en": "Hi"}}, "action.type"},
		{"bad action", BannerInput{Text: map[string]string{"en": "Hi"}, Action: &BannerActionInput{Type: "script"}}, "action.type"},
		{"link without href", BannerInput{Text: map[string]string{"en": "Hi"}, Action: &BannerActionInput{Type: "link"}}, "action.href"},
		{"foreign host", BannerInput{Text: map[string]string{"en": "Hi"}, Action: &BannerActionInput{Type: "link", Href: "https://evil.example.com/"}}, "action.href"},
	} {
		if _, err := f.svc.CreateBanner(f.ctx, c.in); siteFields(t, err)[c.key] == "" {
			t.Errorf("%s: no %s field in %v", c.name, c.key, err)
		}
	}
	if len(f.repo.banners) != 0 || len(f.audit.actions()) != 0 {
		t.Fatal("a rejected banner left traces")
	}
	// A valid banner: none action drops any href; defaults are applied.
	out, err := f.svc.CreateBanner(f.ctx, BannerInput{Text: map[string]string{"en": "  Hello  "}, Action: &BannerActionInput{Type: "none", Href: "https://ignored"}})
	if err != nil || out.Action.Href != "" || out.Audience != "all" || !out.Active || out.Status != "live" || out.Version != 1 ||
		out.Text["en"] != "Hello" || out.UpdatedBy == nil || out.UpdatedBy.ID != f.admin {
		t.Fatalf("created: %+v %v", out, err)
	}
	ok, err := f.svc.CreateBanner(f.ctx, BannerInput{Text: map[string]string{"en": "Partner"}, Action: &BannerActionInput{Type: "link", Href: "https://partner.example.com/x"}})
	if err != nil || ok.Action.Href != "https://partner.example.com/x" || ok.SortOrder != 1 {
		t.Fatalf("partner link: %+v %v", ok, err)
	}
	if got := f.audit.actions(); !slices.Equal(got, []string{"site_banner.created", "site_banner.created"}) {
		t.Fatalf("audit: %v", got)
	}
}

func TestBannerLimitIsTen(t *testing.T) {
	f := newSiteFixture(t)
	for i := 0; i < domain.MaxSiteBanners; i++ {
		if _, err := f.svc.CreateBanner(f.ctx, validBanner("Banner")); err != nil {
			t.Fatalf("banner %d: %v", i+1, err)
		}
	}
	_, err := f.svc.CreateBanner(f.ctx, validBanner("One too many"))
	if !isCode(err, apperror.CodeLimitReached) {
		t.Fatalf("11th banner: %v", err)
	}
	// Deleting one frees a slot.
	if err := f.svc.DeleteBanner(f.ctx, f.repo.banners[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CreateBanner(f.ctx, validBanner("Fits now")); err != nil {
		t.Fatal(err)
	}
	// Public output is capped at ten as well.
	if res, _ := f.svc.Config(f.ctx, "en"); len(res.View.Banners.Items) != domain.MaxSiteBanners {
		t.Fatalf("public items: %d", len(res.View.Banners.Items))
	}
}

func TestUpdateBannerPartialVersionAndSchedule(t *testing.T) {
	f := newSiteFixture(t, "partner.example.com")
	created, err := f.svc.CreateBanner(f.ctx, BannerInput{Text: map[string]string{"en": "Hello", "fr": "Bonjour"}, Action: &BannerActionInput{Type: "none"}})
	if err != nil {
		t.Fatal(err)
	}
	// Only the sent fields change; text replaces the whole object.
	out, err := f.svc.UpdateBanner(f.ctx, created.ID, BannerPatch{Text: map[string]string{"en": "Hi again"}, Audience: ptr("signed_in")})
	if err != nil || out.Version != 2 || out.Text["en"] != "Hi again" || out.Text["fr"] != "" || out.Audience != "signed_in" || !out.Active {
		t.Fatalf("patched: %+v %v", out, err)
	}
	// Stale versions conflict and report the current one.
	_, err = f.svc.UpdateBanner(f.ctx, created.ID, BannerPatch{ExpectedVersion: ptr(1), Active: ptr(false)})
	e, _ := apperror.As(err)
	if e == nil || e.Code != apperror.CodeVersionConflict || e.Details["current_version"] != 2 {
		t.Fatalf("conflict: %v", err)
	}
	// A no-op leaves the version and the audit trail alone.
	n := len(f.audit.actions())
	same, err := f.svc.UpdateBanner(f.ctx, created.ID, BannerPatch{ExpectedVersion: ptr(2), Audience: ptr("signed_in")})
	if err != nil || same.Version != 2 || len(f.audit.actions()) != n {
		t.Fatalf("no-op: %+v %v", same, err)
	}
	// Schedule: set, validated against the stored other bound, cleared with null.
	start := f.clk.Now().Add(time.Hour)
	out, err = f.svc.UpdateBanner(f.ctx, created.ID, BannerPatch{StartsAt: OptTime{Set: true, Value: &start}})
	if err != nil || out.Status != "scheduled" || out.StartsAt == nil {
		t.Fatalf("scheduled: %+v %v", out, err)
	}
	before := start.Add(-time.Minute)
	if _, err = f.svc.UpdateBanner(f.ctx, created.ID, BannerPatch{EndsAt: OptTime{Set: true, Value: &before}}); siteFields(t, err)["ends_at"] != msgBannerSchedule {
		t.Fatalf("end before start: %v", err)
	}
	out, _ = f.svc.UpdateBanner(f.ctx, created.ID, BannerPatch{StartsAt: OptTime{Set: true}})
	if out.StartsAt != nil || out.Status != "live" {
		t.Fatalf("cleared: %+v", out)
	}
	past := f.clk.Now().Add(-time.Hour)
	out, _ = f.svc.UpdateBanner(f.ctx, created.ID, BannerPatch{EndsAt: OptTime{Set: true, Value: &past}})
	if out.Status != "expired" {
		t.Fatalf("expired: %+v", out)
	}
	out, _ = f.svc.UpdateBanner(f.ctx, created.ID, BannerPatch{Active: ptr(false)})
	if out.Status != "inactive" {
		t.Fatalf("inactive: %+v", out)
	}
	// Validation and unknown ids.
	if _, err := f.svc.UpdateBanner(f.ctx, created.ID, BannerPatch{Action: &BannerActionInput{Type: "link", Href: "//evil.com"}}); siteFields(t, err)["action.href"] == "" {
		t.Fatal("protocol-relative href accepted")
	}
	if _, err := f.svc.UpdateBanner(f.ctx, created.ID, BannerPatch{Text: map[string]string{"fr": "sans anglais"}}); siteFields(t, err)["text.en"] == "" {
		t.Fatal("text without English accepted")
	}
	if _, err := f.svc.UpdateBanner(f.ctx, uuid.New(), BannerPatch{Active: ptr(true)}); !isCode(err, apperror.CodeNotFound) {
		t.Fatalf("unknown id: %v", err)
	}
	if err := f.svc.DeleteBanner(f.ctx, uuid.New()); !isCode(err, apperror.CodeNotFound) {
		t.Fatalf("delete unknown id: %v", err)
	}
	if got := f.audit.actions(); !slices.Contains(got, "site_banner.updated") || slices.Contains(got, "site_banner.deleted") {
		t.Fatalf("audit: %v", got)
	}
}

func TestReorderBannersValidation(t *testing.T) {
	f := newSiteFixture(t)
	var ids []uuid.UUID
	for _, name := range []string{"A", "B", "C"} {
		b, err := f.svc.CreateBanner(f.ctx, validBanner(name))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, b.ID)
	}
	rev := []uuid.UUID{ids[2], ids[1], ids[0]}
	items, err := f.svc.ReorderBanners(f.ctx, rev)
	if err != nil || len(items) != 3 || items[0].ID != ids[2] || items[2].ID != ids[0] || items[0].SortOrder != 0 {
		t.Fatalf("reordered: %+v %v", items, err)
	}
	if res, _ := f.svc.Config(f.ctx, "en"); res.View.Banners.Items[0].Text != "C" {
		t.Fatalf("public order: %+v", res.View.Banners.Items)
	}
	for name, bad := range map[string][]uuid.UUID{
		"missing":   {ids[0], ids[1]},
		"duplicate": {ids[0], ids[0], ids[1]},
		"extra":     {ids[0], ids[1], ids[2], uuid.New()},
		"unknown":   {ids[0], ids[1], uuid.New()},
		"empty":     {},
	} {
		if _, err := f.svc.ReorderBanners(f.ctx, bad); siteFields(t, err)["ids"] != msgBannerOrder {
			t.Errorf("%s accepted: %v", name, err)
		}
	}
	if got := f.audit.actions(); got[len(got)-1] != "site_banner.reordered" || len(got) != 4 {
		t.Fatalf("audit: %v", got)
	}
}

func TestBannerSettingsValidationAndVersion(t *testing.T) {
	f := newSiteFixture(t)
	for _, n := range []int{0, 3, 31, -5} {
		if _, err := f.svc.UpdateBannerSettings(f.ctx, BannerSettingsPatch{RotationSeconds: ptr(n)}); siteFields(t, err)["rotation_seconds"] != msgBannerRotation {
			t.Errorf("%d seconds accepted", n)
		}
	}
	for _, n := range []int{4, 30} {
		if _, err := f.svc.UpdateBannerSettings(f.ctx, BannerSettingsPatch{RotationSeconds: ptr(n)}); err != nil {
			t.Errorf("%d seconds refused: %v", n, err)
		}
	}
	cur, _ := f.svc.Banners(f.ctx)
	if _, err := f.svc.UpdateBannerSettings(f.ctx, BannerSettingsPatch{Enabled: ptr(false), ExpectedVersion: ptr(cur.Settings.Version + 5)}); !isCode(err, apperror.CodeVersionConflict) {
		t.Fatalf("stale version: %v", err)
	}
	out, err := f.svc.UpdateBannerSettings(f.ctx, BannerSettingsPatch{Enabled: ptr(false), ExpectedVersion: ptr(cur.Settings.Version)})
	if err != nil || out.Enabled || out.Version != cur.Settings.Version+1 || out.RotationSeconds != 30 {
		t.Fatalf("saved: %+v %v", out, err)
	}
	if got := f.audit.actions(); got[len(got)-1] != "site_banner.settings_updated" {
		t.Fatalf("audit: %v", got)
	}
}

func TestFreeModeConfirmationReasonAndWindow(t *testing.T) {
	f := newSiteFixture(t)
	now := f.clk.Now()
	ends := now.Add(48 * time.Hour)
	good := FreeModeInput{Enabled: ptr(true), Confirmation: "ENABLE FREE MODE", Reason: ptr("Launch week promotion"), EndsAt: &ends}
	with := func(mod func(*FreeModeInput)) FreeModeInput {
		in := good
		mod(&in)
		return in
	}
	cases := map[string]struct {
		in  FreeModeInput
		key string
	}{
		"no enabled flag":      {FreeModeInput{Reason: ptr("Launch week")}, "enabled"},
		"no confirmation":      {with(func(i *FreeModeInput) { i.Confirmation = "" }), "confirmation"},
		"lowercase":            {with(func(i *FreeModeInput) { i.Confirmation = "enable free mode" }), "confirmation"},
		"trailing space":       {with(func(i *FreeModeInput) { i.Confirmation = "ENABLE FREE MODE " }), "confirmation"},
		"other phrase":         {with(func(i *FreeModeInput) { i.Confirmation = "yes" }), "confirmation"},
		"no reason":            {with(func(i *FreeModeInput) { i.Reason = nil }), "reason"},
		"short reason":         {with(func(i *FreeModeInput) { i.Reason = ptr("abcd") }), "reason"},
		"blank reason":         {with(func(i *FreeModeInput) { i.Reason = ptr("      ") }), "reason"},
		"long reason":          {with(func(i *FreeModeInput) { i.Reason = ptr(strings.Repeat("r", 501)) }), "reason"},
		"past end":             {with(func(i *FreeModeInput) { e := now.Add(-time.Minute); i.EndsAt = &e }), "ends_at"},
		"end now":              {with(func(i *FreeModeInput) { i.EndsAt = &now }), "ends_at"},
		"end beyond 90 days":   {with(func(i *FreeModeInput) { e := now.Add(91 * 24 * time.Hour); i.EndsAt = &e }), "ends_at"},
		"version out of range": {with(func(i *FreeModeInput) { i.ExpectedVersion = ptr(0) }), "expected_version"},
	}
	for name, c := range cases {
		_, err := f.svc.UpdateFreeMode(f.ctx, c.in)
		if siteFields(t, err)[c.key] == "" {
			t.Errorf("%s: no %s field: %v", name, c.key, err)
		}
	}
	if f.svc.FreeModeActive(f.ctx) || len(f.audit.actions()) != 0 {
		t.Fatal("a rejected request switched free mode on or was audited")
	}
	if fields := siteFields(t, func() error { _, err := f.svc.UpdateFreeMode(f.ctx, cases["lowercase"].in); return err }()); fields["confirmation"] != msgFreeConfirmation {
		t.Fatalf("message: %v", fields)
	}
	// Boundaries: exactly 5 characters, exactly 90 days, no end at all.
	max := now.Add(90 * 24 * time.Hour)
	st, err := f.svc.UpdateFreeMode(f.ctx, with(func(i *FreeModeInput) { i.Reason = ptr("12345"); i.EndsAt = &max }))
	if err != nil || !st.Enabled || !st.Active || st.EnabledAt == nil || st.EnabledBy == nil || st.EnabledBy.Name != "Ada Admin" {
		t.Fatalf("enabled: %+v %v", st, err)
	}
	if !f.svc.FreeModeActive(f.ctx) {
		t.Fatal("FreeModeActive after enabling")
	}
	// Disabling needs no confirmation; the reason is optional.
	off, err := f.svc.UpdateFreeMode(f.ctx, FreeModeInput{Enabled: ptr(false)})
	if err != nil || off.Enabled || off.Active || off.DisabledAt == nil || off.DisabledBy == nil || off.Reason != "12345" {
		t.Fatalf("disabled: %+v %v", off, err)
	}
	if f.svc.FreeModeActive(f.ctx) {
		t.Fatal("FreeModeActive after disabling")
	}
	if _, err := f.svc.UpdateFreeMode(f.ctx, FreeModeInput{Enabled: ptr(false), Reason: ptr(strings.Repeat("r", 501))}); siteFields(t, err)["reason"] != msgFreeReasonLong {
		t.Fatalf("long disable reason: %v", err)
	}
	// Switching off again is a quiet no-op.
	n := len(f.audit.actions())
	again, err := f.svc.UpdateFreeMode(f.ctx, FreeModeInput{Enabled: ptr(false)})
	if err != nil || again.Version != off.Version || len(f.audit.actions()) != n {
		t.Fatalf("second disable: %+v %v", again, err)
	}
	if got := f.audit.actions(); !slices.Equal(got, []string{"site.free_mode_enabled", "site.free_mode_disabled"}) {
		t.Fatalf("audit: %v", got)
	}
	if prev, _ := f.audit.rows[1].Metadata["previous"].(map[string]any); prev["enabled"] != true || prev["reason"] != "12345" {
		t.Fatalf("audit previous: %v", f.audit.rows[1].Metadata)
	}
	// Re-enabling after a break starts a new period.
	f.clk.Advance(time.Hour)
	again, _ = f.svc.UpdateFreeMode(f.ctx, with(func(i *FreeModeInput) {}))
	if again.EnabledAt == nil || !again.EnabledAt.After(*st.EnabledAt) || again.DisabledAt != nil {
		t.Fatalf("new period: %+v vs %+v", again, st)
	}
	// Changing the end of a running period keeps its start (statistics continue).
	e2 := now.Add(72 * time.Hour)
	upd, err := f.svc.UpdateFreeMode(f.ctx, with(func(i *FreeModeInput) { i.EndsAt = &e2 }))
	if err != nil || !upd.EnabledAt.Equal(*again.EnabledAt) {
		t.Fatalf("update of a running period: %+v %v", upd, err)
	}
	// Optimistic concurrency.
	if _, err := f.svc.UpdateFreeMode(f.ctx, with(func(i *FreeModeInput) { i.ExpectedVersion = ptr(1) })); !isCode(err, apperror.CodeVersionConflict) {
		t.Fatalf("stale version: %v", err)
	}
}

func TestFreeModeStatsAreCached(t *testing.T) {
	f := newSiteFixture(t)
	f.repo.totals = repository.SponsoredTotals{Sessions: 7, Live: 2, Minutes: 31, Credits: 1240}
	f.repo.earnings = 744
	st, _ := f.svc.FreeMode(f.ctx)
	if st.Stats.Since != nil || st.Stats.SponsoredSessions != 0 || f.repo.statsN != 0 {
		t.Fatalf("never enabled: %+v", st.Stats)
	}
	if _, err := f.svc.UpdateFreeMode(f.ctx, FreeModeInput{Enabled: ptr(true), Confirmation: "ENABLE FREE MODE", Reason: ptr("Weekend")}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		st, _ = f.svc.FreeMode(f.ctx)
	}
	s := st.Stats
	if s.Since == nil || s.SponsoredSessions != 7 || s.LiveSponsoredSessions != 2 || s.SponsoredMinutes != 31 || s.SponsoredCredits != 1240 || s.AdvisorEarnings != 744 {
		t.Fatalf("stats: %+v", s)
	}
	if f.repo.statsN != 1 {
		t.Fatalf("statistics were computed %d times for 5 reads", f.repo.statsN)
	}
	// A change of the setting drops the cached statistics.
	if _, err := f.svc.UpdateFreeMode(f.ctx, FreeModeInput{Enabled: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	f.svc.FreeMode(f.ctx)
	if f.repo.statsN != 2 {
		t.Fatalf("statistics not refreshed after a change: %d", f.repo.statsN)
	}
}

func TestOptTimeDistinguishesAbsentFromNull(t *testing.T) {
	var p BannerPatch
	if err := decodeJSONForTest(`{"starts_at": null, "ends_at": "2026-12-01T10:00:00Z"}`, &p); err != nil {
		t.Fatal(err)
	}
	if !p.StartsAt.Set || p.StartsAt.Value != nil || !p.EndsAt.Set || p.EndsAt.Value == nil || p.Active != nil {
		t.Fatalf("decoded: %+v", p)
	}
	var none BannerPatch
	if err := decodeJSONForTest(`{}`, &none); err != nil || none.StartsAt.Set || none.EndsAt.Set {
		t.Fatalf("absent: %+v %v", none, err)
	}
	if err := decodeJSONForTest(`{"starts_at": "tomorrow"}`, &none); err == nil {
		t.Fatal("invalid time accepted")
	}
}
