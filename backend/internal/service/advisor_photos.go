package service

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
)

// Notification kinds of the photo review (in-app only).
const (
	NotifyPhotoApproved = "photo.approved"
	NotifyPhotoRejected = "photo.rejected"
)

// photoMinSide is the smallest accepted photo width and height (after the
// longest side has been scaled down to the advisor image size).
const photoMinSide = 256

// Photo review page sizes.
const (
	photoPageDefault = 20
	photoPageMax     = 100
)

// Photo messages (translated at the API edge).
const (
	msgPhotoUnavailable = "Photo uploads are not available yet."
	msgPhotoTooSmall    = "Use a photo at least 256 pixels wide and 256 pixels high."
	msgPhotoReviewed    = "This photo has already been reviewed."
	msgPhotoGone        = "This photo is no longer available."
	msgPhotoReason      = "A reason (up to 500 characters) is required."
	msgPhotoStatus      = "Choose pending, approved or rejected."
)

// PhotoSubmissionView is a submitted photo as its advisor sees it. The image
// itself is never public before approval: HasImage tells whether
// GET /psychic/photo/image can still stream it.
type PhotoSubmissionView struct {
	ID          uuid.UUID  `json:"id"`
	Status      string     `json:"status"`
	Reason      string     `json:"reason,omitempty"`
	Width       int        `json:"width"`
	Height      int        `json:"height"`
	Bytes       int        `json:"bytes"`
	HasImage    bool       `json:"has_image"`
	SubmittedAt time.Time  `json:"submitted_at"`
	ReviewedAt  *time.Time `json:"reviewed_at,omitempty"`
}

// PhotoLimits are the upload bounds shown next to the picker.
type PhotoLimits struct {
	MaxBytes     int64 `json:"max_bytes"`
	MaxPixels    int   `json:"max_pixels"`
	MinSide      int   `json:"min_side"`
	DailyUploads int   `json:"daily_uploads"`
}

// PhotoStateView is GET /psychic/photo: the live public photo, the latest
// submission (null when none) and the upload bounds.
type PhotoStateView struct {
	ImageURL   string               `json:"image_url"`
	Submission *PhotoSubmissionView `json:"submission"`
	Limits     PhotoLimits          `json:"limits"`
}

// AdminPhotoPsychic is the advisor of a submission in the review console.
type AdminPhotoPsychic struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Slug     string    `json:"slug"`
	ImageURL string    `json:"image_url"`
}

// AdminPhotoSubmissionView is a submission in the review console. The image
// is only reachable through the step-up preview endpoint.
type AdminPhotoSubmissionView struct {
	ID          uuid.UUID         `json:"id"`
	Status      string            `json:"status"`
	Psychic     AdminPhotoPsychic `json:"psychic"`
	ContentType string            `json:"content_type"`
	Bytes       int               `json:"bytes"`
	Width       int               `json:"width"`
	Height      int               `json:"height"`
	Reason      string            `json:"reason,omitempty"`
	HasImage    bool              `json:"has_image"`
	ReviewedBy  *uuid.UUID        `json:"reviewed_by,omitempty"`
	ReviewedAt  *time.Time        `json:"reviewed_at,omitempty"`
	SubmittedAt time.Time         `json:"submitted_at"`
}

func photoView(sub *domain.PhotoSubmission) *PhotoSubmissionView {
	return &PhotoSubmissionView{ID: sub.ID, Status: string(sub.Status), Reason: sub.Reason, Width: sub.Width, Height: sub.Height,
		Bytes: sub.Bytes, HasImage: sub.ObjectKey != "" && sub.Status != domain.PhotoSubmissionApproved,
		SubmittedAt: sub.CreatedAt, ReviewedAt: sub.ReviewedAt}
}

func adminPhotoView(sub *domain.PhotoSubmission, psychic AdminPhotoPsychic) AdminPhotoSubmissionView {
	return AdminPhotoSubmissionView{ID: sub.ID, Status: string(sub.Status), Psychic: psychic, ContentType: sub.ContentType,
		Bytes: sub.Bytes, Width: sub.Width, Height: sub.Height, Reason: sub.Reason,
		HasImage: sub.ObjectKey != "" && sub.Status != domain.PhotoSubmissionApproved, ReviewedBy: sub.ReviewedBy,
		ReviewedAt: sub.ReviewedAt, SubmittedAt: sub.CreatedAt}
}

