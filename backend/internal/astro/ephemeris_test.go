package astro

import (
	"math"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/astro/tz"
)

// Precision against JPL Horizons is tested in package ephem; these tests
// cover the astro wrappers and astrologically meaningful moments.

func TestSignIngresses(t *testing.T) {
	tests := []struct {
		name string
		body Body
		at   time.Time
		want float64
		tol  float64
	}{
		// Published ingress times (UTC); the body sits on a sign boundary.
		{"March equinox 2024", Sun, time.Date(2024, 3, 20, 3, 6, 0, 0, time.UTC), 0, 0.001},
		{"June solstice 2024", Sun, time.Date(2024, 6, 20, 20, 51, 0, 0, time.UTC), 90, 0.001},
		{"Pluto enters Aquarius 2024", Pluto, time.Date(2024, 11, 19, 20, 30, 0, 0, time.UTC), 300, 0.01},
		{"Saturn enters Aries 2025", Saturn, time.Date(2025, 5, 25, 3, 35, 0, 0, time.UTC), 0, 0.01},
		{"Uranus enters Gemini 2025", Uranus, time.Date(2025, 7, 7, 7, 45, 0, 0, time.UTC), 60, 0.01},
	}
	for _, tt := range tests {
		if got := EclipticLongitude(tt.body, tt.at); angleDiff(got, tt.want) > tt.tol {
			t.Errorf("%s: %.4f°, want %.4f° ± %g°", tt.name, got, tt.want, tt.tol)
		}
	}
}

func TestEclipticLongitudeNonPlanet(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, b := range []Body{Ascendant, Midheaven, Body("Chiron")} {
		if got := EclipticLongitude(b, now); !math.IsNaN(got) {
			t.Errorf("EclipticLongitude(%s) = %v, want NaN", b, got)
		}
	}
	if !math.IsNaN(EclipticLongitude(Sun, time.Date(1500, 1, 1, 0, 0, 0, 0, time.UTC))) {
		t.Error("an instant outside the ephemeris range must give NaN")
	}
}

func TestIsRetrograde(t *testing.T) {
	tests := []struct {
		body Body
		at   time.Time
		want bool
	}{
		{Mercury, time.Date(2024, 4, 10, 0, 0, 0, 0, time.UTC), true},
		{Mercury, time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC), false},
		{Mars, time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC), true},
		{Mars, time.Date(2025, 4, 15, 0, 0, 0, 0, time.UTC), false},
		{Sun, time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC), false},
		{Moon, time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC), false},
		{Ascendant, time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC), false},
	}
	for _, tt := range tests {
		if got := IsRetrograde(tt.body, tt.at); got != tt.want {
			t.Errorf("IsRetrograde(%s, %s) = %v, want %v", tt.body, tt.at.Format(time.DateOnly), got, tt.want)
		}
	}
}

func TestNorm360AndWrap180(t *testing.T) {
	tests := []struct{ in, norm, wrap float64 }{
		{0, 0, 0}, {360, 0, 0}, {-1, 359, -1}, {725, 5, 5}, {180, 180, 180}, {181, 181, -179}, {-180, 180, 180},
	}
	for _, tt := range tests {
		if got := norm360(tt.in); math.Abs(got-tt.norm) > 1e-9 {
			t.Errorf("norm360(%v) = %v, want %v", tt.in, got, tt.norm)
		}
		if got := wrap180(tt.in); math.Abs(got-tt.wrap) > 1e-9 {
			t.Errorf("wrap180(%v) = %v, want %v", tt.in, got, tt.wrap)
		}
	}
}

