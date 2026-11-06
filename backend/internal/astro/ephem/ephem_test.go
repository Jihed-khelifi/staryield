package ephem

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"testing"
	"time"
)

// legacyHorizons are apparent geocentric ecliptic-of-date longitudes from JPL
// Horizons (observer table, quantity 31, geocentre) at UT instants, for Sun,
// Moon, Mercury … Pluto in that order. They predate testdata/horizons.json
// and are kept as a second, independently retrieved set.
var legacyHorizons = []struct {
	instant string
	lon     [10]float64
}{
	{"1905-03-10T04:00:00Z", [10]float64{348.9266537, 31.6306735, 348.8967544, 33.0794466, 232.0284132, 30.4956236, 326.4726022, 273.8598624, 95.3847771, 79.6983437}},
	{"1925-06-15T06:00:00Z", [10]float64{83.6023430, 14.8308252, 77.5138135, 97.5444312, 112.9776160, 290.6169108, 218.1883601, 355.3765793, 140.3333678, 102.5552504}},
	{"1940-03-03T18:30:00Z", [10]float64{343.0421900, 284.3429976, 359.7860775, 24.3095946, 40.6082732, 12.5805004, 28.1750464, 48.5662508, 174.4526565, 120.9503531}},
	{"1955-11-20T00:00:00Z", [10]float64{236.8849926, 296.5451827, 228.5770301, 257.3496351, 204.1353179, 150.2550442, 234.1663603, 122.2752705, 209.0224767, 148.5816526}},
	{"1961-07-01T18:45:00Z", [10]float64{99.6630446, 325.0378776, 93.2013895, 54.3998280, 151.6457017, 305.0970740, 297.8140038, 143.3359888, 218.6366645, 156.0440138}},
	{"1961-08-05T05:24:00Z", [10]float64{132.5479089, 63.3573772, 122.3315919, 91.7893450, 172.5765892, 300.8586257, 295.3307494, 145.2707961, 218.6059373, 156.9777658}},
	{"1970-01-01T00:00:00Z", [10]float64{280.1562711, 190.6990372, 299.0214552, 274.4532541, 342.2362370, 212.3250046, 32.0622903, 188.7206104, 239.8858503, 177.3921739}},
	{"1983-09-09T09:09:00Z", [10]float64{166.1258480, 197.7236336, 178.1385367, 143.9961897, 137.0635060, 243.6425164, 211.4037849, 245.3613059, 266.4631690, 207.8083696}},
	{"1990-12-31T23:00:00Z", [10]float64{280.0127789, 102.6047523, 264.3309829, 294.6695048, 57.7553221, 131.9864327, 295.6579691, 279.7271115, 284.1152531, 229.5901878}},
	{"2000-01-01T12:00:00Z", [10]float64{280.3689092, 223.3237860, 271.8892699, 241.5657794, 327.9632921, 25.2530685, 40.3956366, 314.8091680, 303.1930007, 251.4547644}},
	{"2008-08-08T08:08:00Z", [10]float64{136.1527543, 220.5102761, 146.0644794, 152.6530916, 173.0065752, 283.9975321, 158.6035934, 351.9819244, 323.0424418, 268.7491962}},
	{"2012-12-21T11:11:00Z", [10]float64{269.9995560, 14.2375237, 254.6982230, 246.5861326, 296.4276992, 68.9007757, 218.6610314, 4.6410013, 330.8134483, 278.9493787}},
	{"2018-05-05T05:05:00Z", [10]float64{44.6633765, 283.4003241, 18.5039837, 72.7541712, 295.3652712, 228.8420474, 278.9124778, 29.4301210, 345.9581106, 291.2486319}},
	{"2024-04-08T18:21:00Z", [10]float64{19.4005233, 19.4018898, 24.7977807, 4.4452095, 343.0514935, 49.0457559, 344.4552370, 51.1711282, 358.1901376, 301.9676432}},
	{"2025-10-20T12:00:00Z", [10]float64{207.3527534, 196.1555634, 229.4653344, 188.2177240, 229.3374375, 114.3573430, 356.4083820, 60.6871641, 0.0450773, 301.3765377}},
	{"2033-02-14T14:00:00Z", [10]float64{326.1912904, 149.3624676, 344.0085088, 5.3750952, 244.5634144, 317.1379669, 90.4239005, 87.7549665, 15.7824782, 315.1307911}},
	{"2047-07-07T07:07:00Z", [10]float64{105.1432249, 283.2597081, 84.8627153, 140.7260915, 101.6502764, 48.7475073, 270.2636985, 153.6913757, 50.9922045, 336.5824734}},
	{"2061-10-01T00:00:00Z", [10]float64{188.3506903, 29.6044525, 187.9837378, 234.3489546, 249.3053349, 124.4944233, 91.4250151, 221.1263261, 83.0827710, 352.8531764}},
	{"2077-03-15T15:30:00Z", [10]float64{355.6853969, 241.2370173, 13.5755652, 357.3355032, 10.0850926, 224.1571632, 280.2572343, 292.5945437, 113.8759897, 10.1363666}},
	{"2095-08-31T21:00:00Z", [10]float64{158.7685017, 180.9124110, 178.3113862, 118.0013827, 255.6275901, 74.3517865, 146.9012256, 4.7334085, 156.1763392, 30.2670173}},
}

