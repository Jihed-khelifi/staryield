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
		// revisions 3.0: sponsored (free mode) minutes earn the list rate.
		{"sponsored only", Session{Sponsored: true, RatePerMinute: 30, ListRatePerMinute: 30, BilledMinutes: 4, SponsoredMinutes: 4, SponsoredCredits: 120}, 120},
		{"sponsored with a member discount", Session{Sponsored: true, RatePerMinute: 24, ListRatePerMinute: 30, DiscountBps: 2000, BilledMinutes: 3, SponsoredMinutes: 3, SponsoredCredits: 72}, 90},
		{"sponsored adds to paid minutes (no discount)", Session{RatePerMinute: 30, ListRatePerMinute: 30, BilledMinutes: 5, FreeMinutesUsed: 1, SponsoredMinutes: 2, CreditsCharged: 60}, 120},
		{"sponsored adds to discounted paid minutes", Session{RatePerMinute: 24, ListRatePerMinute: 30, DiscountBps: 2000, BilledMinutes: 6, FreeMinutesUsed: 1, SponsoredMinutes: 2, CreditsCharged: 72}, 90 + 60},
		{"no minutes yet", Session{Sponsored: true, RatePerMinute: 30, ListRatePerMinute: 30}, 0},
		{"negative sponsored minutes are ignored", Session{RatePerMinute: 30, ListRatePerMinute: 30, SponsoredMinutes: -3, CreditsCharged: 30}, 30},
	}
	for _, c := range cases {
		if got := c.s.AdvisorGross(); got != c.want {
			t.Errorf("%s: AdvisorGross = %d, want %d", c.name, got, c.want)
		}
	}
}
