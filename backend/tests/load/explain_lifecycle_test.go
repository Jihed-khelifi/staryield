//go:build load

package load

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// growth: lifecycle messaging and the onboarding checklist. Two checks, like
// the free-trial hold's:
//
//   - lifecycleChecks join TestExplainHotQueries: the job's due queries, the
//     admin statistics and activation funnel and the checklist, on the dataset
//     as it is (the lifecycle tables may be empty: the plans must still start
//     from their indexes and not scan big tables);
//   - TestExplainLifecyclePopulated gives the lifecycle tables real volume
//     inside a transaction that is rolled back (the readers of every settled
//     reading, a message for most clients and readers, a live session for half
//     the clients) and asserts that each query reads the index it was built
//     for, within its budget; it also times the migration's one-time backfill
//     and explains the two writes (the conversion UPDATE and a claim INSERT).

// populatedRuns: EXPLAIN ANALYZE runs per query of TestExplainLifecyclePopulated.
const populatedRuns = 5

// lifecycleReady says whether migration 0037 is applied to the load database.
func lifecycleReady(db *gorm.DB) bool {
	var ready bool
	db.Raw(`SELECT to_regclass('lifecycle_messages') IS NOT NULL`).Scan(&ready)
	return ready
}

// lifecycleDueQueries are the job's due queries as one run builds them
// (default delays, first batch): one per journey, and win-back's third cycle
// band (the oldest readers it looks at).
func lifecycleDueQueries(now time.Time) map[string]repository.LifecycleDueQuery {
	day := 24 * time.Hour
	out := map[string]repository.LifecycleDueQuery{}
	for _, j := range []struct {
		name, key string
		delay     time.Duration
		band      int
	}{
		{domain.JourneyVerifyReminder, domain.JourneyVerifyReminder, day, 0},
		{domain.JourneyFirstReading, domain.JourneyFirstReading, day, 0},
		{domain.JourneyAfterFirstReading, domain.JourneyAfterFirstReading, day, 0},
		{domain.JourneyWinBack, domain.JourneyWinBack, 14 * day, 0},
		{domain.JourneyWinBack + " (third cycle)", domain.JourneyWinBack, 14 * day, 2},
	} {
		cutoff := now.Add(-j.delay - time.Duration(j.band)*30*day)
		q := repository.LifecycleDueQuery{Journey: j.key, Cutoff: cutoff, Floor: cutoff.Add(-72 * time.Hour), CapSince: now.Add(-48 * time.Hour), Limit: 500}
		if j.key == domain.JourneyWinBack {
			q.InactiveSince, q.CycleSince, q.MaxCycles = now.Add(-j.delay), now.Add(-30*day), 3
		}
		out[j.name] = q
	}
	return out
}

// busiestClient is the client with the most readings (the checklist's worst case).
func busiestClient(db *gorm.DB) uuid.UUID {
	var row struct{ ClientID uuid.UUID }
	if db.Raw(`SELECT client_id FROM chat_sessions GROUP BY client_id ORDER BY count(*) DESC LIMIT 1`).Scan(&row).Error != nil || row.ClientID == uuid.Nil {
		return uuid.New()
	}
	return row.ClientID
}

func lifecycleChecks(db *gorm.DB) []queryCheck {
	if !lifecycleReady(db) {
		return nil // migration 0037 not applied to this dataset
	}
	now := time.Now()
	client := busiestClient(db)
	due := lifecycleDueQueries(now)
	var checks []queryCheck
	for name, q := range due {
		checks = append(checks, queryCheck{"lifecycle: due " + name, 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLifecycleRepository(d).Due(ctx, q)
		}})
	}
	return append(checks,
		queryCheck{"lifecycle: statistics (7 and 30 days)", 50 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLifecycleRepository(d).Stats(ctx, now, 7)
		}},
		// One primary-key probe pair per account of the cohort; the service
		// caches the result for a minute.
		queryCheck{"lifecycle: activation funnel (30 days)", 150 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLifecycleRepository(d).Funnel(ctx, now.AddDate(0, 0, -30), now)
		}},
		queryCheck{"onboarding: checklist facts", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLifecycleRepository(d).Onboarding(ctx, client)
		}},
		// The job's reconciliation reads the readings settled in the last hours
		// (in a dry run it stops at that read; it then rewrites those readers).
		queryCheck{"lifecycle: readings settled recently", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLifecycleRepository(d).ReconcileReaders(ctx, now.Add(-3*time.Hour))
		}},
	)
}

