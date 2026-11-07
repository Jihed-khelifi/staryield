package e2e

// revisions 3.1: conversation reports (docs/platform-extensions.md section
// 16.4). A client reports a problem with a chat conversation; the report is a
// support ticket of kind "chat_report" that flows through the existing desk.

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/app"
	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

const reportsPath = "/api/v1/me/support/chat-reports"

type reportFix struct {
	e                       *env
	admin, client, advisor  *client
	other                   *client // another client with their own conversation
	conv, otherConv         uuid.UUID
	sessOld, sessNew, sessO service.SessionView
}

// chatSession requests a session with the advisor and cancels it, which leaves
// a conversation with one more (closed) session.
func (f *reportFix) chatSession(c *client) service.SessionView {
	f.e.t.Helper()
	var s service.SessionView
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": f.advisor.slug(), "request_key": uniq("req-key-")}, &s)
	c.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/cancel", nil, nil)
	f.e.clock.Advance(time.Minute) // distinct requested_at values
	return s
}

func newReportFix(t *testing.T, opts ...option) *reportFix {
	t.Helper()
	e, admin := newDesk(t, opts...)
	e.ensurePacks()
	f := &reportFix{e: e, admin: admin}
	// The database is shared by the whole suite: leave no report behind (the
	// summary tests count every open ticket).
	t.Cleanup(func() { e.app.DB.Exec("DELETE FROM support_tickets WHERE kind = 'chat_report'") })
	f.advisor = e.createPsychic(30)
	f.advisor.goOnline()
	f.client, f.other = e.registerClient(), e.registerClient()
	f.client.buyCredits(250)
	f.other.buyCredits(250)
	f.sessOld = f.chatSession(f.client)
	f.sessNew = f.chatSession(f.client)
	f.sessO = f.chatSession(f.other)
	f.conv, f.otherConv = f.sessNew.ConversationID, f.sessO.ConversationID
	if f.conv != f.sessOld.ConversationID || f.conv == f.otherConv {
		t.Fatalf("conversations: %v %v %v", f.sessOld.ConversationID, f.conv, f.otherConv)
	}
	return f
}

func (f *reportFix) post(c *client, body map[string]any) response {
	return c.call(http.MethodPost, reportsPath, body)
}

func (f *reportFix) create(c *client, body map[string]any) service.CustomerTicket {
	f.e.t.Helper()
	var out service.CustomerTicket
	c.must(http.StatusCreated, http.MethodPost, reportsPath, body, &out)
	return out
}

func (f *reportFix) open(c *client, conv uuid.UUID) struct{ Open *service.CustomerTicketRow } {
	f.e.t.Helper()
	var out struct{ Open *service.CustomerTicketRow }
	c.must(http.StatusOK, http.MethodGet, reportsPath+"?conversation_id="+conv.String(), nil, &out)
	return out
}

func (f *reportFix) closeTicket(ref string) {
	f.e.t.Helper()
	if r := adminPatch(f.admin, ref, map[string]any{"status": "closed"}); r.status != http.StatusOK {
		f.e.t.Fatalf("close %s: %d %s", ref, r.status, r.body)
	}
}

