package repository

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// NotificationRepository persists in-app notifications and broadcasts.
type NotificationRepository interface {
	InsertBatch(ctx context.Context, rows []domain.Notification) error
	List(ctx context.Context, userID uuid.UUID, cursor *Cursor, limit int) ([]domain.Notification, error)
	CountUnread(ctx context.Context, userID uuid.UUID) (int64, error)
	MarkRead(ctx context.Context, userID uuid.UUID, ids []uuid.UUID, at time.Time) error
	MarkAllRead(ctx context.Context, userID uuid.UUID, at time.Time) error
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)

	CreateBroadcast(ctx context.Context, b *domain.Broadcast) error
	UpdateBroadcast(ctx context.Context, id uuid.UUID, fields map[string]any) error
	GetBroadcast(ctx context.Context, id uuid.UUID) (*domain.Broadcast, error)
	ListBroadcasts(ctx context.Context, cursor *Cursor, limit int) ([]domain.Broadcast, error)
	// AudienceIDs resolves broadcast recipients (active accounts only).
	AudienceIDs(ctx context.Context, roles []domain.Role, ids []uuid.UUID, campaignID *uuid.UUID, limit int) ([]uuid.UUID, error)
	UserIDsByEmails(ctx context.Context, emails []string) ([]uuid.UUID, error)

	// AudienceChunk resolves one chunk of broadcast recipients in user_id order
	// (active accounts of roles, optionally attributed to a campaign). With
	// candidates set only those ids are considered (no cursor needed: the caller
	// walks the candidate list); otherwise the users table is scanned for ids
	// after the cursor.
	AudienceChunk(ctx context.Context, roles []domain.Role, candidates []uuid.UUID, after *uuid.UUID, campaignID *uuid.UUID, limit int) ([]uuid.UUID, error)
	// InsertBroadcastChunk stores a chunk of a broadcast idempotently (a
	// recipient that already has the notification is skipped) and, in the same
	// transaction, records the fan-out position, the heartbeat and the number
	// of new recipients. It returns the ids of the rows actually inserted.
	InsertBroadcastChunk(ctx context.Context, broadcastID uuid.UUID, rows []domain.Notification, cursor uuid.UUID, now time.Time) ([]uuid.UUID, error)
	// BroadcastHeartbeat refreshes the heartbeat of a running broadcast.
	BroadcastHeartbeat(ctx context.Context, id uuid.UUID, now time.Time) error
	// CountBroadcastRecipients counts the notifications of a broadcast.
	CountBroadcastRecipients(ctx context.Context, id uuid.UUID) (int, error)
	// StaleBroadcasts lists queued or sending broadcasts whose heartbeat (or
	// creation, when it never beat) is older than before.
	StaleBroadcasts(ctx context.Context, before time.Time, limit int) ([]domain.Broadcast, error)
	// WithBroadcastLock runs fn while holding the advisory lock of a
	// broadcast; acquired is false (fn not run) when another runner holds it.
	WithBroadcastLock(ctx context.Context, id uuid.UUID, fn func(ctx context.Context) error) (acquired bool, err error)
}

// GormNotificationRepository implements NotificationRepository.
type GormNotificationRepository struct{ db *gorm.DB }

// NewNotificationRepository returns a GORM notification repository.
func NewNotificationRepository(db *gorm.DB) *GormNotificationRepository {
	return &GormNotificationRepository{db: db}
}

func (r *GormNotificationRepository) InsertBatch(ctx context.Context, rows []domain.Notification) error {
	if len(rows) == 0 {
		return nil
	}
	// A conflict (a retried batch, a broadcast recipient already notified) is
	// skipped, so retrying a partly written batch never fails on the rows that
	// made it.
	return database.Conn(ctx, r.db).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 500).Error
}

func (r *GormNotificationRepository) List(ctx context.Context, userID uuid.UUID, cursor *Cursor, limit int) ([]domain.Notification, error) {
	q := database.Conn(ctx, r.db).Where("user_id = ?", userID)
	q = applyCursor(q, cursor, "created_at", "id")
	var out []domain.Notification
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(limit, 20, 100)).Find(&out).Error
	return out, err
}

func (r *GormNotificationRepository) CountUnread(ctx context.Context, userID uuid.UUID) (int64, error) {
	var n int64
	err := database.Conn(ctx, r.db).Model(&domain.Notification{}).Where("user_id = ? AND read_at IS NULL", userID).Count(&n).Error
	return n, err
}

func (r *GormNotificationRepository) MarkRead(ctx context.Context, userID uuid.UUID, ids []uuid.UUID, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return database.Conn(ctx, r.db).Model(&domain.Notification{}).
		Where("user_id = ? AND id IN ? AND read_at IS NULL", userID, ids).Update("read_at", at).Error
}

