package repository

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// UserFilter narrows admin user listings.
type UserFilter struct {
	Query  string
	Role   domain.Role
	Status domain.UserStatus
	Cursor *Cursor
	Limit  int
}

// UserRepository persists users and their profiles.
type UserRepository interface {
	Create(ctx context.Context, u *domain.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.User, error)
	Updates(ctx context.Context, id uuid.UUID, fields map[string]any) error
	List(ctx context.Context, f UserFilter) ([]domain.User, error)
	GetByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.User, error)
	GetProfiles(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.Profile, error)

	CreateProfile(ctx context.Context, p *domain.Profile) error
	GetProfile(ctx context.Context, userID uuid.UUID) (*domain.Profile, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, fields map[string]any) error
}

// GormUserRepository implements UserRepository.
type GormUserRepository struct{ db *gorm.DB }

// NewUserRepository returns a GORM user repository.
func NewUserRepository(db *gorm.DB) *GormUserRepository { return &GormUserRepository{db: db} }

func (r *GormUserRepository) Create(ctx context.Context, u *domain.User) error {
	if err := database.Conn(ctx, r.db).Create(u).Error; err != nil {
		if isUnique(err, "users_email_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *GormUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var u domain.User
	if err := database.Conn(ctx, r.db).First(&u, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &u, nil
}

func (r *GormUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	if err := database.Conn(ctx, r.db).First(&u, "email = ?", strings.ToLower(strings.TrimSpace(email))).Error; err != nil {
		return nil, notFound(err)
	}
	return &u, nil
}

func (r *GormUserRepository) GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var u domain.User
	if err := database.Conn(ctx, r.db).Clauses(forUpdate).First(&u, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &u, nil
}

func (r *GormUserRepository) Updates(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	if _, ok := fields["updated_at"]; !ok {
		fields["updated_at"] = time.Now().UTC()
	}
	res := database.Conn(ctx, r.db).Model(&domain.User{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormUserRepository) List(ctx context.Context, f UserFilter) ([]domain.User, error) {
	q := database.Conn(ctx, r.db).Model(&domain.User{})
	if f.Query != "" {
		like := "%" + escapeLike(strings.ToLower(f.Query)) + "%"
		q = q.Where("(email LIKE ? ESCAPE '\\' OR lower(display_name) LIKE ? ESCAPE '\\')", like, like)
	}
	if f.Role != "" {
		q = q.Where("role = ?", f.Role)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	q = applyCursor(q, f.Cursor, "created_at", "id")
	var out []domain.User
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(f.Limit, 50, 200)).Find(&out).Error
	return out, err
}

func (r *GormUserRepository) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []domain.User
	err := database.Conn(ctx, r.db).Where("id IN ?", ids).Find(&out).Error
	return out, err
}

func (r *GormUserRepository) GetProfiles(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.Profile, error) {
	out := map[uuid.UUID]domain.Profile{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []domain.Profile
	if err := database.Conn(ctx, r.db).Where("user_id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, p := range rows {
		out[p.UserID] = p
	}
	return out, nil
}

func (r *GormUserRepository) CreateProfile(ctx context.Context, p *domain.Profile) error {
	return database.Conn(ctx, r.db).Create(p).Error
}

func (r *GormUserRepository) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.Profile, error) {
	var p domain.Profile
	if err := database.Conn(ctx, r.db).First(&p, "user_id = ?", userID).Error; err != nil {
		return nil, notFound(err)
	}
	return &p, nil
}

func (r *GormUserRepository) UpdateProfile(ctx context.Context, userID uuid.UUID, fields map[string]any) error {
	fields["updated_at"] = time.Now().UTC()
	res := database.Conn(ctx, r.db).Model(&domain.Profile{}).Where("user_id = ?", userID).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// escapeLike escapes LIKE wildcards in user input.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// TokenRepository persists refresh and password-reset tokens.
type TokenRepository interface {
	CreateRefresh(ctx context.Context, t *domain.RefreshToken) error
	GetRefreshByHashForUpdate(ctx context.Context, hash string) (*domain.RefreshToken, error)
	RevokeRefresh(ctx context.Context, id uuid.UUID, reason string, replacedBy *uuid.UUID, at time.Time) error
	RevokeFamily(ctx context.Context, familyID uuid.UUID, reason string, at time.Time) (int64, error)
	RevokeAllForUser(ctx context.Context, userID uuid.UUID, reason string, at time.Time) (int64, error)
	CreateReset(ctx context.Context, t *domain.PasswordResetToken) error
	GetResetByHashForUpdate(ctx context.Context, hash string) (*domain.PasswordResetToken, error)
	MarkResetUsed(ctx context.Context, id uuid.UUID, at time.Time) error
	InvalidateResets(ctx context.Context, userID uuid.UUID, at time.Time) error
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}

// GormTokenRepository implements TokenRepository.
type GormTokenRepository struct{ db *gorm.DB }

// NewTokenRepository returns a GORM token repository.
func NewTokenRepository(db *gorm.DB) *GormTokenRepository { return &GormTokenRepository{db: db} }

func (r *GormTokenRepository) CreateRefresh(ctx context.Context, t *domain.RefreshToken) error {
	return database.Conn(ctx, r.db).Create(t).Error
}

func (r *GormTokenRepository) GetRefreshByHashForUpdate(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	var t domain.RefreshToken
	if err := database.Conn(ctx, r.db).Clauses(forUpdate).First(&t, "token_hash = ?", hash).Error; err != nil {
		return nil, notFound(err)
	}
	return &t, nil
}

func (r *GormTokenRepository) RevokeRefresh(ctx context.Context, id uuid.UUID, reason string, replacedBy *uuid.UUID, at time.Time) error {
	return database.Conn(ctx, r.db).Model(&domain.RefreshToken{}).Where("id = ? AND revoked_at IS NULL", id).
		Updates(map[string]any{"revoked_at": at, "revoke_reason": reason, "replaced_by": replacedBy, "last_used_at": at}).Error
}

func (r *GormTokenRepository) RevokeFamily(ctx context.Context, familyID uuid.UUID, reason string, at time.Time) (int64, error) {
	res := database.Conn(ctx, r.db).Model(&domain.RefreshToken{}).Where("family_id = ? AND revoked_at IS NULL", familyID).
		Updates(map[string]any{"revoked_at": at, "revoke_reason": reason})
	return res.RowsAffected, res.Error
}

func (r *GormTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID, reason string, at time.Time) (int64, error) {
	res := database.Conn(ctx, r.db).Model(&domain.RefreshToken{}).Where("user_id = ? AND revoked_at IS NULL", userID).
		Updates(map[string]any{"revoked_at": at, "revoke_reason": reason})
	return res.RowsAffected, res.Error
}

func (r *GormTokenRepository) CreateReset(ctx context.Context, t *domain.PasswordResetToken) error {
	return database.Conn(ctx, r.db).Create(t).Error
}

func (r *GormTokenRepository) GetResetByHashForUpdate(ctx context.Context, hash string) (*domain.PasswordResetToken, error) {
	var t domain.PasswordResetToken
	if err := database.Conn(ctx, r.db).Clauses(forUpdate).First(&t, "token_hash = ?", hash).Error; err != nil {
		return nil, notFound(err)
	}
	return &t, nil
}

func (r *GormTokenRepository) MarkResetUsed(ctx context.Context, id uuid.UUID, at time.Time) error {
	return database.Conn(ctx, r.db).Model(&domain.PasswordResetToken{}).Where("id = ?", id).Update("used_at", at).Error
}

func (r *GormTokenRepository) InvalidateResets(ctx context.Context, userID uuid.UUID, at time.Time) error {
	return database.Conn(ctx, r.db).Model(&domain.PasswordResetToken{}).
		Where("user_id = ? AND used_at IS NULL", userID).Update("used_at", at).Error
}

func (r *GormTokenRepository) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	conn := database.Conn(ctx, r.db)
	res := conn.Where("expires_at < ?", before).Delete(&domain.RefreshToken{})
	if res.Error != nil {
		return 0, res.Error
	}
	res2 := conn.Where("expires_at < ?", before).Delete(&domain.PasswordResetToken{})
	return res.RowsAffected + res2.RowsAffected, res2.Error
}
