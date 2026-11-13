package domain

import (
	"math"
	"math/bits"
	"time"

	"github.com/google/uuid"
)

// Credit lots (revisions 5.0, docs/platform-extensions.md section 24.3):
// every credit in a wallet belongs to a lot that remembers where it came from
// and what was paid for it.

// LotSource says where the credits of a lot came from.
type LotSource string

// Lot sources, in consumption order.
const (
	LotPurchase   LotSource = "purchase"   // paid credit pack
	LotMembership LotSource = "membership" // Premium monthly grant
	LotBonus      LotSource = "bonus"      // sign-up bonus, courtesy credits, legacy
)

// Lot reference types (reference_type of a lot).
const (
	LotRefCreditPurchase    = "credit_purchase"
	LotRefSubscriptionGrant = "subscription_grant"
)

// Lot origins that are not a ledger entry type.
const (
	LotOriginLegacyBackfill = "legacy_backfill" // migration 0025
	LotOriginLegacyReturn   = "legacy_return"   // credits returned with no allocations to restore
)

// Priority is the consumption order of a source (lower is spent first).
func (s LotSource) Priority() int16 {
	switch s {
	case LotPurchase:
		return 0
	case LotMembership:
		return 1
	}
	return 2
}

// Valid reports whether s is a known source.
func (s LotSource) Valid() bool {
	return s == LotPurchase || s == LotMembership || s == LotBonus
}

// CreditLot is a batch of credits with one origin.
type CreditLot struct {
	ID                       uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID                   uuid.UUID  `gorm:"type:uuid;not null"`
	Source                   LotSource  `gorm:"not null"`
	Priority                 int16      `gorm:"not null"`
	Origin                   string     `gorm:"not null"`
	LedgerEntryID            *uuid.UUID `gorm:"type:uuid"`
	ReferenceType            string     `gorm:"not null"`
	ReferenceID              *uuid.UUID `gorm:"type:uuid"`
	CreditsTotal             int64      `gorm:"not null"`
	CreditsRemaining         int64      `gorm:"not null"`
	PaidCents                int64      `gorm:"not null"`
	Currency                 string     `gorm:"not null"`
	ValueRemainingMillicents int64      `gorm:"not null"`
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

func (CreditLot) TableName() string { return "credit_lots" }

// CreditsUsed is how many credits of the lot were consumed (and not
// restored). Credits held for a pending request count as used.
func (l *CreditLot) CreditsUsed() int64 { return l.CreditsTotal - l.CreditsRemaining }

// CreditLotAllocation is the part of a lot one ledger movement consumed
// (Credits > 0) or restored (Credits < 0). Append-only.
type CreditLotAllocation struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey"`
	LedgerEntryID   uuid.UUID  `gorm:"type:uuid;not null"`
	LotID           uuid.UUID  `gorm:"type:uuid;not null"`
	UserID          uuid.UUID  `gorm:"type:uuid;not null"`
	Credits         int64      `gorm:"not null"`
	ValueMillicents int64      `gorm:"not null"`
	RestoresEntryID *uuid.UUID `gorm:"type:uuid"`
	CreatedAt       time.Time
}

func (CreditLotAllocation) TableName() string { return "credit_lot_allocations" }

// ---------------------------------------------------------------------------
// Pure arithmetic. Prices are at most 1e7 cents and packs at most 1e6
// credits, so a product such as credits × paid × 1000 needs more than 64
// bits; every multiply-then-divide goes through a 128-bit intermediate.
// ---------------------------------------------------------------------------

// mulDiv returns floor(a×b÷c) (or the ceiling when up is set) for a, b ≥ 0
// and c > 0, saturating at MaxInt64. The product is exact (128-bit).
func mulDiv(a, b, c int64, up bool) int64 {
	if a <= 0 || b <= 0 || c <= 0 {
		return 0
	}
	hi, lo := bits.Mul64(uint64(a), uint64(b))
	uc := uint64(c)
	if hi >= uc { // the quotient does not fit in 64 bits
		return math.MaxInt64
	}
	q, r := bits.Div64(hi, lo, uc)
	if up && r != 0 {
		q++
	}
	if q > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(q)
}

