package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
)

// ExportSettings tune personal-data exports.
type ExportSettings struct {
	IPHashSecret string
	// DailyLimit is how many exports a user may request per 24 hours.
	DailyLimit int
	// Retention is how long a ready export can be downloaded.
	Retention time.Duration
	// DownloadTTL bounds single-use download tokens.
	DownloadTTL time.Duration
	// Lease is how long a worker owns a job before another may retry it.
	Lease       time.Duration
	MaxAttempts int
	// MaxAttachmentBytes bounds the images copied into one archive.
	MaxAttachmentBytes int64
	// RowLimit bounds every category of records.
	RowLimit int
}

func (c ExportSettings) withDefaults() ExportSettings {
	if c.DailyLimit <= 0 {
		c.DailyLimit = 3
	}
	if c.Retention <= 0 {
		c.Retention = 7 * 24 * time.Hour
	}
	if c.DownloadTTL <= 0 {
		c.DownloadTTL = 5 * time.Minute
	}
	if c.Lease <= 0 {
		c.Lease = 15 * time.Minute
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 3
	}
	if c.MaxAttachmentBytes <= 0 {
		c.MaxAttachmentBytes = 50 << 20
	}
	if c.RowLimit <= 0 {
		c.RowLimit = 50000
	}
	return c
}

// ExportDeps are the export service collaborators. Avatars and ChatImages
// (attachment sources) and Notifier are optional.
type ExportDeps struct {
	Tx         database.TxManager
	Jobs       repository.ExportRepository
	Data       repository.ExportDataRepository
	Users      repository.UserRepository
	Actions    repository.ActionTokenRepository
	Audit      repository.AuditRepository
	Store      storage.ObjectStore // private bucket for archives
	Avatars    storage.ObjectStore
	ChatImages storage.ObjectStore
	Sealer     *security.Sealer
	Hasher     *security.PasswordHasher
	Notifier   Notifier
	Clock      clock.Clock
	Settings   ExportSettings
}

// ExportService builds personal-data exports (GDPR art. 15/20): a worker
// writes a ZIP into a private bucket; the owner downloads it with a
// single-use token for a limited time.
type ExportService struct {
	d    ExportDeps
	cfg  ExportSettings
	wake chan struct{}
}

// NewExportService wires the export service.
func NewExportService(d ExportDeps) *ExportService {
	if d.Notifier == nil {
		d.Notifier = NoopNotifier{}
	}
	return &ExportService{d: d, cfg: d.Settings.withDefaults(), wake: make(chan struct{}, 1)}
}

// Wake signals the worker that a job is waiting.
func (s *ExportService) Wake() <-chan struct{} { return s.wake }

