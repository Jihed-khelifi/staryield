package e2e

// revisions 3.0: support desk, operations: summary and its cache, rate limits,
// maintenance (auto-close and retention with a moved clock), personal-data
// export and erasure, database guards, localization and pagination.

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/app"
	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

func summaryOf(t *testing.T, admin *client) service.SupportSummary {
	t.Helper()
	var s service.SupportSummary
	admin.must(http.StatusOK, http.MethodGet, adminSummary, nil, &s)
	return s
}

func TestSupportDeskSummaryCountsAndCache(t *testing.T) {
	e, admin := newDesk(t)
	base := summaryOf(t, admin)

	var refs []string
	for i := 0; i < 4; i++ {
		refs = append(refs, e.newTicket(e.client(), uniq("s")+"@example.com", fmt.Sprintf("Summary %d", i), "Counting tickets for the summary.").Reference)
	}
	got := summaryOf(t, admin)
	if got.Open != base.Open+4 || got.Unassigned != base.Unassigned+4 || got.NeedsReply != base.NeedsReply+4 || got.Mine != base.Mine ||
		got.Pending != base.Pending || got.Resolved != base.Resolved || got.Closed != base.Closed {
		t.Fatalf("after 4 tickets: %+v → %+v", base, got)
	}
	// Writes invalidate the cache straight away: assign, resolve, close.
	if r := adminPatch(admin, refs[0], map[string]any{"assignee_id": admin.adminID.String(), "status": "pending"}); r.status != http.StatusOK {
		t.Fatal(r.body)
	}
	adminPatch(admin, refs[1], map[string]any{"status": "resolved"})
	adminPatch(admin, refs[2], map[string]any{"status": "closed", "assignee_id": admin.adminID.String()})
	got = summaryOf(t, admin)
	want := service.SupportSummary{Open: base.Open + 1, Pending: base.Pending + 1, Resolved: base.Resolved + 1, Closed: base.Closed + 1,
		Unassigned: base.Unassigned + 1, Mine: base.Mine + 1, NeedsReply: base.NeedsReply + 1}
	if got != want {
		t.Fatalf("after updates: got %+v, want %+v", got, want)
	}
	// "mine" is per administrator.
	other := e.secondAdmin()
	if o := summaryOf(t, other); o.Mine != 0 || o.Open != got.Open {
		t.Fatalf("other admin: %+v", o)
	}
	// A requester reply and a staff reply both invalidate.
	token := e.tokenFrom(e.ticketEmailFromMail(refs[0]), refs[0])
	e.client().linkReq(http.MethodPost, refs[0], token, map[string]any{"body": "Customer reopens the first one."})
	if s := summaryOf(t, admin); s.Open != got.Open+1 || s.Pending != got.Pending-1 {
		t.Fatalf("after the requester reply: %+v", s)
	}

	// Cached for 15 s: a row inserted behind the application's back is not visible yet...
	cached := summaryOf(t, admin)
	e.insertTicketRow("open", "normal")
	if s := summaryOf(t, admin); s != cached {
		t.Fatalf("summary must be cached: %+v vs %+v", s, cached)
	}
	// ...until the next write through the API.
	adminPatch(admin, refs[3], map[string]any{"priority": "low"})
	if s := summaryOf(t, admin); s.Open != cached.Open+1 {
		t.Fatalf("a write must invalidate the cache: %+v vs %+v", s, cached)
	}
	if r := e.adminClient().call(http.MethodGet, adminSummary, nil); r.status != http.StatusUnauthorized {
		t.Fatalf("anonymous summary: %d", r.status)
	}
}

// ticketEmailFromMail returns the address a ticket's acknowledgement went to.
func (e *env) ticketEmailFromMail(ref string) string {
	e.t.Helper()
	for _, m := range e.flushMail() {
		if strings.Contains(m.Subject, ref) && m.To != e.app.Config.Mail.SupportInbox {
			return m.To
		}
	}
	e.t.Fatalf("no acknowledgement for %s", ref)
	return ""
}

