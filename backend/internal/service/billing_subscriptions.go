package service

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// subscriptionView renders a membership with its plan and card.
func (s *BillingService) subscriptionView(ctx context.Context, sub *domain.Subscription, locale string) (SubscriptionView, error) {
	plan, err := s.repo.GetPlan(ctx, sub.PlanID)
	if errors.Is(err, repository.ErrNotFound) {
		plan = &domain.SubscriptionPlan{ID: sub.PlanID, Name: domain.LocalizedText{}, Description: domain.LocalizedText{}}
	} else if err != nil {
		return SubscriptionView{}, err
	}
	v, err := s.repo.GetVersion(ctx, sub.PlanVersionID)
	if err != nil {
		return SubscriptionView{}, err
	}
	view := SubscriptionView{ID: sub.ID, Status: string(sub.Status), Plan: planView(plan, v, locale), CancelAtPeriodEnd: sub.CancelAtPeriodEnd,
		CurrentPeriodStart: sub.CurrentPeriodStart, CurrentPeriodEnd: sub.CurrentPeriodEnd, GraceUntil: sub.GraceUntil,
		CanceledAt: sub.CanceledAt, EndedAt: sub.EndedAt, CreatedAt: sub.CreatedAt}
	switch {
	case sub.Status == domain.SubActive && !sub.CancelAtPeriodEnd:
		view.RenewsAt = sub.CurrentPeriodEnd
	case sub.Status == domain.SubPastDue:
		view.NextRetryAt = sub.NextAttemptAt
	}
	if sub.PaymentMethodID != nil {
		if m, err := s.repo.GetMethodAnyState(ctx, *sub.PaymentMethodID); err == nil && !m.DeletedAt.Valid {
			mv := methodView(m, s.clock.Now())
			view.PaymentMethod = &mv
		}
	}
	return view, nil
}

