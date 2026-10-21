package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/config"
)

func TestStreamIDs(t *testing.T) {
	if !IsAfter("5-1", "5-0") || !IsAfter("6-0", "5-9") || IsAfter("5-0", "5-0") || IsAfter("4-9", "5-0") {
		t.Fatal("ordering")
	}
	if !IsAfter("1-0", "") || IsAfter("bogus", "1-0") {
		t.Fatal("edge cases")
	}
	if _, ok := ParseStreamID("12-x"); ok {
		t.Fatal("malformed id parsed")
	}
	if got := string(withID("7-1", []byte(`{"type":"x"}`))); got != `{"id":"7-1","type":"x"}` {
		t.Fatalf("withID: %s", got)
	}
	if got := string(withID("", []byte(`{"type":"x"}`))); got != `{"type":"x"}` {
		t.Fatalf("withID empty: %s", got)
	}
}

func TestMemoryBrokerReplayAndGap(t *testing.T) {
	b := NewMemoryBroker(3)
	u := uuid.New()
	var ids []string
	for i := 0; i < 5; i++ {
		id, _ := b.Publish(context.Background(), u, []byte(`{"type":"e"}`))
		ids = append(ids, id)
	}
	// Retained: last 3. Resuming from ids[2] → ids[3], ids[4], no gap.
	frames, gap, _ := b.Replay(context.Background(), u, ids[2], 100)
	if len(frames) != 2 || frames[0].ID != ids[3] || gap {
		t.Fatalf("replay: %d frames gap=%v", len(frames), gap)
	}
	// Resuming from a trimmed id → gap.
	if _, gap, _ := b.Replay(context.Background(), u, ids[0], 100); !gap {
		t.Fatal("trimmed history must report a gap")
	}
	// Unknown user / garbage id.
	if _, gap, _ := b.Replay(context.Background(), uuid.New(), ids[0], 100); !gap {
		t.Fatal("empty stream must report a gap")
	}
	if _, gap, _ := b.Replay(context.Background(), u, "nope", 100); !gap {
		t.Fatal("invalid id must report a gap")
	}
}

func TestRedisBrokerPublishReplayAndFanout(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	b := NewRedisBroker(client, "t:", 100, time.Hour, zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	got := make(chan Delivery, 10)
	go func() { _ = b.Run(ctx, func(d Delivery) { got <- d }) }()
	time.Sleep(100 * time.Millisecond)

	u := uuid.New()
	payload, _ := encode("message.created", map[string]string{"body": "hi"}, time.Now())
	id1, err := b.Publish(ctx, u, payload)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	select {
	case d := <-got:
		if d.UserID != u || d.ID != id1 || !strings.HasPrefix(string(d.Frame), `{"id":"`+id1+`"`) {
			t.Fatalf("delivery: %+v %s", d, d.Frame)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no fan-out")
	}
	id2, _ := b.Publish(ctx, u, payload)
	frames, gap, err := b.Replay(ctx, u, id1, 10)
	if err != nil || gap || len(frames) != 1 || frames[0].ID != id2 {
		t.Fatalf("replay: %v %v %+v", err, gap, frames)
	}
	if err := b.Control(ctx, u, ControlRevoke); err != nil {
		t.Fatal(err)
	}
	// Skip the second event delivery, then expect the control message.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case d := <-got:
			if d.Control == ControlRevoke && d.UserID == u {
				return
			}
		case <-deadline:
			t.Fatal("no control delivery")
		}
	}
}

func newTestClient(user uuid.UUID, buffer int, replaying bool) *Client {
	return newClient(user, "client", "ws", buffer, replaying, zap.NewNop())
}

func drain(c *Client) []outFrame {
	var out []outFrame
	for {
		select {
		case f := <-c.send:
			out = append(out, f)
		default:
			return out
		}
	}
}

func TestClientReplayOrderingAndDedupe(t *testing.T) {
	c := newTestClient(uuid.New(), 32, true)
	// Live events arrive during replay (one duplicates a replayed event).
	c.deliver(Delivery{ID: "10-0", Frame: []byte("live-10")})
	c.deliver(Delivery{ID: "11-0", Frame: []byte("live-11")})
	c.deliver(Delivery{Frame: []byte("ephemeral")})
	c.finishReplay([]Delivery{{ID: "9-0", Frame: []byte("old-9")}, {ID: "10-0", Frame: []byte("old-10")}})
	var got []string
	for _, f := range drain(c) {
		got = append(got, string(f.Data))
	}
	want := []string{"old-9", "old-10", "live-11", "ephemeral"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("order: got %v want %v", got, want)
	}
	// Late duplicates of already-sent ids are dropped.
	c.deliver(Delivery{ID: "11-0", Frame: []byte("dup")})
	if len(drain(c)) != 0 {
		t.Fatal("duplicate delivered")
	}
}

