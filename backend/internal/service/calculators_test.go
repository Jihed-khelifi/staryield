package service

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

var calcNow = time.Date(2026, 11, 7, 15, 30, 0, 0, time.UTC)

func TestCalculatorPathRegex(t *testing.T) {
	good := []string{
		"/en/calculators/natal-chart", "/fr/calculators/natal-chart", "/es/calculators/natal-chart",
		"/en/calculators/path-of-life", "/fr/calculators/path-of-life/results", "/es/calculators/astrology",
		"/en/calculators/astrology/results",
	}
	for _, p := range good {
		if CalculatorKindOfPath(p) == "" {
			t.Errorf("%s rejected", p)
		}
	}
	if CalculatorKindOfPath("/fr/calculators/path-of-life/results") != "path-of-life" {
		t.Fatal("kind of results page")
	}
	bad := []string{
		"", "/", "/en", "/en/calculators", "/en/calculators/", "/de/calculators/natal-chart", "/EN/calculators/natal-chart",
		"/calculators/natal-chart", "/en/calculators/tarot", "/en/calculators/natal-chart/", "/en/calculators/natal-chart/result",
		"/en/calculators/natal-chart/results/extra", "/en/calculators/natal-chart?x=1", "/en/calculators/natal-chart#top",
		"/en/psychics", "/en/calculators/natal-chartx", "/xx/en/calculators/natal-chart", "https://staryield.net/en/calculators/natal-chart",
		"/en/calculators/natal-chart\n", "/en//calculators/natal-chart",
	}
	for _, p := range bad {
		if k := CalculatorKindOfPath(p); k != "" {
			t.Errorf("%q accepted as %s", p, k)
		}
	}
}

func TestClickIDSanitisation(t *testing.T) {
	for _, ok := range []string{"a", "AbC_123-x", strings.Repeat("a", 128), uuid.NewString()} {
		props := domain.JSONMap{"click_id": ok}
		if sanitizeClickID(props) != ok || props["click_id"] != ok {
			t.Errorf("%q dropped", ok)
		}
	}
	for _, bad := range []any{"", "has space", "a/b", "<script>", strings.Repeat("a", 129), "é", 5, nil, "a\nb"} {
		props := domain.JSONMap{"click_id": bad, "other": "x"}
		if sanitizeClickID(props) != "" {
			t.Errorf("%v accepted", bad)
		}
		if _, still := props["click_id"]; still {
			t.Errorf("%v not removed from the stored properties", bad)
		}
		if props["other"] != "x" {
			t.Error("other properties must be kept")
		}
	}
}

type captureSink struct{ events []domain.VisitorEvent }

func (c *captureSink) EnqueueVisitor(e domain.VisitorEvent) bool {
	c.events = append(c.events, e)
	return true
}

func ingestSvc(sink analytics.VisitorSink) *AnalyticsService {
	return NewAnalyticsService(&analytics.Recorder{}, nil, nil, clock.NewFake(calcNow), "secret", 50).WithVisitors(sink)
}

func ev(name, anon, path string, props map[string]any) ClientEvent {
	return ClientEvent{EventID: uuid.NewString(), Name: name, AnonymousID: anon, Path: path, Locale: "fr", Properties: props, OccurredAt: calcNow}
}

