package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
)

// deviceExpr classifies a user agent (analytics events).
const deviceExpr = `CASE
	WHEN user_agent ~* '(bot|crawler|spider|preview)' THEN 'bot'
	WHEN user_agent ~* '(ipad|tablet|kindle|silk)' THEN 'tablet'
	WHEN user_agent ~* '(mobi|android|iphone|ipod)' THEN 'mobile'
	WHEN user_agent = '' THEN 'unknown'
	ELSE 'desktop' END`

// UserStats are lifetime aggregates of one account.
type UserStats struct {
	Purchases        int64      `json:"purchases"`
	RevenueCents     int64      `json:"revenue_cents"`
	CreditsPurchased int64      `json:"credits_purchased"`
	CreditsSpent     int64      `json:"credits_spent"`
	Sessions         int64      `json:"sessions"`
	ChatMinutes      int64      `json:"chat_minutes"`
	CustomRequests   int64      `json:"custom_requests"`
	LastActiveAt     *time.Time `json:"last_active_at,omitempty"`
}

// MonthDayCount counts birth dates by calendar day (for zodiac signs).
type MonthDayCount struct {
	Month int
	Day   int
	Count int64
}

// TimeCount is a bucketed count.
type TimeCount struct {
	Bucket time.Time
	Count  int64
}

// CohortCell is one retention cell.
type CohortCell struct {
	Cohort time.Time
	Week   int
	Users  int64
}

// EngagementRow aggregates engagement for a range.
type EngagementRow struct {
	DAU, WAU, MAU          int64
	PayingUsers            int64
	RevenueCents           int64
	RepeatBuyers           int64
	Buyers                 int64
	AvgSessionSeconds      float64
	PaidUserSessions       int64
	ActiveClients          int64
	ChattingClients        int64
	StartedSessionsInRange int64
}

// TopUser ranks a user by a metric.
type TopUser struct {
	UserID uuid.UUID
	Value  int64
}

// PsychicUsage is a client's history with one advisor.
type PsychicUsage struct {
	PsychicID uuid.UUID
	Name      string
	Slug      string
	Sessions  int64
	Minutes   int64
}

// ActivityRow is one timeline entry.
type ActivityRow struct {
	At      time.Time
	Type    string
	Summary string
	RefID   *uuid.UUID
}

// PsychicSessionStats aggregates an advisor's sessions in a range.
type PsychicSessionStats struct {
	Requested          int64
	Accepted           int64
	Declined           int64
	Expired            int64
	Cancelled          int64
	Completed          int64
	Interrupted        int64 // completed sessions ended by a lost connection
	AvgResponseSeconds float64
	AvgDurationSeconds float64
	TotalMinutes       int64
}

// PsychicClientStats aggregates an advisor's clients in a range.
type PsychicClientStats struct {
	Unique    int64
	New       int64
	Returning int64
}

// ClientUsage is a top client of an advisor.
type ClientUsage struct {
	ClientID uuid.UUID
	Name     string
	Sessions int64
	Minutes  int64
}

// EarningBucket is an advisor's earnings per bucket.
type EarningBucket struct {
	Bucket   time.Time
	Net      int64
	Gross    int64
	Sessions int64
}

// HourCell counts requests per ISO weekday (1 = Monday) and hour.
type HourCell struct {
	Weekday int
	Hour    int
	Count   int64
}

// ClientHistory is a client's history with one advisor.
type ClientHistory struct {
	Sessions       int64
	Minutes        int64
	FirstSessionAt *time.Time
	LastSessionAt  *time.Time
}

