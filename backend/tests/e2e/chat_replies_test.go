package e2e

// revisions 4.0 (§23.2): reply to a message.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

func (e *env) messageRows(conv uuid.UUID) int64 {
	e.t.Helper()
	var n int64
	if err := e.app.DB.Table("chat_messages").Where("conversation_id = ?", conv).Count(&n).Error; err != nil {
		e.t.Fatal(err)
	}
	return n
}

func wantReply(t *testing.T, what string, got *service.ReplyView, target service.MessageView, kind, excerpt string) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s: no reply_to", what)
	}
	if got.ID != target.ID || got.Seq != target.Seq || got.Kind != kind || got.Excerpt != excerpt ||
		got.SenderID == nil || target.SenderID == nil || *got.SenderID != *target.SenderID {
		t.Fatalf("%s: reply_to %+v, want target %s seq %d kind %s excerpt %q", what, got, target.ID, target.Seq, kind, excerpt)
	}
}

func TestReplyThroughEverySendPath(t *testing.T) {
	e := newEnv(t, roomy)
	cl, ps, cws, pws, s := startedSession(t, e, 10, 250)
	conv := s.ConversationID
	body := "Should I take the job?\n\n   The offer is in   Lisbon 🌙"
	excerpt := "Should I take the job? The offer is in Lisbon 🌙"
	target := cl.mustSay(conv, body, "")
	if target.ReplyTo != nil {
		t.Fatalf("a plain message carries a quote: %+v", target.ReplyTo)
	}
	cws.drain()
	pws.drain()

	// WebSocket frame (conversation-addressed).
	var ack struct {
		Message service.MessageView `json:"message"`
	}
	raw := pws.mustAck(realtime.FrameMessageSend, map[string]any{"conversation_id": conv, "body": "Take it.", "client_msg_id": uniq("ws-"), "reply_to_id": target.ID})
	if err := json.Unmarshal(raw, &ack); err != nil {
		t.Fatal(err)
	}
	wantReply(t, "ws ack", ack.Message.ReplyTo, target, "text", excerpt)
	// The event both participants receive carries the quote too.
	wantReply(t, "client event", messageEvent(t, cws.waitMatch(realtime.EventMessageCreated, ack.Message.ID.String(), 5*time.Second).Data).ReplyTo, target, "text", excerpt)
	wantReply(t, "advisor event", messageEvent(t, pws.waitMatch(realtime.EventMessageCreated, ack.Message.ID.String(), 5*time.Second).Data).ReplyTo, target, "text", excerpt)

	// Legacy WebSocket frame (session-addressed).
	raw = pws.mustAck(realtime.FrameMessageSend, map[string]any{"session_id": s.ID, "body": "Legacy frame.", "client_msg_id": uniq("ws-"), "reply_to_id": target.ID})
	if err := json.Unmarshal(raw, &ack); err != nil {
		t.Fatal(err)
	}
	wantReply(t, "legacy ws ack", ack.Message.ReplyTo, target, "text", excerpt)

	// REST: conversation-addressed and the legacy session endpoint.
	viaREST := ps.mustSay(conv, "Via REST.", target.ID.String())
	wantReply(t, "rest", viaREST.ReplyTo, target, "text", excerpt)
	var legacy sendOut
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/messages",
		map[string]any{"body": "Legacy REST.", "client_msg_id": uniq("lg-"), "reply_to_id": viaREST.ID}, &legacy)
	wantReply(t, "legacy rest", legacy.Message.ReplyTo, viaREST, "text", "Via REST.")

	// Image uploads: both endpoints, with and without a caption; an image can
	// be quoted too (a bare image has an empty excerpt).
	png := pngBytes(t, 32, 32)
	var bare sendOut
	r := ps.sendConvImage(conv, png, map[string]string{"client_msg_id": uniq("img-"), "reply_to_id": target.ID.String()})
	if r.status != http.StatusCreated {
		t.Fatalf("image reply: %d %s", r.status, r.body)
	}
	r.decode(t, &bare)
	wantReply(t, "conversation upload", bare.Message.ReplyTo, target, "text", excerpt)
	if bare.Message.Kind != "image" || len(bare.Message.Attachments) != 1 {
		t.Fatalf("image message: %+v", bare.Message)
	}
	var capt sendOut
	r = cl.sendImage(s.ID.String(), png, map[string]string{"client_msg_id": uniq("img-"), "caption": "Thanks!", "reply_to_id": bare.Message.ID.String()})
	if r.status != http.StatusCreated {
		t.Fatalf("legacy image reply: %d %s", r.status, r.body)
	}
	r.decode(t, &capt)
	wantReply(t, "legacy upload (quotes a bare image)", capt.Message.ReplyTo, bare.Message, "image", "")

	// A text quoting a captioned image shows the caption.
	cap2 := cl.sendConvImage(conv, png, map[string]string{"client_msg_id": uniq("img-"), "caption": "My   birth\nchart"})
	var capOut sendOut
	cap2.decode(t, &capOut)
	reply := ps.mustSay(conv, "Lovely chart.", capOut.Message.ID.String())
	wantReply(t, "caption excerpt", reply.ReplyTo, capOut.Message, "image", "My birth chart")

	// Every read path carries reply_to; one with no quote carries none.
	var hist service.MessagePage
	cl.must(http.StatusOK, http.MethodGet, convMessages(conv)+"?limit=100", nil, &hist)
	byID := map[uuid.UUID]service.MessageView{}
	for _, m := range hist.Items {
		byID[m.ID] = m
	}
	wantReply(t, "history", byID[ack.Message.ID].ReplyTo, target, "text", excerpt)
	wantReply(t, "history (image)", byID[capt.Message.ID].ReplyTo, bare.Message, "image", "")
	if byID[target.ID].ReplyTo != nil {
		t.Fatal("history: an unquoted message gained a quote")
	}
	since := allMessages(t, ps, conv)
	for _, m := range since {
		if m.ID == viaREST.ID {
			wantReply(t, "since", m.ReplyTo, target, "text", excerpt)
		}
	}
	ps.must(http.StatusNoContent, http.MethodPut, "/api/v1/chat/saved/"+reply.ID.String(), nil, nil)
	var saved service.Page[service.MessageView]
	ps.must(http.StatusOK, http.MethodGet, "/api/v1/chat/saved", nil, &saved)
	if len(saved.Items) != 1 || !saved.Items[0].Saved {
		t.Fatalf("saved: %+v", saved.Items)
	}
	wantReply(t, "saved", saved.Items[0].ReplyTo, capOut.Message, "image", "My birth chart")
	// The inbox preview of the last message carries the quote.
	last := inboxEntry(t, ps, conv).LastMessage
	if last == nil || last.ID != reply.ID {
		t.Fatalf("inbox preview: %+v", last)
	}
	wantReply(t, "inbox last_message", last.ReplyTo, capOut.Message, "image", "My birth chart")

	// Old clients: no reply_to_id, no reply_to or liked_by keys on the wire.
	r = cl.sayReply(conv, "Plain message from an old app.", uniq("old-"), "")
	if r.status != http.StatusCreated || strings.Contains(string(r.body), "reply_to") || strings.Contains(string(r.body), "liked_by") {
		t.Fatalf("plain send: %d %s", r.status, r.body)
	}
	r = cl.call(http.MethodGet, convMessages(conv)+"?limit=1", nil)
	if strings.Contains(string(r.body), "reply_to") || strings.Contains(string(r.body), "liked_by") {
		t.Fatalf("history of a plain message: %s", r.body)
	}
}

