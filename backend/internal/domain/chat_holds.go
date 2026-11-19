package domain

import (
	"time"

	"github.com/google/uuid"
)

// revisions 6.0 (docs/platform-extensions.md §26.1): the free-trial hold.

// HoldReason says why a reading was held.
type HoldReason string

// Hold reasons: the reading had only used free minutes so far, or paid
// credits ran out.
const (
	HoldTrialEnded       HoldReason = "trial_ended"
	HoldCreditsExhausted HoldReason = "credits_exhausted"
)

// HoldStatus is the lifecycle of a hold.
type HoldStatus string

// Hold statuses. open: waiting for a top-up. converted: a minute was paid and
// the reading continues. expired: the reading ended and its hidden replies are
// still hidden. revealed: the client paid one minute to read them.
const (
	HoldOpen      HoldStatus = "open"
	HoldConverted HoldStatus = "converted"
	HoldExpired   HoldStatus = "expired"
	HoldRevealed  HoldStatus = "revealed"
)

var holdTransitions = map[HoldStatus][]HoldStatus{
	HoldOpen:    {HoldConverted, HoldExpired},
	HoldExpired: {HoldRevealed},
}

// CanTransition reports whether from→to is allowed.
func (s HoldStatus) CanTransition(to HoldStatus) bool {
	for _, next := range holdTransitions[s] {
		if next == to {
			return true
		}
	}
	return false
}

// IsFinal reports whether no further transition is possible.
func (s HoldStatus) IsFinal() bool { return len(holdTransitions[s]) == 0 }

// Hold timing and message bounds.
const (
	// HoldExtension is how far the expiry job pushes a hold forward, at a time,
	// while a payment the client started during it is still processing.
	HoldExtension = 15 * time.Second
	// LockedBodyLengthCap caps the body_length a client sees of a locked message.
	LockedBodyLengthCap = 2000
)

// ChatHold is one hold of a reading (table chat_session_holds). A reading can
// be held more than once (after_paid).
type ChatHold struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey"`
	SessionID      uuid.UUID  `gorm:"type:uuid;not null"`
	ConversationID uuid.UUID  `gorm:"type:uuid;not null"`
	ClientID       uuid.UUID  `gorm:"type:uuid;not null"`
	PsychicID      uuid.UUID  `gorm:"type:uuid;not null"`
	Reason         HoldReason `gorm:"not null"`
	Status         HoldStatus `gorm:"not null"`
	StartedAt      time.Time  `gorm:"not null"`
	// ExpiresAt moves forward, at most to MaxExpiresAt, while a payment the
	// client started is processing.
	ExpiresAt    time.Time `gorm:"not null"`
	MaxExpiresAt time.Time `gorm:"not null"`
	ResolvedAt   *time.Time
	RevealedAt   *time.Time
	// PriceCredits is what continuing or revealing costs the client (the
	// reading's per-minute rate); ListRateCredits is the advisor's rate, on
	// which the reveal earning is computed.
	PriceCredits    int64 `gorm:"not null"`
	ListRateCredits int64 `gorm:"not null"`
	HeldMessages    int   `gorm:"not null"`
	// ConvertedCredits is what the top-up minute cost (0 when a free minute paid it).
	ConvertedCredits int64 `gorm:"not null"`
	RevealCredits    int64 `gorm:"not null"`
	// RevealRevenueMillicents is the revenue basis of the paid credits the
	// reveal consumed (credit lots, §24.5).
	RevealRevenueMillicents int64 `gorm:"not null"`
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// TableName binds the model to its table.
func (ChatHold) TableName() string { return "chat_session_holds" }

// HoldReasonFor is the reason of a hold entered by a reading that has billed
// credits and free minutes as given.
func HoldReasonFor(freeMinutesUsed int, creditsCharged int64) HoldReason {
	if TrialEnded(freeMinutesUsed, creditsCharged) {
		return HoldTrialEnded
	}
	return HoldCreditsExhausted
}

// TrialEnded reports whether a reading has only used free minutes so far.
func TrialEnded(freeMinutesUsed int, creditsCharged int64) bool {
	return freeMinutesUsed > 0 && creditsCharged == 0
}
