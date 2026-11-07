package service

import (
	"context"
	"errors"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

var clientMsgID = regexp.MustCompile(`^[A-Za-z0-9_\-:.]{6,64}$`)

// orderedPublisher is implemented by publishers that deliver conversation
// events in seq order (realtime.Publisher). Other publishers fall back to
// plain Publish.
type orderedPublisher interface {
	PublishOrdered(ctx context.Context, conversationID uuid.UUID, seq int64, eventType string, data any, userIDs ...uuid.UUID) error
	FlushGaps(ctx context.Context, maxAge time.Duration) (int, error)
}

// publishMessage delivers message.created to the participants in seq order.
func (c *ChatService) publishMessage(ctx context.Context, v MessageView, userIDs ...uuid.UUID) {
	payload := map[string]any{"message": v}
	if op, ok := c.events.(orderedPublisher); ok {
		warnErr(ctx, "chat: publish message", op.PublishOrdered(ctx, v.ConversationID, v.Seq, realtime.EventMessageCreated, payload, userIDs...))
	} else {
		warnErr(ctx, "chat: publish message", c.events.Publish(ctx, realtime.EventMessageCreated, payload, userIDs...))
	}
	// revisions 3.0: chat unread (all callers pass client, then psychic).
	if len(userIDs) == 2 {
		c.onMessageStored(ctx, v, userIDs[0], userIDs[1])
	}
}

// FlushMessageGaps publishes message events held back because a
// predecessor's publication never arrived (worker loop).
func (c *ChatService) FlushMessageGaps(ctx context.Context) (int, error) {
	op, ok := c.events.(orderedPublisher)
	if !ok {
		return 0, nil
	}
	return op.FlushGaps(ctx, realtime.GapFlushAge)
}

// normalizeMessageBody cleans a message body and enforces the length limit
// (in characters).
func normalizeMessageBody(raw string, maxLen int) (string, error) {
	body := CleanText(raw)
	if body == "" {
		return "", apperror.Validation(map[string]string{"body": "Write something before sending."})
	}
	if utf8.RuneCountInString(body) > maxLen {
		return "", apperror.Validation(map[string]string{"body": "Keep your message under the length limit."}).
			WithDetails(map[string]any{"max": maxLen})
	}
	return body, nil
}

// SendMessageInput is a chat message intent.
type SendMessageInput struct {
	SessionID   uuid.UUID
	Body        string
	ClientMsgID string
}

// SendMessage stores and delivers a message in an Active session. Retries
// with the same client message ID return the original message.
func (c *ChatService) SendMessage(ctx context.Context, p reqctx.Principal, in SendMessageInput) (*MessageView, error) {
	body, err := normalizeMessageBody(in.Body, c.cfg.MaxMessageLength)
	if err != nil {
		return nil, err
	}
	var cmid *string
	if in.ClientMsgID != "" {
		if !clientMsgID.MatchString(in.ClientMsgID) {
			return nil, apperror.Validation(map[string]string{"client_msg_id": "Invalid message identifier."})
		}
		if existing, err := c.chat.GetMessageByClientID(ctx, p.UserID, in.ClientMsgID); err == nil {
			v := messageView(existing)
			return &v, nil
		} else if !isNotFound(err) {
			return nil, apperror.Internal(err)
		}
		cmid = &in.ClientMsgID
	}
	if res, err := c.limiter.Allow(ctx, p.UserID.String(), c.cfg.MessageRule); err == nil && !res.Allowed {
		return nil, apperror.RateLimited(res.RetryAfter)
	} else if err != nil {
		warnErr(ctx, "chat: message rate limiter unavailable", err)
	}

	var msg *domain.Message
	var session *domain.Session
	err = c.tx.WithinTx(ctx, func(ctx context.Context) error {
		msg, session = nil, nil
		s, err := c.chat.GetSessionForUpdate(ctx, in.SessionID)
		if err != nil {
			return err
		}
		if !s.IsParticipant(p.UserID) {
			return repository.ErrNotFound
		}
		if s.Status != domain.SessionActive {
			msg := "This chat is not active."
			if s.Status == domain.SessionDisconnected {
				msg = "The chat is paused while the other person reconnects."
			} else if s.Status == domain.SessionAccepted || s.Status == domain.SessionRequested {
				msg = "The chat has not started yet."
			}
			return apperror.InvalidState(msg).WithDetails(map[string]any{"status": s.Status})
		}
		// Retries of the same send serialise on the session lock: look again
		// before assigning a sequence number.
		if cmid != nil {
			if existing, err := c.chat.GetMessageByClientID(ctx, p.UserID, *cmid); err == nil {
				msg = existing
				return nil
			} else if !isNotFound(err) {
				return err
			}
		}
		now := c.clock.Now()
		sender := p.UserID
		m := &domain.Message{ID: domain.NewID(), ConversationID: s.ConversationID, SessionID: s.ID, SenderID: &sender,
			Kind: domain.MessageText, Body: body, ClientMsgID: cmid, CreatedAt: now, UpdatedAt: now}
		// Assigns the seq, marks the sender as having read up to it and
		// counts it as unread for the other participant.
		if _, err := c.chat.AppendMessage(ctx, m); err != nil {
			return err
		}
		msg, session = m, s
		return nil
	})
	if errors.Is(err, repository.ErrDuplicate) && cmid != nil {
		// The same client message ID was stored by a concurrent request (in
		// another session): the transaction rolled back; return the original.
		existing, gerr := c.chat.GetMessageByClientID(ctx, p.UserID, *cmid)
		if gerr != nil {
			return nil, apperror.Internal(gerr)
		}
		v := messageView(existing)
		return &v, nil
	}
	if err != nil {
		return nil, mapRepoErr(err, "Session")
	}
	v := messageView(msg)
	if session != nil {
		c.publishMessage(ctx, v, session.ClientID, session.PsychicID)
		c.tracker.Track(ctx, "chat.message_sent", map[string]any{"session_id": session.ID, "length": utf8.RuneCountInString(body), "role": string(p.Role)})
	}
	return &v, nil
}

// ReadInput moves the caller's read position to Seq, or to the seq of
// MessageID. Exactly one must be set.
type ReadInput struct {
	Seq       *int64
	MessageID *uuid.UUID
}

// MarkRead advances the caller's read position (never backwards, never past
// the newest message), recounts their unread messages and sends a read
// receipt to the other participant when the position moved.
func (c *ChatService) MarkRead(ctx context.Context, p reqctx.Principal, conversationID uuid.UUID, in ReadInput) (*ReadResult, error) {
	if (in.Seq == nil) == (in.MessageID == nil) {
		return nil, apperror.Validation(map[string]string{"seq": "Send either seq or message_id."})
	}
	if in.Seq != nil && *in.Seq < 0 {
		return nil, apperror.Validation(map[string]string{"seq": "Use a whole number, 0 or more."})
	}
	var res ReadResult
	var conv *domain.Conversation
	var advanced bool
	err := c.tx.WithinTx(ctx, func(ctx context.Context) error {
		advanced = false
		// The row lock orders this recount after any message appended
		// concurrently, so the counter cannot miss it.
		cv, err := c.chat.GetConversationForUpdate(ctx, conversationID)
		if err != nil {
			return err
		}
		if !cv.IsParticipant(p.UserID) {
			return repository.ErrNotFound
		}
		var target int64
		if in.MessageID != nil {
			m, err := c.chat.GetMessage(ctx, *in.MessageID)
			if err != nil || m.ConversationID != cv.ID {
				return apperror.NotFound("Message")
			}
			target = m.Seq
		} else {
			target = *in.Seq
		}
		current := cv.ReadSeq(p.UserID)
		readSeq := max(current, min(target, cv.LastSeq))
		unread, err := c.chat.CountUnread(ctx, cv.ID, p.UserID, readSeq)
		if err != nil {
			return err
		}
		if err := c.chat.SetReadPosition(ctx, cv.ID, p.UserID == cv.PsychicID, readSeq, int(unread), c.clock.Now()); err != nil {
			return err
		}
		advanced = readSeq > current
		res = ReadResult{ConversationID: cv.ID, ReadSeq: readSeq, Unread: unread}
		conv = cv
		return nil
	})
	if err != nil {
		return nil, mapRepoErr(err, "Conversation")
	}
	if advanced {
		receipt := map[string]any{"conversation_id": conv.ID, "reader_id": p.UserID, "read_seq": res.ReadSeq}
		// read_at/message_id keep older clients (timestamp receipts) working.
		if rows, err := c.chat.ListMessagesAfterSeq(ctx, conv.ID, res.ReadSeq-1, 1); err == nil && len(rows) == 1 {
			receipt["message_id"], receipt["read_at"] = rows[0].ID, rows[0].CreatedAt
		}
		warnErr(ctx, "chat: publish read receipt", c.events.Publish(ctx, realtime.EventMessageRead, receipt, conv.Counterpart(p.UserID)))
		// revisions 3.0: chat unread — the reader's total moved on every tab.
		c.afterRead(ctx, p.UserID, p.UserID == conv.PsychicID, conv.ID, res.Unread)
	}
	return &res, nil
}

// Typing relays a typing indicator to the other participant (ephemeral,
// throttled server-side).
func (c *ChatService) Typing(ctx context.Context, p reqctx.Principal, sessionID uuid.UUID, typing bool) error {
	s, err := c.loadParticipantSession(ctx, p.UserID, false, sessionID)
	if err != nil {
		return err
	}
	if s.Status != domain.SessionActive {
		return nil
	}
	if typing {
		if res, err := c.limiter.Allow(ctx, p.UserID.String()+":"+sessionID.String(), c.cfg.TypingRule); err == nil && !res.Allowed {
			return nil
		}
	}
	return c.events.Ephemeral(ctx, realtime.EventTyping, map[string]any{
		"session_id": s.ID, "conversation_id": s.ConversationID, "user_id": p.UserID, "typing": typing,
	}, s.Counterpart(p.UserID))
}

// Inbox page bounds.
const (
	inboxPageDefault = 50
	inboxPageMax     = 50
)

// Conversations lists one page of the caller's inbox, newest activity first.
// Everything comes from one inbox query, one open-session query and one
// presence lookup, whatever the page size.
func (c *ChatService) Conversations(ctx context.Context, p reqctx.Principal, cursor string, limit int) (InboxPage, error) {
	cur, err := decodeCursor(cursor)
	if err != nil {
		return InboxPage{}, err
	}
	limit = clampPage(limit, inboxPageDefault, inboxPageMax)
	asPsychic := p.Role == domain.RolePsychic
	rows, err := c.chat.ListInbox(ctx, repository.InboxFilter{UserID: p.UserID, AsPsychic: asPsychic, Cursor: cur, Limit: limit + 1})
	if err != nil {
		return InboxPage{}, apperror.Internal(err)
	}
	page := InboxPage{HasMore: len(rows) > limit}
	if page.HasMore {
		rows = rows[:limit]
	}
	counterparts := make([]uuid.UUID, len(rows))
	for i := range rows {
		counterparts[i] = rows[i].ClientID
		if !asPsychic {
			counterparts[i] = rows[i].PsychicID
		}
	}
	online := map[uuid.UUID]bool{}
	if len(counterparts) > 0 {
		if m, err := c.presence.Online(ctx, counterparts); err == nil {
			online = m
		} else {
			warnErr(ctx, "chat: inbox presence", err)
		}
	}
	openByConv := map[uuid.UUID]*domain.Session{}
	busy := map[uuid.UUID]bool{}
	if asPsychic {
		// An advisor has at most one open session.
		if s, err := c.chat.OpenSessionForPsychic(ctx, p.UserID); err == nil {
			openByConv[s.ConversationID] = s
		} else if !isNotFound(err) {
			return InboxPage{}, apperror.Internal(err)
		}
	} else if len(counterparts) > 0 {
		open, err := c.chat.OpenSessionsForPsychics(ctx, counterparts)
		if err != nil {
			return InboxPage{}, apperror.Internal(err)
		}
		for i := range open {
			busy[open[i].PsychicID] = true
			if open[i].ClientID == p.UserID {
				openByConv[open[i].ConversationID] = &open[i]
			}
		}
	}
	ps := c.newParticipants()
	page.Items = make([]ConversationView, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		v := ConversationView{ID: r.ID, LastMessageAt: r.LastMessageAt, LastActivityAt: r.LastActivityAt, LastSeq: r.LastSeq,
			ReadSeq: r.ReadSeq(p.UserID), PeerReadSeq: r.ReadSeq(r.Counterpart(p.UserID)), Unread: int64(r.UnreadFor(p.UserID)), CreatedAt: r.CreatedAt}
		if asPsychic {
			v.Counterpart = ParticipantView{ID: r.ClientID, Role: string(domain.RoleClient), Name: firstNonEmpty(r.CounterpartNickname, r.CounterpartName, "Client"),
				AvatarURL: deref(r.CounterpartAvatarURL), Status: map[bool]string{true: "online", false: "offline"}[online[r.ClientID]]}
		} else {
			status := EffectiveStatus(domain.Availability(deref(r.CounterpartAvailability)), online[r.PsychicID], busy[r.PsychicID])
			v.Counterpart = ParticipantView{ID: r.PsychicID, Role: string(domain.RolePsychic), Name: firstNonEmpty(nil, r.CounterpartName, "Advisor"),
				AvatarURL: deref(r.CounterpartAvatarURL), Slug: deref(r.CounterpartSlug), Status: string(status)}
		}
		if r.LastBody != nil && r.LastMessageID != nil {
			preview := *r.LastBody
			if utf8.RuneCountInString(preview) > 140 {
				preview = string([]rune(preview)[:140]) + "…"
			}
			kind := domain.MessageText
			if r.LastKind != nil {
				kind = domain.MessageKind(*r.LastKind)
			}
			v.LastMessage = &MessageView{ID: *r.LastMessageID, ConversationID: r.ID, SenderID: r.LastSenderID, Kind: string(kind), Body: preview}
			if r.LastMsgSeq != nil {
				v.LastMessage.Seq = *r.LastMsgSeq
			}
			if r.LastMessageAt != nil {
				v.LastMessage.CreatedAt = *r.LastMessageAt
			}
		}
		if s := openByConv[r.ID]; s != nil {
			sv := c.sessionView(ctx, ps, s, viewer{ID: p.UserID})
			v.OpenSession = &sv
		}
		page.Items = append(page.Items, v)
	}
	if page.HasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.LastActivityAt, last.ID.String())
	}
	return page, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// firstNonEmpty returns the first non-empty value, then def.
