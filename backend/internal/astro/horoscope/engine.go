// Package horoscope composes Staryield's daily horoscope readings in English,
// French and Spanish, and maps them to the response shape of the public
// "aztro" API (https://github.com/sameerkumar18/aztro), whose hosted service
// and upstream source are no longer available.
//
// Readings are deterministic: the day's sky (the Moon's sign and phase at
// local noon) is computed with the package astro ephemeris, and every other
// choice is drawn from curated, index-aligned sentence banks by a generator
// seeded with the sign and the date, so a reader sees the same reading all
// day, in every language, on every server.
package horoscope

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/astro"
)

// EngineVersion identifies the composition rules; bump it when readings
// change so caches keyed on it are refreshed.
const EngineVersion = "staryield-horoscope-2"

// Moods a reading can carry (canonical keys).
const (
	MoodEmpowered  = "Empowered"
	MoodHopeful    = "Hopeful"
	MoodReflective = "Reflective"
	MoodCalm       = "Calm"
	MoodCurious    = "Curious"
)

// Compatibility labels by score band (canonical keys).
const (
	CompatibilityHigh     = "High"     // 80 and above
	CompatibilityBalanced = "Balanced" // 65 to 79
	CompatibilityGentle   = "Gentle"   // below 65
)

// phaseMoods lists the moods that fit each lunar phase.
var phaseMoods = map[string][3]string{
	astro.PhaseNewMoon:        {MoodHopeful, MoodReflective, MoodCalm},
	astro.PhaseWaxingCrescent: {MoodHopeful, MoodCurious, MoodEmpowered},
	astro.PhaseFirstQuarter:   {MoodEmpowered, MoodCurious, MoodHopeful},
	astro.PhaseWaxingGibbous:  {MoodCurious, MoodEmpowered, MoodHopeful},
	astro.PhaseFullMoon:       {MoodEmpowered, MoodReflective, MoodHopeful},
	astro.PhaseWaningGibbous:  {MoodReflective, MoodCalm, MoodHopeful},
	astro.PhaseLastQuarter:    {MoodReflective, MoodCalm, MoodCurious},
	astro.PhaseWaningCrescent: {MoodCalm, MoodReflective, MoodHopeful},
}

// colorKeys are the canonical (English) color names, index-aligned with
// every locale's colors.
var colorKeys = []string{"Gold", "Ivory", "Sage", "Amber", "Cream", "Rose", "Lavender", "Teal", "Midnight Blue", "Copper", "Silver", "Emerald",
	"Coral", "Navy", "Olive", "Plum", "Turquoise", "Burgundy", "Peach", "Sky Blue", "Charcoal", "Mint", "Saffron", "Indigo"}

// ErrUnknownSign is returned for an unrecognised sign.
var ErrUnknownSign = errors.New("horoscope: unknown zodiac sign")

// DateRange is the period the Sun spends in a sign during a year.
type DateRange struct {
	Start string `json:"start"` // YYYY-MM-DD
	End   string `json:"end"`   // YYYY-MM-DD
	Label string `json:"label"` // "Mar 20 - Apr 19", localised
}

// Reading is the daily horoscope for one sign and calendar date.
type Reading struct {
	Sign                  string    `json:"sign"`      // canonical English key
	SignName              string    `json:"sign_name"` // localised
	Date                  string    `json:"date"`      // YYYY-MM-DD in TZ
	CurrentDate           string    `json:"current_date"`
	Locale                string    `json:"locale"`
	TZ                    string    `json:"tz"`
	Text                  string    `json:"text"`
	Focus                 string    `json:"focus"`
	Affirmation           string    `json:"affirmation"`
	Mood                  string    `json:"mood"`
	MoodKey               string    `json:"mood_key"`
	LuckyNumber           int       `json:"lucky_number"`
	LuckyHour             int       `json:"lucky_hour"`
	LuckyTime             string    `json:"lucky_time"`
	Color                 string    `json:"color"`
	ColorKey              string    `json:"color_key"`
	Compatibility         int       `json:"compatibility"` // score, kept for existing clients
	CompatibilityScore    int       `json:"compatibility_score"`
	CompatibilityLabel    string    `json:"compatibility_label"`
	CompatibilityLabelKey string    `json:"compatibility_label_key"`
	CompatibleSign        string    `json:"compatible_sign"`
	CompatibleSignName    string    `json:"compatible_sign_name"`
	MoonSign              string    `json:"moon_sign"`
	MoonSignName          string    `json:"moon_sign_name"`
	MoonPhase             string    `json:"moon_phase"`
	MoonPhaseName         string    `json:"moon_phase_name"`
	DateRange             DateRange `json:"date_range"`
	Provider              string    `json:"provider"`
	EngineVersion         string    `json:"engine_version"`
}

