// Package service is the business-logic layer. Services own every rule of
// the specification (state machines, billing, holds, permissions) and are
// the server-side authority; transports only translate requests into calls.
// Dependencies are injected explicitly through constructors.
package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// EventPublisher emits realtime events (implemented by realtime.Publisher).
type EventPublisher interface {
	Publish(ctx context.Context, eventType string, data any, userIDs ...uuid.UUID) error
	Ephemeral(ctx context.Context, eventType string, data any, userIDs ...uuid.UUID) error
	Broadcast(ctx context.Context, eventType string, data any) error
	Revoke(ctx context.Context, userID uuid.UUID) error
}

// NoopPublisher discards events (tests that do not observe realtime).
type NoopPublisher struct{}

func (NoopPublisher) Publish(context.Context, string, any, ...uuid.UUID) error   { return nil }
func (NoopPublisher) Ephemeral(context.Context, string, any, ...uuid.UUID) error { return nil }
func (NoopPublisher) Broadcast(context.Context, string, any) error               { return nil }
func (NoopPublisher) Revoke(context.Context, uuid.UUID) error                    { return nil }

// Page is a cursor-paginated list.
type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// afterCommit collects side effects (events, analytics) that must only run
// once the surrounding transaction has committed.
// Hooks receive a context detached from the (committed) transaction, so they
// can never query through a finished tx by capturing the closure's context.
type afterCommit []func(ctx context.Context)

func (a *afterCommit) add(fn func(ctx context.Context)) { *a = append(*a, fn) }

func (a afterCommit) run(ctx context.Context) {
	ctx = database.Detach(ctx)
	for _, fn := range a {
		fn(ctx)
	}
}

// decodeCursor maps an invalid cursor to a validation error.
func decodeCursor(s string) (*repository.Cursor, error) {
	c, err := repository.DecodeCursor(s)
	if err != nil {
		return nil, apperror.Validation(map[string]string{"cursor": "Invalid pagination cursor."})
	}
	return c, nil
}

// mapRepoErr converts repository sentinels into application errors.
func mapRepoErr(err error, resource string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrNotFound):
		return apperror.NotFound(resource)
	case errors.Is(err, context.DeadlineExceeded):
		return apperror.Timeout(err)
	case errors.Is(err, context.Canceled):
		return err
	}
	if _, ok := apperror.As(err); ok {
		return err
	}
	return apperror.Internal(err)
}

// warnErr logs a non-fatal failure of a side effect.
func warnErr(ctx context.Context, msg string, err error, fields ...zap.Field) {
	if err == nil {
		return
	}
	logger.FromContext(ctx).Warn(msg, append(fields, zap.Error(err))...)
}

// idempotencyKey validates client-supplied idempotency keys.
var idempotencyKey = regexp.MustCompile(`^[A-Za-z0-9_\-:.]{8,64}$`)

// ValidIdempotencyKey reports whether key is acceptable.
func ValidIdempotencyKey(key string) bool { return idempotencyKey.MatchString(key) }

// CleanText normalises user-entered text: valid UTF-8, CRLF→LF, control
// characters removed (except newline and tab), surrounding space trimmed,
// and at most two consecutive blank lines. Output is stored as plain text;
// clients must render it as text, never as HTML.
func CleanText(s string) string {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "")
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	var b strings.Builder
	b.Grow(len(s))
	newlines := 0
	for _, r := range s {
		switch {
		case r == '\n':
			newlines++
			if newlines > 3 {
				continue
			}
		case r == '\t':
			newlines = 0
		case unicode.IsControl(r), r == '\u200b', r == '\u202e', r == '\u202d', r == '\ufeff':
			continue
		default:
			newlines = 0
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// CleanLine is CleanText for single-line fields (newlines become spaces).
func CleanLine(s string) string {
	s = CleanText(s)
	s = strings.Join(strings.Fields(s), " ")
	return s
}

// ptr returns a pointer to v.
func ptr[T any](v T) *T { return &v }

// timeOrNil returns nil for the zero time.
func timeOrNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
