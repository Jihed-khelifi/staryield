package service

// revisions 6.0 (docs/platform-extensions.md §26.1): what a held reading's
// messages look like to each viewer, and the hold's own view.

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// HoldView is a hold as the contract's ChatHold type.
type HoldView struct {
	ID             uuid.UUID  `json:"id"`
	SessionID      uuid.UUID  `json:"session_id"`
	ConversationID uuid.UUID  `json:"conversation_id"`
	Reason         string     `json:"reason"`
	Status         string     `json:"status"`
	StartedAt      time.Time  `json:"started_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	RevealedAt     *time.Time `json:"revealed_at,omitempty"`
	// PriceCredits is what continuing or revealing costs: the reading's
	// per-minute rate. The advisor sees their list rate instead (a client's
	// member discount is the client's business).
	PriceCredits int64 `json:"price_credits"`
	HeldMessages int   `json:"held_messages"`
}

// holdView renders h for w.
func holdView(h *domain.ChatHold, w viewer) HoldView {
	v := HoldView{ID: h.ID, SessionID: h.SessionID, ConversationID: h.ConversationID, Reason: string(h.Reason), Status: string(h.Status),
		StartedAt: h.StartedAt, ExpiresAt: h.ExpiresAt, ResolvedAt: h.ResolvedAt, RevealedAt: h.RevealedAt,
		PriceCredits: h.PriceCredits, HeldMessages: h.HeldMessages}
	if !w.Admin && w.ID != h.ClientID {
		v.PriceCredits = h.ListRateCredits
	}
	return v
}

// holdOf returns the hold a session view carries: the open hold while the
// reading is held, else its latest expired or revealed hold. A converted hold
// leaves nothing to show. Sessions that were never held cost no query.
func (p *participants) holdOf(ctx context.Context, s *domain.Session) *domain.ChatHold {
	if s.LastHoldID == nil {
		return nil
	}
	if p.holds == nil {
		p.holds = map[uuid.UUID]*domain.ChatHold{}
	}
	h, ok := p.holds[*s.LastHoldID]
	if !ok {
		rows, err := p.svc.chat.HoldsByIDs(ctx, []uuid.UUID{*s.LastHoldID})
		warnErr(ctx, "chat: load hold", err)
		if len(rows) == 1 {
			h = &rows[0]
		}
		p.holds[*s.LastHoldID] = h // a failed lookup is remembered too: one try per call
	}
	if h == nil || h.Status == domain.HoldConverted {
		return nil
	}
	return h
}

// prefetchHolds loads the holds of a page of sessions in one query.
func (p *participants) prefetchHolds(ctx context.Context, sessions []domain.Session) {
	if p.holds == nil {
		p.holds = map[uuid.UUID]*domain.ChatHold{}
	}
	var ids []uuid.UUID
	for i := range sessions {
		if id := sessions[i].LastHoldID; id != nil {
			if _, done := p.holds[*id]; !done {
				ids = append(ids, *id)
				p.holds[*id] = nil
			}
		}
	}
	if len(ids) == 0 {
		return
	}
	rows, err := p.svc.chat.HoldsByIDs(ctx, ids)
	warnErr(ctx, "chat: load holds", err)
	for i := range rows {
		p.holds[rows[i].ID] = &rows[i]
	}
}

// ---------------------------------------------------------------------------
// The redaction. Every path that returns message content to a viewer ends
// here: timeline pages, realtime message.created, the inbox preview, reply
// quotes, the saved list, the answer to a send. (Attachments, likes, saves and
// push are guarded with lockedFrom / HeldMessagePush.)
// ---------------------------------------------------------------------------

// lockedFrom reports whether a hold hides message m from w: the message is
// held and w is neither its sender nor an administrator. It fails closed: a
// held message of unknown sender is hidden from everyone but administrators.
func lockedFrom(m *domain.Message, w viewer) bool {
	return hiddenFrom(m.Held, m.SenderID, w)
}

func hiddenFrom(held bool, sender *uuid.UUID, w viewer) bool {
	return held && !w.Admin && (sender == nil || *sender != w.ID)
}

// redactMessages applies the lock to a page of views for w (in place): a
// message hidden from w loses its body, attachments, quote and likes and
// carries body_length instead; a quote of such a message loses its excerpt.
// The sender and administrators keep the full view, flagged held.
func redactMessages(views []MessageView, w viewer) {
	for i := range views {
		v := &views[i]
		if rp := v.ReplyTo; rp != nil && hiddenFrom(rp.held, rp.SenderID, w) {
			cp := *rp
			cp.Excerpt, cp.Locked = "", true
			v.ReplyTo = &cp
		}
		if !hiddenFrom(v.Held, v.SenderID, w) {
			continue
		}
		v.Body, v.Attachments, v.ReplyTo, v.LikedBy, v.Saved = "", nil, nil, nil, false
		v.Held, v.Locked = false, true
		v.BodyLength = min(v.bodyLen, domain.LockedBodyLengthCap)
	}
}

// redactOne is redactMessages for a single view.
func redactOne(v *MessageView, w viewer) {
	vs := []MessageView{*v}
	redactMessages(vs, w)
	*v = vs[0]
}

// viewFor returns a copy of v as w sees it (the original is untouched).
func viewFor(v MessageView, w viewer) MessageView {
	redactOne(&v, w)
	return v
}

// redactLastMessages applies the lock to the inbox previews.
func redactLastMessages(items []ConversationView, w viewer) {
	for i := range items {
		if lm := items[i].LastMessage; lm != nil {
			redactOne(lm, w)
		}
	}
}

func derefInt(n *int) int {
	if n == nil {
		return 0
	}
	return *n
}
