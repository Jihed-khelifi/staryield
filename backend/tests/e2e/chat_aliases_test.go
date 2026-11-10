package e2e

// revisions 4.0 (§23.4): private client nicknames.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

func aliasPath(conv uuid.UUID) string {
	return "/api/v1/chat/conversations/" + conv.String() + "/alias"
}

func aliasesQuery(ids ...uuid.UUID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = id.String()
	}
	return "/api/v1/chat/aliases?client_ids=" + strings.Join(parts, ",")
}

type aliasList struct {
	Items []service.ClientAliasView `json:"items"`
}

type aliasEvent struct {
	ClientID  uuid.UUID `json:"client_id"`
	Alias     *string   `json:"alias"`
	UpdatedAt time.Time `json:"updated_at"`
}

func aliasEventFrom(t *testing.T, data json.RawMessage) aliasEvent {
	t.Helper()
	var ev aliasEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		t.Fatalf("decode client_alias.updated: %v (%s)", err, data)
	}
	return ev
}

func TestClientAliasLifecycle(t *testing.T) {
	e := newEnv(t, roomy)
	cl, ps, cws, pws, s := startedSession(t, e, 10, 250)
	conv := s.ConversationID
	pws2 := ps.connect("") // a second advisor device
	pws.drain()
	pws2.drain()
	cws.drain()

	// Nothing yet: no alias anywhere.
	if got := inboxEntry(t, ps, conv).Counterpart; got.Alias != "" {
		t.Fatalf("alias before set: %+v", got)
	}
	var empty aliasList
	ps.must(http.StatusOK, http.MethodGet, aliasesQuery(cl.user.ID), nil, &empty)
	if empty.Items == nil || len(empty.Items) != 0 {
		t.Fatalf("lookup before set: %+v", empty)
	}

	// Set: trimmed, inner whitespace collapsed; the real name stays the name.
	realName := inboxEntry(t, ps, conv).Counterpart.Name
	var set service.ClientAliasView
	ps.must(http.StatusOK, http.MethodPut, aliasPath(conv), map[string]any{"alias": "  Mom   of\ttwo 🌙 "}, &set)
	if set.ClientID != cl.user.ID || set.Alias != "Mom of two 🌙" || set.UpdatedAt.IsZero() {
		t.Fatalf("set: %+v", set)
	}
	for name, w := range map[string]*ws{"advisor tab 1": pws, "advisor tab 2": pws2} {
		ev := aliasEventFrom(t, w.waitFor(realtime.EventClientAliasUpdated, 5*time.Second).Data)
		if ev.ClientID != cl.user.ID || ev.Alias == nil || *ev.Alias != "Mom of two 🌙" || ev.UpdatedAt.IsZero() {
			t.Fatalf("%s: %+v", name, ev)
		}
	}
	cws.expectNone(realtime.EventClientAliasUpdated, 300*time.Millisecond)

	// Sealed at rest: the stored value is not the text.
	var enc string
	if err := e.app.DB.Table("client_aliases").Select("alias_enc").Where("psychic_id = ? AND client_id = ?", ps.user.ID, cl.user.ID).Scan(&enc).Error; err != nil {
		t.Fatal(err)
	}
	if enc == "" || strings.Contains(enc, "Mom") {
		t.Fatalf("alias stored in clear: %q", enc)
	}
	// Audited without the text.
	var audits []struct{ Action, Metadata string }
	e.app.DB.Table("audit_logs").Select("action, metadata::text AS metadata").Where("action LIKE 'client_alias.%' AND actor_id = ?", ps.user.ID).Scan(&audits)
	if len(audits) != 1 || audits[0].Action != "client_alias.set" || strings.Contains(audits[0].Metadata, "Mom") {
		t.Fatalf("audit: %+v", audits)
	}

	// Replace; the inbox, the client card and the lookup carry it.
	ps.must(http.StatusOK, http.MethodPut, aliasPath(conv), map[string]any{"alias": "Mama Luna"}, &set)
	if set.Alias != "Mama Luna" {
		t.Fatalf("replace: %+v", set)
	}
	row := inboxEntry(t, ps, conv).Counterpart
	if row.Alias != "Mama Luna" || row.Name != realName || row.Name == row.Alias {
		t.Fatalf("inbox counterpart: %+v (real name %q)", row, realName)
	}
	var card service.ClientProfileView
	ps.must(http.StatusOK, http.MethodGet, convPath(conv, "/client"), nil, &card)
	if card.Alias != "Mama Luna" || card.Name != realName {
		t.Fatalf("client card: %+v", card)
	}
	var list aliasList
	stranger := e.registerClient()
	ps.must(http.StatusOK, http.MethodGet, aliasesQuery(cl.user.ID, stranger.user.ID, uuid.New()), nil, &list)
	if len(list.Items) != 1 || list.Items[0].ClientID != cl.user.ID || list.Items[0].Alias != "Mama Luna" {
		t.Fatalf("lookup: %+v", list)
	}

	// Clear: 204, idempotent, announced once, audited once.
	pws.drain()
	pws2.drain()
	ps.must(http.StatusNoContent, http.MethodDelete, aliasPath(conv), nil, nil)
	for name, w := range map[string]*ws{"advisor tab 1": pws, "advisor tab 2": pws2} {
		ev := aliasEventFrom(t, w.waitFor(realtime.EventClientAliasUpdated, 5*time.Second).Data)
		if ev.ClientID != cl.user.ID || ev.Alias != nil {
			t.Fatalf("%s: clear event %+v", name, ev)
		}
	}
	ps.must(http.StatusNoContent, http.MethodDelete, aliasPath(conv), nil, nil)
	pws.expectNone(realtime.EventClientAliasUpdated, 300*time.Millisecond)
	if got := inboxEntry(t, ps, conv).Counterpart.Alias; got != "" {
		t.Fatalf("alias after clear: %q", got)
	}
	ps.must(http.StatusOK, http.MethodGet, aliasesQuery(cl.user.ID), nil, &list)
	if len(list.Items) != 0 {
		t.Fatalf("lookup after clear: %+v", list)
	}
	// The fake clock gives every entry the same created_at, so count per
	// action instead of relying on row order: two sets, one clear (the
	// repeated DELETE is not audited).
	e.app.DB.Table("audit_logs").Select("action, metadata::text AS metadata").Where("action LIKE 'client_alias.%' AND actor_id = ?", ps.user.ID).Scan(&audits)
	byAction := map[string]int{}
	for _, a := range audits {
		byAction[a.Action]++
	}
	if len(audits) != 3 || byAction["client_alias.set"] != 2 || byAction["client_alias.cleared"] != 1 {
		t.Fatalf("audit trail: %+v", audits)
	}
}

