package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// PhotoSubmissionRepository persists advisor-uploaded profile photos
// awaiting or past review (psychic_photo_submissions).
type PhotoSubmissionRepository interface {
	Create(ctx context.Context, s *domain.PhotoSubmission) error
	// Current returns the advisor's newest live submission (ErrNotFound
	// when there is none).
	Current(ctx context.Context, psychicID uuid.UUID) (*domain.PhotoSubmission, error)
	// PendingForUpdate locks the advisor's live pending submission.
	PendingForUpdate(ctx context.Context, psychicID uuid.UUID) (*domain.PhotoSubmission, error)
	// Get returns a live submission.
	Get(ctx context.Context, id uuid.UUID) (*domain.PhotoSubmission, error)
	// GetForUpdate returns a live submission with a row lock.
	GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.PhotoSubmission, error)
	// Update writes fields of a submission (live or soft-deleted).
	Update(ctx context.Context, id uuid.UUID, fields map[string]any) error
	// CountSince counts an advisor's uploads since t, withdrawn ones
	// included (the daily cap).
	CountSince(ctx context.Context, psychicID uuid.UUID, t time.Time) (int64, error)
	// AdminList pages live submissions for the review console.
	AdminList(ctx context.Context, f PhotoAdminFilter) ([]AdminPhotoRow, error)
	// ExpiredRejected returns live rejected submissions reviewed before t.
	ExpiredRejected(ctx context.Context, before time.Time, limit int) ([]domain.PhotoSubmission, error)
	// Leftovers returns finished submissions whose private image is still
	// stored (approved, withdrawn or soft-deleted rows with an object key).
	Leftovers(ctx context.Context, limit int) ([]domain.PhotoSubmission, error)
	// PurgeDeleted hard-deletes rows soft-deleted before t that no longer
	// hold an image.
	PurgeDeleted(ctx context.Context, before time.Time) (int64, error)
	// ObjectKeysForUser lists the private images of every submission of an
	// advisor (erasure).
	ObjectKeysForUser(ctx context.Context, psychicID uuid.UUID) ([]string, error)
	// DeleteForUser hard-deletes every submission of an advisor (erasure).
	DeleteForUser(ctx context.Context, psychicID uuid.UUID) (int64, error)
}

// PhotoAdminFilter narrows the review console listing. Pending submissions
// are listed oldest first (a queue), the others newest first.
type PhotoAdminFilter struct {
	Status domain.PhotoSubmissionStatus // "" = every live status
	Cursor *Cursor
	Limit  int
}

// AdminPhotoRow is a submission with its advisor's public identity.
type AdminPhotoRow struct {
	domain.PhotoSubmission
	PsychicName     string
	PsychicSlug     string
	PsychicImageURL string
}

// GormPhotoSubmissionRepository implements PhotoSubmissionRepository.
type GormPhotoSubmissionRepository struct{ db *gorm.DB }

// NewPhotoSubmissionRepository returns a GORM photo submission repository.
func NewPhotoSubmissionRepository(db *gorm.DB) *GormPhotoSubmissionRepository {
	return &GormPhotoSubmissionRepository{db: db}
}

