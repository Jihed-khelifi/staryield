package e2e

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

type ticketRow struct {
	Reference    string
	UserID       *uuid.UUID
	Name         string
	EmailEnc     string
	EmailHash    string
	Topic        string
	Subject      string
	MessageEnc   string
	MessageChars int
	Locale       string
	Status       string
	IPHash       string
}

func ticketByReference(t *testing.T, e *env, ref string) ticketRow {
	t.Helper()
	var row ticketRow
	if err := e.app.DB.Table("support_tickets").Where("reference = ?", ref).Scan(&row).Error; err != nil || row.Reference == "" {
		t.Fatalf("ticket %s not stored: %v", ref, err)
	}
	return row
}

func TestSupportTicketsAreStoredSealedAndLinked(t *testing.T) {
	e := newEnv(t)
	refRe := regexp.MustCompile(`^SY-[A-HJ-NP-Z2-9]{6}$`)

	// Anonymous visitor, French UI.
	anon := e.client()
	anon.locale = "fr"
	var receipt service.TicketReceipt
	anon.must(http.StatusCreated, http.MethodPost, "/api/v1/support/tickets", map[string]any{
		"email": "Visitor@Example.com", "topic": "privacy", "subject": "Export my data",
		"message": "Please send me a copy of everything you hold about me.", "consent": true,
	}, &receipt)
	if !refRe.MatchString(receipt.Reference) || receipt.Topic != "privacy" || receipt.CreatedAt.IsZero() {
		t.Fatalf("receipt: %+v", receipt)
	}
	firstRef := receipt.Reference
	row := ticketByReference(t, e, receipt.Reference)
	if row.UserID != nil || row.Locale != "fr" || row.Status != "open" || row.Topic != "privacy" || row.Subject != "Export my data" {
		t.Fatalf("anonymous row: %+v", row)
	}
	if len(row.EmailHash) != 64 || row.IPHash == "" || row.MessageChars != len("Please send me a copy of everything you hold about me.") {
		t.Fatalf("hashes/lengths: %+v", row)
	}
	if strings.Contains(strings.ToLower(row.EmailEnc), "visitor") || strings.Contains(row.MessageEnc, "copy of everything") {
		t.Fatal("email or message stored in clear text")
	}

	// Signed-in client: the ticket is linked to the account.
	client := e.registerClient()
	client.must(http.StatusCreated, http.MethodPost, "/api/v1/support/tickets", map[string]any{
		"name": "Luna", "email": client.user.Email, "topic": "billing", "subject": "Double charge",
		"message": "I was charged twice for the same pack today.", "consent": true,
	}, &receipt)
	row = ticketByReference(t, e, receipt.Reference)
	if row.UserID == nil || *row.UserID != client.user.ID || row.Name != "Luna" || row.Locale != "en" {
		t.Fatalf("signed-in row: %+v", row)
	}

	// The same address hashes the same way, so tickets can be grouped.
	other := e.client()
	other.must(http.StatusCreated, http.MethodPost, "/api/v1/support/tickets", map[string]any{
		"email": "visitor@example.com", "subject": "Follow-up", "message": "Any news about my export request?", "consent": true,
	}, &receipt)
	if got := ticketByReference(t, e, receipt.Reference); got.Topic != "general" {
		t.Fatalf("follow-up should default to the general topic: %+v", got)
	}
	// (Compared by reference: other tests share the database.)
	if a, b := ticketByReference(t, e, firstRef).EmailHash, ticketByReference(t, e, receipt.Reference).EmailHash; a != b || len(a) != 64 {
		t.Fatalf("email hashes should match for the same address: %s vs %s", a, b)
	}
}

func TestSupportTicketValidation(t *testing.T) {
	e := newEnv(t)
	c := e.client()
	r := c.call(http.MethodPost, "/api/v1/support/tickets", map[string]any{
		"email": "nope", "topic": "refunds", "subject": "Hi", "message": "short", "consent": false,
	})
	if r.errCode(t) != "validation_failed" {
		t.Fatalf("invalid ticket: %d %s", r.status, r.body)
	}
	for _, field := range []string{"email", "topic", "subject", "message", "consent"} {
		if !strings.Contains(string(r.body), `"`+field+`"`) {
			t.Errorf("no error for %s: %s", field, r.body)
		}
	}
	// Unknown fields (e.g. a smuggled user_id or status) are rejected.
	r = c.call(http.MethodPost, "/api/v1/support/tickets", map[string]any{
		"email": "a@example.com", "subject": "Hello there", "message": "A perfectly fine message.", "consent": true, "status": "closed",
	})
	if r.errCode(t) != "validation_failed" || !strings.Contains(string(r.body), "status") {
		t.Fatalf("unknown field: %d %s", r.status, r.body)
	}
	// Plain form posts are refused: JSON only.
	if r := c.raw(http.MethodPost, "/api/v1/support/tickets", strings.NewReader("email=a@b.co"), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded", "X-CSRF-Token": csrfFor(c),
	}); r.status != http.StatusUnsupportedMediaType {
		t.Fatalf("form post: %d %s", r.status, r.body)
	}
}

func csrfFor(c *client) string {
	c.ensureCSRF()
	return c.csrf
}

func TestSupportTicketsAreRateLimited(t *testing.T) {
	e := newEnv(t, func(cfg *config.Config) { cfg.RateLimit.Scale = 1 })
	c := e.client()
	body := map[string]any{"email": "spam@example.com", "subject": "Hello there", "message": "Just checking the limit here.", "consent": true}
	for i := 0; i < 5; i++ {
		c.must(http.StatusCreated, http.MethodPost, "/api/v1/support/tickets", body, nil)
	}
	r := c.call(http.MethodPost, "/api/v1/support/tickets", body)
	if r.status != http.StatusTooManyRequests || r.errCode(t) != "rate_limited" || r.header.Get("Retry-After") == "" {
		t.Fatalf("6th ticket within the hour: %d %s", r.status, r.body)
	}
}