func firstNonEmpty(a, b *string, def string) string {
	if a != nil && *a != "" {
		return *a
	}
	if b != nil && *b != "" {
		return *b
	}
	return def
}

// PsychicThread is the chatroom state for a client and one advisor.
type PsychicThread struct {
	Psychic      PsychicView       `json:"psychic"`
	Conversation *ConversationView `json:"conversation,omitempty"`
	OpenSession  *SessionView      `json:"open_session,omitempty"`
	ClientOpen   *SessionView      `json:"client_open_session,omitempty"`
}

// ThreadWithPsychic returns the client's conversation with an advisor (it
// may not exist yet) plus any open session.
func (c *ChatService) ThreadWithPsychic(ctx context.Context, p reqctx.Principal, slug string) (*PsychicThread, error) {
	pv, err := c.psychics.Get(ctx, slug)
	if err != nil {
		return nil, err
	}
	out := &PsychicThread{Psychic: *pv}
	ps := c.newParticipants()
	if conv, err := c.chat.FindConversation(ctx, p.UserID, pv.ID); err == nil {
		cv := ConversationView{ID: conv.ID, Counterpart: ps.psychic(ctx, conv.PsychicID), LastMessageAt: conv.LastMessageAt,
			LastActivityAt: conv.LastActivityAt, LastSeq: conv.LastSeq, ReadSeq: conv.ReadSeq(p.UserID),
			PeerReadSeq: conv.ReadSeq(conv.PsychicID), Unread: int64(conv.UnreadFor(p.UserID)), CreatedAt: conv.CreatedAt}
		cv.Counterpart.Status = pv.Status
		out.Conversation = &cv
		if open, err := c.chat.OpenSessionsForConversations(ctx, []uuid.UUID{conv.ID}); err == nil && len(open) > 0 {
			sv := c.sessionView(ctx, ps, &open[0], viewer{ID: p.UserID})
			out.OpenSession = &sv
			cv.OpenSession = &sv
		}
	} else if !isNotFound(err) {
		return nil, apperror.Internal(err)
	}
	if out.OpenSession == nil {
		if s, err := c.chat.OpenSessionForClient(ctx, p.UserID); err == nil {
			sv := c.sessionView(ctx, ps, s, viewer{ID: p.UserID})
			out.ClientOpen = &sv
		}
	}
	return out, nil
}

