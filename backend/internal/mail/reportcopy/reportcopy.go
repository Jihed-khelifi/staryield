// Package reportcopy is the copy of the calculators' result views that the
// e-mailed report prints (revisions 6.0, §26.4): sign insights and facts,
// tarot names and house keywords, life path portraits, headings and sentence
// templates, in English with their French and Spanish translations.
//
// The data is generated from the web app's sources (npm run gen:report-copy in
// frontend/) into report-copy.json, which this package embeds; a frontend test
// fails when the committed file is stale, and this package's tests fail when
// the file is unreadable or a label is missing.
package reportcopy

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

//go:embed report-copy.json
var raw []byte

// Sign is one zodiac sign's copy.
type Sign struct {
	Name    string `json:"name"`
	Element string `json:"element"`
	Ruler   string `json:"ruler"`
	Insight string `json:"insight"`
}

// House is one house's label ("First house") and one-word theme ("Self").
type House struct {
	Label   string `json:"label"`
	Keyword string `json:"keyword"`
}

// Portrait is a life path's title and description.
type Portrait struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// Number is one of the three number cards of the path of life result.
type Number struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Body  string `json:"body"`
}

// Copy is the generated copy.
type Copy struct {
	Version      int                          `json:"version"`
	Labels       map[string]string            `json:"labels"`
	Signs        []Sign                       `json:"signs"`
	Tarot        map[string]string            `json:"tarot"`
	Houses       []House                      `json:"houses"`
	LifePath     map[string]Portrait          `json:"life_path"`
	Numbers      []Number                     `json:"numbers"`
	HouseSystems map[string]string            `json:"house_systems"`
	Bodies       []string                     `json:"bodies"`
	Elements     []string                     `json:"elements"`
	Modalities   []string                     `json:"modalities"`
	Translations map[string]map[string]string `json:"translations"`

	lower map[string]map[string]string // locale → lower-cased normalised English → translation
}

var (
	once   sync.Once
	loaded *Copy
	err    error
)

// Get returns the embedded copy (parsed once).
func Get() (*Copy, error) {
	once.Do(func() { loaded, err = parse(raw) })
	return loaded, err
}

// Must returns the embedded copy and panics when it is unreadable (a build
// defect caught by the package tests).
func Must() *Copy {
	c, err := Get()
	if err != nil {
		panic(err)
	}
	return c
}

func parse(b []byte) (*Copy, error) {
	var c Copy
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("reportcopy: report-copy.json: %w", err)
	}
	if c.Version != 1 || len(c.Signs) != 12 || len(c.Houses) != 12 || len(c.Tarot) == 0 || len(c.Labels) == 0 {
		return nil, fmt.Errorf("reportcopy: report-copy.json is incomplete (regenerate it with npm run gen:report-copy)")
	}
	c.lower = map[string]map[string]string{}
	for locale, m := range c.Translations {
		c.lower[locale] = make(map[string]string, len(m))
		for en, tr := range m {
			c.lower[locale][strings.ToLower(normalize(en))] = tr
		}
	}
	return &c, nil
}

var spaces = regexp.MustCompile(`\s+`)

func normalize(s string) string { return spaces.ReplaceAllString(strings.TrimSpace(s), " ") }

// T translates an English source string of the copy; English and unknown
// strings are returned unchanged.
func (c *Copy) T(locale, english string) string {
	if locale == "" || locale == "en" {
		return english
	}
	if tr, ok := c.lower[locale][strings.ToLower(normalize(english))]; ok {
		return tr
	}
	return english
}

var placeholder = regexp.MustCompile(`\{value(\d+)\}`)

// F translates a template ("{value0} in {value1}") and fills its
// placeholders with values (already translated by the caller when needed).
func (c *Copy) F(locale, english string, values ...string) string {
	return placeholder.ReplaceAllStringFunc(c.T(locale, english), func(m string) string {
		var i int
		fmt.Sscanf(m, "{value%d}", &i)
		if i >= 0 && i < len(values) {
			return values[i]
		}
		return m
	})
}

// Label returns the translated label of a key of REPORT_LABELS ("" when the
// key is unknown, which the package tests rule out for the keys the renderer
// uses).
func (c *Copy) Label(locale, key string) string {
	en, ok := c.Labels[key]
	if !ok {
		return ""
	}
	return c.T(locale, en)
}

// LabelF is Label for a template label.
func (c *Copy) LabelF(locale, key string, values ...string) string {
	en, ok := c.Labels[key]
	if !ok {
		return ""
	}
	return c.F(locale, en, values...)
}

// SignByName returns a sign by its (case-insensitive) English name.
func (c *Copy) SignByName(name string) (Sign, bool) {
	for _, s := range c.Signs {
		if strings.EqualFold(s.Name, strings.TrimSpace(name)) {
			return s, true
		}
	}
	return Sign{}, false
}

// Portrait returns the copy of a life path number (master numbers included).
func (c *Copy) Portrait(number int) (Portrait, bool) {
	p, ok := c.LifePath[fmt.Sprint(number)]
	return p, ok
}
