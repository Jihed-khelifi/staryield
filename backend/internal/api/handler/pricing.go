package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 5.0: pricing catalog (§24.1).

// Pricing serves the public pricing summary: public and cacheable, with a
// weak ETag (304 on If-None-Match). The locale (query, X-Locale or
// Accept-Language) is part of the cache key through Vary.
func (h *Handler) Pricing(c echo.Context) error {
	loc := c.QueryParam("locale")
	if loc == "" {
		loc = locale(c)
	}
	res, err := h.svc.Pricing.Summary(c.Request().Context(), loc)
	if err != nil {
		return err
	}
	hd := c.Response().Header()
	hd.Set("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	hd.Add("Vary", "Accept-Language, X-Locale")
	hd.Set("ETag", res.ETag)
	if service.ETagMatches(c.Request().Header.Get("If-None-Match"), res.ETag) {
		return c.NoContent(http.StatusNotModified)
	}
	return c.Blob(http.StatusOK, echo.MIMEApplicationJSONCharsetUTF8, res.Body)
}

// CreditCatalog is the signed-in client's top-up catalog.
func (h *Handler) CreditCatalog(c echo.Context) error {
	out, err := h.svc.Pricing.Catalog(c.Request().Context(), principal(c).UserID, locale(c))
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "private, no-store")
	return ok(c, out)
}

// AdminReorderPacks sets the display order of the packs.
func (h *Handler) AdminReorderPacks(c echo.Context) error {
	var in struct {
		IDs []string `json:"ids"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	if in.IDs == nil {
		return apperror.Validation(map[string]string{"ids": "This field is required."})
	}
	ids := make([]uuid.UUID, 0, len(in.IDs))
	for _, raw := range in.IDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return apperror.Validation(map[string]string{"ids": "Send every pack exactly once."})
		}
		ids = append(ids, id)
	}
	items, err := h.svc.Pricing.ReorderPacks(c.Request().Context(), ids)
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"items": items})
}

// AdminPricingSettings returns the pricing settings.
func (h *Handler) AdminPricingSettings(c echo.Context) error {
	out, err := h.svc.Pricing.Settings(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminUpdatePricingSettings replaces the pricing settings.
func (h *Handler) AdminUpdatePricingSettings(c echo.Context) error {
	var in service.PricingSettingsInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Pricing.UpdateSettings(c.Request().Context(), in)
	if err != nil {
		return err
	}
	return ok(c, out)
}
