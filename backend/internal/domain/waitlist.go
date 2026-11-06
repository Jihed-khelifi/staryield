package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Waitlist limits.
const (
	// WaitlistMaxOpen caps how many advisors one client waits for at once.
	WaitlistMaxOpen = 20
	// WaitlistOpenTTL is how long an entry waits before it expires (the
	// Terms and Privacy Notice promise 7 days).
	WaitlistOpenTTL = 7 * 24 * time.Hour
	// WaitlistRetention is how long finished entries are kept.
	WaitlistRetention = 90 * 24 * time.Hour
)

// WaitlistEntry is a client waiting for an advisor to come online. It is
// open while NotifiedAt and DeletedAt are both nil. AvailableAt is set when
// the advisor came online and a notification is due.
type WaitlistEntry struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID      uuid.UUID `gorm:"type:uuid;not null"`
	PsychicID   uuid.UUID `gorm:"type:uuid;not null"`
	AvailableAt *time.Time
	NotifiedAt  *time.Time
	CreatedAt   time.Time      `gorm:"not null"`
	UpdatedAt   time.Time      `gorm:"not null"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

// TableName binds the model to its table.
func (WaitlistEntry) TableName() string { return "advisor_waitlist" }
