package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/seed"
)

// revisions 5.0: pricing catalog and welcome offer (§24.1, §24.2).

// canonical returns the canonical catalog with IDs.
func canonical() []domain.CreditPack {
	packs := seed.CanonicalPacks()
	for i := range packs {
		packs[i].ID = domain.NewID()
	}
	return packs
}

func badgesOf(packs []domain.CreditPack, counts map[string]int64, min int) map[string][]PackBadge {
	bp := make([]badgePack, 0, len(packs))
	byID := map[uuid.UUID]int64{}
	for i := range packs {
		if packs[i].Active {
			bp = append(bp, badgePackOf(&packs[i]))
		}
		byID[packs[i].ID] = counts[packs[i].Code]
	}
	raw := computeBadges(bp, byID, min)
	out := map[string][]PackBadge{}
	for i := range packs {
		out[packs[i].Code] = raw[packs[i].ID]
	}
	return out
}

func badgeString(bs []PackBadge) string {
	var parts []string
	for _, b := range bs {
		if b.Type == BadgeSave {
			parts = append(parts, "save:"+itoa(b.Percent))
		} else {
			parts = append(parts, b.Type)
		}
	}
	return strings.Join(parts, ",")
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestCanonicalBadgesFloorTheSavings(t *testing.T) {
	got := badgesOf(canonical(), nil, 25)
	want := map[string]string{
		"welcome_600":  "new_client_offer",
		"quick_150":    "", // the reference: no savings against itself
		"starter_315":  "save:4",
		"popular_675":  "save:11",
		"deep_1350":    "save:16",
		"premium_2400": "best_value,save:21",
	}
	for code, w := range want {
		if g := badgeString(got[code]); g != w {
			t.Errorf("%s: badges %q, want %q", code, g, w)
		}
	}
}

func TestSavePercentIsIntegerMathAndNeverOverstated(t *testing.T) {
	cases := []struct {
		price, credits, refP, refC int64
		want                       int
	}{
		{4000, 315, 2000, 150, 4},    // 4.76 -> 4
		{8000, 675, 2000, 150, 11},   // 11.1
		{15000, 1350, 2000, 150, 16}, // 16.67 -> 16, never rounded up to 17
		{25000, 2400, 2000, 150, 21}, // 21.875 -> 21
		{2000, 150, 2000, 150, 0},    // the reference itself
		{3000, 150, 2000, 150, 0},    // dearer per credit
		{1999, 150, 2000, 150, 0},    // 0.05% -> below 1
		{1980, 150, 2000, 150, 1},    // exactly 1%
		{1000, 0, 2000, 150, 0},      // no credits: unusable
		{1000, 100, 0, 150, 0},       // unusable reference
	}
	for _, c := range cases {
		if got := savePercent(c.price, c.credits, c.refP, c.refC); got != c.want {
			t.Errorf("savePercent(%+v) = %d, want %d", c, got, c.want)
		}
	}
}

func TestMostPopularNeedsEvidenceAndAStrictLeader(t *testing.T) {
	packs := canonical()
	// Below the threshold: nobody gets the badge, however clear the leader.
	got := badgesOf(packs, map[string]int64{"popular_675": 10, "starter_315": 2}, 25)
	for code, b := range got {
		if strings.Contains(badgeString(b), BadgeMostPopular) {
			t.Fatalf("%s got most_popular below the threshold", code)
		}
	}
	// At the threshold with a strict leader.
	got = badgesOf(packs, map[string]int64{"popular_675": 20, "starter_315": 5}, 25)
	if !strings.HasPrefix(badgeString(got["popular_675"]), BadgeMostPopular) || strings.Contains(badgeString(got["starter_315"]), BadgeMostPopular) {
		t.Fatalf("leader: %v / %v", got["popular_675"], got["starter_315"])
	}
	// The badge comes before best_value and save.
	if g := badgeString(got["popular_675"]); g != "most_popular,save:11" {
		t.Fatalf("order: %s", g)
	}
	// A tie at the top: nobody.
	got = badgesOf(packs, map[string]int64{"popular_675": 13, "starter_315": 13}, 25)
	for code, b := range got {
		if strings.Contains(badgeString(b), BadgeMostPopular) {
			t.Fatalf("%s got most_popular in a tie", code)
		}
	}
	// The intro pack never competes: welcome purchases do not count.
	got = badgesOf(packs, map[string]int64{"welcome_600": 100, "deep_1350": 30, "starter_315": 1}, 25)
	if strings.Contains(badgeString(got["welcome_600"]), BadgeMostPopular) || !strings.Contains(badgeString(got["deep_1350"]), BadgeMostPopular) {
		t.Fatalf("intro pack competed: %v %v", got["welcome_600"], got["deep_1350"])
	}
}

func TestBestValueIsStrictAndNeedsAComparison(t *testing.T) {
	a := badgePack{ID: domain.NewID(), Kind: domain.PackStandard, Credits: 100, PriceCents: 1000}
	b := badgePack{ID: domain.NewID(), Kind: domain.PackStandard, Credits: 200, PriceCents: 2000} // same rate as a
	c := badgePack{ID: domain.NewID(), Kind: domain.PackStandard, Credits: 100, PriceCents: 1500}
	if got := computeBadges([]badgePack{a, b, c}, nil, 25); len(got[a.ID]) != 0 || len(got[b.ID]) != 0 {
		t.Fatalf("a tie for the lowest price per credit has no best value: %v", got)
	}
	d := badgePack{ID: domain.NewID(), Kind: domain.PackStandard, Credits: 300, PriceCents: 2000}
	if got := computeBadges([]badgePack{a, b, d}, nil, 25); badgeString(got[d.ID]) != "best_value" {
		t.Fatalf("strictly cheapest: %v", got)
	}
	if got := computeBadges([]badgePack{a}, nil, 25); len(got[a.ID]) != 0 {
		t.Fatalf("a single pack has nothing to be best against: %v", got)
	}
	// Without a reference there are no savings.
	if got := computeBadges([]badgePack{a, d}, nil, 25); strings.Contains(badgeString(got[d.ID]), BadgeSave) {
		t.Fatalf("savings without a reference: %v", got)
	}
}

func TestOfferEligibilityRules(t *testing.T) {
	cases := []struct {
		name                                         string
		introActive, newOnly, hasRedemption, hasPaid bool
		ok                                           bool
		reason                                       string
	}{
		{"new client", true, true, false, false, true, ""},
		{"no intro pack", false, true, false, false, false, apperror.OfferReasonNoOffer},
		{"already redeemed", true, true, true, false, false, apperror.OfferReasonAlreadyRedeemed},
		{"paid before", true, true, false, true, false, apperror.OfferReasonNotNewClient},
		{"paid before, offer open to everyone", true, false, false, true, true, ""},
		{"redeemed beats paid", true, true, true, true, false, apperror.OfferReasonAlreadyRedeemed},
		{"redeemed even when open to everyone", true, false, true, true, false, apperror.OfferReasonAlreadyRedeemed},
	}
	for _, c := range cases {
		ok, reason := OfferEligible(c.introActive, c.newOnly, c.hasRedemption, c.hasPaid)
		if ok != c.ok || reason != c.reason {
			t.Errorf("%s: got (%v, %q), want (%v, %q)", c.name, ok, reason, c.ok, c.reason)
		}
	}
}

func TestOfferUnavailableCarriesTheReason(t *testing.T) {
	for _, reason := range []string{apperror.OfferReasonAlreadyRedeemed, apperror.OfferReasonNotNewClient, apperror.OfferReasonPaymentUsed,
		apperror.OfferReasonCardCheck, apperror.OfferReasonNoOffer} {
		e := apperror.OfferUnavailable(reason)
		if e.Code != apperror.CodeOfferUnavailable || e.Status != 409 || e.Details["reason"] != reason || e.Message == "" {
			t.Errorf("%s: %+v", reason, e)
		}
	}
	if e := apperror.OfferUnavailable("nonsense"); e.Details["reason"] != apperror.OfferReasonNoOffer {
		t.Fatalf("unknown reasons read as no_offer: %+v", e)
	}
}

func TestCardKeyIsAKeyedHashOfProviderAndFingerprint(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	o := NewOfferService(nil, clock.NewFake(time.Now()), key, true)
	k := o.CardKey("sandbox_a", "fp_123")
	if len(k) != 64 || k != o.CardKey("sandbox_a", "fp_123") {
		t.Fatalf("a card key is a stable hex SHA-256 HMAC: %q", k)
	}
	if strings.Contains(k, "fp_123") {
		t.Fatal("the raw fingerprint must never appear in the key")
	}
	if k == o.CardKey("sandbox_b", "fp_123") || k == o.CardKey("sandbox_a", "fp_124") {
		t.Fatal("provider and fingerprint both feed the key")
	}
	if o.CardKey("sandbox_a", "") != "" {
		t.Fatal("no fingerprint, no key")
	}
	if other := NewOfferService(nil, clock.NewFake(time.Now()), []byte("another-data-encryption-key-0000"), true); other.CardKey("sandbox_a", "fp_123") == k {
		t.Fatal("the key derives from the data encryption key")
	}
	// The derivation has its own label: it differs from the key a plain
	// keyed hash of the fingerprint would give.
	if cardKey(string(key), "sandbox_a", "fp_123") == k {
		t.Fatal("the HMAC key must be derived with a distinct label, not the raw data key")
	}
	if offerKeyLabel == "staryield:address-hash:v1" || offerKeyLabel == "staryield:push-endpoint:v1" {
		t.Fatal("the offer key label must be distinct from the other derived keys")
	}
}

// ---------------------------------------------------------------------------
// state machine
// ---------------------------------------------------------------------------

type fakePricingRepo struct {
	repository.PricingRepository
	mu     sync.Mutex
	offers []*domain.OfferRedemption
	packs  []domain.CreditPack
	counts []repository.PackCount
	state  repository.CatalogState
	fail   error // packs / counts / state fail with it
	loads  int
}

func (f *fakePricingRepo) ReserveOffer(_ context.Context, r *domain.OfferRedemption) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, o := range f.offers {
		if o.Status == domain.OfferReleased {
			continue
		}
		if o.UserID == r.UserID {
			return repository.ErrOfferUserTaken
		}
		if r.CardKey != "" && o.Provider == r.Provider && o.CardKey == r.CardKey {
			return repository.ErrOfferCardTaken
		}
	}
	cp := *r
	f.offers = append(f.offers, &cp)
	return nil
}

