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

	// --- advisor settings (M4) ---
	// GetForUpdate locks an advisor's profile row (services not loaded).
	GetForUpdate(ctx context.Context, userID uuid.UUID) (*domain.Psychic, error)
	// CustomService returns the advisor's live custom offering
	// (kind 'custom', active or not).
	CustomService(ctx context.Context, psychicID uuid.UUID) (*domain.PsychicService, error)
	// GetServiceAny returns a service even when it was removed: orders keep
	// referencing removed services.
	GetServiceAny(ctx context.Context, id uuid.UUID) (*domain.PsychicService, error)
	// --- end advisor settings ---
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
	var n int64
	err := database.Conn(ctx, r.db).Model(&domain.Psychic{}).
		Joins("JOIN users ON users.id = psychics.user_id AND users.status = ?", domain.UserActive).
		Where("psychics.user_id = ? AND psychics.listed", userID).Count(&n).Error
	return n > 0, err
}

// DeleteService soft-deletes a service (gorm.DeletedAt): orders that
// reference it keep their history. ErrInUse is kept for hard deletes.
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

// GetForUpdate implements PsychicRepository.
func (r *GormPsychicRepository) GetForUpdate(ctx context.Context, userID uuid.UUID) (*domain.Psychic, error) {
	var p domain.Psychic
	if err := database.Conn(ctx, r.db).Clauses(forUpdate).First(&p, "user_id = ?", userID).Error; err != nil {
		return nil, notFound(err)
	}
	return &p, nil
}

// CustomService implements PsychicRepository.
func (r *GormPsychicRepository) CustomService(ctx context.Context, psychicID uuid.UUID) (*domain.PsychicService, error) {
	var s domain.PsychicService
	err := database.Conn(ctx, r.db).Where("psychic_id = ? AND kind = ?", psychicID, domain.ServiceCustom).First(&s).Error
	if err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

// GetServiceAny implements PsychicRepository.
func (r *GormPsychicRepository) GetServiceAny(ctx context.Context, id uuid.UUID) (*domain.PsychicService, error) {
	var s domain.PsychicService
	if err := database.Conn(ctx, r.db).Unscoped().First(&s, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
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
	// revisions 5.1: PurchasesByIDs loads purchases in one query (admin
	// consent records); PurchaseReceipt reads what a receipt shows in one join.
	PurchasesByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.CreditPurchase, error)
	PurchaseReceipt(ctx context.Context, purchaseID uuid.UUID) (*PurchaseReceipt, error)
}

// PurchaseReceipt is the pack and the card behind a purchase (revisions 5.1,
// §25.5): the pack name and, when the payment used a card (saved or tokenised),
// its brand and last four digits.
type PurchaseReceipt struct {
	PackName string
	Brand    string
	Last4    string
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

// PurchasesByIDs loads purchases by id in one query.
func (r *GormCreditRepository) PurchasesByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.CreditPurchase, error) {
	var out []domain.CreditPurchase
	if len(ids) == 0 {
		return out, nil
	}
	err := database.Conn(ctx, r.db).Where("id IN ?", ids).Find(&out).Error
	return out, err
}

// PurchaseReceipt reads the pack name and the card of a purchase: one query
// joining the pack, the purchase's ledger transaction and the card it charged
// (cards removed since still carry their brand and last four digits).
func (r *GormCreditRepository) PurchaseReceipt(ctx context.Context, purchaseID uuid.UUID) (*PurchaseReceipt, error) {
	var row struct {
		PackName string
		Brand    *string
		Last4    *string
	}
	res := database.Conn(ctx, r.db).Table("credit_purchases AS p").
		Select("cp.name AS pack_name, pm.brand AS brand, pm.last4 AS last4").
		Joins("JOIN credit_packs cp ON cp.id = p.pack_id").
		Joins("LEFT JOIN payment_transactions t ON t.credit_purchase_id = p.id AND t.kind = ?", domain.TxCreditPurchase).
		Joins("LEFT JOIN payment_methods pm ON pm.id = t.payment_method_id").
		Where("p.id = ?", purchaseID).Limit(1).Scan(&row)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	out := &PurchaseReceipt{PackName: row.PackName}
	if row.Brand != nil {
		out.Brand = *row.Brand
	}
	if row.Last4 != nil {
		out.Last4 = *row.Last4
	}
	return out, nil
}
