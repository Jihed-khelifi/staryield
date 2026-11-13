package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
)

// revisions 5.0: revenue analytics (§24.5). Every query below is set-based, is
// bound to a period on an indexed column (migration 0028 and earlier) and
// binds all values as parameters; the only spliced fragment is the date_trunc
// unit, which comes from the RevenueUnits allow-list.

// RevenueUnits maps a report interval to its date_trunc unit.
var RevenueUnits = map[string]string{"day": "day", "week": "week", "month": "month"}

// RevenuePeriod is the window every revenue query is about.
type RevenuePeriod struct {
	From, To time.Time
	Currency string
	// Unit is a RevenueUnits value.
	Unit string
}

func (p RevenuePeriod) unit() (string, error) {
	u, ok := RevenueUnits[p.Unit]
	if !ok {
		return "", fmt.Errorf("repository: revenue: unknown interval %q", p.Unit)
	}
	return u, nil
}

// RevenueSales totals the purchases paid in the period (refunded ones
// included: a refund is reported next to the purchase it undoes).
type RevenueSales struct {
	Purchases     int64
	PayingClients int64
	GrossCents    int64
	// ProcessingCents is Σ per purchase of round(price × fee_bps ÷ 10000) + fixed.
	ProcessingCents int64
}

// RevenueBucket is one point of a series (amounts in cents, or millicents for
// the recognized series, as documented by the method).
type RevenueBucket struct {
	Bucket time.Time
	Amount int64
	Count  int64
}

// RevenuePackRow is one pack's sales.
type RevenuePackRow struct {
	PackID     uuid.UUID
	Code       string
	Name       string
	Purchases  int64
	GrossCents int64
}

// RevenuePackRefund is what was refunded in the period on the purchases of one pack.
type RevenuePackRefund struct {
	PackID uuid.UUID
	Code   string
	Name   string
	Cents  int64
}

// RevenueOffer counts welcome-offer redemptions.
type RevenueOffer struct {
	Redemptions    int64
	UnverifiedCard int64
	RepeatBuyers   int64
}

// RevenueTrial counts sign-ups that received free trial minutes.
type RevenueTrial struct {
	Signups   int64
	Purchased int64
}

// RevenueRefundRequests are the refund requests created in the period.
type RevenueRefundRequests struct {
	Open, Approved, Denied, Credited, Withdrawn int64
	MedianHours                                 *float64
}

// RevenueRepository computes the revenue report's figures.
type RevenueRepository interface {
	Sales(ctx context.Context, p RevenuePeriod, feeBps, fixedCents int64) (RevenueSales, error)
	GrossSeries(ctx context.Context, p RevenuePeriod) ([]RevenueBucket, error)
	// RefundSeries: successful refunds of credit purchases, in cents.
	RefundSeries(ctx context.Context, p RevenuePeriod) ([]RevenueBucket, error)
	// RecognizedSeries: millicents of purchased credits consumed by chats and
	// released custom requests.
	RecognizedSeries(ctx context.Context, p RevenuePeriod) ([]RevenueBucket, error)
	// DeferredMillicents: unconsumed purchased-credit value at `at` (live
	// column when live, else reconstructed from the allocations after `at`).
	DeferredMillicents(ctx context.Context, at time.Time, live bool) (int64, error)
	AdvisorNetCredits(ctx context.Context, p RevenuePeriod) (int64, error)
	Packs(ctx context.Context, p RevenuePeriod) ([]RevenuePackRow, error)
	PackRefunds(ctx context.Context, p RevenuePeriod) ([]RevenuePackRefund, error)
	WelcomeOffer(ctx context.Context, p RevenuePeriod) (RevenueOffer, error)
	FreeTrial(ctx context.Context, p RevenuePeriod) (RevenueTrial, error)
	RefundRequests(ctx context.Context, p RevenuePeriod) (RevenueRefundRequests, error)
	Disputes(ctx context.Context, p RevenuePeriod) (int64, error)
}

// GormRevenueRepository implements RevenueRepository.
type GormRevenueRepository struct{ db *gorm.DB }

// NewRevenueRepository returns a GORM revenue repository.
func NewRevenueRepository(db *gorm.DB) *GormRevenueRepository { return &GormRevenueRepository{db: db} }

func (r *GormRevenueRepository) conn(ctx context.Context) *gorm.DB { return database.Conn(ctx, r.db) }

// paidStatuses is the purchase statuses that count as paid revenue.
const paidStatuses = `('succeeded', 'refunded')`

