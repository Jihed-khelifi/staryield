package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DeactivatedByClosure marks an account paused because its owner asked to
// close it; it is erased at ClosureScheduledAt unless the owner signs in to
// keep it.
const DeactivatedByClosure = "closure"

// UserSession is one signed-in device: a refresh-token family.
type UserSession struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID       uuid.UUID `gorm:"type:uuid;not null"`
	UserAgent    string    `gorm:"not null"`
	DeviceLabel  string    `gorm:"not null"`
	DeviceType   string    `gorm:"not null;default:unknown"`
	Browser      string    `gorm:"not null"`
	OS           string    `gorm:"column:os;not null"`
	IPHash       string    `gorm:"not null"`
	CountryCode  string    `gorm:"not null"`
	LastSeenAt   time.Time `gorm:"not null"`
	ExpiresAt    time.Time `gorm:"not null"`
	RevokedAt    *time.Time
	RevokeReason string `gorm:"not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TableName binds the model to its table.
func (UserSession) TableName() string { return "user_sessions" }

// ActionPurpose is what a one-time account token authorises.
type ActionPurpose string

// One-time token purposes.
const (
	ActionEmailChange       ActionPurpose = "email_change"
	ActionEmailChangeCancel ActionPurpose = "email_change_cancel"
	ActionReactivate        ActionPurpose = "reactivate"
	ActionClosureCancel     ActionPurpose = "closure_cancel"
	ActionEmailVerify       ActionPurpose = "email_verify"
	ActionExportDownload    ActionPurpose = "export_download"
)

// ActionToken is a single-use, expiring account token. Only the SHA-256 of
// the token is stored; PayloadEnc is sealed and bound to the row.
type ActionToken struct {
	ID         uuid.UUID     `gorm:"type:uuid;primaryKey"`
	UserID     uuid.UUID     `gorm:"type:uuid;not null"`
	Purpose    ActionPurpose `gorm:"not null"`
	TokenHash  string        `gorm:"not null"`
	PayloadEnc string        `gorm:"not null"`
	SubjectID  *uuid.UUID    `gorm:"type:uuid"`
	ExpiresAt  time.Time     `gorm:"not null"`
	UsedAt     *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TableName binds the model to its table.
func (ActionToken) TableName() string { return "user_action_tokens" }

// ConsentCategory is an optional processing purpose the user can refuse.
type ConsentCategory string

// Consent categories (essential processing needs no consent and is not
// recorded).
const (
	ConsentAnalytics            ConsentCategory = "analytics"
	ConsentMarketingAttribution ConsentCategory = "marketing_attribution"
	ConsentPersonalization      ConsentCategory = "personalization"
)

// ConsentCategories lists every category in display order.
var ConsentCategories = []ConsentCategory{ConsentAnalytics, ConsentMarketingAttribution, ConsentPersonalization}

// Valid reports whether c is a known category.
func (c ConsentCategory) Valid() bool {
	return c == ConsentAnalytics || c == ConsentMarketingAttribution || c == ConsentPersonalization
}

// Consent is the current choice for one category.
type Consent struct {
	UserID        uuid.UUID       `gorm:"type:uuid;primaryKey"`
	Category      ConsentCategory `gorm:"primaryKey"`
	Granted       bool            `gorm:"not null"`
	PolicyVersion string          `gorm:"not null"`
	Source        string          `gorm:"not null"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// TableName binds the model to its table.
func (Consent) TableName() string { return "user_consents" }

// ConsentEvent is the append-only history of consent changes.
type ConsentEvent struct {
	ID            uuid.UUID       `gorm:"type:uuid;primaryKey"`
	UserID        uuid.UUID       `gorm:"type:uuid;not null"`
	Category      ConsentCategory `gorm:"not null"`
	Granted       bool            `gorm:"not null"`
	PolicyVersion string          `gorm:"not null"`
	Source        string          `gorm:"not null"`
	IPHash        string          `gorm:"not null"`
	CreatedAt     time.Time
}

// TableName binds the model to its table.
func (ConsentEvent) TableName() string { return "consent_events" }

// ExportStatus is the lifecycle of a personal-data export.
type ExportStatus string

// Export statuses: queued → running → ready → expired, or failed.
const (
	ExportQueued  ExportStatus = "queued"
	ExportRunning ExportStatus = "running"
	ExportReady   ExportStatus = "ready"
	ExportFailed  ExportStatus = "failed"
	ExportExpired ExportStatus = "expired"
)

// DataExportJob is a requested copy of a user's personal data.
type DataExportJob struct {
	ID            uuid.UUID    `gorm:"type:uuid;primaryKey"`
	UserID        uuid.UUID    `gorm:"type:uuid;not null"`
	Status        ExportStatus `gorm:"not null"`
	ObjectKey     string       `gorm:"not null"`
	Bytes         int64        `gorm:"not null"`
	SHA256        string       `gorm:"column:sha256;not null"`
	Attempts      int          `gorm:"not null"`
	LockedUntil   *time.Time
	Error         string    `gorm:"not null"`
	RequestedAt   time.Time `gorm:"not null"`
	StartedAt     *time.Time
	CompletedAt   *time.Time
	ExpiresAt     *time.Time
	DownloadCount int `gorm:"not null"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt
}

// TableName binds the model to its table.
func (DataExportJob) TableName() string { return "data_export_jobs" }
