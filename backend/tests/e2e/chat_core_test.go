package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// seqCollector records the seq of every message.created event a socket
// receives, in arrival order, draining the socket concurrently so a burst
// of events never backs up the server's send buffer.
type seqCollector struct {
	mu   sync.Mutex
	seqs []int64
	stop chan struct{}
	done chan struct{}
}

func collectSeqs(w *ws) *seqCollector {
	c := &seqCollector{stop: make(chan struct{}), done: make(chan struct{})}
	add := func(e realtime.Envelope) {
		if e.Type != realtime.EventMessageCreated {
			return
		}
		var p struct {
			Message service.MessageView `json:"message"`
		}
		if json.Unmarshal(e.Data, &p) == nil {
			c.mu.Lock()
			c.seqs = append(c.seqs, p.Message.Seq)
			c.mu.Unlock()
		}
	}
	w.mu.Lock()
	backlog := w.backlog
	w.backlog = nil
	w.mu.Unlock()
	for _, e := range backlog {
		add(e)
	}
	go func() {
		defer close(c.done)
		for {
			select {
			case e := <-w.events:
				add(e)
			case <-w.closed:
				return
			case <-c.stop:
				return
			}
		}
	}()
	return c
}

func (c *seqCollector) snapshot() []int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]int64(nil), c.seqs...)
}

func (c *seqCollector) close() {
	close(c.stop)
	<-c.done
}

