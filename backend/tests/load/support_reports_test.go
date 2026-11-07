//go:build load

package load

// revisions 3.1: conversation reports. Many distinct signed-in clients that
// own a conversation file a report (POST) and poll for it (GET). The admin
// queue with kind=chat_report needs a TOTP-protected console session, so its
// plans are covered by TestExplainHotQueries ("support: reports …").
//
// Reports are limited to 10 per hour per user (support_report): run the API
// with STARYIELD_RATE_LIMIT_SCALE=1000 as described in the README. The dataset
// comes from the large seed (clients with conversations); the "chat reports"
// generator adds the reports the admin-queue plans are checked against.

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/seed"
)

func TestLoadChatReports(t *testing.T) {
	cfg := LoadConfig()
	RequireAPI(t, cfg)
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("database: %v", err)
	}
	want := envInt("LOAD_REPORT_CLIENTS", 300)
	var rows []struct {
		Email string
		Conv  uuid.UUID
	}
	// Distinct synthetic clients (one conversation each) with no open or pending report yet.
	db.Raw(`SELECT DISTINCT ON (u.id) u.email, c.id AS conv FROM conversations c JOIN users u ON u.id = c.client_id
		WHERE u.email LIKE ? AND u.role = 'client' AND u.status = 'active'
		  AND NOT EXISTS (SELECT 1 FROM support_tickets t WHERE t.user_id = u.id AND t.conversation_id = c.id AND t.kind = 'chat_report'
		                  AND t.status IN ('open','pending') AND t.deleted_at IS NULL)
		ORDER BY u.id, c.id DESC LIMIT ?`, "load-%@"+seed.LoadEmailDomain, want).Scan(&rows)
	if len(rows) == 0 {
		t.Skip("no synthetic clients with conversations (run cmd/seed -profile=large)")
	}
	ctx := context.Background()
	var (
		pool []*Client
		conv = map[*Client]string{}
		mu   sync.Mutex
	)
	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)
	for _, r := range rows {
		wg.Add(1)
		sem <- struct{}{}
		go func(email string, id uuid.UUID) {
			defer wg.Done()
			defer func() { <-sem }()
			c := NewClient(cfg.API, cfg.Origin)
			if err := c.Login(ctx, email, seed.LoadPassword); err != nil {
				return
			}
			mu.Lock()
			pool = append(pool, c)
			conv[c] = id.String()
			mu.Unlock()
		}(r.Email, r.Conv)
	}
	wg.Wait()
	if len(pool) == 0 {
		t.Skip("could not sign in any synthetic client")
	}
	t.Logf("%d distinct clients with a conversation", len(pool))
	reasons := []string{"advisor_unresponsive", "inappropriate", "billing", "technical", "other"}
	fast := SLO{P95: 100 * time.Millisecond, P99: 250 * time.Millisecond, MaxErrRate: 0.001}
	writes := SLO{P95: 200 * time.Millisecond, P99: 500 * time.Millisecond, MaxErrRate: 0.001}
	scenarios := []Scenario{
		// The first report of a client is created (201); later ones answer
		// 409 report_exists (served: the one-open-report rule held).
		{Name: "POST /me/support/chat-reports", Rate: cfg.Rate(20), Duration: cfg.Duration, SLO: writes,
			Do: func(ctx context.Context, c *Client, i int64) (int, error) {
				status, err := c.Do(ctx, http.MethodPost, "/api/v1/me/support/chat-reports", map[string]any{
					"conversation_id": conv[c], "reason": reasons[int(i)%len(reasons)], "details": "Filed by the load test to exercise conversation reports.", "seq": i % 50}, nil)
				if apiErr, ok := err.(*APIError); ok && apiErr.Code == "report_exists" {
					return http.StatusOK, nil
				}
				return status, err
			}},
		{Name: "GET /me/support/chat-reports", Rate: cfg.Rate(120), Duration: cfg.Duration, SLO: fast,
			Do: func(ctx context.Context, c *Client, i int64) (int, error) {
				return c.Get(ctx, "/api/v1/me/support/chat-reports?conversation_id="+conv[c])
			}},
		{Name: "GET /me/support/tickets (with reports)", Rate: cfg.Rate(60), Duration: cfg.Duration, SLO: fast, Do: get("/api/v1/me/support/tickets?limit=20")},
	}
	results := runAll(ctx, scenarios, pool)
	Report(t, cfg, "support-chat-reports", results)
}
