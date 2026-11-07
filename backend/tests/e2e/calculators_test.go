package e2e

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// Calculator usage and conversion analytics (revisions 3.0, §17).
//
// The roll-up tables belong to these tests alone, so each test empties them
// first and reads windows relative to the (fake) clock, which starts at the
// current time: far-future clocks would leave audit rows that sort ahead of
// everything other tests look for, and the cookie jar compares cookie expiry
// with the real clock.

// calcStart empties the roll-up tables (before and after the test).
func (e *env) calcStart() time.Time {
	e.t.Helper()
	reset := func() {
		e.app.DB.Exec(`DELETE FROM calculator_visits`)
		e.app.DB.Exec(`DELETE FROM web_visitors`)
	}
	reset()
	e.t.Cleanup(reset)
	return e.clock.Now()
}

// ds formats the UTC date n days from t (a report window bound).
func ds(t time.Time, n int) string { return t.UTC().AddDate(0, 0, n).Format("2006-01-02") }

// dayStart is the UTC midnight n days from t.
func dayStart(t time.Time, n int) time.Time {
	t = t.UTC().AddDate(0, 0, n)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// calcEvent builds a browser analytics event as the web app sends it.
func (e *env) calcEvent(name, anon, path, locale string, props map[string]any) map[string]any {
	ev := map[string]any{"event_id": uuid.NewString(), "name": name, "occurred_at": e.clock.Now().Format(time.RFC3339Nano),
		"anonymous_id": anon, "path": path, "locale": locale}
	if props != nil {
		ev["properties"] = props
	}
	return ev
}

// postEvents sends a batch and waits until the roll-up has stored it.
func (e *env) postEvents(c *client, events ...map[string]any) {
	e.t.Helper()
	var res struct {
		Accepted int `json:"accepted"`
	}
	c.must(http.StatusAccepted, http.MethodPost, "/api/v1/analytics/events", map[string]any{"events": events}, &res)
	e.flushVisitors()
}

func (e *env) flushVisitors() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	e.app.Visitors.Flush(ctx)
}

// visit posts the page view of a calculator page.
func (e *env) calcView(c *client, anon, locale, kind string) {
	e.t.Helper()
	e.postEvents(c, e.calcEvent("page_view", anon, "/"+locale+"/calculators/"+kind, locale, nil))
}

func (e *env) calcTry(c *client, anon, locale, kind string) {
	e.t.Helper()
	e.postEvents(c, e.calcEvent("calculator_started", anon, "/"+locale+"/calculators/"+kind, locale, map[string]any{"kind": kind}))
}

func (e *env) calcResult(c *client, anon, locale, kind string) {
	e.t.Helper()
	e.postEvents(c, e.calcEvent("calculator_completed", anon, "/"+locale+"/calculators/"+kind+"/results", locale, map[string]any{"kind": kind}))
}

func calcReport(t *testing.T, admin *client, query string) service.CalculatorReport {
	t.Helper()
	var out service.CalculatorReport
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/analytics/calculators?"+query, nil, &out)
	return out
}

func calcConversions(t *testing.T, admin *client, query string) service.Page[service.CalculatorConversion] {
	t.Helper()
	var out service.Page[service.CalculatorConversion]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/analytics/calculators/conversions?"+query, nil, &out)
	return out
}

func kindTotals(r service.CalculatorReport, kind string) service.CalculatorTotals {
	for _, k := range r.ByKind {
		if k.Kind == kind {
			return k.CalculatorTotals
		}
	}
	return service.CalculatorTotals{}
}

func localeTotals(r service.CalculatorReport, locale string) service.CalculatorTotals {
	for _, k := range r.ByLocale {
		if k.Locale == locale {
			return k.CalculatorTotals
		}
	}
	return service.CalculatorTotals{}
}

func sourceRow(r service.CalculatorReport, source string) *service.CalculatorSource {
	for i := range r.BySource {
		if r.BySource[i].Source == source {
			return &r.BySource[i]
		}
	}
	return nil
}

func (e *env) registerWith(c *client, email string, attribution map[string]any, locale string) *service.UserView {
	e.t.Helper()
	var out struct {
		User *service.UserView `json:"user"`
	}
	body := map[string]any{"email": email, "password": strongPassword, "accept_terms": true, "birth_date": "1990-05-17", "locale": locale}
	if attribution != nil {
		body["attribution"] = attribution
	}
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/auth/register", body, &out)
	c.user = out.User
	return out.User
}

