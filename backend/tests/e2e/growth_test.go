package e2e

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

const (
	mobileUA = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148 Instagram 350.0"
	botUA    = "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)"
)

// visit follows a short link like a browser and returns the redirect target.
func (e *env) visit(code, ua, referer, ip string) *url.URL {
	e.t.Helper()
	req, _ := http.NewRequest(http.MethodGet, e.srv.URL+"/l/"+code, nil)
	req.Header.Set("User-Agent", ua)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	if ip != "" {
		req.Header.Set("X-Forwarded-For", ip) // ignored (no trusted proxy): all visits share the test IP
	}
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := noFollow.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusFound {
		e.t.Fatalf("short link: status %d", res.StatusCode)
	}
	if res.Header.Get("Cache-Control") != "no-store" {
		e.t.Fatalf("short link cache: %q", res.Header.Get("Cache-Control"))
	}
	u, err := url.Parse(res.Header.Get("Location"))
	if err != nil {
		e.t.Fatal(err)
	}
	return u
}

func TestCampaignsLinksAttributionAndReports(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	admin := e.createAdmin()

	// Campaign CRUD and validation.
	slug := strings.ReplaceAll(uniq("spring-launch-"), "_", "-")
	slug = strings.ToLower(slug)
	if r := admin.call(http.MethodPost, "/api/v1/admin/campaigns", map[string]any{"name": "Bad", "slug": "Bad Slug!"}); r.errCode(t) != "validation_failed" {
		t.Fatalf("bad slug: %s", r.body)
	}
	var camp service.CampaignView
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/campaigns", map[string]any{"name": "Spring launch", "slug": slug,
		"channel": "instagram", "status": "active", "budget_cents": 10000}, &camp)
	if camp.Slug != slug || camp.Status != "active" || camp.Currency != "USD" {
		t.Fatalf("campaign: %+v", camp)
	}
	if r := admin.call(http.MethodPost, "/api/v1/admin/campaigns", map[string]any{"name": "Dup", "slug": slug}); r.status != http.StatusConflict {
		t.Fatalf("duplicate slug: %d", r.status)
	}

	// Short link with UTM builder; destination host is restricted.
	if r := admin.call(http.MethodPost, "/api/v1/admin/links", map[string]any{"destination": "https://evil.example/phish", "utm_source": "instagram", "utm_medium": "social"}); r.errCode(t) != "validation_failed" {
		t.Fatalf("foreign destination: %s", r.body)
	}
	if r := admin.call(http.MethodPost, "/api/v1/admin/links", map[string]any{"destination": "//evil.example", "utm_source": "x", "utm_medium": "social"}); r.errCode(t) != "validation_failed" {
		t.Fatalf("protocol-relative destination: %s", r.body)
	}
	var link service.LinkView
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/links", map[string]any{"destination": "/en/psychics?ref=bio",
		"campaign_id": camp.ID, "utm_source": "instagram", "utm_medium": "social", "utm_content": "story-1", "label": "IG story"}, &link)
	if link.UTMCampaign != slug || len(link.Code) != 7 || link.ShortURL != "http://localhost:3000/go/"+link.Code {
		t.Fatalf("link: %+v", link)
	}
	full, _ := url.Parse(link.FullURL)
	if full.Path != "/en/psychics" || full.Query().Get("utm_source") != "instagram" || full.Query().Get("utm_campaign") != slug || full.Query().Get("ref") != "bio" {
		t.Fatalf("full url: %s", link.FullURL)
	}
	custom := uniq("tiktok")
	var link2 service.LinkView
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/links", map[string]any{"destination": "/en", "code": custom,
		"utm_source": "tiktok", "utm_medium": "paid_social"}, &link2)
	if r := admin.call(http.MethodPost, "/api/v1/admin/links", map[string]any{"destination": "/en", "code": strings.ToUpper(custom), "utm_source": "x", "utm_medium": "y"}); r.status != http.StatusConflict {
		t.Fatalf("case-insensitive duplicate code: %d", r.status)
	}

	// Clicks: humans are counted (unique by visitor), crawlers are not.
	target := e.visit(link.Code, mobileUA, "https://l.instagram.com/", "")
	click := target.Query().Get("sy_click")
	if click == "" || target.Query().Get("utm_source") != "instagram" || target.Path != "/en/psychics" {
		t.Fatalf("redirect: %s", target)
	}
	e.visit(link.Code, mobileUA, "https://l.instagram.com/", "")
	botTarget := e.visit(link.Code, botUA, "", "")
	if botTarget.Query().Get("sy_click") == "" {
		t.Fatal("bot redirect still carries a click id")
	}
	e.visit(link2.Code, "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/141.0", "", "")
	// Unknown and inactive codes go home.
	if u := e.visit("nope-unknown", mobileUA, "", ""); u.String() != "http://localhost:3000" {
		t.Fatalf("unknown code: %s", u)
	}

	// Sign-up through the click is attributed to the link and campaign.
	visitor := e.client()
	var reg struct {
		User *service.UserView `json:"user"`
	}
	visitor.must(http.StatusCreated, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email": uniq("attr") + "@example.com", "password": strongPassword, "accept_terms": true, "locale": "en",
		"attribution": map[string]any{"click_id": click, "utm_source": "instagram", "utm_medium": "social", "utm_campaign": slug,
			"referrer": "https://l.instagram.com/x?secret=1", "landing_path": "/en/psychics?utm_source=instagram", "first_seen_at": e.clock.Now().Add(-time.Minute)},
	}, &reg)
	visitor.user = reg.User
	// Forged or garbage attribution never fails sign-up.
	garbage := e.client()
	garbage.must(http.StatusCreated, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email": uniq("garbage") + "@example.com", "password": strongPassword, "accept_terms": true,
		"attribution": map[string]any{"click_id": "not-a-uuid", "utm_source": "<script>", "landing_path": "javascript:alert(1)"},
	}, nil)
	// A UTM-only visitor (no short link) is matched to the campaign by slug.
	utmOnly := e.client()
	var regUTM struct {
		User *service.UserView `json:"user"`
	}
	utmOnly.must(http.StatusCreated, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email": uniq("utm") + "@example.com", "password": strongPassword, "accept_terms": true,
		"attribution": map[string]any{"utm_source": "newsletter", "utm_medium": "email", "utm_campaign": slug},
	}, &regUTM)

	var detail service.AdminUserView
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/users/"+visitor.user.ID.String(), nil, &detail)
	if detail.Attribution == nil || detail.Attribution.LinkCode != link.Code || detail.Attribution.CampaignName != "Spring launch" ||
		detail.Attribution.ReferrerHost != "l.instagram.com" || detail.Attribution.LandingPath != "/en/psychics" {
		t.Fatalf("attribution: %+v", detail.Attribution)
	}
	var utmDetail service.AdminUserView
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/users/"+regUTM.User.ID.String(), nil, &utmDetail)
	if a := utmDetail.Attribution; a == nil || a.CampaignID == nil || *a.CampaignID != camp.ID || a.LinkID != nil || a.UTMSource != "newsletter" {
		t.Fatalf("utm attribution: %+v", a)
	}

	// The attributed visitor buys credits: click → sign-up → purchase.
	visitor.buyCredits(600)

	from := url.QueryEscape(e.clock.Now().Add(-time.Hour).Format(time.RFC3339))
	to := url.QueryEscape(e.clock.Now().Add(time.Hour).Format(time.RFC3339))
	var rep service.GrowthReport
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/campaigns/"+camp.ID.String()+"/report?interval=hour&from="+from+"&to="+to, nil, &rep)
	tt := rep.Totals
	if tt.Clicks != 2 || tt.UniqueClicks != 1 || tt.BotClicks != 1 || tt.Signups != 2 || tt.Purchasers != 1 || tt.RevenueCents != 1999 {
		t.Fatalf("campaign totals: %+v", tt)
	}
	if tt.ClickToSignupRate != 2 || tt.ClickToPurchaseRate != 1 || tt.SignupToPurchaseRate != 0.5 {
		t.Fatalf("rates: %+v", tt)
	}
	if tt.CostPerSignupCents == nil || *tt.CostPerSignupCents != 5000 || tt.ROAS == nil || *tt.ROAS < 0.19 || *tt.ROAS > 0.2 {
		t.Fatalf("cost metrics: %+v", tt)
	}
	if len(rep.ByLink) != 1 || rep.ByLink[0].Clicks != 2 || rep.ByLink[0].Signups != 1 || rep.ByLink[0].Purchasers != 1 {
		t.Fatalf("by link: %+v", rep.ByLink)
	}
	if len(rep.Devices) != 1 || rep.Devices[0].Key != "mobile" || rep.Devices[0].Count != 2 {
		t.Fatalf("devices: %+v", rep.Devices)
	}
	if len(rep.Referrers) == 0 || rep.Referrers[0].Key != "l.instagram.com" {
		t.Fatalf("referrers: %+v", rep.Referrers)
	}
	sum := int64(0)
	for _, p := range rep.Series {
		sum += p.Clicks
	}
	if sum != 2 || len(rep.Series) < 2 {
		t.Fatalf("series: %+v", rep.Series)
	}
	var linkRep service.GrowthReport
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/links/"+link2.ID.String()+"/report?from="+from+"&to="+to, nil, &linkRep)
	if linkRep.Totals.Clicks != 1 || linkRep.Devices[0].Key != "desktop" {
		t.Fatalf("link report: %+v", linkRep.Totals)
	}

	var acq service.AcquisitionOverview
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/acquisition/overview?from="+from+"&to="+to, nil, &acq)
	if acq.Totals.Clicks < 3 || acq.Totals.Signups < 2 || len(acq.Campaigns) == 0 || acq.OrganicSignups < 1 {
		t.Fatalf("acquisition: totals=%+v campaigns=%d organic=%d", acq.Totals, len(acq.Campaigns), acq.OrganicSignups)
	}
	sources := map[string]bool{}
	for _, s := range acq.BySource {
		sources[s.Source] = true
	}
	if !sources["instagram"] || !sources["newsletter"] {
		t.Fatalf("sources: %+v", acq.BySource)
	}

	// Campaign list includes metrics; deleting a used campaign archives it.
	var list struct {
		Items []service.CampaignView `json:"items"`
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/campaigns", nil, &list)
	found := false
	for _, c := range list.Items {
		if c.ID == camp.ID {
			found = c.Metrics != nil && c.Metrics.Signups == 2
		}
	}
	if !found {
		t.Fatal("campaign metrics missing")
	}
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/campaigns/"+camp.ID.String(), nil, nil)
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/campaigns/"+camp.ID.String(), nil, &camp)
	if camp.Status != "archived" {
		t.Fatalf("used campaign should be archived: %s", camp.Status)
	}
	var unused service.CampaignView
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/campaigns", map[string]any{"name": "Unused", "slug": slug + "-x"}, &unused)
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/campaigns/"+unused.ID.String(), nil, nil)
	if r := admin.call(http.MethodGet, "/api/v1/admin/campaigns/"+unused.ID.String(), nil); r.status != http.StatusNotFound {
		t.Fatalf("unused campaign not deleted: %d", r.status)
	}

	// Deactivated links stop redirecting to the landing page.
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/links/"+link2.ID.String(), map[string]any{"active": false}, nil)
	if u := e.visit(link2.Code, mobileUA, "", ""); u.String() != "http://localhost:3000" {
		t.Fatalf("inactive link redirect: %s", u)
	}
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/links/"+link2.ID.String(), nil, nil)
	var links service.Page[service.LinkView]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/links?q="+link.Code, nil, &links)
	if len(links.Items) != 1 || links.Items[0].Clicks != 2 || links.Items[0].CampaignName != "Spring launch" {
		t.Fatalf("links: %+v", links.Items)
	}
}

