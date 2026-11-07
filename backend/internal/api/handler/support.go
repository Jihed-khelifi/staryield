package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// CreateSupportTicket stores a contact-form request (anonymous or signed in).
func (h *Handler) CreateSupportTicket(c echo.Context) error {
	var in service.TicketInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Support.Create(c.Request().Context(), principal(c), in)
	if err != nil {
		return err
	}
	return created(c, out)
}

// revisions 3.0: support desk — requester endpoints (section 16.2).

// supportToken reads the private-link token from the X-Support-Token header.
func supportToken(c echo.Context) string { return c.Request().Header.Get("X-Support-Token") }

// MySupportTickets lists the signed-in user's requests.
func (h *Handler) MySupportTickets(c echo.Context) error {
	limit, err := limitParam(c, 20, 50)
	if err != nil {
		return err
	}
	out, err := h.svc.Support.MyTickets(c.Request().Context(), principal(c).UserID, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// MySupportTicket returns one of the signed-in user's requests.
func (h *Handler) MySupportTicket(c echo.Context) error {
	out, err := h.svc.Support.MyTicket(c.Request().Context(), principal(c).UserID, c.Param("reference"))
	if err != nil {
		return err
	}
	return ok(c, out)
}

// ReplyMySupportTicket adds a reply to one of the signed-in user's requests.
func (h *Handler) ReplyMySupportTicket(c echo.Context) error {
	var in service.CustomerReplyInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Support.MyReply(c.Request().Context(), principal(c).UserID, c.Param("reference"), in)
	if err != nil {
		return err
	}
	return created(c, out)
}

// SupportTicketByLink returns a request through its private link.
func (h *Handler) SupportTicketByLink(c echo.Context) error {
	out, err := h.svc.Support.LinkTicket(c.Request().Context(), c.Param("reference"), supportToken(c))
	if err != nil {
		return err
	}
	return ok(c, out)
}

// ReplySupportTicketByLink adds a reply through the private link.
func (h *Handler) ReplySupportTicketByLink(c echo.Context) error {
	var in service.CustomerReplyInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Support.LinkReply(c.Request().Context(), c.Param("reference"), supportToken(c), in)
	if err != nil {
		return err
	}
	return created(c, out)
}
