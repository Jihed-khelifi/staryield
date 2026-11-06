package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/astro"
	"github.com/Jihed-khelifi/staryield/backend/internal/astro/horoscope"
	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// profileRepo serves one stored profile. Every other repository method
// panics (nil embedded interface), which proves the anonymous calculators
// never touch storage.
type profileRepo struct {
	repository.UserRepository
	profile *domain.Profile
}

func (r profileRepo) GetProfile(context.Context, uuid.UUID) (*domain.Profile, error) {
	if r.profile == nil {
		return nil, repository.ErrNotFound
	}
	p := *r.profile
	return &p, nil
}

// countingProvider counts readings composed (cache misses).
type countingProvider struct{ n *int }

func (p countingProvider) Name() string { return "counting" }

func (p countingProvider) Daily(ctx context.Context, req horoscope.Request) (horoscope.Reading, error) {
	*p.n++
	return horoscope.Engine{}.Daily(ctx, req)
}

var astroNow = time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)

func newAstro(t *testing.T, p *domain.Profile) (*AstrologyService, *analytics.Recorder, *clock.Fake) {
	t.Helper()
	clk := clock.NewFake(astroNow)
	rec := &analytics.Recorder{}
	return NewAstrologyService(profileRepo{profile: p}, rec, clk, cache.NewMemory(clk.Now)), rec, clk
}

// fieldsOf returns the validation fields of err (failing when it is not a
// validation error).
func fieldsOf(t *testing.T, err error) map[string]any {
	t.Helper()
	e, ok := apperror.As(err)
	if !ok || e.Code != apperror.CodeValidation {
		t.Fatalf("want a validation error, got %v", err)
	}
	return e.Details["fields"].(map[string]any)
}

