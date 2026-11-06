package e2e

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// A contact-form request reaches the support inbox with its content, and the
// requester gets a localized acknowledgement that never repeats what they
// typed (the form must not relay text to arbitrary inboxes).
func TestSupportTicketIsEmailedAndAcknowledged(t *testing.T) {
	e := newEnv(t)
	anon := e.client()
	anon.locale = "fr"
	var receipt service.TicketReceipt
	anon.must(http.StatusCreated, http.MethodPost, "/api/v1/support/tickets", map[string]any{
		"name": "Mallory", "email": "victim@example.com", "topic": "general", "subject": "Buy cheap pills",
		"message": "Visit spam.example for cheap pills and more offers.", "consent": true,
	}, &receipt)

	var ack, team *struct{ to, subject, text string }
	for _, m := range e.flushMail() {
		m := m
		switch m.To {
		case "victim@example.com":
			ack = &struct{ to, subject, text string }{m.To, m.Subject, m.Text}
		case e.app.Config.Mail.SupportInbox:
			team = &struct{ to, subject, text string }{m.To, m.Subject, m.Text}
		}
	}
	if ack == nil || team == nil {
		t.Fatalf("expected an acknowledgement and a team copy, got ack=%v team=%v", ack, team)
	}
	if !strings.Contains(ack.subject, receipt.Reference) || !strings.Contains(ack.text, "Merci") {
		t.Fatalf("acknowledgement not localized or missing the reference: %q / %q", ack.subject, ack.text)
	}
	for _, leaked := range []string{"pills", "spam.example", "Mallory"} {
		if strings.Contains(ack.subject+ack.text, leaked) {
			t.Fatalf("acknowledgement relays submitted text %q", leaked)
		}
	}
	if !strings.Contains(team.subject, receipt.Reference) || !strings.Contains(team.text, "spam.example") ||
		!strings.Contains(team.text, "victim@example.com") {
		t.Fatalf("team copy: %q / %q", team.subject, team.text)
	}
	// Running the outbox again sends nothing twice.
	before := len(e.mail.Sent())
	if after := len(e.flushMail()); after != before {
		t.Fatalf("support e-mail sent twice: %d → %d", before, after)
	}
}
