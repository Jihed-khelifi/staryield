package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// LifecycleDue is a user a journey is due for.
type LifecycleDue struct {
	UserID uuid.UUID
	// EventAt is the time the journey counts from (sign-up, first or last
	// reading): with UserID, the keyset position of the next batch.
	EventAt time.Time
	// Cycle is the cycle the message will be recorded under (1 for the
	// journeys that run once; the next number for win-back).
	Cycle int
	// FreeMinutes is the wallet's free minutes (first-reading nudge copy).
	FreeMinutes int
}

// LifecycleCursor is the keyset position after the last user of a batch.
type LifecycleCursor struct {
	At     time.Time
	UserID uuid.UUID
}

// LifecycleDueQuery bounds one batch of a journey's due users. Windows keep
// every query on an index range: sign-up journeys look at accounts created
// between Floor and Cutoff, reading journeys at readers whose first or last
// reading is in that range.
type LifecycleDueQuery struct {
	Journey string
	// Cutoff: the event (sign-up, first or last reading, last activity) is at
	// or before it, that is, the journey's delay has passed.
	Cutoff time.Time
	// Floor: events older than this are no longer nudged (a catch-up window, so
	// enabling a journey never messages the whole history).
	Floor time.Time
	// CapSince: a user who was sent a lifecycle message after it is not due yet.
	CapSince time.Time
	// InactiveSince (win-back): no sign-in and no session activity after it.
	InactiveSince time.Time
	// CycleSince (win-back): at most one message since this time.
	CycleSince time.Time
	// MaxCycles (win-back): messages per lapse.
	MaxCycles int
	// After continues a run after the previous batch (nil: from Floor).
	After *LifecycleCursor
	Limit int
}

// LifecycleStatRow is one (journey, status, reason) group of the statistics.
type LifecycleStatRow struct {
	Journey     string
	Status      string
	SkipReason  string
	N7          int64
	N30         int64
	Converted7  int64
	Converted30 int64
}

// LifecycleFunnel counts the sign-up cohort of a window at each activation step.
type LifecycleFunnel struct {
	SignedUp      int64
	Verified      int64
	BirthDetails  int64
	FirstReading  int64
	SecondReading int64
}

// OnboardingFacts are the account facts the checklist shows.
type OnboardingFacts struct {
	Verified     bool
	BirthDetails bool
	FirstReading bool
	Dismissed    bool
}

// LifecycleRepository persists lifecycle messages and the facts they are
// based on. The due and statistics queries are hand written (EXISTS probes,
// FILTER aggregates): every value is a bound parameter and no identifier is
// built from input.
type LifecycleRepository interface {
	// WithRunLock runs fn while holding the job's advisory lock; acquired is
	// false (fn not run) when another instance holds it.
	WithRunLock(ctx context.Context, fn func(ctx context.Context) error) (acquired bool, err error)
	// Due returns the next batch of users a journey is due for, oldest event first.
	Due(ctx context.Context, q LifecycleDueQuery) ([]LifecycleDue, error)
	// Claim inserts the messages (ON CONFLICT DO NOTHING) and returns the ids
	// of the rows that were really inserted: only their inserter sends.
	Claim(ctx context.Context, rows []domain.LifecycleMessage) (map[uuid.UUID]bool, error)
	// Release deletes claims whose delivery failed before anything was sent.
	Release(ctx context.Context, ids []uuid.UUID) error
	// RecordReaders recomputes the reader rows of the clients from their sessions.
	RecordReaders(ctx context.Context, clientIDs []uuid.UUID) error
	// ReconcileReaders refreshes the readers of every reading settled since a time.
	ReconcileReaders(ctx context.Context, since time.Time) (int, error)
	// Convert marks the user's open reading-driven messages created in
	// [from, to] as converted (one UPDATE) and returns how many.
	Convert(ctx context.Context, userID uuid.UUID, from, to time.Time) (int64, error)
	// Stats groups the messages of the last 30 days.
	Stats(ctx context.Context, now time.Time, attributionDays int) ([]LifecycleStatRow, error)
	// Funnel counts the clients who signed up in [from, to).
	Funnel(ctx context.Context, from, to time.Time) (LifecycleFunnel, error)
	// Onboarding reads the checklist facts of a client in one query.
	Onboarding(ctx context.Context, userID uuid.UUID) (OnboardingFacts, error)
	// SetOnboardingDismissed stores (at) or clears (nil) the dismissal.
	SetOnboardingDismissed(ctx context.Context, userID uuid.UUID, at *time.Time) error
}