func (f *fakePricingRepo) MoveOffer(_ context.Context, purchaseID uuid.UUID, from, to domain.OfferStatus, _ time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, o := range f.offers {
		if o.PurchaseID == purchaseID && o.Status == from {
			o.Status = to
			return true, nil
		}
	}
	return false, nil
}

func (f *fakePricingRepo) CatalogState(context.Context, uuid.UUID) (*repository.CatalogState, error) {
	if f.fail != nil {
		return nil, f.fail
	}
	st := f.state
	return &st, nil
}

func (f *fakePricingRepo) Packs(context.Context) ([]domain.CreditPack, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loads++
	return f.packs, f.fail
}

func (f *fakePricingRepo) PurchaseCounts(context.Context, time.Time) ([]repository.PackCount, error) {
	return f.counts, f.fail
}

func (f *fakePricingRepo) status(purchaseID uuid.UUID) domain.OfferStatus {
	for _, o := range f.offers {
		if o.PurchaseID == purchaseID {
			return o.Status
		}
	}
	return ""
}

func purchaseOf(user uuid.UUID) *domain.CreditPurchase {
	return &domain.CreditPurchase{ID: domain.NewID(), UserID: user, PackID: domain.NewID()}
}

func TestOfferStateMachine(t *testing.T) {
	ctx := context.Background()
	repo := &fakePricingRepo{}
	o := NewOfferService(repo, clock.NewFake(time.Now()), []byte("0123456789abcdef0123456789abcdef"), true)
	user, other := uuid.New(), uuid.New()
	plan := &OfferPlan{Provider: "sandbox_a", CardKey: o.CardKey("sandbox_a", "fp_1")}

	// reserved -> released: the client may try again.
	p1 := purchaseOf(user)
	if err := o.Reserve(ctx, plan, p1); err != nil || repo.status(p1.ID) != domain.OfferReserved {
		t.Fatalf("reserve: %v %s", err, repo.status(p1.ID))
	}
	// Reserved blocks the same account and the same card.
	if err := o.Reserve(ctx, plan, purchaseOf(user)); !offerReason(err, apperror.OfferReasonAlreadyRedeemed) {
		t.Fatalf("second reservation by the same account: %v", err)
	}
	if err := o.Reserve(ctx, plan, purchaseOf(other)); !offerReason(err, apperror.OfferReasonPaymentUsed) {
		t.Fatalf("same card on another account: %v", err)
	}
	if err := o.Release(ctx, p1.ID); err != nil || repo.status(p1.ID) != domain.OfferReleased {
		t.Fatalf("release: %v %s", err, repo.status(p1.ID))
	}
	p2 := purchaseOf(user)
	if err := o.Reserve(ctx, plan, p2); err != nil {
		t.Fatalf("a released offer is retryable: %v", err)
	}
	// reserved -> redeemed.
	if err := o.Redeem(ctx, p2.ID); err != nil || repo.status(p2.ID) != domain.OfferRedeemed {
		t.Fatalf("redeem: %v %s", err, repo.status(p2.ID))
	}
	// Terminal: redeemed is never released (a refund does not restore eligibility),
	// and released is never redeemed.
	if err := o.Release(ctx, p2.ID); err != nil || repo.status(p2.ID) != domain.OfferRedeemed {
		t.Fatalf("a redeemed offer stays redeemed: %v %s", err, repo.status(p2.ID))
	}
	if err := o.Redeem(ctx, p1.ID); err != nil || repo.status(p1.ID) != domain.OfferReleased {
		t.Fatalf("a released offer is not redeemed later: %v %s", err, repo.status(p1.ID))
	}
	if err := o.Reserve(ctx, plan, purchaseOf(user)); !offerReason(err, apperror.OfferReasonAlreadyRedeemed) {
		t.Fatalf("redeemed blocks the account for good: %v", err)
	}
	// A purchase without a reservation, and a nil service or plan, are no-ops.
	var nilOffers *OfferService
	if nilOffers.Redeem(ctx, uuid.New()) != nil || nilOffers.Release(ctx, uuid.New()) != nil || o.Redeem(ctx, uuid.New()) != nil ||
		nilOffers.Reserve(ctx, plan, p1) != nil || o.Reserve(ctx, nil, p1) != nil {
		t.Fatal("no-ops must not fail")
	}
}

