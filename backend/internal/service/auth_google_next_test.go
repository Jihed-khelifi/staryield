package service

import (
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

func TestSanitizeNext(t *testing.T) {
	long := "/" + strings.Repeat("a", maxNextLen)
	cases := []struct {
		in, want string
	}{
		// Accepted: locale-less same-site paths.
		{"/", "/"},
		{"/chat", "/chat"},
		{"/chat/ramone", "/chat/ramone"},
		{"/profile/settings#account", "/profile/settings#account"},
		{"/search?q=moon&page=2", "/search?q=moon&page=2"},
		{"/a/b-c_d.e~f", "/a/b-c_d.e~f"},
		{"/path%20with%20space", "/path%20with%20space"},
		{"/enigma", "/enigma"}, // starts with "en" but is not a locale
		{"/espace", "/espace"},
		{"/frame/x", "/frame/x"},

		// Empty and relative.
		{"", ""},
		{"chat", ""},
		{"?next=/chat", ""},
		{"#frag", ""},

		// Other origins.
		{"//evil.com", ""},
		{"//evil.com/chat", ""},
		{"///evil.com", ""},
		{"https://evil.com", ""},
		{"http://evil.com/chat", ""},
		{"HTTPS://evil.com", ""},
		{"javascript:alert(1)", ""},
		{"data:text/html,x", ""},
		{"mailto:a@b.c", ""},
		{"/\\evil.com", ""},
		{"\\\\evil.com", ""},
		{"/\\/evil.com", ""},
		{"/@evil.com", "/@evil.com"}, // a path, not a userinfo: the host stays ours
		{"/chat@evil.com", "/chat@evil.com"},

		// Control characters, whitespace, markup.
		{"/chat\r\nSet-Cookie: x=1", ""},
		{"/chat\nfoo", ""},
		{"/chat\tfoo", ""},
		{"/\t/evil.com", ""},
		{"/ /evil.com", ""},
		{"/chat foo", ""},
		{"/chat\x00", ""},
		{"/chat\x7f", ""},
		{"/\"onmouseover=x", ""},
		{"/<script>", ""},
		{"/`x`", ""},

		// Encoded variants.
		{"/%2f%2fevil.com", ""},
		{"/%2F/evil.com", ""},
		{"/%5cevil.com", ""},
		{"/%5Cevil.com", ""},
		{"/%09/evil.com", ""},
		{"/%0d%0aSet-Cookie:x", ""},
		{"/%0a", ""},
		{"/%00", ""},
		{"/%e2%80%ae/evil", ""}, // bidi override
		{"/%e2%80%8b", ""},      // zero-width space
		{"/%zz", ""},            // malformed escape
		{"/%2e%2e/%2e%2e/etc", ""},
		{"/.%2e/x", ""},
		{"/a/./b", ""},
		{"/a/../b", ""},
		{"/..", ""},
		{"/%252f%252fevil.com", "/%252f%252fevil.com"}, // double-encoded: stays literal text for the browser

		// Non-ASCII look-alikes.
		{"/∕evil.com", ""}, // division slash
		{"/／／evil.com", ""},
		{"/café", ""},

		// Already localised: the web app adds the locale itself.
		{"/en", ""},
		{"/en/chat", ""},
		{"/fr/profile", ""},
		{"/ES/profile", ""},

		// Bounded.
		{long, ""},
		{long[:maxNextLen], long[:maxNextLen]},
	}
	for _, tc := range cases {
		if got := SanitizeNext(tc.in); got != tc.want {
			t.Errorf("SanitizeNext(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMaskGoogleEmail(t *testing.T) {
	for in, want := range map[string]string{
		"jane@gmail.com":     "j•••@gmail.com",
		"j@gmail.com":        "j•••@gmail.com",
		"émile@example.com":  "é•••@example.com",
		"a.b+tag@sub.dom.io": "a•••@sub.dom.io",
		"@x.com":             "•••",
		"nonsense":           "•••",
		"":                   "•••",
	} {
		if got := maskGoogleEmail(in); got != want {
			t.Errorf("maskGoogleEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSignInMethodNames(t *testing.T) {
	g := []domain.UserIdentity{{Provider: domain.ProviderGoogle}}
	for name, tc := range map[string]struct {
		pw   bool
		ids  []domain.UserIdentity
		want string
	}{
		"password only": {true, nil, "password"},
		"both":          {true, g, "password,google"},
		"google only":   {false, g, "google"},
		"none":          {false, nil, ""},
	} {
		got := SignInMethodNames(tc.pw, tc.ids)
		if got == nil || strings.Join(got, ",") != tc.want {
			t.Errorf("%s: %v", name, got)
		}
	}
}

func TestUserHasPassword(t *testing.T) {
	if (&domain.User{PasswordHash: ""}).HasPassword() || !(&domain.User{PasswordHash: "$argon2id$x"}).HasPassword() {
		t.Fatal("'' means no password")
	}
}

func TestSiteConfigAdvertisesGoogleOnlyWhenConfigured(t *testing.T) {
	snap := &siteSnapshot{}
	snap.Signup.Enabled, snap.Signup.FreeMinutes = true, 5
	for _, on := range []bool{false, true} {
		res, err := renderSiteConfig(snap, "en", time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC), on)
		if err != nil {
			t.Fatal(err)
		}
		want := `"auth":{"google":false}`
		if on {
			want = `"auth":{"google":true}`
		}
		if res.View.Auth.Google != on || !strings.Contains(string(res.Body), want) {
			t.Errorf("googleEnabled=%v: %s", on, res.Body)
		}
	}
	a, _ := renderSiteConfig(snap, "en", time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC), false)
	b, _ := renderSiteConfig(snap, "en", time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC), true)
	if a.ETag == b.ETag {
		t.Error("the validator must change when auth.google does")
	}
}