// InsightsRepository answers audience, engagement and advisor analytics
// queries from the transactional and analytics tables.
type InsightsRepository interface {
	UserStats(ctx context.Context, userID uuid.UUID, currency string) (*UserStats, error)
	CountUsers(ctx context.Context, roleClientOnly bool, from, to *time.Time) (int64, error)
	GroupUsers(ctx context.Context, column string, clientsOnly bool) ([]KeyCount, error)
	AgeBuckets(ctx context.Context, now time.Time) ([]KeyCount, error)
	BirthDays(ctx context.Context) ([]MonthDayCount, error)
	Devices(ctx context.Context, from, to time.Time) ([]KeyCount, error)
	Sources(ctx context.Context) ([]KeyCount, error)
	SignupSeries(ctx context.Context, from, to time.Time, unit string) ([]TimeCount, error)
	ActiveUserSeries(ctx context.Context, from, to time.Time, unit string) ([]TimeCount, error)
	Engagement(ctx context.Context, from, to time.Time, currency string) (*EngagementRow, error)
	Retention(ctx context.Context, since time.Time) (sizes map[time.Time]int64, cells []CohortCell, err error)
	TopUsers(ctx context.Context, metric string, from, to time.Time, currency string, limit int) ([]TopUser, error)
	PsychicsUsedBy(ctx context.Context, clientID uuid.UUID) ([]PsychicUsage, error)
	UserDevices(ctx context.Context, userID uuid.UUID) ([]KeyCount, error)
	Activity(ctx context.Context, userID uuid.UUID, limit int) ([]ActivityRow, error)

	PsychicSessions(ctx context.Context, psychicID uuid.UUID, from, to time.Time) (*PsychicSessionStats, error)
	PsychicClients(ctx context.Context, psychicID uuid.UUID, from, to time.Time) (*PsychicClientStats, error)
	PsychicTopClients(ctx context.Context, psychicID uuid.UUID, from, to time.Time, limit int) ([]ClientUsage, error)
	PsychicEarningsSeries(ctx context.Context, psychicID uuid.UUID, from, to time.Time, unit, tz string) ([]EarningBucket, error)
	PsychicHours(ctx context.Context, psychicID uuid.UUID, from, to time.Time, tz string) ([]HourCell, error)
	ClientHistoryWith(ctx context.Context, clientID, psychicID uuid.UUID) (*ClientHistory, error)
}

// GormInsightsRepository implements InsightsRepository.
type GormInsightsRepository struct{ db *gorm.DB }

// NewInsightsRepository returns a GORM insights repository.
func NewInsightsRepository(db *gorm.DB) *GormInsightsRepository {
	return &GormInsightsRepository{db: db}
}

func (r *GormInsightsRepository) UserStats(ctx context.Context, userID uuid.UUID, currency string) (*UserStats, error) {
	var s UserStats
	err := database.Conn(ctx, r.db).Raw(`SELECT
		(SELECT count(*) FROM credit_purchases WHERE user_id = ? AND status = 'succeeded') AS purchases,
		(SELECT COALESCE(sum(price_cents), 0) FROM credit_purchases WHERE user_id = ? AND status = 'succeeded' AND currency = ?) AS revenue_cents,
		(SELECT COALESCE(sum(credits), 0) FROM credit_purchases WHERE user_id = ? AND status = 'succeeded') AS credits_purchased,
		(SELECT COALESCE(-sum(amount) FILTER (WHERE entry_type = 'chat_charge'), 0) + COALESCE(sum(-held_amount) FILTER (WHERE entry_type = 'hold_release'), 0)
		   FROM credit_ledger WHERE user_id = ?) AS credits_spent,
		(SELECT count(*) FROM chat_sessions WHERE (client_id = ? OR psychic_id = ?) AND started_at IS NOT NULL) AS sessions,
		(SELECT COALESCE(sum(billed_minutes), 0) FROM chat_sessions WHERE client_id = ? OR psychic_id = ?) AS chat_minutes,
		(SELECT count(*) FROM custom_requests WHERE client_id = ? OR psychic_id = ?) AS custom_requests,
		GREATEST((SELECT max(occurred_at) FROM analytics_events WHERE user_id = ?), (SELECT last_login_at FROM users WHERE id = ?)) AS last_active_at`,
		userID, userID, currency, userID, userID, userID, userID, userID, userID, userID, userID, userID, userID).Scan(&s).Error
	return &s, err
}

func (r *GormInsightsRepository) CountUsers(ctx context.Context, clientsOnly bool, from, to *time.Time) (int64, error) {
	q := database.Conn(ctx, r.db).Table("users").Where("status <> 'deleted'")
	if clientsOnly {
		q = q.Where("role = 'client'")
	}
	if from != nil {
		q = q.Where("created_at >= ?", *from)
	}
	if to != nil {
		q = q.Where("created_at < ?", *to)
	}
	var n int64
	err := q.Count(&n).Error
	return n, err
}

