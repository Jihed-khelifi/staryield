package mail

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
)

// revisions 6.0 (§26.6): invoice and credit note e-mails and attachments.

const invoiceID = "0b6d7c1e-0000-4000-8000-0000000000aa"

func TestReceiptLinksAndNamesTheInvoice(t *testing.T) {
	view := map[string]string{"en": "View invoice", "fr": "Voir la facture", "es": "Ver la factura"}
	attached := map[string]string{"en": "Your invoice is attached to this e-mail as a PDF.", "fr": "Votre facture est jointe à cet e-mail au format PDF.",
		"es": "Tu factura va adjunta a este correo en PDF."}
	for _, locale := range []string{"en", "fr", "es"} {
		data := map[string]any{}
		for k, v := range samples["purchase.succeeded"] {
			data[k] = v
		}
		data["invoice_id"], data["invoice_number"] = invoiceID, "SY-2026-000042"
		c, ok := Notification(locale, "purchase.succeeded", data, "https://staryield.net/"+locale+"/profile")
		if !ok {
			t.Fatal("no receipt")
		}
		r, err := Render(c, Layout{Locale: locale, FrontendURL: "https://staryield.net", Category: "billing", SupportEmail: testSupport})
		if err != nil {
			t.Fatal(err)
		}
		requireFacts(t, locale, r, view[locale], attached[locale], "SY-2026-000042", "https://staryield.net/"+locale+"/profile/invoices/"+invoiceID)
		// The existing receipt facts stay.
		requireFacts(t, locale, r, "19.99 USD", "Starter")
	}
}

// The copy for a receipt_email address (no account link) carries the number
// but no link and no "attached" sentence: the invoice is not attached to it.
func TestReceiptCopyHasNoInvoiceLink(t *testing.T) {
	data := map[string]any{"credits": float64(50), "invoice_id": invoiceID, "invoice_number": "SY-2026-000042"}
	c, _ := Notification("en", "purchase.succeeded", data, "")
	for _, l := range c.Links {
		if strings.Contains(l.URL, "invoices") {
			t.Fatalf("a copy for another address links the account's invoice: %+v", l)
		}
	}
	if strings.Contains(strings.Join(c.Paragraphs, " "), "attached") {
		t.Fatalf("the copy says an invoice is attached: %v", c.Paragraphs)
	}
}

func TestReceiptWithoutInvoiceIsUnchanged(t *testing.T) {
	c, _ := Notification("en", "purchase.succeeded", samples["purchase.succeeded"], "https://staryield.net/en/profile")
	for _, l := range c.Links {
		if strings.Contains(l.URL, "invoices") {
			t.Fatalf("no invoice, yet a link: %+v", l)
		}
	}
	for _, d := range c.Details {
		if d.Label == "Invoice" {
			t.Fatal("no invoice, yet a detail row")
		}
	}
}

func TestRefundEmailsLinkAndNameTheCreditNote(t *testing.T) {
	view := map[string]string{"en": "View credit note", "fr": "Voir l'avoir", "es": "Ver la nota de crédito"}
	attached := map[string]string{"en": "Your credit note is attached to this e-mail as a PDF.", "fr": "Votre avoir est joint à cet e-mail au format PDF.",
		"es": "Tu nota de crédito va adjunta a este correo en PDF."}
	for _, kind := range []string{"payment.refunded", "refund_request.resolved"} {
		for _, locale := range []string{"en", "fr", "es"} {
			data := map[string]any{}
			for k, v := range samples[kind] {
				data[k] = v
			}
			data["credit_note_id"], data["credit_note_number"] = invoiceID, "SYC-2026-000007"
			c, ok := Notification(locale, kind, data, "https://staryield.net/"+locale+"/profile/settings")
			if !ok {
				t.Fatalf("%s: no e-mail", kind)
			}
			r, err := Render(c, Layout{Locale: locale, FrontendURL: "https://staryield.net", Category: "billing", SupportEmail: testSupport})
			if err != nil {
				t.Fatal(err)
			}
			requireFacts(t, locale+"/"+kind, r, view[locale], attached[locale], "SYC-2026-000007",
				"https://staryield.net/"+locale+"/profile/invoices/"+invoiceID)
		}
	}
}

