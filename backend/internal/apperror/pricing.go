package apperror

import "net/http"

// revisions 5.0: welcome offer (§24.2).

// CodeOfferUnavailable: the welcome offer can't be bought; details.reason
// says why (OfferReason*).
const CodeOfferUnavailable Code = "offer_unavailable"

// details.reason values of offer_unavailable.
const (
	OfferReasonAlreadyRedeemed = "already_redeemed"
	OfferReasonNotNewClient    = "not_new_client"
	OfferReasonPaymentUsed     = "payment_method_used"
	OfferReasonCardCheck       = "card_check_unavailable"
	OfferReasonNoOffer         = "no_offer"
)

// OfferUnavailable refuses a welcome-offer purchase. The message is chosen
// by reason (translated at the API edge); an unknown reason reads as no offer.
func OfferUnavailable(reason string) *Error {
	var msg string
	switch reason {
	case OfferReasonAlreadyRedeemed:
		msg = "The welcome offer has already been used on this account."
	case OfferReasonNotNewClient:
		msg = "The welcome offer is for new clients only."
	case OfferReasonPaymentUsed:
		msg = "The welcome offer has already been used with this payment method."
	case OfferReasonCardCheck:
		msg = "We couldn't verify this card for the welcome offer. Choose another pack or try another card."
	default:
		reason = OfferReasonNoOffer
		msg = "The welcome offer isn't available right now."
	}
	return New(CodeOfferUnavailable, http.StatusConflict, msg).WithDetails(map[string]any{"reason": reason})
}
