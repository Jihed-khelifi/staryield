package e2e

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/migrations"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/seed"
)

// isolatedSchema migrates a fresh schema (named here, never from input) and
// drops it after the test, so synthetic data never meets other tests.
func isolatedSchema(t *testing.T) *gorm.DB {
	t.Helper()
	base, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	schema := "seedtest_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	if err := base.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	sep := "?"
	if strings.Contains(testDSN, "?") {
		sep = "&"
	}
	db, err := gorm.Open(postgres.Open(testDSN+sep+"search_path="+schema), &gorm.Config{Logger: gormlogger.Discard,
		NowFunc: func() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		_ = base.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		if sqlDB, err := base.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if _, err := migrations.Up(context.Background(), db, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	return db
}

type seedTotals struct {
	Users, Psychics, Conversations, Sessions, Messages, Reviews, Ledger, Earnings int64
	LedgerSum, RatingSum                                                          int64
}

func totals(t *testing.T, db *gorm.DB) seedTotals {
	t.Helper()
	var s seedTotals
	db.Model(&domain.User{}).Where("email LIKE ?", "%@"+seed.LoadEmailDomain).Count(&s.Users)
	db.Model(&domain.Psychic{}).Count(&s.Psychics)
	db.Model(&domain.Conversation{}).Count(&s.Conversations)
	db.Model(&domain.Session{}).Count(&s.Sessions)
	db.Model(&domain.Message{}).Count(&s.Messages)
	db.Model(&domain.Review{}).Count(&s.Reviews)
	db.Model(&domain.LedgerEntry{}).Count(&s.Ledger)
	db.Model(&domain.Earning{}).Count(&s.Earnings)
	db.Model(&domain.LedgerEntry{}).Select("COALESCE(sum(amount), 0)").Scan(&s.LedgerSum)
	db.Model(&domain.Review{}).Select("COALESCE(sum(rating), 0)").Scan(&s.RatingSum)
	return s
}

func TestLargeSeedSmallScale(t *testing.T) {
	if !available {
		t.Skip("infrastructure unavailable")
	}
	ctx := context.Background()
	hasher := security.NewPasswordHasher(1024, 1, 1)
	opts := seed.LargeOptions{Users: 300, Psychics: 12, Conversations: 400, MessagesPerConversation: 6, Reviews: 150, LedgerRows: 1200,
		Seed: 7, BatchSize: 100}

	db := isolatedSchema(t)
	rep, err := seed.Large(ctx, db, hasher, opts, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	got := totals(t, db)
	if got.Users != 312 || got.Psychics != 12 || got.Conversations != 400 || got.Ledger != 1200 {
		t.Fatalf("population: %+v", got)
	}
	if got.Sessions < 400 || got.Messages == 0 || got.Reviews == 0 || got.Reviews > got.Sessions || got.Earnings == 0 {
		t.Fatalf("activity: %+v", got)
	}
	if rep.Counts["chat_messages"] != int(got.Messages) || rep.Counts["users"] != 312 {
		t.Fatalf("report: %+v", rep.Counts)
	}

	// Data is internally consistent: wallets match the last ledger balance,
	// advisor aggregates match their visible reviews, conversations point at
	// their latest message.
	var mismatched int64
	db.Model(&domain.Wallet{}).
		Joins("JOIN (SELECT DISTINCT ON (user_id) user_id, balance_after FROM credit_ledger ORDER BY user_id, created_at DESC, id DESC) l ON l.user_id = wallets.user_id").
		Where("wallets.balance <> l.balance_after").Count(&mismatched)
	if mismatched != 0 {
		t.Fatalf("%d wallets disagree with their ledger", mismatched)
	}
	db.Model(&domain.Psychic{}).Where("reviews_count <> (SELECT count(*) FROM psychic_reviews r WHERE r.psychic_id = psychics.user_id AND r.status = 'visible')").
		Count(&mismatched)
	if mismatched != 0 {
		t.Fatalf("%d advisors with a wrong review count", mismatched)
	}
	db.Model(&domain.Conversation{}).Where("last_message_id IS NOT NULL AND last_message_at <> (SELECT max(created_at) FROM chat_messages m WHERE m.conversation_id = conversations.id)").
		Count(&mismatched)
	if mismatched != 0 {
		t.Fatalf("%d conversations with a stale last message", mismatched)
	}
	// Synthetic accounts can sign in with the documented password.
	var u domain.User
	db.Where("email LIKE ?", "%@"+seed.LoadEmailDomain).First(&u)
	if _, err := hasher.Verify(seed.LoadPassword, u.PasswordHash); err != nil {
		t.Fatalf("load password: %v", err)
	}

	// Idempotent guard, and -force adds a separately tagged population.
	if _, err := seed.Large(ctx, db, hasher, opts, zap.NewNop()); !errors.Is(err, seed.ErrAlreadySeeded) {
		t.Fatalf("second run: %v", err)
	}
	small := opts
	small.Users, small.Psychics, small.Conversations, small.LedgerRows, small.Reviews, small.Force = 5, 2, 3, 5, 1, true
	rep2, err := seed.Large(ctx, db, hasher, small, zap.NewNop())
	if err != nil || rep2.Tag == rep.Tag {
		t.Fatalf("forced run: %v %+v", err, rep2)
	}

	// Deterministic: the same seed produces the same population elsewhere.
	other := isolatedSchema(t)
	if _, err := seed.Large(ctx, other, hasher, opts, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	if again := totals(t, other); again != got {
		t.Fatalf("not deterministic:\n first %+v\nsecond %+v", got, again)
	}

	// Validation.
	bad := opts
	bad.BatchSize = 0
	if _, err := seed.Large(ctx, other, hasher, bad, zap.NewNop()); err == nil {
		t.Fatal("invalid options accepted")
	}
}
