package e2e

// revisions 4.0: the "message replies and likes" and "client aliases" large
// seed generators, at small scale (the pattern of TestLargeSeedSmallScale).

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/seed"
)

func TestLargeSeedRepliesLikesAndAliases(t *testing.T) {
	if !available {
		t.Skip("infrastructure unavailable")
	}
	ctx := context.Background()
	hasher := security.NewPasswordHasher(1024, 1, 1)
	sealer, err := security.NewSealer(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	opts := seed.LargeOptions{Users: 300, Psychics: 12, Conversations: 400, MessagesPerConversation: 25, Reviews: 100, LedgerRows: 400,
		Seed: 11, BatchSize: 100, Sealer: sealer}
	db := isolatedSchema(t)
	// The generators run as part of a full population build.
	if _, err := seed.Large(ctx, db, hasher, opts, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	scalar := func(q string, args ...any) int64 {
		t.Helper()
		var n int64
		if err := db.Raw(q, args...).Scan(&n).Error; err != nil {
			t.Fatal(err)
		}
		return n
	}
	texts := scalar(`SELECT count(*) FROM chat_messages WHERE kind = 'text' AND seq > 1`)
	replies := scalar(`SELECT count(*) FROM chat_messages WHERE reply_to_id IS NOT NULL`)
	if texts < 500 || replies < texts*8/100 || replies > texts*25/100 {
		t.Fatalf("%d replies among %d text messages (want about 15%%)", replies, texts)
	}
	// A reply quotes an earlier text or image message of its own conversation.
	if bad := scalar(`SELECT count(*) FROM chat_messages m JOIN chat_messages t ON t.id = m.reply_to_id
		WHERE m.conversation_id <> t.conversation_id OR t.seq >= m.seq OR t.kind NOT IN ('text', 'image') OR m.kind <> 'text'`); bad != 0 {
		t.Fatalf("%d replies with an invalid target", bad)
	}
	likeable := scalar(`SELECT count(*) FROM chat_messages WHERE kind IN ('text', 'image')`)
	likes := scalar(`SELECT count(*) FROM message_likes`)
	liked := scalar(`SELECT count(DISTINCT message_id) FROM message_likes`)
	both := scalar(`SELECT count(*) FROM (SELECT message_id FROM message_likes GROUP BY message_id HAVING count(*) = 2) x`)
	if liked < likeable*12/100 || liked > likeable*30/100 || both == 0 || both >= liked || likes != liked+both {
		t.Fatalf("%d likes on %d of %d messages (%d by both)", likes, liked, likeable, both)
	}
	// Only the two participants like, and never a system notice.
	if bad := scalar(`SELECT count(*) FROM message_likes l JOIN chat_messages m ON m.id = l.message_id JOIN conversations c ON c.id = m.conversation_id
		WHERE l.user_id NOT IN (c.client_id, c.psychic_id) OR m.kind NOT IN ('text', 'image')`); bad != 0 {
		t.Fatalf("%d invalid likes", bad)
	}
	// Aliases: about 30% of the conversations, sealed so the API can open them.
	convs := scalar(`SELECT count(*) FROM conversations`)
	aliases := scalar(`SELECT count(*) FROM client_aliases`)
	if aliases < convs*20/100 || aliases > convs*40/100 {
		t.Fatalf("%d aliases for %d conversations (want about 30%%)", aliases, convs)
	}
	var rows []struct {
		PsychicID, ClientID, AliasEnc string
	}
	if err := db.Raw(`SELECT psychic_id::text, client_id::text, alias_enc FROM client_aliases LIMIT 50`).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if plain, err := sealer.Open(r.AliasEnc, "alias:"+r.PsychicID+":"+r.ClientID); err != nil || plain == "" {
			t.Fatalf("alias does not open: %q %v", plain, err)
		}
	}

	// Running them again changes nothing.
	rep, err := seed.Extend(ctx, db, []string{"message replies and likes", "client aliases"}, opts, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Counts["message_likes"] != 0 || rep.Counts["client_aliases"] != 0 ||
		scalar(`SELECT count(*) FROM chat_messages WHERE reply_to_id IS NOT NULL`) != replies ||
		scalar(`SELECT count(*) FROM message_likes`) != likes || scalar(`SELECT count(*) FROM client_aliases`) != aliases {
		t.Fatalf("re-run changed the data: %+v", rep.Counts)
	}
}
