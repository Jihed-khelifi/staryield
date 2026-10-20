package astro

import (
	"math"
	"time"
)

// Body identifies a celestial body or a calculated chart point.
type Body string

// Bodies and chart points supported by the package.
const (
	Sun       Body = "Sun"
	Moon      Body = "Moon"
	Mercury   Body = "Mercury"
	Venus     Body = "Venus"
	Mars      Body = "Mars"
	Jupiter   Body = "Jupiter"
	Saturn    Body = "Saturn"
	Uranus    Body = "Uranus"
	Neptune   Body = "Neptune"
	Pluto     Body = "Pluto"
	Ascendant Body = "Ascendant"
	Midheaven Body = "Midheaven"
)

// Planets lists the ten bodies the ephemeris can compute, in traditional
// order. ("Planet" is used in the astrological sense and includes the Sun and
// Moon.)
var Planets = [10]Body{Sun, Moon, Mercury, Venus, Mars, Jupiter, Saturn, Uranus, Neptune, Pluto}

// IsPlanet reports whether b is one of the ten bodies in Planets.
func IsPlanet(b Body) bool {
	for _, p := range Planets {
		if p == b {
			return true
		}
	}
	return false
}

// EclipticLongitude returns the geocentric tropical ecliptic longitude of b,
// in degrees within [0, 360), at instant t (referred to the mean equinox of
// date). It only supports the ten bodies in Planets; for any other value,
// including Ascendant and Midheaven (which also need a location), it returns
// NaN.
func EclipticLongitude(b Body, t time.Time) float64 {
	d := dayNumber(t)
	switch b {
	case Sun:
		lon, _ := sunPosition(d)
		return lon
	case Moon:
		return moonLongitude(d)
	case Mercury, Venus, Mars, Jupiter, Saturn, Uranus, Neptune, Pluto:
		return planetLongitude(b, d)
	default:
		return math.NaN()
	}
}

// retrogradeHalfWindow is half the interval over which apparent motion is
// measured when deciding whether a body is retrograde.
const retrogradeHalfWindow = 3 * time.Hour

// IsRetrograde reports whether b appears to move backwards through the zodiac
// at t, by comparing its longitude a few hours before and after t (with
// 0°/360° wrap handled). The Sun and Moon are never retrograde, and points
// other than the ten planets always report false.
func IsRetrograde(b Body, t time.Time) bool {
	if !canRetrograde(b) {
		return false
	}
	return apparentMotion(b, t) < 0
}

func canRetrograde(b Body) bool {
	switch b {
	case Mercury, Venus, Mars, Jupiter, Saturn, Uranus, Neptune, Pluto:
		return true
	}
	return false
}

// apparentMotion returns the signed longitude change of b across a short
// window centred on t, in degrees.
func apparentMotion(b Body, t time.Time) float64 {
	before := EclipticLongitude(b, t.Add(-retrogradeHalfWindow))
	after := EclipticLongitude(b, t.Add(retrogradeHalfWindow))
	return wrap180(after - before)
}

// orbit holds Keplerian elements in Schlyter's notation: longitude of the
// ascending node N, inclination i, argument of perihelion w (all degrees),
// semi-major axis a, eccentricity e and mean anomaly M (degrees).
type orbit struct {
	N, i, w, a, e, M float64
}