// MessagesQuery selects a page of history: the messages before BeforeSeq
// (the newest page when 0), or before the position of a legacy opaque
// cursor from an earlier page.
type MessagesQuery struct {
	BeforeSeq int64
	Cursor    string
	Limit     int
}

// Messages returns a page of conversation history (newest page first,
// items in chronological order).
func (c *ChatService) Messages(ctx context.Context, p reqctx.Principal, conversationID uuid.UUID, q MessagesQuery) (MessagePage, error) {
	conv, err := c.chat.GetConversation(ctx, conversationID)
	if err != nil {
		return MessagePage{}, mapRepoErr(err, "Conversation")
	}
	if !conv.IsParticipant(p.UserID) && !p.IsAdmin() {
		return MessagePage{}, apperror.NotFound("Conversation")
	}
	before := q.BeforeSeq
	if before == 0 && q.Cursor != "" {
		if before, err = c.cursorSeq(ctx, conv.ID, q.Cursor); err != nil {
			return MessagePage{}, err
		}
	}
	return c.messagesBefore(ctx, p.UserID, conv.ID, before, q.Limit)
}

// cursorSeq maps a legacy (created_at, id) cursor to the seq of the message
// it points at.
func (c *ChatService) cursorSeq(ctx context.Context, conversationID uuid.UUID, cursor string) (int64, error) {
	cur, err := decodeCursor(cursor)
	if err != nil || cur == nil {
		return 0, err
	}
	invalid := apperror.Validation(map[string]string{"cursor": "Invalid pagination cursor."})
	id, perr := uuid.Parse(cur.ID)
	if perr != nil {
		return 0, invalid
	}
	m, gerr := c.chat.GetMessage(ctx, id)
	if gerr != nil || m.ConversationID != conversationID {
		return 0, invalid
	}
	return m.Seq, nil
}

