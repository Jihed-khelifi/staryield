package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 3.0: calculator analytics

// WebVisitorsJob is the daily retention job of the visitor roll-up.
const WebVisitorsJob = "web-visitors-retention"

const (
	// DirectSource is the by_source label (and ?source= value) of visitors
	// without a utm_source.
	DirectSource = "(direct)"
	// calculatorCacheTTL is how long a report stays in Redis.
	calculatorCacheTTL = 60 * time.Second
	// calculatorMaxSpan is the longest reporting window.
	calculatorMaxSpan = 366 * 24 * time.Hour
	calculatorTopN    = 50
)

var registerAnonID = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

// CalculatorService answers the calculator usage and conversion reports and
// links registrations to the visitor that made them.
type CalculatorService struct {
	repo    repository.CalculatorRepository
	kv      cache.Store
	clock   clock.Clock
	enabled bool
}

// NewCalculatorService wires the service. enabled mirrors the analytics
// switch: when off, registrations are not linked to visitors.
func NewCalculatorService(repo repository.CalculatorRepository, kv cache.Store, clk clock.Clock, enabled bool) *CalculatorService {
	return &CalculatorService{repo: repo, kv: kv, clock: clk, enabled: enabled}
}

// CalculatorTotals are the distinct-visitor counts of a slice of traffic.
type CalculatorTotals struct {
	Visitors         int64   `json:"visitors"`
	SignedInVisitors int64   `json:"signed_in_visitors"`
	ExistingUsers    int64   `json:"existing_users"`
	Tried            int64   `json:"tried"`
	Completed        int64   `json:"completed"`
	Signups          int64   `json:"signups"`
	TryRate          float64 `json:"try_rate"`
	CompletionRate   float64 `json:"completion_rate"`
	SignupRate       float64 `json:"signup_rate"`
}

// CalculatorKindTotals is CalculatorTotals for one calculator.
type CalculatorKindTotals struct {
	Kind string `json:"kind"`
	CalculatorTotals
}

// CalculatorLocaleTotals is CalculatorTotals for one locale.
type CalculatorLocaleTotals struct {
	Locale string `json:"locale"`
	CalculatorTotals
}

// CalculatorSource is the funnel of one UTM source/medium/campaign.
type CalculatorSource struct {
	Source   string `json:"source"`
	Medium   string `json:"medium"`
	Campaign string `json:"campaign"`
	Visitors int64  `json:"visitors"`
	Tried    int64  `json:"tried"`
	Signups  int64  `json:"signups"`
}

// CalculatorLink is the funnel of one short link.
type CalculatorLink struct {
	LinkID       uuid.UUID `json:"link_id"`
	Code         string    `json:"code"`
	Label        string    `json:"label"`
	CampaignName string    `json:"campaign_name,omitempty"`
	Visitors     int64     `json:"visitors"`
	Tried        int64     `json:"tried"`
	Signups      int64     `json:"signups"`
}

// CalculatorPoint is one series bucket.
type CalculatorPoint struct {
	Bucket    time.Time `json:"bucket"`
	Visitors  int64     `json:"visitors"`
	Tried     int64     `json:"tried"`
	Completed int64     `json:"completed"`
	Signups   int64     `json:"signups"`
}

// CalculatorRange echoes the (day-aligned) window of a report.
type CalculatorRange struct {
	From     time.Time `json:"from"`
	To       time.Time `json:"to"`
	Interval string    `json:"interval"`
}

// CalculatorReport is the calculator usage report.
type CalculatorReport struct {
	Range    CalculatorRange          `json:"range"`
	Totals   CalculatorTotals         `json:"totals"`
	ByKind   []CalculatorKindTotals   `json:"by_kind"`
	ByLocale []CalculatorLocaleTotals `json:"by_locale"`
	BySource []CalculatorSource       `json:"by_source"`
	ByLink   []CalculatorLink         `json:"by_link"`
	Series   []CalculatorPoint        `json:"series"`
}

// CalculatorUser is the account of a conversion.
type CalculatorUser struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// CalculatorAttribution is the first touch of a converted visitor.
type CalculatorAttribution struct {
	UTMSource    string `json:"utm_source"`
	UTMMedium    string `json:"utm_medium"`
	UTMCampaign  string `json:"utm_campaign"`
	LinkCode     string `json:"link_code,omitempty"`
	CampaignName string `json:"campaign_name,omitempty"`
}

// CalculatorConversion is one visitor who signed up after using the calculators.
type CalculatorConversion struct {
	User         CalculatorUser         `json:"user"`
	FirstVisitAt time.Time              `json:"first_visit_at"`
	SignedUpAt   time.Time              `json:"signed_up_at"`
	Kinds        []string               `json:"kinds"`
	Locales      []string               `json:"locales"`
	Tried        bool                   `json:"tried"`
	Attribution  *CalculatorAttribution `json:"attribution,omitempty"`
}