// photosEnabled reports whether both buckets are configured.
func (s *AdvisorService) photosEnabled() bool {
	for _, st := range []storage.ObjectStore{s.d.Private, s.d.Public} {
		if st == nil {
			return false
		}
		if _, off := st.(storage.Disabled); off {
			return false
		}
	}
	return s.d.Photos != nil
}

func (s *AdvisorService) photoLimits() PhotoLimits {
	return PhotoLimits{MaxBytes: s.cfg.Image.MaxBytes, MaxPixels: s.cfg.Image.MaxPixels, MinSide: photoMinSide,
		DailyUploads: s.cfg.DailyUploads}
}

// Photo returns the advisor's live photo and latest submission.
func (s *AdvisorService) Photo(ctx context.Context, psychicID uuid.UUID) (*PhotoStateView, error) {
	p, err := s.d.Psychics.GetByUserID(ctx, psychicID)
	if err != nil {
		return nil, mapRepoErr(err, "Advisor")
	}
	out := &PhotoStateView{ImageURL: p.ImageURL, Limits: s.photoLimits()}
	if s.d.Photos == nil {
		return out, nil
	}
	cur, err := s.d.Photos.Current(ctx, psychicID)
	switch {
	case err == nil:
		out.Submission = photoView(cur)
	case !errors.Is(err, repository.ErrNotFound):
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// syncReviewStatus mirrors the latest live submission into
// psychics.photo_review_status ("approved" when none is pending or
// rejected).
func (s *AdvisorService) syncReviewStatus(ctx context.Context, psychicID uuid.UUID) error {
	status := domain.PhotoApproved
	cur, err := s.d.Photos.Current(ctx, psychicID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	if cur != nil && (cur.Status == domain.PhotoSubmissionPending || cur.Status == domain.PhotoSubmissionRejected) {
		status = string(cur.Status)
	}
	err = s.d.Psychics.Updates(ctx, psychicID, map[string]any{"photo_review_status": status})
	if errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	return err
}

// dropObject deletes a private image after its row stopped needing it and
// clears the row's key; failures are left to the clean-up (the key stays).
func (s *AdvisorService) dropObject(ctx context.Context, id uuid.UUID, key string) {
	if key == "" {
		return
	}
	if err := s.d.Private.Delete(ctx, key); err != nil {
		warnErr(ctx, "advisor: delete private photo", err, zap.String("submission_id", id.String()))
		return
	}
	warnErr(ctx, "advisor: clear photo key", s.d.Photos.Update(ctx, id, map[string]any{"object_key": ""}))
}

// SubmitPhoto stores a new photo for review. The image is validated by
// content, re-encoded (EXIF orientation applied, metadata stripped) and kept
// in the private bucket; a pending submission is superseded by the new one.
func (s *AdvisorService) SubmitPhoto(ctx context.Context, psychicID uuid.UUID, data []byte) (*PhotoStateView, error) {
	if !s.photosEnabled() {
		return nil, apperror.FeatureUnavailable(msgPhotoUnavailable)
	}
	if _, err := s.d.Psychics.GetByUserID(ctx, psychicID); err != nil {
		return nil, mapRepoErr(err, "Advisor")
	}
	now := s.d.Clock.Now()
	// The daily cap lives in the database, so it holds when the Redis rate
	// limiter fails open.
	n, err := s.d.Photos.CountSince(ctx, psychicID, now.Add(-24*time.Hour))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if n >= int64(s.cfg.DailyUploads) {
		return nil, apperror.RateLimited(time.Hour)
	}
	img, err := ProcessImage(data, s.cfg.Image)
	if err != nil {
		return nil, err
	}
	if img.Width < photoMinSide || img.Height < photoMinSide {
		return nil, apperror.Validation(map[string]string{s.cfg.Image.Field: msgPhotoTooSmall})
	}
	key := objectKey(domain.PhotoObjectPrefix+psychicID.String(), img.Extension())
	if _, err := s.d.Private.Put(ctx, key, img.ContentType, img.Data, "private, no-store"); err != nil {
		return nil, apperror.Unavailable(err)
	}
	sub := &domain.PhotoSubmission{ID: domain.NewID(), PsychicID: psychicID, Status: domain.PhotoSubmissionPending, ObjectKey: key,
		ContentType: img.ContentType, Bytes: len(img.Data), Width: img.Width, Height: img.Height, CreatedAt: now, UpdatedAt: now}
	var superseded *domain.PhotoSubmission
	err = s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		// The advisor row lock serialises concurrent uploads.
		if _, err := s.d.Psychics.GetForUpdate(ctx, psychicID); err != nil {
			return err
		}
		prev, err := s.d.Photos.PendingForUpdate(ctx, psychicID)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		if prev != nil {
			if err := s.d.Photos.Update(ctx, prev.ID, map[string]any{"status": domain.PhotoSubmissionWithdrawn,
				"deleted_at": now, "updated_at": now}); err != nil {
				return err
			}
			superseded = prev
		}
		if err := s.d.Photos.Create(ctx, sub); err != nil {
			return err
		}
		return s.d.Psychics.Updates(ctx, psychicID, map[string]any{"photo_review_status": domain.PhotoPending})
	})
	if err != nil {
		warnErr(ctx, "advisor: drop unsaved photo", s.d.Private.Delete(database.Detach(ctx), key))
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, apperror.Conflict(apperror.CodeConflict, "Another photo was uploaded at the same time. Reload and try again.")
		}
		return nil, mapRepoErr(err, "Advisor")
	}
	ctx = database.Detach(ctx)
	if superseded != nil {
		s.dropObject(ctx, superseded.ID, superseded.ObjectKey)
	}
	s.record(ctx, "psychic.photo_submitted", "photo_submission", sub.ID.String(), map[string]any{"bytes": sub.Bytes,
		"width": sub.Width, "height": sub.Height, "superseded": superseded != nil})
	s.d.Tracker.Track(ctx, "psychic.photo_submitted", map[string]any{"replaced_pending": superseded != nil})
	return s.Photo(ctx, psychicID)
}

