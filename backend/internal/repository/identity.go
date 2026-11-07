package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// ---------------------------------------------------------------------------
// Signed-in devices
// ---------------------------------------------------------------------------

// SessionRepository persists signed-in devices (refresh-token families).
// Revoking refresh tokens through TokenRepository also revokes the matching
// session rows, so both stay consistent for every caller.
type SessionRepository interface {
	Create(ctx context.Context, s *domain.UserSession) error
	Get(ctx context.Context, id uuid.UUID) (*domain.UserSession, error)
	// Touch records activity (and, on refresh, the new expiry).
	Touch(ctx context.Context, id uuid.UUID, at time.Time, expiresAt *time.Time) error
	// ListActive pages a user's live devices, most recently active first.
	ListActive(ctx context.Context, userID uuid.UUID, now time.Time, c *Cursor, limit int) ([]domain.UserSession, error)
	// ActiveIDs returns the ids of a user's live devices except keep.
	ActiveIDs(ctx context.Context, userID, keep uuid.UUID, now time.Time) ([]uuid.UUID, error)
	// IsRevoked reports whether a family has been signed out (unknown
	// families are not revoked).
	IsRevoked(ctx context.Context, id uuid.UUID) (bool, error)
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
	// DeleteForUser removes every device of a user (and, through the
	// foreign key, their refresh tokens).
	DeleteForUser(ctx context.Context, userID uuid.UUID) (int64, error)
}

// GormSessionRepository implements SessionRepository.
type GormSessionRepository struct{ db *gorm.DB }

// NewSessionRepository returns a session repository.
func NewSessionRepository(db *gorm.DB) *GormSessionRepository { return &GormSessionRepository{db: db} }

// Create implements SessionRepository.
func (r *GormSessionRepository) Create(ctx context.Context, s *domain.UserSession) error {
	return database.Conn(ctx, r.db).Create(s).Error
}

// Get implements SessionRepository.
func (r *GormSessionRepository) Get(ctx context.Context, id uuid.UUID) (*domain.UserSession, error) {
	var s domain.UserSession
	if err := database.Conn(ctx, r.db).First(&s, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

// Touch implements SessionRepository.
func (r *GormSessionRepository) Touch(ctx context.Context, id uuid.UUID, at time.Time, expiresAt *time.Time) error {
	fields := map[string]any{"last_seen_at": at, "updated_at": at}
	if expiresAt != nil {
		fields["expires_at"] = *expiresAt
	}
	return database.Conn(ctx, r.db).Model(&domain.UserSession{}).Where("id = ? AND revoked_at IS NULL", id).Updates(fields).Error
}

// ListActive implements SessionRepository.
func (r *GormSessionRepository) ListActive(ctx context.Context, userID uuid.UUID, now time.Time, c *Cursor, limit int) ([]domain.UserSession, error) {
	q := database.Conn(ctx, r.db).Model(&domain.UserSession{}).
		Where("user_id = ? AND revoked_at IS NULL AND expires_at > ?", userID, now)
	q = applyCursor(q, c, "last_seen_at", "id")
	var out []domain.UserSession
	err := q.Order("last_seen_at DESC, id DESC").Limit(clampLimit(limit, 10, 51)).Find(&out).Error
	return out, err
}

// ActiveIDs implements SessionRepository.
func (r *GormSessionRepository) ActiveIDs(ctx context.Context, userID, keep uuid.UUID, now time.Time) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := database.Conn(ctx, r.db).Model(&domain.UserSession{}).
		Where("user_id = ? AND id <> ? AND revoked_at IS NULL AND expires_at > ?", userID, keep, now).
		Pluck("id", &ids).Error
	return ids, err
}

// IsRevoked implements SessionRepository.
func (r *GormSessionRepository) IsRevoked(ctx context.Context, id uuid.UUID) (bool, error) {
	var s domain.UserSession
	err := database.Conn(ctx, r.db).Select("revoked_at").First(&s, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return s.RevokedAt != nil, nil
}

// DeleteExpired removes sessions that expired before the cut-off (their
// refresh tokens are removed with them).
func (r *GormSessionRepository) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	res := database.Conn(ctx, r.db).Where("expires_at < ?", before).Delete(&domain.UserSession{})
	return res.RowsAffected, res.Error
}

// DeleteForUser implements SessionRepository.
func (r *GormSessionRepository) DeleteForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	res := database.Conn(ctx, r.db).Where("user_id = ?", userID).Delete(&domain.UserSession{})
	return res.RowsAffected, res.Error
}

