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

// revisions 5.0: pricing catalog and welcome offer (§24.1, §24.2).

// Pricing repository errors.
var (
	// ErrPackCodeTaken: another pack already uses the code.
	ErrPackCodeTaken = errors.New("repository: pack code taken")
	// ErrIntroActive: another active intro pack exists.
	ErrIntroActive = errors.New("repository: an intro pack is already active")
	// ErrReferenceTaken: another pack is the reference (a concurrent change).
	ErrReferenceTaken = errors.New("repository: another pack is the reference")
	// ErrOfferUserTaken: the account already has a reserved or redeemed offer.
	ErrOfferUserTaken = errors.New("repository: the account already has the offer")
	// ErrOfferCardTaken: the payment method already has the offer.
	ErrOfferCardTaken = errors.New("repository: the payment method already has the offer")
)

// PackCount is the number of succeeded purchases of a pack.
type PackCount struct {
	PackID    uuid.UUID
	Purchases int64
}

// CatalogState is what the top-up catalog needs about one account: its
// wallet and the two facts that decide welcome-offer eligibility.
type CatalogState struct {
	Balance     int64
	Held        int64
	FreeMinutes int
	// HasRedemption: a reserved or redeemed welcome-offer redemption exists.
	HasRedemption bool
	// HasPaid: a succeeded (or since refunded) credit purchase exists.
	HasPaid bool
}

// PricingRepository persists the pricing catalog (credit_packs columns of
// revisions 5.0) and the welcome-offer redemptions.
type PricingRepository interface {
	// Packs lists every pack in canonical order (sort_order, credits, id).
	Packs(ctx context.Context) ([]domain.CreditPack, error)
	Pack(ctx context.Context, id uuid.UUID) (*domain.CreditPack, error)
	PackByCode(ctx context.Context, code string) (*domain.CreditPack, error)
	// PackForUpdate locks and returns a pack.
	PackForUpdate(ctx context.Context, id uuid.UUID) (*domain.CreditPack, error)
	// LockPacks serialises catalog writes (pack create/update/delete/order).
	LockPacks(ctx context.Context) error
	CreatePack(ctx context.Context, p *domain.CreditPack) error
	// SavePack writes every column of a pack.
	SavePack(ctx context.Context, p *domain.CreditPack) error
	// ClearReference removes the reference flag from every pack but one.
	ClearReference(ctx context.Context, except uuid.UUID) error
	// SetPackOrder gives the packs the sort_order of their position.
	SetPackOrder(ctx context.Context, ids []uuid.UUID) error
	// PurchaseCounts counts succeeded purchases per pack completed at or after since.
	PurchaseCounts(ctx context.Context, since time.Time) ([]PackCount, error)
	// DeletePack removes a pack (ErrInUse when purchases reference it).
	DeletePack(ctx context.Context, id uuid.UUID) error
	// PackInUse reports whether a purchase references the pack.
	PackInUse(ctx context.Context, id uuid.UUID) (bool, error)

	// CatalogState reads wallet and offer facts of an account in one statement.
	CatalogState(ctx context.Context, userID uuid.UUID) (*CatalogState, error)

	// ReserveOffer inserts a reserved redemption (ErrOfferUserTaken /
	// ErrOfferCardTaken on a unique violation).
	ReserveOffer(ctx context.Context, r *domain.OfferRedemption) error
	// MoveOffer changes the status of the purchase's redemption when it is
	// in `from`; it reports whether a row moved.
	MoveOffer(ctx context.Context, purchaseID uuid.UUID, from, to domain.OfferStatus, at time.Time) (bool, error)
	OfferByPurchase(ctx context.Context, purchaseID uuid.UUID) (*domain.OfferRedemption, error)
}

// GormPricingRepository implements PricingRepository.
type GormPricingRepository struct{ db *gorm.DB }

// NewPricingRepository returns a GORM pricing repository.
func NewPricingRepository(db *gorm.DB) *GormPricingRepository { return &GormPricingRepository{db: db} }

// Packs implements PricingRepository.
func (r *GormPricingRepository) Packs(ctx context.Context) ([]domain.CreditPack, error) {
	var out []domain.CreditPack
	err := database.Conn(ctx, r.db).Order("sort_order ASC, credits ASC, id ASC").Find(&out).Error
	return out, err
}

