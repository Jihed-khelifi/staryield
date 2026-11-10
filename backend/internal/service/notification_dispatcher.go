package service

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// DispatcherConfig sizes the async notification pipeline.
type DispatcherConfig struct {
	BufferSize    int
	BatchSize     int
	FlushInterval time.Duration
}

const (
	// syncWriteTimeout bounds the synchronous write Notify falls back to when
	// the queue is full.
	syncWriteTimeout = 5 * time.Second
	// insertAttempts is how many times a failed batch insert is tried before
	// the rows are written one by one.
	insertAttempts = 3
)

// insertBackoff is the wait before the second and third insert attempt.
var insertBackoff = []time.Duration{100 * time.Millisecond, time.Second}

// publishRetryDelay is the wait before the single retry of failed realtime
// publishes.
var publishRetryDelay = 100 * time.Millisecond

// Dispatcher persists and publishes notifications in the background. Notify
// normally only enqueues into a bounded channel; when that is full (or the
// dispatcher is stopping) it writes synchronously, bounded by 5 s, so a
// notification is never dropped silently. A failed batch insert is retried
// and then written row by row, so one bad row cannot take the others down.
type Dispatcher struct {
	repo   repository.NotificationRepository
	events EventPublisher
	log    *zap.Logger
	clock  clock.Clock
	cfg    DispatcherConfig
	ch     chan domain.Notification

	mu      sync.RWMutex // guards closed against enqueueing after the final drain
	closed  bool
	stop    chan struct{}
	stopCtx chan context.Context
	once    sync.Once
	wg      sync.WaitGroup

	written       atomic.Int64
	dropped       atomic.Int64
	syncFallbacks atomic.Int64
	insertRetries atomic.Int64
	rowFallbacks  atomic.Int64
	pubRetried    atomic.Int64
	pubFailed     atomic.Int64
	// processed counts notifications fully handled (persisted, published and
	// fanned out to e-mail and push).
	processed atomic.Int64
	channels  *NotificationChannels
}

// DispatcherStats are the dispatcher counters (since start).
type DispatcherStats struct {
	Written        int64 `json:"written"`
	SyncFallbacks  int64 `json:"sync_fallbacks"`
	InsertRetries  int64 `json:"insert_retries"`
	RowFallbacks   int64 `json:"row_fallbacks"`
	PublishRetried int64 `json:"publish_retried"`
	PublishFailed  int64 `json:"publish_failed"`
	// Dropped counts notifications that could not be written at all (the
	// database rejected the row, or the shutdown deadline passed first).
	Dropped  int64 `json:"dropped"`
	Buffered int   `json:"buffered"`
}

// Snapshot returns the counters.
func (d *Dispatcher) Snapshot() DispatcherStats {
	return DispatcherStats{Written: d.written.Load(), SyncFallbacks: d.syncFallbacks.Load(), InsertRetries: d.insertRetries.Load(),
		RowFallbacks: d.rowFallbacks.Load(), PublishRetried: d.pubRetried.Load(), PublishFailed: d.pubFailed.Load(),
		Dropped: d.dropped.Load(), Buffered: len(d.ch)}
}

// Processed returns how many notifications were fully handled (tests wait on
// it before inspecting e-mail and push).
func (d *Dispatcher) Processed() int64 { return d.processed.Load() }

// WithChannels fans persisted notifications out to e-mail and Web Push.
func (d *Dispatcher) WithChannels(c *NotificationChannels) *Dispatcher {
	d.channels = c
	return d
}

// NewDispatcher returns a dispatcher; call Start to begin delivery.
func NewDispatcher(repo repository.NotificationRepository, events EventPublisher, log *zap.Logger, clk clock.Clock, cfg DispatcherConfig) *Dispatcher {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 1000
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 250 * time.Millisecond
	}
	return &Dispatcher{repo: repo, events: events, log: log, clock: clk, cfg: cfg,
		ch: make(chan domain.Notification, cfg.BufferSize), stop: make(chan struct{}), stopCtx: make(chan context.Context, 1)}
}

