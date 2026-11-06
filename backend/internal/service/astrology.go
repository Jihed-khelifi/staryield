package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/astro"
	"github.com/Jihed-khelifi/staryield/backend/internal/astro/geo"
	"github.com/Jihed-khelifi/staryield/backend/internal/astro/horoscope"
	"github.com/Jihed-khelifi/staryield/backend/internal/astro/tz"
	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

const chartDisclaimer = "Offered for reflection and entertainment. Your choices remain your own."

// astroEngineVersion identifies the calculation rules (ephemeris, houses,
// aspects, spread, numerology).
const astroEngineVersion = "staryield-astro-2"

// EngineInfo identifies the data behind a calculation.
type EngineInfo struct {
	Version   string `json:"version"`
	TZData    string `json:"tzdata"`
	Gazetteer string `json:"gazetteer"`
	Horoscope string `json:"horoscope,omitempty"`
}

func engineInfo() EngineInfo {
	return EngineInfo{Version: astroEngineVersion, TZData: tz.Version(), Gazetteer: geo.Version()}
}

// Warnings a chart can carry.
const (
	// WarningBirthPlaceMissing: a birth time is stored without a birthplace,
	// so the time cannot be placed in a time zone and is ignored.
	WarningBirthPlaceMissing = "birth_place_missing"
	// WarningPlutoOutOfRange: Pluto's theory does not cover the date.
	WarningPlutoOutOfRange = "pluto_out_of_range"
)

// ChartView is a computed natal chart with the birth data it came from.
type ChartView struct {
	astro.Chart
	BirthDate      string       `json:"birth_date"`
	BirthTime      string       `json:"birth_time,omitempty"`
	WallTimeStatus string       `json:"wall_time_status,omitempty"`
	Place          *astro.Place `json:"place,omitempty"`
	LifePath       int          `json:"life_path"`
	Warnings       []string     `json:"warnings,omitempty"`
	Engine         EngineInfo   `json:"engine"`
	Disclaimer     string       `json:"disclaimer"`
	Attribution    string       `json:"attribution,omitempty"`
}

// NumerologyView holds the numerology numbers for a birth date.
type NumerologyView struct {
	LifePath          int             `json:"life_path"`
	LifePathMaster    bool            `json:"life_path_master"`
	LifePathReduction astro.Reduction `json:"life_path_reduction"`
	PersonalYear      int             `json:"personal_year"`
	PersonalMonth     int             `json:"personal_month"`
	PersonalDay       int             `json:"personal_day"`
	AsOf              string          `json:"as_of"`
	TZ                string          `json:"tz"`
}

// NatalView is the anonymous natal calculator's response.
type NatalView struct {
	Chart       ChartView          `json:"chart"`
	Tarot       *astro.TarotSpread `json:"tarot,omitempty"`
	Numerology  *NumerologyView    `json:"numerology,omitempty"`
	Engine      EngineInfo         `json:"engine"`
	Disclaimer  string             `json:"disclaimer"`
	Attribution string             `json:"attribution,omitempty"`
}

// SunSignView answers the sun-sign calculator.
type SunSignView struct {
	SunSign      string   `json:"sun_sign"`
	Uncertain    bool     `json:"uncertain"`
	Alternatives []string `json:"alternatives,omitempty"`
	Element      string   `json:"element"`
	Modality     string   `json:"modality"`
}

// CosmicEvents lists upcoming lunar phases and retrogrades.
type CosmicEvents struct {
	Lunar       []astro.LunarEvent `json:"lunar"`
	Retrogrades []astro.Period     `json:"retrogrades"`
}

// AstrologyService serves the calculators (FR-01), the birth chart (FR-03)
// and the daily horoscope (FR-04). Anonymous calculations are stateless:
// birth data is never stored, cached or logged, and analytics only record
// which inputs were present.
type AstrologyService struct {
	users     repository.UserRepository
	tracker   analytics.Tracker
	clock     clock.Clock
	kv        cache.Store
	horoscope horoscope.Provider

	eventsMu  sync.Mutex
	eventsKey string
	events    CosmicEvents
}

