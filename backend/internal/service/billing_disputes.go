package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 6.0 (§26.5): disputes.
//
// A chargeback (status needs_response or under_review) takes effect once:
//
//   - the payment becomes `disputed` (its earlier status is remembered);
//   - with the dispute_clawback policy the unused credits of THAT purchase's
//     lot are taken back (ledger dispute_reversal; never other purchases or
//     bonus credits, never below zero) and recorded as credits_frozen;
//   - a membership payment ends the membership;
//   - the team is told (e-mail to the support inbox, audit dispute.opened).
//
// Winning the dispute restores what was frozen into the same lot (ledger
// dispute_restore) and puts the payment's earlier status back; losing it
// restores nothing. An inquiry (warning_*) freezes nothing until it becomes a
// chargeback. All of it runs under the payment's row lock, so the same event
// delivered twice, in parallel, or out of order takes effect once.

// disputeRank orders the statuses of a dispute: they only move forward, so a
// late "needs_response" cannot reopen a dispute already under review or closed.
func disputeRank(s domain.DisputeStatus) int {
	switch s {
	case domain.DisputeWarningNeedsResponse:
		return 1
	case domain.DisputeWarningUnderReview:
		return 2
	case domain.DisputeNeedsResponse:
		return 3
	case domain.DisputeUnderReview:
		return 4
	}
	return 5 // closed: won, lost, warning_closed, prevented
}

// nextDisputeStatus is the status a dispute has after an event reports
// incoming: forward moves are taken, stale or repeated reports (and any report
// on a closed dispute) change nothing. An empty incoming keeps the current one.
func nextDisputeStatus(current, incoming domain.DisputeStatus) domain.DisputeStatus {
	switch {
	case incoming == "" || !incoming.Valid():
		return current
	case current == "":
		return incoming
	case current.Closed():
		return current
	case disputeRank(incoming) < disputeRank(current):
		return current
	}
	return incoming
}

// disputeUpdate is what the provider (or an administrator) says about a dispute.
type disputeUpdate struct {
	Ref            string
	Status         domain.DisputeStatus // "" keeps the current one (needs_response for a new dispute)
	Reason         string
	AmountCents    int64
	Currency       string
	EvidenceDueBy  *time.Time
	FundsWithdrawn *bool
}

// disputeEffect reports what applying an update did, for the work that follows
// the commit (wallet event, audit, alerts).
type disputeEffect struct {
	dispute  *domain.PaymentDispute
	created  bool
	opened   bool // the chargeback took effect now
	won      bool
	lost     bool
	wallet   *domain.Wallet
	canceled *domain.Subscription
}

// webhookDispute applies a dispute event to the payment's dispute.
func (s *BillingService) webhookDispute(ctx context.Context, t *domain.PaymentTransaction, eventID string, evt *payment.WebhookEvent) (webhookResult, error) {
	if s.ops == nil {
		return ignoredEvent("disputes are not enabled"), nil
	}
	u := disputeUpdate{Ref: truncateString(firstNonBlank(evt.DisputeRef, evt.ObjectRef, "evt:"+eventID), 200),
		Status: domain.DisputeStatus(evt.DisputeStatus), Reason: truncateString(evt.DisputeReason, 100),
		AmountCents: evt.AmountCents, Currency: evt.Currency, EvidenceDueBy: evt.EvidenceDueBy}
	if !u.Status.Valid() {
		u.Status = ""
	}
	switch evt.Type {
	case payment.EventDisputeFundsWithdrawn:
		v := true
		u.FundsWithdrawn = &v
	case payment.EventDisputeFundsReinstated:
		v := false
		u.FundsWithdrawn = &v
	case payment.EventDisputeClosed:
		if u.Status == "" || !u.Status.Closed() {
			return ignoredEvent("a closed dispute without a closing status"), nil
		}
	case payment.EventChargeDisputed:
		if u.Status == "" {
			u.Status = domain.DisputeNeedsResponse
		}
	}
	_, err := s.advanceDispute(ctx, t, u)
	return appliedEvent(), err
}

