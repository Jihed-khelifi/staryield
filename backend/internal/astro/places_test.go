package astro

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestGazetteerIntegrity(t *testing.T) {
	if len(gazetteer) < 120 {
		t.Errorf("gazetteer has %d entries, want at least 120", len(gazetteer))
	}
	idPattern := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	codePattern := regexp.MustCompile(`^[A-Z]{2}$`)
	seen := map[string]bool{}
	continents := map[string]bool{}
	for _, p := range gazetteer {
		if seen[p.ID] {
			t.Errorf("duplicate ID %q", p.ID)
		}
		seen[p.ID] = true
		if !idPattern.MatchString(p.ID) {
			t.Errorf("%q is not a lowercase slug", p.ID)
		}
		if !strings.HasSuffix(p.ID, "-"+strings.ToLower(p.CountryCode)) {
			t.Errorf("%q does not end with its country code %s", p.ID, p.CountryCode)
		}
		if !codePattern.MatchString(p.CountryCode) {
			t.Errorf("%s: bad country code %q", p.ID, p.CountryCode)
		}
		if p.Name == "" || p.Country == "" {
			t.Errorf("%s: empty name or country", p.ID)
		}
		if p.Latitude < -90 || p.Latitude > 90 || p.Longitude < -180 || p.Longitude > 180 {
			t.Errorf("%s: coordinates out of range (%v, %v)", p.ID, p.Latitude, p.Longitude)
		}
		loc, err := time.LoadLocation(p.TimeZone)
		if err != nil {
			t.Errorf("%s: time zone %q does not load: %v", p.ID, p.TimeZone, err)
			continue
		}
		// A city's standard UTC offset should roughly match its longitude
		// (15° per hour), within a generous margin for political zones.
		_, offset := time.Date(2025, 1, 15, 12, 0, 0, 0, loc).Zone()
		if diff := float64(offset)/3600 - p.Longitude/15; diff < -3.5 || diff > 3.5 {
			t.Errorf("%s: UTC offset %+.1fh is far from longitude %.2f°", p.ID, float64(offset)/3600, p.Longitude)
		}
		continents[strings.SplitN(p.TimeZone, "/", 2)[0]] = true
	}
	for _, region := range []string{"America", "Europe", "Africa", "Asia", "Australia", "Pacific"} {
		if !continents[region] {
			t.Errorf("no city in the %s time zone region", region)
		}
	}
	for id := range placeAliases {
		if !seen[id] {
			t.Errorf("alias key %q is not a gazetteer ID", id)
		}
	}
}

func TestPlaceByID(t *testing.T) {
	p, ok := PlaceByID("paris-fr")
	if !ok || p.Name != "Paris" || p.CountryCode != "FR" || p.TimeZone != "Europe/Paris" {
		t.Errorf("PlaceByID(paris-fr) = %+v, %v", p, ok)
	}
	if _, ok := PlaceByID(" Tunis-TN "); !ok {
		t.Error("PlaceByID should ignore case and surrounding space")
	}
	if _, ok := PlaceByID("atlantis-xx"); ok {
		t.Error("PlaceByID found a nonexistent place")
	}
	if got := Places(); len(got) != len(gazetteer) {
		t.Errorf("Places() returned %d entries", len(got))
	}
}

func ids(places []Place) []string {
	out := make([]string, len(places))
	for i, p := range places {
		out[i] = p.ID
	}
	return out
}

func TestSearchPlaces(t *testing.T) {
	tests := []struct {
		query     string
		limit     int
		firstID   string // expected first result ("" = expect no results)
		wantCount int    // exact count when > 0
	}{
		{"par", 5, "paris-fr", 0},
		{"Paris", 5, "paris-fr", 1},
		{"sao", 5, "sao-paulo-br", 0},
		{"SÃO PAULO", 5, "sao-paulo-br", 1},
		{"sao-paulo", 5, "sao-paulo-br", 1},
		{"zurich", 5, "zurich-ch", 1},
		{"Zürich", 5, "zurich-ch", 1},
		{"montreal", 5, "montreal-ca", 1},
		{"bogota", 5, "bogota-co", 1},
		{"krakow", 5, "krakow-pl", 1},
		{"york", 5, "new-york-us", 1},
		{"bombay", 5, "mumbai-in", 1},
		{"kiev", 5, "kyiv-ua", 1},
		{"münchen", 5, "munich-de", 1},
		{"cote d'ivoire", 5, "abidjan-ci", 1},
		{"paris, fr", 5, "paris-fr", 1},
		{"Paris, France", 5, "paris-fr", 1},
		{"san, costa", 5, "san-jose-cr", 1},
		{"washington dc", 5, "washington-dc-us", 1},
		{"tunisia", 10, "sfax-tn", 3},
		{"qqqq", 5, "", 0},
		{"", 5, "", 0},
		{"   ", 5, "", 0},
		{",", 5, "", 0},
	}
	for _, tt := range tests {
		got := SearchPlaces(tt.query, tt.limit)
		if tt.firstID == "" {
			if len(got) != 0 {
				t.Errorf("SearchPlaces(%q) = %v, want none", tt.query, ids(got))
			}
			continue
		}
		if len(got) == 0 || got[0].ID != tt.firstID {
			t.Errorf("SearchPlaces(%q) = %v, want first %s", tt.query, ids(got), tt.firstID)
			continue
		}
		if tt.wantCount > 0 && len(got) != tt.wantCount {
			t.Errorf("SearchPlaces(%q) returned %d results %v, want %d", tt.query, len(got), ids(got), tt.wantCount)
		}
	}
	if got := SearchPlaces("", 5); got != nil {
		t.Errorf("empty query should return nil, got %v", got)
	}
}

