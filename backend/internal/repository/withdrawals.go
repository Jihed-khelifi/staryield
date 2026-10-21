package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// WithdrawalFilter narrows withdrawal listings.
type WithdrawalFilter struct {
	PsychicID *uuid.UUID
	Statuses  []domain.WithdrawalStatus
	Cursor    *Cursor
	Limit     int
}

// PayoutBalance summarises an advisor's earned and withdrawn credits.
type PayoutBalance struct {
	LifetimeNet int64
	Reserved    int64 // pending + approved withdrawals
	Paid        int64
}

// Available returns credits that can still be withdrawn.
func (b PayoutBalance) Available() int64 {
	if v := b.LifetimeNet - b.Reserved - b.Paid; v > 0 {
		return v
	}
	return 0
}

// WithdrawalRepository persists advisor payouts.
type WithdrawalRepository interface {
	Create(ctx context.Context, w *domain.Withdrawal) error
	Get(ctx context.Context, id uuid.UUID) (*domain.Withdrawal, error)
	GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.Withdrawal, error)
	GetByKey(ctx context.Context, psychicID uuid.UUID, key string) (*domain.Withdrawal, error)
	Save(ctx context.Context, w *domain.Withdrawal) error
	List(ctx context.Context, f WithdrawalFilter) ([]domain.Withdrawal, error)
	// Balance computes the payout balance; call inside the transaction that
	// locked the advisor row to serialise withdrawals.
	Balance(ctx context.Context, psychicID uuid.UUID) (PayoutBalance, error)
	LockPsychic(ctx context.Context, psychicID uuid.UUID) error
}

// GormWithdrawalRepository implements WithdrawalRepository.
type GormWithdrawalRepository struct{ db *gorm.DB }

// NewWithdrawalRepository returns a GORM withdrawal repository.
func NewWithdrawalRepository(db *gorm.DB) *GormWithdrawalRepository {
	return &GormWithdrawalRepository{db: db}
}

func (r *GormWithdrawalRepository) Create(ctx context.Context, w *domain.Withdrawal) error {
	if err := database.Conn(ctx, r.db).Create(w).Error; err != nil {
		if isUnique(err, "withdrawals_one_open", "withdrawals_idempotency_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *GormWithdrawalRepository) Get(ctx context.Context, id uuid.UUID) (*domain.Withdrawal, error) {
	var w domain.Withdrawal
	if err := database.Conn(ctx, r.db).First(&w, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &w, nil
}

func (r *GormWithdrawalRepository) GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.Withdrawal, error) {
	var w domain.Withdrawal
	if err := database.Conn(ctx, r.db).Clauses(forUpdate).First(&w, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &w, nil
}

func (r *GormWithdrawalRepository) GetByKey(ctx context.Context, psychicID uuid.UUID, key string) (*domain.Withdrawal, error) {
	var w domain.Withdrawal
	if err := database.Conn(ctx, r.db).First(&w, "psychic_id = ? AND idempotency_key = ?", psychicID, key).Error; err != nil {
		return nil, notFound(err)
	}
	return &w, nil
}

func (r *GormWithdrawalRepository) Save(ctx context.Context, w *domain.Withdrawal) error {
	w.UpdatedAt = time.Now().UTC()
	return database.Conn(ctx, r.db).Save(w).Error
}

func (r *GormWithdrawalRepository) List(ctx context.Context, f WithdrawalFilter) ([]domain.Withdrawal, error) {
	q := database.Conn(ctx, r.db).Model(&domain.Withdrawal{})
	if f.PsychicID != nil {
		q = q.Where("psychic_id = ?", *f.PsychicID)
	}
	if len(f.Statuses) > 0 {
		q = q.Where("status IN ?", f.Statuses)
	}
	q = applyCursor(q, f.Cursor, "requested_at", "id")
	var out []domain.Withdrawal
	err := q.Order("requested_at DESC, id DESC").Limit(clampLimit(f.Limit, 20, 100)).Find(&out).Error
	return out, err
}

func (r *GormWithdrawalRepository) Balance(ctx context.Context, psychicID uuid.UUID) (PayoutBalance, error) {
	var b PayoutBalance
	err := database.Conn(ctx, r.db).Raw(`
		SELECT
		  (SELECT COALESCE(sum(net_credits), 0) FROM psychic_earnings WHERE psychic_id = ?) AS lifetime_net,
		  (SELECT COALESCE(sum(amount_credits), 0) FROM withdrawals WHERE psychic_id = ? AND status IN ('pending', 'approved')) AS reserved,
		  (SELECT COALESCE(sum(amount_credits), 0) FROM withdrawals WHERE psychic_id = ? AND status = 'paid') AS paid`,
		psychicID, psychicID, psychicID).Scan(&b).Error
	return b, err
}

func (r *GormWithdrawalRepository) LockPsychic(ctx context.Context, psychicID uuid.UUID) error {
	var p domain.Psychic
	err := database.Conn(ctx, r.db).Clauses(forUpdate).Select("user_id").First(&p, "user_id = ?", psychicID).Error
	return notFound(err)
}
