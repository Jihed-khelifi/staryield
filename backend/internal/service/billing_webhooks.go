package service

import (
	"context"
	"errors"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 6.0 (§26.5): the webhook log and the dispatcher of normalised
// provider events. Every delivery is recorded in webhook_events (de-duplicated
// by event id, payload hash only) with its status:
//
//	received   recorded, not applied yet (or applying failed before the status was written)
//	processed  applied
//	failed     applying failed: the provider is answered 500 and retries
//	ignored    verified, but nothing to do (an event or a payment we do not act on)
//
// A processed or ignored redelivery is answered 200 without being applied
// again; handlers are idempotent anyway, so at-least-once processing (a
// redelivery of a received or failed event, an admin replay, two deliveries
// racing) never applies an event twice.

// webhookResult says what applying an event did.
type webhookResult struct {
	// ignored is why nothing was done ("" when the event was applied).
	ignored string
}

func appliedEvent() webhookResult { return webhookResult{} }

func ignoredEvent(reason string) webhookResult { return webhookResult{ignored: reason} }

// HandleProviderWebhook processes a signed notification from a card
// provider. Redeliveries are acknowledged without being applied twice; the
// raw payload is never stored.
func (s *BillingService) HandleProviderWebhook(ctx context.Context, provider string, header http.Header, body []byte) error {
	invalid := apperror.New(apperror.CodeBadRequest, http.StatusBadRequest, "Invalid webhook signature.")
	p, ok := s.registry.Get(provider)
	if !ok {
		return invalid
	}
	verifier, ok := p.(payment.WebhookVerifier)
	if !ok {
		return invalid
	}
	evt, err := verifier.VerifyWebhookRequest(payment.WebhookRequest{Header: header, Body: body, Now: s.clock.Now()})
	if err != nil {
		logger.FromContext(ctx).Warn("billing: webhook rejected", zap.Error(err), zap.String("provider", provider))
		return invalid
	}
	if s.ops == nil {
		return apperror.FeatureUnavailable("Payment notifications are not available.")
	}
	digest := sha256Hex(body)
	eventID := evt.ID
	if eventID == "" {
		eventID = "sha256:" + digest
	}
	now := s.clock.Now()
	rec := &domain.ProviderWebhookEvent{ID: domain.NewID(), Provider: provider, EventID: truncateString(eventID, 200),
		EventType: truncateString(evt.Type, 64), PayloadSHA256: digest, Status: domain.WebhookReceived, Livemode: evt.Livemode,
		ProviderType: truncateString(evt.ProviderType, 100), ObjectRef: truncateString(evt.ObjectRef, 200), CreatedAt: now, UpdatedAt: now}
	fresh, err := s.repo.RecordWebhook(ctx, rec)
	if err != nil {
		return apperror.Internal(err)
	}
	if !fresh {
		existing, err := s.repo.GetWebhook(ctx, provider, rec.EventID)
		if err != nil {
			return apperror.Internal(err)
		}
		if existing.Status == domain.WebhookProcessed || existing.Status == domain.WebhookIgnored {
			return nil // a redelivery of an event that was dealt with
		}
		rec = existing
	}
	return s.processWebhook(ctx, rec, evt)
}

// processWebhook applies an event and writes the outcome to its log row.
func (s *BillingService) processWebhook(ctx context.Context, rec *domain.ProviderWebhookEvent, evt *payment.WebhookEvent) error {
	res, err := s.applyWebhook(ctx, rec.Provider, rec.EventID, evt)
	dctx := database.Detach(ctx)
	if err != nil {
		warnErr(dctx, "billing: mark webhook failed", s.ops.repo.UpdateWebhook(dctx, rec.ID, domain.WebhookFailed, errorText(err), nil, s.clock.Now()))
		return err
	}
	status, reason, now := domain.WebhookProcessed, "", s.clock.Now()
	if res.ignored != "" {
		status, reason = domain.WebhookIgnored, res.ignored
	}
	warnErr(dctx, "billing: mark webhook processed", s.ops.repo.UpdateWebhook(dctx, rec.ID, status, truncateString(reason, 500), &now, now))
	return nil
}

// errorText is the (bounded) text of an error for the webhook log.
func errorText(err error) string {
	if e, ok := apperror.As(err); ok && e.Err != nil {
		return truncateString(string(e.Code)+": "+e.Err.Error(), 500)
	}
	return truncateString(err.Error(), 500)
}

// applyWebhook routes a normalised event to its handler.
func (s *BillingService) applyWebhook(ctx context.Context, provider, eventID string, evt *payment.WebhookEvent) (webhookResult, error) {
	switch evt.Type {
	case payment.EventIgnored:
		return ignoredEvent("event type not handled"), nil
	case payment.EventMethodDetached:
		return s.webhookMethodDetached(ctx, provider, evt)
	case payment.EventCustomerDeleted:
		return s.webhookCustomerDeleted(ctx, provider, evt)
	}
	t, err := s.webhookTransaction(ctx, provider, evt)
	if errors.Is(err, repository.ErrNotFound) {
		logger.FromContext(ctx).Warn("billing: webhook for unknown payment", zap.String("provider", provider), zap.String("type", evt.Type))
		return ignoredEvent("no payment matches this event"), nil // acknowledge so the provider stops retrying
	}
	if err != nil {
		return webhookResult{}, apperror.Internal(err)
	}
	switch evt.Type {
	case payment.EventChargeSucceeded, payment.EventChargeFailed:
		return s.webhookCharge(ctx, t, evt)
	case payment.EventChargeProcessing:
		return appliedEvent(), nil // logged
	case payment.EventChargeRefunded:
		return s.webhookRefunded(ctx, t, eventID, evt)
	case payment.EventRefundFailed:
		return s.webhookRefundFailed(ctx, t, evt)
	case payment.EventChargeDisputed, payment.EventDisputeUpdated, payment.EventDisputeClosed,
		payment.EventDisputeFundsWithdrawn, payment.EventDisputeFundsReinstated:
		return s.webhookDispute(ctx, t, eventID, evt)
	case payment.EventFraudWarning:
		return s.webhookFraudWarning(ctx, t, evt)
	case payment.EventReviewOpened, payment.EventReviewClosed:
		return s.webhookReview(ctx, t, evt)
	}
	logger.FromContext(ctx).Info("billing: webhook type ignored", zap.String("type", evt.Type))
	return ignoredEvent("event type not handled"), nil
}

// webhookTransaction finds the payment an event is about: by the charge
// reference, else by the idempotency key the charge carried.
func (s *BillingService) webhookTransaction(ctx context.Context, provider string, evt *payment.WebhookEvent) (*domain.PaymentTransaction, error) {
	t, err := s.repo.GetTransactionByProviderRef(ctx, provider, evt.ProviderRef)
	if errors.Is(err, repository.ErrNotFound) && evt.IdempotencyKey != "" {
		t, err = s.repo.GetTransactionByKey(ctx, provider, evt.IdempotencyKey)
	}
	return t, err
}

// webhookCharge settles a charge that succeeded or failed (the provider's
// word, learned by webhook) and stores the card, risk and 3-D Secure details.
func (s *BillingService) webhookCharge(ctx context.Context, t *domain.PaymentTransaction, evt *payment.WebhookEvent) (webhookResult, error) {
	failed := evt.Type == payment.EventChargeFailed
	if !failed {
		// Before the purchase completes: the receipt shows the card.
		s.recordChargeDetails(ctx, t, evt)
	}
	if t.Status != domain.TxPending {
		return appliedEvent(), nil
	}
	out := chargeOutcome{result: payment.ChargeResult{Status: payment.ChargeSucceeded, ProviderRef: evt.ProviderRef}}
	if failed {
		out = chargeOutcome{decline: &payment.DeclineError{Code: firstNonBlank(evt.FailureCode, "card_declined"), Soft: true}}
	}
	switch t.Kind {
	case domain.TxCreditPurchase:
		return appliedEvent(), s.applyPurchaseOutcome(ctx, t, out)
	case domain.TxSubscriptionInitial, domain.TxSubscriptionRenewal:
		_, err := s.applySubscriptionOutcome(ctx, s.attemptFor(ctx, t), out)
		return appliedEvent(), err
	}
	return appliedEvent(), nil
}

// recordChargeDetails stores what the event says about the card, the risk and
// the 3-D Secure result; when it says nothing and the payment has none yet, the
// provider is asked once (best effort).
func (s *BillingService) recordChargeDetails(ctx context.Context, t *domain.PaymentTransaction, evt *payment.WebhookEvent) {
	d := chargeDetails{Card: evt.Card, Risk: evt.Risk, ThreeDS: evt.ThreeDS}
	if d.empty() && detailsMissing(t) {
		if res, _, ok := s.retrieveCharge(ctx, t.Provider, evt.ProviderRef); ok && res != nil {
			d = chargeDetails{Card: res.Card, Risk: res.Risk, ThreeDS: res.ThreeDS}
		}
	}
	s.persistChargeDetails(ctx, t.ID, d)
}

// webhookRefunded books a refund the provider reports (refunds made in the
// provider's dashboard included): once per refund, however often the event is
// delivered.
func (s *BillingService) webhookRefunded(ctx context.Context, t *domain.PaymentTransaction, eventID string, evt *payment.WebhookEvent) (webhookResult, error) {
	if evt.RefundRef != "" {
		// A refund the platform issued: already booked, or issued and
		// awaiting its booking (same amount) — never booked twice.
		if child, err := s.repo.GetTransactionByProviderRef(ctx, t.Provider, evt.RefundRef); err == nil {
			if child.Status != domain.TxPending {
				return appliedEvent(), nil
			}
			return appliedEvent(), s.bookRefund(ctx, t.ID, child.ID, child.AmountCents, evt.RefundRef)
		} else if !errors.Is(err, repository.ErrNotFound) {
			return webhookResult{}, apperror.Internal(err)
		}
		pending, err := s.repo.PendingRefund(ctx, t.ID)
		if err == nil && (evt.AmountCents <= 0 || pending.AmountCents == evt.AmountCents) {
			return appliedEvent(), s.bookRefund(ctx, t.ID, pending.ID, pending.AmountCents, evt.RefundRef)
		} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return webhookResult{}, apperror.Internal(err)
		}
	}
	// One refund row per provider event, keyed by the event, so a
	// re-processed delivery books it at most once.
	key := truncateString("webhook:"+eventID, 128)
	if child, err := s.repo.GetTransactionByKey(ctx, t.Provider, key); err == nil {
		if child.Status != domain.TxPending {
			return appliedEvent(), nil
		}
		return appliedEvent(), s.bookRefund(ctx, t.ID, child.ID, child.AmountCents, evt.RefundRef)
	} else if !errors.Is(err, repository.ErrNotFound) {
		return webhookResult{}, apperror.Internal(err)
	}
	amount := evt.AmountCents
	if amount <= 0 {
		amount = t.Refundable()
	}
	amount = min(amount, t.Refundable())
	if amount <= 0 {
		return appliedEvent(), nil
	}
	now := s.clock.Now()
	child := &domain.PaymentTransaction{ID: domain.NewID(), UserID: t.UserID, Kind: domain.TxRefund, Status: domain.TxPending,
		AmountCents: amount, Currency: t.Currency, Provider: t.Provider, PaymentMethodID: t.PaymentMethodID,
		SubscriptionID: t.SubscriptionID, InvoiceID: t.InvoiceID, CreditPurchaseID: t.CreditPurchaseID, ParentTransactionID: &t.ID,
		Description: truncateString("Refund: "+t.Description, 200), IdempotencyKey: key, AttemptNo: 1, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateTransaction(ctx, child); err != nil {
		return webhookResult{}, apperror.Internal(err)
	}
	return appliedEvent(), s.bookRefund(ctx, t.ID, child.ID, child.AmountCents, evt.RefundRef)
}

// webhookRefundFailed records a refund the processor could not complete: the
// refund row is failed, the money did not leave, and the team is told. The
// credits already taken back are not restored automatically (the console offers
// "Restore credits"): a person decides whether the customer keeps them.
func (s *BillingService) webhookRefundFailed(ctx context.Context, t *domain.PaymentTransaction, evt *payment.WebhookEvent) (webhookResult, error) {
	ref := firstNonBlank(evt.RefundRef, evt.ObjectRef)
	child, err := s.repo.GetTransactionByProviderRef(ctx, t.Provider, ref)
	if errors.Is(err, repository.ErrNotFound) {
		child, err = s.repo.PendingRefund(ctx, t.ID)
	}
	if errors.Is(err, repository.ErrNotFound) {
		return ignoredEvent("no refund matches this event"), nil
	}
	if err != nil {
		return webhookResult{}, apperror.Internal(err)
	}
	if child.Kind != domain.TxRefund || child.ParentTransactionID == nil || *child.ParentTransactionID != t.ID {
		return ignoredEvent("the refund does not belong to this payment"), nil
	}
	now := s.clock.Now()
	changed := false
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		changed = false
		parent, err := s.repo.GetTransactionForUpdate(ctx, t.ID)
		if err != nil {
			return err
		}
		locked, err := s.repo.GetTransactionForUpdate(ctx, child.ID)
		if err != nil {
			return err
		}
		if locked.Status == domain.TxFailed {
			return nil // already recorded
		}
		changed = true
		if err := s.repo.UpdateTransaction(ctx, locked.ID, map[string]any{"status": domain.TxFailed, "failure_code": "refund_failed",
			"locked_until": nil, "updated_at": now}); err != nil {
			return err
		}
		if locked.Status != domain.TxSucceeded {
			return nil // never booked: the payment's totals do not include it
		}
		return s.unbookRefund(ctx, parent, locked.AmountCents, now)
	})
	if err != nil {
		return webhookResult{}, mapRepoErr(err, "Payment")
	}
	if changed {
		s.record(ctx, "refund.failed", "payment_transaction", t.ID.String(), map[string]any{"refund_id": child.ID,
			"amount_cents": child.AmountCents, "currency": t.Currency, "provider_refund": ref})
		s.alert(ctx, "refund_failed:"+child.ID.String(), "A refund failed at the payment provider", []string{
			"Payment: " + t.ID.String(), "Refund: " + child.ID.String(), "Amount: " + money(child.AmountCents, t.Currency),
			"The customer has not been paid back. The credits taken back for this refund were not restored: restore them from the console if the customer keeps the service.",
		}, "/transactions")
	}
	return appliedEvent(), nil
}