// insertTicketRow adds a ticket directly (as a legacy writer would).
func (e *env) insertTicketRow(status, priority string) uuid.UUID {
	e.t.Helper()
	id := uuid.New()
	ref := "SY-QQ" + strings.ToUpper(strings.NewReplacer("-", "", "0", "2", "1", "3", "I", "J", "O", "P").Replace(id.String()))[0:4]
	now := e.clock.Now()
	closed := any(nil)
	if status == "closed" {
		closed = now
	}
	err := e.app.DB.Exec(`INSERT INTO support_tickets (id, reference, name, email_enc, email_hash, topic, subject, message_enc, message_chars,
		locale, status, ip_hash, created_at, updated_at, priority, version, message_count, last_message_at, last_customer_message_at,
		email_masked, closed_at) VALUES (?, ?, '', 'x', ?, 'general', 'Direct insert', 'x', 20, 'en', ?, '', ?, ?, ?, 1, 1, ?, ?, 'd•••@e•••.com', ?)`,
		id, ref, strings.Repeat("a", 64), status, now, now, priority, now, now, closed).Error
	if err != nil {
		e.t.Fatalf("insert ticket: %v", err)
	}
	return id
}

func TestSupportDeskRateLimits(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.RateLimit.Scale = 1 })
	// support_reply: 20 per hour per user.
	c := e.registerClient()
	rec := e.newTicket(c, c.user.Email, "Rate limits", "Replying many times to one ticket.")
	path := "/api/v1/me/support/tickets/" + rec.Reference + "/messages"
	for i := 1; i <= 20; i++ {
		if r := c.call(http.MethodPost, path, map[string]any{"body": fmt.Sprintf("Reply number %d", i)}); r.status != http.StatusCreated {
			t.Fatalf("reply %d: %d %s", i, r.status, r.body)
		}
	}
	r := c.call(http.MethodPost, path, map[string]any{"body": "Reply number 21"})
	if r.status != http.StatusTooManyRequests || r.errCode(t) != "rate_limited" || r.header.Get("Retry-After") == "" {
		t.Fatalf("the 21st reply must be throttled: %d %s", r.status, r.body)
	}
	if got := e.ticketCols(rec.Reference)["message_count"]; got.(int32) != 21 {
		t.Fatalf("throttled reply stored: %v", got)
	}
	// Another user has their own budget.
	d := e.registerClient()
	rec2 := e.newTicket(d, d.user.Email, "Other budget", "Another user, another bucket here.")
	if r := d.call(http.MethodPost, "/api/v1/me/support/tickets/"+rec2.Reference+"/messages", map[string]any{"body": "Still allowed"}); r.status != http.StatusCreated {
		t.Fatalf("other user: %d", r.status)
	}
	// Ticket creation keeps its own limit (5 per hour per user).
	for i := 0; i < 5; i++ {
		last := c.call(http.MethodPost, "/api/v1/support/tickets", map[string]any{"email": c.user.Email, "subject": "Creation limit", "message": "Another request with enough text.", "consent": true})
		if i == 4 && last.status != http.StatusTooManyRequests {
			t.Fatalf("creation limit: %d", last.status)
		}
	}

	// support_link: 30 per minute per IP, whatever the token.
	anon := e.client()
	var last response
	for i := 0; i < 31; i++ {
		last = anon.linkReq(http.MethodGet, rec.Reference, strings.Repeat("B", 43), nil)
		if i < 30 && last.status != http.StatusNotFound {
			t.Fatalf("lookup %d: %d", i+1, last.status)
		}
	}
	if last.status != http.StatusTooManyRequests || last.errCode(t) != "rate_limited" || last.header.Get("Retry-After") == "" {
		t.Fatalf("the 31st lookup must be throttled: %d %s", last.status, last.body)
	}
	// The throttle also covers anonymous replies.
	if r := anon.linkReq(http.MethodPost, rec.Reference, strings.Repeat("B", 43), map[string]any{"body": "Guessing tokens here"}); r.status != http.StatusTooManyRequests {
		t.Fatalf("anonymous reply after the throttle: %d", r.status)
	}
}

