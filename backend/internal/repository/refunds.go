package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 5.0: refund requests (§24.4).

// Refund request repository errors (one per unique index of migration 0028).
var (
	// ErrRefundReferenceTaken: the generated reference collided; try another.
	ErrRefundReferenceTaken = errors.New("repository: refund reference taken")
	// ErrRefundOpenExists: the payment already has an open request.
	ErrRefundOpenExists = errors.New("repository: payment already has an open refund request")
	// ErrRefundKeyTaken: the (user, idempotency key) pair was used.
	ErrRefundKeyTaken = errors.New("repository: refund idempotency key taken")
)

// Constraint names of refund_requests.
const (
	refundReferenceIndex = "refund_requests_reference_key"
	refundOpenIndex      = "refund_requests_open_transaction_key"
	refundKeyIndex       = "refund_requests_idempotency_key"
)

// RefundFilter selects requests for the admin list.
type RefundFilter struct {
	Status domain.RefundRequestStatus
	UserID *uuid.UUID
	Cursor *Cursor
	Limit  int
}

// RefundSummary is the part of a request a transactions page shows.
type RefundSummary struct {
	ID            uuid.UUID
	Reference     string
	TransactionID uuid.UUID
	Status        domain.RefundRequestStatus
}

// PackRef names the pack a purchase bought.
type PackRef struct {
	PurchaseID uuid.UUID
	PackID     uuid.UUID
	Name       string
}

// RefundRepository persists refund requests.
type RefundRepository interface {
	// Create inserts a request; the sentinel errors above name the index that
	// refused it.
	Create(ctx context.Context, r *domain.RefundRequest) error
	Get(ctx context.Context, id uuid.UUID) (*domain.RefundRequest, error)
	// GetForUpdate locks the row (inside a transaction): every state
	// transition starts here.
	GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.RefundRequest, error)
	Update(ctx context.Context, id uuid.UUID, fields map[string]any) error
	// ByKey returns the request a client already made with this key.
	ByKey(ctx context.Context, userID uuid.UUID, key string) (*domain.RefundRequest, error)
	// OpenFor returns the open request of a payment.
	OpenFor(ctx context.Context, transactionID uuid.UUID) (*domain.RefundRequest, error)
	// Latest returns, with ONE query, one request per payment: the open one,
	// else the newest.
	Latest(ctx context.Context, transactionIDs []uuid.UUID) (map[uuid.UUID]RefundSummary, error)
	ListForUser(ctx context.Context, userID uuid.UUID, c *Cursor, limit int) ([]domain.RefundRequest, error)
	List(ctx context.Context, f RefundFilter) ([]domain.RefundRequest, error)
	// Packs names the packs of purchases with ONE query.
	Packs(ctx context.Context, purchaseIDs []uuid.UUID) (map[uuid.UUID]PackRef, error)
}

// GormRefundRepository implements RefundRepository.
type GormRefundRepository struct{ db *gorm.DB }

// NewRefundRepository returns a GORM refund repository.
func NewRefundRepository(db *gorm.DB) *GormRefundRepository { return &GormRefundRepository{db: db} }

func (r *GormRefundRepository) conn(ctx context.Context) *gorm.DB { return database.Conn(ctx, r.db) }

func (r *GormRefundRepository) Create(ctx context.Context, rr *domain.RefundRequest) error {
	err := r.conn(ctx).Create(rr).Error
	switch {
	case err == nil:
		return nil
	case isUnique(err, refundReferenceIndex):
		return ErrRefundReferenceTaken
	case isUnique(err, refundOpenIndex):
		return ErrRefundOpenExists
	case isUnique(err, refundKeyIndex):
		return ErrRefundKeyTaken
	}
	return err
}

