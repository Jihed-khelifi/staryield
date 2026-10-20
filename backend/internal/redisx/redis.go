// Package redisx constructs the Redis client and namespaces keys.
package redisx

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
)

// New parses the URL, applies pool settings and verifies connectivity.
func New(ctx context.Context, cfg config.RedisConfig) (*redis.Client, error) {
	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("redis: parse url: %w", err)
	}
	opts.DialTimeout = cfg.DialTimeout
	opts.ReadTimeout = cfg.ReadTimeout
	opts.WriteTimeout = cfg.WriteTimeout
	if cfg.PoolSize > 0 {
		opts.PoolSize = cfg.PoolSize
	}
	opts.ContextTimeoutEnabled = true
	client := redis.NewClient(opts)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis: ping: %w", err)
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
