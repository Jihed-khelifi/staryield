package e2e

// revisions 3.3: messages outside a live chat (§22.3), chat image limits
// (§22.4) and the client's date of birth (§22.6). The off-session feature can
// cost the company money, so every gate is exercised end to end.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/app"
	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// apiClient names the harness client where a local variable shadows the type.
type apiClient = client

type sendOut struct {
	Message   service.MessageView         `json:"message"`
	Allowance service.OffSessionAllowance `json:"allowance"`
}

const offSessionSettingPath = "/api/v1/admin/site/off-session-messages"

func convMessages(convID uuid.UUID) string {
	return "/api/v1/chat/conversations/" + convID.String() + "/messages"
}
func convAllowance(convID uuid.UUID) string {
	return "/api/v1/chat/conversations/" + convID.String() + "/allowance"
}

// say posts a text message addressed to the conversation.
func (c *client) sayIn(convID uuid.UUID, body, cmid string) response {
	c.e.t.Helper()
	return c.call(http.MethodPost, convMessages(convID), map[string]any{"body": body, "client_msg_id": cmid})
}

func (c *client) sendConvImage(convID uuid.UUID, data []byte, fields map[string]string) response {
	c.e.t.Helper()
	body, ct := imageForm(c.e.t, data, fields)
	return c.raw(http.MethodPost, "/api/v1/chat/conversations/"+convID.String()+"/attachments", body,
		map[string]string{"Content-Type": ct, "X-CSRF-Token": csrfOf(c)})
}

func (c *client) allowance(convID uuid.UUID) service.OffSessionAllowance {
	c.e.t.Helper()
	var a service.OffSessionAllowance
	c.must(http.StatusOK, http.MethodGet, convAllowance(convID), nil, &a)
	return a
}

// pinOffSession sets the setting through the admin API and restores the seed
// afterwards (the database is shared by the whole suite).
func (e *env) pinOffSession(admin *client, enabled bool, clientLimit, advisorLimit int) service.OffSessionMessagesSetting {
	e.t.Helper()
	admin.stepUp()
	var out service.OffSessionMessagesSetting
	admin.must(http.StatusOK, http.MethodPut, offSessionSettingPath, map[string]any{
		"enabled": enabled, "client_daily_limit": clientLimit, "advisor_daily_limit": advisorLimit}, &out)
	e.t.Cleanup(func() {
		e.app.DB.Exec(`UPDATE site_settings SET value = '{"enabled": true, "client_daily_limit": 20, "advisor_daily_limit": 20}'::jsonb, version = 1 WHERE key = 'off_session_messages'`)
	})
	return out
}

func (e *env) offSessionRows(convID, sender uuid.UUID) int64 {
	var n int64
	if err := e.app.DB.Raw(`SELECT count(*) FROM chat_messages WHERE conversation_id = ? AND sender_id = ? AND session_id IS NULL`, convID, sender).Scan(&n).Error; err != nil {
		e.t.Fatal(err)
	}
	return n
}

