package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/astro/tz"
	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/migrations"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// fieldErrors decodes the validation fields of an error response.
func fieldErrors(t *testing.T, r response) map[string]string {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Details struct {
				Fields map[string]string `json:"fields"`
			} `json:"details"`
		} `json:"error"`
	}
	r.decode(t, &body)
	if r.status != http.StatusUnprocessableEntity || body.Error.Code != "validation_failed" {
		t.Fatalf("want a validation error, got %d %s", r.status, r.body)
	}
	return body.Error.Details.Fields
}

func dateIn(t *testing.T, now time.Time, zone string, days int) string {
	t.Helper()
	loc, err := tz.Load(zone)
	if err != nil {
		t.Fatal(err)
	}
	l := now.In(loc)
	return time.Date(l.Year(), l.Month(), l.Day()+days, 0, 0, 0, 0, loc).Format(time.DateOnly)
}

func TestGazetteerSearchPagingAndCaching(t *testing.T) {
	e := newEnv(t)
	c := e.client()
	var page service.PlacesPage
	r := c.must(http.StatusOK, http.MethodGet, "/api/v1/astrology/places?q=paris&limit=2", nil, &page)
	if len(page.Items) != 2 || page.Items[0].ID != "gn:2988507" || page.Items[0].TimeZone != "Europe/Paris" || page.NextCursor == "" ||
		!strings.Contains(page.Attribution, "GeoNames") {
		t.Fatalf("page: %+v", page)
	}
	if cc := r.header.Get("Cache-Control"); cc != "public, max-age=86400" || r.header.Get("Pragma") != "" {
		t.Fatalf("cache-control %q pragma %q", cc, r.header.Get("Pragma"))
	}
	etag := r.header.Get("ETag")
	if etag == "" {
		t.Fatal("no ETag")
	}
	if again := c.raw(http.MethodGet, "/api/v1/astrology/places?q=paris&limit=2", nil, map[string]string{"If-None-Match": etag}); again.status != http.StatusNotModified || len(again.body) != 0 {
		t.Fatalf("conditional request: %d %s", again.status, again.body)
	}
	var next service.PlacesPage
	c.must(http.StatusOK, http.MethodGet, "/api/v1/astrology/places?q=paris&limit=2&cursor="+url.QueryEscape(page.NextCursor), nil, &next)
	if len(next.Items) != 2 || next.Items[0].ID == page.Items[0].ID || next.Items[0].ID == page.Items[1].ID {
		t.Fatalf("next page: %+v", next)
	}
	// Accent- and case-insensitive, with alternate names.
	var accents service.PlacesPage
	c.must(http.StatusOK, http.MethodGet, "/api/v1/astrology/places?q="+url.QueryEscape("ZÜRICH"), nil, &accents)
	if len(accents.Items) == 0 || accents.Items[0].ID != "gn:2657896" {
		t.Fatalf("zurich: %+v", accents.Items)
	}

	for query, field := range map[string]string{"q=p": "q", "q=paris&cursor=bogus": "cursor", "q=paris&limit=50": "limit"} {
		if f := fieldErrors(t, c.call(http.MethodGet, "/api/v1/astrology/places?"+query, nil)); f[field] == "" {
			t.Errorf("%s: fields %v", query, f)
		}
	}

	var paris struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Admin1 string `json:"admin1"`
	}
	r = c.must(http.StatusOK, http.MethodGet, "/api/v1/astrology/places/gn:2988507", nil, &paris)
	if paris.ID != "gn:2988507" || paris.Admin1 != "Île-de-France" || r.header.Get("ETag") == "" {
		t.Fatalf("place: %+v", paris)
	}
	// Legacy slugs (stored before the gazetteer) resolve to their place.
	c.must(http.StatusOK, http.MethodGet, "/api/v1/astrology/places/paris-fr", nil, &paris)
	if paris.ID != "gn:2988507" {
		t.Fatalf("legacy slug: %+v", paris)
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/astrology/places/gn%3A2643743", nil, &paris)
	if paris.Name != "London" {
		t.Fatalf("escaped id: %+v", paris)
	}
	if r := c.call(http.MethodGet, "/api/v1/astrology/places/gn:1", nil); r.status != http.StatusNotFound {
		t.Fatalf("unknown place: %d %s", r.status, r.body)
	}
}

