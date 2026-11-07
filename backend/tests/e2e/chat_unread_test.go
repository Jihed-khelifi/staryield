package e2e

// revisions 3.0: chat unread (spec §18).

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

type unreadFrame struct {
	Count              int64     `json:"count"`
	Conversations      int64     `json:"conversations"`
	ConversationID     uuid.UUID `json:"conversation_id"`
	ConversationUnread int64     `json:"conversation_unread"`
}

func (c *client) unread() service.UnreadView {
	c.e.t.Helper()
	var v service.UnreadView
	c.must(http.StatusOK, http.MethodGet, "/api/v1/chat/unread-count", nil, &v)
	return v
}

func decodeUnread(t *testing.T, ev realtime.Envelope) unreadFrame {
	t.Helper()
	var f unreadFrame
	if err := json.Unmarshal(ev.Data, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// say sends a text message over the websocket and returns once it is acked.
func say(t *testing.T, w *ws, s service.SessionView, body string) {
	t.Helper()
	w.mustAck(realtime.FrameMessageSend, map[string]any{"session_id": s.ID, "body": body, "client_msg_id": uniq("unread-")})
}

// waitUnread waits for a chat.unread frame with the given total.
func waitUnread(t *testing.T, w *ws, count int64) unreadFrame {
	t.Helper()
	return decodeUnread(t, w.waitMatch(realtime.EventChatUnread, fmt.Sprintf(`"count":%d}`, count), 5*time.Second))
}

func TestChatUnreadCountEndpointAuth(t *testing.T) {
	e := newEnv(t)
	if r := e.client().call(http.MethodGet, "/api/v1/chat/unread-count", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", r.status)
	}
	c := e.registerClient()
	if v := c.unread(); v.Count != 0 || v.Conversations != 0 {
		t.Fatalf("new client: %+v", v)
	}
	r := c.call(http.MethodGet, "/api/v1/chat/unread-count", nil)
	if !strings.Contains(r.header.Get("Cache-Control"), "no-store") {
		t.Fatalf("cache-control %q", r.header.Get("Cache-Control"))
	}
	if r := e.adminClient().call(http.MethodGet, "/api/v1/chat/unread-count", nil); r.status == http.StatusOK {
		t.Fatalf("admin console session read a chat badge: %d", r.status)
	}
}

func TestChatUnreadLiveSession(t *testing.T) {
	e := newEnv(t)
	client, advisor, cws, pws, s := startedSession(t, e, 5, 0)
	other := e.registerClient() // unaffected bystander
	cws2 := client.connect("")  // the client's second tab
	pws2 := advisor.connect("") // the advisor's second tab
	time.Sleep(200 * time.Millisecond)
	cws.drain()
	cws2.drain()
	pws.drain()
	pws2.drain()

	// The session-start notice counts as unread for both: take the baseline.
	cBase, pBase := client.unread(), advisor.unread()
	if cBase.Count != pBase.Count || cBase.Conversations != 1 || pBase.Conversations != 1 {
		t.Fatalf("baselines: client %+v advisor %+v", cBase, pBase)
	}

	// The advisor writes: the client's count rises, over REST and live (on
	// both tabs); the advisor's own total drops (replying reads the thread).
	say(t, pws, s, "I see a new chapter opening.")
	f := waitUnread(t, cws, cBase.Count+1)
	if f.Conversations != 1 || f.ConversationID != s.ConversationID || f.ConversationUnread != cBase.Count+1 {
		t.Fatalf("frame: %+v", f)
	}
	waitUnread(t, cws2, cBase.Count+1)
	if v := client.unread(); v.Count != cBase.Count+1 {
		t.Fatalf("client REST after message: %+v", v)
	}
	if v := advisor.unread(); v.Count != 0 || v.Conversations != 0 {
		t.Fatalf("sender total after replying: %+v", v)
	}
	waitUnread(t, pws, 0)
	waitUnread(t, pws2, 0)
	if v := other.unread(); v.Count != 0 {
		t.Fatalf("bystander: %+v", v)
	}
	other.connect("").expectNone(realtime.EventChatUnread, 100*time.Millisecond)

	// The client writes back: the advisor's count rises, the client's own
	// total drops to zero (replying reads the thread) on both of their tabs.
	say(t, cws, s, "Tell me more.")
	waitUnread(t, pws, 1)
	waitUnread(t, pws2, 1)
	waitUnread(t, cws, 0)
	waitUnread(t, cws2, 0)
	if v := advisor.unread(); v.Count != 1 || v.Conversations != 1 {
		t.Fatalf("advisor REST: %+v", v)
	}
	if v := client.unread(); v.Count != 0 {
		t.Fatalf("client after replying: %+v", v)
	}

	// The advisor writes twice more; the client reads (REST): zero everywhere
	// and the other tab hears it.
	say(t, pws, s, "One more thing.")
	say(t, pws, s, "And another.")
	waitUnread(t, cws, 2)
	if v := client.unread(); v.Count != 2 || v.Conversations != 1 {
		t.Fatalf("client before read: %+v", v)
	}
	cws.drain()
	cws2.drain()
	client.must(http.StatusOK, http.MethodPost, convPath(s.ConversationID, "/read"), map[string]any{"seq": 1 << 30}, nil)
	if v := client.unread(); v.Count != 0 || v.Conversations != 0 {
		t.Fatalf("client after read: %+v", v)
	}
	f = waitUnread(t, cws2, 0)
	if f.ConversationUnread != 0 || f.ConversationID != s.ConversationID || f.Conversations != 0 {
		t.Fatalf("read frame: %+v", f)
	}
	waitUnread(t, cws, 0)
	if v := advisor.unread(); v.Count != 0 {
		t.Fatalf("advisor total moved by the client's read: %+v", v)
	}

	// The advisor reads through the websocket frame: the other tab clears.
	say(t, cws, s, "Ping")
	waitUnread(t, pws, 1)
	pws.drain()
	pws2.drain()
	pws.mustAck(realtime.FrameMessageRead, map[string]any{"conversation_id": s.ConversationID, "seq": 1 << 30})
	waitUnread(t, pws2, 0)
	waitUnread(t, pws, 0)
	if v := advisor.unread(); v.Count != 0 || v.Conversations != 0 {
		t.Fatalf("advisor after read: %+v", v)
	}
	// Reading again changes nothing and publishes nothing.
	pws2.drain()
	advisor.must(http.StatusOK, http.MethodPost, convPath(s.ConversationID, "/read"), map[string]any{"seq": 1}, nil)
	pws2.expectNone(realtime.EventChatUnread, 300*time.Millisecond)
}

func TestChatUnreadBurstAndCache(t *testing.T) {
	e := newEnv(t)
	client, advisor, cws, _, s := startedSession(t, e, 5, 0)
	base := client.unread() // warms the cache
	cws.drain()
	const n = 25
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			advisor.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/messages",
				map[string]any{"body": fmt.Sprintf("m%d", i), "client_msg_id": fmt.Sprintf("burst-%04d-x", i)}, nil)
		}(i)
	}
	wg.Wait()
	// The cached total never lags a stored message.
	if v := client.unread(); v.Count != base.Count+n {
		t.Fatalf("after burst: %+v want %d", v, base.Count+n)
	}
	waitUnread(t, cws, base.Count+n)
	// A cache hit is served from Redis and still correct on repeat.
	for i := 0; i < 3; i++ {
		if v := client.unread(); v.Count != base.Count+n {
			t.Fatalf("repeat: %+v", v)
		}
	}
}

