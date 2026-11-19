//go:build load

package load

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/app"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/seed"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
)

// TestLoadTrialHold (revisions 6.0, §26.1) drives thousands of readings
// through the whole life of a hold, concurrently, at the service level (the
// application is booted in-process with a fake clock, because a hold lives
// minutes and the billing worker must run on demand):
//
//	enter    LOAD_HOLD_READINGS trial readings (default 1000, at most one per
//	         synthetic advisor) reach the end of their free minute; BillDue runs
//	         on LOAD_HOLD_WORKERS goroutines (default 16) and holds them all;
//	write    every advisor writes two replies, which are stored held;
//	convert  a third of the clients top up and ResumeHolds converts them
//	         (one minute charged, replies revealed);
//	expire   the clock passes the deadline and ExpireHolds ends the rest;
//	reveal   a third of those clients pay to read the replies, a few of them
//	         twice at once (one charge, one earning).
//
// It checks the counts, the ledger, the earnings and the credit-lot invariant
// at the end, prints the latency of every step, and then removes everything it
// created and restores what it touched (wallets, advisor reading counters).
//
// It needs a database of its own (it applies pending migrations to it) and a
// Redis index of its own: LOAD_DATABASE_DSN and LOAD_REDIS_URL (the test is
// skipped without the latter, so it never touches a stack by accident).
func TestLoadTrialHold(t *testing.T) {
	cfg := LoadConfig()
	redisURL := os.Getenv("LOAD_REDIS_URL")
	if redisURL == "" {
		t.Skip("set LOAD_REDIS_URL (a Redis index of your own) and LOAD_DATABASE_DSN to run the hold scenario")
	}
	readings, workers := envInt("LOAD_HOLD_READINGS", 1000), envInt("LOAD_HOLD_WORKERS", 16)
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("database %s: %v", cfg.DSN, err)
	}
	start := time.Now().Add(-time.Minute)

	// --- the application, in-process, with a clock the test drives ---
	appCfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	appCfg.App.Env = config.EnvTest
	appCfg.Database.DSN = cfg.DSN
	appCfg.Redis.URL, appCfg.Redis.KeyPrefix = redisURL, "loadhold:"+uuid.NewString()[:8]+":"
	appCfg.Auth.Argon2Memory, appCfg.Auth.Argon2Iterations, appCfg.Auth.Argon2Parallelism = 1024, 1, 1
	appCfg.Worker.Enabled = false
	appCfg.RateLimit.Scale = 1000
	appCfg.Chat.MessageRate = 1_000_000
	appCfg.Security.AllowedOrigins = []string{cfg.Origin}
	appCfg.Log.Level = "error"
	appCfg.Database.MaxOpenConns = max(workers*2, 20)
	if err := appCfg.Validate(); err != nil {
		t.Fatalf("config: %v", err)
	}
	fake := clock.NewFake(time.Now())
	a, err := app.New(context.Background(), appCfg, zap.NewNop(), app.Overrides{Clock: fake, Mailer: &mail.Recorder{}, Tracker: &analytics.Recorder{},
		Storage: storage.NewMemory("https://assets.test"), AdvisorStorage: storage.NewMemory("https://advisors.test"), ChatStorage: storage.NewMemory("")})
	if err != nil {
		t.Fatalf("app: %v", err)
	}
	chat, wallet := a.Services.Chat, a.Services.Wallet
	ctx := context.Background()

	// --- the cast: synthetic advisors and clients that share nothing yet ---
	var advisors, clients []uuid.UUID
	db.Raw(`SELECT p.user_id FROM psychics p JOIN users u ON u.id = p.user_id
		WHERE u.email LIKE ? AND NOT EXISTS (SELECT 1 FROM chat_sessions s WHERE s.psychic_id = p.user_id AND s.status IN ?)
		ORDER BY p.user_id LIMIT ?`, "load-%@"+seed.LoadEmailDomain, domain.OpenSessionStatuses, readings).Scan(&advisors)
	db.Raw(`SELECT w.user_id FROM wallets w JOIN users u ON u.id = w.user_id
		WHERE u.email LIKE ? AND w.balance = 0 AND w.held = 0
		AND NOT EXISTS (SELECT 1 FROM credit_lots l WHERE l.user_id = w.user_id)
		AND NOT EXISTS (SELECT 1 FROM chat_sessions s WHERE s.client_id = w.user_id AND s.status IN ?)
		ORDER BY w.user_id LIMIT ?`, "load-%@"+seed.LoadEmailDomain, domain.OpenSessionStatuses, readings*2).Scan(&clients)
	n := min(len(advisors), len(clients), readings)
	if n < 3 {
		t.Skip("not enough synthetic advisors and clients without an open reading (run cmd/seed -profile=large)")
	}
	advisors = advisors[:n]
	// A pair must not have a conversation yet: the test creates and later deletes its own.
	taken := map[[2]uuid.UUID]bool{}
	var existing []struct{ ClientID, PsychicID uuid.UUID }
	db.Raw(`SELECT client_id, psychic_id FROM conversations WHERE client_id IN ? AND psychic_id IN ?`, clients, advisors).Scan(&existing)
	for _, c := range existing {
		taken[[2]uuid.UUID{c.ClientID, c.PsychicID}] = true
	}
	pairs := make([][2]uuid.UUID, 0, n) // client, advisor
	used := map[uuid.UUID]bool{}
	for _, adv := range advisors {
		for _, cl := range clients {
			if !used[cl] && !taken[[2]uuid.UUID{cl, adv}] {
				used[cl] = true
				pairs = append(pairs, [2]uuid.UUID{cl, adv})
				break
			}
		}
	}
	n = len(pairs)
	t.Logf("trial hold scenario: %d readings, %d workers", n, workers)

	var origWallets []domain.Wallet
	clientIDs := make([]uuid.UUID, n)
	advisorIDs := make([]uuid.UUID, n)
	for i, p := range pairs {
		clientIDs[i], advisorIDs[i] = p[0], p[1]
	}
	db.Where("user_id IN ?", clientIDs).Find(&origWallets)

	chatRepo := repository.NewChatRepository(db)
	var convIDs, sessionIDs []uuid.UUID
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		a.Shutdown(sctx)
		cleanupTrialHold(t, db, start, clientIDs, convIDs, sessionIDs, origWallets)
	}()

	// --- set up n trial readings at the end of their free minute ---
	now := fake.Now()
	if err := db.Exec(`UPDATE wallets SET free_minutes = 0, balance = 0, held = 0 WHERE user_id IN ?`, clientIDs).Error; err != nil {
		t.Fatal(err)
	}
	sessions := make([]domain.Session, 0, n)
	for i, p := range pairs {
		conv, err := chatRepo.GetOrCreateConversation(ctx, p[0], p[1])
		if err != nil {
			t.Fatalf("conversation: %v", err)
		}
		convIDs = append(convIDs, conv.ID)
		started, due := now.Add(-time.Minute), now
		sessions = append(sessions, domain.Session{ID: domain.NewID(), ConversationID: conv.ID, ClientID: p[0], PsychicID: p[1], Status: domain.SessionActive,
			RatePerMinute: 40, ListRatePerMinute: 40, RequestKey: "loadhold-" + uuid.NewString(), RequestedAt: now.Add(-2 * time.Minute),
			ExpiresAt: now.Add(-time.Hour), AcceptedAt: &started, StartedAt: &started, LastResumedAt: &started, NextBillingAt: &due,
			ActiveSeconds: 0, BilledMinutes: 1, FreeMinutesUsed: 1, CreatedAt: now, UpdatedAt: now})
		sessionIDs = append(sessionIDs, sessions[i].ID)
	}
	if err := db.CreateInBatches(&sessions, 500).Error; err != nil {
		t.Fatalf("sessions: %v", err)
	}

	var steps []string
	step := func(name string, items int, d time.Duration, lats []time.Duration) {
		line := fmt.Sprintf("%-8s %5d items in %-9s %6.0f/s", name, items, d.Round(time.Millisecond), float64(items)/d.Seconds())
		if len(lats) > 0 {
			sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
			line += fmt.Sprintf("  p50 %-8s p95 %-8s max %s", lats[len(lats)/2].Round(time.Microsecond), lats[len(lats)*95/100].Round(time.Microsecond), lats[len(lats)-1].Round(time.Microsecond))
		}
		steps = append(steps, line)
		t.Log(line)
	}
	var failures atomic.Int64
	check := func(what string, err error) {
		if err != nil {
			if failures.Add(1) <= 5 {
				t.Errorf("%s: %v", what, err)
			}
		}
	}
	// parallel runs fn(i) for i < n on the workers and returns each call's latency.
	parallel := func(items int, fn func(i int) error, what string) (time.Duration, []time.Duration) {
		lats := make([]time.Duration, items)
		var next atomic.Int64
		var wg sync.WaitGroup
		began := time.Now()
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					i := int(next.Add(1) - 1)
					if i >= items {
						return
					}
					t0 := time.Now()
					check(what, fn(i))
					lats[i] = time.Since(t0)
				}
			}()
		}
		wg.Wait()
		return time.Since(began), lats
	}

	// --- enter: the free minute runs out, BillDue holds every reading ---
	fake.Advance(time.Second)
	var billed atomic.Int64
	began := time.Now()
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := chat.BillDue(ctx, 1<<30)
			check("BillDue", err)
			billed.Add(int64(c))
		}()
	}
	wg.Wait()
	step("enter", int(billed.Load()), time.Since(began), nil)
	if got := countRows(db, `SELECT count(*) FROM chat_session_holds WHERE session_id IN ? AND status = 'open'`, sessionIDs); got != int64(n) || billed.Load() != int64(n) {
		t.Fatalf("held %d of %d readings (BillDue processed %d)", got, n, billed.Load())
	}
	if got := countRows(db, `SELECT count(*) FROM chat_sessions WHERE id IN ? AND status = 'held' AND next_billing_at IS NULL AND hold_expires_at IS NOT NULL`, sessionIDs); got != int64(n) {
		t.Fatalf("%d of %d sessions are held with a deadline", got, n)
	}

	// --- write: two held replies per advisor ---
	d, lats := parallel(n*2, func(i int) error {
		i /= 2
		_, err := chat.SendToConversation(ctx, reqctx.Principal{UserID: advisorIDs[i], Role: domain.RolePsychic},
			service.ConversationSendInput{ConversationID: convIDs[i], Body: fmt.Sprintf("reply %d for the hold", i), ClientMsgID: "lh-" + uuid.NewString()})
		return err
	}, "send")
	step("write", n*2, d, lats)
	if got := countRows(db, `SELECT count(*) FROM chat_messages WHERE session_id IN ? AND held`, sessionIDs); got != int64(n*2) {
		t.Fatalf("%d held messages, want %d", got, n*2)
	}
	if got := countRows(db, `SELECT count(*) FROM chat_session_holds WHERE session_id IN ? AND held_messages = 2`, sessionIDs); got != int64(n) {
		t.Fatalf("%d holds count two messages, want %d", got, n)
	}

	// --- convert: a third of the clients top up ---
	third := func(i int) int { return i % 3 } // 0 converts, 1 expires, 2 expires and reveals
	grant := func(i int) error {
		return database.NewTxManager(db).WithinTx(ctx, func(ctx context.Context) error {
			_, _, err := wallet.Apply(ctx, service.Movement{UserID: clientIDs[i], Type: domain.LedgerAdminAdjustment, Amount: 100, ReferenceType: "admin",
				IdempotencyKey: "loadhold-grant:" + uuid.NewString(), Description: "load test", Lot: &service.LotSpec{Source: domain.LotBonus}})
			return err
		})
	}
	var converters []int
	for i := 0; i < n; i++ {
		if third(i) == 0 {
			converters = append(converters, i)
		}
	}
	d, lats = parallel(len(converters), func(k int) error {
		i := converters[k]
		if err := grant(i); err != nil {
			return err
		}
		chat.ResumeHolds(ctx, clientIDs[i])
		return nil
	}, "convert")
	step("convert", len(converters), d, lats)

	// --- expire: the clock passes the deadline, the rest ends ---
	fake.Advance(61 * time.Second)
	var expired atomic.Int64
	began = time.Now()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := chat.ExpireHolds(ctx, 1<<30)
			check("ExpireHolds", err)
			expired.Add(int64(c))
		}()
	}
	wg.Wait()
	wantExpired := n - len(converters)
	step("expire", int(expired.Load()), time.Since(began), nil)
	if expired.Load() != int64(wantExpired) {
		t.Fatalf("ExpireHolds ended %d readings, want %d", expired.Load(), wantExpired)
	}

	// --- reveal: a third of the expired clients pay to read; some press twice at once ---
	var revealers []int
	for i := 0; i < n; i++ {
		if third(i) == 2 {
			revealers = append(revealers, i)
		}
	}
	holdOf := map[int]uuid.UUID{}
	for _, i := range revealers {
		var ids []uuid.UUID
		db.Raw(`SELECT id FROM chat_session_holds WHERE session_id = ?`, sessionIDs[i]).Pluck("id", &ids)
		if len(ids) != 1 {
			t.Fatalf("reading %d has %d holds", i, len(ids))
		}
		holdOf[i] = ids[0]
	}
	d, lats = parallel(len(revealers)+len(revealers)/5, func(k int) error {
		i := revealers[k%len(revealers)]
		if k < len(revealers) {
			if err := grant(i); err != nil {
				return err
			}
		}
		for {
			_, err := chat.RevealHold(ctx, clientIDs[i], holdOf[i], "reveal-"+uuid.NewString())
			if err != nil && strings.Contains(err.Error(), "insufficient") && k >= len(revealers) {
				time.Sleep(5 * time.Millisecond) // the duplicate raced the grant of its twin
				continue
			}
			return err
		}
	}, "reveal")
	step("reveal", len(revealers), d, lats)

	// --- the verdict ---
	holdCount := func(status string) int64 {
		return countRows(db, `SELECT count(*) FROM chat_session_holds WHERE session_id IN ? AND status = ?`, sessionIDs, status)
	}
	if holdCount("open") != 0 || holdCount("converted") != int64(len(converters)) || holdCount("revealed") != int64(len(revealers)) ||
		holdCount("expired") != int64(wantExpired-len(revealers)) {
		t.Fatalf("holds: open %d converted %d (want %d) expired %d (want %d) revealed %d (want %d)", holdCount("open"), holdCount("converted"),
			len(converters), holdCount("expired"), wantExpired-len(revealers), holdCount("revealed"), len(revealers))
	}
	if got := countRows(db, `SELECT count(*) FROM chat_sessions WHERE id IN ? AND status = 'held'`, sessionIDs); got != 0 {
		t.Fatalf("%d readings are still held", got)
	}
	if got := countRows(db, `SELECT count(*) FROM chat_sessions WHERE id IN ? AND end_reason = 'hold_expired' AND status IN ('ended', 'settled')`, sessionIDs); got != int64(wantExpired) {
		t.Fatalf("%d readings ended with hold_expired, want %d", got, wantExpired)
	}
	if got := countRows(db, `SELECT count(*) FROM chat_messages WHERE session_id IN ? AND held`, sessionIDs); got != int64((wantExpired-len(revealers))*2) {
		t.Fatalf("%d messages still held, want %d (the expired and never revealed)", got, (wantExpired-len(revealers))*2)
	}
	if got := countRows(db, `SELECT count(*) FROM credit_ledger WHERE user_id IN ? AND entry_type = 'chat_charge' AND created_at >= ?`, clientIDs, start); got != int64(len(converters)) {
		t.Fatalf("%d minute charges, want %d (one per conversion)", got, len(converters))
	}
	if got := countRows(db, `SELECT count(*) FROM credit_ledger WHERE user_id IN ? AND entry_type = 'chat_reveal' AND created_at >= ?`, clientIDs, start); got != int64(len(revealers)) {
		t.Fatalf("%d reveal charges for %d reveals (a duplicate charged twice?)", got, len(revealers))
	}
	if got := countRows(db, `SELECT count(*) FROM psychic_earnings WHERE source_type = 'chat_hold_reveal' AND psychic_id IN ? AND created_at >= ?`, advisorIDs, start); got != int64(len(revealers)) {
		t.Fatalf("%d reveal earnings for %d reveals", got, len(revealers))
	}
	if got := countRows(db, `SELECT count(*) FROM wallets w WHERE w.user_id IN ? AND w.balance <>
		COALESCE((SELECT sum(credits_remaining) FROM credit_lots l WHERE l.user_id = w.user_id), 0)`, clientIDs); got != 0 {
		t.Fatalf("%d wallets disagree with their lots", got)
	}
	if failures.Load() > 0 {
		t.Fatalf("%d operations failed", failures.Load())
	}
	t.Logf("\n%s", strings.Join(steps, "\n"))
}

