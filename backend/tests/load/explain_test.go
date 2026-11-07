//go:build load

package load

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// seqScanRowLimit: a sequential scan on a table larger than this fails the
// check (small tables are legitimately scanned).
const seqScanRowLimit = 10_000

// explainRuns per query; the p95 of their execution times must stay under
// the query's budget.
const explainRuns = 20

// sqlRecorder captures the SQL a repository method builds without running it.
type sqlRecorder struct {
	mu   sync.Mutex
	last string
}

func newDryDB(t *testing.T, dsn string) (*gorm.DB, *sqlRecorder) {
	t.Helper()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{DryRun: true, Logger: logger.Discard})
	if err != nil {
		t.Fatalf("dry-run db: %v", err)
	}
	rec := &sqlRecorder{}
	capture := func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.SQL.Len() == 0 {
			return
		}
		rec.mu.Lock()
		rec.last = tx.Dialector.Explain(tx.Statement.SQL.String(), tx.Statement.Vars...)
		rec.mu.Unlock()
	}
	_ = db.Callback().Query().After("gorm:query").Register("load:capture_query", capture)
	_ = db.Callback().Row().After("gorm:row").Register("load:capture_row", capture)
	return db, rec
}

func (r *sqlRecorder) take() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.last
	r.last = ""
	return s
}

type planNode struct {
	NodeType string     `json:"Node Type"`
	Relation string     `json:"Relation Name"`
	Index    string     `json:"Index Name"`
	Plans    []planNode `json:"Plans"`
}

type explainOut struct {
	Plan          planNode `json:"Plan"`
	ExecutionTime float64  `json:"Execution Time"`
}

func walk(n planNode, visit func(planNode)) {
	visit(n)
	for _, c := range n.Plans {
		walk(c, visit)
	}
}

type queryCheck struct {
	name   string
	budget time.Duration                           // p95 execution time
	build  func(ctx context.Context, dry *gorm.DB) // calls one repository method
}

