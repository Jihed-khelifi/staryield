package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestGetDelIsAtomicAndSingleUse(t *testing.T) {
	mr := miniredis.RunT(t)
	now := time.Now()
	for name, s := range map[string]Store{
		"memory": NewMemory(func() time.Time { return now }),
		"redis":  NewRedis(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "t:"),
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			taker, ok := s.(Taker)
			if !ok {
				t.Fatal("the store must implement Taker")
			}
			if _, found, err := taker.GetDel(ctx, "missing"); found || err != nil {
				t.Fatalf("missing: %v %v", found, err)
			}
			_ = s.Set(ctx, "state", "payload", time.Minute)
			if v, found, err := taker.GetDel(ctx, "state"); !found || err != nil || v != "payload" {
				t.Fatalf("first: %q %v %v", v, found, err)
			}
			if _, found, _ := s.Get(ctx, "state"); found {
				t.Fatal("the key must be gone")
			}
			if _, found, _ := taker.GetDel(ctx, "state"); found {
				t.Fatal("second take must find nothing")
			}

			// Many concurrent takers: exactly one wins.
			_ = s.Set(ctx, "race", "x", time.Minute)
			var winners atomic.Int32
			var wg sync.WaitGroup
			for i := 0; i < 32; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if _, found, _ := taker.GetDel(ctx, "race"); found {
						winners.Add(1)
					}
				}()
			}
			wg.Wait()
			if winners.Load() != 1 {
				t.Fatalf("%d takers got the value", winners.Load())
			}
		})
	}
	// An expired in-memory key is not returned.
	mem := NewMemory(func() time.Time { return now })
	_ = mem.Set(context.Background(), "e", "1", time.Second)
	now = now.Add(2 * time.Second)
	if _, found, _ := mem.GetDel(context.Background(), "e"); found {
		t.Fatal("expired key returned")
	}
}