func TestNatalCalculator(t *testing.T) {
	e := newEnv(t)
	c := e.client()
	var out service.NatalView
	r := c.must(http.StatusOK, http.MethodPost, "/api/v1/astrology/natal", map[string]any{
		"birth_date": "1990-05-01", "birth_time": "14:30", "place_id": "gn:2988507", "tz": "Europe/Paris",
	}, &out)
	if r.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("calculators must not be cached: %q", r.header.Get("Cache-Control"))
	}
	ch := out.Chart
	if !ch.TimeKnown || ch.Angles == nil || ch.RisingSign == "" || len(ch.Houses) != 12 || ch.HouseSystemUsed != "placidus" ||
		ch.SunSign != "Taurus" || ch.WallTimeStatus != "ok" || ch.Place == nil || ch.Place.ID != "gn:2988507" {
		t.Fatalf("chart: %+v", ch)
	}
	if !ch.Instant.Equal(time.Date(1990, 5, 1, 12, 30, 0, 0, time.UTC)) {
		t.Fatalf("14:30 CEST is 12:30 UTC, got %v", ch.Instant)
	}
	if out.Tarot == nil || len(out.Tarot.Cards) != 12 || out.Numerology == nil || out.Numerology.LifePath != 7 || len(ch.Aspects) == 0 {
		t.Fatalf("includes: tarot=%v numerology=%v aspects=%d", out.Tarot, out.Numerology, len(ch.Aspects))
	}
	for _, card := range out.Tarot.Cards {
		if card.House < 1 || card.House > 12 || card.Card == "" || (card.Source.Kind != "planet" && card.Source.Kind != "cusp_sign") {
			t.Fatalf("card %+v", card)
		}
	}
	if out.Engine.Version == "" || out.Engine.TZData == "" || out.Engine.Gazetteer == "" || out.Disclaimer == "" || out.Attribution == "" {
		t.Fatalf("engine/legal: %+v %q %q", out.Engine, out.Disclaimer, out.Attribution)
	}

	// Unknown time: no angles or houses; the spread uses whole signs from
	// the Sun. Only the requested parts are computed.
	var noon service.NatalView
	c.must(http.StatusOK, http.MethodPost, "/api/v1/astrology/natal", map[string]any{
		"birth_date": "1990-05-01", "include": []string{"tarot"},
	}, &noon)
	if noon.Chart.TimeKnown || noon.Chart.Angles != nil || len(noon.Chart.Houses) != 0 || noon.Tarot == nil ||
		noon.Tarot.HouseSystem != "whole_sign" || noon.Numerology != nil || len(noon.Chart.Aspects) != 0 {
		t.Fatalf("unknown time: %+v", noon)
	}

	// A wall time that never existed (Paris skipped 02:00–03:00 on
	// 25 March 1990) is flagged rather than silently shifted.
	var gap service.NatalView
	c.must(http.StatusOK, http.MethodPost, "/api/v1/astrology/natal", map[string]any{
		"birth_date": "1990-03-25", "birth_time": "02:30", "place_id": "gn:2988507", "include": []string{"numerology"},
	}, &gap)
	if gap.Chart.WallTimeStatus != "skipped" {
		t.Fatalf("skipped wall time: %q", gap.Chart.WallTimeStatus)
	}

	cases := []struct {
		body  map[string]any
		field string
	}{
		{map[string]any{"birth_date": "1990-05-01", "birth_time": "14:30"}, "place_id"},
		{map[string]any{"birth_date": "1990-05-01", "place_id": "atlantis"}, "place_id"},
		{map[string]any{"birth_date": "1990-05-01", "birth_time": "7pm", "place_id": "gn:2988507"}, "birth_time"},
		{map[string]any{"birth_date": "1990-05-01", "house_system": "koch"}, "house_system"},
		{map[string]any{"birth_date": "1990-05-01", "include": []string{"runes"}}, "include"},
		{map[string]any{"birth_date": "1990-05-01", "tz": "Mars/Base"}, "tz"},
		{map[string]any{"birth_date": "3000-01-01"}, "birth_date"},
		{map[string]any{"birth_date": "1990-05-01", "email": "a@b.c"}, "email"},
		{map[string]any{"birth_date": 19900501}, "birth_date"},
	}
	for _, tc := range cases {
		if f := fieldErrors(t, c.call(http.MethodPost, "/api/v1/astrology/natal", tc.body)); f[tc.field] == "" {
			t.Errorf("%v: fields %v", tc.body, f)
		}
	}
	// Messages are translated.
	fr := e.client()
	fr.locale = "fr"
	f := fieldErrors(t, fr.call(http.MethodPost, "/api/v1/astrology/natal", map[string]any{"birth_date": "1990-05-01", "birth_time": "14:30"}))
	if f["place_id"] != "Choisissez votre lieu de naissance pour utiliser une heure de naissance." {
		t.Fatalf("fr: %v", f)
	}
	// The calculators are CSRF-protected like every other write.
	if r := c.raw(http.MethodPost, "/api/v1/astrology/natal", jsonBody(t, map[string]any{"birth_date": "1990-05-01"}),
		map[string]string{"Content-Type": "application/json"}); r.status != http.StatusForbidden {
		t.Fatalf("without a CSRF token: %d", r.status)
	}
}