// TestExplainHotQueries runs EXPLAIN (ANALYZE) on the SQL that repository
// methods actually build, with live IDs from the (large) dataset.
func TestExplainHotQueries(t *testing.T) {
	cfg := LoadConfig()
	live, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("database %s: %v", cfg.DSN, err)
	}
	dry, rec := newDryDB(t, cfg.DSN)
	ctx := context.Background()

	ids := pickIDs(t, live)
	calc := pickCalculatorSample(live) // revisions 3.0: calculator analytics
	sup := pickSupportIDs(live)        // revisions 3.0: support desk
	checks := []queryCheck{
		{"notes: own list", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewNoteRepository(d).ListByAuthor(ctx, ids.noteAuthor, ids.noteClient, nil, 6)
		}},
		{"notes: shared list", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewNoteRepository(d).ListShared(ctx, ids.noteClient, uuid.New(), nil, 6)
		}},
		{"notes: met check", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewNoteRepository(d).HasStartedSession(ctx, ids.sessionPsychic, ids.sessionClient)
		}},
		{"wallet: ledger page", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewWalletRepository(d).ListLedger(ctx, ids.ledgerUser, nil, 20)
		}},
		{"notifications: page", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewNotificationRepository(d).List(ctx, ids.notifiedUser, nil, 20)
		}},
		{"reviews: advisor page", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewReviewRepository(d).List(ctx, repository.ReviewFilter{PsychicID: &ids.reviewedPsychic, Status: domain.ReviewVisible, Limit: 10})
		}},
		{"stats: free→paid", 50 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewStatsRepository(d).FreeToPaid(ctx, ids.sessionPsychic, monthAgo(), time.Now())
		}},
		{"stats: paid→retained", 50 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewStatsRepository(d).PaidToRetained(ctx, ids.sessionPsychic, monthAgo(), time.Now())
		}},
		{"stats: repeat gap", 50 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewStatsRepository(d).RepeatGap(ctx, ids.sessionPsychic, monthAgo(), time.Now())
		}},
		{"withdrawals: advisor page", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewWithdrawalRepository(d).List(ctx, repository.WithdrawalFilter{PsychicID: &ids.sessionPsychic, Limit: 20})
		}},
		// Only reads below: EXPLAIN ANALYZE executes the statement.
		{"catalog: listed cards", 50 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewCatalogRepository(d).ListedCards(ctx)
		}},
		{"presence: open intervals", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewPresenceRepository(d).OpenIntervals(ctx, 5000)
		}},
		{"chat: inbox (advisor)", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).ListInbox(ctx, repository.InboxFilter{UserID: ids.busyPsychic, AsPsychic: true, Limit: 21})
		}},
		{"chat: inbox (client)", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).ListInbox(ctx, repository.InboxFilter{UserID: ids.sessionClient, Limit: 21})
		}},
		{"chat: history before seq", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).ListMessagesBefore(ctx, ids.busyConversation, 1<<40, 51)
		}},
		{"chat: catch-up after seq", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).ListMessagesAfterSeq(ctx, ids.busyConversation, 1, 101)
		}},
		{"chat: busy advisors", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).BusyPsychics(ctx)
		}},
		{"chat: billing claim", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).ClaimDueBilling(ctx, time.Now(), 100)
		}},
		{"waitlist: my lists", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewWaitlistRepository(d).ListOpen(ctx, ids.waitingUser, nil, 21)
		}},
		{"waitlist: due claim", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewWaitlistRepository(d).ClaimDue(ctx, 200)
		}},
		{"devices: active sessions", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewSessionRepository(d).ListActive(ctx, ids.ledgerUser, time.Now(), nil, 11)
		}},
		{"billing: transactions page", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewBillingRepository(d).ListTransactions(ctx, repository.TransactionFilter{UserID: &ids.ledgerUser, Limit: 21})
		}},
		{"chat: saved messages page", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).ListBookmarks(ctx, ids.sessionClient, nil, nil, 6)
		}},
		{"advisor photos: review queue", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewPhotoSubmissionRepository(d).AdminList(ctx, repository.PhotoAdminFilter{Status: domain.PhotoPending, Limit: 21})
		}},
		// revisions 3.0: chat unread
		{"chat: unread totals (advisor)", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _, _ = repository.NewChatRepository(d).UnreadTotals(ctx, ids.busyPsychic, true)
		}},
		{"chat: unread totals (client)", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _, _ = repository.NewChatRepository(d).UnreadTotals(ctx, ids.sessionClient, false)
		}},
		{"advisor photos: current submission", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewPhotoSubmissionRepository(d).Current(ctx, ids.sessionPsychic)
		}},
		// revisions 3.0: site settings. Free-mode statistics and the
		// overview's sponsored block read chat_sessions through the partial
		// index on sponsored chats; the banner list is a handful of rows.
		{"site: sponsored totals", 50 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewSiteRepository(d).SponsoredTotals(ctx, monthAgo())
		}},
		{"site: sponsored earnings", 50 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewSiteRepository(d).SponsoredEarnings(ctx, monthAgo())
		}},
		{"reports: sponsored usage", 50 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewReportRepository(d).SponsoredUsage(ctx, monthAgo(), time.Now())
		}},
		{"site: banner list", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewSiteRepository(d).Banners(ctx)
		}},
		{"site: settings", 25 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewSiteRepository(d).Settings(ctx)
		}},
	}
	checks = append(checks, calculatorChecks(calc)...)
	checks = append(checks, supportChecks(sup)...) // revisions 3.0: support desk

	var lines []string
	for _, qc := range checks {
		qc.build(ctx, dry)
		sql := rec.take()
		if sql == "" {
			t.Errorf("%s: no SQL captured", qc.name)
			continue
		}
		var times []float64
		var plan explainOut
		for i := 0; i < explainRuns; i++ {
			var raw string
			if err := live.Raw("EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) " + sql).Row().Scan(&raw); err != nil {
				t.Fatalf("%s: explain: %v\n%s", qc.name, err, sql)
			}
			var out []explainOut
			if err := json.Unmarshal([]byte(raw), &out); err != nil || len(out) == 0 {
				t.Fatalf("%s: parse plan: %v", qc.name, err)
			}
			plan = out[0]
			times = append(times, plan.ExecutionTime)
		}
		sort.Float64s(times)
		p95 := time.Duration(times[int(float64(len(times))*0.95+0.999999)-1] * float64(time.Millisecond))

		var nodes []string
		walk(plan.Plan, func(n planNode) {
			label := n.NodeType
			if n.Relation != "" {
				label += " on " + n.Relation
			}
			if n.Index != "" {
				label += " using " + n.Index
			}
			nodes = append(nodes, label)
			if n.NodeType == "Seq Scan" && n.Relation != "" && !seqScanAllowed[qc.name] {
				if rows := estimatedRows(live, n.Relation); rows > seqScanRowLimit {
					t.Errorf("%s: sequential scan on %s (~%d rows)\n%s", qc.name, n.Relation, rows, sql)
				}
			}
		})
		if p95 > qc.budget {
			t.Errorf("%s: p95 execution %s exceeds %s", qc.name, p95, qc.budget)
		}
		lines = append(lines, fmt.Sprintf("%-28s p95=%-10s %s", qc.name, p95.Round(10*time.Microsecond), strings.Join(nodes, " → ")))
	}
	t.Logf("\n%s", strings.Join(lines, "\n"))
}

func monthAgo() time.Time { return time.Now().AddDate(0, -1, 0) }

