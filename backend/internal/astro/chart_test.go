package astro

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func TestGreenwichSiderealTime(t *testing.T) {
	// Meeus example 12.a: 1987 April 10, 0h UT → GMST 13h10m46.3668s.
	at := time.Date(1987, 4, 10, 0, 0, 0, 0, time.UTC)
	want := (13 + 10.0/60 + 46.3668/3600) * 15
	if got := greenwichSiderealDegrees(at); angleDiff(got, want) > 1e-4 {
		t.Errorf("GMST = %.6f°, want %.6f°", got, want)
	}
}

func TestAnglesFromRAMCTableOfHouses(t *testing.T) {
	// Raphael's table of houses for London (51°32′N): at sidereal time 0h00m
	// the Midheaven is 0° Aries and the Ascendant 26°36′ Cancer. On the
	// equator, at the cardinal sidereal times, the Ascendant is exactly 90°
	// ahead of the Midheaven.
	tests := []struct {
		name                    string
		ramc, eps, lat          float64
		wantAsc, wantMC         float64
		tolerance               float64
		wantAscSign, wantMCSign string
	}{
		{"London RAMC 0", 0, 23.44, 51 + 32.0/60, 116.6, 0, 0.2, "Cancer", "Aries"},
		{"Equator RAMC 0", 0, 23.44, 0, 90, 0, 1e-9, "Cancer", "Aries"},
		{"Equator RAMC 90", 90, 23.44, 0, 180, 90, 1e-9, "Libra", "Cancer"},
		{"Equator RAMC 180", 180, 23.44, 0, 270, 180, 1e-9, "Capricorn", "Libra"},
		{"Equator RAMC 270", 270, 23.44, 0, 0, 270, 1e-9, "Aries", "Capricorn"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asc, mc := anglesFromRAMC(tt.ramc, tt.eps, tt.lat)
			if angleDiff(asc, tt.wantAsc) > tt.tolerance {
				t.Errorf("asc = %.4f°, want %.4f°", asc, tt.wantAsc)
			}
			if angleDiff(mc, tt.wantMC) > tt.tolerance {
				t.Errorf("mc = %.4f°, want %.4f°", mc, tt.wantMC)
			}
			if SignForLongitude(asc+1e-6) != tt.wantAscSign || SignForLongitude(mc+1e-6) != tt.wantMCSign {
				t.Errorf("signs = %s/%s, want %s/%s", SignForLongitude(asc), SignForLongitude(mc), tt.wantAscSign, tt.wantMCSign)
			}
		})
	}
}

func asind(x float64) float64 { return math.Asin(math.Max(-1, math.Min(1, x))) * rad2deg }

// eclipticToEquatorial converts an ecliptic longitude (latitude 0) into
// right ascension and declination.
func eclipticToEquatorial(lon, eps float64) (ra, dec float64) {
	return norm360(atan2d(sind(lon)*cosd(eps), cosd(lon))), asind(sind(eps) * sind(lon))
}

// TestAnglesGeometry checks the defining properties independently of the
// closed-form formulas: the Ascendant lies on the eastern horizon and the
// Midheaven on the upper meridian, across latitudes and times of day.
func TestAnglesGeometry(t *testing.T) {
	base := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	for lat := -60.0; lat <= 60; lat += 15 {
		for _, lon := range []float64{-157.86, -74, 0, 10.18, 139.65} {
			for h := 0; h < 24; h += 1 {
				at := base.Add(time.Duration(h)*time.Hour + 17*time.Minute)
				asc, mc := Angles(at, GeoLocation{Latitude: lat, Longitude: lon})
				ramc := localSiderealDegrees(at, lon)
				eps := obliquity(dayNumber(at))

				ra, dec := eclipticToEquatorial(asc, eps)
				ha := ramc - ra
				alt := asind(sind(lat)*sind(dec) + cosd(lat)*cosd(dec)*cosd(ha))
				if math.Abs(alt) > 1e-6 {
					t.Fatalf("lat %.0f lon %.2f %s: Ascendant %.3f° has altitude %.2e°", lat, lon, at, asc, alt)
				}
				if sind(ha) >= 0 {
					t.Fatalf("lat %.0f lon %.2f %s: Ascendant %.3f° is setting (hour angle %.2f°)", lat, lon, at, asc, norm360(ha))
				}
				if ahead := norm360(asc - mc); ahead <= 0 || ahead >= 180 {
					t.Fatalf("lat %.0f %s: Ascendant %.3f° not within 180° ahead of MC %.3f°", lat, at, asc, mc)
				}

				mra, _ := eclipticToEquatorial(mc, eps)
				if angleDiff(mra, ramc) > 1e-6 {
					t.Fatalf("lat %.0f %s: Midheaven RA %.5f° != RAMC %.5f°", lat, at, mra, ramc)
				}
			}
		}
	}
}

