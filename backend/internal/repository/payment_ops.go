package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 6.0 (docs/platform-extensions.md §26.5, migration 0033): the
// webhook log, disputes, early fraud warnings and the lookups card-payment
// webhooks need.

// WebhookFilter narrows the webhook log.
type WebhookFilter struct {
	Provider     string
	Status       domain.WebhookStatus
	ProviderType string // matches the normalised type or the provider's own
	Cursor       *Cursor
	Limit        int
}

// DisputeFilter narrows dispute listings.
type DisputeFilter struct {
	Status domain.DisputeStatus
	UserID *uuid.UUID
	Cursor *Cursor
	Limit  int
}

// FraudWarningFilter narrows fraud warning listings.
type FraudWarningFilter struct {
	Action domain.FraudAction
	Cursor *Cursor
	Limit  int
}

// WebhookCounts summarise the webhook log of a provider since a time.
type WebhookCounts struct {
	Received int64
	Failed   int64
	LastAt   *time.Time
}

// UsageRow is one reading of a client (the evidence usage log): never the
// conversation, only when, with whom, how long and what it cost.
type UsageRow struct {
	StartedAt      time.Time
	Advisor        string
	BilledMinutes  int
	FreeMinutes    int
	CreditsCharged int64
}

// PaymentOpsRepository persists disputes, fraud warnings and the webhook log.
type PaymentOpsRepository interface {
	// Webhook log. Receipts themselves are written through BillingRepository.
	GetWebhookByID(ctx context.Context, id uuid.UUID) (*domain.ProviderWebhookEvent, error)
	// UpdateWebhook sets status, error and processed_at of a receipt and
	// counts the attempt (attempts + 1).
	UpdateWebhook(ctx context.Context, id uuid.UUID, status domain.WebhookStatus, errText string, processedAt *time.Time, at time.Time) error
	ListWebhooks(ctx context.Context, f WebhookFilter) ([]domain.ProviderWebhookEvent, error)
	WebhookCounts(ctx context.Context, provider string, since time.Time) (WebhookCounts, error)
	LastWebhook(ctx context.Context, provider string) (*domain.ProviderWebhookEvent, error)

	// Disputes.
	CreateDispute(ctx context.Context, d *domain.PaymentDispute) error
	GetDispute(ctx context.Context, id uuid.UUID) (*domain.PaymentDispute, error)
	GetDisputeForUpdate(ctx context.Context, id uuid.UUID) (*domain.PaymentDispute, error)
	GetDisputeByRef(ctx context.Context, provider, ref string) (*domain.PaymentDispute, error)
	UpdateDispute(ctx context.Context, id uuid.UUID, fields map[string]any) error
	ListDisputes(ctx context.Context, f DisputeFilter) ([]domain.PaymentDispute, error)
	// DisputesByTransactions returns the latest dispute of each payment (one query).
	DisputesByTransactions(ctx context.Context, transactionIDs []uuid.UUID) (map[uuid.UUID]domain.PaymentDispute, error)
	// HasOpenDispute reports whether the user has a chargeback in progress.
	HasOpenDispute(ctx context.Context, userID uuid.UUID) (bool, error)

	// Fraud warnings.
	CreateFraudWarning(ctx context.Context, w *domain.PaymentFraudWarning) error
	GetFraudWarning(ctx context.Context, id uuid.UUID) (*domain.PaymentFraudWarning, error)
	GetFraudWarningForUpdate(ctx context.Context, id uuid.UUID) (*domain.PaymentFraudWarning, error)
	GetFraudWarningByRef(ctx context.Context, provider, ref string) (*domain.PaymentFraudWarning, error)
	UpdateFraudWarning(ctx context.Context, id uuid.UUID, fields map[string]any) error
	ListFraudWarnings(ctx context.Context, f FraudWarningFilter) ([]domain.PaymentFraudWarning, error)

	// Saved cards and customers by the hash of the provider reference.
	MethodsByRefHash(ctx context.Context, provider, hash string) ([]domain.PaymentMethod, error)
	CustomersByRefHash(ctx context.Context, provider, hash string) ([]domain.PaymentCustomer, error)

	// UsageLog lists a client's readings that started at or after since,
	// oldest first (evidence for a dispute).
	UsageLog(ctx context.Context, userID uuid.UUID, since time.Time, limit int) ([]UsageRow, error)
}

// GormPaymentOpsRepository implements PaymentOpsRepository.
type GormPaymentOpsRepository struct{ db *gorm.DB }

