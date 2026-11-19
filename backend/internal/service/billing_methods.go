package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// ClientConfigInput asks how to collect a card for a payment.
type ClientConfigInput struct {
	Purpose       string
	PlanVersionID *uuid.UUID
	// PackID is the credit pack being paid (purpose credits, revisions 6.0
	// §26.5): its price and currency go into the configuration.
	PackID         *uuid.UUID
	TrustedCountry string
}

// ClientConfig tells the browser which provider's hosted fields to load
// (chosen by the routing rules for this user and country).
func (s *BillingService) ClientConfig(ctx context.Context, userID uuid.UUID, in ClientConfigInput) (*payment.ClientConfig, error) {
	if s.disabled() {
		return nil, featureOff("methods")
	}
	purpose := in.Purpose
	if purpose == "" {
		purpose = payment.PurposeCredits
	}
	if purpose != payment.PurposeCredits && purpose != payment.PurposeSubscription && purpose != payment.PurposeMethod {
		return nil, apperror.Validation(map[string]string{"purpose": "Choose credits, subscription or method."})
	}
	// revisions 6.0 (§26.5): saving a card without paying (purpose=method) is routed like credits.
	routePurpose := purpose
	if purpose == payment.PurposeMethod {
		routePurpose = payment.PurposeCredits
	}
	route := payment.RouteInput{UserID: userID.String(), Countries: countries(in.TrustedCountry), Currency: s.cfg.Currency,
		Purpose: routePurpose, Needs: payment.CapClientConfig | payment.CapVault | payment.CapAttach | payment.CapStoredCharge}
	amount := int64(0)
	switch {
	case in.PlanVersionID != nil:
		v, err := s.repo.GetVersion(ctx, *in.PlanVersionID)
		if err != nil {
			return nil, mapRepoErr(err, "Plan")
		}
		route.PlanID, route.Currency, amount = v.PlanID.String(), v.Currency, v.PriceCents
	case purpose == payment.PurposeCredits && in.PackID != nil:
		// revisions 6.0 (§26.5): Stripe's Payment Element is a deferred intent that needs the amount
		// up front (wallets show it to the payer): the pack's price and currency, as the purchase
		// charges and routes them. Whether the pack may still be bought is the purchase's call.
		pack, err := s.credits.GetPack(ctx, *in.PackID)
		if err != nil {
			return nil, mapRepoErr(err, "Credit pack")
		}
		route.Currency, amount = pack.Currency, pack.PriceCents
	}
	providers, _, err := s.route(ctx, route)
	if errors.Is(err, payment.ErrNoProvider) {
		return nil, apperror.FeatureUnavailable("Card payments are not available for your account yet.")
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	tok := providers[0].(payment.ClientTokenizer)
	req := payment.ClientConfigRequest{UserID: userID, Purpose: purpose, AmountCents: amount, Currency: route.Currency, Now: s.clock.Now(),
		ThreeDSecure: s.policies(ctx).ThreeDSecure} // revisions 6.0: the 3-D Secure policy
	if purpose == payment.PurposeMethod {
		// A SetupIntent belongs to the user's customer at the provider.
		if _, req.CustomerRef, err = s.ensureCustomer(ctx, userID, providers[0]); err != nil {
			return nil, err
		}
	}
	cfg, err := tok.ClientConfig(ctx, req)
	if err != nil {
		return nil, s.providerError(ctx, "client config", err)
	}
	return &cfg, nil
}

// PaymentMethods lists the caller's saved cards (at most a handful, so the
// list is not paginated).
func (s *BillingService) PaymentMethods(ctx context.Context, userID uuid.UUID) ([]PaymentMethodView, error) {
	rows, err := s.repo.ListMethods(ctx, userID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	now := s.clock.Now()
	out := make([]PaymentMethodView, 0, len(rows))
	for i := range rows {
		out = append(out, methodView(&rows[i], now))
	}
	return out, nil
}

// AttachMethodInput saves a card tokenised by the provider's fields.
type AttachMethodInput struct {
	Provider       string
	Token          string
	BillingCountry string
	MakeDefault    bool
	IdempotencyKey string
	TrustedCountry string
}

// AttachPaymentMethod saves a card. It returns created=false when the call
// replays an earlier request or the card is already saved.
func (s *BillingService) AttachPaymentMethod(ctx context.Context, userID uuid.UUID, in AttachMethodInput) (*PaymentMethodView, bool, error) {
	if s.disabled() {
		return nil, false, featureOff("methods")
	}
	fields := map[string]string{}
	if !ValidIdempotencyKey(in.IdempotencyKey) {
		fields["idempotency_key"] = "A valid idempotency key is required."
	}
	if !providerName.MatchString(in.Provider) {
		fields["provider"] = "Choose a supported payment provider."
	}
	for k, v := range validateToken(in.Token) {
		fields[k] = v
	}
	in.BillingCountry = strings.ToUpper(strings.TrimSpace(in.BillingCountry))
	if in.BillingCountry != "" && !countryCode.MatchString(in.BillingCountry) {
		fields["billing_country"] = "Use a two-letter country code."
	}
	if len(fields) > 0 {
		return nil, false, apperror.Validation(fields)
	}
	now := s.clock.Now()
	if m, err := s.repo.GetMethodByKey(ctx, userID, in.IdempotencyKey); err == nil {
		v := methodView(m, now)
		return &v, false, nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, false, apperror.Internal(err)
	}
	p, ok := s.registry.Get(in.Provider)
	if !ok || !payment.CapabilitiesOf(p).Has(payment.CapVault|payment.CapAttach|payment.CapStoredCharge) {
		return nil, false, apperror.Validation(map[string]string{"provider": "Choose a supported payment provider."})
	}
	route := payment.RouteInput{UserID: userID.String(), Countries: countries(in.TrustedCountry, in.BillingCountry), Currency: s.cfg.Currency,
		Needs: payment.CapVault | payment.CapAttach | payment.CapStoredCharge}
	allowed, err := s.providerAllowed(ctx, p.Name(), route, payment.PurposeCredits, payment.PurposeSubscription)
	if err != nil {
		return nil, false, apperror.Internal(err)
	}
	if !allowed {
		return nil, false, apperror.Validation(map[string]string{"provider": "This payment provider is not available for your account."})
	}
	if n, err := s.repo.CountMethods(ctx, userID); err != nil {
		return nil, false, apperror.Internal(err)
	} else if n >= int64(s.cfg.MaxPaymentMethods) {
		return nil, false, s.tooManyMethods()
	}
	customer, customerRef, err := s.ensureCustomer(ctx, userID, p)
	if err != nil {
		return nil, false, err
	}
	attacher := p.(payment.MethodAttacher)
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	methodRef, meta, err := attacher.Attach(cctx, payment.AttachRequest{CustomerRef: customerRef, ClientToken: in.Token,
		IdempotencyKey: "pm:" + userID.String() + ":" + in.IdempotencyKey, Now: now})
	cancel()
	if err != nil {
		return nil, false, attachError(ctx, err)
	}
	detach := func() {
		dctx := database.Detach(ctx)
		warnErr(dctx, "billing: detach rejected card", attacher.Detach(dctx, customerRef, methodRef), zap.String("provider", p.Name()))
	}
	// The issuing country is now known: the provider must also be allowed
	// for it (the stricter of the two countries wins).
	if meta.Country != "" {
		route.Countries = countries(in.TrustedCountry, in.BillingCountry, meta.Country)
		if ok, err := s.providerAllowed(ctx, p.Name(), route, payment.PurposeCredits, payment.PurposeSubscription); err != nil || !ok {
			detach()
			if err != nil {
				return nil, false, apperror.Internal(err)
			}
			return nil, false, apperror.Validation(map[string]string{"token": "This card can't be used with this payment provider."})
		}
	}
	if meta.Fingerprint != "" {
		if existing, err := s.repo.GetMethodByFingerprint(ctx, userID, p.Name(), meta.Fingerprint); err == nil {
			detach()
			v := methodView(existing, now)
			return &v, false, nil
		}
	}
	m := &domain.PaymentMethod{ID: domain.NewID(), UserID: userID, CustomerID: customer.ID, Provider: p.Name(),
		FingerprintHash: meta.Fingerprint, Brand: normalizeBrand(meta.Brand), Last4: meta.Last4, ExpMonth: meta.ExpMonth,
		ExpYear: meta.ExpYear, Country: meta.Country, Status: domain.MethodActive, IdempotencyKey: &in.IdempotencyKey,
		CreatedAt: now, UpdatedAt: now}
	if m.ProviderMethodRefEnc, err = s.sealer.Seal(methodRef, methodAD(m)); err != nil {
		detach()
		return nil, false, apperror.Internal(err)
	}
	m.ProviderMethodRefHash = refHash(methodRef) // revisions 6.0 (§26.5): payment_method.detached finds the card by it
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := s.users.GetForUpdate(ctx, userID); err != nil { // serialises a user's card changes
			return err
		}
		n, err := s.repo.CountMethods(ctx, userID)
		if err != nil {
			return err
		}
		if n >= int64(s.cfg.MaxPaymentMethods) {
			return errTooManyMethods
		}
		if in.MakeDefault || n == 0 {
			if err := s.repo.ClearDefaultMethod(ctx, userID, now); err != nil {
				return err
			}
			m.IsDefault = true
		}
		return s.repo.CreateMethod(ctx, m)
	})
	if err != nil {
		detach()
		switch {
		case errors.Is(err, errTooManyMethods):
			return nil, false, s.tooManyMethods()
		case errors.Is(err, repository.ErrDuplicate):
			if existing, err2 := s.repo.GetMethodByKey(ctx, userID, in.IdempotencyKey); err2 == nil {
				v := methodView(existing, now)
				return &v, false, nil
			}
		}
		return nil, false, mapRepoErr(err, "Payment method")
	}
	s.track(ctx, "billing.method_added", map[string]any{"provider": m.Provider, "brand": m.Brand})
	v := methodView(m, now)
	return &v, true, nil
}

