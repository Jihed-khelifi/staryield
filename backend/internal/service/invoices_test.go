package service

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
)

// revisions 6.0 (§26.6): snapshots, e-mail references, the accounting export
// and the attachment resolution of the outbox.

func sampleTx() *domain.PaymentTransaction {
	return &domain.PaymentTransaction{ID: uuid.New(), UserID: uuid.New(), Kind: domain.TxCreditPurchase, Status: domain.TxSucceeded, AmountCents: 1999,
		Currency: "USD", Credits: 250, Provider: "sandbox_a", ProviderRef: ptr("ch_9"), Description: "Starter pack",
		UpdatedAt: time.Date(2026, 11, 19, 12, 5, 0, 0, time.UTC)}
}

func TestPurchaseInvoiceSnapshot(t *testing.T) {
	tx := sampleTx()
	done := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	p := &domain.CreditPurchase{ID: uuid.New(), UserID: tx.UserID, CompletedAt: &done}
	user := &domain.User{ID: tx.UserID, DisplayName: "Ada", Email: "ada@example.com", Locale: "fr"}
	method := &domain.PaymentMethod{Brand: "visa", Last4: "4242", Country: "FR"}
	seller := domain.InvoiceSeller{LegalName: "Staryield Media LLC"}
	inv, buyer := purchaseInvoice(tx, p, user, method, seller, "STARYIELD MEDIA LLC")
	if inv.Kind != domain.InvoiceKindInvoice || inv.UserID != tx.UserID || inv.PaymentTransactionID != tx.ID || *inv.CreditPurchaseID != p.ID {
		t.Fatalf("links: %+v", inv)
	}
	if inv.Currency != "USD" || inv.SubtotalCents != 1999 || inv.TaxCents != 0 || inv.TotalCents != 1999 {
		t.Fatalf("totals: %+v", inv)
	}
	if len(inv.Lines) != 1 || inv.Lines[0] != (domain.InvoiceLine{Description: "Starter pack", Quantity: 1, UnitAmountCents: 1999, AmountCents: 1999, Credits: 250}) {
		t.Fatalf("lines: %+v", inv.Lines)
	}
	if buyer != (domain.InvoiceBuyer{Name: "Ada", Email: "ada@example.com", Country: "FR"}) {
		t.Fatalf("buyer: %+v", buyer)
	}
	pay := inv.Payment
	if pay.Provider != "sandbox_a" || pay.Reference != "ch_9" || pay.Card == nil || pay.Card.Brand != "visa" || pay.Card.Last4 != "4242" ||
		!pay.PaidAt.Equal(done) || pay.RefundedAt != nil || pay.StatementDescriptor != "STARYIELD MEDIA LLC" {
		t.Fatalf("payment: %+v", pay)
	}
	if inv.Locale != "fr" || inv.Seller.LegalName != "Staryield Media LLC" {
		t.Fatalf("locale/seller: %q %+v", inv.Locale, inv.Seller)
	}
	if inv.Number != "" || inv.ID != uuid.Nil || !inv.IssuedAt.IsZero() || inv.BuyerSealed != "" {
		t.Fatal("number, id, date and the sealed buyer are set when the invoice is issued")
	}
}

func TestPurchaseInvoiceWithoutCardOrCompletionTime(t *testing.T) {
	tx := sampleTx()
	tx.ProviderRef, tx.Description = nil, "  "
	p := &domain.CreditPurchase{ID: uuid.New()}
	user := &domain.User{DisplayName: "Ada", Email: "ada@example.com", Locale: "de"}
	inv, buyer := purchaseInvoice(tx, p, user, nil, domain.InvoiceSeller{}, "")
	if inv.Payment.Card != nil || inv.Payment.Reference != "" || buyer.Country != "" {
		t.Fatalf("no card, no reference, no country: %+v %+v", inv.Payment, buyer)
	}
	if !inv.Payment.PaidAt.Equal(tx.UpdatedAt) {
		t.Fatalf("paid time falls back to the payment's: %v", inv.Payment.PaidAt)
	}
	if inv.Lines[0].Description != "Credit pack" || inv.Locale != "en" {
		t.Fatalf("defaults: %+v %q", inv.Lines, inv.Locale)
	}
	// A card without brand or digits is not printed.
	_, _ = purchaseInvoice(tx, p, user, &domain.PaymentMethod{Brand: "visa"}, domain.InvoiceSeller{}, "")
	inv, _ = purchaseInvoice(tx, p, user, &domain.PaymentMethod{Brand: "visa"}, domain.InvoiceSeller{}, "")
	if inv.Payment.Card != nil {
		t.Fatal("a card needs both brand and last four digits")
	}
}