// Tolerances in degrees: 0.001° (3.6″) for the Sun and planets, 0.003°
// (10.8″) for Pluto (Meeus series) and 0.004° (14.4″) for the Moon, whose
// truncated ELP-2000/82 series (Meeus ch. 47) is accurate to about 10″.
var lonTolerance = map[Body]float64{
	Sun: 0.001, Moon: 0.004, Mercury: 0.001, Venus: 0.001, Mars: 0.001,
	Jupiter: 0.001, Saturn: 0.001, Uranus: 0.001, Neptune: 0.001, Pluto: 0.003,
}

type horizonsFile struct {
	Bodies []struct {
		Body    string `json:"body"`
		Samples []struct {
			Instant string  `json:"instant"`
			Lon     float64 `json:"lon"`
			Lat     float64 `json:"lat"`
		} `json:"samples"`
	} `json:"bodies"`
}

func bodyByName(t *testing.T, name string) Body {
	t.Helper()
	for _, b := range Bodies {
		if b.String() == name {
			return b
		}
	}
	t.Fatalf("unknown body %q", name)
	return 0
}

func mustJD(t *testing.T, s string) float64 {
	t.Helper()
	at, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return JulianDay(at)
}

func arcsecDiff(a, b float64) float64 { return math.Abs(wrap180(a-b)) * 3600 }

func TestPositionsAgainstHorizons(t *testing.T) {
	raw, err := os.ReadFile("testdata/horizons.json")
	if err != nil {
		t.Fatal(err)
	}
	var f horizonsFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	e := Default()
	maxLon, maxLat := map[Body]float64{}, map[Body]float64{}
	n := 0
	check := func(b Body, instant string, lon float64, lat *float64) {
		t.Helper()
		got, err := e.Position(b, mustJD(t, instant))
		if err != nil {
			t.Fatalf("%s %s: %v", b, instant, err)
		}
		n++
		d := arcsecDiff(got.Lon, lon)
		maxLon[b] = math.Max(maxLon[b], d)
		if d > lonTolerance[b]*3600 {
			t.Errorf("%s at %s: longitude %.6f°, want %.6f° (off %.2f″)", b, instant, got.Lon, lon, d)
		}
		if lat != nil {
			dl := math.Abs(got.Lat-*lat) * 3600
			maxLat[b] = math.Max(maxLat[b], dl)
			if dl > lonTolerance[b]*3600 {
				t.Errorf("%s at %s: latitude %.6f°, want %.6f° (off %.2f″)", b, instant, got.Lat, *lat, dl)
			}
		}
	}
	for _, set := range f.Bodies {
		b := bodyByName(t, set.Body)
		for _, s := range set.Samples {
			lat := s.Lat
			check(b, s.Instant, s.Lon, &lat)
		}
	}
	for _, ref := range legacyHorizons {
		for i, b := range Bodies {
			check(b, ref.instant, ref.lon[i], nil)
		}
	}
	if n < 150 {
		t.Fatalf("only %d reference vectors checked, want ≥ 150", n)
	}
	for _, b := range Bodies {
		t.Logf("%-8s max |Δλ| %.2f″, max |Δβ| %.2f″", b, maxLon[b], maxLat[b])
	}
	t.Logf("%d reference positions checked", n)
}

