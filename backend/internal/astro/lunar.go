package astro

import (
	"math"
	"time"
)

// Kinds of principal lunar phase reported by NextLunarEvents.
const (
	NewMoonEvent      = "new_moon"
	FirstQuarterEvent = "first_quarter"
	FullMoonEvent     = "full_moon"
	LastQuarterEvent  = "last_quarter"
)

// Lunar phase names returned by PhaseName.
const (
	PhaseNewMoon        = "New Moon"
	PhaseWaxingCrescent = "Waxing Crescent"
	PhaseFirstQuarter   = "First Quarter"
	PhaseWaxingGibbous  = "Waxing Gibbous"
	PhaseFullMoon       = "Full Moon"
	PhaseWaningGibbous  = "Waning Gibbous"
	PhaseLastQuarter    = "Last Quarter"
	PhaseWaningCrescent = "Waning Crescent"
)

// PhaseNames lists the eight phase names in cycle order from New Moon.
var PhaseNames = [8]string{
	PhaseNewMoon, PhaseWaxingCrescent, PhaseFirstQuarter, PhaseWaxingGibbous,
	PhaseFullMoon, PhaseWaningGibbous, PhaseLastQuarter, PhaseWaningCrescent,
}

// LunarEvent is the exact moment of a principal lunar phase.
type LunarEvent struct {
	Kind string    `json:"kind"`
	At   time.Time `json:"at"`
	Sign string    `json:"sign"`
}

// MoonPhaseAngle returns the Moon's elongation from the Sun in ecliptic
// longitude, in degrees within [0, 360): 0 at New Moon, 90 at First Quarter,
// 180 at Full Moon and 270 at Last Quarter.
func MoonPhaseAngle(t time.Time) float64 {
	d := dayNumber(t)
	sun, _ := sunPosition(d)
	return norm360(moonLongitude(d) - sun)
}

// Illumination returns the approximate illuminated fraction of the Moon's
// disc (0 at New Moon, 1 at Full Moon).
func Illumination(t time.Time) float64 {
	return (1 - cosd(MoonPhaseAngle(t))) / 2
}

// principalPhaseWindow is the half-width, in degrees of elongation, within
// which a principal phase name (New Moon, First Quarter, Full Moon, Last
// Quarter) is used. The elongation advances about 12.2° a day, so 7.5° is
// roughly 15 hours either side of the exact moment, which guarantees that at
// least one daily reading carries each principal phase name.
const principalPhaseWindow = 7.5

// PhaseName returns the conventional name of the lunar phase at t. Principal
// phases are reported within about 15 hours of their exact moment; the
// intermediate names cover the time in between.
func PhaseName(t time.Time) string {
	e := MoonPhaseAngle(t)
	nearest := int(math.Round(e/90)) % 4
	if math.Abs(wrap180(e-float64(nearest)*90)) <= principalPhaseWindow {
		return PhaseNames[nearest*2]
	}
	return PhaseNames[int(e/90)*2+1]
}

var lunarEventKinds = [4]string{NewMoonEvent, FirstQuarterEvent, FullMoonEvent, LastQuarterEvent}

// lunarSearchStep is the coarse step used to bracket a phase before
// bisection. The elongation advances at most about 15° a day, so a 6-hour
// step never jumps across more than one target.
const lunarSearchStep = 6 * time.Hour

// lunarEventResolution is the precision of NextLunarEvents. A phase that
// falls within this long after from is treated as coinciding with from and
// skipped, so passing a previously returned event's At yields the next one.
const lunarEventResolution = time.Minute

// NextLunarEvents returns the next n principal lunar phases after from, in
// chronological order. Each moment is found by bracketing the target
// elongation and refining it by bisection to better than a minute; the
// result is rounded to the nearest second. Each event reports the Moon's sign
// at that moment.
func NextLunarEvents(from time.Time, n int) []LunarEvent {
	if n <= 0 {
		return nil
	}
	events := make([]LunarEvent, 0, n)
	lo := from.Add(lunarEventResolution)
	k := int(math.Floor(MoonPhaseAngle(lo)/90)) + 1
	for len(events) < n {
		q := k % 4
		target := float64(q) * 90
		offset := func(t time.Time) float64 { return wrap180(MoonPhaseAngle(t) - target) }

		hi := lo.Add(lunarSearchStep)
		for offset(hi) < 0 {
			lo, hi = hi, hi.Add(lunarSearchStep)
		}
		for hi.Sub(lo) > 30*time.Second {
			mid := lo.Add(hi.Sub(lo) / 2)
			if offset(mid) < 0 {
				lo = mid
			} else {
				hi = mid
			}
		}
		at := lo.Add(hi.Sub(lo) / 2).UTC().Round(time.Second)
		events = append(events, LunarEvent{
			Kind: lunarEventKinds[q],
			At:   at,
			Sign: SignForLongitude(EclipticLongitude(Moon, at)),
		})
		lo = hi
		k++
	}
	return events
}
