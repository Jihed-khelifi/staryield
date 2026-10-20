package astro

import (
	"errors"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"
)

// bannedPhrases must never appear in generated copy: they break the brand
// voice (certainty, fear, urgency, judgment) or stray into advice the product
// does not give.
var bannedPhrases = []string{
	"will happen", "you will", "act now", "too late", "before it is too late",
	"guarantee", "danger", "destiny", "doom", "disaster", "inevitable",
	"certainly", "definitely", "warning", "beware", "punish", "bad luck",
	"diagnos", "medication", "invest", "lawsuit", "lawyer",
}

// bannedWords are matched as whole words only.
var bannedWords = regexp.MustCompile(`\b(must|should|fate|curse|cursed|never|fear|urgent|hurry up)\b`)

var articleMismatch = regexp.MustCompile(`\ba [aeiou]`)

func bannedIn(text string) []string {
	lower := strings.ToLower(text)
	var found []string
	for _, p := range bannedPhrases {
		if strings.Contains(lower, p) {
			found = append(found, p)
		}
	}
	return append(found, bannedWords.FindAllString(lower, -1)...)
}

// allBankStrings returns every raw string the generator can draw from.
func allBankStrings() []string {
	var all []string
	all = append(all, openers...)
	all = append(all, moonLines...)
	all = append(all, genericLines...)
	all = append(all, closers...)
	all = append(all, genericFocus...)
	all = append(all, affirmations...)
	for _, m := range []map[string][]string{moonThemes, phaseMeanings, elementLines, relationLines, elementFocus} {
		for _, v := range m {
			all = append(all, v...)
		}
	}
	return all
}

func TestContentBanksFollowVoiceRules(t *testing.T) {
	for _, s := range allBankStrings() {
		if found := bannedIn(s); len(found) > 0 {
			t.Errorf("banned %v in %q", found, s)
		}
	}
	minimums := map[string]int{
		"openers": len(openers), "moonLines": len(moonLines), "genericLines": len(genericLines),
		"closers": len(closers), "genericFocus": len(genericFocus), "affirmations": len(affirmations),
	}
	for name, n := range minimums {
		if n < 8 {
			t.Errorf("%s has %d options, want at least 8", name, n)
		}
	}
	for _, el := range Elements {
		if len(elementLines[el]) < 8 {
			t.Errorf("elementLines[%s] has %d options, want at least 8", el, len(elementLines[el]))
		}
		if len(elementFocus[el]) == 0 {
			t.Errorf("elementFocus[%s] is empty", el)
		}
	}
	for _, s := range Signs {
		if len(moonThemes[s]) == 0 {
			t.Errorf("no moon theme for %s", s)
		}
	}
	for _, p := range PhaseNames {
		if len(phaseMeanings[p]) == 0 {
			t.Errorf("no phase meaning for %s", p)
		}
		if _, ok := phaseMoods[p]; !ok {
			t.Errorf("no moods for %s", p)
		}
	}
	for _, r := range []string{relationSame, relationSupportive, relationContrast} {
		if len(relationLines[r]) == 0 {
			t.Errorf("no relation lines for %s", r)
		}
	}
	// Complete sentences in the banks must contain exactly one terminator.
	for _, bank := range [][]string{openers, moonLines, genericLines, closers, genericFocus, affirmations} {
		for _, s := range bank {
			if n := countSentences(s); n != 1 {
				t.Errorf("%q counts as %d sentences", s, n)
			}
		}
	}
}

var sentenceEnd = regexp.MustCompile(`[.?!](\s|$)`)

func countSentences(text string) int { return len(sentenceEnd.FindAllStringIndex(text, -1)) }

func TestDailyIsDeterministic(t *testing.T) {
	date := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	a, err := Daily("Leo", date)
	if err != nil {
		t.Fatal(err)
	}
	for _, variant := range []struct {
		sign string
		date time.Time
	}{
		{"Leo", date},
		{"leo", date},
		{"  LEO ", date},
		{"Leo", date.Add(23 * time.Hour)},
		{"Leo", time.Date(2026, 10, 20, 23, 30, 0, 0, time.FixedZone("UTC+14", 14*3600))},
	} {
		b, err := Daily(variant.sign, variant.date)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(a, b) {
			t.Errorf("Daily(%q, %s) differs:\n%+v\n%+v", variant.sign, variant.date, a, b)
		}
	}
	if a.Sign != "Leo" || a.Date != "2026-10-20" {
		t.Errorf("sign/date = %s/%s", a.Sign, a.Date)
	}
	next, _ := Daily("Leo", date.AddDate(0, 0, 1))
	if next.Text == a.Text {
		t.Error("consecutive days produced identical text")
	}
	t.Logf("sample: %+v", a)
}

func TestDailyVariety(t *testing.T) {
	date := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	texts := map[string]bool{}
	for _, s := range Signs {
		h, err := Daily(s, date)
		if err != nil {
			t.Fatal(err)
		}
		texts[h.Text] = true
	}
	if len(texts) < 10 {
		t.Errorf("only %d distinct texts across 12 signs on one day", len(texts))
	}
	texts = map[string]bool{}
	for i := range 30 {
		h, _ := Daily("Pisces", date.AddDate(0, 0, i))
		texts[h.Text] = true
	}
	if len(texts) < 28 {
		t.Errorf("only %d distinct texts across 30 days", len(texts))
	}
}

