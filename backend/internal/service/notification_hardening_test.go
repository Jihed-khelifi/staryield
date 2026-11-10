package service

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

func fastBackoff(t *testing.T) {
	t.Helper()
	oldInsert, oldPub := insertBackoff, publishRetryDelay
	insertBackoff, publishRetryDelay = []time.Duration{time.Millisecond, time.Millisecond}, time.Millisecond
	t.Cleanup(func() { insertBackoff, publishRetryDelay = oldInsert, oldPub })
}

// flakyRepo is a notification writer with injectable slowness and failures.
type flakyRepo struct {
	repository.NotificationRepository
	mu       sync.Mutex
	rows     map[uuid.UUID]domain.Notification
	calls    int
	delay    time.Duration
	failFor  int                // the first n InsertBatch calls fail
	poisoned map[uuid.UUID]bool // any batch containing these users fails
}

func newFlakyRepo() *flakyRepo { return &flakyRepo{rows: map[uuid.UUID]domain.Notification{}} }

func (f *flakyRepo) InsertBatch(_ context.Context, rows []domain.Notification) error {
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.calls <= f.failFor {
		return errors.New("db down")
	}
	for _, r := range rows {
		if f.poisoned[r.UserID] {
			return errors.New("foreign key violation")
		}
	}
	for _, r := range rows {
		f.rows[r.ID] = r // ON CONFLICT DO NOTHING semantics: same id = same row
	}
	return nil
}

func (f *flakyRepo) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.rows)
}

func stopDispatcher(t *testing.T, d *Dispatcher) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	d.Stop(ctx)
}

// A full queue and a slow writer lose nothing: Notify writes synchronously.
func TestDispatcherFullQueueLosesNothing(t *testing.T) {
	repo := newFlakyRepo()
	repo.delay = 5 * time.Millisecond
	d := NewDispatcher(repo, &recordingPublisher{}, zap.NewNop(), clock.Real{}, DispatcherConfig{BufferSize: 2, BatchSize: 1, FlushInterval: time.Hour})
	d.Start()
	const total = 60
	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.Notify(context.Background(), NotificationSpec{Kind: NotifyPurchase}, uuid.New())
		}()
	}
	wg.Wait()
	stopDispatcher(t, d)
	st := d.Snapshot()
	if repo.count() != total || st.Dropped != 0 || st.Written != total {
		t.Fatalf("written %d (stats %+v), want %d and no drops", repo.count(), st, total)
	}
	if st.SyncFallbacks == 0 {
		t.Fatal("expected synchronous fallbacks with a 2-slot queue and a slow writer")
	}
	if d.Processed() != total {
		t.Fatalf("processed %d", d.Processed())
	}
}

// Notify stays non-blocking while the queue has room, however slow the writer.
func TestDispatcherNotifyDoesNotBlockWhileQueueHasRoom(t *testing.T) {
	repo := newFlakyRepo()
	repo.delay = 50 * time.Millisecond
	d := NewDispatcher(repo, &recordingPublisher{}, zap.NewNop(), clock.Real{}, DispatcherConfig{BufferSize: 100, BatchSize: 50, FlushInterval: time.Hour})
	d.Start()
	start := time.Now()
	for i := 0; i < 40; i++ {
		d.Notify(context.Background(), NotificationSpec{Kind: NotifyPurchase}, uuid.New())
	}
	if time.Since(start) > 40*time.Millisecond {
		t.Fatalf("Notify took %v with room in the queue", time.Since(start))
	}
	stopDispatcher(t, d)
	if repo.count() != 40 || d.Snapshot().SyncFallbacks != 0 {
		t.Fatalf("written %d, stats %+v", repo.count(), d.Snapshot())
	}
}

func TestDispatcherSyncFallbackIsBoundedByFiveSeconds(t *testing.T) {
	if syncWriteTimeout != 5*time.Second {
		t.Fatalf("sync write bound: %v", syncWriteTimeout)
	}
}

func notifs(n int, poison ...uuid.UUID) []domain.Notification {
	out := make([]domain.Notification, 0, n)
	for i := 0; i < n; i++ {
		uid := uuid.New()
		if i < len(poison) {
			uid = poison[i]
		}
		out = append(out, domain.Notification{ID: domain.NewID(), UserID: uid, Kind: NotifyPurchase, Data: domain.JSONMap{}})
	}
	return out
}

