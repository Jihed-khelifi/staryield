package service

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// ReplyView is the quoted message of a reply (revisions 4.0, §23.2). The
// text is derived at read time (messages are immutable), so nothing is
// stored twice.
type ReplyView struct {
	ID       uuid.UUID  `json:"id"`
	Seq      int64      `json:"seq"`
	SenderID *uuid.UUID `json:"sender_id,omitempty"`
	Kind     string     `json:"kind"`
	// Excerpt is the first 140 characters of the body (or image caption) with
	// runs of whitespace collapsed to one space; "" for a bare image.
	Excerpt string `json:"excerpt"`
}

// ReplyExcerpt builds the quoted text: whitespace (including line breaks)
// collapsed to single spaces, then at most the first 140 characters. A
// character is a Unicode code point, so multi-byte text is never cut in the
// middle of a code point.
func ReplyExcerpt(body string) string {
	collapsed := strings.Join(strings.Fields(body), " ")
	if utf8.RuneCountInString(collapsed) <= domain.ReplyExcerptChars {
		return collapsed
	}
	return string([]rune(collapsed)[:domain.ReplyExcerptChars])
}

// canQuote reports whether a message of this kind can be replied to or liked.
func canQuote(kind domain.MessageKind) bool {
	return kind == domain.MessageText || kind == domain.MessageImage
}

// checkReplyTarget validates reply_to_id: it must name a text or image
// message of the same conversation. Anything else (an unknown id, another
// conversation's message, a system notice) is the same 422, so ids cannot be
// probed across conversations.
func (c *ChatService) checkReplyTarget(ctx context.Context, conversationID uuid.UUID, id *uuid.UUID) error {
	if id == nil {
		return nil
	}
	m, err := c.chat.GetMessage(ctx, *id)
	if err != nil {
		if isNotFound(err) {
			return apperror.ReplyTargetInvalid()
		}
		return err
	}
	if m.ConversationID != conversationID || !canQuote(m.Kind) {
		return apperror.ReplyTargetInvalid()
	}
	return nil
}

// decorate fills reply_to and liked_by on a page of message views with at
// most two extra queries (one for the quoted messages, one for the likes),
// whatever the page size. likes=false skips the likes lookup (a message that
// was just created has none).
func (c *ChatService) decorate(ctx context.Context, views []MessageView, likes bool) error {
	var replyIDs, likeIDs []uuid.UUID
	seen := map[uuid.UUID]bool{}
	for i := range views {
		if id := views[i].replyToID; id != nil && !seen[*id] {
			seen[*id] = true
			replyIDs = append(replyIDs, *id)
		}
		if likes && canQuote(domain.MessageKind(views[i].Kind)) {
			likeIDs = append(likeIDs, views[i].ID)
		}
	}
	if len(replyIDs) > 0 {
		rows, err := c.chat.ReplyTargets(ctx, replyIDs)
		if err != nil {
			return err
		}
		byID := make(map[uuid.UUID]*ReplyView, len(rows))
		for _, r := range rows {
			byID[r.ID] = &ReplyView{ID: r.ID, Seq: r.Seq, SenderID: r.SenderID, Kind: r.Kind, Excerpt: ReplyExcerpt(r.Body)}
		}
		for i := range views {
			if id := views[i].replyToID; id != nil {
				// A copy per view: views are mutated independently.
				if rv, ok := byID[*id]; ok {
					cp := *rv
					views[i].ReplyTo = &cp
				}
			}
		}
	}
	if len(likeIDs) > 0 {
		rows, err := c.chat.LikesFor(ctx, likeIDs)
		if err != nil {
			return err
		}
		byMsg := make(map[uuid.UUID][]uuid.UUID, len(rows))
		for _, r := range rows {
			byMsg[r.MessageID] = append(byMsg[r.MessageID], r.UserID)
		}
		for i := range views {
			if who := byMsg[views[i].ID]; len(who) > 0 {
				views[i].LikedBy = who
			}
		}
	}
	return nil
}

// decorateBestEffort is decorate for the write paths (acks, replays,
// realtime events): the message is already stored, so a failed lookup is
// logged and the view is sent without the extras instead of failing the send.
func (c *ChatService) decorateBestEffort(ctx context.Context, views []MessageView, likes bool) {
	warnErr(ctx, "chat: decorate messages", c.decorate(ctx, views, likes))
}

// decorateLastMessages fills reply_to and liked_by on the inbox previews with
// the same two batched queries as a history page (a preview that is not a
// reply and was never liked costs no extra work beyond the likes lookup).
func (c *ChatService) decorateLastMessages(ctx context.Context, items []ConversationView) error {
	var views []MessageView
	var at []int
	for i := range items {
		if lm := items[i].LastMessage; lm != nil {
			views, at = append(views, *lm), append(at, i)
		}
	}
	if len(views) == 0 {
		return nil
	}
	if err := c.decorate(ctx, views, true); err != nil {
		return err
	}
	for k, i := range at {
		items[i].LastMessage.ReplyTo, items[i].LastMessage.LikedBy = views[k].ReplyTo, views[k].LikedBy
	}
	return nil
}