// WithdrawPhoto removes the advisor's pending submission before review.
func (s *AdvisorService) WithdrawPhoto(ctx context.Context, psychicID uuid.UUID) error {
	if s.d.Photos == nil {
		return apperror.NotFound("Photo submission")
	}
	var removed *domain.PhotoSubmission
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := s.d.Psychics.GetForUpdate(ctx, psychicID); err != nil {
			return err
		}
		prev, err := s.d.Photos.PendingForUpdate(ctx, psychicID)
		if err != nil {
			return err
		}
		now := s.d.Clock.Now()
		if err := s.d.Photos.Update(ctx, prev.ID, map[string]any{"status": domain.PhotoSubmissionWithdrawn,
			"deleted_at": now, "updated_at": now}); err != nil {
			return err
		}
		removed = prev
		return s.syncReviewStatus(ctx, psychicID)
	})
	if err != nil {
		return mapRepoErr(err, "Photo submission")
	}
	ctx = database.Detach(ctx)
	s.dropObject(ctx, removed.ID, removed.ObjectKey)
	s.record(ctx, "psychic.photo_withdrawn", "photo_submission", removed.ID.String(), nil)
	s.d.Tracker.Track(ctx, "psychic.photo_withdrawn", nil)
	return nil
}

// readPrivate loads a submission's private image.
func (s *AdvisorService) readPrivate(ctx context.Context, sub *domain.PhotoSubmission) ([]byte, string, error) {
	if sub.ObjectKey == "" || sub.Status == domain.PhotoSubmissionApproved || s.d.Private == nil {
		return nil, "", apperror.NotFound("Photo")
	}
	body, ct, err := s.d.Private.Get(ctx, sub.ObjectKey, s.cfg.Image.MaxBytes*4)
	switch {
	case err == nil:
		if ct == "" {
			ct = sub.ContentType
		}
		return body, ct, nil
	case errors.Is(err, storage.ErrNotFound):
		return nil, "", apperror.NotFound("Photo")
	case errors.Is(err, storage.ErrDisabled):
		return nil, "", apperror.FeatureUnavailable(msgPhotoUnavailable)
	}
	return nil, "", apperror.Unavailable(err)
}

// PhotoImage streams the advisor's own pending or rejected photo (owner
// only; everyone else is answered 404 by construction).
func (s *AdvisorService) PhotoImage(ctx context.Context, psychicID uuid.UUID) ([]byte, string, error) {
	if s.d.Photos == nil {
		return nil, "", apperror.NotFound("Photo")
	}
	cur, err := s.d.Photos.Current(ctx, psychicID)
	if err != nil {
		return nil, "", mapRepoErr(err, "Photo")
	}
	return s.readPrivate(ctx, cur)
}

