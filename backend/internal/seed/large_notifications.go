package seed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 4.0: notification hardening. In-app notifications at production
// scale, so the list, unread-count, retention and broadcast fan-out queries
// are checked against realistic sizes (the load database otherwise holds a
// couple of hundred rows).
//
// About ten notifications per synthetic user on average, heavily skewed (most
// users a few, some dozens, a handful hundreds), mixed kinds for the user's
// role, about 30% unread, created over the last 240 days so the 180-day
// retention job has real work. Roughly 5% of the synthetic users also get a
// push subscription (see pushSubscriptions for why it is harmless).
//
// Run: go run ./cmd/seed -profile=large -extend "notifications"
func init() {
	RegisterLarge(Generator{Name: "notifications", Run: func(_ context.Context, d *Dataset) error { return d.notifications() }})
}

// notificationWindow is how far back generated notifications reach; the
// retention job keeps the last 180 days.
const notificationWindow = 240 * 24 * time.Hour

// notificationCount draws how many notifications a user has: 88% a few
// (1-7), 11% dozens (10-49), 1% hundreds (100-399); mean about 9.5.
func (d *Dataset) notificationCount() int {
	switch d.pick(88, 11, 1) {
	case 0:
		return 1 + d.Rand.IntN(7)
	case 1:
		return 10 + d.Rand.IntN(40)
	}
	return 100 + d.Rand.IntN(300)
}

var (
	clientNotificationKinds  = []string{"purchase.succeeded", "chat.review_prompt", "request.updated", "credits.adjusted", "broadcast", "waitlist.available"}
	psychicNotificationKinds = []string{"chat.requested", "chat.missed", "review.received", "request.created", "withdrawal.updated", "broadcast"}
)

func (d *Dataset) notificationFor(kind string) (data map[string]any, link string) {
	id := func() string { return d.ID(d.Now).String() }
	switch kind {
	case "purchase.succeeded":
		return map[string]any{"purchase_id": id(), "credits": 50 * (1 + d.Rand.IntN(10))}, "/profile"
	case "chat.review_prompt":
		return map[string]any{"session_id": id(), "psychic_name": "Advisor", "psychic_slug": "advisor"}, "/chatroom"
	case "request.updated":
		return map[string]any{"request_id": id(), "service_name": "Birth chart reading", "status": "accepted"}, "/profile/requests"
	case "credits.adjusted":
		return map[string]any{"amount": 5 * (1 + d.Rand.IntN(20))}, "/profile"
	case "waitlist.available":
		return map[string]any{"psychic_name": "Advisor", "psychic_slug": "advisor"}, "/psychics"
	case "chat.requested", "chat.missed":
		return map[string]any{"session_id": id(), "client_name": "Nova"}, "/chatroom"
	case "review.received":
		return map[string]any{"review_id": id(), "rating": 3 + d.Rand.IntN(3)}, "/profile/analytics"
	case "request.created":
		return map[string]any{"request_id": id(), "service_name": "Custom reading", "client_name": "Nova"}, "/profile/requests"
	case "withdrawal.updated":
		return map[string]any{"withdrawal_id": id(), "status": "paid", "amount_credits": 100 * (1 + d.Rand.IntN(10))}, "/profile/earnings"
	}
	return map[string]any{"title": map[string]any{"en": "News"}, "body": map[string]any{"en": "A message from Staryield"}}, "/psychics"
}

