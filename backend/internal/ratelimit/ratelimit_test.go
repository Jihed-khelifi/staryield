package ratelimit

import (
	"context"
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
