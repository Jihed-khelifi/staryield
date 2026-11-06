package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
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

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
)

// brokers returns a Redis broker (miniredis) and an in-memory broker.
func brokers(t *testing.T) map[string]Broker {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return map[string]Broker{
		"redis":  NewRedisBroker(client, "t:", 1000, time.Hour, zap.NewNop()),
		"memory": NewMemoryBroker(1000),
	}
}

func seqPayload(seq int64) []byte {
	b, _ := encode(EventMessageCreated, map[string]int64{"seq": seq}, time.Unix(0, 0))
	return b
}

// streamSeqs returns the seq of every event in the user's stream, in order.
func streamSeqs(t *testing.T, b Broker, user uuid.UUID) []int64 {
	t.Helper()
	frames, _, err := b.Replay(context.Background(), user, "0-0", 10000)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	out := make([]int64, 0, len(frames))
	for _, f := range frames {
		var env struct {
			Data struct {
				Seq int64 `json:"seq"`
			} `json:"data"`
		}
		if err := json.Unmarshal(f.Frame, &env); err != nil {
			t.Fatalf("frame %s: %v", f.Frame, err)
		}
		out = append(out, env.Data.Seq)
	}
	return out
}

func TestOrderedGateHoldsReleasesAndFlushes(t *testing.T) {
	for name, b := range brokers(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			conv, a, c := uuid.New(), uuid.New(), uuid.New()
			now := time.Now()
			pub := func(seq int64, at time.Time) int {
				t.Helper()
				n, err := b.PublishOrdered(ctx, OrderedPublish{ConversationID: conv, Seq: seq, UserIDs: []uuid.UUID{a, c}, Payload: seqPayload(seq), Now: at})
				if err != nil {
					t.Fatalf("publish %d: %v", seq, err)
				}
				return n
			}
			if n := pub(1, now); n != 1 {
				t.Fatalf("first event of a new conversation: published %d", n)
			}
			if n := pub(3, now); n != 0 {
				t.Fatalf("event after a gap must be held back, published %d", n)
			}
			if n := pub(2, now); n != 2 {
				t.Fatalf("closing the gap releases 2 then 3, published %d", n)
			}
			if n := pub(2, now); n != 0 {
				t.Fatalf("duplicate published %d", n)
			}
			// 4 and 5 never arrive: 6 waits until the gap is flushed.
			if n := pub(6, now); n != 0 {
				t.Fatalf("held: %d", n)
			}
			if n, err := b.FlushGaps(ctx, now.Add(-time.Second)); err != nil || n != 0 {
				t.Fatalf("a recent gap must not be flushed: %d %v", n, err)
			}
			if n, err := b.FlushGaps(ctx, now.Add(GapFlushAge)); err != nil || n != 1 {
				t.Fatalf("flush: %d %v", n, err)
			}
			if n := pub(4, now); n != 0 {
				t.Fatalf("a late event below the flushed position is a duplicate: %d", n)
			}
			if n := pub(7, now); n != 1 {
				t.Fatalf("after the flush the gate continues: %d", n)
			}
			for _, u := range []uuid.UUID{a, c} {
				if got := fmt.Sprint(streamSeqs(t, b, u)); got != "[1 2 3 6 7]" {
					t.Fatalf("stream order for %s: %s", u, got)
				}
			}
			if n, _ := b.FlushGaps(ctx, now.Add(time.Hour)); n != 0 {
				t.Fatalf("nothing left to flush: %d", n)
			}
		})
	}
}

