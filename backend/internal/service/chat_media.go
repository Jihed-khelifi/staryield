package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/astro"
	"github.com/Jihed-khelifi/staryield/backend/internal/astro/horoscope"
	"github.com/Jihed-khelifi/staryield/backend/internal/astro/tz"
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

// UploadImageInput is an image message addressed to a session (legacy,
// old clients) or, with ConversationID set, to a conversation.
type UploadImageInput struct {
	SessionID      uuid.UUID
	ConversationID uuid.UUID
	Data           []byte
	Caption        string
	ClientMsgID    string
}

// SendImage is the session-scoped image send kept for old clients: the
// session only names the conversation (same routing as SendImageToConversation).
func (c *ChatService) SendImage(ctx context.Context, p reqctx.Principal, in UploadImageInput) (*MessageView, error) {
	res, err := c.SendImageForSession(ctx, p, in)
	if err != nil {
		return nil, err
	}
	return res.Message, nil
}

// SendImageForSession resolves the session's conversation, then sends.
func (c *ChatService) SendImageForSession(ctx context.Context, p reqctx.Principal, in UploadImageInput) (*SendResult, error) {
	convID, err := c.sessionConversation(ctx, p.UserID, in.SessionID)
	if err != nil {
		return nil, err
	}
	in.ConversationID = convID
	return c.SendImageToConversation(ctx, p, in)
}

