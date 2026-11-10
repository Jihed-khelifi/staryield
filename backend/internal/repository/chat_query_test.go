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
)

// dryRunDB builds statements without a database connection.
func dryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=127.0.0.1 user=x dbname=x sslmode=disable"}),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true, Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return db
}

func inboxSQL(t *testing.T, f InboxFilter) (string, []any) {
	t.Helper()
	var rows []InboxRow
	stmt := inboxQuery(dryRunDB(t), f).Scan(&rows).Statement
	return stmt.SQL.String(), stmt.Vars
}

func TestInboxQueryShape(t *testing.T) {
	user := uuid.New()
	for _, asPsychic := range []bool{true, false} {
		sql, vars := inboxSQL(t, InboxFilter{UserID: user, AsPsychic: asPsychic, Limit: 21})
		for _, want := range []string{
			"FROM conversations AS c",
			"LEFT JOIN chat_messages m ON m.id = c.last_message_id",
			"left(m.body, 141) AS last_body",
			"ORDER BY c.last_activity_at DESC, c.id DESC",
			"LIMIT $2",
		} {
			if !strings.Contains(sql, want) {
				t.Fatalf("psychic=%v: %q missing from %s", asPsychic, want, sql)
			}
		}
		// No per-row subquery: unread counts live on the conversation row.
		if strings.Contains(strings.ToLower(sql), "count(") || strings.Count(sql, "SELECT") != 1 {
			t.Fatalf("inbox must be a single query without subqueries: %s", sql)
		}
		owner, join := "c.psychic_id = $1", "JOIN users u ON u.id = c.client_id"
		if !asPsychic {
			owner, join = "c.client_id = $1", "JOIN psychics ps ON ps.user_id = c.psychic_id"
		}
		if !strings.Contains(sql, owner) || !strings.Contains(sql, join) {
			t.Fatalf("psychic=%v: owner/counterpart join wrong: %s", asPsychic, sql)
		}
		if len(vars) != 2 || vars[0] != user || vars[1] != 21 {
			t.Fatalf("vars: %v", vars)
		}
		// revisions 4.0: the preview carries its reply target id, and only the
		// advisor's inbox joins the advisor's own alias (a LEFT JOIN on the
		// primary key, so still one query and no subquery).
		if !strings.Contains(sql, "m.reply_to_id AS last_reply_to_id") {
			t.Fatalf("psychic=%v: reply target missing: %s", asPsychic, sql)
		}
		aliasJoin := "LEFT JOIN client_aliases ca ON ca.psychic_id = c.psychic_id AND ca.client_id = c.client_id"
		if asPsychic != strings.Contains(sql, aliasJoin) || asPsychic != strings.Contains(sql, "ca.alias_enc AS alias_enc") {
			t.Fatalf("psychic=%v: alias join wrong (the client's inbox must never read aliases): %s", asPsychic, sql)
		}
		if !asPsychic && strings.Contains(sql, "client_aliases") {
			t.Fatalf("the client's inbox touches client_aliases: %s", sql)
		}
	}
	// Keyset continuation and clamping.
	at := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	id := uuid.New()
	sql, vars := inboxSQL(t, InboxFilter{UserID: user, Cursor: &Cursor{At: at, ID: id.String()}, Limit: 500})
	if !strings.Contains(sql, "(c.last_activity_at, c.id) < ($2, $3)") || vars[3] != 51 {
		t.Fatalf("keyset: %s %v", sql, vars)
	}
}

// FuzzDecodeCursor checks that cursor decoding never panics, accepts only
// complete cursors and round-trips what EncodeCursor produces.
func FuzzDecodeCursor(f *testing.F) {
	f.Add(EncodeCursor(time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC), uuid.NewString()))
	for _, s := range []string{"", " ", "%%%", "e30", "eyJ0IjoiIiwiaSI6IiJ9", strings.Repeat("A", 257), "\x00"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		c, err := DecodeCursor(s)
		if err != nil {
			if c != nil {
				t.Fatal("cursor returned with an error")
			}
			return
		}
		if c == nil {
			if strings.TrimSpace(s) != "" {
				t.Fatalf("non-empty %q decoded to nil", s)
			}
			return
		}
		if c.At.IsZero() || c.ID == "" {
			t.Fatalf("incomplete cursor accepted: %+v", c)
		}
		again, err := DecodeCursor(EncodeCursor(c.At, c.ID))
		if err != nil || !again.At.Equal(c.At) || again.ID != c.ID {
			t.Fatalf("round trip: %+v -> %+v %v", c, again, err)
		}
	})
}

// recordedSQL runs fn against a dry-run database and returns the statements
// the repository built.
func recordedSQL(t *testing.T, fn func(db *gorm.DB)) []string {
	t.Helper()
	db := dryRunDB(t)
	var out []string
	capture := func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.SQL.Len() > 0 {
			out = append(out, tx.Statement.SQL.String())
		}
	}
	_ = db.Callback().Query().After("gorm:query").Register("test:capture_query", capture)
	_ = db.Callback().Row().After("gorm:row").Register("test:capture_row", capture)
	fn(db)
	return out
}

// revisions 4.0: the reply-target and likes lookups take a whole page of ids
// and run exactly one statement each, on the primary keys.
func TestReplyAndLikeBatchesAreOneQueryEach(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	ctx := context.Background()

	got := recordedSQL(t, func(db *gorm.DB) { _, _ = NewChatRepository(db).ReplyTargets(ctx, ids) })
	if len(got) != 1 || strings.Count(got[0], "SELECT") != 1 || !strings.Contains(got[0], "FROM \"chat_messages\" WHERE id IN ($1,$2,$3)") {
		t.Fatalf("reply targets: %v", got)
	}
	got = recordedSQL(t, func(db *gorm.DB) { _, _ = NewChatRepository(db).LikesFor(ctx, ids) })
	if len(got) != 1 || strings.Count(got[0], "SELECT") != 1 || !strings.Contains(got[0], "FROM \"message_likes\" WHERE message_id IN ($1,$2,$3)") {
		t.Fatalf("likes: %v", got)
	}
	got = recordedSQL(t, func(db *gorm.DB) {
		_, _ = NewChatRepository(db).ReplyTargets(ctx, nil)
		_, _ = NewChatRepository(db).LikesFor(ctx, nil)
	})
	if len(got) != 0 {
		t.Fatalf("an empty page must not query: %v", got)
	}
	got = recordedSQL(t, func(db *gorm.DB) { _, _ = NewAliasRepository(db).List(ctx, uuid.New(), ids) })
	if len(got) != 1 || !strings.Contains(got[0], "psychic_id = $1 AND client_id IN ($2,$3,$4)") {
		t.Fatalf("alias lookup is one query scoped by the advisor: %v", got)
	}
}