func TestCreditNoteSnapshot(t *testing.T) {
	purchaseID := uuid.New()
	original := &domain.Invoice{ID: uuid.New(), Number: "SY-2026-000042", Kind: domain.InvoiceKindInvoice, CreditPurchaseID: &purchaseID,
		Lines:   domain.InvoiceLines{{Description: "Starter pack", Quantity: 1, UnitAmountCents: 1999, AmountCents: 1999, Credits: 250}},
		Seller:  domain.InvoiceSeller{LegalName: "Staryield Media LLC"},
		Payment: domain.InvoicePayment{Card: &domain.InvoiceCard{Brand: "visa", Last4: "4242"}, StatementDescriptor: "STARYIELD MEDIA LLC"}}
	refund := &domain.PaymentTransaction{ID: uuid.New(), UserID: uuid.New(), Kind: domain.TxRefund, AmountCents: 500, Currency: "USD",
		Provider: "sandbox_a", ProviderRef: ptr("re_1"), UpdatedAt: time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)}
	cn := creditNote(refund, original, "es")
	if cn.Kind != domain.InvoiceKindCreditNote || *cn.OriginalInvoiceID != original.ID || cn.OriginalInvoiceNumber != "SY-2026-000042" ||
		cn.PaymentTransactionID != refund.ID || *cn.CreditPurchaseID != purchaseID {
		t.Fatalf("links: %+v", cn)
	}
	if cn.TotalCents != 500 || cn.SubtotalCents != 500 || cn.TaxCents != 0 || cn.Currency != "USD" {
		t.Fatalf("a credit note is for the amount refunded: %+v", cn)
	}
	if len(cn.Lines) != 1 || cn.Lines[0].Description != "Starter pack" || cn.Lines[0].AmountCents != 500 || cn.Lines[0].Credits != 0 {
		t.Fatalf("lines: %+v", cn.Lines)
	}
	if cn.Payment.Reference != "re_1" || cn.Payment.RefundedAt == nil || cn.Payment.PaidAt != nil || cn.Payment.Card.Last4 != "4242" ||
		cn.Payment.StatementDescriptor != "STARYIELD MEDIA LLC" || cn.Locale != "es" {
		t.Fatalf("payment: %+v locale %q", cn.Payment, cn.Locale)
	}
}

func TestSettledStatus(t *testing.T) {
	for st, want := range map[domain.TransactionStatus]bool{domain.TxSucceeded: true, domain.TxPartiallyRefunded: true, domain.TxRefunded: true,
		domain.TxDisputed: true, domain.TxPending: false, domain.TxFailed: false} {
		if settledStatus(st) != want {
			t.Errorf("settledStatus(%s) = %v", st, !want)
		}
	}
}

func TestInvoiceAttachmentsOfNotifications(t *testing.T) {
	id := uuid.New()
	for _, c := range []struct {
		kind string
		data map[string]any
		want int
	}{
		{NotifyPurchase, map[string]any{"invoice_id": id.String()}, 1},
		{NotifyPurchase, map[string]any{"invoice_id": id}, 1}, // in-process value before the database round trip
		{NotifyPaymentRefunded, map[string]any{"credit_note_id": id.String()}, 1},
		{NotifyRefundResolved, map[string]any{"credit_note_id": id.String()}, 1},
		{NotifyPurchase, map[string]any{"credit_note_id": id.String()}, 0}, // wrong key for the kind
		{NotifyPurchase, map[string]any{"invoice_id": "not-a-uuid"}, 0},
		{NotifyPurchase, map[string]any{}, 0},
		{NotifyChatMissed, map[string]any{"invoice_id": id.String()}, 0}, // other kinds never attach
	} {
		got := invoiceAttachments(c.kind, c.data)
		if len(got) != c.want || (c.want == 1 && (got[0].Kind != InvoiceAttachmentKind || got[0].ID != id.String())) {
			t.Errorf("%s %v: %+v", c.kind, c.data, got)
		}
	}
}