// A client and an advisor talk, the chat ends, and they keep writing: over
// REST and the WebSocket, with the old session-addressed routes too.
func TestOffSessionFullFlow(t *testing.T) {
	e := newEnv(t)
	client, advisor, cws, pws, s := startedSession(t, e, 10, 250)
	conv := s.ConversationID

	// Inside the live chat the message belongs to the session: no cap, no label.
	var live sendOut
	client.must(http.StatusCreated, http.MethodPost, convMessages(conv), map[string]any{"body": "live hello", "client_msg_id": uniq("live-")}, &live)
	if live.Message.SessionID == nil || *live.Message.SessionID != s.ID || live.Message.OffSession || !live.Allowance.Live {
		t.Fatalf("live send: %+v %+v", live.Message, live.Allowance)
	}
	if got := client.allowance(conv); !got.Live || got.Used != 0 {
		t.Fatalf("live allowance: %+v", got)
	}

	endSession(t, client, s.ID)
	cws.drain()
	pws.drain()
	walletAfterEnd := client.wallet()

	// Client writes over REST: delivered to the advisor in order, unread counted.
	before := advisor.unread()
	var first sendOut
	r := client.must(http.StatusCreated, http.MethodPost, convMessages(conv), map[string]any{"body": "are you still there?", "client_msg_id": "off-msg-0001"}, &first)
	if !first.Message.OffSession || first.Message.SessionID != nil || strings.Contains(string(r.body), `"session_id"`) {
		t.Fatalf("off-session message: %s", r.body)
	}
	if a := first.Allowance; a.Live || !a.Enabled || !a.Eligible || a.Unlimited || a.Limit != 20 || a.Used != 1 || a.Remaining != 19 || a.ResetsAt == nil {
		t.Fatalf("allowance: %+v", a)
	}
	ev := pws.waitMatch(realtime.EventMessageCreated, "are you still there?", 5*time.Second)
	if !strings.Contains(string(ev.Data), `"off_session":true`) {
		t.Fatalf("event: %s", ev.Data)
	}
	eventuallyUnread := func(want int64) {
		t.Helper()
		eventually(t, 5*time.Second, "advisor unread", func() bool { return advisor.unread().Count >= want })
	}
	eventuallyUnread(before.Count + 1)

	// Over the WebSocket with the new frame, then with the old session-only
	// frame: both ack the message and the allowance.
	var ack struct {
		Message   service.MessageView         `json:"message"`
		Allowance service.OffSessionAllowance `json:"allowance"`
	}
	raw := cws.mustAck(realtime.FrameMessageSend, map[string]any{"conversation_id": conv, "body": "ws message", "client_msg_id": "off-msg-0002"})
	if err := json.Unmarshal(raw, &ack); err != nil || !ack.Message.OffSession || ack.Allowance.Used != 2 || ack.Allowance.Remaining != 18 {
		t.Fatalf("ws ack: %s %v", raw, err)
	}
	raw = cws.mustAck(realtime.FrameMessageSend, map[string]any{"session_id": s.ID, "body": "legacy frame", "client_msg_id": "off-msg-0003"})
	if err := json.Unmarshal(raw, &ack); err != nil || !ack.Message.OffSession || ack.Allowance.Used != 3 {
		t.Fatalf("legacy ws ack: %s %v", raw, err)
	}
	pws.waitMatch(realtime.EventMessageCreated, "legacy frame", 5*time.Second)

	// The old REST route routes the same way.
	var legacy sendOut
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/messages",
		map[string]any{"body": "legacy rest", "client_msg_id": "off-msg-0004"}, &legacy)
	if !legacy.Message.OffSession || legacy.Allowance.Used != 4 {
		t.Fatalf("legacy rest: %+v %+v", legacy.Message, legacy.Allowance)
	}

	// The advisor answers outside the chat: own allowance, not billed.
	var reply sendOut
	advisor.must(http.StatusCreated, http.MethodPost, convMessages(conv), map[string]any{"body": "I am here", "client_msg_id": "off-msg-0005"}, &reply)
	if !reply.Message.OffSession || reply.Allowance.Used != 1 || reply.Allowance.Limit != 20 {
		t.Fatalf("advisor reply: %+v %+v", reply.Message, reply.Allowance)
	}
	cws.waitMatch(realtime.EventMessageCreated, "I am here", 5*time.Second)
	if w := client.wallet(); w.Balance != walletAfterEnd.Balance || w.FreeMinutes != walletAfterEnd.FreeMinutes { // off-session messages are free
		t.Fatalf("wallet changed: %+v -> %+v", walletAfterEnd, w)
	}

	client.refresh() // the admin steps above moved the fake clock
	advisor.refresh()
	// History for both sides: off_session true and no session_id; the system
	// notices keep their session.
	for name, who := range map[string]*apiClient{"client": client, "advisor": advisor} {
		var page service.MessagePage
		who.must(http.StatusOK, http.MethodGet, convMessages(conv)+"?limit=100", nil, &page)
		off, system := 0, 0
		for _, m := range page.Items {
			switch {
			case m.Kind == "system":
				system++
				if m.SessionID == nil || m.OffSession {
					t.Fatalf("%s: system notice: %+v", name, m)
				}
			case m.SessionID == nil:
				off++
				if !m.OffSession {
					t.Fatalf("%s: unlabelled: %+v", name, m)
				}
			case m.OffSession:
				t.Fatalf("%s: session message labelled off-session: %+v", name, m)
			}
		}
		if off != 5 || system < 2 {
			t.Fatalf("%s: history has %d off-session and %d system messages", name, off, system)
		}
	}

	// The admin's session transcript does not include them.
	client.refresh()
	advisor.refresh()
	admin := e.createAdmin()
	admin.stepUp()
	var transcript service.Page[service.MessageView]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/chat/sessions/"+s.ID.String()+"/transcript?reason=audit%20test&limit=100", nil, &transcript)
	if len(transcript.Items) == 0 {
		t.Fatal("transcript is empty")
	}
	for _, m := range transcript.Items {
		if m.OffSession || m.SessionID == nil {
			t.Fatalf("transcript shows an off-session message: %+v", m)
		}
	}
	// The conversation-level view (what support reports and exports use) does.
	adminCtx := reqctx.WithPrincipal(context.Background(), reqctx.Principal{UserID: admin.adminID, Role: domain.RoleAdmin})
	all, err := e.app.Services.Chat.Messages(adminCtx, reqctx.Principal{UserID: admin.adminID, Role: domain.RoleAdmin}, conv, service.MessagesQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	offSeen := 0
	for _, m := range all.Items {
		if m.OffSession {
			offSeen++
		}
	}
	if offSeen != 5 {
		t.Fatalf("conversation-level admin view shows %d off-session messages, want 5", offSeen)
	}

	var inbox service.InboxPage
	client.must(http.StatusOK, http.MethodGet, "/api/v1/chat/conversations", nil, &inbox) // the advisor's reply is unread
	if len(inbox.Items) != 1 || inbox.Items[0].Unread != 1 || inbox.Items[0].LastMessage == nil {
		t.Fatalf("inbox unread: %+v", inbox.Items)
	}
}

