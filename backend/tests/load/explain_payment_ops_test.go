//go:build load

package load

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 6.0 (§26.5): card payment operations. The purchase gate asks "does
// this user have an open dispute?" before every purchase (one indexed probe,
// partial index payment_disputes_open_user_idx); the console pages the webhook
// log and the disputes with keyset pages and resolves the disputes of a page of
// payments with one query; a payment_method.detached webhook finds the sealed
// card by the hash of its reference; the evidence draft reads a client's
// readings after a purchase. Reads only: EXPLAIN ANALYZE executes the statement.

// seedPaymentOps gives the three new tables real volume when the dataset has
// none (the seeder predates them): 300k webhook log rows over the last days,
// and a dispute for about one in eight successful credit purchases, a handful
// of them open. It is idempotent (a marker prefix) and touches only these tables.
func seedPaymentOps(db *gorm.DB) {
	var n int64
	db.Raw(`SELECT count(*) FROM payment_disputes`).Scan(&n)
	if n >= 1000 {
		return
	}
	db.Exec(`INSERT INTO webhook_events (id, provider, event_id, event_type, payload_sha256, status, attempts, provider_type, object_ref, created_at, updated_at, processed_at)
		SELECT gen_random_uuid(), CASE WHEN g % 10 = 0 THEN 'sandbox_b' ELSE 'sandbox_a' END, 'load_evt_' || g, 'charge.succeeded', repeat('0', 64),
		       (ARRAY['processed','processed','processed','processed','ignored','failed'])[1 + g % 6], 1, 'payment_intent.succeeded', 'pi_load_' || g,
		       now() - (g * 2 || ' seconds')::interval, now(), now()
		FROM generate_series(1, 300000) g ON CONFLICT DO NOTHING`)
	db.Exec(`INSERT INTO payment_disputes (id, provider, dispute_ref, transaction_id, user_id, amount_cents, currency, reason, status, prior_status, created_at, updated_at, closed_at)
		SELECT gen_random_uuid(), t.provider, 'load_dp_' || t.id, t.id, t.user_id, t.amount_cents, t.currency, 'fraudulent',
		       (ARRAY['won','lost','warning_closed','won','needs_response','under_review','lost','won'])[1 + (row_number() OVER ())::int % 8],
		       'succeeded', t.created_at + interval '20 days', now(), now()
		FROM (SELECT * FROM payment_transactions WHERE kind = 'credit_purchase' AND status = 'succeeded' AND (hashtext(id::text) % 8) = 0 LIMIT 40000) t
		ON CONFLICT DO NOTHING`)
	db.Exec(`UPDATE payment_disputes SET closed_at = NULL WHERE status IN ('needs_response', 'under_review')`)
	db.Exec(`ANALYZE webhook_events`)
	db.Exec(`ANALYZE payment_disputes`)
}