func TestMeeusWorkedExamples(t *testing.T) {
	e := Default()
	// Instants given in TD by Meeus are converted to UT with our ΔT.
	td := func(y int, m time.Month, d int) float64 {
		jde := JulianDay(time.Date(y, m, d, 0, 0, 0, 0, time.UTC))
		return jde - DeltaT(jde)/86400
	}
	tests := []struct {
		name     string
		body     Body
		jdUT     float64
		lon, lat float64
		tolSec   float64
	}{
		// Example 25.b (VSOP87): apparent Sun 199°54′21.818″, β +0.72″.
		{"Sun 25.b", Sun, td(1992, 10, 13), 199.9060606, 0.0002, 1},
		// Example 47.a: apparent Moon 133.167265°, β −3.229126°.
		{"Moon 47.a", Moon, td(1992, 4, 12), 133.167265, -3.229126, 1},
		// Example 33.a: Venus 313.08102°, β −2.08474°.
		{"Venus 33.a", Venus, td(1992, 12, 20), 313.08102, -2.08474, 2},
	}
	for _, tt := range tests {
		got, err := e.Position(tt.body, tt.jdUT)
		if err != nil {
			t.Fatal(err)
		}
		if d := arcsecDiff(got.Lon, tt.lon); d > tt.tolSec {
			t.Errorf("%s: λ %.6f°, want %.6f° (off %.2f″)", tt.name, got.Lon, tt.lon, d)
		}
		if d := math.Abs(got.Lat-tt.lat) * 3600; d > tt.tolSec {
			t.Errorf("%s: β %.6f°, want %.6f° (off %.2f″)", tt.name, got.Lat, tt.lat, d)
		}
	}
}

func TestSiderealTimeAndObliquity(t *testing.T) {
	// Meeus example 12.a: 1987 April 10, 0h UT.
	jd := JulianDay(time.Date(1987, 4, 10, 0, 0, 0, 0, time.UTC))
	hms := func(h, m int, s float64) float64 { return (float64(h) + float64(m)/60 + s/3600) * 15 }
	if got, want := MeanSiderealTime(jd), hms(13, 10, 46.3668); math.Abs(got-want)*3600 > 0.01 {
		t.Errorf("mean sidereal time %.7f°, want %.7f°", got, want)
	}
	if got, want := ApparentSiderealTime(jd), hms(13, 10, 46.1351); math.Abs(got-want)*3600 > 0.02 {
		t.Errorf("apparent sidereal time %.7f°, want %.7f° (off %.3f″)", got, want, (got-want)*3600)
	}
	// Meeus example 22.a: 1987 April 10, 0h TD.
	jde := JulianDay(time.Date(1987, 4, 10, 0, 0, 0, 0, time.UTC))
	dpsi, deps := Nutation(jde)
	if math.Abs(dpsi*3600+3.788) > 0.01 || math.Abs(deps*3600-9.443) > 0.01 {
		t.Errorf("nutation Δψ %.3f″ Δε %.3f″, want −3.788″ +9.443″", dpsi*3600, deps*3600)
	}
	if got, want := TrueObliquity(jde), 23+26.0/60+36.850/3600; math.Abs(got-want)*3600 > 0.01 {
		t.Errorf("true obliquity %.7f°, want %.7f°", got, want)
	}
}

func TestDeltaT(t *testing.T) {
	tests := []struct {
		at        time.Time
		want, tol float64
	}{
		{time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC), -1.944461, 1e-6},
		{time.Date(1900, 7, 2, 12, 0, 0, 0, time.UTC), (-1.944461 - 0.715674) / 2, 0.01},
		{time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), 64.1839, 0.001}, // TT − UTC = 32.184 + 32
		{time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), 69.18, 0.05},
		{time.Date(2101, 6, 1, 0, 0, 0, 0, time.UTC), 69.183904, 1e-6},
		{time.Date(1850, 1, 1, 0, 0, 0, 0, time.UTC), 7.0, 1.5}, // polynomial branch
	}
	for _, tt := range tests {
		if got := DeltaT(JulianDay(tt.at)); math.Abs(got-tt.want) > tt.tol {
			t.Errorf("DeltaT(%s) = %.4f, want %.4f ± %g", tt.at.Format(time.DateOnly), got, tt.want, tt.tol)
		}
	}
}

