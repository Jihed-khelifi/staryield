package ephem

import "math"

// FastLongitude returns a low-precision geocentric ecliptic longitude
// (degrees, mean equinox of date) of b at jdUT, using Paul Schlyter's "How to
// compute planetary positions" method (Keplerian elements plus the largest
// lunar and Jupiter/Saturn/Uranus perturbations). It is accurate to a few
// arcminutes between about 1900 and 2100 and roughly fifty times cheaper than
// Position, which makes it suitable for coarse scans (phase and station
// searches) whose results are then refined with Position.
func FastLongitude(b Body, jdUT float64) float64 {
	d := jdUT + DeltaT(jdUT)/86400 - jdSchlyterEpoch
	switch b {
	case Sun:
		lon, _ := fastSun(d)
		return lon
	case Moon:
		return fastMoon(d)
	case Mercury, Venus, Mars, Jupiter, Saturn, Uranus, Neptune, Pluto:
		return fastPlanet(b, d)
	}
	return math.NaN()
}

// jdSchlyterEpoch is Schlyter's day zero, 1999-12-31T00:00 ("2000 Jan 0.0").
const jdSchlyterEpoch = 2451543.5

func sind(x float64) float64      { return math.Sin(x * deg2rad) }
func cosd(x float64) float64      { return math.Cos(x * deg2rad) }
func atan2d(y, x float64) float64 { return math.Atan2(y, x) * rad2deg }

type orbit struct{ N, i, w, a, e, M float64 }

func orbitFor(b Body, d float64) orbit {
	switch b {
	case Mercury:
		return orbit{48.3313 + 3.24587e-5*d, 7.0047 + 5.00e-8*d, 29.1241 + 1.01444e-5*d, 0.387098, 0.205635 + 5.59e-10*d, 168.6562 + 4.0923344368*d}
	case Venus:
		return orbit{76.6799 + 2.46590e-5*d, 3.3946 + 2.75e-8*d, 54.8910 + 1.38374e-5*d, 0.723330, 0.006773 - 1.302e-9*d, 48.0052 + 1.6021302244*d}
	case Mars:
		return orbit{49.5574 + 2.11081e-5*d, 1.8497 - 1.78e-8*d, 286.5016 + 2.92961e-5*d, 1.523688, 0.093405 + 2.516e-9*d, 18.6021 + 0.5240207766*d}
	case Jupiter:
		return orbit{100.4542 + 2.76854e-5*d, 1.3030 - 1.557e-7*d, 273.8777 + 1.64505e-5*d, 5.20256, 0.048498 + 4.469e-9*d, 19.8950 + 0.0830853001*d}
	case Saturn:
		return orbit{113.6634 + 2.38980e-5*d, 2.4886 - 1.081e-7*d, 339.3939 + 2.97661e-5*d, 9.55475, 0.055546 - 9.499e-9*d, 316.9670 + 0.0334442282*d}
	case Uranus:
		return orbit{74.0005 + 1.3978e-5*d, 0.7733 + 1.9e-8*d, 96.6612 + 3.0565e-5*d, 19.18171 - 1.55e-8*d, 0.047318 + 7.45e-9*d, 142.5905 + 0.011725806*d}
	case Neptune:
		return orbit{131.7806 + 3.0173e-5*d, 1.7700 - 2.55e-7*d, 272.8461 - 6.027e-6*d, 30.05826 + 3.313e-8*d, 0.008606 + 2.15e-9*d, 260.2471 + 0.005995147*d}
	}
	panic("ephem: no orbital elements for " + b.String())
}

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

func orbitalPosition(o orbit) (v, r float64) {
	E := eccentricAnomaly(o.M, o.e)
	xv := o.a * (math.Cos(E) - o.e)
	yv := o.a * math.Sqrt(1-o.e*o.e) * math.Sin(E)
	return atan2d(yv, xv), math.Hypot(xv, yv)
}

func eclipticFromOrbit(o orbit) (lon, lat, r float64) {
	v, r := orbitalPosition(o)
	vw := v + o.w
	x := r * (cosd(o.N)*cosd(vw) - sind(o.N)*sind(vw)*cosd(o.i))
	y := r * (sind(o.N)*cosd(vw) + cosd(o.N)*sind(vw)*cosd(o.i))
	z := r * sind(vw) * sind(o.i)
	return norm360(atan2d(y, x)), atan2d(z, math.Hypot(x, y)), r
}