// Request asks for one reading.
type Request struct {
	Sign     string         // any case
	Date     time.Time      // its calendar day (year, month, day) is used
	Location *time.Location // the reader's time zone (noon there is sampled); nil means UTC
	Locale   string         // en, fr or es (others fall back to en)
}

// Provider produces readings.
type Provider interface {
	Name() string
	Daily(ctx context.Context, req Request) (Reading, error)
}

// Engine is the built-in provider.
type Engine struct{}

// Name implements Provider.
func (Engine) Name() string { return "staryield" }

// Daily implements Provider.
func (Engine) Daily(_ context.Context, req Request) (Reading, error) { return Daily(req) }

// Element relations between a sign and the day's Moon.
const (
	relationSame       = "same"
	relationSupportive = "supportive"
	relationContrast   = "contrast"
)

func elementRelation(a, b string) string {
	switch {
	case a == b:
		return relationSame
	case (a == astro.Fire && b == astro.Air) || (a == astro.Air && b == astro.Fire) ||
		(a == astro.Earth && b == astro.Water) || (a == astro.Water && b == astro.Earth):
		return relationSupportive
	default:
		return relationContrast
	}
}

// compatibilityRange returns the inclusive score band for a relation.
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

// Daily composes the reading for req.
func Daily(req Request) (Reading, error) {
	canonical, ok := astro.CanonicalSign(req.Sign)
	if !ok {
		return Reading{}, fmt.Errorf("%w: %q", ErrUnknownSign, req.Sign)
	}
	loc := req.Location
	if loc == nil {
		loc = time.UTC
	}
	locale := NormalizeLocale(req.Locale)
	c, err := ContentFor(locale)
	if err != nil {
		return Reading{}, err
	}
	en, err := ContentFor("en")
	if err != nil {
		return Reading{}, err
	}
	y, m, d := req.Date.Date()
	noon := time.Date(y, m, d, 12, 0, 0, 0, loc)
	day := time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Format(time.DateOnly)

	moonSign := astro.SignForLongitude(astro.EclipticLongitude(astro.Moon, noon))
	if moonSign == "" {
		return Reading{}, astro.ErrOutOfRange
	}
	phase := astro.PhaseName(noon)
	element := astro.ElementOf(canonical)
	moonElement := astro.ElementOf(moonSign)
	relation := elementRelation(element, moonElement)

	// The pick order is part of the reading's identity; append new picks
	// at the end only.
	pick := newPicker(canonical + "|" + day)
	themeIdx := pick.intn(len(en.MoonThemes[moonSign]))
	meaningIdx := pick.intn(len(en.PhaseMeanings[phase]))
	openerIdx := pick.intn(len(en.Openers))
	moonLineIdx := pick.intn(len(en.MoonLines))
	relationIdx := pick.intn(len(en.RelationLines[relation]))
	elementIdx := pick.intn(len(en.ElementLines[element]))
	genericIdx := pick.intn(len(en.GenericLines))
	shape := pick.intn(4)
	closerIdx := pick.intn(len(en.Closers))
	focusText := c.GenericFocus[pick.intn(len(en.GenericFocus))]
	if pick.intn(2) == 0 {
		focusText = c.ElementFocus[element][pick.intn(len(en.ElementFocus[element]))]
	}
	lo, hi := compatibilityRange(relation)
	score := lo + pick.intn(hi-lo+1)
	affirmation := c.Affirmations[pick.intn(len(en.Affirmations))]
	moods := phaseMoods[phase]
	moodKey := moods[pick.intn(len(moods))]
	lucky := 1 + pick.intn(99)
	colorIdx := pick.intn(len(colorKeys))
	luckyHour := 6 + pick.intn(16) // 06:00–21:00
	compatible := compatibleSign(canonical, moonSign, pick)

	fill := strings.NewReplacer(
		"{sign}", c.Signs[canonical],
		"{moon_sign}", c.Signs[moonSign],
		"{phase_noun}", c.PhaseNouns[phase],
		"{phase_meaning}", c.PhaseMeanings[phase][meaningIdx],
		"{phase}", c.Phases[phase],
		"{moon_theme}", c.MoonThemes[moonSign][themeIdx],
		"{a_moon_element}", c.ElementsWithArticle[moonElement],
		"{moon_element}", c.Elements[moonElement],
		"{a_element}", c.ElementsWithArticle[element],
		"{element}", c.Elements[element],
	).Replace

	sentences := []string{fill(c.Openers[openerIdx]), fill(c.MoonLines[moonLineIdx])}
	relationLine := fill(c.RelationLines[relation][relationIdx])
	elementLine := fill(c.ElementLines[element][elementIdx])
	genericLine := fill(c.GenericLines[genericIdx])
	switch shape {
	case 0:
		sentences = append(sentences, relationLine, elementLine)
	case 1:
		sentences = append(sentences, elementLine, genericLine)
	case 2:
		sentences = append(sentences, relationLine, elementLine, genericLine)
	default:
		sentences = append(sentences, elementLine)
	}
	sentences = append(sentences, fill(c.Closers[closerIdx]))

	label := compatibilityLabel(score)
	return Reading{
		Sign:                  canonical,
		SignName:              c.Signs[canonical],
		Date:                  day,
		CurrentDate:           FormatDate(locale, y, m, d),
		Locale:                locale,
		TZ:                    loc.String(),
		Text:                  strings.Join(sentences, " "),
		Focus:                 focusText,
		Affirmation:           affirmation,
		Mood:                  c.Moods[moodKey],
		MoodKey:               moodKey,
		LuckyNumber:           lucky,
		LuckyHour:             luckyHour,
		LuckyTime:             FormatHour(locale, luckyHour),
		Color:                 c.Colors[colorIdx],
		ColorKey:              colorKeys[colorIdx],
		Compatibility:         score,
		CompatibilityScore:    score,
		CompatibilityLabel:    c.CompatibilityLabels[label],
		CompatibilityLabelKey: label,
		CompatibleSign:        compatible,
		CompatibleSignName:    c.Signs[compatible],
		MoonSign:              moonSign,
		MoonSignName:          c.Signs[moonSign],
		MoonPhase:             phase,
		MoonPhaseName:         c.PhaseNames[phase],
		DateRange:             signDateRange(c, canonical, y, m, d, loc),
		Provider:              Engine{}.Name(),
		EngineVersion:         EngineVersion,
	}, nil
}

