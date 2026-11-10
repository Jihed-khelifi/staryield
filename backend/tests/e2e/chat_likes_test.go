package e2e

// revisions 4.0 (§23.3): likes.

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

func likePath(id uuid.UUID) string { return "/api/v1/chat/messages/" + id.String() + "/like" }

type likedEvent struct {
	service.MessageLikes
	UserID uuid.UUID `json:"user_id"`
	Liked  bool      `json:"liked"`
}

func likedFrom(t *testing.T, data json.RawMessage) likedEvent {
	t.Helper()
	var ev likedEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		t.Fatalf("decode message.liked: %v (%s)", err, data)
	}
	return ev
}

func (c *client) like(id uuid.UUID) service.MessageLikes {
	c.e.t.Helper()
	var out service.MessageLikes
	c.must(http.StatusOK, http.MethodPut, likePath(id), nil, &out)
	return out
}

func (c *client) unlike(id uuid.UUID) service.MessageLikes {
	c.e.t.Helper()
	var out service.MessageLikes
	c.must(http.StatusOK, http.MethodDelete, likePath(id), nil, &out)
	return out
}

func sameIDs(got []uuid.UUID, want ...uuid.UUID) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range got {
		seen[id] = true
	}
	for _, id := range want {
		if !seen[id] {
			return false
		}
	}
	return true
}