func (s *ExportService) poke() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// ExportView is an export as listed in the privacy settings.
type ExportView struct {
	ID          uuid.UUID  `json:"id"`
	Status      string     `json:"status"`
	RequestedAt time.Time  `json:"requested_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Bytes       int64      `json:"bytes,omitempty"`
	SHA256      string     `json:"sha256,omitempty"`
	Downloads   int        `json:"download_count"`
}

func exportView(j *domain.DataExportJob) ExportView {
	v := ExportView{ID: j.ID, Status: string(j.Status), RequestedAt: j.RequestedAt, CompletedAt: j.CompletedAt, Downloads: j.DownloadCount}
	if j.Status == domain.ExportReady {
		v.ExpiresAt, v.Bytes, v.SHA256 = j.ExpiresAt, j.Bytes, j.SHA256
	}
	return v
}

func (s *ExportService) record(ctx context.Context, action string, j *domain.DataExportJob, meta map[string]any) {
	recordAudit(ctx, s.d.Audit, s.d.Clock, s.cfg.IPHashSecret, action, "data_export", j.ID.String(), meta)
}

func (s *ExportService) reauth(ctx context.Context, userID uuid.UUID, password string) error {
	u, err := s.d.Users.GetByID(ctx, userID)
	if err != nil {
		return mapRepoErr(err, "User")
	}
	if !u.HasPassword() { // created with Google: set a password first
		return apperror.PasswordNotSet()
	}
	if password == "" || len(password) > security.PasswordMaxLength*4 {
		if err := s.d.Hasher.VerifyDummyContext(ctx, password); err != nil {
			return hashError(err)
		}
		return apperror.Validation(map[string]string{"password": "The password is incorrect."})
	}
	if _, err := s.d.Hasher.VerifyContext(ctx, password, u.PasswordHash); err != nil {
		if hasherBusy(err) {
			return hashError(err)
		}
		return apperror.Validation(map[string]string{"password": "The password is incorrect."})
	}
	return nil
}

// Request queues a new export after re-authentication. At most DailyLimit
// exports per 24 hours, and one at a time.
func (s *ExportService) Request(ctx context.Context, userID uuid.UUID, password string) (*ExportView, error) {
	if _, off := s.d.Store.(storage.Disabled); off {
		return nil, apperror.FeatureUnavailable("Data exports are not available yet.")
	}
	if err := s.reauth(ctx, userID, password); err != nil {
		return nil, err
	}
	now := s.d.Clock.Now()
	n, err := s.d.Jobs.CountSince(ctx, userID, now.Add(-24*time.Hour))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if n >= int64(s.cfg.DailyLimit) {
		e := apperror.New(apperror.CodeLimitReached, http.StatusTooManyRequests,
			fmt.Sprintf("You can request up to %d exports a day.", s.cfg.DailyLimit))
		e.RetryAfter = time.Hour
		return nil, e
	}
	j := &domain.DataExportJob{ID: domain.NewID(), UserID: userID, Status: domain.ExportQueued, RequestedAt: now, CreatedAt: now, UpdatedAt: now}
	if err := s.d.Jobs.Create(ctx, j); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, apperror.ExportInProgress()
		}
		return nil, apperror.Internal(err)
	}
	s.record(ctx, "export.requested", j, nil)
	s.poke()
	v := exportView(j)
	return &v, nil
}

// List pages the user's exports, newest first.
func (s *ExportService) List(ctx context.Context, userID uuid.UUID, cursor string, limit int) (Page[ExportView], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[ExportView]{}, err
	}
	size := limit
	if size <= 0 || size > 50 {
		size = 10
	}
	rows, err := s.d.Jobs.List(ctx, userID, c, size+1)
	if err != nil {
		return Page[ExportView]{}, apperror.Internal(err)
	}
	page := Page[ExportView]{Items: make([]ExportView, 0, min(len(rows), size))}
	for i := range rows {
		if i == size {
			last := rows[size-1]
			page.NextCursor = repository.EncodeCursor(last.RequestedAt, last.ID.String())
			break
		}
		page.Items = append(page.Items, exportView(&rows[i]))
	}
	return page, nil
}

// Get returns one of the user's exports (404 for anyone else's).
func (s *ExportService) Get(ctx context.Context, userID, id uuid.UUID) (*ExportView, error) {
	j, err := s.d.Jobs.Get(ctx, userID, id)
	if err != nil {
		return nil, mapRepoErr(err, "Export")
	}
	v := exportView(j)
	return &v, nil
}

// DownloadLink is a single-use download authorisation.
type DownloadLink struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	// URL is the API path to GET (with the session cookie) to download.
	URL string `json:"url"`
}

func (s *ExportService) readyJob(ctx context.Context, userID, id uuid.UUID) (*domain.DataExportJob, error) {
	j, err := s.d.Jobs.Get(ctx, userID, id)
	if err != nil {
		return nil, mapRepoErr(err, "Export")
	}
	if j.Status != domain.ExportReady || j.ExpiresAt == nil || !j.ExpiresAt.After(s.d.Clock.Now()) || j.ObjectKey == "" {
		return nil, apperror.InvalidState("This export is not available for download.")
	}
	return j, nil
}

// DownloadToken re-authenticates and issues a single-use token valid for
// DownloadTTL.
func (s *ExportService) DownloadToken(ctx context.Context, userID, id uuid.UUID, password string) (*DownloadLink, error) {
	if err := s.reauth(ctx, userID, password); err != nil {
		return nil, err
	}
	j, err := s.readyJob(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	raw, err := security.RandomToken(32)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	now := s.d.Clock.Now()
	exp := now.Add(s.cfg.DownloadTTL)
	jobID := j.ID
	if err := s.d.Actions.Create(ctx, &domain.ActionToken{ID: domain.NewID(), UserID: userID, Purpose: domain.ActionExportDownload,
		TokenHash: security.HashToken(raw), SubjectID: &jobID, ExpiresAt: exp, CreatedAt: now, UpdatedAt: now}); err != nil {
		return nil, apperror.Internal(err)
	}
	s.record(ctx, "export.download_authorized", j, nil)
	return &DownloadLink{Token: raw, ExpiresAt: exp, URL: "/api/v1/me/exports/" + j.ID.String() + "/download?token=" + raw}, nil
}

// ExportDownload is an archive being streamed to its owner.
type ExportDownload struct {
	Body     io.ReadCloser
	Size     int64
	Filename string
}

// Open consumes a download token and opens the archive. The stream is
// detached from the request deadline (large files); the caller closes it.
func (s *ExportService) Open(ctx context.Context, userID, id uuid.UUID, raw string) (*ExportDownload, error) {
	if !validRawToken(raw) {
		return nil, apperror.InvalidToken("This download link is invalid or has expired.")
	}
	now := s.d.Clock.Now()
	var job *domain.DataExportJob
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		t, err := s.d.Actions.Consume(ctx, security.HashToken(raw), domain.ActionExportDownload, now)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return apperror.InvalidToken("This download link is invalid or has expired.")
			}
			return err
		}
		if t.UserID != userID || t.SubjectID == nil || *t.SubjectID != id {
			return apperror.InvalidToken("This download link is invalid or has expired.")
		}
		if job, err = s.readyJob(ctx, userID, id); err != nil {
			return err
		}
		return s.d.Jobs.IncrementDownloads(ctx, id, now)
	})
	if err != nil {
		if _, ok := apperror.As(err); ok {
			return nil, err
		}
		return nil, apperror.Internal(err)
	}
	// The stream outlives the request deadline but stays bounded.
	octx, cancel := context.WithTimeout(context.WithoutCancel(ctx), exportStreamTimeout)
	var body io.ReadCloser
	var size int64
	if st, ok := s.d.Store.(storage.Streamer); ok {
		body, size, _, err = st.Open(octx, job.ObjectKey)
	} else {
		var b []byte
		b, _, err = s.d.Store.Get(octx, job.ObjectKey, 0)
		body, size = io.NopCloser(bytes.NewReader(b)), int64(len(b))
	}
	if err != nil {
		cancel()
		if errors.Is(err, storage.ErrNotFound) {
			return nil, apperror.InvalidState("This export is not available for download.")
		}
		return nil, apperror.Unavailable(err)
	}
	s.record(ctx, "export.downloaded", job, map[string]any{"bytes": size})
	return &ExportDownload{Body: cancelOnClose{ReadCloser: body, cancel: cancel}, Size: size,
		Filename: "staryield-export-" + job.RequestedAt.UTC().Format("2006-01-02") + ".zip"}, nil
}

// exportStreamTimeout bounds one archive download.
const exportStreamTimeout = 30 * time.Minute

// cancelOnClose releases the stream's context when the body is closed.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c cancelOnClose) Close() error {
	defer c.cancel()
	return c.ReadCloser.Close()
}

// RunDue builds the exports waiting in the queue (worker job).
func (s *ExportService) RunDue(ctx context.Context) error {
	var errs []error
	for range 5 {
		var jobs []domain.DataExportJob
		now := s.d.Clock.Now()
		err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
			var err error
			jobs, err = s.d.Jobs.ClaimDue(ctx, now, s.cfg.Lease, 2)
			return err
		})
		if err != nil {
			return err
		}
		if len(jobs) == 0 {
			break
		}
		for i := range jobs {
			if err := s.run(ctx, &jobs[i]); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (s *ExportService) run(ctx context.Context, j *domain.DataExportJob) error {
	key, size, sum, err := s.build(ctx, j)
	now := s.d.Clock.Now()
	if err != nil {
		warnErr(ctx, "export: build failed", err, zap.String("export_id", j.ID.String()), zap.Int("attempt", j.Attempts))
		fields := map[string]any{"status": domain.ExportQueued, "locked_until": nil, "error": "build_failed", "updated_at": now}
		if j.Attempts >= s.cfg.MaxAttempts {
			fields["status"], fields["completed_at"] = domain.ExportFailed, now
		}
		if serr := s.d.Jobs.Save(ctx, j.ID, fields); serr != nil {
			return errors.Join(err, serr)
		}
		if j.Attempts >= s.cfg.MaxAttempts {
			s.record(database.Detach(ctx), "export.failed", j, nil)
		}
		return nil
	}
	exp := now.Add(s.cfg.Retention)
	if err := s.d.Jobs.Save(ctx, j.ID, map[string]any{"status": domain.ExportReady, "object_key": key, "bytes": size, "sha256": sum,
		"completed_at": now, "expires_at": exp, "locked_until": nil, "error": "", "updated_at": now}); err != nil {
		warnErr(ctx, "export: delete unrecorded archive", s.d.Store.Delete(ctx, key))
		return err
	}
	s.record(ctx, "export.ready", j, map[string]any{"bytes": size})
	s.d.Notifier.Notify(ctx, NotificationSpec{Kind: NotifyExportReady, Data: map[string]any{"export_id": j.ID.String(),
		"expires_at": exp.Format(time.RFC3339)}, Link: "/profile/settings#privacy"}, j.UserID)
	return nil
}

func (s *ExportService) build(ctx context.Context, j *domain.DataExportJob) (string, int64, string, error) {
	bundle, err := s.collect(ctx, j.UserID)
	if err != nil {
		return "", 0, "", err
	}
	var buf bytes.Buffer
	if err := WriteExportArchive(&buf, bundle); err != nil {
		return "", 0, "", err
	}
	rnd, err := security.RandomToken(16)
	if err != nil {
		return "", 0, "", err
	}
	key := "exports/" + j.UserID.String() + "/" + j.ID.String() + "/" + rnd + ".zip"
	if _, err := s.d.Store.Put(ctx, key, "application/zip", buf.Bytes(), "private, no-store"); err != nil {
		return "", 0, "", err
	}
	sum := sha256.Sum256(buf.Bytes())
	return key, int64(buf.Len()), hex.EncodeToString(sum[:]), nil
}

// PurgeExpired deletes archives past their retention (housekeeping).
func (s *ExportService) PurgeExpired(ctx context.Context, now time.Time) (int64, error) {
	var n int64
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		jobs, err := s.d.Jobs.ClaimExpired(ctx, now, 50)
		if err != nil {
			return err
		}
		for _, j := range jobs {
			if j.ObjectKey != "" {
				if err := s.d.Store.Delete(ctx, j.ObjectKey); err != nil {
					return err
				}
			}
			if err := s.d.Jobs.Save(ctx, j.ID, map[string]any{"status": domain.ExportExpired, "object_key": "", "updated_at": now}); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}
