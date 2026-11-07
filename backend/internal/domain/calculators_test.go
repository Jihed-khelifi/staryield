package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

var t0 = time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)

func TestVisitorBatchCoalescesVisitsPerDayKindLocale(t *testing.T) {
	b := NewVisitorBatch()
	add := func(typ VisitorEventType, kind, locale string, at time.Time, signedIn bool) {
		b.Add(VisitorEvent{Type: typ, AnonymousID: "visitor-0001", At: at, Locale: locale, Kind: kind, SignedIn: signedIn})
	}
	add(VisitView, CalcNatalChart, "en", t0, false)
	add(VisitView, CalcNatalChart, "en", t0.Add(time.Minute), false)
	add(VisitStart, CalcNatalChart, "en", t0.Add(2*time.Minute), true)
	add(VisitComplete, CalcNatalChart, "en", t0.Add(3*time.Minute), false)
	add(VisitView, CalcNatalChart, "fr", t0, false)                   // other locale: own row
	add(VisitView, CalcAstrology, "en", t0, false)                    // other kind: own row
	add(VisitView, CalcNatalChart, "en", t0.Add(24*time.Hour), false) // next day: own row
	if len(b.Visits) != 4 {
		t.Fatalf("rows: %d", len(b.Visits))
	}
	day := time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)
	c := b.Visits[VisitKey{Day: day, AnonymousID: "visitor-0001", Kind: CalcNatalChart, Locale: "en"}]
	if c == nil || c.Views != 2 || c.Started != 1 || c.Completed != 1 || !c.SignedIn {
		t.Fatalf("counts: %+v", c)
	}
	if len(b.Visitors) != 1 {
		t.Fatalf("visitors: %d", len(b.Visitors))
	}
	v := b.Visitors["visitor-0001"]
	if !v.FirstSeen.Equal(t0) || !v.LastSeen.Equal(t0.Add(24*time.Hour)) || !v.Touched {
		t.Fatalf("visitor: %+v", v)
	}
}

func TestVisitorBatchOutOfOrderEventsKeepEarliestFirstSeenAndLocale(t *testing.T) {
	b := NewVisitorBatch()
	b.Add(VisitorEvent{Type: VisitView, AnonymousID: "visitor-0002", At: t0.Add(time.Hour), Locale: "fr", Kind: CalcAstrology})
	b.Add(VisitorEvent{Type: VisitView, AnonymousID: "visitor-0002", At: t0, Locale: "es", Kind: CalcAstrology})
	v := b.Visitors["visitor-0002"]
	if v.FirstLocale != "es" || !v.FirstSeen.Equal(t0) || !v.LastSeen.Equal(t0.Add(time.Hour)) {
		t.Fatalf("visitor: %+v", v)
	}
}

func TestFirstTouchPrecedenceFirstLandingWins(t *testing.T) {
	b := NewVisitorBatch()
	b.Add(VisitorEvent{Type: VisitLanding, AnonymousID: "visitor-0003", At: t0.Add(time.Minute), Locale: "en", UTMSource: "tiktok"})
	b.Add(VisitorEvent{Type: VisitLanding, AnonymousID: "visitor-0003", At: t0, Locale: "en", UTMSource: "instagram", ClickID: "abc"})
	b.Add(VisitorEvent{Type: VisitLanding, AnonymousID: "visitor-0003", At: t0.Add(time.Hour), Locale: "en", UTMSource: "x"})
	l := b.Visitors["visitor-0003"].Landing
	if l == nil || l.UTMSource != "instagram" || l.ClickID != "abc" {
		t.Fatalf("landing: %+v", l)
	}
	// An empty landing never becomes the first touch.
	b.Add(VisitorEvent{Type: VisitLanding, AnonymousID: "visitor-0004", At: t0, Locale: "en"})
	if b.Visitors["visitor-0004"].Landing != nil {
		t.Fatal("empty landing recorded")
	}
	if !b.Visitors["visitor-0004"].Touched {
		t.Fatal("a landing still marks the visitor as seen")
	}
}