func TestExportRange(t *testing.T) {
	for _, c := range []struct {
		from, to string
		field    string
	}{
		{"", "2026-02-01", "from"}, {"2026-01-01", "", "from"}, {"x", "2026-02-01", "from"}, {"2026-01-01", "y", "to"},
		{"2026-02-01", "2026-01-01", "to"}, {"2025-01-01", "2026-01-02", "to"}, // 367 days inclusive
	} {
		_, _, err := ExportRange(c.from, c.to)
		ae, ok := apperror.As(err)
		fields, _ := ae.Details["fields"].(map[string]any)
		if !ok || ae.Status != 422 || fields[c.field] == nil {
			t.Errorf("ExportRange(%q, %q): %v", c.from, c.to, err)
		}
	}
	f, to, err := ExportRange("2026-01-01", "2026-01-01")
	if err != nil || to.Sub(f) != 24*time.Hour {
		t.Fatalf("a single day: %v %v", to.Sub(f), err)
	}
	// 366 days is allowed (a leap year), 367 is not.
	if _, _, err := ExportRange("2024-01-01", "2024-12-31"); err != nil {
		t.Fatalf("366 days: %v", err)
	}
	if _, _, err := ExportRange("2024-01-01", "2025-01-01"); err == nil {
		t.Fatal("367 days must be refused")
	}
	if f, _, err := ExportRange("2026-01-01T10:30:00+02:00", "2026-01-03"); err != nil || f.Hour() != 8 {
		t.Fatalf("RFC 3339 bounds are normalised to UTC: %v %v", f, err)
	}
}

func TestExportRowsAreSignedAndSafe(t *testing.T) {
	inv := &domain.Invoice{Number: "SY-2026-000001", Kind: domain.InvoiceKindInvoice, IssuedAt: time.Date(2026, 11, 19, 1, 2, 3, 0, time.UTC),
		Currency: "USD", SubtotalCents: 1999, TotalCents: 1999, Payment: domain.InvoicePayment{Provider: "sandbox_a", Reference: "=HYPERLINK(1)"}}
	row := exportRow(inv, domain.InvoiceBuyer{Country: "FR"})
	want := []string{"SY-2026-000001", "invoice", "2026-11-19T01:02:03Z", "", "FR", "USD", "19.99", "0.00", "19.99", "sandbox_a", "'=HYPERLINK(1)"}
	if strings.Join(row, "|") != strings.Join(want, "|") || len(row) != len(exportHeader) {
		t.Fatalf("row: %v", row)
	}
	cn := &domain.Invoice{Number: "SYC-2026-000001", Kind: domain.InvoiceKindCreditNote, OriginalInvoiceNumber: "SY-2026-000001", IssuedAt: inv.IssuedAt,
		Currency: "USD", SubtotalCents: 5, TotalCents: 5}
	r := exportRow(cn, domain.InvoiceBuyer{})
	if r[3] != "SY-2026-000001" || r[6] != "-0.05" || r[8] != "-0.05" {
		t.Fatalf("credit notes are negative in the export: %v", r)
	}
	for in, out := range map[string]string{"": "", "FR": "FR", "=1+1": "'=1+1", "+1": "'+1", "-1": "'-1", "@x": "'@x", "\tx": "'\tx"} {
		if got := csvText(in); got != out {
			t.Errorf("csvText(%q) = %q, want %q", in, got, out)
		}
	}
	for cents, want := range map[int64]string{0: "0.00", 1: "0.01", -1: "-0.01", 123456: "1234.56", -100: "-1.00"} {
		if decimal(cents) != want {
			t.Errorf("decimal(%d) = %q", cents, decimal(cents))
		}
	}
	// The row survives a CSV round trip.
	var sb strings.Builder
	w := csv.NewWriter(&sb)
	_ = w.Write(row)
	w.Flush()
	back, err := csv.NewReader(strings.NewReader(sb.String())).Read()
	if err != nil || strings.Join(back, "|") != strings.Join(want, "|") {
		t.Fatalf("round trip: %v %v", back, err)
	}
}

