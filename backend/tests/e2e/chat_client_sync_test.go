package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// Saved messages page by the time they were saved, newest first, with a
// keyset cursor; the last page never announces an empty next page, and one
// user never sees another user's bookmarks.
func TestChatSavedMessagesPaging(t *testing.T) {
	e := newEnv(t)
	client, psychic, cws, _, s := startedSession(t, e, 5, 0)

	var ids []string
	for i := 1; i <= 5; i++ {
		var sent struct {
			Message service.MessageView `json:"message"`
		}
		ack := cws.mustAck(realtime.FrameMessageSend, map[string]any{
			"session_id": s.ID, "body": fmt.Sprintf("Insight %d", i), "client_msg_id": fmt.Sprintf("saved-page-%04d", i)})
		if err := json.Unmarshal(ack, &sent); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, sent.Message.ID.String())
	}
	// Saved in message order, one second apart: newest save first.
	for _, id := range ids {
		client.must(http.StatusNoContent, http.MethodPut, "/api/v1/chat/saved/"+id, nil, nil)
		e.clock.Advance(time.Second)
	}
	base := "/api/v1/chat/saved?conversation_id=" + s.ConversationID.String()

	var all service.Page[service.MessageView]
	client.must(http.StatusOK, http.MethodGet, base, nil, &all)
	if len(all.Items) != 5 || all.NextCursor != "" {
		t.Fatalf("default page: %d items, cursor %q", len(all.Items), all.NextCursor)
	}
	var walked []string
	cursor := ""
	for pages := 0; ; pages++ {
		if pages > 3 {
			t.Fatal("too many pages")
		}
		var page service.Page[service.MessageView]
		client.must(http.StatusOK, http.MethodGet, base+"&limit=2&cursor="+url.QueryEscape(cursor), nil, &page)
		for _, m := range page.Items {
			if !m.Saved {
				t.Fatalf("saved flag missing: %+v", m)
			}
			walked = append(walked, m.ID.String())
		}
		if page.NextCursor == "" {
			if len(page.Items) != 1 {
				t.Fatalf("last page: %d items", len(page.Items))
			}
			break
		}
		if len(page.Items) != 2 {
			t.Fatalf("full page: %d items", len(page.Items))
		}
		cursor = page.NextCursor
	}
	want := []string{ids[4], ids[3], ids[2], ids[1], ids[0]}
	if fmt.Sprint(walked) != fmt.Sprint(want) {
		t.Fatalf("pages walked %v, want %v", walked, want)
	}

	// Exactly two full pages: the second has no cursor.
	client.must(http.StatusNoContent, http.MethodDelete, "/api/v1/chat/saved/"+ids[0], nil, nil)
	var p1, p2 service.Page[service.MessageView]
	client.must(http.StatusOK, http.MethodGet, base+"&limit=2", nil, &p1)
	client.must(http.StatusOK, http.MethodGet, base+"&limit=2&cursor="+url.QueryEscape(p1.NextCursor), nil, &p2)
	if len(p1.Items) != 2 || p1.NextCursor == "" || len(p2.Items) != 2 || p2.NextCursor != "" {
		t.Fatalf("two full pages: %d/%q then %d/%q", len(p1.Items), p1.NextCursor, len(p2.Items), p2.NextCursor)
	}

	// Bookmarks are per user: the advisor sees none of the client's.
	var theirs service.Page[service.MessageView]
	psychic.must(http.StatusOK, http.MethodGet, base, nil, &theirs)
	if len(theirs.Items) != 0 {
		t.Fatalf("advisor sees the client's bookmarks: %+v", theirs.Items)
	}
	// A cursor from another user's list only positions the caller's own list.
	var cross service.Page[service.MessageView]
	psychic.must(http.StatusOK, http.MethodGet, base+"&limit=2&cursor="+url.QueryEscape(p1.NextCursor), nil, &cross)
	if len(cross.Items) != 0 {
		t.Fatalf("foreign cursor leaked items: %+v", cross.Items)
	}
	for _, bad := range []string{"&limit=0", "&limit=201", "&limit=x", "&cursor=bogus"} {
		if r := client.call(http.MethodGet, base+bad, nil); r.status != http.StatusUnprocessableEntity {
			t.Fatalf("saved %s: %d", bad, r.status)
		}
	}
}

