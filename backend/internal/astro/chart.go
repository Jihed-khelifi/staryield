package astro

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/astro/ephem"
)

// GeoLocation is a point on the Earth in degrees, with latitude positive
// north and longitude positive east.
type GeoLocation struct {
	Latitude  float64
	Longitude float64
}

// Position is a body or chart point placed in the zodiac.
type Position struct {
	Body       string  `json:"body"`
	Longitude  float64 `json:"longitude"`
	Latitude   float64 `json:"latitude"`
	Speed      float64 `json:"speed"` // degrees per day; negative when retrograde
	Sign       string  `json:"sign"`
	Degree     int     `json:"degree"`
	Minute     int     `json:"minute"`
	Second     int     `json:"second"`
	Formatted  string  `json:"formatted"`
	House      int     `json:"house,omitempty"`
	HouseLabel string  `json:"house_label,omitempty"`
	Retrograde bool    `json:"retrograde"`
}

// ChartAngles holds the four angles (ecliptic longitudes in degrees).
type ChartAngles struct {
	Asc float64 `json:"asc"`
	MC  float64 `json:"mc"`
	Dsc float64 `json:"dsc"`
	IC  float64 `json:"ic"`
}

// HouseCusp is the start of one house.
type HouseCusp struct {
	Number int     `json:"number"`
	Cusp   float64 `json:"cusp"`
	Sign   string  `json:"sign"`
}

// Chart is a natal (or any moment's) chart.
type Chart struct {
	Instant              time.Time      `json:"instant"`
	TimeKnown            bool           `json:"time_known"`
	HasLocation          bool           `json:"has_location"`
	HouseSystem          HouseSystem    `json:"house_system,omitempty"`
	HouseSystemUsed      HouseSystem    `json:"house_system_used,omitempty"`
	FallbackReason       string         `json:"fallback_reason,omitempty"`
	SunSign              string         `json:"sun_sign"`
	SunSignUncertain     bool           `json:"sun_sign_uncertain"`
	SunSignAlternatives  []string       `json:"sun_sign_alternatives,omitempty"`
	MoonSign             string         `json:"moon_sign"`
	MoonSignUncertain    bool           `json:"moon_sign_uncertain"`
	MoonSignAlternatives []string       `json:"moon_sign_alternatives,omitempty"`
	RisingSign           string         `json:"rising_sign,omitempty"`
	Angles               *ChartAngles   `json:"angles,omitempty"`
	Positions            []Position     `json:"positions"`
	HouseCusps           []float64      `json:"house_cusps,omitempty"`
	Houses               []HouseCusp    `json:"houses,omitempty"`
	Aspects              []Aspect       `json:"aspects"`
	Element              map[string]int `json:"elements"`
	Modality             map[string]int `json:"modalities"`

	// cusps keeps the unrounded cusps for house assignments (tarot, client
	// cards) without re-parsing Houses.
	cusps *[12]float64
}

// Cusps returns the twelve house cusps and true when the chart has houses.
func (c Chart) Cusps() ([12]float64, bool) {
	if c.cusps == nil {
		return [12]float64{}, false
	}
	return *c.cusps, true
}

// PositionOf returns the position of body b in the chart.
func (c Chart) PositionOf(b Body) (Position, bool) {
	for _, p := range c.Positions {
		if p.Body == string(b) {
			return p, true
		}
	}
	return Position{}, false
}

// ChartRequest describes the moment and place to chart.
type ChartRequest struct {
	// Instant is the birth instant (UTC); when the time is unknown it is the
	// representative instant from BirthWindow (local or UTC noon).
	Instant time.Time
	// TimeKnown enables the Ascendant, Midheaven and houses (with Loc).
	TimeKnown bool
	// Loc is the birthplace; nil when unknown.
	Loc *GeoLocation
	// System is the house system (default Placidus).
	System HouseSystem
	// WindowStart and WindowEnd bound the possible birth instants when the
	// time (or place) is unknown; signs that change within the window are
	// flagged uncertain. Zero values mean the instant is exact.
	WindowStart, WindowEnd time.Time
}

