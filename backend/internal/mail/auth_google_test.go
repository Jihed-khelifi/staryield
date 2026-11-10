package mail

import (
	"strings"
	"testing"
)

func TestGoogleSecurityEmailsAreTranslatedAndMasked(t *testing.T) {
	builders := map[string]func(locale string) Content{
		"google_linked": func(l string) Content {
			return GoogleLinked(l, "j•••@gmail.com", "https://staryield.net/"+l+"/profile/settings#account")
		},
		"google_unlinked": func(l string) Content {
			return GoogleUnlinked(l, "https://staryield.net/"+l+"/profile/settings#account")
		},
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
			c := build(loc)
			if c.Action == nil || !strings.Contains(c.Action.URL, "/"+loc+"/profile/settings") {
				t.Errorf("%s/%s: action %+v", name, loc, c.Action)
			}
		}
		if c := build("en"); c.Subject == "" || c.Heading == "" || c.Preheader == "" || len(c.Paragraphs) == 0 || c.Note == "" {
			t.Errorf("%s: incomplete content %+v", name, c)
		}
	}
	for _, loc := range []string{"en", "fr", "es"} {
		c := GoogleLinked(loc, "j•••@gmail.com", "https://x/y")
		if !strings.Contains(c.Paragraphs[0], "j•••@gmail.com") || strings.Contains(c.Paragraphs[0], "%s") {
			t.Errorf("%s: the masked address must be named: %q", loc, c.Paragraphs[0])
		}
	}
}
