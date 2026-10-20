// Package config loads and validates runtime configuration with Viper.
//
// Values come from (lowest to highest precedence): built-in defaults, platform
// defaults (Railway), an optional YAML file, and STARYIELD_* environment
// variables, where nested keys use underscores (e.g.
// STARYIELD_AUTH_JWT_SECRET). The platform conventions DATABASE_URL,
// REDIS_URL, APP_ENV and PORT are honoured when the STARYIELD_* equivalent is
// unset.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Environment names.
const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvProduction  = "production"
)

// Config is the fully-resolved application configuration.
type Config struct {
	App            AppConfig            `mapstructure:"app"`
	HTTP           HTTPConfig           `mapstructure:"http"`
	Database       DatabaseConfig       `mapstructure:"database"`
	Redis          RedisConfig          `mapstructure:"redis"`
	Auth           AuthConfig           `mapstructure:"auth"`
	Security       SecurityConfig       `mapstructure:"security"`
	Chat           ChatConfig           `mapstructure:"chat"`
	Billing        BillingConfig        `mapstructure:"billing"`
	CustomRequests CustomRequestsConfig `mapstructure:"custom_requests"`
	Realtime       RealtimeConfig       `mapstructure:"realtime"`
	RateLimit      RateLimitConfig      `mapstructure:"rate_limit"`
	Storage        StorageConfig        `mapstructure:"storage"`
	Payments       PaymentsConfig       `mapstructure:"payments"`
	Analytics      AnalyticsConfig      `mapstructure:"analytics"`
	Log            LogConfig            `mapstructure:"log"`
	Mail           MailConfig           `mapstructure:"mail"`
	Worker         WorkerConfig         `mapstructure:"worker"`
}

// AppConfig holds process-wide identity settings.
type AppConfig struct {
	Name        string `mapstructure:"name"`
	Env         string `mapstructure:"env"`
	FrontendURL string `mapstructure:"frontend_url"`
	InstanceID  string `mapstructure:"instance_id"`
}

// HTTPConfig controls the HTTP listener.
type HTTPConfig struct {
	Addr              string        `mapstructure:"addr"`
	ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout"`
	IdleTimeout       time.Duration `mapstructure:"idle_timeout"`
	RequestTimeout    time.Duration `mapstructure:"request_timeout"`
	ShutdownTimeout   time.Duration `mapstructure:"shutdown_timeout"`
	BodyLimit         string        `mapstructure:"body_limit"`
	// IPSource selects how the client IP is resolved: "direct", "xff"
	// (X-Forwarded-For from trusted proxies), "x-real-ip" (X-Real-IP from
	// trusted proxies, set by Railway's edge) or "cloudflare"
	// (CF-Connecting-IP from trusted proxies).
	IPSource       string   `mapstructure:"ip_source"`
	TrustedProxies []string `mapstructure:"trusted_proxies"`
}

// DatabaseConfig controls the PostgreSQL pool.
type DatabaseConfig struct {
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
	SlowQuery       time.Duration `mapstructure:"slow_query"`
	AutoMigrate     bool          `mapstructure:"auto_migrate"`
	// ConnectTimeout bounds the startup connection retries (the database may
	// still be booting or the private network warming up).
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"`
}

// RedisConfig controls the Redis client.
type RedisConfig struct {
	URL          string        `mapstructure:"url"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	PoolSize     int           `mapstructure:"pool_size"`
	KeyPrefix    string        `mapstructure:"key_prefix"`
	// ConnectTimeout bounds the startup connection retries.
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"`
}

// AuthConfig controls credentials, tokens and password hashing.
type AuthConfig struct {
	JWTSecret          string        `mapstructure:"jwt_secret"`
	JWTIssuer          string        `mapstructure:"jwt_issuer"`
	JWTAudience        string        `mapstructure:"jwt_audience"`
	AccessTokenTTL     time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL    time.Duration `mapstructure:"refresh_token_ttl"`
	PasswordResetTTL   time.Duration `mapstructure:"password_reset_ttl"`
	Argon2Memory       uint32        `mapstructure:"argon2_memory_kib"`
	Argon2Iterations   uint32        `mapstructure:"argon2_iterations"`
	Argon2Parallelism  uint8         `mapstructure:"argon2_parallelism"`
	LoginMaxFailures   int           `mapstructure:"login_max_failures"`
	LoginLockout       time.Duration `mapstructure:"login_lockout"`
	MinimumAge         int           `mapstructure:"minimum_age"`
	UserStatusCacheTTL time.Duration `mapstructure:"user_status_cache_ttl"`
}

