package astro

import (
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"time"
)

// DailyHoroscope is the reflective daily reading for one sign and date.
type DailyHoroscope struct {
	Sign               string `json:"sign"`
	Date               string `json:"date"`
	Text               string `json:"text"`
	Focus              string `json:"focus"`
	Affirmation        string `json:"affirmation"`
	Mood               string `json:"mood"`
	LuckyNumber        int    `json:"lucky_number"`
	Compatibility      int    `json:"compatibility"`
	CompatibilityLabel string `json:"compatibility_label"`
	MoonSign           string `json:"moon_sign"`
	MoonPhase          string `json:"moon_phase"`
}

// Moods a daily reading can carry.
const (
	MoodEmpowered  = "Empowered"
	MoodHopeful    = "Hopeful"
	MoodReflective = "Reflective"
	MoodCalm       = "Calm"
	MoodCurious    = "Curious"
)

// Moods lists every mood Daily can return.
var Moods = [5]string{MoodEmpowered, MoodHopeful, MoodReflective, MoodCalm, MoodCurious}

// Compatibility labels, by score band.
const (
	CompatibilityHigh     = "High"     // 80 and above
	CompatibilityBalanced = "Balanced" // 65 to 79
	CompatibilityGentle   = "Gentle"   // below 65
)

// ErrUnknownSign is returned (wrapped) by Daily for an unrecognised sign.
var ErrUnknownSign = errors.New("astro: unknown zodiac sign")

// Daily composes the daily horoscope for sign (case-insensitive) on the
// calendar day of date (its year, month and day in date's own location).
//
// The sky is sampled at 12:00 UTC on that day for the Moon's sign and phase.
// Every other choice — sentences, focus, affirmation, mood and numbers — is
// drawn from curated banks by a generator seeded with an FNV-1a hash of the
// sign and date, so the same sign and date always produce the same reading,
// while different signs and days vary. The compatibility score expresses how
// harmoniously the day's Moon element sits with the sign's element.
func Daily(sign string, date time.Time) (DailyHoroscope, error) {
	canonical, ok := CanonicalSign(sign)
	if !ok {
		return DailyHoroscope{}, fmt.Errorf("%w: %q", ErrUnknownSign, sign)
	}
	y, m, d := date.Date()
	noon := time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
	day := noon.Format(time.DateOnly)

	moonSign := SignForLongitude(EclipticLongitude(Moon, noon))
	phase := PhaseName(noon)
	element := ElementOf(canonical)
	moonElement := ElementOf(moonSign)
	relation := elementRelation(element, moonElement)

	pick := newPicker(canonical + "|" + day)
	fill := strings.NewReplacer(
		"{sign}", canonical,
		"{moon_sign}", moonSign,
		"{phase}", phase,
		"{phase_noun}", phaseNoun(phase),
		"{moon_theme}", pick.from(moonThemes[moonSign]),
		"{phase_meaning}", pick.from(phaseMeanings[phase]),
		"{element}", strings.ToLower(element),
		"{a_element}", withArticle(strings.ToLower(element)),
		"{moon_element}", strings.ToLower(moonElement),
		"{a_moon_element}", withArticle(strings.ToLower(moonElement)),
	).Replace

	sentences := []string{
		fill(pick.from(openers)),
		fill(pick.from(moonLines)),
	}
	relationLine := fill(pick.from(relationLines[relation]))
	elementLine := fill(pick.from(elementLines[element]))
	genericLine := fill(pick.from(genericLines))
	switch pick.intn(4) {
	case 0:
		sentences = append(sentences, relationLine, elementLine)
	case 1:
		sentences = append(sentences, elementLine, genericLine)
	case 2:
		sentences = append(sentences, relationLine, elementLine, genericLine)
	default:
		sentences = append(sentences, elementLine)
	}
	sentences = append(sentences, fill(pick.from(closers)))

	focus := pick.from(genericFocus)
	if pick.intn(2) == 0 {
		focus = pick.from(elementFocus[element])
	}

	lo, hi := compatibilityRange(relation)
	score := lo + pick.intn(hi-lo+1)
	moods := phaseMoods[phase]

	return DailyHoroscope{
		Sign:               canonical,
		Date:               day,
		Text:               strings.Join(sentences, " "),
		Focus:              focus,
		Affirmation:        pick.from(affirmations),
		Mood:               pick.from(moods[:]),
		LuckyNumber:        1 + pick.intn(99),
		Compatibility:      score,
		CompatibilityLabel: compatibilityLabel(score),
		MoonSign:           moonSign,
		MoonPhase:          phase,
	}, nil
}

// phaseNoun turns a phase name into a noun phrase ("Waxing Gibbous Moon").
func phaseNoun(phase string) string {
	if strings.HasSuffix(phase, "Moon") {
		return phase
	}
	return phase + " Moon"
}

// withArticle prefixes a lowercase word with "a" or "an".
func withArticle(word string) string {
	if word != "" && strings.ContainsRune("aeiou", rune(word[0])) {
		return "an " + word
	}
	return "a " + word
}

// Element relations between a sign and the day's Moon.
const (
	relationSame       = "same"       // e.g. fire and fire
	relationSupportive = "supportive" // fire–air or earth–water
	relationContrast   = "contrast"   // any other pairing
)

func elementRelation(a, b string) string {
	switch {
	case a == b:
		return relationSame
	case (a == Fire && b == Air) || (a == Air && b == Fire) ||
		(a == Earth && b == Water) || (a == Water && b == Earth):
		return relationSupportive
	default:
		return relationContrast
	}
}

// compatibilityRange returns the inclusive score band for a relation; all
// bands lie within 55..98.
func compatibilityRange(relation string) (lo, hi int) {
	switch relation {
	case relationSame:
		return 80, 98
	case relationSupportive:
		return 68, 94
	default:
		return 55, 84
	}
}

func compatibilityLabel(score int) string {
	switch {
	case score >= 80:
		return CompatibilityHigh
	case score >= 65:
		return CompatibilityBalanced
	default:
		return CompatibilityGentle
	}
}

// picker is a small deterministic pseudo-random source (SplitMix64) seeded
// from a string. It is implemented here rather than using math/rand so the
// sequence can never change between Go releases.
type picker struct{ state uint64 }

func newPicker(seed string) *picker {
	h := fnv.New64a()
	h.Write([]byte(seed))
	return &picker{state: h.Sum64()}
}

func (p *picker) next() uint64 {
	p.state += 0x9e3779b97f4a7c15
	z := p.state
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// intn returns a value in [0, n). n must be positive.
func (p *picker) intn(n int) int { return int(p.next() % uint64(n)) }

// from returns one element of options.
func (p *picker) from(options []string) string { return options[p.intn(len(options))] }
