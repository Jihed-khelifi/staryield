package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/invoicepdf"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// revisions 6.0 (docs/platform-extensions.md §26.6): invoices and credit
// notes. Every paid credit purchase gets an invoice and every refund a credit
// note, whichever processor took the payment. A document is a snapshot of the
// buyer, the seller, the payment and the lines at issue time, numbered without
// gaps per kind and calendar year (UTC) and never edited afterwards.

// InvoiceBackfillJob is the scheduler job that issues missing documents.
const InvoiceBackfillJob = "invoices-backfill"

// InvoiceAttachmentKind is the kind of mail.AttachmentRef that names an
// invoice or a credit note (the PDF is rendered when the e-mail is delivered).
const InvoiceAttachmentKind = "invoice"

// InvoiceSettings are the invoice service's configuration values.
type InvoiceSettings struct {
	// StatementDescriptor is what the buyer's card statement shows (§25.3).
	StatementDescriptor string
	// SupportEmail is printed on every document (STARYIELD_MAIL_SUPPORT_INBOX;
	// empty: the built-in support address).
	SupportEmail string
	// FrontendURL is the site origin (the seller's website, the policy link).
	FrontendURL string
	// IPHashSecret keys the audit record's IP hash.
	IPHashSecret string
	// BackfillWindow is how far back the backfill job looks for payments that
	// were settled without a document (default 45 days). Older payments are
	// not touched: the lookup stays proportional to recent activity.
	BackfillWindow time.Duration
	// BackfillBatch bounds the documents issued per job run (default 100).
	BackfillBatch int
}

func (c *InvoiceSettings) defaults() {
	if c.BackfillWindow <= 0 {
		c.BackfillWindow = 45 * 24 * time.Hour
	}
	if c.BackfillBatch <= 0 {
		c.BackfillBatch = 100
	}
}

// InvoiceDeps are the invoice service's collaborators.
type InvoiceDeps struct {
	Tx      database.TxManager
	Repo    repository.InvoiceRepository
	Billing repository.BillingRepository
	Credits repository.CreditRepository
	Users   repository.UserRepository
	Sealer  *security.Sealer
	Audit   repository.AuditRepository
	Clock   clock.Clock
	Log     *zap.Logger
}

// InvoiceService issues, stores and renders invoices and credit notes. It
// implements Invoicer (the hooks of the money flows), AttachmentResolver (the
// e-mail outbox) and the lookups of the purchase and transaction views.
type InvoiceService struct {
	d   InvoiceDeps
	cfg InvoiceSettings
}

// NewInvoiceService wires the invoice service.
func NewInvoiceService(d InvoiceDeps, cfg InvoiceSettings) *InvoiceService {
	cfg.defaults()
	if d.Log == nil {
		d.Log = zap.NewNop()
	}
	return &InvoiceService{d: d, cfg: cfg}
}

var (
	_ Invoicer           = (*InvoiceService)(nil)
	_ AttachmentResolver = (*InvoiceService)(nil)
)

func errNotInvoiceable() error {
	return apperror.InvalidState("This payment cannot be invoiced yet.")
}

func invoiceRef(inv *domain.Invoice) *InvoiceRef { return &InvoiceRef{ID: inv.ID, Number: inv.Number} }

func invoiceAD(id uuid.UUID) string { return "invoice:" + id.String() }

// settledStatus reports whether a payment has been paid (and may since have
// been refunded or disputed).
func settledStatus(s domain.TransactionStatus) bool {
	switch s {
	case domain.TxSucceeded, domain.TxPartiallyRefunded, domain.TxRefunded, domain.TxDisputed:
		return true
	}
	return false
}

// seller is the issuing company, copied from the company details.
func (s *InvoiceService) seller() domain.InvoiceSeller {
	email := strings.TrimSpace(s.cfg.SupportEmail)
	if email == "" {
		email = mail.DefaultSupportEmail
	}
	return domain.InvoiceSeller{LegalName: mail.CompanyLegalName, Address: append([]string(nil), mail.CompanyAddressLines...), Email: email,
		Website: strings.TrimRight(strings.TrimSpace(s.cfg.FrontendURL), "/")}
}