var errTooManyMethods = errors.New("billing: too many payment methods")

func (s *BillingService) tooManyMethods() error {
	return apperror.Conflict(CodeLimitReached, fmt.Sprintf("You can save up to %d cards. Remove one first.", s.cfg.MaxPaymentMethods))
}

func attachError(ctx context.Context, err error) error {
	if d, ok := payment.AsDecline(err); ok {
		if d.Code == "expired_card" {
			return apperror.Validation(map[string]string{"token": "This card has expired."})
		}
		return paymentFailed(d.Code, nil)
	}
	if errors.Is(err, payment.ErrInvalidToken) || errors.Is(err, payment.ErrNotFound) {
		return apperror.Validation(map[string]string{"token": "The card details could not be verified. Please enter them again."})
	}
	logger.FromContext(ctx).Error("billing: attach card failed", zap.Error(err))
	return apperror.Unavailable(err)
}

func normalizeBrand(b string) string {
	b = strings.ToLower(strings.TrimSpace(b))
	if !regexpBrand.MatchString(b) {
		return "card"
	}
	return b
}

// ensureCustomer returns the user's customer at provider p, creating it
// at the provider (and locally) the first time.
func (s *BillingService) ensureCustomer(ctx context.Context, userID uuid.UUID, p payment.Named) (*domain.PaymentCustomer, string, error) {
	vault := p.(payment.CustomerVault)
	// revisions 6.0 (§26.5): the account e-mail and name go to the provider (fraud screening, dispute evidence).
	email, name := s.customerIdentity(ctx, userID)
	key := "cus:" + userID.String() + ":" + p.Name()
	fresh := "" // a replacement customer the provider already created
	if c, err := s.repo.GetCustomer(ctx, userID, p.Name()); err == nil {
		ref, err := s.sealer.Open(c.ProviderCustomerRefEnc, customerAD(c))
		if err != nil {
			return nil, "", apperror.Internal(err)
		}
		cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		confirmed, err := vault.EnsureCustomer(cctx, payment.CustomerRequest{UserID: userID, ExistingRef: ref, Email: email, Name: name})
		cancel()
		if err != nil {
			return nil, "", s.providerError(ctx, "ensure customer", err)
		}
		if confirmed == ref {
			return c, ref, nil
		}
		// The provider lost the customer and created a new one while confirming it:
		// retire the local link and keep the new reference (no second customer).
		warnErr(ctx, "billing: retire stale customer", s.repo.SoftDeleteCustomer(ctx, c.ID, s.clock.Now()))
		fresh = confirmed
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, "", apperror.Internal(err)
	}
	ref := fresh
	if ref == "" {
		cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		var err error
		ref, err = vault.EnsureCustomer(cctx, payment.CustomerRequest{UserID: userID, IdempotencyKey: key, Email: email, Name: name})
		cancel()
		if err != nil {
			return nil, "", s.providerError(ctx, "ensure customer", err)
		}
	}
	now := s.clock.Now()
	c := &domain.PaymentCustomer{ID: domain.NewID(), UserID: userID, Provider: p.Name(), CreatedAt: now, UpdatedAt: now}
	sealed, err := s.sealer.Seal(ref, customerAD(c))
	if err != nil {
		return nil, "", apperror.Internal(err)
	}
	c.ProviderCustomerRefEnc = sealed
	c.ProviderCustomerRefHash = refHash(ref) // revisions 6.0 (§26.5): customer.deleted finds the customer by it
	if err := s.repo.CreateCustomer(ctx, c); err != nil {
		if errors.Is(err, repository.ErrDuplicate) { // a concurrent request created it
			existing, err2 := s.repo.GetCustomer(ctx, userID, p.Name())
			if err2 != nil {
				return nil, "", apperror.Internal(err2)
			}
			ref2, err2 := s.sealer.Open(existing.ProviderCustomerRefEnc, customerAD(existing))
			if err2 != nil {
				return nil, "", apperror.Internal(err2)
			}
			return existing, ref2, nil
		}
		return nil, "", apperror.Internal(err)
	}
	return c, ref, nil
}

