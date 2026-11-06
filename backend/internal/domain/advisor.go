package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PhotoSubmissionStatus is the review state of an advisor-uploaded photo.
type PhotoSubmissionStatus string

// Photo submission states. Pending submissions wait for an administrator;
// approved ones became the public profile photo; rejected ones carry the
// reason shown to the advisor; withdrawn ones were removed by the advisor
// (or superseded by a newer upload) before review.
const (
	PhotoSubmissionPending   PhotoSubmissionStatus = "pending"
	PhotoSubmissionApproved  PhotoSubmissionStatus = "approved"
	PhotoSubmissionRejected  PhotoSubmissionStatus = "rejected"
	PhotoSubmissionWithdrawn PhotoSubmissionStatus = "withdrawn"
)

// Valid reports whether s is a known state.
func (s PhotoSubmissionStatus) Valid() bool {
	switch s {
	case PhotoSubmissionPending, PhotoSubmissionApproved, PhotoSubmissionRejected, PhotoSubmissionWithdrawn:
		return true
	}
	return false
}

// Limits of advisor photo submissions (mirrored by CHECK constraints).
const (
	PhotoReasonMaxChars = 500
	// PhotoObjectPrefix is where pending images live in the private bucket.
	PhotoObjectPrefix = "advisor-photos/"
)

// PhotoSubmission is a profile photo an advisor uploaded for review. The
// image is stored in the private bucket under ObjectKey until it is
// approved (then copied to the public advisor bucket) or cleaned up;
// ObjectKey is "" once the private object has been deleted.
type PhotoSubmission struct {
	ID          uuid.UUID             `gorm:"type:uuid;primaryKey"`
	PsychicID   uuid.UUID             `gorm:"type:uuid;not null"`
	Status      PhotoSubmissionStatus `gorm:"not null"`
	ObjectKey   string                `gorm:"not null"`
	ContentType string                `gorm:"not null"`
	Bytes       int                   `gorm:"not null"`
	Width       int                   `gorm:"not null"`
	Height      int                   `gorm:"not null"`
	// Reason is the rejection reason shown to the advisor.
	Reason     string     `gorm:"not null"`
	ReviewedBy *uuid.UUID `gorm:"type:uuid"`
	ReviewedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt
}

func (PhotoSubmission) TableName() string { return "psychic_photo_submissions" }

// Bounds of the advisor's own custom offering (psychic_services with kind
// 'custom'). The table CHECKs bound name (1–80), description (≤ 1000),
// delivery days (1–60) and price (1–1,000,000); advisors get a narrower
// price range than administrators.
const (
	CustomServiceNameMax        = 80
	CustomServiceDescriptionMax = 1000
	CustomServiceMinPrice       = 10
	CustomServiceMaxPrice       = 50_000
	CustomServiceMinDays        = 1
	CustomServiceMaxDays        = 60
	// CustomServiceSortOrder lists the custom offering after the curated
	// services on the profile.
	CustomServiceSortOrder = 1000
)
