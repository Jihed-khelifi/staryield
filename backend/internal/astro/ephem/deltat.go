package ephem

import "time"

// DeltaT returns TT − UT in seconds for the Julian Day jdUT (UT scale).
//
// Between 1900 and 2101 it interpolates the yearly values published by JPL
// Horizons (see deltaTTable), so positions agree with Horizons to the
// precision of the theories. For modern dates Horizons' "UT" is UTC, so the
// table holds TT − UTC (32.184 s + TAI − UTC), which is exactly the
// conversion needed for the UTC instants this package receives; civil birth
// times are never more precise than the < 1 s UTC − UT1 difference. Outside
// that range it falls back to the Espenak & Meeus polynomial expressions.
func DeltaT(jdUT float64) float64 {
	t := TimeFromJD(jdUT)
	y := t.Year()
	if y >= deltaTFirstYear && y < deltaTLastYear {
		start := JulianDay(time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC))
		end := JulianDay(time.Date(y+1, 1, 1, 0, 0, 0, 0, time.UTC))
		f := (jdUT - start) / (end - start)
		i := y - deltaTFirstYear
		return deltaTTable[i] + f*(deltaTTable[i+1]-deltaTTable[i])
	}
	if y == deltaTLastYear {
		return deltaTTable[len(deltaTTable)-1]
	}
	return deltaTPolynomial(2000 + (jdUT-J2000)/365.25)
}

// deltaTPolynomial evaluates the Espenak & Meeus expressions ("Polynomial
// Expressions for Delta T", NASA Five Millennium Canon) for a decimal year.
func deltaTPolynomial(y float64) float64 {
	switch {
	case y < -500:
		u := (y - 1820) / 100
		return -20 + 32*u*u
	case y < 500:
		u := y / 100
		return 10583.6 - 1014.41*u + 33.78311*u*u - 5.952053*u*u*u - 0.1798452*u*u*u*u +
			0.022174192*u*u*u*u*u + 0.0090316521*u*u*u*u*u*u
	case y < 1600:
		u := (y - 1000) / 100
		return 1574.2 - 556.01*u + 71.23472*u*u + 0.319781*u*u*u - 0.8503463*u*u*u*u -
			0.005050998*u*u*u*u*u + 0.0083572073*u*u*u*u*u*u
	case y < 1700:
		t := y - 1600
		return 120 - 0.9808*t - 0.01532*t*t + t*t*t/7129
	case y < 1800:
		t := y - 1700
		return 8.83 + 0.1603*t - 0.0059285*t*t + 0.00013336*t*t*t - t*t*t*t/1174000
	case y < 1860:
		t := y - 1800
		return 13.72 - 0.332447*t + 0.0068612*t*t + 0.0041116*t*t*t - 0.00037436*t*t*t*t +
			0.0000121272*t*t*t*t*t - 0.0000001699*t*t*t*t*t*t + 0.000000000875*t*t*t*t*t*t*t
	case y < 1900:
		t := y - 1860
		return 7.62 + 0.5737*t - 0.251754*t*t + 0.01680668*t*t*t -
			0.0004473624*t*t*t*t + t*t*t*t*t/233174
	case y < 1920:
		t := y - 1900
		return -2.79 + 1.494119*t - 0.0598939*t*t + 0.0061966*t*t*t - 0.000197*t*t*t*t
	case y < 1941:
		t := y - 1920
		return 21.20 + 0.84493*t - 0.076100*t*t + 0.0020936*t*t*t
	case y < 1961:
		t := y - 1950
		return 29.07 + 0.407*t - t*t/233 + t*t*t/2547
	case y < 1986:
		t := y - 1975
		return 45.45 + 1.067*t - t*t/260 - t*t*t/718
	case y < 2005:
		t := y - 2000
		return 63.86 + 0.3345*t - 0.060374*t*t + 0.0017275*t*t*t +
			0.000651814*t*t*t*t + 0.00002373599*t*t*t*t*t
	case y < 2050:
		t := y - 2000
		return 62.92 + 0.32217*t + 0.005589*t*t
	case y < 2150:
		u := (y - 1820) / 100
		return -20 + 32*u*u - 0.5628*(2150-y)
	default:
		u := (y - 1820) / 100
		return -20 + 32*u*u
	}
}

// JDE converts a Julian Day in UT to the Terrestrial Time scale used by the
// planetary theories.
func JDE(jdUT float64) float64 { return jdUT + DeltaT(jdUT)/86400 }
