// Package invoicepdf renders Staryield invoices and credit notes as PDF
// (revisions 6.0, docs/platform-extensions.md §26.6).
//
// The output is a pure function of the document: A4, the standard Helvetica
// font with the cp1252 code page (which covers the accents, ¿ ¡ € and « » of
// English, French and Spanish), a fixed producer, the creation date set to the
// issue time and no random identifiers, so rendering the same invoice twice
// gives the same bytes. Streams are left uncompressed: a document is a few
// kilobytes and its text stays searchable and testable.
package invoicepdf

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"unicode"

	"codeberg.org/go-pdf/fpdf"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/i18n"
)

// Brand colours (the e-mail layout's palette): gold rule, ink text, paper fill.
var (
	gold  = [3]int{0xAA, 0x8E, 0x62}
	ink   = [3]int{0x15, 0x10, 0x0B}
	muted = [3]int{0x6B, 0x60, 0x55}
	paper = [3]int{0xF4, 0xF0, 0xEA}
)

// Page geometry in millimetres (A4 portrait).
const (
	marginX   = 18.0
	pageW     = 210.0
	contentW  = pageW - 2*marginX
	colQty    = 16.0
	colUnit   = 34.0
	colAmount = 34.0
	colDesc   = contentW - colQty - colUnit - colAmount
	lineH     = 5.2
)

// Document is what a PDF shows.
type Document struct {
	Invoice *domain.Invoice
	Buyer   domain.InvoiceBuyer
}

// Filename is the download name of a document ("SY-2026-000001.pdf").
func Filename(inv *domain.Invoice) string { return inv.Number + ".pdf" }

// Money formats minor units as "12.34 USD" (the platform's convention in
// every e-mail and receipt).
func Money(cents int64, currency string) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s%d.%02d %s", sign, cents/100, cents%100, currency)
}

