package seed

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// AdminResult says what EnsureAdmin did.
type AdminResult string

const (
	// AdminCreated means a new administrator account was inserted.
	AdminCreated AdminResult = "created"
	// AdminPasswordReset means an existing administrator got a new password
	// and was signed out everywhere.
	AdminPasswordReset AdminResult = "password_reset"
)

// ErrNotAdmin is returned when the e-mail belongs to a client or advisor:
// the bootstrap never promotes an existing account.
var ErrNotAdmin = errors.New("the account exists and is not an administrator; refusing to promote it")

// ErrAdminInactive is returned for a banned, deactivated or deleted
// administrator: reactivating one is a decision for the admin console.
var ErrAdminInactive = errors.New("the administrator account is not active; refusing to change it")

// EnsureAdmin bootstraps the administrator account and touches nothing else
// (no advisors, credit packs or demo data). A new e-mail creates the account;
// an existing administrator gets the new password, and its refresh tokens and
// admin console sessions are revoked so the old password stops working
// everywhere. TOTP enrolment, when already done, is kept; otherwise it starts
// at the first console sign-in. The password policy applies, and the action is
// written to the audit log (actor: none, i.e. the command line).
func EnsureAdmin(ctx context.Context, db *gorm.DB, hasher *security.PasswordHasher, acct Account, log *zap.Logger) (AdminResult, error) {
	email := strings.ToLower(strings.TrimSpace(acct.Email))
	if at := strings.Index(email, "@"); at < 1 || at == len(email)-1 || strings.ContainsAny(email, " \t") {
		return "", errors.New("admin e-mail must be a valid address")
	}
	if msg := security.ValidatePassword(acct.Password, email); msg != "" {
		return "", fmt.Errorf("admin password: %s", msg)
	}
	hash, err := hasher.Hash(acct.Password)
	if err != nil {
		return "", err
	}

	users := repository.NewUserRepository(db)
	wallets := repository.NewWalletRepository(db)
	tokens := repository.NewTokenRepository(db)
	admins := repository.NewAdminRepository(db)
	audit := repository.NewAuditRepository(db)
	now := time.Now().UTC()

	var result AdminResult
	err = database.NewTxManager(db).WithinTx(ctx, func(ctx context.Context) error {
		u, err := users.GetByEmail(ctx, email)
		switch {
		case errors.Is(err, repository.ErrNotFound):
			if u, err = createAccount(ctx, users, wallets, email, hash, domain.RoleAdmin, "Administrator", 0, now); err != nil {
				return err
			}
			result = AdminCreated
		case err != nil:
			return err
		case u.Role != domain.RoleAdmin:
			return fmt.Errorf("%w (role %s)", ErrNotAdmin, u.Role)
		case u.Status != domain.UserActive:
			return fmt.Errorf("%w (status %s)", ErrAdminInactive, u.Status)
		default:
			if err := users.Updates(ctx, u.ID, map[string]any{"password_hash": hash, "token_version": gorm.Expr("token_version + 1"), "updated_at": now}); err != nil {
				return err
			}
			if _, err := tokens.RevokeAllForUser(ctx, u.ID, "admin_bootstrap", now); err != nil {
				return err
			}
			if _, err := admins.RevokeUserSessions(ctx, u.ID, "admin_bootstrap", now); err != nil {
				return err
			}
			result = AdminPasswordReset
		}
		return audit.Create(ctx, &domain.AuditLog{ID: domain.NewID(), Action: "admin.bootstrap_" + string(result), TargetType: "user",
			TargetID: u.ID.String(), Metadata: domain.JSONMap{"source": "seed -admin-only"}, CreatedAt: now})
	})
	if err != nil {
		return "", fmt.Errorf("admin bootstrap: %w", err)
	}
	log.Info("admin account ready", zap.String("email", email), zap.String("result", string(result)))
	return result, nil
}
