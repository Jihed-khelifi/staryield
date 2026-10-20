package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// startedSession runs steps 1–7 of the sequence diagram: request, accept,
// both connected → Active with the first minute billed.
func startedSession(t *testing.T, e *env, rate, credits int64) (client, psychic *client, cws, pws *ws, s service.SessionView) {
	t.Helper()
	e.ensurePacks()
	psychic = e.createPsychic(rate)
	pws = psychic.goOnline()
	client = e.registerClient()
	if credits > 0 {
		client.buyCredits(credits)
	}
	cws = client.connect("")

	var requested service.SessionView
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions", map[string]any{
		"psychic_slug": psychic.slug(), "request_key": uniq("req-key-"),
	}, &requested)
	if requested.Status != "requested" || requested.RatePerMinute != rate {
		t.Fatalf("requested: %+v", requested)
	}
	// Step 4: the psychic is notified in real time.
	ev := pws.waitFor(realtime.EventSessionRequested, 5*time.Second)
	if got := sessionFrom(t, ev.Data); got.ID != requested.ID || got.Client.Name == "" {
		t.Fatalf("psychic got %+v", got)
	}
	// Client waits in the chatroom; psychic accepts and joins.
	cws.mustAck(realtime.FrameSessionJoin, map[string]any{"session_id": requested.ID})
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+requested.ID.String()+"/accept", nil, nil)
	pws.mustAck(realtime.FrameSessionJoin, map[string]any{"session_id": requested.ID})

	started := sessionFrom(t, cws.waitFor(realtime.EventSessionStarted, 5*time.Second).Data)
	pws.waitFor(realtime.EventSessionStarted, 5*time.Second)
	if started.Status != "active" || started.StartedAt == nil || started.BilledMinutes != 1 {
		t.Fatalf("started: %+v", started)
	}
	return client, psychic, cws, pws, started
}

