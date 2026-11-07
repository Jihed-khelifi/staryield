package e2e

// revisions 3.0: support desk, workflow rules (section 16.3): requester
// replies, internal notes, assignment and priority, closing, signed-in
// requesters and notifications.

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

func TestSupportDeskRequesterReplyReopensAndEmailsTheTeam(t *testing.T) {
	e, admin := newDesk(t)
	inbox := e.app.Config.Mail.SupportInbox
	who := uniq("visitor") + "@example.com"
	rec := e.newTicket(e.client(), who, "Needs a follow-up", "First message with enough characters.", map[string]any{"name": "Vera"})
	id := e.ticketID(rec.Reference)
	token := e.tokenFrom(who, rec.Reference)
	admin.stepUp()
	adminReply(t, admin, id.String(), "Staff answer number one.", "public")
	if p := adminPage(t, admin, "?q="+rec.Reference); p.Items[0].Status != "pending" || p.Items[0].NeedsReply || p.Items[0].Unread {
		t.Fatalf("after the staff reply: %+v", p.Items[0])
	}
	before := len(e.mailsTo(inbox))

	// The requester answers through the link: the ticket re-opens and is unread.
	anon := e.client()
	r := anon.linkReq(http.MethodPost, rec.Reference, token, map[string]any{"body": "Customer answer with <b>markup</b> & more."})
	if r.status != http.StatusCreated {
		t.Fatalf("reply: %d %s", r.status, r.body)
	}
	var v service.CustomerTicket
	r.decode(t, &v)
	if v.Status != "open" || v.Awaiting != "support" || !v.CanReply || len(v.Messages) != 3 || v.Messages[2].Author.Type != "customer" ||
		v.Messages[2].Body != "Customer answer with <b>markup</b> & more." {
		t.Fatalf("view after reply: %+v", v)
	}
	row := adminPage(t, admin, "?q="+rec.Reference).Items[0]
	if row.Status != "open" || !row.NeedsReply || !row.Unread || row.MessageCount != 3 || row.Version != 3 {
		t.Fatalf("row after the customer reply: %+v", row)
	}
	// The team gets the reply with the reference and a console link; the support inbox as nobody is assigned.
	team := e.mailsTo(inbox)
	if len(team) != before+1 {
		t.Fatalf("support inbox mails: %d → %d", before, len(team))
	}
	m := team[len(team)-1]
	if !strings.Contains(m.Subject, rec.Reference) || !strings.Contains(m.Text, "Customer answer with <b>markup</b> & more.") ||
		!strings.Contains(m.Text, "http://localhost:3002/support/"+id.String()) || !strings.Contains(m.Text, "Vera") ||
		strings.Contains(m.Text, token) {
		t.Fatalf("team e-mail: %q / %q", m.Subject, m.Text)
	}
	if strings.Contains(m.HTML, "<b>markup</b>") {
		t.Fatal("requester text must be escaped in the HTML part")
	}

	// Assigned tickets go to the assignee instead of the inbox.
	boss := e.secondAdmin()
	if r := adminPatch(admin, id.String(), map[string]any{"assignee_id": boss.adminID.String()}); r.status != http.StatusOK {
		t.Fatalf("assign: %d %s", r.status, r.body)
	}
	adminReply(t, admin, id.String(), "Staff answer number two.", "public")
	inboxBefore, bossBefore := len(e.mailsTo(inbox)), len(e.mailsTo(boss.user.Email))
	if r := anon.linkReq(http.MethodPost, rec.Reference, token, map[string]any{"body": "Second customer answer."}); r.status != http.StatusCreated {
		t.Fatalf("second reply: %d %s", r.status, r.body)
	}
	if got := len(e.mailsTo(boss.user.Email)); got != bossBefore+1 {
		t.Fatalf("assignee mails: %d → %d", bossBefore, got)
	}
	if got := len(e.mailsTo(inbox)); got != inboxBefore {
		t.Fatalf("the inbox must not get assigned tickets' replies: %d → %d", inboxBefore, got)
	}

	// Resolved tickets re-open as well and lose their resolution time.
	admin.stepUp()
	adminReply(t, admin, id.String(), "Marking this as solved.", "public", "resolved")
	if p := adminPage(t, admin, "?q="+rec.Reference).Items[0]; p.Status != "resolved" || p.ResolvedAt == nil {
		t.Fatalf("resolved: %+v", p)
	}
	if r := anon.linkReq(http.MethodPost, rec.Reference, token, map[string]any{"body": "Actually it is not solved."}); r.status != http.StatusCreated {
		t.Fatalf("reply to a resolved ticket: %d %s", r.status, r.body)
	}
	if p := adminPage(t, admin, "?q="+rec.Reference).Items[0]; p.Status != "open" || p.ResolvedAt != nil {
		t.Fatalf("re-opened: %+v", p)
	}
	if e.ticketCols(rec.Reference)["resolved_at"] != nil {
		t.Fatal("resolved_at must be cleared on re-open")
	}

	// Validation and CSRF.
	for name, body := range map[string]any{"empty": map[string]any{"body": ""}, "blank": map[string]any{"body": " \n "},
		"too long": map[string]any{"body": strings.Repeat("a", 5001)}, "missing": map[string]any{}} {
		r := anon.linkReq(http.MethodPost, rec.Reference, token, body)
		if r.status != http.StatusUnprocessableEntity || r.errCode(t) != "validation_failed" || !strings.Contains(string(r.body), `"body"`) {
			t.Errorf("%s: %d %s", name, r.status, r.body)
		}
	}
	if r := anon.linkReq(http.MethodPost, rec.Reference, token, map[string]any{"body": strings.Repeat("é", 5000)}); r.status != http.StatusCreated {
		t.Errorf("5000 characters (multi-byte) must be accepted: %d %s", r.status, r.body)
	}
	nocsrf := e.client()
	r = nocsrf.raw(http.MethodPost, "/api/v1/support/tickets/"+rec.Reference+"/messages", strings.NewReader(`{"body":"no csrf token"}`),
		map[string]string{"Content-Type": "application/json", "X-Support-Token": token})
	if r.status != http.StatusForbidden || r.errCode(t) != "csrf_failed" {
		t.Fatalf("CSRF is required on POST: %d %s", r.status, r.body)
	}
	if r := anon.linkReq(http.MethodPost, rec.Reference, token, map[string]any{"body": "hello there", "extra": 1}); r.status != http.StatusBadRequest && r.status != http.StatusUnprocessableEntity {
		t.Errorf("unknown fields must be rejected: %d", r.status)
	}
}