// TestSunSignCusps checks the canonical sun sign on ingress days: the sign
// depends on the actual instant, and births on such a day without a time
// are flagged uncertain.
func TestSunSignCusps(t *testing.T) {
	// The Sun entered Taurus on 2024-04-19 at 14:00 UTC.
	ingress := SunIngress(1, 2024)
	if want := time.Date(2024, 4, 19, 14, 0, 0, 0, time.UTC); ingress.Sub(want).Abs() > 2*time.Minute {
		t.Fatalf("Taurus ingress 2024 = %s, want ≈ %s", ingress, want)
	}
	if got := SunSignAt(ingress.Add(-time.Minute)); got != "Aries" {
		t.Errorf("a minute before the ingress: %s", got)
	}
	if got := SunSignAt(ingress.Add(time.Minute)); got != "Taurus" {
		t.Errorf("a minute after the ingress: %s", got)
	}
	ny, err := tz.Load("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2024, 4, 19, 0, 0, 0, 0, time.UTC)
	// Exact time and place: certain, and on either side of the ingress.
	at, s, e := BirthWindow(day, 9, 0, true, ny) // 13:00 UTC, before 14:00
	if got := SunSignForBirth(at, s, e); got.Sign != "Aries" || got.Uncertain {
		t.Errorf("09:00 New York: %+v", got)
	}
	at, s, e = BirthWindow(day, 11, 0, true, ny) // 15:00 UTC
	if got := SunSignForBirth(at, s, e); got.Sign != "Taurus" || got.Uncertain {
		t.Errorf("11:00 New York: %+v", got)
	}
	// Place without time: the local day spans the ingress.
	at, s, e = BirthWindow(day, 0, 0, false, ny)
	if got := SunSignForBirth(at, s, e); !got.Uncertain || len(got.Alternatives) != 2 || got.Alternatives[0] != "Aries" || got.Alternatives[1] != "Taurus" {
		t.Errorf("unknown time: %+v", got)
	}
	// Neither time nor place: every zone's version of the day.
	at, s, e = BirthWindow(day.AddDate(0, 0, 1), 0, 0, false, nil) // 20 April
	if got := SunSignForBirth(at, s, e); !got.Uncertain {
		t.Errorf("20 April without a place should be uncertain (it starts at 10:00 UTC on the 19th): %+v", got)
	}
	at, s, e = BirthWindow(day.AddDate(0, 0, 5), 0, 0, false, nil)
	if got := SunSignForBirth(at, s, e); got.Uncertain || got.Sign != "Taurus" {
		t.Errorf("24 April: %+v", got)
	}
}

func TestTypicalSunSignAndDates(t *testing.T) {
	tests := map[[2]int]string{{3, 25}: "Aries", {4, 30}: "Taurus", {12, 25}: "Capricorn", {1, 5}: "Capricorn", {2, 29}: "Pisces", {8, 15}: "Leo"}
	for md, want := range tests {
		if got := TypicalSunSign(md[0], md[1]); got != want {
			t.Errorf("TypicalSunSign(%v) = %s, want %s", md, got, want)
		}
	}
	for _, bad := range [][2]int{{0, 1}, {13, 1}, {2, 30}, {4, 31}} {
		if got := TypicalSunSign(bad[0], bad[1]); got != "" {
			t.Errorf("TypicalSunSign(%v) = %q", bad, got)
		}
	}
	first, last, ok := SunSignDates("Capricorn", 2026, time.UTC)
	if !ok || first.Format(time.DateOnly) != "2026-12-22" || last.Format(time.DateOnly) != "2027-01-19" { // solstice 21 Dec 20:50 UTC
		t.Errorf("Capricorn 2026: %s – %s", first.Format(time.DateOnly), last.Format(time.DateOnly))
	}
	first, last, ok = SunSignDates("aries", 2026, time.UTC)
	if !ok || first.Format(time.DateOnly) != "2026-03-21" || last.Format(time.DateOnly) != "2026-04-19" { // equinox 20 Mar 14:46 UTC
		t.Errorf("Aries 2026: %s – %s", first.Format(time.DateOnly), last.Format(time.DateOnly))
	}
	if _, _, ok := SunSignDates("Ophiuchus", 2026, time.UTC); ok {
		t.Error("unknown sign accepted")
	}
}
