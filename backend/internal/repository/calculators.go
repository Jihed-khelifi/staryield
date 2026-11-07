package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 3.0: calculator analytics

// CalculatorFilter narrows the calculator reports. FromDay/ToDay are UTC
// dates (ToDay exclusive).
type CalculatorFilter struct {
	FromDay, ToDay time.Time
	Kind, Locale   string
	// Source filters on utm_source: nil = any, "" = direct (no utm_source).
	Source     *string
	CampaignID *uuid.UUID
	LinkID     *uuid.UUID
}

// CalcTotalsRow holds raw distinct-visitor counts.
type CalcTotalsRow struct {
	Key              string
	Visitors         int64
	SignedInVisitors int64
	ExistingUsers    int64
	Tried            int64
	Completed        int64
	Signups          int64
}

// CalcSourceRow is the funnel of one utm_source/medium/campaign.
type CalcSourceRow struct {
	Source, Medium, Campaign string
	Visitors, Tried, Signups int64
}

// CalcLinkRow is the funnel of one short link.
type CalcLinkRow struct {
	LinkID                   uuid.UUID
	Code, Label              string
	CampaignName             string
	Visitors, Tried, Signups int64
}

// CalcBucketRow is one series bucket (only buckets with data).
type CalcBucketRow struct {
	Bucket                              time.Time
	Visitors, Tried, Completed, Signups int64
}

// CalcConversionRow is one visitor who became a user.
type CalcConversionRow struct {
	UserID        uuid.UUID
	Email         string
	DisplayName   string
	Role          string
	Status        string
	UserCreatedAt time.Time
	FirstVisitAt  time.Time
	SignedUpAt    time.Time
	Kinds         []string
	Locales       []string
	Tried         bool
	UTMSource     string
	UTMMedium     string
	UTMCampaign   string
	LinkCode      string
	CampaignName  string
}

// CalculatorRepository stores the visitor roll-up and answers the calculator
// usage and conversion reports.
type CalculatorRepository interface {
	ApplyVisitorBatch(ctx context.Context, b *domain.VisitorBatch) error
	LinkSignup(ctx context.Context, userID uuid.UUID, anonymousID, locale string, landing *domain.VisitorLanding, referrerHost string, now time.Time) error
	Totals(ctx context.Context, f CalculatorFilter) (CalcTotalsRow, error)
	ByDimension(ctx context.Context, f CalculatorFilter, dim string) ([]CalcTotalsRow, error)
	BySource(ctx context.Context, f CalculatorFilter, limit int) ([]CalcSourceRow, error)
	ByLink(ctx context.Context, f CalculatorFilter, limit int) ([]CalcLinkRow, error)
	Series(ctx context.Context, f CalculatorFilter, unit string) ([]CalcBucketRow, error)
	SignupSeries(ctx context.Context, f CalculatorFilter, unit string) ([]CalcBucketRow, error)
	Conversions(ctx context.Context, f CalculatorFilter, cursor *Cursor, limit int) ([]CalcConversionRow, error)
	PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error)
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// GormCalculatorRepository implements CalculatorRepository.
type GormCalculatorRepository struct{ db *gorm.DB }

// NewCalculatorRepository returns a GORM calculator analytics repository.
func NewCalculatorRepository(db *gorm.DB) *GormCalculatorRepository {
	return &GormCalculatorRepository{db: db}
}

const (
	upsertVisitorChunk = 400
	upsertVisitChunk   = 2000
)

// hasAttribution is true for a web_visitors row (alias t) that already has a
// first touch: the first campaign_landing wins and later ones never replace it.
func hasAttribution(t string) string {
	return "(" + t + ".utm_source <> '' OR " + t + ".utm_medium <> '' OR " + t + ".utm_campaign <> '' OR " + t + ".utm_content <> '' OR " +
		t + ".utm_term <> '' OR " + t + ".click_id IS NOT NULL OR " + t + ".link_id IS NOT NULL OR " + t + ".campaign_id IS NOT NULL)"
}