// eventually polls cond until it holds or the timeout expires.
func eventually(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func convPath(conv uuid.UUID, rest string) string {
	return "/api/v1/chat/conversations/" + conv.String() + rest
}

// allMessages walks the whole transcript forwards through the since endpoint.
func allMessages(t *testing.T, c *client, conv uuid.UUID) []service.MessageView {
	t.Helper()
	var out []service.MessageView
	after := int64(0)
	for {
		var page service.SincePage
		c.must(http.StatusOK, http.MethodGet, convPath(conv, fmt.Sprintf("/messages/since?after_seq=%d&limit=500", after)), nil, &page)
		out = append(out, page.Items...)
		if !page.HasMore {
			return out
		}
		after = page.Items[len(page.Items)-1].Seq
	}
}

func inboxEntry(t *testing.T, c *client, conv uuid.UUID) service.ConversationView {
	t.Helper()
	var page service.InboxPage
	c.must(http.StatusOK, http.MethodGet, "/api/v1/chat/conversations", nil, &page)
	for _, v := range page.Items {
		if v.ID == conv {
			return v
		}
	}
	t.Fatalf("conversation %s not in inbox (%d items)", conv, len(page.Items))
	return service.ConversationView{}
}

// insertMessages stores messages seq from..to directly (bulk history),
// alternating senders, and moves the conversation's head.
func insertMessages(t *testing.T, e *env, s service.SessionView, clientID, psychicID uuid.UUID, from, to int64) {
	t.Helper()
	base := e.clock.Now()
	rows := make([]domain.Message, 0, to-from+1)
	for seq := from; seq <= to; seq++ {
		sender := clientID
		if seq%2 == 0 {
			sender = psychicID
		}
		at := base.Add(time.Duration(seq) * time.Millisecond)
		rows = append(rows, domain.Message{ID: domain.NewID(), ConversationID: s.ConversationID, SessionID: s.ID, Seq: seq,
			SenderID: &sender, Kind: domain.MessageText, Body: fmt.Sprintf("bulk %d", seq), CreatedAt: at, UpdatedAt: at})
	}
	if err := e.app.DB.CreateInBatches(rows, 1000).Error; err != nil {
		t.Fatalf("insert messages: %v", err)
	}
	last := rows[len(rows)-1]
	if err := e.app.DB.Model(&domain.Conversation{}).Where("id = ?", s.ConversationID).Updates(map[string]any{
		"last_seq": to, "last_message_id": last.ID, "last_message_at": last.CreatedAt, "last_activity_at": last.CreatedAt,
	}).Error; err != nil {
		t.Fatalf("move head: %v", err)
	}
}

func seqsOf(items []service.MessageView) []int64 {
	out := make([]int64, len(items))
	for i, m := range items {
		out[i] = m.Seq
	}
	return out
}

func TestChatSeqConcurrentSenders(t *testing.T) {
	e := newEnv(t)
	cli, psychic, cws, pws, s := startedSession(t, e, 5, 250)
	cSeqs, pSeqs := collectSeqs(cws), collectSeqs(pws)
	cli.ensureCSRF()
	psychic.ensureCSRF()

	// Both participants send 200 messages at the same time.
	const perSide = 200
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for side, sender := range []*client{cli, psychic} {
		wg.Add(1)
		go func(side int, c *client) {
			defer wg.Done()
			for i := 0; i < perSide; i++ {
				if err := postMessage(c, s.ID, fmt.Sprintf("side %d message %d", side, i), fmt.Sprintf("conc-%d-%03d", side, i)); err != nil {
					errs <- err
					return
				}
			}
		}(side, sender)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	// Seq 1 is the "session.started" notice; every message got the next
	// seq, without gaps, in commit order.
	const last = 1 + 2*perSide
	all := allMessages(t, cli, s.ConversationID)
	if len(all) != last {
		t.Fatalf("transcript has %d messages, want %d", len(all), last)
	}
	for i, m := range all {
		if m.Seq != int64(i+1) {
			t.Fatalf("message %d has seq %d", i, m.Seq)
		}
	}
	// Both sides received every message.created in seq order.
	for name, col := range map[string]*seqCollector{"client": cSeqs, "psychic": pSeqs} {
		eventually(t, 20*time.Second, name+" deliveries", func() bool {
			got := col.snapshot()
			return len(got) > 0 && got[len(got)-1] == last
		})
		got := col.snapshot()
		for i := 1; i < len(got); i++ {
			if got[i] != got[i-1]+1 {
				t.Fatalf("%s received seq %d after %d (gap, duplicate or reordering)", name, got[i], got[i-1])
			}
		}
		if got[0] > 2 || int64(len(got)) < last-1 {
			t.Fatalf("%s received %d events starting at %d", name, len(got), got[0])
		}
	}
	cSeqs.close()
	pSeqs.close()

	// Unread counters match the transcript: what the other side sent after
	// one's own last message (the notice counts for both).
	expect := func(me uuid.UUID) (readSeq, unread int64) {
		for _, m := range all {
			if m.SenderID != nil && *m.SenderID == me {
				readSeq = m.Seq
			}
		}
		for _, m := range all {
			if m.Seq > readSeq && (m.SenderID == nil || *m.SenderID != me) {
				unread++
			}
		}
		return readSeq, unread
	}
	for _, pc := range []*client{cli, psychic} {
		v := inboxEntry(t, pc, s.ConversationID)
		readSeq, unread := expect(pc.user.ID)
		if v.LastSeq != last || v.ReadSeq != readSeq || v.Unread != unread {
			t.Fatalf("%s inbox: last=%d read=%d unread=%d, want %d/%d/%d", pc.user.Role, v.LastSeq, v.ReadSeq, v.Unread, last, readSeq, unread)
		}
		if v.LastMessage == nil || v.LastMessage.Seq != last {
			t.Fatalf("last message preview: %+v", v.LastMessage)
		}
	}

	// Reading everything clears the counter and tells the other side. The
	// reader is whoever did not send the last message (its sender already
	// read up to it).
	read := convPath(s.ConversationID, "/read")
	reader, writer, writerWS := cli, psychic, pws
	if all[last-1].SenderID != nil && *all[last-1].SenderID == cli.user.ID {
		reader, writer, writerWS = psychic, cli, cws
	}
	var res service.ReadResult
	reader.must(http.StatusOK, http.MethodPost, read, map[string]any{"seq": last}, &res)
	if res.ReadSeq != last || res.Unread != 0 {
		t.Fatalf("read: %+v", res)
	}
	writerWS.waitMatch(realtime.EventMessageRead, fmt.Sprintf(`"read_seq":%d`, last), 5*time.Second)
	if v := inboxEntry(t, reader, s.ConversationID); v.Unread != 0 || v.ReadSeq != last {
		t.Fatalf("after read: %+v", v)
	}
	// The position never moves backwards and never passes the newest message.
	reader.must(http.StatusOK, http.MethodPost, read, map[string]any{"seq": 3}, &res)
	if res.ReadSeq != last {
		t.Fatalf("read moved backwards: %+v", res)
	}
	var other service.ReadResult
	writer.must(http.StatusOK, http.MethodPost, read, map[string]any{"seq": last + 50}, &other)
	if other.ReadSeq != last || other.Unread != 0 {
		t.Fatalf("read past the head: %+v", other)
	}
	for _, bad := range []map[string]any{{"seq": 1, "message_id": all[0].ID}, {}, {"seq": -1}, {"seq": "1"}, {"seq": 1, "extra": true}} {
		if r := cli.call(http.MethodPost, read, bad); r.status != http.StatusUnprocessableEntity && r.status != http.StatusBadRequest {
			t.Fatalf("read %v: %d %s", bad, r.status, r.body)
		}
	}
	// Someone else's conversation does not exist for a stranger.
	stranger := e.registerClient()
	for _, path := range []string{"/messages", "/messages/since?after_seq=0"} {
		if r := stranger.call(http.MethodGet, convPath(s.ConversationID, path), nil); r.status != http.StatusNotFound {
			t.Fatalf("stranger GET %s: %d", path, r.status)
		}
	}
	if r := stranger.call(http.MethodPost, read, map[string]any{"seq": 1}); r.status != http.StatusNotFound {
		t.Fatalf("stranger read: %d", r.status)
	}
}

func TestChatHistoryPagingBothDirections(t *testing.T) {
	e := newEnv(t)
	client, psychic, _, _, s := startedSession(t, e, 5, 0)
	const total = 10001 // the notice (seq 1) + 10 000 messages
	insertMessages(t, e, s, client.user.ID, psychic.user.ID, 2, total)

	// Backwards, 200 per page, with before_seq.
	seen := make(map[int64]bool, total)
	before, pages := int64(0), 0
	for {
		path := convPath(s.ConversationID, "/messages?limit=200")
		if before > 0 {
			path += fmt.Sprintf("&before_seq=%d", before)
		}
		var page service.MessagePage
		client.must(http.StatusOK, http.MethodGet, path, nil, &page)
		pages++
		for i, m := range page.Items {
			if seen[m.Seq] {
				t.Fatalf("seq %d returned twice", m.Seq)
			}
			seen[m.Seq] = true
			if i > 0 && m.Seq != page.Items[i-1].Seq+1 {
				t.Fatalf("page not chronological/contiguous at %d", m.Seq)
			}
			if before > 0 && m.Seq >= before {
				t.Fatalf("seq %d not older than %d", m.Seq, before)
			}
		}
		if !page.HasMore {
			if page.NextBeforeSeq != 0 || page.NextCursor != "" {
				t.Fatalf("last page advertises more: %+v", page.NextBeforeSeq)
			}
			break
		}
		if page.NextBeforeSeq != page.Items[0].Seq || page.NextCursor == "" {
			t.Fatalf("next position %d, oldest item %d", page.NextBeforeSeq, page.Items[0].Seq)
		}
		before = page.NextBeforeSeq
	}
	if len(seen) != total || pages != (total+199)/200 {
		t.Fatalf("backwards: %d messages in %d pages", len(seen), pages)
	}

	// Forwards, 500 per page, with after_seq.
	fwd := allMessages(t, client, s.ConversationID)
	if len(fwd) != total {
		t.Fatalf("forwards: %d messages", len(fwd))
	}
	for i, m := range fwd {
		if m.Seq != int64(i+1) {
			t.Fatalf("forwards item %d has seq %d", i, m.Seq)
		}
	}

	// The legacy opaque cursor walks exactly the same pages.
	var first, legacy, bySeq service.MessagePage
	client.must(http.StatusOK, http.MethodGet, convPath(s.ConversationID, "/messages?limit=200"), nil, &first)
	client.must(http.StatusOK, http.MethodGet, convPath(s.ConversationID, "/messages?limit=200&before="+url.QueryEscape(first.NextCursor)), nil, &legacy)
	client.must(http.StatusOK, http.MethodGet, convPath(s.ConversationID, fmt.Sprintf("/messages?limit=200&before_seq=%d", first.NextBeforeSeq)), nil, &bySeq)
	if fmt.Sprint(seqsOf(legacy.Items)) != fmt.Sprint(seqsOf(bySeq.Items)) || len(bySeq.Items) != 200 {
		t.Fatal("legacy cursor and before_seq disagree")
	}
}

func TestChatSinceAndHistoryAtExactPageBoundaries(t *testing.T) {
	e := newEnv(t)
	client, psychic, _, _, s := startedSession(t, e, 5, 0)
	insertMessages(t, e, s, client.user.ID, psychic.user.ID, 2, 1000) // exactly 1000 messages
	since := func(after int64, limit int) service.SincePage {
		var p service.SincePage
		client.must(http.StatusOK, http.MethodGet, convPath(s.ConversationID, fmt.Sprintf("/messages/since?after_seq=%d&limit=%d", after, limit)), nil, &p)
		return p
	}
	check := func(p service.SincePage, first, last int64, more bool) {
		t.Helper()
		if p.HasMore != more || p.LastSeq != 1000 || (len(p.Items) == 0) != (first == 0) {
			t.Fatalf("since: has_more=%v last_seq=%d items=%d", p.HasMore, p.LastSeq, len(p.Items))
		}
		if first > 0 && (p.Items[0].Seq != first || p.Items[len(p.Items)-1].Seq != last) {
			t.Fatalf("since range %d..%d, want %d..%d", p.Items[0].Seq, p.Items[len(p.Items)-1].Seq, first, last)
		}
	}
	check(since(0, 500), 1, 500, true)
	check(since(500, 500), 501, 1000, false) // a full last page is not followed by an empty one
	check(since(1000, 500), 0, 0, false)
	check(since(999, 1), 1000, 1000, false)
	check(since(998, 1), 999, 999, true)

	history := func(query string) service.MessagePage {
		var p service.MessagePage
		client.must(http.StatusOK, http.MethodGet, convPath(s.ConversationID, "/messages?"+query), nil, &p)
		return p
	}
	// History in pages of 200: 1000 messages are exactly five full pages.
	if p := history("limit=200"); !p.HasMore || p.Items[0].Seq != 801 || p.Items[199].Seq != 1000 || p.NextBeforeSeq != 801 {
		t.Fatalf("newest page: more=%v %d..%d next=%d", p.HasMore, p.Items[0].Seq, p.Items[len(p.Items)-1].Seq, p.NextBeforeSeq)
	}
	if p := history("limit=200&before_seq=201"); p.HasMore || p.Items[0].Seq != 1 || len(p.Items) != 200 || p.NextBeforeSeq != 0 {
		t.Fatalf("oldest (full) page: more=%v items=%d next=%d", p.HasMore, len(p.Items), p.NextBeforeSeq)
	}
	if p := history("limit=200&before_seq=1"); p.HasMore || len(p.Items) != 0 {
		t.Fatalf("before the first message: %+v", p)
	}
	for _, bad := range []string{"/messages/since?after_seq=-1", "/messages/since?after_seq=abc", "/messages/since",
		"/messages/since?after_seq=0&limit=501", "/messages?before_seq=0", "/messages?before_seq=x", "/messages?limit=201",
		"/messages?before=not-a-cursor"} {
		if r := client.call(http.MethodGet, convPath(s.ConversationID, bad), nil); r.status != http.StatusUnprocessableEntity {
			t.Fatalf("GET %s: %d %s", bad, r.status, r.body)
		}
	}
}

// expectSeq waits for the next message.created and checks its seq.
func expectSeq(t *testing.T, w *ws, want int64) {
	t.Helper()
	ev := w.waitFor(realtime.EventMessageCreated, 5*time.Second)
	var p struct {
		Message struct {
			Seq int64 `json:"seq"`
		} `json:"message"`
	}
	if err := json.Unmarshal(ev.Data, &p); err != nil || p.Message.Seq != want {
		t.Fatalf("message.created seq %d, want %d (%s)", p.Message.Seq, want, ev.Data)
	}
}

func TestChatOrderedDeliveryInjectedAndGapFlush(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	w := c.connect("")
	ctx := context.Background()
	conv := uuid.New() // a conversation gate of its own
	pub := func(seq int64) {
		t.Helper()
		if err := e.app.Events.PublishOrdered(ctx, conv, seq, realtime.EventMessageCreated,
			map[string]any{"message": map[string]any{"conversation_id": conv, "seq": seq}}, c.user.ID); err != nil {
			t.Fatalf("publish %d: %v", seq, err)
		}
	}
	pub(1)
	expectSeq(t, w, 1)
	pub(3) // committed after 2 but published first
	w.expectNone(realtime.EventMessageCreated, 300*time.Millisecond)
	pub(2)
	expectSeq(t, w, 2)
	expectSeq(t, w, 3)

	pub(6) // 4 and 5 were committed but their publication was lost
	w.expectNone(realtime.EventMessageCreated, 300*time.Millisecond)
	e.tick(time.Second) // the gap is younger than realtime.GapFlushAge: still held
	w.expectNone(realtime.EventMessageCreated, 300*time.Millisecond)
	e.tick(realtime.GapFlushAge)
	expectSeq(t, w, 6)
	pub(5) // too late: clients fetch it through the since endpoint
	w.expectNone(realtime.EventMessageCreated, 300*time.Millisecond)
	pub(7)
	expectSeq(t, w, 7)
}

func TestChatBillingBoundaryAgreement(t *testing.T) {
	e := newEnv(t)
	end := func(c *client, id uuid.UUID) service.SessionSummary {
		t.Helper()
		var sum service.SessionSummary
		c.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+id.String()+"/end", nil, &sum)
		return sum
	}
	get := func(c *client, id uuid.UUID) service.SessionView {
		t.Helper()
		var v service.SessionView
		c.must(http.StatusOK, http.MethodGet, "/api/v1/chat/sessions/"+id.String(), nil, &v)
		return v
	}

	// Exactly one minute after the start, the second minute has not started:
	// neither the worker nor the end flow charges it.
	c1, _, _, _, s1 := startedSession(t, e, 10, 250)
	e.tick(time.Minute)
	if v := get(c1, s1.ID); v.BilledMinutes != 1 {
		t.Fatalf("worker billed at the exact boundary: %d minutes", v.BilledMinutes)
	}
	if sum := end(c1, s1.ID); sum.Session.BilledMinutes != 1 || sum.DurationSeconds != 60 {
		t.Fatalf("end at the exact boundary: %d minutes, %ds", sum.Session.BilledMinutes, sum.DurationSeconds)
	}

	// One microsecond (the database precision) later it has started for both.
	c2, _, _, _, s2 := startedSession(t, e, 10, 250)
	e.clock.Advance(time.Minute + time.Microsecond)
	if sum := end(c2, s2.ID); sum.Session.BilledMinutes != 2 {
		t.Fatalf("end flow just past the boundary: %d minutes", sum.Session.BilledMinutes)
	}
	c3, _, _, _, s3 := startedSession(t, e, 10, 250)
	e.tick(time.Minute + time.Microsecond)
	if v := get(c3, s3.ID); v.BilledMinutes != 2 {
		t.Fatalf("worker just past the boundary: %d minutes", v.BilledMinutes)
	}
	if sum := end(c3, s3.ID); sum.Session.BilledMinutes != 2 {
		t.Fatalf("end after the worker billed: %d minutes (double charge?)", sum.Session.BilledMinutes)
	}
	// Below the database precision the outcome depends on the start's
	// sub-microsecond part, but the worker and the end flow always agree.
	c4, _, _, _, s4 := startedSession(t, e, 10, 250)
	e.tick(time.Minute + 999*time.Nanosecond)
	worker := get(c4, s4.ID).BilledMinutes
	if sum := end(c4, s4.ID); sum.Session.BilledMinutes != worker {
		t.Fatalf("sub-microsecond: worker %d minutes, end flow %d", worker, sum.Session.BilledMinutes)
	}
}

// closeCode reads until the server closes the connection and returns the
// close code (0 when it stays open).
func closeCode(conn *websocket.Conn, timeout time.Duration) int {
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			if ce, ok := err.(*websocket.CloseError); ok {
				return ce.Code
			}
			return 0
		}
	}
}