func (r *GormRevenueRepository) Sales(ctx context.Context, p RevenuePeriod, feeBps, fixedCents int64) (RevenueSales, error) {
	var out RevenueSales
	err := r.conn(ctx).Raw(`SELECT count(*) AS purchases, count(DISTINCT user_id) AS paying_clients,
		COALESCE(sum(price_cents), 0) AS gross_cents,
		COALESCE(sum((price_cents * ? + 5000) / 10000 + ?), 0) AS processing_cents
		FROM credit_purchases WHERE status IN `+paidStatuses+` AND currency = ? AND completed_at >= ? AND completed_at < ?`,
		feeBps, fixedCents, p.Currency, p.From, p.To).Scan(&out).Error
	return out, err
}

func (r *GormRevenueRepository) series(ctx context.Context, p RevenuePeriod, query string, args ...any) ([]RevenueBucket, error) {
	var out []RevenueBucket
	err := r.conn(ctx).Raw(query, args...).Scan(&out).Error
	return out, err
}

func (r *GormRevenueRepository) GrossSeries(ctx context.Context, p RevenuePeriod) ([]RevenueBucket, error) {
	u, err := p.unit()
	if err != nil {
		return nil, err
	}
	return r.series(ctx, p, `SELECT date_trunc('`+u+`', completed_at AT TIME ZONE 'UTC') AS bucket, sum(price_cents) AS amount, count(*) AS count
		FROM credit_purchases WHERE status IN `+paidStatuses+` AND currency = ? AND completed_at >= ? AND completed_at < ?
		GROUP BY 1 ORDER BY 1`, p.Currency, p.From, p.To)
}

func (r *GormRevenueRepository) RefundSeries(ctx context.Context, p RevenuePeriod) ([]RevenueBucket, error) {
	u, err := p.unit()
	if err != nil {
		return nil, err
	}
	return r.series(ctx, p, `SELECT date_trunc('`+u+`', created_at AT TIME ZONE 'UTC') AS bucket, sum(amount_cents) AS amount, count(*) AS count
		FROM payment_transactions WHERE kind = 'refund' AND status = 'succeeded' AND credit_purchase_id IS NOT NULL
		AND currency = ? AND created_at >= ? AND created_at < ?
		GROUP BY 1 ORDER BY 1`, p.Currency, p.From, p.To)
}

func (r *GormRevenueRepository) RecognizedSeries(ctx context.Context, p RevenuePeriod) ([]RevenueBucket, error) {
	u, err := p.unit()
	if err != nil {
		return nil, err
	}
	// Chat charges recognise the value of the lots they consumed when they are
	// charged; a custom request's held credits are recognised when the
	// request is released to the advisor (the hold entry's allocations).
	return r.series(ctx, p, `SELECT date_trunc('`+u+`', ts AT TIME ZONE 'UTC') AS bucket, sum(v)::bigint AS amount, count(*) AS count FROM (
			SELECT e.created_at AS ts, a.value_millicents AS v
			FROM credit_ledger e JOIN credit_lot_allocations a ON a.ledger_entry_id = e.id
			WHERE e.entry_type = 'chat_charge' AND e.created_at >= ? AND e.created_at < ?
			UNION ALL
			SELECT rel.created_at, a.value_millicents
			FROM credit_ledger rel
			JOIN credit_ledger h ON h.idempotency_key = 'request_hold:' || rel.reference_id::text
			JOIN credit_lot_allocations a ON a.ledger_entry_id = h.id AND a.credits > 0
			WHERE rel.entry_type = 'hold_release' AND rel.created_at >= ? AND rel.created_at < ?
		) x GROUP BY 1 ORDER BY 1`, p.From, p.To, p.From, p.To)
}

func (r *GormRevenueRepository) DeferredMillicents(ctx context.Context, at time.Time, live bool) (int64, error) {
	var v int64
	if live {
		err := r.conn(ctx).Raw(`SELECT COALESCE(sum(value_remaining_millicents), 0) FROM credit_lots
			WHERE source = 'purchase' AND value_remaining_millicents > 0`).Scan(&v).Error
		return v, err
	}
	// At a past instant: what the lots hold now plus what was consumed or
	// clawed back after it, minus what was restored after it (restores are
	// negative allocations), for the lots that existed then.
	err := r.conn(ctx).Raw(`SELECT
		(SELECT COALESCE(sum(l.value_remaining_millicents), 0) FROM credit_lots l
			WHERE l.source = 'purchase' AND l.created_at < ? AND l.value_remaining_millicents > 0)
		+ (SELECT COALESCE(sum(a.value_millicents), 0) FROM credit_lot_allocations a JOIN credit_lots l2 ON l2.id = a.lot_id
			WHERE a.created_at >= ? AND l2.source = 'purchase' AND l2.created_at < ?)`, at, at, at).Scan(&v).Error
	return v, err
}

func (r *GormRevenueRepository) AdvisorNetCredits(ctx context.Context, p RevenuePeriod) (int64, error) {
	var v int64
	err := r.conn(ctx).Raw(`SELECT COALESCE(sum(net_credits), 0) FROM psychic_earnings WHERE created_at >= ? AND created_at < ?`,
		p.From, p.To).Scan(&v).Error
	return v, err
}

