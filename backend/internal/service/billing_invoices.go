package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 6.0 (§26.6): invoice and credit note references on payment rows.

// WithInvoiceLookup shows the invoice of each purchase and the credit note of
// each refund on the payment history (client and admin).
func (s *BillingService) WithInvoiceLookup(l InvoiceLookup) *BillingService {
	s.invoiceLookup = l
	return s
}

// invoiceRefsOf loads the documents of a page of payment rows with one query;
// nil when invoices are not wired.
func (s *BillingService) invoiceRefsOf(ctx context.Context, rows []domain.PaymentTransaction) (map[uuid.UUID]TransactionInvoiceRefs, error) {
	if s.invoiceLookup == nil || len(rows) == 0 {
		return nil, nil
	}
	return s.invoiceLookup.TransactionRefs(ctx, rows)
}
