package e2e

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/app"
	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 6.0 (§26.4): the calculators' e-mail report. The database is shared
// by the whole suite: every test starts from (and restores) the migration's seed
// state of the setting and an empty lead table.

const (
	reportPath        = "/api/v1/calculators/report-email"
	reportSettingPath = "/api/v1/admin/site/calculator-report"
	leadsPath         = "/api/v1/admin/marketing/leads"

	reportNatal   = `{"birth_date":"1990-03-14","birth_time":"08:45","place_id":"gn:2988507","house_system":"whole_sign","include":["tarot"]}`
	reportSun     = `{"birth_date":"1990-08-23"}`
	reportNumbers = `{"birth_date":"1990-11-29","tz":"Europe/Paris"}`
)

// birthSecrets are fragments of the birth details used by these tests: none may
// remain in the database once the report is sent.
var birthSecrets = []string{"1990-03-14", "1990-08-23", "1990-11-29", "08:45", "08%3A45", "2988507"}

// reportStart restores the seed setting and empties the lead table, now and at
// the end of the test.
func (e *env) reportStart() {
	e.t.Helper()
	reset := func() {
		e.app.DB.Exec(`DELETE FROM email_leads`)
		e.app.DB.Exec(`DELETE FROM email_suppressions WHERE source = 'e2e-report'`)
		e.app.DB.Exec(`DELETE FROM email_outbox WHERE template = 'calculator_report'`)
		e.app.DB.Exec(`UPDATE site_settings SET value = '{"enabled": true, "marketing_prechecked_countries": ["US"]}', version = version + 1 WHERE key = 'calculator_report'`)
	}
	reset()
	e.t.Cleanup(reset)
}

// withCloudflare makes CF-IPCountry trusted (the test server's peer is the proxy).
func withCloudflare(c *config.Config) {
	c.HTTP.IPSource = "cloudflare"
	c.HTTP.TrustedProxies = []string{"127.0.0.1/32", "::1/128"}
}

// withRateLimitScale sets the multiplier of every rate-limit rule (the suite default is 1000).
func withRateLimitScale(scale float64) option {
	return func(c *config.Config) { c.RateLimit.Scale = scale }
}

// reportBody is a report request.
func reportBody(kind, input, email string, extra map[string]any) map[string]any {
	body := map[string]any{"kind": kind, "input": rawJSON(input), "email": email, "locale": "en", "marketing_opt_in": false,
		"marketing_prechecked": false, "website": ""}
	for k, v := range extra {
		body[k] = v
	}
	return body
}

type rawJSON string

func (r rawJSON) MarshalJSON() ([]byte, error) { return []byte(r), nil }

// postReport sends a report request, optionally from a given country.
func (c *client) postReport(body map[string]any, country string) response {
	c.e.t.Helper()
	headers := map[string]string{"Content-Type": "application/json", "X-CSRF-Token": c.csrfToken()}
	if country != "" {
		headers["CF-IPCountry"] = country
	}
	return c.raw(http.MethodPost, reportPath, jsonBody(c.e.t, body), headers)
}

func (e *env) leadCount() (n int64) {
	e.app.DB.Model(&domain.EmailLead{}).Count(&n)
	return n
}

func (e *env) leadOf(email string) domain.EmailLead {
	e.t.Helper()
	var l domain.EmailLead
	hash := e.app.Services.Mail.AddressHash(email)
	if err := e.app.DB.First(&l, "email_hash = ?", hash).Error; err != nil {
		e.t.Fatalf("no lead for %s: %v", email, err)
	}
	return l
}

// mailTo delivers the outbox and returns what the provider received for one
// address. The outbox is shared by the whole suite (rows other tests left queued
// are delivered too), so tests look at their own recipients only.
func (e *env) mailTo(email string) []mail.Message {
	e.t.Helper()
	var out []mail.Message
	for _, m := range e.flushMail() {
		if m.To == email {
			out = append(out, m)
		}
	}
	return out
}

// noBirthDetails fails when a stored row mentions the birth details.
func (e *env) noBirthDetails(table string) {
	e.t.Helper()
	var rows []string
	if err := e.app.DB.Raw(fmt.Sprintf("SELECT row_to_json(t)::text FROM %s t", table)).Scan(&rows).Error; err != nil {
		e.t.Fatal(err)
	}
	for _, row := range rows {
		for _, secret := range birthSecrets {
			if strings.Contains(row, secret) {
				e.t.Fatalf("%s holds birth details (%q): %s", table, secret, row)
			}
		}
	}
}

