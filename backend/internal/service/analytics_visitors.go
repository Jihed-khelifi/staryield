package service

import (
	"regexp"
	"strings"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 3.0: calculator analytics

// calculatorPath matches the three calculators (and their results page) in
// every locale. Query strings are stripped before matching.
var calculatorPath = regexp.MustCompile(`^/(en|fr|es)/calculators/(natal-chart|path-of-life|astrology)(/results)?$`)

// clickIDPattern is the accepted form of the sy_click value.
var clickIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// CalculatorKindOfPath returns the calculator a page path belongs to, "" when
// it is not one of the calculator pages.
func CalculatorKindOfPath(path string) string {
	m := calculatorPath.FindStringSubmatch(path)
	if m == nil {
		return ""
	}
	return m[2]
}

// sanitizeClickID drops an invalid click_id property from event properties
// (the value is a lookup key and must never carry free text).
func sanitizeClickID(props domain.JSONMap) string {
	raw, present := props["click_id"]
	if !present {
		return ""
	}
	id, _ := raw.(string)
	if !clickIDPattern.MatchString(id) {
		delete(props, "click_id")
		return ""
	}
	return id
}

// visitorEvent maps a validated client event to its roll-up contribution. ok
// is false when the event contributes nothing.
func visitorEvent(e ClientEvent, evt domain.AnalyticsEvent, clickID string, authed bool, locale string, at time.Time) (domain.VisitorEvent, bool) {
	if !anonID.MatchString(e.AnonymousID) {
		return domain.VisitorEvent{}, false
	}
	v := domain.VisitorEvent{AnonymousID: e.AnonymousID, At: at, Locale: locale, SignedIn: authed, UserID: evt.UserID,
		ReferrerHost: referrerHost(e.Referrer)}
	switch e.Name {
	case "page_view":
		kind := CalculatorKindOfPath(evt.Path)
		if kind == "" {
			break
		}
		v.Type, v.Kind = domain.VisitView, kind
		return v, true
	case "calculator_started", "calculator_completed":
		kind, _ := evt.Properties["kind"].(string)
		if !domain.ValidCalculatorKind(kind) {
			break
		}
		v.Type, v.Kind = domain.VisitStart, kind
		if e.Name == "calculator_completed" {
			v.Type = domain.VisitComplete
		}
		return v, true
	case "campaign_landing":
		str := func(k string) string { s, _ := evt.Properties[k].(string); return cleanUTM(strings.TrimSpace(s)) }
		v.Type = domain.VisitLanding
		v.UTMSource, v.UTMMedium, v.UTMCampaign, v.UTMContent, v.UTMTerm = str("utm_source"), str("utm_medium"), str("utm_campaign"),
			str("utm_content"), str("utm_term")
		v.ClickID = clickID
		return v, true
	}
	if authed && evt.UserID != nil {
		// Any other signed-in event only identifies the visitor.
		v.Type, v.ReferrerHost = domain.VisitLink, ""
		return v, true
	}
	return domain.VisitorEvent{}, false
}

// WithVisitors attaches the visitor roll-up fed by Ingest.
func (s *AnalyticsService) WithVisitors(sink analytics.VisitorSink) *AnalyticsService {
	s.visitors = sink
	return s
}