// A transient database error is retried (3 attempts) and the batch survives.
func TestDispatcherRetriesFailedBatchInserts(t *testing.T) {
	fastBackoff(t)
	repo := newFlakyRepo()
	repo.failFor = 2
	d := NewDispatcher(repo, NoopPublisher{}, zap.NewNop(), clock.Real{}, DispatcherConfig{})
	d.deliver(context.Background(), notifs(20))
	st := d.Snapshot()
	if repo.count() != 20 || st.InsertRetries != 2 || st.RowFallbacks != 0 || st.Dropped != 0 {
		t.Fatalf("count %d stats %+v", repo.count(), st)
	}
}

// After three failed attempts the rows are written one by one: one poisoned
// row is dropped, the other 199 survive.
func TestDispatcherFallsBackRowByRowAndOnePoisonedRowLosesOnlyItself(t *testing.T) {
	fastBackoff(t)
	repo := newFlakyRepo()
	bad := uuid.New()
	repo.poisoned = map[uuid.UUID]bool{bad: true}
	d := NewDispatcher(repo, &recordingPublisher{}, zap.NewNop(), clock.Real{}, DispatcherConfig{})
	batch := notifs(200, bad)
	d.deliver(context.Background(), batch)
	st := d.Snapshot()
	if repo.count() != 199 || st.Dropped != 1 || st.RowFallbacks != 1 || st.Written != 199 {
		t.Fatalf("count %d stats %+v", repo.count(), st)
	}
	for _, r := range repo.rows {
		if r.UserID == bad {
			t.Fatal("poisoned row stored")
		}
	}
	if d.Processed() != 200 {
		t.Fatalf("processed %d", d.Processed())
	}
}

func TestDispatcherDrainsTheQueueOnStopAndWritesSyncAfterwards(t *testing.T) {
	repo := newFlakyRepo()
	d := NewDispatcher(repo, NoopPublisher{}, zap.NewNop(), clock.Real{}, DispatcherConfig{BufferSize: 100, BatchSize: 1000, FlushInterval: time.Hour})
	d.Start()
	for i := 0; i < 30; i++ {
		d.Notify(context.Background(), NotificationSpec{Kind: NotifyPurchase}, uuid.New())
	}
	stopDispatcher(t, d) // nothing flushed yet: only the drain writes them
	if repo.count() != 30 {
		t.Fatalf("drained %d of 30", repo.count())
	}
	// After Stop nothing is queued into a channel nobody drains.
	d.Notify(context.Background(), NotificationSpec{Kind: NotifyPurchase}, uuid.New())
	if repo.count() != 31 || d.Snapshot().SyncFallbacks != 1 {
		t.Fatalf("notify after stop: %d rows, %+v", repo.count(), d.Snapshot())
	}
}

func TestDispatcherCountsWhatTheShutdownDeadlineCutsOff(t *testing.T) {
	repo := newFlakyRepo()
	d := NewDispatcher(repo, NoopPublisher{}, zap.NewNop(), clock.Real{}, DispatcherConfig{BufferSize: 100, BatchSize: 1000, FlushInterval: time.Hour})
	d.Start()
	for i := 0; i < 10; i++ {
		d.Notify(context.Background(), NotificationSpec{Kind: NotifyPurchase}, uuid.New())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d.Stop(ctx)
	deadline := time.Now().Add(2 * time.Second)
	for d.Snapshot().Dropped != 10 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if st := d.Snapshot(); st.Dropped != 10 {
		t.Fatalf("stats %+v: the loss must be counted, not silent", st)
	}
}

// failOncePublisher fails the first publish to each user.
type failOncePublisher struct {
	NoopPublisher
	mu     sync.Mutex
	seen   map[uuid.UUID]int
	failed int
}

func (p *failOncePublisher) Publish(_ context.Context, _ string, _ any, ids ...uuid.UUID) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.seen == nil {
		p.seen = map[uuid.UUID]int{}
	}
	var err error
	for _, id := range ids {
		p.seen[id]++
		if p.seen[id] == 1 {
			p.failed++
			err = errors.New("redis down")
		}
	}
	return err
}

func TestDispatcherRetriesFailedPublishOnce(t *testing.T) {
	fastBackoff(t)
	pub := &failOncePublisher{}
	d := NewDispatcher(newFlakyRepo(), pub, zap.NewNop(), clock.Real{}, DispatcherConfig{})
	d.deliver(context.Background(), notifs(5))
	st := d.Snapshot()
	if st.PublishRetried != 5 || st.PublishFailed != 0 {
		t.Fatalf("stats %+v", st)
	}
	for id, n := range pub.seen {
		if n != 2 {
			t.Fatalf("user %s published %d times, want 2 (one retry)", id, n)
		}
	}
}