// IssueForPurchase issues the invoice of a paid credit purchase; issuing twice
// (replays, duplicate webhooks, concurrent completion, the backfill job)
// returns the invoice issued the first time. Implements Invoicer.
func (s *InvoiceService) IssueForPurchase(ctx context.Context, purchaseID uuid.UUID) (*InvoiceRef, error) {
	tx, err := s.d.Billing.GetTransactionByPurchase(ctx, purchaseID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, errNotInvoiceable()
	}
	if err != nil {
		return nil, fmt.Errorf("invoices: payment of purchase: %w", err)
	}
	if tx.Kind != domain.TxCreditPurchase || !settledStatus(tx.Status) {
		return nil, errNotInvoiceable()
	}
	if inv, err := s.d.Repo.GetByTransaction(ctx, tx.ID, domain.InvoiceKindInvoice); err == nil {
		return invoiceRef(inv), nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("invoices: lookup: %w", err)
	}
	purchases, err := s.d.Credits.PurchasesByIDs(ctx, []uuid.UUID{purchaseID})
	if err != nil || len(purchases) == 0 {
		return nil, fmt.Errorf("invoices: purchase: %w", err)
	}
	user, err := s.d.Users.GetByID(ctx, tx.UserID)
	if err != nil {
		return nil, fmt.Errorf("invoices: buyer: %w", err)
	}
	var method *domain.PaymentMethod
	if tx.PaymentMethodID != nil {
		if m, err := s.d.Billing.GetMethodAnyState(ctx, *tx.PaymentMethodID); err == nil {
			method = m
		} else if !errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("invoices: card: %w", err)
		}
	}
	inv, buyer := purchaseInvoice(tx, &purchases[0], user, method, s.seller(), s.cfg.StatementDescriptor)
	out, err := s.issue(ctx, tx.ID, inv, buyer)
	if err != nil {
		return nil, err
	}
	return invoiceRef(out), nil
}

// IssueCreditNote issues the credit note of a booked refund of a credit
// purchase (nil, nil for refunds of anything else: memberships have no
// invoices). It references the purchase's invoice, issuing that first when
// the purchase predates invoices. Idempotent. Implements Invoicer.
func (s *InvoiceService) IssueCreditNote(ctx context.Context, refundTransactionID uuid.UUID) (*InvoiceRef, error) {
	refund, err := s.d.Billing.GetTransaction(ctx, refundTransactionID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, errNotInvoiceable()
	}
	if err != nil {
		return nil, fmt.Errorf("invoices: refund: %w", err)
	}
	if refund.Kind != domain.TxRefund || refund.Status != domain.TxSucceeded || refund.ParentTransactionID == nil {
		return nil, errNotInvoiceable()
	}
	if cn, err := s.d.Repo.GetByTransaction(ctx, refund.ID, domain.InvoiceKindCreditNote); err == nil {
		return invoiceRef(cn), nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("invoices: lookup: %w", err)
	}
	parent, err := s.d.Billing.GetTransaction(ctx, *refund.ParentTransactionID)
	if err != nil {
		return nil, fmt.Errorf("invoices: refunded payment: %w", err)
	}
	if parent.Kind != domain.TxCreditPurchase || parent.CreditPurchaseID == nil {
		return nil, nil
	}
	original, err := s.d.Repo.GetByTransaction(ctx, parent.ID, domain.InvoiceKindInvoice)
	if errors.Is(err, repository.ErrNotFound) {
		if _, err = s.IssueForPurchase(ctx, *parent.CreditPurchaseID); err != nil {
			return nil, err
		}
		original, err = s.d.Repo.GetByTransaction(ctx, parent.ID, domain.InvoiceKindInvoice)
	}
	if err != nil {
		return nil, fmt.Errorf("invoices: original invoice: %w", err)
	}
	buyer, err := s.buyerOf(original)
	if err != nil {
		return nil, err
	}
	user, err := s.d.Users.GetByID(ctx, refund.UserID)
	if err != nil {
		return nil, fmt.Errorf("invoices: buyer: %w", err)
	}
	cn := creditNote(refund, original, user.Locale)
	out, err := s.issue(ctx, refund.ID, cn, buyer)
	if err != nil {
		return nil, err
	}
	return invoiceRef(out), nil
}

