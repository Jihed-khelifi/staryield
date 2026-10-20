// Package redisx constructs the Redis client and namespaces keys.
package redisx

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/retry"
)

// zapRedisLogger routes go-redis internal logs into zap at debug level
// (they are reconnect/pool notices, not application errors).
type zapRedisLogger struct{ log *zap.Logger }

func (l zapRedisLogger) Printf(_ context.Context, format string, v ...any) {
	l.log.Debug(fmt.Sprintf(format, v...), zap.String("component", "redis"))
}

// SetLogger installs log as the go-redis internal logger.
func SetLogger(log *zap.Logger) { redis.SetLogger(zapRedisLogger{log: log}) }

// New parses the URL, applies pool settings and verifies connectivity,
// retrying with backoff for up to cfg.ConnectTimeout (bounded by ctx).
func New(ctx context.Context, cfg config.RedisConfig, log *zap.Logger) (*redis.Client, error) {
	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("redis: parse url: %w", err)
	}
	log.Info("redis: connecting", zap.String("addr", opts.Addr), zap.Int("db", opts.DB), zap.Bool("tls", opts.TLSConfig != nil))
	opts.DialTimeout = cfg.DialTimeout
	opts.ReadTimeout = cfg.ReadTimeout
	opts.WriteTimeout = cfg.WriteTimeout
	if cfg.PoolSize > 0 {
		opts.PoolSize = cfg.PoolSize
	}
	opts.ContextTimeoutEnabled = true
	client := redis.NewClient(opts)

	err = retry.Until(ctx, cfg.ConnectTimeout, log, "redis", func(ctx context.Context) error {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return client.Ping(pingCtx).Err()
	})
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis: ping %s: %w", opts.Addr, err)
	}
	return client, nil
}

// Keys builds namespaced keys so several environments can share a server.
type Keys struct{ Prefix string }

// K joins the prefix and parts with ':'.
func (k Keys) K(parts ...string) string {
	out := k.Prefix
	for i, p := range parts {
		if i > 0 {
			out += ":"
		}
		out += p
	}
	return out
}