// NewPaymentOpsRepository returns a GORM payment operations repository.
func NewPaymentOpsRepository(db *gorm.DB) *GormPaymentOpsRepository {
	return &GormPaymentOpsRepository{db: db}
}

func (r *GormPaymentOpsRepository) conn(ctx context.Context) *gorm.DB {
	return database.Conn(ctx, r.db)
}

// --------------------------------------------------------------- webhooks

func (r *GormPaymentOpsRepository) GetWebhookByID(ctx context.Context, id uuid.UUID) (*domain.ProviderWebhookEvent, error) {
	var e domain.ProviderWebhookEvent
	if err := r.conn(ctx).First(&e, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &e, nil
}

func (r *GormPaymentOpsRepository) UpdateWebhook(ctx context.Context, id uuid.UUID, status domain.WebhookStatus, errText string,
	processedAt *time.Time, at time.Time) error {
	return r.conn(ctx).Model(&domain.ProviderWebhookEvent{}).Where("id = ?", id).Updates(map[string]any{
		"status": status, "error": errText, "processed_at": processedAt, "attempts": gorm.Expr("attempts + 1"), "updated_at": at}).Error
}

func (r *GormPaymentOpsRepository) ListWebhooks(ctx context.Context, f WebhookFilter) ([]domain.ProviderWebhookEvent, error) {
	q := r.conn(ctx)
	if f.Provider != "" {
		q = q.Where("provider = ?", f.Provider)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.ProviderType != "" {
		q = q.Where("(event_type = ? OR provider_type = ?)", f.ProviderType, f.ProviderType)
	}
	q = applyCursor(q, f.Cursor, "created_at", "id")
	var out []domain.ProviderWebhookEvent
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(f.Limit, 20, 100)).Find(&out).Error
	return out, err
}

func (r *GormPaymentOpsRepository) WebhookCounts(ctx context.Context, provider string, since time.Time) (WebhookCounts, error) {
	var row struct {
		Received int64
		Failed   int64
		LastAt   *time.Time
	}
	err := r.conn(ctx).Model(&domain.ProviderWebhookEvent{}).
		Select("count(*) AS received, count(*) FILTER (WHERE status = 'failed') AS failed, max(created_at) AS last_at").
		Where("provider = ? AND created_at >= ?", provider, since).Scan(&row).Error
	return WebhookCounts{Received: row.Received, Failed: row.Failed, LastAt: row.LastAt}, err
}

func (r *GormPaymentOpsRepository) LastWebhook(ctx context.Context, provider string) (*domain.ProviderWebhookEvent, error) {
	var e domain.ProviderWebhookEvent
	if err := r.conn(ctx).Where("provider = ?", provider).Order("created_at DESC, id DESC").First(&e).Error; err != nil {
		return nil, notFound(err)
	}
	return &e, nil
}

// --------------------------------------------------------------- disputes

func (r *GormPaymentOpsRepository) CreateDispute(ctx context.Context, d *domain.PaymentDispute) error {
	if err := r.conn(ctx).Create(d).Error; err != nil {
		if isUnique(err, "payment_disputes_ref_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *GormPaymentOpsRepository) GetDispute(ctx context.Context, id uuid.UUID) (*domain.PaymentDispute, error) {
	var d domain.PaymentDispute
	if err := r.conn(ctx).First(&d, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &d, nil
}

func (r *GormPaymentOpsRepository) GetDisputeForUpdate(ctx context.Context, id uuid.UUID) (*domain.PaymentDispute, error) {
	var d domain.PaymentDispute
	if err := r.conn(ctx).Clauses(forUpdate).First(&d, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &d, nil
}

func (r *GormPaymentOpsRepository) GetDisputeByRef(ctx context.Context, provider, ref string) (*domain.PaymentDispute, error) {
	var d domain.PaymentDispute
	if err := r.conn(ctx).First(&d, "provider = ? AND dispute_ref = ?", provider, ref).Error; err != nil {
		return nil, notFound(err)
	}
	return &d, nil
}

func (r *GormPaymentOpsRepository) UpdateDispute(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	return r.conn(ctx).Model(&domain.PaymentDispute{}).Where("id = ?", id).Updates(fields).Error
}

func (r *GormPaymentOpsRepository) ListDisputes(ctx context.Context, f DisputeFilter) ([]domain.PaymentDispute, error) {
	q := r.conn(ctx)
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.UserID != nil {
		q = q.Where("user_id = ?", *f.UserID)
	}
	q = applyCursor(q, f.Cursor, "created_at", "id")
	var out []domain.PaymentDispute
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(f.Limit, 20, 100)).Find(&out).Error
	return out, err
}

func (r *GormPaymentOpsRepository) DisputesByTransactions(ctx context.Context, transactionIDs []uuid.UUID) (map[uuid.UUID]domain.PaymentDispute, error) {
	out := map[uuid.UUID]domain.PaymentDispute{}
	if len(transactionIDs) == 0 {
		return out, nil
	}
	var rows []domain.PaymentDispute
	if err := r.conn(ctx).Where("transaction_id IN ?", transactionIDs).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, d := range rows { // oldest first, so the latest dispute wins
		out[d.TransactionID] = d
	}
	return out, nil
}

func (r *GormPaymentOpsRepository) HasOpenDispute(ctx context.Context, userID uuid.UUID) (bool, error) {
	var ids []uuid.UUID
	err := r.conn(ctx).Model(&domain.PaymentDispute{}).
		Where("user_id = ? AND status IN ?", userID, domain.OpenDisputeStatuses).Limit(1).Pluck("id", &ids).Error
	return len(ids) > 0, err
}

// ----------------------------------------------------------- fraud warnings

func (r *GormPaymentOpsRepository) CreateFraudWarning(ctx context.Context, w *domain.PaymentFraudWarning) error {
	if err := r.conn(ctx).Create(w).Error; err != nil {
		if isUnique(err, "payment_fraud_warnings_ref_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *GormPaymentOpsRepository) GetFraudWarning(ctx context.Context, id uuid.UUID) (*domain.PaymentFraudWarning, error) {
	var w domain.PaymentFraudWarning
	if err := r.conn(ctx).First(&w, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &w, nil
}

func (r *GormPaymentOpsRepository) GetFraudWarningForUpdate(ctx context.Context, id uuid.UUID) (*domain.PaymentFraudWarning, error) {
	var w domain.PaymentFraudWarning
	if err := r.conn(ctx).Clauses(forUpdate).First(&w, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &w, nil
}

func (r *GormPaymentOpsRepository) GetFraudWarningByRef(ctx context.Context, provider, ref string) (*domain.PaymentFraudWarning, error) {
	var w domain.PaymentFraudWarning
	if err := r.conn(ctx).First(&w, "provider = ? AND warning_ref = ?", provider, ref).Error; err != nil {
		return nil, notFound(err)
	}
	return &w, nil
}

func (r *GormPaymentOpsRepository) UpdateFraudWarning(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	return r.conn(ctx).Model(&domain.PaymentFraudWarning{}).Where("id = ?", id).Updates(fields).Error
}

func (r *GormPaymentOpsRepository) ListFraudWarnings(ctx context.Context, f FraudWarningFilter) ([]domain.PaymentFraudWarning, error) {
	q := r.conn(ctx)
	if f.Action != "" {
		q = q.Where("action = ?", f.Action)
	}
	q = applyCursor(q, f.Cursor, "created_at", "id")
	var out []domain.PaymentFraudWarning
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(f.Limit, 20, 100)).Find(&out).Error
	return out, err
}

// ------------------------------------------------------ methods, customers

func (r *GormPaymentOpsRepository) MethodsByRefHash(ctx context.Context, provider, hash string) ([]domain.PaymentMethod, error) {
	var out []domain.PaymentMethod
	if hash == "" {
		return out, nil
	}
	err := r.conn(ctx).Where("provider = ? AND provider_method_ref_hash = ?", provider, hash).Limit(5).Find(&out).Error
	return out, err
}

func (r *GormPaymentOpsRepository) CustomersByRefHash(ctx context.Context, provider, hash string) ([]domain.PaymentCustomer, error) {
	var out []domain.PaymentCustomer
	if hash == "" {
		return out, nil
	}
	err := r.conn(ctx).Where("provider = ? AND provider_customer_ref_hash = ?", provider, hash).Limit(5).Find(&out).Error
	return out, err
}

// ----------------------------------------------------------------- usage

func (r *GormPaymentOpsRepository) UsageLog(ctx context.Context, userID uuid.UUID, since time.Time, limit int) ([]UsageRow, error) {
	var out []UsageRow
	err := r.conn(ctx).Table("chat_sessions AS s").
		Select("s.started_at AS started_at, p.name AS advisor, s.billed_minutes AS billed_minutes, "+
			"s.free_minutes_used AS free_minutes, s.credits_charged AS credits_charged").
		Joins("JOIN psychics p ON p.user_id = s.psychic_id").
		Where("s.client_id = ? AND s.started_at IS NOT NULL AND s.started_at >= ?", userID, since).
		Order("s.started_at ASC").Limit(clampLimit(limit, 50, 200)).Scan(&out).Error
	return out, err
}
