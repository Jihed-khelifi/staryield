package e2e

// revisions 6.0 (docs/platform-extensions.md §26.1): the free-trial hold.
//
// A reading whose next minute cannot be paid is held instead of ending: the
// advisor keeps writing, the client sees the replies locked, a top-up in time
// continues the reading and reveals them, otherwise the reading ends with
// hold_expired and the replies stay hidden until the client pays one minute.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// holdRate is the advisors' rate in these tests; the 40-credit test pack pays
// exactly one minute.
const holdRate = 40

// paidRate is the advisor's rate where a 250-credit pack pays exactly one
// minute (the 40-credit test pack is the sandbox's failing amount).
const paidRate = 250

const trialHoldPath = "/api/v1/admin/site/trial-hold"

// holdFrame is the payload of session.held and session.hold_ended.
type holdFrame struct {
	Session service.SessionView `json:"session"`
	Hold    service.HoldView    `json:"hold"`
}

func holdFrom(t *testing.T, data json.RawMessage) holdFrame {
	t.Helper()
	var f holdFrame
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("decode hold frame: %v (%s)", err, data)
	}
	return f
}

// revealedFrame is the payload of messages.revealed (a client gets the
// messages, an advisor their ids).
type revealedFrame struct {
	ConversationID uuid.UUID             `json:"conversation_id"`
	SessionID      uuid.UUID             `json:"session_id"`
	HoldID         uuid.UUID             `json:"hold_id"`
	Messages       []service.MessageView `json:"messages"`
	MessageIDs     []uuid.UUID           `json:"message_ids"`
}

func revealedFrom(t *testing.T, data json.RawMessage) revealedFrame {
	t.Helper()
	var f revealedFrame
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("decode messages.revealed: %v (%s)", err, data)
	}
	return f
}

// pinTrialHold sets the hold settings for the test (restored at its end; the
// database is shared by the whole suite). It goes through the service, so the
// cached site configuration is dropped at once.
func (e *env) pinTrialHold(afterTrial, afterPaid bool, holdSeconds, graceSeconds int) {
	e.t.Helper()
	ctx := context.Background()
	cur, err := e.app.Services.Site.AdminTrialHold(ctx)
	if err != nil {
		e.t.Fatal(err)
	}
	if _, err := e.app.Services.Site.UpdateTrialHold(ctx, service.TrialHoldInput{AfterTrial: &afterTrial, AfterPaid: &afterPaid,
		HoldSeconds: &holdSeconds, PaymentGraceSeconds: &graceSeconds, Version: &cur.Version}); err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() {
		e.app.DB.Exec(`UPDATE site_settings SET value = '{"after_trial": true, "after_paid": false, "hold_seconds": 60, "payment_grace_seconds": 120}'::jsonb,
			version = 1 WHERE key = 'trial_hold'`)
	})
}

// heldReading is a reading whose free minute ran out: held, both parties connected.
type heldReading struct {
	e               *env
	client, advisor *client
	cws, pws        *ws
	started         service.SessionView
	clientHeld      holdFrame // session.held as the client got it
	advisorHeld     holdFrame
}

// newHeldReading starts a reading for a client with one free minute and no
// credits, and runs the minute out.
func newHeldReading(t *testing.T, e *env) *heldReading {
	t.Helper()
	e.pinSignupFreeMinutes(1)
	cl, adv, cws, pws, s := startedSession(t, e, holdRate, 0)
	if s.FreeMinutesUsed != 1 || s.BilledMinutes != 1 {
		t.Fatalf("started: %+v", s)
	}
	e.tick(time.Minute + time.Second)
	h := &heldReading{e: e, client: cl, advisor: adv, cws: cws, pws: pws, started: s}
	h.clientHeld = holdFrom(t, cws.waitFor(realtime.EventSessionHeld, 5*time.Second).Data)
	h.advisorHeld = holdFrom(t, pws.waitFor(realtime.EventSessionHeld, 5*time.Second).Data)
	// What came before the hold (start notice, billing of the free minute) is of no interest.
	cws.drain()
	pws.drain()
	return h
}

func (h *heldReading) conv() uuid.UUID { return h.started.ConversationID }
func (h *heldReading) sid() uuid.UUID  { return h.started.ID }

func (h *heldReading) session(c *client) service.SessionView {
	h.e.t.Helper()
	var sv service.SessionView
	c.must(http.StatusOK, http.MethodGet, "/api/v1/chat/sessions/"+h.sid().String(), nil, &sv)
	return sv
}

// holdRow reads the hold row straight from the database.
func (e *env) holdRow(id uuid.UUID) domain.ChatHold {
	e.t.Helper()
	var hold domain.ChatHold
	if err := e.app.DB.First(&hold, "id = ?", id).Error; err != nil {
		e.t.Fatalf("hold %s: %v", id, err)
	}
	return hold
}

func (e *env) sessionRow(id uuid.UUID) domain.Session {
	e.t.Helper()
	var s domain.Session
	if err := e.app.DB.First(&s, "id = ?", id).Error; err != nil {
		e.t.Fatalf("session %s: %v", id, err)
	}
	return s
}

func (e *env) count(query string, args ...any) int64 {
	e.t.Helper()
	var n int64
	if err := e.app.DB.Raw(query, args...).Scan(&n).Error; err != nil {
		e.t.Fatal(err)
	}
	return n
}

// grantWithoutHook adds credits to a wallet through the wallet service alone:
// no purchase observer or admin hook runs, so a held reading stays held.
func (e *env) grantWithoutHook(userID uuid.UUID, credits int64) {
	e.t.Helper()
	err := database.NewTxManager(e.app.DB).WithinTx(context.Background(), func(ctx context.Context) error {
		_, _, err := e.app.Services.Wallet.Apply(ctx, service.Movement{UserID: userID, Type: domain.LedgerAdminAdjustment, Amount: credits,
			ReferenceType: "admin", IdempotencyKey: "test-grant:" + uuid.NewString(), Description: "test",
			Lot: &service.LotSpec{Source: domain.LotBonus}})
		return err
	})
	if err != nil {
		e.t.Fatal(err)
	}
}

// advisorSay sends a text message as the advisor and returns the answer.
func (h *heldReading) advisorSay(body string) service.MessageView {
	h.e.t.Helper()
	return h.advisor.mustSay(h.conv(), body, "")
}

func postKey(c *client, path string) response {
	return c.call(http.MethodPost, path, map[string]any{"idempotency_key": uniq("hold-key-")})
}

func continuePath(sid uuid.UUID) string {
	return "/api/v1/chat/sessions/" + sid.String() + "/hold/continue"
}

func revealPath(hold uuid.UUID) string { return "/api/v1/chat/holds/" + hold.String() + "/reveal" }

// waitEvent waits for the analytics event name whose property key is value
// (events are flushed in the background).
func (e *env) waitEvent(name, key, value string) {
	e.t.Helper()
	eventually(e.t, 5*time.Second, "analytics event "+name, func() bool {
		var n int64
		e.app.DB.Raw(`SELECT count(*) FROM analytics_events WHERE name = ? AND properties ->> ? = ?`, name, key, value).Scan(&n)
		return n > 0
	})
}

// kinds lists the types of the events queued on a socket, in order.
func kinds(events []realtime.Envelope) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.Type
	}
	return out
}

// ---------------------------------------------------------------------------

