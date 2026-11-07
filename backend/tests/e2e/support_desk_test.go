package e2e

// revisions 3.0: support desk (docs/platform-extensions.md section 16).
//
// The bug these tests pin down: "users' messages from /en/contact are not
// landing in the admin page". A contact-form ticket must show up in the admin
// queue, be readable (after step-up), be answerable, and every rule around it
// (private link, visibility, states, retention) must hold end to end.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

const (
	adminTickets = "/api/v1/admin/support/tickets"
	adminSummary = "/api/v1/admin/support/summary"
	tokenInLink  = `#t=([A-Za-z0-9_-]{43})`
)

var tokenRE = regexp.MustCompile(tokenInLink)

// --- helpers -----------------------------------------------------------------

func (e *env) newTicket(c *client, email, subject, message string, extra ...map[string]any) service.TicketReceipt {
	e.t.Helper()
	body := map[string]any{"email": email, "topic": "general", "subject": subject, "message": message, "consent": true}
	for _, m := range extra {
		for k, v := range m {
			body[k] = v
		}
	}
	var rec service.TicketReceipt
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/support/tickets", body, &rec)
	return rec
}

func (e *env) ticketID(ref string) uuid.UUID {
	e.t.Helper()
	var row struct{ ID uuid.UUID }
	if err := e.app.DB.Table("support_tickets").Select("id").Where("reference = ?", ref).Scan(&row).Error; err != nil || row.ID == uuid.Nil {
		e.t.Fatalf("ticket %s: %v", ref, err)
	}
	return row.ID
}

func (e *env) ticketCols(ref string) map[string]any {
	e.t.Helper()
	out := map[string]any{}
	if err := e.app.DB.Table("support_tickets").Where("reference = ?", ref).Take(&out).Error; err != nil {
		e.t.Fatalf("ticket %s: %v", ref, err)
	}
	return out
}

// mailsTo returns the delivered e-mails to an address (the outbox is flushed first).
func (e *env) mailsTo(to string) []mail.Message {
	e.t.Helper()
	var out []mail.Message
	for _, m := range e.flushMail() {
		if strings.EqualFold(m.To, to) {
			out = append(out, m)
		}
	}
	return out
}

// tokenFrom finds the private-link token in the newest e-mail to `to` that mentions marker.
func (e *env) tokenFrom(to, marker string) string {
	e.t.Helper()
	msgs := e.mailsTo(to)
	for i := len(msgs) - 1; i >= 0; i-- {
		if strings.Contains(msgs[i].Subject+msgs[i].Text, marker) {
			if m := tokenRE.FindStringSubmatch(msgs[i].Text); m != nil {
				return m[1]
			}
		}
	}
	e.t.Fatalf("no e-mail with a private link to %s mentioning %q among %d", to, marker, len(msgs))
	return ""
}

func adminPage(t *testing.T, admin *client, query string) service.Page[service.AdminTicketRow] {
	t.Helper()
	var page service.Page[service.AdminTicketRow]
	admin.must(http.StatusOK, http.MethodGet, adminTickets+query, nil, &page)
	return page
}

func adminDetail(t *testing.T, admin *client, key string) service.AdminTicket {
	t.Helper()
	var d service.AdminTicket
	admin.must(http.StatusOK, http.MethodGet, adminTickets+"/"+key, nil, &d)
	return d
}

func adminReply(t *testing.T, admin *client, key, body, visibility string, status ...string) service.AdminTicket {
	t.Helper()
	payload := map[string]any{"body": body, "visibility": visibility}
	if len(status) > 0 {
		payload["status"] = status[0]
	}
	var d service.AdminTicket
	admin.must(http.StatusCreated, http.MethodPost, adminTickets+"/"+key+"/messages", payload, &d)
	return d
}

func adminPatch(admin *client, key string, payload map[string]any) response {
	return admin.call(http.MethodPatch, adminTickets+"/"+key, payload)
}