func (r *GormNotificationRepository) MarkAllRead(ctx context.Context, userID uuid.UUID, at time.Time) error {
	return database.Conn(ctx, r.db).Model(&domain.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).Update("read_at", at).Error
}

// Retention deletes in batches (RetentionBatch rows, ordered by age through
// notifications_created_idx) with a short pause between them, so the cleanup
// never holds long locks or floods replication; rows another instance is
// already deleting are skipped.
const RetentionBatch = 5000

// RetentionPause is the pause between two retention batches.
var RetentionPause = 50 * time.Millisecond

// retentionSelect picks one retention batch through notifications_created_idx.
const retentionSelect = `SELECT id FROM notifications WHERE created_at < ? ORDER BY created_at LIMIT ? FOR UPDATE SKIP LOCKED`

// ExpiredBatch returns the ids the next retention batch would delete (the
// selection DeleteOlderThan runs; load tests EXPLAIN it without deleting).
func (r *GormNotificationRepository) ExpiredBatch(ctx context.Context, before time.Time, limit int) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := database.Conn(ctx, r.db).Raw(retentionSelect, before, limit).Scan(&ids).Error
	return ids, err
}

// DeleteOlderThan implements NotificationRepository.
func (r *GormNotificationRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	var total int64
	for {
		res := database.Conn(ctx, r.db).Exec(`DELETE FROM notifications WHERE id IN (`+retentionSelect+`)`, before, RetentionBatch)
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
		if res.RowsAffected < RetentionBatch {
			return total, nil
		}
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		case <-time.After(RetentionPause):
		}
	}
}

func (r *GormNotificationRepository) CreateBroadcast(ctx context.Context, b *domain.Broadcast) error {
	return database.Conn(ctx, r.db).Create(b).Error
}

func (r *GormNotificationRepository) UpdateBroadcast(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	return database.Conn(ctx, r.db).Model(&domain.Broadcast{}).Where("id = ?", id).Updates(fields).Error
}

func (r *GormNotificationRepository) GetBroadcast(ctx context.Context, id uuid.UUID) (*domain.Broadcast, error) {
	var b domain.Broadcast
	if err := database.Conn(ctx, r.db).First(&b, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &b, nil
}

func (r *GormNotificationRepository) ListBroadcasts(ctx context.Context, cursor *Cursor, limit int) ([]domain.Broadcast, error) {
	q := applyCursor(database.Conn(ctx, r.db).Model(&domain.Broadcast{}), cursor, "created_at", "id")
	var out []domain.Broadcast
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(limit, 20, 100)).Find(&out).Error
	return out, err
}

func (r *GormNotificationRepository) AudienceIDs(ctx context.Context, roles []domain.Role, ids []uuid.UUID, campaignID *uuid.UUID, limit int) ([]uuid.UUID, error) {
	q := database.Conn(ctx, r.db).Table("users AS u").Select("u.id").Where("u.status = ?", domain.UserActive)
	if len(roles) > 0 {
		q = q.Where("u.role IN ?", roles)
	}
	if ids != nil {
		if len(ids) == 0 {
			return nil, nil
		}
		q = q.Where("u.id IN ?", ids)
	}
	if campaignID != nil {
		q = q.Where("EXISTS (SELECT 1 FROM user_attributions a WHERE a.user_id = u.id AND a.campaign_id = ?)", *campaignID)
	}
	var out []uuid.UUID
	err := q.Order("u.created_at").Limit(limit).Pluck("u.id", &out).Error
	return out, err
}

func (r *GormNotificationRepository) UserIDsByEmails(ctx context.Context, emails []string) ([]uuid.UUID, error) {
	if len(emails) == 0 {
		return nil, nil
	}
	var out []uuid.UUID
	err := database.Conn(ctx, r.db).Model(&domain.User{}).Where("email IN ?", emails).Pluck("id", &out).Error
	return out, err
}

// AudienceChunk implements NotificationRepository.
func (r *GormNotificationRepository) AudienceChunk(ctx context.Context, roles []domain.Role, candidates []uuid.UUID, after *uuid.UUID,
	campaignID *uuid.UUID, limit int) ([]uuid.UUID, error) {
	q := database.Conn(ctx, r.db).Table("users AS u").Select("u.id").Where("u.status = ?", domain.UserActive)
	if len(roles) > 0 {
		q = q.Where("u.role IN ?", roles)
	}
	if candidates != nil {
		if len(candidates) == 0 {
			return nil, nil
		}
		q = q.Where("u.id IN ?", candidates)
	} else if after != nil {
		q = q.Where("u.id > ?", *after)
	}
	if campaignID != nil {
		q = q.Where("EXISTS (SELECT 1 FROM user_attributions a WHERE a.user_id = u.id AND a.campaign_id = ?)", *campaignID)
	}
	var out []uuid.UUID
	err := q.Order("u.id").Limit(limit).Pluck("u.id", &out).Error
	return out, err
}

