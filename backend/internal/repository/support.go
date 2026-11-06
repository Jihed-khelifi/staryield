package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// ErrDuplicateReference reports a support reference that is already taken
// (the service retries with a fresh one).
var ErrDuplicateReference = errors.New("repository: support reference already used")

// SupportRepository persists contact-form tickets.
type SupportRepository interface {
	Create(ctx context.Context, t *domain.SupportTicket) error
	// PurgeForUser hard-deletes the tickets linked to a user (erasure).
	PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error)
}

// GormSupportRepository implements SupportRepository.
type GormSupportRepository struct{ db *gorm.DB }

// NewSupportRepository returns a GORM support repository.
func NewSupportRepository(db *gorm.DB) *GormSupportRepository { return &GormSupportRepository{db: db} }

// Create inserts a ticket.
func (r *GormSupportRepository) Create(ctx context.Context, t *domain.SupportTicket) error {
	err := database.Conn(ctx, r.db).Create(t).Error
	if isUnique(err, "support_tickets_reference_key") {
		return ErrDuplicateReference
	}
	return err
}

// PurgeForUser implements SupportRepository.
func (r *GormSupportRepository) PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	res := database.Conn(ctx, r.db).Unscoped().Where("user_id = ?", userID).Delete(&domain.SupportTicket{})
	return res.RowsAffected, res.Error
}
