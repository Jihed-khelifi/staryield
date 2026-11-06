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

// Modalities (qualities) in the Aries-to-Pisces cycle.
const (
	Cardinal = "Cardinal"
	Fixed    = "Fixed"
	Mutable  = "Mutable"
)

// Modalities lists the three modalities in zodiac order.
var Modalities = [3]string{Cardinal, Fixed, Mutable}

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

// SignIndex returns the zodiac index (0 = Aries … 11 = Pisces) of a sign
// name, matched case-insensitively, or -1.
func SignIndex(sign string) int { return signIndex(sign) }

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

// ModalityOf returns the modality ("Cardinal", "Fixed" or "Mutable") of a
// sign (case-insensitive), or "" if the sign is unknown.
func ModalityOf(sign string) string {
	i := signIndex(sign)
	if i < 0 {
		return ""
	}
	return Modalities[i%3]
}