// --- broadcasts -------------------------------------------------------------

// fakeBroadcastRepo implements the broadcast part of NotificationRepository
// in memory with the semantics of the SQL implementation: users walked in
// uuid byte order, idempotent chunk insert with cursor and recipients in one
// step, advisory lock per broadcast.
type fakeBroadcastRepo struct {
	repository.NotificationRepository
	mu     sync.Mutex
	users  []uuid.UUID // sorted
	b      domain.Broadcast
	rows   map[[2]uuid.UUID]uuid.UUID // (broadcast, user) -> notification id
	locked bool
}

func newFakeBroadcastRepo(n int) *fakeBroadcastRepo {
	f := &fakeBroadcastRepo{rows: map[[2]uuid.UUID]uuid.UUID{}}
	for i := 0; i < n; i++ {
		f.users = append(f.users, uuid.New())
	}
	sort.Slice(f.users, func(i, j int) bool { return bytes.Compare(f.users[i][:], f.users[j][:]) < 0 })
	hb := time.Now()
	f.b = domain.Broadcast{ID: domain.NewID(), Audience: "clients", Title: domain.JSONMap{"en": "t"}, Body: domain.JSONMap{"en": "b"},
		Status: "queued", CreatedAt: hb, HeartbeatAt: &hb}
	return f
}

func (f *fakeBroadcastRepo) GetBroadcast(context.Context, uuid.UUID) (*domain.Broadcast, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b := f.b
	return &b, nil
}

func (f *fakeBroadcastRepo) UpdateBroadcast(_ context.Context, _ uuid.UUID, fields map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for k, v := range fields {
		switch k {
		case "status":
			f.b.Status = v.(string)
		case "recipients":
			f.b.Recipients = v.(int)
		case "heartbeat_at":
			t := v.(time.Time)
			f.b.HeartbeatAt = &t
		case "sent_at":
			t := v.(time.Time)
			f.b.SentAt = &t
		}
	}
	return nil
}

