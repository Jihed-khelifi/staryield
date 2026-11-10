package e2e

// Shared helpers of the revisions 4.0 chat tests (replies, likes, aliases).

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/app"
	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// roomy lifts the message rate limit so a test can send a lot of messages.
func roomy(cfg *config.Config) { cfg.Chat.MessageRate = 100000 }

// sayReply posts a text message addressed to the conversation, optionally
// quoting a message ("" for a plain message).
func (c *client) sayReply(conv uuid.UUID, body, cmid, replyTo string) response {
	c.e.t.Helper()
	payload := map[string]any{"body": body}
	if cmid != "" {
		payload["client_msg_id"] = cmid
	}
	if replyTo != "" {
		payload["reply_to_id"] = replyTo
	}
	return c.call(http.MethodPost, convMessages(conv), payload)
}

// mustSay sends a message and returns it.
func (c *client) mustSay(conv uuid.UUID, body, replyTo string) service.MessageView {
	c.e.t.Helper()
	r := c.sayReply(conv, body, uniq("msg-"), replyTo)
	if r.status != http.StatusCreated {
		c.e.t.Fatalf("send %q: %d %s", body, r.status, r.body)
	}
	var out sendOut
	r.decode(c.e.t, &out)
	return out.Message
}

// messageEvent decodes the message of a message.created event.
func messageEvent(t *testing.T, data json.RawMessage) service.MessageView {
	t.Helper()
	var out struct {
		Message service.MessageView `json:"message"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode message event: %v (%s)", err, data)
	}
	return out.Message
}

// pending returns every event still queued on the socket (waiting briefly
// for stragglers), without consuming anything else.
func (w *ws) pending() []realtime.Envelope {
	var out []realtime.Envelope
	for {
		e, ok := w.next(time.After(200 * time.Millisecond))
		if !ok {
			return out
		}
		out = append(out, e)
	}
}

// sqlLog is the list of statements the application ran while one test
// action executed (run one action at a time per environment).
type sqlLog struct {
	mu   sync.Mutex
	stmt []string
}

// sqlRecorder feeds the current sqlLog, if any. Its gorm callbacks are
// registered once, before the application starts (see newEnv): registering
// or removing callbacks later would race with the background workers'
// queries, because gorm's callback registry is not synchronised.
type sqlRecorder struct{ cur atomic.Pointer[sqlLog] }

func (r *sqlRecorder) observe(tx *gorm.DB) {
	if l := r.cur.Load(); l != nil {
		l.add(tx)
	}
}

func installSQLRecorder(t *testing.T, db *gorm.DB) *sqlRecorder {
	t.Helper()
	r := &sqlRecorder{}
	if err := db.Callback().Query().After("gorm:query").Register("e2e:sql:query", r.observe); err != nil {
		t.Fatal(err)
	}
	if err := db.Callback().Row().After("gorm:row").Register("e2e:sql:row", r.observe); err != nil {
		t.Fatal(err)
	}
	return r
}

func (l *sqlLog) add(tx *gorm.DB) {
	if tx.Statement == nil || tx.Statement.SQL.Len() == 0 {
		return
	}
	l.mu.Lock()
	l.stmt = append(l.stmt, tx.Statement.SQL.String())
	l.mu.Unlock()
}

// count returns how many recorded statements contain every fragment.
func (l *sqlLog) count(fragments ...string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
next:
	for _, s := range l.stmt {
		for _, f := range fragments {
			if !strings.Contains(s, f) {
				continue next
			}
		}
		n++
	}
	return n
}

// selects returns the number of recorded SELECT statements.
func (l *sqlLog) selects() int { return l.count("SELECT") }

// recordSQL runs fn and returns the SELECT/row statements the application
// ran meanwhile.
func recordSQL(e *env, fn func()) *sqlLog {
	e.t.Helper()
	if e.sql == nil {
		e.t.Fatal("this environment has no SQL recorder (create it with newEnv)")
	}
	l := &sqlLog{}
	e.sql.cur.Store(l)
	defer e.sql.cur.Store(nil)
	fn()
	return l
}

// exportFiles requests, builds and downloads the caller's data export and
// returns the archive's files.
func exportFiles(t *testing.T, e *env, c *client) map[string][]byte {
	t.Helper()
	var job service.ExportView
	c.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/exports", map[string]any{"password": strongPassword}, &job)
	if err := e.app.Scheduler.RunJob(context.Background(), app.ExportJob); err != nil {
		t.Fatal(err)
	}
	var link service.DownloadLink
	c.must(http.StatusOK, http.MethodPost, "/api/v1/me/exports/"+job.ID.String()+"/download-token", map[string]any{"password": strongPassword}, &link)
	return readZip(t, c.raw(http.MethodGet, link.URL, nil, nil).body)
}

// runClosure runs the account closure job (after the grace period).
func runClosure(t *testing.T, e *env) {
	t.Helper()
	if err := e.app.Scheduler.RunJob(context.Background(), app.ClosureJob); err != nil {
		t.Fatal(err)
	}
}

// likeRows counts the likes on a conversation's messages (the test database
// is shared by the whole suite).
func (e *env) likeRows(conv uuid.UUID) int64 {
	e.t.Helper()
	var n int64
	if err := e.app.DB.Table("message_likes").Where("message_id IN (SELECT id FROM chat_messages WHERE conversation_id = ?)", conv).Count(&n).Error; err != nil {
		e.t.Fatal(err)
	}
	return n
}
