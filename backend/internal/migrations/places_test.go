package migrations

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Jihed-khelifi/staryield/backend/internal/astro/geo"
)

// The places migration moves every slug of the original city list to the
// GeoNames place the gazetteer resolves it to at runtime, so a migrated
// profile and an unmigrated one compute the same chart.
func TestPlacesMigrationMatchesGazetteer(t *testing.T) {
	all, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	var sql string
	for _, m := range all {
		if m.Version == "0006_places_geonames" {
			sql = m.SQL
		}
	}
	if sql == "" {
		t.Fatal("0006_places_geonames not embedded")
	}
	row := regexp.MustCompile(`\('([a-z0-9-]+)', '(gn:\d+)', '((?:[^']|'')*)', '((?:[^']|'')*)', (-?[\d.]+), (-?[\d.]+), '([A-Za-z_/+-]+)'\)`)
	rows := map[string][]string{}
	for _, m := range row.FindAllStringSubmatch(sql, -1) {
		rows[m[1]] = m[2:]
	}
	slugs := geo.LegacySlugs()
	if len(slugs) != 173 || len(rows) != len(slugs) {
		t.Fatalf("%d legacy slugs, %d migration rows", len(slugs), len(rows))
	}
	for _, slug := range slugs {
		p, ok := geo.ByID(slug)
		r := rows[slug]
		if !ok || r == nil {
			t.Errorf("%s: gazetteer %v, migration row %v", slug, ok, r)
			continue
		}
		if r[0] != p.ID || r[5] != p.TimeZone || strings.ReplaceAll(r[1], "''", "'") != p.Label() {
			t.Errorf("%s: migration %v, gazetteer %s %q %s", slug, r, p.ID, p.Label(), p.TimeZone)
		}
	}
}
