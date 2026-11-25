package service

import (
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// UserListOption narrows the admin user listing.
type UserListOption func(*repository.UserFilter)

// VerifiedFilter parses the verified=true|false query parameter of the admin
// user list ("" keeps every account).
func VerifiedFilter(raw string) (UserListOption, error) {
	switch raw {
	case "":
		return func(*repository.UserFilter) {}, nil
	case "true", "false":
		v := raw == "true"
		return func(f *repository.UserFilter) { f.Verified = &v }, nil
	}
	return nil, apperror.Validation(map[string]string{"verified": "Use true or false."})
}