// Notify implements Notifier. It never blocks in the normal case; with a full
// queue (or after Stop) the notification is written synchronously instead of
// being dropped.
func (d *Dispatcher) Notify(ctx context.Context, spec NotificationSpec, userIDs ...uuid.UUID) {
	if len(userIDs) == 0 || spec.Kind == "" {
		return
	}
	now := d.clock.Now()
	seen := make(map[uuid.UUID]bool, len(userIDs))
	for _, uid := range userIDs {
		if uid == uuid.Nil || seen[uid] {
			continue
		}
		seen[uid] = true
		n := domain.Notification{ID: domain.NewID(), UserID: uid, Kind: spec.Kind, Data: domain.JSONMap(copyMap(spec.Data)),
			Link: truncateString(spec.Link, 300), BroadcastID: spec.BroadcastID, CreatedAt: now}
		if !d.enqueue(n) {
			d.writeSync(n)
		}
	}
}

// enqueue adds n to the queue without blocking; false means the queue is full
// or closed.
func (d *Dispatcher) enqueue(n domain.Notification) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		return false
	}
	select {
	case d.ch <- n:
		return true
	default:
		return false
	}
}

// writeSync persists, publishes and fans out one notification in the
// caller's goroutine, bounded by syncWriteTimeout.
func (d *Dispatcher) writeSync(n domain.Notification) {
	if c := d.syncFallbacks.Add(1); c == 1 || c%1000 == 0 {
		d.log.Warn("notifications: queue full, writing synchronously", zap.Int64("sync_total", c), zap.String("kind", n.Kind))
	}
	ctx, cancel := context.WithTimeout(context.Background(), syncWriteTimeout)
	defer cancel()
	d.deliver(ctx, []domain.Notification{n})
}

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// Stats returns written and dropped counters.
func (d *Dispatcher) Stats() (written, dropped int64) { return d.written.Load(), d.dropped.Load() }

// Start launches the background writer.
func (d *Dispatcher) Start() {
	d.wg.Add(1)
	go d.run()
}

// Stop drains the queue within ctx (the shutdown deadline) and waits for the
// writer. Notifications still unwritten when ctx ends are counted as dropped.
func (d *Dispatcher) Stop(ctx context.Context) {
	d.mu.Lock()
	d.closed = true
	d.mu.Unlock()
	d.once.Do(func() {
		d.stopCtx <- ctx
		close(d.stop)
	})
	done := make(chan struct{})
	go func() { d.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		d.log.Warn("notifications: shutdown before drain completed", zap.Int("buffered", len(d.ch)))
	}
}

func (d *Dispatcher) run() {
	defer d.wg.Done()
	ticker := time.NewTicker(d.cfg.FlushInterval)
	defer ticker.Stop()
	batch := make([]domain.Notification, 0, d.cfg.BatchSize)
	flush := func(parent context.Context) {
		if len(batch) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(parent, 15*time.Second)
		d.deliver(ctx, batch)
		cancel()
		batch = batch[:0]
	}
	for {
		select {
		case n := <-d.ch:
			batch = append(batch, n)
			if len(batch) >= d.cfg.BatchSize {
				flush(context.Background())
			}
		case <-ticker.C:
			flush(context.Background())
		case <-d.stop:
			drain := context.Background()
			select {
			case c := <-d.stopCtx:
				drain = c
			default:
			}
			for {
				select {
				case n := <-d.ch:
					batch = append(batch, n)
					if len(batch) >= d.cfg.BatchSize {
						flush(drain)
					}
					if drain.Err() != nil {
						d.abandon(len(batch) + len(d.ch))
						return
					}
					continue
				default:
				}
				break
			}
			if drain.Err() != nil {
				d.abandon(len(batch) + len(d.ch))
				return
			}
			flush(drain)
			return
		}
	}
}

// abandon counts notifications lost to the shutdown deadline.
func (d *Dispatcher) abandon(n int) {
	if n > 0 {
		d.dropped.Add(int64(n))
		d.log.Error("notifications: shutdown deadline reached, notifications lost", zap.Int("count", n))
	}
}

