package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// ErrIdentityTaken is returned when the provider subject, or the account's
// identity for that provider, is already linked.
var ErrIdentityTaken = errors.New("repository: identity already linked")

// IdentityRepository persists external sign-in identities.
type IdentityRepository interface {
	// Create links an identity. ErrIdentityTaken on a unique violation of
	// (provider, subject) or (user_id, provider).
	Create(ctx context.Context, i *domain.UserIdentity) error
	// GetBySubject finds the identity of a provider subject.
	GetBySubject(ctx context.Context, provider, subject string) (*domain.UserIdentity, error)
	// GetForUser finds a user's identity for a provider.
	GetForUser(ctx context.Context, userID uuid.UUID, provider string) (*domain.UserIdentity, error)
	// ListForUser returns every identity of a user, oldest first.
	ListForUser(ctx context.Context, userID uuid.UUID) ([]domain.UserIdentity, error)
	// Touch records a sign-in (and the e-mail the provider reported).
	Touch(ctx context.Context, id uuid.UUID, email string, at time.Time) error
	// Delete unlinks a user's identity for a provider; ErrNotFound when none.
	Delete(ctx context.Context, userID uuid.UUID, provider string) error
	// PurgeForUser removes every identity of a user (account erasure).
	PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error)
}

// GormIdentityRepository implements IdentityRepository.
type GormIdentityRepository struct{ db *gorm.DB }

// NewIdentityRepository returns an identity repository.
func NewIdentityRepository(db *gorm.DB) *GormIdentityRepository {
	return &GormIdentityRepository{db: db}
}

// Create implements IdentityRepository.
func (r *GormIdentityRepository) Create(ctx context.Context, i *domain.UserIdentity) error {
	if err := database.Conn(ctx, r.db).Create(i).Error; err != nil {
		if isUnique(err, "user_identities_provider_subject_key", "user_identities_user_provider_key") {
			return ErrIdentityTaken
		}
		return err
	}
	return nil
}

// GetBySubject implements IdentityRepository.
func (r *GormIdentityRepository) GetBySubject(ctx context.Context, provider, subject string) (*domain.UserIdentity, error) {
	var i domain.UserIdentity
	if err := database.Conn(ctx, r.db).First(&i, "provider = ? AND subject = ?", provider, subject).Error; err != nil {
		return nil, notFound(err)
	}
	return &i, nil
}

// GetForUser implements IdentityRepository.
func (r *GormIdentityRepository) GetForUser(ctx context.Context, userID uuid.UUID, provider string) (*domain.UserIdentity, error) {
	var i domain.UserIdentity
	if err := database.Conn(ctx, r.db).First(&i, "user_id = ? AND provider = ?", userID, provider).Error; err != nil {
		return nil, notFound(err)
	}
	return &i, nil
}

// ListForUser implements IdentityRepository.
func (r *GormIdentityRepository) ListForUser(ctx context.Context, userID uuid.UUID) ([]domain.UserIdentity, error) {
	var out []domain.UserIdentity
	err := database.Conn(ctx, r.db).Where("user_id = ?", userID).Order("created_at, id").Find(&out).Error
	return out, err
}

// Touch implements IdentityRepository.
func (r *GormIdentityRepository) Touch(ctx context.Context, id uuid.UUID, email string, at time.Time) error {
	return database.Conn(ctx, r.db).Model(&domain.UserIdentity{}).Where("id = ?", id).
		Updates(map[string]any{"last_login_at": at, "email": email, "updated_at": at}).Error
}

// Delete implements IdentityRepository.
func (r *GormIdentityRepository) Delete(ctx context.Context, userID uuid.UUID, provider string) error {
	res := database.Conn(ctx, r.db).Where("user_id = ? AND provider = ?", userID, provider).Delete(&domain.UserIdentity{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// PurgeForUser implements IdentityRepository (and service.UserDataPurger).
func (r *GormIdentityRepository) PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	res := database.Conn(ctx, r.db).Where("user_id = ?", userID).Delete(&domain.UserIdentity{})
	return res.RowsAffected, res.Error
}
