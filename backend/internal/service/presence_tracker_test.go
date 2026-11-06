package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

type fakePresenceRepo struct {
	mu         sync.Mutex
	intervals  []domain.PresenceInterval
	lastOnline map[uuid.UUID]time.Time
	avail      map[uuid.UUID]domain.Availability
	failStart  int // Start returns a unique violation this many times
}

func newFakePresenceRepo() *fakePresenceRepo {
	return &fakePresenceRepo{lastOnline: map[uuid.UUID]time.Time{}, avail: map[uuid.UUID]domain.Availability{}}
}

func (f *fakePresenceRepo) OpenForUpdate(_ context.Context, id uuid.UUID) (*domain.PresenceInterval, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.intervals {
		if f.intervals[i].PsychicID == id && f.intervals[i].EndedAt == nil {
			iv := f.intervals[i]
			return &iv, nil
		}
	}
	return nil, nil
}

func (f *fakePresenceRepo) OpenIntervals(context.Context, int) ([]domain.PresenceInterval, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.PresenceInterval
	for _, iv := range f.intervals {
		if iv.EndedAt == nil {
			out = append(out, iv)
		}
	}
	return out, nil
}

func (f *fakePresenceRepo) Start(_ context.Context, iv *domain.PresenceInterval) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failStart > 0 {
		f.failStart--
		return &pgconn.PgError{Code: "23505", ConstraintName: "psychic_presence_intervals_open_idx"}
	}
	f.intervals = append(f.intervals, *iv)
	return nil
}

func (f *fakePresenceRepo) Close(_ context.Context, id uuid.UUID, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.intervals {
		if f.intervals[i].ID == id && f.intervals[i].EndedAt == nil {
			end := at
			if end.Before(f.intervals[i].StartedAt) {
				end = f.intervals[i].StartedAt
			}
			f.intervals[i].EndedAt = &end
		}
	}
	return nil
}

func (f *fakePresenceRepo) TouchLastOnline(_ context.Context, id uuid.UUID, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if cur, ok := f.lastOnline[id]; !ok || at.After(cur) {
		f.lastOnline[id] = at
	}
	return nil
}

func (f *fakePresenceRepo) AvailabilityOf(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.Availability, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[uuid.UUID]domain.Availability{}
	for _, id := range ids {
		if av, ok := f.avail[id]; ok {
			out[id] = av
		}
	}
	return out, nil
}

func (f *fakePresenceRepo) snapshot() []domain.PresenceInterval {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.PresenceInterval(nil), f.intervals...)
}

// statusFromAvailability mirrors the live lookup for advisors with no
// session: the chosen availability is the status.
func statusFromAvailability(_ context.Context, list []domain.Psychic) (map[uuid.UUID]domain.PresenceStatus, error) {
	out := map[uuid.UUID]domain.PresenceStatus{}
	for _, p := range list {
		switch p.Availability {
		case domain.AvailabilityOnline:
			out[p.UserID] = domain.PresenceOnline
		case domain.AvailabilityAway:
			out[p.UserID] = domain.PresenceAway
		default:
			out[p.UserID] = domain.PresenceOffline
		}
	}
	return out, nil
}

func TestPresenceTrackerRecordsTransitions(t *testing.T) {
	repo := newFakePresenceRepo()
	clk := clock.NewFake(time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC))
	changes := 0
	tr := NewPresenceTracker(inlineTx{}, repo, statusFromAvailability, clk, func() { changes++ })
	ctx := context.Background()
	id := uuid.New()

	tr.Observe(ctx, id, domain.PresenceOnline)
	clk.Advance(30 * time.Minute)
	tr.Observe(ctx, id, domain.PresenceOnline) // repeated publish: no new interval
	clk.Advance(30 * time.Minute)
	tr.Observe(ctx, id, domain.PresenceBusy)
	clk.Advance(20 * time.Minute)
	tr.Observe(ctx, id, domain.PresenceAway)
	clk.Advance(10 * time.Minute)
	tr.Observe(ctx, id, domain.PresenceOffline)
	tr.Observe(ctx, id, domain.PresenceOffline) // already closed

	ivs := repo.snapshot()
	if len(ivs) != 3 {
		t.Fatalf("intervals = %d, want 3: %+v", len(ivs), ivs)
	}
	wantStatus := []domain.PresenceStatus{domain.PresenceOnline, domain.PresenceBusy, domain.PresenceAway}
	wantMinutes := []float64{60, 20, 10}
	for i, iv := range ivs {
		if iv.Status != wantStatus[i] || iv.EndedAt == nil || iv.EndedAt.Sub(iv.StartedAt).Minutes() != wantMinutes[i] {
			t.Errorf("interval %d = %s %v..%v", i, iv.Status, iv.StartedAt, iv.EndedAt)
		}
	}
	// Last online is the moment the busy interval ended (away does not count).
	if got, want := repo.lastOnline[id], time.Date(2026, 11, 5, 10, 20, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("last_online_at = %v, want %v", got, want)
	}
	if changes != 4 {
		t.Errorf("onChange ran %d times, want 4 (online, busy, away, offline)", changes)
	}
}

func TestPresenceTrackerRetriesAfterConcurrentOpen(t *testing.T) {
	repo := newFakePresenceRepo()
	repo.failStart = 1
	tr := NewPresenceTracker(inlineTx{}, repo, statusFromAvailability, clock.NewFake(time.Now()), nil)
	id := uuid.New()
	tr.Observe(context.Background(), id, domain.PresenceOnline)
	if ivs := repo.snapshot(); len(ivs) != 1 || ivs[0].Status != domain.PresenceOnline {
		t.Fatalf("retry did not open the interval: %+v", ivs)
	}
}

func TestPresenceTrackerReconcileClosesStaleIntervals(t *testing.T) {
	repo := newFakePresenceRepo()
	clk := clock.NewFake(time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC))
	tr := NewPresenceTracker(inlineTx{}, repo, statusFromAvailability, clk, nil)
	ctx := context.Background()
	stillOnline, wentAway, crashed, deleted := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{stillOnline, wentAway, crashed, deleted} {
		tr.Observe(ctx, id, domain.PresenceOnline)
	}
	repo.avail[stillOnline] = domain.AvailabilityOnline
	repo.avail[wentAway] = domain.AvailabilityAway
	repo.avail[crashed] = domain.AvailabilityOffline
	// deleted has no row at all.
	clk.Advance(time.Hour)
	if err := tr.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	open := map[uuid.UUID]domain.PresenceStatus{}
	closed := map[uuid.UUID]int{}
	for _, iv := range repo.snapshot() {
		if iv.EndedAt == nil {
			open[iv.PsychicID] = iv.Status
		} else {
			closed[iv.PsychicID]++
		}
	}
	if open[stillOnline] != domain.PresenceOnline || closed[stillOnline] != 0 {
		t.Errorf("an advisor still online was touched: open=%v closed=%d", open[stillOnline], closed[stillOnline])
	}
	if open[wentAway] != domain.PresenceAway || closed[wentAway] != 1 {
		t.Errorf("away advisor: open=%v closed=%d", open[wentAway], closed[wentAway])
	}
	for _, id := range []uuid.UUID{crashed, deleted} {
		if _, ok := open[id]; ok || closed[id] != 1 {
			t.Errorf("advisor %s left open", id)
		}
	}
	if err := tr.Reconcile(ctx); err != nil { // idempotent
		t.Fatal(err)
	}
	if n := len(repo.snapshot()); n != 5 {
		t.Errorf("second reconcile changed history: %d intervals", n)
	}
}
