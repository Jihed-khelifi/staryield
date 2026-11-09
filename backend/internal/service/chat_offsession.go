package service

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// revisions 3.3 (§22.3): messages outside a live chat ("off-session").
//
// A message written while the conversation has a live chat in status active
// or disconnected (paused) belongs to that session, exactly as before: never
// capped, never counted. Every other message is off-session: it is stored with
// session_id NULL, is free and is capped per sender, per conversation, per
// rolling 24 h.
//
// Gate order (the API is the only authority):
//  1. the caller participates in the conversation (else 404);
//  2. routing: a live session takes the message, no further checks;
//  3. the setting is enabled and the caller's role limit is > 0;
//  4. the conversation had at least one started live chat;
//  5. the other participant can receive messages;
//  6. the daily cap, unless free mode is active.
//
// The count and the insert happen in one transaction that holds the
// conversation row lock, so concurrent sends cannot exceed the cap. The lock
// order is session row first, then conversation row (as everywhere else); the
// off-session path never takes a session lock after the conversation lock.

// OffSessionAllowance is what the caller may still send outside a live chat.
type OffSessionAllowance struct {
	Live      bool       `json:"live"`      // a live chat (active or paused) exists: no cap
	Enabled   bool       `json:"enabled"`   // feature on and the caller's role limit > 0
	Eligible  bool       `json:"eligible"`  // the conversation had a started live chat
	Unlimited bool       `json:"unlimited"` // free mode: no cap
	Limit     int        `json:"limit"`     // the caller's cap per rolling 24 h (0 when disabled)
	Used      int        `json:"used"`      // the caller's off-session messages here in the last 24 h
	Remaining int        `json:"remaining"` // max(0, limit-used); limit when unlimited
	ResetsAt  *time.Time `json:"resets_at,omitempty"`
}

// SetOffSessionSettings installs the settings lookup (SiteService.OffSessionSettings).
// Without it the feature is disabled.
func (c *ChatService) SetOffSessionSettings(fn func(ctx context.Context) OffSessionSettings) {
	c.offSession = fn
}

func (c *ChatService) offSessionSettings(ctx context.Context) OffSessionSettings {
	if c.offSession == nil {
		return OffSessionSettings{}
	}
	return c.offSession(ctx)
}

func (c *ChatService) freeMode(ctx context.Context) bool {
	return c.freeModeActive != nil && c.freeModeActive(ctx)
}

func isLiveStatus(s domain.SessionStatus) bool {
	return s == domain.SessionActive || s == domain.SessionDisconnected
}

// liveSession returns the conversation's active or paused session, if any.
func (c *ChatService) liveSession(ctx context.Context, conversationID uuid.UUID) (*domain.Session, error) {
	open, err := c.chat.OpenSessionsForConversations(ctx, []uuid.UUID{conversationID})
	if err != nil {
		return nil, err
	}
	for i := range open {
		if isLiveStatus(open[i].Status) {
			return &open[i], nil
		}
	}
	return nil, nil
}

// offSessionState is the evaluated state of gates 3-6 for one caller.
type offSessionState struct {
	allowance OffSessionAllowance
	oldest    *time.Time
	// refusal is the first failing gate (nil when a message may be sent).
	refusal error
}

// evaluateOffSession evaluates gates 3-6 for the caller in the conversation.
// It never mutates anything. cfg and free are read by the caller (outside any
// transaction).
func (c *ChatService) evaluateOffSession(ctx context.Context, userID uuid.UUID, conv *domain.Conversation, cfg OffSessionSettings, free bool, now time.Time) (offSessionState, error) {
	asPsychic := userID == conv.PsychicID
	limit := cfg.LimitFor(asPsychic)
	st := offSessionState{allowance: OffSessionAllowance{Unlimited: free}}
	a := &st.allowance
	a.Enabled = cfg.Enabled && limit > 0
	if a.Enabled {
		a.Limit = limit
	}
	eligible, err := c.chat.HasStartedSession(ctx, conv.ID)
	if err != nil {
		return st, err
	}
	a.Eligible = eligible
	cnt, err := c.chat.CountOffSession(ctx, conv.ID, userID, now.Add(-domain.OffSessionWindow))
	if err != nil {
		return st, err
	}
	a.Used = int(cnt.Count)
	st.oldest = cnt.Oldest
	if cnt.Oldest != nil {
		r := cnt.Oldest.Add(domain.OffSessionWindow)
		a.ResetsAt = &r
	}
	a.Remaining = max(0, a.Limit-a.Used)
	if free {
		a.Remaining = a.Limit
	}
	switch {
	case !a.Enabled:
		st.refusal = apperror.OffSessionDisabled()
	case !a.Eligible:
		st.refusal = apperror.OffSessionNotEligible()
	default:
		ok, err := c.recipientCanReceive(ctx, conv.Counterpart(userID), userID == conv.ClientID)
		if err != nil {
			return st, err
		}
		if !ok {
			st.refusal = apperror.RecipientUnavailable()
		} else if !free && a.Used >= limit {
			st.refusal = apperror.OffSessionLimit(limit, a.Used, a.ResetsAt.UTC(), now)
		}
	}
	return st, nil
}

