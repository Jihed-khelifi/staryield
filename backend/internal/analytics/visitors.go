package analytics

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// VisitorStore persists coalesced visitor batches.
type VisitorStore interface {
	ApplyVisitorBatch(ctx context.Context, b *domain.VisitorBatch) error
}

// VisitorSink receives validated visitor events.
type VisitorSink interface {
	// EnqueueVisitor never blocks; it returns false when the event was dropped.
	EnqueueVisitor(e domain.VisitorEvent) bool
}

// NoopVisitors discards visitor events (tests, analytics disabled).
type NoopVisitors struct{}

// EnqueueVisitor implements VisitorSink.
func (NoopVisitors) EnqueueVisitor(domain.VisitorEvent) bool { return true }

// VisitorConfig tunes the roll-up writer.
type VisitorConfig struct {
	Enabled       bool
	BufferSize    int
	BatchSize     int // events coalesced per flush
	FlushInterval time.Duration
}

// VisitorAggregator rolls browser events up into web_visitors and
// calculator_visits. Like the event tracker it is asynchronous and bounded:
// events are queued without blocking ingestion, coalesced in memory, written
// with multi-row upserts, and dropped (and counted) when the buffer is full.
type VisitorAggregator struct {
	cfg     VisitorConfig
	store   VisitorStore
	log     *zap.Logger
	ch      chan domain.VisitorEvent
	flushQ  chan chan struct{}
	dropped atomic.Int64
	written atomic.Int64
	wg      sync.WaitGroup
	once    sync.Once
	stop    chan struct{}
}

// NewVisitorAggregator returns an aggregator; call Start to begin writing.
func NewVisitorAggregator(cfg VisitorConfig, store VisitorStore, log *zap.Logger) *VisitorAggregator {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 10000
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 500
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 2 * time.Second
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &VisitorAggregator{cfg: cfg, store: store, log: log, ch: make(chan domain.VisitorEvent, cfg.BufferSize),
		flushQ: make(chan chan struct{}), stop: make(chan struct{})}
}

// Start launches the background writer.
func (a *VisitorAggregator) Start() {
	a.wg.Add(1)
	go a.run()
}

// Stop flushes buffered events and stops the writer, waiting until ctx ends.
func (a *VisitorAggregator) Stop(ctx context.Context) {
	a.once.Do(func() { close(a.stop) })
	done := make(chan struct{})
	go func() { a.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		a.log.Warn("visitors: shutdown before flush completed", zap.Int("buffered", len(a.ch)))
	}
}

// Stats returns events written and dropped.
func (a *VisitorAggregator) Stats() (written, dropped int64) {
	return a.written.Load(), a.dropped.Load()
}

// Enabled reports whether events are accepted.
func (a *VisitorAggregator) Enabled() bool { return a.cfg.Enabled }

// EnqueueVisitor implements VisitorSink.
func (a *VisitorAggregator) EnqueueVisitor(e domain.VisitorEvent) bool {
	if !a.cfg.Enabled {
		return false
	}
	select {
	case a.ch <- e:
		return true
	default:
		if n := a.dropped.Add(1); n == 1 || n%1000 == 0 {
			a.log.Warn("visitors: buffer full, dropping events", zap.Int64("dropped_total", n))
		}
		return false
	}
}

// Flush writes everything queued before the call and waits for it (tests,
// graceful hand-offs). It returns early when ctx ends or the writer stopped.
func (a *VisitorAggregator) Flush(ctx context.Context) {
	done := make(chan struct{})
	select {
	case a.flushQ <- done:
	case <-ctx.Done():
		return
	case <-a.stop:
		return
	}
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func (a *VisitorAggregator) run() {
	defer a.wg.Done()
	ticker := time.NewTicker(a.cfg.FlushInterval)
	defer ticker.Stop()
	batch := domain.NewVisitorBatch()
	n := 0
	flush := func() {
		if batch.Empty() {
			n = 0
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := a.store.ApplyVisitorBatch(ctx, batch)
		cancel()
		if err != nil {
			a.log.Error("visitors: batch write failed", zap.Error(err), zap.Int("events", n))
			a.dropped.Add(int64(n))
		} else {
			a.written.Add(int64(n))
		}
		batch, n = domain.NewVisitorBatch(), 0
	}
	add := func(e domain.VisitorEvent) {
		batch.Add(e)
		n++
		if n >= a.cfg.BatchSize {
			flush()
		}
	}
	drain := func() {
		for {
			select {
			case e := <-a.ch:
				add(e)
			default:
				return
			}
		}
	}
	for {
		select {
		case e := <-a.ch:
			add(e)
		case <-ticker.C:
			flush()
		case done := <-a.flushQ:
			drain()
			flush()
			close(done)
		case <-a.stop:
			drain()
			flush()
			return
		}
	}
}