// firstTouchSet keeps the existing first touch or takes the incoming one.
func firstTouchSet() string {
	keep := hasAttribution("web_visitors")
	var b strings.Builder
	for _, c := range []string{"utm_source", "utm_medium", "utm_campaign", "utm_content", "utm_term", "click_id", "link_id", "campaign_id"} {
		fmt.Fprintf(&b, "%s = CASE WHEN %s THEN web_visitors.%s ELSE EXCLUDED.%s END,\n", c, keep, c, c)
	}
	b.WriteString("referrer_host = CASE WHEN web_visitors.referrer_host = '' THEN EXCLUDED.referrer_host ELSE web_visitors.referrer_host END")
	return b.String()
}

func (r *GormCalculatorRepository) resolveLandings(ctx context.Context, db *gorm.DB, landings []*domain.VisitorLanding, now time.Time) error {
	clickIDs := map[uuid.UUID]bool{}
	slugs := map[string]bool{}
	for _, l := range landings {
		if id, err := uuid.Parse(l.ClickID); err == nil {
			clickIDs[id] = true
		}
		if l.UTMCampaign != "" {
			slugs[strings.ToLower(l.UTMCampaign)] = true
		}
	}
	clicks := map[uuid.UUID]*domain.ClickResolution{}
	if len(clickIDs) > 0 {
		ids := make([]uuid.UUID, 0, len(clickIDs))
		for id := range clickIDs {
			ids = append(ids, id)
		}
		var rows []struct {
			ID                                                     uuid.UUID
			LinkID                                                 uuid.UUID
			CampaignID                                             *uuid.UUID
			UTMSource, UTMMedium, UTMCampaign, UTMContent, UTMTerm string
		}
		// Only clicks of the last 90 days count, like sign-up attribution.
		err := db.WithContext(ctx).Raw(`SELECT lc.id, lc.link_id, lc.campaign_id,
			sl.utm_source AS utm_source, sl.utm_medium AS utm_medium, sl.utm_campaign AS utm_campaign,
			sl.utm_content AS utm_content, sl.utm_term AS utm_term
			FROM link_clicks lc JOIN short_links sl ON sl.id = lc.link_id
			WHERE lc.id IN ? AND lc.created_at > ?`, ids, now.AddDate(0, 0, -90)).Scan(&rows).Error
		if err != nil {
			return err
		}
		for _, row := range rows {
			clicks[row.ID] = &domain.ClickResolution{ClickID: row.ID, LinkID: row.LinkID, CampaignID: row.CampaignID,
				UTMSource: row.UTMSource, UTMMedium: row.UTMMedium, UTMCampaign: row.UTMCampaign, UTMContent: row.UTMContent, UTMTerm: row.UTMTerm}
		}
	}
	bySlug := map[string]uuid.UUID{}
	// Slugs of resolved clicks' links are needed as well, so look up after
	// the click UTM values are known.
	for _, l := range landings {
		if id, err := uuid.Parse(l.ClickID); err == nil {
			if c := clicks[id]; c != nil && c.UTMCampaign != "" {
				slugs[strings.ToLower(c.UTMCampaign)] = true
			}
		}
	}
	if len(slugs) > 0 {
		list := make([]string, 0, len(slugs))
		for s := range slugs {
			list = append(list, s)
		}
		var rows []struct {
			ID   uuid.UUID
			Slug string
		}
		if err := db.WithContext(ctx).Raw(`SELECT id, slug FROM campaigns WHERE slug IN ?`, list).Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			bySlug[row.Slug] = row.ID
		}
	}
	for _, l := range landings {
		var click *domain.ClickResolution
		if id, err := uuid.Parse(l.ClickID); err == nil {
			click = clicks[id]
		}
		domain.ResolveLanding(l, click, bySlug)
	}
	return nil
}

