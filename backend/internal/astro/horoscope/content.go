package horoscope

import (
	"embed"
	"encoding/json"
	"fmt"
	"sync"
)

// Sentence banks and vocabulary, one file per locale. Every bank is
// index-aligned across locales (same lengths, same placeholders), so a given
// sign and date produce the same reading in every language.
//
// Voice rules (Staryield brand guide): compassionate, clear and
// non-deterministic. Describe perspective, not destiny; preserve the reader's
// agency; no fear, urgency, judgment or claims of certainty; no medical,
// legal or financial advice; gender-neutral wording in French and Spanish.
//
//go:embed content/*.json
var contentFS embed.FS

// Locales lists the supported content languages; English is the default.
var Locales = []string{"en", "fr", "es"}

// Content is one locale's vocabulary and sentence banks.
type Content struct {
	Locale              string            `json:"locale"`
	Signs               map[string]string `json:"signs"`
	Elements            map[string]string `json:"elements"`
	ElementsWithArticle map[string]string `json:"elements_with_article"`
	Phases              map[string]string `json:"phases"`
	PhaseNames          map[string]string `json:"phase_names"`
	PhaseNouns          map[string]string `json:"phase_nouns"`
	Moods               map[string]string `json:"moods"`
	CompatibilityLabels map[string]string `json:"compatibility_labels"`
	Colors              []string          `json:"colors"`
	Months              []string          `json:"months"`
	MonthsShort         []string          `json:"months_short"`

	Openers       []string            `json:"openers"`
	MoonLines     []string            `json:"moon_lines"`
	MoonThemes    map[string][]string `json:"moon_themes"`
	PhaseMeanings map[string][]string `json:"phase_meanings"`
	ElementLines  map[string][]string `json:"element_lines"`
	RelationLines map[string][]string `json:"relation_lines"`
	GenericLines  []string            `json:"generic_lines"`
	Closers       []string            `json:"closers"`
	GenericFocus  []string            `json:"generic_focus"`
	ElementFocus  map[string][]string `json:"element_focus"`
	Affirmations  []string            `json:"affirmations"`
}

var (
	contentOnce sync.Once
	contents    map[string]*Content
	contentErr  error
)

func loadContent() (map[string]*Content, error) {
	contentOnce.Do(func() {
		contents = map[string]*Content{}
		for _, l := range Locales {
			raw, err := contentFS.ReadFile("content/" + l + ".json")
			if err != nil {
				contentErr = err
				return
			}
			var c Content
			if err := json.Unmarshal(raw, &c); err != nil {
				contentErr = fmt.Errorf("horoscope: %s.json: %w", l, err)
				return
			}
			contents[l] = &c
		}
	})
	return contents, contentErr
}

// ContentFor returns the content for locale, falling back to English for
// unsupported locales.
func ContentFor(locale string) (*Content, error) {
	all, err := loadContent()
	if err != nil {
		return nil, err
	}
	if c, ok := all[locale]; ok {
		return c, nil
	}
	return all["en"], nil
}

// NormalizeLocale maps "fr-FR", "es_419" … to a supported locale, or "en".
func NormalizeLocale(locale string) string {
	if len(locale) >= 2 {
		l := locale[:2]
		for _, s := range Locales {
			if s == l || s == lower(l) {
				return s
			}
		}
	}
	return "en"
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
