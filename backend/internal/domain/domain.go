// Package domain defines the persistent entities, enumerations and pure
// business rules (state machines, billing arithmetic) of the platform. It has
// no dependency on transport or storage code.
package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Role is a user's RBAC role.
type Role string

// Roles recognised by the platform (spec §2).
const (
	RoleClient  Role = "client"
	RolePsychic Role = "psychic"
	RoleAdmin   Role = "admin"
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool { return r == RoleClient || r == RolePsychic || r == RoleAdmin }

// UserStatus is the lifecycle state of an account.
type UserStatus string

// Account statuses.
const (
	UserActive      UserStatus = "active"
	UserDeactivated UserStatus = "deactivated"
	UserBanned      UserStatus = "banned"
	UserDeleted     UserStatus = "deleted"
)

// Valid reports whether s is a known status.
func (s UserStatus) Valid() bool {
	return s == UserActive || s == UserDeactivated || s == UserBanned || s == UserDeleted
}

// User is an authenticated principal.
type User struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey"`
	Email           string     `gorm:"not null"`
	PasswordHash    string     `gorm:"not null"`
	Role            Role       `gorm:"not null"`
	Status          UserStatus `gorm:"not null"`
	DisplayName     string     `gorm:"not null"`
	Locale          string     `gorm:"not null"`
	EmailVerifiedAt *time.Time
	TermsAcceptedAt time.Time `gorm:"not null"`
	// TokenVersion is embedded in access tokens; incrementing it revokes
	// every outstanding access token of the user immediately.
	TokenVersion    int `gorm:"not null"`
	LastLoginAt     *time.Time
	StatusReason    string `gorm:"not null"`
	StatusChangedAt *time.Time
	DeletedAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (User) TableName() string { return "users" }

// Active reports whether the account may sign in.
func (u *User) Active() bool { return u.Status == UserActive }

// Profile holds personal details (FR-03, FR-08).
type Profile struct {
	UserID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	Nickname        string     `gorm:"not null"`
	BirthDate       *time.Time `gorm:"type:date"`
	BirthTime       string     `gorm:"not null"`
	BirthPlaceID    string     `gorm:"not null"`
	BirthPlace      string     `gorm:"not null"`
	BirthLatitude   *float64
	BirthLongitude  *float64
	BirthTimezone   string `gorm:"not null"`
	Location        string `gorm:"not null"`
	Mood            string `gorm:"not null"`
	Theme           string `gorm:"not null"`
	AvatarURL       string `gorm:"not null"`
	AvatarKey       string `gorm:"not null"`
	NotifyHoroscope bool   `gorm:"not null"`
	NotifyEvents    bool   `gorm:"not null"`
	NotifyChart     bool   `gorm:"not null"`
	CountryCode     string `gorm:"not null"`
	UpdatedAt       time.Time
}

func (Profile) TableName() string { return "user_profiles" }

// RefreshToken is a rotating, single-use refresh credential. Tokens issued
// from the same login share a FamilyID; reuse of a rotated token revokes the
// family (theft detection).
type RefreshToken struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID       uuid.UUID `gorm:"type:uuid;not null"`
	FamilyID     uuid.UUID `gorm:"type:uuid;not null"`
	TokenHash    string    `gorm:"not null"`
	ExpiresAt    time.Time `gorm:"not null"`
	RevokedAt    *time.Time
	RevokeReason string `gorm:"not null"`
	ReplacedBy   *uuid.UUID
	UserAgent    string `gorm:"not null"`
	IPHash       string `gorm:"not null"`
	CreatedAt    time.Time
	LastUsedAt   *time.Time
}

func (RefreshToken) TableName() string { return "refresh_tokens" }

// PasswordResetToken is a single-use password reset credential.
type PasswordResetToken struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;not null"`
	TokenHash string    `gorm:"not null"`
	ExpiresAt time.Time `gorm:"not null"`
	UsedAt    *time.Time
	CreatedAt time.Time
}

func (PasswordResetToken) TableName() string { return "password_reset_tokens" }

// Availability is the status a psychic chooses for themselves.
type Availability string

