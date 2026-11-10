package e2e

// revisions 4.0 (§23.4 + §9.5 pattern): the admin console can reveal an
// advisor's private nickname for a client from a session, behind step-up and
// a stated reason, audited without the nickname text.

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type adminAliasReply struct {
	Alias     *string `json:"alias"`
	UpdatedAt string  `json:"updated_at"`
	Client    struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	} `json:"client"`
	Advisor struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
		Slug string    `json:"slug"`
	} `json:"advisor"`
}

func adminAliasPath(session uuid.UUID, reason string) string {
	p := "/api/v1/admin/chat/sessions/" + session.String() + "/client-alias"
	if reason != "" {
		p += "?reason=" + url.QueryEscape(reason)
	}
	return p
}

func auditRows(e *env, action string) []struct{ Action, TargetType, TargetID, Metadata string } {
	var rows []struct{ Action, TargetType, TargetID, Metadata string }
	e.app.DB.Table("audit_logs").Select("action, target_type, target_id, metadata::text AS metadata").
		Where("action = ?", action).Scan(&rows)
	return rows
}

func TestAdminClientAliasRevealIsGuardedAndAudited(t *testing.T) {
	e := newEnv(t, roomy)
	cl, ps, _, _, s := startedSession(t, e, 10, 250)
	const nick = "Zorblax Moonbeam"
	ps.must(http.StatusOK, http.MethodPut, aliasPath(s.ConversationID), map[string]any{"alias": nick}, nil)

	// Not reachable without an admin session: a web session is refused by the
	// origin check, and a client on the admin origin is simply not signed in.
	if r := cl.call(http.MethodGet, adminAliasPath(s.ID, "Review"), nil); r.status != http.StatusForbidden {
		t.Fatalf("client on the web origin: %d %s", r.status, r.body)
	}
	if r := ps.call(http.MethodGet, adminAliasPath(s.ID, "Review"), nil); r.status != http.StatusForbidden {
		t.Fatalf("advisor on the web origin: %d %s", r.status, r.body)
	}
	for name, who := range map[string]*client{"client": cl, "advisor": ps} {
		onAdmin := e.adminClient()
		onAdmin.http.Jar = who.http.Jar
		if r := onAdmin.call(http.MethodGet, adminAliasPath(s.ID, "Review"), nil); r.status != http.StatusUnauthorized {
			t.Fatalf("%s session on the admin origin: %d %s", name, r.status, r.body)
		}
	}
	anon := e.adminClient()
	if r := anon.call(http.MethodGet, adminAliasPath(s.ID, "Review"), nil); r.status != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d %s", r.status, r.body)
	}

	admin := e.createAdmin()
	e.clock.Advance(11 * time.Minute) // the step-up window from the sign-in has passed
	// Step-up first (like the transcript), before anything else is checked.
	if r := admin.call(http.MethodGet, adminAliasPath(s.ID, "Dispute review"), nil); r.errCode(t) != "step_up_required" {
		t.Fatalf("no step-up: %d %s", r.status, r.body)
	}
	admin.stepUp()

	// Reason: required, trimmed, at most 300 characters.
	for name, reason := range map[string]string{"missing": "", "blank": "   ", "too long": strings.Repeat("a", 301)} {
		r := admin.call(http.MethodGet, adminAliasPath(s.ID, reason), nil)
		if r.status != http.StatusUnprocessableEntity || r.errCode(t) != "validation_failed" || !strings.Contains(string(r.body), `"reason"`) {
			t.Fatalf("%s reason: %d %s", name, r.status, r.body)
		}
	}
	if r := admin.call(http.MethodGet, adminAliasPath(s.ID, strings.Repeat("é", 300)), nil); r.status != http.StatusOK {
		t.Fatalf("300 characters must pass: %d %s", r.status, r.body)
	}
	// An unknown session is a 404.
	if r := admin.call(http.MethodGet, adminAliasPath(uuid.New(), "Dispute review"), nil); r.status != http.StatusNotFound {
		t.Fatalf("unknown session: %d %s", r.status, r.body)
	}
	before := len(auditRows(e, "client_alias.viewed"))

	// With a nickname: 200 with the text and both names.
	var got adminAliasReply
	r := admin.must(http.StatusOK, http.MethodGet, adminAliasPath(s.ID, "  Dispute review  "), nil, &got)
	if cc := r.header.Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Fatalf("Cache-Control: %q", cc)
	}
	if got.Alias == nil || *got.Alias != nick || got.UpdatedAt == "" {
		t.Fatalf("alias reply: %s", r.body)
	}
	if got.Client.ID != cl.user.ID || got.Client.Name != s.Client.Name || got.Client.Name == nick {
		t.Fatalf("client: %+v (session shows %q)", got.Client, s.Client.Name)
	}
	if got.Advisor.ID != ps.user.ID || got.Advisor.Name != s.Psychic.Name || got.Advisor.Slug != ps.slug() {
		t.Fatalf("advisor: %+v", got.Advisor)
	}

	// Audited with the reason and found=true, never the nickname.
	rows := auditRows(e, "client_alias.viewed")
	if len(rows) != before+1 {
		t.Fatalf("audit rows: %d (was %d)", len(rows), before)
	}
	var found bool
	for _, a := range rows {
		if strings.Contains(a.Metadata, "Zorblax") || strings.Contains(a.Metadata, "Moonbeam") {
			t.Fatalf("audit metadata leaks the nickname: %s", a.Metadata)
		}
		if a.TargetType == "user" && a.TargetID == cl.user.ID.String() && strings.Contains(a.Metadata, `"Dispute review"`) &&
			strings.Contains(a.Metadata, `"found": true`) && strings.Contains(a.Metadata, s.ID.String()) &&
			strings.Contains(a.Metadata, s.ConversationID.String()) && strings.Contains(a.Metadata, ps.user.ID.String()) {
			found = true
		}
	}
	if !found {
		t.Fatalf("no matching audit row: %+v", rows)
	}
	// The audit API does not carry it either.
	if r := admin.call(http.MethodGet, "/api/v1/admin/audit-logs?action=client_alias.viewed", nil); r.status != http.StatusOK || strings.Contains(string(r.body), "Zorblax") {
		t.Fatalf("audit listing: %d %s", r.status, r.body)
	}

	// The session and the transcript never carry the nickname.
	ps.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/messages", map[string]any{"body": "hello there", "client_msg_id": uniq("m-")}, nil)
	sess := admin.call(http.MethodGet, "/api/v1/admin/chat/sessions/"+s.ID.String(), nil)
	trans := admin.call(http.MethodGet, "/api/v1/admin/chat/sessions/"+s.ID.String()+"/transcript?reason=Review", nil)
	for name, resp := range map[string]response{"session": sess, "transcript": trans} {
		if resp.status != http.StatusOK || strings.Contains(string(resp.body), "Zorblax") || strings.Contains(string(resp.body), "alias") {
			t.Fatalf("%s response: %d %s", name, resp.status, resp.body)
		}
	}

	// After the advisor clears it, the endpoint answers null (and audits found=false).
	ps.must(http.StatusNoContent, http.MethodDelete, aliasPath(s.ConversationID), nil, nil)
	var none adminAliasReply
	r = admin.must(http.StatusOK, http.MethodGet, adminAliasPath(s.ID, "Follow-up"), nil, &none)
	if none.Alias != nil || none.UpdatedAt != "" || !strings.Contains(string(r.body), `"alias":null`) {
		t.Fatalf("after clear: %s", r.body)
	}
	if none.Client.ID != cl.user.ID || none.Advisor.ID != ps.user.ID {
		t.Fatalf("names without a nickname: %s", r.body)
	}
	foundFalse := false
	for _, a := range auditRows(e, "client_alias.viewed") {
		if strings.Contains(a.Metadata, `"Follow-up"`) && strings.Contains(a.Metadata, `"found": false`) {
			foundFalse = true
		}
	}
	if !foundFalse {
		t.Fatal("no found=false audit row")
	}
}