func TestRanges(t *testing.T) {
	e := Default()
	if _, err := e.Position(Pluto, JulianDay(time.Date(2100, 6, 1, 0, 0, 0, 0, time.UTC))); !errors.Is(err, ErrOutOfRange) {
		t.Errorf("Pluto in 2100: err %v, want ErrOutOfRange", err)
	}
	if _, err := e.Position(Mars, JulianDay(time.Date(2100, 6, 1, 0, 0, 0, 0, time.UTC))); err != nil {
		t.Errorf("Mars in 2100: %v", err)
	}
	pos, errs := e.Positions(JulianDay(time.Date(1700, 1, 1, 0, 0, 0, 0, time.UTC)))
	for i := range pos {
		if !errors.Is(errs[i], ErrOutOfRange) {
			t.Errorf("%s in 1700: err %v, want ErrOutOfRange", Bodies[i], errs[i])
		}
	}
	if _, err := e.Position(Body(42), JulianDay(time.Now())); err == nil {
		t.Error("unknown body accepted")
	}
}

func TestSpeedAndRetrograde(t *testing.T) {
	e := Default()
	day := func(y int, m time.Month, d int) float64 { return JulianDay(time.Date(y, m, d, 0, 0, 0, 0, time.UTC)) }
	tests := []struct {
		body       Body
		jd         float64
		retrograde bool
	}{
		{Mercury, day(2024, 4, 10), true},
		{Mercury, day(2024, 5, 10), false},
		{Mars, day(2025, 1, 15), true},
		{Mars, day(2025, 4, 15), false},
		{Neptune, JulianDay(time.Date(2025, 10, 22, 9, 0, 0, 0, time.UTC)), true}, // crosses 0° Aries backwards
	}
	for _, tt := range tests {
		p, err := e.Position(tt.body, tt.jd)
		if err != nil {
			t.Fatal(err)
		}
		if got := p.Speed < 0; got != tt.retrograde {
			t.Errorf("%s at %s: speed %.4f°/day, retrograde %v, want %v", tt.body, TimeFromJD(tt.jd).Format(time.DateOnly), p.Speed, got, tt.retrograde)
		}
	}
	for jd := day(2026, 1, 1); jd < day(2026, 12, 31); jd += 7.3 {
		pos, _ := e.Positions(jd)
		if s := pos[Sun].Speed; s < 0.94 || s > 1.03 {
			t.Errorf("Sun speed %.4f°/day out of range", s)
		}
		if s := pos[Moon].Speed; s < 11.5 || s > 15.6 {
			t.Errorf("Moon speed %.4f°/day out of range", s)
		}
	}
}

func TestFastLongitudeTracksPrecise(t *testing.T) {
	e := Default()
	for jd := JulianDay(time.Date(1950, 1, 1, 0, 0, 0, 0, time.UTC)); jd < JulianDay(time.Date(2050, 1, 1, 0, 0, 0, 0, time.UTC)); jd += 997.3 {
		pos, _ := e.Positions(jd)
		for i, b := range Bodies {
			if d := math.Abs(wrap180(FastLongitude(b, jd) - pos[i].Lon)); d > 0.25 {
				t.Errorf("%s at %s: fast model off by %.3f°", b, TimeFromJD(jd).Format(time.DateOnly), d)
			}
		}
	}
	if !math.IsNaN(FastLongitude(Body(42), 2451545)) {
		t.Error("FastLongitude accepted an unknown body")
	}
}

func TestJulianDayRoundTrip(t *testing.T) {
	at := time.Date(1987, 11, 29, 14, 30, 15, 0, time.UTC)
	if got := TimeFromJD(JulianDay(at)); got.Sub(at).Abs() > time.Millisecond {
		t.Errorf("round trip %s → %s", at, got)
	}
	if got := JulianDay(time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)); got != J2000 {
		t.Errorf("J2000 = %v", got)
	}
}

func BenchmarkPositions(b *testing.B) {
	e := Default()
	jd := JulianDay(time.Date(1987, 11, 29, 14, 30, 0, 0, time.UTC))
	for b.Loop() {
		e.Positions(jd)
	}
}
