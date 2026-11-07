package handler

// revisions 3.0: support desk — administrator endpoints (section 16.1).

import (
	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// AdminSupportTickets pages the support queue (metadata only).
func (h *Handler) AdminSupportTickets(c echo.Context) error {
	limit, err := limitParam(c, 25, 100)
	if err != nil {
		return err
	}
	out, err := h.svc.Support.AdminTickets(c.Request().Context(), principal(c).UserID, service.SupportQuery{
		Status: c.QueryParam("status"), Topic: c.QueryParam("topic"), Priority: c.QueryParam("priority"),
		Assignee: c.QueryParam("assignee"), UserID: c.QueryParam("user_id"), Q: c.QueryParam("q"), Sort: c.QueryParam("sort"),
		Cursor: c.QueryParam("cursor"), Limit: limit})
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminSupportSummary returns the queue counters.
func (h *Handler) AdminSupportSummary(c echo.Context) error {
	out, err := h.svc.Support.AdminSummary(c.Request().Context(), principal(c).UserID)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminSupportTicket decrypts one ticket and its thread (step-up).
func (h *Handler) AdminSupportTicket(c echo.Context) error {
	out, err := h.svc.Support.AdminTicketDetail(c.Request().Context(), principal(c).UserID, c.Param("id"))
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminSupportReply adds a public reply or an internal note (step-up).
func (h *Handler) AdminSupportReply(c echo.Context) error {
	var in service.AdminReplyInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Support.AdminReply(c.Request().Context(), principal(c).UserID, c.Param("id"), in)
	if err != nil {
		return err
	}
	return created(c, out)
}

// AdminSupportUpdate changes status, priority and assignee.
func (h *Handler) AdminSupportUpdate(c echo.Context) error {
	var in service.AdminUpdateInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Support.AdminUpdate(c.Request().Context(), principal(c).UserID, c.Param("id"), in)
	if err != nil {
		return err
	}
	return ok(c, out)
}