func TestChatGlobalConnectionCapAcrossInstances(t *testing.T) {
	nodes := newCluster(t, 2, func(c *config.Config) { c.Realtime.MaxConnsPerUser = 2 })
	a, b := nodes[0], nodes[1]
	u := a.registerClient()
	wa := u.connect("")
	u.on(b).connect("")

	// A third connection is refused although instance A holds only one.
	conn, _, err := u.dial("")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if code := closeCode(conn, 5*time.Second); code != realtime.CloseTooManyConns {
		t.Fatalf("over the cap across instances: close code %d", code)
	}
	// Closing one connection frees a slot on any instance.
	_ = wa.conn.Close()
	eventually(t, 5*time.Second, "freed slot", func() bool {
		conn, _, err := u.dial("")
		if err != nil {
			return false
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var env realtime.Envelope
		return conn.ReadJSON(&env) == nil && env.Type == realtime.EventHello
	})
}

func TestChatInstanceSubscribesOnlyConnectedUsers(t *testing.T) {
	nodes := newCluster(t, 2)
	a, b := nodes[0], nodes[1]
	ctx := context.Background()
	numSub := func(id uuid.UUID) int64 {
		ch := a.app.Config.Redis.KeyPrefix + "rt:u:" + id.String()
		res, err := a.app.Redis.PubSubNumSub(ctx, ch).Result()
		if err != nil {
			t.Fatalf("numsub: %v", err)
		}
		return res[ch]
	}
	alice, bob := a.registerClient(), a.registerClient()
	wa := alice.connect("")     // alice is connected to A only
	wb := bob.on(b).connect("") // bob to B only
	if numSub(alice.user.ID) != 1 || numSub(bob.user.ID) != 1 {
		t.Fatalf("one instance per user expected: alice=%d bob=%d", numSub(alice.user.ID), numSub(bob.user.ID))
	}
	if a.app.Broker.SubscribedUsers() != 1 || b.app.Broker.SubscribedUsers() != 1 {
		t.Fatalf("subscribed users: A=%d B=%d", a.app.Broker.SubscribedUsers(), b.app.Broker.SubscribedUsers())
	}
	// Events published on any instance still reach the user's instance.
	if err := b.app.Events.Publish(ctx, "test.ping", map[string]string{"to": "alice"}, alice.user.ID); err != nil {
		t.Fatal(err)
	}
	wa.waitMatch("test.ping", "alice", 5*time.Second)
	if err := a.app.Events.Publish(ctx, "test.ping", map[string]string{"to": "bob"}, bob.user.ID); err != nil {
		t.Fatal(err)
	}
	wb.waitMatch("test.ping", "bob", 5*time.Second)

	// A second connection of alice on B subscribes B as well; when both
	// close, no instance listens to alice any more.
	wa2 := alice.on(b).connect("")
	if numSub(alice.user.ID) != 2 {
		t.Fatalf("alice on both instances: %d", numSub(alice.user.ID))
	}
	_ = wa.conn.Close()
	_ = wa2.conn.Close()
	eventually(t, 5*time.Second, "unsubscribe", func() bool { return numSub(alice.user.ID) == 0 })
	eventually(t, 5*time.Second, "local refcounts", func() bool {
		return a.app.Broker.SubscribedUsers() == 0 && b.app.Broker.SubscribedUsers() == 1
	})
}

func TestChatAcrossInstances(t *testing.T) {
	nodes := newCluster(t, 2)
	a, b := nodes[0], nodes[1]
	a.ensurePacks()
	psychic := a.createPsychic(5)
	pb := psychic.on(b)
	pws := pb.goOnline() // the advisor is served by B
	client := a.registerClient()
	cws := client.connect("") // the client by A

	var req service.SessionView
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": psychic.slug(), "request_key": uniq("req-key-")}, &req)
	pws.waitFor(realtime.EventSessionRequested, 5*time.Second)
	cws.mustAck(realtime.FrameSessionJoin, map[string]any{"session_id": req.ID})
	pb.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+req.ID.String()+"/accept", nil, nil)
	pws.mustAck(realtime.FrameSessionJoin, map[string]any{"session_id": req.ID})
	cws.waitFor(realtime.EventSessionStarted, 5*time.Second)
	pws.waitFor(realtime.EventSessionStarted, 5*time.Second)

	cSeqs, pSeqs := collectSeqs(cws), collectSeqs(pws)
	defer cSeqs.close()
	defer pSeqs.close()
	client.ensureCSRF()
	pb.ensureCSRF()
	for i := 0; i < 20; i++ {
		sender := client
		if i%2 == 1 {
			sender = pb
		}
		if err := postMessage(sender, req.ID, fmt.Sprintf("hop %d", i), fmt.Sprintf("hop-%03d", i)); err != nil {
			t.Fatal(err)
		}
	}
	for name, col := range map[string]*seqCollector{"client on A": cSeqs, "advisor on B": pSeqs} {
		eventually(t, 10*time.Second, name, func() bool {
			got := col.snapshot()
			return len(got) > 0 && got[len(got)-1] == 21
		})
		got := col.snapshot()
		for i := 1; i < len(got); i++ {
			if got[i] != got[i-1]+1 {
				t.Fatalf("%s: seq %d after %d", name, got[i], got[i-1])
			}
		}
	}
}

