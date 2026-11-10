package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// MessageLikes is the authoritative like state of one message (revisions
// 4.0, §23.3). LikedBy is always present (possibly empty).
type MessageLikes struct {
	MessageID      uuid.UUID   `json:"message_id"`
	ConversationID uuid.UUID   `json:"conversation_id"`
	Seq            int64       `json:"seq"`
	LikedBy        []uuid.UUID `json:"liked_by"`
}

// SetLike likes (like=true) or unlikes the message for the caller. Both calls
// are idempotent. The caller must take part in the message's conversation
// (anyone else, and unknown ids, get 404); system notices cannot be liked
// (422). A like is free at any time: it never touches unread counters,
// last_seq, notifications or push.
func (c *ChatService) SetLike(ctx context.Context, p reqctx.Principal, messageID uuid.UUID, like bool) (*MessageLikes, error) {
	m, err := c.chat.GetMessage(ctx, messageID)
	if err != nil {
		return nil, mapRepoErr(err, "Message")
	}
	conv, err := c.chat.GetConversation(ctx, m.ConversationID)
	if err != nil {
		return nil, mapRepoErr(err, "Message")
	}
	if !conv.IsParticipant(p.UserID) {
		return nil, apperror.NotFound("Message")
	}
	if !canQuote(m.Kind) {
		return nil, apperror.MessageNotLikeable()
	}
	var out MessageLikes
	var changed bool
	err = c.tx.WithinTx(ctx, func(ctx context.Context) error {
		changed = false
		// The row lock orders concurrent likes of one message: each event
		// carries the state after the previous like, so clients that apply
		// liked_by as authoritative never end on a stale list.
		if _, err := c.chat.LockMessage(ctx, messageID); err != nil {
			return err
		}
		if like {
			changed, err = c.chat.LikeMessage(ctx, messageID, p.UserID, c.clock.Now())
		} else {
			changed, err = c.chat.UnlikeMessage(ctx, messageID, p.UserID)
		}
		if err != nil {
			return err
		}
		rows, err := c.chat.LikesFor(ctx, []uuid.UUID{messageID})
		if err != nil {
			return err
		}
		out = MessageLikes{MessageID: m.ID, ConversationID: m.ConversationID, Seq: m.Seq, LikedBy: make([]uuid.UUID, 0, len(rows))}
		for _, r := range rows {
			out.LikedBy = append(out.LikedBy, r.UserID)
		}
		if changed {
			// Durable, to both participants, every device. Published under the
			// message lock (see above).
			payload := map[string]any{"message_id": out.MessageID, "conversation_id": out.ConversationID, "seq": out.Seq,
				"liked_by": out.LikedBy, "user_id": p.UserID, "liked": like}
			warnErr(ctx, "chat: publish like", c.events.Publish(ctx, realtime.EventMessageLiked, payload, conv.ClientID, conv.PsychicID))
		}
		return nil
	})
	if err != nil {
		return nil, mapRepoErr(err, "Message")
	}
	if changed {
		name := "chat.message_unliked"
		if like {
			name = "chat.message_liked"
		}
		c.tracker.Track(ctx, name, map[string]any{"conversation_id": conv.ID, "role": string(p.Role)})
	}
	return &out, nil
}
