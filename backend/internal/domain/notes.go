package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Client note limits. NoteMaxChars counts characters (runes), not bytes.
const (
	NoteMaxChars = 2000
	// NoteMaxPerClient caps how many live notes one advisor keeps about one
	// client, which bounds storage and abuse of the feature.
	NoteMaxPerClient = 500
)

// ClientNote is an advisor's note about a client. The body is sealed at rest
// (see service.NoteService); only the author can edit or delete it, and other
// advisors who met the client can read it when Shared is set.
type ClientNote struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey"`
	ClientID       uuid.UUID      `gorm:"type:uuid;not null"`
	AuthorID       uuid.UUID      `gorm:"type:uuid;not null"`
	ConversationID *uuid.UUID     `gorm:"type:uuid"`
	BodyEnc        string         `gorm:"not null"`
	BodyChars      int            `gorm:"not null"`
	Shared         bool           `gorm:"not null"`
	CreatedAt      time.Time      `gorm:"not null"`
	UpdatedAt      time.Time      `gorm:"not null"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

// TableName binds the model to its table.
func (ClientNote) TableName() string { return "client_notes" }
