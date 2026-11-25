package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/ratelimit"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// reportMemLeads is an in-memory EmailLeadRepository.
type reportMemLeads struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]domain.EmailLead
	create int
}

func newReportMemLeads() *reportMemLeads {
	return &reportMemLeads{byID: map[uuid.UUID]domain.EmailLead{}}
}

func (r *reportMemLeads) ForUpdateByHash(_ context.Context, hash string) (*domain.EmailLead, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, l := range r.byID {
		if l.EmailHash == hash {
			l.Kinds = slices.Clone(l.Kinds)
			return &l, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *reportMemLeads) Create(_ context.Context, l *domain.EmailLead) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, have := range r.byID {
		if have.EmailHash == l.EmailHash {
			return repository.ErrLeadExists
		}
	}
	r.create++
	r.byID[l.ID] = *l
	return nil
}

func (r *reportMemLeads) Save(_ context.Context, l *domain.EmailLead) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	have, ok := r.byID[l.ID]
	if !ok {
		return repository.ErrNotFound
	}
	have.Locale, have.Country, have.Kinds, have.ReportsSent = l.Locale, l.Country, slices.Clone(l.Kinds), l.ReportsSent
	have.MarketingStatus, have.ConsentAt, have.ConsentPrechecked, have.ConsentTextVersion = l.MarketingStatus, l.ConsentAt, l.ConsentPrechecked, l.ConsentTextVersion
	have.ConsentIPHash, have.ConsentUserAgent, have.UnsubscribedAt, have.UserID, have.LastRequestAt = l.ConsentIPHash, l.ConsentUserAgent, l.UnsubscribedAt, l.UserID, l.LastRequestAt
	r.byID[l.ID] = have
	return nil
}

func (r *reportMemLeads) ForUpdate(ctx context.Context, id uuid.UUID) (*domain.EmailLead, error) {
	return r.Get(ctx, id)
}

func (r *reportMemLeads) Get(_ context.Context, id uuid.UUID) (*domain.EmailLead, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	l.Kinds = slices.Clone(l.Kinds)
	return &l, nil
}

func (r *reportMemLeads) List(_ context.Context, f repository.LeadFilter) ([]domain.EmailLead, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.EmailLead
	for _, l := range r.byID {
		if (f.Status != "" && l.MarketingStatus != f.Status) || (f.EmailHash != "" && l.EmailHash != f.EmailHash) ||
			(f.Kind != "" && !l.HasKind(f.Kind)) {
			continue
		}
		if f.Cursor != nil {
			if l.CreatedAt.After(f.Cursor.At) || (l.CreatedAt.Equal(f.Cursor.At) && l.ID.String() >= f.Cursor.ID) {
				continue
			}
		}
		out = append(out, l)
	}
	slices.SortFunc(out, func(a, b domain.EmailLead) int {
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return b.CreatedAt.Compare(a.CreatedAt)
		}
		return strings.Compare(b.ID.String(), a.ID.String())
	})
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (r *reportMemLeads) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[id]; !ok {
		return repository.ErrNotFound
	}
	delete(r.byID, id)
	return nil
}

func (r *reportMemLeads) LinkUser(_ context.Context, hash string, userID uuid.UUID) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, l := range r.byID {
		if l.EmailHash == hash && l.UserID == nil {
			l.UserID = &userID
			r.byID[id] = l
			return 1, nil
		}
	}
	return 0, nil
}

func (r *reportMemLeads) PurgeForUser(_ context.Context, userID uuid.UUID) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for id, l := range r.byID {
		if l.UserID != nil && *l.UserID == userID {
			delete(r.byID, id)
			n++
		}
	}
	return n, nil
}

func (r *reportMemLeads) all() []domain.EmailLead {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.EmailLead
	for _, l := range r.byID {
		out = append(out, l)
	}
	return out
}

// reportFakeMail records enqueued e-mail and knows a suppression list.
type reportFakeMail struct {
	mu         sync.Mutex
	sent       []EmailRequest
	suppressed map[string]bool
	err        error
}

func (m *reportFakeMail) AddressHash(email string) string {
	return "h:" + strings.ToLower(strings.TrimSpace(email))
}

func (m *reportFakeMail) Suppressed(_ context.Context, hash string) (bool, error) {
	return m.suppressed[hash], nil
}

func (m *reportFakeMail) Enqueue(_ context.Context, reqs ...EmailRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, reqs...)
	return nil
}

type reportUsers struct {
	repository.UserRepository
	users map[uuid.UUID]*domain.User
}

func (u reportUsers) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	if x, ok := u.users[id]; ok {
		return x, nil
	}
	return nil, repository.ErrNotFound
}

type reportFixture struct {
	svc      *CalculatorReportService
	leads    *reportMemLeads
	mail     *reportFakeMail
	audit    *siteAudit
	tracker  *analytics.Recorder
	clk      *clock.Fake
	users    map[uuid.UUID]*domain.User
	settings CalculatorReportSettings
	ctx      context.Context
}

