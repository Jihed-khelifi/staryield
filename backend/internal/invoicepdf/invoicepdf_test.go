package invoicepdf

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 6.0 (§26.6): the invoice PDF.

var issued = time.Date(2026, 11, 19, 14, 30, 0, 0, time.UTC)

func sampleInvoice(locale string) Document {
	paid := issued.Add(-time.Minute)
	id := uuid.New()
	return Document{
		Invoice: &domain.Invoice{
			ID: uuid.New(), Number: "SY-2026-000042", Kind: domain.InvoiceKindInvoice, Currency: "USD", SubtotalCents: 1999, TotalCents: 1999,
			CreditPurchaseID: &id,
			Lines:            domain.InvoiceLines{{Description: "Starter pack", Quantity: 1, UnitAmountCents: 1999, AmountCents: 1999, Credits: 250}},
			Seller: domain.InvoiceSeller{LegalName: "Staryield Media LLC", Address: []string{"30 N Gould St, Ste N", "Sheridan, WY 82801", "United States"},
				Email: "support@staryield.net", Website: "https://staryield.net"},
			Payment: domain.InvoicePayment{Provider: "sandbox_a", Card: &domain.InvoiceCard{Brand: "visa", Last4: "4242"}, Reference: "ch_123",
				PaidAt: &paid, StatementDescriptor: "STARYIELD MEDIA LLC"},
			Locale: locale, IssuedAt: issued,
		},
		Buyer: domain.InvoiceBuyer{Name: "Ada Lovelace", Email: "ada@example.com", Country: "US"},
	}
}