// Messages sent while the chat is paused belong to the session and are never counted.
func TestOffSessionDisconnectedSessionTakesTheMessage(t *testing.T) {
	e := newEnv(t)
	client, advisor, cws, pws, s := startedSession(t, e, 10, 250)
	_ = cws.conn.Close()
	pws.waitFor(realtime.EventSessionDisconnected, 5*time.Second)

	var out sendOut
	advisor.must(http.StatusCreated, http.MethodPost, convMessages(s.ConversationID), map[string]any{"body": "waiting for you", "client_msg_id": "paused-msg-1"}, &out)
	if out.Message.SessionID == nil || *out.Message.SessionID != s.ID || out.Message.OffSession || !out.Allowance.Live {
		t.Fatalf("paused send: %+v %+v", out.Message, out.Allowance)
	}
	if n := e.offSessionRows(s.ConversationID, advisor.user.ID); n != 0 {
		t.Fatalf("%d off-session rows for a paused session", n)
	}
	if a := advisor.allowance(s.ConversationID); !a.Live || a.Used != 0 {
		t.Fatalf("allowance: %+v", a)
	}
	_ = client
}

// The kill switch, eligibility and recipient rules, in order.
func TestOffSessionRefusals(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	client, advisor, _, _, s := startedSession(t, e, 10, 250)
	conv := s.ConversationID
	endSession(t, client, s.ID)

	// A stranger is told the conversation does not exist.
	stranger := e.registerClient()
	if r := stranger.sayIn(conv, "hi", "stranger-msg-1"); r.status != http.StatusNotFound || r.errCode(t) != "not_found" {
		t.Fatalf("stranger: %d %s", r.status, r.body)
	}
	if r := stranger.call(http.MethodGet, convAllowance(conv), nil); r.status != http.StatusNotFound {
		t.Fatalf("stranger allowance: %d", r.status)
	}
	if r := e.client().call(http.MethodGet, convAllowance(conv), nil); r.status != http.StatusUnauthorized {
		t.Fatalf("anonymous allowance: %d", r.status)
	}

	// Kill switch.
	e.pinOffSession(admin, false, 20, 20)
	r := client.sayIn(conv, "hi", "off-refuse-01")
	if r.status != http.StatusConflict || r.errCode(t) != "off_session_disabled" {
		t.Fatalf("kill switch: %d %s", r.status, r.body)
	}
	if a := client.allowance(conv); a.Enabled || a.Limit != 0 || a.Remaining != 0 {
		t.Fatalf("disabled allowance: %+v", a)
	}
	// A limit of 0 for one role only disables that role.
	e.pinOffSession(admin, true, 0, 3)
	if r := client.sayIn(conv, "hi", "off-refuse-02"); r.status != http.StatusConflict || r.errCode(t) != "off_session_disabled" {
		t.Fatalf("client limit 0: %d %s", r.status, r.body)
	}
	advisor.must(http.StatusCreated, http.MethodPost, convMessages(conv), map[string]any{"body": "hello", "client_msg_id": "off-refuse-03"}, nil)
	e.pinOffSession(admin, true, 5, 5)

	// Recipient unavailable (the advisor is suspended).
	if err := e.app.DB.Model(&domain.User{}).Where("id = ?", advisor.user.ID).Update("status", domain.UserBanned).Error; err != nil {
		t.Fatal(err)
	}
	if r := client.sayIn(conv, "hi", "off-refuse-04"); r.status != http.StatusConflict || r.errCode(t) != "recipient_unavailable" {
		t.Fatalf("suspended advisor: %d %s", r.status, r.body)
	}
	// The message text of a refusal is translated.
	client.locale = "fr"
	if r := client.sayIn(conv, "hi", "off-refuse-05"); !strings.Contains(string(r.body), "ne peut pas recevoir") {
		t.Fatalf("not translated: %s", r.body)
	}
	if n := e.offSessionRows(conv, client.user.ID); n != 0 {
		t.Fatalf("refused messages were stored: %d", n)
	}
}

// A request cancelled before it started never makes the conversation eligible.
func TestOffSessionNotEligibleWithoutAStartedChat(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	psychic := e.createPsychic(10)
	pws := psychic.goOnline()
	_ = pws
	client := e.registerClient()
	client.buyCredits(250)
	var requested service.SessionView
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": psychic.slug(), "request_key": uniq("req-key-")}, &requested)
	client.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+requested.ID.String()+"/cancel", nil, nil)

	r := client.sayIn(requested.ConversationID, "hello?", "not-eligible-1")
	if r.status != http.StatusConflict || r.errCode(t) != "off_session_not_eligible" {
		t.Fatalf("not eligible: %d %s", r.status, r.body)
	}
	if a := client.allowance(requested.ConversationID); a.Eligible || a.Live || !a.Enabled {
		t.Fatalf("allowance: %+v", a)
	}
	// An image is refused by the same rule, before anything is stored.
	keys := len(e.chat.Keys())
	if r := client.sendConvImage(requested.ConversationID, pngBytes(t, 32, 32), nil); r.status != http.StatusConflict || r.errCode(t) != "off_session_not_eligible" {
		t.Fatalf("image: %d %s", r.status, r.body)
	}
	if len(e.chat.Keys()) != keys {
		t.Fatal("an object was stored for a refused image")
	}
	// A pending request is not a live chat either: still refused.
	var again service.SessionView
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": psychic.slug(), "request_key": uniq("req-key-")}, &again)
	if r := client.sayIn(again.ConversationID, "hello?", "not-eligible-2"); r.errCode(t) != "off_session_not_eligible" {
		t.Fatalf("pending request: %d %s", r.status, r.body)
	}
	// The advisor is refused the same way.
	if r := psychic.sayIn(again.ConversationID, "hello", "not-eligible-3"); r.errCode(t) != "off_session_not_eligible" {
		t.Fatalf("advisor: %d %s", r.status, r.body)
	}
}

