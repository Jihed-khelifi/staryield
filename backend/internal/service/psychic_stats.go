package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// statsCacheTTL bounds how stale the advisor overview can be.
const statsCacheTTL = time.Minute

// statsMaxWindow mirrors the analytics reporting cap.
const statsMaxWindow = 400 * 24 * time.Hour

// StatsRatio is a KPI share. Pct is null when nobody qualified, so the UI
// can say "no data yet" instead of a misleading 0 %.
type StatsRatio struct {
	Pct         *float64 `json:"pct"`
	Numerator   int64    `json:"numerator"`
	Denominator int64    `json:"denominator"`
}

// StatsRepeat is the median gap between paid sessions of returning clients.
type StatsRepeat struct {
	MedianDays *float64 `json:"median_days"`
	Sample     int64    `json:"sample"`
}

// StatsActiveHours is availability (online or busy) and time in sessions.
type StatsActiveHours struct {
	Hours          float64    `json:"hours"`
	InSessionHours float64    `json:"in_session_hours"`
	TrackedSince   *time.Time `json:"tracked_since,omitempty"`
}

// StatsKPIs are the "Advisor stats overview" cards.
type StatsKPIs struct {
	FreeToPaid         StatsRatio       `json:"free_to_paid"`
	PaidToRetained     StatsRatio       `json:"paid_to_retained"`
	PaidMinutes        int64            `json:"paid_minutes"`
	RetainedRepeatDays StatsRepeat      `json:"retained_repeat_days"`
	ActiveHours        StatsActiveHours `json:"active_hours"`
}

// StatsRangeView echoes the resolved reporting window.
type StatsRangeView struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
	TZ   string    `json:"tz"`
}

// StatsOverview is the advisor overview: the current window and the window
// of the same length just before it (for trend arrows).
type StatsOverview struct {
	Range    StatsRangeView `json:"range"`
	KPIs     StatsKPIs      `json:"kpis"`
	Previous StatsKPIs      `json:"previous"`
}

// PsychicStatsService computes the advisor overview KPIs.
type PsychicStatsService struct {
	repo  repository.StatsRepository
	cache cache.Store
	clock clock.Clock
}

// NewPsychicStatsService wires the service; cache may be nil.
func NewPsychicStatsService(repo repository.StatsRepository, store cache.Store, clk clock.Clock) *PsychicStatsService {
	return &PsychicStatsService{repo: repo, cache: store, clock: clk}
}

// statsRange resolves from/to. Dates (YYYY-MM-DD) are calendar days in loc,
// and "to" is inclusive. The default is the 30 days up to the end of the
// current minute, which keeps the cache key stable for a minute.
func statsRange(fromRaw, toRaw string, loc *time.Location, now time.Time) (Range, error) {
	parse := func(s string, endOfDay bool) (time.Time, error) {
		s = strings.TrimSpace(s)
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t.UTC(), nil
		}
		d, err := time.ParseInLocation("2006-01-02", s, loc)
		if err != nil {
			return time.Time{}, err
		}
		if endOfDay {
			d = d.AddDate(0, 0, 1)
		}
		return d.UTC(), nil
	}
	r := Range{To: now.UTC().Truncate(time.Minute).Add(time.Minute)}
	r.From = r.To.Add(-30 * 24 * time.Hour)
	if toRaw != "" {
		t, err := parse(toRaw, true)
		if err != nil {
			return r, apperror.Validation(map[string]string{"to": "Use RFC 3339 or YYYY-MM-DD."})
		}
		r.To = t
		if fromRaw == "" {
			r.From = r.To.Add(-30 * 24 * time.Hour)
		}
	}
	if fromRaw != "" {
		t, err := parse(fromRaw, false)
		if err != nil {
			return r, apperror.Validation(map[string]string{"from": "Use RFC 3339 or YYYY-MM-DD."})
		}
		r.From = t
	}
	if !r.From.Before(r.To) {
		return r, apperror.Validation(map[string]string{"from": "from must be before to."})
	}
	if r.To.Sub(r.From) > statsMaxWindow {
		return r, apperror.Validation(map[string]string{"from": "The reporting window is limited to 400 days."})
	}
	return r, nil
}

func ratio(r repository.Ratio) StatsRatio {
	out := StatsRatio{Numerator: r.Numerator, Denominator: r.Denominator}
	if r.Denominator > 0 {
		pct := math.Round(float64(r.Numerator)*1000/float64(r.Denominator)) / 10
		out.Pct = &pct
	}
	return out
}

func roundTo(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}

func (s *PsychicStatsService) kpis(ctx context.Context, psychicID uuid.UUID, from, to, now time.Time) (StatsKPIs, error) {
	var k StatsKPIs
	f2p, err := s.repo.FreeToPaid(ctx, psychicID, from, to)
	if err != nil {
		return k, err
	}
	p2r, err := s.repo.PaidToRetained(ctx, psychicID, from, to)
	if err != nil {
		return k, err
	}
	gap, err := s.repo.RepeatGap(ctx, psychicID, from, to)
	if err != nil {
		return k, err
	}
	minutes, err := s.repo.PaidMinutes(ctx, psychicID, from, to)
	if err != nil {
		return k, err
	}
	inSession, err := s.repo.InSessionSeconds(ctx, psychicID, from, to)
	if err != nil {
		return k, err
	}
	available, since, err := s.repo.AvailableSeconds(ctx, psychicID, from, to, now)
	if err != nil {
		return k, err
	}
	k.FreeToPaid, k.PaidToRetained, k.PaidMinutes = ratio(f2p), ratio(p2r), minutes
	k.RetainedRepeatDays.Sample = gap.Sample
	if gap.MedianDays.Valid {
		d := roundTo(gap.MedianDays.Float64, 1)
		k.RetainedRepeatDays.MedianDays = &d
	}
	k.ActiveHours = StatsActiveHours{Hours: roundTo(float64(available)/3600, 1), InSessionHours: roundTo(float64(inSession)/3600, 1), TrackedSince: since}
	return k, nil
}

// Overview returns the KPI cards for an advisor and the preceding period.
func (s *PsychicStatsService) Overview(ctx context.Context, psychicID uuid.UUID, fromRaw, toRaw, tz string) (*StatsOverview, error) {
	tzName, loc, err := ParseTimezone(tz)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	r, err := statsRange(fromRaw, toRaw, loc, now)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("stats:overview:%s:%d:%d", psychicID, r.From.Unix(), r.To.Unix())
	if s.cache != nil {
		if raw, ok, err := s.cache.Get(ctx, key); err == nil && ok {
			var out StatsOverview
			if json.Unmarshal([]byte(raw), &out) == nil {
				out.Range.TZ = tzName
				return &out, nil
			}
		}
	}
	cur, err := s.kpis(ctx, psychicID, r.From, r.To, now)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	span := r.To.Sub(r.From)
	prev, err := s.kpis(ctx, psychicID, r.From.Add(-span), r.From, now)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := &StatsOverview{Range: StatsRangeView{From: r.From, To: r.To, TZ: tzName}, KPIs: cur, Previous: prev}
	if s.cache != nil {
		if b, err := json.Marshal(out); err == nil {
			warnErr(ctx, "stats: cache overview", s.cache.Set(ctx, key, string(b), statsCacheTTL))
		}
	}
	return out, nil
}
