package astro

import (
	"fmt"
	"math"
	"time"
)

// GeoLocation is a point on the Earth in degrees, with latitude positive
// north and longitude positive east.
type GeoLocation struct {
	Latitude  float64
	Longitude float64
}

// Position is a body or chart point placed in the zodiac.
type Position struct {
	Body       string  `json:"body"`
	Longitude  float64 `json:"longitude"`
	Sign       string  `json:"sign"`
	Degree     int     `json:"degree"`
	Minute     int     `json:"minute"`
	Second     int     `json:"second"`
	Formatted  string  `json:"formatted"`
	House      int     `json:"house,omitempty"`
	HouseLabel string  `json:"house_label,omitempty"`
	Retrograde bool    `json:"retrograde"`
}

// Chart is a natal (or any moment's) chart.
type Chart struct {
	Instant     time.Time      `json:"instant"`
	HasLocation bool           `json:"has_location"`
	HouseSystem string         `json:"house_system,omitempty"`
	SunSign     string         `json:"sun_sign"`
	MoonSign    string         `json:"moon_sign"`
	RisingSign  string         `json:"rising_sign,omitempty"`
	Positions   []Position     `json:"positions"`
	HouseCusps  []float64      `json:"house_cusps,omitempty"`
	Element     map[string]int `json:"elements"`
}

// HouseSystemEqual is the identifier of the equal house system, in which
// every house spans 30° starting from the Ascendant.
const HouseSystemEqual = "equal"

// houseLabels names houses 1..12 (index 0 is the first house).
var houseLabels = [12]string{
	"First house", "Second house", "Third house", "Fourth house",
	"Fifth house", "Sixth house", "Seventh house", "Eighth house",
	"Ninth house", "Tenth house", "Eleventh house", "Twelfth house",
}

// HouseLabel returns the English label for house n (1..12), such as
// "First house", or "" when n is out of range.
func HouseLabel(n int) string {
	if n < 1 || n > 12 {
		return ""
	}
	return houseLabels[n-1]
}

// maxAngleLatitude keeps the Ascendant formula away from the poles, where
// tan(latitude) diverges.
const maxAngleLatitude = 89.9

// Angles returns the Ascendant and Midheaven (ecliptic longitudes in degrees)
// for instant t at loc. They are derived from the local apparent sidereal
// time (approximated by the local mean sidereal time), the obliquity of the
// ecliptic and the geographic latitude. Near and beyond the polar circles the
// Ascendant is mathematically defined but astrologically unstable.
func Angles(t time.Time, loc GeoLocation) (asc, mc float64) {
	return anglesFromRAMC(localSiderealDegrees(t, loc.Longitude), obliquity(dayNumber(t)), loc.Latitude)
}

// anglesFromRAMC computes the Ascendant and Midheaven from the right
// ascension of the meridian, the obliquity of the ecliptic and the latitude
// (all degrees).
func anglesFromRAMC(ramc, eps, lat float64) (asc, mc float64) {
	lat = math.Max(-maxAngleLatitude, math.Min(maxAngleLatitude, lat))
	mc = norm360(atan2d(sind(ramc), cosd(ramc)*cosd(eps)))
	// The two-argument arctangent resolves the quadrant: the Ascendant is
	// the ecliptic point rising on the eastern horizon, which always lies
	// less than 180° ahead of the Midheaven.
	asc = norm360(atan2d(cosd(ramc), -(sind(ramc)*cosd(eps) + tand(lat)*sind(eps))))
	return asc, mc
}

// greenwichSiderealDegrees returns the Greenwich mean sidereal time at t as
// an angle in degrees (IAU 1982 expression, UT based).
func greenwichSiderealDegrees(t time.Time) float64 {
	jd := julianDayUT(t)
	T := (jd - jdJ2000) / 36525
	return norm360(280.46061837 + 360.98564736629*(jd-jdJ2000) + 0.000387933*T*T - T*T*T/38710000)
}

// localSiderealDegrees returns the local mean sidereal time (the right
// ascension of the meridian, RAMC) in degrees for an east-positive longitude.
func localSiderealDegrees(t time.Time, longitude float64) float64 {
	return norm360(greenwichSiderealDegrees(t) + longitude)
}

// ComputeChart computes the positions of the ten planets at instant. When loc
// is non-nil it also computes the Ascendant, Midheaven and equal houses;
// otherwise house fields are left empty and HasLocation is false.
func ComputeChart(instant time.Time, loc *GeoLocation) Chart {
	instant = instant.UTC()
	c := Chart{
		Instant:     instant,
		HasLocation: loc != nil,
		Element:     map[string]int{Fire: 0, Earth: 0, Air: 0, Water: 0},
		Positions:   make([]Position, 0, len(Planets)+2),
	}

	var asc, mc float64
	if loc != nil {
		asc, mc = Angles(instant, *loc)
		c.HouseSystem = HouseSystemEqual
		c.HouseCusps = make([]float64, 12)
		for i := range c.HouseCusps {
			c.HouseCusps[i] = norm360(roundTo(norm360(asc+30*float64(i)), 6))
		}
		ap := newPosition(Ascendant, asc, false)
		c.RisingSign = ap.Sign
		c.Positions = append(c.Positions, ap)
	}

	for _, b := range Planets {
		p := newPosition(b, EclipticLongitude(b, instant), IsRetrograde(b, instant))
		c.Positions = append(c.Positions, p)
		c.Element[ElementOf(p.Sign)]++
		switch b {
		case Sun:
			c.SunSign = p.Sign
		case Moon:
			c.MoonSign = p.Sign
		}
	}

	if loc != nil {
		c.Positions = append(c.Positions, newPosition(Midheaven, mc, false))
		// Houses are assigned from the rounded longitudes that are reported,
		// so the Ascendant always sits exactly on the first cusp.
		first := c.HouseCusps[0]
		for i := range c.Positions {
			p := &c.Positions[i]
			p.House = equalHouse(p.Longitude, first)
			p.HouseLabel = HouseLabel(p.House)
		}
	}
	return c
}

// equalHouse returns the equal-system house (1..12) containing lon.
func equalHouse(lon, asc float64) int {
	h := int(norm360(lon-asc)/30) + 1
	if h > 12 {
		h = 12
	}
	return h
}

// arcsecondsPerCircle and arcsecondsPerSign express the zodiac in whole
// arcseconds so that the sign, degree, minute and second always agree after
// rounding.
const (
	arcsecondsPerCircle = 360 * 3600
	arcsecondsPerSign   = 30 * 3600
)

// newPosition builds a Position for body at ecliptic longitude lon. The
// longitude is rounded to the nearest arcsecond before being split into
// sign, degree, minute and second.
func newPosition(body Body, lon float64, retro bool) Position {
	lon = norm360(lon)
	total := int(math.Round(lon*3600)) % arcsecondsPerCircle
	within := total % arcsecondsPerSign
	p := Position{
		Body:       string(body),
		Longitude:  norm360(roundTo(lon, 6)),
		Sign:       Signs[total/arcsecondsPerSign],
		Degree:     within / 3600,
		Minute:     within % 3600 / 60,
		Second:     within % 60,
		Retrograde: retro,
	}
	p.Formatted = FormatZodiac(p.Sign, p.Degree, p.Minute, p.Second)
	return p
}

// FormatZodiac renders a zodiac position the way the frontend displays it,
// for example "Cancer, 8°22′47″" (minutes and seconds are not zero-padded).
func FormatZodiac(sign string, degree, minute, second int) string {
	return fmt.Sprintf("%s, %d°%d′%d″", sign, degree, minute, second)
}
