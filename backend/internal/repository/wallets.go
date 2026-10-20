package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// WalletRepository persists wallets and the append-only credit ledger.
type WalletRepository interface {
	Create(ctx context.Context, w *domain.Wallet) error
	Get(ctx context.Context, userID uuid.UUID) (*domain.Wallet, error)
	GetForUpdate(ctx context.Context, userID uuid.UUID) (*domain.Wallet, error)
	Save(ctx context.Context, w *domain.Wallet) error
	AppendLedger(ctx context.Context, e *domain.LedgerEntry) error
	LedgerByKey(ctx context.Context, key string) (*domain.LedgerEntry, error)
	ListLedger(ctx context.Context, userID uuid.UUID, cursor *Cursor, limit int) ([]domain.LedgerEntry, error)
}

// GormWalletRepository implements WalletRepository.
type GormWalletRepository struct{ db *gorm.DB }

// NewWalletRepository returns a GORM wallet repository.
func NewWalletRepository(db *gorm.DB) *GormWalletRepository { return &GormWalletRepository{db: db} }

func (r *GormWalletRepository) Create(ctx context.Context, w *domain.Wallet) error {
	return database.Conn(ctx, r.db).Create(w).Error
}

func (r *GormWalletRepository) Get(ctx context.Context, userID uuid.UUID) (*domain.Wallet, error) {
	var w domain.Wallet
	if err := database.Conn(ctx, r.db).First(&w, "user_id = ?", userID).Error; err != nil {
		return nil, notFound(err)
	}
	return &w, nil
}

func (r *GormWalletRepository) GetForUpdate(ctx context.Context, userID uuid.UUID) (*domain.Wallet, error) {
	var w domain.Wallet
	if err := database.Conn(ctx, r.db).Clauses(forUpdate).First(&w, "user_id = ?", userID).Error; err != nil {
		return nil, notFound(err)
	}
	return &w, nil
}

func (r *GormWalletRepository) Save(ctx context.Context, w *domain.Wallet) error {
	return database.Conn(ctx, r.db).Model(&domain.Wallet{}).Where("user_id = ?", w.UserID).Updates(map[string]any{
		"balance": w.Balance, "held": w.Held, "free_minutes": w.FreeMinutes, "updated_at": w.UpdatedAt,
	}).Error
}

func (r *GormWalletRepository) AppendLedger(ctx context.Context, e *domain.LedgerEntry) error {
	if err := database.Conn(ctx, r.db).Create(e).Error; err != nil {
		if isUnique(err, "credit_ledger_idempotency_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *GormWalletRepository) LedgerByKey(ctx context.Context, key string) (*domain.LedgerEntry, error) {
	var e domain.LedgerEntry
	if err := database.Conn(ctx, r.db).First(&e, "idempotency_key = ?", key).Error; err != nil {
		return nil, notFound(err)
	}
	return &e, nil
}

func (r *GormWalletRepository) ListLedger(ctx context.Context, userID uuid.UUID, cursor *Cursor, limit int) ([]domain.LedgerEntry, error) {
	q := database.Conn(ctx, r.db).Where("user_id = ?", userID)
	q = applyCursor(q, cursor, "created_at", "id")
	var out []domain.LedgerEntry
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(limit, 50, 100)).Find(&out).Error
	return out, err
}
