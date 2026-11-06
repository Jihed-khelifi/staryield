package astro

import (
	"testing"
	"time"
)

func TestGoldenDawnCards(t *testing.T) {
	seen := map[string]bool{}
	for _, b := range Planets {
		c := CardForPlanet(b)
		if c == "" || seen[c] {
			t.Errorf("%s → %q", b, c)
		}
		seen[c] = true
	}
	for _, s := range Signs {
		c := CardForSign(s)
		if c == "" || seen[c] {
			t.Errorf("%s → %q", s, c)
		}
		seen[c] = true
	}
	if len(seen) != 22 {
		t.Fatalf("%d distinct Major Arcana, want 22", len(seen))
	}
	for sign, want := range map[string]string{"Aries": "the-emperor", "Leo": "strength", "Libra": "justice", "Aquarius": "the-star", "pisces": "the-moon"} {
		if got := CardForSign(sign); got != want {
			t.Errorf("CardForSign(%s) = %s, want %s", sign, got, want)
		}
	}
	if CardForPlanet(Moon) != "the-high-priestess" || CardForPlanet(Jupiter) != "wheel-of-fortune" || CardForSign("Ophiuchus") != "" {
		t.Error("planet/sign card table")
	}
}

func TestEssentialDignity(t *testing.T) {
	tests := []struct {
		body Body
		sign string
		want int
	}{
		{Sun, "Leo", 5}, {Sun, "Aries", 4}, {Sun, "Aquarius", -5}, {Sun, "Libra", -4}, {Sun, "Gemini", 0},
		{Mercury, "Virgo", 5},                                                                         // domicile and exaltation: the higher counts
		{Mercury, "Pisces", -4},                                                                       // detriment and fall: the higher (less negative) counts
		{Saturn, "Aquarius", 5}, {Uranus, "Aquarius", 5}, {Uranus, "Leo", -5}, {Uranus, "Scorpio", 0}, // outer planets: no exaltation
		{Pluto, "Scorpio", 5}, {Neptune, "Virgo", -5}, {Moon, "Taurus", 4}, {Venus, "Virgo", -4}, {Mars, "Capricorn", 4},
		{Jupiter, "Cancer", 4}, {Moon, "nowhere", 0},
	}
	for _, tt := range tests {
		if got := EssentialDignity(tt.body, tt.sign); got != tt.want {
			t.Errorf("EssentialDignity(%s, %s) = %d, want %d", tt.body, tt.sign, got, tt.want)
		}
	}
}

// chartWith builds a chart with equal houses from 0° Aries and the given
// planets (longitude, speed).
func chartWith(planets map[Body][2]float64) Chart {
	var cusps [12]float64
	for i := range cusps {
		cusps[i] = float64(i * 30)
	}
	c := Chart{HouseSystemUsed: Equal, cusps: &cusps}
	for _, b := range Planets {
		if v, ok := planets[b]; ok {
			c.Positions = append(c.Positions, newPosition(b, v[0], 0, v[1]))
		}
	}
	return c
}

func TestTwelveHousesRules(t *testing.T) {
	c := chartWith(map[Body][2]float64{
		Sun:     {128, 1},    // Leo, 5th house, domicile, 8° from the cusp: 10+5 = 15
		Mercury: {121, 1.2},  // Leo, 5th house, within 5° of the cusp: 7+0+3 = 10
		Saturn:  {10, -0.05}, // Aries (fall), 1st house, retrograde: 6−4 = 2
		Venus:   {200, 1},    // Libra (domicile), 7th: 12
		Mars:    {205, 0.7},  // Libra (detriment), 7th: 7−5 = 2
		Moon:    {65, 13},    // Gemini, 3rd: 9
		Uranus:  {62, 0.01},  // Gemini, 3rd, within 5° of cusp: 4+0+3 = 7
	})
	s := TwelveHouses(c)
	if len(s.Cards) != 12 || s.HouseSystem != Equal || s.Uncertain {
		t.Fatalf("spread %+v", s)
	}
	want := map[int]struct {
		card     string
		reversed bool
		kind     string
	}{
		1:  {"the-world", true, "planet"},           // retrograde Saturn → reversed
		2:  {"the-hierophant", false, "cusp_sign"},  // empty: Taurus on the cusp
		3:  {"the-high-priestess", false, "planet"}, // Moon outweighs Uranus
		5:  {"the-sun", false, "planet"},            // Sun outweighs Mercury
		7:  {"the-empress", false, "planet"},        // Venus in domicile beats Mars in detriment
		12: {"the-moon", false, "cusp_sign"},        // Pisces
	}
	for house, w := range want {
		got := s.Cards[house-1]
		if got.House != house || got.Card != w.card || got.Reversed != w.reversed || got.Source.Kind != w.kind {
			t.Errorf("house %d = %+v, want %s reversed=%v %s", house, got, w.card, w.reversed, w.kind)
		}
	}
	if s.Cards[4].Source.Body != "Sun" || s.Cards[4].Source.Score != 15 || s.Cards[4].Source.Sign != "Leo" {
		t.Errorf("house 5 source %+v", s.Cards[4].Source)
	}
}