func TestChatReportCreateWithAndWithoutSession(t *testing.T) {
	f := newReportFix(t)
	e := f.e

	// Without session_id: the latest session of the conversation is linked.
	e.app.DB.Exec("UPDATE conversations SET last_seq = 12 WHERE id = ?", f.conv)
	got := f.create(f.client, map[string]any{"conversation_id": f.conv, "reason": "inappropriate", "details": "The advisor was rude to me.", "seq": 999})
	if got.Kind != "chat_report" || got.Status != "open" || got.Awaiting != "support" || !got.CanReply || got.Topic != "advisor" ||
		got.Subject != "Chat report · Inappropriate behavior · "+f.advisor.user.Psychic.Name {
		t.Fatalf("created: %+v", got)
	}
	if got.Report == nil || got.Report.Reason != "inappropriate" || *got.Report.ConversationID != f.conv || *got.Report.SessionID != f.sessNew.ID ||
		got.Report.Advisor == nil || got.Report.Advisor.Name != f.advisor.user.Psychic.Name || got.Report.Advisor.Slug != f.advisor.slug() ||
		got.Report.SessionStartedAt == nil {
		t.Fatalf("report context: %+v", got.Report)
	}
	if len(got.Messages) != 1 || got.Messages[0].Body != "The advisor was rude to me." || got.Messages[0].Author.Type != "customer" {
		t.Fatalf("thread: %+v", got.Messages)
	}
	cols := e.ticketCols(got.Reference)
	if cols["kind"] != "chat_report" || cols["report_reason"] != "inappropriate" || cols["priority"] != "urgent" || cols["topic"] != "advisor" ||
		cols["reported_user_id"] == nil || cols["email_masked"] == "" || cols["message_enc"] == "The advisor was rude to me." {
		t.Fatalf("columns: %v", cols)
	}
	if cols["reported_seq"].(int64) != 12 || cols["email_enc"] == f.client.user.Email {
		t.Fatalf("reported_seq must be clamped to last_seq (12): %v", cols["reported_seq"])
	}

	// With an explicit session_id (after the first one is closed) and no details.
	f.closeTicket(got.Reference)
	second := f.create(f.client, map[string]any{"conversation_id": f.conv, "session_id": f.sessOld.ID, "reason": "billing", "seq": 5})
	if *second.Report.SessionID != f.sessOld.ID || second.Topic != "billing" || len(second.Messages) != 0 {
		t.Fatalf("with session: %+v", second)
	}
	if c := e.ticketCols(second.Reference); c["priority"] != "high" || c["message_chars"].(int32) != 0 || c["reported_seq"].(int64) != 5 {
		t.Fatalf("billing report: %v", c)
	}
	// Topic mapping for the remaining reasons.
	f.closeTicket(second.Reference)
	for reason, topic := range map[string]string{"technical": "technical", "advisor_unresponsive": "advisor"} {
		r := f.create(f.client, map[string]any{"conversation_id": f.conv, "reason": reason})
		if r.Topic != topic {
			t.Errorf("%s: topic %s", reason, r.Topic)
		}
		f.closeTicket(r.Reference)
	}
	other := f.create(f.client, map[string]any{"conversation_id": f.conv, "reason": "other", "details": "Something odd happened."})
	if other.Topic != "general" {
		t.Errorf("other: topic %s", other.Topic)
	}
}

func TestChatReportViewsForTheRequesterAndThePrivateLink(t *testing.T) {
	f := newReportFix(t)
	e := f.e
	got := f.create(f.client, map[string]any{"conversation_id": f.conv, "reason": "technical", "details": "The chat froze twice today."})
	contact := e.newTicket(f.client, f.client.user.Email, "A contact request", "Just a normal contact request here.")

	var list service.Page[service.CustomerTicketRow]
	f.client.must(http.StatusOK, http.MethodGet, "/api/v1/me/support/tickets", nil, &list)
	byRef := map[string]service.CustomerTicketRow{}
	for _, r := range list.Items {
		byRef[r.Reference] = r
	}
	rep, con := byRef[got.Reference], byRef[contact.Reference]
	if rep.Kind != "chat_report" || rep.Report == nil || rep.Report.Reason != "technical" || rep.Report.Advisor == nil || *rep.Report.SessionID != f.sessNew.ID {
		t.Fatalf("report row: %+v", rep)
	}
	if con.Kind != "contact" || con.Report != nil {
		t.Fatalf("contact rows are unchanged: %+v", con)
	}
	var detail service.CustomerTicket
	f.client.must(http.StatusOK, http.MethodGet, "/api/v1/me/support/tickets/"+got.Reference, nil, &detail)
	if detail.Kind != "chat_report" || detail.Report == nil || detail.Report.Advisor.Slug != f.advisor.slug() {
		t.Fatalf("detail: %+v", detail)
	}
	// Somebody else's report is a 404 on every requester path.
	if r := f.other.call(http.MethodGet, "/api/v1/me/support/tickets/"+got.Reference, nil); r.status != http.StatusNotFound {
		t.Fatalf("foreign detail: %d", r.status)
	}

	// The private link (e-mailed) carries the same context.
	token := e.tokenFrom(f.client.user.Email, got.Reference)
	via := linkView(t, e.client(), got.Reference, token)
	if via.Kind != "chat_report" || via.Report == nil || via.Report.Reason != "technical" || via.Report.Advisor == nil || *via.Report.ConversationID != f.conv {
		t.Fatalf("link view: %+v", via)
	}
	// Replies work like on any ticket and re-open nothing special.
	anon := e.client()
	if r := anon.linkReq(http.MethodPost, got.Reference, token, map[string]any{"body": "Adding one more detail."}); r.status != http.StatusCreated {
		t.Fatalf("link reply: %d %s", r.status, r.body)
	}
}

