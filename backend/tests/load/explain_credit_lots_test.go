//go:build load

package load

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 5.0: credit lots (docs/platform-extensions.md section 24.3). The
// consumption read is on the hot billing path (one per billed minute), the
// purchase-lot lookups serve the payment lists and refund quotes, and the
// reconciliation chunk runs daily over every wallet. Reads only: EXPLAIN
// ANALYZE executes the statement (the lot update is checked by the billing
// scenario in credit_lots_test.go).

type creditLotSample struct {
	user       uuid.UUID   // the user with the most lots
	purchases  []uuid.UUID // purchase ids that have lots (random ids when none exist)
	entry      uuid.UUID   // a ledger entry id
	key        string      // an idempotency key
	after, end uuid.UUID   // a reconciliation chunk of 5 000 wallets from the middle of the table
}

func pickCreditLots(db *gorm.DB, ids sampleIDs) creditLotSample {
	s := creditLotSample{user: ids.ledgerUser, entry: uuid.New(), key: "request_hold:" + uuid.NewString()}
	var one struct{ A uuid.UUID }
	if db.Table("credit_lots").Select("user_id AS a").Group("user_id").Order("count(*) DESC").Limit(1).Scan(&one).Error == nil && one.A != uuid.Nil {
		s.user = one.A
	}
	db.Table("credit_lots").Where("reference_type = ?", "credit_purchase").Order("created_at DESC").Limit(20).Pluck("reference_id", &s.purchases)
	if len(s.purchases) == 0 {
		s.purchases = []uuid.UUID{uuid.New()}
	}
	// (uuid columns are read with Row().Scan: gorm's Scan leaves a uuid.UUID untouched.)
	var entry uuid.UUID
	if db.Raw(`SELECT id FROM credit_ledger WHERE user_id = ? ORDER BY created_at DESC LIMIT 1`, ids.ledgerUser).Row().Scan(&entry) == nil {
		s.entry = entry
	}
	db.Raw(`SELECT idempotency_key FROM credit_ledger WHERE user_id = ? ORDER BY created_at DESC LIMIT 1`, ids.ledgerUser).Scan(&s.key)
	var after, end uuid.UUID
	if db.Raw(`SELECT user_id FROM wallets ORDER BY user_id OFFSET 50000 LIMIT 1`).Row().Scan(&after) == nil &&
		db.Raw(`SELECT user_id FROM wallets ORDER BY user_id OFFSET 55000 LIMIT 1`).Row().Scan(&end) == nil {
		s.after, s.end = after, end
	}
	if s.end == uuid.Nil {
		s.after, s.end = uuid.Nil, uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	}
	return s
}

func creditLotChecks(db *gorm.DB, ids sampleIDs) []queryCheck {
	s := pickCreditLots(db, ids)
	return []queryCheck{
		{"lots: spendable (consumption)", 10 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLotRepository(d).SpendableLots(ctx, s.user, 16)
		}},
		{"lots: spendable (second read)", 10 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLotRepository(d).SpendableLots(ctx, s.user, 1000)
		}},
		{"lots: of a purchase", 10 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLotRepository(d).PurchaseLot(ctx, s.purchases[0])
		}},
		{"lots: of 20 purchases", 10 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLotRepository(d).PurchaseLots(ctx, s.purchases)
		}},
		{"lots: user statement", 10 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLotRepository(d).UserLots(ctx, s.user, 100)
		}},
		{"lots: allocations of an entry", 10 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLotRepository(d).Allocations(ctx, s.user, &s.entry, "")
		}},
		{"lots: allocations by key", 10 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLotRepository(d).Allocations(ctx, s.user, nil, s.key)
		}},
		{"lots: reconcile bounds", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewLotRepository(d).Reconcile(ctx, s.after, 5000) // the dry run captures the bounds statement
		}},
		{"lots: reconcile chunk (5 000 wallets)", 500 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			var rows []repository.LotMismatch
			_ = repository.NewLotRepository(d).Mismatches(ctx, s.after, s.end).Scan(&rows)
		}},
	}
}

// The consumption read must be answered by credit_lots_consume_idx in index
// order, without a sort, whatever the planner thinks of a small database
// (sequential scans are disabled for the check, like the unread-totals one).
func TestExplainCreditLotConsumptionUsesTheIndex(t *testing.T) {
	cfg := LoadConfig()
	live, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("database %s: %v", cfg.DSN, err)
	}
	dry, rec := newDryDB(t, cfg.DSN)
	ids := pickIDs(t, live)
	s := pickCreditLots(live, ids)
	_, _ = repository.NewLotRepository(dry).SpendableLots(context.Background(), s.user, 16)
	sql := rec.take()
	if sql == "" {
		t.Fatal("no SQL captured")
	}
	var raw string
	err = live.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL enable_seqscan = off").Error; err != nil {
			return err
		}
		return tx.Raw("EXPLAIN (ANALYZE, FORMAT JSON) " + sql).Row().Scan(&raw)
	})
	if err != nil {
		t.Fatal(err)
	}
	var out []explainOut
	if err := json.Unmarshal([]byte(raw), &out); err != nil || len(out) == 0 {
		t.Fatalf("plan: %v", err)
	}
	var usedIndex, sorted bool
	walk(out[0].Plan, func(n planNode) {
		if n.Index == "credit_lots_consume_idx" {
			usedIndex = true
		}
		if n.NodeType == "Sort" || n.NodeType == "Incremental Sort" {
			sorted = true
		}
	})
	if !usedIndex || sorted {
		t.Errorf("want credit_lots_consume_idx in index order, plan: %s", raw)
	}
}
