package stripepay

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v87"

	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
)

// Metadata keys every PaymentIntent carries (§26.5). They tie a Stripe
// payment back to the purchase, ledger row, user and idempotency key, so a
// webhook or a lost response can always be matched.
const (
	MetaPurchaseID = "sy_purchase_id"
	MetaTxID       = "sy_tx_id"
	MetaUserID     = "sy_user_id"
	MetaKey        = "sy_key"
)

// Stripe metadata limits: 40-character keys, 500-character values, 50 keys.
const (
	maxMetaKey   = 40
	maxMetaValue = 500
	maxMetaPairs = 50
)

// cardOnly is the only payment method type the platform offers (Apple Pay and
// Google Pay are card wallets, so every payment confirms immediately).
var cardOnly = []*string{stripe.String("card")}

// metadataFor merges the caller's metadata with the adapter's own keys
// (sy_key always; sy_purchase_id and sy_user_id when known).
func metadataFor(extra map[string]string, key string, purchaseID, userID uuid.UUID) map[string]string {
	out := map[string]string{}
	for k, v := range extra {
		if k == "" || len(k) > maxMetaKey || len(out) >= maxMetaPairs {
			continue
		}
		out[k] = truncate(v, maxMetaValue)
	}
	if key != "" {
		out[MetaKey] = truncate(key, maxMetaValue)
	}
	if purchaseID != uuid.Nil {
		if _, ok := out[MetaPurchaseID]; !ok {
			out[MetaPurchaseID] = purchaseID.String()
		}
	}
	if userID != uuid.Nil {
		if _, ok := out[MetaUserID]; !ok {
			out[MetaUserID] = userID.String()
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// threeDSecureOptions is payment_method_options.card.request_three_d_secure
// for the platform policy ("any" authenticates every card payment;
// "automatic" lets Stripe decide). Unknown or empty values leave Stripe's
// default.
func threeDSecureOptions(policy string) *stripe.PaymentIntentCreatePaymentMethodOptionsParams {
	switch policy {
	case payment.ThreeDSAny, payment.ThreeDSAutomatic:
		return &stripe.PaymentIntentCreatePaymentMethodOptionsParams{
			Card: &stripe.PaymentIntentCreatePaymentMethodOptionsCardParams{RequestThreeDSecure: stripe.String(policy)},
		}
	}
	return nil
}

// Checkout implements payment.Checkouter: the token is a ConfirmationToken
// (ctoken_…) created by Stripe.js from the Payment Element; a PaymentMethod id
// (pm_…) is accepted too.
func (p *Provider) Checkout(ctx context.Context, req payment.CheckoutRequest) (payment.CheckoutResult, error) {
	if req.AmountCents <= 0 || strings.TrimSpace(req.Currency) == "" {
		return payment.CheckoutResult{}, errors.New("stripepay: a positive amount and a currency are required")
	}
	params := &stripe.PaymentIntentCreateParams{
		Amount:                    stripe.Int64(req.AmountCents),
		Currency:                  stripe.String(strings.ToLower(req.Currency)),
		Confirm:                   stripe.Bool(true),
		AllowedPaymentMethodTypes: cardOnly,
		Metadata:                  metadataFor(req.Metadata, req.IdempotencyKey, req.PurchaseID, req.UserID),
		PaymentMethodOptions:      threeDSecureOptions(req.ThreeDSecure),
		Expand:                    []*string{stripe.String("latest_charge")},
		// Staryield sends its own receipts: never receipt_email. The statement
		// descriptor is account-level at Stripe, so req.StatementDescriptor is ignored.
	}
	switch {
	case strings.HasPrefix(req.PaymentToken, "ctoken_"):
		params.ConfirmationToken = stripe.String(req.PaymentToken)
	case strings.HasPrefix(req.PaymentToken, "pm_"):
		params.PaymentMethod = stripe.String(req.PaymentToken)
	default:
		return payment.CheckoutResult{}, payment.ErrInvalidToken
	}
	if req.Description != "" {
		params.Description = stripe.String(req.Description)
	}
	if req.CustomerRef != "" {
		params.Customer = stripe.String(req.CustomerRef)
	}
	if req.ReturnURL != "" {
		params.ReturnURL = stripe.String(req.ReturnURL)
	}
	withKey(&params.Params, req.IdempotencyKey)
	pi, err := p.client.V1PaymentIntents.Create(ctx, params)
	if err != nil {
		return p.checkoutFromError("create payment", err)
	}
	return p.checkoutFromIntent(pi)
}

// checkoutFromError turns a failed create into a checkout outcome: a decline
// is a failed result (with the PaymentIntent's id when Stripe returned it),
// everything else an error.
func (p *Provider) checkoutFromError(op string, err error) (payment.CheckoutResult, error) {
	mapped := p.mapError(op, err)
	if d, ok := payment.AsDecline(mapped); ok {
		res := payment.CheckoutResult{Status: payment.CheckoutFailed, FailureCode: d.Code}
		var se *stripe.Error
		if errors.As(err, &se) && se.PaymentIntent != nil {
			res.ProviderRef = se.PaymentIntent.ID
		}
		return res, nil
	}
	return payment.CheckoutResult{}, mapped
}

// checkoutFromIntent maps a PaymentIntent returned by create to a checkout result.
func (p *Provider) checkoutFromIntent(pi *stripe.PaymentIntent) (payment.CheckoutResult, error) {
	res, decline, err := p.outcome(pi)
	if err != nil {
		return payment.CheckoutResult{}, err
	}
	if decline != nil {
		return payment.CheckoutResult{Status: payment.CheckoutFailed, ProviderRef: pi.ID, FailureCode: decline.Code}, nil
	}
	out := payment.CheckoutResult{ProviderRef: res.ProviderRef, NextAction: res.NextAction, Card: res.Card, Risk: res.Risk, ThreeDS: res.ThreeDS}
	if res.Status == payment.ChargeSucceeded {
		out.Status = payment.CheckoutSucceeded
	} else {
		out.Status = payment.CheckoutPending
	}
	return out, nil
}

// ChargeStored implements payment.StoredCharger: a PaymentIntent for the
// saved card (customer and payment method), confirmed at once.
// Customer-initiated charges are on session; renewals and other merchant
// initiated charges are off session.
func (p *Provider) ChargeStored(ctx context.Context, req payment.StoredCharge) (payment.ChargeResult, error) {
	if req.AmountCents <= 0 || strings.TrimSpace(req.Currency) == "" || req.CustomerRef == "" || req.MethodRef == "" {
		return payment.ChargeResult{}, errors.New("stripepay: an amount, currency, customer and payment method are required")
	}
	params := &stripe.PaymentIntentCreateParams{
		Amount:                    stripe.Int64(req.AmountCents),
		Currency:                  stripe.String(strings.ToLower(req.Currency)),
		Customer:                  stripe.String(req.CustomerRef),
		PaymentMethod:             stripe.String(req.MethodRef),
		Confirm:                   stripe.Bool(true),
		OffSession:                stripe.Bool(req.Reason == payment.ReasonRecurring || req.Reason == payment.ReasonUnscheduled),
		AllowedPaymentMethodTypes: cardOnly,
		Metadata:                  metadataFor(req.Metadata, req.IdempotencyKey, uuid.Nil, uuid.Nil),
		PaymentMethodOptions:      threeDSecureOptions(req.ThreeDSecure),
		Expand:                    []*string{stripe.String("latest_charge")},
	}
	if req.Description != "" {
		params.Description = stripe.String(req.Description)
	}
	if req.ReturnURL != "" {
		params.ReturnURL = stripe.String(req.ReturnURL)
	}
	withKey(&params.Params, req.IdempotencyKey)
	pi, err := p.client.V1PaymentIntents.Create(ctx, params)
	if err != nil {
		return payment.ChargeResult{}, p.mapError("charge stored card", err)
	}
	res, decline, err := p.outcome(pi)
	if err != nil {
		return payment.ChargeResult{}, err
	}
	if decline != nil {
		return payment.ChargeResult{}, decline
	}
	return res, nil
}

// RetrieveCharge implements payment.ChargeRetriever: the current outcome of
// a PaymentIntent (after the customer finished or abandoned 3-D Secure).
func (p *Provider) RetrieveCharge(ctx context.Context, providerRef string) (payment.ChargeResult, *payment.DeclineError, error) {
	if !strings.HasPrefix(providerRef, "pi_") {
		return payment.ChargeResult{}, nil, payment.ErrNotFound
	}
	pi, err := p.client.V1PaymentIntents.Retrieve(ctx, providerRef, &stripe.PaymentIntentRetrieveParams{
		Expand: []*string{stripe.String("latest_charge")}})
	if err != nil {
		return payment.ChargeResult{}, nil, p.mapError("retrieve payment", err)
	}
	return p.outcome(pi)
}

// Lookup implements payment.ChargeLookup: it finds the PaymentIntent created
// with the idempotency key (metadata sy_key) after a lost response. Stripe's
// search is eventually consistent (up to about a minute), so "not found" means
// "not yet": callers retry.
func (p *Provider) Lookup(ctx context.Context, idempotencyKey string) (payment.LookupResult, error) {
	if idempotencyKey == "" {
		return payment.LookupResult{}, nil
	}
	params := &stripe.PaymentIntentSearchParams{
		SearchParams: stripe.SearchParams{Query: fmt.Sprintf("metadata['%s']:'%s'", MetaKey, escapeSearch(idempotencyKey)), Limit: stripe.Int64(1)},
		Expand:       []*string{stripe.String("data.latest_charge")},
	}
	list := p.client.V1PaymentIntents.Search(ctx, params)
	if err := list.Err(); err != nil {
		return payment.LookupResult{}, p.mapError("search payments", err)
	}
	data := list.Data()
	if len(data) == 0 {
		return payment.LookupResult{}, nil
	}
	res, decline, err := p.outcome(data[0])
	if err != nil {
		return payment.LookupResult{}, err
	}
	if decline != nil {
		return payment.LookupResult{Found: true, Decline: decline}, nil
	}
	return payment.LookupResult{Found: true, Result: res}, nil
}

// escapeSearch escapes a value for Stripe's search query language.
func escapeSearch(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	return strings.ReplaceAll(v, `'`, `\'`)
}

// outcome maps a PaymentIntent to a charge outcome.
//
//	succeeded                      → ChargeSucceeded (card, risk, 3-D Secure from the charge)
//	requires_action                → ChargeRequiresAction with the next action
//	processing / requires_capture  → ChargePending
//	requires_payment_method / canceled
//	                               → a decline (the last payment error, or "canceled")
func (p *Provider) outcome(pi *stripe.PaymentIntent) (payment.ChargeResult, *payment.DeclineError, error) {
	if pi == nil || pi.ID == "" {
		return payment.ChargeResult{}, nil, errors.New("stripepay: empty payment intent")
	}
	res := payment.ChargeResult{ProviderRef: pi.ID}
	if ch := pi.LatestCharge; ch != nil {
		res.Card, res.Risk, res.ThreeDS = chargeDetails(ch)
		if card := chargeCard(ch); card != nil && card.NetworkTransactionID != "" {
			res.NetworkTxnRef = card.NetworkTransactionID
		}
	}
	switch pi.Status {
	case stripe.PaymentIntentStatusSucceeded:
		res.Status = payment.ChargeSucceeded
		return res, nil, nil
	case stripe.PaymentIntentStatusRequiresAction, stripe.PaymentIntentStatusRequiresConfirmation:
		res.Status = payment.ChargeRequiresAction
		res.NextAction = nextAction(pi)
		return res, nil, nil
	case stripe.PaymentIntentStatusProcessing, stripe.PaymentIntentStatusRequiresCapture:
		res.Status = payment.ChargePending
		return res, nil, nil
	case stripe.PaymentIntentStatusCanceled:
		return payment.ChargeResult{}, &payment.DeclineError{Code: "canceled"}, nil
	case stripe.PaymentIntentStatusRequiresPaymentMethod:
		if e := pi.LastPaymentError; e != nil {
			return payment.ChargeResult{}, newDecline(string(e.Code), string(e.DeclineCode)), nil
		}
		return payment.ChargeResult{}, newDecline("", ""), nil
	}
	return payment.ChargeResult{}, nil, fmt.Errorf("stripepay: unexpected payment intent status %q", pi.Status)
}

// nextAction is the customer step a requires_action PaymentIntent needs. The
// browser completes it with stripe.handleNextAction(client_secret), so the
// client secret always travels with it.
func nextAction(pi *stripe.PaymentIntent) *payment.NextAction {
	na := &payment.NextAction{Type: "use_stripe_sdk", ClientSecret: pi.ClientSecret}
	if pi.NextAction != nil {
		if t := string(pi.NextAction.Type); t != "" {
			na.Type = t
		}
		if r := pi.NextAction.RedirectToURL; r != nil {
			na.URL = r.URL
		}
	}
	return na
}

func chargeCard(ch *stripe.Charge) *stripe.ChargePaymentMethodDetailsCard {
	if ch == nil || ch.PaymentMethodDetails == nil {
		return nil
	}
	return ch.PaymentMethodDetails.Card
}

// chargeDetails reads the card, risk assessment and 3-D Secure result from an
// (expanded) charge. Each part is nil or empty when Stripe gave none.
func chargeDetails(ch *stripe.Charge) (*payment.CardMeta, *payment.RiskInfo, string) {
	var card *payment.CardMeta
	var threeDS string
	if c := chargeCard(ch); c != nil {
		card = &payment.CardMeta{Brand: string(c.Brand), Last4: c.Last4, ExpMonth: int(c.ExpMonth), ExpYear: int(c.ExpYear),
			Country: c.Country, Fingerprint: c.Fingerprint}
		if c.ThreeDSecure != nil {
			threeDS = threeDSResult(string(c.ThreeDSecure.Result))
		}
	}
	var risk *payment.RiskInfo
	if o := ch.Outcome; o != nil && o.RiskLevel != "" {
		score := -1
		if o.RiskScore > 0 {
			score = int(o.RiskScore)
		}
		risk = &payment.RiskInfo{Level: o.RiskLevel, Score: score}
	}
	return card, risk, threeDS
}

// threeDSResult maps Stripe's 3-D Secure result to the platform's values
// ("authenticated", "attempted", "not_supported", "failed" or "").
func threeDSResult(r string) string {
	switch r {
	case "authenticated":
		return "authenticated"
	case "attempt_acknowledged":
		return "attempted"
	case "not_supported":
		return "not_supported"
	case "failed":
		return "failed"
	}
	return ""
}