// SecurityConfig controls browser-facing protections.
type SecurityConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	CookieDomain   string   `mapstructure:"cookie_domain"`
	CookieSecure   bool     `mapstructure:"cookie_secure"`
	CSRFSecret     string   `mapstructure:"csrf_secret"`
	HSTS           bool     `mapstructure:"hsts"`
}

// ChatConfig controls the live chat session lifecycle.
type ChatConfig struct {
	RequestTimeout   time.Duration `mapstructure:"request_timeout"`
	AcceptedTimeout  time.Duration `mapstructure:"accepted_timeout"`
	GracePeriod      time.Duration `mapstructure:"grace_period"`
	BillingInterval  time.Duration `mapstructure:"billing_interval"`
	MaxMessageLength int           `mapstructure:"max_message_length"`
	HistoryPageSize  int           `mapstructure:"history_page_size"`
	LowBalanceMinute int           `mapstructure:"low_balance_minutes"`
	MessageRate      int           `mapstructure:"message_rate"`
	MessageRateEvery time.Duration `mapstructure:"message_rate_every"`
}

// BillingConfig controls credits and earnings.
type BillingConfig struct {
	PsychicSharePercent int   `mapstructure:"psychic_share_percent"`
	SignupFreeMinutes   int   `mapstructure:"signup_free_minutes"`
	SignupBonusCredits  int64 `mapstructure:"signup_bonus_credits"`
	MaxAdminAdjustment  int64 `mapstructure:"max_admin_adjustment"`
}

// CustomRequestsConfig controls the custom request (order) lifecycle.
type CustomRequestsConfig struct {
	AcceptTimeout   time.Duration `mapstructure:"accept_timeout"`
	DeliveryTimeout time.Duration `mapstructure:"delivery_timeout"`
	DisputeWindow   time.Duration `mapstructure:"dispute_window"`
	MaxNoteLength   int           `mapstructure:"max_note_length"`
	MaxDelivery     int           `mapstructure:"max_delivery_length"`
}

// RealtimeConfig controls WebSocket/SSE delivery.
type RealtimeConfig struct {
	PingInterval      time.Duration `mapstructure:"ping_interval"`
	PongWait          time.Duration `mapstructure:"pong_wait"`
	WriteWait         time.Duration `mapstructure:"write_wait"`
	MaxFrameBytes     int64         `mapstructure:"max_frame_bytes"`
	SendBuffer        int           `mapstructure:"send_buffer"`
	TicketTTL         time.Duration `mapstructure:"ticket_ttl"`
	StreamMaxLen      int64         `mapstructure:"stream_max_len"`
	StreamTTL         time.Duration `mapstructure:"stream_ttl"`
	ReplayLimit       int64         `mapstructure:"replay_limit"`
	MaxConnsPerUser   int           `mapstructure:"max_conns_per_user"`
	PresenceTTL       time.Duration `mapstructure:"presence_ttl"`
	FrameRate         float64       `mapstructure:"frame_rate"`
	FrameBurst        int           `mapstructure:"frame_burst"`
	TypingMinInterval time.Duration `mapstructure:"typing_min_interval"`
}

// RateLimitConfig controls request throttling.
type RateLimitConfig struct {
	Enabled  bool    `mapstructure:"enabled"`
	FailOpen bool    `mapstructure:"fail_open"`
	Global   Rule    `mapstructure:"global"`
	Auth     Rule    `mapstructure:"auth"`
	Writes   Rule    `mapstructure:"writes"`
	Ingest   Rule    `mapstructure:"ingest"`
	Upload   Rule    `mapstructure:"upload"`
	Scale    float64 `mapstructure:"scale"`
}

// Rule is a token bucket: Rate tokens per Period, up to Burst.
type Rule struct {
	Rate   int           `mapstructure:"rate"`
	Period time.Duration `mapstructure:"period"`
	Burst  int           `mapstructure:"burst"`
}