// Psychic availability values.
const (
	AvailabilityOnline  Availability = "online"
	AvailabilityAway    Availability = "away"
	AvailabilityOffline Availability = "offline"
)

// Valid reports whether a is a known availability.
func (a Availability) Valid() bool {
	return a == AvailabilityOnline || a == AvailabilityAway || a == AvailabilityOffline
}

// PresenceStatus is the effective, server-computed status shown to clients.
type PresenceStatus string

// Effective psychic statuses.
const (
	PresenceOnline  PresenceStatus = "online"
	PresenceBusy    PresenceStatus = "busy"
	PresenceAway    PresenceStatus = "away"
	PresenceOffline PresenceStatus = "offline"
)

// StringList is a JSONB-encoded []string.
type StringList []string

// Value implements driver.Valuer.
func (s StringList) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal([]string(s))
	return string(b), err
}

// Scan implements sql.Scanner.
func (s *StringList) Scan(src any) error {
	var b []byte
	switch v := src.(type) {
	case nil:
		*s = StringList{}
		return nil
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("domain: StringList: unsupported source")
	}
	var out []string
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	*s = out
	return nil
}

// JSONMap is a JSONB-encoded object.
type JSONMap map[string]any

// Value implements driver.Valuer.
func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(map[string]any(m))
	return string(b), err
}

// Scan implements sql.Scanner.
func (m *JSONMap) Scan(src any) error {
	var b []byte
	switch v := src.(type) {
	case nil:
		*m = JSONMap{}
		return nil
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("domain: JSONMap: unsupported source")
	}
	out := map[string]any{}
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	*m = out
	return nil
}

