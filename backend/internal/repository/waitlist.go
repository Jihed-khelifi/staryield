package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// WaitlistRow is an open entry with the advisor's public card fields.
type WaitlistRow struct {
	domain.WaitlistEntry
	PsychicSlug  string
	PsychicName  string
	PsychicImage string
	Availability domain.Availability
}

// WaitlistRepository persists advisor waiting lists.
type WaitlistRepository interface {
	// Open returns the user's open entry for an advisor, or ErrNotFound.
	Open(ctx context.Context, userID, psychicID uuid.UUID) (*domain.WaitlistEntry, error)
	// CountOpen counts the user's open entries for advisors who are public.
	CountOpen(ctx context.Context, userID uuid.UUID) (int64, error)
	Create(ctx context.Context, e *domain.WaitlistEntry) error
	// Leave soft-deletes the user's open entry for an advisor; it reports
	// whether there was one.
	Leave(ctx context.Context, userID, psychicID uuid.UUID, at time.Time) (bool, error)
	// ListOpen pages the user's open entries for public advisors, newest first.
	ListOpen(ctx context.Context, userID uuid.UUID, c *Cursor, limit int) ([]WaitlistRow, error)
	// MarkAvailable stamps an advisor's open, unstamped entries.
	MarkAvailable(ctx context.Context, psychicID uuid.UUID, at time.Time) (int64, error)
	// ClaimDue locks up to limit stamped open entries, oldest stamp first,
	// skipping rows another worker holds. Call inside a transaction.
	ClaimDue(ctx context.Context, limit int) ([]domain.WaitlistEntry, error)
	// MarkNotified finishes entries.
	MarkNotified(ctx context.Context, ids []uuid.UUID, at time.Time) error
	// ResetAvailable clears the stamp of entries whose advisor went away
	// before they were notified (they wait for the next time).
	ResetAvailable(ctx context.Context, ids []uuid.UUID, at time.Time) error
	// ExpireOpen soft-deletes up to limit open entries created before cutoff.
	ExpireOpen(ctx context.Context, cutoff, at time.Time, limit int) (int64, error)
	// PurgeFinished hard-deletes up to limit finished entries last changed
	// before cutoff.
	PurgeFinished(ctx context.Context, cutoff time.Time, limit int) (int64, error)
	// PurgeForUser hard-deletes a user's entries and every entry waiting for
	// them as an advisor (account erasure).
	PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error)
}

// GormWaitlistRepository implements WaitlistRepository.
type GormWaitlistRepository struct{ db *gorm.DB }

// NewWaitlistRepository returns a GORM waitlist repository.
func NewWaitlistRepository(db *gorm.DB) *GormWaitlistRepository {
	return &GormWaitlistRepository{db: db}
}

// open scopes to open entries (GORM already skips soft-deleted rows).
func (r *GormWaitlistRepository) open(ctx context.Context) *gorm.DB {
	return database.Conn(ctx, r.db).Model(&domain.WaitlistEntry{}).Where("advisor_waitlist.notified_at IS NULL")
}

// publicAdvisor joins the advisor and keeps listed advisors of active accounts.
func publicAdvisor(q *gorm.DB) *gorm.DB {
	return q.Joins("JOIN psychics ON psychics.user_id = advisor_waitlist.psychic_id AND psychics.listed").
		Joins("JOIN users ON users.id = advisor_waitlist.psychic_id AND users.status = ?", domain.UserActive)
}

// Open implements WaitlistRepository.
func (r *GormWaitlistRepository) Open(ctx context.Context, userID, psychicID uuid.UUID) (*domain.WaitlistEntry, error) {
	var e domain.WaitlistEntry
	if err := r.open(ctx).Where("user_id = ? AND psychic_id = ?", userID, psychicID).First(&e).Error; err != nil {
		return nil, notFound(err)
	}
	return &e, nil
}

// CountOpen implements WaitlistRepository.
func (r *GormWaitlistRepository) CountOpen(ctx context.Context, userID uuid.UUID) (int64, error) {
	var n int64
	err := publicAdvisor(r.open(ctx)).Where("advisor_waitlist.user_id = ?", userID).Count(&n).Error
	return n, err
}

// Create implements WaitlistRepository.
func (r *GormWaitlistRepository) Create(ctx context.Context, e *domain.WaitlistEntry) error {
	err := database.Conn(ctx, r.db).Create(e).Error
	if database.IsUniqueViolation(err) {
		return ErrDuplicate
	}
	return err
}

