//go:build load

package load

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/app"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
)

// revisions 5.0: refund requests and the revenue report on the large dataset.
//
// TestLoadRevenueAndRefundQueue builds the application in process (no running
// API: it calls the services the handlers call) and times the revenue report
// (30 days, 366 days, and as of a past date), the admin refund list (open and
// all, with its batched user/payment/lot/pack loads) and a client's transactions
// page with its refund info, each LOAD_REVENUE_RUNS times (default 10). The
// objective is p95 < 1 s for the 30-day report. Needs LOAD_DATABASE_DSN: a
// private copy of the large dataset with migration 0028 and, for volume, the
// "credit purchases" and "payment transactions" seeds.
func TestLoadRevenueAndRefundQueue(t *testing.T) {
	cfg := LoadConfig()
	runs := envInt("LOAD_REVENUE_RUNS", 10)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	acfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	acfg.App.Env = config.EnvTest
	acfg.Database.DSN = cfg.DSN
	acfg.Redis.URL = env("LOAD_REDIS_URL", "redis://127.0.0.1:6379/13")
	acfg.Redis.KeyPrefix = "loadrev:" + uuid.NewString()[:8] + ":"
	acfg.Worker.Enabled = false
	acfg.Log.Level = "error"
	if err := acfg.Validate(); err != nil {
		t.Fatalf("config: %v", err)
	}
	a, err := app.New(ctx, acfg, zap.NewNop(), app.Overrides{Clock: clock.Real{}, Mailer: &mail.Recorder{}, Tracker: &analytics.Recorder{},
		Storage: storage.NewMemory("https://assets.test"), AdvisorStorage: storage.NewMemory("https://advisors.test"), ChatStorage: storage.NewMemory("")})
	if err != nil {
		t.Fatalf("app: %v", err)
	}
	defer a.Shutdown(context.Background())

	now := time.Now().UTC()
	report := func(days int, past int) func() error {
		return func() error {
			to := now.AddDate(0, 0, -past)
			_, err := a.Services.Revenue.Report(ctx, service.Range{From: to.AddDate(0, 0, -days), To: to}, "day")
			return err
		}
	}
	var user uuid.UUID
	if err := a.DB.Raw(`SELECT user_id FROM refund_requests GROUP BY user_id ORDER BY count(*) DESC LIMIT 1`).Row().Scan(&user); err != nil {
		t.Fatalf("no refund requests in this database (run the seeds and migration 0028): %v", err)
	}
	cases := []struct {
		name   string
		budget time.Duration
		run    func() error
	}{
		{"revenue report, 30 days", time.Second, report(30, 0)},
		{"revenue report, 90 days", time.Second, report(90, 0)},
		{"revenue report, 366 days", 3 * time.Second, report(366, 0)},
		{"revenue report, 30 days ending 20 days ago", time.Second, report(30, 20)},
		{"admin refund list, open (20)", 250 * time.Millisecond, func() error {
			_, err := a.Services.Refunds.AdminList(ctx, "open", nil, "", 20)
			return err
		}},
		{"admin refund list, all (100)", 500 * time.Millisecond, func() error {
			_, err := a.Services.Refunds.AdminList(ctx, "", nil, "", 100)
			return err
		}},
		{"client transactions page with refund info (20)", 100 * time.Millisecond, func() error {
			_, err := a.Services.Billing.Transactions(ctx, user, "", 20)
			return err
		}},
	}
	for _, c := range cases {
		var times []time.Duration
		for i := 0; i < runs; i++ {
			start := time.Now()
			if err := c.run(); err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			times = append(times, time.Since(start))
		}
		sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
		p95 := times[min(len(times)-1, int(float64(len(times))*0.95+0.999999)-1)]
		t.Logf("%-48s min %-8s median %-8s p95 %s", c.name, times[0].Round(time.Millisecond), times[len(times)/2].Round(time.Millisecond), p95.Round(time.Millisecond))
		if p95 > c.budget {
			t.Errorf("%s: p95 %s exceeds %s", c.name, p95, c.budget)
		}
	}
}