// linkReq calls the anonymous private-link endpoints (CSRF on POST, token header).
func (c *client) linkReq(method, ref, token string, body any) response {
	c.e.t.Helper()
	path := "/api/v1/support/tickets/" + ref
	headers := map[string]string{}
	if token != "" {
		headers["X-Support-Token"] = token
	}
	var rd *bytes.Reader
	if method == http.MethodPost {
		path += "/messages"
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
		c.ensureCSRF()
		headers["X-CSRF-Token"] = c.csrf
		headers["Content-Type"] = "application/json"
		return c.raw(method, path, rd, headers)
	}
	return c.raw(method, path, nil, headers)
}

func linkView(t *testing.T, c *client, ref, token string) service.CustomerTicket {
	t.Helper()
	r := c.linkReq(http.MethodGet, ref, token, nil)
	if r.status != http.StatusOK {
		t.Fatalf("link GET %s: %d %s", ref, r.status, r.body)
	}
	var v service.CustomerTicket
	r.decode(t, &v)
	return v
}

func (e *env) secondAdmin() *client { return e.createAdmin() }

func newDesk(t *testing.T, opts ...option) (*env, *client) {
	t.Helper()
	e := newEnv(t, opts...)
	return e, e.createAdmin()
}

func bodiesOf(v service.CustomerTicket) []string {
	var out []string
	for _, m := range v.Messages {
		out = append(out, m.Body)
	}
	return out
}

// --- the bug: a contact-form message lands in the admin queue ---------------------

func TestSupportDeskContactFormMessageLandsInTheAdminQueue(t *testing.T) {
	e, admin := newDesk(t)
	who := uniq("visitor") + "@Example.com"
	marker := uniq("zz")
	anon := e.client()
	rec := e.newTicket(anon, who, "Billing question "+marker, "I was charged twice for the same pack today.",
		map[string]any{"topic": "billing", "name": "Visitor Name"})

	page := adminPage(t, admin, "?q="+rec.Reference)
	if len(page.Items) != 1 || page.NextCursor != "" {
		t.Fatalf("the contact-form ticket must be in the admin queue: %+v", page)
	}
	row := page.Items[0]
	if row.Reference != rec.Reference || row.Status != "open" || row.Priority != "normal" || row.Topic != "billing" || row.Subject != "Billing question "+marker ||
		!row.NeedsReply || !row.Unread || row.MessageCount != 1 || row.Version != 1 || row.Locale != "en" || row.Assignee != nil ||
		row.Requester.Name != "Visitor Name" || row.Requester.UserID != nil || row.FirstResponseAt != nil || row.ResolvedAt != nil {
		t.Fatalf("row: %+v", row)
	}
	if row.Requester.EmailMasked != service.MaskEmail(strings.ToLower(who)) || !strings.Contains(row.Requester.EmailMasked, "•••") ||
		row.Requester.EmailMasked != "v•••@E•••.com" && row.Requester.EmailMasked != "v•••@e•••.com" {
		t.Fatalf("masked address: %q", row.Requester.EmailMasked)
	}

	// The queue is metadata only: nothing is decrypted.
	raw := admin.call(http.MethodGet, adminTickets+"?q="+rec.Reference, nil)
	for _, secret := range []string{strings.ToLower(who), "charged twice", "email_enc", "message_enc", "access_token"} {
		if strings.Contains(strings.ToLower(string(raw.body)), strings.ToLower(secret)) {
			t.Fatalf("the list leaks %q: %s", secret, raw.body)
		}
	}

	// Every way of finding it.
	for name, q := range map[string]string{
		"reference lower": strings.ToLower(rec.Reference), "email (any case)": strings.ToUpper(who), "subject word": marker,
		"subject words (case-insensitive)": "BILLING " + strings.ToUpper(marker),
	} {
		if p := adminPage(t, admin, "?q="+url.QueryEscape(q)); len(p.Items) != 1 || p.Items[0].Reference != rec.Reference {
			t.Errorf("q by %s: %+v", name, p.Items)
		}
	}
	for name, q := range map[string]string{"unknown reference": "SY-AAAAAA", "other address": "nobody-" + marker + "@example.com",
		"words that do not match": marker + " nonexistentword", "partial address": strings.Split(who, "@")[0]} {
		if p := adminPage(t, admin, "?q="+url.QueryEscape(q)); len(p.Items) != 0 {
			t.Errorf("q by %s matched %d rows", name, len(p.Items))
		}
	}
	// Filters combine.
	for query, want := range map[string]int{
		"?q=" + marker + "&status=open": 1, "?q=" + marker + "&status=pending,resolved": 0, "?q=" + marker + "&status=pending,open": 1,
		"?q=" + marker + "&topic=billing": 1, "?q=" + marker + "&topic=privacy": 0, "?q=" + marker + "&priority=normal": 1,
		"?q=" + marker + "&priority=urgent": 0, "?q=" + marker + "&assignee=unassigned": 1, "?q=" + marker + "&assignee=me": 0,
	} {
		if p := adminPage(t, admin, query); len(p.Items) != want {
			t.Errorf("%s: %d rows, want %d", query, len(p.Items), want)
		}
	}

	// The queue is for administrators only.
	for _, path := range []string{adminTickets, adminSummary, adminTickets + "/" + rec.Reference} {
		if r := e.adminClient().call(http.MethodGet, path, nil); r.status != http.StatusUnauthorized {
			t.Errorf("anonymous GET %s: %d", path, r.status)
		}
	}
	web := e.registerClient()
	adminFromWeb := e.adminClient()
	adminFromWeb.http.Jar = web.http.Jar
	if r := adminFromWeb.call(http.MethodGet, adminTickets, nil); r.status != http.StatusUnauthorized {
		t.Errorf("a main-site session must not open the queue: %d", r.status)
	}
	if r := web.call(http.MethodGet, adminTickets, nil); r.status != http.StatusForbidden {
		t.Errorf("main-site origin: %d", r.status)
	}
}

