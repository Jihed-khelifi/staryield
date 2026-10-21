package service

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/astro"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// InsightsService answers the admin audience and engagement dashboards.
type InsightsService struct {
	repo     repository.InsightsRepository
	users    repository.UserRepository
	admin    *AdminService
	clock    clock.Clock
	currency string
}

// NewInsightsService wires the insights service.
func NewInsightsService(repo repository.InsightsRepository, users repository.UserRepository, admin *AdminService, clk clock.Clock, currency string) *InsightsService {
	return &InsightsService{repo: repo, users: users, admin: admin, clock: clk, currency: currency}
}

// BucketCount is one point of a count series.
type BucketCount struct {
	Bucket time.Time `json:"bucket"`
	Count  int64     `json:"count"`
}

// Audience describes who the users are.
type Audience struct {
	TotalUsers int64                 `json:"total_users"`
	NewUsers   int64                 `json:"new_users"`
	ByRole     []repository.KeyCount `json:"by_role"`
	ByStatus   []repository.KeyCount `json:"by_status"`
	ByLocale   []repository.KeyCount `json:"by_locale"`
	ByCountry  []repository.KeyCount `json:"by_country"`
	ByAge      []repository.KeyCount `json:"by_age"`
	BySign     []repository.KeyCount `json:"by_sign"`
	ByDevice   []repository.KeyCount `json:"by_device"`
	BySource   []repository.KeyCount `json:"by_source"`
	Signups    []BucketCount         `json:"signups"`
}

func nonNil(in []repository.KeyCount) []repository.KeyCount {
	if in == nil {
		return []repository.KeyCount{}
	}
	return in
}

func intervalFor(r Range) string {
	if r.To.Sub(r.From) > 120*24*time.Hour {
		return "week"
	}
	return "day"
}

// Audience returns demographic and acquisition breakdowns of clients.
func (s *InsightsService) Audience(ctx context.Context, r Range) (*Audience, error) {
	a := &Audience{}
	var err error
	if a.TotalUsers, err = s.repo.CountUsers(ctx, true, nil, nil); err != nil {
		return nil, apperror.Internal(err)
	}
	if a.NewUsers, err = s.repo.CountUsers(ctx, true, &r.From, &r.To); err != nil {
		return nil, apperror.Internal(err)
	}
	for _, g := range []struct {
		col     string
		clients bool
		dst     *[]repository.KeyCount
	}{{"role", false, &a.ByRole}, {"status", false, &a.ByStatus}, {"locale", true, &a.ByLocale}, {"country", true, &a.ByCountry}} {
		rows, err := s.repo.GroupUsers(ctx, g.col, g.clients)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		*g.dst = nonNil(rows)
	}
	if a.ByAge, err = s.repo.AgeBuckets(ctx, s.clock.Now()); err != nil {
		return nil, apperror.Internal(err)
	}
	days, err := s.repo.BirthDays(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	signs := map[string]int64{}
	for _, d := range days {
		if sign := astro.SunSignForDate(d.Month, d.Day); sign != "" {
			signs[sign] += d.Count
		}
	}
	a.BySign = []repository.KeyCount{}
	for sign, n := range signs {
		a.BySign = append(a.BySign, repository.KeyCount{Key: sign, Count: n})
	}
	sort.Slice(a.BySign, func(i, j int) bool {
		if a.BySign[i].Count != a.BySign[j].Count {
			return a.BySign[i].Count > a.BySign[j].Count
		}
		return a.BySign[i].Key < a.BySign[j].Key
	})
	if a.ByDevice, err = s.repo.Devices(ctx, r.From, r.To); err != nil {
		return nil, apperror.Internal(err)
	}
	if a.BySource, err = s.repo.Sources(ctx); err != nil {
		return nil, apperror.Internal(err)
	}
	series, err := s.repo.SignupSeries(ctx, r.From, r.To, intervalFor(r))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	a.Signups = make([]BucketCount, 0, len(series))
	for _, p := range series {
		a.Signups = append(a.Signups, BucketCount{Bucket: p.Bucket, Count: p.Count})
	}
	a.ByAge, a.ByDevice, a.BySource = nonNil(a.ByAge), nonNil(a.ByDevice), nonNil(a.BySource)
	return a, nil
}

// Engagement summarises activity and monetisation.
type Engagement struct {
	Currency              string        `json:"currency"`
	ActiveUsers           []ActivePoint `json:"active_users"`
	DAU                   int64         `json:"dau"`
	WAU                   int64         `json:"wau"`
	MAU                   int64         `json:"mau"`
	Stickiness            float64       `json:"stickiness"`
	PayingUsers           int64         `json:"paying_users"`
	ARPPUCents            int64         `json:"arppu_cents"`
	RepeatPurchaseRate    float64       `json:"repeat_purchase_rate"`
	AvgSessionMinutes     float64       `json:"avg_session_minutes"`
	SessionsPerPayingUser float64       `json:"sessions_per_paying_user"`
	ChatConversionRate    float64       `json:"chat_conversion_rate"`
}

// ActivePoint is the number of active users in a bucket.
type ActivePoint struct {
	Bucket time.Time `json:"bucket"`
	Users  int64     `json:"users"`
}

// Engagement returns activity and monetisation metrics for a range.
func (s *InsightsService) Engagement(ctx context.Context, r Range, interval string) (*Engagement, error) {
	if interval == "" {
		interval = intervalFor(r)
	}
	switch interval {
	case "day", "week", "month":
	default:
		return nil, apperror.Validation(map[string]string{"interval": "Use day, week or month."})
	}
	series, err := s.repo.ActiveUserSeries(ctx, r.From, r.To, interval)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	e := &Engagement{Currency: s.currency, ActiveUsers: make([]ActivePoint, 0, len(series))}
	for _, p := range series {
		e.ActiveUsers = append(e.ActiveUsers, ActivePoint{Bucket: p.Bucket, Users: p.Count})
	}
	row, err := s.repo.Engagement(ctx, r.From, r.To, s.currency)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	e.DAU, e.WAU, e.MAU, e.PayingUsers = row.DAU, row.WAU, row.MAU, row.PayingUsers
	e.Stickiness = rate(row.DAU, row.MAU)
	if row.PayingUsers > 0 {
		e.ARPPUCents = row.RevenueCents / row.PayingUsers
		e.SessionsPerPayingUser = round1(float64(row.PaidUserSessions) / float64(row.PayingUsers))
	}
	e.RepeatPurchaseRate = rate(row.RepeatBuyers, row.Buyers)
	e.AvgSessionMinutes = round1(row.AvgSessionSeconds / 60)
	e.ChatConversionRate = rate(row.ChattingClients, max(row.ActiveClients, row.ChattingClients))
	return e, nil
}

// Cohort is one weekly sign-up cohort.
type Cohort struct {
	Cohort time.Time `json:"cohort"`
	Size   int64     `json:"size"`
	Weeks  []float64 `json:"weeks"`
}

// Retention returns weekly cohort retention for the last n weeks.
func (s *InsightsService) Retention(ctx context.Context, weeks int) ([]Cohort, error) {
	if weeks <= 0 {
		weeks = 8
	}
	if weeks > 26 {
		return nil, apperror.Validation(map[string]string{"weeks": "Use at most 26 weeks."})
	}
	now := s.clock.Now().UTC()
	thisWeek := repository.Buckets(now, now.Add(time.Second), "week")[0]
	since := thisWeek.AddDate(0, 0, -7*(weeks-1))
	sizes, cells, err := s.repo.Retention(ctx, since)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	active := map[time.Time]map[int]int64{}
	for _, c := range cells {
		if active[c.Cohort] == nil {
			active[c.Cohort] = map[int]int64{}
		}
		active[c.Cohort][c.Week] = c.Users
	}
	out := []Cohort{}
	for i := 0; i < weeks; i++ {
		start := since.AddDate(0, 0, 7*i)
		size := sizes[start]
		elapsed := int(thisWeek.Sub(start).Hours() / (24 * 7))
		row := Cohort{Cohort: start, Size: size, Weeks: make([]float64, 0, elapsed+1)}
		for w := 0; w <= elapsed; w++ {
			row.Weeks = append(row.Weeks, rate(active[start][w], size))
		}
		out = append(out, row)
	}
	return out, nil
}

// TopUserView ranks a user by a metric.
type TopUserView struct {
	User  AdminUserRow `json:"user"`
	Value int64        `json:"value"`
	// Currency is set for the spend metric (value in cents).
	Currency string `json:"currency,omitempty"`
}

// TopUsers ranks clients by spend, chat minutes or sessions.
func (s *InsightsService) TopUsers(ctx context.Context, metric string, r Range, limit int) ([]TopUserView, error) {
	switch metric {
	case "", "spend", "minutes", "sessions":
	default:
		return nil, apperror.Validation(map[string]string{"metric": "Use spend, minutes or sessions."})
	}
	rows, err := s.repo.TopUsers(ctx, metric, r.From, r.To, s.currency, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.UserID)
	}
	users, err := s.users.GetByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	byID := map[uuid.UUID]AdminUserRow{}
	for i := range users {
		byID[users[i].ID] = adminUserRow(&users[i])
	}
	out := make([]TopUserView, 0, len(rows))
	for _, r := range rows {
		if u, ok := byID[r.UserID]; ok {
			v := TopUserView{User: u, Value: r.Value}
			if metric == "" || metric == "spend" {
				v.Currency = s.currency
			}
			out = append(out, v)
		}
	}
	return out, nil
}

