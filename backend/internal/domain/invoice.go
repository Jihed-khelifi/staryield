package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// revisions 6.0 (docs/platform-extensions.md §26.6): Staryield invoices and
// credit notes. (SubscriptionInvoice, from the membership billing, is a
// different thing: a period of a membership.)

// InvoiceKind tells invoices and credit notes apart.
type InvoiceKind string

// Invoice kinds.
const (
	InvoiceKindInvoice    InvoiceKind = "invoice"
	InvoiceKindCreditNote InvoiceKind = "credit_note"
)

// Valid reports whether k is a known kind.
func (k InvoiceKind) Valid() bool { return k == InvoiceKindInvoice || k == InvoiceKindCreditNote }

// NumberPrefix is the prefix of the document number ("SY-2026-000001",
// "SYC-2026-000001").
func (k InvoiceKind) NumberPrefix() string {
	if k == InvoiceKindCreditNote {
		return "SYC"
	}
	return "SY"
}

// InvoiceLine is one line of a document. Amounts are minor units of the
// document's currency.
type InvoiceLine struct {
	Description     string `json:"description"`
	Quantity        int64  `json:"quantity"`
	UnitAmountCents int64  `json:"unit_amount_cents"`
	AmountCents     int64  `json:"amount_cents"`
	Credits         int64  `json:"credits,omitempty"`
}

// InvoiceLines is the JSONB array of lines.
type InvoiceLines []InvoiceLine

// Value implements driver.Valuer.
func (l InvoiceLines) Value() (driver.Value, error) { return jsonValue([]InvoiceLine(l), "[]") }

// Scan implements sql.Scanner.
func (l *InvoiceLines) Scan(src any) error {
	out := []InvoiceLine{}
	if err := jsonScan(src, &out); err != nil {
		return err
	}
	*l = out
	return nil
}

// InvoiceSeller is the issuing company, copied from the company details at
// issue time.
type InvoiceSeller struct {
	LegalName string   `json:"legal_name"`
	Address   []string `json:"address"`
	Email     string   `json:"email"`
	Website   string   `json:"website"`
}

// Value implements driver.Valuer.
func (s InvoiceSeller) Value() (driver.Value, error) { return jsonValue(s, "{}") }

// Scan implements sql.Scanner.
func (s *InvoiceSeller) Scan(src any) error {
	out := InvoiceSeller{}
	if err := jsonScan(src, &out); err != nil {
		return err
	}
	*s = out
	return nil
}

// InvoiceCard is the card behind a payment: brand and last four digits only.
type InvoiceCard struct {
	Brand string `json:"brand"`
	Last4 string `json:"last4"`
}

// InvoicePayment is how the document was paid (or refunded).
type InvoicePayment struct {
	Provider            string       `json:"provider"`
	Card                *InvoiceCard `json:"card,omitempty"`
	Reference           string       `json:"reference,omitempty"`
	PaidAt              *time.Time   `json:"paid_at,omitempty"`
	RefundedAt          *time.Time   `json:"refunded_at,omitempty"`
	StatementDescriptor string       `json:"statement_descriptor,omitempty"`
}

// Value implements driver.Valuer.
func (p InvoicePayment) Value() (driver.Value, error) { return jsonValue(p, "{}") }

// Scan implements sql.Scanner.
func (p *InvoicePayment) Scan(src any) error {
	out := InvoicePayment{}
	if err := jsonScan(src, &out); err != nil {
		return err
	}
	*p = out
	return nil
}

// InvoiceBuyer is the buyer snapshot. It is stored sealed (Invoice.BuyerSealed).
type InvoiceBuyer struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Country string `json:"country,omitempty"`
}

// Invoice is an issued invoice or credit note. Immutable once issued.
type Invoice struct {
	ID     uuid.UUID   `gorm:"type:uuid;primaryKey"`
	Number string      `gorm:"not null"`
	Kind   InvoiceKind `gorm:"not null"`
	UserID uuid.UUID   `gorm:"type:uuid;not null"`
	// PaymentTransactionID is the purchase (invoice) or the refund (credit
	// note) the document is about.
	PaymentTransactionID  uuid.UUID      `gorm:"type:uuid;not null"`
	CreditPurchaseID      *uuid.UUID     `gorm:"type:uuid"`
	OriginalInvoiceID     *uuid.UUID     `gorm:"type:uuid"`
	OriginalInvoiceNumber string         `gorm:"not null"`
	Currency              string         `gorm:"not null"`
	SubtotalCents         int64          `gorm:"not null"`
	TaxCents              int64          `gorm:"not null"`
	TotalCents            int64          `gorm:"not null"`
	Lines                 InvoiceLines   `gorm:"type:jsonb;not null"`
	BuyerSealed           string         `gorm:"not null"`
	Seller                InvoiceSeller  `gorm:"type:jsonb;not null"`
	Payment               InvoicePayment `gorm:"type:jsonb;not null"`
	Locale                string         `gorm:"not null"`
	IssuedAt              time.Time      `gorm:"not null"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// TableName implements gorm's Tabler.
func (Invoice) TableName() string { return "invoices" }

// InvoiceSequence is the gapless counter of one kind and year.
type InvoiceSequence struct {
	Scope     string `gorm:"primaryKey"`
	Next      int64  `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName implements gorm's Tabler.
func (InvoiceSequence) TableName() string { return "invoice_sequences" }

// InvoiceScope is the sequence scope of a document issued at t ("invoice:2026").
func InvoiceScope(kind InvoiceKind, t time.Time) string {
	return fmt.Sprintf("%s:%04d", string(kind), t.UTC().Year())
}

// InvoiceNumber formats the sequence value n of a kind and year
// ("SY-2026-000001").
func InvoiceNumber(kind InvoiceKind, t time.Time, n int64) string {
	return fmt.Sprintf("%s-%04d-%06d", kind.NumberPrefix(), t.UTC().Year(), n)
}

func jsonValue(v any, empty string) (driver.Value, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if string(b) == "null" {
		return empty, nil
	}
	return string(b), nil
}

func jsonScan(src any, out any) error {
	var b []byte
	switch v := src.(type) {
	case nil:
		return nil
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("domain: JSON column: unsupported source")
	}
	return json.Unmarshal(b, out)
}
