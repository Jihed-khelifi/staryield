package geo

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/astro/geo/fold"
	"github.com/Jihed-khelifi/staryield/backend/internal/astro/tz"
)

func TestWarmAndVersion(t *testing.T) {
	start := time.Now()
	if err := Warm(); err != nil {
		t.Fatal(err)
	}
	t.Logf("indexed in %s", time.Since(start))
	if v := Version(); !strings.HasPrefix(v, "geonames-") {
		t.Errorf("Version() = %q", v)
	}
	if n := len(idx.entries); n < 50000 {
		t.Errorf("only %d places", n)
	}
}

func TestByID(t *testing.T) {
	p, ok := ByID("gn:2988507")
	if !ok || p.Name != "Paris" || p.CountryCode != "FR" || p.Country != "France" || p.Admin1 != "Île-de-France" || p.TimeZone != "Europe/Paris" {
		t.Fatalf("Paris = %+v, %v", p, ok)
	}
	if p.Label() != "Paris, Île-de-France, France" {
		t.Errorf("Label() = %q", p.Label())
	}
	if legacy, ok := ByID("paris-fr"); !ok || legacy.ID != "gn:2988507" {
		t.Errorf("legacy paris-fr = %+v, %v", legacy, ok)
	}
	if !IsLegacyID("PARIS-FR") || IsLegacyID("gn:2988507") {
		t.Error("IsLegacyID")
	}
	for _, bad := range []string{"", "gn:", "gn:0", "gn:-5", "gn:abc", "gn:999999999", "atlantis-xx", "gn:2988507x"} {
		if _, ok := ByID(bad); ok {
			t.Errorf("ByID(%q) succeeded", bad)
		}
	}
}

