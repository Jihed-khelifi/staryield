package domain

import (
	"time"

	"github.com/google/uuid"
)

// Reply excerpt and client alias limits (revisions 4.0, §23.2 and §23.4).
// Both count characters (runes), not bytes.
const (
	ReplyExcerptChars   = 140
	ClientAliasMaxChars = 40
)

// MessageLike is one participant's like of a message. A message has at most
// two (the conversation's participants).
type MessageLike struct {
	MessageID uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time
}

// TableName binds the model to its table.
func (MessageLike) TableName() string { return "message_likes" }

// ClientAlias is an advisor's private nickname for a client. The alias is
// sealed at rest (see service.ChatService); it is only ever returned to the
// advisor who wrote it.
type ClientAlias struct {
	PsychicID uuid.UUID `gorm:"type:uuid;primaryKey"`
	ClientID  uuid.UUID `gorm:"type:uuid;primaryKey"`
	AliasEnc  string    `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName binds the model to its table.
func (ClientAlias) TableName() string { return "client_aliases" }
