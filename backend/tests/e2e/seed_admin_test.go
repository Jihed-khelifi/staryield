package e2e

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/seed"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// `seed -admin-only` bootstraps the administrator on a production database:
// it creates the account (and nothing else), the account signs in to the
// console with TOTP enrolment, and re-running it resets the password and
// signs the administrator out everywhere.
func TestAdminOnlySeedBootstrapsTheAdministrator(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	hasher := security.NewPasswordHasher(1024, 1, 1)
	count := func(table string) int64 {
		var n int64
		if err := e.app.DB.Raw("SELECT count(*) FROM " + table).Scan(&n).Error; err != nil {
			t.Fatal(err)
		}
		return n
	}
	psychics, packs, users := count("psychics"), count("credit_packs"), count("users")

	email := uniq("Owner") + "@Example.com"
	res, err := seed.EnsureAdmin(ctx, e.app.DB, hasher, seed.Account{Email: "  " + email + " ", Password: strongPassword}, zap.NewNop())
	if err != nil || res != seed.AdminCreated {
		t.Fatalf("create: %v %v", res, err)
	}
	// Only the administrator was added: no advisors, packs or demo accounts.
	if count("psychics") != psychics || count("credit_packs") != packs || count("users") != users+1 {
		t.Fatal("admin-only seed created more than the administrator")
	}
	var u domain.User
	if err := e.app.DB.First(&u, "email = ?", strings.ToLower(email)).Error; err != nil {
		t.Fatalf("stored e-mail is not normalised: %v", err)
	}
	if u.Role != domain.RoleAdmin || u.Status != domain.UserActive {
		t.Fatalf("account: role=%s status=%s", u.Role, u.Status)
	}
	var audit domain.AuditLog
	if err := e.app.DB.First(&audit, "action = ? AND target_id = ?", "admin.bootstrap_created", u.ID.String()).Error; err != nil || audit.ActorID != nil {
		t.Fatalf("bootstrap audit entry: %v %+v", err, audit)
	}

	// First console sign-in enrols TOTP, like any new administrator.
	admin := e.enrollAdmin(u.ID, u.Email, strongPassword)
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/auth/session", nil, nil)

	// Re-running with a new password resets it and revokes every session.
	newPassword := "An0ther!Strong#Pass"
	res, err = seed.EnsureAdmin(ctx, e.app.DB, hasher, seed.Account{Email: email, Password: newPassword}, zap.NewNop())
	if err != nil || res != seed.AdminPasswordReset {
		t.Fatalf("reset: %v %v", res, err)
	}
	if r := admin.call(http.MethodGet, "/api/v1/admin/auth/session", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("old admin session still valid: %d", r.status)
	}
	if r := e.adminClient().call(http.MethodPost, "/api/v1/admin/auth/login", map[string]any{"email": email, "password": strongPassword}); r.status != http.StatusUnauthorized {
		t.Fatalf("old password still accepted: %d %s", r.status, r.body)
	}
	// TOTP enrolment is kept: the new password leads to the code step.
	var ch service.LoginChallenge
	e.adminClient().must(http.StatusOK, http.MethodPost, "/api/v1/admin/auth/login", map[string]any{"email": email, "password": newPassword}, &ch)
	if ch.Status != "mfa_required" {
		t.Fatalf("after reset: %+v", ch)
	}
	var resetAudit domain.AuditLog // fresh: GORM would add the previous row's ID
	if err := e.app.DB.First(&resetAudit, "action = ? AND target_id = ?", "admin.bootstrap_password_reset", u.ID.String()).Error; err != nil {
		t.Fatalf("reset audit entry: %v", err)
	}
}

func TestAdminOnlySeedRefusesUnsafeChanges(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	hasher := security.NewPasswordHasher(1024, 1, 1)
	ensure := func(email, password string) error {
		_, err := seed.EnsureAdmin(ctx, e.app.DB, hasher, seed.Account{Email: email, Password: password}, zap.NewNop())
		return err
	}

	// A client is never promoted.
	cl := e.registerClient()
	if err := ensure(cl.user.Email, strongPassword); !errors.Is(err, seed.ErrNotAdmin) {
		t.Fatalf("client promotion: %v", err)
	}
	var role string
	e.app.DB.Raw("SELECT role FROM users WHERE id = ?", cl.user.ID).Scan(&role)
	if role != string(domain.RoleClient) {
		t.Fatalf("client role changed to %s", role)
	}
	e.login(cl.user.Email, cl.pw) // the client's own password still works

	// A banned administrator is not silently reactivated.
	id, email := e.insertAdmin()
	if err := e.app.DB.Exec("UPDATE users SET status = 'banned', status_reason = 'test' WHERE id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	if err := ensure(email, strongPassword); !errors.Is(err, seed.ErrAdminInactive) {
		t.Fatalf("banned admin: %v", err)
	}

	// Password policy and e-mail format apply; nothing is created on failure.
	fresh := uniq("weak") + "@example.com"
	if err := ensure(fresh, "password"); err == nil || !strings.Contains(err.Error(), "admin password") {
		t.Fatalf("weak password: %v", err)
	}
	for _, bad := range []string{"", "no-at-sign", "@example.com", "owner@", "two words@example.com"} {
		if err := ensure(bad, strongPassword); err == nil {
			t.Fatalf("e-mail %q accepted", bad)
		}
	}
	var n int64
	e.app.DB.Raw("SELECT count(*) FROM users WHERE email = ?", fresh).Scan(&n)
	if n != 0 {
		t.Fatal("account created despite a rejected password")
	}
}
