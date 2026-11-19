package stripepay

import (
	"context"
	"errors"
	"strings"

	"github.com/stripe/stripe-go/v87"

	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
)

// Refund implements payment.Refunder: a Stripe refund of the PaymentIntent.
// The reason is Stripe's enum: "fraudulent" (fraud warnings) or "duplicate"
// when the platform says so, "requested_by_customer" otherwise; the platform's
// own free-text reason travels in the metadata. A disputed charge cannot be
// refunded (payment.ErrChargeDisputed): the dispute must be answered instead.
func (p *Provider) Refund(ctx context.Context, req payment.RefundRequest) (payment.RefundResult, error) {
	if !strings.HasPrefix(req.ProviderRef, "pi_") {
		return payment.RefundResult{}, payment.ErrNotFound
	}
	if req.AmountCents <= 0 {
		return payment.RefundResult{}, payment.ErrRefundTooLarge
	}
	params := &stripe.RefundCreateParams{
		PaymentIntent: stripe.String(req.ProviderRef),
		Amount:        stripe.Int64(req.AmountCents),
		Reason:        stripe.String(refundReason(req.Reason)),
		Metadata:      map[string]string{"sy_refund_key": truncate(req.IdempotencyKey, maxMetaValue)},
	}
	if reason := strings.TrimSpace(req.Reason); reason != "" {
		params.Metadata["sy_reason"] = truncate(reason, maxMetaValue)
	}
	withKey(&params.Params, req.IdempotencyKey)
	r, err := p.client.V1Refunds.Create(ctx, params)
	if err != nil {
		mapped := p.mapError("refund payment", err)
		if errors.Is(mapped, payment.ErrNotFound) {
			return payment.RefundResult{}, payment.ErrNotFound
		}
		return payment.RefundResult{}, mapped
	}
	return payment.RefundResult{ProviderRef: r.ID}, nil
}

// refundReason maps the platform's reason to Stripe's refund reason enum.
func refundReason(reason string) string {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case string(stripe.RefundReasonFraudulent):
		return string(stripe.RefundReasonFraudulent)
	case string(stripe.RefundReasonDuplicate):
		return string(stripe.RefundReasonDuplicate)
	}
	return string(stripe.RefundReasonRequestedByCustomer)
}
