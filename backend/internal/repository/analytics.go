package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// EventFilter narrows analytics event listings.
type EventFilter struct {
	Name     string
	Source   string
	UserID   *uuid.UUID
	From, To time.Time
	AfterID  int64
	Limit    int
}

// SeriesPoint is one bucket of a time series.
type SeriesPoint struct {
	Bucket time.Time `json:"bucket"`
	Count  int64     `json:"count"`
	Users  int64     `json:"users"`
}

// FunnelStep is the reach of one funnel step.
type FunnelStep struct {
	Name   string `json:"name"`
	Actors int64  `json:"actors"`
}

// NameCount is an aggregated count by event name.
type NameCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
	Users int64  `json:"users"`
}

// AnalyticsRepository persists and aggregates analytics events.
type AnalyticsRepository interface {
	InsertBatch(ctx context.Context, events []domain.AnalyticsEvent) (int64, error)
	List(ctx context.Context, f EventFilter) ([]domain.AnalyticsEvent, error)
	Timeseries(ctx context.Context, name string, from, to time.Time, interval string) ([]SeriesPoint, error)
	Funnel(ctx context.Context, steps []string, from, to time.Time) ([]FunnelStep, error)
	TopEvents(ctx context.Context, from, to time.Time, limit int) ([]NameCount, error)
}

// GormAnalyticsRepository implements AnalyticsRepository.
type GormAnalyticsRepository struct{ db *gorm.DB }

// NewAnalyticsRepository returns a GORM analytics repository.
func NewAnalyticsRepository(db *gorm.DB) *GormAnalyticsRepository {
	return &GormAnalyticsRepository{db: db}
}

// InsertBatch inserts events, silently skipping duplicates (client retries).
func (r *GormAnalyticsRepository) InsertBatch(ctx context.Context, events []domain.AnalyticsEvent) (int64, error) {
	if len(events) == 0 {
		return 0, nil
	}
	res := database.Conn(ctx, r.db).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "event_id"}}, DoNothing: true,
	}).CreateInBatches(events, 500)
	return res.RowsAffected, res.Error
}

func (r *GormAnalyticsRepository) List(ctx context.Context, f EventFilter) ([]domain.AnalyticsEvent, error) {
	q := database.Conn(ctx, r.db).Model(&domain.AnalyticsEvent{}).Where("occurred_at >= ? AND occurred_at < ?", f.From, f.To)
	if f.Name != "" {
		q = q.Where("name = ?", f.Name)
	}
	if f.Source != "" {
		q = q.Where("source = ?", f.Source)
	}
	if f.UserID != nil {
		q = q.Where("user_id = ?", *f.UserID)
	}
	if f.AfterID > 0 {
		q = q.Where("id < ?", f.AfterID)
	}
	var out []domain.AnalyticsEvent
	err := q.Order("id DESC").Limit(clampLimit(f.Limit, 100, 1000)).Find(&out).Error
	return out, err
}

// validIntervals maps API intervals to date_trunc units.
var validIntervals = map[string]string{"hour": "hour", "day": "day", "week": "week", "month": "month"}

func (r *GormAnalyticsRepository) Timeseries(ctx context.Context, name string, from, to time.Time, interval string) ([]SeriesPoint, error) {
	unit, ok := validIntervals[interval]
	if !ok {
		return nil, fmt.Errorf("repository: invalid interval %q", interval)
	}
	var out []SeriesPoint
	// unit comes from the allow-list above, never from user input directly.
	err := database.Conn(ctx, r.db).Raw(`
		SELECT date_trunc('`+unit+`', occurred_at) AS bucket,
		       count(*) AS count,
		       count(DISTINCT COALESCE(user_id::text, NULLIF(anonymous_id, ''))) AS users
		FROM analytics_events
		WHERE name = ? AND occurred_at >= ? AND occurred_at < ?
		GROUP BY 1 ORDER BY 1`, name, from, to).Scan(&out).Error
	return out, err
}

