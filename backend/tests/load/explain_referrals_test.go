//go:build load

package load

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// Growth: the referral loop (migration 0036). The hot queries are the
// inviter's counters (GET /me/referrals, one aggregate per request), the
// sign-up checks (code lookup, burst count, shared network, browser link),
// the qualification reads of a friend (their referral, a qualifying reading,
// the inviter's rewarded count), the grant job's claim, the sweep and the
// admin console's list. The admin summary aggregates the whole table by
// design (low traffic: the console polls it only while launched rewards are
// being added); it reads its covering index index-only once the table is
// vacuumed, and is an allowed sequential scan right after bulk writes.

// referralSample holds live ids for the checks (random ones when a table is empty).
type referralSample struct {
	inviter, referee uuid.UUID
	code             string
	cursor           repository.Cursor
	ipHash           string
	anonymousID      string
}

func pickReferralSample(db *gorm.DB) referralSample {
	s := referralSample{inviter: uuid.New(), referee: uuid.New(), code: "ABCDEFGH", ipHash: "00000000000000000000000000000000",
		anonymousID: "load-anonymous-id", cursor: repository.Cursor{At: time.Now(), ID: uuid.NewString()}}
	var busy struct{ ReferrerID uuid.UUID }
	if db.Raw(`SELECT referrer_id FROM referrals GROUP BY referrer_id ORDER BY count(*) DESC LIMIT 1`).Scan(&busy).Error == nil && busy.ReferrerID != uuid.Nil {
		s.inviter = busy.ReferrerID
	}
	var code string
	if db.Raw(`SELECT code FROM referral_codes WHERE user_id = ?`, s.inviter).Row().Scan(&code) == nil && code != "" {
		s.code = code
	}
	var referee uuid.UUID
	if db.Raw(`SELECT referee_id FROM referrals WHERE referee_id IS NOT NULL ORDER BY created_at DESC LIMIT 1`).Row().Scan(&referee) == nil {
		s.referee = referee
	}
	var at time.Time
	var id string
	if db.Raw(`SELECT created_at, id::text FROM referrals ORDER BY created_at DESC, id DESC OFFSET 500 LIMIT 1`).Row().Scan(&at, &id) == nil {
		s.cursor = repository.Cursor{At: at, ID: id}
	}
	var hash string
	if db.Raw(`SELECT ip_hash FROM user_sessions WHERE user_id = ? AND ip_hash <> '' LIMIT 1`, s.inviter).Row().Scan(&hash) == nil && hash != "" {
		s.ipHash = hash
	}
	return s
}

// referralPlan is one repository call, the index its main table must be read
// through ("" = only "no sequential scan") and its budget.
type referralPlan struct {
	name   string
	build  func(ctx context.Context, r repository.ReferralRepository)
	table  string
	index  string
	budget time.Duration
}

