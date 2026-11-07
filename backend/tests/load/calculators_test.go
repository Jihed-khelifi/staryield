//go:build load

package load

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
	"os"
	"strings"
)

// revisions 3.0: calculator analytics.
//
// TestLoadCalculatorIngestion posts batched browser analytics events (page
// views of the three calculators in every locale, tries, results and campaign
// landings) at a high arrival rate and checks the ingestion latency (the roll-up
// is asynchronous and must never slow the request down), then checks that the
// events reach web_visitors / calculator_visits in a bounded time. Meanwhile
// the admin reports are read straight from the database (the admin session
// needs a TOTP code, so the service is driven directly) on top of the large
// seed ("calculator visitors"):
//
//	go run ./cmd/seed -profile=large -extend "calculator visitors"
//	go test -tags load ./tests/load/ -run TestLoadCalculatorIngestion -v

var (
	calcKinds   = []string{"natal-chart", "path-of-life", "astrology"}
	calcLocales = []string{"en", "fr", "es"}
	calcSources = []string{"instagram", "tiktok", "newsletter", "google", "facebook"}
)

// visitorPool is the number of distinct anonymous visitors the run cycles through.
const visitorPool = 200_000

func loadEvent(name, anon, path, locale string, props map[string]any) map[string]any {
	ev := map[string]any{"event_id": uuid.NewString(), "name": name, "occurred_at": time.Now().UTC().Format(time.RFC3339Nano),
		"anonymous_id": anon, "path": path, "locale": locale}
	if props != nil {
		ev["properties"] = props
	}
	return ev
}

func postEvents(events func(i int64) []map[string]any) Request {
	return func(ctx context.Context, c *Client, i int64) (int, error) {
		return c.Do(ctx, http.MethodPost, "/api/v1/analytics/events", map[string]any{"events": events(i)}, nil)
	}
}

func TestLoadCalculatorIngestion(t *testing.T) {
	cfg := LoadConfig()
	RequireAPI(t, cfg)
	ctx := context.Background()
	runID := time.Now().UnixNano() % 1_000_000
	anon := func(i int64) string { return fmt.Sprintf("ld%06d-%07d", runID, i%visitorPool) }

	pool := make([]*Client, 64)
	for i := range pool {
		pool[i] = NewClient(cfg.API, cfg.Origin)
	}
	live, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("database %s: %v", cfg.DSN, err)
	}
	var before int64
	live.Raw(`SELECT count(*) FROM web_visitors`).Row().Scan(&before)

	ingest := SLO{P95: 100 * time.Millisecond, P99: 300 * time.Millisecond, MaxErrRate: 0.001}
	reports := SLO{P95: 5 * time.Second, P99: 10 * time.Second, MaxErrRate: 0.001}
	svc := service.NewCalculatorService(repository.NewCalculatorRepository(live), nil, clock.Real{}, true) // no cache: every read hits PostgreSQL
	var reportSeq atomic.Int64

	scenarios := []Scenario{
		{Name: "POST events: calculator views", Rate: cfg.Rate(250), Duration: cfg.Duration, SLO: ingest,
			Do: postEvents(func(i int64) []map[string]any {
				a, l, k := anon(i), calcLocales[i%3], calcKinds[(i/3)%3]
				return []map[string]any{
					loadEvent("page_view", a, "/"+l+"/calculators/"+k, l, nil),
					loadEvent("page_view", a, "/"+l+"/calculators/"+k+"/results", l, nil),
					loadEvent("page_view", a, "/"+l+"/psychics", l, nil),
				}
			})},
		{Name: "POST events: tries and results", Rate: cfg.Rate(200), Duration: cfg.Duration, SLO: ingest,
			Do: postEvents(func(i int64) []map[string]any {
				a, l, k := anon(i*7), calcLocales[(i/3)%3], calcKinds[i%3]
				evs := []map[string]any{loadEvent("calculator_started", a, "/"+l+"/calculators/"+k, l, map[string]any{"kind": k})}
				if i%2 == 0 {
					evs = append(evs, loadEvent("calculator_completed", a, "/"+l+"/calculators/"+k+"/results", l, map[string]any{"kind": k}))
				}
				return evs
			})},
		{Name: "POST events: campaign landings", Rate: cfg.Rate(100), Duration: cfg.Duration, SLO: ingest,
			Do: postEvents(func(i int64) []map[string]any {
				a := anon(i * 13)
				return []map[string]any{loadEvent("campaign_landing", a, "/en", "en", map[string]any{
					"utm_source": calcSources[i%int64(len(calcSources))], "utm_medium": "social", "utm_campaign": "spring-sale", "has_click": i%5 == 0,
					"click_id": uuid.NewString()})}
			})},
		{Name: "report reads (service, no cache)", Rate: cfg.Rate(2), Duration: cfg.Duration, SLO: reports, MaxInFlight: 8,
			Do: func(ctx context.Context, _ *Client, _ int64) (int, error) {
				n := reportSeq.Add(1)
				q := service.CalculatorQuery{From: time.Now().AddDate(0, 0, -300).Format("2006-01-02"), To: time.Now().Format("2006-01-02"),
					Kind: []string{"", "natal-chart", "path-of-life", "astrology"}[n%4], Locale: []string{"", "en", "fr", "es"}[(n/4)%4],
					Interval: []string{"day", "week", "month"}[n%3]}
				if _, err := svc.Report(ctx, q); err != nil {
					return 500, err
				}
				if n%3 == 0 {
					q.Limit = 25
					if _, err := svc.Conversions(ctx, q); err != nil {
						return 500, err
					}
				}
				return http.StatusOK, nil
			}},
	}
	// LOAD_CALC_REPORTS=0 measures ingestion alone: the uncached report reads
	// are heavy analytical queries that compete for the same database CPU.
	if os.Getenv("LOAD_CALC_REPORTS") == "0" {
		kept := scenarios[:0]
		for _, sc := range scenarios {
			if !strings.HasPrefix(sc.Name, "report reads") {
				kept = append(kept, sc)
			}
		}
		scenarios = kept
	}
	results := runAll(ctx, scenarios, pool)
	Report(t, cfg, "calculator-ingestion", results)

	// The roll-up is asynchronous: a marker event must become visible quickly
	// and the aggregator must have kept up (nothing dropped shows as lost rows).
	marker := fmt.Sprintf("ldmark%06d-0000001", runID)
	sent := time.Now()
	if _, err := pool[0].Do(ctx, http.MethodPost, "/api/v1/analytics/events", map[string]any{"events": []map[string]any{
		loadEvent("page_view", marker, "/en/calculators/astrology", "en", nil)}}, nil); err != nil {
		t.Fatalf("marker event: %v", err)
	}
	var seen int64
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		live.Raw(`SELECT count(*) FROM web_visitors WHERE anonymous_id = ?`, marker).Row().Scan(&seen)
		if seen == 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if seen != 1 {
		t.Errorf("the marker event did not reach web_visitors within 15 s")
	} else {
		t.Logf("roll-up visibility latency: %s", time.Since(sent).Round(10*time.Millisecond))
	}
	var after, mine int64
	live.Raw(`SELECT count(*) FROM web_visitors`).Row().Scan(&after)
	live.Raw(`SELECT count(*) FROM web_visitors WHERE anonymous_id LIKE ?`, fmt.Sprintf("ld%06d-%%", runID)).Row().Scan(&mine)
	t.Logf("web_visitors: %d -> %d (this run: %d distinct visitors)", before, after, mine)
	if mine == 0 {
		t.Error("no visitor of this run reached web_visitors")
	}
}