func TestOrderedGateConcurrentShuffledPublishes(t *testing.T) {
	for name, b := range brokers(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			conv, user := uuid.New(), uuid.New()
			// Seq 1 opens the gate; 2..300 then race in a random order the way
			// committed transactions race to publish.
			if _, err := b.PublishOrdered(ctx, OrderedPublish{ConversationID: conv, Seq: 1, UserIDs: []uuid.UUID{user}, Payload: seqPayload(1), Now: time.Now()}); err != nil {
				t.Fatal(err)
			}
			const n = 300
			seqs := rand.Perm(n - 1)
			var wg sync.WaitGroup
			var mu sync.Mutex
			var errs []error
			for w := 0; w < 8; w++ {
				wg.Add(1)
				go func(w int) {
					defer wg.Done()
					for i := w; i < len(seqs); i += 8 {
						s := int64(seqs[i] + 2)
						if _, err := b.PublishOrdered(ctx, OrderedPublish{ConversationID: conv, Seq: s, UserIDs: []uuid.UUID{user}, Payload: seqPayload(s), Now: time.Now()}); err != nil {
							mu.Lock()
							errs = append(errs, err)
							mu.Unlock()
						}
					}
				}(w)
			}
			wg.Wait()
			if len(errs) > 0 {
				t.Fatalf("publish errors: %v", errs[0])
			}
			got := streamSeqs(t, b, user)
			if len(got) != n {
				t.Fatalf("delivered %d events, want %d", len(got), n)
			}
			for i, s := range got {
				if s != int64(i+1) {
					t.Fatalf("event %d has seq %d: out of order", i, s)
				}
			}
		})
	}
}

func TestOrderedPublishRejectsInvalidSeq(t *testing.T) {
	for name, b := range brokers(t) {
		t.Run(name, func(t *testing.T) {
			if _, err := b.PublishOrdered(context.Background(), OrderedPublish{ConversationID: uuid.New(), Seq: 0, Payload: seqPayload(0)}); err == nil {
				t.Fatal("seq 0 accepted")
			}
		})
	}
}

