package e2e

import (
	"bytes"
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/push"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/seed"
)

// The "notifications" load generator: about ten notifications per synthetic
// user, skewed, ~30% unread, spread over 240 days, plus harmless push
// subscriptions for ~5% of the users; safe to re-run.
func TestLargeSeedNotificationsGenerator(t *testing.T) {
	if !available {
		t.Skip("infrastructure unavailable")
	}
	ctx := context.Background()
	hasher := security.NewPasswordHasher(1024, 1, 1)
	sealer, err := security.NewSealer(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	opts := seed.LargeOptions{Users: 2000, Psychics: 20, Conversations: 100, MessagesPerConversation: 2, Reviews: 20, LedgerRows: 100,
		Seed: 11, BatchSize: 500, Sealer: sealer}
	db := isolatedSchema(t)
	rep, err := seed.Large(ctx, db, hasher, opts, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	users := int64(2020)
	var rows, unread, old, maxPerUser, subs int64
	db.Raw(`SELECT count(*) FROM notifications`).Scan(&rows)
	db.Raw(`SELECT count(*) FROM notifications WHERE read_at IS NULL`).Scan(&unread)
	db.Raw(`SELECT count(*) FROM notifications WHERE created_at < now() - interval '180 days'`).Scan(&old)
	db.Raw(`SELECT max(c) FROM (SELECT count(*) c FROM notifications GROUP BY user_id) s`).Scan(&maxPerUser)
	db.Raw(`SELECT count(*) FROM push_subscriptions`).Scan(&subs)
	avg := float64(rows) / float64(users)
	if avg < 6 || avg > 14 || int64(rep.Counts["notifications"]) != rows {
		t.Fatalf("notifications: %d rows, %.1f per user, report %d", rows, avg, rep.Counts["notifications"])
	}
	if share := float64(unread) / float64(rows); share < 0.25 || share > 0.35 {
		t.Fatalf("unread share %.2f", share)
	}
	if oldShare := float64(old) / float64(rows); oldShare < 0.15 || oldShare > 0.35 { // 60 of 240 days
		t.Fatalf("expired share %.2f: retention needs real work", oldShare)
	}
	if maxPerUser < 100 {
		t.Fatalf("no heavy user: max %d notifications", maxPerUser)
	}
	if share := float64(subs) / float64(users); share < 0.02 || share > 0.09 {
		t.Fatalf("push subscriptions: %d (%.3f)", subs, share)
	}

	// Subscriptions are harmless: the API can open them, and the sender
	// refuses their endpoints before any network request.
	var sub domain.PushSubscription
	if err := db.First(&sub).Error; err != nil {
		t.Fatal(err)
	}
	endpoint, err := sealer.Open(sub.EndpointSealed, "push:"+sub.EndpointHash)
	if err != nil || len(sub.EndpointHash) != 64 {
		t.Fatalf("sealed endpoint: %v", err)
	}
	if _, err := push.ValidateEndpoint(endpoint); err == nil {
		t.Fatalf("seeded endpoint %q would be contacted", endpoint)
	}

	// Re-running adds nothing; -force adds another population's worth.
	if _, err := seed.Extend(ctx, db, []string{"notifications"}, opts, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	var again int64
	db.Raw(`SELECT count(*) FROM notifications`).Scan(&again)
	if again != rows {
		t.Fatalf("re-run changed the row count: %d -> %d", rows, again)
	}
	opts.Force = true
	if _, err := seed.Extend(ctx, db, []string{"notifications"}, opts, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	db.Raw(`SELECT count(*) FROM notifications`).Scan(&again)
	if again <= rows+rows/2 {
		t.Fatalf("forced re-run: %d -> %d", rows, again)
	}
}