// TestLoadCalculatorReportsConcurrent reads the heaviest reports (12 months,
// every filter) from many goroutines at once: the database must stay
// responsive when several admins refresh dashboards together.
func TestLoadCalculatorReportsConcurrent(t *testing.T) {
	cfg := LoadConfig()
	live, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Skipf("database %s: %v", cfg.DSN, err)
	}
	var visits int64
	live.Raw(`SELECT count(*) FROM calculator_visits`).Row().Scan(&visits)
	if visits < 100_000 {
		t.Skipf("only %d calculator_visits: seed first (-extend \"calculator visitors\")", visits)
	}
	svc := service.NewCalculatorService(repository.NewCalculatorRepository(live), nil, clock.Real{}, true)
	from, to := time.Now().AddDate(0, 0, -365).Format("2006-01-02"), time.Now().Format("2006-01-02")
	var wg sync.WaitGroup
	var worst atomic.Int64
	start := time.Now()
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			q := service.CalculatorQuery{From: from, To: to, Kind: []string{"", "natal-chart", "astrology"}[i%3], Interval: []string{"day", "week", "month"}[(i/3)%3]}
			t0 := time.Now()
			if _, err := svc.Report(context.Background(), q); err != nil {
				t.Errorf("report %d: %v", i, err)
			}
			for d := time.Since(t0); ; {
				cur := worst.Load()
				if int64(d) <= cur || worst.CompareAndSwap(cur, int64(d)) {
					break
				}
			}
		}(i)
	}
	wg.Wait()
	t.Logf("12 concurrent 12-month reports: total %s, slowest %s", time.Since(start).Round(time.Millisecond), time.Duration(worst.Load()).Round(time.Millisecond))
	if cfg.Strict && time.Duration(worst.Load()) > 30*time.Second {
		t.Errorf("slowest report %s", time.Duration(worst.Load()))
	}
}