// insertUnreadConversations links client and advisor accounts created in
// bulk (one SQL statement per table, generate_series) and returns the
// expected totals. Rows are removed again when the test ends.
func bulkAccounts(t *testing.T, e *env, prefix string, n int, role string) {
	t.Helper()
	if err := e.app.DB.Exec(`INSERT INTO users (id, email, password_hash, role, display_name, terms_accepted_at)
		SELECT gen_random_uuid(), ? || i || '@bulk.example.com', 'x', ?, 'Bulk ' || i, now() FROM generate_series(1, ?) AS i`, prefix, role, n).Error; err != nil {
		t.Fatal(err)
	}
	if role == "psychic" {
		if err := e.app.DB.Exec(`INSERT INTO psychics (user_id, slug, name, rate_per_minute, listed)
			SELECT id, 'bulk-' || replace(id::text, '-', ''), 'Bulk advisor', 10, false FROM users WHERE email LIKE ? AND role = 'psychic'`, prefix+"%@bulk.example.com").Error; err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		pattern := prefix + "%@bulk.example.com"
		e.app.DB.Exec(`DELETE FROM conversations WHERE client_id IN (SELECT id FROM users WHERE email LIKE ?)
			OR psychic_id IN (SELECT id FROM users WHERE email LIKE ?)`, pattern, pattern)
		e.app.DB.Exec(`DELETE FROM psychics WHERE user_id IN (SELECT id FROM users WHERE email LIKE ?)`, pattern)
		e.app.DB.Exec(`DELETE FROM users WHERE email LIKE ?`, pattern)
	})
}

