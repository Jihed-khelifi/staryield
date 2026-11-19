package domain

import "testing"

// revisions 6.0 (§26.1): the held status and the hold's own lifecycle.

func TestHeldSessionTransitions(t *testing.T) {
	for _, tc := range []struct {
		from, to SessionStatus
		want     bool
	}{
		{SessionActive, SessionHeld, true},
		{SessionHeld, SessionActive, true},
		{SessionHeld, SessionDisconnected, true},
		{SessionHeld, SessionEnded, true},
		// Not allowed: a hold is entered only from a running reading and never
		// jumps to a final state other than Ended.
		{SessionHeld, SessionSettled, false},
		{SessionHeld, SessionExpired, false},
		{SessionHeld, SessionHeld, false},
		{SessionDisconnected, SessionHeld, false},
		{SessionAccepted, SessionHeld, false},
		{SessionRequested, SessionHeld, false},
		{SessionEnded, SessionHeld, false},
	} {
		if got := tc.from.CanTransition(tc.to); got != tc.want {
			t.Errorf("%s -> %s: %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}

func TestHeldSessionIsOpenAndLive(t *testing.T) {
	if !SessionHeld.IsOpen() || !SessionHeld.IsLive() || SessionHeld.IsFinal() {
		t.Fatal("a held reading is open, live and not final")
	}
	found := false
	for _, st := range OpenSessionStatuses {
		found = found || st == SessionHeld
	}
	if !found {
		t.Fatal("OpenSessionStatuses must list held (the SQL filters of busy clients and advisors)")
	}
	// Every open status is open, and every other status is not.
	listed := map[SessionStatus]bool{}
	for _, st := range OpenSessionStatuses {
		listed[st] = true
		if !st.IsOpen() {
			t.Errorf("%s is listed as open but IsOpen says no", st)
		}
	}
	for _, st := range []SessionStatus{SessionEnded, SessionSettled, SessionDeclined, SessionExpired, SessionCancelled} {
		if st.IsOpen() || listed[st] {
			t.Errorf("%s must not be open", st)
		}
	}
}

func TestHoldStatusLifecycle(t *testing.T) {
	for _, tc := range []struct {
		from, to HoldStatus
		want     bool
	}{
		{HoldOpen, HoldConverted, true},
		{HoldOpen, HoldExpired, true},
		{HoldExpired, HoldRevealed, true},
		{HoldOpen, HoldRevealed, false}, // top up instead
		{HoldConverted, HoldRevealed, false},
		{HoldConverted, HoldExpired, false},
		{HoldRevealed, HoldExpired, false},
		{HoldExpired, HoldConverted, false}, // the reading ended: a late top-up cannot resurrect it
	} {
		if got := tc.from.CanTransition(tc.to); got != tc.want {
			t.Errorf("%s -> %s: %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
	if HoldOpen.IsFinal() || HoldExpired.IsFinal() || !HoldConverted.IsFinal() || !HoldRevealed.IsFinal() {
		t.Fatal("finality")
	}
}

func TestHoldReason(t *testing.T) {
	for _, tc := range []struct {
		free    int
		credits int64
		trial   bool
	}{
		{1, 0, true},   // only free minutes so far
		{3, 0, true},   // several free minutes
		{1, 40, false}, // a paid minute was billed
		{0, 40, false},
		{0, 0, false}, // nothing billed at all is not "the trial ended"
	} {
		if got := TrialEnded(tc.free, tc.credits); got != tc.trial {
			t.Errorf("TrialEnded(%d, %d) = %v", tc.free, tc.credits, got)
		}
		want := HoldCreditsExhausted
		if tc.trial {
			want = HoldTrialEnded
		}
		if got := HoldReasonFor(tc.free, tc.credits); got != want {
			t.Errorf("HoldReasonFor(%d, %d) = %s, want %s", tc.free, tc.credits, got, want)
		}
	}
}