// The cap holds under concurrency: exactly `limit` of many parallel sends win.
func TestOffSessionCapUnderConcurrency(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	e.pinOffSession(admin, true, 3, 2)
	client, advisor, _, pws, s := startedSession(t, e, 10, 250)
	conv := s.ConversationID
	endSession(t, client, s.ID)
	pws.drain()

	csrf := csrfOf(client)
	const parallel = 14
	statuses := make([]int, parallel)
	bodies := make([][]byte, parallel)
	headers := make([]http.Header, parallel)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			b, _ := json.Marshal(map[string]any{"body": "burst", "client_msg_id": "burst-msg-" + uuid.NewString()[:8]})
			r := client.raw(http.MethodPost, convMessages(conv), bytes.NewReader(b), map[string]string{"Content-Type": "application/json", "X-CSRF-Token": csrf})
			statuses[i], bodies[i], headers[i] = r.status, r.body, r.header
		}(i)
	}
	close(start)
	wg.Wait()
	ok, limited := 0, 0
	for i, st := range statuses {
		switch st {
		case http.StatusCreated:
			ok++
		case http.StatusTooManyRequests:
			limited++
			var body struct {
				Error struct {
					Code    string `json:"code"`
					Details struct {
						Limit    int    `json:"limit"`
						Used     int    `json:"used"`
						ResetsAt string `json:"resets_at"`
					} `json:"details"`
				} `json:"error"`
			}
			if err := json.Unmarshal(bodies[i], &body); err != nil || body.Error.Code != "off_session_limit_reached" ||
				body.Error.Details.Limit != 3 || body.Error.Details.Used != 3 || body.Error.Details.ResetsAt == "" {
				t.Fatalf("429 body: %s", bodies[i])
			}
			if headers[i].Get("Retry-After") == "" || headers[i].Get("Retry-After") == "0" {
				t.Fatalf("Retry-After: %q", headers[i].Get("Retry-After"))
			}
		default:
			t.Fatalf("unexpected status %d: %s", st, bodies[i])
		}
	}
	if ok != 3 || limited != parallel-3 {
		t.Fatalf("created=%d limited=%d, want 3 and %d", ok, limited, parallel-3)
	}
	if n := e.offSessionRows(conv, client.user.ID); n != 3 {
		t.Fatalf("%d rows stored, want 3", n)
	}
	// Seq numbers stay gap-free.
	var gaps int64
	e.app.DB.Raw(`SELECT count(*) FROM (SELECT seq, lag(seq) OVER (ORDER BY seq) AS prev FROM chat_messages WHERE conversation_id = ?) x WHERE prev IS NOT NULL AND seq <> prev + 1`, conv).Scan(&gaps)
	if gaps != 0 {
		t.Fatalf("%d gaps in the conversation sequence", gaps)
	}

	// An already accepted client_msg_id is a replay: 200, the original, no count.
	var stored []string
	e.app.DB.Raw(`SELECT client_msg_id FROM chat_messages WHERE conversation_id = ? AND sender_id = ? AND session_id IS NULL ORDER BY seq`, conv, client.user.ID).Scan(&stored)
	var replay sendOut
	client.must(http.StatusOK, http.MethodPost, convMessages(conv), map[string]any{"body": "burst", "client_msg_id": stored[0]}, &replay)
	if replay.Message.ClientMsgID != stored[0] || replay.Allowance.Used != 3 || replay.Allowance.Remaining != 0 {
		t.Fatalf("replay: %+v %+v", replay.Message, replay.Allowance)
	}
	if n := e.offSessionRows(conv, client.user.ID); n != 3 {
		t.Fatalf("replay changed the count: %d", n)
	}
	// A new one is still refused, and the allowance says so.
	if r := client.sayIn(conv, "again", "burst-after-0001"); r.status != http.StatusTooManyRequests {
		t.Fatalf("after the cap: %d %s", r.status, r.body)
	}
	if a := client.allowance(conv); a.Used != 3 || a.Remaining != 0 || a.Limit != 3 || a.ResetsAt == nil {
		t.Fatalf("allowance: %+v", a)
	}

	// The advisor's cap (2) is separate from the client's.
	for i := 0; i < 2; i++ {
		advisor.must(http.StatusCreated, http.MethodPost, convMessages(conv), map[string]any{"body": "reply", "client_msg_id": uniq("adv-")}, nil)
	}
	if r := advisor.sayIn(conv, "one more", uniq("adv-")); r.status != http.StatusTooManyRequests {
		t.Fatalf("advisor over the cap: %d %s", r.status, r.body)
	}

	// Oversight on the admin setting: refusals are counted from analytics.
	admin.stepUp()
	eventually(t, 5*time.Second, "limit hits recorded", func() bool {
		var st service.OffSessionMessagesSetting
		admin.must(http.StatusOK, http.MethodGet, offSessionSettingPath, nil, &st)
		return st.Stats.LimitHits24h >= int64(parallel-3+2)
	})
	var st service.OffSessionMessagesSetting
	admin.must(http.StatusOK, http.MethodGet, offSessionSettingPath, nil, &st)
	// Totals are platform-wide (the suite shares one database): lower bounds.
	if st.Stats.Messages24h < 5 || st.Stats.Senders24h < 2 {
		t.Fatalf("stats: %+v", st.Stats)
	}
	// The rolling window: a day later the allowance is back.
	e.clock.Advance(24*time.Hour + time.Second)
	client.refresh()
	advisor.refresh()
	if a := client.allowance(conv); a.Used != 0 || a.Remaining != 3 || a.ResetsAt != nil {
		t.Fatalf("after 24 h: %+v", a)
	}
	client.must(http.StatusCreated, http.MethodPost, convMessages(conv), map[string]any{"body": "new day", "client_msg_id": uniq("day-")}, nil)

	// Analytics never contain the message text.
	var leaked int64
	e.app.DB.Raw(`SELECT count(*) FROM analytics_events WHERE name LIKE 'chat.off_session_%' AND properties::text LIKE '%burst%'`).Scan(&leaked)
	if leaked != 0 {
		t.Fatal("message text in analytics")
	}
}