func TestAdminClientAliasIsScopedToTheSessionsAdvisorAndClient(t *testing.T) {
	e := newEnv(t, roomy)
	cl, ps, _, _, s := startedSession(t, e, 10, 250)
	ps.must(http.StatusOK, http.MethodPut, aliasPath(s.ConversationID), map[string]any{"alias": "Sunny"}, nil)

	// Another advisor's nickname for the same client is not returned for this
	// session, and this advisor's nickname is not returned for a session the
	// same client had with someone else.
	other := e.createPsychic(10)
	_, _, _, _, s2 := startedSessionWith(t, e, other, 250)
	other.must(http.StatusOK, http.MethodPut, aliasPath(s2.ConversationID), map[string]any{"alias": "Other advisor's nickname"}, nil)

	admin := e.createAdmin()
	admin.stepUp()
	var got adminAliasReply
	admin.must(http.StatusOK, http.MethodGet, adminAliasPath(s.ID, "Scope check"), nil, &got)
	if got.Alias == nil || *got.Alias != "Sunny" || got.Client.ID != cl.user.ID {
		t.Fatalf("session 1: %+v", got)
	}
	admin.must(http.StatusOK, http.MethodGet, adminAliasPath(s2.ID, "Scope check"), nil, &got)
	if got.Alias == nil || *got.Alias != "Other advisor's nickname" || got.Advisor.ID != other.user.ID {
		t.Fatalf("session 2: %+v", got)
	}
}

func TestAdminClientAliasIsGoneAfterTheClientIsErased(t *testing.T) {
	e := newEnv(t, roomy)
	cl, ps, _, _, s := startedSession(t, e, 10, 250)
	ps.must(http.StatusOK, http.MethodPut, aliasPath(s.ConversationID), map[string]any{"alias": "Gone Soon"}, nil)

	admin := e.createAdmin()
	admin.stepUp()
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/users/"+cl.user.ID.String(), map[string]any{"reason": "GDPR erasure"}, nil)

	r := admin.call(http.MethodGet, adminAliasPath(s.ID, "After erasure"), nil)
	if strings.Contains(string(r.body), "Gone Soon") {
		t.Fatalf("erased client's nickname returned: %d %s", r.status, r.body)
	}
	t.Logf("after erasure: %d %s", r.status, r.body)
	switch r.status {
	case http.StatusNotFound:
	case http.StatusOK:
		var got adminAliasReply
		r.decode(t, &got)
		if got.Alias != nil {
			t.Fatalf("alias after erasure: %s", r.body)
		}
	default:
		t.Fatalf("after erasure: %d %s", r.status, r.body)
	}
}
