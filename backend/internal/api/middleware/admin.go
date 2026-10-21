package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// AdminOrigin only lets the admin console call admin routes: every request
// (including GETs) must carry an Origin from the allow-list. Browsers always
// send Origin on cross-origin fetches, so pages on any other site, the main
// web app included, cannot reach the admin API with an admin's cookies.
func AdminOrigin(allowed map[string]bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Method == http.MethodOptions {
				return next(c)
			}
			origin := strings.ToLower(c.Request().Header.Get("Origin"))
			if origin == "" || !allowed[origin] {
				logger.FromContext(c.Request().Context()).Warn("admin: origin rejected", zap.String("origin", origin))
				return apperror.New(apperror.CodeCSRF, http.StatusForbidden, "Request origin is not allowed.")
			}
			return next(c)
		}
	}
}

// IPAllowlist restricts a route group to client IPs inside nets (no-op when
// nets is empty). The client IP is resolved by the router's trusted-proxy
// aware extractor.
func IPAllowlist(nets []*net.IPNet) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if len(nets) == 0 {
				return next(c)
			}
			ip := net.ParseIP(c.RealIP())
			for _, n := range nets {
				if ip != nil && n.Contains(ip) {
					return next(c)
				}
			}
			logger.FromContext(c.Request().Context()).Warn("admin: ip not allowed", zap.String("ip", AnonymizeIP(c.RealIP())))
			return apperror.Forbidden("")
		}
	}
}

// AdminAuthenticator resolves an admin session cookie into a request
// context carrying the admin principal.
type AdminAuthenticator interface {
	AuthenticateAdmin(ctx context.Context, token string) (context.Context, error)
}

// AdminSession requires a valid admin console session cookie.
func AdminSession(auth AdminAuthenticator, cookieName string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			r := c.Request()
			token, ok := cookie(r, cookieName)
			if !ok {
				return apperror.Unauthenticated("")
			}
			ctx, err := auth.AuthenticateAdmin(r.Context(), token)
			if err != nil {
				return err
			}
			if p, ok := reqctx.PrincipalFrom(ctx); ok {
				ctx = logger.With(ctx, zap.String("user_id", p.UserID.String()), zap.String("role", string(p.Role)))
			}
			c.SetRequest(r.WithContext(ctx))
			return next(c)
		}
	}
}

// StepUp requires a recent second-factor verification for sensitive routes.
func StepUp(check func(ctx context.Context) error) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if err := check(c.Request().Context()); err != nil {
				return err
			}
			return next(c)
		}
	}
}