func fastSunElements(d float64) (w, e, M float64) {
	return 282.9404 + 4.70935e-5*d, 0.016709 - 1.151e-9*d, 356.0470 + 0.9856002585*d
}

func fastSun(d float64) (lon, r float64) {
	w, e, M := fastSunElements(d)
	v, r := orbitalPosition(orbit{a: 1, e: e, M: M})
	return norm360(v + w), r
}

func fastMoon(d float64) float64 {
	o := orbit{125.1228 - 0.0529538083*d, 5.1454, 318.0634 + 0.1643573223*d, 60.2666, 0.054900, 115.3654 + 13.0649929509*d}
	lon, _, _ := eclipticFromOrbit(o)
	ws, _, ms := fastSunElements(d)
	mm := o.M
	ls := ms + ws
	lm := mm + o.w + o.N
	D := lm - ls
	F := lm - o.N
	lon += -1.274*sind(mm-2*D) + 0.658*sind(2*D) - 0.186*sind(ms) - 0.059*sind(2*mm-2*D) -
		0.057*sind(mm-2*D+ms) + 0.053*sind(mm+2*D) + 0.046*sind(2*D-ms) + 0.041*sind(mm-ms) -
		0.035*sind(D) - 0.031*sind(mm+ms) - 0.015*sind(2*F-2*D) + 0.011*sind(mm-4*D)
	return norm360(lon)
}

func fastHeliocentric(b Body, d float64) (lon, lat, r float64) {
	if b == Pluto {
		S := 50.03 + 0.033459652*d
		P := 238.95 + 0.003968789*d
		lon = 238.9508 + 0.00400703*d - 19.799*sind(P) + 19.848*cosd(P) + 0.897*sind(2*P) - 4.956*cosd(2*P) +
			0.610*sind(3*P) + 1.211*cosd(3*P) - 0.341*sind(4*P) - 0.190*cosd(4*P) + 0.128*sind(5*P) -
			0.034*cosd(5*P) - 0.038*sind(6*P) + 0.031*cosd(6*P) + 0.020*sind(S-P) - 0.010*cosd(S-P)
		lat = -3.9082 - 5.453*sind(P) - 14.975*cosd(P) + 3.527*sind(2*P) + 1.673*cosd(2*P) - 1.051*sind(3*P) +
			0.328*cosd(3*P) + 0.179*sind(4*P) - 0.292*cosd(4*P) + 0.019*sind(5*P) + 0.100*cosd(5*P) -
			0.031*sind(6*P) - 0.026*cosd(6*P) + 0.011*cosd(S-P)
		r = 40.72 + 6.68*sind(P) + 6.90*cosd(P) - 1.18*sind(2*P) - 0.03*cosd(2*P) + 0.15*sind(3*P) - 0.14*cosd(3*P)
		return norm360(lon), lat, r
	}
	lon, lat, r = eclipticFromOrbit(orbitFor(b, d))
	mj := 19.8950 + 0.0830853001*d
	ms := 316.9670 + 0.0334442282*d
	mu := 142.5905 + 0.011725806*d
	switch b {
	case Jupiter:
		lon += -0.332*sind(2*mj-5*ms-67.6) - 0.056*sind(2*mj-2*ms+21) + 0.042*sind(3*mj-5*ms+21) -
			0.036*sind(mj-2*ms) + 0.022*cosd(mj-ms) + 0.023*sind(2*mj-3*ms+52) - 0.016*sind(mj-5*ms-69)
	case Saturn:
		lon += 0.812*sind(2*mj-5*ms-67.6) - 0.229*cosd(2*mj-4*ms-2) + 0.119*sind(mj-2*ms-3) +
			0.046*sind(2*mj-6*ms-69) + 0.014*sind(mj-3*ms+32)
		lat += -0.020*cosd(2*mj-4*ms-2) + 0.018*sind(2*mj-6*ms-49)
	case Uranus:
		lon += 0.040*sind(ms-2*mu+6) + 0.035*sind(ms-3*mu+33) - 0.015*sind(mj-mu+20)
	}
	return norm360(lon), lat, r
}

func fastPlanet(b Body, d float64) float64 {
	lon, lat, r := fastHeliocentric(b, d)
	x := r * cosd(lon) * cosd(lat)
	y := r * sind(lon) * cosd(lat)
	slon, sr := fastSun(d)
	x += sr * cosd(slon)
	y += sr * sind(slon)
	return norm360(atan2d(y, x))
}
