package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// optionalInt parses an optional integer query parameter.
func optionalInt(c echo.Context, name, message string) (int, error) {
	raw := strings.TrimSpace(c.QueryParam(name))
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, apperror.Validation(map[string]string{name: message})
	}
	return n, nil
}

// browsePsychics serves GET /psychics: a numbered directory page
// (?page&page_size&status&q&skill) from the in-memory catalogue.
func (h *Handler) browsePsychics(c echo.Context) error {
	page, err := optionalInt(c, "page", "Use a page number of 1 or more.")
	if err != nil {
		return err
	}
	size, err := optionalInt(c, "page_size", "Use a page size between 1 and 48.")
	if err != nil {
		return err
	}
	out, err := h.svc.Catalog.Browse(c.Request().Context(), service.BrowseQuery{Page: page, PageSize: size,
		Status: c.QueryParam("status"), Search: c.QueryParam("q"), Skill: c.QueryParam("skill")})
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=10")
	return ok(c, out)
}

// FeaturedPsychics serves GET /psychics/featured?limit=3 for the home page.
// It is public and cacheable; the ETag covers the cards (not the build time)
// so unchanged selections revalidate with 304.
func (h *Handler) FeaturedPsychics(c echo.Context) error {
	limit, err := optionalInt(c, "limit", "Use a number between 1 and 6.")
	if err != nil {
		return err
	}
	out, err := h.svc.Catalog.Featured(c.Request().Context(), limit)
	if err != nil {
		return err
	}
	items, err := json.Marshal(out.Items)
	if err != nil {
		return apperror.Internal(err)
	}
	sum := sha256.Sum256(items)
	etag := `W/"` + hex.EncodeToString(sum[:12]) + `"`
	hd := c.Response().Header()
	hd.Set("Cache-Control", "public, max-age=10, stale-while-revalidate=30")
	hd.Set("ETag", etag)
	if match := c.Request().Header.Get("If-None-Match"); match != "" && strings.Contains(match, etag) {
		return c.NoContent(http.StatusNotModified)
	}
	body, err := json.Marshal(out)
	if err != nil {
		return apperror.Internal(err)
	}
	return c.JSONBlob(http.StatusOK, body)
}
