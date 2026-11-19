package service

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 6.0 (§26.6): what clients and the admin console see of invoices
// and credit notes, the lookups the purchase and transaction views use, and the
// accounting export.

// InvoiceBuyerView is the buyer on a document.
type InvoiceBuyerView struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Country string `json:"country,omitempty"`
}

// InvoiceView is an invoice or a credit note (type Invoice of §26.6).
type InvoiceView struct {
	ID              uuid.UUID             `json:"id"`
	Number          string                `json:"number"`
	Kind            string                `json:"kind"`
	TransactionID   uuid.UUID             `json:"transaction_id"`
	OriginalInvoice *InvoiceRef           `json:"original_invoice,omitempty"`
	Currency        string                `json:"currency"`
	SubtotalCents   int64                 `json:"subtotal_cents"`
	TaxCents        int64                 `json:"tax_cents"`
	TotalCents      int64                 `json:"total_cents"`
	Lines           []domain.InvoiceLine  `json:"lines"`
	Buyer           InvoiceBuyerView      `json:"buyer"`
	Seller          domain.InvoiceSeller  `json:"seller"`
	Payment         domain.InvoicePayment `json:"payment"`
	Locale          string                `json:"locale"`
	IssuedAt        time.Time             `json:"issued_at"`
}

// InvoiceUserView is the account of a document in the admin console.
type InvoiceUserView struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
}

// AdminInvoiceView is an invoice with its account (admin list and detail).
type AdminInvoiceView struct {
	InvoiceView
	User InvoiceUserView `json:"user"`
}

func (s *InvoiceService) view(inv *domain.Invoice) (InvoiceView, error) {
	buyer, err := s.buyerOf(inv)
	if err != nil {
		return InvoiceView{}, err
	}
	v := InvoiceView{ID: inv.ID, Number: inv.Number, Kind: string(inv.Kind), TransactionID: inv.PaymentTransactionID, Currency: inv.Currency,
		SubtotalCents: inv.SubtotalCents, TaxCents: inv.TaxCents, TotalCents: inv.TotalCents, Lines: []domain.InvoiceLine(inv.Lines),
		Buyer: InvoiceBuyerView{Name: buyer.Name, Email: buyer.Email, Country: buyer.Country}, Seller: inv.Seller, Payment: inv.Payment,
		Locale: inv.Locale, IssuedAt: inv.IssuedAt}
	if v.Lines == nil {
		v.Lines = []domain.InvoiceLine{}
	}
	if inv.OriginalInvoiceID != nil {
		v.OriginalInvoice = &InvoiceRef{ID: *inv.OriginalInvoiceID, Number: inv.OriginalInvoiceNumber}
	}
	return v, nil
}

// owned loads a document of the caller; documents of other accounts are
// reported as not found.
func (s *InvoiceService) owned(ctx context.Context, userID, id uuid.UUID) (*domain.Invoice, error) {
	inv, err := s.d.Repo.Get(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Invoice")
	}
	if inv.UserID != userID {
		return nil, apperror.NotFound("Invoice")
	}
	return inv, nil
}

