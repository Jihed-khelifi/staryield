package handler

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// Shared-cache lifetimes (seconds) of the public astrology responses. Birth
// data never reaches these: the calculators are POSTs answered no-store.
const (
	placesMaxAge    = 86400 // the gazetteer only changes with a release
	horoscopeMaxAge = 900
	aztroMaxAge     = 1800
	eventsMaxAge    = 900
)

// localeVary lists the request headers the reading's language comes from.
var localeVary = []string{"X-Locale", "Accept-Language"}

// publicJSON writes v as a publicly cacheable JSON response with a strong
// ETag, and answers 304 when the client already holds this version.
func publicJSON(c echo.Context, maxAge int, v any, vary ...string) error {
	body, err := json.Marshal(v)
	if err != nil {
		return apperror.Internal(err)
	}
	sum := sha256.Sum256(body)
	etag := `"` + base64.RawURLEncoding.EncodeToString(sum[:18]) + `"`
	h := c.Response().Header()
	h.Set("Cache-Control", "public, max-age="+strconv.Itoa(maxAge))
	h.Del("Pragma")
	h.Set("ETag", etag)
	for _, name := range vary {
		h.Add("Vary", name)
	}
	if etagMatches(c.Request().Header.Get("If-None-Match"), etag) {
		return c.NoContent(http.StatusNotModified)
	}
	return c.JSONBlob(http.StatusOK, body)
}

// etagMatches applies If-None-Match's weak comparison (RFC 9110 §13.1.2).
func etagMatches(header, etag string) bool {
	if header == "" {
		return false
	}
	for _, tag := range strings.Split(header, ",") {
		tag = strings.TrimSpace(tag)
		if tag == "*" || strings.TrimPrefix(tag, "W/") == etag {
			return true
		}
	}
	return false
}

// Places searches the birthplace gazetteer (?q=, ?limit=, ?cursor=).
func (h *Handler) Places(c echo.Context) error {
	limit, err := limitParam(c, 8, 20)
	if err != nil {
		return err
	}
	out, err := h.svc.Astrology.Places(c.QueryParam("q"), limit, c.QueryParam("cursor"))
	if err != nil {
		return err
	}
	return publicJSON(c, placesMaxAge, out)
}

// Place returns one gazetteer entry by ID ("gn:2988507", or a legacy slug).
func (h *Handler) Place(c echo.Context) error {
	id, err := url.PathUnescape(c.Param("id"))
	if err != nil || len(id) > 64 {
		return apperror.NotFound("Place")
	}
	out, err := h.svc.Astrology.Place(id)
	if err != nil {
		return err
	}
	return publicJSON(c, placesMaxAge, out)
}

// Natal computes the anonymous natal chart with its Twelve Houses spread
// and numerology (FR-01). Stateless: the body is never stored or logged.
func (h *Handler) Natal(c echo.Context) error {
	var req service.BirthInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Astrology.Natal(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// chartRequest is the original calculator body (kept for existing clients).
type chartRequest struct {
	BirthDate   string `json:"birth_date"`
	BirthTime   string `json:"birth_time"`
	PlaceID     string `json:"place_id"`
	HouseSystem string `json:"house_system"`
}

// PublicChart computes a natal chart for the anonymous calculator (FR-01).
func (h *Handler) PublicChart(c echo.Context) error {
	var req chartRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Astrology.Chart(c.Request().Context(), service.BirthInput{
		BirthDate: req.BirthDate, BirthTime: req.BirthTime, PlaceID: req.PlaceID, HouseSystem: req.HouseSystem,
	})
	if err != nil {
		return err
	}
	return ok(c, out)
}

// Numerology computes the life path and personal cycles.
func (h *Handler) Numerology(c echo.Context) error {
	var req service.NumerologyInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Astrology.Numerology(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// sunSignRequest asks for the sun sign of a birth.
type sunSignRequest struct {
	BirthDate string `json:"birth_date"`
	BirthTime string `json:"birth_time"`
	PlaceID   string `json:"place_id"`
}

// SunSign answers the sun sign of a birth, flagging cusp births whose sign
// depends on an unknown time or place.
func (h *Handler) SunSign(c echo.Context) error {
	var req sunSignRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Astrology.SunSign(c.Request().Context(), service.BirthInput{
		BirthDate: req.BirthDate, BirthTime: req.BirthTime, PlaceID: req.PlaceID,
	})
	if err != nil {
		return err
	}
	return ok(c, out)
}

// horoscopeQuery reads ?day=, ?date= and ?tz= with the request's locale.
func horoscopeQuery(c echo.Context, sign string) service.HoroscopeQuery {
	return service.HoroscopeQuery{
		Sign: sign, Day: c.QueryParam("day"), Date: c.QueryParam("date"), TZ: c.QueryParam("tz"),
		Locale: service.LocaleFrom(c.Request().Context()),
	}
}

// SignHoroscope returns a sign's daily horoscope (public).
func (h *Handler) SignHoroscope(c echo.Context) error {
	out, err := h.svc.Astrology.HoroscopeFor(c.Request().Context(), horoscopeQuery(c, c.Param("sign")))
	if err != nil {
		return err
	}
	return publicJSON(c, horoscopeMaxAge, out, localeVary...)
}

// Aztro serves a reading in the aztro API's shape (GET, or POST for parity):
// ?sign=, ?day=yesterday|today|tomorrow, ?tz= and ?lang=en|fr|es (the
// request's locale when absent). Parameters may also come form-encoded.
func (h *Handler) Aztro(c echo.Context) error {
	q := horoscopeQuery(c, c.FormValue("sign"))
	q.Day, q.Date, q.TZ = c.FormValue("day"), "", c.FormValue("tz")
	if lang := c.FormValue("lang"); lang != "" {
		q.Locale = lang
	}
	out, err := h.svc.Astrology.Aztro(c.Request().Context(), q)
	if err != nil {
		return err
	}
	if c.Request().Method != http.MethodGet {
		return ok(c, out) // responses to POST are not reusable by caches
	}
	return publicJSON(c, aztroMaxAge, out, localeVary...)
}

// CosmicEvents lists upcoming lunar phases and retrogrades (?count=1–12).
func (h *Handler) CosmicEvents(c echo.Context) error {
	count := 4
	if raw := c.QueryParam("count"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 12 {
			return apperror.Validation(map[string]string{"count": "Use a number between 1 and 12."})
		}
		count = n
	}
	return publicJSON(c, eventsMaxAge, h.svc.Astrology.Events(count))
}

// MyChart returns the client's birth chart (FR-03) (?house_system=).
func (h *Handler) MyChart(c echo.Context) error {
	out, err := h.svc.Astrology.MyChart(c.Request().Context(), principal(c).UserID, c.QueryParam("house_system"))
	if err != nil {
		return err
	}
	return ok(c, out)
}

// MyHoroscope returns the client's daily horoscope (FR-04) (?day=, ?date=,
// ?tz=).
func (h *Handler) MyHoroscope(c echo.Context) error {
	out, err := h.svc.Astrology.MyHoroscope(c.Request().Context(), principal(c).UserID, horoscopeQuery(c, ""))
	if err != nil {
		return err
	}
	return ok(c, out)
}

// MyNumerology returns the client's numerology numbers (?tz=).
func (h *Handler) MyNumerology(c echo.Context) error {
	out, err := h.svc.Astrology.MyNumerology(c.Request().Context(), principal(c).UserID, c.QueryParam("tz"))
	if err != nil {
		return err
	}
	return ok(c, out)
}
