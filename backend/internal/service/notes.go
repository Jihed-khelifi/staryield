package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// Note page sizes.
const (
	notePageDefault = 5
	notePageMax     = 50
)

// Note validation and access messages (translated at the API edge).
const (
	msgNoteBody       = "Write a note between 1 and 2,000 characters."
	msgNoteNeedsMeet  = "You can keep notes about a client after your first session together."
	msgNoteLimit      = "You have reached the note limit for this client. Delete older notes to add new ones."
	msgNotesNoChanges = "Nothing to update."
)

// NoteView is one of the signed-in advisor's own notes.
type NoteView struct {
	ID        uuid.UUID `json:"id"`
	Body      string    `json:"body"`
	Shared    bool      `json:"shared"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NoteAuthorView is the public identity of the advisor who shared a note.
type NoteAuthorView struct {
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// SharedNoteView is a note another advisor shared about the same client. It
// is read-only for the viewer.
type SharedNoteView struct {
	ID        uuid.UUID      `json:"id"`
	Body      string         `json:"body"`
	Author    NoteAuthorView `json:"author"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// NoteInput creates a note. Shared defaults to false (private).
type NoteInput struct {
	Body   string `json:"body"`
	Shared bool   `json:"shared"`
}

// NotePatch edits a note; absent fields are left unchanged.
type NotePatch struct {
	Body   *string `json:"body"`
	Shared *bool   `json:"shared"`
}

// NoteSharingPolicy reports whether a client's notes may be shown to other
// advisors (for example while the client has withdrawn personalisation
// consent). A nil policy allows sharing.
type NoteSharingPolicy func(ctx context.Context, clientID uuid.UUID) (bool, error)

// NoteService lets advisors keep notes about the clients they read for.
type NoteService struct {
	tx       database.TxManager
	notes    repository.NoteRepository
	chat     repository.ChatRepository
	audit    repository.AuditRepository
	sealer   *security.Sealer
	clock    clock.Clock
	ipSecret string
	sharing  NoteSharingPolicy
}

// NewNoteService wires the note service.
func NewNoteService(tx database.TxManager, notes repository.NoteRepository, chat repository.ChatRepository,
	audit repository.AuditRepository, sealer *security.Sealer, clk clock.Clock, ipSecret string) *NoteService {
	return &NoteService{tx: tx, notes: notes, chat: chat, audit: audit, sealer: sealer, clock: clk, ipSecret: ipSecret}
}

// SetSharingPolicy installs the consent check used before shared notes are
// shown to other advisors.
func (s *NoteService) SetSharingPolicy(p NoteSharingPolicy) { s.sharing = p }

// noteAD binds a sealed body to its row and author, so a ciphertext copied
// to another row (or attributed to another advisor) does not open.
func noteAD(id, authorID uuid.UUID) string {
	return "note:" + id.String() + ":" + authorID.String()
}

// cleanNoteBody normalises and validates a note body.
func cleanNoteBody(raw string) (string, error) {
	body := strings.TrimSpace(CleanText(raw))
	if n := utf8.RuneCountInString(body); n < 1 || n > domain.NoteMaxChars {
		return "", apperror.Validation(map[string]string{"body": msgNoteBody})
	}
	return body, nil
}

// clientOf returns the client of a conversation owned by the advisor. Any
// other conversation is reported as not found, so IDs cannot be probed.
func (s *NoteService) clientOf(ctx context.Context, p reqctx.Principal, conversationID uuid.UUID) (*domain.Conversation, error) {
	conv, err := s.chat.GetConversation(ctx, conversationID)
	if err != nil {
		return nil, mapRepoErr(err, "Conversation")
	}
	if conv.PsychicID != p.UserID {
		return nil, apperror.NotFound("Conversation")
	}
	return conv, nil
}

// requireMet ensures the advisor has had at least one started session with
// the client.
func (s *NoteService) requireMet(ctx context.Context, psychicID, clientID uuid.UUID) error {
	met, err := s.notes.HasStartedSession(ctx, psychicID, clientID)
	if err != nil {
		return apperror.Internal(err)
	}
	if !met {
		return apperror.Forbidden(msgNoteNeedsMeet)
	}
	return nil
}

func (s *NoteService) open(n *domain.ClientNote) (string, error) {
	return s.sealer.Open(n.BodyEnc, noteAD(n.ID, n.AuthorID))
}

func (s *NoteService) view(n *domain.ClientNote) (NoteView, error) {
	body, err := s.open(n)
	if err != nil {
		return NoteView{}, err
	}
	return NoteView{ID: n.ID, Body: body, Shared: n.Shared, CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt}, nil
}

// pageLimit clamps a requested page size for note listings.
func pageLimit(limit int) int {
	if limit <= 0 {
		return notePageDefault
	}
	if limit > notePageMax {
		return notePageMax
	}
	return limit
}

// List returns the advisor's own notes about the conversation's client.
func (s *NoteService) List(ctx context.Context, p reqctx.Principal, conversationID uuid.UUID, cursor string, limit int) (Page[NoteView], error) {
	conv, err := s.clientOf(ctx, p, conversationID)
	if err != nil {
		return Page[NoteView]{}, err
	}
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[NoteView]{}, err
	}
	size := pageLimit(limit)
	// One extra row tells us whether another page exists, so the last page
	// never advertises an empty "next".
	rows, err := s.notes.ListByAuthor(ctx, p.UserID, conv.ClientID, c, size+1)
	if err != nil {
		return Page[NoteView]{}, apperror.Internal(err)
	}
	page := Page[NoteView]{Items: make([]NoteView, 0, min(len(rows), size))}
	for i := range rows {
		if i == size {
			last := rows[size-1]
			page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
			break
		}
		v, err := s.view(&rows[i])
		if err != nil {
			warnErr(ctx, "notes: open own note", err)
			continue
		}
		page.Items = append(page.Items, v)
	}
	return page, nil
}

