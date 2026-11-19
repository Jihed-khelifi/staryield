package realtime

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
)

// revisions 6.0 (§26.5): the filtered stream behind the purchase stream.

type streamFixture struct {
	t      *testing.T
	s      *Server
	srv    *httptest.Server
	broker *MemoryBroker
	h      *fakeHandler
	user   uuid.UUID
}

// newStreamFixture serves spec (built by the test) for one user.
func newStreamFixture(t *testing.T, spec func() StreamSpec) *streamFixture {
	t.Helper()
	s, _, _, h, broker := newTestServer(t, func(c *config.RealtimeConfig) { c.PingInterval = 100 * time.Millisecond })
	f := &streamFixture{t: t, s: s, broker: broker, h: h, user: uuid.New()}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.ServeStream(w, r, f.user, "client", spec())
	}))
	t.Cleanup(f.srv.Close)
	return f
}

type sseLine struct {
	comment bool
	env     Envelope
}

// open starts the stream and returns its lines; closed is closed at EOF.
func (f *streamFixture) open(ctx context.Context) (lines chan sseLine, header http.Header, closed chan struct{}) {
	f.t.Helper()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, f.srv.URL, nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatalf("stream: %v", err)
	}
	lines, closed = make(chan sseLine, 64), make(chan struct{})
	go func() {
		defer close(closed)
		defer res.Body.Close()
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, ": "):
				lines <- sseLine{comment: true}
			case strings.HasPrefix(line, "data: "):
				var e Envelope
				if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &e) == nil {
					lines <- sseLine{env: e}
				}
			}
		}
	}()
	return lines, res.Header, closed
}

func nextEnvelope(t *testing.T, lines chan sseLine) Envelope {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case l := <-lines:
			if !l.comment {
				return l.env
			}
		case <-deadline:
			t.Fatal("timed out waiting for a frame")
		}
	}
}

func waitClosed(t *testing.T, closed chan struct{}) {
	t.Helper()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("the stream did not close")
	}
}

