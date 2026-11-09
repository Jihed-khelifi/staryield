package repository

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// allowedRawSQL lists every function that builds SQL by hand (.Raw / .Exec)
// and how many such calls it makes. Queries go through the GORM builder; the
// exceptions below are reporting aggregates (CTEs, FILTER, date_trunc
// buckets, window functions) the builder cannot express. All of them bind
// values as parameters, and identifiers they splice in come from allow-lists
// (validIntervals, groupColumns, bucketExpr) — never from user input.
//
// Adding a raw query means adding it here, in review, with that guarantee.
var allowedRawSQL = map[string]int{
	// revisions 3.0: calculator analytics. Multi-row INSERT … ON CONFLICT
	// upserts of the visitor roll-up, UPDATE … FROM (VALUES …) user links and
	// report aggregates (CTEs, FILTER, LATERAL, DISTINCT ON, date_trunc with a
	// unit from the calUnits allow-list). Every value is bound as a parameter;
	// the only spliced fragments are constants and the dim/column allow-lists.
	"calculators.go:GormCalculatorRepository.ByLink":          1,
	"calculators.go:GormCalculatorRepository.BySource":        1,
	"calculators.go:GormCalculatorRepository.Conversions":     1,
	"calculators.go:GormCalculatorRepository.DeleteOlderThan": 1,
	"calculators.go:GormCalculatorRepository.LinkSignup":      1,
	"calculators.go:GormCalculatorRepository.PurgeForUser":    2,
	"calculators.go:GormCalculatorRepository.Series":          1,
	"calculators.go:GormCalculatorRepository.SignupSeries":    1,
	"calculators.go:GormCalculatorRepository.dimRows":         1,
	"calculators.go:GormCalculatorRepository.linkVisitors":    1,
	"calculators.go:GormCalculatorRepository.resolveLandings": 2,
	"calculators.go:GormCalculatorRepository.upsertVisitors":  1,
	"calculators.go:GormCalculatorRepository.upsertVisits":    1,
	// Product analytics and admin reports (time buckets from validIntervals).
	"analytics.go:GormAnalyticsRepository.Funnel":     1,
	"analytics.go:GormAnalyticsRepository.Timeseries": 1,
	"analytics.go:GormAnalyticsRepository.TopEvents":  1,
	"analytics.go:GormReportRepository.Daily":         1,
	"analytics.go:GormReportRepository.Overview":      6,
	"analytics.go:GormReportRepository.keyed":         1,
	// Advisor overview KPIs: DISTINCT ON + EXISTS cohorts and a lag()/
	// percentile_cont median; values are bound with sql.Named, no identifiers.
	"stats.go:GormStatsRepository.FreeToPaid":     1,
	"stats.go:GormStatsRepository.PaidToRetained": 1,
	"stats.go:GormStatsRepository.RepeatGap":      1,
	// Growth reports (bucketExpr allow-list).
	"growth.go:GormGrowthRepository.ByCampaign":     1,
	"growth.go:GormGrowthRepository.ByLink":         1,
	"growth.go:GormGrowthRepository.BySource":       3,
	"growth.go:GormGrowthRepository.CampaignInUse":  1,
	"growth.go:GormGrowthRepository.Devices":        1,
	"growth.go:GormGrowthRepository.OrganicSignups": 1,
	"growth.go:GormGrowthRepository.Referrers":      1,
	"growth.go:GormGrowthRepository.Series":         3,
	"growth.go:GormGrowthRepository.Totals":         1,
	// Audience insights and advisor analytics (groupColumns, bucketExpr).
	"insights.go:GormInsightsRepository.ActiveUserSeries":      1,
	"insights.go:GormInsightsRepository.Activity":              1,
	"insights.go:GormInsightsRepository.AgeBuckets":            1,
	"insights.go:GormInsightsRepository.BirthDays":             1,
	"insights.go:GormInsightsRepository.ClientHistoryWith":     1,
	"insights.go:GormInsightsRepository.Devices":               1,
	"insights.go:GormInsightsRepository.Engagement":            1,
	"insights.go:GormInsightsRepository.GroupUsers":            1,
	"insights.go:GormInsightsRepository.PsychicClients":        1,
	"insights.go:GormInsightsRepository.PsychicEarningsSeries": 1,
	"insights.go:GormInsightsRepository.PsychicHours":          1,
	"insights.go:GormInsightsRepository.PsychicSessions":       1,
	"insights.go:GormInsightsRepository.PsychicTopClients":     1,
	"insights.go:GormInsightsRepository.PsychicsUsedBy":        1,
	"insights.go:GormInsightsRepository.Retention":             2,
	"insights.go:GormInsightsRepository.SignupSeries":          1,
	"insights.go:GormInsightsRepository.Sources":               1,
	"insights.go:GormInsightsRepository.TopUsers":              1,
	"insights.go:GormInsightsRepository.UserDevices":           1,
	"insights.go:GormInsightsRepository.UserStats":             1,
	// revisions 3.0: site settings. Sponsored (free mode) chat totals; every
	// value is a bound parameter.
	"analytics.go:GormReportRepository.SponsoredUsage": 1,
	"site.go:GormSiteRepository.SponsoredEarnings":     1,
	"site.go:GormSiteRepository.SponsoredTotals":       1,
	// revisions 3.3: messages outside a live chat. Index-backed counters and an
	// EXISTS probe (chat_messages_off_session_*_idx, chat_sessions_conversation_idx);
	// every value is a bound parameter, no identifiers.
	"chat.go:GormChatRepository.CountOffSession":     1,
	"chat.go:GormChatRepository.HasStartedSession":   1,
	"site.go:GormSiteRepository.OffSessionLimitHits": 1,
	"site.go:GormSiteRepository.OffSessionTotals":    1,
	// revisions 3.0: support desk. Index-only counters in one statement and a
	// batched DELETE ... FOR UPDATE SKIP LOCKED; every value is bound, no identifiers.
	"support.go:GormSupportRepository.DeleteClosed": 1,
	"support.go:GormSupportRepository.Summary":      1,
}

