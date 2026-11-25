package apperror

import "net/http"

// E-mail verification (growth: e-mail verification).
const (
	// CodeAlreadyVerified: the address of the account is already verified,
	// so there is nothing to send.
	CodeAlreadyVerified Code = "already_verified"
)

// AlreadyVerified reports a resend request for an address that is verified.
func AlreadyVerified() *Error {
	return New(CodeAlreadyVerified, http.StatusConflict, "Your email address is already verified.")
}
