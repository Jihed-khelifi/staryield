package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// setAdminCookie writes the admin console session cookie: HttpOnly, Secure
// in production (__Host- prefix, host-only), SameSite=Strict.
func (h *Handler) setAdminCookie(c echo.Context, token string, expires time.Time) {
	c.SetCookie(h.cookie(h.cookies.Names.Admin, token, "/", time.Until(expires), true, http.SameSiteStrictMode))
}

func (h *Handler) clearAdminCookie(c echo.Context) {
	c.SetCookie(h.cookie(h.cookies.Names.Admin, "", "/", 0, true, http.SameSiteStrictMode))
}

func (h *Handler) adminSessionResponse(c echo.Context, issued *service.IssuedAdminSession) error {
	h.setAdminCookie(c, issued.Token, issued.Expires)
	issued.View.CSRFToken = h.issueCSRF(c)
	return ok(c, issued.View)
}

type adminLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AdminLogin verifies an administrator's password and returns a second
// factor challenge.
func (h *Handler) AdminLogin(c echo.Context) error {
	var req adminLoginRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.AdminAuth.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		return err
	}
	return ok(c, out)
}

type challengeRequest struct {
	Challenge    string `json:"challenge"`
	Code         string `json:"code"`
	RecoveryCode string `json:"recovery_code"`
}

// AdminEnroll starts authenticator enrolment.
func (h *Handler) AdminEnroll(c echo.Context) error {
	var req challengeRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.AdminAuth.StartEnrollment(c.Request().Context(), req.Challenge)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminEnrollConfirm finishes enrolment and signs in.
func (h *Handler) AdminEnrollConfirm(c echo.Context) error {
	var req challengeRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	issued, err := h.svc.AdminAuth.ConfirmEnrollment(c.Request().Context(), req.Challenge, req.Code)
	if err != nil {
		return err
	}
	return h.adminSessionResponse(c, issued)
}

// AdminVerify completes sign-in with a TOTP or recovery code.
func (h *Handler) AdminVerify(c echo.Context) error {
	var req challengeRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	issued, err := h.svc.AdminAuth.Verify(c.Request().Context(), req.Challenge, req.Code, req.RecoveryCode)
	if err != nil {
		return err
	}
	return h.adminSessionResponse(c, issued)
}

// AdminSessionInfo returns the current admin session.
func (h *Handler) AdminSessionInfo(c echo.Context) error {
	out, err := h.svc.AdminAuth.Session(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, out)
}

type stepUpRequest struct {
	Code string `json:"code"`
}

// AdminStepUp re-verifies a TOTP code for sensitive actions.
func (h *Handler) AdminStepUp(c echo.Context) error {
	var req stepUpRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.AdminAuth.StepUp(c.Request().Context(), req.Code)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminLogout ends the admin session.
func (h *Handler) AdminLogout(c echo.Context) error {
	err := h.svc.AdminAuth.Logout(c.Request().Context(), refreshCookie(c, h.cookies.Names.Admin))
	h.clearAdminCookie(c)
	if err != nil {
		return err
	}
	return noContent(c)
}

// AdminAuthSessions lists the admin's active console sessions.
func (h *Handler) AdminAuthSessions(c echo.Context) error {
	out, err := h.svc.AdminAuth.Sessions(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"items": orEmpty(out)})
}

// AdminRevokeAuthSession ends one of the admin's console sessions.
func (h *Handler) AdminRevokeAuthSession(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	if err := h.svc.AdminAuth.RevokeSession(c.Request().Context(), id); err != nil {
		return err
	}
	return noContent(c)
}

// AdminRecoveryCodes regenerates recovery codes.
func (h *Handler) AdminRecoveryCodes(c echo.Context) error {
	codes, err := h.svc.AdminAuth.RegenerateRecoveryCodes(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"recovery_codes": codes})
}