// issue inserts the document with the next number of its kind and year. The
// number is taken in the same transaction as the insert (the counter row is
// locked until commit), so numbers are gapless: a failed insert gives its
// number back. The payment transaction row is locked first, so concurrent
// issuers of the same document queue up instead of burning numbers; the loser
// finds the winner's document.
func (s *InvoiceService) issue(ctx context.Context, transactionID uuid.UUID, inv *domain.Invoice, buyer domain.InvoiceBuyer) (*domain.Invoice, error) {
	var out *domain.Invoice
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		out = nil
		if _, err := s.d.Billing.GetTransactionForUpdate(ctx, transactionID); err != nil {
			return err
		}
		if existing, err := s.d.Repo.GetByTransaction(ctx, transactionID, inv.Kind); err == nil {
			out = existing
			return nil
		} else if !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		now := s.d.Clock.Now()
		n, err := s.d.Repo.NextNumber(ctx, domain.InvoiceScope(inv.Kind, now), now)
		if err != nil {
			return err
		}
		doc := *inv
		doc.ID, doc.Number, doc.IssuedAt, doc.CreatedAt, doc.UpdatedAt = domain.NewID(), domain.InvoiceNumber(inv.Kind, now, n), now, now, now
		raw, err := json.Marshal(buyer)
		if err != nil {
			return err
		}
		if doc.BuyerSealed, err = s.d.Sealer.Seal(string(raw), invoiceAD(doc.ID)); err != nil {
			return err
		}
		if err := s.d.Repo.Create(ctx, &doc); err != nil {
			return err
		}
		out = &doc
		return nil
	})
	if errors.Is(err, repository.ErrDuplicate) {
		// Another instance issued it between our check and insert.
		existing, gerr := s.d.Repo.GetByTransaction(ctx, transactionID, inv.Kind)
		if gerr != nil {
			return nil, fmt.Errorf("invoices: lookup after duplicate: %w", gerr)
		}
		return existing, nil
	}
	if err != nil {
		return nil, fmt.Errorf("invoices: issue: %w", err)
	}
	return out, nil
}

// buyerOf opens the sealed buyer snapshot of a document.
func (s *InvoiceService) buyerOf(inv *domain.Invoice) (domain.InvoiceBuyer, error) {
	var b domain.InvoiceBuyer
	raw, err := s.d.Sealer.Open(inv.BuyerSealed, invoiceAD(inv.ID))
	if err != nil {
		return b, fmt.Errorf("invoices: open buyer of %s: %w", inv.ID, err)
	}
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		return b, fmt.Errorf("invoices: decode buyer of %s: %w", inv.ID, err)
	}
	return b, nil
}

// ---------------------------------------------------------------- snapshots

// purchaseInvoice builds the invoice of a paid purchase (number, id and
// dates are given when it is issued). The buyer is sealed at issue time.
func purchaseInvoice(tx *domain.PaymentTransaction, p *domain.CreditPurchase, user *domain.User, method *domain.PaymentMethod,
	seller domain.InvoiceSeller, descriptor string) (*domain.Invoice, domain.InvoiceBuyer) {
	name := strings.TrimSpace(tx.Description)
	if name == "" {
		name = "Credit pack"
	}
	buyer := domain.InvoiceBuyer{Name: user.DisplayName, Email: user.Email}
	pay := domain.InvoicePayment{Provider: tx.Provider, StatementDescriptor: descriptor}
	if tx.ProviderRef != nil {
		pay.Reference = *tx.ProviderRef
	}
	paid := tx.UpdatedAt
	if p.CompletedAt != nil {
		paid = *p.CompletedAt
	}
	pay.PaidAt = &paid
	if method != nil {
		buyer.Country = method.Country
		if method.Brand != "" && method.Last4 != "" {
			pay.Card = &domain.InvoiceCard{Brand: method.Brand, Last4: method.Last4}
		}
	}
	// revisions 6.0 (§26.5): a one-off card is not saved, but the processor's
	// details are stored on the payment itself.
	if buyer.Country == "" {
		buyer.Country = tx.CardCountry
	}
	if pay.Card == nil && tx.CardBrand != "" && len(tx.CardLast4) == 4 {
		pay.Card = &domain.InvoiceCard{Brand: tx.CardBrand, Last4: tx.CardLast4}
	}
	purchaseID := p.ID
	return &domain.Invoice{
		Kind: domain.InvoiceKindInvoice, UserID: tx.UserID, PaymentTransactionID: tx.ID, CreditPurchaseID: &purchaseID,
		Currency: tx.Currency, SubtotalCents: tx.AmountCents, TaxCents: 0, TotalCents: tx.AmountCents,
		Lines:  domain.InvoiceLines{{Description: name, Quantity: 1, UnitAmountCents: tx.AmountCents, AmountCents: tx.AmountCents, Credits: tx.Credits}},
		Seller: seller, Payment: pay, Locale: localeOf(user.Locale),
	}, buyer
}