// SetDefaultPaymentMethod makes a card the default.
func (s *BillingService) SetDefaultPaymentMethod(ctx context.Context, userID, id uuid.UUID) (*PaymentMethodView, error) {
	now := s.clock.Now()
	var m *domain.PaymentMethod
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := s.users.GetForUpdate(ctx, userID); err != nil {
			return err
		}
		var err error
		if m, err = s.repo.GetMethod(ctx, userID, id); err != nil {
			return err
		}
		if !m.Usable(now) {
			return apperror.Validation(map[string]string{"payment_method_id": "This card can no longer be charged. Add another card."})
		}
		if err := s.repo.ClearDefaultMethod(ctx, userID, now); err != nil {
			return err
		}
		m.IsDefault, m.UpdatedAt = true, now
		return s.repo.UpdateMethod(ctx, m.ID, map[string]any{"is_default": true, "updated_at": now})
	})
	if err != nil {
		return nil, mapRepoErr(err, "Payment method")
	}
	v := methodView(m, now)
	return &v, nil
}

// DeletePaymentMethod removes a saved card. A card paying an auto-renewing
// membership is replaced by another usable card, or the removal is refused
// when there is none.
func (s *BillingService) DeletePaymentMethod(ctx context.Context, userID, id uuid.UUID) error {
	now := s.clock.Now()
	var m *domain.PaymentMethod
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		m, err = s.removeMethodTx(ctx, userID, id, now, false)
		return err
	})
	if err != nil {
		return mapRepoErr(err, "Payment method")
	}
	if p, ok := s.registry.Get(m.Provider); ok {
		if attacher, ok := p.(payment.MethodAttacher); ok {
			dctx := database.Detach(ctx)
			if customerRef, methodRef, _, err := s.refs(dctx, m); err == nil {
				warnErr(dctx, "billing: detach card at provider", attacher.Detach(dctx, customerRef, methodRef), zap.String("provider", m.Provider))
			}
		}
	}
	s.track(ctx, "billing.method_removed", map[string]any{"provider": m.Provider})
	return nil
}

