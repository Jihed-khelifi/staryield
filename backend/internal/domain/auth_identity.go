package domain

import (
	"time"

	"github.com/google/uuid"
)

// Identity providers (user_identities.provider).
const ProviderGoogle = "google"

// UserIdentity links an account to an external sign-in provider (revisions
// 4.0: Google). Subject is the provider's stable account id: it, never the
// e-mail, identifies the person. Email is the address the provider reported
// when the identity was linked or last used (display only).
type UserIdentity struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID      uuid.UUID `gorm:"type:uuid;not null"`
	Provider    string    `gorm:"not null"`
	Subject     string    `gorm:"not null"`
	Email       string    `gorm:"not null"`
	LastLoginAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TableName binds the model to its table.
func (UserIdentity) TableName() string { return "user_identities" }

// HasPassword reports whether the account can sign in with a password. An
// account created with Google has none until its owner sets one through the
// password reset flow (users.password_hash is ”).
func (u *User) HasPassword() bool { return u.PasswordHash != "" }