func TestCampaignLandingIsAcceptedAndStored(t *testing.T) {
	e := newEnv(t)
	e.calcStart()
	c := e.client()
	click := uuid.NewString()
	e.postEvents(c,
		e.calcEvent("campaign_landing", "landing-visitor-1", "/en/calculators/natal-chart", "en",
			map[string]any{"utm_source": "instagram", "utm_medium": "social", "utm_campaign": "spring", "has_click": true, "click_id": click}),
		e.calcEvent("campaign_landing", "landing-visitor-2", "/en", "en", map[string]any{"utm_source": "tiktok", "click_id": "not valid!"}),
	)
	var stored []struct {
		AnonymousID string
		Properties  string
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		e.app.DB.Raw(`SELECT anonymous_id, properties::text AS properties FROM analytics_events
			WHERE name = 'web.campaign_landing' AND anonymous_id IN ('landing-visitor-1','landing-visitor-2') ORDER BY anonymous_id`).Scan(&stored)
		if len(stored) == 2 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(stored) != 2 {
		t.Fatalf("campaign_landing events stored: %d (the event used to be dropped)", len(stored))
	}
	if !strings.Contains(stored[0].Properties, click) || !strings.Contains(stored[0].Properties, "instagram") {
		t.Fatalf("landing properties: %s", stored[0].Properties)
	}
	if strings.Contains(stored[1].Properties, "click_id") {
		t.Fatalf("invalid click_id kept: %s", stored[1].Properties)
	}
	var v []struct {
		UTMSource string
		ClickID   *uuid.UUID
		Campaign  string
	}
	e.app.DB.Raw(`SELECT utm_source, click_id, utm_campaign AS campaign FROM web_visitors WHERE anonymous_id IN ('landing-visitor-1','landing-visitor-2') ORDER BY anonymous_id`).Scan(&v)
	// The first click id is a valid uuid that no click matches: it is not kept.
	if len(v) != 2 || v[0].UTMSource != "instagram" || v[0].Campaign != "spring" || v[0].ClickID != nil || v[1].UTMSource != "tiktok" {
		t.Fatalf("visitors: %+v", v)
	}
}

func TestCalculatorVisitorsAcrossLocalesAndKinds(t *testing.T) {
	e := newEnv(t)
	today := e.calcStart()
	admin := e.createAdmin()
	c := e.client()

	e.calcView(c, "visitor-en-natal", "en", "natal-chart")
	e.calcTry(c, "visitor-en-natal", "en", "natal-chart")
	e.calcResult(c, "visitor-en-natal", "en", "natal-chart")
	e.calcView(c, "visitor-fr-path", "fr", "path-of-life")
	e.calcTry(c, "visitor-fr-path", "fr", "path-of-life")
	e.calcView(c, "visitor-es-astro", "es", "astrology")
	e.calcView(c, "visitor-fr-two", "fr", "natal-chart")
	e.calcView(c, "visitor-fr-two", "fr", "astrology")
	e.calcTry(c, "visitor-fr-two", "fr", "natal-chart")
	e.calcView(c, "visitor-en-twice", "en", "natal-chart")
	e.calcView(c, "visitor-en-twice", "en", "natal-chart")
	e.postEvents(c, e.calcEvent("page_view", "visitor-elsewhere", "/en/psychics", "en", nil))
	// Results page views and tries without a valid kind do not count.
	e.postEvents(c,
		e.calcEvent("calculator_started", "visitor-bad-kind", "/en/calculators/natal-chart", "en", map[string]any{"kind": "tarot"}),
		e.calcEvent("page_view", "visitor-bad-path", "/de/calculators/natal-chart", "de", nil),
		e.calcEvent("page_view", "visitor-slash", "/en/calculators/natal-chart/", "en", nil))
	// Visitors who opted out send nothing, so there is nothing to count.

	q := "from=" + ds(today, -6) + "&to=" + ds(today, 0)
	r := calcReport(t, admin, q)
	if r.Totals.Visitors != 5 || r.Totals.Tried != 3 || r.Totals.Completed != 1 || r.Totals.Signups != 0 || r.Totals.ExistingUsers != 0 || r.Totals.SignedInVisitors != 0 {
		t.Fatalf("totals: %+v", r.Totals)
	}
	if r.Totals.TryRate != 0.6 || r.Totals.CompletionRate != 1.0/3 || r.Totals.SignupRate != 0 {
		t.Fatalf("rates: %+v", r.Totals)
	}
	if n, p, a := kindTotals(r, "natal-chart"), kindTotals(r, "path-of-life"), kindTotals(r, "astrology"); n.Visitors != 3 || n.Tried != 2 || n.Completed != 1 ||
		p.Visitors != 1 || p.Tried != 1 || a.Visitors != 2 || a.Tried != 0 {
		t.Fatalf("by kind: %+v %+v %+v", n, p, a)
	}
	if en, fr, es := localeTotals(r, "en"), localeTotals(r, "fr"), localeTotals(r, "es"); en.Visitors != 2 || fr.Visitors != 2 || es.Visitors != 1 || fr.Tried != 2 {
		t.Fatalf("by locale: %+v %+v %+v", en, fr, es)
	}
	if len(r.ByKind) != 3 || len(r.ByLocale) != 3 {
		t.Fatal("by_kind and by_locale always list all three values")
	}
	if len(r.BySource) != 1 || r.BySource[0].Source != "(direct)" || r.BySource[0].Visitors != 5 || r.BySource[0].Tried != 3 {
		t.Fatalf("direct visitors: %+v", r.BySource)
	}
	if len(r.ByLink) != 0 || r.ByLink == nil {
		t.Fatalf("by_link: %+v", r.ByLink)
	}
	if r.Range.Interval != "day" || len(r.Series) != 7 {
		t.Fatalf("series: %d points, interval %s", len(r.Series), r.Range.Interval)
	}
	for _, p := range r.Series {
		if p.Bucket.Equal(dayStart(today, 0)) {
			if p.Visitors != 5 || p.Tried != 3 || p.Completed != 1 {
				t.Fatalf("series point: %+v", p)
			}
		} else if p.Visitors != 0 || p.Tried != 0 {
			t.Fatalf("zero fill: %+v", p)
		}
	}
	// A window without traffic is all zeros, never an error.
	empty := calcReport(t, admin, "from="+ds(today, -40)+"&to="+ds(today, -34))
	if empty.Totals.Visitors != 0 || empty.Totals.TryRate != 0 || len(empty.Series) != 7 || len(empty.ByKind) != 3 || empty.BySource == nil {
		t.Fatalf("empty window: %+v", empty)
	}
	// Row-level: one row per (day, visitor, kind, locale), counts add up.
	var views, started int64
	e.app.DB.Raw(`SELECT COALESCE(sum(views),0), COALESCE(sum(started),0) FROM calculator_visits WHERE anonymous_id = 'visitor-en-twice'`).Row().Scan(&views, &started)
	var rows int64
	e.app.DB.Raw(`SELECT count(*) FROM calculator_visits WHERE anonymous_id = 'visitor-en-twice'`).Row().Scan(&rows)
	if rows != 1 || views != 2 || started != 0 {
		t.Fatalf("visitor-en-twice: rows=%d views=%d started=%d", rows, views, started)
	}
	// No raw IPs or referrer paths are stored: referrer host only.
	e.postEvents(c, map[string]any{"event_id": uuid.NewString(), "name": "page_view", "anonymous_id": "visitor-ref-host", "path": "/en/calculators/astrology",
		"locale": "en", "referrer": "https://www.google.com/search/secret/path?q=1#x", "occurred_at": e.clock.Now().Format(time.RFC3339)})
	var host string
	e.app.DB.Raw(`SELECT referrer_host FROM web_visitors WHERE anonymous_id = 'visitor-ref-host'`).Row().Scan(&host)
	if host != "www.google.com" {
		t.Fatalf("referrer host: %q", host)
	}
}

func TestCalculatorAttributionByLinkUTMAndDirect(t *testing.T) {
	e := newEnv(t)
	today := e.calcStart()
	admin := e.createAdmin()
	c := e.client()

	slug := strings.ToLower(strings.ReplaceAll(uniq("calc-spring-"), "_", "-"))
	var camp service.CampaignView
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/campaigns", map[string]any{"name": "Calculator spring", "slug": slug, "status": "active"}, &camp)
	var link service.LinkView
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/links", map[string]any{"destination": "/en/calculators/natal-chart", "campaign_id": camp.ID,
		"utm_source": "instagram", "utm_medium": "social", "label": "IG bio"}, &link)
	target1 := e.visit(link.Code, mobileUA, "https://l.instagram.com/", "")
	target2 := e.visit(link.Code, mobileUA, "https://l.instagram.com/", "")
	click1, click2 := target1.Query().Get("sy_click"), target2.Query().Get("sy_click")
	if click1 == "" || click2 == "" || click1 == click2 {
		t.Fatalf("click ids: %q %q", click1, click2)
	}
	landing := func(anon string, props map[string]any) map[string]any {
		return e.calcEvent("campaign_landing", anon, "/en/calculators/natal-chart", "en", props)
	}
	// Two visitors arrive through the real short link (browser sends its click id).
	for i, click := range []string{click1, click2} {
		anon := fmt.Sprintf("link-visitor-%d", i+1)
		e.postEvents(c, landing(anon, map[string]any{"utm_source": "instagram", "utm_medium": "social", "utm_campaign": slug, "has_click": true, "click_id": click}))
		e.calcView(c, anon, "en", "natal-chart")
	}
	e.calcTry(c, "link-visitor-1", "en", "natal-chart")
	// First touch only: a later landing never replaces it.
	e.postEvents(c, landing("link-visitor-2", map[string]any{"utm_source": "other", "utm_medium": "x"}))
	// UTM-only visitors go by source; the campaign matches by slug.
	e.postEvents(c, landing("utm-visitor-1", map[string]any{"utm_source": "newsletter", "utm_medium": "email", "utm_campaign": slug}))
	e.calcView(c, "utm-visitor-1", "fr", "astrology")
	e.postEvents(c, landing("utm-visitor-2", map[string]any{"utm_source": "newsletter", "utm_medium": "email"}))
	e.calcView(c, "utm-visitor-2", "es", "path-of-life")
	// A click id nobody issued must not attribute anything to a link.
	e.postEvents(c, landing("forged-visitor", map[string]any{"utm_source": "forged", "click_id": uuid.NewString()}))
	e.calcView(c, "forged-visitor", "en", "natal-chart")
	// Direct visitor.
	e.calcView(c, "direct-visitor", "en", "astrology")
	// Views that arrive before the landing event still pick up the first touch.
	e.calcView(c, "late-landing", "en", "natal-chart")
	e.postEvents(c, landing("late-landing", map[string]any{"utm_source": "late", "utm_medium": "social"}))

	q := "from=" + ds(today, -6) + "&to=" + ds(today, 0)
	r := calcReport(t, admin, q)
	if r.Totals.Visitors != 7 {
		t.Fatalf("visitors: %+v", r.Totals)
	}
	if row := sourceRow(r, "instagram"); row == nil || row.Visitors != 2 || row.Tried != 1 || row.Medium != "social" || row.Campaign != slug {
		t.Fatalf("instagram: %+v", row)
	}
	// by_source groups by (source, medium, campaign): the slug-matched visitor is its own row.
	var newsletter int64
	for _, row := range r.BySource {
		if row.Source == "newsletter" {
			newsletter += row.Visitors
		}
	}
	if newsletter != 2 || len(r.BySource) != 6 {
		t.Fatalf("newsletter: %d visitors, %d source rows: %+v", newsletter, len(r.BySource), r.BySource)
	}
	if row := sourceRow(r, "(direct)"); row == nil || row.Visitors != 1 {
		t.Fatalf("direct: %+v", row)
	}
	if row := sourceRow(r, "forged"); row == nil || row.Visitors != 1 {
		t.Fatalf("forged: %+v", row)
	}
	if sourceRow(r, "other") != nil {
		t.Fatal("a later landing replaced the first touch")
	}
	if row := sourceRow(r, "late"); row == nil || row.Visitors != 1 {
		t.Fatalf("late: %+v", row)
	}
	if len(r.ByLink) != 1 || r.ByLink[0].LinkID != link.ID || r.ByLink[0].Code != link.Code || r.ByLink[0].Label != "IG bio" ||
		r.ByLink[0].CampaignName != "Calculator spring" || r.ByLink[0].Visitors != 2 || r.ByLink[0].Tried != 1 {
		t.Fatalf("by_link: %+v", r.ByLink)
	}
	// Filters.
	if f := calcReport(t, admin, q+"&source=instagram"); f.Totals.Visitors != 2 || kindTotals(f, "natal-chart").Visitors != 2 || kindTotals(f, "astrology").Visitors != 0 {
		t.Fatalf("source=instagram: %+v", f.Totals)
	}
	if f := calcReport(t, admin, q+"&source="+url.QueryEscape("(direct)")); f.Totals.Visitors != 1 || len(f.BySource) != 1 || f.BySource[0].Source != "(direct)" {
		t.Fatalf("source=(direct): %+v", f.Totals)
	}
	if f := calcReport(t, admin, q+"&campaign_id="+camp.ID.String()); f.Totals.Visitors != 3 { // two via click, one via slug
		t.Fatalf("campaign filter: %+v", f.Totals)
	}
	if f := calcReport(t, admin, q+"&link_id="+link.ID.String()); f.Totals.Visitors != 2 || len(f.ByLink) != 1 {
		t.Fatalf("link filter: %+v", f.Totals)
	}
	if f := calcReport(t, admin, q+"&link_id="+uuid.NewString()); f.Totals.Visitors != 0 {
		t.Fatalf("unknown link: %+v", f.Totals)
	}
	if f := calcReport(t, admin, q+"&locale=fr&source=newsletter"); f.Totals.Visitors != 1 || localeTotals(f, "fr").Visitors != 1 || localeTotals(f, "en").Visitors != 0 {
		t.Fatalf("locale+source: %+v", f.Totals)
	}
	if f := calcReport(t, admin, q+"&kind=astrology"); f.Totals.Visitors != 2 { // utm-visitor-1 and direct-visitor
		t.Fatalf("kind filter: %+v", f.Totals)
	}
	if f := calcReport(t, admin, q+"&source=nothing-like-this"); f.Totals.Visitors != 0 || len(f.BySource) != 0 || len(f.Series) != 7 {
		t.Fatalf("unknown source: %+v", f)
	}
}

