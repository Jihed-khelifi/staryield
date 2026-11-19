package apperror

import "net/http"

// Free-trial hold (revisions 6.0, §26.1).
const (
	// CodeMessageLocked: the message is hidden from the client by a hold and
	// can be neither liked nor saved until it is revealed.
	CodeMessageLocked Code = "message_locked"
)

// MessageLocked refuses a like or a save of a message a hold hides.
func MessageLocked() *Error {
	return New(CodeMessageLocked, http.StatusConflict, "This message is hidden until you top up.")
}

// InsufficientCreditsFor answers a payment that needs required credits when
// the wallet holds balance. The details carry the balance as "balance"
// (§26.9) and, like InsufficientCredits, as "available".
func InsufficientCreditsFor(required, balance int64) *Error {
	e := InsufficientCredits(required, balance)
	e.Details["balance"] = balance
	return e
}
