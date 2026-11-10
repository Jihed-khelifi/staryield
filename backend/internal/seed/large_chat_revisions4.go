package seed

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm/clause"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 4.0: replies, likes and private client aliases at size, so the
// query plans of the reply-target and likes lookups and of the advisor's
// inbox join are checked on a large dataset (tests/load/explain_test.go).
//
//	go run ./cmd/seed -profile=large -extend "message replies and likes,client aliases"
//
// Both generators work on the conversations of the synthetic clients already
// in the database (an extension run has no in-memory population), choose rows
// from a hash of the row id and the dataset seed (so the same seed always
// picks the same rows) and are safe to run again.
func init() {
	RegisterLarge(Generator{Name: "message replies and likes", Run: func(_ context.Context, d *Dataset) error { return d.repliesAndLikes() }})
	RegisterLarge(Generator{Name: "client aliases", Run: func(_ context.Context, d *Dataset) error { return d.clientAliases() }})
}

const (
	// replyShare and likeShare are percentages of the text (and image)
	// messages; bothLikeShare of the messages are liked by both participants.
	replyShare    = 15
	likeShare     = 20
	bothLikeShare = 6
	aliasShare    = 30
	// convChunk bounds the conversations handled by one batched statement.
	convChunk = 500
)

// syntheticConversations returns the ids of the conversations of synthetic
// clients, in a stable order.
func (d *Dataset) syntheticConversations() ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := d.DB.Raw(`SELECT c.id FROM conversations c JOIN users u ON u.id = c.client_id
		WHERE u.email LIKE ? AND u.role = 'client' ORDER BY c.id`, "%@"+LoadEmailDomain).Scan(&ids).Error
	return ids, err
}

// repliesAndLikes sets reply_to_id on about 15% of the text messages (each
// quotes an earlier text or image message of the same conversation, one to
// five messages back) and likes about 20% of the text and image messages by
// one participant, a third of those by both. Statements are batched by
// chunks of conversations; a run that finds replies already generated for
// the synthetic population does nothing (likes use ON CONFLICT DO NOTHING).
func (d *Dataset) repliesAndLikes() error {
	var done bool
	if err := d.DB.Raw(`SELECT EXISTS (SELECT 1 FROM chat_messages m JOIN conversations c ON c.id = m.conversation_id
		JOIN users u ON u.id = c.client_id WHERE m.reply_to_id IS NOT NULL AND u.email LIKE ?)`, "%@"+LoadEmailDomain).Scan(&done).Error; err != nil {
		return err
	}
	if done {
		d.Log.Info("message replies and likes skipped: already generated")
		return nil
	}
	convs, err := d.syntheticConversations()
	if err != nil {
		return err
	}
	if len(convs) == 0 {
		d.Log.Warn("message replies and likes skipped: no conversations of synthetic clients")
		return nil
	}
	// A stable non-negative hash of the message id and the dataset seed; the
	// salts are numeric literals built here, never input.
	hash := func(salt int64) string {
		return fmt.Sprintf("(hashtextextended(m.id::text, %d) & 2147483647)", int64(d.Opts.Seed)+salt)
	}
	// 15% of the text messages quote one of the 1-5 previous text or image
	// messages of their conversation (never the first message).
	replySQL := fmt.Sprintf(`UPDATE chat_messages m SET reply_to_id = (
			SELECT t.id FROM chat_messages t WHERE t.conversation_id = m.conversation_id AND t.seq < m.seq AND t.kind IN ('text', 'image')
			ORDER BY t.seq DESC OFFSET (%s %% 5) LIMIT 1)
		WHERE m.conversation_id IN ? AND m.kind = 'text' AND m.seq > 1 AND m.reply_to_id IS NULL AND %s %% 100 < %d`,
		hash(11), hash(12), replyShare)
	// 20% of the text and image messages are liked by one participant (chosen
	// by the parity of another hash); 6% of the messages by the other one too.
	likeSQL := func(participant string, share int) string {
		return fmt.Sprintf(`INSERT INTO message_likes (message_id, user_id, created_at)
			SELECT m.id, %s, m.created_at + (1 + %s %% 600) * interval '1 minute'
			FROM chat_messages m JOIN conversations c ON c.id = m.conversation_id
			WHERE m.conversation_id IN ? AND m.kind IN ('text', 'image') AND %s %% 100 < %d
			ON CONFLICT DO NOTHING`, participant, hash(14), hash(15), share)
	}
	first := fmt.Sprintf("CASE WHEN %s %% 2 = 0 THEN c.client_id ELSE c.psychic_id END", hash(13))
	second := fmt.Sprintf("CASE WHEN %s %% 2 = 0 THEN c.psychic_id ELSE c.client_id END", hash(13))
	var replies, likes int64
	for start := 0; start < len(convs); start += convChunk {
		chunk := convs[start:min(start+convChunk, len(convs))]
		res := d.DB.Exec(replySQL, chunk)
		if res.Error != nil {
			return fmt.Errorf("replies: %w", res.Error)
		}
		replies += res.RowsAffected
		for _, l := range []struct {
			participant string
			share       int
		}{{first, likeShare}, {second, bothLikeShare}} {
			res := d.DB.Exec(likeSQL(l.participant, l.share), chunk)
			if res.Error != nil {
				return fmt.Errorf("likes: %w", res.Error)
			}
			likes += res.RowsAffected
		}
	}
	d.Counts["chat_messages.reply_to_id"] += int(replies)
	d.Counts["message_likes"] += int(likes)
	d.Log.Info("message replies and likes generated", zap.Int64("replies", replies), zap.Int64("likes", likes))
	return nil
}

