package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// ErrNoteFilterRequired rejects unscoped admin note listings: notes are only
// ever looked up for a given client or advisor.
var ErrNoteFilterRequired = errors.New("repository: a client or author filter is required")

// SharedNoteRow is a shared note joined with its author's public identity.
type SharedNoteRow struct {
	domain.ClientNote
	AuthorName  string
	AuthorSlug  string
	AuthorImage string
}

// NoteRepository persists advisors' notes about clients.
type NoteRepository interface {
	Create(ctx context.Context, n *domain.ClientNote) error
	// GetForAuthor returns a live note written by authorID; anything else
	// (another author's note, a deleted note) is ErrNotFound.
	GetForAuthor(ctx context.Context, id, authorID uuid.UUID) (*domain.ClientNote, error)
	// GetForAuthorForUpdate is GetForAuthor with a row lock.
	GetForAuthorForUpdate(ctx context.Context, id, authorID uuid.UUID) (*domain.ClientNote, error)
	// Update writes fields of a live note owned by authorID.
	Update(ctx context.Context, id, authorID uuid.UUID, fields map[string]any) error
	// SoftDelete marks a live note owned by authorID as deleted.
	SoftDelete(ctx context.Context, id, authorID uuid.UUID, at time.Time) error
	// ListByAuthor pages an advisor's live notes about one client (newest first).
	ListByAuthor(ctx context.Context, authorID, clientID uuid.UUID, c *Cursor, limit int) ([]domain.ClientNote, error)
	// CountByAuthor counts an advisor's live notes about one client.
	CountByAuthor(ctx context.Context, authorID, clientID uuid.UUID) (int64, error)
	// ListShared pages live shared notes about a client written by advisors
	// other than excludeAuthor (newest first).
	ListShared(ctx context.Context, clientID, excludeAuthor uuid.UUID, c *Cursor, limit int) ([]SharedNoteRow, error)
	// HasStartedSession reports whether the advisor has had at least one
	// session with the client that actually started.
	HasStartedSession(ctx context.Context, psychicID, clientID uuid.UUID) (bool, error)
	// PurgeForUser hard-deletes every note written by or about a user (account
	// erasure). It returns the number of rows removed.
	PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error)
	// AdminList pages notes for oversight and access requests, private ones
	// included. At least one of ClientID or AuthorID must be set.
	AdminList(ctx context.Context, f NoteAdminFilter) ([]AdminNoteRow, error)
	// AdminSoftDelete removes any live note (moderation).
	AdminSoftDelete(ctx context.Context, id uuid.UUID, at time.Time) (*domain.ClientNote, error)
}

// NoteAdminFilter narrows admin note listings.
type NoteAdminFilter struct {
	ClientID       *uuid.UUID
	AuthorID       *uuid.UUID
	IncludeDeleted bool
	Cursor         *Cursor
	Limit          int
}

// AdminNoteRow is a note with both parties' identities for the console.
type AdminNoteRow struct {
	domain.ClientNote
	ClientName  string
	ClientEmail string
	AuthorName  string
	AuthorSlug  string
}

// GormNoteRepository implements NoteRepository.
type GormNoteRepository struct{ db *gorm.DB }

// NewNoteRepository returns a GORM note repository.
func NewNoteRepository(db *gorm.DB) *GormNoteRepository {
	return &GormNoteRepository{db: db}
}

// Create inserts a note.
func (r *GormNoteRepository) Create(ctx context.Context, n *domain.ClientNote) error {
	return database.Conn(ctx, r.db).Create(n).Error
}

func (r *GormNoteRepository) ownNote(ctx context.Context, id, authorID uuid.UUID) *gorm.DB {
	return database.Conn(ctx, r.db).Where("id = ? AND author_id = ?", id, authorID)
}

// GetForAuthor implements NoteRepository.
func (r *GormNoteRepository) GetForAuthor(ctx context.Context, id, authorID uuid.UUID) (*domain.ClientNote, error) {
	var n domain.ClientNote
	if err := r.ownNote(ctx, id, authorID).First(&n).Error; err != nil {
		return nil, notFound(err)
	}
	return &n, nil
}

// GetForAuthorForUpdate implements NoteRepository.
func (r *GormNoteRepository) GetForAuthorForUpdate(ctx context.Context, id, authorID uuid.UUID) (*domain.ClientNote, error) {
	var n domain.ClientNote
	if err := r.ownNote(ctx, id, authorID).Clauses(forUpdate).First(&n).Error; err != nil {
		return nil, notFound(err)
	}
	return &n, nil
}