// groupColumns is the allow-list of groupable user dimensions.
var groupColumns = map[string]string{
	"role":    "u.role",
	"status":  "u.status",
	"locale":  "u.locale",
	"country": "p.country_code",
}

func (r *GormInsightsRepository) GroupUsers(ctx context.Context, column string, clientsOnly bool) ([]KeyCount, error) {
	col, ok := groupColumns[column]
	if !ok {
		col = "u.role"
	}
	where := "u.status <> 'deleted'"
	if column == "status" {
		where = "TRUE"
	}
	if clientsOnly {
		where += " AND u.role = 'client'"
	}
	var out []KeyCount
	err := database.Conn(ctx, r.db).Raw(`SELECT ` + col + ` AS key, count(*) AS count
		FROM users u LEFT JOIN user_profiles p ON p.user_id = u.id WHERE ` + where + `
		GROUP BY 1 ORDER BY 2 DESC LIMIT 100`).Scan(&out).Error
	return out, err
}

func (r *GormInsightsRepository) AgeBuckets(ctx context.Context, now time.Time) ([]KeyCount, error) {
	var out []KeyCount
	err := database.Conn(ctx, r.db).Raw(`SELECT CASE
		WHEN p.birth_date IS NULL THEN 'unknown'
		WHEN age(?::date, p.birth_date) < interval '25 years' THEN '18-24'
		WHEN age(?::date, p.birth_date) < interval '35 years' THEN '25-34'
		WHEN age(?::date, p.birth_date) < interval '45 years' THEN '35-44'
		WHEN age(?::date, p.birth_date) < interval '55 years' THEN '45-54'
		WHEN age(?::date, p.birth_date) < interval '65 years' THEN '55-64'
		ELSE '65+' END AS key, count(*) AS count
		FROM users u LEFT JOIN user_profiles p ON p.user_id = u.id
		WHERE u.role = 'client' AND u.status <> 'deleted' GROUP BY 1 ORDER BY 1`, now, now, now, now, now).Scan(&out).Error
	return out, err
}

func (r *GormInsightsRepository) BirthDays(ctx context.Context) ([]MonthDayCount, error) {
	var out []MonthDayCount
	err := database.Conn(ctx, r.db).Raw(`SELECT EXTRACT(MONTH FROM p.birth_date)::int AS month, EXTRACT(DAY FROM p.birth_date)::int AS day, count(*) AS count
		FROM users u JOIN user_profiles p ON p.user_id = u.id
		WHERE u.role = 'client' AND u.status <> 'deleted' AND p.birth_date IS NOT NULL GROUP BY 1, 2`).Scan(&out).Error
	return out, err
}

func (r *GormInsightsRepository) Devices(ctx context.Context, from, to time.Time) ([]KeyCount, error) {
	var out []KeyCount
	err := database.Conn(ctx, r.db).Raw(`SELECT device AS key, count(DISTINCT actor) AS count FROM (
		SELECT `+deviceExpr+` AS device, COALESCE(user_id::text, NULLIF(anonymous_id, '')) AS actor
		FROM analytics_events WHERE source = 'client' AND occurred_at >= ? AND occurred_at < ?) e
		WHERE actor IS NOT NULL AND device <> 'bot' GROUP BY 1 ORDER BY 2 DESC`, from, to).Scan(&out).Error
	return out, err
}

func (r *GormInsightsRepository) Sources(ctx context.Context) ([]KeyCount, error) {
	var out []KeyCount
	err := database.Conn(ctx, r.db).Raw(`SELECT COALESCE(NULLIF(a.utm_source, ''), 'direct') AS key, count(*) AS count
		FROM users u LEFT JOIN user_attributions a ON a.user_id = u.id
		WHERE u.role = 'client' AND u.status <> 'deleted' GROUP BY 1 ORDER BY 2 DESC LIMIT 50`).Scan(&out).Error
	return out, err
}

func (r *GormInsightsRepository) SignupSeries(ctx context.Context, from, to time.Time, unit string) ([]TimeCount, error) {
	if _, ok := validIntervals[unit]; !ok {
		unit = "day"
	}
	var rows []TimeCount
	err := database.Conn(ctx, r.db).Raw(`SELECT `+bucketExpr(unit, "created_at")+` AS bucket, count(*) AS count
		FROM users WHERE role = 'client' AND created_at >= ? AND created_at < ? GROUP BY 1`, from, to).Scan(&rows).Error
	return fillCounts(rows, from, to, unit), err
}