func TestSupportDeskInternalNotesAreNeverShownToTheRequester(t *testing.T) {
	e, admin := newDesk(t)
	who := uniq("note") + "@example.com"
	client := e.registerClient()
	rec := e.newTicket(client, client.user.Email, "Sensitive account question", "Please look at my account state.")
	anonWho := who
	anonRec := e.newTicket(e.client(), anonWho, "Anonymous question", "Anonymous requester message here.")
	for _, tc := range []struct {
		ref   string
		token string
		view  func() service.CustomerTicket
	}{
		{rec.Reference, "", func() service.CustomerTicket {
			var v service.CustomerTicket
			client.must(http.StatusOK, http.MethodGet, "/api/v1/me/support/tickets/"+rec.Reference, nil, &v)
			return v
		}},
		{anonRec.Reference, e.tokenFrom(anonWho, anonRec.Reference), func() service.CustomerTicket {
			return linkView(t, e.client(), anonRec.Reference, e.tokenFrom(anonWho, anonRec.Reference))
		}},
	} {
		id := e.ticketID(tc.ref)
		admin.stepUp()
		mailsBefore := len(e.flushMail())
		d := adminReply(t, admin, id.String(), "INTERNAL-NOTE-SECRET fraud suspicion", "internal")
		if d.Status != "open" || d.MessageCount != 2 || d.FirstResponseAt != nil || d.Messages[1].Visibility != "internal" ||
			d.Messages[1].Author.Type != "staff" || d.Messages[1].EmailStatus != "" {
			t.Fatalf("internal note: %+v", d)
		}
		if got := len(e.flushMail()); got != mailsBefore {
			t.Fatalf("an internal note must never be e-mailed (%d → %d)", mailsBefore, got)
		}
		if e.auditCount("support_ticket.noted", id) != 1 || strings.Contains(strings.ToLower(toJSON(auditMeta(e, "support_ticket.noted", id))), "secret") {
			t.Fatal("note audit")
		}
		// A note with a status moves the ticket, and still stays internal.
		d = adminReply(t, admin, id.String(), "INTERNAL-2 waiting for the bank", "internal", "pending")
		if d.Status != "pending" || d.FirstResponseAt != nil {
			t.Fatalf("note with status: %+v", d)
		}
		v := tc.view()
		if len(v.Messages) != 1 {
			t.Fatalf("the requester sees %d messages, want only the original", len(v.Messages))
		}
		for _, m := range v.Messages {
			if strings.Contains(m.Body, "INTERNAL") {
				t.Fatalf("internal note leaked: %+v", m)
			}
		}
		// Neither does the raw response contain it, nor system or staff identity details.
		var raw response
		if tc.token == "" {
			raw = client.call(http.MethodGet, "/api/v1/me/support/tickets/"+tc.ref, nil)
		} else {
			raw = e.client().linkReq(http.MethodGet, tc.ref, tc.token, nil)
		}
		for _, secret := range []string{"INTERNAL", "fraud", "bank", `"system"`, `"internal"`, admin.user.Email, "email_status", "staff_seen_at"} {
			if strings.Contains(string(raw.body), secret) {
				t.Fatalf("response leaks %q: %s", secret, raw.body)
			}
		}
		// Admins still see everything.
		admin.stepUp()
		if d := adminDetail(t, admin, id.String()); len(d.Messages) != 3 || d.Messages[2].Visibility != "internal" {
			t.Fatalf("admin thread: %+v", d.Messages)
		}
	}
	// Staff replies never carry the staff e-mail; the exported/listed name is a first name only.
	id := e.ticketID(rec.Reference)
	admin.stepUp()
	adminReply(t, admin, id.String(), "Public answer for the requester.", "public")
	var v service.CustomerTicket
	client.must(http.StatusOK, http.MethodGet, "/api/v1/me/support/tickets/"+rec.Reference, nil, &v)
	if len(v.Messages) != 2 || v.Messages[1].Author.Name != "Staryield Support (Admin)" {
		t.Fatalf("public reply: %+v", v.Messages)
	}
}

