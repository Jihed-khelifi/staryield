package domain

import (
	"time"

	"github.com/google/uuid"
)

// Growth: the referral loop (migration 0036). An inviter earns free minutes,
// granted at launch, for each invited friend who signs up with their link,
// verifies the e-mail address and completes a reading.

// SiteSettingReferralProgram is the referral program's settings document
// (the CHECK constraints of site_settings list the same key).
const SiteSettingReferralProgram = "referral_program"

// Bounds of the referral program setting (migration 0036 says the same).
const (
	ReferralRewardMinutesMin     = 1
	ReferralRewardMinutesMax     = 60
	ReferralRewardMinutesDefault = 5
	ReferralMaxRewardsMin        = 1
	ReferralMaxRewardsMax        = 500
	ReferralMaxRewardsDefault    = 20
)

// ReferralStatus is where an invited friend stands.
type ReferralStatus string

// Referral statuses.
const (
	ReferralJoined    ReferralStatus = "joined"
	ReferralQualified ReferralStatus = "qualified"
	ReferralGranted   ReferralStatus = "granted"
	ReferralRejected  ReferralStatus = "rejected"
)

// Valid reports whether s is a known status.
func (s ReferralStatus) Valid() bool {
	switch s {
	case ReferralJoined, ReferralQualified, ReferralGranted, ReferralRejected:
		return true
	}
	return false
}

// Machine-readable reasons a referral was not rewarded (Referral.RejectReason)
// and flags that hold one for review (Referral.Flags). Admin rejections carry
// the administrator's own text.
const (
	ReferralRejectCap          = "cap_reached"
	ReferralRejectSelf         = "self_referral"
	ReferralFlagSameNetwork    = "same_network"
	ReferralFlagSignupBurst    = "signup_burst"
	ReferralRejectReasonMaxLen = 200
)

// ReferralCode is an inviter's personal code.
type ReferralCode struct {
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	Code      string    `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (ReferralCode) TableName() string { return "referral_codes" }

// Referral is one invited friend. RefereeID is nil once the friend's account
// is erased.
type Referral struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey"`
	ReferrerID     uuid.UUID      `gorm:"type:uuid;not null"`
	RefereeID      *uuid.UUID     `gorm:"type:uuid"`
	Code           string         `gorm:"not null"`
	Status         ReferralStatus `gorm:"not null"`
	RewardMinutes  int            `gorm:"not null"`
	Flagged        bool           `gorm:"not null"`
	Flags          StringList     `gorm:"type:jsonb;not null"`
	RejectReason   string         `gorm:"not null"`
	FirstSessionID *uuid.UUID     `gorm:"type:uuid"`
	SignupIPHash   string         `gorm:"column:signup_ip_hash;not null"`
	JoinedAt       time.Time      `gorm:"not null"`
	QualifiedAt    *time.Time
	GrantedAt      *time.Time
	DecidedBy      *uuid.UUID `gorm:"type:uuid"`
	DecidedAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (Referral) TableName() string { return "referrals" }