func paymentOpsChecks(db *gorm.DB, ids sampleIDs) []queryCheck {
	seedPaymentOps(db)
	var user uuid.UUID
	var payments []uuid.UUID
	if db.Raw(`SELECT user_id FROM payment_disputes WHERE status = 'needs_response' LIMIT 1`).Row().Scan(&user) != nil {
		user = ids.ledgerUser
	}
	db.Raw(`SELECT id FROM payment_transactions WHERE user_id = ? ORDER BY created_at DESC LIMIT 20`, user).Pluck("id", &payments)
	if len(payments) == 0 {
		payments = []uuid.UUID{uuid.New()}
	}
	var at time.Time
	var refHash string
	if db.Raw(`SELECT created_at FROM webhook_events WHERE provider = 'sandbox_a' ORDER BY created_at DESC OFFSET 500 LIMIT 1`).Row().Scan(&at) != nil {
		at = time.Now()
	}
	_ = db.Raw(`SELECT coalesce(max(provider_method_ref_hash), '') FROM payment_methods WHERE provider_method_ref_hash <> ''`).Row().Scan(&refHash)
	if refHash == "" {
		// Cards saved before 0033 carry no hash (and an empty hash is answered
		// without a query): probe with a well-formed one so the plan is checked.
		refHash = strings.Repeat("0", 64)
	}
	cur := &repository.Cursor{At: at, ID: uuid.NewString()}
	ops := func(f func(ctx context.Context, r repository.PaymentOpsRepository)) func(context.Context, *gorm.DB) {
		return func(ctx context.Context, d *gorm.DB) { f(ctx, repository.NewPaymentOpsRepository(d)) }
	}
	return []queryCheck{
		{"payment ops: open dispute of a user (purchase gate)", 5 * time.Millisecond, ops(func(ctx context.Context, r repository.PaymentOpsRepository) {
			_, _ = r.HasOpenDispute(ctx, user)
		})},
		{"payment ops: webhook log (newest, all)", 25 * time.Millisecond, ops(func(ctx context.Context, r repository.PaymentOpsRepository) {
			_, _ = r.ListWebhooks(ctx, repository.WebhookFilter{Limit: 21})
		})},
		{"payment ops: webhook log (provider, page)", 25 * time.Millisecond, ops(func(ctx context.Context, r repository.PaymentOpsRepository) {
			_, _ = r.ListWebhooks(ctx, repository.WebhookFilter{Provider: "sandbox_a", Cursor: cur, Limit: 21})
		})},
		{"payment ops: webhook log (failed)", 25 * time.Millisecond, ops(func(ctx context.Context, r repository.PaymentOpsRepository) {
			_, _ = r.ListWebhooks(ctx, repository.WebhookFilter{Status: domain.WebhookFailed, Limit: 21})
		})},
		{"payment ops: webhook counts (24 h)", 50 * time.Millisecond, ops(func(ctx context.Context, r repository.PaymentOpsRepository) {
			_, _ = r.WebhookCounts(ctx, "sandbox_a", time.Now().Add(-24*time.Hour))
		})},
		{"payment ops: last webhook of a provider", 10 * time.Millisecond, ops(func(ctx context.Context, r repository.PaymentOpsRepository) {
			_, _ = r.LastWebhook(ctx, "sandbox_a")
		})},
		{"payment ops: disputes (open)", 25 * time.Millisecond, ops(func(ctx context.Context, r repository.PaymentOpsRepository) {
			_, _ = r.ListDisputes(ctx, repository.DisputeFilter{Status: domain.DisputeNeedsResponse, Limit: 21})
		})},
		{"payment ops: disputes (of a user)", 10 * time.Millisecond, ops(func(ctx context.Context, r repository.PaymentOpsRepository) {
			_, _ = r.ListDisputes(ctx, repository.DisputeFilter{UserID: &user, Limit: 21})
		})},
		{"payment ops: disputes (all, page)", 25 * time.Millisecond, ops(func(ctx context.Context, r repository.PaymentOpsRepository) {
			_, _ = r.ListDisputes(ctx, repository.DisputeFilter{Cursor: cur, Limit: 21})
		})},
		{"payment ops: disputes of 20 payments", 10 * time.Millisecond, ops(func(ctx context.Context, r repository.PaymentOpsRepository) {
			_, _ = r.DisputesByTransactions(ctx, payments)
		})},
		{"payment ops: fraud warnings (page)", 25 * time.Millisecond, ops(func(ctx context.Context, r repository.PaymentOpsRepository) {
			_, _ = r.ListFraudWarnings(ctx, repository.FraudWarningFilter{Action: domain.FraudActionNone, Cursor: cur, Limit: 21})
		})},
		{"payment ops: saved card by reference hash", 10 * time.Millisecond, ops(func(ctx context.Context, r repository.PaymentOpsRepository) {
			_, _ = r.MethodsByRefHash(ctx, "stripe", refHash)
		})},
		{"payment ops: usage log of a client", 25 * time.Millisecond, ops(func(ctx context.Context, r repository.PaymentOpsRepository) {
			_, _ = r.UsageLog(ctx, ids.sessionClient, time.Now().AddDate(0, 0, -90), 51)
		})},
	}
}