// GormLifecycleRepository implements LifecycleRepository.
type GormLifecycleRepository struct{ db *gorm.DB }

// NewLifecycleRepository returns a GORM lifecycle repository.
func NewLifecycleRepository(db *gorm.DB) *GormLifecycleRepository {
	return &GormLifecycleRepository{db: db}
}

// lifecycleLockKey is the advisory lock of the job (any constant; it only has
// to differ from the other advisory locks).
const lifecycleLockKey int64 = 0x6c696665 // "life"

// WithRunLock implements LifecycleRepository. The lock is session-level on one
// pinned connection for the whole run; fn itself uses the pool.
func (r *GormLifecycleRepository) WithRunLock(ctx context.Context, fn func(ctx context.Context) error) (bool, error) {
	acquired := false
	err := r.db.WithContext(ctx).Connection(func(conn *gorm.DB) error {
		var ok bool
		if err := conn.Raw("SELECT pg_try_advisory_lock(?)", lifecycleLockKey).Scan(&ok).Error; err != nil {
			return err
		}
		if !ok {
			return nil
		}
		acquired = true
		defer func() {
			_ = conn.WithContext(context.WithoutCancel(ctx)).Exec("SELECT pg_advisory_unlock(?)", lifecycleLockKey).Error
		}()
		return fn(ctx)
	})
	return acquired, err
}

// Shared predicates. dueUser keeps the nudges to active, adult-or-unknown,
// non-closing client accounts; dueCap is the global frequency cap.
//
// Every probe is bounded in time as well as by user: a message or a reading of
// a user whose event is after the window's floor cannot be older than the
// floor, and the cap only looks at the last hours. Whichever join the planner
// picks (a nested-loop probe per user, or a hash of the other side), it reads
// an index range, never a whole table.
const (
	dueUser = `u.role = 'client' AND u.status = 'active' AND u.deleted_at IS NULL AND u.closure_scheduled_at IS NULL
		AND u.age_restricted_at IS NULL`
	dueCap = `NOT EXISTS (SELECT 1 FROM lifecycle_messages c WHERE c.user_id = u.id AND c.journey IN @journeys AND c.status = 'sent'
		AND c.created_at > @cap)`
	// dueOnce: the journey's single message (cycle 1) was not recorded yet.
	dueOnce = `NOT EXISTS (SELECT 1 FROM lifecycle_messages m WHERE m.user_id = u.id AND m.journey = @journey AND m.cycle = 1
		AND m.created_at > @floor)`
)

// readerAccount joins each reader of the band to its account, which must pass
// dueUser (plus cond), with one users_pkey probe per reader in band order, so
// the batch stops after @limit rows. OFFSET 0 keeps the subquery a per-reader
// probe: as a plain join the planner over-estimates the accounts that pass
// dueUser and hashes the band against a sequential scan of users (1.3 s on
// the 200k-user load dataset for win_back, 6 ms this way).
func readerAccount(cond string) string {
	return `CROSS JOIN LATERAL (SELECT u.id FROM users u WHERE u.id = r.user_id AND ` + dueUser + cond + ` OFFSET 0) u`
}

// dueRange is the lower bound of a due query on (event column, user id): the
// floor of the window for the first batch, then strictly after the cursor
// (keyset), so a batch never comes back within a run whether its users were
// claimed, lost to another instance or passed over. Each variant states one
// lower bound only, which keeps the planner's estimate of the range exact.
func dueRange(at, id string, after bool) string {
	if !after {
		return at + ` > @floor`
	}
	return at + ` >= @after_at AND (` + at + ` > @after_at OR ` + id + ` > @after_id)`
}

