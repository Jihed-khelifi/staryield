// Package ephem computes apparent geocentric positions of the Sun, the Moon
// and the planets Mercury to Pluto: ecliptic longitude and latitude referred
// to the true ecliptic and equinox of date, as seen from the Earth's centre
// (light-time and annual aberration included), plus the daily motion.
//
// Sources:
//   - planets: the VSOP87B theory (Bretagnon & Francou, A&A 202, 309, 1988),
//     embedded and truncated below 1e-8 (see data/NOTICE);
//   - Moon: the ELP-2000/82 series of Meeus, Astronomical Algorithms
//     (2nd ed.), chapter 47;
//   - Pluto: Meeus chapter 37, valid from 1885 to 2099;
//   - nutation: IAU 1980; precession: IAU 1976 (Meeus chapters 21–22).
//
// Against JPL Horizons the Sun and planets agree to about an arcsecond and
// the Moon to about ten arcseconds over 1900–2100, far below the precision
// astrology uses. Everything here is pure computation and safe for
// concurrent use once Default (or Load) has returned.
package ephem

import (
	"errors"
	"math"
	"sync"
	"time"

	"github.com/soniakeys/meeus/v3/moonposition"
	"github.com/soniakeys/meeus/v3/nutation"
	"github.com/soniakeys/meeus/v3/pluto"
)

// Body identifies a body supported by the ephemeris.
type Body int

// Supported bodies, in traditional astrological order.
const (
	Sun Body = iota
	Moon
	Mercury
	Venus
	Mars
	Jupiter
	Saturn
	Uranus
	Neptune
	Pluto

	// earth is only used internally as the observer.
	earth Body = -1
)

// Bodies lists every supported body in order.
var Bodies = [...]Body{Sun, Moon, Mercury, Venus, Mars, Jupiter, Saturn, Uranus, Neptune, Pluto}

var bodyNames = [...]string{"Sun", "Moon", "Mercury", "Venus", "Mars", "Jupiter", "Saturn", "Uranus", "Neptune", "Pluto"}

// String returns the English name of the body.
func (b Body) String() string {
	if b < Sun || b > Pluto {
		return "Earth"
	}
	return bodyNames[b]
}

// Coord is an apparent geocentric position.
type Coord struct {
	Lon   float64 // ecliptic longitude, degrees in [0, 360)
	Lat   float64 // ecliptic latitude, degrees
	Dist  float64 // distance, AU
	Speed float64 // daily motion in longitude, degrees per day (negative when retrograde)
}

// Astronomical constants.
const (
	J2000       = 2451545.0
	jdUnixEpoch = 2440587.5
	auKm        = 149597870.7
	// cLight is the speed of light in AU per day.
	cLight = 173.14463267424034
	// lightTime is the light-time per AU, in days.
	lightTime = 1 / cLight

	deg2rad = math.Pi / 180
	rad2deg = 180 / math.Pi
	arcsec  = deg2rad / 3600
)

// Validity limits (Julian Days, TT). VSOP87 is truncated for 1750–2250;
// the Pluto series covers 1885–2099.
var (
	minJD      = JulianDay(time.Date(1750, 1, 1, 0, 0, 0, 0, time.UTC))
	maxJD      = JulianDay(time.Date(2250, 1, 1, 0, 0, 0, 0, time.UTC))
	minPlutoJD = JulianDay(time.Date(1885, 1, 1, 0, 0, 0, 0, time.UTC))
	maxPlutoJD = JulianDay(time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC))
)

// ErrOutOfRange is returned for instants the theories do not cover.
var ErrOutOfRange = errors.New("ephem: instant outside the supported range")

// JulianDay returns the Julian Day (on t's own scale, normally UT) of t.
func JulianDay(t time.Time) float64 {
	return float64(t.Unix())/86400 + float64(t.Nanosecond())/86400e9 + jdUnixEpoch
}