// Funnel counts actors (users or anonymous visitors) that performed each step
// in order within the window. Step i requires step i-1 earlier in time.
func (r *GormAnalyticsRepository) Funnel(ctx context.Context, steps []string, from, to time.Time) ([]FunnelStep, error) {
	out := make([]FunnelStep, 0, len(steps))
	if len(steps) == 0 {
		return out, nil
	}
	// Build: first occurrence per actor for step 1, then for each next step the
	// first occurrence after the previous step's time.
	query := `WITH s0 AS (
		SELECT COALESCE(user_id::text, anonymous_id) AS actor, min(occurred_at) AS at
		FROM analytics_events WHERE name = ? AND occurred_at >= ? AND occurred_at < ?
		  AND (user_id IS NOT NULL OR anonymous_id <> '')
		GROUP BY 1)`
	args := []any{steps[0], from, to}
	for i := 1; i < len(steps); i++ {
		query += fmt.Sprintf(`, s%d AS (
		SELECT p.actor, min(e.occurred_at) AS at
		FROM s%d p JOIN analytics_events e
		  ON COALESCE(e.user_id::text, e.anonymous_id) = p.actor AND e.occurred_at >= p.at
		WHERE e.name = ? AND e.occurred_at < ?
		GROUP BY p.actor)`, i, i-1)
		args = append(args, steps[i], to)
	}
	query += " SELECT "
	for i := range steps {
		if i > 0 {
			query += ", "
		}
		query += fmt.Sprintf("(SELECT count(*) FROM s%d) AS c%d", i, i)
	}
	rows, err := database.Conn(ctx, r.db).Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := make([]int64, len(steps))
	ptrs := make([]any, len(steps))
	for i := range counts {
		ptrs[i] = &counts[i]
	}
	if rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
	}
	for i, name := range steps {
		out = append(out, FunnelStep{Name: name, Actors: counts[i]})
	}
	return out, rows.Err()
}

func (r *GormAnalyticsRepository) TopEvents(ctx context.Context, from, to time.Time, limit int) ([]NameCount, error) {
	var out []NameCount
	err := database.Conn(ctx, r.db).Raw(`
		SELECT name, count(*) AS count, count(DISTINCT COALESCE(user_id::text, NULLIF(anonymous_id, ''))) AS users
		FROM analytics_events WHERE occurred_at >= ? AND occurred_at < ?
		GROUP BY name ORDER BY count DESC LIMIT ?`, from, to, clampLimit(limit, 50, 200)).Scan(&out).Error
	return out, err
}

// Overview is the headline business report for the admin dashboard.
type Overview struct {
	From                 time.Time         `json:"from"`
	To                   time.Time         `json:"to"`
	NewUsers             map[string]int64  `json:"new_users"`
	TotalUsers           map[string]int64  `json:"total_users"`
	RevenueCents         map[string]int64  `json:"revenue_cents"`
	Purchases            int64             `json:"purchases"`
	PayingClients        int64             `json:"paying_clients"`
	CreditsPurchased     int64             `json:"credits_purchased"`
	CreditsSpentChat     int64             `json:"credits_spent_chat"`
	CreditsSpentRequests int64             `json:"credits_spent_requests"`
	CreditsRefunded      int64             `json:"credits_refunded"`
	Sessions             map[string]int64  `json:"sessions_by_status"`
	SessionEndReasons    map[string]int64  `json:"session_end_reasons"`
	AvgSessionSeconds    float64           `json:"avg_session_seconds"`
	AvgResponseSeconds   float64           `json:"avg_accept_seconds"`
	AcceptanceRate       float64           `json:"acceptance_rate"`
	Messages             int64             `json:"messages"`
	CustomRequests       map[string]int64  `json:"custom_requests_by_status"`
	PsychicNetCredits    int64             `json:"psychic_net_credits"`
	PlatformCommission   int64             `json:"platform_commission_credits"`
	TopPsychics          []PsychicEarnings `json:"top_psychics"`
}

// PsychicEarnings ranks psychics by earnings.
type PsychicEarnings struct {
	PsychicID  uuid.UUID `json:"psychic_id"`
	Name       string    `json:"name"`
	Slug       string    `json:"slug"`
	Sessions   int64     `json:"sessions"`
	NetCredits int64     `json:"net_credits"`
}