func TestClientAliasValidation(t *testing.T) {
	e := newEnv(t)
	_, ps, _, _, s := startedSession(t, e, 10, 250)
	path := aliasPath(s.ConversationID)
	for name, alias := range map[string]string{
		"empty":           "",
		"only whitespace": " \t\n ",
		"41 characters":   strings.Repeat("é", 41),
		"control char":    "Mo\x07m",
		"null":            "a\x00b",
		"zero width":      "a​b",
		"bidi override":   "a‮b",
	} {
		r := ps.call(http.MethodPut, path, map[string]any{"alias": alias})
		if r.status != http.StatusUnprocessableEntity || r.errCode(t) != "validation_failed" || fieldMap(t, r)["alias"] == "" {
			t.Fatalf("%s: %d %s", name, r.status, r.body)
		}
	}
	// Exactly 40 characters (counted as characters, not bytes) is fine.
	var ok service.ClientAliasView
	ps.must(http.StatusOK, http.MethodPut, path, map[string]any{"alias": strings.Repeat("é", 40)}, &ok)
	if len([]rune(ok.Alias)) != 40 {
		t.Fatalf("40 chars: %+v", ok)
	}
	// Unknown fields and a missing body are refused like everywhere else.
	if r := ps.call(http.MethodPut, path, map[string]any{"alias": "Mom", "client_id": uuid.New()}); r.errCode(t) != "validation_failed" {
		t.Fatalf("unknown field: %d %s", r.status, r.body)
	}
	if r := ps.call(http.MethodPut, path, nil); r.status < 400 {
		t.Fatalf("no body: %d", r.status)
	}
	// Lookup: 1 to 50 well-formed ids.
	ids := make([]uuid.UUID, 51)
	for i := range ids {
		ids[i] = uuid.New()
	}
	for name, q := range map[string]string{
		"none":      "/api/v1/chat/aliases",
		"empty":     "/api/v1/chat/aliases?client_ids=",
		"51":        aliasesQuery(ids...),
		"malformed": "/api/v1/chat/aliases?client_ids=" + uuid.NewString() + ",nope",
	} {
		if r := ps.call(http.MethodGet, q, nil); r.status != http.StatusUnprocessableEntity || r.errCode(t) != "validation_failed" {
			t.Fatalf("lookup %s: %d %s", name, r.status, r.body)
		}
	}
	if r := ps.call(http.MethodGet, aliasesQuery(ids[:50]...), nil); r.status != http.StatusOK {
		t.Fatalf("50 ids: %d %s", r.status, r.body)
	}
}