// Create adds a note about the conversation's client.
func (s *NoteService) Create(ctx context.Context, p reqctx.Principal, conversationID uuid.UUID, in NoteInput) (*NoteView, error) {
	body, err := cleanNoteBody(in.Body)
	if err != nil {
		return nil, err
	}
	conv, err := s.clientOf(ctx, p, conversationID)
	if err != nil {
		return nil, err
	}
	if err := s.requireMet(ctx, p.UserID, conv.ClientID); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	n := &domain.ClientNote{ID: domain.NewID(), ClientID: conv.ClientID, AuthorID: p.UserID, ConversationID: &conv.ID,
		BodyChars: utf8.RuneCountInString(body), Shared: in.Shared, CreatedAt: now, UpdatedAt: now}
	if n.BodyEnc, err = s.sealer.Seal(body, noteAD(n.ID, n.AuthorID)); err != nil {
		return nil, apperror.Internal(err)
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		count, err := s.notes.CountByAuthor(ctx, p.UserID, conv.ClientID)
		if err != nil {
			return err
		}
		if count >= domain.NoteMaxPerClient {
			return apperror.Conflict(apperror.CodeLimitReached, msgNoteLimit)
		}
		return s.notes.Create(ctx, n)
	})
	if err != nil {
		return nil, mapNoteErr(err)
	}
	s.record(ctx, "note.created", n, map[string]any{"shared": n.Shared})
	return &NoteView{ID: n.ID, Body: body, Shared: n.Shared, CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt}, nil
}