func TestLiveChatFullLifecycle(t *testing.T) {
	e := newEnv(t)
	const rate = 40
	client, psychic, cws, pws, s := startedSession(t, e, rate, 250)

	// First started minute is a free promotional minute (3 granted on signup).
	if s.FreeMinutesUsed != 1 || *s.CreditsCharged != 0 {
		t.Fatalf("first minute should be free: %+v", s)
	}
	if s.PsychicEarnings != nil {
		t.Fatal("client view must not expose psychic earnings")
	}
	if w := client.wallet(); w.FreeMinutes != 2 || w.Balance != 250 {
		t.Fatalf("wallet after minute 1: %+v", w)
	}

	// Messages flow both ways with server-assigned ids and timestamps.
	ack := cws.mustAck(realtime.FrameMessageSend, map[string]any{"session_id": s.ID, "body": "Hello, will my project go well?", "client_msg_id": "cmsg-0001"})
	var sent struct {
		Message service.MessageView `json:"message"`
	}
	_ = json.Unmarshal(ack, &sent)
	if sent.Message.ID.String() == "" || sent.Message.Body != "Hello, will my project go well?" {
		t.Fatalf("ack message: %s", ack)
	}
	got := pws.waitMatch(realtime.EventMessageCreated, "will my project", 5*time.Second)
	if strings.Contains(string(got.Data), `"kind":"system"`) {
		t.Fatalf("psychic did not receive message: %s", got.Data)
	}
	// Retrying with the same client id is idempotent.
	again := cws.mustAck(realtime.FrameMessageSend, map[string]any{"session_id": s.ID, "body": "Hello, will my project go well?", "client_msg_id": "cmsg-0001"})
	var dup struct {
		Message service.MessageView `json:"message"`
	}
	_ = json.Unmarshal(again, &dup)
	if dup.Message.ID != sent.Message.ID {
		t.Fatal("duplicate send created a second message")
	}
	pws.mustAck(realtime.FrameMessageSend, map[string]any{"session_id": s.ID, "body": "Let me shuffle the cards for you.", "client_msg_id": "pmsg-0001"})
	cws.waitFor(realtime.EventMessageCreated, 5*time.Second)

	// Typing indicators are relayed to the other side only.
	pws.mustAck(realtime.FrameTyping, map[string]any{"session_id": s.ID, "typing": true})
	typing := cws.waitFor(realtime.EventTyping, 5*time.Second)
	if !strings.Contains(string(typing.Data), `"typing":true`) {
		t.Fatalf("typing: %s", typing.Data)
	}
	// Input validation: empty, too long, unknown fields.
	if e := cws.request(realtime.FrameMessageSend, map[string]any{"session_id": s.ID, "body": "   "}); e.Type != realtime.EventError || e.Err.Code != "validation_failed" {
		t.Fatalf("empty message: %+v", e)
	}
	if e := cws.request(realtime.FrameMessageSend, map[string]any{"session_id": s.ID, "body": strings.Repeat("a", 1001)}); e.Err == nil || e.Err.Code != "validation_failed" {
		t.Fatalf("long message: %+v", e)
	}
	if e := cws.request(realtime.FrameMessageSend, map[string]any{"session_id": s.ID, "body": "x", "html": "<b>"}); e.Err == nil || e.Err.Code != "bad_request" {
		t.Fatalf("unknown field: %+v", e)
	}

	// Minutes 2 and 3 use the remaining free minutes; minute 4 is paid (BR-02).
	for minute := 2; minute <= 4; minute++ {
		e.tick(time.Minute)
		billed := cws.waitMatch(realtime.EventSessionBilled, fmt.Sprintf(`"minute":%d,`, minute), 5*time.Second)
		var b struct {
			Minute int                `json:"minute"`
			Free   bool               `json:"free"`
			Wallet service.WalletView `json:"wallet"`
		}
		_ = json.Unmarshal(billed.Data, &b)
		if b.Minute != minute || b.Free != (minute <= 3) {
			t.Fatalf("minute %d billing: %s", minute, billed.Data)
		}
		pws.waitMatch(realtime.EventSessionBilled, fmt.Sprintf(`"minute":%d,`, minute), 5*time.Second)
	}
	if w := client.wallet(); w.Balance != 250-rate || w.FreeMinutes != 0 {
		t.Fatalf("wallet after paid minute: %+v", w)
	}

	// Ledger records the chat charge immutably (BR-10).
	var ledger service.Page[service.LedgerView]
	client.must(http.StatusOK, http.MethodGet, "/api/v1/wallet/ledger", nil, &ledger)
	if len(ledger.Items) < 2 || ledger.Items[0].Type != "chat_charge" || ledger.Items[0].Amount != -rate {
		t.Fatalf("ledger: %+v", ledger.Items)
	}
	if err := e.app.DB.Exec("UPDATE credit_ledger SET amount = 0 WHERE id = ?", ledger.Items[0].ID).Error; err == nil {
		t.Fatal("ledger rows must be immutable")
	}
	if err := e.app.DB.Exec("DELETE FROM credit_ledger WHERE id = ?", ledger.Items[0].ID).Error; err == nil {
		t.Fatal("ledger rows must not be deletable")
	}

	// Mark read → read receipt to the psychic.
	cws.mustAck(realtime.FrameMessageRead, map[string]any{"conversation_id": s.ConversationID, "message_id": sent.Message.ID})
	pws.waitFor(realtime.EventMessageRead, 5*time.Second)

	// Bookmark a message, list saved.
	client.must(http.StatusNoContent, http.MethodPut, "/api/v1/chat/saved/"+sent.Message.ID.String(), nil, nil)
	var saved struct {
		Items []service.MessageView `json:"items"`
	}
	client.must(http.StatusOK, http.MethodGet, "/api/v1/chat/saved", nil, &saved)
	if len(saved.Items) != 1 {
		t.Fatalf("saved: %+v", saved)
	}

	// Client ends the chat (step 15) → summary for both (step 18).
	e.clock.Advance(20 * time.Second)
	var sum service.SessionSummary
	client.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/end", nil, &sum)
	if sum.Session.Status != "settled" || sum.Session.EndReason != "client_ended" || sum.Session.BilledMinutes != 4 || sum.Messages != 2 {
		t.Fatalf("summary: %+v", sum)
	}
	if sum.DurationSeconds != 200 {
		t.Fatalf("duration: want 200s (3m20s active), got %d", sum.DurationSeconds)
	}
	ended := pws.waitFor(realtime.EventSessionEnded, 5*time.Second)
	if !strings.Contains(string(ended.Data), `"psychic_earnings":24`) {
		t.Fatalf("psychic should earn 60%% of 40 = 24: %s", ended.Data)
	}
	if strings.Contains(string(ended.Data), "credits_charged") {
		t.Fatalf("psychic view leaks client charges: %s", ended.Data)
	}
	pws.waitFor(realtime.EventSessionSettled, 5*time.Second)
	cws.waitFor(realtime.EventSessionEnded, 5*time.Second)

	// Earnings recorded per session (BR-08, FR-12).
	var earnings service.EarningsReport
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/psychic/earnings", nil, &earnings)
	if earnings.Summary.NetCredits != 24 || earnings.Summary.GrossCredits != 40 || earnings.Summary.Sessions != 1 {
		t.Fatalf("earnings: %+v", earnings.Summary)
	}
	// Past chats for both roles.
	var past service.Page[service.SessionView]
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/chat/sessions?status=settled", nil, &past)
	if len(past.Items) != 1 || past.Items[0].ID != s.ID {
		t.Fatalf("psychic past chats: %+v", past)
	}
	// History contains both messages and the system notices.
	var history service.Page[service.MessageView]
	client.must(http.StatusOK, http.MethodGet, "/api/v1/chat/conversations/"+s.ConversationID.String()+"/messages", nil, &history)
	kinds := []string{}
	for _, m := range history.Items {
		kinds = append(kinds, m.Kind+":"+m.Body[:min(len(m.Body), 15)])
	}
	if len(history.Items) != 4 || history.Items[0].Body != "session.started" || history.Items[3].Body != "session.ended:client_ended" {
		t.Fatalf("history: %v", kinds)
	}
	if !history.Items[1].Saved {
		t.Fatal("saved flag missing in history")
	}
	// Messages are rejected once the session is over.
	if e := cws.request(realtime.FrameMessageSend, map[string]any{"session_id": s.ID, "body": "still there?"}); e.Err == nil || e.Err.Code != "invalid_state" {
		t.Fatalf("message after end: %+v", e)
	}
	// Inbox shows the conversation without unread messages for the client.
	var inbox struct {
		Items []service.ConversationView `json:"items"`
	}
	client.must(http.StatusOK, http.MethodGet, "/api/v1/chat/conversations", nil, &inbox)
	if len(inbox.Items) != 1 || inbox.Items[0].Counterpart.Slug != psychic.slug() || inbox.Items[0].OpenSession != nil {
		t.Fatalf("inbox: %+v", inbox)
	}
}

