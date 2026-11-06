package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// sessionBetween starts and joins a session between an existing client and
// another advisor. The client's previous session must already be over.
func sessionBetween(t *testing.T, client *client, cws *ws, psychic *client) service.SessionView {
	t.Helper()
	pws := psychic.goOnline()
	cws.drain()
	var requested service.SessionView
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions", map[string]any{
		"psychic_slug": psychic.slug(), "request_key": uniq("req-key-"),
	}, &requested)
	pws.waitFor(realtime.EventSessionRequested, 5*time.Second)
	cws.mustAck(realtime.FrameSessionJoin, map[string]any{"session_id": requested.ID})
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+requested.ID.String()+"/accept", nil, nil)
	pws.mustAck(realtime.FrameSessionJoin, map[string]any{"session_id": requested.ID})
	for {
		started := sessionFrom(t, cws.waitFor(realtime.EventSessionStarted, 5*time.Second).Data)
		if started.ID == requested.ID {
			if started.StartedAt == nil {
				t.Fatalf("session not started: %+v", started)
			}
			return started
		}
	}
}

func endSession(t *testing.T, c *client, id fmt.Stringer) {
	t.Helper()
	if r := c.call(http.MethodPost, "/api/v1/chat/sessions/"+id.String()+"/end", nil); r.status != http.StatusOK {
		t.Fatalf("end session: %d %s", r.status, r.body)
	}
}

func notesPath(conversationID fmt.Stringer) string {
	return "/api/v1/chat/conversations/" + conversationID.String() + "/notes"
}