// TimeFromJD converts a Julian Day into a UTC time (rounded to the
// nanosecond representable range).
func TimeFromJD(jd float64) time.Time {
	sec := (jd - jdUnixEpoch) * 86400
	whole := math.Floor(sec)
	return time.Unix(int64(whole), int64((sec-whole)*1e9)).UTC()
}

// Ephemeris holds the decoded planetary series.
type Ephemeris struct {
	vsop map[Body]*vsopPlanet
}

var (
	defaultOnce sync.Once
	defaultEph  *Ephemeris
	defaultErr  error
)

// Load decodes the embedded series.
func Load() (*Ephemeris, error) {
	e := &Ephemeris{vsop: make(map[Body]*vsopPlanet, len(vsopFiles))}
	for b, name := range vsopFiles {
		p, err := loadVSOP(name)
		if err != nil {
			return nil, err
		}
		e.vsop[b] = p
	}
	return e, nil
}

// Default returns the shared ephemeris, decoding the embedded data on first
// use. It panics if the embedded data is corrupt, which is a build defect.
func Default() *Ephemeris {
	defaultOnce.Do(func() { defaultEph, defaultErr = Load() })
	if defaultErr != nil {
		panic(defaultErr)
	}
	return defaultEph
}

// vec3 is a rectangular vector.
type vec3 [3]float64

func sphericalToRect(l, b, r float64) vec3 {
	sl, cl := math.Sincos(l)
	sb, cb := math.Sincos(b)
	return vec3{r * cb * cl, r * cb * sl, r * sb}
}

func (v vec3) sub(w vec3) vec3      { return vec3{v[0] - w[0], v[1] - w[1], v[2] - w[2]} }
func (v vec3) add(w vec3) vec3      { return vec3{v[0] + w[0], v[1] + w[1], v[2] + w[2]} }
func (v vec3) scale(f float64) vec3 { return vec3{v[0] * f, v[1] * f, v[2] * f} }
func (v vec3) norm() float64        { return math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2]) }

// state is the observer's situation at one TT instant.
type state struct {
	jde        float64
	t          float64 // Julian centuries of TT from J2000
	earth      vec3    // heliocentric J2000 ecliptic position, AU
	vel        vec3    // heliocentric velocity, AU/day
	dpsi, deps float64 // nutation, radians
}

// earthVelocityStep is the half-width of the central difference used for
// the Earth's velocity (days).
const earthVelocityStep = 0.005

func (e *Ephemeris) newState(jde float64, vel *vec3) state {
	s := state{jde: jde, t: (jde - J2000) / 36525}
	s.earth = e.vsop[earth].rect((jde - J2000) / 365250)
	if vel != nil {
		s.vel = *vel
	} else {
		a := e.vsop[earth].rect((jde - earthVelocityStep - J2000) / 365250)
		b := e.vsop[earth].rect((jde + earthVelocityStep - J2000) / 365250)
		s.vel = b.sub(a).scale(1 / (2 * earthVelocityStep))
	}
	dpsi, deps := nutation.Nutation(jde)
	s.dpsi, s.deps = dpsi.Rad(), deps.Rad()
	return s
}

// helio returns the heliocentric J2000 ecliptic position of a planet.
func (e *Ephemeris) helio(b Body, jde float64) vec3 {
	if b == Pluto {
		l, lat, r := pluto.Heliocentric(jde)
		return sphericalToRect(l.Rad(), lat.Rad(), r)
	}
	return e.vsop[b].rect((jde - J2000) / 365250)
}

// apparent computes the apparent geocentric longitude and latitude (radians,
// true ecliptic and equinox of date) and the distance (AU) of a body other
// than the Moon, and returns the light-time used (days). One light-time
// refinement suffices: a second one changes the result by less than 0.001″.
func (e *Ephemeris) apparent(b Body, s *state) (lon, lat, dist, tau float64) {
	var geo vec3
	if b == Sun {
		// In the heliocentric frame the Sun stays at the origin, so the
		// light-time correction vanishes; aberration is applied below.
		geo = s.earth.scale(-1)
	} else {
		geo = e.helio(b, s.jde).sub(s.earth)
		tau = geo.norm() * lightTime
		geo = e.helio(b, s.jde-tau).sub(s.earth)
	}
	lon, lat, dist = s.observe(geo)
	return lon, lat, dist, tau
}