func TestDailyContentRules(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	moods := map[string]int{}
	phases := map[string]int{}
	labels := map[string]int{}
	lengths := map[int]int{}
	for i := range 400 {
		date := start.AddDate(0, 0, i)
		noon := time.Date(date.Year(), date.Month(), date.Day(), 12, 0, 0, 0, time.UTC)
		wantMoon := SignForLongitude(EclipticLongitude(Moon, noon))
		wantPhase := PhaseName(noon)
		for _, sign := range Signs {
			h, err := Daily(sign, date)
			if err != nil {
				t.Fatal(err)
			}
			ctx := sign + " " + h.Date
			if h.MoonSign != wantMoon || h.MoonPhase != wantPhase {
				t.Fatalf("%s: moon %s/%s, want %s/%s", ctx, h.MoonSign, h.MoonPhase, wantMoon, wantPhase)
			}
			if !strings.Contains(h.Text, h.MoonSign) || !strings.Contains(h.Text, h.MoonPhase) {
				t.Fatalf("%s: text does not mention %s and %s: %s", ctx, h.MoonSign, h.MoonPhase, h.Text)
			}
			n := countSentences(h.Text)
			if n < 4 || n > 6 {
				t.Fatalf("%s: %d sentences: %s", ctx, n, h.Text)
			}
			lengths[n]++
			if countSentences(h.Focus) != 1 || countSentences(h.Affirmation) != 1 {
				t.Fatalf("%s: focus/affirmation not a single sentence: %q / %q", ctx, h.Focus, h.Affirmation)
			}
			for _, field := range []string{h.Text, h.Focus, h.Affirmation} {
				if strings.ContainsAny(field, "{}") || strings.Contains(field, "  ") {
					t.Fatalf("%s: unfilled placeholder or double space: %q", ctx, field)
				}
				if found := bannedIn(field); len(found) > 0 {
					t.Fatalf("%s: banned %v in %q", ctx, found, field)
				}
				if r := []rune(field); !unicode.IsUpper(r[0]) {
					t.Fatalf("%s: does not start with a capital: %q", ctx, field)
				}
			}
			if articleMismatch.MatchString(strings.ToLower(h.Text)) {
				t.Fatalf("%s: article mismatch in %q", ctx, h.Text)
			}
			if !slices.Contains(Moods[:], h.Mood) {
				t.Fatalf("%s: mood %q", ctx, h.Mood)
			}
			if h.LuckyNumber < 1 || h.LuckyNumber > 99 {
				t.Fatalf("%s: lucky number %d", ctx, h.LuckyNumber)
			}
			if h.Compatibility < 55 || h.Compatibility > 98 {
				t.Fatalf("%s: compatibility %d", ctx, h.Compatibility)
			}
			if h.CompatibilityLabel != compatibilityLabel(h.Compatibility) {
				t.Fatalf("%s: label %q for %d", ctx, h.CompatibilityLabel, h.Compatibility)
			}
			moods[h.Mood]++
			phases[h.MoonPhase]++
			labels[h.CompatibilityLabel]++
		}
	}
	for _, m := range Moods {
		if moods[m] == 0 {
			t.Errorf("mood %s never produced", m)
		}
	}
	for _, p := range PhaseNames {
		if phases[p] == 0 {
			t.Errorf("phase %s never produced", p)
		}
	}
	for _, l := range []string{CompatibilityHigh, CompatibilityBalanced, CompatibilityGentle} {
		if labels[l] == 0 {
			t.Errorf("label %s never produced", l)
		}
	}
	for n := 4; n <= 6; n++ {
		if lengths[n] == 0 {
			t.Errorf("no readings with %d sentences", n)
		}
	}
}

func TestDailyUnknownSign(t *testing.T) {
	for _, sign := range []string{"", "Ophiuchus", "Leo Minor"} {
		_, err := Daily(sign, time.Now())
		if !errors.Is(err, ErrUnknownSign) {
			t.Errorf("Daily(%q) err = %v, want ErrUnknownSign", sign, err)
		}
	}
}

func TestCompatibilityLabel(t *testing.T) {
	tests := map[int]string{55: "Gentle", 64: "Gentle", 65: "Balanced", 79: "Balanced", 80: "High", 98: "High"}
	for score, want := range tests {
		if got := compatibilityLabel(score); got != want {
			t.Errorf("compatibilityLabel(%d) = %s, want %s", score, got, want)
		}
	}
	for _, r := range []string{relationSame, relationSupportive, relationContrast} {
		lo, hi := compatibilityRange(r)
		if lo < 55 || hi > 98 || lo > hi {
			t.Errorf("range for %s = %d..%d", r, lo, hi)
		}
	}
	if elementRelation(Fire, Air) != relationSupportive || elementRelation(Water, Earth) != relationSupportive ||
		elementRelation(Fire, Water) != relationContrast || elementRelation(Air, Air) != relationSame {
		t.Error("elementRelation")
	}
}

func TestPickerStableSequence(t *testing.T) {
	// Pin the generator (FNV-1a seed + SplitMix64) so that stored or cached
	// readings never silently change between releases.
	p := newPicker("Leo|2026-10-20")
	want := []uint64{6964600128645009152, 13223104406513115158, 17301364396308490384}
	for i, w := range want {
		if got := p.next(); got != w {
			t.Errorf("value %d = %d, want %d", i, got, w)
		}
	}
	if newPicker("Leo|2026-10-20").next() == newPicker("Leo|2026-10-21").next() {
		t.Error("different seeds produced the same first value")
	}
	for range 1000 {
		if n := p.intn(7); n < 0 || n >= 7 {
			t.Fatalf("intn(7) = %d", n)
		}
	}
}
