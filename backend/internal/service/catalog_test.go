package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

type fakeCatalogRepo struct {
	mu    sync.Mutex
	rows  []repository.AdvisorCardRow
	err   error
	calls atomic.Int32
	delay time.Duration
	// entered and release, when set, pause a read until the test lets it go.
	entered, release chan struct{}
}

func (f *fakeCatalogRepo) ListedCards(context.Context) ([]repository.AdvisorCardRow, error) {
	f.calls.Add(1)
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	if f.entered != nil {
		close(f.entered)
		<-f.release
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return append([]repository.AdvisorCardRow(nil), f.rows...), nil
}

func card(name string, rating float64, reviews int, skills ...string) repository.AdvisorCardRow {
	return repository.AdvisorCardRow{UserID: uuid.New(), Slug: name, Name: name, Headline: name + " reads tarot", Rating: rating,
		ReviewsCount: reviews, Skills: skills, Availability: domain.AvailabilityOnline}
}

func catalogWith(rows []repository.AdvisorCardRow, statuses map[string]domain.PresenceStatus) (*AdvisorCatalog, *fakeCatalogRepo, *clock.Fake) {
	repo := &fakeCatalogRepo{rows: rows}
	clk := clock.NewFake(time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC))
	bySlug := map[uuid.UUID]domain.PresenceStatus{}
	for _, r := range rows {
		if st, ok := statuses[r.Slug]; ok {
			bySlug[r.UserID] = st
		}
	}
	lookup := func(_ context.Context, list []domain.Psychic) (map[uuid.UUID]domain.PresenceStatus, error) {
		out := map[uuid.UUID]domain.PresenceStatus{}
		for _, p := range list {
			out[p.UserID] = bySlug[p.UserID]
		}
		return out, nil
	}
	return NewAdvisorCatalog(repo, lookup, clk), repo, clk
}

func slugs(items []AdvisorCard) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Slug
	}
	return out
}

func TestCatalogBrowsePagesAndFilters(t *testing.T) {
	var rows []repository.AdvisorCardRow
	for _, n := range []string{"ada", "bea", "cyd", "dax", "eve", "fay", "gus"} {
		rows = append(rows, card(n, 4.5, 10, "Tarot"))
	}
	rows[2].Skills = []string{"Numerology", "Astrology"}
	cat, _, _ := catalogWith(rows, map[string]domain.PresenceStatus{"ada": domain.PresenceOnline, "cyd": domain.PresenceBusy})
	ctx := context.Background()

	p, err := cat.Browse(ctx, BrowseQuery{PageSize: 3})
	if err != nil || p.Total != 7 || p.TotalPages != 3 || p.Page != 1 || len(p.Items) != 3 || slugs(p.Items)[0] != "ada" {
		t.Fatalf("page 1: %+v %v", p, err)
	}
	p, _ = cat.Browse(ctx, BrowseQuery{Page: 3, PageSize: 3})
	if len(p.Items) != 1 || p.Items[0].Slug != "gus" {
		t.Fatalf("last page: %+v", p)
	}
	p, _ = cat.Browse(ctx, BrowseQuery{Page: 9, PageSize: 3})
	if len(p.Items) != 0 || p.Total != 7 {
		t.Fatalf("past the end: %+v", p)
	}
	if p, _ = cat.Browse(ctx, BrowseQuery{Status: "Online"}); len(p.Items) != 1 || p.Items[0].Slug != "ada" {
		t.Fatalf("status filter: %+v", p)
	}
	if p, _ = cat.Browse(ctx, BrowseQuery{Search: "DAX"}); len(p.Items) != 1 || p.Items[0].Slug != "dax" {
		t.Fatalf("search: %+v", p)
	}
	if p, _ = cat.Browse(ctx, BrowseQuery{Skill: "astrology"}); len(p.Items) != 1 || p.Items[0].Slug != "cyd" {
		t.Fatalf("skill: %+v", p)
	}
	if p, _ = cat.Browse(ctx, BrowseQuery{Status: "offline"}); p.Total != 5 {
		t.Fatalf("offline default for unknown presence: %+v", p)
	}
	for _, q := range []BrowseQuery{{Page: -1}, {PageSize: 49}, {Status: "sleeping"}, {Search: strings.Repeat("a", 61)}, {Skill: strings.Repeat("b", 41)}} {
		if _, err := cat.Browse(ctx, q); !isValidation(err) {
			t.Errorf("%+v accepted: %v", q, err)
		}
	}
}