// unbookRefund takes a refund that turned out failed out of its payment's
// totals (the payment is locked by the caller): its refunded amount shrinks
// and a payment that read refunded goes back to partially refunded or paid.
func (s *BillingService) unbookRefund(ctx context.Context, parent *domain.PaymentTransaction, amount int64, now time.Time) error {
	refunded := max(parent.RefundedAmountCents-amount, 0)
	fields := map[string]any{"refunded_amount_cents": refunded, "updated_at": now}
	status := parent.Status
	if parent.Status == domain.TxRefunded || parent.Status == domain.TxPartiallyRefunded {
		status = domain.TxSucceeded
		if refunded > 0 {
			status = domain.TxPartiallyRefunded
		}
		fields["status"] = status
	}
	if err := s.repo.UpdateTransaction(ctx, parent.ID, fields); err != nil {
		return err
	}
	if parent.Status != domain.TxRefunded || status == domain.TxRefunded {
		return nil
	}
	switch {
	case parent.CreditPurchaseID != nil:
		return s.credits.UpdatePurchase(ctx, *parent.CreditPurchaseID, map[string]any{"status": domain.PurchaseSucceeded})
	case parent.InvoiceID != nil:
		return s.repo.UpdateInvoice(ctx, *parent.InvoiceID, map[string]any{"status": domain.InvoicePaid, "updated_at": now})
	}
	return nil
}