func TestExplainLifecyclePopulated(t *testing.T) {
	cfg := LoadConfig()
	live, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("database %s: %v", cfg.DSN, err)
	}
	if !lifecycleReady(live) {
		t.Skip("migration 0037 is not applied to the load database")
	}
	var clients int64
	live.Raw(`SELECT count(*) FROM users WHERE role = 'client'`).Scan(&clients)
	if clients < 50_000 {
		t.Skipf("the dataset has %d clients; seed the large profile (at least 50,000)", clients)
	}
	dry, rec := newDryDB(t, cfg.DSN)
	// The conversion is a GORM update: capture it too.
	_ = dry.Callback().Update().After("gorm:update").Register("load:capture_update", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.SQL.Len() > 0 {
			rec.mu.Lock()
			rec.last = tx.Dialector.Explain(tx.Statement.SQL.String(), tx.Statement.Vars...)
			rec.mu.Unlock()
		}
	})
	ctx := context.Background()
	now := time.Now()

	tx := live.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	analyzed := []string{"lifecycle_messages", "lifecycle_readers", "refresh_tokens", "users"}
	defer func() {
		tx.Rollback() // nothing of this test is ever committed
		// ANALYZE inside the transaction updated the planner statistics in place
		// (they are not transactional): measure the tables again as they are, so
		// the checks that run next see the real dataset.
		for _, table := range analyzed {
			live.Exec(`ANALYZE ` + table)
		}
	}()

	// The migration's one-time backfill of the readers, timed on this dataset.
	tx.Exec(`DELETE FROM lifecycle_readers`)
	started := time.Now()
	backfill := tx.Exec(`INSERT INTO lifecycle_readers (user_id, first_reading_at, last_reading_at, readings)
		SELECT client_id, min(settled_at), max(settled_at), count(*)
		FROM chat_sessions
		WHERE status = 'settled' AND billed_minutes > 0 AND settled_at IS NOT NULL
		GROUP BY client_id
		ON CONFLICT (user_id) DO NOTHING`)
	if backfill.Error != nil {
		t.Fatal(backfill.Error)
	}
	t.Logf("migration backfill: %d readers in %s", backfill.RowsAffected, time.Since(started).Round(time.Millisecond))

	// A live session (refresh token) for half the clients, so the win-back
	// probe meets real rows.
	if err := tx.Exec(`INSERT INTO refresh_tokens (id, user_id, family_id, token_hash, expires_at, created_at, last_used_at)
		SELECT gen_random_uuid(), u.id, gen_random_uuid(), md5(u.id::text || 'lifecycle-load'), now() + interval '30 days',
			u.created_at, COALESCE(u.last_login_at, u.created_at)
		FROM users u WHERE u.role = 'client' AND (hashtext(u.id::text) & 1) = 0`).Error; err != nil {
		t.Fatal(err)
	}
	// Messages: a first-reading nudge for 60% of the clients, a verification
	// reminder for 20%, an invitation for half the readers and a win-back for a
	// third of them (cycle 1), with conversions; a fifth of the recent ones
	// are inside the frequency cap.
	for _, q := range []string{
		`INSERT INTO lifecycle_messages (id, user_id, journey, cycle, status, channels, converted_at, created_at, updated_at)
		 SELECT gen_random_uuid(), u.id, 'first_reading', 1, 'sent', '{"in_app": true}',
			CASE WHEN abs(hashtext(u.id::text)) % 10 < 2 THEN u.created_at + interval '30 hours' END,
			u.created_at + interval '24 hours', u.created_at + interval '24 hours'
		 FROM users u WHERE u.role = 'client' AND abs(hashtext(u.id::text)) % 10 < 6 AND u.created_at < now() - interval '24 hours'`,
		`INSERT INTO lifecycle_messages (id, user_id, journey, cycle, status, channels, created_at, updated_at)
		 SELECT gen_random_uuid(), u.id, 'verify_reminder', 1, 'sent', '{"in_app": true, "email": true}',
			u.created_at + interval '25 hours', u.created_at + interval '25 hours'
		 FROM users u WHERE u.role = 'client' AND abs(hashtext(u.id::text || 'v')) % 10 < 2 AND u.created_at < now() - interval '25 hours'`,
		`INSERT INTO lifecycle_messages (id, user_id, journey, cycle, status, skip_reason, channels, created_at, updated_at)
		 SELECT gen_random_uuid(), r.user_id, 'after_first_reading', 1,
			CASE WHEN abs(hashtext(r.user_id::text)) % 4 = 0 THEN 'skipped' ELSE 'sent' END,
			CASE WHEN abs(hashtext(r.user_id::text)) % 4 = 0 THEN 'referrals_disabled' ELSE '' END, '{}',
			r.first_reading_at + interval '24 hours', r.first_reading_at + interval '24 hours'
		 FROM lifecycle_readers r WHERE abs(hashtext(r.user_id::text || 'a')) % 2 = 0 AND r.first_reading_at < now() - interval '24 hours'`,
		`INSERT INTO lifecycle_messages (id, user_id, journey, cycle, status, channels, created_at, updated_at)
		 SELECT gen_random_uuid(), r.user_id, 'win_back', 1, 'sent', '{"in_app": true}',
			r.last_reading_at + interval '14 days', r.last_reading_at + interval '14 days'
		 FROM lifecycle_readers r WHERE abs(hashtext(r.user_id::text || 'w')) % 3 = 0 AND r.last_reading_at < now() - interval '14 days'`,
	} {
		if err := tx.Exec(q).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range analyzed {
		tx.Exec(`ANALYZE ` + table)
	}
	var messages int64
	tx.Raw(`SELECT count(*) FROM lifecycle_messages`).Scan(&messages)
	t.Logf("populated: %d clients, %d readers, %d messages", clients, backfill.RowsAffected, messages)

	client := busiestClient(tx)
	due := lifecycleDueQueries(now)
	type planCase struct {
		name   string
		build  func(ctx context.Context, d *gorm.DB)
		index  string // the index the plan must read the table through ("a|b": either)
		table  string
		budget time.Duration
	}
	// Sign-up journeys read a created_at range of users: through the
	// (role, created_at) or the (status, created_at) index, whichever the
	// planner prefers.
	const signupIndexes = "users_role_created_idx|users_status_idx"
	var cases []planCase
	for name, index := range map[string]string{
		// The verification reminder reads unverified accounts only: the
		// e-mail verification feature's partial index (0039) fits it best.
		domain.JourneyVerifyReminder:             signupIndexes + "|users_unverified_created_idx",
		domain.JourneyFirstReading:               signupIndexes,
		domain.JourneyAfterFirstReading:          "lifecycle_readers_first_idx",
		domain.JourneyWinBack:                    "lifecycle_readers_last_idx",
		domain.JourneyWinBack + " (third cycle)": "lifecycle_readers_last_idx",
	} {
		q := due[name]
		table := "users"
		if q.Journey == domain.JourneyAfterFirstReading || q.Journey == domain.JourneyWinBack {
			table = "lifecycle_readers"
		}
		cases = append(cases, planCase{"due " + name, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLifecycleRepository(d).Due(ctx, q)
		}, index, table, 25 * time.Millisecond})
		// A later batch of the same run (keyset after the middle of the window).
		next := q
		next.After = &repository.LifecycleCursor{At: q.Floor.Add(q.Cutoff.Sub(q.Floor) / 2), UserID: uuid.New()}
		cases = append(cases, planCase{"due " + name + " (next batch)", func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLifecycleRepository(d).Due(ctx, next)
		}, index, table, 25 * time.Millisecond})
	}
	cases = append(cases,
		planCase{"statistics (7 and 30 days)", func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLifecycleRepository(d).Stats(ctx, now, 7)
		}, "lifecycle_messages_stats_idx", "lifecycle_messages", 100 * time.Millisecond},
		planCase{"activation funnel (30 days)", func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLifecycleRepository(d).Funnel(ctx, now.AddDate(0, 0, -30), now)
		}, "users_role_created_idx", "users", 150 * time.Millisecond},
		planCase{"activation funnel (90 days)", func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLifecycleRepository(d).Funnel(ctx, now.AddDate(0, 0, -90), now)
		}, "users_role_created_idx", "users", 300 * time.Millisecond},
		planCase{"checklist facts (busiest client)", func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLifecycleRepository(d).Onboarding(ctx, client)
		}, "users_pkey", "users", 25 * time.Millisecond},
		planCase{"readings settled in the last 3 hours", func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLifecycleRepository(d).ReconcileReaders(ctx, now.Add(-3*time.Hour))
		}, "chat_sessions_reading_settled_idx", "chat_sessions", 25 * time.Millisecond},
		planCase{"conversion on a settled reading", func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLifecycleRepository(d).Convert(ctx, client, now.AddDate(0, 0, -7), now)
		}, "lifecycle_messages_open_idx", "lifecycle_messages", 25 * time.Millisecond},
		planCase{"claim (500 rows, ON CONFLICT DO NOTHING)", func(ctx context.Context, d *gorm.DB) {
			rows := make([]domain.LifecycleMessage, 500)
			for i := range rows {
				rows[i] = domain.LifecycleMessage{ID: uuid.New(), UserID: client, Journey: domain.JourneyWinBack, Cycle: 100 + i,
					Status: domain.LifecycleSent, Channels: domain.JSONMap{"in_app": true}, CreatedAt: now}
			}
			_, _ = repository.NewLifecycleRepository(d).Claim(ctx, rows)
		}, "", "lifecycle_messages", 100 * time.Millisecond},
	)
	for _, tc := range cases {
		tc.build(ctx, dry)
		sql := rec.take()
		if sql == "" {
			t.Fatalf("%s: no SQL captured", tc.name)
		}
		// The median of a few runs: one slow run on a busy machine is not the plan's cost.
		var plan explainOut
		times := make([]float64, 0, populatedRuns)
		for i := 0; i < populatedRuns; i++ {
			plan = explainTx(t, tx, tc.name, sql)
			times = append(times, plan.ExecutionTime)
		}
		sort.Float64s(times)
		median := times[len(times)/2]
		var label string
		if tc.index != "" {
			ok := false
			for _, index := range strings.Split(tc.index, "|") {
				var used bool
				used, label = usesIndexOn(plan, tc.table, index)
				ok = ok || used
			}
			if !ok {
				t.Errorf("%s: %s is not read through %s: %s\n%s", tc.name, tc.table, tc.index, label, sql)
			}
		} else {
			_, label = usesIndexOn(plan, tc.table, "")
		}
		walk(plan.Plan, func(n planNode) {
			// Row estimates from inside the transaction (its ANALYZE sees the volume).
			if n.NodeType == "Seq Scan" && n.Relation != "" && estimatedRows(tx, n.Relation) > seqScanRowLimit {
				t.Errorf("%s: sequential scan on %s: %s\n%s", tc.name, n.Relation, label, sql)
			}
		})
		if d := time.Duration(median * float64(time.Millisecond)); d > tc.budget {
			t.Errorf("%s: median execution %s exceeds %s", tc.name, d, tc.budget)
		}
		t.Logf("%-44s %7.2f ms  %s", tc.name, median, label)
	}
}