// SendImageToConversation stores a private image in the chat bucket and
// posts it as a message of the conversation, with the routing and the gates
// of §22.3. Gates 1-6 run before the image is decoded or stored (cheap,
// unlocked pre-check) and again inside the transaction. Only the two
// participants can ever fetch the image.
func (c *ChatService) SendImageToConversation(ctx context.Context, p reqctx.Principal, in UploadImageInput) (*SendResult, error) {
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
		// Before every gate and cap: a replay returns the original.
		if existing, err := c.chat.GetMessageByClientID(ctx, p.UserID, in.ClientMsgID); err == nil {
			return c.replayResult(ctx, p, existing, true), nil
		} else if !isNotFound(err) {
			return nil, apperror.Internal(err)
		}
		cmid = &in.ClientMsgID
	}
	cfg, free := c.offSessionSettings(ctx), c.freeMode(ctx)
	// Cheap checks before the expensive decode and the upload.
	conv, err := c.chat.GetConversation(ctx, in.ConversationID)
	if err != nil {
		return nil, mapRepoErr(err, "Conversation")
	}
	if !conv.IsParticipant(p.UserID) {
		return nil, apperror.NotFound("Conversation")
	}
	live, err := c.liveSession(ctx, conv.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if live == nil {
		st, err := c.evaluateOffSession(ctx, p.UserID, conv, cfg, free, c.clock.Now())
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if st.refusal != nil {
			c.trackOffSessionRefusal(ctx, p, conv.ID, st.refusal)
			return nil, st.refusal
		}
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
	key := objectKey("chat/"+conv.ID.String(), img.Extension())
	if _, err := c.extras.Store.Put(ctx, key, img.ContentType, img.Data, "private, no-store"); err != nil {
		return nil, apperror.Unavailable(err)
	}
	spec := storeSpec{kind: domain.MessageImage, body: caption, cmid: cmid,
		attach: func(ctx context.Context, m *domain.Message, now time.Time) (*domain.Attachment, error) {
			a := &domain.Attachment{ID: domain.NewID(), MessageID: m.ID, ConversationID: m.ConversationID, UploaderID: p.UserID,
				ObjectKey: key, ContentType: img.ContentType, Width: img.Width, Height: img.Height, Bytes: len(img.Data), CreatedAt: now}
			return a, c.extras.Attachments.Create(ctx, a)
		}}
	out, err := c.storeMessage(ctx, p, conv.ID, spec, cfg, free)
	if err != nil || out.att == nil {
		// Nothing references the object (refused, replayed or failed): remove it.
		warnErr(ctx, "chat: delete orphaned image", c.extras.Store.Delete(ctx, key))
	}
	if errors.Is(err, repository.ErrDuplicate) && cmid != nil {
		// Stored by a concurrent request: return the original message.
		existing, gerr := c.chat.GetMessageByClientID(ctx, p.UserID, *cmid)
		if gerr != nil {
			return nil, apperror.Internal(gerr)
		}
		return c.replayResult(ctx, p, existing, true), nil
	}
	if err != nil {
		c.trackOffSessionRefusal(ctx, p, conv.ID, err)
		return nil, mapRepoErr(err, "Conversation")
	}
	v := messageView(out.msg)
	if out.att != nil {
		v.Attachments = attachmentViews([]domain.Attachment{*out.att})
	} else {
		v = c.withAttachments(ctx, []MessageView{v})[0]
	}
	return c.finishSend(ctx, p, out, v, string(domain.MessageImage), len(img.Data)), nil
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
	SunSign          string `json:"sun_sign"`
	SunSignUncertain bool   `json:"sun_sign_uncertain"`
	MoonSign         string `json:"moon_sign,omitempty"`
	RisingSign       string `json:"rising_sign,omitempty"`
	LifePath         int    `json:"life_path,omitempty"`
	TimeKnown        bool   `json:"time_known"`
	// Placements are the natal planets' signs. Degrees and houses are only
	// given when the birth time is known (and houses with a birthplace);
	// the Moon is left out when its sign is uncertain.
	Placements []ClientPlacement `json:"placements,omitempty"`
}

// ClientPlacement is one natal planet in the client card.
type ClientPlacement struct {
	Body       string `json:"body"`
	Sign       string `json:"sign"`
	Degree     *int   `json:"degree,omitempty"`
	House      int    `json:"house,omitempty"`
	Retrograde bool   `json:"retrograde"`
}

// ClientBirthDetails are shown to advisors only when the client turned on
// share_birth_details_with_advisors (off by default); that choice now
// governs the birth city only, since the date of birth is always shown
// (ClientProfileView.DateOfBirth, revisions 3.3). They never include the
// birth time or coordinates.
type ClientBirthDetails struct {
	DateOfBirth string `json:"date_of_birth"`
	Place       string `json:"place,omitempty"`
}

// ClientHoroscope is the client's daily reading (in the advisor's language).
type ClientHoroscope struct {
	Sign        string `json:"sign"`
	SignName    string `json:"sign_name"`
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

// ClientProfileView is what an advisor may know about a client. It always
// includes the date of birth when known (readings rely on it; not for
// deleted accounts), and never contact details, balances, purchases or the
// exact birth place and time. The birth city is opt-in.
type ClientProfileView struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	Online      bool      `json:"online"`
	Locale      string    `json:"locale"`
	CountryCode string    `json:"country_code,omitempty"`
	Age         *int      `json:"age,omitempty"`
	// DateOfBirth (YYYY-MM-DD) is present whenever the client's birth date
	// is known, whatever the sharing choice.
	DateOfBirth string       `json:"date_of_birth,omitempty"`
	MemberSince time.Time    `json:"member_since"`
	Astro       *ClientAstro `json:"astro,omitempty"`
	// BirthDetailsShared tells the advisor whether the client chose to share
	// their birth city (BirthDetails, with the place, is set only then).
	BirthDetailsShared bool                `json:"birth_details_shared"`
	BirthDetails       *ClientBirthDetails `json:"birth_details,omitempty"`
	Horoscope          *ClientHoroscope    `json:"horoscope,omitempty"`
	History            ClientHistoryView   `json:"history"`
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
			v.DateOfBirth = prof.BirthDate.Format("2006-01-02")
			v.Astro = clientAstro(prof)
			if prof.ShareBirthDetailsWithAdvisors {
				place := strings.TrimSpace(prof.BirthPlaceLabel)
				if place == "" {
					place = strings.TrimSpace(prof.BirthPlace)
				}
				v.BirthDetailsShared = true
				v.BirthDetails = &ClientBirthDetails{DateOfBirth: prof.BirthDate.Format("2006-01-02"), Place: place}
			}
			if v.Astro.SunSign != "" {
				if h, err := horoscope.Daily(horoscope.Request{Sign: v.Astro.SunSign, Date: now.UTC(), Locale: LocaleFrom(ctx)}); err == nil {
					v.Horoscope = &ClientHoroscope{Sign: h.Sign, SignName: h.SignName, Date: h.Date, Text: h.Text, Focus: h.Focus, Mood: h.Mood, Affirmation: h.Affirmation}
				}
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

// clientAstro computes the client's sun, moon and rising signs with the same
// engine as their own chart (the sun sign comes from the Sun's actual
// longitude). Only what can be known is returned: the Moon's sign when it
// cannot have changed during the possible birth window, and the rising sign
// only with both a birth time and a birthplace. A birth time without a place
// is never interpreted as UTC.
func clientAstro(prof *domain.Profile) *ClientAstro {
	b := *prof.BirthDate
	date := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	out := &ClientAstro{LifePath: astro.LifePath(b.Year(), int(b.Month()), b.Day())}

	var zone *time.Location
	var where *astro.GeoLocation
	switch {
	case prof.BirthLatitude != nil && prof.BirthLongitude != nil && prof.BirthTimezone != "":
		if l, err := tz.Load(prof.BirthTimezone); err == nil {
			zone, where = l, &astro.GeoLocation{Latitude: *prof.BirthLatitude, Longitude: *prof.BirthLongitude}
		}
	case prof.BirthPlaceID != "":
		if pl, ok := astro.PlaceByID(prof.BirthPlaceID); ok {
			if l, err := tz.Load(pl.TimeZone); err == nil {
				loc := astro.LocationOf(pl)
				zone, where = l, &loc
			}
		}
	}
	req := astro.ChartRequest{Loc: where}
	if h, m, err := tz.ParseClock(prof.BirthTime); err == nil && zone != nil {
		req.Instant, _ = tz.Resolve(date, h, m, zone, where.Longitude)
		req.TimeKnown = true
	} else {
		req.Instant, req.WindowStart, req.WindowEnd = astro.BirthWindow(date, 0, 0, false, zone)
	}
	chart, err := astro.ComputeChart(req)
	if err != nil {
		out.SunSign = astro.SunSignAt(req.Instant)
		return out
	}
	out.SunSign, out.SunSignUncertain, out.TimeKnown = chart.SunSign, chart.SunSignUncertain, req.TimeKnown
	if !chart.MoonSignUncertain {
		out.MoonSign = chart.MoonSign
	}
	out.RisingSign = chart.RisingSign
	for _, pos := range chart.Positions {
		if pos.Body == string(astro.Moon) && chart.MoonSignUncertain {
			continue
		}
		pl := ClientPlacement{Body: pos.Body, Sign: pos.Sign, Retrograde: pos.Retrograde}
		if req.TimeKnown {
			d := pos.Degree
			pl.Degree, pl.House = &d, pos.House
		}
		out.Placements = append(out.Placements, pl)
	}
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