// Due implements LifecycleRepository. Each journey starts from an index range
// on its event time (a created_at range of users, lifecycle_readers_first_idx
// or lifecycle_readers_last_idx) and walks it in keyset order; the probes are
// index lookups by user (lifecycle_messages_once_key or _sent_idx, or the
// time range of _stats_idx; the primary keys of lifecycle_readers and wallets;
// chat_sessions_one_open_per_client; refresh_tokens_user_idx). Checked at
// 100,000 clients by tests/load TestExplainLifecyclePopulated.
func (r *GormLifecycleRepository) Due(ctx context.Context, q LifecycleDueQuery) ([]LifecycleDue, error) {
	var query string
	after := q.After != nil
	switch q.Journey {
	case domain.JourneyVerifyReminder:
		query = `SELECT u.id AS user_id, u.created_at AS event_at, 1 AS cycle, 0 AS free_minutes
			FROM users u
			WHERE u.role = 'client' AND u.created_at <= @cutoff AND ` + dueRange("u.created_at", "u.id", after) + `
			  AND u.email_verified_at IS NULL AND ` + dueUser + ` AND ` + dueOnce + ` AND ` + dueCap + `
			ORDER BY u.created_at, u.id LIMIT @limit`
	case domain.JourneyFirstReading:
		// No completed reading (no reader row: a scalar primary-key probe), none
		// in progress right now (chat_sessions_one_open_per_client) and the 18+
		// check passed: before that a client cannot start one.
		query = `SELECT u.id AS user_id, u.created_at AS event_at, 1 AS cycle,
				COALESCE((SELECT w.free_minutes FROM wallets w WHERE w.user_id = u.id), 0) AS free_minutes
			FROM users u
			WHERE u.role = 'client' AND u.created_at <= @cutoff AND ` + dueRange("u.created_at", "u.id", after) + `
			  AND u.adult_confirmed_at IS NOT NULL AND ` + dueUser + ` AND ` + dueOnce + ` AND ` + dueCap + `
			  AND (SELECT r.readings FROM lifecycle_readers r WHERE r.user_id = u.id) IS NULL
			  AND NOT EXISTS (SELECT 1 FROM chat_sessions s WHERE s.client_id = u.id
			        AND s.status IN ('requested', 'accepted', 'active', 'disconnected', 'held'))
			ORDER BY u.created_at, u.id LIMIT @limit`
	case domain.JourneyAfterFirstReading:
		query = `SELECT u.id AS user_id, r.first_reading_at AS event_at, 1 AS cycle, 0 AS free_minutes
			FROM lifecycle_readers r ` + readerAccount("") + `
			WHERE r.first_reading_at <= @cutoff AND ` + dueRange("r.first_reading_at", "r.user_id", after) + `
			  AND ` + dueOnce + ` AND ` + dueCap + `
			ORDER BY r.first_reading_at, r.user_id LIMIT @limit`
	case domain.JourneyWinBack:
		// A lapsed reader: the last reading is in the band and neither a
		// sign-in nor a session refresh happened since InactiveSince. At most
		// one message per CycleSince and MaxCycles per lapse (since the last
		// reading).
		query = `SELECT u.id AS user_id, r.last_reading_at AS event_at,
				(SELECT COALESCE(max(m.cycle), 0) + 1 FROM lifecycle_messages m WHERE m.user_id = u.id AND m.journey = @journey) AS cycle,
				0 AS free_minutes
			FROM lifecycle_readers r ` + readerAccount(" AND COALESCE(u.last_login_at, u.created_at) <= @inactive") + `
			WHERE r.last_reading_at <= @cutoff AND ` + dueRange("r.last_reading_at", "r.user_id", after) + `
			  AND ` + dueCap + `
			  AND NOT EXISTS (SELECT 1 FROM refresh_tokens t WHERE t.user_id = u.id AND t.revoked_at IS NULL
			        AND GREATEST(t.created_at, COALESCE(t.last_used_at, t.created_at)) > @inactive)
			  AND NOT EXISTS (SELECT 1 FROM lifecycle_messages m WHERE m.user_id = u.id AND m.journey = @journey AND m.created_at > @cycle_since)
			  AND (SELECT count(*) FROM lifecycle_messages m WHERE m.user_id = u.id AND m.journey = @journey AND m.created_at > r.last_reading_at) < @max_cycles
			ORDER BY r.last_reading_at, r.user_id LIMIT @limit`
	default:
		return nil, nil
	}
	var afterAt time.Time
	var afterID uuid.UUID
	if after {
		afterAt, afterID = q.After.At, q.After.UserID
	}
	var out []LifecycleDue
	err := database.Conn(ctx, r.db).Raw(query,
		sql.Named("journey", q.Journey), sql.Named("journeys", domain.LifecycleJourneys), sql.Named("cutoff", q.Cutoff), sql.Named("floor", q.Floor),
		sql.Named("after_at", afterAt), sql.Named("after_id", afterID), sql.Named("inactive", q.InactiveSince),
		sql.Named("cap", q.CapSince), sql.Named("cycle_since", q.CycleSince), sql.Named("max_cycles", max(q.MaxCycles, 1)),
		sql.Named("limit", clampLimit(q.Limit, 500, 500))).Scan(&out).Error
	return out, err
}