func referralPlans(s referralSample) []referralPlan {
	flagged := true
	now := time.Now()
	return []referralPlan{
		{"referrals: inviter counters (GET /me/referrals)", func(ctx context.Context, r repository.ReferralRepository) {
			_, _ = r.Stats(ctx, s.inviter)
		}, "referrals", "referrals_referrer_status_idx", 10 * time.Millisecond},
		{"referrals: code of a user", func(ctx context.Context, r repository.ReferralRepository) {
			_, _ = r.CodeOf(ctx, s.inviter)
		}, "referral_codes", "referral_codes_pkey", 5 * time.Millisecond},
		{"referrals: inviter of a code (sign-up)", func(ctx context.Context, r repository.ReferralRepository) {
			_, _ = r.InviterByCode(ctx, s.code)
		}, "referral_codes", "referral_codes_code_key", 5 * time.Millisecond},
		{"referrals: sign-ups in 24 h (burst)", func(ctx context.Context, r repository.ReferralRepository) {
			_, _ = r.JoinedSince(ctx, s.inviter, now.Add(-24*time.Hour))
		}, "referrals", "referrals_referrer_joined_idx", 10 * time.Millisecond},
		{"referrals: inviter's network (sign-up)", func(ctx context.Context, r repository.ReferralRepository) {
			_, _ = r.SharesSessionNetwork(ctx, s.inviter, s.ipHash, now.Add(-30*24*time.Hour))
		}, "user_sessions", "", 10 * time.Millisecond},
		{"referrals: browser of the inviter (sign-up)", func(ctx context.Context, r repository.ReferralRepository) {
			_, _ = r.VisitorOwner(ctx, s.anonymousID)
		}, "web_visitors", "", 5 * time.Millisecond},
		{"referrals: referral of a friend", func(ctx context.Context, r repository.ReferralRepository) {
			_, _ = r.ByReferee(ctx, s.referee)
		}, "referrals", "referrals_referee_key", 5 * time.Millisecond},
		{"referrals: qualifying reading of a friend", func(ctx context.Context, r repository.ReferralRepository) {
			_, _ = r.QualifyingSession(ctx, s.referee)
		}, "chat_sessions", "", 10 * time.Millisecond},
		// growth (wiring): the lifecycle onboarding checklist and admin funnel.
		{"referrals: invited anyone (onboarding checklist)", func(ctx context.Context, r repository.ReferralRepository) {
			_, _ = r.HasInvited(ctx, s.inviter)
		}, "referrals", "referrals_referrer_status_idx", 5 * time.Millisecond},
		// One index-only probe of referrals_referrer_status_idx per account of
		// the cohort, so the cost follows the sign-up rate, never the size of
		// referrals (a hash of every referral took 374 ms at 200k users). The
		// populated check's cohort is about 47,000 sign-ups in 30 days (12x the
		// load dataset's) on rows not yet all-visible, about 300 ms; the load
		// dataset's real cohort (3,857) takes 12 ms warm. Admin-only and cached.
		{"referrals: inviters among a 30-day sign-up cohort (lifecycle funnel)", func(ctx context.Context, r repository.ReferralRepository) {
			_, _ = r.InvitersAmong(ctx, now.AddDate(0, 0, -30), now)
		}, "referrals", "referrals_referrer_status_idx", 500 * time.Millisecond},
		{"referrals: rewarded friends (cap)", func(ctx context.Context, r repository.ReferralRepository) {
			_, _ = r.Rewarded(ctx, s.inviter)
		}, "referrals", "referrals_referrer_status_idx", 10 * time.Millisecond},
		{"referrals: grant claim (SKIP LOCKED)", func(ctx context.Context, r repository.ReferralRepository) {
			_, _ = r.ClaimGrantable(ctx, 200)
		}, "referrals", "referrals_grant_idx", 25 * time.Millisecond},
		{"referrals: rewards ready to grant", func(ctx context.Context, r repository.ReferralRepository) {
			_, _, _ = r.PendingGrants(ctx)
		}, "referrals", "", 50 * time.Millisecond}, // referrals_grant_idx or referrals_summary_idx
		{"referrals: sweep (missed hooks)", func(ctx context.Context, r repository.ReferralRepository) {
			_, _ = r.ReadyToQualify(ctx, 200)
		}, "referrals", "referrals_joined_idx", 200 * time.Millisecond},
		{"referrals: admin list (all)", func(ctx context.Context, r repository.ReferralRepository) {
			_, _ = r.List(ctx, repository.ReferralFilter{Limit: 21})
		}, "referrals", "referrals_recent_idx", 10 * time.Millisecond},
		{"referrals: admin list (page 25)", func(ctx context.Context, r repository.ReferralRepository) {
			_, _ = r.List(ctx, repository.ReferralFilter{Limit: 21, Cursor: &s.cursor})
		}, "referrals", "referrals_recent_idx", 10 * time.Millisecond},
		{"referrals: admin list (qualified)", func(ctx context.Context, r repository.ReferralRepository) {
			_, _ = r.List(ctx, repository.ReferralFilter{Status: domain.ReferralQualified, Limit: 21})
		}, "referrals", "referrals_status_recent_idx", 10 * time.Millisecond},
		{"referrals: admin list (qualified, page 25)", func(ctx context.Context, r repository.ReferralRepository) {
			_, _ = r.List(ctx, repository.ReferralFilter{Status: domain.ReferralQualified, Limit: 21, Cursor: &s.cursor})
		}, "referrals", "referrals_status_recent_idx", 10 * time.Millisecond},
		{"referrals: admin list (held for review)", func(ctx context.Context, r repository.ReferralRepository) {
			_, _ = r.List(ctx, repository.ReferralFilter{Flagged: &flagged, Limit: 21})
		}, "referrals", "referrals_flagged_recent_idx", 10 * time.Millisecond},
		{"referrals: admin summary totals", func(ctx context.Context, r repository.ReferralRepository) {
			_, _ = r.Totals(ctx)
		}, "referrals", "referrals_summary_idx", 150 * time.Millisecond},
		{"referrals: admin top inviters", func(ctx context.Context, r repository.ReferralRepository) {
			_, _ = r.TopInviters(ctx, 10)
		}, "referrals", "", 200 * time.Millisecond},
	}
}

