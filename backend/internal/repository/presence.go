package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// PresenceRepository records advisors' availability history.
type PresenceRepository interface {
	// OpenForUpdate returns the advisor's open interval, locked, or nil.
	OpenForUpdate(ctx context.Context, psychicID uuid.UUID) (*domain.PresenceInterval, error)
	// OpenIntervals lists every open interval (for reconciliation).
	OpenIntervals(ctx context.Context, limit int) ([]domain.PresenceInterval, error)
	Start(ctx context.Context, iv *domain.PresenceInterval) error
	Close(ctx context.Context, id uuid.UUID, at time.Time) error
	// TouchLastOnline moves psychics.last_online_at forward to at.
	TouchLastOnline(ctx context.Context, psychicID uuid.UUID, at time.Time) error
	// AvailabilityOf returns the chosen availability of the given advisors
	// (missing IDs are absent from the map).
	AvailabilityOf(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.Availability, error)
}

// GormPresenceRepository implements PresenceRepository.
type GormPresenceRepository struct{ db *gorm.DB }

// NewPresenceRepository returns a presence history repository.
func NewPresenceRepository(db *gorm.DB) *GormPresenceRepository {
	return &GormPresenceRepository{db: db}
}

// OpenForUpdate implements PresenceRepository.
func (r *GormPresenceRepository) OpenForUpdate(ctx context.Context, psychicID uuid.UUID) (*domain.PresenceInterval, error) {
	var out []domain.PresenceInterval
	err := database.Conn(ctx, r.db).Clauses(forUpdate).
		Where("psychic_id = ? AND ended_at IS NULL", psychicID).Limit(1).Find(&out).Error
	if err != nil || len(out) == 0 {
		return nil, err
	}
	return &out[0], nil
}

// OpenIntervals implements PresenceRepository.
func (r *GormPresenceRepository) OpenIntervals(ctx context.Context, limit int) ([]domain.PresenceInterval, error) {
	var out []domain.PresenceInterval
	err := database.Conn(ctx, r.db).Where("ended_at IS NULL").Order("started_at").Limit(clampLimit(limit, 1000, 10000)).Find(&out).Error
	return out, err
}

// Start implements PresenceRepository.
func (r *GormPresenceRepository) Start(ctx context.Context, iv *domain.PresenceInterval) error {
	return database.Conn(ctx, r.db).Create(iv).Error
}

// Close implements PresenceRepository.
func (r *GormPresenceRepository) Close(ctx context.Context, id uuid.UUID, at time.Time) error {
	return database.Conn(ctx, r.db).Model(&domain.PresenceInterval{}).Where("id = ? AND ended_at IS NULL", id).
		Updates(map[string]any{"ended_at": gorm.Expr("GREATEST(started_at, ?)", at), "updated_at": at}).Error
}

// TouchLastOnline implements PresenceRepository.
func (r *GormPresenceRepository) TouchLastOnline(ctx context.Context, psychicID uuid.UUID, at time.Time) error {
	return database.Conn(ctx, r.db).Model(&domain.Psychic{}).Where("user_id = ?", psychicID).
		UpdateColumn("last_online_at", gorm.Expr("GREATEST(COALESCE(last_online_at, ?), ?)", at, at)).Error
}

// AvailabilityOf implements PresenceRepository.
func (r *GormPresenceRepository) AvailabilityOf(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.Availability, error) {
	out := make(map[uuid.UUID]domain.Availability, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		UserID       uuid.UUID
		Availability domain.Availability
	}
	if err := database.Conn(ctx, r.db).Model(&domain.Psychic{}).Select("user_id, availability").
		Where("user_id IN ?", ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.UserID] = row.Availability
	}
	return out, nil
}
