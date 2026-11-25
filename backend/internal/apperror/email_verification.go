package apperror

import (
	"net/http"
	"time"
)

// E-mail verification (growth: e-mail verification).
const (
	// CodeAlreadyVerified: the address of the account is already verified,
	// so there is nothing to send.
	CodeAlreadyVerified Code = "already_verified"
	// CodeEmailUndeliverable: the address stopped accepting e-mail (a
	// permanent bounce or a complaint), so no link can reach it.
	CodeEmailUndeliverable Code = "email_undeliverable"
)

// AlreadyVerified reports a resend request for an address that is verified.
func AlreadyVerified() *Error {
	return New(CodeAlreadyVerified, http.StatusConflict, "Your email address is already verified.")
}

// EmailUndeliverable reports a link that cannot be delivered: the address is
// on the suppression list.
func EmailUndeliverable() *Error {
	return New(CodeEmailUndeliverable, http.StatusConflict,
		"We can't deliver email to this address: earlier messages bounced or were refused. Check the address or change it in your account settings.")
}

// VerificationDailyLimit is the 429 once an address received the most
// verification links allowed in 24 hours.
func VerificationDailyLimit(retryAfter time.Duration) *Error {
	e := New(CodeRateLimited, http.StatusTooManyRequests,
		"This address has received the maximum number of verification emails for today. Please try again later.")
	e.RetryAfter = retryAfter
	return e
}
