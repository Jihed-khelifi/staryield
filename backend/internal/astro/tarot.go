package astro

import (
	"math"
	"sort"
)

// The Golden Dawn attributes the 22 Major Arcana to the seven classical
// planets, the twelve signs and the three elements (ruled here by the modern
// planets Uranus, Neptune and Pluto). Card keys match the artwork slugs.
var planetCards = map[Body]string{
	Sun: "the-sun", Moon: "the-high-priestess", Mercury: "the-magician", Venus: "the-empress",
	Mars: "the-tower", Jupiter: "wheel-of-fortune", Saturn: "the-world",
	Uranus: "the-fool", Neptune: "the-hanged-man", Pluto: "judgement",
}

var signCards = [12]string{
	"the-emperor", "the-hierophant", "the-lovers", "the-chariot", "strength", "the-hermit",
	"justice", "death", "temperance", "the-devil", "the-star", "the-moon",
}

// CardForPlanet returns the Major Arcana card attributed to a planet.
func CardForPlanet(b Body) string { return planetCards[b] }

// CardForSign returns the Major Arcana card attributed to a sign.
func CardForSign(sign string) string {
	if i := signIndex(sign); i >= 0 {
		return signCards[i]
	}
	return ""
}

// Base weights of the planets when several share a house.
var planetWeight = map[Body]float64{
	Sun: 10, Moon: 9, Mercury: 7, Venus: 7, Mars: 7, Jupiter: 6, Saturn: 6, Uranus: 4, Neptune: 4, Pluto: 4,
}

// Essential dignities (sign indexes: 0 = Aries … 11 = Pisces). Domicile
// includes the modern co-rulers; outer planets only have domicile and
// detriment.
var (
	domicile = map[Body][]int{
		Sun: {4}, Moon: {3}, Mercury: {2, 5}, Venus: {1, 6}, Mars: {0, 7}, Jupiter: {8, 11},
		Saturn: {9, 10}, Uranus: {10}, Neptune: {11}, Pluto: {7},
	}
	exaltation = map[Body][]int{Sun: {0}, Moon: {1}, Mercury: {5}, Venus: {11}, Mars: {9}, Jupiter: {3}, Saturn: {6}}
	detriment  = map[Body][]int{
		Sun: {10}, Moon: {9}, Mercury: {8, 11}, Venus: {7, 0}, Mars: {6, 1}, Jupiter: {2, 5},
		Saturn: {3, 4}, Uranus: {4}, Neptune: {5}, Pluto: {1},
	}
	fall = map[Body][]int{Sun: {6}, Moon: {7}, Mercury: {11}, Venus: {5}, Mars: {3}, Jupiter: {9}, Saturn: {0}}
)

