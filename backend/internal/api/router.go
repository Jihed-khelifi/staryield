// Package api assembles the HTTP server: middleware ordering, route table
// and role-based access per route group.
package api

import (
	"context"
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
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
)

// Deps are the router's collaborators.
type Deps struct {
	Config    *config.Config
	Log       *zap.Logger
	Handler   *handler.Handler
	Auth      mw.Authenticator
	AdminAuth AdminAuth
	CSRF      *security.CSRF
	Limiter   ratelimit.Limiter
	Cookies   mw.CookieNames
	// DevObjects serves public in-memory buckets in development (memory
	// storage driver only; never set in production, never the chat bucket).
	DevObjects map[string]storage.ObjectStore
}

// AdminAuth authenticates admin console sessions and enforces step-up.
type AdminAuth interface {
	mw.AdminAuthenticator
	RequireStepUp(ctx context.Context) error
}

// uploadRoutes accept multipart bodies larger than the JSON body limit;
// their handlers enforce per-route limits.
var uploadRoutes = map[string]bool{
	"/api/v1/me/avatar":                     true,
	"/api/v1/admin/psychics/:id/image":      true,
	"/api/v1/chat/sessions/:id/attachments": true,
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
		return mw.CookieNames{Access: access, Refresh: "__Secure-sy_rt", CSRF: "__Secure-sy_csrf", Admin: "__Host-sy_adm"}
	}
	return mw.CookieNames{Access: "sy_at", Refresh: "sy_rt", CSRF: "sy_csrf", Admin: "sy_adm"}
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
	adminOrigins := map[string]bool{}
	corsOrigins := append([]string{}, cfg.Security.AllowedOrigins...)
	for _, o := range cfg.Security.AllowedOrigins {
		allowed[strings.ToLower(o)] = true
	}
	for _, o := range cfg.Security.AdminOrigins {
		allowed[strings.ToLower(o)] = true
		adminOrigins[strings.ToLower(o)] = true
		corsOrigins = append(corsOrigins, o)
	}
	var adminNets []*net.IPNet
	for _, cidr := range cfg.Security.AdminIPAllowlist {
		if _, n, err := net.ParseCIDR(cidr); err == nil {
			adminNets = append(adminNets, n)
		}
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
		AllowOrigins:     corsOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", mw.CSRFHeader, mw.HeaderRequestID, "Idempotency-Key", "Authorization", "Last-Event-ID", "X-Locale"},
		ExposeHeaders:    []string{mw.HeaderRequestID, "Retry-After", mw.CSRFHeader, "RateLimit-Remaining"},
		AllowCredentials: true,
		MaxAge:           600,
	}))
	e.Use(mw.BodyLimit(bodyLimit, func(c echo.Context) bool { return uploadRoutes[c.Path()] }))
	e.Use(mw.Timeout(cfg.HTTP.RequestTimeout, isStreaming))
	e.Use(mw.RateLimit(limiter, rule("global", cfg.RateLimit.Global, scale), mw.ByIP, cfg.RateLimit.FailOpen))
	e.Use(mw.CSRF(d.CSRF, d.Cookies, allowed, func(c echo.Context) bool {
		return strings.HasPrefix(c.Path(), "/api/v1/webhooks/")
	}))

	e.GET("/healthz", h.Healthz)
	e.GET("/readyz", h.Readyz)
	if len(d.DevObjects) > 0 && !cfg.IsProduction() {
		e.GET("/_dev/storage/:bucket/*", devObject(d.DevObjects))
	}
	// Marketing short links (recorded, then redirected to the landing page).
	e.GET("/l/:code", h.ShortLink, mw.RateLimit(limiter, rule("links", cfg.RateLimit.Ingest, scale), mw.ByIP, true))

	v1 := e.Group("/api/v1")
	optional := mw.Authenticate(d.Auth, d.Cookies, false)
	authed := mw.Authenticate(d.Auth, d.Cookies, true)
	authRL := mw.RateLimit(limiter, rule("auth", cfg.RateLimit.Auth, scale), mw.ByIP, false)
	writeRL := mw.RateLimit(limiter, rule("writes", cfg.RateLimit.Writes, scale), mw.ByUserOrIP, cfg.RateLimit.FailOpen)
	ingestRL := mw.RateLimit(limiter, rule("ingest", cfg.RateLimit.Ingest, scale), mw.ByIP, true)
	uploadRL := mw.RateLimit(limiter, rule("upload", cfg.RateLimit.Upload, scale), mw.ByUserOrIP, cfg.RateLimit.FailOpen)
	client := mw.RequireRole(domain.RoleClient)
	psychic := mw.RequireRole(domain.RolePsychic)

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
	v1.GET("/psychics/:slug/reviews", h.PsychicReviews)
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
	me.GET("/notifications", h.Notifications)
	me.GET("/notifications/unread-count", h.UnreadNotifications)
	me.POST("/notifications/read", h.MarkNotificationsRead, writeRL)

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
	ch.GET("/conversations/:id/client", h.ClientProfile, psychic)
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
	ch.POST("/sessions/:id/attachments", h.SendImage, uploadRL)
	ch.POST("/sessions/:id/review", h.SubmitReview, client, writeRL)
	ch.GET("/attachments/:id", h.Attachment)
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
	ps.GET("/analytics", h.MyAnalytics)
	ps.GET("/reviews", h.MyReviews)
	ps.GET("/withdrawals", h.MyWithdrawals)
	ps.POST("/withdrawals", h.RequestWithdrawal, writeRL)
	ps.POST("/withdrawals/:id/cancel", h.CancelWithdrawal, writeRL)

	// Administration (FR-13, FR-14). Admin routes are only reachable from
	// the admin console origin (and allow-listed IPs), with a dedicated
	// password + TOTP session; sensitive actions require a fresh TOTP code.
	adm := v1.Group("/admin", mw.AdminOrigin(adminOrigins), mw.IPAllowlist(adminNets))
	adm.POST("/auth/login", h.AdminLogin, authRL)
	adm.POST("/auth/enroll", h.AdminEnroll, authRL)
	adm.POST("/auth/enroll/confirm", h.AdminEnrollConfirm, authRL)
	adm.POST("/auth/verify", h.AdminVerify, authRL)
	adm.POST("/auth/logout", h.AdminLogout)

	ad := adm.Group("", mw.AdminSession(d.AdminAuth, d.Cookies.Admin))
	stepUp := mw.StepUp(d.AdminAuth.RequireStepUp)
	adminRL := mw.RateLimit(limiter, rule("admin", cfg.RateLimit.Writes, scale*4), mw.ByUserOrIP, cfg.RateLimit.FailOpen)
	ad.GET("/auth/session", h.AdminSessionInfo)
	ad.POST("/auth/step-up", h.AdminStepUp, authRL)
	ad.GET("/auth/sessions", h.AdminAuthSessions)
	ad.DELETE("/auth/sessions/:id", h.AdminRevokeAuthSession)
	ad.POST("/auth/recovery-codes", h.AdminRecoveryCodes, stepUp)

	ad.GET("/users", h.AdminUsers)
	ad.POST("/users", h.AdminCreateUser, adminRL)
	ad.GET("/users/:id", h.AdminUser)
	ad.PATCH("/users/:id", h.AdminUpdateUser, adminRL)
	ad.DELETE("/users/:id", h.AdminDeleteUser, stepUp, adminRL)
	ad.POST("/users/:id/credits", h.AdminAdjustCredits, stepUp, adminRL)
	ad.POST("/users/:id/logout-all", h.AdminLogoutUser, adminRL)
	ad.GET("/psychics", h.AdminPsychics)
	ad.POST("/psychics", h.AdminCreatePsychic, adminRL)
	ad.GET("/psychics/:id", h.AdminPsychic)
	ad.PATCH("/psychics/:id", h.AdminUpdatePsychic, adminRL)
	ad.DELETE("/psychics/:id", h.AdminDeletePsychic, stepUp, adminRL)
	ad.POST("/psychics/:id/image", h.AdminUploadPsychicImage, uploadRL)
	ad.DELETE("/psychics/:id/image", h.AdminDeletePsychicImage, adminRL)
	ad.GET("/psychics/:id/earnings", h.AdminPsychicEarnings)
	ad.GET("/psychics/:id/analytics", h.AdminPsychicAnalytics)
	ad.POST("/psychics/:id/services", h.AdminCreateService, adminRL)
	ad.PATCH("/services/:id", h.AdminUpdateService, adminRL)
	ad.DELETE("/services/:id", h.AdminDeleteService, adminRL)
	ad.GET("/credit-packs", h.AdminPacks)
	ad.POST("/credit-packs", h.AdminCreatePack, adminRL)
	ad.PATCH("/credit-packs/:id", h.AdminUpdatePack, adminRL)
	ad.DELETE("/credit-packs/:id", h.AdminDeletePack, adminRL)
	ad.GET("/custom-requests", h.AdminRequests)
	ad.POST("/custom-requests/:id/resolve", h.AdminResolveRequest, adminRL)
	ad.GET("/chat/sessions", h.AdminSessions)
	ad.GET("/chat/sessions/:id", h.AdminSession)
	ad.GET("/chat/sessions/:id/transcript", h.AdminTranscript, stepUp)
	ad.POST("/chat/sessions/:id/terminate", h.AdminTerminateSession, adminRL)
	ad.GET("/reviews", h.AdminReviews)
	ad.PATCH("/reviews/:id", h.AdminModerateReview, adminRL)
	ad.GET("/withdrawals", h.AdminWithdrawals)
	ad.GET("/withdrawals/:id/destination", h.AdminWithdrawalDestination, stepUp)
	ad.POST("/withdrawals/:id/approve", h.AdminApproveWithdrawal, stepUp, adminRL)
	ad.POST("/withdrawals/:id/pay", h.AdminPayWithdrawal, stepUp, adminRL)
	ad.POST("/withdrawals/:id/reject", h.AdminRejectWithdrawal, adminRL)
	ad.GET("/presence", h.AdminPresence)
	ad.GET("/broadcasts", h.AdminBroadcasts)
	ad.POST("/broadcasts", h.AdminBroadcast, adminRL)
	ad.GET("/broadcasts/:id", h.AdminBroadcastDetail)
	ad.GET("/campaigns", h.AdminCampaigns)
	ad.POST("/campaigns", h.AdminCreateCampaign, adminRL)
	ad.GET("/campaigns/:id", h.AdminCampaign)
	ad.PATCH("/campaigns/:id", h.AdminUpdateCampaign, adminRL)
	ad.DELETE("/campaigns/:id", h.AdminDeleteCampaign, adminRL)
	ad.GET("/campaigns/:id/report", h.AdminCampaignReport)
	ad.GET("/links", h.AdminLinks)
	ad.GET("/links/settings", h.AdminLinkSettings)
	ad.POST("/links", h.AdminCreateLink, adminRL)
	ad.GET("/links/:id", h.AdminLink)
	ad.PATCH("/links/:id", h.AdminUpdateLink, adminRL)
	ad.DELETE("/links/:id", h.AdminDeleteLink, adminRL)
	ad.GET("/links/:id/report", h.AdminLinkReport)
	ad.GET("/acquisition/overview", h.AdminAcquisition)
	ad.GET("/insights/audience", h.AdminAudience)
	ad.GET("/insights/engagement", h.AdminEngagement)
	ad.GET("/insights/retention", h.AdminRetention)
	ad.GET("/insights/top-users", h.AdminTopUsers)
	ad.GET("/insights/users/:id", h.AdminUserInsights)
	ad.GET("/reports/overview", h.AdminOverview)
	ad.GET("/reports/daily", h.AdminDaily)
	ad.GET("/analytics/timeseries", h.AdminTimeseries)
	ad.GET("/analytics/funnel", h.AdminFunnel)
	ad.GET("/analytics/top", h.AdminTopEvents)
	ad.GET("/analytics/events", h.AdminEvents)
	ad.GET("/audit-logs", h.AdminAudit)

	return e
}

