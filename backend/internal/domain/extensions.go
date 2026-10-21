package domain

import (
	"time"

	"github.com/google/uuid"
)

// Attachment is a private chat image. The object lives in the private chat
// bucket and is only streamed to the conversation's two participants.
type Attachment struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	MessageID      uuid.UUID `gorm:"type:uuid;not null"`
	ConversationID uuid.UUID `gorm:"type:uuid;not null"`
	UploaderID     uuid.UUID `gorm:"type:uuid;not null"`
	ObjectKey      string    `gorm:"not null"`
	ContentType    string    `gorm:"not null"`
	Width          int       `gorm:"not null"`
	Height         int       `gorm:"not null"`
	Bytes          int       `gorm:"not null"`
	CreatedAt      time.Time
}

func (Attachment) TableName() string { return "chat_attachments" }

// ReviewStatus is the moderation state of a review.
type ReviewStatus string

// Review statuses.
const (
	ReviewVisible ReviewStatus = "visible"
	ReviewHidden  ReviewStatus = "hidden"
)

// Review is a client's rating of an advisor after a session.
type Review struct {
	ID             uuid.UUID    `gorm:"type:uuid;primaryKey"`
	SessionID      uuid.UUID    `gorm:"type:uuid;not null"`
	ClientID       uuid.UUID    `gorm:"type:uuid;not null"`
	PsychicID      uuid.UUID    `gorm:"type:uuid;not null"`
	Rating         int          `gorm:"not null"`
	Comment        string       `gorm:"not null"`
	Status         ReviewStatus `gorm:"not null"`
	ModerationNote string       `gorm:"not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (Review) TableName() string { return "psychic_reviews" }

// WithdrawalStatus is the payout lifecycle.
type WithdrawalStatus string

// Withdrawal statuses: pending → approved → paid; pending|approved →
// rejected; pending → cancelled (by the advisor).
const (
	WithdrawalPending   WithdrawalStatus = "pending"
	WithdrawalApproved  WithdrawalStatus = "approved"
	WithdrawalPaid      WithdrawalStatus = "paid"
	WithdrawalRejected  WithdrawalStatus = "rejected"
	WithdrawalCancelled WithdrawalStatus = "cancelled"
)

// Reserves reports whether the withdrawal still holds earned credits.
func (s WithdrawalStatus) Reserves() bool {
	return s == WithdrawalPending || s == WithdrawalApproved || s == WithdrawalPaid
}

// Withdrawal is an advisor's payout request. Destination details are
// encrypted at rest.
type Withdrawal struct {
	ID              uuid.UUID        `gorm:"type:uuid;primaryKey"`
	PsychicID       uuid.UUID        `gorm:"type:uuid;not null"`
	AmountCredits   int64            `gorm:"not null"`
	AmountCents     int64            `gorm:"not null"`
	Currency        string           `gorm:"not null"`
	Method          string           `gorm:"not null"`
	DestinationEnc  string           `gorm:"not null"`
	DestinationHint string           `gorm:"not null"`
	Status          WithdrawalStatus `gorm:"not null"`
	Note            string           `gorm:"not null"`
	Reference       string           `gorm:"not null"`
	IdempotencyKey  string           `gorm:"not null"`
	RequestedAt     time.Time        `gorm:"not null"`
	ProcessedAt     *time.Time
	ProcessedBy     *uuid.UUID `gorm:"type:uuid"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (Withdrawal) TableName() string { return "withdrawals" }

// Notification is an in-app notification. Text is rendered by clients in
// the viewer's language from Kind and Data.
type Notification struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID      uuid.UUID  `gorm:"type:uuid;not null"`
	Kind        string     `gorm:"not null"`
	Data        JSONMap    `gorm:"type:jsonb;not null"`
	Link        string     `gorm:"not null"`
	BroadcastID *uuid.UUID `gorm:"type:uuid"`
	ReadAt      *time.Time
	CreatedAt   time.Time
}

func (Notification) TableName() string { return "notifications" }

// Broadcast is an admin message to a set of users.
type Broadcast struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey"`
	CreatedBy  *uuid.UUID `gorm:"type:uuid"`
	Audience   string     `gorm:"not null"`
	OnlineOnly bool       `gorm:"not null"`
	TargetIDs  StringList `gorm:"type:jsonb;not null"`
	CampaignID *uuid.UUID `gorm:"type:uuid"`
	Title      JSONMap    `gorm:"type:jsonb;not null"`
	Body       JSONMap    `gorm:"type:jsonb;not null"`
	Link       string     `gorm:"not null"`
	Status     string     `gorm:"not null"`
	Recipients int        `gorm:"not null"`
	CreatedAt  time.Time
	SentAt     *time.Time
}