func TestPublisherPublishOrdered(t *testing.T) {
	b := NewMemoryBroker(100)
	p := NewPublisher(b, nil)
	conv, a := uuid.New(), uuid.New()
	ctx := context.Background()
	if err := p.PublishOrdered(ctx, conv, 2, EventMessageCreated, map[string]int{"seq": 2}, a, uuid.Nil); err != nil {
		t.Fatal(err)
	}
	if err := p.PublishOrdered(ctx, conv, 4, EventMessageCreated, map[string]int{"seq": 4}, a); err != nil {
		t.Fatal(err)
	}
	if n, err := p.FlushGaps(ctx, -time.Second); err != nil || n != 1 {
		t.Fatalf("flush: %d %v", n, err)
	}
	if got := fmt.Sprint(streamSeqs(t, b, a)); got != "[2 4]" {
		t.Fatalf("stream: %s", got)
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func numSub(t *testing.T, client *redis.Client, channel string) int64 {
	t.Helper()
	res, err := client.PubSubNumSub(context.Background(), channel).Result()
	if err != nil {
		t.Fatalf("numsub: %v", err)
	}
	return res[channel]
}

func TestRedisBrokerSubscribesPerUser(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	b := NewRedisBroker(client, "t:", 100, time.Hour, zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got := make(chan Delivery, 32)
	go func() { _ = b.Run(ctx, func(d Delivery) { got <- d }) }()

	here, elsewhere := uuid.New(), uuid.New()
	// Two local connections of the same user share one subscription.
	if err := b.Subscribe(ctx, here); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if err := b.Subscribe(ctx, here); err != nil {
		t.Fatalf("second subscribe: %v", err)
	}
	if n := numSub(t, client, b.userChan(here)); n != 1 {
		t.Fatalf("one SUBSCRIBE per user, got %d", n)
	}
	if n := numPat(t, client); n != 0 {
		t.Fatalf("no pattern subscriptions expected, got %d", n)
	}
	payload, _ := encode("x", nil, time.Now())
	if _, err := b.Publish(ctx, elsewhere, payload); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Publish(ctx, here, payload); err != nil {
		t.Fatal(err)
	}
	select {
	case d := <-got:
		if d.UserID != here {
			t.Fatalf("received an event for a user not connected here: %+v", d)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no delivery for the connected user")
	}
	select {
	case d := <-got:
		t.Fatalf("unexpected extra delivery: %+v", d)
	case <-time.After(200 * time.Millisecond):
	}

	b.Unsubscribe(here)
	if numSub(t, client, b.userChan(here)) != 1 || b.SubscribedUsers() != 1 {
		t.Fatal("the subscription must survive while a connection remains")
	}
	b.Unsubscribe(here)
	waitFor(t, "unsubscribe", func() bool { return numSub(t, client, b.userChan(here)) == 0 })
	if b.SubscribedUsers() != 0 {
		t.Fatalf("subscribed users: %d", b.SubscribedUsers())
	}
	b.Unsubscribe(here) // extra releases are ignored
}

func numPat(t *testing.T, client *redis.Client) int64 {
	t.Helper()
	n, err := client.PubSubNumPat(context.Background()).Result()
	if err != nil {
		t.Fatalf("numpat: %v", err)
	}
	return n
}

func TestRedisBrokerSubscribeBeforeRunAndResyncAfterRestart(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	b := NewRedisBroker(client, "t:", 100, time.Hour, zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	u := uuid.New()

	// A connection arriving before the broker runs waits for the initial
	// subscription, which includes its user.
	subscribed := make(chan error, 1)
	go func() { subscribed <- b.Subscribe(ctx, u) }()
	time.Sleep(50 * time.Millisecond)
	got := make(chan Delivery, 64)
	go func() { _ = b.Run(ctx, func(d Delivery) { got <- d }) }()
	if err := <-subscribed; err != nil {
		t.Fatalf("subscribe before run: %v", err)
	}

	// Redis restarts: Pub/Sub messages published meanwhile are lost, so
	// local clients must be told to resync, and the subscription returns.
	mr.Close()
	time.Sleep(100 * time.Millisecond)
	if err := mr.Restart(); err != nil {
		t.Fatalf("restart: %v", err)
	}
	deadline := time.After(10 * time.Second)
	for resync := false; !resync; {
		select {
		case d := <-got:
			resync = d.Broadcast && d.Control == ControlResync
		case <-deadline:
			t.Fatal("no resync after the Redis connection came back")
		}
	}
	payload, _ := encode("after-restart", nil, time.Now())
	waitFor(t, "resubscription", func() bool { return numSub(t, client, b.userChan(u)) == 1 })
	if _, err := b.Publish(ctx, u, payload); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case d := <-got:
			if d.UserID == u && strings.Contains(string(d.Frame), "after-restart") {
				return
			}
		case <-time.After(5 * time.Second):
			t.Fatal("subscription not restored after restart")
		}
	}
}

func TestRedisBrokerSubscribeTimesOutWithoutRedis(t *testing.T) {
	// Nothing is listening: Run never connects, so the subscription cannot
	// be confirmed and the caller is told so (the connection then resyncs).
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1, DialTimeout: 50 * time.Millisecond})
	b := NewRedisBroker(client, "t:", 100, time.Hour, zap.NewNop())
	start := time.Now()
	if err := b.Subscribe(context.Background(), uuid.New()); err != ErrSubscribeTimeout {
		t.Fatalf("want ErrSubscribeTimeout, got %v", err)
	}
	if time.Since(start) < SubscribeTimeout {
		t.Fatal("returned before the timeout")
	}
}

func TestBrokerTail(t *testing.T) {
	for name, b := range brokers(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			u := uuid.New()
			if tail, err := b.Tail(ctx, u); err != nil || tail != "" {
				t.Fatalf("empty stream tail: %q %v", tail, err)
			}
			payload, _ := encode("x", nil, time.Now())
			_, _ = b.Publish(ctx, u, payload)
			id, _ := b.Publish(ctx, u, payload)
			if tail, err := b.Tail(ctx, u); err != nil || tail != id {
				t.Fatalf("tail %q, want %q (%v)", tail, id, err)
			}
		})
	}
}