func toJSON(v any) string { return strings.ToLower(fmt.Sprint(v)) }

func TestSupportDeskAssignPriorityAndVersionConflict(t *testing.T) {
	e, admin := newDesk(t)
	boss := e.secondAdmin()
	rec := e.newTicket(e.client(), uniq("v")+"@example.com", "Workflow fields", "Testing assignment and priority fields.")
	id := e.ticketID(rec.Reference)
	// PATCH is not step-up protected: prove it with a stale step-up window.
	e.clock.Advance(11 * time.Minute)
	patch := func(payload map[string]any) response { return adminPatch(admin, id.String(), payload) }

	var row service.AdminTicketRow
	r := patch(map[string]any{"assignee_id": boss.adminID.String(), "priority": "urgent", "expected_version": 1})
	if r.status != http.StatusOK {
		t.Fatalf("patch: %d %s", r.status, r.body)
	}
	r.decode(t, &row)
	if row.Assignee == nil || row.Assignee.ID != boss.adminID || row.Assignee.Name != "Admin" || row.Priority != "urgent" || row.Version != 2 ||
		row.Status != "open" || row.Reference != rec.Reference {
		t.Fatalf("row: %+v", row)
	}
	// "assignee=me" and "assignee=<uuid>" find it; the summary counts it as the assignee's.
	if p := adminPage(t, boss, "?assignee=me&q="+rec.Reference); len(p.Items) != 1 {
		t.Fatal("assignee=me")
	}
	if p := adminPage(t, admin, "?assignee=me&q="+rec.Reference); len(p.Items) != 0 {
		t.Fatal("assignee=me for another admin")
	}
	if p := adminPage(t, admin, "?assignee="+boss.adminID.String()+"&priority=urgent&q="+rec.Reference); len(p.Items) != 1 {
		t.Fatal("assignee=<uuid>")
	}
	if p := adminPage(t, admin, "?assignee=unassigned&q="+rec.Reference); len(p.Items) != 0 {
		t.Fatal("assigned tickets are not unassigned")
	}

	// Optimistic concurrency.
	r = patch(map[string]any{"priority": "low", "expected_version": 1})
	if r.status != http.StatusConflict || r.errCode(t) != "version_conflict" || !strings.Contains(string(r.body), `"current_version":2`) {
		t.Fatalf("stale version: %d %s", r.status, r.body)
	}
	if e.ticketCols(rec.Reference)["priority"] != "urgent" {
		t.Fatal("a conflicting patch must not apply")
	}
	if r := patch(map[string]any{"priority": "low", "expected_version": 2}); r.status != http.StatusOK {
		t.Fatalf("current version: %d %s", r.status, r.body)
	}
	// Without expected_version the last write wins.
	if r := patch(map[string]any{"status": "pending"}); r.status != http.StatusOK {
		t.Fatalf("no expected_version: %d", r.status)
	}

	// The assignee must be an active administrator.
	client := e.registerClient()
	deactivated := e.secondAdmin()
	e.app.DB.Model(&domain.User{}).Where("id = ?", deactivated.adminID).Update("status", domain.UserDeactivated)
	for name, assignee := range map[string]any{"a client": client.user.ID.String(), "an unknown user": uuid.NewString(),
		"a deactivated admin": deactivated.adminID.String(), "not a uuid": "nobody", "empty": ""} {
		r := patch(map[string]any{"assignee_id": assignee})
		if r.status != http.StatusUnprocessableEntity || r.errCode(t) != "validation_failed" || !strings.Contains(string(r.body), "assignee_id") {
			t.Errorf("assignee %s: %d %s", name, r.status, r.body)
		}
	}
	// Other fields are validated too, all at once.
	r = patch(map[string]any{"status": "done", "priority": "critical"})
	if r.status != http.StatusUnprocessableEntity || !strings.Contains(string(r.body), `"status"`) || !strings.Contains(string(r.body), `"priority"`) {
		t.Fatalf("invalid status/priority: %d %s", r.status, r.body)
	}
	// Unassign with null; an absent assignee_id leaves it alone.
	if r := patch(map[string]any{"priority": "high"}); r.status != http.StatusOK {
		t.Fatal(r.status)
	}
	if p := adminPage(t, admin, "?q="+rec.Reference).Items[0]; p.Assignee == nil {
		t.Fatal("an absent assignee_id must keep the assignee")
	}
	r = patch(map[string]any{"assignee_id": nil})
	row = service.AdminTicketRow{}
	r.decode(t, &row)
	if r.status != http.StatusOK || row.Assignee != nil {
		t.Fatalf("unassign: %d %+v", r.status, row)
	}
	// An empty patch changes nothing (not even the version).
	v := row.Version
	r = patch(map[string]any{})
	row = service.AdminTicketRow{}
	r.decode(t, &row)
	if r.status != http.StatusOK || row.Version != v {
		t.Fatalf("empty patch: %d version %d → %d", r.status, v, row.Version)
	}
	if r := patch(map[string]any{"priority": row.Priority}); r.status != http.StatusOK {
		t.Fatal("same value")
	}
	if r := adminPatch(admin, uuid.NewString(), map[string]any{"priority": "low"}); r.status != http.StatusNotFound {
		t.Fatalf("unknown ticket: %d", r.status)
	}

	// Audit: fields with before and after values, never the request text.
	var audits []domain.AuditLog
	e.app.DB.Where("action = ? AND target_id = ?", "support_ticket.updated", id.String()).Order("created_at").Find(&audits)
	if len(audits) < 4 {
		t.Fatalf("audit rows: %d", len(audits))
	}
	// The fake clock does not move between writes, so find the first change by content.
	first := ""
	for _, a := range audits {
		if j := toJSON(a.Metadata); strings.Contains(j, "urgent") {
			first = j
		}
	}
	for _, want := range []string{"assignee_id", "priority", "urgent", boss.adminID.String(), "from", "to"} {
		if !strings.Contains(first, strings.ToLower(want)) {
			t.Errorf("audit metadata lacks %q: %s", want, first)
		}
	}
}