func TestSupportDeskMaintenanceAutoClosesAndAppliesRetention(t *testing.T) {
	e, admin := newDesk(t)
	ctx := context.Background()
	run := func() { t.Helper(); mustRunJob(t, e) }

	who := uniq("visitor") + "@example.com"
	solved := e.newTicket(e.client(), who, "Will be auto-closed", "A request that gets resolved and then forgotten.")
	token := e.tokenFrom(who, solved.Reference)
	reopened := e.newTicket(e.client(), uniq("r")+"@example.com", "Will be reopened", "A request resolved but then answered.")
	reopenedToken := e.tokenFrom(e.ticketEmailFromMail(reopened.Reference), reopened.Reference)
	pending := e.newTicket(e.client(), uniq("p")+"@example.com", "Stays pending", "A pending request is never auto-closed.")
	admin.stepUp()
	adminReply(t, admin, solved.Reference, "Solved, please tell us if not.", "public", "resolved")
	adminReply(t, admin, reopened.Reference, "Solved as well.", "public", "resolved")
	adminReply(t, admin, pending.Reference, "Waiting for you.", "public")
	cols := e.ticketCols(solved.Reference)
	if cols["status"] != "resolved" || cols["resolved_at"] == nil {
		t.Fatalf("resolved: %+v", cols)
	}
	if r := e.client().linkReq(http.MethodPost, reopened.Reference, reopenedToken, map[string]any{"body": "Not solved at all!"}); r.status != http.StatusCreated {
		t.Fatalf("reopen: %d", r.status)
	}

	// 7 days minus a minute: nothing happens. (The job runs on the fake clock.)
	e.clock.Advance(7*24*time.Hour - time.Minute)
	run()
	if e.ticketCols(solved.Reference)["status"] != "resolved" {
		t.Fatal("closed too early")
	}
	e.clock.Advance(2 * time.Minute)
	run()
	cols = e.ticketCols(solved.Reference)
	if cols["status"] != "closed" || cols["closed_at"] == nil || cols["message_count"].(int32) != 3 || cols["version"].(int32) < 3 {
		t.Fatalf("not auto-closed: %+v", cols)
	}
	if e.ticketCols(reopened.Reference)["status"] != "open" || e.ticketCols(pending.Reference)["status"] != "pending" {
		t.Fatal("only resolved tickets may auto-close")
	}
	// A system entry says why: visible to admins only.
	var sys struct{ N int }
	e.app.DB.Raw(`SELECT count(*) AS n FROM support_messages WHERE ticket_id = ? AND author_type = 'system' AND visibility = 'internal'`, e.ticketID(solved.Reference)).Scan(&sys)
	if sys.N != 1 {
		t.Fatalf("system entries: %d", sys.N)
	}
	// Closed means closed: requests are refused and the link still reads.
	r := e.client().linkReq(http.MethodPost, solved.Reference, token, map[string]any{"body": "I want to add something."})
	if r.status != http.StatusConflict || r.errCode(t) != "invalid_state" {
		t.Fatalf("reply after auto-close: %d %s", r.status, r.body)
	}
	if v := linkView(t, e.client(), solved.Reference, token); v.Status != "closed" || v.CanReply || len(v.Messages) != 2 {
		t.Fatalf("view: %+v", v)
	}
	// The job is idempotent.
	run()
	if e.ticketCols(solved.Reference)["message_count"].(int32) != 3 {
		t.Fatal("second run closed again")
	}
	admin2 := e.createAdmin()
	admin2.stepUp()
	if d := adminDetail(t, admin2, solved.Reference); len(d.Messages) != 3 || d.Messages[2].Author.Type != "system" || d.Messages[2].Visibility != "internal" ||
		!strings.Contains(d.Messages[2].Body, "Closed automatically") {
		t.Fatalf("admin thread: %+v", d.Messages)
	}

	// Retention: closed tickets and their messages are deleted 24 months after closing.
	closedID := e.ticketID(solved.Reference)
	e.clock.Advance(730*24*time.Hour - time.Hour)
	run()
	if n := e.countTickets(closedID); n != 1 {
		t.Fatal("deleted before the retention period ended")
	}
	e.clock.Advance(2 * time.Hour)
	run()
	if n := e.countTickets(closedID); n != 0 {
		t.Fatal("closed ticket kept past the retention period")
	}
	var msgs struct{ N int }
	e.app.DB.Raw(`SELECT count(*) AS n FROM support_messages WHERE ticket_id = ?`, closedID).Scan(&msgs)
	if msgs.N != 0 {
		t.Fatalf("messages survive their ticket: %d", msgs.N)
	}
	if r := e.client().linkReq(http.MethodGet, solved.Reference, token, nil); r.status != http.StatusNotFound {
		t.Fatalf("deleted ticket: %d", r.status)
	}
	// Open and pending tickets are never deleted by retention.
	if e.countTickets(e.ticketID(pending.Reference)) != 1 || e.countTickets(e.ticketID(reopened.Reference)) != 1 {
		t.Fatal("retention deleted a live ticket")
	}
	_ = ctx
}

