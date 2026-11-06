package service

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
)

// Invisible code points are written as UTF-8 byte escapes.
const (
	zeroWidthSpace = "\xe2\x80\x8b" // U+200B
	rtlOverride    = "\xe2\x80\xae" // U+202E
	ltrOverride    = "\xe2\x80\xad" // U+202D
	byteOrderMark  = "\xef\xbb\xbf" // U+FEFF
	lineSeparator  = "\xe2\x80\xa8" // U+2028
	paraSeparator  = "\xe2\x80\xa9" // U+2029
	nextLine       = "\xc2\x85"     // U+0085
)

var cleanSeeds = []string{
	"", "hello", "  padded  ", "line\r\nbreak\rold mac", "a\n\n\n\n\nb", "\x00\x01nul\x7f", "tab\there",
	"zero" + zeroWidthSpace + "width", "rtl" + rtlOverride + "override" + ltrOverride, byteOrderMark + "bom",
	"\xff\xfe invalid utf8", "emoji \xf0\x9f\x8c\x99", "x\n\n\n\x00\ny", " \n\n\n\nx",
	lineSeparator + "sep" + paraSeparator, nextLine + "nel", strings.Repeat("ab\n", 50),
}

// forbidden reports runes CleanText must remove.
func forbidden(r rune) bool {
	switch r {
	case '\r', 0x200b, 0x202e, 0x202d, 0xfeff:
		return true
	}
	return unicode.IsControl(r) && r != '\n' && r != '\t'
}

// FuzzCleanText checks the invariants of text normalisation for any input.
func FuzzCleanText(f *testing.F) {
	for _, s := range cleanSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		out := CleanText(in)
		if !utf8.ValidString(out) {
			t.Fatalf("invalid UTF-8 output for %q", in)
		}
		if again := CleanText(out); again != out {
			t.Fatalf("not idempotent: %q -> %q -> %q", in, out, again)
		}
		if out != strings.TrimSpace(out) {
			t.Fatalf("surrounding space kept: %q", out)
		}
		if strings.Contains(out, "\n\n\n\n") {
			t.Fatalf("more than three consecutive newlines: %q", out)
		}
		for _, r := range out {
			if forbidden(r) {
				t.Fatalf("forbidden rune %U kept in %q", r, out)
			}
		}
		line := CleanLine(in)
		if strings.ContainsAny(line, "\n\t") || strings.Contains(line, "  ") || line != strings.TrimSpace(line) {
			t.Fatalf("CleanLine kept layout: %q", line)
		}
		if CleanLine(line) != line {
			t.Fatalf("CleanLine not idempotent: %q", line)
		}
	})
}

// FuzzNormalizeMessageBody checks message body validation never panics,
// rejects empty bodies, enforces the limit in characters and returns clean
// text.
func FuzzNormalizeMessageBody(f *testing.F) {
	for _, s := range cleanSeeds {
		f.Add(s, 10)
	}
	f.Add(strings.Repeat("é", 1000), 1000)
	f.Add(strings.Repeat("é", 1001), 1000)
	f.Fuzz(func(t *testing.T, in string, max int) {
		if max < 1 || max > 5000 {
			max = 1000
		}
		out, err := normalizeMessageBody(in, max)
		if err != nil {
			e, ok := apperror.As(err)
			if !ok || e.Code != apperror.CodeValidation {
				t.Fatalf("unexpected error type for %q: %v", in, err)
			}
			if out != "" {
				t.Fatal("rejected input must not return a body")
			}
			clean := CleanText(in)
			if clean != "" && utf8.RuneCountInString(clean) <= max {
				t.Fatalf("valid body %q rejected", in)
			}
			return
		}
		if out == "" || utf8.RuneCountInString(out) > max {
			t.Fatalf("accepted %q (len %d, max %d)", out, utf8.RuneCountInString(out), max)
		}
		if out != CleanText(in) {
			t.Fatalf("body not normalised: %q", out)
		}
	})
}

func TestNormalizeMessageBody(t *testing.T) {
	if _, err := normalizeMessageBody("  \n"+zeroWidthSpace+" ", 10); err == nil {
		t.Fatal("blank body accepted")
	}
	if _, err := normalizeMessageBody(strings.Repeat("é", 11), 10); err == nil {
		t.Fatal("too long (in characters) accepted")
	}
	if out, err := normalizeMessageBody(strings.Repeat("é", 10), 10); err != nil || out != strings.Repeat("é", 10) {
		t.Fatalf("10 multi-byte characters must fit a limit of 10: %q %v", out, err)
	}
	if out, _ := normalizeMessageBody(" hi\r\n\x00there ", 50); out != "hi\nthere" {
		t.Fatalf("normalised: %q", out)
	}
}