func TestAscendantCompletesCircleInSiderealDay(t *testing.T) {
	const siderealDay = 23*time.Hour + 56*time.Minute + 4091*time.Millisecond
	loc := GeoLocation{Latitude: 40.7128, Longitude: -74.0060}
	start := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	asc0, _ := Angles(start, loc)
	asc1, _ := Angles(start.Add(siderealDay), loc)
	if d := angleDiff(asc0, asc1); d > 0.01 {
		t.Errorf("Ascendant after one sidereal day differs by %.4f°", d)
	}

	total := 0.0
	prev := asc0
	const step = 10 * time.Minute
	for at := start.Add(step); !at.After(start.Add(siderealDay)); at = at.Add(step) {
		asc, _ := Angles(at, loc)
		delta := norm360(asc - prev)
		if delta <= 0 || delta > 30 {
			t.Fatalf("Ascendant moved by %.3f° in %s at %s", delta, step, at)
		}
		total += delta
		prev = asc
	}
	asc, _ := Angles(start.Add(siderealDay), loc)
	total += norm360(asc - prev)
	if math.Abs(total-360) > 0.05 {
		t.Errorf("Ascendant advanced %.3f° over a sidereal day, want 360°", total)
	}
}

func TestComputeChartKnownCharts(t *testing.T) {
	honolulu, ok := PlaceByID("honolulu-us")
	if !ok {
		t.Fatal("honolulu-us missing from gazetteer")
	}
	obamaBirth, err := LocalBirthInstant(time.Date(1961, 8, 4, 0, 0, 0, 0, time.UTC), "19:24", honolulu.TimeZone)
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(1961, 8, 5, 5, 24, 0, 0, time.UTC); !obamaBirth.Equal(want) {
		t.Fatalf("Honolulu birth instant = %s, want %s", obamaBirth, want)
	}

	tests := []struct {
		name                       string
		at                         time.Time
		loc                        GeoLocation
		sun, moon, rising          string
		ascLon, sunLon, moonLon    float64
		ascTolerance, lonTolerance float64
	}{
		{
			// Published chart (Rodden AA): Asc 18°24′ Sagittarius, Sun 9°40′
			// Cancer, Moon 25°02′ Aquarius. Sandringham, 52°50′N 0°30′E.
			name: "1961-07-01 19:45 BST Sandringham",
			at:   time.Date(1961, 7, 1, 18, 45, 0, 0, time.UTC),
			loc:  GeoLocation{Latitude: 52 + 50.0/60, Longitude: 0.5},
			sun:  "Cancer", moon: "Aquarius", rising: "Sagittarius",
			ascLon: 258.40, sunLon: 99.66, moonLon: 325.04,
			ascTolerance: 0.5, lonTolerance: 0.1,
		},
		{
			// Published chart (Rodden AA): Asc 18°03′ Aquarius, Sun 12°33′
			// Leo, Moon 3°21′ Gemini.
			name: "1961-08-04 19:24 HST Honolulu",
			at:   obamaBirth,
			loc:  honolulu.Location(),
			sun:  "Leo", moon: "Gemini", rising: "Aquarius",
			ascLon: 318.05, sunLon: 132.55, moonLon: 63.36,
			ascTolerance: 0.5, lonTolerance: 0.1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := ComputeChart(tt.at, &tt.loc)
			if c.SunSign != tt.sun || c.MoonSign != tt.moon || c.RisingSign != tt.rising {
				t.Errorf("signs = Sun %s / Moon %s / Rising %s, want %s / %s / %s",
					c.SunSign, c.MoonSign, c.RisingSign, tt.sun, tt.moon, tt.rising)
			}
			byBody := map[string]Position{}
			for _, p := range c.Positions {
				byBody[p.Body] = p
			}
			checks := []struct {
				body string
				want float64
				tol  float64
			}{
				{"Ascendant", tt.ascLon, tt.ascTolerance},
				{"Sun", tt.sunLon, tt.lonTolerance},
				{"Moon", tt.moonLon, tt.lonTolerance},
			}
			for _, ck := range checks {
				got := byBody[ck.body].Longitude
				d := angleDiff(got, ck.want)
				if d > ck.tol {
					t.Errorf("%s = %.3f°, want %.3f° ± %.2f°", ck.body, got, ck.want, ck.tol)
				}
				t.Logf("%-9s %s (off by %.3f°)", ck.body, byBody[ck.body].Formatted, d)
			}
		})
	}
}