// ---------------------------------------------------------------------------
// One-time account tokens
// ---------------------------------------------------------------------------

// ActionTokenRepository persists single-use account tokens.
type ActionTokenRepository interface {
	Create(ctx context.Context, t *domain.ActionToken) error
	// InvalidateOpen marks a user's unused tokens of a purpose as used.
	InvalidateOpen(ctx context.Context, userID uuid.UUID, purpose domain.ActionPurpose, at time.Time) error
	// Consume atomically marks an unused, unexpired token used and returns
	// it; anything else is ErrNotFound.
	Consume(ctx context.Context, hash string, purpose domain.ActionPurpose, now time.Time) (*domain.ActionToken, error)
	// LatestOpen returns the newest unused, unexpired token of a purpose.
	LatestOpen(ctx context.Context, userID uuid.UUID, purpose domain.ActionPurpose, now time.Time) (*domain.ActionToken, error)
	DeleteForUser(ctx context.Context, userID uuid.UUID) (int64, error)
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}

// GormActionTokenRepository implements ActionTokenRepository.
type GormActionTokenRepository struct{ db *gorm.DB }

// NewActionTokenRepository returns an action token repository.
func NewActionTokenRepository(db *gorm.DB) *GormActionTokenRepository {
	return &GormActionTokenRepository{db: db}
}

// Create implements ActionTokenRepository.
func (r *GormActionTokenRepository) Create(ctx context.Context, t *domain.ActionToken) error {
	return database.Conn(ctx, r.db).Create(t).Error
}

// InvalidateOpen implements ActionTokenRepository.
func (r *GormActionTokenRepository) InvalidateOpen(ctx context.Context, userID uuid.UUID, purpose domain.ActionPurpose, at time.Time) error {
	return database.Conn(ctx, r.db).Model(&domain.ActionToken{}).
		Where("user_id = ? AND purpose = ? AND used_at IS NULL", userID, purpose).
		Updates(map[string]any{"used_at": at, "updated_at": at}).Error
}

// Consume implements ActionTokenRepository.
func (r *GormActionTokenRepository) Consume(ctx context.Context, hash string, purpose domain.ActionPurpose, now time.Time) (*domain.ActionToken, error) {
	var t domain.ActionToken
	conn := database.Conn(ctx, r.db)
	err := conn.Clauses(forUpdate).
		Where("token_hash = ? AND purpose = ? AND used_at IS NULL AND expires_at > ?", hash, purpose, now).
		First(&t).Error
	if err != nil {
		return nil, notFound(err)
	}
	res := conn.Model(&domain.ActionToken{}).Where("id = ? AND used_at IS NULL", t.ID).
		Updates(map[string]any{"used_at": now, "updated_at": now})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	t.UsedAt = &now
	return &t, nil
}

// LatestOpen implements ActionTokenRepository.
func (r *GormActionTokenRepository) LatestOpen(ctx context.Context, userID uuid.UUID, purpose domain.ActionPurpose, now time.Time) (*domain.ActionToken, error) {
	var t domain.ActionToken
	err := database.Conn(ctx, r.db).
		Where("user_id = ? AND purpose = ? AND used_at IS NULL AND expires_at > ?", userID, purpose, now).
		Order("created_at DESC, id DESC").First(&t).Error
	if err != nil {
		return nil, notFound(err)
	}
	return &t, nil
}

// DeleteForUser implements ActionTokenRepository.
func (r *GormActionTokenRepository) DeleteForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	res := database.Conn(ctx, r.db).Where("user_id = ?", userID).Delete(&domain.ActionToken{})
	return res.RowsAffected, res.Error
}

// DeleteExpired implements ActionTokenRepository.
func (r *GormActionTokenRepository) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	res := database.Conn(ctx, r.db).Where("expires_at < ?", before).Delete(&domain.ActionToken{})
	return res.RowsAffected, res.Error
}

// ---------------------------------------------------------------------------
// Consent
// ---------------------------------------------------------------------------