// apparentLonWithTau returns the apparent longitude (radians) of b at s
// using a known light-time; it is used for the daily-motion samples.
func (e *Ephemeris) apparentLonWithTau(b Body, s *state, tau float64) float64 {
	var geo vec3
	if b == Sun {
		geo = s.earth.scale(-1)
	} else {
		geo = e.helio(b, s.jde-tau).sub(s.earth)
	}
	lon, _, _ := s.observe(geo)
	return lon
}

// observe converts a light-time corrected geocentric vector (heliocentric
// J2000 ecliptic axes) into apparent longitude and latitude (radians, true
// ecliptic and equinox of date) and distance (AU).
func (s *state) observe(geo vec3) (lon, lat, dist float64) {
	dist = geo.norm()
	// Annual aberration, to first order in v/c: the apparent direction is
	// displaced toward the observer's velocity.
	u := geo.scale(1 / dist).add(s.vel.scale(1 / cLight))
	l0 := math.Atan2(u[1], u[0])
	b0 := math.Atan2(u[2], math.Hypot(u[0], u[1]))
	l0, b0 = toFK5(l0, b0)
	lon, lat = precessFromJ2000(l0, b0, s.t)
	return lon + s.dpsi, lat, dist
}

// moon returns the Moon's apparent longitude, latitude (radians) and
// distance (AU) at s.
func moon(s *state) (lon, lat, dist float64) {
	l, b, delta := moonposition.Position(s.jde)
	return l.Rad() + s.dpsi, b.Rad(), delta / auKm
}

// toFK5 applies the small rotation from the VSOP87 dynamical frame of J2000
// to FK5 (Meeus 32.3 with T = 0).
func toFK5(l, b float64) (float64, float64) {
	sl, cl := math.Sincos(l)
	dl := -0.09033*arcsec + 0.03916*arcsec*(cl+sl)*math.Tan(b)
	db := 0.03916 * arcsec * (cl - sl)
	return l + dl, b + db
}

// precessFromJ2000 precesses ecliptic coordinates from the ecliptic and
// equinox of J2000 to those of the date t Julian centuries later (Meeus
// 21.5 and 21.7, IAU 1976).
func precessFromJ2000(l0, b0, t float64) (l, b float64) {
	eta := (47.0029*t - 0.03302*t*t + 0.000060*t*t*t) * arcsec
	pi := 174.876384*deg2rad + (-869.8089*t+0.03536*t*t)*arcsec
	p := (5029.0966*t + 1.11113*t*t - 0.000006*t*t*t) * arcsec
	sEta, cEta := math.Sincos(eta)
	sb, cb := math.Sincos(b0)
	spl, cpl := math.Sincos(pi - l0)
	a := cEta*cb*spl - sEta*sb
	bb := cb * cpl
	c := cEta*sb + sEta*cb*spl
	return p + pi - math.Atan2(a, bb), math.Asin(math.Max(-1, math.Min(1, c)))
}

// speedStep returns the half-width (days) of the central difference used
// for a body's daily motion.
func speedStep(b Body) float64 {
	if b == Moon {
		return 1.0 / 24
	}
	return 0.5
}

func checkRange(b Body, jde float64) error {
	if b < Sun || b > Pluto {
		return errors.New("ephem: unknown body")
	}
	if jde < minJD || jde > maxJD {
		return ErrOutOfRange
	}
	if b == Pluto && (jde < minPlutoJD || jde > maxPlutoJD) {
		return ErrOutOfRange
	}
	return nil
}

// Position returns the apparent geocentric position of b at jdUT (Julian Day
// on the UT scale).
func (e *Ephemeris) Position(b Body, jdUT float64) (Coord, error) {
	all, errs := e.positions([]Body{b}, jdUT)
	return all[0], errs[0]
}