func TestLikePrefixEscapesWildcards(t *testing.T) {
	// (repository.likePrefix is unexported; the behaviour is covered by the e2e search test.)
	if localeOf("FR") != "fr" || localeOf(" es ") != "es" || localeOf("de") != "en" || localeOf("") != "en" {
		t.Fatal("locale normalisation")
	}
}

// ------------------------------------------------------- outbox attachments

type fakeResolver struct {
	files map[string]*mail.Attachment
	err   map[string]error
	calls int
}

func (f *fakeResolver) ResolveAttachment(_ context.Context, ref mail.AttachmentRef) (*mail.Attachment, error) {
	f.calls++
	if err := f.err[ref.ID]; err != nil {
		return nil, err
	}
	return f.files[ref.ID], nil
}

func withAttachments(user uuid.UUID, refs ...mail.AttachmentRef) EmailRequest {
	r := req(user, domain.EmailBilling, "notification:purchase.succeeded")
	r.Attachments = refs
	return r
}

func TestOutboxStoresReferencesAndAttachesTheResolvedFile(t *testing.T) {
	svc, repo, rec, _ := newTestMail(t)
	res := &fakeResolver{files: map[string]*mail.Attachment{"inv-1": {Filename: "SY-2026-000001.pdf", ContentType: "application/pdf", Content: []byte("%PDF-fake")}}}
	svc.RegisterAttachments("invoice", res)
	user := uuid.New()
	if err := svc.Enqueue(context.Background(), withAttachments(user, mail.AttachmentRef{Kind: "invoice", ID: "inv-1"})); err != nil {
		t.Fatal(err)
	}
	rows := repo.byTemplate("notification:purchase.succeeded")
	if len(rows) != 1 {
		t.Fatalf("rows: %+v", rows)
	}
	raw, err := svc.sealer.Open(rows[0].PayloadSealed, outboxAD(rows[0].ID))
	if err != nil {
		t.Fatal(err)
	}
	var stored emailPayload
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		t.Fatal(err)
	}
	if len(stored.Attachments) != 1 || stored.Attachments[0] != (mail.AttachmentRef{Kind: "invoice", ID: "inv-1"}) || strings.Contains(raw, "%PDF") {
		t.Fatalf("the outbox stores the reference, never the file: %s", raw)
	}
	if res.calls != 0 {
		t.Fatal("the file must be resolved at delivery, not when queued")
	}
	if _, err := svc.Drain(context.Background()); err != nil {
		t.Fatal(err)
	}
	sent := rec.Sent()
	if len(sent) != 1 || len(sent[0].Attachments) != 1 || sent[0].Attachments[0].Filename != "SY-2026-000001.pdf" || string(sent[0].Attachments[0].Content) != "%PDF-fake" {
		t.Fatalf("sent: %+v", sent)
	}
}