// ApplyVisitorBatch stores one coalesced batch in a single transaction:
// visitors first, then their user links, then the per-day calculator rows.
func (r *GormCalculatorRepository) ApplyVisitorBatch(ctx context.Context, b *domain.VisitorBatch) error {
	if b == nil || b.Empty() {
		return nil
	}
	var visitors []*domain.VisitorUpsert
	var links []*domain.VisitorUpsert
	var landings []*domain.VisitorLanding
	for _, v := range b.Visitors {
		if v.Touched {
			visitors = append(visitors, v)
			if v.Landing != nil {
				landings = append(landings, v.Landing)
			}
		}
		if v.UserID != nil {
			links = append(links, v)
		}
	}
	// Stable lock order across concurrent writers.
	sort.Slice(visitors, func(i, j int) bool { return visitors[i].AnonymousID < visitors[j].AnonymousID })
	sort.Slice(links, func(i, j int) bool { return links[i].AnonymousID < links[j].AnonymousID })
	visits := make([]domain.VisitKey, 0, len(b.Visits))
	for k := range b.Visits {
		visits = append(visits, k)
	}
	sort.Slice(visits, func(i, j int) bool {
		a, c := visits[i], visits[j]
		if !a.Day.Equal(c.Day) {
			return a.Day.Before(c.Day)
		}
		if a.AnonymousID != c.AnonymousID {
			return a.AnonymousID < c.AnonymousID
		}
		if a.Kind != c.Kind {
			return a.Kind < c.Kind
		}
		return a.Locale < c.Locale
	})
	return database.Conn(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		if len(landings) > 0 {
			// Clicks of the last 90 days count, relative to the landing.
			since := landings[0].At
			for _, l := range landings {
				if l.At.Before(since) {
					since = l.At
				}
			}
			if err := r.resolveLandings(ctx, tx, landings, since); err != nil {
				return err
			}
		}
		for start := 0; start < len(visitors); start += upsertVisitorChunk {
			end := min(start+upsertVisitorChunk, len(visitors))
			if err := r.upsertVisitors(ctx, tx, visitors[start:end]); err != nil {
				return err
			}
		}
		for start := 0; start < len(links); start += upsertVisitorChunk {
			end := min(start+upsertVisitorChunk, len(links))
			if err := r.linkVisitors(ctx, tx, links[start:end]); err != nil {
				return err
			}
		}
		for start := 0; start < len(visits); start += upsertVisitChunk {
			end := min(start+upsertVisitChunk, len(visits))
			if err := r.upsertVisits(ctx, tx, visits[start:end], b.Visits); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *GormCalculatorRepository) upsertVisitors(ctx context.Context, tx *gorm.DB, rows []*domain.VisitorUpsert) error {
	var sb strings.Builder
	args := make([]any, 0, len(rows)*13)
	sb.WriteString(`INSERT INTO web_visitors (anonymous_id, first_seen_at, last_seen_at, first_locale, utm_source, utm_medium, utm_campaign,
		utm_content, utm_term, click_id, link_id, campaign_id, referrer_host) VALUES `)
	for i, v := range rows {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("(?,?,?,?,?,?,?,?,?,?,?,?,?)")
		l := v.Landing
		if l == nil {
			l = &domain.VisitorLanding{}
		}
		args = append(args, v.AnonymousID, v.FirstSeen, v.LastSeen, v.FirstLocale, l.UTMSource, l.UTMMedium, l.UTMCampaign,
			l.UTMContent, l.UTMTerm, l.ResolvedClick, l.LinkID, l.CampaignID, v.ReferrerHost)
	}
	sb.WriteString(` ON CONFLICT (anonymous_id) DO UPDATE SET
		first_locale = CASE WHEN EXCLUDED.first_seen_at < web_visitors.first_seen_at THEN EXCLUDED.first_locale ELSE web_visitors.first_locale END,
		first_seen_at = LEAST(web_visitors.first_seen_at, EXCLUDED.first_seen_at),
		last_seen_at = GREATEST(web_visitors.last_seen_at, EXCLUDED.last_seen_at),
		` + firstTouchSet())
	return tx.WithContext(ctx).Exec(sb.String(), args...).Error
}

// linkVisitors links visitors to accounts, first link wins. A visitor whose
// account was created after the visitor was first seen is a sign-up.
func (r *GormCalculatorRepository) linkVisitors(ctx context.Context, tx *gorm.DB, rows []*domain.VisitorUpsert) error {
	var sb strings.Builder
	args := make([]any, 0, len(rows)*3)
	sb.WriteString(`UPDATE web_visitors w SET user_id = v.uid, linked_at = v.at,
		signed_up_at = CASE WHEN u.created_at > w.first_seen_at THEN u.created_at ELSE NULL END
		FROM (VALUES `)
	for i, v := range rows {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("(?::text, ?::uuid, ?::timestamptz)")
		args = append(args, v.AnonymousID, *v.UserID, v.LinkedAt)
	}
	sb.WriteString(`) AS v(aid, uid, at) JOIN users u ON u.id = v.uid
		WHERE w.anonymous_id = v.aid AND w.user_id IS NULL`)
	return tx.WithContext(ctx).Exec(sb.String(), args...).Error
}

func (r *GormCalculatorRepository) upsertVisits(ctx context.Context, tx *gorm.DB, keys []domain.VisitKey, counts map[domain.VisitKey]*domain.VisitCounts) error {
	var sb strings.Builder
	args := make([]any, 0, len(keys)*8)
	sb.WriteString(`INSERT INTO calculator_visits (day, anonymous_id, kind, locale, views, started, completed, signed_in) VALUES `)
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("(?,?,?,?,?,?,?,?)")
		c := counts[k]
		args = append(args, k.Day, k.AnonymousID, k.Kind, k.Locale, c.Views, c.Started, c.Completed, c.SignedIn)
	}
	sb.WriteString(` ON CONFLICT (day, anonymous_id, kind, locale) DO UPDATE SET
		views = calculator_visits.views + EXCLUDED.views,
		started = calculator_visits.started + EXCLUDED.started,
		completed = calculator_visits.completed + EXCLUDED.completed,
		signed_in = calculator_visits.signed_in OR EXCLUDED.signed_in`)
	return tx.WithContext(ctx).Exec(sb.String(), args...).Error
}

// LinkSignup links the browser that registered an account to it
// (signed_up_at = the account's created_at), creating the visitor when no
// calculator event reached the server first. The first link wins: a visitor
// already linked to an account keeps it. A first touch carried by the
// registration only fills a visitor that has none.
func (r *GormCalculatorRepository) LinkSignup(ctx context.Context, userID uuid.UUID, anonymousID, locale string, landing *domain.VisitorLanding, referrerHost string, now time.Time) error {
	l := landing
	if l == nil {
		l = &domain.VisitorLanding{}
	}
	db := database.Conn(ctx, r.db)
	if !l.Empty() {
		if err := r.resolveLandings(ctx, db, []*domain.VisitorLanding{l}, now); err != nil {
			return err
		}
	}
	return db.WithContext(ctx).Exec(`INSERT INTO web_visitors (anonymous_id, first_seen_at, last_seen_at, first_locale, utm_source, utm_medium,
			utm_campaign, utm_content, utm_term, click_id, link_id, campaign_id, referrer_host, user_id, linked_at, signed_up_at)
		SELECT ?::text, ?::timestamptz, ?::timestamptz, ?::text, ?::text, ?::text, ?::text, ?::text, ?::text, ?::uuid, ?::uuid, ?::uuid, ?::text, u.id, ?::timestamptz, u.created_at FROM users u WHERE u.id = ?
		ON CONFLICT (anonymous_id) DO UPDATE SET
			last_seen_at = GREATEST(web_visitors.last_seen_at, EXCLUDED.last_seen_at),
			signed_up_at = CASE WHEN web_visitors.user_id IS NULL THEN EXCLUDED.signed_up_at ELSE web_visitors.signed_up_at END,
			linked_at = CASE WHEN web_visitors.user_id IS NULL THEN EXCLUDED.linked_at ELSE web_visitors.linked_at END,
			user_id = COALESCE(web_visitors.user_id, EXCLUDED.user_id),
			`+firstTouchSet(),
		anonymousID, now, now, locale, l.UTMSource, l.UTMMedium, l.UTMCampaign, l.UTMContent, l.UTMTerm, l.ResolvedClick, l.LinkID,
		l.CampaignID, referrerHost, now, userID).Error
}

// ---------------------------------------------------------------------------
// Reports
// ---------------------------------------------------------------------------

func (f CalculatorFilter) visits(alias string) (string, []any) {
	where := alias + ".day >= ? AND " + alias + ".day < ?"
	args := []any{f.FromDay, f.ToDay}
	if f.Kind != "" {
		where += " AND " + alias + ".kind = ?"
		args = append(args, f.Kind)
	}
	if f.Locale != "" {
		where += " AND " + alias + ".locale = ?"
		args = append(args, f.Locale)
	}
	return where, args
}

// visitor filters apply to the LEFT-JOINed web_visitors alias.
func (f CalculatorFilter) visitor(alias string) (string, []any) {
	where := "TRUE"
	var args []any
	if f.Source != nil {
		if *f.Source == "" {
			where += " AND COALESCE(" + alias + ".utm_source, '') = ''"
		} else {
			where += " AND " + alias + ".utm_source = ?"
			args = append(args, *f.Source)
		}
	}
	if f.CampaignID != nil {
		where += " AND " + alias + ".campaign_id = ?"
		args = append(args, *f.CampaignID)
	}
	if f.LinkID != nil {
		where += " AND " + alias + ".link_id = ?"
		args = append(args, *f.LinkID)
	}
	return where, args
}

func (f CalculatorFilter) hasVisitorFilter() bool {
	return f.Source != nil || f.CampaignID != nil || f.LinkID != nil
}

// firstVisit is the instant of a visitor's first visit in the window: the
// visitor's first sighting when that is on the first visit day, otherwise the
// start of that day (visits are stored per day).
const firstVisit = "GREATEST(w.first_seen_at, (pv.first_day::timestamp AT TIME ZONE 'UTC'))"

const (
	signupCond   = "(w.signed_up_at IS NOT NULL AND w.signed_up_at >= " + firstVisit + ")"
	existingCond = "(w.user_id IS NOT NULL AND (w.signed_up_at IS NULL OR w.signed_up_at < " + firstVisit + "))"
)

// A visitor with a visit in the window was last seen inside it or later (the
// roll-up writes both together), so joining only those lets PostgreSQL build
// the visitor side from the last_seen_at index instead of scanning every
// visitor ever recorded.
const seenInWindow = " AND w.last_seen_at >= ?"

// perVisitor is the CTE grouping the window's visits per visitor and dim.
func perVisitor(f CalculatorFilter, dim string) (string, []any) {
	where, args := f.visits("cv")
	return `pv AS (
		SELECT cv.anonymous_id, ` + dim + ` AS dim, min(cv.day) AS first_day,
		       bool_or(cv.signed_in) AS signed_in, bool_or(cv.started > 0) AS tried, bool_or(cv.completed > 0) AS completed
		FROM calculator_visits cv WHERE ` + where + ` GROUP BY cv.anonymous_id, ` + dim + `)`, args
}

func (r *GormCalculatorRepository) dimRows(ctx context.Context, f CalculatorFilter, dim string) ([]CalcTotalsRow, error) {
	cte, args := perVisitor(f, dim)
	vw, vargs := f.visitor("w")
	args = append(append(args, f.FromDay), vargs...)
	var out []CalcTotalsRow
	err := database.Conn(ctx, r.db).Raw(`WITH `+cte+`
		SELECT pv.dim AS key,
		       count(*) AS visitors,
		       count(*) FILTER (WHERE pv.signed_in) AS signed_in_visitors,
		       count(*) FILTER (WHERE `+existingCond+`) AS existing_users,
		       count(*) FILTER (WHERE pv.tried) AS tried,
		       count(*) FILTER (WHERE pv.completed) AS completed,
		       count(*) FILTER (WHERE `+signupCond+`) AS signups
		FROM pv LEFT JOIN web_visitors w ON w.anonymous_id = pv.anonymous_id`+seenInWindow+`
		WHERE `+vw+` GROUP BY pv.dim ORDER BY pv.dim`, args...).Scan(&out).Error
	return out, err
}

// Totals returns the headline counts for the window and filters.
func (r *GormCalculatorRepository) Totals(ctx context.Context, f CalculatorFilter) (CalcTotalsRow, error) {
	rows, err := r.dimRows(ctx, f, "''::text")
	if err != nil || len(rows) == 0 {
		return CalcTotalsRow{}, err
	}
	return rows[0], nil
}

// ByDimension returns the counts per calculator kind or per locale.
func (r *GormCalculatorRepository) ByDimension(ctx context.Context, f CalculatorFilter, dim string) ([]CalcTotalsRow, error) {
	switch dim {
	case "kind":
		return r.dimRows(ctx, f, "cv.kind")
	case "locale":
		return r.dimRows(ctx, f, "cv.locale")
	}
	return nil, fmt.Errorf("repository: invalid calculator dimension %q", dim)
}

// BySource returns the top utm_source/medium/campaign groups by visitors.
func (r *GormCalculatorRepository) BySource(ctx context.Context, f CalculatorFilter, limit int) ([]CalcSourceRow, error) {
	cte, args := perVisitor(f, "''::text")
	vw, vargs := f.visitor("w")
	args = append(append(args, f.FromDay), vargs...)
	args = append(args, limit)
	var out []CalcSourceRow
	err := database.Conn(ctx, r.db).Raw(`WITH `+cte+`
		SELECT COALESCE(w.utm_source, '') AS source, COALESCE(w.utm_medium, '') AS medium, COALESCE(w.utm_campaign, '') AS campaign,
		       count(*) AS visitors, count(*) FILTER (WHERE pv.tried) AS tried, count(*) FILTER (WHERE `+signupCond+`) AS signups
		FROM pv LEFT JOIN web_visitors w ON w.anonymous_id = pv.anonymous_id`+seenInWindow+`
		WHERE `+vw+` GROUP BY 1, 2, 3 ORDER BY visitors DESC, source, medium, campaign LIMIT ?`, args...).Scan(&out).Error
	return out, err
}

// ByLink returns the top short links by visitors.
func (r *GormCalculatorRepository) ByLink(ctx context.Context, f CalculatorFilter, limit int) ([]CalcLinkRow, error) {
	cte, args := perVisitor(f, "''::text")
	vw, vargs := f.visitor("w")
	args = append(append(args, f.FromDay), vargs...)
	args = append(args, limit)
	var out []CalcLinkRow
	err := database.Conn(ctx, r.db).Raw(`WITH `+cte+`
		SELECT sl.id AS link_id, sl.code AS code, sl.label AS label, COALESCE(c.name, '') AS campaign_name,
		       count(*) AS visitors, count(*) FILTER (WHERE pv.tried) AS tried, count(*) FILTER (WHERE `+signupCond+`) AS signups
		FROM pv JOIN web_visitors w ON w.anonymous_id = pv.anonymous_id AND w.link_id IS NOT NULL`+seenInWindow+`
		JOIN short_links sl ON sl.id = w.link_id LEFT JOIN campaigns c ON c.id = sl.campaign_id
		WHERE `+vw+` GROUP BY sl.id, sl.code, sl.label, c.name ORDER BY visitors DESC, sl.code LIMIT ?`, args...).Scan(&out).Error
	return out, err
}

var calcUnits = map[string]string{"day": "day", "week": "week", "month": "month"}

// Series returns per-bucket distinct visitor counts (buckets without data are
// omitted; the service fills them).
func (r *GormCalculatorRepository) Series(ctx context.Context, f CalculatorFilter, unit string) ([]CalcBucketRow, error) {
	u, ok := calcUnits[unit]
	if !ok {
		return nil, fmt.Errorf("repository: invalid interval %q", unit)
	}
	vis, vargs := f.visits("cv")
	join, where := "", "TRUE"
	var jargs []any
	if f.hasVisitorFilter() {
		w, a := f.visitor("w")
		join, where, jargs = " LEFT JOIN web_visitors w ON w.anonymous_id = cv.anonymous_id", w, a
	}
	// unit comes from the allow-list above.
	args := append(append([]any{}, vargs...), jargs...)
	var out []CalcBucketRow
	err := database.Conn(ctx, r.db).Raw(`WITH pb AS (
			SELECT date_trunc('`+u+`', cv.day::timestamp) AS bucket, cv.anonymous_id,
			       bool_or(cv.started > 0) AS tried, bool_or(cv.completed > 0) AS completed
			FROM calculator_visits cv`+join+` WHERE `+vis+` AND `+where+` GROUP BY 1, 2)
		SELECT bucket, count(*) AS visitors, count(*) FILTER (WHERE tried) AS tried, count(*) FILTER (WHERE completed) AS completed
		FROM pb GROUP BY bucket ORDER BY bucket`, args...).Scan(&out).Error
	return out, err
}

// SignupSeries returns the sign-ups per bucket (in Signups): a sign-up counts
// in the bucket of the visitor's first visit in the window.
func (r *GormCalculatorRepository) SignupSeries(ctx context.Context, f CalculatorFilter, unit string) ([]CalcBucketRow, error) {
	u, ok := calcUnits[unit]
	if !ok {
		return nil, fmt.Errorf("repository: invalid interval %q", unit)
	}
	cte, args := perVisitor(f, "''::text")
	vw, wargs := f.visitor("w")
	args = append(append(args, f.FromDay), wargs...)
	var out []CalcBucketRow
	err := database.Conn(ctx, r.db).Raw(`WITH `+cte+`
		SELECT date_trunc('`+u+`', pv.first_day::timestamp) AS bucket, count(*) AS signups
		FROM pv JOIN web_visitors w ON w.anonymous_id = pv.anonymous_id`+seenInWindow+`
		WHERE `+signupCond+` AND `+vw+` GROUP BY 1 ORDER BY 1`, args...).Scan(&out).Error
	return out, err
}

// Conversions lists sign-ups (newest first) of visitors whose first visit in
// the window preceded the account, keyset-paginated on (signed_up_at, user).
func (r *GormCalculatorRepository) Conversions(ctx context.Context, f CalculatorFilter, cursor *Cursor, limit int) ([]CalcConversionRow, error) {
	vis, args := f.visits("cv")
	vw, vargs := f.visitor("w")
	where := "w.signed_up_at IS NOT NULL AND " + vw
	args = append(args, vargs...)
	if cursor != nil {
		uid, err := uuid.Parse(cursor.ID)
		if err != nil {
			return nil, ErrInvalidCursor
		}
		where += " AND (w.signed_up_at, w.user_id) < (?, ?::uuid)"
		args = append(args, cursor.At, uid)
	}
	args = append(args, limit)
	type row struct {
		UserID        uuid.UUID
		Email         string
		DisplayName   string
		Role          string
		Status        string
		UserCreatedAt time.Time
		FirstVisitAt  time.Time
		SignedUpAt    time.Time
		Kinds         string
		Locales       string
		Tried         bool
		UTMSource     string
		UTMMedium     string
		UTMCampaign   string
		LinkCode      string
		CampaignName  string
	}
	var rows []row
	// The candidates are picked (newest first, one row per user) before the
	// accounts are joined, so only a page of users is ever looked up.
	err := database.Conn(ctx, r.db).Raw(`WITH cand AS (
		SELECT DISTINCT ON (w.signed_up_at, w.user_id)
		       w.user_id, w.signed_up_at, w.link_id, w.campaign_id, w.utm_source, w.utm_medium, w.utm_campaign,
		       GREATEST(w.first_seen_at, (f.first_day::timestamp AT TIME ZONE 'UTC')) AS first_visit_at,
		       f.kinds, f.locales, f.tried
		FROM web_visitors w
		JOIN LATERAL (
		    SELECT min(cv.day) AS first_day, string_agg(DISTINCT cv.kind, ',') AS kinds, string_agg(DISTINCT cv.locale, ',') AS locales,
		           bool_or(cv.started > 0) AS tried
		    FROM calculator_visits cv WHERE cv.anonymous_id = w.anonymous_id AND `+vis+`
		) f ON f.first_day IS NOT NULL
		WHERE `+where+` AND w.signed_up_at >= GREATEST(w.first_seen_at, (f.first_day::timestamp AT TIME ZONE 'UTC'))
		ORDER BY w.signed_up_at DESC, w.user_id DESC, w.first_seen_at
		LIMIT ?)
		SELECT u.id AS user_id, u.email AS email, u.display_name AS display_name, u.role::text AS role, u.status::text AS status,
		       u.created_at AS user_created_at, c.first_visit_at, c.signed_up_at, c.kinds, c.locales, c.tried,
		       c.utm_source, c.utm_medium, c.utm_campaign, COALESCE(sl.code, '') AS link_code, COALESCE(cp.name, '') AS campaign_name
		FROM cand c
		JOIN users u ON u.id = c.user_id
		LEFT JOIN short_links sl ON sl.id = c.link_id
		LEFT JOIN campaigns cp ON cp.id = c.campaign_id
		ORDER BY c.signed_up_at DESC, c.user_id DESC`, args...).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]CalcConversionRow, 0, len(rows))
	for _, x := range rows {
		out = append(out, CalcConversionRow{UserID: x.UserID, Email: x.Email, DisplayName: x.DisplayName, Role: x.Role, Status: x.Status,
			UserCreatedAt: x.UserCreatedAt, FirstVisitAt: x.FirstVisitAt.UTC(), SignedUpAt: x.SignedUpAt.UTC(),
			Kinds: splitCSV(x.Kinds), Locales: splitCSV(x.Locales), Tried: x.Tried, UTMSource: x.UTMSource, UTMMedium: x.UTMMedium,
			UTMCampaign: x.UTMCampaign, LinkCode: x.LinkCode, CampaignName: x.CampaignName})
	}
	return out, nil
}

