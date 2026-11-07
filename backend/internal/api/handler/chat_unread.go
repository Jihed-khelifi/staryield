package handler

import (
	"github.com/labstack/echo/v4"
)

// ChatUnreadCount returns the caller's unread chat total (revisions 3.0:
// chat unread): {count, conversations}.
func (h *Handler) ChatUnreadCount(c echo.Context) error {
	out, err := h.svc.Chat.UnreadCount(c.Request().Context(), principal(c))
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return ok(c, out)
}