func TestOutboxSendsWithoutAMissingDocument(t *testing.T) {
	svc, _, rec, _ := newTestMail(t)
	svc.RegisterAttachments("invoice", &fakeResolver{files: map[string]*mail.Attachment{"ok": {Filename: "a.pdf", Content: []byte("x")}},
		err: map[string]error{"gone": ErrAttachmentGone}})
	user := uuid.New()
	// One document gone, one unknown kind, one nil result (also "gone"), one present.
	refs := []mail.AttachmentRef{{Kind: "invoice", ID: "gone"}, {Kind: "mystery", ID: "x"}, {Kind: "invoice", ID: "nil"}, {Kind: "invoice", ID: "ok"}}
	if err := svc.Enqueue(context.Background(), withAttachments(user, refs...)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Drain(context.Background()); err != nil {
		t.Fatal(err)
	}
	sent := rec.Sent()
	if len(sent) != 1 || len(sent[0].Attachments) != 1 || sent[0].Attachments[0].Filename != "a.pdf" {
		t.Fatalf("a missing document must not block the message nor carry a placeholder: %+v", sent)
	}
}

func TestOutboxRetriesATransientResolverFailureThenSendsWithoutOnTheLastAttempt(t *testing.T) {
	svc, repo, rec, clk := newTestMail(t) // MaxAttempts: 3
	res := &fakeResolver{err: map[string]error{"inv": errors.New("database down")}}
	svc.RegisterAttachments("invoice", res)
	user := uuid.New()
	if err := svc.Enqueue(context.Background(), withAttachments(user, mail.AttachmentRef{Kind: "invoice", ID: "inv"})); err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if n, _ := svc.DeliverDue(context.Background(), 10); n != 1 {
			t.Fatalf("attempt %d delivered %d rows", attempt, n)
		}
		if len(rec.Sent()) != 0 {
			t.Fatalf("attempt %d: the message was sent while the document was only temporarily unavailable", attempt)
		}
		rows := repo.byTemplate("notification:purchase.succeeded")
		if rows[0].Status != domain.EmailQueued || rows[0].LastError == "" {
			t.Fatalf("attempt %d: row %+v", attempt, rows[0])
		}
		clk.Advance(2 * time.Hour)
	}
	// Last attempt: the receipt matters more than its attachment.
	if n, _ := svc.DeliverDue(context.Background(), 10); n != 1 {
		t.Fatal("last attempt not delivered")
	}
	sent := rec.Sent()
	if len(sent) != 1 || len(sent[0].Attachments) != 0 {
		t.Fatalf("the last attempt sends without the attachment: %+v", sent)
	}
	if rows := repo.byTemplate("notification:purchase.succeeded"); rows[0].Status != domain.EmailSent {
		t.Fatalf("status %s", rows[0].Status)
	}
}

func TestOutboxWithoutAResolverSendsWithoutAndCapsReferences(t *testing.T) {
	svc, _, rec, _ := newTestMail(t)
	user := uuid.New()
	if err := svc.Enqueue(context.Background(), withAttachments(user, mail.AttachmentRef{Kind: "invoice", ID: "x"})); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Drain(context.Background()); err != nil || len(rec.Sent()) != 1 || len(rec.Sent()[0].Attachments) != 0 {
		t.Fatalf("no resolver registered: %+v %v", rec.Sent(), err)
	}
	// References beyond maxAttachmentRefs are dropped when queued.
	svc2, _, rec2, _ := newTestMail(t)
	files := map[string]*mail.Attachment{}
	var refs []mail.AttachmentRef
	for i := 0; i < maxAttachmentRefs+3; i++ {
		id := uuid.NewString()
		files[id] = &mail.Attachment{Filename: id + ".pdf", Content: []byte("x")}
		refs = append(refs, mail.AttachmentRef{Kind: "invoice", ID: id})
	}
	svc2.RegisterAttachments("invoice", &fakeResolver{files: files})
	_ = svc2.Enqueue(context.Background(), withAttachments(uuid.New(), refs...))
	_, _ = svc2.Drain(context.Background())
	if got := len(rec2.Sent()[0].Attachments); got != maxAttachmentRefs {
		t.Fatalf("attachments per message: %d, want at most %d", got, maxAttachmentRefs)
	}
}

func TestNewInvoiceServiceDefaults(t *testing.T) {
	s := NewInvoiceService(InvoiceDeps{Log: zap.NewNop()}, InvoiceSettings{FrontendURL: "https://staryield.net/", SupportEmail: " "})
	if s.cfg.BackfillWindow != 45*24*time.Hour || s.cfg.BackfillBatch != 100 {
		t.Fatalf("defaults: %+v", s.cfg)
	}
	seller := s.seller()
	if seller.LegalName != mail.CompanyLegalName || seller.Email != mail.DefaultSupportEmail || seller.Website != "https://staryield.net" ||
		strings.Join(seller.Address, ", ") != mail.CompanyAddress {
		t.Fatalf("seller: %+v", seller)
	}
	// The snapshot is a copy: editing it never changes the package's address.
	seller.Address[0] = "changed"
	if mail.CompanyAddressLines[0] == "changed" {
		t.Fatal("the seller snapshot must copy the address lines")
	}
}
