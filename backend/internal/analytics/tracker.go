// Package analytics records business events for the admin dashboard.
//
// Events are enqueued without blocking request handling and written to
// PostgreSQL in batches by a background worker. When the buffer is full,
// events are dropped (and counted) rather than slowing user traffic.
// Event properties never contain message content, passwords or raw IPs; IP
// addresses are stored as keyed hashes.
package analytics

import (
	"context"
	"regexp"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// Store persists event batches.
type Store interface {
	InsertBatch(ctx context.Context, events []domain.AnalyticsEvent) (int64, error)
}

// Tracker records events.
type Tracker interface {
	// Track records a server-side event, enriched from ctx (principal and
	// request metadata).
	Track(ctx context.Context, name string, props map[string]any)
	// Enqueue records a fully-formed event (client ingestion).
	Enqueue(e domain.AnalyticsEvent) bool
}

// Config tunes the async writer.
type Config struct {
	Enabled       bool
	BufferSize    int
	BatchSize     int
	FlushInterval time.Duration
	IPHashSecret  string
}

// AsyncTracker buffers and batch-writes events.
type AsyncTracker struct {
	cfg     Config
	store   Store
	log     *zap.Logger
	now     func() time.Time
	ch      chan domain.AnalyticsEvent
	dropped atomic.Int64
	written atomic.Int64
	wg      sync.WaitGroup
	once    sync.Once
	stop    chan struct{}
}

// NewAsyncTracker returns a tracker; call Start to begin writing.
func NewAsyncTracker(cfg Config, store Store, log *zap.Logger, now func() time.Time) *AsyncTracker {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 1000
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 2 * time.Second
	}
	if now == nil {
		now = time.Now
	}
	return &AsyncTracker{cfg: cfg, store: store, log: log, now: now,
		ch: make(chan domain.AnalyticsEvent, cfg.BufferSize), stop: make(chan struct{})}
}

// Start launches the background writer.
func (t *AsyncTracker) Start() {
	t.wg.Add(1)
	go t.run()
}

// Stop flushes buffered events and stops the writer, waiting until ctx ends.
func (t *AsyncTracker) Stop(ctx context.Context) {
	t.once.Do(func() { close(t.stop) })
	done := make(chan struct{})
	go func() { t.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		t.log.Warn("analytics: shutdown before flush completed", zap.Int("buffered", len(t.ch)))
	}
}

// Stats returns written and dropped counters.
func (t *AsyncTracker) Stats() (written, dropped int64) { return t.written.Load(), t.dropped.Load() }

// Track implements Tracker.
func (t *AsyncTracker) Track(ctx context.Context, name string, props map[string]any) {
	if !t.cfg.Enabled {
		return
	}
	meta := reqctx.MetaFrom(ctx)
	e := domain.AnalyticsEvent{
		EventID:    domain.NewID(),
		Name:       name,
		Source:     "server",
		OccurredAt: t.now().UTC(),
		ReceivedAt: t.now().UTC(),
		Properties: SanitizeProperties(props),
		Path:       truncate(meta.Path, 512),
		Locale:     truncate(meta.Locale, 16),
		UserAgent:  truncate(meta.UserAgent, 256),
		IPHash:     security.Keyed(t.cfg.IPHashSecret, meta.IP, 32),
		RequestID:  truncate(meta.RequestID, 64),
	}
	if p, ok := reqctx.PrincipalFrom(ctx); ok {
		id := p.UserID
		e.UserID = &id
		e.UserRole = string(p.Role)
	}
	if uid, ok := props["user_id"].(uuid.UUID); ok && e.UserID == nil {
		e.UserID = &uid
	}
	t.Enqueue(e)
}

// Enqueue implements Tracker.
func (t *AsyncTracker) Enqueue(e domain.AnalyticsEvent) bool {
	if !t.cfg.Enabled {
		return false
	}
	select {
	case t.ch <- e:
		return true
	default:
		if n := t.dropped.Add(1); n == 1 || n%1000 == 0 {
			t.log.Warn("analytics: buffer full, dropping events", zap.Int64("dropped_total", n))
		}
		return false
	}
}

// HashIP pseudonymises an IP address with the configured secret.
func (t *AsyncTracker) HashIP(ip string) string { return security.Keyed(t.cfg.IPHashSecret, ip, 32) }

func (t *AsyncTracker) run() {
	defer t.wg.Done()
	ticker := time.NewTicker(t.cfg.FlushInterval)
	defer ticker.Stop()
	batch := make([]domain.AnalyticsEvent, 0, t.cfg.BatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		n, err := t.store.InsertBatch(ctx, batch)
		cancel()
		if err != nil {
			t.log.Error("analytics: batch insert failed", zap.Error(err), zap.Int("events", len(batch)))
			t.dropped.Add(int64(len(batch)))
		} else {
			t.written.Add(n)
		}
		batch = batch[:0]
	}
	for {
		select {
		case e := <-t.ch:
			batch = append(batch, e)
			if len(batch) >= t.cfg.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-t.stop:
			for {
				select {
				case e := <-t.ch:
					batch = append(batch, e)
					if len(batch) >= t.cfg.BatchSize {
						flush()
					}
					continue
				default:
				}
				break
			}
			flush()
			return
		}
	}
}

// Property limits keep events small and free of free-form PII.
const (
	maxProps       = 32
	maxKeyLen      = 64
	maxStringValue = 256
)

var propKey = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// SanitizeProperties keeps scalar values with safe keys and bounded sizes.
func SanitizeProperties(in map[string]any) domain.JSONMap {
	out := domain.JSONMap{}
	for k, v := range in {
		if len(out) >= maxProps || !propKey.MatchString(k) || k == "user_id" {
			continue
		}
		switch val := v.(type) {
		case string:
			out[k] = truncate(val, maxStringValue)
		case bool, int, int32, int64, uint, uint32, uint64, float32, float64:
			out[k] = val
		case uuid.UUID:
			out[k] = val.String()
		case time.Time:
			out[k] = val.UTC().Format(time.RFC3339)
		case time.Duration:
			out[k] = val.Milliseconds()
		case nil:
			out[k] = nil
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// Noop discards events (tests).
type Noop struct{}

// Track implements Tracker.
func (Noop) Track(context.Context, string, map[string]any) {}

// Enqueue implements Tracker.
func (Noop) Enqueue(domain.AnalyticsEvent) bool { return true }

// Recorder captures events in memory (tests).
type Recorder struct {
	mu     sync.Mutex
	Events []domain.AnalyticsEvent
}

// Track implements Tracker.
func (r *Recorder) Track(ctx context.Context, name string, props map[string]any) {
	e := domain.AnalyticsEvent{Name: name, Properties: SanitizeProperties(props), Source: "server"}
	if p, ok := reqctx.PrincipalFrom(ctx); ok {
		id := p.UserID
		e.UserID = &id
	}
	r.mu.Lock()
	r.Events = append(r.Events, e)
	r.mu.Unlock()
}

// Enqueue implements Tracker.
func (r *Recorder) Enqueue(e domain.AnalyticsEvent) bool {
	r.mu.Lock()
	r.Events = append(r.Events, e)
	r.mu.Unlock()
	return true
}

// Names returns the recorded event names in order.
func (r *Recorder) Names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.Events))
	for i, e := range r.Events {
		out[i] = e.Name
	}
	return out
}
