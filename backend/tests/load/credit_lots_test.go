//go:build load

package load

import (
	"context"
	"fmt"
	"sort"
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
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/seed"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
)

// revisions 5.0: credit lots.
//
// TestLoadBilledMinutes drives the real billing loop (ChatService.BillDue, the
// code the worker runs every tick) over thousands of active sessions of
// distinct clients at once: every minute locks a wallet, consumes credit lots
// (clients hold two paid packs and a bonus lot, so a minute usually touches
// one lot and sometimes spans two), appends the ledger entry and the lot
// allocations, saves the wallet and the session. The scenario creates its own
// synthetic clients and sessions (advisors are the large seed's, only
// referenced) and removes every row it wrote when it ends.
//
// Variables (all optional): LOAD_BILL_CLIENTS (default 2000, at most the number
// of advisors without an open session), LOAD_BILL_ROUNDS (default 5 billed
// minutes per client), LOAD_BILL_WORKERS (default 16 concurrent billing
// workers, like several API instances), LOAD_REDIS_URL, LOAD_BILL_SKIP_LOT_CHECK=1
// (to measure a checkout from before credit lots, which leaves lots alone). It needs
// LOAD_DATABASE_DSN (a private copy of the large dataset) and a PostgreSQL
// superuser, because the cleanup disables the ledger's append-only trigger for
// its own session (session_replication_role = replica).
func TestLoadBilledMinutes(t *testing.T) {
	cfg := LoadConfig()
	clients, rounds, workers := envInt("LOAD_BILL_CLIENTS", 2000), envInt("LOAD_BILL_ROUNDS", 5), envInt("LOAD_BILL_WORKERS", 16)
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("database: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	// The in-process application (workers off: this test drives billing).
	acfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	acfg.App.Env = config.EnvTest
	acfg.Database.DSN = cfg.DSN
	acfg.Redis.URL = env("LOAD_REDIS_URL", "redis://127.0.0.1:6379/13")
	acfg.Redis.KeyPrefix = "loadbill:" + uuid.NewString()[:8] + ":"
	acfg.Worker.Enabled = false
	acfg.Log.Level = "error"
	acfg.Database.MaxOpenConns = max(workers*2, 20)
	if err := acfg.Validate(); err != nil {
		t.Fatalf("config: %v", err)
	}
	a, err := app.New(ctx, acfg, zap.NewNop(), app.Overrides{Clock: clock.Real{}, Mailer: &mail.Recorder{}, Tracker: &analytics.Recorder{},
		Storage: storage.NewMemory("https://assets.test"), AdvisorStorage: storage.NewMemory("https://advisors.test"), ChatStorage: storage.NewMemory("")})
	if err != nil {
		t.Fatalf("app: %v", err)
	}
	defer a.Shutdown(context.Background())

	// Count the statements the billing path issues.
	var statements atomic.Int64
	count := func(*gorm.DB) { statements.Add(1) }
	for name, reg := range map[string]func(string, func(*gorm.DB)) error{
		"query": func(n string, f func(*gorm.DB)) error {
			return a.DB.Callback().Query().After("gorm:query").Register(n, f)
		},
		"row": func(n string, f func(*gorm.DB)) error { return a.DB.Callback().Row().After("gorm:row").Register(n, f) },
		"create": func(n string, f func(*gorm.DB)) error {
			return a.DB.Callback().Create().After("gorm:create").Register(n, f)
		},
		"update": func(n string, f func(*gorm.DB)) error {
			return a.DB.Callback().Update().After("gorm:update").Register(n, f)
		},
		"delete": func(n string, f func(*gorm.DB)) error {
			return a.DB.Callback().Delete().After("gorm:delete").Register(n, f)
		},
		"raw": func(n string, f func(*gorm.DB)) error { return a.DB.Callback().Raw().After("gorm:raw").Register(n, f) },
	} {
		if err := reg("load:count:"+name, count); err != nil {
			t.Fatal(err)
		}
	}

	run := uuid.NewString()[:8]
	emailPattern := "load-lotbill-" + run + "-%@" + seed.LoadEmailDomain
	cleanup := func() {
		// The ledger and the allocations are append-only: a private load database
		// is cleaned with triggers (and foreign keys) off for this transaction only.
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("SET LOCAL session_replication_role = replica").Error; err != nil {
				return err
			}
			for _, q := range []string{
				"DELETE FROM credit_lot_allocations WHERE user_id IN (SELECT id FROM users WHERE email LIKE ?)",
				"DELETE FROM credit_lots WHERE user_id IN (SELECT id FROM users WHERE email LIKE ?)",
				"DELETE FROM credit_ledger WHERE user_id IN (SELECT id FROM users WHERE email LIKE ?)",
				"DELETE FROM chat_sessions WHERE client_id IN (SELECT id FROM users WHERE email LIKE ?)",
				"DELETE FROM chat_messages WHERE conversation_id IN (SELECT id FROM conversations WHERE client_id IN (SELECT id FROM users WHERE email LIKE ?))",
				"DELETE FROM conversations WHERE client_id IN (SELECT id FROM users WHERE email LIKE ?)",
				"DELETE FROM notifications WHERE user_id IN (SELECT id FROM users WHERE email LIKE ?)",
				"DELETE FROM wallets WHERE user_id IN (SELECT id FROM users WHERE email LIKE ?)",
				"DELETE FROM user_profiles WHERE user_id IN (SELECT id FROM users WHERE email LIKE ?)",
				"DELETE FROM users WHERE email LIKE ?",
			} {
				if err := tx.Exec(q, emailPattern).Error; err != nil {
					return fmt.Errorf("%s: %w", q, err)
				}
			}
			return nil
		})
		if err != nil {
			t.Errorf("cleanup of the synthetic clients failed (delete users like %q by hand): %v", emailPattern, err)
		}
	}
	t.Cleanup(cleanup)

	// Advisors without an open session (one open session per advisor).
	var advisors []uuid.UUID
	db.Raw(`SELECT p.user_id FROM psychics p JOIN users u ON u.id = p.user_id
		WHERE u.status = 'active' AND NOT EXISTS (SELECT 1 FROM chat_sessions s WHERE s.psychic_id = p.user_id
		  AND s.status IN ('requested','accepted','active','disconnected'))
		ORDER BY p.user_id LIMIT ?`, clients).Scan(&advisors)
	if len(advisors) < clients {
		t.Logf("only %d advisors are free: using %d clients", len(advisors), len(advisors))
		clients = len(advisors)
	}
	if clients == 0 {
		t.Skip("no free synthetic advisors (run cmd/seed -profile=large)")
	}

	// Synthetic clients: 600 + 250 credit packs and 150 bonus credits (1000 credits, 20 per minute).
	now := time.Now().UTC()
	const rate = 20
	userRows := make([]domain.User, 0, clients)
	profiles := make([]domain.Profile, 0, clients)
	wallets := make([]domain.Wallet, 0, clients)
	convs := make([]domain.Conversation, 0, clients)
	sessions := make([]domain.Session, 0, clients)
	ids := make([]uuid.UUID, 0, clients)
	started := now.Add(-time.Minute - 2*time.Second)
	due := now.Add(-time.Second)
	for i := 0; i < clients; i++ {
		u := domain.User{ID: domain.NewID(), Email: fmt.Sprintf("load-lotbill-%s-%05d@%s", run, i, seed.LoadEmailDomain), PasswordHash: "x",
			Role: domain.RoleClient, Status: domain.UserActive, DisplayName: "Load billing", Locale: "en", TermsAcceptedAt: now, CreatedAt: now, UpdatedAt: now}
		userRows, ids = append(userRows, u), append(ids, u.ID)
		profiles = append(profiles, domain.Profile{UserID: u.ID, Mood: "Calm", Theme: "Warm Beige", UpdatedAt: now})
		wallets = append(wallets, domain.Wallet{UserID: u.ID, Balance: 1000, UpdatedAt: now})
		conv := domain.Conversation{ID: domain.NewID(), ClientID: u.ID, PsychicID: advisors[i], LastActivityAt: now, CreatedAt: now, UpdatedAt: now}
		convs = append(convs, conv)
		s := domain.Session{ID: domain.NewID(), ConversationID: conv.ID, ClientID: u.ID, PsychicID: advisors[i], Status: domain.SessionActive,
			RatePerMinute: rate, ListRatePerMinute: rate, RequestKey: uuid.NewString(), RequestedAt: started, ExpiresAt: started.Add(time.Hour),
			StartedAt: &started, LastResumedAt: &started, NextBillingAt: &due, BilledMinutes: 0, CreatedAt: now, UpdatedAt: now}
		sessions = append(sessions, s)
	}
	for _, rows := range []any{&userRows, &profiles, &wallets, &convs, &sessions} {
		if err := db.Select("*").CreateInBatches(rows, 500).Error; err != nil {
			t.Fatalf("seed rows: %v", err)
		}
	}
	// Lots: no ledger entry exists for them, so they are legacy-style rows (origin legacy_backfill).
	for i := 0; i < clients; i += 500 {
		end := min(i+500, clients)
		for _, spec := range []struct {
			source          string
			priority        int
			credits, cents  int64
			ageSecondsAgo   int
			referenceTypeFn bool
		}{{"purchase", 0, 600, 1999, 7200, true}, {"purchase", 0, 250, 999, 3600, true}, {"bonus", 2, 150, 0, 10800, false}} {
			for _, id := range ids[i:end] {
				refType, refID := "", any(nil)
				if spec.referenceTypeFn {
					refType, refID = "credit_purchase", uuid.New()
				}
				if err := db.Exec(`INSERT INTO credit_lots (id, user_id, source, priority, origin, reference_type, reference_id, credits_total,
					credits_remaining, paid_cents, currency, value_remaining_millicents, created_at, updated_at)
					VALUES (?, ?, ?, ?, 'legacy_backfill', ?, ?, ?, ?, ?, 'USD', ?, ?, ?)`, domain.NewID(), id, spec.source, spec.priority, refType, refID,
					spec.credits, spec.credits, spec.cents, spec.cents*1000, now.Add(-time.Duration(spec.ageSecondsAgo)*time.Second), now).Error; err != nil {
					t.Fatalf("seed lots: %v", err)
				}
			}
		}
	}
	t.Logf("billing %d clients × %d minutes with %d workers", clients, rounds, workers)

	var (
		latMu     sync.Mutex
		latencies []time.Duration
		failed    atomic.Int64
		billed    atomic.Int64
		samples   []string
	)
	statements.Store(0)
	began := time.Now()
	for round := 0; round < rounds; round++ {
		if round > 0 { // make the next minute of every session due again
			if err := db.Exec(`UPDATE chat_sessions SET next_billing_at = now() - interval '1 second' WHERE id IN ?`, sessionIDs(sessions)).Error; err != nil {
				t.Fatal(err)
			}
		}
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					t0 := time.Now()
					n, err := a.Services.Chat.BillDue(ctx, 1) // one billed minute per call: its latency is the minute's
					d := time.Since(t0)
					if err != nil {
						failed.Add(1)
						latMu.Lock()
						if len(samples) < 5 {
							samples = append(samples, err.Error())
						}
						latMu.Unlock()
						return
					}
					if n == 0 {
						return
					}
					billed.Add(int64(n))
					latMu.Lock()
					latencies = append(latencies, d)
					latMu.Unlock()
				}
			}()
		}
		wg.Wait()
	}
	elapsed := time.Since(began)
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	// Verify: every wallet = Σ lots = 1000 − 20 × rounds and the ledger holds one entry per billed minute.
	want := int64(clients * rounds)
	var wrong, mismatched int64
	db.Raw(`SELECT count(*) FROM wallets w WHERE w.user_id IN ? AND w.balance <> ?`, ids, 1000-int64(rate*rounds)).Scan(&wrong)
	db.Raw(`SELECT count(*) FROM wallets w WHERE w.user_id IN ? AND w.balance <> COALESCE((SELECT SUM(credits_remaining) FROM credit_lots l WHERE l.user_id = w.user_id), 0)`, ids).Scan(&mismatched)
	var entries int64
	db.Raw(`SELECT count(*) FROM credit_ledger WHERE user_id IN ? AND entry_type = 'chat_charge'`, ids).Scan(&entries)
	if env("LOAD_BILL_SKIP_LOT_CHECK", "") == "1" { // measuring code from before credit lots: it leaves the lots alone
		mismatched = 0
	}
	if billed.Load() != want || entries != want || wrong != 0 || mismatched != 0 || failed.Load() != 0 {
		t.Errorf("billed %d of %d, ledger entries %d, wallets with a wrong balance %d, lot mismatches %d, failures %d %v",
			billed.Load(), want, entries, wrong, mismatched, failed.Load(), samples)
	}

	res := Result{Name: fmt.Sprintf("billed minutes (%d clients, %d workers)", clients, workers), Rate: int(float64(billed.Load()) / max(elapsed.Seconds(), 1)),
		Duration: elapsed, Sent: billed.Load() + failed.Load(), OK: billed.Load(), Errors: failed.Load(), Status: map[int]int64{}, ErrorSamples: samples,
		SLO: SLO{P95: 200 * time.Millisecond, P99: 500 * time.Millisecond, MaxErrRate: 0.001}}
	if len(latencies) > 0 {
		res.P50, res.P90 = percentile(latencies, 0.50), percentile(latencies, 0.90)
		res.P95, res.P99, res.Max = percentile(latencies, 0.95), percentile(latencies, 0.99), latencies[len(latencies)-1]
		res.Throughput = float64(billed.Load()) / elapsed.Seconds()
	}
	res.Check()
	t.Logf("statements per billed minute: %.2f (%d statements over %d minutes)", float64(statements.Load())/float64(max(billed.Load(), 1)), statements.Load(), billed.Load())
	Report(t, cfg, "billed-minutes", []Result{res})
}

func sessionIDs(sessions []domain.Session) []uuid.UUID {
	out := make([]uuid.UUID, len(sessions))
	for i, s := range sessions {
		out[i] = s.ID
	}
	return out
}
