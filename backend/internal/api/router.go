// Package api assembles the HTTP server: middleware ordering, route table
// and role-based access per route group.
package api

import (
	"net"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/api/handler"
	mw "github.com/Jihed-khelifi/staryield/backend/internal/api/middleware"
	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/ratelimit"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// Deps are the router's collaborators.
type Deps struct {
	Config  *config.Config
	Log     *zap.Logger
	Handler *handler.Handler
	Auth    mw.Authenticator
	CSRF    *security.CSRF
	Limiter ratelimit.Limiter
	Cookies mw.CookieNames
}

// CookieNamesFor returns cookie names, using the __Host-/__Secure- prefixes
// whenever cookies are Secure (they then cannot be set by sub-domains or
// over plain HTTP).
func CookieNamesFor(cfg config.SecurityConfig) mw.CookieNames {
	if cfg.CookieSecure {
		access := "__Secure-sy_at"
		if cfg.CookieDomain == "" {
			access = "__Host-sy_at"
		}
		return mw.CookieNames{Access: access, Refresh: "__Secure-sy_rt", CSRF: "__Secure-sy_csrf"}
	}
	return mw.CookieNames{Access: "sy_at", Refresh: "sy_rt", CSRF: "sy_csrf"}
}

func rule(name string, r config.Rule, scale float64) ratelimit.Rule {
	return ratelimit.Rule{Name: name, Rate: r.Rate, Period: r.Period, Burst: r.Burst}.Scaled(scale)
}

func isStreaming(c echo.Context) bool {
	p := c.Path()
	return p == "/api/v1/realtime/ws" || p == "/api/v1/realtime/stream"
}

// NewRouter builds the Echo instance.
func NewRouter(d Deps) *echo.Echo {
	cfg := d.Config
	e := echo.New()
	e.HideBanner, e.HidePort = true, true
	e.HTTPErrorHandler = ErrorHandler
	e.IPExtractor = ipExtractor(cfg.HTTP)

	allowed := map[string]bool{}
	for _, o := range cfg.Security.AllowedOrigins {
		allowed[strings.ToLower(o)] = true
	}
	bodyLimit := parseSize(cfg.HTTP.BodyLimit, 64<<10)
	h := d.Handler
	scale := cfg.RateLimit.Scale
	limiter := d.Limiter
	if !cfg.RateLimit.Enabled {
		limiter = nil
	}

	// Order matters: correlation first, then logging (which renders errors),
	// panic recovery, headers, CORS, then request shaping and protections.
	e.Use(mw.RequestID())
	e.Use(mw.RequestContext(d.Log, cfg.Analytics.IPHashSecret))
	e.Use(mw.AccessLog(nil))
	e.Use(mw.Recover())
	e.Use(mw.SecurityHeaders(cfg.Security.HSTS))
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins:     cfg.Security.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", mw.CSRFHeader, mw.HeaderRequestID, "Idempotency-Key", "Authorization", "Last-Event-ID", "X-Locale"},
		ExposeHeaders:    []string{mw.HeaderRequestID, "Retry-After", mw.CSRFHeader, "RateLimit-Remaining"},
		AllowCredentials: true,
		MaxAge:           600,
	}))
	e.Use(mw.BodyLimit(bodyLimit, func(c echo.Context) bool { return c.Path() == "/api/v1/me/avatar" }))
	e.Use(mw.Timeout(cfg.HTTP.RequestTimeout, isStreaming))
	e.Use(mw.RateLimit(limiter, rule("global", cfg.RateLimit.Global, scale), mw.ByIP, cfg.RateLimit.FailOpen))
	e.Use(mw.CSRF(d.CSRF, d.Cookies, allowed, func(c echo.Context) bool {
		return strings.HasPrefix(c.Path(), "/api/v1/webhooks/")
	}))

	e.GET("/healthz", h.Healthz)
	e.GET("/readyz", h.Readyz)

	v1 := e.Group("/api/v1")
	optional := mw.Authenticate(d.Auth, d.Cookies, false)
	authed := mw.Authenticate(d.Auth, d.Cookies, true)
	authRL := mw.RateLimit(limiter, rule("auth", cfg.RateLimit.Auth, scale), mw.ByIP, false)
	writeRL := mw.RateLimit(limiter, rule("writes", cfg.RateLimit.Writes, scale), mw.ByUserOrIP, cfg.RateLimit.FailOpen)
	ingestRL := mw.RateLimit(limiter, rule("ingest", cfg.RateLimit.Ingest, scale), mw.ByIP, true)
	uploadRL := mw.RateLimit(limiter, rule("upload", cfg.RateLimit.Upload, scale), mw.ByUserOrIP, cfg.RateLimit.FailOpen)
	client := mw.RequireRole(domain.RoleClient)
	psychic := mw.RequireRole(domain.RolePsychic)
	admin := mw.RequireRole(domain.RoleAdmin)

	v1.GET("/time", h.ServerTime)

	// Authentication (FR-02).
	a := v1.Group("/auth")
	a.GET("/csrf", h.CSRFToken)
	a.POST("/register", h.Register, authRL)
	a.POST("/login", h.Login, authRL)
	a.POST("/refresh", h.Refresh, authRL)
	a.POST("/logout", h.Logout)
	a.POST("/logout-all", h.LogoutAll, authed)
	a.POST("/password/forgot", h.ForgotPassword, authRL)
	a.POST("/password/reset", h.ResetPassword, authRL)
	a.POST("/password/change", h.ChangePassword, authed, authRL)

	// Public catalogue and calculators (FR-01, FR-06).
	v1.GET("/psychics", h.Psychics)
	v1.GET("/psychics/:slug", h.Psychic)
	v1.GET("/credit-packs", h.CreditPacks)
	v1.GET("/astrology/places", h.Places)
	v1.POST("/astrology/chart", h.PublicChart)
	v1.GET("/astrology/horoscope/:sign", h.SignHoroscope)
	v1.GET("/astrology/events", h.CosmicEvents)
	v1.POST("/analytics/events", h.IngestEvents, optional, ingestRL)
	v1.POST("/client-logs", h.ClientLog, optional, ingestRL)
	v1.POST("/webhooks/payments", h.PaymentWebhook)

	// Signed-in user.
	me := v1.Group("/me", authed)
	me.GET("", h.Me)
	me.PATCH("/profile", h.UpdateProfile, writeRL)
	me.POST("/avatar", h.UploadAvatar, uploadRL)
	me.DELETE("/avatar", h.DeleteAvatar)
	me.GET("/chart", h.MyChart)
	me.GET("/horoscope", h.MyHoroscope)

	v1.GET("/wallet", h.Wallet, authed)
	v1.GET("/wallet/ledger", h.Ledger, authed)
	v1.POST("/credits/purchases", h.Purchase, authed, client, writeRL)
	v1.GET("/credits/purchases", h.Purchases, authed, client)

	// Realtime (WebSocket primary, SSE fallback).
	v1.POST("/realtime/ticket", h.RealtimeTicket, authed)
	v1.GET("/realtime/ws", h.RealtimeWS)
	v1.GET("/realtime/stream", h.RealtimeStream, authed)
	v1.POST("/realtime/presence", h.RealtimePresence, authed)

	// Live chat (FR-09–FR-12).
	ch := v1.Group("/chat", authed)
	ch.GET("/conversations", h.Conversations)
	ch.GET("/conversations/:id/messages", h.Messages)
	ch.GET("/conversations/:id/messages/since", h.MessagesSince)
	ch.POST("/conversations/:id/read", h.MarkRead)
	ch.GET("/psychics/:slug", h.Thread, client)
	ch.POST("/sessions", h.StartSession, client, writeRL)
	ch.GET("/sessions", h.Sessions)
	ch.GET("/sessions/:id", h.Session)
	ch.GET("/sessions/:id/summary", h.SessionSummary)
	ch.POST("/sessions/:id/accept", h.AcceptSession, psychic)
	ch.POST("/sessions/:id/decline", h.DeclineSession, psychic)
	ch.POST("/sessions/:id/cancel", h.CancelSession, client)
	ch.POST("/sessions/:id/end", h.EndSession)
	ch.POST("/sessions/:id/messages", h.SendMessage)
	ch.POST("/sessions/:id/typing", h.Typing)
	ch.GET("/saved", h.SavedMessages)
	ch.PUT("/saved/:messageId", h.SaveMessage)
	ch.DELETE("/saved/:messageId", h.UnsaveMessage)

	// Custom requests (FR-07, §5.1).
	cr := v1.Group("/custom-requests", authed)
	cr.GET("", h.ListRequests)
	cr.POST("", h.OrderRequest, client, writeRL)
	cr.GET("/:id", h.GetRequest)
	cr.POST("/:id/cancel", h.CancelRequest, client)
	cr.POST("/:id/dispute", h.DisputeRequest, client)
	cr.POST("/:id/accept", h.AcceptRequest, psychic)
	cr.POST("/:id/decline", h.DeclineRequest, psychic)
	cr.POST("/:id/deliver", h.DeliverRequest, psychic)

	// Psychic tools.
	ps := v1.Group("/psychic", authed, psychic)
	ps.PATCH("/availability", h.SetAvailability)
	ps.PATCH("/profile", h.UpdatePsychicProfile)
	ps.GET("/earnings", h.Earnings)

	// Administration (FR-13, FR-14).
	ad := v1.Group("/admin", authed, admin)
	ad.GET("/users", h.AdminUsers)
	ad.GET("/users/:id", h.AdminUser)
	ad.PATCH("/users/:id", h.AdminUpdateUser)
	ad.POST("/users/:id/credits", h.AdminAdjustCredits)
	ad.GET("/psychics", h.AdminPsychics)
	ad.POST("/psychics", h.AdminCreatePsychic)
	ad.PATCH("/psychics/:id", h.AdminUpdatePsychic)
	ad.GET("/psychics/:id/earnings", h.AdminPsychicEarnings)
	ad.POST("/psychics/:id/services", h.AdminCreateService)
	ad.PATCH("/services/:id", h.AdminUpdateService)
	ad.GET("/credit-packs", h.AdminPacks)
	ad.POST("/credit-packs", h.AdminCreatePack)
	ad.PATCH("/credit-packs/:id", h.AdminUpdatePack)
	ad.GET("/custom-requests", h.AdminRequests)
	ad.POST("/custom-requests/:id/resolve", h.AdminResolveRequest)
	ad.GET("/chat/sessions", h.AdminSessions)
	ad.GET("/chat/sessions/:id", h.AdminSession)
	ad.GET("/chat/sessions/:id/transcript", h.AdminTranscript)
	ad.POST("/chat/sessions/:id/terminate", h.AdminTerminateSession)
	ad.GET("/reports/overview", h.AdminOverview)
	ad.GET("/reports/daily", h.AdminDaily)
	ad.GET("/analytics/timeseries", h.AdminTimeseries)
	ad.GET("/analytics/funnel", h.AdminFunnel)
	ad.GET("/analytics/top", h.AdminTopEvents)
	ad.GET("/analytics/events", h.AdminEvents)
	ad.GET("/audit-logs", h.AdminAudit)

	return e
}

