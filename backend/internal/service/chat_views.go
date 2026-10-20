package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// ParticipantView identifies one side of a conversation.
type ParticipantView struct {
	ID        uuid.UUID `json:"id"`
	Role      string    `json:"role"`
	Name      string    `json:"name"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Slug      string    `json:"slug,omitempty"`
	Status    string    `json:"status,omitempty"`
}

// SessionView is a live chat session as seen by one viewer. Monetary fields
// are redacted per role: clients never see psychic earnings and psychics
// never see what the client was charged.
type SessionView struct {
	ID                uuid.UUID       `json:"id"`
	ConversationID    uuid.UUID       `json:"conversation_id"`
	Status            string          `json:"status"`
	EndReason         string          `json:"end_reason,omitempty"`
	Client            ParticipantView `json:"client"`
	Psychic           ParticipantView `json:"psychic"`
	RatePerMinute     int64           `json:"rate_per_minute"`
	RequestedAt       time.Time       `json:"requested_at"`
	ExpiresAt         *time.Time      `json:"expires_at,omitempty"`
	AcceptedAt        *time.Time      `json:"accepted_at,omitempty"`
	StartedAt         *time.Time      `json:"started_at,omitempty"`
	DisconnectedAt    *time.Time      `json:"disconnected_at,omitempty"`
	DisconnectedParty string          `json:"disconnected_party,omitempty"`
	GraceExpiresAt    *time.Time      `json:"grace_expires_at,omitempty"`
	EndedAt           *time.Time      `json:"ended_at,omitempty"`
	SettledAt         *time.Time      `json:"settled_at,omitempty"`
	NextBillingAt     *time.Time      `json:"next_billing_at,omitempty"`
	ActiveSeconds     int64           `json:"active_seconds"`
	BilledMinutes     int             `json:"billed_minutes"`
	FreeMinutesUsed   int             `json:"free_minutes_used"`
	CreditsCharged    *int64          `json:"credits_charged,omitempty"`
	PsychicEarnings   *int64          `json:"psychic_earnings,omitempty"`
	Reconnects        int             `json:"reconnects"`
	ServerTime        time.Time       `json:"server_time"`
}

// SessionSummary is the end-of-session recap (sequence step 18).
type SessionSummary struct {
	Session         SessionView `json:"session"`
	DurationSeconds int64       `json:"duration_seconds"`
	Messages        int64       `json:"messages"`
}

// MessageView is a chat message.
type MessageView struct {
	ID             uuid.UUID  `json:"id"`
	ConversationID uuid.UUID  `json:"conversation_id"`
	SessionID      uuid.UUID  `json:"session_id"`
	SenderID       *uuid.UUID `json:"sender_id,omitempty"`
	Kind           string     `json:"kind"`
	Body           string     `json:"body"`
	ClientMsgID    string     `json:"client_msg_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	Saved          bool       `json:"saved,omitempty"`
}