func TestCreditsExhaustedEndsSession(t *testing.T) {
	e := newEnv(t)
	const rate = 100
	client, _, cws, pws, s := startedSession(t, e, rate, 250)
	// 3 free minutes, then 2 paid minutes (250 credits / 100), then the
	// next started minute cannot be paid → session ends (BR-03).
	for i := 0; i < 5; i++ {
		e.tick(time.Minute)
	}
	low := cws.waitFor(realtime.EventSessionLowBalance, 5*time.Second)
	if !strings.Contains(string(low.Data), "minutes_remaining") {
		t.Fatalf("low balance: %s", low.Data)
	}
	ended := cws.waitFor(realtime.EventSessionEnded, 5*time.Second)
	if !strings.Contains(string(ended.Data), `"reason":"credits_exhausted"`) {
		t.Fatalf("ended: %s", ended.Data)
	}
	pws.waitFor(realtime.EventSessionEnded, 5*time.Second)
	var got service.SessionView
	client.must(http.StatusOK, http.MethodGet, "/api/v1/chat/sessions/"+s.ID.String(), nil, &got)
	if got.Status != "settled" || got.BilledMinutes != 5 || *got.CreditsCharged != 200 {
		t.Fatalf("session after exhaustion: %+v", got)
	}
	if w := client.wallet(); w.Balance != 50 {
		t.Fatalf("balance must never go negative and leftover stays: %+v", w)
	}
}