// Get returns one of the caller's invoices or credit notes.
func (s *InvoiceService) Get(ctx context.Context, userID, id uuid.UUID) (*InvoiceView, error) {
	inv, err := s.owned(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	v, err := s.view(inv)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &v, nil
}

// PDF renders one of the caller's documents; the second value is the download
// name ("SY-2026-000001.pdf").
func (s *InvoiceService) PDF(ctx context.Context, userID, id uuid.UUID) ([]byte, string, error) {
	inv, err := s.owned(ctx, userID, id)
	if err != nil {
		return nil, "", err
	}
	pdf, name, err := s.document(inv)
	if err != nil {
		return nil, "", apperror.Internal(err)
	}
	return pdf, name, nil
}

// AdminInvoiceQuery filters the admin list.
type AdminInvoiceQuery struct {
	UserID *uuid.UUID
	Kind   string
	// Q is a number or the start of one ("SY-2026-0001").
	Q        string
	From, To string
	Cursor   string
	Limit    int
}

// AdminList lists documents of every account, newest first.
func (s *InvoiceService) AdminList(ctx context.Context, q AdminInvoiceQuery) (Page[AdminInvoiceView], error) {
	cursor, err := decodeCursor(q.Cursor)
	if err != nil {
		return Page[AdminInvoiceView]{}, err
	}
	kind := domain.InvoiceKind(q.Kind)
	if kind != "" && !kind.Valid() {
		return Page[AdminInvoiceView]{}, apperror.Validation(map[string]string{"kind": "Unknown kind."})
	}
	f := repository.InvoiceFilter{UserID: q.UserID, Kind: kind, NumberPrefix: strings.ToUpper(strings.TrimSpace(q.Q)), Cursor: cursor, Limit: q.Limit}
	fields := map[string]string{}
	if f.From, err = parseDay(q.From, false); err != nil {
		fields["from"] = "Enter a valid date (YYYY-MM-DD)."
	}
	if f.To, err = parseDay(q.To, true); err != nil {
		fields["to"] = "Enter a valid date (YYYY-MM-DD)."
	}
	if len(fields) > 0 {
		return Page[AdminInvoiceView]{}, apperror.Validation(fields)
	}
	rows, err := s.d.Repo.List(ctx, f)
	if err != nil {
		return Page[AdminInvoiceView]{}, apperror.Internal(err)
	}
	views, err := s.adminViews(ctx, rows)
	if err != nil {
		return Page[AdminInvoiceView]{}, apperror.Internal(err)
	}
	page := Page[AdminInvoiceView]{Items: views}
	if len(rows) > 0 && len(rows) == clampPage(q.Limit, 20, 100) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.IssuedAt, last.ID.String())
	}
	return page, nil
}

// adminViews builds the admin views of a page: one query for the accounts.
func (s *InvoiceService) adminViews(ctx context.Context, rows []domain.Invoice) ([]AdminInvoiceView, error) {
	ids := make([]uuid.UUID, 0, len(rows))
	seen := map[uuid.UUID]bool{}
	for i := range rows {
		if !seen[rows[i].UserID] {
			seen[rows[i].UserID] = true
			ids = append(ids, rows[i].UserID)
		}
	}
	users := map[uuid.UUID]domain.User{}
	if len(ids) > 0 {
		list, err := s.d.Users.GetByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, u := range list {
			users[u.ID] = u
		}
	}
	out := make([]AdminInvoiceView, 0, len(rows))
	for i := range rows {
		v, err := s.view(&rows[i])
		if err != nil {
			return nil, err
		}
		u := users[rows[i].UserID]
		out = append(out, AdminInvoiceView{InvoiceView: v, User: InvoiceUserView{ID: rows[i].UserID, Email: u.Email, DisplayName: u.DisplayName}})
	}
	return out, nil
}

// AdminGet returns any document.
func (s *InvoiceService) AdminGet(ctx context.Context, id uuid.UUID) (*AdminInvoiceView, error) {
	inv, err := s.d.Repo.Get(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Invoice")
	}
	views, err := s.adminViews(ctx, []domain.Invoice{*inv})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &views[0], nil
}

// AdminPDF renders any document.
func (s *InvoiceService) AdminPDF(ctx context.Context, id uuid.UUID) ([]byte, string, error) {
	inv, err := s.d.Repo.Get(ctx, id)
	if err != nil {
		return nil, "", mapRepoErr(err, "Invoice")
	}
	pdf, name, err := s.document(inv)
	if err != nil {
		return nil, "", apperror.Internal(err)
	}
	return pdf, name, nil
}

// PurchaseInvoicePDF renders the invoice of a purchase (dispute evidence,
// §26.5: implements InvoicePDFs). ErrNotFound when none was issued.
func (s *InvoiceService) PurchaseInvoicePDF(ctx context.Context, purchaseID uuid.UUID) ([]byte, string, error) {
	rows, err := s.d.Repo.InvoicesByPurchases(ctx, []uuid.UUID{purchaseID})
	if err != nil {
		return nil, "", apperror.Internal(err)
	}
	for i := range rows {
		if rows[i].Kind == domain.InvoiceKindInvoice {
			pdf, name, err := s.document(&rows[i])
			if err != nil {
				return nil, "", apperror.Internal(err)
			}
			return pdf, name, nil
		}
	}
	return nil, "", apperror.NotFound("Invoice")
}

// ---------------------------------------------------------------- lookups

// InvoiceLookup finds the documents of purchases and payments for the views
// that list them (one query per page).
type InvoiceLookup interface {
	TransactionRefs(ctx context.Context, rows []domain.PaymentTransaction) (map[uuid.UUID]TransactionInvoiceRefs, error)
	PurchaseInvoices(ctx context.Context, purchaseIDs []uuid.UUID) (map[uuid.UUID]*InvoiceRef, error)
}