func TestCalculatorSignupsAndExistingUsers(t *testing.T) {
	e := newEnv(t)
	today := e.calcStart()
	anon := e.client()

	// An existing account, created before it browses the calculators.
	existing := e.registerClient()
	existingEmail := existing.user.Email

	// The visitor who will sign up: browses, tries, then registers.
	e.postEvents(anon, e.calcEvent("campaign_landing", "signup-visitor-1", "/fr/calculators/astrology", "fr",
		map[string]any{"utm_source": "tiktok", "utm_medium": "paid_social"}))
	e.calcView(anon, "signup-visitor-1", "fr", "astrology")
	e.calcTry(anon, "signup-visitor-1", "fr", "astrology")
	// Visitors that never sign up, and a visitor who only registers.
	e.calcView(anon, "browsing-only-1", "en", "natal-chart")
	e.clock.Advance(10 * time.Minute)
	signer := e.client()
	user := e.registerWith(signer, uniq("signup")+"@example.com", map[string]any{"anonymous_id": "signup-visitor-1", "utm_source": "tiktok",
		"utm_medium": "paid_social", "first_seen_at": e.clock.Now().Add(-10 * time.Minute)}, "fr")
	// Registering without ever visiting a calculator creates no calculator traffic.
	e.registerWith(e.client(), uniq("nocalc")+"@example.com", map[string]any{"anonymous_id": "register-only-1"}, "en")
	// An invalid visitor id is ignored and never blocks sign-up; unknown fields stay rejected.
	e.registerWith(e.client(), uniq("badid")+"@example.com", map[string]any{"anonymous_id": "bad id!"}, "en")
	var n int64
	e.app.DB.Raw(`SELECT count(*) FROM web_visitors WHERE anonymous_id = 'bad id!'`).Row().Scan(&n)
	if n != 0 {
		t.Fatal("invalid anonymous id stored")
	}
	r := e.client().call(http.MethodPost, "/api/v1/auth/register", map[string]any{"email": uniq("x") + "@example.com", "password": strongPassword,
		"accept_terms": true, "attribution": map[string]any{"anonymous_id": "valid-anon-id-1", "bogus": 1}})
	if r.status == http.StatusCreated {
		t.Fatalf("unknown attribution field accepted: %s", r.body)
	}

	// Two days later the existing user browses while signed in.
	e.clock.Advance(48 * time.Hour)
	existing = e.login(existingEmail, strongPassword) // the access token expired meanwhile
	e.postEvents(existing, e.calcEvent("page_view", "existing-visitor-1", "/en/calculators/natal-chart", "en", nil))
	e.postEvents(existing, e.calcEvent("calculator_started", "existing-visitor-1", "/en/calculators/natal-chart", "en", map[string]any{"kind": "natal-chart"}))
	// Another account signing in on the same browser does not take the visitor over.
	other := e.registerClient()
	e.postEvents(other, e.calcEvent("page_view", "existing-visitor-1", "/en/psychics", "en", nil))
	var linked uuid.UUID
	e.app.DB.Raw(`SELECT user_id FROM web_visitors WHERE anonymous_id = 'existing-visitor-1'`).Row().Scan(&linked)
	if linked != existing.user.ID {
		t.Fatalf("first link must win: %s", linked)
	}
	// Signed-in events of a visitor that never used the calculators create nothing.
	e.app.DB.Raw(`SELECT count(*) FROM web_visitors WHERE anonymous_id = 'someone-else-9'`).Row().Scan(&n)
	e.postEvents(other, e.calcEvent("page_view", "someone-else-9", "/en/psychics", "en", nil))
	var m int64
	e.app.DB.Raw(`SELECT count(*) FROM web_visitors WHERE anonymous_id = 'someone-else-9'`).Row().Scan(&m)
	if n != 0 || m != 0 {
		t.Fatal("a signed-in event outside the calculators created a visitor")
	}

	admin := e.createAdmin() // after the clock moved: admin sessions idle out after 30 minutes
	win := "from=" + ds(today, 0) + "&to=" + ds(today, 3)
	rep := calcReport(t, admin, win)
	tot := rep.Totals
	if tot.Visitors != 3 || tot.SignedInVisitors != 1 || tot.ExistingUsers != 1 || tot.Signups != 1 || tot.Tried != 2 {
		t.Fatalf("totals: %+v", tot)
	}
	if tot.SignupRate != 0.5 { // 1 sign-up of the 2 visitors who were not users yet
		t.Fatalf("signup rate: %v", tot.SignupRate)
	}
	if row := sourceRow(rep, "tiktok"); row == nil || row.Visitors != 1 || row.Signups != 1 || row.Tried != 1 {
		t.Fatalf("tiktok: %+v", row)
	}
	if row := sourceRow(rep, "(direct)"); row == nil || row.Visitors != 2 || row.Signups != 0 {
		t.Fatalf("direct: %+v", row)
	}
	// The sign-up counts in the bucket of the first visit.
	var signups int64
	for _, p := range rep.Series {
		signups += p.Signups
		if p.Signups > 0 && !p.Bucket.Equal(dayStart(today, 0)) {
			t.Fatalf("sign-up bucket: %+v", p)
		}
	}
	if signups != 1 {
		t.Fatalf("series signups: %d", signups)
	}
	if f := calcReport(t, admin, win+"&source=tiktok"); f.Totals.Signups != 1 || f.Totals.SignupRate != 1 {
		t.Fatalf("tiktok filter: %+v", f.Totals)
	}
	// Starting the window after the visit leaves no first visit in it.
	if f := calcReport(t, admin, "from="+ds(today, 1)+"&to="+ds(today, 3)); f.Totals.Signups != 0 || f.Totals.Visitors != 1 {
		t.Fatalf("window after the sign-up: %+v", f.Totals)
	}

	conv := calcConversions(t, admin, win)
	if len(conv.Items) != 1 || conv.NextCursor != "" {
		t.Fatalf("conversions: %+v", conv)
	}
	c := conv.Items[0]
	if c.User.ID != user.ID || c.User.Email != user.Email || c.User.Role != "client" || c.User.Status != "active" || !c.Tried ||
		len(c.Kinds) != 1 || c.Kinds[0] != "astrology" || len(c.Locales) != 1 || c.Locales[0] != "fr" {
		t.Fatalf("conversion: %+v", c)
	}
	if c.Attribution == nil || c.Attribution.UTMSource != "tiktok" || c.Attribution.UTMMedium != "paid_social" {
		t.Fatalf("attribution: %+v", c.Attribution)
	}
	if !c.FirstVisitAt.Before(c.SignedUpAt) || !c.SignedUpAt.Equal(c.User.CreatedAt) {
		t.Fatalf("times: first visit %v signed up %v created %v", c.FirstVisitAt, c.SignedUpAt, c.User.CreatedAt)
	}
	// Filters apply to the conversions too.
	if f := calcConversions(t, admin, win+"&kind=natal-chart"); len(f.Items) != 0 {
		t.Fatalf("kind filter: %+v", f.Items)
	}
	if f := calcConversions(t, admin, win+"&locale=fr&source=tiktok"); len(f.Items) != 1 {
		t.Fatalf("locale+source filter: %+v", f.Items)
	}
	if f := calcConversions(t, admin, win+"&source="+url.QueryEscape("(direct)")); len(f.Items) != 0 {
		t.Fatalf("direct filter: %+v", f.Items)
	}
	for _, it := range calcConversions(t, admin, win+"&limit=100").Items {
		if it.User.Email == existingEmail {
			t.Fatal("an existing user browsing signed in is not a conversion")
		}
	}
}

