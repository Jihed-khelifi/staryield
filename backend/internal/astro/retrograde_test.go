package astro

import (
	"testing"
	"time"
)

func TestRetrogradePeriodsKnownStations(t *testing.T) {
	tests := []struct {
		name       string
		body       Body
		from, to   time.Time
		start, end time.Time
		tolerance  time.Duration
	}{
		{
			name: "Mercury spring 2024", body: Mercury,
			from:      time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
			to:        time.Date(2024, 5, 15, 0, 0, 0, 0, time.UTC),
			start:     time.Date(2024, 4, 1, 22, 14, 0, 0, time.UTC),
			end:       time.Date(2024, 4, 25, 12, 54, 0, 0, time.UTC),
			tolerance: 36 * time.Hour,
		},
		{
			name: "Mars 2024-2025", body: Mars,
			from: time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC),
			to:   time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC),
			// Station in apparent ecliptic longitude interpolated from JPL
			// Horizons (6-hourly values peak at 126.1711° near this time).
			start:     time.Date(2024, 12, 6, 23, 33, 0, 0, time.UTC),
			end:       time.Date(2025, 2, 24, 2, 0, 0, 0, time.UTC),
			tolerance: 36 * time.Hour,
		},
		{
			name: "Venus 2023", body: Venus,
			from:      time.Date(2023, 7, 1, 0, 0, 0, 0, time.UTC),
			to:        time.Date(2023, 10, 1, 0, 0, 0, 0, time.UTC),
			start:     time.Date(2023, 7, 23, 1, 33, 0, 0, time.UTC),
			end:       time.Date(2023, 9, 4, 1, 20, 0, 0, time.UTC),
			tolerance: 36 * time.Hour,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RetrogradePeriods(tt.body, tt.from, tt.to)
			if len(got) != 1 {
				t.Fatalf("got %d periods, want 1: %+v", len(got), got)
			}
			p := got[0]
			if p.Body != string(tt.body) {
				t.Errorf("body = %s", p.Body)
			}
			ds, de := p.Start.Sub(tt.start), p.End.Sub(tt.end)
			if absDuration(ds) > tt.tolerance || absDuration(de) > tt.tolerance {
				t.Errorf("period %s → %s, want %s → %s", p.Start, p.End, tt.start, tt.end)
			}
			t.Logf("%s → %s (station errors %s / %s)", p.Start.Format(time.RFC3339), p.End.Format(time.RFC3339), ds, de)
		})
	}
}

func TestRetrogradePeriodsMercury2024(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	got := RetrogradePeriods(Mercury, from, to)
	// The first period began in December 2023 and overlaps `from`.
	want := []struct{ start, end time.Time }{
		{time.Date(2023, 12, 13, 7, 9, 0, 0, time.UTC), time.Date(2024, 1, 2, 3, 8, 0, 0, time.UTC)},
		{time.Date(2024, 4, 1, 22, 14, 0, 0, time.UTC), time.Date(2024, 4, 25, 12, 54, 0, 0, time.UTC)},
		{time.Date(2024, 8, 5, 4, 56, 0, 0, time.UTC), time.Date(2024, 8, 28, 21, 14, 0, 0, time.UTC)},
		{time.Date(2024, 11, 26, 2, 42, 0, 0, time.UTC), time.Date(2024, 12, 15, 20, 56, 0, 0, time.UTC)},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d periods, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if absDuration(got[i].Start.Sub(w.start)) > 36*time.Hour || absDuration(got[i].End.Sub(w.end)) > 36*time.Hour {
			t.Errorf("period %d = %s → %s, want %s → %s", i, got[i].Start, got[i].End, w.start, w.end)
		}
		if !got[i].Start.Before(got[i].End) {
			t.Errorf("period %d is empty", i)
		}
		mid := got[i].Start.Add(got[i].End.Sub(got[i].Start) / 2)
		if !IsRetrograde(Mercury, mid) {
			t.Errorf("Mercury not retrograde in the middle of period %d", i)
		}
	}
	if !got[0].Start.Before(from) {
		t.Errorf("first period should start before from: %s", got[0].Start)
	}
}

func TestRetrogradePeriodsOuterPlanetSpanningWindow(t *testing.T) {
	// Saturn is retrograde for about 4.5 months; a one-week window inside a
	// retrograde loop must still return the full period.
	from := time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)
	got := RetrogradePeriods(Saturn, from, from.AddDate(0, 0, 7))
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	if !got[0].Start.Before(from) || !got[0].End.After(from.AddDate(0, 0, 7)) {
		t.Errorf("period %s → %s does not span the window", got[0].Start, got[0].End)
	}
	if days := got[0].End.Sub(got[0].Start).Hours() / 24; days < 120 || days > 150 {
		t.Errorf("Saturn retrograde lasted %.0f days", days)
	}
}

func TestRetrogradePeriodsNone(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(1, 0, 0)
	for _, b := range []Body{Sun, Moon, Ascendant, Midheaven} {
		if got := RetrogradePeriods(b, from, to); got != nil {
			t.Errorf("%s: got %+v, want nil", b, got)
		}
	}
	if got := RetrogradePeriods(Mercury, to, from); got != nil {
		t.Errorf("reversed window returned %+v", got)
	}
	// Mercury is direct for this whole window.
	if got := RetrogradePeriods(Mercury, time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC), time.Date(2024, 6, 10, 0, 0, 0, 0, time.UTC)); got != nil {
		t.Errorf("direct window returned %+v", got)
	}
}