// InsertBroadcastChunk implements NotificationRepository.
func (r *GormNotificationRepository) InsertBroadcastChunk(ctx context.Context, broadcastID uuid.UUID, rows []domain.Notification,
	cursor uuid.UUID, now time.Time) ([]uuid.UUID, error) {
	var inserted []uuid.UUID
	err := database.Conn(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		inserted = inserted[:0]
		for start := 0; start < len(rows); start += 1000 {
			part := rows[start:min(start+1000, len(rows))]
			args := make([]any, 0, len(part)*8)
			var sb strings.Builder
			sb.WriteString("INSERT INTO notifications (id, user_id, kind, data, link, broadcast_id, created_at, updated_at) VALUES ")
			for i := range part {
				if i > 0 {
					sb.WriteByte(',')
				}
				sb.WriteString("(?, ?, ?, ?::jsonb, ?, ?, ?, ?)")
				data, _ := json.Marshal(part[i].Data)
				args = append(args, part[i].ID, part[i].UserID, part[i].Kind, string(data), part[i].Link, broadcastID, part[i].CreatedAt, part[i].CreatedAt)
			}
			sb.WriteString(" ON CONFLICT (broadcast_id, user_id) WHERE broadcast_id IS NOT NULL DO NOTHING RETURNING id")
			rs, err := tx.Raw(sb.String(), args...).Rows()
			if err != nil {
				return err
			}
			for rs.Next() {
				var id uuid.UUID
				if err := rs.Scan(&id); err != nil {
					_ = rs.Close()
					return err
				}
				inserted = append(inserted, id)
			}
			if err := rs.Err(); err != nil {
				_ = rs.Close()
				return err
			}
			_ = rs.Close()
		}
		return tx.Exec(`UPDATE broadcasts SET cursor_user_id = ?, heartbeat_at = ?, status = 'sending', recipients = recipients + ? WHERE id = ?`,
			cursor, now, len(inserted), broadcastID).Error
	})
	if err != nil {
		return nil, err
	}
	return inserted, nil
}

// BroadcastHeartbeat implements NotificationRepository.
func (r *GormNotificationRepository) BroadcastHeartbeat(ctx context.Context, id uuid.UUID, now time.Time) error {
	return database.Conn(ctx, r.db).Exec(`UPDATE broadcasts SET heartbeat_at = ? WHERE id = ?`, now, id).Error
}

// CountBroadcastRecipients implements NotificationRepository.
func (r *GormNotificationRepository) CountBroadcastRecipients(ctx context.Context, id uuid.UUID) (int, error) {
	var n int64
	err := database.Conn(ctx, r.db).Model(&domain.Notification{}).Where("broadcast_id = ?", id).Count(&n).Error
	return int(n), err
}

// StaleBroadcasts implements NotificationRepository.
func (r *GormNotificationRepository) StaleBroadcasts(ctx context.Context, before time.Time, limit int) ([]domain.Broadcast, error) {
	var out []domain.Broadcast
	err := database.Conn(ctx, r.db).Where("status IN ('queued', 'sending') AND COALESCE(heartbeat_at, created_at) < ?", before).
		Order("created_at").Limit(limit).Find(&out).Error
	return out, err
}

// broadcastLockKey maps a broadcast to its advisory-lock key.
func broadcastLockKey(id uuid.UUID) int64 {
	return int64(binary.BigEndian.Uint64(id[8:])) ^ 0x4252_4f41_4443_4153 // "BROADCAS"
}

// WithBroadcastLock implements NotificationRepository. The session-level
// advisory lock lives on one pinned connection for the whole run; fn itself
// uses the pool.
func (r *GormNotificationRepository) WithBroadcastLock(ctx context.Context, id uuid.UUID, fn func(ctx context.Context) error) (bool, error) {
	acquired := false
	err := r.db.WithContext(ctx).Connection(func(conn *gorm.DB) error {
		var ok bool
		if err := conn.Raw("SELECT pg_try_advisory_lock(?)", broadcastLockKey(id)).Scan(&ok).Error; err != nil {
			return err
		}
		if !ok {
			return nil
		}
		acquired = true
		defer func() {
			// Unlock with a fresh context: the run's may be cancelled.
			_ = conn.WithContext(context.WithoutCancel(ctx)).Exec("SELECT pg_advisory_unlock(?)", broadcastLockKey(id)).Error
		}()
		return fn(ctx)
	})
	return acquired, err
}