func TestClientAliasAccessRules(t *testing.T) {
	e := newEnv(t, roomy)
	cl, ps, _, _, s := startedSession(t, e, 10, 250)
	conv := s.ConversationID
	ps.must(http.StatusOK, http.MethodPut, aliasPath(conv), map[string]any{"alias": "Hidden"}, nil)

	// A client cannot use any alias endpoint (role), nor can an anonymous caller.
	for _, call := range []func() response{
		func() response { return cl.call(http.MethodPut, aliasPath(conv), map[string]any{"alias": "Me"}) },
		func() response { return cl.call(http.MethodDelete, aliasPath(conv), nil) },
		func() response { return cl.call(http.MethodGet, aliasesQuery(cl.user.ID), nil) },
	} {
		if r := call(); r.status != http.StatusForbidden || r.errCode(t) != "forbidden" {
			t.Fatalf("client: %d %s", r.status, r.body)
		}
	}
	if r := e.client().call(http.MethodGet, aliasesQuery(cl.user.ID), nil); r.status != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", r.status)
	}
	// Another advisor: 404 on set and clear, and the lookup is scoped to
	// their own aliases (never the first advisor's).
	other := e.createPsychic(10)
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		var payload any
		if method == http.MethodPut {
			payload = map[string]any{"alias": "Stolen"}
		}
		if r := other.call(method, aliasPath(conv), payload); r.status != http.StatusNotFound || r.errCode(t) != "not_found" {
			t.Fatalf("other advisor %s: %d %s", method, r.status, r.body)
		}
	}
	if r := other.call(http.MethodGet, convPath(conv, "/client"), nil); r.status != http.StatusNotFound {
		t.Fatalf("other advisor client card: %d", r.status)
	}
	if r := ps.call(http.MethodPut, aliasPath(uuid.New()), map[string]any{"alias": "x"}); r.status != http.StatusNotFound {
		t.Fatalf("unknown conversation: %d", r.status)
	}
	var theirs aliasList
	other.must(http.StatusOK, http.MethodGet, aliasesQuery(cl.user.ID), nil, &theirs)
	if len(theirs.Items) != 0 {
		t.Fatalf("another advisor reads aliases: %+v", theirs)
	}
	var mine aliasList
	ps.must(http.StatusOK, http.MethodGet, aliasesQuery(cl.user.ID), nil, &mine)
	if len(mine.Items) != 1 || mine.Items[0].Alias != "Hidden" {
		t.Fatalf("the author lost it: %+v", mine)
	}
}