// orbitFor returns the osculating elements of a planet at day number d,
// referred to the ecliptic and equinox of date.
func orbitFor(b Body, d float64) orbit {
	switch b {
	case Mercury:
		return orbit{
			N: 48.3313 + 3.24587e-5*d, i: 7.0047 + 5.00e-8*d, w: 29.1241 + 1.01444e-5*d,
			a: 0.387098, e: 0.205635 + 5.59e-10*d, M: 168.6562 + 4.0923344368*d,
		}
	case Venus:
		return orbit{
			N: 76.6799 + 2.46590e-5*d, i: 3.3946 + 2.75e-8*d, w: 54.8910 + 1.38374e-5*d,
			a: 0.723330, e: 0.006773 - 1.302e-9*d, M: 48.0052 + 1.6021302244*d,
		}
	case Mars:
		return orbit{
			N: 49.5574 + 2.11081e-5*d, i: 1.8497 - 1.78e-8*d, w: 286.5016 + 2.92961e-5*d,
			a: 1.523688, e: 0.093405 + 2.516e-9*d, M: 18.6021 + 0.5240207766*d,
		}
	case Jupiter:
		return orbit{
			N: 100.4542 + 2.76854e-5*d, i: 1.3030 - 1.557e-7*d, w: 273.8777 + 1.64505e-5*d,
			a: 5.20256, e: 0.048498 + 4.469e-9*d, M: 19.8950 + 0.0830853001*d,
		}
	case Saturn:
		return orbit{
			N: 113.6634 + 2.38980e-5*d, i: 2.4886 - 1.081e-7*d, w: 339.3939 + 2.97661e-5*d,
			a: 9.55475, e: 0.055546 - 9.499e-9*d, M: 316.9670 + 0.0334442282*d,
		}
	case Uranus:
		return orbit{
			N: 74.0005 + 1.3978e-5*d, i: 0.7733 + 1.9e-8*d, w: 96.6612 + 3.0565e-5*d,
			a: 19.18171 - 1.55e-8*d, e: 0.047318 + 7.45e-9*d, M: 142.5905 + 0.011725806*d,
		}
	case Neptune:
		return orbit{
			N: 131.7806 + 3.0173e-5*d, i: 1.7700 - 2.55e-7*d, w: 272.8461 - 6.027e-6*d,
			a: 30.05826 + 3.313e-8*d, e: 0.008606 + 2.15e-9*d, M: 260.2471 + 0.005995147*d,
		}
	}
	panic("astro: no orbital elements for " + string(b))
}

// eccentricAnomaly solves Kepler's equation M = E − e·sin E by Newton
// iteration. M is in degrees; the result is in radians.
func eccentricAnomaly(mDeg, e float64) float64 {
	m := norm360(mDeg) * deg2rad
	E := m + e*math.Sin(m)*(1+e*math.Cos(m))
	for range 30 {
		dE := (E - e*math.Sin(E) - m) / (1 - e*math.Cos(E))
		E -= dE
		if math.Abs(dE) < 1e-12 {
			break
		}
	}
	return E
}

// orbitalPosition returns the true anomaly v (degrees) and radius vector r
// for the given elements.
func orbitalPosition(o orbit) (v, r float64) {
	E := eccentricAnomaly(o.M, o.e)
	xv := o.a * (math.Cos(E) - o.e)
	yv := o.a * math.Sqrt(1-o.e*o.e) * math.Sin(E)
	return atan2d(yv, xv), math.Hypot(xv, yv)
}

// eclipticFromOrbit converts elements into ecliptic longitude and latitude
// (degrees) and distance, centred on the attracting body.
func eclipticFromOrbit(o orbit) (lon, lat, r float64) {
	v, r := orbitalPosition(o)
	vw := v + o.w
	x := r * (cosd(o.N)*cosd(vw) - sind(o.N)*sind(vw)*cosd(o.i))
	y := r * (sind(o.N)*cosd(vw) + cosd(o.N)*sind(vw)*cosd(o.i))
	z := r * sind(vw) * sind(o.i)
	return norm360(atan2d(y, x)), atan2d(z, math.Hypot(x, y)), r
}

// sunElements returns the argument of perihelion, eccentricity and mean
// anomaly of the Sun's apparent orbit around the Earth.
func sunElements(d float64) (w, e, M float64) {
	return 282.9404 + 4.70935e-5*d, 0.016709 - 1.151e-9*d, 356.0470 + 0.9856002585*d
}

// sunPosition returns the Sun's geocentric ecliptic longitude (degrees) and
// distance (AU) at day number d.
func sunPosition(d float64) (lon, r float64) {
	w, e, M := sunElements(d)
	v, r := orbitalPosition(orbit{a: 1, e: e, M: M})
	return norm360(v + w), r
}

// moonLongitude returns the Moon's geocentric ecliptic longitude at day
// number d, including Schlyter's twelve largest periodic terms.
func moonLongitude(d float64) float64 {
	o := orbit{
		N: 125.1228 - 0.0529538083*d,
		i: 5.1454,
		w: 318.0634 + 0.1643573223*d,
		a: 60.2666,
		e: 0.054900,
		M: 115.3654 + 13.0649929509*d,
	}
	lon, _, _ := eclipticFromOrbit(o)

	ws, _, ms := sunElements(d)
	mm := o.M
	ls := ms + ws        // Sun's mean longitude
	lm := mm + o.w + o.N // Moon's mean longitude
	D := lm - ls         // mean elongation
	F := lm - o.N        // argument of latitude

	lon += -1.274*sind(mm-2*D) + // evection
		0.658*sind(2*D) + // variation
		-0.186*sind(ms) + // yearly equation
		-0.059*sind(2*mm-2*D) +
		-0.057*sind(mm-2*D+ms) +
		0.053*sind(mm+2*D) +
		0.046*sind(2*D-ms) +
		0.041*sind(mm-ms) +
		-0.035*sind(D) + // parallactic equation
		-0.031*sind(mm+ms) +
		-0.015*sind(2*F-2*D) +
		0.011*sind(mm-4*D)
	return norm360(lon)
}