// removeMethodTx removes the saved card id of userID inside the caller's
// transaction: memberships paid with it move to the user's other usable card
// (the default one first), and the default flag moves too. Without another
// card a membership that still renews blocks the removal (Conflict), unless
// force is set: the card is already gone at the provider (revisions 6.0,
// §26.5: payment_method.detached), so the membership keeps pointing at it and
// falls into dunning instead of keeping the card alive.
func (s *BillingService) removeMethodTx(ctx context.Context, userID, id uuid.UUID, now time.Time, force bool) (*domain.PaymentMethod, error) {
	if _, err := s.users.GetForUpdate(ctx, userID); err != nil {
		return nil, err
	}
	m, err := s.repo.GetMethod(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	others, err := s.repo.ListMethods(ctx, userID)
	if err != nil {
		return nil, err
	}
	var replacement *domain.PaymentMethod
	for i := range others {
		o := &others[i]
		if o.ID != m.ID && o.Usable(now) && (replacement == nil || o.IsDefault) {
			replacement = o
		}
	}
	subs, err := s.repo.OpenSubscriptionsUsingMethod(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	for _, sub := range subs {
		if sub.CancelAtPeriodEnd && sub.Status == domain.SubActive {
			continue // ends anyway; no renewal will charge this card
		}
		if replacement == nil {
			if force {
				continue
			}
			return nil, apperror.Conflict(CodePaymentMethodRequired,
				"This card pays for your membership. Add another card or cancel the membership first.")
		}
		if err := s.repo.UpdateSubscription(ctx, sub.ID, map[string]any{"payment_method_id": replacement.ID,
			"provider": replacement.Provider, "updated_at": now}); err != nil {
			return nil, err
		}
	}
	if err := s.repo.SoftDeleteMethod(ctx, m.ID, now); err != nil {
		return nil, err
	}
	if m.IsDefault && replacement != nil {
		return m, s.repo.UpdateMethod(ctx, replacement.ID, map[string]any{"is_default": true, "updated_at": now})
	}
	return m, nil
}