// The free minute runs out: the reading is held, not ended, and hold time is
// neither billed nor counted as active time.
func TestHoldEntersWhenTheTrialEnds(t *testing.T) {
	e := newEnv(t)
	h := newHeldReading(t, e)

	for who, f := range map[string]holdFrame{"client": h.clientHeld, "advisor": h.advisorHeld} {
		if f.Session.Status != "held" || f.Session.ID != h.sid() || f.Session.Hold == nil || f.Session.Hold.ID != f.Hold.ID {
			t.Fatalf("%s: session in session.held: %+v", who, f.Session)
		}
		if f.Hold.Status != "open" || f.Hold.Reason != "trial_ended" || f.Hold.SessionID != h.sid() || f.Hold.ConversationID != h.conv() ||
			f.Hold.HeldMessages != 0 || f.Hold.ResolvedAt != nil || f.Hold.RevealedAt != nil {
			t.Fatalf("%s: hold: %+v", who, f.Hold)
		}
		if got := f.Hold.ExpiresAt.Sub(f.Hold.StartedAt); got != 60*time.Second {
			t.Fatalf("%s: the hold lasts %s, want the seeded 60 s", who, got)
		}
		if f.Hold.PriceCredits != holdRate {
			t.Fatalf("%s: price %d", who, f.Hold.PriceCredits)
		}
	}

	sv := h.session(h.client)
	if sv.Status != "held" || sv.Hold == nil || sv.Hold.Status != "open" || sv.NextBillingAt != nil || sv.BilledMinutes != 1 ||
		sv.FreeMinutesUsed != 1 || *sv.CreditsCharged != 0 || sv.ActiveSeconds != 60 {
		t.Fatalf("held session: %+v", sv)
	}
	row := e.sessionRow(h.sid())
	if row.HoldExpiresAt == nil || row.NextBillingAt != nil || row.LastResumedAt != nil || row.LastHoldID == nil || *row.LastHoldID != h.clientHeld.Hold.ID {
		t.Fatalf("session row: %+v", row)
	}

	// Hold time is never billed and never counts as active time.
	e.tick(20 * time.Second)
	e.tick(20 * time.Second)
	sv = h.session(h.client)
	if sv.Status != "held" || sv.BilledMinutes != 1 || sv.ActiveSeconds != 60 || *sv.CreditsCharged != 0 {
		t.Fatalf("after 40 s of hold: %+v", sv)
	}
	if w := h.client.wallet(); w.Balance != 0 || w.FreeMinutes != 0 {
		t.Fatalf("wallet: %+v", w)
	}

	// A held reading is still open: the client cannot start another, the advisor stays busy.
	r := h.client.call(http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": h.advisor.slug(), "request_key": uniq("req-key-")})
	if r.status != http.StatusConflict || r.errCode(t) != "session_already_open" {
		t.Fatalf("second reading while held: %d %s", r.status, r.body)
	}
	other := e.registerClient()
	r = other.call(http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": h.advisor.slug(), "request_key": uniq("req-key-")})
	if r.status != http.StatusConflict {
		t.Fatalf("another client while the advisor is held: %d %s", r.status, r.body)
	}
	if e.count(`SELECT count(*) FROM chat_session_holds WHERE session_id = ?`, h.sid()) != 1 {
		t.Fatal("exactly one hold row")
	}
	e.waitEvent("chat.hold_started", "session_id", h.sid().String())
}

// Leaving the on-screen conversation sends the participant event but neither
// changes the hold nor stops its timer (BR-05).
func TestHoldPresenceLossDoesNotTouchTheHold(t *testing.T) {
	e := newEnv(t)
	h := newHeldReading(t, e)
	before := e.sessionRow(h.sid())

	h.cws.mustAck(realtime.FrameSessionLeave, map[string]any{"session_id": h.sid()})
	h.pws.waitMatch(realtime.EventParticipant, `"present":false`, 5*time.Second)
	_ = h.pws.conn.Close()
	time.Sleep(300 * time.Millisecond)
	if err := e.app.Services.Chat.SweepPresence(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
	after := e.sessionRow(h.sid())
	if after.Status != domain.SessionHeld || !after.HoldExpiresAt.Equal(*before.HoldExpiresAt) || after.DisconnectedAt != nil {
		t.Fatalf("presence loss changed the hold: %+v", after)
	}
	// The timer keeps running: the hold still expires on time.
	e.tick(61 * time.Second)
	if got := e.sessionRow(h.sid()); got.EndReason != domain.EndHoldExpired {
		t.Fatalf("after the hold time: %+v", got)
	}
}

// Every path that returns message content to the client applies the lock; the
// advisor and the admin see everything, flagged.
func TestHeldRepliesAreLockedOnEveryPath(t *testing.T) {
	e := newEnv(t, roomy)
	h := newHeldReading(t, e)
	// A second connection of the client collects the message events in order.
	cc := collectSeqs(h.client.connect(""))
	defer cc.close()

	const secret = "your third house holds the answer"
	clientNote := h.client.mustSay(h.conv(), "can you tell me more?", "")
	h.cws.waitFor(realtime.EventMessageCreated, 5*time.Second)
	h.pws.waitFor(realtime.EventMessageCreated, 5*time.Second)

	// The advisor's reply: full for the advisor, flagged held.
	held := h.advisorSay(secret)
	if !held.Held || held.Body != secret || held.Locked || held.HoldID == nil || *held.HoldID != h.clientHeld.Hold.ID {
		t.Fatalf("the advisor's answer: %+v", held)
	}
	// Realtime, same seq slot: the client gets the locked view, the advisor the full one.
	cev := messageEvent(t, h.cws.waitFor(realtime.EventMessageCreated, 5*time.Second).Data)
	pev := messageEvent(t, h.pws.waitFor(realtime.EventMessageCreated, 5*time.Second).Data)
	if !cev.Locked || cev.Held || cev.Body != "" || cev.BodyLength != len(secret) || len(cev.Attachments) != 0 || cev.ReplyTo != nil || len(cev.LikedBy) != 0 ||
		cev.Seq != held.Seq || cev.ID != held.ID || cev.HoldID == nil {
		t.Fatalf("the client's message.created: %+v", cev)
	}
	if !pev.Held || pev.Locked || pev.Body != secret || pev.Seq != held.Seq {
		t.Fatalf("the advisor's message.created: %+v", pev)
	}
	if raw, _ := json.Marshal(cev); strings.Contains(string(raw), "third house") {
		t.Fatalf("the secret reached the client: %s", raw)
	}

	// Inbox preview of the client: this reply is the last message.
	if lm := inboxEntry(t, h.client, h.conv()).LastMessage; lm == nil || !lm.Locked || lm.Body != "" || lm.BodyLength != len(secret) {
		t.Fatalf("client inbox preview: %+v", lm)
	}
	if lm := inboxEntry(t, h.advisor, h.conv()).LastMessage; lm == nil || lm.Body != secret || !lm.Held {
		t.Fatalf("advisor inbox preview: %+v", lm)
	}

	// An image: attachments are left out for the client and the bytes are "not found".
	r := h.advisor.sendConvImage(h.conv(), pngBytes(t, 8, 8), map[string]string{"caption": "the spread"})
	if r.status != http.StatusCreated {
		t.Fatalf("advisor image: %d %s", r.status, r.body)
	}
	var img sendOut
	r.decode(t, &img)
	if !img.Message.Held || len(img.Message.Attachments) != 1 {
		t.Fatalf("advisor's image answer: %+v", img.Message)
	}
	imgEv := messageEvent(t, h.cws.waitMatch(realtime.EventMessageCreated, img.Message.ID.String(), 5*time.Second).Data)
	if !imgEv.Locked || len(imgEv.Attachments) != 0 || imgEv.Body != "" {
		t.Fatalf("client's image event: %+v", imgEv)
	}
	attPath := "/api/v1/chat/attachments/" + img.Message.Attachments[0].ID.String()
	if r := h.client.call(http.MethodGet, attPath, nil); r.status != http.StatusNotFound {
		t.Fatalf("client fetching a held image: %d", r.status)
	}
	if r := h.advisor.call(http.MethodGet, attPath, nil); r.status != http.StatusOK {
		t.Fatalf("advisor fetching their image: %d", r.status)
	}

	// Timeline pages (before) and gap filling (since), for both sides.
	var page service.MessagePage
	h.client.must(http.StatusOK, http.MethodGet, convPath(h.conv(), "/messages?limit=50"), nil, &page)
	lockedSeen := 0
	for _, m := range page.Items {
		if m.Locked {
			lockedSeen++
			if m.Body != "" || len(m.Attachments) != 0 || m.BodyLength == 0 && m.Kind == "text" {
				t.Fatalf("timeline: %+v", m)
			}
		}
		if strings.Contains(m.Body, "third house") {
			t.Fatalf("timeline leaked: %+v", m)
		}
	}
	if lockedSeen != 2 {
		t.Fatalf("locked messages in the client's timeline: %d, want 2", lockedSeen)
	}
	for _, m := range allMessages(t, h.client, h.conv()) {
		if m.Held || (m.Locked && m.Body != "") {
			t.Fatalf("since (client): %+v", m)
		}
	}
	advisorSees := 0
	for _, m := range allMessages(t, h.advisor, h.conv()) {
		if m.Held && m.Body != "" {
			advisorSees++
		}
		if m.Locked {
			t.Fatalf("the advisor got a locked view: %+v", m)
		}
	}
	if advisorSees != 2 {
		t.Fatalf("held messages in the advisor's timeline: %d", advisorSees)
	}

	// Replying to a locked message is allowed; its quote stays locked for the client and is full for the advisor.
	reply := h.client.mustSay(h.conv(), "thank you, I'll top up", held.ID.String())
	if reply.ReplyTo == nil || !reply.ReplyTo.Locked || reply.ReplyTo.Excerpt != "" {
		t.Fatalf("the client's own reply quoting a locked message: %+v", reply.ReplyTo)
	}
	advEv := messageEvent(t, h.pws.waitMatch(realtime.EventMessageCreated, reply.ID.String(), 5*time.Second).Data)
	if advEv.ReplyTo == nil || advEv.ReplyTo.Locked || !strings.Contains(advEv.ReplyTo.Excerpt, "third house") {
		t.Fatalf("the advisor's view of that reply: %+v", advEv.ReplyTo)
	}
	for _, m := range allMessages(t, h.client, h.conv()) {
		if m.ID == reply.ID && (m.ReplyTo == nil || !m.ReplyTo.Locked || m.ReplyTo.Excerpt != "") {
			t.Fatalf("quote in the client's timeline: %+v", m.ReplyTo)
		}
	}

	// A client cannot like or save a locked message; the advisor can like their own.
	like := h.client.call(http.MethodPut, "/api/v1/chat/messages/"+held.ID.String()+"/like", nil)
	if like.status != http.StatusConflict || like.errCode(t) != "message_locked" {
		t.Fatalf("like: %d %s", like.status, like.body)
	}
	save := h.client.call(http.MethodPut, "/api/v1/chat/saved/"+held.ID.String(), nil)
	if save.status != http.StatusConflict || save.errCode(t) != "message_locked" {
		t.Fatalf("save: %d %s", save.status, save.body)
	}
	var saved service.Page[service.MessageView]
	h.client.must(http.StatusOK, http.MethodGet, "/api/v1/chat/saved", nil, &saved)
	if len(saved.Items) != 0 {
		t.Fatalf("saved list: %+v", saved.Items)
	}
	if e.likeRows(h.conv()) != 0 {
		t.Fatal("a like was stored for a locked message")
	}

	// The admin transcript flags held messages, never locks them.
	admin := e.createAdmin()
	admin.stepUp()
	var transcript service.Page[service.MessageView]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/chat/sessions/"+h.sid().String()+"/transcript?reason=hold+review", nil, &transcript)
	flagged := 0
	for _, m := range transcript.Items {
		if m.Held && m.Body != "" && !m.Locked {
			flagged++
		}
	}
	if flagged != 2 {
		t.Fatalf("transcript: %d held messages flagged, want 2 (%+v)", flagged, transcript.Items)
	}
	var adminSessions service.Page[service.SessionView]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/chat/sessions?status=held", nil, &adminSessions)
	var seenHeld bool
	for _, sv := range adminSessions.Items {
		if sv.ID == h.sid() {
			seenHeld = sv.Status == "held" && sv.Hold != nil && sv.Hold.Status == "open"
		}
	}
	if !seenHeld {
		t.Fatalf("admin session list lacks the held reading with its hold: %+v", adminSessions.Items)
	}

	// The hold counts the messages written under it.
	if hold := e.holdRow(h.clientHeld.Hold.ID); hold.HeldMessages != 2 {
		t.Fatalf("held_messages: %d", hold.HeldMessages)
	}
	if got := h.session(h.advisor).Hold; got == nil || got.HeldMessages != 2 {
		t.Fatalf("advisor's session view hold: %+v", got)
	}

	// Seq order on the client's stream: every seq once, ascending, held messages included.
	eventually(t, 3*time.Second, "ordered delivery", func() bool { return len(cc.snapshot()) >= 4 })
	seqs := cc.snapshot()
	for i := 1; i < len(seqs); i++ {
		if seqs[i] != seqs[i-1]+1 {
			t.Fatalf("the client's stream is out of order or has a gap: %v", seqs)
		}
	}
	_ = clientNote // the client's own message is part of the ordered stream above
	e.invariant(h.client.user.ID)
}

// A reply the client cannot read yet goes out as "top up to read it", never as text.
func TestHeldReplyPushNeverCarriesTheText(t *testing.T) {
	e := newEnv(t, withPush(t), roomy)
	h := newHeldReading(t, e)
	endpoint := "https://fcm.googleapis.com/fcm/send/" + uniq("cli")
	h.client.must(http.StatusCreated, http.MethodPost, "/api/v1/me/push-subscriptions", browserSubscription(t, endpoint), nil)
	e.setPrefs(h.client.user.ID, map[string]any{"notify_push": true})
	_ = h.cws.conn.Close() // offline: only a push can reach the client
	time.Sleep(300 * time.Millisecond)

	const secret = "beware of the stranger in March"
	h.advisorSay(secret)
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
		raw, _ := json.Marshal(p.Payload)
		if strings.Contains(string(raw), "stranger") {
			t.Fatalf("the push carries the text: %s", raw)
		}
		if p.Payload.Kind == "chat.message" && !strings.Contains(p.Payload.Body, "Top up to read it.") {
			t.Fatalf("the push body: %q", p.Payload.Body)
		}
	}
}

// A purchase that completes during the hold converts it: one minute is
// charged, the replies are revealed, the reading goes on.
func TestHoldConvertsWhenAPurchaseSucceeds(t *testing.T) {
	e := newEnv(t, roomy)
	h := newHeldReading(t, e)
	first := h.advisorSay("the cards say yes")
	second := h.advisorSay("and the moon agrees")
	h.cws.drain()
	h.pws.drain()

	wallet := h.client.buyCredits(250)
	// The wallet in the purchase answer is already after the conversion.
	if wallet.Balance != 250-holdRate {
		t.Fatalf("wallet after the top-up: %+v", wallet)
	}

	events := h.cws.pending()
	order := kinds(events)
	for _, want := range []string{realtime.EventSessionHoldEnded, realtime.EventMessagesRevealed, realtime.EventSessionBilled,
		realtime.EventWalletUpdated, realtime.EventSessionResumed} {
		if indexOf(order, want) < 0 {
			t.Fatalf("client events %v lack %s", order, want)
		}
	}
	if !(indexOf(order, realtime.EventSessionHoldEnded) < indexOf(order, realtime.EventMessagesRevealed) &&
		indexOf(order, realtime.EventMessagesRevealed) < indexOf(order, realtime.EventSessionBilled) &&
		indexOf(order, realtime.EventSessionBilled) < indexOf(order, realtime.EventSessionResumed)) {
		t.Fatalf("event order: %v", order)
	}
	for _, ev := range events {
		switch ev.Type {
		case realtime.EventSessionHoldEnded:
			f := holdFrom(t, ev.Data)
			if f.Hold.Status != "converted" || f.Hold.ResolvedAt == nil || f.Session.Status != "active" || f.Session.BilledMinutes != 2 || f.Hold.HeldMessages != 2 {
				t.Fatalf("hold_ended: %+v", f)
			}
		case realtime.EventMessagesRevealed:
			rf := revealedFrom(t, ev.Data)
			if rf.HoldID != h.clientHeld.Hold.ID || rf.SessionID != h.sid() || rf.ConversationID != h.conv() || len(rf.Messages) != 2 {
				t.Fatalf("messages.revealed (client): %+v", rf)
			}
			if rf.Messages[0].ID != first.ID || rf.Messages[0].Body != "the cards say yes" || rf.Messages[1].ID != second.ID ||
				rf.Messages[1].Body != "and the moon agrees" || rf.Messages[0].Locked || rf.Messages[1].Held {
				t.Fatalf("revealed messages: %+v", rf.Messages)
			}
		case realtime.EventSessionBilled:
			var b struct {
				Minute  int  `json:"minute"`
				Free    bool `json:"free"`
				Charged int  `json:"charged"`
			}
			_ = json.Unmarshal(ev.Data, &b)
			if b.Minute != 2 || b.Free || b.Charged != holdRate {
				t.Fatalf("session.billed: %s", ev.Data)
			}
		}
	}
	padvisor := kinds(h.pws.pending())
	for _, want := range []string{realtime.EventSessionHoldEnded, realtime.EventMessagesRevealed, realtime.EventSessionBilled, realtime.EventSessionResumed} {
		if indexOf(padvisor, want) < 0 {
			t.Fatalf("advisor events %v lack %s", padvisor, want)
		}
	}

	sv := h.session(h.client)
	if sv.Status != "active" || sv.BilledMinutes != 2 || *sv.CreditsCharged != holdRate || sv.Hold != nil || sv.NextBillingAt == nil {
		t.Fatalf("converted session: %+v", sv)
	}
	row := e.sessionRow(h.sid())
	if row.HoldExpiresAt != nil || row.LastResumedAt == nil {
		t.Fatalf("session row: %+v", row)
	}
	hold := e.holdRow(h.clientHeld.Hold.ID)
	if hold.Status != domain.HoldConverted || hold.ResolvedAt == nil || hold.ConvertedCredits != holdRate {
		t.Fatalf("hold row: %+v", hold)
	}
	if e.count(`SELECT count(*) FROM chat_messages WHERE hold_id = ? AND held`, hold.ID) != 0 {
		t.Fatal("messages are still held after the conversion")
	}
	if e.count(`SELECT count(*) FROM credit_ledger WHERE entry_type = 'chat_charge' AND idempotency_key = ?`, fmt.Sprintf("chat:%s:2", h.sid())) != 1 {
		t.Fatal("exactly one ledger entry for the held minute")
	}
	for _, m := range allMessages(t, h.client, h.conv()) {
		if m.Locked || m.Held {
			t.Fatalf("timeline after the conversion: %+v", m)
		}
	}
	// The reading continues on the clock: the next minute is billed on time.
	e.tick(time.Minute + time.Second)
	if sv := h.session(h.client); sv.BilledMinutes != 3 || *sv.CreditsCharged != 2*holdRate || sv.Status != "active" {
		t.Fatalf("billing after the hold: %+v", sv)
	}
	e.invariant(h.client.user.ID)
	e.waitEvent("chat.hold_converted", "session_id", h.sid().String())
}

// POST /chat/sessions/:id/hold/continue converts the hold on demand.
func TestHoldContinueEndpoint(t *testing.T) {
	e := newEnv(t)
	h := newHeldReading(t, e)
	h.advisorSay("one more thing")

	// Not enough credits: 402 with the contract's details, nothing changes.
	r := postKey(h.client, continuePath(h.sid()))
	if r.status != http.StatusPaymentRequired || r.errCode(t) != "insufficient_credits" {
		t.Fatalf("continue without credits: %d %s", r.status, r.body)
	}
	var body struct {
		Error struct {
			Details struct {
				Required float64 `json:"required"`
				Balance  float64 `json:"balance"`
			} `json:"details"`
		} `json:"error"`
	}
	r.decode(t, &body)
	if body.Error.Details.Required != holdRate || body.Error.Details.Balance != 0 {
		t.Fatalf("details: %s", r.body)
	}
	if sv := h.session(h.client); sv.Status != "held" || sv.BilledMinutes != 1 {
		t.Fatalf("session after a refused continue: %+v", sv)
	}
	// A bad key, someone else's reading, the advisor and a stranger.
	if r := h.client.call(http.MethodPost, continuePath(h.sid()), map[string]any{"idempotency_key": "x"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("bad key: %d %s", r.status, r.body)
	}
	if r := postKey(h.advisor, continuePath(h.sid())); r.status != http.StatusForbidden {
		t.Fatalf("advisor: %d", r.status)
	}
	if r := postKey(e.registerClient(), continuePath(h.sid())); r.status != http.StatusNotFound {
		t.Fatalf("stranger: %d", r.status)
	}

	// With credits (granted without triggering the automatic conversion) it converts.
	e.grantWithoutHook(h.client.user.ID, 100)
	if sv := h.session(h.client); sv.Status != "held" {
		t.Fatalf("the grant must not convert by itself here: %+v", sv)
	}
	var out service.HoldContinued
	h.client.must(http.StatusOK, http.MethodPost, continuePath(h.sid()), map[string]any{"idempotency_key": uniq("hold-key-")}, &out)
	if out.Session.Status != "active" || out.Hold.Status != "converted" || out.Wallet.Balance != 100-holdRate || out.Session.BilledMinutes != 2 {
		t.Fatalf("continue answer: %+v", out)
	}
	// Converted: a second call is 409 invalid_state, and nothing more is charged.
	r = postKey(h.client, continuePath(h.sid()))
	if r.status != http.StatusConflict || r.errCode(t) != "invalid_state" {
		t.Fatalf("second continue: %d %s", r.status, r.body)
	}
	if w := h.client.wallet(); w.Balance != 100-holdRate {
		t.Fatalf("wallet: %+v", w)
	}
	e.invariant(h.client.user.ID)
}

// An administrator's credit grant resumes a held reading too.
func TestAdminCreditGrantResumesAHeldReading(t *testing.T) {
	e := newEnv(t)
	h := newHeldReading(t, e)
	admin := e.createAdmin()
	admin.stepUp()
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/users/"+h.client.user.ID.String()+"/credits",
		map[string]any{"amount": 100, "reason": "goodwill", "idempotency_key": uniq("grant-key-")}, nil)
	h.cws.waitFor(realtime.EventSessionHoldEnded, 5*time.Second)
	if sv := h.session(h.client); sv.Status != "active" || sv.BilledMinutes != 2 {
		t.Fatalf("after the grant: %+v", sv)
	}
	if w := h.client.wallet(); w.Balance != 100-holdRate {
		t.Fatalf("wallet: %+v", w)
	}
}

// A hold nobody pays ends the reading with hold_expired: nothing is charged
// and the replies stay hidden.
func TestHoldExpiresAndTheReadingEnds(t *testing.T) {
	e := newEnv(t, roomy)
	h := newHeldReading(t, e)
	h.advisorSay("a message that stays hidden")
	h.advisorSay("and another one")
	h.cws.drain()

	e.tick(59 * time.Second)
	if sv := h.session(h.client); sv.Status != "held" {
		t.Fatalf("one second early: %+v", sv)
	}
	e.tick(2 * time.Second)

	ended := holdFrom(t, h.cws.waitFor(realtime.EventSessionHoldEnded, 5*time.Second).Data)
	if ended.Hold.Status != "expired" || ended.Hold.ResolvedAt == nil || ended.Hold.HeldMessages != 2 {
		t.Fatalf("hold_ended: %+v", ended.Hold)
	}
	evEnded := h.cws.waitFor(realtime.EventSessionEnded, 5*time.Second)
	if !strings.Contains(string(evEnded.Data), `"reason":"hold_expired"`) {
		t.Fatalf("session.ended: %s", evEnded.Data)
	}
	h.pws.waitFor(realtime.EventSessionHoldEnded, 5*time.Second)
	h.pws.waitFor(realtime.EventSessionEnded, 5*time.Second)

	sv := h.session(h.client)
	if sv.Status != "settled" || sv.EndReason != "hold_expired" || *sv.CreditsCharged != 0 || sv.BilledMinutes != 1 {
		t.Fatalf("ended session: %+v", sv)
	}
	if sv.Hold == nil || sv.Hold.Status != "expired" || sv.Hold.HeldMessages != 2 {
		t.Fatalf("the ended session carries the expired hold (the reveal UI after a reload): %+v", sv.Hold)
	}
	if got := h.session(h.advisor); got.Hold == nil || got.Hold.Status != "expired" || *got.PsychicEarnings != 0 {
		t.Fatalf("advisor's view: %+v earnings %v", got.Hold, got.PsychicEarnings)
	}
	hold := e.holdRow(ended.Hold.ID)
	if hold.Status != domain.HoldExpired || hold.RevealedAt != nil {
		t.Fatalf("hold row: %+v", hold)
	}
	if e.count(`SELECT count(*) FROM chat_messages WHERE hold_id = ? AND held`, hold.ID) != 2 {
		t.Fatal("the replies must stay hidden")
	}
	// The client still sees them locked, the end notice is in the timeline.
	var sawNotice bool
	for _, m := range allMessages(t, h.client, h.conv()) {
		if m.Body == "session.ended:hold_expired" {
			sawNotice = true
		}
		if m.Kind == "text" && m.SenderID != nil && *m.SenderID == h.advisor.user.ID && (!m.Locked || m.Body != "") {
			t.Fatalf("an expired hold must keep replies locked: %+v", m)
		}
	}
	if !sawNotice {
		t.Fatal("no session.ended:hold_expired notice")
	}
	if w := h.client.wallet(); w.Balance != 0 || w.FreeMinutes != 0 {
		t.Fatalf("wallet: %+v", w)
	}
	if e.count(`SELECT count(*) FROM psychic_earnings WHERE source_id = ?`, h.sid()) != 0 {
		t.Fatal("the advisor earns nothing for the hold time")
	}
	// The advisor is free again.
	other := e.registerClient()
	if r := other.call(http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": h.advisor.slug(), "request_key": uniq("req-key-")}); r.status == http.StatusConflict {
		t.Fatalf("the advisor is still busy: %s", r.body)
	}
	e.waitEvent("chat.hold_expired", "session_id", h.sid().String())
	e.waitEvent("chat.ended", "session_id", h.sid().String())
}

// While a payment the client started during the hold is processing, the hold
// is extended 15 s at a time, never past the payment grace.
func TestHoldIsExtendedWhileAPaymentIsPendingCappedByTheGrace(t *testing.T) {
	e := newEnv(t)
	h := newHeldReading(t, e)
	hold := h.clientHeld.Hold
	maxExpiry := hold.StartedAt.Add(180 * time.Second) // 60 s hold + 120 s grace (seeded)

	pendingPurchase := func(createdAt time.Time) {
		var pack domain.CreditPack
		if err := e.app.DB.First(&pack, "code = 'test_250'").Error; err != nil {
			t.Fatal(err)
		}
		p := domain.CreditPurchase{ID: domain.NewID(), UserID: h.client.user.ID, PackID: pack.ID, Credits: 250, PriceCents: 999, Currency: "USD",
			Status: domain.PurchasePending, Provider: "sandbox", IdempotencyKey: uniq("pending-key-"), CreatedAt: createdAt, UpdatedAt: createdAt}
		if err := e.app.DB.Create(&p).Error; err != nil {
			t.Fatal(err)
		}
	}
	pendingPurchase(e.clock.Now())

	e.tick(61 * time.Second) // past the original expiry
	ext := h.cws.waitFor(realtime.EventSessionHoldExtended, 5*time.Second)
	var x struct {
		SessionID uuid.UUID `json:"session_id"`
		HoldID    uuid.UUID `json:"hold_id"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(ext.Data, &x); err != nil || x.SessionID != h.sid() || x.HoldID != hold.ID {
		t.Fatalf("session.hold_extended: %s", ext.Data)
	}
	if want := e.clock.Now().Add(15 * time.Second); !x.ExpiresAt.Equal(want) {
		t.Fatalf("extended to %s, want now+15s = %s", x.ExpiresAt, want)
	}
	h.pws.waitFor(realtime.EventSessionHoldExtended, 5*time.Second)
	if sv := h.session(h.client); sv.Status != "held" || !sv.Hold.ExpiresAt.Equal(x.ExpiresAt) {
		t.Fatalf("session after the extension: %+v", sv)
	}

	// Keep ticking: the deadline never passes the grace, and the reading ends once it does.
	for i := 0; i < 20; i++ {
		e.tick(16 * time.Second)
		row := e.holdRow(hold.ID)
		if row.ExpiresAt.After(maxExpiry) {
			t.Fatalf("the hold was extended to %s, past the grace limit %s", row.ExpiresAt, maxExpiry)
		}
		if row.Status != domain.HoldOpen {
			break
		}
	}
	row := e.holdRow(hold.ID)
	if row.Status != domain.HoldExpired {
		t.Fatalf("the hold must end at the grace limit: %+v", row)
	}
	if got := e.sessionRow(h.sid()); got.EndReason != domain.EndHoldExpired {
		t.Fatalf("end reason: %+v", got)
	}
	if e.count(`SELECT count(*) FROM chat_session_holds WHERE session_id = ?`, h.sid()) != 1 {
		t.Fatal("exactly one hold")
	}
}

// A purchase created before the hold started, or a failed one, never extends it.
func TestHoldIsNotExtendedByAnOldOrNonPendingPurchase(t *testing.T) {
	e := newEnv(t)
	h := newHeldReading(t, e)
	var pack domain.CreditPack
	if err := e.app.DB.First(&pack, "code = 'test_250'").Error; err != nil {
		t.Fatal(err)
	}
	old := e.clock.Now().Add(-time.Hour)
	for status, at := range map[domain.PurchaseStatus]time.Time{domain.PurchasePending: old, domain.PurchaseFailed: e.clock.Now()} {
		p := domain.CreditPurchase{ID: domain.NewID(), UserID: h.client.user.ID, PackID: pack.ID, Credits: 250, PriceCents: 999, Currency: "USD",
			Status: status, Provider: "sandbox", IdempotencyKey: uniq("old-key-"), CreatedAt: at, UpdatedAt: at}
		if status == domain.PurchaseFailed {
			p.FailureReason = "card_declined"
		}
		if err := e.app.DB.Create(&p).Error; err != nil {
			t.Fatal(err)
		}
	}
	e.tick(61 * time.Second)
	if got := e.sessionRow(h.sid()); got.EndReason != domain.EndHoldExpired {
		t.Fatalf("the hold must expire: %+v", got)
	}
}

// After expiry the client pays one minute to read the replies: charged once,
// the advisor earns their share of that minute, and every retry answers 200.
func TestRevealChargesOnceAndPaysTheAdvisor(t *testing.T) {
	e := newEnv(t, roomy)
	h := newHeldReading(t, e)
	first := h.advisorSay("the first hidden reply")
	second := h.advisorSay("the second hidden reply")
	holdID := h.clientHeld.Hold.ID

	// An open hold cannot be revealed: top up instead.
	r := postKey(h.client, revealPath(holdID))
	if r.status != http.StatusConflict || r.errCode(t) != "invalid_state" {
		t.Fatalf("reveal while open: %d %s", r.status, r.body)
	}
	e.tick(61 * time.Second)
	h.cws.waitFor(realtime.EventSessionEnded, 5*time.Second)
	h.cws.drain()
	h.pws.drain()

	// No credits: 402 with the price and the balance; the hold stays expired.
	r = postKey(h.client, revealPath(holdID))
	if r.status != http.StatusPaymentRequired || r.errCode(t) != "insufficient_credits" || !strings.Contains(string(r.body), `"balance":0`) ||
		!strings.Contains(string(r.body), `"required":40`) {
		t.Fatalf("reveal without credits: %d %s", r.status, r.body)
	}
	if got := e.holdRow(holdID); got.Status != domain.HoldExpired {
		t.Fatalf("hold: %+v", got)
	}
	// The advisor and a stranger cannot reveal; the advisor can read the hold.
	if r := postKey(h.advisor, revealPath(holdID)); r.status != http.StatusForbidden {
		t.Fatalf("advisor reveal: %d", r.status)
	}
	stranger := e.registerClient()
	if r := postKey(stranger, revealPath(holdID)); r.status != http.StatusNotFound {
		t.Fatalf("stranger reveal: %d", r.status)
	}
	if r := stranger.call(http.MethodGet, "/api/v1/chat/holds/"+holdID.String(), nil); r.status != http.StatusNotFound {
		t.Fatalf("stranger reading the hold: %d", r.status)
	}
	var advisorHold service.HoldView
	h.advisor.must(http.StatusOK, http.MethodGet, "/api/v1/chat/holds/"+holdID.String(), nil, &advisorHold)
	if advisorHold.Status != "expired" || advisorHold.HeldMessages != 2 {
		t.Fatalf("advisor's hold: %+v", advisorHold)
	}
	if r := h.client.call(http.MethodPost, revealPath(holdID), map[string]any{"idempotency_key": "x"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("bad key: %d", r.status)
	}

	h.client.buyCredits(250)
	var out service.HoldRevealed
	key := uniq("reveal-key-")
	h.client.must(http.StatusOK, http.MethodPost, revealPath(holdID), map[string]any{"idempotency_key": key}, &out)
	if out.Hold.Status != "revealed" || out.Hold.RevealedAt == nil || out.Wallet.Balance != 250-holdRate || len(out.Messages) != 2 {
		t.Fatalf("reveal answer: %+v", out)
	}
	if out.Messages[0].ID != first.ID || out.Messages[0].Body != "the first hidden reply" || out.Messages[1].ID != second.ID ||
		out.Messages[1].Body != "the second hidden reply" || out.Messages[0].Locked || out.Messages[1].Held {
		t.Fatalf("revealed messages: %+v", out.Messages)
	}
	// Both parties are told; the wallet event goes to the client.
	rc := revealedFrom(t, h.cws.waitFor(realtime.EventMessagesRevealed, 5*time.Second).Data)
	if len(rc.Messages) != 2 || rc.Messages[0].Body != "the first hidden reply" || rc.HoldID != holdID {
		t.Fatalf("client's messages.revealed: %+v", rc)
	}
	ra := revealedFrom(t, h.pws.waitFor(realtime.EventMessagesRevealed, 5*time.Second).Data)
	if len(ra.MessageIDs) != 2 || ra.MessageIDs[0] != first.ID || len(ra.Messages) != 0 {
		t.Fatalf("advisor's messages.revealed: %+v", ra)
	}
	h.cws.waitFor(realtime.EventWalletUpdated, 5*time.Second)

	// One ledger entry, one earning: the advisor's share of the list rate.
	if n := e.count(`SELECT count(*) FROM credit_ledger WHERE entry_type = 'chat_reveal' AND user_id = ? AND amount = ? AND idempotency_key = ?`,
		h.client.user.ID, -holdRate, "chat-hold-reveal:"+holdID.String()); n != 1 {
		t.Fatalf("reveal ledger entries: %d", n)
	}
	var earning domain.Earning
	if err := e.app.DB.First(&earning, "source_type = ? AND source_id = ?", domain.EarningChatHoldReveal, holdID).Error; err != nil {
		t.Fatalf("earning: %v", err)
	}
	net, commission := domain.SplitEarnings(holdRate, e.app.Config.Billing.PsychicSharePercent)
	if earning.PsychicID != h.advisor.user.ID || earning.GrossCredits != holdRate || earning.NetCredits != net || earning.CommissionCredits != commission ||
		earning.EntryKind != domain.EarningKindEarning {
		t.Fatalf("earning: %+v (net %d commission %d)", earning, net, commission)
	}
	hold := e.holdRow(holdID)
	if hold.Status != domain.HoldRevealed || hold.RevealCredits != holdRate || hold.RevealRevenueMillicents <= 0 {
		t.Fatalf("hold row: %+v", hold)
	}
	if e.count(`SELECT count(*) FROM chat_messages WHERE hold_id = ? AND held`, holdID) != 0 {
		t.Fatal("messages are still held")
	}
	for _, m := range allMessages(t, h.client, h.conv()) {
		if m.Locked || m.Held {
			t.Fatalf("timeline after the reveal: %+v", m)
		}
	}
	// Replays (same key, other key) are 200 and charge nothing more.
	for _, k := range []string{key, uniq("reveal-key-")} {
		var again service.HoldRevealed
		h.client.must(http.StatusOK, http.MethodPost, revealPath(holdID), map[string]any{"idempotency_key": k}, &again)
		if again.Hold.Status != "revealed" || len(again.Messages) != 2 || again.Wallet.Balance != 250-holdRate {
			t.Fatalf("replay: %+v", again)
		}
	}
	if w := h.client.wallet(); w.Balance != 250-holdRate {
		t.Fatalf("wallet: %+v", w)
	}
	if e.count(`SELECT count(*) FROM psychic_earnings WHERE source_id = ?`, holdID) != 1 {
		t.Fatal("exactly one earning")
	}
	// The advisor's balance and earnings summary include it.
	var balance service.PsychicBalanceView
	h.advisor.must(http.StatusOK, http.MethodGet, "/api/v1/psychic/balance", nil, &balance)
	if balance.LifetimeNetCredits < net {
		t.Fatalf("advisor's lifetime earnings %d must include the reveal (%d)", balance.LifetimeNetCredits, net)
	}
	e.invariant(h.client.user.ID)
	e.waitEvent("chat.hold_revealed", "session_id", h.sid().String())
}

// The reveal counts as chat revenue in the admin revenue report (§24.5).
func TestRevealCountsInTheRevenueReport(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	day := func(d int) string { return e.clock.Now().AddDate(0, 0, d).UTC().Format("2006-01-02") }
	path := fmt.Sprintf("/api/v1/admin/reports/revenue?from=%s&to=%s", day(-1), day(2))
	recognized := func() int64 {
		var rep service.RevenueReport
		admin.must(http.StatusOK, http.MethodGet, path, nil, &rep)
		return rep.Totals.RecognizedCents
	}
	before := recognized()
	h := newHeldReading(t, e)
	h.advisorSay("worth one minute")
	e.tick(61 * time.Second)
	h.client.buyCredits(250)
	mid := recognized()
	var out service.HoldRevealed
	h.client.must(http.StatusOK, http.MethodPost, revealPath(h.clientHeld.Hold.ID), map[string]any{"idempotency_key": uniq("reveal-key-")}, &out)
	after := recognized()
	// 40 credits of a 250-credit, 9.99 pack: 40 × 999 / 250 = 159.84 cents.
	if got := after - mid; got < 159 || got > 160 {
		t.Fatalf("the reveal recognised %d cents (before %d, after purchase %d, after reveal %d)", got, before, mid, after)
	}
}

// Two reveals at once charge one minute and pay one earning.
func TestConcurrentRevealsChargeOnce(t *testing.T) {
	e := newEnv(t)
	h := newHeldReading(t, e)
	h.advisorSay("hidden for a while")
	e.tick(61 * time.Second)
	h.client.buyCredits(250)
	holdID := h.clientHeld.Hold.ID

	const callers = 10
	results := make([]response, callers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i] = postKey(h.client, revealPath(holdID))
		}(i)
	}
	close(start)
	wg.Wait()
	for i, r := range results {
		if r.status != http.StatusOK {
			t.Fatalf("reveal %d: %d %s", i, r.status, r.body)
		}
	}
	if n := e.count(`SELECT count(*) FROM credit_ledger WHERE entry_type = 'chat_reveal' AND user_id = ?`, h.client.user.ID); n != 1 {
		t.Fatalf("%d reveal charges for %d concurrent calls", n, callers)
	}
	if n := e.count(`SELECT count(*) FROM psychic_earnings WHERE source_type = 'chat_hold_reveal' AND source_id = ?`, holdID); n != 1 {
		t.Fatalf("%d reveal earnings", n)
	}
	if w := h.client.wallet(); w.Balance != 250-holdRate {
		t.Fatalf("wallet: %+v", w)
	}
	e.invariant(h.client.user.ID)
}

// Revealing a converted hold is a plain 200: nothing was hidden, nothing is charged.
// A hold the advisor wrote nothing during has nothing to sell: revealing it
// is refused and charges nothing, even with credits (integration review).
func TestRevealWithoutHiddenMessagesChargesNothing(t *testing.T) {
	e := newEnv(t, roomy)
	h := newHeldReading(t, e)
	holdID := h.clientHeld.Hold.ID
	e.tick(61 * time.Second)
	h.cws.waitFor(realtime.EventSessionEnded, 5*time.Second)
	h.client.buyCredits(250)
	r := postKey(h.client, revealPath(holdID))
	if r.status != http.StatusConflict || r.errCode(t) != "invalid_state" {
		t.Fatalf("reveal of an empty hold: %d %s", r.status, r.body)
	}
	if got := e.holdRow(holdID); got.Status != domain.HoldExpired || got.RevealCredits != 0 {
		t.Fatalf("hold: %+v", got)
	}
	if e.count(`SELECT count(*) FROM credit_ledger WHERE entry_type = 'chat_reveal' AND user_id = ?`, h.client.user.ID) != 0 {
		t.Fatal("an empty hold was charged")
	}
	if e.count(`SELECT count(*) FROM psychic_earnings WHERE source_type = 'chat_hold_reveal' AND source_id = ?`, holdID) != 0 {
		t.Fatal("an empty hold paid the advisor")
	}
}

func TestRevealOfAConvertedHoldChargesNothing(t *testing.T) {
	e := newEnv(t)
	h := newHeldReading(t, e)
	h.advisorSay("this was revealed by the top-up")
	h.client.buyCredits(250)
	var out service.HoldRevealed
	h.client.must(http.StatusOK, http.MethodPost, revealPath(h.clientHeld.Hold.ID), map[string]any{"idempotency_key": uniq("reveal-key-")}, &out)
	if out.Hold.Status != "converted" || len(out.Messages) != 1 || out.Messages[0].Body == "" || out.Wallet.Balance != 250-holdRate {
		t.Fatalf("reveal of a converted hold: %+v", out)
	}
	if e.count(`SELECT count(*) FROM credit_ledger WHERE entry_type = 'chat_reveal' AND user_id = ?`, h.client.user.ID) != 0 {
		t.Fatal("a reveal charge was booked for a converted hold")
	}
}

// Ending a held reading (client, advisor, administrator, account closed)
// expires the hold; the reading keeps its own end reason.
func TestEndingAHeldReadingExpiresTheHold(t *testing.T) {
	for name, tc := range map[string]struct {
		end    func(e *env, h *heldReading)
		reason string
	}{
		"client": {func(e *env, h *heldReading) { endSession(t, h.client, h.sid()) }, "client_ended"},
		"advisor": {func(e *env, h *heldReading) {
			if r := h.advisor.call(http.MethodPost, "/api/v1/chat/sessions/"+h.sid().String()+"/end", nil); r.status != http.StatusOK {
				t.Fatalf("advisor end: %d %s", r.status, r.body)
			}
		}, "psychic_ended"},
		"admin": {func(e *env, h *heldReading) {
			admin := e.createAdmin()
			admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/chat/sessions/"+h.sid().String()+"/terminate", map[string]any{"reason": "test"}, nil)
		}, "admin_terminated"},
		"account disabled": {func(e *env, h *heldReading) {
			e.app.Services.Chat.EndAllForUser(context.Background(), h.client.user.ID)
		}, "account_disabled"},
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			h := newHeldReading(t, e)
			h.advisorSay("will stay hidden")
			tc.end(e, h)
			ended := holdFrom(t, h.cws.waitFor(realtime.EventSessionHoldEnded, 5*time.Second).Data)
			if ended.Hold.Status != "expired" {
				t.Fatalf("hold_ended: %+v", ended.Hold)
			}
			row := e.sessionRow(h.sid())
			if string(row.EndReason) != tc.reason {
				t.Fatalf("end reason %q, want %q", row.EndReason, tc.reason)
			}
			if got := e.holdRow(ended.Hold.ID); got.Status != domain.HoldExpired || got.ResolvedAt == nil {
				t.Fatalf("hold: %+v", got)
			}
			// The replies can be revealed afterwards (one minute at the reading's rate).
			h.client.buyCredits(250)
			var out service.HoldRevealed
			h.client.must(http.StatusOK, http.MethodPost, revealPath(ended.Hold.ID), map[string]any{"idempotency_key": uniq("reveal-key-")}, &out)
			if len(out.Messages) != 1 || out.Messages[0].Body != "will stay hidden" {
				t.Fatalf("reveal after %s ended it: %+v", name, out)
			}
		})
	}
}

// The settings decide: off for the trial, on for paid credits; the reading
// ends at once exactly as before when they say so.
func TestHoldSettingsDecideWhetherAReadingIsHeld(t *testing.T) {
	t.Run("after_trial off: the reading ends", func(t *testing.T) {
		e := newEnv(t)
		e.pinTrialHold(false, false, 60, 120)
		e.pinSignupFreeMinutes(1)
		_, _, cws, pws, s := startedSession(t, e, holdRate, 0)
		e.tick(time.Minute + time.Second)
		ended := cws.waitFor(realtime.EventSessionEnded, 5*time.Second)
		if !strings.Contains(string(ended.Data), `"reason":"credits_exhausted"`) {
			t.Fatalf("ended: %s", ended.Data)
		}
		pws.waitFor(realtime.EventSessionEnded, 5*time.Second)
		cws.expectNone(realtime.EventSessionHeld, 300*time.Millisecond)
		if e.count(`SELECT count(*) FROM chat_session_holds WHERE session_id = ?`, s.ID) != 0 {
			t.Fatal("a hold was created")
		}
	})
	t.Run("paid credits run out: only with after_paid", func(t *testing.T) {
		for _, afterPaid := range []bool{false, true} {
			e := newEnv(t)
			e.pinTrialHold(true, afterPaid, 60, 120)
			e.disableSignupTrial()
			_, _, cws, _, s := startedSession(t, e, paidRate, 250) // exactly one paid minute
			e.tick(time.Minute + time.Second)
			if afterPaid {
				f := holdFrom(t, cws.waitFor(realtime.EventSessionHeld, 5*time.Second).Data)
				if f.Hold.Reason != "credits_exhausted" || f.Session.Status != "held" || f.Hold.PriceCredits != paidRate {
					t.Fatalf("paid hold: %+v", f)
				}
				continue
			}
			ended := cws.waitFor(realtime.EventSessionEnded, 5*time.Second)
			if !strings.Contains(string(ended.Data), `"reason":"credits_exhausted"`) {
				t.Fatalf("ended: %s", ended.Data)
			}
			if e.count(`SELECT count(*) FROM chat_session_holds WHERE session_id = ?`, s.ID) != 0 {
				t.Fatal("a hold was created")
			}
		}
	})
	t.Run("a paid reading is held for credits_exhausted and the hold converts on top-up", func(t *testing.T) {
		e := newEnv(t)
		e.pinTrialHold(true, true, 60, 120)
		e.disableSignupTrial()
		cl, _, cws, _, s := startedSession(t, e, paidRate, 250)
		e.tick(time.Minute + time.Second)
		cws.waitFor(realtime.EventSessionHeld, 5*time.Second)
		cl.buyCredits(250)
		cws.waitFor(realtime.EventSessionHoldEnded, 5*time.Second)
		var sv service.SessionView
		cl.must(http.StatusOK, http.MethodGet, "/api/v1/chat/sessions/"+s.ID.String(), nil, &sv)
		if sv.Status != "active" || sv.BilledMinutes != 2 || *sv.CreditsCharged != 2*paidRate {
			t.Fatalf("converted paid reading: %+v", sv)
		}
	})
	t.Run("a configured hold length", func(t *testing.T) {
		e := newEnv(t)
		e.pinTrialHold(true, false, 30, 0)
		h := newHeldReading(t, e)
		if got := h.clientHeld.Hold.ExpiresAt.Sub(h.clientHeld.Hold.StartedAt); got != 30*time.Second {
			t.Fatalf("hold length %s", got)
		}
		e.tick(31 * time.Second)
		if got := e.sessionRow(h.sid()); got.EndReason != domain.EndHoldExpired {
			t.Fatalf("the 30 s hold must have expired: %+v", got)
		}
	})
}

// A top-up racing the expiry has exactly one outcome: converted (one minute
// charged) or expired (nothing charged), never both and never a lost credit.
// (A purchase row created before the job looks counts as "payment pending", so
// the job extends the hold and the top-up then converts it: also one outcome.)
func TestTopUpRacingExpiryHasExactlyOneOutcome(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	var pack domain.CreditPack
	if err := e.app.DB.First(&pack, "code = 'test_250'").Error; err != nil {
		t.Fatal(err)
	}
	converted, expired := 0, 0
	for i := 0; i < 12; i++ {
		h := newHeldReading(t, e)
		e.clock.Advance(61 * time.Second) // past the expiry; the job has not run yet
		var wg sync.WaitGroup
		var buy response
		var tickErr error
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			if i%2 == 1 {
				time.Sleep(time.Duration(i) * 4 * time.Millisecond) // odd rounds: the tick gets a head start
			}
			buy = h.client.call(http.MethodPost, "/api/v1/credits/purchases", map[string]any{
				"pack_id": pack.ID, "idempotency_key": uniq("purchase-key-"), "consent": purchaseConsent()})
		}()
		go func() {
			defer wg.Done()
			<-start
			if i%2 == 0 {
				time.Sleep(time.Duration(i) * 4 * time.Millisecond) // even rounds: the top-up gets a head start
			}
			tickErr = e.app.Scheduler.Tick(context.Background())
		}()
		close(start)
		wg.Wait()
		if tickErr != nil || buy.status != http.StatusCreated {
			t.Fatalf("round %d: tick %v, purchase %d %s", i, tickErr, buy.status, buy.body)
		}
		sess, hold := e.sessionRow(h.sid()), e.holdRow(h.clientHeld.Hold.ID)
		charges := e.count(`SELECT count(*) FROM credit_ledger WHERE entry_type = 'chat_charge' AND user_id = ?`, h.client.user.ID)
		balance := h.client.wallet().Balance
		switch {
		case hold.Status == domain.HoldConverted:
			converted++
			if sess.Status != domain.SessionActive || sess.BilledMinutes != 2 || charges != 1 || balance != 250-holdRate {
				t.Fatalf("round %d converted but: session %+v charges %d balance %d", i, sess, charges, balance)
			}
		case hold.Status == domain.HoldExpired:
			expired++
			if sess.EndReason != domain.EndHoldExpired || sess.BilledMinutes != 1 || charges != 0 || balance != 250 {
				t.Fatalf("round %d expired but: session %+v charges %d balance %d", i, sess, charges, balance)
			}
		default:
			t.Fatalf("round %d: the hold is %s", i, hold.Status)
		}
		e.invariant(h.client.user.ID)
		e.tick(0) // let a late tick settle anything left
		if got := e.holdRow(hold.ID); got.Status != hold.Status {
			t.Fatalf("round %d: the outcome changed from %s to %s", i, hold.Status, got.Status)
		}
	}
	t.Logf("converted %d, expired %d", converted, expired)
}

// A top-up that lands after the deadline but before the expiry job ran still
// converts: the client paid while the reading was still held.
func TestTopUpBeforeTheJobRunsConvertsEvenPastTheDeadline(t *testing.T) {
	e := newEnv(t)
	h := newHeldReading(t, e)
	e.clock.Advance(61 * time.Second)
	h.client.buyCredits(250)
	if sv := h.session(h.client); sv.Status != "active" || sv.BilledMinutes != 2 {
		t.Fatalf("after the late top-up: %+v", sv)
	}
	if err := e.app.Scheduler.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hold := e.holdRow(h.clientHeld.Hold.ID); hold.Status != domain.HoldConverted {
		t.Fatalf("the job must find nothing to expire: %+v", hold)
	}
	if sv := h.session(h.client); sv.Status != "active" {
		t.Fatalf("session: %+v", sv)
	}
}

// The personal-data export includes the replies a hold hid, in full (§26.8).
func TestExportIncludesHeldMessages(t *testing.T) {
	e := newEnv(t)
	h := newHeldReading(t, e)
	h.advisorSay("a reply the client never unlocked")
	e.tick(61 * time.Second)
	files := exportFiles(t, e, h.client)
	var found bool
	for name, data := range files {
		if strings.Contains(string(data), "a reply the client never unlocked") {
			found = true
			if !strings.Contains(string(data), `"held"`) {
				t.Fatalf("%s: the held flag is missing", name)
			}
		}
	}
	if !found {
		t.Fatalf("the export lacks the held message (%d files)", len(files))
	}
}

// The admin setting: validation, concurrency control, step-up, audit, stats.
func TestAdminTrialHoldSetting(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	t.Cleanup(func() {
		e.app.DB.Exec(`UPDATE site_settings SET value = '{"after_trial": true, "after_paid": false, "hold_seconds": 60, "payment_grace_seconds": 120}'::jsonb, version = 1 WHERE key = 'trial_hold'`)
	})
	var cur service.TrialHoldSetting
	admin.must(http.StatusOK, http.MethodGet, trialHoldPath, nil, &cur)
	if !cur.AfterTrial || cur.AfterPaid || cur.HoldSeconds != 60 || cur.PaymentGraceSeconds != 120 || cur.Version < 1 || cur.Stats.WindowDays != 30 {
		t.Fatalf("seed: %+v", cur)
	}
	if r := admin.call(http.MethodGet, trialHoldPath, nil); !strings.Contains(string(r.body), `"stats":{"window_days":30`) {
		t.Fatalf("stats must always be present: %s", r.body)
	}
	if r := e.registerClient().call(http.MethodGet, trialHoldPath, nil); r.status == http.StatusOK {
		t.Fatalf("a client read the admin setting: %d", r.status)
	}
	body := map[string]any{"after_trial": false, "after_paid": true, "hold_seconds": 90, "payment_grace_seconds": 30, "version": cur.Version}

	e.clock.Advance(11 * time.Minute)
	if r := admin.call(http.MethodPut, trialHoldPath, body); r.status != http.StatusForbidden || r.errCode(t) != "step_up_required" {
		t.Fatalf("no step-up: %d %s", r.status, r.body)
	}
	admin.stepUp()
	for name, tc := range map[string]struct {
		body map[string]any
		want []string
	}{
		"empty":         {map[string]any{}, []string{"after_trial", "after_paid", "hold_seconds", "payment_grace_seconds", "version"}},
		"hold too low":  {map[string]any{"after_trial": true, "after_paid": false, "hold_seconds": 29, "payment_grace_seconds": 0, "version": cur.Version}, []string{"hold_seconds"}},
		"hold too high": {map[string]any{"after_trial": true, "after_paid": false, "hold_seconds": 301, "payment_grace_seconds": 0, "version": cur.Version}, []string{"hold_seconds"}},
		"grace too high": {map[string]any{"after_trial": true, "after_paid": false, "hold_seconds": 60, "payment_grace_seconds": 301, "version": cur.Version},
			[]string{"payment_grace_seconds"}},
		"negative grace": {map[string]any{"after_trial": true, "after_paid": false, "hold_seconds": 60, "payment_grace_seconds": -1, "version": cur.Version},
			[]string{"payment_grace_seconds"}},
		"missing switch": {map[string]any{"after_paid": false, "hold_seconds": 60, "payment_grace_seconds": 0, "version": cur.Version}, []string{"after_trial"}},
		"no version":     {map[string]any{"after_trial": true, "after_paid": false, "hold_seconds": 60, "payment_grace_seconds": 0}, []string{"version"}},
	} {
		r := admin.call(http.MethodPut, trialHoldPath, tc.body)
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
	for _, n := range []int{30, 300} { // the bounds themselves are valid
		admin.stepUp()
		admin.must(http.StatusOK, http.MethodPut, trialHoldPath, map[string]any{"after_trial": true, "after_paid": false, "hold_seconds": n,
			"payment_grace_seconds": 0, "version": cur.Version}, &cur)
	}
	admin.stepUp()
	admin.must(http.StatusOK, http.MethodGet, trialHoldPath, nil, &cur)

	stale := map[string]any{"after_trial": false, "after_paid": true, "hold_seconds": 90, "payment_grace_seconds": 30, "version": cur.Version - 1}
	if r := admin.call(http.MethodPut, trialHoldPath, stale); r.status != http.StatusConflict || r.errCode(t) != "version_conflict" {
		t.Fatalf("stale version: %d %s", r.status, r.body)
	}
	body["version"] = cur.Version
	var saved service.TrialHoldSetting
	admin.must(http.StatusOK, http.MethodPut, trialHoldPath, body, &saved)
	if saved.Version != cur.Version+1 || saved.AfterTrial || !saved.AfterPaid || saved.HoldSeconds != 90 || saved.PaymentGraceSeconds != 30 ||
		saved.UpdatedBy == nil || saved.UpdatedBy.ID != admin.adminID {
		t.Fatalf("saved: %+v", saved)
	}
	// Live at once for the chat service.
	if got := e.app.Services.Site.TrialHold(context.Background()); got.AfterTrial || !got.AfterPaid || got.HoldSeconds != 90 || got.PaymentGraceSeconds != 30 {
		t.Fatalf("service sees %+v", got)
	}
	// Saving the same values again changes nothing (no new version, no audit row).
	admin.stepUp()
	same := body
	same["version"] = saved.Version
	var again service.TrialHoldSetting
	admin.must(http.StatusOK, http.MethodPut, trialHoldPath, same, &again)
	if again.Version != saved.Version {
		t.Fatalf("an unchanged save bumped the version: %d -> %d", saved.Version, again.Version)
	}
	rows := e.adminAudit(admin.adminID, "site.trial_hold.updated")
	if len(rows) < 1 {
		t.Fatal("no audit row")
	}
	last := rows[len(rows)-1]
	if b, _ := last.Metadata["before"].(map[string]any); b["hold_seconds"] != float64(300) || b["after_trial"] != true {
		t.Fatalf("audit before: %v", last.Metadata)
	}
	if a, _ := last.Metadata["after"].(map[string]any); a["hold_seconds"] != float64(90) || a["after_paid"] != true || a["payment_grace_seconds"] != float64(30) {
		t.Fatalf("audit after: %v", last.Metadata)
	}
	// The database constraint is the last line of defence.
	for _, bad := range []string{
		`{"after_trial": true, "after_paid": false, "hold_seconds": 29, "payment_grace_seconds": 0}`,
		`{"after_trial": true, "after_paid": false, "hold_seconds": 60, "payment_grace_seconds": 301}`,
		`{"after_trial": "yes", "after_paid": false, "hold_seconds": 60, "payment_grace_seconds": 0}`,
	} {
		if err := e.app.DB.Exec(`UPDATE site_settings SET value = ?::jsonb WHERE key = 'trial_hold'`, bad).Error; err == nil {
			t.Fatalf("the CHECK constraint accepted %s", bad)
		}
	}
}

// The 30-day statistics count what really happened and are cached for 60 s.
func TestTrialHoldStats(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	// The database is shared by the whole suite: the figures are compared with
	// what the window already held (read with the definitions of the card).
	var base struct{ Started, Converted, Expired, Revealed, FromConversion, FromReveals int64 }
	e.app.DB.Raw(`SELECT count(*) AS started, count(*) FILTER (WHERE status = 'converted') AS converted,
		count(*) FILTER (WHERE status IN ('expired', 'revealed')) AS expired, count(*) FILTER (WHERE status = 'revealed') AS revealed,
		COALESCE(sum(converted_credits), 0) AS from_conversion, COALESCE(sum(reveal_credits), 0) AS from_reveals
		FROM chat_session_holds WHERE started_at >= ?`, e.clock.Now().AddDate(0, 0, -30)).Scan(&base)
	// Holds that earlier tests left open may expire during this test's clock
	// jumps (the scheduler expires every due hold): they are counted apart.
	var leftover []uuid.UUID
	e.app.DB.Raw(`SELECT id FROM chat_session_holds WHERE status = 'open' AND started_at >= ?`, e.clock.Now().AddDate(0, 0, -30)).Scan(&leftover)

	a := newHeldReading(t, e) // converted by a top-up
	a.advisorSay("converted")
	a.client.buyCredits(250)
	b := newHeldReading(t, e) // expired, then revealed
	b.advisorSay("revealed")
	e.tick(61 * time.Second)
	b.client.buyCredits(250)
	b.client.must(http.StatusOK, http.MethodPost, revealPath(b.clientHeld.Hold.ID), map[string]any{"idempotency_key": uniq("reveal-key-")}, nil)
	c := newHeldReading(t, e) // expired and left hidden
	e.tick(61 * time.Second)
	_ = c

	var st service.TrialHoldSetting
	admin.must(http.StatusOK, http.MethodGet, trialHoldPath, nil, &st)
	s := st.Stats
	var leftoverExpired int64
	if len(leftover) > 0 {
		leftoverExpired = e.count(`SELECT count(*) FROM chat_session_holds WHERE id IN ? AND status IN ('expired', 'revealed')`, leftover)
	}
	if s.WindowDays != 30 || s.Started != base.Started+3 || s.Converted != base.Converted+1 || s.Expired != base.Expired+2+leftoverExpired || s.Revealed != base.Revealed+1 ||
		s.CreditsFromConversion != base.FromConversion+holdRate || s.CreditsFromReveals != base.FromReveals+holdRate {
		t.Fatalf("stats: %+v (before: %+v)", s, base)
	}
	if want := float64(s.Converted) / float64(s.Started); s.ConversionRate < want-1e-9 || s.ConversionRate > want+1e-9 ||
		s.RevealRate < float64(s.Revealed)/float64(s.Started)-1e-9 || s.RevealRate > float64(s.Revealed)/float64(s.Started)+1e-9 {
		t.Fatalf("rates: %+v", s)
	}
	// Cached for 60 s: a change in the table does not show yet.
	e.app.DB.Exec(`UPDATE chat_session_holds SET converted_credits = converted_credits + 999 WHERE id = ?`, a.clientHeld.Hold.ID)
	var cached service.TrialHoldSetting
	admin.must(http.StatusOK, http.MethodGet, trialHoldPath, nil, &cached)
	if cached.Stats.CreditsFromConversion != s.CreditsFromConversion {
		t.Fatalf("stats are not cached: %+v", cached.Stats)
	}
}

// The hold and its stats are not rate-limited per se, but every hold route is
// age-gated and authenticated like the rest of /chat.
func TestHoldRoutesNeedASession(t *testing.T) {
	e := newEnv(t)
	anon := e.client()
	for _, p := range []string{"/api/v1/chat/holds/" + uuid.NewString(), continuePath(uuid.New()), revealPath(uuid.New())} {
		method := http.MethodGet
		if strings.HasSuffix(p, "reveal") || strings.HasSuffix(p, "continue") {
			method = http.MethodPost
		}
		if r := anon.call(method, p, map[string]any{"idempotency_key": uniq("hold-key-")}); r.status != http.StatusUnauthorized {
			t.Fatalf("%s %s: %d", method, p, r.status)
		}
	}
	if r := anon.call(http.MethodGet, trialHoldPath, nil); r.status != http.StatusUnauthorized && r.status != http.StatusForbidden {
		t.Fatalf("anonymous read of the admin setting: %d", r.status)
	}
}

// A free trial that continues after its hold is a free-to-paid conversion in
// the advisor's KPI (the first reading started free and went on paying); a
// trial whose hold expired is not (integration review, §8.1 / §26.1).
func TestHoldConversionCountsAsFreeToPaid(t *testing.T) {
	e := newEnv(t)
	converted := newHeldReading(t, e)
	converted.advisorSay("continue")
	converted.client.buyCredits(250)
	if got := e.sessionRow(converted.sid()); got.Status != domain.SessionActive || got.CreditsCharged == 0 {
		t.Fatalf("converted reading: %+v", got)
	}
	day := e.clock.Now().UTC()
	from, to := day.AddDate(0, 0, -1).Format("2006-01-02"), day.AddDate(0, 0, 1).Format("2006-01-02")
	var out service.StatsOverview
	converted.advisor.must(http.StatusOK, http.MethodGet, "/api/v1/psychic/stats/overview?from="+from+"&to="+to+"&tz=UTC", nil, &out)
	if out.KPIs.FreeToPaid.Numerator != 1 || out.KPIs.FreeToPaid.Denominator != 1 {
		t.Fatalf("free→paid after a converted hold: %+v", out.KPIs.FreeToPaid)
	}
}
