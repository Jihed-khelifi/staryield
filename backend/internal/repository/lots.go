package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 5.0: credit lots (docs/platform-extensions.md section 24.3).

// LotAllocationRow is one allocation of a movement joined with the current
// state of its lot (one query gives everything a restore needs).
type LotAllocationRow struct {
	LotID           uuid.UUID
	LedgerEntryID   uuid.UUID
	RestoresEntryID *uuid.UUID
	Credits         int64
	ValueMillicents int64
	LotPriority     int16
	LotCreatedAt    time.Time
	LotTotal        int64
	LotRemaining    int64
	LotPaidCents    int64
	LotValue        int64
}

// LotMismatch is a wallet whose balance differs from the sum of its lots.
type LotMismatch struct {
	UserID        uuid.UUID
	Balance       int64
	LotsRemaining int64
}

// LotChunk is one page of the reconciliation scan.
type LotChunk struct {
	Scanned    int
	Last       uuid.UUID // the highest user id scanned (pass it as the next `after`)
	Mismatches []LotMismatch
}

// LotRepository persists credit lots and their allocations. Only
// WalletService.Apply changes lots; everything else reads.
type LotRepository interface {
	CreateLot(ctx context.Context, l *domain.CreditLot) error
	// SpendableLots returns the user's lots with credits left in consumption
	// order (priority, created_at, id), at most limit of them.
	SpendableLots(ctx context.Context, userID uuid.UUID, limit int) ([]domain.LotStock, error)
	// LotOf returns one lot of the user.
	LotOf(ctx context.Context, userID, lotID uuid.UUID) (*domain.CreditLot, error)
	// ApplyDeltas changes the lots by deltas (positive consumes, negative
	// restores) and appends one allocation per delta for the movement, in one
	// statement. It fails (and the caller's transaction must roll back) if a
	// lot does not belong to the user or would leave its bounds.
	ApplyDeltas(ctx context.Context, userID, entryID uuid.UUID, restores *uuid.UUID, deltas []domain.LotDelta, now time.Time) error
	// Allocations lists what a movement consumed, and what later movements
	// restored of it, joined with the lots' current state. The movement is
	// named by ledger entry id or, when entryID is nil, by idempotency key.
	Allocations(ctx context.Context, userID uuid.UUID, entryID *uuid.UUID, key string) ([]LotAllocationRow, error)

	PurchaseLot(ctx context.Context, purchaseID uuid.UUID) (*domain.CreditLot, error)
	GrantLot(ctx context.Context, grantID uuid.UUID) (*domain.CreditLot, error)
	PurchaseLots(ctx context.Context, purchaseIDs []uuid.UUID) (map[uuid.UUID]domain.CreditLot, error)
	UserLots(ctx context.Context, userID uuid.UUID, limit int) ([]domain.CreditLot, error)

	// Reconcile scans size wallets after the given user id and returns the
	// ones whose balance differs from Σ credits_remaining.
	Reconcile(ctx context.Context, after uuid.UUID, size int) (LotChunk, error)
}

// GormLotRepository implements LotRepository.
type GormLotRepository struct{ db *gorm.DB }

// NewLotRepository returns a GORM lot repository.
func NewLotRepository(db *gorm.DB) *GormLotRepository { return &GormLotRepository{db: db} }