func TestSearchPlacesRanking(t *testing.T) {
	// "san" is a prefix of several cities and a substring of Busan; prefix
	// matches must come first.
	got := SearchPlaces("san", 20)
	busan := -1
	lastPrefix := -1
	for i, p := range got {
		if strings.HasPrefix(foldText(p.Name), "san") {
			lastPrefix = i
		}
		if p.ID == "busan-kr" {
			busan = i
		}
	}
	if busan < 0 || lastPrefix < 0 || busan < lastPrefix {
		t.Errorf("ranking wrong for %q: %v", "san", ids(got))
	}

	// A country-name match ranks after city-name prefixes but before
	// substrings.
	got = SearchPlaces("japan", 20)
	if len(got) != 3 {
		t.Errorf("japan: %v", ids(got))
	}
	for _, p := range got {
		if p.CountryCode != "JP" {
			t.Errorf("japan returned %s", p.ID)
		}
	}
}

func TestSearchPlacesLimit(t *testing.T) {
	if n := len(SearchPlaces("a", 100)); n != 20 {
		t.Errorf("limit 100 returned %d, want clamp to 20", n)
	}
	if n := len(SearchPlaces("a", 0)); n != 1 {
		t.Errorf("limit 0 returned %d, want clamp to 1", n)
	}
	if n := len(SearchPlaces("a", -5)); n != 1 {
		t.Errorf("limit -5 returned %d, want clamp to 1", n)
	}
	if n := len(SearchPlaces("a", 7)); n != 7 {
		t.Errorf("limit 7 returned %d", n)
	}
}

func TestFoldText(t *testing.T) {
	tests := map[string]string{
		"São Paulo":          "sao paulo",
		"  Zürich ":          "zurich",
		"Côte d'Ivoire":      "cote divoire",
		"Washington, D.C.":   "washington dc",
		"KRAKÓW":             "krakow",
		"Ho-Chi-Minh   City": "ho chi minh city",
		"Reykjavík":          "reykjavik",
		"Bogotá":            "bogota", // decomposed accent
	}
	for in, want := range tests {
		if got := foldText(in); got != want {
			t.Errorf("foldText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLocalBirthInstant(t *testing.T) {
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
	tests := []struct {
		name  string
		date  time.Time
		clock string
		tz    string
		want  time.Time
	}{
		{"Honolulu 1961 (HST, UTC-10)", day(1961, 8, 4), "19:24", "Pacific/Honolulu", time.Date(1961, 8, 5, 5, 24, 0, 0, time.UTC)},
		{"London summer time", day(1961, 7, 1), "19:45", "Europe/London", time.Date(1961, 7, 1, 18, 45, 0, 0, time.UTC)},
		{"Paris winter", day(2000, 1, 1), "00:30", "Europe/Paris", time.Date(1999, 12, 31, 23, 30, 0, 0, time.UTC)},
		{"empty clock means noon", day(2000, 1, 1), "", "Europe/Paris", time.Date(2000, 1, 1, 11, 0, 0, 0, time.UTC)},
		{"single-digit hour", day(2020, 6, 1), "7:05", "Asia/Tokyo", time.Date(2020, 5, 31, 22, 5, 0, 0, time.UTC)},
		{"Kathmandu quarter-hour offset", day(2020, 6, 1), "12:00", "Asia/Kathmandu", time.Date(2020, 6, 1, 6, 15, 0, 0, time.UTC)},
		{"empty zone means UTC", day(2020, 6, 1), "23:59", "", time.Date(2020, 6, 1, 23, 59, 0, 0, time.UTC)},
		{
			// Tunisia observed summer time (UTC+2) in 1990; the clock and
			// zone of the date argument itself are ignored.
			"Tunis 1990 summer time, date's own zone ignored",
			time.Date(1990, 5, 17, 23, 0, 0, 0, time.FixedZone("UTC-5", -5*3600)),
			"08:30", "Africa/Tunis", time.Date(1990, 5, 17, 6, 30, 0, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LocalBirthInstant(tt.date, tt.clock, tt.tz)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(tt.want) || got.Location() != time.UTC {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestLocalBirthInstantErrors(t *testing.T) {
	date := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, clock := range []string{"24:00", "12:60", "12", "ab:cd", "1:2", "12:345", "-1:00", "+1:00", "12:5a", ":30", "12:30:00"} {
		if _, err := LocalBirthInstant(date, clock, "UTC"); !errors.Is(err, ErrInvalidClock) {
			t.Errorf("clock %q: err = %v, want ErrInvalidClock", clock, err)
		}
	}
	if _, err := LocalBirthInstant(date, "12:00", "Mars/Olympus_Mons"); err == nil {
		t.Error("unknown time zone accepted")
	}
}