// NewAstrologyService wires the service. kv may be nil (no shared cache).
func NewAstrologyService(users repository.UserRepository, tracker analytics.Tracker, clk clock.Clock, kv cache.Store) *AstrologyService {
	go func() { _ = geo.Warm() }() // index the gazetteer before the first search
	return &AstrologyService{users: users, tracker: tracker, clock: clk, kv: kv, horoscope: horoscope.Engine{}}
}

// AstroPlaceLookup adapts the gazetteer to PlaceLookup. Legacy slugs
// ("paris-fr") resolve to their GeoNames place.
func AstroPlaceLookup(id string) (PlaceInfo, bool) {
	p, ok := astro.PlaceByID(id)
	if !ok {
		return PlaceInfo{}, false
	}
	return PlaceInfo{ID: p.ID, Name: p.Name, Admin1: p.Admin1, Label: p.Label(), Country: p.Country, CountryCode: p.CountryCode,
		Latitude: p.Latitude, Longitude: p.Longitude, TimeZone: p.TimeZone}, true
}

// --- places ------------------------------------------------------------------

// PlacesPage is a page of gazetteer results.
type PlacesPage struct {
	Items       []astro.Place `json:"items"`
	NextCursor  string        `json:"next_cursor,omitempty"`
	Attribution string        `json:"attribution"`
}

// Places searches the birthplace gazetteer.
func (s *AstrologyService) Places(q string, limit int, cursor string) (*PlacesPage, error) {
	page, err := geo.Search(q, geo.SearchOptions{Limit: limit, Cursor: cursor})
	switch {
	case errors.Is(err, geo.ErrQueryTooShort):
		return nil, apperror.Validation(map[string]string{"q": "Type at least 2 characters."})
	case errors.Is(err, geo.ErrBadCursor):
		return nil, apperror.Validation(map[string]string{"cursor": "This page link has expired. Please search again."})
	case err != nil:
		return nil, apperror.Internal(err)
	}
	return &PlacesPage{Items: page.Items, NextCursor: page.NextCursor, Attribution: geo.Attribution}, nil
}

// Place returns one gazetteer entry.
func (s *AstrologyService) Place(id string) (*astro.Place, error) {
	p, ok := astro.PlaceByID(id)
	if !ok {
		return nil, apperror.NotFound("Place")
	}
	return &p, nil
}

// PlacesVersion identifies the gazetteer data for HTTP caching.
func (s *AstrologyService) PlacesVersion() string { return geo.Version() }

// --- birth data --------------------------------------------------------------

// BirthInput describes a birth moment for the calculators.
type BirthInput struct {
	BirthDate   string   `json:"birth_date"`
	BirthTime   string   `json:"birth_time"`
	PlaceID     string   `json:"place_id"`
	HouseSystem string   `json:"house_system"`
	Include     []string `json:"include"`
	TZ          string   `json:"tz"` // requester's zone, for "today" in numerology
}

// birth is validated birth data.
type birth struct {
	date        time.Time // calendar day (UTC midnight)
	clock       string    // "HH:MM" or ""
	hour, min   int
	place       *astro.Place
	loc         *time.Location
	houseSystem astro.HouseSystem
}

// Calculator includes.
const (
	includeAspects    = "aspects"
	includeTarot      = "tarot"
	includeNumerology = "numerology"
)

