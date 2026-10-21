// Package handler is the transport (controller) layer: it decodes and
// validates HTTP input, calls services, and encodes responses. It contains
// no business rules.
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/api/middleware"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// Services groups the business services used by handlers.
type Services struct {
	Auth      *service.AuthService
	Users     *service.UserService
	Wallet    *service.WalletService
	Psychics  *service.PsychicService
	Credits   *service.CreditService
	Chat      *service.ChatService
	Gateway   *service.RealtimeGateway
	Requests  *service.CustomRequestService
	Admin     *service.AdminService
	Analytics *service.AnalyticsService
	Astrology *service.AstrologyService

	AdminAuth        *service.AdminAuthService
	Growth           *service.GrowthService
	Insights         *service.InsightsService
	Notifications    *service.NotificationService
	Reviews          *service.ReviewService
	Withdrawals      *service.WithdrawalService
	PsychicAnalytics *service.PsychicAnalyticsService
}

// Limits bounds request bodies per upload type.
type Limits struct {
	AvatarBytes       int64
	AdvisorImageBytes int64
	ChatImageBytes    int64
}

// CookieSettings control auth cookies.
type CookieSettings struct {
	Names      middleware.CookieNames
	Domain     string
	Secure     bool
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

// Health checks dependencies for readiness.
type Health func(r *http.Request) map[string]error

// Handler holds dependencies for every endpoint.
type Handler struct {
	svc             Services
	csrf            *security.CSRF
	cookies         CookieSettings
	realtime        *realtime.Server
	health          Health
	defaultCurrency string
	maxAvatarBytes  int64
	limits          Limits
	adminSessionTTL time.Duration
}

// New returns a handler set.
func New(svc Services, csrf *security.CSRF, cookies CookieSettings, rt *realtime.Server, health Health, currency string, limits Limits,
	adminSessionTTL time.Duration) *Handler {
	return &Handler{svc: svc, csrf: csrf, cookies: cookies, realtime: rt, health: health, defaultCurrency: currency,
		maxAvatarBytes: limits.AvatarBytes, limits: limits, adminSessionTTL: adminSessionTTL}
}

// bind strictly decodes a JSON body: correct content type, no unknown
// fields, a single JSON value, bounded size (BodyLimit middleware).
func bind(c echo.Context, v any) error {
	ct := c.Request().Header.Get("Content-Type")
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil || mt != "application/json" {
		return apperror.UnsupportedMedia("Send JSON with Content-Type: application/json.")
	}
	dec := json.NewDecoder(c.Request().Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return decodeError(err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperror.BadRequest("The request body must contain a single JSON object.")
	}
	return nil
}

func decodeError(err error) error {
	var syntax *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	var maxBytes *http.MaxBytesError
	switch {
	case errors.As(err, &maxBytes):
		return apperror.PayloadTooLarge("The request body is too large.")
	case errors.As(err, &syntax), errors.Is(err, io.ErrUnexpectedEOF):
		return apperror.BadRequest("The request body is not valid JSON.")
	case errors.As(err, &typeErr):
		field := typeErr.Field
		if field == "" {
			field = "body"
		}
		return apperror.Validation(map[string]string{field: "Has the wrong type."})
	case errors.Is(err, io.EOF):
		return apperror.BadRequest("The request body is empty.")
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		field := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
		return apperror.Validation(map[string]string{field: "Unknown field."})
	}
	return apperror.BadRequest("The request body could not be read.")
}

// principal returns the authenticated principal (routes guarantee it).
func principal(c echo.Context) reqctx.Principal {
	p, _ := reqctx.PrincipalFrom(c.Request().Context())
	return p
}

// uuidParam parses a UUID path parameter.
func uuidParam(c echo.Context, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		return uuid.Nil, apperror.NotFound("Resource")
	}
	return id, nil
}

// optionalUUID parses an optional UUID query parameter.
func optionalUUID(c echo.Context, name string) (*uuid.UUID, error) {
	raw := c.QueryParam(name)
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, apperror.Validation(map[string]string{name: "Invalid identifier."})
	}
	return &id, nil
}

// limitParam parses ?limit= within [1, max].
func limitParam(c echo.Context, def, max int) (int, error) {
	raw := c.QueryParam("limit")
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > max {
		return 0, apperror.Validation(map[string]string{"limit": fmt.Sprintf("Use a number between 1 and %d.", max)})
	}
	return n, nil
}

// optionalTime parses an RFC 3339 query parameter.
func optionalTime(c echo.Context, name string) (*time.Time, error) {
	raw := c.QueryParam(name)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return nil, apperror.Validation(map[string]string{name: "Use an RFC 3339 timestamp."})
	}
	return &t, nil
}

func (h *Handler) cookie(name, value, path string, maxAge time.Duration, httpOnly bool, sameSite http.SameSite) *http.Cookie {
	ck := &http.Cookie{Name: name, Value: value, Path: path, HttpOnly: httpOnly, Secure: h.cookies.Secure, SameSite: sameSite}
	if !strings.HasPrefix(name, "__Host-") {
		ck.Domain = h.cookies.Domain
	}
	if maxAge > 0 {
		ck.MaxAge = int(maxAge.Seconds())
		ck.Expires = time.Now().Add(maxAge)
	} else {
		ck.MaxAge = -1
		ck.Expires = time.Unix(0, 0)
	}
	return ck
}

// setSession writes the access and refresh cookies.
func (h *Handler) setSession(c echo.Context, res *service.AuthResult) {
	c.SetCookie(h.cookie(h.cookies.Names.Access, res.AccessToken, "/", time.Until(res.AccessExpires), true, http.SameSiteLaxMode))
	c.SetCookie(h.cookie(h.cookies.Names.Refresh, res.RefreshToken, refreshPath, time.Until(res.RefreshExpires), true, http.SameSiteStrictMode))
}

// clearSession expires the auth cookies.
func (h *Handler) clearSession(c echo.Context) {
	c.SetCookie(h.cookie(h.cookies.Names.Access, "", "/", 0, true, http.SameSiteLaxMode))
	c.SetCookie(h.cookie(h.cookies.Names.Refresh, "", refreshPath, 0, true, http.SameSiteStrictMode))
}

// refreshPath scopes the refresh cookie to the auth endpoints only.
const refreshPath = "/api/v1/auth"

// issueCSRF rotates the CSRF token (cookie + response value).
func (h *Handler) issueCSRF(c echo.Context) string {
	token := h.csrf.Issue()
	c.SetCookie(h.cookie(h.cookies.Names.CSRF, token, "/", h.cookies.RefreshTTL, false, http.SameSiteLaxMode))
	c.Response().Header().Set(middleware.CSRFHeader, token)
	return token
}

func refreshCookie(c echo.Context, name string) string {
	ck, err := c.Request().Cookie(name)
	if err != nil {
		return ""
	}
	return ck.Value
}

// ok writes a JSON 200.
func ok(c echo.Context, v any) error { return c.JSON(http.StatusOK, v) }

// created writes a JSON 201.
func created(c echo.Context, v any) error { return c.JSON(http.StatusCreated, v) }

// noContent writes 204.
func noContent(c echo.Context) error { return c.NoContent(http.StatusNoContent) }

// orEmpty makes nil slices encode as [] instead of null, so clients can
// always iterate list responses.
func orEmpty[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}
