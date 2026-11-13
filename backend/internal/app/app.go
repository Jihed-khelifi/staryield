// Package app is the composition root: it constructs every dependency
// explicitly (no globals, no reflection-based injection) and owns the
// process lifecycle. Tests build the same graph with in-memory doubles.
package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/api"
	"github.com/Jihed-khelifi/staryield/backend/internal/api/handler"
	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/migrations"
	"github.com/Jihed-khelifi/staryield/backend/internal/oidc"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/push"
	"github.com/Jihed-khelifi/staryield/backend/internal/ratelimit"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/redisx"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
	"github.com/Jihed-khelifi/staryield/backend/internal/worker"
)

// MailJob is the scheduler job delivering the e-mail outbox.
const MailJob = "mail"

// identity: scheduler jobs of the account self-service module.
const (
	// ExportJob builds queued personal-data exports.
	ExportJob = "exports"
	// ClosureJob erases accounts whose closure grace period ended.
	ClosureJob = "account-closures"
)

// Overrides replace infrastructure for tests (nil = production default).
type Overrides struct {
	Clock          clock.Clock
	Mailer         mail.Mailer
	Storage        storage.ObjectStore // avatars
	AdvisorStorage storage.ObjectStore // advisor images (R2_ADVISOR_IMAGE)
	ChatStorage    storage.ObjectStore // private chat images (R2_CHAT_IMAGES_PRIVATE)
	Tracker        analytics.Tracker
	// PushSender replaces the Web Push transport (tests never reach real
	// push services); push still needs VAPID keys in the configuration.
	PushSender push.Sender
	// billing: a shared simulated processor lets several app instances (and
	// the test) see the same sandbox charges.
	PaymentProcessor *payment.SandboxProcessor
	// ExportStorage holds personal-data export archives (identity).
	ExportStorage storage.ObjectStore
	// Google replaces the OpenID Connect client of Sign in with Google
	// (revisions 4.0); tests never reach Google. The feature is still on
	// only when STARYIELD_GOOGLE_CLIENT_ID/_SECRET/_REDIRECT_URL are set.
	Google oidc.Provider
}

