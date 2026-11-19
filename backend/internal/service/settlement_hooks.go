package service

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
)

// revisions 6.0 (docs/platform-extensions.md §26.0): hooks shared by the
// features that react to money moving. They are wired in app.go:
//
//   - Invoicer (§26.6) issues an invoice when a purchase succeeds (before the
//     receipt goes out, so the receipt can attach it) and a credit note when a
//     refund is booked.
//   - Purchase observers run after commit whenever a purchase reaches a final
//     state: the free-trial hold resumes held readings (§26.1) and the
//     purchase stream relays the new state (§26.5).
//   - The purchase gate refuses new purchases (payment disputes, §26.5).
//   - Payment policies (§26.5) are read by billing through a function.

// InvoiceRef identifies an issued invoice or credit note.
type InvoiceRef struct {
	ID     uuid.UUID `json:"id"`
	Number string    `json:"number"`
}

// Invoicer issues Staryield invoices and credit notes. Both calls are
// idempotent: issuing twice returns the document issued the first time.
type Invoicer interface {
	IssueForPurchase(ctx context.Context, purchaseID uuid.UUID) (*InvoiceRef, error)
	IssueCreditNote(ctx context.Context, refundTransactionID uuid.UUID) (*InvoiceRef, error)
}

// PurchaseObserver is called after commit, with a detached context, every
// time a purchase reaches a final state (succeeded, failed or refunded).
// Observers must be quick, must not fail the caller and log their own errors.
type PurchaseObserver func(ctx context.Context, p *domain.CreditPurchase)

// PurchaseGate may refuse a new credit purchase before anything is created
// (an apperror is returned to the buyer as is).
type PurchaseGate func(ctx context.Context, userID uuid.UUID) error

// WithInvoicer issues an invoice for every succeeded purchase.
func (s *CreditService) WithInvoicer(i Invoicer) *CreditService {
	s.invoicer = i
	return s
}

// OnPurchaseSettled registers an observer of final purchase states.
func (s *CreditService) OnPurchaseSettled(fn PurchaseObserver) {
	s.observers = append(s.observers, fn)
}

// WithPurchaseGate installs the gate checked before every new purchase.
func (s *CreditService) WithPurchaseGate(g PurchaseGate) *CreditService {
	s.gate = g
	return s
}

// purchaseSettled tells the observers (after commit).
func (s *CreditService) purchaseSettled(ctx context.Context, p *domain.CreditPurchase) {
	if p == nil || len(s.observers) == 0 {
		return
	}
	dctx := database.Detach(ctx)
	snap := *p
	for _, fn := range s.observers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.FromContext(dctx).Error("credits: purchase observer panicked", zap.Any("panic", r),
						zap.String("purchase_id", snap.ID.String()))
				}
			}()
			fn(dctx, &snap)
		}()
	}
}

// issueInvoice issues the invoice of a succeeded purchase; nil when invoices
// are not wired or issuing failed (the backfill job retries).
func (s *CreditService) issueInvoice(ctx context.Context, purchaseID uuid.UUID) *InvoiceRef {
	if s.invoicer == nil {
		return nil
	}
	ref, err := s.invoicer.IssueForPurchase(database.Detach(ctx), purchaseID)
	if err != nil {
		logger.FromContext(ctx).Warn("credits: invoice not issued yet; the backfill job will retry", zap.Error(err),
			zap.String("purchase_id", purchaseID.String()))
		return nil
	}
	return ref
}

// WithInvoicer issues a credit note for every booked refund.
func (s *BillingService) WithInvoicer(i Invoicer) *BillingService {
	s.invoicer = i
	return s
}

// WithPolicies installs the payment policies reader (SiteService.PaymentPolicies).
func (s *BillingService) WithPolicies(fn func(ctx context.Context) PaymentPolicies) *BillingService {
	s.policiesFn = fn
	return s
}

// policies returns the payment policies, or the protective defaults when no
// reader is installed.
func (s *BillingService) policies(ctx context.Context) PaymentPolicies {
	if s.policiesFn == nil {
		return defaultPaymentPolicies(s.clock.Now())
	}
	return s.policiesFn(ctx)
}

// issueCreditNote issues the credit note of a booked refund; nil when
// invoices are not wired or issuing failed (the backfill job retries).
func (s *BillingService) issueCreditNote(ctx context.Context, refundTransactionID uuid.UUID) *InvoiceRef {
	if s.invoicer == nil {
		return nil
	}
	ref, err := s.invoicer.IssueCreditNote(database.Detach(ctx), refundTransactionID)
	if err != nil {
		logger.FromContext(ctx).Warn("billing: credit note not issued yet; the backfill job will retry", zap.Error(err),
			zap.String("refund_transaction_id", refundTransactionID.String()))
		return nil
	}
	return ref
}
