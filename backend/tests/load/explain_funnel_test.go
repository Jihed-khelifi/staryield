//go:build load

package load

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// Conversion follow-ups (§27.5): the admin event funnel resolves each event's
// actor at query time (account of the event, else the account its browser is
// linked to through web_visitors, else the browser id). The visitor lookup
// must read web_visitors through its primary key and the events through the
// (name, occurred_at) index, never sequentially, within the budget below.

// funnelBudget is the execution budget of a 30-day funnel (admin-only,
// cached by nobody: a person waits for it).
const funnelBudget = 1500 * time.Millisecond

// TestExplainFunnelPopulated gives analytics_events and web_visitors
// production-like volume inside a transaction that is rolled back (40,000
// synthetic clients, 120,000 browsers of which a third are linked to a client,
// about 290,000 events spread over a year) and asserts the funnel's plan.
// Nothing is committed, so it can run against any migrated database
// (LOAD_DATABASE_DSN); it is skipped when that variable is not set.
func TestExplainFunnelPopulated(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LOAD_DATABASE_DSN"))
	if dsn == "" {
		t.Skip("LOAD_DATABASE_DSN is not set")
	}
	live, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Skipf("database is not reachable: %v", err)
	}
	var ready bool
	live.Raw(`SELECT to_regclass('web_visitors') IS NOT NULL AND to_regclass('analytics_events') IS NOT NULL`).Scan(&ready)
	if !ready {
		t.Skip("web_visitors / analytics_events are not migrated in the load database")
	}
	dry, rec := newDryDB(t, dsn)
	ctx := context.Background()

	tx := live.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback() // nothing of this test is ever committed

	steps := []struct{ name, sql string }{
		{"clients", `INSERT INTO users (id, email, password_hash, role, terms_accepted_at, email_verified_at, created_at, updated_at)
			SELECT gen_random_uuid(), 'explain-funnel-' || lpad(g::text, 6, '0') || '@load.staryield.test', 'x', 'client', now(), now(),
				now() - (g || ' minutes')::interval, now()
			FROM generate_series(1, 40000) g`},
		{"numbering", `CREATE TEMP TABLE explain_funnel_users ON COMMIT DROP AS
			SELECT id, row_number() OVER (ORDER BY email) AS n FROM users WHERE email LIKE 'explain-funnel-%'`},
		// Browsers: one in three is linked to a client (browser n belongs to client n).
		{"visitors", `INSERT INTO web_visitors (anonymous_id, first_seen_at, last_seen_at, user_id, linked_at)
			SELECT 'fv' || lpad(g::text, 10, '0'), now() - interval '300 days', now() - interval '300 days', u.id,
				CASE WHEN u.id IS NOT NULL THEN now() - interval '299 days' END
			FROM generate_series(1, 120000) g LEFT JOIN explain_funnel_users u ON u.n = g AND g <= 40000`},
		// Anonymous page views of every browser, spread over a year (older ones
		// are the pre-sign-up history that stitching resolves).
		{"page views", `INSERT INTO analytics_events (event_id, name, source, anonymous_id, occurred_at)
			SELECT gen_random_uuid(), 'web.page_view', 'client', 'fv' || lpad(g::text, 10, '0'), now() - (((g * 7919) % 525600) || ' minutes')::interval
			FROM generate_series(1, 120000) g`},
		{"profile views", `INSERT INTO analytics_events (event_id, name, source, anonymous_id, occurred_at)
			SELECT gen_random_uuid(), 'web.psychic_profile_viewed', 'client', 'fv' || lpad(g::text, 10, '0'),
				now() - (((g * 7919) % 525600) || ' minutes')::interval + interval '2 minutes'
			FROM generate_series(1, 120000, 3) g`},
		{"cta clicks", `INSERT INTO analytics_events (event_id, name, source, anonymous_id, occurred_at)
			SELECT gen_random_uuid(), 'web.chat_cta_clicked', 'client', 'fv' || lpad(g::text, 10, '0'),
				now() - (((g * 7919) % 525600) || ' minutes')::interval + interval '4 minutes'
			FROM generate_series(1, 120000, 9) g`},
		// The server's sign-up event carries the account only.
		{"registrations", `INSERT INTO analytics_events (event_id, name, source, user_id, occurred_at)
			SELECT gen_random_uuid(), 'user.registered', 'server', u.id, now() - (((u.n * 7919) % 525600) || ' minutes')::interval + interval '1 minute'
			FROM explain_funnel_users u`},
		// Signed-in page views carry both the account and the browser.
		{"signed-in page views", `INSERT INTO analytics_events (event_id, name, source, user_id, anonymous_id, occurred_at)
			SELECT gen_random_uuid(), 'web.page_view', 'client', u.id, 'fv' || lpad(u.n::text, 10, '0'),
				now() - (((u.n * 104729 + s) % 525600) || ' minutes')::interval
			FROM explain_funnel_users u, generate_series(1, 2) s`},
	}
	for _, st := range steps {
		if err := tx.Exec(st.sql).Error; err != nil {
			t.Fatalf("%s: %v", st.name, err)
		}
	}
	for _, table := range []string{"users", "web_visitors", "analytics_events"} {
		tx.Exec("ANALYZE " + table)
	}
	var count int64
	tx.Raw(`SELECT count(*) FROM analytics_events`).Scan(&count)
	if count < 250000 {
		t.Fatalf("only %d events were generated", count)
	}

	now := time.Now()
	cases := []struct {
		name   string
		steps  []string
		window time.Duration
	}{
		{"3 steps, 30 days", []string{"web.page_view", "user.registered", "web.chat_cta_clicked"}, 30 * 24 * time.Hour},
		{"2 steps, 7 days", []string{"web.page_view", "user.registered"}, 7 * 24 * time.Hour},
		{"3 steps, 90 days", []string{"web.page_view", "web.psychic_profile_viewed", "web.chat_cta_clicked"}, 90 * 24 * time.Hour},
	}
	for _, c := range cases {
		name := "funnel: " + c.name
		_, _ = repository.NewAnalyticsRepository(dry).Funnel(ctx, c.steps, now.Add(-c.window), now)
		sql := rec.take()
		if sql == "" {
			t.Fatalf("%s: no SQL captured", name)
		}
		plan := explainTx(t, tx, name, sql)
		_, label := usesIndexOn(plan, "", "")
		walk(plan.Plan, func(n planNode) {
			if n.NodeType == "Seq Scan" && (n.Relation == "analytics_events" || n.Relation == "web_visitors") {
				t.Errorf("%s: sequential scan on %s: %s\n%s", name, n.Relation, label, sql)
			}
		})
		if ok, _ := usesIndexOn(plan, "", "web_visitors_pkey"); !ok {
			t.Errorf("%s: web_visitors is not read through its primary key: %s", name, label)
		}
		if d := time.Duration(plan.ExecutionTime * float64(time.Millisecond)); d > funnelBudget {
			t.Errorf("%s: execution %s exceeds %s", name, d, funnelBudget)
		}
		t.Logf("%-28s %8.2f ms  %s", name, plan.ExecutionTime, label)
	}
}