// devObject serves objects of public in-memory buckets so uploaded images
// display during local development.
func devObject(stores map[string]storage.ObjectStore) echo.HandlerFunc {
	return func(c echo.Context) error {
		store, ok := stores[c.Param("bucket")]
		if !ok {
			return echo.ErrNotFound
		}
		body, ct, err := store.Get(c.Request().Context(), c.Param("*"), 16<<20)
		if err != nil {
			return echo.ErrNotFound
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		c.Response().Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
		return c.Blob(http.StatusOK, ct, body)
	}
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
	switch cfg.IPSource {
	case "xff":
		return echo.ExtractIPFromXFFHeader(append(opts, echo.TrustLoopback(false), echo.TrustLinkLocal(false), echo.TrustPrivateNet(false))...)
	case "x-real-ip":
		return headerFromTrusted("X-Real-IP", nets) // Railway's edge
	case "cloudflare":
		return headerFromTrusted("CF-Connecting-IP", nets)
	}
	return echo.ExtractIPDirect()
}

// headerFromTrusted reads the client IP from a single-value header set by a
// reverse proxy, honoured only when the TCP peer is one of the trusted
// proxies so clients cannot spoof it.
func headerFromTrusted(header string, nets []*net.IPNet) echo.IPExtractor {
	direct := echo.ExtractIPDirect()
	return func(r *http.Request) string {
		peer := direct(r)
		if ip := strings.TrimSpace(r.Header.Get(header)); ip != "" && net.ParseIP(ip) != nil && inNets(peer, nets) {
			return ip
		}
		return peer
	}
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