func TestNumerologyAndSunSignCalculators(t *testing.T) {
	e := newEnv(t)
	c := e.client()
	var n service.NumerologyView
	c.must(http.StatusOK, http.MethodPost, "/api/v1/astrology/numerology", map[string]any{"birth_date": "1987-11-29", "as_of": "2026-03-01"}, &n)
	// 11 (master) + 11 (master: 2+9) + 1+9+8+7 = 25 → 7 → 11 + 11 + 7 = 29 → 11.
	if n.LifePath != 11 || !n.LifePathMaster || len(n.LifePathReduction.Components) != 3 || n.AsOf != "2026-03-01" {
		t.Fatalf("numerology: %+v", n)
	}
	// "Today" is the requester's date.
	c.must(http.StatusOK, http.MethodPost, "/api/v1/astrology/numerology", map[string]any{"birth_date": "1987-11-29", "tz": "Pacific/Kiritimati"}, &n)
	if want := dateIn(t, e.clock.Now(), "Pacific/Kiritimati", 0); n.AsOf != want || n.TZ != "Pacific/Kiritimati" {
		t.Fatalf("as of %s, want %s", n.AsOf, want)
	}
	if f := fieldErrors(t, c.call(http.MethodPost, "/api/v1/astrology/numerology", map[string]any{"birth_date": "1987-11-29", "as_of": "2101-01-01"})); f["as_of"] == "" {
		t.Fatalf("as_of: %v", f)
	}
	if f := fieldErrors(t, c.call(http.MethodPost, "/api/v1/astrology/numerology", map[string]any{})); f["birth_date"] == "" {
		t.Fatalf("birth date: %v", f)
	}

	var sun service.SunSignView
	c.must(http.StatusOK, http.MethodPost, "/api/v1/astrology/sun-sign", map[string]any{"birth_date": "1990-03-20"}, &sun)
	if !sun.Uncertain || strings.Join(sun.Alternatives, ",") != "Pisces,Aries" {
		t.Fatalf("cusp without a time: %+v", sun)
	}
	c.must(http.StatusOK, http.MethodPost, "/api/v1/astrology/sun-sign", map[string]any{"birth_date": "1990-03-20", "birth_time": "23:30", "place_id": "gn:2988507"}, &sun)
	if sun.Uncertain || sun.SunSign != "Aries" || sun.Element != "Fire" || sun.Modality != "Cardinal" {
		t.Fatalf("cusp with time and place: %+v", sun)
	}
	if f := fieldErrors(t, c.call(http.MethodPost, "/api/v1/astrology/sun-sign", map[string]any{"birth_date": "1990-03-20", "include": []string{"tarot"}})); f["include"] == "" {
		t.Fatalf("sun-sign accepts only birth fields: %v", f)
	}
}