func estimatedRows(db *gorm.DB, relation string) int64 {
	var n float64
	_ = db.Table("pg_class").Select("reltuples").Where("relname = ? AND relkind = 'r'", relation).Scan(&n).Error
	return int64(n)
}

type sampleIDs struct {
	noteAuthor, noteClient        uuid.UUID
	sessionPsychic, sessionClient uuid.UUID
	ledgerUser, notifiedUser      uuid.UUID
	reviewedPsychic               uuid.UUID
	busyPsychic, busyConversation uuid.UUID
	waitingUser                   uuid.UUID
}

// pickIDs chooses the busiest rows (worst case for each query); random IDs
// are used when a table is empty so the check still runs.
func pickIDs(t *testing.T, db *gorm.DB) sampleIDs {
	t.Helper()
	ids := sampleIDs{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	var pair struct{ A, B uuid.UUID }
	if db.Table("client_notes").Select("author_id AS a, client_id AS b").Group("author_id, client_id").
		Order("count(*) DESC").Limit(1).Scan(&pair).Error == nil && pair.A != uuid.Nil {
		ids.noteAuthor, ids.noteClient = pair.A, pair.B
	}
	pair = struct{ A, B uuid.UUID }{}
	if db.Table("chat_sessions").Select("psychic_id AS a, client_id AS b").Where("started_at IS NOT NULL").
		Group("psychic_id, client_id").Order("count(*) DESC").Limit(1).Scan(&pair).Error == nil && pair.A != uuid.Nil {
		ids.sessionPsychic, ids.sessionClient = pair.A, pair.B
	}
	var one struct{ A uuid.UUID }
	if db.Table("credit_ledger").Select("user_id AS a").Group("user_id").Order("count(*) DESC").Limit(1).Scan(&one).Error == nil && one.A != uuid.Nil {
		ids.ledgerUser = one.A
	}
	one = struct{ A uuid.UUID }{}
	if db.Table("notifications").Select("user_id AS a").Group("user_id").Order("count(*) DESC").Limit(1).Scan(&one).Error == nil && one.A != uuid.Nil {
		ids.notifiedUser = one.A
	}
	one = struct{ A uuid.UUID }{}
	if db.Table("psychic_reviews").Select("psychic_id AS a").Group("psychic_id").Order("count(*) DESC").Limit(1).Scan(&one).Error == nil && one.A != uuid.Nil {
		ids.reviewedPsychic = one.A
	}
	one = struct{ A uuid.UUID }{}
	if db.Table("conversations").Select("psychic_id AS a").Group("psychic_id").Order("count(*) DESC").Limit(1).Scan(&one).Error == nil && one.A != uuid.Nil {
		ids.busyPsychic = one.A
	}
	one = struct{ A uuid.UUID }{}
	if db.Table("conversations").Select("id AS a").Order("last_seq DESC").Limit(1).Scan(&one).Error == nil && one.A != uuid.Nil {
		ids.busyConversation = one.A
	}
	one = struct{ A uuid.UUID }{}
	if db.Table("advisor_waitlist").Select("user_id AS a").Where("deleted_at IS NULL AND notified_at IS NULL").Group("user_id").
		Order("count(*) DESC").Limit(1).Scan(&one).Error == nil && one.A != uuid.Nil {
		ids.waitingUser = one.A
	}
	return ids
}

// revisions 3.0: chat unread. The unread totals must be answered by the
// partial indexes of migration 0019 (an index-only scan), not by walking the
// user's conversations. Sequential scans are disabled for the check so the
// assertion also holds on a small development database, where the planner
// would legitimately pick a table scan.
func TestExplainChatUnreadUsesPartialIndex(t *testing.T) {
	cfg := LoadConfig()
	live, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("database %s: %v", cfg.DSN, err)
	}
	dry, rec := newDryDB(t, cfg.DSN)
	ctx := context.Background()
	ids := pickIDs(t, live)
	for _, tc := range []struct {
		name      string
		asPsychic bool
		user      uuid.UUID
		index     string
	}{
		{"client", false, ids.sessionClient, "conversations_client_unread_idx"},
		{"advisor", true, ids.busyPsychic, "conversations_psychic_unread_idx"},
	} {
		_, _, _ = repository.NewChatRepository(dry).UnreadTotals(ctx, tc.user, tc.asPsychic)
		sql := rec.take()
		if sql == "" {
			t.Fatalf("%s: no SQL captured", tc.name)
		}
		var raw string
		err := live.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("SET LOCAL enable_seqscan = off").Error; err != nil {
				return err
			}
			return tx.Raw("EXPLAIN (FORMAT JSON) " + sql).Row().Scan(&raw)
		})
		if err != nil {
			t.Fatalf("%s: explain: %v\n%s", tc.name, err, sql)
		}
		var out []explainOut
		if err := json.Unmarshal([]byte(raw), &out); err != nil || len(out) == 0 {
			t.Fatalf("%s: parse plan: %v", tc.name, err)
		}
		var usedIndex, indexOnly bool
		walk(out[0].Plan, func(n planNode) {
			if n.Index == tc.index {
				usedIndex = true
			}
			if n.NodeType == "Index Only Scan" {
				indexOnly = true
			}
		})
		if !usedIndex || !indexOnly {
			t.Errorf("%s: want an index-only scan of %s, plan: %s", tc.name, tc.index, raw)
		}
	}
}

