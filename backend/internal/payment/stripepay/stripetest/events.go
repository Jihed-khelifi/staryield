package stripetest

import (
	"encoding/json"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
)

// APIVersion is the API version the fake's events carry (the SDK's).
const APIVersion = "2026-10-28.endive"

// Sign returns the Stripe-Signature header for payload signed with the
// endpoint secret at time t: "t=<unix>,v1=<hex hmac-sha256(secret, t.payload)>".
func Sign(secret string, payload []byte, t time.Time) string {
	return payment.Sign([]byte(secret), payload, t)
}

// Build returns the JSON of an event of the given type about object. Use it
// for event types without a dedicated builder.
func Build(eventID, eventType string, object map[string]any) []byte {
	b, err := json.Marshal(map[string]any{"id": eventID, "object": "event", "api_version": APIVersion, "created": time.Now().Unix(),
		"livemode": false, "pending_webhooks": 1, "type": eventType, "request": map[string]any{"id": nil, "idempotency_key": nil},
		"data": map[string]any{"object": object}})
	if err != nil {
		panic(err)
	}
	return b
}

func intentObject(piID string, amount int64, currency string, meta map[string]string) map[string]any {
	if meta == nil {
		meta = map[string]string{}
	}
	return map[string]any{"id": piID, "object": "payment_intent", "amount": amount, "amount_received": amount, "currency": currency,
		"metadata": meta, "livemode": false, "latest_charge": "ch_of_" + piID}
}

// PaymentIntentSucceeded is payment_intent.succeeded.
func PaymentIntentSucceeded(eventID, piID string, amount int64, currency string, meta map[string]string) []byte {
	return Build(eventID, "payment_intent.succeeded", intentObject(piID, amount, currency, meta))
}

// PaymentIntentFailed is payment_intent.payment_failed with the issuer's decline code.
func PaymentIntentFailed(eventID, piID string, amount int64, currency string, meta map[string]string, declineCode string) []byte {
	o := intentObject(piID, amount, currency, meta)
	o["amount_received"] = 0
	o["last_payment_error"] = map[string]any{"type": "card_error", "code": "card_declined", "decline_code": declineCode}
	return Build(eventID, "payment_intent.payment_failed", o)
}

// PaymentIntentCanceled is payment_intent.canceled.
func PaymentIntentCanceled(eventID, piID string, amount int64, currency string, meta map[string]string) []byte {
	o := intentObject(piID, amount, currency, meta)
	o["amount_received"] = 0
	o["cancellation_reason"] = "abandoned"
	return Build(eventID, "payment_intent.canceled", o)
}

// PaymentIntentProcessing is payment_intent.processing.
func PaymentIntentProcessing(eventID, piID string, amount int64, currency string, meta map[string]string) []byte {
	o := intentObject(piID, amount, currency, meta)
	o["amount_received"] = 0
	return Build(eventID, "payment_intent.processing", o)
}

// RefundEvent is refund.created, refund.updated or refund.failed.
func RefundEvent(eventID, eventType, refundID, piID string, amount int64, currency, status string) []byte {
	return Build(eventID, eventType, map[string]any{"id": refundID, "object": "refund", "amount": amount, "currency": currency,
		"status": status, "payment_intent": piID, "charge": "ch_of_" + piID, "metadata": map[string]string{}})
}

// DisputeObject describes a dispute for DisputeEvent.
type DisputeObject struct {
	ID            string
	PaymentIntent string
	Amount        int64
	Currency      string
	Reason        string // fraudulent, product_not_received…
	Status        string // needs_response, won, lost, warning_needs_response…
	EvidenceDueBy time.Time
}

// DisputeEvent is charge.dispute.created, .updated, .closed, .funds_withdrawn
// or .funds_reinstated.
func DisputeEvent(eventID, eventType string, d DisputeObject) []byte {
	o := map[string]any{"id": d.ID, "object": "dispute", "amount": d.Amount, "currency": d.Currency, "reason": d.Reason,
		"status": d.Status, "payment_intent": d.PaymentIntent, "charge": "ch_of_" + d.PaymentIntent}
	if !d.EvidenceDueBy.IsZero() {
		o["evidence_details"] = map[string]any{"due_by": d.EvidenceDueBy.Unix(), "has_evidence": false, "past_due": false}
	}
	return Build(eventID, eventType, o)
}

// FraudWarningEvent is radar.early_fraud_warning.created or .updated.
func FraudWarningEvent(eventID, eventType, warningID, piID, fraudType string, actionable bool) []byte {
	return Build(eventID, eventType, map[string]any{"id": warningID, "object": "radar.early_fraud_warning", "actionable": actionable,
		"fraud_type": fraudType, "payment_intent": piID, "charge": "ch_of_" + piID})
}

// PaymentMethodDetached is payment_method.detached.
func PaymentMethodDetached(eventID, pmID string) []byte {
	return Build(eventID, "payment_method.detached", map[string]any{"id": pmID, "object": "payment_method", "type": "card", "customer": nil})
}

// CustomerDeleted is customer.deleted.
func CustomerDeleted(eventID, customerID string) []byte {
	return Build(eventID, "customer.deleted", map[string]any{"id": customerID, "object": "customer", "deleted": true})
}

// ReviewEvent is review.opened or review.closed.
func ReviewEvent(eventID, eventType, reviewID, piID, reason string) []byte {
	return Build(eventID, eventType, map[string]any{"id": reviewID, "object": "review", "payment_intent": piID,
		"charge": "ch_of_" + piID, "reason": reason, "open": eventType == "review.opened"})
}

// UnknownEvent is an event of a type the platform does not handle.
func UnknownEvent(eventID, eventType string) []byte {
	return Build(eventID, eventType, map[string]any{"id": "obj_" + eventID, "object": "thing"})
}