func (Broadcast) TableName() string { return "broadcasts" }

// Campaign is a marketing campaign; Slug is its utm_campaign value.
type Campaign struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name        string    `gorm:"not null"`
	Slug        string    `gorm:"not null"`
	Channel     string    `gorm:"not null"`
	Status      string    `gorm:"not null"`
	BudgetCents int64     `gorm:"not null"`
	Currency    string    `gorm:"not null"`
	StartsAt    *time.Time
	EndsAt      *time.Time
	Notes       string     `gorm:"not null"`
	CreatedBy   *uuid.UUID `gorm:"type:uuid"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Campaign) TableName() string { return "campaigns" }

// ShortLink redirects a short code to a UTM-tagged destination.
type ShortLink struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	Code        string     `gorm:"not null"`
	CampaignID  *uuid.UUID `gorm:"type:uuid"`
	Destination string     `gorm:"not null"`
	UTMSource   string     `gorm:"column:utm_source;not null"`
	UTMMedium   string     `gorm:"column:utm_medium;not null"`
	UTMCampaign string     `gorm:"column:utm_campaign;not null"`
	UTMContent  string     `gorm:"column:utm_content;not null"`
	UTMTerm     string     `gorm:"column:utm_term;not null"`
	Label       string     `gorm:"not null"`
	Active      bool       `gorm:"not null"`
	ExpiresAt   *time.Time
	Clicks      int64      `gorm:"not null"`
	CreatedBy   *uuid.UUID `gorm:"type:uuid"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (ShortLink) TableName() string { return "short_links" }

// LinkClick is one recorded short-link visit (no raw IP is stored).
type LinkClick struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey"`
	LinkID       uuid.UUID  `gorm:"type:uuid;not null"`
	CampaignID   *uuid.UUID `gorm:"type:uuid"`
	VisitorHash  string     `gorm:"not null"`
	ReferrerHost string     `gorm:"not null"`
	Device       string     `gorm:"not null"`
	IsBot        bool       `gorm:"not null"`
	CreatedAt    time.Time
}

func (LinkClick) TableName() string { return "link_clicks" }

// Attribution is a user's first-touch acquisition source.
type Attribution struct {
	UserID       uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ClickID      *uuid.UUID `gorm:"type:uuid"`
	LinkID       *uuid.UUID `gorm:"type:uuid"`
	CampaignID   *uuid.UUID `gorm:"type:uuid"`
	UTMSource    string     `gorm:"column:utm_source;not null"`
	UTMMedium    string     `gorm:"column:utm_medium;not null"`
	UTMCampaign  string     `gorm:"column:utm_campaign;not null"`
	UTMContent   string     `gorm:"column:utm_content;not null"`
	UTMTerm      string     `gorm:"column:utm_term;not null"`
	ReferrerHost string     `gorm:"not null"`
	LandingPath  string     `gorm:"not null"`
	FirstSeenAt  *time.Time
	CreatedAt    time.Time
}

func (Attribution) TableName() string { return "user_attributions" }

// AdminMFA is an administrator's TOTP enrolment.
type AdminMFA struct {
	UserID         uuid.UUID  `gorm:"type:uuid;primaryKey"`
	SecretEnc      string     `gorm:"not null"`
	LastUsedStep   int64      `gorm:"not null"`
	RecoveryHashes StringList `gorm:"type:jsonb;not null"`
	EnabledAt      time.Time  `gorm:"not null"`
	UpdatedAt      time.Time
}

func (AdminMFA) TableName() string { return "admin_mfa" }

// AdminSession is a server-side admin console session.
type AdminSession struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID       uuid.UUID `gorm:"type:uuid;not null"`
	TokenHash    string    `gorm:"not null"`
	IPHash       string    `gorm:"not null"`
	UserAgent    string    `gorm:"not null"`
	CreatedAt    time.Time `gorm:"not null"`
	LastSeenAt   time.Time `gorm:"not null"`
	ExpiresAt    time.Time `gorm:"not null"`
	MFAAt        time.Time `gorm:"column:mfa_at;not null"`
	RevokedAt    *time.Time
	RevokeReason string `gorm:"not null"`
}

func (AdminSession) TableName() string { return "admin_sessions" }