// Update edits one of the advisor's own notes.
func (s *NoteService) Update(ctx context.Context, p reqctx.Principal, noteID uuid.UUID, patch NotePatch) (*NoteView, error) {
	if patch.Body == nil && patch.Shared == nil {
		return nil, apperror.BadRequest(msgNotesNoChanges)
	}
	var body string
	if patch.Body != nil {
		var err error
		if body, err = cleanNoteBody(*patch.Body); err != nil {
			return nil, err
		}
	}
	var out *NoteView
	var sharedChanged, before bool
	var updated *domain.ClientNote
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		n, err := s.notes.GetForAuthorForUpdate(ctx, noteID, p.UserID)
		if err != nil {
			return err
		}
		before = n.Shared
		now := s.clock.Now()
		fields := map[string]any{"updated_at": now}
		if patch.Body != nil {
			sealed, err := s.sealer.Seal(body, noteAD(n.ID, n.AuthorID))
			if err != nil {
				return err
			}
			fields["body_enc"], fields["body_chars"] = sealed, utf8.RuneCountInString(body)
			n.BodyEnc, n.BodyChars = sealed, utf8.RuneCountInString(body)
		}
		if patch.Shared != nil && *patch.Shared != n.Shared {
			fields["shared"] = *patch.Shared
			n.Shared = *patch.Shared
			sharedChanged = true
		}
		if err := s.notes.Update(ctx, n.ID, p.UserID, fields); err != nil {
			return err
		}
		n.UpdatedAt = now
		v, err := s.view(n)
		if err != nil {
			return err
		}
		out, updated = &v, n
		return nil
	})
	if err != nil {
		return nil, mapNoteErr(err)
	}
	switch {
	case sharedChanged && updated.Shared:
		s.record(ctx, "note.shared", updated, nil)
	case sharedChanged && before:
		s.record(ctx, "note.unshared", updated, nil)
	}
	if patch.Body != nil {
		s.record(ctx, "note.edited", updated, nil)
	}
	return out, nil
}

// Delete soft-deletes one of the advisor's own notes.
func (s *NoteService) Delete(ctx context.Context, p reqctx.Principal, noteID uuid.UUID) error {
	n, err := s.notes.GetForAuthor(ctx, noteID, p.UserID)
	if err != nil {
		return mapNoteErr(err)
	}
	if err := s.notes.SoftDelete(ctx, noteID, p.UserID, s.clock.Now()); err != nil {
		return mapNoteErr(err)
	}
	s.record(ctx, "note.deleted", n, nil)
	return nil
}

// ListShared returns notes other advisors shared about the conversation's
// client. Only advisors who have met the client may read them.
func (s *NoteService) ListShared(ctx context.Context, p reqctx.Principal, conversationID uuid.UUID, cursor string, limit int) (Page[SharedNoteView], error) {
	conv, err := s.clientOf(ctx, p, conversationID)
	if err != nil {
		return Page[SharedNoteView]{}, err
	}
	if err := s.requireMet(ctx, p.UserID, conv.ClientID); err != nil {
		return Page[SharedNoteView]{}, err
	}
	if s.sharing != nil {
		allowed, err := s.sharing(ctx, conv.ClientID)
		if err != nil {
			return Page[SharedNoteView]{}, apperror.Internal(err)
		}
		if !allowed {
			return Page[SharedNoteView]{Items: []SharedNoteView{}}, nil
		}
	}
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[SharedNoteView]{}, err
	}
	size := pageLimit(limit)
	rows, err := s.notes.ListShared(ctx, conv.ClientID, p.UserID, c, size+1)
	if err != nil {
		return Page[SharedNoteView]{}, apperror.Internal(err)
	}
	page := Page[SharedNoteView]{Items: make([]SharedNoteView, 0, min(len(rows), size))}
	for i := range rows {
		if i == size {
			last := rows[size-1]
			page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
			break
		}
		r := &rows[i]
		body, err := s.open(&r.ClientNote)
		if err != nil {
			warnErr(ctx, "notes: open shared note", err)
			continue
		}
		page.Items = append(page.Items, SharedNoteView{ID: r.ID, Body: body, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			Author: NoteAuthorView{Name: r.AuthorName, Slug: r.AuthorSlug, AvatarURL: r.AuthorImage}})
	}
	return page, nil
}

// AdminNoteParty identifies a client or advisor in the console.
type AdminNoteParty struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Email string    `json:"email,omitempty"`
	Slug  string    `json:"slug,omitempty"`
}