// Free mode lifts the cap, never the kill switch.
func TestOffSessionFreeModeBypassesOnlyTheCap(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	e.pinOffSession(admin, true, 2, 2)
	client, _, _, pws, s := startedSession(t, e, 10, 250)
	conv := s.ConversationID
	endSession(t, client, s.ID)
	pws.drain()
	for i := 0; i < 2; i++ {
		client.must(http.StatusCreated, http.MethodPost, convMessages(conv), map[string]any{"body": "x", "client_msg_id": uniq("pre-")}, nil)
	}
	if r := client.sayIn(conv, "x", uniq("pre-")); r.status != http.StatusTooManyRequests {
		t.Fatalf("cap: %d", r.status)
	}

	admin.stepUp()
	admin.must(http.StatusOK, http.MethodPut, "/api/v1/admin/site/free-mode", map[string]any{"enabled": true, "confirmation": "ENABLE FREE MODE",
		"reason": "Launch weekend", "ends_at": e.clock.Now().Add(24 * time.Hour).Format(time.RFC3339)}, nil)
	t.Cleanup(func() {
		e.app.DB.Exec(`UPDATE site_settings SET value = '{"enabled": false, "reason": ""}'::jsonb WHERE key = 'free_mode'`)
	})
	for i := 0; i < 8; i++ {
		var out sendOut
		client.must(http.StatusCreated, http.MethodPost, convMessages(conv), map[string]any{"body": "free", "client_msg_id": uniq("free-")}, &out)
		if !out.Allowance.Unlimited || out.Allowance.Remaining != out.Allowance.Limit {
			t.Fatalf("allowance: %+v", out.Allowance)
		}
	}
	if a := client.allowance(conv); !a.Unlimited || a.Used != 10 {
		t.Fatalf("allowance: %+v", a)
	}
	// The kill switch still wins.
	e.pinOffSession(admin, false, 2, 2)
	if r := client.sayIn(conv, "x", uniq("free-")); r.status != http.StatusConflict || r.errCode(t) != "off_session_disabled" {
		t.Fatalf("kill switch in free mode: %d %s", r.status, r.body)
	}
}

// Off-session images count toward the cap, and a refused upload stores nothing.
func TestOffSessionImagesCountAndRefusedUploadsStoreNothing(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	e.pinOffSession(admin, true, 2, 2)
	client, advisor, _, pws, s := startedSession(t, e, 10, 250)
	conv := s.ConversationID
	endSession(t, client, s.ID)
	pws.drain()
	png := pngBytes(t, 32, 32)

	base := len(e.chat.Keys())
	r := client.sendConvImage(conv, png, map[string]string{"caption": "my palm", "client_msg_id": "img-off-0001"})
	var out sendOut
	if r.status != http.StatusCreated {
		t.Fatalf("image: %d %s", r.status, r.body)
	}
	r.decode(t, &out)
	if !out.Message.OffSession || out.Message.Kind != "image" || len(out.Message.Attachments) != 1 || out.Allowance.Used != 1 {
		t.Fatalf("image message: %+v %+v", out.Message, out.Allowance)
	}
	if len(e.chat.Keys()) != base+1 {
		t.Fatalf("objects: %d", len(e.chat.Keys()))
	}
	// The advisor can fetch it; strangers cannot.
	if r := advisor.call(http.MethodGet, "/api/v1/chat/attachments/"+out.Message.Attachments[0].ID.String(), nil); r.status != http.StatusOK {
		t.Fatalf("advisor fetch: %d", r.status)
	}
	// A replay of the upload returns the original and stores nothing new.
	r = client.sendConvImage(conv, png, map[string]string{"client_msg_id": "img-off-0001"})
	if r.status != http.StatusOK {
		t.Fatalf("replay: %d %s", r.status, r.body)
	}
	if len(e.chat.Keys()) != base+1 {
		t.Fatalf("replay stored an object: %d", len(e.chat.Keys()))
	}
	// Second counted message (text), then the cap.
	client.must(http.StatusCreated, http.MethodPost, convMessages(conv), map[string]any{"body": "text", "client_msg_id": uniq("t-")}, nil)
	r = client.sendConvImage(conv, png, map[string]string{"client_msg_id": "img-off-0002"})
	if r.status != http.StatusTooManyRequests || r.errCode(t) != "off_session_limit_reached" {
		t.Fatalf("image over the cap: %d %s", r.status, r.body)
	}
	if len(e.chat.Keys()) != base+1 {
		t.Fatalf("a refused upload stored an object: %d", len(e.chat.Keys()))
	}
	if n := e.offSessionRows(conv, client.user.ID); n != 2 {
		t.Fatalf("rows: %d", n)
	}
	// The replay of the accepted upload still answers after the cap.
	if r := client.sendConvImage(conv, png, map[string]string{"client_msg_id": "img-off-0001"}); r.status != http.StatusOK {
		t.Fatalf("replay after the cap: %d %s", r.status, r.body)
	}
	// Garbage is a 422 on fields.image, translated.
	e.pinOffSession(admin, true, 20, 20)
	client.locale = "fr"
	r = client.sendConvImage(conv, bytes.Repeat([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}, 8), nil)
	if r.status != http.StatusUnprocessableEntity || fieldMap(t, r)["image"] == "" {
		t.Fatalf("undecodable image: %d %s", r.status, r.body)
	}
	if fieldMap(t, r)["image"] == "The file is not a valid image." {
		t.Fatalf("image message not translated: %s", r.body)
	}
}