func TestConnectLimited(t *testing.T) {
	mr := miniredis.RunT(t)
	rp := NewRedisPresence(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "t:", time.Minute)
	for name, p := range map[string]Presence{"memory": NewMemoryPresence(time.Minute, nil), "redis": rp} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			u := uuid.New()
			for _, c := range []string{"c1", "c2"} {
				if ok, err := p.ConnectLimited(ctx, c, u, 2); err != nil || !ok {
					t.Fatalf("%s: %v %v", c, ok, err)
				}
			}
			if ok, _ := p.ConnectLimited(ctx, "c3", u, 2); ok {
				t.Fatal("third connection admitted over the cap")
			}
			if ok, _ := p.ConnectLimited(ctx, "c1", u, 2); !ok {
				t.Fatal("re-registering a live connection must not count twice")
			}
			if ok, _ := p.ConnectLimited(ctx, "x1", uuid.New(), 2); !ok {
				t.Fatal("the cap is per user")
			}
			if online, _ := p.Online(ctx, []uuid.UUID{u}); !online[u] {
				t.Fatal("admitted connections count as online")
			}
			_, _ = p.Disconnect(ctx, "c1", u)
			if ok, _ := p.ConnectLimited(ctx, "c3", u, 2); !ok {
				t.Fatal("a freed slot must be reusable")
			}
			if ok, _ := p.ConnectLimited(ctx, "c4", u, 0); !ok {
				t.Fatal("max 0 means unlimited")
			}
		})
	}
	// Connections of a crashed instance expire and free their slots.
	u := uuid.New()
	ctx := context.Background()
	if ok, _ := rp.ConnectLimited(ctx, "old", u, 1); !ok {
		t.Fatal("first connection")
	}
	rp.now = func() time.Time { return time.Now().Add(2 * time.Minute) }
	if ok, _ := rp.ConnectLimited(ctx, "new", u, 1); !ok {
		t.Fatal("an expired connection must not hold a slot")
	}
}

// twoInstances serves the same users from two servers that share presence
// (like two API instances sharing Redis) but have their own hubs.
func twoInstances(t *testing.T, maxConns int) (srvA, srvB *httptest.Server, tickets TicketStore, broker *MemoryBroker) {
	t.Helper()
	cfg := config.RealtimeConfig{PingInterval: time.Second, PongWait: 3 * time.Second, WriteWait: time.Second, MaxFrameBytes: 1024,
		SendBuffer: 64, ReplayLimit: 100, MaxConnsPerUser: maxConns, PresenceTTL: time.Minute, FrameRate: 100, FrameBurst: 100}
	presence := NewMemoryPresence(time.Minute, nil)
	tickets = NewMemoryTickets(nil)
	broker = NewMemoryBroker(100)
	newSrv := func() *httptest.Server {
		hub := NewHub(zap.NewNop(), cfg.MaxConnsPerUser)
		s := NewServer(hub, broker, presence, tickets, &fakeHandler{}, nil, cfg, []string{"http://app.test"}, zap.NewNop())
		broker.SubscribeAll(hub.Dispatch)
		srv := httptest.NewServer(http.HandlerFunc(s.ServeWS))
		t.Cleanup(srv.Close)
		return srv
	}
	return newSrv(), newSrv(), tickets, broker
}

