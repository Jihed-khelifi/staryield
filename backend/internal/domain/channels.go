package domain

import (
	"time"

	"github.com/google/uuid"
)

// EmailCategory classifies an e-mail for delivery policy: security mail is
// always sent and exempt from the daily cap; billing and account mail is
// always sent; notification mail follows the user's e-mail preference;
// marketing is opt-in and never sent through a transactional-only provider.
type EmailCategory string

// E-mail categories.
const (
	EmailSecurity     EmailCategory = "security"
	EmailBilling      EmailCategory = "billing"
	EmailAccount      EmailCategory = "account"
	EmailNotification EmailCategory = "notification"
	EmailMarketing    EmailCategory = "marketing"
)

// Valid reports whether c is a known category.
func (c EmailCategory) Valid() bool {
	switch c {
	case EmailSecurity, EmailBilling, EmailAccount, EmailNotification, EmailMarketing:
		return true
	}
	return false
}

// EmailStatus is the delivery state of an outbox row.
type EmailStatus string

// Outbox states. queued → sending → sent is the happy path; sending rows
// whose lease expired are retried; failed, dead, suppressed and cancelled
// are final.
const (
	EmailQueued     EmailStatus = "queued"
	EmailSending    EmailStatus = "sending"
	EmailSent       EmailStatus = "sent"
	EmailFailed     EmailStatus = "failed"
	EmailDead       EmailStatus = "dead"
	EmailSuppressed EmailStatus = "suppressed"
	EmailCancelled  EmailStatus = "cancelled"
)

// Final reports whether the status will not change again.
func (s EmailStatus) Final() bool {
	switch s {
	case EmailSent, EmailFailed, EmailDead, EmailSuppressed, EmailCancelled:
		return true
	}
	return false
}

// EmailOutbox is a queued transactional e-mail. ToSealed and PayloadSealed
// are AES-GCM ciphertexts bound to the row id; both are wiped once the row is
// final.
type EmailOutbox struct {
	ID                uuid.UUID     `gorm:"type:uuid;primaryKey"`
	UserID            *uuid.UUID    `gorm:"type:uuid"`
	Category          EmailCategory `gorm:"not null"`
	Template          string        `gorm:"not null"`
	Locale            string        `gorm:"not null"`
	ToHash            string        `gorm:"not null"`
	ToSealed          string        `gorm:"not null"`
	PayloadSealed     string        `gorm:"not null"`
	DedupeKey         string        `gorm:"not null"`
	Status            EmailStatus   `gorm:"not null"`
	StatusReason      string        `gorm:"not null"`
	Attempts          int           `gorm:"not null"`
	NextAttemptAt     time.Time     `gorm:"not null"`
	LockedUntil       *time.Time
	LastError         string `gorm:"not null"`
	Provider          string `gorm:"not null"`
	ProviderMessageID string `gorm:"not null"`
	SentAt            *time.Time
	Timestamps
}

func (EmailOutbox) TableName() string { return "email_outbox" }

// SuppressionReason explains why an address is not e-mailed.
type SuppressionReason string

// Suppression reasons.
const (
	SuppressHardBounce SuppressionReason = "hard_bounce"
	SuppressComplaint  SuppressionReason = "complaint"
	SuppressManual     SuppressionReason = "manual"
)

// EmailSuppression blocks e-mail to an address, identified only by its keyed
// hash. A nil ExpiresAt suppresses permanently.
type EmailSuppression struct {
	EmailHash string            `gorm:"primaryKey"`
	Reason    SuppressionReason `gorm:"not null"`
	Source    string            `gorm:"not null"`
	ExpiresAt *time.Time
	Timestamps
}

func (EmailSuppression) TableName() string { return "email_suppressions" }

// PushSubscription is a browser's Web Push registration. The endpoint and
// keys are sealed (bound to EndpointHash); unsubscribing soft-deletes it.
type PushSubscription struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID         uuid.UUID `gorm:"type:uuid;not null"`
	EndpointHash   string    `gorm:"not null"`
	EndpointSealed string    `gorm:"not null"`
	P256dhSealed   string    `gorm:"column:p256dh_sealed;not null"`
	AuthSealed     string    `gorm:"not null"`
	UserAgent      string    `gorm:"not null"`
	LastSuccessAt  *time.Time
	FailureCount   int `gorm:"not null"`
	SoftDeleteTimestamps
}

func (PushSubscription) TableName() string { return "push_subscriptions" }
