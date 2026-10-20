package domain

import (
	"testing"
	"time"
)

func TestSessionStateMachine(t *testing.T) {
	allowed := map[SessionStatus][]SessionStatus{
		SessionRequested:    {SessionAccepted, SessionDeclined, SessionExpired, SessionCancelled},
		SessionAccepted:     {SessionActive, SessionExpired, SessionEnded},
		SessionActive:       {SessionDisconnected, SessionEnded},
		SessionDisconnected: {SessionActive, SessionEnded},
		SessionEnded:        {SessionSettled},
	}
	all := []SessionStatus{SessionRequested, SessionAccepted, SessionActive, SessionDisconnected, SessionEnded,
		SessionSettled, SessionDeclined, SessionExpired, SessionCancelled}
	for _, from := range all {
		for _, to := range all {
			want := false
			for _, ok := range allowed[from] {
				if ok == to {
					want = true
				}
			}
			if got := from.CanTransition(to); got != want {
				t.Errorf("%s → %s: got %v, want %v", from, to, got, want)
			}
		}
	}
	// Spec §6: Active cannot jump back to Requested; Settled is terminal.
	if SessionActive.CanTransition(SessionRequested) || !SessionSettled.IsFinal() || !SessionDeclined.IsFinal() {
		t.Fatal("terminal states must be final")
	}
	for _, s := range []SessionStatus{SessionRequested, SessionAccepted, SessionActive, SessionDisconnected} {
		if !s.IsOpen() {
			t.Errorf("%s should be open", s)
		}
	}
	if SessionEnded.IsOpen() || SessionRequested.IsLive() || !SessionDisconnected.IsLive() {
		t.Fatal("open/live classification")
	}
}

func TestStartedMinutes(t *testing.T) {
	// BR-02: each started minute counts; no per-second pro-rating.
	cases := map[time.Duration]int{
		0: 0, time.Second: 1, 59 * time.Second: 1, time.Minute: 1,
		time.Minute + time.Millisecond: 2, 150 * time.Second: 3, -time.Second: 0,
	}
	for d, want := range cases {
		if got := StartedMinutes(d, time.Minute); got != want {
			t.Errorf("StartedMinutes(%v) = %d, want %d", d, got, want)
		}
	}
}

func TestSplitEarnings(t *testing.T) {
	for _, tc := range []struct {
		gross       int64
		share       int
		net, commis int64
	}{
		{100, 60, 60, 40}, {40, 60, 24, 16}, {1, 60, 0, 1}, {0, 60, 0, 0}, {-5, 60, 0, 0}, {99, 100, 99, 0}, {99, 0, 0, 99},
	} {
		net, commission := SplitEarnings(tc.gross, tc.share)
		if net != tc.net || commission != tc.commis || (tc.gross > 0 && net+commission != tc.gross) {
			t.Errorf("SplitEarnings(%d,%d) = %d,%d", tc.gross, tc.share, net, commission)
		}
	}
}

func TestSessionElapsedAndCoverage(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := Session{Status: SessionActive, ActiveSeconds: 30, LastResumedAt: &start, BilledMinutes: 2}
	if got := s.ElapsedActive(start.Add(15 * time.Second)); got != 45*time.Second {
		t.Fatalf("elapsed: %v", got)
	}
	if s.PaidCoverage(time.Minute) != 2*time.Minute {
		t.Fatal("coverage")
	}
	s.Status = SessionDisconnected
	if got := s.ElapsedActive(start.Add(time.Hour)); got != 30*time.Second {
		t.Fatalf("disconnected sessions must not accrue time: %v", got)
	}
}

func TestCustomRequestTransitions(t *testing.T) {
	if !RequestPending.CanTransition(RequestAccepted) || !RequestAccepted.CanTransition(RequestDelivered) ||
		!RequestDelivered.CanTransition(RequestDisputed) || !RequestDisputed.CanTransition(RequestRefunded) {
		t.Fatal("happy path transitions")
	}
	if RequestDelivered.CanTransition(RequestCancelled) || RequestRefunded.CanTransition(RequestPending) {
		t.Fatal("invalid transitions allowed")
	}
	if !RequestPending.HoldsCredits() || !RequestAccepted.HoldsCredits() || RequestDelivered.HoldsCredits() {
		t.Fatal("holding states")
	}
}

func TestJSONTypesRoundTrip(t *testing.T) {
	var l StringList
	if err := l.Scan([]byte(`["a","b"]`)); err != nil || len(l) != 2 {
		t.Fatalf("scan list: %v %v", l, err)
	}
	if v, _ := StringList(nil).Value(); v != "[]" {
		t.Fatalf("nil list value: %v", v)
	}
	var m JSONMap
	if err := m.Scan(`{"k":1}`); err != nil || m["k"].(float64) != 1 {
		t.Fatalf("scan map: %v %v", m, err)
	}
	if err := m.Scan(42); err == nil {
		t.Fatal("unsupported source must error")
	}
	if NewID().Version() != 7 {
		t.Fatal("ids must be UUIDv7")
	}
}
