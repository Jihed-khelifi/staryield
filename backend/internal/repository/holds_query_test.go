package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 6.0 (§26.1): the hold's queries are single statements on their
// indexes: one batched lookup per page, a SKIP LOCKED claim, one UPDATE to
// unlock a hold's messages and one aggregate for the admin card.

// recordedStatements is recordedSQL that also captures UPDATE statements.
func recordedStatements(t *testing.T, fn func(db *gorm.DB)) []string {
	t.Helper()
	// Writes open a transaction by default, which needs a connection.
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=127.0.0.1 user=x dbname=x sslmode=disable"}),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true, Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	var out []string
	capture := func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.SQL.Len() > 0 {
			out = append(out, tx.Statement.SQL.String())
		}
	}
	_ = db.Callback().Query().After("gorm:query").Register("test:capture_query", capture)
	_ = db.Callback().Row().After("gorm:row").Register("test:capture_row", capture)
	_ = db.Callback().Update().After("gorm:update").Register("test:capture_update", capture)
	fn(db)
	return out
}

func TestHoldQueriesAreSingleStatements(t *testing.T) {
	ctx := context.Background()
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	hold := &domain.ChatHold{ID: ids[0], SessionID: ids[1], StartedAt: now}

	for name, tc := range map[string]struct {
		run  func(r *GormChatRepository)
		want []string
	}{
		"holds of a page": {func(r *GormChatRepository) { _, _ = r.HoldsByIDs(ctx, ids) },
			[]string{`FROM "chat_session_holds" WHERE id IN ($1,$2,$3)`}},
		"claim expired holds": {func(r *GormChatRepository) { _, _ = r.ClaimExpiredHolds(ctx, now, 1) },
			[]string{`FROM "chat_sessions" WHERE status = $1 AND hold_expires_at <= $2`, "ORDER BY hold_expires_at ASC", "FOR UPDATE SKIP LOCKED"}},
		"held readings of a client": {func(r *GormChatRepository) { _, _ = r.HeldSessionIDsForClient(ctx, ids[0]) },
			[]string{`FROM "chat_sessions" WHERE client_id = $1 AND status = $2`}},
		"unlock a hold's messages": {func(r *GormChatRepository) { _, _ = r.UnlockHoldMessages(ctx, hold) },
			[]string{`UPDATE "chat_messages" SET "held"=$1`, `WHERE session_id = $2 AND created_at >= $3 AND hold_id = $4 AND held`}},
		"messages of a hold": {func(r *GormChatRepository) { _, _ = r.ListHoldMessages(ctx, hold) },
			[]string{`FROM "chat_messages" WHERE session_id = $1 AND created_at >= $2 AND hold_id = $3 ORDER BY seq ASC`}},
		"pending purchase since": {func(r *GormChatRepository) { _, _ = r.HasPendingPurchaseSince(ctx, ids[0], now) },
			[]string{`FROM "credit_purchases" WHERE user_id = $1 AND status = $2 AND created_at >= $3`}},
		"one more held message": {func(r *GormChatRepository) { _ = r.IncrementHeldMessages(ctx, ids[0]) },
			[]string{`UPDATE "chat_session_holds" SET "held_messages"=held_messages + 1`}},
		"open hold, locked": {func(r *GormChatRepository) { _, _ = r.OpenHoldForSessionForUpdate(ctx, ids[0]) },
			[]string{`FROM "chat_session_holds" WHERE session_id = $1 AND status = $2`, "FOR UPDATE"}},
		"30-day statistics": {func(r *GormChatRepository) { _, _ = r.HoldStatsSince(ctx, now) },
			[]string{`FROM "chat_session_holds" WHERE started_at >= $1`, "count(*) FILTER (WHERE status = 'converted')"}},
	} {
		t.Run(name, func(t *testing.T) {
			got := recordedStatements(t, func(db *gorm.DB) { tc.run(NewChatRepository(db)) })
			if len(got) != 1 {
				t.Fatalf("want exactly one statement, got %d: %v", len(got), got)
			}
			if strings.Count(got[0], "SELECT") > 1 {
				t.Fatalf("no subqueries: %s", got[0])
			}
			for _, w := range tc.want {
				if !strings.Contains(got[0], w) {
					t.Fatalf("%q missing from %s", w, got[0])
				}
			}
		})
	}
}

func TestEmptyHoldPageDoesNotQuery(t *testing.T) {
	got := recordedSQL(t, func(db *gorm.DB) { _, _ = NewChatRepository(db).HoldsByIDs(context.Background(), nil) })
	if len(got) != 0 {
		t.Fatalf("an empty page must not query: %v", got)
	}
}

// The inbox preview carries what the service needs to lock it.
func TestInboxPreviewCarriesTheHoldFlags(t *testing.T) {
	sql, _ := inboxSQL(t, InboxFilter{UserID: uuid.New(), Limit: 21})
	for _, want := range []string{"m.held AS last_held", "m.hold_id AS last_hold_id", "char_length(m.body) AS last_body_len"} {
		if !strings.Contains(sql, want) {
			t.Fatalf("%q missing from %s", want, sql)
		}
	}
	if strings.Count(sql, "SELECT") != 1 {
		t.Fatalf("still one query: %s", sql)
	}
}

// Reply targets say whether the quoted message is hidden.
func TestReplyTargetsCarryTheHeldFlag(t *testing.T) {
	got := recordedSQL(t, func(db *gorm.DB) {
		_, _ = NewChatRepository(db).ReplyTargets(context.Background(), []uuid.UUID{uuid.New()})
	})
	if len(got) != 1 || !strings.Contains(got[0], "SELECT id, seq, sender_id, kind, body, held FROM") {
		t.Fatalf("reply targets: %v", got)
	}
}