func TestChatRedisOutageResync(t *testing.T) {
	mr := miniredis.RunT(t)
	e := newEnv(t, func(c *config.Config) {
		// Fail fast while Redis is down; rate limits fail open (default).
		c.Redis.URL = "redis://" + mr.Addr() + "/0?max_retries=-1"
		c.Redis.DialTimeout, c.Redis.ReadTimeout, c.Redis.WriteTimeout = 200*time.Millisecond, 500*time.Millisecond, 500*time.Millisecond
	})
	client, psychic, cws, pws, s := startedSession(t, e, 5, 250)
	head := allMessages(t, psychic, s.ConversationID)
	lastSeen := head[len(head)-1].Seq
	cws.drain()
	pws.drain()
	client.ensureCSRF()

	// Redis disappears mid-chat: the message is still stored (the database
	// is the source of truth) but its realtime delivery is lost.
	mr.Close()
	if err := postMessage(client, s.ID, "sent while redis was down", "outage-0001"); err != nil {
		t.Fatalf("send during the outage: %v", err)
	}
	if err := mr.Restart(); err != nil {
		t.Fatalf("restart: %v", err)
	}
	// Every connected client is told to resync, then fills the hole.
	pws.waitFor(realtime.EventResyncRequired, 20*time.Second)
	cws.waitFor(realtime.EventResyncRequired, 20*time.Second)
	var gap service.SincePage
	psychic.must(http.StatusOK, http.MethodGet, convPath(s.ConversationID, fmt.Sprintf("/messages/since?after_seq=%d", lastSeen)), nil, &gap)
	if len(gap.Items) != 1 || gap.Items[0].Body != "sent while redis was down" || gap.LastSeq != lastSeen+1 {
		t.Fatalf("gap fill: %+v", gap)
	}
	// Live delivery resumes. The lost publication left a gap in the ordering
	// gate, so the next message is held until the gap is flushed.
	if err := postMessage(client, s.ID, "back to normal", "outage-0002"); err != nil {
		t.Fatal(err)
	}
	e.tick(realtime.GapFlushAge + time.Second)
	pws.waitMatch(realtime.EventMessageCreated, "back to normal", 10*time.Second)
}

