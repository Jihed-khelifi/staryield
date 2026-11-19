package service

import (
	"context"
	"errors"
	"fmt"
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
	// revisions 5.0: refund info of the page's credit purchases (one batched
	// lots query and one batched requests query, however many rows).
	var refunds map[uuid.UUID]*TransactionRefundInfo
	if s.refundInfo != nil {
		if refunds, err = s.refundInfo(ctx, rows); err != nil {
			return Page[TransactionView]{}, apperror.Internal(err)
		}
	}
	docs, err := s.invoiceRefsOf(ctx, rows) // revisions 6.0 (§26.6): one batched query for the page
	if err != nil {
		return Page[TransactionView]{}, apperror.Internal(err)
	}
	page := Page[TransactionView]{Items: make([]TransactionView, 0, len(rows))}
	for i := range rows {
		v := transactionView(&rows[i], methods)
		v.Refund = refunds[rows[i].ID]
		v.Invoice, v.CreditNote = docs[rows[i].ID].Invoice, docs[rows[i].ID].CreditNote
		page.Items = append(page.Items, v)
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
			s.persistChargeDetails(ctx, t.ID, chargeDetails{Card: out.result.Card, Risk: out.result.Risk, ThreeDS: out.result.ThreeDS}) // revisions 6.0
			s.recordPendingRef(ctx, t, out.result.ProviderRef)
			return PurchaseCharge{Status: payment.CheckoutPending, ProviderRef: out.result.ProviderRef, NextAction: out.result.NextAction}, nil
		}
		s.persistChargeDetails(ctx, t.ID, chargeDetails{Card: out.result.Card, Risk: out.result.Risk, ThreeDS: out.result.ThreeDS}) // revisions 6.0
		s.recordPendingRef(ctx, t, out.result.ProviderRef)
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
	// revisions 6.0 (§26.5): the charge belongs to the user's customer at the provider
	// (fraud screening, dispute evidence) and carries our ids, the 3-D Secure policy and
	// the return URL of a redirect-based authentication step.
	customerRef, err := s.customerRefFor(ctx, p.UserID, prov)
	if err != nil {
		return PurchaseCharge{}, err
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	res, err := co.Checkout(cctx, payment.CheckoutRequest{PurchaseID: p.ID, UserID: p.UserID, AmountCents: p.PriceCents, Currency: p.Currency,
		Description: t.Description, PaymentToken: src.token, IdempotencyKey: t.IdempotencyKey,
		StatementDescriptor: s.cfg.StatementDescriptor, // revisions 5.1
		CustomerRef:         customerRef, ReturnURL: s.returnURL(ctx, p.UserID, p.ID), Metadata: chargeMetadata(t),
		ThreeDSecure: s.policies(ctx).ThreeDSecure})
	cancel()
	switch {
	case errors.Is(err, payment.ErrInvalidToken):
		return PurchaseCharge{}, apperror.Validation(map[string]string{"payment_token": "The card details could not be verified. Please enter them again."})
	case errors.Is(err, payment.ErrConfiguration):
		return PurchaseCharge{}, s.providerError(ctx, "checkout", err)
	case err != nil:
		logger.FromContext(ctx).Warn("billing: token checkout outcome unknown; will reconcile", zap.Error(err))
		return PurchaseCharge{Status: payment.CheckoutPending}, nil
	}
	s.persistChargeDetails(ctx, t.ID, chargeDetails{Card: res.Card, Risk: res.Risk, ThreeDS: res.ThreeDS}) // revisions 6.0 (§26.5)
	if res.Status != payment.CheckoutFailed {
		s.recordPendingRef(ctx, t, res.ProviderRef)
	}
	return PurchaseCharge{Status: res.Status, ProviderRef: res.ProviderRef, FailureCode: res.FailureCode, NextAction: res.NextAction}, nil
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
	// Consent is what the buyer accepted before paying a credit purchase
	// (revisions 5.1, §25.2); absent for older purchases and other rows.
	Consent *PurchaseConsentView `json:"consent,omitempty"`
	// revisions 6.0 (§26.5): what the processor reported, the dispute and the
	// provider's own page of the payment (decorateRows).
	CardCountry string              `json:"card_country,omitempty"`
	RiskLevel   string              `json:"risk_level,omitempty"`
	RiskScore   *int                `json:"risk_score,omitempty"`
	ThreeDS     string              `json:"three_ds,omitempty"`
	InReview    bool                `json:"in_review"`
	Dispute     *TransactionDispute `json:"dispute,omitempty"`
	ProviderURL string              `json:"provider_url,omitempty"`
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
	items, err := s.adminRows(ctx, rows) // revisions 5.1/6.0: cards, consent, risk, disputes in batched queries
	if err != nil {
		return Page[AdminTransactionRow]{}, apperror.Internal(err)
	}
	page := Page[AdminTransactionRow]{Items: items}
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
	row, _, err := s.refundTransaction(ctx, id, in)
	return row, err
}

// refundTransaction is RefundTransaction that also returns the id of the
// refund row it booked (refund requests link to it).
func (s *BillingService) refundTransaction(ctx context.Context, id uuid.UUID, in RefundInput) (*AdminTransactionRow, uuid.UUID, error) {
	reason := CleanLine(in.Reason)
	fields := map[string]string{}
	if reason == "" || utf8.RuneCountInString(reason) > 300 {
		fields["reason"] = "A reason (up to 300 characters) is required."
	}
	if in.AmountCents <= 0 {
		fields["amount_cents"] = "Enter an amount to refund."
	}
	if len(fields) > 0 {
		return nil, uuid.Nil, apperror.Validation(fields)
	}
	parent, err := s.repo.GetTransaction(ctx, id)
	if err != nil {
		return nil, uuid.Nil, mapRepoErr(err, "Payment")
	}
	if parent.Status == domain.TxDisputed { // revisions 6.0: the dispute must be answered instead
		return nil, uuid.Nil, apperror.RefundDisputed()
	}
	if parent.ProviderRef == nil || *parent.ProviderRef == "" {
		return nil, uuid.Nil, apperror.InvalidState("This payment has no provider reference and must be refunded with the provider.")
	}
	refunder, ok := s.refunder(parent.Provider)
	if !ok {
		return nil, uuid.Nil, apperror.InvalidState("This payment must be refunded with the provider.")
	}
	// A refund issued earlier but never booked (crash after the provider
	// call) is completed first, with its own idempotency key, so a retry can
	// never refund twice.
	child, err := s.repo.PendingRefund(ctx, parent.ID)
	if errors.Is(err, repository.ErrNotFound) {
		if in.AmountCents > parent.Refundable() {
			return nil, uuid.Nil, apperror.Validation(map[string]string{"amount_cents": "The amount exceeds what can still be refunded."})
		}
		n, err := s.repo.CountChildren(ctx, parent.ID)
		if err != nil {
			return nil, uuid.Nil, apperror.Internal(err)
		}
		now := s.clock.Now()
		child = &domain.PaymentTransaction{ID: domain.NewID(), UserID: parent.UserID, Kind: domain.TxRefund, Status: domain.TxPending,
			AmountCents: in.AmountCents, Currency: parent.Currency, Provider: parent.Provider, PaymentMethodID: parent.PaymentMethodID,
			SubscriptionID: parent.SubscriptionID, InvoiceID: parent.InvoiceID, CreditPurchaseID: parent.CreditPurchaseID,
			ParentTransactionID: &parent.ID, Description: truncateString("Refund: "+parent.Description, 200),
			IdempotencyKey: fmt.Sprintf("refund:%s:%d", parent.ID, n+1), AttemptNo: 1, CreatedAt: now, UpdatedAt: now}
		if err := s.repo.CreateTransaction(ctx, child); err != nil {
			return nil, uuid.Nil, apperror.Internal(err)
		}
	} else if err != nil {
		return nil, uuid.Nil, apperror.Internal(err)
	} else if child.AmountCents != in.AmountCents {
		return nil, uuid.Nil, apperror.InvalidState("A refund of this payment is still pending. Retry with the same amount to complete it.")
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	res, err := refunder.Refund(cctx, payment.RefundRequest{ProviderRef: *parent.ProviderRef, AmountCents: child.AmountCents,
		Currency: parent.Currency, IdempotencyKey: child.IdempotencyKey, Reason: reason})
	cancel()
	if err != nil {
		logger.FromContext(ctx).Error("billing: refund failed", zap.Error(err), zap.String("transaction_id", parent.ID.String()))
		var decline *payment.DeclineError
		disputed := errors.Is(err, payment.ErrChargeDisputed) // revisions 6.0: the dispute must be answered instead
		definitive := disputed || errors.Is(err, payment.ErrRefundTooLarge) || errors.Is(err, payment.ErrNotFound) || errors.As(err, &decline)
		if !definitive {
			// Unknown outcome: the row stays pending so a retry reuses its
			// idempotency key instead of refunding twice.
			return nil, uuid.Nil, apperror.Unavailable(err)
		}
		_ = s.repo.UpdateTransaction(ctx, child.ID, map[string]any{"status": domain.TxFailed, "failure_code": "refund_failed", "updated_at": s.clock.Now()})
		if disputed {
			return nil, uuid.Nil, apperror.RefundDisputed()
		}
		if errors.Is(err, payment.ErrRefundTooLarge) {
			return nil, uuid.Nil, apperror.Validation(map[string]string{"amount_cents": "The amount exceeds what can still be refunded."})
		}
		return nil, uuid.Nil, apperror.RefundRefused() // revisions 5.0: its own code (409 refund_refused), not invalid_state
	}
	if err := s.bookRefund(ctx, parent.ID, child.ID, child.AmountCents, res.ProviderRef); err != nil {
		return nil, uuid.Nil, err
	}
	s.record(ctx, "billing.refund", "payment_transaction", parent.ID.String(), map[string]any{"amount_cents": child.AmountCents,
		"currency": parent.Currency, "reason": reason, "refund_id": child.ID})
	updated, err := s.repo.GetTransaction(ctx, parent.ID)
	if err != nil {
		return nil, uuid.Nil, apperror.Internal(err)
	}
	rows, err := s.adminRows(ctx, []domain.PaymentTransaction{*updated})
	if err != nil || len(rows) != 1 {
		return nil, uuid.Nil, apperror.Internal(err)
	}
	return &rows[0], child.ID, nil
}

// bookRefund records a refund (issued by us, or reported by the provider)
// and claws back what the payment bought.
func (s *BillingService) bookRefund(ctx context.Context, parentID, childID uuid.UUID, amount int64, providerRef string) error {
	now := s.clock.Now()
	var wallet *domain.Wallet
	var owner uuid.UUID
	var notice map[string]any // revisions 5.1: the customer's refund notice, built when this call books the refund
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
		notice = refundNotice(parent, childID, amount, s.cfg.StatementDescriptor)
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
			// revisions 5.0: credit lots. Only this purchase's own lot is clawed back.
			if wallet, err = s.clawBackPurchase(ctx, parent, amount, childID); err != nil {
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
				w, err := s.reverseGrant(ctx, parent, g, reverse, childID)
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
	// revisions 6.0 (§26.6): the credit note is issued before the notice so it can link it.
	if notice != nil {
		if ref := s.issueCreditNote(ctx, childID); ref != nil {
			notice["credit_note_id"], notice["credit_note_number"] = ref.ID, ref.Number
		}
	}
	// revisions 5.1: every booked refund tells the customer (in-app and e-mail,
	// §25.5) unless the caller sends its own notice (a refund request approval).
	if notice != nil && !quietRefund(ctx) {
		s.notify.Notify(database.Detach(ctx), NotificationSpec{Kind: NotifyPaymentRefunded, Data: notice, Link: "/profile/settings#billing"}, owner)
	}
	return nil
}

// refundNotice is the data of a payment.refunded notification (§25.5): the
// amount and currency refunded and the original payment, never a card number.
func refundNotice(parent *domain.PaymentTransaction, refundID uuid.UUID, amount int64, descriptor string) map[string]any {
	data := map[string]any{"amount_cents": amount, "currency": parent.Currency, "refund_id": refundID, "transaction_id": parent.ID,
		"description": parent.Description, "purchased_at": parent.CreatedAt}
	if parent.Kind == domain.TxCreditPurchase {
		data["credits"] = parent.Credits
		data["pack_name"] = parent.Description
		if parent.CreditPurchaseID != nil {
			data["purchase_id"] = *parent.CreditPurchaseID
		}
	}
	if descriptor != "" {
		data["statement_descriptor"] = descriptor
	}
	return data
}

type quietRefundKey struct{}

// withQuietRefund marks a refund whose caller sends the customer's notice
// itself (refund request approval), so bookRefund does not send a second one.
func withQuietRefund(ctx context.Context) context.Context {
	return context.WithValue(ctx, quietRefundKey{}, true)
}

func quietRefund(ctx context.Context) bool { v, _ := ctx.Value(quietRefundKey{}).(bool); return v }

// clawBackPurchase takes back the credits a refund of amount cents un-buys,
// from the purchase's own lot and nowhere else: ceil(amount × total ÷ paid),
// never more than what is left of the lot, so a refund larger than the unused
// value (a technical problem, say) only takes what is left. Runs inside a
// transaction; the wallet is locked first so the lot cannot change before
// Apply. Returns the updated wallet, or nil when nothing was taken.
func (s *BillingService) clawBackPurchase(ctx context.Context, parent *domain.PaymentTransaction, amount int64, refundID uuid.UUID) (*domain.Wallet, error) {
	w, lot, err := s.lockedPurchaseLot(ctx, parent)
	if err != nil || lot == nil {
		return nil, err
	}
	credits := domain.ClawbackCredits(lot.CreditsTotal, lot.CreditsRemaining, lot.PaidCents, amount)
	// The balance is authoritative; a lot claiming more than the wallet holds
	// (invariant broken, reported by the reconciliation job) is capped by it.
	credits = min(credits, w.Balance)
	if credits <= 0 {
		return nil, nil
	}
	w, _, err = s.wallet.Apply(ctx, Movement{UserID: parent.UserID, Type: domain.LedgerRefund, Amount: -credits,
		ReferenceType: "credit_purchase", ReferenceID: parent.CreditPurchaseID, IdempotencyKey: "refund:" + refundID.String(),
		Description: "Payment refunded", ConsumeLot: &lot.ID})
	return w, err
}

// lockedPurchaseLot locks the buyer's wallet and returns it with the credit lot
// of the payment's purchase (a nil lot when the payment is no credit purchase or
// the lot does not exist: a purchase from before lots existed whose credits were
// all spent). The wallet is locked first so the lot cannot change before the
// caller's movement; refunds and disputes take credits back through it, from
// that lot and nowhere else. Runs inside a transaction.
func (s *BillingService) lockedPurchaseLot(ctx context.Context, parent *domain.PaymentTransaction) (*domain.Wallet, *domain.CreditLot, error) {
	if parent.CreditPurchaseID == nil {
		return nil, nil, nil
	}
	w, err := s.wallets.GetForUpdate(ctx, parent.UserID)
	if err != nil {
		return nil, nil, err
	}
	lot, err := s.wallet.PurchaseLot(ctx, *parent.CreditPurchaseID)
	if errors.Is(err, repository.ErrNotFound) {
		logger.FromContext(ctx).Warn("billing: purchase has no credit lot; nothing to take back",
			zap.String("purchase_id", parent.CreditPurchaseID.String()))
		return w, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return w, lot, nil
}

// reverseGrant takes back up to reverse credits of a membership grant from
// that grant's lot (never other lots, never below zero). Grants from before
// lots existed have no lot of their own (their credits were merged into the
// user's legacy bonus lot), so nothing is reversed and a warning says so.
func (s *BillingService) reverseGrant(ctx context.Context, parent *domain.PaymentTransaction, g domain.SubscriptionGrant, reverse int64,
	refundID uuid.UUID) (*domain.Wallet, error) {
	w, err := s.wallets.GetForUpdate(ctx, parent.UserID)
	if err != nil {
		return nil, err
	}
	lot, err := s.wallet.GrantLot(ctx, g.ID)
	if errors.Is(err, repository.ErrNotFound) {
		logger.FromContext(ctx).Warn("billing: membership grant has no credit lot; its credits are not reversed",
			zap.String("grant_id", g.ID.String()), zap.Int64("credits", reverse))
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	credits := min(reverse, lot.CreditsRemaining, w.Balance)
	if credits < reverse {
		logger.FromContext(ctx).Warn("billing: membership credits already spent; partial reversal",
			zap.String("grant_id", g.ID.String()), zap.Int64("credits", reverse), zap.Int64("reversed", credits))
	}
	if credits <= 0 {
		return nil, nil
	}
	w, _, err = s.wallet.Apply(ctx, Movement{UserID: parent.UserID, Type: domain.LedgerSubscriptionGrantReversal, Amount: -credits,
		ReferenceType: "subscription", ReferenceID: parent.SubscriptionID,
		IdempotencyKey: fmt.Sprintf("subgrant_rev:%s:%s", g.ID, refundID), Description: "Membership credits refunded", ConsumeLot: &lot.ID})
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

// Provider webhooks are handled in billing_webhooks.go (revisions 6.0, §26.5).

func firstNonBlank(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
