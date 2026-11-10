package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// AliasRepository persists advisors' private client nicknames. Every method
// is scoped by the advisor's id: one advisor can never reach another's rows.
type AliasRepository interface {
	// Upsert creates or replaces the advisor's alias for a client.
	Upsert(ctx context.Context, a *domain.ClientAlias) error
	// Delete removes the alias and reports whether one existed.
	Delete(ctx context.Context, psychicID, clientID uuid.UUID) (bool, error)
	// Get returns the advisor's alias for one client (ErrNotFound when none).
	Get(ctx context.Context, psychicID, clientID uuid.UUID) (*domain.ClientAlias, error)
	// List returns the advisor's aliases for the given clients in one query.
	List(ctx context.Context, psychicID uuid.UUID, clientIDs []uuid.UUID) ([]domain.ClientAlias, error)
	// PurgeForUser hard-deletes every alias written by or about a user.
	PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error)
}

// GormAliasRepository implements AliasRepository.
type GormAliasRepository struct{ db *gorm.DB }

// NewAliasRepository returns a GORM alias repository.
func NewAliasRepository(db *gorm.DB) *GormAliasRepository { return &GormAliasRepository{db: db} }

// Upsert implements AliasRepository.
func (r *GormAliasRepository) Upsert(ctx context.Context, a *domain.ClientAlias) error {
	return database.Conn(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "psychic_id"}, {Name: "client_id"}},
		DoUpdates: clause.Assignments(map[string]any{"alias_enc": a.AliasEnc, "updated_at": a.UpdatedAt}),
	}).Create(a).Error
}

// Delete implements AliasRepository.
func (r *GormAliasRepository) Delete(ctx context.Context, psychicID, clientID uuid.UUID) (bool, error) {
	res := database.Conn(ctx, r.db).Where("psychic_id = ? AND client_id = ?", psychicID, clientID).Delete(&domain.ClientAlias{})
	return res.RowsAffected > 0, res.Error
}

// Get implements AliasRepository.
func (r *GormAliasRepository) Get(ctx context.Context, psychicID, clientID uuid.UUID) (*domain.ClientAlias, error) {
	var a domain.ClientAlias
	if err := database.Conn(ctx, r.db).Where("psychic_id = ? AND client_id = ?", psychicID, clientID).First(&a).Error; err != nil {
		return nil, notFound(err)
	}
	return &a, nil
}

// List implements AliasRepository.
func (r *GormAliasRepository) List(ctx context.Context, psychicID uuid.UUID, clientIDs []uuid.UUID) ([]domain.ClientAlias, error) {
	if len(clientIDs) == 0 {
		return nil, nil
	}
	var out []domain.ClientAlias
	err := database.Conn(ctx, r.db).Where("psychic_id = ? AND client_id IN ?", psychicID, clientIDs).Find(&out).Error
	return out, err
}

// PurgeForUser implements AliasRepository.
func (r *GormAliasRepository) PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	res := database.Conn(ctx, r.db).Where("psychic_id = ? OR client_id = ?", userID, userID).Delete(&domain.ClientAlias{})
	return res.RowsAffected, res.Error
}
