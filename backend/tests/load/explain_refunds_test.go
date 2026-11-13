//go:build load

package load

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 5.0: refund requests (§24.4) and the revenue report (§24.5).
// The transactions page reads the lots and the newest request of its 20 payments
// with one query each; the console lists requests with keyset pages; the
// revenue report runs one statement per figure over a 30-day window. Reads
// only: EXPLAIN ANALYZE executes the statement.

// Two revenue statements may hash-join with one sequential scan where the
// window covers a large share of a table (the planner decides by the
// statistics; with a small share it probes the indexes instead): the
// recognized series (chat charges → their allocations) and the deferred figure
// at a past instant (allocations since then → their lots). Both are measured
// against their budgets below; the live deferred figure and the 30-day
// figures are index-only.
func init() {
	seqScanAllowed["revenue: recognized series"] = true
	seqScanAllowed["revenue: deferred (10 days ago)"] = true
}

type refundSample struct {
	user      uuid.UUID   // a client with refund requests
	payments  []uuid.UUID // 20 payments of the user (a transactions page)
	purchases []uuid.UUID // the purchases behind them
	period    repository.RevenuePeriod
}

func pickRefundSample(db *gorm.DB, ids sampleIDs) refundSample {
	s := refundSample{user: ids.ledgerUser, period: repository.RevenuePeriod{From: time.Now().AddDate(0, 0, -30), To: time.Now(), Currency: "USD", Unit: "day"}}
	var user uuid.UUID
	if db.Raw(`SELECT user_id FROM refund_requests GROUP BY user_id ORDER BY count(*) DESC LIMIT 1`).Row().Scan(&user) == nil {
		s.user = user
	}
	db.Raw(`SELECT id FROM payment_transactions WHERE user_id = ? AND kind = 'credit_purchase' ORDER BY created_at DESC LIMIT 20`, s.user).Pluck("id", &s.payments)
	db.Raw(`SELECT credit_purchase_id FROM payment_transactions WHERE user_id = ? AND kind = 'credit_purchase' ORDER BY created_at DESC LIMIT 20`, s.user).Pluck("credit_purchase_id", &s.purchases)
	if len(s.payments) == 0 {
		s.payments, s.purchases = []uuid.UUID{uuid.New()}, []uuid.UUID{uuid.New()}
	}
	return s
}

func refundChecks(db *gorm.DB, ids sampleIDs) []queryCheck {
	s := pickRefundSample(db, ids)
	p := s.period
	rev := func(f func(ctx context.Context, r repository.RevenueRepository)) func(context.Context, *gorm.DB) {
		return func(ctx context.Context, d *gorm.DB) { f(ctx, repository.NewRevenueRepository(d)) }
	}
	return []queryCheck{
		// The transactions page: lots (see "lots: of 20 purchases") + newest request per payment + payments by id.
		{"refunds: latest per payment (20)", 10 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewRefundRepository(d).Latest(ctx, s.payments)
		}},
		{"refunds: packs of 20 purchases", 10 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewRefundRepository(d).Packs(ctx, s.purchases)
		}},
		{"billing: payments by id (20)", 10 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewBillingRepository(d).TransactionsByIDs(ctx, s.payments)
		}},
		{"refunds: open request of a payment", 10 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewRefundRepository(d).OpenFor(ctx, s.payments[0])
		}},
		{"refunds: client list", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewRefundRepository(d).ListForUser(ctx, s.user, nil, 21)
		}},
		{"refunds: admin list (all)", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewRefundRepository(d).List(ctx, repository.RefundFilter{Limit: 21})
		}},
		{"refunds: admin list (open)", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewRefundRepository(d).List(ctx, repository.RefundFilter{Status: domain.RefundOpen, Limit: 21})
		}},
		{"refunds: admin list (user)", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewRefundRepository(d).List(ctx, repository.RefundFilter{UserID: &s.user, Limit: 21})
		}},
		// Revenue report, 30 days: one statement per figure.
		{"revenue: sales", 150 * time.Millisecond, rev(func(ctx context.Context, r repository.RevenueRepository) { _, _ = r.Sales(ctx, p, 800, 30) })},
		{"revenue: gross series", 150 * time.Millisecond, rev(func(ctx context.Context, r repository.RevenueRepository) { _, _ = r.GrossSeries(ctx, p) })},
		{"revenue: refund series", 100 * time.Millisecond, rev(func(ctx context.Context, r repository.RevenueRepository) { _, _ = r.RefundSeries(ctx, p) })},
		{"revenue: recognized series", 500 * time.Millisecond, rev(func(ctx context.Context, r repository.RevenueRepository) { _, _ = r.RecognizedSeries(ctx, p) })},
		{"revenue: deferred (live)", 300 * time.Millisecond, rev(func(ctx context.Context, r repository.RevenueRepository) {
			_, _ = r.DeferredMillicents(ctx, p.To, true)
		})},
		{"revenue: deferred (10 days ago)", 500 * time.Millisecond, rev(func(ctx context.Context, r repository.RevenueRepository) {
			_, _ = r.DeferredMillicents(ctx, time.Now().AddDate(0, 0, -10), false)
		})},
		{"revenue: advisor earnings", 100 * time.Millisecond, rev(func(ctx context.Context, r repository.RevenueRepository) { _, _ = r.AdvisorNetCredits(ctx, p) })},
		{"revenue: pack mix", 150 * time.Millisecond, rev(func(ctx context.Context, r repository.RevenueRepository) { _, _ = r.Packs(ctx, p) })},
		{"revenue: pack refunds", 150 * time.Millisecond, rev(func(ctx context.Context, r repository.RevenueRepository) { _, _ = r.PackRefunds(ctx, p) })},
		{"revenue: welcome offer", 150 * time.Millisecond, rev(func(ctx context.Context, r repository.RevenueRepository) { _, _ = r.WelcomeOffer(ctx, p) })},
		{"revenue: free trial", 150 * time.Millisecond, rev(func(ctx context.Context, r repository.RevenueRepository) { _, _ = r.FreeTrial(ctx, p) })},
		{"revenue: refund requests", 100 * time.Millisecond, rev(func(ctx context.Context, r repository.RevenueRepository) { _, _ = r.RefundRequests(ctx, p) })},
		{"revenue: disputes", 50 * time.Millisecond, rev(func(ctx context.Context, r repository.RevenueRepository) { _, _ = r.Disputes(ctx, p) })},
	}
}