// ipExtractor trusts forwarding headers only from configured proxies.
func ipExtractor(cfg config.HTTPConfig) echo.IPExtractor {
	var opts []echo.TrustOption
	var nets []*net.IPNet
	for _, cidr := range cfg.TrustedProxies {
		if _, n, err := net.ParseCIDR(cidr); err == nil {
			opts = append(opts, echo.TrustIPRange(n))
			nets = append(nets, n)
		}
	}
	direct := echo.ExtractIPDirect()
	switch cfg.IPSource {
	case "xff":
		return echo.ExtractIPFromXFFHeader(append(opts, echo.TrustLoopback(false), echo.TrustLinkLocal(false), echo.TrustPrivateNet(false))...)
	case "cloudflare":
		return func(r *http.Request) string {
			peer := direct(r)
			if ip := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); ip != "" && net.ParseIP(ip) != nil && inNets(peer, nets) {
				return ip
			}
			return peer
		}
	}
	return direct
}

func inNets(ip string, nets []*net.IPNet) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(parsed) {
			return true
		}
	}
	return false
}

func parseSize(s string, def int64) int64 {
	s = strings.ToUpper(strings.TrimSpace(s))
	mult := int64(1)
	switch {
	case strings.HasSuffix(s, "KB"):
		mult, s = 1<<10, strings.TrimSuffix(s, "KB")
	case strings.HasSuffix(s, "MB"):
		mult, s = 1<<20, strings.TrimSuffix(s, "MB")
	case strings.HasSuffix(s, "B"):
		s = strings.TrimSuffix(s, "B")
	}
	var n int64
	for _, r := range s {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int64(r-'0')
	}
	if n <= 0 {
		return def
	}
	return n * mult
}
