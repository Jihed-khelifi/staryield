package service

import (
	"context"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// PsychicAnalytics is an advisor's performance dashboard.
type PsychicAnalytics struct {
	Range    AnalyticsRange    `json:"range"`
	Earnings EarningsAnalytics `json:"earnings"`
	Payout   PayoutView        `json:"payout"`
	Sessions SessionAnalytics  `json:"sessions"`
	Clients  ClientAnalytics   `json:"clients"`
	Rating   RatingAnalytics   `json:"rating"`
	Score    SuccessScore      `json:"score"`
	Hours    WorkingHours      `json:"hours"`
}

// AnalyticsRange echoes the bucketing parameters.
type AnalyticsRange struct {
	From     time.Time `json:"from"`
	To       time.Time `json:"to"`
	TZ       string    `json:"tz"`
	Interval string    `json:"interval"`
}

// EarningsAnalytics summarises earnings in the range.
type EarningsAnalytics struct {
	GrossCredits      int64           `json:"gross_credits"`
	CommissionCredits int64           `json:"commission_credits"`
	NetCredits        int64           `json:"net_credits"`
	Sessions          int64           `json:"sessions"`
	CustomRequests    int64           `json:"custom_requests"`
	Series            []EarningsPoint `json:"series"`
}

// EarningsPoint is one bucket of earnings.
type EarningsPoint struct {
	Bucket       time.Time `json:"bucket"`
	NetCredits   int64     `json:"net_credits"`
	GrossCredits int64     `json:"gross_credits"`
	Sessions     int64     `json:"sessions"`
}

// SessionAnalytics summarises chat requests and readings.
type SessionAnalytics struct {
	Requested          int64   `json:"requested"`
	Accepted           int64   `json:"accepted"`
	Declined           int64   `json:"declined"`
	Expired            int64   `json:"expired"`
	Cancelled          int64   `json:"cancelled"`
	Completed          int64   `json:"completed"`
	AcceptanceRate     float64 `json:"acceptance_rate"`
	AvgResponseSeconds float64 `json:"avg_response_seconds"`
	AvgDurationSeconds float64 `json:"avg_duration_seconds"`
	TotalMinutes       int64   `json:"total_minutes"`
}

// ClientAnalytics summarises an advisor's clients (first names only).
type ClientAnalytics struct {
	Unique        int64       `json:"unique"`
	New           int64       `json:"new"`
	Returning     int64       `json:"returning"`
	ReturningRate float64     `json:"returning_rate"`
	Top           []TopClient `json:"top"`
}

// TopClient is one of an advisor's most engaged clients.
type TopClient struct {
	Name     string `json:"name"`
	Sessions int64  `json:"sessions"`
	Minutes  int64  `json:"minutes"`
}

// RatingAnalytics summarises ratings.
type RatingAnalytics struct {
	Average      float64          `json:"average"`
	Count        int              `json:"count"`
	Distribution map[string]int64 `json:"distribution"`
}

// SuccessScore is a 0–100 composite of service quality.
type SuccessScore struct {
	Value      int             `json:"value"`
	Grade      string          `json:"grade"`
	Components ScoreComponents `json:"components"`
}

// ScoreComponents are the 0–100 inputs of the success score.
type ScoreComponents struct {
	Acceptance     int `json:"acceptance"`
	Rating         int `json:"rating"`
	Retention      int `json:"retention"`
	Completion     int `json:"completion"`
	Responsiveness int `json:"responsiveness"`
}

// WorkingHours shows demand by weekday and hour in the advisor's timezone.
type WorkingHours struct {
	Grid   [][]int64  `json:"grid"`
	Levels [][]string `json:"levels"`
	Best   []BestHour `json:"best"`
}

// BestHour is a high-demand time slot.
type BestHour struct {
	Weekday  int   `json:"weekday"`
	Hour     int   `json:"hour"`
	Sessions int64 `json:"sessions"`
}

// PsychicAnalyticsService computes advisor dashboards.
type PsychicAnalyticsService struct {
	insights    repository.InsightsRepository
	earnings    repository.EarningRepository
	reviews     repository.ReviewRepository
	psychics    repository.PsychicRepository
	withdrawals *WithdrawalService
	clock       clock.Clock
}

// NewPsychicAnalyticsService wires the service.
func NewPsychicAnalyticsService(insights repository.InsightsRepository, earnings repository.EarningRepository, reviews repository.ReviewRepository,
	psychics repository.PsychicRepository, withdrawals *WithdrawalService, clk clock.Clock) *PsychicAnalyticsService {
	return &PsychicAnalyticsService{insights: insights, earnings: earnings, reviews: reviews, psychics: psychics, withdrawals: withdrawals, clock: clk}
}

var tzName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_+\-]*(/[A-Za-z0-9_+\-]+){0,2}$`)

// ParseTimezone validates an IANA zone name (empty = UTC).
func ParseTimezone(tz string) (string, *time.Location, error) {
	tz = strings.TrimSpace(tz)
	if tz == "" || tz == "UTC" {
		return "UTC", time.UTC, nil
	}
	if len(tz) > 64 || tz == "Local" || !tzName.MatchString(tz) || strings.Contains(tz, "..") {
		return "", nil, apperror.Validation(map[string]string{"tz": "Use an IANA time zone such as Europe/Paris."})
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "", nil, apperror.Validation(map[string]string{"tz": "Use an IANA time zone such as Europe/Paris."})
	}
	return tz, loc, nil
}

// Analytics returns the dashboard for one advisor.
func (s *PsychicAnalyticsService) Analytics(ctx context.Context, psychicID uuid.UUID, r Range, tzRaw, interval string) (*PsychicAnalytics, error) {
	tz, _, err := ParseTimezone(tzRaw)
	if err != nil {
		return nil, err
	}
	if interval == "" {
		interval = "day"
	}
	switch interval {
	case "day", "week", "month":
	default:
		return nil, apperror.Validation(map[string]string{"interval": "Use day, week or month."})
	}
	p, err := s.psychics.GetByUserID(ctx, psychicID)
	if err != nil {
		return nil, mapRepoErr(err, "Advisor")
	}
	out := &PsychicAnalytics{Range: AnalyticsRange{From: r.From, To: r.To, TZ: tz, Interval: interval}}

	from, to := r.From, r.To
	sum, err := s.earnings.Summary(ctx, psychicID, &from, &to)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out.Earnings = EarningsAnalytics{GrossCredits: sum.GrossCredits, CommissionCredits: sum.CommissionCredits, NetCredits: sum.NetCredits,
		Sessions: sum.Sessions, CustomRequests: sum.CustomRequests, Series: []EarningsPoint{}}
	series, err := s.insights.PsychicEarningsSeries(ctx, psychicID, r.From, r.To, interval, tz)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for _, b := range series {
		out.Earnings.Series = append(out.Earnings.Series, EarningsPoint{Bucket: b.Bucket, NetCredits: b.Net, GrossCredits: b.Gross, Sessions: b.Sessions})
	}

	payout, err := s.withdrawals.Balance(ctx, psychicID)
	if err != nil {
		return nil, err
	}
	out.Payout = *payout

	ss, err := s.insights.PsychicSessions(ctx, psychicID, r.From, r.To)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	decided := ss.Accepted + ss.Declined + ss.Expired
	out.Sessions = SessionAnalytics{Requested: ss.Requested, Accepted: ss.Accepted, Declined: ss.Declined, Expired: ss.Expired,
		Cancelled: ss.Cancelled, Completed: ss.Completed, AcceptanceRate: rate(ss.Accepted, decided),
		AvgResponseSeconds: round1(ss.AvgResponseSeconds), AvgDurationSeconds: round1(ss.AvgDurationSeconds), TotalMinutes: ss.TotalMinutes}

	cs, err := s.insights.PsychicClients(ctx, psychicID, r.From, r.To)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out.Clients = ClientAnalytics{Unique: cs.Unique, New: cs.New, Returning: cs.Returning, ReturningRate: rate(cs.Returning, cs.Unique), Top: []TopClient{}}
	top, err := s.insights.PsychicTopClients(ctx, psychicID, r.From, r.To, 5)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for _, c := range top {
		out.Clients.Top = append(out.Clients.Top, TopClient{Name: firstName("", c.Name), Sessions: c.Sessions, Minutes: c.Minutes})
	}

	stats, err := s.reviews.Stats(ctx, psychicID, nil, nil)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out.Rating = RatingAnalytics{Average: p.Rating, Count: p.ReviewsCount, Distribution: map[string]int64{}}
	for i := 1; i <= 5; i++ {
		out.Rating.Distribution[string(rune('0'+i))] = stats.Distribution[i]
	}

	out.Score = computeScore(ss, cs, p.Rating, p.ReviewsCount)

	cells, err := s.insights.PsychicHours(ctx, psychicID, r.From, r.To, tz)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out.Hours = workingHours(cells)
	return out, nil
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// computeScore weighs the quality signals that have data; a new advisor is
// scored on what is known rather than penalised for missing history.
func computeScore(ss *repository.PsychicSessionStats, cs *repository.PsychicClientStats, avgRating float64, ratings int) SuccessScore {
	type part struct {
		value  float64
		weight float64
		ok     bool
	}
	decided := ss.Accepted + ss.Declined + ss.Expired
	parts := map[string]part{
		"acceptance": {rate(ss.Accepted, decided) * 100, 25, decided > 0},
		"rating":     {avgRating / 5 * 100, 30, ratings > 0},
		"retention":  {rate(cs.Returning, cs.Unique) * 100, 15, cs.Unique > 0},
		"completion": {rate(ss.Completed-ss.Interrupted, ss.Completed) * 100, 15, ss.Completed > 0},
		"responsiveness": {(1 - math.Min(ss.AvgResponseSeconds/90, 1)) * 100, 15,
			ss.Accepted > 0},
	}
	var total, weights float64
	for _, p := range parts {
		if p.ok {
			total += p.value * p.weight
			weights += p.weight
		}
	}
	value := 0
	if weights > 0 {
		value = int(math.Round(total / weights))
	}
	grade := "needs_attention"
	switch {
	case value >= 85:
		grade = "excellent"
	case value >= 70:
		grade = "good"
	case value >= 50:
		grade = "fair"
	}
	comp := func(k string) int {
		if !parts[k].ok {
			return 0
		}
		return int(math.Round(parts[k].value))
	}
	return SuccessScore{Value: value, Grade: grade, Components: ScoreComponents{Acceptance: comp("acceptance"), Rating: comp("rating"),
		Retention: comp("retention"), Completion: comp("completion"), Responsiveness: comp("responsiveness")}}
}

// workingHours builds the 7×24 demand grid (Monday first) and splits busy
// slots into terciles: peak, medium and low.
func workingHours(cells []repository.HourCell) WorkingHours {
	grid := make([][]int64, 7)
	levels := make([][]string, 7)
	for d := range grid {
		grid[d] = make([]int64, 24)
		levels[d] = make([]string, 24)
		for h := range levels[d] {
			levels[d][h] = "none"
		}
	}
	for _, c := range cells {
		if c.Weekday >= 1 && c.Weekday <= 7 && c.Hour >= 0 && c.Hour < 24 {
			grid[c.Weekday-1][c.Hour] += c.Count
		}
	}
	type slot struct {
		d, h int
		n    int64
	}
	var busy []slot
	for d := range grid {
		for h, n := range grid[d] {
			if n > 0 {
				busy = append(busy, slot{d, h, n})
			}
		}
	}
	sort.Slice(busy, func(i, j int) bool {
		if busy[i].n != busy[j].n {
			return busy[i].n > busy[j].n
		}
		if busy[i].d != busy[j].d {
			return busy[i].d < busy[j].d
		}
		return busy[i].h < busy[j].h
	})
	if len(busy) > 0 {
		// Thresholds by value so equal counts always share a level.
		hi := busy[(len(busy)-1)/3].n
		lo := busy[(2*(len(busy)-1))/3].n
		for _, b := range busy {
			switch {
			case b.n >= hi:
				levels[b.d][b.h] = "peak"
			case b.n >= lo && lo < hi:
				levels[b.d][b.h] = "medium"
			default:
				levels[b.d][b.h] = "low"
			}
		}
	}
	best := []BestHour{}
	for i := 0; i < len(busy) && i < 3; i++ {
		best = append(best, BestHour{Weekday: busy[i].d, Hour: busy[i].h, Sessions: busy[i].n})
	}
	return WorkingHours{Grid: grid, Levels: levels, Best: best}
}
