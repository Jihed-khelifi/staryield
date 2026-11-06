package domain

import (
	"time"

	"gorm.io/gorm"
)

// Timestamps are the audit columns every mutable table carries. GORM fills
// them on create/update from the database clock function (the application
// clock, so tests with a fake clock stay deterministic); the
// touch_updated_at trigger keeps updated_at current for raw updates.
type Timestamps struct {
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SoftDeleteTimestamps adds GORM-managed soft deletion: Delete sets
// deleted_at and every query through the model filters deleted rows.
type SoftDeleteTimestamps struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt
}