// --- revisions 3.0: calculator analytics ------------------------------------

type calculatorSample struct {
	last          time.Time // the newest calculator day
	link          uuid.UUID
	cursorAt      time.Time // a sign-up from the middle of the list
	cursorUser    uuid.UUID
	hasConversion bool
}

// pickCalculatorSample anchors the report windows on the newest data (the
// large seed is relative to the day it was generated) and picks a link and a
// mid-list conversions cursor.
func pickCalculatorSample(db *gorm.DB) calculatorSample {
	s := calculatorSample{last: time.Now().UTC(), link: uuid.New()}
	var day *time.Time
	if db.Raw(`SELECT max(day)::timestamptz FROM calculator_visits`).Row().Scan(&day) == nil && day != nil {
		s.last = day.UTC()
	}
	var one struct{ A uuid.UUID }
	if db.Table("web_visitors").Select("link_id AS a").Where("link_id IS NOT NULL").Group("link_id").Order("count(*) DESC").Limit(1).Scan(&one).Error == nil && one.A != uuid.Nil {
		s.link = one.A
	}
	var cur struct {
		At   time.Time
		User uuid.UUID
	}
	if db.Raw(`SELECT signed_up_at AS at, user_id AS "user" FROM web_visitors WHERE signed_up_at IS NOT NULL ORDER BY signed_up_at DESC, user_id DESC OFFSET 2000 LIMIT 1`).
		Scan(&cur).Error == nil && cur.User != uuid.Nil {
		s.cursorAt, s.cursorUser, s.hasConversion = cur.At, cur.User, true
	}
	return s
}

func calculatorChecks(s calculatorSample) []queryCheck {
	day := func(d time.Time) time.Time { return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC) }
	month := repository.CalculatorFilter{FromDay: day(s.last).AddDate(0, 0, -29), ToDay: day(s.last).AddDate(0, 0, 1)}
	year := repository.CalculatorFilter{FromDay: day(s.last).AddDate(0, 0, -364), ToDay: day(s.last).AddDate(0, 0, 1)}
	instagram := "instagram"
	filtered := month
	filtered.Kind, filtered.Source = "natal-chart", &instagram
	byLink := month
	byLink.LinkID = &s.link
	checks := []queryCheck{
		{"calculators: totals (30 days)", 400 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewCalculatorRepository(d).Totals(ctx, month)
		}},
		{"calculators: by kind (30 days)", 400 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewCalculatorRepository(d).ByDimension(ctx, month, "kind")
		}},
		{"calculators: by locale (30 days)", 400 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewCalculatorRepository(d).ByDimension(ctx, month, "locale")
		}},
		{"calculators: by source (30 days)", 400 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewCalculatorRepository(d).BySource(ctx, month, 50)
		}},
		{"calculators: by link (30 days)", 400 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewCalculatorRepository(d).ByLink(ctx, month, 50)
		}},
		{"calculators: visitor series (30 days)", 400 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewCalculatorRepository(d).Series(ctx, month, "day")
		}},
		{"calculators: filtered series", 400 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewCalculatorRepository(d).Series(ctx, filtered, "day")
		}},
		{"calculators: sign-up series (30 days)", 400 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewCalculatorRepository(d).SignupSeries(ctx, month, "week")
		}},
		{"calculators: filtered totals", 400 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewCalculatorRepository(d).Totals(ctx, filtered)
		}},
		{"calculators: link totals", 400 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewCalculatorRepository(d).Totals(ctx, byLink)
		}},
		{"calculators: conversions page 1 (12 months)", 250 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewCalculatorRepository(d).Conversions(ctx, year, nil, 26)
		}},
		{"calculators: conversions page 1 (30 days)", 400 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewCalculatorRepository(d).Conversions(ctx, month, nil, 26)
		}},
	}
	if s.hasConversion {
		cursor := &repository.Cursor{At: s.cursorAt, ID: s.cursorUser.String()}
		checks = append(checks, queryCheck{"calculators: conversions deep page", 250 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewCalculatorRepository(d).Conversions(ctx, year, cursor, 26)
		}})
	}
	return checks
}