// DailyMetric is one day of key business metrics.
type DailyMetric struct {
	Day          time.Time `json:"day"`
	Signups      int64     `json:"signups"`
	RevenueCents int64     `json:"revenue_cents"`
	Sessions     int64     `json:"sessions"`
	ChatMinutes  int64     `json:"chat_minutes"`
	CreditsSpent int64     `json:"credits_spent"`
}

// ReportRepository computes reports from the transactional tables.
type ReportRepository interface {
	Overview(ctx context.Context, from, to time.Time) (*Overview, error)
	Daily(ctx context.Context, from, to time.Time) ([]DailyMetric, error)
}

// GormReportRepository implements ReportRepository.
type GormReportRepository struct{ db *gorm.DB }

// NewReportRepository returns a GORM report repository.
func NewReportRepository(db *gorm.DB) *GormReportRepository { return &GormReportRepository{db: db} }

type keyCount struct {
	Key   string
	Count int64
}

func (r *GormReportRepository) keyed(ctx context.Context, query string, args ...any) (map[string]int64, error) {
	var rows []keyCount
	if err := database.Conn(ctx, r.db).Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[row.Key] = row.Count
	}
	return out, nil
}

func (r *GormReportRepository) Overview(ctx context.Context, from, to time.Time) (*Overview, error) {
	o := &Overview{From: from, To: to}
	var err error
	conn := database.Conn(ctx, r.db)

	if o.NewUsers, err = r.keyed(ctx, `SELECT role AS key, count(*) AS count FROM users WHERE created_at >= ? AND created_at < ? GROUP BY role`, from, to); err != nil {
		return nil, err
	}
	if o.TotalUsers, err = r.keyed(ctx, `SELECT role AS key, count(*) AS count FROM users WHERE created_at < ? GROUP BY role`, to); err != nil {
		return nil, err
	}
	if o.RevenueCents, err = r.keyed(ctx, `SELECT currency AS key, COALESCE(sum(price_cents),0) AS count FROM credit_purchases
		WHERE status = 'succeeded' AND completed_at >= ? AND completed_at < ? GROUP BY currency`, from, to); err != nil {
		return nil, err
	}
	var purchases struct {
		Purchases        int64
		PayingClients    int64
		CreditsPurchased int64
	}
	if err := conn.Raw(`SELECT count(*) AS purchases, count(DISTINCT user_id) AS paying_clients, COALESCE(sum(credits),0) AS credits_purchased
		FROM credit_purchases WHERE status = 'succeeded' AND completed_at >= ? AND completed_at < ?`, from, to).Scan(&purchases).Error; err != nil {
		return nil, err
	}
	o.Purchases, o.PayingClients, o.CreditsPurchased = purchases.Purchases, purchases.PayingClients, purchases.CreditsPurchased

	var ledger struct {
		Chat     int64
		Requests int64
		Refunded int64
	}
	if err := conn.Raw(`SELECT
		COALESCE(-sum(amount) FILTER (WHERE entry_type = 'chat_charge'), 0) AS chat,
		COALESCE(sum(-held_amount) FILTER (WHERE entry_type = 'hold_release'), 0) AS requests,
		COALESCE(sum(amount) FILTER (WHERE entry_type IN ('hold_refund', 'refund')), 0) AS refunded
		FROM credit_ledger WHERE created_at >= ? AND created_at < ?`, from, to).Scan(&ledger).Error; err != nil {
		return nil, err
	}
	o.CreditsSpentChat, o.CreditsSpentRequests, o.CreditsRefunded = ledger.Chat, ledger.Requests, ledger.Refunded

	if o.Sessions, err = r.keyed(ctx, `SELECT status AS key, count(*) AS count FROM chat_sessions WHERE requested_at >= ? AND requested_at < ? GROUP BY status`, from, to); err != nil {
		return nil, err
	}
	if o.SessionEndReasons, err = r.keyed(ctx, `SELECT end_reason AS key, count(*) AS count FROM chat_sessions
		WHERE requested_at >= ? AND requested_at < ? AND end_reason <> '' GROUP BY end_reason`, from, to); err != nil {
		return nil, err
	}
	var sess struct {
		AvgSeconds float64
		AvgAccept  float64
		Accepted   int64
		Decided    int64
	}
	if err := conn.Raw(`SELECT
		COALESCE(avg(active_seconds) FILTER (WHERE started_at IS NOT NULL AND status IN ('ended','settled')), 0) AS avg_seconds,
		COALESCE(avg(EXTRACT(EPOCH FROM accepted_at - requested_at)) FILTER (WHERE accepted_at IS NOT NULL), 0) AS avg_accept,
		count(*) FILTER (WHERE accepted_at IS NOT NULL) AS accepted,
		count(*) FILTER (WHERE accepted_at IS NOT NULL OR status IN ('declined','expired')) AS decided
		FROM chat_sessions WHERE requested_at >= ? AND requested_at < ?`, from, to).Scan(&sess).Error; err != nil {
		return nil, err
	}
	o.AvgSessionSeconds, o.AvgResponseSeconds = sess.AvgSeconds, sess.AvgAccept
	if sess.Decided > 0 {
		o.AcceptanceRate = float64(sess.Accepted) / float64(sess.Decided)
	}
	if err := conn.Raw(`SELECT count(*) FROM chat_messages WHERE kind = 'text' AND created_at >= ? AND created_at < ?`, from, to).Scan(&o.Messages).Error; err != nil {
		return nil, err
	}
	if o.CustomRequests, err = r.keyed(ctx, `SELECT status AS key, count(*) AS count FROM custom_requests WHERE created_at >= ? AND created_at < ? GROUP BY status`, from, to); err != nil {
		return nil, err
	}
	var earn struct {
		Net        int64
		Commission int64
	}
	if err := conn.Raw(`SELECT COALESCE(sum(net_credits),0) AS net, COALESCE(sum(commission_credits),0) AS commission
		FROM psychic_earnings WHERE created_at >= ? AND created_at < ?`, from, to).Scan(&earn).Error; err != nil {
		return nil, err
	}
	o.PsychicNetCredits, o.PlatformCommission = earn.Net, earn.Commission
	if err := conn.Raw(`SELECT p.user_id AS psychic_id, p.name, p.slug,
		count(*) FILTER (WHERE e.source_type = 'chat_session' AND e.entry_kind = 'earning') AS sessions,
		COALESCE(sum(e.net_credits),0) AS net_credits
		FROM psychic_earnings e JOIN psychics p ON p.user_id = e.psychic_id
		WHERE e.created_at >= ? AND e.created_at < ?
		GROUP BY p.user_id, p.name, p.slug ORDER BY net_credits DESC LIMIT 10`, from, to).Scan(&o.TopPsychics).Error; err != nil {
		return nil, err
	}
	return o, nil
}

