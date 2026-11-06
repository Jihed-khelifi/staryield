package astro

import (
	"time"
)

// SunSignAt returns the tropical sign occupied by the Sun at instant t,
// derived from its apparent ecliptic longitude. It is the one canonical
// sun-sign function: horoscopes, the advisor's client card, the calculators
// and the natal chart all use it, so a cusp birthday gets the same sign on
// every screen. It returns "" outside the ephemeris range.
func SunSignAt(t time.Time) string {
	return SignForLongitude(EclipticLongitude(Sun, t))
}

// SunSign is the Sun's sign for a birth, with uncertainty when the birth
// time or place is unknown and the Sun changed sign during the possible
// window.
type SunSign struct {
	Sign         string   `json:"sun_sign"`
	Uncertain    bool     `json:"uncertain"`
	Alternatives []string `json:"alternatives,omitempty"`
}

// Birth windows used when the time or the place is unknown. Civil time zones
// span UTC−12 to UTC+14, so a calendar day D begins somewhere between
// D−1 10:00 UTC and D 12:00 UTC and ends between D 10:00 and D+1 12:00 UTC.
const (
	earliestZoneOffset = 14 * time.Hour
	latestZoneOffset   = 12 * time.Hour
)

// BirthWindow returns the instant used to represent a birth on the calendar
// day of date and the span of instants the birth could have taken place in:
//   - with a clock and a location: the exact instant (start = end);
//   - with a location only: local noon, within the local day;
//   - otherwise: 12:00 UTC, within every time zone's version of that day.
//
// The clock must be a valid "HH:MM" or empty; invalid clocks are treated as
// unknown.
func BirthWindow(date time.Time, hour, minute int, clockKnown bool, loc *time.Location) (at, start, end time.Time) {
	y, m, d := date.Date()
	switch {
	case clockKnown && loc != nil:
		at = time.Date(y, m, d, hour, minute, 0, 0, loc).UTC()
		return at, at, at
	case loc != nil:
		at = time.Date(y, m, d, 12, 0, 0, 0, loc).UTC()
		start = time.Date(y, m, d, 0, 0, 0, 0, loc).UTC()
		end = time.Date(y, m, d+1, 0, 0, 0, 0, loc).UTC().Add(-time.Minute)
		return at, start, end
	default:
		midnight := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
		at = midnight.Add(12 * time.Hour)
		start = midnight.Add(-earliestZoneOffset)
		end = midnight.Add(24*time.Hour + latestZoneOffset - time.Minute)
		return at, start, end
	}
}

// SunSignForBirth returns the Sun's sign for a birth at `at` that could have
// happened anywhere in [start, end] (see BirthWindow). The Sun moves about a
// degree a day, so the window spans at most two signs; when it does, the
// result is uncertain and lists both, in zodiac order of the transit.
func SunSignForBirth(at, start, end time.Time) SunSign {
	out := SunSign{Sign: SunSignAt(at)}
	first, last := SunSignAt(start), SunSignAt(end)
	if first != last && first != "" && last != "" {
		out.Uncertain = true
		out.Alternatives = []string{first, last}
	}
	return out
}

// TypicalSunSign returns the sign the Sun occupies at 12:00 UTC on a
// month/day, computed for the reference year 2001 (2000 for 29 February).
// It is meant for aggregate statistics where the birth year is unknown; use
// SunSignForBirth for a person.
func TypicalSunSign(month, day int) string {
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return ""
	}
	year := 2001
	if month == 2 && day == 29 {
		year = 2000
	}
	t := time.Date(year, time.Month(month), day, 12, 0, 0, 0, time.UTC)
	if t.Month() != time.Month(month) {
		return ""
	}
	return SunSignAt(t)
}

// SunIngress returns the instant (UTC, to the second) at which the Sun
// enters the sign with index signIdx (0 = Aries) during the given year.
func SunIngress(signIdx, year int) time.Time {
	target := float64(signIdx%12) * 30
	// The Sun enters Aries around 20 March and advances ~30.4 days per sign.
	approx := time.Date(year, 3, 20, 12, 0, 0, 0, time.UTC).Add(time.Duration(float64(signIdx%12)*30.44*24) * time.Hour)
	if approx.Year() > year { // Capricorn/Aquarius/Pisces fall early in the year
		approx = approx.AddDate(-1, 0, 0)
	}
	offset := func(t time.Time) float64 { return wrap180(EclipticLongitude(Sun, t) - target) }
	lo, hi := approx.Add(-6*24*time.Hour), approx.Add(6*24*time.Hour)
	for offset(lo) > 0 {
		lo = lo.Add(-3 * 24 * time.Hour)
	}
	for offset(hi) < 0 {
		hi = hi.Add(3 * 24 * time.Hour)
	}
	for hi.Sub(lo) > time.Second {
		mid := lo.Add(hi.Sub(lo) / 2)
		if offset(mid) < 0 {
			lo = mid
		} else {
			hi = mid
		}
	}
	return hi.Truncate(time.Second).UTC()
}

// SunSignDates returns the first and last calendar days, in loc, of the
// sign's period in the year it begins. A day belongs to the sign the Sun
// occupies at local noon, so consecutive signs never share a day.
func SunSignDates(sign string, year int, loc *time.Location) (first, last time.Time, ok bool) {
	i := signIndex(sign)
	if i < 0 {
		return time.Time{}, time.Time{}, false
	}
	start := SunIngress(i, year)
	end := SunIngress((i+1)%12, year)
	if !end.After(start) { // Capricorn ends in January of the next year
		end = SunIngress((i+1)%12, year+1)
	}
	firstDay := func(ingress time.Time) time.Time {
		l := ingress.In(loc)
		day := time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, loc)
		if !l.Before(time.Date(l.Year(), l.Month(), l.Day(), 12, 0, 0, 0, loc)) {
			day = day.AddDate(0, 0, 1)
		}
		return day
	}
	return firstDay(start), firstDay(end).AddDate(0, 0, -1), true
}
