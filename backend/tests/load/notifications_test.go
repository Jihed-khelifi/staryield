//go:build load

package load

// revisions 4.0: notification hardening (spec 23.6). The dataset comes from
// `go run ./cmd/seed -profile=large -extend "notifications"` (about ten
// notifications for each of the 200k synthetic users, 30% unread, spread over
// 240 days, 5% of the users with a push subscription).

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

type notificationSample struct {
	user       uuid.UUID // the user with the most notifications
	cursor     repository.Cursor
	midClient  uuid.UUID // a client id in the middle of the id range
	broadcast  uuid.UUID
	candidates []uuid.UUID
}

func pickNotificationSample(db *gorm.DB) notificationSample {
	s := notificationSample{user: uuid.New(), midClient: uuid.New(), broadcast: uuid.New(),
		cursor: repository.Cursor{At: time.Now().Add(-30 * 24 * time.Hour), ID: uuid.NewString()}}
	var one struct{ A uuid.UUID }
	if db.Table("notifications").Select("user_id AS a").Group("user_id").Order("count(*) DESC").Limit(1).Scan(&one).Error == nil && one.A != uuid.Nil {
		s.user = one.A
	}
	var row struct {
		ID        uuid.UUID
		CreatedAt time.Time
	}
	if db.Table("notifications").Select("id, created_at").Where("user_id = ?", s.user).Order("created_at DESC, id DESC").Offset(50).Limit(1).
		Scan(&row).Error == nil && row.ID != uuid.Nil {
		s.cursor = repository.Cursor{At: row.CreatedAt, ID: row.ID.String()}
	}
	one = struct{ A uuid.UUID }{}
	if db.Raw(`SELECT id AS a FROM users WHERE role = 'client' ORDER BY id OFFSET (SELECT count(*) / 2 FROM users WHERE role = 'client') LIMIT 1`).
		Scan(&one).Error == nil && one.A != uuid.Nil {
		s.midClient = one.A
	}
	one = struct{ A uuid.UUID }{}
	if db.Table("broadcasts").Select("id AS a").Order("created_at DESC").Limit(1).Scan(&one).Error == nil && one.A != uuid.Nil {
		s.broadcast = one.A
	}
	db.Raw(`SELECT id FROM users WHERE role = 'client' ORDER BY id DESC LIMIT 1000`).Scan(&s.candidates)
	return s
}

func notificationChecks(s notificationSample) []queryCheck {
	clients := []domain.Role{domain.RoleClient}
	return []queryCheck{
		{"notifications: unread count", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewNotificationRepository(d).CountUnread(ctx, s.user)
		}},
		{"notifications: page (deep keyset)", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			c := s.cursor
			_, _ = repository.NewNotificationRepository(d).List(ctx, s.user, &c, 20)
		}},
		{"notifications: retention batch select", 100 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewNotificationRepository(d).ExpiredBatch(ctx, time.Now().AddDate(0, 0, -180), repository.RetentionBatch)
		}},
		{"broadcast: recipient chunk (scan)", 50 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			after := s.midClient
			_, _ = repository.NewNotificationRepository(d).AudienceChunk(ctx, clients, nil, &after, nil, 1000)
		}},
		{"broadcast: recipient chunk (first)", 50 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewNotificationRepository(d).AudienceChunk(ctx, clients, nil, nil, nil, 1000)
		}},
		{"broadcast: recipient chunk (candidates)", 50 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewNotificationRepository(d).AudienceChunk(ctx, nil, s.candidates, nil, nil, 1000)
		}},
		{"broadcast: recipients count", 100 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewNotificationRepository(d).CountBroadcastRecipients(ctx, s.broadcast)
		}},
		{"broadcast: stale lookup", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewNotificationRepository(d).StaleBroadcasts(ctx, time.Now().Add(-2*time.Minute), 10)
		}},
	}
}

