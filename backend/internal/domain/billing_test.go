package domain

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 9, 30, 15, 0, time.UTC) }

func TestAddMonthsAnchoredKeepsTheAnchorDay(t *testing.T) {
	cases := []struct {
		name   string
		from   time.Time
		months int
		anchor int
		want   time.Time
	}{
		{"jan31 to feb (non-leap)", date(2025, time.January, 31), 1, 31, date(2025, time.February, 28)},
		{"feb28 back to mar31", date(2025, time.February, 28), 1, 31, date(2025, time.March, 31)},
		{"mar31 to apr30", date(2025, time.March, 31), 1, 31, date(2025, time.April, 30)},
		{"jan31 to feb29 (leap)", date(2024, time.January, 31), 1, 31, date(2024, time.February, 29)},
		{"feb29 to mar31 (leap)", date(2024, time.February, 29), 1, 31, date(2024, time.March, 31)},
		{"dec31 rolls the year", date(2025, time.December, 31), 1, 31, date(2026, time.January, 31)},
		{"anchor 30 in february", date(2026, time.January, 30), 1, 30, date(2026, time.February, 28)},
		{"mid-month unchanged", date(2026, time.May, 15), 1, 15, date(2026, time.June, 15)},
		{"yearly plan", date(2024, time.February, 29), 12, 29, date(2025, time.February, 28)},
		{"quarterly", date(2026, time.November, 30), 3, 30, date(2027, time.February, 28)},
		{"negative months", date(2026, time.March, 31), -1, 31, date(2026, time.February, 28)},
	}
	for _, c := range cases {
		if got := AddMonthsAnchored(c.from, c.months, c.anchor); !got.Equal(c.want) {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
	// A year of monthly renewals anchored on the 31st never drifts.
	at := date(2026, time.January, 31)
	for i := 0; i < 12; i++ {
		at = AddMonthsAnchored(at, 1, 31)
	}
	if !at.Equal(date(2027, time.January, 31)) {
		t.Fatalf("drift after 12 renewals: %s", at)
	}
}

func TestDunningSchedule(t *testing.T) {
	end := date(2026, time.March, 1)
	want := []time.Duration{0, 24 * time.Hour, 72 * time.Hour, 120 * time.Hour}
	for i, d := range want {
		at, ok := NextRetryAt(end, i)
		if !ok || !at.Equal(end.Add(d)) {
			t.Fatalf("attempt %d: %s %v", i, at, ok)
		}
	}
	if _, ok := NextRetryAt(end, len(want)); ok {
		t.Fatal("retries must be exhausted after the schedule")
	}
	if _, ok := NextRetryAt(end, -1); ok {
		t.Fatal("negative attempts")
	}
}

func TestApplyDiscountFavoursTheCustomer(t *testing.T) {
	cases := []struct {
		rate int64
		bps  int
		want int64
	}{
		{35, 0, 35}, {35, 1000, 31}, {100, 2500, 75}, {1, 5000, 1}, {3, 5000, 1}, {10, 10000, 1}, {0, 1000, 0}, {50, -5, 50}, {40, 20000, 1},
	}
	for _, c := range cases {
		if got := ApplyDiscount(c.rate, c.bps); got != c.want {
			t.Errorf("ApplyDiscount(%d, %d) = %d, want %d", c.rate, c.bps, got, c.want)
		}
	}
}

func TestSubscriptionTransitions(t *testing.T) {
	allowed := map[[2]SubscriptionStatus]bool{
		{SubIncomplete, SubActive}: true, {SubIncomplete, SubExpired}: true, {SubIncomplete, SubCanceled}: true,
		{SubActive, SubPastDue}: true, {SubActive, SubCanceled}: true,
		{SubPastDue, SubActive}: true, {SubPastDue, SubExpired}: true, {SubPastDue, SubCanceled}: true,
	}
	all := []SubscriptionStatus{SubIncomplete, SubActive, SubPastDue, SubCanceled, SubExpired}
	for _, from := range all {
		for _, to := range all {
			if got := from.CanTransition(to); got != allowed[[2]SubscriptionStatus{from, to}] {
				t.Errorf("%s → %s: got %v", from, to, got)
			}
		}
	}
	for _, s := range []SubscriptionStatus{SubCanceled, SubExpired} {
		if s.Open() {
			t.Errorf("%s must be terminal", s)
		}
	}
	for _, s := range []SubscriptionStatus{SubIncomplete, SubActive, SubPastDue} {
		if !s.Open() {
			t.Errorf("%s must be open", s)
		}
	}
}

func TestPaymentMethodExpiry(t *testing.T) {
	m := &PaymentMethod{ExpMonth: 12, ExpYear: 2026, Status: MethodActive}
	if m.ExpiredAt(date(2026, time.December, 31)) {
		t.Fatal("a card is valid through its expiry month")
	}
	if !m.ExpiredAt(date(2027, time.January, 1)) {
		t.Fatal("expired card accepted")
	}
	if !m.Usable(date(2026, time.June, 1)) {
		t.Fatal("active card not usable")
	}
	m.Status = MethodFailed
	if m.Usable(date(2026, time.June, 1)) {
		t.Fatal("failed card must not be charged")
	}
}

func TestPlanVersionPeriodMonthsAndLocalizedFallback(t *testing.T) {
	if (&PlanVersion{BillingInterval: IntervalMonth, IntervalCount: 3}).PeriodMonths() != 3 ||
		(&PlanVersion{BillingInterval: IntervalYear, IntervalCount: 1}).PeriodMonths() != 12 ||
		(&PlanVersion{BillingInterval: IntervalMonth}).PeriodMonths() != 1 {
		t.Fatal("period months")
	}
	txt := LocalizedText{"en": "Premium", "fr": "Abonnement"}
	if txt.For("fr") != "Abonnement" || txt.For("es") != "Premium" {
		t.Fatal("localized fallback")
	}
	list := LocalizedList{"en": {"a"}}
	if len(list.For("es")) != 1 || len((LocalizedList{}).For("fr")) != 0 {
		t.Fatal("localized list fallback")
	}
	var scanned LocalizedText
	if err := scanned.Scan([]byte(`{"en":"x"}`)); err != nil || scanned["en"] != "x" {
		t.Fatalf("scan: %v %v", scanned, err)
	}
	if err := scanned.Scan(nil); err != nil || len(scanned) != 0 {
		t.Fatalf("scan nil: %v", err)
	}
	if err := scanned.Scan(42); err == nil {
		t.Fatal("scan of an unsupported type must fail")
	}
	v, _ := LocalizedText(nil).Value()
	if v != "{}" {
		t.Fatalf("nil value: %v", v)
	}
}

func TestRefundable(t *testing.T) {
	tx := &PaymentTransaction{Kind: TxCreditPurchase, Status: TxSucceeded, AmountCents: 1000}
	if tx.Refundable() != 1000 {
		t.Fatal("full amount refundable")
	}
	tx.RefundedAmountCents, tx.Status = 400, TxPartiallyRefunded
	if tx.Refundable() != 600 {
		t.Fatal("partial refund")
	}
	for _, st := range []TransactionStatus{TxPending, TxFailed, TxRefunded, TxDisputed} {
		tx.Status = st
		if tx.Refundable() != 0 {
			t.Errorf("%s must not be refundable", st)
		}
	}
	if (&PaymentTransaction{Kind: TxRefund, Status: TxSucceeded, AmountCents: 10}).Refundable() != 0 {
		t.Fatal("a refund is not refundable")
	}
}