// ConsentRepository persists consent choices and their history.
type ConsentRepository interface {
	List(ctx context.Context, userID uuid.UUID) ([]domain.Consent, error)
	Get(ctx context.Context, userID uuid.UUID, category domain.ConsentCategory) (*domain.Consent, error)
	Upsert(ctx context.Context, c *domain.Consent) error
	AddEvent(ctx context.Context, e *domain.ConsentEvent) error
	ListEvents(ctx context.Context, userID uuid.UUID, limit int) ([]domain.ConsentEvent, error)
	// DeleteForUser removes current choices (the history is kept as proof).
	DeleteForUser(ctx context.Context, userID uuid.UUID) (int64, error)
}

// GormConsentRepository implements ConsentRepository.
type GormConsentRepository struct{ db *gorm.DB }

// NewConsentRepository returns a consent repository.
func NewConsentRepository(db *gorm.DB) *GormConsentRepository { return &GormConsentRepository{db: db} }

// List implements ConsentRepository.
func (r *GormConsentRepository) List(ctx context.Context, userID uuid.UUID) ([]domain.Consent, error) {
	var out []domain.Consent
	err := database.Conn(ctx, r.db).Where("user_id = ?", userID).Order("category").Find(&out).Error
	return out, err
}

// Get implements ConsentRepository.
func (r *GormConsentRepository) Get(ctx context.Context, userID uuid.UUID, category domain.ConsentCategory) (*domain.Consent, error) {
	var c domain.Consent
	if err := database.Conn(ctx, r.db).First(&c, "user_id = ? AND category = ?", userID, category).Error; err != nil {
		return nil, notFound(err)
	}
	return &c, nil
}

// Upsert implements ConsentRepository.
func (r *GormConsentRepository) Upsert(ctx context.Context, c *domain.Consent) error {
	return database.Conn(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "category"}},
		DoUpdates: clause.AssignmentColumns([]string{"granted", "policy_version", "source", "updated_at"}),
	}).Create(c).Error
}

// AddEvent implements ConsentRepository.
func (r *GormConsentRepository) AddEvent(ctx context.Context, e *domain.ConsentEvent) error {
	return database.Conn(ctx, r.db).Create(e).Error
}

// ListEvents implements ConsentRepository.
func (r *GormConsentRepository) ListEvents(ctx context.Context, userID uuid.UUID, limit int) ([]domain.ConsentEvent, error) {
	var out []domain.ConsentEvent
	err := database.Conn(ctx, r.db).Where("user_id = ?", userID).Order("created_at DESC, id DESC").
		Limit(clampLimit(limit, 100, 1000)).Find(&out).Error
	return out, err
}

// DeleteForUser implements ConsentRepository.
func (r *GormConsentRepository) DeleteForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	res := database.Conn(ctx, r.db).Where("user_id = ?", userID).Delete(&domain.Consent{})
	return res.RowsAffected, res.Error
}

// ---------------------------------------------------------------------------
// Personal-data exports
// ---------------------------------------------------------------------------

// ExportRepository persists export jobs.
type ExportRepository interface {
	// Create inserts a job; ErrDuplicate when one is already queued/running.
	Create(ctx context.Context, j *domain.DataExportJob) error
	Get(ctx context.Context, userID, id uuid.UUID) (*domain.DataExportJob, error)
	List(ctx context.Context, userID uuid.UUID, c *Cursor, limit int) ([]domain.DataExportJob, error)
	CountSince(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error)
	// ClaimDue leases queued jobs (and running ones whose lease expired).
	ClaimDue(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.DataExportJob, error)
	Save(ctx context.Context, id uuid.UUID, fields map[string]any) error
	IncrementDownloads(ctx context.Context, id uuid.UUID, at time.Time) error
	// ClaimExpired returns ready jobs past their expiry (locked).
	ClaimExpired(ctx context.Context, now time.Time, limit int) ([]domain.DataExportJob, error)
	ListAllForUser(ctx context.Context, userID uuid.UUID) ([]domain.DataExportJob, error)
	SoftDeleteForUser(ctx context.Context, userID uuid.UUID, at time.Time) (int64, error)
}

// GormExportRepository implements ExportRepository.
type GormExportRepository struct{ db *gorm.DB }

// NewExportRepository returns an export repository.
func NewExportRepository(db *gorm.DB) *GormExportRepository { return &GormExportRepository{db: db} }

