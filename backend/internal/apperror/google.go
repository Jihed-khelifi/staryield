package apperror

import (
	"net/http"
	"time"
)

// Sign in with Google and password-less accounts (revisions 4.0, §23).
const (
	// CodeGoogleUnavailable: Google sign-in is not configured, or Google
	// could not be reached. Retryable.
	CodeGoogleUnavailable Code = "google_unavailable"
	// CodeGoogleStateInvalid: the sign-in attempt expired, was already used
	// or does not belong to this browser.
	CodeGoogleStateInvalid Code = "google_state_invalid"
	// CodeGoogleCancelled: the person closed or refused Google's screen.
	CodeGoogleCancelled Code = "google_cancelled"
	// CodeGoogleEmailUnverified: Google reports the address as unverified.
	CodeGoogleEmailUnverified Code = "google_email_unverified"
	// CodeGoogleAccountInUse: that Google account is connected elsewhere.
	CodeGoogleAccountInUse Code = "google_account_in_use"
	// CodeGoogleNotAllowed: Google sign-in is not available for the account.
	CodeGoogleNotAllowed Code = "google_not_allowed"
	// CodePasswordNotSet: the action needs the account password and the
	// account has none yet (it was created with Google).
	CodePasswordNotSet Code = "password_not_set"
	// CodeLastSignInMethod: Google cannot be disconnected before the
	// account has a password.
	CodeLastSignInMethod Code = "last_sign_in_method"
)

// GoogleUnavailable answers when Google sign-in is off or Google failed.
func GoogleUnavailable(cause error) *Error {
	return &Error{Code: CodeGoogleUnavailable, Status: http.StatusServiceUnavailable,
		Message: "Google sign-in is not available right now. Please try again.", Err: cause, RetryAfter: 5 * time.Second}
}

// GoogleStateInvalid reports an expired, reused or foreign sign-in attempt.
func GoogleStateInvalid() *Error {
	return New(CodeGoogleStateInvalid, http.StatusBadRequest, "This sign-in attempt expired or was already used. Please start again.")
}

// GoogleCancelled reports a refused or closed Google screen.
func GoogleCancelled() *Error {
	return New(CodeGoogleCancelled, http.StatusBadRequest, "Google sign-in was cancelled.")
}

// GoogleEmailUnverified reports an e-mail address Google does not vouch for.
func GoogleEmailUnverified() *Error {
	return New(CodeGoogleEmailUnverified, http.StatusForbidden, "Google reports that this email address is not verified.")
}

// GoogleAccountInUse reports a Google account connected to another account.
func GoogleAccountInUse() *Error {
	return New(CodeGoogleAccountInUse, http.StatusConflict, "This Google account is already connected to a Staryield account.")
}

// GoogleNotAllowed reports an account that may not use Google (admins).
func GoogleNotAllowed() *Error {
	return New(CodeGoogleNotAllowed, http.StatusForbidden, "Google sign-in is not available for this account.")
}

// PasswordNotSet answers a password-gated action on an account without one.
func PasswordNotSet() *Error {
	return New(CodePasswordNotSet, http.StatusConflict, "This action needs a password, and your account does not have one yet.")
}

// LastSignInMethod refuses to remove the only way to sign in.
func LastSignInMethod() *Error {
	return New(CodeLastSignInMethod, http.StatusConflict, "Create a password before disconnecting Google.")
}
