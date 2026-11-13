package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 5.0: 18+ age gate (§24.6).
//
// The status lives on users (adult_confirmed_at, age_restricted_at) and rides
// in the cached authorization state (statusEntry.Age), so the middleware
// answers without a query. Every write below invalidates that cache.

// UnderMinimumAge reports whether s is a well-formed, plausible birth date
// (not in the future, 1900 or later) that is younger than minimumAge. A
// malformed or impossible date is a validation error, not a restriction.
func UnderMinimumAge(s string, now time.Time, minimumAge int) bool {
	s = strings.TrimSpace(s)
	if s == "" || minimumAge <= 0 {
		return false
	}
	d, err := time.Parse("2006-01-02", s)
	if err != nil || d.Year() < 1900 || d.After(now) {
		return false
	}
	return d.AddDate(minimumAge, 0, 0).After(now)
}

// ageGuard applies the birth-date rules of the age gate to profile writes.
type ageGuard struct {
	users      repository.UserRepository
	clock      clock.Clock
	minimumAge int
	// invalidate drops the cached authorization state (nil in unit tests).
	invalidate func(ctx context.Context, userID uuid.UUID)
}

func (g ageGuard) drop(ctx context.Context, userID uuid.UUID) {
	if g.invalidate != nil {
		g.invalidate(ctx, userID)
	}
}

// beforeBirthDateWrite runs before a birth date from a profile or settings
// patch is validated. It returns skip=true when the field must be ignored
// (a restricted account re-sending the date already on file), a 403
// age_restricted when a restricted account tries to change it, and records
// the restriction when the date is under the minimum age. That write is
// deliberately separate from the (validation-failing) save: the 422 refusal
// must still persist the restriction, and no transaction rolls it back.
func (g ageGuard) beforeBirthDateWrite(ctx context.Context, userID uuid.UUID, raw *string) (skip bool, err error) {
	if raw == nil {
		return false, nil
	}
	u, err := g.users.GetByID(ctx, userID)
	if err != nil {
		return false, mapRepoErr(err, "User")
	}
	if u.Role != domain.RoleClient {
		return false, nil // vetted accounts are always adult and never restricted
	}
	if u.AgeRestrictedAt != nil {
		on := ""
		if p, perr := g.users.GetProfile(ctx, userID); perr == nil && p.BirthDate != nil {
			on = p.BirthDate.Format("2006-01-02")
		}
		if strings.TrimSpace(*raw) == on {
			return true, nil // unchanged: the settings form re-sends it
		}
		return false, apperror.AgeRestricted()
	}
	if UnderMinimumAge(*raw, g.clock.Now(), g.minimumAge) {
		now := g.clock.Now()
		if err := g.users.Updates(ctx, userID, map[string]any{"age_restricted_at": now}); err != nil {
			return false, mapRepoErr(err, "User")
		}
		g.drop(ctx, userID)
		logAgeRestricted(ctx, userID, "under_age_birth_date")
	}
	return false, nil
}

func logAgeRestricted(ctx context.Context, userID uuid.UUID, reason string) {
	logger.FromContext(ctx).Info("age gate: account restricted", zap.String("user_id", userID.String()), zap.String("reason", reason))
}
