package e2e

// revisions 3.0: support desk at size. About 100,000 tickets are bulk-inserted
// with generate_series (a realistic mix of statuses, priorities, assignees,
// topics and subjects) and every filter, search and sort of the admin queue is
// walked page by page and compared with an independent SQL ORDER BY: the two
// sequences must be identical (no duplicate, no gap, same order), within a
// generous latency bound. Set SUPPORT_LARGE_KEEP=1 to keep the rows, for the
// EXPLAIN checks in tests/load.

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

const largeTickets = 100_000

type timing struct {
	mu sync.Mutex
	d  []time.Duration
}

func (tm *timing) add(d time.Duration) { tm.mu.Lock(); tm.d = append(tm.d, d); tm.mu.Unlock() }

func (tm *timing) p95() time.Duration {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if len(tm.d) == 0 {
		return 0
	}
	s := append([]time.Duration(nil), tm.d...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[(len(s)*95+99)/100-1]
}

func (tm *timing) max() time.Duration {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	var m time.Duration
	for _, d := range tm.d {
		if d > m {
			m = d
		}
	}
	return m
}

func TestSupportDeskLargeDatasetFiltersSortsAndKeysetPagination(t *testing.T) {
	if testing.Short() {
		t.Skip("large dataset")
	}
	e, admin := newDesk(t)
	db := e.app.DB
	marker := "bulk" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")

	// Actors: administrators to assign to and signed-in requesters.
	var adminIDs []uuid.UUID
	adminIDs = append(adminIDs, admin.adminID)
	for i := 0; i < 3; i++ {
		id, _ := e.insertAdmin()
		adminIDs = append(adminIDs, id)
	}
	clients := []*client{e.registerClient(), e.registerClient(), e.registerClient()}
	vipEmail := "vip-" + marker + "@example.com"
	vipHash := security.Keyed(e.app.Config.Analytics.IPHashSecret, "support-email:"+vipEmail, 0)

	t.Cleanup(func() {
		if os.Getenv("SUPPORT_LARGE_KEEP") == "" {
			db.Exec("DELETE FROM support_tickets WHERE name = ?", marker)
		}
	})
	start := time.Now()
	insert := fmt.Sprintf(`
INSERT INTO support_tickets (id, reference, user_id, name, email_enc, email_hash, topic, subject, message_enc, message_chars, locale, status,
	ip_hash, created_at, updated_at, priority, assignee_id, version, message_count, last_message_at, last_customer_message_at,
	last_staff_message_at, first_response_at, resolved_at, closed_at, staff_seen_at, email_masked)
SELECT gen_random_uuid(),
	'SY-ZZ' || substr(a, ((i >> 15) & 31) + 1, 1) || substr(a, ((i >> 10) & 31) + 1, 1) || substr(a, ((i >> 5) & 31) + 1, 1) || substr(a, (i & 31) + 1, 1),
	CASE WHEN (h / 10000000) %% 5 = 0 THEN (ARRAY[?::uuid, ?::uuid, ?::uuid])[1 + (h / 17) %% 3] END,
	?, 'x', CASE WHEN i %% 2000 = 0 THEN ? ELSE md5(i::text) || md5(i::text || 'x') END,
	(ARRAY['general', 'privacy', 'billing', 'advisor', 'technical'])[1 + (h / 100000) %% 5],
	(ARRAY['Refund', 'Login', 'Charge', 'Advisor', 'Export', 'Privacy', 'Invoice', 'Chat'])[1 + (h / 7) %% 8] || ' request ' || ? || ' ' || (i %% 977),
	'x', 20, (ARRAY['en', 'en', 'en', 'en', 'en', 'en', 'en', 'fr', 'fr', 'es'])[1 + (h / 11) %% 10], st,
	'', created, created, pr,
	CASE WHEN st <> 'open' AND (h / 10000) %% 10 < 7 THEN (ARRAY[?::uuid, ?::uuid, ?::uuid, ?::uuid])[1 + (h / 1000000) %% 4] END,
	1, CASE WHEN st = 'open' THEN 1 ELSE 2 END,
	CASE WHEN st = 'open' THEN lcma ELSE lcma + interval '10 minutes' END, lcma,
	CASE WHEN st <> 'open' THEN lcma + interval '10 minutes' END, CASE WHEN st <> 'open' THEN lcma + interval '10 minutes' END,
	CASE WHEN st = 'resolved' THEN lcma + interval '1 hour' END, CASE WHEN st = 'closed' THEN lcma + interval '2 hours' END,
	CASE WHEN (h / 100000000) %% 10 < 6 THEN lcma + interval '1 minute' END, 'b•••@e•••.com'
FROM (
	SELECT i, h, 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789' AS a,
		CASE WHEN h %% 100 < 30 THEN 'open' WHEN h %% 100 < 50 THEN 'pending' WHEN h %% 100 < 70 THEN 'resolved' ELSE 'closed' END AS st,
		CASE WHEN (h / 100) %% 100 < 15 THEN 'low' WHEN (h / 100) %% 100 < 75 THEN 'normal' WHEN (h / 100) %% 100 < 93 THEN 'high' ELSE 'urgent' END AS pr,
		now() - interval '1 hour' - (i / 3) * interval '1 minute' AS created,
		now() - interval '1 hour' - (i / 3) * interval '1 minute' + ((h / 13) %% 500) * interval '1 minute' AS lcma
	FROM (SELECT i, (i::bigint * 2654435761) %% 4294967296 AS h FROM generate_series(1, %d) AS i) AS x
) AS g`, largeTickets)
	args := []any{clients[0].user.ID, clients[1].user.ID, clients[2].user.ID, marker, vipHash, marker,
		adminIDs[0], adminIDs[1], adminIDs[2], adminIDs[3]}
	if err := db.Exec(insert, args...).Error; err != nil {
		t.Fatalf("bulk insert: %v", err)
	}
	if err := db.Exec("ANALYZE support_tickets").Error; err != nil {
		t.Fatal(err)
	}
	var total struct{ N int }
	db.Raw("SELECT count(*) AS n FROM support_tickets WHERE name = ?", marker).Scan(&total)
	if total.N != largeTickets {
		t.Fatalf("seeded %d tickets, want %d", total.N, largeTickets)
	}
	t.Logf("seeded %d tickets in %s", total.N, time.Since(start).Round(time.Millisecond))

	// sqlRefs returns the expected references in order.
	sqlRefs := func(where, order string, args ...any) []string {
		var refs []string
		q := "SELECT reference FROM support_tickets WHERE name = ? AND deleted_at IS NULL"
		if where != "" {
			q += " AND " + where
		}
		if err := db.Raw(q+" ORDER BY "+order, append([]any{marker}, args...)...).Scan(&refs).Error; err != nil {
			t.Fatal(err)
		}
		return refs
	}
	const newest = "created_at DESC, id DESC"
	const oldest = "created_at ASC, id ASC"
	const waiting = "last_customer_message_at ASC, id ASC"
	const waitingMixed = "(CASE WHEN status = 'open' THEN 0 ELSE 1 END), last_customer_message_at ASC, id ASC"

	// walk pages the queue (limit 100) and returns every reference in order.
	walk := func(name, query string, tm *timing, maxPages int) []string {
		t.Helper()
		var out []string
		seen := map[string]bool{}
		cursor := ""
		for page := 0; page < 2000; page++ {
			if maxPages > 0 && page == maxPages {
				return out
			}
			q := adminTickets + query + "&limit=100"
			if cursor != "" {
				q += "&cursor=" + url.QueryEscape(cursor)
			}
			began := time.Now()
			r := admin.call(http.MethodGet, q, nil)
			tm.add(time.Since(began))
			if r.status != http.StatusOK {
				t.Fatalf("%s page %d: %d %s", name, page, r.status, r.body)
			}
			var p service.Page[service.AdminTicketRow]
			r.decode(t, &p)
			for _, it := range p.Items {
				if seen[it.Reference] {
					t.Fatalf("%s: duplicate %s on page %d", name, it.Reference, page)
				}
				seen[it.Reference] = true
				out = append(out, it.Reference)
			}
			if p.NextCursor == "" {
				return out
			}
			if len(p.Items) != 100 {
				t.Fatalf("%s: page %d has %d items but a cursor", name, page, len(p.Items))
			}
			cursor = p.NextCursor
		}
		t.Fatalf("%s: pagination did not end", name)
		return nil
	}
	// check walks the whole result (or the first maxPages pages) and compares it with SQL.
	check := func(name, query string, want []string, maxPages ...int) {
		t.Helper()
		var tm timing
		began := time.Now()
		limit := 0
		if len(maxPages) > 0 {
			limit = maxPages[0]
			if limit*100 < len(want) {
				want = want[:limit*100]
			}
		}
		if len(want) == 0 && !strings.Contains(name, "unknown") {
			t.Fatalf("%s: the SQL expectation is empty, the check would be vacuous", name)
		}
		got := walk(name, query, &tm, limit)
		if len(got) != len(want) {
			t.Fatalf("%s: %d rows, SQL says %d", name, len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s: row %d is %s, SQL order says %s", name, i, got[i], want[i])
			}
		}
		if tm.p95() > 400*time.Millisecond || tm.max() > 3*time.Second {
			t.Errorf("%s: slow pages: p95 %s, max %s", name, tm.p95(), tm.max())
		}
		t.Logf("%-46s %6d rows, %4d pages, p95 %-8s max %-8s total %s", name, len(got), len(tm.d), tm.p95().Round(time.Millisecond),
			tm.max().Round(time.Millisecond), time.Since(began).Round(time.Millisecond))
	}
	q := "?q=" + marker

	// Every sort, every filter, every combination worth having.
	check("newest, all (first 300 pages)", q, sqlRefs("", newest), 300)
	check("oldest, pending+resolved", q+"&sort=oldest&status=pending,resolved", sqlRefs("status IN ('pending','resolved')", oldest))
	check("waiting, open", q+"&sort=waiting&status=open", sqlRefs("status = 'open'", waiting))
	check("waiting, open+pending (ranked)", q+"&sort=waiting&status=open,pending&priority=high", sqlRefs("status IN ('open','pending') AND priority = 'high'", waitingMixed))
	check("waiting, open+pending (ranked, first 200 pages)", q+"&sort=waiting&status=open,pending", sqlRefs("status IN ('open','pending')", waitingMixed), 200)
	check("waiting, every status (ranked)", q+"&sort=waiting&priority=high", sqlRefs("priority = 'high'", waitingMixed))
	check("assignee + priority", q+"&assignee="+adminIDs[1].String()+"&priority=high",
		sqlRefs("assignee_id = ? AND priority = 'high'", newest, adminIDs[1]))
	check("assignee=me", q+"&assignee=me&sort=oldest", sqlRefs("assignee_id = ?", oldest, admin.adminID))
	check("unassigned, oldest", q+"&assignee=unassigned&sort=oldest&status=open,pending", sqlRefs("assignee_id IS NULL AND status IN ('open','pending')", oldest))
	check("user_id", "?user_id="+clients[0].user.ID.String(), sqlRefs("user_id = ?", newest, clients[0].user.ID))
	check("topic + status", q+"&topic=billing&status=closed", sqlRefs("topic = 'billing' AND status = 'closed'", newest))
	check("subject words", "?q="+url.QueryEscape("refund "+marker), sqlRefs("subject LIKE ?", newest, "Refund request "+marker+"%"))
	check("subject word + filters", "?q="+url.QueryEscape("LOGIN "+marker)+"&status=resolved&priority=low&sort=oldest",
		sqlRefs("subject LIKE ? AND status = 'resolved' AND priority = 'low'", oldest, "Login request "+marker+"%"))
	check("e-mail (keyed hash, case-insensitive)", "?q="+url.QueryEscape(strings.ToUpper(vipEmail)), sqlRefs("email_hash = ?", newest, vipHash))
	if got := sqlRefs("email_hash = ?", newest, vipHash); len(got) != largeTickets/2000 {
		t.Fatalf("vip rows: %d", len(got))
	}
	// A reference is exact and case-insensitive.
	ref := sqlRefs("", newest)[largeTickets/2]
	check("reference", "?q="+ref, []string{ref})
	check("reference, lowercase", "?q="+strings.ToLower(ref), []string{ref})
	check("unknown reference", "?q=SY-AAAAAA&status=open", sqlRefs("reference = 'SY-AAAAAA'", newest))

	// Deep pages cost the same as the first (keyset, not OFFSET): jump to the middle with a real cursor.
	var first service.Page[service.AdminTicketRow]
	admin.must(http.StatusOK, http.MethodGet, adminTickets+q+"&limit=100", nil, &first)
	var deep timing
	cursor := first.NextCursor
	for i := 0; i < 200 && cursor != ""; i++ {
		var p service.Page[service.AdminTicketRow]
		began := time.Now()
		admin.must(http.StatusOK, http.MethodGet, adminTickets+q+"&limit=100&cursor="+url.QueryEscape(cursor), nil, &p)
		deep.add(time.Since(began))
		cursor = p.NextCursor
	}
	var shallow timing
	for i := 0; i < 30; i++ {
		began := time.Now()
		admin.must(http.StatusOK, http.MethodGet, adminTickets+q+"&limit=100", nil, nil)
		shallow.add(time.Since(began))
	}
	t.Logf("first page p95 %s, pages 2–201 p95 %s", shallow.p95().Round(time.Millisecond), deep.p95().Round(time.Millisecond))
	if deep.p95() > 400*time.Millisecond {
		t.Errorf("deep keyset pages are slow: p95 %s", deep.p95())
	}

	// The summary is exact at size, and fast.
	e.newTicket(e.client(), uniq("sum")+"@example.com", "Invalidate the summary", "Creating a ticket invalidates the cached summary.")
	var want service.SupportSummary
	var byStatus []struct {
		Status string
		N      int64
	}
	db.Raw("SELECT status, count(*) AS n FROM support_tickets WHERE deleted_at IS NULL GROUP BY status").Scan(&byStatus)
	for _, s := range byStatus {
		switch s.Status {
		case "open":
			want.Open = s.N
		case "pending":
			want.Pending = s.N
		case "resolved":
			want.Resolved = s.N
		case "closed":
			want.Closed = s.N
		}
	}
	want.NeedsReply = want.Open
	db.Raw("SELECT count(*) FROM support_tickets WHERE deleted_at IS NULL AND assignee_id IS NULL AND status IN ('open','pending')").Scan(&want.Unassigned)
	db.Raw("SELECT count(*) FROM support_tickets WHERE deleted_at IS NULL AND assignee_id = ? AND status IN ('open','pending')", admin.adminID).Scan(&want.Mine)
	var sumT timing
	var got service.SupportSummary
	for i := 0; i < 20; i++ {
		began := time.Now()
		admin.must(http.StatusOK, http.MethodGet, adminSummary, nil, &got)
		sumT.add(time.Since(began))
	}
	if got != want {
		t.Fatalf("summary at size: got %+v, want %+v", got, want)
	}
	if sumT.max() > 2*time.Second {
		t.Errorf("summary too slow: max %s", sumT.max())
	}
	t.Logf("summary p95 %s max %s (first call includes the count)", sumT.p95().Round(time.Millisecond), sumT.max().Round(time.Millisecond))

	// A write on a bulk ticket stays fast, and optimistic concurrency holds.
	var one service.AdminTicketRow
	began := time.Now()
	r := adminPatch(admin, ref, map[string]any{"priority": "urgent", "assignee_id": adminIDs[2].String(), "expected_version": 1})
	if r.status != http.StatusOK {
		t.Fatalf("patch at size: %d %s", r.status, r.body)
	}
	r.decode(t, &one)
	if d := time.Since(began); d > time.Second || one.Version != 2 || one.Priority != "urgent" {
		t.Fatalf("patch at size: %s %+v", d, one)
	}

	// A signed-in requester with thousands of tickets pages through their own list.
	owner := clients[0]
	var ownTm timing
	var ownRows []string
	seen := map[string]bool{}
	cursor = ""
	for page := 0; page < 400; page++ {
		path := "/api/v1/me/support/tickets?limit=50"
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}
		began := time.Now()
		var p service.Page[service.CustomerTicketRow]
		owner.must(http.StatusOK, http.MethodGet, path, nil, &p)
		ownTm.add(time.Since(began))
		for _, it := range p.Items {
			if seen[it.Reference] {
				t.Fatalf("/me: duplicate %s", it.Reference)
			}
			seen[it.Reference] = true
			ownRows = append(ownRows, it.Reference)
		}
		if p.NextCursor == "" {
			break
		}
		cursor = p.NextCursor
	}
	var wantOwn []string
	db.Raw("SELECT reference FROM support_tickets WHERE user_id = ? ORDER BY last_message_at DESC, id DESC", owner.user.ID).Scan(&wantOwn)
	if len(ownRows) != len(wantOwn) || len(wantOwn) < 1000 {
		t.Fatalf("/me list: %d rows, SQL says %d", len(ownRows), len(wantOwn))
	}
	for i := range wantOwn {
		if ownRows[i] != wantOwn[i] {
			t.Fatalf("/me list: row %d is %s, SQL says %s", i, ownRows[i], wantOwn[i])
		}
	}
	if ownTm.p95() > 400*time.Millisecond {
		t.Errorf("/me pages slow: p95 %s", ownTm.p95())
	}
	t.Logf("/me list: %d tickets, %d pages, p95 %s", len(ownRows), len(ownTm.d), ownTm.p95().Round(time.Millisecond))

	// Concurrent readers: mixed queue views from many workers, no errors, bounded latency.
	paths := []string{
		adminTickets + q + "&limit=25", adminTickets + q + "&status=open&sort=waiting", adminTickets + q + "&assignee=unassigned&status=open,pending",
		adminTickets + q + "&priority=urgent&sort=oldest", adminTickets + "?q=" + url.QueryEscape("refund "+marker), adminSummary,
		adminTickets + "?q=" + ref, adminTickets + "?user_id=" + clients[1].user.ID.String() + "&limit=50",
	}
	var wg sync.WaitGroup
	var conc timing
	errs := make(chan string, 400)
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				path := paths[(w+i)%len(paths)]
				req, _ := http.NewRequest(http.MethodGet, e.srv.URL+path, nil)
				req.Header.Set("Origin", adminOrigin)
				began := time.Now()
				res, err := admin.http.Do(req)
				if err != nil {
					errs <- err.Error()
					return
				}
				res.Body.Close()
				conc.add(time.Since(began))
				if res.StatusCode != http.StatusOK {
					errs <- fmt.Sprintf("%s: %d", path, res.StatusCode)
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Errorf("concurrent read: %s", msg)
	}
	if conc.p95() > time.Second {
		t.Errorf("concurrent reads slow: p95 %s", conc.p95())
	}
	t.Logf("concurrent: %d requests, p95 %s max %s", len(conc.d), conc.p95().Round(time.Millisecond), conc.max().Round(time.Millisecond))
}