// The conversation views carry the other participant's read position, and
// the advisor's per-minute billing event carries their live earnings.
func TestChatPeerReadSeqAndLiveEarnings(t *testing.T) {
	e := newEnv(t)
	client, psychic, cws, pws, s := startedSession(t, e, 40, 250)

	var sent struct {
		Message service.MessageView `json:"message"`
	}
	if err := json.Unmarshal(cws.mustAck(realtime.FrameMessageSend, map[string]any{
		"session_id": s.ID, "body": "Can you see my cards?", "client_msg_id": "peer-read-0001"}), &sent); err != nil {
		t.Fatal(err)
	}
	pws.waitFor(realtime.EventMessageCreated, 5*time.Second)
	if got := inboxEntry(t, client, s.ConversationID); got.PeerReadSeq != 0 {
		t.Fatalf("unread by the advisor yet: %+v", got)
	}
	pws.mustAck(realtime.FrameMessageRead, map[string]any{"conversation_id": s.ConversationID, "seq": sent.Message.Seq})
	cws.waitMatch(realtime.EventMessageRead, fmt.Sprintf(`"read_seq":%d`, sent.Message.Seq), 5*time.Second)

	if got := inboxEntry(t, client, s.ConversationID); got.PeerReadSeq != sent.Message.Seq || got.ReadSeq < sent.Message.Seq {
		t.Fatalf("client inbox read positions: %+v", got)
	}
	var thread service.PsychicThread
	client.must(http.StatusOK, http.MethodGet, "/api/v1/chat/psychics/"+psychic.slug(), nil, &thread)
	if thread.Conversation == nil || thread.Conversation.PeerReadSeq != sent.Message.Seq {
		t.Fatalf("thread read position: %+v", thread.Conversation)
	}
	if got := inboxEntry(t, psychic, s.ConversationID); got.ReadSeq != sent.Message.Seq || got.PeerReadSeq < sent.Message.Seq {
		t.Fatalf("advisor inbox read positions: %+v", got)
	}

	// Three free signup minutes, then a paid minute at 40: the advisor's
	// event carries 60 % of it in credits and in money.
	e.tick(time.Minute + time.Second)
	e.tick(time.Minute)
	e.tick(time.Minute)
	for {
		b := pws.waitFor(realtime.EventSessionBilled, 5*time.Second)
		var billed struct {
			Minute               int    `json:"minute"`
			Charged              int64  `json:"charged"`
			PsychicEarnings      *int64 `json:"psychic_earnings"`
			PsychicEarningsCents *int64 `json:"psychic_earnings_cents"`
			Currency             string `json:"currency"`
			MinutesRemaining     *int64 `json:"minutes_remaining"`
			Wallet               any    `json:"wallet"`
		}
		if err := json.Unmarshal(b.Data, &billed); err != nil {
			t.Fatal(err)
		}
		if billed.MinutesRemaining != nil || billed.Wallet != nil {
			t.Fatalf("the advisor must not see the client's wallet: %s", b.Data)
		}
		if billed.PsychicEarnings == nil {
			t.Fatalf("live earnings missing: %s", b.Data)
		}
		if billed.Minute < 4 {
			if *billed.PsychicEarnings != 0 {
				t.Fatalf("free minutes earn nothing: %s", b.Data)
			}
			continue
		}
		if billed.Charged != 40 || *billed.PsychicEarnings != 24 || billed.PsychicEarningsCents == nil || *billed.PsychicEarningsCents != 48 || billed.Currency != "USD" {
			t.Fatalf("paid minute: %s", b.Data)
		}
		break
	}
	// The same values as the advisor's session view.
	var view service.SessionView
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/chat/sessions/"+s.ID.String(), nil, &view)
	if view.PsychicEarnings == nil || *view.PsychicEarnings != 24 || view.PsychicEarningsCents == nil || *view.PsychicEarningsCents != 48 {
		t.Fatalf("advisor session view: %+v", view)
	}
	// The client's billing event never carries the advisor's earnings.
	for {
		b := cws.waitFor(realtime.EventSessionBilled, 5*time.Second)
		var billed map[string]any
		_ = json.Unmarshal(b.Data, &billed)
		if _, leak := billed["psychic_earnings"]; leak {
			t.Fatalf("client sees advisor earnings: %s", b.Data)
		}
		if m, _ := billed["minute"].(float64); m >= 4 {
			break
		}
	}
}