func TestReportEmailConfigByCountry(t *testing.T) {
	e := newEnv(t, withCloudflare)
	e.reportStart()
	c := e.client()
	get := func(country string) (service.ReportEmailConfig, response) {
		headers := map[string]string{}
		if country != "" {
			headers["CF-IPCountry"] = country
		}
		r := c.raw(http.MethodGet, reportPath+"/config", nil, headers)
		var out service.ReportEmailConfig
		r.decode(t, &out)
		return out, r
	}
	for country, want := range map[string]bool{"US": true, "us": true, "FR": false, "DE": false, "XX": false, "T1": false, "": false} {
		out, r := get(country)
		if r.status != http.StatusOK || !out.Enabled || out.MarketingPrechecked != want {
			t.Errorf("%q: %d %+v", country, r.status, out)
		}
		if cc := r.header.Get("Cache-Control"); cc != "private, no-store" {
			t.Errorf("%q: Cache-Control %q", country, cc)
		}
	}
	// A forged header from outside Cloudflare's proxies is ignored (here: no proxy trusted).
	plain := newEnv(t)
	pc := plain.client()
	r := pc.raw(http.MethodGet, reportPath+"/config", nil, map[string]string{"CF-IPCountry": "US"})
	var out service.ReportEmailConfig
	r.decode(t, &out)
	if !out.Enabled || out.MarketingPrechecked {
		t.Fatalf("a client-chosen country must never pre-tick the box: %+v", out)
	}
	// Switched off by the admin (the write drops the cached setting at once).
	admin := e.createAdmin()
	admin.stepUp()
	var cur service.CalculatorReportSetting
	admin.must(http.StatusOK, http.MethodGet, reportSettingPath, nil, &cur)
	admin.must(http.StatusOK, http.MethodPut, reportSettingPath, map[string]any{"enabled": false,
		"marketing_prechecked_countries": cur.MarketingPrecheckedCountries, "version": cur.Version}, nil)
	if out, _ := get("US"); out.Enabled || out.MarketingPrechecked {
		t.Fatalf("switched off: %+v", out)
	}
}

func TestReportEmailQueuesTheReportAndKeepsNoBirthDetails(t *testing.T) {
	e := newEnv(t)
	e.reportStart()
	e.flushMail()
	c := e.client()
	for kind, input := range map[string]string{"natal-chart": reportNatal, "astrology": reportSun, "path-of-life": reportNumbers} {
		email := kind + "@example.com"
		var out map[string]string
		r := c.postReport(reportBody(kind, input, email, nil), "")
		if r.status != http.StatusAccepted {
			t.Fatalf("%s: %d %s", kind, r.status, r.body)
		}
		r.decode(t, &out)
		if out["status"] != "queued" {
			t.Fatalf("%s: %v", kind, out)
		}
		if r.header.Get("Cache-Control") != "private, no-store" {
			t.Fatalf("%s: Cache-Control %q", kind, r.header.Get("Cache-Control"))
		}
	}
	// The outbox holds the sealed e-mails until the worker sends them.
	var queued int64
	e.app.DB.Model(&domain.EmailOutbox{}).Where("template = ? AND status = ?", "calculator_report", domain.EmailQueued).Count(&queued)
	if queued != 3 {
		t.Fatalf("%d report e-mails queued, want 3", queued)
	}
	var delivered []mail.Message
	for _, kind := range []string{"natal-chart", "astrology", "path-of-life"} {
		delivered = append(delivered, e.mailTo(kind+"@example.com")...)
	}
	if len(delivered) != 3 {
		t.Fatalf("%d e-mails delivered, want 3", len(delivered))
	}
	bySubject := map[string]mail.Message{}
	for _, m := range delivered {
		bySubject[m.Subject] = m
		if m.Category != "account" || !strings.Contains(m.HTML, "Staryield Media LLC") || !strings.Contains(m.Text, "entertainment") {
			t.Errorf("e-mail frame: %+v", m)
		}
	}
	natal := bySubject["Your Staryield natal chart report"]
	if n := strings.Count(natal.HTML, "https://assets.staryield.net/assets/tarot/"); n != 12 {
		t.Fatalf("%d tarot pictures in the natal report, want 12", n)
	}
	for _, want := range []string{"Natal alignment calculated for:", "14 / 03 / 1990", "08:45 · Paris", "Your spread", "Your cards and why they were drawn", "House 1 ·",
		"Houses: Whole Sign.", "Click here!", "View your report", "Offered for reflection and entertainment."} {
		if !strings.Contains(natal.Text, want) {
			t.Errorf("natal report lacks %q", want)
		}
	}
	if !strings.Contains(natal.Text, "http://localhost:3000/en/calculators/natal-chart#v=1&d=1990-03-14&t=08%3A45&p=gn%3A2988507&h=whole_sign") &&
		!strings.Contains(natal.Text, "/en/calculators/natal-chart#v=1&d=1990-03-14&t=08%3A45&p=gn%3A2988507&h=whole_sign") {
		t.Fatalf("natal report has no link that reopens the result:\n%s", natal.Text)
	}
	if sun := bySubject["Your Staryield astrology report"]; !strings.Contains(sun.Text, "Your zodiac") || !strings.Contains(sun.Text, "Virgo") || !strings.Contains(sun.Text, "Mercury") {
		t.Fatalf("astrology report:\n%s", sun.Text)
	}
	if num := bySubject["Your Staryield path of life report"]; !strings.Contains(num.Text, "Life path 5") || !strings.Contains(num.Text, "How it adds up: Month: 11 · Day: 29 → 11 · Year: 1990 → 19 → 10 → 1 · Total: 23 → 5") {
		t.Fatalf("path of life report:\n%s", num.Text)
	}

	// No birth detail stays in the database: the leads never had any and the
	// sent outbox rows were wiped (recipient and body) the moment they were sent.
	e.noBirthDetails("email_leads")
	e.noBirthDetails("email_outbox")
	var wiped int64
	e.app.DB.Model(&domain.EmailOutbox{}).Where("template = ? AND status = ? AND to_sealed = '' AND payload_sealed = ''", "calculator_report", domain.EmailSent).Count(&wiped)
	if wiped != 3 {
		t.Fatalf("%d sent report rows wiped, want 3", wiped)
	}
	l := e.leadOf("natal-chart@example.com")
	if l.Source != "natal-chart" || l.ReportsSent != 1 || l.MarketingStatus != domain.MarketingNone || l.Locale != "en" || l.UserID != nil {
		t.Fatalf("lead %+v", l)
	}
}