func TestChatLegacyParams(t *testing.T) {
	e := newEnv(t)
	client, psychic, _, pws, s := startedSession(t, e, 5, 0)
	insertMessages(t, e, s, client.user.ID, psychic.user.ID, 2, 120)
	pws.drain()

	var first, viaBefore, viaCursor, viaSeq service.MessagePage
	client.must(http.StatusOK, http.MethodGet, convPath(s.ConversationID, "/messages?limit=50"), nil, &first)
	esc := url.QueryEscape(first.NextCursor)
	client.must(http.StatusOK, http.MethodGet, convPath(s.ConversationID, "/messages?limit=50&before="+esc), nil, &viaBefore)
	client.must(http.StatusOK, http.MethodGet, convPath(s.ConversationID, "/messages?limit=50&cursor="+esc), nil, &viaCursor)
	client.must(http.StatusOK, http.MethodGet, convPath(s.ConversationID, fmt.Sprintf("/messages?limit=50&before_seq=%d", first.NextBeforeSeq)), nil, &viaSeq)
	want := fmt.Sprint(seqsOf(viaSeq.Items))
	if fmt.Sprint(seqsOf(viaBefore.Items)) != want || fmt.Sprint(seqsOf(viaCursor.Items)) != want || viaSeq.Items[0].Seq != 21 {
		t.Fatalf("legacy cursors: %v %v %v", seqsOf(viaBefore.Items)[:1], seqsOf(viaCursor.Items)[:1], seqsOf(viaSeq.Items)[:1])
	}
	// A cursor of another conversation is rejected.
	other, otherPsy, _, _, os := startedSession(t, e, 5, 0)
	insertMessages(t, e, os, other.user.ID, otherPsy.user.ID, 2, 80)
	var foreign service.MessagePage
	other.must(http.StatusOK, http.MethodGet, convPath(os.ConversationID, "/messages?limit=10"), nil, &foreign)
	if r := client.call(http.MethodGet, convPath(s.ConversationID, "/messages?before="+url.QueryEscape(foreign.NextCursor)), nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("foreign cursor: %d", r.status)
	}

	// Legacy timestamp gap filling (?after=RFC3339).
	mid := first.Items[0] // seq 71
	var legacy service.SincePage
	client.must(http.StatusOK, http.MethodGet, convPath(s.ConversationID, "/messages/since?after="+url.QueryEscape(mid.CreatedAt.Format(time.RFC3339Nano))), nil, &legacy)
	if len(legacy.Items) != 120-71 || legacy.Items[0].Seq != 72 {
		t.Fatalf("legacy since: %d items from %d", len(legacy.Items), legacy.Items[0].Seq)
	}
	// Legacy read by message id now answers with the read state, and the
	// receipt still carries message_id/read_at for older clients.
	var res service.ReadResult
	client.must(http.StatusOK, http.MethodPost, convPath(s.ConversationID, "/read"), map[string]any{"message_id": mid.ID}, &res)
	// Unread after 71: the advisor's messages among 72..120 (even seqs).
	if res.ReadSeq != 71 || res.Unread != 25 {
		t.Fatalf("read by id: %+v", res)
	}
	receipt := pws.waitFor(realtime.EventMessageRead, 5*time.Second)
	for _, want := range []string{`"read_seq":71`, `"message_id":"` + mid.ID.String() + `"`, `"read_at":`} {
		if !strings.Contains(string(receipt.Data), want) {
			t.Fatalf("receipt %s lacks %s", receipt.Data, want)
		}
	}
	// A message of another conversation cannot be used as a read position.
	if r := client.call(http.MethodPost, convPath(s.ConversationID, "/read"), map[string]any{"message_id": foreign.Items[0].ID}); r.status != http.StatusNotFound {
		t.Fatalf("foreign message read: %d", r.status)
	}
}