// recipientCanReceive reports whether the other participant's account is
// active (not deleted, banned or deactivated) and, for an advisor, still
// exists (advisors are soft-deleted when removed).
func (c *ChatService) recipientCanReceive(ctx context.Context, recipient uuid.UUID, recipientIsPsychic bool) (bool, error) {
	u, err := c.users.GetByID(ctx, recipient)
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if !u.Active() || u.DeletedAt != nil {
		return false, nil
	}
	if recipientIsPsychic {
		rows, err := c.psychicsRepo.GetByUserIDs(ctx, []uuid.UUID{recipient})
		if err != nil {
			return false, err
		}
		return len(rows) > 0, nil
	}
	return true, nil
}

// trackOffSessionRefusal records chat.off_session_refused for a refusal of
// the off-session gates (never the message text).
func (c *ChatService) trackOffSessionRefusal(ctx context.Context, p reqctx.Principal, conversationID uuid.UUID, err error) {
	var ae *apperror.Error
	if !errors.As(err, &ae) {
		return
	}
	reason := ""
	switch ae.Code {
	case apperror.CodeOffSessionDisabled:
		reason = "disabled"
	case apperror.CodeOffSessionNotEligible:
		reason = "not_eligible"
	case apperror.CodeRecipientUnavailable:
		reason = "recipient"
	case apperror.CodeOffSessionLimit:
		reason = "limit"
	default:
		return
	}
	c.tracker.Track(ctx, "chat.off_session_refused", map[string]any{"conversation_id": conversationID, "role": string(p.Role), "reason": reason})
}

// OffSessionAllowanceFor returns the caller's allowance in a conversation.
func (c *ChatService) OffSessionAllowanceFor(ctx context.Context, p reqctx.Principal, conversationID uuid.UUID) (*OffSessionAllowance, error) {
	conv, err := c.chat.GetConversation(ctx, conversationID)
	if err != nil {
		return nil, mapRepoErr(err, "Conversation")
	}
	if !conv.IsParticipant(p.UserID) {
		return nil, apperror.NotFound("Conversation")
	}
	a, err := c.allowanceFor(ctx, p.UserID, conv)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return a, nil
}

func (c *ChatService) allowanceFor(ctx context.Context, userID uuid.UUID, conv *domain.Conversation) (*OffSessionAllowance, error) {
	live, err := c.liveSession(ctx, conv.ID)
	if err != nil {
		return nil, err
	}
	st, err := c.evaluateOffSession(ctx, userID, conv, c.offSessionSettings(ctx), c.freeMode(ctx), c.clock.Now())
	if err != nil {
		return nil, err
	}
	a := st.allowance
	a.Live = live != nil
	return &a, nil
}

// SendResult is the outcome of a send: the stored message, the caller's
// allowance (nil only when it could not be computed) and whether the message
// already existed (an idempotent replay: HTTP 200, not 201).
type SendResult struct {
	Message   *MessageView         `json:"message"`
	Allowance *OffSessionAllowance `json:"allowance,omitempty"`
	Replayed  bool                 `json:"-"`
}

// ConversationSendInput is a text message addressed to a conversation.
type ConversationSendInput struct {
	ConversationID uuid.UUID
	Body           string
	ClientMsgID    string
}

// errReroute asks the caller to evaluate the routing again (a live chat
// appeared between the routing read and the conversation lock).
var errReroute = errors.New("service: reroute message")

// storeSpec describes what to store; attach adds the image attachment in the
// same transaction.
type storeSpec struct {
	kind   domain.MessageKind
	body   string
	cmid   *string
	attach func(ctx context.Context, m *domain.Message, now time.Time) (*domain.Attachment, error)
}

// stored is the committed outcome of storeMessage.
type stored struct {
	msg       *domain.Message
	att       *domain.Attachment
	conv      *domain.Conversation
	session   *domain.Session // nil for an off-session message
	replay    bool
	allowance *OffSessionAllowance // set for an off-session message
}

