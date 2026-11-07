//go:build load

package load

// revisions 3.0: chat unread (spec §18).

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/seed"
)

// TestLoadChatUnreadCount adds the header badge's request to the signed-in
// mix: every page of a signed-in user calls it on load and on focus, so it
// runs at a higher rate than the other dashboard reads. It is cached per user
// (30 s) and backed by an index-only sum, so it must stay cheap.
func TestLoadChatUnreadCount(t *testing.T) {
	cfg := LoadConfig()
	RequireAPI(t, cfg)
	ctx := context.Background()
	pool := signedInClients(t, ctx, cfg, cfg.Users)
	fast := SLO{P95: 100 * time.Millisecond, P99: 250 * time.Millisecond, MaxErrRate: 0.001}
	heavy := SLO{P95: 150 * time.Millisecond, P99: 400 * time.Millisecond, MaxErrRate: 0.001}
	scenarios := []Scenario{
		{Name: "GET /chat/unread-count", Rate: cfg.Rate(200), Duration: cfg.Duration, Do: get("/api/v1/chat/unread-count"), SLO: fast},
		{Name: "GET /me", Rate: cfg.Rate(100), Duration: cfg.Duration, Do: get("/api/v1/me"), SLO: fast},
		{Name: "GET /chat/conversations", Rate: cfg.Rate(60), Duration: cfg.Duration, Do: get("/api/v1/chat/conversations"), SLO: heavy},
		{Name: "GET /me/notifications/unread-count", Rate: cfg.Rate(100), Duration: cfg.Duration, Do: get("/api/v1/me/notifications/unread-count"), SLO: fast},
	}
	results := runAll(ctx, scenarios, pool)
	Report(t, cfg, "chat-unread-count", results)
}

// TestLoadChatUnreadDelivery measures the time from sending a message to the
// recipient's chat.unread frame while LOAD_CHAT_PAIRS live chats (default 50)
// exchange LOAD_CHAT_MESSAGES messages (default 20) in parallel, with the
// unread-count endpoint polled on the side. The SLO matches message delivery.
func TestLoadChatUnreadDelivery(t *testing.T) {
	cfg := LoadConfig()
	RequireAPI(t, cfg)
	pairs, perPair := envInt("LOAD_CHAT_PAIRS", 50), envInt("LOAD_CHAT_MESSAGES", 20)
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("database: %v", err)
	}
	var advisors []struct{ Email, Slug string }
	db.Raw(`SELECT u.email, p.slug FROM psychics p JOIN users u ON u.id = p.user_id
		WHERE u.email LIKE ? AND u.status = 'active' AND p.listed
		  AND NOT EXISTS (SELECT 1 FROM chat_sessions s WHERE s.psychic_id = p.user_id AND s.status IN ('requested','accepted','active','disconnected'))
		ORDER BY u.email DESC LIMIT ?`, "load-%@"+seed.LoadEmailDomain, pairs).Scan(&advisors)
	var clients []string
	db.Raw(`SELECT u.email FROM users u JOIN wallets w ON w.user_id = u.id
		WHERE u.email LIKE ? AND u.role = 'client' AND u.status = 'active' AND w.balance >= 2000
		  AND NOT EXISTS (SELECT 1 FROM chat_sessions s WHERE s.client_id = u.id AND s.status IN ('requested','accepted','active','disconnected'))
		ORDER BY u.email DESC LIMIT ?`, "load-%@"+seed.LoadEmailDomain, pairs).Scan(&clients)
	n := min(len(advisors), len(clients))
	if n == 0 {
		t.Skip("no synthetic advisors/clients available (run cmd/seed -profile=large)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var (
		errs      atomic.Int64
		mu        sync.Mutex
		latencies []time.Duration
		samples   []string
	)
	fail := func(format string, args ...any) {
		errs.Add(1)
		mu.Lock()
		if len(samples) < 10 {
			samples = append(samples, fmt.Sprintf(format, args...))
		}
		mu.Unlock()
	}

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
			if p.advisorWS, err = dialSocket(ctx, p.advisor, nil); err != nil {
				fail("advisor socket: %v", err)
				return
			}
			if _, err := p.advisor.Do(ctx, http.MethodPatch, "/api/v1/psychic/availability", map[string]any{"availability": "online"}, nil); err != nil {
				fail("advisor online: %v", err)
				return
			}
			if p.clientWS, err = dialSocket(ctx, p.client, nil); err != nil {
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

	// Each pair alternates: the sender sends, the recipient's chat.unread
	// frame is timed, and the recipient reads (so the next message raises the
	// count again). Totals are polled over REST in parallel.
	started := time.Now()
	var sent atomic.Int64
	for _, p := range ps {
		if p == nil {
			continue
		}
		wg.Add(1)
		go func(p *chatPair) {
			defer wg.Done()
			for k := 0; k < perPair; k++ {
				sender, recipient, who := p.clientWS, p.advisorWS, p.advisor
				if k%2 == 1 {
					sender, recipient, who = p.advisorWS, p.clientWS, p.client
				}
				frame := recipient.expect(realtime.EventChatUnread)
				at := time.Now()
				sent.Add(1)
				if err := sender.request(realtime.FrameMessageSend, map[string]any{"session_id": p.session,
					"body": fmt.Sprintf("unread load %d", k), "client_msg_id": uuid.NewString()}); err != nil {
					fail("send: %v", err)
					recipient.types.Delete(realtime.EventChatUnread)
					continue
				}
				select {
				case <-frame:
					mu.Lock()
					latencies = append(latencies, time.Since(at))
					mu.Unlock()
				case <-time.After(5 * time.Second):
					fail("no chat.unread frame for message %d", k)
				}
				var tot struct{ Count int64 }
				if _, err := who.Do(ctx, http.MethodGet, "/api/v1/chat/unread-count", nil, &tot); err != nil {
					fail("unread-count: %v", err)
				} else if tot.Count < 1 {
					fail("unread-count %d right after a message", tot.Count)
				}
				time.Sleep(250 * time.Millisecond)
			}
		}(p)
	}
	wg.Wait()
	elapsed := time.Since(started)

	for _, p := range ps {
		if p == nil {
			continue
		}
		_, _ = p.client.Do(ctx, http.MethodPost, "/api/v1/chat/sessions/"+p.session+"/end", nil, nil)
		_, _ = p.advisor.Do(ctx, http.MethodPatch, "/api/v1/psychic/availability", map[string]any{"availability": "offline"}, nil)
		p.clientWS.close()
		p.advisorWS.close()
	}

	sort.Slice(latencies, func(a, b int) bool { return latencies[a] < latencies[b] })
	res := Result{Name: fmt.Sprintf("chat.unread delivery (%d sessions)", n), Rate: int(float64(sent.Load()) / max(elapsed.Seconds(), 1)),
		Duration: elapsed, Sent: sent.Load(), OK: int64(len(latencies)), Errors: errs.Load(), Status: map[int]int64{}, ErrorSamples: samples,
		SLO: SLO{P95: 250 * time.Millisecond, P99: 500 * time.Millisecond, MaxErrRate: 0.001}}
	if len(latencies) > 0 {
		res.P50, res.P90 = percentile(latencies, 0.50), percentile(latencies, 0.90)
		res.P95, res.P99, res.Max = percentile(latencies, 0.95), percentile(latencies, 0.99), latencies[len(latencies)-1]
		res.Throughput = float64(len(latencies)) / elapsed.Seconds()
	}
	res.Check()
	Report(t, cfg, "chat-unread-delivery", []Result{res})
}
