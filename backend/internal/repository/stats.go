package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// Ratio is a KPI share: Numerator of Denominator.
type Ratio struct {
	Numerator   int64
	Denominator int64
}

// RepeatGap is the median number of days between consecutive paid sessions.
type RepeatGap struct {
	MedianDays sql.NullFloat64
	Sample     int64
}

// StatsRepository computes an advisor's performance KPIs from chat sessions
// and presence history.
//
// Three queries use raw SQL because they need window functions, DISTINCT ON
// and percentile_cont, which the GORM builder cannot express. Every value is
// a bound parameter; no identifier is ever built from input.
type StatsRepository interface {
	FreeToPaid(ctx context.Context, psychicID uuid.UUID, from, to time.Time) (Ratio, error)
	PaidToRetained(ctx context.Context, psychicID uuid.UUID, from, to time.Time) (Ratio, error)
	RepeatGap(ctx context.Context, psychicID uuid.UUID, from, to time.Time) (RepeatGap, error)
	PaidMinutes(ctx context.Context, psychicID uuid.UUID, from, to time.Time) (int64, error)
	InSessionSeconds(ctx context.Context, psychicID uuid.UUID, from, to time.Time) (int64, error)
	// AvailableSeconds sums the overlap of online/busy intervals with
	// [from, to); open intervals count up to now. trackedSince is the first
	// recorded interval, if any.
	AvailableSeconds(ctx context.Context, psychicID uuid.UUID, from, to, now time.Time) (secs int64, trackedSince *time.Time, err error)
}

// GormStatsRepository implements StatsRepository.
type GormStatsRepository struct{ db *gorm.DB }

// NewStatsRepository returns a stats repository.
func NewStatsRepository(db *gorm.DB) *GormStatsRepository { return &GormStatsRepository{db: db} }

// FreeToPaid: clients whose first started session with the advisor fell in
// the range and was free, and how many of them later paid.
func (r *GormStatsRepository) FreeToPaid(ctx context.Context, psychicID uuid.UUID, from, to time.Time) (Ratio, error) {
	var out Ratio
	err := database.Conn(ctx, r.db).Raw(`
		WITH firsts AS (
		  SELECT DISTINCT ON (client_id) client_id, started_at, credits_charged
		  FROM chat_sessions
		  WHERE psychic_id = @psychic AND started_at IS NOT NULL
		  ORDER BY client_id, started_at, id
		)
		SELECT count(*) AS denominator,
		       count(*) FILTER (WHERE EXISTS (
		         SELECT 1 FROM chat_sessions later
		         WHERE later.psychic_id = @psychic AND later.client_id = f.client_id
		           AND later.started_at > f.started_at AND later.credits_charged > 0)) AS numerator
		FROM firsts f
		WHERE f.started_at >= @from AND f.started_at < @to AND f.credits_charged = 0`,
		sql.Named("psychic", psychicID), sql.Named("from", from), sql.Named("to", to)).Scan(&out).Error
	return out, err
}

// PaidToRetained: clients whose first paid session with the advisor fell in
// the range, and how many came back for another paid session.
func (r *GormStatsRepository) PaidToRetained(ctx context.Context, psychicID uuid.UUID, from, to time.Time) (Ratio, error) {
	var out Ratio
	err := database.Conn(ctx, r.db).Raw(`
		WITH first_paid AS (
		  SELECT DISTINCT ON (client_id) client_id, started_at
		  FROM chat_sessions
		  WHERE psychic_id = @psychic AND started_at IS NOT NULL AND credits_charged > 0
		  ORDER BY client_id, started_at, id
		)
		SELECT count(*) AS denominator,
		       count(*) FILTER (WHERE EXISTS (
		         SELECT 1 FROM chat_sessions later
		         WHERE later.psychic_id = @psychic AND later.client_id = f.client_id
		           AND later.credits_charged > 0 AND later.started_at > f.started_at)) AS numerator
		FROM first_paid f
		WHERE f.started_at >= @from AND f.started_at < @to`,
		sql.Named("psychic", psychicID), sql.Named("from", from), sql.Named("to", to)).Scan(&out).Error
	return out, err
}

// RepeatGap: median days between consecutive paid sessions of the same
// client, counting gaps whose later session falls in the range.
func (r *GormStatsRepository) RepeatGap(ctx context.Context, psychicID uuid.UUID, from, to time.Time) (RepeatGap, error) {
	var out RepeatGap
	err := database.Conn(ctx, r.db).Raw(`
		WITH paid AS (
		  SELECT started_at, lag(started_at) OVER (PARTITION BY client_id ORDER BY started_at, id) AS prev
		  FROM chat_sessions
		  WHERE psychic_id = @psychic AND started_at IS NOT NULL AND credits_charged > 0
		)
		SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (started_at - prev)) / 86400.0) AS median_days,
		       count(*) AS sample
		FROM paid
		WHERE prev IS NOT NULL AND started_at >= @from AND started_at < @to`,
		sql.Named("psychic", psychicID), sql.Named("from", from), sql.Named("to", to)).Scan(&out).Error
	return out, err
}

func (r *GormStatsRepository) sessionsInRange(ctx context.Context, psychicID uuid.UUID, from, to time.Time) *gorm.DB {
	return database.Conn(ctx, r.db).Model(&domain.Session{}).
		Where("psychic_id = ? AND started_at >= ? AND started_at < ?", psychicID, from, to)
}

// PaidMinutes sums billed minutes that were not covered by free minutes.
func (r *GormStatsRepository) PaidMinutes(ctx context.Context, psychicID uuid.UUID, from, to time.Time) (int64, error) {
	var n int64
	err := r.sessionsInRange(ctx, psychicID, from, to).
		Select("COALESCE(sum(GREATEST(billed_minutes - free_minutes_used, 0)), 0)").Scan(&n).Error
	return n, err
}

// InSessionSeconds sums time spent in live sessions.
func (r *GormStatsRepository) InSessionSeconds(ctx context.Context, psychicID uuid.UUID, from, to time.Time) (int64, error) {
	var n int64
	err := r.sessionsInRange(ctx, psychicID, from, to).Select("COALESCE(sum(active_seconds), 0)").Scan(&n).Error
	return n, err
}

// AvailableSeconds implements StatsRepository.
func (r *GormStatsRepository) AvailableSeconds(ctx context.Context, psychicID uuid.UUID, from, to, now time.Time) (int64, *time.Time, error) {
	conn := database.Conn(ctx, r.db)
	var secs float64
	err := conn.Model(&domain.PresenceInterval{}).
		Where("psychic_id = ? AND status IN ?", psychicID, []domain.PresenceStatus{domain.PresenceOnline, domain.PresenceBusy}).
		Where("started_at < ? AND COALESCE(ended_at, ?) > ?", to, now, from).
		Select("COALESCE(sum(GREATEST(0, EXTRACT(EPOCH FROM (LEAST(COALESCE(ended_at, ?), ?) - GREATEST(started_at, ?))))), 0)", now, to, from).
		Scan(&secs).Error
	if err != nil {
		return 0, nil, err
	}
	var first sql.NullTime
	if err := conn.Model(&domain.PresenceInterval{}).Where("psychic_id = ?", psychicID).
		Select("min(started_at)").Scan(&first).Error; err != nil {
		return 0, nil, err
	}
	var since *time.Time
	if first.Valid {
		t := first.Time
		since = &t
	}
	return int64(secs), since, nil
}