func TestCalculatorBucketsFiltersAndValidation(t *testing.T) {
	e := newEnv(t)
	today := e.calcStart()
	c := e.client()
	kinds := []string{"natal-chart", "path-of-life", "astrology"}
	locales := []string{"en", "fr", "es"}
	perDay := map[time.Time]int64{} // visitors per UTC day
	total := 0
	for day := 0; day < 10; day++ {
		for v := 0; v <= day%4; v++ {
			anon := fmt.Sprintf("bucket-d%02d-v%d", day, v)
			e.calcView(c, anon, locales[(day+v)%3], kinds[(day+v)%3])
			if v == 0 {
				e.calcTry(c, anon, locales[(day+v)%3], kinds[(day+v)%3])
			}
			perDay[dayStart(today, day)]++
			total++
		}
		e.clock.Advance(24 * time.Hour)
	}
	admin := e.createAdmin()
	from, to := dayStart(today, -4), dayStart(today, 13) // the window: 17 days with the data inside
	q := "from=" + ds(today, -4) + "&to=" + ds(today, 12)
	day := calcReport(t, admin, q)
	if day.Totals.Visitors != int64(total) || len(day.Series) != 17 || day.Range.Interval != "day" ||
		!day.Range.From.Equal(from) || !day.Range.To.Equal(to) {
		t.Fatalf("day: visitors %d (want %d), %d points, range %+v", day.Totals.Visitors, total, len(day.Series), day.Range)
	}
	for i, p := range day.Series {
		if want := perDay[dayStart(today, i-4)]; p.Visitors != want || !p.Bucket.Equal(dayStart(today, i-4)) {
			t.Fatalf("day %d: %+v want %d visitors", i, p, want)
		}
	}
	hour := calcReport(t, admin, q+"&interval=hour")
	if hour.Range.Interval != "day" || len(hour.Series) != 17 {
		t.Fatalf("hour interval is served per day: %s %d", hour.Range.Interval, len(hour.Series))
	}
	// Weeks start on Monday, months on the 1st; every point is present and
	// every visitor counts in exactly one bucket.
	weekStart := func(d time.Time) time.Time { return d.AddDate(0, 0, -((int(d.Weekday()) + 6) % 7)) }
	monthStart := func(d time.Time) time.Time { return time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC) }
	for _, tc := range []struct {
		interval string
		start    func(time.Time) time.Time
		next     func(time.Time) time.Time
	}{
		{"week", weekStart, func(b time.Time) time.Time { return b.AddDate(0, 0, 7) }},
		{"month", monthStart, func(b time.Time) time.Time { return b.AddDate(0, 1, 0) }},
	} {
		want := map[time.Time]int64{}
		for d, n := range perDay {
			want[tc.start(d)] += n
		}
		var buckets []time.Time
		for b := tc.start(from); b.Before(to); b = tc.next(b) {
			buckets = append(buckets, b)
		}
		got := calcReport(t, admin, q+"&interval="+tc.interval)
		if len(got.Series) != len(buckets) || got.Range.Interval != tc.interval {
			t.Fatalf("%s: %d points, want %d", tc.interval, len(got.Series), len(buckets))
		}
		var sum int64
		for i, p := range got.Series {
			if !p.Bucket.Equal(buckets[i]) || p.Visitors != want[buckets[i]] {
				t.Fatalf("%s bucket %d: %+v, want %v with %d visitors", tc.interval, i, p, buckets[i], want[buckets[i]])
			}
			sum += p.Visitors
		}
		if sum != int64(total) {
			t.Fatalf("%s visitors add up to %d, want %d", tc.interval, sum, total)
		}
	}
	// Narrow window and filters.
	if r := calcReport(t, admin, "from="+ds(today, 0)+"&to="+ds(today, 0)); r.Totals.Visitors != 1 || len(r.Series) != 1 {
		t.Fatalf("one day: %+v", r.Totals)
	}
	var sum int64
	for _, l := range locales {
		sum += calcReport(t, admin, q+"&locale="+l).Totals.Visitors
	}
	if sum != int64(total) {
		t.Fatalf("locale filters add up to %d, want %d", sum, total)
	}
	var kindSum int64
	for _, k := range kinds {
		kindSum += calcReport(t, admin, q+"&kind="+k).Totals.Visitors
	}
	if kindSum != int64(total) {
		t.Fatalf("kind filters add up to %d, want %d", kindSum, total)
	}
	// A date and an RFC 3339 spelling of the same day are the same query.
	if a, b := calcReport(t, admin, "from="+ds(today, 0)+"&to="+ds(today, 0)),
		calcReport(t, admin, "from="+ds(today, 0)+"T00:00:00Z&to="+ds(today, 1)+"T00:00:00Z"); a.Totals != b.Totals {
		t.Fatalf("equivalent ranges differ: %+v %+v", a.Totals, b.Totals)
	}

	// Validation: every error is a 422 validation_failed with the field named.
	bad := map[string]string{
		"from=nope": "from", "to=nope": "to", "from=2026-03-01&to=2026-02-01": "from", "from=2020-01-01&to=2024-02-01": "from",
		"interval=year": "interval", "kind=tarot": "kind", "locale=de": "locale", "source=a%3Cb": "source", "source=": "source",
		"campaign_id=nope": "campaign_id", "link_id=123": "link_id",
	}
	for query, field := range bad {
		r := admin.call(http.MethodGet, "/api/v1/admin/analytics/calculators?"+query, nil)
		if r.status != http.StatusUnprocessableEntity || r.errCode(t) != "validation_failed" || !strings.Contains(string(r.body), `"`+field+`"`) {
			t.Errorf("%s: %d %s", query, r.status, r.body)
		}
	}
	for _, query := range []string{"limit=0", "limit=101", "limit=abc", "cursor=%25%25%25", "cursor=e30"} {
		r := admin.call(http.MethodGet, "/api/v1/admin/analytics/calculators/conversions?"+query, nil)
		if r.status != http.StatusUnprocessableEntity {
			t.Errorf("conversions %s: %d %s", query, r.status, r.body)
		}
	}
	// 366 days is the longest window, 367 is refused.
	if r := admin.call(http.MethodGet, "/api/v1/admin/analytics/calculators?from="+ds(today, -365)+"&to="+ds(today, 0), nil); r.status != http.StatusOK {
		t.Fatalf("366 days: %d %s", r.status, r.body)
	}
	if r := admin.call(http.MethodGet, "/api/v1/admin/analytics/calculators?from="+ds(today, -366)+"&to="+ds(today, 0), nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("367 days: %d %s", r.status, r.body)
	}
	// Messages are localized.
	admin.locale = "fr"
	r := admin.call(http.MethodGet, "/api/v1/admin/analytics/calculators?kind=tarot", nil)
	if !strings.Contains(string(r.body), "Utilisez natal-chart, path-of-life ou astrology.") {
		t.Fatalf("french message: %s", r.body)
	}
	admin.locale = "es"
	r = admin.call(http.MethodGet, "/api/v1/admin/analytics/calculators?from=2020-01-01&to=2024-02-01", nil)
	if !strings.Contains(string(r.body), "366 días") {
		t.Fatalf("spanish message: %s", r.body)
	}
}

