package mail

import (
	"strings"
	"testing"
)

func TestEmailVerificationIsTranslatedAndCarriesTheLink(t *testing.T) {
	const link = "https://staryield.test/en/verify-email#token=abc"
	en := EmailVerification("en", link, 48)
	if en.Action == nil || en.Action.URL != link || !strings.Contains(strings.Join(en.Paragraphs, " "), "48 hours") {
		t.Fatalf("english content: %+v", en)
	}
	for _, loc := range []string{"fr", "es"} {
		c := EmailVerification(loc, link, 48)
		if c.Action == nil || c.Action.URL != link || !strings.Contains(strings.Join(c.Paragraphs, " "), "48") {
			t.Fatalf("%s: %+v", loc, c)
		}
		for _, s := range contentStrings(c) {
			for _, e := range contentStrings(en) {
				if s == e && s != "" {
					t.Errorf("%s: %q is not translated", loc, s)
				}
			}
		}
	}
}