func TestSupportDeskDetailNeedsStepUpAndIsAudited(t *testing.T) {
	e, admin := newDesk(t)
	who := uniq("visitor") + "@example.com"
	msg := "My export request never arrived. Please check."
	rec := e.newTicket(e.client(), who, "Export missing", msg, map[string]any{"name": "Ana"})
	id := e.ticketID(rec.Reference)

	// A fresh step-up is required before anything is decrypted.
	e.clock.Advance(11 * time.Minute)
	r := admin.call(http.MethodGet, adminTickets+"/"+id.String(), nil)
	if r.status != http.StatusForbidden || r.errCode(t) != "step_up_required" {
		t.Fatalf("detail without step-up: %d %s", r.status, r.body)
	}
	if r := admin.call(http.MethodPost, adminTickets+"/"+id.String()+"/messages", map[string]any{"body": "x", "visibility": "public"}); r.errCode(t) != "step_up_required" {
		t.Fatalf("reply without step-up: %s", r.body)
	}
	if e.auditCount("support_ticket.viewed", id) != 0 {
		t.Fatal("a refused view must not be audited as a view")
	}
	// PATCH and the queue do not need it.
	if r := adminPatch(admin, id.String(), map[string]any{"priority": "high"}); r.status != http.StatusOK {
		t.Fatalf("patch without step-up: %d %s", r.status, r.body)
	}
	if p := adminPage(t, admin, "?q="+rec.Reference); len(p.Items) != 1 || !p.Items[0].Unread {
		t.Fatalf("queue: %+v", p.Items)
	}

	admin.stepUp()
	d := adminDetail(t, admin, id.String())
	if d.Requester.Email != who || d.Requester.Name != "Ana" || !strings.Contains(d.Requester.EmailMasked, "•••") || d.Status != "open" ||
		len(d.Messages) != 1 || d.Messages[0].Body != msg || d.Messages[0].Author.Type != "customer" || d.Messages[0].Visibility != "public" ||
		d.User != nil || d.Unread || d.Priority != "high" {
		t.Fatalf("detail: %+v", d)
	}
	// The address and the message are sealed at rest.
	cols := e.ticketCols(rec.Reference)
	if strings.Contains(fmt.Sprint(cols["email_enc"], cols["message_enc"]), "export request") || strings.Contains(strings.ToLower(fmt.Sprint(cols["email_enc"])), "visitor") {
		t.Fatal("sealed columns hold clear text")
	}
	if cols["staff_seen_at"] == nil {
		t.Fatal("viewing sets staff_seen_at")
	}
	if p := adminPage(t, admin, "?q="+rec.Reference); p.Items[0].Unread {
		t.Fatal("the ticket is read now")
	}

	// Audited once per admin and ticket within 10 minutes; the key may be the reference.
	if got := e.auditCount("support_ticket.viewed", id); got != 1 {
		t.Fatalf("viewed audit rows: %d", got)
	}
	adminDetail(t, admin, strings.ToLower(rec.Reference))
	adminDetail(t, admin, id.String())
	if got := e.auditCount("support_ticket.viewed", id); got != 1 {
		t.Fatalf("view audit must be throttled: %d rows", got)
	}
	var audit domain.AuditLog
	e.app.DB.Where("action = ? AND target_id = ?", "support_ticket.viewed", id.String()).First(&audit)
	if audit.ActorID == nil || *audit.ActorID != admin.adminID || audit.TargetType != "support_ticket" ||
		strings.Contains(fmt.Sprint(audit.Metadata), "export request") || strings.Contains(fmt.Sprint(audit.Metadata), who) {
		t.Fatalf("audit row: %+v", audit)
	}
	// Another administrator is audited separately.
	other := e.secondAdmin()
	other.stepUp()
	adminDetail(t, other, id.String())
	if got := e.auditCount("support_ticket.viewed", id); got != 2 {
		t.Fatalf("second admin: %d rows", got)
	}

	// Unknown and malformed keys: 404 (after step-up).
	admin.stepUp()
	for _, key := range []string{uuid.NewString(), "SY-AAAAAA", "not-a-ticket", "SY-1", "%27%3B--"} {
		if r := admin.call(http.MethodGet, adminTickets+"/"+key, nil); r.status != http.StatusNotFound || r.errCode(t) != "not_found" {
			t.Errorf("GET %s: %d %s", key, r.status, r.body)
		}
	}
}

