package domain

import (
	"time"

	"github.com/google/uuid"
)

// PresenceInterval is one stretch of availability (PresenceOnline, PresenceBusy
// or PresenceAway). EndedAt is nil while the interval is open (at most one
// open interval per advisor).
type PresenceInterval struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey"`
	PsychicID uuid.UUID      `gorm:"type:uuid;not null"`
	Status    PresenceStatus `gorm:"not null"`
	StartedAt time.Time      `gorm:"not null"`
	EndedAt   *time.Time
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

// TableName binds the model to its table.
func (PresenceInterval) TableName() string { return "psychic_presence_intervals" }