// heliocentric returns a planet's heliocentric ecliptic longitude, latitude
// (degrees) and distance (AU) at day number d, with perturbations applied.
func heliocentric(b Body, d float64) (lon, lat, r float64) {
	if b == Pluto {
		return plutoHeliocentric(d)
	}
	lon, lat, r = eclipticFromOrbit(orbitFor(b, d))

	mj := 19.8950 + 0.0830853001*d  // Jupiter mean anomaly
	ms := 316.9670 + 0.0334442282*d // Saturn mean anomaly
	mu := 142.5905 + 0.011725806*d  // Uranus mean anomaly
	switch b {
	case Jupiter:
		lon += -0.332*sind(2*mj-5*ms-67.6) +
			-0.056*sind(2*mj-2*ms+21) +
			0.042*sind(3*mj-5*ms+21) +
			-0.036*sind(mj-2*ms) +
			0.022*cosd(mj-ms) +
			0.023*sind(2*mj-3*ms+52) +
			-0.016*sind(mj-5*ms-69)
	case Saturn:
		lon += 0.812*sind(2*mj-5*ms-67.6) +
			-0.229*cosd(2*mj-4*ms-2) +
			0.119*sind(mj-2*ms-3) +
			0.046*sind(2*mj-6*ms-69) +
			0.014*sind(mj-3*ms+32)
		lat += -0.020*cosd(2*mj-4*ms-2) +
			0.018*sind(2*mj-6*ms-49)
	case Uranus:
		lon += 0.040*sind(ms-2*mu+6) +
			0.035*sind(ms-3*mu+33) +
			-0.015*sind(mj-mu+20)
	}
	return norm360(lon), lat, r
}

// plutoHeliocentric evaluates Schlyter's periodic fit for Pluto (valid
// roughly 1885–2099). Its secular term already includes general precession,
// so the result is referred to the equinox of date like the other bodies.
func plutoHeliocentric(d float64) (lon, lat, r float64) {
	S := 50.03 + 0.033459652*d
	P := 238.95 + 0.003968789*d

	lon = 238.9508 + 0.00400703*d +
		-19.799*sind(P) + 19.848*cosd(P) +
		0.897*sind(2*P) - 4.956*cosd(2*P) +
		0.610*sind(3*P) + 1.211*cosd(3*P) +
		-0.341*sind(4*P) - 0.190*cosd(4*P) +
		0.128*sind(5*P) - 0.034*cosd(5*P) +
		-0.038*sind(6*P) + 0.031*cosd(6*P) +
		0.020*sind(S-P) - 0.010*cosd(S-P)
	lat = -3.9082 +
		-5.453*sind(P) - 14.975*cosd(P) +
		3.527*sind(2*P) + 1.673*cosd(2*P) +
		-1.051*sind(3*P) + 0.328*cosd(3*P) +
		0.179*sind(4*P) - 0.292*cosd(4*P) +
		0.019*sind(5*P) + 0.100*cosd(5*P) +
		-0.031*sind(6*P) - 0.026*cosd(6*P) +
		0.011*cosd(S-P)
	r = 40.72 +
		6.68*sind(P) + 6.90*cosd(P) +
		-1.18*sind(2*P) - 0.03*cosd(2*P) +
		0.15*sind(3*P) - 0.14*cosd(3*P)
	return norm360(lon), lat, r
}

// planetLongitude converts a planet's heliocentric position to a geocentric
// ecliptic longitude by adding the Sun's geocentric position vector.
func planetLongitude(b Body, d float64) float64 {
	lon, lat, r := heliocentric(b, d)
	x := r * cosd(lon) * cosd(lat)
	y := r * sind(lon) * cosd(lat)

	slon, sr := sunPosition(d)
	x += sr * cosd(slon)
	y += sr * sind(slon)
	return norm360(atan2d(y, x))
}

// obliquity returns the mean obliquity of the ecliptic at day number d.
func obliquity(d float64) float64 {
	return 23.4393 - 3.563e-7*d
}