// App is a fully wired server.
type App struct {
	Config    *config.Config
	Log       *zap.Logger
	DB        *gorm.DB
	Redis     *redis.Client
	Router    http.Handler
	Services  handler.Services
	Scheduler *worker.Scheduler
	Hub       *realtime.Hub
	Realtime  *realtime.Server
	Tracker   *analytics.AsyncTracker
	Visitors  *analytics.VisitorAggregator // revisions 3.0: calculator analytics
	Clock     clock.Clock
	// Dispatcher delivers notifications asynchronously.
	Dispatcher *service.Dispatcher
	// Events publishes realtime events; Broker is this instance's fan-out
	// (chat ordering gate, per-user subscriptions).
	Events *realtime.Publisher
	Broker *realtime.RedisBroker
	// ExportStore holds personal-data export archives (identity).
	ExportStore storage.ObjectStore
	// Hasher is the shared, concurrency-bounded password hasher.
	Hasher *security.PasswordHasher

	server *http.Server
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New wires the application. It connects to PostgreSQL, applies pending
// migrations, then connects to Redis.
func New(ctx context.Context, cfg *config.Config, log *zap.Logger, ov Overrides) (*App, error) {
	if cfg.App.InstanceID == "" {
		host, _ := os.Hostname()
		cfg.App.InstanceID = fmt.Sprintf("%s-%d", host, os.Getpid())
	}
	clk := ov.Clock
	if clk == nil {
		clk = clock.Real{}
	}

	// GORM timestamps (created_at, updated_at, deleted_at) follow the app
	// clock, so a fake clock in tests governs them too.
	db, err := database.Open(ctx, cfg.Database, log, database.WithClock(clk.Now))
	if err != nil {
		return nil, err
	}
	// The schema is brought up to date on every start, before anything else
	// touches the database; an advisory lock serialises concurrent replicas.
	if _, err := migrations.Up(ctx, db, log); err != nil {
		closeDB(db)
		return nil, fmt.Errorf("migrate: %w", err)
	}
	redisx.SetLogger(log)
	rdb, err := redisx.New(ctx, cfg.Redis, log)
	if err != nil {
		closeDB(db)
		return nil, err
	}
	prefix := cfg.Redis.KeyPrefix

	// Data layer.
	txm := database.NewTxManager(db)
	users := repository.NewUserRepository(db)
	tokens := repository.NewTokenRepository(db)
	wallets := repository.NewWalletRepository(db)
	psychics := repository.NewPsychicRepository(db)
	credits := repository.NewCreditRepository(db)
	chatRepo := repository.NewChatRepository(db)
	requests := repository.NewCustomRequestRepository(db)
	earnings := repository.NewEarningRepository(db)
	analyticsRepo := repository.NewAnalyticsRepository(db)
	audit := repository.NewAuditRepository(db)
	reports := repository.NewReportRepository(db)
	admins := repository.NewAdminRepository(db)
	growth := repository.NewGrowthRepository(db)
	notificationsRepo := repository.NewNotificationRepository(db)
	reviewsRepo := repository.NewReviewRepository(db)
	withdrawalsRepo := repository.NewWithdrawalRepository(db)
	notesRepo := repository.NewNoteRepository(db)
	attachments := repository.NewAttachmentRepository(db)
	insights := repository.NewInsightsRepository(db)

	// Security primitives.
	hasher := security.NewPasswordHasher(cfg.Auth.Argon2Memory, cfg.Auth.Argon2Iterations, cfg.Auth.Argon2Parallelism).
		WithLimits(cfg.Auth.Argon2Concurrency, cfg.Auth.PasswordHashWait)
	issuer, err := security.NewTokenIssuer(cfg.Auth.JWTSecret, cfg.Auth.JWTIssuer, cfg.Auth.JWTAudience, cfg.Auth.AccessTokenTTL, clk.Now)
	if err != nil {
		return nil, err
	}
	csrf, err := security.NewCSRF(cfg.Security.CSRFSecret)
	if err != nil {
		return nil, err
	}
	key, err := cfg.EncryptionKey()
	if err != nil {
		return nil, err
	}
	sealer, err := security.NewSealer(key)
	if err != nil {
		return nil, err
	}

	// Realtime infrastructure.
	broker := realtime.NewRedisBroker(rdb, prefix, cfg.Realtime.StreamMaxLen, cfg.Realtime.StreamTTL, log.With(zap.String("component", "broker")))
	presence := realtime.NewRedisPresence(rdb, prefix, cfg.Realtime.PresenceTTL)
	tickets := realtime.NewRedisTickets(rdb, prefix)
	hub := realtime.NewHub(log, cfg.Realtime.MaxConnsPerUser)
	publisher := realtime.NewPublisher(broker, clk.Now)

	kv := cache.NewRedis(rdb, prefix)
	limiter := ratelimit.NewRedisLimiter(rdb, prefix)

	tracker := analytics.NewAsyncTracker(analytics.Config{
		Enabled: cfg.Analytics.Enabled, BufferSize: cfg.Analytics.BufferSize, BatchSize: cfg.Analytics.BatchSize,
		FlushInterval: cfg.Analytics.FlushInterval, IPHashSecret: cfg.Analytics.IPHashSecret,
	}, analyticsRepo, log.With(zap.String("component", "analytics")), clk.Now)
	var track analytics.Tracker = tracker
	if ov.Tracker != nil {
		track = ov.Tracker
	}

	store := ov.Storage
	if store == nil {
		if store, err = storage.New(cfg.Storage); err != nil {
			return nil, err
		}
	}
	advisorStore := ov.AdvisorStorage
	if advisorStore == nil {
		if advisorStore, err = storage.NewBucket(cfg.Storage, cfg.Storage.AdvisorBucket, cfg.Storage.AdvisorPublicBaseURL); err != nil {
			return nil, err
		}
	}
	chatStore := ov.ChatStorage
	if chatStore == nil {
		if chatStore, err = storage.NewBucket(cfg.Storage, cfg.Storage.ChatBucket, ""); err != nil {
			return nil, err
		}
	}
	mailer := ov.Mailer
	if mailer == nil {
		switch cfg.Mail.Driver {
		case "cloudflare":
			cf, err := mail.NewCloudflare(mail.CloudflareConfig{AccountID: cfg.Mail.CloudflareAccountID, APIToken: cfg.Mail.CloudflareAPIToken,
				From: cfg.Mail.From, FromName: cfg.Mail.FromName, ReplyTo: cfg.Mail.ReplyTo, BaseURL: cfg.Mail.CloudflareBaseURL,
				Timeout: cfg.Mail.Timeout})
			if err != nil {
				return nil, err
			}
			mailer = cf
		case "log":
			mailer = mail.LogMailer{From: cfg.Mail.From}
		default:
			mailer = mail.NoopMailer{}
			log.Warn("mail driver is noop: e-mail (password resets, account notices) is discarded")
		}
	}
	// Keyed hashes identify e-mail addresses and push endpoints without
	// storing them in clear; the keys derive from the data encryption key.
	addressHashKey := security.Keyed(string(key), "staryield:address-hash:v1", 0)
	endpointHashKey := security.Keyed(string(key), "staryield:push-endpoint:v1", 0)
	mailSvc := service.NewMailService(txm, repository.NewEmailRepository(db), mailer, sealer, clk, log.With(zap.String("component", "mail")),
		service.MailSettings{FrontendURL: cfg.App.FrontendURL, MaxAttempts: cfg.Mail.MaxAttempts, DailyCap: cfg.Mail.DailyCap,
			AllowMarketing: cfg.Mail.AllowMarketing, SendTimeout: cfg.Mail.Timeout + 5*time.Second, HashKey: addressHashKey,
			SupportEmail: cfg.Mail.SupportInbox}) // revisions 5.1: footer of every e-mail (§25.4)
	var pushSender push.Sender
	pushPublicKey := ""
	if cfg.Push.Active() {
		pcfg := push.Config{PublicKey: cfg.Push.VAPIDPublicKey, PrivateKey: cfg.Push.VAPIDPrivateKey, Subject: cfg.Push.Subject,
			TTL: cfg.Push.TTL, Timeout: cfg.Push.Timeout}
		if err := pcfg.Validate(); err != nil {
			return nil, fmt.Errorf("%s/%s: %w", config.EnvVar("push.vapid_public_key"), config.EnvVar("push.vapid_private_key"), err)
		}
		if pushSender = ov.PushSender; pushSender == nil {
			wp, err := push.NewWebPush(pcfg, nil)
			if err != nil {
				return nil, err
			}
			pushSender = wp
		}
		pushPublicKey = cfg.Push.VAPIDPublicKey
	} else {
		log.Info("web push disabled: set the VAPID keys to enable it (go run ./cmd/vapid)")
	}
	pushSvc := service.NewPushService(repository.NewPushRepository(db), sealer, pushSender, clk, log.With(zap.String("component", "push")),
		service.PushSettings{PublicKey: pushPublicKey, FrontendURL: cfg.App.FrontendURL, HashKey: endpointHashKey,
			MaxPerUser: cfg.Push.MaxPerUser, Workers: cfg.Push.Workers, QueueSize: cfg.Push.QueueSize,
			ChatRequestTTL: cfg.Chat.RequestTimeout, TTL: cfg.Push.TTL})
	channels := service.NewNotificationChannels(repository.NewContactRepository(db), mailSvc, pushSvc, cfg.App.FrontendURL,
		log.With(zap.String("component", "notification-channels")))
	var provider payment.Provider = payment.NewSandbox(cfg.Payments.WebhookSecret, cfg.Payments.WebhookTolerance)
	if cfg.Payments.Provider == "disabled" {
		provider = payment.Disabled{}
		log.Warn("payments disabled: credit purchases are refused until a provider is configured")
	}
	if _, off := store.(storage.Disabled); off {
		log.Warn("object storage disabled: avatar uploads are refused until STARYIELD_STORAGE_DRIVER=r2")
	}
	if _, off := advisorStore.(storage.Disabled); off {
		log.Warn("advisor image storage disabled: set R2_ADVISOR_IMAGE to enable uploads")
	}
	if _, off := chatStore.(storage.Disabled); off {
		log.Warn("chat image storage disabled: set R2_CHAT_IMAGES_PRIVATE to enable image messages")
	}
	dispatcher := service.NewDispatcher(notificationsRepo, publisher, log.With(zap.String("component", "notifications")), clk,
		service.DispatcherConfig{BufferSize: cfg.Notifications.BufferSize, BatchSize: cfg.Notifications.BatchSize,
			FlushInterval: cfg.Notifications.FlushInterval}).WithChannels(channels)

	// Business layer.
	share := cfg.Billing.PsychicSharePercent
	walletSvc := service.NewWalletService(txm, wallets, repository.NewLotRepository(db), clk) // revisions 5.0: credit lots
	authSvc := service.NewAuthService(txm, users, tokens, walletSvc, wallets, hasher, issuer, kv, tickets, mailSvc, publisher, track, clk,
		service.AuthSettings{
			RefreshTTL: cfg.Auth.RefreshTokenTTL, ResetTTL: cfg.Auth.PasswordResetTTL, MaxFailures: cfg.Auth.LoginMaxFailures,
			Lockout: cfg.Auth.LoginLockout, MinimumAge: cfg.Auth.MinimumAge, SignupFreeMinutes: cfg.Billing.SignupFreeMinutes,
			SignupBonusCredits: cfg.Billing.SignupBonusCredits, StatusCacheTTL: cfg.Auth.UserStatusCacheTTL,
			TicketTTL: cfg.Realtime.TicketTTL, FrontendURL: cfg.App.FrontendURL, IPHashSecret: cfg.Analytics.IPHashSecret,
		})
	authSvc.SetPush(pushSvc) // revisions 4.0: subscriptions end with the session
	userSvc := service.NewUserService(users, wallets, psychics, store, service.AstroPlaceLookup, track, clk, cfg.Auth.MinimumAge,
		service.AvatarSettings{MaxBytes: cfg.Storage.MaxAvatarBytes, MaxPixels: cfg.Storage.MaxAvatarPixels, Size: cfg.Storage.AvatarSize})
	psychicSvc := service.NewPsychicService(psychics, chatRepo, earnings, presence, publisher, track, clk)
	creditSvc := service.NewCreditService(txm, credits, walletSvc, wallets, provider, publisher, track, clk).WithNotifier(dispatcher)
	// revisions 5.1: consent evidence (IP hash with the analytics secret, as refund requests), statement
	// descriptor on every charge and the receipt copy to receipt_email (§25.2, §25.3, §25.5).
	creditSvc.WithSettings(service.CreditSettings{IPHashSecret: cfg.Analytics.IPHashSecret, StatementDescriptor: cfg.Payments.StatementDescriptor,
		FrontendURL: cfg.App.FrontendURL}).WithReceiptMail(repository.NewContactRepository(db), mailSvc)
	chatSvc := service.NewChatService(txm, chatRepo, users, psychics, psychicSvc, walletSvc, wallets, earnings, presence, publisher, track, limiter, clk,
		service.ChatSettings{
			RequestTimeout: cfg.Chat.RequestTimeout, AcceptedTimeout: cfg.Chat.AcceptedTimeout, GracePeriod: cfg.Chat.GracePeriod,
			BillingInterval: cfg.Chat.BillingInterval, MaxMessageLength: cfg.Chat.MaxMessageLength, HistoryPageSize: cfg.Chat.HistoryPageSize,
			LowBalanceMinutes: cfg.Chat.LowBalanceMinute, SharePercent: share,
			PayoutCentsPerCredit: cfg.Billing.PayoutCentsPerCredit, Currency: cfg.Payments.Currency,
			MessageRule: ratelimit.Rule{Name: "msg", Rate: cfg.Chat.MessageRate, Period: cfg.Chat.MessageRateEvery, Burst: cfg.Chat.MessageRate}.Scaled(cfg.RateLimit.Scale),
			TypingRule:  ratelimit.Rule{Name: "typing", Rate: 1, Period: cfg.Realtime.TypingMinInterval, Burst: 2},
		})
	requestSvc := service.NewCustomRequestService(txm, requests, psychics, walletSvc, wallets, earnings, chatSvc, publisher, track, clk,
		service.RequestSettings{AcceptTimeout: cfg.CustomRequests.AcceptTimeout, DeliveryTimeout: cfg.CustomRequests.DeliveryTimeout,
			DisputeWindow: cfg.CustomRequests.DisputeWindow, MaxNoteLength: cfg.CustomRequests.MaxNoteLength,
			MaxDelivery: cfg.CustomRequests.MaxDelivery, SharePercent: share}).WithNotifier(dispatcher)
	adminAuthSvc := service.NewAdminAuthService(txm, authSvc, users, admins, audit, kv, sealer, track, clk, service.AdminAuthSettings{
		SessionTTL: cfg.Admin.SessionTTL, IdleTimeout: cfg.Admin.IdleTimeout, StepUpWindow: cfg.Admin.StepUpWindow,
		ChallengeTTL: cfg.Admin.ChallengeTTL, Issuer: cfg.Admin.TOTPIssuer, IPHashSecret: cfg.Analytics.IPHashSecret})
	reviewSvc := service.NewReviewService(txm, reviewsRepo, chatRepo, users, psychics, audit, dispatcher, track, clk,
		cfg.Reviews.Window, cfg.Analytics.IPHashSecret)
	chatSvc.WithExtras(service.ChatExtras{Attachments: attachments, Store: chatStore, Insights: insights, Reviews: reviewSvc,
		Notifier: dispatcher, Images: service.ImagePolicy{MaxBytes: cfg.Storage.MaxChatImageBytes, MaxPixels: cfg.Storage.MaxImagePixels,
			MaxSide: cfg.Storage.ChatImageMaxSide, AllowGIF: true, KeepTransparency: true, Field: "image"}})
	chatSvc.WithUnread(kv, repository.NewContactRepository(db), pushSvc, cfg.App.FrontendURL) // revisions 3.0: chat unread
	// revisions 4.0: private client nicknames
	chatSvc.WithAliases(repository.NewAliasRepository(db), sealer, audit, cfg.Analytics.IPHashSecret)
	growthSvc := service.NewGrowthService(growth, audit, track, clk, service.GrowthSettings{FrontendURL: cfg.App.FrontendURL,
		PublicBaseURL: cfg.Links.PublicBaseURL, AllowedHosts: cfg.Links.AllowedHosts, Currency: cfg.Payments.Currency,
		IPHashSecret: cfg.Analytics.IPHashSecret})
	notificationSvc := service.NewNotificationService(notificationsRepo, growth, presence, publisher, audit, clk,
		cfg.Analytics.IPHashSecret, log.With(zap.String("component", "broadcasts"))).WithChannels(channels)
	withdrawalSvc := service.NewWithdrawalService(txm, withdrawalsRepo, psychics, audit, sealer, dispatcher, track, clk, service.PayoutSettings{
		CentsPerCredit: cfg.Billing.PayoutCentsPerCredit, MinCredits: cfg.Billing.MinWithdrawalCredits, Currency: cfg.Payments.Currency,
		IPHashSecret: cfg.Analytics.IPHashSecret})
	psychicAnalyticsSvc := service.NewPsychicAnalyticsService(insights, earnings, reviewsRepo, psychics, withdrawalSvc, clk)
	adminSvc := service.NewAdminService(service.AdminDeps{Tx: txm, Users: users, Wallets: wallets, Tokens: tokens, Psychics: psychics,
		Credits: credits, Audit: audit, Chat: chatRepo, Growth: growth, Insights: insights, Notes: notesRepo, Waitlist: repository.NewWaitlistRepository(db),
		Wallet: walletSvc, Auth: authSvc,
		AdminAuth: adminAuthSvc, ChatSvc: chatSvc, PsychicSvc: psychicSvc, Events: publisher, Notifier: dispatcher, Presence: presence,
		Hasher: hasher, AvatarStore: store, AdvisorStore: advisorStore, Clock: clk, IPSecret: cfg.Analytics.IPHashSecret,
		MaxAdjust: cfg.Billing.MaxAdminAdjustment, Currency: cfg.Payments.Currency,
		AdvisorImage: service.ImagePolicy{MaxBytes: cfg.Storage.MaxAdvisorImageBytes, MaxPixels: cfg.Storage.MaxImagePixels,
			MaxSide: cfg.Storage.AdvisorImageMaxSide, Field: "image"}})
	insightsSvc := service.NewInsightsService(insights, users, adminSvc, clk, cfg.Payments.Currency)
	analyticsSvc := service.NewAnalyticsService(track, analyticsRepo, reports, clk, cfg.Analytics.IPHashSecret, cfg.Analytics.MaxBatch)
	// revisions 3.0: calculator analytics
	calculatorRepo := repository.NewCalculatorRepository(db)
	visitors := analytics.NewVisitorAggregator(analytics.VisitorConfig{Enabled: cfg.Analytics.Enabled, BufferSize: cfg.Analytics.BufferSize,
		BatchSize: cfg.Analytics.BatchSize, FlushInterval: cfg.Analytics.FlushInterval}, calculatorRepo, log.With(zap.String("component", "visitors")))
	analyticsSvc.WithVisitors(visitors)
	calculatorSvc := service.NewCalculatorService(calculatorRepo, kv, clk, cfg.Analytics.Enabled)
	// end revisions 3.0: calculator analytics
	astroSvc := service.NewAstrologyService(users, track, clk, kv)
	notesSvc := service.NewNoteService(txm, notesRepo, chatRepo, audit, sealer, clk, cfg.Analytics.IPHashSecret)
	statsSvc := service.NewPsychicStatsService(repository.NewStatsRepository(db), kv, clk)
	// Public directory snapshot and presence history, both driven by status changes.
	catalogSvc := service.NewAdvisorCatalog(repository.NewCatalogRepository(db), psychicSvc.StatusesFor, clk)
	presenceTracker := service.NewPresenceTracker(txm, repository.NewPresenceRepository(db), psychicSvc.StatusesFor, clk, nil)
	waitlistRepo := repository.NewWaitlistRepository(db)
	waitlistSvc := service.NewWaitlistService(txm, waitlistRepo, psychics, psychicSvc.StatusesFor,
		func(ctx context.Context, id uuid.UUID) (domain.PresenceStatus, error) {
			st, _, err := psychicSvc.Status(ctx, id)
			return st, err
		}, dispatcher, track, clk)
	psychicSvc.OnDirectoryChange(catalogSvc.MarkStale)
	psychicSvc.OnStatusChange(func(ctx context.Context, id uuid.UUID, st domain.PresenceStatus) {
		catalogSvc.SetStatus(id, st)
		presenceTracker.Observe(ctx, id, st)
		if st == domain.PresenceOnline {
			waitlistSvc.AdvisorOnline(ctx, id)
		}
	})
	supportRepo := repository.NewSupportRepository(db)
	supportSvc := service.NewSupportService(supportRepo, sealer, service.NewMailSupportNotifier(mailSvc, cfg.Mail.SupportInbox), clk,
		cfg.Analytics.IPHashSecret)
	// revisions 3.0: support desk
	supportSvc.WithDesk(service.SupportDeskDeps{Tx: txm, Users: users, Audit: audit, Cache: kv, Notifier: dispatcher, Chat: chatRepo, Psychics: psychics, // revisions 3.1
		Settings: service.SupportSettings{FrontendURL: cfg.App.FrontendURL, ConsoleURL: cfg.Admin.ConsoleURL,
			Inbox: cfg.Mail.SupportInbox, LinkTTL: cfg.Support.LinkTTL, Retention: cfg.Support.Retention}})
	// end revisions 3.0: support desk
	gateway := service.NewRealtimeGateway(chatSvc, psychicSvc, presence)
	gateway.SetAgeStatus(authSvc.AgeStatus) // revisions 5.0: 18+ age gate (§24.6)

	// billing: saved-card providers (routing rules choose among them) and
	// the membership service. Payments disabled ⇒ no provider is registered.
	registry := payment.NewRegistry("")
	if cfg.Payments.Provider != "disabled" {
		proc := ov.PaymentProcessor
		if proc == nil {
			proc = payment.NewSandboxProcessor()
		}
		var providers []payment.Named
		for _, name := range cfg.Payments.Providers {
			switch name {
			case "sandbox_a":
				providers = append(providers, payment.NewSandboxA(proc, cfg.Payments.WebhookSecret, cfg.Payments.WebhookTolerance))
			case "sandbox_b":
				providers = append(providers, payment.NewSandboxB(proc))
			}
		}
		registry = payment.NewRegistry(cfg.Payments.DefaultProvider, providers...)
	}
	billingSvc := service.NewBillingService(service.BillingDeps{Tx: txm, Repo: repository.NewBillingRepository(db), Users: users,
		Wallets: wallets, Wallet: walletSvc, Credits: credits, Registry: registry, Legacy: provider, Sealer: sealer, Audit: audit,
		Events: publisher, Notify: dispatcher, Tracker: track, Clock: clk},
		service.BillingSettings{Currency: cfg.Payments.Currency, RenewalLease: cfg.Payments.RenewalLease, GracePeriod: cfg.Payments.GracePeriod,
			IncompleteTTL: cfg.Payments.IncompleteTTL, MaxPaymentMethods: cfg.Payments.MaxPaymentMethods, IPHashSecret: cfg.Analytics.IPHashSecret,
			MembershipsEnabled: cfg.Billing.MembershipsEnabled, StatementDescriptor: cfg.Payments.StatementDescriptor}) // revisions 5.1
	// revisions 5.0: refund requests (§24.4) and the revenue report (§24.5).
	refundRepo := repository.NewRefundRepository(db)
	refundSvc := service.NewRefundService(service.RefundDeps{Tx: txm, Repo: refundRepo, Billing: billingSvc,
		Users: users, Audit: audit, Events: publisher, Notify: dispatcher, Clock: clk},
		service.RefundSettings{WindowDays: cfg.Refunds.WindowDays, RequestsEnabled: cfg.Refunds.RequestsEnabled,
			MaxCredit: cfg.Billing.MaxAdminAdjustment, IPHashSecret: cfg.Analytics.IPHashSecret,
			StatementDescriptor: cfg.Payments.StatementDescriptor}) // revisions 5.1
	// end revisions 5.0: refund requests
	creditSvc.WithBilling(billingSvc)
	billingSvc.WithSettler(creditSvc)
	waitlistSvc.SetPriority(billingSvc.HasWaitlistPriority) // members are told first
	chatSvc.SetDiscounts(billingSvc.DiscountFor)            // member chat discount, snapshotted per session
	// --- advisor settings: contact details, photo review (private bucket
	// until approved), custom offering ---
	advisorSvc := service.NewAdvisorService(service.AdvisorDeps{Tx: txm, Psychics: psychics,
		Photos: repository.NewPhotoSubmissionRepository(db), PsychicSvc: psychicSvc, Audit: audit, Sealer: sealer,
		Private: chatStore, Public: advisorStore, Notifier: dispatcher, Tracker: track, Clock: clk,
		Settings: service.AdvisorSettings{IPHashSecret: cfg.Analytics.IPHashSecret,
			Image: service.ImagePolicy{MaxBytes: cfg.Storage.MaxAdvisorImageBytes, MaxPixels: cfg.Storage.MaxImagePixels,
				MaxSide: cfg.Storage.AdvisorImageMaxSide, Field: "photo"}}})
	// --- end advisor settings ---
	// revisions 3.0: site settings. Promo banners, sign-up offer and free
	// mode: cached in-process and in Redis, invalidated on every instance
	// through Pub/Sub. Registration and chat requests read it through hooks.
	siteSvc := service.NewSiteService(service.SiteDeps{Tx: txm, Repo: repository.NewSiteRepository(db), Cache: kv,
		Bus: cache.NewRedisBus(rdb, prefix), Audit: audit, Clock: clk, Log: log.With(zap.String("component", "site")),
		Cfg: service.SiteSettings{AllowedHosts: cfg.Links.AllowedHosts, SignupFallbackMinutes: cfg.Billing.SignupFreeMinutes,
			IPHashSecret: cfg.Analytics.IPHashSecret, GoogleEnabled: cfg.Google.Enabled(),
			MembershipsEnabled: cfg.Billing.MembershipsEnabled}}) // revisions 5.1
	authSvc.SetSignupOffer(siteSvc.SignupFreeMinutes)
	// revisions 5.0: pricing catalog (§24.1) and the welcome offer (§24.2). The
	// catalog snapshot is cached like the site configuration and invalidated on
	// every pack or pricing-settings write; the welcome offer's card keys use a
	// key derived (own label) from the data encryption key.
	pricingRepo := repository.NewPricingRepository(db)
	pricingSvc := service.NewPricingService(service.PricingDeps{Tx: txm, Repo: pricingRepo, Site: repository.NewSiteRepository(db),
		Signup: siteSvc.SignupOfferState, Cache: kv, Bus: cache.NewRedisBus(rdb, prefix), Audit: audit, Clock: clk,
		Log: log.With(zap.String("component", "pricing")),
		Cfg: service.PricingSettingsConfig{Currency: cfg.Payments.Currency, IPHashSecret: cfg.Analytics.IPHashSecret,
			StatementDescriptor: cfg.Payments.StatementDescriptor}}) // revisions 5.1
	adminSvc.SetRateRange(pricingSvc.RateRange)
	creditSvc.WithOffers(service.NewOfferService(pricingRepo, clk, key, cfg.Pricing.OfferRequireCardCheck))
	// end revisions 5.0: pricing
	chatSvc.SetFreeMode(siteSvc.FreeModeActive)
	chatSvc.SetOffSessionSettings(siteSvc.OffSessionSettings) // revisions 3.3: messages outside a live chat
	// --- end site settings ---
	// --- identity: devices, settings, consents, exports, closure ---
	sessionsRepo := repository.NewSessionRepository(db)
	actionsRepo := repository.NewActionTokenRepository(db)
	exportsRepo := repository.NewExportRepository(db)
	consentsRepo := repository.NewConsentRepository(db)
	authSvc.WithIdentity(sessionsRepo, actionsRepo)
	consentSvc := service.NewConsentService(txm, consentsRepo, audit, clk, cfg.Analytics.IPHashSecret)
	notesSvc.SetSharingPolicy(consentSvc.NoteSharing)
	exportStore := ov.ExportStorage
	if exportStore == nil {
		bucket := cfg.Storage.ExportBucket
		if bucket == "" {
			bucket = cfg.Storage.ChatBucket
		}
		if exportStore, err = storage.NewBucket(cfg.Storage, bucket, ""); err != nil {
			return nil, err
		}
	}
	if _, off := exportStore.(storage.Disabled); off {
		log.Warn("data export storage disabled: set R2_EXPORTS_PRIVATE (or R2_CHAT_IMAGES_PRIVATE) to enable exports")
	}
	// revisions 4.0: Sign in with Google. The provider is built only when the
	// feature is configured; tests inject theirs through Overrides.
	identities := repository.NewIdentityRepository(db)
	googleProvider := ov.Google
	if googleProvider == nil && cfg.Google.Enabled() {
		googleProvider, err = oidc.NewClient(oidc.Config{Issuer: cfg.Google.Issuer, ClientID: cfg.Google.ClientID,
			ClientSecret: cfg.Google.ClientSecret, RedirectURL: cfg.Google.RedirectURL,
			AllowLoopbackHTTP: cfg.Google.Issuer != config.GoogleDefaultIssuer && !cfg.IsProduction(), Now: clk.Now})
		if err != nil {
			return nil, fmt.Errorf("google sign-in: %w", err)
		}
	}
	googleSvc := service.NewGoogleAuth(service.GoogleDeps{Auth: authSvc, Provider: googleProvider, Identities: identities, Users: users,
		Audit: audit, KV: kv, Notifier: dispatcher, Mail: mailSvc, Clock: clk,
		Settings: service.GoogleSettings{Enabled: cfg.Google.Enabled(), StateTTL: cfg.Google.StateTTL, FrontendURL: cfg.App.FrontendURL,
			IPHashSecret: cfg.Analytics.IPHashSecret}})
	adminSvc.SetIdentities(identities)
	// end revisions 4.0: Sign in with Google
	eraser := service.NewAccountEraser(service.EraserDeps{Tx: txm, Users: users, Tokens: tokens, Psychics: psychics, Hasher: hasher,
		Clock: clk, Notes: notesRepo, Push: repository.NewPushRepository(db), Actions: actionsRepo, Exports: exportsRepo,
		Sessions: sessionsRepo, Consents: consentsRepo, Waitlist: waitlistRepo, Support: supportRepo, Advisor: advisorSvc, ChatData: chatSvc, AvatarStore: store, ExportStore: exportStore,
		Auth: authSvc, AdminAuth: adminAuthSvc, Chat: chatSvc, PsychicSvc: psychicSvc, Events: publisher, Mail: mailSvc,
		Identities: identities, // revisions 4.0
		Refunds:    refundRepo, // revisions 5.0: refund request free text (§24.4)
	})
	eraser.SetBilling(billingSvc)        // memberships end and saved cards are forgotten
	eraser.SetCalculators(calculatorSvc) // revisions 3.0: calculator analytics
	adminSvc.SetEraser(eraser)
	accountSvc := service.NewAccountService(service.AccountDeps{Tx: txm, Users: users, Accounts: repository.NewAccountRepository(db),
		Tokens: tokens, Actions: actionsRepo, Psychics: psychics, Requests: requests, Audit: audit, Auth: authSvc, UserSvc: userSvc,
		Consents: consentSvc, Payouts: withdrawalSvc, Chat: chatSvc, PsychicSvc: psychicSvc, Eraser: eraser, Mail: mailSvc,
		Notifier: dispatcher, Events: publisher, Hasher: hasher, Sealer: sealer, Places: service.AstroPlaceLookup, Tracker: track,
		Clock: clk, Settings: service.AccountSettings{FrontendURL: cfg.App.FrontendURL, IPHashSecret: cfg.Analytics.IPHashSecret,
			MinimumAge: cfg.Auth.MinimumAge, ResetTTL: cfg.Auth.PasswordResetTTL}})
	exportSvc := service.NewExportService(service.ExportDeps{Tx: txm, Jobs: exportsRepo, Data: repository.NewExportDataRepository(db),
		Users: users, Actions: actionsRepo, Audit: audit, Store: exportStore, Avatars: store, ChatImages: chatStore, Sealer: sealer,
		Hasher: hasher, Notifier: dispatcher, Clock: clk, Settings: service.ExportSettings{IPHashSecret: cfg.Analytics.IPHashSecret}})
	// --- end identity ---

	rtServer := realtime.NewServer(hub, broker, presence, tickets, gateway, authSvc.AuthorizeRealtime, cfg.Realtime,
		cfg.Security.AllowedOrigins, log.With(zap.String("component", "realtime")))

	services := handler.Services{Auth: authSvc, Users: userSvc, Wallet: walletSvc, Psychics: psychicSvc, Credits: creditSvc,
		Chat: chatSvc, Gateway: gateway, Requests: requestSvc, Admin: adminSvc, Analytics: analyticsSvc, Astrology: astroSvc,
		AdminAuth: adminAuthSvc, Growth: growthSvc, Insights: insightsSvc, Notifications: notificationSvc, Reviews: reviewSvc,
		Withdrawals: withdrawalSvc, PsychicAnalytics: psychicAnalyticsSvc, Mail: mailSvc, Push: pushSvc, Notes: notesSvc,
		PsychicStats: statsSvc, Billing: billingSvc, Catalog: catalogSvc, Waitlist: waitlistSvc,
		Support: supportSvc, Calculators: calculatorSvc, // revisions 3.0: calculator analytics
		Account: accountSvc, Consents: consentSvc, Exports: exportSvc,
		Advisor: advisorSvc, // advisor settings
		Site:    siteSvc}    // revisions 3.0: site settings
	services.GoogleAuth = googleSvc // revisions 4.0: Sign in with Google
	services.Pricing = pricingSvc   // revisions 5.0: pricing catalog
	// revisions 5.0: refund requests and the revenue report
	services.Refunds = refundSvc
	services.Revenue = service.NewRevenueService(repository.NewRevenueRepository(db), pricingSvc, clk,
		service.RevenueSettings{Currency: cfg.Payments.Currency, PayoutCentsPerCredit: cfg.Billing.PayoutCentsPerCredit})
	cookieNames := api.CookieNamesFor(cfg.Security)
	health := func(r *http.Request) map[string]error {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		sqlDB, err := db.DB()
		if err == nil {
			err = sqlDB.PingContext(ctx)
		}
		return map[string]error{"database": err, "redis": rdb.Ping(ctx).Err()}
	}
	h := handler.New(services, csrf, handler.CookieSettings{Names: cookieNames, Domain: cfg.Security.CookieDomain,
		Secure: cfg.Security.CookieSecure, AccessTTL: cfg.Auth.AccessTokenTTL, RefreshTTL: cfg.Auth.RefreshTokenTTL},
		rtServer, health, cfg.Payments.Currency, handler.Limits{AvatarBytes: cfg.Storage.MaxAvatarBytes,
			AdvisorImageBytes: cfg.Storage.MaxAdvisorImageBytes, ChatImageBytes: cfg.Storage.MaxChatImageBytes}, cfg.Admin.SessionTTL)
	h.SetCountryResolver(api.CountryResolver(cfg.HTTP)) // billing
	var devObjects map[string]storage.ObjectStore
	if cfg.Storage.Driver == "memory" && !cfg.IsProduction() {
		devObjects = map[string]storage.ObjectStore{"avatars": store, "advisors": advisorStore}
	}
	router := api.NewRouter(api.Deps{Config: cfg, Log: log, Handler: h, Auth: authSvc, AdminAuth: adminAuthSvc, CSRF: csrf,
		Limiter: limiter, Cookies: cookieNames, DevObjects: devObjects})
	scheduler := worker.New(chatSvc, requestSvc, tokens, clk, cfg.Worker, log).WithCleanups(
		worker.Cleanup{Name: "admin sessions", Run: func(ctx context.Context, now time.Time) (int64, error) {
			return admins.DeleteExpiredSessions(ctx, now.Add(-24*time.Hour))
		}},
		worker.Cleanup{Name: "old notifications", Run: func(ctx context.Context, now time.Time) (int64, error) {
			return notificationsRepo.DeleteOlderThan(ctx, now.AddDate(0, 0, -180))
		}},
		worker.Cleanup{Name: "finished e-mail", Run: func(ctx context.Context, now time.Time) (int64, error) {
			return mailSvc.PurgeFinished(ctx, now, cfg.Mail.Retention)
		}},
		worker.Cleanup{Name: "expired waitlist entries", Run: waitlistSvc.ExpireStale},
		worker.Cleanup{Name: "finished waitlist entries", Run: waitlistSvc.PurgeFinished},
		worker.Cleanup{Name: "advisor photo submissions", Run: advisorSvc.CleanupPhotos}, // advisor settings
	).WithJobs(worker.Job{Name: MailJob, Every: cfg.Mail.PollInterval, Wake: mailSvc.Wake(), Run: func(ctx context.Context) error {
		_, err := mailSvc.Drain(ctx)
		return err
	}}, worker.Job{Name: service.PresenceJob, Every: 30 * time.Second, Run: presenceTracker.Reconcile},
		worker.Job{Name: service.WaitlistJob, Every: 15 * time.Second, Wake: waitlistSvc.Wake(), Run: waitlistSvc.RunDue}).WithTickJobs(worker.TickJob{Name: "billing", Run: func(ctx context.Context) error { // billing renewals & dunning
		_, err := billingSvc.RunDue(ctx, cfg.Worker.BatchSize)
		return err
	}})

	// --- identity: export builds, closure erasure, housekeeping ---
	scheduler.WithJobs(
		worker.Job{Name: ExportJob, Every: time.Minute, Wake: exportSvc.Wake(), Run: exportSvc.RunDue},
		worker.Job{Name: ClosureJob, Every: 10 * time.Minute, Run: func(ctx context.Context) error {
			_, err := accountSvc.ProcessClosures(ctx, 50)
			return err
		}},
	).WithCleanups(
		worker.Cleanup{Name: "expired exports", Run: exportSvc.PurgeExpired},
		worker.Cleanup{Name: "account tokens", Run: func(ctx context.Context, now time.Time) (int64, error) {
			return actionsRepo.DeleteExpired(ctx, now.Add(-24*time.Hour))
		}},
	)
	// --- end identity ---

	// revisions 3.0: calculator analytics: daily retention of the visitor roll-up.
	scheduler.WithJobs(worker.Job{Name: service.WebVisitorsJob, Every: 24 * time.Hour, Run: func(ctx context.Context) error {
		_, err := calculatorSvc.RunRetention(ctx, clk.Now())
		return err
	}})
	// revisions 4.0: notification hardening. Interrupted broadcasts resume; the
	// pipeline counters are logged once a minute when something notable happened.
	observer := service.NewNotificationObserver(dispatcher, pushSvc, notificationSvc, log.With(zap.String("component", "notification-stats")))
	scheduler.WithJobs(worker.Job{Name: service.BroadcastRecoveryJob, Every: time.Minute, Run: notificationSvc.RecoverBroadcasts},
		worker.Job{Name: "notification-stats", Every: time.Minute, Run: observer.Tick})
	// revisions 5.0: credit lots. Daily check that every wallet balance equals the
	// credits left in its lots; mismatches are logged at error level and never repaired.
	scheduler.WithJobs(worker.Job{Name: service.LotReconcileJob, Every: 24 * time.Hour, Run: func(ctx context.Context) error {
		_, err := walletSvc.ReconcileLots(ctx)
		return err
	}})
	// end revisions 5.0: credit lots
	// revisions 3.0: support desk
	scheduler.WithJobs(worker.Job{Name: service.SupportJob, Every: time.Hour, Run: supportSvc.RunMaintenance})
	// end revisions 3.0: support desk

	return &App{
		Config: cfg, Log: log, DB: db, Redis: rdb, Router: router, Services: services, Hub: hub, Realtime: rtServer,
		Scheduler: scheduler, Tracker: tracker, Visitors: visitors, Clock: clk, Dispatcher: dispatcher,
		Events: publisher, Broker: broker,
		ExportStore: exportStore, // identity
		Hasher:      hasher,
	}, nil
}

// Start launches background components (broker fan-out, workers, analytics).
func (a *App) Start(ctx context.Context) {
	ctx, a.cancel = context.WithCancel(ctx)
	a.Tracker.Start()
	a.Visitors.Start() // revisions 3.0: calculator analytics
	a.Dispatcher.Start()
	a.Services.Push.Start()
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.Realtime.RunBroker(ctx); err != nil {
			a.Log.Error("realtime broker stopped", zap.Error(err))
		}
	}()
	// revisions 3.0: site settings. Drop the in-process copy when another
	// instance changes the configuration.
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.Services.Site.Run(ctx)
	}()
	// --- end site settings ---
	// revisions 5.0: drop the pricing snapshot when another instance changes the catalog.
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.Services.Pricing.Run(ctx)
	}()
	if a.Config.Worker.Enabled {
		a.Scheduler.Start(ctx)
	}
}