// Psychic is an advisor's public profile.
type Psychic struct {
	UserID          uuid.UUID    `gorm:"type:uuid;primaryKey"`
	Slug            string       `gorm:"not null"`
	Name            string       `gorm:"not null"`
	Headline        string       `gorm:"not null"`
	About           StringList   `gorm:"type:jsonb;not null"`
	Philosophy      string       `gorm:"not null"`
	ImageURL        string       `gorm:"not null"`
	Zodiac          string       `gorm:"not null"`
	Skills          StringList   `gorm:"type:jsonb;not null"`
	Rating          float64      `gorm:"not null"`
	ReviewsCount    int          `gorm:"not null"`
	ReadingsCount   int          `gorm:"not null"`
	YearsExperience int          `gorm:"not null"`
	RatePerMinute   int64        `gorm:"not null"`
	Availability    Availability `gorm:"not null"`
	Listed          bool         `gorm:"not null"`
	SortOrder       int          `gorm:"not null"`
	Featured        bool         `gorm:"not null"`
	FeaturedRank    int          `gorm:"not null"`
	ImageKey        string       `gorm:"not null"`
	RatingSum       float64      `gorm:"not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Services        []PsychicService `gorm:"foreignKey:PsychicID;references:UserID"`
}

func (Psychic) TableName() string { return "psychics" }

// PsychicService is a pre-priced custom request listed on a profile.
type PsychicService struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	PsychicID    uuid.UUID `gorm:"type:uuid;not null"`
	Name         string    `gorm:"not null"`
	Subtitle     string    `gorm:"not null"`
	Description  string    `gorm:"not null"`
	PriceCredits int64     `gorm:"not null"`
	Active       bool      `gorm:"not null"`
	SortOrder    int       `gorm:"not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (PsychicService) TableName() string { return "psychic_services" }

// Wallet is a user's credit balance. Held credits are reserved for pending
// custom requests (BR-06). FreeMinutes are promotional chat minutes.
type Wallet struct {
	UserID      uuid.UUID `gorm:"type:uuid;primaryKey"`
	Balance     int64     `gorm:"not null"`
	Held        int64     `gorm:"not null"`
	FreeMinutes int       `gorm:"not null"`
	UpdatedAt   time.Time
}

func (Wallet) TableName() string { return "wallets" }

// LedgerEntryType classifies credit movements.
type LedgerEntryType string

// Ledger entry types.
const (
	LedgerPurchase        LedgerEntryType = "purchase"
	LedgerSignupBonus     LedgerEntryType = "signup_bonus"
	LedgerChatCharge      LedgerEntryType = "chat_charge"
	LedgerHold            LedgerEntryType = "hold"
	LedgerHoldRelease     LedgerEntryType = "hold_release"
	LedgerHoldRefund      LedgerEntryType = "hold_refund"
	LedgerAdminAdjustment LedgerEntryType = "admin_adjustment"
	LedgerRefund          LedgerEntryType = "refund"
)

// LedgerEntry is an immutable credit movement (BR-10). Amount changes the
// available balance; HeldAmount changes the held balance.
type LedgerEntry struct {
	ID             uuid.UUID       `gorm:"type:uuid;primaryKey"`
	UserID         uuid.UUID       `gorm:"type:uuid;not null"`
	EntryType      LedgerEntryType `gorm:"not null"`
	Amount         int64           `gorm:"not null"`
	HeldAmount     int64           `gorm:"not null"`
	BalanceAfter   int64           `gorm:"not null"`
	HeldAfter      int64           `gorm:"not null"`
	ReferenceType  string          `gorm:"not null"`
	ReferenceID    *uuid.UUID      `gorm:"type:uuid"`
	IdempotencyKey string          `gorm:"not null"`
	Description    string          `gorm:"not null"`
	ActorID        *uuid.UUID      `gorm:"type:uuid"`
	CreatedAt      time.Time
}

func (LedgerEntry) TableName() string { return "credit_ledger" }

// CreditPack is a purchasable bundle of credits (admin-managed, BR-09).
type CreditPack struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name       string    `gorm:"not null"`
	Credits    int64     `gorm:"not null"`
	PriceCents int64     `gorm:"not null"`
	Currency   string    `gorm:"not null"`
	Active     bool      `gorm:"not null"`
	Featured   bool      `gorm:"not null"`
	SortOrder  int       `gorm:"not null"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (CreditPack) TableName() string { return "credit_packs" }

// PurchaseStatus is the lifecycle of a credit purchase.
type PurchaseStatus string

// Purchase statuses.
const (
	PurchasePending   PurchaseStatus = "pending"
	PurchaseSucceeded PurchaseStatus = "succeeded"
	PurchaseFailed    PurchaseStatus = "failed"
	PurchaseRefunded  PurchaseStatus = "refunded"
)

// CreditPurchase records a payment for a credit pack (FR-05).
type CreditPurchase struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey"`
	UserID         uuid.UUID      `gorm:"type:uuid;not null"`
	PackID         uuid.UUID      `gorm:"type:uuid;not null"`
	Credits        int64          `gorm:"not null"`
	PriceCents     int64          `gorm:"not null"`
	Currency       string         `gorm:"not null"`
	Status         PurchaseStatus `gorm:"not null"`
	Provider       string         `gorm:"not null"`
	ProviderRef    *string
	IdempotencyKey string `gorm:"not null"`
	ReceiptEmail   string `gorm:"not null"`
	FailureReason  string `gorm:"not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

func (CreditPurchase) TableName() string { return "credit_purchases" }

// Conversation is the long-lived thread between one client and one psychic.
type Conversation struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ClientID          uuid.UUID  `gorm:"type:uuid;not null"`
	PsychicID         uuid.UUID  `gorm:"type:uuid;not null"`
	LastMessageID     *uuid.UUID `gorm:"type:uuid"`
	LastMessageAt     *time.Time
	ClientLastReadAt  *time.Time
	PsychicLastReadAt *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (Conversation) TableName() string { return "conversations" }

// IsParticipant reports whether userID belongs to the conversation.
func (c *Conversation) IsParticipant(userID uuid.UUID) bool {
	return c.ClientID == userID || c.PsychicID == userID
}

// Counterpart returns the other participant.
func (c *Conversation) Counterpart(userID uuid.UUID) uuid.UUID {
	if c.ClientID == userID {
		return c.PsychicID
	}
	return c.ClientID
}

// MessageKind distinguishes participant text from system notices.
type MessageKind string

// Message kinds.
const (
	MessageText   MessageKind = "text"
	MessageSystem MessageKind = "system"
	MessageImage  MessageKind = "image"
)

// Message is a persisted chat message (the session transcript).
type Message struct {
	ID             uuid.UUID   `gorm:"type:uuid;primaryKey"`
	ConversationID uuid.UUID   `gorm:"type:uuid;not null"`
	SessionID      uuid.UUID   `gorm:"type:uuid;not null"`
	SenderID       *uuid.UUID  `gorm:"type:uuid"`
	Kind           MessageKind `gorm:"not null"`
	Body           string      `gorm:"not null"`
	ClientMsgID    *string
	CreatedAt      time.Time
}

func (Message) TableName() string { return "chat_messages" }

// SavedMessage is a bookmarked message.
type SavedMessage struct {
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	MessageID uuid.UUID `gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time
}