func TestComputeChartStructure(t *testing.T) {
	at := time.Date(1990, 5, 17, 8, 30, 0, 0, time.UTC)
	loc := GeoLocation{Latitude: 36.8065, Longitude: 10.1815}
	c := ComputeChart(at, &loc)

	wantOrder := []string{"Ascendant", "Sun", "Moon", "Mercury", "Venus", "Mars", "Jupiter", "Saturn", "Uranus", "Neptune", "Pluto", "Midheaven"}
	if len(c.Positions) != len(wantOrder) {
		t.Fatalf("got %d positions, want %d", len(c.Positions), len(wantOrder))
	}
	for i, p := range c.Positions {
		if p.Body != wantOrder[i] {
			t.Errorf("position %d = %s, want %s", i, p.Body, wantOrder[i])
		}
		if p.House < 1 || p.House > 12 || p.HouseLabel != HouseLabel(p.House) {
			t.Errorf("%s: house %d label %q", p.Body, p.House, p.HouseLabel)
		}
		if p.Sign != SignForLongitude(p.Longitude) && p.Second != 0 {
			t.Errorf("%s: sign %s disagrees with longitude %.6f", p.Body, p.Sign, p.Longitude)
		}
		if (p.Body == "Ascendant" || p.Body == "Midheaven") && p.Retrograde {
			t.Errorf("%s flagged retrograde", p.Body)
		}
	}
	if !c.HasLocation || c.HouseSystem != "equal" || c.RisingSign == "" {
		t.Errorf("location fields not set: %+v", c)
	}
	if c.Positions[0].House != 1 || c.Positions[0].HouseLabel != "First house" {
		t.Errorf("Ascendant house = %d %q, want 1 First house", c.Positions[0].House, c.Positions[0].HouseLabel)
	}
	if len(c.HouseCusps) != 12 {
		t.Fatalf("got %d cusps", len(c.HouseCusps))
	}
	for i, cusp := range c.HouseCusps {
		want := norm360(c.Positions[0].Longitude + 30*float64(i))
		if angleDiff(cusp, want) > 1e-5 {
			t.Errorf("cusp %d = %.6f, want %.6f", i+1, cusp, want)
		}
	}
	if c.SunSign != c.Positions[1].Sign || c.MoonSign != c.Positions[2].Sign {
		t.Errorf("sun/moon sign summary mismatch")
	}
	total := 0
	for _, el := range Elements {
		n, ok := c.Element[el]
		if !ok {
			t.Errorf("element %s missing from counts", el)
		}
		total += n
	}
	if total != 10 {
		t.Errorf("element counts sum to %d, want 10", total)
	}
	if !c.Instant.Equal(at) || c.Instant.Location() != time.UTC {
		t.Errorf("instant = %v", c.Instant)
	}

	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"house_system":"equal"`, `"rising_sign"`, `"house_cusps"`, `"house_label":"First house"`, `"elements"`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("JSON missing %s", key)
		}
	}
}

func TestComputeChartWithoutLocation(t *testing.T) {
	c := ComputeChart(time.Date(1990, 5, 17, 12, 0, 0, 0, time.UTC), nil)
	if c.HasLocation || c.HouseSystem != "" || c.RisingSign != "" || c.HouseCusps != nil {
		t.Errorf("unexpected location fields: %+v", c)
	}
	if len(c.Positions) != 10 || c.Positions[0].Body != "Sun" || c.Positions[9].Body != "Pluto" {
		t.Fatalf("positions = %+v", c.Positions)
	}
	for _, p := range c.Positions {
		if p.House != 0 || p.HouseLabel != "" {
			t.Errorf("%s has house %d %q without a location", p.Body, p.House, p.HouseLabel)
		}
	}
	if c.SunSign != "Taurus" {
		t.Errorf("SunSign = %s, want Taurus", c.SunSign)
	}
	b, _ := json.Marshal(c)
	for _, key := range []string{`"house_system"`, `"rising_sign"`, `"house_cusps"`, `"house"`, `"house_label"`} {
		if strings.Contains(string(b), key) {
			t.Errorf("JSON unexpectedly contains %s: %s", key, b)
		}
	}
}

func TestPositionFormatting(t *testing.T) {
	tests := []struct {
		lon            float64
		sign           string
		deg, min, sec  int
		formatted      string
		retro, wantRet bool
	}{
		{90 + 8 + 22.0/60 + 47.0/3600, "Cancer", 8, 22, 47, "Cancer, 8°22′47″", false, false},
		{150 + 18 + 42.0/60 + 5.0/3600, "Virgo", 18, 42, 5, "Virgo, 18°42′5″", true, true},
		{120 + 25 + 4.0/60 + 2.0/3600, "Leo", 25, 4, 2, "Leo, 25°4′2″", false, false},
		{210 + 12 + 45.0/60, "Scorpio", 12, 45, 0, "Scorpio, 12°45′0″", false, false},
		{0, "Aries", 0, 0, 0, "Aries, 0°0′0″", false, false},
		{29.9999999, "Taurus", 0, 0, 0, "Taurus, 0°0′0″", false, false}, // rounds up into the next sign
		{359.9999999, "Aries", 0, 0, 0, "Aries, 0°0′0″", false, false},  // and wraps
		{-0.5, "Pisces", 29, 30, 0, "Pisces, 29°30′0″", false, false},
	}
	for _, tt := range tests {
		p := newPosition(Mercury, tt.lon, tt.retro)
		if p.Sign != tt.sign || p.Degree != tt.deg || p.Minute != tt.min || p.Second != tt.sec {
			t.Errorf("newPosition(%v) = %s %d°%d′%d″, want %s %d°%d′%d″", tt.lon, p.Sign, p.Degree, p.Minute, p.Second, tt.sign, tt.deg, tt.min, tt.sec)
		}
		if p.Formatted != tt.formatted {
			t.Errorf("Formatted = %q, want %q", p.Formatted, tt.formatted)
		}
		if p.Retrograde != tt.wantRet || p.Body != "Mercury" {
			t.Errorf("body/retrograde = %s/%v", p.Body, p.Retrograde)
		}
	}
}

func TestEqualHouse(t *testing.T) {
	tests := []struct {
		lon, asc float64
		want     int
	}{
		{100, 100, 1}, {129.99, 100, 1}, {130, 100, 2}, {99.99, 100, 12}, {10, 350, 1}, {340, 350, 12}, {200, 20, 7},
	}
	for _, tt := range tests {
		if got := equalHouse(tt.lon, tt.asc); got != tt.want {
			t.Errorf("equalHouse(%v, %v) = %d, want %d", tt.lon, tt.asc, got, tt.want)
		}
	}
	if HouseLabel(0) != "" || HouseLabel(13) != "" || HouseLabel(12) != "Twelfth house" {
		t.Error("HouseLabel bounds")
	}
}