func TestChatReportAdminQueueSummaryAndDetail(t *testing.T) {
	f := newReportFix(t)
	e, admin := f.e, f.admin
	// Baseline: nothing to compare against but the counters themselves.
	var before service.SupportSummary
	admin.must(http.StatusOK, http.MethodGet, adminSummary, nil, &before)

	e.app.DB.Exec("UPDATE conversations SET last_seq = 3 WHERE id = ?", f.conv)
	secret := "PRIVATE-DETAILS-" + uniq("zz")
	rep := f.create(f.client, map[string]any{"conversation_id": f.conv, "reason": "inappropriate", "details": secret, "seq": 1})
	contact := e.newTicket(e.client(), uniq("v")+"@example.com", "Unrelated contact", "Nothing to do with chats at all.")
	repOther := f.create(f.other, map[string]any{"conversation_id": f.otherConv, "reason": "billing"})

	var after service.SupportSummary
	admin.must(http.StatusOK, http.MethodGet, adminSummary, nil, &after)
	if after.ReportsOpen != before.ReportsOpen+2 || after.Open != before.Open+3 {
		t.Fatalf("summary: before %+v after %+v", before, after)
	}

	// kind filter: the metadata queue, nothing decrypted.
	page := adminPage(t, admin, "?kind=chat_report&limit=100")
	refs := map[string]service.AdminTicketRow{}
	for _, r := range page.Items {
		if r.Kind != "chat_report" || r.Report == nil {
			t.Fatalf("non-report row in the report queue: %+v", r)
		}
		refs[r.Reference] = r
	}
	if _, ok := refs[contact.Reference]; ok || refs[rep.Reference].Reference == "" || refs[repOther.Reference].Reference == "" {
		t.Fatalf("filter result: %v", refs)
	}
	row := refs[rep.Reference]
	if row.Priority != "urgent" || row.Topic != "advisor" || row.Report.Reason != "inappropriate" || row.Report.ReportedSeq == 0 ||
		*row.Report.ConversationID != f.conv || *row.Report.SessionID != f.sessNew.ID || row.Report.Advisor == nil ||
		row.Report.Advisor.UserID != f.advisor.user.ID || row.Report.Advisor.Name != f.advisor.user.Psychic.Name ||
		row.Report.Advisor.Slug != f.advisor.slug() || row.Requester.UserID == nil {
		t.Fatalf("report row: %+v / %+v", row, row.Report)
	}
	raw := admin.call(http.MethodGet, adminTickets+"?kind=chat_report", nil)
	for _, leak := range []string{secret, strings.ToLower(f.client.user.Email), "message_enc"} {
		if strings.Contains(strings.ToLower(string(raw.body)), strings.ToLower(leak)) {
			t.Fatalf("the queue leaks %q", leak)
		}
	}
	if !strings.Contains(string(raw.body), `"kind":"chat_report"`) || !strings.Contains(string(raw.body), `"reported_seq"`) {
		t.Fatalf("shape: %s", raw.body)
	}
	for _, sort := range []string{"newest", "oldest", "waiting"} {
		p := adminPage(t, admin, "?kind=chat_report&status=open,pending&sort="+sort)
		if len(p.Items) < 2 {
			t.Errorf("sort %s: %d rows", sort, len(p.Items))
		}
		for _, r := range p.Items {
			if r.Kind != "chat_report" {
				t.Errorf("sort %s: %+v", sort, r)
			}
		}
	}
	cp := adminPage(t, admin, "?kind=contact&limit=100")
	for _, r := range cp.Items {
		if r.Kind != "contact" || r.Report != nil {
			t.Fatalf("contact filter: %+v", r)
		}
	}
	if r := admin.call(http.MethodGet, adminTickets+"?kind=spam", nil); r.status != http.StatusUnprocessableEntity || !strings.Contains(string(r.body), `"kind"`) {
		t.Fatalf("invalid kind: %d %s", r.status, r.body)
	}
	// Mixed page: both kinds, contact rows without a report.
	mixed := adminPage(t, admin, "?limit=100")
	seenReport, seenContact := false, false
	for _, r := range mixed.Items {
		seenReport = seenReport || (r.Kind == "chat_report" && r.Report != nil)
		seenContact = seenContact || (r.Kind == "contact" && r.Report == nil)
	}
	if !seenReport || !seenContact {
		t.Fatal("an unfiltered page mixes both kinds")
	}

	// Detail: decrypted, with the linked session summary.
	admin.stepUp()
	d := adminDetail(t, admin, rep.Reference)
	if d.Kind != "chat_report" || d.Report == nil || d.Report.Session == nil || d.Report.Session.ID != f.sessNew.ID || d.Report.Session.Status != "cancelled" ||
		d.Report.Session.RequestedAt.IsZero() || d.Report.Session.RatePerMinute != 30 || d.Report.Session.Sponsored {
		t.Fatalf("detail report: %+v", d.Report)
	}
	if len(d.Messages) != 1 || d.Messages[0].Body != secret || d.Requester.Email != strings.ToLower(f.client.user.Email) {
		t.Fatalf("detail thread: %+v", d.Messages)
	}
	// Replies go through the same desk (the reporter gets the reply e-mail).
	adminReply(t, admin, rep.Reference, "We are looking into it.", "public")
	f.closeTicket(rep.Reference)
	var final service.SupportSummary
	admin.must(http.StatusOK, http.MethodGet, adminSummary, nil, &final)
	if final.ReportsOpen != after.ReportsOpen-1 {
		t.Fatalf("closing a report lowers reports_open: %+v", final)
	}
}