func TestLikeUnlikeIsIdempotentAndReachesEveryConnection(t *testing.T) {
	e := newEnv(t, roomy)
	cl, ps, cws, pws, s := startedSession(t, e, 10, 250)
	conv := s.ConversationID
	// A second device each.
	cws2, pws2 := cl.connect(""), ps.connect("")
	msg := ps.mustSay(conv, "The cards say yes.", "")
	for _, w := range []*ws{cws, cws2, pws, pws2} {
		w.drain()
	}
	out := cl.like(msg.ID)
	if out.MessageID != msg.ID || out.ConversationID != conv || out.Seq != msg.Seq || !sameIDs(out.LikedBy, cl.user.ID) {
		t.Fatalf("like: %+v", out)
	}
	for name, w := range map[string]*ws{"client tab 1": cws, "client tab 2": cws2, "advisor tab 1": pws, "advisor tab 2": pws2} {
		ev := likedFrom(t, w.waitFor(realtime.EventMessageLiked, 5*time.Second).Data)
		if ev.MessageID != msg.ID || ev.ConversationID != conv || ev.Seq != msg.Seq || ev.UserID != cl.user.ID || !ev.Liked || !sameIDs(ev.LikedBy, cl.user.ID) {
			t.Fatalf("%s: %+v", name, ev)
		}
	}
	// Idempotent: the same like again changes nothing and is not re-announced.
	if again := cl.like(msg.ID); !sameIDs(again.LikedBy, cl.user.ID) {
		t.Fatalf("second like: %+v", again)
	}
	pws.expectNone(realtime.EventMessageLiked, 400*time.Millisecond)
	if rows := e.likeRows(conv); rows != 1 {
		t.Fatalf("%d like rows", rows)
	}

	// The advisor likes their own message too (any text or image of the chat).
	both := ps.like(msg.ID)
	if !sameIDs(both.LikedBy, cl.user.ID, ps.user.ID) {
		t.Fatalf("both likes: %+v", both)
	}
	ev := likedFrom(t, cws.waitFor(realtime.EventMessageLiked, 5*time.Second).Data)
	if ev.UserID != ps.user.ID || !ev.Liked || !sameIDs(ev.LikedBy, cl.user.ID, ps.user.ID) {
		t.Fatalf("advisor like event: %+v", ev)
	}

	// Read paths carry liked_by; messages nobody liked carry none.
	var hist service.MessagePage
	ps.must(http.StatusOK, http.MethodGet, convMessages(conv)+"?limit=50", nil, &hist)
	for _, m := range hist.Items {
		if m.ID == msg.ID && !sameIDs(m.LikedBy, cl.user.ID, ps.user.ID) {
			t.Fatalf("history liked_by: %+v", m.LikedBy)
		}
		if m.ID != msg.ID && len(m.LikedBy) != 0 {
			t.Fatalf("history: %s liked_by %v", m.ID, m.LikedBy)
		}
	}
	for _, m := range allMessages(t, cl, conv) {
		if m.ID == msg.ID && len(m.LikedBy) != 2 {
			t.Fatalf("since liked_by: %+v", m.LikedBy)
		}
	}
	if last := inboxEntry(t, cl, conv).LastMessage; last == nil || last.ID != msg.ID || len(last.LikedBy) != 2 {
		t.Fatalf("inbox preview liked_by: %+v", last)
	}
	cl.must(http.StatusNoContent, http.MethodPut, "/api/v1/chat/saved/"+msg.ID.String(), nil, nil)
	var saved service.Page[service.MessageView]
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/chat/saved", nil, &saved)
	if len(saved.Items) != 1 || len(saved.Items[0].LikedBy) != 2 {
		t.Fatalf("saved liked_by: %+v", saved.Items)
	}
	// An idempotent send replay returns the current likes too.
	cmid := uniq("lk-")
	first := cl.sayReply(conv, "Likeable.", cmid, "")
	var sent sendOut
	first.decode(t, &sent)
	ps.like(sent.Message.ID)
	r := cl.sayReply(conv, "Likeable.", cmid, "")
	var replay sendOut
	r.decode(t, &replay)
	if r.status != http.StatusOK || !sameIDs(replay.Message.LikedBy, ps.user.ID) {
		t.Fatalf("replay: %d %+v", r.status, replay.Message.LikedBy)
	}

	before := inboxEntry(t, cl, conv)
	// Unlike: announced once, idempotent afterwards.
	for _, w := range []*ws{cws, cws2, pws, pws2} {
		w.drain()
	}
	off := cl.unlike(msg.ID)
	if !sameIDs(off.LikedBy, ps.user.ID) {
		t.Fatalf("unlike: %+v", off)
	}
	for _, w := range []*ws{cws, cws2, pws, pws2} {
		ev := likedFrom(t, w.waitFor(realtime.EventMessageLiked, 5*time.Second).Data)
		if ev.UserID != cl.user.ID || ev.Liked || !sameIDs(ev.LikedBy, ps.user.ID) {
			t.Fatalf("unlike event: %+v", ev)
		}
	}
	if again := cl.unlike(msg.ID); !sameIDs(again.LikedBy, ps.user.ID) {
		t.Fatalf("second unlike: %+v", again)
	}
	pws.expectNone(realtime.EventMessageLiked, 400*time.Millisecond)
	ps.unlike(msg.ID)
	if empty := cl.unlike(msg.ID); empty.LikedBy == nil || len(empty.LikedBy) != 0 {
		t.Fatalf("liked_by must be an empty array: %+v", empty)
	}
	// "liked_by": [] on the wire for the explicit response.
	r = cl.call(http.MethodDelete, likePath(msg.ID), nil)
	var raw map[string]json.RawMessage
	r.decode(t, &raw)
	if string(raw["liked_by"]) != "[]" {
		t.Fatalf("raw: %s", r.body)
	}

	// Likes never touch counters, the conversation head, notifications or push.
	after := inboxEntry(t, cl, conv)
	if after.LastSeq < before.LastSeq || after.Unread != before.Unread {
		t.Fatalf("inbox changed by likes: before %+v after %+v", before, after)
	}
	var head struct{ LastSeq int64 }
	e.app.DB.Table("conversations").Select("last_seq").Where("id = ?", conv).Scan(&head)
	var msgs int64
	e.app.DB.Table("chat_messages").Where("conversation_id = ?", conv).Count(&msgs)
	if head.LastSeq != msgs {
		t.Fatalf("last_seq %d with %d messages: a like took a seq", head.LastSeq, msgs)
	}
	if len(e.push.Sent()) != 0 {
		t.Fatalf("a like sent a push: %+v", e.push.Sent())
	}
}