// UserInsights is the admin drill-down of one account.
type UserInsights struct {
	User     *AdminUserView        `json:"user"`
	Psychics []PsychicUsageView    `json:"psychics"`
	Devices  []repository.KeyCount `json:"devices"`
	Activity []ActivityView        `json:"activity"`
}

// PsychicUsageView is a client's history with one advisor.
type PsychicUsageView struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Slug     string    `json:"slug"`
	Sessions int64     `json:"sessions"`
	Minutes  int64     `json:"minutes"`
}

// ActivityView is one timeline entry.
type ActivityView struct {
	At      time.Time  `json:"at"`
	Type    string     `json:"type"`
	Summary string     `json:"summary"`
	RefID   *uuid.UUID `json:"ref_id,omitempty"`
}

// User returns the drill-down of one account.
func (s *InsightsService) User(ctx context.Context, id uuid.UUID) (*UserInsights, error) {
	u, err := s.admin.User(ctx, id)
	if err != nil {
		return nil, err
	}
	out := &UserInsights{User: u, Psychics: []PsychicUsageView{}, Activity: []ActivityView{}}
	usage, err := s.repo.PsychicsUsedBy(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for _, p := range usage {
		out.Psychics = append(out.Psychics, PsychicUsageView{ID: p.PsychicID, Name: p.Name, Slug: p.Slug, Sessions: p.Sessions, Minutes: p.Minutes})
	}
	if out.Devices, err = s.repo.UserDevices(ctx, id); err != nil {
		return nil, apperror.Internal(err)
	}
	out.Devices = nonNil(out.Devices)
	rows, err := s.repo.Activity(ctx, id, 100)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for _, a := range rows {
		out.Activity = append(out.Activity, ActivityView{At: a.At, Type: a.Type, Summary: a.Summary, RefID: a.RefID})
	}
	return out, nil
}