func TestReportEmailIsSentInTheVisitorsLanguage(t *testing.T) {
	e := newEnv(t)
	e.reportStart()
	e.flushMail()
	c := e.client()
	for locale, want := range map[string]string{"fr": "Votre rapport de thème natal Staryield", "es": "Tu informe de carta natal de Staryield"} {
		body := reportBody("natal-chart", reportNatal, "lang-"+locale+"@example.com", map[string]any{"locale": locale})
		if r := c.postReport(body, ""); r.status != http.StatusAccepted {
			t.Fatalf("%s: %d %s", locale, r.status, r.body)
		}
		got := e.mailTo("lang-" + locale + "@example.com")
		if len(got) != 1 || got[0].Subject != want || strings.Contains(got[0].Text, "Your spread") {
			t.Fatalf("%s: %+v", locale, got)
		}
		if e.leadOf("lang-"+locale+"@example.com").Locale != locale {
			t.Fatalf("%s: the lead keeps the language", locale)
		}
	}
}

func TestReportEmailValidationHoneypotAndSwitch(t *testing.T) {
	e := newEnv(t)
	e.reportStart()
	e.flushMail()
	c := e.client()

	check := func(name string, body map[string]any, field string) {
		t.Helper()
		r := c.postReport(body, "")
		var resp localizedError
		r.decode(t, &resp)
		if r.status != http.StatusUnprocessableEntity || resp.Error.Code != "validation_failed" || resp.Error.Details.Fields[field] == "" {
			t.Errorf("%s: %d %s", name, r.status, r.body)
		}
	}
	check("bad e-mail", reportBody("astrology", reportSun, "nope", nil), "email")
	check("bad kind", reportBody("tarot", reportSun, "a@example.com", nil), "kind")
	check("bad locale", reportBody("astrology", reportSun, "a@example.com", map[string]any{"locale": "de"}), "locale")
	check("future birth date", reportBody("path-of-life", `{"birth_date":"2999-01-01"}`, "a@example.com", nil), "input.birth_date")
	check("time without a place", reportBody("natal-chart", `{"birth_date":"1990-03-14","birth_time":"08:45"}`, "a@example.com", nil), "input.place_id")
	check("empty input", map[string]any{"kind": "astrology", "email": "a@example.com", "locale": "en"}, "input")
	// Unknown top-level fields are refused like everywhere else.
	if r := c.postReport(reportBody("astrology", reportSun, "a@example.com", map[string]any{"role": "admin"}), ""); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown field: %d %s", r.status, r.body)
	}
	// The messages follow the visitor's language.
	fr := e.client()
	fr.locale = "fr"
	r := fr.postReport(reportBody("astrology", reportSun, "nope", nil), "")
	var resp localizedError
	r.decode(t, &resp)
	if resp.Error.Details.Fields["email"] != "Saisissez une adresse e-mail valide." {
		t.Fatalf("fr: %s", r.body)
	}

	// Honeypot: the 202 is the same, nothing is stored or sent.
	r = c.postReport(reportBody("astrology", reportSun, "bot@example.com", map[string]any{"website": "https://spam.example"}), "")
	if r.status != http.StatusAccepted || e.leadCount() != 0 {
		t.Fatalf("honeypot: %d leads=%d", r.status, e.leadCount())
	}
	if got := e.mailTo("bot@example.com"); len(got) != 0 {
		t.Fatalf("the honeypot sent %d e-mails", len(got))
	}

	// Switched off by the admin: 503 feature_unavailable.
	admin := e.createAdmin()
	admin.stepUp()
	var cur service.CalculatorReportSetting
	admin.must(http.StatusOK, http.MethodGet, reportSettingPath, nil, &cur)
	admin.must(http.StatusOK, http.MethodPut, reportSettingPath, map[string]any{"enabled": false,
		"marketing_prechecked_countries": cur.MarketingPrecheckedCountries, "version": cur.Version}, nil)
	r = c.postReport(reportBody("astrology", reportSun, "off@example.com", nil), "")
	if r.status != http.StatusServiceUnavailable || r.errCode(t) != "feature_unavailable" || e.leadCount() != 0 {
		t.Fatalf("switched off: %d %s", r.status, r.body)
	}
	if got := e.mailTo("off@example.com"); len(got) != 0 {
		t.Fatal("a switched-off report was sent")
	}
	var queued int64
	e.app.DB.Model(&domain.EmailOutbox{}).Where("template = ?", "calculator_report").Count(&queued)
	if queued != 0 {
		t.Fatalf("%d report e-mails queued by requests that were refused", queued)
	}
}

