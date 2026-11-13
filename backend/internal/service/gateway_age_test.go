package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// revisions 5.0: 18+ age gate over the WebSocket (§24.6).
func TestGatewayRequireAdult(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	status := domain.AgeBirthDateRequired
	calls := 0
	g := &RealtimeGateway{}
	g.SetAgeStatus(func(_ context.Context, got uuid.UUID) (domain.AgeStatus, error) {
		calls++
		if got != id {
			t.Errorf("looked up %s", got)
		}
		return status, nil
	})
	client := reqctx.Principal{UserID: id, Role: domain.RoleClient}

	if errCode(g.requireAdult(ctx, client)) != apperror.CodeBirthDateRequired {
		t.Fatal("a client without a date must be refused")
	}
	status = domain.AgeRestricted
	if errCode(g.requireAdult(ctx, client)) != apperror.CodeAgeRestricted {
		t.Fatal("a restricted client must be refused")
	}
	// The status is read on every frame, so a change applies at once.
	status = domain.AgeAdult
	if err := g.requireAdult(ctx, client); err != nil {
		t.Fatalf("an adult client: %v", err)
	}
	// Advisors are never looked up.
	before := calls
	if err := g.requireAdult(ctx, reqctx.Principal{UserID: id, Role: domain.RolePsychic}); err != nil || calls != before {
		t.Fatalf("advisor: %v (lookups %d)", err, calls-before)
	}
	// A failing lookup refuses rather than letting the frame through.
	g.SetAgeStatus(func(context.Context, uuid.UUID) (domain.AgeStatus, error) { return "", errors.New("cache down") })
	if err := g.requireAdult(ctx, client); err == nil {
		t.Fatal("a failed lookup must not allow the frame")
	}
	// Without a lookup (unit tests of other services) the gate is off.
	if err := (&RealtimeGateway{}).requireAdult(ctx, client); err != nil {
		t.Fatalf("no lookup installed: %v", err)
	}
}
