package astro

// isMasterNumber reports whether n is one of the numerology master numbers.
func isMasterNumber(n int) bool { return n == 11 || n == 22 || n == 33 }

// ReduceNumber repeatedly sums the decimal digits of n until a single digit
// remains. When keepMasters is true the master numbers 11, 22 and 33 are
// returned as-is instead of being reduced further. Negative inputs are
// treated by absolute value.
func ReduceNumber(n int, keepMasters bool) int {
	if n < 0 {
		n = -n
	}
	for n > 9 && !(keepMasters && isMasterNumber(n)) {
		sum := 0
		for n > 0 {
			sum += n % 10
			n /= 10
		}
		n = sum
	}
	return n
}

// LifePath returns the numerology life path number for a birth date. Each of
// the year, month and day is reduced separately (keeping master numbers), the
// three results are summed, and the sum is reduced again keeping master
// numbers. This mirrors the frontend calculator.
func LifePath(year, month, day int) int {
	return ReduceNumber(
		ReduceNumber(year, true)+ReduceNumber(month, true)+ReduceNumber(day, true),
		true,
	)
}