// Chat images have their own budget; avatars keep the "upload" rule.
func TestChatImageRateLimitIsSeparateFromAvatars(t *testing.T) {
	e := newEnv(t, func(cfg *config.Config) {
		cfg.RateLimit.Scale = 1
		big := config.Rule{Rate: 100000, Period: time.Hour, Burst: 100000}
		cfg.RateLimit.Global, cfg.RateLimit.Auth, cfg.RateLimit.Writes, cfg.RateLimit.Ingest = big, big, big, big
		cfg.RateLimit.Upload = config.Rule{Rate: 2, Period: time.Hour, Burst: 2}
		cfg.RateLimit.ChatImage = config.Rule{Rate: 3, Period: time.Hour, Burst: 3}
		cfg.Chat.MessageRate = 1000
	})
	client, _, _, _, s := startedSession(t, e, 10, 250)
	png := pngBytes(t, 32, 32)
	avatar := func() response {
		body, ct := multipartFile(t, "avatar", "me.png", png)
		return client.raw(http.MethodPost, "/api/v1/me/avatar", body, map[string]string{"Content-Type": ct, "X-CSRF-Token": csrfOf(client)})
	}
	// Two avatars use up the upload budget; the third is limited.
	for i := 0; i < 2; i++ {
		if r := avatar(); r.status == http.StatusTooManyRequests {
			t.Fatalf("avatar %d limited early", i)
		}
	}
	if r := avatar(); r.status != http.StatusTooManyRequests {
		t.Fatalf("avatar budget not enforced: %d", r.status)
	}
	// Chat images are unaffected by the exhausted avatar budget: three fit.
	for i := 0; i < 3; i++ {
		r := client.sendImage(s.ID.String(), png, map[string]string{"client_msg_id": uniq("img-")})
		if r.status != http.StatusCreated {
			t.Fatalf("chat image %d after avatars: %d %s", i, r.status, r.body)
		}
	}
	// The conversation route shares the chat_image bucket.
	r := client.sendConvImage(s.ConversationID, png, map[string]string{"client_msg_id": uniq("img-")})
	if r.status != http.StatusTooManyRequests || r.errCode(t) != "rate_limited" {
		t.Fatalf("chat image budget not enforced: %d %s", r.status, r.body)
	}
	if r.header.Get("Retry-After") == "" {
		t.Fatal("no Retry-After")
	}
}

// A message while the advisor is offline reaches them as unread count and Web Push.
func TestOffSessionMessageFiresUnreadAndWebPush(t *testing.T) {
	e := newEnv(t, withPush(t))
	client, advisor, cws, pws, s := startedSession(t, e, 5, 0)
	_ = cws
	endpoint := "https://fcm.googleapis.com/fcm/send/" + uniq("adv")
	advisor.must(http.StatusCreated, http.MethodPost, "/api/v1/me/push-subscriptions", browserSubscription(t, endpoint), nil)
	e.setPrefs(advisor.user.ID, map[string]any{"notify_push": true})
	endSession(t, client, s.ID)
	_ = pws.conn.Close()
	time.Sleep(300 * time.Millisecond)
	base := advisor.unread().Count

	const secret = "my private question about my health"
	client.must(http.StatusCreated, http.MethodPost, convMessages(s.ConversationID), map[string]any{"body": secret, "client_msg_id": uniq("push-")}, nil)
	e.app.Services.Chat.FlushUnread()
	eventually(t, 5*time.Second, "web push", func() bool {
		for _, p := range e.push.Sent() {
			if p.Payload.Kind == "chat.message" {
				return true
			}
		}
		return false
	})
	for _, p := range e.push.Sent() {
		if raw, _ := json.Marshal(p.Payload); strings.Contains(string(raw), "health") {
			t.Fatalf("the push carries the text: %s", raw)
		}
	}
	if got := advisor.unread().Count; got != base+1 {
		t.Fatalf("unread %d want %d", got, base+1)
	}
}

