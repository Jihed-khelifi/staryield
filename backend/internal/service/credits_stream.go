package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 6.0 (§26.5): the purchase stream. After a card payment that is not
// final yet (3-D Secure, a processing charge) the browser opens
// GET /credits/purchases/:id/stream and waits there:
//
//   - the current state is sent at once;
//   - while the purchase is pending and its provider can RetrieveCharge, the
//     server asks the provider immediately and then every PollEvery (at most
//     MaxChecks times) and settles a final outcome itself, through the same
//     idempotent paths as webhooks and the reconciliation worker;
//   - every settlement, wherever it happens, publishes the durable realtime
//     event purchase.updated to the user, which the stream relays;
//   - a final state (succeeded, failed, refunded) is sent and the stream ends;
//     after MaxLifetime the stream says purchase.timeout and ends.

// PurchaseStreamSettings tune the purchase stream.
type PurchaseStreamSettings struct {
	// PollEvery is the delay between provider checks while pending (3 s).
	PollEvery time.Duration
	// MaxChecks bounds the provider checks of one stream (20).
	MaxChecks int
	// MaxLifetime ends the stream (120 s).
	MaxLifetime time.Duration
}

// DefaultPurchaseStream is the contract's behaviour.
func DefaultPurchaseStream() PurchaseStreamSettings {
	return PurchaseStreamSettings{PollEvery: 3 * time.Second, MaxChecks: 20, MaxLifetime: 120 * time.Second}
}

// WithStreamSettings overrides the purchase stream settings (tests shorten them).
func (s *CreditService) WithStreamSettings(c PurchaseStreamSettings) *CreditService {
	s.stream = c
	return s
}

// StreamSettings returns the purchase stream settings.
func (s *CreditService) StreamSettings() PurchaseStreamSettings {
	if s.stream.PollEvery <= 0 || s.stream.MaxLifetime <= 0 {
		return DefaultPurchaseStream()
	}
	return s.stream
}

// FinalPurchaseStatus reports whether a purchase status is final for the stream.
func FinalPurchaseStatus(status string) bool {
	switch domain.PurchaseStatus(status) {
	case domain.PurchaseSucceeded, domain.PurchaseFailed, domain.PurchaseRefunded:
		return true
	}
	return false
}

// purchaseCard is the card that paid a purchase (nil when unknown).
func (s *CreditService) purchaseCard(ctx context.Context, p *domain.CreditPurchase) *CardSummary {
	r, err := s.credits.PurchaseReceipt(ctx, p.ID)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			warnErr(ctx, "credits: card of purchase", err, zap.String("purchase_id", p.ID.String()))
		}
		return nil
	}
	if r.Brand == "" || r.Last4 == "" {
		return nil
	}
	return &CardSummary{Brand: r.Brand, Last4: r.Last4}
}

// OwnPurchase returns a purchase of the user in its current state (404 for
// anyone else's, so ids cannot be probed).
func (s *CreditService) OwnPurchase(ctx context.Context, userID, purchaseID uuid.UUID) (*PurchaseView, error) {
	p, err := s.ownPurchase(ctx, userID, purchaseID)
	if err != nil {
		return nil, err
	}
	return s.withWallet(ctx, p)
}

func (s *CreditService) ownPurchase(ctx context.Context, userID, purchaseID uuid.UUID) (*domain.CreditPurchase, error) {
	rows, err := s.credits.PurchasesByIDs(ctx, []uuid.UUID{purchaseID})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if len(rows) != 1 || rows[0].UserID != userID {
		return nil, apperror.NotFound("Purchase")
	}
	return &rows[0], nil
}

// SyncPurchase checks a pending purchase with its provider (once) and settles a
// final outcome, then returns the purchase as it stands. check=false only reads.
func (s *CreditService) SyncPurchase(ctx context.Context, userID, purchaseID uuid.UUID, check bool) (*PurchaseView, error) {
	p, err := s.ownPurchase(ctx, userID, purchaseID)
	if err != nil {
		return nil, err
	}
	if check && p.Status == domain.PurchasePending && s.billing != nil {
		if err := s.billing.SyncPurchase(ctx, p); err != nil {
			warnErr(ctx, "credits: sync pending purchase", err, zap.String("purchase_id", p.ID.String()))
		}
		if p, err = s.ownPurchase(ctx, userID, purchaseID); err != nil {
			return nil, err
		}
	}
	return s.withWallet(ctx, p)
}

// SyncPurchase asks the provider about the charge of a pending purchase and
// settles it when the answer is final (a still-pending charge is left alone).
func (s *BillingService) SyncPurchase(ctx context.Context, p *domain.CreditPurchase) error {
	if p.ProviderRef == nil || *p.ProviderRef == "" {
		return nil
	}
	t, err := s.repo.GetTransactionByPurchase(ctx, p.ID)
	if errors.Is(err, repository.ErrNotFound) || (err == nil && t.Status != domain.TxPending) {
		return nil
	}
	if err != nil {
		return err
	}
	res, decline, ok := s.retrieveCharge(ctx, p.Provider, *p.ProviderRef)
	if !ok {
		return nil
	}
	switch {
	case decline != nil:
		return s.applyPurchaseOutcome(ctx, t, chargeOutcome{decline: decline})
	case res != nil && res.Status == payment.ChargeSucceeded:
		s.persistChargeDetails(ctx, t.ID, chargeDetails{Card: res.Card, Risk: res.Risk, ThreeDS: res.ThreeDS})
		if res.ProviderRef == "" {
			res.ProviderRef = *p.ProviderRef
		}
		return s.applyPurchaseOutcome(ctx, t, chargeOutcome{result: *res})
	}
	return nil
}

// PublishPurchaseUpdate is the purchase observer that tells the buyer's open
// streams (and every realtime connection) that a purchase reached a final
// state. Register it with OnPurchaseSettled.
func (s *CreditService) PublishPurchaseUpdate(ctx context.Context, p *domain.CreditPurchase) {
	v, err := s.withWallet(ctx, p)
	if err != nil {
		warnErr(ctx, "credits: purchase update", err, zap.String("purchase_id", p.ID.String()))
		return
	}
	warnErr(ctx, "credits: publish purchase update",
		s.events.Publish(ctx, realtime.EventPurchaseUpdated, map[string]any{"purchase": v}, p.UserID), zap.String("purchase_id", p.ID.String()))
}
