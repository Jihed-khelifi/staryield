package domain

import (
	"crypto/rand"
	"errors"
	"io"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// Refund requests (revisions 5.0, docs/platform-extensions.md §24.4).

// RefundReason is why a client asks for money back.
type RefundReason string

// Refund reasons.
const (
	RefundUnusedCredits      RefundReason = "unused_credits"
	RefundTechnicalProblem   RefundReason = "technical_problem"
	RefundIncorrectCharge    RefundReason = "incorrect_charge"
	RefundUnauthorizedCharge RefundReason = "unauthorized_charge"
	RefundReadingConcern     RefundReason = "reading_concern"
	RefundOther              RefundReason = "other"
)

// RefundReasons lists every reason (the database CHECK agrees).
var RefundReasons = []RefundReason{RefundUnusedCredits, RefundTechnicalProblem, RefundIncorrectCharge, RefundUnauthorizedCharge,
	RefundReadingConcern, RefundOther}

// Valid reports whether r is a known reason.
func (r RefundReason) Valid() bool {
	for _, x := range RefundReasons {
		if r == x {
			return true
		}
	}
	return false
}

// RefundChannel is how the request reached us.
type RefundChannel string

// Refund request channels.
const (
	RefundChannelWeb   RefundChannel = "web"
	RefundChannelEmail RefundChannel = "email"
	RefundChannelAdmin RefundChannel = "admin"
)

// RefundRequestStatus is the state of a refund request.
type RefundRequestStatus string

// Refund request states. open is the only state with a way out:
//
//	open → approved | denied | credited | withdrawn
//
// and the four others are terminal.
const (
	RefundOpen      RefundRequestStatus = "open"
	RefundApproved  RefundRequestStatus = "approved"
	RefundDenied    RefundRequestStatus = "denied"
	RefundCredited  RefundRequestStatus = "credited"
	RefundWithdrawn RefundRequestStatus = "withdrawn"
)

// RefundStatuses lists every state (the database CHECK agrees).
var RefundStatuses = []RefundRequestStatus{RefundOpen, RefundApproved, RefundDenied, RefundCredited, RefundWithdrawn}

// refundTransitions is the whole state machine: from → the states it may
// move to. RefundService.transition is the only code that applies a move and
// it asks CanTransition first, with the row locked.
var refundTransitions = map[RefundRequestStatus][]RefundRequestStatus{
	RefundOpen: {RefundApproved, RefundDenied, RefundCredited, RefundWithdrawn},
}

// CanTransition reports whether from → to is a legal move.
func (s RefundRequestStatus) CanTransition(to RefundRequestStatus) bool {
	for _, next := range refundTransitions[s] {
		if next == to {
			return true
		}
	}
	return false
}

// Terminal reports whether the state can never change again.
func (s RefundRequestStatus) Terminal() bool { return len(refundTransitions[s]) == 0 }

// Valid reports whether s is a known state.
func (s RefundRequestStatus) Valid() bool {
	for _, x := range RefundStatuses {
		if s == x {
			return true
		}
	}
	return false
}

// RefundDetailsMax is the longest details text, in characters.
const RefundDetailsMax = 2000

// RefundNoteMax is the longest resolution note, in characters.
const RefundNoteMax = 1000

// RefundQuote is what a credit-pack payment can be refunded for right now
// (the JSON shape of §24.4). Stored on a request as the snapshot the client
// saw; computed live for the admin console.
type RefundQuote struct {
	TransactionID       uuid.UUID `json:"transaction_id" gorm:"-"`
	PackName            string    `json:"pack_name" gorm:"not null"`
	Currency            string    `json:"currency" gorm:"not null"`
	AmountPaidCents     int64     `json:"amount_paid_cents" gorm:"not null"`
	AlreadyRefundedCent int64     `json:"already_refunded_cents" gorm:"column:already_refunded_cents;not null"`
	CreditsPurchased    int64     `json:"credits_purchased" gorm:"not null"`
	CreditsUsed         int64     `json:"credits_used" gorm:"not null"`
	CreditsRemaining    int64     `json:"credits_remaining" gorm:"not null"`
	RefundableCents     int64     `json:"refundable_cents" gorm:"not null"`
	PurchasedAt         time.Time `json:"purchased_at" gorm:"not null"`
	WindowEndsAt        time.Time `json:"window_ends_at" gorm:"not null"`
	WithinWindow        bool      `json:"within_window" gorm:"not null"`
}

// QuoteInput is everything a quote is computed from.
type QuoteInput struct {
	TransactionID uuid.UUID
	PackName      string
	Currency      string
	AmountPaid    int64
	Refunded      int64
	// Credits is what the payment bought (the fallback when the purchase has
	// no lot, e.g. one that predates lots and was spent entirely).
	Credits int64
	// Lot is the purchase's lot, nil when it has none (then nothing is
	// left: remaining credits are what the lot holds, and there is none).
	Lot         *CreditLot
	PurchasedAt time.Time
	WindowDays  int
	Now         time.Time
	// Refundable says whether the payment is in a state that can be refunded
	// at all (succeeded or partially refunded); otherwise the refundable
	// amount is 0.
	Refundable bool
}

// NewRefundQuote computes the quote of a payment. The refundable amount is
// the Refund Policy's: the unused share of what was paid, never more than what
// is left of the payment (RefundableCents). Credits held for a pending custom
// request already left the lot, so they count as used.
func NewRefundQuote(in QuoteInput) RefundQuote {
	purchased, remaining := max(in.Credits, 0), int64(0)
	if in.Lot != nil {
		purchased, remaining = in.Lot.CreditsTotal, in.Lot.CreditsRemaining
	}
	remaining = min(max(remaining, 0), purchased)
	q := RefundQuote{TransactionID: in.TransactionID, PackName: in.PackName, Currency: in.Currency, AmountPaidCents: in.AmountPaid,
		AlreadyRefundedCent: max(in.Refunded, 0), CreditsPurchased: purchased, CreditsUsed: purchased - remaining, CreditsRemaining: remaining,
		PurchasedAt: in.PurchasedAt, WindowEndsAt: in.PurchasedAt.AddDate(0, 0, max(in.WindowDays, 0))}
	q.WithinWindow = !in.Now.After(q.WindowEndsAt)
	if in.Refundable {
		q.RefundableCents = RefundableCents(purchased, remaining, in.AmountPaid, in.Refunded)
	}
	return q
}

// RefundRequest is a client's request to get money back for one credit-pack
// payment.
type RefundRequest struct {
	ID                  uuid.UUID           `gorm:"type:uuid;primaryKey"`
	Reference           string              `gorm:"not null"`
	UserID              uuid.UUID           `gorm:"type:uuid;not null"`
	TransactionID       uuid.UUID           `gorm:"type:uuid;not null"`
	CreditPurchaseID    uuid.UUID           `gorm:"type:uuid;not null"`
	PackID              uuid.UUID           `gorm:"type:uuid;not null"`
	Status              RefundRequestStatus `gorm:"not null"`
	Reason              RefundReason        `gorm:"not null"`
	Details             string              `gorm:"not null"`
	Channel             RefundChannel       `gorm:"not null"`
	IdempotencyKey      string              `gorm:"not null"`
	Quote               RefundQuote         `gorm:"embedded;embeddedPrefix:quote_"`
	ResolutionNote      string              `gorm:"not null"`
	RefundedCents       *int64
	CreditsGranted      *int64
	RefundTransactionID *uuid.UUID `gorm:"type:uuid"`
	ResolvedBy          *uuid.UUID `gorm:"type:uuid"`
	ResolvedAt          *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (RefundRequest) TableName() string { return "refund_requests" }

// Reference format: "RR-" + 8 Crockford base32 characters (no I, L, O, U).
const (
	RefundReferencePrefix = "RR-"
	refundReferenceAlpha  = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	refundReferenceChars  = 8
)

// RefundReferencePattern matches a well-formed reference.
var RefundReferencePattern = regexp.MustCompile(`^RR-[0-9A-HJKMNP-TV-Z]{8}$`)

// NewRefundReference returns a fresh reference read from crypto/rand: 8
// characters of 5 random bits each (the alphabet has exactly 32 symbols, so
// masking a random byte is uniform). Callers retry on a unique violation.
func NewRefundReference() (string, error) { return refundReferenceFrom(rand.Reader) }

func refundReferenceFrom(r io.Reader) (string, error) {
	var b [refundReferenceChars]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return "", errors.New("domain: refund reference: " + err.Error())
	}
	out := make([]byte, 0, len(RefundReferencePrefix)+refundReferenceChars)
	out = append(out, RefundReferencePrefix...)
	for _, v := range b {
		out = append(out, refundReferenceAlpha[v&31])
	}
	return string(out), nil
}
