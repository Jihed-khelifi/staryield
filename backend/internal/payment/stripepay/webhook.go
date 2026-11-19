package stripepay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v87"
	"github.com/stripe/stripe-go/v87/webhook"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
)

// SignatureHeader is the header Stripe signs webhook deliveries with.
const SignatureHeader = "Stripe-Signature"

// VerifyWebhookRequest implements payment.WebhookVerifier. The signature
// (HMAC with the endpoint secret) is verified by the SDK, the signature's age
// against the request's clock and the configured tolerance. The event's API
// version is not enforced (objects are decoded leniently); a mismatch with the
// SDK's version is logged once. Verification never calls the Stripe API:
// card and risk details missing from an event are left empty and read later
// with RetrieveCharge.
func (p *Provider) VerifyWebhookRequest(req payment.WebhookRequest) (*payment.WebhookEvent, error) {
	header := req.Header.Get(SignatureHeader)
	if header == "" {
		return nil, payment.ErrBadSignature
	}
	evt, err := webhook.ConstructEventWithOptions(req.Body, header, p.webhookKey, webhook.ConstructEventOptions{
		IgnoreTolerance: true, IgnoreAPIVersionMismatch: true})
	if err != nil {
		switch {
		case errors.Is(err, webhook.ErrNoValidSignature), errors.Is(err, webhook.ErrInvalidHeader),
			errors.Is(err, webhook.ErrNotSigned), errors.Is(err, webhook.ErrEmptySecret):
			return nil, payment.ErrBadSignature
		}
		return nil, fmt.Errorf("payment: decode stripe webhook: %w", err)
	}
	if t, ok := signatureTime(header); !ok {
		return nil, payment.ErrBadSignature
	} else if d := req.Now.Sub(t); d > p.tolerance || d < -p.tolerance {
		return nil, payment.ErrStale
	}
	p.noteAPIVersion(evt.ID, evt.APIVersion)
	return normalise(&evt)
}

// signatureTime reads the timestamp of the Stripe-Signature header ("t=…,v1=…").
func signatureTime(header string) (time.Time, bool) {
	for _, part := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || k != "t" {
			continue
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return time.Time{}, false
		}
		return time.Unix(n, 0), true
	}
	return time.Time{}, false
}

// noteAPIVersion logs once per version when events are sent with another
// release train than the SDK expects: they are still decoded, leniently.
func (p *Provider) noteAPIVersion(eventID, version string) {
	if version == "" || sameTrain(version, stripe.APIVersion) {
		return
	}
	p.versionMu.Lock()
	seen := p.versions[version]
	p.versions[version] = true
	p.versionMu.Unlock()
	if !seen {
		p.log.Warn("stripe: webhook API version differs from the SDK's; decoding leniently (create the endpoint with the SDK's version)",
			zap.String("event_version", version), zap.String("sdk_version", stripe.APIVersion), zap.String("event_id", eventID))
	}
}

// sameTrain compares the release train of two "yyyy-MM-dd.train" versions.
func sameTrain(a, b string) bool {
	_, ta, okA := strings.Cut(a, ".")
	_, tb, okB := strings.Cut(b, ".")
	return okA && okB && ta == tb
}

// FetchEvent implements payment.EventFetcher: the event re-read from Stripe by
// id and normalised like a webhook delivery (admin replay).
func (p *Provider) FetchEvent(ctx context.Context, eventID string) (*payment.WebhookEvent, error) {
	if !strings.HasPrefix(eventID, "evt_") {
		return nil, payment.ErrNotFound
	}
	evt, err := p.client.V1Events.Retrieve(ctx, eventID, nil)
	if err != nil {
		return nil, p.mapError("retrieve event", err)
	}
	return normalise(evt)
}

// ------------------------------------------------------- normalisation

// objectRef decodes a Stripe reference that is an id or an expanded object.
type objectRef string

func (r *objectRef) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*r = objectRef(s)
		return nil
	}
	var o struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(b, &o) == nil {
		*r = objectRef(o.ID)
	}
	return nil // lenient: any other shape is "no reference"
}

// wireObject is the union of the fields the handled events' objects carry
// (PaymentIntent, Refund, Dispute, Early Fraud Warning, Review, PaymentMethod,
// Customer). Unknown fields are ignored and absent ones stay zero.
type wireObject struct {
	ID                 string            `json:"id"`
	Amount             int64             `json:"amount"`
	AmountReceived     int64             `json:"amount_received"`
	Currency           string            `json:"currency"`
	Status             string            `json:"status"`
	Reason             string            `json:"reason"`
	Metadata           map[string]string `json:"metadata"`
	PaymentIntent      objectRef         `json:"payment_intent"`
	Actionable         bool              `json:"actionable"`
	FraudType          string            `json:"fraud_type"`
	CancellationReason string            `json:"cancellation_reason"`
	LastPaymentError   *struct {
		Code        string `json:"code"`
		DeclineCode string `json:"decline_code"`
	} `json:"last_payment_error"`
	EvidenceDetails *struct {
		DueBy *int64 `json:"due_by"`
	} `json:"evidence_details"`
}

