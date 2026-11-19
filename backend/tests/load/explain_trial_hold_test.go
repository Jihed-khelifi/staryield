//go:build load

package load

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 6.0 (§26.1): the free-trial hold's queries. Two checks:
//
//   - trialHoldChecks join TestExplainHotQueries (read-only queries on the
//     dataset as it is: the plans must not scan big tables);
//   - TestExplainTrialHoldPopulated gives the hold tables real volume inside a
//     transaction that is rolled back (nothing is left behind), so the plans of
//     the expiry claim, a client's held readings, a page of holds and a hold's
//     messages are checked against the indexes they were built for, and the
//     two writes (unlock a hold's messages, count a held message) are explained
//     without changing anything.

type trialHoldSample struct {
	session  uuid.UUID // the busiest reading (most messages)
	started  time.Time // when its first message was written
	holdIDs  []uuid.UUID
	conv     uuid.UUID
	psychic  uuid.UUID
	clientID uuid.UUID
}

func pickTrialHold(db *gorm.DB, ids sampleIDs) trialHoldSample {
	s := trialHoldSample{session: uuid.New(), started: time.Now().Add(-time.Hour), conv: ids.busyConversation, psychic: ids.sessionPsychic, clientID: ids.sessionClient}
	var row struct {
		SessionID uuid.UUID
		First     time.Time
	}
	if db.Raw(`SELECT session_id, min(created_at) AS first FROM chat_messages WHERE session_id IS NOT NULL
		AND session_id = (SELECT session_id FROM chat_messages WHERE conversation_id = ? AND session_id IS NOT NULL ORDER BY seq DESC LIMIT 1)
		GROUP BY session_id`, ids.busyConversation).Scan(&row).Error == nil && row.SessionID != uuid.Nil {
		s.session, s.started = row.SessionID, row.First
	}
	for i := 0; i < 50; i++ {
		s.holdIDs = append(s.holdIDs, uuid.New())
	}
	return s
}

func trialHoldChecks(db *gorm.DB, ids sampleIDs) []queryCheck {
	s := pickTrialHold(db, ids)
	hold := &domain.ChatHold{ID: uuid.New(), SessionID: s.session, StartedAt: s.started}
	return []queryCheck{
		{"holds: expiry claim (SKIP LOCKED)", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).ClaimExpiredHolds(ctx, time.Now(), 100)
		}},
		{"holds: held readings of a client", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).HeldSessionIDsForClient(ctx, ids.sessionClient)
		}},
		{"holds: a page of holds (50)", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).HoldsByIDs(ctx, s.holdIDs)
		}},
		{"holds: messages of a hold (busiest reading)", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).ListHoldMessages(ctx, hold)
		}},
		{"holds: pending purchase since the hold began", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).HasPendingPurchaseSince(ctx, ids.ledgerUser, monthAgo())
		}},
		{"holds: 30-day statistics", 50 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).HoldStatsSince(ctx, monthAgo())
		}},
		// The locked-message reads are the ordinary timeline reads: the held flag
		// is a column of the same rows ("chat: history before seq", "chat: catch-up
		// after seq", "chat: saved messages page", "chat: inbox", "reply targets").
	}
}

// explainTx runs EXPLAIN (ANALYZE, FORMAT JSON) for sql inside tx.
func explainTx(t *testing.T, tx *gorm.DB, name, sql string) explainOut {
	t.Helper()
	var raw string
	if err := tx.Raw("EXPLAIN (ANALYZE, FORMAT JSON) " + sql).Row().Scan(&raw); err != nil {
		t.Fatalf("%s: explain: %v\n%s", name, err, sql)
	}
	var out []explainOut
	if err := json.Unmarshal([]byte(raw), &out); err != nil || len(out) == 0 {
		t.Fatalf("%s: parse plan: %v", name, err)
	}
	return out[0]
}

