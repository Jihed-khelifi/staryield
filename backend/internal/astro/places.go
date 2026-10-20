package astro

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	// Embed the IANA time zone database so LocalBirthInstant works even on
	// hosts or minimal containers without system zoneinfo files. The system
	// database is still preferred when present.
	_ "time/tzdata"
)

// Place is a gazetteer entry.
type Place struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	CountryCode string  `json:"country_code"`
	Country     string  `json:"country"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	TimeZone    string  `json:"time_zone"`
}

// Location returns the place's coordinates as a GeoLocation.
func (p Place) Location() GeoLocation {
	return GeoLocation{Latitude: p.Latitude, Longitude: p.Longitude}
}

// maxSearchResults caps the number of places SearchPlaces returns.
const maxSearchResults = 20

// placeIndexEntry caches the folded search keys of a gazetteer entry.
type placeIndexEntry struct {
	place   Place
	names   []string // folded name followed by folded aliases
	country string   // folded country name
	code    string   // folded ISO code
}

var (
	placeIndex []placeIndexEntry
	placesByID map[string]Place
)

func init() {
	placeIndex = make([]placeIndexEntry, len(gazetteer))
	placesByID = make(map[string]Place, len(gazetteer))
	for i, p := range gazetteer {
		e := placeIndexEntry{
			place:   p,
			names:   []string{foldText(p.Name)},
			country: foldText(p.Country),
			code:    foldText(p.CountryCode),
		}
		for _, alias := range placeAliases[p.ID] {
			e.names = append(e.names, foldText(alias))
		}
		placeIndex[i] = e
		placesByID[p.ID] = p
	}
}

// Places returns a copy of the whole gazetteer.
func Places() []Place {
	out := make([]Place, len(gazetteer))
	copy(out, gazetteer)
	return out
}

// PlaceByID looks up a gazetteer entry by its stable ID, such as "paris-fr".
func PlaceByID(id string) (Place, bool) {
	p, ok := placesByID[strings.ToLower(strings.TrimSpace(id))]
	return p, ok
}

// Match ranks used by SearchPlaces; lower is better.
const (
	rankNamePrefix = iota
	rankNameWordPrefix
	rankCountryPrefix
	rankNameSubstring
	rankCountrySubstring
	noMatch
)

// SearchPlaces finds gazetteer entries whose name (or a common alternative
// spelling) or country matches query, ignoring case and accents. Prefix
// matches on the name rank first, then matches at the start of a later word
// in the name, then country prefixes, then substring matches; ties are
// ordered by name. A query of the form "city, country" restricts the city
// part to names and the country part to country names or ISO codes. An empty
// query returns nil, and limit is clamped to 1..20.
func SearchPlaces(query string, limit int) []Place {
	limit = max(1, min(limit, maxSearchResults))
	cityPart, countryPart, hasCountry := strings.Cut(query, ",")
	q := foldText(cityPart)
	cq := foldText(countryPart)
	if q == "" && (!hasCountry || cq == "") {
		return nil
	}

	type hit struct {
		rank  int
		entry *placeIndexEntry
	}
	var hits []hit
	for i := range placeIndex {
		e := &placeIndex[i]
		var rank int
		if hasCountry && cq != "" {
			if !strings.HasPrefix(e.country, cq) && e.code != cq {
				continue
			}
			if q == "" {
				rank = rankCountryPrefix
			} else if rank = nameRank(e, q); rank == noMatch {
				continue
			}
		} else {
			rank = nameRank(e, q)
			if strings.HasPrefix(e.country, q) && rank > rankCountryPrefix {
				rank = rankCountryPrefix
			} else if rank == noMatch && strings.Contains(e.country, q) {
				rank = rankCountrySubstring
			}
			if rank == noMatch {
				continue
			}
		}
		hits = append(hits, hit{rank, e})
	}

	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].rank != hits[j].rank {
			return hits[i].rank < hits[j].rank
		}
		if hits[i].entry.names[0] != hits[j].entry.names[0] {
			return hits[i].entry.names[0] < hits[j].entry.names[0]
		}
		return hits[i].entry.country < hits[j].entry.country
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]Place, len(hits))
	for i, h := range hits {
		out[i] = h.entry.place
	}
	return out
}

// nameRank scores how well the folded query q matches any of the entry's
// names.
func nameRank(e *placeIndexEntry, q string) int {
	best := noMatch
	for _, name := range e.names {
		switch {
		case strings.HasPrefix(name, q):
			return rankNamePrefix
		case strings.Contains(name, " "+q):
			best = min(best, rankNameWordPrefix)
		case strings.Contains(name, q):
			best = min(best, rankNameSubstring)
		}
	}
	return best
}

// foldText lowercases s, strips common Latin diacritics, turns punctuation
// into spaces and collapses runs of whitespace, so that "São Paulo",
// "sao-paulo" and "SAO  PAULO" all compare equal.
func foldText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := true // suppress leading spaces
	for _, r := range strings.ToLower(s) {
		if rep, ok := diacriticFolds[r]; ok {
			b.WriteString(rep)
			space = false
			continue
		}
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			space = false
		case unicode.Is(unicode.Mn, r):
			// Drop combining marks left by decomposed input.
		case r == '\'' || r == '’' || r == '.':
			// "Côte d'Ivoire" → "cote divoire"; "D.C." → "dc".
		default:
			if !space {
				b.WriteByte(' ')
				space = true
			}
		}
	}
	return strings.TrimRight(b.String(), " ")
}

// diacriticFolds maps lowercase accented Latin letters to ASCII.
var diacriticFolds = map[rune]string{
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'ā': "a", 'ă': "a", 'ą': "a",
	'æ': "ae", 'ç': "c", 'ć': "c", 'č': "c", 'ď': "d", 'đ': "d", 'ð': "d",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e", 'ē': "e", 'ė': "e", 'ę': "e", 'ě': "e",
	'ğ': "g", 'ì': "i", 'í': "i", 'î': "i", 'ï': "i", 'ī': "i", 'į': "i", 'ı': "i",
	'ł': "l", 'ľ': "l", 'ñ': "n", 'ń': "n", 'ň': "n",
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o", 'ō': "o", 'ő': "o", 'œ': "oe",
	'ř': "r", 'ś': "s", 'š': "s", 'ş': "s", 'ș': "s", 'ß': "ss", 'ť': "t", 'ţ': "t", 'ț': "t", 'þ': "th",
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ū': "u", 'ů': "u", 'ű': "u", 'ų': "u",
	'ý': "y", 'ÿ': "y", 'ź': "z", 'ż': "z", 'ž': "z",
}

// ErrInvalidClock is returned by LocalBirthInstant for a malformed time of
// day.
var ErrInvalidClock = errors.New("astro: invalid clock time, want HH:MM")

// LocalBirthInstant interprets the calendar day of date (its year, month and
// day as seen in date's own location; the time of day is ignored) together
// with a 24-hour wall clock "HH:MM" in the IANA time zone tz, and returns the
// corresponding instant in UTC. An empty clock means 12:00, the conventional
// choice when the birth time is unknown. An empty tz means UTC. For wall times
// skipped or repeated by a daylight-saving transition the result follows
// time.Date's normalisation.
func LocalBirthInstant(date time.Time, clock string, tz string) (time.Time, error) {
	hour, minute, err := parseClock(clock)
	if err != nil {
		return time.Time{}, err
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Time{}, fmt.Errorf("astro: unknown time zone %q: %w", tz, err)
	}
	y, m, d := date.Date()
	return time.Date(y, m, d, hour, minute, 0, 0, loc).UTC(), nil
}

// parseClock parses "HH:MM" (a one-digit hour is accepted) into hour and
// minute. An empty string yields 12:00.
func parseClock(clock string) (hour, minute int, err error) {
	clock = strings.TrimSpace(clock)
	if clock == "" {
		return 12, 0, nil
	}
	hs, ms, ok := strings.Cut(clock, ":")
	if !ok || len(hs) < 1 || len(hs) > 2 || len(ms) != 2 {
		return 0, 0, ErrInvalidClock
	}
	hour, err1 := strconv.Atoi(hs)
	minute, err2 := strconv.Atoi(ms)
	if err1 != nil || err2 != nil || !isDigits(hs) || !isDigits(ms) ||
		hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, ErrInvalidClock
	}
	return hour, minute, nil
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