func TestChatReadFrameBySeqAndInboxPaging(t *testing.T) {
	e := newEnv(t)
	client, psychic, cws, pws, s := startedSession(t, e, 5, 0)

	// The realtime read frame accepts a seq as well as a message id.
	ack := pws.mustAck(realtime.FrameMessageSend, map[string]any{"session_id": s.ID, "body": "Your cards are ready.", "client_msg_id": "frame-0001"})
	var sent struct {
		Message service.MessageView `json:"message"`
	}
	_ = json.Unmarshal(ack, &sent)
	cws.waitFor(realtime.EventMessageCreated, 5*time.Second)
	var read struct {
		Read    bool  `json:"read"`
		ReadSeq int64 `json:"read_seq"`
		Unread  int64 `json:"unread"`
	}
	_ = json.Unmarshal(cws.mustAck(realtime.FrameMessageRead, map[string]any{"conversation_id": s.ConversationID, "seq": sent.Message.Seq}), &read)
	if !read.Read || read.ReadSeq != sent.Message.Seq || read.Unread != 0 {
		t.Fatalf("read frame: %+v", read)
	}
	pws.waitMatch(realtime.EventMessageRead, fmt.Sprintf(`"read_seq":%d`, sent.Message.Seq), 5*time.Second)
	if e := cws.request(realtime.FrameMessageRead, map[string]any{"conversation_id": s.ConversationID}); e.Err == nil || e.Err.Code != "validation_failed" {
		t.Fatalf("read frame without a position: %+v", e)
	}

	// The advisor's inbox pages by last activity with a cursor.
	now := e.clock.Now()
	var others []uuid.UUID
	for i := 1; i <= 3; i++ {
		c := e.registerClient()
		conv := domain.Conversation{ID: domain.NewID(), ClientID: c.user.ID, PsychicID: psychic.user.ID,
			LastActivityAt: now.Add(time.Duration(i) * time.Hour), CreatedAt: now, UpdatedAt: now}
		if err := e.app.DB.Create(&conv).Error; err != nil {
			t.Fatal(err)
		}
		others = append(others, conv.ID)
	}
	var page1, page2 service.InboxPage
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/chat/conversations?limit=2", nil, &page1)
	if len(page1.Items) != 2 || !page1.HasMore || page1.NextCursor == "" || page1.Items[0].ID != others[2] || page1.Items[1].ID != others[1] {
		t.Fatalf("page 1: %d items, more=%v", len(page1.Items), page1.HasMore)
	}
	// Four conversations fill exactly two pages: the last page is full, has
	// no cursor, and no empty trailing page is announced.
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/chat/conversations?limit=2&cursor="+url.QueryEscape(page1.NextCursor), nil, &page2)
	if len(page2.Items) != 2 || page2.HasMore || page2.NextCursor != "" || page2.Items[0].ID != others[0] || page2.Items[1].ID != s.ConversationID {
		t.Fatalf("page 2: %d items, more=%v cursor=%q", len(page2.Items), page2.HasMore, page2.NextCursor)
	}
	if v := page2.Items[1]; v.Counterpart.ID != client.user.ID || v.OpenSession == nil || v.LastMessage == nil || v.LastMessage.Seq != sent.Message.Seq {
		t.Fatalf("live conversation row: %+v", v)
	}
	// The client sees the advisor with a live status and no unread message.
	var mine service.InboxPage
	client.must(http.StatusOK, http.MethodGet, "/api/v1/chat/conversations", nil, &mine)
	if len(mine.Items) != 1 || mine.Items[0].Counterpart.Slug != psychic.slug() || mine.Items[0].Counterpart.Status != "busy" || mine.Items[0].Unread != 0 {
		t.Fatalf("client inbox: %+v", mine.Items)
	}
	for _, bad := range []string{"?limit=51", "?limit=0", "?cursor=bogus"} {
		if r := psychic.call(http.MethodGet, "/api/v1/chat/conversations"+bad, nil); r.status != http.StatusUnprocessableEntity {
			t.Fatalf("inbox %s: %d", bad, r.status)
		}
	}
}

