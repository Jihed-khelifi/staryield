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
const (
	SupportOpen   SupportStatus = "open"
	SupportClosed SupportStatus = "closed"
)

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
}

// TableName binds the model to its table.
func (SupportTicket) TableName() string { return "support_tickets" }