func TestCalculatorConversionsPagination(t *testing.T) {
	e := newEnv(t)
	today := e.calcStart()
	anon := e.client()
	const n = 7
	emails := make([]string, 0, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("paging-visitor-%d", i)
		e.calcView(anon, id, "en", "natal-chart")
		e.clock.Advance(time.Minute)
		email := uniq("page") + "@example.com"
		e.registerWith(e.client(), email, map[string]any{"anonymous_id": id}, "en")
		emails = append(emails, email)
		e.clock.Advance(time.Minute)
	}
	admin := e.createAdmin()
	q := "from=" + ds(today, 0) + "&to=" + ds(today, 1)
	if r := calcReport(t, admin, q); r.Totals.Signups != n || r.Totals.Visitors != n {
		t.Fatalf("totals: %+v", r.Totals)
	}
	var got []string
	cursor, pages := "", 0
	for {
		page := calcConversions(t, admin, q+"&limit=3&cursor="+url.QueryEscape(cursor))
		pages++
		for _, it := range page.Items {
			got = append(got, it.User.Email)
		}
		if page.NextCursor == "" {
			break
		}
		if len(page.Items) != 3 {
			t.Fatalf("short page with a cursor: %d", len(page.Items))
		}
		cursor = page.NextCursor
	}
	if pages != 3 || len(got) != n {
		t.Fatalf("pages %d, items %d", pages, len(got))
	}
	for i, email := range got { // newest sign-up first
		if email != emails[n-1-i] {
			t.Fatalf("order: %v (want reverse of %v)", got, emails)
		}
	}
	// Default limit is 25, everything fits on one page.
	if p := calcConversions(t, admin, q); len(p.Items) != n || p.NextCursor != "" {
		t.Fatalf("default page: %d %q", len(p.Items), p.NextCursor)
	}
	// Exactly a page worth: no dangling cursor.
	if p := calcConversions(t, admin, q+"&limit=7"); len(p.Items) != n || p.NextCursor != "" {
		t.Fatalf("exact page: %d %q", len(p.Items), p.NextCursor)
	}
}

