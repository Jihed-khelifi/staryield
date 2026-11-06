// Package geo is the embedded birthplace gazetteer: about 70,000 populated
// places from GeoNames (every place with at least 5,000 inhabitants plus
// administrative seats) with region, country, coordinates and IANA time zone,
// and English, French and Spanish names for larger places. Search folds
// accents and punctuation, ranks exact and prefix matches before word and
// fuzzy matches, and breaks ties by population.
//
// Place data © GeoNames (geonames.org), licensed under Creative Commons
// Attribution 4.0; see data/NOTICE.
package geo

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"container/list"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Jihed-khelifi/staryield/backend/internal/astro/geo/fold"
)

//go:generate go run ./gen -out data -legacy-out legacy_gen.go
//go:embed data/places.tsv.gz data/countries.tsv data/VERSION
var dataFS embed.FS

// Attribution must accompany any display of place data.
const Attribution = "Place data © GeoNames (geonames.org), CC BY 4.0"

// IDPrefix marks gazetteer IDs ("gn:2988507").
const IDPrefix = "gn:"

// Place is a gazetteer entry.
type Place struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Admin1      string  `json:"admin1,omitempty"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	TimeZone    string  `json:"time_zone"`
	Population  int     `json:"population"`
}

// Label returns "Name, Region, Country" without repeating equal parts.
func (p Place) Label() string {
	parts := []string{p.Name}
	if p.Admin1 != "" && p.Admin1 != p.Name {
		parts = append(parts, p.Admin1)
	}
	if p.Country != "" {
		parts = append(parts, p.Country)
	}
	return strings.Join(parts, ", ")
}

type entry struct {
	place    Place
	geoID    int
	keys     []string // folded name, ASCII name and alternates (deduplicated)
	admin1   string   // folded
	country  string   // folded
	cc       string   // lower-case ISO code
	nameLen  int
	altStart int // keys[altStart:] are alternates
}

// key kinds in the sorted search index.
const (
	kindPrimary = iota
	kindAlternate
	kindWord
)

type keyRef struct {
	key   string
	entry int32
	kind  uint8
}

type index struct {
	version string
	entries []entry
	byGeoID map[int]int32
	keys    []keyRef           // sorted by key
	buckets map[string][]int32 // first two runes of primary/alternate keys → entries
}

var (
	loadOnce sync.Once
	idx      *index
	loadErr  error
)

// Warm decodes and indexes the embedded data (about 100 ms); call it at
// start-up so the first search is fast. It is safe to call repeatedly.
func Warm() error {
	loadOnce.Do(func() { idx, loadErr = build() })
	return loadErr
}

func get() (*index, error) {
	if err := Warm(); err != nil {
		return nil, err
	}
	return idx, nil
}

// Version identifies the embedded dataset ("geonames-2026-11-04").
func Version() string {
	ix, err := get()
	if err != nil {
		return ""
	}
	return ix.version
}

func build() (*index, error) {
	version, err := dataFS.ReadFile("data/VERSION")
	if err != nil {
		return nil, err
	}
	countriesRaw, err := dataFS.ReadFile("data/countries.tsv")
	if err != nil {
		return nil, err
	}
	countries := map[string]string{}
	for _, line := range strings.Split(string(countriesRaw), "\n") {
		if code, name, ok := strings.Cut(line, "\t"); ok {
			countries[code] = name
		}
	}
	gz, err := dataFS.ReadFile("data/places.tsv.gz")
	if err != nil {
		return nil, err
	}
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return nil, err
	}
	ix := &index{version: strings.TrimSpace(string(version)), byGeoID: map[int]int32{}, buckets: map[string][]int32{}}
	sc := bufio.NewScanner(zr)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		f := strings.Split(sc.Text(), "\t")
		if len(f) != 10 {
			return nil, fmt.Errorf("geo: malformed row %q", sc.Text())
		}
		id, err := strconv.Atoi(f[0])
		if err != nil {
			return nil, err
		}
		lat, err1 := strconv.ParseFloat(f[5], 64)
		lon, err2 := strconv.ParseFloat(f[6], 64)
		pop, err3 := strconv.Atoi(f[7])
		if err1 != nil || err2 != nil || err3 != nil {
			return nil, fmt.Errorf("geo: malformed numbers in %q", sc.Text())
		}
		e := entry{
			geoID: id,
			place: Place{
				ID: IDPrefix + f[0], Name: f[1], CountryCode: f[3], Country: countries[f[3]], Admin1: f[4],
				Latitude: lat, Longitude: lon, Population: pop, TimeZone: f[8],
			},
		}
		e.admin1, e.country, e.cc = fold.String(f[4]), fold.String(e.place.Country), strings.ToLower(f[3])
		seen := map[string]bool{}
		add := func(s string) {
			if k := fold.String(s); k != "" && !seen[k] {
				seen[k] = true
				e.keys = append(e.keys, k)
			}
		}
		add(f[1])
		if f[2] != "" {
			add(f[2])
		}
		e.altStart = len(e.keys)
		if f[9] != "" {
			for _, a := range strings.Split(f[9], "|") {
				add(a)
			}
		}
		e.nameLen = len(e.keys[0])
		ix.byGeoID[id] = int32(len(ix.entries))
		ix.entries = append(ix.entries, e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	for i := range ix.entries {
		e := &ix.entries[i]
		for k, key := range e.keys {
			kind := uint8(kindPrimary)
			if k >= e.altStart {
				kind = kindAlternate
			}
			ix.keys = append(ix.keys, keyRef{key, int32(i), kind})
			b := bucketOf(key)
			if l := ix.buckets[b]; len(l) == 0 || l[len(l)-1] != int32(i) {
				ix.buckets[b] = append(ix.buckets[b], int32(i))
			}
			// Later words of multi-word names ("rio de janeiro" → "janeiro").
			words := strings.Fields(key)
			for _, w := range words[1:] {
				if len(w) >= 3 {
					ix.keys = append(ix.keys, keyRef{w, int32(i), kindWord})
				}
			}
		}
	}
	sort.Slice(ix.keys, func(a, b int) bool { return ix.keys[a].key < ix.keys[b].key })
	return ix, nil
}

func bucketOf(key string) string {
	r := []rune(key)
	if len(r) < 2 {
		return string(r)
	}
	return string(r[:2])
}

// legacyIDs (generated) maps the original city slugs to GeoNames IDs.

// ByID returns the place with a gazetteer ID ("gn:2988507") or a legacy slug
// of the original city list ("paris-fr").
func ByID(id string) (Place, bool) {
	ix, err := get()
	if err != nil {
		return Place{}, false
	}
	id = strings.ToLower(strings.TrimSpace(id))
	var gid int
	if rest, ok := strings.CutPrefix(id, IDPrefix); ok {
		n, err := strconv.Atoi(rest)
		if err != nil || n <= 0 {
			return Place{}, false
		}
		gid = n
	} else if n, ok := legacyIDs[id]; ok {
		gid = n
	} else {
		return Place{}, false
	}
	i, ok := ix.byGeoID[gid]
	if !ok {
		return Place{}, false
	}
	return ix.entries[i].place, true
}

// IsLegacyID reports whether id is a slug of the original city list.
func IsLegacyID(id string) bool {
	_, ok := legacyIDs[strings.ToLower(strings.TrimSpace(id))]
	return ok
}

// LegacySlugs lists the original city list's slugs, sorted.
func LegacySlugs() []string {
	out := make([]string, 0, len(legacyIDs))
	for slug := range legacyIDs {
		out = append(out, slug)
	}
	sort.Strings(out)
	return out
}

// Match ranks; lower is better.
const (
	rankExact = iota
	rankPrefix
	rankAlternatePrefix
	rankWordPrefix
	rankFuzzy
)

// SearchOptions controls Search.
type SearchOptions struct {
	Limit  int    // 1–20, default 8
	Cursor string // opaque cursor from a previous page
}

// Page is one page of results.
type Page struct {
	Items      []Place `json:"items"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

