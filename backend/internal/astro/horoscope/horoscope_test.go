package horoscope

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/astro"
	"github.com/Jihed-khelifi/staryield/backend/internal/astro/tz"
)

var placeholderRE = regexp.MustCompile(`\{[a-z_]+\}`)

// placeholders returns the sorted set of placeholders in s, with the English
// "a_" (indefinite article) variants folded into their base names.
func placeholders(s string) []string {
	set := map[string]bool{}
	for _, p := range placeholderRE.FindAllString(s, -1) {
		set[strings.Replace(p, "{a_", "{", 1)] = true
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

type bank struct {
	name  string
	lists map[string][]string
}

func banksOf(c *Content) []bank {
	one := func(name string, l []string) bank { return bank{name, map[string][]string{"": l}} }
	return []bank{
		one("openers", c.Openers), one("moon_lines", c.MoonLines), {"moon_themes", c.MoonThemes},
		{"phase_meanings", c.PhaseMeanings}, {"element_lines", c.ElementLines}, {"relation_lines", c.RelationLines},
		one("generic_lines", c.GenericLines), one("closers", c.Closers), one("generic_focus", c.GenericFocus),
		{"element_focus", c.ElementFocus}, one("affirmations", c.Affirmations), one("colors", c.Colors),
		one("months", c.Months), one("months_short", c.MonthsShort),
	}
}

// TestContentParity requires every locale to mirror English: same keys, same
// bank lengths and the same placeholders in each aligned sentence, so a
// reading is the same in every language.
func TestContentParity(t *testing.T) {
	en, err := ContentFor("en")
	if err != nil {
		t.Fatal(err)
	}
	vocab := func(c *Content) map[string]map[string]string {
		return map[string]map[string]string{"signs": c.Signs, "elements": c.Elements, "phases": c.Phases, "phase_names": c.PhaseNames,
			"phase_nouns": c.PhaseNouns, "moods": c.Moods, "compatibility_labels": c.CompatibilityLabels}
	}
	for _, l := range Locales {
		c, err := ContentFor(l)
		if err != nil || c.Locale != l {
			t.Fatalf("%s: %v (%q)", l, err, c.Locale)
		}
		for name, m := range vocab(c) {
			for k := range vocab(en)[name] {
				if strings.TrimSpace(m[k]) == "" {
					t.Errorf("%s: %s[%q] missing", l, name, k)
				}
			}
			if len(m) != len(vocab(en)[name]) {
				t.Errorf("%s: %s has %d entries, English %d", l, name, len(m), len(vocab(en)[name]))
			}
		}
		enBanks := banksOf(en)
		for i, b := range banksOf(c) {
			for key, list := range enBanks[i].lists {
				got := b.lists[key]
				if len(got) != len(list) {
					t.Errorf("%s %s[%s]: %d entries, English %d", l, b.name, key, len(got), len(list))
					continue
				}
				for j := range list {
					if strings.TrimSpace(got[j]) == "" {
						t.Errorf("%s %s[%s][%d] empty", l, b.name, key, j)
					}
					if pe, pl := placeholders(list[j]), placeholders(got[j]); !reflect.DeepEqual(pe, pl) {
						t.Errorf("%s %s[%s][%d]: placeholders %v, English %v", l, b.name, key, j, pl, pe)
					}
				}
			}
		}
	}
	if len(colorKeys) != len(en.Colors) {
		t.Fatalf("colorKeys %d vs %d colors", len(colorKeys), len(en.Colors))
	}
	for i, k := range colorKeys {
		if en.Colors[i] != k {
			t.Errorf("color %d: key %q vs English %q", i, k, en.Colors[i])
		}
	}
}

// TestVoiceRules checks the brand voice mechanically: one sentence per
// entry, ending with a single ".", "?" or "!" (French puts a non-breaking
// space before "?"; Spanish questions open with "¿"), no words of fear,
// certainty or advice.
func TestVoiceRules(t *testing.T) {
	forbidden := map[string][]string{
		"en": {"must ", "will definitely", "guarantee", "danger", "warning", "doom", "never fails", "destiny is", "diagnos", "invest "},
		"fr": {"il faut", "vous devez", "garanti", "danger", "malheur", "destin est", "diagnost", "investiss"},
		"es": {"debe ", "tiene que", "garantiza", "peligro", "desgracia", "destino es", "diagnóst", "invierta"},
	}
	for _, l := range Locales {
		c, _ := ContentFor(l)
		sentences := append([]string{}, c.Openers...)
		for _, lists := range []map[string][]string{c.ElementLines, c.RelationLines} {
			for _, list := range lists {
				sentences = append(sentences, list...)
			}
		}
		sentences = append(sentences, c.MoonLines...)
		sentences = append(sentences, c.GenericLines...)
		sentences = append(sentences, c.Closers...)
		sentences = append(sentences, c.Affirmations...)
		questions := append([]string{}, c.GenericFocus...)
		for _, list := range c.ElementFocus {
			questions = append(questions, list...)
		}
		check := func(s string, question bool) {
			trimmed := strings.TrimRight(s, ".?!")
			if strings.ContainsAny(strings.ReplaceAll(trimmed, "...", ""), ".?!") {
				t.Errorf("%s: more than one sentence: %q", l, s)
			}
			switch {
			case question && !strings.HasSuffix(s, "?"):
				t.Errorf("%s: question without '?': %q", l, s)
			case question && l == "fr" && !strings.HasSuffix(s, " ?"):
				t.Errorf("fr: '?' needs a non-breaking space before it: %q", s)
			case question && l == "es" && !strings.HasPrefix(s, "¿"):
				t.Errorf("es: question must open with '¿': %q", s)
			case !question && !strings.HasSuffix(s, "."):
				t.Errorf("%s: sentence must end with '.': %q", l, s)
			}
			low := strings.ToLower(s)
			for _, f := range forbidden[l] {
				if strings.Contains(low, f) {
					t.Errorf("%s: forbidden %q in %q", l, f, s)
				}
			}
		}
		for _, s := range sentences {
			check(s, false)
		}
		for _, q := range questions {
			check(q, true)
		}
	}
}

func TestDailyDeterministicAndAligned(t *testing.T) {
	date := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	readings := map[string]Reading{}
	for _, l := range Locales {
		r1, err := Daily(Request{Sign: "leo", Date: date, Locale: l})
		if err != nil {
			t.Fatal(err)
		}
		r2, _ := Daily(Request{Sign: "LEO", Date: date.Add(5 * time.Hour), Locale: l})
		if !reflect.DeepEqual(r1, r2) {
			t.Fatalf("%s: same sign and day differ", l)
		}
		readings[l] = r1
	}
	en, fr, es := readings["en"], readings["fr"], readings["es"]
	// Canonical fields are identical across languages; text is translated.
	for _, r := range []Reading{fr, es} {
		if r.Sign != en.Sign || r.MoodKey != en.MoodKey || r.ColorKey != en.ColorKey || r.LuckyNumber != en.LuckyNumber ||
			r.LuckyHour != en.LuckyHour || r.CompatibilityScore != en.CompatibilityScore || r.CompatibleSign != en.CompatibleSign ||
			r.MoonSign != en.MoonSign || r.MoonPhase != en.MoonPhase || r.DateRange.Start != en.DateRange.Start {
			t.Errorf("%s reading diverges from English: %+v vs %+v", r.Locale, r, en)
		}
		if r.Text == en.Text || r.Focus == en.Focus || r.Mood == en.Mood && r.Locale != "en" && en.Mood != "Calm" {
			t.Errorf("%s: text not translated", r.Locale)
		}
		if strings.Contains(r.Text, "{") {
			t.Errorf("%s: unfilled placeholder in %q", r.Locale, r.Text)
		}
	}
	if fr.SignName != "Lion" || es.SignName != "Leo" || en.SignName != "Leo" {
		t.Errorf("sign names %q %q %q", en.SignName, fr.SignName, es.SignName)
	}
	if en.Compatibility != en.CompatibilityScore || en.CompatibilityScore < 55 || en.CompatibilityScore > 98 {
		t.Errorf("compatibility %d / %d", en.Compatibility, en.CompatibilityScore)
	}
	if en.CompatibleSign == "Leo" || astro.ElementOf(en.CompatibleSign) == "" {
		t.Errorf("compatible sign %q", en.CompatibleSign)
	}
	if en.CurrentDate != "November 4, 2026" || fr.CurrentDate != "4 novembre 2026" || es.CurrentDate != "4 de noviembre de 2026" {
		t.Errorf("current dates %q %q %q", en.CurrentDate, fr.CurrentDate, es.CurrentDate)
	}
	if en.Provider != "staryield" || en.EngineVersion != EngineVersion || en.TZ != "UTC" {
		t.Errorf("metadata %+v", en)
	}
}

func TestDailyVarietyAndRules(t *testing.T) {
	texts := map[string]bool{}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 60; i++ {
		for _, s := range astro.Signs {
			r, err := Daily(Request{Sign: s, Date: start.AddDate(0, 0, i), Locale: "fr"})
			if err != nil {
				t.Fatal(err)
			}
			texts[r.Text] = true
			if r.LuckyHour < 6 || r.LuckyHour > 21 || r.LuckyNumber < 1 || r.LuckyNumber > 99 {
				t.Fatalf("lucky values %d %d", r.LuckyNumber, r.LuckyHour)
			}
			if r.CompatibilityLabel == "" || r.Color == "" || r.Mood == "" || r.MoonPhaseName == "" || r.DateRange.Label == "" {
				t.Fatalf("empty localized field: %+v", r)
			}
			if r.CompatibleSign == r.Sign {
				t.Fatalf("%s compatible with itself", r.Sign)
			}
		}
	}
	if len(texts) < 600 {
		t.Errorf("only %d distinct readings across 720", len(texts))
	}
}

// TestRequesterTimeZone: at the same instant, a reader in Kiritimati (UTC+14)
// and one in Pago Pago (UTC−11) are on different calendar days.
func TestRequesterTimeZone(t *testing.T) {
	instant := time.Date(2026, 11, 30, 10, 30, 0, 0, time.UTC)
	kir, _ := tz.Load("Pacific/Kiritimati")
	ppg, _ := tz.Load("Pacific/Pago_Pago")
	a, err := Daily(Request{Sign: "aries", Date: instant.In(kir), Location: kir})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Daily(Request{Sign: "aries", Date: instant.In(ppg), Location: ppg})
	if a.Date != "2026-12-01" || b.Date != "2026-11-29" || a.TZ != "Pacific/Kiritimati" || b.TZ != "Pacific/Pago_Pago" {
		t.Errorf("dates %s (%s) / %s (%s)", a.Date, a.TZ, b.Date, b.TZ)
	}
	if a.Text == b.Text {
		t.Error("different days produced the same reading")
	}
}

func TestDailyErrorsAndLocales(t *testing.T) {
	if _, err := Daily(Request{Sign: "ophiuchus", Date: time.Now()}); !errors.Is(err, ErrUnknownSign) {
		t.Errorf("unknown sign: %v", err)
	}
	r, err := Daily(Request{Sign: "virgo", Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Locale: "de-DE"})
	if err != nil || r.Locale != "en" {
		t.Errorf("unsupported locale falls back to English: %q %v", r.Locale, err)
	}
	for in, want := range map[string]string{"fr-FR": "fr", "es_419": "es", "EN": "en", "": "en", "x": "en", "FR": "fr"} {
		if got := NormalizeLocale(in); got != want {
			t.Errorf("NormalizeLocale(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatting(t *testing.T) {
	hours := map[int][3]string{0: {"12am", "0 h", "0:00"}, 9: {"9am", "9 h", "9:00"}, 12: {"12pm", "12 h", "12:00"}, 15: {"3pm", "15 h", "15:00"}, 21: {"9pm", "21 h", "21:00"}}
	for h, want := range hours {
		for i, l := range Locales {
			if got := FormatHour(l, h); got != want[i] {
				t.Errorf("FormatHour(%s, %d) = %q, want %q", l, h, got, want[i])
			}
		}
	}
	if got := FormatDate("fr", 2026, time.March, 1); got != "1er mars 2026" {
		t.Errorf("French first of month: %q", got)
	}
	r, _ := Daily(Request{Sign: "aries", Date: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Locale: "en"})
	if r.DateRange.Label != "Mar 21 - Apr 19" || r.DateRange.Start != "2026-03-21" || r.DateRange.End != "2026-04-19" {
		t.Errorf("Aries 2026 range %+v", r.DateRange)
	}
	rf, _ := Daily(Request{Sign: "capricorn", Date: time.Date(2027, 1, 10, 0, 0, 0, 0, time.UTC), Locale: "fr"})
	if rf.DateRange.Label != "22 déc. - 19 janv." {
		t.Errorf("Capricorn (January) range %q", rf.DateRange.Label)
	}
}

func TestAztroShapeAndProviders(t *testing.T) {
	ctx := context.Background()
	date := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	var p Provider = Feed{} // unconfigured feed delegates to the engine
	if p.Name() != "staryield" {
		t.Errorf("unconfigured feed name %q", p.Name())
	}
	r, err := p.Daily(ctx, Request{Sign: "Gemini", Date: date, Locale: "es"})
	if err != nil {
		t.Fatal(err)
	}
	a := ToAztro(r)
	raw, _ := json.Marshal(a)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"date_range", "current_date", "description", "compatibility", "mood", "color", "lucky_number", "lucky_time", "staryield"} {
		v, ok := m[k]
		if !ok {
			t.Errorf("aztro field %q missing", k)
			continue
		}
		if s, isStr := v.(string); isStr && (s == "" || s != strings.TrimSpace(s)) {
			t.Errorf("aztro field %q = %q", k, s)
		}
	}
	if a.LuckyNumber != "" && a.LuckyNumber[0] == ' ' || a.Compatibility != r.CompatibleSignName || a.Description != r.Text {
		t.Errorf("aztro mapping %+v", a)
	}
	if a.Staryield.Locale != "es" || a.Staryield.Sign != "Gemini" || a.Staryield.EngineVersion != EngineVersion {
		t.Errorf("meta %+v", a.Staryield)
	}
	if (Feed{Endpoint: "https://feed.example"}).Name() != "feed" {
		t.Error("configured feed name")
	}
	for in, want := range map[string]int{"": 0, "today": 0, "TODAY": 0, "yesterday": -1, "tomorrow": 1} {
		if got, ok := ParseDay(in); !ok || got != want {
			t.Errorf("ParseDay(%q) = %d, %v", in, got, ok)
		}
	}
	if _, ok := ParseDay("next week"); ok {
		t.Error("ParseDay accepted junk")
	}
}

func FuzzParseDay(f *testing.F) {
	for _, s := range []string{"today", "Yesterday", " tomorrow ", "", "\x00", "todayy"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if off, ok := ParseDay(s); ok && (off < -1 || off > 1) {
			t.Fatalf("ParseDay(%q) = %d", s, off)
		}
	})
}

func BenchmarkDaily(b *testing.B) {
	date := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	for b.Loop() {
		if _, err := Daily(Request{Sign: "scorpio", Date: date, Locale: "fr"}); err != nil {
			b.Fatal(err)
		}
	}
}