func TestHubDispatchRevokeAndSlowConsumer(t *testing.T) {
	h := NewHub(zap.NewNop(), 2)
	u := uuid.New()
	a, b := newTestClient(u, 16, false), newTestClient(u, 16, false)
	if err := h.Register(a); err != nil {
		t.Fatal(err)
	}
	if err := h.Register(b); err != nil {
		t.Fatal(err)
	}
	if err := h.Register(newTestClient(u, 16, false)); !errors.Is(err, ErrTooManyConnections) {
		t.Fatalf("connection cap: %v", err)
	}
	other := newTestClient(uuid.New(), 16, false)
	_ = h.Register(other)

	h.Dispatch(Delivery{UserID: u, ID: "1-0", Frame: []byte(`{"type":"x"}`)})
	if len(drain(a)) != 1 || len(drain(b)) != 1 || len(drain(other)) != 0 {
		t.Fatal("user-targeted delivery")
	}
	h.Dispatch(Delivery{Broadcast: true, Frame: []byte(`{"type":"bc"}`)})
	if len(drain(other)) != 1 || len(drain(a)) != 1 || len(drain(b)) != 1 {
		t.Fatal("broadcast")
	}
	h.Dispatch(Delivery{Broadcast: true, Control: ControlResync})
	if f := drain(a); len(f) != 1 || !strings.Contains(string(f[0].Data), EventResyncRequired) {
		t.Fatalf("resync: %v", f)
	}

	h.Dispatch(Delivery{UserID: u, Control: ControlRevoke})
	select {
	case <-a.Done():
	default:
		t.Fatal("revoke must close the client")
	}
	if code, _ := a.closeInfo(); code != CloseUnauthorized {
		t.Fatalf("close code %d", code)
	}

	slow := newTestClient(uuid.New(), 2, false)
	_ = h.Register(slow)
	for i := 0; i < 3; i++ {
		h.Dispatch(Delivery{UserID: slow.UserID, Frame: []byte("x")})
	}
	if code, _ := slow.closeInfo(); code != CloseSlowConsumer {
		t.Fatalf("slow consumer not closed: %d", code)
	}
	h.Unregister(a)
	h.Unregister(a) // idempotent
	if h.UserConnections(u) != 1 {
		t.Fatalf("connections: %d", h.UserConnections(u))
	}
}

func TestHubShutdownClosesClients(t *testing.T) {
	h := NewHub(zap.NewNop(), 0)
	c := newTestClient(uuid.New(), 8, false)
	_ = h.Register(c)
	done := make(chan struct{})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		h.Shutdown(ctx)
		close(done)
	}()
	<-c.Done()
	h.Unregister(c)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not finish after drain")
	}
	if err := h.Register(newTestClient(uuid.New(), 8, false)); !errors.Is(err, ErrShuttingDown) {
		t.Fatal("registration after shutdown")
	}
}

func TestPresenceImplementations(t *testing.T) {
	mr := miniredis.RunT(t)
	rp := NewRedisPresence(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "t:", time.Minute)
	for name, p := range map[string]Presence{"memory": NewMemoryPresence(time.Minute, nil), "redis": rp} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			u, s := uuid.New(), uuid.New()
			if err := p.Connect(ctx, "c1", u); err != nil {
				t.Fatal(err)
			}
			if owner, ok, _ := p.ConnectionOwner(ctx, "c1"); !ok || owner != u {
				t.Fatal("owner")
			}
			online, _ := p.Online(ctx, []uuid.UUID{u, uuid.New()})
			if !online[u] || len(online) != 2 {
				t.Fatalf("online: %v", online)
			}
			_ = p.Join(ctx, "c1", u, s)
			_ = p.Connect(ctx, "c2", u)
			_ = p.Join(ctx, "c2", u, s)
			if in, _ := p.InSession(ctx, s, u); !in {
				t.Fatal("in session")
			}
			_ = p.Heartbeat(ctx, "c1", u)
			joined, _ := p.Disconnect(ctx, "c1", u)
			if len(joined) != 1 || joined[0] != s {
				t.Fatalf("joined: %v", joined)
			}
			if in, _ := p.InSession(ctx, s, u); !in {
				t.Fatal("second connection keeps presence")
			}
			_ = p.Leave(ctx, "c2", u, s)
			if in, _ := p.InSession(ctx, s, u); in {
				t.Fatal("left")
			}
		})
	}
}