func TestHoroscopeAndAztro(t *testing.T) {
	e := newEnv(t)
	c := e.client()
	c.locale = "fr"
	var h struct {
		Sign               string `json:"sign"`
		SignName           string `json:"sign_name"`
		Date               string `json:"date"`
		Locale             string `json:"locale"`
		TZ                 string `json:"tz"`
		Text               string `json:"text"`
		Focus              string `json:"focus"`
		CompatibilityScore int    `json:"compatibility_score"`
		MoonSign           string `json:"moon_sign"`
		DateRange          struct {
			Label string `json:"label"`
		} `json:"date_range"`
	}
	r := c.must(http.StatusOK, http.MethodGet, "/api/v1/astrology/horoscope/aries?day=tomorrow&tz=Pacific/Kiritimati", nil, &h)
	if h.Sign != "Aries" || h.SignName != "Bélier" || h.Locale != "fr" || h.TZ != "Pacific/Kiritimati" || h.Text == "" || h.Focus == "" ||
		h.MoonSign == "" || h.DateRange.Label == "" || h.CompatibilityScore < 0 || h.CompatibilityScore > 100 {
		t.Fatalf("reading: %+v", h)
	}
	if want := dateIn(t, e.clock.Now(), "Pacific/Kiritimati", 1); h.Date != want {
		t.Fatalf("tomorrow on Kiritimati is %s, got %s", want, h.Date)
	}
	vary := strings.Join(r.header.Values("Vary"), ",")
	if r.header.Get("Cache-Control") != "public, max-age=900" || !strings.Contains(vary, "X-Locale") || !strings.Contains(vary, "Accept-Language") {
		t.Fatalf("headers: %v", r.header)
	}
	// The same day in Pago Pago (UTC−11) can be two calendar days earlier.
	c.must(http.StatusOK, http.MethodGet, "/api/v1/astrology/horoscope/aries?day=tomorrow&tz=Pacific/Pago_Pago", nil, &h)
	if want := dateIn(t, e.clock.Now(), "Pacific/Pago_Pago", 1); h.Date != want {
		t.Fatalf("tomorrow in Pago Pago is %s, got %s", want, h.Date)
	}
	for query, field := range map[string]string{"dragon": "sign", "aries?day=someday": "day", "aries?tz=Nowhere/Land": "tz", "aries?date=2001-01-01": "date"} {
		if f := fieldErrors(t, c.call(http.MethodGet, "/api/v1/astrology/horoscope/"+query, nil)); f[field] == "" {
			t.Errorf("%s: %v", query, f)
		}
	}

	// aztro: same parameters and fields as the aztro API, plus staryield{}.
	var a struct {
		DateRange     string `json:"date_range"`
		CurrentDate   string `json:"current_date"`
		Description   string `json:"description"`
		Compatibility string `json:"compatibility"`
		Mood          string `json:"mood"`
		Color         string `json:"color"`
		LuckyNumber   string `json:"lucky_number"`
		LuckyTime     string `json:"lucky_time"`
		Staryield     struct {
			Sign   string `json:"sign"`
			Date   string `json:"date"`
			Locale string `json:"locale"`
		} `json:"staryield"`
	}
	en := e.client()
	r = en.must(http.StatusOK, http.MethodGet, "/api/v1/aztro?sign=leo&day=yesterday", nil, &a)
	if a.Description == "" || a.Compatibility == "" || a.Mood == "" || a.Color == "" || a.LuckyNumber == "" || a.LuckyTime == "" ||
		a.DateRange == "" || a.CurrentDate == "" || a.Staryield.Sign != "Leo" || a.Staryield.Locale != "en" {
		t.Fatalf("aztro: %+v", a)
	}
	if want := dateIn(t, e.clock.Now(), "UTC", -1); a.Staryield.Date != want || r.header.Get("Cache-Control") != "public, max-age=1800" {
		t.Fatalf("yesterday %s (want %s), cache %q", a.Staryield.Date, want, r.header.Get("Cache-Control"))
	}
	if again := en.raw(http.MethodGet, "/api/v1/aztro?sign=leo&day=yesterday", nil, map[string]string{"If-None-Match": r.header.Get("ETag")}); again.status != http.StatusNotModified {
		t.Fatalf("aztro conditional GET: %d", again.status)
	}
	// POST like aztro clients do: no CSRF token, query or form parameters.
	third := e.client()
	third.origin = ""
	r = third.raw(http.MethodPost, "/api/v1/aztro?sign=aries&day=today&lang=es", nil, nil)
	if r.status != http.StatusOK || r.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("aztro POST: %d %s %q", r.status, r.body, r.header.Get("Cache-Control"))
	}
	r.decode(t, &a)
	if a.Staryield.Sign != "Aries" || a.Staryield.Locale != "es" {
		t.Fatalf("aztro POST: %+v", a)
	}
	r = third.raw(http.MethodPost, "/api/v1/aztro", strings.NewReader("sign=Pisces&day=tomorrow"), map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	if r.status != http.StatusOK {
		t.Fatalf("aztro form POST: %d %s", r.status, r.body)
	}
	r.decode(t, &a)
	if a.Staryield.Sign != "Pisces" {
		t.Fatalf("aztro form POST: %+v", a)
	}
	if f := fieldErrors(t, third.raw(http.MethodPost, "/api/v1/aztro?sign=ophiuchus", nil, nil)); f["sign"] == "" {
		t.Fatalf("aztro sign: %v", f)
	}

	var ev service.CosmicEvents
	r = c.must(http.StatusOK, http.MethodGet, "/api/v1/astrology/events?count=2", nil, &ev)
	if len(ev.Lunar) != 2 || r.header.Get("Cache-Control") != "public, max-age=900" {
		t.Fatalf("events: %+v", ev)
	}
	if f := fieldErrors(t, c.call(http.MethodGet, "/api/v1/astrology/events?count=13", nil)); f["count"] == "" {
		t.Fatalf("count: %v", f)
	}
}

