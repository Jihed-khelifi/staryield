package service

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// RealtimeGateway executes realtime client frames (realtime.FrameHandler).
type RealtimeGateway struct {
	chat     *ChatService
	psychics *PsychicService
	presence realtime.Presence
}

// NewRealtimeGateway wires the gateway.
func NewRealtimeGateway(chat *ChatService, psychics *PsychicService, presence realtime.Presence) *RealtimeGateway {
	return &RealtimeGateway{chat: chat, psychics: psychics, presence: presence}
}

func principalOf(c *realtime.Client) reqctx.Principal {
	return reqctx.Principal{UserID: c.UserID, Role: domain.Role(c.Role)}
}

// OnConnect implements realtime.FrameHandler.
func (g *RealtimeGateway) OnConnect(ctx context.Context, c *realtime.Client) {
	switch domain.Role(c.Role) {
	case domain.RolePsychic:
		g.psychics.PublishStatus(reqctx.WithPrincipal(ctx, principalOf(c)), c.UserID)
	case domain.RoleClient:
		g.chat.PublishClientPresence(ctx, c.UserID)
	}
}

// OnDisconnect implements realtime.FrameHandler.
func (g *RealtimeGateway) OnDisconnect(ctx context.Context, c *realtime.Client, joined []uuid.UUID) {
	ctx = reqctx.WithPrincipal(ctx, principalOf(c))
	g.chat.ConnectionClosed(ctx, c.UserID, joined)
	switch domain.Role(c.Role) {
	case domain.RolePsychic:
		g.psychics.PublishStatus(ctx, c.UserID)
	case domain.RoleClient:
		g.chat.PublishClientPresence(ctx, c.UserID)
	}
}

// Frame payloads.
type (
	sessionFrame struct {
		SessionID uuid.UUID `json:"session_id"`
	}
	sendFrame struct {
		SessionID   uuid.UUID `json:"session_id"`
		Body        string    `json:"body"`
		ClientMsgID string    `json:"client_msg_id"`
	}
	typingFrame struct {
		SessionID uuid.UUID `json:"session_id"`
		Typing    bool      `json:"typing"`
	}
	readFrame struct {
		ConversationID uuid.UUID `json:"conversation_id"`
		MessageID      uuid.UUID `json:"message_id"`
	}
)

// decodeStrict rejects unknown fields and trailing data.
func decodeStrict(data json.RawMessage, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return apperror.BadRequest("Malformed frame payload.")
	}
	if dec.More() {
		return apperror.BadRequest("Malformed frame payload.")
	}
	return nil
}

func requireID(id uuid.UUID, field string) error {
	if id == uuid.Nil {
		return apperror.Validation(map[string]string{field: "This field is required."})
	}
	return nil
}

// HandleFrame implements realtime.FrameHandler.
func (g *RealtimeGateway) HandleFrame(ctx context.Context, c *realtime.Client, frameType string, data json.RawMessage) (any, error) {
	p := principalOf(c)
	ctx = reqctx.WithPrincipal(ctx, p)
	if len(data) == 0 {
		data = json.RawMessage("{}")
	}
	switch frameType {
	case realtime.FrameSessionJoin:
		var f sessionFrame
		if err := decodeStrict(data, &f); err != nil {
			return nil, err
		}
		if err := requireID(f.SessionID, "session_id"); err != nil {
			return nil, err
		}
		return g.chat.Join(ctx, p, c.ID, f.SessionID)
	case realtime.FrameSessionLeave:
		var f sessionFrame
		if err := decodeStrict(data, &f); err != nil {
			return nil, err
		}
		if err := requireID(f.SessionID, "session_id"); err != nil {
			return nil, err
		}
		return map[string]bool{"left": true}, g.chat.Leave(ctx, p, c.ID, f.SessionID)
	case realtime.FrameMessageSend:
		var f sendFrame
		if err := decodeStrict(data, &f); err != nil {
			return nil, err
		}
		if err := requireID(f.SessionID, "session_id"); err != nil {
			return nil, err
		}
		msg, err := g.chat.SendMessage(ctx, p, SendMessageInput{SessionID: f.SessionID, Body: f.Body, ClientMsgID: f.ClientMsgID})
		if err != nil {
			return nil, err
		}
		return map[string]any{"message": msg}, nil
	case realtime.FrameTyping:
		var f typingFrame
		if err := decodeStrict(data, &f); err != nil {
			return nil, err
		}
		return nil, g.chat.Typing(ctx, p, f.SessionID, f.Typing)
	case realtime.FrameMessageRead:
		var f readFrame
		if err := decodeStrict(data, &f); err != nil {
			return nil, err
		}
		if err := requireID(f.MessageID, "message_id"); err != nil {
			return nil, err
		}
		return map[string]bool{"read": true}, g.chat.MarkRead(ctx, p, f.ConversationID, f.MessageID)
	}
	logger.FromContext(ctx).Debug("realtime: unknown frame", zap.String("type", frameType))
	return nil, apperror.BadRequest("Unknown frame type.")
}

// JoinFromREST performs a session join for an SSE client, verifying that the
// connection belongs to the caller.
func (g *RealtimeGateway) JoinFromREST(ctx context.Context, p reqctx.Principal, connID string, sessionID uuid.UUID, join bool) (any, error) {
	owner, ok, err := g.presence.ConnectionOwner(ctx, connID)
	if err != nil {
		return nil, apperror.Unavailable(err)
	}
	if !ok || owner != p.UserID {
		return nil, apperror.Validation(map[string]string{"connection_id": "Unknown or expired connection."})
	}
	if join {
		return g.chat.Join(ctx, p, connID, sessionID)
	}
	return map[string]bool{"left": true}, g.chat.Leave(ctx, p, connID, sessionID)
}

var _ realtime.FrameHandler = (*RealtimeGateway)(nil)
var _ = errNotParticipant