func (s *AstrologyService) parseBirth(dateRaw, clockRaw, placeID, houseRaw string, requireDate bool) (*birth, map[string]string) {
	fields := map[string]string{}
	b := &birth{}
	d, msg := ParseBirthDate(dateRaw, s.clock.Now(), 0)
	switch {
	case msg != "":
		fields["birth_date"] = msg
	case d == nil && requireDate:
		fields["birth_date"] = "Enter your birth date."
	case d != nil:
		b.date = *d
	}
	if c := strings.TrimSpace(clockRaw); c != "" {
		h, m, err := tz.ParseClock(c)
		if err != nil {
			fields["birth_time"] = "Use the 24-hour format HH:MM."
		} else {
			b.clock, b.hour, b.min = c, h, m
		}
	}
	if id := strings.TrimSpace(placeID); id != "" {
		p, ok := astro.PlaceByID(id)
		if !ok {
			fields["place_id"] = "Choose a birth place from the list."
		} else if loc, err := tz.Load(p.TimeZone); err != nil {
			fields["place_id"] = "Choose a birth place from the list."
		} else {
			b.place, b.loc = &p, loc
		}
	}
	if b.clock != "" && b.place == nil && fields["place_id"] == "" {
		fields["place_id"] = "Choose your birth place to use a birth time."
	}
	sys, ok := astro.ParseHouseSystem(houseRaw)
	if !ok {
		fields["house_system"] = "Choose Placidus, Whole Sign or Equal houses."
	}
	b.houseSystem = sys
	return b, fields
}

// moment resolves the birth data to the chart request: the exact instant
// when time and place are known, otherwise the representative instant and
// the window of possible instants.
func (b *birth) moment() (astro.ChartRequest, tz.WallStatus) {
	req := astro.ChartRequest{System: b.houseSystem}
	var loc *astro.GeoLocation
	if b.place != nil {
		l := astro.LocationOf(*b.place)
		loc = &l
	}
	req.Loc = loc
	if b.clock != "" && b.place != nil {
		instant, status := tz.Resolve(b.date, b.hour, b.min, b.loc, b.place.Longitude)
		req.Instant, req.TimeKnown = instant, true
		return req, status
	}
	at, start, end := astro.BirthWindow(b.date, 0, 0, false, b.loc)
	req.Instant, req.WindowStart, req.WindowEnd = at, start, end
	return req, ""
}

func (s *AstrologyService) chartView(b *birth) (*ChartView, *astro.ChartRequest, error) {
	req, status := b.moment()
	c, err := astro.ComputeChart(req)
	if err != nil {
		return nil, nil, apperror.Validation(map[string]string{"birth_date": "Choose a date between 1900 and 2100."})
	}
	v := &ChartView{
		Chart: c, BirthDate: b.date.Format(time.DateOnly), BirthTime: b.clock, WallTimeStatus: string(status),
		Place: b.place, LifePath: astro.LifePath(b.date.Year(), int(b.date.Month()), b.date.Day()),
		Engine: engineInfo(), Disclaimer: chartDisclaimer,
	}
	if b.place != nil {
		v.Attribution = geo.Attribution
	}
	if _, ok := c.PositionOf(astro.Pluto); !ok {
		v.Warnings = append(v.Warnings, WarningPlutoOutOfRange)
	}
	return v, &req, nil
}

func parseIncludes(raw []string) (map[string]bool, string) {
	if len(raw) == 0 {
		return map[string]bool{includeAspects: true, includeTarot: true, includeNumerology: true}, ""
	}
	if len(raw) > 3 {
		return nil, "Choose from aspects, tarot and numerology."
	}
	out := map[string]bool{}
	for _, v := range raw {
		switch v = strings.ToLower(strings.TrimSpace(v)); v {
		case includeAspects, includeTarot, includeNumerology:
			out[v] = true
		default:
			return nil, "Choose from aspects, tarot and numerology."
		}
	}
	return out, ""
}

// requesterZone returns the zone named by the request (UTC when none is
// named), and false with UTC when the name is not a known zone.
func requesterZone(name string) (*time.Location, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return time.UTC, true
	}
	loc, err := tz.Load(name)
	if err != nil {
		return time.UTC, false
	}
	return loc, true
}

// --- calculators -------------------------------------------------------------