// advanceDispute applies an update to the dispute of a payment, creating the
// dispute when the payment has none yet, and returns the dispute as stored.
func (s *BillingService) advanceDispute(ctx context.Context, t *domain.PaymentTransaction, u disputeUpdate) (*disputeEffect, error) {
	var eff *disputeEffect
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		eff = &disputeEffect{}
		locked, err := s.repo.GetTransactionForUpdate(ctx, t.ID) // serialises every event of this payment
		if err != nil {
			return err
		}
		return s.applyDisputeUpdate(ctx, locked, u, eff)
	})
	if err != nil {
		return nil, mapRepoErr(err, "Dispute")
	}
	s.afterDispute(ctx, t, eff)
	return eff, nil
}

// applyDisputeUpdate is advanceDispute's work: the payment is locked.
func (s *BillingService) applyDisputeUpdate(ctx context.Context, locked *domain.PaymentTransaction, u disputeUpdate, eff *disputeEffect) error {
	now := s.clock.Now()
	d, err := s.ops.repo.GetDisputeByRef(ctx, locked.Provider, u.Ref)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		d = &domain.PaymentDispute{ID: domain.NewID(), Provider: locked.Provider, DisputeRef: u.Ref, TransactionID: locked.ID,
			UserID: locked.UserID, AmountCents: locked.AmountCents, Currency: locked.Currency, Status: domain.DisputeNeedsResponse,
			CreatedAt: now, UpdatedAt: now}
		if u.Status != "" {
			d.Status = u.Status
		}
		eff.created = true
	case err != nil:
		return err
	}
	var before domain.DisputeStatus // what the dispute read before this update ("" for a new one)
	if !eff.created {
		before = d.Status
		d.Status = nextDisputeStatus(d.Status, u.Status)
	}
	if u.Reason != "" {
		d.Reason = u.Reason
	}
	if u.AmountCents > 0 {
		d.AmountCents = u.AmountCents
	}
	if c := strings.ToUpper(strings.TrimSpace(u.Currency)); currencyCode.MatchString(c) {
		d.Currency = c
	}
	if u.EvidenceDueBy != nil {
		d.EvidenceDueBy = u.EvidenceDueBy
	}
	if u.FundsWithdrawn != nil {
		d.FundsWithdrawn = *u.FundsWithdrawn
	}
	// The chargeback takes effect (once): it opened, an inquiry escalated, or a
	// dispute we never saw open was lost.
	if (d.Status.Open() || d.Status == domain.DisputeLost) && !d.Activated() {
		if err := s.activateDispute(ctx, locked, d, now, eff); err != nil {
			return err
		}
		eff.opened = true
	}
	if d.Status == domain.DisputeWon && d.Activated() {
		w, err := s.restoreDisputedCredits(ctx, locked, d, now)
		if err != nil {
			return err
		}
		eff.wallet = firstWallet(w, eff.wallet)
		eff.won = before != domain.DisputeWon
	}
	eff.lost = d.Status == domain.DisputeLost && before != domain.DisputeLost
	if d.Status.Closed() && d.ClosedAt == nil {
		d.ClosedAt = &now
	}
	d.UpdatedAt = now
	if eff.created {
		if err := s.ops.repo.CreateDispute(ctx, d); err != nil {
			return err
		}
	} else if err := s.ops.repo.UpdateDispute(ctx, d.ID, disputeFields(d)); err != nil {
		return err
	}
	eff.dispute = d
	return nil
}

func firstWallet(a, b *domain.Wallet) *domain.Wallet {
	if a != nil {
		return a
	}
	return b
}

