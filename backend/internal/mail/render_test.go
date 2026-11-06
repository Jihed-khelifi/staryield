package mail

import (
	"strings"
	"testing"
)

func TestRenderEscapesUserContent(t *testing.T) {
	c, ok := Notification("en", "chat.missed", map[string]any{"client_name": `<script>alert(1)</script>`}, "https://staryield.net/en/chatroom")
	if !ok {
		t.Fatal("chat.missed has an e-mail")
	}
	r, err := Render(c, Layout{Locale: "en", FrontendURL: "https://staryield.net", Category: "notification"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.HTML, "<script>") {
		t.Fatal("user-provided name rendered as markup")
	}
	if !strings.Contains(r.HTML, "&lt;script&gt;") {
		t.Fatal("name missing from the HTML body")
	}
	if !strings.Contains(r.Text, "https://staryield.net/en/chatroom") {
		t.Fatal("text body lacks the action link")
	}
}

func TestRenderHeaders(t *testing.T) {
	c := PasswordReset("en", "https://staryield.net/en/reset-password?token=abc", 30)
	c.Subject = "Line one\r\nBcc: attacker@example.com"
	r, err := Render(c, Layout{Locale: "en", FrontendURL: "https://staryield.net", Category: "security"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(r.Subject, "\r\n") {
		t.Fatalf("subject keeps line breaks: %q", r.Subject)
	}
	if _, ok := r.Headers["List-Unsubscribe"]; ok {
		t.Fatal("security e-mail must not offer unsubscribe")
	}
	if r.Headers["Auto-Submitted"] != "auto-generated" {
		t.Fatal("missing Auto-Submitted")
	}
	n, _ := Notification("en", "review.received", map[string]any{"rating": 5}, "https://staryield.net/en/profile/analytics")
	r, _ = Render(n, Layout{Locale: "en", FrontendURL: "https://staryield.net", Category: "notification"})
	if r.Headers["List-Unsubscribe"] != "<https://staryield.net/en/profile/settings#notifications>" {
		t.Fatalf("list-unsubscribe: %q", r.Headers["List-Unsubscribe"])
	}
	// Cloudflare rejects non-https unsubscribe links: none in development.
	r, _ = Render(n, Layout{Locale: "en", FrontendURL: "http://localhost:3000", Category: "notification"})
	if _, ok := r.Headers["List-Unsubscribe"]; ok {
		t.Fatal("http unsubscribe link must be omitted")
	}
}

func TestAppURL(t *testing.T) {
	cases := map[string]string{
		"/chatroom?reader=ramone": "https://staryield.net/fr/chatroom?reader=ramone",
		"":                        "https://staryield.net/fr",
		"//evil.example":          "https://staryield.net/fr",
		"https://evil.example/x":  "https://staryield.net/fr",
		`/\evil`:                  "https://staryield.net/fr",
	}
	for in, want := range cases {
		if got := AppURL("https://staryield.net/", "fr-CA", in); got != want {
			t.Errorf("AppURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPasswordResetKeepsTokenExtractable(t *testing.T) {
	link := "https://staryield.net/fr/reset-password?token=TOKEN123"
	r, err := Render(PasswordReset("fr", link, 30), Layout{Locale: "fr", FrontendURL: "https://staryield.net", Category: "security"})
	if err != nil {
		t.Fatal(err)
	}
	i := strings.Index(r.Text, "token=")
	if i < 0 || strings.Fields(r.Text[i+6:])[0] != "TOKEN123" {
		t.Fatalf("token not extractable from text body:\n%s", r.Text)
	}
	if !strings.Contains(r.Subject, "Réinitialisez") {
		t.Fatalf("subject not localised: %q", r.Subject)
	}
}