// referralChecks adds the referral queries to TestExplainHotQueries (on the
// dataset as it is; TestExplainReferralsPopulated gives them volume).
func referralChecks(db *gorm.DB) []queryCheck {
	var ready bool
	db.Raw(`SELECT to_regclass('referrals') IS NOT NULL`).Scan(&ready)
	if !ready {
		return nil // migration 0036 is not applied to the load database
	}
	var out []queryCheck
	for _, p := range referralPlans(pickReferralSample(db)) {
		out = append(out, queryCheck{p.name, p.budget, func(ctx context.Context, d *gorm.DB) {
			p.build(ctx, repository.NewReferralRepository(d))
		}})
	}
	return out
}

// The admin summary is one pass over the referrals by design (it reads the
// covering index referrals_summary_idx when the visibility map allows it, and
// the planner may prefer the heap on a freshly written table); the top
// inviters aggregate the rewarded rows. Both are admin-only and low traffic.
func init() {
	seqScanAllowed["referrals: admin summary totals"] = true
	seqScanAllowed["referrals: admin top inviters"] = true
}

// TestExplainReferralsPopulated gives the referral tables production-like
// volume inside a transaction that is rolled back (120,000 synthetic clients,
// 3,000 inviters with a code, about 117,000 invited friends with every status,
// a skewed inviter with about 600 friends, one device per inviter and a
// settled reading for a third of the friends), then asserts each hot query's
// plan reads its index and stays within its budget. Nothing is committed, so
// it can run against any migrated database (LOAD_DATABASE_DSN).
func TestExplainReferralsPopulated(t *testing.T) {
	cfg := LoadConfig()
	live, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("database %s: %v", cfg.DSN, err)
	}
	var ready bool
	live.Raw(`SELECT to_regclass('referrals') IS NOT NULL`).Scan(&ready)
	if !ready {
		t.Skip("migration 0036 is not applied to the load database")
	}
	var advisor uuid.UUID
	if live.Raw(`SELECT user_id FROM psychics LIMIT 1`).Row().Scan(&advisor) != nil {
		t.Skip("the database has no advisor to attach readings to")
	}
	dry, rec := newDryDB(t, cfg.DSN)
	ctx := context.Background()

	tx := live.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback() // nothing of this test is ever committed

	steps := []struct {
		name string
		sql  string
		args []any
	}{
		{"clients", `INSERT INTO users (id, email, password_hash, role, terms_accepted_at, email_verified_at, created_at, updated_at)
			SELECT gen_random_uuid(), 'explain-referral-' || lpad(g::text, 6, '0') || '@load.staryield.test', 'x', 'client', now(),
				CASE WHEN g % 3 = 0 THEN now() END, now() - (g || ' minutes')::interval, now()
			FROM generate_series(1, 120000) g`, nil},
		{"numbering", `CREATE TEMP TABLE explain_referral_users ON COMMIT DROP AS
			SELECT id, row_number() OVER (ORDER BY email) AS n FROM users WHERE email LIKE 'explain-referral-%'`, nil},
		{"codes", `INSERT INTO referral_codes (user_id, code)
			SELECT id, translate(lpad(to_hex(n), 8, '0'), '0123456789abcdef', 'ABCDEFGHJKMNPQRS') FROM explain_referral_users WHERE n <= 3000`, nil},
		{"devices", `INSERT INTO user_sessions (id, user_id, ip_hash, last_seen_at, expires_at)
			SELECT gen_random_uuid(), id, substr(md5(n::text), 1, 32), now() - interval '1 day', now() + interval '30 days'
			FROM explain_referral_users WHERE n <= 3000`, nil},
		// Friends: one inviter in 190 is the busiest one; statuses by n % 10
		// (4 joined, 2 qualified, 3 granted, 1 rejected), one in 23 flagged.
		{"friends", `INSERT INTO referrals (id, referrer_id, referee_id, code, status, reward_minutes, flagged, flags, reject_reason,
				signup_ip_hash, joined_at, qualified_at, granted_at, created_at, updated_at)
			SELECT gen_random_uuid(), i.id, f.id, c.code,
				(ARRAY['joined','joined','joined','joined','qualified','qualified','granted','granted','granted','rejected'])[1 + f.n % 10],
				CASE WHEN f.n % 10 BETWEEN 4 AND 8 THEN 5 ELSE 0 END,
				f.n % 23 = 0, CASE WHEN f.n % 23 = 0 THEN '["same_network"]'::jsonb ELSE '[]'::jsonb END,
				CASE WHEN f.n % 10 = 9 THEN 'cap_reached' ELSE '' END,
				substr(md5(f.n::text), 1, 32), now() - (f.n || ' minutes')::interval,
				CASE WHEN f.n % 10 BETWEEN 4 AND 8 THEN now() - (f.n || ' seconds')::interval END,
				CASE WHEN f.n % 10 BETWEEN 6 AND 8 THEN now() - (f.n || ' seconds')::interval END,
				now() - (f.n || ' minutes')::interval, now()
			FROM explain_referral_users f
			JOIN explain_referral_users i ON i.n = CASE WHEN f.n % 190 = 0 THEN 1 ELSE 1 + (f.n * 31) % 3000 END
			JOIN referral_codes c ON c.user_id = i.id
			WHERE f.n > 3000`, nil},
		{"conversations", `INSERT INTO conversations (id, client_id, psychic_id)
			SELECT gen_random_uuid(), id, ? FROM explain_referral_users WHERE n > 3000 AND n % 3 <> 1`, []any{advisor}},
		{"readings", `INSERT INTO chat_sessions (id, conversation_id, client_id, psychic_id, status, rate_per_minute, list_rate_per_minute,
				request_key, requested_at, expires_at, started_at, ended_at, settled_at, billed_minutes, free_minutes_used)
			SELECT gen_random_uuid(), c.id, c.client_id, c.psychic_id, 'settled', 30, 30, 'explain-' || c.id, now() - interval '2 days',
				now() - interval '2 days', now() - interval '2 days', now() - interval '2 days', now() - interval '2 days', 3, 2
			FROM conversations c JOIN explain_referral_users u ON u.id = c.client_id`, nil},
	}
	for _, st := range steps {
		if err := tx.Exec(st.sql, st.args...).Error; err != nil {
			t.Fatalf("%s: %v", st.name, err)
		}
	}
	for _, table := range []string{"users", "referral_codes", "referrals", "user_sessions", "conversations", "chat_sessions"} {
		tx.Exec("ANALYZE " + table)
	}
	var count int64
	tx.Raw(`SELECT count(*) FROM referrals`).Scan(&count)
	if count < 100000 {
		t.Fatalf("only %d referrals were generated", count)
	}

	s := pickReferralSampleTx(tx)
	for _, p := range referralPlans(s) {
		p.build(ctx, repository.NewReferralRepository(dry))
		sql := rec.take()
		if sql == "" {
			t.Fatalf("%s: no SQL captured", p.name)
		}
		plan := explainTx(t, tx, p.name, sql)
		_, label := usesIndexOn(plan, p.table, "")
		if p.index != "" && !seqScanAllowed[p.name] {
			if ok, _ := usesIndexOn(plan, p.table, p.index); !ok {
				t.Errorf("%s: %s is not read through %s: %s\n%s", p.name, p.table, p.index, label, sql)
			}
		}
		if !seqScanAllowed[p.name] {
			walk(plan.Plan, func(n planNode) {
				if n.NodeType == "Seq Scan" && (n.Relation == "referrals" || n.Relation == "users" || n.Relation == "chat_sessions") {
					t.Errorf("%s: sequential scan on %s: %s\n%s", p.name, n.Relation, label, sql)
				}
			})
		}
		if d := time.Duration(plan.ExecutionTime * float64(time.Millisecond)); d > p.budget {
			t.Errorf("%s: execution %s exceeds %s", p.name, d, p.budget)
		}
		t.Logf("%-48s %7.2f ms  %s", p.name, plan.ExecutionTime, label)
	}
}

// pickReferralSampleTx picks the sample inside the populated transaction:
// the busiest inviter, a friend with a reading and a cursor 500 rows deep.
func pickReferralSampleTx(tx *gorm.DB) referralSample {
	s := pickReferralSample(tx)
	var referee uuid.UUID
	if tx.Raw(`SELECT r.referee_id FROM referrals r JOIN chat_sessions c ON c.client_id = r.referee_id
		WHERE r.referrer_id = ? LIMIT 1`, s.inviter).Row().Scan(&referee) == nil {
		s.referee = referee
	}
	return s
}