func (f *fakeBroadcastRepo) AudienceChunk(_ context.Context, _ []domain.Role, candidates []uuid.UUID, after *uuid.UUID, _ *uuid.UUID, limit int) ([]uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []uuid.UUID
	if candidates != nil {
		return append(out, candidates...), nil
	}
	for _, u := range f.users {
		if after != nil && bytes.Compare(u[:], after[:]) <= 0 {
			continue
		}
		out = append(out, u)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (f *fakeBroadcastRepo) InsertBroadcastChunk(_ context.Context, id uuid.UUID, rows []domain.Notification, cursor uuid.UUID, now time.Time) ([]uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var inserted []uuid.UUID
	for _, r := range rows {
		k := [2]uuid.UUID{id, r.UserID}
		if _, dup := f.rows[k]; dup {
			continue
		}
		f.rows[k] = r.ID
		inserted = append(inserted, r.ID)
	}
	f.b.CursorUserID, f.b.HeartbeatAt, f.b.Status = &cursor, &now, "sending"
	f.b.Recipients += len(inserted)
	return inserted, nil
}

func (f *fakeBroadcastRepo) CountBroadcastRecipients(_ context.Context, id uuid.UUID) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for k := range f.rows {
		if k[0] == id {
			n++
		}
	}
	return n, nil
}

func (f *fakeBroadcastRepo) StaleBroadcasts(_ context.Context, before time.Time, _ int) ([]domain.Broadcast, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if (f.b.Status == "queued" || f.b.Status == "sending") && f.b.HeartbeatAt.Before(before) {
		return []domain.Broadcast{f.b}, nil
	}
	return nil, nil
}

func (f *fakeBroadcastRepo) WithBroadcastLock(ctx context.Context, _ uuid.UUID, fn func(context.Context) error) (bool, error) {
	f.mu.Lock()
	if f.locked {
		f.mu.Unlock()
		return false, nil
	}
	f.locked = true
	f.mu.Unlock()
	defer func() { f.mu.Lock(); f.locked = false; f.mu.Unlock() }()
	return true, fn(ctx)
}

func (f *fakeBroadcastRepo) age() {
	f.mu.Lock()
	defer f.mu.Unlock()
	old := time.Now().Add(-10 * time.Minute)
	f.b.HeartbeatAt = &old
}

func newBroadcastService(repo *fakeBroadcastRepo, pub EventPublisher) *NotificationService {
	return NewNotificationService(repo, nil, nil, pub, nil, clock.Real{}, "secret", zap.NewNop())
}

// An interrupted fan-out is resumed by the recovery job: every recipient
// ends up with exactly one notification, one realtime event, and the final
// recipients count is exact.
func TestBroadcastResumesAfterAnInterruptedRunWithoutDuplicates(t *testing.T) {
	repo := newFakeBroadcastRepo(2500)
	pub := &countingPublisher{}
	s := newBroadcastService(repo, pub)

	crash := errors.New("process killed")
	s.chunkHook = func(chunk int) error {
		if chunk == 1 { // two chunks done (2 x 1000), then the "crash"
			return crash
		}
		return nil
	}
	s.runBroadcast(context.Background(), repo.b.ID)
	if repo.b.Status != "sending" || repo.b.Recipients != 2000 || repo.b.CursorUserID == nil || *repo.b.CursorUserID != repo.users[1999] {
		t.Fatalf("after the crash: status=%s recipients=%d cursor=%v", repo.b.Status, repo.b.Recipients, repo.b.CursorUserID)
	}

	// A fresh heartbeat is left alone; a stale one is resumed.
	if err := s.RecoverBroadcasts(context.Background()); err != nil || repo.b.Status != "sending" {
		t.Fatalf("recovery touched a broadcast with a fresh heartbeat: %v %s", err, repo.b.Status)
	}
	s.chunkHook = nil
	repo.age()
	if err := s.RecoverBroadcasts(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repo.b.Status != "sent" || repo.b.Recipients != 2500 || repo.b.SentAt == nil {
		t.Fatalf("after recovery: status=%s recipients=%d", repo.b.Status, repo.b.Recipients)
	}
	if len(repo.rows) != 2500 || pub.total() != 2500 || pub.maxPerUser() != 1 {
		t.Fatalf("rows %d, events %d, max events per user %d", len(repo.rows), pub.total(), pub.maxPerUser())
	}
	if got := s.BroadcastSnapshot(); got.Resumed != 1 || got.Completed != 1 {
		t.Fatalf("stats %+v", got)
	}

	// Idempotent re-run: even forced back to sending from the start, nothing
	// is inserted or published twice and recipients stays exact.
	repo.b.Status, repo.b.CursorUserID = "sending", nil
	repo.age()
	if err := s.RecoverBroadcasts(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repo.b.Status != "sent" || repo.b.Recipients != 2500 || len(repo.rows) != 2500 || pub.total() != 2500 {
		t.Fatalf("re-run: status=%s recipients=%d rows=%d events=%d", repo.b.Status, repo.b.Recipients, len(repo.rows), pub.total())
	}
}

// Two runners (two instances) never work on one broadcast at once.
func TestBroadcastHasOneRunnerAtATime(t *testing.T) {
	repo := newFakeBroadcastRepo(1500)
	s := newBroadcastService(repo, NoopPublisher{})
	entered, release := make(chan struct{}), make(chan struct{})
	s.chunkHook = func(chunk int) error {
		if chunk == 0 {
			close(entered)
			<-release
		}
		return nil
	}
	done := make(chan struct{})
	go func() { s.runBroadcast(context.Background(), repo.b.ID); close(done) }()
	<-entered
	s.chunkHook = nil
	repo.age() // looks stale, but the first runner holds the lock
	if err := s.RecoverBroadcasts(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.rows) != 1000 {
		t.Fatalf("a second runner worked in parallel: %d rows", len(repo.rows))
	}
	close(release)
	<-done
	if repo.b.Status != "sent" || repo.b.Recipients != 1500 || len(repo.rows) != 1500 {
		t.Fatalf("status=%s recipients=%d rows=%d", repo.b.Status, repo.b.Recipients, len(repo.rows))
	}
}

func TestBroadcastRecoveryGivesUpOnAncientBroadcasts(t *testing.T) {
	repo := newFakeBroadcastRepo(10)
	repo.b.CreatedAt = time.Now().Add(-48 * time.Hour)
	repo.age()
	s := newBroadcastService(repo, NoopPublisher{})
	if err := s.RecoverBroadcasts(context.Background()); err != nil || repo.b.Status != "failed" {
		t.Fatalf("status %s err %v", repo.b.Status, err)
	}
}

func TestBroadcastShutdownIsBoundedByTheDeadline(t *testing.T) {
	repo := newFakeBroadcastRepo(3000)
	s := newBroadcastService(repo, NoopPublisher{})
	started, release := make(chan struct{}), make(chan struct{})
	s.chunkHook = func(chunk int) error {
		if chunk == 0 {
			close(started)
			select {
			case <-release:
			case <-s.runCtx.Done():
			}
		}
		return nil
	}
	s.wg.Add(1)
	go func() { defer s.wg.Done(); s.runBroadcast(context.Background(), repo.b.ID) }()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	s.Shutdown(ctx)
	if time.Since(start) > 3*time.Second {
		t.Fatalf("shutdown took %v", time.Since(start))
	}
	if repo.b.Status != "sending" {
		t.Fatalf("an interrupted broadcast must stay resumable, status %s", repo.b.Status)
	}
	// It resumes later.
	s2 := newBroadcastService(repo, NoopPublisher{})
	repo.age()
	if err := s2.RecoverBroadcasts(context.Background()); err != nil || repo.b.Status != "sent" || repo.b.Recipients != 3000 {
		t.Fatalf("resume: %v status=%s recipients=%d", err, repo.b.Status, repo.b.Recipients)
	}
}

// countingPublisher counts notification.created events per user, and
// implements the pipelined publisher like realtime.Publisher.
type countingPublisher struct {
	NoopPublisher
	mu    sync.Mutex
	users map[uuid.UUID]int
	n     atomic.Int64
}

func (p *countingPublisher) add(id uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.users == nil {
		p.users = map[uuid.UUID]int{}
	}
	p.users[id]++
	p.n.Add(1)
}

func (p *countingPublisher) Publish(_ context.Context, _ string, _ any, ids ...uuid.UUID) error {
	for _, id := range ids {
		p.add(id)
	}
	return nil
}

func (p *countingPublisher) PublishMany(_ context.Context, _ string, evs []realtime.UserEvent) ([]int, error) {
	for _, e := range evs {
		p.add(e.UserID)
	}
	return nil, nil
}

func (p *countingPublisher) total() int { return int(p.n.Load()) }

func (p *countingPublisher) maxPerUser() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	m := 0
	for _, n := range p.users {
		m = max(m, n)
	}
	return m
}

// Mark-read publishes the ephemeral notification.read event.
type readRepo struct {
	repository.NotificationRepository
	unread int64
}

func (r *readRepo) MarkRead(context.Context, uuid.UUID, []uuid.UUID, time.Time) error { return nil }
func (r *readRepo) MarkAllRead(context.Context, uuid.UUID, time.Time) error           { return nil }
func (r *readRepo) CountUnread(context.Context, uuid.UUID) (int64, error)             { return r.unread, nil }

type ephemeralRecorder struct {
	NoopPublisher
	mu     sync.Mutex
	events []map[string]any
	typ    []string
	to     []uuid.UUID
}

func (p *ephemeralRecorder) Ephemeral(_ context.Context, typ string, data any, ids ...uuid.UUID) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, data.(map[string]any))
	p.typ = append(p.typ, typ)
	p.to = append(p.to, ids...)
	return nil
}