// Natal computes the anonymous natal chart, Twelve Houses spread and
// numerology (FR-01). Nothing is stored.
func (s *AstrologyService) Natal(ctx context.Context, in BirthInput) (*NatalView, error) {
	b, fields := s.parseBirth(in.BirthDate, in.BirthTime, in.PlaceID, in.HouseSystem, true)
	includes, msg := parseIncludes(in.Include)
	if msg != "" {
		fields["include"] = msg
	}
	reqLoc, ok := requesterZone(in.TZ)
	if !ok {
		fields["tz"] = "Choose a valid time zone."
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	chart, req, err := s.chartView(b)
	if err != nil {
		return nil, err
	}
	out := &NatalView{Engine: chart.Engine, Disclaimer: chartDisclaimer, Attribution: chart.Attribution}
	if includes[includeTarot] {
		var alts []astro.Chart
		if !req.TimeKnown {
			for _, at := range []time.Time{req.WindowStart, req.WindowEnd} {
				alt := *req
				alt.Instant, alt.WindowStart, alt.WindowEnd = at, time.Time{}, time.Time{}
				if c, err := astro.ComputeChart(alt); err == nil {
					alts = append(alts, c)
				}
			}
		}
		spread := astro.TwelveHouses(chart.Chart, alts...)
		out.Tarot = &spread
	}
	if includes[includeNumerology] {
		n := numerologyFor(b.date, s.clock.Now().In(reqLoc))
		out.Numerology = &n
	}
	if !includes[includeAspects] {
		chart.Aspects = []astro.Aspect{}
	}
	out.Chart = *chart
	system := string(b.houseSystem)
	s.tracker.Track(ctx, "astro.natal_computed", map[string]any{"has_time": chart.TimeKnown, "has_place": b.place != nil, "house_system": system})
	return out, nil
}

// Chart computes a natal chart (the original calculator endpoint, kept for
// existing clients).
func (s *AstrologyService) Chart(ctx context.Context, in BirthInput) (*ChartView, error) {
	b, fields := s.parseBirth(in.BirthDate, in.BirthTime, in.PlaceID, in.HouseSystem, true)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	view, _, err := s.chartView(b)
	if err != nil {
		return nil, err
	}
	s.tracker.Track(ctx, "astro.chart_computed", map[string]any{"has_time": view.TimeKnown, "has_place": b.place != nil})
	return view, nil
}

// NumerologyInput asks for numerology numbers.
type NumerologyInput struct {
	BirthDate string `json:"birth_date"`
	AsOf      string `json:"as_of"`
	TZ        string `json:"tz"`
}

func numerologyFor(birthDate, asOf time.Time) NumerologyView {
	r := astro.LifePathReduction(birthDate.Year(), int(birthDate.Month()), birthDate.Day())
	m, d := int(birthDate.Month()), birthDate.Day()
	return NumerologyView{
		LifePath: r.Number, LifePathMaster: r.Master, LifePathReduction: r,
		PersonalYear: astro.PersonalYear(m, d, asOf), PersonalMonth: astro.PersonalMonth(m, d, asOf), PersonalDay: astro.PersonalDay(m, d, asOf),
		AsOf: asOf.Format(time.DateOnly), TZ: asOf.Location().String(),
	}
}

// Numerology computes the life path and personal cycles (the path-of-life
// calculator). "Today" is the calendar date in the requester's time zone.
func (s *AstrologyService) Numerology(ctx context.Context, in NumerologyInput) (*NumerologyView, error) {
	fields := map[string]string{}
	d, msg := ParseBirthDate(in.BirthDate, s.clock.Now(), 0)
	if msg != "" || d == nil {
		if msg == "" {
			msg = "Enter your birth date."
		}
		fields["birth_date"] = msg
	}
	loc, ok := requesterZone(in.TZ)
	if !ok {
		fields["tz"] = "Choose a valid time zone."
	}
	asOf := s.clock.Now().In(loc)
	if raw := strings.TrimSpace(in.AsOf); raw != "" {
		a, err := time.ParseInLocation(time.DateOnly, raw, loc)
		if err != nil || a.Year() < 1900 || a.Year() > 2100 {
			fields["as_of"] = "Use a date between 1900 and 2100 (YYYY-MM-DD)."
		} else {
			asOf = a.Add(12 * time.Hour)
		}
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	n := numerologyFor(*d, asOf)
	s.tracker.Track(ctx, "astro.numerology_computed", map[string]any{"life_path_master": n.LifePathMaster})
	return &n, nil
}

// SunSign answers the canonical sun sign for a birth.
func (s *AstrologyService) SunSign(ctx context.Context, in BirthInput) (*SunSignView, error) {
	b, fields := s.parseBirth(in.BirthDate, in.BirthTime, in.PlaceID, "", true)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	at, start, end := astro.BirthWindow(b.date, b.hour, b.min, b.clock != "", b.loc)
	if b.clock != "" && b.place != nil {
		at, _ = tz.Resolve(b.date, b.hour, b.min, b.loc, b.place.Longitude)
		start, end = at, at
	}
	sun := astro.SunSignForBirth(at, start, end)
	s.tracker.Track(ctx, "astro.sun_sign_computed", map[string]any{"has_time": b.clock != "", "has_place": b.place != nil, "uncertain": sun.Uncertain})
	return &SunSignView{SunSign: sun.Sign, Uncertain: sun.Uncertain, Alternatives: sun.Alternatives,
		Element: astro.ElementOf(sun.Sign), Modality: astro.ModalityOf(sun.Sign)}, nil
}

// --- signed-in client ----------------------------------------------------------

// profileBirth reads the stored birth data. Stored coordinates and zone are
// preferred; otherwise the stored place ID (GeoNames or legacy slug) is
// resolved. A stored time without any place cannot be placed in a time
// zone, so it is ignored and reported.
func (s *AstrologyService) profileBirth(ctx context.Context, userID uuid.UUID, houseRaw string, missingMsg string) (*birth, []string, error) {
	p, err := s.users.GetProfile(ctx, userID)
	if err != nil {
		return nil, nil, mapRepoErr(err, "Profile")
	}
	if p.BirthDate == nil {
		return nil, nil, apperror.Validation(map[string]string{"birth_date": missingMsg})
	}
	sys, ok := astro.ParseHouseSystem(houseRaw)
	if !ok {
		return nil, nil, apperror.Validation(map[string]string{"house_system": "Choose Placidus, Whole Sign or Equal houses."})
	}
	d := *p.BirthDate
	b := &birth{date: time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC), houseSystem: sys}
	var warnings []string
	if p.BirthPlaceID != "" {
		if pl, ok := astro.PlaceByID(p.BirthPlaceID); ok {
			b.place = &pl
		}
	}
	if p.BirthLatitude != nil && p.BirthLongitude != nil && p.BirthTimezone != "" {
		// The stored coordinates are authoritative (they are what the user
		// confirmed); keep the gazetteer entry for display when it matches.
		stored := astro.Place{ID: p.BirthPlaceID, Name: p.BirthPlace, Latitude: *p.BirthLatitude, Longitude: *p.BirthLongitude,
			TimeZone: p.BirthTimezone, CountryCode: p.CountryCode}
		if b.place != nil && math.Abs(b.place.Latitude-stored.Latitude) < 0.01 && math.Abs(b.place.Longitude-stored.Longitude) < 0.01 {
			stored = *b.place
		}
		b.place = &stored
	}
	if b.place != nil {
		loc, err := tz.Load(b.place.TimeZone)
		if err != nil {
			b.place = nil
		} else {
			b.loc = loc
		}
	}
	if c := strings.TrimSpace(p.BirthTime); c != "" {
		if h, m, err := tz.ParseClock(c); err == nil {
			if b.place != nil {
				b.clock, b.hour, b.min = c, h, m
			} else {
				warnings = append(warnings, WarningBirthPlaceMissing)
			}
		}
	}
	return b, warnings, nil
}

// MyChart computes the signed-in client's birth chart (FR-03).
func (s *AstrologyService) MyChart(ctx context.Context, userID uuid.UUID, houseSystem string) (*ChartView, error) {
	b, warnings, err := s.profileBirth(ctx, userID, houseSystem, "Add your birth date in settings to see your chart.")
	if err != nil {
		return nil, err
	}
	view, _, err := s.chartView(b)
	if err != nil {
		return nil, err
	}
	view.Warnings = append(view.Warnings, warnings...)
	s.tracker.Track(ctx, "astro.chart_viewed", map[string]any{"has_time": view.TimeKnown, "has_place": b.place != nil})
	return view, nil
}

// MyNumerology returns the signed-in client's numbers, "today" being the
// date in tzName (the requester's zone).
func (s *AstrologyService) MyNumerology(ctx context.Context, userID uuid.UUID, tzName string) (*NumerologyView, error) {
	loc, ok := requesterZone(tzName)
	if !ok {
		return nil, apperror.Validation(map[string]string{"tz": "Choose a valid time zone."})
	}
	b, _, err := s.profileBirth(ctx, userID, "", "Add your birth date in settings to see your numbers.")
	if err != nil {
		return nil, err
	}
	n := numerologyFor(b.date, s.clock.Now().In(loc))
	return &n, nil
}

// --- horoscopes ----------------------------------------------------------------

// HoroscopeQuery selects a daily reading.
type HoroscopeQuery struct {
	Sign   string
	Day    string // yesterday | today | tomorrow (default today)
	Date   string // YYYY-MM-DD, overrides Day
	TZ     string // requester's IANA zone (default UTC)
	Locale string // en | fr | es
}

// horoscopeDate resolves the calendar day in the requester's zone.
func (s *AstrologyService) horoscopeDate(q HoroscopeQuery) (time.Time, *time.Location, map[string]string) {
	fields := map[string]string{}
	loc, ok := requesterZone(q.TZ)
	if !ok {
		fields["tz"] = "Choose a valid time zone."
		loc = time.UTC
	}
	now := s.clock.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	date := today
	if raw := strings.TrimSpace(q.Date); raw != "" {
		d, err := time.ParseInLocation(time.DateOnly, raw, loc)
		switch {
		case err != nil:
			fields["date"] = "Use YYYY-MM-DD."
		case d.After(today.AddDate(0, 0, 7)) || d.Before(today.AddDate(-1, 0, 0)):
			fields["date"] = "Choose a date within the last year or the coming week."
		default:
			date = d
		}
	} else if off, ok := horoscope.ParseDay(q.Day); ok {
		date = today.AddDate(0, 0, off)
	} else {
		fields["day"] = "Choose yesterday, today or tomorrow."
	}
	if _, ok := astro.CanonicalSign(q.Sign); !ok {
		fields["sign"] = "Unknown zodiac sign."
	}
	return date, loc, fields
}

// ttlUntilEndOfDay is how long a reading for date stays current in loc.
func (s *AstrologyService) ttlUntilEndOfDay(date time.Time, loc *time.Location) time.Duration {
	end := time.Date(date.Year(), date.Month(), date.Day()+1, 0, 0, 0, 0, loc)
	return min(max(end.Sub(s.clock.Now()), time.Minute), 3*24*time.Hour)
}

// cacheable reports whether date is yesterday, today or tomorrow in loc.
// Only those readings are shared through the cache, which bounds its keys
// (sign × 3 days × locale × zone) however other dates are requested.
func (s *AstrologyService) cacheable(date time.Time, loc *time.Location) bool {
	now := s.clock.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	return !date.Before(today.AddDate(0, 0, -1)) && !date.After(today.AddDate(0, 0, 1))
}

// reading returns a cached reading or composes it.
func (s *AstrologyService) reading(ctx context.Context, sign string, date time.Time, loc *time.Location, locale string) (*horoscope.Reading, error) {
	canonical, _ := astro.CanonicalSign(sign)
	locale = horoscope.NormalizeLocale(locale)
	key := "horoscope:v2:" + horoscope.EngineVersion + ":" + strings.ToLower(canonical) + ":" + date.Format(time.DateOnly) + ":" + locale + ":" + loc.String()
	shared := s.kv != nil && s.cacheable(date, loc)
	if shared {
		if raw, ok, err := s.kv.Get(ctx, key); err == nil && ok {
			var r horoscope.Reading
			if json.Unmarshal([]byte(raw), &r) == nil {
				return &r, nil
			}
		}
	}
	r, err := s.horoscope.Daily(ctx, horoscope.Request{Sign: canonical, Date: date, Location: loc, Locale: locale})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if shared {
		if raw, err := json.Marshal(r); err == nil {
			_ = s.kv.Set(ctx, key, string(raw), s.ttlUntilEndOfDay(date, loc))
		}
	}
	return &r, nil
}

// HoroscopeFor returns a sign's daily reading (public).
func (s *AstrologyService) HoroscopeFor(ctx context.Context, q HoroscopeQuery) (*horoscope.Reading, error) {
	date, loc, fields := s.horoscopeDate(q)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	return s.reading(ctx, q.Sign, date, loc, q.Locale)
}

// Aztro returns a reading in the aztro API's shape (public).
func (s *AstrologyService) Aztro(ctx context.Context, q HoroscopeQuery) (*horoscope.Aztro, error) {
	q.Date = "" // aztro only knows yesterday, today and tomorrow
	date, loc, fields := s.horoscopeDate(q)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	r, err := s.reading(ctx, q.Sign, date, loc, q.Locale)
	if err != nil {
		return nil, err
	}
	a := horoscope.ToAztro(*r)
	return &a, nil
}

// MyHoroscopeView is the client's reading, how their sign was found and
// the upcoming sky events (Account Overview).
type MyHoroscopeView struct {
	horoscope.Reading
	SunSignUncertain    bool         `json:"sun_sign_uncertain"`
	SunSignAlternatives []string     `json:"sun_sign_alternatives,omitempty"`
	Events              CosmicEvents `json:"events"`
}

// MyHoroscope returns the daily horoscope for the user's sun sign (FR-04),
// derived from the actual solar longitude at birth (the same sign their
// chart shows).
func (s *AstrologyService) MyHoroscope(ctx context.Context, userID uuid.UUID, q HoroscopeQuery) (*MyHoroscopeView, error) {
	b, _, err := s.profileBirth(ctx, userID, "", "Add your birth date in settings to see your horoscope.")
	if err != nil {
		return nil, err
	}
	at, start, end := astro.BirthWindow(b.date, b.hour, b.min, b.clock != "", b.loc)
	if b.clock != "" && b.place != nil {
		at, _ = tz.Resolve(b.date, b.hour, b.min, b.loc, b.place.Longitude)
		start, end = at, at
	}
	sun := astro.SunSignForBirth(at, start, end)
	q.Sign = sun.Sign
	r, err := s.HoroscopeFor(ctx, q)
	if err != nil {
		return nil, err
	}
	s.tracker.Track(ctx, "astro.horoscope_viewed", map[string]any{"locale": r.Locale})
	return &MyHoroscopeView{Reading: *r, SunSignUncertain: sun.Uncertain, SunSignAlternatives: sun.Alternatives, Events: s.Events(4)}, nil
}

// LocaleFrom returns the request's negotiated locale.
func LocaleFrom(ctx context.Context) string { return reqctx.MetaFrom(ctx).Locale }

// truncateRunes shortens s to at most n characters without splitting one.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// --- events --------------------------------------------------------------------

// Events returns upcoming lunar phases and retrograde periods. Results are
// memoised for the current UTC hour.
func (s *AstrologyService) Events(count int) CosmicEvents {
	if count <= 0 || count > 12 {
		count = 4
	}
	now := s.clock.Now().UTC()
	key := now.Truncate(time.Hour).Format(time.RFC3339) + "|" + strconv.Itoa(count)
	s.eventsMu.Lock()
	defer s.eventsMu.Unlock()
	if s.eventsKey == key {
		return s.events
	}
	out := CosmicEvents{Lunar: astro.NextLunarEvents(now, count), Retrogrades: []astro.Period{}}
	for _, b := range []astro.Body{astro.Mercury, astro.Venus, astro.Mars} {
		out.Retrogrades = append(out.Retrogrades, astro.RetrogradePeriods(b, now, now.AddDate(0, 4, 0))...)
	}
	s.eventsKey, s.events = key, out
	return out
}
