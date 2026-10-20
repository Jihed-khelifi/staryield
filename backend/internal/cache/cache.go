// Package cache is a minimal key/value abstraction over Redis (shared across
// instances) with an in-memory implementation for tests.
package cache

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Store is a TTL key/value store with atomic counters.
type Store interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
	// Incr increments key, setting ttl when the key is created.
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
}

// Redis implements Store.
type Redis struct {
	client *redis.Client
	prefix string
}

// NewRedis returns a Redis store with a key prefix.
func NewRedis(client *redis.Client, prefix string) *Redis {
	return &Redis{client: client, prefix: prefix}
}

var incrScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
return n
`)

func (r *Redis) Get(ctx context.Context, key string) (string, bool, error) {
	v, err := r.client.Get(ctx, r.prefix+key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	return v, err == nil, err
}

func (r *Redis) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.client.Set(ctx, r.prefix+key, value, ttl).Err()
}

func (r *Redis) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	full := make([]string, len(keys))
	for i, k := range keys {
		full[i] = r.prefix + k
	}
	return r.client.Del(ctx, full...).Err()
}

func (r *Redis) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	return incrScript.Run(ctx, r.client, []string{r.prefix + key}, ttl.Milliseconds()).Int64()
}

func (r *Redis) TTL(ctx context.Context, key string) (time.Duration, error) {
	d, err := r.client.PTTL(ctx, r.prefix+key).Result()
	if err != nil || d < 0 {
		return 0, err
	}
	return d, nil
}

// Memory implements Store in-process.
type Memory struct {
	mu   sync.Mutex
	data map[string]entry
	now  func() time.Time
}

type entry struct {
	value   string
	expires time.Time
}

// NewMemory returns an in-memory store.
func NewMemory(now func() time.Time) *Memory {
	if now == nil {
		now = time.Now
	}
	return &Memory{data: map[string]entry{}, now: now}
}

func (m *Memory) live(key string) (entry, bool) {
	e, ok := m.data[key]
	if ok && !e.expires.IsZero() && !m.now().Before(e.expires) {
		delete(m.data, key)
		return entry{}, false
	}
	return e, ok
}

func (m *Memory) Get(_ context.Context, key string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.live(key)
	return e.value, ok, nil
}

func (m *Memory) Set(_ context.Context, key, value string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := entry{value: value}
	if ttl > 0 {
		e.expires = m.now().Add(ttl)
	}
	m.data[key] = e
	return nil
}

func (m *Memory) Del(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		delete(m.data, k)
	}
	return nil
}

func (m *Memory) Incr(_ context.Context, key string, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.live(key)
	var n int64
	if ok {
		for _, c := range e.value {
			n = n*10 + int64(c-'0')
		}
	} else if ttl > 0 {
		e.expires = m.now().Add(ttl)
	}
	n++
	e.value = itoa(n)
	m.data[key] = e
	return n, nil
}

func (m *Memory) TTL(_ context.Context, key string) (time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.live(key)
	if !ok || e.expires.IsZero() {
		return 0, nil
	}
	return e.expires.Sub(m.now()), nil
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
