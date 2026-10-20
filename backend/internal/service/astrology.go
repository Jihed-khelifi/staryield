package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/astro"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// ChartView is a computed natal chart with numerology highlights.
type ChartView struct {
	astro.Chart
	BirthDate  string       `json:"birth_date"`
	BirthTime  string       `json:"birth_time,omitempty"`
	TimeKnown  bool         `json:"time_known"`
	Place      *astro.Place `json:"place,omitempty"`
	LifePath   int          `json:"life_path"`
	Disclaimer string       `json:"disclaimer"`
}

// CosmicEvents lists upcoming lunar phases and retrogrades.
type CosmicEvents struct {
	Lunar       []astro.LunarEvent `json:"lunar"`
	Retrogrades []astro.Period     `json:"retrogrades"`
}

const chartDisclaimer = "Offered for reflection and entertainment. Your choices remain your own."

// AstrologyService serves calculators (FR-01), the birth chart (FR-03) and
// the daily horoscope (FR-04).
type AstrologyService struct {
	users   repository.UserRepository
	tracker analytics.Tracker
	clock   clock.Clock
}

// NewAstrologyService wires the service.
func NewAstrologyService(users repository.UserRepository, tracker analytics.Tracker, clk clock.Clock) *AstrologyService {
	return &AstrologyService{users: users, tracker: tracker, clock: clk}
}

// AstroPlaceLookup adapts the gazetteer to PlaceLookup.
func AstroPlaceLookup(id string) (PlaceInfo, bool) {
	p, ok := astro.PlaceByID(id)
	if !ok {
		return PlaceInfo{}, false
	}
	return PlaceInfo{ID: p.ID, Name: p.Name, Country: p.Country, Latitude: p.Latitude, Longitude: p.Longitude, TimeZone: p.TimeZone}, true
}

// Places searches the birth-place gazetteer.
func (s *AstrologyService) Places(q string, limit int) []astro.Place {
	q = strings.TrimSpace(q)
	if len(q) > 80 {
		q = q[:80]
	}
	out := astro.SearchPlaces(q, limit)
	if out == nil {
		out = []astro.Place{}
	}
	return out
}

// ChartInput describes a birth moment.
type ChartInput struct {
	BirthDate string `json:"birth_date"`
	BirthTime string `json:"birth_time"`
	PlaceID   string `json:"place_id"`
}

// Chart computes a natal chart from birth data (anonymous calculator).
func (s *AstrologyService) Chart(ctx context.Context, in ChartInput) (*ChartView, error) {
	d, msg := ParseBirthDate(in.BirthDate, s.clock.Now(), 0)
	if msg != "" || d == nil {
		if msg == "" {
			msg = "Enter your birth date."
		}
		return nil, apperror.Validation(map[string]string{"birth_date": msg})
	}
	var place *astro.Place
	if strings.TrimSpace(in.PlaceID) != "" {
		p, ok := astro.PlaceByID(strings.TrimSpace(in.PlaceID))
		if !ok {
			return nil, apperror.Validation(map[string]string{"place_id": "Choose a birth place from the list."})
		}
		place = &p
	}
	view, err := s.compute(*d, strings.TrimSpace(in.BirthTime), place)
	if err != nil {
		return nil, err
	}
	s.tracker.Track(ctx, "astro.chart_computed", map[string]any{"has_time": view.TimeKnown, "has_place": place != nil, "sun_sign": view.SunSign})
	return view, nil
}

func (s *AstrologyService) compute(date time.Time, clockStr string, place *astro.Place) (*ChartView, error) {
	tz := "UTC"
	if place != nil {
		tz = place.TimeZone
	}
	instant, err := astro.LocalBirthInstant(date, clockStr, tz)
	if err != nil {
		return nil, apperror.Validation(map[string]string{"birth_time": "Use the 24-hour format HH:MM."})
	}
	var loc *astro.GeoLocation
	// Houses and the Ascendant need both an exact time and a place.
	if place != nil && clockStr != "" {
		l := place.Location()
		loc = &l
	}
	chart := astro.ComputeChart(instant, loc)
	return &ChartView{Chart: chart, BirthDate: date.Format("2006-01-02"), BirthTime: clockStr, TimeKnown: clockStr != "",
		Place: place, LifePath: astro.LifePath(date.Year(), int(date.Month()), date.Day()), Disclaimer: chartDisclaimer}, nil
}

// MyChart computes the signed-in client's full birth chart (FR-03).
func (s *AstrologyService) MyChart(ctx context.Context, userID uuid.UUID) (*ChartView, error) {
	p, err := s.users.GetProfile(ctx, userID)
	if err != nil {
		return nil, mapRepoErr(err, "Profile")
	}
	if p.BirthDate == nil {
		return nil, apperror.Validation(map[string]string{"birth_date": "Add your birth date in settings to see your chart."})
	}
	var place *astro.Place
	if p.BirthPlaceID != "" {
		if pl, ok := astro.PlaceByID(p.BirthPlaceID); ok {
			place = &pl
		}
	}
	view, err := s.compute(*p.BirthDate, p.BirthTime, place)
	if err != nil {
		return nil, err
	}
	s.tracker.Track(ctx, "astro.chart_viewed", map[string]any{"has_time": view.TimeKnown, "has_place": place != nil})
	return view, nil
}

// HoroscopeFor returns a sign's daily horoscope for date.
func (s *AstrologyService) HoroscopeFor(ctx context.Context, sign string, dateRaw string) (*astro.DailyHoroscope, error) {
	date := s.clock.Now()
	if dateRaw != "" {
		d, err := time.Parse("2006-01-02", dateRaw)
		if err != nil {
			return nil, apperror.Validation(map[string]string{"date": "Use YYYY-MM-DD."})
		}
		if d.Sub(date) > 7*24*time.Hour || date.Sub(d) > 366*24*time.Hour {
			return nil, apperror.Validation(map[string]string{"date": "Choose a date within the last year or the coming week."})
		}
		date = d
	}
	h, err := astro.Daily(sign, date)
	if err != nil {
		return nil, apperror.Validation(map[string]string{"sign": "Unknown zodiac sign."})
	}
	return &h, nil
}

// MyHoroscope returns the daily horoscope for the user's sun sign (FR-04).
func (s *AstrologyService) MyHoroscope(ctx context.Context, userID uuid.UUID, date string) (*astro.DailyHoroscope, error) {
	p, err := s.users.GetProfile(ctx, userID)
	if err != nil {
		return nil, mapRepoErr(err, "Profile")
	}
	if p.BirthDate == nil {
		return nil, apperror.Validation(map[string]string{"birth_date": "Add your birth date in settings to see your horoscope."})
	}
	sign := astro.SunSignForDate(int(p.BirthDate.Month()), p.BirthDate.Day())
	h, err := s.HoroscopeFor(ctx, sign, date)
	if err == nil {
		s.tracker.Track(ctx, "astro.horoscope_viewed", map[string]any{"sign": sign})
	}
	return h, err
}

// Events returns upcoming lunar phases and retrograde periods.
func (s *AstrologyService) Events(count int) CosmicEvents {
	if count <= 0 || count > 12 {
		count = 4
	}
	now := s.clock.Now()
	out := CosmicEvents{Lunar: astro.NextLunarEvents(now, count), Retrogrades: []astro.Period{}}
	for _, b := range []astro.Body{astro.Mercury, astro.Venus, astro.Mars} {
		out.Retrogrades = append(out.Retrogrades, astro.RetrogradePeriods(b, now, now.AddDate(0, 4, 0))...)
	}
	return out
}