func (c *ChatService) messagesBefore(ctx context.Context, viewerID, conversationID uuid.UUID, beforeSeq int64, limit int) (MessagePage, error) {
	limit = clampPage(limit, c.cfg.HistoryPageSize, 200)
	rows, err := c.chat.ListMessagesBefore(ctx, conversationID, beforeSeq, limit+1)
	if err != nil {
		return MessagePage{}, apperror.Internal(err)
	}
	page := MessagePage{HasMore: len(rows) > limit}
	if page.HasMore {
		rows = rows[:limit]
	}
	items, err := c.messageViews(ctx, viewerID, rows, true)
	if err != nil {
		return MessagePage{}, err
	}
	page.Items = items
	if page.HasMore {
		oldest := rows[len(rows)-1]
		page.NextBeforeSeq = oldest.Seq
		page.NextCursor = repository.EncodeCursor(oldest.CreatedAt, oldest.ID.String())
	}
	return page, nil
}

// messageViews renders rows with the viewer's bookmarks and attachments,
// reversing newest-first rows into chronological order when reverse is set.
func (c *ChatService) messageViews(ctx context.Context, viewerID uuid.UUID, rows []domain.Message, reverse bool) ([]MessageView, error) {
	ids := make([]uuid.UUID, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	saved, err := c.chat.BookmarkedIDs(ctx, viewerID, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]MessageView, len(rows))
	for i := range rows {
		v := messageView(&rows[i])
		v.Saved = saved[v.ID]
		if reverse {
			out[len(rows)-1-i] = v
		} else {
			out[i] = v
		}
	}
	return c.withAttachments(ctx, out), nil
}

