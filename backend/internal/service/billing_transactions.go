package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// ------------------------------------------------------------ user side

// Transactions lists the caller's payments (credit purchases, membership
// charges and refunds), newest first.
func (s *BillingService) Transactions(ctx context.Context, userID uuid.UUID, cursor string, limit int) (Page[TransactionView], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[TransactionView]{}, err
	}
	rows, err := s.repo.ListTransactions(ctx, repository.TransactionFilter{UserID: &userID, Cursor: c, Limit: limit})
	if err != nil {
		return Page[TransactionView]{}, apperror.Internal(err)
	}
	methods, err := s.methodsOf(ctx, rows)
	if err != nil {
		return Page[TransactionView]{}, apperror.Internal(err)
	}
	page := Page[TransactionView]{Items: make([]TransactionView, 0, len(rows))}
	for i := range rows {
		page.Items = append(page.Items, transactionView(&rows[i], methods))
	}
	if len(rows) > 0 && len(rows) == clampPage(limit, 20, 100) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return page, nil
}

func (s *BillingService) methodsOf(ctx context.Context, rows []domain.PaymentTransaction) (map[uuid.UUID]domain.PaymentMethod, error) {
	ids := make([]uuid.UUID, 0, len(rows))
	seen := map[uuid.UUID]bool{}
	for _, t := range rows {
		if t.PaymentMethodID != nil && !seen[*t.PaymentMethodID] {
			seen[*t.PaymentMethodID] = true
			ids = append(ids, *t.PaymentMethodID)
		}
	}
	return s.repo.GetMethodsAnyState(ctx, ids)
}

// ---------------------------------------------------- credit purchases

// PurchasePayment says how a credit purchase is paid when it does not use
// the hosted checkout: a saved card, or a fresh token (optionally saved).
type PurchasePayment struct {
	PaymentMethodID *uuid.UUID
	Token           string
	Provider        string
	SaveMethod      bool
	IdempotencyKey  string // the purchase's key (used to derive the card's)
	TrustedCountry  string
	Currency        string
}

// PurchaseSource is a resolved way to pay a credit purchase.
type PurchaseSource struct {
	Provider string
	Method   *domain.PaymentMethod
	token    string
}

// PurchaseCharge is a purchase charge outcome.
type PurchaseCharge struct {
	Status      payment.CheckoutStatus
	ProviderRef string
	FailureCode string
	NextAction  *payment.NextAction
}

// UsesBilling reports whether a purchase request pays with a saved card
// or a token (instead of the hosted checkout).
func (in PurchasePayment) UsesBilling() bool { return in.PaymentMethodID != nil || in.Token != "" }

// ResolvePurchaseSource validates the card or token of a credit purchase
// (routing rules included). A token with SaveMethod is saved first.
func (s *BillingService) ResolvePurchaseSource(ctx context.Context, userID uuid.UUID, in PurchasePayment) (*PurchaseSource, error) {
	if s.disabled() {
		return nil, featureOff("methods")
	}
	route := payment.RouteInput{UserID: userID.String(), Countries: countries(in.TrustedCountry), Currency: in.Currency}
	switch {
	case in.PaymentMethodID != nil && in.Token != "":
		return nil, apperror.Validation(map[string]string{"payment_token": "Use either a saved card or a new card, not both."})
	case in.PaymentMethodID != nil:
		m, err := s.repo.GetMethod(ctx, userID, *in.PaymentMethodID)
		if err != nil {
			return nil, mapRepoErr(err, "Payment method")
		}
		if !m.Usable(s.clock.Now()) {
			return nil, apperror.Validation(map[string]string{"payment_method_id": "This card can no longer be charged. Add another card."})
		}
		route.Countries = countries(in.TrustedCountry, m.Country)
		route.Needs = payment.CapStoredCharge
		if ok, err := s.providerAllowed(ctx, m.Provider, route, payment.PurposeCredits); err != nil {
			return nil, apperror.Internal(err)
		} else if !ok {
			return nil, apperror.Validation(map[string]string{"payment_method_id": "This card can't be used for this purchase."})
		}
		return &PurchaseSource{Provider: m.Provider, Method: m}, nil
	}
	if fields := validateToken(in.Token); fields != nil {
		return nil, apperror.Validation(map[string]string{"payment_token": fields["token"]})
	}
	if in.SaveMethod {
		key := "buy" + sha256Hex([]byte(userID.String() + ":" + in.IdempotencyKey))[:40]
		v, _, err := s.AttachPaymentMethod(ctx, userID, AttachMethodInput{Provider: in.Provider, Token: in.Token, IdempotencyKey: key,
			TrustedCountry: in.TrustedCountry})
		if err != nil {
			return nil, err
		}
		id := v.ID
		return s.ResolvePurchaseSource(ctx, userID, PurchasePayment{PaymentMethodID: &id, TrustedCountry: in.TrustedCountry, Currency: in.Currency})
	}
	p, ok := s.registry.Get(in.Provider)
	if !ok {
		return nil, apperror.Validation(map[string]string{"provider": "Choose a supported payment provider."})
	}
	route.Needs = payment.CapCheckout
	if ok, err := s.providerAllowed(ctx, p.Name(), route, payment.PurposeCredits); err != nil {
		return nil, apperror.Internal(err)
	} else if !ok {
		return nil, apperror.Validation(map[string]string{"provider": "This payment provider is not available for your account."})
	}
	return &PurchaseSource{Provider: p.Name(), token: in.Token}, nil
}

