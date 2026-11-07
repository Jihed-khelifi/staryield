package analytics

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

type memVisitorStore struct {
	mu      sync.Mutex
	batches []*domain.VisitorBatch
	fail    bool
	block   chan struct{}
}

func (m *memVisitorStore) ApplyVisitorBatch(_ context.Context, b *domain.VisitorBatch) error {
	if m.block != nil {
		<-m.block
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("boom")
	}
	m.batches = append(m.batches, b)
	return nil
}

func (m *memVisitorStore) views() (n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, b := range m.batches {
		for _, c := range b.Visits {
			n += c.Views
		}
	}
	return n
}

func (m *memVisitorStore) batchCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.batches)
}

func viewEvent(id string) domain.VisitorEvent {
	return domain.VisitorEvent{Type: domain.VisitView, AnonymousID: id, At: time.Now(), Locale: "en", Kind: domain.CalcAstrology}
}

func TestVisitorAggregatorBatchesBySize(t *testing.T) {
	st := &memVisitorStore{}
	a := NewVisitorAggregator(VisitorConfig{Enabled: true, BufferSize: 100, BatchSize: 10, FlushInterval: time.Hour}, st, zap.NewNop())
	a.Start()
	for i := 0; i < 25; i++ {
		if !a.EnqueueVisitor(viewEvent("visitor-0001")) {
			t.Fatal("enqueue refused")
		}
	}
	deadline := time.Now().Add(2 * time.Second)
	for st.batchCount() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if st.batchCount() != 2 {
		t.Fatalf("size-triggered flushes: %d", st.batchCount())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	a.Stop(ctx) // the remaining 5 events are flushed on stop
	if st.views() != 25 {
		t.Fatalf("views stored: %d", st.views())
	}
	// Events of one visitor coalesce into one row per batch.
	st.mu.Lock()
	for _, b := range st.batches {
		if len(b.Visits) != 1 || len(b.Visitors) != 1 {
			t.Fatalf("batch not coalesced: %d rows", len(b.Visits))
		}
	}
	st.mu.Unlock()
	if w, d := a.Stats(); w != 25 || d != 0 {
		t.Fatalf("stats: written=%d dropped=%d", w, d)
	}
}

func TestVisitorAggregatorFlushesOnInterval(t *testing.T) {
	st := &memVisitorStore{}
	a := NewVisitorAggregator(VisitorConfig{Enabled: true, BufferSize: 100, BatchSize: 1000, FlushInterval: 20 * time.Millisecond}, st, nil)
	a.Start()
	defer a.Stop(context.Background())
	a.EnqueueVisitor(viewEvent("visitor-0002"))
	deadline := time.Now().Add(2 * time.Second)
	for st.views() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if st.views() != 1 {
		t.Fatal("interval flush never happened")
	}
}

func TestVisitorAggregatorFlushOnStopAndExplicitFlush(t *testing.T) {
	st := &memVisitorStore{}
	a := NewVisitorAggregator(VisitorConfig{Enabled: true, BufferSize: 100, BatchSize: 1000, FlushInterval: time.Hour}, st, zap.NewNop())
	a.Start()
	a.EnqueueVisitor(viewEvent("visitor-0003"))
	a.Flush(context.Background())
	if st.views() != 1 {
		t.Fatalf("explicit flush stored %d", st.views())
	}
	a.EnqueueVisitor(viewEvent("visitor-0003"))
	a.EnqueueVisitor(viewEvent("visitor-0004"))
	a.Stop(context.Background())
	if st.views() != 3 {
		t.Fatalf("stop flush stored %d", st.views())
	}
	a.Flush(context.Background()) // after Stop: returns instead of blocking
}

func TestVisitorAggregatorDropsAndCountsWhenFull(t *testing.T) {
	st := &memVisitorStore{block: make(chan struct{})}
	a := NewVisitorAggregator(VisitorConfig{Enabled: true, BufferSize: 4, BatchSize: 1, FlushInterval: time.Hour}, st, zap.NewNop())
	a.Start()
	accepted := 0
	for i := 0; i < 50; i++ {
		if a.EnqueueVisitor(viewEvent("visitor-0005")) {
			accepted++
		}
	}
	_, dropped := a.Stats()
	if dropped == 0 || int(dropped)+accepted != 50 {
		t.Fatalf("accepted=%d dropped=%d", accepted, dropped)
	}
	close(st.block)
	a.Stop(context.Background())
}

func TestVisitorAggregatorDisabledAndFailedWrites(t *testing.T) {
	st := &memVisitorStore{}
	off := NewVisitorAggregator(VisitorConfig{Enabled: false}, st, nil)
	if off.EnqueueVisitor(viewEvent("visitor-0006")) || off.Enabled() {
		t.Fatal("disabled aggregator accepted an event")
	}
	failing := &memVisitorStore{fail: true}
	a := NewVisitorAggregator(VisitorConfig{Enabled: true, BufferSize: 10, BatchSize: 100, FlushInterval: time.Hour}, failing, zap.NewNop())
	a.Start()
	a.EnqueueVisitor(viewEvent("visitor-0007"))
	a.EnqueueVisitor(viewEvent("visitor-0007"))
	a.Flush(context.Background())
	w, d := a.Stats()
	if w != 0 || d != 2 {
		t.Fatalf("failed batch must count as dropped: written=%d dropped=%d", w, d)
	}
	a.Stop(context.Background())
}

func TestNoopVisitors(t *testing.T) {
	var s VisitorSink = NoopVisitors{}
	if !s.EnqueueVisitor(viewEvent("visitor-0008")) {
		t.Fatal("noop")
	}
}
