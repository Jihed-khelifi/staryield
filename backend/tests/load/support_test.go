//go:build load

package load

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// revisions 3.0: support desk. Signed-in requesters list their tickets, open
// one and reply (the contact form creates the tickets). Staff replies and the
// admin queue need a TOTP-protected console session; their query plans are
// covered by TestExplainHotQueries ("support: …").
//
// Replies are limited to 20 per hour per user (support_reply): run the API
// with STARYIELD_RATE_LIMIT_SCALE=1000 as described in the README.
func TestLoadSupportRequesters(t *testing.T) {
	cfg := LoadConfig()
	RequireAPI(t, cfg)
	ctx := context.Background()
	pool := signedInClients(t, ctx, cfg, cfg.Users)

	// Every virtual user files two requests (the creation limit is 5 per hour).
	var mu sync.Mutex
	refs := map[*Client][]string{}
	run := time.Now().UnixNano()
	for i, c := range pool {
		for k := 0; k < 2; k++ {
			var receipt struct {
				Reference string `json:"reference"`
			}
			_, err := c.Do(ctx, http.MethodPost, "/api/v1/support/tickets", map[string]any{
				"email": fmt.Sprintf("support-load-%d-%d@example.test", run, i), "topic": "general",
				"subject": fmt.Sprintf("Load test request %d", k), "message": "A request filed by the load test to exercise the support desk.",
				"consent": true}, &receipt)
			if err != nil {
				t.Logf("create ticket (user %d): %v", i, err)
				continue
			}
			mu.Lock()
			refs[c] = append(refs[c], receipt.Reference)
			mu.Unlock()
		}
	}
	if len(refs) == 0 {
		t.Skip("no ticket could be created (is the contact-form rate limit scaled up?)")
	}
	ref := func(c *Client, i int64) string {
		mu.Lock()
		defer mu.Unlock()
		r := refs[c]
		if len(r) == 0 {
			return "SY-AAAAAA"
		}
		return r[int(i)%len(r)]
	}
	fast := SLO{P95: 100 * time.Millisecond, P99: 250 * time.Millisecond, MaxErrRate: 0.001}
	writes := SLO{P95: 200 * time.Millisecond, P99: 500 * time.Millisecond, MaxErrRate: 0.001}
	scenarios := []Scenario{
		{Name: "GET /me/support/tickets", Rate: cfg.Rate(60), Duration: cfg.Duration, Do: get("/api/v1/me/support/tickets?limit=20"), SLO: fast},
		{Name: "GET /me/support/tickets/:reference", Rate: cfg.Rate(80), Duration: cfg.Duration, SLO: fast,
			Do: func(ctx context.Context, c *Client, i int64) (int, error) {
				return c.Get(ctx, "/api/v1/me/support/tickets/"+ref(c, i))
			}},
		// A thread holds at most 200 messages (409 limit_reached): a full
		// thread counts as served, so long or scaled runs do not fail on it.
		{Name: "POST /me/support/tickets/:reference/messages", Rate: cfg.Rate(10), Duration: cfg.Duration, SLO: writes,
			Do: func(ctx context.Context, c *Client, i int64) (int, error) {
				status, err := c.Do(ctx, http.MethodPost, "/api/v1/me/support/tickets/"+ref(c, i)+"/messages",
					map[string]any{"body": fmt.Sprintf("Load test reply %d at %s", i, time.Now().Format(time.RFC3339Nano))}, nil)
				if apiErr, ok := err.(*APIError); ok && strings.Contains(apiErr.Code, "limit_reached") {
					return http.StatusOK, nil
				}
				return status, err
			}},
		// Private-link lookups with a wrong token: the same cheap 404 for everyone.
		{Name: "GET /support/tickets/:reference (wrong token)", Rate: cfg.Rate(20), Duration: cfg.Duration, SLO: fast,
			Do: func(ctx context.Context, c *Client, i int64) (int, error) {
				status, err := c.Get(ctx, "/api/v1/support/tickets/"+ref(c, i))
				if apiErr, ok := err.(*APIError); ok && apiErr.Status == http.StatusNotFound {
					return http.StatusOK, nil
				}
				return status, err
			}},
	}
	results := runAll(ctx, scenarios, pool)
	Report(t, cfg, "support-requesters", results)
}