// TestLoadNotificationsAndBroadcast reads notifications under load while a
// broadcast fans out to every client. The broadcast is queued directly in the
// database with a stale heartbeat, so the API's broadcast-recovery job (every
// minute; keep the API's worker enabled) runs the real fan-out: chunked,
// idempotent, with pipelined realtime publishes. LOAD_BROADCAST_AUDIENCE picks
// the audience (default psychics: use clients on a dedicated load database to
// stress 200k recipients) and LOAD_BROADCAST_WAIT the time allowed (default
// 10m). The list, unread-count and mark-read endpoints must keep their
// objectives while it runs.
func TestLoadNotificationsAndBroadcast(t *testing.T) {
	cfg := LoadConfig()
	RequireAPI(t, cfg)
	ctx := context.Background()
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("database: %v", err)
	}
	audience := env("LOAD_BROADCAST_AUDIENCE", "psychics")
	var want int64
	role := map[string]string{"clients": "client", "psychics": "psychic"}[audience]
	if role == "" {
		t.Fatalf("LOAD_BROADCAST_AUDIENCE must be clients or psychics, got %q", audience)
	}
	db.Raw(`SELECT count(*) FROM users WHERE role = ? AND status = 'active'`, role).Scan(&want)

	pool := signedInClients(t, ctx, cfg, cfg.Users)
	fast := SLO{P95: 100 * time.Millisecond, P99: 250 * time.Millisecond, MaxErrRate: 0.001}
	write := SLO{P95: 200 * time.Millisecond, P99: 500 * time.Millisecond, MaxErrRate: 0.001}
	var marks atomic.Int64
	scenarios := []Scenario{
		{Name: "GET /me/notifications", Rate: cfg.Rate(150), Duration: cfg.Duration, Do: get("/api/v1/me/notifications?limit=20"), SLO: fast},
		{Name: "GET /me/notifications/unread-count", Rate: cfg.Rate(250), Duration: cfg.Duration, Do: get("/api/v1/me/notifications/unread-count"), SLO: fast},
		{Name: "POST /me/notifications/read (all)", Rate: cfg.Rate(40), Duration: cfg.Duration, SLO: write,
			Do: func(ctx context.Context, c *Client, _ int64) (int, error) {
				marks.Add(1)
				return c.Do(ctx, http.MethodPost, "/api/v1/me/notifications/read", map[string]any{"all": true}, nil)
			}},
	}

	// Queue the broadcast: the recovery job picks it up within a minute.
	id := domain.NewID()
	stale := time.Now().Add(-10 * time.Minute)
	b := domain.Broadcast{ID: id, Audience: audience, Title: domain.JSONMap{"en": "Load test broadcast"}, Body: domain.JSONMap{"en": "Fan-out under load"},
		Status: "queued", CreatedAt: stale, HeartbeatAt: &stale, TargetIDs: domain.StringList{}}
	if err := db.Create(&b).Error; err != nil {
		t.Fatalf("queue broadcast: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM notifications WHERE broadcast_id = ?`, id)
		db.Exec(`DELETE FROM broadcasts WHERE id = ?`, id)
	})
	queued := time.Now()

	done := make(chan struct{})
	var final domain.Broadcast
	var took time.Duration
	go func() {
		defer close(done)
		deadline := time.Now().Add(time.Duration(envInt("LOAD_BROADCAST_WAIT_MINUTES", 10)) * time.Minute)
		for time.Now().Before(deadline) {
			db.First(&final, "id = ?", id)
			if final.Status == "sent" || final.Status == "failed" {
				took = time.Since(queued)
				return
			}
			time.Sleep(2 * time.Second)
		}
	}()
	results := runAll(ctx, scenarios, pool)
	<-done
	Report(t, cfg, "notifications-and-broadcast", results)

	var rows, distinct int64
	db.Raw(`SELECT count(*), count(DISTINCT user_id) FROM notifications WHERE broadcast_id = ?`, id).Row().Scan(&rows, &distinct)
	t.Logf("broadcast to %s: status=%s recipients=%d rows=%d distinct=%d expected=%d, %s after queueing (includes up to 1 min waiting for the recovery job)",
		audience, final.Status, final.Recipients, rows, distinct, want, took.Round(time.Second))
	if final.Status != "sent" || rows != distinct || int64(final.Recipients) != rows {
		t.Fatalf("broadcast not delivered exactly once: %s", fmt.Sprint(final.Status, final.Recipients, rows, distinct))
	}
	if rows < want-want/50 { // accounts created while it ran may be missed; nothing else
		t.Fatalf("broadcast reached %d of %d recipients", rows, want)
	}
}
