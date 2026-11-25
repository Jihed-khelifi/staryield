package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// Growth: e-mail verification. The token queries of the verification flow,
// kept apart from the shared identity code.
//
// Lock order: every transaction of the flow locks the user row first and
// only then touches the user's tokens (issuing a link invalidates the open
// ones; opening a link consumes one). Opening a link therefore reads its
// token without a lock (FindOpen), locks the user, then uses the token
// (Use): consuming the token first would take the two locks in the opposite
// order and deadlock against a concurrent resend.

// VerificationTokenRepository is what the e-mail verification flow needs
// from the single-use token store.
type VerificationTokenRepository interface {
	ActionTokenRepository
	// FindOpen returns an unused, unexpired token by hash without locking
	// it; anything else is ErrNotFound.
	FindOpen(ctx context.Context, hash string, purpose domain.ActionPurpose, now time.Time) (*domain.ActionToken, error)
	// Use marks an open token used. ErrNotFound when it was used or expired
	// in the meantime.
	Use(ctx context.Context, id uuid.UUID, now time.Time) error
	// IssuedSince returns when the user's tokens of a purpose created after
	// since were created (used or not), newest first, at most limit.
	IssuedSince(ctx context.Context, userID uuid.UUID, purpose domain.ActionPurpose, since time.Time, limit int) ([]time.Time, error)
}

// FindOpen implements VerificationTokenRepository (user_action_tokens_hash_key).
func (r *GormActionTokenRepository) FindOpen(ctx context.Context, hash string, purpose domain.ActionPurpose, now time.Time) (*domain.ActionToken, error) {
	var t domain.ActionToken
	err := database.Conn(ctx, r.db).
		Where("token_hash = ? AND purpose = ? AND used_at IS NULL AND expires_at > ?", hash, purpose, now).
		First(&t).Error
	if err != nil {
		return nil, notFound(err)
	}
	return &t, nil
}

// Use implements VerificationTokenRepository: one conditional UPDATE by
// primary key (the row stays locked until the transaction ends).
func (r *GormActionTokenRepository) Use(ctx context.Context, id uuid.UUID, now time.Time) error {
	res := database.Conn(ctx, r.db).Model(&domain.ActionToken{}).
		Where("id = ? AND used_at IS NULL AND expires_at > ?", id, now).
		Updates(map[string]any{"used_at": now, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// IssuedSince implements VerificationTokenRepository
// (user_action_tokens_user_issued_idx, migration 0039).
func (r *GormActionTokenRepository) IssuedSince(ctx context.Context, userID uuid.UUID, purpose domain.ActionPurpose, since time.Time, limit int) ([]time.Time, error) {
	if limit <= 0 {
		return nil, nil
	}
	var out []time.Time
	err := database.Conn(ctx, r.db).Model(&domain.ActionToken{}).
		Where("user_id = ? AND purpose = ? AND created_at > ?", userID, purpose, since).
		Order("created_at DESC").Limit(limit).Pluck("created_at", &out).Error
	return out, err
}
