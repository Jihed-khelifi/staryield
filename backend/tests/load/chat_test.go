//go:build load

package load

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/seed"
)

func envInt(key string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key))); err == nil && n > 0 {
		return n
	}
	return def
}

// socket is one realtime connection; every message.created event it
// receives is reported to onMessage.
type socket struct {
	conn  *websocket.Conn
	mu    sync.Mutex
	refN  int
	acks  sync.Map // ref -> chan realtime.Envelope
	types sync.Map // event type -> chan realtime.Envelope (first waiter)
	done  chan struct{}
}

func dialSocket(ctx context.Context, c *Client, onMessage func(realtime.Envelope)) (*socket, error) {
	var tk struct {
		Ticket string `json:"ticket"`
	}
	if _, err := c.Do(ctx, http.MethodPost, "/api/v1/realtime/ticket", nil, &tk); err != nil {
		return nil, fmt.Errorf("ticket: %w", err)
	}
	u := "ws" + strings.TrimPrefix(c.base, "http") + "/api/v1/realtime/ws?ticket=" + url.QueryEscape(tk.Ticket)
	h := http.Header{}
	h.Set("Origin", c.origin)
	d := websocket.Dialer{Subprotocols: []string{realtime.Subprotocol}, HandshakeTimeout: 10 * time.Second}
	conn, _, err := d.DialContext(ctx, u, h)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	s := &socket{conn: conn, done: make(chan struct{})}
	hello := s.expect(realtime.EventHello)
	go func() {
		defer close(s.done)
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var env realtime.Envelope
			if json.Unmarshal(data, &env) != nil {
				continue
			}
			if env.Ref != "" {
				if ch, ok := s.acks.LoadAndDelete(env.Ref); ok {
					ch.(chan realtime.Envelope) <- env
					continue
				}
			}
			if env.Type == realtime.EventMessageCreated && onMessage != nil {
				onMessage(env)
			}
			if ch, ok := s.types.LoadAndDelete(env.Type); ok {
				ch.(chan realtime.Envelope) <- env
			}
		}
	}()
	select {
	case <-hello:
		return s, nil
	case <-time.After(10 * time.Second):
		_ = conn.Close()
		return nil, fmt.Errorf("no hello")
	}
}

// expect registers interest in the next event of a type.
func (s *socket) expect(typ string) <-chan realtime.Envelope {
	ch := make(chan realtime.Envelope, 1)
	s.types.Store(typ, ch)
	return ch
}

// request sends a frame and waits for its ack.
func (s *socket) request(typ string, data any) error {
	s.mu.Lock()
	s.refN++
	ref := fmt.Sprintf("r%d", s.refN)
	ch := make(chan realtime.Envelope, 1)
	s.acks.Store(ref, ch)
	raw, _ := json.Marshal(data)
	err := s.conn.WriteJSON(realtime.ClientFrame{Type: typ, Ref: ref, Data: raw})
	s.mu.Unlock()
	if err != nil {
		return err
	}
	select {
	case e := <-ch:
		if e.Type != realtime.EventAck {
			return fmt.Errorf("%s: %s", typ, e.Data)
		}
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("%s: no ack", typ)
	}
}

func (s *socket) close() { _ = s.conn.Close(); <-s.done }

type chatPair struct {
	advisor, client     *Client
	advisorWS, clientWS *socket
	slug                string
	session             string
}

