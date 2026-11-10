package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm/clause"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// forNoKeyUpdate locks a row against concurrent writers of the same row
// without blocking inserts that only reference it (replies, likes).
var forNoKeyUpdate = clause.Locking{Strength: "NO KEY UPDATE"}

// ReplyTargetRow is the quoted message of a reply, as stored.
type ReplyTargetRow struct {
	ID       uuid.UUID
	Seq      int64
	SenderID *uuid.UUID
	Kind     string
	Body     string
}

// LikeRow is one like in a batched lookup.
type LikeRow struct {
	MessageID uuid.UUID
	UserID    uuid.UUID
}

// ReplyTargets implements ChatRepository: one primary-key lookup for a whole
// page, never one query per message.
func (r *GormChatRepository) ReplyTargets(ctx context.Context, ids []uuid.UUID) ([]ReplyTargetRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []ReplyTargetRow
	err := database.Conn(ctx, r.db).Table("chat_messages").Select("id, seq, sender_id, kind, body").Where("id IN ?", ids).Scan(&out).Error
	return out, err
}

// LikesFor implements ChatRepository: the likes of a page of messages in one
// query on the primary key (message_id, user_id), oldest like first.
func (r *GormChatRepository) LikesFor(ctx context.Context, messageIDs []uuid.UUID) ([]LikeRow, error) {
	if len(messageIDs) == 0 {
		return nil, nil
	}
	var out []LikeRow
	err := database.Conn(ctx, r.db).Model(&domain.MessageLike{}).Select("message_id, user_id").
		Where("message_id IN ?", messageIDs).Order("created_at, user_id").Scan(&out).Error
	return out, err
}

// LockMessage implements ChatRepository (inside a transaction): serialises
// the likes of one message so the liked_by each event carries is the state
// after the previous like.
func (r *GormChatRepository) LockMessage(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
	var m domain.Message
	if err := database.Conn(ctx, r.db).Clauses(forNoKeyUpdate).First(&m, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &m, nil
}

// LikeMessage implements ChatRepository. A concurrent or repeated like is a
// no-op (ON CONFLICT DO NOTHING) and reports false.
func (r *GormChatRepository) LikeMessage(ctx context.Context, messageID, userID uuid.UUID, at time.Time) (bool, error) {
	res := database.Conn(ctx, r.db).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&domain.MessageLike{MessageID: messageID, UserID: userID, CreatedAt: at})
	return res.RowsAffected == 1, res.Error
}

// UnlikeMessage implements ChatRepository; it reports whether a like existed.
func (r *GormChatRepository) UnlikeMessage(ctx context.Context, messageID, userID uuid.UUID) (bool, error) {
	res := database.Conn(ctx, r.db).Where("message_id = ? AND user_id = ?", messageID, userID).Delete(&domain.MessageLike{})
	return res.RowsAffected > 0, res.Error
}

// PurgeLikesForUser implements ChatRepository (account erasure: the user's
// likes are removed, the messages stay).
func (r *GormChatRepository) PurgeLikesForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	res := database.Conn(ctx, r.db).Where("user_id = ?", userID).Delete(&domain.MessageLike{})
	return res.RowsAffected, res.Error
}
