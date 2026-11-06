package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// Renewals are safe across instances without holding a lock during the
// provider call:
//
//  1. claim (tx): SELECT … FOR UPDATE SKIP LOCKED the due subscriptions,
//     create the invoice and a pending transaction with idempotency key
//     sub:<id>:p<period>:a<attempt>, and lease both for RenewalLease;
//  2. charge the provider with that key (no database lock held);
//  3. apply (tx): lock again, record the outcome, renew or move to past_due.
//
// If an instance dies between 1 and 3, the lease expires and the next run
// reconciles the attempt with the provider (Lookup) or retries it with the
// same key, so a period is never charged twice.

// chargeAttempt is a prepared charge leased to this worker.
type chargeAttempt struct {
	tx             *domain.PaymentTransaction
	subscriptionID uuid.UUID
	periodIndex    int
	method         *domain.PaymentMethod
	reason         payment.ChargeReason
}

// chargeOutcome is the provider's answer.
type chargeOutcome struct {
	result  payment.ChargeResult
	decline *payment.DeclineError
	unknown bool // reconcile later (timeout, pending, 3-D Secure)
}

// execute charges the attempt's card.
func (s *BillingService) execute(ctx context.Context, a *chargeAttempt) chargeOutcome {
	if a.method == nil {
		return chargeOutcome{decline: &payment.DeclineError{Code: "payment_method_required"}}
	}
	charger, ok := s.storedCharger(a.method.Provider)
	if !ok {
		return chargeOutcome{decline: &payment.DeclineError{Code: "provider_unavailable", Soft: true}}
	}
	customerRef, methodRef, networkRef, err := s.refs(ctx, a.method)
	if err != nil {
		logger.FromContext(ctx).Error("billing: card references unavailable", zap.Error(err), zap.String("method_id", a.method.ID.String()))
		return chargeOutcome{decline: &payment.DeclineError{Code: "payment_method_unavailable"}}
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	res, err := charger.ChargeStored(cctx, payment.StoredCharge{CustomerRef: customerRef, MethodRef: methodRef, AmountCents: a.tx.AmountCents,
		Currency: a.tx.Currency, IdempotencyKey: a.tx.IdempotencyKey, Reason: a.reason, NetworkTxnRef: networkRef, Description: a.tx.Description})
	cancel()
	if d, ok := payment.AsDecline(err); ok {
		return chargeOutcome{decline: d}
	}
	if err != nil {
		logger.FromContext(ctx).Warn("billing: charge outcome unknown; will reconcile", zap.Error(err),
			zap.String("transaction_id", a.tx.ID.String()))
		return chargeOutcome{unknown: true}
	}
	if res.Status != payment.ChargeSucceeded {
		return chargeOutcome{result: res, unknown: true}
	}
	return chargeOutcome{result: res}
}

// applySubscriptionOutcome records a membership charge's result. Unknown
// outcomes stay pending under their lease for reconciliation.
func (s *BillingService) applySubscriptionOutcome(ctx context.Context, a *chargeAttempt, out chargeOutcome) (*domain.Subscription, error) {
	if out.unknown {
		return nil, nil
	}
	now := s.clock.Now()
	var sub *domain.Subscription
	var notifyKind string
	var notifyData map[string]any
	var wallet *domain.Wallet
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if sub, err = s.repo.GetSubscriptionForUpdate(ctx, a.subscriptionID); err != nil {
			return err
		}
		t, err := s.repo.GetTransactionForUpdate(ctx, a.tx.ID)
		if err != nil {
			return err
		}
		if t.Status != domain.TxPending { // already applied (webhook, another worker)
			return s.repo.UpdateSubscription(ctx, sub.ID, map[string]any{"locked_until": nil, "updated_at": now})
		}
		inv, err := s.repo.GetInvoice(ctx, sub.ID, a.periodIndex)
		if err != nil {
			return err
		}
		v, err := s.repo.GetVersion(ctx, sub.PlanVersionID)
		if err != nil {
			return err
		}
		initial := t.Kind == domain.TxSubscriptionInitial
		if out.decline == nil {
			ref := out.result.ProviderRef
			if err := s.repo.UpdateTransaction(ctx, t.ID, map[string]any{"status": domain.TxSucceeded, "provider_ref": nonEmpty(ref),
				"locked_until": nil, "failure_code": "", "updated_at": now}); err != nil {
				return err
			}
			if err := s.repo.UpdateInvoice(ctx, inv.ID, map[string]any{"status": domain.InvoicePaid, "paid_at": now, "updated_at": now}); err != nil {
				return err
			}
			if !sub.Status.Open() {
				// Paid after the membership ended (late reconciliation): keep
				// the record for a refund, do not resurrect the membership.
				logger.FromContext(ctx).Error("billing: charge succeeded for an ended membership; refund it",
					zap.String("subscription_id", sub.ID.String()), zap.String("transaction_id", t.ID.String()))
				return nil
			}
			grantIdx := sub.GrantIndex + 1
			var nextGrant any
			if g := domain.AddMonthsAnchored(inv.PeriodStart, 1, sub.BillingAnchorDay); g.Before(inv.PeriodEnd) {
				nextGrant = g
			}
			fields := map[string]any{"status": domain.SubActive, "period_index": inv.PeriodIndex, "current_period_start": inv.PeriodStart,
				"current_period_end": inv.PeriodEnd, "next_attempt_at": inv.PeriodEnd, "attempt_count": 0, "grace_until": nil,
				"locked_until": nil, "grant_index": grantIdx, "next_grant_at": nextGrant, "updated_at": now}
			if err := s.repo.UpdateSubscription(ctx, sub.ID, fields); err != nil {
				return err
			}
			if wallet, err = s.grant(ctx, sub, v, grantIdx, inv.PeriodIndex, now); err != nil {
				return err
			}
			if a.method != nil && out.result.NetworkTxnRef != "" && a.method.NetworkTxnRefEnc == "" {
				if sealed, err := s.sealer.Seal(out.result.NetworkTxnRef, networkAD(a.method)); err == nil {
					if err := s.repo.UpdateMethod(ctx, a.method.ID, map[string]any{"network_txn_ref_enc": sealed, "updated_at": now}); err != nil {
						return err
					}
				}
			}
			if !initial {
				notifyKind = NotifySubscriptionRenewed
				notifyData = map[string]any{"subscription_id": sub.ID, "period_end": inv.PeriodEnd, "amount_cents": t.AmountCents,
					"currency": t.Currency}
			}
			return nil
		}
		// Declined.
		d := out.decline
		if err := s.repo.UpdateTransaction(ctx, t.ID, map[string]any{"status": domain.TxFailed, "failure_code": truncateString(d.Code, 64),
			"locked_until": nil, "updated_at": now}); err != nil {
			return err
		}
		if !d.Soft && a.method != nil {
			if err := s.repo.UpdateMethod(ctx, a.method.ID, map[string]any{"status": domain.MethodFailed, "updated_at": now}); err != nil &&
				!errors.Is(err, repository.ErrNotFound) {
				return err
			}
		}
		if initial {
			if err := s.repo.UpdateInvoice(ctx, inv.ID, map[string]any{"status": domain.InvoiceFailed, "updated_at": now}); err != nil {
				return err
			}
			if !sub.Status.Open() {
				return nil
			}
			return s.endSubscription(ctx, sub, domain.SubExpired, "payment_failed", now, now)
		}
		if !sub.Status.Open() {
			return nil
		}
		attempts := inv.AttemptCount
		var next any
		if d.Soft {
			if at, ok := domain.NextRetryAt(inv.PeriodStart, attempts); ok {
				next = maxTime(at, now)
			}
		}
		invStatus := domain.InvoiceOpen
		if next == nil {
			invStatus = domain.InvoiceFailed
		}
		if err := s.repo.UpdateInvoice(ctx, inv.ID, map[string]any{"status": invStatus, "updated_at": now}); err != nil {
			return err
		}
		grace := inv.PeriodStart.Add(s.cfg.GracePeriod)
		if sub.GraceUntil != nil {
			grace = *sub.GraceUntil
		}
		if err := s.repo.UpdateSubscription(ctx, sub.ID, map[string]any{"status": domain.SubPastDue, "attempt_count": attempts,
			"next_attempt_at": next, "grace_until": grace, "locked_until": nil, "updated_at": now}); err != nil {
			return err
		}
		if attempts == 1 || !d.Soft {
			notifyKind = NotifySubscriptionPaymentFailed
			notifyData = map[string]any{"subscription_id": sub.ID, "reason": d.Code, "retry": next != nil, "grace_until": grace}
		}
		return nil
	})
	if err != nil {
		return nil, mapRepoErr(err, "Membership")
	}
	ctx = database.Detach(ctx)
	if wallet != nil {
		warnErr(ctx, "billing: publish wallet", s.events.Publish(ctx, realtime.EventWalletUpdated, walletView(wallet), sub.UserID))
	}
	if notifyKind != "" {
		s.notify.Notify(ctx, NotificationSpec{Kind: notifyKind, Data: notifyData, Link: "/profile/settings"}, sub.UserID)
	}
	return sub, nil
}

func nonEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// grant applies one monthly membership grant (credits and free minutes)
// exactly once. Unused free minutes of the previous grant expire: granted
// minutes are assumed to be used first, so promotional minutes the user
// had before are never taken away. Runs inside a transaction.
func (s *BillingService) grant(ctx context.Context, sub *domain.Subscription, v *domain.PlanVersion, grantIndex, periodIndex int,
	now time.Time) (*domain.Wallet, error) {
	if _, err := s.repo.GetGrant(ctx, sub.ID, grantIndex); err == nil {
		return nil, nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if v.MonthlyCreditGrant == 0 && v.MonthlyFreeMinutes == 0 {
		return nil, s.repo.CreateGrant(ctx, &domain.SubscriptionGrant{ID: domain.NewID(), SubscriptionID: sub.ID, UserID: sub.UserID,
			GrantIndex: grantIndex, PeriodIndex: periodIndex, GrantedAt: now, CreatedAt: now, UpdatedAt: now})
	}
	g := &domain.SubscriptionGrant{ID: domain.NewID(), SubscriptionID: sub.ID, UserID: sub.UserID, GrantIndex: grantIndex,
		PeriodIndex: periodIndex, Credits: v.MonthlyCreditGrant, FreeMinutes: v.MonthlyFreeMinutes, GrantedAt: now, CreatedAt: now, UpdatedAt: now}
	w, err := s.wallets.GetForUpdate(ctx, sub.UserID)
	if err != nil {
		return nil, err
	}
	if v.MonthlyFreeMinutes > 0 {
		if grantIndex > 1 {
			if err := s.revokeUnusedMinutes(ctx, sub.ID, grantIndex-1, w, now); err != nil {
				return nil, err
			}
		}
		w.FreeMinutes += v.MonthlyFreeMinutes
		w.UpdatedAt = now
		if err := s.wallets.Save(ctx, w); err != nil {
			return nil, err
		}
		g.FreeMinutesAfter = w.FreeMinutes
	}
	if err := s.repo.CreateGrant(ctx, g); err != nil {
		return nil, err
	}
	if v.MonthlyCreditGrant > 0 {
		w, _, err = s.wallet.Apply(ctx, Movement{UserID: sub.UserID, Type: domain.LedgerSubscriptionGrant, Amount: v.MonthlyCreditGrant,
			ReferenceType: "subscription", ReferenceID: &sub.ID, IdempotencyKey: fmt.Sprintf("subgrant:%s:%d", sub.ID, grantIndex),
			Description: "Membership credits"})
		if err != nil {
			return nil, err
		}
	}
	return w, nil
}

// revokeUnusedMinutes expires what is left of grant #index's free minutes
// (bounded by the wallet balance). w must be locked; the caller saves it.
func (s *BillingService) revokeUnusedMinutes(ctx context.Context, subID uuid.UUID, index int, w *domain.Wallet, now time.Time) error {
	prev, err := s.repo.GetGrant(ctx, subID, index)
	if errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if prev.FreeMinutes == 0 || prev.FreeMinutesRevoked > 0 {
		return nil
	}
	used := max(prev.FreeMinutesAfter-w.FreeMinutes, 0)
	revoke := min(max(prev.FreeMinutes-used, 0), w.FreeMinutes)
	if revoke == 0 {
		return nil
	}
	w.FreeMinutes -= revoke
	w.UpdatedAt = now
	return s.repo.UpdateGrant(ctx, prev.ID, map[string]any{"free_minutes_revoked": revoke, "updated_at": now})
}

// expireGrantedMinutes removes the unused free minutes of the latest grant
// when a membership ends.
func (s *BillingService) expireGrantedMinutes(ctx context.Context, sub *domain.Subscription, now time.Time) error {
	if sub.GrantIndex == 0 {
		return nil
	}
	prev, err := s.repo.GetGrant(ctx, sub.ID, sub.GrantIndex)
	if errors.Is(err, repository.ErrNotFound) || (err == nil && prev.FreeMinutes == 0) {
		return nil
	}
	if err != nil {
		return err
	}
	w, err := s.wallets.GetForUpdate(ctx, sub.UserID)
	if err != nil {
		return err
	}
	before := w.FreeMinutes
	if err := s.revokeUnusedMinutes(ctx, sub.ID, sub.GrantIndex, w, now); err != nil {
		return err
	}
	if w.FreeMinutes == before {
		return nil
	}
	return s.wallets.Save(ctx, w)
}

// claimRenewal leases one due subscription (used when a member replaces a
// declined card and asks to pay now).
func (s *BillingService) claimRenewal(ctx context.Context, subID uuid.UUID, now time.Time) (*chargeAttempt, error) {
	var a *chargeAttempt
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		sub, err := s.repo.GetSubscriptionForUpdate(ctx, subID)
		if err != nil {
			return err
		}
		if (sub.Status != domain.SubActive && sub.Status != domain.SubPastDue) || sub.NextAttemptAt == nil || sub.NextAttemptAt.After(now) ||
			(sub.LockedUntil != nil && sub.LockedUntil.After(now)) {
			return nil
		}
		a, err = s.prepareRenewal(ctx, sub, now)
		return err
	})
	if err != nil {
		return nil, mapRepoErr(err, "Membership")
	}
	return a, nil
}