// Create implements ExportRepository.
func (r *GormExportRepository) Create(ctx context.Context, j *domain.DataExportJob) error {
	if err := database.Conn(ctx, r.db).Create(j).Error; err != nil {
		if isUnique(err, "data_export_jobs_active_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

// Get implements ExportRepository.
func (r *GormExportRepository) Get(ctx context.Context, userID, id uuid.UUID) (*domain.DataExportJob, error) {
	var j domain.DataExportJob
	if err := database.Conn(ctx, r.db).First(&j, "id = ? AND user_id = ?", id, userID).Error; err != nil {
		return nil, notFound(err)
	}
	return &j, nil
}

// List implements ExportRepository.
func (r *GormExportRepository) List(ctx context.Context, userID uuid.UUID, c *Cursor, limit int) ([]domain.DataExportJob, error) {
	q := database.Conn(ctx, r.db).Model(&domain.DataExportJob{}).Where("user_id = ?", userID)
	q = applyCursor(q, c, "requested_at", "id")
	var out []domain.DataExportJob
	err := q.Order("requested_at DESC, id DESC").Limit(clampLimit(limit, 10, 51)).Find(&out).Error
	return out, err
}

// CountSince implements ExportRepository.
func (r *GormExportRepository) CountSince(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error) {
	var n int64
	err := database.Conn(ctx, r.db).Model(&domain.DataExportJob{}).Unscoped().
		Where("user_id = ? AND requested_at >= ?", userID, since).Count(&n).Error
	return n, err
}

// ClaimDue implements ExportRepository. Call it inside a transaction so the
// row locks hold until the lease is written.
func (r *GormExportRepository) ClaimDue(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.DataExportJob, error) {
	var out []domain.DataExportJob
	conn := database.Conn(ctx, r.db)
	err := conn.Clauses(forUpdateSkipLocked).
		Where("status = ? OR (status = ? AND locked_until < ?)", domain.ExportQueued, domain.ExportRunning, now).
		Order("requested_at").Limit(clampLimit(limit, 5, 50)).Find(&out).Error
	if err != nil || len(out) == 0 {
		return out, err
	}
	until := now.Add(lease)
	for i := range out {
		fields := map[string]any{"status": domain.ExportRunning, "locked_until": until, "attempts": gorm.Expr("attempts + 1"), "updated_at": now}
		if out[i].StartedAt == nil {
			fields["started_at"] = now
			out[i].StartedAt = &now
		}
		if err := conn.Model(&domain.DataExportJob{}).Where("id = ?", out[i].ID).Updates(fields).Error; err != nil {
			return nil, err
		}
		out[i].Status, out[i].LockedUntil = domain.ExportRunning, &until
		out[i].Attempts++
	}
	return out, nil
}

// Save implements ExportRepository.
func (r *GormExportRepository) Save(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	return database.Conn(ctx, r.db).Model(&domain.DataExportJob{}).Where("id = ?", id).Updates(fields).Error
}

// IncrementDownloads implements ExportRepository.
func (r *GormExportRepository) IncrementDownloads(ctx context.Context, id uuid.UUID, at time.Time) error {
	return database.Conn(ctx, r.db).Model(&domain.DataExportJob{}).Where("id = ?", id).
		Updates(map[string]any{"download_count": gorm.Expr("download_count + 1"), "updated_at": at}).Error
}

// ClaimExpired implements ExportRepository.
func (r *GormExportRepository) ClaimExpired(ctx context.Context, now time.Time, limit int) ([]domain.DataExportJob, error) {
	var out []domain.DataExportJob
	err := database.Conn(ctx, r.db).Clauses(forUpdateSkipLocked).
		Where("status = ? AND expires_at < ?", domain.ExportReady, now).Order("expires_at").Limit(limit).Find(&out).Error
	return out, err
}

// ListAllForUser implements ExportRepository.
func (r *GormExportRepository) ListAllForUser(ctx context.Context, userID uuid.UUID) ([]domain.DataExportJob, error) {
	var out []domain.DataExportJob
	err := database.Conn(ctx, r.db).Where("user_id = ?", userID).Find(&out).Error
	return out, err
}

// SoftDeleteForUser implements ExportRepository.
func (r *GormExportRepository) SoftDeleteForUser(ctx context.Context, userID uuid.UUID, at time.Time) (int64, error) {
	res := database.Conn(ctx, r.db).Model(&domain.DataExportJob{}).Where("user_id = ?", userID).
		Updates(map[string]any{"deleted_at": at, "updated_at": at, "status": domain.ExportExpired, "object_key": ""})
	return res.RowsAffected, res.Error
}

// ---------------------------------------------------------------------------
// Account settings and closures
// ---------------------------------------------------------------------------

// AccountRepository holds the account self-service queries that the user
// repository does not cover.
type AccountRepository interface {
	// SettingsVersion returns user_profiles.settings_version, locking the
	// row when lock is set (inside a transaction).
	SettingsVersion(ctx context.Context, userID uuid.UUID, lock bool) (int, error)
	// DueClosures returns accounts whose scheduled closure date has passed.
	DueClosures(ctx context.Context, now time.Time, limit int) ([]uuid.UUID, error)
}

// GormAccountRepository implements AccountRepository.
type GormAccountRepository struct{ db *gorm.DB }

// NewAccountRepository returns an account repository.
func NewAccountRepository(db *gorm.DB) *GormAccountRepository { return &GormAccountRepository{db: db} }

// SettingsVersion implements AccountRepository.
func (r *GormAccountRepository) SettingsVersion(ctx context.Context, userID uuid.UUID, lock bool) (int, error) {
	q := database.Conn(ctx, r.db).Model(&domain.Profile{}).Where("user_id = ?", userID)
	if lock {
		q = q.Clauses(forUpdate)
	}
	var versions []int
	if err := q.Pluck("settings_version", &versions).Error; err != nil {
		return 0, err
	}
	if len(versions) == 0 {
		return 0, ErrNotFound
	}
	return versions[0], nil
}

// DueClosures implements AccountRepository.
func (r *GormAccountRepository) DueClosures(ctx context.Context, now time.Time, limit int) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := database.Conn(ctx, r.db).Model(&domain.User{}).
		Where("status = ? AND deactivated_by = ? AND closure_scheduled_at <= ?", domain.UserDeactivated, domain.DeactivatedByClosure, now).
		Order("closure_scheduled_at").Limit(clampLimit(limit, 20, 200)).Pluck("id", &ids).Error
	return ids, err
}

// ---------------------------------------------------------------------------
// Personal-data export reads
// ---------------------------------------------------------------------------

// exportColumns are the owner columns export reads may filter on.
var exportColumns = map[string]bool{"user_id": true, "client_id": true, "psychic_id": true, "uploader_id": true, "author_id": true, "ticket_id": true}

// ExportDataRepository reads a user's personal data for an export. Reads
// are bounded and oldest first.
type ExportDataRepository interface {
	// Rows loads into dest (a pointer to a slice of a model) the rows whose
	// owner column equals id. column must be one of exportColumns.
	Rows(ctx context.Context, dest any, column string, id uuid.UUID, limit int) error
	// ConversationMessages loads the messages of every conversation the
	// user takes part in.
	ConversationMessages(ctx context.Context, userID uuid.UUID, limit int) ([]domain.Message, error)
	// Conversations loads the user's conversations (as client or advisor).
	Conversations(ctx context.Context, userID uuid.UUID, limit int) ([]domain.Conversation, error)
}

// GormExportDataRepository implements ExportDataRepository.
type GormExportDataRepository struct{ db *gorm.DB }

// NewExportDataRepository returns an export data repository.
func NewExportDataRepository(db *gorm.DB) *GormExportDataRepository {
	return &GormExportDataRepository{db: db}
}

// Rows implements ExportDataRepository.
func (r *GormExportDataRepository) Rows(ctx context.Context, dest any, column string, id uuid.UUID, limit int) error {
	if !exportColumns[column] {
		return errors.New("repository: export column not allowed")
	}
	return database.Conn(ctx, r.db).Where(column+" = ?", id).Order("created_at").Limit(limit).Find(dest).Error
}

// ConversationMessages implements ExportDataRepository.
func (r *GormExportDataRepository) ConversationMessages(ctx context.Context, userID uuid.UUID, limit int) ([]domain.Message, error) {
	conn := database.Conn(ctx, r.db)
	mine := conn.Model(&domain.Conversation{}).Select("id").Where("client_id = ? OR psychic_id = ?", userID, userID)
	var out []domain.Message
	err := conn.Where("conversation_id IN (?)", mine).Order("created_at, id").Limit(limit).Find(&out).Error
	return out, err
}

// Conversations implements ExportDataRepository.
func (r *GormExportDataRepository) Conversations(ctx context.Context, userID uuid.UUID, limit int) ([]domain.Conversation, error) {
	var out []domain.Conversation
	err := database.Conn(ctx, r.db).Where("client_id = ? OR psychic_id = ?", userID, userID).
		Order("created_at, id").Limit(limit).Find(&out).Error
	return out, err
}