func TestIngestFeedsVisitorsAfterValidation(t *testing.T) {
	sink := &captureSink{}
	svc := ingestSvc(sink)
	click := uuid.NewString()
	events := []ClientEvent{
		ev("page_view", "anon-visitor-1", "/fr/calculators/natal-chart?utm_source=x", nil),
		ev("page_view", "anon-visitor-1", "/fr/psychics", nil),        // not a calculator
		ev("page_view", "bad id", "/fr/calculators/natal-chart", nil), // invalid anonymous id
		ev("page_view", "", "/fr/calculators/natal-chart", nil),       // no anonymous id
		ev("calculator_started", "anon-visitor-1", "/fr/calculators/natal-chart", map[string]any{"kind": "natal-chart"}),
		ev("calculator_completed", "anon-visitor-1", "/fr/calculators/natal-chart", map[string]any{"kind": "astrology"}),
		ev("calculator_completed", "anon-visitor-1", "/fr/calculators/natal-chart", map[string]any{"kind": "tarot"}), // invalid kind
		ev("calculator_started", "anon-visitor-1", "/fr/calculators/natal-chart", nil),                               // no kind
		ev("campaign_landing", "anon-visitor-2", "/en", map[string]any{"utm_source": "instagram", "utm_medium": "<b>", "click_id": click}),
		ev("campaign_landing", "anon-visitor-3", "/en", map[string]any{"utm_source": "tiktok", "click_id": "bad id!"}),
		ev("chat_opened", "anon-visitor-4", "/en/chat", nil),
	}
	n, err := svc.Ingest(context.Background(), events)
	if err != nil || n != len(events) {
		t.Fatalf("accepted %d: %v", n, err)
	}
	var types []domain.VisitorEventType
	for _, e := range sink.events {
		types = append(types, e.Type)
	}
	want := []domain.VisitorEventType{domain.VisitView, domain.VisitStart, domain.VisitComplete, domain.VisitLanding, domain.VisitLanding}
	if len(types) != len(want) {
		t.Fatalf("visitor events: %v", types)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("visitor events: %v", types)
		}
	}
	if sink.events[0].Kind != "natal-chart" || sink.events[0].Locale != "fr" || sink.events[0].SignedIn || sink.events[0].UserID != nil {
		t.Fatalf("view: %+v", sink.events[0])
	}
	if sink.events[2].Kind != "astrology" {
		t.Fatalf("completed kind: %+v", sink.events[2])
	}
	landing := sink.events[3]
	if landing.UTMSource != "instagram" || landing.UTMMedium != "" || landing.ClickID != click {
		t.Fatalf("landing: %+v", landing)
	}
	if bad := sink.events[4]; bad.ClickID != "" || bad.UTMSource != "tiktok" {
		t.Fatalf("landing with invalid click id: %+v", bad)
	}
}