// Search errors.
var (
	ErrQueryTooShort = errors.New("geo: query too short")
	ErrBadCursor     = errors.New("geo: invalid cursor")
)

// MinQueryLength and MaxQueryLength bound queries (in runes, after
// trimming).
const (
	MinQueryLength = 2
	MaxQueryLength = 80
	maxResults     = 200 // ranked results kept per query (pages beyond are empty)
)

// Search finds places matching query, which may be "city" or
// "city, region|country|ISO code". Results are ranked exact match, name
// prefix, alternate-name prefix, word prefix, then fuzzy (typos), and by
// population within a rank.
func Search(query string, opts SearchOptions) (Page, error) {
	ix, err := get()
	if err != nil {
		return Page{}, err
	}
	query = strings.TrimSpace(query)
	if n := len([]rune(query)); n > MaxQueryLength {
		query = string([]rune(query)[:MaxQueryLength])
	}
	city, filter, _ := strings.Cut(query, ",")
	q, f := fold.String(city), fold.String(filter)
	if len([]rune(q)) < MinQueryLength {
		return Page{}, ErrQueryTooShort
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 8
	}
	limit = min(limit, 20)
	offset := 0
	if opts.Cursor != "" {
		offset, err = decodeCursor(opts.Cursor, ix.version)
		if err != nil {
			return Page{}, err
		}
	}
	ranked := cachedSearch(ix, q, f)
	page := Page{Items: []Place{}}
	for i := offset; i < len(ranked) && i < offset+limit; i++ {
		page.Items = append(page.Items, ix.entries[ranked[i]].place)
	}
	if offset+limit < len(ranked) {
		page.NextCursor = encodeCursor(ix.version, offset+limit)
	}
	return page, nil
}

func encodeCursor(version string, offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(version + "|" + strconv.Itoa(offset)))
}

