package apperror

import (
	"net/http"
	"time"
)

// Account self-service error codes (settings, devices, exports, closure).
const (
	// CodeVersionConflict: the settings changed since the client loaded
	// them (another tab or device saved first).
	CodeVersionConflict Code = "version_conflict"
	// CodeAccountDeactivated: the owner paused the account; the response
	// carries a short-lived reactivation token.
	CodeAccountDeactivated Code = "account_deactivated"
	// CodeClosurePending: the account is scheduled for closure; signing in
	// again offers to keep it (same reactivation token).
	CodeClosurePending Code = "account_closure_pending"
	// CodeClosureBlocked: the account cannot be closed yet (details.reasons).
	CodeClosureBlocked Code = "closure_blocked"
	// CodeExportInProgress: a data export is already being prepared.
	CodeExportInProgress Code = "export_in_progress"
)

// VersionConflict reports a stale expected_version.
func VersionConflict(current int) *Error {
	return New(CodeVersionConflict, http.StatusConflict, "Your settings were changed elsewhere. Reload them and try again.").
		WithDetails(map[string]any{"current_version": current})
}

// AccountDeactivated answers a sign-in on a self-deactivated account.
func AccountDeactivated(token string, expiresAt time.Time) *Error {
	return New(CodeAccountDeactivated, http.StatusForbidden, "This account is deactivated. Reactivate it to continue.").
		WithDetails(map[string]any{"reactivation_token": token, "reactivation_expires_at": expiresAt})
}

// ClosurePending answers a sign-in on an account scheduled for closure.
func ClosurePending(token string, expiresAt, deletionAt time.Time) *Error {
	return New(CodeClosurePending, http.StatusForbidden, "This account is scheduled for closure. Keep it to continue.").
		WithDetails(map[string]any{"reactivation_token": token, "reactivation_expires_at": expiresAt, "deletion_scheduled_at": deletionAt})
}

// ClosureBlocked lists why an account cannot be closed yet.
func ClosureBlocked(reasons []string) *Error {
	return New(CodeClosureBlocked, http.StatusConflict, "Your account cannot be closed yet.").
		WithDetails(map[string]any{"reasons": reasons})
}

// ExportInProgress reports an export that is still being prepared.
func ExportInProgress() *Error {
	return New(CodeExportInProgress, http.StatusConflict, "Your previous export is still being prepared.")
}
