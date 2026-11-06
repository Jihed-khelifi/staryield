package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// EmailRepository persists the e-mail outbox and the suppression list.
type EmailRepository interface {
	// Enqueue inserts outbox rows; rows whose dedupe key already exists are
	// skipped (idempotent enqueue).
	Enqueue(ctx context.Context, rows []domain.EmailOutbox) error
	// CountRecent counts e-mails created for a user since a time, excluding
	// one category (the security category is exempt from the daily cap) and
	// rows that were never going to be sent.
	CountRecent(ctx context.Context, userID uuid.UUID, since time.Time, exclude domain.EmailCategory) (int64, error)
	// Suppressed returns which of the address hashes are currently
	// suppressed.
	Suppressed(ctx context.Context, hashes []string, now time.Time) (map[string]bool, error)
	// Suppress records (or refreshes) a suppression.
	Suppress(ctx context.Context, s *domain.EmailSuppression) error
	// Claim leases due rows for delivery (queued and due, or sending with an
	// expired lease) with FOR UPDATE SKIP LOCKED; call inside a transaction.
	Claim(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.EmailOutbox, error)
	// Finish updates a claimed row.
	Finish(ctx context.Context, id uuid.UUID, fields map[string]any) error
	// PurgeFinished deletes final rows last changed before a time.
	PurgeFinished(ctx context.Context, before time.Time) (int64, error)
	Get(ctx context.Context, id uuid.UUID) (*domain.EmailOutbox, error)
	// List returns outbox rows (newest first) for a user or template, for
	// support tooling and tests.
	List(ctx context.Context, userID *uuid.UUID, template string, limit int) ([]domain.EmailOutbox, error)
}

// GormEmailRepository implements EmailRepository.
type GormEmailRepository struct{ db *gorm.DB }

// NewEmailRepository returns a GORM e-mail repository.
func NewEmailRepository(db *gorm.DB) *GormEmailRepository { return &GormEmailRepository{db: db} }

var finalEmailStatuses = []domain.EmailStatus{domain.EmailSent, domain.EmailFailed, domain.EmailDead, domain.EmailSuppressed, domain.EmailCancelled}

func (r *GormEmailRepository) Enqueue(ctx context.Context, rows []domain.EmailOutbox) error {
	if len(rows) == 0 {
		return nil
	}
	// The conflict target must repeat the partial index's predicate as a
	// literal: with a bind parameter Postgres cannot match the index once a
	// cached statement runs on a generic plan (42P10).
	return database.Conn(ctx, r.db).Clauses(clause.OnConflict{
		Columns:     []clause.Column{{Name: "dedupe_key"}},
		TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "dedupe_key <> ''"}}},
		DoNothing:   true,
	}).CreateInBatches(rows, 500).Error
}

func (r *GormEmailRepository) CountRecent(ctx context.Context, userID uuid.UUID, since time.Time, exclude domain.EmailCategory) (int64, error) {
	var n int64
	err := database.Conn(ctx, r.db).Model(&domain.EmailOutbox{}).
		Where("user_id = ? AND created_at >= ? AND category <> ?", userID, since, exclude).
		Where("status NOT IN ?", []domain.EmailStatus{domain.EmailCancelled, domain.EmailSuppressed}).
		Count(&n).Error
	return n, err
}

func (r *GormEmailRepository) Suppressed(ctx context.Context, hashes []string, now time.Time) (map[string]bool, error) {
	out := map[string]bool{}
	if len(hashes) == 0 {
		return out, nil
	}
	var found []string
	err := database.Conn(ctx, r.db).Model(&domain.EmailSuppression{}).
		Where("email_hash IN ? AND (expires_at IS NULL OR expires_at > ?)", hashes, now).
		Pluck("email_hash", &found).Error
	for _, h := range found {
		out[h] = true
	}
	return out, err
}

func (r *GormEmailRepository) Suppress(ctx context.Context, s *domain.EmailSuppression) error {
	return database.Conn(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "email_hash"}},
		DoUpdates: clause.AssignmentColumns([]string{"reason", "source", "expires_at", "updated_at"}),
	}).Create(s).Error
}

func (r *GormEmailRepository) Claim(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.EmailOutbox, error) {
	var rows []domain.EmailOutbox
	err := database.Conn(ctx, r.db).Clauses(forUpdateSkipLocked).
		Where("(status = ? AND next_attempt_at <= ?) OR (status = ? AND locked_until <= ?)",
			domain.EmailQueued, now, domain.EmailSending, now).
		Order("next_attempt_at ASC").Limit(clampLimit(limit, 50, 500)).Find(&rows).Error
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	ids := make([]uuid.UUID, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	until := now.Add(lease)
	if err := database.Conn(ctx, r.db).Model(&domain.EmailOutbox{}).Where("id IN ?", ids).Updates(map[string]any{
		"status": domain.EmailSending, "locked_until": until, "attempts": gorm.Expr("attempts + 1"), "updated_at": now,
	}).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].Status = domain.EmailSending
		rows[i].LockedUntil = &until
		rows[i].Attempts++
	}
	return rows, nil
}

func (r *GormEmailRepository) Finish(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	return database.Conn(ctx, r.db).Model(&domain.EmailOutbox{}).Where("id = ?", id).Updates(fields).Error
}

func (r *GormEmailRepository) PurgeFinished(ctx context.Context, before time.Time) (int64, error) {
	res := database.Conn(ctx, r.db).Where("status IN ? AND updated_at < ?", finalEmailStatuses, before).Delete(&domain.EmailOutbox{})
	return res.RowsAffected, res.Error
}

func (r *GormEmailRepository) Get(ctx context.Context, id uuid.UUID) (*domain.EmailOutbox, error) {
	var row domain.EmailOutbox
	if err := database.Conn(ctx, r.db).First(&row, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &row, nil
}

func (r *GormEmailRepository) List(ctx context.Context, userID *uuid.UUID, template string, limit int) ([]domain.EmailOutbox, error) {
	q := database.Conn(ctx, r.db).Model(&domain.EmailOutbox{})
	if userID != nil {
		q = q.Where("user_id = ?", *userID)
	}
	if template != "" {
		q = q.Where("template = ?", template)
	}
	var out []domain.EmailOutbox
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(limit, 50, 500)).Find(&out).Error
	return out, err
}