func (r *GormReportRepository) Daily(ctx context.Context, from, to time.Time) ([]DailyMetric, error) {
	var out []DailyMetric
	err := database.Conn(ctx, r.db).Raw(`
		WITH days AS (SELECT generate_series(date_trunc('day', ?::timestamptz), date_trunc('day', ?::timestamptz - interval '1 microsecond'), interval '1 day') AS day)
		SELECT d.day,
		  (SELECT count(*) FROM users u WHERE u.created_at >= d.day AND u.created_at < d.day + interval '1 day') AS signups,
		  (SELECT COALESCE(sum(price_cents),0) FROM credit_purchases p WHERE p.status = 'succeeded' AND p.completed_at >= d.day AND p.completed_at < d.day + interval '1 day') AS revenue_cents,
		  (SELECT count(*) FROM chat_sessions s WHERE s.started_at >= d.day AND s.started_at < d.day + interval '1 day') AS sessions,
		  (SELECT COALESCE(sum(billed_minutes),0) FROM chat_sessions s WHERE s.started_at >= d.day AND s.started_at < d.day + interval '1 day') AS chat_minutes,
		  (SELECT COALESCE(-sum(amount),0) FROM credit_ledger l WHERE l.entry_type = 'chat_charge' AND l.created_at >= d.day AND l.created_at < d.day + interval '1 day') AS credits_spent
		FROM days d ORDER BY d.day`, from, to).Scan(&out).Error
	return out, err
}