func TestSupportDeskReplyEmailsTheRequesterWithTheLinkAndTheTokenWorks(t *testing.T) {
	e, admin := newDesk(t)
	who := uniq("visitor") + "@example.com"
	anon := e.client()
	anon.locale = "fr"
	rec := e.newTicket(anon, who, "Question sur ma facture", "Pouvez-vous m'expliquer le montant débité ?")
	id := e.ticketID(rec.Reference)

	// The acknowledgement carries the private link (token in the fragment) and never the text.
	ack := e.mailsTo(who)
	if len(ack) != 1 || strings.Contains(ack[0].Text, "facture") || strings.Contains(ack[0].Text, "débité") {
		t.Fatalf("acknowledgement: %+v", ack)
	}
	token := e.tokenFrom(who, rec.Reference)
	wantLink := e.app.Config.App.FrontendURL + "/fr/support/" + rec.Reference + "#t=" + token
	if !strings.Contains(ack[0].Text, wantLink) {
		t.Fatalf("link missing from the acknowledgement: want %s in %q", wantLink, ack[0].Text)
	}
	// Only a keyed hash (and a sealed copy) is stored, never the token itself.
	cols := e.ticketCols(rec.Reference)
	hash, _ := cols["access_token_hash"].(string)
	if len(hash) != 64 || hash == token || strings.Contains(fmt.Sprint(cols["access_token_enc"]), token) {
		t.Fatalf("token storage: hash=%q", hash)
	}
	if exp, _ := cols["access_token_expires_at"].(time.Time); !exp.Equal(e.clock.Now().Add(720 * time.Hour)) {
		t.Fatalf("link validity: %v", cols["access_token_expires_at"])
	}

	// A public reply is e-mailed in the ticket's language, with the reply text and the same link.
	admin.stepUp()
	d := adminReply(t, admin, id.String(), "Bonjour, le montant correspond au pack de 600 crédits.\n\nBien cordialement.", "public")
	if d.Status != "pending" || d.MessageCount != 2 || d.FirstResponseAt == nil || d.Version != 2 || len(d.Messages) != 2 ||
		d.Messages[1].Author.Type != "staff" || d.Messages[1].Visibility != "public" || d.Messages[1].EmailStatus != "queued" {
		t.Fatalf("after reply: %+v", d)
	}
	mails := e.mailsTo(who)
	if len(mails) != 2 {
		t.Fatalf("want acknowledgement + reply, got %d", len(mails))
	}
	reply := mails[1]
	if !strings.Contains(reply.Subject, rec.Reference) || !strings.Contains(reply.Text, "le montant correspond au pack de 600") ||
		!strings.Contains(reply.Text, "Bien cordialement.") || !strings.Contains(reply.Text, wantLink) ||
		!strings.Contains(reply.Text, "Notre équipe") || strings.Contains(reply.Subject+reply.Text, "Question sur ma facture") {
		t.Fatalf("reply e-mail: %q / %q", reply.Subject, reply.Text)
	}
	if e.auditCount("support_ticket.replied", id) != 1 || strings.Contains(fmt.Sprint(auditMeta(e, "support_ticket.replied", id)), "montant") {
		t.Fatal("reply must be audited without its text")
	}

	// The anonymous page works with the token header.
	v := linkView(t, e.client(), rec.Reference, token)
	if v.Reference != rec.Reference || v.Status != "pending" || v.Awaiting != "you" || !v.CanReply || len(v.Messages) != 2 ||
		v.Messages[0].Author.Type != "customer" || v.Messages[1].Author.Type != "staff" ||
		v.Messages[1].Author.Name != "Staryield Support (Admin)" || !strings.Contains(v.Messages[1].Body, "pack de 600") {
		t.Fatalf("customer view: %+v", v)
	}
	if lower := linkView(t, e.client(), strings.ToLower(rec.Reference), token); lower.Reference != rec.Reference {
		t.Fatal("reference is case-insensitive")
	}

	// Every failure is the same 404, whatever the reason.
	otherWho := uniq("other") + "@example.com"
	other := e.newTicket(e.client(), otherWho, "Another request", "Some other request text here.")
	otherToken := e.tokenFrom(otherWho, other.Reference)
	var first response
	for name, tc := range map[string]struct{ ref, token string }{
		"no token": {rec.Reference, ""}, "wrong token": {rec.Reference, strings.Repeat("A", 43)}, "near miss": {rec.Reference, token[:42] + "x"},
		"another ticket's token": {rec.Reference, otherToken}, "unknown reference": {"SY-AAAAAA", token}, "malformed reference": {"nope", token},
		"short token": {rec.Reference, "abc"}, "long token": {rec.Reference, token + token}, "other ticket, my token": {other.Reference, token},
	} {
		r := e.client().linkReq(http.MethodGet, tc.ref, tc.token, nil)
		if r.status != http.StatusNotFound || r.errCode(t) != "not_found" {
			t.Errorf("%s: %d %s", name, r.status, r.body)
		}
		if first.body == nil {
			first = r
		} else if got, want := decodeErr(t, r), decodeErr(t, first); got != want {
			t.Errorf("%s answers differently: %q vs %q", name, got, want)
		}
	}
	// POST with a wrong token is the same 404 and stores nothing.
	if r := e.client().linkReq(http.MethodPost, rec.Reference, strings.Repeat("A", 43), map[string]any{"body": "hello there"}); r.status != http.StatusNotFound {
		t.Fatalf("reply with a wrong token: %d %s", r.status, r.body)
	}
	if got := e.ticketCols(rec.Reference)["message_count"]; fmt.Sprint(got) != "2" {
		t.Fatalf("message stored by a wrong token: %v", got)
	}
	// The token travels in a header only: a query-string token is ignored.
	if r := e.client().raw(http.MethodGet, "/api/v1/support/tickets/"+rec.Reference+"?token="+token, nil, nil); r.status != http.StatusNotFound {
		t.Fatalf("token in the query string: %d", r.status)
	}
	if r := e.client().raw(http.MethodGet, "/api/v1/support/tickets/"+rec.Reference, nil, map[string]string{"Authorization": "Bearer " + token}); r.status != http.StatusNotFound {
		t.Fatalf("token as bearer: %d", r.status)
	}
}