func offerReason(err error, reason string) bool {
	var e *apperror.Error
	return errors.As(err, &e) && e.Code == apperror.CodeOfferUnavailable && e.Details["reason"] == reason
}

func TestOfferPlanChecksCardAndEligibility(t *testing.T) {
	ctx := context.Background()
	repo := &fakePricingRepo{}
	key := []byte("0123456789abcdef0123456789abcdef")
	o := NewOfferService(repo, clock.NewFake(time.Now()), key, true)
	intro := &domain.CreditPack{ID: uuid.New(), Kind: domain.PackIntro, Active: true, NewClientsOnly: true}
	user := uuid.New()

	// A hosted checkout has no card to check, whatever the configuration.
	if _, err := o.Plan(ctx, user, intro, false, "", ""); !offerReason(err, apperror.OfferReasonCardCheck) {
		t.Fatalf("hosted checkout: %v", err)
	}
	if _, err := NewOfferService(repo, clock.NewFake(time.Now()), key, false).Plan(ctx, user, intro, false, "", ""); !offerReason(err, apperror.OfferReasonCardCheck) {
		t.Fatalf("hosted checkout with the card check off: %v", err)
	}
	// No fingerprint: refused while the card check is required.
	if _, err := o.Plan(ctx, user, intro, true, "sandbox_a", ""); !offerReason(err, apperror.OfferReasonCardCheck) {
		t.Fatalf("unknown fingerprint: %v", err)
	}
	// With the check off the offer is per account and the key is empty.
	off := NewOfferService(repo, clock.NewFake(time.Now()), key, false)
	plan, err := off.Plan(ctx, user, intro, true, "sandbox_a", "")
	if err != nil || plan.CardKey != "" || plan.Provider != "sandbox_a" {
		t.Fatalf("card check off: %+v %v", plan, err)
	}
	plan, err = o.Plan(ctx, user, intro, true, "sandbox_a", "fp_1")
	if err != nil || plan.CardKey != o.CardKey("sandbox_a", "fp_1") {
		t.Fatalf("plan: %+v %v", plan, err)
	}
	// Eligibility facts.
	repo.state = repository.CatalogState{HasPaid: true}
	if _, err := o.Plan(ctx, user, intro, true, "sandbox_a", "fp_1"); !offerReason(err, apperror.OfferReasonNotNewClient) {
		t.Fatalf("paid before: %v", err)
	}
	repo.state = repository.CatalogState{HasRedemption: true}
	if _, err := o.Plan(ctx, user, intro, true, "sandbox_a", "fp_1"); !offerReason(err, apperror.OfferReasonAlreadyRedeemed) {
		t.Fatalf("redeemed: %v", err)
	}
	repo.state = repository.CatalogState{}
	inactive := *intro
	inactive.Active = false
	if _, err := o.Plan(ctx, user, &inactive, true, "sandbox_a", "fp_1"); !offerReason(err, apperror.OfferReasonNoOffer) {
		t.Fatalf("inactive intro pack: %v", err)
	}
}

