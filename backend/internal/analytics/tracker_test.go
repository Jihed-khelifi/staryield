package analytics

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

type memStore struct {
	mu      sync.Mutex
	batches [][]domain.AnalyticsEvent
}

func (m *memStore) InsertBatch(_ context.Context, events []domain.AnalyticsEvent) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batches = append(m.batches, append([]domain.AnalyticsEvent(nil), events...))
	return int64(len(events)), nil
}

func (m *memStore) total() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, b := range m.batches {
		n += len(b)
	}
	return n
}

func TestSanitizeProperties(t *testing.T) {
	in := map[string]any{
		"ok": "value", "Bad Key": 1, "user_id": "x", "nested": map[string]any{"a": 1},
		"long": strings.Repeat("x", 1000), "n": 3, "flag": true, "at": time.Unix(0, 0), "id": uuid.Nil,
	}
	out := SanitizeProperties(in)
	if _, ok := out["Bad Key"]; ok {
		t.Fatal("invalid keys must be dropped")
	}
	if _, ok := out["nested"]; ok {
		t.Fatal("non-scalar values must be dropped")
	}
	if _, ok := out["user_id"]; ok {
		t.Fatal("user_id is reserved")
	}
	if len(out["long"].(string)) != 256 || out["n"] != 3 || out["flag"] != true || out["at"] != "1970-01-01T00:00:00Z" {
		t.Fatalf("sanitized: %v", out)
	}
	many := map[string]any{}
	for i := 0; i < 100; i++ {
		many["k"+strings.Repeat("a", i%50)+string(rune('a'+i%26))] = i
	}
	if len(SanitizeProperties(many)) > maxProps {
		t.Fatal("property count must be capped")
	}
}

func TestAsyncTrackerBatchesAndFlushesOnStop(t *testing.T) {
	store := &memStore{}
	tr := NewAsyncTracker(Config{Enabled: true, BufferSize: 100, BatchSize: 10, FlushInterval: time.Hour, IPHashSecret: "s"}, store, zap.NewNop(), nil)
	tr.Start()
	ctx := reqctx.WithPrincipal(reqctx.WithMeta(context.Background(), reqctx.Meta{IP: "203.0.113.9", RequestID: "r1"}),
		reqctx.Principal{UserID: uuid.New(), Role: domain.RoleClient})
	for i := 0; i < 25; i++ {
		tr.Track(ctx, "chat.message_sent", map[string]any{"length": i})
	}
	tr.Stop(context.Background())
	if store.total() != 25 {
		t.Fatalf("written %d, want 25", store.total())
	}
	e := store.batches[0][0]
	if e.UserID == nil || e.IPHash == "" || e.IPHash == "203.0.113.9" || e.RequestID != "r1" || e.Source != "server" {
		t.Fatalf("enrichment: %+v", e)
	}
	if written, dropped := tr.Stats(); written != 25 || dropped != 0 {
		t.Fatalf("stats: %d %d", written, dropped)
	}
}

func TestAsyncTrackerDropsWhenFull(t *testing.T) {
	tr := NewAsyncTracker(Config{Enabled: true, BufferSize: 2, BatchSize: 10}, &memStore{}, zap.NewNop(), nil)
	// Not started: the buffer fills up and further events are dropped.
	for i := 0; i < 5; i++ {
		tr.Track(context.Background(), "x.y", nil)
	}
	if _, dropped := tr.Stats(); dropped != 3 {
		t.Fatalf("dropped %d, want 3", dropped)
	}
	disabled := NewAsyncTracker(Config{Enabled: false}, &memStore{}, zap.NewNop(), nil)
	if disabled.Enqueue(domain.AnalyticsEvent{}) {
		t.Fatal("disabled tracker must not enqueue")
	}
}
