package reportcopy

import (
	"strings"
	"testing"
)

func TestEmbeddedCopyIsComplete(t *testing.T) {
	c, err := Get()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Signs) != 12 || len(c.Houses) != 12 || len(c.Tarot) != 22 || len(c.Numbers) != 3 {
		t.Fatalf("signs %d, houses %d, tarot %d, numbers %d", len(c.Signs), len(c.Houses), len(c.Tarot), len(c.Numbers))
	}
	for _, n := range []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "11", "22", "33"} {
		if p, ok := c.LifePath[n]; !ok || p.Title == "" || p.Body == "" {
			t.Errorf("life path %s has no copy", n)
		}
	}
	// Every English string has a French and a Spanish translation.
	for _, locale := range []string{"fr", "es"} {
		if len(c.Translations[locale]) < 100 {
			t.Errorf("%s: only %d translations", locale, len(c.Translations[locale]))
		}
		for en, tr := range c.Translations[locale] {
			if strings.TrimSpace(tr) == "" {
				t.Errorf("%s: empty translation of %q", locale, en)
			}
		}
	}
}

func TestTranslateAndFormat(t *testing.T) {
	c := Must()
	if got := c.T("fr", "Your spread"); got != "Votre tirage" {
		t.Fatalf("fr: %q", got)
	}
	if got := c.T("fr", "  your   SPREAD "); got != "Votre tirage" {
		t.Fatalf("normalised lookup: %q", got)
	}
	if got := c.T("es", "Reversed"); got != "Invertida" {
		t.Fatalf("es: %q", got)
	}
	if got := c.T("en", "Your spread"); got != "Your spread" {
		t.Fatalf("en is the source: %q", got)
	}
	if got := c.T("fr", "A sentence nobody translated"); got != "A sentence nobody translated" {
		t.Fatalf("unknown strings stay English: %q", got)
	}
	if got := c.F("en", "{value0} in {value1}, in the {value2}", "Sun", "Leo", "tenth house"); got != "Sun in Leo, in the tenth house" {
		t.Fatalf("format: %q", got)
	}
	if got := c.F("en", "{value0} {value3}", "a"); got != "a {value3}" {
		t.Fatalf("missing values stay as placeholders: %q", got)
	}
	if c.Label("en", "nope") != "" || c.LabelF("fr", "nope") != "" {
		t.Fatal("unknown label")
	}
	if s, ok := c.SignByName(" leo "); !ok || s.Name != "Leo" || s.Ruler != "Sun" {
		t.Fatalf("sign: %+v %v", s, ok)
	}
	if _, ok := c.Portrait(11); !ok {
		t.Fatal("master number portrait")
	}
}

func TestParseRefusesAnIncompleteFile(t *testing.T) {
	for _, bad := range []string{"", "{}", `{"version":2}`, "not json"} {
		if _, err := parse([]byte(bad)); err == nil {
			t.Errorf("parse(%q) accepted", bad)
		}
	}
}
