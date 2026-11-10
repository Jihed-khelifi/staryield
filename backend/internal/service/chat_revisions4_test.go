package service

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// The in-memory send-path repository has no likes or quoted messages.
func (r *osChat) LikesFor(context.Context, []uuid.UUID) ([]repository.LikeRow, error) {
	return nil, nil
}
func (r *osChat) ReplyTargets(context.Context, []uuid.UUID) ([]repository.ReplyTargetRow, error) {
	return nil, nil
}

func TestReplyExcerpt(t *testing.T) {
	long := strings.Repeat("é", 200)
	emoji := strings.Repeat("😀", 150)
	cjk := strings.Repeat("字", 141)
	for name, tc := range map[string]struct {
		in, want string
	}{
		"empty (bare image)":          {"", ""},
		"short text":                  {"Hello there", "Hello there"},
		"whitespace collapsed":        {"  Hello \n\n  there\t friend  ", "Hello there friend"},
		"exactly 140":                 {strings.Repeat("a", 140), strings.Repeat("a", 140)},
		"141 is cut to 140":           {strings.Repeat("a", 141), strings.Repeat("a", 140)},
		"multi-byte cut by runes":     {long, strings.Repeat("é", 140)},
		"emoji cut by code points":    {emoji, strings.Repeat("😀", 140)},
		"cjk":                         {cjk, strings.Repeat("字", 140)},
		"collapse happens before cut": {strings.Repeat("a \n ", 100), strings.Repeat("a ", 70)},
	} {
		got := ReplyExcerpt(tc.in)
		if got != tc.want {
			t.Errorf("%s: got %q want %q", name, got, tc.want)
		}
		if !utf8.ValidString(got) || utf8.RuneCountInString(got) > 140 {
			t.Errorf("%s: invalid or too long excerpt (%d runes)", name, utf8.RuneCountInString(got))
		}
	}
}

func TestNormalizeAlias(t *testing.T) {
	for name, tc := range map[string]struct {
		in   string
		want string
		ok   bool
	}{
		"plain":                    {"Mom", "Mom", true},
		"trim and collapse":        {"  The   dog \t lady\n", "The dog lady", true},
		"accents and emoji":        {"Élodie 🌙", "Élodie 🌙", true},
		"exactly 40":               {strings.Repeat("é", 40), strings.Repeat("é", 40), true},
		"41 is too long":           {strings.Repeat("é", 41), "", false},
		"empty":                    {"", "", false},
		"only spaces":              {" \t\n ", "", false},
		"control character":        {"Mo\x07m", "", false},
		"null byte":                {"a\x00b", "", false},
		"zero width space":         {"a" + string(rune(0x200b)) + "b", "", false},
		"right-to-left override":   {"a" + string(rune(0x202e)) + "b", "", false},
		"byte order mark":          {string(rune(0xfeff)) + "a", "", false},
		"invalid utf-8":            {"a\xffb", "", false},
		"no-break space collapses": {"a" + string(rune(0xa0)) + string(rune(0xa0)) + "b", "a b", true},
	} {
		got, err := NormalizeAlias(tc.in)
		if tc.ok {
			if err != nil || got != tc.want {
				t.Errorf("%s: got %q, %v want %q", name, got, err, tc.want)
			}
			continue
		}
		ae, isApp := apperror.As(err)
		if err == nil || !isApp || ae.Code != apperror.CodeValidation || ae.Details["fields"].(map[string]any)["alias"] == nil {
			t.Errorf("%s: want a validation error on alias, got %q, %v", name, got, err)
		}
	}
}