func TestSupportDeskClosedTicketsRefuseReplies(t *testing.T) {
	e, admin := newDesk(t)
	who := uniq("visitor") + "@example.com"
	anon := e.client()
	anon.locale = "fr"
	rec := e.newTicket(anon, who, "To be closed", "A request that will be closed shortly.")
	id := e.ticketID(rec.Reference)
	token := e.tokenFrom(who, rec.Reference)

	var row service.AdminTicketRow
	r := adminPatch(admin, id.String(), map[string]any{"status": "closed"})
	r.decode(t, &row)
	if r.status != http.StatusOK || row.Status != "closed" || e.ticketCols(rec.Reference)["closed_at"] == nil {
		t.Fatalf("close: %d %+v", r.status, row)
	}
	admin.stepUp()
	for _, vis := range []string{"public", "internal"} {
		r := admin.call(http.MethodPost, adminTickets+"/"+id.String()+"/messages", map[string]any{"body": "Too late.", "visibility": vis})
		if r.status != http.StatusConflict || r.errCode(t) != "invalid_state" {
			t.Fatalf("staff %s reply on a closed ticket: %d %s", vis, r.status, r.body)
		}
	}
	// The requester is refused too, in the language of the request.
	r = anon.linkReq(http.MethodPost, rec.Reference, token, map[string]any{"body": "Can I still write?"})
	if r.status != http.StatusConflict || r.errCode(t) != "invalid_state" || !strings.Contains(string(r.body), "fermée") {
		t.Fatalf("requester reply: %d %s", r.status, r.body)
	}
	v := linkView(t, e.client(), rec.Reference, token)
	if v.CanReply || v.Status != "closed" || v.Awaiting != "none" || len(v.Messages) != 1 {
		t.Fatalf("closed view: %+v", v)
	}
	if got := e.ticketCols(rec.Reference)["message_count"]; got.(int32) != 1 {
		t.Fatalf("nothing may be stored: %v", got)
	}
	// An administrator can re-open it (PATCH), which clears the retention clock.
	adminPatch(admin, id.String(), map[string]any{"status": "open"})
	if e.ticketCols(rec.Reference)["closed_at"] != nil {
		t.Fatal("closed_at must be cleared on re-open")
	}
	anon.linkReq(http.MethodPost, rec.Reference, token, map[string]any{"body": "Thanks for re-opening."})
	if v := linkView(t, e.client(), rec.Reference, token); !v.CanReply || len(v.Messages) != 2 {
		t.Fatalf("re-opened view: %+v", v)
	}
}

