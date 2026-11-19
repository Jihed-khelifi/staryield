package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 6.0 (§26.4): e-mail leads of the calculators' report.

// ErrLeadExists is returned by Create when the address already has a lead
// (a concurrent request created it first).
var ErrLeadExists = errors.New("repository: e-mail lead exists")

// LeadFilter selects leads for the admin list.
type LeadFilter struct {
	Status domain.MarketingStatus
	Kind   domain.ReportKind
	// EmailHash matches one address exactly (the admin's q).
	EmailHash string
	Cursor    *Cursor
	Limit     int
}

// EmailLeadRepository persists the e-mail leads of the calculators' report.
type EmailLeadRepository interface {
	// ForUpdateByHash locks and returns the lead of an address.
	ForUpdateByHash(ctx context.Context, hash string) (*domain.EmailLead, error)
	// Create inserts a lead; ErrLeadExists when the address already has one.
	Create(ctx context.Context, l *domain.EmailLead) error
	// Save writes the mutable columns of a lead (never the address).
	Save(ctx context.Context, l *domain.EmailLead) error
	// ForUpdate locks and returns a lead by id.
	ForUpdate(ctx context.Context, id uuid.UUID) (*domain.EmailLead, error)
	// Get returns a lead by id.
	Get(ctx context.Context, id uuid.UUID) (*domain.EmailLead, error)
	// List returns one page, newest first.
	List(ctx context.Context, f LeadFilter) ([]domain.EmailLead, error)
	// Delete removes a lead; ErrNotFound when it does not exist.
	Delete(ctx context.Context, id uuid.UUID) error
	// LinkUser sets user_id on the unlinked lead of an address.
	LinkUser(ctx context.Context, hash string, userID uuid.UUID) (int64, error)
	// PurgeForUser deletes the leads linked to an account (account erasure).
	PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error)
}

// GormEmailLeadRepository implements EmailLeadRepository.
type GormEmailLeadRepository struct{ db *gorm.DB }

// NewEmailLeadRepository returns a GORM e-mail lead repository.
func NewEmailLeadRepository(db *gorm.DB) *GormEmailLeadRepository {
	return &GormEmailLeadRepository{db: db}
}

func (r *GormEmailLeadRepository) conn(ctx context.Context) *gorm.DB { return database.Conn(ctx, r.db) }

func (r *GormEmailLeadRepository) ForUpdateByHash(ctx context.Context, hash string) (*domain.EmailLead, error) {
	var l domain.EmailLead
	if err := r.conn(ctx).Clauses(forUpdate).First(&l, "email_hash = ?", hash).Error; err != nil {
		return nil, notFound(err)
	}
	return &l, nil
}

// Create inserts a lead. A concurrent request may create the address's lead
// first: ON CONFLICT DO NOTHING (never a unique violation, which would abort the
// caller's transaction) reports it as ErrLeadExists so the caller can update it.
func (r *GormEmailLeadRepository) Create(ctx context.Context, l *domain.EmailLead) error {
	res := r.conn(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "email_hash"}}, DoNothing: true}).Create(l)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrLeadExists
	}
	return nil
}

func (r *GormEmailLeadRepository) Save(ctx context.Context, l *domain.EmailLead) error {
	return r.conn(ctx).Model(&domain.EmailLead{}).Where("id = ?", l.ID).Updates(map[string]any{
		"locale": l.Locale, "country": l.Country, "kinds": l.Kinds, "reports_sent": l.ReportsSent,
		"marketing_status": l.MarketingStatus, "consent_at": l.ConsentAt, "consent_prechecked": l.ConsentPrechecked,
		"consent_text_version": l.ConsentTextVersion, "consent_ip_hash": l.ConsentIPHash, "consent_user_agent": l.ConsentUserAgent,
		"unsubscribed_at": l.UnsubscribedAt, "user_id": l.UserID, "last_request_at": l.LastRequestAt,
	}).Error
}

func (r *GormEmailLeadRepository) ForUpdate(ctx context.Context, id uuid.UUID) (*domain.EmailLead, error) {
	var l domain.EmailLead
	if err := r.conn(ctx).Clauses(forUpdate).First(&l, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &l, nil
}

func (r *GormEmailLeadRepository) Get(ctx context.Context, id uuid.UUID) (*domain.EmailLead, error) {
	var l domain.EmailLead
	if err := r.conn(ctx).First(&l, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &l, nil
}

func (r *GormEmailLeadRepository) List(ctx context.Context, f LeadFilter) ([]domain.EmailLead, error) {
	q := r.conn(ctx).Model(&domain.EmailLead{})
	if f.Status != "" {
		q = q.Where("marketing_status = ?", f.Status)
	}
	if f.Kind != "" {
		q = q.Where("kinds @> ?::jsonb", `["`+string(f.Kind)+`"]`)
	}
	if f.EmailHash != "" {
		q = q.Where("email_hash = ?", f.EmailHash)
	}
	q = applyCursor(q, f.Cursor, "created_at", "id")
	var out []domain.EmailLead
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(f.Limit, 25, 100)).Find(&out).Error
	return out, err
}

func (r *GormEmailLeadRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.conn(ctx).Where("id = ?", id).Delete(&domain.EmailLead{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormEmailLeadRepository) LinkUser(ctx context.Context, hash string, userID uuid.UUID) (int64, error) {
	res := r.conn(ctx).Model(&domain.EmailLead{}).Where("email_hash = ? AND user_id IS NULL", hash).Update("user_id", userID)
	return res.RowsAffected, res.Error
}

func (r *GormEmailLeadRepository) PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	res := r.conn(ctx).Where("user_id = ?", userID).Delete(&domain.EmailLead{})
	return res.RowsAffected, res.Error
}

var _ EmailLeadRepository = (*GormEmailLeadRepository)(nil)