func TestChatMessageRateLimitAndTypingThrottle(t *testing.T) {
	e := newEnv(t, func(c *config.Config) {
		c.RateLimit.Scale = 1
		c.Chat.MessageRate, c.Chat.MessageRateEvery = 3, time.Minute
	})
	client, _, cws, pws, s := startedSession(t, e, 5, 0)
	send := func(body, id string) response {
		return client.call(http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/messages", map[string]any{"body": body, "client_msg_id": id})
	}
	for i := 0; i < 3; i++ {
		if r := send(fmt.Sprintf("burst %d", i), fmt.Sprintf("burst-%d", i)); r.status != http.StatusCreated {
			t.Fatalf("message %d: %d %s", i, r.status, r.body)
		}
	}
	r := send("one too many", "burst-3")
	if r.status != http.StatusTooManyRequests || r.errCode(t) != "rate_limited" {
		t.Fatalf("4th message within the window: %d %s", r.status, r.body)
	}
	// Retrying an already stored message is idempotent, not rate limited.
	if r := send("burst 0", "burst-0"); r.status != http.StatusCreated {
		t.Fatalf("idempotent retry: %d %s", r.status, r.body)
	}

	// A burst of typing indicators is throttled server-side…
	pws.drain()
	for i := 0; i < 6; i++ {
		cws.mustAck(realtime.FrameTyping, map[string]any{"session_id": s.ID, "typing": true})
	}
	relayed := 0
	deadline := time.After(700 * time.Millisecond)
	for done := false; !done; {
		ev, ok := pws.next(deadline)
		switch {
		case !ok:
			done = true
		case ev.Type == realtime.EventTyping:
			relayed++
		}
	}
	if relayed < 1 || relayed > 2 {
		t.Fatalf("relayed %d of 6 typing indicators, want 1–2", relayed)
	}
	// …but "stopped typing" always goes through.
	cws.mustAck(realtime.FrameTyping, map[string]any{"session_id": s.ID, "typing": false})
	pws.waitMatch(realtime.EventTyping, `"typing":false`, 5*time.Second)
}