// Positions returns the apparent geocentric positions of every body in
// Bodies at jdUT, sharing the observer's state between them. A body outside
// its validity range (only Pluto before 1885 or after 2099) gets a zero
// Coord and ErrOutOfRange in the matching slot of errs.
func (e *Ephemeris) Positions(jdUT float64) (out [len(Bodies)]Coord, errs [len(Bodies)]error) {
	c, er := e.positions(Bodies[:], jdUT)
	copy(out[:], c)
	copy(errs[:], er)
	return out, errs
}

func (e *Ephemeris) positions(bodies []Body, jdUT float64) ([]Coord, []error) {
	out := make([]Coord, len(bodies))
	errs := make([]error, len(bodies))
	jde := JDE(jdUT)
	if jde < minJD || jde > maxJD {
		for i := range errs {
			errs[i] = ErrOutOfRange
		}
		return out, errs
	}
	centre := e.newState(jde, nil)
	// Neighbouring states for the daily motion share the centre's velocity
	// (aberration changes negligibly within a day).
	h := speedStep(Sun)
	before := e.newState(jde-h, &centre.vel)
	after := e.newState(jde+h, &centre.vel)
	var moonBefore, moonAfter *state
	for i, b := range bodies {
		if err := checkRange(b, jde); err != nil {
			errs[i] = err
			continue
		}
		var lon, lat, dist, l1, l2 float64
		step := speedStep(b)
		if b == Moon {
			lon, lat, dist = moon(&centre)
			if moonBefore == nil {
				mb := state{jde: jde - step, dpsi: centre.dpsi}
				ma := state{jde: jde + step, dpsi: centre.dpsi}
				moonBefore, moonAfter = &mb, &ma
			}
			l1, _, _ = moon(moonBefore)
			l2, _, _ = moon(moonAfter)
		} else {
			var tau float64
			lon, lat, dist, tau = e.apparent(b, &centre)
			l1 = e.apparentLonWithTau(b, &before, tau)
			l2 = e.apparentLonWithTau(b, &after, tau)
		}
		out[i] = Coord{
			Lon:   norm360(lon * rad2deg),
			Lat:   lat * rad2deg,
			Dist:  dist,
			Speed: wrap180((l2-l1)*rad2deg) / (2 * step),
		}
	}
	return out, errs
}

// Nutation returns the nutation in longitude and in obliquity, in degrees,
// at jde (IAU 1980).
func Nutation(jde float64) (dpsi, deps float64) {
	p, e := nutation.Nutation(jde)
	return p.Deg(), e.Deg()
}

// MeanObliquity returns the mean obliquity of the ecliptic at jde, in
// degrees (IAU).
func MeanObliquity(jde float64) float64 { return nutation.MeanObliquity(jde).Deg() }

// TrueObliquity returns the true obliquity of the ecliptic (mean plus
// nutation) at jde, in degrees.
func TrueObliquity(jde float64) float64 {
	_, deps := Nutation(jde)
	return MeanObliquity(jde) + deps
}

// MeanSiderealTime returns the Greenwich mean sidereal time at jdUT, in
// degrees (IAU 1982).
func MeanSiderealTime(jdUT float64) float64 {
	T := (jdUT - J2000) / 36525
	return norm360(280.46061837 + 360.98564736629*(jdUT-J2000) + 0.000387933*T*T - T*T*T/38710000)
}

// ApparentSiderealTime returns the Greenwich apparent sidereal time at jdUT,
// in degrees: mean sidereal time plus the equation of the equinoxes.
func ApparentSiderealTime(jdUT float64) float64 {
	jde := JDE(jdUT)
	dpsi, deps := Nutation(jde)
	eps := MeanObliquity(jde) + deps
	return norm360(MeanSiderealTime(jdUT) + dpsi*math.Cos(eps*deg2rad))
}

// norm360 maps an angle in degrees to [0, 360).
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

// wrap180 maps an angle in degrees to (-180, 180].
func wrap180(x float64) float64 {
	x = norm360(x)
	if x > 180 {
		x -= 360
	}
	return x
}