// Subscription returns the caller's membership: the open one, else the
// most recent (so a lapsed membership can be shown), else nil.
func (s *BillingService) Subscription(ctx context.Context, userID uuid.UUID, locale string) (*SubscriptionView, error) {
	sub, err := s.repo.GetOpenSubscription(ctx, userID)
	if errors.Is(err, repository.ErrNotFound) {
		sub, err = s.repo.LatestSubscription(ctx, userID)
	}
	if errors.Is(err, repository.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	v, err := s.subscriptionView(ctx, sub, locale)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &v, nil
}

// SubscribeInput starts a membership.
type SubscribeInput struct {
	PlanVersionID   uuid.UUID
	PaymentMethodID uuid.UUID
	IdempotencyKey  string
	TrustedCountry  string
	Locale          string
}

// Subscribe starts a membership and charges its first period with a saved
// card (customer-initiated). Replays with the same idempotency key return
// the original membership.
func (s *BillingService) Subscribe(ctx context.Context, userID uuid.UUID, in SubscribeInput) (*SubscriptionResult, error) {
	if s.disabled() {
		return nil, featureOff("subscriptions")
	}
	fields := map[string]string{}
	if !ValidIdempotencyKey(in.IdempotencyKey) {
		fields["idempotency_key"] = "A valid idempotency key is required."
	}
	if in.PlanVersionID == uuid.Nil {
		fields["plan_version_id"] = "Choose a plan."
	}
	if in.PaymentMethodID == uuid.Nil {
		fields["payment_method_id"] = "Choose a card."
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	if sub, err := s.repo.GetSubscriptionByKey(ctx, userID, in.IdempotencyKey); err == nil {
		return s.result(ctx, sub, nil, nil, in.Locale)
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, apperror.Internal(err)
	}
	v, err := s.repo.GetVersion(ctx, in.PlanVersionID)
	if err != nil {
		return nil, mapRepoErr(err, "Plan")
	}
	plan, err := s.repo.GetPlan(ctx, v.PlanID)
	if err != nil || !plan.Active || !v.IsCurrent {
		return nil, apperror.NotFound("Plan")
	}
	now := s.clock.Now()
	method, err := s.repo.GetMethod(ctx, userID, in.PaymentMethodID)
	if err != nil {
		return nil, mapRepoErr(err, "Payment method")
	}
	if !method.Usable(now) {
		return nil, apperror.Validation(map[string]string{"payment_method_id": "This card can no longer be charged. Add another card."})
	}
	if _, ok := s.storedCharger(method.Provider); !ok {
		return nil, apperror.Conflict(CodePaymentMethodRequired, "Add a card that can pay for memberships.")
	}
	route := payment.RouteInput{UserID: userID.String(), Countries: countries(in.TrustedCountry, method.Country), Currency: v.Currency,
		PlanID: v.PlanID.String(), Needs: payment.CapStoredCharge}
	if ok, err := s.providerAllowed(ctx, method.Provider, route, payment.PurposeSubscription); err != nil {
		return nil, apperror.Internal(err)
	} else if !ok {
		return nil, apperror.Conflict(CodePaymentMethodRequired, "Add a card that can pay for memberships.")
	}
	if _, err := s.repo.GetOpenSubscription(ctx, userID); err == nil {
		return nil, apperror.Conflict(CodeSubscriptionExists, "You already have a membership.")
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, apperror.Internal(err)
	}

	periodEnd := domain.AddMonthsAnchored(now, v.PeriodMonths(), now.Day())
	sub := &domain.Subscription{ID: domain.NewID(), UserID: userID, PlanID: v.PlanID, PlanVersionID: v.ID, Status: domain.SubIncomplete,
		PaymentMethodID: &method.ID, Provider: method.Provider, BillingAnchorDay: now.Day(), IdempotencyKey: in.IdempotencyKey,
		CreatedAt: now, UpdatedAt: now}
	inv := &domain.SubscriptionInvoice{ID: domain.NewID(), SubscriptionID: sub.ID, UserID: userID, PeriodIndex: 1, PeriodStart: now,
		PeriodEnd: periodEnd, AmountCents: v.PriceCents, Currency: v.Currency, Status: domain.InvoiceOpen, AttemptCount: 1,
		CreatedAt: now, UpdatedAt: now}
	lease := now.Add(s.cfg.RenewalLease)
	t := &domain.PaymentTransaction{ID: domain.NewID(), UserID: userID, Kind: domain.TxSubscriptionInitial, Status: domain.TxPending,
		AmountCents: v.PriceCents, Currency: v.Currency, Provider: method.Provider, PaymentMethodID: &method.ID, SubscriptionID: &sub.ID,
		InvoiceID: &inv.ID, Description: plan.Name.For("en"), IdempotencyKey: fmt.Sprintf("sub:%s:p1:a1", sub.ID), AttemptNo: 1,
		LockedUntil: &lease, CreatedAt: now, UpdatedAt: now}
	inv.PaymentTransactionID = nil
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.repo.CreateSubscription(ctx, sub); err != nil {
			return err
		}
		if err := s.repo.CreateInvoice(ctx, inv); err != nil {
			return err
		}
		if err := s.repo.CreateTransaction(ctx, t); err != nil {
			return err
		}
		return s.repo.UpdateInvoice(ctx, inv.ID, map[string]any{"payment_transaction_id": t.ID})
	})
	switch {
	case errors.Is(err, repository.ErrSubscriptionOpen):
		return nil, apperror.Conflict(CodeSubscriptionExists, "You already have a membership.")
	case errors.Is(err, repository.ErrDuplicate):
		if existing, err2 := s.repo.GetSubscriptionByKey(ctx, userID, in.IdempotencyKey); err2 == nil {
			return s.result(ctx, existing, nil, nil, in.Locale)
		}
		return nil, apperror.Internal(err)
	case err != nil:
		return nil, apperror.Internal(err)
	}
	s.track(ctx, "billing.subscription_started", map[string]any{"plan": plan.Code, "version": v.Version, "provider": method.Provider})

	a := &chargeAttempt{tx: t, subscriptionID: sub.ID, periodIndex: 1, method: method, reason: payment.ReasonCustomerInitiated}
	out := s.execute(ctx, a)
	if _, err := s.applySubscriptionOutcome(ctx, a, out); err != nil {
		return nil, err
	}
	final, err := s.repo.GetSubscription(ctx, sub.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if out.decline != nil {
		s.track(ctx, "billing.subscription_declined", map[string]any{"code": out.decline.Code, "soft": out.decline.Soft})
		return nil, paymentFailed(out.decline.Code, map[string]any{"subscription_id": sub.ID})
	}
	ft, _ := s.repo.GetTransaction(ctx, t.ID)
	return s.result(ctx, final, ft, out.result.NextAction, in.Locale)
}

func (s *BillingService) result(ctx context.Context, sub *domain.Subscription, t *domain.PaymentTransaction, next *payment.NextAction,
	locale string) (*SubscriptionResult, error) {
	view, err := s.subscriptionView(ctx, sub, locale)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := &SubscriptionResult{Subscription: view, NextAction: next}
	if t != nil {
		methods := map[uuid.UUID]domain.PaymentMethod{}
		if t.PaymentMethodID != nil {
			methods, _ = s.repo.GetMethodsAnyState(ctx, []uuid.UUID{*t.PaymentMethodID})
		}
		tv := transactionView(t, methods)
		out.Transaction = &tv
	}
	return out, nil
}

// CancelSubscription stops auto-renewal: an active membership runs until
// the end of the paid period; an unpaid (past-due or incomplete) one ends
// now.
func (s *BillingService) CancelSubscription(ctx context.Context, userID uuid.UUID, reason, locale string) (*SubscriptionView, error) {
	reason = CleanLine(reason)
	if utf8.RuneCountInString(reason) > 300 {
		return nil, apperror.Validation(map[string]string{"reason": "Use at most 300 characters."})
	}
	now := s.clock.Now()
	var sub *domain.Subscription
	endedNow := false
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		open, err := s.repo.GetOpenSubscription(ctx, userID)
		if err != nil {
			return err
		}
		if sub, err = s.repo.GetSubscriptionForUpdate(ctx, open.ID); err != nil {
			return err
		}
		switch sub.Status {
		case domain.SubActive:
			if sub.CancelAtPeriodEnd {
				return nil
			}
			return s.repo.UpdateSubscription(ctx, sub.ID, map[string]any{"cancel_at_period_end": true, "canceled_at": now,
				"cancel_reason": reason, "updated_at": now})
		case domain.SubPastDue, domain.SubIncomplete:
			endedNow = true
			return s.endSubscription(ctx, sub, domain.SubCanceled, firstNonBlank(reason, "canceled_by_user"), now, now)
		}
		return apperror.InvalidState("This membership has already ended.")
	})
	if errors.Is(err, repository.ErrNotFound) {
		return nil, apperror.NotFound("Membership")
	}
	if err != nil {
		return nil, mapRepoErr(err, "Membership")
	}
	s.record(ctx, "billing.subscription_canceled", "subscription", sub.ID.String(), map[string]any{"by": "user", "immediate": endedNow})
	if endedNow {
		s.notify.Notify(ctx, NotificationSpec{Kind: NotifySubscriptionCanceled, Data: map[string]any{"subscription_id": sub.ID,
			"reason": "canceled_by_user"}, Link: "/profile/settings"}, userID)
	}
	s.track(ctx, "billing.subscription_canceled", map[string]any{"immediate": endedNow})
	return s.Subscription(ctx, userID, locale)
}