func TestReplyTargetsAreValidatedAndNothingIsStored(t *testing.T) {
	e := newEnv(t, roomy)
	cl, ps, _, pws, s := startedSession(t, e, 10, 250)
	conv := s.ConversationID
	mine := cl.mustSay(conv, "My message.", "")
	// Another conversation's message, a system notice and an unknown id.
	_, other, _, _, s2 := startedSession(t, e, 10, 250)
	foreign := other.mustSay(s2.ConversationID, "Someone else's secret.", "")
	var hist service.MessagePage
	cl.must(http.StatusOK, http.MethodGet, convMessages(conv)+"?limit=100", nil, &hist)
	var system service.MessageView
	for _, m := range hist.Items {
		if m.Kind == "system" {
			system = m
		}
	}
	if system.ID == uuid.Nil {
		t.Fatal("no system message in the transcript")
	}

	rows, lastSeq := e.messageRows(conv), mine.Seq
	objects := len(e.chat.Keys())
	for name, id := range map[string]string{"another conversation": foreign.ID.String(), "system notice": system.ID.String(), "unknown": uuid.NewString()} {
		cmid := uniq("bad-")
		r := cl.sayReply(conv, "Should never be stored.", cmid, id)
		if r.status != http.StatusUnprocessableEntity || r.errCode(t) != "reply_target_invalid" {
			t.Fatalf("%s via REST: %d %s", name, r.status, r.body)
		}
		if strings.Contains(string(r.body), foreign.Body) {
			t.Fatalf("%s: the refusal leaks the other message: %s", name, r.body)
		}
		// WebSocket frame.
		ev := pws.request(realtime.FrameMessageSend, map[string]any{"conversation_id": conv, "body": "Never stored.", "client_msg_id": uniq("bad-"), "reply_to_id": id})
		if ev.Type != realtime.EventError || ev.Err == nil || ev.Err.Code != "reply_target_invalid" {
			t.Fatalf("%s via WS: %+v", name, ev)
		}
		// Image uploads are refused before anything is stored.
		for _, up := range []func() response{
			func() response {
				return ps.sendConvImage(conv, pngBytes(t, 32, 32), map[string]string{"client_msg_id": uniq("img-"), "reply_to_id": id})
			},
			func() response {
				return ps.sendImage(s.ID.String(), pngBytes(t, 32, 32), map[string]string{"client_msg_id": uniq("img-"), "reply_to_id": id})
			},
		} {
			if r := up(); r.status != http.StatusUnprocessableEntity || r.errCode(t) != "reply_target_invalid" {
				t.Fatalf("%s via upload: %d %s", name, r.status, r.body)
			}
		}
	}
	// A malformed id is a validation error, also stored nowhere.
	if r := cl.call(http.MethodPost, convMessages(conv), map[string]any{"body": "x", "reply_to_id": "not-a-uuid"}); r.status < 400 {
		t.Fatalf("malformed id accepted: %d %s", r.status, r.body)
	}
	if r := ps.sendConvImage(conv, pngBytes(t, 32, 32), map[string]string{"reply_to_id": "not-a-uuid"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("malformed upload id: %d %s", r.status, r.body)
	}
	if got := e.messageRows(conv); got != rows {
		t.Fatalf("%d messages stored by refused replies", got-rows)
	}
	var conversation struct{ LastSeq int64 }
	e.app.DB.Table("conversations").Select("last_seq").Where("id = ?", conv).Scan(&conversation)
	if conversation.LastSeq < lastSeq || e.messageRows(conv) != rows {
		t.Fatalf("conversation head moved: %d (was %d)", conversation.LastSeq, lastSeq)
	}
	if len(e.chat.Keys()) != objects {
		t.Fatal("an image object was stored for a refused reply")
	}
	if inboxEntry(t, cl, conv).Unread != 0 {
		t.Fatal("a refused reply counted as unread")
	}
}

func TestReplyReplayReturnsTheOriginalUnchanged(t *testing.T) {
	e := newEnv(t, roomy)
	cl, ps, _, _, s := startedSession(t, e, 10, 250)
	conv := s.ConversationID
	first := cl.mustSay(conv, "First.", "")
	second := cl.mustSay(conv, "Second.", "")
	cmid := uniq("replay-")
	r := ps.sayReply(conv, "Answer to the first.", cmid, first.ID.String())
	if r.status != http.StatusCreated {
		t.Fatalf("%d %s", r.status, r.body)
	}
	var orig sendOut
	r.decode(t, &orig)
	rows := e.messageRows(conv)

	// The retry names another (even an invalid) target: the original comes
	// back, unchanged, with 200 and nothing new stored.
	for _, target := range []string{second.ID.String(), uuid.NewString(), ""} {
		r = ps.sayReply(conv, "Answer to the first.", cmid, target)
		if r.status != http.StatusOK {
			t.Fatalf("replay with target %q: %d %s", target, r.status, r.body)
		}
		var again sendOut
		r.decode(t, &again)
		if again.Message.ID != orig.Message.ID || again.Message.Seq != orig.Message.Seq {
			t.Fatalf("replay returned another message: %+v", again.Message)
		}
		wantReply(t, "replay", again.Message.ReplyTo, first, "text", "First.")
	}
	if e.messageRows(conv) != rows {
		t.Fatal("a replay stored a message")
	}
	// Same for an image reply.
	png := pngBytes(t, 32, 32)
	imgID := uniq("img-")
	r = ps.sendConvImage(conv, png, map[string]string{"client_msg_id": imgID, "reply_to_id": first.ID.String()})
	var img sendOut
	r.decode(t, &img)
	r = ps.sendConvImage(conv, png, map[string]string{"client_msg_id": imgID, "reply_to_id": uuid.NewString()})
	var imgAgain sendOut
	r.decode(t, &imgAgain)
	if r.status != http.StatusOK || imgAgain.Message.ID != img.Message.ID {
		t.Fatalf("image replay: %d %s", r.status, r.body)
	}
	wantReply(t, "image replay", imgAgain.Message.ReplyTo, first, "text", "First.")
}

// A page of history costs the same number of queries whether none or all of
// its messages are replies (one batched lookup, never one per message).
func TestReplyLookupsAreBatchedPerPage(t *testing.T) {
	e := newEnv(t, roomy)
	cl, ps, _, _, s := startedSession(t, e, 10, 250)
	conv := s.ConversationID
	root := cl.mustSay(conv, "Root.", "")
	for i := 0; i < 5; i++ {
		ps.mustSay(conv, fmt.Sprintf("plain %d", i), "")
	}
	page := func(limit int) *sqlLog {
		time.Sleep(400 * time.Millisecond) // let the asynchronous side effects of the sends finish
		return recordSQL(e, func() {
			cl.must(http.StatusOK, http.MethodGet, convMessages(conv)+fmt.Sprintf("?limit=%d", limit), nil, nil)
		})
	}
	plain := page(8)
	for i := 0; i < 40; i++ {
		who := cl
		if i%2 == 0 {
			who = ps
		}
		who.mustSay(conv, fmt.Sprintf("reply %d", i), root.ID.String())
	}
	small, large := page(8), page(50)
	for name, l := range map[string]*sqlLog{"plain": plain, "8 replies": small, "46 replies": large} {
		if got := l.count(`FROM "chat_messages" WHERE id IN`); name != "plain" && got != 1 {
			t.Fatalf("%s: %d reply lookups, want 1", name, got)
		}
		if got := l.count(`FROM "message_likes"`); got != 1 {
			t.Fatalf("%s: %d likes lookups, want 1", name, got)
		}
	}
	if plain.count(`FROM "chat_messages" WHERE id IN`) != 0 {
		t.Fatal("a page without replies still ran the reply lookup")
	}
	if a, b := small.count(`"chat_messages"`)+small.count(`"message_likes"`), large.count(`"chat_messages"`)+large.count(`"message_likes"`); a != b {
		t.Fatalf("history page queries grow with the page: %d vs %d (%v)", a, b, large.stmt)
	}
	// Same for the since endpoint, the saved list and the inbox.
	time.Sleep(400 * time.Millisecond)
	sinceLog := recordSQL(e, func() {
		ps.must(http.StatusOK, http.MethodGet, convMessages(conv)+"/since?after_seq=0&limit=500", nil, nil)
	})
	if sinceLog.count(`FROM "chat_messages" WHERE id IN`) != 1 || sinceLog.count(`FROM "message_likes"`) != 1 {
		t.Fatalf("since: %v", sinceLog.stmt)
	}
}