// storeMessage routes and stores one message (rules 1-6). It must be called
// after the idempotency lookup and the per-user rate limit. cfg and free are
// read once by the caller.
func (c *ChatService) storeMessage(ctx context.Context, p reqctx.Principal, conversationID uuid.UUID, spec storeSpec, cfg OffSessionSettings, free bool) (*stored, error) {
	var out *stored
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		out = &stored{}
		err = c.tx.WithinTx(ctx, func(ctx context.Context) error {
			*out = stored{}
			return c.storeOnce(ctx, p, conversationID, spec, cfg, free, out)
		})
		if !errors.Is(err, errReroute) {
			break
		}
	}
	if errors.Is(err, errReroute) {
		err = apperror.InvalidState("Please try again.")
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ChatService) storeOnce(ctx context.Context, p reqctx.Principal, conversationID uuid.UUID, spec storeSpec, cfg OffSessionSettings, free bool, out *stored) error {
	// 1. Participation (unlocked read: only participant ids are used here).
	peek, err := c.chat.GetConversation(ctx, conversationID)
	if err != nil {
		return err
	}
	if !peek.IsParticipant(p.UserID) {
		return repository.ErrNotFound
	}
	now := c.clock.Now()
	sender := p.UserID
	replayed := func() (bool, error) {
		if spec.cmid == nil {
			return false, nil
		}
		existing, err := c.chat.GetMessageByClientID(ctx, p.UserID, *spec.cmid)
		if err == nil {
			out.msg, out.replay, out.conv = existing, true, peek
			return true, nil
		}
		if !isNotFound(err) {
			return false, err
		}
		return false, nil
	}
	build := func(sessionID *uuid.UUID) (*domain.Message, error) {
		m := &domain.Message{ID: domain.NewID(), ConversationID: conversationID, SessionID: sessionID, SenderID: &sender,
			Kind: spec.kind, Body: spec.body, ClientMsgID: spec.cmid, CreatedAt: now, UpdatedAt: now}
		conv, err := c.chat.AppendMessage(ctx, m)
		if err != nil {
			return nil, err
		}
		out.conv = conv
		if spec.attach != nil {
			att, err := spec.attach(ctx, m, now)
			if err != nil {
				return nil, err
			}
			out.att = att
		}
		out.msg = m
		return m, nil
	}

	// 2. Routing: a live session (active or paused) takes the message. The
	// session row is locked first, then the conversation row by AppendMessage.
	live, err := c.liveSession(ctx, conversationID)
	if err != nil {
		return err
	}
	if live != nil {
		s, err := c.chat.GetSessionForUpdate(ctx, live.ID)
		if err != nil {
			return err
		}
		if isLiveStatus(s.Status) {
			if done, err := replayed(); done || err != nil {
				return err
			}
			if _, err := build(&s.ID); err != nil {
				return err
			}
			out.session = s
			return nil
		}
		// The chat ended since the read: continue off-session, still holding
		// the session lock (session before conversation).
	}

	// Off-session: lock the conversation, then count and insert under it.
	conv, err := c.chat.GetConversationForUpdate(ctx, conversationID)
	if err != nil {
		return err
	}
	if live == nil {
		// A chat may have gone live while we waited for the lock.
		if again, err := c.liveSession(ctx, conversationID); err != nil {
			return err
		} else if again != nil {
			return errReroute
		}
	}
	// Retries serialise on the conversation lock: an already stored message
	// is returned before any gate or count.
	if done, err := replayed(); done || err != nil {
		return err
	}
	st, err := c.evaluateOffSession(ctx, p.UserID, conv, cfg, free, now)
	if err != nil {
		return err
	}
	if st.refusal != nil {
		return st.refusal
	}
	if _, err := build(nil); err != nil {
		return err
	}
	a := st.allowance
	a.Used++
	if !free {
		a.Remaining = max(0, a.Limit-a.Used)
	}
	if a.ResetsAt == nil {
		r := now.Add(domain.OffSessionWindow)
		a.ResetsAt = &r
	}
	out.allowance = &a
	return nil
}

