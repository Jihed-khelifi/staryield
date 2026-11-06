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