func (r *GormInsightsRepository) ActiveUserSeries(ctx context.Context, from, to time.Time, unit string) ([]TimeCount, error) {
	if _, ok := validIntervals[unit]; !ok {
		unit = "day"
	}
	var rows []TimeCount
	err := database.Conn(ctx, r.db).Raw(`SELECT `+bucketExpr(unit, "occurred_at")+` AS bucket, count(DISTINCT user_id) AS count
		FROM analytics_events WHERE user_id IS NOT NULL AND occurred_at >= ? AND occurred_at < ? GROUP BY 1`, from, to).Scan(&rows).Error
	return fillCounts(rows, from, to, unit), err
}

func fillCounts(rows []TimeCount, from, to time.Time, unit string) []TimeCount {
	byBucket := map[time.Time]int64{}
	for _, r := range rows {
		byBucket[r.Bucket.UTC()] = r.Count
	}
	out := []TimeCount{}
	for _, b := range Buckets(from, to, unit) {
		out = append(out, TimeCount{Bucket: b, Count: byBucket[b]})
	}
	return out
}

func (r *GormInsightsRepository) Engagement(ctx context.Context, from, to time.Time, currency string) (*EngagementRow, error) {
	var e EngagementRow
	err := database.Conn(ctx, r.db).Raw(`SELECT
		(SELECT count(DISTINCT user_id) FROM analytics_events WHERE user_id IS NOT NULL AND occurred_at >= ?::timestamptz - interval '1 day' AND occurred_at < ?) AS dau,
		(SELECT count(DISTINCT user_id) FROM analytics_events WHERE user_id IS NOT NULL AND occurred_at >= ?::timestamptz - interval '7 days' AND occurred_at < ?) AS wau,
		(SELECT count(DISTINCT user_id) FROM analytics_events WHERE user_id IS NOT NULL AND occurred_at >= ?::timestamptz - interval '30 days' AND occurred_at < ?) AS mau,
		(SELECT count(DISTINCT user_id) FROM credit_purchases WHERE status = 'succeeded' AND completed_at >= ? AND completed_at < ?) AS paying_users,
		(SELECT COALESCE(sum(price_cents), 0) FROM credit_purchases WHERE status = 'succeeded' AND currency = ? AND completed_at >= ? AND completed_at < ?) AS revenue_cents,
		(SELECT count(*) FROM (SELECT user_id FROM credit_purchases WHERE status = 'succeeded' AND completed_at < ? GROUP BY user_id HAVING count(*) >= 2) x) AS repeat_buyers,
		(SELECT count(DISTINCT user_id) FROM credit_purchases WHERE status = 'succeeded' AND completed_at < ?) AS buyers,
		(SELECT COALESCE(avg(active_seconds), 0) FROM chat_sessions WHERE started_at >= ? AND started_at < ?) AS avg_session_seconds,
		(SELECT count(*) FROM chat_sessions s WHERE s.started_at >= ? AND s.started_at < ? AND EXISTS (
		   SELECT 1 FROM credit_purchases p WHERE p.user_id = s.client_id AND p.status = 'succeeded' AND p.completed_at >= ? AND p.completed_at < ?)) AS paid_user_sessions,
		(SELECT count(DISTINCT e.user_id) FROM analytics_events e WHERE e.user_role = 'client' AND e.occurred_at >= ? AND e.occurred_at < ?) AS active_clients,
		(SELECT count(DISTINCT client_id) FROM chat_sessions WHERE started_at >= ? AND started_at < ?) AS chatting_clients,
		(SELECT count(*) FROM chat_sessions WHERE started_at >= ? AND started_at < ?) AS started_sessions_in_range`,
		to, to, to, to, to, to, from, to, currency, from, to, to, to, from, to, from, to, from, to, from, to, from, to, from, to).Scan(&e).Error
	return &e, err
}