func (r *GormRevenueRepository) Packs(ctx context.Context, p RevenuePeriod) ([]RevenuePackRow, error) {
	var out []RevenuePackRow
	err := r.conn(ctx).Raw(`SELECT p.pack_id, k.code, k.name, count(*) AS purchases, sum(p.price_cents) AS gross_cents
		FROM credit_purchases p JOIN credit_packs k ON k.id = p.pack_id
		WHERE p.status IN `+paidStatuses+` AND p.currency = ? AND p.completed_at >= ? AND p.completed_at < ?
		GROUP BY p.pack_id, k.code, k.name ORDER BY gross_cents DESC, k.code`, p.Currency, p.From, p.To).Scan(&out).Error
	return out, err
}

func (r *GormRevenueRepository) PackRefunds(ctx context.Context, p RevenuePeriod) ([]RevenuePackRefund, error) {
	var out []RevenuePackRefund
	err := r.conn(ctx).Raw(`SELECT cp.pack_id, k.code, k.name, sum(t.amount_cents) AS cents
		FROM payment_transactions t JOIN credit_purchases cp ON cp.id = t.credit_purchase_id JOIN credit_packs k ON k.id = cp.pack_id
		WHERE t.kind = 'refund' AND t.status = 'succeeded' AND t.currency = ? AND t.created_at >= ? AND t.created_at < ?
		GROUP BY cp.pack_id, k.code, k.name`, p.Currency, p.From, p.To).Scan(&out).Error
	return out, err
}

func (r *GormRevenueRepository) WelcomeOffer(ctx context.Context, p RevenuePeriod) (RevenueOffer, error) {
	var out RevenueOffer
	err := r.conn(ctx).Raw(`SELECT count(*) AS redemptions, count(*) FILTER (WHERE o.card_key = '') AS unverified_card,
		count(*) FILTER (WHERE EXISTS (SELECT 1 FROM credit_purchases cp
			WHERE cp.user_id = o.user_id AND cp.status IN `+paidStatuses+` AND cp.id <> o.purchase_id
			AND cp.completed_at > o.redeemed_at AND cp.completed_at <= o.redeemed_at + interval '30 days')) AS repeat_buyers
		FROM offer_redemptions o WHERE o.status = 'redeemed' AND o.redeemed_at >= ? AND o.redeemed_at < ?`, p.From, p.To).Scan(&out).Error
	return out, err
}

func (r *GormRevenueRepository) FreeTrial(ctx context.Context, p RevenuePeriod) (RevenueTrial, error) {
	var out RevenueTrial
	// The sign-up grant is recorded on the user.registered analytics event
	// (property signup_free_minutes, the value actually granted).
	err := r.conn(ctx).Raw(`WITH s AS (
			SELECT DISTINCT user_id FROM analytics_events
			WHERE name = 'user.registered' AND occurred_at >= ? AND occurred_at < ? AND user_id IS NOT NULL
			AND properties->>'signup_free_minutes' ~ '^[1-9][0-9]*$')
		SELECT count(*) AS signups, count(*) FILTER (WHERE EXISTS (SELECT 1 FROM credit_purchases cp
			WHERE cp.user_id = s.user_id AND cp.status IN `+paidStatuses+` AND cp.completed_at < ?)) AS purchased FROM s`,
		p.From, p.To, p.To).Scan(&out).Error
	return out, err
}

func (r *GormRevenueRepository) RefundRequests(ctx context.Context, p RevenuePeriod) (RevenueRefundRequests, error) {
	var out RevenueRefundRequests
	err := r.conn(ctx).Raw(`SELECT count(*) FILTER (WHERE status = 'open') AS open,
		count(*) FILTER (WHERE status = 'approved') AS approved, count(*) FILTER (WHERE status = 'denied') AS denied,
		count(*) FILTER (WHERE status = 'credited') AS credited, count(*) FILTER (WHERE status = 'withdrawn') AS withdrawn,
		percentile_cont(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM resolved_at - created_at) / 3600.0)
			FILTER (WHERE status IN ('approved', 'denied', 'credited')) AS median_hours
		FROM refund_requests WHERE created_at >= ? AND created_at < ?`, p.From, p.To).Scan(&out).Error
	return out, err
}

func (r *GormRevenueRepository) Disputes(ctx context.Context, p RevenuePeriod) (int64, error) {
	var v int64
	err := r.conn(ctx).Raw(`SELECT count(*) FROM payment_transactions WHERE status = 'disputed' AND kind = 'credit_purchase'
		AND updated_at >= ? AND updated_at < ?`, p.From, p.To).Scan(&v).Error
	return v, err
}