func TestIngestLinksSignedInVisitors(t *testing.T) {
	sink := &captureSink{}
	svc := ingestSvc(sink)
	uid := uuid.New()
	ctx := reqctx.WithPrincipal(context.Background(), reqctx.Principal{UserID: uid, Role: domain.RoleClient})
	_, err := svc.Ingest(ctx, []ClientEvent{
		ev("page_view", "anon-visitor-1", "/en/calculators/astrology", nil),
		ev("page_view", "anon-visitor-1", "/en/psychics", nil),
		ev("page_view", "", "/en/psychics", nil), // no anonymous id: nothing to link
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sink.events) != 2 {
		t.Fatalf("events: %+v", sink.events)
	}
	if e := sink.events[0]; e.Type != domain.VisitView || !e.SignedIn || e.UserID == nil || *e.UserID != uid {
		t.Fatalf("signed-in view: %+v", e)
	}
	if e := sink.events[1]; e.Type != domain.VisitLink || e.UserID == nil || *e.UserID != uid {
		t.Fatalf("link: %+v", e)
	}
}

func TestIngestClientEventsAllowListIncludesCampaignLanding(t *testing.T) {
	if !ClientEvents["campaign_landing"] {
		t.Fatal("campaign_landing must be accepted from browsers")
	}
}

// --- reports ---------------------------------------------------------------

type fakeCalcRepo struct {
	calls            atomic.Int64
	totals           repository.CalcTotalsRow
	kinds, locales   []repository.CalcTotalsRow
	sources          []repository.CalcSourceRow
	links            []repository.CalcLinkRow
	series, signups  []repository.CalcBucketRow
	conv             []repository.CalcConversionRow
	lastFilter       repository.CalculatorFilter
	lastCursor       *repository.Cursor
	lastLimit        int
	linked           []string
	purged, retained int
}

func (f *fakeCalcRepo) ApplyVisitorBatch(context.Context, *domain.VisitorBatch) error { return nil }
func (f *fakeCalcRepo) LinkSignup(_ context.Context, _ uuid.UUID, anon, _ string, _ *domain.VisitorLanding, _ string, _ time.Time) error {
	f.linked = append(f.linked, anon)
	return nil
}
func (f *fakeCalcRepo) Totals(_ context.Context, fl repository.CalculatorFilter) (repository.CalcTotalsRow, error) {
	f.calls.Add(1)
	f.lastFilter = fl
	return f.totals, nil
}
func (f *fakeCalcRepo) ByDimension(_ context.Context, _ repository.CalculatorFilter, dim string) ([]repository.CalcTotalsRow, error) {
	if dim == "kind" {
		return f.kinds, nil
	}
	return f.locales, nil
}
func (f *fakeCalcRepo) BySource(context.Context, repository.CalculatorFilter, int) ([]repository.CalcSourceRow, error) {
	return f.sources, nil
}
func (f *fakeCalcRepo) ByLink(context.Context, repository.CalculatorFilter, int) ([]repository.CalcLinkRow, error) {
	return f.links, nil
}
func (f *fakeCalcRepo) Series(context.Context, repository.CalculatorFilter, string) ([]repository.CalcBucketRow, error) {
	return f.series, nil
}
func (f *fakeCalcRepo) SignupSeries(context.Context, repository.CalculatorFilter, string) ([]repository.CalcBucketRow, error) {
	return f.signups, nil
}
func (f *fakeCalcRepo) Conversions(_ context.Context, fl repository.CalculatorFilter, c *repository.Cursor, limit int) ([]repository.CalcConversionRow, error) {
	f.lastFilter, f.lastCursor, f.lastLimit = fl, c, limit
	if limit > len(f.conv) {
		limit = len(f.conv)
	}
	return f.conv[:limit], nil
}
func (f *fakeCalcRepo) PurgeForUser(context.Context, uuid.UUID) (int64, error) {
	f.purged++
	return 1, nil
}
func (f *fakeCalcRepo) DeleteOlderThan(_ context.Context, cutoff time.Time) (int64, error) {
	f.retained++
	return 0, nil
}

func calcSvc(repo repository.CalculatorRepository) *CalculatorService {
	clk := clock.NewFake(calcNow)
	return NewCalculatorService(repo, cache.NewMemory(clk.Now), clk, true)
}

func validationField(t *testing.T, err error, field string) {
	t.Helper()
	e, ok := apperror.As(err)
	if !ok || e.Status != 422 && e.Status != 400 {
		t.Fatalf("want a validation error on %s, got %v", field, err)
	}
	if !strings.Contains(fmt.Sprint(e.Details["fields"]), field+":") {
		t.Fatalf("validation error without field %s: %+v", field, e.Details)
	}
}

func TestCalculatorRatesWithZeroDivisors(t *testing.T) {
	z := calcTotals(repository.CalcTotalsRow{})
	if z.TryRate != 0 || z.CompletionRate != 0 || z.SignupRate != 0 {
		t.Fatalf("zero divisors: %+v", z)
	}
	// Everyone existing: sign-up rate has a zero divisor.
	if r := calcTotals(repository.CalcTotalsRow{Visitors: 4, ExistingUsers: 4, Tried: 2, Signups: 0}); r.SignupRate != 0 || r.TryRate != 0.5 {
		t.Fatalf("existing only: %+v", r)
	}
	r := calcTotals(repository.CalcTotalsRow{Visitors: 100, ExistingUsers: 20, Tried: 40, Completed: 10, Signups: 8})
	if r.TryRate != 0.4 || r.CompletionRate != 0.25 || r.SignupRate != 0.1 {
		t.Fatalf("rates: %+v", r)
	}
}

func TestCalculatorQueryNormalization(t *testing.T) {
	s := calcSvc(&fakeCalcRepo{})
	norm := func(q CalculatorQuery) calculatorQuery {
		t.Helper()
		out, err := s.normalize(q)
		if err != nil {
			t.Fatalf("%+v: %v", q, err)
		}
		return out
	}
	// Defaults: the last 30 days up to the end of today, per day.
	d := norm(CalculatorQuery{})
	if !d.to.Equal(time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)) || !d.from.Equal(time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)) || d.interval != "day" {
		t.Fatalf("defaults: %v %v %s", d.from, d.to, d.interval)
	}
	// Equivalent spellings share a cache key.
	a := norm(CalculatorQuery{From: "2026-10-01", To: "2026-10-31", Interval: "hour", Locale: " FR "})
	b := norm(CalculatorQuery{From: "2026-10-01T13:00:00Z", To: "2026-10-31", Interval: "day", Locale: "fr"})
	if a.key() != b.key() || a.cacheKey() != b.cacheKey() {
		t.Fatalf("keys differ:\n%s\n%s", a.key(), b.key())
	}
	if !a.to.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("inclusive end date: %v", a.to)
	}
	// Different filters never collide.
	camp := uuid.New()
	keys := map[string]bool{}
	for _, q := range []CalculatorQuery{
		{From: "2026-10-01", To: "2026-10-31"}, {From: "2026-10-01", To: "2026-10-31", Kind: "astrology"},
		{From: "2026-10-01", To: "2026-10-31", Locale: "es"}, {From: "2026-10-01", To: "2026-10-31", Source: "(direct)", SourceSet: true},
		{From: "2026-10-01", To: "2026-10-31", Source: "instagram"}, {From: "2026-10-01", To: "2026-10-31", CampaignID: &camp},
		{From: "2026-10-01", To: "2026-10-31", LinkID: &camp}, {From: "2026-10-01", To: "2026-10-31", Interval: "week"},
	} {
		k := norm(q).cacheKey()
		if keys[k] {
			t.Fatalf("cache key collision for %+v", q)
		}
		keys[k] = true
	}
	// (direct) is the empty source filter, a source is exact.
	if f := norm(CalculatorQuery{Source: "(direct)", SourceSet: true}).filter; f.Source == nil || *f.Source != "" {
		t.Fatalf("direct: %+v", f.Source)
	}
	if f := norm(CalculatorQuery{Source: "instagram"}).filter; f.Source == nil || *f.Source != "instagram" {
		t.Fatalf("source: %+v", f.Source)
	}
}