// messagesPage serves cursor-based history to administrators (transcripts).
func (c *ChatService) messagesPage(ctx context.Context, viewerID, conversationID uuid.UUID, before string, limit int) (Page[MessageView], error) {
	var beforeSeq int64
	if before != "" {
		seq, err := c.cursorSeq(ctx, conversationID, before)
		if err != nil {
			return Page[MessageView]{}, err
		}
		beforeSeq = seq
	}
	mp, err := c.messagesBefore(ctx, viewerID, conversationID, beforeSeq, limit)
	if err != nil {
		return Page[MessageView]{}, err
	}
	return Page[MessageView]{Items: mp.Items, NextCursor: mp.NextCursor}, nil
}

// SinceQuery selects messages after AfterSeq, or after a legacy timestamp.
type SinceQuery struct {
	AfterSeq int64
	After    *time.Time
	Limit    int
}

// MessagesSince returns messages after a sequence number, oldest first. It
// fills gaps after a reconnect or when a client sees a seq jump; HasMore
// tells the client to continue from the last item.
func (c *ChatService) MessagesSince(ctx context.Context, p reqctx.Principal, conversationID uuid.UUID, q SinceQuery) (SincePage, error) {
	conv, err := c.chat.GetConversation(ctx, conversationID)
	if err != nil {
		return SincePage{}, mapRepoErr(err, "Conversation")
	}
	if !conv.IsParticipant(p.UserID) {
		return SincePage{}, apperror.NotFound("Conversation")
	}
	after := q.AfterSeq
	if q.After != nil {
		if after, err = c.chat.SeqAtOrBefore(ctx, conv.ID, *q.After); err != nil {
			return SincePage{}, apperror.Internal(err)
		}
	}
	limit := clampPage(q.Limit, 200, 500)
	rows, err := c.chat.ListMessagesAfterSeq(ctx, conv.ID, after, limit+1)
	if err != nil {
		return SincePage{}, apperror.Internal(err)
	}
	page := SincePage{HasMore: len(rows) > limit, LastSeq: conv.LastSeq}
	if page.HasMore {
		rows = rows[:limit]
	}
	if page.Items, err = c.messageViews(ctx, p.UserID, rows, false); err != nil {
		return SincePage{}, err
	}
	if n := len(page.Items); n > 0 && page.Items[n-1].Seq > page.LastSeq {
		page.LastSeq = page.Items[n-1].Seq // messages appended after the head was read
	}
	return page, nil
}

