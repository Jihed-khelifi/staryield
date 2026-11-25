package e2e

// Parallel shards of the suite.
//
// The suite runs one test at a time and spends most of its wall time waiting
// on PostgreSQL: the test process itself uses about a seventh of one core. So
// by default the test binary acts as a coordinator. It migrates the test
// database once, clones it for every shard (CREATE DATABASE ... TEMPLATE, a
// file copy), splits the selected tests into groups of similar cost, and runs
// each group in a child process of the same binary against its own clone.
// Tests within a shard still run one at a time, in source order, exactly as
// before. The large-dataset tests (largeTests) then run one at a time and
// alone, after the shards: they are I/O bound and measure latencies.
//
//	STARYIELD_E2E_SHARDS=n   number of shards (default: half the CPUs, at most 4)
//	STARYIELD_E2E_SHARDS=1   everything in this process, the way it used to run
//
// A run that selects a single test, lists tests, writes a profile or coverage,
// or is driven by `go test -json` stays in-process. Each shard's output is
// printed as one block when the shard ends, so lines of concurrent tests never
// interleave. The clones are dropped after a passing run and kept (their names
// are printed) after a failing one.

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const (
	shardsEnv = "STARYIELD_E2E_SHARDS"
	// shardEnv is set ("k/n") in the child processes: their database is a
	// migrated clone, so they skip the schema reset.
	shardEnv = "STARYIELD_E2E_SHARD"
	// maxDefaultShards bounds the default: beyond four, PostgreSQL rather
	// than the suite becomes the bottleneck on a laptop.
	maxDefaultShards = 4
)

// testCost is the measured duration in seconds of the tests that take more
// than a second (full serial run, 2026-11-25); every other test counts
// defaultTestCost. It only balances the shards, so it need not be exact:
// refresh it when a test becomes much slower or faster
// (`go test ./tests/e2e -count=1 -v | grep -- '--- PASS'`).
var testCost = map[string]float64{
	"TestSupportDeskLargeDatasetFiltersSortsAndKeysetPagination": 104,
	"TestCalculatorLargeDataset":                                 75,
	"TestChatUnreadLargeDataset":                                 36,
	"TestBroadcastToThousandsIsExactIdempotentAndResumable":      14.5,
	"TestSiteStatsAndOverviewOnALargeDataset":                    11.4,
	"TestAdvisorInboxWithThousandsOfAliasesIsOneQuery":           6.5,
	"TestRepliesAndLikesAtSize":                                  6,
	"TestTopUpRacingExpiryHasExactlyOneOutcome":                  5.8,
	"TestLargeSeedSmallScale":                                    3.9,
	"TestLargeSeedNotificationsGenerator":                        3.4,
	"TestEndingAHeldReadingExpiresTheHold":                       3.4,
	"TestReferralLoopEndToEnd":                                   3.1,
	"TestChatRedisOutageResync":                                  2.6,
}

const defaultTestCost = 0.45

// largeTests build datasets of 100,000 rows and more and measure latencies
// (they skip under -short). They run after the shards, one at a time and
// alone: four of them at once (with the shards) made PostgreSQL thrash, every
// shard about seven times slower than its share and their latency budgets
// fail. Add a test here when it seeds data at that scale.
var largeTests = map[string]bool{
	"TestSupportDeskLargeDatasetFiltersSortsAndKeysetPagination": true,
	"TestCalculatorLargeDataset":                                 true,
	"TestChatUnreadLargeDataset":                                 true,
	"TestBroadcastToThousandsIsExactIdempotentAndResumable":      true,
	"TestSiteStatsAndOverviewOnALargeDataset":                    true,
	"TestAdvisorInboxWithThousandsOfAliasesIsOneQuery":           true,
	"TestRepliesAndLikesAtSize":                                  true,
}

// shardCount reads STARYIELD_E2E_SHARDS (default: half the CPUs, at most 4).
func shardCount() int {
	if v := strings.TrimSpace(os.Getenv(shardsEnv)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			return n
		}
		fmt.Fprintf(os.Stderr, "e2e: ignoring %s=%q (want a whole number >= 1)\n", shardsEnv, v)
	}
	return min(maxDefaultShards, max(1, runtime.NumCPU()/2))
}

// flagValue returns a testing flag's value ("" when it is not registered).
func flagValue(name string) string {
	if f := flag.Lookup(name); f != nil {
		return f.Value.String()
	}
	return ""
}