func TestLikeGuards(t *testing.T) {
	e := newEnv(t, roomy)
	cl, ps, _, _, s := startedSession(t, e, 10, 250)
	conv := s.ConversationID
	msg := cl.mustSay(conv, "Private words.", "")
	var hist service.MessagePage
	cl.must(http.StatusOK, http.MethodGet, convMessages(conv)+"?limit=50", nil, &hist)
	var system service.MessageView
	for _, m := range hist.Items {
		if m.Kind == "system" {
			system = m
		}
	}
	// Outsiders (another client, another advisor) and unknown ids: 404.
	stranger := e.registerClient()
	otherAdvisor := e.createPsychic(10)
	for name, c := range map[string]*client{"client": stranger, "advisor": otherAdvisor} {
		for _, method := range []string{http.MethodPut, http.MethodDelete} {
			if r := c.call(method, likePath(msg.ID), nil); r.status != http.StatusNotFound || r.errCode(t) != "not_found" {
				t.Fatalf("%s %s: %d %s", name, method, r.status, r.body)
			}
		}
	}
	if r := cl.call(http.MethodPut, likePath(uuid.New()), nil); r.status != http.StatusNotFound {
		t.Fatalf("unknown id: %d %s", r.status, r.body)
	}
	if r := cl.call(http.MethodPut, "/api/v1/chat/messages/not-a-uuid/like", nil); r.status < 400 || r.status >= 500 {
		t.Fatalf("malformed id: %d", r.status)
	}
	if rows := e.likeRows(conv); rows != 0 {
		t.Fatalf("outsiders left %d likes", rows)
	}
	// System notices cannot be liked or unliked.
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		if r := cl.call(method, likePath(system.ID), nil); r.status != http.StatusUnprocessableEntity || r.errCode(t) != "message_not_likeable" {
			t.Fatalf("system %s: %d %s", method, r.status, r.body)
		}
	}
	// Anonymous callers are refused.
	if r := e.client().call(http.MethodPut, likePath(msg.ID), nil); r.status != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", r.status)
	}
	_ = ps
}

func TestLikesFromBothParticipantsRaceCleanly(t *testing.T) {
	e := newEnv(t, roomy)
	cl, ps, cws, pws, s := startedSession(t, e, 10, 250)
	conv := s.ConversationID
	const rounds = 15
	msgs := make([]service.MessageView, rounds)
	for i := range msgs {
		msgs[i] = ps.mustSay(conv, "message to like", "")
	}
	cws.drain()
	pws.drain()
	var wg sync.WaitGroup
	errs := make(chan string, 4*rounds)
	for _, m := range msgs {
		for _, c := range []*client{cl, ps} {
			wg.Add(1)
			go func(c *client, id uuid.UUID) {
				defer wg.Done()
				// Hammer the same like twice as well: still no error.
				for k := 0; k < 2; k++ {
					r := c.call(http.MethodPut, likePath(id), nil)
					if r.status != http.StatusOK {
						errs <- r.errCode(t) + " " + string(r.body)
					}
				}
			}(c, m.ID)
		}
	}
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Fatalf("concurrent like failed: %s", msg)
	}
	if rows := e.likeRows(conv); rows != 2*rounds {
		t.Fatalf("%d like rows, want %d", rows, 2*rounds)
	}
	// Each connection receives two events per message; the last event for a
	// message names both likers (events were published in lock order).
	for name, w := range map[string]*ws{"client": cws, "advisor": pws} {
		final := map[uuid.UUID]likedEvent{}
		counts := map[uuid.UUID]int{}
		for len(counts) < rounds || minCount(counts) < 2 {
			ev := likedFrom(t, w.waitFor(realtime.EventMessageLiked, 5*time.Second).Data)
			final[ev.MessageID] = ev
			counts[ev.MessageID]++
		}
		for id, ev := range final {
			if !sameIDs(ev.LikedBy, cl.user.ID, ps.user.ID) {
				t.Fatalf("%s: last event of %s is stale: %+v", name, id, ev.LikedBy)
			}
		}
	}
	// Unliking concurrently ends empty.
	for _, m := range msgs {
		for _, c := range []*client{cl, ps} {
			wg.Add(1)
			go func(c *client, id uuid.UUID) {
				defer wg.Done()
				if r := c.call(http.MethodDelete, likePath(id), nil); r.status != http.StatusOK {
					t.Errorf("unlike: %d %s", r.status, r.body)
				}
			}(c, m.ID)
		}
	}
	wg.Wait()
	if rows := e.likeRows(conv); rows != 0 {
		t.Fatalf("%d likes left", rows)
	}
}