func TestClientAliasRateLimitBucket(t *testing.T) {
	e := newEnv(t, func(cfg *config.Config) {
		cfg.RateLimit.Scale = 1
		big := config.Rule{Rate: 100000, Period: time.Hour, Burst: 100000}
		cfg.RateLimit.Global, cfg.RateLimit.Auth, cfg.RateLimit.Writes, cfg.RateLimit.Ingest = big, big, big, big
		cfg.RateLimit.Alias = config.Rule{Rate: 2, Period: time.Hour, Burst: 2}
	})
	_, ps, _, _, s := startedSession(t, e, 10, 250)
	path := aliasPath(s.ConversationID)
	ps.must(http.StatusOK, http.MethodPut, path, map[string]any{"alias": "One"}, nil)
	ps.must(http.StatusNoContent, http.MethodDelete, path, nil, nil)
	r := ps.call(http.MethodPut, path, map[string]any{"alias": "Three"})
	if r.status != http.StatusTooManyRequests || r.errCode(t) != "rate_limited" || r.header.Get("Retry-After") == "" {
		t.Fatalf("alias budget not enforced: %d %s", r.status, r.body)
	}
	// Reading is not limited by the write bucket.
	ps.must(http.StatusOK, http.MethodGet, aliasesQuery(uuid.New()), nil, nil)
}

// The privacy invariant: the alias is never in anything delivered to the
// client or to another advisor, over REST or realtime.
func TestClientAliasNeverReachesTheClientOrAnotherAdvisor(t *testing.T) {
	e := newEnv(t, roomy)
	cl, ps, cws, pws, s := startedSession(t, e, 10, 250)
	conv := s.ConversationID
	const marker = "ZqPrivateNickname"
	ps.must(http.StatusOK, http.MethodPut, aliasPath(conv), map[string]any{"alias": marker}, nil)

	// Everything the pair does while the alias exists.
	cws.drain()
	pws.drain()
	cl.mustSay(conv, "Hello from the client", "")
	reply := ps.mustSay(conv, "Hello back", "")
	cl.like(reply.ID)
	ps.must(http.StatusNoContent, http.MethodPut, "/api/v1/chat/saved/"+reply.ID.String(), nil, nil)
	pws.mustAck(realtime.FrameMessageSend, map[string]any{"conversation_id": conv, "body": "Over the socket", "client_msg_id": uniq("ws-")})
	// Presence changes travel to the advisor only; make some anyway.
	extra := cl.connect("")
	_ = extra.conn.Close()
	time.Sleep(300 * time.Millisecond)
	endSession(t, cl, s.ID)
	cl.mustSay(conv, "Off-session after the chat", "")
	ps.mustSay(conv, "Off-session answer", "")

	// A second advisor who also chatted with this client.
	other := e.createPsychic(10)
	ows := other.goOnline()
	cws.drain()
	other2 := sessionBetween(t, cl, cws, other)
	ows.drain()
	otherConv := other2.ConversationID
	other.mustSay(otherConv, "Hi from the other advisor", "")

	leaks := func(what string, raw []byte) {
		t.Helper()
		if strings.Contains(strings.ToLower(string(raw)), strings.ToLower(marker)) {
			t.Fatalf("%s leaks the alias: %s", what, raw)
		}
	}
	get := func(c *client, path string) []byte {
		t.Helper()
		r := c.call(http.MethodGet, path, nil)
		if r.status != http.StatusOK {
			t.Fatalf("GET %s: %d %s", path, r.status, r.body)
		}
		return r.body
	}
	// Client responses.
	for _, path := range []string{
		"/api/v1/chat/conversations", "/api/v1/chat/psychics/" + ps.slug(), "/api/v1/chat/psychics/" + other.slug(),
		"/api/v1/chat/sessions", "/api/v1/chat/sessions/" + s.ID.String(), "/api/v1/chat/sessions/" + s.ID.String() + "/summary",
		convMessages(conv) + "?limit=100", convMessages(conv) + "/since?after_seq=0&limit=500", "/api/v1/chat/saved",
		convAllowance(conv), "/api/v1/chat/unread-count", "/api/v1/me/notifications",
	} {
		leaks("client GET "+path, get(cl, path))
	}
	// Another advisor's responses (their own inbox, client card, lookups).
	for _, path := range []string{
		"/api/v1/chat/conversations", convPath(otherConv, "/client"), convMessages(otherConv) + "?limit=100",
		"/api/v1/chat/sessions", aliasesQuery(cl.user.ID),
	} {
		leaks("other advisor GET "+path, get(other, path))
	}
	// Realtime frames received by the client, by the other advisor and by the
	// client's other connection.
	for name, w := range map[string]*ws{"client": cws, "other advisor": ows} {
		frames := w.pending()
		if len(frames) == 0 {
			t.Fatalf("%s received no events to check", name)
		}
		for _, f := range frames {
			leaks(fmt.Sprintf("%s event %s", name, f.Type), f.Data)
			if f.Type == realtime.EventClientAliasUpdated {
				t.Fatalf("%s received %s", name, f.Type)
			}
		}
	}
	// The advisor does see it, in the dedicated field only.
	own := inboxEntry(t, ps, conv).Counterpart
	if own.Alias != marker || strings.Contains(own.Name, marker) {
		t.Fatalf("advisor inbox: %+v", own)
	}
	otherSees := inboxEntry(t, other, otherConv).Counterpart
	if otherSees.Alias != "" {
		t.Fatalf("another advisor got an alias: %+v", otherSees)
	}
	// Session views of the advisor never carry it either.
	leaks("advisor session view", get(ps, "/api/v1/chat/sessions/"+s.ID.String()))
	leaks("advisor session list", get(ps, "/api/v1/chat/sessions"))
	leaks("advisor history", get(ps, convMessages(conv)+"?limit=100"))
}

