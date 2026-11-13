package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// revisions 5.0: the welcome (intro) offer's state machine (§24.2).
//
// OfferService is the ONE place that moves a redemption between states:
//
//	(purchase of an intro pack) -> reserved   Reserve, in the purchase's transaction, before the charge
//	reserved -> redeemed                      Redeem: the charge succeeded
//	reserved -> released                      Release: the charge failed or reconciliation failed
//	redeemed -> redeemed                      a refund never restores eligibility (Refund Policy §5)
//
// Limits: one per customer and one per payment method, enforced by the
// partial unique indexes of offer_redemptions (which also make concurrent
// attempts safe: exactly one reservation wins).

// offerKeyLabel is the derivation label of the card-key HMAC key; it is
// distinct from every other key derived from the data encryption key.
const offerKeyLabel = "staryield:offer-card:v1"

// OfferService reserves, redeems and releases welcome-offer redemptions.
type OfferService struct {
	repo  repository.PricingRepository
	clock clock.Clock
	key   string
	// requireCardCheck: refuse a purchase whose card fingerprint is unknown
	// (STARYIELD_PRICING_OFFER_REQUIRE_CARD_CHECK).
	requireCardCheck bool
}

// NewOfferService returns the offer state machine. dataKey is the data
// encryption key; the card-key HMAC key is derived from it.
func NewOfferService(repo repository.PricingRepository, clk clock.Clock, dataKey []byte, requireCardCheck bool) *OfferService {
	return &OfferService{repo: repo, clock: clk, key: security.Keyed(string(dataKey), offerKeyLabel, 0), requireCardCheck: requireCardCheck}
}

// CardKey is HMAC-SHA256(key, provider ":" fingerprint), hex encoded, or ""
// when there is no fingerprint. The raw fingerprint is never stored.
func (o *OfferService) CardKey(provider, fingerprint string) string {
	return cardKey(o.key, provider, fingerprint)
}

func cardKey(key, provider, fingerprint string) string {
	if fingerprint == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(provider + ":" + fingerprint))
	return hex.EncodeToString(mac.Sum(nil))
}

// OfferPlan is a checked intent to buy the welcome pack: the card the
// offer will be bound to.
type OfferPlan struct {
	Provider string
	CardKey  string // "" when the card check is switched off and no fingerprint exists
}

// Plan checks a welcome-pack purchase before anything is created: the
// account's eligibility and the card's fingerprint. fingerprint is the
// card's provider fingerprint ("" when it cannot be known); hasCard says
// whether the purchase has a card at all (a hosted checkout has none and is
// never allowed).
func (o *OfferService) Plan(ctx context.Context, userID uuid.UUID, pack *domain.CreditPack, hasCard bool, provider, fingerprint string) (*OfferPlan, error) {
	if !hasCard {
		return nil, apperror.OfferUnavailable(apperror.OfferReasonCardCheck)
	}
	st, err := o.repo.CatalogState(ctx, userID)
	if err != nil {
		return nil, mapRepoErr(err, "Wallet")
	}
	if ok, reason := OfferEligible(pack.Active && pack.Kind == domain.PackIntro, pack.NewClientsOnly, st.HasRedemption, st.HasPaid); !ok {
		return nil, apperror.OfferUnavailable(reason)
	}
	key := o.CardKey(provider, fingerprint)
	if key == "" && o.requireCardCheck {
		return nil, apperror.OfferUnavailable(apperror.OfferReasonCardCheck)
	}
	return &OfferPlan{Provider: provider, CardKey: key}, nil
}

// Reserve inserts the reservation of a purchase. It must run inside the
// purchase's transaction, before the charge. A unique violation means the
// account or the card already has the offer (409 offer_unavailable).
// A nil plan is a no-op, so callers need not branch.
func (o *OfferService) Reserve(ctx context.Context, plan *OfferPlan, p *domain.CreditPurchase) error {
	if o == nil || plan == nil {
		return nil
	}
	now := o.clock.Now()
	err := o.repo.ReserveOffer(ctx, &domain.OfferRedemption{ID: domain.NewID(), UserID: p.UserID, PackID: p.PackID, PurchaseID: p.ID,
		Provider: plan.Provider, CardKey: plan.CardKey, Status: domain.OfferReserved, CreatedAt: now, UpdatedAt: now})
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrOfferUserTaken):
		return apperror.OfferUnavailable(apperror.OfferReasonAlreadyRedeemed)
	case errors.Is(err, repository.ErrOfferCardTaken):
		return apperror.OfferUnavailable(apperror.OfferReasonPaymentUsed)
	}
	return err
}

// Redeem marks the purchase's reservation redeemed (the charge succeeded).
// It joins the caller's transaction and does nothing for a purchase without
// a reservation or one that is no longer reserved.
func (o *OfferService) Redeem(ctx context.Context, purchaseID uuid.UUID) error {
	if o == nil {
		return nil
	}
	_, err := o.repo.MoveOffer(ctx, purchaseID, domain.OfferReserved, domain.OfferRedeemed, o.clock.Now())
	return err
}

// Release frees the purchase's reservation (the charge failed or was
// reconciled as failed): the client may try again. A redeemed offer is never
// released.
func (o *OfferService) Release(ctx context.Context, purchaseID uuid.UUID) error {
	if o == nil {
		return nil
	}
	_, err := o.repo.MoveOffer(ctx, purchaseID, domain.OfferReserved, domain.OfferReleased, o.clock.Now())
	return err
}
