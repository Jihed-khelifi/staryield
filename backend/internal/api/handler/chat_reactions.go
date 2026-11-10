package handler

import (
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
)

// replyToField parses the optional reply_to_id multipart field of a chat
// image upload (revisions 4.0, §23.2).
func replyToField(values map[string]string) (*uuid.UUID, error) {
	raw := strings.TrimSpace(values["reply_to_id"])
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, apperror.Validation(map[string]string{"reply_to_id": "Use a valid message id."})
	}
	return &id, nil
}

// LikeMessage likes a message (idempotent; revisions 4.0, §23.3).
func (h *Handler) LikeMessage(c echo.Context) error {
	return h.setLike(c, true)
}

// UnlikeMessage removes the caller's like (idempotent).
func (h *Handler) UnlikeMessage(c echo.Context) error {
	return h.setLike(c, false)
}

func (h *Handler) setLike(c echo.Context, like bool) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Chat.SetLike(c.Request().Context(), principal(c), id, like)
	if err != nil {
		return err
	}
	return ok(c, out)
}

type aliasRequest struct {
	Alias string `json:"alias"`
}

// SetClientAlias gives the conversation's client a private nickname
// (advisor only; revisions 4.0, §23.4).
func (h *Handler) SetClientAlias(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req aliasRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Chat.SetAlias(c.Request().Context(), principal(c), id, req.Alias)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// ClearClientAlias removes the nickname (idempotent).
func (h *Handler) ClearClientAlias(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	if err := h.svc.Chat.ClearAlias(c.Request().Context(), principal(c), id); err != nil {
		return err
	}
	return noContent(c)
}

// ClientAliases returns the caller's aliases for ?client_ids=<uuid>,<uuid>…
// (1 to 50 ids).
func (h *Handler) ClientAliases(c echo.Context) error {
	invalid := apperror.Validation(map[string]string{"client_ids": "Send between 1 and 50 comma-separated client ids."})
	raw := strings.TrimSpace(c.QueryParam("client_ids"))
	if raw == "" {
		return invalid
	}
	parts := strings.Split(raw, ",")
	if len(parts) > 50 {
		return invalid
	}
	seen := map[uuid.UUID]bool{}
	ids := make([]uuid.UUID, 0, len(parts))
	for _, part := range parts {
		id, err := uuid.Parse(strings.TrimSpace(part))
		if err != nil {
			return invalid
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	items, err := h.svc.Chat.LookupAliases(c.Request().Context(), principal(c), ids)
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"items": items})
}