func TestCalculatorReportIsCachedForAMinute(t *testing.T) {
	e := newEnv(t)
	today := e.calcStart()
	admin := e.createAdmin()
	c := e.client()
	e.calcView(c, "cache-visitor-1", "en", "natal-chart")
	q := "from=" + ds(today, -1) + "&to=" + ds(today, 1)
	if r := calcReport(t, admin, q); r.Totals.Visitors != 1 {
		t.Fatalf("first read: %+v", r.Totals)
	}
	e.calcView(c, "cache-visitor-2", "en", "natal-chart")
	// Same normalized query (other spellings of the same window): cached.
	for _, same := range []string{q, "from=" + ds(today, -1) + "T10:00:00Z&to=" + ds(today, 1) + "&interval=hour", "to=" + ds(today, 1) + "&from=" + ds(today, -1) + "&interval=day"} {
		if r := calcReport(t, admin, same); r.Totals.Visitors != 1 {
			t.Fatalf("%s: not served from the cache: %+v", same, r.Totals)
		}
	}
	// A different query is computed fresh and sees the new visitor.
	if r := calcReport(t, admin, q+"&locale=en"); r.Totals.Visitors != 2 {
		t.Fatalf("fresh query: %+v", r.Totals)
	}
	// Conversions are not cached.
	if p := calcConversions(t, admin, q); len(p.Items) != 0 {
		t.Fatal("unexpected conversions")
	}
}

