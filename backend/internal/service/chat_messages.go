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

// SendMessageInput is a chat message intent.
type SendMessageInput struct {
	SessionID   uuid.UUID
	Body        string
	ClientMsgID string
}

// SendMessage stores and delivers a message in an Active session. Retries
// with the same client message ID return the original message.
func (c *ChatService) SendMessage(ctx context.Context, p reqctx.Principal, in SendMessageInput) (*MessageView, error) {
	body := CleanText(in.Body)
	if body == "" {
		return nil, apperror.Validation(map[string]string{"body": "Write something before sending."})
	}
	if utf8.RuneCountInString(body) > c.cfg.MaxMessageLength {
		return nil, apperror.Validation(map[string]string{"body": "Keep your message under the length limit."}).
			WithDetails(map[string]any{"max": c.cfg.MaxMessageLength})
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
	err := c.tx.WithinTx(ctx, func(ctx context.Context) error {
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
		now := c.clock.Now()
		sender := p.UserID
		m := &domain.Message{ID: domain.NewID(), ConversationID: s.ConversationID, SessionID: s.ID, SenderID: &sender,
			Kind: domain.MessageText, Body: body, ClientMsgID: cmid, CreatedAt: now}
		created, err := c.chat.CreateMessage(ctx, m)
		if err != nil {
			return err
		}
		if !created {
			existing, err := c.chat.GetMessageByClientID(ctx, p.UserID, *cmid)
			if err != nil {
				return err
			}
			msg = existing
			return nil
		}
		if err := c.chat.TouchConversation(ctx, s.ConversationID, m.ID, now); err != nil {
			return err
		}
		// The sender has implicitly read everything up to their own message.
		if err := c.chat.MarkConversationRead(ctx, s.ConversationID, p.UserID == s.PsychicID, now); err != nil {
			return err
		}
		msg, session = m, s
		return nil
	})
	if err != nil {
		return nil, mapRepoErr(err, "Session")
	}
	v := messageView(msg)
	if session != nil {
		warnErr(ctx, "chat: publish message", c.events.Publish(ctx, realtime.EventMessageCreated,
			map[string]any{"message": v}, session.ClientID, session.PsychicID))
		c.tracker.Track(ctx, "chat.message_sent", map[string]any{"session_id": session.ID, "length": utf8.RuneCountInString(body), "role": string(p.Role)})
	}
	return &v, nil
}

// MarkRead advances the caller's read marker to messageID and sends a read
// receipt to the other participant.
func (c *ChatService) MarkRead(ctx context.Context, p reqctx.Principal, conversationID, messageID uuid.UUID) error {
	conv, err := c.chat.GetConversation(ctx, conversationID)
	if err != nil {
		return mapRepoErr(err, "Conversation")
	}
	if !conv.IsParticipant(p.UserID) {
		return apperror.NotFound("Conversation")
	}
	m, err := c.chat.GetMessage(ctx, messageID)
	if err != nil || m.ConversationID != conv.ID {
		return apperror.NotFound("Message")
	}
	if err := c.chat.MarkConversationRead(ctx, conv.ID, p.UserID == conv.PsychicID, m.CreatedAt); err != nil {
		return apperror.Internal(err)
	}
	warnErr(ctx, "chat: publish read receipt", c.events.Publish(ctx, realtime.EventMessageRead, map[string]any{
		"conversation_id": conv.ID, "reader_id": p.UserID, "message_id": m.ID, "read_at": m.CreatedAt,
	}, conv.Counterpart(p.UserID)))
	return nil
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

// Conversations lists the caller's inbox, newest first.
func (c *ChatService) Conversations(ctx context.Context, p reqctx.Principal, limit int) ([]ConversationView, error) {
	asPsychic := p.Role == domain.RolePsychic
	rows, err := c.chat.ListConversations(ctx, p.UserID, asPsychic, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	open, err := c.chat.OpenSessionsForConversations(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	openBy := map[uuid.UUID]*domain.Session{}
	for i := range open {
		openBy[open[i].ConversationID] = &open[i]
	}
	ps := c.newParticipants()
	// Advisors see whether each client is connected right now.
	online := map[uuid.UUID]bool{}
	if asPsychic && len(rows) > 0 {
		clients := make([]uuid.UUID, len(rows))
		for i := range rows {
			clients[i] = rows[i].ClientID
		}
		if m, err := c.presence.Online(ctx, clients); err == nil {
			online = m
		} else {
			warnErr(ctx, "chat: client presence", err)
		}
	}
	out := make([]ConversationView, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		v := ConversationView{ID: r.ID, LastMessageAt: r.LastMessageAt, Unread: r.Unread, CreatedAt: r.CreatedAt}
		if asPsychic {
			v.Counterpart = ps.client(ctx, r.ClientID)
			v.Counterpart.Status = map[bool]string{true: "online", false: "offline"}[online[r.ClientID]]
		} else {
			v.Counterpart = ps.psychic(ctx, r.PsychicID)
			if st, _, err := c.psychics.Status(ctx, r.PsychicID); err == nil {
				v.Counterpart.Status = string(st)
			}
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
			if r.LastMessageAt != nil {
				v.LastMessage.CreatedAt = *r.LastMessageAt
			}
		}
		if s := openBy[r.ID]; s != nil {
			sv := c.sessionView(ctx, ps, s, viewer{ID: p.UserID})
			v.OpenSession = &sv
		}
		out = append(out, v)
	}
	return out, nil
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
		cv := ConversationView{ID: conv.ID, Counterpart: ps.psychic(ctx, conv.PsychicID), LastMessageAt: conv.LastMessageAt, CreatedAt: conv.CreatedAt}
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

// Messages returns a page of conversation history (newest page first,
// items in chronological order). before is a cursor from a previous page.
func (c *ChatService) Messages(ctx context.Context, p reqctx.Principal, conversationID uuid.UUID, before string, limit int) (Page[MessageView], error) {
	conv, err := c.chat.GetConversation(ctx, conversationID)
	if err != nil {
		return Page[MessageView]{}, mapRepoErr(err, "Conversation")
	}
	if !conv.IsParticipant(p.UserID) && !p.IsAdmin() {
		return Page[MessageView]{}, apperror.NotFound("Conversation")
	}
	return c.messagesPage(ctx, p.UserID, conversationID, before, limit)
}

func (c *ChatService) messagesPage(ctx context.Context, viewerID, conversationID uuid.UUID, before string, limit int) (Page[MessageView], error) {
	cur, err := decodeCursor(before)
	if err != nil {
		return Page[MessageView]{}, err
	}
	limit = clampPage(limit, c.cfg.HistoryPageSize, 200)
	rows, err := c.chat.ListMessages(ctx, conversationID, cur, limit)
	if err != nil {
		return Page[MessageView]{}, apperror.Internal(err)
	}
	ids := make([]uuid.UUID, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	saved, err := c.chat.BookmarkedIDs(ctx, viewerID, ids)
	if err != nil {
		return Page[MessageView]{}, apperror.Internal(err)
	}
	page := Page[MessageView]{Items: make([]MessageView, len(rows))}
	for i := range rows {
		v := messageView(&rows[i])
		v.Saved = saved[v.ID]
		page.Items[len(rows)-1-i] = v // chronological order
	}
	if len(rows) == limit {
		oldest := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(oldest.CreatedAt, oldest.ID.String())
	}
	page.Items = c.withAttachments(ctx, page.Items)
	return page, nil
}

// MessagesSince returns messages after a timestamp, used to fill gaps after
// a reconnect when the event stream could not be replayed.
func (c *ChatService) MessagesSince(ctx context.Context, p reqctx.Principal, conversationID uuid.UUID, after time.Time) ([]MessageView, error) {
	conv, err := c.chat.GetConversation(ctx, conversationID)
	if err != nil {
		return nil, mapRepoErr(err, "Conversation")
	}
	if !conv.IsParticipant(p.UserID) {
		return nil, apperror.NotFound("Conversation")
	}
	rows, err := c.chat.ListMessagesAfter(ctx, conversationID, after, 500)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]MessageView, len(rows))
	for i := range rows {
		out[i] = messageView(&rows[i])
	}
	return c.withAttachments(ctx, out), nil
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

// Bookmarks lists saved messages, optionally for one conversation.
func (c *ChatService) Bookmarks(ctx context.Context, p reqctx.Principal, conversationID *uuid.UUID) ([]MessageView, error) {
	rows, err := c.chat.ListBookmarks(ctx, p.UserID, conversationID, 200)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]MessageView, len(rows))
	for i := range rows {
		out[i] = messageView(&rows[i])
		out[i].Saved = true
	}
	return c.withAttachments(ctx, out), nil
}

// errNotParticipant is returned when a connection is not owned by the caller.
var errNotParticipant = errors.New("service: connection does not belong to user")