// creditNote builds the credit note of a booked refund: one line for the
// refunded amount, the payment details of the original invoice and the
// refund's own reference and time.
func creditNote(refund *domain.PaymentTransaction, original *domain.Invoice, locale string) *domain.Invoice {
	desc := "Credit pack"
	if len(original.Lines) > 0 {
		desc = original.Lines[0].Description
	}
	pay := domain.InvoicePayment{Provider: refund.Provider, Card: original.Payment.Card,
		StatementDescriptor: original.Payment.StatementDescriptor}
	if refund.ProviderRef != nil {
		pay.Reference = *refund.ProviderRef
	}
	at := refund.UpdatedAt
	pay.RefundedAt = &at
	originalID := original.ID
	return &domain.Invoice{
		Kind: domain.InvoiceKindCreditNote, UserID: refund.UserID, PaymentTransactionID: refund.ID, CreditPurchaseID: original.CreditPurchaseID,
		OriginalInvoiceID: &originalID, OriginalInvoiceNumber: original.Number,
		Currency: refund.Currency, SubtotalCents: refund.AmountCents, TaxCents: 0, TotalCents: refund.AmountCents,
		Lines:  domain.InvoiceLines{{Description: desc, Quantity: 1, UnitAmountCents: refund.AmountCents, AmountCents: refund.AmountCents}},
		Seller: original.Seller, Payment: pay, Locale: localeOf(locale),
	}
}

func localeOf(l string) string {
	switch strings.ToLower(strings.TrimSpace(l)) {
	case "fr", "es":
		return strings.ToLower(strings.TrimSpace(l))
	}
	return "en"
}

// ---------------------------------------------------------------- rendering

// document loads a document, opens its buyer and renders its PDF.
func (s *InvoiceService) document(inv *domain.Invoice) (pdf []byte, filename string, err error) {
	buyer, err := s.buyerOf(inv)
	if err != nil {
		return nil, "", err
	}
	pdf, err = invoicepdf.Render(invoicepdf.Document{Invoice: inv, Buyer: buyer})
	if err != nil {
		return nil, "", err
	}
	return pdf, invoicepdf.Filename(inv), nil
}

// ResolveAttachment renders the PDF named by an e-mail attachment reference.
// Implements AttachmentResolver.
func (s *InvoiceService) ResolveAttachment(ctx context.Context, ref mail.AttachmentRef) (*mail.Attachment, error) {
	id, err := uuid.Parse(ref.ID)
	if err != nil {
		return nil, ErrAttachmentGone
	}
	inv, err := s.d.Repo.Get(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrAttachmentGone
	}
	if err != nil {
		return nil, err
	}
	pdf, name, err := s.document(inv)
	if err != nil {
		return nil, err
	}
	return &mail.Attachment{Filename: name, ContentType: "application/pdf", Content: pdf}, nil
}

// invoiceAttachments names the documents an e-mail of a notification carries:
// the receipt attaches the invoice, a refund notice the credit note.
func invoiceAttachments(kind string, data map[string]any) []mail.AttachmentRef {
	key := ""
	switch kind {
	case NotifyPurchase:
		key = "invoice_id"
	case NotifyPaymentRefunded, NotifyRefundResolved:
		key = "credit_note_id"
	default:
		return nil
	}
	var id string
	switch v := data[key].(type) {
	case string:
		id = v
	case fmt.Stringer:
		id = v.String()
	}
	if _, err := uuid.Parse(id); err != nil {
		return nil
	}
	return []mail.AttachmentRef{{Kind: InvoiceAttachmentKind, ID: id}}
}