// CalculatorQuery is the raw (unvalidated) query string of both reports.
type CalculatorQuery struct {
	From, To, Interval   string
	Kind, Locale, Source string
	CampaignID, LinkID   *uuid.UUID
	Cursor               string
	Limit                int
	SourceSet            bool // ?source= was present (an empty value is invalid)
}

// calculatorQuery is a validated, normalized query.
type calculatorQuery struct {
	from, to time.Time // UTC midnights, to exclusive
	interval string    // day | week | month
	filter   repository.CalculatorFilter
}

// key is the canonical form of the query (the cache key is its hash): two
// requests that mean the same thing share it.
func (q calculatorQuery) key() string {
	src := "*"
	if q.filter.Source != nil {
		src = "=" + *q.filter.Source
	}
	camp, link := "", ""
	if q.filter.CampaignID != nil {
		camp = q.filter.CampaignID.String()
	}
	if q.filter.LinkID != nil {
		link = q.filter.LinkID.String()
	}
	return fmt.Sprintf("v1|%s|%s|%s|%s|%s|%s|%s|%s", q.from.Format("2006-01-02"), q.to.Format("2006-01-02"), q.interval,
		q.filter.Kind, q.filter.Locale, src, camp, link)
}

func (q calculatorQuery) cacheKey() string {
	sum := sha256.Sum256([]byte(q.key()))
	return "calc:report:" + hex.EncodeToString(sum[:])
}

func utcDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// normalize validates a query. Data is stored per UTC day, so the window is
// aligned to whole days (the end rounded up) and "hour" series are served
// per day.
func (s *CalculatorService) normalize(in CalculatorQuery) (calculatorQuery, error) {
	r, err := ParseRange(in.From, in.To, s.clock.Now())
	if err != nil {
		// ParseRange caps windows at 400 days; these reports cap them at 366.
		if ae, ok := apperror.As(err); ok && fmt.Sprint(ae.Details["fields"]) == "map[from:The reporting window is limited to 400 days.]" {
			return calculatorQuery{}, apperror.Validation(map[string]string{"from": "The reporting window is limited to 366 days."})
		}
		return calculatorQuery{}, err
	}
	if r.To.Sub(r.From) > calculatorMaxSpan {
		return calculatorQuery{}, apperror.Validation(map[string]string{"from": "The reporting window is limited to 366 days."})
	}
	q := calculatorQuery{from: utcDay(r.From), to: utcDay(r.To)}
	if r.To.After(q.to) {
		q.to = q.to.AddDate(0, 0, 1)
	}
	switch in.Interval {
	case "", "day", "hour":
		q.interval = "day"
	case "week", "month":
		q.interval = in.Interval
	default:
		return calculatorQuery{}, apperror.Validation(map[string]string{"interval": "Use hour, day, week or month."})
	}
	f := repository.CalculatorFilter{FromDay: q.from, ToDay: q.to, CampaignID: in.CampaignID, LinkID: in.LinkID}
	if k := strings.TrimSpace(in.Kind); k != "" {
		if !domain.ValidCalculatorKind(k) {
			return calculatorQuery{}, apperror.Validation(map[string]string{"kind": "Use natal-chart, path-of-life or astrology."})
		}
		f.Kind = k
	}
	if l := strings.ToLower(strings.TrimSpace(in.Locale)); l != "" {
		if !domain.ValidCalculatorLocale(l) {
			return calculatorQuery{}, apperror.Validation(map[string]string{"locale": "Use en, fr or es."})
		}
		f.Locale = l
	}
	if in.SourceSet || in.Source != "" {
		src := strings.TrimSpace(in.Source)
		switch {
		case src == DirectSource:
			empty := ""
			f.Source = &empty
		case utmValue.MatchString(src):
			f.Source = &src
		default:
			return calculatorQuery{}, apperror.Validation(map[string]string{"source": "Use a UTM source or (direct)."})
		}
	}
	q.filter = f
	return q, nil
}

// fillSeries returns one point per bucket of [from, to), zero where the
// repository had no data.
func fillSeries(from, to time.Time, interval string, rows []repository.CalcBucketRow) []CalculatorPoint {
	byBucket := map[time.Time]repository.CalcBucketRow{}
	for _, r := range rows {
		byBucket[bucketStart(r.Bucket, interval)] = r
	}
	out := []CalculatorPoint{}
	for b := bucketStart(from, interval); b.Before(to); b = nextBucket(b, interval) {
		r := byBucket[b]
		out = append(out, CalculatorPoint{Bucket: b, Visitors: r.Visitors, Tried: r.Tried, Completed: r.Completed, Signups: r.Signups})
	}
	return out
}

