package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// Sign in with Google (revisions 4.0, §23.1).

// oauthCookieName is the browser-binding cookie of a sign-in attempt:
// HttpOnly, SameSite=Lax, scoped to the auth endpoints, __Secure- prefixed
// when cookies are Secure.
func (h *Handler) oauthCookieName() string {
	if h.cookies.Secure {
		return "__Secure-sy_oa"
	}
	return "sy_oa"
}

// oauthCookieValue returns the sy_oa value the browser sent ("" when absent).
func (h *Handler) oauthCookieValue(c echo.Context) string {
	ck, err := c.Request().Cookie(h.oauthCookieName())
	if err != nil || len(ck.Value) > 256 {
		return ""
	}
	return ck.Value
}

func (h *Handler) setOAuthCookie(c echo.Context, value string, ttl time.Duration) {
	c.SetCookie(h.cookie(h.oauthCookieName(), value, refreshPath, ttl, true, http.SameSiteLaxMode))
}

// clearOAuthCookie expires sy_oa (the attempt was consumed).
func (h *Handler) clearOAuthCookie(c echo.Context) {
	c.SetCookie(h.cookie(h.oauthCookieName(), "", refreshPath, 0, true, http.SameSiteLaxMode))
}

func (h *Handler) google() (*service.GoogleAuth, error) {
	if h.svc.GoogleAuth == nil {
		return nil, apperror.GoogleUnavailable(nil)
	}
	return h.svc.GoogleAuth, nil
}

type googleStartRequest struct {
	Intent      string                    `json:"intent"`
	AcceptTerms bool                      `json:"accept_terms"`
	Next        string                    `json:"next"`
	Locale      string                    `json:"locale"`
	Attribution *service.AttributionInput `json:"attribution"`
}

// GoogleStart begins a Google sign-in attempt and returns Google's URL.
func (h *Handler) GoogleStart(c echo.Context) error {
	var req googleStartRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	g, err := h.google()
	if err != nil {
		return err
	}
	in := service.GoogleStartInput{Intent: req.Intent, AcceptTerms: req.AcceptTerms, Next: req.Next, Locale: req.Locale,
		Attribution: req.Attribution}
	if in.Locale == "" {
		in.Locale = locale(c)
	}
	if p := principal(c); p.UserID != uuid.Nil {
		in.Principal = &p
	}
	res, err := g.Start(c.Request().Context(), in)
	if err != nil {
		return err
	}
	h.setOAuthCookie(c, res.Cookie, res.TTL)
	return ok(c, map[string]string{"authorization_url": res.AuthorizationURL})
}

// GoogleCallback receives the browser coming back from Google. It always
// answers a 303 to the web app (never JSON, never an error page): the
// attempt is protected by state + the sy_oa cookie, not by a CSRF header.
func (h *Handler) GoogleCallback(c echo.Context) error {
	ctx := c.Request().Context()
	g, err := h.google()
	if err != nil {
		return err
	}
	out := g.Callback(ctx, service.GoogleCallbackInput{Code: c.QueryParam("code"), State: c.QueryParam("state"),
		Error: c.QueryParam("error"), Cookie: h.oauthCookieValue(c), Locale: locale(c)})
	if out.Auth != nil {
		h.setSession(c, out.Auth)
		h.rotateCSRF(c)
	}
	if out.Created {
		h.afterGoogleSignup(ctx, out.UserID, out.Locale, out.Attribution)
	}
	if !out.KeepCookie {
		h.clearOAuthCookie(c)
	}
	return h.googleRedirect(c, g, out)
}

// googleRedirect answers the callback: 303, never cached, no referrer.
func (h *Handler) googleRedirect(c echo.Context, g *service.GoogleAuth, out *service.GoogleCallbackOutcome) error {
	hd := c.Response().Header()
	hd.Set("Cache-Control", "no-store")
	hd.Set("Referrer-Policy", "no-referrer")
	return c.Redirect(http.StatusSeeOther, g.RedirectURL(out))
}

// GoogleCallbackGuard turns a failure outside the service (rate limit,
// limiter or timeout) into the same 303 the callback always answers.
func (h *Handler) GoogleCallbackGuard(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := next(c)
		if err == nil || c.Response().Committed {
			return err
		}
		logger.FromContext(c.Request().Context()).Warn("google callback failed before the sign-in logic", zap.Error(err))
		g, gerr := h.google()
		if gerr != nil || g == nil {
			return err
		}
		return h.googleRedirect(c, g, &service.GoogleCallbackOutcome{Result: service.GoogleResultUnavailable, Locale: locale(c)})
	}
}

// afterGoogleSignup runs the steps that follow every new account, exactly
// like POST /auth/register: first-touch attribution and the calculator link.
func (h *Handler) afterGoogleSignup(ctx context.Context, userID uuid.UUID, loc string, attribution *service.AttributionInput) {
	ctx = reqctx.WithPrincipal(ctx, reqctx.Principal{UserID: userID, Role: domain.RoleClient})
	if h.svc.Growth != nil {
		h.svc.Growth.Attribute(ctx, userID, attribution)
	}
	if h.svc.Calculators != nil {
		h.svc.Calculators.LinkSignup(ctx, userID, loc, attribution)
	}
}

// GooglePending tells the web app what the sy_oa cookie is waiting for.
func (h *Handler) GooglePending(c echo.Context) error {
	g, err := h.google()
	if err != nil {
		return err
	}
	v, err := g.Pending(c.Request().Context(), h.oauthCookieValue(c))
	if err != nil {
		return err
	}
	return ok(c, v)
}

type googleCompleteRequest struct {
	AcceptTerms bool                      `json:"accept_terms"`
	Locale      string                    `json:"locale"`
	Attribution *service.AttributionInput `json:"attribution"`
}

// GoogleComplete creates the account of a pending Google sign-up.
func (h *Handler) GoogleComplete(c echo.Context) error {
	var req googleCompleteRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	g, err := h.google()
	if err != nil {
		return err
	}
	res, err := g.Complete(c.Request().Context(), service.GoogleCompleteInput{AcceptTerms: req.AcceptTerms, Locale: req.Locale,
		Attribution: req.Attribution, Cookie: h.oauthCookieValue(c)})
	if err != nil {
		return err
	}
	h.clearOAuthCookie(c)
	if res.Created {
		h.afterGoogleSignup(c.Request().Context(), res.UserID, res.Locale, res.Attribution)
	}
	return h.session(c, res.Auth, http.StatusCreated)
}

// SignInMethods lists how the caller can sign in.
func (h *Handler) SignInMethods(c echo.Context) error {
	g, err := h.google()
	if err != nil {
		return err
	}
	v, err := g.SignInMethods(c.Request().Context(), principal(c).UserID)
	if err != nil {
		return err
	}
	return ok(c, v)
}

type disconnectGoogleRequest struct {
	Password string `json:"password"`
}

// DisconnectGoogle removes the caller's Google sign-in (password required).
func (h *Handler) DisconnectGoogle(c echo.Context) error {
	var req disconnectGoogleRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	g, err := h.google()
	if err != nil {
		return err
	}
	if err := g.Disconnect(c.Request().Context(), principal(c).UserID, req.Password); err != nil {
		return err
	}
	return noContent(c)
}