func (d *Dataset) notifications() error {
	var seeded bool
	if err := d.DB.Raw(`SELECT EXISTS (SELECT 1 FROM notifications n JOIN users u ON u.id = n.user_id WHERE u.email LIKE ?)`,
		"%@"+LoadEmailDomain).Row().Scan(&seeded); err != nil {
		return err
	}
	if seeded && !d.Opts.Force {
		d.Log.Info("notifications skipped: load notifications already exist (use -force to add more)")
		return nil
	}
	batch := make([]domain.Notification, 0, d.Opts.BatchSize)
	flush := func() error {
		err := d.insert("notifications", &batch, len(batch), true)
		batch = batch[:0]
		return err
	}
	unread := 0
	add := func(users []uuid.UUID, kinds []string) error {
		for _, uid := range users {
			for k, n := 0, d.notificationCount(); k < n; k++ {
				at := d.Now.Add(-time.Duration(d.Rand.Int64N(int64(notificationWindow)))).Truncate(time.Second)
				kind := kinds[d.Rand.IntN(len(kinds))]
				data, link := d.notificationFor(kind)
				row := domain.Notification{ID: d.ID(at), UserID: uid, Kind: kind, Data: domain.JSONMap(data), Link: link, CreatedAt: at, UpdatedAt: at}
				if d.Rand.IntN(100) < 30 {
					unread++
				} else {
					read := at.Add(time.Duration(1+d.Rand.IntN(72*60)) * time.Minute)
					if read.After(d.Now) {
						read = d.Now
					}
					row.ReadAt, row.UpdatedAt = &read, read
				}
				batch = append(batch, row)
				if len(batch) == cap(batch) {
					if err := flush(); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	if err := add(d.Clients, clientNotificationKinds); err != nil {
		return err
	}
	if err := add(d.Psychics, psychicNotificationKinds); err != nil {
		return err
	}
	if err := flush(); err != nil {
		return err
	}
	if err := d.pushSubscriptions(); err != nil {
		return err
	}
	d.Log.Info("notifications generated", zap.Int("rows", d.Counts["notifications"]), zap.Int("unread", unread),
		zap.Int("push_subscriptions", d.Counts["push_subscriptions"]))
	return nil
}

// pushSubscriptions gives about 5% of the synthetic users one subscription.
//
// They are harmless by construction, in three independent ways:
//   - the endpoint host (push.load.staryield.test) is not on the push
//     allow-list, so push.WebPush.Send refuses it before any network request
//     (it answers "gone" and the row is simply deleted) and SafeTransport
//     would refuse the dial anyway; nothing is ever sent to a real push
//     service;
//   - Web Push is off unless VAPID keys are configured, which a load run does
//     not do (the sender is nil: PushService.Enabled() is false);
//   - the keys are random bytes, never valid subscription keys.
//
// The sealed columns use the same sealer as the API, so the API can open
// them (and prune them) like any other row. Skipped without a sealer.
func (d *Dataset) pushSubscriptions() error {
	if d.Sealer == nil {
		d.Log.Warn("push subscriptions skipped: no sealer configured")
		return nil
	}
	var existing bool
	if err := d.DB.Raw(`SELECT EXISTS (SELECT 1 FROM push_subscriptions s JOIN users u ON u.id = s.user_id WHERE u.email LIKE ?)`,
		"%@"+LoadEmailDomain).Row().Scan(&existing); err != nil {
		return err
	}
	if existing && !d.Opts.Force {
		return nil
	}
	users := append(append([]uuid.UUID{}, d.Clients...), d.Psychics...)
	batch := make([]domain.PushSubscription, 0, d.Opts.BatchSize)
	flush := func() error {
		err := d.insert("push_subscriptions", &batch, len(batch), true)
		batch = batch[:0]
		return err
	}
	for i, uid := range users {
		if d.Rand.IntN(100) >= 5 {
			continue
		}
		endpoint := fmt.Sprintf("https://push.load.staryield.test/%s/%d/%d", d.Tag, d.Now.Unix(), i) // unique per run (forced re-runs)
		sum := sha256.Sum256([]byte(endpoint))
		hash := hex.EncodeToString(sum[:]) // 64 chars, like the API's keyed hash
		ad := "push:" + hash
		at := d.Now.Add(-time.Duration(d.Rand.Int64N(int64(historyWindow)))).Truncate(time.Second)
		sub := domain.PushSubscription{ID: d.ID(at), UserID: uid, EndpointHash: hash, UserAgent: "load-test",
			SoftDeleteTimestamps: domain.SoftDeleteTimestamps{CreatedAt: at, UpdatedAt: at}}
		var err error
		if sub.EndpointSealed, err = d.Sealer.Seal(endpoint, ad); err != nil {
			return err
		}
		if sub.P256dhSealed, err = d.Sealer.Seal("load-test-not-a-key", ad); err != nil {
			return err
		}
		if sub.AuthSealed, err = d.Sealer.Seal("load-test-not-a-secret", ad); err != nil {
			return err
		}
		batch = append(batch, sub)
		if len(batch) == cap(batch) {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	return flush()
}