// disputeFields are the columns an update rewrites.
func disputeFields(d *domain.PaymentDispute) map[string]any {
	return map[string]any{"status": d.Status, "reason": d.Reason, "amount_cents": d.AmountCents, "currency": d.Currency,
		"prior_status": d.PriorStatus, "evidence_due_by": d.EvidenceDueBy, "evidence_submitted_at": d.EvidenceSubmittedAt,
		"credits_frozen": d.CreditsFrozen, "credits_restored": d.CreditsRestored, "funds_withdrawn": d.FundsWithdrawn,
		"activated_at": d.ActivatedAt, "closed_at": d.ClosedAt, "updated_at": d.UpdatedAt}
}

// activateDispute applies a chargeback's effects to the locked payment.
func (s *BillingService) activateDispute(ctx context.Context, t *domain.PaymentTransaction, d *domain.PaymentDispute, now time.Time, eff *disputeEffect) error {
	prior := t.Status
	if prior == domain.TxDisputed {
		prior = domain.TxSucceeded // another dispute of the payment is already in effect
	}
	d.PriorStatus, d.ActivatedAt = prior, &now
	if t.Status != domain.TxDisputed {
		if err := s.repo.UpdateTransaction(ctx, t.ID, map[string]any{"status": domain.TxDisputed, "updated_at": now}); err != nil {
			return err
		}
	}
	if t.SubscriptionID != nil {
		sub, err := s.repo.GetSubscriptionForUpdate(ctx, *t.SubscriptionID)
		if err != nil {
			return err
		}
		if sub.Status.Open() {
			if err := s.endSubscription(ctx, sub, domain.SubCanceled, "dispute", now, now); err != nil {
				return err
			}
			eff.canceled = sub
		}
	}
	if !s.policies(ctx).DisputeClawback || prior == domain.TxPending || prior == domain.TxFailed {
		return nil
	}
	w, credits, err := s.freezeDisputedCredits(ctx, t, d)
	if err != nil {
		return err
	}
	d.CreditsFrozen, eff.wallet = credits, w
	return nil
}

// freezeDisputedCredits takes the unused credits of the payment's purchase lot
// back (ledger dispute_reversal): never more than the lot holds or the wallet
// has, so never below zero, and never other purchases' or bonus credits.
func (s *BillingService) freezeDisputedCredits(ctx context.Context, t *domain.PaymentTransaction, d *domain.PaymentDispute) (*domain.Wallet, int64, error) {
	w, lot, err := s.lockedPurchaseLot(ctx, t)
	if err != nil || lot == nil {
		return nil, 0, err
	}
	credits := min(lot.CreditsRemaining, w.Balance)
	if credits <= 0 {
		return nil, 0, nil
	}
	w, _, err = s.wallet.Apply(ctx, Movement{UserID: t.UserID, Type: domain.LedgerDisputeReversal, Amount: -credits,
		ReferenceType: "credit_purchase", ReferenceID: t.CreditPurchaseID, IdempotencyKey: disputeReversalKey(d.ID),
		Description: "Payment disputed", ConsumeLot: &lot.ID})
	return w, credits, err
}

func disputeReversalKey(id uuid.UUID) string { return "dispute_reversal:" + id.String() }

// restoreDisputedCredits gives back what a won dispute had frozen (into the lot
// it came from) and puts the payment's earlier status back. Idempotent.
func (s *BillingService) restoreDisputedCredits(ctx context.Context, t *domain.PaymentTransaction, d *domain.PaymentDispute, now time.Time) (*domain.Wallet, error) {
	var wallet *domain.Wallet
	if owed := d.CreditsFrozen - d.CreditsRestored; owed > 0 {
		w, _, err := s.wallet.Apply(ctx, Movement{UserID: t.UserID, Type: domain.LedgerDisputeRestore, Amount: owed,
			ReferenceType: "credit_purchase", ReferenceID: t.CreditPurchaseID, IdempotencyKey: "dispute_restore:" + d.ID.String(),
			Description: "Payment dispute won", RestoreKey: disputeReversalKey(d.ID)})
		if err != nil {
			return nil, err
		}
		d.CreditsRestored, wallet = d.CreditsFrozen, w
	}
	if t.Status == domain.TxDisputed {
		prior := d.PriorStatus
		if prior == "" || prior == domain.TxDisputed {
			prior = domain.TxSucceeded
		}
		if err := s.repo.UpdateTransaction(ctx, t.ID, map[string]any{"status": prior, "updated_at": now}); err != nil {
			return nil, err
		}
	}
	return wallet, nil
}