func TestClientAliasErasure(t *testing.T) {
	e := newEnv(t, roomy)
	cl, ps, _, _, s := startedSession(t, e, 10, 250)
	conv := s.ConversationID
	ps.must(http.StatusOK, http.MethodPut, aliasPath(conv), map[string]any{"alias": "Mom"}, nil)

	// The advisor's export lists the aliases they wrote (not the client's).
	files := exportFiles(t, e, ps)
	var written []struct {
		ClientID uuid.UUID `json:"client_id"`
		Alias    string    `json:"alias"`
	}
	if err := json.Unmarshal(files["advisor/client_aliases.json"], &written); err != nil || len(written) != 1 || written[0].Alias != "Mom" || written[0].ClientID != cl.user.ID {
		t.Fatalf("advisor export: %v %s", err, files["advisor/client_aliases.json"])
	}
	for name, body := range exportFiles(t, e, cl) {
		if strings.Contains(string(body), "Mom") && strings.Contains(name, "alias") {
			t.Fatalf("client export leaks the alias in %s", name)
		}
		if name == "advisor/client_aliases.json" {
			t.Fatalf("client export has an advisor alias file")
		}
	}

	// Erasing the client removes the rows about them.
	count := func(where string, args ...any) int64 {
		var n int64
		e.app.DB.Table("client_aliases").Where(where, args...).Count(&n)
		return n
	}
	if count("client_id = ?", cl.user.ID) != 1 {
		t.Fatal("alias missing before erasure")
	}
	cl.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/close", map[string]any{"password": strongPassword, "confirmation": "DELETE"}, nil)
	e.clock.Advance(31*24*time.Hour + time.Minute)
	runClosure(t, e)
	if n := count("client_id = ?", cl.user.ID); n != 0 {
		t.Fatalf("%d aliases about an erased client", n)
	}

	// Erasing an advisor removes the aliases they wrote.
	cl2, ps2, _, _, s2 := startedSession(t, e, 10, 250)
	ps2.must(http.StatusOK, http.MethodPut, aliasPath(s2.ConversationID), map[string]any{"alias": "Dear client"}, nil)
	admin := e.createAdmin()
	admin.stepUp()
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/users/"+ps2.user.ID.String(), map[string]any{"reason": "Erasure request"}, nil)
	if n := count("psychic_id = ?", ps2.user.ID); n != 0 {
		t.Fatalf("%d aliases by an erased advisor", n)
	}
	_ = cl2
}
