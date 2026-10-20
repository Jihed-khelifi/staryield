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
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/migrations"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/ratelimit"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/redisx"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
	"github.com/Jihed-khelifi/staryield/backend/internal/worker"
)

// Overrides replace infrastructure for tests (nil = production default).
type Overrides struct {
	Clock   clock.Clock
	Mailer  mail.Mailer
	Storage storage.ObjectStore
	Tracker analytics.Tracker
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
	Clock     clock.Clock

	server *http.Server
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New wires the application. It connects to PostgreSQL and Redis and runs
// migrations when enabled.
func New(ctx context.Context, cfg *config.Config, log *zap.Logger, ov Overrides) (*App, error) {
	if cfg.App.InstanceID == "" {
		host, _ := os.Hostname()
		cfg.App.InstanceID = fmt.Sprintf("%s-%d", host, os.Getpid())
	}
	clk := ov.Clock
	if clk == nil {
		clk = clock.Real{}
	}

	db, err := database.Open(ctx, cfg.Database, log)
	if err != nil {
		return nil, err
	}
	if cfg.Database.AutoMigrate {
		if _, err := migrations.Up(ctx, db, log); err != nil {
			return nil, fmt.Errorf("migrate: %w", err)
		}
	}
	redisx.SetLogger(log)
	rdb, err := redisx.New(ctx, cfg.Redis)
	if err != nil {
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

	// Security primitives.
	hasher := security.NewPasswordHasher(cfg.Auth.Argon2Memory, cfg.Auth.Argon2Iterations, cfg.Auth.Argon2Parallelism)
	issuer, err := security.NewTokenIssuer(cfg.Auth.JWTSecret, cfg.Auth.JWTIssuer, cfg.Auth.JWTAudience, cfg.Auth.AccessTokenTTL, clk.Now)
	if err != nil {
		return nil, err
	}
	csrf, err := security.NewCSRF(cfg.Security.CSRFSecret)
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
	mailer := ov.Mailer
	if mailer == nil {
		switch cfg.Mail.Driver {
		case "log":
			mailer = mail.LogMailer{From: cfg.Mail.From}
		default:
			mailer = mail.NoopMailer{}
		}
	}
	var provider payment.Provider = payment.NewSandbox(cfg.Payments.WebhookSecret, cfg.Payments.WebhookTolerance)

	// Business layer.
	share := cfg.Billing.PsychicSharePercent
	walletSvc := service.NewWalletService(txm, wallets, clk)
	authSvc := service.NewAuthService(txm, users, tokens, walletSvc, wallets, hasher, issuer, kv, tickets, mailer, publisher, track, clk,
		service.AuthSettings{
			RefreshTTL: cfg.Auth.RefreshTokenTTL, ResetTTL: cfg.Auth.PasswordResetTTL, MaxFailures: cfg.Auth.LoginMaxFailures,
			Lockout: cfg.Auth.LoginLockout, MinimumAge: cfg.Auth.MinimumAge, SignupFreeMinutes: cfg.Billing.SignupFreeMinutes,
			SignupBonusCredits: cfg.Billing.SignupBonusCredits, StatusCacheTTL: cfg.Auth.UserStatusCacheTTL,
			TicketTTL: cfg.Realtime.TicketTTL, FrontendURL: cfg.App.FrontendURL, IPHashSecret: cfg.Analytics.IPHashSecret,
		})
	userSvc := service.NewUserService(users, wallets, psychics, store, service.AstroPlaceLookup, track, clk, cfg.Auth.MinimumAge,
		service.AvatarSettings{MaxBytes: cfg.Storage.MaxAvatarBytes, MaxPixels: cfg.Storage.MaxAvatarPixels, Size: cfg.Storage.AvatarSize})
	psychicSvc := service.NewPsychicService(psychics, chatRepo, earnings, presence, publisher, track, clk)
	creditSvc := service.NewCreditService(txm, credits, walletSvc, wallets, provider, publisher, track, clk)
	chatSvc := service.NewChatService(txm, chatRepo, users, psychics, psychicSvc, walletSvc, wallets, earnings, presence, publisher, track, limiter, clk,
		service.ChatSettings{
			RequestTimeout: cfg.Chat.RequestTimeout, AcceptedTimeout: cfg.Chat.AcceptedTimeout, GracePeriod: cfg.Chat.GracePeriod,
			BillingInterval: cfg.Chat.BillingInterval, MaxMessageLength: cfg.Chat.MaxMessageLength, HistoryPageSize: cfg.Chat.HistoryPageSize,
			LowBalanceMinutes: cfg.Chat.LowBalanceMinute, SharePercent: share,
			MessageRule: ratelimit.Rule{Name: "msg", Rate: cfg.Chat.MessageRate, Period: cfg.Chat.MessageRateEvery, Burst: cfg.Chat.MessageRate}.Scaled(cfg.RateLimit.Scale),
			TypingRule:  ratelimit.Rule{Name: "typing", Rate: 1, Period: cfg.Realtime.TypingMinInterval, Burst: 2},
		})
	requestSvc := service.NewCustomRequestService(txm, requests, psychics, walletSvc, wallets, earnings, chatSvc, publisher, track, clk,
		service.RequestSettings{AcceptTimeout: cfg.CustomRequests.AcceptTimeout, DeliveryTimeout: cfg.CustomRequests.DeliveryTimeout,
			DisputeWindow: cfg.CustomRequests.DisputeWindow, MaxNoteLength: cfg.CustomRequests.MaxNoteLength,
			MaxDelivery: cfg.CustomRequests.MaxDelivery, SharePercent: share})
	adminSvc := service.NewAdminService(txm, users, wallets, tokens, psychics, credits, audit, chatRepo, walletSvc, authSvc, chatSvc,
		psychicSvc, publisher, hasher, clk, cfg.Analytics.IPHashSecret, cfg.Billing.MaxAdminAdjustment)
	analyticsSvc := service.NewAnalyticsService(track, analyticsRepo, reports, clk, cfg.Analytics.IPHashSecret, cfg.Analytics.MaxBatch)
	astroSvc := service.NewAstrologyService(users, track, clk)
	gateway := service.NewRealtimeGateway(chatSvc, psychicSvc, presence)

	rtServer := realtime.NewServer(hub, broker, presence, tickets, gateway, authSvc.AuthorizeRealtime, cfg.Realtime,
		cfg.Security.AllowedOrigins, log.With(zap.String("component", "realtime")))

	services := handler.Services{Auth: authSvc, Users: userSvc, Wallet: walletSvc, Psychics: psychicSvc, Credits: creditSvc,
		Chat: chatSvc, Gateway: gateway, Requests: requestSvc, Admin: adminSvc, Analytics: analyticsSvc, Astrology: astroSvc}
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
		rtServer, health, cfg.Payments.Currency, cfg.Storage.MaxAvatarBytes)
	router := api.NewRouter(api.Deps{Config: cfg, Log: log, Handler: h, Auth: authSvc, CSRF: csrf, Limiter: limiter, Cookies: cookieNames})

	return &App{
		Config: cfg, Log: log, DB: db, Redis: rdb, Router: router, Services: services, Hub: hub, Realtime: rtServer,
		Scheduler: worker.New(chatSvc, requestSvc, tokens, clk, cfg.Worker, log), Tracker: tracker, Clock: clk,
	}, nil
}

// Start launches background components (broker fan-out, workers, analytics).
func (a *App) Start(ctx context.Context) {
	ctx, a.cancel = context.WithCancel(ctx)
	a.Tracker.Start()
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.Realtime.RunBroker(ctx); err != nil {
			a.Log.Error("realtime broker stopped", zap.Error(err))
		}
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
	a.Tracker.Stop(ctx)
	_ = a.Redis.Close()
	if sqlDB, err := a.DB.DB(); err == nil {
		_ = sqlDB.Close()
	}
	a.Log.Info("shutdown complete")
}
