package domain

import (
	"time"

	"github.com/google/uuid"
)

// revisions 6.0 (docs/platform-extensions.md §26.5, migration 0033): card
// payment operations: the webhook log, disputes and early fraud warnings.

// WebhookStatus is where a provider notification stands in the webhook log.
type WebhookStatus string

// Webhook statuses.
const (
	// WebhookReceived: recorded, not (successfully) applied yet.
	WebhookReceived WebhookStatus = "received"
	// WebhookProcessed: applied.
	WebhookProcessed WebhookStatus = "processed"
	// WebhookFailed: applying it failed; the provider retries (answered 500).
	WebhookFailed WebhookStatus = "failed"
	// WebhookIgnored: verified, but nothing to do (an event type or a payment
	// the platform does not act on).
	WebhookIgnored WebhookStatus = "ignored"
)

// Valid reports whether s is a known status.
func (s WebhookStatus) Valid() bool {
	switch s {
	case WebhookReceived, WebhookProcessed, WebhookFailed, WebhookIgnored:
		return true
	}
	return false
}

// DisputeStatus is the state of a dispute or inquiry (the processor's own
// statuses).
type DisputeStatus string

// Dispute statuses. The warning_* ones are inquiries (a bank question that
// can still become a dispute), the others chargebacks.
const (
	DisputeWarningNeedsResponse DisputeStatus = "warning_needs_response"
	DisputeWarningUnderReview   DisputeStatus = "warning_under_review"
	DisputeWarningClosed        DisputeStatus = "warning_closed"
	DisputeNeedsResponse        DisputeStatus = "needs_response"
	DisputeUnderReview          DisputeStatus = "under_review"
	DisputeWon                  DisputeStatus = "won"
	DisputeLost                 DisputeStatus = "lost"
	DisputePrevented            DisputeStatus = "prevented"
)

// Valid reports whether s is a known status.
func (s DisputeStatus) Valid() bool {
	switch s {
	case DisputeWarningNeedsResponse, DisputeWarningUnderReview, DisputeWarningClosed, DisputeNeedsResponse,
		DisputeUnderReview, DisputeWon, DisputeLost, DisputePrevented:
		return true
	}
	return false
}

// Inquiry reports whether s is an inquiry status (nothing is frozen by it).
func (s DisputeStatus) Inquiry() bool {
	return s == DisputeWarningNeedsResponse || s == DisputeWarningUnderReview || s == DisputeWarningClosed
}

// Open reports whether the chargeback is in progress: it freezes the
// purchase's credits and, while the policy says so, blocks new purchases.
func (s DisputeStatus) Open() bool { return s == DisputeNeedsResponse || s == DisputeUnderReview }

// Closed reports whether the dispute or inquiry is over.
func (s DisputeStatus) Closed() bool {
	switch s {
	case DisputeWon, DisputeLost, DisputeWarningClosed, DisputePrevented:
		return true
	}
	return false
}

// AcceptsEvidence reports whether evidence can still be submitted.
func (s DisputeStatus) AcceptsEvidence() bool {
	return s == DisputeNeedsResponse || s == DisputeWarningNeedsResponse
}

// OpenDisputeStatuses lists the statuses that block purchases (for SQL filters).
var OpenDisputeStatuses = []DisputeStatus{DisputeNeedsResponse, DisputeUnderReview}

// PaymentDispute is a dispute or inquiry on a payment.
type PaymentDispute struct {
	ID            uuid.UUID     `gorm:"type:uuid;primaryKey"`
	Provider      string        `gorm:"not null"`
	DisputeRef    string        `gorm:"not null"`
	TransactionID uuid.UUID     `gorm:"type:uuid;not null"`
	UserID        uuid.UUID     `gorm:"type:uuid;not null"`
	AmountCents   int64         `gorm:"not null"`
	Currency      string        `gorm:"not null"`
	Reason        string        `gorm:"not null"`
	Status        DisputeStatus `gorm:"not null"`
	// PriorStatus is the payment's status before the dispute took effect; it
	// is back in place when the dispute is won.
	PriorStatus         TransactionStatus `gorm:"not null"`
	EvidenceDueBy       *time.Time
	EvidenceSubmittedAt *time.Time
	CreditsFrozen       int64 `gorm:"not null"`
	CreditsRestored     int64 `gorm:"not null"`
	FundsWithdrawn      bool  `gorm:"not null"`
	ActivatedAt         *time.Time
	ClosedAt            *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (PaymentDispute) TableName() string { return "payment_disputes" }

// Activated reports whether the dispute's effects were applied.
func (d *PaymentDispute) Activated() bool { return d.ActivatedAt != nil }

// FraudAction is what was done about an early fraud warning.
type FraudAction string

// Fraud warning actions.
const (
	FraudActionNone      FraudAction = "none"
	FraudActionRefunded  FraudAction = "refunded"
	FraudActionDismissed FraudAction = "dismissed"
)

// PaymentFraudWarning is an issuer's early fraud warning on a payment.
type PaymentFraudWarning struct {
	ID            uuid.UUID   `gorm:"type:uuid;primaryKey"`
	Provider      string      `gorm:"not null"`
	WarningRef    string      `gorm:"not null"`
	TransactionID uuid.UUID   `gorm:"type:uuid;not null"`
	UserID        uuid.UUID   `gorm:"type:uuid;not null"`
	FraudType     string      `gorm:"not null"`
	Actionable    bool        `gorm:"not null"`
	Action        FraudAction `gorm:"not null"`
	ActionedAt    *time.Time
	Note          string `gorm:"not null"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (PaymentFraudWarning) TableName() string { return "payment_fraud_warnings" }