// ---------------------------------------------------------------------------
// Review console (/admin/photo-submissions; step-up, audited)
// ---------------------------------------------------------------------------

// AdminPhotoSubmissions pages submissions by status: pending ones oldest
// first (a queue), approved and rejected ones newest first.
func (s *AdvisorService) AdminPhotoSubmissions(ctx context.Context, status, cursor string, limit int) (Page[AdminPhotoSubmissionView], error) {
	st := domain.PhotoSubmissionStatus(status)
	if st != "" && st != domain.PhotoSubmissionPending && st != domain.PhotoSubmissionApproved && st != domain.PhotoSubmissionRejected {
		return Page[AdminPhotoSubmissionView]{}, apperror.Validation(map[string]string{"status": msgPhotoStatus})
	}
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[AdminPhotoSubmissionView]{}, err
	}
	size := clampPage(limit, photoPageDefault, photoPageMax)
	page := Page[AdminPhotoSubmissionView]{Items: []AdminPhotoSubmissionView{}}
	if s.d.Photos == nil {
		return page, nil
	}
	rows, err := s.d.Photos.AdminList(ctx, repository.PhotoAdminFilter{Status: st, Cursor: c, Limit: size + 1})
	if err != nil {
		return Page[AdminPhotoSubmissionView]{}, apperror.Internal(err)
	}
	for i := range rows {
		if i == size {
			last := rows[size-1]
			page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
			break
		}
		r := &rows[i]
		page.Items = append(page.Items, adminPhotoView(&r.PhotoSubmission, AdminPhotoPsychic{ID: r.PsychicID, Name: r.PsychicName,
			Slug: r.PsychicSlug, ImageURL: r.PsychicImageURL}))
	}
	return page, nil
}

// AdminPhotoImage streams a submission's private image to an administrator
// (the route requires a fresh step-up); every view is audited.
func (s *AdvisorService) AdminPhotoImage(ctx context.Context, id uuid.UUID) ([]byte, string, error) {
	if s.d.Photos == nil {
		return nil, "", apperror.NotFound("Photo submission")
	}
	sub, err := s.d.Photos.Get(ctx, id)
	if err != nil {
		return nil, "", mapRepoErr(err, "Photo submission")
	}
	body, ct, err := s.readPrivate(ctx, sub)
	if err != nil {
		return nil, "", err
	}
	s.record(ctx, "photo_submission.viewed", "photo_submission", sub.ID.String(), map[string]any{"psychic_id": sub.PsychicID.String()})
	return body, ct, nil
}

func (s *AdvisorService) adminPsychic(ctx context.Context, psychicID uuid.UUID) AdminPhotoPsychic {
	out := AdminPhotoPsychic{ID: psychicID}
	if p, err := s.d.Psychics.GetByUserID(ctx, psychicID); err == nil {
		out.Name, out.Slug, out.ImageURL = p.Name, p.Slug, p.ImageURL
	}
	return out
}

