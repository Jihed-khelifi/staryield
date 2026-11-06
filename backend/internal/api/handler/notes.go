package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// --- Client notes (advisors only) -----------------------------------------------------

// ConversationNotes lists the advisor's own notes about the conversation's client.
func (h *Handler) ConversationNotes(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 5, 50)
	if err != nil {
		return err
	}
	out, err := h.svc.Notes.List(c.Request().Context(), principal(c), id, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// CreateConversationNote adds a note about the conversation's client.
func (h *Handler) CreateConversationNote(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var in service.NoteInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Notes.Create(c.Request().Context(), principal(c), id, in)
	if err != nil {
		return err
	}
	return created(c, out)
}

// UpdateNote edits one of the advisor's own notes.
func (h *Handler) UpdateNote(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var in service.NotePatch
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Notes.Update(c.Request().Context(), principal(c), id, in)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// DeleteNote soft-deletes one of the advisor's own notes.
func (h *Handler) DeleteNote(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	if err := h.svc.Notes.Delete(c.Request().Context(), principal(c), id); err != nil {
		return err
	}
	return noContent(c)
}

// SharedConversationNotes lists notes other advisors shared about the
// conversation's client.
func (h *Handler) SharedConversationNotes(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 5, 50)
	if err != nil {
		return err
	}
	out, err := h.svc.Notes.ListShared(c.Request().Context(), principal(c), id, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// --- Client notes: administrator oversight -------------------------------------------

// AdminClientNotes lists notes about a client or by an advisor (step-up, audited).
func (h *Handler) AdminClientNotes(c echo.Context) error {
	clientID, err := optionalUUID(c, "client_id")
	if err != nil {
		return err
	}
	authorID, err := optionalUUID(c, "author_id")
	if err != nil {
		return err
	}
	includeDeleted, err := optionalBool(c, "include_deleted")
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 20, 100)
	if err != nil {
		return err
	}
	f := service.AdminNoteFilter{ClientID: clientID, AuthorID: authorID, IncludeDeleted: includeDeleted != nil && *includeDeleted}
	out, err := h.svc.Notes.AdminList(c.Request().Context(), f, c.QueryParam("reason"), c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminRemoveClientNote removes a note for moderation (step-up, audited).
func (h *Handler) AdminRemoveClientNote(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req reasonRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	if err := h.svc.Notes.AdminRemove(c.Request().Context(), id, req.Reason); err != nil {
		return err
	}
	return noContent(c)
}
