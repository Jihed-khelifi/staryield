package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 3.0: site settings (§15).

// SiteConfig serves the public site configuration (promo banners, sign-up
// offer, free mode). It is public and cacheable: a weak ETag lets clients
// and CDNs revalidate with If-None-Match (304), and the locale (query,
// X-Locale or Accept-Language) is part of the cache key through Vary.
func (h *Handler) SiteConfig(c echo.Context) error {
	loc := c.QueryParam("locale")
	if loc == "" {
		loc = locale(c)
	}
	res, err := h.svc.Site.Config(c.Request().Context(), loc)
	if err != nil {
		return err
	}
	hd := c.Response().Header()
	hd.Set("Cache-Control", "public, max-age=30, stale-while-revalidate=60")
	hd.Add("Vary", "Accept-Language, X-Locale")
	hd.Set("ETag", res.ETag)
	if service.ETagMatches(c.Request().Header.Get("If-None-Match"), res.ETag) {
		return c.NoContent(http.StatusNotModified)
	}
	return c.Blob(http.StatusOK, echo.MIMEApplicationJSONCharsetUTF8, res.Body)
}

// AdminSiteBanners lists every banner with the promo bar settings.
func (h *Handler) AdminSiteBanners(c echo.Context) error {
	out, err := h.svc.Site.Banners(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminCreateSiteBanner adds a banner.
func (h *Handler) AdminCreateSiteBanner(c echo.Context) error {
	var in service.BannerInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Site.CreateBanner(c.Request().Context(), in)
	if err != nil {
		return err
	}
	return created(c, out)
}

// AdminUpdateSiteBanner changes a banner.
func (h *Handler) AdminUpdateSiteBanner(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var in service.BannerPatch
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Site.UpdateBanner(c.Request().Context(), id, in)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminDeleteSiteBanner removes a banner.
func (h *Handler) AdminDeleteSiteBanner(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	if err := h.svc.Site.DeleteBanner(c.Request().Context(), id); err != nil {
		return err
	}
	return noContent(c)
}

// AdminReorderSiteBanners sets the display order.
func (h *Handler) AdminReorderSiteBanners(c echo.Context) error {
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
			return apperror.Validation(map[string]string{"ids": "Send every banner exactly once."})
		}
		ids = append(ids, id)
	}
	items, err := h.svc.Site.ReorderBanners(c.Request().Context(), ids)
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"items": items})
}

// AdminUpdateBannerSettings changes the promo bar switch and rotation.
func (h *Handler) AdminUpdateBannerSettings(c echo.Context) error {
	var in service.BannerSettingsPatch
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Site.UpdateBannerSettings(c.Request().Context(), in)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminSignupOffer returns the sign-up offer.
func (h *Handler) AdminSignupOffer(c echo.Context) error {
	out, err := h.svc.Site.SignupOffer(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminUpdateSignupOffer replaces the sign-up offer.
func (h *Handler) AdminUpdateSignupOffer(c echo.Context) error {
	var in service.SignupOfferInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Site.UpdateSignupOffer(c.Request().Context(), in)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminOffSessionMessages returns the messages-outside-a-live-chat setting
// with its 24 h oversight totals (revisions 3.3).
func (h *Handler) AdminOffSessionMessages(c echo.Context) error {
	out, err := h.svc.Site.OffSessionMessages(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminUpdateOffSessionMessages replaces the setting.
func (h *Handler) AdminUpdateOffSessionMessages(c echo.Context) error {
	var in service.OffSessionInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Site.UpdateOffSessionMessages(c.Request().Context(), in)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminFreeMode returns the free-mode state and its statistics.
func (h *Handler) AdminFreeMode(c echo.Context) error {
	out, err := h.svc.Site.FreeMode(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminUpdateFreeMode switches free mode on or off.
func (h *Handler) AdminUpdateFreeMode(c echo.Context) error {
	var in service.FreeModeInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Site.UpdateFreeMode(c.Request().Context(), in)
	if err != nil {
		return err
	}
	return ok(c, out)
}
