package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 5.0: the welcome offer inside the purchase flow (§24.2). The
// state machine itself lives in offers.go; these are the CreditService's
// thin hooks around it.

// WithOffers enables the welcome offer.
func (s *CreditService) WithOffers(o *OfferService) *CreditService {
	s.offers = o
	return s
}

// planOffer checks a welcome-pack purchase before anything is created: the
// account's eligibility and the card behind the purchase. A hosted checkout
// (no card) is refused.
func (s *CreditService) planOffer(ctx context.Context, userID uuid.UUID, pack *domain.CreditPack, src *PurchaseSource) (*OfferPlan, error) {
	if s.offers == nil {
		return nil, apperror.OfferUnavailable(apperror.OfferReasonNoOffer)
	}
	if src == nil || s.billing == nil {
		return s.offers.Plan(ctx, userID, pack, false, "", "")
	}
	fingerprint, err := s.billing.SourceFingerprint(ctx, src)
	if err != nil {
		return nil, err
	}
	return s.offers.Plan(ctx, userID, pack, true, src.Provider, fingerprint)
}

// createPurchase inserts the purchase and, for a welcome pack, its
// reservation in the same transaction (before the charge). Without a plan it
// is the plain insert.
func (s *CreditService) createPurchase(ctx context.Context, p *domain.CreditPurchase, plan *OfferPlan) error {
	if plan == nil {
		return s.credits.CreatePurchase(ctx, p)
	}
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.credits.CreatePurchase(ctx, p); err != nil {
			return err
		}
		return s.offers.Reserve(ctx, plan, p)
	})
}

// releaseOffer frees a failed purchase's reservation (best effort: a failed
// release is logged, and the next attempt of the client reports the offer as
// used until an operator intervenes).
func (s *CreditService) releaseOffer(ctx context.Context, purchaseID uuid.UUID) {
	warnErr(ctx, "credits: release welcome offer", s.offers.Release(ctx, purchaseID))
}