// Update implements NoteRepository.
func (r *GormNoteRepository) Update(ctx context.Context, id, authorID uuid.UUID, fields map[string]any) error {
	res := database.Conn(ctx, r.db).Model(&domain.ClientNote{}).Where("id = ? AND author_id = ?", id, authorID).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// SoftDelete implements NoteRepository. The timestamp comes from the caller
// so it follows the application clock.
func (r *GormNoteRepository) SoftDelete(ctx context.Context, id, authorID uuid.UUID, at time.Time) error {
	res := database.Conn(ctx, r.db).Model(&domain.ClientNote{}).Where("id = ? AND author_id = ?", id, authorID).
		Updates(map[string]any{"deleted_at": at, "updated_at": at})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByAuthor implements NoteRepository.
func (r *GormNoteRepository) ListByAuthor(ctx context.Context, authorID, clientID uuid.UUID, c *Cursor, limit int) ([]domain.ClientNote, error) {
	q := database.Conn(ctx, r.db).Model(&domain.ClientNote{}).Where("author_id = ? AND client_id = ?", authorID, clientID)
	q = applyCursor(q, c, "created_at", "id")
	var out []domain.ClientNote
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(limit, 5, 50)).Find(&out).Error
	return out, err
}

// CountByAuthor implements NoteRepository.
func (r *GormNoteRepository) CountByAuthor(ctx context.Context, authorID, clientID uuid.UUID) (int64, error) {
	var n int64
	err := database.Conn(ctx, r.db).Model(&domain.ClientNote{}).Where("author_id = ? AND client_id = ?", authorID, clientID).Count(&n).Error
	return n, err
}

// ListShared implements NoteRepository.
func (r *GormNoteRepository) ListShared(ctx context.Context, clientID, excludeAuthor uuid.UUID, c *Cursor, limit int) ([]SharedNoteRow, error) {
	q := database.Conn(ctx, r.db).Table("client_notes").
		Select("client_notes.*, psychics.name AS author_name, psychics.slug AS author_slug, psychics.image_url AS author_image").
		Joins("JOIN psychics ON psychics.user_id = client_notes.author_id").
		Joins("JOIN users ON users.id = client_notes.author_id").
		Where("client_notes.client_id = ? AND client_notes.author_id <> ?", clientID, excludeAuthor).
		Where("client_notes.shared AND client_notes.deleted_at IS NULL").
		// Notes by erased or banned advisors are no longer shown.
		Where("users.status = ?", domain.UserActive)
	q = applyCursor(q, c, "client_notes.created_at", "client_notes.id")
	var out []SharedNoteRow
	err := q.Order("client_notes.created_at DESC, client_notes.id DESC").Limit(clampLimit(limit, 5, 50)).Scan(&out).Error
	return out, err
}

// HasStartedSession implements NoteRepository.
func (r *GormNoteRepository) HasStartedSession(ctx context.Context, psychicID, clientID uuid.UUID) (bool, error) {
	var n int64
	err := database.Conn(ctx, r.db).Model(&domain.Session{}).
		Where("psychic_id = ? AND client_id = ? AND started_at IS NOT NULL", psychicID, clientID).
		Limit(1).Count(&n).Error
	return n > 0, err
}

// PurgeForUser implements NoteRepository.
func (r *GormNoteRepository) PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	res := database.Conn(ctx, r.db).Unscoped().Where("client_id = ? OR author_id = ?", userID, userID).Delete(&domain.ClientNote{})
	return res.RowsAffected, res.Error
}

// AdminList implements NoteRepository.
func (r *GormNoteRepository) AdminList(ctx context.Context, f NoteAdminFilter) ([]AdminNoteRow, error) {
	if f.ClientID == nil && f.AuthorID == nil {
		return nil, ErrNoteFilterRequired
	}
	q := database.Conn(ctx, r.db).Table("client_notes").
		Select("client_notes.*, clients.display_name AS client_name, clients.email AS client_email, " +
			"psychics.name AS author_name, psychics.slug AS author_slug").
		Joins("JOIN users AS clients ON clients.id = client_notes.client_id").
		Joins("JOIN psychics ON psychics.user_id = client_notes.author_id")
	if f.ClientID != nil {
		q = q.Where("client_notes.client_id = ?", *f.ClientID)
	}
	if f.AuthorID != nil {
		q = q.Where("client_notes.author_id = ?", *f.AuthorID)
	}
	if !f.IncludeDeleted {
		q = q.Where("client_notes.deleted_at IS NULL")
	}
	q = applyCursor(q, f.Cursor, "client_notes.created_at", "client_notes.id")
	var out []AdminNoteRow
	err := q.Order("client_notes.created_at DESC, client_notes.id DESC").Limit(clampLimit(f.Limit, 20, 100)).Scan(&out).Error
	return out, err
}

// AdminSoftDelete implements NoteRepository.
func (r *GormNoteRepository) AdminSoftDelete(ctx context.Context, id uuid.UUID, at time.Time) (*domain.ClientNote, error) {
	var n domain.ClientNote
	conn := database.Conn(ctx, r.db)
	if err := conn.Clauses(forUpdate).Where("id = ?", id).First(&n).Error; err != nil {
		return nil, notFound(err)
	}
	if err := conn.Model(&domain.ClientNote{}).Where("id = ?", id).Updates(map[string]any{"deleted_at": at, "updated_at": at}).Error; err != nil {
		return nil, err
	}
	return &n, nil
}