func splitCSV(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, ",")
}

// PurgeForUser deletes the web visitors linked to the user and their
// calculator rows (account erasure).
func (r *GormCalculatorRepository) PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	db := database.Conn(ctx, r.db)
	res := db.Exec(`DELETE FROM calculator_visits WHERE anonymous_id IN (SELECT anonymous_id FROM web_visitors WHERE user_id = ?)`, userID)
	if res.Error != nil {
		return 0, res.Error
	}
	n := res.RowsAffected
	res = db.Exec(`DELETE FROM web_visitors WHERE user_id = ?`, userID)
	return n + res.RowsAffected, res.Error
}

// DeleteOlderThan removes roll-up rows not seen since cutoff, in bounded
// chunks so the job never holds long locks.
func (r *GormCalculatorRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	const chunk = 20000
	var total int64
	for _, q := range []struct {
		table, col string
		arg        any
	}{
		{"calculator_visits", "day", time.Date(cutoff.Year(), cutoff.Month(), cutoff.Day(), 0, 0, 0, 0, time.UTC)},
		{"web_visitors", "last_seen_at", cutoff},
	} {
		for {
			res := database.Conn(ctx, r.db).Exec(fmt.Sprintf(
				`DELETE FROM %s WHERE ctid IN (SELECT ctid FROM %s WHERE %s < ? LIMIT %d)`, q.table, q.table, q.col, chunk), q.arg)
			if res.Error != nil {
				return total, res.Error
			}
			total += res.RowsAffected
			if res.RowsAffected < chunk {
				break
			}
		}
	}
	return total, nil
}