func TestOnlineUsersIndex(t *testing.T) {
	mr := miniredis.RunT(t)
	rp := NewRedisPresence(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "t:", time.Minute)
	for name, p := range map[string]Presence{"memory": NewMemoryPresence(time.Minute, nil), "redis": rp} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			a, b := uuid.New(), uuid.New()
			_ = p.Connect(ctx, "a1", a)
			_ = p.Connect(ctx, "a2", a)
			_ = p.Connect(ctx, "b1", b)
			users, err := p.OnlineUsers(ctx, 10)
			if err != nil || len(users) != 2 {
				t.Fatalf("online users: %v %v", users, err)
			}
			if limited, _ := p.OnlineUsers(ctx, 1); len(limited) != 1 {
				t.Fatalf("limit: %v", limited)
			}
			// a stays online while one of their connections remains.
			_, _ = p.Disconnect(ctx, "a1", a)
			_, _ = p.Disconnect(ctx, "b1", b)
			users, _ = p.OnlineUsers(ctx, 10)
			if len(users) != 1 || users[0] != a {
				t.Fatalf("after disconnects: %v", users)
			}
			_, _ = p.Disconnect(ctx, "a2", a)
			if users, _ = p.OnlineUsers(ctx, 10); len(users) != 0 {
				t.Fatalf("all gone: %v", users)
			}
		})
	}
}

func TestTickets(t *testing.T) {
	mr := miniredis.RunT(t)
	for name, ts := range map[string]TicketStore{"memory": NewMemoryTickets(nil), "redis": NewRedisTickets(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "t:")} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			u := uuid.New()
			tk, err := ts.Issue(ctx, u, "psychic", time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			id, role, ok, err := ts.Consume(ctx, tk)
			if err != nil || !ok || id != u || role != "psychic" {
				t.Fatalf("consume: %v %v %v %v", id, role, ok, err)
			}
			if _, _, ok, _ := ts.Consume(ctx, tk); ok {
				t.Fatal("tickets must be single use")
			}
			if _, _, ok, _ := ts.Consume(ctx, "unknown"); ok {
				t.Fatal("unknown ticket accepted")
			}
		})
	}
}

// fakeHandler echoes frames and records lifecycle calls.
type fakeHandler struct {
	mu           sync.Mutex
	connected    int
	disconnected int
}

func (f *fakeHandler) OnConnect(context.Context, *Client) {
	f.mu.Lock()
	f.connected++
	f.mu.Unlock()
}

func (f *fakeHandler) OnDisconnect(context.Context, *Client, []uuid.UUID) {
	f.mu.Lock()
	f.disconnected++
	f.mu.Unlock()
}

func (f *fakeHandler) HandleFrame(_ context.Context, _ *Client, typ string, data json.RawMessage) (any, error) {
	switch typ {
	case "echo":
		return json.RawMessage(data), nil
	case "boom":
		panic("handler exploded")
	}
	return nil, apperror.BadRequest("Unknown frame type.")
}

func newTestServer(t *testing.T, mutate func(*config.RealtimeConfig)) (*Server, *httptest.Server, TicketStore, *fakeHandler, *MemoryBroker) {
	t.Helper()
	cfg := config.RealtimeConfig{PingInterval: time.Second, PongWait: 3 * time.Second, WriteWait: time.Second, MaxFrameBytes: 1024,
		SendBuffer: 64, ReplayLimit: 100, MaxConnsPerUser: 3, PresenceTTL: time.Minute, FrameRate: 100, FrameBurst: 100}
	if mutate != nil {
		mutate(&cfg)
	}
	broker := NewMemoryBroker(100)
	tickets := NewMemoryTickets(nil)
	h := &fakeHandler{}
	hub := NewHub(zap.NewNop(), cfg.MaxConnsPerUser)
	s := NewServer(hub, broker, NewMemoryPresence(time.Minute, nil), tickets, h, nil, cfg, []string{"http://app.test"}, zap.NewNop())
	broker.Subscribe(hub.Dispatch)
	srv := httptest.NewServer(http.HandlerFunc(s.ServeWS))
	t.Cleanup(srv.Close)
	return s, srv, tickets, h, broker
}

