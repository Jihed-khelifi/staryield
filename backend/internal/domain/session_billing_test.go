package domain

import (
	"testing"
	"time"
)

func TestMinuteStartedBoundary(t *testing.T) {
	t0 := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	next := t0.Add(time.Minute)
	active := Session{Status: SessionActive, NextBillingAt: &next}
	cases := []struct {
		name string
		s    Session
		now  time.Time
		want bool
	}{
		{"before the boundary", active, next.Add(-time.Microsecond), false},
		{"exactly at the boundary: a zero-length minute", active, next, false},
		// The database keeps microseconds: below that precision Go and SQL
		// would disagree, so the predicate compares at database precision.
		{"less than the database precision after the boundary", active, next.Add(999 * time.Nanosecond), false},
		{"just after the boundary", active, next.Add(time.Microsecond), true},
		{"long after (catch-up)", active, next.Add(5 * time.Minute), true},
		{"not active", Session{Status: SessionDisconnected, NextBillingAt: &next}, next.Add(time.Second), false},
		{"no next minute scheduled", Session{Status: SessionActive}, next.Add(time.Second), false},
	}
	for _, c := range cases {
		if got := c.s.MinuteStarted(c.now); got != c.want {
			t.Errorf("%s: MinuteStarted = %v, want %v", c.name, got, c.want)
		}
	}
	// Consistent with StartedMinutes: exactly one minute of activity is one
	// started minute; one nanosecond more starts the second.
	if StartedMinutes(time.Minute, time.Minute) != 1 || StartedMinutes(time.Minute+time.Nanosecond, time.Minute) != 2 {
		t.Fatal("StartedMinutes boundary")
	}
}

func TestAdvisorGrossUsesTheListRate(t *testing.T) {
	cases := []struct {
		name string
		s    Session
		want int64
	}{
		{"no discount", Session{RatePerMinute: 30, ListRatePerMinute: 30, BilledMinutes: 5, FreeMinutesUsed: 3, CreditsCharged: 60}, 60},
		{"legacy row without list rate", Session{RatePerMinute: 30, BilledMinutes: 2, CreditsCharged: 60}, 60},
		{"member discount", Session{RatePerMinute: 24, ListRatePerMinute: 30, DiscountBps: 2000, BilledMinutes: 5, FreeMinutesUsed: 3, CreditsCharged: 48}, 60},
		{"free minutes only", Session{RatePerMinute: 24, ListRatePerMinute: 30, DiscountBps: 2000, BilledMinutes: 3, FreeMinutesUsed: 3}, 0},
	}
	for _, c := range cases {
		if got := c.s.AdvisorGross(); got != c.want {
			t.Errorf("%s: AdvisorGross = %d, want %d", c.name, got, c.want)
		}
	}
}
