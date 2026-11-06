package astro

import (
	"math"
	"strings"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/astro/ephem"
)

// HouseSystem names a division of the chart into twelve houses.
type HouseSystem string

// Supported house systems. Porphyry is only used as the fallback when
// Placidus is undefined (inside the polar circles).
const (
	Placidus  HouseSystem = "placidus"
	WholeSign HouseSystem = "whole_sign"
	Equal     HouseSystem = "equal"
	Porphyry  HouseSystem = "porphyry"
)

// DefaultHouseSystem is used when no system is requested.
const DefaultHouseSystem = Placidus

// FallbackPolarLatitude is reported when Placidus cannot be computed because
// part of the ecliptic never rises or sets at the birthplace.
const FallbackPolarLatitude = "polar_latitude"

// ParseHouseSystem accepts "placidus", "whole_sign" (or "whole-sign",
// "wholesign") and "equal", case-insensitively; an empty string selects the
// default. Porphyry cannot be requested.
func ParseHouseSystem(s string) (HouseSystem, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "placidus":
		return Placidus, true
	case "whole_sign", "whole-sign", "wholesign":
		return WholeSign, true
	case "equal":
		return Equal, true
	}
	return "", false
}

// maxAngleLatitude keeps the Ascendant formula away from the poles, where
// tan(latitude) diverges.
const maxAngleLatitude = 89.9

// anglesAt returns the Ascendant, Midheaven, the right ascension of the
// meridian (RAMC) and the true obliquity of the ecliptic, all in degrees,
// for instant t at loc. They use the apparent sidereal time (mean sidereal
// time plus the equation of the equinoxes) and the true obliquity.
func anglesAt(t time.Time, loc GeoLocation) (asc, mc, ramc, eps float64) {
	jd := ephem.JulianDay(t)
	ramc = norm360(ephem.ApparentSiderealTime(jd) + loc.Longitude)
	eps = ephem.TrueObliquity(ephem.JDE(jd))
	asc, mc = anglesFromRAMC(ramc, eps, loc.Latitude)
	return asc, mc, ramc, eps
}

// Angles returns the Ascendant and Midheaven (ecliptic longitudes in
// degrees) for instant t at loc.
func Angles(t time.Time, loc GeoLocation) (asc, mc float64) {
	asc, mc, _, _ = anglesAt(t, loc)
	return asc, mc
}

// anglesFromRAMC computes the Ascendant and Midheaven from the right
// ascension of the meridian, the obliquity of the ecliptic and the latitude
// (all degrees).
func anglesFromRAMC(ramc, eps, lat float64) (asc, mc float64) {
	lat = math.Max(-maxAngleLatitude, math.Min(maxAngleLatitude, lat))
	mc = norm360(atan2d(sind(ramc), cosd(ramc)*cosd(eps)))
	// The two-argument arctangent resolves the quadrant: the Ascendant is
	// the ecliptic point rising on the eastern horizon.
	asc = norm360(atan2d(cosd(ramc), -(sind(ramc)*cosd(eps) + tand(lat)*sind(eps))))
	// Inside the polar circles the formula can return the Descendant; like
	// the Swiss Ephemeris, keep the Ascendant within 180° ahead of the
	// Midheaven.
	if math.Abs(lat) >= 90-eps && norm360(asc-mc) >= 180 {
		asc = norm360(asc + 180)
	}
	return asc, mc
}