func (r *GormInsightsRepository) Retention(ctx context.Context, since time.Time) (map[time.Time]int64, []CohortCell, error) {
	conn := database.Conn(ctx, r.db)
	var sizes []struct {
		Cohort time.Time
		Size   int64
	}
	if err := conn.Raw(`SELECT date_trunc('week', created_at AT TIME ZONE 'UTC') AS cohort, count(*) AS size
		FROM users WHERE role = 'client' AND created_at >= ? GROUP BY 1`, since).Scan(&sizes).Error; err != nil {
		return nil, nil, err
	}
	out := map[time.Time]int64{}
	for _, s := range sizes {
		out[s.Cohort.UTC()] = s.Size
	}
	var cells []CohortCell
	err := conn.Raw(`WITH cohort AS (
		SELECT id AS user_id, date_trunc('week', created_at AT TIME ZONE 'UTC') AS cw FROM users WHERE role = 'client' AND created_at >= ?
	), activity AS (
		SELECT DISTINCT user_id, date_trunc('week', occurred_at AT TIME ZONE 'UTC') AS aw FROM analytics_events WHERE user_id IS NOT NULL AND occurred_at >= ?
		UNION
		SELECT DISTINCT client_id AS user_id, date_trunc('week', requested_at AT TIME ZONE 'UTC') AS aw FROM chat_sessions WHERE requested_at >= ?
	)
	SELECT c.cw AS cohort, (EXTRACT(EPOCH FROM (a.aw - c.cw)) / 604800)::int AS week, count(DISTINCT a.user_id) AS users
	FROM cohort c JOIN activity a ON a.user_id = c.user_id AND a.aw >= c.cw
	GROUP BY 1, 2`, since, since, since).Scan(&cells).Error
	for i := range cells {
		cells[i].Cohort = cells[i].Cohort.UTC()
	}
	return out, cells, err
}

func (r *GormInsightsRepository) TopUsers(ctx context.Context, metric string, from, to time.Time, currency string, limit int) ([]TopUser, error) {
	var q string
	args := []any{}
	switch metric {
	case "minutes":
		q = `SELECT client_id AS user_id, COALESCE(sum(billed_minutes), 0) AS value FROM chat_sessions
			WHERE started_at >= ? AND started_at < ? GROUP BY 1 ORDER BY 2 DESC LIMIT ?`
		args = append(args, from, to)
	case "sessions":
		q = `SELECT client_id AS user_id, count(*) AS value FROM chat_sessions
			WHERE started_at >= ? AND started_at < ? GROUP BY 1 ORDER BY 2 DESC LIMIT ?`
		args = append(args, from, to)
	default:
		q = `SELECT user_id, COALESCE(sum(price_cents), 0) AS value FROM credit_purchases
			WHERE status = 'succeeded' AND currency = ? AND completed_at >= ? AND completed_at < ? GROUP BY 1 ORDER BY 2 DESC LIMIT ?`
		args = append(args, currency, from, to)
	}
	var out []TopUser
	err := database.Conn(ctx, r.db).Raw(q, append(args, clampLimit(limit, 20, 100))...).Scan(&out).Error
	return out, err
}

func (r *GormInsightsRepository) PsychicsUsedBy(ctx context.Context, clientID uuid.UUID) ([]PsychicUsage, error) {
	var out []PsychicUsage
	err := database.Conn(ctx, r.db).Raw(`SELECT s.psychic_id, p.name, p.slug,
		count(*) FILTER (WHERE s.started_at IS NOT NULL) AS sessions, COALESCE(sum(s.billed_minutes), 0) AS minutes
		FROM chat_sessions s JOIN psychics p ON p.user_id = s.psychic_id
		WHERE s.client_id = ? GROUP BY 1, 2, 3 ORDER BY minutes DESC, sessions DESC LIMIT 50`, clientID).Scan(&out).Error
	return out, err
}

func (r *GormInsightsRepository) UserDevices(ctx context.Context, userID uuid.UUID) ([]KeyCount, error) {
	var out []KeyCount
	err := database.Conn(ctx, r.db).Raw(`SELECT `+deviceExpr+` AS key, count(*) AS count FROM analytics_events
		WHERE user_id = ? AND source = 'client' GROUP BY 1 ORDER BY 2 DESC`, userID).Scan(&out).Error
	return out, err
}

