package e2e

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/seed"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// A year of calculator traffic at production-like volume: about 1M
// calculator_visits rows for 200k visitors (5% converted), generated with
// generate_series. The reports must stay correct and fast, and the
// conversions must page through completely.
func TestCalculatorLargeDataset(t *testing.T) {
	if testing.Short() {
		t.Skip("large dataset")
	}
	e := newEnv(t)
	db := e.app.DB
	now := e.clock.Now()
	const (
		tag       = "E2ELarge"
		visitors  = 200_000
		userCount = 12_000
	)
	cleanup := func() {
		db.Exec(`DELETE FROM calculator_visits WHERE anonymous_id LIKE 'lv` + tag + `-%'`)
		db.Exec(`DELETE FROM web_visitors WHERE anonymous_id LIKE 'lv` + tag + `-%'`)
		db.Exec(`DELETE FROM short_links WHERE code LIKE 'cl` + tag + `%'`)
		db.Exec(`DELETE FROM campaigns WHERE slug = 'calc-load-` + "e2elarge" + `'`)
		db.Exec(`DELETE FROM users WHERE email LIKE '%@calclarge.test'`)
	}
	cleanup()
	t.Cleanup(cleanup)

	start := time.Now()
	if err := db.Exec(`INSERT INTO users (id, email, password_hash, role, status, display_name, locale, terms_accepted_at, created_at, updated_at)
		SELECT gen_random_uuid(), 'calclarge-' || i || '@calclarge.test', 'x', 'client', 'active', 'Large ' || i,
		       (ARRAY['en','fr','es'])[1 + (i % 3)], ?::timestamptz, ts, ts
		FROM (SELECT i, ?::timestamptz - (random() * 330 * interval '1 day') AS ts FROM generate_series(1, ?::int) i) s`, now, now, userCount).Error; err != nil {
		t.Fatal(err)
	}
	res, err := seed.SeedCalculatorVisitors(t.Context(), db, seed.CalculatorSeedOptions{Visitors: visitors, Tag: tag, Now: now, Days: 360,
		ConvertedShare: 0.05, UserEmailLike: "%@calclarge.test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("seeded %d visitors, %d visit rows, %d converted, %d links in %s", res.Visitors, res.Visits, res.Converted, res.Links, time.Since(start).Round(time.Millisecond))
	if res.Visitors != visitors || res.Visits < 800_000 || res.Converted != visitors/20 {
		t.Fatalf("seed sizes: %+v", res)
	}
	// Seeding again with the same tag adds nothing (idempotent).
	again, err := seed.SeedCalculatorVisitors(t.Context(), db, seed.CalculatorSeedOptions{Visitors: visitors, Tag: tag, Now: now, Days: 360,
		ConvertedShare: 0.05, UserEmailLike: "%@calclarge.test"})
	if err != nil || again.Visitors != 0 || again.Visits != 0 {
		t.Fatalf("second run: %+v %v", again, err)
	}

	admin := e.createAdmin()
	from, to := now.AddDate(0, 0, -365).Format("2006-01-02"), now.Format("2006-01-02")
	q := "from=" + from + "&to=" + to
	const budget = 20 * time.Second // generous: CI machines vary; typical is well under a second

	t0 := time.Now()
	r := calcReport(t, admin, q)
	took := time.Since(t0)
	t.Logf("12-month report: %s", took.Round(time.Millisecond))
	if took > budget {
		t.Fatalf("report took %s", took)
	}

	// Expected values from plain SQL over the same window.
	fromDay := now.AddDate(0, 0, -365).Truncate(24 * time.Hour)
	toDay := now.Truncate(24*time.Hour).AddDate(0, 0, 1)
	var want struct{ Visitors, Tried, Completed int64 }
	db.Raw(`SELECT count(DISTINCT anonymous_id) AS visitors, count(DISTINCT anonymous_id) FILTER (WHERE started > 0) AS tried,
		count(DISTINCT anonymous_id) FILTER (WHERE completed > 0) AS completed
		FROM calculator_visits WHERE anonymous_id LIKE 'lv`+tag+`-%' AND day >= ? AND day < ?`, fromDay, toDay).Scan(&want)
	var linked int64
	db.Raw(`SELECT count(*) FROM web_visitors w WHERE w.user_id IS NOT NULL AND w.anonymous_id LIKE 'lv`+tag+`-%' AND EXISTS (
		SELECT 1 FROM calculator_visits cv WHERE cv.anonymous_id = w.anonymous_id AND cv.day >= ? AND cv.day < ?)`, fromDay, toDay).Scan(&linked)
	tot := r.Totals
	if tot.Visitors != want.Visitors || tot.Tried != want.Tried || tot.Completed != want.Completed {
		t.Fatalf("totals %+v, plain SQL says %+v", tot, want)
	}
	if want.Visitors < 190_000 || tot.Tried == 0 || tot.Completed == 0 || tot.Completed > tot.Tried || tot.TryRate <= 0 || tot.TryRate >= 1 {
		t.Fatalf("implausible totals: %+v", tot)
	}
	if tot.Signups+tot.ExistingUsers != linked {
		t.Fatalf("linked visitors %d != signups %d + existing %d", linked, tot.Signups, tot.ExistingUsers)
	}
	if tot.Signups < 1000 || tot.Signups > visitors/20 {
		t.Fatalf("signups: %d", tot.Signups)
	}
	if tot.SignedInVisitors == 0 || tot.SignedInVisitors > tot.Visitors {
		t.Fatalf("signed-in visitors: %d", tot.SignedInVisitors)
	}
	// Breakdowns agree with the totals.
	var sumKind, sumLocale, sumSource, sumSeries, signupSeries int64
	for _, k := range r.ByKind {
		sumKind += k.Visitors
		if k.Visitors == 0 || k.Visitors > tot.Visitors {
			t.Fatalf("kind %s: %+v", k.Kind, k.CalculatorTotals)
		}
	}
	for _, l := range r.ByLocale {
		sumLocale += l.Visitors
	}
	for _, s := range r.BySource {
		sumSource += s.Visitors
	}
	for _, p := range r.Series {
		sumSeries += p.Visitors
		signupSeries += p.Signups
	}
	if sumKind < tot.Visitors || sumLocale < tot.Visitors || sumSeries < tot.Visitors {
		t.Fatalf("breakdowns lose visitors: kinds %d locales %d series %d of %d", sumKind, sumLocale, sumSeries, tot.Visitors)
	}
	if sumSource != tot.Visitors { // every visitor has exactly one source row (under the top 50)
		t.Fatalf("by_source adds up to %d of %d", sumSource, tot.Visitors)
	}
	if signupSeries != tot.Signups {
		t.Fatalf("series signups %d != %d", signupSeries, tot.Signups)
	}
	if len(r.Series) < 365 || len(r.Series) > 367 || len(r.ByLink) == 0 || len(r.ByLink) > 50 || len(r.BySource) < 5 {
		t.Fatalf("shape: %d points, %d links, %d sources", len(r.Series), len(r.ByLink), len(r.BySource))
	}
	var directRow *service.CalculatorSource
	if directRow = sourceRow(r, "(direct)"); directRow == nil || directRow.Visitors < tot.Visitors/4 {
		t.Fatalf("direct share: %+v", directRow)
	}

	// Other views of the same year.
	for _, extra := range []string{"&interval=week", "&interval=month", "&kind=natal-chart&locale=fr", "&source=instagram", "&source=%28direct%29&kind=astrology"} {
		t0 := time.Now()
		rr := calcReport(t, admin, q+extra)
		if time.Since(t0) > budget || rr.Totals.Visitors == 0 || rr.Totals.Visitors > tot.Visitors {
			t.Fatalf("%s: %s, %+v", extra, time.Since(t0), rr.Totals)
		}
	}
	// A cached read is quick.
	t0 = time.Now()
	calcReport(t, admin, q)
	if d := time.Since(t0); d > 500*time.Millisecond {
		t.Fatalf("cached read took %s", d)
	}

	// Conversions: page through every sign-up.
	var all []service.CalculatorConversion
	cursor, pages := "", 0
	for {
		t0 := time.Now()
		page := calcConversions(t, admin, q+"&limit=100&cursor="+cursor)
		if d := time.Since(t0); d > 5*time.Second {
			t.Fatalf("conversions page %d took %s", pages, d)
		}
		all = append(all, page.Items...)
		pages++
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
		if pages > 200 {
			t.Fatal("pagination does not end")
		}
	}
	if int64(len(all)) != tot.Signups {
		t.Fatalf("conversions listed %d, report counts %d sign-ups", len(all), tot.Signups)
	}
	seen := map[string]bool{}
	for i, c := range all {
		if seen[c.User.ID.String()] {
			t.Fatalf("user %s listed twice", c.User.ID)
		}
		seen[c.User.ID.String()] = true
		if i > 0 && c.SignedUpAt.After(all[i-1].SignedUpAt) {
			t.Fatalf("not newest first at %d", i)
		}
		if c.FirstVisitAt.After(c.SignedUpAt) || len(c.Kinds) == 0 || len(c.Locales) == 0 {
			t.Fatalf("conversion %d: %+v", i, c)
		}
	}
	t.Logf("%d conversions in %d pages", len(all), pages)

	// Heavy concurrent reads (different queries, so the cache cannot help).
	kinds := []string{"", "natal-chart", "path-of-life", "astrology"}
	locales := []string{"", "en", "fr", "es"}
	var wg sync.WaitGroup
	errs := make(chan string, 64)
	t0 = time.Now()
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cc := *admin // own CSRF field, shared session cookies
			c := &cc
			qq := fmt.Sprintf("%s&kind=%s&locale=%s&interval=week", q, kinds[i%4], locales[(i/4)%4])
			r := c.call(http.MethodGet, "/api/v1/admin/analytics/calculators?"+qq, nil)
			if r.status != http.StatusOK {
				errs <- fmt.Sprintf("%s: %d %s", qq, r.status, r.body)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Error(msg)
	}
	t.Logf("16 concurrent uncached reports: %s", time.Since(t0).Round(time.Millisecond))
	if time.Since(t0) > 2*budget {
		t.Fatalf("concurrent reports took %s", time.Since(t0))
	}

	// New events keep flowing into the big tables.
	c := e.client()
	e.calcView(c, "large-live-visitor", "en", "astrology")
	e.calcTry(c, "large-live-visitor", "en", "astrology")
	defer db.Exec(`DELETE FROM calculator_visits WHERE anonymous_id = 'large-live-visitor'`)
	defer db.Exec(`DELETE FROM web_visitors WHERE anonymous_id = 'large-live-visitor'`)
	if r2 := calcReport(t, admin, q+"&kind=astrology&locale=en&interval=day"); r2.Totals.Visitors == 0 {
		t.Fatal("live visitor not counted")
	}
}
