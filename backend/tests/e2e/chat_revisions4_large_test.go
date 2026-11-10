package e2e

// revisions 4.0 at size. A conversation with several thousand messages, most
// of them replies and many liked, is read page by page: the number of
// queries per page must not depend on the page size or on how many
// messages are replies or liked, every quote and like must match the
// database, and a page must stay fast. A second test gives one advisor a few
// thousand conversations with private aliases and walks their inbox: the
// aliases ride along in the inbox's single query.

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

const (
	largeThreadMessages = 6000
	largeAdvisorClients = 3000
	// A page read must stay well under this even on a slow machine.
	largePageBudget = time.Second
)

func p95(d []time.Duration) time.Duration {
	s := append([]time.Duration(nil), d...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[(len(s)*95+99)/100-1]
}

func TestRepliesAndLikesAtSize(t *testing.T) {
	if testing.Short() {
		t.Skip("large dataset")
	}
	e := newEnv(t, roomy)
	cl, ps, _, _, s := startedSession(t, e, 10, 250)
	conv := s.ConversationID
	first := int64(2)
	insertMessages(t, e, s, cl.user.ID, ps.user.ID, first, first+largeThreadMessages-1)
	last := first + largeThreadMessages - 1
	db := e.app.DB

	// About 60% of the messages after the 10th quote one of the previous nine;
	// a third are liked by the client, a quarter by the advisor (both, now and
	// then): roughly half of the thread carries a like.
	if err := db.Exec(`UPDATE chat_messages m SET reply_to_id = t.id FROM chat_messages t
		WHERE m.conversation_id = ? AND t.conversation_id = ? AND m.seq > 10 AND m.seq % 5 < 3
		AND t.seq = m.seq - 1 - (m.seq % 9) AND t.kind = 'text'`, conv, conv).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO message_likes (message_id, user_id, created_at)
		SELECT id, ?::uuid, created_at FROM chat_messages WHERE conversation_id = ? AND kind = 'text' AND seq % 3 = 0
		ON CONFLICT DO NOTHING`, cl.user.ID, conv).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO message_likes (message_id, user_id, created_at)
		SELECT id, ?::uuid, created_at FROM chat_messages WHERE conversation_id = ? AND kind = 'text' AND seq % 4 = 0
		ON CONFLICT DO NOTHING`, ps.user.ID, conv).Error; err != nil {
		t.Fatal(err)
	}
	db.Exec("ANALYZE chat_messages")
	db.Exec("ANALYZE message_likes")

	// The truth, from the database.
	type truthRow struct {
		ID, ReplyToID *uuid.UUID
		Seq           int64
	}
	var rows []truthRow
	if err := db.Table("chat_messages").Select("id, reply_to_id, seq").Where("conversation_id = ?", conv).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	replyOf, seqOf := map[uuid.UUID]uuid.UUID{}, map[uuid.UUID]int64{}
	for _, r := range rows {
		seqOf[*r.ID] = r.Seq
		if r.ReplyToID != nil {
			replyOf[*r.ID] = *r.ReplyToID
		}
	}
	var likeRows []struct{ MessageID, UserID uuid.UUID }
	if err := db.Table("message_likes").Select("message_id, user_id").Where("message_id IN (SELECT id FROM chat_messages WHERE conversation_id = ?)", conv).Scan(&likeRows).Error; err != nil {
		t.Fatal(err)
	}
	likesOf := map[uuid.UUID]map[uuid.UUID]bool{}
	for _, l := range likeRows {
		if likesOf[l.MessageID] == nil {
			likesOf[l.MessageID] = map[uuid.UUID]bool{}
		}
		likesOf[l.MessageID][l.UserID] = true
	}
	t.Logf("%d messages, %d replies, %d liked", len(rows), len(replyOf), len(likesOf))
	if len(replyOf) < largeThreadMessages/3 || len(likesOf) < largeThreadMessages/3 {
		t.Fatalf("test data too thin: %d replies, %d liked", len(replyOf), len(likesOf))
	}

	verify := func(what string, items []service.MessageView) {
		t.Helper()
		for _, m := range items {
			if m.Kind != "text" {
				continue
			}
			want, isReply := replyOf[m.ID]
			switch {
			case isReply && (m.ReplyTo == nil || m.ReplyTo.ID != want || m.ReplyTo.Seq != seqOf[want] || m.ReplyTo.Excerpt != fmt.Sprintf("bulk %d", seqOf[want]) || m.ReplyTo.Kind != "text"):
				t.Fatalf("%s: seq %d quotes %+v, want %s (seq %d)", what, m.Seq, m.ReplyTo, want, seqOf[want])
			case !isReply && m.ReplyTo != nil:
				t.Fatalf("%s: seq %d has a quote it should not: %+v", what, m.Seq, m.ReplyTo)
			}
			wantLikes := likesOf[m.ID]
			if len(m.LikedBy) != len(wantLikes) {
				t.Fatalf("%s: seq %d liked_by %v, want %d likers", what, m.Seq, m.LikedBy, len(wantLikes))
			}
			for _, u := range m.LikedBy {
				if !wantLikes[u] {
					t.Fatalf("%s: seq %d liked_by has %s", what, m.Seq, u)
				}
			}
		}
	}

	// The query count of a page does not depend on its size.
	relevant := func(l *sqlLog) int { return l.count(`"chat_messages"`) + l.count(`"message_likes"`) }
	pageLog := func(limit int) *sqlLog {
		time.Sleep(300 * time.Millisecond)
		return recordSQL(e, func() {
			cl.must(http.StatusOK, http.MethodGet, convMessages(conv)+fmt.Sprintf("?limit=%d", limit), nil, nil)
		})
	}
	small, large := pageLog(10), pageLog(200)
	if relevant(small) != relevant(large) || large.count(`FROM "chat_messages" WHERE id IN`) != 1 || large.count(`FROM "message_likes"`) != 1 {
		t.Fatalf("page queries: 10 -> %d, 200 -> %d\n%v", relevant(small), relevant(large), large.stmt)
	}

	// Walk the whole thread backwards, then forwards: every page is correct,
	// each costs the same, and each is fast.
	var times []time.Duration
	before, seen := int64(0), 0
	for {
		path := convMessages(conv) + "?limit=200"
		if before > 0 {
			path += fmt.Sprintf("&before_seq=%d", before)
		}
		var page service.MessagePage
		started := time.Now()
		cl.must(http.StatusOK, http.MethodGet, path, nil, &page)
		times = append(times, time.Since(started))
		verify("history", page.Items)
		seen += len(page.Items)
		if !page.HasMore {
			break
		}
		before = page.NextBeforeSeq
	}
	if seen != int(last) {
		t.Fatalf("history walked %d messages, want %d", seen, last)
	}
	after, seenSince := int64(0), 0
	for {
		var page service.SincePage
		started := time.Now()
		ps.must(http.StatusOK, http.MethodGet, convMessages(conv)+fmt.Sprintf("/since?after_seq=%d&limit=500", after), nil, &page)
		times = append(times, time.Since(started))
		verify("since", page.Items)
		seenSince += len(page.Items)
		if !page.HasMore {
			break
		}
		after = page.Items[len(page.Items)-1].Seq
	}
	if seenSince != int(last) {
		t.Fatalf("since walked %d messages, want %d", seenSince, last)
	}
	if got := p95(times); got > largePageBudget {
		t.Fatalf("p95 page read %s exceeds %s", got, largePageBudget)
	}
	t.Logf("%d page reads, p95 %s", len(times), p95(times))

	// Saved messages: 150 of the most reply-heavy ones in one list.
	saved := make([]domain.SavedMessage, 0, 150)
	for id := range replyOf {
		saved = append(saved, domain.SavedMessage{UserID: cl.user.ID, MessageID: id, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()})
		if len(saved) == 150 {
			break
		}
	}
	if err := db.CreateInBatches(saved, 100).Error; err != nil {
		t.Fatal(err)
	}
	var list service.Page[service.MessageView]
	savedLog := recordSQL(e, func() { cl.must(http.StatusOK, http.MethodGet, "/api/v1/chat/saved", nil, &list) })
	if len(list.Items) != 150 || savedLog.count(`FROM "chat_messages" WHERE id IN`) != 1 || savedLog.count(`FROM "message_likes"`) != 1 {
		t.Fatalf("saved list: %d items\n%v", len(list.Items), savedLog.stmt)
	}
	verify("saved", list.Items)
}

