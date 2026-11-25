package handler

import (
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// growth: e-mail delivery. The admin console's view of the e-mail outbox and
// its test send.

// AdminMailHealth reports the provider and the outbox of the last `days` days
// (default 7, 1..30): counts per template and status, why messages did not go
// out, the backlog and operator alerts. No recipient or body is ever returned.
func (h *Handler) AdminMailHealth(c echo.Context) error {
	days := 7
	if raw := c.QueryParam("days"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return apperror.Validation(map[string]string{"days": "Use a number of days between 1 and 30."})
		}
		days = n
	}
	out, err := h.svc.Mail.Health(c.Request().Context(), days)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminMailTest sends a test e-mail through the provider at once and returns
// the provider's answer (step-up; audited with the masked address).
func (h *Handler) AdminMailTest(c echo.Context) error {
	var in service.MailTestInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Mail.SendTest(c.Request().Context(), in)
	if err != nil {
		return err
	}
	return ok(c, out)
}
