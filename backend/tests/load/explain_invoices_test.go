//go:build load

package load

import (
	"context"
	"encoding/json"
	"fmt"
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

// revisions 6.0 (§26.6): invoices and credit notes. The transactions and
// purchase pages find the documents of their rows with one query on
// (payment_transaction_id, kind) / (credit_purchase_id, kind); the console lists
// them by issue time (optionally by user, kind, number prefix or date range);
// the backfill job looks for settled payments of the last weeks without a
// document through payment_transactions_invoicing_idx. Reads only: EXPLAIN
// ANALYZE executes the statement (NextNumber, which writes, is not checked).

type invoiceSample struct {
	user      uuid.UUID
	payments  []uuid.UUID
	purchases []uuid.UUID
	prefix    string
}

func pickInvoiceSample(db *gorm.DB) invoiceSample {
	s := invoiceSample{prefix: "SY-2026-0001"}
	var user uuid.UUID
	if db.Raw(`SELECT user_id FROM invoices GROUP BY user_id ORDER BY count(*) DESC LIMIT 1`).Row().Scan(&user) == nil {
		s.user = user
	} else {
		s.user = uuid.New()
	}
	db.Raw(`SELECT payment_transaction_id FROM invoices ORDER BY issued_at DESC LIMIT 20`).Pluck("payment_transaction_id", &s.payments)
	db.Raw(`SELECT credit_purchase_id FROM invoices WHERE kind = 'invoice' ORDER BY issued_at DESC LIMIT 20`).Pluck("credit_purchase_id", &s.purchases)
	if len(s.payments) == 0 {
		s.payments, s.purchases = []uuid.UUID{uuid.New()}, []uuid.UUID{uuid.New()}
	}
	return s
}

func invoiceChecks(db *gorm.DB) []queryCheck {
	s := pickInvoiceSample(db)
	now := time.Now()
	from, to := now.AddDate(0, 0, -30), now
	kind := domain.InvoiceKindCreditNote
	return []queryCheck{
		{"invoices: documents of 20 payments", 10 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewInvoiceRepository(d).RefsByTransactions(ctx, s.payments)
		}},
		{"invoices: invoices of 20 purchases", 10 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewInvoiceRepository(d).InvoicesByPurchases(ctx, s.purchases)
		}},
		{"invoices: payment → invoice", 10 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewInvoiceRepository(d).GetByTransaction(ctx, s.payments[0], domain.InvoiceKindInvoice)
		}},
		{"invoices: admin list", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewInvoiceRepository(d).List(ctx, repository.InvoiceFilter{Limit: 21})
		}},
		{"invoices: admin list (kind)", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewInvoiceRepository(d).List(ctx, repository.InvoiceFilter{Kind: kind, Limit: 21})
		}},
		{"invoices: admin list (user)", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewInvoiceRepository(d).List(ctx, repository.InvoiceFilter{UserID: &s.user, Limit: 21})
		}},
		{"invoices: admin list (number prefix)", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewInvoiceRepository(d).List(ctx, repository.InvoiceFilter{NumberPrefix: s.prefix, Limit: 21})
		}},
		{"invoices: admin list (date range)", 50 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewInvoiceRepository(d).List(ctx, repository.InvoiceFilter{From: &from, To: &to, Limit: 21})
		}},
		{"invoices: export page", 50 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewInvoiceRepository(d).List(ctx, repository.InvoiceFilter{From: &from, To: &to, Limit: 100,
				Cursor: &repository.Cursor{At: now, ID: uuid.New().String()}})
		}},
		{"invoices: backfill (purchases)", 100 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewInvoiceRepository(d).PurchasesWithoutInvoice(ctx, now.AddDate(0, 0, -45), now.Add(-time.Minute), 100)
		}},
		{"invoices: backfill (refunds)", 100 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewInvoiceRepository(d).RefundsWithoutCreditNote(ctx, now.AddDate(0, 0, -45), now.Add(-time.Minute), 100)
		}},
	}
}

// TestExplainInvoiceQueries checks the invoice queries alone (the full check,
// TestExplainHotQueries, needs the whole large dataset): every one must use an
// index on tables beyond seqScanRowLimit rows and stay under its budget.
func TestExplainInvoiceQueries(t *testing.T) {
	cfg := LoadConfig()
	live, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("database %s: %v", cfg.DSN, err)
	}
	for _, table := range []string{"invoices", "payment_transactions", "credit_purchases"} {
		live.Exec("ANALYZE " + table)
	}
	dry, rec := newDryDB(t, cfg.DSN)
	var lines []string
	for _, qc := range invoiceChecks(live) {
		qc.build(context.Background(), dry)
		sql := rec.take()
		if sql == "" {
			t.Errorf("%s: no SQL captured", qc.name)
			continue
		}
		var worst float64
		var nodes []string
		for i := 0; i < explainRuns; i++ {
			var raw string
			if err := live.Raw("EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) " + sql).Row().Scan(&raw); err != nil {
				t.Fatalf("%s: explain: %v\n%s", qc.name, err, sql)
			}
			var out []explainOut
			if err := json.Unmarshal([]byte(raw), &out); err != nil || len(out) == 0 {
				t.Fatalf("%s: parse plan: %v", qc.name, err)
			}
			worst = max(worst, out[0].ExecutionTime)
			if i == 0 {
				walk(out[0].Plan, func(n planNode) {
					label := n.NodeType
					if n.Relation != "" {
						label += " on " + n.Relation
					}
					if n.Index != "" {
						label += " using " + n.Index
					}
					nodes = append(nodes, label)
					if n.NodeType == "Seq Scan" && n.Relation != "" && estimatedRows(live, n.Relation) > seqScanRowLimit {
						t.Errorf("%s: sequential scan on %s\n%s", qc.name, n.Relation, sql)
					}
				})
			}
		}
		if d := time.Duration(worst * float64(time.Millisecond)); d > qc.budget {
			t.Errorf("%s: slowest of %d runs %s exceeds %s", qc.name, explainRuns, d, qc.budget)
		}
		lines = append(lines, fmt.Sprintf("%-38s max=%-8.2fms %s", qc.name, worst, strings.Join(nodes, " → ")))
	}
	t.Logf("\n%s", strings.Join(lines, "\n"))
}