// TransactionInvoiceRefs are the documents of one payment row: the invoice of
// a purchase, the credit note of a refund.
type TransactionInvoiceRefs struct {
	Invoice    *InvoiceRef
	CreditNote *InvoiceRef
}

var _ InvoiceLookup = (*InvoiceService)(nil)

// TransactionRefs implements InvoiceLookup with one query for the page.
func (s *InvoiceService) TransactionRefs(ctx context.Context, rows []domain.PaymentTransaction) (map[uuid.UUID]TransactionInvoiceRefs, error) {
	ids := make([]uuid.UUID, 0, len(rows))
	for i := range rows {
		if rows[i].Kind == domain.TxCreditPurchase || rows[i].Kind == domain.TxRefund {
			ids = append(ids, rows[i].ID)
		}
	}
	docs, err := s.d.Repo.RefsByTransactions(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]TransactionInvoiceRefs, len(docs))
	for i := range docs {
		r := out[docs[i].PaymentTransactionID]
		if docs[i].Kind == domain.InvoiceKindCreditNote {
			r.CreditNote = invoiceRef(&docs[i])
		} else {
			r.Invoice = invoiceRef(&docs[i])
		}
		out[docs[i].PaymentTransactionID] = r
	}
	return out, nil
}

// PurchaseInvoices implements InvoiceLookup with one query.
func (s *InvoiceService) PurchaseInvoices(ctx context.Context, purchaseIDs []uuid.UUID) (map[uuid.UUID]*InvoiceRef, error) {
	docs, err := s.d.Repo.InvoicesByPurchases(ctx, purchaseIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]*InvoiceRef, len(docs))
	for i := range docs {
		if docs[i].CreditPurchaseID != nil {
			out[*docs[i].CreditPurchaseID] = invoiceRef(&docs[i])
		}
	}
	return out, nil
}

// ---------------------------------------------------------------- export

// Accounting export limits: at most 366 days, at most maxExportRows rows.
const (
	maxExportDays  = 366
	maxExportRows  = 100_000
	exportPageSize = 500
)

// exportHeader is the header of the accounting CSV. Amounts are decimal
// (major units); credit notes are negative so a column sum is the net revenue.
var exportHeader = []string{"number", "kind", "issued_at", "original_number", "buyer_country", "currency", "subtotal", "tax", "total",
	"provider", "provider_reference"}

// parseDay reads a date (YYYY-MM-DD, UTC) or an RFC 3339 time. A bare date
// bounds a day: the start for a "from", the start of the next day for an
// inclusive "to". Empty input is no bound.
func parseDay(s string, endInclusive bool) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		if endInclusive {
			t = t.AddDate(0, 0, 1)
		}
		return &t, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	t = t.UTC()
	return &t, nil
}

// ExportRange validates the range of an accounting export: both bounds, in
// order, at most 366 days.
func ExportRange(from, to string) (time.Time, time.Time, error) {
	if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
		return time.Time{}, time.Time{}, apperror.Validation(map[string]string{"from": "Enter both dates (from and to) of the export."})
	}
	f, err := parseDay(from, false)
	if err != nil {
		return time.Time{}, time.Time{}, apperror.Validation(map[string]string{"from": "Enter a valid date (YYYY-MM-DD)."})
	}
	t, err := parseDay(to, true)
	if err != nil {
		return time.Time{}, time.Time{}, apperror.Validation(map[string]string{"to": "Enter a valid date (YYYY-MM-DD)."})
	}
	if !t.After(*f) {
		return time.Time{}, time.Time{}, apperror.Validation(map[string]string{"to": "The end date must not be before the start date."})
	}
	if t.Sub(*f) > maxExportDays*24*time.Hour {
		return time.Time{}, time.Time{}, apperror.Validation(map[string]string{"to": "The export covers at most 366 days."})
	}
	return *f, *t, nil
}