func TestReportEmailRateLimits(t *testing.T) {
	e := newEnv(t, withRateLimitScale(1))
	e.reportStart()
	c := e.client()
	post := func(email string) response {
		return c.postReport(reportBody("astrology", reportSun, email, nil), "")
	}
	// 3 per address per day.
	for i := 1; i <= 3; i++ {
		if r := post("busy@example.com"); r.status != http.StatusAccepted {
			t.Fatalf("request %d: %d %s", i, r.status, r.body)
		}
	}
	r := post("Busy@example.com")
	if r.status != http.StatusTooManyRequests || r.errCode(t) != "rate_limited" || r.header.Get("Retry-After") == "" {
		t.Fatalf("fourth request for the address: %d %s", r.status, r.body)
	}
	// 5 per hour per IP: four accepted so far from this address bucket's
	// perspective (3 + the refused one counted), one more address fits ...
	if r := post("other@example.com"); r.status != http.StatusAccepted {
		t.Fatalf("another address: %d %s", r.status, r.body)
	}
	// ... and the next request is over the IP's budget.
	r = post("third@example.com")
	if r.status != http.StatusTooManyRequests || r.errCode(t) != "rate_limited" {
		t.Fatalf("sixth request from the IP: %d %s", r.status, r.body)
	}
	if got := e.leadOf("busy@example.com").ReportsSent; got != 3 {
		t.Fatalf("reports sent %d", got)
	}
}

func TestReportEmailBurstKeepsTheQueueBounded(t *testing.T) {
	e := newEnv(t, withRateLimitScale(1))
	e.reportStart()
	var wg sync.WaitGroup
	var mu sync.Mutex
	statuses := map[int]int{}
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := e.client()
			r := c.postReport(reportBody("astrology", reportSun, fmt.Sprintf("burst%d@example.com", i), nil), "")
			mu.Lock()
			statuses[r.status]++
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	if statuses[http.StatusAccepted] != 5 || statuses[http.StatusTooManyRequests] != 35 {
		t.Fatalf("a burst of 40 from one IP: %v (5 per hour per IP)", statuses)
	}
	var queued int64
	e.app.DB.Model(&domain.EmailOutbox{}).Where("template = ?", "calculator_report").Count(&queued)
	if queued != 5 || e.leadCount() != 5 {
		t.Fatalf("queued %d, leads %d: the queue must not grow with refused requests", queued, e.leadCount())
	}
}

func TestReportEmailSuppressedAddressGetsNothing(t *testing.T) {
	e := newEnv(t)
	e.reportStart()
	e.flushMail()
	hash := e.app.Services.Mail.AddressHash("bounced@example.com")
	now := time.Now()
	if err := e.app.DB.Create(&domain.EmailSuppression{EmailHash: hash, Reason: domain.SuppressHardBounce, Source: "e2e-report",
		Timestamps: domain.Timestamps{CreatedAt: now, UpdatedAt: now}}).Error; err != nil {
		t.Fatal(err)
	}
	c := e.client()
	r := c.postReport(reportBody("astrology", reportSun, "Bounced@Example.com", nil), "")
	if r.status != http.StatusAccepted {
		t.Fatalf("a suppressed address gets the 202: %d %s", r.status, r.body)
	}
	if e.leadCount() != 0 || len(e.mailTo("bounced@example.com")) != 0 {
		t.Fatal("a suppressed address must be neither stored nor e-mailed")
	}
	var rows int64
	e.app.DB.Model(&domain.EmailOutbox{}).Where("to_hash = ?", hash).Count(&rows)
	if rows != 0 {
		t.Fatalf("%d outbox rows for a suppressed address", rows)
	}
}

func TestReportEmailLinksAccounts(t *testing.T) {
	e := newEnv(t)
	e.reportStart()
	// A signed-in visitor asking for the report of their account's address.
	me := e.registerClient()
	if r := me.postReport(reportBody("astrology", reportSun, me.user.Email, nil), ""); r.status != http.StatusAccepted {
		t.Fatalf("signed in: %d %s", r.status, r.body)
	}
	l := e.leadOf(me.user.Email)
	if l.UserID == nil || l.UserID.String() != me.user.ID.String() {
		t.Fatalf("lead %+v is not linked to %s", l, me.user.ID)
	}
	// Another address typed by a signed-in visitor is not linked.
	if r := me.postReport(reportBody("astrology", reportSun, "friend@example.com", nil), ""); r.status != http.StatusAccepted {
		t.Fatalf("friend: %d %s", r.status, r.body)
	}
	if e.leadOf("friend@example.com").UserID != nil {
		t.Fatal("a friend's address was linked to the account")
	}

	// A visitor asks first and registers later with the same address: the lead is linked.
	anon := e.client()
	if r := anon.postReport(reportBody("natal-chart", reportNatal, "later@example.com", nil), ""); r.status != http.StatusAccepted {
		t.Fatalf("anonymous: %d %s", r.status, r.body)
	}
	if e.leadOf("later@example.com").UserID != nil {
		t.Fatal("an anonymous lead was linked")
	}
	reg := e.client()
	var out struct {
		User *service.UserView `json:"user"`
	}
	reg.must(http.StatusCreated, http.MethodPost, "/api/v1/auth/register", map[string]any{"email": "Later@Example.com", "password": strongPassword,
		"accept_terms": true, "birth_date": "1990-05-17", "locale": "en"}, &out)
	l = e.leadOf("later@example.com")
	if l.UserID == nil || l.UserID.String() != out.User.ID.String() {
		t.Fatalf("registration did not link the lead: %+v", l)
	}

	// Erasing the account deletes its lead, and only its lead.
	admin := e.createAdmin()
	admin.stepUp()
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/users/"+out.User.ID.String(), map[string]any{"reason": "GDPR erasure"}, nil)
	var gone int64
	e.app.DB.Model(&domain.EmailLead{}).Where("email_hash = ?", e.app.Services.Mail.AddressHash("later@example.com")).Count(&gone)
	if gone != 0 {
		t.Fatal("the erased account's lead is still there")
	}
	if e.leadOf(me.user.Email).UserID == nil || e.leadCount() != 2 {
		t.Fatal("other leads must stay")
	}
}

