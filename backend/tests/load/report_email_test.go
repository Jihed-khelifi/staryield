//go:build load

package load

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// revisions 6.0 (§26.4): bursts of calculator report requests.
//
// Every request recalculates the chart (CPU), writes a lead and queues a sealed
// e-mail, so the endpoint is limited per IP and per address. Two modes:
//
//   - default (STARYIELD_RATE_LIMIT_SCALE=1000, as the other scenarios): every
//     request uses its own address and is accepted; the latency of the whole
//     path (recalculation, lead upsert, outbox row) is measured and the queue
//     must hold exactly one row per accepted request.
//
//   - LOAD_REPORT_LIMITS=1 with the API started at STARYIELD_RATE_LIMIT_SCALE=1:
//     the same burst must be refused by the limiter: at most 5 accepted (per IP
//     per hour), everything else 429, cheaply, and nothing queued for the
//     refused ones.
//
//     go test -tags load ./tests/load/ -run TestLoadReportEmail -v
func TestLoadReportEmail(t *testing.T) {
	cfg := LoadConfig()
	RequireAPI(t, cfg)
	ctx := context.Background()
	live, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("database %s: %v", cfg.DSN, err)
	}
	limits := os.Getenv("LOAD_REPORT_LIMITS") == "1"
	runID := time.Now().UnixNano() % 1_000_000
	start := time.Now().UTC()
	t.Cleanup(func() {
		// Dedicated load database: forget what this run created.
		live.Exec(`DELETE FROM email_leads WHERE created_at >= ?`, start)
		live.Exec(`DELETE FROM email_outbox WHERE template = 'calculator_report' AND created_at >= ?`, start)
	})
	pool := make([]*Client, 32)
	for i := range pool {
		pool[i] = NewClient(cfg.API, cfg.Origin)
	}
	inputs := []struct{ kind, input string }{
		{"natal-chart", `{"birth_date":"1990-03-14","birth_time":"08:45","place_id":"gn:2988507","house_system":"placidus","include":["tarot"]}`},
		{"astrology", `{"birth_date":"1990-08-23"}`},
		{"path-of-life", `{"birth_date":"1990-11-29","tz":"Europe/Paris"}`},
	}
	slo := SLO{P95: 400 * time.Millisecond, P99: 1500 * time.Millisecond, MaxErrRate: 0.001}
	do := func(ctx context.Context, c *Client, i int64) (int, error) {
		in := inputs[i%int64(len(inputs))]
		var raw map[string]any
		status, err := c.Do(ctx, http.MethodPost, "/api/v1/calculators/report-email", map[string]any{
			"kind": in.kind, "input": jsonRaw(in.input), "email": fmt.Sprintf("load-report-%06d-%d@example.com", runID, i),
			"locale": []string{"en", "fr", "es"}[i%3], "marketing_opt_in": i%2 == 0, "marketing_prechecked": false, "website": ""}, &raw)
		if limits && status == http.StatusTooManyRequests {
			return status, nil // the expected answer once the budget is spent
		}
		return status, err
	}
	results := runAll(ctx, []Scenario{{Name: "POST report-email (burst)", Rate: cfg.Rate(25), Duration: cfg.Duration, SLO: slo, Do: do}}, pool)
	Report(t, cfg, "report-email", results)

	accepted := results[0].Status[http.StatusAccepted]
	var queued, leads int64
	live.Raw(`SELECT count(*) FROM email_outbox WHERE template = 'calculator_report' AND created_at >= ?`, start).Row().Scan(&queued)
	live.Raw(`SELECT count(*) FROM email_leads WHERE created_at >= ?`, start).Row().Scan(&leads)
	t.Logf("accepted %d, refused %d, outbox rows %d, new leads %d", accepted, results[0].Status[http.StatusTooManyRequests], queued, leads)
	if queued != accepted || leads != accepted {
		t.Errorf("the queue must hold one row and one lead per accepted request: accepted %d, outbox %d, leads %d", accepted, queued, leads)
	}
	if limits && accepted > 5 {
		t.Errorf("the per-IP limit let %d requests through (5 per hour)", accepted)
	}
	if limits && results[0].Status[http.StatusTooManyRequests] == 0 {
		t.Error("the burst was never limited: start the API with STARYIELD_RATE_LIMIT_SCALE=1")
	}
}

// jsonRaw marshals a string that already is JSON.
type jsonRaw string

func (j jsonRaw) MarshalJSON() ([]byte, error) { return []byte(j), nil }
