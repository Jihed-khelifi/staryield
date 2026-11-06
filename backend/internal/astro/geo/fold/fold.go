// Package fold normalises place names for search: it lowercases, removes
// diacritics (NFD decomposition without combining marks, plus the few Latin
// letters that do not decompose), drops apostrophes and dots, turns other
// punctuation into spaces and collapses whitespace, so "São Paulo",
// "sao-paulo" and "SAO  PAULO" compare equal. It is shared by the gazetteer
// and its data generator so their keys always agree.
package fold

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// special maps letters that NFD leaves intact.
var special = map[rune]string{
	'ß': "ss", 'æ': "ae", 'œ': "oe", 'ø': "o", 'ł': "l", 'đ': "d", 'ð': "d", 'þ': "th", 'ı': "i", 'ħ': "h",
}

// String folds s.
func String(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := true // suppress leading spaces
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		if rep, ok := special[r]; ok {
			b.WriteString(rep)
			space = false
			continue
		}
		switch {
		case unicode.Is(unicode.Mn, r):
			// combining mark left by the decomposition
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			space = false
		case r == '\'' || r == '’' || r == '.' || r == 'ʻ' || r == 'ʼ':
			// "Côte d'Ivoire" → "cote divoire"; "D.C." → "dc"
		default:
			if !space {
				b.WriteByte(' ')
				space = true
			}
		}
	}
	return strings.TrimRight(b.String(), " ")
}

// IsLatin reports whether every letter of s is Latin script (used to keep
// only alternate names our users can type).
func IsLatin(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) && !unicode.Is(unicode.Latin, r) {
			return false
		}
	}
	return true
}