// RecordPurchaseTransaction adds a credit purchase to the money ledger. A
// purchase charged here (card or token) is leased so a lost response is
// reconciled; hosted checkouts settle through their webhook.
func (s *BillingService) RecordPurchaseTransaction(ctx context.Context, p *domain.CreditPurchase, description string, src *PurchaseSource) error {
	now := s.clock.Now()
	t := &domain.PaymentTransaction{ID: domain.NewID(), UserID: p.UserID, Kind: domain.TxCreditPurchase, Status: domain.TxPending,
		AmountCents: p.PriceCents, Currency: p.Currency, Credits: p.Credits, Provider: p.Provider, CreditPurchaseID: &p.ID,
		Description: truncateString(description, 200), IdempotencyKey: "purchase:" + p.ID.String(), AttemptNo: 1, CreatedAt: now, UpdatedAt: now}
	if src != nil {
		lease := now.Add(s.cfg.RenewalLease)
		t.LockedUntil = &lease
		if src.Method != nil {
			t.PaymentMethodID = &src.Method.ID
		}
	}
	err := s.repo.CreateTransaction(ctx, t)
	if errors.Is(err, repository.ErrDuplicate) {
		return nil
	}
	return err
}

// ChargePurchase charges a credit purchase with a saved card (customer
// initiated) or a single-use token.
func (s *BillingService) ChargePurchase(ctx context.Context, p *domain.CreditPurchase, src *PurchaseSource) (PurchaseCharge, error) {
	t, err := s.repo.GetTransactionByPurchase(ctx, p.ID)
	if err != nil {
		return PurchaseCharge{}, apperror.Internal(err)
	}
	if src.Method != nil {
		out := s.execute(ctx, &chargeAttempt{tx: t, method: src.Method, reason: payment.ReasonCustomerInitiated})
		switch {
		case out.decline != nil:
			return PurchaseCharge{Status: payment.CheckoutFailed, FailureCode: out.decline.Code}, nil
		case out.unknown:
			return PurchaseCharge{Status: payment.CheckoutPending, ProviderRef: out.result.ProviderRef, NextAction: out.result.NextAction}, nil
		}
		return PurchaseCharge{Status: payment.CheckoutSucceeded, ProviderRef: out.result.ProviderRef}, nil
	}
	prov, ok := s.registry.Get(src.Provider)
	if !ok {
		return PurchaseCharge{}, apperror.FeatureUnavailable("Card payments are not available for your account yet.")
	}
	co, ok := prov.(payment.Checkouter)
	if !ok {
		return PurchaseCharge{}, apperror.FeatureUnavailable("Card payments are not available for your account yet.")
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	res, err := co.Checkout(cctx, payment.CheckoutRequest{PurchaseID: p.ID, UserID: p.UserID, AmountCents: p.PriceCents, Currency: p.Currency,
		Description: t.Description, PaymentToken: src.token, IdempotencyKey: t.IdempotencyKey})
	cancel()
	if err != nil {
		logger.FromContext(ctx).Warn("billing: token checkout outcome unknown; will reconcile", zap.Error(err))
		return PurchaseCharge{Status: payment.CheckoutPending}, nil
	}
	return PurchaseCharge{Status: res.Status, ProviderRef: res.ProviderRef, FailureCode: res.FailureCode}, nil
}

// SettlePurchaseTransaction mirrors a credit purchase's final state in the
// money ledger.
func (s *BillingService) SettlePurchaseTransaction(ctx context.Context, purchaseID uuid.UUID, status domain.TransactionStatus,
	providerRef, failureCode string) error {
	t, err := s.repo.GetTransactionByPurchase(ctx, purchaseID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	fields := map[string]any{"status": status, "locked_until": nil, "updated_at": s.clock.Now()}
	if providerRef != "" && t.ProviderRef == nil {
		fields["provider_ref"] = providerRef
	}
	if failureCode != "" {
		fields["failure_code"] = truncateString(failureCode, 64)
	}
	if status == domain.TxRefunded {
		fields["refunded_amount_cents"] = t.AmountCents
	}
	return s.repo.UpdateTransaction(ctx, t.ID, fields)
}

// ---------------------------------------------------------------- admin

// AdminSubscriptionRow is a membership in the admin console.
type AdminSubscriptionRow struct {
	SubscriptionView
	UserID       uuid.UUID `json:"user_id"`
	Provider     string    `json:"provider"`
	AttemptCount int       `json:"attempt_count"`
	CancelReason string    `json:"cancel_reason,omitempty"`
}

// AdminSubscriptions lists memberships (optionally of one user / status).
func (s *BillingService) AdminSubscriptions(ctx context.Context, userID *uuid.UUID, status, cursor string, limit int) (Page[AdminSubscriptionRow], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[AdminSubscriptionRow]{}, err
	}
	st := domain.SubscriptionStatus(status)
	switch st {
	case "", domain.SubIncomplete, domain.SubActive, domain.SubPastDue, domain.SubCanceled, domain.SubExpired:
	default:
		return Page[AdminSubscriptionRow]{}, apperror.Validation(map[string]string{"status": "Unknown status."})
	}
	rows, err := s.repo.ListSubscriptions(ctx, repository.SubscriptionFilter{UserID: userID, Status: st, Cursor: c, Limit: limit})
	if err != nil {
		return Page[AdminSubscriptionRow]{}, apperror.Internal(err)
	}
	page := Page[AdminSubscriptionRow]{Items: make([]AdminSubscriptionRow, 0, len(rows))}
	for i := range rows {
		v, err := s.subscriptionView(ctx, &rows[i], "en")
		if err != nil {
			return Page[AdminSubscriptionRow]{}, apperror.Internal(err)
		}
		page.Items = append(page.Items, AdminSubscriptionRow{SubscriptionView: v, UserID: rows[i].UserID, Provider: rows[i].Provider,
			AttemptCount: rows[i].AttemptCount, CancelReason: rows[i].CancelReason})
	}
	if len(rows) > 0 && len(rows) == clampPage(limit, 20, 100) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return page, nil
}

// AdminTransactionRow is a money movement in the admin console.
type AdminTransactionRow struct {
	TransactionView
	UserID              uuid.UUID  `json:"user_id"`
	Provider            string     `json:"provider"`
	ProviderRef         string     `json:"provider_ref,omitempty"`
	SubscriptionID      *uuid.UUID `json:"subscription_id,omitempty"`
	CreditPurchaseID    *uuid.UUID `json:"credit_purchase_id,omitempty"`
	ParentTransactionID *uuid.UUID `json:"parent_transaction_id,omitempty"`
	AttemptNo           int        `json:"attempt_no"`
	Refundable          int64      `json:"refundable_cents"`
}

func adminTransactionRow(t *domain.PaymentTransaction, methods map[uuid.UUID]domain.PaymentMethod) AdminTransactionRow {
	r := AdminTransactionRow{TransactionView: transactionView(t, methods), UserID: t.UserID, Provider: t.Provider,
		SubscriptionID: t.SubscriptionID, CreditPurchaseID: t.CreditPurchaseID, ParentTransactionID: t.ParentTransactionID,
		AttemptNo: t.AttemptNo, Refundable: t.Refundable()}
	if t.ProviderRef != nil {
		r.ProviderRef = *t.ProviderRef
	}
	return r
}

// AdminTransactions lists money movements with filters.
func (s *BillingService) AdminTransactions(ctx context.Context, userID *uuid.UUID, status, kind, cursor string, limit int) (Page[AdminTransactionRow], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[AdminTransactionRow]{}, err
	}
	st, k := domain.TransactionStatus(status), domain.TransactionKind(kind)
	switch st {
	case "", domain.TxPending, domain.TxSucceeded, domain.TxFailed, domain.TxRefunded, domain.TxPartiallyRefunded, domain.TxDisputed:
	default:
		return Page[AdminTransactionRow]{}, apperror.Validation(map[string]string{"status": "Unknown status."})
	}
	switch k {
	case "", domain.TxCreditPurchase, domain.TxSubscriptionInitial, domain.TxSubscriptionRenewal, domain.TxRefund:
	default:
		return Page[AdminTransactionRow]{}, apperror.Validation(map[string]string{"kind": "Unknown kind."})
	}
	rows, err := s.repo.ListTransactions(ctx, repository.TransactionFilter{UserID: userID, Status: st, Kind: k, Cursor: c, Limit: limit})
	if err != nil {
		return Page[AdminTransactionRow]{}, apperror.Internal(err)
	}
	methods, err := s.methodsOf(ctx, rows)
	if err != nil {
		return Page[AdminTransactionRow]{}, apperror.Internal(err)
	}
	page := Page[AdminTransactionRow]{Items: make([]AdminTransactionRow, 0, len(rows))}
	for i := range rows {
		page.Items = append(page.Items, adminTransactionRow(&rows[i], methods))
	}
	if len(rows) > 0 && len(rows) == clampPage(limit, 20, 100) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return page, nil
}