// mergeSignups adds the per-bucket sign-ups to the usage rows.
func mergeSignups(usage, signups []repository.CalcBucketRow) []repository.CalcBucketRow {
	idx := map[time.Time]int{}
	out := append([]repository.CalcBucketRow(nil), usage...)
	for i, r := range out {
		idx[r.Bucket.UTC()] = i
	}
	for _, s := range signups {
		if i, ok := idx[s.Bucket.UTC()]; ok {
			out[i].Signups = s.Signups
			continue
		}
		out = append(out, repository.CalcBucketRow{Bucket: s.Bucket, Signups: s.Signups})
	}
	return out
}

// bucketStart truncates like PostgreSQL's date_trunc (weeks start on Monday).
func bucketStart(t time.Time, interval string) time.Time {
	d := utcDay(t)
	switch interval {
	case "week":
		return d.AddDate(0, 0, -((int(d.Weekday()) + 6) % 7))
	case "month":
		return time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	return d
}

func nextBucket(b time.Time, interval string) time.Time {
	switch interval {
	case "week":
		return b.AddDate(0, 0, 7)
	case "month":
		return b.AddDate(0, 1, 0)
	}
	return b.AddDate(0, 0, 1)
}

func calcTotals(r repository.CalcTotalsRow) CalculatorTotals {
	return CalculatorTotals{Visitors: r.Visitors, SignedInVisitors: r.SignedInVisitors, ExistingUsers: r.ExistingUsers,
		Tried: r.Tried, Completed: r.Completed, Signups: r.Signups,
		TryRate: rate(r.Tried, r.Visitors), CompletionRate: rate(r.Completed, r.Tried),
		SignupRate: rate(r.Signups, r.Visitors-r.ExistingUsers)}
}

// Report returns the calculator usage report (cached for 60 s per normalized query).
func (s *CalculatorService) Report(ctx context.Context, in CalculatorQuery) (*CalculatorReport, error) {
	q, err := s.normalize(in)
	if err != nil {
		return nil, err
	}
	key := q.cacheKey()
	if s.kv != nil {
		if raw, ok, err := s.kv.Get(ctx, key); err == nil && ok {
			var out CalculatorReport
			if json.Unmarshal([]byte(raw), &out) == nil {
				return &out, nil
			}
		}
	}
	var (
		totals          repository.CalcTotalsRow
		kinds, locales  []repository.CalcTotalsRow
		sources         []repository.CalcSourceRow
		links           []repository.CalcLinkRow
		series, signups []repository.CalcBucketRow
		wg              sync.WaitGroup
		mu              sync.Mutex
		firstErr        error
	)
	run := func(fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}()
	}
	f := q.filter
	run(func() (err error) { totals, err = s.repo.Totals(ctx, f); return })
	run(func() (err error) { kinds, err = s.repo.ByDimension(ctx, f, "kind"); return })
	run(func() (err error) { locales, err = s.repo.ByDimension(ctx, f, "locale"); return })
	run(func() (err error) { sources, err = s.repo.BySource(ctx, f, calculatorTopN); return })
	run(func() (err error) { links, err = s.repo.ByLink(ctx, f, calculatorTopN); return })
	run(func() (err error) { series, err = s.repo.Series(ctx, f, q.interval); return })
	run(func() (err error) { signups, err = s.repo.SignupSeries(ctx, f, q.interval); return })
	wg.Wait()
	if firstErr != nil {
		return nil, apperror.Internal(firstErr)
	}
	out := &CalculatorReport{Range: CalculatorRange{From: q.from, To: q.to, Interval: q.interval}, Totals: calcTotals(totals),
		ByKind: []CalculatorKindTotals{}, ByLocale: []CalculatorLocaleTotals{}, BySource: []CalculatorSource{}, ByLink: []CalculatorLink{}}
	kindRows, localeRows := map[string]repository.CalcTotalsRow{}, map[string]repository.CalcTotalsRow{}
	for _, r := range kinds {
		kindRows[r.Key] = r
	}
	for _, r := range locales {
		localeRows[r.Key] = r
	}
	for _, k := range domain.CalculatorKinds {
		out.ByKind = append(out.ByKind, CalculatorKindTotals{Kind: k, CalculatorTotals: calcTotals(kindRows[k])})
	}
	for _, l := range domain.CalculatorLocales {
		out.ByLocale = append(out.ByLocale, CalculatorLocaleTotals{Locale: l, CalculatorTotals: calcTotals(localeRows[l])})
	}
	for _, r := range sources {
		src := r.Source
		if src == "" {
			src = DirectSource
		}
		out.BySource = append(out.BySource, CalculatorSource{Source: src, Medium: r.Medium, Campaign: r.Campaign,
			Visitors: r.Visitors, Tried: r.Tried, Signups: r.Signups})
	}
	for _, r := range links {
		out.ByLink = append(out.ByLink, CalculatorLink{LinkID: r.LinkID, Code: r.Code, Label: r.Label, CampaignName: r.CampaignName,
			Visitors: r.Visitors, Tried: r.Tried, Signups: r.Signups})
	}
	out.Series = fillSeries(q.from, q.to, q.interval, mergeSignups(series, signups))
	if s.kv != nil {
		if b, err := json.Marshal(out); err == nil {
			warnErr(ctx, "calculators: cache report", s.kv.Set(ctx, key, string(b), calculatorCacheTTL))
		}
	}
	return out, nil
}