func TestChatReportTeamEmailAndTheAdvisorIsNeverNotified(t *testing.T) {
	f := newReportFix(t)
	e := f.e
	urgent := f.create(f.client, map[string]any{"conversation_id": f.conv, "reason": "inappropriate", "details": "Totally inappropriate."})
	high := f.create(f.other, map[string]any{"conversation_id": f.otherConv, "reason": "billing"})

	inbox := e.app.Config.Mail.SupportInbox
	var teamSubjects []string
	for _, m := range e.flushMail() {
		switch {
		case m.To == inbox:
			teamSubjects = append(teamSubjects, m.Subject)
		case strings.EqualFold(m.To, f.advisor.user.Email):
			t.Fatalf("the advisor must never be e-mailed about a report: %q", m.Subject)
		}
	}
	var gotUrgent, gotHigh bool
	for _, s := range teamSubjects {
		gotUrgent = gotUrgent || (strings.HasPrefix(s, "[Urgent][Chat report]") && strings.Contains(s, urgent.Reference))
		gotHigh = gotHigh || (strings.HasPrefix(s, "[Chat report]") && strings.Contains(s, high.Reference))
	}
	if !gotUrgent || !gotHigh {
		t.Fatalf("team subjects: %q", teamSubjects)
	}
	// The reporter gets the acknowledgement with the private link.
	if tok := e.tokenFrom(f.client.user.Email, urgent.Reference); tok == "" {
		t.Fatal("no private link in the acknowledgement")
	}
	// No notification, no unread, nothing about it for the advisor.
	var page struct {
		Items []map[string]any `json:"items"`
	}
	f.advisor.must(http.StatusOK, http.MethodGet, "/api/v1/me/notifications", nil, &page)
	for _, n := range page.Items {
		if strings.Contains(strings.ToLower(n["kind"].(string)), "support") {
			t.Fatalf("advisor notification: %v", n)
		}
	}
	if r := f.advisor.call(http.MethodGet, "/api/v1/me/support/tickets/"+urgent.Reference, nil); r.status != http.StatusNotFound {
		t.Fatalf("the advisor cannot read the report: %d", r.status)
	}
	if r := f.advisor.call(http.MethodGet, reportsPath+"?conversation_id="+f.conv.String(), nil); r.status != http.StatusNotFound {
		t.Fatalf("the advisor cannot learn whether a report exists: %d", r.status)
	}
}