// TestLoadChatFanout runs live chats between synthetic advisors and clients
// of the large seed: LOAD_CHAT_PAIRS sessions (default 50) exchange
// LOAD_CHAT_MESSAGES messages per side (default 20). The SLO is on the time
// from sending a message to its delivery on the other side.
func TestLoadChatFanout(t *testing.T) {
	cfg := LoadConfig()
	RequireAPI(t, cfg)
	pairs, perSide := envInt("LOAD_CHAT_PAIRS", 50), envInt("LOAD_CHAT_MESSAGES", 20)
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("database: %v", err)
	}
	// Synthetic advisors and clients with no open session; clients with
	// enough credits for a few minutes.
	var advisors []struct{ Email, Slug string }
	db.Raw(`SELECT u.email, p.slug FROM psychics p JOIN users u ON u.id = p.user_id
		WHERE u.email LIKE ? AND u.status = 'active' AND p.listed
		  AND NOT EXISTS (SELECT 1 FROM chat_sessions s WHERE s.psychic_id = p.user_id AND s.status IN ('requested','accepted','active','disconnected'))
		ORDER BY u.email LIMIT ?`, "load-%@"+seed.LoadEmailDomain, pairs).Scan(&advisors)
	var clients []string
	db.Raw(`SELECT u.email FROM users u JOIN wallets w ON w.user_id = u.id
		WHERE u.email LIKE ? AND u.role = 'client' AND u.status = 'active' AND w.balance >= 2000
		  AND NOT EXISTS (SELECT 1 FROM chat_sessions s WHERE s.client_id = u.id AND s.status IN ('requested','accepted','active','disconnected'))
		ORDER BY u.email LIMIT ?`, "load-%@"+seed.LoadEmailDomain, pairs).Scan(&clients)
	n := min(len(advisors), len(clients))
	if n == 0 {
		t.Skip("no synthetic advisors/clients available (run cmd/seed -profile=large)")
	}
	t.Logf("chat fan-out: %d pairs, %d messages per side", n, perSide)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var (
		sentAt    sync.Map // client_msg_id -> time.Time
		latMu     sync.Mutex
		latencies []time.Duration
		delivered atomic.Int64
		errs      atomic.Int64
		samples   []string
		sampleMu  sync.Mutex
	)
	fail := func(format string, args ...any) {
		errs.Add(1)
		sampleMu.Lock()
		if len(samples) < 10 {
			samples = append(samples, fmt.Sprintf(format, args...))
		}
		sampleMu.Unlock()
	}
	onMessage := func(e realtime.Envelope) {
		var p struct {
			Message struct {
				ClientMsgID string `json:"client_msg_id"`
			} `json:"message"`
		}
		if json.Unmarshal(e.Data, &p) != nil || p.Message.ClientMsgID == "" {
			return
		}
		// Each message is delivered to both participants; count the
		// counterpart's copy only (the sender's echo also arrives).
		if at, ok := sentAt.LoadAndDelete(p.Message.ClientMsgID); ok {
			latMu.Lock()
			latencies = append(latencies, time.Since(at.(time.Time)))
			latMu.Unlock()
			delivered.Add(1)
		}
	}

	// Set up every pair concurrently.
	ps := make([]*chatPair, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p := &chatPair{advisor: NewClient(cfg.API, cfg.Origin), client: NewClient(cfg.API, cfg.Origin), slug: advisors[i].Slug}
			if err := p.advisor.Login(ctx, advisors[i].Email, seed.LoadPassword); err != nil {
				fail("advisor login: %v", err)
				return
			}
			if err := p.client.Login(ctx, clients[i], seed.LoadPassword); err != nil {
				fail("client login: %v", err)
				return
			}
			var err error
			if p.advisorWS, err = dialSocket(ctx, p.advisor, onMessage); err != nil {
				fail("advisor socket: %v", err)
				return
			}
			if _, err := p.advisor.Do(ctx, http.MethodPatch, "/api/v1/psychic/availability", map[string]any{"availability": "online"}, nil); err != nil {
				fail("advisor online: %v", err)
				return
			}
			if p.clientWS, err = dialSocket(ctx, p.client, onMessage); err != nil {
				fail("client socket: %v", err)
				return
			}
			var sess struct {
				ID string `json:"id"`
			}
			if _, err := p.client.Do(ctx, http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": p.slug, "request_key": uuid.NewString()}, &sess); err != nil {
				fail("request session: %v", err)
				return
			}
			p.session = sess.ID
			startedC, startedA := p.clientWS.expect(realtime.EventSessionStarted), p.advisorWS.expect(realtime.EventSessionStarted)
			if err := p.clientWS.request(realtime.FrameSessionJoin, map[string]any{"session_id": sess.ID}); err != nil {
				fail("client join: %v", err)
				return
			}
			if _, err := p.advisor.Do(ctx, http.MethodPost, "/api/v1/chat/sessions/"+sess.ID+"/accept", nil, nil); err != nil {
				fail("accept: %v", err)
				return
			}
			if err := p.advisorWS.request(realtime.FrameSessionJoin, map[string]any{"session_id": sess.ID}); err != nil {
				fail("advisor join: %v", err)
				return
			}
			for _, ch := range []<-chan realtime.Envelope{startedC, startedA} {
				select {
				case <-ch:
				case <-time.After(15 * time.Second):
					fail("session did not start")
					return
				}
			}
			ps[i] = p
		}(i)
	}
	wg.Wait()

	// Exchange messages: both sides send, one message every 250 ms.
	started := time.Now()
	var sent atomic.Int64
	for _, p := range ps {
		if p == nil {
			continue
		}
		for _, side := range []*socket{p.clientWS, p.advisorWS} {
			wg.Add(1)
			go func(p *chatPair, s *socket) {
				defer wg.Done()
				for k := 0; k < perSide; k++ {
					id := uuid.NewString()
					sentAt.Store(id, time.Now())
					sent.Add(1)
					if err := s.request(realtime.FrameMessageSend, map[string]any{"session_id": p.session,
						"body": fmt.Sprintf("load message %d", k), "client_msg_id": id}); err != nil {
						sentAt.Delete(id)
						fail("send: %v", err)
					}
					time.Sleep(250 * time.Millisecond)
				}
			}(p, side)
		}
	}
	wg.Wait()
	// Let the last deliveries arrive.
	deadline := time.Now().Add(5 * time.Second)
	for delivered.Load()+errs.Load() < sent.Load() && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	elapsed := time.Since(started)

	// End the sessions and go offline.
	for _, p := range ps {
		if p == nil {
			continue
		}
		_, _ = p.client.Do(ctx, http.MethodPost, "/api/v1/chat/sessions/"+p.session+"/end", nil, nil)
		_, _ = p.advisor.Do(ctx, http.MethodPatch, "/api/v1/psychic/availability", map[string]any{"availability": "offline"}, nil)
		p.clientWS.close()
		p.advisorWS.close()
	}

	lost := sent.Load() - delivered.Load()
	sort.Slice(latencies, func(a, b int) bool { return latencies[a] < latencies[b] })
	res := Result{Name: fmt.Sprintf("chat delivery (%d sessions)", n), Rate: int(float64(sent.Load()) / max(elapsed.Seconds(), 1)),
		Duration: elapsed, Sent: sent.Load(), OK: delivered.Load(), Errors: errs.Load() + max(lost-errs.Load(), 0),
		Status: map[int]int64{}, ErrorSamples: samples,
		SLO: SLO{P95: 250 * time.Millisecond, P99: 500 * time.Millisecond, MaxErrRate: 0.001}}
	if len(latencies) > 0 {
		res.P50, res.P90 = percentile(latencies, 0.50), percentile(latencies, 0.90)
		res.P95, res.P99, res.Max = percentile(latencies, 0.95), percentile(latencies, 0.99), latencies[len(latencies)-1]
		res.Throughput = float64(len(latencies)) / elapsed.Seconds()
	}
	res.Check()
	Report(t, cfg, "chat-fanout", []Result{res})
}