// RefundableCents is what a purchase can still refund under the Refund
// Policy: the unused share of what was paid, never more than what is left of
// the payment. lotTotal and lotRemaining are the credits of the purchase's
// lot; credits held for a pending request are not remaining.
//
//	min(amountPaid − alreadyRefunded, floor(remaining × amountPaid ÷ total))
func RefundableCents(lotTotal, lotRemaining, amountPaid, alreadyRefunded int64) int64 {
	if lotTotal <= 0 || amountPaid <= 0 {
		return 0
	}
	left := max(amountPaid-max(alreadyRefunded, 0), 0)
	remaining := min(max(lotRemaining, 0), lotTotal)
	return min(left, mulDiv(remaining, amountPaid, lotTotal, false))
}

// ClawbackCredits is how many credits a refund of refundCents takes back from
// the purchase's own lot: the credits that amount buys, rounded up so the
// customer never keeps credits that were paid back, and never more than what
// is left in the lot.
//
//	min(remaining, ceil(refund × total ÷ paid))
func ClawbackCredits(lotTotal, lotRemaining, amountPaid, refundCents int64) int64 {
	if lotTotal <= 0 || amountPaid <= 0 || refundCents <= 0 || lotRemaining <= 0 {
		return 0
	}
	return min(lotRemaining, mulDiv(refundCents, lotTotal, amountPaid, true))
}

// LotStock is what the consumption planner needs to know about a lot.
type LotStock struct {
	ID        uuid.UUID `gorm:"column:id"`
	Remaining int64     `gorm:"column:credits_remaining"`
	Value     int64     `gorm:"column:value_remaining_millicents"`
}

// LotDelta is a planned change to one lot: Credits and ValueMillicents are
// positive when consumed and negative when restored (the allocation's signs).
type LotDelta struct {
	LotID           uuid.UUID
	Credits         int64
	ValueMillicents int64
}

// PlanConsumption spends credits from lots in the order given. The value an
// allocation carries is floor(credits × value_remaining ÷ credits_remaining);
// the allocation that empties a lot takes all of its remaining value, so the
// allocations of a lot always add up to exactly what was paid. uncovered is
// what the lots could not supply (an invariant break the caller reports).
func PlanConsumption(lots []LotStock, credits int64) (deltas []LotDelta, uncovered int64) {
	left := credits
	for _, l := range lots {
		if left <= 0 {
			break
		}
		if l.Remaining <= 0 {
			continue
		}
		take := min(left, l.Remaining)
		value := l.Value
		if take < l.Remaining {
			value = min(mulDiv(take, l.Value, l.Remaining, false), l.Value)
		}
		deltas = append(deltas, LotDelta{LotID: l.ID, Credits: take, ValueMillicents: value})
		left -= take
	}
	return deltas, max(left, 0)
}

// LotRestoreSource is one lot a movement consumed that can take credits back:
// the credits and value still allocated to the movement (consumed minus
// already restored) and the lot's current room.
type LotRestoreSource struct {
	LotID uuid.UUID
	// Credits and Value are what the movement consumed from the lot that has
	// not been restored yet (both ≥ 0).
	Credits int64
	Value   int64
	// Room is credits_total − credits_remaining; ValueRoom is
	// paid_cents × 1000 − value_remaining_millicents.
	Room      int64
	ValueRoom int64
}

// PlanRestore returns credits to the lots a movement consumed, in the order
// given, up to amount. Restoring a part of an allocation returns
// floor(part × value ÷ credits) of its value, the whole value when the whole
// allocation returns. A lot that has no room for its credits (it cannot
// happen while the invariant holds) is skipped; leftover is what could not be
// restored and must go to a legacy_return lot.
func PlanRestore(sources []LotRestoreSource, amount int64) (deltas []LotDelta, leftover int64) {
	left := amount
	for _, s := range sources {
		if left <= 0 {
			break
		}
		part := min(left, s.Credits, max(s.Room, 0))
		if part <= 0 {
			continue
		}
		value := s.Value
		if part < s.Credits {
			value = mulDiv(part, s.Value, s.Credits, false)
		}
		value = min(value, max(s.ValueRoom, 0))
		deltas = append(deltas, LotDelta{LotID: s.LotID, Credits: -part, ValueMillicents: -value})
		left -= part
	}
	return deltas, max(left, 0)
}
