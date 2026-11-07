package service

// revisions 3.0: chat unread
//
// The header chat badge: per-user unread totals (cached in Redis), live
// chat.unread events, and Web Push for messages that reach an offline
// recipient.

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/i18n"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// Chat unread settings.
const (
	// UnreadCacheTTL bounds how long a cached total can be stale (it is also
	// invalidated on every change).
	UnreadCacheTTL = 30 * time.Second
	// ChatPushWindow is the minimum time between two message pushes for one
	// conversation and recipient.
	ChatPushWindow = 60 * time.Second
	// PushKindChatMessage is the kind of a chat message push.
	PushKindChatMessage = "chat.message"

	unreadWorkers = 128
	unreadTimeout = 15 * time.Second
	// unreadBacklog bounds the background jobs waiting for a worker. Beyond
	// it, live frames and pushes are dropped (and counted): REST totals stay
	// exact because the cache is invalidated before the job is queued, and
	// clients refetch on reconnect and when the page becomes visible.
	unreadBacklog = 8192
)

// EventChatUnread is the ephemeral realtime event carrying unread totals.
const EventChatUnread = realtime.EventChatUnread

// UnreadView is the response of GET /chat/unread-count and the base of the
// chat.unread event payload.
type UnreadView struct {
	Count         int64 `json:"count"`
	Conversations int64 `json:"conversations"`
}

// chatPusher queues chat message pushes (implemented by *PushService).
type chatPusher interface {
	Enabled() bool
	QueueChatMessage(userID uuid.UUID, p PushPayload)
}

// chatUnread holds the optional collaborators of the unread feature.
type chatUnread struct {
	kv          cache.Store
	contacts    repository.ContactRepository
	push        chatPusher
	frontendURL string
	sem         chan struct{}
	pending     chan struct{}
	dropped     atomic.Int64
	wg          sync.WaitGroup
}

// WithUnread enables the unread cache and message push notifications
// (composition root). Any collaborator may be nil.
func (c *ChatService) WithUnread(kv cache.Store, contacts repository.ContactRepository, push chatPusher, frontendURL string) *ChatService {
	c.unread = &chatUnread{kv: kv, contacts: contacts, push: push, frontendURL: frontendURL,
		sem: make(chan struct{}, unreadWorkers), pending: make(chan struct{}, unreadBacklog)}
	return c
}

func unreadKey(userID uuid.UUID) string { return "chat:unread:" + userID.String() }

func chatPushKey(conversationID, userID uuid.UUID) string {
	return "chat:push:" + conversationID.String() + ":" + userID.String()
}

// UnreadCount returns the caller's unread message total and the number of
// conversations holding them. Cached per user for UnreadCacheTTL.
func (c *ChatService) UnreadCount(ctx context.Context, p reqctx.Principal) (UnreadView, error) {
	asPsychic := p.Role == domain.RolePsychic
	if p.Role != domain.RolePsychic && p.Role != domain.RoleClient {
		return UnreadView{}, apperror.Forbidden("You do not have access to this resource.")
	}
	var kv cache.Store
	if c.unread != nil {
		kv = c.unread.kv
	}
	if kv != nil {
		if raw, ok, err := kv.Get(ctx, unreadKey(p.UserID)); err != nil {
			warnErr(ctx, "chat: unread cache read", err)
		} else if ok {
			if v, ok := parseUnread(raw); ok {
				return v, nil
			}
		}
	}
	v, err := c.unreadFromDB(ctx, p.UserID, asPsychic)
	if err != nil {
		return UnreadView{}, apperror.Internal(err)
	}
	if kv != nil {
		warnErr(ctx, "chat: unread cache write", kv.Set(ctx, unreadKey(p.UserID), strconv.FormatInt(v.Count, 10)+":"+strconv.FormatInt(v.Conversations, 10), UnreadCacheTTL))
	}
	return v, nil
}

func parseUnread(raw string) (UnreadView, bool) {
	a, b, ok := strings.Cut(raw, ":")
	if !ok {
		return UnreadView{}, false
	}
	n, e1 := strconv.ParseInt(a, 10, 64)
	m, e2 := strconv.ParseInt(b, 10, 64)
	if e1 != nil || e2 != nil || n < 0 || m < 0 {
		return UnreadView{}, false
	}
	return UnreadView{Count: n, Conversations: m}, true
}