func countRows(db *gorm.DB, query string, args ...any) int64 {
	var n int64
	db.Raw(query, args...).Scan(&n)
	return n
}

// cleanupTrialHold removes everything TestLoadTrialHold created and restores
// what it touched. Rows are found by the ids the test kept and by the users it
// used, never by pattern alone. The credit ledger and the lot allocations are
// append-only (a trigger refuses every UPDATE and DELETE), so the synthetic
// movements are removed in one transaction that switches the triggers off for
// itself (session_replication_role = replica, which needs a superuser, as the
// load database's owner is); nothing else is touched in that transaction.
func cleanupTrialHold(t *testing.T, db *gorm.DB, since time.Time, clients, convs, sessions []uuid.UUID, wallets []domain.Wallet) {
	t.Helper()
	var holds, settled []uuid.UUID
	db.Raw(`SELECT id FROM chat_session_holds WHERE session_id IN ?`, sessions).Pluck("id", &holds)
	db.Raw(`SELECT psychic_id FROM chat_sessions WHERE id IN ? AND status = 'settled' AND billed_minutes > 0`, sessions).Pluck("psychic_id", &settled)
	err := db.Transaction(func(tx *gorm.DB) error {
		steps := []struct {
			query string
			args  []any
		}{
			{`SET LOCAL session_replication_role = replica`, nil},
			{`DELETE FROM psychic_earnings WHERE source_id IN ?`, []any{sessions}},
			{`DELETE FROM chat_messages WHERE session_id IN ?`, []any{sessions}},
			{`UPDATE conversations SET last_message_id = NULL WHERE id IN ?`, []any{convs}},
			{`UPDATE chat_sessions SET last_hold_id = NULL WHERE id IN ?`, []any{sessions}},
			{`DELETE FROM chat_session_holds WHERE session_id IN ?`, []any{sessions}},
			{`DELETE FROM chat_sessions WHERE id IN ?`, []any{sessions}},
			{`DELETE FROM conversations WHERE id IN ?`, []any{convs}},
			{`DELETE FROM credit_lot_allocations WHERE ledger_entry_id IN (SELECT id FROM credit_ledger WHERE user_id IN ? AND created_at >= ?)`, []any{clients, since}},
			{`DELETE FROM credit_lots WHERE user_id IN ? AND created_at >= ?`, []any{clients, since}},
			{`DELETE FROM credit_ledger WHERE user_id IN ? AND created_at >= ?`, []any{clients, since}},
			{`DELETE FROM notifications WHERE user_id IN ? AND created_at >= ?`, []any{clients, since}},
		}
		if len(holds) > 0 {
			steps = append(steps, struct {
				query string
				args  []any
			}{`DELETE FROM psychic_earnings WHERE source_id IN ?`, []any{holds}})
		}
		for _, w := range wallets {
			steps = append(steps, struct {
				query string
				args  []any
			}{`UPDATE wallets SET balance = ?, held = ?, free_minutes = ?, updated_at = ? WHERE user_id = ?`,
				[]any{w.Balance, w.Held, w.FreeMinutes, w.UpdatedAt, w.UserID}})
		}
		if len(settled) > 0 {
			steps = append(steps, struct {
				query string
				args  []any
			}{`UPDATE psychics SET readings_count = GREATEST(readings_count - 1, 0) WHERE user_id IN ?`, []any{settled}})
		}
		for _, st := range steps {
			if err := tx.Exec(st.query, st.args...).Error; err != nil {
				return fmt.Errorf("%.70s: %w", strings.Join(strings.Fields(st.query), " "), err)
			}
		}
		return nil
	})
	if err != nil {
		t.Errorf("cleanup: %v", err)
		return
	}
	// Nothing of the scenario may remain.
	if n := countRows(db, `SELECT (SELECT count(*) FROM chat_sessions WHERE id IN ?) + (SELECT count(*) FROM chat_session_holds WHERE session_id IN ?)
		+ (SELECT count(*) FROM chat_messages WHERE session_id IN ?) + (SELECT count(*) FROM credit_ledger WHERE user_id IN ? AND created_at >= ?)`,
		sessions, sessions, sessions, clients, since); n != 0 {
		t.Errorf("cleanup left %d rows behind", n)
	}
}
