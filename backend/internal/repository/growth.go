package repository

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// LinkFilter narrows short link listings.
type LinkFilter struct {
	CampaignID *uuid.UUID
	Query      string
	Active     *bool
	Cursor     *Cursor
	Limit      int
}

// GrowthScope selects the links whose traffic a report covers. With both
// fields nil the report covers every link (and every attributed sign-up).
type GrowthScope struct {
	LinkID     *uuid.UUID
	CampaignID *uuid.UUID
}

func (s GrowthScope) clicks(alias string) (string, []any) {
	switch {
	case s.LinkID != nil:
		return alias + ".link_id = ?", []any{*s.LinkID}
	case s.CampaignID != nil:
		return alias + ".campaign_id = ?", []any{*s.CampaignID}
	}
	return "TRUE", nil
}

func (s GrowthScope) attributions(alias string) (string, []any) {
	switch {
	case s.LinkID != nil:
		return alias + ".link_id = ?", []any{*s.LinkID}
	case s.CampaignID != nil:
		return alias + ".campaign_id = ?", []any{*s.CampaignID}
	}
	return "(" + alias + ".link_id IS NOT NULL OR " + alias + ".campaign_id IS NOT NULL OR " + alias + ".utm_source <> '')", nil
}

// GrowthTotalsRow holds raw funnel counts.
type GrowthTotalsRow struct {
	Clicks       int64
	UniqueClicks int64
	BotClicks    int64
	Signups      int64
	Purchasers   int64
	Purchases    int64
	RevenueCents int64
}

// BucketCounts is one series bucket.
type BucketCounts struct {
	Bucket       time.Time
	Clicks       int64
	UniqueClicks int64
	Signups      int64
	Purchasers   int64
	RevenueCents int64
}

// LinkCounts is the funnel of one link.
type LinkCounts struct {
	LinkID       uuid.UUID
	Code         string
	Label        string
	Clicks       int64
	UniqueClicks int64
	Signups      int64
	Purchasers   int64
	RevenueCents int64
}

// SourceCounts is the funnel of one utm_source / utm_medium pair.
type SourceCounts struct {
	Source       string
	Medium       string
	Clicks       int64
	Signups      int64
	Purchasers   int64
	RevenueCents int64
}

// CampaignCounts is the funnel of one campaign.
type CampaignCounts struct {
	CampaignID   uuid.UUID
	Clicks       int64
	UniqueClicks int64
	Signups      int64
	Purchasers   int64
	RevenueCents int64
}

// KeyCount is a generic grouped count.
type KeyCount struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

// GrowthRepository persists campaigns, short links, clicks and attribution
// and computes acquisition funnels.
type GrowthRepository interface {
	CreateCampaign(ctx context.Context, c *domain.Campaign) error
	GetCampaign(ctx context.Context, id uuid.UUID) (*domain.Campaign, error)
	GetCampaignBySlug(ctx context.Context, slug string) (*domain.Campaign, error)
	UpdateCampaign(ctx context.Context, id uuid.UUID, fields map[string]any) error
	DeleteCampaign(ctx context.Context, id uuid.UUID) error
	ListCampaigns(ctx context.Context, status string) ([]domain.Campaign, error)
	CampaignInUse(ctx context.Context, id uuid.UUID) (bool, error)
	CampaignNames(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)

	CreateLink(ctx context.Context, l *domain.ShortLink) error
	GetLink(ctx context.Context, id uuid.UUID) (*domain.ShortLink, error)
	GetLinkByCode(ctx context.Context, code string) (*domain.ShortLink, error)
	UpdateLink(ctx context.Context, id uuid.UUID, fields map[string]any) error
	DeleteLink(ctx context.Context, id uuid.UUID) error
	ListLinks(ctx context.Context, f LinkFilter) ([]domain.ShortLink, error)
	LinkCodes(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)

	RecordClick(ctx context.Context, c *domain.LinkClick) error
	GetClick(ctx context.Context, id uuid.UUID) (*domain.LinkClick, error)

	CreateAttribution(ctx context.Context, a *domain.Attribution) error
	GetAttribution(ctx context.Context, userID uuid.UUID) (*domain.Attribution, error)

	Totals(ctx context.Context, scope GrowthScope, from, to time.Time, currency string) (GrowthTotalsRow, error)
	Series(ctx context.Context, scope GrowthScope, from, to time.Time, unit, currency string) ([]BucketCounts, error)
	ByLink(ctx context.Context, scope GrowthScope, from, to time.Time, currency string, limit int) ([]LinkCounts, error)
	BySource(ctx context.Context, scope GrowthScope, from, to time.Time, currency string, limit int) ([]SourceCounts, error)
	ByCampaign(ctx context.Context, from, to time.Time, currency string) ([]CampaignCounts, error)
	Devices(ctx context.Context, scope GrowthScope, from, to time.Time) ([]KeyCount, error)
	Referrers(ctx context.Context, scope GrowthScope, from, to time.Time, limit int) ([]KeyCount, error)
	OrganicSignups(ctx context.Context, from, to time.Time) (int64, error)
}