func newReportFixture(t *testing.T) *reportFixture {
	t.Helper()
	astroSvc, _, clk := newAstro(t, nil)
	sealer, err := security.NewSealer(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	f := &reportFixture{leads: newReportMemLeads(), mail: &reportFakeMail{suppressed: map[string]bool{}}, audit: &siteAudit{},
		tracker: &analytics.Recorder{}, clk: clk, users: map[uuid.UUID]*domain.User{},
		settings: CalculatorReportSettings{Enabled: true, MarketingPrecheckedCountries: []string{"US"}},
		ctx:      reqctx.WithMeta(context.Background(), reqctx.Meta{IP: "203.0.113.9", UserAgent: "Mozilla/5.0\x00 (Test)\n Browser", Locale: "en"})}
	f.svc = NewCalculatorReportService(ReportDeps{Tx: inlineTx{}, Leads: f.leads, Users: reportUsers{users: f.users}, Astro: astroSvc, Mail: f.mail,
		Sealer: sealer, Audit: f.audit, Limiter: ratelimit.NewMemoryLimiter(clk.Now), Tracker: f.tracker, Clock: clk,
		Settings: func(context.Context) CalculatorReportSettings { return f.settings },
		Cfg:      ReportSettings{FrontendURL: "https://staryield.net", IPHashSecret: "ip-secret", UnsubscribeKey: "unsub-key"}})
	return f
}

const (
	natalInput   = `{"birth_date":"1990-03-14","birth_time":"08:45","place_id":"gn:2988507","house_system":"whole_sign","include":["tarot"]}`
	sunInput     = `{"birth_date":"1990-08-23"}`
	numbersInput = `{"birth_date":"1990-11-29","tz":"Europe/Paris"}`
)

func reportReq(kind, input, email string) ReportEmailInput {
	return ReportEmailInput{Kind: kind, Input: json.RawMessage(input), Email: email, Locale: "en"}
}

func (f *reportFixture) lead(t *testing.T, email string) domain.EmailLead {
	t.Helper()
	l, err := f.leads.ForUpdateByHash(f.ctx, f.mail.AddressHash(email))
	if err != nil {
		t.Fatalf("no lead for %s: %v", email, err)
	}
	return *l
}

func reportFields(t *testing.T, err error) map[string]string {
	t.Helper()
	e, ok := apperror.As(err)
	if !ok || e.Code != apperror.CodeValidation {
		t.Fatalf("want a validation error, got %v", err)
	}
	return reportValidationFields(e)
}

func TestReportQueuesTheRecalculatedReportAndStoresALeadWithoutBirthDetails(t *testing.T) {
	f := newReportFixture(t)
	for kind, input := range map[string]string{"natal-chart": natalInput, "astrology": sunInput, "path-of-life": numbersInput} {
		f.mail.sent = nil
		email := kind + "@example.com"
		in := reportReq(kind, input, " "+strings.ToUpper(email)+" ")
		in.Country = "FR"
		if err := f.svc.Request(f.ctx, in); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if len(f.mail.sent) != 1 {
			t.Fatalf("%s: %d e-mails queued", kind, len(f.mail.sent))
		}
		req := f.mail.sent[0]
		if req.To != email || req.Template != ReportEmailTemplate || req.Category != domain.EmailAccount || req.UserID != nil || req.Locale != "en" {
			t.Fatalf("%s: request %+v", kind, req)
		}
		if req.Content.Action == nil || !strings.HasPrefix(req.Content.Action.URL, "https://staryield.net/en/calculators/"+kind+"?utm_source=staryield&utm_medium=email&utm_campaign=calculator_report&utm_content="+kind+"#v=1&d=1990-") {
			t.Fatalf("%s: view link %+v", kind, req.Content.Action)
		}
		if len(req.Content.Sections) == 0 || req.Content.Subject == "" {
			t.Fatalf("%s: empty report %+v", kind, req.Content)
		}
		l := f.lead(t, email)
		if opened, err := f.svc.d.Sealer.Open(l.EmailSealed, leadAD(l.ID)); err != nil || opened != email {
			t.Fatalf("%s: sealed address %q %v", kind, opened, err)
		}
		if l.Source != kind || !l.HasKind(domain.ReportKind(kind)) || l.ReportsSent != 1 || l.Country != "FR" || l.Locale != "en" ||
			l.MarketingStatus != domain.MarketingNone || l.ConsentAt != nil {
			t.Fatalf("%s: lead %+v", kind, l)
		}
		// Birth details are never stored on the lead.
		dump, _ := json.Marshal(l)
		for _, secret := range []string{"1990", "08:45", "2988507", "03-14"} {
			if strings.Contains(string(dump), secret) {
				t.Fatalf("%s: the lead holds birth details (%s): %s", kind, secret, dump)
			}
		}
	}
	names := f.tracker.Names()
	if n := strings.Count(strings.Join(names, ","), "calculator.report_requested"); n != 3 {
		t.Fatalf("analytics: %v", names)
	}
	if strings.Contains(strings.Join(names, ","), "astro.") {
		t.Fatalf("a recalculation is not a new calculation: %v", names)
	}
}

func TestReportNatalDefaultsAndIncludes(t *testing.T) {
	f := newReportFixture(t)
	// The visitor's include list never changes the report; an invalid one is refused like the calculator does.
	if err := f.svc.Request(f.ctx, reportReq("natal-chart", `{"birth_date":"1990-03-14","include":["aspects"]}`, "a@example.com")); err != nil {
		t.Fatal(err)
	}
	err := f.svc.Request(f.ctx, reportReq("natal-chart", `{"birth_date":"1990-03-14","include":["tea-leaves"]}`, "a@example.com"))
	if got := reportFields(t, err); got["input.include"] == "" {
		t.Fatalf("fields %v", got)
	}
}

func TestReportValidation(t *testing.T) {
	f := newReportFixture(t)
	for name, tc := range map[string]struct {
		in    ReportEmailInput
		field string
	}{
		"unknown kind":      {reportReq("tarot", sunInput, "a@example.com"), "kind"},
		"no kind":           {reportReq("", sunInput, "a@example.com"), "kind"},
		"bad e-mail":        {reportReq("astrology", sunInput, "not-an-email"), "email"},
		"header injection":  {reportReq("astrology", sunInput, "a@example.com\r\nBcc: x@example.com"), "email"},
		"bad locale":        {ReportEmailInput{Kind: "astrology", Input: json.RawMessage(sunInput), Email: "a@example.com", Locale: "de"}, "locale"},
		"no input":          {ReportEmailInput{Kind: "astrology", Email: "a@example.com", Locale: "en"}, "input"},
		"null input":        {reportReq("astrology", `null`, "a@example.com"), "input"},
		"unknown key":       {reportReq("astrology", `{"birth_date":"1990-08-23","house_system":"equal"}`, "a@example.com"), "input"},
		"not an object":     {reportReq("astrology", `"1990-08-23"`, "a@example.com"), "input"},
		"trailing data":     {reportReq("astrology", `{"birth_date":"1990-08-23"}{}`, "a@example.com"), "input"},
		"oversized input":   {reportReq("astrology", `{"birth_date":"`+strings.Repeat("1", 3000)+`"}`, "a@example.com"), "input"},
		"bad birth date":    {reportReq("astrology", `{"birth_date":"1990-02-30"}`, "a@example.com"), "input.birth_date"},
		"future birth date": {reportReq("path-of-life", `{"birth_date":"2999-01-01"}`, "a@example.com"), "input.birth_date"},
		"time needs place":  {reportReq("natal-chart", `{"birth_date":"1990-03-14","birth_time":"08:45"}`, "a@example.com"), "input.place_id"},
		"unknown place":     {reportReq("natal-chart", `{"birth_date":"1990-03-14","place_id":"gn:1"}`, "a@example.com"), "input.place_id"},
		"bad time":          {reportReq("astrology", `{"birth_date":"1990-03-14","birth_time":"8h45","place_id":"gn:2988507"}`, "a@example.com"), "input.birth_time"},
		"bad house system":  {reportReq("natal-chart", `{"birth_date":"1990-03-14","house_system":"kooky"}`, "a@example.com"), "input.house_system"},
		"bad zone":          {reportReq("path-of-life", `{"birth_date":"1990-03-14","tz":"Mars/Olympus"}`, "a@example.com"), "input.tz"},
	} {
		err := f.svc.Request(f.ctx, tc.in)
		if got := reportFields(t, err); got[tc.field] == "" {
			t.Errorf("%s: fields %v, want one on %q", name, got, tc.field)
		}
	}
	// Several problems are reported together.
	err := f.svc.Request(f.ctx, reportReq("astrology", `{"birth_date":"nope"}`, "bad"))
	if got := reportFields(t, err); got["email"] == "" || got["input.birth_date"] == "" {
		t.Fatalf("fields %v", got)
	}
	if len(f.mail.sent) != 0 || len(f.leads.all()) != 0 {
		t.Fatal("an invalid request stored or sent something")
	}
	// An empty locale falls back to the request's.
	in := reportReq("astrology", sunInput, "fr@example.com")
	in.Locale = ""
	f.ctx = reqctx.WithMeta(f.ctx, reqctx.Meta{Locale: "fr-CA"})
	if err := f.svc.Request(f.ctx, in); err != nil {
		t.Fatal(err)
	}
	if f.mail.sent[0].Locale != "fr" || f.lead(t, "fr@example.com").Locale != "fr" {
		t.Fatalf("locale %q", f.mail.sent[0].Locale)
	}
}

func TestReportHoneypotAndSwitch(t *testing.T) {
	f := newReportFixture(t)
	in := reportReq("astrology", sunInput, "bot@example.com")
	in.Website = "https://spam.example"
	if err := f.svc.Request(f.ctx, in); err != nil {
		t.Fatalf("a filled honeypot gets the 202: %v", err)
	}
	if len(f.mail.sent) != 0 || len(f.leads.all()) != 0 {
		t.Fatal("the honeypot stored or sent something")
	}
	// Even an invalid request with the honeypot filled is answered the same way.
	bad := ReportEmailInput{Website: " x "}
	if err := f.svc.Request(f.ctx, bad); err != nil {
		t.Fatal(err)
	}
	f.settings.Enabled = false
	if err := f.svc.Request(f.ctx, in); err != nil {
		t.Fatalf("the honeypot answers before the switch: %v", err)
	}
	err := f.svc.Request(f.ctx, reportReq("astrology", sunInput, "a@example.com"))
	if e, ok := apperror.As(err); !ok || e.Code != apperror.CodeFeatureUnavailable || e.Status != 503 {
		t.Fatalf("switched off: %v", err)
	}
	if len(f.mail.sent) != 0 || len(f.leads.all()) != 0 {
		t.Fatal("a switched-off feature stored or sent something")
	}
}

func TestReportSuppressedAddressGetsTheSameAnswerAndNothingIsSent(t *testing.T) {
	f := newReportFixture(t)
	f.mail.suppressed[f.mail.AddressHash("bounced@example.com")] = true
	if err := f.svc.Request(f.ctx, reportReq("astrology", sunInput, "Bounced@Example.com")); err != nil {
		t.Fatal(err)
	}
	if len(f.mail.sent) != 0 || len(f.leads.all()) != 0 {
		t.Fatal("a suppressed address must not be e-mailed or stored")
	}
}

func TestReportAddressLimit(t *testing.T) {
	f := newReportFixture(t)
	for i := 0; i < 3; i++ {
		if err := f.svc.Request(f.ctx, reportReq("astrology", sunInput, "busy@example.com")); err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
	}
	err := f.svc.Request(f.ctx, reportReq("astrology", sunInput, "BUSY@example.com"))
	e, ok := apperror.As(err)
	if !ok || e.Code != apperror.CodeRateLimited || e.RetryAfter <= 0 {
		t.Fatalf("fourth request: %v", err)
	}
	if err := f.svc.Request(f.ctx, reportReq("astrology", sunInput, "other@example.com")); err != nil {
		t.Fatalf("another address is unaffected: %v", err)
	}
	if got := f.lead(t, "busy@example.com").ReportsSent; got != 3 || len(f.mail.sent) != 4 {
		t.Fatalf("reports sent %d, queued %d", got, len(f.mail.sent))
	}
	// An invalid request spends nothing; the budget refills over a day.
	f.clk.Advance(24 * time.Hour)
	if err := f.svc.Request(f.ctx, reportReq("astrology", sunInput, "busy@example.com")); err != nil {
		t.Fatalf("after a day: %v", err)
	}
}

func TestReportRepeatRequestsMergeIntoOneLead(t *testing.T) {
	f := newReportFixture(t)
	first := reportReq("natal-chart", natalInput, "same@example.com")
	first.Country = "FR"
	if err := f.svc.Request(f.ctx, first); err != nil {
		t.Fatal(err)
	}
	f.clk.Advance(time.Hour)
	second := reportReq("path-of-life", numbersInput, "Same@example.com")
	second.Locale, second.Country = "es", ""
	if err := f.svc.Request(f.ctx, second); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Request(f.ctx, reportReq("path-of-life", numbersInput, "same@example.com")); err != nil {
		t.Fatal(err)
	}
	if n := len(f.leads.all()); n != 1 {
		t.Fatalf("%d leads", n)
	}
	l := f.lead(t, "same@example.com")
	if l.Source != "natal-chart" || !slices.Equal(l.Kinds, domain.StringList{"natal-chart", "path-of-life"}) || l.ReportsSent != 3 {
		t.Fatalf("lead %+v", l)
	}
	if l.Country != "FR" || l.Locale != "en" || !l.LastRequestAt.Equal(f.clk.Now()) {
		t.Fatalf("an empty country keeps the known one: %+v", l)
	}
}

func TestReportConsentEvidence(t *testing.T) {
	type step struct {
		optIn, prechecked bool
		country           string
		want              domain.MarketingStatus
	}
	for name, steps := range map[string][]step{
		"ticked by the visitor":                  {{true, false, "FR", domain.MarketingSubscribed}},
		"ticked box in a pre-ticked country":     {{true, true, "US", domain.MarketingSubscribed}},
		"pre-ticked box outside the list":        {{true, true, "FR", domain.MarketingNone}},
		"pre-ticked box with an unknown country": {{true, true, "", domain.MarketingNone}},
		"not ticked":                             {{false, false, "FR", domain.MarketingNone}},
		"unticking later never unsubscribes":     {{true, false, "FR", domain.MarketingSubscribed}, {false, false, "FR", domain.MarketingSubscribed}},
	} {
		t.Run(name, func(t *testing.T) {
			f := newReportFixture(t)
			for _, s := range steps {
				in := reportReq("astrology", sunInput, "consent@example.com")
				in.MarketingOptIn, in.MarketingPrechecked, in.Country = s.optIn, s.prechecked, s.country
				if err := f.svc.Request(f.ctx, in); err != nil {
					t.Fatal(err)
				}
				if got := f.lead(t, "consent@example.com").MarketingStatus; got != s.want {
					t.Fatalf("status %q, want %q", got, s.want)
				}
			}
		})
	}

	f := newReportFixture(t)
	in := reportReq("astrology", sunInput, "evidence@example.com")
	in.MarketingOptIn, in.MarketingPrechecked, in.Country = true, true, "US"
	if err := f.svc.Request(f.ctx, in); err != nil {
		t.Fatal(err)
	}
	l := f.lead(t, "evidence@example.com")
	if l.ConsentAt == nil || !l.ConsentAt.Equal(f.clk.Now()) || l.ConsentPrechecked == nil || !*l.ConsentPrechecked ||
		l.ConsentTextVersion == nil || *l.ConsentTextVersion != "2026-11-19" {
		t.Fatalf("evidence %+v", l)
	}
	if l.ConsentIPHash == nil || *l.ConsentIPHash != securityKeyed("ip-secret", "203.0.113.9") || strings.Contains(*l.ConsentIPHash, "203.0.113") {
		t.Fatalf("ip hash %v", l.ConsentIPHash)
	}
	if l.ConsentUserAgent == nil || *l.ConsentUserAgent != "Mozilla/5.0 (Test) Browser" {
		t.Fatalf("user agent %v", l.ConsentUserAgent)
	}
	// A second request keeps the first evidence.
	f.clk.Advance(time.Hour)
	again := reportReq("astrology", sunInput, "evidence@example.com")
	again.MarketingOptIn, again.Country = true, "FR"
	if err := f.svc.Request(f.ctx, again); err != nil {
		t.Fatal(err)
	}
	if got := f.lead(t, "evidence@example.com"); !got.ConsentAt.Equal(*l.ConsentAt) || !*got.ConsentPrechecked {
		t.Fatalf("evidence replaced: %+v", got)
	}
}

func TestReportUnsubscribeLinkAndResubscription(t *testing.T) {
	f := newReportFixture(t)
	f.svc.d.Cfg.AddressRule = ReportAddressRule.Scaled(1000) // this test sends many reports to one address
	request := func(optIn, prechecked bool) {
		t.Helper()
		in := reportReq("astrology", sunInput, "toggle@example.com")
		in.MarketingOptIn, in.MarketingPrechecked, in.Country = optIn, prechecked, "US"
		if err := f.svc.Request(f.ctx, in); err != nil {
			t.Fatal(err)
		}
	}
	unsubscribeLink := func() string {
		t.Helper()
		last := f.mail.sent[len(f.mail.sent)-1].Content
		if len(last.Links) == 0 {
			return ""
		}
		return strings.TrimPrefix(last.Links[0].URL, "/unsubscribe?token=")
	}
	request(false, false)
	if unsubscribeLink() != "" {
		t.Fatal("an address that never subscribed gets no unsubscribe link")
	}
	request(true, true)
	token := unsubscribeLink()
	if token == "" {
		t.Fatal("a subscribed address gets the unsubscribe link")
	}
	if err := f.svc.Unsubscribe(f.ctx, token); err != nil {
		t.Fatal(err)
	}
	l := f.lead(t, "toggle@example.com")
	if l.MarketingStatus != domain.MarketingUnsubscribed || l.UnsubscribedAt == nil {
		t.Fatalf("lead %+v", l)
	}
	// Idempotent: the second use changes nothing.
	was := *l.UnsubscribedAt
	f.clk.Advance(time.Hour)
	if err := f.svc.Unsubscribe(f.ctx, token); err != nil {
		t.Fatal(err)
	}
	if got := f.lead(t, "toggle@example.com"); !got.UnsubscribedAt.Equal(was) {
		t.Fatal("a repeated unsubscribe moved the date")
	}
	// A box that started ticked never re-subscribes an address that unsubscribed ...
	request(true, true)
	if got := f.lead(t, "toggle@example.com"); got.MarketingStatus != domain.MarketingUnsubscribed || unsubscribeLink() != "" {
		t.Fatalf("a pre-ticked box re-subscribed: %+v", got)
	}
	// ... only a box the visitor ticked does.
	request(true, false)
	got := f.lead(t, "toggle@example.com")
	if got.MarketingStatus != domain.MarketingSubscribed || got.UnsubscribedAt != nil || *got.ConsentPrechecked || unsubscribeLink() == "" {
		t.Fatalf("lead %+v", got)
	}
}

func TestUnsubscribeTokens(t *testing.T) {
	f := newReportFixture(t)
	if err := f.svc.Request(f.ctx, reportReq("astrology", sunInput, "tok@example.com")); err != nil {
		t.Fatal(err)
	}
	l := f.lead(t, "tok@example.com")
	token := f.svc.UnsubscribeToken(l.ID)
	if id, ok := parseUnsubscribeToken("unsub-key", token); !ok || id != l.ID {
		t.Fatal("a token verifies with its key")
	}
	idPart, macPart, _ := strings.Cut(token, ".")
	other := unsubscribeToken("another-key", l.ID)
	flipped := idPart + "." + macPart[:len(macPart)-1] + map[bool]string{true: "B", false: "A"}[macPart[len(macPart)-1] == 'A']
	for name, bad := range map[string]string{
		"empty": "", "garbage": "!!!", "no dot": idPart, "other key": other, "flipped mac": flipped, "other id": unsubscribeToken("unsub-key", uuid.New()),
		"truncated id": idPart[:10] + "." + macPart, "too long": strings.Repeat("a", 200) + "." + macPart,
	} {
		err := f.svc.Unsubscribe(f.ctx, bad)
		if e, ok := apperror.As(err); name == "other id" {
			// A correctly signed token for a lead that does not exist is the same 404.
			if !ok || e.Status != 404 {
				t.Errorf("%s: %v", name, err)
			}
			continue
		} else if !ok || e.Status != 404 || e.Code != apperror.CodeNotFound {
			t.Errorf("%s: %v", name, err)
		}
	}
	if f.lead(t, "tok@example.com").MarketingStatus != domain.MarketingNone {
		t.Fatal("a bad token changed the lead")
	}
	// A deleted lead's token is a 404 too.
	if err := f.svc.DeleteLead(f.ctx, l.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Unsubscribe(f.ctx, token); err == nil {
		t.Fatal("unsubscribe of a deleted lead")
	}
}

func TestReportLinksTheSignedInAccountWhenItUsesTheAddress(t *testing.T) {
	f := newReportFixture(t)
	user := &domain.User{ID: uuid.New(), Email: "me@example.com"}
	f.users[user.ID] = user
	signedIn := reqctx.WithPrincipal(f.ctx, reqctx.Principal{UserID: user.ID, Role: domain.RoleClient})
	if err := f.svc.Request(signedIn, reportReq("astrology", sunInput, "Me@Example.com")); err != nil {
		t.Fatal(err)
	}
	if l := f.lead(t, "me@example.com"); l.UserID == nil || *l.UserID != user.ID {
		t.Fatalf("lead %+v", l)
	}
	// Another address typed by a signed-in visitor is not the account's.
	if err := f.svc.Request(signedIn, reportReq("astrology", sunInput, "friend@example.com")); err != nil {
		t.Fatal(err)
	}
	if l := f.lead(t, "friend@example.com"); l.UserID != nil {
		t.Fatalf("a friend's address was linked: %+v", l)
	}
	if f.mail.sent[0].UserID != nil {
		t.Fatal("the report is not tied to the account's e-mail cap")
	}
}

func TestReportRegistrationLinksAnExistingLead(t *testing.T) {
	f := newReportFixture(t)
	if err := f.svc.Request(f.ctx, reportReq("astrology", sunInput, "later@example.com")); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Request(f.ctx, reportReq("astrology", sunInput, "other@example.com")); err != nil {
		t.Fatal(err)
	}
	user := &domain.User{ID: uuid.New(), Email: "Later@Example.com"}
	f.svc.LinkRegistration(f.ctx, user)
	f.svc.LinkRegistration(f.ctx, nil)
	if l := f.lead(t, "later@example.com"); l.UserID == nil || *l.UserID != user.ID {
		t.Fatalf("lead %+v", l)
	}
	if l := f.lead(t, "other@example.com"); l.UserID != nil {
		t.Fatal("the wrong lead was linked")
	}
	// A new account never takes over a lead another account already holds.
	thief := &domain.User{ID: uuid.New(), Email: "later@example.com"}
	f.svc.LinkRegistration(f.ctx, thief)
	if l := f.lead(t, "later@example.com"); *l.UserID != user.ID {
		t.Fatal("a linked lead was taken over")
	}
	// Erasing the account deletes its lead only.
	n, err := PurgeAll(f.svc).PurgeForUser(f.ctx, user.ID)
	if err != nil || n != 1 {
		t.Fatalf("purged %d %v", n, err)
	}
	if len(f.leads.all()) != 1 {
		t.Fatal("the other lead must stay")
	}
}

func TestPurgeAllStopsOnTheFirstError(t *testing.T) {
	boom := errors.New("boom")
	calls := 0
	p := PurgeAll(purgerFunc(func() (int64, error) { calls++; return 2, nil }), purgerFunc(func() (int64, error) { calls++; return 1, boom }),
		purgerFunc(func() (int64, error) { calls++; return 5, nil }))
	n, err := p.PurgeForUser(context.Background(), uuid.New())
	if !errors.Is(err, boom) || n != 3 || calls != 2 {
		t.Fatalf("n=%d err=%v calls=%d", n, err, calls)
	}
}

type purgerFunc func() (int64, error)

func (p purgerFunc) PurgeForUser(context.Context, uuid.UUID) (int64, error) { return p() }

func TestReportEnqueueFailureIsReported(t *testing.T) {
	f := newReportFixture(t)
	f.mail.err = errors.New("outbox down")
	err := f.svc.Request(f.ctx, reportReq("astrology", sunInput, "x@example.com"))
	if e, ok := apperror.As(err); !ok || e.Status != 500 {
		t.Fatalf("err %v", err)
	}
}

func TestReportConfigByCountry(t *testing.T) {
	f := newReportFixture(t)
	for country, want := range map[string]bool{"US": true, "us": true, "FR": false, "": false, "ZZZ": false} {
		if got := f.svc.Config(f.ctx, country); !got.Enabled || got.MarketingPrechecked != want {
			t.Errorf("%q: %+v", country, got)
		}
	}
	f.settings.Enabled = false
	if got := f.svc.Config(f.ctx, "US"); got.Enabled || got.MarketingPrechecked {
		t.Fatalf("switched off: %+v", got)
	}
}

func TestAdminLeadsListFilterExportAndDelete(t *testing.T) {
	f := newReportFixture(t)
	admin := reqctx.WithPrincipal(f.ctx, reqctx.Principal{UserID: uuid.New(), Role: domain.RoleAdmin})
	add := func(email, kind string, optIn bool) {
		t.Helper()
		in := reportReq(kind, map[string]string{"natal-chart": natalInput, "astrology": sunInput, "path-of-life": numbersInput}[kind], email)
		in.MarketingOptIn, in.Country = optIn, "FR"
		if err := f.svc.Request(f.ctx, in); err != nil {
			t.Fatal(err)
		}
		f.clk.Advance(time.Minute)
	}
	add("a@example.com", "astrology", true)
	add("b@example.com", "natal-chart", false)
	add("+plus@example.com", "path-of-life", true)
	add("=cmd@example.com", "astrology", true) // formula-looking local part
	add("d@example.com", "astrology", true)
	if err := f.svc.Unsubscribe(f.ctx, f.svc.UnsubscribeToken(f.lead(t, "d@example.com").ID)); err != nil {
		t.Fatal(err)
	}

	all, err := f.svc.Leads(admin, LeadQuery{Limit: 2})
	if err != nil || len(all.Items) != 2 || all.NextCursor == "" {
		t.Fatalf("page 1: %+v %v", all, err)
	}
	if all.Items[0].Email != "d@example.com" || all.Items[0].MarketingStatus != "unsubscribed" || all.Items[0].UnsubscribedAt == nil {
		t.Fatalf("newest first: %+v", all.Items[0])
	}
	var seen []string
	for q := (LeadQuery{Limit: 2}); ; {
		page, err := f.svc.Leads(admin, q)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range page.Items {
			seen = append(seen, it.Email)
		}
		if page.NextCursor == "" {
			break
		}
		q.Cursor = page.NextCursor
	}
	if len(seen) != 5 {
		t.Fatalf("paged %v", seen)
	}
	sub, _ := f.svc.Leads(admin, LeadQuery{Status: "subscribed"})
	if len(sub.Items) != 3 {
		t.Fatalf("subscribed: %d", len(sub.Items))
	}
	if it := sub.Items[2]; it.SubscribedAt == nil || it.Prechecked == nil || *it.Prechecked || it.Source != "astrology" || it.ReportsSent != 1 {
		t.Fatalf("subscribed view %+v", it)
	}
	if kind, _ := f.svc.Leads(admin, LeadQuery{Kind: "natal-chart"}); len(kind.Items) != 1 || kind.Items[0].Email != "b@example.com" {
		t.Fatalf("kind filter: %+v", kind.Items)
	}
	if q, _ := f.svc.Leads(admin, LeadQuery{Q: " B@Example.com "}); len(q.Items) != 1 || q.Items[0].Email != "b@example.com" {
		t.Fatalf("q: %+v", q.Items)
	}
	if q, err := f.svc.Leads(admin, LeadQuery{Q: "nobody@example.com"}); err != nil || len(q.Items) != 0 || q.Items == nil {
		t.Fatalf("unknown address: %+v %v", q, err)
	}
	for _, bad := range []LeadQuery{{Status: "gone"}, {Kind: "tarot"}, {Q: "b@"}, {Cursor: "%%%"}} {
		if _, err := f.svc.Leads(admin, bad); err == nil {
			t.Errorf("%+v accepted", bad)
		}
	}

	// CSV export: subscribed by default, formulas neutralised, audited with the row count.
	filter, err := f.svc.LeadExportFilter("", "")
	if err != nil || filter.Status != domain.MarketingSubscribed {
		t.Fatalf("default filter %+v %v", filter, err)
	}
	var buf bytes.Buffer
	n, err := f.svc.ExportLeads(admin, filter, &buf)
	if err != nil || n != 3 {
		t.Fatalf("exported %d %v", n, err)
	}
	rows, err := csv.NewReader(&buf).ReadAll()
	if err != nil || len(rows) != 4 || strings.Join(rows[0], ",") != "email,locale,country,subscribed_at,source" {
		t.Fatalf("csv %v %v", rows, err)
	}
	var emails []string
	for _, r := range rows[1:] {
		emails = append(emails, r[0])
		if r[2] != "FR" || r[1] != "en" || r[3] == "" {
			t.Fatalf("row %v", r)
		}
	}
	if !slices.Contains(emails, "a@example.com") || !slices.Contains(emails, "'+plus@example.com") || !slices.Contains(emails, "'=cmd@example.com") ||
		slices.Contains(emails, "d@example.com") || slices.Contains(emails, "b@example.com") {
		t.Fatalf("exported %v", emails)
	}
	last := f.audit.rows[len(f.audit.rows)-1]
	if last.Action != "marketing.leads.exported" || last.Metadata["rows"] != 3 || last.Metadata["status"] != "subscribed" {
		t.Fatalf("audit %+v", last)
	}
	if _, err := f.svc.LeadExportFilter("everything", ""); err == nil {
		t.Fatal("unknown export status accepted")
	}

	// Large exports page through the keyset.
	for i := 0; i < leadExportBatch+7; i++ {
		f.leads.mustAdd(f, i)
	}
	buf.Reset()
	if n, err := f.svc.ExportLeads(admin, filter, &buf); err != nil || n != 3+leadExportBatch+7 {
		t.Fatalf("large export: %d %v", n, err)
	}

	// Deleting a lead is an erasure: gone, audited without the address.
	target := f.lead(t, "b@example.com")
	if err := f.svc.DeleteLead(admin, target.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.leads.Get(admin, target.ID); err == nil {
		t.Fatal("lead not deleted")
	}
	last = f.audit.rows[len(f.audit.rows)-1]
	if last.Action != "marketing.lead.deleted" || last.TargetID != target.ID.String() || strings.Contains(strings.ToLower(toJSON(last)), "b@example.com") {
		t.Fatalf("audit %+v", last)
	}
	if err := f.svc.DeleteLead(admin, target.ID); !isCode(err, apperror.CodeNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}

func toJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

// mustAdd inserts a subscribed lead directly (bulk data for the export test).
func (r *reportMemLeads) mustAdd(f *reportFixture, i int) {
	id := domain.NewID()
	email := "bulk" + strings.Repeat("x", i%5) + uuid.NewString()[:8] + "@example.com"
	sealed, _ := f.svc.d.Sealer.Seal(email, leadAD(id))
	now := f.clk.Now().Add(-time.Duration(i+1) * time.Second)
	version, prechecked := CalculatorReportConsentVersion, false
	r.byID[id] = domain.EmailLead{ID: id, EmailHash: "h:" + email, EmailSealed: sealed, Locale: "en", Source: "astrology", Kinds: domain.StringList{"astrology"},
		MarketingStatus: domain.MarketingSubscribed, ConsentAt: &now, ConsentPrechecked: &prechecked, ConsentTextVersion: &version,
		LastRequestAt: now, Timestamps: domain.Timestamps{CreatedAt: now, UpdatedAt: now}}
}

func TestExportStopsWhenTheClientLeaves(t *testing.T) {
	f := newReportFixture(t)
	for i := 0; i < 3; i++ {
		f.leads.mustAdd(f, i)
	}
	filter, _ := f.svc.LeadExportFilter("", "")
	n, err := f.svc.ExportLeads(f.ctx, filter, failingWriter{})
	if err == nil {
		t.Fatalf("a failed write must end the export (%d rows)", n)
	}
	if len(f.audit.rows) != 1 || f.audit.rows[0].Action != "marketing.leads.exported" {
		t.Fatalf("the partial export is still audited: %+v", f.audit.rows)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("client gone") }

// The path of life report prints the numbers the web computes in the browser:
// both are checked against the shared vectors (internal/astro/testdata), so a
// report can never disagree with the page it was sent from.
func TestReportNumbersMatchTheSharedVectors(t *testing.T) {
	raw, err := os.ReadFile("../astro/testdata/numerology_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Vectors []struct {
			BirthDate      string `json:"birth_date"`
			AsOf           string `json:"as_of"`
			LifePath       int    `json:"life_path"`
			LifePathMaster bool   `json:"life_path_master"`
			PersonalYear   int    `json:"personal_year"`
			PersonalMonth  int    `json:"personal_month"`
		} `json:"vectors"`
	}
	if err := json.Unmarshal(raw, &fx); err != nil || len(fx.Vectors) < 20 {
		t.Fatalf("vectors: %d %v", len(fx.Vectors), err)
	}
	f := newReportFixture(t)
	for _, v := range fx.Vectors {
		body, _ := json.Marshal(map[string]string{"birth_date": v.BirthDate, "as_of": v.AsOf})
		built, err := f.svc.build(domain.ReportPathOfLife, body)
		if err != nil {
			t.Fatalf("%s: %v", v.BirthDate, err)
		}
		n := built.data.Numbers
		if n.LifePath.Number != v.LifePath || n.LifePath.Master != v.LifePathMaster || n.PersonalYear != v.PersonalYear || n.PersonalMonth != v.PersonalMonth ||
			n.AsOf != v.AsOf {
			t.Errorf("%s as of %s: report has %d/%d/%d (as of %s), vectors say %d/%d/%d", v.BirthDate, v.AsOf, n.LifePath.Number, n.PersonalYear,
				n.PersonalMonth, n.AsOf, v.LifePath, v.PersonalYear, v.PersonalMonth)
		}
	}
}

func TestReportViewLinkReopensTheSameResult(t *testing.T) {
	f := newReportFixture(t)
	for name, tc := range map[string]struct{ kind, input, want string }{
		"natal with time and place": {"natal-chart", natalInput, "/calculators/natal-chart#v=1&d=1990-03-14&t=08%3A45&p=gn%3A2988507&h=whole_sign"},
		"natal default houses":      {"natal-chart", `{"birth_date":"1990-03-14","place_id":"gn:2988507"}`, "/calculators/natal-chart#v=1&d=1990-03-14&p=gn%3A2988507"},
		"natal date only":           {"natal-chart", `{"birth_date":"1990-03-14","include":["tarot","aspects"]}`, "/calculators/natal-chart#v=1&d=1990-03-14"},
		"astrology with place":      {"astrology", `{"birth_date":"1990-08-23","birth_time":"00:05","place_id":"gn:2988507"}`, "/calculators/astrology#v=1&d=1990-08-23&t=00%3A05&p=gn%3A2988507"},
		"path of life":              {"path-of-life", `{"birth_date":"1990-11-29","tz":"Europe/Paris"}`, "/calculators/path-of-life#v=1&d=1990-11-29"},
	} {
		built, err := f.svc.build(domain.ReportKind(tc.kind), []byte(tc.input))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if built.data.ViewPath != tc.want {
			t.Errorf("%s: %q, want %q", name, built.data.ViewPath, tc.want)
		}
	}
}

func TestReportContentOfEachKind(t *testing.T) {
	f := newReportFixture(t)
	natal, err := f.svc.build(domain.ReportNatalChart, []byte(natalInput))
	if err != nil {
		t.Fatal(err)
	}
	n := natal.data.Natal
	if len(n.Spread.Cards) != 12 || !n.TimeKnown || n.HouseSystem != "whole_sign" || n.BirthTime != "08:45" || !strings.HasPrefix(n.PlaceLabel, "Paris") ||
		natal.data.Attribution == "" || natal.data.BirthDate != "1990-03-14" {
		t.Fatalf("natal %+v", natal.data)
	}
	// Without a birth time the spread is uncertain and the houses are whole-sign from the Sun.
	unknown, err := f.svc.build(domain.ReportNatalChart, []byte(`{"birth_date":"1990-03-14"}`))
	if err != nil {
		t.Fatal(err)
	}
	if u := unknown.data.Natal; u.TimeKnown || u.PlaceLabel != "" || len(u.Spread.Cards) != 12 {
		t.Fatalf("unknown time %+v", u)
	}
	// A cusp birthday without time is flagged.
	sun, err := f.svc.build(domain.ReportAstrology, []byte(`{"birth_date":"1990-08-23"}`))
	if err != nil {
		t.Fatal(err)
	}
	if s := sun.data.Sun; s.Sign == "" || s.Element == "" || s.Modality == "" || len(s.Alternatives) != 2 && s.Uncertain {
		t.Fatalf("sun %+v", s)
	}
}