// Session returns one session for a participant or admin.
func (c *ChatService) Session(ctx context.Context, p reqctx.Principal, id uuid.UUID) (*SessionView, error) {
	s, err := c.loadParticipantSession(ctx, p.UserID, p.IsAdmin(), id)
	if err != nil {
		return nil, err
	}
	v := c.sessionView(ctx, c.newParticipants(), s, viewer{ID: p.UserID, Admin: p.IsAdmin()})
	return &v, nil
}

// Summary returns the session recap.
func (c *ChatService) Summary(ctx context.Context, p reqctx.Principal, id uuid.UUID) (*SessionSummary, error) {
	s, err := c.loadParticipantSession(ctx, p.UserID, p.IsAdmin(), id)
	if err != nil {
		return nil, err
	}
	sum := c.summary(ctx, s, viewer{ID: p.UserID, Admin: p.IsAdmin()})
	return &sum, nil
}

// Sessions lists the caller's sessions — "past chats" (FR-12).
func (c *ChatService) Sessions(ctx context.Context, p reqctx.Principal, statuses []domain.SessionStatus, cursor string, limit int) (Page[SessionView], error) {
	cur, err := decodeCursor(cursor)
	if err != nil {
		return Page[SessionView]{}, err
	}
	f := repository.SessionFilter{Statuses: statuses, Cursor: cur, Limit: limit}
	switch p.Role {
	case domain.RolePsychic:
		f.PsychicID = &p.UserID
	default:
		f.ClientID = &p.UserID
	}
	return c.sessionsPage(ctx, f, viewer{ID: p.UserID})
}

