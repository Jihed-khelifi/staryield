package service

import (
	"errors"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// hashError maps a password-hashing failure. Back-pressure from the hasher
// (security.ErrBusy: no slot became free in time) is a retryable 503 that
// says nothing about the password; anything else is internal.
func hashError(err error) error {
	if errors.Is(err, security.ErrBusy) {
		return apperror.Unavailable(err)
	}
	return apperror.Internal(err)
}

// hasherBusy reports whether a verification failed only because the hasher
// was saturated (never a wrong password; not a failed attempt).
func hasherBusy(err error) bool { return errors.Is(err, security.ErrBusy) }
