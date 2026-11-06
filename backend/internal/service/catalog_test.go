package service

import (
	"context"
	"errors"
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
	want := []string{"steady", "lucky", "busy", "away", "off-new", "off-old"}
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
