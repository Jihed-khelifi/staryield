// Package retry waits for startup dependencies (PostgreSQL, Redis) with capped
// exponential backoff. Hosted databases may still be booting, restarting or
// waiting on the private network when the API starts, so a failed first
// connection should not crash the process.
package retry

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// Backoff bounds.
const (
	initialDelay = 250 * time.Millisecond
	maxDelay     = 5 * time.Second
)

// Until calls fn until it succeeds, ctx is done or timeout elapses, and
// returns the last error. A non-positive timeout makes a single attempt.
func Until(ctx context.Context, timeout time.Duration, log *zap.Logger, what string, fn func(ctx context.Context) error) error {
	deadline := time.Now().Add(timeout)
	delay := initialDelay
	for attempt := 1; ; attempt++ {
		err := fn(ctx)
		if err == nil {
			if attempt > 1 {
				log.Info(what+" reachable", zap.Int("attempts", attempt))
			}
			return nil
		}
		if ctx.Err() != nil {
			return err
		}
		if timeout <= 0 || time.Now().Add(delay).After(deadline) {
			return fmt.Errorf("%w (gave up after %d attempts)", err, attempt)
		}
		log.Warn(what+" not reachable yet, retrying", zap.Int("attempt", attempt), zap.Duration("backoff", delay), zap.Error(err))
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return err
		case <-t.C:
		}
		delay = min(delay*2, maxDelay)
	}
}