func contains(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// EssentialDignity scores a planet in a sign: domicile +5, exaltation +4,
// detriment −5, fall −4, otherwise 0. When two apply (Mercury in Virgo is
// both domicile and exaltation), the higher score is used.
func EssentialDignity(b Body, sign string) int {
	i := signIndex(sign)
	if i < 0 {
		return 0
	}
	var scores []int
	if contains(domicile[b], i) {
		scores = append(scores, 5)
	}
	if contains(exaltation[b], i) {
		scores = append(scores, 4)
	}
	if contains(detriment[b], i) {
		scores = append(scores, -5)
	}
	if contains(fall[b], i) {
		scores = append(scores, -4)
	}
	if len(scores) == 0 {
		return 0
	}
	best := scores[0]
	for _, s := range scores[1:] {
		best = max(best, s)
	}
	return best
}

// cuspBonusOrb and cuspBonus reward a planet close to its house cusp.
const (
	cuspBonusOrb = 5.0
	cuspBonus    = 3.0
)

// TarotCard is one card of the Twelve Houses spread.
type TarotCard struct {
	House     int        `json:"house"`
	Card      string     `json:"card"`
	Reversed  bool       `json:"reversed"`
	Source    CardSource `json:"source"`
	Uncertain bool       `json:"uncertain"`
}

// CardSource explains why a card sits in a house.
type CardSource struct {
	Kind string `json:"kind"` // "planet" or "cusp_sign"
	Body string `json:"body,omitempty"`
	Sign string `json:"sign"`
	// Score is the planet's strength in the house (planet sources only).
	Score float64 `json:"score,omitempty"`
}

// TarotSpread is the Twelve Houses spread derived from a natal chart.
type TarotSpread struct {
	HouseSystem HouseSystem `json:"house_system"`
	Uncertain   bool        `json:"uncertain"`
	Cards       []TarotCard `json:"cards"`
}

// spreadPlanet is a planet placed in a house for the spread.
type spreadPlanet struct {
	body  Body
	lon   float64
	sign  string
	retro bool
}

// TwelveHouses lays out the Twelve Houses spread for chart c. Each house
// shows the Major Arcana card of its strongest planet, scored as the planet's
// base weight plus its essential dignity plus a bonus within 5° after the
// cusp; ties go to the planet nearest the cusp, then to the traditional
// order. A retrograde planet's card is reversed. An empty house shows the
// card of the sign on its cusp.
//
// When the chart has no houses (unknown birth time or place), whole-sign
// houses counted from the Sun's sign are used. alts are the same chart
// computed at the edges of the possible birth window; cards that differ
// from c in any of them are flagged uncertain, and the whole spread is
// uncertain when any card is or when the Sun's sign is.
func TwelveHouses(c Chart, alts ...Chart) TarotSpread {
	spread, _ := layout(c)
	for _, alt := range alts {
		other, _ := layout(alt)
		for i := range spread.Cards {
			o := other.Cards[i]
			if o.Card != spread.Cards[i].Card || o.Reversed != spread.Cards[i].Reversed {
				spread.Cards[i].Uncertain = true
				spread.Uncertain = true
			}
		}
	}
	if c.SunSignUncertain {
		spread.Uncertain = true
	}
	return spread
}

// layout builds the spread for one chart and returns the cusps used.
func layout(c Chart) (TarotSpread, [12]float64) {
	cusps, ok := c.Cusps()
	system := c.HouseSystemUsed
	if !ok {
		system = WholeSign
		sun, _ := c.PositionOf(Sun)
		first := float64(signIndex(sun.Sign)) * 30
		for i := range cusps {
			cusps[i] = norm360(first + 30*float64(i))
		}
	}
	var planets []spreadPlanet
	for _, p := range c.Positions {
		b := Body(p.Body)
		if !IsPlanet(b) {
			continue
		}
		planets = append(planets, spreadPlanet{body: b, lon: p.Longitude, sign: p.Sign, retro: p.Retrograde})
	}
	spread := TarotSpread{HouseSystem: system, Cards: make([]TarotCard, 12)}
	for h := 0; h < 12; h++ {
		type candidate struct {
			p     spreadPlanet
			score float64
			arc   float64
			order int
		}
		var cands []candidate
		for i, p := range planets {
			if HouseOf(p.lon, cusps) != h+1 {
				continue
			}
			arc := norm360(p.lon - cusps[h]) // forward arc from the cusp
			score := planetWeight[p.body] + float64(EssentialDignity(p.body, p.sign))
			if arc <= cuspBonusOrb {
				score += cuspBonus
			}
			cands = append(cands, candidate{p, score, arc, i})
		}
		cuspSign := SignForLongitude(cusps[h])
		card := TarotCard{House: h + 1}
		if len(cands) == 0 {
			card.Card = CardForSign(cuspSign)
			card.Source = CardSource{Kind: "cusp_sign", Sign: cuspSign}
		} else {
			sort.SliceStable(cands, func(i, j int) bool {
				if cands[i].score != cands[j].score {
					return cands[i].score > cands[j].score
				}
				if math.Abs(cands[i].arc-cands[j].arc) > 1e-9 {
					return cands[i].arc < cands[j].arc
				}
				return cands[i].order < cands[j].order
			})
			best := cands[0]
			card.Card = CardForPlanet(best.p.body)
			card.Reversed = best.p.retro
			card.Source = CardSource{Kind: "planet", Body: string(best.p.body), Sign: best.p.sign, Score: best.score}
		}
		spread.Cards[h] = card
	}
	return spread, cusps
}