func dial(t *testing.T, srv *httptest.Server, ticket, origin, resume string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/?ticket=" + ticket
	if resume != "" {
		u += "&resume=" + resume
	}
	return websocket.DefaultDialer.Dial(u, http.Header{"Origin": {origin}})
}

func readEnvelope(t *testing.T, c *websocket.Conn) Envelope {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	var e Envelope
	if err := c.ReadJSON(&e); err != nil {
		t.Fatalf("read: %v", err)
	}
	return e
}

func TestServerHandshakeAndFrames(t *testing.T) {
	_, srv, tickets, h, _ := newTestServer(t, nil)
	u := uuid.New()

	if _, res, err := dial(t, srv, "bad", "http://app.test", ""); err == nil || res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("invalid ticket: %v", res)
	}
	tk, _ := tickets.Issue(context.Background(), u, "client", time.Minute)
	if _, res, err := dial(t, srv, tk, "http://evil.test", ""); err == nil || res.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign origin: %v", res)
	}
	conn, _, err := dial(t, srv, tk, "http://app.test", "")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	if hello := readEnvelope(t, conn); hello.Type != EventHello {
		t.Fatalf("hello: %+v", hello)
	}
	_ = conn.WriteJSON(ClientFrame{Type: "echo", Ref: "1", Data: json.RawMessage(`{"v":1}`)})
	if ack := readEnvelope(t, conn); ack.Type != EventAck || ack.Ref != "1" || string(ack.Data) != `{"v":1}` {
		t.Fatalf("ack: %+v", ack)
	}
	_ = conn.WriteJSON(ClientFrame{Type: "boom", Ref: "2"})
	if e := readEnvelope(t, conn); e.Type != EventError || e.Err.Code != "internal_error" {
		t.Fatalf("panic must become an internal error frame: %+v", e)
	}
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":`))
	if e := readEnvelope(t, conn); e.Err == nil || e.Err.Code != "bad_request" {
		t.Fatalf("malformed: %+v", e)
	}
	// Oversized frames close the connection (1009 message too big).
	_ = conn.WriteMessage(websocket.TextMessage, []byte(strings.Repeat("x", 4096)))
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			var ce *websocket.CloseError
			if !errors.As(err, &ce) || ce.Code != CloseMessageTooBig {
				t.Fatalf("expected 1009, got %v", err)
			}
			break
		}
	}
	time.Sleep(100 * time.Millisecond)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.connected != 1 || h.disconnected != 1 {
		t.Fatalf("lifecycle: %+v", h)
	}
}

func TestServerResumeReplaysMissedEvents(t *testing.T) {
	_, srv, tickets, _, broker := newTestServer(t, nil)
	u := uuid.New()
	payload, _ := encode("message.created", map[string]int{"n": 1}, time.Now())
	first, _ := broker.Publish(context.Background(), u, payload)
	second, _ := broker.Publish(context.Background(), u, payload)

	tk, _ := tickets.Issue(context.Background(), u, "client", time.Minute)
	conn, _, err := dial(t, srv, tk, "http://app.test", first)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if hello := readEnvelope(t, conn); hello.Type != EventHello {
		t.Fatal("hello first")
	}
	if e := readEnvelope(t, conn); e.ID != second {
		t.Fatalf("replayed %q, want %q", e.ID, second)
	}
	// Live events continue after the replay.
	third, _ := broker.Publish(context.Background(), u, payload)
	if e := readEnvelope(t, conn); e.ID != third {
		t.Fatalf("live %q, want %q", e.ID, third)
	}
}

func TestServerFrameRateLimit(t *testing.T) {
	_, srv, tickets, _, _ := newTestServer(t, func(c *config.RealtimeConfig) { c.FrameRate, c.FrameBurst = 1, 1 })
	tk, _ := tickets.Issue(context.Background(), uuid.New(), "client", time.Minute)
	conn, _, err := dial(t, srv, tk, "http://app.test", "")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	readEnvelope(t, conn) // hello
	for i := 0; i < 40; i++ {
		_ = conn.WriteJSON(ClientFrame{Type: "echo", Data: json.RawMessage(`{}`)})
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	sawLimit := false
	for {
		var e Envelope
		if err := conn.ReadJSON(&e); err != nil {
			var ce *websocket.CloseError
			if errors.As(err, &ce) && ce.Code == ClosePolicyViolation && sawLimit {
				return
			}
			t.Fatalf("expected rate-limit close, got %v (saw limit %v)", err, sawLimit)
		}
		if e.Err != nil && e.Err.Code == "rate_limited" {
			sawLimit = true
		}
	}
}