// StorageConfig controls object storage (Cloudflare R2).
type StorageConfig struct {
	Driver          string `mapstructure:"driver"` // r2 | memory | disabled
	R2AccountID     string `mapstructure:"r2_account_id"`
	R2AccessKeyID   string `mapstructure:"r2_access_key_id"`
	R2SecretKey     string `mapstructure:"r2_secret_access_key"`
	R2Bucket        string `mapstructure:"r2_bucket"`
	R2Endpoint      string `mapstructure:"r2_endpoint"`
	PublicBaseURL   string `mapstructure:"public_base_url"`
	MaxAvatarBytes  int64  `mapstructure:"max_avatar_bytes"`
	MaxAvatarPixels int    `mapstructure:"max_avatar_pixels"`
	AvatarSize      int    `mapstructure:"avatar_size"`
}

// PaymentsConfig controls the payment provider integration.
type PaymentsConfig struct {
	Provider                 string        `mapstructure:"provider"` // sandbox | disabled
	WebhookSecret            string        `mapstructure:"webhook_secret"`
	WebhookTolerance         time.Duration `mapstructure:"webhook_tolerance"`
	AllowSandboxInProduction bool          `mapstructure:"allow_sandbox_in_production"`
	Currency                 string        `mapstructure:"currency"`
}

// AnalyticsConfig controls business event tracking.
type AnalyticsConfig struct {
	Enabled       bool          `mapstructure:"enabled"`
	BufferSize    int           `mapstructure:"buffer_size"`
	BatchSize     int           `mapstructure:"batch_size"`
	FlushInterval time.Duration `mapstructure:"flush_interval"`
	IPHashSecret  string        `mapstructure:"ip_hash_secret"`
	MaxBatch      int           `mapstructure:"max_client_batch"`
}

// LogConfig controls structured logging.
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"` // json | console
}

// MailConfig controls transactional e-mail.
type MailConfig struct {
	Driver string `mapstructure:"driver"` // log | noop
	From   string `mapstructure:"from"`
}

// WorkerConfig controls background schedulers.
type WorkerConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	TickInterval    time.Duration `mapstructure:"tick_interval"`
	PresenceSweep   time.Duration `mapstructure:"presence_sweep"`
	MaintenanceTick time.Duration `mapstructure:"maintenance_tick"`
	BatchSize       int           `mapstructure:"batch_size"`
}

// IsProduction reports whether the app runs in production.
func (c *Config) IsProduction() bool { return c.App.Env == EnvProduction }

// Load reads configuration from defaults, an optional file and environment.
// path may be empty; then ./config.yaml is used when present.
func Load(path string) (*Config, error) {
	v := viper.New()
	setDefaults(v)
	if OnRailway() {
		setRailwayDefaults(v)
	}

	v.SetEnvPrefix("STARYIELD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	for key, names := range platformEnv {
		if err := v.BindEnv(append([]string{key}, names...)...); err != nil {
			return nil, fmt.Errorf("config: bind %s: %w", key, err)
		}
	}

	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
	}
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if path != "" || !errors.As(err, &notFound) {
			return nil, fmt.Errorf("config: read: %w", err)
		}
	}

	// PORT is the platform-assigned listen port; an explicit
	// STARYIELD_HTTP_ADDR still wins.
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" && os.Getenv("STARYIELD_HTTP_ADDR") == "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("config: PORT must be a TCP port (got %q)", port)
		}
		v.Set("http.addr", ":"+port)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config: decode: %w", err)
	}
	cfg.normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// platformEnv maps config keys to the environment variables consulted, in
// order: the STARYIELD_* name first, then the conventional variable hosting
// platforms inject (Railway, Heroku, Render, Fly…).
var platformEnv = map[string][]string{
	"database.dsn":    {"STARYIELD_DATABASE_DSN", "DATABASE_URL"},
	"redis.url":       {"STARYIELD_REDIS_URL", "REDIS_URL"},
	"app.env":         {"STARYIELD_APP_ENV", "APP_ENV"},
	"app.instance_id": {"STARYIELD_APP_INSTANCE_ID", "RAILWAY_REPLICA_ID"},
}

// OnRailway reports whether the process runs on Railway.
func OnRailway() bool { return os.Getenv("RAILWAY_ENVIRONMENT_ID") != "" }