// shardable reports whether this invocation may be split: not a listing, not
// test2json (`go test -json`), no profile or coverage output, no subtest
// pattern, and a URL-style DSN the clones can be derived from.
func shardable() bool {
	if flagValue("test.list") != "" || flagValue("test.v") == "test2json" {
		return false
	}
	for _, f := range []string{"test.cpuprofile", "test.memprofile", "test.blockprofile", "test.mutexprofile",
		"test.trace", "test.coverprofile", "test.gocoverdir", "test.outputdir"} {
		if flagValue(f) != "" {
			return false
		}
	}
	if strings.Contains(flagValue("test.run"), "/") {
		return false
	}
	u, err := url.Parse(testDSN)
	return err == nil && u.Scheme != "" && strings.Trim(u.Path, "/") != ""
}

// listTests asks the binary itself which top-level tests -test.run selects.
func listTests() ([]string, error) {
	pattern := flagValue("test.run")
	if pattern == "" {
		pattern = "."
	}
	out, err := exec.Command(os.Args[0], "-test.list", pattern).Output()
	if err != nil {
		return nil, fmt.Errorf("list tests: %w", err)
	}
	var names []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); strings.HasPrefix(line, "Test") {
			names = append(names, line)
		}
	}
	return names, sc.Err()
}

// shardPlan splits tests into n groups of similar cost: the costliest test
// goes to the cheapest group first (longest-processing-time rule). Each group
// keeps the tests' original order.
func shardPlan(tests []string, n int) [][]string {
	order := make(map[string]int, len(tests))
	for i, t := range tests {
		order[t] = i
	}
	byCost := append([]string(nil), tests...)
	cost := func(t string) float64 {
		if c, ok := testCost[t]; ok {
			return c
		}
		return defaultTestCost
	}
	sort.SliceStable(byCost, func(i, j int) bool { return cost(byCost[i]) > cost(byCost[j]) })
	groups := make([][]string, n)
	load := make([]float64, n)
	for _, t := range byCost {
		k := 0
		for i := 1; i < n; i++ {
			if load[i] < load[k] {
				k = i
			}
		}
		groups[k] = append(groups[k], t)
		load[k] += cost(t)
	}
	for _, g := range groups {
		sort.Slice(g, func(i, j int) bool { return order[g[i]] < order[g[j]] })
	}
	return groups
}

// dsnFor returns the DSN of database name on testDSN's server.
func dsnFor(name string) string {
	u, _ := url.Parse(testDSN)
	u.Path = "/" + name
	return u.String()
}

func baseDatabase() string {
	u, _ := url.Parse(testDSN)
	return strings.Trim(u.Path, "/")
}

// cloneName is the database of shard k: <test database>_shard<k>.
func cloneName(k int) string { return fmt.Sprintf("%s_shard%d", baseDatabase(), k) }