func (r *GormRefundRepository) Get(ctx context.Context, id uuid.UUID) (*domain.RefundRequest, error) {
	var out domain.RefundRequest
	if err := r.conn(ctx).First(&out, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &out, nil
}

func (r *GormRefundRepository) GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.RefundRequest, error) {
	var out domain.RefundRequest
	if err := r.conn(ctx).Clauses(forUpdate).First(&out, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &out, nil
}

func (r *GormRefundRepository) Update(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	res := r.conn(ctx).Model(&domain.RefundRequest{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormRefundRepository) ByKey(ctx context.Context, userID uuid.UUID, key string) (*domain.RefundRequest, error) {
	var out domain.RefundRequest
	if err := r.conn(ctx).First(&out, "user_id = ? AND idempotency_key = ?", userID, key).Error; err != nil {
		return nil, notFound(err)
	}
	return &out, nil
}

func (r *GormRefundRepository) OpenFor(ctx context.Context, transactionID uuid.UUID) (*domain.RefundRequest, error) {
	var out domain.RefundRequest
	if err := r.conn(ctx).First(&out, "transaction_id = ? AND status = ?", transactionID, domain.RefundOpen).Error; err != nil {
		return nil, notFound(err)
	}
	return &out, nil
}

func (r *GormRefundRepository) Latest(ctx context.Context, transactionIDs []uuid.UUID) (map[uuid.UUID]RefundSummary, error) {
	out := make(map[uuid.UUID]RefundSummary, len(transactionIDs))
	if len(transactionIDs) == 0 {
		return out, nil
	}
	var rows []RefundSummary
	err := r.conn(ctx).Model(&domain.RefundRequest{}).
		Select("DISTINCT ON (transaction_id) id, reference, transaction_id, status").
		Where("transaction_id IN ?", transactionIDs).
		Order("transaction_id, (status = 'open') DESC, created_at DESC, id DESC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, s := range rows {
		out[s.TransactionID] = s
	}
	return out, nil
}

func (r *GormRefundRepository) ListForUser(ctx context.Context, userID uuid.UUID, c *Cursor, limit int) ([]domain.RefundRequest, error) {
	var out []domain.RefundRequest
	q := applyCursor(r.conn(ctx).Where("user_id = ?", userID), c, "created_at", "id")
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(limit, 20, 100)).Find(&out).Error
	return out, err
}

func (r *GormRefundRepository) List(ctx context.Context, f RefundFilter) ([]domain.RefundRequest, error) {
	q := r.conn(ctx)
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.UserID != nil {
		q = q.Where("user_id = ?", *f.UserID)
	}
	q = applyCursor(q, f.Cursor, "created_at", "id")
	var out []domain.RefundRequest
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(f.Limit, 20, 100)).Find(&out).Error
	return out, err
}

func (r *GormRefundRepository) Packs(ctx context.Context, purchaseIDs []uuid.UUID) (map[uuid.UUID]PackRef, error) {
	out := make(map[uuid.UUID]PackRef, len(purchaseIDs))
	if len(purchaseIDs) == 0 {
		return out, nil
	}
	var rows []PackRef
	err := r.conn(ctx).Table("credit_purchases p").
		Select("p.id AS purchase_id, p.pack_id AS pack_id, k.name AS name").
		Joins("JOIN credit_packs k ON k.id = p.pack_id").
		Where("p.id IN ?", purchaseIDs).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, p := range rows {
		out[p.PurchaseID] = p
	}
	return out, nil
}

// PurgeForUser implements service.UserDataPurger (revisions 5.0, account
// erasure). The account is anonymised rather than deleted, and refund
// requests are financial records, so the rows stay (amounts, statuses,
// dates, the quote snapshot) while their free text — what the client wrote
// and the staff's resolution note, which may name them — is erased.
func (r *GormRefundRepository) PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	res := database.Conn(ctx, r.db).Model(&domain.RefundRequest{}).
		Where("user_id = ? AND (details <> '' OR resolution_note <> '')", userID).
		Updates(map[string]any{"details": "", "resolution_note": ""})
	return res.RowsAffected, res.Error
}