func TestCalculatorQueryValidation(t *testing.T) {
	s := calcSvc(&fakeCalcRepo{})
	cases := map[string]struct {
		q     CalculatorQuery
		field string
	}{
		"bad from":      {CalculatorQuery{From: "yesterday"}, "from"},
		"bad to":        {CalculatorQuery{To: "nope"}, "to"},
		"inverted":      {CalculatorQuery{From: "2026-10-31", To: "2026-10-01"}, "from"},
		"367 days":      {CalculatorQuery{From: "2025-10-01", To: "2026-10-03"}, "from"},
		"interval":      {CalculatorQuery{Interval: "year"}, "interval"},
		"kind":          {CalculatorQuery{Kind: "tarot"}, "kind"},
		"locale":        {CalculatorQuery{Locale: "de"}, "locale"},
		"source chars":  {CalculatorQuery{Source: "a<b", SourceSet: true}, "source"},
		"source empty":  {CalculatorQuery{SourceSet: true}, "source"},
		"source length": {CalculatorQuery{Source: strings.Repeat("a", 101)}, "source"},
	}
	for name, c := range cases {
		_, err := s.normalize(c.q)
		if err == nil {
			t.Errorf("%s accepted", name)
			continue
		}
		validationField(t, err, c.field)
	}
	// 366 days exactly is fine.
	if _, err := s.normalize(CalculatorQuery{From: "2025-11-07T00:00:00Z", To: "2026-11-08T00:00:00Z"}); err != nil {
		t.Fatalf("366 days: %v", err)
	}
}

func TestFillSeriesZeroFillsAllIntervals(t *testing.T) {
	from := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC) // a Wednesday
	to := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
	rows := []repository.CalcBucketRow{{Bucket: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), Visitors: 5, Tried: 2, Completed: 1, Signups: 1}}
	day := fillSeries(from, to, "day", rows)
	if len(day) != 11 || day[0].Visitors != 0 || day[5].Visitors != 5 || !day[5].Bucket.Equal(rows[0].Bucket) {
		t.Fatalf("day series: %d points, %+v", len(day), day[5])
	}
	week := fillSeries(from, to, "week", rows) // Monday buckets: Oct 26, Nov 2
	if len(week) != 2 || !week[0].Bucket.Equal(time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)) || week[1].Visitors != 5 {
		t.Fatalf("week series: %+v", week)
	}
	month := fillSeries(from, to, "month", []repository.CalcBucketRow{{Bucket: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Visitors: 3}})
	if len(month) != 2 || month[0].Visitors != 0 || month[1].Visitors != 3 || !month[1].Bucket.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("month series: %+v", month)
	}
	if got := fillSeries(from, from, "day", nil); got == nil || len(got) != 0 {
		t.Fatal("empty window must give an empty array")
	}
}