func decodeErr(t *testing.T, r response) string {
	t.Helper()
	var out struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	r.decode(t, &out)
	return fmt.Sprintf("%d/%s/%s", r.status, out.Error.Code, out.Error.Message)
}

func TestSupportDeskLinkExpiresAndIsExtendedNotRotated(t *testing.T) {
	e, admin := newDesk(t)

	// 30 days after the e-mail the link is dead.
	deadWho := uniq("d") + "@example.com"
	dead := e.newTicket(e.client(), deadWho, "Dead link", "This link will expire after thirty days.")
	deadToken := e.tokenFrom(deadWho, dead.Reference)
	e.clock.Advance(720*time.Hour - time.Minute)
	if r := e.client().linkReq(http.MethodGet, dead.Reference, deadToken, nil); r.status != http.StatusOK {
		t.Fatalf("one minute before expiry: %d", r.status)
	}
	e.clock.Advance(2 * time.Minute)
	if r := e.client().linkReq(http.MethodGet, dead.Reference, deadToken, nil); r.status != http.StatusNotFound || r.errCode(t) != "not_found" {
		t.Fatalf("expired link: %d %s", r.status, r.body)
	}
	if r := e.client().linkReq(http.MethodPost, dead.Reference, deadToken, map[string]any{"body": "too late to reply"}); r.status != http.StatusNotFound {
		t.Fatalf("reply through an expired link: %d", r.status)
	}

	// A staff reply e-mails the link again and extends it without changing it.
	who := uniq("visitor") + "@example.com"
	rec := e.newTicket(e.client(), who, "Extended link", "A reply will extend this link.")
	id := e.ticketID(rec.Reference)
	token := e.tokenFrom(who, rec.Reference)
	e.clock.Advance(20 * 24 * time.Hour)
	admin = e.createAdmin() // the console session did not survive 20 days
	admin.stepUp()
	adminReply(t, admin, id.String(), "First answer from the team.", "public")
	if got := e.tokenFrom(who, "First answer"); got != token {
		t.Fatalf("the link must be extended, not rotated: %s vs %s", got, token)
	}
	exp, _ := e.ticketCols(rec.Reference)["access_token_expires_at"].(time.Time)
	if want := e.clock.Now().Add(720 * time.Hour); exp.Before(want.Add(-time.Minute)) || exp.After(want.Add(time.Minute)) {
		t.Fatalf("validity after the reply: %v, want about %v", exp, want)
	}
	// 40 days after the first e-mail the link still works thanks to the extension...
	e.clock.Advance(20 * 24 * time.Hour)
	if r := e.client().linkReq(http.MethodGet, rec.Reference, token, nil); r.status != http.StatusOK {
		t.Fatalf("extended link: %d %s", r.status, r.body)
	}
	// ...and expires 30 days after the last e-mail.
	e.clock.Advance(11 * 24 * time.Hour)
	if r := e.client().linkReq(http.MethodGet, rec.Reference, token, nil); r.status != http.StatusNotFound {
		t.Fatalf("link after the extended validity: %d", r.status)
	}
	// A reply after expiry issues a fresh link (the old token stays dead).
	admin = e.createAdmin()
	admin.stepUp()
	adminReply(t, admin, id.String(), "Second answer after expiry.", "public")
	fresh := e.tokenFrom(who, "Second answer")
	if fresh == token {
		t.Fatal("an expired token must not be revived")
	}
	if r := e.client().linkReq(http.MethodGet, rec.Reference, token, nil); r.status != http.StatusNotFound {
		t.Fatalf("old token after a new one was issued: %d", r.status)
	}
	if r := e.client().linkReq(http.MethodGet, rec.Reference, fresh, nil); r.status != http.StatusOK {
		t.Fatalf("new token: %d", r.status)
	}
}

func auditMeta(e *env, action string, id uuid.UUID) any {
	var a domain.AuditLog
	e.app.DB.Where("action = ? AND target_id = ?", action, id.String()).Order("created_at DESC").First(&a)
	return a.Metadata
}
