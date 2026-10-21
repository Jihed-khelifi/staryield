package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// AdminRepository persists admin MFA enrolments and console sessions.
type AdminRepository interface {
	GetMFA(ctx context.Context, userID uuid.UUID) (*domain.AdminMFA, error)
	GetMFAForUpdate(ctx context.Context, userID uuid.UUID) (*domain.AdminMFA, error)
	SaveMFA(ctx context.Context, m *domain.AdminMFA) error
	DeleteMFA(ctx context.Context, userID uuid.UUID) error

	CreateSession(ctx context.Context, s *domain.AdminSession) error
	GetSessionByHash(ctx context.Context, hash string) (*domain.AdminSession, error)
	TouchSession(ctx context.Context, id uuid.UUID, at time.Time) error
	SetSessionMFA(ctx context.Context, id uuid.UUID, at time.Time) error
	RevokeSession(ctx context.Context, id uuid.UUID, reason string, at time.Time) error
	RevokeUserSessions(ctx context.Context, userID uuid.UUID, reason string, at time.Time) (int64, error)
	ListActiveSessions(ctx context.Context, userID uuid.UUID, now time.Time) ([]domain.AdminSession, error)
	DeleteExpiredSessions(ctx context.Context, before time.Time) (int64, error)
}

// GormAdminRepository implements AdminRepository.
type GormAdminRepository struct{ db *gorm.DB }

// NewAdminRepository returns a GORM admin repository.
func NewAdminRepository(db *gorm.DB) *GormAdminRepository { return &GormAdminRepository{db: db} }

func (r *GormAdminRepository) GetMFA(ctx context.Context, userID uuid.UUID) (*domain.AdminMFA, error) {
	var m domain.AdminMFA
	if err := database.Conn(ctx, r.db).First(&m, "user_id = ?", userID).Error; err != nil {
		return nil, notFound(err)
	}
	return &m, nil
}

func (r *GormAdminRepository) GetMFAForUpdate(ctx context.Context, userID uuid.UUID) (*domain.AdminMFA, error) {
	var m domain.AdminMFA
	if err := database.Conn(ctx, r.db).Clauses(forUpdate).First(&m, "user_id = ?", userID).Error; err != nil {
		return nil, notFound(err)
	}
	return &m, nil
}

// SaveMFA upserts an enrolment.
func (r *GormAdminRepository) SaveMFA(ctx context.Context, m *domain.AdminMFA) error {
	m.UpdatedAt = time.Now().UTC()
	return database.Conn(ctx, r.db).Save(m).Error
}

func (r *GormAdminRepository) DeleteMFA(ctx context.Context, userID uuid.UUID) error {
	return database.Conn(ctx, r.db).Where("user_id = ?", userID).Delete(&domain.AdminMFA{}).Error
}

func (r *GormAdminRepository) CreateSession(ctx context.Context, s *domain.AdminSession) error {
	return database.Conn(ctx, r.db).Create(s).Error
}

func (r *GormAdminRepository) GetSessionByHash(ctx context.Context, hash string) (*domain.AdminSession, error) {
	var s domain.AdminSession
	if err := database.Conn(ctx, r.db).First(&s, "token_hash = ?", hash).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

func (r *GormAdminRepository) TouchSession(ctx context.Context, id uuid.UUID, at time.Time) error {
	return database.Conn(ctx, r.db).Model(&domain.AdminSession{}).Where("id = ? AND revoked_at IS NULL", id).
		Update("last_seen_at", at).Error
}

func (r *GormAdminRepository) SetSessionMFA(ctx context.Context, id uuid.UUID, at time.Time) error {
	return database.Conn(ctx, r.db).Model(&domain.AdminSession{}).Where("id = ? AND revoked_at IS NULL", id).
		Updates(map[string]any{"mfa_at": at, "last_seen_at": at}).Error
}

func (r *GormAdminRepository) RevokeSession(ctx context.Context, id uuid.UUID, reason string, at time.Time) error {
	return database.Conn(ctx, r.db).Model(&domain.AdminSession{}).Where("id = ? AND revoked_at IS NULL", id).
		Updates(map[string]any{"revoked_at": at, "revoke_reason": reason}).Error
}

func (r *GormAdminRepository) RevokeUserSessions(ctx context.Context, userID uuid.UUID, reason string, at time.Time) (int64, error) {
	res := database.Conn(ctx, r.db).Model(&domain.AdminSession{}).Where("user_id = ? AND revoked_at IS NULL", userID).
		Updates(map[string]any{"revoked_at": at, "revoke_reason": reason})
	return res.RowsAffected, res.Error
}

func (r *GormAdminRepository) ListActiveSessions(ctx context.Context, userID uuid.UUID, now time.Time) ([]domain.AdminSession, error) {
	var out []domain.AdminSession
	err := database.Conn(ctx, r.db).Where("user_id = ? AND revoked_at IS NULL AND expires_at > ?", userID, now).
		Order("last_seen_at DESC").Limit(50).Find(&out).Error
	return out, err
}

func (r *GormAdminRepository) DeleteExpiredSessions(ctx context.Context, before time.Time) (int64, error) {
	res := database.Conn(ctx, r.db).Where("expires_at < ? OR (revoked_at IS NOT NULL AND revoked_at < ?)", before, before).
		Delete(&domain.AdminSession{})
	return res.RowsAffected, res.Error
}
