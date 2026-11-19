package apperror

import (
	"net/http"
	"time"
)

// revisions 6.0 (docs/platform-extensions.md §26.5, §26.9): card payment operations.

// Payment operation error codes.
const (
	// CodePurchasesBlocked: a payment dispute is in progress, so new purchases
	// are paused (details.reason "dispute").
	CodePurchasesBlocked Code = "purchases_blocked"
	// CodeRefundDisputed: the payment is disputed; the dispute must be
	// answered (or accepted) instead of refunding.
	CodeRefundDisputed Code = "refund_disputed"
	// CodeProviderUnavailable: the payment processor did not answer; retry.
	CodeProviderUnavailable Code = "provider_unavailable"
)

// PurchasesBlockedReasonDispute is details.reason of purchases_blocked.
const PurchasesBlockedReasonDispute = "dispute"

// PurchasesBlocked refuses a purchase while the account has a payment dispute
// in progress.
func PurchasesBlocked() *Error {
	return New(CodePurchasesBlocked, http.StatusForbidden,
		"Your account has a payment dispute in progress. Contact support.").
		WithDetails(map[string]any{"reason": PurchasesBlockedReasonDispute})
}

// RefundDisputed refuses a refund of a disputed payment.
func RefundDisputed() *Error {
	return New(CodeRefundDisputed, http.StatusConflict,
		"This payment is disputed. Answer the dispute instead of refunding it.")
}

// ProviderUnavailable reports that the payment processor did not answer.
func ProviderUnavailable(cause error) *Error {
	return &Error{Code: CodeProviderUnavailable, Status: http.StatusServiceUnavailable,
		Message: "The payment provider did not answer. Please try again.", Err: cause, RetryAfter: 5 * time.Second}
}
