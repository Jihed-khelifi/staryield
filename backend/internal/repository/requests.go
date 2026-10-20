package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// RequestFilter narrows custom request listings.
type RequestFilter struct {
	ClientID  *uuid.UUID
	PsychicID *uuid.UUID
	Statuses  []domain.CustomRequestStatus
	Cursor    *Cursor
	Limit     int
}

// CustomRequestRepository persists custom request orders.
type CustomRequestRepository interface {
	Create(ctx context.Context, r *domain.CustomRequest) error
	Get(ctx context.Context, id uuid.UUID) (*domain.CustomRequest, error)
	GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.CustomRequest, error)
	GetByKey(ctx context.Context, clientID uuid.UUID, key string) (*domain.CustomRequest, error)
	Save(ctx context.Context, r *domain.CustomRequest) error
	List(ctx context.Context, f RequestFilter) ([]domain.CustomRequest, error)
	ClaimExpiredPending(ctx context.Context, now time.Time, limit int) ([]domain.CustomRequest, error)
	ClaimOverdueDelivery(ctx context.Context, now time.Time, limit int) ([]domain.CustomRequest, error)
}

// GormCustomRequestRepository implements CustomRequestRepository.
type GormCustomRequestRepository struct{ db *gorm.DB }

// NewCustomRequestRepository returns a GORM custom request repository.
func NewCustomRequestRepository(db *gorm.DB) *GormCustomRequestRepository {
	return &GormCustomRequestRepository{db: db}
}

