package handler

import (
	"github.com/labstack/echo/v4"
)

// --- Advisor waiting lists (clients) ------------------------------------------------

// JoinWaitlist puts the signed-in client on an advisor's waiting list. It is
// idempotent: joining a list the client is already on returns that entry.
func (h *Handler) JoinWaitlist(c echo.Context) error {
	p := principal(c)
	out, err := h.svc.Waitlist.Join(c.Request().Context(), p.UserID, p.Role, c.Param("slug"))
	if err != nil {
		return err
	}
	return ok(c, out)
}

// LeaveWaitlist takes the signed-in client off an advisor's waiting list.
func (h *Handler) LeaveWaitlist(c echo.Context) error {
	if err := h.svc.Waitlist.Leave(c.Request().Context(), principal(c).UserID, c.Param("slug")); err != nil {
		return err
	}
	return noContent(c)
}

// MyWaitlist pages the advisors the signed-in user is waiting for.
func (h *Handler) MyWaitlist(c echo.Context) error {
	limit, err := limitParam(c, 20, 50)
	if err != nil {
		return err
	}
	out, err := h.svc.Waitlist.List(c.Request().Context(), principal(c).UserID, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}
