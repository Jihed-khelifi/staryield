package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Billing: Premium Membership plans, saved payment methods, subscriptions,
// a single money-movement ledger (payment_transactions) and provider
// routing rules. Card data never reaches Staryield: payment methods store
// sealed provider references plus display metadata (brand, last4, expiry).

// Ledger entry types for membership credit grants.
const (
	LedgerSubscriptionGrant         LedgerEntryType = "subscription_grant"
	LedgerSubscriptionGrantReversal LedgerEntryType = "subscription_grant_reversal"
)

// LocalizedText is a JSONB object mapping a locale to text.
type LocalizedText map[string]string

// For returns the text for locale, falling back to English.
func (t LocalizedText) For(locale string) string {
	if v := t[locale]; v != "" {
		return v
	}
	return t["en"]
}

// Value implements driver.Valuer.
func (t LocalizedText) Value() (driver.Value, error) {
	if t == nil {
		return "{}", nil
	}
	b, err := json.Marshal(map[string]string(t))
	return string(b), err
}

// Scan implements sql.Scanner.
func (t *LocalizedText) Scan(src any) error {
	b, err := jsonBytes(src)
	if err != nil || b == nil {
		*t = LocalizedText{}
		return err
	}
	out := map[string]string{}
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	*t = out
	return nil
}

// LocalizedList is a JSONB object mapping a locale to a list of lines.
type LocalizedList map[string][]string

// For returns the lines for locale, falling back to English.
func (l LocalizedList) For(locale string) []string {
	if v := l[locale]; len(v) > 0 {
		return v
	}
	if v := l["en"]; v != nil {
		return v
	}
	return []string{}
}

// Value implements driver.Valuer.
func (l LocalizedList) Value() (driver.Value, error) {
	if l == nil {
		return "{}", nil
	}
	b, err := json.Marshal(map[string][]string(l))
	return string(b), err
}

// Scan implements sql.Scanner.
func (l *LocalizedList) Scan(src any) error {
	b, err := jsonBytes(src)
	if err != nil || b == nil {
		*l = LocalizedList{}
		return err
	}
	out := map[string][]string{}
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	*l = out
	return nil
}

func jsonBytes(src any) ([]byte, error) {
	switch v := src.(type) {
	case nil:
		return nil, nil
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	}
	return nil, errors.New("domain: unsupported JSON source")
}