func (r *GormCustomRequestRepository) Create(ctx context.Context, cr *domain.CustomRequest) error {
	if err := database.Conn(ctx, r.db).Create(cr).Error; err != nil {
		if isUnique(err, "custom_requests_idempotency_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *GormCustomRequestRepository) Get(ctx context.Context, id uuid.UUID) (*domain.CustomRequest, error) {
	var cr domain.CustomRequest
	if err := database.Conn(ctx, r.db).First(&cr, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &cr, nil
}

func (r *GormCustomRequestRepository) GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.CustomRequest, error) {
	var cr domain.CustomRequest
	if err := database.Conn(ctx, r.db).Clauses(forUpdate).First(&cr, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &cr, nil
}

func (r *GormCustomRequestRepository) GetByKey(ctx context.Context, clientID uuid.UUID, key string) (*domain.CustomRequest, error) {
	var cr domain.CustomRequest
	if err := database.Conn(ctx, r.db).First(&cr, "client_id = ? AND idempotency_key = ?", clientID, key).Error; err != nil {
		return nil, notFound(err)
	}
	return &cr, nil
}

func (r *GormCustomRequestRepository) Save(ctx context.Context, cr *domain.CustomRequest) error {
	cr.UpdatedAt = time.Now().UTC()
	return database.Conn(ctx, r.db).Save(cr).Error
}

func (r *GormCustomRequestRepository) List(ctx context.Context, f RequestFilter) ([]domain.CustomRequest, error) {
	q := database.Conn(ctx, r.db).Model(&domain.CustomRequest{})
	if f.ClientID != nil {
		q = q.Where("client_id = ?", *f.ClientID)
	}
	if f.PsychicID != nil {
		q = q.Where("psychic_id = ?", *f.PsychicID)
	}
	if len(f.Statuses) > 0 {
		q = q.Where("status IN ?", f.Statuses)
	}
	q = applyCursor(q, f.Cursor, "created_at", "id")
	var out []domain.CustomRequest
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(f.Limit, 20, 100)).Find(&out).Error
	return out, err
}

func (r *GormCustomRequestRepository) ClaimExpiredPending(ctx context.Context, now time.Time, limit int) ([]domain.CustomRequest, error) {
	var out []domain.CustomRequest
	err := database.Conn(ctx, r.db).Clauses(forUpdateSkipLocked).
		Where("status = ? AND accept_by <= ?", domain.RequestPending, now).
		Limit(clampLimit(limit, 50, 500)).Find(&out).Error
	return out, err
}

func (r *GormCustomRequestRepository) ClaimOverdueDelivery(ctx context.Context, now time.Time, limit int) ([]domain.CustomRequest, error) {
	var out []domain.CustomRequest
	err := database.Conn(ctx, r.db).Clauses(forUpdateSkipLocked).
		Where("status = ? AND deliver_by <= ?", domain.RequestAccepted, now).
		Limit(clampLimit(limit, 50, 500)).Find(&out).Error
	return out, err
}

// EarningsSummary aggregates a psychic's earnings.
type EarningsSummary struct {
	GrossCredits      int64 `json:"gross_credits"`
	CommissionCredits int64 `json:"commission_credits"`
	NetCredits        int64 `json:"net_credits"`
	Sessions          int64 `json:"sessions"`
	CustomRequests    int64 `json:"custom_requests"`
}

// EarningRepository persists immutable earning records.
type EarningRepository interface {
	Create(ctx context.Context, e *domain.Earning) error
	List(ctx context.Context, psychicID uuid.UUID, from, to *time.Time, cursor *Cursor, limit int) ([]domain.Earning, error)
	Summary(ctx context.Context, psychicID uuid.UUID, from, to *time.Time) (EarningsSummary, error)
}

// GormEarningRepository implements EarningRepository.
type GormEarningRepository struct{ db *gorm.DB }

// NewEarningRepository returns a GORM earning repository.
func NewEarningRepository(db *gorm.DB) *GormEarningRepository { return &GormEarningRepository{db: db} }

func (r *GormEarningRepository) Create(ctx context.Context, e *domain.Earning) error {
	if err := database.Conn(ctx, r.db).Create(e).Error; err != nil {
		if isUnique(err, "psychic_earnings_source_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *GormEarningRepository) scope(q *gorm.DB, psychicID uuid.UUID, from, to *time.Time) *gorm.DB {
	q = q.Where("psychic_id = ?", psychicID)
	if from != nil {
		q = q.Where("created_at >= ?", *from)
	}
	if to != nil {
		q = q.Where("created_at < ?", *to)
	}
	return q
}

func (r *GormEarningRepository) List(ctx context.Context, psychicID uuid.UUID, from, to *time.Time, cursor *Cursor, limit int) ([]domain.Earning, error) {
	q := r.scope(database.Conn(ctx, r.db).Model(&domain.Earning{}), psychicID, from, to)
	q = applyCursor(q, cursor, "created_at", "id")
	var out []domain.Earning
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(limit, 50, 200)).Find(&out).Error
	return out, err
}

func (r *GormEarningRepository) Summary(ctx context.Context, psychicID uuid.UUID, from, to *time.Time) (EarningsSummary, error) {
	var s EarningsSummary
	q := r.scope(database.Conn(ctx, r.db).Model(&domain.Earning{}), psychicID, from, to)
	err := q.Select(`COALESCE(SUM(gross_credits),0) AS gross_credits,
		COALESCE(SUM(commission_credits),0) AS commission_credits,
		COALESCE(SUM(net_credits),0) AS net_credits,
		COUNT(*) FILTER (WHERE source_type = 'chat_session' AND entry_kind = 'earning') AS sessions,
		COUNT(*) FILTER (WHERE source_type = 'custom_request' AND entry_kind = 'earning') AS custom_requests`).
		Scan(&s).Error
	return s, err
}

// AuditRepository persists the admin audit trail.
type AuditRepository interface {
	Create(ctx context.Context, a *domain.AuditLog) error
	List(ctx context.Context, actorID *uuid.UUID, targetType, targetID string, cursor *Cursor, limit int) ([]domain.AuditLog, error)
}

// GormAuditRepository implements AuditRepository.
type GormAuditRepository struct{ db *gorm.DB }

// NewAuditRepository returns a GORM audit repository.
func NewAuditRepository(db *gorm.DB) *GormAuditRepository { return &GormAuditRepository{db: db} }

func (r *GormAuditRepository) Create(ctx context.Context, a *domain.AuditLog) error {
	return database.Conn(ctx, r.db).Create(a).Error
}

func (r *GormAuditRepository) List(ctx context.Context, actorID *uuid.UUID, targetType, targetID string, cursor *Cursor, limit int) ([]domain.AuditLog, error) {
	q := database.Conn(ctx, r.db).Model(&domain.AuditLog{})
	if actorID != nil {
		q = q.Where("actor_id = ?", *actorID)
	}
	if targetType != "" {
		q = q.Where("target_type = ?", targetType)
	}
	if targetID != "" {
		q = q.Where("target_id = ?", targetID)
	}
	q = applyCursor(q, cursor, "created_at", "id")
	var out []domain.AuditLog
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(limit, 50, 200)).Find(&out).Error
	return out, err
}