// Activity merges an account's recent history (newest first). Summaries are
// machine-friendly English for the internal admin console.
func (r *GormInsightsRepository) Activity(ctx context.Context, userID uuid.UUID, limit int) ([]ActivityRow, error) {
	limit = clampLimit(limit, 100, 200)
	var out []ActivityRow
	err := database.Conn(ctx, r.db).Raw(`
		(SELECT created_at AS at, 'signup' AS type, 'Account created (' || role || ')' AS summary, NULL::uuid AS ref_id FROM users WHERE id = ?)
		UNION ALL
		(SELECT COALESCE(completed_at, created_at), 'purchase', status || ': ' || credits || ' credits for ' || price_cents || ' ' || currency, id
		   FROM credit_purchases WHERE user_id = ? ORDER BY created_at DESC LIMIT ?)
		UNION ALL
		(SELECT requested_at, 'session', status || CASE WHEN end_reason <> '' THEN ' (' || end_reason || ')' ELSE '' END || ', ' || billed_minutes || ' min', id
		   FROM chat_sessions WHERE client_id = ? OR psychic_id = ? ORDER BY requested_at DESC LIMIT ?)
		UNION ALL
		(SELECT created_at, 'request', service_name || ': ' || status, id FROM custom_requests WHERE client_id = ? OR psychic_id = ? ORDER BY created_at DESC LIMIT ?)
		UNION ALL
		(SELECT created_at, 'review', rating || '/5 (' || status || ')', id FROM psychic_reviews WHERE client_id = ? OR psychic_id = ? ORDER BY created_at DESC LIMIT ?)
		UNION ALL
		(SELECT created_at, 'ledger', entry_type || ' ' || amount || CASE WHEN description <> '' THEN ': ' || description ELSE '' END, id
		   FROM credit_ledger WHERE user_id = ? AND entry_type IN ('admin_adjustment', 'refund', 'signup_bonus', 'hold_refund') ORDER BY created_at DESC LIMIT ?)
		UNION ALL
		(SELECT occurred_at, 'event', name || CASE WHEN path <> '' THEN ' ' || path ELSE '' END, NULL::uuid
		   FROM analytics_events WHERE user_id = ? ORDER BY occurred_at DESC LIMIT ?)
		UNION ALL
		(SELECT created_at, 'admin', action, id FROM audit_logs WHERE target_id = ?::text ORDER BY created_at DESC LIMIT ?)
		ORDER BY at DESC LIMIT ?`,
		userID, userID, limit, userID, userID, limit, userID, userID, limit, userID, userID, limit, userID, limit, userID, limit, userID, limit, limit).
		Scan(&out).Error
	return out, err
}

func (r *GormInsightsRepository) PsychicSessions(ctx context.Context, psychicID uuid.UUID, from, to time.Time) (*PsychicSessionStats, error) {
	var s PsychicSessionStats
	err := database.Conn(ctx, r.db).Raw(`SELECT
		count(*) AS requested,
		count(*) FILTER (WHERE accepted_at IS NOT NULL) AS accepted,
		count(*) FILTER (WHERE status = 'declined') AS declined,
		count(*) FILTER (WHERE status = 'expired' AND accepted_at IS NULL) AS expired,
		count(*) FILTER (WHERE status = 'cancelled') AS cancelled,
		count(*) FILTER (WHERE started_at IS NOT NULL AND status IN ('ended', 'settled')) AS completed,
		count(*) FILTER (WHERE started_at IS NOT NULL AND end_reason = 'grace_expired') AS interrupted,
		COALESCE(avg(EXTRACT(EPOCH FROM accepted_at - requested_at)) FILTER (WHERE accepted_at IS NOT NULL), 0) AS avg_response_seconds,
		COALESCE(avg(active_seconds) FILTER (WHERE started_at IS NOT NULL AND status IN ('ended', 'settled')), 0) AS avg_duration_seconds,
		COALESCE(sum(billed_minutes), 0) AS total_minutes
		FROM chat_sessions WHERE psychic_id = ? AND requested_at >= ? AND requested_at < ?`, psychicID, from, to).Scan(&s).Error
	return &s, err
}

