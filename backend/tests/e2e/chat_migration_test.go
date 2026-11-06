package e2e

import (
	"bytes"
	"context"
	"net/url"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/migrations"
)

// TestChatOrderingMigrationBackfill applies 0007_chat_ordering to a
// database holding pre-existing chat history and checks the backfilled
// sequence numbers, read positions and unread counters.
func TestChatOrderingMigrationBackfill(t *testing.T) {
	admin, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	adminSQL, _ := admin.DB()
	t.Cleanup(func() { _ = adminSQL.Close() })
	name := "staryield_chatmig_" + uuid.NewString()[:8]
	if err := admin.Exec("CREATE DATABASE " + name).Error; err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() { admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)") })
	u, _ := url.Parse(testDSN)
	u.Path = "/" + name
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	exec := func(q string, args ...any) {
		t.Helper()
		if err := db.Exec(q, args...).Error; err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	// Bring the schema to the version just before chat ordering.
	all, err := migrations.Load()
	if err != nil {
		t.Fatal(err)
	}
	exec(`CREATE TABLE schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`)
	for _, m := range all {
		if m.Version >= "0007" {
			break
		}
		exec(m.SQL)
		exec("INSERT INTO schema_migrations (version) VALUES (?)", m.Version)
	}

	// Legacy history: five messages inserted out of order, two sharing a
	// timestamp (the id breaks the tie), one system notice.
	t0 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	client, other, psychic := domain.NewID(), domain.NewID(), domain.NewID()
	exec("INSERT INTO users (id, email, password_hash, role, terms_accepted_at) VALUES (?, ?, 'x', 'client', ?), (?, ?, 'x', 'client', ?), (?, ?, 'x', 'psychic', ?)",
		client, "legacy-client-"+name+"@example.com", t0, other, "legacy-other-"+name+"@example.com", t0,
		psychic, "legacy-psychic-"+name+"@example.com", t0)
	exec("INSERT INTO psychics (user_id, slug, name, rate_per_minute) VALUES (?, ?, 'Legacy', 10)", psychic, "legacy-"+name[len(name)-8:])
	conv, empty, session := domain.NewID(), domain.NewID(), domain.NewID()
	type msg struct {
		id     uuid.UUID
		sender *uuid.UUID
		kind   string
		at     time.Time
	}
	msgs := []msg{
		{domain.NewID(), &client, "text", t0.Add(1 * time.Second)},
		{domain.NewID(), &psychic, "text", t0.Add(2 * time.Second)},
		{domain.NewID(), nil, "system", t0.Add(3 * time.Second)},
		{domain.NewID(), &client, "text", t0.Add(4 * time.Second)},
		{domain.NewID(), &psychic, "image", t0.Add(4 * time.Second)},
	}
	lastAt := msgs[4].at
	exec(`INSERT INTO conversations (id, client_id, psychic_id, last_message_at, client_last_read_at, psychic_last_read_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, conv, client, psychic, lastAt, t0.Add(2*time.Second), lastAt, t0)
	exec(`INSERT INTO conversations (id, client_id, psychic_id, created_at) VALUES (?, ?, ?, ?)`, empty, other, psychic, t0)
	exec(`INSERT INTO chat_sessions (id, conversation_id, client_id, psychic_id, status, rate_per_minute, request_key, requested_at, expires_at)
		VALUES (?, ?, ?, ?, 'settled', 10, 'legacy-request', ?, ?)`, session, conv, client, psychic, t0, t0)
	for _, i := range []int{4, 1, 3, 0, 2} {
		m := msgs[i]
		exec("INSERT INTO chat_messages (id, conversation_id, session_id, sender_id, kind, body, created_at) VALUES (?, ?, ?, ?, ?, 'legacy', ?)",
			m.id, conv, session, m.sender, m.kind, m.at)
	}
	exec("UPDATE conversations SET last_message_id = ? WHERE id = ?", msgs[4].id, conv)

	applied, err := migrations.Up(context.Background(), db, zap.NewNop())
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if len(applied) == 0 || applied[0] != "0007_chat_ordering" {
		t.Fatalf("applied %v", applied)
	}

	// Seq follows (created_at, id): the shared timestamp is ordered by id.
	want := append([]msg(nil), msgs...)
	sort.SliceStable(want, func(i, j int) bool {
		if !want[i].at.Equal(want[j].at) {
			return want[i].at.Before(want[j].at)
		}
		return bytes.Compare(want[i].id[:], want[j].id[:]) < 0
	})
	var rows []struct {
		ID        uuid.UUID
		Seq       int64
		CreatedAt time.Time
		UpdatedAt time.Time
	}
	if err := db.Raw("SELECT id, seq, created_at, updated_at FROM chat_messages WHERE conversation_id = ? ORDER BY seq", conv).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(want) {
		t.Fatalf("%d messages after migration", len(rows))
	}
	for i, r := range rows {
		if r.Seq != int64(i+1) || r.ID != want[i].id || !r.UpdatedAt.Equal(r.CreatedAt) {
			t.Fatalf("message %d: seq=%d id=%s (want %s) updated_at=%v created_at=%v", i, r.Seq, r.ID, want[i].id, r.UpdatedAt, r.CreatedAt)
		}
	}

	var c domain.Conversation
	if err := db.First(&c, "id = ?", conv).Error; err != nil {
		t.Fatal(err)
	}
	// Client read up to t0+2s → seq 2; the advisor read up to t0+4s → seq 5.
	// The client's unread: the system notice and the advisor's image; the
	// advisor has read everything.
	if c.LastSeq != 5 || c.ClientReadSeq != 2 || c.PsychicReadSeq != 5 || c.ClientUnread != 2 || c.PsychicUnread != 0 || !c.LastActivityAt.Equal(lastAt) {
		t.Fatalf("conversation after backfill: last=%d read=%d/%d unread=%d/%d activity=%v", c.LastSeq, c.ClientReadSeq,
			c.PsychicReadSeq, c.ClientUnread, c.PsychicUnread, c.LastActivityAt)
	}
	var e domain.Conversation
	if err := db.First(&e, "id = ?", empty).Error; err != nil {
		t.Fatal(err)
	}
	if e.LastSeq != 0 || e.ClientUnread != 0 || !e.LastActivityAt.Equal(t0) {
		t.Fatalf("empty conversation: %+v", e)
	}

	// The new invariants are enforced by the database.
	if err := db.Exec("INSERT INTO chat_messages (id, conversation_id, session_id, seq, kind, body, created_at, updated_at) VALUES (?, ?, ?, 3, 'system', 'dup', now(), now())",
		domain.NewID(), conv, session).Error; err == nil {
		t.Fatal("duplicate seq accepted")
	}
	if err := db.Exec("UPDATE conversations SET client_read_seq = 9 WHERE id = ?", conv).Error; err == nil {
		t.Fatal("read position past the newest message accepted")
	}
	if err := db.Exec("UPDATE conversations SET client_unread = -1 WHERE id = ?", conv).Error; err == nil {
		t.Fatal("negative unread counter accepted")
	}
}