func (c *ChatService) unreadFromDB(ctx context.Context, userID uuid.UUID, asPsychic bool) (UnreadView, error) {
	n, convs, err := c.chat.UnreadTotals(ctx, userID, asPsychic)
	return UnreadView{Count: n, Conversations: convs}, err
}

// invalidateUnread drops the cached totals of users (synchronously, so a read
// right after the change never sees the old total).
func (c *ChatService) invalidateUnread(ctx context.Context, userIDs ...uuid.UUID) {
	if c.unread == nil || c.unread.kv == nil || len(userIDs) == 0 {
		return
	}
	keys := make([]string, len(userIDs))
	for i, id := range userIDs {
		keys[i] = unreadKey(id)
	}
	warnErr(ctx, "chat: unread cache invalidate", c.unread.kv.Del(ctx, keys...))
}

// publishUnread sends the user's fresh totals to every one of their
// connections. conversationUnread is the unread count of conversationID.
func (c *ChatService) publishUnread(ctx context.Context, userID uuid.UUID, asPsychic bool, conversationID uuid.UUID, conversationUnread int64) {
	v, err := c.unreadFromDB(ctx, userID, asPsychic)
	if err != nil {
		warnErr(ctx, "chat: unread totals", err)
		return
	}
	c.emitUnread(ctx, userID, v, conversationID, conversationUnread)
}

func (c *ChatService) emitUnread(ctx context.Context, userID uuid.UUID, v UnreadView, conversationID uuid.UUID, conversationUnread int64) {
	warnErr(ctx, "chat: publish unread", c.events.Ephemeral(ctx, realtime.EventChatUnread, map[string]any{
		"count": v.Count, "conversations": v.Conversations, "conversation_id": conversationID, "conversation_unread": conversationUnread,
	}, userID))
}

// afterRead runs after a read position moved: the reader's total changed on
// every one of their tabs and devices.
func (c *ChatService) afterRead(ctx context.Context, userID uuid.UUID, asPsychic bool, conversationID uuid.UUID, unread int64) {
	c.invalidateUnread(ctx, userID)
	c.publishUnread(ctx, userID, asPsychic, conversationID, unread)
}

// onMessageStored runs after a message was committed and published. The
// cache is invalidated before the request returns; the totals event and the
// push are produced in the background so the sender never waits for them.
// Call sites pass the conversation's participants as (client, psychic).
func (c *ChatService) onMessageStored(ctx context.Context, v MessageView, clientID, psychicID uuid.UUID) {
	u := c.unread
	if u == nil {
		return
	}
	system := v.Kind == string(domain.MessageSystem) || v.SenderID == nil
	type target struct {
		id        uuid.UUID
		asPsychic bool
		sender    bool
		prev      UnreadView
		hadPrev   bool
	}
	var targets []target
	switch {
	case system:
		// System notices count as unread for both participants.
		targets = []target{{id: clientID}, {id: psychicID, asPsychic: true}}
	case *v.SenderID == clientID:
		// The recipient gains an unread message; the sender has read
		// everything up to their own message, so their total may drop.
		targets = []target{{id: psychicID, asPsychic: true}, {id: clientID, sender: true}}
	case *v.SenderID == psychicID:
		targets = []target{{id: clientID}, {id: psychicID, asPsychic: true, sender: true}}
	default:
		return
	}
	for i := range targets {
		t := &targets[i]
		if t.sender && u.kv != nil {
			// The sender's event is only worth sending when their total moved.
			if raw, ok, err := u.kv.Get(ctx, unreadKey(t.id)); err == nil && ok {
				t.prev, t.hadPrev = parseUnread(raw)
			}
		}
		c.invalidateUnread(ctx, t.id)
	}
	bg := context.WithoutCancel(ctx)
	select {
	case u.pending <- struct{}{}:
	default:
		if n := u.dropped.Add(1); n == 1 || n%1000 == 0 {
			logger.FromContext(ctx).Warn("chat: unread backlog full, dropping live update", zap.Int64("dropped_total", n))
		}
		return
	}
	u.wg.Add(1)
	go func() {
		defer u.wg.Done()
		defer func() { <-u.pending }()
		u.sem <- struct{}{}
		defer func() { <-u.sem }()
		bg, cancel := context.WithTimeout(bg, unreadTimeout)
		defer cancel()
		conv, convErr := c.chat.GetConversation(bg, v.ConversationID)
		for _, t := range targets {
			now, err := c.unreadFromDB(bg, t.id, t.asPsychic)
			if err != nil {
				warnErr(bg, "chat: unread totals", err)
				continue
			}
			if t.sender && t.hadPrev && t.prev == now {
				continue
			}
			per := int64(0)
			if convErr == nil {
				per = int64(conv.UnreadFor(t.id))
			}
			c.emitUnread(bg, t.id, now, v.ConversationID, per)
		}
		if !system && u.push != nil && u.push.Enabled() {
			c.pushMessage(bg, v, targets[0].id, targets[0].asPsychic, clientID, psychicID)
		}
	}()
}