func date(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func TestNatalValidation(t *testing.T) {
	s, rec, _ := newAstro(t, nil)
	ctx := context.Background()
	cases := []struct {
		in    BirthInput
		field string
		msg   string
	}{
		{BirthInput{BirthDate: "1990-05-01", BirthTime: "14:30"}, "place_id", "Choose your birth place to use a birth time."},
		{BirthInput{BirthDate: "1990-05-01", PlaceID: "gn:0"}, "place_id", "Choose a birth place from the list."},
		{BirthInput{BirthDate: "1990-05-01", BirthTime: "25:00", PlaceID: "paris-fr"}, "birth_time", "Use the 24-hour format HH:MM."},
		{BirthInput{BirthDate: "1990-05-01", HouseSystem: "koch"}, "house_system", "Choose Placidus, Whole Sign or Equal houses."},
		{BirthInput{BirthDate: "1990-05-01", Include: []string{"tarot", "runes"}}, "include", "Choose from aspects, tarot and numerology."},
		{BirthInput{BirthDate: "1990-05-01", TZ: "Mars/Olympus"}, "tz", "Choose a valid time zone."},
		{BirthInput{}, "birth_date", "Enter your birth date."},
		{BirthInput{BirthDate: "1890-01-01"}, "birth_date", ""},
		{BirthInput{BirthDate: "2030-01-01"}, "birth_date", ""},
	}
	for _, c := range cases {
		_, err := s.Natal(ctx, c.in)
		fields := fieldsOf(t, err)
		got, _ := fields[c.field].(string)
		if got == "" || (c.msg != "" && got != c.msg) {
			t.Errorf("%+v: fields %v, want %s=%q", c.in, fields, c.field, c.msg)
		}
	}
	if len(rec.Events) != 0 {
		t.Fatalf("rejected requests must not be tracked: %v", rec.Events)
	}
}

func TestNatalWithTimeAndPlace(t *testing.T) {
	s, rec, _ := newAstro(t, nil)
	out, err := s.Natal(context.Background(), BirthInput{BirthDate: "1990-05-01", BirthTime: "14:30", PlaceID: "gn:2988507"})
	if err != nil {
		t.Fatal(err)
	}
	c := out.Chart
	if !c.TimeKnown || !c.HasLocation || c.Angles == nil || c.RisingSign == "" || len(c.Houses) != 12 || c.HouseSystemUsed != astro.Placidus {
		t.Fatalf("chart: %+v", c)
	}
	// 14:30 CEST is 12:30 UTC.
	if !c.Instant.Equal(time.Date(1990, 5, 1, 12, 30, 0, 0, time.UTC)) || c.WallTimeStatus != "ok" {
		t.Fatalf("instant %v status %q", c.Instant, c.WallTimeStatus)
	}
	if c.SunSign != "Taurus" || c.Place == nil || c.Place.ID != "gn:2988507" || c.Attribution == "" || out.Attribution == "" {
		t.Fatalf("sun %s place %+v attribution %q", c.SunSign, c.Place, c.Attribution)
	}
	// Life path of 1990-05-01: 5 + 1 + (1+9+9+0 = 19 → 1) = 7.
	if out.Tarot == nil || len(out.Tarot.Cards) != 12 || out.Numerology == nil || out.Numerology.LifePath != 7 || len(c.Aspects) == 0 {
		t.Fatalf("includes: tarot %+v numerology %+v aspects %d", out.Tarot, out.Numerology, len(c.Aspects))
	}
	if out.Engine.Version == "" || out.Engine.TZData == "" || out.Engine.Gazetteer == "" || out.Disclaimer == "" {
		t.Fatalf("engine %+v", out.Engine)
	}
	// Analytics record which inputs were present, never the inputs.
	if len(rec.Events) != 1 || rec.Events[0].Name != "astro.natal_computed" {
		t.Fatalf("events %+v", rec.Events)
	}
	for k, v := range rec.Events[0].Properties {
		if k != "has_time" && k != "has_place" && k != "house_system" {
			t.Errorf("unexpected property %s=%v", k, v)
		}
	}
}

func TestNatalWithoutTime(t *testing.T) {
	s, _, _ := newAstro(t, nil)
	out, err := s.Natal(context.Background(), BirthInput{BirthDate: "1990-05-01", PlaceID: "paris-fr", Include: []string{"tarot"}})
	if err != nil {
		t.Fatal(err)
	}
	c := out.Chart
	if c.TimeKnown || c.Angles != nil || c.RisingSign != "" || len(c.Houses) != 0 || c.BirthTime != "" {
		t.Fatalf("an unknown time has no angles or houses: %+v", c)
	}
	// The place still sets the day: noon in Paris.
	if !c.Instant.Equal(time.Date(1990, 5, 1, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("instant %v", c.Instant)
	}
	if out.Tarot == nil || out.Tarot.HouseSystem != astro.WholeSign || out.Numerology != nil || len(c.Aspects) != 0 {
		t.Fatalf("include tarot only: %+v %+v %d", out.Tarot, out.Numerology, len(c.Aspects))
	}
	if c.MoonSign == "" || c.SunSign != "Taurus" {
		t.Fatalf("signs %s %s", c.SunSign, c.MoonSign)
	}
}

func TestChartLegacyEndpointKeepsShape(t *testing.T) {
	s, rec, _ := newAstro(t, nil)
	out, err := s.Chart(context.Background(), BirthInput{BirthDate: "1945-04-30", BirthTime: "15:30", PlaceID: "london-gb"})
	if err != nil {
		t.Fatal(err)
	}
	// London kept double summer time (BST+1) in April 1945: 15:30 is 13:30 UTC.
	if !out.Instant.Equal(time.Date(1945, 4, 30, 13, 30, 0, 0, time.UTC)) || len(out.HouseCusps) != 12 || out.LifePath == 0 {
		t.Fatalf("instant %v cusps %v life path %d", out.Instant, out.HouseCusps, out.LifePath)
	}
	if rec.Events[0].Name != "astro.chart_computed" || rec.Events[0].Properties["sun_sign"] != nil {
		t.Fatalf("events %+v", rec.Events)
	}
}

func TestNumerologyUsesRequesterDate(t *testing.T) {
	s, _, _ := newAstro(t, nil)
	ctx := context.Background()
	// At 10:00 UTC it is already 6 November on Kiritimati (UTC+14) and still
	// 4 November in Pago Pago (UTC−11).
	east, err := s.Numerology(ctx, NumerologyInput{BirthDate: "1990-05-01", TZ: "Pacific/Kiritimati"})
	if err != nil {
		t.Fatal(err)
	}
	west, err := s.Numerology(ctx, NumerologyInput{BirthDate: "1990-05-01", TZ: "Pacific/Pago_Pago"})
	if err != nil {
		t.Fatal(err)
	}
	if east.AsOf != "2026-11-06" || west.AsOf != "2026-11-04" || east.PersonalDay == west.PersonalDay {
		t.Fatalf("east %+v west %+v", east, west)
	}
	if east.LifePath != 7 || east.LifePathReduction.Number != 7 || len(east.LifePathReduction.Components) != 3 {
		t.Fatalf("life path %+v", east.LifePathReduction)
	}
	fixed, err := s.Numerology(ctx, NumerologyInput{BirthDate: "1990-05-01", AsOf: "2026-01-01"})
	if err != nil || fixed.AsOf != "2026-01-01" || fixed.TZ != "UTC" {
		t.Fatalf("as_of: %+v %v", fixed, err)
	}
	// 5 + 1 + (2+0+2+6) = 16 → 7.
	if fixed.PersonalYear != 7 {
		t.Fatalf("personal year %d", fixed.PersonalYear)
	}
	_, err = s.Numerology(ctx, NumerologyInput{BirthDate: "1990-05-01", AsOf: "2200-01-01", TZ: "Nowhere/City"})
	fields := fieldsOf(t, err)
	if fields["as_of"] != "Use a date between 1900 and 2100 (YYYY-MM-DD)." || fields["tz"] != "Choose a valid time zone." {
		t.Fatalf("fields %v", fields)
	}
}

func TestSunSignOnTheCusp(t *testing.T) {
	s, _, _ := newAstro(t, nil)
	ctx := context.Background()
	// The Sun entered Aries at 21:19 UTC on 20 March 1990.
	vague, err := s.SunSign(ctx, BirthInput{BirthDate: "1990-03-20"})
	if err != nil {
		t.Fatal(err)
	}
	if !vague.Uncertain || strings.Join(vague.Alternatives, ",") != "Pisces,Aries" || vague.SunSign != "Pisces" {
		t.Fatalf("no time: %+v", vague)
	}
	late, err := s.SunSign(ctx, BirthInput{BirthDate: "1990-03-20", BirthTime: "23:30", PlaceID: "gn:2988507"})
	if err != nil {
		t.Fatal(err)
	}
	if late.Uncertain || late.SunSign != "Aries" || late.Element != "Fire" || late.Modality != "Cardinal" {
		t.Fatalf("23:30 in Paris: %+v", late)
	}
	early, _ := s.SunSign(ctx, BirthInput{BirthDate: "1990-03-20", BirthTime: "08:00", PlaceID: "gn:2988507"})
	if early.SunSign != "Pisces" || early.Uncertain {
		t.Fatalf("08:00 in Paris: %+v", early)
	}
	mid, _ := s.SunSign(ctx, BirthInput{BirthDate: "1990-07-15"})
	if mid.Uncertain || mid.SunSign != "Cancer" || len(mid.Alternatives) != 0 {
		t.Fatalf("mid-sign: %+v", mid)
	}
}

func TestHoroscopeValidationCacheAndZones(t *testing.T) {
	s, _, clk := newAstro(t, nil)
	n := 0
	s.horoscope = countingProvider{n: &n}
	ctx := context.Background()

	for q, field := range map[HoroscopeQuery]string{
		{Sign: "dragon"}:                    "sign",
		{Sign: "aries", Day: "someday"}:     "day",
		{Sign: "aries", TZ: "Moon/Base"}:    "tz",
		{Sign: "aries", Date: "05/11/2026"}: "date",
		{Sign: "aries", Date: "2030-01-01"}: "date",
	} {
		_, err := s.HoroscopeFor(ctx, q)
		if _, ok := fieldsOf(t, err)[field]; !ok {
			t.Errorf("%+v: want field %s", q, field)
		}
	}

	r, err := s.HoroscopeFor(ctx, HoroscopeQuery{Sign: "ARIES", Day: "tomorrow", TZ: "Pacific/Kiritimati", Locale: "fr"})
	if err != nil {
		t.Fatal(err)
	}
	// Tomorrow on Kiritimati (already 6 November there) is 7 November.
	if r.Sign != "Aries" || r.Date != "2026-11-07" || r.Locale != "fr" || r.TZ != "Pacific/Kiritimati" || r.Text == "" {
		t.Fatalf("reading %+v", r)
	}
	again, _ := s.HoroscopeFor(ctx, HoroscopeQuery{Sign: "aries", Day: "tomorrow", TZ: "Pacific/Kiritimati", Locale: "fr-CA"})
	if n != 1 || again.Text != r.Text {
		t.Fatalf("the second request must come from the cache (%d compositions)", n)
	}
	// The cached reading expires at the end of its day in its zone:
	// 8 November 00:00 on Kiritimati is 7 November 10:00 UTC, 48h away.
	key := "horoscope:v2:" + horoscope.EngineVersion + ":aries:2026-11-07:fr:Pacific/Kiritimati"
	ttl, err := s.kv.TTL(ctx, key)
	if err != nil || ttl != 48*time.Hour {
		t.Fatalf("ttl %v %v", ttl, err)
	}
	// Different zones read different days.
	west, _ := s.HoroscopeFor(ctx, HoroscopeQuery{Sign: "aries", TZ: "Pacific/Pago_Pago"})
	if west.Date != "2026-11-04" || n != 2 {
		t.Fatalf("Pago Pago today %s (%d)", west.Date, n)
	}
	// Other dates are composed on demand and never stored, so requests
	// cannot grow the shared cache beyond yesterday, today and tomorrow.
	for i := 0; i < 2; i++ {
		if _, err := s.HoroscopeFor(ctx, HoroscopeQuery{Sign: "aries", Date: "2026-11-01"}); err != nil {
			t.Fatal(err)
		}
	}
	if stored, _ := s.kv.TTL(ctx, "horoscope:v2:"+horoscope.EngineVersion+":aries:2026-11-01:en:UTC"); n != 4 || stored > 0 {
		t.Fatalf("an older date must bypass the cache (%d compositions, ttl %v)", n, stored)
	}
	clk.Advance(time.Hour)
	if _, err := s.Aztro(ctx, HoroscopeQuery{Sign: "aries", Date: "2030-01-01"}); err != nil {
		t.Fatalf("aztro ignores date: %v", err)
	}
}

func TestAztroShape(t *testing.T) {
	s, _, _ := newAstro(t, nil)
	a, err := s.Aztro(context.Background(), HoroscopeQuery{Sign: "leo", Day: "yesterday", Locale: "es"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Description == "" || a.Compatibility == "" || a.LuckyNumber == "" || a.LuckyTime == "" || a.DateRange == "" ||
		a.Staryield.Sign != "Leo" || a.Staryield.Date != "2026-11-04" || a.Staryield.Locale != "es" {
		t.Fatalf("aztro %+v", a)
	}
}

func TestMyChartUsesStoredBirthData(t *testing.T) {
	lat, lon := 48.8566, 2.3522 // the original list's Paris
	s, _, _ := newAstro(t, &domain.Profile{BirthDate: date(1990, 5, 1), BirthTime: "14:30", BirthPlaceID: "paris-fr",
		BirthPlace: "Paris, France", BirthLatitude: &lat, BirthLongitude: &lon, BirthTimezone: "Europe/Paris", CountryCode: "FR"})
	v, err := s.MyChart(context.Background(), uuid.New(), "whole_sign")
	if err != nil {
		t.Fatal(err)
	}
	// The legacy slug resolves to its GeoNames place, and the stored
	// coordinates (within 0.01°) keep the gazetteer entry for display.
	if v.Place == nil || v.Place.ID != "gn:2988507" || v.Place.Admin1 == "" || !v.TimeKnown || v.HouseSystemUsed != astro.WholeSign {
		t.Fatalf("chart %+v place %+v", v.Chart, v.Place)
	}
	if len(v.Warnings) != 0 {
		t.Fatalf("warnings %v", v.Warnings)
	}
	if _, err := s.MyChart(context.Background(), uuid.New(), "koch"); fieldsOf(t, err)["house_system"] == nil {
		t.Fatal("house system must be validated")
	}

	// Stored coordinates that differ from the gazetteer are authoritative.
	far := 48.70
	s2, _, _ := newAstro(t, &domain.Profile{BirthDate: date(1990, 5, 1), BirthTime: "14:30", BirthPlaceID: "gn:2988507",
		BirthPlace: "Near Paris", BirthLatitude: &far, BirthLongitude: &lon, BirthTimezone: "Europe/Paris"})
	v2, err := s2.MyChart(context.Background(), uuid.New(), "")
	if err != nil || v2.Place.Latitude != far || v2.Place.Name != "Near Paris" {
		t.Fatalf("stored coordinates: %+v %v", v2.Place, err)
	}
}

func TestMyChartIgnoresTimeWithoutPlace(t *testing.T) {
	s, _, _ := newAstro(t, &domain.Profile{BirthDate: date(1990, 5, 1), BirthTime: "14:30"})
	v, err := s.MyChart(context.Background(), uuid.New(), "")
	if err != nil {
		t.Fatal(err)
	}
	if v.TimeKnown || v.Angles != nil || len(v.Warnings) != 1 || v.Warnings[0] != WarningBirthPlaceMissing {
		t.Fatalf("chart %+v warnings %v", v.Chart, v.Warnings)
	}
}

func TestMyEndpointsNeedABirthDate(t *testing.T) {
	s, _, _ := newAstro(t, &domain.Profile{})
	ctx := context.Background()
	id := uuid.New()
	_, err := s.MyChart(ctx, id, "")
	if fieldsOf(t, err)["birth_date"] != "Add your birth date in settings to see your chart." {
		t.Fatalf("chart: %v", err)
	}
	_, err = s.MyHoroscope(ctx, id, HoroscopeQuery{})
	if fieldsOf(t, err)["birth_date"] != "Add your birth date in settings to see your horoscope." {
		t.Fatalf("horoscope: %v", err)
	}
	_, err = s.MyNumerology(ctx, id, "")
	if fieldsOf(t, err)["birth_date"] != "Add your birth date in settings to see your numbers." {
		t.Fatalf("numerology: %v", err)
	}
	if _, err := s.MyNumerology(ctx, id, "Bad/Zone"); fieldsOf(t, err)["tz"] == nil {
		t.Fatal("tz must be validated first")
	}
	missing, _, _ := newAstro(t, nil)
	if _, err := missing.MyChart(ctx, id, ""); err == nil {
		t.Fatal("missing profile")
	} else if e, ok := apperror.As(err); !ok || e.Code != apperror.CodeNotFound {
		t.Fatalf("missing profile: %v", err)
	}
}

func TestMyHoroscopeUsesTheSunAtBirth(t *testing.T) {
	// 23:30 on 20 March 1990 in Paris (22:30 UTC) is after the Sun entered
	// Aries; a date table would say Pisces.
	lat, lon := 48.8534, 2.3488
	s, rec, _ := newAstro(t, &domain.Profile{BirthDate: date(1990, 3, 20), BirthTime: "23:30", BirthPlaceID: "gn:2988507",
		BirthLatitude: &lat, BirthLongitude: &lon, BirthTimezone: "Europe/Paris"})
	v, err := s.MyHoroscope(context.Background(), uuid.New(), HoroscopeQuery{TZ: "Europe/Paris", Locale: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if v.Sign != "Aries" || v.SunSignUncertain || v.Date != "2026-11-05" || len(v.Events.Lunar) == 0 {
		t.Fatalf("reading %+v", v)
	}
	for _, e := range rec.Events {
		for k := range e.Properties {
			if k == "sign" || k == "sun_sign" {
				t.Fatalf("the user's sign must not be tracked: %+v", e)
			}
		}
	}

	// Without a time the day straddles the ingress: the reading uses the
	// noon sign and says so.
	s2, _, _ := newAstro(t, &domain.Profile{BirthDate: date(1990, 3, 20)})
	v2, err := s2.MyHoroscope(context.Background(), uuid.New(), HoroscopeQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if v2.Sign != "Pisces" || !v2.SunSignUncertain || strings.Join(v2.SunSignAlternatives, ",") != "Pisces,Aries" {
		t.Fatalf("no time: %+v", v2)
	}
}

// The advisor's client card uses the same engine as the client's chart.
func TestClientAstroMatchesTheClientsChart(t *testing.T) {
	lat, lon := 48.8534, 2.3488
	full := clientAstro(&domain.Profile{BirthDate: date(1990, 3, 20), BirthTime: "23:30", BirthPlaceID: "gn:2988507",
		BirthLatitude: &lat, BirthLongitude: &lon, BirthTimezone: "Europe/Paris"})
	if full.SunSign != "Aries" || full.SunSignUncertain || !full.TimeKnown || full.RisingSign == "" || full.MoonSign == "" || full.LifePath != 6 {
		t.Fatalf("full data: %+v", full)
	}
	// A time without a place is never read as UTC.
	vague := clientAstro(&domain.Profile{BirthDate: date(1990, 3, 20), BirthTime: "23:30"})
	if vague.TimeKnown || vague.RisingSign != "" || !vague.SunSignUncertain || vague.SunSign != "Pisces" {
		t.Fatalf("no place: %+v", vague)
	}
	// A legacy slug alone still places the birth.
	legacy := clientAstro(&domain.Profile{BirthDate: date(1990, 3, 20), BirthTime: "23:30", BirthPlaceID: "paris-fr"})
	if legacy.SunSign != "Aries" || !legacy.TimeKnown || legacy.RisingSign != full.RisingSign {
		t.Fatalf("legacy slug: %+v", legacy)
	}
}

func TestMyNumerologyUsesRequesterZone(t *testing.T) {
	s, _, _ := newAstro(t, &domain.Profile{BirthDate: date(1990, 5, 1)})
	v, err := s.MyNumerology(context.Background(), uuid.New(), "Pacific/Kiritimati")
	if err != nil || v.AsOf != "2026-11-06" || v.LifePath != 7 || v.TZ != "Pacific/Kiritimati" {
		t.Fatalf("numerology %+v %v", v, err)
	}
}

func TestPlacesValidationAndLookup(t *testing.T) {
	s, _, _ := newAstro(t, nil)
	if _, err := s.Places("p", 8, ""); fieldsOf(t, err)["q"] != "Type at least 2 characters." {
		t.Fatalf("short query: %v", err)
	}
	if _, err := s.Places("paris", 8, "not-a-cursor"); fieldsOf(t, err)["cursor"] != "This page link has expired. Please search again." {
		t.Fatalf("bad cursor: %v", err)
	}
	page, err := s.Places("paris", 2, "")
	if err != nil || len(page.Items) != 2 || page.Items[0].ID != "gn:2988507" || page.NextCursor == "" || page.Attribution == "" {
		t.Fatalf("page %+v %v", page, err)
	}
	next, err := s.Places("paris", 2, page.NextCursor)
	if err != nil || len(next.Items) == 0 || next.Items[0].ID == page.Items[0].ID {
		t.Fatalf("next page %+v %v", next, err)
	}
	if p, err := s.Place("paris-fr"); err != nil || p.ID != "gn:2988507" {
		t.Fatalf("legacy lookup %+v %v", p, err)
	}
	if _, err := s.Place("gn:1"); err == nil {
		t.Fatal("unknown place")
	} else if e, ok := apperror.As(err); !ok || e.Code != apperror.CodeNotFound {
		t.Fatalf("unknown place: %v", err)
	}
	if info, ok := AstroPlaceLookup("london-gb"); !ok || info.ID != "gn:2643743" || info.Label != "London, England, United Kingdom" {
		t.Fatalf("lookup %+v", info)
	}
}

// Any calculator input either fails validation or yields a whole chart.
func FuzzNatalInput(f *testing.F) {
	s := NewAstrologyService(profileRepo{}, analytics.Noop{}, clock.NewFake(astroNow), nil)
	f.Add("1990-05-01", "14:30", "gn:2988507", "placidus", "Europe/Paris")
	f.Add("1900-01-01", "00:00", "paris-fr", "whole_sign", "")
	f.Add("2026-11-05", "23:59", "gn:5879400", "equal", "Pacific/Kiritimati")
	f.Add("1990-03-25", "02:30", "gn:2988507", "", "UTC")
	f.Add("1945-04-30", "7:05", "london-gb", "WHOLE-SIGN", "")
	f.Add("2000-02-30", "24:00", "gn:", "koch", "Mars/Base")
	f.Add("1969-12-20", "12:00", "gn:3133895", "", "") // Tromsø: Placidus falls back to Porphyry
	f.Fuzz(func(t *testing.T, date, clock, place, house, zone string) {
		out, err := s.Natal(context.Background(), BirthInput{BirthDate: date, BirthTime: clock, PlaceID: place, HouseSystem: house, TZ: zone})
		if err != nil {
			if e, ok := apperror.As(err); !ok || e.Code != apperror.CodeValidation {
				t.Fatalf("%q %q %q %q %q: %v", date, clock, place, house, zone, err)
			}
			return
		}
		c := out.Chart
		if c.TimeKnown != (strings.TrimSpace(clock) != "") || c.SunSign == "" || c.MoonSign == "" || len(c.Positions) < 10 {
			t.Fatalf("chart %+v", c.Chart)
		}
		if c.TimeKnown && (len(c.Houses) != 12 || c.Angles == nil || c.HouseSystemUsed == "") {
			t.Fatalf("timed chart without houses: %+v", c.Chart)
		}
		if out.Tarot == nil || len(out.Tarot.Cards) != 12 || out.Numerology == nil || out.Numerology.LifePath < 1 {
			t.Fatalf("includes %+v %+v", out.Tarot, out.Numerology)
		}
	})
}

func TestEventsAreMemoisedPerHour(t *testing.T) {
	s, _, clk := newAstro(t, nil)
	a := s.Events(4)
	if len(a.Lunar) != 4 {
		t.Fatalf("lunar %+v", a.Lunar)
	}
	if b := s.Events(4); &b.Lunar[0] != &a.Lunar[0] {
		t.Fatal("same hour must reuse the computation")
	}
	if c := s.Events(2); len(c.Lunar) != 2 {
		t.Fatal("count is part of the key")
	}
	clk.Advance(time.Hour)
	if d := s.Events(0); len(d.Lunar) != 4 {
		t.Fatal("count defaults to 4")
	}
}