func publishTo(t *testing.T, b *MemoryBroker, u uuid.UUID, typ string, data any) {
	t.Helper()
	payload, err := encode(typ, data, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Publish(context.Background(), u, payload); err != nil {
		t.Fatal(err)
	}
}

func relayByID(want string) func(string, json.RawMessage) (StreamEvent, bool) {
	return func(typ string, data json.RawMessage) (StreamEvent, bool) {
		var v struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if typ != "purchase.updated" || json.Unmarshal(data, &v) != nil || v.ID != want {
			return StreamEvent{}, false
		}
		return StreamEvent{Type: typ, Data: data}, true
	}
}

func finalStatus(ev StreamEvent) bool {
	if ev.Type == EventPurchaseTimeout {
		return true
	}
	raw, ok := ev.Data.(json.RawMessage)
	if !ok {
		return false
	}
	var v struct {
		Status string `json:"status"`
	}
	return json.Unmarshal(raw, &v) == nil && v.Status == "succeeded"
}

func TestStreamSendsInitialFrameFiltersRelaysAndClosesOnTerminal(t *testing.T) {
	f := newStreamFixture(t, func() StreamSpec {
		return StreamSpec{Initial: &StreamEvent{Type: EventPurchaseUpdated, Data: map[string]string{"id": "p1", "status": "pending"}},
			Relay: relayByID("p1"), Terminal: finalStatus}
	})
	lines, header, closed := f.open(context.Background())
	if ct := header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") || header.Get("Cache-Control") != "no-cache, no-transform" ||
		header.Get("X-Accel-Buffering") != "no" {
		t.Fatalf("headers: %v", header)
	}
	first := nextEnvelope(t, lines)
	if first.Type != EventPurchaseUpdated || !strings.Contains(string(first.Data), `"pending"`) || first.ID != "" {
		t.Fatalf("initial frame: %+v", first)
	}
	// Other types and other purchases never reach the stream.
	publishTo(t, f.broker, f.user, "wallet.updated", map[string]int{"balance": 5})
	publishTo(t, f.broker, f.user, "purchase.updated", map[string]string{"id": "other", "status": "succeeded"})
	// Another user's event is not delivered at all.
	publishTo(t, f.broker, uuid.New(), "purchase.updated", map[string]string{"id": "p1", "status": "succeeded"})
	select {
	case l := <-lines:
		if !l.comment {
			t.Fatalf("unexpected frame: %+v", l.env)
		}
	case <-time.After(250 * time.Millisecond):
	}
	publishTo(t, f.broker, f.user, "purchase.updated", map[string]string{"id": "p1", "status": "succeeded"})
	last := nextEnvelope(t, lines)
	if !strings.Contains(string(last.Data), `"succeeded"`) {
		t.Fatalf("final frame: %+v", last)
	}
	waitClosed(t, closed)
	f.h.mu.Lock()
	defer f.h.mu.Unlock()
	if f.h.connected != 0 || f.h.disconnected != 0 {
		t.Fatalf("a filtered stream is no presence connection: %+v", f.h)
	}
}

func TestStreamEndsRightAfterAFinalInitialFrame(t *testing.T) {
	f := newStreamFixture(t, func() StreamSpec {
		return StreamSpec{Initial: &StreamEvent{Type: EventPurchaseUpdated, Data: json.RawMessage(`{"id":"p1","status":"succeeded"}`)},
			Terminal: finalStatus}
	})
	lines, _, closed := f.open(context.Background())
	if e := nextEnvelope(t, lines); !strings.Contains(string(e.Data), "succeeded") {
		t.Fatalf("frame: %+v", e)
	}
	waitClosed(t, closed)
}

func TestStreamKeepAliveAndLifetimeTimeout(t *testing.T) {
	f := newStreamFixture(t, func() StreamSpec {
		return StreamSpec{Initial: &StreamEvent{Type: EventPurchaseUpdated, Data: map[string]string{"id": "p1", "status": "pending"}},
			Terminal: finalStatus, MaxLifetime: 450 * time.Millisecond, Timeout: &StreamEvent{Type: EventPurchaseTimeout}}
	})
	lines, _, closed := f.open(context.Background())
	nextEnvelope(t, lines)
	sawComment := false
	deadline := time.After(3 * time.Second)
	for {
		select {
		case l := <-lines:
			if l.comment {
				sawComment = true
				continue
			}
			if l.env.Type != EventPurchaseTimeout {
				t.Fatalf("frame: %+v", l.env)
			}
			if !sawComment {
				t.Fatal("expected keep-alive comments before the timeout")
			}
			waitClosed(t, closed)
			return
		case <-deadline:
			t.Fatal("no timeout frame")
		}
	}
}

func TestStreamPollWritesChangesOnlyAndSurvivesErrors(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	f := newStreamFixture(t, func() StreamSpec {
		return StreamSpec{Initial: &StreamEvent{Type: EventPurchaseUpdated, Data: map[string]string{"id": "p1", "status": "pending"}},
			Terminal: finalStatus, PollEvery: 40 * time.Millisecond,
			Poll: func(_ context.Context, n int) (*StreamEvent, error) {
				mu.Lock()
				defer mu.Unlock()
				calls++
				switch {
				case n == 1:
					return nil, context.DeadlineExceeded // a failed check does not end the stream
				case n < 5:
					return &StreamEvent{Type: EventPurchaseUpdated, Data: map[string]string{"id": "p1", "status": "pending"}}, nil // same state
				}
				return &StreamEvent{Type: EventPurchaseUpdated, Data: json.RawMessage(`{"id":"p1","status":"succeeded"}`)}, nil
			}}
	})
	lines, _, closed := f.open(context.Background())
	first := nextEnvelope(t, lines)
	if !strings.Contains(string(first.Data), "pending") {
		t.Fatalf("initial: %+v", first)
	}
	// The repeated pending state is written once, the change after it.
	next := nextEnvelope(t, lines)
	if !strings.Contains(string(next.Data), "succeeded") {
		t.Fatalf("second frame must be the change, got %+v", next)
	}
	waitClosed(t, closed)
	mu.Lock()
	defer mu.Unlock()
	if calls < 5 {
		t.Fatalf("polls: %d", calls)
	}
}

func TestStreamEndsOnRevokedSessionAndClientDisconnect(t *testing.T) {
	f := newStreamFixture(t, func() StreamSpec {
		return StreamSpec{Initial: &StreamEvent{Type: EventPurchaseUpdated, Data: map[string]string{"id": "p1", "status": "pending"}}, Terminal: finalStatus}
	})
	lines, _, closed := f.open(context.Background())
	nextEnvelope(t, lines)
	publishTo(t, f.broker, f.user, EventAuthRevoked, map[string]string{"reason": "session_revoked"})
	waitClosed(t, closed)

	// A client that goes away releases its hub slot.
	f2 := newStreamFixture(t, func() StreamSpec {
		return StreamSpec{Initial: &StreamEvent{Type: EventPurchaseUpdated, Data: map[string]string{"id": "p1", "status": "pending"}}, Terminal: finalStatus}
	})
	ctx, cancel := context.WithCancel(context.Background())
	l2, _, closed2 := f2.open(ctx)
	nextEnvelope(t, l2)
	if n := f2.s.hub.UserConnections(f2.user); n != 1 {
		t.Fatalf("connections while open: %d", n)
	}
	cancel()
	waitClosed(t, closed2)
	deadline := time.Now().Add(2 * time.Second)
	for f2.s.hub.UserConnections(f2.user) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("the hub slot was not released")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestFullSSEStillGreetsAndSharesTheHeaders(t *testing.T) {
	s, _, _, h, broker := newTestServer(t, func(c *config.RealtimeConfig) { c.PingInterval = 100 * time.Millisecond })
	u := uuid.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { s.ServeSSE(w, r, u, "client") }))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") || res.Header.Get("Cache-Control") != "no-cache, no-transform" {
		t.Fatalf("headers: %v", res.Header)
	}
	sc := bufio.NewScanner(res.Body)
	seen := map[string]bool{}
	for sc.Scan() {
		line := sc.Text()
		if line == "retry: 3000" {
			seen["retry"] = true
		}
		if strings.HasPrefix(line, "data: ") && strings.Contains(line, `"type":"hello"`) {
			seen["hello"] = true
			publishTo(t, broker, u, "wallet.updated", map[string]int{"balance": 1})
		}
		if strings.HasPrefix(line, "data: ") && strings.Contains(line, "wallet.updated") {
			seen["event"] = true
		}
		if line == ": keep-alive" {
			seen["keepalive"] = true
		}
		if len(seen) == 4 {
			break
		}
	}
	cancel()
	if !seen["retry"] || !seen["hello"] || !seen["event"] || !seen["keepalive"] {
		t.Fatalf("full stream behaviour changed: %v", seen)
	}
	time.Sleep(100 * time.Millisecond)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.connected != 1 {
		t.Fatalf("the full stream still runs the connect hook: %+v", h)
	}
}

func TestFilteredStreamsHaveTheirOwnConnectionCap(t *testing.T) {
	hub := NewHub(zapNop(), 2)
	u := uuid.New()
	mk := func(filtered bool) *Client {
		c := newClient(u, "client", "sse", 4, false, zapNop())
		c.filtered = filtered
		return c
	}
	for i := 0; i < 2; i++ {
		if err := hub.Register(mk(false)); err != nil {
			t.Fatalf("live connection %d: %v", i, err)
		}
	}
	if err := hub.Register(mk(false)); err == nil {
		t.Fatal("the per-user cap of live connections must hold")
	}
	// Streams do not use up the live connections, and have a cap of their own.
	for i := 0; i < maxStreamsPerUser; i++ {
		if err := hub.Register(mk(true)); err != nil {
			t.Fatalf("stream %d: %v", i, err)
		}
	}
	if err := hub.Register(mk(true)); err == nil {
		t.Fatal("too many simultaneous streams")
	}
	if hub.UserConnections(u) != 2+maxStreamsPerUser {
		t.Fatalf("connections: %d", hub.UserConnections(u))
	}
}

func zapNop() *zap.Logger { return zap.NewNop() }