// Leave implements WaitlistRepository.
func (r *GormWaitlistRepository) Leave(ctx context.Context, userID, psychicID uuid.UUID, at time.Time) (bool, error) {
	res := r.open(ctx).Where("user_id = ? AND psychic_id = ?", userID, psychicID).
		Updates(map[string]any{"deleted_at": at, "updated_at": at})
	return res.RowsAffected > 0, res.Error
}

// ListOpen implements WaitlistRepository.
func (r *GormWaitlistRepository) ListOpen(ctx context.Context, userID uuid.UUID, c *Cursor, limit int) ([]WaitlistRow, error) {
	q := publicAdvisor(r.open(ctx)).
		Select("advisor_waitlist.*, psychics.slug AS psychic_slug, psychics.name AS psychic_name, "+
			"psychics.image_url AS psychic_image, psychics.availability AS availability").
		Where("advisor_waitlist.user_id = ?", userID)
	q = applyCursor(q, c, "advisor_waitlist.created_at", "advisor_waitlist.id")
	var out []WaitlistRow
	err := q.Order("advisor_waitlist.created_at DESC, advisor_waitlist.id DESC").Limit(clampLimit(limit, 5, 51)).Scan(&out).Error
	return out, err
}

// MarkAvailable implements WaitlistRepository.
func (r *GormWaitlistRepository) MarkAvailable(ctx context.Context, psychicID uuid.UUID, at time.Time) (int64, error) {
	res := r.open(ctx).Where("psychic_id = ? AND available_at IS NULL", psychicID).
		Updates(map[string]any{"available_at": at, "updated_at": at})
	return res.RowsAffected, res.Error
}

// ClaimDue implements WaitlistRepository.
func (r *GormWaitlistRepository) ClaimDue(ctx context.Context, limit int) ([]domain.WaitlistEntry, error) {
	var out []domain.WaitlistEntry
	err := r.open(ctx).Where("available_at IS NOT NULL").Clauses(forUpdateSkipLocked).
		Order("available_at, created_at, id").Limit(clampLimit(limit, 200, 1000)).Find(&out).Error
	return out, err
}

// MarkNotified implements WaitlistRepository.
func (r *GormWaitlistRepository) MarkNotified(ctx context.Context, ids []uuid.UUID, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return r.open(ctx).Where("id IN ?", ids).Updates(map[string]any{"notified_at": at, "updated_at": at}).Error
}

// ResetAvailable implements WaitlistRepository.
func (r *GormWaitlistRepository) ResetAvailable(ctx context.Context, ids []uuid.UUID, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return r.open(ctx).Where("id IN ?", ids).Updates(map[string]any{"available_at": nil, "updated_at": at}).Error
}

// ExpireOpen implements WaitlistRepository.
func (r *GormWaitlistRepository) ExpireOpen(ctx context.Context, cutoff, at time.Time, limit int) (int64, error) {
	ids := r.open(ctx).Select("id").Where("created_at < ?", cutoff).Order("created_at").Limit(clampLimit(limit, 500, 5000))
	res := database.Conn(ctx, r.db).Model(&domain.WaitlistEntry{}).Where("id IN (?)", ids).
		Updates(map[string]any{"deleted_at": at, "updated_at": at})
	return res.RowsAffected, res.Error
}

// PurgeFinished implements WaitlistRepository.
func (r *GormWaitlistRepository) PurgeFinished(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	ids := database.Conn(ctx, r.db).Unscoped().Model(&domain.WaitlistEntry{}).Select("id").
		Where("(deleted_at IS NOT NULL OR notified_at IS NOT NULL) AND updated_at < ?", cutoff).
		Order("updated_at").Limit(clampLimit(limit, 500, 5000))
	res := database.Conn(ctx, r.db).Unscoped().Where("id IN (?)", ids).Delete(&domain.WaitlistEntry{})
	return res.RowsAffected, res.Error
}

// PurgeForUser implements WaitlistRepository.
func (r *GormWaitlistRepository) PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	res := database.Conn(ctx, r.db).Unscoped().Where("user_id = ? OR psychic_id = ?", userID, userID).Delete(&domain.WaitlistEntry{})
	return res.RowsAffected, res.Error
}
