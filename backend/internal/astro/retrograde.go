package astro

import "time"

// Period is a time interval during which a body is retrograde.
type Period struct {
	Body  string    `json:"body"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// retrogradeScanMargin extends the scan on both sides of the requested
// window so that periods overlapping its edges are found in full. The longest
// retrograde loops (Neptune, Pluto) last a little over five months.
const retrogradeScanMargin = 200 * 24 * time.Hour

// RetrogradePeriods returns the retrograde periods of b that overlap the
// interval [from, to], in chronological order. Stations are located by
// scanning the apparent motion once a day and refining each change of
// direction by bisection to under a minute. A period already in progress at
// from (or still in progress at to) is returned with its true start (or end).
// The Sun, Moon and chart points have no retrograde periods.
func RetrogradePeriods(b Body, from, to time.Time) []Period {
	if !canRetrograde(b) || to.Before(from) {
		return nil
	}
	start := from.Add(-retrogradeScanMargin)
	end := to.Add(retrogradeScanMargin)
	const step = 24 * time.Hour

	var periods []Period
	var periodStart time.Time
	open := false

	prev := start
	prevRetro := IsRetrograde(b, prev)
	for t := start.Add(step); !t.After(end); t = t.Add(step) {
		retro := IsRetrograde(b, t)
		if retro != prevRetro {
			station := refineStation(b, prev, t, prevRetro)
			if retro {
				periodStart, open = station, true
			} else if open {
				periods = append(periods, Period{Body: string(b), Start: periodStart, End: station})
				open = false
			}
		}
		prev, prevRetro = t, retro
	}

	overlapping := periods[:0]
	for _, p := range periods {
		if p.Start.Before(to) && p.End.After(from) {
			overlapping = append(overlapping, p)
		}
	}
	if len(overlapping) == 0 {
		return nil
	}
	return overlapping
}

// refineStation bisects [lo, hi] — where b's retrograde state changes from
// loRetro — down to under a minute and returns the station time, rounded to
// the second.
func refineStation(b Body, lo, hi time.Time, loRetro bool) time.Time {
	for hi.Sub(lo) > 30*time.Second {
		mid := lo.Add(hi.Sub(lo) / 2)
		if IsRetrograde(b, mid) == loRetro {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo.Add(hi.Sub(lo) / 2).UTC().Round(time.Second)
}
