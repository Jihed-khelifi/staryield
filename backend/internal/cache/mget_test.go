package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestMGet(t *testing.T) {
	mr := miniredis.RunT(t)
	now := time.Now()
	mem := NewMemory(func() time.Time { return now })
	for name, s := range map[string]Store{"memory": mem, "redis": NewRedis(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "t:")} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			mg, ok := s.(MultiGetter)
			if !ok {
				t.Fatal("store does not implement MultiGetter")
			}
			_ = s.Set(ctx, "a", "1", time.Minute)
			_ = s.Set(ctx, "c", "3", time.Minute)
			vals, found, err := mg.MGet(ctx, "a", "b", "c")
			if err != nil || len(vals) != 3 || !found[0] || found[1] || !found[2] || vals[0] != "1" || vals[2] != "3" {
				t.Fatalf("mget: %v %v %v", vals, found, err)
			}
			if vals, found, err := mg.MGet(ctx); err != nil || len(vals) != 0 || len(found) != 0 {
				t.Fatalf("empty mget: %v %v %v", vals, found, err)
			}
		})
	}
	_ = mem.Set(context.Background(), "e", "1", time.Second)
	now = now.Add(2 * time.Second)
	if _, found, _ := mem.MGet(context.Background(), "e"); found[0] {
		t.Fatal("expired key returned")
	}
}