// Claim implements LifecycleRepository.
func (r *GormLifecycleRepository) Claim(ctx context.Context, rows []domain.LifecycleMessage) (map[uuid.UUID]bool, error) {
	inserted := map[uuid.UUID]bool{}
	if len(rows) == 0 {
		return inserted, nil
	}
	for start := 0; start < len(rows); start += 500 {
		part := rows[start:min(start+500, len(rows))]
		args := make([]any, 0, len(part)*8)
		var sb strings.Builder
		sb.WriteString("INSERT INTO lifecycle_messages (id, user_id, journey, cycle, status, skip_reason, channels, created_at, updated_at) VALUES ")
		for i := range part {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString("(?, ?, ?, ?, ?, ?, ?::jsonb, ?, ?)")
			ch, _ := json.Marshal(part[i].Channels)
			args = append(args, part[i].ID, part[i].UserID, part[i].Journey, part[i].Cycle, part[i].Status, part[i].SkipReason,
				string(ch), part[i].CreatedAt, part[i].CreatedAt)
		}
		sb.WriteString(" ON CONFLICT (user_id, journey, cycle) DO NOTHING RETURNING id")
		rs, err := database.Conn(ctx, r.db).Raw(sb.String(), args...).Rows()
		if err != nil {
			return nil, err
		}
		for rs.Next() {
			var id uuid.UUID
			if err := rs.Scan(&id); err != nil {
				_ = rs.Close()
				return nil, err
			}
			inserted[id] = true
		}
		if err := rs.Err(); err != nil {
			_ = rs.Close()
			return nil, err
		}
		_ = rs.Close()
	}
	return inserted, nil
}

// Release implements LifecycleRepository.
func (r *GormLifecycleRepository) Release(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	return database.Conn(ctx, r.db).Where("id IN ?", ids).Delete(&domain.LifecycleMessage{}).Error
}

// RecordReaders implements LifecycleRepository. The row is recomputed from the
// client's sessions, so running it twice (a retried hook, the reconciliation)
// changes nothing.
func (r *GormLifecycleRepository) RecordReaders(ctx context.Context, clientIDs []uuid.UUID) error {
	if len(clientIDs) == 0 {
		return nil
	}
	return database.Conn(ctx, r.db).Exec(`
		INSERT INTO lifecycle_readers (user_id, first_reading_at, last_reading_at, readings)
		SELECT client_id, min(settled_at), max(settled_at), count(*)
		FROM chat_sessions
		WHERE client_id IN ? AND status = 'settled' AND billed_minutes > 0 AND settled_at IS NOT NULL
		GROUP BY client_id
		ON CONFLICT (user_id) DO UPDATE SET first_reading_at = EXCLUDED.first_reading_at,
			last_reading_at = EXCLUDED.last_reading_at, readings = EXCLUDED.readings
		WHERE (lifecycle_readers.first_reading_at, lifecycle_readers.last_reading_at, lifecycle_readers.readings)
			IS DISTINCT FROM (EXCLUDED.first_reading_at, EXCLUDED.last_reading_at, EXCLUDED.readings)`, clientIDs).Error
}

// ReconcileReaders implements LifecycleRepository (chat_sessions_reading_settled_idx).
func (r *GormLifecycleRepository) ReconcileReaders(ctx context.Context, since time.Time) (int, error) {
	var ids []uuid.UUID
	err := database.Conn(ctx, r.db).Model(&domain.Session{}).
		Where("status = ? AND billed_minutes > 0 AND settled_at >= ?", domain.SessionSettled, since).
		Distinct().Limit(5000).Pluck("client_id", &ids).Error
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	return len(ids), r.RecordReaders(ctx, ids)
}

// Convert implements LifecycleRepository (lifecycle_messages_open_idx).
func (r *GormLifecycleRepository) Convert(ctx context.Context, userID uuid.UUID, from, to time.Time) (int64, error) {
	res := database.Conn(ctx, r.db).Model(&domain.LifecycleMessage{}).
		Where("user_id = ? AND status = ? AND converted_at IS NULL AND journey <> ? AND created_at >= ? AND created_at <= ?",
			userID, domain.LifecycleSent, domain.JourneyVerifyReminder, from, to).
		Updates(map[string]any{"converted_at": to, "updated_at": to})
	return res.RowsAffected, res.Error
}

