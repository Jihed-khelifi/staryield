//go:build load

package load

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 4.0 (§23.2–23.4): the reply-target and likes lookups of a page of
// history, the advisor's inbox with the alias join and the alias lookup. They
// run on the data of `cmd/seed -profile=large -extend "message replies and
// likes,client aliases"` (random ids otherwise, so the check still runs).

type chatRevisions4Sample struct {
	pageIDs   []uuid.UUID // the newest messages of the busiest conversation
	replyIDs  []uuid.UUID // the messages they quote
	aliasUser uuid.UUID   // the advisor with the most aliases
	aliasOf   []uuid.UUID // up to 50 of that advisor's aliased clients
}

func pickChatRevisions4(db *gorm.DB, ids sampleIDs) chatRevisions4Sample {
	s := chatRevisions4Sample{aliasUser: uuid.New()}
	db.Raw(`SELECT id FROM chat_messages WHERE conversation_id = ? ORDER BY seq DESC LIMIT 200`, ids.busyConversation).Scan(&s.pageIDs)
	db.Raw(`SELECT DISTINCT reply_to_id FROM (SELECT reply_to_id FROM chat_messages WHERE conversation_id = ? AND reply_to_id IS NOT NULL
		ORDER BY seq DESC LIMIT 200) r`, ids.busyConversation).Scan(&s.replyIDs)
	if len(s.pageIDs) == 0 {
		s.pageIDs = []uuid.UUID{uuid.New()}
	}
	if len(s.replyIDs) == 0 {
		s.replyIDs = []uuid.UUID{uuid.New()}
	}
	var one struct{ A uuid.UUID }
	if db.Table("client_aliases").Select("psychic_id AS a").Group("psychic_id").Order("count(*) DESC").Limit(1).Scan(&one).Error == nil && one.A != uuid.Nil {
		s.aliasUser = one.A
		db.Raw(`SELECT client_id FROM client_aliases WHERE psychic_id = ? LIMIT 50`, one.A).Scan(&s.aliasOf)
	}
	if len(s.aliasOf) == 0 {
		s.aliasOf = []uuid.UUID{uuid.New()}
	}
	return s
}

func chatRevisions4Checks(db *gorm.DB, ids sampleIDs) []queryCheck {
	s := pickChatRevisions4(db, ids)
	page := func(n int, from []uuid.UUID) []uuid.UUID { return from[:min(n, len(from))] }
	return []queryCheck{
		{"chat: reply targets (page of 50)", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).ReplyTargets(ctx, page(50, s.replyIDs))
		}},
		{"chat: reply targets (page of 200)", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).ReplyTargets(ctx, page(200, s.replyIDs))
		}},
		{"chat: likes of a page (50)", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).LikesFor(ctx, page(50, s.pageIDs))
		}},
		{"chat: likes of a page (200)", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).LikesFor(ctx, page(200, s.pageIDs))
		}},
		{"chat: inbox (advisor with aliases)", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).ListInbox(ctx, repository.InboxFilter{UserID: s.aliasUser, AsPsychic: true, Limit: 51})
		}},
		{"aliases: lookup (50 clients)", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewAliasRepository(d).List(ctx, s.aliasUser, s.aliasOf)
		}},
	}
}