// ---------------------------------------------------------------------------
// catalog and public summary
// ---------------------------------------------------------------------------

type fakeSiteRepo struct {
	repository.SiteRepository
	value domain.JSONMap
	err   error
}

func (f *fakeSiteRepo) Setting(_ context.Context, key string) (*domain.SiteSetting, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &domain.SiteSetting{Key: key, Value: f.value, Version: 3, UpdatedAt: time.Unix(1700000000, 0)}, nil
}

func newTestPricing(repo *fakePricingRepo) (*PricingService, *clock.Fake) {
	clk := clock.NewFake(time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC))
	site := &fakeSiteRepo{value: domain.JSONMap{}}
	trial := func(context.Context) (bool, int) { return true, 4 }
	return NewPricingService(PricingDeps{Repo: repo, Site: site, Signup: trial, Clock: clk, Cfg: PricingSettingsConfig{Currency: "USD"}}), clk
}

func TestPublicSummaryShapeLocaleAndValidator(t *testing.T) {
	repo := &fakePricingRepo{packs: canonical()}
	s, clk := newTestPricing(repo)
	ctx := context.Background()
	en, err := s.Summary(ctx, "en")
	if err != nil {
		t.Fatal(err)
	}
	v := en.View
	if v.Currency != "USD" || !v.FreeTrial.Enabled || v.FreeTrial.Minutes != 4 || v.RateRange != (RateRange{Min: 30, Max: 60}) {
		t.Fatalf("summary header: %+v", v)
	}
	if v.Reference == nil || v.Reference.Credits != 150 || v.Reference.PriceCents != 2000 {
		t.Fatalf("reference: %+v", v.Reference)
	}
	codes := []string{}
	for _, p := range v.Packs {
		codes = append(codes, p.Code)
	}
	if strings.Join(codes, ",") != "welcome_600,quick_150,starter_315,popular_675,deep_1350,premium_2400" {
		t.Fatalf("canonical order: %v", codes)
	}
	if v.Packs[1].Label != "One focused question" || v.Packs[2].Label != "" || v.Packs[0].Badges[0].Type != BadgeNewClientOffer {
		t.Fatalf("labels and badges: %+v", v.Packs)
	}
	fr, _ := s.Summary(ctx, "fr-CA")
	if fr.View.Packs[0].Name != "Pack de bienvenue" || fr.ETag == en.ETag {
		t.Fatalf("locale: %q etag %s vs %s", fr.View.Packs[0].Name, fr.ETag, en.ETag)
	}
	if de, _ := s.Summary(ctx, "de"); de.View.Packs[0].Name != "Welcome Pack" || de.ETag != en.ETag {
		t.Fatalf("an unsupported locale reads as English: %q", de.View.Packs[0].Name)
	}
	// The validator ignores generated_at: it only moves with the content.
	clk.Advance(5 * time.Second)
	again, _ := s.Summary(ctx, "en")
	if again.ETag != en.ETag || again.View.GeneratedAt.Equal(en.View.GeneratedAt) {
		t.Fatalf("etag must be stable while generated_at moves: %s %s", again.ETag, en.ETag)
	}
	if !strings.HasPrefix(en.ETag, `W/"`) {
		t.Fatalf("weak validator: %s", en.ETag)
	}
	// The JSON matches the contract (no internal economics, badges never null).
	var raw map[string]any
	if err := json.Unmarshal(en.Body, &raw); err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"processing_fee_bps", "processing_fixed_cents", "popular_min_purchases"} {
		if strings.Contains(string(en.Body), banned) {
			t.Fatalf("%s leaked into the public summary", banned)
		}
	}
	if !strings.Contains(string(en.Body), `"badges":[]`) {
		t.Fatal("a pack without badges serialises an empty list")
	}
}

