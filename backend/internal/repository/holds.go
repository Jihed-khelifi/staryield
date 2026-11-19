package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 6.0 (§26.1): the free-trial hold. These methods belong to
// ChatRepository; the hold rows live in chat_session_holds (migration 0032).

// HoldStats are the free-trial hold figures of a window (admin card).
type HoldStats struct {
	Started               int64
	Converted             int64
	Expired               int64
	Revealed              int64
	CreditsFromConversion int64
	CreditsFromReveals    int64
}

// CreateHold implements ChatRepository.
func (r *GormChatRepository) CreateHold(ctx context.Context, h *domain.ChatHold) error {
	return database.Conn(ctx, r.db).Create(h).Error
}

// GetHold implements ChatRepository.
func (r *GormChatRepository) GetHold(ctx context.Context, id uuid.UUID) (*domain.ChatHold, error) {
	var h domain.ChatHold
	if err := database.Conn(ctx, r.db).First(&h, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &h, nil
}

// GetHoldForUpdate implements ChatRepository. Lock order: session, hold, wallet.
func (r *GormChatRepository) GetHoldForUpdate(ctx context.Context, id uuid.UUID) (*domain.ChatHold, error) {
	var h domain.ChatHold
	if err := database.Conn(ctx, r.db).Clauses(forUpdate).First(&h, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &h, nil
}

// OpenHoldForSessionForUpdate implements ChatRepository (the unique partial
// index chat_session_holds_one_open_per_session serves it).
func (r *GormChatRepository) OpenHoldForSessionForUpdate(ctx context.Context, sessionID uuid.UUID) (*domain.ChatHold, error) {
	var h domain.ChatHold
	if err := database.Conn(ctx, r.db).Clauses(forUpdate).
		First(&h, "session_id = ? AND status = ?", sessionID, domain.HoldOpen).Error; err != nil {
		return nil, notFound(err)
	}
	return &h, nil
}

// SaveHold implements ChatRepository.
func (r *GormChatRepository) SaveHold(ctx context.Context, h *domain.ChatHold) error {
	h.UpdatedAt = time.Now().UTC()
	return database.Conn(ctx, r.db).Save(h).Error
}

// IncrementHeldMessages implements ChatRepository: a relative update, so
// concurrent writers (serialised by the session lock anyway) never lose a count.
func (r *GormChatRepository) IncrementHeldMessages(ctx context.Context, holdID uuid.UUID) error {
	return database.Conn(ctx, r.db).Model(&domain.ChatHold{}).Where("id = ?", holdID).
		UpdateColumn("held_messages", gorm.Expr("held_messages + 1")).Error
}

// HoldsByIDs implements ChatRepository: one primary-key lookup for a page.
func (r *GormChatRepository) HoldsByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.ChatHold, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []domain.ChatHold
	err := database.Conn(ctx, r.db).Where("id IN ?", ids).Find(&out).Error
	return out, err
}

// HeldSessionIDsForClient implements ChatRepository. A client has at most one
// open session (chat_sessions_one_open_per_client covers the predicate).
func (r *GormChatRepository) HeldSessionIDsForClient(ctx context.Context, clientID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := database.Conn(ctx, r.db).Model(&domain.Session{}).
		Where("client_id = ? AND status = ?", clientID, domain.SessionHeld).Pluck("id", &ids).Error
	return ids, err
}

// ClaimExpiredHolds implements ChatRepository: held readings whose hold ran
// out, locked with SKIP LOCKED so several workers never take the same one
// (chat_sessions_held_idx).
func (r *GormChatRepository) ClaimExpiredHolds(ctx context.Context, now time.Time, limit int) ([]domain.Session, error) {
	var out []domain.Session
	err := database.Conn(ctx, r.db).Clauses(forUpdateSkipLocked).
		Where("status = ? AND hold_expires_at <= ?", domain.SessionHeld, now).
		Order("hold_expires_at ASC").Limit(clampLimit(limit, 50, 500)).Find(&out).Error
	return out, err
}

// holdScanSlack widens the time bound of the queries below. hold_id alone is
// exact; the bound only lets the planner use chat_messages_session_idx
// (session_id, created_at) instead of an index of its own on the big messages
// table, and must never exclude a message of the hold even when the instance
// that stored it has a clock a little behind the one that started the hold.
const holdScanSlack = 10 * time.Minute

// UnlockHoldMessages implements ChatRepository: every message the hold hid
// becomes visible in one UPDATE, bounded to the reading since the hold began
// (chat_messages_session_idx).
func (r *GormChatRepository) UnlockHoldMessages(ctx context.Context, h *domain.ChatHold) (int64, error) {
	res := database.Conn(ctx, r.db).Model(&domain.Message{}).
		Where("session_id = ? AND created_at >= ? AND hold_id = ? AND held", h.SessionID, h.StartedAt.Add(-holdScanSlack), h.ID).
		UpdateColumn("held", false)
	return res.RowsAffected, res.Error
}

// ListHoldMessages implements ChatRepository: the messages written under a
// hold, oldest first (chat_messages_session_idx).
func (r *GormChatRepository) ListHoldMessages(ctx context.Context, h *domain.ChatHold) ([]domain.Message, error) {
	var out []domain.Message
	err := database.Conn(ctx, r.db).
		Where("session_id = ? AND created_at >= ? AND hold_id = ?", h.SessionID, h.StartedAt.Add(-holdScanSlack), h.ID).
		Order("seq ASC").Find(&out).Error
	return out, err
}

// HasPendingPurchaseSince implements ChatRepository: whether the client has a
// credit purchase still processing that was created at or after since
// (credit_purchases_user_idx).
func (r *GormChatRepository) HasPendingPurchaseSince(ctx context.Context, clientID uuid.UUID, since time.Time) (bool, error) {
	var n int64
	err := database.Conn(ctx, r.db).Model(&domain.CreditPurchase{}).
		Where("user_id = ? AND status = ? AND created_at >= ?", clientID, domain.PurchasePending, since).
		Limit(1).Count(&n).Error
	return n > 0, err
}

// HoldStatsSince implements ChatRepository: one aggregate over the window
// (chat_session_holds_started_idx).
func (r *GormChatRepository) HoldStatsSince(ctx context.Context, since time.Time) (HoldStats, error) {
	var s HoldStats
	err := database.Conn(ctx, r.db).Model(&domain.ChatHold{}).Where("started_at >= ?", since).
		Select(`count(*) AS started,
			count(*) FILTER (WHERE status = 'converted') AS converted,
			count(*) FILTER (WHERE status IN ('expired', 'revealed')) AS expired,
			count(*) FILTER (WHERE status = 'revealed') AS revealed,
			COALESCE(sum(converted_credits), 0) AS credits_from_conversion,
			COALESCE(sum(reveal_credits), 0) AS credits_from_reveals`).Scan(&s).Error
	return s, err
}
