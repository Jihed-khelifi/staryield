package seed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// Load-test data for the admin console's e-mail delivery view (growth: e-mail
// delivery): a month of finished outbox rows in every status, so the
// per-template counts, the recent problems and the backlog are checked at a
// realistic size (TestExplainHotQueries "mail health: …"). Run it alone with
//
//	go run ./cmd/seed -profile=large -extend "email outbox"
//
// Rows carry no recipient or body (final rows never do) and a placeholder
// address hash derived from the run's tag.
func init() {
	RegisterLarge(Generator{Name: "email outbox", Run: func(_ context.Context, d *Dataset) error { return d.emailOutbox() }})
}

// emailOutbox inserts about one e-mail per synthetic user and a half spread
// over the last 30 days (the outbox retention): 88% sent, the rest failed,
// dead, suppressed or cancelled, plus a few hundred queued rows being retried.
func (d *Dataset) emailOutbox() error {
	n := max(d.Opts.Users*3/2, 1)
	templates := []struct {
		name     string
		category domain.EmailCategory
		weight   int
	}{
		{"calculator_report", domain.EmailAccount, 30}, {"email_verify", domain.EmailSecurity, 20},
		{"notification:chat.missed", domain.EmailNotification, 15}, {"notification:purchase.succeeded", domain.EmailBilling, 15},
		{"password_reset", domain.EmailSecurity, 8}, {"notification:review.received", domain.EmailNotification, 7},
		{"notification:broadcast", domain.EmailMarketing, 5},
	}
	total := 0
	for _, t := range templates {
		total += t.weight
	}
	now := time.Now().UTC()
	batch := make([]domain.EmailOutbox, 0, d.Opts.BatchSize)
	flush := func() error {
		err := d.insert("email_outbox", &batch, len(batch), true)
		batch = batch[:0]
		return err
	}
	for i := 0; i < n; i++ {
		pick := d.Rand.IntN(total)
		tpl := templates[0]
		for _, t := range templates {
			if pick < t.weight {
				tpl = t
				break
			}
			pick -= t.weight
		}
		created := now.Add(-time.Duration(d.Rand.Int64N(int64(30 * 24 * time.Hour))))
		sum := sha256.Sum256([]byte(fmt.Sprintf("outbox:%s:%d", d.Tag, i)))
		row := domain.EmailOutbox{ID: d.ID(created), Category: tpl.category, Template: tpl.name, Locale: "en",
			ToHash: hex.EncodeToString(sum[:]), Status: domain.EmailSent, Attempts: 1, NextAttemptAt: created,
			Provider: "cloudflare", Timestamps: domain.Timestamps{CreatedAt: created, UpdatedAt: created.Add(2 * time.Second)}}
		switch r := d.Rand.Float64(); {
		case tpl.category == domain.EmailMarketing:
			row.Status, row.StatusReason, row.Attempts, row.Provider = domain.EmailCancelled, "no_marketing_provider", 0, ""
		case r < 0.88:
			sent := row.UpdatedAt
			row.SentAt, row.ProviderMessageID = &sent, fmt.Sprintf("seed-%d", i)
		case r < 0.93:
			row.Status, row.StatusReason, row.LastError = domain.EmailFailed, "rejected", "status 400 code 10202 email.sending.error.email.invalid"
		case r < 0.96:
			row.Status, row.StatusReason, row.Attempts = domain.EmailDead, "max_attempts", 8
			row.LastError = "status 403 code 10208 email.sending.error.email.recipient_plan_restricted"
		case r < 0.98:
			row.Status, row.StatusReason = domain.EmailSuppressed, "permanent_bounce"
		case r < 0.995:
			row.Status, row.StatusReason, row.Attempts, row.Provider = domain.EmailCancelled, "daily_cap", 0, ""
		default:
			// Still being retried after a provider error.
			row.Status, row.Attempts, row.ToSealed, row.PayloadSealed = domain.EmailQueued, 3, "seed", "seed"
			row.LastError = "status 429 code 10004 email.sending.error.throttled"
			row.NextAttemptAt = now.Add(time.Duration(d.Rand.IntN(3600)) * time.Second)
		}
		batch = append(batch, row)
		if len(batch) == cap(batch) {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := flush(); err != nil {
		return err
	}
	d.Log.Info("email outbox generated", zap.Int("rows", d.Counts["email_outbox"]))
	return nil
}