func TestSummaryTrialAndInactivePacks(t *testing.T) {
	packs := canonical()
	packs[2].Active = false // Starter off: no longer a pack, no longer compared
	repo := &fakePricingRepo{packs: packs}
	s, _ := newTestPricing(repo)
	s.d.Signup = func(context.Context) (bool, int) { return false, 0 }
	res, err := s.Summary(context.Background(), "en")
	if err != nil {
		t.Fatal(err)
	}
	if res.View.FreeTrial.Enabled || res.View.FreeTrial.Minutes != 0 || len(res.View.Packs) != 5 {
		t.Fatalf("trial off / inactive pack hidden: %+v", res.View)
	}
}

func TestSnapshotOutage(t *testing.T) {
	repo := &fakePricingRepo{packs: canonical()}
	s, _ := newTestPricing(repo)
	ctx := context.Background()
	// Nothing cached and the database down: no invented prices.
	repo.fail = errors.New("database down")
	if _, err := s.Summary(ctx, "en"); err == nil {
		t.Fatal("expected an error")
	} else if e, ok := apperror.As(err); !ok || e.Status != 503 {
		t.Fatalf("outage must be a 503: %v", err)
	}
	// A good copy is kept as the outage fallback, however stale.
	repo.fail = nil
	if _, err := s.Summary(ctx, "en"); err != nil {
		t.Fatal(err)
	}
	s.dropLocal() // stale, but still held
	repo.fail = errors.New("database down")
	res, err := s.Summary(ctx, "en")
	if err != nil || len(res.View.Packs) != 6 {
		t.Fatalf("the last good copy is served during an outage: %v", err)
	}
}