// sleepCtx waits d or until ctx ends; false means ctx ended.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// persist writes a batch: three attempts with backoff, then row by row so one
// poisoned row cannot take the others down. It returns the rows written.
func (d *Dispatcher) persist(ctx context.Context, batch []domain.Notification) []domain.Notification {
	rows := append([]domain.Notification(nil), batch...)
	var err error
	for attempt := 0; attempt < insertAttempts; attempt++ {
		if err = d.repo.InsertBatch(ctx, rows); err == nil {
			d.written.Add(int64(len(rows)))
			return rows
		}
		d.log.Warn("notifications: insert failed", zap.Error(err), zap.Int("count", len(rows)), zap.Int("attempt", attempt+1))
		if attempt+1 < insertAttempts {
			d.insertRetries.Add(1)
			if !sleepCtx(ctx, insertBackoff[min(attempt, len(insertBackoff)-1)]) {
				break
			}
		}
	}
	if len(rows) == 1 {
		d.dropRow(rows[0], err)
		return nil
	}
	d.rowFallbacks.Add(1)
	written := rows[:0:0]
	for i := range rows {
		if rerr := d.repo.InsertBatch(ctx, rows[i:i+1]); rerr != nil {
			d.dropRow(rows[i], rerr)
			continue
		}
		written = append(written, rows[i])
	}
	d.written.Add(int64(len(written)))
	return written
}

func (d *Dispatcher) dropRow(n domain.Notification, err error) {
	d.dropped.Add(1)
	d.log.Error("notifications: row could not be written, dropped", zap.Error(err), zap.String("id", n.ID.String()),
		zap.String("kind", n.Kind), zap.String("user_id", n.UserID.String()))
}

// manyPublisher is implemented by realtime.Publisher: pipelined publishing
// of many per-user events.
type manyPublisher interface {
	PublishMany(ctx context.Context, eventType string, events []realtime.UserEvent) ([]int, error)
}

// publishNotifications publishes notification.created for rows[idx] (all rows
// when idx is nil) and returns the indexes that failed.
func publishNotifications(ctx context.Context, events EventPublisher, rows []domain.Notification, idx []int) []int {
	if idx == nil {
		idx = make([]int, len(rows))
		for i := range idx {
			idx[i] = i
		}
	}
	if mp, ok := events.(manyPublisher); ok {
		evs := make([]realtime.UserEvent, len(idx))
		for k, i := range idx {
			evs[k] = realtime.UserEvent{UserID: rows[i].UserID, Data: map[string]any{"notification": notificationView(&rows[i])}}
		}
		failed, _ := mp.PublishMany(ctx, EventNotificationCreated, evs)
		out := make([]int, len(failed))
		for k, f := range failed {
			out[k] = idx[f]
		}
		return out
	}
	var failed []int
	for _, i := range idx {
		v := notificationView(&rows[i])
		if err := events.Publish(ctx, EventNotificationCreated, map[string]any{"notification": v}, rows[i].UserID); err != nil {
			failed = append(failed, i)
		}
	}
	return failed
}

// publish sends notification.created for written rows; the failed ones are
// retried once.
func (d *Dispatcher) publish(ctx context.Context, rows []domain.Notification) {
	failed := publishNotifications(ctx, d.events, rows, nil)
	if len(failed) == 0 {
		return
	}
	d.pubRetried.Add(int64(len(failed)))
	sleepCtx(ctx, publishRetryDelay)
	if failed = publishNotifications(ctx, d.events, rows, failed); len(failed) > 0 {
		d.pubFailed.Add(int64(len(failed)))
		d.log.Warn("notifications: realtime publish failed after retry (clients resync on reconnect)", zap.Int("count", len(failed)))
	}
}

// deliver persists a batch, publishes it and fans it out to e-mail and push.
func (d *Dispatcher) deliver(ctx context.Context, batch []domain.Notification) {
	if rows := d.persist(ctx, batch); len(rows) > 0 {
		d.publish(ctx, rows)
		d.channels.Deliver(ctx, rows)
	}
	d.processed.Add(int64(len(batch)))
}
