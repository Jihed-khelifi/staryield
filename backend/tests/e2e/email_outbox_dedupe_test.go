package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// The outbox's dedupe upsert targets a partial unique index
// (dedupe_key is not empty). Postgres can only match it when the conflict target's
// predicate is a literal: as a bind parameter it works in custom plans and
// fails with 42P10 once a cached statement switches to a generic plan
// (after a few executions). force_generic_plan reproduces that at once.
func TestEmailOutboxDedupeSurvivesGenericPlans(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	now := time.Now().UTC()
	row := func(key string) domain.EmailOutbox {
		return domain.EmailOutbox{
			ID: uuid.New(), Category: domain.EmailAccount, Template: "e2e-dedupe-test", Locale: "en",
			ToHash: strings.Repeat("a", 64), ToSealed: "sealed", PayloadSealed: "sealed", DedupeKey: key,
			Status: domain.EmailQueued, NextAttemptAt: now,
			Timestamps: domain.Timestamps{CreatedAt: now, UpdatedAt: now},
		}
	}
	key := "e2e:dedupe:" + uuid.NewString()

	err := e.app.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL plan_cache_mode = force_generic_plan").Error; err != nil {
			return err
		}
		repo := repository.NewEmailRepository(tx)
		for i := 0; i < 8; i++ {
			// The same key every time: the first insert wins, the rest are no-ops.
			if err := repo.Enqueue(ctx, []domain.EmailOutbox{row(key), row("")}); err != nil {
				return fmt.Errorf("enqueue %d: %w", i, err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("dedupe upsert under generic plans: %v", err)
	}

	var deduped, blank int64
	e.app.DB.Model(&domain.EmailOutbox{}).Where("dedupe_key = ?", key).Count(&deduped)
	e.app.DB.Model(&domain.EmailOutbox{}).Where("dedupe_key = '' AND template = 'e2e-dedupe-test' AND created_at = ?", now).Count(&blank)
	if deduped != 1 {
		t.Fatalf("rows with the dedupe key = %d, want 1", deduped)
	}
	if blank != 8 {
		t.Fatalf("rows without a dedupe key = %d, want 8 (never deduplicated)", blank)
	}
}