func TestCalculatorReportsAreAdminOnly(t *testing.T) {
	e := newEnv(t)
	for _, path := range []string{"/api/v1/admin/analytics/calculators", "/api/v1/admin/analytics/calculators/conversions"} {
		if r := e.adminClient().call(http.MethodGet, path, nil); r.status != http.StatusUnauthorized {
			t.Errorf("%s without a session: %d", path, r.status)
		}
		// A client session on the admin origin is not an admin session.
		cl := e.registerClient()
		cl.origin = adminOrigin
		if r := cl.call(http.MethodGet, path, nil); r.status != http.StatusUnauthorized && r.status != http.StatusForbidden {
			t.Errorf("%s with a client session: %d", path, r.status)
		}
		// The main site origin never reaches admin routes.
		admin := e.createAdmin()
		admin.origin = origin
		if r := admin.call(http.MethodGet, path, nil); r.status != http.StatusForbidden {
			t.Errorf("%s from the site origin: %d", path, r.status)
		}
	}
}

func TestCalculatorRowsAreErasedWithTheAccount(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.calcStart()
	anon := e.client()
	keep := "erase-keep-visitor"
	e.calcView(anon, keep, "en", "astrology")
	gone := "erase-gone-visitor"
	e.calcView(anon, gone, "fr", "natal-chart")
	e.calcTry(anon, gone, "fr", "natal-chart")
	e.clock.Advance(time.Minute)
	c := e.client()
	user := e.registerWith(c, uniq("erase")+"@example.com", map[string]any{"anonymous_id": gone}, "fr")
	count := func(table, anonID string) (n int64) {
		e.app.DB.Raw("SELECT count(*) FROM "+table+" WHERE anonymous_id = ?", anonID).Row().Scan(&n)
		return
	}
	if count("web_visitors", gone) != 1 || count("calculator_visits", gone) != 1 {
		t.Fatal("rows missing before erasure")
	}
	c.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/close", map[string]any{"password": strongPassword, "confirmation": "DELETE"}, nil)
	e.flushMail()
	e.clock.Advance(31 * 24 * time.Hour)
	if err := e.app.Scheduler.RunJob(ctx, "account-closures"); err != nil {
		t.Fatal(err)
	}
	var status string
	e.app.DB.Raw(`SELECT status FROM users WHERE id = ?`, user.ID).Row().Scan(&status)
	if status != "deleted" {
		t.Fatalf("account not erased: %s", status)
	}
	if count("web_visitors", gone) != 0 || count("calculator_visits", gone) != 0 {
		t.Fatal("the erased account's visitor rows were kept")
	}
	if count("web_visitors", keep) != 1 || count("calculator_visits", keep) != 1 {
		t.Fatal("erasure removed another visitor's rows")
	}
}

