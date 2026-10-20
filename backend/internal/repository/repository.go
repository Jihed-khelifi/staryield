// Package repository is the data layer: GORM-backed persistence for every
// aggregate. All methods take a context.Context and run through
// database.Conn, so they honour cancellation/deadlines and join the
// transaction a service opened with database.TxManager.
package repository

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
)

// Sentinel errors returned by repositories.
var (
	ErrNotFound      = errors.New("repository: not found")
	ErrDuplicate     = errors.New("repository: duplicate")
	ErrInvalidCursor = errors.New("repository: invalid cursor")
)

// notFound converts gorm.ErrRecordNotFound into ErrNotFound.
func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

// forUpdate is the SELECT ... FOR UPDATE row lock clause.
var forUpdate = clause.Locking{Strength: "UPDATE"}

// forUpdateSkipLocked lets concurrent workers claim disjoint rows.
var forUpdateSkipLocked = clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}

// Cursor is a keyset pagination position (created_at DESC, id DESC).
type Cursor struct {
	At time.Time `json:"t"`
	ID string    `json:"i"`
}

// EncodeCursor returns an opaque cursor string.
func EncodeCursor(at time.Time, id string) string {
	b, _ := json.Marshal(Cursor{At: at.UTC(), ID: id})
	return base64.RawURLEncoding.EncodeToString(b)
}

// DecodeCursor parses an opaque cursor. Empty input yields nil.
func DecodeCursor(s string) (*Cursor, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if len(s) > 256 {
		return nil, ErrInvalidCursor
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, ErrInvalidCursor
	}
	var c Cursor
	if err := json.Unmarshal(b, &c); err != nil || c.At.IsZero() || c.ID == "" {
		return nil, ErrInvalidCursor
	}
	return &c, nil
}

// clampLimit bounds page sizes.
func clampLimit(limit, def, max int) int {
	if limit <= 0 {
		return def
	}
	if limit > max {
		return max
	}
	return limit
}

// applyCursor adds the keyset predicate for DESC ordering on (col, id).
func applyCursor(q *gorm.DB, c *Cursor, timeCol, idCol string) *gorm.DB {
	if c == nil {
		return q
	}
	if id, err := uuid.Parse(c.ID); err == nil {
		return q.Where("("+timeCol+", "+idCol+") < (?, ?)", c.At, id)
	}
	return q.Where(timeCol+" < ?", c.At)
}

// isUnique reports a unique violation (optionally on named constraints).
func isUnique(err error, constraints ...string) bool {
	return database.IsUniqueViolation(err, constraints...)
}
