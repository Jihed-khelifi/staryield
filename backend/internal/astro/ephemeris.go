package astro

import (
	"math"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/astro/ephem"
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

var ephemBodies = map[Body]ephem.Body{
	Sun: ephem.Sun, Moon: ephem.Moon, Mercury: ephem.Mercury, Venus: ephem.Venus, Mars: ephem.Mars,
	Jupiter: ephem.Jupiter, Saturn: ephem.Saturn, Uranus: ephem.Uranus, Neptune: ephem.Neptune, Pluto: ephem.Pluto,
}

// IsPlanet reports whether b is one of the ten bodies in Planets.
func IsPlanet(b Body) bool {
	_, ok := ephemBodies[b]
	return ok
}

// planetPosition returns the apparent geocentric position of a planet.
func planetPosition(b Body, t time.Time) (ephem.Coord, bool) {
	eb, ok := ephemBodies[b]
	if !ok {
		return ephem.Coord{}, false
	}
	c, err := ephem.Default().Position(eb, ephem.JulianDay(t))
	if err != nil {
		return ephem.Coord{}, false
	}
	return c, true
}

// EclipticLongitude returns the apparent geocentric tropical ecliptic
// longitude of b (degrees within [0, 360), true ecliptic and equinox of date)
// at instant t. It returns NaN for points other than the ten planets
// (including Ascendant and Midheaven, which also need a location) and for
// instants outside the ephemeris range.
func EclipticLongitude(b Body, t time.Time) float64 {
	p, ok := planetPosition(b, t)
	if !ok {
		return math.NaN()
	}
	return p.Lon
}

// IsRetrograde reports whether b's apparent motion in longitude is
// backwards at t. The Sun, the Moon and chart points are never retrograde.
func IsRetrograde(b Body, t time.Time) bool {
	if !canRetrograde(b) {
		return false
	}
	p, ok := planetPosition(b, t)
	return ok && p.Speed < 0
}

func canRetrograde(b Body) bool {
	switch b {
	case Mercury, Venus, Mars, Jupiter, Saturn, Uranus, Neptune, Pluto:
		return true
	}
	return false
}

// fastLongitude is the low-precision model used for coarse scans.
func fastLongitude(b Body, t time.Time) float64 {
	eb, ok := ephemBodies[b]
	if !ok {
		return math.NaN()
	}
	return ephem.FastLongitude(eb, ephem.JulianDay(t))
}

// fastMotion returns the signed longitude change of b across six hours
// centred on t with the fast model (degrees).
func fastMotion(b Body, t time.Time) float64 {
	return wrap180(fastLongitude(b, t.Add(3*time.Hour)) - fastLongitude(b, t.Add(-3*time.Hour)))
}
