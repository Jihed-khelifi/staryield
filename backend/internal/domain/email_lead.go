package domain

import (
	"time"

	"github.com/google/uuid"
)

// revisions 6.0 (§26.4): the calculators' e-mail report.

// ReportKind names a calculator whose report can be e-mailed.
type ReportKind string

// Calculators (the same names as the public routes and the analytics).
const (
	ReportNatalChart ReportKind = "natal-chart"
	ReportPathOfLife ReportKind = "path-of-life"
	ReportAstrology  ReportKind = "astrology"
)

// ReportKinds lists the calculators in their display order.
var ReportKinds = []ReportKind{ReportNatalChart, ReportAstrology, ReportPathOfLife}

// Valid reports whether k is a known calculator.
func (k ReportKind) Valid() bool {
	switch k {
	case ReportNatalChart, ReportPathOfLife, ReportAstrology:
		return true
	}
	return false
}

// MarketingStatus is where an e-mail lead stands on marketing e-mail.
type MarketingStatus string

// Marketing states: none = never opted in; unsubscribed is final until the
// address opts in again with a box it ticked itself.
const (
	MarketingNone         MarketingStatus = "none"
	MarketingSubscribed   MarketingStatus = "subscribed"
	MarketingUnsubscribed MarketingStatus = "unsubscribed"
)

// Valid reports whether s is a known status.
func (s MarketingStatus) Valid() bool {
	switch s {
	case MarketingNone, MarketingSubscribed, MarketingUnsubscribed:
		return true
	}
	return false
}

// EmailLead is an address that asked for a calculator report. EmailSealed is
// an AES-GCM ciphertext bound to the row id; birth details are never stored.
type EmailLead struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	EmailHash   string     `gorm:"not null"`
	EmailSealed string     `gorm:"not null"`
	Locale      string     `gorm:"not null"`
	Country     string     `gorm:"not null"`
	Source      string     `gorm:"not null"`
	Kinds       StringList `gorm:"type:jsonb;not null"`
	ReportsSent int        `gorm:"not null"`

	MarketingStatus MarketingStatus `gorm:"not null"`
	// Consent evidence of the latest opt-in (nil while never subscribed).
	ConsentAt          *time.Time
	ConsentPrechecked  *bool
	ConsentTextVersion *string
	ConsentIPHash      *string
	ConsentUserAgent   *string
	UnsubscribedAt     *time.Time

	UserID        *uuid.UUID `gorm:"type:uuid"`
	LastRequestAt time.Time  `gorm:"not null"`
	Timestamps
}

// TableName implements gorm's Tabler.
func (EmailLead) TableName() string { return "email_leads" }

// HasKind reports whether the lead already asked for k.
func (l *EmailLead) HasKind(k ReportKind) bool {
	for _, have := range l.Kinds {
		if have == string(k) {
			return true
		}
	}
	return false
}
