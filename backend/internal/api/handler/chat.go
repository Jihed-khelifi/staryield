package handler

import (
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// seqParam parses an optional non-negative sequence number query parameter
// (minimum min).
func seqParam(c echo.Context, name string, min int64) (int64, bool, error) {
	raw := c.QueryParam(name)
	if raw == "" {
		return 0, false, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < min {
		msg := "Use a whole number, 0 or more."
		if min > 0 {
			msg = "Use a positive whole number."
		}
		return 0, false, apperror.Validation(map[string]string{name: msg})
	}
	return n, true, nil
}

// Conversations lists one page of the inbox (?cursor=&limit=, at most 50).
func (h *Handler) Conversations(c echo.Context) error {
	limit, err := limitParam(c, 50, 50)
	if err != nil {
		return err
	}
	page, err := h.svc.Chat.Conversations(c.Request().Context(), principal(c), c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	page.Items = orEmpty(page.Items)
	return ok(c, page)
}

// Messages returns a page of history: ?before_seq= (newest page when
// absent), or the legacy opaque ?before= / ?cursor=.
func (h *Handler) Messages(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 50, 200)
	if err != nil {
		return err
	}
	beforeSeq, _, err := seqParam(c, "before_seq", 1)
	if err != nil {
		return err
	}
	cursor := c.QueryParam("before")
	if cursor == "" {
		cursor = c.QueryParam("cursor")
	}
	page, err := h.svc.Chat.Messages(c.Request().Context(), principal(c), id,
		service.MessagesQuery{BeforeSeq: beforeSeq, Cursor: cursor, Limit: limit})
	if err != nil {
		return err
	}
	page.Items = orEmpty(page.Items)
	return ok(c, page)
}

// MessagesSince returns messages after ?after_seq= (gap filling), or after
// the legacy RFC 3339 ?after= timestamp.
func (h *Handler) MessagesSince(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 200, 500)
	if err != nil {
		return err
	}
	afterSeq, hasSeq, err := seqParam(c, "after_seq", 0)
	if err != nil {
		return err
	}
	q := service.SinceQuery{AfterSeq: afterSeq, Limit: limit}
	if !hasSeq {
		after, err := optionalTime(c, "after")
		if err != nil {
			return err
		}
		if after == nil {
			return apperror.Validation(map[string]string{"after_seq": "This parameter is required."})
		}
		q.After = after
	}
	page, err := h.svc.Chat.MessagesSince(c.Request().Context(), principal(c), id, q)
	if err != nil {
		return err
	}
	page.Items = orEmpty(page.Items)
	return ok(c, page)
}

type readRequest struct {
	MessageID *uuid.UUID `json:"message_id"`
	Seq       *int64     `json:"seq"`
}

// MarkRead advances the read position to {"seq"} or {"message_id"} and
// returns the caller's read state.
func (h *Handler) MarkRead(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req readRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	res, err := h.svc.Chat.MarkRead(c.Request().Context(), principal(c), id, service.ReadInput{Seq: req.Seq, MessageID: req.MessageID})
	if err != nil {
		return err
	}
	return ok(c, res)
}

// Thread returns the chatroom state with one advisor.
func (h *Handler) Thread(c echo.Context) error {
	out, err := h.svc.Chat.ThreadWithPsychic(c.Request().Context(), principal(c), c.Param("slug"))
	if err != nil {
		return err
	}
	return ok(c, out)
}

type startSessionRequest struct {
	PsychicSlug string `json:"psychic_slug"`
	RequestKey  string `json:"request_key"`
}

// StartSession requests a live chat (FR-09).
func (h *Handler) StartSession(c echo.Context) error {
	var req startSessionRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	if req.PsychicSlug == "" {
		return apperror.Validation(map[string]string{"psychic_slug": "Choose an advisor."})
	}
	out, err := h.svc.Chat.RequestSession(c.Request().Context(), principal(c).UserID, req.PsychicSlug, req.RequestKey)
	if err != nil {
		return err
	}
	return created(c, out)
}

// Sessions lists past and current sessions (FR-12).
func (h *Handler) Sessions(c echo.Context) error {
	statuses, err := service.ParseSessionStatuses(c.QueryParam("status"))
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 20, 100)
	if err != nil {
		return err
	}
	page, err := h.svc.Chat.Sessions(c.Request().Context(), principal(c), statuses, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, page)
}