// ListenAndServe serves HTTP until the server is shut down.
func (a *App) ListenAndServe() error {
	a.server = &http.Server{
		Addr:              a.Config.HTTP.Addr,
		Handler:           a.Router,
		ReadHeaderTimeout: a.Config.HTTP.ReadHeaderTimeout,
		ReadTimeout:       a.Config.HTTP.ReadTimeout,
		WriteTimeout:      a.Config.HTTP.WriteTimeout,
		IdleTimeout:       a.Config.HTTP.IdleTimeout,
		MaxHeaderBytes:    32 << 10,
		ErrorLog:          zap.NewStdLog(a.Log.With(zap.String("component", "http"))),
	}
	a.Log.Info("http server listening", zap.String("addr", a.Config.HTTP.Addr))
	if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown drains connections and stops background work within ctx.
func (a *App) Shutdown(ctx context.Context) {
	a.Log.Info("shutting down")
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if a.server != nil {
			if err := a.server.Shutdown(ctx); err != nil {
				a.Log.Warn("http shutdown incomplete", zap.Error(err))
			}
		}
	}()
	go func() {
		defer wg.Done()
		a.Hub.Shutdown(ctx) // WebSockets are hijacked: close them explicitly
	}()
	wg.Wait()
	if a.cancel != nil {
		a.cancel()
	}
	a.Scheduler.Wait()
	a.wg.Wait()
	a.Services.Notifications.Shutdown(ctx) // bounded by the shutdown deadline; unfinished broadcasts resume after restart
	a.Dispatcher.Stop(ctx)
	// After the dispatcher: draining it may still queue pushes.
	a.Services.Push.Stop(ctx)
	a.Tracker.Stop(ctx)
	a.Visitors.Stop(ctx) // revisions 3.0: calculator analytics
	_ = a.Redis.Close()
	closeDB(a.DB)
	a.Log.Info("shutdown complete")
}

func closeDB(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}
