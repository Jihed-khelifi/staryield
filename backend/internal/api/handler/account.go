package handler

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// Account self-service endpoints (identity module): settings, signed-in
// devices, e-mail change, consents, data exports, deactivation and closure.

func (h *Handler) accountService() (*service.AccountService, error) {
	if h.svc.Account == nil {
		return nil, apperror.FeatureUnavailable("Account settings are not available.")
	}
	return h.svc.Account, nil
}

// Settings returns the whole settings document.
func (h *Handler) Settings(c echo.Context) error {
	acc, err := h.accountService()
	if err != nil {
		return err
	}
	v, err := acc.Settings(c.Request().Context(), principal(c).UserID)
	if err != nil {
		return err
	}
	return ok(c, v)
}

// UpdateSettings saves a partial settings document in one transaction.
func (h *Handler) UpdateSettings(c echo.Context) error {
	acc, err := h.accountService()
	if err != nil {
		return err
	}
	var req service.SettingsPatch
	if err := bind(c, &req); err != nil {
		return err
	}
	v, err := acc.UpdateSettings(c.Request().Context(), principal(c).UserID, req)
	if err != nil {
		return err
	}
	return ok(c, v)
}

// Devices lists the signed-in devices.
func (h *Handler) Devices(c echo.Context) error {
	limit, err := limitParam(c, 20, 50)
	if err != nil {
		return err
	}
	page, err := h.svc.Auth.Devices(c.Request().Context(), principal(c), c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	page.Items = orEmpty(page.Items)
	return ok(c, page)
}

// RevokeDevice signs one device out (204). Signing out the current device
// also clears this browser's cookies.
func (h *Handler) RevokeDevice(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return apperror.NotFound("Device")
	}
	p := principal(c)
	if err := h.svc.Auth.RevokeDevice(c.Request().Context(), p, id); err != nil {
		return err
	}
	if id == p.FamilyID {
		h.clearSession(c)
	}
	return noContent(c)
}

// RevokeDevices signs out every other device (?scope=others).
func (h *Handler) RevokeDevices(c echo.Context) error {
	if c.QueryParam("scope") != "others" {
		return apperror.Validation(map[string]string{"scope": "Use scope=others to sign out your other devices."})
	}
	n, err := h.svc.Auth.RevokeOtherDevices(c.Request().Context(), principal(c))
	if err != nil {
		return err
	}
	return ok(c, map[string]int{"signed_out": n})
}

// RequestEmailChange always answers 202 (no account enumeration).
func (h *Handler) RequestEmailChange(c echo.Context) error {
	acc, err := h.accountService()
	if err != nil {
		return err
	}
	var req service.EmailChangeInput
	if err := bind(c, &req); err != nil {
		return err
	}
	if err := acc.RequestEmailChange(c.Request().Context(), principal(c).UserID, req); err != nil {
		return err
	}
	return c.JSON(http.StatusAccepted, map[string]string{"status": "pending_confirmation"})
}

func (h *Handler) tokenRequest(c echo.Context) (string, error) {
	var req service.TokenInput
	if err := bind(c, &req); err != nil {
		return "", err
	}
	return req.Token, nil
}

// ConfirmEmailChange completes a change from the e-mailed link. Every
// session ends, including this browser's.
func (h *Handler) ConfirmEmailChange(c echo.Context) error {
	acc, err := h.accountService()
	if err != nil {
		return err
	}
	token, err := h.tokenRequest(c)
	if err != nil {
		return err
	}
	res, err := acc.ConfirmEmailChange(c.Request().Context(), token)
	if err != nil {
		return err
	}
	h.clearSession(c)
	return ok(c, res)
}

// CancelEmailChange cancels (or, once confirmed, undoes) a change from the
// link sent to the previous address.
func (h *Handler) CancelEmailChange(c echo.Context) error {
	acc, err := h.accountService()
	if err != nil {
		return err
	}
	token, err := h.tokenRequest(c)
	if err != nil {
		return err
	}
	res, err := acc.CancelEmailChange(c.Request().Context(), token)
	if err != nil {
		return err
	}
	if res.Status == "restored" {
		h.clearSession(c)
	}
	return ok(c, res)
}

func (h *Handler) consentService() (*service.ConsentService, error) {
	if h.svc.Consents == nil {
		return nil, apperror.FeatureUnavailable("Privacy preferences are not available.")
	}
	return h.svc.Consents, nil
}

// ConsentPolicy describes the consent categories (public).
func (h *Handler) ConsentPolicy(c echo.Context) error {
	cs, err := h.consentService()
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=300")
	return ok(c, cs.Policy())
}

// Consents returns the caller's consent choices.
func (h *Handler) Consents(c echo.Context) error {
	cs, err := h.consentService()
	if err != nil {
		return err
	}
	v, err := cs.Get(c.Request().Context(), principal(c).UserID)
	if err != nil {
		return err
	}
	return ok(c, v)
}