// GormGrowthRepository implements GrowthRepository.
type GormGrowthRepository struct{ db *gorm.DB }

// NewGrowthRepository returns a GORM growth repository.
func NewGrowthRepository(db *gorm.DB) *GormGrowthRepository { return &GormGrowthRepository{db: db} }

func (r *GormGrowthRepository) CreateCampaign(ctx context.Context, c *domain.Campaign) error {
	if err := database.Conn(ctx, r.db).Create(c).Error; err != nil {
		if isUnique(err, "campaigns_slug_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *GormGrowthRepository) GetCampaign(ctx context.Context, id uuid.UUID) (*domain.Campaign, error) {
	var c domain.Campaign
	if err := database.Conn(ctx, r.db).First(&c, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &c, nil
}

func (r *GormGrowthRepository) GetCampaignBySlug(ctx context.Context, slug string) (*domain.Campaign, error) {
	var c domain.Campaign
	if err := database.Conn(ctx, r.db).First(&c, "slug = ?", slug).Error; err != nil {
		return nil, notFound(err)
	}
	return &c, nil
}

func (r *GormGrowthRepository) UpdateCampaign(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	fields["updated_at"] = time.Now().UTC()
	res := database.Conn(ctx, r.db).Model(&domain.Campaign{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		if isUnique(res.Error, "campaigns_slug_key") {
			return ErrDuplicate
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormGrowthRepository) DeleteCampaign(ctx context.Context, id uuid.UUID) error {
	res := database.Conn(ctx, r.db).Where("id = ?", id).Delete(&domain.Campaign{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormGrowthRepository) ListCampaigns(ctx context.Context, status string) ([]domain.Campaign, error) {
	q := database.Conn(ctx, r.db).Model(&domain.Campaign{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var out []domain.Campaign
	err := q.Order("created_at DESC, id DESC").Limit(500).Find(&out).Error
	return out, err
}

func (r *GormGrowthRepository) CampaignInUse(ctx context.Context, id uuid.UUID) (bool, error) {
	var used bool
	err := database.Conn(ctx, r.db).Raw(`SELECT EXISTS (SELECT 1 FROM short_links WHERE campaign_id = ?)
		OR EXISTS (SELECT 1 FROM user_attributions WHERE campaign_id = ?)
		OR EXISTS (SELECT 1 FROM link_clicks WHERE campaign_id = ?)
		OR EXISTS (SELECT 1 FROM broadcasts WHERE campaign_id = ?)`, id, id, id, id).Scan(&used).Error
	return used, err
}

func (r *GormGrowthRepository) CampaignNames(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []domain.Campaign
	if err := database.Conn(ctx, r.db).Select("id, name").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, c := range rows {
		out[c.ID] = c.Name
	}
	return out, nil
}

func (r *GormGrowthRepository) CreateLink(ctx context.Context, l *domain.ShortLink) error {
	if err := database.Conn(ctx, r.db).Create(l).Error; err != nil {
		if isUnique(err, "short_links_code_key") {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *GormGrowthRepository) GetLink(ctx context.Context, id uuid.UUID) (*domain.ShortLink, error) {
	var l domain.ShortLink
	if err := database.Conn(ctx, r.db).First(&l, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &l, nil
}

func (r *GormGrowthRepository) GetLinkByCode(ctx context.Context, code string) (*domain.ShortLink, error) {
	var l domain.ShortLink
	if err := database.Conn(ctx, r.db).First(&l, "lower(code) = lower(?)", code).Error; err != nil {
		return nil, notFound(err)
	}
	return &l, nil
}

func (r *GormGrowthRepository) UpdateLink(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	fields["updated_at"] = time.Now().UTC()
	res := database.Conn(ctx, r.db).Model(&domain.ShortLink{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormGrowthRepository) DeleteLink(ctx context.Context, id uuid.UUID) error {
	res := database.Conn(ctx, r.db).Where("id = ?", id).Delete(&domain.ShortLink{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormGrowthRepository) ListLinks(ctx context.Context, f LinkFilter) ([]domain.ShortLink, error) {
	q := database.Conn(ctx, r.db).Model(&domain.ShortLink{})
	if f.CampaignID != nil {
		q = q.Where("campaign_id = ?", *f.CampaignID)
	}
	if f.Active != nil {
		q = q.Where("active = ?", *f.Active)
	}
	if f.Query != "" {
		like := "%" + escapeLike(strings.ToLower(f.Query)) + "%"
		q = q.Where("(lower(code) LIKE ? ESCAPE '\\' OR lower(label) LIKE ? ESCAPE '\\' OR lower(destination) LIKE ? ESCAPE '\\')", like, like, like)
	}
	q = applyCursor(q, f.Cursor, "created_at", "id")
	var out []domain.ShortLink
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(f.Limit, 50, 200)).Find(&out).Error
	return out, err
}

func (r *GormGrowthRepository) LinkCodes(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []domain.ShortLink
	if err := database.Conn(ctx, r.db).Select("id, code").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, l := range rows {
		out[l.ID] = l.Code
	}
	return out, nil
}

// RecordClick stores a click and bumps the link's denormalised counter.
func (r *GormGrowthRepository) RecordClick(ctx context.Context, c *domain.LinkClick) error {
	conn := database.Conn(ctx, r.db)
	if err := conn.Create(c).Error; err != nil {
		return err
	}
	if c.IsBot {
		return nil
	}
	return conn.Model(&domain.ShortLink{}).Where("id = ?", c.LinkID).UpdateColumn("clicks", gorm.Expr("clicks + 1")).Error
}

func (r *GormGrowthRepository) GetClick(ctx context.Context, id uuid.UUID) (*domain.LinkClick, error) {
	var c domain.LinkClick
	if err := database.Conn(ctx, r.db).First(&c, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &c, nil
}

func (r *GormGrowthRepository) CreateAttribution(ctx context.Context, a *domain.Attribution) error {
	return database.Conn(ctx, r.db).Create(a).Error
}

func (r *GormGrowthRepository) GetAttribution(ctx context.Context, userID uuid.UUID) (*domain.Attribution, error) {
	var a domain.Attribution
	if err := database.Conn(ctx, r.db).First(&a, "user_id = ?", userID).Error; err != nil {
		return nil, notFound(err)
	}
	return &a, nil
}

func (r *GormGrowthRepository) Totals(ctx context.Context, scope GrowthScope, from, to time.Time, currency string) (GrowthTotalsRow, error) {
	cw, cargs := scope.clicks("c")
	aw, aargs := scope.attributions("a")
	args := []any{}
	args = append(append(args, from, to), cargs...)
	args = append(append(args, from, to), cargs...)
	args = append(append(args, from, to), cargs...)
	args = append(append(args, from, to), aargs...)
	args = append(append(args, from, to), aargs...)
	args = append(append(args, from, to), aargs...)
	args = append(append(args, from, to, currency), aargs...)
	var row GrowthTotalsRow
	err := database.Conn(ctx, r.db).Raw(`SELECT
		(SELECT count(*) FROM link_clicks c WHERE NOT c.is_bot AND c.created_at >= ? AND c.created_at < ? AND `+cw+`) AS clicks,
		(SELECT count(DISTINCT c.visitor_hash) FROM link_clicks c WHERE NOT c.is_bot AND c.created_at >= ? AND c.created_at < ? AND `+cw+`) AS unique_clicks,
		(SELECT count(*) FROM link_clicks c WHERE c.is_bot AND c.created_at >= ? AND c.created_at < ? AND `+cw+`) AS bot_clicks,
		(SELECT count(*) FROM user_attributions a JOIN users u ON u.id = a.user_id
		   WHERE u.created_at >= ? AND u.created_at < ? AND `+aw+`) AS signups,
		(SELECT count(DISTINCT p.user_id) FROM credit_purchases p JOIN user_attributions a ON a.user_id = p.user_id
		   WHERE p.status = 'succeeded' AND p.completed_at >= ? AND p.completed_at < ? AND `+aw+`) AS purchasers,
		(SELECT count(*) FROM credit_purchases p JOIN user_attributions a ON a.user_id = p.user_id
		   WHERE p.status = 'succeeded' AND p.completed_at >= ? AND p.completed_at < ? AND `+aw+`) AS purchases,
		(SELECT COALESCE(sum(p.price_cents), 0) FROM credit_purchases p JOIN user_attributions a ON a.user_id = p.user_id
		   WHERE p.status = 'succeeded' AND p.completed_at >= ? AND p.completed_at < ? AND p.currency = ? AND `+aw+`) AS revenue_cents`,
		args...).Scan(&row).Error
	return row, err
}

// bucketExpr truncates a timestamptz column in UTC. unit comes from an
// allow-list (validIntervals), never directly from user input.
func bucketExpr(unit, col string) string {
	return "date_trunc('" + unit + "', " + col + " AT TIME ZONE 'UTC')"
}

func (r *GormGrowthRepository) Series(ctx context.Context, scope GrowthScope, from, to time.Time, unit, currency string) ([]BucketCounts, error) {
	if _, ok := validIntervals[unit]; !ok {
		unit = "day"
	}
	cw, cargs := scope.clicks("c")
	aw, aargs := scope.attributions("a")
	conn := database.Conn(ctx, r.db)
	merged := map[time.Time]*BucketCounts{}
	get := func(t time.Time) *BucketCounts {
		t = t.UTC()
		if b, ok := merged[t]; ok {
			return b
		}
		b := &BucketCounts{Bucket: t}
		merged[t] = b
		return b
	}
	var clicks []struct {
		Bucket       time.Time
		Clicks       int64
		UniqueClicks int64
	}
	if err := conn.Raw(`SELECT `+bucketExpr(unit, "c.created_at")+` AS bucket, count(*) AS clicks, count(DISTINCT c.visitor_hash) AS unique_clicks
		FROM link_clicks c WHERE NOT c.is_bot AND c.created_at >= ? AND c.created_at < ? AND `+cw+` GROUP BY 1`,
		append([]any{from, to}, cargs...)...).Scan(&clicks).Error; err != nil {
		return nil, err
	}
	for _, c := range clicks {
		b := get(c.Bucket)
		b.Clicks, b.UniqueClicks = c.Clicks, c.UniqueClicks
	}
	var signups []struct {
		Bucket time.Time
		N      int64
	}
	if err := conn.Raw(`SELECT `+bucketExpr(unit, "u.created_at")+` AS bucket, count(*) AS n
		FROM user_attributions a JOIN users u ON u.id = a.user_id WHERE u.created_at >= ? AND u.created_at < ? AND `+aw+` GROUP BY 1`,
		append([]any{from, to}, aargs...)...).Scan(&signups).Error; err != nil {
		return nil, err
	}
	for _, s := range signups {
		get(s.Bucket).Signups = s.N
	}
	var purchases []struct {
		Bucket     time.Time
		Purchasers int64
		Revenue    int64
	}
	if err := conn.Raw(`SELECT `+bucketExpr(unit, "p.completed_at")+` AS bucket, count(DISTINCT p.user_id) AS purchasers,
		COALESCE(sum(p.price_cents) FILTER (WHERE p.currency = ?), 0) AS revenue
		FROM credit_purchases p JOIN user_attributions a ON a.user_id = p.user_id
		WHERE p.status = 'succeeded' AND p.completed_at >= ? AND p.completed_at < ? AND `+aw+` GROUP BY 1`,
		append([]any{currency, from, to}, aargs...)...).Scan(&purchases).Error; err != nil {
		return nil, err
	}
	for _, p := range purchases {
		b := get(p.Bucket)
		b.Purchasers, b.RevenueCents = p.Purchasers, p.Revenue
	}
	out := make([]BucketCounts, 0, len(merged))
	for _, t := range Buckets(from, to, unit) {
		if b, ok := merged[t]; ok {
			out = append(out, *b)
		} else {
			out = append(out, BucketCounts{Bucket: t})
		}
	}
	return out, nil
}

// Buckets lists the UTC bucket starts covering [from, to).
func Buckets(from, to time.Time, unit string) []time.Time {
	return BucketsIn(from, to, unit, time.UTC)
}

// BucketsIn lists bucket starts (as wall-clock times in loc, returned with
// UTC location to match "timestamp without time zone" scans) covering
// [from, to).
func BucketsIn(from, to time.Time, unit string, loc *time.Location) []time.Time {
	start := truncate(from.In(loc), unit)
	end := to.In(loc)
	var out []time.Time
	for t := start; t.Before(end) && len(out) < 5000; t = next(t, unit) {
		out = append(out, time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.UTC))
	}
	return out
}

func truncate(t time.Time, unit string) time.Time {
	switch unit {
	case "hour":
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	case "week":
		d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		offset := (int(d.Weekday()) + 6) % 7 // ISO weeks start on Monday, like date_trunc
		return d.AddDate(0, 0, -offset)
	case "month":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func next(t time.Time, unit string) time.Time {
	switch unit {
	case "hour":
		return t.Add(time.Hour)
	case "week":
		return t.AddDate(0, 0, 7)
	case "month":
		return t.AddDate(0, 1, 0)
	}
	return t.AddDate(0, 0, 1)
}

func (r *GormGrowthRepository) ByLink(ctx context.Context, scope GrowthScope, from, to time.Time, currency string, limit int) ([]LinkCounts, error) {
	lw, largs := "TRUE", []any(nil)
	switch {
	case scope.LinkID != nil:
		lw, largs = "l.id = ?", []any{*scope.LinkID}
	case scope.CampaignID != nil:
		lw, largs = "l.campaign_id = ?", []any{*scope.CampaignID}
	}
	// Arguments in placeholder order: clicks, unique clicks, purchases
	// (currency first), sign-ups, link scope, limit.
	args := []any{from, to, from, to, currency, from, to, from, to}
	args = append(args, largs...)
	args = append(args, clampLimit(limit, 50, 500))
	var out []LinkCounts
	err := database.Conn(ctx, r.db).Raw(`SELECT l.id AS link_id, l.code, l.label,
		(SELECT count(*) FROM link_clicks c WHERE c.link_id = l.id AND NOT c.is_bot AND c.created_at >= ? AND c.created_at < ?) AS clicks,
		(SELECT count(DISTINCT c.visitor_hash) FROM link_clicks c WHERE c.link_id = l.id AND NOT c.is_bot AND c.created_at >= ? AND c.created_at < ?) AS unique_clicks,
		COALESCE(p.purchasers, 0) AS purchasers, COALESCE(p.revenue, 0) AS revenue_cents,
		COALESCE(s.signups, 0) AS signups
		FROM short_links l
		LEFT JOIN LATERAL (SELECT count(DISTINCT pp.user_id) AS purchasers, COALESCE(sum(pp.price_cents) FILTER (WHERE pp.currency = ?), 0) AS revenue
		   FROM credit_purchases pp JOIN user_attributions a ON a.user_id = pp.user_id
		   WHERE a.link_id = l.id AND pp.status = 'succeeded' AND pp.completed_at >= ? AND pp.completed_at < ?) p ON TRUE
		LEFT JOIN LATERAL (SELECT count(*) AS signups FROM user_attributions a JOIN users u ON u.id = a.user_id
		   WHERE a.link_id = l.id AND u.created_at >= ? AND u.created_at < ?) s ON TRUE
		WHERE `+lw+`
		ORDER BY clicks DESC, l.created_at DESC LIMIT ?`, args...).Scan(&out).Error
	return out, err
}

func (r *GormGrowthRepository) BySource(ctx context.Context, scope GrowthScope, from, to time.Time, currency string, limit int) ([]SourceCounts, error) {
	cw, cargs := scope.clicks("c")
	aw, aargs := scope.attributions("a")
	conn := database.Conn(ctx, r.db)
	type key struct{ source, medium string }
	merged := map[key]*SourceCounts{}
	get := func(s, m string) *SourceCounts {
		if s == "" {
			s = "direct"
		}
		k := key{s, m}
		if v, ok := merged[k]; ok {
			return v
		}
		v := &SourceCounts{Source: s, Medium: m}
		merged[k] = v
		return v
	}
	var clicks []struct {
		Source, Medium string
		N              int64
	}
	if err := conn.Raw(`SELECT l.utm_source AS source, l.utm_medium AS medium, count(*) AS n
		FROM link_clicks c JOIN short_links l ON l.id = c.link_id
		WHERE NOT c.is_bot AND c.created_at >= ? AND c.created_at < ? AND `+cw+` GROUP BY 1, 2`,
		append([]any{from, to}, cargs...)...).Scan(&clicks).Error; err != nil {
		return nil, err
	}
	for _, c := range clicks {
		get(c.Source, c.Medium).Clicks = c.N
	}
	var signups []struct {
		Source, Medium string
		N              int64
	}
	if err := conn.Raw(`SELECT a.utm_source AS source, a.utm_medium AS medium, count(*) AS n
		FROM user_attributions a JOIN users u ON u.id = a.user_id
		WHERE u.created_at >= ? AND u.created_at < ? AND `+aw+` GROUP BY 1, 2`,
		append([]any{from, to}, aargs...)...).Scan(&signups).Error; err != nil {
		return nil, err
	}
	for _, s := range signups {
		get(s.Source, s.Medium).Signups = s.N
	}
	var purchases []struct {
		Source, Medium string
		Purchasers     int64
		Revenue        int64
	}
	if err := conn.Raw(`SELECT a.utm_source AS source, a.utm_medium AS medium, count(DISTINCT p.user_id) AS purchasers,
		COALESCE(sum(p.price_cents) FILTER (WHERE p.currency = ?), 0) AS revenue
		FROM credit_purchases p JOIN user_attributions a ON a.user_id = p.user_id
		WHERE p.status = 'succeeded' AND p.completed_at >= ? AND p.completed_at < ? AND `+aw+` GROUP BY 1, 2`,
		append([]any{currency, from, to}, aargs...)...).Scan(&purchases).Error; err != nil {
		return nil, err
	}
	for _, p := range purchases {
		v := get(p.Source, p.Medium)
		v.Purchasers, v.RevenueCents = p.Purchasers, p.Revenue
	}
	out := make([]SourceCounts, 0, len(merged))
	for _, v := range merged {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Signups != b.Signups {
			return a.Signups > b.Signups
		}
		if a.Clicks != b.Clicks {
			return a.Clicks > b.Clicks
		}
		return a.Source+a.Medium < b.Source+b.Medium
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *GormGrowthRepository) ByCampaign(ctx context.Context, from, to time.Time, currency string) ([]CampaignCounts, error) {
	var out []CampaignCounts
	err := database.Conn(ctx, r.db).Raw(`SELECT c.id AS campaign_id,
		(SELECT count(*) FROM link_clicks k WHERE k.campaign_id = c.id AND NOT k.is_bot AND k.created_at >= ? AND k.created_at < ?) AS clicks,
		(SELECT count(DISTINCT k.visitor_hash) FROM link_clicks k WHERE k.campaign_id = c.id AND NOT k.is_bot AND k.created_at >= ? AND k.created_at < ?) AS unique_clicks,
		(SELECT count(*) FROM user_attributions a JOIN users u ON u.id = a.user_id WHERE a.campaign_id = c.id AND u.created_at >= ? AND u.created_at < ?) AS signups,
		(SELECT count(DISTINCT p.user_id) FROM credit_purchases p JOIN user_attributions a ON a.user_id = p.user_id
		   WHERE a.campaign_id = c.id AND p.status = 'succeeded' AND p.completed_at >= ? AND p.completed_at < ?) AS purchasers,
		(SELECT COALESCE(sum(p.price_cents), 0) FROM credit_purchases p JOIN user_attributions a ON a.user_id = p.user_id
		   WHERE a.campaign_id = c.id AND p.status = 'succeeded' AND p.currency = ? AND p.completed_at >= ? AND p.completed_at < ?) AS revenue_cents
		FROM campaigns c`, from, to, from, to, from, to, from, to, currency, from, to).Scan(&out).Error
	return out, err
}

func (r *GormGrowthRepository) Devices(ctx context.Context, scope GrowthScope, from, to time.Time) ([]KeyCount, error) {
	cw, cargs := scope.clicks("c")
	var out []KeyCount
	err := database.Conn(ctx, r.db).Raw(`SELECT c.device AS key, count(*) AS count FROM link_clicks c
		WHERE NOT c.is_bot AND c.created_at >= ? AND c.created_at < ? AND `+cw+` GROUP BY 1 ORDER BY 2 DESC`,
		append([]any{from, to}, cargs...)...).Scan(&out).Error
	return out, err
}

func (r *GormGrowthRepository) Referrers(ctx context.Context, scope GrowthScope, from, to time.Time, limit int) ([]KeyCount, error) {
	cw, cargs := scope.clicks("c")
	var out []KeyCount
	args := append([]any{from, to}, cargs...)
	args = append(args, clampLimit(limit, 10, 50))
	err := database.Conn(ctx, r.db).Raw(`SELECT CASE WHEN c.referrer_host = '' THEN 'direct' ELSE c.referrer_host END AS key, count(*) AS count
		FROM link_clicks c WHERE NOT c.is_bot AND c.created_at >= ? AND c.created_at < ? AND `+cw+`
		GROUP BY 1 ORDER BY 2 DESC LIMIT ?`, args...).Scan(&out).Error
	return out, err
}

func (r *GormGrowthRepository) OrganicSignups(ctx context.Context, from, to time.Time) (int64, error) {
	var n int64
	err := database.Conn(ctx, r.db).Raw(`SELECT count(*) FROM users u
		WHERE u.role = 'client' AND u.created_at >= ? AND u.created_at < ?
		AND NOT EXISTS (SELECT 1 FROM user_attributions a WHERE a.user_id = u.id
		                AND (a.link_id IS NOT NULL OR a.campaign_id IS NOT NULL OR a.utm_source <> ''))`, from, to).Scan(&n).Error
	return n, err
}
