package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

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
	return database.Conn(ctx, r.db).CreateInBatches(rows, 500).Error
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

func (r *GormNotificationRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	res := database.Conn(ctx, r.db).Where("created_at < ?", before).Delete(&domain.Notification{})
	return res.RowsAffected, res.Error
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