func mustRunJob(t *testing.T, e *env) {
	t.Helper()
	if err := e.app.Scheduler.RunJob(context.Background(), service.SupportJob); err != nil {
		t.Fatalf("support-maintenance: %v", err)
	}
}

func (e *env) countTickets(id uuid.UUID) int64 {
	var n int64
	e.app.DB.Table("support_tickets").Where("id = ?", id).Count(&n)
	return n
}

func TestSupportDeskMaintenanceFillsMaskedAddressesOfOlderTickets(t *testing.T) {
	e, admin := newDesk(t)
	who := "Legacy.Person@Example.org"
	rec := e.newTicket(e.client(), who, "Created before the desk", "A request that has no masked address yet.")
	id := e.ticketID(rec.Reference)
	e.app.DB.Table("support_tickets").Where("id = ?", id).Update("email_masked", "")
	if p := adminPage(t, admin, "?q="+rec.Reference); p.Items[0].Requester.EmailMasked != "•••" {
		t.Fatalf("a missing mask must never show the address: %q", p.Items[0].Requester.EmailMasked)
	}
	mustRunJob(t, e)
	want := service.MaskEmail("legacy.person@example.org")
	if p := adminPage(t, admin, "?q="+rec.Reference); p.Items[0].Requester.EmailMasked != want || want != "l•••@e•••.org" {
		t.Fatalf("backfilled mask: %q want %q", p.Items[0].Requester.EmailMasked, want)
	}
}