func TestAdvisorInboxWithThousandsOfAliasesIsOneQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("large dataset")
	}
	e := newEnv(t, roomy)
	db := e.app.DB
	ps := e.createPsychic(10)
	marker := strings.ReplaceAll(uuid.NewString()[:8], "-", "")

	// A few thousand synthetic clients, one conversation each, distinct
	// activity times so the keyset order is total.
	if err := db.Exec(`WITH u AS (
			INSERT INTO users (id, email, password_hash, role, status, display_name, terms_accepted_at, created_at, updated_at)
			SELECT gen_random_uuid(), 'alias' || i || '-' || ? || '@load.example.com', 'x', 'client', 'active', 'Client ' || i, now(), now(), now()
			FROM generate_series(1, ?) AS i RETURNING id, display_name)
		INSERT INTO conversations (id, client_id, psychic_id, last_activity_at, created_at, updated_at)
		SELECT gen_random_uuid(), u.id, ?::uuid, now() - (row_number() OVER ()) * interval '1 minute', now(), now() FROM u`,
		marker, largeAdvisorClients, ps.user.ID).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM client_aliases WHERE psychic_id = ?`, ps.user.ID)
		db.Exec(`DELETE FROM conversations WHERE psychic_id = ?`, ps.user.ID)
		db.Exec(`DELETE FROM users WHERE email LIKE ?`, "alias%-"+marker+"@load.example.com")
	})
	type convRow struct {
		ID, ClientID uuid.UUID
		DisplayName  string
	}
	var convs []convRow
	if err := db.Raw(`SELECT c.id, c.client_id, u.display_name FROM conversations c JOIN users u ON u.id = c.client_id WHERE c.psychic_id = ?`, ps.user.ID).Scan(&convs).Error; err != nil {
		t.Fatal(err)
	}
	if len(convs) != largeAdvisorClients {
		t.Fatalf("%d conversations", len(convs))
	}
	// Aliases sealed exactly as the service does, for 60% of the clients.
	key, err := e.app.Config.EncryptionKey()
	if err != nil {
		t.Fatal(err)
	}
	sealer, err := security.NewSealer(key)
	if err != nil {
		t.Fatal(err)
	}
	aliasOf := map[uuid.UUID]string{}
	var aliases []domain.ClientAlias
	now := time.Now().UTC()
	for i, c := range convs {
		if i%5 >= 3 {
			continue
		}
		alias := fmt.Sprintf("Nick %d é", i)
		enc, err := sealer.Seal(alias, "alias:"+ps.user.ID.String()+":"+c.ClientID.String())
		if err != nil {
			t.Fatal(err)
		}
		aliasOf[c.ClientID] = alias
		aliases = append(aliases, domain.ClientAlias{PsychicID: ps.user.ID, ClientID: c.ClientID, AliasEnc: enc, CreatedAt: now, UpdatedAt: now})
	}
	if err := db.CreateInBatches(aliases, 500).Error; err != nil {
		t.Fatal(err)
	}
	db.Exec("ANALYZE conversations")
	db.Exec("ANALYZE client_aliases")
	nameOf := map[uuid.UUID]string{}
	for _, c := range convs {
		nameOf[c.ClientID] = c.DisplayName
	}

	// A page of 10 and a page of 50 cost the same number of queries, and the
	// aliases come from the inbox's own statement (a join), not from lookups.
	inboxLog := func(limit int) *sqlLog {
		time.Sleep(200 * time.Millisecond)
		return recordSQL(e, func() {
			ps.must(http.StatusOK, http.MethodGet, fmt.Sprintf("/api/v1/chat/conversations?limit=%d", limit), nil, nil)
		})
	}
	small, large := inboxLog(10), inboxLog(50)
	for name, l := range map[string]*sqlLog{"10": small, "50": large} {
		if l.count("conversations", "client_aliases") != 1 || l.count(`FROM "client_aliases"`) != 0 || l.count("FROM client_aliases") != 0 {
			t.Fatalf("page %s: alias join not in the inbox query:\n%v", name, l.stmt)
		}
	}
	for name, l := range map[string]*sqlLog{"10": small, "50": large} {
		if n := l.count("conversations AS c"); n != 1 || l.count("client_aliases") != 1 || l.count("user_profiles") != 1 || l.count("conversations AS c", "client_aliases") != 1 {
			t.Fatalf("page %s: the inbox page is not one query (%d inbox statements)\n%v", name, n, l.stmt)
		}
	}

	// Walk the entire inbox: every conversation once, aliases exact, names real.
	var times []time.Duration
	cursor, got := "", map[uuid.UUID]bool{}
	for {
		path := "/api/v1/chat/conversations?limit=50"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		var page service.InboxPage
		started := time.Now()
		ps.must(http.StatusOK, http.MethodGet, path, nil, &page)
		times = append(times, time.Since(started))
		for _, v := range page.Items {
			id := v.Counterpart.ID
			if got[id] {
				t.Fatalf("client %s listed twice", id)
			}
			got[id] = true
			if v.Counterpart.Alias != aliasOf[id] || v.Counterpart.Name != nameOf[id] {
				t.Fatalf("client %s: alias %q name %q, want %q / %q", id, v.Counterpart.Alias, v.Counterpart.Name, aliasOf[id], nameOf[id])
			}
		}
		if !page.HasMore {
			break
		}
		cursor = page.NextCursor
	}
	if len(got) != largeAdvisorClients {
		t.Fatalf("inbox listed %d conversations, want %d", len(got), largeAdvisorClients)
	}
	if v := p95(times); v > largePageBudget {
		t.Fatalf("p95 inbox page %s exceeds %s", v, largePageBudget)
	} else {
		t.Logf("%d inbox pages, p95 %s", len(times), v)
	}

	// The lookup of 50 aliases is one query and exact.
	ids := make([]uuid.UUID, 0, 50)
	for _, c := range convs[:50] {
		ids = append(ids, c.ClientID)
	}
	var list aliasList
	lookup := recordSQL(e, func() { ps.must(http.StatusOK, http.MethodGet, aliasesQuery(ids...), nil, &list) })
	if lookup.count(`FROM "client_aliases"`) != 1 {
		t.Fatalf("lookup queries: %v", lookup.stmt)
	}
	want := 0
	for _, id := range ids {
		if _, ok := aliasOf[id]; ok {
			want++
		}
	}
	if len(list.Items) != want {
		t.Fatalf("lookup returned %d aliases, want %d", len(list.Items), want)
	}
	for _, it := range list.Items {
		if it.Alias != aliasOf[it.ClientID] {
			t.Fatalf("lookup alias %q for %s, want %q", it.Alias, it.ClientID, aliasOf[it.ClientID])
		}
	}
}