// ResumeSubscription undoes a cancellation before the period ends.
func (s *BillingService) ResumeSubscription(ctx context.Context, userID uuid.UUID, locale string) (*SubscriptionView, error) {
	now := s.clock.Now()
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		open, err := s.repo.GetOpenSubscription(ctx, userID)
		if err != nil {
			return err
		}
		sub, err := s.repo.GetSubscriptionForUpdate(ctx, open.ID)
		if err != nil {
			return err
		}
		if sub.Status != domain.SubActive || !sub.CancelAtPeriodEnd || sub.CurrentPeriodEnd == nil || !sub.CurrentPeriodEnd.After(now) {
			return apperror.InvalidState("This membership can't be resumed. Start a new one instead.")
		}
		return s.repo.UpdateSubscription(ctx, sub.ID, map[string]any{"cancel_at_period_end": false, "canceled_at": nil,
			"cancel_reason": "", "updated_at": now})
	})
	if errors.Is(err, repository.ErrNotFound) {
		return nil, apperror.NotFound("Membership")
	}
	if err != nil {
		return nil, mapRepoErr(err, "Membership")
	}
	s.track(ctx, "billing.subscription_resumed", nil)
	return s.Subscription(ctx, userID, locale)
}

// UpdateSubscriptionInput changes the membership's card.
type UpdateSubscriptionInput struct {
	PaymentMethodID uuid.UUID
	TrustedCountry  string
	Locale          string
}