// Conversions lists the sign-ups of visitors who used the calculators.
func (s *CalculatorService) Conversions(ctx context.Context, in CalculatorQuery) (*Page[CalculatorConversion], error) {
	q, err := s.normalize(in)
	if err != nil {
		return nil, err
	}
	cursor, err := repository.DecodeCursor(in.Cursor)
	if err != nil {
		return nil, apperror.Validation(map[string]string{"cursor": "Invalid cursor."})
	}
	limit := in.Limit
	if limit < 1 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := s.repo.Conversions(ctx, q.filter, cursor, limit+1)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	page := &Page[CalculatorConversion]{Items: make([]CalculatorConversion, 0, min(len(rows), limit))}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.SignedUpAt, last.UserID.String())
	}
	for _, r := range rows {
		c := CalculatorConversion{
			User:         CalculatorUser{ID: r.UserID, Email: r.Email, DisplayName: r.DisplayName, Role: r.Role, Status: r.Status, CreatedAt: r.UserCreatedAt},
			FirstVisitAt: r.FirstVisitAt, SignedUpAt: r.SignedUpAt, Kinds: orderedSubset(domain.CalculatorKinds, r.Kinds),
			Locales: orderedSubset(domain.CalculatorLocales, r.Locales), Tried: r.Tried}
		if r.UTMSource != "" || r.UTMMedium != "" || r.UTMCampaign != "" || r.LinkCode != "" || r.CampaignName != "" {
			c.Attribution = &CalculatorAttribution{UTMSource: r.UTMSource, UTMMedium: r.UTMMedium, UTMCampaign: r.UTMCampaign,
				LinkCode: r.LinkCode, CampaignName: r.CampaignName}
		}
		page.Items = append(page.Items, c)
	}
	return page, nil
}

// orderedSubset returns the members of order that appear in have.
func orderedSubset(order, have []string) []string {
	set := map[string]bool{}
	for _, h := range have {
		set[h] = true
	}
	out := []string{}
	for _, o := range order {
		if set[o] {
			out = append(out, o)
		}
	}
	return out
}

// ValidAnonymousID reports whether id is an acceptable browser visitor id.
func ValidAnonymousID(id string) bool { return registerAnonID.MatchString(id) }

// LinkSignup links the browser that registered an account to the new user
// (signed_up_at = the account's created_at). Invalid or missing visitor ids
// are ignored, and failures never block sign-up.
func (s *CalculatorService) LinkSignup(ctx context.Context, userID uuid.UUID, locale string, in *AttributionInput) {
	if s == nil || !s.enabled || in == nil || !ValidAnonymousID(in.AnonymousID) {
		return
	}
	l := &domain.VisitorLanding{UTMSource: cleanUTM(in.UTMSource), UTMMedium: cleanUTM(in.UTMMedium), UTMCampaign: cleanUTM(in.UTMCampaign),
		UTMContent: cleanUTM(in.UTMContent), UTMTerm: cleanUTM(in.UTMTerm)}
	if clickIDPattern.MatchString(strings.TrimSpace(in.ClickID)) {
		l.ClickID = strings.TrimSpace(in.ClickID)
	}
	warnErr(ctx, "calculators: link sign-up", s.repo.LinkSignup(ctx, userID, in.AnonymousID, normalizeLocale(locale), l,
		referrerHost(in.Referrer), s.clock.Now().UTC()))
}

// RunRetention deletes roll-up rows older than 25 months (job
// web-visitors-retention).
func (s *CalculatorService) RunRetention(ctx context.Context, now time.Time) (int64, error) {
	return s.repo.DeleteOlderThan(ctx, now.UTC().AddDate(0, -25, 0))
}

// PurgeForUser implements UserDataPurger (account erasure).
func (s *CalculatorService) PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.repo.PurgeForUser(ctx, userID)
}
