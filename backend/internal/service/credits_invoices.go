package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 6.0 (§26.6): the invoice of a purchase on the purchase views.

// WithInvoiceLookup shows the invoice on purchase views.
func (s *CreditService) WithInvoiceLookup(l InvoiceLookup) *CreditService {
	s.invoiceLookup = l
	return s
}

// attachInvoices sets PurchaseView.Invoice on the views of paid purchases with
// one query, however many views there are. It is the single place purchase
// views get their invoice (the history, a completed purchase, a replay). A
// lookup failure leaves the views without it: the invoice is an addition, never
// a reason to fail the purchase.
func (s *CreditService) attachInvoices(ctx context.Context, views []PurchaseView) {
	if s.invoiceLookup == nil || len(views) == 0 {
		return
	}
	ids := make([]uuid.UUID, 0, len(views))
	for i := range views {
		if views[i].Status == string(domain.PurchaseSucceeded) || views[i].Status == string(domain.PurchaseRefunded) {
			ids = append(ids, views[i].ID)
		}
	}
	if len(ids) == 0 {
		return
	}
	refs, err := s.invoiceLookup.PurchaseInvoices(ctx, ids)
	if err != nil {
		warnErr(ctx, "credits: invoice lookup", err)
		return
	}
	for i := range views {
		views[i].Invoice = refs[views[i].ID]
	}
}
