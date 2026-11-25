package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// Growth: e-mail verification by secure link.

func (h *Handler) emailVerificationService() (*service.EmailVerificationService, error) {
	if h.svc.EmailVerification == nil {
		return nil, apperror.FeatureUnavailable("Account settings are not available.")
	}
	return h.svc.EmailVerification, nil
}

// VerifyEmail consumes the link of the welcome e-mail. It is public: the
// link may be opened on another device, signed out.
func (h *Handler) VerifyEmail(c echo.Context) error {
	svc, err := h.emailVerificationService()
	if err != nil {
		return err
	}
	var req service.TokenInput
	if err := bind(c, &req); err != nil {
		return err
	}
	res, err := svc.Verify(c.Request().Context(), req.Token)
	if err != nil {
		return err
	}
	return ok(c, res)
}

// ResendEmailVerification sends the caller a new link.
func (h *Handler) ResendEmailVerification(c echo.Context) error {
	svc, err := h.emailVerificationService()
	if err != nil {
		return err
	}
	res, err := svc.Resend(c.Request().Context(), principal(c).UserID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusAccepted, res)
}

// AdminResendEmailVerification sends a user a new link (audited).
func (h *Handler) AdminResendEmailVerification(c echo.Context) error {
	svc, err := h.emailVerificationService()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	res, err := svc.AdminResend(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusAccepted, res)
}