func TestMarkReadPublishesNotificationReadEvent(t *testing.T) {
	rec := &ephemeralRecorder{}
	s := NewNotificationService(&readRepo{unread: 3}, nil, nil, rec, nil, clock.Real{}, "s", zap.NewNop())
	user, a, b := uuid.New(), uuid.New(), uuid.New()
	if n, err := s.MarkRead(context.Background(), user, []uuid.UUID{a, b}, false); err != nil || n != 3 {
		t.Fatalf("mark: %d %v", n, err)
	}
	if _, err := s.MarkRead(context.Background(), user, nil, true); err != nil {
		t.Fatal(err)
	}
	if len(rec.events) != 2 || rec.typ[0] != EventNotificationRead || rec.to[0] != user {
		t.Fatalf("events: %+v %v", rec.events, rec.typ)
	}
	ids, _ := rec.events[0]["ids"].([]string)
	if len(ids) != 2 || ids[0] != a.String() || rec.events[0]["unread"] != int64(3) || rec.events[0]["all"] != nil {
		t.Fatalf("ids event: %+v", rec.events[0])
	}
	if rec.events[1]["all"] != true || rec.events[1]["ids"] != nil || rec.events[1]["unread"] != int64(3) {
		t.Fatalf("all event: %+v", rec.events[1])
	}
}
