//go:build load

package load

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/seed"
)

// likePair is one synthetic client and advisor with a conversation and the
// message both of them like and unlike.
type likePair struct {
	advisor, client     *Client
	advisorWS, clientWS *socket
	message             string
}

// TestLoadLikeFanout (revisions 4.0, §23.3): LOAD_LIKE_PAIRS synthetic
// client/advisor pairs (default 20) of the large seed, each with both sockets
// open, toggle a like on one message LOAD_LIKE_ROUNDS times (default 20):
// the client likes and waits for the advisor's message.liked, the advisor
// unlikes and waits for the client's. The SLO is on the time from the request
// to the delivery of the event on the other side. Run
// `go run ./cmd/seed -profile=large -extend "message replies and likes"` first
// to have the thread carry realistic quotes and likes.
func TestLoadLikeFanout(t *testing.T) {
	cfg := LoadConfig()
	RequireAPI(t, cfg)
	pairs, rounds := envInt("LOAD_LIKE_PAIRS", 20), envInt("LOAD_LIKE_ROUNDS", 20)
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("database: %v", err)
	}
	var found []struct{ ClientEmail, AdvisorEmail, Conv string }
	db.Raw(`SELECT cu.email AS client_email, pu.email AS advisor_email, c.id::text AS conv
		FROM conversations c JOIN users cu ON cu.id = c.client_id JOIN users pu ON pu.id = c.psychic_id
		WHERE cu.email LIKE ? AND pu.email LIKE ? AND cu.status = 'active' AND pu.status = 'active' AND c.last_seq >= 5
		ORDER BY c.last_activity_at DESC LIMIT ?`, "load-%@"+seed.LoadEmailDomain, "load-%@"+seed.LoadEmailDomain, pairs).Scan(&found)
	if len(found) == 0 {
		t.Skip("no synthetic conversations available (run cmd/seed -profile=large)")
	}
	t.Logf("like fan-out: %d pairs, %d rounds", len(found), rounds)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var (
		latMu     sync.Mutex
		latencies []time.Duration
		errs      atomic.Int64
		sent      atomic.Int64
		sampleMu  sync.Mutex
		samples   []string
	)
	fail := func(format string, args ...any) {
		errs.Add(1)
		sampleMu.Lock()
		if len(samples) < 10 {
			samples = append(samples, fmt.Sprintf(format, args...))
		}
		sampleMu.Unlock()
	}

	ps := make([]*likePair, len(found))
	var wg sync.WaitGroup
	for i := range found {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p := &likePair{advisor: NewClient(cfg.API, cfg.Origin), client: NewClient(cfg.API, cfg.Origin)}
			if err := p.advisor.Login(ctx, found[i].AdvisorEmail, seed.LoadPassword); err != nil {
				fail("advisor login: %v", err)
				return
			}
			if err := p.client.Login(ctx, found[i].ClientEmail, seed.LoadPassword); err != nil {
				fail("client login: %v", err)
				return
			}
			var page struct {
				Items []struct {
					ID   string `json:"id"`
					Kind string `json:"kind"`
				} `json:"items"`
			}
			if _, err := p.client.Do(ctx, http.MethodGet, "/api/v1/chat/conversations/"+found[i].Conv+"/messages?limit=20", nil, &page); err != nil {
				fail("history: %v", err)
				return
			}
			for _, m := range page.Items {
				if m.Kind == "text" {
					p.message = m.ID
					break
				}
			}
			if p.message == "" {
				fail("no text message in %s", found[i].Conv)
				return
			}
			var err error
			if p.advisorWS, err = dialSocket(ctx, p.advisor, nil); err != nil {
				fail("advisor socket: %v", err)
				return
			}
			if p.clientWS, err = dialSocket(ctx, p.client, nil); err != nil {
				fail("client socket: %v", err)
				return
			}
			ps[i] = p
		}(i)
	}
	wg.Wait()

	// toggle sends the request and waits for the event on the other side.
	toggle := func(from *Client, to *socket, method, message string) {
		arrived := to.expect(realtime.EventMessageLiked)
		started := time.Now()
		sent.Add(1)
		if _, err := from.Do(ctx, method, "/api/v1/chat/messages/"+message+"/like", nil, nil); err != nil {
			fail("%s like: %v", method, err)
			return
		}
		select {
		case <-arrived:
			latMu.Lock()
			latencies = append(latencies, time.Since(started))
			latMu.Unlock()
		case <-time.After(10 * time.Second):
			fail("%s like: no message.liked event", method)
		}
	}
	started := time.Now()
	for _, p := range ps {
		if p == nil {
			continue
		}
		wg.Add(1)
		go func(p *likePair) {
			defer wg.Done()
			// The seed may have liked this message already, and an event is
			// only published when the state changes: start from "nobody likes
			// it", then let the events of that reset pass.
			for _, c := range []*Client{p.client, p.advisor} {
				if _, err := c.Do(ctx, http.MethodDelete, "/api/v1/chat/messages/"+p.message+"/like", nil, nil); err != nil {
					fail("reset like: %v", err)
					return
				}
			}
			time.Sleep(500 * time.Millisecond)
			// Each participant likes and then unlikes their own like, so every
			// request changes the state and the other side must hear of it.
			for k := 0; k < rounds; k++ {
				toggle(p.client, p.advisorWS, http.MethodPut, p.message)
				toggle(p.client, p.advisorWS, http.MethodDelete, p.message)
				toggle(p.advisor, p.clientWS, http.MethodPut, p.message)
				toggle(p.advisor, p.clientWS, http.MethodDelete, p.message)
				time.Sleep(250 * time.Millisecond)
			}
		}(p)
	}
	wg.Wait()
	elapsed := time.Since(started)
	for _, p := range ps {
		if p != nil {
			p.clientWS.close()
			p.advisorWS.close()
		}
	}

	sort.Slice(latencies, func(a, b int) bool { return latencies[a] < latencies[b] })
	res := Result{Name: fmt.Sprintf("like fan-out (%d pairs)", len(found)), Rate: int(float64(sent.Load()) / max(elapsed.Seconds(), 1)),
		Duration: elapsed, Sent: sent.Load(), OK: int64(len(latencies)), Errors: errs.Load(), Status: map[int]int64{}, ErrorSamples: samples,
		SLO: SLO{P95: 250 * time.Millisecond, P99: 500 * time.Millisecond, MaxErrRate: 0.001}}
	if len(latencies) > 0 {
		res.P50, res.P90 = percentile(latencies, 0.50), percentile(latencies, 0.90)
		res.P95, res.P99, res.Max = percentile(latencies, 0.95), percentile(latencies, 0.99), latencies[len(latencies)-1]
		res.Throughput = float64(len(latencies)) / elapsed.Seconds()
	}
	res.Check()
	Report(t, cfg, "like-fanout", []Result{res})
}