var aliasNames = []string{"Moon child", "Sunshine", "Tarot fan", "Night owl", "Scorpio friend", "Star gazer", "Dream walker", "Lucky charm",
	"Skeptic", "Regular", "Aries lady", "Mystery caller", "Old friend", "New moon", "Cosmic kid", "Vee"}

// pickBucket returns a stable 0..99 value for an id, the dataset seed and a
// salt (FNV-1a; independent of the random stream, so a re-run picks the same
// rows).
func (d *Dataset) pickBucket(id uuid.UUID, salt string) (bucket int, h uint64) {
	f := fnv.New64a()
	_, _ = fmt.Fprintf(f, "%d:%s:%s", d.Opts.Seed, salt, id)
	h = f.Sum64()
	return int(h % 100), h
}

// clientAliases gives about 30% of the synthetic conversations an advisor
// nickname for the client, sealed exactly as the API seals it (associated
// data "alias:<psychic_id>:<client_id>"), so the API can open it. Without a
// sealer the step is skipped. Re-runs add nothing new (the pair is the key).
func (d *Dataset) clientAliases() error {
	if d.Sealer == nil {
		d.Log.Warn("client aliases skipped: no sealer configured")
		return nil
	}
	var pairs []struct{ ID, PsychicID, ClientID uuid.UUID }
	if err := d.DB.Raw(`SELECT c.id, c.psychic_id, c.client_id FROM conversations c JOIN users u ON u.id = c.client_id
		WHERE u.email LIKE ? AND u.role = 'client' ORDER BY c.id`, "%@"+LoadEmailDomain).Scan(&pairs).Error; err != nil {
		return err
	}
	if len(pairs) == 0 {
		d.Log.Warn("client aliases skipped: no conversations of synthetic clients")
		return nil
	}
	batch := make([]domain.ClientAlias, 0, d.Opts.BatchSize)
	inserted := 0
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		res := d.DB.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(&batch, d.Opts.BatchSize)
		if res.Error != nil {
			return fmt.Errorf("insert client_aliases: %w", res.Error)
		}
		inserted += int(res.RowsAffected)
		batch = batch[:0]
		return nil
	}
	for _, p := range pairs {
		b, h := d.pickBucket(p.ID, "alias")
		if b >= aliasShare {
			continue
		}
		alias := fmt.Sprintf("%s %d", aliasNames[h/100%uint64(len(aliasNames))], h/1000%1000)
		enc, err := d.Sealer.Seal(alias, "alias:"+p.PsychicID.String()+":"+p.ClientID.String())
		if err != nil {
			return err
		}
		at := d.Now.Add(-time.Duration(h/7%(90*24*60)) * time.Minute)
		batch = append(batch, domain.ClientAlias{PsychicID: p.PsychicID, ClientID: p.ClientID, AliasEnc: enc, CreatedAt: at, UpdatedAt: at})
		if len(batch) == cap(batch) {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := flush(); err != nil {
		return err
	}
	d.Counts["client_aliases"] += inserted
	d.Log.Info("client aliases generated", zap.Int("rows", inserted))
	return nil
}