var unsubscribeToken = regexp.MustCompile(`/unsubscribe\?token=([A-Za-z0-9_.\-]+)`)

func TestReportEmailMarketingConsentAndUnsubscribe(t *testing.T) {
	e := newEnv(t, withCloudflare)
	e.reportStart()
	e.flushMail()
	c := e.client()
	ua := map[string]string{"User-Agent": "Mozilla/5.0 (E2E)"}
	post := func(email string, optIn, prechecked bool, country string) mail.Message {
		t.Helper()
		headers := map[string]string{"Content-Type": "application/json", "X-CSRF-Token": c.csrfToken(), "User-Agent": ua["User-Agent"]}
		if country != "" {
			headers["CF-IPCountry"] = country
		}
		body := reportBody("astrology", reportSun, email, map[string]any{"marketing_opt_in": optIn, "marketing_prechecked": prechecked})
		if r := c.raw(http.MethodPost, reportPath, jsonBody(t, body), headers); r.status != http.StatusAccepted {
			t.Fatalf("%s: %d %s", email, r.status, r.body)
		}
		// The newest report e-mail to this address.
		got := e.mailTo(email)
		if len(got) == 0 {
			t.Fatalf("%s: no e-mail", email)
		}
		return got[len(got)-1]
	}

	// A pre-ticked box from the pre-ticked country is consent with its evidence.
	msg := post("us@example.com", true, true, "US")
	l := e.leadOf("us@example.com")
	if l.MarketingStatus != domain.MarketingSubscribed || l.ConsentAt == nil || l.ConsentPrechecked == nil || !*l.ConsentPrechecked ||
		l.ConsentTextVersion == nil || *l.ConsentTextVersion != "2026-11-19" || l.Country != "US" {
		t.Fatalf("US lead %+v", l)
	}
	if l.ConsentIPHash == nil || *l.ConsentIPHash != security.Keyed(e.app.Config.Analytics.IPHashSecret, "127.0.0.1", 32) ||
		l.ConsentUserAgent == nil || *l.ConsentUserAgent != "Mozilla/5.0 (E2E)" {
		t.Fatalf("evidence %v %v", l.ConsentIPHash, l.ConsentUserAgent)
	}
	m := unsubscribeToken.FindStringSubmatch(msg.Text)
	if m == nil || !strings.Contains(msg.Text, "/en/unsubscribe?token=") || !strings.Contains(msg.HTML, "/en/unsubscribe?token=") {
		t.Fatalf("a subscribed address gets the unsubscribe link:\n%s", msg.Text)
	}
	// (The List-Unsubscribe header is https-only; this suite's front end is http://localhost: see the renderer's unit test.)
	token, _ := url.QueryUnescape(m[1])

	// A pre-ticked box from anywhere else is not consent: the report is sent, nobody is subscribed.
	if msg := post("fr@example.com", true, true, "FR"); strings.Contains(msg.Text, "unsubscribe") {
		t.Fatal("an address that is not subscribed gets no unsubscribe link")
	}
	if got := e.leadOf("fr@example.com"); got.MarketingStatus != domain.MarketingNone || got.ConsentAt != nil || got.Country != "FR" {
		t.Fatalf("FR lead %+v", got)
	}
	// A box the visitor ticked is consent anywhere.
	post("fr-ticked@example.com", true, false, "FR")
	if got := e.leadOf("fr-ticked@example.com"); got.MarketingStatus != domain.MarketingSubscribed || *got.ConsentPrechecked {
		t.Fatalf("ticked lead %+v", got)
	}
	// Unticking later never unsubscribes.
	post("us@example.com", false, false, "US")
	if e.leadOf("us@example.com").MarketingStatus != domain.MarketingSubscribed {
		t.Fatal("an unticked box unsubscribed the address")
	}

	// The unsubscribe link: idempotent, 404 for an unknown token.
	var out map[string]string
	c.must(http.StatusOK, http.MethodPost, "/api/v1/marketing/unsubscribe", map[string]string{"token": token}, &out)
	if out["status"] != "unsubscribed" {
		t.Fatalf("unsubscribe: %v", out)
	}
	first := e.leadOf("us@example.com")
	if first.MarketingStatus != domain.MarketingUnsubscribed || first.UnsubscribedAt == nil {
		t.Fatalf("lead %+v", first)
	}
	e.clock.Advance(time.Hour)
	c.must(http.StatusOK, http.MethodPost, "/api/v1/marketing/unsubscribe", map[string]string{"token": token}, &out)
	if again := e.leadOf("us@example.com"); !again.UnsubscribedAt.Equal(*first.UnsubscribedAt) {
		t.Fatal("a second unsubscribe moved the date")
	}
	for _, bad := range []string{"", "garbage", token[:len(token)-3] + "xyz", strings.Replace(token, ".", "x.", 1)} {
		if r := c.call(http.MethodPost, "/api/v1/marketing/unsubscribe", map[string]string{"token": bad}); r.status != http.StatusNotFound {
			t.Errorf("token %q: %d %s", bad, r.status, r.body)
		}
	}
	// After unsubscribing, a pre-ticked box does not subscribe again; a ticked one does.
	if msg := post("us@example.com", true, true, "US"); strings.Contains(msg.Text, "unsubscribe") {
		t.Fatal("an unsubscribed address got the unsubscribe link")
	}
	if e.leadOf("us@example.com").MarketingStatus != domain.MarketingUnsubscribed {
		t.Fatal("a pre-ticked box re-subscribed an address that unsubscribed")
	}
	// (the third report today for this address; the budget is 3 per day at scale 1000 → unlimited here)
	post("us@example.com", true, false, "US")
	if e.leadOf("us@example.com").MarketingStatus != domain.MarketingSubscribed {
		t.Fatal("a box the visitor ticked must subscribe again")
	}
}