// Pack implements PricingRepository.
func (r *GormPricingRepository) Pack(ctx context.Context, id uuid.UUID) (*domain.CreditPack, error) {
	var p domain.CreditPack
	if err := database.Conn(ctx, r.db).First(&p, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &p, nil
}

// PackByCode implements PricingRepository.
func (r *GormPricingRepository) PackByCode(ctx context.Context, code string) (*domain.CreditPack, error) {
	var p domain.CreditPack
	if err := database.Conn(ctx, r.db).First(&p, "code = ?", code).Error; err != nil {
		return nil, notFound(err)
	}
	return &p, nil
}

// PackForUpdate implements PricingRepository.
func (r *GormPricingRepository) PackForUpdate(ctx context.Context, id uuid.UUID) (*domain.CreditPack, error) {
	var p domain.CreditPack
	if err := database.Conn(ctx, r.db).Clauses(forUpdate).First(&p, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &p, nil
}

// LockPacks implements PricingRepository: the pricing settings row is the
// catalog's write lock (admin catalog writes are rare).
func (r *GormPricingRepository) LockPacks(ctx context.Context) error {
	var s domain.SiteSetting
	return notFound(database.Conn(ctx, r.db).Clauses(forUpdate).First(&s, "key = ?", domain.SiteSettingPricing).Error)
}

func packWriteError(err error) error {
	switch {
	case err == nil:
		return nil
	case isUnique(err, "credit_packs_code_key"):
		return ErrPackCodeTaken
	case isUnique(err, "credit_packs_one_intro_idx"):
		return ErrIntroActive
	case isUnique(err, "credit_packs_one_reference_idx"):
		return ErrReferenceTaken
	}
	return err
}

// CreatePack implements PricingRepository.
func (r *GormPricingRepository) CreatePack(ctx context.Context, p *domain.CreditPack) error {
	return packWriteError(database.Conn(ctx, r.db).Create(p).Error)
}

// SavePack implements PricingRepository.
func (r *GormPricingRepository) SavePack(ctx context.Context, p *domain.CreditPack) error {
	return packWriteError(database.Conn(ctx, r.db).Save(p).Error)
}

// ClearReference implements PricingRepository.
func (r *GormPricingRepository) ClearReference(ctx context.Context, except uuid.UUID) error {
	return database.Conn(ctx, r.db).Model(&domain.CreditPack{}).Where("reference AND id <> ?", except).
		Update("reference", false).Error
}

// SetPackOrder implements PricingRepository.
func (r *GormPricingRepository) SetPackOrder(ctx context.Context, ids []uuid.UUID) error {
	conn := database.Conn(ctx, r.db)
	for i, id := range ids {
		if err := conn.Model(&domain.CreditPack{}).Where("id = ? AND sort_order <> ?", id, i+1).
			Update("sort_order", i+1).Error; err != nil {
			return err
		}
	}
	return nil
}

// PurchaseCounts implements PricingRepository (credit_purchases_succeeded_completed_idx).
func (r *GormPricingRepository) PurchaseCounts(ctx context.Context, since time.Time) ([]PackCount, error) {
	var out []PackCount
	err := database.Conn(ctx, r.db).Model(&domain.CreditPurchase{}).Select("pack_id, count(*) AS purchases").
		Where("status = ? AND completed_at >= ?", domain.PurchaseSucceeded, since).Group("pack_id").Scan(&out).Error
	return out, err
}

// PackInUse implements PricingRepository.
func (r *GormPricingRepository) PackInUse(ctx context.Context, id uuid.UUID) (bool, error) {
	var n int64
	err := database.Conn(ctx, r.db).Model(&domain.CreditPurchase{}).Where("pack_id = ?", id).Limit(1).Count(&n).Error
	return n > 0, err
}

// CatalogState implements PricingRepository: one statement, the offer
// facts as EXISTS subqueries (offer_redemptions_user_active_idx,
// credit_purchases_user_paid_idx).
func (r *GormPricingRepository) CatalogState(ctx context.Context, userID uuid.UUID) (*CatalogState, error) {
	redeemed := r.db.Table("offer_redemptions").Select("1").
		Where("user_id = ? AND status IN ?", userID, []domain.OfferStatus{domain.OfferReserved, domain.OfferRedeemed})
	paid := r.db.Table("credit_purchases").Select("1").
		Where("user_id = ? AND status IN ?", userID, []domain.PurchaseStatus{domain.PurchaseSucceeded, domain.PurchaseRefunded})
	var st CatalogState
	res := database.Conn(ctx, r.db).Table("wallets").
		Select("balance, held, free_minutes, EXISTS(?) AS has_redemption, EXISTS(?) AS has_paid", redeemed, paid).
		Where("user_id = ?", userID).Scan(&st)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return &st, nil
}

// ReserveOffer implements PricingRepository.
func (r *GormPricingRepository) ReserveOffer(ctx context.Context, o *domain.OfferRedemption) error {
	err := database.Conn(ctx, r.db).Create(o).Error
	switch {
	case err == nil:
		return nil
	case isUnique(err, domain.OfferUserIndex):
		return ErrOfferUserTaken
	case isUnique(err, domain.OfferCardIndex):
		return ErrOfferCardTaken
	case isUnique(err):
		return ErrDuplicate
	}
	return err
}

// MoveOffer implements PricingRepository.
func (r *GormPricingRepository) MoveOffer(ctx context.Context, purchaseID uuid.UUID, from, to domain.OfferStatus, at time.Time) (bool, error) {
	fields := map[string]any{"status": to, "updated_at": at}
	switch to {
	case domain.OfferRedeemed:
		fields["redeemed_at"] = at
	case domain.OfferReleased:
		fields["released_at"] = at
	}
	res := database.Conn(ctx, r.db).Model(&domain.OfferRedemption{}).
		Where("purchase_id = ? AND status = ?", purchaseID, from).Updates(fields)
	return res.RowsAffected > 0, res.Error
}

// OfferByPurchase implements PricingRepository.
func (r *GormPricingRepository) OfferByPurchase(ctx context.Context, purchaseID uuid.UUID) (*domain.OfferRedemption, error) {
	var o domain.OfferRedemption
	if err := database.Conn(ctx, r.db).First(&o, "purchase_id = ?", purchaseID).Error; err != nil {
		return nil, notFound(err)
	}
	return &o, nil
}

// DeletePack implements PricingRepository.
func (r *GormPricingRepository) DeletePack(ctx context.Context, id uuid.UUID) error {
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
