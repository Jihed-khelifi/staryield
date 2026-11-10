package service

import (
	"context"

	"github.com/google/uuid"
)

// sessionPush removes browser push subscriptions when a session ends
// (implemented by *PushService).
type sessionPush interface {
	RemoveSubscription(ctx context.Context, userID uuid.UUID, endpoint string)
	RemoveAllForUser(ctx context.Context, userID uuid.UUID)
}

// SetPush wires push cleanup: signing out removes this browser's
// subscription, and ending every session (logout everywhere, password reset
// or change, deactivation, closure, ban) removes them all, so a shared device
// stops receiving the previous person's notifications.
func (s *AuthService) SetPush(p sessionPush) { s.push = p }

func (s *AuthService) endPushSubscription(ctx context.Context, userID uuid.UUID, endpoint string) {
	if s.push != nil && endpoint != "" {
		s.push.RemoveSubscription(ctx, userID, endpoint)
	}
}

// endPushSessions removes every push subscription of a user.
func (s *AuthService) endPushSessions(ctx context.Context, userID uuid.UUID) {
	if s.push != nil {
		s.push.RemoveAllForUser(ctx, userID)
	}
}

// Logout signs this device out (see logout).
func (s *AuthService) Logout(ctx context.Context, raw string) error { return s.logout(ctx, raw, "") }

// LogoutDevice signs this device out and removes the push subscription for
// pushEndpoint when it belongs to the signed-out user.
func (s *AuthService) LogoutDevice(ctx context.Context, raw, pushEndpoint string) error {
	return s.logout(ctx, raw, pushEndpoint)
}