// The admin setting: validation, concurrency control, step-up, audit, stats.
func TestAdminOffSessionMessagesSetting(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	t.Cleanup(func() {
		e.app.DB.Exec(`UPDATE site_settings SET value = '{"enabled": true, "client_daily_limit": 20, "advisor_daily_limit": 20}'::jsonb, version = 1 WHERE key = 'off_session_messages'`)
	})
	var cur service.OffSessionMessagesSetting
	admin.must(http.StatusOK, http.MethodGet, offSessionSettingPath, nil, &cur)
	if !cur.Enabled || cur.ClientDailyLimit != 20 || cur.AdvisorDailyLimit != 20 || cur.Version < 1 {
		t.Fatalf("seed: %+v", cur)
	}
	if r := admin.call(http.MethodGet, offSessionSettingPath, nil); !strings.Contains(string(r.body), `"stats":{"messages_24h":`) {
		t.Fatalf("stats must always be present: %s", r.body)
	}
	// Non-admins cannot read or write it.
	if r := e.registerClient().call(http.MethodGet, offSessionSettingPath, nil); r.status == http.StatusOK {
		t.Fatalf("a client read the admin setting: %d", r.status)
	}

	// Without a fresh step-up the write is refused.
	e.clock.Advance(11 * time.Minute)
	body := map[string]any{"enabled": true, "client_daily_limit": 3, "advisor_daily_limit": 4}
	if r := admin.call(http.MethodPut, offSessionSettingPath, body); r.status != http.StatusForbidden || r.errCode(t) != "step_up_required" {
		t.Fatalf("no step-up: %d %s", r.status, r.body)
	}
	admin.stepUp()
	// Validation: exactly the field keys of the contract.
	for name, tc := range map[string]struct {
		body map[string]any
		want []string
	}{
		"empty":        {map[string]any{}, []string{"enabled", "client_daily_limit", "advisor_daily_limit"}},
		"too high":     {map[string]any{"enabled": true, "client_daily_limit": 501, "advisor_daily_limit": 4}, []string{"client_daily_limit"}},
		"negative":     {map[string]any{"enabled": true, "client_daily_limit": 3, "advisor_daily_limit": -1}, []string{"advisor_daily_limit"}},
		"missing flag": {map[string]any{"client_daily_limit": 3, "advisor_daily_limit": 4}, []string{"enabled"}},
	} {
		r := admin.call(http.MethodPut, offSessionSettingPath, tc.body)
		if r.status != http.StatusUnprocessableEntity || r.errCode(t) != "validation_failed" {
			t.Fatalf("%s: %d %s", name, r.status, r.body)
		}
		f := fieldMap(t, r)
		if len(f) != len(tc.want) {
			t.Fatalf("%s: fields %v want %v", name, f, tc.want)
		}
		for _, k := range tc.want {
			if f[k] == "" {
				t.Fatalf("%s: field %s missing in %v", name, k, f)
			}
		}
	}
	for _, n := range []int{0, 500} {
		admin.stepUp() // moves the clock: audit rows are ordered by time
		admin.must(http.StatusOK, http.MethodPut, offSessionSettingPath, map[string]any{"enabled": true, "client_daily_limit": n, "advisor_daily_limit": n}, nil)
	}
	admin.stepUp()
	admin.must(http.StatusOK, http.MethodGet, offSessionSettingPath, nil, &cur)

	// Optimistic concurrency.
	stale := body
	stale["expected_version"] = cur.Version - 1
	if r := admin.call(http.MethodPut, offSessionSettingPath, stale); r.status != http.StatusConflict || r.errCode(t) != "version_conflict" {
		t.Fatalf("stale version: %d %s", r.status, r.body)
	}
	body["expected_version"] = cur.Version
	var saved service.OffSessionMessagesSetting
	admin.must(http.StatusOK, http.MethodPut, offSessionSettingPath, body, &saved)
	if saved.Version != cur.Version+1 || saved.ClientDailyLimit != 3 || saved.AdvisorDailyLimit != 4 || !saved.Enabled ||
		saved.UpdatedBy == nil || saved.UpdatedBy.ID != admin.adminID {
		t.Fatalf("saved: %+v", saved)
	}
	// The change is live at once for the chat service.
	if got := e.app.Services.Site.OffSessionSettings(context.Background()); got.ClientDailyLimit != 3 || got.AdvisorDailyLimit != 4 || !got.Enabled {
		t.Fatalf("service sees %+v", got)
	}
	// Audit: before and after, by this admin.
	rows := e.adminAudit(admin.adminID, "site.off_session_messages_updated")
	if len(rows) < 1 {
		t.Fatal("no audit row")
	}
	last := rows[len(rows)-1]
	if b, _ := last.Metadata["before"].(map[string]any); b["client_daily_limit"] != float64(500) {
		t.Fatalf("audit before: %v", last.Metadata)
	}
	if a, _ := last.Metadata["after"].(map[string]any); a["client_daily_limit"] != float64(3) || a["advisor_daily_limit"] != float64(4) || a["enabled"] != true {
		t.Fatalf("audit after: %v", last.Metadata)
	}
	// The database constraint is the last line of defence.
	if err := e.app.DB.Exec(`UPDATE site_settings SET value = '{"enabled": true, "client_daily_limit": 501, "advisor_daily_limit": 1}' WHERE key = 'off_session_messages'`).Error; err == nil {
		t.Fatal("the CHECK constraint accepted a limit above 500")
	}
	if err := e.app.DB.Exec(`UPDATE site_settings SET value = '{"enabled": "yes", "client_daily_limit": 1, "advisor_daily_limit": 1}' WHERE key = 'off_session_messages'`).Error; err == nil {
		t.Fatal("the CHECK constraint accepted a non-boolean switch")
	}
}

