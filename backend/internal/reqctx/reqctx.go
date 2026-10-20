// Package reqctx carries per-request metadata (request ID, client IP, user
// agent) and the authenticated principal through context.Context.
package reqctx

import (
	"context"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

type metaKey struct{}
type principalKey struct{}

// Meta describes the inbound request.
type Meta struct {
	RequestID string
	IP        string
	UserAgent string
	Locale    string
	Path      string
	Referrer  string
}

// Principal is the authenticated caller.
type Principal struct {
	UserID   uuid.UUID
	Role     domain.Role
	FamilyID uuid.UUID
}

// IsAdmin reports whether the principal is an administrator.
func (p Principal) IsAdmin() bool { return p.Role == domain.RoleAdmin }

// WithMeta stores m in ctx.
func WithMeta(ctx context.Context, m Meta) context.Context {
	return context.WithValue(ctx, metaKey{}, m)
}

// MetaFrom returns the request metadata (zero value when absent).
func MetaFrom(ctx context.Context) Meta {
	m, _ := ctx.Value(metaKey{}).(Meta)
	return m
}

// WithPrincipal stores p in ctx.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom returns the principal, if authenticated.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok && p.UserID != uuid.Nil
}