// Session returns one session.
func (h *Handler) Session(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Chat.Session(c.Request().Context(), principal(c), id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// SessionSummary returns the end-of-session recap.
func (h *Handler) SessionSummary(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Chat.Summary(c.Request().Context(), principal(c), id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AcceptSession accepts a request (psychic).
func (h *Handler) AcceptSession(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Chat.AcceptSession(c.Request().Context(), principal(c).UserID, id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// DeclineSession declines a request (psychic).
func (h *Handler) DeclineSession(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Chat.DeclineSession(c.Request().Context(), principal(c).UserID, id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// CancelSession withdraws a pending request (client).
func (h *Handler) CancelSession(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Chat.CancelSession(c.Request().Context(), principal(c).UserID, id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// EndSession ends a session (either participant).
func (h *Handler) EndSession(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Chat.EndSession(c.Request().Context(), principal(c), id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

type sendMessageRequest struct {
	Body        string `json:"body"`
	ClientMsgID string `json:"client_msg_id"`
}

// SendMessage is the REST fallback for sending (SSE clients).
func (h *Handler) SendMessage(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req sendMessageRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	msg, err := h.svc.Chat.SendMessage(c.Request().Context(), principal(c), service.SendMessageInput{SessionID: id, Body: req.Body, ClientMsgID: req.ClientMsgID})
	if err != nil {
		return err
	}
	return created(c, map[string]any{"message": msg})
}

type typingRequest struct {
	Typing bool `json:"typing"`
}

// Typing is the REST fallback for typing indicators.
func (h *Handler) Typing(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req typingRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	if err := h.svc.Chat.Typing(c.Request().Context(), principal(c), id, req.Typing); err != nil {
		return err
	}
	return noContent(c)
}

// SavedMessages lists one page of bookmarks (?conversation_id=&cursor=&limit=,
// most recently saved first; next_cursor only when another page exists).
func (h *Handler) SavedMessages(c echo.Context) error {
	conv, err := optionalUUID(c, "conversation_id")
	if err != nil {
		return err
	}
	limit, err := limitParam(c, service.BookmarksPageDefault, service.BookmarksPageMax)
	if err != nil {
		return err
	}
	page, err := h.svc.Chat.Bookmarks(c.Request().Context(), principal(c), conv, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	page.Items = orEmpty(page.Items)
	return ok(c, page)
}

// SaveMessage bookmarks a message.
func (h *Handler) SaveMessage(c echo.Context) error {
	id, err := uuidParam(c, "messageId")
	if err != nil {
		return err
	}
	if err := h.svc.Chat.Bookmark(c.Request().Context(), principal(c), id, true); err != nil {
		return err
	}
	return noContent(c)
}

// UnsaveMessage removes a bookmark.
func (h *Handler) UnsaveMessage(c echo.Context) error {
	id, err := uuidParam(c, "messageId")
	if err != nil {
		return err
	}
	if err := h.svc.Chat.Bookmark(c.Request().Context(), principal(c), id, false); err != nil {
		return err
	}
	return noContent(c)
}

// RealtimeTicket issues a single-use WebSocket ticket.
func (h *Handler) RealtimeTicket(c echo.Context) error {
	ticket, ttl, err := h.svc.Auth.IssueRealtimeTicket(c.Request().Context(), principal(c))
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"ticket": ticket, "expires_in": int(ttl / time.Second), "path": "/api/v1/realtime/ws"})
}

// RealtimeWS upgrades to a WebSocket (ticket-authenticated).
func (h *Handler) RealtimeWS(c echo.Context) error {
	h.realtime.ServeWS(c.Response(), c.Request())
	return nil
}

// RealtimeStream serves the SSE fallback.
func (h *Handler) RealtimeStream(c echo.Context) error {
	p := principal(c)
	h.realtime.ServeSSE(c.Response(), c.Request(), p.UserID, string(p.Role))
	return nil
}

type presenceRequest struct {
	ConnectionID string    `json:"connection_id"`
	SessionID    uuid.UUID `json:"session_id"`
	Action       string    `json:"action"`
}

// RealtimePresence joins or leaves a session for an SSE connection.
func (h *Handler) RealtimePresence(c echo.Context) error {
	var req presenceRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	if req.SessionID == uuid.Nil || req.ConnectionID == "" || len(req.ConnectionID) > 64 {
		return apperror.Validation(map[string]string{"session_id": "session_id and connection_id are required."})
	}
	if req.Action != "join" && req.Action != "leave" {
		return apperror.Validation(map[string]string{"action": "Use join or leave."})
	}
	out, err := h.svc.Gateway.JoinFromREST(c.Request().Context(), principal(c), req.ConnectionID, req.SessionID, req.Action == "join")
	if err != nil {
		return err
	}
	return ok(c, out)
}