func TestLinkOnceFirstUserWinsAndLinkOnlyVisitorsAreNotTouched(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	b := NewVisitorBatch()
	b.Add(VisitorEvent{Type: VisitLink, AnonymousID: "visitor-0005", At: t0, Locale: "en", UserID: &first})
	b.Add(VisitorEvent{Type: VisitLink, AnonymousID: "visitor-0005", At: t0.Add(time.Minute), Locale: "en", UserID: &second})
	v := b.Visitors["visitor-0005"]
	if v.UserID == nil || *v.UserID != first {
		t.Fatalf("user: %v", v.UserID)
	}
	if v.Touched || len(b.Visits) != 0 {
		t.Fatal("a link-only event must not create a visitor or visit rows")
	}
	// A calculator event of a signed-in user both counts and links.
	b.Add(VisitorEvent{Type: VisitView, AnonymousID: "visitor-0006", At: t0, Locale: "en", Kind: CalcPathOfLife, SignedIn: true, UserID: &second})
	w := b.Visitors["visitor-0006"]
	if !w.Touched || w.UserID == nil || *w.UserID != second || len(b.Visits) != 1 {
		t.Fatalf("visitor: %+v", w)
	}
}

func TestReferrerHostFirstNonEmptyWins(t *testing.T) {
	b := NewVisitorBatch()
	b.Add(VisitorEvent{Type: VisitView, AnonymousID: "visitor-0007", At: t0, Locale: "en", Kind: CalcAstrology})
	b.Add(VisitorEvent{Type: VisitView, AnonymousID: "visitor-0007", At: t0, Locale: "en", Kind: CalcAstrology, ReferrerHost: "google.com"})
	b.Add(VisitorEvent{Type: VisitView, AnonymousID: "visitor-0007", At: t0, Locale: "en", Kind: CalcAstrology, ReferrerHost: "bing.com"})
	if h := b.Visitors["visitor-0007"].ReferrerHost; h != "google.com" {
		t.Fatalf("referrer: %q", h)
	}
}

func TestResolveLanding(t *testing.T) {
	camp, link := uuid.New(), uuid.New()
	slugCamp := uuid.New()
	click := &ClickResolution{ClickID: uuid.New(), LinkID: link, CampaignID: &camp, UTMSource: "instagram", UTMMedium: "social", UTMCampaign: "spring"}
	l := VisitorLanding{UTMMedium: "paid"}
	ResolveLanding(&l, click, nil)
	if l.LinkID == nil || *l.LinkID != link || l.CampaignID == nil || *l.CampaignID != camp || l.UTMSource != "instagram" || l.UTMMedium != "paid" || l.UTMCampaign != "spring" {
		t.Fatalf("click resolution: %+v", l)
	}
	// Without a click the campaign comes from the slug, case-insensitively.
	l = VisitorLanding{UTMCampaign: "Spring"}
	ResolveLanding(&l, nil, map[string]uuid.UUID{"spring": slugCamp})
	if l.CampaignID == nil || *l.CampaignID != slugCamp || l.LinkID != nil {
		t.Fatalf("slug resolution: %+v", l)
	}
	l = VisitorLanding{UTMCampaign: "unknown"}
	ResolveLanding(&l, nil, map[string]uuid.UUID{"spring": slugCamp})
	if l.CampaignID != nil {
		t.Fatal("unknown slug resolved")
	}
}

func TestCalculatorKindAndLocaleValidation(t *testing.T) {
	for _, k := range CalculatorKinds {
		if !ValidCalculatorKind(k) {
			t.Errorf("%s rejected", k)
		}
	}
	for _, k := range []string{"", "Natal-Chart", "natal_chart", "tarot", "natal-chart ", "../astrology"} {
		if ValidCalculatorKind(k) {
			t.Errorf("%q accepted", k)
		}
	}
	if !ValidCalculatorLocale("fr") || ValidCalculatorLocale("de") || ValidCalculatorLocale("") || ValidCalculatorLocale("EN") {
		t.Fatal("locale validation")
	}
}
