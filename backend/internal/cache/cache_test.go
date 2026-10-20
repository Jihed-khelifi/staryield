package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestStores(t *testing.T) {
	mr := miniredis.RunT(t)
	now := time.Now()
	mem := NewMemory(func() time.Time { return now })
	for name, s := range map[string]Store{"memory": mem, "redis": NewRedis(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "t:")} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if _, ok, _ := s.Get(ctx, "missing"); ok {
				t.Fatal("missing key found")
			}
			_ = s.Set(ctx, "k", "v", time.Minute)
			if v, ok, _ := s.Get(ctx, "k"); !ok || v != "v" {
				t.Fatalf("get: %q %v", v, ok)
			}
			for i := 1; i <= 3; i++ {
				if n, _ := s.Incr(ctx, "c", time.Minute); n != int64(i) {
					t.Fatalf("incr %d: %d", i, n)
				}
			}
			if ttl, _ := s.TTL(ctx, "c"); ttl <= 0 || ttl > time.Minute {
				t.Fatalf("ttl: %v", ttl)
			}
			_ = s.Del(ctx, "k", "c")
			if _, ok, _ := s.Get(ctx, "k"); ok {
				t.Fatal("deleted key found")
			}
		})
	}
	_ = mem.Set(context.Background(), "e", "1", time.Second)
	now = now.Add(2 * time.Second)
	if _, ok, _ := mem.Get(context.Background(), "e"); ok {
		t.Fatal("expired key returned")
	}
}
