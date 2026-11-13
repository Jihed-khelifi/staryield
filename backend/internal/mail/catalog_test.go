package mail

import (
	"strings"
	"testing"
)

// samples covers every kind with an e-mail or push form.
var samples = map[string]map[string]any{
	"chat.requested":     {"client_name": "Nova"},
	"chat.missed":        {"client_name": "Nova"},
	"chat.review_prompt": {"psychic_name": "Ramone"},
	"review.received":    {"rating": float64(5)},
	"request.created":    {"service_name": "Candle Ritual", "client_name": "Nova"},
	"request.updated":    {"service_name": "Candle Ritual", "status": "accepted"},
	"purchase.succeeded": {"credits": float64(600), "price_cents": float64(1999), "currency": "USD", "pack_name": "Starter",
		"purchase_id": "0b6d7c1e-0000-4000-8000-000000000001", "completed_at": "2026-11-12T10:15:00Z", "card_brand": "visa", "card_last4": "4242",
		"statement_descriptor": "STARYIELD MEDIA LLC"},
	"credits.adjusted":   {"amount": float64(-50)},
	"withdrawal.updated": {"status": "paid", "amount_credits": float64(2500)},
	"waitlist.available": {"psychic_name": "Theo"},
	"export.ready":       {"export_id": "x"},
	"subscription.renewed": {
		"plan_name":    "Premium",
		"amount_cents": float64(999), "currency": "USD", "period_months": float64(1), "period_start": "2026-11-12T10:15:00Z",
		"period_end": "2026-12-12T10:15:00Z", "next_renewal_at": "2026-12-12T10:15:00Z", "statement_descriptor": "STARYIELD MEDIA LLC",
	},
	"subscription.started": {
		"plan_name": "Premium", "plan_names": map[string]any{"en": "Premium", "fr": "Premium FR"},
		"amount_cents": float64(999), "currency": "USD", "period_months": float64(1), "period_start": "2026-11-12T10:15:00Z",
		"period_end": "2026-12-12T10:15:00Z", "next_renewal_at": "2026-12-12T10:15:00Z", "statement_descriptor": "STARYIELD MEDIA LLC",
	},
	"payment.refunded": {"amount_cents": float64(1999), "currency": "USD", "pack_name": "Starter", "credits": float64(600),
		"purchased_at": "2026-11-12T10:15:00Z", "statement_descriptor": "STARYIELD MEDIA LLC"},
	"refund_request.resolved": {"status": "approved", "reference": "RR-ABCD2345", "refunded_cents": float64(1999), "currency": "USD",
		"pack_name": "Starter", "credits": float64(600), "purchased_at": "2026-11-12T10:15:00Z", "statement_descriptor": "STARYIELD MEDIA LLC"},
	"subscription.payment_failed": {},
	"subscription.canceled":       {},
	"broadcast": {"title": map[string]any{"en": "New moon", "fr": "Nouvelle lune"},
		"body": map[string]any{"en": "Tonight", "fr": "Ce soir"}},
}

func contentStrings(c Content) []string {
	out := append([]string{c.Subject, c.Heading}, c.Paragraphs...)
	if c.Action != nil {
		out = append(out, c.Action.Label)
	}
	if c.Note != "" {
		out = append(out, c.Note)
	}
	for _, d := range c.Details { // revisions 5.1: labels are copy, values are data
		out = append(out, d.Label)
	}
	for _, l := range c.Links {
		out = append(out, l.Label)
	}
	return out
}

// Every catalogue string must be translated: rendering a kind in French and
// Spanish must not leave an English sentence behind.
func TestCatalogueIsTranslated(t *testing.T) {
	builders := map[string]func(locale string) Content{
		"password_reset": func(l string) Content { return PasswordReset(l, "https://x/y", 30) },
		"email_change":   func(l string) Content { return EmailChangeRequested(l, "https://x/y", 60) },
		"email_changed":  func(l string) Content { return EmailChanged(l, "j***@example.com") },
		"closure":        func(l string) Content { return ClosureScheduled(l, "2026-12-04", "https://x/y") },
		"reactivated":    func(l string) Content { return Reactivated(l, "https://x/y") },
	}
	for kind, data := range samples {
		kind, data := kind, data
		if kind == "broadcast" {
			continue // admin-authored, already localised
		}
		builders[kind] = func(l string) Content {
			c, ok := Notification(l, kind, data, "https://x/y")
			if !ok {
				t.Fatalf("%s has no content", kind)
			}
			return c
		}
	}
	for name, build := range builders {
		en := contentStrings(build("en"))
		for _, loc := range []string{"fr", "es"} {
			got := contentStrings(build(loc))
			for i := range en {
				if en[i] != "" && got[i] == en[i] {
					t.Errorf("%s/%s: untranslated %q", name, loc, en[i])
				}
			}
		}
	}
}

func TestNotificationVariants(t *testing.T) {
	for _, status := range []string{"accepted", "delivered", "declined", "cancelled", "expired", "disputed", "refunded", "weird"} {
		c, _ := Notification("es", "request.updated", map[string]any{"service_name": "Tarot", "status": status}, "")
		if !strings.Contains(c.Paragraphs[0], "Tarot") || strings.Contains(c.Paragraphs[0], "%s") {
			t.Errorf("request.updated %s: %q", status, c.Paragraphs[0])
		}
	}
	for _, status := range []string{"approved", "paid", "rejected", "other"} {
		c, _ := Notification("fr", "withdrawal.updated", map[string]any{"status": status, "amount_credits": 2500}, "")
		if !strings.Contains(c.Paragraphs[0], "2500") {
			t.Errorf("withdrawal %s: %q", status, c.Paragraphs[0])
		}
	}
	c, _ := Notification("en", "credits.adjusted", map[string]any{"amount": float64(-50)}, "")
	if !strings.Contains(c.Paragraphs[0], "50 credits were removed") {
		t.Fatalf("negative adjustment: %q", c.Paragraphs[0])
	}
	// Missing names fall back to neutral copy.
	c, _ = Notification("en", "chat.missed", map[string]any{}, "")
	if !strings.HasPrefix(c.Paragraphs[0], "A client tried") {
		t.Fatalf("fallback: %q", c.Paragraphs[0])
	}
	// Names are single-line and bounded.
	c, _ = Notification("en", "chat.missed", map[string]any{"client_name": "A\nB " + strings.Repeat("x", 200)}, "")
	if strings.Contains(c.Paragraphs[0], "\n") || len([]rune(c.Paragraphs[0])) > 160 {
		t.Fatalf("name not cleaned: %q", c.Paragraphs[0])
	}
	if _, ok := Notification("en", "unknown.kind", nil, ""); ok {
		t.Fatal("unknown kinds have no e-mail")
	}
	// Broadcasts use the admin's localised text, falling back to English.
	c, _ = Notification("fr", "broadcast", samples["broadcast"], "")
	if c.Subject != "Nouvelle lune" {
		t.Fatalf("broadcast fr: %q", c.Subject)
	}
	c, _ = Notification("es", "broadcast", samples["broadcast"], "")
	if c.Subject != "New moon" {
		t.Fatalf("broadcast fallback: %q", c.Subject)
	}
}

func TestPushText(t *testing.T) {
	title, body, ok := PushText("fr", "chat.requested", map[string]any{"client_name": "Nova"})
	if !ok || title != "Nouvelle demande de chat" || !strings.Contains(body, "Nova") {
		t.Fatalf("push text: %q %q %v", title, body, ok)
	}
	if _, _, ok := PushText("en", "nope", nil); ok {
		t.Fatal("unknown kind")
	}
}
