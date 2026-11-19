package repository

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 6.0 (§26.6): invoices and credit notes.

// InvoiceFilter narrows the admin invoice list and the accounting export.
type InvoiceFilter struct {
	UserID *uuid.UUID
	Kind   domain.InvoiceKind
	// NumberPrefix matches the start of the number ("SY-2026-0001").
	NumberPrefix string
	// From (inclusive) and To (exclusive) bound issued_at.
	From, To *time.Time
	Cursor   *Cursor
	Limit    int
}

// settledPurchaseStatuses are the payment states in which a purchase has been
// paid (and may since have been refunded or disputed): they all deserve an
// invoice. The partial index payment_transactions_invoicing_idx (migration
// 0034) is defined on exactly this list.
var settledPurchaseStatuses = []domain.TransactionStatus{domain.TxSucceeded, domain.TxPartiallyRefunded, domain.TxRefunded, domain.TxDisputed}

// InvoiceRepository persists invoices and credit notes.
type InvoiceRepository interface {
	// NextNumber takes the next sequence value of the scope. It must run in the
	// transaction that inserts the document (the counter row stays locked until
	// then, which is what makes the numbers gapless).
	NextNumber(ctx context.Context, scope string, now time.Time) (int64, error)
	// Create inserts a document; ErrDuplicate when the transaction already has
	// one of that kind (or the number exists).
	Create(ctx context.Context, inv *domain.Invoice) error
	Get(ctx context.Context, id uuid.UUID) (*domain.Invoice, error)
	GetByTransaction(ctx context.Context, transactionID uuid.UUID, kind domain.InvoiceKind) (*domain.Invoice, error)
	List(ctx context.Context, f InvoiceFilter) ([]domain.Invoice, error)
	// RefsByTransactions returns the documents (id, number, kind, transaction)
	// of a page of payment transactions with one query: the invoice of a
	// purchase and the credit note of a refund both point at their transaction.
	RefsByTransactions(ctx context.Context, transactionIDs []uuid.UUID) ([]domain.Invoice, error)
	// InvoicesByPurchases returns the invoice of each purchase (one query).
	InvoicesByPurchases(ctx context.Context, purchaseIDs []uuid.UUID) ([]domain.Invoice, error)
	// PurchasesWithoutInvoice returns the credit purchases paid in [from, to)
	// (by the transaction's last update) that have no invoice yet, oldest first.
	PurchasesWithoutInvoice(ctx context.Context, from, to time.Time, limit int) ([]uuid.UUID, error)
	// RefundsWithoutCreditNote returns the booked refunds of credit purchases
	// settled in [from, to) that have no credit note yet, oldest first.
	RefundsWithoutCreditNote(ctx context.Context, from, to time.Time, limit int) ([]uuid.UUID, error)
}

// GormInvoiceRepository implements InvoiceRepository.
type GormInvoiceRepository struct{ db *gorm.DB }

// NewInvoiceRepository returns a GORM invoice repository.
func NewInvoiceRepository(db *gorm.DB) *GormInvoiceRepository { return &GormInvoiceRepository{db: db} }

func (r *GormInvoiceRepository) conn(ctx context.Context) *gorm.DB { return database.Conn(ctx, r.db) }

// NextNumber implements InvoiceRepository: one atomic upsert that creates the
// counter (taking number 1) or increments it, and returns the value taken.
func (r *GormInvoiceRepository) NextNumber(ctx context.Context, scope string, now time.Time) (int64, error) {
	seq := domain.InvoiceSequence{Scope: scope, Next: 2, CreatedAt: now, UpdatedAt: now}
	err := r.conn(ctx).Clauses(
		clause.OnConflict{
			Columns: []clause.Column{{Name: "scope"}},
			DoUpdates: clause.Assignments(map[string]any{
				"next":       gorm.Expr("invoice_sequences.next + 1"),
				"updated_at": now,
			}),
		},
		clause.Returning{Columns: []clause.Column{{Name: "next"}}},
	).Create(&seq).Error
	if err != nil {
		return 0, err
	}
	return seq.Next - 1, nil
}