func TestWebVisitorsRetentionJob(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	now := e.clock.Now() // the cutoff is 25 months back
	old, fresh := now.AddDate(0, -27, 0), now.AddDate(0, -22, 0)
	for _, id := range []string{"retain-old-visitor", "retain-new-visitor"} {
		seen := fresh
		if strings.Contains(id, "old") {
			seen = old
		}
		if err := e.app.DB.Exec(`INSERT INTO web_visitors (anonymous_id, first_seen_at, last_seen_at) VALUES (?, ?, ?)`, id, seen, seen).Error; err != nil {
			t.Fatal(err)
		}
		if err := e.app.DB.Exec(`INSERT INTO calculator_visits (day, anonymous_id, kind, locale, views) VALUES (?, ?, 'astrology', 'en', 1)`, seen, id).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := e.app.Scheduler.RunJob(ctx, service.WebVisitorsJob); err != nil {
		t.Fatal(err)
	}
	var visitors, visits []string
	e.app.DB.Raw(`SELECT anonymous_id FROM web_visitors WHERE anonymous_id LIKE 'retain-%'`).Scan(&visitors)
	e.app.DB.Raw(`SELECT anonymous_id FROM calculator_visits WHERE anonymous_id LIKE 'retain-%'`).Scan(&visits)
	if len(visitors) != 1 || visitors[0] != "retain-new-visitor" || len(visits) != 1 || visits[0] != "retain-new-visitor" {
		t.Fatalf("after retention: %v %v", visitors, visits)
	}
}

func TestCalculatorDatabaseConstraints(t *testing.T) {
	e := newEnv(t)
	db := e.app.DB
	bad := []string{
		`INSERT INTO calculator_visits (day, anonymous_id, kind, locale) VALUES ('2041-01-01', 'constraint-visitor', 'tarot', 'en')`,
		`INSERT INTO calculator_visits (day, anonymous_id, kind, locale) VALUES ('2041-01-01', 'constraint-visitor', 'astrology', 'de')`,
		`INSERT INTO calculator_visits (day, anonymous_id, kind, locale, views) VALUES ('2041-01-01', 'constraint-visitor', 'astrology', 'en', -1)`,
		`INSERT INTO calculator_visits (day, anonymous_id, kind, locale, started) VALUES ('2041-01-01', 'constraint-visitor', 'astrology', 'en', -1)`,
		`INSERT INTO calculator_visits (day, anonymous_id, kind, locale, completed) VALUES ('2041-01-01', 'constraint-visitor', 'astrology', 'en', -1)`,
		`INSERT INTO calculator_visits (day, anonymous_id, kind, locale) VALUES ('2041-01-01', 'bad id', 'astrology', 'en')`,
		`INSERT INTO web_visitors (anonymous_id, first_seen_at, last_seen_at, first_locale) VALUES ('constraint-visitor', now(), now(), 'de')`,
		`INSERT INTO web_visitors (anonymous_id, first_seen_at, last_seen_at) VALUES ('short', now(), now())`,
		`INSERT INTO web_visitors (anonymous_id, first_seen_at, last_seen_at) VALUES ('constraint-visitor', now(), now() - interval '1 day')`,
		`INSERT INTO web_visitors (anonymous_id, first_seen_at, last_seen_at, signed_up_at) VALUES ('constraint-visitor', now(), now(), now())`,
	}
	for _, q := range bad {
		if err := db.Exec(q).Error; err == nil {
			t.Errorf("accepted: %s", q)
		}
	}
	if err := db.Exec(`INSERT INTO calculator_visits (day, anonymous_id, kind, locale) VALUES ('2041-01-01', 'constraint-visitor', 'astrology', 'en')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO calculator_visits (day, anonymous_id, kind, locale) VALUES ('2041-01-01', 'constraint-visitor', 'astrology', 'en')`).Error; err == nil {
		t.Error("duplicate primary key accepted")
	}
	db.Exec(`DELETE FROM calculator_visits WHERE anonymous_id = 'constraint-visitor'`)
}

// Many browsers posting at once: every visitor is counted exactly once and
// every counter adds up, whatever order the batches interleave in.
func TestCalculatorConcurrentIngestionCountsEverythingOnce(t *testing.T) {
	e := newEnv(t)
	today := e.calcStart()
	admin := e.createAdmin()
	const workers, perWorker = 8, 25
	done := make(chan struct{}, workers)
	for w := 0; w < workers; w++ {
		go func(w int) {
			defer func() { done <- struct{}{} }()
			c := e.client()
			for i := 0; i < perWorker; i++ {
				anon := fmt.Sprintf("conc-%d-%02d", w, i)
				path := "/en/calculators/natal-chart"
				events := []map[string]any{
					e.calcEvent("page_view", anon, path, "en", nil),
					e.calcEvent("calculator_started", anon, path, "en", map[string]any{"kind": "natal-chart"}),
					e.calcEvent("calculator_completed", anon, path+"/results", "en", map[string]any{"kind": "natal-chart"}),
					// the same visitor from another worker's perspective: a shared id
					e.calcEvent("page_view", "conc-shared", path, "en", nil),
				}
				if r := c.call(http.MethodPost, "/api/v1/analytics/events", map[string]any{"events": events}); r.status != http.StatusAccepted {
					t.Errorf("post: %d %s", r.status, r.body)
					return
				}
			}
		}(w)
	}
	for w := 0; w < workers; w++ {
		<-done
	}
	e.flushVisitors()
	r := calcReport(t, admin, "from="+ds(today, -1)+"&to="+ds(today, 1))
	if r.Totals.Visitors != workers*perWorker+1 || r.Totals.Tried != workers*perWorker || r.Totals.Completed != workers*perWorker {
		t.Fatalf("totals: %+v", r.Totals)
	}
	var views int64
	e.app.DB.Raw(`SELECT views FROM calculator_visits WHERE anonymous_id = 'conc-shared'`).Row().Scan(&views)
	if views != workers*perWorker {
		t.Fatalf("shared visitor views: %d", views)
	}
	written, dropped := e.app.Visitors.Stats()
	if dropped != 0 || written == 0 {
		t.Fatalf("aggregator stats: written=%d dropped=%d", written, dropped)
	}
}