func TestChatReportDuplicatesRaceAndReopening(t *testing.T) {
	f := newReportFix(t)
	if o := f.open(f.client, f.conv); o.Open != nil {
		t.Fatalf("nothing is open yet: %+v", o.Open)
	}
	first := f.create(f.client, map[string]any{"conversation_id": f.conv, "reason": "billing"})
	o := f.open(f.client, f.conv)
	if o.Open == nil || o.Open.Reference != first.Reference || o.Open.Kind != "chat_report" || o.Open.Report == nil || o.Open.Report.Reason != "billing" {
		t.Fatalf("open: %+v", o.Open)
	}
	if o := f.open(f.other, f.otherConv); o.Open != nil {
		t.Fatal("another client's conversation has no open report")
	}

	r := f.post(f.client, map[string]any{"conversation_id": f.conv, "reason": "technical"})
	var env struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	r.decode(t, &env)
	if r.status != http.StatusConflict || env.Error.Code != "report_exists" || env.Error.Details["reference"] != first.Reference ||
		env.Error.Message != "You already reported this conversation." {
		t.Fatalf("duplicate: %d %s", r.status, r.body)
	}
	// Pending (waiting for the requester) still blocks a new report.
	if r := adminPatch(f.admin, first.Reference, map[string]any{"status": "pending"}); r.status != http.StatusOK {
		t.Fatal(r.status)
	}
	if r := f.post(f.client, map[string]any{"conversation_id": f.conv, "reason": "technical"}); r.status != http.StatusConflict {
		t.Fatalf("pending blocks: %d", r.status)
	}
	// Resolved or closed: a new report is allowed.
	if r := adminPatch(f.admin, first.Reference, map[string]any{"status": "resolved"}); r.status != http.StatusOK {
		t.Fatal(r.status)
	}
	second := f.create(f.client, map[string]any{"conversation_id": f.conv, "reason": "technical"})
	if second.Reference == first.Reference {
		t.Fatal("a new report is a new ticket")
	}
	if o := f.open(f.client, f.conv); o.Open == nil || o.Open.Reference != second.Reference {
		t.Fatalf("open after reopening: %+v", o.Open)
	}
	f.closeTicket(second.Reference)
	if o := f.open(f.client, f.conv); o.Open != nil {
		t.Fatalf("closed is not open: %+v", o.Open)
	}

	// Race: eight simultaneous reports of one conversation yield exactly one ticket.
	var wg sync.WaitGroup
	statuses := make([]int, 8)
	bodies := make([][]byte, 8)
	f.client.ensureCSRF()
	for i := range statuses {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := f.post(f.client, map[string]any{"conversation_id": f.conv, "reason": "other", "details": "Race condition probe."})
			statuses[i], bodies[i] = r.status, r.body
		}(i)
	}
	wg.Wait()
	created, conflicts := 0, 0
	for i, s := range statuses {
		switch s {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
			if !strings.Contains(string(bodies[i]), "report_exists") || !strings.Contains(string(bodies[i]), `"reference":"SY-`) {
				t.Errorf("conflict body: %s", bodies[i])
			}
		default:
			t.Errorf("unexpected status %d: %s", s, bodies[i])
		}
	}
	if created != 1 || conflicts != 7 {
		t.Fatalf("race: %d created, %d conflicts", created, conflicts)
	}
	var n int64
	f.e.app.DB.Table("support_tickets").Where("user_id = ? AND conversation_id = ? AND status IN ('open','pending')", f.client.user.ID, f.conv).Count(&n)
	if n != 1 {
		t.Fatalf("open reports in the database: %d", n)
	}
}