// Create implements InvoiceRepository.
func (r *GormInvoiceRepository) Create(ctx context.Context, inv *domain.Invoice) error {
	if err := r.conn(ctx).Create(inv).Error; err != nil {
		if isUnique(err, "invoices_transaction_kind_key", "invoices_number_key", "invoices_pkey") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

// Get implements InvoiceRepository.
func (r *GormInvoiceRepository) Get(ctx context.Context, id uuid.UUID) (*domain.Invoice, error) {
	var inv domain.Invoice
	if err := r.conn(ctx).First(&inv, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &inv, nil
}

// GetByTransaction implements InvoiceRepository.
func (r *GormInvoiceRepository) GetByTransaction(ctx context.Context, transactionID uuid.UUID, kind domain.InvoiceKind) (*domain.Invoice, error) {
	var inv domain.Invoice
	if err := r.conn(ctx).First(&inv, "payment_transaction_id = ? AND kind = ?", transactionID, kind).Error; err != nil {
		return nil, notFound(err)
	}
	return &inv, nil
}

// List implements InvoiceRepository: newest first by issue time (keyset).
func (r *GormInvoiceRepository) List(ctx context.Context, f InvoiceFilter) ([]domain.Invoice, error) {
	q := r.conn(ctx)
	if f.UserID != nil {
		q = q.Where("user_id = ?", *f.UserID)
	}
	if f.Kind != "" {
		q = q.Where("kind = ?", f.Kind)
	}
	if p := strings.TrimSpace(f.NumberPrefix); p != "" {
		q = q.Where(`number LIKE ? ESCAPE '\'`, likePrefix(p))
	}
	if f.From != nil {
		q = q.Where("issued_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("issued_at < ?", *f.To)
	}
	q = applyCursor(q, f.Cursor, "issued_at", "id")
	var out []domain.Invoice
	err := q.Order("issued_at DESC, id DESC").Limit(clampLimit(f.Limit, 20, 100)).Find(&out).Error
	return out, err
}

// likePrefix escapes LIKE wildcards and appends the prefix wildcard.
func likePrefix(s string) string {
	s = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
	return s + "%"
}

// RefsByTransactions implements InvoiceRepository.
func (r *GormInvoiceRepository) RefsByTransactions(ctx context.Context, transactionIDs []uuid.UUID) ([]domain.Invoice, error) {
	var out []domain.Invoice
	if len(transactionIDs) == 0 {
		return out, nil
	}
	err := r.conn(ctx).Select("id", "number", "kind", "payment_transaction_id", "credit_purchase_id").
		Where("payment_transaction_id IN ?", transactionIDs).Find(&out).Error
	return out, err
}

// InvoicesByPurchases implements InvoiceRepository.
func (r *GormInvoiceRepository) InvoicesByPurchases(ctx context.Context, purchaseIDs []uuid.UUID) ([]domain.Invoice, error) {
	var out []domain.Invoice
	if len(purchaseIDs) == 0 {
		return out, nil
	}
	err := r.conn(ctx).Select("id", "number", "kind", "payment_transaction_id", "credit_purchase_id").
		Where("credit_purchase_id IN ? AND kind = ?", purchaseIDs, domain.InvoiceKindInvoice).Find(&out).Error
	return out, err
}

// PurchasesWithoutInvoice implements InvoiceRepository.
func (r *GormInvoiceRepository) PurchasesWithoutInvoice(ctx context.Context, from, to time.Time, limit int) ([]uuid.UUID, error) {
	has := r.conn(ctx).Table("invoices i").Select("1").Where("i.payment_transaction_id = t.id AND i.kind = ?", domain.InvoiceKindInvoice)
	var ids []uuid.UUID
	err := r.conn(ctx).Table("payment_transactions t").
		Where("t.kind = ? AND t.status IN ? AND t.credit_purchase_id IS NOT NULL", domain.TxCreditPurchase, settledPurchaseStatuses).
		Where("t.updated_at >= ? AND t.updated_at < ?", from, to).
		Where("NOT EXISTS (?)", has).
		Order("t.updated_at ASC, t.id ASC").Limit(clampLimit(limit, 50, 500)).
		Pluck("t.credit_purchase_id", &ids).Error
	return ids, err
}

// RefundsWithoutCreditNote implements InvoiceRepository.
func (r *GormInvoiceRepository) RefundsWithoutCreditNote(ctx context.Context, from, to time.Time, limit int) ([]uuid.UUID, error) {
	has := r.conn(ctx).Table("invoices i").Select("1").Where("i.payment_transaction_id = t.id AND i.kind = ?", domain.InvoiceKindCreditNote)
	var ids []uuid.UUID
	err := r.conn(ctx).Table("payment_transactions t").
		Joins("JOIN payment_transactions p ON p.id = t.parent_transaction_id AND p.kind = ?", domain.TxCreditPurchase).
		Where("t.kind = ? AND t.status IN ?", domain.TxRefund, settledPurchaseStatuses).
		Where("t.updated_at >= ? AND t.updated_at < ?", from, to).
		Where("NOT EXISTS (?)", has).
		Order("t.updated_at ASC, t.id ASC").Limit(clampLimit(limit, 50, 500)).
		Pluck("t.id", &ids).Error
	return ids, err
}
