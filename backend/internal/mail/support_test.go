package mail

import (
	"strings"
	"testing"
)

const testSupportLink = "https://staryield.test/fr/support/SY-AB2CD3#t=abc"

func TestSupportReplyIsLocalizedAndCarriesLink(t *testing.T) {
	body := "Bonjour,\nvoici la réponse.\n\nSecond paragraphe."
	for _, loc := range []string{"en", "fr", "es"} {
		c := SupportReply(loc, "SY-AB2CD3", body, testSupportLink)
		if !strings.Contains(c.Subject, "SY-AB2CD3") || c.Action == nil || c.Action.URL != testSupportLink {
			t.Fatalf("%s: %+v", loc, c)
		}
		// The staff reply is kept verbatim, split at blank lines.
		joined := strings.Join(c.Paragraphs, "||")
		if !strings.Contains(joined, "Bonjour,\nvoici la réponse.||Second paragraphe.") {
			t.Fatalf("%s: reply text altered: %q", loc, joined)
		}
		r, err := Render(c, Layout{Locale: loc, FrontendURL: "https://staryield.test"})
		if err != nil || !strings.Contains(r.Text, testSupportLink) || !strings.Contains(r.HTML, "SY-AB2CD3") {
			t.Fatalf("%s render: %v", loc, err)
		}
	}
	en, fr, es := SupportReply("en", "SY-AB2CD3", "x", testSupportLink), SupportReply("fr", "SY-AB2CD3", "x", testSupportLink), SupportReply("es", "SY-AB2CD3", "x", testSupportLink)
	if en.Subject == fr.Subject || en.Subject == es.Subject || fr.Subject == es.Subject {
		t.Fatalf("subjects must differ per locale: %q %q %q", en.Subject, fr.Subject, es.Subject)
	}
	for _, c := range []Content{fr, es} {
		for _, p := range append([]string{c.Heading, c.Note, c.Action.Label}, c.Paragraphs[0], c.Paragraphs[len(c.Paragraphs)-1]) {
			for _, english := range []string{"Our support team", "Use the button", "Replies to this email", "View your request", "Our team replied"} {
				if strings.Contains(p, english) {
					t.Errorf("untranslated copy %q in %q", english, p)
				}
			}
		}
	}
	// The reply never repeats the requester's subject: the form accepts any address.
	if strings.Contains(strings.ToLower(en.Subject), "subject") {
		t.Fatal("subject must not echo requester input")
	}
}

func TestSupportAcknowledgementCarriesPrivateLinkAndNoSubmittedText(t *testing.T) {
	for _, loc := range []string{"en", "fr", "es"} {
		c := SupportAcknowledgement(loc, "SY-AB2CD3", testSupportLink)
		if c.Action == nil || c.Action.URL != testSupportLink || !strings.Contains(c.Subject, "SY-AB2CD3") {
			t.Fatalf("%s: %+v", loc, c)
		}
	}
	if c := SupportAcknowledgement("fr", "SY-AB2CD3", ""); c.Action != nil {
		t.Fatal("no link, no button")
	}
	if c := SupportAcknowledgement("fr", "SY-AB2CD3", testSupportLink); !strings.Contains(strings.Join(c.Paragraphs, " "), "lien") {
		t.Fatalf("French acknowledgement should explain the link: %v", c.Paragraphs)
	}
}

func TestSupportCustomerReplyForTeam(t *testing.T) {
	c := SupportCustomerReply("SY-AB2CD3", "Visitor", "More details\n\nplease", "https://admin.test/support/1", true)
	if !strings.Contains(c.Subject, "SY-AB2CD3") || c.Action == nil || c.Action.URL != "https://admin.test/support/1" {
		t.Fatalf("%+v", c)
	}
	joined := strings.Join(c.Paragraphs, "|")
	for _, want := range []string{"Request: SY-AB2CD3", "From: Visitor", "More details|please", "re-opened"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %q", want, joined)
		}
	}
	if c := SupportCustomerReply("SY-AB2CD3", "", "x", "", false); c.Action != nil || !strings.Contains(strings.Join(c.Paragraphs, "|"), "(no name)") ||
		!strings.Contains(strings.Join(c.Paragraphs, "|"), "stays open") {
		t.Fatalf("%+v", c)
	}
}

func TestSupportRepliedPushText(t *testing.T) {
	title, body, ok := PushText("fr", "support.replied", map[string]any{"reference": "SY-AB2CD3", "subject": "Secret subject"})
	if !ok || !strings.Contains(body, "SY-AB2CD3") || strings.Contains(title+body, "Secret") {
		t.Fatalf("fr: %q %q %v", title, body, ok)
	}
	if strings.Contains(title, "Our team") || strings.Contains(body, "Our support") {
		t.Fatalf("French push is not translated: %q %q", title, body)
	}
	titleES, bodyES, _ := PushText("es", "support.replied", map[string]any{"reference": "SY-AB2CD3"})
	titleEN, bodyEN, _ := PushText("en", "support.replied", map[string]any{"reference": "SY-AB2CD3"})
	if titleES == titleEN || bodyES == bodyEN || titleES == title {
		t.Fatalf("per-locale copy: %q/%q %q/%q", titleES, bodyES, titleEN, bodyEN)
	}
	if _, body, _ := PushText("en", "support.replied", nil); body == "" {
		t.Fatal("missing reference must still produce a body")
	}
	// A hostile reference is cut to one short line.
	_, body, _ = PushText("en", "support.replied", map[string]any{"reference": "SY-1\nBcc: evil@example.com" + strings.Repeat("x", 100)})
	if strings.Contains(body, "\n") || len([]rune(body)) > 180 {
		t.Fatalf("body: %q", body)
	}
}