func TestRequestRequiresCreditsAndOnlinePsychic(t *testing.T) {
	e := newEnv(t)
	psychic := e.createPsychic(30)
	client := e.registerClient()
	key := func() string { return uniq("req-key-") }

	// Psychic offline.
	r := client.call(http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": psychic.slug(), "request_key": key()})
	if r.status != http.StatusConflict || r.errCode(t) != "psychic_unavailable" {
		t.Fatalf("offline psychic: %d %s", r.status, r.body)
	}
	// Online flag without a live connection still counts as offline.
	psychic.must(http.StatusOK, http.MethodPatch, "/api/v1/psychic/availability", map[string]any{"availability": "online"}, nil)
	if r := client.call(http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": psychic.slug(), "request_key": key()}); r.errCode(t) != "psychic_unavailable" {
		t.Fatalf("no connection: %s", r.body)
	}
	pws := psychic.connect("")
	var list struct {
		Items []service.PsychicView `json:"items"`
	}
	client.must(http.StatusOK, http.MethodGet, "/api/v1/psychics", nil, &list)
	found := false
	for _, p := range list.Items {
		if p.Slug == psychic.slug() {
			found = p.Status == "online"
		}
	}
	if !found {
		t.Fatal("psychic should be listed online")
	}

	// A client without free minutes or credits cannot start a chat.
	if err := e.app.DB.Exec("UPDATE wallets SET free_minutes = 0 WHERE user_id = ?", client.user.ID).Error; err != nil {
		t.Fatal(err)
	}
	r = client.call(http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": psychic.slug(), "request_key": key()})
	if r.status != http.StatusPaymentRequired || r.errCode(t) != "insufficient_credits" {
		t.Fatalf("no credits: %d %s", r.status, r.body)
	}
	if err := e.app.DB.Exec("UPDATE wallets SET free_minutes = 1 WHERE user_id = ?", client.user.ID).Error; err != nil {
		t.Fatal(err)
	}
	// Idempotent request key.
	k := key()
	var first, second service.SessionView
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": psychic.slug(), "request_key": k}, &first)
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": psychic.slug(), "request_key": k}, &second)
	if first.ID != second.ID {
		t.Fatal("request key must make the request idempotent")
	}
	// Psychic is now busy for everyone else; the client cannot open a second chat.
	other := e.registerClient()
	if r := other.call(http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": psychic.slug(), "request_key": key()}); r.errCode(t) != "psychic_unavailable" {
		t.Fatalf("busy psychic: %s", r.body)
	}
	if r := client.call(http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": psychic.slug(), "request_key": key()}); r.errCode(t) != "session_already_open" {
		t.Fatalf("second session: %s", r.body)
	}
	_ = pws
}

func TestRequestExpiresAndDecline(t *testing.T) {
	e := newEnv(t)
	psychic := e.createPsychic(30)
	pws := psychic.goOnline()
	client := e.registerClient()
	cws := client.connect("")

	var s service.SessionView
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": psychic.slug(), "request_key": uniq("req-key-")}, &s)
	pws.waitFor(realtime.EventSessionRequested, 5*time.Second)
	// No answer within the timeout → Expired (BR-04).
	e.tick(91 * time.Second)
	ev := cws.waitFor(realtime.EventSessionExpired, 5*time.Second)
	if sessionFrom(t, ev.Data).EndReason != "request_timeout" {
		t.Fatalf("expired: %s", ev.Data)
	}
	if r := psychic.call(http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/accept", nil); r.status != http.StatusConflict {
		t.Fatalf("accepting an expired request: %d %s", r.status, r.body)
	}

	// Decline flow.
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": psychic.slug(), "request_key": uniq("req-key-")}, &s)
	pws.waitFor(realtime.EventSessionRequested, 5*time.Second)
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/decline", nil, nil)
	ev = cws.waitFor(realtime.EventSessionDeclined, 5*time.Second)
	if sessionFrom(t, ev.Data).Status != "declined" {
		t.Fatalf("declined: %s", ev.Data)
	}
	// Another psychic cannot act on someone else's request.
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": psychic.slug(), "request_key": uniq("req-key-")}, &s)
	intruder := e.createPsychic(30)
	if r := intruder.call(http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/accept", nil); r.status != http.StatusNotFound {
		t.Fatalf("foreign accept: %d", r.status)
	}
	// Client cancels the pending request.
	client.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/cancel", nil, nil)
	pws.waitFor(realtime.EventSessionCancelled, 5*time.Second)
}

func TestDisconnectGraceAndResume(t *testing.T) {
	e := newEnv(t)
	client, _, cws, pws, s := startedSession(t, e, 20, 250)
	lastSeen := cws.last()

	// Client connection drops → Disconnected with a grace period (BR-05).
	_ = cws.conn.Close()
	ev := pws.waitFor(realtime.EventSessionDisconnected, 5*time.Second)
	if !strings.Contains(string(ev.Data), `"party":"client"`) {
		t.Fatalf("disconnected: %s", ev.Data)
	}
	// While disconnected, billing pauses and the psychic cannot send.
	pws.drain()
	e.tick(30 * time.Second)
	if e := pws.request(realtime.FrameMessageSend, map[string]any{"session_id": s.ID, "body": "are you there?"}); e.Err == nil || e.Err.Code != "invalid_state" {
		t.Fatalf("send during disconnect: %+v", e)
	}
	pws.expectNone(realtime.EventSessionBilled, 300*time.Millisecond)

	// Reconnect with resume: missed durable events are replayed in order.
	cws2 := client.connect(lastSeen)
	replayed := cws2.waitFor(realtime.EventSessionDisconnected, 5*time.Second)
	if replayed.ID == "" {
		t.Fatal("replayed event must carry its stream id")
	}
	cws2.mustAck(realtime.FrameSessionJoin, map[string]any{"session_id": s.ID})
	resumed := sessionFrom(t, cws2.waitFor(realtime.EventSessionResumed, 5*time.Second).Data)
	if resumed.Status != "active" || resumed.Reconnects != 1 {
		t.Fatalf("resumed: %+v", resumed)
	}
	// Paid coverage continues where it stopped: minute 2 bills 60s after start
	// minus nothing elapsed while disconnected.
	e.tick(60 * time.Second)
	cws2.waitMatch(realtime.EventSessionBilled, `"minute":2,`, 5*time.Second)

	// Disconnect again and let the grace period lapse → Ended (grace expired).
	_ = cws2.conn.Close()
	pws.waitFor(realtime.EventSessionDisconnected, 5*time.Second)
	e.tick(91 * time.Second)
	ended := pws.waitFor(realtime.EventSessionEnded, 5*time.Second)
	if !strings.Contains(string(ended.Data), `"reason":"grace_expired"`) {
		t.Fatalf("grace end: %s", ended.Data)
	}
}

func TestResumeGapRequestsResync(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	w := c.connect("1-0")
	w.waitFor(realtime.EventResyncRequired, 5*time.Second)
}

func TestWebSocketSecurity(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()

	// Foreign origin is refused (CSWSH protection).
	c.wsOrigin = "https://evil.example"
	if _, res, err := c.dial(""); err == nil || res == nil || res.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign origin websocket: %v %v", err, res)
	}
	c.wsOrigin = ""

	// Tickets are single use.
	ticket := c.ticket()
	u := "ws" + strings.TrimPrefix(e.srv.URL, "http") + "/api/v1/realtime/ws?ticket=" + ticket
	h := http.Header{"Origin": {origin}}
	conn, _, err := websocketDial(u, h)
	if err != nil {
		t.Fatalf("first use: %v", err)
	}
	conn.Close()
	if _, res, err := websocketDial(u, h); err == nil || res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("ticket reuse must fail: %v", res)
	}

	// Application ping/pong and malformed frames.
	w := c.connect("")
	if err := w.conn.WriteJSON(realtime.ClientFrame{Type: realtime.FramePing, Ref: "p1"}); err != nil {
		t.Fatal(err)
	}
	if pong := w.waitFor(realtime.EventPong, 5*time.Second); pong.Ref != "p1" {
		t.Fatalf("pong ref: %+v", pong)
	}
	if err := w.conn.WriteMessage(1, []byte("not json")); err != nil {
		t.Fatal(err)
	}
	if ev := w.waitFor(realtime.EventError, 5*time.Second); ev.Err.Code != "bad_request" {
		t.Fatalf("malformed: %+v", ev.Err)
	}
	// Joining someone else's session is not possible.
	other := e.registerClient()
	_ = other
	if e := w.request(realtime.FrameSessionJoin, map[string]any{"session_id": "01a15a00-0000-7000-8000-000000000000"}); e.Err == nil || e.Err.Code != "not_found" {
		t.Fatalf("join unknown session: %+v", e)
	}

	// Deactivation closes live sockets immediately with 4001.
	admin := e.createAdmin()
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/users/"+c.user.ID.String(), map[string]any{"status": "deactivated"}, nil)
	w.waitFor(realtime.EventAuthRevoked, 5*time.Second)
	if code := w.waitClosed(5 * time.Second); code != realtime.CloseUnauthorized {
		t.Fatalf("close code: %d", code)
	}
	if r := c.call(http.MethodGet, "/api/v1/me", nil); r.status != http.StatusForbidden || r.errCode(t) != "account_disabled" {
		t.Fatalf("deactivated access: %d %s", r.status, r.body)
	}
}

