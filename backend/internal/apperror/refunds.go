package apperror

import "net/http"

// revisions 5.0: refund requests (§24.4).

// Refund error codes.
const (
	// CodeRefundRequestExists: an open request already exists for this payment
	// (details.request_id).
	CodeRefundRequestExists Code = "refund_request_exists"
	// CodeRefundNotEligible: this payment cannot be refunded through a
	// request; details.reason says why (RefundReason*).
	CodeRefundNotEligible Code = "refund_not_eligible"
	// CodeRefundRefused: the payment provider definitively refused a refund
	// (administrators only). The money has not moved: it must be refunded from
	// the provider's dashboard.
	CodeRefundRefused Code = "refund_refused"
)

// details.reason values of refund_not_eligible.
const (
	RefundReasonNotCreditPurchase = "not_credit_purchase"
	RefundReasonNotPaid           = "not_paid"
	RefundReasonFullyRefunded     = "fully_refunded"
	RefundReasonOutsideWindow     = "outside_window"
)

// RefundRequestExists reports the open request of a payment.
func RefundRequestExists(requestID string) *Error {
	return New(CodeRefundRequestExists, http.StatusConflict, "A refund request for this payment is already open.").
		WithDetails(map[string]any{"request_id": requestID})
}

// RefundNotEligible refuses a request; the message is chosen by reason (an
// unknown reason reads as not eligible without detail).
func RefundNotEligible(reason string) *Error {
	var msg string
	switch reason {
	case RefundReasonNotCreditPurchase:
		msg = "Only credit pack payments can be refunded this way."
	case RefundReasonNotPaid:
		msg = "This payment was not completed, so there is nothing to refund."
	case RefundReasonFullyRefunded:
		msg = "This payment has already been refunded in full."
	case RefundReasonOutsideWindow:
		msg = "The refund window for unused credits has ended for this payment."
	default:
		msg = "This payment is not eligible for a refund request."
	}
	return New(CodeRefundNotEligible, http.StatusConflict, msg).WithDetails(map[string]any{"reason": reason})
}

// RefundRefused reports a definitive refusal by the payment provider.
func RefundRefused() *Error {
	return New(CodeRefundRefused, http.StatusConflict,
		"The provider refused this refund. Refund it from the provider's dashboard.")
}
