//go:build load

package load

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// TestLoadDirectoryUnderStatusChurn (revisions 6.0, §26.3): the public
// directory and home selection keep their latency while an advisor's status
// keeps changing. Every status change drops the in-memory snapshot's cached
// order, so the next read re-sorts it: this is the worst case for the order.
func TestLoadDirectoryUnderStatusChurn(t *testing.T) {
	cfg := LoadConfig()
	RequireAPI(t, cfg)
	ctx := context.Background()
	psychic := NewClient(cfg.API, cfg.Origin)
	if err := psychic.Login(ctx, cfg.PsychicEmail, cfg.PsychicPassword); err != nil {
		t.Fatalf("advisor login (%s): %v", cfg.PsychicEmail, err)
	}
	pool := make([]*Client, 32)
	for i := range pool {
		pool[i] = NewClient(cfg.API, cfg.Origin)
	}
	fast := SLO{P95: 100 * time.Millisecond, P99: 250 * time.Millisecond, MaxErrRate: 0.001}
	// The availability switch is rate-limited per advisor: stay well under it.
	flips := SLO{P95: 200 * time.Millisecond, P99: 500 * time.Millisecond, MaxErrRate: 0.001}
	// Busy is not settable: it is derived from a live reading (§9.4).
	availability := []string{"online", "away", "offline", "online"}
	scenarios := []Scenario{
		{Name: "GET /psychics", Rate: cfg.Rate(150), Duration: cfg.Duration, Do: get("/api/v1/psychics?page_size=24"), SLO: fast},
		{Name: "GET /psychics (online)", Rate: cfg.Rate(100), Duration: cfg.Duration, Do: get("/api/v1/psychics?status=online"), SLO: fast},
		{Name: "GET /psychics/featured", Rate: cfg.Rate(200), Duration: cfg.Duration, Do: get("/api/v1/psychics/featured"), SLO: fast},
		{Name: "PATCH /psychic/availability", Rate: cfg.Rate(2), Duration: cfg.Duration, SLO: flips,
			Do: func(ctx context.Context, c *Client, i int64) (int, error) {
				return c.Do(ctx, http.MethodPatch, "/api/v1/psychic/availability",
					map[string]any{"availability": availability[i%int64(len(availability))]}, nil)
			}},
	}
	results := make([]Result, len(scenarios))
	done := make(chan int, len(scenarios))
	for i := range scenarios {
		go func(i int) {
			clients := pool
			if i == len(scenarios)-1 {
				clients = []*Client{psychic}
			}
			results[i] = Run(ctx, scenarios[i], clients)
			done <- i
		}(i)
	}
	for range scenarios {
		<-done
	}
	Report(t, cfg, "directory-status-churn", results)
}