// AdminCancelInput ends a membership from the console.
type AdminCancelInput struct {
	Immediate bool   `json:"immediate"`
	Reason    string `json:"reason"`
}

// AdminCancelSubscription cancels a membership now or at period end.
func (s *BillingService) AdminCancelSubscription(ctx context.Context, id uuid.UUID, in AdminCancelInput) (*AdminSubscriptionRow, error) {
	reason := CleanLine(in.Reason)
	if reason == "" || utf8.RuneCountInString(reason) > 300 {
		return nil, apperror.Validation(map[string]string{"reason": "A reason (up to 300 characters) is required."})
	}
	now := s.clock.Now()
	var sub *domain.Subscription
	ended := false
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if sub, err = s.repo.GetSubscriptionForUpdate(ctx, id); err != nil {
			return err
		}
		if !sub.Status.Open() {
			return apperror.InvalidState("This membership has already ended.")
		}
		if in.Immediate || sub.Status != domain.SubActive {
			ended = true
			return s.endSubscription(ctx, sub, domain.SubCanceled, "admin: "+reason, now, now)
		}
		return s.repo.UpdateSubscription(ctx, sub.ID, map[string]any{"cancel_at_period_end": true, "canceled_at": now,
			"cancel_reason": "admin: " + reason, "updated_at": now})
	})
	if err != nil {
		return nil, mapRepoErr(err, "Membership")
	}
	s.record(ctx, "billing.subscription_canceled", "subscription", id.String(), map[string]any{"by": "admin", "immediate": ended,
		"reason": reason})
	if ended {
		s.notify.Notify(ctx, NotificationSpec{Kind: NotifySubscriptionCanceled, Data: map[string]any{"subscription_id": id,
			"reason": "admin"}, Link: "/profile/settings"}, sub.UserID)
	}
	updated, err := s.repo.GetSubscription(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	v, err := s.subscriptionView(ctx, updated, "en")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &AdminSubscriptionRow{SubscriptionView: v, UserID: updated.UserID, Provider: updated.Provider, AttemptCount: updated.AttemptCount,
		CancelReason: updated.CancelReason}, nil
}