func (c *ChatService) sessionsPage(ctx context.Context, f repository.SessionFilter, w viewer) (Page[SessionView], error) {
	rows, err := c.chat.ListSessions(ctx, f)
	if err != nil {
		return Page[SessionView]{}, apperror.Internal(err)
	}
	ps := c.newParticipants()
	page := Page[SessionView]{Items: make([]SessionView, 0, len(rows))}
	for i := range rows {
		page.Items = append(page.Items, c.sessionView(ctx, ps, &rows[i], w))
	}
	if len(rows) > 0 && len(rows) == clampPage(f.Limit, 20, 100) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.RequestedAt, last.ID.String())
	}
	return page, nil
}

// Bookmark saves or removes a message for the caller.
func (c *ChatService) Bookmark(ctx context.Context, p reqctx.Principal, messageID uuid.UUID, save bool) error {
	m, err := c.chat.GetMessage(ctx, messageID)
	if err != nil {
		return mapRepoErr(err, "Message")
	}
	conv, err := c.chat.GetConversation(ctx, m.ConversationID)
	if err != nil {
		return mapRepoErr(err, "Conversation")
	}
	if !conv.IsParticipant(p.UserID) {
		return apperror.NotFound("Message")
	}
	if !save {
		return mapRepoErr(c.chat.DeleteBookmark(ctx, p.UserID, messageID), "Bookmark")
	}
	if m.Kind != domain.MessageText && m.Kind != domain.MessageImage {
		return apperror.Validation(map[string]string{"message_id": "Only messages can be saved."})
	}
	if err := c.chat.SaveBookmark(ctx, &domain.SavedMessage{UserID: p.UserID, MessageID: messageID, CreatedAt: c.clock.Now()}); err != nil {
		return apperror.Internal(err)
	}
	c.tracker.Track(ctx, "chat.message_saved", nil)
	return nil
}

// Saved-messages page bounds. Without ?limit= a page holds up to 200, as
// the list did before it was paginated.
const (
	BookmarksPageDefault = 200
	BookmarksPageMax     = 200
)

// Bookmarks lists one page of saved messages, most recently saved first,
// optionally for one conversation. NextCursor is set only when another page
// exists (one extra row is fetched).
func (c *ChatService) Bookmarks(ctx context.Context, p reqctx.Principal, conversationID *uuid.UUID, cursor string, limit int) (Page[MessageView], error) {
	cur, err := decodeCursor(cursor)
	if err != nil {
		return Page[MessageView]{}, err
	}
	limit = clampPage(limit, BookmarksPageDefault, BookmarksPageMax)
	rows, err := c.chat.ListBookmarks(ctx, p.UserID, conversationID, cur, limit+1)
	if err != nil {
		return Page[MessageView]{}, apperror.Internal(err)
	}
	more := len(rows) > limit
	if more {
		rows = rows[:limit]
	}
	out := make([]MessageView, len(rows))
	for i := range rows {
		out[i] = messageView(&rows[i].Message)
		out[i].Saved = true
	}
	page := Page[MessageView]{Items: c.withAttachments(ctx, out)}
	if more {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.SavedAt, last.ID.String())
	}
	return page, nil
}

// errNotParticipant is returned when a connection is not owned by the caller.
var errNotParticipant = errors.New("service: connection does not belong to user")
