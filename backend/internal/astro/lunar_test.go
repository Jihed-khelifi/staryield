package astro

import (
	"testing"
	"time"
)

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func TestNextLunarEventsKnownPhases(t *testing.T) {
	const tolerance = 3 * time.Hour
	tests := []struct {
		name string
		kind string
		want time.Time
	}{
		{"full moon 2024-01-25", FullMoonEvent, time.Date(2024, 1, 25, 17, 54, 0, 0, time.UTC)},
		{"full moon 2024-02-24", FullMoonEvent, time.Date(2024, 2, 24, 12, 30, 0, 0, time.UTC)},
		{"full moon 2025-03-14 (lunar eclipse)", FullMoonEvent, time.Date(2025, 3, 14, 6, 55, 0, 0, time.UTC)},
		{"new moon 2024-04-08 (solar eclipse)", NewMoonEvent, time.Date(2024, 4, 8, 18, 21, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Search from five days earlier; the event must be among the next
			// principal phases found.
			events := NextLunarEvents(tt.want.Add(-5*24*time.Hour), 2)
			var found *LunarEvent
			for i := range events {
				if events[i].Kind == tt.kind {
					found = &events[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("no %s in %+v", tt.kind, events)
			}
			diff := found.At.Sub(tt.want)
			if absDuration(diff) > tolerance {
				t.Errorf("%s at %s, want %s (off by %s)", tt.kind, found.At, tt.want, diff)
			}
			t.Logf("%s at %s (off by %s), Moon in %s", tt.kind, found.At.Format(time.RFC3339), diff, found.Sign)
		})
	}
}

func TestNextLunarEventsSequence(t *testing.T) {
	// January 2024 principal phases (UTC).
	want := []LunarEvent{
		{LastQuarterEvent, time.Date(2024, 1, 4, 3, 30, 0, 0, time.UTC), "Libra"},
		{NewMoonEvent, time.Date(2024, 1, 11, 11, 57, 0, 0, time.UTC), "Capricorn"},
		{FirstQuarterEvent, time.Date(2024, 1, 18, 3, 52, 0, 0, time.UTC), "Aries"},
		{FullMoonEvent, time.Date(2024, 1, 25, 17, 54, 0, 0, time.UTC), "Leo"},
		{LastQuarterEvent, time.Date(2024, 2, 2, 23, 18, 0, 0, time.UTC), "Scorpio"},
	}
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	got := NextLunarEvents(from, len(want))
	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Kind != want[i].Kind {
			t.Errorf("event %d kind = %s, want %s", i, got[i].Kind, want[i].Kind)
		}
		if d := absDuration(got[i].At.Sub(want[i].At)); d > 3*time.Hour {
			t.Errorf("event %d at %s, want %s", i, got[i].At, want[i].At)
		}
		if got[i].Sign != want[i].Sign {
			t.Errorf("event %d sign = %s, want %s", i, got[i].Sign, want[i].Sign)
		}
		if !got[i].At.After(from) || (i > 0 && !got[i].At.After(got[i-1].At)) {
			t.Errorf("event %d at %s is not strictly increasing after %s", i, got[i].At, from)
		}
		if got[i].At.Location() != time.UTC {
			t.Errorf("event %d not in UTC", i)
		}
	}
}

func TestNextLunarEventsEdgeCases(t *testing.T) {
	if got := NextLunarEvents(time.Now(), 0); got != nil {
		t.Errorf("n=0 returned %v", got)
	}
	if got := NextLunarEvents(time.Now(), -3); got != nil {
		t.Errorf("n<0 returned %v", got)
	}
	// Starting exactly at an event returns the following one, not the same.
	first := NextLunarEvents(time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC), 1)[0]
	next := NextLunarEvents(first.At, 1)[0]
	if next.At.Sub(first.At) < 5*24*time.Hour || next.Kind == first.Kind {
		t.Errorf("starting at %s %s gave %s %s", first.Kind, first.At, next.Kind, next.At)
	}
	// A long run keeps the strict quarter cycle.
	events := NextLunarEvents(time.Date(2030, 6, 1, 0, 0, 0, 0, time.UTC), 50)
	for i := 1; i < len(events); i++ {
		gap := events[i].At.Sub(events[i-1].At)
		if gap < 5*24*time.Hour || gap > 9*24*time.Hour {
			t.Errorf("gap %s between %s and %s", gap, events[i-1].At, events[i].At)
		}
		if lunarKindIndex(events[i].Kind) != (lunarKindIndex(events[i-1].Kind)+1)%4 {
			t.Errorf("phase order broken at %d: %s after %s", i, events[i].Kind, events[i-1].Kind)
		}
	}
}

func lunarKindIndex(kind string) int {
	for i, k := range lunarEventKinds {
		if k == kind {
			return i
		}
	}
	return -1
}

func TestPhaseNameAndIllumination(t *testing.T) {
	full := time.Date(2024, 1, 25, 17, 54, 0, 0, time.UTC)
	newMoon := time.Date(2024, 4, 8, 18, 21, 0, 0, time.UTC)
	tests := []struct {
		at        time.Time
		want      string
		illumMin  float64
		illumMax  float64
		angleNear float64
	}{
		{newMoon, PhaseNewMoon, 0, 0.01, 0},
		{newMoon.Add(4 * 24 * time.Hour), PhaseWaxingCrescent, 0.1, 0.4, 49},
		{time.Date(2024, 1, 18, 3, 52, 0, 0, time.UTC), PhaseFirstQuarter, 0.45, 0.55, 90},
		{full.Add(-4 * 24 * time.Hour), PhaseWaxingGibbous, 0.7, 0.95, 131},
		{full, PhaseFullMoon, 0.99, 1, 180},
		{full.Add(4 * 24 * time.Hour), PhaseWaningGibbous, 0.7, 0.95, 229},
		{time.Date(2024, 1, 4, 3, 30, 0, 0, time.UTC), PhaseLastQuarter, 0.45, 0.55, 270},
		{newMoon.Add(-4 * 24 * time.Hour), PhaseWaningCrescent, 0.1, 0.4, 311},
	}
	for _, tt := range tests {
		if got := PhaseName(tt.at); got != tt.want {
			t.Errorf("PhaseName(%s) = %s, want %s (angle %.1f°)", tt.at, got, tt.want, MoonPhaseAngle(tt.at))
		}
		if il := Illumination(tt.at); il < tt.illumMin || il > tt.illumMax {
			t.Errorf("Illumination(%s) = %.3f, want [%.2f, %.2f]", tt.at, il, tt.illumMin, tt.illumMax)
		}
		if a := MoonPhaseAngle(tt.at); angleDiff(a, tt.angleNear) > 10 || a < 0 || a >= 360 {
			t.Errorf("MoonPhaseAngle(%s) = %.2f, want about %.0f", tt.at, a, tt.angleNear)
		}
	}
}

func TestPhaseNameDailyCoverage(t *testing.T) {
	// Sampling once a day at noon, every principal phase name must appear in
	// every lunar cycle.
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	counts := map[string]int{}
	const days = 366
	for i := range days {
		counts[PhaseName(start.AddDate(0, 0, i))]++
	}
	for _, name := range []string{PhaseNewMoon, PhaseFirstQuarter, PhaseFullMoon, PhaseLastQuarter} {
		if counts[name] < 12 {
			t.Errorf("%s appeared on %d days in 2024, want at least 12", name, counts[name])
		}
	}
	for _, name := range PhaseNames {
		if counts[name] == 0 {
			t.Errorf("%s never appeared", name)
		}
	}
}
