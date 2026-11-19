//go:build load

package load

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 6.0 (§26.6): issue invoices for 10 000 paid purchases from several
// workers at once, as the backfill job and the inline path do together, and
// check what the contract promises: one invoice per purchase, numbers without
// gaps, no deadlock. Needs migration 0034 on LOAD_DATABASE_DSN. It seeds its
// own synthetic purchases (settled card payments of existing clients) and
// removes everything it created when it ends.
//
//	LOAD_DATABASE_DSN=... go test -tags load ./tests/load/ -run TestLoadInvoicesFor10kPurchases -v
//
// With LOAD_KEEP=1 the synthetic data stays, so TestExplainInvoiceQueries can
// check the plans of the invoice queries on a database that has no large
// dataset (a scratch one).
func TestLoadInvoicesFor10kPurchases(t *testing.T) {
	const purchases = 10_000
	cfg := LoadConfig()
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("database: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(24)
	var users []uuid.UUID
	db.Raw(`SELECT id FROM users WHERE role = 'client' ORDER BY created_at LIMIT 500`).Pluck("id", &users)
	var packID uuid.UUID
	if len(users) == 0 || db.Raw(`SELECT id FROM credit_packs LIMIT 1`).Row().Scan(&packID) != nil {
		t.Skip("the database needs clients and credit packs (go run ./cmd/seed)")
	}
	tag := "loadinv-" + uuid.NewString()[:8]
	cleanup := func() {
		db.Exec(`DELETE FROM invoices WHERE payment_transaction_id IN (SELECT id FROM payment_transactions WHERE idempotency_key LIKE ?) AND kind = 'credit_note'`, tag+"%")
		db.Exec(`DELETE FROM invoices WHERE payment_transaction_id IN (SELECT id FROM payment_transactions WHERE idempotency_key LIKE ?)`, tag+"%")
		db.Exec(`DELETE FROM payment_transactions WHERE idempotency_key LIKE ?`, tag+"%")
		db.Exec(`DELETE FROM credit_purchases WHERE idempotency_key LIKE ?`, tag+"%")
	}
	cleanup()
	if env("LOAD_KEEP", "") != "1" { // LOAD_KEEP=1 leaves the data for TestExplainInvoiceQueries on a scratch database
		t.Cleanup(cleanup)
	}

	// Seed: n succeeded purchases with their settled payment (one statement each, set-based).
	if err := db.Exec(`
		WITH u AS (SELECT id, row_number() OVER () AS n FROM unnest(?::uuid[]) AS id),
		p AS (
			INSERT INTO credit_purchases (id, user_id, pack_id, credits, price_cents, currency, status, provider, idempotency_key, created_at, updated_at, completed_at)
			SELECT gen_random_uuid(), (SELECT id FROM u WHERE n = 1 + (g % ?)), ?, 250, 999, 'USD', 'succeeded', 'sandbox_a', ? || '-' || g,
			       now() - interval '1 hour', now() - interval '1 hour', now() - interval '1 hour'
			FROM generate_series(1, ?) g
			RETURNING id, user_id, idempotency_key)
		INSERT INTO payment_transactions (id, user_id, kind, status, amount_cents, currency, credits, provider, description, idempotency_key,
		                                  credit_purchase_id, created_at, updated_at)
		SELECT gen_random_uuid(), user_id, 'credit_purchase', 'succeeded', 999, 'USD', 250, 'sandbox_a', 'Load pack', idempotency_key, id,
		       now() - interval '1 hour', now() - interval '1 hour'
		FROM p`, "{"+joinUUIDs(users)+"}", len(users), packID, tag, purchases).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	var ids []uuid.UUID
	db.Raw(`SELECT credit_purchase_id FROM payment_transactions WHERE idempotency_key LIKE ?`, tag+"%").Pluck("credit_purchase_id", &ids)
	if len(ids) != purchases {
		t.Fatalf("seeded %d purchases", len(ids))
	}

	sealer, _ := security.NewSealer([]byte(strings.Repeat("l", 32)))
	svc := service.NewInvoiceService(service.InvoiceDeps{Tx: database.NewTxManager(db), Repo: repository.NewInvoiceRepository(db),
		Billing: repository.NewBillingRepository(db), Credits: repository.NewCreditRepository(db), Users: repository.NewUserRepository(db),
		Sealer: sealer, Audit: repository.NewAuditRepository(db), Clock: clock.Real{}, Log: zap.NewNop()},
		service.InvoiceSettings{FrontendURL: "https://staryield.net"})

	const workers = 16
	var next, failed atomic.Int64
	var mu sync.Mutex
	var lat []time.Duration
	started := time.Now()
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1)) - 1
				if i >= len(ids) {
					return
				}
				t0 := time.Now()
				if _, err := svc.IssueForPurchase(context.Background(), ids[i]); err != nil {
					failed.Add(1)
					t.Logf("purchase %s: %v", ids[i], err)
					continue
				}
				d := time.Since(t0)
				mu.Lock()
				lat = append(lat, d)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(started)
	if failed.Load() != 0 {
		t.Fatalf("%d purchases could not be invoiced", failed.Load())
	}

	// One invoice per purchase, numbers distinct and without gaps.
	var numbers []string
	db.Raw(`SELECT number FROM invoices WHERE credit_purchase_id = ANY(?::uuid[]) AND kind = 'invoice'`, "{"+joinUUIDs(ids)+"}").Pluck("number", &numbers)
	if len(numbers) != purchases {
		t.Fatalf("%d invoices for %d purchases", len(numbers), purchases)
	}
	seq := make([]int, len(numbers))
	for i, n := range numbers {
		v, _ := strconv.Atoi(n[strings.LastIndex(n, "-")+1:])
		seq[i] = v
	}
	sort.Ints(seq)
	for i := 1; i < len(seq); i++ {
		if seq[i] != seq[i-1]+1 {
			t.Fatalf("gap or duplicate between %d and %d", seq[i-1], seq[i])
		}
	}
	// A second pass issues nothing new.
	if _, err := svc.IssueForPurchase(context.Background(), ids[0]); err != nil {
		t.Fatal(err)
	}
	var again int64
	db.Raw(`SELECT count(*) FROM invoices WHERE credit_purchase_id = ANY(?::uuid[])`, "{"+joinUUIDs(ids)+"}").Scan(&again)
	if again != purchases {
		t.Fatalf("a repeat issued documents: %d", again)
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	p95 := lat[len(lat)*95/100]
	t.Logf("issued %d invoices with %d workers in %s (%.0f/s); p50=%s p95=%s p99=%s; numbers %d..%d contiguous", purchases, workers,
		elapsed.Round(time.Millisecond), float64(purchases)/elapsed.Seconds(), lat[len(lat)/2], p95, lat[len(lat)*99/100], seq[0], seq[len(seq)-1])
	// Numbering serialises on the counter row by design (gapless); even so it must stay fast enough for the busiest launch.
	if rate := float64(purchases) / elapsed.Seconds(); rate < 50 {
		t.Errorf("only %.0f invoices/s; the purchase flow needs at least 50/s", rate)
	}
	if p95 > 2*time.Second {
		t.Errorf("p95 %s to issue one invoice", p95)
	}

	// The backfill's lookup over the same data: nothing is missing now.
	missing, err := repository.NewInvoiceRepository(db).PurchasesWithoutInvoice(context.Background(), time.Now().AddDate(0, 0, -45), time.Now(), 500)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range missing {
		for _, mine := range ids {
			if id == mine {
				t.Fatalf("purchase %s was left without invoice", id)
			}
		}
	}
}

func joinUUIDs(ids []uuid.UUID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = id.String()
	}
	return strings.Join(parts, ",")
}