func TestServerGlobalConnectionCapAndSubscriptionLifecycle(t *testing.T) {
	srvA, srvB, tickets, broker := twoInstances(t, 1)
	u := uuid.New()
	ctx := context.Background()
	tailID, _ := broker.Publish(ctx, u, mustEncode(t, "x"))

	tkA, _ := tickets.Issue(ctx, u, "client", time.Minute)
	connA, _, err := dial(t, srvA, tkA, "http://app.test", "")
	if err != nil {
		t.Fatal(err)
	}
	hello := readEnvelope(t, connA)
	var h struct {
		StreamTail string `json:"stream_tail"`
	}
	_ = json.Unmarshal(hello.Data, &h)
	if hello.Type != EventHello || h.StreamTail != tailID {
		t.Fatalf("hello must carry the stream tail %q: %+v %s", tailID, hello, hello.Data)
	}
	waitFor(t, "subscription", func() bool { return broker.SubscribedUsers() == 1 })

	// A second connection on the other instance exceeds the cap of 1.
	tkB, _ := tickets.Issue(ctx, u, "client", time.Minute)
	connB, _, err := dial(t, srvB, tkB, "http://app.test", "")
	if err != nil {
		t.Fatal(err)
	}
	_ = connB.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, _, err := connB.ReadMessage(); err == nil {
		t.Fatal("expected the over-cap connection to be closed")
	} else if ce, ok := err.(*websocket.CloseError); !ok || ce.Code != CloseTooManyConns {
		t.Fatalf("over-cap close: %v", err)
	}

	// Closing the first connection releases its slot and its subscription.
	_ = connA.Close()
	waitFor(t, "unsubscribe", func() bool { return broker.SubscribedUsers() == 0 })
	waitFor(t, "slot released", func() bool {
		tk, _ := tickets.Issue(ctx, u, "client", time.Minute) // tickets are single use
		conn, _, err := dial(t, srvB, tk, "http://app.test", "")
		if err != nil {
			return false
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var e Envelope
		return conn.ReadJSON(&e) == nil && e.Type == EventHello
	})
}

// pendingBroker never confirms a user-channel subscription, as when Redis
// stalls during the handshake. The reference is still taken.
type pendingBroker struct{ *MemoryBroker }

func (b pendingBroker) Subscribe(ctx context.Context, userID uuid.UUID) error {
	_ = b.MemoryBroker.Subscribe(ctx, userID)
	return ErrSubscribeTimeout
}

func TestServerAsksForResyncWhenSubscriptionIsNotConfirmed(t *testing.T) {
	cfg := config.RealtimeConfig{PingInterval: time.Second, PongWait: 3 * time.Second, WriteWait: time.Second, MaxFrameBytes: 1024,
		SendBuffer: 64, ReplayLimit: 100, MaxConnsPerUser: 3, PresenceTTL: time.Minute, FrameRate: 100, FrameBurst: 100}
	mem := NewMemoryBroker(100)
	tickets := NewMemoryTickets(nil)
	hub := NewHub(zap.NewNop(), cfg.MaxConnsPerUser)
	s := NewServer(hub, pendingBroker{mem}, NewMemoryPresence(time.Minute, nil), tickets, &fakeHandler{}, nil, cfg,
		[]string{"http://app.test"}, zap.NewNop())
	mem.SubscribeAll(hub.Dispatch)
	srv := httptest.NewServer(http.HandlerFunc(s.ServeWS))
	t.Cleanup(srv.Close)

	u := uuid.New()
	tk, _ := tickets.Issue(context.Background(), u, "client", time.Minute)
	conn, _, err := dial(t, srv, tk, "http://app.test", "")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if hello := readEnvelope(t, conn); hello.Type != EventHello {
		t.Fatalf("hello first: %+v", hello)
	}
	// Events published before the subscription is confirmed may be missed,
	// so the client must reload its state.
	e := readEnvelope(t, conn)
	var data struct {
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(e.Data, &data)
	if e.Type != EventResyncRequired || data.Reason != "subscription_pending" {
		t.Fatalf("want resync_required(subscription_pending), got %+v %s", e, e.Data)
	}
	// The connection stays usable and its reference is released on close.
	_ = conn.WriteJSON(ClientFrame{Type: "echo", Ref: "1", Data: json.RawMessage(`{"v":1}`)})
	if ack := readEnvelope(t, conn); ack.Type != EventAck || ack.Ref != "1" {
		t.Fatalf("ack after resync: %+v", ack)
	}
	waitFor(t, "subscription reference", func() bool { return mem.SubscribedUsers() == 1 })
	_ = conn.Close()
	waitFor(t, "reference released", func() bool { return mem.SubscribedUsers() == 0 })
}

func mustEncode(t *testing.T, typ string) []byte {
	t.Helper()
	b, err := encode(typ, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return b
}