func TestSupportDeskThreadLimit(t *testing.T) {
	e, admin := newDesk(t)
	who := uniq("visitor") + "@example.com"
	rec := e.newTicket(e.client(), who, "Long thread", "A request with a very long conversation.")
	id := e.ticketID(rec.Reference)
	token := e.tokenFrom(who, rec.Reference)
	e.app.DB.Table("support_tickets").Where("id = ?", id).Update("message_count", 199)
	admin.stepUp()
	adminReply(t, admin, id.String(), "The 200th message.", "internal")
	if got := e.ticketCols(rec.Reference)["message_count"]; got.(int32) != 200 {
		t.Fatalf("count: %v", got)
	}
	r := admin.call(http.MethodPost, adminTickets+"/"+id.String()+"/messages", map[string]any{"body": "One too many.", "visibility": "public"})
	if r.status != http.StatusConflict || r.errCode(t) != "limit_reached" {
		t.Fatalf("staff: %d %s", r.status, r.body)
	}
	r = e.client().linkReq(http.MethodPost, rec.Reference, token, map[string]any{"body": "One too many from me."})
	if r.status != http.StatusConflict || r.errCode(t) != "limit_reached" {
		t.Fatalf("requester: %d %s", r.status, r.body)
	}
	// The database refuses more than 200 as well.
	if err := e.app.DB.Table("support_tickets").Where("id = ?", id).Update("message_count", 201).Error; err == nil {
		t.Fatal("message_count above 200 must violate the CHECK")
	}
}