func (r *GormInsightsRepository) PsychicClients(ctx context.Context, psychicID uuid.UUID, from, to time.Time) (*PsychicClientStats, error) {
	var s PsychicClientStats
	err := database.Conn(ctx, r.db).Raw(`WITH firsts AS (
		SELECT client_id, min(started_at) AS first_at FROM chat_sessions WHERE psychic_id = ? AND started_at IS NOT NULL GROUP BY client_id
	), in_range AS (
		SELECT DISTINCT client_id FROM chat_sessions WHERE psychic_id = ? AND started_at >= ? AND started_at < ?
	)
	SELECT count(*) AS unique,
		count(*) FILTER (WHERE f.first_at >= ?) AS new,
		count(*) FILTER (WHERE f.first_at < ?) AS returning
	FROM in_range i JOIN firsts f ON f.client_id = i.client_id`, psychicID, psychicID, from, to, from, from).Scan(&s).Error
	return &s, err
}

func (r *GormInsightsRepository) PsychicTopClients(ctx context.Context, psychicID uuid.UUID, from, to time.Time, limit int) ([]ClientUsage, error) {
	var out []ClientUsage
	err := database.Conn(ctx, r.db).Raw(`SELECT s.client_id, COALESCE(NULLIF(p.nickname, ''), u.display_name) AS name,
		count(*) AS sessions, COALESCE(sum(s.billed_minutes), 0) AS minutes
		FROM chat_sessions s JOIN users u ON u.id = s.client_id LEFT JOIN user_profiles p ON p.user_id = s.client_id
		WHERE s.psychic_id = ? AND s.started_at >= ? AND s.started_at < ?
		GROUP BY 1, 2 ORDER BY minutes DESC, sessions DESC LIMIT ?`, psychicID, from, to, clampLimit(limit, 5, 20)).Scan(&out).Error
	return out, err
}

func (r *GormInsightsRepository) PsychicEarningsSeries(ctx context.Context, psychicID uuid.UUID, from, to time.Time, unit, tz string) ([]EarningBucket, error) {
	if _, ok := validIntervals[unit]; !ok {
		unit = "day"
	}
	var rows []EarningBucket
	err := database.Conn(ctx, r.db).Raw(`SELECT date_trunc('`+unit+`', created_at AT TIME ZONE ?) AS bucket,
		COALESCE(sum(net_credits), 0) AS net, COALESCE(sum(gross_credits), 0) AS gross,
		count(*) FILTER (WHERE source_type = 'chat_session' AND entry_kind = 'earning') AS sessions
		FROM psychic_earnings WHERE psychic_id = ? AND created_at >= ? AND created_at < ? GROUP BY 1`,
		tz, psychicID, from, to).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	loc, lerr := time.LoadLocation(tz)
	if lerr != nil {
		loc = time.UTC
	}
	by := map[time.Time]EarningBucket{}
	for _, b := range rows {
		by[b.Bucket.UTC()] = b
	}
	out := []EarningBucket{}
	for _, t := range BucketsIn(from, to, unit, loc) {
		b := by[t]
		b.Bucket = t
		out = append(out, b)
	}
	return out, nil
}

func (r *GormInsightsRepository) PsychicHours(ctx context.Context, psychicID uuid.UUID, from, to time.Time, tz string) ([]HourCell, error) {
	var out []HourCell
	err := database.Conn(ctx, r.db).Raw(`SELECT EXTRACT(ISODOW FROM requested_at AT TIME ZONE ?)::int AS weekday,
		EXTRACT(HOUR FROM requested_at AT TIME ZONE ?)::int AS hour, count(*) AS count
		FROM chat_sessions WHERE psychic_id = ? AND requested_at >= ? AND requested_at < ? GROUP BY 1, 2`,
		tz, tz, psychicID, from, to).Scan(&out).Error
	return out, err
}

func (r *GormInsightsRepository) ClientHistoryWith(ctx context.Context, clientID, psychicID uuid.UUID) (*ClientHistory, error) {
	var h ClientHistory
	err := database.Conn(ctx, r.db).Raw(`SELECT count(*) FILTER (WHERE started_at IS NOT NULL) AS sessions,
		COALESCE(sum(billed_minutes), 0) AS minutes, min(started_at) AS first_session_at, max(started_at) AS last_session_at
		FROM chat_sessions WHERE client_id = ? AND psychic_id = ?`, clientID, psychicID).Scan(&h).Error
	return &h, err
}