func TestAdvisorNotesLifecycle(t *testing.T) {
	e := newEnv(t)
	client, psychic, _, _, s := startedSession(t, e, 10, 250)
	path := notesPath(s.ConversationID)

	// Validation: empty, whitespace-only, too long, unknown fields.
	for _, body := range []string{"", "   \n\t ", strings.Repeat("é", 2001)} {
		if r := psychic.call(http.MethodPost, path, map[string]any{"body": body}); r.errCode(t) != "validation_failed" {
			t.Fatalf("body %q accepted: %d %s", body[:min(len(body), 10)], r.status, r.body)
		}
	}
	if r := psychic.call(http.MethodPost, path, map[string]any{"body": "x", "author_id": psychic.user.ID}); r.errCode(t) != "validation_failed" ||
		!strings.Contains(string(r.body), "author_id") {
		t.Fatalf("unknown field accepted: %d %s", r.status, r.body)
	}

	// Exactly 2,000 characters is fine; control characters are stripped.
	var long service.NoteView
	psychic.must(http.StatusCreated, http.MethodPost, path, map[string]any{"body": strings.Repeat("é", 2000)}, &long)

	var note service.NoteView
	psychic.must(http.StatusCreated, http.MethodPost, path, map[string]any{"body": "Lost her dog yesterday\u0007"}, &note)
	if note.Body != "Lost her dog yesterday" || note.Shared {
		t.Fatalf("created note: %+v", note)
	}

	// The body is sealed at rest.
	var stored string
	if err := e.app.DB.Table("client_notes").Select("body_enc").Where("id = ?", note.ID).Scan(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored == "" || strings.Contains(stored, "dog") {
		t.Fatalf("note body is not sealed: %q", stored)
	}

	// Edit the body and share it; the update is visible in the list.
	e.clock.Advance(time.Minute)
	var edited service.NoteView
	psychic.must(http.StatusOK, http.MethodPatch, "/api/v1/chat/notes/"+note.ID.String(),
		map[string]any{"body": "Lost her dog in an accident", "shared": true}, &edited)
	if edited.Body != "Lost her dog in an accident" || !edited.Shared || !edited.UpdatedAt.After(edited.CreatedAt) {
		t.Fatalf("edited note: %+v", edited)
	}
	if r := psychic.call(http.MethodPatch, "/api/v1/chat/notes/"+note.ID.String(), map[string]any{}); r.status != http.StatusBadRequest {
		t.Fatalf("empty patch: %d %s", r.status, r.body)
	}
	var page service.Page[service.NoteView]
	psychic.must(http.StatusOK, http.MethodGet, path, nil, &page)
	if len(page.Items) != 2 || page.Items[0].ID != note.ID || page.Items[0].Body != "Lost her dog in an accident" || page.NextCursor != "" {
		t.Fatalf("list: %+v", page)
	}

	// Clients never reach the notes API; other advisors cannot probe IDs.
	if r := client.call(http.MethodGet, path, nil); r.status != http.StatusForbidden {
		t.Fatalf("client listing notes: %d", r.status)
	}
	other := e.createPsychic(10)
	if r := other.call(http.MethodGet, path, nil); r.status != http.StatusNotFound {
		t.Fatalf("other advisor listing notes: %d", r.status)
	}
	if r := other.call(http.MethodPost, path, map[string]any{"body": "hijack"}); r.status != http.StatusNotFound {
		t.Fatalf("other advisor writing notes: %d", r.status)
	}
	if r := other.call(http.MethodPatch, "/api/v1/chat/notes/"+note.ID.String(), map[string]any{"body": "x"}); r.status != http.StatusNotFound {
		t.Fatalf("other advisor editing note: %d", r.status)
	}
	if r := other.call(http.MethodDelete, "/api/v1/chat/notes/"+note.ID.String(), nil); r.status != http.StatusNotFound {
		t.Fatalf("other advisor deleting note: %d", r.status)
	}
	if r := psychic.call(http.MethodGet, notesPath(note.ID), nil); r.status != http.StatusNotFound {
		t.Fatalf("unknown conversation: %d", r.status)
	}

	// Delete is soft: the row stays but disappears from every listing.
	psychic.must(http.StatusNoContent, http.MethodDelete, "/api/v1/chat/notes/"+note.ID.String(), nil, nil)
	if r := psychic.call(http.MethodDelete, "/api/v1/chat/notes/"+note.ID.String(), nil); r.status != http.StatusNotFound {
		t.Fatalf("second delete: %d", r.status)
	}
	psychic.must(http.StatusOK, http.MethodGet, path, nil, &page)
	if len(page.Items) != 1 || page.Items[0].ID != long.ID {
		t.Fatalf("after delete: %+v", page.Items)
	}
	var deletedAt *time.Time
	if err := e.app.DB.Table("client_notes").Select("deleted_at").Where("id = ?", note.ID).Scan(&deletedAt).Error; err != nil || deletedAt == nil {
		t.Fatalf("soft delete: %v %v", deletedAt, err)
	}
}

func TestAdvisorNotesPagination(t *testing.T) {
	e := newEnv(t)
	_, psychic, _, _, s := startedSession(t, e, 10, 250)
	path := notesPath(s.ConversationID)
	const total = 12
	for i := 0; i < total; i++ {
		e.clock.Advance(time.Second)
		psychic.must(http.StatusCreated, http.MethodPost, path, map[string]any{"body": fmt.Sprintf("note %02d", i)}, nil)
	}
	seen := map[string]bool{}
	cursor, pages := "", 0
	for {
		var page service.Page[service.NoteView]
		psychic.must(http.StatusOK, http.MethodGet, path+"?limit=5&cursor="+cursor, nil, &page)
		pages++
		for _, n := range page.Items {
			if seen[n.Body] {
				t.Fatalf("duplicate %q", n.Body)
			}
			seen[n.Body] = true
		}
		if page.NextCursor == "" {
			if len(page.Items) == 0 {
				t.Fatal("last page should not be empty")
			}
			break
		}
		cursor = page.NextCursor
	}
	if len(seen) != total || pages != 3 {
		t.Fatalf("paged %d notes over %d pages", len(seen), pages)
	}
	if r := psychic.call(http.MethodGet, path+"?limit=0", nil); r.errCode(t) != "validation_failed" {
		t.Fatalf("limit=0: %s", r.body)
	}
	if r := psychic.call(http.MethodGet, path+"?cursor=not-a-cursor", nil); r.errCode(t) != "validation_failed" {
		t.Fatalf("bad cursor: %s", r.body)
	}
}

func TestSharedNotesReachOnlyAdvisorsWhoMetTheClient(t *testing.T) {
	e := newEnv(t)
	client, ramone, cws, _, first := startedSession(t, e, 10, 250)
	shared := service.NoteView{}
	ramone.must(http.StatusCreated, http.MethodPost, notesPath(first.ConversationID),
		map[string]any{"body": "Got divorced last year", "shared": true}, &shared)
	ramone.must(http.StatusCreated, http.MethodPost, notesPath(first.ConversationID),
		map[string]any{"body": "Private impression"}, nil)
	endSession(t, client, first.ID)

	// Theo reads for the same client afterwards.
	theo := e.createPsychic(10)
	second := sessionBetween(t, client, cws, theo)
	var fromOthers service.Page[service.SharedNoteView]
	theo.must(http.StatusOK, http.MethodGet, notesPath(second.ConversationID)+"/shared", nil, &fromOthers)
	if len(fromOthers.Items) != 1 || fromOthers.Items[0].ID != shared.ID || fromOthers.Items[0].Body != "Got divorced last year" ||
		fromOthers.Items[0].Author.Slug != ramone.slug() || fromOthers.Items[0].Author.Name == "" {
		t.Fatalf("shared notes for theo: %+v", fromOthers.Items)
	}
	// Theo's own list stays his own, and he cannot edit Ramone's note.
	var own service.Page[service.NoteView]
	theo.must(http.StatusOK, http.MethodGet, notesPath(second.ConversationID), nil, &own)
	if len(own.Items) != 0 {
		t.Fatalf("theo sees someone else's notes as his own: %+v", own.Items)
	}
	if r := theo.call(http.MethodPatch, "/api/v1/chat/notes/"+shared.ID.String(), map[string]any{"shared": false}); r.status != http.StatusNotFound {
		t.Fatalf("theo editing ramone's note: %d", r.status)
	}
	// Ramone does not see his own notes in "from other advisors".
	ramone.must(http.StatusOK, http.MethodGet, notesPath(first.ConversationID)+"/shared", nil, &fromOthers)
	if len(fromOthers.Items) != 0 {
		t.Fatalf("own notes listed as shared: %+v", fromOthers.Items)
	}
	// Unsharing hides it again.
	ramone.must(http.StatusOK, http.MethodPatch, "/api/v1/chat/notes/"+shared.ID.String(), map[string]any{"shared": false}, nil)
	theo.must(http.StatusOK, http.MethodGet, notesPath(second.ConversationID)+"/shared", nil, &fromOthers)
	if len(fromOthers.Items) != 0 {
		t.Fatalf("unshared note still visible: %+v", fromOthers.Items)
	}
	endSession(t, client, second.ID)

	// Solomon declined the request: a conversation exists but they never met,
	// so he can neither read shared notes nor write his own.
	solomon := e.createPsychic(10)
	sws := solomon.goOnline()
	var requested service.SessionView
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions", map[string]any{
		"psychic_slug": solomon.slug(), "request_key": uniq("req-key-"),
	}, &requested)
	sws.waitFor(realtime.EventSessionRequested, 5*time.Second)
	solomon.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+requested.ID.String()+"/decline", nil, nil)
	if r := solomon.call(http.MethodGet, notesPath(requested.ConversationID)+"/shared", nil); r.status != http.StatusForbidden {
		t.Fatalf("advisor who never met the client reads shared notes: %d %s", r.status, r.body)
	}
	if r := solomon.call(http.MethodPost, notesPath(requested.ConversationID), map[string]any{"body": "hello"}); r.status != http.StatusForbidden {
		t.Fatalf("advisor who never met the client writes a note: %d %s", r.status, r.body)
	}
}

