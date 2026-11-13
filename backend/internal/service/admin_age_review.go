package service

import (
	"context"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 5.0: 18+ age gate (§24.6). An age review is the only way out of
// "restricted": "clear" removes the restriction and the confirmation, so the
// person has to enter a date of birth again; "restrict" restricts by hand.

// Age review decisions.
const (
	AgeReviewClear    = "clear"
	AgeReviewRestrict = "restrict"
)

// adultAge is the age the admin review uses to drop an under-age date of
// birth when clearing (the legal floor; production refuses a lower minimum).
const adultAge = 18

// AgeReviewInput is the body of POST /admin/users/:id/age-review.
type AgeReviewInput struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

// AgeReview applies an administrator's age decision to a client account and
// audits it as user.age_review. The caller has passed step-up.
func (s *AdminService) AgeReview(ctx context.Context, id uuid.UUID, in AgeReviewInput) (*AdminUserView, error) {
	fields := map[string]string{}
	if in.Decision != AgeReviewClear && in.Decision != AgeReviewRestrict {
		fields["decision"] = "The decision must be clear or restrict."
	}
	reason := CleanLine(in.Reason)
	if n := utf8.RuneCountInString(reason); n < 5 || n > 500 {
		fields["reason"] = "A reason between 5 and 500 characters is required."
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	before, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "User")
	}
	if before.Role != domain.RoleClient {
		return nil, apperror.Validation(map[string]string{"decision": "Age reviews apply to client accounts only."})
	}
	now := s.clock.Now()
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if in.Decision == AgeReviewRestrict {
			return s.users.Updates(ctx, id, map[string]any{"age_restricted_at": now})
		}
		if err := s.users.Updates(ctx, id, map[string]any{"age_restricted_at": nil, "adult_confirmed_at": nil}); err != nil {
			return err
		}
		// An under-age date of birth on file would only restrict the person
		// again: drop it so the form starts empty.
		if p, perr := s.users.GetProfile(ctx, id); perr == nil && p.BirthDate != nil &&
			UnderMinimumAge(p.BirthDate.Format("2006-01-02"), now, adultAge) {
			return s.users.UpdateProfile(ctx, id, map[string]any{"birth_date": nil})
		}
		return nil
	})
	if err != nil {
		return nil, mapRepoErr(err, "User")
	}
	s.auth.InvalidateStatus(ctx, id)
	if in.Decision == AgeReviewRestrict {
		// No live conversation continues for a restricted account, and open
		// tabs re-read their account (auth.revoked, §22.1).
		s.chat.EndAllForUser(ctx, id)
		warnErr(ctx, "admin: realtime revoke", s.events.Revoke(ctx, id))
	}
	s.record(ctx, "user.age_review", "user", id.String(), map[string]any{
		"decision": in.Decision, "reason": reason, "before": string(before.AgeStatus())})
	return s.User(ctx, id)
}
