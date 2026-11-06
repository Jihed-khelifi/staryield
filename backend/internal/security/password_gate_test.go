package security

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The hasher admits a bounded number of Argon2 computations; callers that
// cannot get a slot in time get ErrBusy instead of piling up memory.
func TestPasswordHasherBoundsConcurrentHashing(t *testing.T) {
	h := NewPasswordHasher(8*1024, 1, 1).WithLimits(1, 40*time.Millisecond)
	hash, err := h.Hash("Correct!Horse1")
	if err != nil {
		t.Fatal(err)
	}

	release, err := h.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := h.VerifyContext(context.Background(), "Correct!Horse1", hash); !errors.Is(err, ErrBusy) {
		t.Fatalf("verify while saturated: %v, want ErrBusy", err)
	}
	if waited := time.Since(start); waited < 30*time.Millisecond || waited > time.Second {
		t.Fatalf("waited %v, want about the 40ms limit", waited)
	}
	if _, err := h.HashContext(context.Background(), "x"); !errors.Is(err, ErrBusy) {
		t.Fatalf("hash while saturated: %v, want ErrBusy", err)
	}
	if err := h.VerifyDummyContext(context.Background(), "x"); !errors.Is(err, ErrBusy) {
		t.Fatalf("dummy verify while saturated: %v, want ErrBusy (same answer as a real account)", err)
	}
	// A caller whose request is already over gives up at once.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.VerifyContext(ctx, "Correct!Horse1", hash); !errors.Is(err, ErrBusy) || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled caller: %v", err)
	}
	// ErrBusy is never a password verdict.
	if errors.Is(ErrBusy, ErrMismatch) {
		t.Fatal("ErrBusy must not look like a mismatch")
	}

	release()
	if _, err := h.VerifyContext(context.Background(), "Correct!Horse1", hash); err != nil {
		t.Fatalf("verify after release: %v", err)
	}
	if err := h.VerifyDummyContext(context.Background(), "x"); err != nil {
		t.Fatalf("dummy verify after release: %v", err)
	}
}

func TestPasswordHasherNeverRunsMoreThanItsLimit(t *testing.T) {
	const limit = 2
	h := NewPasswordHasher(8*1024, 1, 1).WithLimits(limit, 5*time.Second)
	hash, err := h.Hash("Correct!Horse1")
	if err != nil {
		t.Fatal(err)
	}
	var running, peak atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := h.Acquire(context.Background())
			if err != nil {
				t.Error(err)
				return
			}
			n := running.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			running.Add(-1)
			release()
		}()
	}
	wg.Wait()
	if p := peak.Load(); p > limit {
		t.Fatalf("peak concurrency %d, limit %d", p, limit)
	}
	// Waiting callers are admitted as slots free up.
	if _, err := h.Verify("Correct!Horse1", hash); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestPasswordHasherDefaultLimits(t *testing.T) {
	h := NewPasswordHasher(8*1024, 1, 1)
	if cap(h.slots) < 2 || h.maxWait != DefaultHashWait {
		t.Fatalf("defaults: %d slots, wait %v", cap(h.slots), h.maxWait)
	}
}