func TestChatReportAuthorizationAndValidation(t *testing.T) {
	f := newReportFix(t)
	body := map[string]any{"conversation_id": f.conv, "reason": "billing"}

	// Anonymous.
	anon := f.e.client()
	if r := anon.call(http.MethodPost, reportsPath, body); r.status != http.StatusUnauthorized {
		t.Errorf("anonymous POST: %d", r.status)
	}
	if r := anon.call(http.MethodGet, reportsPath+"?conversation_id="+f.conv.String(), nil); r.status != http.StatusUnauthorized {
		t.Errorf("anonymous GET: %d", r.status)
	}
	// Advisors and admins are refused (403 forbidden); the translated message is clear.
	if r := f.post(f.advisor, body); r.status != http.StatusForbidden || r.errCode(t) != "forbidden" {
		t.Errorf("advisor POST: %d %s", r.status, r.body)
	}
	if r := f.post(f.e.registerClient(), body); r.status != http.StatusNotFound {
		t.Errorf("a client with no relation to the conversation: %d", r.status)
	}
	// A foreign and an unknown conversation are indistinguishable.
	foreign := f.post(f.other, body)
	unknown := f.post(f.other, map[string]any{"conversation_id": uuid.New(), "reason": "billing"})
	if foreign.status != http.StatusNotFound || unknown.status != http.StatusNotFound || foreign.errCode(t) != unknown.errCode(t) || !strings.Contains(string(foreign.body), "Conversation not found.") ||
		!strings.Contains(string(unknown.body), "Conversation not found.") {
		t.Errorf("404s differ: %d %s / %d %s", foreign.status, foreign.body, unknown.status, unknown.body)
	}
	for _, q := range []string{"?conversation_id=" + f.otherConv.String(), "?conversation_id=" + uuid.NewString(), "?conversation_id=nope", ""} {
		if r := f.client.call(http.MethodGet, reportsPath+q, nil); r.status != http.StatusNotFound {
			t.Errorf("GET %q: %d", q, r.status)
		}
	}
	// Nothing was stored by any of the refused calls.
	var n int64
	f.e.app.DB.Table("support_tickets").Where("kind = 'chat_report' AND user_id IN ?", []uuid.UUID{f.client.user.ID, f.other.user.ID, f.advisor.user.ID}).Count(&n)
	if n != 0 {
		t.Fatalf("refused reports created %d tickets", n)
	}

	// Validation: field errors, in the caller's language.
	type bad struct {
		name  string
		body  map[string]any
		field string
		msg   string
	}
	foreignSession := map[string]any{"conversation_id": f.conv, "reason": "billing", "session_id": f.sessO.ID}
	for _, tc := range []bad{
		{"missing reason", map[string]any{"conversation_id": f.conv}, "reason", "Choose what went wrong."},
		{"unknown reason", map[string]any{"conversation_id": f.conv, "reason": "spam"}, "reason", "Choose what went wrong."},
		{"other without details", map[string]any{"conversation_id": f.conv, "reason": "other"}, "details", "Describe the problem in 10 to 2,000 characters."},
		{"other too short", map[string]any{"conversation_id": f.conv, "reason": "other", "details": "short"}, "details", "Describe the problem in 10 to 2,000 characters."},
		{"details too long", map[string]any{"conversation_id": f.conv, "reason": "billing", "details": strings.Repeat("x", 2001)}, "details", "Use at most 2,000 characters."},
		{"session of another conversation", foreignSession, "session_id", "This session is not part of the conversation."},
		{"unknown session", map[string]any{"conversation_id": f.conv, "reason": "billing", "session_id": uuid.New()}, "session_id", "This session is not part of the conversation."},
		{"bad conversation id", map[string]any{"conversation_id": "nope", "reason": "billing"}, "conversation_id", "Invalid identifier."},
	} {
		r := f.post(f.client, tc.body)
		var v struct {
			Error struct {
				Details struct {
					Fields map[string]string `json:"fields"`
				} `json:"details"`
			} `json:"error"`
		}
		r.decode(t, &v)
		if r.status != http.StatusUnprocessableEntity || r.errCode(t) != "validation_failed" || v.Error.Details.Fields[tc.field] != tc.msg {
			t.Errorf("%s: %d %s", tc.name, r.status, r.body)
		}
	}
	// 2000 characters (multi-byte) and the 10-character minimum for "other" are accepted.
	ok := f.create(f.client, map[string]any{"conversation_id": f.conv, "reason": "other", "details": strings.Repeat("é", 2000)})
	f.closeTicket(ok.Reference)
	f.create(f.client, map[string]any{"conversation_id": f.conv, "reason": "other", "details": "0123456789"})
	// Unknown fields and a missing JSON body are rejected.
	if r := f.post(f.client, map[string]any{"conversation_id": f.conv, "reason": "billing", "kind": "contact"}); r.status != http.StatusBadRequest && r.status != http.StatusUnprocessableEntity {
		t.Errorf("unknown field: %d", r.status)
	}

	// Messages are translated for the caller (French here).
	f.client.locale = "fr"
	r := f.post(f.client, map[string]any{"conversation_id": f.conv, "reason": "nope"})
	if !strings.Contains(string(r.body), "Choisissez ce qui s'est mal passé.") {
		t.Errorf("French validation message: %s", r.body)
	}
	if r := f.post(f.client, map[string]any{"conversation_id": f.conv, "reason": "billing"}); r.status != http.StatusConflict ||
		!strings.Contains(string(r.body), "Vous avez déjà signalé cette conversation.") {
		t.Errorf("French report_exists: %d %s", r.status, r.body)
	}
}

