package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// Advisor settings (M4): contact details and profile text, the photo
// submitted for review and the advisor's own custom offering. Every route
// acts on the signed-in advisor only, so there is no ID to probe.

// PsychicSettings returns the advisor's settings document.
func (h *Handler) PsychicSettings(c echo.Context) error {
	out, err := h.svc.Advisor.Settings(c.Request().Context(), principal(c).UserID)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// UpdatePsychicSettings saves a partial settings update.
func (h *Handler) UpdatePsychicSettings(c echo.Context) error {
	var req service.AdvisorSettingsPatch
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Advisor.UpdateSettings(c.Request().Context(), principal(c).UserID, req)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// PsychicPhoto returns the live photo and the latest submission.
func (h *Handler) PsychicPhoto(c echo.Context) error {
	out, err := h.svc.Advisor.Photo(c.Request().Context(), principal(c).UserID)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// SubmitPsychicPhoto uploads a photo for review (multipart field "photo").
func (h *Handler) SubmitPsychicPhoto(c echo.Context) error {
	data, _, err := readUpload(c, "photo", h.limits.AdvisorImageBytes)
	if err != nil {
		return err
	}
	out, err := h.svc.Advisor.SubmitPhoto(c.Request().Context(), principal(c).UserID, data)
	if err != nil {
		return err
	}
	return created(c, out)
}

// WithdrawPsychicPhoto removes the pending submission.
func (h *Handler) WithdrawPsychicPhoto(c echo.Context) error {
	if err := h.svc.Advisor.WithdrawPhoto(c.Request().Context(), principal(c).UserID); err != nil {
		return err
	}
	return noContent(c)
}

// PsychicPhotoImage streams the advisor's own pending or rejected photo.
func (h *Handler) PsychicPhotoImage(c echo.Context) error {
	body, ct, err := h.svc.Advisor.PhotoImage(c.Request().Context(), principal(c).UserID)
	if err != nil {
		return err
	}
	return privateImage(c, ct, body)
}

// PsychicCustomService returns the advisor's custom offering.
func (h *Handler) PsychicCustomService(c echo.Context) error {
	out, err := h.svc.Advisor.CustomService(c.Request().Context(), principal(c).UserID)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// PutPsychicCustomService creates (201) or replaces (200) the offering.
func (h *Handler) PutPsychicCustomService(c echo.Context) error {
	var req service.CustomServiceInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, isNew, err := h.svc.Advisor.PutCustomService(c.Request().Context(), principal(c).UserID, req)
	if err != nil {
		return err
	}
	if isNew {
		return created(c, out)
	}
	return ok(c, out)
}

// DeletePsychicCustomService removes the offering.
func (h *Handler) DeletePsychicCustomService(c echo.Context) error {
	if err := h.svc.Advisor.DeleteCustomService(c.Request().Context(), principal(c).UserID); err != nil {
		return err
	}
	return noContent(c)
}

// --- review console (step-up, audited) ---

// AdminPhotoSubmissions pages submissions by status.
func (h *Handler) AdminPhotoSubmissions(c echo.Context) error {
	limit, err := limitParam(c, 20, 100)
	if err != nil {
		return err
	}
	out, err := h.svc.Advisor.AdminPhotoSubmissions(c.Request().Context(), c.QueryParam("status"), c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminPhotoSubmissionImage streams a submission's private image.
func (h *Handler) AdminPhotoSubmissionImage(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	body, ct, err := h.svc.Advisor.AdminPhotoImage(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return privateImage(c, ct, body)
}

// AdminApprovePhoto publishes a pending photo.
func (h *Handler) AdminApprovePhoto(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Advisor.ApprovePhoto(c.Request().Context(), principal(c).UserID, id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminRejectPhoto declines a pending photo with a reason.
func (h *Handler) AdminRejectPhoto(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Advisor.RejectPhoto(c.Request().Context(), principal(c).UserID, id, req.Reason)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// privateImage writes image bytes that must never be stored by a browser or
// shared cache (unreviewed advisor photos).
func privateImage(c echo.Context, contentType string, body []byte) error {
	hd := c.Response().Header()
	hd.Set("Cache-Control", "no-store, private")
	hd.Set("Pragma", "no-cache")
	hd.Set("Content-Disposition", "inline")
	hd.Set("Cross-Origin-Resource-Policy", "same-site")
	hd.Set("Vary", "Cookie")
	return c.Blob(http.StatusOK, contentType, body)
}