func render(t *testing.T, d Document) []byte {
	t.Helper()
	b, err := Render(d)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func has(b []byte, s string) bool { return bytes.Contains(b, []byte(s)) }

func TestPDFStartsWithTheMagicAndShowsTheFacts(t *testing.T) {
	b := render(t, sampleInvoice("en"))
	if !bytes.HasPrefix(b, []byte("%PDF-")) || !bytes.HasSuffix(bytes.TrimSpace(b), []byte("%%EOF")) {
		t.Fatalf("not a PDF: %q ... %q", b[:8], b[len(b)-8:])
	}
	for _, want := range []string{"SY-2026-000042", "INVOICE", "Staryield Media LLC", "Sheridan, WY 82801", "support@staryield.net", "Ada Lovelace",
		"ada@example.com", "Country: US", "Starter pack", "250 credits", "19.99 USD", "Tax", "Not applicable", "Total paid", "Visa \x95\x95\x95\x95 4242",
		"ch_123", "2026-11-19", "STARYIELD MEDIA LLC", "Credits have no cash value", "https://staryield.net/en/refund-policy"} {
		if !has(b, want) {
			t.Errorf("the PDF lacks %q", want)
		}
	}
}

func TestPDFIsLocalized(t *testing.T) {
	cases := map[string][]string{
		"fr": {"FACTURE", "FACTUR\xc9 \xc0", "Non applicable", "Total pay\xe9", "D\xc9TAILS DU PAIEMENT", "Les cr\xe9dits n'ont aucune valeur mon\xe9taire",
			"Politique de remboursement et d'annulation: https://staryield.net/fr/refund-policy"},
		"es": {"FACTURA", "FACTURADO A", "No aplicable", "Total pagado", "DETALLES DEL PAGO", "Los cr\xe9ditos no tienen valor en efectivo",
			"Pol\xedtica de reembolso y cancelaci\xf3n: https://staryield.net/es/refund-policy", "Descripci\xf3n"},
	}
	for locale, wants := range cases {
		b := render(t, sampleInvoice(locale))
		for _, want := range wants {
			if !has(b, want) {
				t.Errorf("%s: the PDF lacks %q", locale, want)
			}
		}
		if has(b, "Not applicable") || has(b, "BILLED TO") {
			t.Errorf("%s: English labels leaked into a translated PDF", locale)
		}
	}
}

func TestPDFRendersTheAccentsOfEnglishFrenchAndSpanish(t *testing.T) {
	d := sampleInvoice("es")
	d.Buyer.Name = "¿Señor Müller? ¡Ñandú! «André» 5 €"
	d.Invoice.Lines[0].Description = "Paquete café, crème brûlée, español"
	b := render(t, d)
	// cp1252 bytes: ¿ BF, ñ F1, ü FC, ¡ A1, « AB, » BB, é E9, € 80, ú FA, è E8, û FB.
	for _, want := range []string{"\xbfSe\xf1or M\xfcller? \xa1\xd1and\xfa! \xabAndr\xe9\xbb 5 \x80", "caf\xe9, cr\xe8me br\xfbl\xe9e, espa\xf1ol"} {
		if !has(b, want) {
			t.Errorf("accents did not survive the cp1252 translation: want %q", want)
		}
	}
}

func TestPDFTurnsUnprintableTextIntoOneCleanLine(t *testing.T) {
	d := sampleInvoice("en")
	d.Buyer.Name = "Line1\nLine2\t\x00 end 漢字"
	b := render(t, d)
	if !has(b, "Line1 Line2 end ..") {
		t.Fatalf("buyer name not cleaned to one line: %q", string(b))
	}
	if has(b, "Line1\n") {
		t.Fatal("a raw line break reached the PDF text")
	}
}

func TestPDFIsByteIdenticalForTheSameInvoice(t *testing.T) {
	d := sampleInvoice("fr")
	first := render(t, d)
	for i := 0; i < 5; i++ {
		if !bytes.Equal(first, render(t, d)) {
			t.Fatal("rendering the same invoice twice gave different bytes")
		}
	}
	if bytes.Count(first, []byte("/Type /Page\n")) != 1 {
		t.Fatal("an invoice must fit on one page")
	}
	// The creation date is the issue time and the producer is fixed.
	if !has(first, "D:20261119143000") || !has(first, "Staryield") {
		t.Fatal("the PDF metadata must carry the issue time and the fixed producer")
	}
	other := sampleInvoice("fr")
	other.Invoice.Number = "SY-2026-000043"
	if bytes.Equal(first, render(t, other)) {
		t.Fatal("different invoices must differ")
	}
}

func TestCreditNotePDFReferencesTheInvoice(t *testing.T) {
	d := sampleInvoice("en")
	refunded := issued.Add(24 * time.Hour)
	d.Invoice.Number, d.Invoice.Kind = "SYC-2026-000007", domain.InvoiceKindCreditNote
	d.Invoice.OriginalInvoiceNumber = "SY-2026-000042"
	d.Invoice.Payment.PaidAt, d.Invoice.Payment.RefundedAt = nil, &refunded
	d.Invoice.Lines = domain.InvoiceLines{{Description: "Starter pack", Quantity: 1, UnitAmountCents: 500, AmountCents: 500}}
	d.Invoice.SubtotalCents, d.Invoice.TotalCents = 500, 500
	b := render(t, d)
	for _, want := range []string{"CREDIT NOTE", "SYC-2026-000007", "Original invoice: SY-2026-000042", "Total refunded", "5.00 USD", "Refunded on", "2026-11-20 14:30 UTC"} {
		if !has(b, want) {
			t.Errorf("the credit note lacks %q", want)
		}
	}
	if has(b, "Paid on") {
		t.Error("a credit note must not show a paid-on date")
	}
}

func TestPDFShowsTaxWhenTaxIsCollected(t *testing.T) {
	d := sampleInvoice("en")
	d.Invoice.TaxCents, d.Invoice.TotalCents = 200, 2199
	b := render(t, d)
	if !has(b, "2.00 USD") || has(b, "Not applicable") || !has(b, "21.99 USD") {
		t.Fatal("a taxed invoice must show the tax amount and the total")
	}
}

func TestPDFHandlesAnInvoiceWithoutCardOrReference(t *testing.T) {
	d := sampleInvoice("en")
	d.Invoice.Payment = domain.InvoicePayment{Provider: "sandbox_b"}
	d.Buyer.Country = ""
	b := render(t, d)
	if has(b, "Payment method") || has(b, "Payment reference") || has(b, "Country:") {
		t.Fatal("empty details must not print empty rows")
	}
	if !has(b, "sandbox_b") {
		t.Fatal("the provider is always shown")
	}
}

func TestMoneyAndFilename(t *testing.T) {
	for cents, want := range map[int64]string{0: "0.00 USD", 5: "0.05 USD", 1999: "19.99 USD", -250: "-2.50 USD", 123456789: "1234567.89 USD"} {
		if got := Money(cents, "USD"); got != want {
			t.Errorf("Money(%d) = %q, want %q", cents, got, want)
		}
	}
	if got := Filename(&domain.Invoice{Number: "SY-2026-000001"}); got != "SY-2026-000001.pdf" || strings.ContainsAny(got, `"/\ `) {
		t.Errorf("filename %q", got)
	}
}
