package retry

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestUntilRetriesUntilSuccess(t *testing.T) {
	calls := 0
	err := Until(context.Background(), 5*time.Second, zap.NewNop(), "dep", func(context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("connection refused")
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestUntilGivesUpAtTimeout(t *testing.T) {
	start := time.Now()
	calls := 0
	err := Until(context.Background(), 600*time.Millisecond, zap.NewNop(), "dep", func(context.Context) error {
		calls++
		return errors.New("connection refused")
	})
	if err == nil || !strings.Contains(err.Error(), "connection refused") || !strings.Contains(err.Error(), "gave up") {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls < 2 || time.Since(start) > 2*time.Second {
		t.Fatalf("calls=%d elapsed=%s", calls, time.Since(start))
	}
}

func TestUntilSingleAttemptWithoutTimeout(t *testing.T) {
	calls := 0
	_ = Until(context.Background(), 0, zap.NewNop(), "dep", func(context.Context) error { calls++; return errors.New("x") })
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestUntilStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(100 * time.Millisecond); cancel() }()
	start := time.Now()
	err := Until(ctx, time.Minute, zap.NewNop(), "dep", func(context.Context) error { return errors.New("down") })
	if err == nil || time.Since(start) > 2*time.Second {
		t.Fatalf("err=%v elapsed=%s", err, time.Since(start))
	}
}
