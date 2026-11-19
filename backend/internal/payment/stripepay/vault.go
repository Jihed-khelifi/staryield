package stripepay

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v87"

	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
)

// EnsureCustomer implements payment.CustomerVault: the Stripe customer of a
// user, with the account e-mail and display name (Radar uses them for fraud
// screening and they are the dispute evidence). A known reference is
// confirmed (and its e-mail and name refreshed); when Stripe lost it, a new
// customer is created.
func (p *Provider) EnsureCustomer(ctx context.Context, req payment.CustomerRequest) (string, error) {
	if req.ExistingRef != "" {
		c, err := p.client.V1Customers.Retrieve(ctx, req.ExistingRef, nil)
		switch {
		case err == nil && !c.Deleted:
			p.refreshCustomer(ctx, c, req)
			return c.ID, nil
		case err == nil, errors.Is(p.mapError("retrieve customer", err), payment.ErrNotFound):
			// Deleted or unknown at Stripe: fall through and create a new one.
		default:
			return "", p.mapError("retrieve customer", err)
		}
	}
	params := &stripe.CustomerCreateParams{Metadata: customerMetadata(req.UserID)}
	if req.Email != "" {
		params.Email = stripe.String(req.Email)
	}
	if req.Name != "" {
		params.Name = stripe.String(req.Name)
	}
	if req.ExistingRef == "" { // a replay of a key would hand back the deleted customer again
		withKey(&params.Params, req.IdempotencyKey)
	}
	c, err := p.client.V1Customers.Create(ctx, params)
	if err != nil {
		return "", p.mapError("create customer", err)
	}
	return c.ID, nil
}

func customerMetadata(userID uuid.UUID) map[string]string {
	if userID == uuid.Nil {
		return nil
	}
	return map[string]string{MetaUserID: userID.String()}
}

// refreshCustomer keeps the customer's e-mail and name in step with the
// account (e-mail changes, display-name changes). Best effort: a failure is
// not an error, the customer exists.
func (p *Provider) refreshCustomer(ctx context.Context, c *stripe.Customer, req payment.CustomerRequest) {
	upd := &stripe.CustomerUpdateParams{}
	changed := false
	if req.Email != "" && c.Email != req.Email {
		upd.Email, changed = stripe.String(req.Email), true
	}
	if req.Name != "" && c.Name != req.Name {
		upd.Name, changed = stripe.String(req.Name), true
	}
	if !changed {
		return
	}
	if _, err := p.client.V1Customers.Update(ctx, c.ID, upd); err != nil {
		_ = p.mapError("update customer", err) // logs configuration errors
	}
}

// DeleteCustomer implements payment.CustomerVault. Deleting a customer that
// is already gone succeeds (account closure may be retried).
func (p *Provider) DeleteCustomer(ctx context.Context, ref string) error {
	if !strings.HasPrefix(ref, "cus_") {
		return payment.ErrNotFound
	}
	if _, err := p.client.V1Customers.Delete(ctx, ref, nil); err != nil {
		if mapped := p.mapError("delete customer", err); !errors.Is(mapped, payment.ErrNotFound) {
			return mapped
		}
	}
	return nil
}

// Attach implements payment.MethodAttacher. The token is the PaymentMethod id
// (pm_…) of a SetupIntent the browser confirmed; it must already belong to the
// user's customer (a SetupIntent created for the customer attaches it), or be
// unattached, in which case it is attached now. A payment method of another
// customer is ErrInvalidToken.
func (p *Provider) Attach(ctx context.Context, req payment.AttachRequest) (string, payment.CardMeta, error) {
	if !strings.HasPrefix(req.ClientToken, "pm_") || req.CustomerRef == "" {
		return "", payment.CardMeta{}, payment.ErrInvalidToken
	}
	pm, err := p.client.V1PaymentMethods.Retrieve(ctx, req.ClientToken, nil)
	if err != nil {
		if mapped := p.mapError("retrieve payment method", err); !errors.Is(mapped, payment.ErrNotFound) {
			return "", payment.CardMeta{}, mapped
		}
		return "", payment.CardMeta{}, payment.ErrInvalidToken // an unknown payment method is a bad token
	}
	if pm.Type != stripe.PaymentMethodTypeCard || pm.Card == nil {
		return "", payment.CardMeta{}, payment.ErrInvalidToken
	}
	switch owner := customerID(pm); {
	case owner == req.CustomerRef:
	case owner == "":
		params := &stripe.PaymentMethodAttachParams{Customer: stripe.String(req.CustomerRef)}
		withKey(&params.Params, req.IdempotencyKey)
		if pm, err = p.client.V1PaymentMethods.Attach(ctx, pm.ID, params); err != nil {
			return "", payment.CardMeta{}, p.mapError("attach payment method", err)
		}
	default:
		return "", payment.CardMeta{}, payment.ErrInvalidToken
	}
	return pm.ID, cardMetaOf(pm.Card), nil
}