// ErrOutOfRange is returned for instants the ephemeris does not cover.
var ErrOutOfRange = errors.New("astro: date outside the supported range")

// houseLabels names houses 1..12 (index 0 is the first house).
var houseLabels = [12]string{
	"First house", "Second house", "Third house", "Fourth house",
	"Fifth house", "Sixth house", "Seventh house", "Eighth house",
	"Ninth house", "Tenth house", "Eleventh house", "Twelfth house",
}

// HouseLabel returns the English label for house n (1..12), such as
// "First house", or "" when n is out of range.
func HouseLabel(n int) string {
	if n < 1 || n > 12 {
		return ""
	}
	return houseLabels[n-1]
}

// moonWindowStep is the sampling step used to list every sign the Moon
// crosses during an uncertain birth window (it needs about 2.5 days per sign).
const moonWindowStep = 2 * time.Hour

// ComputeChart computes the positions of the ten planets at req.Instant.
// When the time is known and req.Loc is set it also computes the angles,
// the houses (Placidus by default, Porphyry inside the polar circles) and
// assigns every point to a house. Aspects between the planets (and the
// angles, when known) are always included.
func ComputeChart(req ChartRequest) (Chart, error) {
	instant := req.Instant.UTC()
	sys := req.System
	if sys == "" {
		sys = DefaultHouseSystem
	}
	c := Chart{
		Instant:     instant,
		TimeKnown:   req.TimeKnown,
		HasLocation: req.Loc != nil,
		Element:     map[string]int{Fire: 0, Earth: 0, Air: 0, Water: 0},
		Modality:    map[string]int{Cardinal: 0, Fixed: 0, Mutable: 0},
		Positions:   make([]Position, 0, len(Planets)+2),
		Aspects:     []Aspect{},
	}

	coords, errs := ephem.Default().Positions(ephem.JulianDay(instant))
	for i := range errs {
		if errs[i] != nil {
			if Planets[i] == Pluto && errors.Is(errs[i], ephem.ErrOutOfRange) && errs[ephem.Sun] == nil {
				continue // Pluto alone is undefined before 1885 or after 2099
			}
			return Chart{}, fmt.Errorf("%w: %v", ErrOutOfRange, errs[i])
		}
	}

	withHouses := req.TimeKnown && req.Loc != nil
	var cusps [12]float64
	var points []aspectPoint
	if withHouses {
		asc, mc, ramc, eps := anglesAt(instant, *req.Loc)
		var used HouseSystem
		cusps, used, c.FallbackReason = Houses(sys, ramc, eps, req.Loc.Latitude)
		c.HouseSystem, c.HouseSystemUsed = sys, used
		c.cusps = &cusps
		c.HouseCusps = make([]float64, 12)
		c.Houses = make([]HouseCusp, 12)
		for i, cusp := range cusps {
			c.HouseCusps[i] = roundTo(cusp, 6)
			c.Houses[i] = HouseCusp{Number: i + 1, Cusp: roundTo(cusp, 6), Sign: SignForLongitude(cusp)}
		}
		c.Angles = &ChartAngles{Asc: roundTo(asc, 6), MC: roundTo(mc, 6), Dsc: roundTo(norm360(asc+180), 6), IC: roundTo(norm360(mc+180), 6)}
		ap := newPosition(Ascendant, asc, 0, 0)
		c.RisingSign = ap.Sign
		c.Positions = append(c.Positions, ap)
		points = append(points, aspectPoint{body: Ascendant, lon: asc, angle: true})
		points = append(points, aspectPoint{body: Midheaven, lon: mc, angle: true})
	}

	for i, b := range Planets {
		if errs[i] != nil {
			continue
		}
		co := coords[i]
		p := newPosition(b, co.Lon, co.Lat, co.Speed)
		c.Positions = append(c.Positions, p)
		c.Element[ElementOf(p.Sign)]++
		c.Modality[ModalityOf(p.Sign)]++
		points = append(points, aspectPoint{body: b, lon: co.Lon, speed: co.Speed})
		switch b {
		case Sun:
			c.SunSign = p.Sign
		case Moon:
			c.MoonSign = p.Sign
		}
	}

	if withHouses {
		c.Positions = append(c.Positions, newPosition(Midheaven, c.Angles.MC, 0, 0))
		for i := range c.Positions {
			p := &c.Positions[i]
			p.House = HouseOf(p.Longitude, cusps)
			p.HouseLabel = HouseLabel(p.House)
		}
		// The angles define cusps 1 and 10 exactly.
		c.Positions[0].House, c.Positions[0].HouseLabel = 1, HouseLabel(1)
		last := len(c.Positions) - 1
		c.Positions[last].House, c.Positions[last].HouseLabel = 10, HouseLabel(10)
	}
	c.Aspects = findAspects(orderAspectPoints(points))

	if !req.WindowStart.IsZero() && !req.WindowEnd.IsZero() && req.WindowEnd.After(req.WindowStart) {
		sun := SunSignForBirth(instant, req.WindowStart, req.WindowEnd)
		c.SunSignUncertain, c.SunSignAlternatives = sun.Uncertain, sun.Alternatives
		c.MoonSignAlternatives = signsCrossed(Moon, req.WindowStart, req.WindowEnd, moonWindowStep)
		c.MoonSignUncertain = len(c.MoonSignAlternatives) > 1
		if !c.MoonSignUncertain {
			c.MoonSignAlternatives = nil
		}
	}
	return c, nil
}