// webhookReview flags a payment while the processor's fraud team reviews it.
func (s *BillingService) webhookReview(ctx context.Context, t *domain.PaymentTransaction, evt *payment.WebhookEvent) (webhookResult, error) {
	open := evt.Type == payment.EventReviewOpened
	if t.InReview == open {
		return appliedEvent(), nil
	}
	if err := s.repo.UpdateTransaction(ctx, t.ID, map[string]any{"in_review": open, "updated_at": s.clock.Now()}); err != nil {
		return webhookResult{}, mapRepoErr(err, "Payment")
	}
	return appliedEvent(), nil
}

// webhookMethodDetached removes a saved card the provider says was detached
// (the customer removed it from the issuer or the provider's dashboard).
func (s *BillingService) webhookMethodDetached(ctx context.Context, provider string, evt *payment.WebhookEvent) (webhookResult, error) {
	if s.ops == nil || evt.ObjectRef == "" {
		return ignoredEvent("no payment method reference"), nil
	}
	methods, err := s.ops.repo.MethodsByRefHash(ctx, provider, refHash(evt.ObjectRef))
	if err != nil {
		return webhookResult{}, apperror.Internal(err)
	}
	if len(methods) == 0 {
		return ignoredEvent("no saved card matches this payment method"), nil
	}
	now := s.clock.Now()
	for i := range methods {
		m := methods[i]
		err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
			_, err := s.removeMethodTx(ctx, m.UserID, m.ID, now, true)
			return err
		})
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return webhookResult{}, mapRepoErr(err, "Payment method")
		}
		s.track(ctx, "billing.method_removed", map[string]any{"provider": provider, "by": "provider"})
	}
	return appliedEvent(), nil
}

