package cache

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// Taker atomically reads and deletes a key (Redis GETDEL): exactly one of
// several concurrent callers receives the value. It backs single-use
// records such as OpenID Connect sign-in attempts.
type Taker interface {
	GetDel(ctx context.Context, key string) (string, bool, error)
}

// GetDel implements Taker with a single GETDEL.
func (r *Redis) GetDel(ctx context.Context, key string) (string, bool, error) {
	v, err := r.client.GetDel(ctx, r.prefix+key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

// GetDel implements Taker.
func (m *Memory) GetDel(_ context.Context, key string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.live(key)
	if ok {
		delete(m.data, key)
	}
	return e.value, ok, nil
}
