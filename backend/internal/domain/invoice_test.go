package domain

import (
	"testing"
	"time"
)

// revisions 6.0 (§26.6): numbering.

func TestInvoiceNumbersAndScopes(t *testing.T) {
	at := time.Date(2026, 3, 9, 10, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		kind  InvoiceKind
		n     int64
		scope string
		num   string
	}{
		{InvoiceKindInvoice, 1, "invoice:2026", "SY-2026-000001"},
		{InvoiceKindInvoice, 123456, "invoice:2026", "SY-2026-123456"},
		{InvoiceKindInvoice, 1234567, "invoice:2026", "SY-2026-1234567"}, // grows past six digits, never truncates
		{InvoiceKindCreditNote, 7, "credit_note:2026", "SYC-2026-000007"},
	} {
		if got := InvoiceScope(c.kind, at); got != c.scope {
			t.Errorf("scope(%s) = %q, want %q", c.kind, got, c.scope)
		}
		if got := InvoiceNumber(c.kind, at, c.n); got != c.num {
			t.Errorf("number(%s, %d) = %q, want %q", c.kind, c.n, got, c.num)
		}
	}
}

// The year is the UTC year of issued_at: a document issued a minute before
// midnight UTC on 31 December belongs to that year whatever the server's zone.
func TestInvoiceYearRolloverIsUTC(t *testing.T) {
	paris := time.FixedZone("CET", 3600)
	lastMinute := time.Date(2026, 12, 31, 23, 59, 0, 0, time.UTC)
	firstMinute := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	if got := InvoiceScope(InvoiceKindInvoice, lastMinute.In(paris)); got != "invoice:2026" {
		t.Errorf("23:59 UTC on 31 December in Paris time (00:59 on 1 January) = %q, want 2026", got)
	}
	if got := InvoiceScope(InvoiceKindInvoice, firstMinute.In(time.FixedZone("W", -3600))); got != "invoice:2027" {
		t.Errorf("00:00 UTC on 1 January in a western zone (23:00 on 31 December) = %q, want 2027", got)
	}
	if InvoiceNumber(InvoiceKindInvoice, firstMinute, 1) != "SY-2027-000001" {
		t.Error("the counter restarts each year")
	}
}

func TestInvoiceKindValid(t *testing.T) {
	if !InvoiceKindInvoice.Valid() || !InvoiceKindCreditNote.Valid() || InvoiceKind("receipt").Valid() || InvoiceKind("").Valid() {
		t.Fatal("kind validation")
	}
}

func TestInvoiceJSONColumnsRoundTrip(t *testing.T) {
	paid := time.Date(2026, 11, 19, 1, 2, 3, 0, time.UTC)
	lines := InvoiceLines{{Description: "Pack", Quantity: 1, UnitAmountCents: 500, AmountCents: 500, Credits: 60}}
	v, err := lines.Value()
	if err != nil {
		t.Fatal(err)
	}
	var back InvoiceLines
	if err := back.Scan(v); err != nil || len(back) != 1 || back[0] != lines[0] {
		t.Fatalf("lines: %+v %v", back, err)
	}
	pay := InvoicePayment{Provider: "stripe", Card: &InvoiceCard{Brand: "visa", Last4: "4242"}, PaidAt: &paid, Reference: "ch_1"}
	v, _ = pay.Value()
	var pb InvoicePayment
	if err := pb.Scan([]byte(v.(string))); err != nil || pb.Card == nil || pb.Card.Last4 != "4242" || !pb.PaidAt.Equal(paid) || pb.RefundedAt != nil {
		t.Fatalf("payment: %+v %v", pb, err)
	}
	seller := InvoiceSeller{LegalName: "X", Address: []string{"a", "b"}, Email: "e", Website: "w"}
	v, _ = seller.Value()
	var sb InvoiceSeller
	if err := sb.Scan(v); err != nil || sb.LegalName != "X" || len(sb.Address) != 2 {
		t.Fatalf("seller: %+v %v", sb, err)
	}
	// NULL columns scan to empty values; nil lines are stored as an empty array.
	var empty InvoiceLines
	if err := empty.Scan(nil); err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("null lines: %v %v", empty, err)
	}
	if v, _ := InvoiceLines(nil).Value(); v != "[]" {
		t.Fatalf("nil lines stored as %v", v)
	}
	if err := empty.Scan(42); err == nil {
		t.Fatal("an unsupported source must fail")
	}
}
