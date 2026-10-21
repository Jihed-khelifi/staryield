// Package config loads and validates runtime configuration from the
// environment.
//
// Every setting is an environment variable named STARYIELD_<SECTION>_<KEY>
// (e.g. STARYIELD_AUTH_JWT_SECRET; see .env.example for the full list).
// Values come from, lowest to highest precedence: built-in defaults, platform
// defaults (Railway), a local .env file and the process environment. The
// platform conventions DATABASE_URL, REDIS_URL, APP_ENV and PORT are honoured
// when the STARYIELD_* equivalent is unset.
package config

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// Environment names.
const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvProduction  = "production"
)

// Config is the fully-resolved application configuration.
type Config struct {
	App            AppConfig            `env:"app"`
	HTTP           HTTPConfig           `env:"http"`
	Database       DatabaseConfig       `env:"database"`
	Redis          RedisConfig          `env:"redis"`
	Auth           AuthConfig           `env:"auth"`
	Security       SecurityConfig       `env:"security"`
	Chat           ChatConfig           `env:"chat"`
	Billing        BillingConfig        `env:"billing"`
	CustomRequests CustomRequestsConfig `env:"custom_requests"`
	Realtime       RealtimeConfig       `env:"realtime"`
	RateLimit      RateLimitConfig      `env:"rate_limit"`
	Storage        StorageConfig        `env:"storage"`
	Payments       PaymentsConfig       `env:"payments"`
	Analytics      AnalyticsConfig      `env:"analytics"`
	Log            LogConfig            `env:"log"`
	Mail           MailConfig           `env:"mail"`
	Worker         WorkerConfig         `env:"worker"`
	Admin          AdminConfig          `env:"admin"`
	Links          LinksConfig          `env:"links"`
	Reviews        ReviewsConfig        `env:"reviews"`
	Notifications  NotificationsConfig  `env:"notifications"`
}

// AppConfig holds process-wide identity settings.
type AppConfig struct {
	Name        string `env:"name"`
	Env         string `env:"env"`
	FrontendURL string `env:"frontend_url"`
	InstanceID  string `env:"instance_id"`
}

// HTTPConfig controls the HTTP listener.
type HTTPConfig struct {
	Addr              string        `env:"addr"`
	ReadHeaderTimeout time.Duration `env:"read_header_timeout"`
	ReadTimeout       time.Duration `env:"read_timeout"`
	WriteTimeout      time.Duration `env:"write_timeout"`
	IdleTimeout       time.Duration `env:"idle_timeout"`
	RequestTimeout    time.Duration `env:"request_timeout"`
	ShutdownTimeout   time.Duration `env:"shutdown_timeout"`
	BodyLimit         string        `env:"body_limit"`
	// IPSource selects how the client IP is resolved: "direct", "xff"
	// (X-Forwarded-For from trusted proxies), "x-real-ip" (X-Real-IP from
	// trusted proxies, set by Railway's edge) or "cloudflare"
	// (CF-Connecting-IP from trusted proxies).
	IPSource       string   `env:"ip_source"`
	TrustedProxies []string `env:"trusted_proxies"`
}