func TestSnapshotIsLoadedOncePerBurst(t *testing.T) {
	repo := &fakePricingRepo{packs: canonical()}
	s, _ := newTestPricing(repo)
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Summary(context.Background(), "en"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if repo.loads > 3 {
		t.Fatalf("a burst should share one load (singleflight + cache), got %d", repo.loads)
	}
	before := repo.loads
	s.changed(context.Background())
	if _, err := s.Summary(context.Background(), "en"); err != nil || repo.loads != before+1 {
		t.Fatalf("a change drops the copy: %v loads %d -> %d", err, before, repo.loads)
	}
}

func TestTopUpCatalogEligibility(t *testing.T) {
	repo := &fakePricingRepo{packs: canonical()}
	s, _ := newTestPricing(repo)
	ctx := context.Background()
	user := uuid.New()
	codes := func(c *TopUpCatalog) string {
		var out []string
		for _, p := range c.Packs {
			out = append(out, p.Code)
		}
		return strings.Join(out, ",")
	}

	// A new client: the welcome pack first, offer eligible, wallet and range included.
	repo.state = repository.CatalogState{Balance: 40, Held: 5, FreeMinutes: 3}
	c, err := s.Catalog(ctx, user, "en")
	if err != nil {
		t.Fatal(err)
	}
	if !c.WelcomeOffer.Eligible || c.WelcomeOffer.PackID == nil || codes(c) != "welcome_600,quick_150,starter_315,popular_675,deep_1350,premium_2400" ||
		c.Wallet != (WalletView{Balance: 40, Held: 5, FreeMinutes: 3}) || c.RateRange != (RateRange{Min: 30, Max: 60}) {
		t.Fatalf("new client: %+v", c)
	}
	// After a succeeded purchase: no welcome pack, never disabled, just absent.
	repo.state = repository.CatalogState{HasPaid: true}
	c, _ = s.Catalog(ctx, user, "en")
	if c.WelcomeOffer.Eligible || c.WelcomeOffer.PackID != nil || codes(c) != "quick_150,starter_315,popular_675,deep_1350,premium_2400" {
		t.Fatalf("returning client: %+v", c)
	}
	// A reserved or redeemed offer hides it, even for a client who never paid.
	repo.state = repository.CatalogState{HasRedemption: true}
	if c, _ = s.Catalog(ctx, user, "en"); c.WelcomeOffer.Eligible || strings.Contains(codes(c), "welcome") {
		t.Fatalf("redeemed: %+v", c)
	}
	// An intro pack open to everyone stays for returning clients.
	packs := canonical()
	packs[0].NewClientsOnly = false
	repo.packs = packs
	s.changed(ctx)
	repo.state = repository.CatalogState{HasPaid: true}
	if c, _ = s.Catalog(ctx, user, "en"); !c.WelcomeOffer.Eligible {
		t.Fatalf("not new_clients_only: %+v", c)
	}
	// An eligible intro pack that an admin sorted last is still shown first.
	packs = canonical()
	packs[0].SortOrder = 99
	repo.packs = []domain.CreditPack{packs[1], packs[2], packs[0]}
	s.changed(ctx)
	repo.state = repository.CatalogState{}
	if c, _ = s.Catalog(ctx, user, "fr"); c.Packs[0].Code != "welcome_600" || c.Packs[0].Name != "Pack de bienvenue" {
		t.Fatalf("intro first: %+v", c.Packs)
	}
	// No intro pack at all.
	repo.packs = canonical()[1:]
	s.changed(ctx)
	if c, _ = s.Catalog(ctx, user, "en"); c.WelcomeOffer.Eligible || c.WelcomeOffer.PackID != nil {
		t.Fatalf("no intro pack: %+v", c)
	}
}

func TestPackAndSettingsValidation(t *testing.T) {
	fields := map[string]string{}
	got := validatePackL10n("name", map[string]string{"en": "  Welcome‮ Pack ", "fr": "<b>Pack</b>", "de": "x"}, 60, true, fields)
	if got["en"] != "Welcome Pack" || fields["name.fr"] != msgBannerHTML || fields["name.de"] != msgBannerLocale {
		t.Fatalf("clean/markup/locale: %v %v", got, fields)
	}
	fields = map[string]string{}
	validatePackL10n("name", map[string]string{"fr": "Seulement"}, 60, true, fields)
	if fields["name.en"] != msgPackNameEN {
		t.Fatalf("english name required: %v", fields)
	}
	fields = map[string]string{}
	validatePackL10n("tagline", map[string]string{"en": strings.Repeat("a", 121)}, 120, false, fields)
	validatePackL10n("label", map[string]string{"en": strings.Repeat("a", 41)}, 40, false, fields)
	validatePackL10n("name", map[string]string{"en": strings.Repeat("a", 61)}, 60, true, fields)
	if fields["tagline.en"] != msgPackTagline || fields["label.en"] != msgBannerCTALength || fields["name.en"] == "" {
		t.Fatalf("lengths: %v", fields)
	}
	// A plain < in prose is accepted like banners.
	fields = map[string]string{}
	if got := validatePackL10n("tagline", map[string]string{"en": "5 < 6 and 7 > 3"}, 120, false, fields); got["en"] == "" || len(fields) != 0 {
		t.Fatalf("plain comparison: %v %v", got, fields)
	}

	for _, code := range []string{"welcome_600", "ab", "a1_b2"} {
		if !validPackCode(code) {
			t.Errorf("%q should be valid", code)
		}
	}
	for _, code := range []string{"a", "Welcome", "has space", "dash-ed", strings.Repeat("a", 41), ""} {
		if validPackCode(code) {
			t.Errorf("%q should be invalid", code)
		}
	}
	fields = map[string]string{}
	validPackNumbers(ptr64(0), ptr64(49), true, fields)
	if fields["credits"] == "" || fields["price_cents"] == "" {
		t.Fatalf("bounds: %v", fields)
	}
	fields = map[string]string{}
	validPackNumbers(ptr64(1_000_000), ptr64(10_000_000), true, fields)
	validPackNumbers(nil, nil, true, fields)
	if len(fields) != 2 || fields["credits"] != msgBannerRequired {
		t.Fatalf("upper bounds fine, absent required on create: %v", fields)
	}

	// Cross-field rules.
	std := &domain.CreditPack{Kind: domain.PackStandard, Active: true, Reference: true}
	f := map[string]string{}
	packRules(std, f)
	if len(f) != 0 {
		t.Fatalf("an active standard reference is fine: %v", f)
	}
	for name, p := range map[string]*domain.CreditPack{
		"inactive reference": {Kind: domain.PackStandard, Active: false, Reference: true},
		"intro reference":    {Kind: domain.PackIntro, Active: true, Reference: true},
	} {
		f = map[string]string{}
		if packRules(p, f); f["reference"] != msgPackReference {
			t.Errorf("%s: %v", name, f)
		}
	}
	f = map[string]string{}
	if packRules(&domain.CreditPack{Kind: domain.PackStandard, Active: true, NewClientsOnly: true}, f); f["new_clients_only"] == "" {
		t.Errorf("new_clients_only is for the welcome offer: %v", f)
	}
}

func ptr64(v int64) *int64 { return &v }
func ptrInt(v int) *int    { return &v }

func TestPricingSettingsValidation(t *testing.T) {
	ok := PricingSettingsInput{RateMin: ptrInt(30), RateMax: ptrInt(60), PopularMinPurchases: ptrInt(25), PopularWindowDays: ptrInt(90),
		ProcessingFeeBps: ptrInt(800), ProcessingFixedCents: ptrInt(30)}
	got, fields := validatePricingSettings(ok)
	if len(fields) != 0 || got.RateMin != 30 || got.RateMax != 60 || got.ProcessingFeeBps != 800 {
		t.Fatalf("valid: %+v %v", got, fields)
	}
	bad := func(mutate func(*PricingSettingsInput), key string) {
		t.Helper()
		in := ok
		mutate(&in)
		if _, f := validatePricingSettings(in); f[key] == "" {
			t.Errorf("%s: expected a message, got %v", key, f)
		}
	}
	bad(func(i *PricingSettingsInput) { i.RateMin = ptrInt(0) }, "rate_min")
	bad(func(i *PricingSettingsInput) { i.RateMax = ptrInt(10001) }, "rate_max")
	bad(func(i *PricingSettingsInput) { i.RateMin, i.RateMax = ptrInt(50), ptrInt(40) }, "rate_max")
	bad(func(i *PricingSettingsInput) { i.PopularMinPurchases = ptrInt(0) }, "popular_min_purchases")
	bad(func(i *PricingSettingsInput) { i.PopularWindowDays = ptrInt(6) }, "popular_window_days")
	bad(func(i *PricingSettingsInput) { i.PopularWindowDays = ptrInt(366) }, "popular_window_days")
	bad(func(i *PricingSettingsInput) { i.ProcessingFeeBps = ptrInt(3001) }, "processing_fee_bps")
	bad(func(i *PricingSettingsInput) { i.ProcessingFixedCents = ptrInt(501) }, "processing_fixed_cents")
	bad(func(i *PricingSettingsInput) { i.RateMin = nil }, "rate_min")
	bad(func(i *PricingSettingsInput) { i.ExpectedVersion = ptrInt(0) }, "expected_version")

	// An unreadable or out-of-range stored document falls back to the defaults.
	st := domain.SiteSetting{Value: domain.JSONMap{"rate_min": float64(70), "rate_max": float64(60), "popular_window_days": float64(1)}, Version: 2}
	d := pricingSettingsFrom(st)
	if d.RateMin != 30 || d.RateMax != 60 || d.PopularWindowDays != 90 || d.PopularMinPurchases != 25 || d.Version != 2 {
		t.Fatalf("fallback: %+v", d)
	}
}

func TestRateRangeRule(t *testing.T) {
	if !rateOutOfRange(29, 30, 60) || !rateOutOfRange(61, 30, 60) || rateOutOfRange(30, 30, 60) || rateOutOfRange(60, 30, 60) {
		t.Fatal("the range is inclusive")
	}
	if got := rateRangeMessage(30, 60); got != "Use a rate between 30 and 60 credits." {
		t.Fatalf("message: %s", got)
	}
}
