package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// PushConfig returns whether Web Push is available and the VAPID public key
// browsers subscribe with.
func (h *Handler) PushConfig(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "public, max-age=300")
	return ok(c, h.svc.Push.Config())
}

// SubscribePush registers the caller's browser (PushSubscription.toJSON()).
func (h *Handler) SubscribePush(c echo.Context) error {
	var in service.PushSubscriptionInput
	if err := bind(c, &in); err != nil {
		return err
	}
	v, err := h.svc.Push.Subscribe(c.Request().Context(), principal(c).UserID, in, c.Request().UserAgent())
	if err != nil {
		return err
	}
	return created(c, v)
}

// UnsubscribePush removes the caller's browser subscription.
func (h *Handler) UnsubscribePush(c echo.Context) error {
	var in struct {
		Endpoint string `json:"endpoint"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	if err := h.svc.Push.Unsubscribe(c.Request().Context(), principal(c).UserID, in.Endpoint); err != nil {
		return err
	}
	return noContent(c)
}
