package astro

import "math"

// Aspect types.
const (
	Conjunction = "conjunction"
	Sextile     = "sextile"
	Square      = "square"
	Trine       = "trine"
	Opposition  = "opposition"
)

// aspectAngles lists the major aspects and their base orbs (degrees).
var aspectAngles = []struct {
	name  string
	angle float64
	orb   float64
}{
	{Conjunction, 0, 8},
	{Sextile, 60, 5},
	{Square, 90, 7},
	{Trine, 120, 7},
	{Opposition, 180, 8},
}

// Aspect is a major angular relationship between two chart points.
type Aspect struct {
	A          string  `json:"a"`
	B          string  `json:"b"`
	Type       string  `json:"type"`
	ExactAngle float64 `json:"exact_angle"` // the aspect's nominal angle
	Angle      float64 `json:"angle"`       // the actual separation, 0–180
	Orb        float64 `json:"orb"`         // |angle − exact_angle|
	// Applying reports that the aspect is tightening (computed from the two
	// planets' daily motions; always false when an angle is involved).
	Applying bool `json:"applying"`
}

// aspectPoint is a point taking part in aspect detection.
type aspectPoint struct {
	body  Body
	lon   float64
	speed float64
	angle bool // Ascendant or Midheaven
}

// orbFor returns the allowed orb of an aspect between two points: the base
// orb, two degrees wider when the Sun or the Moon is involved and two
// degrees narrower when an angle is involved.
func orbFor(base float64, a, b aspectPoint) float64 {
	orb := base
	if a.body == Sun || a.body == Moon || b.body == Sun || b.body == Moon {
		orb += 2
	}
	if a.angle || b.angle {
		orb -= 2
	}
	return orb
}

// findAspects returns at most one aspect (the tightest) per pair of points,
// in the order the points are given.
func findAspects(points []aspectPoint) []Aspect {
	out := []Aspect{}
	for i := 0; i < len(points); i++ {
		for j := i + 1; j < len(points); j++ {
			a, b := points[i], points[j]
			if a.angle && b.angle {
				continue // the Ascendant–Midheaven relation is not an aspect
			}
			sep := math.Abs(wrap180(a.lon - b.lon))
			best := -1
			bestOrb := 0.0
			for k, asp := range aspectAngles {
				orb := math.Abs(sep - asp.angle)
				if orb <= orbFor(asp.orb, a, b) && (best < 0 || orb < bestOrb) {
					best, bestOrb = k, orb
				}
			}
			if best < 0 {
				continue
			}
			asp := aspectAngles[best]
			applying := false
			if !a.angle && !b.angle {
				const dt = 0.01 // days
				next := math.Abs(wrap180((a.lon + a.speed*dt) - (b.lon + b.speed*dt)))
				applying = math.Abs(next-asp.angle) < bestOrb
			}
			out = append(out, Aspect{
				A: string(a.body), B: string(b.body), Type: asp.name,
				ExactAngle: asp.angle, Angle: roundTo(sep, 4), Orb: roundTo(bestOrb, 4), Applying: applying,
			})
		}
	}
	return out
}