// afterDispute does what follows the commit of a dispute update: the wallet
// event, the membership notice, the audit records and the alerts.
func (s *BillingService) afterDispute(ctx context.Context, t *domain.PaymentTransaction, eff *disputeEffect) {
	d := eff.dispute
	if d == nil {
		return
	}
	dctx := database.Detach(ctx)
	if eff.wallet != nil {
		warnErr(dctx, "billing: publish wallet", s.events.Publish(dctx, realtime.EventWalletUpdated, walletView(eff.wallet), d.UserID))
	}
	if sub := eff.canceled; sub != nil {
		s.notify.Notify(dctx, NotificationSpec{Kind: NotifySubscriptionCanceled, Data: map[string]any{"subscription_id": sub.ID,
			"reason": "dispute"}, Link: "/profile/settings"}, sub.UserID)
	}
	meta := map[string]any{"dispute_ref": d.DisputeRef, "status": d.Status, "amount_cents": d.AmountCents, "currency": d.Currency,
		"reason": d.Reason, "credits_frozen": d.CreditsFrozen, "credits_restored": d.CreditsRestored, "user_id": d.UserID}
	switch {
	case eff.opened:
		s.record(dctx, "dispute.opened", "payment_dispute", d.ID.String(), meta)
		s.alert(dctx, "dispute_opened:"+d.ID.String(), "Payment dispute opened", s.disputeLines(t, d,
			fmt.Sprintf("Credits taken back: %d.", d.CreditsFrozen)), "/payments/disputes/"+d.ID.String())
		s.track(dctx, "billing.dispute_opened", map[string]any{"provider": d.Provider, "reason": d.Reason})
	case eff.created && d.Status.Inquiry():
		s.record(dctx, "dispute.inquiry", "payment_dispute", d.ID.String(), meta)
	}
	switch {
	case eff.won:
		s.record(dctx, "dispute.won", "payment_dispute", d.ID.String(), meta)
	case eff.lost:
		s.record(dctx, "dispute.lost", "payment_dispute", d.ID.String(), meta)
		s.alert(dctx, "dispute_lost:"+d.ID.String(), "Payment dispute lost", s.disputeLines(t, d,
			"The funds stay with the cardholder and no credits were restored."), "/payments/disputes/"+d.ID.String())
	}
}

// disputeLines are the facts of a dispute for an alert.
func (s *BillingService) disputeLines(t *domain.PaymentTransaction, d *domain.PaymentDispute, extra string) []string {
	lines := []string{"Dispute: " + d.DisputeRef, "Payment: " + t.ID.String(), "Customer: " + d.UserID.String(),
		"Amount: " + money(d.AmountCents, d.Currency)}
	if d.Reason != "" {
		lines = append(lines, "Reason: "+d.Reason)
	}
	if d.EvidenceDueBy != nil {
		lines = append(lines, "Evidence due by: "+d.EvidenceDueBy.UTC().Format("2006-01-02 15:04 UTC"))
	}
	return append(lines, extra)
}

// disputeManager returns the provider that answers disputes, or why it cannot.
func (s *BillingService) disputeManager(provider string) (payment.DisputeManager, error) {
	p, ok := s.registry.Get(provider)
	if !ok {
		return nil, apperror.ProviderUnavailable(nil)
	}
	m, ok := p.(payment.DisputeManager)
	if !ok {
		return nil, apperror.InvalidState("This payment provider does not support answering disputes from the console.")
	}
	return m, nil
}