func TestTwelveHousesTieBreaks(t *testing.T) {
	// Mercury and Venus both score 7 in the 5th house (Leo: no dignity for
	// either, both more than 5° past the cusp); the one nearer the cusp wins.
	c := chartWith(map[Body][2]float64{Mercury: {132, 1}, Venus: {127, 1}})
	if got := TwelveHouses(c).Cards[4]; got.Source.Body != "Venus" || got.Source.Score != 7 {
		t.Errorf("tie broken by %+v, want Venus (7° from the cusp vs 12°)", got.Source)
	}
	// Equal score and equal arc: traditional order (Mercury before Venus).
	c = chartWith(map[Body][2]float64{Venus: {130, 1}, Mercury: {130, 1}})
	if got := TwelveHouses(c).Cards[4]; got.Source.Body != "Mercury" {
		t.Errorf("tie broken by %s, want Mercury", got.Source.Body)
	}
}

func TestTwelveHousesUnknownTime(t *testing.T) {
	// Without houses the spread uses whole signs from the Sun's sign.
	date := time.Date(1987, 11, 29, 0, 0, 0, 0, time.UTC)
	at, start, end := BirthWindow(date, 0, 0, false, time.UTC)
	c, err := ComputeChart(ChartRequest{Instant: at, WindowStart: start, WindowEnd: end})
	if err != nil {
		t.Fatal(err)
	}
	cStart, _ := ComputeChart(ChartRequest{Instant: start})
	cEnd, _ := ComputeChart(ChartRequest{Instant: end})
	s := TwelveHouses(c, cStart, cEnd)
	if s.HouseSystem != WholeSign {
		t.Errorf("house system %s, want whole_sign", s.HouseSystem)
	}
	// The Sun is in Sagittarius: the first house is Sagittarius.
	if s.Cards[0].Source.Kind == "cusp_sign" && s.Cards[0].Card != "temperance" {
		t.Errorf("first house card %+v", s.Cards[0])
	}
	sun, _ := c.PositionOf(Sun)
	if sun.Sign != "Sagittarius" {
		t.Fatalf("Sun in %s", sun.Sign)
	}
	// The Moon changes sign during the day, so the card of the house it
	// occupies (or leaves) must be flagged.
	if !s.Uncertain {
		t.Error("spread with a Moon ingress during the birth day should be uncertain")
	}
	found := false
	for _, card := range s.Cards {
		found = found || card.Uncertain
	}
	if !found {
		t.Error("no card flagged uncertain")
	}
}

func TestTwelveHousesDeterministic(t *testing.T) {
	at := time.Date(1990, 5, 17, 8, 30, 0, 0, time.UTC)
	loc := GeoLocation{Latitude: 36.8065, Longitude: 10.1815}
	a, _ := ComputeChart(ChartRequest{Instant: at, TimeKnown: true, Loc: &loc})
	b, _ := ComputeChart(ChartRequest{Instant: at, TimeKnown: true, Loc: &loc})
	sa, sb := TwelveHouses(a), TwelveHouses(b)
	for i := range sa.Cards {
		if sa.Cards[i] != sb.Cards[i] {
			t.Fatalf("card %d differs: %+v vs %+v", i, sa.Cards[i], sb.Cards[i])
		}
	}
	if sa.HouseSystem != Placidus {
		t.Errorf("house system %s", sa.HouseSystem)
	}
}