func isValidation(err error) bool {
	e, ok := apperror.As(err)
	return ok && e.Code == apperror.CodeValidation
}

func TestCatalogFeaturedRanking(t *testing.T) {
	old, recent := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	rows := []repository.AdvisorCardRow{
		card("lucky", 5.0, 1),                        // one perfect review
		card("steady", 4.9, 400, "a", "b", "c", "d"), // rated well by many
		card("busy", 5.0, 900),
		card("away", 5.0, 900),
		card("off-old", 4.2, 300),
		card("off-new", 4.2, 300),
	}
	rows[4].LastOnlineAt, rows[5].LastOnlineAt = &old, &recent
	cat, _, _ := catalogWith(rows, map[string]domain.PresenceStatus{"lucky": domain.PresenceOnline, "steady": domain.PresenceOnline,
		"busy": domain.PresenceBusy, "away": domain.PresenceAway})
	out, err := cat.Featured(context.Background(), 6)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(out.Items))
	for i, it := range out.Items {
		got[i] = it.Slug
	}
	want := []string{"steady", "lucky", "away", "busy", "off-new", "off-old"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
	if out.Items[0].CTA != "chat" || out.Items[2].CTA != "waitlist" || len(out.Items[0].Skills) != 3 {
		t.Fatalf("cta/skills: %+v", out.Items[:3])
	}
	if out3, _ := cat.Featured(context.Background(), 0); len(out3.Items) != featuredDefault {
		t.Fatalf("default limit: %d", len(out3.Items))
	}
	for _, n := range []int{-1, 7} {
		if _, err := cat.Featured(context.Background(), n); !isValidation(err) {
			t.Errorf("limit %d accepted", n)
		}
	}
}

func TestCatalogSnapshotCachingAndStaleness(t *testing.T) {
	cat, repo, clk := catalogWith([]repository.AdvisorCardRow{card("ada", 4.5, 3)}, nil)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if _, err := cat.Browse(ctx, BrowseQuery{}); err != nil {
			t.Fatal(err)
		}
	}
	if n := repo.calls.Load(); n != 1 {
		t.Fatalf("snapshot rebuilt %d times within the TTL", n)
	}
	cat.MarkStale()
	_, _ = cat.Browse(ctx, BrowseQuery{})
	if n := repo.calls.Load(); n != 2 {
		t.Fatalf("stale snapshot not rebuilt: %d", n)
	}
	clk.Advance(catalogTTL + time.Second)
	repo.mu.Lock()
	repo.err = errors.New("db down")
	repo.mu.Unlock()
	p, err := cat.Browse(ctx, BrowseQuery{})
	if err != nil || p.Total != 1 {
		t.Fatalf("previous snapshot should be served when a rebuild fails: %+v %v", p, err)
	}
}

func TestCatalogRebuildIsSingleFlight(t *testing.T) {
	cat, repo, _ := catalogWith([]repository.AdvisorCardRow{card("ada", 4.5, 3)}, nil)
	repo.delay = 50 * time.Millisecond
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := cat.Featured(context.Background(), 3); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if n := repo.calls.Load(); n != 1 {
		t.Fatalf("50 concurrent requests caused %d rebuilds", n)
	}
}

func TestWeightedRatingPrefersVolume(t *testing.T) {
	if weightedRating(5.0, 1, 4.5) >= weightedRating(4.9, 400, 4.5) {
		t.Fatal("a single 5-star review should not outrank 400 reviews at 4.9")
	}
	if weightedRating(4.5, 0, 4.5) != 4.5 {
		t.Fatal("no reviews should sit at the platform mean")
	}
}