// AdminNoteView is a note as shown to administrators handling oversight or a
// client's access request. Private notes are included.
type AdminNoteView struct {
	ID        uuid.UUID      `json:"id"`
	Body      string         `json:"body"`
	Shared    bool           `json:"shared"`
	Client    AdminNoteParty `json:"client"`
	Author    AdminNoteParty `json:"author"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt *time.Time     `json:"deleted_at,omitempty"`
}

// AdminNoteFilter narrows admin note listings; a client or an advisor is
// required so the console never browses notes wholesale.
type AdminNoteFilter struct {
	ClientID       *uuid.UUID
	AuthorID       *uuid.UUID
	IncludeDeleted bool
}

const (
	msgNoteAdminReason = "State why you need to view these notes."
	msgNoteAdminScope  = "Choose a client or an advisor."
	msgNoteRemoveWhy   = "A reason (up to 300 characters) is required."
)

func adminReason(raw, message string) (string, error) {
	reason := CleanLine(raw)
	if reason == "" || utf8.RuneCountInString(reason) > 300 {
		return "", apperror.Validation(map[string]string{"reason": message})
	}
	return reason, nil
}

// AdminList returns notes about a client or by an advisor. Every access is
// audited with the stated reason because notes are sensitive personal data.
func (s *NoteService) AdminList(ctx context.Context, f AdminNoteFilter, reason, cursor string, limit int) (Page[AdminNoteView], error) {
	why, err := adminReason(reason, msgNoteAdminReason)
	if err != nil {
		return Page[AdminNoteView]{}, err
	}
	if f.ClientID == nil && f.AuthorID == nil {
		return Page[AdminNoteView]{}, apperror.Validation(map[string]string{"client_id": msgNoteAdminScope})
	}
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[AdminNoteView]{}, err
	}
	size := clampPage(limit, 20, 100)
	rows, err := s.notes.AdminList(ctx, repository.NoteAdminFilter{ClientID: f.ClientID, AuthorID: f.AuthorID,
		IncludeDeleted: f.IncludeDeleted, Cursor: c, Limit: size + 1})
	if err != nil {
		return Page[AdminNoteView]{}, apperror.Internal(err)
	}
	meta := map[string]any{"reason": why, "include_deleted": f.IncludeDeleted, "count": min(len(rows), size)}
	target := ""
	if f.ClientID != nil {
		meta["client_id"], target = f.ClientID.String(), f.ClientID.String()
	}
	if f.AuthorID != nil {
		meta["author_id"] = f.AuthorID.String()
		if target == "" {
			target = f.AuthorID.String()
		}
	}
	recordAudit(ctx, s.audit, s.clock, s.ipSecret, "notes.viewed", "user", target, meta)

	page := Page[AdminNoteView]{Items: make([]AdminNoteView, 0, min(len(rows), size))}
	for i := range rows {
		if i == size {
			last := rows[size-1]
			page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
			break
		}
		r := &rows[i]
		body, err := s.open(&r.ClientNote)
		if err != nil {
			warnErr(ctx, "notes: open note for admin", err)
			continue
		}
		v := AdminNoteView{ID: r.ID, Body: body, Shared: r.Shared, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			Client: AdminNoteParty{ID: r.ClientID, Name: r.ClientName, Email: r.ClientEmail},
			Author: AdminNoteParty{ID: r.AuthorID, Name: r.AuthorName, Slug: r.AuthorSlug}}
		if r.DeletedAt.Valid {
			at := r.DeletedAt.Time
			v.DeletedAt = &at
		}
		page.Items = append(page.Items, v)
	}
	return page, nil
}

// AdminRemove deletes a note for moderation (soft delete, audited).
func (s *NoteService) AdminRemove(ctx context.Context, id uuid.UUID, reason string) error {
	why, err := adminReason(reason, msgNoteRemoveWhy)
	if err != nil {
		return err
	}
	n, err := s.notes.AdminSoftDelete(ctx, id, s.clock.Now())
	if err != nil {
		return mapNoteErr(err)
	}
	s.record(ctx, "note.removed", n, map[string]any{"reason": why, "author_id": n.AuthorID.String()})
	return nil
}

// PurgeForUser hard-deletes every note written by or about a user. It is
// part of account erasure.
func (s *NoteService) PurgeForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := s.notes.PurgeForUser(ctx, userID)
	return err
}

// record writes an audit entry. The note body is never logged.
func (s *NoteService) record(ctx context.Context, action string, n *domain.ClientNote, meta map[string]any) {
	if meta == nil {
		meta = map[string]any{}
	}
	meta["client_id"] = n.ClientID.String()
	recordAudit(ctx, s.audit, s.clock, s.ipSecret, action, "client_note", n.ID.String(), meta)
}

func mapNoteErr(err error) error {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, repository.ErrNotFound) {
		return apperror.NotFound("Note")
	}
	return apperror.Internal(fmt.Errorf("notes: %w", err))
}