// SubscriptionPlan is a membership product (admin-managed, BR-09). Prices
// and benefits live on immutable PlanVersions.
type SubscriptionPlan struct {
	ID          uuid.UUID     `gorm:"type:uuid;primaryKey"`
	Code        string        `gorm:"not null"`
	Name        LocalizedText `gorm:"type:jsonb;not null"`
	Description LocalizedText `gorm:"type:jsonb;not null"`
	Active      bool          `gorm:"not null"`
	SortOrder   int           `gorm:"not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt
}

func (SubscriptionPlan) TableName() string { return "subscription_plans" }

// PlanInterval is a billing period unit.
type PlanInterval string

// Billing period units.
const (
	IntervalMonth PlanInterval = "month"
	IntervalYear  PlanInterval = "year"
)

// PlanVersion is one immutable price and benefit set of a plan. Existing
// subscriptions keep the version they started on (grandfathering); a new
// price is a new version.
type PlanVersion struct {
	ID              uuid.UUID    `gorm:"type:uuid;primaryKey"`
	PlanID          uuid.UUID    `gorm:"type:uuid;not null"`
	Version         int          `gorm:"not null"`
	PriceCents      int64        `gorm:"not null"`
	Currency        string       `gorm:"not null"`
	BillingInterval PlanInterval `gorm:"not null"`
	IntervalCount   int          `gorm:"not null"`
	// MonthlyCreditGrant credits are added to the wallet once per paid month.
	MonthlyCreditGrant int64 `gorm:"not null"`
	// ChatDiscountBps is the discount on billed chat minutes (100 = 1%).
	ChatDiscountBps int `gorm:"not null"`
	// MonthlyFreeMinutes free chat minutes are granted per paid month; the
	// unused ones of the previous grant expire.
	MonthlyFreeMinutes int `gorm:"not null"`
	// WaitlistPriority members are notified first when an advisor frees up.
	WaitlistPriority bool          `gorm:"not null"`
	Benefits         LocalizedList `gorm:"type:jsonb;not null"`
	IsCurrent        bool          `gorm:"not null"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (PlanVersion) TableName() string { return "subscription_plan_versions" }

// PeriodMonths is the length of one billing period in months.
func (v *PlanVersion) PeriodMonths() int {
	n := max(v.IntervalCount, 1)
	if v.BillingInterval == IntervalYear {
		return 12 * n
	}
	return n
}

// PaymentCustomer links a user to a provider-side customer (vault).
type PaymentCustomer struct {
	ID                     uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID                 uuid.UUID `gorm:"type:uuid;not null"`
	Provider               string    `gorm:"not null"`
	ProviderCustomerRefEnc string    `gorm:"not null"`
	CreatedAt              time.Time
	UpdatedAt              time.Time
	DeletedAt              gorm.DeletedAt
}

func (PaymentCustomer) TableName() string { return "payment_customers" }

// PaymentMethodStatus is the usability of a saved card.
type PaymentMethodStatus string

// Payment method statuses.
const (
	MethodActive   PaymentMethodStatus = "active"
	MethodExpired  PaymentMethodStatus = "expired"
	MethodFailed   PaymentMethodStatus = "failed" // hard decline: replace before retrying
	MethodDetached PaymentMethodStatus = "detached"
)

// PaymentMethod is a saved card: a sealed provider reference plus display
// metadata. The card number never reaches Staryield.
type PaymentMethod struct {
	ID                   uuid.UUID           `gorm:"type:uuid;primaryKey"`
	UserID               uuid.UUID           `gorm:"type:uuid;not null"`
	CustomerID           uuid.UUID           `gorm:"type:uuid;not null"`
	Provider             string              `gorm:"not null"`
	ProviderMethodRefEnc string              `gorm:"not null"`
	FingerprintHash      string              `gorm:"not null"`
	Brand                string              `gorm:"not null"`
	Last4                string              `gorm:"column:last4;not null"`
	ExpMonth             int                 `gorm:"not null"`
	ExpYear              int                 `gorm:"not null"`
	Country              string              `gorm:"not null"`
	IsDefault            bool                `gorm:"not null"`
	Status               PaymentMethodStatus `gorm:"not null"`
	NetworkTxnRefEnc     string              `gorm:"not null"`
	IdempotencyKey       *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	DeletedAt            gorm.DeletedAt
}

func (PaymentMethod) TableName() string { return "payment_methods" }

// ExpiredAt reports whether the card has expired at t (cards are valid
// through the last day of their expiry month).
func (m *PaymentMethod) ExpiredAt(t time.Time) bool {
	t = t.UTC()
	return t.Year() > m.ExpYear || (t.Year() == m.ExpYear && int(t.Month()) > m.ExpMonth)
}

// Usable reports whether the method can be charged.
func (m *PaymentMethod) Usable(t time.Time) bool {
	return m.Status == MethodActive && !m.ExpiredAt(t)
}

// SubscriptionStatus is the membership lifecycle.
type SubscriptionStatus string

// Subscription statuses.
//
//	incomplete → active (first charge succeeded) | expired (not resolved within 24h)
//	active     → past_due (renewal failed) | canceled (period end, admin, dispute, closure)
//	past_due   → active (retry or new card succeeded) | expired (retries/grace exhausted) | canceled
//	canceled, expired: terminal
const (
	SubIncomplete SubscriptionStatus = "incomplete"
	SubActive     SubscriptionStatus = "active"
	SubPastDue    SubscriptionStatus = "past_due"
	SubCanceled   SubscriptionStatus = "canceled"
	SubExpired    SubscriptionStatus = "expired"
)

// Open reports whether the subscription still counts as the user's current
// membership (at most one open subscription per user).
func (s SubscriptionStatus) Open() bool {
	return s == SubIncomplete || s == SubActive || s == SubPastDue
}

var subscriptionTransitions = map[SubscriptionStatus][]SubscriptionStatus{
	SubIncomplete: {SubActive, SubExpired, SubCanceled},
	SubActive:     {SubPastDue, SubCanceled},
	SubPastDue:    {SubActive, SubExpired, SubCanceled},
}

// CanTransition reports whether from → to is a legal subscription move.
func (s SubscriptionStatus) CanTransition(to SubscriptionStatus) bool {
	for _, next := range subscriptionTransitions[s] {
		if next == to {
			return true
		}
	}
	return false
}

// Subscription is a user's membership.
type Subscription struct {
	ID                 uuid.UUID          `gorm:"type:uuid;primaryKey"`
	UserID             uuid.UUID          `gorm:"type:uuid;not null"`
	PlanID             uuid.UUID          `gorm:"type:uuid;not null"`
	PlanVersionID      uuid.UUID          `gorm:"type:uuid;not null"`
	Status             SubscriptionStatus `gorm:"not null"`
	CancelAtPeriodEnd  bool               `gorm:"not null"`
	PaymentMethodID    *uuid.UUID         `gorm:"type:uuid"`
	Provider           string             `gorm:"not null"`
	BillingAnchorDay   int                `gorm:"not null"`
	CurrentPeriodStart *time.Time
	CurrentPeriodEnd   *time.Time
	// PeriodIndex is the number of paid periods so far (1 after the first charge).
	PeriodIndex int `gorm:"not null"`
	// NextGrantAt schedules the next monthly grant inside a multi-month period.
	NextGrantAt *time.Time
	GrantIndex  int `gorm:"not null"`
	// NextAttemptAt is when the next charge is due (period end, or a dunning retry).
	NextAttemptAt *time.Time
	AttemptCount  int `gorm:"not null"`
	// LockedUntil is the claim lease of a worker processing this row.
	LockedUntil    *time.Time
	GraceUntil     *time.Time
	CanceledAt     *time.Time
	EndedAt        *time.Time
	CancelReason   string `gorm:"not null"`
	IdempotencyKey string `gorm:"not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (Subscription) TableName() string { return "subscriptions" }

// InvoiceStatus is the state of one billing period's charge.
type InvoiceStatus string

// Invoice statuses.
const (
	InvoiceOpen     InvoiceStatus = "open"
	InvoicePaid     InvoiceStatus = "paid"
	InvoiceFailed   InvoiceStatus = "failed"
	InvoiceVoid     InvoiceStatus = "void"
	InvoiceRefunded InvoiceStatus = "refunded"
)

// SubscriptionInvoice is the amount due for one billing period.
type SubscriptionInvoice struct {
	ID                   uuid.UUID     `gorm:"type:uuid;primaryKey"`
	SubscriptionID       uuid.UUID     `gorm:"type:uuid;not null"`
	UserID               uuid.UUID     `gorm:"type:uuid;not null"`
	PeriodIndex          int           `gorm:"not null"`
	PeriodStart          time.Time     `gorm:"not null"`
	PeriodEnd            time.Time     `gorm:"not null"`
	AmountCents          int64         `gorm:"not null"`
	Currency             string        `gorm:"not null"`
	Status               InvoiceStatus `gorm:"not null"`
	AttemptCount         int           `gorm:"not null"`
	PaidAt               *time.Time
	PaymentTransactionID *uuid.UUID `gorm:"type:uuid"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (SubscriptionInvoice) TableName() string { return "subscription_invoices" }

// TransactionKind classifies money movements.
type TransactionKind string

// Transaction kinds.
const (
	TxCreditPurchase      TransactionKind = "credit_purchase"
	TxSubscriptionInitial TransactionKind = "subscription_initial"
	TxSubscriptionRenewal TransactionKind = "subscription_renewal"
	TxRefund              TransactionKind = "refund"
)

// TransactionStatus is a money movement's outcome.
type TransactionStatus string

// Transaction statuses.
const (
	TxPending           TransactionStatus = "pending"
	TxSucceeded         TransactionStatus = "succeeded"
	TxFailed            TransactionStatus = "failed"
	TxRefunded          TransactionStatus = "refunded"
	TxPartiallyRefunded TransactionStatus = "partially_refunded"
	TxDisputed          TransactionStatus = "disputed"
)

// Refundable reports whether some of the amount can still be refunded.
func (t *PaymentTransaction) Refundable() int64 {
	if t.Kind == TxRefund || (t.Status != TxSucceeded && t.Status != TxPartiallyRefunded) {
		return 0
	}
	return max(t.AmountCents-t.RefundedAmountCents, 0)
}

// PaymentTransaction is one money movement with a provider: the single
// ledger for credit purchases, subscription charges and refunds.
type PaymentTransaction struct {
	ID                  uuid.UUID         `gorm:"type:uuid;primaryKey"`
	UserID              uuid.UUID         `gorm:"type:uuid;not null"`
	Kind                TransactionKind   `gorm:"not null"`
	Status              TransactionStatus `gorm:"not null"`
	AmountCents         int64             `gorm:"not null"`
	RefundedAmountCents int64             `gorm:"not null"`
	Currency            string            `gorm:"not null"`
	Credits             int64             `gorm:"not null"`
	Provider            string            `gorm:"not null"`
	ProviderRef         *string
	PaymentMethodID     *uuid.UUID `gorm:"type:uuid"`
	SubscriptionID      *uuid.UUID `gorm:"type:uuid"`
	InvoiceID           *uuid.UUID `gorm:"type:uuid"`
	CreditPurchaseID    *uuid.UUID `gorm:"type:uuid"`
	ParentTransactionID *uuid.UUID `gorm:"type:uuid"`
	FailureCode         string     `gorm:"not null"`
	Description         string     `gorm:"not null"`
	IdempotencyKey      string     `gorm:"not null"`
	AttemptNo           int        `gorm:"not null"`
	// LockedUntil is the lease of the charge attempt in flight: after it
	// expires the outcome is reconciled with the provider (Lookup).
	LockedUntil *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (PaymentTransaction) TableName() string { return "payment_transactions" }

// RoutingPurpose is what a payment is for.
type RoutingPurpose string

// Routing purposes ("any" rules match every purpose).
const (
	PurposeAny          RoutingPurpose = "any"
	PurposeCredits      RoutingPurpose = "credits"
	PurposeSubscription RoutingPurpose = "subscription"
)

// Valid reports whether p is a known purpose.
func (p RoutingPurpose) Valid() bool {
	return p == PurposeAny || p == PurposeCredits || p == PurposeSubscription
}

// RoutingRule selects payment providers. The first enabled rule (lowest
// priority number) whose non-empty match lists all contain the request's
// values wins; its providers are tried in order.
type RoutingRule struct {
	ID              uuid.UUID      `gorm:"type:uuid;primaryKey"`
	Name            string         `gorm:"not null"`
	Priority        int            `gorm:"not null"`
	Enabled         bool           `gorm:"not null"`
	Purpose         RoutingPurpose `gorm:"not null"`
	Providers       StringList     `gorm:"type:jsonb;not null"`
	MatchCountries  StringList     `gorm:"type:jsonb;not null"`
	MatchCurrencies StringList     `gorm:"type:jsonb;not null"`
	MatchPlanIDs    StringList     `gorm:"column:match_plan_ids;type:jsonb;not null"`
	MatchUserIDs    StringList     `gorm:"column:match_user_ids;type:jsonb;not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       gorm.DeletedAt
}

func (RoutingRule) TableName() string { return "payment_routing_rules" }

// ProviderWebhookEvent records a processed provider notification so a
// redelivered event is acknowledged without being applied twice. The raw
// payload is never stored.
type ProviderWebhookEvent struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	Provider      string    `gorm:"not null"`
	EventID       string    `gorm:"not null"`
	EventType     string    `gorm:"not null"`
	PayloadSHA256 string    `gorm:"column:payload_sha256;not null"`
	ProcessedAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (ProviderWebhookEvent) TableName() string { return "webhook_events" }

// SubscriptionGrant records one monthly membership grant (credits and free
// minutes) so it is applied exactly once and can be reversed on refund.
type SubscriptionGrant struct {
	ID                 uuid.UUID `gorm:"type:uuid;primaryKey"`
	SubscriptionID     uuid.UUID `gorm:"type:uuid;not null"`
	UserID             uuid.UUID `gorm:"type:uuid;not null"`
	GrantIndex         int       `gorm:"not null"`
	PeriodIndex        int       `gorm:"not null"`
	Credits            int64     `gorm:"not null"`
	CreditsReversed    int64     `gorm:"not null"`
	FreeMinutes        int       `gorm:"not null"`
	FreeMinutesRevoked int       `gorm:"not null"`
	// FreeMinutesAfter is the wallet's free-minute balance right after the
	// grant; the next grant uses it to compute how many granted minutes are
	// still unused (and expire).
	FreeMinutesAfter int       `gorm:"not null"`
	GrantedAt        time.Time `gorm:"not null"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (SubscriptionGrant) TableName() string { return "subscription_grants" }

// AddMonthsAnchored moves t by months, keeping the billing anchor day and
// clamping it to the month length: anchored on the 31st, Jan 31 → Feb 28
// (29 in leap years) → Mar 31. The time of day is preserved.
func AddMonthsAnchored(t time.Time, months, anchorDay int) time.Time {
	m := int(t.Month()) - 1 + months
	y := t.Year() + floorDiv(m, 12)
	month := time.Month(floorMod(m, 12) + 1)
	day := min(max(anchorDay, 1), daysIn(y, month))
	return time.Date(y, month, day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

func daysIn(y int, m time.Month) int {
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func floorMod(a, b int) int { return a - floorDiv(a, b)*b }

// DunningSchedule is when renewal attempts are made, relative to the end of
// the unpaid period: at the end, then +24h, +72h and +120h.
var DunningSchedule = []time.Duration{0, 24 * time.Hour, 72 * time.Hour, 120 * time.Hour}

// NextRetryAt returns the next renewal attempt after attemptsMade failed
// attempts, or false when retries are exhausted.
func NextRetryAt(periodEnd time.Time, attemptsMade int) (time.Time, bool) {
	if attemptsMade < 0 || attemptsMade >= len(DunningSchedule) {
		return time.Time{}, false
	}
	return periodEnd.Add(DunningSchedule[attemptsMade]), true
}

// ApplyDiscount returns the discounted per-minute rate for a member: the
// list rate reduced by bps basis points, rounded in the customer's favour,
// never below one credit. Advisor earnings stay on the list rate (the
// discount is platform-funded).
func ApplyDiscount(listRate int64, bps int) int64 {
	if bps <= 0 || listRate <= 0 {
		return listRate
	}
	bps = min(bps, 10000)
	return max(listRate*int64(10000-bps)/10000, 1)
}