// openMaintenance connects to the server's postgres database: a template
// cannot be cloned while a session (ours included) is connected to it.
func openMaintenance() (*gorm.DB, func(), error) {
	db, err := gorm.Open(postgres.Open(dsnFor("postgres")), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		return nil, nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	return db, func() { _ = sqlDB.Close() }, nil
}

// cloneDatabases creates the n shard databases from the migrated test
// database, replacing any left over from an earlier run.
func cloneDatabases(n int) error {
	db, done, err := openMaintenance()
	if err != nil {
		return err
	}
	defer done()
	quote := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	for k := 1; k <= n; k++ {
		name := cloneName(k)
		for _, stmt := range []string{
			"DROP DATABASE IF EXISTS " + quote(name) + " WITH (FORCE)",
			"CREATE DATABASE " + quote(name) + " TEMPLATE " + quote(baseDatabase()),
			// Per-database settings are not copied from the template.
			"ALTER DATABASE " + quote(name) + " SET synchronous_commit = off",
		} {
			if err := db.Exec(stmt).Error; err != nil {
				return fmt.Errorf("%s: %w", stmt, err)
			}
		}
	}
	return nil
}

func dropClones(n int) {
	db, done, err := openMaintenance()
	if err != nil {
		return
	}
	defer done()
	for k := 1; k <= n; k++ {
		db.Exec(`DROP DATABASE IF EXISTS "` + cloneName(k) + `" WITH (FORCE)`)
	}
}

// childArgs repeats this process's arguments with -test.run replaced.
func childArgs(run string) []string {
	var args []string
	in := os.Args[1:]
	for i := 0; i < len(in); i++ {
		a := strings.TrimPrefix(in[i], "-")
		switch {
		case a == "-test.run" || a == "test.run":
			i++ // the value follows
		case strings.HasPrefix(a, "-test.run=") || strings.HasPrefix(a, "test.run="):
		default:
			args = append(args, in[i])
		}
	}
	return append(args, "-test.run="+run)
}

// shardResult is one child's outcome.
type shardResult struct {
	label  string
	tests  int
	took   time.Duration
	err    error
	db     string
	output []byte
}

// runChild runs tests in a child process of this binary against database db.
func runChild(label, shard string, tests []string, db string) shardResult {
	quoted := make([]string, len(tests))
	for j, t := range tests {
		quoted[j] = regexp.QuoteMeta(t)
	}
	cmd := exec.Command(os.Args[0], childArgs("^("+strings.Join(quoted, "|")+")$")...)
	cmd.Env = append(os.Environ(), shardEnv+"="+shard, "STARYIELD_TEST_DATABASE_DSN="+dsnFor(db))
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	t0 := time.Now()
	err := cmd.Run()
	return shardResult{label: label, tests: len(tests), took: time.Since(t0), err: err, db: db, output: out.Bytes()}
}

// print writes a child's output (without its own PASS or FAIL verdict: one is
// printed for the whole run) and returns its summary line.
func (r shardResult) print() string {
	for _, line := range strings.SplitAfter(string(r.output), "\n") {
		if t := strings.TrimSpace(line); t != "PASS" && t != "FAIL" && line != "" {
			os.Stdout.WriteString(line)
		}
	}
	verdict := "ok"
	if r.err != nil {
		verdict = "FAIL (" + r.err.Error() + ", database " + r.db + " kept)"
	}
	return fmt.Sprintf("%s: %d tests in %s %s", r.label, r.tests, r.took.Round(time.Second), verdict)
}

// runShards runs the selected tests in child processes: first every test but
// the large-dataset ones in n shards at once, then the large-dataset ones one
// at a time and alone (see largeTests). ok is false when the run should stay
// in-process instead (fewer than two tests selected, or the clones could not
// be created); the database is then already reset.
func runShards(n int) (code int, ok bool) {
	tests, err := listTests()
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e: running in-process:", err)
		return 0, false
	}
	if len(tests) < 2 {
		return 0, false
	}
	var light, large []string
	for _, t := range tests {
		if largeTests[t] {
			large = append(large, t)
		} else {
			light = append(light, t)
		}
	}
	n = max(1, min(n, len(light)))
	if err := cloneDatabases(n); err != nil {
		fmt.Fprintln(os.Stderr, "e2e: running in-process, the shard databases could not be created:", err)
		return 0, false
	}
	start := time.Now()
	var summary []string
	failed := false
	report := func(r shardResult) {
		summary = append(summary, r.print())
		failed = failed || r.err != nil
	}

	if len(light) > 0 {
		groups := shardPlan(light, n)
		results := make(chan shardResult, n)
		var wg sync.WaitGroup
		for i, group := range groups {
			k := i + 1
			wg.Add(1)
			go func() {
				defer wg.Done()
				results <- runChild(fmt.Sprintf("shard %d/%d", k, n), fmt.Sprintf("%d/%d", k, n), group, cloneName(k))
			}()
		}
		go func() { wg.Wait(); close(results) }()
		var phase []string
		for r := range results {
			report(r)
			phase = append(phase, summary[len(summary)-1])
		}
		sort.Strings(phase)
		copy(summary[len(summary)-len(phase):], phase)
	}
	if len(large) > 0 {
		report(runChild("large-dataset tests, one at a time", "large", large, cloneName(1)))
	}

	fmt.Printf("e2e: %d tests (%d in %d shards, then %d large-dataset tests alone), %s\n  %s\n",
		len(tests), len(light), n, len(large), time.Since(start).Round(time.Second), strings.Join(summary, "\n  "))
	if failed {
		fmt.Println("FAIL")
		return 1, true
	}
	dropClones(n)
	fmt.Println("PASS")
	return 0, true
}

// pingDatabase checks a shard's database before its tests start.
func pingDatabase() error {
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return sqlDB.PingContext(ctx)
}