// Stats implements LifecycleRepository (lifecycle_messages_stats_idx: one
// range per journey).
func (r *GormLifecycleRepository) Stats(ctx context.Context, now time.Time, attributionDays int) ([]LifecycleStatRow, error) {
	var out []LifecycleStatRow
	// A verification reminder converts when the address was confirmed within
	// the attribution window after it (a primary-key probe of that message's
	// user only); the other journeys when a reading was completed
	// (converted_at, set by the settlement hook).
	err := database.Conn(ctx, r.db).Raw(`
		WITH m AS (
		  SELECT m.journey, m.status, m.skip_reason, m.created_at,
		    (m.converted_at IS NOT NULL OR (m.journey = 'verify_reminder' AND m.status = 'sent' AND EXISTS (
		      SELECT 1 FROM users u WHERE u.id = m.user_id AND u.email_verified_at >= m.created_at
		        AND u.email_verified_at <= m.created_at + make_interval(days => @attr)))) AS converted
		  FROM lifecycle_messages m
		  WHERE m.journey IN @journeys AND m.created_at >= @since30)
		SELECT journey, status, skip_reason,
		  count(*) FILTER (WHERE created_at >= @since7) AS n7, count(*) AS n30,
		  count(*) FILTER (WHERE created_at >= @since7 AND converted) AS converted7,
		  count(*) FILTER (WHERE converted) AS converted30
		FROM m GROUP BY journey, status, skip_reason`,
		sql.Named("attr", attributionDays), sql.Named("journeys", domain.LifecycleJourneys),
		sql.Named("since7", now.AddDate(0, 0, -7)), sql.Named("since30", now.AddDate(0, 0, -30))).
		Scan(&out).Error
	return out, err
}

// Funnel implements LifecycleRepository: the cohort is an index range of
// users_role_created_idx; each step is one primary-key probe of that account's
// profile and reader row (materialized once per account), so the cost follows
// the cohort, not the size of the tables.
func (r *GormLifecycleRepository) Funnel(ctx context.Context, from, to time.Time) (LifecycleFunnel, error) {
	var out LifecycleFunnel
	err := database.Conn(ctx, r.db).Raw(`
		WITH f AS MATERIALIZED (
		  SELECT u.email_verified_at AS verified_at,
		    (SELECT p.birth_date IS NOT NULL AND p.birth_latitude IS NOT NULL FROM user_profiles p WHERE p.user_id = u.id) AS birth,
		    (SELECT lr.readings FROM lifecycle_readers lr WHERE lr.user_id = u.id) AS readings
		  FROM users u
		  WHERE u.role = 'client' AND u.created_at >= @from AND u.created_at < @to)
		SELECT count(*) AS signed_up, count(verified_at) AS verified, count(*) FILTER (WHERE birth) AS birth_details,
		  count(readings) AS first_reading, count(*) FILTER (WHERE readings >= 2) AS second_reading
		FROM f`,
		sql.Named("from", from), sql.Named("to", to)).Scan(&out).Error
	return out, err
}

// Onboarding implements LifecycleRepository: one statement, primary-key
// lookups only.
func (r *GormLifecycleRepository) Onboarding(ctx context.Context, userID uuid.UUID) (OnboardingFacts, error) {
	var out OnboardingFacts
	err := database.Conn(ctx, r.db).Raw(`
		SELECT (u.email_verified_at IS NOT NULL) AS verified,
		  COALESCE(p.birth_date IS NOT NULL AND p.birth_latitude IS NOT NULL, false) AS birth_details,
		  (EXISTS (SELECT 1 FROM lifecycle_readers r WHERE r.user_id = u.id)
		    OR EXISTS (SELECT 1 FROM chat_sessions s WHERE s.client_id = u.id AND s.status = 'settled' AND s.billed_minutes > 0)) AS first_reading,
		  COALESCE(p.onboarding_dismissed_at IS NOT NULL, false) AS dismissed
		FROM users u LEFT JOIN user_profiles p ON p.user_id = u.id
		WHERE u.id = @id`, sql.Named("id", userID)).Scan(&out).Error
	return out, err
}

// SetOnboardingDismissed implements LifecycleRepository. A user without a
// profile row (the table's defaults apply) gets one; updated_at is kept by
// the table's trigger.
func (r *GormLifecycleRepository) SetOnboardingDismissed(ctx context.Context, userID uuid.UUID, at *time.Time) error {
	return database.Conn(ctx, r.db).Table("user_profiles").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"onboarding_dismissed_at"}),
	}).Create(map[string]any{"user_id": userID, "onboarding_dismissed_at": at}).Error
}
