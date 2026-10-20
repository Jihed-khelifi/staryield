package astro

import (
	"math"
	"strings"
)

// Signs lists the twelve tropical zodiac signs in order from 0° Aries.
var Signs = [12]string{
	"Aries", "Taurus", "Gemini", "Cancer", "Leo", "Virgo",
	"Libra", "Scorpio", "Sagittarius", "Capricorn", "Aquarius", "Pisces",
}

// Elements in the traditional Aries-to-Pisces cycle.
const (
	Fire  = "Fire"
	Earth = "Earth"
	Air   = "Air"
	Water = "Water"
)

// Elements lists the four classical elements in zodiac order.
var Elements = [4]string{Fire, Earth, Air, Water}

// SignForLongitude returns the tropical sign containing ecliptic longitude
// lon (degrees; any value is normalised into [0, 360)).
func SignForLongitude(lon float64) string {
	if math.IsNaN(lon) || math.IsInf(lon, 0) {
		return ""
	}
	idx := int(norm360(lon) / 30)
	if idx > 11 { // guards against rounding at the very top of Pisces
		idx = 11
	}
	return Signs[idx]
}

// calendarSigns is indexed so that month m (1-based) maps to calendarSigns[m-1]
// before that month's cusp day and to calendarSigns[m%12] from it onwards.
var calendarSigns = [12]string{
	"Capricorn", "Aquarius", "Pisces", "Aries", "Taurus", "Gemini",
	"Cancer", "Leo", "Virgo", "Libra", "Scorpio", "Sagittarius",
}

// signStartDay holds, for January..December, the first day of the sign that
// begins during that month (the same table the frontend uses).
var signStartDay = [12]int{20, 19, 21, 20, 21, 21, 23, 23, 23, 23, 22, 22}

// SunSignForDate returns the conventional calendar sun sign for a birthday,
// using fixed tropical boundaries (Aquarius from Jan 20, Pisces Feb 19, Aries
// Mar 21, Taurus Apr 20, Gemini May 21, Cancer Jun 21, Leo Jul 23, Virgo
// Aug 23, Libra Sep 23, Scorpio Oct 23, Sagittarius Nov 22, Capricorn Dec 22).
// It returns "" for an out-of-range month or day.
func SunSignForDate(month, day int) string {
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return ""
	}
	if day < signStartDay[month-1] {
		return calendarSigns[month-1]
	}
	return calendarSigns[month%12]
}

// signIndex returns the zodiac index (0 = Aries) of a sign name, matched
// case-insensitively, or -1 if it is unknown.
func signIndex(sign string) int {
	s := strings.TrimSpace(sign)
	for i, name := range Signs {
		if strings.EqualFold(name, s) {
			return i
		}
	}
	return -1
}

// CanonicalSign returns the properly capitalised sign name for a
// case-insensitive input such as "leo" or " SCORPIO ", and false if the input
// is not a zodiac sign.
func CanonicalSign(sign string) (string, bool) {
	i := signIndex(sign)
	if i < 0 {
		return "", false
	}
	return Signs[i], true
}

// ElementOf returns the element ("Fire", "Earth", "Air" or "Water") of a sign
// (case-insensitive), or "" if the sign is unknown.
func ElementOf(sign string) string {
	i := signIndex(sign)
	if i < 0 {
		return ""
	}
	return Elements[i%4]
}