func customerID(pm *stripe.PaymentMethod) string {
	if pm.Customer == nil {
		return ""
	}
	return pm.Customer.ID
}

func cardMetaOf(c *stripe.PaymentMethodCard) payment.CardMeta {
	return payment.CardMeta{Brand: string(c.Brand), Last4: c.Last4, ExpMonth: int(c.ExpMonth), ExpYear: int(c.ExpYear),
		Country: c.Country, Fingerprint: c.Fingerprint}
}

// Detach implements payment.MethodAttacher. A payment method that is already
// detached or gone counts as removed.
func (p *Provider) Detach(ctx context.Context, customerRef, methodRef string) error {
	if !strings.HasPrefix(methodRef, "pm_") {
		return payment.ErrNotFound
	}
	if _, err := p.client.V1PaymentMethods.Detach(ctx, methodRef, nil); err != nil {
		mapped := p.mapError("detach payment method", err)
		// Already detached (Stripe answers 400 about the payment method) or gone.
		if errors.Is(mapped, payment.ErrNotFound) || errors.Is(mapped, payment.ErrInvalidToken) {
			return nil
		}
		return mapped
	}
	return nil
}

// InspectToken implements payment.TokenInspector: the card behind a
// ConfirmationToken (payment_method_preview.card) or a PaymentMethod, with its
// fingerprint, without charging it (the welcome offer allows one offer per
// card, §24.2). A token that was already used or has expired is ErrInvalidToken.
func (p *Provider) InspectToken(ctx context.Context, token string) (payment.CardMeta, error) {
	switch {
	case strings.HasPrefix(token, "ctoken_"):
		ct, err := p.client.V1ConfirmationTokens.Retrieve(ctx, token, nil)
		if err != nil {
			return payment.CardMeta{}, p.mapError("retrieve confirmation token", err)
		}
		if ct.PaymentIntent != "" || ct.SetupIntent != "" || (ct.ExpiresAt > 0 && !p.now().Before(timeUnix(ct.ExpiresAt))) {
			return payment.CardMeta{}, payment.ErrInvalidToken
		}
		prev := ct.PaymentMethodPreview
		if prev == nil || prev.Card == nil {
			return payment.CardMeta{}, payment.ErrInvalidToken
		}
		c := prev.Card
		return payment.CardMeta{Brand: c.Brand, Last4: c.Last4, ExpMonth: int(c.ExpMonth), ExpYear: int(c.ExpYear),
			Country: c.Country, Fingerprint: c.Fingerprint}, nil
	case strings.HasPrefix(token, "pm_"):
		pm, err := p.client.V1PaymentMethods.Retrieve(ctx, token, nil)
		if err != nil {
			return payment.CardMeta{}, p.mapError("retrieve payment method", err)
		}
		if pm.Card == nil {
			return payment.CardMeta{}, payment.ErrInvalidToken
		}
		return cardMetaOf(pm.Card), nil
	}
	return payment.CardMeta{}, payment.ErrInvalidToken
}

// ClientConfig implements payment.ClientTokenizer. Credits and memberships
// use a deferred intent: the browser creates a ConfirmationToken from the
// Payment Element for an amount it sets itself. Saving a card (PurposeMethod)
// needs a SetupIntent for the user's customer, confirmed in the browser.
func (p *Provider) ClientConfig(ctx context.Context, req payment.ClientConfigRequest) (payment.ClientConfig, error) {
	livemode := p.livemode
	cfg := payment.ClientConfig{Provider: Name, Type: "stripe_elements", PublishableKey: p.publishKey,
		PaymentMethodTypes: []string{"card"}, Livemode: &livemode}
	if req.Purpose != payment.PurposeMethod {
		cfg.Mode = "payment"
		cfg.AmountCents = req.AmountCents
		cfg.Currency = strings.ToLower(req.Currency)
		return cfg, nil
	}
	if req.CustomerRef == "" {
		return payment.ClientConfig{}, errors.New("stripepay: saving a card needs the provider customer")
	}
	params := &stripe.SetupIntentCreateParams{
		Customer:                  stripe.String(req.CustomerRef),
		Usage:                     stripe.String("on_session"),
		AllowedPaymentMethodTypes: cardOnly,
		Metadata:                  customerMetadata(req.UserID),
	}
	switch req.ThreeDSecure {
	case payment.ThreeDSAny, payment.ThreeDSAutomatic:
		params.PaymentMethodOptions = &stripe.SetupIntentCreatePaymentMethodOptionsParams{
			Card: &stripe.SetupIntentCreatePaymentMethodOptionsCardParams{RequestThreeDSecure: stripe.String(req.ThreeDSecure)}}
	}
	si, err := p.client.V1SetupIntents.Create(ctx, params)
	if err != nil {
		return payment.ClientConfig{}, p.mapError("create setup intent", err)
	}
	cfg.Mode = "setup"
	cfg.ClientSecret = si.ClientSecret
	return cfg, nil
}
