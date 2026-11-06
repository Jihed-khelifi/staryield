package astro

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/astro/ephem"
	"github.com/Jihed-khelifi/staryield/backend/internal/astro/tz"
)

func angleDiff(a, b float64) float64 { return math.Abs(wrap180(a - b)) }

func asind(x float64) float64 { return math.Asin(math.Max(-1, math.Min(1, x))) * rad2deg }

// eclipticToEquatorial converts an ecliptic longitude (latitude 0) into
// right ascension and declination.
func eclipticToEquatorial(lon, eps float64) (ra, dec float64) {
	return norm360(atan2d(sind(lon)*cosd(eps), cosd(lon))), asind(sind(eps) * sind(lon))
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
			if angleDiff(asc, tt.wantAsc) > tt.tolerance || angleDiff(mc, tt.wantMC) > tt.tolerance {
				t.Errorf("asc/mc = %.4f°/%.4f°, want %.4f°/%.4f°", asc, mc, tt.wantAsc, tt.wantMC)
			}
			if SignForLongitude(asc+1e-6) != tt.wantAscSign || SignForLongitude(mc+1e-6) != tt.wantMCSign {
				t.Errorf("signs = %s/%s, want %s/%s", SignForLongitude(asc), SignForLongitude(mc), tt.wantAscSign, tt.wantMCSign)
			}
		})
	}
}

