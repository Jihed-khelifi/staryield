package service

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/astro"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
)

// EventPresenceUpdated tells an advisor that a client in an open session
// connected or disconnected.
const EventPresenceUpdated = realtime.EventPresenceUpdated

// ChatExtras are optional collaborators added to live chat: private image
// attachments, review prompts, notifications and the advisor's view of a
// client.
type ChatExtras struct {
	Attachments repository.AttachmentRepository
	Store       storage.ObjectStore // private chat images (R2_CHAT_IMAGES_PRIVATE)
	Images      ImagePolicy
	Insights    repository.InsightsRepository
	Reviews     *ReviewService
	Notifier    Notifier
}

// WithExtras attaches the optional collaborators (composition root).
func (c *ChatService) WithExtras(x ChatExtras) *ChatService {
	if x.Notifier == nil {
		x.Notifier = NoopNotifier{}
	}
	c.extras = x
	return c
}

func (c *ChatService) notifier() Notifier {
	if c.extras.Notifier == nil {
		return NoopNotifier{}
	}
	return c.extras.Notifier
}

// AttachmentView describes an image attached to a message (metadata only;
// the bytes are served by GET /chat/attachments/:id to participants).
type AttachmentView struct {
	ID          uuid.UUID `json:"id"`
	ContentType string    `json:"content_type"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	Bytes       int       `json:"bytes"`
}

func attachmentViews(rows []domain.Attachment) []AttachmentView {
	out := make([]AttachmentView, 0, len(rows))
	for _, a := range rows {
		out = append(out, AttachmentView{ID: a.ID, ContentType: a.ContentType, Width: a.Width, Height: a.Height, Bytes: a.Bytes})
	}
	return out
}

// withAttachments fills image metadata on message views.
func (c *ChatService) withAttachments(ctx context.Context, views []MessageView) []MessageView {
	if c.extras.Attachments == nil {
		return views
	}
	ids := []uuid.UUID{}
	for _, v := range views {
		if v.Kind == string(domain.MessageImage) {
			ids = append(ids, v.ID)
		}
	}
	if len(ids) == 0 {
		return views
	}
	byMsg, err := c.extras.Attachments.ForMessages(ctx, ids)
	if err != nil {
		warnErr(ctx, "chat: load attachments", err)
		return views
	}
	for i := range views {
		if rows, ok := byMsg[views[i].ID]; ok {
			views[i].Attachments = attachmentViews(rows)
		}
	}
	return views
}

// UploadImageInput is an image message.
type UploadImageInput struct {
	SessionID   uuid.UUID
	Data        []byte
	Caption     string
	ClientMsgID string
}

// SendImage stores a private image in the chat bucket and posts it as a
// message in an active session. Only the two participants can ever fetch
// the image.
func (c *ChatService) SendImage(ctx context.Context, p reqctx.Principal, in UploadImageInput) (*MessageView, error) {
	if c.extras.Store == nil || c.extras.Attachments == nil {
		return nil, apperror.FeatureUnavailable("Image sharing is not available yet.")
	}
	if _, off := c.extras.Store.(storage.Disabled); off {
		return nil, apperror.FeatureUnavailable("Image sharing is not available yet.")
	}
	caption := CleanText(in.Caption)
	if utf8.RuneCountInString(caption) > c.cfg.MaxMessageLength {
		return nil, apperror.Validation(map[string]string{"caption": "Keep your message under the length limit."}).
			WithDetails(map[string]any{"max": c.cfg.MaxMessageLength})
	}
	var cmid *string
	if in.ClientMsgID != "" {
		if !clientMsgID.MatchString(in.ClientMsgID) {
			return nil, apperror.Validation(map[string]string{"client_msg_id": "Invalid message identifier."})
		}
		if existing, err := c.chat.GetMessageByClientID(ctx, p.UserID, in.ClientMsgID); err == nil {
			v := c.withAttachments(ctx, []MessageView{messageView(existing)})[0]
			return &v, nil
		} else if !isNotFound(err) {
			return nil, apperror.Internal(err)
		}
		cmid = &in.ClientMsgID
	}
	// Cheap checks before the expensive decode: the caller must be in an
	// active session.
	sess, err := c.loadParticipantSession(ctx, p.UserID, false, in.SessionID)
	if err != nil {
		return nil, err
	}
	if sess.Status != domain.SessionActive {
		return nil, apperror.InvalidState("This chat is not active.").WithDetails(map[string]any{"status": sess.Status})
	}
	if res, err := c.limiter.Allow(ctx, p.UserID.String(), c.cfg.MessageRule); err == nil && !res.Allowed {
		return nil, apperror.RateLimited(res.RetryAfter)
	} else if err != nil {
		warnErr(ctx, "chat: message rate limiter unavailable", err)
	}
	img, err := ProcessImage(in.Data, c.extras.Images)
	if err != nil {
		return nil, err
	}
	key := objectKey("chat/"+sess.ConversationID.String(), img.Extension())
	if _, err := c.extras.Store.Put(ctx, key, img.ContentType, img.Data, "private, no-store"); err != nil {
		return nil, apperror.Unavailable(err)
	}
	var msg *domain.Message
	var att *domain.Attachment
	var session *domain.Session
	err = c.tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := c.chat.GetSessionForUpdate(ctx, in.SessionID)
		if err != nil {
			return err
		}
		if s.Status != domain.SessionActive {
			return apperror.InvalidState("This chat is not active.").WithDetails(map[string]any{"status": s.Status})
		}
		now := c.clock.Now()
		sender := p.UserID
		m := &domain.Message{ID: domain.NewID(), ConversationID: s.ConversationID, SessionID: s.ID, SenderID: &sender,
			Kind: domain.MessageImage, Body: caption, ClientMsgID: cmid, CreatedAt: now}
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
		a := &domain.Attachment{ID: domain.NewID(), MessageID: m.ID, ConversationID: s.ConversationID, UploaderID: p.UserID,
			ObjectKey: key, ContentType: img.ContentType, Width: img.Width, Height: img.Height, Bytes: len(img.Data), CreatedAt: now}
		if err := c.extras.Attachments.Create(ctx, a); err != nil {
			return err
		}
		if err := c.chat.TouchConversation(ctx, s.ConversationID, m.ID, now); err != nil {
			return err
		}
		if err := c.chat.MarkConversationRead(ctx, s.ConversationID, p.UserID == s.PsychicID, now); err != nil {
			return err
		}
		msg, att, session = m, a, s
		return nil
	})
	if err != nil || att == nil {
		// Nothing references the object: remove it.
		warnErr(ctx, "chat: delete orphaned image", c.extras.Store.Delete(ctx, key))
	}
	if err != nil {
		return nil, mapRepoErr(err, "Session")
	}
	v := messageView(msg)
	if att != nil {
		v.Attachments = attachmentViews([]domain.Attachment{*att})
	} else {
		v = c.withAttachments(ctx, []MessageView{v})[0]
	}
	if session != nil {
		warnErr(ctx, "chat: publish image message", c.events.Publish(ctx, realtime.EventMessageCreated,
			map[string]any{"message": v}, session.ClientID, session.PsychicID))
		c.tracker.Track(ctx, "chat.image_sent", map[string]any{"session_id": session.ID, "bytes": len(img.Data), "role": string(p.Role)})
	}
	return &v, nil
}

// ErrAttachmentHidden hides attachments from anyone but the participants.
var ErrAttachmentHidden = apperror.NotFound("Attachment")

// Attachment returns a private image's bytes to a conversation participant
// (the sender or the receiver). Everyone else, administrators included,
// gets "not found".
func (c *ChatService) Attachment(ctx context.Context, p reqctx.Principal, id uuid.UUID) ([]byte, string, error) {
	if c.extras.Attachments == nil || c.extras.Store == nil {
		return nil, "", ErrAttachmentHidden
	}
	a, err := c.extras.Attachments.Get(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, "", ErrAttachmentHidden
		}
		return nil, "", apperror.Internal(err)
	}
	conv, err := c.chat.GetConversation(ctx, a.ConversationID)
	if err != nil || !conv.IsParticipant(p.UserID) {
		return nil, "", ErrAttachmentHidden
	}
	body, ct, err := c.extras.Store.Get(ctx, a.ObjectKey, int64(a.Bytes)+1024)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, "", ErrAttachmentHidden
		}
		return nil, "", apperror.Unavailable(err)
	}
	if ct == "" {
		ct = a.ContentType
	}
	return body, ct, nil
}

// ClientAstro is a client's astrological snapshot shown to their advisor.
type ClientAstro struct {
	SunSign    string `json:"sun_sign"`
	MoonSign   string `json:"moon_sign,omitempty"`
	RisingSign string `json:"rising_sign,omitempty"`
	LifePath   int    `json:"life_path,omitempty"`
	TimeKnown  bool   `json:"time_known"`
}

// ClientHoroscope is the client's daily reading.
type ClientHoroscope struct {
	Sign        string `json:"sign"`
	Date        string `json:"date"`
	Text        string `json:"text"`
	Focus       string `json:"focus"`
	Mood        string `json:"mood"`
	Affirmation string `json:"affirmation"`
}

// ClientHistoryView is a client's history with the viewing advisor.
type ClientHistoryView struct {
	Sessions       int64      `json:"sessions"`
	Minutes        int64      `json:"minutes"`
	FirstSessionAt *time.Time `json:"first_session_at,omitempty"`
	LastSessionAt  *time.Time `json:"last_session_at,omitempty"`
}

// ClientProfileView is what an advisor may know about a client. It never
// includes contact details, balances, purchases or the exact birth place
// and time.
type ClientProfileView struct {
	ID          uuid.UUID         `json:"id"`
	Name        string            `json:"name"`
	AvatarURL   string            `json:"avatar_url,omitempty"`
	Online      bool              `json:"online"`
	Locale      string            `json:"locale"`
	CountryCode string            `json:"country_code,omitempty"`
	Age         *int              `json:"age,omitempty"`
	MemberSince time.Time         `json:"member_since"`
	Astro       *ClientAstro      `json:"astro,omitempty"`
	Horoscope   *ClientHoroscope  `json:"horoscope,omitempty"`
	History     ClientHistoryView `json:"history"`
}

// ageOn returns completed years between birth and now.
func ageOn(birth, now time.Time) int {
	years := now.Year() - birth.Year()
	if now.Month() < birth.Month() || (now.Month() == birth.Month() && now.Day() < birth.Day()) {
		years--
	}
	return years
}

// ClientProfile returns the client card for the advisor of a conversation.
func (c *ChatService) ClientProfile(ctx context.Context, p reqctx.Principal, conversationID uuid.UUID) (*ClientProfileView, error) {
	conv, err := c.chat.GetConversation(ctx, conversationID)
	if err != nil {
		return nil, mapRepoErr(err, "Conversation")
	}
	if conv.PsychicID != p.UserID {
		return nil, apperror.NotFound("Conversation")
	}
	u, err := c.users.GetByID(ctx, conv.ClientID)
	if err != nil {
		return nil, mapRepoErr(err, "Conversation")
	}
	v := &ClientProfileView{ID: u.ID, Name: firstName("", u.DisplayName), Locale: u.Locale, MemberSince: u.CreatedAt}
	if u.Status == domain.UserDeleted {
		v.Name = "Client"
		return v, nil
	}
	now := c.clock.Now()
	if prof, err := c.users.GetProfile(ctx, u.ID); err == nil {
		if prof.Nickname != "" {
			v.Name = prof.Nickname
		}
		v.AvatarURL, v.CountryCode = prof.AvatarURL, prof.CountryCode
		if prof.BirthDate != nil {
			age := ageOn(*prof.BirthDate, now)
			v.Age = &age
			v.Astro = clientAstro(prof)
			if h, err := astro.Daily(v.Astro.SunSign, now); err == nil {
				v.Horoscope = &ClientHoroscope{Sign: h.Sign, Date: h.Date, Text: h.Text, Focus: h.Focus, Mood: h.Mood, Affirmation: h.Affirmation}
			}
		}
	}
	if online, err := c.presence.Online(ctx, []uuid.UUID{u.ID}); err == nil {
		v.Online = online[u.ID]
	}
	if c.extras.Insights != nil {
		if h, err := c.extras.Insights.ClientHistoryWith(ctx, u.ID, p.UserID); err == nil {
			v.History = ClientHistoryView{Sessions: h.Sessions, Minutes: h.Minutes, FirstSessionAt: h.FirstSessionAt, LastSessionAt: h.LastSessionAt}
		}
	}
	c.tracker.Track(ctx, "chat.client_profile_viewed", map[string]any{"conversation_id": conv.ID})
	return v, nil
}

// clientAstro computes sun, moon and rising signs. The moon sign needs the
// birth time (it changes sign every ~2.5 days) and the rising sign also the
// birth place; only what can be known is returned.
func clientAstro(prof *domain.Profile) *ClientAstro {
	b := *prof.BirthDate
	out := &ClientAstro{SunSign: astro.SunSignForDate(int(b.Month()), b.Day()), LifePath: astro.LifePath(b.Year(), int(b.Month()), b.Day()),
		TimeKnown: prof.BirthTime != ""}
	if prof.BirthTime == "" {
		return out
	}
	tz := "UTC"
	var place *astro.Place
	if prof.BirthPlaceID != "" {
		if pl, ok := astro.PlaceByID(prof.BirthPlaceID); ok {
			place, tz = &pl, pl.TimeZone
		}
	}
	instant, err := astro.LocalBirthInstant(b, prof.BirthTime, tz)
	if err != nil {
		return out
	}
	var loc *astro.GeoLocation
	if place != nil {
		l := place.Location()
		loc = &l
	}
	chart := astro.ComputeChart(instant, loc)
	out.MoonSign = chart.MoonSign
	out.RisingSign = chart.RisingSign
	return out
}

// PublishClientPresence tells the advisor of a client's open session that
// the client connected or disconnected (ephemeral).
func (c *ChatService) PublishClientPresence(ctx context.Context, clientID uuid.UUID) {
	s, err := c.chat.OpenSessionForClient(ctx, clientID)
	if err != nil {
		return
	}
	online, err := c.presence.Online(ctx, []uuid.UUID{clientID})
	if err != nil {
		return
	}
	warnErr(ctx, "chat: publish client presence", c.events.Ephemeral(ctx, EventPresenceUpdated,
		map[string]any{"user_id": clientID, "online": online[clientID], "conversation_id": s.ConversationID}, s.PsychicID))
}

// notifyReviewPrompt invites the client to rate a reading that started.
func (c *ChatService) notifyReviewPrompt(ctx context.Context, s *domain.Session) {
	if s.StartedAt == nil {
		return
	}
	ps := c.newParticipants().psychic(ctx, s.PsychicID)
	c.notifier().Notify(ctx, NotificationSpec{Kind: NotifyReviewPrompt,
		Data: map[string]any{"session_id": s.ID, "psychic_name": ps.Name, "psychic_slug": ps.Slug},
		Link: "/chatroom?reader=" + ps.Slug + "&review=" + s.ID.String()}, s.ClientID)
}
