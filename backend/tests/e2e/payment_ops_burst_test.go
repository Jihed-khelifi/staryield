package e2e

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
)

// revisions 6.0 (§26.10): a webhook burst. Thousands of deliveries about
// many payments arrive duplicated, concurrently and out of order; every one is
// answered 200, every event is logged once, and the final state is the one the
// events describe whatever their order: every dispute won (restored, payment
// back to its earlier status), one dispute and one fraud warning per payment,
// card details stored, review flags set.
func TestWebhookBurstWithDuplicatesAndOutOfOrderEvents(t *testing.T) {
	const payments, copies, workers = 120, 3, 24
	w := newOpsWorld(t)
	db := w.e.app.DB

	refs := make([]string, payments)
	for i := range refs {
		refs[i] = fmt.Sprintf("ch_burst_%s_%d", uuid.NewString()[:8], i)
		id := uuid.New()
		if err := db.Exec(`INSERT INTO payment_transactions (id, user_id, kind, status, amount_cents, currency, credits, provider, provider_ref, description, idempotency_key)
			VALUES (?, ?, 'credit_purchase', 'succeeded', 999, 'USD', 250, 'sandbox_a', ?, 'Burst pack', ?)`, id, w.c.user.ID, refs[i], "burst:"+id.String()).Error; err != nil {
			t.Fatal(err)
		}
	}
	run := uuid.NewString()[:8]
	var events []payment.WebhookEvent
	for i, ref := range refs {
		dp := fmt.Sprintf("dp_burst_%s_%d", run, i)
		mk := func(kind string, edit func(*payment.WebhookEvent)) {
			e := payment.WebhookEvent{ID: fmt.Sprintf("evt_burst_%s_%d_%s", run, i, kind), ProviderRef: ref}
			edit(&e)
			events = append(events, e)
		}
		mk("created", func(e *payment.WebhookEvent) { e.Type = "charge.disputed"; openDispute(dp, "needs_response")(e) })
		mk("review", func(e *payment.WebhookEvent) { e.Type = "charge.dispute_updated"; openDispute(dp, "under_review")(e) })
		mk("withdrawn", func(e *payment.WebhookEvent) { e.Type = "charge.dispute_funds_withdrawn"; e.DisputeRef = dp })
		mk("reinstated", func(e *payment.WebhookEvent) { e.Type = "charge.dispute_funds_reinstated"; e.DisputeRef = dp })
		mk("won", func(e *payment.WebhookEvent) { e.Type = "charge.dispute_closed"; closedDispute(dp, "won")(e) })
		mk("fraud", func(e *payment.WebhookEvent) {
			e.Type = "fraud.early_warning"
			fraudWarning(fmt.Sprintf("issfr_burst_%s_%d", run, i), false)(e)
		})
		mk("review_open", func(e *payment.WebhookEvent) { e.Type = "review.opened" })
		mk("details", func(e *payment.WebhookEvent) {
			e.Type = "charge.succeeded"
			e.Card, e.Risk, e.ThreeDS = &payment.CardMeta{Country: "NL"}, &payment.RiskInfo{Level: payment.RiskNormal, Score: 7}, "authenticated"
		})
	}
	deliveries := make([]payment.WebhookEvent, 0, len(events)*copies)
	for c := 0; c < copies; c++ {
		deliveries = append(deliveries, events...)
	}
	rand.New(rand.NewSource(20261119)).Shuffle(len(deliveries), func(i, j int) { deliveries[i], deliveries[j] = deliveries[j], deliveries[i] })

	start := time.Now()
	jobs := make(chan payment.WebhookEvent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	statuses := map[int]int{}
	for k := 0; k < workers; k++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for evt := range jobs {
				st := signedWebhook(w.e, "sandbox_a", evt, "").status
				mu.Lock()
				statuses[st]++
				mu.Unlock()
			}
		}()
	}
	for _, d := range deliveries {
		jobs <- d
	}
	close(jobs)
	wg.Wait()
	t.Logf("%d deliveries (%d events × %d) in %s", len(deliveries), len(events), copies, time.Since(start).Round(time.Millisecond))
	if statuses[http.StatusOK] != len(deliveries) {
		t.Fatalf("every delivery must be acknowledged: %v", statuses)
	}

	prefix := "evt_burst_" + run + "_%"
	if n := w.count("SELECT count(*) FROM webhook_events WHERE provider = 'sandbox_a' AND event_id LIKE ?", prefix); n != int64(len(events)) {
		t.Fatalf("logged %d events, want %d (de-duplicated by event id)", n, len(events))
	}
	if n := w.count("SELECT count(*) FROM webhook_events WHERE event_id LIKE ? AND status <> 'processed'", prefix); n != 0 {
		t.Fatalf("%d events are not processed", n)
	}
	dp := "dp_burst_" + run + "_%"
	if n := w.count("SELECT count(*) FROM payment_disputes WHERE dispute_ref LIKE ?", dp); n != payments {
		t.Fatalf("disputes: %d, want one per payment", n)
	}
	if n := w.count("SELECT count(*) FROM payment_disputes WHERE dispute_ref LIKE ? AND status = 'won' AND closed_at IS NOT NULL", dp); n != payments {
		t.Fatalf("won disputes: %d", n)
	}
	if n := w.count(`SELECT count(*) FROM payment_transactions WHERE user_id = ? AND description = 'Burst pack' AND status = 'succeeded' AND in_review
		AND card_country = 'NL' AND risk_level = 'normal' AND three_ds = 'authenticated'`, w.c.user.ID); n != payments {
		t.Fatalf("payments back to succeeded with their details: %d", n)
	}
	if n := w.count("SELECT count(*) FROM payment_fraud_warnings WHERE warning_ref LIKE ?", "issfr_burst_"+run+"_%"); n != payments {
		t.Fatalf("fraud warnings: %d", n)
	}
	if n := w.count("SELECT count(*) FROM payment_disputes WHERE dispute_ref LIKE ? AND credits_restored <> credits_frozen", dp); n != 0 {
		t.Fatalf("%d disputes with credits left frozen", n)
	}
	w.e.invariant(w.c.user.ID)
}