// Render returns the PDF of a document in the invoice's own locale.
func Render(d Document) ([]byte, error) {
	inv := d.Invoice
	locale := i18n.Normalize(inv.Locale)
	t := func(s string) string { return i18n.T(locale, s) }

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.SetCatalogSort(true)
	pdf.SetCreationDate(inv.IssuedAt.UTC())
	pdf.SetModificationDate(inv.IssuedAt.UTC())
	pdf.SetProducer("Staryield", false)
	pdf.SetCreator("Staryield", false)
	pdf.SetMargins(marginX, 16, marginX)
	pdf.SetAutoPageBreak(true, 22)
	cp := pdf.UnicodeTranslatorFromDescriptor("cp1252")
	// clean makes any text safe for the code page: one line of printable runes.
	clean := func(s string) string { return cp(printable(s)) }
	title := t("Invoice")
	if inv.Kind == domain.InvoiceKindCreditNote {
		title = t("Credit note")
	}
	pdf.SetTitle(clean(title+" "+inv.Number), false)
	pdf.AddPage()

	text := func(size float64, style string, c [3]int) {
		pdf.SetFont("Helvetica", style, size)
		pdf.SetTextColor(c[0], c[1], c[2])
	}
	cell := func(w, h float64, s, align string) { pdf.CellFormat(w, h, clean(s), "", 0, align, false, 0, "") }

	// Header: the seller on the left, the title and number on the right.
	top := pdf.GetY()
	text(15, "B", ink)
	cell(110, 7, inv.Seller.LegalName, "L")
	pdf.Ln(7)
	text(9, "", muted)
	for _, line := range inv.Seller.Address {
		cell(110, 4.4, line, "L")
		pdf.Ln(4.4)
	}
	if inv.Seller.Email != "" {
		cell(110, 4.4, inv.Seller.Email, "L")
		pdf.Ln(4.4)
	}
	if inv.Seller.Website != "" {
		cell(110, 4.4, inv.Seller.Website, "L")
		pdf.Ln(4.4)
	}
	left := pdf.GetY()
	pdf.SetXY(pageW-marginX-70, top)
	text(22, "B", ink)
	cell(70, 10, strings.ToUpper(title), "R")
	pdf.SetXY(pageW-marginX-70, top+11)
	text(11, "B", gold)
	cell(70, 6, inv.Number, "R")
	pdf.SetXY(pageW-marginX-70, top+18)
	text(9, "", muted)
	cell(70, 5, t("Issue date")+": "+inv.IssuedAt.UTC().Format("2006-01-02"), "R")
	if inv.Kind == domain.InvoiceKindCreditNote && inv.OriginalInvoiceNumber != "" {
		pdf.SetXY(pageW-marginX-70, top+23)
		cell(70, 5, t("Original invoice")+": "+inv.OriginalInvoiceNumber, "R")
	}
	pdf.SetY(max(left, top+30) + 3)

	// The gold rule.
	pdf.SetDrawColor(gold[0], gold[1], gold[2])
	pdf.SetLineWidth(0.7)
	pdf.Line(marginX, pdf.GetY(), pageW-marginX, pdf.GetY())
	pdf.Ln(7)

	// Buyer.
	text(8.5, "B", gold)
	cell(contentW, 4.5, strings.ToUpper(t("Billed to")), "L")
	pdf.Ln(5.2)
	text(10.5, "B", ink)
	pdf.MultiCell(contentW/2, 5.2, clean(d.Buyer.Name), "", "L", false)
	text(9.5, "", muted)
	if d.Buyer.Email != "" {
		pdf.MultiCell(contentW/2, 4.8, clean(d.Buyer.Email), "", "L", false)
	}
	if d.Buyer.Country != "" {
		pdf.MultiCell(contentW/2, 4.8, clean(t("Country")+": "+d.Buyer.Country), "", "L", false)
	}
	pdf.Ln(8)

	// Lines table.
	pdf.SetFillColor(paper[0], paper[1], paper[2])
	text(8.5, "B", ink)
	pdf.CellFormat(colDesc, 7.5, clean("  "+t("Description")), "", 0, "L", true, 0, "")
	pdf.CellFormat(colQty, 7.5, clean(t("Qty")), "", 0, "R", true, 0, "")
	pdf.CellFormat(colUnit, 7.5, clean(t("Unit price")), "", 0, "R", true, 0, "")
	pdf.CellFormat(colAmount, 7.5, clean(t("Amount")+"  "), "", 0, "R", true, 0, "")
	pdf.Ln(7.5)
	pdf.SetDrawColor(paper[0]-24, paper[1]-24, paper[2]-24)
	pdf.SetLineWidth(0.2)
	for _, l := range inv.Lines {
		y := pdf.GetY()
		text(10, "", ink)
		pdf.SetX(marginX + 2)
		desc := l.Description
		pdf.MultiCell(colDesc-2, lineH, clean(desc), "", "L", false)
		if l.Credits > 0 {
			text(8.5, "", muted)
			pdf.SetX(marginX + 2)
			pdf.MultiCell(colDesc-2, 4.4, clean(fmt.Sprintf("%d %s", l.Credits, t("credits"))), "", "L", false)
		}
		end := pdf.GetY()
		text(10, "", ink)
		pdf.SetXY(marginX+colDesc, y)
		cell(colQty, lineH, fmt.Sprintf("%d", l.Quantity), "R")
		cell(colUnit, lineH, Money(l.UnitAmountCents, inv.Currency), "R")
		cell(colAmount-2, lineH, Money(l.AmountCents, inv.Currency), "R")
		pdf.SetY(end + 2.5)
		pdf.Line(marginX, pdf.GetY(), pageW-marginX, pdf.GetY())
		pdf.Ln(2.5)
	}
	pdf.Ln(3)

	// Totals.
	labelW := 52.0
	totalsX := pageW - marginX - labelW - colAmount
	row := func(label, value string, bold bool) {
		style := ""
		if bold {
			style = "B"
		}
		text(10, style, ink)
		pdf.SetX(totalsX)
		cell(labelW, 6, label, "L")
		cell(colAmount-2, 6, value, "R")
		pdf.Ln(6)
	}
	row(t("Subtotal"), Money(inv.SubtotalCents, inv.Currency), false)
	taxValue := t("Not applicable")
	if inv.TaxCents > 0 {
		taxValue = Money(inv.TaxCents, inv.Currency)
	}
	row(t("Tax"), taxValue, false)
	pdf.SetDrawColor(gold[0], gold[1], gold[2])
	pdf.SetLineWidth(0.4)
	pdf.Line(totalsX, pdf.GetY()+0.5, pageW-marginX-2, pdf.GetY()+0.5)
	pdf.Ln(2)
	totalLabel := t("Total paid")
	if inv.Kind == domain.InvoiceKindCreditNote {
		totalLabel = t("Total refunded")
	}
	row(totalLabel, Money(inv.TotalCents, inv.Currency), true)
	pdf.Ln(8)

	// Payment details.
	text(8.5, "B", gold)
	cell(contentW, 4.5, strings.ToUpper(t("Payment details")), "L")
	pdf.Ln(5.5)
	detail := func(label, value string) {
		if value == "" {
			return
		}
		text(9.5, "", muted)
		cell(48, 5, label, "L")
		text(9.5, "", ink)
		pdf.MultiCell(contentW-48, 5, clean(value), "", "L", false)
	}
	p := inv.Payment
	detail(t("Payment provider"), p.Provider)
	if p.Card != nil && p.Card.Last4 != "" {
		detail(t("Payment method"), cardText(p.Card))
	}
	detail(t("Payment reference"), p.Reference)
	if p.PaidAt != nil {
		detail(t("Paid on"), dateTime(*p.PaidAt))
	}
	if p.RefundedAt != nil {
		detail(t("Refunded on"), dateTime(*p.RefundedAt))
	}
	detail(t("Statement descriptor"), p.StatementDescriptor)

	// Footer: fixed at the bottom of the page (inside the bottom margin, so no
	// page break may split it).
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetY(-36)
	pdf.SetDrawColor(gold[0], gold[1], gold[2])
	pdf.SetLineWidth(0.3)
	pdf.Line(marginX, pdf.GetY(), pageW-marginX, pdf.GetY())
	pdf.Ln(3)
	text(8.5, "", muted)
	pdf.MultiCell(contentW, 4.2, clean(t("Credits have no cash value and cannot be exchanged for money.")), "", "L", false)
	if site := strings.TrimRight(inv.Seller.Website, "/"); site != "" {
		pdf.MultiCell(contentW, 4.2, clean(t("Refund & Cancellation Policy")+": "+site+"/"+locale+"/refund-policy"), "", "L", false)
	}
	if inv.Seller.Email != "" {
		pdf.MultiCell(contentW, 4.2, clean(t("Support")+": "+inv.Seller.Email), "", "L", false)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("invoicepdf: %w", err)
	}
	return buf.Bytes(), nil
}

// cardText is "Visa •••• 4242".
func cardText(c *domain.InvoiceCard) string {
	brand := strings.TrimSpace(c.Brand)
	switch strings.ToLower(brand) {
	case "visa":
		brand = "Visa"
	case "mastercard":
		brand = "Mastercard"
	case "amex":
		brand = "American Express"
	case "discover":
		brand = "Discover"
	case "":
	default:
		brand = strings.ToUpper(brand[:1]) + brand[1:]
	}
	return strings.TrimSpace(brand + " •••• " + c.Last4)
}

func dateTime(t time.Time) string { return t.UTC().Format("2006-01-02 15:04") + " UTC" }

// printable collapses whitespace and drops control characters so a value
// (a buyer's display name, a descriptor) always renders as one clean line.
func printable(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			space = true
		case unicode.IsControl(r):
		default:
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(r)
		}
	}
	return b.String()
}