// TestLegacyMapping checks every slug of the original city list maps to a
// GeoNames place of the same country close to the original coordinates and
// in a zone that keeps today's offset.
func TestLegacyMapping(t *testing.T) {
	raw, err := os.ReadFile("gen/legacy_places.json")
	if err != nil {
		t.Fatal(err)
	}
	var legacy []struct {
		ID          string  `json:"id"`
		CountryCode string  `json:"country_code"`
		Latitude    float64 `json:"latitude"`
		Longitude   float64 `json:"longitude"`
		TimeZone    string  `json:"time_zone"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		t.Fatal(err)
	}
	if len(legacy) != 173 {
		t.Fatalf("%d legacy places", len(legacy))
	}
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	for _, l := range legacy {
		p, ok := ByID(l.ID)
		if !ok {
			t.Errorf("%s does not resolve", l.ID)
			continue
		}
		if p.CountryCode != l.CountryCode {
			t.Errorf("%s → %s in %s, want %s", l.ID, p.Name, p.CountryCode, l.CountryCode)
		}
		if d := distanceKm(l.Latitude, l.Longitude, p.Latitude, p.Longitude); d > 60 {
			t.Errorf("%s → %s is %.0f km away", l.ID, p.Name, d)
		}
		a, errA := tz.Load(l.TimeZone)
		b, errB := tz.Load(p.TimeZone)
		if errA != nil || errB != nil {
			t.Errorf("%s: zones %s/%s: %v %v", l.ID, l.TimeZone, p.TimeZone, errA, errB)
			continue
		}
		if oa, _ := tz.OffsetAt(now, a); func() bool { ob, _ := tz.OffsetAt(now, b); return oa != ob }() {
			t.Errorf("%s: zone %s differs from %s today", l.ID, p.TimeZone, l.TimeZone)
		}
	}
}

func distanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	rad := math.Pi / 180
	dlat, dlon := (lat2-lat1)*rad, (lon2-lon1)*rad
	a := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dlon/2)*math.Sin(dlon/2)
	return 2 * 6371 * math.Asin(math.Min(1, math.Sqrt(a)))
}

func first(t *testing.T, q string) Place {
	t.Helper()
	page, err := Search(q, SearchOptions{Limit: 5})
	if err != nil {
		t.Fatalf("Search(%q): %v", q, err)
	}
	if len(page.Items) == 0 {
		t.Fatalf("Search(%q) found nothing", q)
	}
	return page.Items[0]
}

func TestSearchRanking(t *testing.T) {
	tests := []struct {
		query, wantID, why string
	}{
		{"paris", "gn:2988507", "the most populous exact match"},
		{"PARIS", "gn:2988507", "case-insensitive"},
		{"Paris, US", "gn:4717560", "country filter (Paris, Texas)"},
		{"paris, tennessee", "gn:4647963", "region filter"},
		{"são paulo", "gn:3448439", "accents"},
		{"sao-paulo", "gn:3448439", "punctuation"},
		{"new york", "gn:5128581", "prefix of New York City"},
		{"nueva york", "gn:5128581", "Spanish name"},
		{"londres", "gn:2643743", "French/Spanish name for London"},
		{"moscou", "gn:524901", "French name for Moscow"},
		{"pékin", "gn:1816670", "French name for Beijing"},
		{"janeiro", "gn:3451190", "later word of Rio de Janeiro"},
		{"lodnon", "gn:2643743", "fuzzy: transposed letters"},
		{"barcleona", "gn:3128760", "fuzzy: two edits on a long name"},
		{"memphis, tennessee", "gn:4641239", "client card example city"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			if got := first(t, tt.query); got.ID != tt.wantID {
				t.Errorf("%s: first result %s (%s, %s), want %s", tt.why, got.ID, got.Name, got.Country, tt.wantID)
			}
		})
	}
}

func TestSearchPagination(t *testing.T) {
	page1, err := Search("san", SearchOptions{Limit: 10})
	if err != nil || len(page1.Items) != 10 || page1.NextCursor == "" {
		t.Fatalf("page 1: %d items, cursor %q, %v", len(page1.Items), page1.NextCursor, err)
	}
	page2, err := Search("san", SearchOptions{Limit: 10, Cursor: page1.NextCursor})
	if err != nil || len(page2.Items) != 10 {
		t.Fatalf("page 2: %v", err)
	}
	seen := map[string]bool{}
	for _, p := range append(page1.Items, page2.Items...) {
		if seen[p.ID] {
			t.Errorf("duplicate %s across pages", p.ID)
		}
		seen[p.ID] = true
	}
	// Exact matches come first; within the name-prefix rank, population
	// decides.
	prefixOnly := func(p Place) bool { n := fold.String(p.Name); return n != "san" && strings.HasPrefix(n, "san") }
	for i := 1; i < len(page1.Items); i++ {
		a, b := page1.Items[i-1], page1.Items[i]
		if prefixOnly(a) && prefixOnly(b) && a.Population < b.Population {
			t.Errorf("population order broken: %s (%d) before %s (%d)", a.Name, a.Population, b.Name, b.Population)
		}
	}
	// A cursor from another dataset version or a forged one is rejected.
	for _, c := range []string{"!!", encodeCursor("geonames-1999-01-01", 10), encodeCursor(Version(), -1), encodeCursor(Version(), 9999)} {
		if _, err := Search("san", SearchOptions{Cursor: c}); !errors.Is(err, ErrBadCursor) {
			t.Errorf("cursor %q: err %v, want ErrBadCursor", c, err)
		}
	}
}

func TestSearchLimitsAndShortQueries(t *testing.T) {
	for _, q := range []string{"", " ", "a", "é", ",", ", fr", "..."} {
		if _, err := Search(q, SearchOptions{}); !errors.Is(err, ErrQueryTooShort) {
			t.Errorf("Search(%q): err %v, want ErrQueryTooShort", q, err)
		}
	}
	page, err := Search("la", SearchOptions{Limit: 500})
	if err != nil || len(page.Items) != 20 {
		t.Errorf("limit clamp: %d items, %v", len(page.Items), err)
	}
	if page, err := Search(strings.Repeat("x", 500), SearchOptions{}); err != nil || len(page.Items) != 0 {
		t.Errorf("long query: %d items, %v", len(page.Items), err)
	}
	if page, err := Search("zzqqxx", SearchOptions{}); err != nil || len(page.Items) != 0 || page.Items == nil {
		t.Errorf("no match must be an empty, non-nil list: %+v %v", page, err)
	}
}

func TestEveryPlaceHasAValidZone(t *testing.T) {
	if err := Warm(); err != nil {
		t.Fatal(err)
	}
	bad := map[string]int{}
	for _, e := range idx.entries {
		if !tz.Valid(e.place.TimeZone) {
			bad[e.place.TimeZone]++
		}
		if e.place.Country == "" {
			t.Fatalf("%s has no country", e.place.ID)
		}
	}
	if len(bad) > 0 {
		t.Errorf("zones missing from the tz build: %v", bad)
	}
}

func TestOSADistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"london", "london", 0}, {"lodnon", "london", 1}, {"londn", "london", 1}, {"barcleona", "barcelona", 1},
		{"barclona", "barcelona", 1}, {"paris", "pairs", 1}, {"abc", "xyz", 3},
	}
	for _, tt := range tests {
		if got := osaDistance([]rune(tt.a), []rune(tt.b), 5); got != tt.want {
			t.Errorf("osa(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
	if got := osaDistance([]rune("abcdef"), []rune("uvwxyz"), 1); got != 2 {
		t.Errorf("early exit returns limit+1, got %d", got)
	}
}

func FuzzSearch(f *testing.F) {
	for _, s := range []string{"paris", "são paulo, br", "new y", "ñ", ",,,", "a, b, c", "\x00\xff", "日本"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, q string) {
		page, err := Search(q, SearchOptions{Limit: 7})
		if err != nil {
			if !errors.Is(err, ErrQueryTooShort) {
				t.Fatalf("Search(%q): %v", q, err)
			}
			return
		}
		if len(page.Items) > 7 {
			t.Fatalf("Search(%q) returned %d items", q, len(page.Items))
		}
	})
}

func BenchmarkSearchCold(b *testing.B) {
	if err := Warm(); err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		idx.rank("sant", "")
	}
}

func BenchmarkSearchCached(b *testing.B) {
	for b.Loop() {
		_, _ = Search("paris", SearchOptions{})
	}
}