func decodeCursor(c, version string) (int, error) {
	raw, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil || len(raw) > 64 {
		return 0, ErrBadCursor
	}
	v, off, ok := strings.Cut(string(raw), "|")
	n, err := strconv.Atoi(off)
	if !ok || v != version || err != nil || n < 0 || n > maxResults {
		return 0, ErrBadCursor
	}
	return n, nil
}

// --- ranking -----------------------------------------------------------------

type hit struct {
	entry int32
	rank  int
}

func (ix *index) passes(e *entry, filter string) bool {
	if filter == "" {
		return true
	}
	return strings.HasPrefix(e.admin1, filter) || strings.HasPrefix(e.country, filter) || e.cc == filter
}

func (ix *index) rank(q, filter string) []int32 {
	best := map[int32]int{}
	note := func(i int32, r int) {
		if old, ok := best[i]; !ok || r < old {
			best[i] = r
		}
	}
	start := sort.Search(len(ix.keys), func(i int) bool { return ix.keys[i].key >= q })
	for i := start; i < len(ix.keys) && strings.HasPrefix(ix.keys[i].key, q); i++ {
		k := ix.keys[i]
		e := &ix.entries[k.entry]
		if !ix.passes(e, filter) {
			continue
		}
		switch {
		case k.key == q && k.kind != kindWord:
			note(k.entry, rankExact)
		case k.kind == kindPrimary:
			note(k.entry, rankPrefix)
		case k.kind == kindAlternate:
			note(k.entry, rankAlternatePrefix)
		default:
			note(k.entry, rankWordPrefix)
		}
	}
	if len(best) < maxResults {
		ix.fuzzy(q, filter, best, note)
	}
	hits := make([]hit, 0, len(best))
	for i, r := range best {
		hits = append(hits, hit{i, r})
	}
	sort.Slice(hits, func(a, b int) bool {
		if hits[a].rank != hits[b].rank {
			return hits[a].rank < hits[b].rank
		}
		ea, eb := &ix.entries[hits[a].entry], &ix.entries[hits[b].entry]
		if ea.place.Population != eb.place.Population {
			return ea.place.Population > eb.place.Population
		}
		return ea.geoID < eb.geoID
	})
	if len(hits) > maxResults {
		hits = hits[:maxResults]
	}
	out := make([]int32, len(hits))
	for i, h := range hits {
		out[i] = h.entry
	}
	return out
}

// fuzzy adds entries whose name (or alternate) is within a small edit
// distance of q: 1 for 4–7 runes, 2 for 8 or more. Candidates share the
// first two letters and are within two runes in length.
func (ix *index) fuzzy(q, filter string, seen map[int32]int, note func(int32, int)) {
	qr := []rune(q)
	maxDist := 0
	switch {
	case len(qr) >= 8:
		maxDist = 2
	case len(qr) >= 4:
		maxDist = 1
	default:
		return
	}
	for _, i := range ix.buckets[bucketOf(q)] {
		if _, ok := seen[i]; ok {
			continue
		}
		e := &ix.entries[i]
		if !ix.passes(e, filter) {
			continue
		}
		for _, k := range e.keys {
			kr := []rune(k)
			if abs(len(kr)-len(qr)) > 2 {
				continue
			}
			if osaDistance(qr, kr, maxDist) <= maxDist {
				note(i, rankFuzzy)
				break
			}
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// osaDistance is the optimal string alignment (restricted Damerau–
// Levenshtein) distance between a and b, giving up early once it exceeds
// limit.
func osaDistance(a, b []rune, limit int) int {
	if abs(len(a)-len(b)) > limit {
		return limit + 1
	}
	prev2 := make([]int, len(b)+1)
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		rowMin := cur[0]
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			v := min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				v = min(v, prev2[j-2]+1)
			}
			cur[j] = v
			rowMin = min(rowMin, v)
		}
		if rowMin > limit {
			return limit + 1
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[len(b)]
}

// --- query cache -------------------------------------------------------------

const cacheSize = 10000

type cacheEntry struct {
	key   string
	value []int32
}

var cache = struct {
	sync.Mutex
	ll    *list.List
	items map[string]*list.Element
}{ll: list.New(), items: map[string]*list.Element{}}

func cachedSearch(ix *index, q, filter string) []int32 {
	key := q + "\x00" + filter
	cache.Lock()
	if el, ok := cache.items[key]; ok {
		cache.ll.MoveToFront(el)
		v := el.Value.(*cacheEntry).value
		cache.Unlock()
		return v
	}
	cache.Unlock()
	v := ix.rank(q, filter)
	cache.Lock()
	defer cache.Unlock()
	if el, ok := cache.items[key]; ok {
		cache.ll.MoveToFront(el)
		return el.Value.(*cacheEntry).value
	}
	cache.items[key] = cache.ll.PushFront(&cacheEntry{key, v})
	if cache.ll.Len() > cacheSize {
		oldest := cache.ll.Back()
		cache.ll.Remove(oldest)
		delete(cache.items, oldest.Value.(*cacheEntry).key)
	}
	return v
}
