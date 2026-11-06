package ephem

import (
	"bytes"
	"compress/gzip"
	"embed"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// The VSOP87B series (heliocentric spherical coordinates referred to the
// dynamical ecliptic and equinox of J2000) for Mercury to Neptune, truncated
// and converted by scripts/vsop87. See data/NOTICE for provenance.
//
//go:generate go run ../../../scripts/vsop87 -out data
//go:embed data/vsop87b_*.bin.gz
var vsopFS embed.FS

const vsopMagic = "VSOP87B1"

// vsopTerm is one periodic term A·cos(B + C·τ).
type vsopTerm struct{ A, B, C float64 }

// vsopSeries holds, for one variable (L, B or R), the series multiplied by
// τ⁰, τ¹ … τ⁵.
type vsopSeries [][]vsopTerm

// vsopPlanet holds the L, B and R series of one planet.
type vsopPlanet struct {
	vars [3]vsopSeries
}

var vsopFiles = map[Body]string{
	Mercury: "mer", Venus: "ven", earth: "ear", Mars: "mar",
	Jupiter: "jup", Saturn: "sat", Uranus: "ura", Neptune: "nep",
}

// loadVSOP decodes one embedded planet file.
func loadVSOP(name string) (*vsopPlanet, error) {
	raw, err := vsopFS.ReadFile("data/vsop87b_" + name + ".bin.gz")
	if err != nil {
		return nil, err
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	if len(data) < len(vsopMagic) || string(data[:len(vsopMagic)]) != vsopMagic {
		return nil, errors.New("ephem: bad VSOP87 data header")
	}
	r := bytes.NewReader(data[len(vsopMagic):])
	p := &vsopPlanet{}
	for v := range p.vars {
		n, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if n == 0 || n > 6 {
			return nil, fmt.Errorf("ephem: bad VSOP87 series count %d", n)
		}
		p.vars[v] = make(vsopSeries, n)
		for k := range p.vars[v] {
			var count uint32
			if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
				return nil, err
			}
			if count > 20000 {
				return nil, fmt.Errorf("ephem: bad VSOP87 term count %d", count)
			}
			terms := make([]vsopTerm, count)
			if err := binary.Read(r, binary.LittleEndian, terms); err != nil {
				return nil, err
			}
			p.vars[v][k] = terms
		}
	}
	if r.Len() != 0 {
		return nil, errors.New("ephem: trailing VSOP87 data")
	}
	return p, nil
}

// evalSeries sums Σ_k τ^k Σ_i A cos(B + C τ).
func evalSeries(s vsopSeries, tau float64) float64 {
	sum, pow := 0.0, 1.0
	for _, terms := range s {
		part := 0.0
		// Add small terms first for better rounding.
		for i := len(terms) - 1; i >= 0; i-- {
			t := terms[i]
			part += t.A * math.Cos(t.B+t.C*tau)
		}
		sum += part * pow
		pow *= tau
	}
	return sum
}

// heliocentric returns the planet's heliocentric ecliptic longitude and
// latitude (radians) and radius vector (AU), referred to the dynamical
// ecliptic and equinox of J2000, for τ Julian millennia of TDB from J2000.
func (p *vsopPlanet) heliocentric(tau float64) (l, b, r float64) {
	return evalSeries(p.vars[0], tau), evalSeries(p.vars[1], tau), evalSeries(p.vars[2], tau)
}

// rect returns heliocentric rectangular J2000 ecliptic coordinates (AU).
func (p *vsopPlanet) rect(tau float64) vec3 {
	l, b, r := p.heliocentric(tau)
	return sphericalToRect(l, b, r)
}