func TestChatUnreadMultipleConversationsAndAdvisorWithManyClients(t *testing.T) {
	e := newEnv(t)
	advisor := e.createPsychic(10)
	client := e.registerClient()
	bystander := e.registerClient()
	const clients, advisors = 60, 6
	bulkAccounts(t, e, "ucl", clients, "client")
	bulkAccounts(t, e, "uad", advisors, "psychic")

	// The advisor has `clients` conversations with i%4 unread each (0 for
	// some), and the client has `advisors` conversations with i+1 unread.
	if err := e.app.DB.Exec(`INSERT INTO conversations (id, client_id, psychic_id, last_seq, psychic_unread, last_activity_at)
		SELECT gen_random_uuid(), u.id, ?, 10, (row_number() OVER (ORDER BY u.id)) % 4, now()
		FROM users u WHERE u.email LIKE 'ucl%@bulk.example.com'`, advisor.user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := e.app.DB.Exec(`INSERT INTO conversations (id, client_id, psychic_id, last_seq, client_unread, last_activity_at)
		SELECT gen_random_uuid(), ?, u.id, 10, (row_number() OVER (ORDER BY u.id)), now()
		FROM users u WHERE u.email LIKE 'uad%@bulk.example.com'`, client.user.ID).Error; err != nil {
		t.Fatal(err)
	}
	var wantSum, wantConvs int64
	for i := 1; i <= clients; i++ {
		if i%4 != 0 {
			wantSum += int64(i % 4)
			wantConvs++
		}
	}
	if v := advisor.unread(); v.Count != wantSum || v.Conversations != wantConvs {
		t.Fatalf("advisor: %+v want %d/%d", v, wantSum, wantConvs)
	}
	if v := client.unread(); v.Count != advisors*(advisors+1)/2 || v.Conversations != advisors {
		t.Fatalf("client: %+v", v)
	}
	if v := bystander.unread(); v.Count != 0 || v.Conversations != 0 {
		t.Fatalf("bystander: %+v", v)
	}
	// Reading one of the client's conversations lowers the sum by exactly
	// its unread count and the conversation total by one.
	var inbox service.InboxPage
	client.must(http.StatusOK, http.MethodGet, "/api/v1/chat/conversations", nil, &inbox)
	pick := inbox.Items[0]
	client.must(http.StatusOK, http.MethodPost, convPath(pick.ID, "/read"), map[string]any{"seq": 10}, nil)
	if v := client.unread(); v.Count != advisors*(advisors+1)/2-pick.Unread || v.Conversations != advisors-1 {
		t.Fatalf("client after reading %+v: %+v", pick, v)
	}
}

type unreadTotals struct{ Sum, Convs int64 }

type unreadProbe struct {
	c    *client
	want unreadTotals
}

// One advisor with 20k conversations and one client with 2k: the totals are
// right and come back fast (the partial indexes make it an index-only sum).
func TestChatUnreadLargeDataset(t *testing.T) {
	if testing.Short() {
		t.Skip("large dataset")
	}
	e := newEnv(t)
	advisor := e.createPsychic(10)
	client := e.registerClient()
	const clients, advisors = 20000, 2000
	bulkAccounts(t, e, "lcl", clients, "client")
	bulkAccounts(t, e, "lad", advisors, "psychic")
	// Mixed: every third conversation has 1..7 unread messages.
	if err := e.app.DB.Exec(`INSERT INTO conversations (id, client_id, psychic_id, last_seq, psychic_unread, last_activity_at)
		SELECT gen_random_uuid(), id, ?, 10, CASE WHEN n % 3 = 0 THEN n % 7 + 1 ELSE 0 END, now()
		FROM (SELECT id, row_number() OVER (ORDER BY id) AS n FROM users WHERE email LIKE 'lcl%@bulk.example.com') q`, advisor.user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := e.app.DB.Exec(`INSERT INTO conversations (id, client_id, psychic_id, last_seq, client_unread, last_activity_at)
		SELECT gen_random_uuid(), ?, id, 10, CASE WHEN n % 2 = 0 THEN n % 5 + 1 ELSE 0 END, now()
		FROM (SELECT id, row_number() OVER (ORDER BY id) AS n FROM users WHERE email LIKE 'lad%@bulk.example.com') q`, client.user.ID).Error; err != nil {
		t.Fatal(err)
	}
	e.app.DB.Exec("ANALYZE conversations")
	var wantA, wantC unreadTotals
	e.app.DB.Raw("SELECT COALESCE(SUM(psychic_unread),0) AS sum, COUNT(*) FILTER (WHERE psychic_unread > 0) AS convs FROM conversations WHERE psychic_id = ?", advisor.user.ID).Scan(&wantA)
	e.app.DB.Raw("SELECT COALESCE(SUM(client_unread),0) AS sum, COUNT(*) FILTER (WHERE client_unread > 0) AS convs FROM conversations WHERE client_id = ?", client.user.ID).Scan(&wantC)
	if wantA.Convs < 6000 || wantC.Convs < 900 {
		t.Fatalf("seed looks wrong: %+v %+v", wantA, wantC)
	}
	for name, tc := range map[string]unreadProbe{"advisor": {advisor, wantA}, "client": {client, wantC}} {
		start := time.Now()
		v := tc.c.unread()
		first := time.Since(start)
		if v.Count != tc.want.Sum || v.Conversations != tc.want.Convs {
			t.Fatalf("%s: %+v want %+v", name, v, tc.want)
		}
		start = time.Now()
		for i := 0; i < 20; i++ {
			tc.c.unread() // cached
		}
		t.Logf("%s: %d unread in %d conversations; first %v, 20 cached calls %v", name, v.Count, v.Conversations, first, time.Since(start))
		if first > 2*time.Second {
			t.Fatalf("%s total took %v", name, first)
		}
	}
	// A new message lands on top of the big total immediately (cache invalidated).
	var inbox service.InboxPage
	client.must(http.StatusOK, http.MethodGet, "/api/v1/chat/conversations", nil, &inbox)
	pick := inbox.Items[0]
	client.must(http.StatusOK, http.MethodPost, convPath(pick.ID, "/read"), map[string]any{"seq": 10}, nil)
	if v := client.unread(); v.Count != wantC.Sum-pick.Unread || (pick.Unread > 0 && v.Conversations != wantC.Convs-1) {
		t.Fatalf("after reading %+v: %+v want sum %d", pick, v, wantC.Sum-pick.Unread)
	}
}

func TestChatMessagePushOnlyWhenOffline(t *testing.T) {
	e := newEnv(t, withPush(t))
	client, advisor, cws, pws, s := startedSession(t, e, 5, 0)
	_ = cws
	endpoint := "https://fcm.googleapis.com/fcm/send/" + uniq("adv")
	advisor.must(http.StatusCreated, http.MethodPost, "/api/v1/me/push-subscriptions", browserSubscription(t, endpoint), nil)
	e.setPrefs(advisor.user.ID, map[string]any{"notify_push": true})
	if err := e.app.DB.Model(&domain.User{}).Where("id = ?", advisor.user.ID).Update("locale", "fr").Error; err != nil {
		t.Fatal(err)
	}
	chatPushes := func() []pushSend {
		var out []pushSend
		for _, p := range e.push.Sent() {
			if p.Payload.Kind == "chat.message" {
				out = append(out, p)
			}
		}
		return out
	}
	const secret = "my private question about my health"
	send := func(body string) {
		t.Helper()
		client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/messages",
			map[string]any{"body": body, "client_msg_id": uniq("push-")}, nil)
		e.app.Services.Chat.FlushUnread()
		time.Sleep(250 * time.Millisecond) // push workers run in the background
	}

	// The advisor is connected: the live chat is enough, no push.
	send(secret)
	if got := chatPushes(); len(got) != 0 {
		t.Fatalf("pushed to a connected advisor: %+v", got)
	}

	// The advisor goes offline (the paused session is put back to active so
	// the client can keep writing): no connection, but the push switch is off.
	_ = pws.conn.Close()
	eventually(t, 5*time.Second, "advisor presence gone", func() bool {
		var n int64
		e.app.DB.Model(&domain.Session{}).Where("id = ? AND status = ?", s.ID, domain.SessionDisconnected).Count(&n)
		return n == 1
	})
	if err := e.app.DB.Model(&domain.Session{}).Where("id = ?", s.ID).Update("status", domain.SessionActive).Error; err != nil {
		t.Fatal(err)
	}
	e.setPrefs(advisor.user.ID, map[string]any{"notify_push": false})
	send(secret)
	if got := chatPushes(); len(got) != 0 {
		t.Fatalf("pushed although the switch is off: %+v", got)
	}

	// Switch on: exactly one push, localized, without the message text.
	e.setPrefs(advisor.user.ID, map[string]any{"notify_push": true})
	send(secret)
	got := e.waitPushes(1)
	pushes := chatPushes()
	if len(pushes) != 1 {
		t.Fatalf("pushes: %+v (all %+v)", pushes, got)
	}
	p := pushes[0]
	if p.Endpoint != endpoint || p.Urgency != "high" || p.Payload.Tag != "chat:"+s.ConversationID.String() ||
		!strings.HasPrefix(p.Payload.Title, "Nouveau message de ") || p.Payload.Body != "Ouvrez le salon de chat pour le lire." ||
		p.Payload.URL != e.app.Config.App.FrontendURL+"/fr/chatroom" {
		t.Fatalf("push: %+v", p)
	}
	if raw, _ := json.Marshal(p); strings.Contains(string(raw), "health") || strings.Contains(string(raw), "private") {
		t.Fatalf("push carries the message text: %s", raw)
	}

	// More messages within 60 s add nothing.
	send("second")
	send("third")
	if got := chatPushes(); len(got) != 1 {
		t.Fatalf("throttle: %d pushes", len(got))
	}
	// The unread badge still counted every message while offline.
	if v := advisor.unread(); v.Count < 4 {
		t.Fatalf("advisor total: %+v", v)
	}
}

func TestChatMessagePushToClientAndSystemMessages(t *testing.T) {
	e := newEnv(t, withPush(t))
	client, advisor, cws, pws, s := startedSession(t, e, 5, 0)
	_ = pws
	endpoint := "https://updates.push.services.mozilla.com/wpush/v2/" + uniq("cl")
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/me/push-subscriptions", browserSubscription(t, endpoint), nil)
	e.setPrefs(client.user.ID, map[string]any{"notify_push": true})
	if err := e.app.DB.Model(&domain.User{}).Where("id = ?", client.user.ID).Update("locale", "es").Error; err != nil {
		t.Fatal(err)
	}
	_ = cws.conn.Close()
	eventually(t, 5*time.Second, "client presence gone", func() bool {
		var n int64
		e.app.DB.Model(&domain.Session{}).Where("id = ? AND status = ?", s.ID, domain.SessionDisconnected).Count(&n)
		return n == 1
	})
	e.app.Services.Chat.FlushUnread()
	time.Sleep(250 * time.Millisecond)
	for _, p := range e.push.Sent() {
		if p.Payload.Kind == "chat.message" {
			t.Fatalf("a system notice was pushed: %+v", p)
		}
	}
	if err := e.app.DB.Model(&domain.Session{}).Where("id = ?", s.ID).Update("status", domain.SessionActive).Error; err != nil {
		t.Fatal(err)
	}
	advisor.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/messages",
		map[string]any{"body": "Hola, ya estoy aqui.", "client_msg_id": uniq("push-")}, nil)
	var got pushSend
	eventually(t, 5*time.Second, "chat push", func() bool {
		for _, p := range e.push.Sent() {
			if p.Payload.Kind == "chat.message" {
				got = p
				return true
			}
		}
		return false
	})
	if !strings.HasPrefix(got.Payload.Title, "Nuevo mensaje de ") || got.Payload.Body != "Abre la sala de chat para leerlo." ||
		got.Payload.URL != e.app.Config.App.FrontendURL+"/es/chatroom?reader="+advisor.slug() || strings.Contains(got.Payload.Body, "Hola") {
		t.Fatalf("client push: %+v", got)
	}
}
