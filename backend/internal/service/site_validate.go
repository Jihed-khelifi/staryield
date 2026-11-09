package service

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 3.0: site settings. Pure validation helpers for banner copy,
// destinations and schedules (no I/O, unit tested on their own).

// Site configuration messages (translated at the API edge).
const (
	msgBannerTextRequired = "Enter the banner text in English."
	msgBannerTextLength   = "Use between 1 and 140 characters."
	msgBannerCTALength    = "Use at most 40 characters."
	msgBannerHTML         = "Use plain text without HTML tags."
	msgBannerPlaceholder  = "The only placeholder allowed is {free_minutes}."
	msgBannerLocale       = "Unsupported language. Use en, fr or es."
	msgBannerActionType   = "Choose link or none."
	msgBannerHrefRequired = "Enter a destination for the link."
	msgBannerHref         = "Use a page path such as /signup, or an https address on an approved site."
	msgBannerHrefLocale   = "Leave out the language prefix; it is added automatically."
	msgBannerAudience     = "Choose all, anonymous or signed_in."
	msgBannerSchedule     = "The end must be after the start."
	msgBannerLimit        = "You can have at most 10 banners. Delete one to add another."
	msgBannerOrder        = "Send every banner exactly once."
	msgBannerRotation     = "Use between 4 and 30 seconds."
	msgBannerRequired     = "This field is required."
	msgSignupMinutes      = "Use between 1 and 60 minutes."
	msgOffSessionLimit    = "Use a whole number between 0 and 500."
	msgFreeConfirmation   = "Type ENABLE FREE MODE exactly to confirm."
	msgFreeReason         = "Give a reason between 5 and 500 characters."
	msgFreeReasonLong     = "Use at most 500 characters."
	msgFreeEnds           = "Choose an end time in the future, within 90 days."
	msgSiteWrongType      = "Has the wrong type."
)

const freeMinutesPlaceholder = "{free_minutes}"

var (
	// internalHref is a locale-less site path. Go's RE2 has no look-ahead,
	// so the "no protocol-relative //" rule is checked separately.
	internalHref = regexp.MustCompile(`^/[A-Za-z0-9\-._~/?=&%#]{0,299}$`)
	// localePrefix is a language segment the clients add themselves.
	localePrefix = regexp.MustCompile(`^/(en|fr|es)([/?#]|$)`)
	// htmlTag matches something that looks like a tag or comment.
	htmlTag = regexp.MustCompile(`</?[A-Za-z!?][^>]*>?`)
)

// cleanBannerLine cleans one line of banner copy: the usual control and
// bidi stripping of CleanLine, plus every invisible format character
// (bidi isolates, word joiners, tag characters) which CleanLine keeps.
func cleanBannerLine(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) || (r >= 0xE0000 && r <= 0xE007F) {
			return -1
		}
		return r
	}, CleanLine(s))
	return strings.Join(strings.Fields(s), " ")
}

// validateL10n cleans localized copy. en is required for text; max is the
// per-language limit in characters. Languages sent empty are dropped. It
// returns the cleaned copy and field messages keyed "<field>.<locale>".
func validateL10n(field string, in map[string]string, max int, requireEN bool, fields map[string]string) domain.L10n {
	out := domain.L10n{}
	known := map[string]bool{"en": true, "fr": true, "es": true}
	for loc, raw := range in {
		key := field + "." + loc
		if !known[loc] {
			fields[key] = msgBannerLocale
			continue
		}
		v := cleanBannerLine(raw)
		switch {
		case v == "":
			if loc == "en" && requireEN {
				fields[key] = msgBannerTextRequired
			}
			continue
		case utf8.RuneCountInString(v) > max:
			if field == "text" {
				fields[key] = msgBannerTextLength
			} else {
				fields[key] = msgBannerCTALength
			}
			continue
		case htmlTag.MatchString(v):
			fields[key] = msgBannerHTML
			continue
		case !validPlaceholders(v):
			fields[key] = msgBannerPlaceholder
			continue
		}
		out[loc] = v
	}
	if requireEN {
		if _, ok := in["en"]; !ok {
			fields[field+".en"] = msgBannerTextRequired
		}
	}
	return out
}

// validPlaceholders accepts only {free_minutes}: after removing it no brace
// may remain (so "{name}", "{{x}}" and stray braces are all rejected).
func validPlaceholders(s string) bool {
	return !strings.ContainsAny(strings.ReplaceAll(s, freeMinutesPlaceholder, ""), "{}")
}

// hasPlaceholder reports whether copy uses {free_minutes}.
func hasPlaceholder(l domain.L10n) bool {
	for _, v := range l {
		if strings.Contains(v, freeMinutesPlaceholder) {
			return true
		}
	}
	return false
}

// href validation.

// hrefChecker validates banner destinations against the approved hosts.
type hrefChecker struct{ hosts map[string]bool }

func newHrefChecker(allowed []string) hrefChecker {
	h := map[string]bool{}
	for _, a := range allowed {
		if a = strings.ToLower(strings.TrimSpace(a)); a != "" {
			h[a] = true
		}
	}
	return hrefChecker{hosts: h}
}

// check returns the destination to store, or the message explaining why it
// is refused. Only two shapes pass: a locale-less internal path, and an
// absolute https URL whose host is on the allow-list. Everything else
// (javascript:, data:, protocol-relative, http:, userinfo, backslashes,
// whitespace and control characters) is refused.
func (c hrefChecker) check(raw string) (string, string) {
	href := strings.TrimSpace(raw)
	if href == "" {
		return "", msgBannerHrefRequired
	}
	if len(href) > domain.BannerHrefMax {
		return "", msgBannerHref
	}
	for _, r := range href {
		if unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\\' || r > unicode.MaxASCII {
			return "", msgBannerHref
		}
	}
	if strings.HasPrefix(href, "/") {
		if strings.HasPrefix(href, "//") || !internalHref.MatchString(href) {
			return "", msgBannerHref
		}
		if localePrefix.MatchString(href) {
			return "", msgBannerHrefLocale
		}
		return href, ""
	}
	u, err := url.Parse(href)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Opaque != "" {
		return "", msgBannerHref
	}
	if !c.hosts[strings.ToLower(u.Host)] {
		return "", msgBannerHref
	}
	return href, ""
}

// validAudience reports a known audience.
func validAudience(a string) bool {
	switch a {
	case domain.AudienceAll, domain.AudienceAnonymous, domain.AudienceSignedIn:
		return true
	}
	return false
}

// validSchedule reports whether ends is after starts (either may be unset).
func validSchedule(starts, ends *time.Time) bool {
	return starts == nil || ends == nil || ends.After(*starts)
}

// validFreeModeEnd checks that ends is in the future and at most 90 days away.
func validFreeModeEnd(ends, now time.Time) bool {
	return ends.After(now) && !ends.After(now.Add(domain.FreeModeMaxDuration))
}

// substituteMinutes replaces the {free_minutes} placeholder.
func substituteMinutes(s string, minutes int) string {
	if !strings.Contains(s, freeMinutesPlaceholder) {
		return s
	}
	return strings.ReplaceAll(s, freeMinutesPlaceholder, strconv.Itoa(minutes))
}
