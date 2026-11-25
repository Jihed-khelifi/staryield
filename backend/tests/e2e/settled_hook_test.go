package e2e

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// OnSessionSettled observers (referral qualification, lifecycle messaging)
// run once per session, after the settlement committed, and never again for
// settlement retries.
func TestSessionSettledHookRunsOncePerSession(t *testing.T) {
	e := newEnv(t)
	var mu sync.Mutex
	calls := map[uuid.UUID]int{}
	var seen domain.Session
	e.app.Services.Chat.OnSessionSettled(func(_ context.Context, s domain.Session) {
		mu.Lock()
		defer mu.Unlock()
		calls[s.ID]++
		seen = s
	})
	client, _, _, _, s := startedSession(t, e, 10, 250)
	e.tick(time.Minute + time.Second) // one billed minute
	endSession(t, client, s.ID)
	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		n := calls[s.ID]
		mu.Unlock()
		if n > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	e.maintenance() // settlement retries find nothing to do
	mu.Lock()
	defer mu.Unlock()
	if calls[s.ID] != 1 {
		t.Fatalf("hook calls for the session: %d", calls[s.ID])
	}
	if seen.Status != domain.SessionSettled || seen.ClientID != client.user.ID || seen.SettledAt == nil {
		t.Fatalf("hook saw %+v", seen)
	}
}