// ApprovePhoto publishes a pending photo: it is copied to the public
// advisor bucket and becomes the profile photo, the previous one is
// deleted, the directory refreshes and the advisor is notified.
func (s *AdvisorService) ApprovePhoto(ctx context.Context, adminID, id uuid.UUID) (*AdminPhotoSubmissionView, error) {
	if !s.photosEnabled() {
		return nil, apperror.FeatureUnavailable(msgPhotoUnavailable)
	}
	sub, err := s.d.Photos.Get(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Photo submission")
	}
	if sub.Status != domain.PhotoSubmissionPending {
		return nil, apperror.InvalidState(msgPhotoReviewed)
	}
	body, ct, err := s.readPrivate(ctx, sub)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			return nil, apperror.InvalidState(msgPhotoGone)
		}
		return nil, err
	}
	if ct != "image/png" {
		ct = "image/jpeg"
	}
	ext := ".jpg"
	if ct == "image/png" {
		ext = ".png"
	}
	publicKey := objectKey("advisors/"+sub.PsychicID.String(), ext)
	url, err := s.d.Public.Put(ctx, publicKey, ct, body, "public, max-age=31536000, immutable")
	if err != nil {
		return nil, apperror.Unavailable(err)
	}
	now := s.d.Clock.Now()
	var oldKey string
	err = s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		locked, err := s.d.Photos.GetForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if locked.Status != domain.PhotoSubmissionPending {
			return apperror.InvalidState(msgPhotoReviewed)
		}
		p, err := s.d.Psychics.GetForUpdate(ctx, sub.PsychicID)
		if err != nil {
			return err
		}
		oldKey = p.ImageKey
		if err := s.d.Photos.Update(ctx, id, map[string]any{"status": domain.PhotoSubmissionApproved, "reviewed_by": adminID,
			"reviewed_at": now, "updated_at": now}); err != nil {
			return err
		}
		return s.d.Psychics.Updates(ctx, sub.PsychicID, map[string]any{"image_url": url, "image_key": publicKey,
			"photo_review_status": domain.PhotoApproved})
	})
	if err != nil {
		warnErr(ctx, "advisor: drop unpublished photo", s.d.Public.Delete(database.Detach(ctx), publicKey))
		if _, ok := apperror.As(err); ok {
			return nil, err
		}
		return nil, mapRepoErr(err, "Photo submission")
	}
	ctx = database.Detach(ctx)
	if oldKey != "" && oldKey != publicKey {
		warnErr(ctx, "advisor: delete previous photo", s.d.Public.Delete(ctx, oldKey))
	}
	s.dropObject(ctx, sub.ID, sub.ObjectKey)
	if s.d.PsychicSvc != nil {
		s.d.PsychicSvc.DirectoryChanged()
		s.d.PsychicSvc.PublishStatus(ctx, sub.PsychicID)
	}
	s.d.Notifier.Notify(ctx, NotificationSpec{Kind: NotifyPhotoApproved, Data: map[string]any{"submission_id": sub.ID.String()},
		Link: "/profile/settings"}, sub.PsychicID)
	s.record(ctx, "photo_submission.approved", "photo_submission", sub.ID.String(), map[string]any{"psychic_id": sub.PsychicID.String()})
	s.d.Tracker.Track(ctx, "psychic.photo_approved", nil)
	sub.Status, sub.ReviewedBy, sub.ReviewedAt, sub.UpdatedAt, sub.ObjectKey = domain.PhotoSubmissionApproved, &adminID, &now, now, ""
	v := adminPhotoView(sub, s.adminPsychic(ctx, sub.PsychicID))
	return &v, nil
}

// RejectPhoto declines a pending photo with a reason shown to the advisor.
// The image stays private and is deleted by the clean-up after the
// retention period, so the advisor can still see what was declined.
func (s *AdvisorService) RejectPhoto(ctx context.Context, adminID, id uuid.UUID, reason string) (*AdminPhotoSubmissionView, error) {
	reason = CleanText(reason)
	if n := utf8.RuneCountInString(reason); n < 1 || n > domain.PhotoReasonMaxChars {
		return nil, apperror.Validation(map[string]string{"reason": msgPhotoReason})
	}
	if s.d.Photos == nil {
		return nil, apperror.NotFound("Photo submission")
	}
	now := s.d.Clock.Now()
	var sub *domain.PhotoSubmission
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		locked, err := s.d.Photos.GetForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if locked.Status != domain.PhotoSubmissionPending {
			return apperror.InvalidState(msgPhotoReviewed)
		}
		if err := s.d.Photos.Update(ctx, id, map[string]any{"status": domain.PhotoSubmissionRejected, "reason": reason,
			"reviewed_by": adminID, "reviewed_at": now, "updated_at": now}); err != nil {
			return err
		}
		locked.Status, locked.Reason, locked.ReviewedBy, locked.ReviewedAt, locked.UpdatedAt = domain.PhotoSubmissionRejected, reason, &adminID, &now, now
		sub = locked
		return s.syncReviewStatus(ctx, locked.PsychicID)
	})
	if err != nil {
		if _, ok := apperror.As(err); ok {
			return nil, err
		}
		return nil, mapRepoErr(err, "Photo submission")
	}
	ctx = database.Detach(ctx)
	s.d.Notifier.Notify(ctx, NotificationSpec{Kind: NotifyPhotoRejected, Data: map[string]any{"submission_id": sub.ID.String(),
		"reason": reason}, Link: "/profile/settings"}, sub.PsychicID)
	s.record(ctx, "photo_submission.rejected", "photo_submission", sub.ID.String(), map[string]any{"psychic_id": sub.PsychicID.String(),
		"reason": reason})
	s.d.Tracker.Track(ctx, "psychic.photo_rejected", nil)
	v := adminPhotoView(sub, s.adminPsychic(ctx, sub.PsychicID))
	return &v, nil
}