// prepareRenewal creates (or reuses) the pending charge for the next
// period of a locked subscription and leases it.
func (s *BillingService) prepareRenewal(ctx context.Context, sub *domain.Subscription, now time.Time) (*chargeAttempt, error) {
	if sub.CurrentPeriodEnd == nil {
		return nil, nil
	}
	v, err := s.repo.GetVersion(ctx, sub.PlanVersionID)
	if err != nil {
		return nil, err
	}
	periodIdx := sub.PeriodIndex + 1
	periodStart := *sub.CurrentPeriodEnd
	periodEnd := domain.AddMonthsAnchored(periodStart, v.PeriodMonths(), sub.BillingAnchorDay)
	lease := now.Add(s.cfg.RenewalLease)
	inv, err := s.repo.GetInvoice(ctx, sub.ID, periodIdx)
	if errors.Is(err, repository.ErrNotFound) {
		inv = &domain.SubscriptionInvoice{ID: domain.NewID(), SubscriptionID: sub.ID, UserID: sub.UserID, PeriodIndex: periodIdx,
			PeriodStart: periodStart, PeriodEnd: periodEnd, AmountCents: v.PriceCents, Currency: v.Currency, Status: domain.InvoiceOpen,
			CreatedAt: now, UpdatedAt: now}
		if err := s.repo.CreateInvoice(ctx, inv); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	var method *domain.PaymentMethod
	if sub.PaymentMethodID != nil {
		if m, err := s.repo.GetMethodAnyState(ctx, *sub.PaymentMethodID); err == nil && !m.DeletedAt.Valid {
			method = m
		}
	}
	// A pending attempt left by a crashed worker is retried with its own
	// idempotency key, never as a new attempt.
	if inv.PaymentTransactionID != nil {
		t, err := s.repo.GetTransactionForUpdate(ctx, *inv.PaymentTransactionID)
		if err != nil {
			return nil, err
		}
		if t.Status == domain.TxPending {
			if err := s.lease(ctx, sub.ID, t.ID, lease, now); err != nil {
				return nil, err
			}
			if t.PaymentMethodID != nil {
				if m, err := s.repo.GetMethodAnyState(ctx, *t.PaymentMethodID); err == nil {
					method = m
				}
			}
			return &chargeAttempt{tx: t, subscriptionID: sub.ID, periodIndex: periodIdx, method: method, reason: payment.ReasonRecurring}, nil
		}
	}
	if method == nil || !method.Usable(now) {
		// Nothing to charge: wait for a new card until the grace period ends.
		grace := periodStart.Add(s.cfg.GracePeriod)
		if sub.GraceUntil != nil {
			grace = *sub.GraceUntil
		}
		if err := s.repo.UpdateSubscription(ctx, sub.ID, map[string]any{"status": domain.SubPastDue, "next_attempt_at": nil,
			"grace_until": grace, "updated_at": now}); err != nil {
			return nil, err
		}
		return nil, s.repo.UpdateInvoice(ctx, inv.ID, map[string]any{"status": domain.InvoiceFailed, "updated_at": now})
	}
	attempt := inv.AttemptCount + 1
	desc := "Membership"
	if plan, err := s.repo.GetPlan(ctx, sub.PlanID); err == nil {
		desc = plan.Name.For("en")
	}
	t := &domain.PaymentTransaction{ID: domain.NewID(), UserID: sub.UserID, Kind: domain.TxSubscriptionRenewal, Status: domain.TxPending,
		AmountCents: v.PriceCents, Currency: v.Currency, Provider: method.Provider, PaymentMethodID: &method.ID, SubscriptionID: &sub.ID,
		InvoiceID: &inv.ID, Description: desc, IdempotencyKey: fmt.Sprintf("sub:%s:p%d:a%d", sub.ID, periodIdx, attempt), AttemptNo: attempt,
		LockedUntil: &lease, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateTransaction(ctx, t); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateInvoice(ctx, inv.ID, map[string]any{"attempt_count": attempt, "payment_transaction_id": t.ID,
		"status": domain.InvoiceOpen, "updated_at": now}); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateSubscription(ctx, sub.ID, map[string]any{"locked_until": lease, "updated_at": now}); err != nil {
		return nil, err
	}
	return &chargeAttempt{tx: t, subscriptionID: sub.ID, periodIndex: periodIdx, method: method, reason: payment.ReasonRecurring}, nil
}

func (s *BillingService) lease(ctx context.Context, subID, txID uuid.UUID, until, now time.Time) error {
	if err := s.repo.UpdateTransaction(ctx, txID, map[string]any{"locked_until": until, "updated_at": now}); err != nil {
		return err
	}
	return s.repo.UpdateSubscription(ctx, subID, map[string]any{"locked_until": until, "updated_at": now})
}

// RunDue runs the membership jobs once: reconciliation of lost charge
// responses, period-end cancellations and renewals (with dunning retries),
// monthly grants, grace-period and incomplete expiries. It is safe to run
// on every instance concurrently.
func (s *BillingService) RunDue(ctx context.Context, limit int) (int, error) {
	now := s.clock.Now()
	total := 0
	var errs []error
	for _, job := range []struct {
		name string
		run  func(context.Context, time.Time, int) (int, error)
	}{
		{"reconcile", s.reconcileStale},
		{"renewals", s.runRenewals},
		{"grants", s.runGrants},
		{"grace", s.expirePastDue},
		{"incomplete", s.expireIncomplete},
	} {
		n, err := job.run(ctx, now, limit)
		total += n
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", job.name, err))
		}
	}
	return total, errors.Join(errs...)
}

func (s *BillingService) runRenewals(ctx context.Context, now time.Time, limit int) (int, error) {
	var attempts []*chargeAttempt
	var ended []domain.Subscription
	var waiting []domain.Subscription
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		subs, err := s.repo.ClaimDueRenewals(ctx, now, limit)
		if err != nil {
			return err
		}
		for i := range subs {
			sub := &subs[i]
			if sub.Status == domain.SubActive && sub.CancelAtPeriodEnd && sub.CurrentPeriodEnd != nil && !sub.CurrentPeriodEnd.After(now) {
				if err := s.endSubscription(ctx, sub, domain.SubCanceled, "", *sub.CurrentPeriodEnd, now); err != nil {
					return err
				}
				ended = append(ended, *sub)
				continue
			}
			a, err := s.prepareRenewal(ctx, sub, now)
			if err != nil {
				return err
			}
			if a == nil {
				if sub.Status == domain.SubActive {
					waiting = append(waiting, *sub)
				}
				continue
			}
			attempts = append(attempts, a)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	ctx = database.Detach(ctx)
	for _, sub := range ended {
		s.notify.Notify(ctx, NotificationSpec{Kind: NotifySubscriptionCanceled, Data: map[string]any{"subscription_id": sub.ID,
			"reason": "period_ended"}, Link: "/profile/settings"}, sub.UserID)
	}
	for _, sub := range waiting {
		s.notify.Notify(ctx, NotificationSpec{Kind: NotifySubscriptionPaymentFailed, Data: map[string]any{"subscription_id": sub.ID,
			"reason": "payment_method_required", "retry": false}, Link: "/profile/settings"}, sub.UserID)
	}
	var errs []error
	for _, a := range attempts {
		out := s.execute(ctx, a)
		if _, err := s.applySubscriptionOutcome(ctx, a, out); err != nil {
			errs = append(errs, err)
		}
	}
	return len(ended) + len(waiting) + len(attempts), errors.Join(errs...)
}

func (s *BillingService) runGrants(ctx context.Context, now time.Time, limit int) (int, error) {
	n := 0
	var wallets []*domain.Wallet
	var owners []uuid.UUID
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		subs, err := s.repo.ClaimDueGrants(ctx, now, limit)
		if err != nil {
			return err
		}
		for i := range subs {
			sub := &subs[i]
			if sub.CurrentPeriodEnd == nil || sub.NextGrantAt == nil || !sub.NextGrantAt.Before(*sub.CurrentPeriodEnd) {
				if err := s.repo.UpdateSubscription(ctx, sub.ID, map[string]any{"next_grant_at": nil, "updated_at": now}); err != nil {
					return err
				}
				continue
			}
			v, err := s.repo.GetVersion(ctx, sub.PlanVersionID)
			if err != nil {
				return err
			}
			idx := sub.GrantIndex + 1
			w, err := s.grant(ctx, sub, v, idx, sub.PeriodIndex, now)
			if err != nil {
				return err
			}
			var next any
			if g := domain.AddMonthsAnchored(*sub.NextGrantAt, 1, sub.BillingAnchorDay); g.Before(*sub.CurrentPeriodEnd) {
				next = g
			}
			if err := s.repo.UpdateSubscription(ctx, sub.ID, map[string]any{"grant_index": idx, "next_grant_at": next, "updated_at": now}); err != nil {
				return err
			}
			if w != nil {
				wallets, owners = append(wallets, w), append(owners, sub.UserID)
			}
			n++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	ctx = database.Detach(ctx)
	for i, w := range wallets {
		warnErr(ctx, "billing: publish wallet", s.events.Publish(ctx, realtime.EventWalletUpdated, walletView(w), owners[i]))
	}
	return n, nil
}

func (s *BillingService) expirePastDue(ctx context.Context, now time.Time, limit int) (int, error) {
	var ended []domain.Subscription
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		subs, err := s.repo.ClaimGraceExpired(ctx, now, limit)
		if err != nil {
			return err
		}
		for i := range subs {
			if err := s.endSubscription(ctx, &subs[i], domain.SubExpired, "payment_failed", now, now); err != nil {
				return err
			}
			if inv, err := s.repo.GetInvoice(ctx, subs[i].ID, subs[i].PeriodIndex+1); err == nil && inv.Status == domain.InvoiceOpen {
				if err := s.repo.UpdateInvoice(ctx, inv.ID, map[string]any{"status": domain.InvoiceVoid, "updated_at": now}); err != nil {
					return err
				}
			}
			ended = append(ended, subs[i])
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	ctx = database.Detach(ctx)
	for _, sub := range ended {
		s.notify.Notify(ctx, NotificationSpec{Kind: NotifySubscriptionCanceled, Data: map[string]any{"subscription_id": sub.ID,
			"reason": "payment_failed"}, Link: "/profile/settings"}, sub.UserID)
	}
	return len(ended), nil
}

// expireIncomplete resolves memberships whose first charge never settled:
// the provider is asked one last time, then the membership expires.
func (s *BillingService) expireIncomplete(ctx context.Context, now time.Time, limit int) (int, error) {
	var stale []domain.Subscription
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		subs, err := s.repo.ClaimStaleIncomplete(ctx, now.Add(-s.cfg.IncompleteTTL), now, limit)
		if err != nil {
			return err
		}
		lease := now.Add(s.cfg.RenewalLease)
		for _, sub := range subs {
			if err := s.repo.UpdateSubscription(ctx, sub.ID, map[string]any{"locked_until": lease, "updated_at": now}); err != nil {
				return err
			}
		}
		stale = subs
		return nil
	})
	if err != nil {
		return 0, err
	}
	n := 0
	for i := range stale {
		sub := &stale[i]
		inv, err := s.repo.GetInvoice(ctx, sub.ID, 1)
		resolved := false
		if err == nil && inv.PaymentTransactionID != nil {
			if t, err := s.repo.GetTransaction(ctx, *inv.PaymentTransactionID); err == nil && t.Status == domain.TxPending {
				a := s.attemptFor(ctx, t)
				out := s.resolve(ctx, a)
				if !out.unknown {
					if _, err := s.applySubscriptionOutcome(ctx, a, out); err == nil {
						resolved = true
					}
				}
			}
		}
		if resolved {
			n++
			continue
		}
		err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
			locked, err := s.repo.GetSubscriptionForUpdate(ctx, sub.ID)
			if err != nil || locked.Status != domain.SubIncomplete {
				return err
			}
			if inv != nil && inv.PaymentTransactionID != nil {
				if err := s.repo.UpdateTransaction(ctx, *inv.PaymentTransactionID, map[string]any{"status": domain.TxFailed,
					"failure_code": "unresolved", "locked_until": nil, "updated_at": now}); err != nil {
					return err
				}
			}
			return s.endSubscription(ctx, locked, domain.SubExpired, "payment_unresolved", now, now)
		})
		if err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// attemptFor rebuilds the context of a pending charge.
func (s *BillingService) attemptFor(ctx context.Context, t *domain.PaymentTransaction) *chargeAttempt {
	a := &chargeAttempt{tx: t, reason: payment.ReasonRecurring}
	if t.Kind == domain.TxSubscriptionInitial || t.Kind == domain.TxCreditPurchase {
		a.reason = payment.ReasonCustomerInitiated
	}
	if t.SubscriptionID != nil {
		a.subscriptionID = *t.SubscriptionID
	}
	if t.InvoiceID != nil && t.SubscriptionID != nil {
		a.periodIndex = 1
		if t.Kind == domain.TxSubscriptionRenewal {
			if sub, err := s.repo.GetSubscription(ctx, *t.SubscriptionID); err == nil {
				a.periodIndex = sub.PeriodIndex + 1
			}
		}
	}
	if t.PaymentMethodID != nil {
		if m, err := s.repo.GetMethodAnyState(ctx, *t.PaymentMethodID); err == nil {
			a.method = m
		}
	}
	return a
}

// resolve learns the outcome of a pending charge: Lookup when the provider
// supports it, otherwise a replay with the same idempotency key (providers
// return the original outcome instead of charging again).
func (s *BillingService) resolve(ctx context.Context, a *chargeAttempt) chargeOutcome {
	if p, ok := s.registry.Get(a.tx.Provider); ok {
		if lk, ok := p.(payment.ChargeLookup); ok {
			cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
			res, err := lk.Lookup(cctx, a.tx.IdempotencyKey)
			cancel()
			switch {
			case err != nil:
				return chargeOutcome{unknown: true}
			case res.Found && res.Decline != nil:
				return chargeOutcome{decline: res.Decline}
			case res.Found && res.Result.Status == payment.ChargeSucceeded:
				return chargeOutcome{result: res.Result}
			case res.Found:
				return chargeOutcome{result: res.Result, unknown: true}
			}
		}
	}
	if a.method == nil {
		return chargeOutcome{unknown: true}
	}
	return s.execute(ctx, a)
}

// reconcileStale settles charge attempts whose lease expired without an
// outcome.
func (s *BillingService) reconcileStale(ctx context.Context, now time.Time, limit int) (int, error) {
	var rows []domain.PaymentTransaction
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if rows, err = s.repo.ClaimStalePending(ctx, now, limit); err != nil {
			return err
		}
		lease := now.Add(s.cfg.RenewalLease)
		for _, t := range rows {
			if err := s.repo.UpdateTransaction(ctx, t.ID, map[string]any{"locked_until": lease, "updated_at": now}); err != nil {
				return err
			}
			if t.SubscriptionID != nil {
				if err := s.repo.UpdateSubscription(ctx, *t.SubscriptionID, map[string]any{"locked_until": lease, "updated_at": now}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	n := 0
	var errs []error
	for i := range rows {
		t := &rows[i]
		a := s.attemptFor(ctx, t)
		out := s.resolve(ctx, a)
		if out.unknown {
			if now.Sub(t.CreatedAt) > 24*time.Hour {
				logger.FromContext(ctx).Error("billing: charge still unresolved after 24h; reconcile with the provider dashboard",
					zap.String("transaction_id", t.ID.String()), zap.String("provider", t.Provider))
			}
			continue
		}
		switch t.Kind {
		case domain.TxSubscriptionInitial, domain.TxSubscriptionRenewal:
			if _, err := s.applySubscriptionOutcome(ctx, a, out); err != nil {
				errs = append(errs, err)
				continue
			}
		case domain.TxCreditPurchase:
			if err := s.applyPurchaseOutcome(ctx, t, out); err != nil {
				errs = append(errs, err)
				continue
			}
		}
		n++
	}
	return n, errors.Join(errs...)
}

// applyPurchaseOutcome finishes a credit purchase charged with a saved
// card or token whose outcome was learned asynchronously.
func (s *BillingService) applyPurchaseOutcome(ctx context.Context, t *domain.PaymentTransaction, out chargeOutcome) error {
	if t.CreditPurchaseID == nil || s.settler == nil {
		return nil
	}
	if out.decline != nil {
		if err := s.SettlePurchaseTransaction(ctx, *t.CreditPurchaseID, domain.TxFailed, "", out.decline.Code); err != nil {
			return err
		}
		return s.settler.FailPurchase(ctx, *t.CreditPurchaseID, out.decline.Code)
	}
	return s.settler.CompletePurchase(ctx, *t.CreditPurchaseID, out.result.ProviderRef)
}
