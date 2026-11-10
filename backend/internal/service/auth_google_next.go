package service

import (
	"net/url"
	"strings"
	"unicode/utf8"
)

// maxNextLen bounds the post-sign-in path kept in a sign-in attempt.
const maxNextLen = 512

// SanitizeNext returns next when it is a locale-less, same-site path the web
// app may navigate to after Google sign-in, else "". The rules follow the web
// app's safeNext (a single leading slash, no backslash, no control
// characters, never another origin) and add what an open-redirect attempt
// can hide behind: percent-encoded slashes and backslashes, dot segments,
// non-ASCII look-alikes and an already localised path (the web app adds
// the locale itself).
func SanitizeNext(next string) string {
	if next == "" || len(next) > maxNextLen || next[0] != '/' || strings.HasPrefix(next, "//") {
		return ""
	}
	for i := 0; i < len(next); i++ {
		b := next[i]
		if b <= 0x20 || b >= 0x7f || strings.IndexByte("\\\"<>`{}|^", b) >= 0 {
			return "" // control, space, non-ASCII, backslash and markup characters
		}
	}
	u, err := url.Parse(next)
	if err != nil || u.IsAbs() || u.Host != "" || u.User != nil || u.Opaque != "" || !strings.HasPrefix(u.Path, "/") {
		return ""
	}
	// What the browser will see after one round of percent-decoding.
	if decoded, err := url.PathUnescape(next); err != nil || strings.HasPrefix(decoded, "//") ||
		strings.ContainsAny(decoded, "\\") || hasControl(decoded) {
		return ""
	}
	path := u.Path
	if strings.HasPrefix(path, "//") {
		return ""
	}
	for _, seg := range strings.Split(path, "/") {
		if seg == "." || seg == ".." {
			return ""
		}
	}
	first := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)[0]
	switch strings.ToLower(first) {
	case "en", "fr", "es":
		return "" // locale-less only
	}
	return next
}

func hasControl(s string) bool {
	for _, r := range s {
		switch {
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0):
			return true
		case r >= 0x200b && r <= 0x200f, r >= 0x2028 && r <= 0x202e, r >= 0x2066 && r <= 0x2069, r == 0xfeff:
			return true // zero-width and bidirectional controls
		}
	}
	return false
}

// maskGoogleEmail hides an address except its first character and its
// domain (j•••@gmail.com), as in the notification and security e-mail of
// §23.1.
func maskGoogleEmail(email string) string {
	at := strings.LastIndexByte(email, '@')
	if at <= 0 {
		return "•••"
	}
	r, _ := utf8.DecodeRuneInString(email)
	return string(r) + "•••" + email[at:]
}
