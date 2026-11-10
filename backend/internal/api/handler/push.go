package handler

import (
	"bufio"
	"errors"
	"io"
	"net/http"

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

// logoutPushEndpoint reads the optional logout body {push_endpoint?: string}.
// An empty or absent body is valid (a plain sign-out); a body must be strict
// JSON like every other request.
func logoutPushEndpoint(c echo.Context) (string, error) {
	req := c.Request()
	if req.Body == nil || req.Body == http.NoBody {
		return "", nil
	}
	br := bufio.NewReader(req.Body)
	if _, err := br.Peek(1); errors.Is(err, io.EOF) {
		return "", nil
	}
	req.Body = struct {
		io.Reader
		io.Closer
	}{br, req.Body}
	var in struct {
		PushEndpoint string `json:"push_endpoint"`
	}
	if err := bind(c, &in); err != nil {
		return "", err
	}
	return in.PushEndpoint, nil
}