// FlushUnread waits for background unread work (tests and shutdown).
func (c *ChatService) FlushUnread() {
	if c.unread != nil {
		c.unread.wg.Wait()
	}
}

// pushMessage queues a Web Push for the recipient of a stored message when
// they have no open realtime connection, their push switch is on and no push
// went out for this conversation in the last ChatPushWindow.
func (c *ChatService) pushMessage(ctx context.Context, v MessageView, recipient uuid.UUID, asPsychic bool, clientID, psychicID uuid.UUID) {
	u := c.unread
	if online, err := c.presence.Online(ctx, []uuid.UUID{recipient}); err != nil {
		warnErr(ctx, "chat: push presence", err)
	} else if online[recipient] {
		return
	}
	if u.contacts == nil {
		return
	}
	contacts, err := u.contacts.Contacts(ctx, []uuid.UUID{recipient})
	if err != nil {
		warnErr(ctx, "chat: push recipient", err)
		return
	}
	if len(contacts) != 1 || contacts[0].Status != domain.UserActive || !contacts[0].NotifyPush {
		return
	}
	if u.kv != nil {
		n, err := u.kv.Incr(ctx, chatPushKey(v.ConversationID, recipient), ChatPushWindow)
		if err != nil {
			// Without the throttle, skipping is safer than flooding.
			warnErr(ctx, "chat: push throttle", err)
			return
		}
		if n > 1 {
			return
		}
	}
	name, slug := c.pushSenderIdentity(ctx, asPsychic, clientID, psychicID)
	u.push.QueueChatMessage(recipient, ChatMessagePush(u.frontendURL, contacts[0].Locale, name, slug, asPsychic, v.ConversationID))
}

// pushSenderIdentity returns the sender's display name (advisor public name
// for clients; client nickname or name for advisors) and the advisor's slug.
func (c *ChatService) pushSenderIdentity(ctx context.Context, recipientIsPsychic bool, clientID, psychicID uuid.UUID) (name, slug string) {
	if recipientIsPsychic {
		if profiles, err := c.users.GetProfiles(ctx, []uuid.UUID{clientID}); err == nil {
			name = profiles[clientID].Nickname
		}
		if name == "" {
			if us, err := c.users.GetByIDs(ctx, []uuid.UUID{clientID}); err == nil && len(us) == 1 {
				name = us[0].DisplayName
			}
		}
		return CleanLine(name), ""
	}
	if ps, err := c.psychicsRepo.GetByUserID(ctx, psychicID); err == nil {
		return CleanLine(ps.Name), ps.Slug
	}
	return "", ""
}

// ChatMessagePush builds the Web Push for a new chat message. It carries the
// sender's name only: the message text never leaves the chat.
func ChatMessagePush(frontendURL, locale, senderName, advisorSlug string, recipientIsPsychic bool, conversationID uuid.UUID) PushPayload {
	locale = i18n.Normalize(locale)
	senderName = truncateString(CleanLine(senderName), 80)
	title := i18n.T(locale, "New message")
	if senderName != "" {
		title = i18n.T(locale, fmt.Sprintf("New message from %s", senderName))
	}
	path := "/chatroom"
	if !recipientIsPsychic && advisorSlug != "" {
		path += "?reader=" + url.QueryEscape(advisorSlug)
	}
	return PushPayload{Kind: PushKindChatMessage, Title: title, Body: i18n.T(locale, "Open the chatroom to read it."),
		URL: mail.AppURL(frontendURL, locale, path), Tag: "chat:" + conversationID.String()}
}