func minCount(m map[uuid.UUID]int) int {
	lo := 1 << 30
	for _, n := range m {
		lo = min(lo, n)
	}
	return lo
}

func TestLikeRateLimitBucket(t *testing.T) {
	e := newEnv(t, roomy, func(cfg *config.Config) {
		cfg.RateLimit.Scale = 1
		big := config.Rule{Rate: 100000, Period: time.Hour, Burst: 100000}
		cfg.RateLimit.Global, cfg.RateLimit.Auth, cfg.RateLimit.Writes, cfg.RateLimit.Ingest = big, big, big, big
		cfg.RateLimit.Like = config.Rule{Rate: 3, Period: time.Hour, Burst: 3}
	})
	cl, ps, _, _, s := startedSession(t, e, 10, 250)
	msg := ps.mustSay(s.ConversationID, "Like me.", "")
	for i := 0; i < 3; i++ {
		cl.like(msg.ID)
	}
	r := cl.call(http.MethodPut, likePath(msg.ID), nil)
	if r.status != http.StatusTooManyRequests || r.errCode(t) != "rate_limited" || r.header.Get("Retry-After") == "" {
		t.Fatalf("like budget not enforced: %d %s", r.status, r.body)
	}
	// The budget is per user: the other participant still can.
	ps.like(msg.ID)
}

func TestLikesAreExportedAndErased(t *testing.T) {
	e := newEnv(t, roomy)
	cl, ps, _, _, s := startedSession(t, e, 10, 250)
	conv := s.ConversationID
	fromAdvisor := ps.mustSay(conv, "Advisor words.", "")
	fromClient := cl.mustSay(conv, "Client words.", "")
	cl.like(fromAdvisor.ID)
	cl.like(fromClient.ID)
	ps.like(fromClient.ID)

	files := exportFiles(t, e, cl)
	var likes []struct {
		MessageID uuid.UUID `json:"message_id"`
	}
	if err := json.Unmarshal(files["chat/message_likes.json"], &likes); err != nil || len(likes) != 2 {
		t.Fatalf("export likes: %v %s", err, files["chat/message_likes.json"])
	}
	// Closing the client account erases their likes; the advisor's stay.
	cl.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/close", map[string]any{"password": strongPassword, "confirmation": "DELETE"}, nil)
	e.clock.Advance(31*24*time.Hour + time.Minute)
	runClosure(t, e)
	var byClient, byAdvisor int64
	e.app.DB.Table("message_likes").Where("user_id = ?", cl.user.ID).Count(&byClient)
	e.app.DB.Table("message_likes").Where("user_id = ?", ps.user.ID).Count(&byAdvisor)
	if byClient != 0 || byAdvisor != 1 {
		t.Fatalf("after erasure: client likes %d, advisor likes %d", byClient, byAdvisor)
	}
	// Deleting a message (cascade) removes its likes.
	if err := e.app.DB.Exec("UPDATE conversations SET last_message_id = NULL WHERE id = ?", conv).Error; err != nil {
		t.Fatal(err)
	}
	if err := e.app.DB.Exec("DELETE FROM chat_messages WHERE id = ?", fromClient.ID).Error; err != nil {
		t.Fatal(err)
	}
	if left := e.likeRows(conv); left != 0 {
		t.Fatalf("likes survived their message: %d", left)
	}
}