// orderAspectPoints lists planets first (in traditional order) and the
// angles last, so aspects read "Sun trine Moon", "Mars square Ascendant".
func orderAspectPoints(points []aspectPoint) []aspectPoint {
	out := make([]aspectPoint, 0, len(points))
	for _, p := range points {
		if !p.angle {
			out = append(out, p)
		}
	}
	for _, p := range points {
		if p.angle {
			out = append(out, p)
		}
	}
	return out
}

// signsCrossed lists, in order, the distinct signs b occupies between start
// and end, sampling every step (and at both ends).
func signsCrossed(b Body, start, end time.Time, step time.Duration) []string {
	var out []string
	add := func(t time.Time) {
		s := SignForLongitude(EclipticLongitude(b, t))
		if s != "" && (len(out) == 0 || out[len(out)-1] != s) {
			out = append(out, s)
		}
	}
	for t := start; t.Before(end); t = t.Add(step) {
		add(t)
	}
	add(end)
	return out
}

// arcsecondsPerCircle and arcsecondsPerSign express the zodiac in whole
// arcseconds so that the sign, degree, minute and second always agree after
// rounding.
const (
	arcsecondsPerCircle = 360 * 3600
	arcsecondsPerSign   = 30 * 3600
)

// newPosition builds a Position for body at ecliptic longitude lon. The
// longitude is rounded to the nearest arcsecond before being split into
// sign, degree, minute and second, so the parts always agree.
func newPosition(body Body, lon, lat, speed float64) Position {
	lon = norm360(lon)
	total := int(math.Round(lon*3600)) % arcsecondsPerCircle
	within := total % arcsecondsPerSign
	p := Position{
		Body:       string(body),
		Longitude:  norm360(roundTo(lon, 6)),
		Latitude:   roundTo(lat, 6),
		Speed:      roundTo(speed, 6),
		Sign:       Signs[total/arcsecondsPerSign],
		Degree:     within / 3600,
		Minute:     within % 3600 / 60,
		Second:     within % 60,
		Retrograde: canRetrograde(body) && speed < 0,
	}
	p.Formatted = FormatZodiac(p.Sign, p.Degree, p.Minute, p.Second)
	return p
}

// FormatZodiac renders a zodiac position the way the frontend displays it,
// for example "Cancer, 8°22′47″" (minutes and seconds are not zero-padded).
func FormatZodiac(sign string, degree, minute, second int) string {
	return fmt.Sprintf("%s, %d°%d′%d″", sign, degree, minute, second)
}