// normalise maps a Stripe event to the platform's webhook event (§26.5 table).
func normalise(evt *stripe.Event) (*payment.WebhookEvent, error) {
	if evt == nil || evt.ID == "" || evt.Data == nil || len(evt.Data.Raw) == 0 {
		return nil, errors.New("payment: incomplete stripe event")
	}
	var o wireObject
	if err := json.Unmarshal(evt.Data.Raw, &o); err != nil {
		return nil, fmt.Errorf("payment: decode stripe event object: %w", err)
	}
	out := &payment.WebhookEvent{ID: evt.ID, ProviderType: string(evt.Type), Livemode: evt.Livemode, ObjectRef: o.ID,
		Currency: strings.ToUpper(o.Currency)}
	switch evt.Type {
	case stripe.EventTypePaymentIntentSucceeded:
		out.Type = payment.EventChargeSucceeded
		paymentIntentFields(out, &o)
		out.AmountCents = firstPositive(o.AmountReceived, o.Amount)
	case stripe.EventTypePaymentIntentPaymentFailed:
		out.Type = payment.EventChargeFailed
		paymentIntentFields(out, &o)
		out.FailureCode = "card_declined"
		if e := o.LastPaymentError; e != nil {
			out.FailureCode = declineCode(e.Code, e.DeclineCode)
		}
	case stripe.EventTypePaymentIntentCanceled:
		out.Type = payment.EventChargeFailed
		paymentIntentFields(out, &o)
		out.FailureCode = "canceled"
	case stripe.EventTypePaymentIntentProcessing:
		out.Type = payment.EventChargeProcessing
		paymentIntentFields(out, &o)
	case stripe.EventTypeRefundCreated, stripe.EventTypeRefundUpdated:
		out.ProviderRef, out.AmountCents, out.RefundRef, out.RefundStatus = string(o.PaymentIntent), o.Amount, o.ID, o.Status
		switch o.Status {
		case "pending", "succeeded":
			out.Type = payment.EventChargeRefunded
		case "failed", "canceled":
			out.Type = payment.EventRefundFailed
		default: // requires_action or a status Stripe adds later: nothing to book
			out.Type = payment.EventIgnored
		}
	case stripe.EventTypeRefundFailed:
		out.Type = payment.EventRefundFailed
		out.ProviderRef, out.AmountCents, out.RefundRef, out.RefundStatus = string(o.PaymentIntent), o.Amount, o.ID, "failed"
	case stripe.EventTypeChargeDisputeCreated:
		out.Type = payment.EventChargeDisputed
		disputeFields(out, &o)
	case stripe.EventTypeChargeDisputeUpdated:
		out.Type = payment.EventDisputeUpdated
		disputeFields(out, &o)
	case stripe.EventTypeChargeDisputeClosed:
		out.Type = payment.EventDisputeClosed
		disputeFields(out, &o)
	case stripe.EventTypeChargeDisputeFundsWithdrawn:
		out.Type = payment.EventDisputeFundsWithdrawn
		disputeFields(out, &o)
	case stripe.EventTypeChargeDisputeFundsReinstated:
		out.Type = payment.EventDisputeFundsReinstated
		disputeFields(out, &o)
	case stripe.EventTypeRadarEarlyFraudWarningCreated, stripe.EventTypeRadarEarlyFraudWarningUpdated:
		out.Type = payment.EventFraudWarning
		out.ProviderRef, out.Actionable, out.FraudType = string(o.PaymentIntent), o.Actionable, o.FraudType
	case stripe.EventTypePaymentMethodDetached:
		out.Type = payment.EventMethodDetached
	case stripe.EventTypeCustomerDeleted:
		out.Type = payment.EventCustomerDeleted
	case stripe.EventTypeReviewOpened:
		out.Type = payment.EventReviewOpened
		out.ProviderRef = string(o.PaymentIntent)
	case stripe.EventTypeReviewClosed:
		out.Type = payment.EventReviewClosed
		out.ProviderRef = string(o.PaymentIntent)
	default:
		out.Type = payment.EventIgnored
	}
	return out, nil
}

// paymentIntentFields: the PaymentIntent is the payment's reference; our
// metadata (sy_key, sy_purchase_id) lets the purchase be found before its
// provider reference is known.
func paymentIntentFields(out *payment.WebhookEvent, o *wireObject) {
	out.ProviderRef = o.ID
	out.IdempotencyKey = o.Metadata[MetaKey]
	out.PurchaseID = o.Metadata[MetaPurchaseID]
}

// disputeFields: the dispute is the object; the payment it is about is its
// PaymentIntent.
func disputeFields(out *payment.WebhookEvent, o *wireObject) {
	out.ProviderRef, out.DisputeRef, out.DisputeStatus, out.DisputeReason, out.AmountCents = string(o.PaymentIntent), o.ID, o.Status, o.Reason, o.Amount
	if d := o.EvidenceDetails; d != nil && d.DueBy != nil && *d.DueBy > 0 {
		t := time.Unix(*d.DueBy, 0).UTC()
		out.EvidenceDueBy = &t
	}
}

func firstPositive(values ...int64) int64 {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}