// ConversationView is an inbox row.
type ConversationView struct {
	ID            uuid.UUID       `json:"id"`
	Counterpart   ParticipantView `json:"counterpart"`
	LastMessage   *MessageView    `json:"last_message,omitempty"`
	LastMessageAt *time.Time      `json:"last_message_at,omitempty"`
	Unread        int64           `json:"unread"`
	OpenSession   *SessionView    `json:"open_session,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// viewer describes who a view is rendered for.
type viewer struct {
	ID    uuid.UUID
	Admin bool
}

func messageView(m *domain.Message) MessageView {
	v := MessageView{ID: m.ID, ConversationID: m.ConversationID, SessionID: m.SessionID, SenderID: m.SenderID,
		Kind: string(m.Kind), Body: m.Body, CreatedAt: m.CreatedAt}
	if m.ClientMsgID != nil {
		v.ClientMsgID = *m.ClientMsgID
	}
	return v
}

// participants resolves display identities for users, caching per call.
type participants struct {
	svc   *ChatService
	cache map[uuid.UUID]ParticipantView
}

func (c *ChatService) newParticipants() *participants {
	return &participants{svc: c, cache: map[uuid.UUID]ParticipantView{}}
}

func (p *participants) client(ctx context.Context, id uuid.UUID) ParticipantView {
	if v, ok := p.cache[id]; ok {
		return v
	}
	v := ParticipantView{ID: id, Role: string(domain.RoleClient), Name: "Client"}
	if u, err := p.svc.users.GetByID(ctx, id); err == nil {
		v.Name = u.DisplayName
	}
	if prof, err := p.svc.users.GetProfile(ctx, id); err == nil {
		if prof.Nickname != "" {
			v.Name = prof.Nickname
		}
		v.AvatarURL = prof.AvatarURL
	}
	p.cache[id] = v
	return v
}

func (p *participants) psychic(ctx context.Context, id uuid.UUID) ParticipantView {
	if v, ok := p.cache[id]; ok {
		return v
	}
	v := ParticipantView{ID: id, Role: string(domain.RolePsychic), Name: "Advisor"}
	if ps, err := p.svc.psychicsRepo.GetByUserID(ctx, id); err == nil {
		v.Name, v.AvatarURL, v.Slug = ps.Name, ps.ImageURL, ps.Slug
	}
	p.cache[id] = v
	return v
}

// sessionView renders s for w at now.
func (c *ChatService) sessionView(ctx context.Context, ps *participants, s *domain.Session, w viewer) SessionView {
	now := c.clock.Now()
	v := SessionView{
		ID: s.ID, ConversationID: s.ConversationID, Status: string(s.Status), EndReason: string(s.EndReason),
		Client: ps.client(ctx, s.ClientID), Psychic: ps.psychic(ctx, s.PsychicID),
		RatePerMinute: s.RatePerMinute, RequestedAt: s.RequestedAt, AcceptedAt: s.AcceptedAt, StartedAt: s.StartedAt,
		DisconnectedAt: s.DisconnectedAt, DisconnectedParty: s.DisconnectedParty, GraceExpiresAt: s.GraceExpiresAt,
		EndedAt: s.EndedAt, SettledAt: s.SettledAt, NextBillingAt: s.NextBillingAt,
		ActiveSeconds: int64(s.ElapsedActive(now) / time.Second), BilledMinutes: s.BilledMinutes,
		FreeMinutesUsed: s.FreeMinutesUsed, Reconnects: s.Reconnects, ServerTime: now,
	}
	if s.Status == domain.SessionRequested || s.Status == domain.SessionAccepted {
		v.ExpiresAt = ptr(s.ExpiresAt)
	}
	if w.Admin || w.ID == s.ClientID {
		v.CreditsCharged = ptr(s.CreditsCharged)
	}
	if w.Admin || w.ID == s.PsychicID {
		v.PsychicEarnings = ptr(s.PsychicEarnings)
	}
	return v
}

// publishSession sends eventType with a per-viewer session view to both
// participants, merging extra fields into the payload.
func (c *ChatService) publishSession(ctx context.Context, eventType string, s *domain.Session, extra map[string]any) {
	ps := c.newParticipants()
	for _, uid := range []uuid.UUID{s.ClientID, s.PsychicID} {
		payload := map[string]any{"session": c.sessionView(ctx, ps, s, viewer{ID: uid})}
		for k, v := range extra {
			payload[k] = v
		}
		warnErr(ctx, "chat: publish "+eventType, c.events.Publish(ctx, eventType, payload, uid))
	}
}

// summary builds the end-of-session recap for w.
func (c *ChatService) summary(ctx context.Context, s *domain.Session, w viewer) SessionSummary {
	n, err := c.chat.CountSessionMessages(ctx, s.ID)
	warnErr(ctx, "chat: count messages", err)
	return SessionSummary{Session: c.sessionView(ctx, c.newParticipants(), s, w), DurationSeconds: s.ActiveSeconds, Messages: n}
}

// loadParticipantSession returns the session if userID may see it.
func (c *ChatService) loadParticipantSession(ctx context.Context, userID uuid.UUID, admin bool, id uuid.UUID) (*domain.Session, error) {
	s, err := c.chat.GetSession(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Session")
	}
	if !admin && !s.IsParticipant(userID) {
		// Do not reveal the existence of other users' sessions.
		return nil, mapRepoErr(repository.ErrNotFound, "Session")
	}
	return s, nil
}

// isNotFound reports a repository not-found error.
func isNotFound(err error) bool { return errors.Is(err, repository.ErrNotFound) }
