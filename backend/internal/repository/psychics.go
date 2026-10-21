package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// PsychicRepository persists psychic profiles and their services.
type PsychicRepository interface {
	Create(ctx context.Context, p *domain.Psychic) error
	List(ctx context.Context, includeUnlisted bool) ([]domain.Psychic, error)
	GetBySlug(ctx context.Context, slug string) (*domain.Psychic, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) (*domain.Psychic, error)
	GetByUserIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Psychic, error)
	Updates(ctx context.Context, userID uuid.UUID, fields map[string]any) error
	IncrementReadings(ctx context.Context, userID uuid.UUID) error

	CreateService(ctx context.Context, s *domain.PsychicService) error
	GetService(ctx context.Context, id uuid.UUID) (*domain.PsychicService, error)
	UpdateService(ctx context.Context, id uuid.UUID, fields map[string]any) error
	DeleteService(ctx context.Context, id uuid.UUID) error
	ListServices(ctx context.Context, psychicID uuid.UUID, activeOnly bool) ([]domain.PsychicService, error)
	// IsPublic reports whether an advisor may appear publicly (listed and
	// the account active).
	IsPublic(ctx context.Context, userID uuid.UUID) (bool, error)
}

// GormPsychicRepository implements PsychicRepository.
type GormPsychicRepository struct{ db *gorm.DB }

// NewPsychicRepository returns a GORM psychic repository.
func NewPsychicRepository(db *gorm.DB) *GormPsychicRepository { return &GormPsychicRepository{db: db} }

