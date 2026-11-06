package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// Contact is what notification channels need to know about a recipient.
type Contact struct {
	UserID          uuid.UUID
	Email           string
	Locale          string
	Status          domain.UserStatus
	NotifyEmail     bool
	NotifyPush      bool
	NotifyMarketing bool
}

// ContactRepository resolves notification recipients.
type ContactRepository interface {
	Contacts(ctx context.Context, ids []uuid.UUID) ([]Contact, error)
}

// GormContactRepository implements ContactRepository.
type GormContactRepository struct{ db *gorm.DB }

// NewContactRepository returns a GORM contact repository.
func NewContactRepository(db *gorm.DB) *GormContactRepository { return &GormContactRepository{db: db} }

// Contacts returns the address, language, status and channel preferences of
// each user (a missing profile means the defaults: e-mail on, push and
// marketing off).
func (r *GormContactRepository) Contacts(ctx context.Context, ids []uuid.UUID) ([]Contact, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []Contact
	err := database.Conn(ctx, r.db).Model(&domain.User{}).
		Select("users.id AS user_id, users.email, users.locale, users.status, "+
			"COALESCE(p.notify_email, true) AS notify_email, COALESCE(p.notify_push, false) AS notify_push, "+
			"COALESCE(p.notify_marketing, false) AS notify_marketing").
		Joins("LEFT JOIN user_profiles p ON p.user_id = users.id").
		Where("users.id IN ?", ids).Scan(&out).Error
	return out, err
}
