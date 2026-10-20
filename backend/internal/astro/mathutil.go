package astro

import (
	"math"
	"time"
)

const (
	deg2rad = math.Pi / 180
	rad2deg = 180 / math.Pi

	// jdUnixEpoch is the Julian Day of 1970-01-01T00:00:00Z.
	jdUnixEpoch = 2440587.5
	// jdSchlyterEpoch is the Julian Day of Schlyter's day zero,
	// 1999-12-31T00:00:00 (i.e. "2000 Jan 0.0").
	jdSchlyterEpoch = 2451543.5
	// jdJ2000 is the Julian Day of the J2000.0 epoch.
	jdJ2000 = 2451545.0
)

func sind(x float64) float64 { return math.Sin(x * deg2rad) }
func cosd(x float64) float64 { return math.Cos(x * deg2rad) }
func tand(x float64) float64 { return math.Tan(x * deg2rad) }

func atan2d(y, x float64) float64 { return math.Atan2(y, x) * rad2deg }

// norm360 maps an angle in degrees to the half-open range [0, 360).
func norm360(x float64) float64 {
	x = math.Mod(x, 360)
	if x < 0 {
		x += 360
	}
	if x >= 360 {
		x = 0
	}
	return x
}

// wrap180 maps an angle in degrees to the range (-180, 180].
func wrap180(x float64) float64 {
	x = norm360(x)
	if x > 180 {
		x -= 360
	}
	return x
}

// roundTo rounds x to the given number of decimal places.
func roundTo(x float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(x*p) / p
}

// julianDayUT returns the Julian Day (UT scale) of t.
func julianDayUT(t time.Time) float64 {
	return float64(t.Unix())/86400 + float64(t.Nanosecond())/86400e9 + jdUnixEpoch
}

// dayNumber returns Schlyter's day number d (days since 2000 Jan 0.0) on the
// Terrestrial Time scale, which is what the orbital elements expect.
func dayNumber(t time.Time) float64 {
	jd := julianDayUT(t)
	return jd + deltaT(jd)/86400 - jdSchlyterEpoch
}

// deltaT approximates TT − UT in seconds for the given Julian Day using the
// Espenak & Meeus polynomial expressions. The difference is about a minute in
// the modern era, which moves the Moon by roughly 0.01°.
func deltaT(jd float64) float64 {
	y := 2000 + (jd-jdJ2000)/365.25
	switch {
	case y < 1860:
		u := (y - 1820) / 100
		return -20 + 32*u*u
	case y < 1900:
		t := y - 1860
		return 7.62 + 0.5737*t - 0.251754*t*t + 0.01680668*t*t*t -
			0.0004473624*t*t*t*t + t*t*t*t*t/233174
	case y < 1920:
		t := y - 1900
		return -2.79 + 1.494119*t - 0.0598939*t*t + 0.0061966*t*t*t - 0.000197*t*t*t*t
	case y < 1941:
		t := y - 1920
		return 21.20 + 0.84493*t - 0.076100*t*t + 0.0020936*t*t*t
	case y < 1961:
		t := y - 1950
		return 29.07 + 0.407*t - t*t/233 + t*t*t/2547
	case y < 1986:
		t := y - 1975
		return 45.45 + 1.067*t - t*t/260 - t*t*t/718
	case y < 2005:
		t := y - 2000
		return 63.86 + 0.3345*t - 0.060374*t*t + 0.0017275*t*t*t +
			0.000651814*t*t*t*t + 0.00002373599*t*t*t*t*t
	case y < 2050:
		t := y - 2000
		return 62.92 + 0.32217*t + 0.005589*t*t
	case y < 2150:
		u := (y - 1820) / 100
		return -20 + 32*u*u - 0.5628*(2150-y)
	default:
		u := (y - 1820) / 100
		return -20 + 32*u*u
	}
}