func TestCloudflareSendsAttachmentsBase64Encoded(t *testing.T) {
	f, c := newFake(t, http.StatusOK, okBody)
	pdf := []byte("%PDF-1.3\nbinary \x00\xff\xfe data")
	_, err := c.Send(context.Background(), Message{To: "a@example.com", Subject: "Receipt", Text: "Hi", Category: "billing",
		Attachments: []Attachment{{Filename: "SY-2026-000001.pdf", ContentType: "application/pdf", Content: pdf}, {Filename: "note.bin", Content: []byte{1, 2, 3}}}})
	if err != nil {
		t.Fatal(err)
	}
	atts, _ := f.requests[0]["attachments"].([]any)
	if len(atts) != 2 {
		t.Fatalf("attachments: %v", f.requests[0]["attachments"])
	}
	first, _ := atts[0].(map[string]any)
	got, err := base64.StdEncoding.DecodeString(first["content"].(string))
	if err != nil || string(got) != string(pdf) {
		t.Fatalf("content did not round-trip: %v", err)
	}
	if first["filename"] != "SY-2026-000001.pdf" || first["type"] != "application/pdf" || first["disposition"] != "attachment" {
		t.Fatalf("attachment fields: %v", first)
	}
	second, _ := atts[1].(map[string]any)
	if second["type"] != "application/octet-stream" {
		t.Fatalf("a missing content type defaults to octet-stream: %v", second)
	}
}

func TestCloudflareOmitsAttachmentsWhenThereAreNone(t *testing.T) {
	f, c := newFake(t, http.StatusOK, okBody)
	if _, err := c.Send(context.Background(), Message{To: "a@example.com", Subject: "s", Text: "t"}); err != nil {
		t.Fatal(err)
	}
	if _, present := f.requests[0]["attachments"]; present {
		t.Fatalf("attachments key sent without attachments: %v", f.requests[0])
	}
}

func TestCloudflareRefusesAnOversizedMessageWithoutRetrying(t *testing.T) {
	f, c := newFake(t, http.StatusOK, okBody)
	_, err := c.Send(context.Background(), Message{To: "a@example.com", Subject: "s", Text: "t",
		Attachments: []Attachment{{Filename: "big.pdf", ContentType: "application/pdf", Content: make([]byte, 20<<20)}}})
	se, ok := err.(*SendError)
	if !ok || se.Retryable || se.Reason != "message_too_large" {
		t.Fatalf("an oversized message must fail permanently: %v", err)
	}
	if len(f.requests) != 0 {
		t.Fatal("an oversized message must not reach the provider")
	}
}

func TestDriversRecordAttachmentsWithoutLeakingContent(t *testing.T) {
	rec := &Recorder{}
	m := Message{To: "a@example.com", Subject: "s", Text: "t", Attachments: []Attachment{{Filename: "a.pdf", ContentType: "application/pdf", Content: []byte("PDFDATA")}}}
	if _, err := rec.Send(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	sent := rec.Sent()
	if len(sent) != 1 || len(sent[0].Attachments) != 1 || sent[0].Attachments[0].Filename != "a.pdf" || string(sent[0].Attachments[0].Content) != "PDFDATA" {
		t.Fatalf("recorder: %+v", sent)
	}
	fields := attachmentSummary(m.Attachments)
	if len(fields) != 2 {
		t.Fatalf("summary fields: %v", fields)
	}
	for _, f := range fields {
		if strings.Contains(f.String, "PDFDATA") {
			t.Fatal("the summary carries the content")
		}
	}
	if attachmentSummary(nil) != nil {
		t.Fatal("no attachments, no fields")
	}
	// The log and no-op drivers accept messages with attachments.
	if _, err := (LogMailer{From: "x"}).Send(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	if _, err := (NoopMailer{}).Send(context.Background(), m); err != nil {
		t.Fatal(err)
	}
}