// Houses returns the twelve house cusps (ecliptic longitudes in degrees,
// cusps[0] is the first house) for the requested system, given the RAMC,
// the obliquity and the geographic latitude. Placidus falls back to
// Porphyry inside the polar circles, where it is undefined; the system
// actually used and the reason for any fallback are returned.
func Houses(sys HouseSystem, ramc, eps, lat float64) (cusps [12]float64, used HouseSystem, fallback string) {
	asc, mc := anglesFromRAMC(ramc, eps, lat)
	switch sys {
	case WholeSign:
		first := math.Floor(asc/30) * 30
		for i := range cusps {
			cusps[i] = norm360(first + 30*float64(i))
		}
		return cusps, WholeSign, ""
	case Equal:
		for i := range cusps {
			cusps[i] = norm360(asc + 30*float64(i))
		}
		return cusps, Equal, ""
	case Porphyry:
		return porphyry(asc, mc), Porphyry, ""
	default:
		if c, ok := placidus(ramc, eps, lat, asc, mc); ok {
			return c, Placidus, ""
		}
		return porphyry(asc, mc), Porphyry, FallbackPolarLatitude
	}
}

// porphyry trisects each quadrant between the angles along the ecliptic.
func porphyry(asc, mc float64) (c [12]float64) {
	upper := norm360(asc - mc)       // arc from the Midheaven to the Ascendant
	lower := norm360(mc + 180 - asc) // arc from the Ascendant to the IC
	c[0], c[9] = asc, mc
	c[10] = norm360(mc + upper/3)
	c[11] = norm360(mc + 2*upper/3)
	c[1] = norm360(asc + lower/3)
	c[2] = norm360(asc + 2*lower/3)
	for i := 3; i < 9; i++ {
		c[i] = norm360(c[(i+6)%12] + 180)
	}
	return c
}

// placidusTolerance and placidusIterations bound the semi-arc iteration.
const (
	placidusTolerance  = 1e-9 // degrees
	placidusIterations = 50
)

// placidus computes Placidus cusps by trisecting each point's diurnal or
// nocturnal semi-arc: the 11th and 12th cusps are the ecliptic points whose
// hour angle is one and two thirds of their own diurnal semi-arc east of the
// meridian, the 3rd and 2nd one and two thirds of their nocturnal semi-arc
// from the lower meridian. Each cusp is found by fixed-point iteration on its
// declination. It reports false when a cusp's point is circumpolar.
func placidus(ramc, eps, lat, asc, mc float64) (c [12]float64, ok bool) {
	if math.Abs(lat) >= 90-eps {
		return c, false
	}
	cusp := func(fraction float64, below bool) (float64, bool) {
		ra := ramc + fraction*90
		if below {
			ra = ramc + 180 - fraction*90
		}
		lon := norm360(atan2d(sind(ra), cosd(ra)*cosd(eps)))
		for range placidusIterations {
			decl := math.Asin(sind(eps)*sind(lon)) * rad2deg
			x := tand(lat) * tand(decl)
			if math.Abs(x) >= 1 {
				return 0, false
			}
			ad := math.Asin(x) * rad2deg // ascensional difference
			if below {
				ra = ramc + 180 - fraction*(90-ad)
			} else {
				ra = ramc + fraction*(90+ad)
			}
			next := norm360(atan2d(sind(ra), cosd(ra)*cosd(eps)))
			if math.Abs(wrap180(next-lon)) < placidusTolerance {
				return next, true
			}
			lon = next
		}
		return 0, false
	}
	c11, ok11 := cusp(1.0/3, false)
	c12, ok12 := cusp(2.0/3, false)
	c2, ok2 := cusp(2.0/3, true)
	c3, ok3 := cusp(1.0/3, true)
	if !ok11 || !ok12 || !ok2 || !ok3 {
		return c, false
	}
	c[0], c[1], c[2], c[9], c[10], c[11] = asc, c2, c3, mc, c11, c12
	for i := 3; i < 9; i++ {
		c[i] = norm360(c[(i+6)%12] + 180)
	}
	return c, true
}

// HouseOf returns the house (1..12) containing ecliptic longitude lon for
// the given cusps; a point exactly on a cusp belongs to the house that cusp
// begins.
func HouseOf(lon float64, cusps [12]float64) int {
	lon = norm360(lon)
	for i := range cusps {
		width := norm360(cusps[(i+1)%12] - cusps[i])
		if norm360(lon-cusps[i]) < width {
			return i + 1
		}
	}
	return 12
}