// DatabaseConfig controls the PostgreSQL pool.
type DatabaseConfig struct {
	DSN             string        `env:"dsn"`
	MaxOpenConns    int           `env:"max_open_conns"`
	MaxIdleConns    int           `env:"max_idle_conns"`
	ConnMaxLifetime time.Duration `env:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `env:"conn_max_idle_time"`
	SlowQuery       time.Duration `env:"slow_query"`
	// ConnectTimeout bounds the startup connection retries (the database may
	// still be booting or the private network warming up).
	ConnectTimeout time.Duration `env:"connect_timeout"`
}

// RedisConfig controls the Redis client.
type RedisConfig struct {
	URL          string        `env:"url"`
	DialTimeout  time.Duration `env:"dial_timeout"`
	ReadTimeout  time.Duration `env:"read_timeout"`
	WriteTimeout time.Duration `env:"write_timeout"`
	PoolSize     int           `env:"pool_size"`
	KeyPrefix    string        `env:"key_prefix"`
	// ConnectTimeout bounds the startup connection retries.
	ConnectTimeout time.Duration `env:"connect_timeout"`
}

// AuthConfig controls credentials, tokens and password hashing.
type AuthConfig struct {
	JWTSecret          string        `env:"jwt_secret"`
	JWTIssuer          string        `env:"jwt_issuer"`
	JWTAudience        string        `env:"jwt_audience"`
	AccessTokenTTL     time.Duration `env:"access_token_ttl"`
	RefreshTokenTTL    time.Duration `env:"refresh_token_ttl"`
	PasswordResetTTL   time.Duration `env:"password_reset_ttl"`
	Argon2Memory       uint32        `env:"argon2_memory_kib"`
	Argon2Iterations   uint32        `env:"argon2_iterations"`
	Argon2Parallelism  uint8         `env:"argon2_parallelism"`
	LoginMaxFailures   int           `env:"login_max_failures"`
	LoginLockout       time.Duration `env:"login_lockout"`
	MinimumAge         int           `env:"minimum_age"`
	UserStatusCacheTTL time.Duration `env:"user_status_cache_ttl"`
}

// SecurityConfig controls browser-facing protections.
type SecurityConfig struct {
	AllowedOrigins []string `env:"allowed_origins"`
	CookieDomain   string   `env:"cookie_domain"`
	CookieSecure   bool     `env:"cookie_secure"`
	CSRFSecret     string   `env:"csrf_secret"`
	HSTS           bool     `env:"hsts"`
	// AdminOrigins are the only origins allowed to call /api/v1/admin/**
	// (the admin console, e.g. https://admin.staryield.net).
	AdminOrigins []string `env:"admin_origins"`
	// AdminIPAllowlist optionally restricts admin routes to these CIDRs.
	AdminIPAllowlist []string `env:"admin_ip_allowlist"`
	// DataEncryptionKey (32 bytes, hex or base64) encrypts secrets at rest:
	// admin TOTP seeds and advisor payout details.
	DataEncryptionKey string `env:"data_encryption_key"`
}

// ChatConfig controls the live chat session lifecycle.
type ChatConfig struct {
	RequestTimeout   time.Duration `env:"request_timeout"`
	AcceptedTimeout  time.Duration `env:"accepted_timeout"`
	GracePeriod      time.Duration `env:"grace_period"`
	BillingInterval  time.Duration `env:"billing_interval"`
	MaxMessageLength int           `env:"max_message_length"`
	HistoryPageSize  int           `env:"history_page_size"`
	LowBalanceMinute int           `env:"low_balance_minutes"`
	MessageRate      int           `env:"message_rate"`
	MessageRateEvery time.Duration `env:"message_rate_every"`
}

// BillingConfig controls credits and earnings.
type BillingConfig struct {
	PsychicSharePercent int   `env:"psychic_share_percent"`
	SignupFreeMinutes   int   `env:"signup_free_minutes"`
	SignupBonusCredits  int64 `env:"signup_bonus_credits"`
	MaxAdminAdjustment  int64 `env:"max_admin_adjustment"`
	// PayoutCentsPerCredit is the money value of one earned credit when an
	// advisor withdraws (in payments.currency).
	PayoutCentsPerCredit int64 `env:"payout_cents_per_credit"`
	MinWithdrawalCredits int64 `env:"min_withdrawal_credits"`
}

// CustomRequestsConfig controls the custom request (order) lifecycle.
type CustomRequestsConfig struct {
	AcceptTimeout   time.Duration `env:"accept_timeout"`
	DeliveryTimeout time.Duration `env:"delivery_timeout"`
	DisputeWindow   time.Duration `env:"dispute_window"`
	MaxNoteLength   int           `env:"max_note_length"`
	MaxDelivery     int           `env:"max_delivery_length"`
}

// RealtimeConfig controls WebSocket/SSE delivery.
type RealtimeConfig struct {
	PingInterval      time.Duration `env:"ping_interval"`
	PongWait          time.Duration `env:"pong_wait"`
	WriteWait         time.Duration `env:"write_wait"`
	MaxFrameBytes     int64         `env:"max_frame_bytes"`
	SendBuffer        int           `env:"send_buffer"`
	TicketTTL         time.Duration `env:"ticket_ttl"`
	StreamMaxLen      int64         `env:"stream_max_len"`
	StreamTTL         time.Duration `env:"stream_ttl"`
	ReplayLimit       int64         `env:"replay_limit"`
	MaxConnsPerUser   int           `env:"max_conns_per_user"`
	PresenceTTL       time.Duration `env:"presence_ttl"`
	FrameRate         float64       `env:"frame_rate"`
	FrameBurst        int           `env:"frame_burst"`
	TypingMinInterval time.Duration `env:"typing_min_interval"`
}

// RateLimitConfig controls request throttling.
type RateLimitConfig struct {
	Enabled  bool    `env:"enabled"`
	FailOpen bool    `env:"fail_open"`
	Global   Rule    `env:"global"`
	Auth     Rule    `env:"auth"`
	Writes   Rule    `env:"writes"`
	Ingest   Rule    `env:"ingest"`
	Upload   Rule    `env:"upload"`
	Scale    float64 `env:"scale"`
}

// Rule is a token bucket: Rate tokens per Period, up to Burst.
type Rule struct {
	Rate   int           `env:"rate"`
	Period time.Duration `env:"period"`
	Burst  int           `env:"burst"`
}

// StorageConfig controls object storage (Cloudflare R2).
type StorageConfig struct {
	Driver          string `env:"driver"` // r2 | memory | disabled
	R2AccountID     string `env:"r2_account_id"`
	R2AccessKeyID   string `env:"r2_access_key_id"`
	R2SecretKey     string `env:"r2_secret_access_key"`
	R2Bucket        string `env:"r2_bucket"`
	R2Endpoint      string `env:"r2_endpoint"`
	PublicBaseURL   string `env:"public_base_url"`
	MaxAvatarBytes  int64  `env:"max_avatar_bytes"`
	MaxAvatarPixels int    `env:"max_avatar_pixels"`
	AvatarSize      int    `env:"avatar_size"`
	// AdvisorBucket (env R2_ADVISOR_IMAGE) is the public bucket for advisor
	// profile images, served from AdvisorPublicBaseURL
	// (env R2_ADVISOR_IMAGE_PUBLIC_URL).
	AdvisorBucket        string `env:"advisor_bucket"`
	AdvisorPublicBaseURL string `env:"advisor_public_base_url"`
	MaxAdvisorImageBytes int64  `env:"max_advisor_image_bytes"`
	AdvisorImageMaxSide  int    `env:"advisor_image_max_side"`
	// ChatBucket (env R2_CHAT_IMAGES_PRIVATE) is the private bucket for chat
	// images; objects are only ever streamed to conversation participants.
	ChatBucket        string `env:"chat_bucket"`
	MaxChatImageBytes int64  `env:"max_chat_image_bytes"`
	ChatImageMaxSide  int    `env:"chat_image_max_side"`
	// MaxImagePixels bounds decoded image size (decompression bombs).
	MaxImagePixels int `env:"max_image_pixels"`
}

// PaymentsConfig controls the payment provider integration.
type PaymentsConfig struct {
	Provider                 string        `env:"provider"` // sandbox | disabled
	WebhookSecret            string        `env:"webhook_secret"`
	WebhookTolerance         time.Duration `env:"webhook_tolerance"`
	AllowSandboxInProduction bool          `env:"allow_sandbox_in_production"`
	Currency                 string        `env:"currency"`
}

// AnalyticsConfig controls business event tracking.
type AnalyticsConfig struct {
	Enabled       bool          `env:"enabled"`
	BufferSize    int           `env:"buffer_size"`
	BatchSize     int           `env:"batch_size"`
	FlushInterval time.Duration `env:"flush_interval"`
	IPHashSecret  string        `env:"ip_hash_secret"`
	MaxBatch      int           `env:"max_client_batch"`
}

// LogConfig controls structured logging.
type LogConfig struct {
	Level  string `env:"level"`
	Format string `env:"format"` // json | console
}

// MailConfig controls transactional e-mail.
type MailConfig struct {
	Driver string `env:"driver"` // log | noop
	From   string `env:"from"`
}

// WorkerConfig controls background schedulers.
type WorkerConfig struct {
	Enabled         bool          `env:"enabled"`
	TickInterval    time.Duration `env:"tick_interval"`
	PresenceSweep   time.Duration `env:"presence_sweep"`
	MaintenanceTick time.Duration `env:"maintenance_tick"`
	BatchSize       int           `env:"batch_size"`
}

// AdminConfig controls admin console sessions.
type AdminConfig struct {
	SessionTTL   time.Duration `env:"session_ttl"`
	IdleTimeout  time.Duration `env:"idle_timeout"`
	StepUpWindow time.Duration `env:"step_up_window"`
	ChallengeTTL time.Duration `env:"challenge_ttl"`
	TOTPIssuer   string        `env:"totp_issuer"`
}

// LinksConfig controls marketing short links.
type LinksConfig struct {
	// PublicBaseURL prefixes short codes, e.g. https://staryield.net/go.
	PublicBaseURL string `env:"public_base_url"`
	// AllowedHosts may be used as absolute link destinations (the frontend
	// host is always allowed).
	AllowedHosts []string `env:"allowed_hosts"`
}

// ReviewsConfig controls advisor ratings.
type ReviewsConfig struct {
	Window time.Duration `env:"window"`
}

// NotificationsConfig controls the asynchronous notification dispatcher.
type NotificationsConfig struct {
	BufferSize    int           `env:"buffer_size"`
	BatchSize     int           `env:"batch_size"`
	FlushInterval time.Duration `env:"flush_interval"`
}

// IsProduction reports whether the app runs in production.
func (c *Config) IsProduction() bool { return c.App.Env == EnvProduction }

// railwayTrustedProxies are the private ranges Railway's edge proxy connects
// from. Containers are only reachable through the edge or the project's
// private network, so headers from these peers are trusted.
var railwayTrustedProxies = []string{"10.0.0.0/8", "100.64.0.0/10", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"}

// applyRailwayDefaults makes a Railway deployment secure by default: it runs
// in production mode (so development secrets are never derived), behind TLS,
// and resolves client IPs from the edge's X-Real-IP header. Every value can
// still be overridden by its environment variable.
func (c *Config) applyRailwayDefaults() {
	c.App.Env = EnvProduction
	c.HTTP.IPSource = "x-real-ip"
	c.HTTP.TrustedProxies = append([]string(nil), railwayTrustedProxies...)
	c.Security.CookieSecure = true
	c.Security.HSTS = true
	c.Log.Format = "json"
}

// defaults returns the built-in configuration (local development).
func defaults() Config {
	return Config{
		App: AppConfig{Name: "staryield-api", Env: EnvDevelopment, FrontendURL: "http://localhost:3000"},
		HTTP: HTTPConfig{
			Addr:              ":8080",
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      0, // streaming endpoints manage their own deadlines
			IdleTimeout:       120 * time.Second,
			RequestTimeout:    15 * time.Second,
			ShutdownTimeout:   20 * time.Second,
			BodyLimit:         "64KB",
			IPSource:          "direct",
			TrustedProxies:    []string{},
		},
		Database: DatabaseConfig{
			DSN:             "postgres://postgres@127.0.0.1:5432/staryield?sslmode=disable",
			MaxOpenConns:    25,
			MaxIdleConns:    10,
			ConnMaxLifetime: 30 * time.Minute,
			ConnMaxIdleTime: 5 * time.Minute,
			SlowQuery:       250 * time.Millisecond,
			ConnectTimeout:  60 * time.Second,
		},
		Redis: RedisConfig{
			URL:            "redis://127.0.0.1:6379/0",
			DialTimeout:    3 * time.Second,
			ReadTimeout:    3 * time.Second,
			WriteTimeout:   3 * time.Second,
			PoolSize:       50,
			KeyPrefix:      "sy:",
			ConnectTimeout: 60 * time.Second,
		},
		Auth: AuthConfig{
			JWTIssuer:          "staryield-api",
			JWTAudience:        "staryield-web",
			AccessTokenTTL:     15 * time.Minute,
			RefreshTokenTTL:    720 * time.Hour,
			PasswordResetTTL:   30 * time.Minute,
			Argon2Memory:       64 * 1024,
			Argon2Iterations:   3,
			Argon2Parallelism:  2,
			LoginMaxFailures:   5,
			LoginLockout:       15 * time.Minute,
			MinimumAge:         18,
			UserStatusCacheTTL: 30 * time.Second,
		},
		Security: SecurityConfig{
			AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:3001"},
			AdminOrigins:     []string{"http://localhost:3002"},
			AdminIPAllowlist: []string{},
		},
		Chat: ChatConfig{
			RequestTimeout:   90 * time.Second,
			AcceptedTimeout:  120 * time.Second,
			GracePeriod:      90 * time.Second,
			BillingInterval:  60 * time.Second,
			MaxMessageLength: 1000,
			HistoryPageSize:  50,
			LowBalanceMinute: 2,
			MessageRate:      20,
			MessageRateEvery: 10 * time.Second,
		},
		Billing: BillingConfig{
			PsychicSharePercent:  60,
			SignupFreeMinutes:    3,
			MaxAdminAdjustment:   100000,
			PayoutCentsPerCredit: 2,
			MinWithdrawalCredits: 2500,
		},
		CustomRequests: CustomRequestsConfig{
			AcceptTimeout:   48 * time.Hour,
			DeliveryTimeout: 168 * time.Hour,
			DisputeWindow:   336 * time.Hour,
			MaxNoteLength:   2000,
			MaxDelivery:     10000,
		},
		Realtime: RealtimeConfig{
			PingInterval:      25 * time.Second,
			PongWait:          60 * time.Second,
			WriteWait:         10 * time.Second,
			MaxFrameBytes:     16 * 1024,
			SendBuffer:        256,
			TicketTTL:         30 * time.Second,
			StreamMaxLen:      500,
			StreamTTL:         24 * time.Hour,
			ReplayLimit:       500,
			MaxConnsPerUser:   6,
			PresenceTTL:       45 * time.Second,
			FrameRate:         10,
			FrameBurst:        30,
			TypingMinInterval: 1500 * time.Millisecond,
		},
		RateLimit: RateLimitConfig{
			Enabled:  true,
			FailOpen: true,
			Global:   Rule{Rate: 300, Period: time.Minute, Burst: 120},
			Auth:     Rule{Rate: 10, Period: time.Minute, Burst: 10},
			Writes:   Rule{Rate: 60, Period: time.Minute, Burst: 30},
			Ingest:   Rule{Rate: 120, Period: time.Minute, Burst: 60},
			Upload:   Rule{Rate: 10, Period: time.Hour, Burst: 5},
			Scale:    1,
		},
		Storage: StorageConfig{
			Driver:               "memory",
			R2Bucket:             "staryield-app-assets",
			PublicBaseURL:        "https://assets.staryield.net",
			MaxAvatarBytes:       1 << 20,
			MaxAvatarPixels:      4096 * 4096,
			AvatarSize:           256,
			AdvisorPublicBaseURL: "https://advisorsassets.staryield.net",
			MaxAdvisorImageBytes: 5 << 20,
			AdvisorImageMaxSide:  1200,
			MaxChatImageBytes:    8 << 20,
			ChatImageMaxSide:     2048,
			MaxImagePixels:       40_000_000,
		},
		Payments:      PaymentsConfig{Provider: "sandbox", WebhookTolerance: 5 * time.Minute, Currency: "USD"},
		Analytics:     AnalyticsConfig{Enabled: true, BufferSize: 10000, BatchSize: 200, FlushInterval: 2 * time.Second, MaxBatch: 50},
		Log:           LogConfig{Level: "info", Format: "json"},
		Mail:          MailConfig{Driver: "log", From: "Staryield <no-reply@staryield.net>"},
		Worker:        WorkerConfig{Enabled: true, TickInterval: time.Second, PresenceSweep: 5 * time.Second, MaintenanceTick: 30 * time.Second, BatchSize: 100},
		Admin:         AdminConfig{SessionTTL: 8 * time.Hour, IdleTimeout: 30 * time.Minute, StepUpWindow: 10 * time.Minute, ChallengeTTL: 5 * time.Minute, TOTPIssuer: "Staryield Admin"},
		Links:         LinksConfig{PublicBaseURL: "http://localhost:3000/go", AllowedHosts: []string{}},
		Reviews:       ReviewsConfig{Window: 168 * time.Hour},
		Notifications: NotificationsConfig{BufferSize: 10000, BatchSize: 200, FlushInterval: 250 * time.Millisecond},
	}
}

// normalize applies derived values and trims list inputs that may arrive as a
// single comma-separated environment variable.
func (c *Config) normalize() {
	c.App.Env = strings.ToLower(strings.TrimSpace(c.App.Env))
	c.Security.AllowedOrigins = splitList(c.Security.AllowedOrigins)
	c.Security.AdminOrigins = splitList(c.Security.AdminOrigins)
	c.Security.AdminIPAllowlist = splitList(c.Security.AdminIPAllowlist)
	c.Links.AllowedHosts = splitList(c.Links.AllowedHosts)
	c.Links.PublicBaseURL = strings.TrimRight(c.Links.PublicBaseURL, "/")
	c.Storage.AdvisorPublicBaseURL = strings.TrimRight(c.Storage.AdvisorPublicBaseURL, "/")
	c.HTTP.TrustedProxies = splitList(c.HTTP.TrustedProxies)
	c.App.FrontendURL = strings.TrimRight(c.App.FrontendURL, "/")
	c.Storage.PublicBaseURL = strings.TrimRight(c.Storage.PublicBaseURL, "/")
	if c.Storage.R2Endpoint == "" && c.Storage.R2AccountID != "" {
		c.Storage.R2Endpoint = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", c.Storage.R2AccountID)
	}
	if c.RateLimit.Scale <= 0 {
		c.RateLimit.Scale = 1
	}
	if c.Payments.Currency == "" {
		c.Payments.Currency = "USD"
	}
	c.Payments.Currency = strings.ToUpper(c.Payments.Currency)
	// Development conveniences: derive secrets so the app boots locally, but
	// never in production (Validate rejects missing secrets there).
	if c.App.Env != EnvProduction {
		if c.Auth.JWTSecret == "" {
			c.Auth.JWTSecret = "dev-only-insecure-jwt-secret-change-me-0123456789"
		}
		if c.Security.CSRFSecret == "" {
			c.Security.CSRFSecret = "dev-only-insecure-csrf-secret-change-me-0123456789"
		}
		if c.Analytics.IPHashSecret == "" {
			c.Analytics.IPHashSecret = "dev-only-insecure-ip-hash-secret"
		}
		if c.Payments.WebhookSecret == "" {
			c.Payments.WebhookSecret = "dev-only-insecure-webhook-secret"
		}
		if c.Security.DataEncryptionKey == "" {
			sum := sha256.Sum256([]byte("staryield-data-key:" + c.Security.CSRFSecret))
			c.Security.DataEncryptionKey = "dev-only:" + hex.EncodeToString(sum[:])
		}
	}
}

// EncryptionKey decodes STARYIELD_SECURITY_DATA_ENCRYPTION_KEY: 32 bytes as
// 64 hex characters or base64 (development keys carry a "dev-only:" prefix).
// Errors describe the problem without echoing the key.
func (c *Config) EncryptionKey() ([]byte, error) {
	name := EnvVar("security.data_encryption_key")
	raw := strings.TrimPrefix(strings.TrimSpace(c.Security.DataEncryptionKey), "dev-only:")
	if raw == "" {
		return nil, fmt.Errorf("%s is required: 32 random bytes as 64 hex characters (openssl rand -hex 32)", name)
	}
	if b, err := hex.DecodeString(raw); err == nil {
		if len(b) == 32 {
			return b, nil
		}
		return nil, fmt.Errorf("%s must be 32 bytes (64 hex characters), got %d bytes (%d hex characters); generate one with openssl rand -hex 32", name, len(b), len(raw))
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(raw); err == nil {
			if len(b) == 32 {
				return b, nil
			}
			return nil, fmt.Errorf("%s must be 32 bytes, got %d bytes of base64; generate one with openssl rand -hex 32", name, len(b))
		}
	}
	return nil, fmt.Errorf("%s must be 32 bytes as 64 hex characters or base64 (%d characters given); generate one with openssl rand -hex 32", name, len(raw))
}

func splitList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		for _, part := range strings.Split(item, ",") {
			if p := strings.TrimSpace(part); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

// Validate checks invariants and production hardening requirements. Errors
// name the environment variable to change.
func (c *Config) Validate() error {
	var errs []error
	add := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	e := EnvVar

	switch c.App.Env {
	case EnvDevelopment, EnvTest, EnvProduction:
	default:
		add("%s must be one of development, test, production (got %q)", e("app.env"), c.App.Env)
	}
	if c.Database.DSN == "" {
		add("%s (or DATABASE_URL) is required", e("database.dsn"))
	}
	if c.Redis.URL == "" {
		add("%s (or REDIS_URL) is required", e("redis.url"))
	}
	if len(c.Auth.JWTSecret) < 32 {
		add("%s must be at least 32 bytes", e("auth.jwt_secret"))
	}
	if len(c.Security.CSRFSecret) < 32 {
		add("%s must be at least 32 bytes", e("security.csrf_secret"))
	}
	if c.Auth.AccessTokenTTL <= 0 || c.Auth.AccessTokenTTL > time.Hour {
		add("%s must be in (0, 1h]", e("auth.access_token_ttl"))
	}
	if c.Auth.RefreshTokenTTL < c.Auth.AccessTokenTTL {
		add("%s must be >= %s", e("auth.refresh_token_ttl"), e("auth.access_token_ttl"))
	}
	if c.Auth.Argon2Memory < 8*1024 && c.App.Env != EnvTest {
		add("%s must be >= 8192 outside tests", e("auth.argon2_memory_kib"))
	}
	if c.Auth.Argon2Iterations < 1 || c.Auth.Argon2Parallelism < 1 {
		add("%s and %s must be >= 1", e("auth.argon2_iterations"), e("auth.argon2_parallelism"))
	}
	if c.Billing.PsychicSharePercent < 0 || c.Billing.PsychicSharePercent > 100 {
		add("%s must be within 0..100", e("billing.psychic_share_percent"))
	}
	if c.Chat.BillingInterval <= 0 || c.Chat.RequestTimeout <= 0 || c.Chat.GracePeriod <= 0 || c.Chat.AcceptedTimeout <= 0 {
		add("%s, %s, %s and %s must be positive", e("chat.billing_interval"), e("chat.request_timeout"), e("chat.grace_period"), e("chat.accepted_timeout"))
	}
	if c.Chat.MaxMessageLength < 1 || c.Chat.MaxMessageLength > 10000 {
		add("%s must be within 1..10000", e("chat.max_message_length"))
	}
	if c.Realtime.PingInterval >= c.Realtime.PongWait {
		add("%s must be shorter than %s", e("realtime.ping_interval"), e("realtime.pong_wait"))
	}
	if c.Realtime.SendBuffer < 16 {
		add("%s must be >= 16", e("realtime.send_buffer"))
	}
	for _, origin := range c.Security.AllowedOrigins {
		if origin == "*" {
			add("%s must not contain '*' (credentials are used)", e("security.allowed_origins"))
			continue
		}
		u, err := url.Parse(origin)
		if err != nil || u.Scheme == "" || u.Host == "" || u.Path != "" {
			add("%s entry %q must be scheme://host[:port]", e("security.allowed_origins"), origin)
		}
	}
	for _, cidr := range c.HTTP.TrustedProxies {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			add("%s entry %q is not a CIDR", e("http.trusted_proxies"), cidr)
		}
	}
	for _, cidr := range c.Security.AdminIPAllowlist {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			add("%s entry %q is not a CIDR", e("security.admin_ip_allowlist"), cidr)
		}
	}
	if len(c.Security.AdminOrigins) == 0 {
		add("%s must list the admin console origin", e("security.admin_origins"))
	}
	for _, origin := range c.Security.AdminOrigins {
		u, err := url.Parse(origin)
		if origin == "*" || err != nil || u.Scheme == "" || u.Host == "" || u.Path != "" {
			add("%s entry %q must be scheme://host[:port]", e("security.admin_origins"), origin)
		}
	}
	if _, err := c.EncryptionKey(); err != nil {
		add("%v", err)
	}
	if c.Admin.SessionTTL <= 0 || c.Admin.IdleTimeout <= 0 || c.Admin.IdleTimeout > c.Admin.SessionTTL || c.Admin.StepUpWindow <= 0 || c.Admin.ChallengeTTL <= 0 {
		add("%s, %s (<= session TTL), %s and %s must be positive", e("admin.session_ttl"), e("admin.idle_timeout"), e("admin.step_up_window"), e("admin.challenge_ttl"))
	}
	if c.Admin.SessionTTL > 24*time.Hour {
		add("%s must be at most 24h", e("admin.session_ttl"))
	}
	if u, err := url.Parse(c.Links.PublicBaseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		add("%s must be an absolute http(s) URL", e("links.public_base_url"))
	}
	if c.Billing.PayoutCentsPerCredit < 0 || c.Billing.MinWithdrawalCredits < 1 {
		add("%s must be >= 0 and %s >= 1", e("billing.payout_cents_per_credit"), e("billing.min_withdrawal_credits"))
	}
	if c.Reviews.Window <= 0 {
		add("%s must be positive", e("reviews.window"))
	}
	if c.Notifications.BufferSize < 1 || c.Notifications.BatchSize < 1 {
		add("%s and %s must be >= 1", e("notifications.buffer_size"), e("notifications.batch_size"))
	}
	switch c.HTTP.IPSource {
	case "direct", "xff", "x-real-ip", "cloudflare":
	default:
		add("%s must be direct, xff, x-real-ip or cloudflare", e("http.ip_source"))
	}
	if (c.HTTP.IPSource == "x-real-ip" || c.HTTP.IPSource == "cloudflare") && len(c.HTTP.TrustedProxies) == 0 {
		add("%s=%s requires %s", e("http.ip_source"), c.HTTP.IPSource, e("http.trusted_proxies"))
	}
	if c.Database.ConnectTimeout < 0 || c.Redis.ConnectTimeout < 0 {
		add("%s and %s must not be negative", e("database.connect_timeout"), e("redis.connect_timeout"))
	}
	switch c.Storage.Driver {
	case "memory", "disabled":
	case "r2":
		if c.Storage.R2AccessKeyID == "" || c.Storage.R2SecretKey == "" || c.Storage.R2Bucket == "" || c.Storage.R2Endpoint == "" {
			add("%s=r2 requires %s, %s, %s and %s (or %s)", e("storage.driver"), e("storage.r2_access_key_id"), e("storage.r2_secret_access_key"),
				e("storage.r2_bucket"), e("storage.r2_account_id"), e("storage.r2_endpoint"))
		}
	default:
		add("%s must be r2, memory or disabled", e("storage.driver"))
	}
	switch c.Payments.Provider {
	case "disabled":
	case "sandbox":
		if c.IsProduction() && !c.Payments.AllowSandboxInProduction {
			add("%s=sandbox is not allowed in production", e("payments.provider"))
		}
	default:
		add("%s %q is not supported", e("payments.provider"), c.Payments.Provider)
	}
	switch c.Log.Format {
	case "json", "console":
	default:
		add("%s must be json or console", e("log.format"))
	}

	if c.IsProduction() {
		if !c.Security.CookieSecure {
			add("%s must be true in production", e("security.cookie_secure"))
		}
		if len(c.Security.AllowedOrigins) == 0 {
			add("%s must be set in production", e("security.allowed_origins"))
		}
		for _, origin := range c.Security.AllowedOrigins {
			if strings.HasPrefix(origin, "http://") {
				add("%s must use https in production (%q)", e("security.allowed_origins"), origin)
			}
		}
		if strings.HasPrefix(c.Auth.JWTSecret, "dev-only") || strings.HasPrefix(c.Security.CSRFSecret, "dev-only") {
			add("development secrets must not be used in production")
		}
		if len(c.Analytics.IPHashSecret) < 16 {
			add("%s must be at least 16 bytes in production", e("analytics.ip_hash_secret"))
		}
		if len(c.Payments.WebhookSecret) < 16 {
			add("%s must be at least 16 bytes in production", e("payments.webhook_secret"))
		}
		if c.Storage.Driver == "memory" {
			add("%s=memory is not allowed in production (use r2, or disabled to turn uploads off)", e("storage.driver"))
		}
		if c.Mail.Driver == "log" {
			add("%s=log is not allowed in production (use noop)", e("mail.driver"))
		}
		if strings.HasPrefix(c.Security.DataEncryptionKey, "dev-only") {
			add("%s must be set in production", e("security.data_encryption_key"))
		}
		for _, origin := range c.Security.AdminOrigins {
			if !strings.HasPrefix(origin, "https://") {
				add("%s must use https in production (%q)", e("security.admin_origins"), origin)
			}
		}
	}
	return errors.Join(errs...)
}