func TestMyAstrologyUsesStoredBirthData(t *testing.T) {
	e := newEnv(t)
	anon := e.client()
	for _, path := range []string{"/api/v1/me/chart", "/api/v1/me/horoscope", "/api/v1/me/numerology"} {
		if r := anon.call(http.MethodGet, path, nil); r.status != http.StatusUnauthorized {
			t.Fatalf("%s anonymously: %d", path, r.status)
		}
	}

	c := e.registerClient() // born 1990-05-17, no time or place yet
	var chart service.ChartView
	r := c.must(http.StatusOK, http.MethodGet, "/api/v1/me/chart", nil, &chart)
	if chart.TimeKnown || chart.HasLocation || chart.SunSign != "Taurus" || r.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("date only: %+v", chart.Chart)
	}
	c.must(http.StatusOK, http.MethodPatch, "/api/v1/me/profile", map[string]any{"birth_time": "07:45", "birth_place_id": "gn:2988507"}, nil)
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/chart", nil, &chart)
	if !chart.TimeKnown || chart.RisingSign == "" || len(chart.Houses) != 12 || chart.Place == nil || chart.Place.ID != "gn:2988507" {
		t.Fatalf("time and place: %+v", chart.Chart)
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/chart?house_system=equal", nil, &chart)
	if chart.HouseSystemUsed != "equal" {
		t.Fatalf("equal houses: %s", chart.HouseSystemUsed)
	}
	if f := fieldErrors(t, c.call(http.MethodGet, "/api/v1/me/chart?house_system=koch", nil)); f["house_system"] == "" {
		t.Fatalf("house system: %v", f)
	}

	var h service.MyHoroscopeView
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/horoscope?tz=Pacific/Kiritimati", nil, &h)
	if h.Sign != "Taurus" || h.SunSignUncertain || h.Text == "" || len(h.Events.Lunar) == 0 {
		t.Fatalf("horoscope: %+v", h)
	}
	if want := dateIn(t, e.clock.Now(), "Pacific/Kiritimati", 0); h.Date != want {
		t.Fatalf("today on Kiritimati is %s, got %s", want, h.Date)
	}
	var n service.NumerologyView
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/numerology?tz=Pacific/Pago_Pago", nil, &n)
	// 5 + (1+7) + (1+9+9+0 → 1) = 14 → 5.
	if want := dateIn(t, e.clock.Now(), "Pacific/Pago_Pago", 0); n.LifePath != 5 || n.AsOf != want {
		t.Fatalf("numerology: %+v (want as of %s)", n, want)
	}

	// A place stored before the gazetteer (legacy slug and coordinates)
	// resolves to its GeoNames place.
	if err := e.app.DB.Exec(`UPDATE user_profiles SET birth_place_id = 'london-gb', birth_place = 'London, United Kingdom',
		birth_latitude = 51.5074, birth_longitude = -0.1278, birth_timezone = 'Europe/London' WHERE user_id = ?`, c.user.ID).Error; err != nil {
		t.Fatal(err)
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/chart", nil, &chart)
	if chart.Place == nil || chart.Place.ID != "gn:2643743" || chart.Place.Admin1 != "England" || !chart.TimeKnown {
		t.Fatalf("legacy place: %+v", chart.Place)
	}

	// A stored time without a place is ignored, with a warning.
	if err := e.app.DB.Exec(`UPDATE user_profiles SET birth_place_id = '', birth_place = '', birth_latitude = NULL,
		birth_longitude = NULL, birth_timezone = '' WHERE user_id = ?`, c.user.ID).Error; err != nil {
		t.Fatal(err)
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/chart", nil, &chart)
	if chart.TimeKnown || len(chart.Warnings) != 1 || chart.Warnings[0] != service.WarningBirthPlaceMissing {
		t.Fatalf("time without place: %+v %v", chart.Chart, chart.Warnings)
	}
}

// Anonymous calculations leave nothing behind: no rows besides analytics
// events, which only record which inputs were present.
func TestCalculatorsAreStateless(t *testing.T) {
	e := newEnv(t)
	count := func() (n int64) {
		e.app.DB.Raw("SELECT (SELECT count(*) FROM users) + (SELECT count(*) FROM user_profiles)").Scan(&n)
		return n
	}
	before := count()
	c := e.client()
	birth := map[string]any{"birth_date": "1990-05-01", "birth_time": "14:30", "place_id": "gn:2988507"}
	c.must(http.StatusOK, http.MethodPost, "/api/v1/astrology/natal", birth, nil)
	c.must(http.StatusOK, http.MethodPost, "/api/v1/astrology/chart", birth, nil)
	c.must(http.StatusOK, http.MethodPost, "/api/v1/astrology/sun-sign", birth, nil)
	c.must(http.StatusOK, http.MethodPost, "/api/v1/astrology/numerology", map[string]any{"birth_date": "1990-05-01"}, nil)
	c.must(http.StatusOK, http.MethodGet, "/api/v1/astrology/places?q=paris", nil, nil)
	if after := count(); after != before {
		t.Fatalf("rows %d → %d", before, after)
	}
	time.Sleep(200 * time.Millisecond)
	e.app.Tracker.Stop(context.Background())
	var rows []string
	e.app.DB.Raw("SELECT properties::text FROM analytics_events WHERE name LIKE 'astro.%'").Scan(&rows)
	if len(rows) < 4 {
		t.Fatalf("astro events: %v", rows)
	}
	for _, raw := range rows {
		var props map[string]any
		if err := json.Unmarshal([]byte(raw), &props); err != nil {
			t.Fatal(err)
		}
		for k := range props {
			if analytics.Personal(k) || k == "sign" {
				t.Fatalf("personal data tracked: %s", raw)
			}
		}
		if strings.Contains(raw, "1990") || strings.Contains(raw, "14:30") || strings.Contains(raw, "2988507") {
			t.Fatalf("birth data tracked: %s", raw)
		}
	}
}

func TestAstrologyRateLimits(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.RateLimit.Scale = 1 })
	c := e.client()
	body := map[string]any{"birth_date": "1990-05-01"}
	for i := 0; i < 10; i++ { // burst of 10
		c.must(http.StatusOK, http.MethodPost, "/api/v1/astrology/numerology", body, nil)
	}
	r := c.call(http.MethodPost, "/api/v1/astrology/natal", body)
	if r.status != http.StatusTooManyRequests || r.header.Get("Retry-After") == "" {
		t.Fatalf("the calculators share one bucket: %d %v", r.status, r.header)
	}
	// Other buckets are independent.
	c.must(http.StatusOK, http.MethodGet, "/api/v1/astrology/places?q=paris", nil, nil)
	c.must(http.StatusOK, http.MethodGet, "/api/v1/aztro?sign=aries", nil, nil)
	for i := 0; i < 19; i++ {
		c.must(http.StatusOK, http.MethodGet, "/api/v1/aztro?sign=aries", nil, nil)
	}
	if r := c.call(http.MethodGet, "/api/v1/aztro?sign=aries", nil); r.status != http.StatusTooManyRequests {
		t.Fatalf("aztro burst of 20: %d", r.status)
	}
}

