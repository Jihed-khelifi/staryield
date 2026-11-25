package domain

import (
	"time"

	"github.com/google/uuid"
)

// SiteSettingLifecycle is the lifecycle messaging document of site_settings:
// {"journeys": {key: {"enabled": bool, "delay_hours": int}}, "attribution_days": int}.
const SiteSettingLifecycle = "lifecycle"

// Lifecycle journeys (lifecycle_messages.journey).
const (
	JourneyVerifyReminder    = "verify_reminder"
	JourneyFirstReading      = "first_reading"
	JourneyAfterFirstReading = "after_first_reading"
	JourneyWinBack           = "win_back"
)

// LifecycleJourneys lists the journeys in the order the job runs them (the
// first to claim a user wins the frequency cap).
var LifecycleJourneys = []string{JourneyVerifyReminder, JourneyFirstReading, JourneyAfterFirstReading, JourneyWinBack}

// Lifecycle message statuses.
const (
	LifecycleSent    = "sent"
	LifecycleSkipped = "skipped"
)

// Reasons a due message was not sent (lifecycle_messages.skip_reason).
const (
	LifecycleSkipReferralsOff = "referrals_disabled"
)

// LifecycleMessage is the claim and the record of one nudge.
type LifecycleMessage struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID      uuid.UUID `gorm:"type:uuid;not null"`
	Journey     string    `gorm:"not null"`
	Cycle       int       `gorm:"not null"`
	Status      string    `gorm:"not null"`
	SkipReason  string    `gorm:"not null"`
	Channels    JSONMap   `gorm:"type:jsonb;not null"`
	ConvertedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (LifecycleMessage) TableName() string { return "lifecycle_messages" }
