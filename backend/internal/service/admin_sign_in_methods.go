package service

import (
	"context"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// SetIdentities lets the user page show how an account signs in (revisions
// 4.0: Sign in with Google). Without it only the password is reported.
func (s *AdminService) SetIdentities(r repository.IdentityRepository) { s.identities = r }

// signInMethods reports an account's sign-in methods.
func (s *AdminService) signInMethods(ctx context.Context, u *domain.User) []string {
	if u.Status == domain.UserDeleted {
		return []string{} // erased: the placeholder hash is not a password
	}
	var idents []domain.UserIdentity
	if s.identities != nil {
		var err error
		if idents, err = s.identities.ListForUser(ctx, u.ID); err != nil {
			warnErr(ctx, "admin: sign-in methods", err)
		}
	}
	return SignInMethodNames(u.HasPassword(), idents)
}