// Create implements PhotoSubmissionRepository.
func (r *GormPhotoSubmissionRepository) Create(ctx context.Context, s *domain.PhotoSubmission) error {
	if err := database.Conn(ctx, r.db).Create(s).Error; err != nil {
		if isUnique(err, "psychic_photo_one_pending") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

// Current implements PhotoSubmissionRepository.
func (r *GormPhotoSubmissionRepository) Current(ctx context.Context, psychicID uuid.UUID) (*domain.PhotoSubmission, error) {
	var s domain.PhotoSubmission
	err := database.Conn(ctx, r.db).Where("psychic_id = ?", psychicID).Order("created_at DESC, id DESC").First(&s).Error
	if err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

// PendingForUpdate implements PhotoSubmissionRepository.
func (r *GormPhotoSubmissionRepository) PendingForUpdate(ctx context.Context, psychicID uuid.UUID) (*domain.PhotoSubmission, error) {
	var s domain.PhotoSubmission
	err := database.Conn(ctx, r.db).Clauses(forUpdate).
		Where("psychic_id = ? AND status = ?", psychicID, domain.PhotoSubmissionPending).First(&s).Error
	if err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

// Get implements PhotoSubmissionRepository.
func (r *GormPhotoSubmissionRepository) Get(ctx context.Context, id uuid.UUID) (*domain.PhotoSubmission, error) {
	var s domain.PhotoSubmission
	if err := database.Conn(ctx, r.db).First(&s, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

// GetForUpdate implements PhotoSubmissionRepository.
func (r *GormPhotoSubmissionRepository) GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.PhotoSubmission, error) {
	var s domain.PhotoSubmission
	if err := database.Conn(ctx, r.db).Clauses(forUpdate).First(&s, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

// Update implements PhotoSubmissionRepository.
func (r *GormPhotoSubmissionRepository) Update(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	res := database.Conn(ctx, r.db).Unscoped().Model(&domain.PhotoSubmission{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// CountSince implements PhotoSubmissionRepository.
func (r *GormPhotoSubmissionRepository) CountSince(ctx context.Context, psychicID uuid.UUID, t time.Time) (int64, error) {
	var n int64
	err := database.Conn(ctx, r.db).Unscoped().Model(&domain.PhotoSubmission{}).
		Where("psychic_id = ? AND created_at >= ?", psychicID, t).Count(&n).Error
	return n, err
}

// AdminList implements PhotoSubmissionRepository.
func (r *GormPhotoSubmissionRepository) AdminList(ctx context.Context, f PhotoAdminFilter) ([]AdminPhotoRow, error) {
	q := database.Conn(ctx, r.db).Table("psychic_photo_submissions").
		Select("psychic_photo_submissions.*, psychics.name AS psychic_name, psychics.slug AS psychic_slug, " +
			"psychics.image_url AS psychic_image_url").
		Joins("JOIN psychics ON psychics.user_id = psychic_photo_submissions.psychic_id").
		Where("psychic_photo_submissions.deleted_at IS NULL")
	if f.Status != "" {
		q = q.Where("psychic_photo_submissions.status = ?", f.Status)
	}
	if f.Status == domain.PhotoSubmissionPending {
		// The queue: oldest first, so no submission waits forever.
		if f.Cursor != nil {
			if id, err := uuid.Parse(f.Cursor.ID); err == nil {
				q = q.Where("(psychic_photo_submissions.created_at, psychic_photo_submissions.id) > (?, ?)", f.Cursor.At, id)
			} else {
				q = q.Where("psychic_photo_submissions.created_at > ?", f.Cursor.At)
			}
		}
		q = q.Order("psychic_photo_submissions.created_at ASC, psychic_photo_submissions.id ASC")
	} else {
		q = applyCursor(q, f.Cursor, "psychic_photo_submissions.created_at", "psychic_photo_submissions.id")
		q = q.Order("psychic_photo_submissions.created_at DESC, psychic_photo_submissions.id DESC")
	}
	var out []AdminPhotoRow
	err := q.Limit(clampLimit(f.Limit, 20, 101)).Scan(&out).Error
	return out, err
}

// ExpiredRejected implements PhotoSubmissionRepository.
func (r *GormPhotoSubmissionRepository) ExpiredRejected(ctx context.Context, before time.Time, limit int) ([]domain.PhotoSubmission, error) {
	var out []domain.PhotoSubmission
	err := database.Conn(ctx, r.db).Where("status = ? AND reviewed_at < ?", domain.PhotoSubmissionRejected, before).
		Order("reviewed_at ASC").Limit(clampLimit(limit, 100, 500)).Find(&out).Error
	return out, err
}

// Leftovers implements PhotoSubmissionRepository.
func (r *GormPhotoSubmissionRepository) Leftovers(ctx context.Context, limit int) ([]domain.PhotoSubmission, error) {
	var out []domain.PhotoSubmission
	err := database.Conn(ctx, r.db).Unscoped().
		Where("object_key <> '' AND (deleted_at IS NOT NULL OR status IN ?)",
			[]domain.PhotoSubmissionStatus{domain.PhotoSubmissionApproved, domain.PhotoSubmissionWithdrawn}).
		Order("updated_at ASC").Limit(clampLimit(limit, 100, 500)).Find(&out).Error
	return out, err
}

// PurgeDeleted implements PhotoSubmissionRepository.
func (r *GormPhotoSubmissionRepository) PurgeDeleted(ctx context.Context, before time.Time) (int64, error) {
	res := database.Conn(ctx, r.db).Unscoped().Where("deleted_at < ? AND object_key = ''", before).Delete(&domain.PhotoSubmission{})
	return res.RowsAffected, res.Error
}

// ObjectKeysForUser implements PhotoSubmissionRepository.
func (r *GormPhotoSubmissionRepository) ObjectKeysForUser(ctx context.Context, psychicID uuid.UUID) ([]string, error) {
	var keys []string
	err := database.Conn(ctx, r.db).Unscoped().Model(&domain.PhotoSubmission{}).
		Where("psychic_id = ? AND object_key <> ''", psychicID).Pluck("object_key", &keys).Error
	return keys, err
}

// DeleteForUser implements PhotoSubmissionRepository.
func (r *GormPhotoSubmissionRepository) DeleteForUser(ctx context.Context, psychicID uuid.UUID) (int64, error) {
	res := database.Conn(ctx, r.db).Unscoped().Where("psychic_id = ?", psychicID).Delete(&domain.PhotoSubmission{})
	return res.RowsAffected, res.Error
}