// finishSend publishes, tracks and builds the result of a committed send.
func (c *ChatService) finishSend(ctx context.Context, p reqctx.Principal, in *stored, view MessageView, kind string, length int) *SendResult {
	res := &SendResult{Message: &view, Replayed: in.replay}
	conv := in.conv
	if !in.replay {
		c.publishMessage(ctx, view, conv.ClientID, conv.PsychicID)
		if in.session != nil {
			props := map[string]any{"session_id": in.session.ID, "length": length, "role": string(p.Role)}
			if kind == string(domain.MessageImage) {
				c.tracker.Track(ctx, "chat.image_sent", map[string]any{"session_id": in.session.ID, "bytes": length, "role": string(p.Role)})
			} else {
				c.tracker.Track(ctx, "chat.message_sent", props)
			}
		} else {
			c.tracker.Track(ctx, "chat.off_session_message_sent", map[string]any{"conversation_id": conv.ID, "role": string(p.Role), "kind": kind, "length": length})
		}
	}
	res.Allowance = in.allowance
	if res.Allowance == nil && conv != nil {
		// Live-routed sends and replays: best effort, never fails the send.
		if a, err := c.allowanceFor(ctx, p.UserID, conv); err == nil {
			res.Allowance = a
		} else {
			warnErr(ctx, "chat: allowance", err)
		}
	}
	return res
}

// replayResult answers an idempotent replay with the original message.
func (c *ChatService) replayResult(ctx context.Context, p reqctx.Principal, existing *domain.Message, withAttachments bool) *SendResult {
	v := messageView(existing)
	if withAttachments {
		v = c.withAttachments(ctx, []MessageView{v})[0]
	}
	res := &SendResult{Message: &v, Replayed: true}
	if conv, err := c.chat.GetConversation(ctx, existing.ConversationID); err == nil && conv.IsParticipant(p.UserID) {
		if a, err := c.allowanceFor(ctx, p.UserID, conv); err == nil {
			res.Allowance = a
		}
	}
	return res
}

// SendToConversation stores and delivers a text message addressed to a
// conversation (rules of §22.3). Retries with the same client message ID
// return the original message and never count against the cap.
func (c *ChatService) SendToConversation(ctx context.Context, p reqctx.Principal, in ConversationSendInput) (*SendResult, error) {
	body, err := normalizeMessageBody(in.Body, c.cfg.MaxMessageLength)
	if err != nil {
		return nil, err
	}
	var cmid *string
	if in.ClientMsgID != "" {
		if !clientMsgID.MatchString(in.ClientMsgID) {
			return nil, apperror.Validation(map[string]string{"client_msg_id": "Invalid message identifier."})
		}
		// Before every gate and cap: a replay returns the original.
		if existing, err := c.chat.GetMessageByClientID(ctx, p.UserID, in.ClientMsgID); err == nil {
			return c.replayResult(ctx, p, existing, false), nil
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
	cfg, free := c.offSessionSettings(ctx), c.freeMode(ctx)
	out, err := c.storeMessage(ctx, p, in.ConversationID, storeSpec{kind: domain.MessageText, body: body, cmid: cmid}, cfg, free)
	if errors.Is(err, repository.ErrDuplicate) && cmid != nil {
		// Stored by a concurrent request (rolled back here): the original.
		existing, gerr := c.chat.GetMessageByClientID(ctx, p.UserID, *cmid)
		if gerr != nil {
			return nil, apperror.Internal(gerr)
		}
		return c.replayResult(ctx, p, existing, false), nil
	}
	if err != nil {
		c.trackOffSessionRefusal(ctx, p, in.ConversationID, err)
		return nil, mapRepoErr(err, "Conversation")
	}
	return c.finishSend(ctx, p, out, messageView(out.msg), string(domain.MessageText), utf8.RuneCountInString(body)), nil
}

// sessionConversation resolves the conversation of a session the caller
// participates in (legacy session-scoped endpoints and frames).
func (c *ChatService) sessionConversation(ctx context.Context, userID, sessionID uuid.UUID) (uuid.UUID, error) {
	s, err := c.loadParticipantSession(ctx, userID, false, sessionID)
	if err != nil {
		return uuid.Nil, err
	}
	return s.ConversationID, nil
}

// SendMessageForSession is the session-scoped send kept for old clients: the
// session only names the conversation, and the same routing applies, so a
// session that ended no longer rejects the send.
func (c *ChatService) SendMessageForSession(ctx context.Context, p reqctx.Principal, in SendMessageInput) (*SendResult, error) {
	convID, err := c.sessionConversation(ctx, p.UserID, in.SessionID)
	if err != nil {
		return nil, err
	}
	return c.SendToConversation(ctx, p, ConversationSendInput{ConversationID: convID, Body: in.Body, ClientMsgID: in.ClientMsgID})
}

// SendMessage stores and delivers a message for a session-scoped caller and
// returns only the message (see SendMessageForSession).
func (c *ChatService) SendMessage(ctx context.Context, p reqctx.Principal, in SendMessageInput) (*MessageView, error) {
	res, err := c.SendMessageForSession(ctx, p, in)
	if err != nil {
		return nil, err
	}
	return res.Message, nil
}