// compatibleSign picks a sign of the same or a supportive element (never the
// sign itself), favouring the Moon's sign when it qualifies.
func compatibleSign(sign, moonSign string, pick *picker) string {
	element := astro.ElementOf(sign)
	var candidates []string
	moonOK := false
	for _, s := range astro.Signs {
		if s == sign {
			continue
		}
		if r := elementRelation(element, astro.ElementOf(s)); r == relationSame || r == relationSupportive {
			candidates = append(candidates, s)
			if s == moonSign {
				moonOK = true
			}
		}
	}
	if moonOK && pick.intn(2) == 0 {
		return moonSign
	}
	return candidates[pick.intn(len(candidates))]
}

// FormatHour renders an hour of the day the way each locale writes it:
// "3pm", "15 h", "15:00".
func FormatHour(locale string, h int) string {
	switch locale {
	case "fr":
		return fmt.Sprintf("%d h", h)
	case "es":
		return fmt.Sprintf("%d:00", h)
	}
	suffix := "am"
	if h >= 12 {
		suffix = "pm"
	}
	h12 := h % 12
	if h12 == 0 {
		h12 = 12
	}
	return fmt.Sprintf("%d%s", h12, suffix)
}

// FormatDate renders a calendar date: "November 4, 2026", "4 novembre 2026"
// ("1er" on the first), "4 de noviembre de 2026".
func FormatDate(locale string, y int, m time.Month, d int) string {
	c, err := ContentFor(locale)
	if err != nil {
		return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
	}
	month := c.Months[m-1]
	switch NormalizeLocale(locale) {
	case "fr":
		day := fmt.Sprint(d)
		if d == 1 {
			day = "1er"
		}
		return fmt.Sprintf("%s %s %d", day, month, y)
	case "es":
		return fmt.Sprintf("%d de %s de %d", d, month, y)
	}
	return fmt.Sprintf("%s %d, %d", month, d, y)
}

// rangeCache memoises sign date ranges per sign, year and zone.
var rangeCache sync.Map

func signDateRange(c *Content, sign string, y int, m time.Month, d int, loc *time.Location) DateRange {
	year := y
	// Capricorn straddles the new year: in January use the period that
	// began in the previous December.
	if sign == "Capricorn" && m == time.January {
		year--
	}
	key := fmt.Sprintf("%s|%d|%s", sign, year, loc.String())
	var first, last time.Time
	if v, ok := rangeCache.Load(key); ok {
		r := v.([2]time.Time)
		first, last = r[0], r[1]
	} else {
		var ok bool
		first, last, ok = astro.SunSignDates(sign, year, loc)
		if !ok {
			return DateRange{}
		}
		rangeCache.Store(key, [2]time.Time{first, last})
	}
	label := ""
	short := func(t time.Time) (string, int) { return c.MonthsShort[t.Month()-1], t.Day() }
	fm, fd := short(first)
	lm, ld := short(last)
	switch c.Locale {
	case "fr", "es":
		label = fmt.Sprintf("%d %s - %d %s", fd, fm, ld, lm)
	default:
		label = fmt.Sprintf("%s %d - %s %d", fm, fd, lm, ld)
	}
	return DateRange{Start: first.Format(time.DateOnly), End: last.Format(time.DateOnly), Label: label}
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
