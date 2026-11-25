package mail

import (
	"strings"
	"testing"
)

// Every lifecycle nudge variant renders in each language, states the numbers
// it was sent with, and leaves no English sentence behind in French or Spanish.
func TestLifecycleNotificationVariants(t *testing.T) {
	cases := []struct {
		kind    string
		data    map[string]any
		subject string
		number  string // must appear in the first paragraph ("" when none)
	}{
		{"lifecycle.first_reading", map[string]any{"free_minutes": float64(1), "online_advisors": float64(4)}, "Your free minute is waiting", "1"},
		{"lifecycle.first_reading", map[string]any{"free_minutes": float64(5)}, "Your free minutes are waiting", "5"},
		{"lifecycle.first_reading", map[string]any{"free_minutes": float64(0), "online_advisors": float64(3)}, "Advisors are online now", ""},
		{"lifecycle.first_reading", map[string]any{}, "Your first reading is waiting", ""},
		{"lifecycle.invite_friends", nil, "Enjoyed your reading?", ""},
		{"lifecycle.win_back", map[string]any{"online_advisors": float64(1)}, "An advisor is online now", "1"},
		{"lifecycle.win_back", map[string]any{"online_advisors": float64(12)}, "Advisors are online now", "12"},
		{"lifecycle.win_back", map[string]any{"online_advisors": float64(0)}, "Your daily horoscope is ready", ""},
	}
	for _, tc := range cases {
		en, ok := Notification("en", tc.kind, tc.data, "https://staryield.net/en/psychics")
		if !ok || en.Subject != tc.subject || len(en.Paragraphs) != 1 || en.Action == nil || en.Action.URL != "https://staryield.net/en/psychics" {
			t.Fatalf("%s %v: %+v", tc.kind, tc.data, en)
		}
		if tc.number != "" && !strings.Contains(en.Paragraphs[0], tc.number) {
			t.Errorf("%s %v: %q does not state %s", tc.kind, tc.data, en.Paragraphs[0], tc.number)
		}
		for _, loc := range []string{"fr", "es"} {
			got, _ := Notification(loc, tc.kind, tc.data, "")
			want := contentStrings(en)
			for i, s := range contentStrings(got) {
				if want[i] != "" && s == want[i] {
					t.Errorf("%s/%s: untranslated %q", tc.kind, loc, s)
				}
			}
			if tc.number != "" && !strings.Contains(got.Paragraphs[0], tc.number) {
				t.Errorf("%s/%s: %q does not state %s", tc.kind, loc, got.Paragraphs[0], tc.number)
			}
		}
		// Push uses the subject and the first paragraph.
		if title, body, ok := PushText("es", tc.kind, tc.data); !ok || title == "" || body == "" {
			t.Errorf("%s push: %q %q", tc.kind, title, body)
		}
	}
	// The verification reminder has no e-mail or push form of its own (its
	// e-mail is the verification link).
	if _, ok := Notification("en", "lifecycle.verify_reminder", nil, ""); ok {
		t.Fatal("lifecycle.verify_reminder rendered an e-mail")
	}
}

// Promotional e-mail says why it was received (the marketing opt-in) and
// carries the List-Unsubscribe header pointing to the settings.
func TestMarketingFooterNamesTheOptIn(t *testing.T) {
	c, _ := Notification("fr", "lifecycle.invite_friends", nil, "https://staryield.net/fr/profile/invite")
	out, err := Render(c, Layout{Locale: "fr", FrontendURL: "https://staryield.net", Category: "marketing"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Text, "conseils, des offres et des rappels") || strings.Contains(out.Text, "notifications par e-mail sont activées") {
		t.Fatalf("footer: %s", out.Text)
	}
	if !strings.HasPrefix(out.Headers["List-Unsubscribe"], "<https://staryield.net/") {
		t.Fatalf("headers: %v", out.Headers)
	}
	n, _ := Render(c, Layout{Locale: "en", FrontendURL: "https://staryield.net", Category: "notification"})
	if !strings.Contains(n.Text, "email notifications are on") || strings.Contains(n.Text, "tips, offers and reminders") {
		t.Fatalf("notification footer: %s", n.Text)
	}
}
