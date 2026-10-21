// Package i18n translates user-facing server messages (error messages,
// validation field messages, realtime error frames) into the caller's
// language. English source strings are the catalogue keys, so services keep
// writing plain English; translation happens once, at the transport edge.
//
// Keys may contain %d / %s verbs for messages built with fmt.Sprintf; the
// captured values are re-inserted into the translation (and %s values are
// themselves translated when the catalogue knows them, e.g. resource names
// in "%s not found.").
package i18n

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Supported locales. English is the source language.
var Supported = []string{"en", "fr", "es"}

var catalogs = map[string]map[string]string{"fr": fr, "es": es}

type template struct {
	key   string
	re    *regexp.Regexp
	verbs []byte // 'd' or 's' per capture, in order
}

var templates = buildTemplates()

func buildTemplates() []template {
	seen := map[string]bool{}
	var out []template
	for _, cat := range catalogs {
		for key := range cat {
			if seen[key] || !strings.Contains(key, "%") {
				continue
			}
			seen[key] = true
			var pattern strings.Builder
			var verbs []byte
			pattern.WriteString("^")
			for i := 0; i < len(key); i++ {
				if key[i] == '%' && i+1 < len(key) && (key[i+1] == 'd' || key[i+1] == 's') {
					if key[i+1] == 'd' {
						pattern.WriteString(`(-?[0-9][0-9,.]*)`)
					} else {
						pattern.WriteString(`(.+?)`)
					}
					verbs = append(verbs, key[i+1])
					i++
					continue
				}
				pattern.WriteString(regexp.QuoteMeta(string(key[i])))
			}
			pattern.WriteString("$")
			out = append(out, template{key: key, re: regexp.MustCompile(pattern.String()), verbs: verbs})
		}
	}
	// Longer (more specific) templates first.
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].key) != len(out[j].key) {
			return len(out[i].key) > len(out[j].key)
		}
		return out[i].key < out[j].key
	})
	return out
}

// Normalize maps a locale tag ("fr-CA", "ES", "en_GB") to a supported
// locale, defaulting to English.
func Normalize(locale string) string {
	l := strings.ToLower(strings.TrimSpace(locale))
	if i := strings.IndexAny(l, "-_"); i >= 0 {
		l = l[:i]
	}
	switch l {
	case "fr", "es":
		return l
	}
	return "en"
}

// T translates an English message into locale. Unknown messages are
// returned unchanged (English).
func T(locale, msg string) string {
	cat, ok := catalogs[Normalize(locale)]
	if !ok || msg == "" {
		return msg
	}
	if v, ok := cat[msg]; ok {
		return v
	}
	for _, t := range templates {
		translated, ok := cat[t.key]
		if !ok {
			continue
		}
		m := t.re.FindStringSubmatch(msg)
		if m == nil {
			continue
		}
		args := make([]any, 0, len(m)-1)
		for i, v := range m[1:] {
			if t.verbs[i] == 's' {
				if tv, ok := cat[v]; ok {
					v = tv
				}
			}
			args = append(args, v)
		}
		return fmt.Sprintf(strings.ReplaceAll(translated, "%d", "%s"), args...)
	}
	return msg
}

// Has reports whether locale has a translation for msg (exact or template).
func Has(locale, msg string) bool {
	if Normalize(locale) == "en" {
		return true
	}
	return T(locale, msg) != msg || catalogs[Normalize(locale)][msg] == msg
}

// Fields translates the values of a validation field map, returning a new
// map (the input may be shared, e.g. by sentinel errors).
func Fields(locale string, fields map[string]any) map[string]any {
	out := make(map[string]any, len(fields))
	for k, v := range fields {
		if s, ok := v.(string); ok {
			out[k] = T(locale, s)
		} else {
			out[k] = v
		}
	}
	return out
}

// Details translates the "fields" entry of an error details map.
func Details(locale string, details map[string]any) map[string]any {
	if details == nil || Normalize(locale) == "en" {
		return details
	}
	out := make(map[string]any, len(details))
	for k, v := range details {
		if f, ok := v.(map[string]any); ok && k == "fields" {
			out[k] = Fields(locale, f)
		} else {
			out[k] = v
		}
	}
	return out
}