// RefundInput refunds (part of) a payment.
type RefundInput struct {
	AmountCents int64  `json:"amount_cents"`
	Reason      string `json:"reason"`
}

// RefundTransaction refunds a payment through its provider and claws back
// what it bought: purchased credits, or the membership credits granted for
// the refunded period (proportionally for partial refunds, never below the
// available balance).
func (s *BillingService) RefundTransaction(ctx context.Context, id uuid.UUID, in RefundInput) (*AdminTransactionRow, error) {
	reason := CleanLine(in.Reason)
	fields := map[string]string{}
	if reason == "" || utf8.RuneCountInString(reason) > 300 {
		fields["reason"] = "A reason (up to 300 characters) is required."
	}
	if in.AmountCents <= 0 {
		fields["amount_cents"] = "Enter an amount to refund."
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	parent, err := s.repo.GetTransaction(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Payment")
	}
	if parent.ProviderRef == nil || *parent.ProviderRef == "" {
		return nil, apperror.InvalidState("This payment has no provider reference and must be refunded with the provider.")
	}
	refunder, ok := s.refunder(parent.Provider)
	if !ok {
		return nil, apperror.InvalidState("This payment must be refunded with the provider.")
	}
	// A refund issued earlier but never booked (crash after the provider
	// call) is completed first, with its own idempotency key, so a retry can
	// never refund twice.
	child, err := s.repo.PendingRefund(ctx, parent.ID)
	if errors.Is(err, repository.ErrNotFound) {
		if in.AmountCents > parent.Refundable() {
			return nil, apperror.Validation(map[string]string{"amount_cents": "The amount exceeds what can still be refunded."})
		}
		n, err := s.repo.CountChildren(ctx, parent.ID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		now := s.clock.Now()
		child = &domain.PaymentTransaction{ID: domain.NewID(), UserID: parent.UserID, Kind: domain.TxRefund, Status: domain.TxPending,
			AmountCents: in.AmountCents, Currency: parent.Currency, Provider: parent.Provider, PaymentMethodID: parent.PaymentMethodID,
			SubscriptionID: parent.SubscriptionID, InvoiceID: parent.InvoiceID, CreditPurchaseID: parent.CreditPurchaseID,
			ParentTransactionID: &parent.ID, Description: truncateString("Refund: "+parent.Description, 200),
			IdempotencyKey: fmt.Sprintf("refund:%s:%d", parent.ID, n+1), AttemptNo: 1, CreatedAt: now, UpdatedAt: now}
		if err := s.repo.CreateTransaction(ctx, child); err != nil {
			return nil, apperror.Internal(err)
		}
	} else if err != nil {
		return nil, apperror.Internal(err)
	} else if child.AmountCents != in.AmountCents {
		return nil, apperror.InvalidState("A refund of this payment is still pending. Retry with the same amount to complete it.")
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	res, err := refunder.Refund(cctx, payment.RefundRequest{ProviderRef: *parent.ProviderRef, AmountCents: child.AmountCents,
		Currency: parent.Currency, IdempotencyKey: child.IdempotencyKey, Reason: reason})
	cancel()
	if err != nil {
		logger.FromContext(ctx).Error("billing: refund failed", zap.Error(err), zap.String("transaction_id", parent.ID.String()))
		var decline *payment.DeclineError
		definitive := errors.Is(err, payment.ErrRefundTooLarge) || errors.Is(err, payment.ErrNotFound) || errors.As(err, &decline)
		if !definitive {
			// Unknown outcome: the row stays pending so a retry reuses its
			// idempotency key instead of refunding twice.
			return nil, apperror.Unavailable(err)
		}
		_ = s.repo.UpdateTransaction(ctx, child.ID, map[string]any{"status": domain.TxFailed, "failure_code": "refund_failed", "updated_at": s.clock.Now()})
		if errors.Is(err, payment.ErrRefundTooLarge) {
			return nil, apperror.Validation(map[string]string{"amount_cents": "The amount exceeds what can still be refunded."})
		}
		return nil, apperror.InvalidState("The provider refused this refund. Refund it from the provider's dashboard.")
	}
	if err := s.bookRefund(ctx, parent.ID, child.ID, child.AmountCents, res.ProviderRef); err != nil {
		return nil, err
	}
	s.record(ctx, "billing.refund", "payment_transaction", parent.ID.String(), map[string]any{"amount_cents": child.AmountCents,
		"currency": parent.Currency, "reason": reason, "refund_id": child.ID})
	updated, err := s.repo.GetTransaction(ctx, parent.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	methods, _ := s.methodsOf(ctx, []domain.PaymentTransaction{*updated})
	row := adminTransactionRow(updated, methods)
	return &row, nil
}

// bookRefund records a refund (issued by us, or reported by the provider)
// and claws back what the payment bought.
func (s *BillingService) bookRefund(ctx context.Context, parentID, childID uuid.UUID, amount int64, providerRef string) error {
	now := s.clock.Now()
	var wallet *domain.Wallet
	var owner uuid.UUID
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		parent, err := s.repo.GetTransactionForUpdate(ctx, parentID)
		if err != nil {
			return err
		}
		// Bookings of one payment serialise on the parent lock; a refund row
		// that is no longer pending was booked by a concurrent path.
		pending, err := s.repo.GetTransactionForUpdate(ctx, childID)
		if err != nil {
			return err
		}
		if pending.Status != domain.TxPending {
			return nil
		}
		amount = min(amount, parent.AmountCents-parent.RefundedAmountCents)
		if amount <= 0 {
			return nil
		}
		owner = parent.UserID
		child := map[string]any{"status": domain.TxSucceeded, "updated_at": now}
		if providerRef != "" {
			child["provider_ref"] = providerRef
		}
		if err := s.repo.UpdateTransaction(ctx, childID, child); err != nil {
			return err
		}
		refunded := parent.RefundedAmountCents + amount
		status := domain.TxPartiallyRefunded
		if refunded >= parent.AmountCents {
			status = domain.TxRefunded
		}
		if err := s.repo.UpdateTransaction(ctx, parent.ID, map[string]any{"refunded_amount_cents": refunded, "status": status,
			"updated_at": now}); err != nil {
			return err
		}
		switch parent.Kind {
		case domain.TxCreditPurchase:
			credits := parent.Credits * amount / parent.AmountCents
			if wallet, err = s.clawBack(ctx, parent.UserID, credits, domain.LedgerRefund, "credit_purchase", parent.CreditPurchaseID,
				"refund:"+childID.String(), "Payment refunded"); err != nil {
				return err
			}
			if status == domain.TxRefunded && parent.CreditPurchaseID != nil {
				return s.credits.UpdatePurchase(ctx, *parent.CreditPurchaseID, map[string]any{"status": domain.PurchaseRefunded})
			}
		case domain.TxSubscriptionInitial, domain.TxSubscriptionRenewal:
			if parent.SubscriptionID == nil || parent.InvoiceID == nil {
				return nil
			}
			period := periodFromKey(parent.IdempotencyKey)
			grants, err := s.repo.GrantsForPeriod(ctx, *parent.SubscriptionID, period)
			if err != nil {
				return err
			}
			for _, g := range grants {
				reverse := (g.Credits * amount / parent.AmountCents)
				if status == domain.TxRefunded {
					reverse = g.Credits - g.CreditsReversed
				}
				reverse = min(reverse, g.Credits-g.CreditsReversed)
				if reverse <= 0 {
					continue
				}
				w, err := s.clawBack(ctx, parent.UserID, reverse, domain.LedgerSubscriptionGrantReversal, "subscription", parent.SubscriptionID,
					fmt.Sprintf("subgrant_rev:%s:%s", g.ID, childID), "Membership credits refunded")
				if err != nil {
					return err
				}
				if w != nil {
					wallet = w
				}
				if err := s.repo.UpdateGrant(ctx, g.ID, map[string]any{"credits_reversed": g.CreditsReversed + reverse, "updated_at": now}); err != nil {
					return err
				}
			}
			if status == domain.TxRefunded {
				return s.repo.UpdateInvoice(ctx, *parent.InvoiceID, map[string]any{"status": domain.InvoiceRefunded, "updated_at": now})
			}
		}
		return nil
	})
	if err != nil {
		return mapRepoErr(err, "Payment")
	}
	if wallet != nil {
		dctx := database.Detach(ctx)
		warnErr(dctx, "billing: publish wallet", s.events.Publish(dctx, realtime.EventWalletUpdated, walletView(wallet), owner))
	}
	return nil
}

// clawBack removes up to credits from the wallet (spent credits cannot go
// below zero). Runs inside a transaction.
func (s *BillingService) clawBack(ctx context.Context, userID uuid.UUID, credits int64, kind domain.LedgerEntryType, refType string,
	refID *uuid.UUID, key, description string) (*domain.Wallet, error) {
	if credits <= 0 {
		return nil, nil
	}
	w, err := s.wallets.GetForUpdate(ctx, userID)
	if err != nil {
		return nil, err
	}
	amount := min(credits, w.Balance)
	if amount < credits {
		logger.FromContext(ctx).Warn("billing: refund exceeds available balance; partial clawback",
			zap.Int64("credits", credits), zap.Int64("clawed_back", amount))
	}
	if amount <= 0 {
		return nil, nil
	}
	w, _, err = s.wallet.Apply(ctx, Movement{UserID: userID, Type: kind, Amount: -amount, ReferenceType: refType, ReferenceID: refID,
		IdempotencyKey: key, Description: description})
	return w, err
}

// periodFromKey extracts the period from sub:<id>:p<n>:a<k>.
func periodFromKey(key string) int {
	for _, part := range strings.Split(key, ":") {
		if strings.HasPrefix(part, "p") {
			var n int
			if _, err := fmt.Sscanf(part, "p%d", &n); err == nil && n > 0 {
				return n
			}
		}
	}
	return 0
}

// ------------------------------------------------------------- webhooks

// HandleProviderWebhook processes a signed notification from a saved-card
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
	digest := sha256Hex(body)
	eventID := evt.ID
	if eventID == "" {
		eventID = "sha256:" + digest
	}
	now := s.clock.Now()
	rec := &domain.ProviderWebhookEvent{ID: domain.NewID(), Provider: provider, EventID: truncateString(eventID, 200),
		EventType: truncateString(evt.Type, 64), PayloadSHA256: digest, CreatedAt: now, UpdatedAt: now}
	fresh, err := s.repo.RecordWebhook(ctx, rec)
	if err != nil {
		return apperror.Internal(err)
	}
	if !fresh {
		// A redelivery: acknowledge it unless the first delivery failed
		// before being applied (handlers are idempotent, so at-least-once
		// processing never applies an event twice).
		existing, err := s.repo.GetWebhook(ctx, provider, rec.EventID)
		if err != nil {
			return apperror.Internal(err)
		}
		if existing.ProcessedAt != nil {
			return nil
		}
		rec = existing
	}
	if err := s.applyWebhook(ctx, provider, rec.EventID, evt); err != nil {
		return err
	}
	warnErr(ctx, "billing: mark webhook processed", s.repo.MarkWebhookProcessed(ctx, rec.ID, s.clock.Now()))
	return nil
}

func (s *BillingService) applyWebhook(ctx context.Context, provider, eventID string, evt *payment.WebhookEvent) error {
	t, err := s.repo.GetTransactionByProviderRef(ctx, provider, evt.ProviderRef)
	if errors.Is(err, repository.ErrNotFound) && evt.IdempotencyKey != "" {
		t, err = s.repo.GetTransactionByKey(ctx, provider, evt.IdempotencyKey)
	}
	if errors.Is(err, repository.ErrNotFound) {
		logger.FromContext(ctx).Warn("billing: webhook for unknown payment", zap.String("provider", provider), zap.String("type", evt.Type))
		return nil // acknowledge so the provider stops retrying
	}
	if err != nil {
		return apperror.Internal(err)
	}
	switch evt.Type {
	case "charge.succeeded", "charge.failed":
		if t.Status != domain.TxPending {
			return nil
		}
		out := chargeOutcome{result: payment.ChargeResult{Status: payment.ChargeSucceeded, ProviderRef: evt.ProviderRef}}
		if evt.Type == "charge.failed" {
			out = chargeOutcome{decline: &payment.DeclineError{Code: firstNonBlank(evt.FailureCode, "card_declined"), Soft: true}}
		}
		switch t.Kind {
		case domain.TxCreditPurchase:
			return s.applyPurchaseOutcome(ctx, t, out)
		case domain.TxSubscriptionInitial, domain.TxSubscriptionRenewal:
			_, err := s.applySubscriptionOutcome(ctx, s.attemptFor(ctx, t), out)
			return err
		}
	case "charge.refunded":
		if evt.RefundRef != "" {
			// A refund the platform issued: already booked, or issued and
			// awaiting its booking (same amount) — never booked twice.
			if child, err := s.repo.GetTransactionByProviderRef(ctx, t.Provider, evt.RefundRef); err == nil {
				if child.Status != domain.TxPending {
					return nil
				}
				return s.bookRefund(ctx, t.ID, child.ID, child.AmountCents, evt.RefundRef)
			} else if !errors.Is(err, repository.ErrNotFound) {
				return apperror.Internal(err)
			}
			pending, err := s.repo.PendingRefund(ctx, t.ID)
			if err == nil && (evt.AmountCents <= 0 || pending.AmountCents == evt.AmountCents) {
				return s.bookRefund(ctx, t.ID, pending.ID, pending.AmountCents, evt.RefundRef)
			} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
				return apperror.Internal(err)
			}
		}
		// One refund row per provider event, keyed by the event, so a
		// re-processed delivery books it at most once.
		key := truncateString("webhook:"+eventID, 128)
		if child, err := s.repo.GetTransactionByKey(ctx, t.Provider, key); err == nil {
			if child.Status != domain.TxPending {
				return nil
			}
			return s.bookRefund(ctx, t.ID, child.ID, child.AmountCents, evt.RefundRef)
		} else if !errors.Is(err, repository.ErrNotFound) {
			return apperror.Internal(err)
		}
		amount := evt.AmountCents
		if amount <= 0 {
			amount = t.Refundable()
		}
		amount = min(amount, t.Refundable())
		if amount <= 0 {
			return nil
		}
		now := s.clock.Now()
		child := &domain.PaymentTransaction{ID: domain.NewID(), UserID: t.UserID, Kind: domain.TxRefund, Status: domain.TxPending,
			AmountCents: amount, Currency: t.Currency, Provider: t.Provider, PaymentMethodID: t.PaymentMethodID,
			SubscriptionID: t.SubscriptionID, InvoiceID: t.InvoiceID, CreditPurchaseID: t.CreditPurchaseID, ParentTransactionID: &t.ID,
			Description: truncateString("Refund: "+t.Description, 200), IdempotencyKey: key, AttemptNo: 1, CreatedAt: now, UpdatedAt: now}
		if err := s.repo.CreateTransaction(ctx, child); err != nil {
			return apperror.Internal(err)
		}
		return s.bookRefund(ctx, t.ID, child.ID, child.AmountCents, evt.RefundRef)
	case "charge.disputed":
		now := s.clock.Now()
		var sub *domain.Subscription
		err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
			if err := s.repo.UpdateTransaction(ctx, t.ID, map[string]any{"status": domain.TxDisputed, "updated_at": now}); err != nil {
				return err
			}
			if t.SubscriptionID == nil {
				return nil
			}
			locked, err := s.repo.GetSubscriptionForUpdate(ctx, *t.SubscriptionID)
			if err != nil || !locked.Status.Open() {
				return err
			}
			sub = locked
			return s.endSubscription(ctx, locked, domain.SubCanceled, "dispute", now, now)
		})
		if err != nil {
			return mapRepoErr(err, "Payment")
		}
		if sub != nil {
			s.notify.Notify(ctx, NotificationSpec{Kind: NotifySubscriptionCanceled, Data: map[string]any{"subscription_id": sub.ID,
				"reason": "dispute"}, Link: "/profile/settings"}, sub.UserID)
		}
	default:
		logger.FromContext(ctx).Info("billing: webhook type ignored", zap.String("type", evt.Type))
	}
	return nil
}

func firstNonBlank(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