// usesIndexOn reports whether the plan reads through index (and returns the
// labels of every node for the log). The table is named by the index.
func usesIndexOn(plan explainOut, _, index string) (bool, string) {
	var nodes []string
	used := false
	walk(plan.Plan, func(n planNode) {
		label := n.NodeType
		if n.Relation != "" {
			label += " on " + n.Relation
		}
		if n.Index != "" {
			label += " using " + n.Index
		}
		nodes = append(nodes, label)
		// A bitmap index scan names the index but not the table (its heap scan does).
		if n.Index == index {
			used = true
		}
	})
	return used, strings.Join(nodes, " → ")
}

func TestExplainTrialHoldPopulated(t *testing.T) {
	cfg := LoadConfig()
	live, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("database %s: %v", cfg.DSN, err)
	}
	var ready bool
	live.Raw(`SELECT to_regclass('chat_session_holds') IS NOT NULL`).Scan(&ready)
	if !ready {
		t.Skip("migration 0032 is not applied to the load database")
	}
	dry, rec := newDryDB(t, cfg.DSN)
	ctx := context.Background()
	ids := pickIDs(t, live)

	const holds = 20000
	tx := live.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback() // nothing of this test is ever committed

	// 20000 holds over real readings, 2000 of them held (one open reading per
	// client and per advisor, so the held ones are chosen distinct) with their
	// deadline in the past, plus 3 held messages on each.
	if err := tx.Exec(`
		INSERT INTO chat_session_holds (id, session_id, conversation_id, client_id, psychic_id, reason, status, started_at, expires_at, max_expires_at,
			resolved_at, price_credits, list_rate_credits, held_messages, created_at, updated_at)
		SELECT gen_random_uuid(), s.id, s.conversation_id, s.client_id, s.psychic_id,
			CASE WHEN s.free_minutes_used > 0 THEN 'trial_ended' ELSE 'credits_exhausted' END, 'expired',
			s.requested_at, s.requested_at + interval '60 seconds', s.requested_at + interval '180 seconds', s.requested_at + interval '60 seconds',
			s.rate_per_minute, s.list_rate_per_minute, 3, s.requested_at, s.requested_at
		FROM chat_sessions s WHERE s.started_at IS NOT NULL ORDER BY s.requested_at DESC LIMIT ?`, holds).Error; err != nil {
		t.Fatal(err)
	}
	// Open holds on the 2000 newest readings whose client and advisor are distinct.
	if err := tx.Exec(`
		WITH picked AS (
			SELECT DISTINCT ON (s.psychic_id) s.id, s.client_id, s.psychic_id, s.requested_at
			FROM chat_sessions s WHERE s.started_at IS NOT NULL ORDER BY s.psychic_id, s.requested_at DESC),
		one_per_client AS (
			SELECT DISTINCT ON (client_id) id, client_id, psychic_id FROM picked ORDER BY client_id, requested_at DESC LIMIT 2000)
		UPDATE chat_sessions s SET status = 'held', hold_expires_at = now() - interval '5 seconds', end_reason = ''
		FROM one_per_client p WHERE s.id = p.id`).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(`
		INSERT INTO chat_session_holds (id, session_id, conversation_id, client_id, psychic_id, reason, status, started_at, expires_at, max_expires_at,
			price_credits, list_rate_credits, held_messages, created_at, updated_at)
		SELECT gen_random_uuid(), s.id, s.conversation_id, s.client_id, s.psychic_id, 'trial_ended', 'open', now() - interval '65 seconds',
			now() - interval '5 seconds', now() + interval '115 seconds', s.rate_per_minute, s.list_rate_per_minute, 0, now(), now()
		FROM chat_sessions s WHERE s.status = 'held'`).Error; err != nil {
		t.Fatal(err)
	}
	var open int64
	tx.Raw(`SELECT count(*) FROM chat_sessions WHERE status = 'held'`).Scan(&open)
	if open == 0 {
		t.Skip("the dataset has no readings to hold")
	}
	tx.Exec(`ANALYZE chat_session_holds`)
	tx.Exec(`ANALYZE chat_sessions`)

	var heldClient, heldSession uuid.UUID
	var heldFrom time.Time
	tx.Raw(`SELECT client_id, id, started_at FROM chat_sessions WHERE status = 'held' LIMIT 1`).Row().Scan(&heldClient, &heldSession, &heldFrom)
	var pageIDs []uuid.UUID
	tx.Raw(`SELECT id FROM chat_session_holds ORDER BY random() LIMIT 50`).Scan(&pageIDs)
	var busy struct {
		SessionID uuid.UUID
		First     time.Time
	}
	tx.Raw(`SELECT session_id, min(created_at) AS first FROM chat_messages
		WHERE session_id = (SELECT session_id FROM chat_messages WHERE conversation_id = ? AND session_id IS NOT NULL ORDER BY seq DESC LIMIT 1) GROUP BY session_id`,
		ids.busyConversation).Scan(&busy)
	if busy.SessionID == uuid.Nil {
		busy.SessionID, busy.First = heldSession, time.Now().Add(-time.Hour)
	}
	hold := &domain.ChatHold{ID: uuid.New(), SessionID: busy.SessionID, StartedAt: busy.First}

	type planCase struct {
		name   string
		build  func(ctx context.Context, d *gorm.DB)
		index  string // the index the plan must read the table through ("" = only no sequential scan)
		table  string
		budget time.Duration
	}
	cases := []planCase{
		{"expiry claim (SKIP LOCKED)", func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).ClaimExpiredHolds(ctx, time.Now(), 100)
		}, "chat_sessions_held_idx", "chat_sessions", 25 * time.Millisecond},
		{"held readings of a client", func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).HeldSessionIDsForClient(ctx, heldClient)
		}, "", "chat_sessions", 25 * time.Millisecond},
		{"a page of holds", func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).HoldsByIDs(ctx, pageIDs)
		}, "chat_session_holds_pkey", "chat_session_holds", 25 * time.Millisecond},
		{"the messages of a hold", func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).ListHoldMessages(ctx, hold)
		}, "chat_messages_session_idx", "chat_messages", 25 * time.Millisecond},
	}
	for _, tc := range cases {
		tc.build(ctx, dry)
		sql := rec.take()
		if sql == "" {
			t.Fatalf("%s: no SQL captured", tc.name)
		}
		plan := explainTx(t, tx, tc.name, sql)
		var label string
		if tc.index != "" {
			var ok bool
			if ok, label = usesIndexOn(plan, tc.table, tc.index); !ok {
				t.Errorf("%s: %s is not read through %s: %s\n%s", tc.name, tc.table, tc.index, label, sql)
			}
		} else {
			_, label = usesIndexOn(plan, tc.table, "")
			walk(plan.Plan, func(n planNode) {
				if n.NodeType == "Seq Scan" && n.Relation == tc.table {
					t.Errorf("%s: sequential scan on %s: %s\n%s", tc.name, tc.table, label, sql)
				}
			})
		}
		if d := time.Duration(plan.ExecutionTime * float64(time.Millisecond)); d > tc.budget {
			t.Errorf("%s: execution %s exceeds %s", tc.name, d, tc.budget)
		}
		t.Logf("%-28s %6.2f ms  %s", tc.name, plan.ExecutionTime, label)
	}

	// The two writes, explained inside the rolled-back transaction.
	for name, sql := range map[string]string{
		"unlock a hold's messages": fmt.Sprintf(`UPDATE chat_messages SET held = false WHERE session_id = '%s' AND created_at >= '%s' AND hold_id = '%s' AND held`,
			hold.SessionID, hold.StartedAt.Add(-10*time.Minute).UTC().Format(time.RFC3339Nano), hold.ID),
		"count a held message": fmt.Sprintf(`UPDATE chat_session_holds SET held_messages = held_messages + 1 WHERE id = '%s'`, uuid.New()),
	} {
		plan := explainTx(t, tx, name, sql)
		var nodes []string
		walk(plan.Plan, func(n planNode) {
			nodes = append(nodes, n.NodeType+" "+n.Relation+" "+n.Index)
			if n.NodeType == "Seq Scan" && n.Relation != "" && estimatedRows(live, n.Relation) > seqScanRowLimit {
				t.Errorf("%s: sequential scan on %s", name, n.Relation)
			}
		})
		t.Logf("%-28s %6.2f ms  %s", name, plan.ExecutionTime, strings.Join(nodes, " → "))
	}
}
