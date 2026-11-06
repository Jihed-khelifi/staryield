package ratelimit

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func testLimiter(t *testing.T, l Limiter, advance func(time.Duration)) {
	t.Helper()
	ctx := context.Background()
	rule := Rule{Name: "test", Rate: 2, Period: time.Second, Burst: 3}
	for i := 0; i < 3; i++ {
		if res, err := l.Allow(ctx, "k", rule); err != nil || !res.Allowed {
			t.Fatalf("burst request %d rejected: %+v %v", i, res, err)
		}
	}
	res, err := l.Allow(ctx, "k", rule)
	if err != nil || res.Allowed || res.RetryAfter <= 0 || res.RetryAfter > time.Second {
		t.Fatalf("expected throttle with retry-after: %+v %v", res, err)
	}
	// Other keys are independent.
	if res, _ := l.Allow(ctx, "other", rule); !res.Allowed {
		t.Fatal("keys must be independent")
	}
	advance(600 * time.Millisecond) // refills 1.2 tokens
	if res, _ := l.Allow(ctx, "k", rule); !res.Allowed {
		t.Fatal("refill did not happen")
	}
}

func TestMemoryLimiter(t *testing.T) {
	now := time.Now()
	l := NewMemoryLimiter(func() time.Time { return now })
	testLimiter(t, l, func(d time.Duration) { now = now.Add(d) })
}

func TestRedisLimiter(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	l := NewRedisLimiter(client, "t:")
	now := time.Now()
	l.now = func() time.Time { return now }
	testLimiter(t, l, func(d time.Duration) { now = now.Add(d) })

	mr.Close()
	if _, err := l.Allow(context.Background(), "k", Rule{Name: "x", Rate: 1, Period: time.Second}); err == nil {
		t.Fatal("expected an error when Redis is down")
	}
}

func TestZeroRuleAllows(t *testing.T) {
	l := NewMemoryLimiter(nil)
	if res, _ := l.Allow(context.Background(), "k", Rule{Name: "off"}); !res.Allowed {
		t.Fatal("disabled rule must allow")
	}
}

func TestScaled(t *testing.T) {
	r := Rule{Rate: 10, Burst: 5, Period: time.Minute}.Scaled(10)
	if r.Rate != 100 || r.Burst != 50 {
		t.Fatalf("scaled: %+v", r)
	}
	if (Rule{Rate: 3}).Scaled(1).Rate != 3 {
		t.Fatal("scale 1 must be identity")
	}
}

func TestLocalBucket(t *testing.T) {
	l := NewLocal(10, 2)
	now := time.Now()
	if !l.Allow(now) || !l.Allow(now) || l.Allow(now) {
		t.Fatal("burst of 2")
	}
	if !l.Allow(now.Add(150 * time.Millisecond)) {
		t.Fatal("refill at 10/s")
	}
}

// failing is a limiter whose backend is down.
type failing struct{ calls int }

func (f *failing) Allow(context.Context, string, Rule) (Result, error) {
	f.calls++
	return Result{}, errors.New("redis: connection refused")
}

func TestWithFallbackUsesFallbackOnlyWhenPrimaryFails(t *testing.T) {
	ctx := context.Background()
	rule := Rule{Name: "astro", Rate: 1, Period: time.Minute, Burst: 2}

	now := time.Now()
	healthy := NewMemoryLimiter(func() time.Time { return now })
	spare := NewMemoryLimiter(func() time.Time { return now })
	l := WithFallback(healthy, spare)
	for i := 0; i < 2; i++ {
		if res, err := l.Allow(ctx, "ip:1", rule); err != nil || !res.Allowed {
			t.Fatalf("request %d: %+v %v", i, res, err)
		}
	}
	if res, _ := l.Allow(ctx, "ip:1", rule); res.Allowed {
		t.Fatal("the healthy primary's decision must stand")
	}
	if len(spare.buckets) != 0 {
		t.Fatal("the fallback must not be consulted while the primary works")
	}

	down := &failing{}
	l = WithFallback(down, spare)
	for i := 0; i < 2; i++ {
		if res, err := l.Allow(ctx, "ip:2", rule); err != nil || !res.Allowed {
			t.Fatalf("fallback request %d: %+v %v", i, res, err)
		}
	}
	res, err := l.Allow(ctx, "ip:2", rule)
	if err != nil || res.Allowed || res.RetryAfter <= 0 {
		t.Fatalf("the fallback must keep limiting (not fail open): %+v %v", res, err)
	}
	if down.calls != 3 {
		t.Fatalf("the primary must be retried on every request, got %d calls", down.calls)
	}

	if _, err := WithFallback(&failing{}, &failing{}).Allow(ctx, "k", rule); err == nil {
		t.Fatal("an error is returned when both limiters fail")
	}
}

func TestMemoryLimiterDropsIdleBuckets(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	l := NewMemoryLimiter(func() time.Time { return now })
	l.swept = now
	rule := Rule{Name: "r", Rate: 60, Period: time.Minute, Burst: 2} // 1 token/s
	for i := 0; i < 1000; i++ {
		_, _ = l.Allow(ctx, strconv.Itoa(i), rule)
	}
	_, _ = l.Allow(ctx, "busy", rule)
	_, _ = l.Allow(ctx, "busy", rule)
	if len(l.buckets) != 1001 {
		t.Fatalf("buckets: %d", len(l.buckets))
	}
	// Every bucket refills within 2s, so after a sweep interval all are
	// full again and dropped; a dropped bucket behaves like a full one.
	now = now.Add(sweepEvery)
	if res, _ := l.Allow(ctx, "busy", rule); !res.Allowed || res.Remaining != 1 {
		t.Fatalf("a dropped bucket behaves like a full one: %+v", res)
	}
	if len(l.buckets) != 1 {
		t.Fatalf("idle buckets must be dropped, %d left", len(l.buckets))
	}
	// A bucket that is still refilling survives the sweep.
	l.buckets["r:slow"] = &bucket{tokens: 0, ts: now, full: now.Add(time.Hour)}
	now = now.Add(sweepEvery)
	_, _ = l.Allow(ctx, "other", rule)
	if _, ok := l.buckets["r:slow"]; !ok {
		t.Fatal("a bucket that is not yet full must be kept")
	}
	if _, ok := l.buckets["r:busy"]; ok {
		t.Fatal("the idle busy bucket must be dropped")
	}
}