func TestMergeSignupsIntoUsageBuckets(t *testing.T) {
	d := func(day int) time.Time { return time.Date(2026, 11, day, 0, 0, 0, 0, time.UTC) }
	out := mergeSignups(
		[]repository.CalcBucketRow{{Bucket: d(1), Visitors: 4}, {Bucket: d(3), Visitors: 2}},
		[]repository.CalcBucketRow{{Bucket: d(3), Signups: 1}, {Bucket: d(2), Signups: 2}})
	if len(out) != 3 {
		t.Fatalf("merged: %+v", out)
	}
	pts := fillSeries(d(1), d(5), "day", out)
	if pts[0].Visitors != 4 || pts[1].Signups != 2 || pts[1].Visitors != 0 || pts[2].Visitors != 2 || pts[2].Signups != 1 || pts[3].Visitors != 0 {
		t.Fatalf("points: %+v", pts)
	}
}

func TestCalculatorReportShapeAndCache(t *testing.T) {
	repo := &fakeCalcRepo{
		totals:  repository.CalcTotalsRow{Visitors: 10, Tried: 5, Completed: 2, Signups: 1, ExistingUsers: 2},
		kinds:   []repository.CalcTotalsRow{{Key: "astrology", Visitors: 7}},
		sources: []repository.CalcSourceRow{{Source: "", Visitors: 6}, {Source: "instagram", Medium: "social", Campaign: "spring", Visitors: 4, Signups: 1}},
	}
	s := calcSvc(repo)
	q := CalculatorQuery{From: "2026-11-01", To: "2026-11-07"}
	r, err := s.Report(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.ByKind) != 3 || r.ByKind[0].Kind != "natal-chart" || r.ByKind[2].Kind != "astrology" || r.ByKind[2].Visitors != 7 || r.ByKind[0].Visitors != 0 {
		t.Fatalf("by_kind: %+v", r.ByKind)
	}
	if len(r.ByLocale) != 3 || r.ByLocale[0].Locale != "en" || r.ByLocale[2].Locale != "es" {
		t.Fatalf("by_locale: %+v", r.ByLocale)
	}
	if r.BySource[0].Source != DirectSource || r.BySource[1].Source != "instagram" || r.ByLink == nil || len(r.Series) != 7 {
		t.Fatalf("report: %+v", r)
	}
	if r.Totals.SignupRate != 1.0/8 {
		t.Fatalf("signup rate: %v", r.Totals.SignupRate)
	}
	// Second identical request is served from the cache; a changed query is not.
	if _, err := s.Report(context.Background(), CalculatorQuery{From: "2026-11-01T09:00:00Z", To: "2026-11-07"}); err != nil {
		t.Fatal(err)
	}
	if repo.calls.Load() != 1 {
		t.Fatalf("repository reads: %d (want 1, cached)", repo.calls.Load())
	}
	if _, err := s.Report(context.Background(), CalculatorQuery{From: "2026-11-01", To: "2026-11-07", Kind: "astrology"}); err != nil {
		t.Fatal(err)
	}
	if repo.calls.Load() != 2 {
		t.Fatalf("repository reads: %d (want 2)", repo.calls.Load())
	}
}