// TestAnglesGeometry checks the defining properties independently of the
// closed-form formulas: the Ascendant lies on the eastern horizon and the
// Midheaven on the upper meridian, across latitudes and times of day.
func TestAnglesGeometry(t *testing.T) {
	base := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	for lat := -60.0; lat <= 60; lat += 15 {
		for _, lon := range []float64{-157.86, -74, 0, 10.18, 139.65} {
			for h := 0; h < 24; h += 2 {
				at := base.Add(time.Duration(h)*time.Hour + 17*time.Minute)
				asc, mc, ramc, eps := anglesAt(at, GeoLocation{Latitude: lat, Longitude: lon})
				ra, dec := eclipticToEquatorial(asc, eps)
				ha := ramc - ra
				if alt := asind(sind(lat)*sind(dec) + cosd(lat)*cosd(dec)*cosd(ha)); math.Abs(alt) > 1e-6 {
					t.Fatalf("lat %.0f lon %.2f %s: Ascendant %.3f° has altitude %.2e°", lat, lon, at, asc, alt)
				}
				if sind(ha) >= 0 {
					t.Fatalf("lat %.0f lon %.2f %s: Ascendant %.3f° is setting", lat, lon, at, asc)
				}
				if ahead := norm360(asc - mc); ahead <= 0 || ahead >= 180 {
					t.Fatalf("lat %.0f %s: Ascendant %.3f° not within 180° ahead of MC %.3f°", lat, at, asc, mc)
				}
				if mra, _ := eclipticToEquatorial(mc, eps); angleDiff(mra, ramc) > 1e-6 {
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
	if asc1, _ := Angles(start.Add(siderealDay), loc); angleDiff(asc0, asc1) > 0.01 {
		t.Errorf("Ascendant after one sidereal day differs by %.4f°", angleDiff(asc0, asc1))
	}
	total, prev := 0.0, asc0
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
	if total += norm360(asc - prev); math.Abs(total-360) > 0.05 {
		t.Errorf("Ascendant advanced %.3f° over a sidereal day, want 360°", total)
	}
}

type swissChart struct {
	Name      string               `json:"name"`
	Instant   string               `json:"instant"`
	Latitude  float64              `json:"latitude"`
	Longitude float64              `json:"longitude"`
	Systems   map[string][]float64 `json:"systems"`
	Asc       float64              `json:"asc"`
	MC        float64              `json:"mc"`
	ARMC      float64              `json:"armc"`
}

// TestHousesAgainstSwissEphemeris compares angles and cusps with independent
// values computed offline by the Swiss Ephemeris (testdata/houses_swisseph.json).
func TestHousesAgainstSwissEphemeris(t *testing.T) {
	raw, err := os.ReadFile("testdata/houses_swisseph.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Charts []swissChart `json:"charts"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Charts) < 8 {
		t.Fatalf("only %d reference charts", len(file.Charts))
	}
	const tol = 0.002 // degrees (7.2″): Swiss uses IAU 2006 precession and IAU 2000B nutation
	maxAngle, maxCusp := 0.0, 0.0
	for _, rc := range file.Charts {
		at, err := time.Parse(time.RFC3339, rc.Instant)
		if err != nil {
			t.Fatal(err)
		}
		loc := GeoLocation{Latitude: rc.Latitude, Longitude: rc.Longitude}
		asc, mc, ramc, eps := anglesAt(at, loc)
		for _, c := range []struct {
			name      string
			got, want float64
		}{{"RAMC", ramc, rc.ARMC}, {"Ascendant", asc, rc.Asc}, {"Midheaven", mc, rc.MC}} {
			d := angleDiff(c.got, c.want)
			maxAngle = math.Max(maxAngle, d)
			if d > tol {
				t.Errorf("%s %s = %.5f°, Swiss %.5f° (off %.1f″)", rc.Name, c.name, c.got, c.want, d*3600)
			}
		}
		for sysName, want := range rc.Systems {
			sys := HouseSystem(sysName)
			cusps, used, fallback := Houses(sys, ramc, eps, rc.Latitude)
			if sys == Porphyry && used != Porphyry {
				t.Errorf("%s: Porphyry not used", rc.Name)
			}
			if fallback != "" {
				t.Errorf("%s %s: unexpected fallback %q", rc.Name, sys, fallback)
			}
			for i := range cusps {
				d := angleDiff(cusps[i], want[i])
				maxCusp = math.Max(maxCusp, d)
				if d > tol {
					t.Errorf("%s %s cusp %d = %.5f°, Swiss %.5f° (off %.1f″)", rc.Name, sys, i+1, cusps[i], want[i], d*3600)
				}
			}
		}
	}
	t.Logf("max difference: angles %.2f″, cusps %.2f″", maxAngle*3600, maxCusp*3600)
}

// TestPlacidusSemiArcs verifies the defining property of Placidus cusps: the
// 11th and 12th cusps sit at one and two thirds of their own diurnal
// semi-arc east of the meridian.
func TestPlacidusSemiArcs(t *testing.T) {
	eps := 23.4367
	for _, lat := range []float64{-55, -33.9, 0.2, 40.7, 51.5, 60.2, 64.1} {
		for ramc := 0.0; ramc < 360; ramc += 29 {
			cusps, used, _ := Houses(Placidus, ramc, eps, lat)
			if used != Placidus {
				t.Fatalf("lat %.1f: Placidus fell back", lat)
			}
			for _, k := range []struct {
				idx      int
				fraction float64
			}{{10, 1.0 / 3}, {11, 2.0 / 3}} {
				ra, dec := eclipticToEquatorial(cusps[k.idx], eps)
				dsa := 90 + asind(tand(lat)*tand(dec))
				ha := wrap180(ra - ramc) // degrees east of the meridian
				if math.Abs(ha-k.fraction*dsa) > 1e-6 {
					t.Errorf("lat %.1f RAMC %.0f cusp %d: hour angle %.6f°, want %.6f°", lat, ramc, k.idx+1, ha, k.fraction*dsa)
				}
			}
		}
	}
}

func TestPolarFallback(t *testing.T) {
	at := time.Date(1990, 1, 15, 10, 0, 0, 0, time.UTC)
	c, err := ComputeChart(ChartRequest{Instant: at, TimeKnown: true, Loc: &GeoLocation{Latitude: 69.6492, Longitude: 18.9553}})
	if err != nil {
		t.Fatal(err)
	}
	if c.HouseSystem != Placidus || c.HouseSystemUsed != Porphyry || c.FallbackReason != FallbackPolarLatitude {
		t.Errorf("polar chart: requested %s used %s reason %q", c.HouseSystem, c.HouseSystemUsed, c.FallbackReason)
	}
	ws, _ := ComputeChart(ChartRequest{Instant: at, TimeKnown: true, Loc: &GeoLocation{Latitude: 69.6492, Longitude: 18.9553}, System: WholeSign})
	if ws.HouseSystemUsed != WholeSign || ws.FallbackReason != "" {
		t.Errorf("whole sign at 69°N: %s %q", ws.HouseSystemUsed, ws.FallbackReason)
	}
}

func TestHouseOf(t *testing.T) {
	var equal [12]float64
	for i := range equal {
		equal[i] = norm360(350 + 30*float64(i))
	}
	tests := []struct {
		lon  float64
		want int
	}{{350, 1}, {359.99, 1}, {10, 1}, {19.99, 1}, {20, 2}, {349.99, 12}, {170, 7}}
	for _, tt := range tests {
		if got := HouseOf(tt.lon, equal); got != tt.want {
			t.Errorf("HouseOf(%v) = %d, want %d", tt.lon, got, tt.want)
		}
	}
	if HouseLabel(0) != "" || HouseLabel(13) != "" || HouseLabel(12) != "Twelfth house" {
		t.Error("HouseLabel bounds")
	}
}

func TestParseHouseSystem(t *testing.T) {
	for in, want := range map[string]HouseSystem{"": Placidus, "Placidus": Placidus, "whole_sign": WholeSign, "Whole-Sign": WholeSign, "equal": Equal} {
		if got, ok := ParseHouseSystem(in); !ok || got != want {
			t.Errorf("ParseHouseSystem(%q) = %s, %v", in, got, ok)
		}
	}
	for _, in := range []string{"porphyry", "koch", "x"} {
		if _, ok := ParseHouseSystem(in); ok {
			t.Errorf("ParseHouseSystem(%q) accepted", in)
		}
	}
}

func TestComputeChartKnownCharts(t *testing.T) {
	honolulu, ok := PlaceByID("honolulu-us")
	if !ok {
		t.Fatal("legacy honolulu-us does not resolve")
	}
	hst, err := tz.Load(honolulu.TimeZone)
	if err != nil {
		t.Fatal(err)
	}
	obamaBirth, status := tz.Resolve(time.Date(1961, 8, 4, 0, 0, 0, 0, time.UTC), 19, 24, hst, honolulu.Longitude)
	if want := time.Date(1961, 8, 5, 5, 24, 0, 0, time.UTC); !obamaBirth.Equal(want) || status != tz.WallOK {
		t.Fatalf("Honolulu birth instant = %s %s, want %s", obamaBirth, status, want)
	}
	tests := []struct {
		name                    string
		at                      time.Time
		loc                     GeoLocation
		sun, moon, rising       string
		ascLon, sunLon, moonLon float64
	}{
		{
			// Published chart (Rodden AA): Asc 18°24′ Sagittarius, Sun 9°40′
			// Cancer, Moon 25°02′ Aquarius. Sandringham, 52°50′N 0°30′E.
			name: "1961-07-01 19:45 BST Sandringham", at: time.Date(1961, 7, 1, 18, 45, 0, 0, time.UTC),
			loc: GeoLocation{Latitude: 52 + 50.0/60, Longitude: 0.5},
			sun: "Cancer", moon: "Aquarius", rising: "Sagittarius", ascLon: 258.40, sunLon: 99.66, moonLon: 325.04,
		},
		{
			// Published chart (Rodden AA): Asc 18°03′ Aquarius, Sun 12°33′
			// Leo, Moon 3°21′ Gemini.
			name: "1961-08-04 19:24 HST Honolulu", at: obamaBirth, loc: LocationOf(honolulu),
			sun: "Leo", moon: "Gemini", rising: "Aquarius", ascLon: 318.05, sunLon: 132.55, moonLon: 63.36,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := ComputeChart(ChartRequest{Instant: tt.at, TimeKnown: true, Loc: &tt.loc})
			if err != nil {
				t.Fatal(err)
			}
			if c.SunSign != tt.sun || c.MoonSign != tt.moon || c.RisingSign != tt.rising {
				t.Errorf("signs = %s/%s/%s, want %s/%s/%s", c.SunSign, c.MoonSign, c.RisingSign, tt.sun, tt.moon, tt.rising)
			}
			for _, ck := range []struct {
				body Body
				want float64
				tol  float64
			}{{Ascendant, tt.ascLon, 0.5}, {Sun, tt.sunLon, 0.05}, {Moon, tt.moonLon, 0.05}} {
				p, _ := c.PositionOf(ck.body)
				if d := angleDiff(p.Longitude, ck.want); d > ck.tol {
					t.Errorf("%s = %.3f°, want %.3f° ± %.2f°", ck.body, p.Longitude, ck.want, ck.tol)
				}
			}
		})
	}
}

func TestComputeChartStructure(t *testing.T) {
	at := time.Date(1990, 5, 17, 8, 30, 0, 0, time.UTC)
	loc := GeoLocation{Latitude: 36.8065, Longitude: 10.1815}
	c, err := ComputeChart(ChartRequest{Instant: at, TimeKnown: true, Loc: &loc})
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := []string{"Ascendant", "Sun", "Moon", "Mercury", "Venus", "Mars", "Jupiter", "Saturn", "Uranus", "Neptune", "Pluto", "Midheaven"}
	if len(c.Positions) != len(wantOrder) {
		t.Fatalf("got %d positions, want %d", len(c.Positions), len(wantOrder))
	}
	cusps, ok := c.Cusps()
	if !ok {
		t.Fatal("no cusps")
	}
	for i, p := range c.Positions {
		if p.Body != wantOrder[i] {
			t.Errorf("position %d = %s, want %s", i, p.Body, wantOrder[i])
		}
		if p.House < 1 || p.House > 12 || p.HouseLabel != HouseLabel(p.House) {
			t.Errorf("%s: house %d label %q", p.Body, p.House, p.HouseLabel)
		}
		if i > 0 && i < 11 && p.House != HouseOf(p.Longitude, cusps) {
			t.Errorf("%s: house %d disagrees with cusps", p.Body, p.House)
		}
		if p.Retrograde != (p.Speed < 0 && canRetrograde(Body(p.Body))) {
			t.Errorf("%s: retrograde %v with speed %.4f", p.Body, p.Retrograde, p.Speed)
		}
	}
	if c.Positions[0].House != 1 || c.Positions[11].House != 10 {
		t.Errorf("angles in houses %d/%d, want 1/10", c.Positions[0].House, c.Positions[11].House)
	}
	if c.HouseSystem != Placidus || c.HouseSystemUsed != Placidus || len(c.Houses) != 12 || c.Angles == nil {
		t.Fatalf("houses not set: %+v", c)
	}
	if angleDiff(c.Houses[0].Cusp, c.Angles.Asc) > 1e-5 || angleDiff(c.Houses[9].Cusp, c.Angles.MC) > 1e-5 {
		t.Errorf("cusps 1/10 should equal Asc/MC")
	}
	if angleDiff(c.Angles.Dsc, c.Angles.Asc+180) > 1e-5 || angleDiff(c.Angles.IC, c.Angles.MC+180) > 1e-5 {
		t.Errorf("Dsc/IC not opposite")
	}
	total, mtotal := 0, 0
	for _, el := range Elements {
		total += c.Element[el]
	}
	for _, m := range Modalities {
		mtotal += c.Modality[m]
	}
	if total != 10 || mtotal != 10 {
		t.Errorf("element/modality totals %d/%d, want 10/10", total, mtotal)
	}
	if len(c.Aspects) == 0 {
		t.Error("no aspects found")
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"house_system":"placidus"`, `"house_system_used":"placidus"`, `"rising_sign"`, `"house_cusps"`, `"houses"`,
		`"angles"`, `"aspects"`, `"modalities"`, `"house_label":"First house"`, `"elements"`, `"speed"`, `"latitude"`, `"time_known":true`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("JSON missing %s", key)
		}
	}
}

func TestComputeChartWithoutLocation(t *testing.T) {
	c, err := ComputeChart(ChartRequest{Instant: time.Date(1990, 5, 17, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if c.HasLocation || c.HouseSystem != "" || c.RisingSign != "" || c.HouseCusps != nil || c.Angles != nil {
		t.Errorf("unexpected location fields: %+v", c)
	}
	if len(c.Positions) != 10 || c.Positions[0].Body != "Sun" || c.Positions[9].Body != "Pluto" {
		t.Fatalf("positions = %+v", c.Positions)
	}
	for _, p := range c.Positions {
		if p.House != 0 || p.HouseLabel != "" {
			t.Errorf("%s has house %d without a location", p.Body, p.House)
		}
	}
	for _, a := range c.Aspects {
		if a.A == "Ascendant" || a.B == "Ascendant" || a.A == "Midheaven" || a.B == "Midheaven" {
			t.Errorf("angle aspect without a location: %+v", a)
		}
	}
	if c.SunSign != "Taurus" {
		t.Errorf("SunSign = %s, want Taurus", c.SunSign)
	}
	b, _ := json.Marshal(c)
	for _, key := range []string{`"house_system"`, `"rising_sign"`, `"house_cusps"`, `"house"`, `"angles"`} {
		if strings.Contains(string(b), key) {
			t.Errorf("JSON unexpectedly contains %s", key)
		}
	}
}

func TestUnknownTimeUncertainty(t *testing.T) {
	paris, err := tz.Load("Europe/Paris")
	if err != nil {
		t.Fatal(err)
	}
	// 1987-11-29: the Moon enters Taurus during the Paris day.
	date := time.Date(1987, 11, 29, 0, 0, 0, 0, time.UTC)
	at, start, end := BirthWindow(date, 0, 0, false, paris)
	c, err := ComputeChart(ChartRequest{Instant: at, Loc: &GeoLocation{Latitude: 48.85, Longitude: 2.35}, WindowStart: start, WindowEnd: end})
	if err != nil {
		t.Fatal(err)
	}
	if c.TimeKnown || c.RisingSign != "" || c.Angles != nil {
		t.Errorf("unknown time must not produce angles: %+v", c)
	}
	moonStart := SignForLongitude(EclipticLongitude(Moon, start))
	moonEnd := SignForLongitude(EclipticLongitude(Moon, end))
	if moonStart != moonEnd && (!c.MoonSignUncertain || len(c.MoonSignAlternatives) != 2) {
		t.Errorf("Moon %s→%s during the day but uncertain=%v alternatives=%v", moonStart, moonEnd, c.MoonSignUncertain, c.MoonSignAlternatives)
	}
	if moonStart == moonEnd {
		t.Fatalf("test date no longer has a lunar ingress (%s)", moonStart)
	}
	// A mid-sign Moon is certain.
	at2, s2, e2 := BirthWindow(time.Date(1990, 5, 17, 0, 0, 0, 0, time.UTC), 0, 0, false, paris)
	c2, _ := ComputeChart(ChartRequest{Instant: at2, WindowStart: s2, WindowEnd: e2})
	if c2.MoonSignUncertain || c2.MoonSignAlternatives != nil {
		t.Errorf("certain Moon flagged: %v %v", c2.MoonSignUncertain, c2.MoonSignAlternatives)
	}
}

func TestOutOfRange(t *testing.T) {
	if _, err := ComputeChart(ChartRequest{Instant: time.Date(1600, 1, 1, 0, 0, 0, 0, time.UTC)}); err == nil {
		t.Error("1600 accepted")
	}
	// Pluto alone is outside its theory after 2099; the chart still computes.
	c, err := ComputeChart(ChartRequest{Instant: time.Date(2100, 6, 1, 12, 0, 0, 0, time.UTC)})
	if err != nil || len(c.Positions) != 9 {
		t.Errorf("2100 chart: %v, %d positions", err, len(c.Positions))
	}
}

func TestAspects(t *testing.T) {
	pt := func(b Body, lon, speed float64) aspectPoint { return aspectPoint{body: b, lon: lon, speed: speed} }
	asc := aspectPoint{body: Ascendant, lon: 100, angle: true}
	got := findAspects([]aspectPoint{
		pt(Sun, 10, 1), pt(Moon, 19.5, 13), // conjunction 9.5°: within 8+2 for the Sun/Moon
		pt(Mars, 70, 0.5), pt(Jupiter, 131, 0.1),
		pt(Saturn, 250, -0.05), // Saturn opposite Mars 180°
		asc,                    // Ascendant square Sun at 90° (orb 7−2=5)
	})
	find := func(a, b string) *Aspect {
		for i := range got {
			if (got[i].A == a && got[i].B == b) || (got[i].A == b && got[i].B == a) {
				return &got[i]
			}
		}
		return nil
	}
	// The faster Moon is moving away from the Sun: separating.
	if a := find("Sun", "Moon"); a == nil || a.Type != Conjunction || math.Abs(a.Orb-9.5) > 1e-9 || a.Applying {
		t.Errorf("Sun–Moon: %+v", a)
	}
	// Mars–Jupiter at 61° is a sextile, one degree wide.
	if a := find("Mars", "Jupiter"); a == nil || a.Type != Sextile || math.Abs(a.Orb-1) > 1e-9 {
		t.Errorf("Mars–Jupiter: %+v", a)
	}
	if a := find("Mars", "Saturn"); a == nil || a.Type != Opposition || a.Orb != 0 {
		t.Errorf("Mars–Saturn: %+v", a)
	}
	if a := find("Sun", "Ascendant"); a == nil || a.Type != Square || a.Applying {
		t.Errorf("Sun–Ascendant: %+v", a)
	}
	// One aspect per pair; Moon–Ascendant (80.5°) is outside every orb.
	if find("Moon", "Ascendant") != nil {
		t.Error("Moon–Ascendant 80.5° should not aspect")
	}
	seen := map[string]bool{}
	for _, a := range got {
		k := a.A + "|" + a.B
		if seen[k] {
			t.Errorf("duplicate aspect %s", k)
		}
		seen[k] = true
	}
	// An applying aspect: faster Mercury approaching Venus.
	app := findAspects([]aspectPoint{pt(Mercury, 50, 1.5), pt(Venus, 53, 1.0)})
	if len(app) != 1 || !app[0].Applying || app[0].Type != Conjunction {
		t.Errorf("applying conjunction: %+v", app)
	}
	// The tightest aspect wins when two would fit (an orb overlap at 52°
	// cannot happen with these orbs, so check the Sun–Moon +2 boundary).
	if out := findAspects([]aspectPoint{pt(Mars, 0, 0), pt(Venus, 8.01, 0)}); len(out) != 0 {
		t.Errorf("8.01° Mars–Venus should be outside the 8° conjunction orb: %+v", out)
	}
}

func TestPositionFormatting(t *testing.T) {
	tests := []struct {
		lon           float64
		speed         float64
		sign          string
		deg, min, sec int
		formatted     string
		retro         bool
	}{
		{90 + 8 + 22.0/60 + 47.0/3600, 1, "Cancer", 8, 22, 47, "Cancer, 8°22′47″", false},
		{150 + 18 + 42.0/60 + 5.0/3600, -0.3, "Virgo", 18, 42, 5, "Virgo, 18°42′5″", true},
		{29.9999999, 1, "Taurus", 0, 0, 0, "Taurus, 0°0′0″", false},
		{359.9999999, 1, "Aries", 0, 0, 0, "Aries, 0°0′0″", false},
		{-0.5, 1, "Pisces", 29, 30, 0, "Pisces, 29°30′0″", false},
	}
	for _, tt := range tests {
		p := newPosition(Mercury, tt.lon, 0, tt.speed)
		if p.Sign != tt.sign || p.Degree != tt.deg || p.Minute != tt.min || p.Second != tt.sec || p.Formatted != tt.formatted {
			t.Errorf("newPosition(%v) = %+v", tt.lon, p)
		}
		if p.Retrograde != tt.retro {
			t.Errorf("retrograde = %v for speed %v", p.Retrograde, tt.speed)
		}
	}
	if newPosition(Sun, 10, 0, -1).Retrograde || newPosition(Ascendant, 10, 0, -1).Retrograde {
		t.Error("Sun or Ascendant flagged retrograde")
	}
}

func BenchmarkComputeChart(b *testing.B) {
	ephem.Default()
	loc := GeoLocation{Latitude: 48.8566, Longitude: 2.3522}
	at := time.Date(1987, 11, 29, 13, 30, 0, 0, time.UTC)
	for b.Loop() {
		if _, err := ComputeChart(ChartRequest{Instant: at, TimeKnown: true, Loc: &loc}); err != nil {
			b.Fatal(err)
		}
	}
}
