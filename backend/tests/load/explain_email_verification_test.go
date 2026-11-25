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

// Growth: e-mail verification. Opening a link looks its token up by hash (the
// unique index of user_action_tokens), a resend reads the user's newest open
// token (the partial index on unused tokens), and the admin console lists
// users by verification status paging by (created_at, id): the partial
// indexes of migration 0039 keep those pages index-ordered.
func emailVerificationChecks(db *gorm.DB) []queryCheck {
	var hash string
	if db.Raw(`SELECT token_hash FROM user_action_tokens ORDER BY created_at DESC LIMIT 1`).Row().Scan(&hash) != nil || hash == "" {
		hash = "0000000000000000000000000000000000000000000000000000000000000000"
	}
	var user uuid.UUID
	if db.Raw(`SELECT user_id FROM user_action_tokens ORDER BY created_at DESC LIMIT 1`).Row().Scan(&user) != nil {
		user = uuid.New()
	}
	var cursor repository.Cursor
	var at time.Time
	var id string
	if db.Raw(`SELECT created_at, id::text FROM users ORDER BY created_at DESC, id DESC OFFSET 500 LIMIT 1`).Row().Scan(&at, &id) == nil {
		cursor = repository.Cursor{At: at, ID: id}
	} else {
		cursor = repository.Cursor{At: time.Now(), ID: "00000000-0000-7000-8000-000000000000"}
	}
	yes, no := true, false
	users := func(f repository.UserFilter) func(context.Context, *gorm.DB) {
		return func(ctx context.Context, d *gorm.DB) { _, _ = repository.NewUserRepository(d).List(ctx, f) }
	}
	return []queryCheck{
		{"e-mail verification: token lookup by hash", 10 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewActionTokenRepository(d).Consume(ctx, hash, domain.ActionEmailVerify, time.Now())
		}},
		{"e-mail verification: newest open token", 10 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewActionTokenRepository(d).LatestOpen(ctx, user, domain.ActionEmailVerify, time.Now())
		}},
		{"admin users: not verified", 25 * time.Millisecond, users(repository.UserFilter{Verified: &no, Limit: 51})},
		{"admin users: not verified (page 10)", 25 * time.Millisecond, users(repository.UserFilter{Verified: &no, Limit: 51, Cursor: &cursor})},
		{"admin users: verified", 25 * time.Millisecond, users(repository.UserFilter{Verified: &yes, Limit: 51})},
		{"admin users: verified (page 10)", 25 * time.Millisecond, users(repository.UserFilter{Verified: &yes, Limit: 51, Cursor: &cursor})},
	}
}