// ExportCSV streams the documents issued in the range as CSV (accounting
// export), newest first, in keyset pages so memory stays flat. The range is
// validated (ExportRange) and the export is audited before the first byte. It
// returns the number of rows written.
func (s *InvoiceService) ExportCSV(ctx context.Context, from, to string, w io.Writer) (int, error) {
	f, t, err := ExportRange(from, to)
	if err != nil {
		return 0, err
	}
	recordAudit(ctx, s.d.Audit, s.d.Clock, s.cfg.IPHashSecret, "invoices.exported", "invoice", "",
		map[string]any{"from": f.Format(time.RFC3339), "to": t.Format(time.RFC3339)})
	cw := csv.NewWriter(w)
	if err := cw.Write(exportHeader); err != nil {
		return 0, err
	}
	rows, flushEvery := 0, 0
	var cursor *repository.Cursor
	for {
		page, err := s.d.Repo.List(ctx, repository.InvoiceFilter{From: &f, To: &t, Cursor: cursor, Limit: exportPageSize})
		if err != nil {
			return rows, apperror.Internal(err)
		}
		for i := range page {
			if rows >= maxExportRows {
				cw.Flush()
				return rows, apperror.Validation(map[string]string{"to": "The export is too large. Choose a shorter date range."})
			}
			buyer, err := s.buyerOf(&page[i])
			if err != nil {
				return rows, apperror.Internal(err)
			}
			if err := cw.Write(exportRow(&page[i], buyer)); err != nil {
				return rows, err
			}
			rows++
			if flushEvery++; flushEvery%exportPageSize == 0 {
				cw.Flush()
			}
		}
		if len(page) < exportPageSize {
			break
		}
		last := page[len(page)-1]
		cursor = &repository.Cursor{At: last.IssuedAt, ID: last.ID.String()}
	}
	cw.Flush()
	return rows, cw.Error()
}

// exportRow is one CSV row.
func exportRow(inv *domain.Invoice, buyer domain.InvoiceBuyer) []string {
	sign := int64(1)
	if inv.Kind == domain.InvoiceKindCreditNote {
		sign = -1
	}
	return []string{inv.Number, string(inv.Kind), inv.IssuedAt.UTC().Format(time.RFC3339), inv.OriginalInvoiceNumber, csvText(buyer.Country),
		inv.Currency, decimal(sign * inv.SubtotalCents), decimal(sign * inv.TaxCents), decimal(sign * inv.TotalCents),
		csvText(inv.Payment.Provider), csvText(inv.Payment.Reference)}
}

// decimal formats minor units as a decimal amount ("12.34", "-0.05").
func decimal(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}

// csvText neutralises spreadsheet formulas: a text cell that starts with a
// formula character is prefixed with an apostrophe.
func csvText(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// ---------------------------------------------------------------- backfill

// RunBackfill issues the documents the inline path missed (a crash between the
// payment and the invoice, a provider timeout, an outage of the database): the
// purchases paid and the refunds booked in the last BackfillWindow that have no
// invoice or credit note, a bounded batch per run. Invoices come before credit
// notes. The lookups are indexed (payment_transactions_invoicing_idx,
// invoices_transaction_kind_key). It returns how many documents were issued.
func (s *InvoiceService) RunBackfill(ctx context.Context) (int, error) {
	now := s.d.Clock.Now()
	from, to := now.Add(-s.cfg.BackfillWindow), now.Add(-time.Minute) // a payment settled this minute is still being invoiced inline
	issued := 0
	purchases, err := s.d.Repo.PurchasesWithoutInvoice(ctx, from, to, s.cfg.BackfillBatch)
	if err != nil {
		return 0, fmt.Errorf("invoices backfill: purchases: %w", err)
	}
	var firstErr error
	for _, id := range purchases {
		if _, err := s.IssueForPurchase(ctx, id); err != nil {
			s.d.Log.Warn("invoices backfill: purchase not invoiced", zap.String("purchase_id", id.String()), zap.Error(err))
			firstErr = errors.Join(firstErr, err)
			continue
		}
		issued++
	}
	if room := s.cfg.BackfillBatch - len(purchases); room > 0 {
		refunds, err := s.d.Repo.RefundsWithoutCreditNote(ctx, from, to, room)
		if err != nil {
			return issued, fmt.Errorf("invoices backfill: refunds: %w", err)
		}
		for _, id := range refunds {
			if _, err := s.IssueCreditNote(ctx, id); err != nil {
				s.d.Log.Warn("invoices backfill: refund not documented", zap.String("refund_id", id.String()), zap.Error(err))
				firstErr = errors.Join(firstErr, err)
				continue
			}
			issued++
		}
	}
	return issued, firstErr
}