func TestCatalogSetStatusPatchesInPlace(t *testing.T) {
	rows := []repository.AdvisorCardRow{card("ada", 4.5, 3), card("bea", 4.5, 3)}
	cat, repo, clk := catalogWith(rows, nil)
	ctx := context.Background()
	cat.SetStatus(rows[0].UserID, domain.PresenceOnline) // no snapshot yet: nothing to patch
	if p, _ := cat.Browse(ctx, BrowseQuery{Status: "online"}); p.Total != 0 {
		t.Fatalf("online before any change: %+v", p)
	}
	clk.Advance(time.Second)
	cat.SetStatus(rows[1].UserID, domain.PresenceOnline)
	cat.SetStatus(uuid.New(), domain.PresenceOnline) // not listed: ignored
	p, _ := cat.Browse(ctx, BrowseQuery{Status: "online"})
	if p.Total != 1 || p.Items[0].Slug != "bea" {
		t.Fatalf("patched status not served: %+v", p)
	}
	f, _ := cat.Featured(ctx, 2)
	if f.Items[0].Slug != "bea" || f.Items[0].CTA != "chat" || f.Items[1].CTA != "waitlist" {
		t.Fatalf("featured after patch: %+v", f.Items)
	}
	// Going offline keeps bea ahead of ada, who has never been online.
	cat.SetStatus(rows[1].UserID, domain.PresenceOffline)
	if f, _ = cat.Featured(ctx, 2); f.Items[0].Slug != "bea" || f.Items[0].Status != "offline" {
		t.Fatalf("last online not recorded: %+v", f.Items)
	}
	if n := repo.calls.Load(); n != 1 {
		t.Fatalf("status changes caused %d reads, want 1", n)
	}
}

func TestCatalogRebuildRacingAStatusChangeIsRedone(t *testing.T) {
	rows := []repository.AdvisorCardRow{card("ada", 4.5, 3)}
	cat, repo, _ := catalogWith(rows, nil)
	repo.entered, repo.release = make(chan struct{}), make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := cat.Browse(context.Background(), BrowseQuery{}); err != nil {
			t.Error(err)
		}
	}()
	<-repo.entered
	cat.SetStatus(rows[0].UserID, domain.PresenceOnline) // lands while the build reads
	repo.entered = nil
	close(repo.release)
	<-done
	if _, err := cat.Browse(context.Background(), BrowseQuery{}); err != nil {
		t.Fatal(err)
	}
	if n := repo.calls.Load(); n != 2 {
		t.Fatalf("a build that raced a status change was kept (%d reads)", n)
	}
}

