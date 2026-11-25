package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// AdminAutoAway returns the advisors' auto-away setting.
func (h *Handler) AdminAutoAway(c echo.Context) error {
	out, err := h.svc.AutoAway.Settings(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminUpdateAutoAway changes the advisors' auto-away setting.
func (h *Handler) AdminUpdateAutoAway(c echo.Context) error {
	var in service.AutoAwayInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.AutoAway.UpdateSettings(c.Request().Context(), in)
	if err != nil {
		return err
	}
	return ok(c, out)
}