func TestChatReportRequiresCSRF(t *testing.T) {
	f := newReportFix(t)
	f.client.ensureCSRF()
	r := f.client.raw(http.MethodPost, reportsPath, strings.NewReader(`{"conversation_id":"`+f.conv.String()+`","reason":"billing"}`),
		map[string]string{"Content-Type": "application/json"})
	if r.status != http.StatusForbidden || r.errCode(t) != "csrf_failed" {
		t.Fatalf("CSRF is required: %d %s", r.status, r.body)
	}
	var n int64
	f.e.app.DB.Table("support_tickets").Where("kind = 'chat_report' AND user_id = ?", f.client.user.ID).Count(&n)
	if n != 0 {
		t.Fatal("a request without CSRF must not store anything")
	}
}

func TestChatReportRateLimit(t *testing.T) {
	f := newReportFix(t, func(c *config.Config) { c.RateLimit.Scale = 1 })
	body := map[string]any{"conversation_id": f.conv, "reason": "billing"}
	// 10 per hour per user (duplicates count: the limit sits in front of the handler).
	for i := 1; i <= 10; i++ {
		r := f.post(f.client, body)
		want := http.StatusConflict
		if i == 1 {
			want = http.StatusCreated
		}
		if r.status != want {
			t.Fatalf("request %d: %d %s", i, r.status, r.body)
		}
	}
	r := f.post(f.client, body)
	if r.status != http.StatusTooManyRequests || r.errCode(t) != "rate_limited" || r.header.Get("Retry-After") == "" {
		t.Fatalf("the 11th request must be throttled: %d %s", r.status, r.body)
	}
	// Another user has their own budget.
	if r := f.post(f.other, map[string]any{"conversation_id": f.otherConv, "reason": "billing"}); r.status != http.StatusCreated {
		t.Fatalf("other user: %d", r.status)
	}
}