func (r *GormLotRepository) CreateLot(ctx context.Context, l *domain.CreditLot) error {
	if err := database.Conn(ctx, r.db).Create(l).Error; err != nil {
		if isUnique(err, "credit_lots_ledger_entry_key", "credit_lots_purchase_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *GormLotRepository) SpendableLots(ctx context.Context, userID uuid.UUID, limit int) ([]domain.LotStock, error) {
	var out []domain.LotStock
	err := database.Conn(ctx, r.db).Model(&domain.CreditLot{}).
		Select("id, credits_remaining, value_remaining_millicents").
		Where("user_id = ? AND credits_remaining > 0", userID).
		Order("priority, created_at, id").Limit(limit).Find(&out).Error
	return out, err
}

func (r *GormLotRepository) LotOf(ctx context.Context, userID, lotID uuid.UUID) (*domain.CreditLot, error) {
	var l domain.CreditLot
	if err := database.Conn(ctx, r.db).First(&l, "id = ? AND user_id = ?", lotID, userID).Error; err != nil {
		return nil, notFound(err)
	}
	return &l, nil
}

// ApplyDeltas: the lots are changed with relative updates (remaining −
// credits), so consumption and restoration are the same statement. The
// allocation insert joins the update's RETURNING rows, so an allocation is
// written only for a lot that really changed; any mismatch is an error.
func (r *GormLotRepository) ApplyDeltas(ctx context.Context, userID, entryID uuid.UUID, restores *uuid.UUID, deltas []domain.LotDelta, now time.Time) error {
	if len(deltas) == 0 {
		return nil
	}
	values := make([]string, len(deltas))
	args := make([]any, 0, len(deltas)*4+6)
	for i, d := range deltas {
		values[i] = "(?::uuid, ?::uuid, ?::bigint, ?::bigint)"
		args = append(args, domain.NewID(), d.LotID, d.Credits, d.ValueMillicents)
	}
	var restoresArg any
	if restores != nil {
		restoresArg = *restores
	}
	args = append(args, now, userID, entryID, userID, restoresArg, now)
	res := database.Conn(ctx, r.db).Exec(`WITH d (alloc_id, lot_id, credits, value) AS (VALUES `+strings.Join(values, ", ")+`),
u AS (
    UPDATE credit_lots l SET credits_remaining = l.credits_remaining - d.credits,
        value_remaining_millicents = l.value_remaining_millicents - d.value, updated_at = ?
    FROM d WHERE l.id = d.lot_id AND l.user_id = ? RETURNING l.id
)
INSERT INTO credit_lot_allocations (id, ledger_entry_id, lot_id, user_id, credits, value_millicents, restores_entry_id, created_at)
SELECT d.alloc_id, ?::uuid, d.lot_id, ?::uuid, d.credits, d.value, ?::uuid, ? FROM d JOIN u ON u.id = d.lot_id`, args...)
	if res.Error != nil {
		return res.Error
	}
	if int(res.RowsAffected) != len(deltas) {
		return fmt.Errorf("repository: lot update touched %d of %d lots", res.RowsAffected, len(deltas))
	}
	return nil
}

func (r *GormLotRepository) Allocations(ctx context.Context, userID uuid.UUID, entryID *uuid.UUID, key string) ([]LotAllocationRow, error) {
	q := database.Conn(ctx, r.db).Table("credit_lot_allocations a").
		Select("a.lot_id, a.ledger_entry_id, a.restores_entry_id, a.credits, a.value_millicents, l.priority AS lot_priority, "+
			"l.created_at AS lot_created_at, l.credits_total AS lot_total, l.credits_remaining AS lot_remaining, "+
			"l.paid_cents AS lot_paid_cents, l.value_remaining_millicents AS lot_value").
		Joins("JOIN credit_lots l ON l.id = a.lot_id").
		Where("a.user_id = ?", userID)
	if entryID != nil {
		q = q.Where("((a.ledger_entry_id = ? AND a.credits > 0) OR a.restores_entry_id = ?)", *entryID, *entryID)
	} else {
		q = q.Joins("JOIN credit_ledger e ON e.idempotency_key = ?", key).
			Where("((a.ledger_entry_id = e.id AND a.credits > 0) OR a.restores_entry_id = e.id)")
	}
	var out []LotAllocationRow
	err := q.Order("l.priority, l.created_at, l.id, a.created_at, a.id").Scan(&out).Error
	return out, err
}

func (r *GormLotRepository) PurchaseLot(ctx context.Context, purchaseID uuid.UUID) (*domain.CreditLot, error) {
	return r.byReference(ctx, domain.LotRefCreditPurchase, purchaseID)
}

func (r *GormLotRepository) GrantLot(ctx context.Context, grantID uuid.UUID) (*domain.CreditLot, error) {
	return r.byReference(ctx, domain.LotRefSubscriptionGrant, grantID)
}

func (r *GormLotRepository) byReference(ctx context.Context, refType string, id uuid.UUID) (*domain.CreditLot, error) {
	var l domain.CreditLot
	if err := database.Conn(ctx, r.db).First(&l, "reference_type = ? AND reference_id = ?", refType, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &l, nil
}

func (r *GormLotRepository) PurchaseLots(ctx context.Context, purchaseIDs []uuid.UUID) (map[uuid.UUID]domain.CreditLot, error) {
	out := make(map[uuid.UUID]domain.CreditLot, len(purchaseIDs))
	if len(purchaseIDs) == 0 {
		return out, nil
	}
	var rows []domain.CreditLot
	if err := database.Conn(ctx, r.db).Where("reference_type = ? AND reference_id IN ?", domain.LotRefCreditPurchase, purchaseIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, l := range rows {
		if l.ReferenceID != nil {
			out[*l.ReferenceID] = l
		}
	}
	return out, nil
}

func (r *GormLotRepository) UserLots(ctx context.Context, userID uuid.UUID, limit int) ([]domain.CreditLot, error) {
	var out []domain.CreditLot
	err := database.Conn(ctx, r.db).Where("user_id = ?", userID).Order("created_at DESC, id DESC").
		Limit(clampLimit(limit, 100, 500)).Find(&out).Error
	return out, err
}

// Reconcile: two index-backed statements per chunk of wallets (the chunk's
// upper bound, then the mismatches inside it), so no statement scans the lots
// table as a whole.
func (r *GormLotRepository) Reconcile(ctx context.Context, after uuid.UUID, size int) (LotChunk, error) {
	size = clampLimit(size, 5000, 50000)
	var bounds struct {
		Last    uuid.UUID
		Scanned int
	}
	// (max() has no uuid overload before PostgreSQL 18, so take the last row.)
	if err := r.bounds(ctx, after, size).Scan(&bounds).Error; err != nil {
		return LotChunk{}, err
	}
	out := LotChunk{Scanned: bounds.Scanned}
	if bounds.Scanned == 0 {
		return out, nil
	}
	out.Last = bounds.Last
	err := r.Mismatches(ctx, after, bounds.Last).Scan(&out.Mismatches).Error
	return out, err
}

// bounds selects the last user id of the next chunk of size wallets and the
// chunk's row count.
func (r *GormLotRepository) bounds(ctx context.Context, after uuid.UUID, size int) *gorm.DB {
	return database.Conn(ctx, r.db).Raw(`SELECT user_id AS last, count(*) OVER () AS scanned
		FROM (SELECT user_id FROM wallets WHERE user_id > ? ORDER BY user_id LIMIT ?) c
		ORDER BY user_id DESC LIMIT 1`, after, size)
}

// Mismatches builds the statement (one range scan of the lots of the chunk,
// hash-joined to its wallets) that lists the wallets in (after, last]
// whose balance differs from Σ credits_remaining (the caller scans it into
// []LotMismatch). Exposed so the load tests can EXPLAIN it.
func (r *GormLotRepository) Mismatches(ctx context.Context, after, last uuid.UUID) *gorm.DB {
	return database.Conn(ctx, r.db).Raw(`SELECT w.user_id, w.balance, COALESCE(s.rem, 0) AS lots_remaining
		FROM wallets w
		LEFT JOIN (SELECT user_id, SUM(credits_remaining)::bigint AS rem FROM credit_lots
			WHERE user_id > ? AND user_id <= ? AND credits_remaining > 0 GROUP BY user_id) s ON s.user_id = w.user_id
		WHERE w.user_id > ? AND w.user_id <= ? AND COALESCE(s.rem, 0) <> w.balance
		ORDER BY w.user_id`, after, last, after, last)
}