// rawSQLInventory returns "file:Receiver.Func" → number of .Raw/.Exec calls
// in the non-test files of this package.
func rawSQLInventory(t *testing.T) map[string]int {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	out := map[string]int{}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := fn.Name.Name
			if fn.Recv != nil && len(fn.Recv.List) == 1 {
				typ := fn.Recv.List[0].Type
				if star, ok := typ.(*ast.StarExpr); ok {
					typ = star.X
				}
				if id, ok := typ.(*ast.Ident); ok {
					name = id.Name + "." + name
				}
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && (sel.Sel.Name == "Raw" || sel.Sel.Name == "Exec") {
					out[path+":"+name]++
				}
				return true
			})
		}
	}
	return out
}

// TestRawSQLIsAllowListed fails when a repository builds SQL by hand outside
// the reviewed allow-list (or an allow-listed query disappeared, so the list
// stays exact).
func TestRawSQLIsAllowListed(t *testing.T) {
	got := rawSQLInventory(t)
	if os.Getenv("RAWSQL_DUMP") != "" {
		keys := make([]string, 0, len(got))
		for k := range got {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("\t%q: %d,\n", k, got[k])
		}
	}
	var problems []string
	for k, n := range got {
		allowed, ok := allowedRawSQL[k]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s builds %d raw SQL statement(s); use the GORM builder or allow-list it with a justification", k, n))
		case n > allowed:
			problems = append(problems, fmt.Sprintf("%s builds %d raw SQL statements (allowed %d)", k, n, allowed))
		}
	}
	for k, allowed := range allowedRawSQL {
		if got[k] < allowed {
			problems = append(problems, fmt.Sprintf("%s is allow-listed for %d raw SQL statement(s) but has %d; tighten the allow-list", k, allowed, got[k]))
		}
	}
	sort.Strings(problems)
	if len(problems) > 0 {
		t.Fatalf("raw SQL guard:\n%s", strings.Join(problems, "\n"))
	}
}
