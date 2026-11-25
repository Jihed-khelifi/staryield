package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
)

// Steps of the client onboarding checklist, in display order.
const (
	OnboardingVerifyEmail  = "verify_email"
	OnboardingBirthDetails = "birth_details"
	OnboardingFirstReading = "first_reading"
	OnboardingInviteFriend = "invite_friend"
)

// OnboardingStep is one item of the checklist.
type OnboardingStep struct {
	Key  string `json:"key"`
	Done bool   `json:"done"`
}

// OnboardingView is GET /me/onboarding.
type OnboardingView struct {
	Steps     []OnboardingStep `json:"steps"`
	Completed int              `json:"completed"`
	Total     int              `json:"total"`
	Dismissed bool             `json:"dismissed"`
}

// Onboarding returns a client's checklist: the account facts in one query and
// the number of invited friends from the referral program (0 without it).
func (s *LifecycleService) Onboarding(ctx context.Context, userID uuid.UUID) (*OnboardingView, error) {
	facts, err := s.d.Repo.Onboarding(ctx, userID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	invited := 0
	s.mu.RLock()
	count := s.invitedFriends
	s.mu.RUnlock()
	if count != nil {
		n, err := count(ctx, userID)
		if err != nil {
			// The step stays open rather than failing the whole checklist.
			warnErr(ctx, "onboarding: count invited friends", err)
		} else {
			invited = n
		}
	}
	steps := []OnboardingStep{{OnboardingVerifyEmail, facts.Verified}, {OnboardingBirthDetails, facts.BirthDetails},
		{OnboardingFirstReading, facts.FirstReading}, {OnboardingInviteFriend, invited > 0}}
	out := &OnboardingView{Steps: steps, Total: len(steps), Dismissed: facts.Dismissed}
	for _, st := range steps {
		if st.Done {
			out.Completed++
		}
	}
	return out, nil
}

// DismissOnboarding hides the checklist (dismiss false: shows it again, the
// undo) and returns the new state, which every tab and device of the client
// also receives (onboarding.updated).
func (s *LifecycleService) DismissOnboarding(ctx context.Context, userID uuid.UUID, dismiss bool) (*OnboardingView, error) {
	var at *time.Time
	if dismiss {
		now := s.d.Clock.Now()
		at = &now
	}
	if err := s.d.Repo.SetOnboardingDismissed(ctx, userID, at); err != nil {
		return nil, apperror.Internal(err)
	}
	out, err := s.Onboarding(ctx, userID)
	if err != nil {
		return nil, err
	}
	warnErr(ctx, "onboarding: publish", s.d.Events.Publish(ctx, realtime.EventOnboardingUpdated, out, userID))
	return out, nil
}
