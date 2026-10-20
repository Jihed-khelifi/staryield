// Package ratelimit implements token-bucket rate limiting backed by Redis
// (shared across instances) with an in-memory implementation for tests and
// single-process use.
package ratelimit

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Rule allows Rate events per Period with bursts up to Burst.
type Rule struct {
	Name   string
	Rate   int
	Period time.Duration
	Burst  int
}

// perMilli returns the refill rate in tokens per millisecond.
func (r Rule) perMilli() float64 {
	if r.Period <= 0 || r.Rate <= 0 {
		return 0
	}
	return float64(r.Rate) / float64(r.Period.Milliseconds())
}

func (r Rule) burst() int {
	if r.Burst <= 0 {
		return max(r.Rate, 1)
	}
	return r.Burst
}

// Scaled returns the rule with rate and burst multiplied by f (tests use a
// large factor to disable throttling without changing code paths).
func (r Rule) Scaled(f float64) Rule {
	if f == 1 || f <= 0 {
		return r
	}
	r.Rate = int(math.Ceil(float64(r.Rate) * f))
	r.Burst = int(math.Ceil(float64(r.burst()) * f))
	return r
}

// Result is the outcome of a limiter check.
type Result struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
}

// Limiter checks and consumes tokens.
type Limiter interface {
	Allow(ctx context.Context, key string, rule Rule) (Result, error)
}

// tokenBucket atomically refills and consumes a bucket stored in a hash.
var tokenBucket = redis.NewScript(`
local rate = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local cost = tonumber(ARGV[4])
local data = redis.call('HMGET', KEYS[1], 't', 'ts')
local tokens = tonumber(data[1])
local ts = tonumber(data[2])
if tokens == nil or ts == nil then
  tokens = burst
  ts = now
end
local elapsed = now - ts
if elapsed < 0 then elapsed = 0 end
tokens = math.min(burst, tokens + elapsed * rate)
local allowed = 0
local retry = 0
if tokens >= cost then
  tokens = tokens - cost
  allowed = 1
else
  retry = math.ceil((cost - tokens) / rate)
end
redis.call('HSET', KEYS[1], 't', tostring(tokens), 'ts', tostring(now))
redis.call('PEXPIRE', KEYS[1], math.ceil(burst / rate) + 1000)
return {allowed, retry, math.floor(tokens)}
`)

// RedisLimiter is a distributed token bucket.
type RedisLimiter struct {
	client *redis.Client
	prefix string
	now    func() time.Time
}

// NewRedisLimiter returns a Redis-backed limiter.
func NewRedisLimiter(client *redis.Client, prefix string) *RedisLimiter {
	return &RedisLimiter{client: client, prefix: prefix, now: time.Now}
}

// Allow implements Limiter.
func (l *RedisLimiter) Allow(ctx context.Context, key string, rule Rule) (Result, error) {
	rate := rule.perMilli()
	if rate == 0 {
		return Result{Allowed: true}, nil
	}
	vals, err := tokenBucket.Run(ctx, l.client, []string{l.prefix + "rl:" + rule.Name + ":" + key},
		rate, rule.burst(), l.now().UnixMilli(), 1).Int64Slice()
	if err != nil {
		return Result{}, err
	}
	return Result{Allowed: vals[0] == 1, RetryAfter: time.Duration(vals[1]) * time.Millisecond, Remaining: int(vals[2])}, nil
}

// MemoryLimiter is an in-process token bucket.
type MemoryLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	now     func() time.Time
}

type bucket struct {
	tokens float64
	ts     time.Time
}

// NewMemoryLimiter returns an in-memory limiter. now may be nil.
func NewMemoryLimiter(now func() time.Time) *MemoryLimiter {
	if now == nil {
		now = time.Now
	}
	return &MemoryLimiter{buckets: map[string]*bucket{}, now: now}
}

// Allow implements Limiter.
func (l *MemoryLimiter) Allow(_ context.Context, key string, rule Rule) (Result, error) {
	rate := rule.perMilli()
	if rate == 0 {
		return Result{Allowed: true}, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	k := rule.Name + ":" + key
	b, ok := l.buckets[k]
	burst := float64(rule.burst())
	if !ok {
		b = &bucket{tokens: burst, ts: now}
		l.buckets[k] = b
	}
	elapsed := float64(now.Sub(b.ts).Milliseconds())
	if elapsed > 0 {
		b.tokens = math.Min(burst, b.tokens+elapsed*rate)
		b.ts = now
	}
	if b.tokens >= 1 {
		b.tokens--
		return Result{Allowed: true, Remaining: int(b.tokens)}, nil
	}
	retry := time.Duration(math.Ceil((1-b.tokens)/rate)) * time.Millisecond
	return Result{Allowed: false, RetryAfter: retry}, nil
}

// Local is a lock-free-for-callers per-connection token bucket used by the
// WebSocket read loop (no shared state needed).
type Local struct {
	rate   float64 // tokens per second
	burst  float64
	tokens float64
	last   time.Time
}

// NewLocal returns a per-connection bucket.
func NewLocal(perSecond float64, burst int) *Local {
	return &Local{rate: perSecond, burst: float64(burst), tokens: float64(burst), last: time.Now()}
}

// Allow consumes one token if available. Not safe for concurrent use.
func (l *Local) Allow(now time.Time) bool {
	if l.rate <= 0 {
		return true
	}
	l.tokens = math.Min(l.burst, l.tokens+now.Sub(l.last).Seconds()*l.rate)
	l.last = now
	if l.tokens >= 1 {
		l.tokens--
		return true
	}
	return false
}
