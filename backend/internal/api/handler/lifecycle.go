package handler

import (
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// MyOnboarding returns the signed-in client's onboarding checklist.
func (h *Handler) MyOnboarding(c echo.Context) error {
	out, err := h.svc.Lifecycle.Onboarding(c.Request().Context(), principal(c).UserID)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// DismissOnboarding hides the checklist.
func (h *Handler) DismissOnboarding(c echo.Context) error {
	out, err := h.svc.Lifecycle.DismissOnboarding(c.Request().Context(), principal(c).UserID, true)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// RestoreOnboarding shows the checklist again (undo of the dismissal).
func (h *Handler) RestoreOnboarding(c echo.Context) error {
	out, err := h.svc.Lifecycle.DismissOnboarding(c.Request().Context(), principal(c).UserID, false)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminLifecycle returns the lifecycle messaging setting with its statistics.
func (h *Handler) AdminLifecycle(c echo.Context) error {
	out, err := h.svc.Lifecycle.Lifecycle(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminUpdateLifecycle changes the lifecycle messaging setting.
func (h *Handler) AdminUpdateLifecycle(c echo.Context) error {
	var in service.LifecycleInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Lifecycle.UpdateLifecycle(c.Request().Context(), in)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminLifecycleFunnel returns the activation funnel of the recent sign-ups.
func (h *Handler) AdminLifecycleFunnel(c echo.Context) error {
	days := 30
	if raw := c.QueryParam("days"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return apperror.Validation(map[string]string{"days": "Choose a window between 1 and 90 days."})
		}
		days = n
	}
	out, err := h.svc.Lifecycle.Funnel(c.Request().Context(), days)
	if err != nil {
		return err
	}
	return ok(c, out)
}
