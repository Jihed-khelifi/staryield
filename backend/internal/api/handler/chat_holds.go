package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 6.0 (§26.1): the free-trial hold.

type holdKeyRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
}

// ContinueHold converts the client's held reading now (a top-up in time).
func (h *Handler) ContinueHold(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req holdKeyRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Chat.ContinueHold(c.Request().Context(), principal(c).UserID, id, req.IdempotencyKey)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// RevealHold charges one minute to read the replies an expired hold hid.
func (h *Handler) RevealHold(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req holdKeyRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Chat.RevealHold(c.Request().Context(), principal(c).UserID, id, req.IdempotencyKey)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// GetHold returns a hold to its participants.
func (h *Handler) GetHold(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Chat.GetHold(c.Request().Context(), principal(c), id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminTrialHold returns the free-trial hold setting with its 30-day figures.
func (h *Handler) AdminTrialHold(c echo.Context) error {
	out, err := h.svc.Site.AdminTrialHold(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminUpdateTrialHold replaces the setting.
func (h *Handler) AdminUpdateTrialHold(c echo.Context) error {
	var in service.TrialHoldInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Site.UpdateTrialHold(c.Request().Context(), in)
	if err != nil {
		return err
	}
	return ok(c, out)
}