// UpdateSubscription switches the membership to another saved card. A
// past-due membership is charged immediately with the new card.
func (s *BillingService) UpdateSubscription(ctx context.Context, userID uuid.UUID, in UpdateSubscriptionInput) (*SubscriptionResult, error) {
	if in.PaymentMethodID == uuid.Nil {
		return nil, apperror.Validation(map[string]string{"payment_method_id": "Choose a card."})
	}
	now := s.clock.Now()
	method, err := s.repo.GetMethod(ctx, userID, in.PaymentMethodID)
	if err != nil {
		return nil, mapRepoErr(err, "Payment method")
	}
	if !method.Usable(now) {
		return nil, apperror.Validation(map[string]string{"payment_method_id": "This card can no longer be charged. Add another card."})
	}
	if _, ok := s.storedCharger(method.Provider); !ok {
		return nil, apperror.Conflict(CodePaymentMethodRequired, "Add a card that can pay for memberships.")
	}
	open, err := s.repo.GetOpenSubscription(ctx, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, apperror.NotFound("Membership")
	} else if err != nil {
		return nil, apperror.Internal(err)
	}
	route := payment.RouteInput{UserID: userID.String(), Countries: countries(in.TrustedCountry, method.Country), PlanID: open.PlanID.String(),
		Needs: payment.CapStoredCharge}
	if v, err := s.repo.GetVersion(ctx, open.PlanVersionID); err == nil {
		route.Currency = v.Currency
	}
	if ok, err := s.providerAllowed(ctx, method.Provider, route, payment.PurposeSubscription); err != nil {
		return nil, apperror.Internal(err)
	} else if !ok {
		return nil, apperror.Conflict(CodePaymentMethodRequired, "Add a card that can pay for memberships.")
	}
	retry := false
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		sub, err := s.repo.GetSubscriptionForUpdate(ctx, open.ID)
		if err != nil {
			return err
		}
		if !sub.Status.Open() {
			return apperror.InvalidState("This membership has already ended.")
		}
		upd := map[string]any{"payment_method_id": method.ID, "provider": method.Provider, "updated_at": now}
		if sub.Status == domain.SubPastDue && (sub.LockedUntil == nil || !sub.LockedUntil.After(now)) {
			upd["next_attempt_at"] = now
			retry = true
		}
		return s.repo.UpdateSubscription(ctx, sub.ID, upd)
	})
	if err != nil {
		return nil, mapRepoErr(err, "Membership")
	}
	var t *domain.PaymentTransaction
	var next *payment.NextAction
	if retry {
		if a, err := s.claimRenewal(ctx, open.ID, now); err != nil {
			return nil, err
		} else if a != nil {
			out := s.execute(ctx, a)
			if _, err := s.applySubscriptionOutcome(ctx, a, out); err != nil {
				return nil, err
			}
			t, _ = s.repo.GetTransaction(ctx, a.tx.ID)
			next = out.result.NextAction
		}
	}
	sub, err := s.repo.GetSubscription(ctx, open.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return s.result(ctx, sub, t, next, in.Locale)
}

// endSubscription moves an open membership to a terminal status and
// expires the free minutes it granted (inside a transaction, row locked).
func (s *BillingService) endSubscription(ctx context.Context, sub *domain.Subscription, to domain.SubscriptionStatus, reason string,
	endedAt, now time.Time) error {
	if !sub.Status.CanTransition(to) {
		return apperror.InvalidState("This membership has already ended.")
	}
	fields := map[string]any{"status": to, "ended_at": endedAt, "next_attempt_at": nil, "next_grant_at": nil, "locked_until": nil,
		"updated_at": now}
	if to == domain.SubCanceled && sub.CanceledAt == nil {
		fields["canceled_at"] = now
	}
	if reason != "" {
		fields["cancel_reason"] = truncateString(reason, 300)
	}
	if err := s.repo.UpdateSubscription(ctx, sub.ID, fields); err != nil {
		return err
	}
	return s.expireGrantedMinutes(ctx, sub, now)
}