func TestPsychicEndsAndPresenceBroadcast(t *testing.T) {
	e := newEnv(t)
	_, psychic, cws, _, s := startedSession(t, e, 25, 250)
	// Observers see the psychic as busy while in a reading.
	observer := e.registerClient()
	var p service.PsychicView
	observer.must(http.StatusOK, http.MethodGet, "/api/v1/psychics/"+psychic.slug(), nil, &p)
	if p.Status != "busy" {
		t.Fatalf("status during session: %s", p.Status)
	}
	ows := observer.connect("")
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/end", nil, nil)
	ended := cws.waitFor(realtime.EventSessionEnded, 5*time.Second)
	if !strings.Contains(string(ended.Data), `"reason":"psychic_ended"`) {
		t.Fatalf("ended: %s", ended.Data)
	}
	ev := ows.waitFor(realtime.EventPsychicStatusChanged, 5*time.Second)
	if !strings.Contains(string(ev.Data), `"status":"online"`) {
		t.Fatalf("status broadcast: %s", ev.Data)
	}
}

func TestSSEFallback(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	psychic := e.createPsychic(20)
	pws := psychic.goOnline()
	client := e.registerClient()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := openSSE(t, ctx, client)
	hello := stream.next(t, realtime.EventHello)
	var h struct {
		ConnectionID string `json:"connection_id"`
	}
	_ = json.Unmarshal(hello.Data, &h)

	var s service.SessionView
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": psychic.slug(), "request_key": uniq("req-key-")}, &s)
	// SSE clients join through REST using their connection id.
	client.must(http.StatusOK, http.MethodPost, "/api/v1/realtime/presence", map[string]any{"connection_id": h.ConnectionID, "session_id": s.ID, "action": "join"}, nil)
	if r := client.call(http.MethodPost, "/api/v1/realtime/presence", map[string]any{"connection_id": "c_forged", "session_id": s.ID, "action": "join"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("forged connection id: %d", r.status)
	}
	pws.waitFor(realtime.EventSessionRequested, 5*time.Second)
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/accept", nil, nil)
	pws.mustAck(realtime.FrameSessionJoin, map[string]any{"session_id": s.ID})
	started := stream.next(t, realtime.EventSessionStarted)
	if started.ID == "" {
		t.Fatal("SSE durable events must carry ids for Last-Event-ID resume")
	}
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/messages", map[string]any{"body": "via REST", "client_msg_id": "rest-0001"}, nil)
	pws.waitMatch(realtime.EventMessageCreated, "via REST", 5*time.Second)
	stream.next(t, realtime.EventMessageCreated)
}

// TestConcurrentEndAndBilling makes sure racing writers never double-charge
// or corrupt the session (row locks + idempotent ledger keys).
func TestConcurrentEndAndBilling(t *testing.T) {
	e := newEnv(t)
	client, _, _, _, s := startedSession(t, e, 10, 250)
	e.clock.Advance(3 * time.Minute)
	done := make(chan struct{}, 3)
	for i := 0; i < 3; i++ {
		go func() {
			_ = e.app.Scheduler.Tick(context.Background())
			done <- struct{}{}
		}()
	}
	r := client.call(http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/end", nil)
	for i := 0; i < 3; i++ {
		<-done
	}
	if r.status != http.StatusOK {
		t.Fatalf("end: %d %s", r.status, r.body)
	}
	var got service.SessionView
	client.must(http.StatusOK, http.MethodGet, "/api/v1/chat/sessions/"+s.ID.String(), nil, &got)
	var charges int64
	e.app.DB.Raw("SELECT count(*) FROM credit_ledger WHERE reference_id = ? AND entry_type = 'chat_charge'", s.ID).Scan(&charges)
	paid := got.BilledMinutes - got.FreeMinutesUsed
	if int64(paid) != charges || *got.CreditsCharged != int64(paid)*10 {
		t.Fatalf("inconsistent billing: minutes=%d free=%d charges=%d credits=%d", got.BilledMinutes, got.FreeMinutesUsed, charges, *got.CreditsCharged)
	}
	if w := client.wallet(); w.Balance != 250-*got.CreditsCharged {
		t.Fatalf("wallet mismatch: %+v vs charged %d", w, *got.CreditsCharged)
	}
}