// UpdateConsents records new consent choices.
func (h *Handler) UpdateConsents(c echo.Context) error {
	cs, err := h.consentService()
	if err != nil {
		return err
	}
	var req service.ConsentInput
	if err := bind(c, &req); err != nil {
		return err
	}
	v, err := cs.Set(c.Request().Context(), principal(c).UserID, req)
	if err != nil {
		return err
	}
	return ok(c, v)
}

func (h *Handler) exportService() (*service.ExportService, error) {
	if h.svc.Exports == nil {
		return nil, apperror.FeatureUnavailable("Data exports are not available yet.")
	}
	return h.svc.Exports, nil
}

// RequestExport queues a personal-data export (202).
func (h *Handler) RequestExport(c echo.Context) error {
	ex, err := h.exportService()
	if err != nil {
		return err
	}
	var req service.PasswordInput
	if err := bind(c, &req); err != nil {
		return err
	}
	v, err := ex.Request(c.Request().Context(), principal(c).UserID, req.Password)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusAccepted, v)
}

// Exports lists the caller's exports.
func (h *Handler) Exports(c echo.Context) error {
	ex, err := h.exportService()
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 10, 50)
	if err != nil {
		return err
	}
	page, err := ex.List(c.Request().Context(), principal(c).UserID, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	page.Items = orEmpty(page.Items)
	return ok(c, page)
}

// Export returns one export.
func (h *Handler) Export(c echo.Context) error {
	ex, err := h.exportService()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return apperror.NotFound("Export")
	}
	v, err := ex.Get(c.Request().Context(), principal(c).UserID, id)
	if err != nil {
		return err
	}
	return ok(c, v)
}

// ExportDownloadToken issues a single-use, short-lived download token.
func (h *Handler) ExportDownloadToken(c echo.Context) error {
	ex, err := h.exportService()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return apperror.NotFound("Export")
	}
	var req service.PasswordInput
	if err := bind(c, &req); err != nil {
		return err
	}
	v, err := ex.DownloadToken(c.Request().Context(), principal(c).UserID, id, req.Password)
	if err != nil {
		return err
	}
	return ok(c, v)
}

// DownloadExport streams the archive as an uncacheable attachment.
func (h *Handler) DownloadExport(c echo.Context) error {
	ex, err := h.exportService()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return apperror.NotFound("Export")
	}
	dl, err := ex.Open(c.Request().Context(), principal(c).UserID, id, c.QueryParam("token"))
	if err != nil {
		return err
	}
	defer dl.Body.Close()
	// Large archives may take longer than the API's usual deadlines.
	_ = http.NewResponseController(c.Response().Writer).SetWriteDeadline(time.Now().Add(30 * time.Minute))
	hdr := c.Response().Header()
	hdr.Set("Cache-Control", "no-store, private")
	hdr.Set("Pragma", "no-cache")
	hdr.Set("Content-Disposition", `attachment; filename="`+dl.Filename+`"`)
	hdr.Set("X-Content-Type-Options", "nosniff")
	if dl.Size > 0 {
		hdr.Set("Content-Length", strconv.FormatInt(dl.Size, 10))
	}
	c.Response().Header().Set(echo.HeaderContentType, "application/zip")
	c.Response().WriteHeader(http.StatusOK)
	_, err = io.Copy(c.Response(), dl.Body)
	return err
}

// Deactivate pauses the caller's account and signs this browser out.
func (h *Handler) Deactivate(c echo.Context) error {
	acc, err := h.accountService()
	if err != nil {
		return err
	}
	var req service.PasswordInput
	if err := bind(c, &req); err != nil {
		return err
	}
	if err := acc.Deactivate(c.Request().Context(), principal(c).UserID, req); err != nil {
		return err
	}
	h.clearSession(c)
	return noContent(c)
}

// Reactivate consumes the token offered at sign-in and signs the user in.
func (h *Handler) Reactivate(c echo.Context) error {
	acc, err := h.accountService()
	if err != nil {
		return err
	}
	token, err := h.tokenRequest(c)
	if err != nil {
		return err
	}
	res, err := acc.Reactivate(c.Request().Context(), token)
	if err != nil {
		return err
	}
	return h.session(c, res, http.StatusOK)
}

// CloseAccount schedules the caller's account for erasure (202) and signs
// this browser out.
func (h *Handler) CloseAccount(c echo.Context) error {
	acc, err := h.accountService()
	if err != nil {
		return err
	}
	var req service.CloseInput
	if err := bind(c, &req); err != nil {
		return err
	}
	v, err := acc.Close(c.Request().Context(), principal(c).UserID, req)
	if err != nil {
		return err
	}
	h.clearSession(c)
	return c.JSON(http.StatusAccepted, v)
}

// CancelClosure keeps an account from the e-mailed link.
func (h *Handler) CancelClosure(c echo.Context) error {
	acc, err := h.accountService()
	if err != nil {
		return err
	}
	token, err := h.tokenRequest(c)
	if err != nil {
		return err
	}
	if err := acc.CancelClosure(c.Request().Context(), token); err != nil {
		return err
	}
	return ok(c, map[string]string{"status": "active"})
}