func TestSupportDeskSignedInRequesterSeesOnlyTheirTickets(t *testing.T) {
	e, admin := newDesk(t, withPush(t))
	owner := e.registerClient()
	endpoint := "https://fcm.googleapis.com/fcm/send/" + uniq("dev")
	owner.must(http.StatusCreated, http.MethodPost, "/api/v1/me/push-subscriptions", browserSubscription(t, endpoint), nil)
	e.setPrefs(owner.user.ID, map[string]any{"notify_push": true})
	other := e.registerClient()

	mk := func(c *client, subject string) service.TicketReceipt {
		return e.newTicket(c, c.user.Email, subject, "Message for "+subject+" with enough text.")
	}
	first, second, third := mk(owner, "First request"), mk(owner, "Second request"), mk(owner, "Third request")
	foreign := mk(other, "Foreign request")
	// The same address typed anonymously is NOT linked to the account: matched on user_id only.
	anonSame := e.newTicket(e.client(), owner.user.Email, "Anonymous with my address", "Typed while signed out, same address.")

	var page service.Page[service.CustomerTicketRow]
	owner.must(http.StatusOK, http.MethodGet, "/api/v1/me/support/tickets", nil, &page)
	if len(page.Items) != 3 || page.Items[0].Reference != third.Reference || page.Items[2].Reference != first.Reference {
		t.Fatalf("list (latest activity first): %+v", page.Items)
	}
	for _, it := range page.Items {
		if it.Reference == foreign.Reference || it.Reference == anonSame.Reference {
			t.Fatalf("foreign ticket in my list: %+v", it)
		}
		if it.Status != "open" || it.Awaiting != "support" || it.Topic != "general" || it.CreatedAt.IsZero() || it.LastMessageAt.IsZero() {
			t.Fatalf("row: %+v", it)
		}
	}
	// Pagination: limit 2, then the rest.
	owner.must(http.StatusOK, http.MethodGet, "/api/v1/me/support/tickets?limit=2", nil, &page)
	if len(page.Items) != 2 || page.NextCursor == "" {
		t.Fatalf("page 1: %+v", page)
	}
	var page2 service.Page[service.CustomerTicketRow]
	owner.must(http.StatusOK, http.MethodGet, "/api/v1/me/support/tickets?limit=2&cursor="+page.NextCursor, nil, &page2)
	if len(page2.Items) != 1 || page2.NextCursor != "" || page2.Items[0].Reference != first.Reference {
		t.Fatalf("page 2: %+v", page2)
	}
	for _, q := range []string{"?limit=51", "?limit=0", "?limit=x", "?cursor=garbage"} {
		if r := owner.call(http.MethodGet, "/api/v1/me/support/tickets"+q, nil); r.status != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d", q, r.status)
		}
	}
	if r := e.client().call(http.MethodGet, "/api/v1/me/support/tickets", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("anonymous list: %d", r.status)
	}

	// Detail: mine yes, another user's ticket and the anonymous one: 404.
	var d service.CustomerTicket
	owner.must(http.StatusOK, http.MethodGet, "/api/v1/me/support/tickets/"+first.Reference, nil, &d)
	if d.Reference != first.Reference || len(d.Messages) != 1 || d.Messages[0].Body != "Message for First request with enough text." || !d.CanReply {
		t.Fatalf("detail: %+v", d)
	}
	for _, ref := range []string{foreign.Reference, anonSame.Reference, "SY-AAAAAA", "garbage"} {
		if r := owner.call(http.MethodGet, "/api/v1/me/support/tickets/"+ref, nil); r.status != http.StatusNotFound || r.errCode(t) != "not_found" {
			t.Errorf("detail %s: %d %s", ref, r.status, r.body)
		}
		if r := owner.call(http.MethodPost, "/api/v1/me/support/tickets/"+ref+"/messages", map[string]any{"body": "Hijack attempt here."}); r.status != http.StatusNotFound {
			t.Errorf("reply %s: %d", ref, r.status)
		}
	}
	if got := e.ticketCols(foreign.Reference)["message_count"]; got.(int32) != 1 {
		t.Fatal("a foreign reply must not be stored")
	}
	// The same reference as another user is also 404 in the other direction.
	if r := other.call(http.MethodGet, "/api/v1/me/support/tickets/"+first.Reference, nil); r.status != http.StatusNotFound {
		t.Fatalf("other user's view of my ticket: %d", r.status)
	}

	// A signed-in reply works and moves the ticket to the top of the list.
	e.clock.Advance(time.Minute)
	owner.must(http.StatusCreated, http.MethodPost, "/api/v1/me/support/tickets/"+first.Reference+"/messages", map[string]any{"body": "A follow-up from my account."}, &d)
	if len(d.Messages) != 2 || d.Messages[1].Author.Type != "customer" {
		t.Fatalf("after reply: %+v", d)
	}
	owner.must(http.StatusOK, http.MethodGet, "/api/v1/me/support/tickets", nil, &page)
	if page.Items[0].Reference != first.Reference {
		t.Fatalf("latest activity must come first: %+v", page.Items)
	}
	if r := owner.call(http.MethodPost, "/api/v1/me/support/tickets/"+first.Reference+"/messages", map[string]any{"body": ""}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("empty reply: %d", r.status)
	}

	// The admin sees the link to the account.
	admin.stepUp()
	det := adminDetail(t, admin, first.Reference)
	if det.User == nil || det.User.ID != owner.user.ID || det.User.Email != owner.user.Email || det.User.Role != "client" || det.User.Status != "active" ||
		det.Requester.UserID == nil || *det.Requester.UserID != owner.user.ID {
		t.Fatalf("linked account: %+v", det.User)
	}
	if p := adminPage(t, admin, "?user_id="+owner.user.ID.String()); len(p.Items) != 3 {
		t.Fatalf("user_id filter: %d", len(p.Items))
	}

	// A staff reply notifies the signed-in requester in-app and by push (e-mail goes out as well).
	adminReply(t, admin, first.Reference, "We are on it, thanks for waiting.", "public")
	deadline := time.Now().Add(5 * time.Second)
	var note *service.NotificationView
	for note == nil {
		var notes struct {
			Items []service.NotificationView `json:"items"`
		}
		owner.must(http.StatusOK, http.MethodGet, "/api/v1/me/notifications", nil, &notes)
		for i := range notes.Items {
			if notes.Items[i].Kind == service.NotifySupportReplied {
				note = &notes.Items[i]
			}
		}
		if note == nil {
			if time.Now().After(deadline) {
				t.Fatalf("no support.replied notification: %+v", notes.Items)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	if note.Data["reference"] != first.Reference || note.Data["subject"] != "First request" || note.Link != "/support/requests/"+first.Reference {
		t.Fatalf("notification: %+v", note)
	}
	pushes := e.waitPushes(1)
	if p := pushes[len(pushes)-1].Payload; p.Kind != service.NotifySupportReplied || p.Title == "" || !strings.Contains(p.Body, first.Reference) ||
		!strings.Contains(p.URL, "/support/requests/"+first.Reference) || strings.Contains(p.Body, "We are on it") {
		t.Fatalf("push: %+v", p)
	}
	if got := e.mailsTo(owner.user.Email); len(got) < 2 || !strings.Contains(got[len(got)-1].Text, "We are on it, thanks for waiting.") {
		t.Fatalf("reply e-mail: %d mails", len(got))
	}
	// Internal notes, and replies to anonymous requesters, notify nobody.
	pushesBefore := len(e.push.Sent())
	adminReply(t, admin, first.Reference, "Internal chatter.", "internal")
	adminReply(t, admin, anonSame.Reference, "Answer to the anonymous one.", "public")
	time.Sleep(150 * time.Millisecond)
	if len(e.push.Sent()) != pushesBefore {
		t.Fatal("only public replies to signed-in requesters are pushed")
	}
	var notes struct {
		Items []service.NotificationView `json:"items"`
	}
	owner.must(http.StatusOK, http.MethodGet, "/api/v1/me/notifications", nil, &notes)
	count := 0
	for _, n := range notes.Items {
		if n.Kind == service.NotifySupportReplied {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("support.replied notifications: %d", count)
	}
	_ = second
}