// orderSlugs reads the directory (one page, no filter) in its public order.
func orderSlugs(t *testing.T, cat *AdvisorCatalog, q BrowseQuery) []string {
	t.Helper()
	q.PageSize = browseMaxSize
	p, err := cat.Browse(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	return slugs(p.Items)
}

func sameOrder(got, want []string) bool {
	return len(got) == len(want) && strings.Join(got, ",") == strings.Join(want, ",")
}

// TestPublicOrderLess walks every rule of the order, one boundary at a time:
// each case differs from its neighbour in exactly one respect.
func TestPublicOrderLess(t *testing.T) {
	early, late := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	mk := func(status string, reviews int, featured bool, name string) AdvisorCard {
		return AdvisorCard{Name: name, Status: status, ReviewsCount: reviews, Featured: featured}
	}
	key := func(weighted float64, rank, sortOrder int, last *time.Time) orderKey {
		return orderKey{weighted: weighted, featuredRank: rank, sortOrder: sortOrder, lastOnline: last}
	}
	cases := []struct {
		name   string
		a, b   AdvisorCard
		ka, kb orderKey
		want   bool // a sorts before b
	}{
		{"online before away", mk("online", 0, false, "z"), mk("away", 900, true, "a"), key(1, 9, 9, nil), key(5, 0, 0, &late), true},
		{"away before busy", mk("away", 0, false, "z"), mk("busy", 900, true, "a"), key(1, 9, 9, nil), key(5, 0, 0, &late), true},
		{"busy before offline", mk("busy", 0, false, "z"), mk("offline", 900, true, "a"), key(1, 9, 9, nil), key(5, 0, 0, &late), true},
		{"unknown status is offline", mk("", 0, false, "a"), mk("busy", 0, false, "a"), key(1, 0, 0, nil), key(1, 0, 0, nil), false},
		{"offline last", mk("offline", 0, false, "a"), mk("online", 0, false, "z"), key(5, 0, 0, nil), key(1, 0, 0, nil), false},
		{"weighted rating within a tier", mk("online", 1, false, "z"), mk("online", 900, false, "a"), key(4.6, 9, 9, nil), key(4.5, 0, 0, &late), true},
		{"review count after equal weighted rating", mk("online", 80, false, "z"), mk("online", 20, false, "a"), key(4.5, 9, 9, nil), key(4.5, 0, 0, &late), true},
		{"featured flag after equal reviews", mk("online", 10, true, "z"), mk("online", 10, false, "a"), key(4.5, 9, 9, nil), key(4.5, 0, 0, &late), true},
		{"featured rank, lower first", mk("online", 10, true, "z"), mk("online", 10, true, "a"), key(4.5, 1, 9, nil), key(4.5, 2, 0, &late), true},
		{"last online, newer first", mk("online", 10, false, "z"), mk("online", 10, false, "a"), key(4.5, 0, 9, &late), key(4.5, 0, 0, &early), true},
		{"ever online before never online", mk("online", 10, false, "z"), mk("online", 10, false, "a"), key(4.5, 0, 9, &early), key(4.5, 0, 0, nil), true},
		{"sort order", mk("online", 10, false, "z"), mk("online", 10, false, "a"), key(4.5, 0, 1, &late), key(4.5, 0, 2, &late), true},
		{"name last", mk("online", 10, false, "ada"), mk("online", 10, false, "bea"), key(4.5, 0, 1, &late), key(4.5, 0, 1, &late), true},
		{"identical is not less", mk("online", 10, false, "ada"), mk("online", 10, false, "ada"), key(4.5, 0, 1, &late), key(4.5, 0, 1, &late), false},
	}
	for _, c := range cases {
		if got := publicOrderLess(c.a, c.b, c.ka, c.kb); got != c.want {
			t.Errorf("%s: less = %v, want %v", c.name, got, c.want)
		}
		if c.want {
			if publicOrderLess(c.b, c.a, c.kb, c.ka) {
				t.Errorf("%s: the order is not antisymmetric", c.name)
			}
		}
	}
}

// Browse and Featured share one order, and filters keep it.
func TestCatalogBrowseAndFeaturedShareTheOrder(t *testing.T) {
	late := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	rows := []repository.AdvisorCardRow{
		card("off-star", 5.0, 900, "Tarot"),
		card("away-a", 4.4, 30, "Tarot"),
		card("busy-a", 4.9, 400, "Tarot"),
		card("online-low", 4.2, 30, "Tarot", "Love"),
		card("online-top", 4.8, 300, "Tarot", "Love"),
		card("online-pick", 4.8, 300, "Tarot"),
		card("online-rank2", 4.8, 300, "Tarot"),
	}
	// Equal rating and reviews: the admin's flag, rank, sort order break ties.
	rows[5].Featured, rows[5].FeaturedRank = true, 1
	rows[6].Featured, rows[6].FeaturedRank = true, 2
	rows[4].SortOrder = 7
	rows[4].LastOnlineAt = &late
	cat, _, _ := catalogWith(rows, map[string]domain.PresenceStatus{"away-a": domain.PresenceAway, "busy-a": domain.PresenceBusy,
		"online-low": domain.PresenceOnline, "online-top": domain.PresenceOnline, "online-pick": domain.PresenceOnline,
		"online-rank2": domain.PresenceOnline})
	want := []string{"online-pick", "online-rank2", "online-top", "online-low", "away-a", "busy-a", "off-star"}
	if got := orderSlugs(t, cat, BrowseQuery{}); !sameOrder(got, want) {
		t.Fatalf("browse order = %v, want %v", got, want)
	}
	f, err := cat.Featured(context.Background(), 6)
	if err != nil {
		t.Fatal(err)
	}
	for i, it := range f.Items {
		if it.Slug != want[i] {
			t.Fatalf("featured order differs from the directory at %d: %s, want %s", i, it.Slug, want[i])
		}
	}
	if got := orderSlugs(t, cat, BrowseQuery{Skill: "love"}); !sameOrder(got, []string{"online-top", "online-low"}) {
		t.Fatalf("skill filter lost the order: %v", got)
	}
	if got := orderSlugs(t, cat, BrowseQuery{Search: "online-"}); !sameOrder(got, want[:4]) {
		t.Fatalf("search filter lost the order: %v", got)
	}
	if got := orderSlugs(t, cat, BrowseQuery{Status: "online"}); !sameOrder(got, want[:4]) {
		t.Fatalf("status filter lost the order: %v", got)
	}
	// Paging cuts the ordered list, not the raw one.
	p, _ := cat.Browse(context.Background(), BrowseQuery{Page: 2, PageSize: 3})
	if got := slugs(p.Items); !sameOrder(got, want[3:6]) {
		t.Fatalf("page 2 = %v, want %v", got, want[3:6])
	}
}

// A status change moves the advisor between tiers at once, in both lists.
func TestCatalogOrderFollowsStatusPatches(t *testing.T) {
	rows := []repository.AdvisorCardRow{card("ada", 4.5, 10), card("bea", 4.5, 10), card("cyd", 4.5, 10)}
	cat, repo, clk := catalogWith(rows, map[string]domain.PresenceStatus{"ada": domain.PresenceOnline})
	if got := orderSlugs(t, cat, BrowseQuery{}); !sameOrder(got, []string{"ada", "bea", "cyd"}) {
		t.Fatalf("initial: %v", got)
	}
	for _, step := range []struct {
		who  int
		to   domain.PresenceStatus
		want []string
	}{
		{2, domain.PresenceBusy, []string{"ada", "cyd", "bea"}},
		{1, domain.PresenceAway, []string{"ada", "bea", "cyd"}},
		{0, domain.PresenceOffline, []string{"bea", "cyd", "ada"}},
		{2, domain.PresenceOnline, []string{"cyd", "bea", "ada"}},
	} {
		clk.Advance(time.Second)
		cat.SetStatus(rows[step.who].UserID, step.to)
		if got := orderSlugs(t, cat, BrowseQuery{}); !sameOrder(got, step.want) {
			t.Fatalf("%s -> %s: %v, want %v", rows[step.who].Slug, step.to, got, step.want)
		}
		f, _ := cat.Featured(context.Background(), 3)
		for i, it := range f.Items {
			if it.Slug != step.want[i] {
				t.Fatalf("featured diverged after %s -> %s: %v", rows[step.who].Slug, step.to, f.Items)
			}
		}
	}
	if n := repo.calls.Load(); n != 1 {
		t.Fatalf("status changes caused %d reads", n)
	}
}

// BenchmarkCatalogUnderStatusChurn reads the directory and the home selection
// between status patches of a 2,000-advisor catalogue: the worst case, as
// every patch drops the cached order.
func BenchmarkCatalogUnderStatusChurn(b *testing.B) {
	const n = 2000
	rows := make([]repository.AdvisorCardRow, n)
	for i := range rows {
		rows[i] = card(fmt.Sprintf("adv-%04d", i), 3+float64(i%20)/10, i%500, "Tarot")
		rows[i].SortOrder = i % 50
	}
	cat, _, _ := catalogWith(rows, nil)
	ctx := context.Background()
	statuses := []domain.PresenceStatus{domain.PresenceOnline, domain.PresenceAway, domain.PresenceBusy, domain.PresenceOffline}
	if _, err := cat.Browse(ctx, BrowseQuery{}); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cat.SetStatus(rows[i%n].UserID, statuses[i%len(statuses)])
		if _, err := cat.Browse(ctx, BrowseQuery{PageSize: 24}); err != nil {
			b.Fatal(err)
		}
		if _, err := cat.Featured(ctx, 3); err != nil {
			b.Fatal(err)
		}
	}
}
