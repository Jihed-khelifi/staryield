//go:build load

package load

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// Follow-ups: auto-away. The counter statements run on every request that
// expires unanswered and on every accept, decline and advisor availability
// change: each must be a primary-key update on psychics (no sequential scan).
// The statements are the ones the repository builds (dry run), explained with
// EXPLAIN (ANALYZE) inside a transaction that is rolled back, so nothing is
// changed. The test skips cleanly without a load database or before
// migration 0041.

// updateRecorder captures the SQL of the update statements a dry-run
// database builds (newDryDB only records queries).
type updateRecorder struct {
	mu   sync.Mutex
	last string
}

func (r *updateRecorder) take() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.last
	r.last = ""
	return s
}

func newDryUpdateDB(t *testing.T, dsn string) (*gorm.DB, *updateRecorder) {
	t.Helper()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{DryRun: true, Logger: logger.Discard})
	if err != nil {
		t.Fatalf("dry-run db: %v", err)
	}
	rec := &updateRecorder{}
	_ = db.Callback().Update().After("gorm:update").Register("load:capture_update", func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.SQL.Len() == 0 {
			return
		}
		rec.mu.Lock()
		rec.last = tx.Dialector.Explain(tx.Statement.SQL.String(), tx.Statement.Vars...)
		rec.mu.Unlock()
	})
	return db, rec
}

func TestExplainAutoAway(t *testing.T) {
	cfg := LoadConfig()
	live, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Skipf("no load database (%s): %v", cfg.DSN, err)
	}
	var ready bool
	live.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'psychics' AND column_name = 'missed_requests_in_row')`).Scan(&ready)
	if !ready {
		t.Skip("migration 0041 is not applied to the load database")
	}
	var advisor uuid.UUID
	if err := live.Raw(`SELECT user_id FROM psychics ORDER BY user_id LIMIT 1`).Row().Scan(&advisor); err != nil {
		t.Skip("the load database has no advisor")
	}
	dry, rec := newDryUpdateDB(t, cfg.DSN)
	ctx := context.Background()
	repo := repository.NewPsychicRepository(dry)

	cases := []struct {
		name  string
		build func()
	}{
		{"increment the missed-request counter (UPDATE … RETURNING)", func() { _, _ = repo.IncrementMissed(ctx, advisor) }},
		{"reset the counter (answer or availability change)", func() { _ = repo.ResetMissed(ctx, advisor) }},
		{"claim the reset at the threshold", func() { _, _ = repo.ResetMissedAtLeast(ctx, advisor, 3) }},
		{"set Away if Online", func() { _, _ = repo.SetAwayIfOnline(ctx, advisor) }},
	}
	tx := live.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback() // EXPLAIN ANALYZE executes the updates: never commit them
	for _, tc := range cases {
		tc.build()
		sql := rec.take()
		if sql == "" {
			t.Fatalf("%s: no SQL captured", tc.name)
		}
		plan := explainTx(t, tx, tc.name, sql)
		var nodes []string
		viaKey := false
		walk(plan.Plan, func(n planNode) {
			nodes = append(nodes, strings.TrimSpace(n.NodeType+" "+n.Relation+" "+n.Index))
			if n.Index == "psychics_pkey" {
				viaKey = true
			}
			if n.NodeType == "Seq Scan" && n.Relation == "psychics" && estimatedRows(live, "psychics") > seqScanRowLimit {
				t.Errorf("%s: sequential scan on psychics\n%s", tc.name, sql)
			}
		})
		if !viaKey && estimatedRows(live, "psychics") > seqScanRowLimit {
			t.Errorf("%s: psychics is not updated through its primary key: %s\n%s", tc.name, strings.Join(nodes, " → "), sql)
		}
		t.Logf("%-62s %6.2f ms  %s", tc.name, plan.ExecutionTime, strings.Join(nodes, " → "))
	}
}