func TestSupportDeskTicketsAreInTheExportAndErasedWithTheAccount(t *testing.T) {
	e, admin := newDesk(t)
	ctx := context.Background()
	owner := e.registerClient()
	other := e.registerClient()
	mine := e.newTicket(owner, owner.user.Email, "My export subject", "My original request text for the export.")
	theirs := e.newTicket(other, other.user.Email, "Someone else", "Another account's request text here.")
	admin.stepUp()
	adminReply(t, admin, mine.Reference, "PUBLIC-REPLY for the export.", "public")
	adminReply(t, admin, mine.Reference, "INTERNAL-NOTE must not be exported.", "internal")
	owner.must(http.StatusCreated, http.MethodPost, "/api/v1/me/support/tickets/"+mine.Reference+"/messages", map[string]any{"body": "My own follow-up in the export."}, nil)

	// Export.
	var job service.ExportView
	owner.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/exports", map[string]any{"password": strongPassword}, &job)
	if err := e.app.Scheduler.RunJob(ctx, app.ExportJob); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/me/exports/" + job.ID.String()
	owner.must(http.StatusOK, http.MethodGet, path, nil, &job)
	if job.Status != "ready" {
		t.Fatalf("export: %+v", job)
	}
	var link service.DownloadLink
	owner.must(http.StatusOK, http.MethodPost, path+"/download-token", map[string]any{"password": strongPassword}, &link)
	r := owner.raw(http.MethodGet, link.URL, nil, nil)
	zr, err := zip.NewReader(bytes.NewReader(r.body), int64(len(r.body)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		files[f.Name], _ = io.ReadAll(rc)
		rc.Close()
	}
	sup, ok := files["support.json"]
	if !ok {
		t.Fatalf("the export lacks support.json: %v", supportKeysOf(files))
	}
	for _, want := range []string{mine.Reference, "My export subject", "My original request text for the export.", "PUBLIC-REPLY for the export.", "My own follow-up in the export.", owner.user.Email} {
		if !bytes.Contains(sup, []byte(want)) {
			t.Errorf("support.json lacks %q", want)
		}
	}
	for _, secret := range []string{"INTERNAL-NOTE", theirs.Reference, "Another account", "system", admin.user.Email, "access_token"} {
		if bytes.Contains(sup, []byte(secret)) {
			t.Errorf("support.json leaks %q", secret)
		}
	}
	for name, data := range files {
		if bytes.Contains(data, []byte("INTERNAL-NOTE")) {
			t.Errorf("%s leaks an internal note", name)
		}
	}

	// Erasure deletes the account's tickets and their messages, and only those.
	mineID, theirsID := e.ticketID(mine.Reference), e.ticketID(theirs.Reference)
	owner.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/close", map[string]any{"password": strongPassword, "confirmation": "DELETE"}, nil)
	e.clock.Advance(31 * 24 * time.Hour)
	if err := e.app.Scheduler.RunJob(ctx, app.ClosureJob); err != nil {
		t.Fatal(err)
	}
	if e.countTickets(mineID) != 0 {
		t.Fatal("the erased account's ticket survived")
	}
	var msgs struct{ N int }
	e.app.DB.Raw(`SELECT count(*) AS n FROM support_messages WHERE ticket_id = ?`, mineID).Scan(&msgs)
	if msgs.N != 0 {
		t.Fatalf("messages of erased tickets survived: %d", msgs.N)
	}
	if e.countTickets(theirsID) != 1 {
		t.Fatal("another account's ticket must stay")
	}
}

func supportKeysOf(m map[string][]byte) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestSupportDeskDatabaseGuards(t *testing.T) {
	e, admin := newDesk(t)
	rec := e.newTicket(e.client(), uniq("g")+"@example.com", "Database guards", "Checking constraints and triggers.")
	id := e.ticketID(rec.Reference)
	admin.stepUp()
	d := adminReply(t, admin, id.String(), "A staff reply.", "public")
	msgID := d.Messages[1].ID
	db := e.app.DB

	// Bodies are immutable; only the e-mail status moves.
	for name, stmt := range map[string]string{
		"body": "UPDATE support_messages SET body_enc = 'tampered' WHERE id = ?", "author type": "UPDATE support_messages SET author_type = 'customer', author_id = NULL WHERE id = ?",
		"visibility": "UPDATE support_messages SET visibility = 'internal' WHERE id = ?", "ticket": "UPDATE support_messages SET ticket_id = gen_random_uuid() WHERE id = ?",
		"created_at": "UPDATE support_messages SET created_at = now() - interval '1 day' WHERE id = ?", "body chars": "UPDATE support_messages SET body_chars = 10 WHERE id = ?",
		"author": "UPDATE support_messages SET author_id = (SELECT id FROM users LIMIT 1) WHERE id = ?",
	} {
		if err := db.Exec(stmt, msgID).Error; err == nil {
			t.Errorf("updating %s must be refused", name)
		}
	}
	if err := db.Exec("UPDATE support_messages SET email_status = 'sent' WHERE id = ?", msgID).Error; err != nil {
		t.Fatalf("email_status must stay updatable: %v", err)
	}
	if err := db.Exec("UPDATE support_messages SET email_status = 'bounced' WHERE id = ?", msgID).Error; err == nil {
		t.Error("unknown email status accepted")
	}
	// CHECK constraints.
	ins := func(author, vis, email string) error {
		return db.Exec(`INSERT INTO support_messages (id, ticket_id, author_type, visibility, body_enc, body_chars, email_status) VALUES (?, ?, ?, ?, 'x', 1, ?)`,
			uuid.New(), id, author, vis, email).Error
	}
	for name, args := range map[string][3]string{"customer internal": {"customer", "internal", "none"}, "system public": {"system", "public", "none"},
		"unknown author": {"robot", "public", "none"}, "unknown visibility": {"staff", "secret", "none"}, "email on a note": {"staff", "internal", "queued"},
		"email on a customer message": {"customer", "public", "sent"}} {
		if err := ins(args[0], args[1], args[2]); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if err := ins("staff", "internal", "none"); err != nil {
		t.Errorf("a valid note refused: %v", err)
	}
	if err := db.Exec(`INSERT INTO support_messages (id, ticket_id, author_type, visibility, body_enc, body_chars) VALUES (?, ?, 'customer', 'public', 'x', 5001)`, uuid.New(), id).Error; err == nil {
		t.Error("body_chars above 5000 accepted")
	}
	for name, stmt := range map[string]string{
		"status": "UPDATE support_tickets SET status = 'done' WHERE id = ?", "priority": "UPDATE support_tickets SET priority = 'critical' WHERE id = ?",
		"closed without time": "UPDATE support_tickets SET status = 'closed', closed_at = NULL WHERE id = ?", "version": "UPDATE support_tickets SET version = 0 WHERE id = ?",
		"token pair":        "UPDATE support_tickets SET access_token_hash = repeat('a', 64), access_token_expires_at = NULL WHERE id = ?",
		"token hash length": "UPDATE support_tickets SET access_token_hash = 'short', access_token_expires_at = now() WHERE id = ?",
		"message count":     "UPDATE support_tickets SET message_count = 0 WHERE id = ?",
	} {
		if err := db.Exec(stmt, id).Error; err == nil {
			t.Errorf("ticket %s accepted", name)
		}
	}
	// Existing statuses stay valid and the new ones are accepted.
	for _, st := range []string{"pending", "resolved", "open"} {
		if err := db.Exec("UPDATE support_tickets SET status = ? WHERE id = ?", st, id).Error; err != nil {
			t.Errorf("status %s: %v", st, err)
		}
	}
	// Deleting a ticket removes its messages.
	if err := db.Exec("DELETE FROM support_tickets WHERE id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	var left struct{ N int }
	db.Raw("SELECT count(*) AS n FROM support_messages WHERE ticket_id = ?", id).Scan(&left)
	if left.N != 0 {
		t.Fatalf("orphan messages: %d", left.N)
	}
}

func TestSupportDeskMessagesAreLocalized(t *testing.T) {
	e, admin := newDesk(t)
	who := uniq("loc") + "@example.com"
	rec := e.newTicket(e.client(), who, "Localized errors", "Looking at translated messages here.")
	id := e.ticketID(rec.Reference)
	token := e.tokenFrom(who, rec.Reference)

	admin.locale = "fr"
	r := admin.call(http.MethodGet, adminTickets+"?status=nope&sort=random", nil)
	if r.status != http.StatusUnprocessableEntity || !strings.Contains(string(r.body), "Choisissez ouverte, en attente, résolue ou fermée.") ||
		!strings.Contains(string(r.body), "Choisissez les plus récentes") {
		t.Fatalf("fr list errors: %s", r.body)
	}
	admin.stepUp()
	r = admin.call(http.MethodPost, adminTickets+"/"+id.String()+"/messages", map[string]any{"body": "", "visibility": "secret"})
	if !strings.Contains(string(r.body), "Écrivez un message de 1 à 5 000 caractères.") || !strings.Contains(string(r.body), "Choisissez public ou interne.") {
		t.Fatalf("fr reply errors: %s", r.body)
	}
	r = adminPatch(admin, id.String(), map[string]any{"assignee_id": uuid.NewString(), "expected_version": 99})
	if r.status != http.StatusUnprocessableEntity && !strings.Contains(string(r.body), "modifiée par quelqu'un d'autre") {
		t.Fatalf("fr: %d %s", r.status, r.body)
	}
	if r = adminPatch(admin, id.String(), map[string]any{"priority": "low", "expected_version": 99}); r.status != http.StatusConflict ||
		!strings.Contains(string(r.body), "Cette demande a été modifiée par quelqu'un d'autre.") {
		t.Fatalf("fr conflict: %d %s", r.status, r.body)
	}
	r = admin.call(http.MethodGet, adminTickets+"/"+uuid.NewString(), nil)
	if !strings.Contains(string(r.body), "Demande d'assistance introuvable.") {
		t.Fatalf("fr not found: %s", r.body)
	}
	admin.locale = "es"
	r = admin.call(http.MethodGet, adminTickets+"?priority=nope", nil)
	if !strings.Contains(string(r.body), "Elige baja, normal, alta o urgente.") {
		t.Fatalf("es: %s", r.body)
	}

	anon := e.client()
	anon.locale = "es"
	r = anon.linkReq(http.MethodPost, rec.Reference, token, map[string]any{"body": ""})
	if !strings.Contains(string(r.body), "Escribe un mensaje de 1 a 5.000 caracteres.") {
		t.Fatalf("es requester: %s", r.body)
	}
	r = anon.linkReq(http.MethodGet, rec.Reference, "wrong", nil)
	if !strings.Contains(string(r.body), "Solicitud de soporte") {
		t.Fatalf("es not found: %s", r.body)
	}
	anon.locale = "fr"
	if r := anon.linkReq(http.MethodGet, rec.Reference, "wrong", nil); !strings.Contains(string(r.body), "Demande d'assistance") {
		t.Fatalf("fr not found: %s", r.body)
	}
}

func TestSupportDeskKeysetPaginationAndSorts(t *testing.T) {
	e, admin := newDesk(t)
	marker := uniq("pg")
	var refs []string
	for i := 0; i < 8; i++ {
		refs = append(refs, e.newTicket(e.client(), uniq("p")+"@example.com", fmt.Sprintf("Page %s number %d", marker, i), "Paginating through the queue.").Reference)
		if i%3 != 2 { // some tickets share the same instant: the id breaks the tie
			e.clock.Advance(time.Minute)
		}
	}
	walk := func(query string) []string {
		var out []string
		seen := map[string]bool{}
		cursor := ""
		for page := 0; page < 20; page++ {
			q := query + "&limit=3"
			if cursor != "" {
				q += "&cursor=" + url.QueryEscape(cursor)
			}
			p := adminPage(t, admin, q)
			if len(p.Items) > 3 {
				t.Fatalf("page of %d for limit 3", len(p.Items))
			}
			for _, it := range p.Items {
				if seen[it.Reference] {
					t.Fatalf("duplicate %s across pages in %s", it.Reference, query)
				}
				seen[it.Reference] = true
				out = append(out, it.Reference)
			}
			if p.NextCursor == "" {
				return out
			}
			if len(p.Items) != 3 {
				t.Fatalf("a short page must be the last: %d items with a cursor", len(p.Items))
			}
			cursor = p.NextCursor
		}
		t.Fatal("pagination did not end")
		return nil
	}
	newest := walk("?q=" + marker)
	oldest := walk("?sort=oldest&q=" + marker)
	if len(newest) != 8 || len(oldest) != 8 {
		t.Fatalf("pages cover %d / %d tickets, want 8", len(newest), len(oldest))
	}
	for i := range newest { // exact mirror, ties included
		if newest[i] != oldest[7-i] {
			t.Fatalf("oldest is not the mirror of newest at %d: %v vs %v", i, newest, oldest)
		}
	}
	// Newest is ordered by creation time, descending.
	var times []time.Time
	for _, ref := range newest {
		times = append(times, e.ticketCols(ref)["created_at"].(time.Time))
	}
	for i := 1; i < len(times); i++ {
		if times[i].After(times[i-1]) {
			t.Fatalf("newest not descending at %d", i)
		}
	}
	// Exactly limit rows with nothing after: no cursor.
	if p := adminPage(t, admin, "?limit=8&q="+marker); len(p.Items) != 8 || p.NextCursor != "" {
		t.Fatalf("a full last page must not carry a cursor: %d %q", len(p.Items), p.NextCursor)
	}

	// waiting: open tickets first (the oldest unanswered message at the top), then the rest.
	admin.stepUp()
	adminReply(t, admin, refs[0], "Answered, now pending.", "public")
	adminReply(t, admin, refs[5], "Answered, now pending.", "public")
	waiting := walk("?sort=waiting&q=" + marker)
	if len(waiting) != 8 {
		t.Fatalf("waiting covers %d tickets: %v", len(waiting), waiting)
	}
	pend := map[string]bool{refs[0]: true, refs[5]: true}
	for i, ref := range waiting {
		if (i >= 6) != pend[ref] {
			t.Fatalf("pending tickets must come after the open ones: %v", waiting)
		}
	}
	var lcm []time.Time
	for _, ref := range waiting[:6] {
		lcm = append(lcm, e.ticketCols(ref)["last_customer_message_at"].(time.Time))
	}
	for i := 1; i < len(lcm); i++ {
		if lcm[i].Before(lcm[i-1]) {
			t.Fatalf("waiting is not oldest-first at %d", i)
		}
	}
	onlyOpen := walk("?sort=waiting&status=open&q=" + marker)
	if len(onlyOpen) != 6 || strings.Join(onlyOpen, ",") != strings.Join(waiting[:6], ",") {
		t.Fatalf("status=open waiting queue: %v vs %v", onlyOpen, waiting[:6])
	}
	if got := walk("?status=pending&q=" + marker); len(got) != 2 {
		t.Fatalf("pending: %v", got)
	}
	// A cursor of the wrong shape is refused, not misread.
	for _, q := range []string{"?cursor=garbage", "?sort=waiting&cursor=" + url.QueryEscape(adminPage(t, admin, "?limit=1&q="+marker).NextCursor), "?limit=101", "?limit=0"} {
		if r := admin.call(http.MethodGet, adminTickets+q, nil); r.status != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %s", q, r.status, r.body)
		}
	}
	if r := admin.call(http.MethodGet, adminTickets+"?limit=100", nil); r.status != http.StatusOK {
		t.Errorf("limit=100: %d", r.status)
	}
}

func TestSupportDeskCORSAllowsTheTokenHeader(t *testing.T) {
	e := newEnv(t)
	c := e.client()
	r := c.raw(http.MethodOptions, "/api/v1/support/tickets/SY-AAAAAA", nil, map[string]string{
		"Access-Control-Request-Method": "GET", "Access-Control-Request-Headers": "x-support-token"})
	if r.status != http.StatusNoContent || !strings.Contains(strings.ToLower(r.header.Get("Access-Control-Allow-Headers")), "x-support-token") ||
		r.header.Get("Access-Control-Allow-Origin") != origin {
		t.Fatalf("preflight: %d %v", r.status, r.header)
	}
	// The private link is never cached or leaked through a referrer.
	rec := e.newTicket(c, uniq("c")+"@example.com", "Headers", "Checking response headers of private pages.")
	token := e.tokenFrom(e.ticketEmailFromMail(rec.Reference), rec.Reference)
	r = c.linkReq(http.MethodGet, rec.Reference, token, nil)
	if r.status != http.StatusOK || !strings.Contains(r.header.Get("Cache-Control"), "no-store") || r.header.Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("headers: %d %v", r.status, r.header)
	}
}

func TestSupportDeskAdminRequestsRejectUnknownFields(t *testing.T) {
	e, admin := newDesk(t)
	rec := e.newTicket(e.client(), uniq("u")+"@example.com", "Strict bodies", "Unknown fields must be rejected.")
	admin.stepUp()
	if r := admin.call(http.MethodPost, adminTickets+"/"+rec.Reference+"/messages", map[string]any{"body": "x", "visibility": "public", "author_id": uuid.NewString()}); r.status < 400 || r.status >= 500 {
		t.Fatalf("reply with an unknown field: %d", r.status)
	}
	if r := adminPatch(admin, rec.Reference, map[string]any{"version": 99, "message_count": 0}); r.status < 400 || r.status >= 500 {
		t.Fatalf("patch with unknown fields: %d", r.status)
	}
	if got := e.ticketCols(rec.Reference); got["version"].(int32) != 1 || got["message_count"].(int32) != 1 {
		t.Fatalf("mass assignment: %+v", got)
	}
}