func TestChatReportIsInTheDataExportAndErasedWithTheAccount(t *testing.T) {
	f := newReportFix(t)
	e := f.e
	ctx := context.Background()
	rep := f.create(f.client, map[string]any{"conversation_id": f.conv, "reason": "advisor_unresponsive", "details": "Nobody answered me."})
	contact := e.newTicket(f.client, f.client.user.Email, "Plain contact", "A plain contact request for the export.")

	var job service.ExportView
	f.client.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/exports", map[string]any{"password": strongPassword}, &job)
	if err := e.app.Scheduler.RunJob(ctx, app.ExportJob); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/me/exports/" + job.ID.String()
	f.client.must(http.StatusOK, http.MethodGet, path, nil, &job)
	var link service.DownloadLink
	f.client.must(http.StatusOK, http.MethodPost, path+"/download-token", map[string]any{"password": strongPassword}, &link)
	r := f.client.raw(http.MethodGet, link.URL, nil, nil)
	zr, err := zip.NewReader(bytes.NewReader(r.body), int64(len(r.body)))
	if err != nil {
		t.Fatal(err)
	}
	var sup []byte
	for _, zf := range zr.File {
		if zf.Name == "support.json" {
			rc, _ := zf.Open()
			sup, _ = io.ReadAll(rc)
			rc.Close()
		}
	}
	for _, want := range []string{rep.Reference, `"kind": "chat_report"`, `"reason": "advisor_unresponsive"`, f.conv.String(), f.sessNew.ID.String(),
		"Nobody answered me.", contact.Reference, `"kind": "contact"`} {
		if !bytes.Contains(sup, []byte(want)) && !bytes.Contains(sup, []byte(strings.ReplaceAll(want, `": "`, `":"`))) {
			t.Errorf("support.json lacks %q:\n%s", want, sup)
		}
	}
	if bytes.Contains(sup, []byte(f.otherConv.String())) {
		t.Error("the export leaks another user's conversation")
	}

	// The database enforces the shape of a report.
	db := e.app.DB
	for name, stmt := range map[string]string{
		"report without a reason": "UPDATE support_tickets SET report_reason = NULL WHERE id = ?",
		"unknown reason":          "UPDATE support_tickets SET report_reason = 'spam' WHERE id = ?",
		"unknown kind":            "UPDATE support_tickets SET kind = 'complaint' WHERE id = ?",
		"negative seq":            "UPDATE support_tickets SET reported_seq = -1 WHERE id = ?",
		"contact with a reason":   "UPDATE support_tickets SET kind = 'contact' WHERE id = ?",
	} {
		if err := db.Exec(stmt, e.ticketID(rep.Reference)).Error; err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	// A second open report of the same conversation is refused by the unique index.
	if err := db.Exec(`INSERT INTO support_tickets (id, reference, user_id, name, email_enc, email_hash, topic, subject, message_enc, message_chars, locale,
		status, ip_hash, kind, conversation_id, report_reason, email_masked) VALUES (gen_random_uuid(), 'SY-ZZZZZZ', ?, '', 'x', ?, 'general', 'Chat report', 'x', 0, 'en',
		'open', 'x', 'chat_report', ?, 'other', 'x')`, f.client.user.ID, strings.Repeat("a", 64), f.conv).Error; err == nil {
		t.Error("the unique index must refuse a second open report")
	}

	// Erasure of the advisor keeps the report (links cleared); erasure of the reporter deletes it.
	if err := db.Exec("UPDATE support_tickets SET reported_user_id = NULL WHERE id = ?", e.ticketID(rep.Reference)).Error; err != nil {
		t.Fatal(err)
	}
	mine := e.ticketID(rep.Reference)
	f.client.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/close", map[string]any{"password": strongPassword, "confirmation": "DELETE"}, nil)
	e.clock.Advance(31 * 24 * time.Hour)
	if err := e.app.Scheduler.RunJob(ctx, app.ClosureJob); err != nil {
		t.Fatal(err)
	}
	if e.countTickets(mine) != 0 {
		t.Fatal("the erased account's report survived")
	}
}
