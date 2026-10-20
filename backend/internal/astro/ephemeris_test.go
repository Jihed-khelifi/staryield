package astro

import (
	"math"
	"testing"
	"time"
)

// horizonsReference holds apparent geocentric ecliptic-of-date longitudes
// (degrees) from JPL Horizons (observer table, quantity 31, geocentre) for
// Sun, Moon, Mercury, Venus, Mars, Jupiter, Saturn, Uranus, Neptune and
// Pluto, in that order, at each UTC instant.
var horizonsReference = []struct {
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

// ephemerisTolerance is the maximum accepted error per body, in degrees.
// The product targets are 0.3° for the Sun and Moon and 1° for planets; the
// measured errors are an order of magnitude smaller, so the tolerances here
// are tighter to catch regressions.
var ephemerisTolerance = map[Body]float64{
	Sun: 0.05, Moon: 0.2, Mercury: 0.1, Venus: 0.1, Mars: 0.1,
	Jupiter: 0.1, Saturn: 0.1, Uranus: 0.1, Neptune: 0.1, Pluto: 0.1,
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return v
}

func angleDiff(a, b float64) float64 { return math.Abs(wrap180(a - b)) }

func TestEclipticLongitudeAgainstHorizons(t *testing.T) {
	maxErr := map[Body]float64{}
	sumErr := map[Body]float64{}
	for _, ref := range horizonsReference {
		at := mustTime(t, ref.instant)
		for i, b := range Planets {
			got := EclipticLongitude(b, at)
			if got < 0 || got >= 360 {
				t.Errorf("%s %s: longitude %v out of [0,360)", b, ref.instant, got)
			}
			err := angleDiff(got, ref.lon[i])
			sumErr[b] += err
			if err > maxErr[b] {
				maxErr[b] = err
			}
			if err > ephemerisTolerance[b] {
				t.Errorf("%s at %s: got %.4f°, want %.4f° (error %.4f° > %.2f°)",
					b, ref.instant, got, ref.lon[i], err, ephemerisTolerance[b])
			}
		}
	}
	for _, b := range Planets {
		t.Logf("%-8s max error %.4f° (%.1f′), mean %.4f°", b, maxErr[b], maxErr[b]*60, sumErr[b]/float64(len(horizonsReference)))
	}
}

// ttToUTC converts a Terrestrial Time calendar instant to UTC using the
// package's ΔT model.
func ttToUTC(tt time.Time) time.Time {
	jd := julianDayUT(tt)
	return tt.Add(-time.Duration(deltaT(jd) * float64(time.Second)))
}

func TestEclipticLongitudeReferenceValues(t *testing.T) {
	tests := []struct {
		name string
		body Body
		at   time.Time
		want float64
		tol  float64
	}{
		// J2000.0 (2000-01-01 12:00 TT): the Sun's mean longitude is 280.46°
		// and its apparent longitude 280.37°.
		{"Sun at J2000.0 (mean)", Sun, ttToUTC(time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)), 280.46, 0.3},
		{"Sun at J2000.0 (apparent)", Sun, ttToUTC(time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)), 280.37, 0.02},
		// Meeus, Astronomical Algorithms, examples 25.a, 47.a and 33.a.
		{"Sun Meeus 25.a", Sun, ttToUTC(time.Date(1992, 10, 13, 0, 0, 0, 0, time.UTC)), 199.90895, 0.05},
		{"Moon Meeus 47.a", Moon, ttToUTC(time.Date(1992, 4, 12, 0, 0, 0, 0, time.UTC)), 133.167265, 0.3},
		{"Venus Meeus 33.a", Venus, ttToUTC(time.Date(1992, 12, 20, 0, 0, 0, 0, time.UTC)), 313.08102, 0.3},
		// Sign ingresses: the body sits on a sign boundary at these moments.
		{"Sun at March equinox 2024", Sun, time.Date(2024, 3, 20, 3, 6, 0, 0, time.UTC), 0, 0.02},
		{"Pluto enters Aquarius 2024", Pluto, time.Date(2024, 11, 19, 20, 30, 0, 0, time.UTC), 300, 0.2},
		{"Neptune enters Aries 2025", Neptune, time.Date(2025, 3, 30, 12, 0, 0, 0, time.UTC), 0, 0.2},
		{"Saturn enters Aries 2025", Saturn, time.Date(2025, 5, 25, 3, 35, 0, 0, time.UTC), 0, 0.2},
		{"Uranus enters Gemini 2025", Uranus, time.Date(2025, 7, 7, 7, 45, 0, 0, time.UTC), 60, 0.2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EclipticLongitude(tt.body, tt.at)
			if d := angleDiff(got, tt.want); d > tt.tol {
				t.Errorf("got %.4f°, want %.4f° ± %.2f° (off by %.4f°)", got, tt.want, tt.tol, d)
			} else {
				t.Logf("got %.4f°, want %.4f° (off by %.4f°)", got, tt.want, d)
			}
		})
	}
}

func TestEclipticLongitudeNonPlanet(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, b := range []Body{Ascendant, Midheaven, Body("Chiron")} {
		if got := EclipticLongitude(b, now); !math.IsNaN(got) {
			t.Errorf("EclipticLongitude(%s) = %v, want NaN", b, got)
		}
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

func TestRetrogradeWrapAround(t *testing.T) {
	// Neptune moves retrograde from Aries back into Pisces on 2025-10-22,
	// crossing 0°/360°; the motion test must not be confused by the wrap.
	at := time.Date(2025, 10, 22, 9, 0, 0, 0, time.UTC)
	lon := EclipticLongitude(Neptune, at)
	if math.Abs(wrap180(lon)) > 0.05 {
		t.Fatalf("test assumption broken: Neptune at %.3f°", lon)
	}
	if !IsRetrograde(Neptune, at) {
		t.Errorf("Neptune should be retrograde on %s (lon %.3f°)", at.Format(time.DateOnly), lon)
	}
}

func TestDeltaT(t *testing.T) {
	tests := []struct {
		year      int
		want, tol float64
	}{
		{1900, -2.8, 1}, {1950, 29.1, 1}, {1975, 45.5, 1}, {2000, 63.8, 1}, {2025, 70, 6},
	}
	for _, tt := range tests {
		jd := julianDayUT(time.Date(tt.year, 1, 1, 0, 0, 0, 0, time.UTC))
		if got := deltaT(jd); math.Abs(got-tt.want) > tt.tol {
			t.Errorf("deltaT(%d) = %.2f s, want %.1f ± %.0f", tt.year, got, tt.want, tt.tol)
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
