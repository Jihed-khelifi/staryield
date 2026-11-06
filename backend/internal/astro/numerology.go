package astro

import (
	"strconv"
	"strings"
	"time"
)

// isMasterNumber reports whether n is one of the numerology master numbers.
func isMasterNumber(n int) bool { return n == 11 || n == 22 || n == 33 }

// IsMasterNumber reports whether n is 11, 22 or 33.
func IsMasterNumber(n int) bool { return isMasterNumber(n) }

// ReduceNumber repeatedly sums the decimal digits of n until a single digit
// remains. When keepMasters is true the master numbers 11, 22 and 33 are
// returned as-is instead of being reduced further. Negative inputs are
// treated by absolute value.
func ReduceNumber(n int, keepMasters bool) int {
	n, _ = reduceSteps(n, keepMasters)
	return n
}

// reduceSteps reduces n like ReduceNumber and records the intermediate
// digit sums, e.g. 1990 → ["1+9+9+0=19", "1+9=10", "1+0=1"].
func reduceSteps(n int, keepMasters bool) (int, []string) {
	if n < 0 {
		n = -n
	}
	var steps []string
	for n > 9 && !(keepMasters && isMasterNumber(n)) {
		digits := strconv.Itoa(n)
		sum := 0
		parts := make([]string, len(digits))
		for i, d := range digits {
			parts[i] = string(d)
			sum += int(d - '0')
		}
		steps = append(steps, strings.Join(parts, "+")+"="+strconv.Itoa(sum))
		n = sum
	}
	return n, steps
}

// LifePath returns the numerology life path number for a birth date with the
// component (period-cycle) reduction method: the month, the day and the
// year are each reduced separately keeping master numbers, the three results
// are summed, and the sum is reduced again keeping master numbers 11, 22
// and 33. For 29 November 1990: 11 + 11 + 1 = 23 → 5.
func LifePath(year, month, day int) int {
	return LifePathReduction(year, month, day).Number
}

// Reduction documents each step of a numerology calculation.
type Reduction struct {
	Number int  `json:"number"`
	Master bool `json:"master"`
	// Components lists the reduced month, day and year (life path) or the
	// inputs that were added (personal cycles).
	Components []ReductionStep `json:"components"`
	// Final lists the digit sums applied to the total.
	Final []string `json:"final,omitempty"`
	Total int      `json:"total"`
}

// ReductionStep is one reduced component.
type ReductionStep struct {
	Label   string   `json:"label"` // "month", "day", "year", "personal_year", "calendar_month" …
	Value   int      `json:"value"` // the raw input
	Reduced int      `json:"reduced"`
	Steps   []string `json:"steps,omitempty"`
}

// LifePathReduction computes the life path and records every step.
func LifePathReduction(year, month, day int) Reduction {
	comp := func(label string, v int) ReductionStep {
		r, steps := reduceSteps(v, true)
		return ReductionStep{Label: label, Value: v, Reduced: r, Steps: steps}
	}
	m, d, y := comp("month", month), comp("day", day), comp("year", year)
	total := m.Reduced + d.Reduced + y.Reduced
	n, final := reduceSteps(total, true)
	return Reduction{Number: n, Master: isMasterNumber(n), Components: []ReductionStep{m, d, y}, Total: total, Final: final}
}

// PersonalYear returns the personal year number for a birth month and day
// in the calendar year of asOf: R(R(month) + R(day) + R(year)), reduced to a
// single digit 1–9 (personal cycles do not keep master numbers).
func PersonalYear(birthMonth, birthDay int, asOf time.Time) int {
	return PersonalYearReduction(birthMonth, birthDay, asOf).Number
}

// PersonalYearReduction computes the personal year and records the steps.
func PersonalYearReduction(birthMonth, birthDay int, asOf time.Time) Reduction {
	comp := func(label string, v int) ReductionStep {
		r, steps := reduceSteps(v, false)
		return ReductionStep{Label: label, Value: v, Reduced: r, Steps: steps}
	}
	m, d, y := comp("birth_month", birthMonth), comp("birth_day", birthDay), comp("year", asOf.Year())
	total := m.Reduced + d.Reduced + y.Reduced
	n, final := reduceSteps(total, false)
	return Reduction{Number: n, Components: []ReductionStep{m, d, y}, Total: total, Final: final}
}

// PersonalMonth returns R(personal year + calendar month of asOf).
func PersonalMonth(birthMonth, birthDay int, asOf time.Time) int {
	return PersonalMonthReduction(birthMonth, birthDay, asOf).Number
}

// PersonalMonthReduction computes the personal month and records the steps.
func PersonalMonthReduction(birthMonth, birthDay int, asOf time.Time) Reduction {
	py := PersonalYear(birthMonth, birthDay, asOf)
	month := int(asOf.Month())
	total := py + month
	n, final := reduceSteps(total, false)
	return Reduction{Number: n, Total: total, Final: final, Components: []ReductionStep{
		{Label: "personal_year", Value: py, Reduced: py},
		{Label: "calendar_month", Value: month, Reduced: month},
	}}
}

// PersonalDay returns R(personal month + calendar day of asOf).
func PersonalDay(birthMonth, birthDay int, asOf time.Time) int {
	pm := PersonalMonth(birthMonth, birthDay, asOf)
	return ReduceNumber(pm+asOf.Day(), false)
}