// ---------------------------------------------------------------------------
// Housekeeping and erasure
// ---------------------------------------------------------------------------

// CleanupPhotos is the scheduler clean-up: rejected photos past their
// retention lose their image and leave the advisor's page, images left
// behind by finished submissions are deleted (retrying earlier failures),
// and soft-deleted rows are purged. It returns the rows it changed.
func (s *AdvisorService) CleanupPhotos(ctx context.Context, now time.Time) (int64, error) {
	if s.d.Photos == nil {
		return 0, nil
	}
	var changed int64
	expired, err := s.d.Photos.ExpiredRejected(ctx, now.Add(-s.cfg.RejectedRetention), 200)
	if err != nil {
		return 0, err
	}
	for i := range expired {
		sub := &expired[i]
		fields := map[string]any{"deleted_at": now, "updated_at": now}
		if sub.ObjectKey != "" && s.d.Private != nil {
			if err := s.d.Private.Delete(ctx, sub.ObjectKey); err != nil {
				warnErr(ctx, "advisor: clean-up delete rejected photo", err, zap.String("submission_id", sub.ID.String()))
			} else {
				fields["object_key"] = ""
			}
		}
		if err := s.d.Photos.Update(ctx, sub.ID, fields); err != nil {
			return changed, err
		}
		if err := s.syncReviewStatus(ctx, sub.PsychicID); err != nil {
			warnErr(ctx, "advisor: clean-up review status", err)
		}
		changed++
	}
	left, err := s.d.Photos.Leftovers(ctx, 200)
	if err != nil {
		return changed, err
	}
	for i := range left {
		sub := &left[i]
		if s.d.Private == nil {
			break
		}
		if err := s.d.Private.Delete(ctx, sub.ObjectKey); err != nil {
			warnErr(ctx, "advisor: clean-up delete leftover photo", err, zap.String("submission_id", sub.ID.String()))
			continue
		}
		if err := s.d.Photos.Update(ctx, sub.ID, map[string]any{"object_key": ""}); err != nil {
			return changed, err
		}
		changed++
	}
	purged, err := s.d.Photos.PurgeDeleted(ctx, now.Add(-s.cfg.PurgeAfter))
	return changed + purged, err
}

// PurgeForUser erases the advisor-settings data of an account being erased
// (UserDataPurger, run inside the eraser's transaction): every photo
// submission and its private image, the published profile photo and the
// sealed phone number. Images are deleted before the rows; when storage
// fails the erasure fails and can be retried, so no image outlives it.
func (s *AdvisorService) PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	var removed int64
	if s.d.Photos != nil {
		keys, err := s.d.Photos.ObjectKeysForUser(ctx, userID)
		if err != nil {
			return 0, err
		}
		for _, key := range keys {
			if s.d.Private == nil {
				break
			}
			if err := s.deleteWithTimeout(ctx, s.d.Private, key); err != nil {
				return 0, apperror.Unavailable(err)
			}
		}
		if removed, err = s.d.Photos.DeleteForUser(ctx, userID); err != nil {
			return 0, err
		}
	}
	p, err := s.d.Psychics.GetByUserID(ctx, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return removed, nil
	}
	if err != nil {
		return removed, err
	}
	if p.ImageKey != "" && s.d.Public != nil {
		if err := s.deleteWithTimeout(ctx, s.d.Public, p.ImageKey); err != nil {
			return removed, apperror.Unavailable(err)
		}
	}
	if p.ImageKey != "" || p.ImageURL != "" || p.PhoneSealed != "" || p.PhotoReviewStatus != domain.PhotoApproved {
		if err := s.d.Psychics.Updates(ctx, userID, map[string]any{"image_url": "", "image_key": "", "phone_sealed": "",
			"photo_review_status": domain.PhotoApproved}); err != nil {
			return removed, err
		}
	}
	return removed, nil
}

func (s *AdvisorService) deleteWithTimeout(ctx context.Context, store storage.ObjectStore, key string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return store.Delete(ctx, key)
}