func TestNotesAreErasedWithTheClientAccount(t *testing.T) {
	e := newEnv(t)
	client, psychic, _, _, s := startedSession(t, e, 10, 250)
	psychic.must(http.StatusCreated, http.MethodPost, notesPath(s.ConversationID), map[string]any{"body": "Struggles with sleep", "shared": true}, nil)
	var before int64
	e.app.DB.Table("client_notes").Where("client_id = ?", client.user.ID).Count(&before)
	if before != 1 {
		t.Fatalf("notes before erasure: %d", before)
	}
	admin := e.createAdmin()
	admin.stepUp()
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/users/"+client.user.ID.String(), map[string]any{"reason": "GDPR erasure"}, nil)
	var after int64
	e.app.DB.Table("client_notes").Where("client_id = ?", client.user.ID).Count(&after)
	if after != 0 {
		t.Fatalf("notes survived account erasure: %d", after)
	}
}

func TestAdminNoteOversightIsScopedAndAudited(t *testing.T) {
	e := newEnv(t)
	client, psychic, _, _, s := startedSession(t, e, 10, 250)
	var private, shared service.NoteView
	psychic.must(http.StatusCreated, http.MethodPost, notesPath(s.ConversationID), map[string]any{"body": "Private impression"}, &private)
	psychic.must(http.StatusCreated, http.MethodPost, notesPath(s.ConversationID), map[string]any{"body": "Shared context", "shared": true}, &shared)
	base := "/api/v1/admin/client-notes?client_id=" + client.user.ID.String()

	// Regular accounts never reach the console API.
	if r := psychic.call(http.MethodGet, base+"&reason=curious", nil); r.status == http.StatusOK {
		t.Fatalf("advisor read the admin notes API: %s", r.body)
	}

	admin := e.createAdmin()
	e.clock.Advance(11 * time.Minute) // step-up window from the login has passed
	if r := admin.call(http.MethodGet, base+"&reason=Access+request", nil); r.errCode(t) != "step_up_required" {
		t.Fatalf("notes without step-up: %d %s", r.status, r.body)
	}
	admin.stepUp()
	if r := admin.call(http.MethodGet, base, nil); r.errCode(t) != "validation_failed" {
		t.Fatalf("notes without a reason: %s", r.body)
	}
	if r := admin.call(http.MethodGet, "/api/v1/admin/client-notes?reason=Access+request", nil); r.errCode(t) != "validation_failed" {
		t.Fatalf("unscoped notes listing: %s", r.body)
	}

	var page service.Page[service.AdminNoteView]
	admin.must(http.StatusOK, http.MethodGet, base+"&reason=GDPR+access+request", nil, &page)
	if len(page.Items) != 2 || page.Items[0].ID != shared.ID || page.Items[1].ID != private.ID || page.Items[1].Body != "Private impression" ||
		page.Items[0].Author.Slug != psychic.slug() || page.Items[0].Client.ID != client.user.ID || page.Items[0].Client.Email != client.user.Email {
		t.Fatalf("admin notes: %+v", page.Items)
	}

	// Moderation removal is soft, audited, and hides the note from advisors.
	if r := admin.call(http.MethodDelete, "/api/v1/admin/client-notes/"+shared.ID.String(), map[string]any{"reason": ""}); r.errCode(t) != "validation_failed" {
		t.Fatalf("remove without reason: %s", r.body)
	}
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/client-notes/"+shared.ID.String(), map[string]any{"reason": "Contains a health detail"}, nil)
	var own service.Page[service.NoteView]
	psychic.must(http.StatusOK, http.MethodGet, notesPath(s.ConversationID), nil, &own)
	if len(own.Items) != 1 || own.Items[0].ID != private.ID {
		t.Fatalf("advisor still sees removed note: %+v", own.Items)
	}
	admin.must(http.StatusOK, http.MethodGet, base+"&include_deleted=true&reason=Review", nil, &page)
	if len(page.Items) != 2 || page.Items[0].DeletedAt == nil {
		t.Fatalf("include_deleted: %+v", page.Items)
	}

	var audit service.Page[service.AuditView]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/audit-logs?action=notes.viewed", nil, &audit)
	if len(audit.Items) < 2 || audit.Items[len(audit.Items)-1].Metadata["reason"] != "GDPR access request" {
		t.Fatalf("notes.viewed audit: %+v", audit.Items)
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/audit-logs?action=note.removed", nil, &audit)
	if len(audit.Items) != 1 || audit.Items[0].TargetID != shared.ID.String() || audit.Items[0].Metadata["reason"] != "Contains a health detail" {
		t.Fatalf("note.removed audit: %+v", audit.Items)
	}
	// Nothing in the audit trail contains a note body.
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/audit-logs?target_type=client_note", nil, &audit)
	for _, a := range audit.Items {
		for _, v := range a.Metadata {
			if s, ok := v.(string); ok && (strings.Contains(s, "impression") || strings.Contains(s, "context")) {
				t.Fatalf("audit leaks a note body: %+v", a)
			}
		}
	}
}
