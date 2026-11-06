package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// PushRepository persists Web Push subscriptions.
type PushRepository interface {
	// Upsert registers a subscription keyed by its endpoint hash: an existing
	// row (even unsubscribed, even another user's) is re-assigned and revived.
	Upsert(ctx context.Context, s *domain.PushSubscription) error
	// Active returns the live subscription with this endpoint hash, if any.
	Active(ctx context.Context, endpointHash string) (*domain.PushSubscription, error)
	CountActive(ctx context.Context, userID uuid.UUID) (int64, error)
	// DeleteForUser soft-deletes the user's subscription with this hash.
	DeleteForUser(ctx context.Context, userID uuid.UUID, endpointHash string) (int64, error)
	// DeleteAllForUser soft-deletes every subscription of a user.
	DeleteAllForUser(ctx context.Context, userID uuid.UUID) (int64, error)
	ListActive(ctx context.Context, userIDs []uuid.UUID) ([]domain.PushSubscription, error)
	MarkSuccess(ctx context.Context, id uuid.UUID, at time.Time) error
	// MarkFailure increments the failure counter and returns its new value.
	MarkFailure(ctx context.Context, id uuid.UUID) (int, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// GormPushRepository implements PushRepository.
type GormPushRepository struct{ db *gorm.DB }

// NewPushRepository returns a GORM push repository.
func NewPushRepository(db *gorm.DB) *GormPushRepository { return &GormPushRepository{db: db} }

func (r *GormPushRepository) Upsert(ctx context.Context, s *domain.PushSubscription) error {
	return database.Conn(ctx, r.db).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "endpoint_hash"}},
		DoUpdates: clause.Assignments(map[string]any{
			"user_id":         gorm.Expr("excluded.user_id"),
			"endpoint_sealed": gorm.Expr("excluded.endpoint_sealed"),
			"p256dh_sealed":   gorm.Expr("excluded.p256dh_sealed"),
			"auth_sealed":     gorm.Expr("excluded.auth_sealed"),
			"user_agent":      gorm.Expr("excluded.user_agent"),
			"failure_count":   0,
			"deleted_at":      nil,
			"updated_at":      gorm.Expr("excluded.updated_at"),
		}),
	}).Create(s).Error
}

func (r *GormPushRepository) Active(ctx context.Context, endpointHash string) (*domain.PushSubscription, error) {
	var s domain.PushSubscription
	if err := database.Conn(ctx, r.db).First(&s, "endpoint_hash = ?", endpointHash).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

func (r *GormPushRepository) CountActive(ctx context.Context, userID uuid.UUID) (int64, error) {
	var n int64
	err := database.Conn(ctx, r.db).Model(&domain.PushSubscription{}).Where("user_id = ?", userID).Count(&n).Error
	return n, err
}

func (r *GormPushRepository) DeleteForUser(ctx context.Context, userID uuid.UUID, endpointHash string) (int64, error) {
	res := database.Conn(ctx, r.db).Where("user_id = ? AND endpoint_hash = ?", userID, endpointHash).Delete(&domain.PushSubscription{})
	return res.RowsAffected, res.Error
}

func (r *GormPushRepository) DeleteAllForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	res := database.Conn(ctx, r.db).Where("user_id = ?", userID).Delete(&domain.PushSubscription{})
	return res.RowsAffected, res.Error
}

func (r *GormPushRepository) ListActive(ctx context.Context, userIDs []uuid.UUID) ([]domain.PushSubscription, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var out []domain.PushSubscription
	err := database.Conn(ctx, r.db).Where("user_id IN ?", userIDs).Order("created_at DESC").Limit(1000).Find(&out).Error
	return out, err
}

func (r *GormPushRepository) MarkSuccess(ctx context.Context, id uuid.UUID, at time.Time) error {
	return database.Conn(ctx, r.db).Model(&domain.PushSubscription{}).Where("id = ?", id).
		UpdateColumns(map[string]any{"last_success_at": at, "failure_count": 0, "updated_at": at}).Error
}

func (r *GormPushRepository) MarkFailure(ctx context.Context, id uuid.UUID) (int, error) {
	var s domain.PushSubscription
	err := database.Conn(ctx, r.db).Model(&s).Clauses(clause.Returning{Columns: []clause.Column{{Name: "failure_count"}}}).
		Where("id = ?", id).UpdateColumn("failure_count", gorm.Expr("failure_count + 1")).Error
	return s.FailureCount, err
}

func (r *GormPushRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return database.Conn(ctx, r.db).Where("id = ?", id).Delete(&domain.PushSubscription{}).Error
}
