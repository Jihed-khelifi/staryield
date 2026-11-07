package service

import (
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

func TestHrefAllowList(t *testing.T) {
	c := newHrefChecker([]string{"Partner.example.com", " shop.example.com:8443 ", ""})
	ok := map[string]string{
		"/signup":                           "/signup",
		"/":                                 "/",
		"/psychics?sort=rating&page=2#top":  "/psychics?sort=rating&page=2#top",
		"/offers/spring-2026_%F0%9F%8C%99":  "/offers/spring-2026_%F0%9F%8C%99",
		"  /signup  ":                       "/signup",
		"https://partner.example.com/promo": "https://partner.example.com/promo",
		"https://PARTNER.example.com":       "https://PARTNER.example.com",
		"https://shop.example.com:8443/x":   "https://shop.example.com:8443/x",
	}
	for in, want := range ok {
		got, msg := c.check(in)
		if msg != "" || got != want {
			t.Errorf("check(%q) = %q, %q; want %q", in, got, msg, want)
		}
	}
	bad := []string{
		"", "   ",
		"javascript:alert(1)", "JaVaScRiPt:alert(1)", " javascript:alert(1)", "java\tscript:alert(1)",
		"data:text/html;base64,PHNjcmlwdD4=", "vbscript:x", "file:///etc/passwd", "blob:https://partner.example.com/x",
		"//evil.example.com", "//evil.example.com/path", "/\\evil.example.com", "\\\\evil.example.com", "/%0d%0aLocation: x",
		"http://partner.example.com/", "ftp://partner.example.com/", "mailto:a@b.co", "tel:+123456789",
		"https://evil.example.com/", "https://partner.example.com.evil.com/", "https://evil.com/?https://partner.example.com",
		"https://user:pass@partner.example.com/", "https://partner.example.com@evil.com/", "https://partner.example.com:444/",
		"https://", "https:///x", "https:partner.example.com",
		"signup", "./signup", "../signup", "partner.example.com", "?x=1", "#top",
		"/path with space", "/path\nbreak", "/ünïcode", "/<script>", "/a\"b", "/a'b", "/a;b", "/a(b)",
		"/" + strings.Repeat("a", 300), "https://partner.example.com/" + strings.Repeat("a", 300),
	}
	for _, in := range bad {
		if got, msg := c.check(in); msg == "" {
			t.Errorf("check(%q) accepted as %q", in, got)
		}
	}
	// The longest internal path is 300 characters including the slash.
	if _, msg := c.check("/" + strings.Repeat("a", 299)); msg != "" {
		t.Errorf("300-character path refused: %s", msg)
	}
	// Language prefixes are added by the clients.
	for _, in := range []string{"/en", "/fr/signup", "/es?x=1", "/en/psychics", "/es#a"} {
		if _, msg := c.check(in); msg != msgBannerHrefLocale {
			t.Errorf("check(%q): %q", in, msg)
		}
	}
	for _, in := range []string{"/english", "/frank", "/espresso", "/encyclopedia/fr"} {
		if _, msg := c.check(in); msg != "" {
			t.Errorf("check(%q) refused: %q", in, msg)
		}
	}
	// With no approved hosts no external link passes.
	if _, msg := newHrefChecker(nil).check("https://partner.example.com/"); msg == "" {
		t.Error("external link accepted without approved hosts")
	}
}

func TestValidateBannerCopy(t *testing.T) {
	check := func(field string, in map[string]string, max int, requireEN bool) (domain.L10n, map[string]string) {
		fields := map[string]string{}
		return validateL10n(field, in, max, requireEN, fields), fields
	}

	out, f := check("text", map[string]string{"en": "  {free_minutes} MINUTES\nFREE ", "fr": "OFFERT", "es": ""}, 140, true)
	if len(f) != 0 || out["en"] != "{free_minutes} MINUTES FREE" || out["fr"] != "OFFERT" || len(out) != 2 {
		t.Fatalf("happy path: %v %v", out, f)
	}
	// English is required for text, optional for call-to-action labels.
	for _, in := range []map[string]string{nil, {}, {"fr": "Bonjour"}, {"en": "   "}} {
		if _, f := check("text", in, 140, true); f["text.en"] != msgBannerTextRequired {
			t.Errorf("missing en %v: %v", in, f)
		}
	}
	if out, f := check("cta_label", nil, 40, false); len(f) != 0 || len(out) != 0 {
		t.Errorf("empty cta: %v %v", out, f)
	}
	// Length is counted in characters, not bytes.
	if _, f := check("text", map[string]string{"en": strings.Repeat("é", 140)}, 140, true); len(f) != 0 {
		t.Errorf("140 characters refused: %v", f)
	}
	if _, f := check("text", map[string]string{"en": "ok", "fr": strings.Repeat("é", 141)}, 140, true); f["text.fr"] != msgBannerTextLength {
		t.Errorf("141 characters: %v", f)
	}
	if _, f := check("cta_label", map[string]string{"en": strings.Repeat("a", 41)}, 40, false); f["cta_label.en"] != msgBannerCTALength {
		t.Errorf("41-character cta: %v", f)
	}
	// Unknown languages.
	if _, f := check("text", map[string]string{"en": "ok", "de": "Hallo", "EN": "x"}, 140, true); f["text.de"] != msgBannerLocale || f["text.EN"] != msgBannerLocale {
		t.Errorf("unknown locale: %v", f)
	}
	// HTML and script attempts are refused; plain "<" in prose is fine.
	for _, in := range []string{`<script>alert(1)</script>`, `<img src=x onerror=alert(1)>`, `Hello <b>there</b>`, `<!-- x -->`, `</div>`, `<svg/onload=alert(1)>`, `a <a href="javascript:x">b</a>`} {
		if _, f := check("text", map[string]string{"en": in}, 140, true); f["text.en"] != msgBannerHTML {
			t.Errorf("html %q: %v", in, f)
		}
	}
	if out, f := check("text", map[string]string{"en": "Save <10% today & more > 5 stars"}, 140, true); len(f) != 0 || out["en"] == "" {
		t.Errorf("plain comparison signs refused: %v", f)
	}
	// Only {free_minutes} may appear between braces.
	for _, in := range []string{"{name} welcome", "{{free_minutes}}", "{free_minutes", "free_minutes}", "{FREE_MINUTES}", "{ free_minutes }", "${free_minutes}x{}", "{free_minutes} {price}"} {
		if _, f := check("text", map[string]string{"en": in}, 140, true); f["text.en"] != msgBannerPlaceholder {
			t.Errorf("placeholder %q: %v", in, f)
		}
	}
	if _, f := check("text", map[string]string{"en": "{free_minutes} and {free_minutes}"}, 140, true); len(f) != 0 {
		t.Errorf("repeated placeholder refused: %v", f)
	}
	// Control, bidi and invisible characters are stripped, not stored.
	dirty := "A" + rtlOverride + "B" + zeroWidthSpace + "C" + "\xe2\x81\xa6" + "D" + "\xe2\x81\xa9" + "E" + byteOrderMark + "F\x00G" + "\xc2\xad" + "H" + "\xf3\xa0\x81\x81" + "I"
	if out, f := check("text", map[string]string{"en": dirty}, 140, true); len(f) != 0 || out["en"] != "ABCDEFGHI" {
		t.Errorf("invisible characters kept: %q %v", out["en"], f)
	}
	if got := cleanBannerLine("a\r\nb\tc d"); strings.ContainsAny(got, "\r\n\t") || got != "a b c d" && got != "a b cd" {
		t.Errorf("line breaks: %q", got)
	}
}

func TestPlaceholderSubstitution(t *testing.T) {
	if got := substituteMinutes("{free_minutes} MINUTES FREE", 5); got != "5 MINUTES FREE" {
		t.Fatal(got)
	}
	if got := substituteMinutes("{free_minutes}+{free_minutes}", 12); got != "12+12" {
		t.Fatal(got)
	}
	if got := substituteMinutes("No placeholder", 5); got != "No placeholder" {
		t.Fatal(got)
	}
	if !hasPlaceholder(domain.L10n{"en": "a", "fr": "{free_minutes}"}) || hasPlaceholder(domain.L10n{"en": "a"}) {
		t.Fatal("hasPlaceholder")
	}
}

func TestScheduleAndFreeModeWindows(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	later, earlier := now.Add(time.Hour), now.Add(-time.Hour)
	if !validSchedule(nil, nil) || !validSchedule(&now, nil) || !validSchedule(nil, &now) || !validSchedule(&now, &later) {
		t.Error("valid schedules refused")
	}
	if validSchedule(&later, &now) || validSchedule(&now, &now) {
		t.Error("an end at or before the start accepted")
	}
	for name, c := range map[string]struct {
		end  time.Time
		want bool
	}{
		"past":          {earlier, false},
		"now":           {now, false},
		"one second":    {now.Add(time.Second), true},
		"89 days":       {now.Add(89 * 24 * time.Hour), true},
		"exactly 90":    {now.Add(90 * 24 * time.Hour), true},
		"90 days + 1 s": {now.Add(90*24*time.Hour + time.Second), false},
	} {
		if got := validFreeModeEnd(c.end, now); got != c.want {
			t.Errorf("%s: %v", name, got)
		}
	}
}

func TestBannerStatusComputation(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { t := now.Add(d); return &t }
	cases := []struct {
		name string
		b    domain.SiteBanner
		want domain.BannerStatus
	}{
		{"no schedule", domain.SiteBanner{Active: true}, domain.BannerLive},
		{"inside the window", domain.SiteBanner{Active: true, StartsAt: at(-time.Hour), EndsAt: at(time.Hour)}, domain.BannerLive},
		{"starts exactly now", domain.SiteBanner{Active: true, StartsAt: at(0)}, domain.BannerLive},
		{"starts later", domain.SiteBanner{Active: true, StartsAt: at(time.Second)}, domain.BannerScheduled},
		{"ends exactly now", domain.SiteBanner{Active: true, EndsAt: at(0)}, domain.BannerExpired},
		{"ended", domain.SiteBanner{Active: true, StartsAt: at(-2 * time.Hour), EndsAt: at(-time.Hour)}, domain.BannerExpired},
		{"inactive wins over live", domain.SiteBanner{Active: false}, domain.BannerInactive},
		{"inactive wins over expired", domain.SiteBanner{Active: false, EndsAt: at(-time.Hour)}, domain.BannerInactive},
		{"inactive wins over scheduled", domain.SiteBanner{Active: false, StartsAt: at(time.Hour)}, domain.BannerInactive},
	}
	for _, c := range cases {
		if got := c.b.StatusAt(now); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}

func TestETagMatches(t *testing.T) {
	const tag = `W/"abc123"`
	for header, want := range map[string]bool{
		"":                    false,
		`W/"abc123"`:          true,
		`"abc123"`:            true,
		`W/"zzz", W/"abc123"`: true,
		`W/"zzz",W/"abc123" `: true,
		"*":                   true,
		`W/"other"`:           false,
		`W/"abc1234"`:         false,
		`"abc123`:             false,
	} {
		if got := ETagMatches(header, tag); got != want {
			t.Errorf("ETagMatches(%q) = %v", header, got)
		}
	}
}

func FuzzHrefCheck(f *testing.F) {
	for _, s := range []string{"/signup", "//x.com", "javascript:alert(1)", "https://a.example.com/", "\x00", "/\\x", "https://a.example.com@evil.com"} {
		f.Add(s)
	}
	c := newHrefChecker([]string{"a.example.com"})
	f.Fuzz(func(t *testing.T, in string) {
		got, msg := c.check(in)
		if msg != "" {
			return
		}
		// Whatever passes is either an internal path or an https URL on the
		// allow-list, free of control characters and never protocol-relative.
		if strings.HasPrefix(got, "//") || strings.ContainsAny(got, "\\\x00\r\n\t ") {
			t.Fatalf("dangerous destination accepted: %q -> %q", in, got)
		}
		if !strings.HasPrefix(got, "/") && !strings.HasPrefix(got, "https://a.example.com") {
			t.Fatalf("unexpected destination accepted: %q -> %q", in, got)
		}
	})
}