// railwayTrustedProxies are the private ranges Railway's edge proxy connects
// from. Containers are only reachable through the edge or the project's
// private network, so headers from these peers are trusted.
var railwayTrustedProxies = []string{"10.0.0.0/8", "100.64.0.0/10", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"}

// setRailwayDefaults makes a Railway deployment secure by default: it runs in
// production mode (so development secrets are never derived), behind TLS,
// and resolves client IPs from the edge's X-Real-IP header. Every value can
// still be overridden explicitly.
func setRailwayDefaults(v *viper.Viper) {
	v.SetDefault("app.env", EnvProduction)
	v.SetDefault("http.ip_source", "x-real-ip")
	v.SetDefault("http.trusted_proxies", railwayTrustedProxies)
	v.SetDefault("security.cookie_secure", true)
	v.SetDefault("security.hsts", true)
	v.SetDefault("log.format", "json")
}

func setDefaults(v *viper.Viper) {
	d := map[string]any{
		"app.name":         "staryield-api",
		"app.env":          EnvDevelopment,
		"app.frontend_url": "http://localhost:3000",
		"app.instance_id":  "",

		"http.addr":                ":8080",
		"http.read_header_timeout": "5s",
		"http.read_timeout":        "30s",
		"http.write_timeout":       "0s", // streaming endpoints manage their own deadlines
		"http.idle_timeout":        "120s",
		"http.request_timeout":     "15s",
		"http.shutdown_timeout":    "20s",
		"http.body_limit":          "64KB",
		"http.ip_source":           "direct",
		"http.trusted_proxies":     []string{},

		"database.dsn":                "postgres://postgres@127.0.0.1:5432/staryield?sslmode=disable",
		"database.max_open_conns":     25,
		"database.max_idle_conns":     10,
		"database.conn_max_lifetime":  "30m",
		"database.conn_max_idle_time": "5m",
		"database.slow_query":         "250ms",
		"database.auto_migrate":       true,
		"database.connect_timeout":    "60s",

		"redis.url":             "redis://127.0.0.1:6379/0",
		"redis.dial_timeout":    "3s",
		"redis.read_timeout":    "3s",
		"redis.write_timeout":   "3s",
		"redis.pool_size":       50,
		"redis.key_prefix":      "sy:",
		"redis.connect_timeout": "60s",

		"auth.jwt_secret":            "",
		"auth.jwt_issuer":            "staryield-api",
		"auth.jwt_audience":          "staryield-web",
		"auth.access_token_ttl":      "15m",
		"auth.refresh_token_ttl":     "720h",
		"auth.password_reset_ttl":    "30m",
		"auth.argon2_memory_kib":     64 * 1024,
		"auth.argon2_iterations":     3,
		"auth.argon2_parallelism":    2,
		"auth.login_max_failures":    5,
		"auth.login_lockout":         "15m",
		"auth.minimum_age":           18,
		"auth.user_status_cache_ttl": "30s",

		"security.allowed_origins": []string{"http://localhost:3000", "http://localhost:3001"},
		"security.cookie_domain":   "",
		"security.cookie_secure":   false,
		"security.csrf_secret":     "",
		"security.hsts":            false,

		"chat.request_timeout":     "90s",
		"chat.accepted_timeout":    "120s",
		"chat.grace_period":        "90s",
		"chat.billing_interval":    "60s",
		"chat.max_message_length":  1000,
		"chat.history_page_size":   50,
		"chat.low_balance_minutes": 2,
		"chat.message_rate":        20,
		"chat.message_rate_every":  "10s",

		"billing.psychic_share_percent": 60,
		"billing.signup_free_minutes":   3,
		"billing.signup_bonus_credits":  0,
		"billing.max_admin_adjustment":  100000,

		"custom_requests.accept_timeout":      "48h",
		"custom_requests.delivery_timeout":    "168h",
		"custom_requests.dispute_window":      "336h",
		"custom_requests.max_note_length":     2000,
		"custom_requests.max_delivery_length": 10000,

		"realtime.ping_interval":       "25s",
		"realtime.pong_wait":           "60s",
		"realtime.write_wait":          "10s",
		"realtime.max_frame_bytes":     16 * 1024,
		"realtime.send_buffer":         256,
		"realtime.ticket_ttl":          "30s",
		"realtime.stream_max_len":      500,
		"realtime.stream_ttl":          "24h",
		"realtime.replay_limit":        500,
		"realtime.max_conns_per_user":  6,
		"realtime.presence_ttl":        "45s",
		"realtime.frame_rate":          10.0,
		"realtime.frame_burst":         30,
		"realtime.typing_min_interval": "1500ms",

		"rate_limit.enabled":       true,
		"rate_limit.fail_open":     true,
		"rate_limit.scale":         1.0,
		"rate_limit.global.rate":   300,
		"rate_limit.global.period": "1m",
		"rate_limit.global.burst":  120,
		"rate_limit.auth.rate":     10,
		"rate_limit.auth.period":   "1m",
		"rate_limit.auth.burst":    10,
		"rate_limit.writes.rate":   60,
		"rate_limit.writes.period": "1m",
		"rate_limit.writes.burst":  30,
		"rate_limit.ingest.rate":   120,
		"rate_limit.ingest.period": "1m",
		"rate_limit.ingest.burst":  60,
		"rate_limit.upload.rate":   10,
		"rate_limit.upload.period": "1h",
		"rate_limit.upload.burst":  5,

		"storage.driver":               "memory",
		"storage.r2_account_id":        "",
		"storage.r2_access_key_id":     "",
		"storage.r2_secret_access_key": "",
		"storage.r2_bucket":            "staryield-app-assets",
		"storage.r2_endpoint":          "",
		"storage.public_base_url":      "https://assets.staryield.net",
		"storage.max_avatar_bytes":     1 << 20,
		"storage.max_avatar_pixels":    4096 * 4096,
		"storage.avatar_size":          256,

		"payments.provider":                    "sandbox",
		"payments.webhook_secret":              "",
		"payments.webhook_tolerance":           "5m",
		"payments.allow_sandbox_in_production": false,
		"payments.currency":                    "USD",

		"analytics.enabled":          true,
		"analytics.buffer_size":      10000,
		"analytics.batch_size":       200,
		"analytics.flush_interval":   "2s",
		"analytics.ip_hash_secret":   "",
		"analytics.max_client_batch": 50,

		"log.level":  "info",
		"log.format": "json",

		"mail.driver": "log",
		"mail.from":   "Staryield <no-reply@staryield.net>",

		"worker.enabled":          true,
		"worker.tick_interval":    "1s",
		"worker.presence_sweep":   "5s",
		"worker.maintenance_tick": "30s",
		"worker.batch_size":       100,
	}
	for k, val := range d {
		v.SetDefault(k, val)
	}
}

// normalize applies derived values and trims list inputs that may arrive as a
// single comma-separated environment variable.
func (c *Config) normalize() {
	c.App.Env = strings.ToLower(strings.TrimSpace(c.App.Env))
	c.Security.AllowedOrigins = splitList(c.Security.AllowedOrigins)
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
	}
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

// Validate checks invariants and production hardening requirements.
func (c *Config) Validate() error {
	var errs []error
	add := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	switch c.App.Env {
	case EnvDevelopment, EnvTest, EnvProduction:
	default:
		add("app.env must be one of development, test, production (got %q)", c.App.Env)
	}
	if c.Database.DSN == "" {
		add("database.dsn is required")
	}
	if c.Redis.URL == "" {
		add("redis.url is required")
	}
	if len(c.Auth.JWTSecret) < 32 {
		add("auth.jwt_secret must be at least 32 bytes")
	}
	if len(c.Security.CSRFSecret) < 32 {
		add("security.csrf_secret must be at least 32 bytes")
	}
	if c.Auth.AccessTokenTTL <= 0 || c.Auth.AccessTokenTTL > time.Hour {
		add("auth.access_token_ttl must be in (0, 1h]")
	}
	if c.Auth.RefreshTokenTTL < c.Auth.AccessTokenTTL {
		add("auth.refresh_token_ttl must be >= access_token_ttl")
	}
	if c.Auth.Argon2Memory < 8*1024 && c.App.Env != EnvTest {
		add("auth.argon2_memory_kib must be >= 8192 outside tests")
	}
	if c.Auth.Argon2Iterations < 1 || c.Auth.Argon2Parallelism < 1 {
		add("auth.argon2_iterations and argon2_parallelism must be >= 1")
	}
	if c.Billing.PsychicSharePercent < 0 || c.Billing.PsychicSharePercent > 100 {
		add("billing.psychic_share_percent must be within 0..100")
	}
	if c.Chat.BillingInterval <= 0 || c.Chat.RequestTimeout <= 0 || c.Chat.GracePeriod <= 0 || c.Chat.AcceptedTimeout <= 0 {
		add("chat timeouts and billing_interval must be positive")
	}
	if c.Chat.MaxMessageLength < 1 || c.Chat.MaxMessageLength > 10000 {
		add("chat.max_message_length must be within 1..10000")
	}
	if c.Realtime.PingInterval >= c.Realtime.PongWait {
		add("realtime.ping_interval must be shorter than realtime.pong_wait")
	}
	if c.Realtime.SendBuffer < 16 {
		add("realtime.send_buffer must be >= 16")
	}
	for _, origin := range c.Security.AllowedOrigins {
		if origin == "*" {
			add("security.allowed_origins must not contain '*' (credentials are used)")
			continue
		}
		u, err := url.Parse(origin)
		if err != nil || u.Scheme == "" || u.Host == "" || u.Path != "" {
			add("security.allowed_origins entry %q must be scheme://host[:port]", origin)
		}
	}
	for _, cidr := range c.HTTP.TrustedProxies {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			add("http.trusted_proxies entry %q is not a CIDR", cidr)
		}
	}
	switch c.HTTP.IPSource {
	case "direct", "xff", "x-real-ip", "cloudflare":
	default:
		add("http.ip_source must be direct, xff, x-real-ip or cloudflare")
	}
	if (c.HTTP.IPSource == "x-real-ip" || c.HTTP.IPSource == "cloudflare") && len(c.HTTP.TrustedProxies) == 0 {
		add("http.ip_source %s requires http.trusted_proxies", c.HTTP.IPSource)
	}
	if c.Database.ConnectTimeout < 0 || c.Redis.ConnectTimeout < 0 {
		add("database.connect_timeout and redis.connect_timeout must not be negative")
	}
	switch c.Storage.Driver {
	case "memory", "disabled":
	case "r2":
		if c.Storage.R2AccessKeyID == "" || c.Storage.R2SecretKey == "" || c.Storage.R2Bucket == "" || c.Storage.R2Endpoint == "" {
			add("storage driver r2 requires r2_access_key_id, r2_secret_access_key, r2_bucket and r2_account_id/r2_endpoint")
		}
	default:
		add("storage.driver must be r2, memory or disabled")
	}
	switch c.Payments.Provider {
	case "disabled":
	case "sandbox":
		if c.IsProduction() && !c.Payments.AllowSandboxInProduction {
			add("payments.provider sandbox is not allowed in production")
		}
	default:
		add("payments.provider %q is not supported", c.Payments.Provider)
	}
	switch c.Log.Format {
	case "json", "console":
	default:
		add("log.format must be json or console")
	}

	if c.IsProduction() {
		if !c.Security.CookieSecure {
			add("security.cookie_secure must be true in production")
		}
		if len(c.Security.AllowedOrigins) == 0 {
			add("security.allowed_origins must be set in production")
		}
		for _, origin := range c.Security.AllowedOrigins {
			if strings.HasPrefix(origin, "http://") {
				add("security.allowed_origins must use https in production (%q)", origin)
			}
		}
		if strings.HasPrefix(c.Auth.JWTSecret, "dev-only") || strings.HasPrefix(c.Security.CSRFSecret, "dev-only") {
			add("development secrets must not be used in production")
		}
		if len(c.Analytics.IPHashSecret) < 16 {
			add("analytics.ip_hash_secret must be at least 16 bytes in production")
		}
		if len(c.Payments.WebhookSecret) < 16 {
			add("payments.webhook_secret must be at least 16 bytes in production")
		}
		if c.Storage.Driver == "memory" {
			add("storage.driver memory is not allowed in production (use r2, or disabled to turn uploads off)")
		}
		if c.Mail.Driver == "log" {
			add("mail.driver log is not allowed in production (use noop)")
		}
	}
	return errors.Join(errs...)
}
