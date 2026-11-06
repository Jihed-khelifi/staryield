package astro

import "math"

const (
	deg2rad = math.Pi / 180
	rad2deg = 180 / math.Pi
)

func sind(x float64) float64 { return math.Sin(x * deg2rad) }
func cosd(x float64) float64 { return math.Cos(x * deg2rad) }
func tand(x float64) float64 { return math.Tan(x * deg2rad) }

func atan2d(y, x float64) float64 { return math.Atan2(y, x) * rad2deg }

// norm360 maps an angle in degrees to the half-open range [0, 360).
func norm360(x float64) float64 {
	x = math.Mod(x, 360)
	if x < 0 {
		x += 360
	}
	if x >= 360 {
		x = 0
	}
	return x
}

// wrap180 maps an angle in degrees to the range (-180, 180].
func wrap180(x float64) float64 {
	x = norm360(x)
	if x > 180 {
		x -= 360
	}
	return x
}

// roundTo rounds x to the given number of decimal places.
func roundTo(x float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(x*p) / p
}