// webhookCustomerDeleted forgets a customer the provider deleted, and its
// saved cards.
func (s *BillingService) webhookCustomerDeleted(ctx context.Context, provider string, evt *payment.WebhookEvent) (webhookResult, error) {
	if s.ops == nil || evt.ObjectRef == "" {
		return ignoredEvent("no customer reference"), nil
	}
	customers, err := s.ops.repo.CustomersByRefHash(ctx, provider, refHash(evt.ObjectRef))
	if err != nil {
		return webhookResult{}, apperror.Internal(err)
	}
	if len(customers) == 0 {
		return ignoredEvent("no customer matches this reference"), nil
	}
	now := s.clock.Now()
	for _, c := range customers {
		err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
			if _, err := s.users.GetForUpdate(ctx, c.UserID); err != nil {
				return err
			}
			methods, err := s.repo.ListMethods(ctx, c.UserID)
			if err != nil {
				return err
			}
			for _, m := range methods {
				if m.CustomerID != c.ID {
					continue
				}
				if _, err := s.removeMethodTx(ctx, c.UserID, m.ID, now, true); err != nil {
					return err
				}
			}
			return s.repo.SoftDeleteCustomer(ctx, c.ID, now)
		})
		if err != nil {
			return webhookResult{}, mapRepoErr(err, "Customer")
		}
	}
	return appliedEvent(), nil
}
