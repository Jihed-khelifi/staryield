package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SupportTopic routes a contact-form request.
type SupportTopic string

// Support topics offered by the contact form.
const (
	SupportGeneral   SupportTopic = "general"
	SupportPrivacy   SupportTopic = "privacy"
	SupportBilling   SupportTopic = "billing"
	SupportAdvisor   SupportTopic = "advisor"
	SupportTechnical SupportTopic = "technical"
)

// ValidSupportTopic reports whether t is an offered topic.
func ValidSupportTopic(t SupportTopic) bool {
	switch t {
	case SupportGeneral, SupportPrivacy, SupportBilling, SupportAdvisor, SupportTechnical:
		return true
	}
	return false
}

// SupportStatus is a ticket's lifecycle state.
type SupportStatus string

// Support ticket statuses.
// open = waiting for the team, pending = waiting for the requester.
const (
	SupportOpen     SupportStatus = "open"
	SupportPending  SupportStatus = "pending"
	SupportResolved SupportStatus = "resolved"
	SupportClosed   SupportStatus = "closed"
)

// ValidSupportStatus reports whether s is a ticket status.
func ValidSupportStatus(s SupportStatus) bool {
	switch s {
	case SupportOpen, SupportPending, SupportResolved, SupportClosed:
		return true
	}
	return false
}

// SupportPriority orders the queue.
type SupportPriority string

// Support ticket priorities.
const (
	SupportLow    SupportPriority = "low"
	SupportNormal SupportPriority = "normal"
	SupportHigh   SupportPriority = "high"
	SupportUrgent SupportPriority = "urgent"
)

// ValidSupportPriority reports whether p is a ticket priority.
func ValidSupportPriority(p SupportPriority) bool {
	switch p {
	case SupportLow, SupportNormal, SupportHigh, SupportUrgent:
		return true
	}
	return false
}

// MaxSupportMessages is the thread limit, the original request included.
const MaxSupportMessages = 200

// SupportTicket is a request sent through the contact form. The reply
// address and the message are sealed at rest.
type SupportTicket struct {
	ID           uuid.UUID     `gorm:"type:uuid;primaryKey"`
	Reference    string        `gorm:"not null"`
	UserID       *uuid.UUID    `gorm:"type:uuid"`
	Name         string        `gorm:"not null"`
	EmailEnc     string        `gorm:"not null"`
	EmailHash    string        `gorm:"not null"`
	Topic        SupportTopic  `gorm:"not null"`
	Subject      string        `gorm:"not null"`
	MessageEnc   string        `gorm:"not null"`
	MessageChars int           `gorm:"not null"`
	Locale       string        `gorm:"not null"`
	Status       SupportStatus `gorm:"not null"`
	IPHash       string        `gorm:"column:ip_hash;not null"`
	CreatedAt    time.Time     `gorm:"not null"`
	UpdatedAt    time.Time     `gorm:"not null"`
	DeletedAt    gorm.DeletedAt

	// revisions 3.0: support desk
	Priority              SupportPriority `gorm:"not null"`
	AssigneeID            *uuid.UUID      `gorm:"type:uuid"`
	Version               int             `gorm:"not null"`
	MessageCount          int             `gorm:"not null"`
	LastMessageAt         time.Time       `gorm:"not null"`
	LastCustomerMessageAt time.Time       `gorm:"not null"`
	LastStaffMessageAt    *time.Time
	FirstResponseAt       *time.Time
	ResolvedAt            *time.Time
	ClosedAt              *time.Time
	StaffSeenAt           *time.Time
	// AccessTokenHash is the keyed hash of the private-link token;
	// AccessTokenEnc a sealed copy so the same link can be e-mailed again.
	AccessTokenHash      *string
	AccessTokenEnc       *string
	AccessTokenExpiresAt *time.Time
	EmailMasked          string `gorm:"not null"`
}

// TableName binds the model to its table.
func (SupportTicket) TableName() string { return "support_tickets" }

// SupportAuthor says who wrote a ticket message.
type SupportAuthor string

// Message authors.
const (
	SupportAuthorCustomer SupportAuthor = "customer"
	SupportAuthorStaff    SupportAuthor = "staff"
	SupportAuthorSystem   SupportAuthor = "system"
)

// SupportVisibility says who may read a message.
type SupportVisibility string

// Message visibilities: internal notes and system entries never reach the requester.
const (
	SupportPublic   SupportVisibility = "public"
	SupportInternal SupportVisibility = "internal"
)

// SupportEmailStatus tracks the e-mail a public staff reply triggers.
type SupportEmailStatus string

// E-mail statuses of a message.
const (
	SupportEmailNone   SupportEmailStatus = "none"
	SupportEmailQueued SupportEmailStatus = "queued"
	SupportEmailSent   SupportEmailStatus = "sent"
	SupportEmailFailed SupportEmailStatus = "failed"
)

// SupportMessage is one entry of a ticket thread after the original request
// (which stays on the ticket). Bodies are sealed and immutable.
type SupportMessage struct {
	ID          uuid.UUID          `gorm:"type:uuid;primaryKey"`
	TicketID    uuid.UUID          `gorm:"type:uuid;not null"`
	AuthorType  SupportAuthor      `gorm:"not null"`
	AuthorID    *uuid.UUID         `gorm:"type:uuid"`
	Visibility  SupportVisibility  `gorm:"not null"`
	BodyEnc     string             `gorm:"not null"`
	BodyChars   int                `gorm:"not null"`
	EmailStatus SupportEmailStatus `gorm:"not null"`
	CreatedAt   time.Time          `gorm:"not null"`
}

// TableName binds the model to its table.
func (SupportMessage) TableName() string { return "support_messages" }