// Migration 0006 moves profiles that chose a place from the original city
// list to GeoNames, on a database holding such profiles.
func TestPlacesMigrationMovesLegacyProfiles(t *testing.T) {
	admin, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	name := "staryield_places_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	if err := admin.Exec("CREATE DATABASE " + name).Error; err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() { admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)") })
	u, err := url.Parse(testDSN)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if conn, err := db.DB(); err == nil {
		t.Cleanup(func() { _ = conn.Close() })
	}

	all, err := migrations.Load()
	if err != nil {
		t.Fatal(err)
	}
	const target = "0006_places_geonames"
	apply := func(from, to string) {
		for _, m := range all {
			if m.Version >= from && m.Version < to {
				if err := db.Exec(m.SQL).Error; err != nil {
					t.Fatalf("apply %s: %v", m.Version, err)
				}
			}
		}
	}
	apply("", target)

	ids := map[string]uuid.UUID{"paris": uuid.New(), "unknown": uuid.New(), "none": uuid.New(), "current": uuid.New()}
	for label, id := range ids {
		if err := db.Exec(`INSERT INTO users (id, email, password_hash, role, terms_accepted_at) VALUES (?, ?, 'x', 'client', now())`,
			id, label+"@example.com").Error; err != nil {
			t.Fatal(err)
		}
	}
	insert := func(id uuid.UUID, placeID, place string, lat, lon any, zone string) {
		t.Helper()
		if err := db.Exec(`INSERT INTO user_profiles (user_id, birth_date, birth_time, birth_place_id, birth_place, birth_latitude, birth_longitude, birth_timezone)
			VALUES (?, '1990-05-01', '14:30', ?, ?, ?, ?, ?)`, id, placeID, place, lat, lon, zone).Error; err != nil {
			t.Fatal(err)
		}
	}
	insert(ids["paris"], "paris-fr", "Paris, France", 48.8566, 2.3522, "Europe/Paris")
	insert(ids["unknown"], "atlantis-xx", "Atlantis", 0.0, 0.0, "UTC")
	insert(ids["none"], "", "", nil, nil, "")
	insert(ids["current"], "gn:2643743", "London, England, United Kingdom", 51.50853, -0.12574, "Europe/London")

	apply(target, "9999")

	type row struct {
		BirthPlaceID     string
		BirthPlace       string
		BirthAdmin1      string
		BirthLatitude    *float64
		BirthLongitude   *float64
		BirthTimezone    string
		BirthPlaceSource string
	}
	get := func(id uuid.UUID) row {
		var r row
		if err := db.Raw(`SELECT birth_place_id, birth_place, birth_admin1, birth_latitude, birth_longitude, birth_timezone, birth_place_source
			FROM user_profiles WHERE user_id = ?`, id).Scan(&r).Error; err != nil {
			t.Fatal(err)
		}
		return r
	}
	p := get(ids["paris"])
	if p.BirthPlaceID != "gn:2988507" || p.BirthPlace != "Paris, Île-de-France, France" || p.BirthAdmin1 != "Île-de-France" ||
		p.BirthTimezone != "Europe/Paris" || p.BirthPlaceSource != "geonames" || p.BirthLatitude == nil || *p.BirthLatitude != 48.8534 {
		t.Fatalf("paris: %+v", p)
	}
	if unk := get(ids["unknown"]); unk.BirthPlaceID != "atlantis-xx" || unk.BirthPlaceSource != "legacy" {
		t.Fatalf("unknown slug: %+v", unk)
	}
	if none := get(ids["none"]); none.BirthPlaceSource != "" || none.BirthPlaceID != "" {
		t.Fatalf("no place: %+v", none)
	}
	// Rows already on GeoNames IDs are only marked.
	if cur := get(ids["current"]); cur.BirthPlaceID != "gn:2643743" || cur.BirthPlaceSource != "geonames" || cur.BirthPlace != "London, England, United Kingdom" {
		t.Fatalf("current: %+v", cur)
	}
	// The constraints hold.
	if err := db.Exec(`UPDATE user_profiles SET birth_place_source = 'guess' WHERE user_id = ?`, ids["none"]).Error; err == nil {
		t.Fatal("birth_place_source must be constrained")
	}
}