func (r *GormPsychicRepository) Create(ctx context.Context, p *domain.Psychic) error {
	if err := database.Conn(ctx, r.db).Omit("Services").Create(p).Error; err != nil {
		if isUnique(err, "psychics_slug_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *GormPsychicRepository) activeServices(db *gorm.DB) *gorm.DB {
	return db.Where("active = true").Order("sort_order ASC, created_at ASC")
}

// List returns advisors in directory order: featured first (by rank),
// then the editorial sort order. The public directory only includes listed
// advisors whose account is active (banned/deleted accounts disappear).
func (r *GormPsychicRepository) List(ctx context.Context, includeUnlisted bool) ([]domain.Psychic, error) {
	q := database.Conn(ctx, r.db).Preload("Services", r.activeServices)
	if !includeUnlisted {
		q = q.Joins("JOIN users ON users.id = psychics.user_id AND users.status = ?", domain.UserActive).Where("psychics.listed = true")
	}
	var out []domain.Psychic
	err := q.Order("psychics.featured DESC, psychics.featured_rank ASC, psychics.sort_order ASC, psychics.name ASC").Find(&out).Error
	return out, err
}

func (r *GormPsychicRepository) IsPublic(ctx context.Context, userID uuid.UUID) (bool, error) {
	var ok bool
	err := database.Conn(ctx, r.db).Raw(`SELECT EXISTS (SELECT 1 FROM psychics p JOIN users u ON u.id = p.user_id
		WHERE p.user_id = ? AND p.listed AND u.status = ?)`, userID, domain.UserActive).Scan(&ok).Error
	return ok, err
}

// DeleteService removes a service no order references (ErrInUse otherwise).
func (r *GormPsychicRepository) DeleteService(ctx context.Context, id uuid.UUID) error {
	res := database.Conn(ctx, r.db).Where("id = ?", id).Delete(&domain.PsychicService{})
	if res.Error != nil {
		if database.IsForeignKeyViolation(res.Error) {
			return ErrInUse
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormPsychicRepository) GetBySlug(ctx context.Context, slug string) (*domain.Psychic, error) {
	var p domain.Psychic
	if err := database.Conn(ctx, r.db).Preload("Services", r.activeServices).First(&p, "slug = ?", slug).Error; err != nil {
		return nil, notFound(err)
	}
	return &p, nil
}

func (r *GormPsychicRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*domain.Psychic, error) {
	var p domain.Psychic
	if err := database.Conn(ctx, r.db).Preload("Services", r.activeServices).First(&p, "user_id = ?", userID).Error; err != nil {
		return nil, notFound(err)
	}
	return &p, nil
}

func (r *GormPsychicRepository) GetByUserIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Psychic, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []domain.Psychic
	err := database.Conn(ctx, r.db).Where("user_id IN ?", ids).Find(&out).Error
	return out, err
}

func (r *GormPsychicRepository) Updates(ctx context.Context, userID uuid.UUID, fields map[string]any) error {
	fields["updated_at"] = time.Now().UTC()
	res := database.Conn(ctx, r.db).Model(&domain.Psychic{}).Where("user_id = ?", userID).Updates(fields)
	if res.Error != nil {
		if isUnique(res.Error, "psychics_slug_key") {
			return ErrDuplicate
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormPsychicRepository) IncrementReadings(ctx context.Context, userID uuid.UUID) error {
	return database.Conn(ctx, r.db).Model(&domain.Psychic{}).Where("user_id = ?", userID).
		UpdateColumn("readings_count", gorm.Expr("readings_count + 1")).Error
}

func (r *GormPsychicRepository) CreateService(ctx context.Context, s *domain.PsychicService) error {
	return database.Conn(ctx, r.db).Create(s).Error
}

func (r *GormPsychicRepository) GetService(ctx context.Context, id uuid.UUID) (*domain.PsychicService, error) {
	var s domain.PsychicService
	if err := database.Conn(ctx, r.db).First(&s, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

func (r *GormPsychicRepository) UpdateService(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	fields["updated_at"] = time.Now().UTC()
	res := database.Conn(ctx, r.db).Model(&domain.PsychicService{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormPsychicRepository) ListServices(ctx context.Context, psychicID uuid.UUID, activeOnly bool) ([]domain.PsychicService, error) {
	q := database.Conn(ctx, r.db).Where("psychic_id = ?", psychicID)
	if activeOnly {
		q = q.Where("active = true")
	}
	var out []domain.PsychicService
	err := q.Order("sort_order ASC, created_at ASC").Find(&out).Error
	return out, err
}

// CreditRepository persists credit packs and purchases.
type CreditRepository interface {
	ListPacks(ctx context.Context, activeOnly bool) ([]domain.CreditPack, error)
	GetPack(ctx context.Context, id uuid.UUID) (*domain.CreditPack, error)
	CreatePack(ctx context.Context, p *domain.CreditPack) error
	UpdatePack(ctx context.Context, id uuid.UUID, fields map[string]any) error
	DeletePack(ctx context.Context, id uuid.UUID) error

	CreatePurchase(ctx context.Context, p *domain.CreditPurchase) error
	GetPurchaseForUpdate(ctx context.Context, id uuid.UUID) (*domain.CreditPurchase, error)
	GetPurchaseByKey(ctx context.Context, userID uuid.UUID, key string) (*domain.CreditPurchase, error)
	GetPurchaseByProviderRef(ctx context.Context, provider, ref string) (*domain.CreditPurchase, error)
	UpdatePurchase(ctx context.Context, id uuid.UUID, fields map[string]any) error
	ListPurchases(ctx context.Context, userID uuid.UUID, cursor *Cursor, limit int) ([]domain.CreditPurchase, error)
}

// GormCreditRepository implements CreditRepository.
type GormCreditRepository struct{ db *gorm.DB }

// NewCreditRepository returns a GORM credit repository.
func NewCreditRepository(db *gorm.DB) *GormCreditRepository { return &GormCreditRepository{db: db} }

func (r *GormCreditRepository) ListPacks(ctx context.Context, activeOnly bool) ([]domain.CreditPack, error) {
	q := database.Conn(ctx, r.db)
	if activeOnly {
		q = q.Where("active = true")
	}
	var out []domain.CreditPack
	err := q.Order("sort_order ASC, credits ASC").Find(&out).Error
	return out, err
}

func (r *GormCreditRepository) GetPack(ctx context.Context, id uuid.UUID) (*domain.CreditPack, error) {
	var p domain.CreditPack
	if err := database.Conn(ctx, r.db).First(&p, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &p, nil
}

func (r *GormCreditRepository) CreatePack(ctx context.Context, p *domain.CreditPack) error {
	return database.Conn(ctx, r.db).Create(p).Error
}

// DeletePack removes a pack no purchase references (ErrInUse otherwise).
func (r *GormCreditRepository) DeletePack(ctx context.Context, id uuid.UUID) error {
	res := database.Conn(ctx, r.db).Where("id = ?", id).Delete(&domain.CreditPack{})
	if res.Error != nil {
		if database.IsForeignKeyViolation(res.Error) {
			return ErrInUse
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormCreditRepository) UpdatePack(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	fields["updated_at"] = time.Now().UTC()
	res := database.Conn(ctx, r.db).Model(&domain.CreditPack{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormCreditRepository) CreatePurchase(ctx context.Context, p *domain.CreditPurchase) error {
	if err := database.Conn(ctx, r.db).Create(p).Error; err != nil {
		if isUnique(err, "credit_purchases_idempotency_key", "credit_purchases_provider_ref_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *GormCreditRepository) GetPurchaseForUpdate(ctx context.Context, id uuid.UUID) (*domain.CreditPurchase, error) {
	var p domain.CreditPurchase
	if err := database.Conn(ctx, r.db).Clauses(forUpdate).First(&p, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &p, nil
}

func (r *GormCreditRepository) GetPurchaseByKey(ctx context.Context, userID uuid.UUID, key string) (*domain.CreditPurchase, error) {
	var p domain.CreditPurchase
	if err := database.Conn(ctx, r.db).First(&p, "user_id = ? AND idempotency_key = ?", userID, key).Error; err != nil {
		return nil, notFound(err)
	}
	return &p, nil
}

func (r *GormCreditRepository) GetPurchaseByProviderRef(ctx context.Context, provider, ref string) (*domain.CreditPurchase, error) {
	var p domain.CreditPurchase
	if err := database.Conn(ctx, r.db).First(&p, "provider = ? AND provider_ref = ?", provider, ref).Error; err != nil {
		return nil, notFound(err)
	}
	return &p, nil
}

func (r *GormCreditRepository) UpdatePurchase(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	fields["updated_at"] = time.Now().UTC()
	return database.Conn(ctx, r.db).Model(&domain.CreditPurchase{}).Where("id = ?", id).Updates(fields).Error
}

func (r *GormCreditRepository) ListPurchases(ctx context.Context, userID uuid.UUID, cursor *Cursor, limit int) ([]domain.CreditPurchase, error) {
	q := database.Conn(ctx, r.db).Where("user_id = ?", userID)
	q = applyCursor(q, cursor, "created_at", "id")
	var out []domain.CreditPurchase
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(limit, 20, 100)).Find(&out).Error
	return out, err
}
