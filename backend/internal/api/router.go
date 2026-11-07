// Package api assembles the HTTP server: middleware ordering, route table
// and role-based access per route group.
package api

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

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
	"/api/v1/psychic/photo":                 true, // advisor settings
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
		AllowHeaders:     []string{"Content-Type", mw.CSRFHeader, mw.HeaderRequestID, "Idempotency-Key", "Authorization", "Last-Event-ID", "X-Locale", "X-Support-Token"}, // revisions 3.0: support desk (private link)
		ExposeHeaders:    []string{mw.HeaderRequestID, "Retry-After", mw.CSRFHeader, "RateLimit-Remaining"},
		AllowCredentials: true,
		MaxAge:           600,
	}))
	e.Use(mw.BodyLimit(bodyLimit, func(c echo.Context) bool { return uploadRoutes[c.Path()] }))
	e.Use(mw.Timeout(cfg.HTTP.RequestTimeout, isStreaming))
	e.Use(mw.RateLimit(limiter, rule("global", cfg.RateLimit.Global, scale), mw.ByIP, cfg.RateLimit.FailOpen))
	e.Use(mw.CSRF(d.CSRF, d.Cookies, allowed, func(c echo.Context) bool {
		// astrology: /aztro accepts POST only for parity with the aztro API;
		// it is stateless, reads no cookies and returns public data.
		return strings.HasPrefix(c.Path(), "/api/v1/webhooks/") || c.Path() == "/api/v1/aztro"
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
	featuredRL := mw.RateLimit(limiter, rule("featured", config.Rule{Rate: 120, Period: time.Minute, Burst: 60}, scale), mw.ByIP, true)
	v1.GET("/psychics/featured", h.FeaturedPsychics, featuredRL)
	v1.GET("/psychics/:slug", h.Psychic)
	v1.GET("/psychics/:slug/reviews", h.PsychicReviews)
	// Waiting lists: told once when the advisor is online (clients only).
	waitlistRL := mw.RateLimit(limiter, rule("waitlist", config.Rule{Rate: 30, Period: time.Hour, Burst: 10}, scale), mw.ByUserOrIP, cfg.RateLimit.FailOpen)
	v1.POST("/psychics/:slug/waitlist", h.JoinWaitlist, authed, client, waitlistRL)
	v1.DELETE("/psychics/:slug/waitlist", h.LeaveWaitlist, authed, waitlistRL)
	v1.GET("/credit-packs", h.CreditPacks)
	// astrology: stateless calculators (birth data in POST bodies only,
	// answered no-store), the gazetteer and the public horoscope. The
	// buckets are code constants; when Redis is unavailable they fall back
	// to per-instance limits rather than failing open or closed.
	astroLimiter := limiter
	if limiter != nil {
		astroLimiter = ratelimit.WithFallback(limiter, ratelimit.NewMemoryLimiter(nil))
	}
	astroRL := mw.RateLimit(astroLimiter, ratelimit.Rule{Name: "astro", Rate: 30, Period: time.Minute, Burst: 10}.Scaled(scale), mw.ByIP, false)
	placesRL := mw.RateLimit(astroLimiter, ratelimit.Rule{Name: "places", Rate: 120, Period: time.Minute, Burst: 40}.Scaled(scale), mw.ByIP, false)
	aztroRL := mw.RateLimit(astroLimiter, ratelimit.Rule{Name: "aztro", Rate: 60, Period: time.Minute, Burst: 20}.Scaled(scale), mw.ByIP, false)
	v1.GET("/astrology/places", h.Places, placesRL)
	v1.GET("/astrology/places/:id", h.Place, placesRL)
	v1.POST("/astrology/natal", h.Natal, astroRL)
	v1.POST("/astrology/numerology", h.Numerology, astroRL)
	v1.POST("/astrology/sun-sign", h.SunSign, astroRL)
	v1.POST("/astrology/chart", h.PublicChart, astroRL)
	v1.GET("/astrology/horoscope/:sign", h.SignHoroscope, aztroRL)
	v1.GET("/astrology/events", h.CosmicEvents, aztroRL)
	v1.GET("/aztro", h.Aztro, aztroRL)
	v1.POST("/aztro", h.Aztro, aztroRL)
	v1.POST("/analytics/events", h.IngestEvents, optional, ingestRL)
	v1.POST("/client-logs", h.ClientLog, optional, ingestRL)
	// Contact form: public (optionally signed in), 5 per hour per IP and per user.
	supportIPRL := mw.RateLimit(limiter, rule("support_ip", config.Rule{Rate: 5, Period: time.Hour, Burst: 5}, scale), mw.ByIP, cfg.RateLimit.FailOpen)
	supportUserRL := mw.RateLimit(limiter, rule("support_user", config.Rule{Rate: 5, Period: time.Hour, Burst: 5}, scale), mw.ByUserOrIP, cfg.RateLimit.FailOpen)
	v1.POST("/support/tickets", h.CreateSupportTicket, optional, supportIPRL, supportUserRL)
	// revisions 3.0: support desk. Requesters read and answer their tickets
	// signed in (/me/support/tickets…, matched on user_id only) or through the
	// private link e-mailed to them (X-Support-Token; the same 404 for every
	// failure). Replies are limited per user (or per IP), link lookups per IP.
	supportReplyRL := mw.RateLimit(limiter, rule("support_reply", config.Rule{Rate: 20, Period: time.Hour, Burst: 20}, scale), mw.ByUserOrIP, cfg.RateLimit.FailOpen)
	supportLinkRL := mw.RateLimit(limiter, rule("support_link", config.Rule{Rate: 30, Period: time.Minute, Burst: 30}, scale), mw.ByIP, false)
	v1.GET("/support/tickets/:reference", h.SupportTicketByLink, supportLinkRL)
	v1.POST("/support/tickets/:reference/messages", h.ReplySupportTicketByLink, supportLinkRL, supportReplyRL)
	// end revisions 3.0: support desk
	v1.POST("/webhooks/payments", h.PaymentWebhook)
	// revisions 3.0: site settings. Public configuration (promo banners,
	// sign-up offer, free mode): cacheable, per IP, fails open so the site
	// never breaks because of the limiter. The bucket is generous because the
	// web app's server renders reach it from a few shared edge addresses (it
	// also caches the answer per isolate); the reply comes from memory.
	siteRL := mw.RateLimit(limiter, rule("site", config.Rule{Rate: 3000, Period: time.Minute, Burst: 600}, scale), mw.ByIP, true)
	v1.GET("/site/config", h.SiteConfig, siteRL)
	// --- end site settings ---

	// Signed-in user.
	me := v1.Group("/me", authed)
	me.GET("", h.Me)
	me.PATCH("/profile", h.UpdateProfile, writeRL)
	me.POST("/avatar", h.UploadAvatar, uploadRL)
	me.DELETE("/avatar", h.DeleteAvatar)
	// astrology
	me.GET("/chart", h.MyChart)
	me.GET("/horoscope", h.MyHoroscope)
	me.GET("/waitlist", h.MyWaitlist)
	me.GET("/numerology", h.MyNumerology)
	// revisions 3.0: support desk
	me.GET("/support/tickets", h.MySupportTickets)
	me.GET("/support/tickets/:reference", h.MySupportTicket)
	me.POST("/support/tickets/:reference/messages", h.ReplyMySupportTicket, supportReplyRL)
	// end revisions 3.0: support desk
	me.GET("/notifications", h.Notifications)
	me.GET("/notifications/unread-count", h.UnreadNotifications)
	me.POST("/notifications/read", h.MarkNotificationsRead, writeRL)

	// --- Web Push (M0 delivery channels) ---
	// Public VAPID key for browsers; per-user browser subscriptions.
	pushRL := mw.RateLimit(limiter, rule("push", cfg.RateLimit.Push, scale), mw.ByUserOrIP, cfg.RateLimit.FailOpen)
	v1.GET("/push/config", h.PushConfig)
	me.POST("/push-subscriptions", h.SubscribePush, pushRL)
	me.DELETE("/push-subscriptions", h.UnsubscribePush, pushRL)
	// --- end Web Push ---

	// --- identity: settings, devices, e-mail change, consents, exports,
	// deactivation and closure. Re-authenticated, abuse-prone actions share
	// a small hourly budget that fails closed.
	sensitiveRL := mw.RateLimit(limiter, rule("sensitive", config.Rule{Rate: 5, Period: time.Hour, Burst: 5}, scale), mw.ByUserOrIP, false)
	v1.GET("/consent-policy", h.ConsentPolicy)
	me.GET("/settings", h.Settings)
	me.PATCH("/settings", h.UpdateSettings, writeRL)
	for _, base := range []string{"/sessions", "/devices"} {
		me.GET(base, h.Devices)
		me.DELETE(base, h.RevokeDevices, writeRL)
		me.DELETE(base+"/:id", h.RevokeDevice, writeRL)
	}
	me.POST("/email-change", h.RequestEmailChange, sensitiveRL)
	a.POST("/email-change/confirm", h.ConfirmEmailChange, authRL)
	a.POST("/email-change/cancel", h.CancelEmailChange, authRL)
	me.GET("/consents", h.Consents)
	me.PUT("/consents", h.UpdateConsents, writeRL)
	me.POST("/exports", h.RequestExport, sensitiveRL)
	me.GET("/exports", h.Exports)
	me.GET("/exports/:id", h.Export)
	me.POST("/exports/:id/download-token", h.ExportDownloadToken, sensitiveRL)
	me.GET("/exports/:id/download", h.DownloadExport, writeRL)
	me.POST("/deactivate", h.Deactivate, sensitiveRL)
	me.POST("/close", h.CloseAccount, sensitiveRL)
	a.POST("/reactivate", h.Reactivate, authRL)
	a.POST("/closure/cancel", h.CancelClosure, authRL)
	// --- end identity ---

	v1.GET("/wallet", h.Wallet, authed)
	v1.GET("/wallet/ledger", h.Ledger, authed)
	v1.POST("/credits/purchases", h.Purchase, authed, client, writeRL)
	v1.GET("/credits/purchases", h.Purchases, authed, client)

	// billing: Premium Membership, saved cards and payment history. Card
	// data never reaches these routes (provider tokens only); attaching
	// cards and subscribing are tightly rate limited (card testing).
	billingRL := mw.RateLimit(limiter, rule("billing", cfg.Payments.RateLimit, scale), mw.ByUserOrIP, false)
	v1.GET("/billing/plans", h.BillingPlans)
	v1.POST("/webhooks/payments/:provider", h.ProviderWebhook)
	bl := v1.Group("/billing", authed, client)
	bl.GET("/client-config", h.BillingClientConfig)
	bl.GET("/payment-methods", h.PaymentMethods)
	bl.POST("/payment-methods", h.AttachPaymentMethod, billingRL)
	bl.DELETE("/payment-methods/:id", h.DeletePaymentMethod, writeRL)
	bl.POST("/payment-methods/:id/default", h.SetDefaultPaymentMethod, writeRL)
	bl.GET("/subscription", h.Subscription)
	bl.POST("/subscriptions", h.Subscribe, billingRL)
	bl.PATCH("/subscription", h.UpdateSubscription, billingRL)
	bl.POST("/subscription/cancel", h.CancelSubscription, writeRL)
	bl.POST("/subscription/resume", h.ResumeSubscription, writeRL)
	bl.GET("/transactions", h.Transactions)

	// Realtime (WebSocket primary, SSE fallback).
	v1.POST("/realtime/ticket", h.RealtimeTicket, authed)
	v1.GET("/realtime/ws", h.RealtimeWS)
	v1.GET("/realtime/stream", h.RealtimeStream, authed)
	v1.POST("/realtime/presence", h.RealtimePresence, authed)

	// Live chat (FR-09–FR-12).
	ch := v1.Group("/chat", authed)
	ch.GET("/conversations", h.Conversations)
	ch.GET("/unread-count", h.ChatUnreadCount) // revisions 3.0: chat unread
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

	// Client notes: advisors only; writes have their own hourly bucket.
	noteRL := mw.RateLimit(limiter, rule("notes", config.Rule{Rate: 30, Period: time.Hour, Burst: 10}, scale), mw.ByUserOrIP, cfg.RateLimit.FailOpen)
	ch.GET("/conversations/:id/notes", h.ConversationNotes, psychic)
	ch.POST("/conversations/:id/notes", h.CreateConversationNote, psychic, noteRL)
	ch.GET("/conversations/:id/notes/shared", h.SharedConversationNotes, psychic)
	ch.PATCH("/notes/:id", h.UpdateNote, psychic, noteRL)
	ch.DELETE("/notes/:id", h.DeleteNote, psychic, noteRL)

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
	ps.GET("/stats/overview", h.MyStatsOverview)
	ps.GET("/reviews", h.MyReviews)
	ps.GET("/balance", h.MyBalance)
	ps.GET("/withdrawals", h.MyWithdrawals)
	ps.POST("/withdrawals", h.RequestWithdrawal, writeRL)
	ps.POST("/withdrawals/:id/cancel", h.CancelWithdrawal, writeRL)

	// --- advisor settings (M4): contact details and profile text, the
	// photo submitted for review (private until approved) and the advisor's
	// own custom offering. Photo uploads have their own small budget that
	// fails closed (the service also caps uploads per day in the database).
	photoRL := mw.RateLimit(limiter, rule("advisor_photo", config.Rule{Rate: 6, Period: time.Hour, Burst: 3}, scale), mw.ByUserOrIP, false)
	ps.GET("/settings", h.PsychicSettings)
	ps.PATCH("/settings", h.UpdatePsychicSettings, writeRL)
	ps.GET("/photo", h.PsychicPhoto)
	ps.POST("/photo", h.SubmitPsychicPhoto, photoRL)
	ps.DELETE("/photo", h.WithdrawPsychicPhoto, writeRL)
	ps.GET("/photo/image", h.PsychicPhotoImage)
	ps.GET("/service", h.PsychicCustomService)
	ps.PUT("/service", h.PutPsychicCustomService, writeRL)
	ps.DELETE("/service", h.DeletePsychicCustomService, writeRL)
	// --- end advisor settings ---

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
	// Client notes: oversight and access requests (always audited with a reason).
	ad.GET("/client-notes", h.AdminClientNotes, stepUp)
	ad.DELETE("/client-notes/:id", h.AdminRemoveClientNote, stepUp, adminRL)
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
	// revisions 3.0: calculator analytics
	ad.GET("/analytics/calculators", h.AdminCalculators)
	ad.GET("/analytics/calculators/conversions", h.AdminCalculatorConversions)
	ad.GET("/audit-logs", h.AdminAudit)

	// revisions 3.0: support desk. The queue and the counters are metadata;
	// reading a ticket and answering it need a fresh step-up (they decrypt).
	ad.GET("/support/tickets", h.AdminSupportTickets)
	ad.GET("/support/summary", h.AdminSupportSummary)
	ad.GET("/support/tickets/:id", h.AdminSupportTicket, stepUp)
	ad.POST("/support/tickets/:id/messages", h.AdminSupportReply, stepUp, adminRL)
	ad.PATCH("/support/tickets/:id", h.AdminSupportUpdate, adminRL)
	// end revisions 3.0: support desk

	// --- advisor settings: photo review (step-up; previews and decisions
	// are audited). Unreviewed photos are only ever streamed from here.
	ad.GET("/photo-submissions", h.AdminPhotoSubmissions, stepUp)
	ad.GET("/photo-submissions/:id/image", h.AdminPhotoSubmissionImage, stepUp)
	ad.POST("/photo-submissions/:id/approve", h.AdminApprovePhoto, stepUp, adminRL)
	ad.POST("/photo-submissions/:id/reject", h.AdminRejectPhoto, stepUp, adminRL)
	// --- end advisor settings ---

	// revisions 3.0: site settings. Reads are open to every admin; each write
	// needs a fresh TOTP step-up and is audited by the service.
	ad.GET("/site/banners", h.AdminSiteBanners)
	ad.POST("/site/banners", h.AdminCreateSiteBanner, stepUp, adminRL)
	ad.PUT("/site/banners/order", h.AdminReorderSiteBanners, stepUp, adminRL)
	ad.PATCH("/site/banners/settings", h.AdminUpdateBannerSettings, stepUp, adminRL)
	ad.PATCH("/site/banners/:id", h.AdminUpdateSiteBanner, stepUp, adminRL)
	ad.DELETE("/site/banners/:id", h.AdminDeleteSiteBanner, stepUp, adminRL)
	ad.GET("/site/signup-offer", h.AdminSignupOffer)
	ad.PUT("/site/signup-offer", h.AdminUpdateSignupOffer, stepUp, adminRL)
	ad.GET("/site/free-mode", h.AdminFreeMode)
	ad.PUT("/site/free-mode", h.AdminUpdateFreeMode, stepUp, adminRL)
	// --- end site settings ---

	// billing: plans (immutable versions), routing, memberships, payments.
	ad.GET("/billing/plans", h.AdminBillingPlans)
	ad.POST("/billing/plans", h.AdminCreateBillingPlan, stepUp, adminRL)
	ad.PATCH("/billing/plans/:id", h.AdminUpdateBillingPlan, stepUp, adminRL)
	ad.DELETE("/billing/plans/:id", h.AdminDeleteBillingPlan, stepUp, adminRL)
	ad.POST("/billing/plans/:id/versions", h.AdminCreateBillingPlanVersion, stepUp, adminRL)
	ad.GET("/billing/routing-rules", h.AdminRoutingRules)
	ad.POST("/billing/routing-rules", h.AdminCreateRoutingRule, stepUp, adminRL)
	ad.POST("/billing/routing-rules/simulate", h.AdminSimulateRoute)
	ad.PATCH("/billing/routing-rules/:id", h.AdminUpdateRoutingRule, stepUp, adminRL)
	ad.DELETE("/billing/routing-rules/:id", h.AdminDeleteRoutingRule, stepUp, adminRL)
	ad.GET("/billing/providers", h.AdminPaymentProviders)
	ad.GET("/billing/subscriptions", h.AdminBillingSubscriptions)
	ad.POST("/billing/subscriptions/:id/cancel", h.AdminCancelBillingSubscription, stepUp, adminRL)
	ad.GET("/billing/transactions", h.AdminBillingTransactions)
	ad.POST("/billing/transactions/:id/refund", h.AdminRefundTransaction, stepUp, adminRL)

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