func TestAudienceAndInsights(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	c, _, cws, _, s := startedSession(t, e, 10, 250)
	c.must(http.StatusOK, http.MethodPatch, "/api/v1/me/profile", map[string]any{"birth_place_id": "paris-fr"}, nil)
	// A page view so the visitor shows up in device and activity insights.
	c.must(http.StatusAccepted, http.MethodPost, "/api/v1/analytics/events", map[string]any{"events": []map[string]any{{
		"event_id": uuidString(), "name": "page_view", "occurred_at": e.clock.Now(), "anonymous_id": "anon_" + strings.ReplaceAll(uniq("a"), "-", ""),
		"path": "/en/psychics", "locale": "en"}}}, nil)
	c.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/end", nil, nil)
	cws.drain()
	time.Sleep(150 * time.Millisecond) // analytics flush

	from := url.QueryEscape(e.clock.Now().Add(-24 * time.Hour).Format(time.RFC3339))
	to := url.QueryEscape(e.clock.Now().Add(time.Hour).Format(time.RFC3339))
	var aud service.Audience
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/insights/audience?from="+from+"&to="+to, nil, &aud)
	if aud.TotalUsers == 0 || aud.NewUsers == 0 || len(aud.ByRole) == 0 || len(aud.BySign) == 0 || len(aud.ByAge) == 0 || len(aud.Signups) == 0 {
		t.Fatalf("audience: %+v", aud)
	}
	countries := map[string]bool{}
	for _, k := range aud.ByCountry {
		countries[k.Key] = true
	}
	if !countries["FR"] {
		t.Fatalf("country from birth place missing: %+v", aud.ByCountry)
	}

	var eng service.Engagement
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/insights/engagement?from="+from+"&to="+to, nil, &eng)
	if eng.MAU == 0 || eng.PayingUsers == 0 || eng.ARPPUCents == 0 || len(eng.ActiveUsers) == 0 {
		t.Fatalf("engagement: %+v", eng)
	}

	var ret struct {
		Cohorts []service.Cohort `json:"cohorts"`
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/insights/retention?weeks=4", nil, &ret)
	if len(ret.Cohorts) != 4 {
		t.Fatalf("cohorts: %+v", ret.Cohorts)
	}
	last := ret.Cohorts[3]
	if last.Size == 0 || len(last.Weeks) != 1 || last.Weeks[0] <= 0 {
		t.Fatalf("current cohort: %+v", last)
	}
	if r := admin.call(http.MethodGet, "/api/v1/admin/insights/retention?weeks=99", nil); r.errCode(t) != "validation_failed" {
		t.Fatalf("weeks bound: %s", r.body)
	}

	var top struct {
		Items []service.TopUserView `json:"items"`
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/insights/top-users?metric=spend&from="+from+"&to="+to, nil, &top)
	if len(top.Items) == 0 || top.Items[0].Value <= 0 {
		t.Fatalf("top users: %+v", top.Items)
	}

	var ui service.UserInsights
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/insights/users/"+c.user.ID.String(), nil, &ui)
	if ui.User == nil || len(ui.Psychics) != 1 || ui.Psychics[0].Sessions != 1 || len(ui.Activity) < 3 || ui.User.Stats.Purchases != 1 {
		t.Fatalf("user insights: psychics=%+v activity=%d stats=%+v", ui.Psychics, len(ui.Activity), ui.User.Stats)
	}
	types := map[string]bool{}
	for _, a := range ui.Activity {
		types[a.Type] = true
	}
	for _, want := range []string{"signup", "purchase", "session"} {
		if !types[want] {
			t.Errorf("activity missing %s: %v", want, types)
		}
	}
}

func TestPatchNullClearsValuesAndEmptyListsAreArrays(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	start := e.clock.Now().Add(time.Hour).UTC().Truncate(time.Second)
	end := start.Add(48 * time.Hour)
	var camp service.CampaignView
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/campaigns", map[string]any{"name": "Dates", "slug": strings.ToLower(uniq("dates-")),
		"starts_at": start, "ends_at": end}, &camp)
	if camp.StartsAt == nil || camp.EndsAt == nil {
		t.Fatalf("dates not stored: %+v", camp)
	}
	var cleared service.CampaignView
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/campaigns/"+camp.ID.String(), map[string]any{"ends_at": nil}, &cleared)
	if cleared.EndsAt != nil || cleared.StartsAt == nil {
		t.Fatalf("ends_at not cleared: %+v", cleared)
	}
	// Absent fields stay untouched.
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/campaigns/"+camp.ID.String(), map[string]any{"name": "Dates 2"}, &cleared)
	if cleared.StartsAt == nil || cleared.Name != "Dates 2" {
		t.Fatalf("absent field changed: %+v", cleared)
	}
	if r := admin.call(http.MethodPatch, "/api/v1/admin/campaigns/"+camp.ID.String(), map[string]any{"ends_at": start.Add(-time.Hour)}); r.errCode(t) != "validation_failed" {
		t.Fatalf("end before start: %s", r.body)
	}

	var link service.LinkView
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/links", map[string]any{"destination": "/en", "utm_source": "x", "utm_medium": "social",
		"campaign_id": camp.ID, "expires_at": e.clock.Now().Add(24 * time.Hour)}, &link)
	if link.CampaignID == nil || link.ExpiresAt == nil {
		t.Fatalf("link: %+v", link)
	}
	var plain service.LinkView
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/links/"+link.ID.String(), map[string]any{"campaign_id": nil, "expires_at": nil}, &plain)
	if plain.CampaignID != nil || plain.ExpiresAt != nil || plain.UTMCampaign != link.UTMCampaign {
		t.Fatalf("link not cleared: %+v", plain)
	}

	var settings service.LinkSettings
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/links/settings", nil, &settings)
	if settings.PublicBaseURL != "http://localhost:3000/go" || len(settings.AllowedHosts) == 0 || settings.AllowedHosts[0] != "localhost:3000" || settings.Currency != "USD" {
		t.Fatalf("settings: %+v", settings)
	}

	// Empty lists are [] (never null) so clients can always iterate.
	far := url.QueryEscape(e.clock.Now().AddDate(-5, 0, 0).Format(time.RFC3339))
	farEnd := url.QueryEscape(e.clock.Now().AddDate(-5, 0, 1).Format(time.RFC3339))
	for _, path := range []string{
		"/api/v1/admin/reports/overview?from=" + far + "&to=" + farEnd,
		"/api/v1/admin/analytics/top?from=" + far + "&to=" + farEnd,
		"/api/v1/admin/analytics/timeseries?name=user.registered&from=" + far + "&to=" + farEnd,
		"/api/v1/admin/analytics/events?name=nope.event&from=" + far + "&to=" + farEnd,
	} {
		r := admin.must(http.StatusOK, http.MethodGet, path, nil, nil)
		if strings.Contains(string(r.body), ":null") {
			t.Errorf("%s returned null: %s", path, r.body)
		}
	}
}