func TestCalculatorConversionsPaging(t *testing.T) {
	repo := &fakeCalcRepo{}
	for i := 0; i < 30; i++ {
		repo.conv = append(repo.conv, repository.CalcConversionRow{UserID: uuid.New(), Email: "u@example.com",
			SignedUpAt: calcNow.Add(-time.Duration(i) * time.Hour), Kinds: []string{"astrology", "natal-chart"}, Locales: []string{"fr", "en"}})
	}
	repo.conv[0].UTMSource, repo.conv[0].LinkCode = "instagram", "abc1234"
	s := calcSvc(repo)
	page, err := s.Conversions(context.Background(), CalculatorQuery{Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 25 || page.NextCursor == "" || repo.lastLimit != 26 {
		t.Fatalf("page: %d items, cursor %q, limit %d", len(page.Items), page.NextCursor, repo.lastLimit)
	}
	if k := page.Items[0].Kinds; len(k) != 2 || k[0] != "natal-chart" || k[1] != "astrology" {
		t.Fatalf("kinds not in contract order: %v", k)
	}
	if l := page.Items[0].Locales; l[0] != "en" || l[1] != "fr" {
		t.Fatalf("locales: %v", l)
	}
	if page.Items[0].Attribution == nil || page.Items[0].Attribution.LinkCode != "abc1234" || page.Items[1].Attribution != nil {
		t.Fatalf("attribution: %+v / %+v", page.Items[0].Attribution, page.Items[1].Attribution)
	}
	// The cursor decodes to the last returned (signed_up_at, user).
	last := page.Items[24]
	cur, err := repository.DecodeCursor(page.NextCursor)
	if err != nil || cur.ID != last.User.ID.String() || !cur.At.Equal(last.SignedUpAt) {
		t.Fatalf("cursor: %+v %v", cur, err)
	}
	if _, err := s.Conversions(context.Background(), CalculatorQuery{Cursor: page.NextCursor, Limit: 25}); err != nil || repo.lastCursor == nil || repo.lastCursor.ID != cur.ID {
		t.Fatalf("cursor not forwarded: %v %+v", err, repo.lastCursor)
	}
	// Limit is clamped to 100; a short page has no cursor.
	if p, _ := s.Conversions(context.Background(), CalculatorQuery{Limit: 1000}); repo.lastLimit != 101 || p.NextCursor != "" || len(p.Items) != 30 {
		t.Fatalf("clamp: limit %d, %d items", repo.lastLimit, len(p.Items))
	}
	for _, bad := range []string{"%%%", "e30", strings.Repeat("a", 300)} {
		_, err := s.Conversions(context.Background(), CalculatorQuery{Cursor: bad})
		validationField(t, err, "cursor")
	}
	empty, _ := calcSvc(&fakeCalcRepo{}).Conversions(context.Background(), CalculatorQuery{})
	if empty.Items == nil || len(empty.Items) != 0 {
		t.Fatal("empty page must have an empty items array")
	}
}

func TestLinkSignupValidatesAnonymousID(t *testing.T) {
	repo := &fakeCalcRepo{}
	s := calcSvc(repo)
	uid := uuid.New()
	for _, bad := range []string{"", "short", "has space 1234", strings.Repeat("a", 65), "bad/chars/12"} {
		s.LinkSignup(context.Background(), uid, "en", &AttributionInput{AnonymousID: bad})
	}
	s.LinkSignup(context.Background(), uid, "en", nil)
	if len(repo.linked) != 0 {
		t.Fatalf("invalid ids linked: %v", repo.linked)
	}
	s.LinkSignup(context.Background(), uid, "fr", &AttributionInput{AnonymousID: "valid-anon_id-1", UTMSource: "x"})
	if len(repo.linked) != 1 || repo.linked[0] != "valid-anon_id-1" {
		t.Fatalf("linked: %v", repo.linked)
	}
	off := NewCalculatorService(repo, nil, clock.NewFake(calcNow), false)
	off.LinkSignup(context.Background(), uid, "en", &AttributionInput{AnonymousID: "valid-anon_id-2"})
	if len(repo.linked) != 1 {
		t.Fatal("linked while analytics is disabled")
	}
}

func TestCalculatorRetentionAndErasureDelegate(t *testing.T) {
	repo := &fakeCalcRepo{}
	s := calcSvc(repo)
	if _, err := s.RunRetention(context.Background(), calcNow); err != nil || repo.retained != 1 {
		t.Fatal("retention")
	}
	if _, err := s.PurgeForUser(context.Background(), uuid.New()); err != nil || repo.purged != 1 {
		t.Fatal("purge")
	}
}