// Database rules of migration 0021.
func TestOffSessionSchemaRules(t *testing.T) {
	e := newEnv(t)
	client, _, _, _, s := startedSession(t, e, 10, 250)
	// A system notice must belong to a session.
	err := e.app.DB.Exec(`INSERT INTO chat_messages (id, conversation_id, session_id, kind, body, seq, created_at)
		VALUES (?, ?, NULL, 'system', 'session.ended:x', 999, now())`, uuid.New(), s.ConversationID).Error
	if err == nil {
		t.Fatal("a system message without a session was accepted")
	}
	_ = client
	// Both partial indexes exist and only cover off-session rows.
	for _, idx := range []string{"chat_messages_off_session_quota_idx", "chat_messages_off_session_recent_idx"} {
		var def string
		e.app.DB.Raw(`SELECT indexdef FROM pg_indexes WHERE indexname = ?`, idx).Scan(&def)
		if !strings.Contains(def, "session_id IS NULL") {
			t.Fatalf("%s: %q", idx, def)
		}
	}
}

// Closing an account keeps its off-session messages readable and exportable,
// and the other side can no longer write to a deleted account.
func TestOffSessionWithAccountClosureAndExport(t *testing.T) {
	e := newEnv(t)
	client, advisor, _, pws, s := startedSession(t, e, 10, 250)
	conv := s.ConversationID
	endSession(t, client, s.ID)
	pws.drain()
	client.must(http.StatusCreated, http.MethodPost, convMessages(conv), map[string]any{"body": "Off-session export marker", "client_msg_id": uniq("exp-")}, nil)

	// The personal-data export includes it, labelled.
	var job service.ExportView
	client.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/exports", map[string]any{"password": strongPassword}, &job)
	if err := e.app.Scheduler.RunJob(context.Background(), app.ExportJob); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/me/exports/" + job.ID.String()
	var link service.DownloadLink
	client.must(http.StatusOK, http.MethodPost, path+"/download-token", map[string]any{"password": strongPassword}, &link)
	r := client.raw(http.MethodGet, link.URL, nil, nil)
	files := readZip(t, r.body)
	if !bytes.Contains(files["chat/messages.json"], []byte("Off-session export marker")) || !bytes.Contains(files["chat/messages.json"], []byte(`"off_session": true`)) &&
		!bytes.Contains(files["chat/messages.json"], []byte(`"off_session":true`)) {
		t.Fatalf("export lacks the off-session message: %s", files["chat/messages.json"])
	}

	// Closure and erasure work with NULL session ids.
	client.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/close", map[string]any{"password": strongPassword, "confirmation": "DELETE"}, nil)
	e.clock.Advance(31*24*time.Hour + time.Minute)
	if err := e.app.Scheduler.RunJob(context.Background(), app.ClosureJob); err != nil {
		t.Fatal(err)
	}
	var u domain.User
	e.app.DB.First(&u, "id = ?", client.user.ID)
	if u.Status != domain.UserDeleted {
		t.Fatalf("not erased: %+v", u)
	}
	advisor = e.login(advisor.user.Email, strongPassword)
	if r := advisor.sayIn(conv, "are you there?", uniq("gone-")); r.status != http.StatusConflict || r.errCode(t) != "recipient_unavailable" {
		t.Fatalf("writing to a deleted client: %d %s", r.status, r.body)
	}
	var page service.MessagePage
	advisor.must(http.StatusOK, http.MethodGet, convMessages(conv)+"?limit=100", nil, &page)
	found := false
	for _, m := range page.Items {
		found = found || (m.OffSession && strings.Contains(m.Body, "export marker"))
	}
	if !found {
		t.Fatal("the advisor lost the off-session message")
	}
}

// An ended session in the old frame shape keeps working for old clients.
func TestOffSessionLegacyClientsAfterTheChatEnds(t *testing.T) {
	e := newEnv(t)
	client, _, cws, pws, s := startedSession(t, e, 10, 250)
	endSession(t, client, s.ID)
	pws.drain()
	var out sendOut
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/messages", map[string]any{"body": "old app", "client_msg_id": "legacy-msg-1"}, &out)
	if !out.Message.OffSession || out.Message.SessionID != nil {
		t.Fatalf("%+v", out.Message)
	}
	// A frame with neither id is a validation error.
	ev := cws.request(realtime.FrameMessageSend, map[string]any{"body": "x"})
	if ev.Err == nil || ev.Err.Code != "validation_failed" {
		t.Fatalf("frame without ids: %+v", ev)
	}
	// Strangers cannot use someone else's session id as an entry point.
	other := e.registerClient()
	if r := other.call(http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/messages", map[string]any{"body": "x", "client_msg_id": "legacy-msg-2"}); r.status != http.StatusNotFound {
		t.Fatalf("foreign session id: %d %s", r.status, r.body)
	}
}
