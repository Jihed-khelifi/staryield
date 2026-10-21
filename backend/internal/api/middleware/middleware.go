// Package middleware holds the Echo middleware chain. Each middleware is a
// small, explicit decorator; ordering is assembled in api.NewRouter.
package middleware

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/ratelimit"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// HeaderRequestID carries the correlation ID in both directions.
const HeaderRequestID = "X-Request-ID"

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9_\-.:]{8,64}$`)

// RequestID accepts a well-formed inbound X-Request-ID (so the frontend can
// correlate its logs with ours) or generates one.
func RequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			id := c.Request().Header.Get(HeaderRequestID)
			if !validRequestID.MatchString(id) {
				id = uuid.NewString()
			}
			c.Request().Header.Set(HeaderRequestID, id)
			c.Response().Header().Set(HeaderRequestID, id)
			return next(c)
		}
	}
}

// AnonymizeIP truncates an address (IPv4 /24, IPv6 /48) for logs.
func AnonymizeIP(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ""
	}
	if v4 := parsed.To4(); v4 != nil {
		return net.IPv4(v4[0], v4[1], v4[2], 0).String()
	}
	mask := net.CIDRMask(48, 128)
	return parsed.Mask(mask).String()
}

// RequestContext stores request metadata and a request-scoped logger in the
// request context, so services and repositories log with correlation fields.
func RequestContext(base *zap.Logger, ipSecret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			r := c.Request()
			ip := c.RealIP()
			meta := reqctx.Meta{
				RequestID: r.Header.Get(HeaderRequestID),
				IP:        ip,
				UserAgent: truncate(r.UserAgent(), 256),
				Locale:    localeFrom(r),
				Path:      c.Path(),
				Referrer:  truncate(r.Referer(), 512),
			}
			log := base.With(
				zap.String("request_id", meta.RequestID),
				zap.String("method", r.Method),
				zap.String("route", c.Path()),
				zap.String("ip", AnonymizeIP(ip)),
				zap.String("ip_hash", security.Keyed(ipSecret, ip, 16)),
			)
			ctx := reqctx.WithMeta(r.Context(), meta)
			ctx = logger.WithContext(ctx, log)
			c.SetRequest(r.WithContext(ctx))
			return next(c)
		}
	}
}

func localeFrom(r *http.Request) string {
	if l := r.Header.Get("X-Locale"); l != "" {
		return truncate(l, 8)
	}
	al := r.Header.Get("Accept-Language")
	if al == "" {
		return ""
	}
	tag, _, _ := strings.Cut(al, ",")
	tag, _, _ = strings.Cut(tag, ";")
	tag, _, _ = strings.Cut(strings.TrimSpace(tag), "-")
	return truncate(strings.ToLower(tag), 8)
}

// AccessLog writes one structured line per request, after the error handler
// has produced the final status.
func AccessLog(skip func(echo.Context) bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			if err != nil {
				c.Error(err)
			}
			if skip != nil && skip(c) {
				return nil
			}
			status := c.Response().Status
			log := logger.FromContext(c.Request().Context())
			fields := []zap.Field{
				zap.Int("status", status),
				zap.Duration("latency", time.Since(start)),
				zap.Int64("bytes_out", c.Response().Size),
				zap.String("user_agent", truncate(c.Request().UserAgent(), 128)),
			}
			if p, ok := reqctx.PrincipalFrom(c.Request().Context()); ok {
				fields = append(fields, zap.String("user_id", p.UserID.String()), zap.String("role", string(p.Role)))
			}
			level := zapcore.InfoLevel
			switch {
			case status >= 500:
				level = zapcore.ErrorLevel
			case status == http.StatusTooManyRequests || status == http.StatusForbidden:
				level = zapcore.WarnLevel
			case c.Path() == "/healthz" || c.Path() == "/readyz":
				level = zapcore.DebugLevel
			}
			if ce := log.Check(level, "http request"); ce != nil {
				ce.Write(fields...)
			}
			return nil
		}
	}
}

// Recover converts panics into 500 responses and logs the stack.
func Recover() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					if r == http.ErrAbortHandler {
						panic(r)
					}
					logger.FromContext(c.Request().Context()).Error("panic recovered",
						zap.Any("panic", r), zap.ByteString("stack", debug.Stack()))
					err = apperror.Internal(fmt.Errorf("panic: %v", r))
				}
			}()
			return next(c)
		}
	}
}

// SecurityHeaders hardens every API response.
func SecurityHeaders(hsts bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			h := c.Response().Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("Cross-Origin-Resource-Policy", "same-site")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
			h.Set("X-Permitted-Cross-Domain-Policies", "none")
			if hsts {
				h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
			}
			if strings.HasPrefix(c.Request().URL.Path, "/api/") && h.Get("Cache-Control") == "" {
				h.Set("Cache-Control", "no-store")
				h.Set("Pragma", "no-cache")
			}
			return next(c)
		}
	}
}

// BodyLimit caps request bodies with http.MaxBytesReader.
func BodyLimit(limit int64, skip func(echo.Context) bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if skip != nil && skip(c) {
				return next(c)
			}
			r := c.Request()
			if r.ContentLength > limit {
				return apperror.PayloadTooLarge("The request body is too large.")
			}
			if r.Body != nil {
				r.Body = http.MaxBytesReader(c.Response(), r.Body, limit)
			}
			return next(c)
		}
	}
}

// Timeout bounds request handling with a context deadline that cascades to
// every database and Redis call made on behalf of the request.
func Timeout(d time.Duration, skip func(echo.Context) bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if d <= 0 || (skip != nil && skip(c)) {
				return next(c)
			}
			ctx, cancel := context.WithTimeout(c.Request().Context(), d)
			defer cancel()
			c.SetRequest(c.Request().WithContext(ctx))
			err := next(c)
			if err == nil && errors.Is(ctx.Err(), context.DeadlineExceeded) && !c.Response().Committed {
				return apperror.Timeout(ctx.Err())
			}
			return err
		}
	}
}

// KeyFunc derives a rate-limit key from the request.
type KeyFunc func(c echo.Context) string

// ByIP keys by client IP.
func ByIP(c echo.Context) string { return "ip:" + c.RealIP() }

// ByUserOrIP keys by authenticated user, falling back to IP.
func ByUserOrIP(c echo.Context) string {
	if p, ok := reqctx.PrincipalFrom(c.Request().Context()); ok {
		return "u:" + p.UserID.String()
	}
	return ByIP(c)
}

// RateLimit applies a token bucket. When the limiter backend fails the
// request is allowed if failOpen, else rejected (used for auth endpoints).
func RateLimit(l ratelimit.Limiter, rule ratelimit.Rule, key KeyFunc, failOpen bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if l == nil {
				return next(c)
			}
			res, err := l.Allow(c.Request().Context(), key(c), rule)
			if err != nil {
				logger.FromContext(c.Request().Context()).Error("rate limiter unavailable", zap.Error(err), zap.String("rule", rule.Name))
				if failOpen {
					return next(c)
				}
				return apperror.Unavailable(err)
			}
			c.Response().Header().Set("RateLimit-Policy", fmt.Sprintf("%d;w=%d", rule.Rate, int(rule.Period.Seconds())))
			c.Response().Header().Set("RateLimit-Remaining", strconv.Itoa(res.Remaining))
			if !res.Allowed {
				logger.FromContext(c.Request().Context()).Warn("rate limited", zap.String("rule", rule.Name))
				return apperror.RateLimited(res.RetryAfter)
			}
			return next(c)
		}
	}
}

// Cookie names, resolved from configuration (secure prefixes in production).
type CookieNames struct {
	Access  string
	Refresh string
	CSRF    string
	// Admin is the admin console session cookie.
	Admin string
}

// CSRFHeader is the double-submit header.
const CSRFHeader = "X-CSRF-Token"

// CSRF enforces signed double-submit tokens and an Origin allow-list on
// state-changing requests that carry session cookies.
func CSRF(csrf *security.CSRF, names CookieNames, allowed map[string]bool, skip func(echo.Context) bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			r := c.Request()
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				return next(c)
			}
			if skip != nil && skip(c) {
				return next(c)
			}
			if origin := r.Header.Get("Origin"); origin != "" && !allowed[strings.ToLower(origin)] {
				logger.FromContext(r.Context()).Warn("csrf: origin rejected", zap.String("origin", origin))
				return apperror.New(apperror.CodeCSRF, http.StatusForbidden, "Request origin is not allowed.")
			}
			_, hasAccess := cookie(r, names.Access)
			_, hasRefresh := cookie(r, names.Refresh)
			_, hasAdmin := cookie(r, names.Admin)
			if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") && !hasAccess && !hasRefresh && !hasAdmin {
				return next(c) // token clients are not exposed to CSRF
			}
			cv, _ := cookie(r, names.CSRF)
			if !csrf.Match(r.Header.Get(CSRFHeader), cv) {
				logger.FromContext(r.Context()).Info("csrf: token mismatch")
				return apperror.New(apperror.CodeCSRF, http.StatusForbidden, "Your session security token is missing or expired. Please retry.")
			}
			return next(c)
		}
	}
}

func cookie(r *http.Request, name string) (string, bool) {
	c, err := r.Cookie(name)
	if err != nil || c.Value == "" {
		return "", false
	}
	return c.Value, true
}

// Authenticator validates access tokens.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (reqctx.Principal, error)
}

// Authenticate resolves the principal from the access cookie or a bearer
// token. With required=false anonymous requests pass through.
func Authenticate(auth Authenticator, names CookieNames, required bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			r := c.Request()
			token, ok := cookie(r, names.Access)
			if !ok {
				if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
					token, ok = strings.TrimPrefix(h, "Bearer "), true
				}
			}
			if !ok {
				if required {
					return apperror.Unauthenticated("")
				}
				return next(c)
			}
			p, err := auth.Authenticate(r.Context(), token)
			if err != nil {
				if required {
					return err
				}
				return next(c) // optional auth: treat invalid tokens as anonymous
			}
			ctx := reqctx.WithPrincipal(r.Context(), p)
			ctx = logger.With(ctx, zap.String("user_id", p.UserID.String()), zap.String("role", string(p.Role)))
			c.SetRequest(r.WithContext(ctx))
			return next(c)
		}
	}
}

// RequireRole enforces RBAC (spec §2).
func RequireRole(roles ...domain.Role) echo.MiddlewareFunc {
	allowed := map[domain.Role]bool{}
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			p, ok := reqctx.PrincipalFrom(c.Request().Context())
			if !ok {
				return apperror.Unauthenticated("")
			}
			if !allowed[p.Role] {
				logger.FromContext(c.Request().Context()).Warn("rbac: forbidden", zap.String("role", string(p.Role)), zap.String("route", c.Path()))
				return apperror.Forbidden("")
			}
			return next(c)
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