func TestAdminMarketingLeads(t *testing.T) {
	e := newEnv(t)
	e.reportStart()
	e.flushMail()
	c := e.client()
	add := func(email, kind, input string, optIn bool) {
		t.Helper()
		if r := c.postReport(reportBody(kind, input, email, map[string]any{"marketing_opt_in": optIn}), ""); r.status != http.StatusAccepted {
			t.Fatalf("%s: %d %s", email, r.status, r.body)
		}
		e.clock.Advance(time.Minute)
	}
	add("a@example.com", "astrology", reportSun, true)
	add("b@example.com", "natal-chart", reportNatal, false)
	add("c@example.com", "path-of-life", reportNumbers, true)
	add("d@example.com", "astrology", reportSun, true)
	// Make the creation times distinct (the fake clock does not drive created_at in the lead repository's defaults).
	for i, email := range []string{"a", "b", "c", "d"} {
		e.app.DB.Exec(`UPDATE email_leads SET created_at = ? WHERE email_hash = ?`, time.Now().Add(time.Duration(i)*time.Minute),
			e.app.Services.Mail.AddressHash(email+"@example.com"))
	}

	// Clients never reach the admin API.
	client := e.registerClient()
	if r := client.call(http.MethodGet, leadsPath, nil); r.status == http.StatusOK {
		t.Fatal("a client listed the leads")
	}
	admin := e.createAdmin()
	e.clock.Advance(11 * time.Minute) // the enrolment's step-up has passed
	list := func(query string) (service.Page[service.EmailLeadView], response) {
		var page service.Page[service.EmailLeadView]
		r := admin.call(http.MethodGet, leadsPath+query, nil)
		if r.status == http.StatusOK {
			r.decode(t, &page)
		}
		return page, r
	}
	page, r := list("")
	if r.status != http.StatusOK || len(page.Items) != 4 || page.Items[0].Email != "d@example.com" || page.Items[3].Email != "a@example.com" {
		t.Fatalf("list: %d %s", r.status, r.body)
	}
	if it := page.Items[3]; it.MarketingStatus != "subscribed" || it.SubscribedAt == nil || it.Prechecked == nil || it.Source != "astrology" ||
		it.ReportsSent != 1 || it.Locale != "en" || len(it.Kinds) != 1 {
		t.Fatalf("view %+v", it)
	}
	// Pagination by cursor, filters and the exact-address search.
	p1, _ := list("?limit=3")
	p2, _ := list("?limit=3&cursor=" + url.QueryEscape(p1.NextCursor))
	if len(p1.Items) != 3 || p1.NextCursor == "" || len(p2.Items) != 1 || p2.NextCursor != "" {
		t.Fatalf("pages: %d/%d %q", len(p1.Items), len(p2.Items), p1.NextCursor)
	}
	if sub, _ := list("?status=subscribed"); len(sub.Items) != 3 {
		t.Fatalf("subscribed: %d", len(sub.Items))
	}
	if k, _ := list("?kind=natal-chart"); len(k.Items) != 1 || k.Items[0].Email != "b@example.com" {
		t.Fatalf("kind: %+v", k.Items)
	}
	if q, _ := list("?q=" + url.QueryEscape("C@Example.com")); len(q.Items) != 1 || q.Items[0].Email != "c@example.com" {
		t.Fatalf("q: %+v", q.Items)
	}
	if _, r := list("?q=c%40example"); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("a partial address is refused: %d", r.status)
	}
	if _, r := list("?status=bogus"); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("bogus status: %d", r.status)
	}

	// The CSV export needs a fresh step-up, lists the subscribed leads and is audited.
	if r := admin.call(http.MethodGet, leadsPath+"/export.csv?status=subscribed", nil); r.status != http.StatusForbidden || r.errCode(t) != "step_up_required" {
		t.Fatalf("export without step-up: %d %s", r.status, r.body)
	}
	admin.stepUp()
	r = admin.call(http.MethodGet, leadsPath+"/export.csv", nil)
	if r.status != http.StatusOK || !strings.HasPrefix(r.header.Get("Content-Type"), "text/csv") || !strings.Contains(r.header.Get("Content-Disposition"), "attachment") ||
		r.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("export: %d %v", r.status, r.header)
	}
	rows, err := csv.NewReader(strings.NewReader(string(r.body))).ReadAll()
	if err != nil || len(rows) != 4 || strings.Join(rows[0], ",") != "email,locale,country,subscribed_at,source" {
		t.Fatalf("csv %v %v", rows, err)
	}
	got := map[string]bool{}
	for _, row := range rows[1:] {
		got[row[0]] = true
	}
	if !got["a@example.com"] || !got["c@example.com"] || !got["d@example.com"] || got["b@example.com"] {
		t.Fatalf("exported %v: subscribed leads only", got)
	}
	var audit struct {
		Items []struct {
			Action   string         `json:"action"`
			Metadata map[string]any `json:"metadata"`
		} `json:"items"`
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/audit-logs?action=marketing.leads.exported", nil, &audit)
	if len(audit.Items) != 1 || audit.Items[0].Metadata["rows"] != float64(3) {
		t.Fatalf("export audit: %+v", audit)
	}
	if r := admin.call(http.MethodGet, leadsPath+"/export.csv?status=everything", nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("bad export filter: %d", r.status)
	}

	// Erasure request: step-up, row gone, audited without the address.
	target := e.leadOf("b@example.com")
	if r := admin.call(http.MethodDelete, leadsPath+"/"+target.ID.String(), nil); r.status != http.StatusNoContent {
		// the step-up of the export is still fresh: the delete passes
		t.Fatalf("delete: %d %s", r.status, r.body)
	}
	if r := admin.call(http.MethodDelete, leadsPath+"/"+target.ID.String(), nil); r.status != http.StatusNotFound {
		t.Fatalf("second delete: %d %s", r.status, r.body)
	}
	if r := admin.call(http.MethodDelete, leadsPath+"/not-a-uuid", nil); r.status != http.StatusNotFound {
		t.Fatalf("bad id: %d", r.status)
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/audit-logs?action=marketing.lead.deleted", nil, &audit)
	if len(audit.Items) != 1 || strings.Contains(strings.ToLower(fmt.Sprint(audit.Items[0])), "b@example.com") {
		t.Fatalf("delete audit: %+v", audit)
	}
	if e.leadCount() != 3 {
		t.Fatalf("%d leads left", e.leadCount())
	}
}

func TestAdminDeleteLeadNeedsStepUp(t *testing.T) {
	e := newEnv(t)
	e.reportStart()
	c := e.client()
	if r := c.postReport(reportBody("astrology", reportSun, "x@example.com", nil), ""); r.status != http.StatusAccepted {
		t.Fatal(r.status)
	}
	admin := e.createAdmin()
	lead := e.leadOf("x@example.com")
	e.clock.Advance(11 * time.Minute) // the enrolment's step-up has passed
	if r := admin.call(http.MethodDelete, leadsPath+"/"+lead.ID.String(), nil); r.status != http.StatusForbidden || r.errCode(t) != "step_up_required" {
		t.Fatalf("delete without step-up: %d %s", r.status, r.body)
	}
	if e.leadCount() != 1 {
		t.Fatal("the lead was deleted without a step-up")
	}
}

func TestCalculatorReportSettingEndpoints(t *testing.T) {
	e := newEnv(t, withCloudflare)
	e.reportStart()
	admin := e.createAdmin()
	var audit struct {
		Items []struct {
			Action string `json:"action"`
		} `json:"items"`
	}
	// The audit trail is shared by the suite: count what this test adds.
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/audit-logs?action=site.calculator_report.updated", nil, &audit)
	auditedBefore := len(audit.Items)

	var cur service.CalculatorReportSetting
	admin.must(http.StatusOK, http.MethodGet, reportSettingPath, nil, &cur)
	if !cur.Enabled || len(cur.MarketingPrecheckedCountries) != 1 || cur.MarketingPrecheckedCountries[0] != "US" || cur.Version < 1 {
		t.Fatalf("seeded setting %+v", cur)
	}
	// Writes need a fresh step-up.
	e.clock.Advance(11 * time.Minute)
	body := map[string]any{"enabled": true, "marketing_prechecked_countries": []string{"us", "ca"}, "version": cur.Version}
	if r := admin.call(http.MethodPut, reportSettingPath, body); r.status != http.StatusForbidden || r.errCode(t) != "step_up_required" {
		t.Fatalf("put without step-up: %d %s", r.status, r.body)
	}
	admin.stepUp()
	var out service.CalculatorReportSetting
	admin.must(http.StatusOK, http.MethodPut, reportSettingPath, body, &out)
	if out.Version != cur.Version+1 || strings.Join(out.MarketingPrecheckedCountries, ",") != "US,CA" || out.UpdatedBy == nil {
		t.Fatalf("updated %+v", out)
	}
	// The dialog's config follows at once (cache dropped).
	client := e.client()
	for country, want := range map[string]bool{"CA": true, "US": true, "FR": false} {
		var cfg service.ReportEmailConfig
		client.raw(http.MethodGet, reportPath+"/config", nil, map[string]string{"CF-IPCountry": country}).decode(t, &cfg)
		if cfg.MarketingPrechecked != want {
			t.Errorf("%s: %+v", country, cfg)
		}
	}
	// A stale version is a conflict; an invalid body a validation error.
	if r := admin.call(http.MethodPut, reportSettingPath, body); r.status != http.StatusConflict || r.errCode(t) != "version_conflict" {
		t.Fatalf("stale version: %d %s", r.status, r.body)
	}
	for _, bad := range []map[string]any{
		{"enabled": true, "marketing_prechecked_countries": []string{"USA"}},
		{"enabled": true},
		{"marketing_prechecked_countries": []string{"US"}},
		{"enabled": true, "marketing_prechecked_countries": []string{"U1"}},
	} {
		if r := admin.call(http.MethodPut, reportSettingPath, bad); r.status != http.StatusUnprocessableEntity {
			t.Errorf("%v: %d %s", bad, r.status, r.body)
		}
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/audit-logs?action=site.calculator_report.updated", nil, &audit)
	if len(audit.Items) != auditedBefore+1 {
		t.Fatalf("audit: %d entries, want %d", len(audit.Items), auditedBefore+1)
	}
	// Clients cannot read or write it.
	me := e.registerClient()
	if r := me.call(http.MethodGet, reportSettingPath, nil); r.status == http.StatusOK {
		t.Fatal("a client read the admin setting")
	}
}

// Concurrent first requests for one address create one lead, never an aborted
// transaction or a lost update.
func TestReportEmailConcurrentRequestsMakeOneLead(t *testing.T) {
	e := newEnv(t)
	e.reportStart()
	const n = 12
	var wg sync.WaitGroup
	var mu sync.Mutex
	statuses := map[int]int{}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := e.client()
			kind, input := "astrology", reportSun
			if i%2 == 1 {
				kind, input = "path-of-life", reportNumbers
			}
			r := c.postReport(reportBody(kind, input, "same@example.com", map[string]any{"marketing_opt_in": i%3 == 0}), "")
			mu.Lock()
			statuses[r.status]++
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	if statuses[http.StatusAccepted] != n {
		t.Fatalf("statuses %v, want %d x 202", statuses, n)
	}
	if e.leadCount() != 1 {
		t.Fatalf("%d leads for one address", e.leadCount())
	}
	l := e.leadOf("same@example.com")
	if l.ReportsSent != n || len(l.Kinds) != 2 || l.MarketingStatus != domain.MarketingSubscribed {
		t.Fatalf("lead %+v", l)
	}
	var queued int64
	e.app.DB.Model(&domain.EmailOutbox{}).Where("template = ?", "calculator_report").Count(&queued)
	if queued != n {
		t.Fatalf("%d e-mails queued, want %d", queued, n)
	}
}

// The personal-data export includes the lead linked to the account: what was
// asked for and the marketing consent with its evidence, never the IP hash nor
// any birth detail.
func TestReportEmailLeadIsInThePersonalDataExport(t *testing.T) {
	e := newEnv(t)
	e.reportStart()
	me := e.registerClient()
	other := e.client()
	if r := other.postReport(reportBody("astrology", reportSun, "stranger@example.com", map[string]any{"marketing_opt_in": true}), ""); r.status != http.StatusAccepted {
		t.Fatal(r.status)
	}
	if r := me.postReport(reportBody("natal-chart", reportNatal, me.user.Email, map[string]any{"marketing_opt_in": true}), ""); r.status != http.StatusAccepted {
		t.Fatal(r.status)
	}
	var job service.ExportView
	me.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/exports", map[string]any{"password": strongPassword}, &job)
	if err := e.app.Scheduler.RunJob(context.Background(), app.ExportJob); err != nil {
		t.Fatal(err)
	}
	var link service.DownloadLink
	me.must(http.StatusOK, http.MethodPost, "/api/v1/me/exports/"+job.ID.String()+"/download-token", map[string]any{"password": strongPassword}, &link)
	files := readZip(t, me.raw(http.MethodGet, link.URL, nil, nil).body)
	doc, ok := files["calculator-reports.json"]
	if !ok {
		t.Fatalf("calculator-reports.json missing from %v", keysOf(files))
	}
	var entries []map[string]any
	if err := json.Unmarshal(doc, &entries); err != nil || len(entries) != 1 {
		t.Fatalf("calculator-reports.json: %s %v", doc, err)
	}
	if entries[0]["marketing_status"] != "subscribed" || entries[0]["reports_sent"] != float64(1) || entries[0]["first_calculator"] != "natal-chart" {
		t.Fatalf("entry %v", entries[0])
	}
	consent, _ := entries[0]["marketing_consent"].(map[string]any)
	if consent["consent_text_version"] != "2026-11-19" || consent["box_started_ticked"] != false {
		t.Fatalf("consent %v", consent)
	}
	for _, secret := range append([]string{"stranger@example.com", "ip_hash"}, birthSecrets...) {
		if strings.Contains(string(doc), secret) {
			t.Fatalf("the export leaks %q: %s", secret, doc)
		}
	}
}