func (SavedMessage) TableName() string { return "saved_messages" }

// EarningSource identifies what produced an earning.
type EarningSource string

// Earning sources and kinds.
const (
	EarningChatSession   EarningSource = "chat_session"
	EarningCustomRequest EarningSource = "custom_request"
	EarningKindEarning                 = "earning"
	EarningKindReversal                = "reversal"
)

// Earning is an immutable psychic earning record (BR-08).
type Earning struct {
	ID                uuid.UUID     `gorm:"type:uuid;primaryKey"`
	PsychicID         uuid.UUID     `gorm:"type:uuid;not null"`
	SourceType        EarningSource `gorm:"not null"`
	SourceID          uuid.UUID     `gorm:"type:uuid;not null"`
	EntryKind         string        `gorm:"not null"`
	GrossCredits      int64         `gorm:"not null"`
	CommissionCredits int64         `gorm:"not null"`
	NetCredits        int64         `gorm:"not null"`
	SharePercent      int           `gorm:"not null"`
	CreatedAt         time.Time
}

func (Earning) TableName() string { return "psychic_earnings" }

// SplitEarnings divides gross credits between psychic (net) and platform
// (commission). The psychic share is rounded down so the platform never pays
// out more than was charged.
func SplitEarnings(gross int64, sharePercent int) (net, commission int64) {
	if gross <= 0 {
		return 0, 0
	}
	net = gross * int64(sharePercent) / 100
	return net, gross - net
}

// AnalyticsEvent is a business analytics record.
type AnalyticsEvent struct {
	ID          int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	EventID     uuid.UUID  `gorm:"type:uuid;not null" json:"event_id"`
	Name        string     `gorm:"not null" json:"name"`
	Source      string     `gorm:"not null" json:"source"`
	UserID      *uuid.UUID `gorm:"type:uuid" json:"user_id,omitempty"`
	UserRole    string     `gorm:"not null" json:"user_role"`
	AnonymousID string     `gorm:"not null" json:"anonymous_id"`
	SessionKey  string     `gorm:"not null" json:"session_key"`
	OccurredAt  time.Time  `gorm:"not null" json:"occurred_at"`
	ReceivedAt  time.Time  `gorm:"not null" json:"received_at"`
	Properties  JSONMap    `gorm:"type:jsonb;not null" json:"properties"`
	Path        string     `gorm:"not null" json:"path"`
	Referrer    string     `gorm:"not null" json:"referrer"`
	Locale      string     `gorm:"not null" json:"locale"`
	UserAgent   string     `gorm:"not null" json:"user_agent"`
	IPHash      string     `gorm:"not null" json:"-"`
	RequestID   string     `gorm:"not null" json:"request_id"`
}

func (AnalyticsEvent) TableName() string { return "analytics_events" }

// AuditLog records privileged actions.
type AuditLog struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ActorID    *uuid.UUID `gorm:"type:uuid"`
	Action     string     `gorm:"not null"`
	TargetType string     `gorm:"not null"`
	TargetID   string     `gorm:"not null"`
	Metadata   JSONMap    `gorm:"type:jsonb;not null"`
	IPHash     string     `gorm:"not null"`
	RequestID  string     `gorm:"not null"`
	CreatedAt  time.Time
}

func (AuditLog) TableName() string { return "audit_logs" }

// NewID returns a time-ordered UUIDv7 (index-friendly primary keys).
func NewID() uuid.UUID {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.New()
	}
	return id
}
