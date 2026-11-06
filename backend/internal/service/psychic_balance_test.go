package service

import "testing"

func TestBalanceViewConvertsCreditsToMoney(t *testing.T) {
	v := balanceView(&PayoutView{AvailableCredits: 1250, ReservedCredits: 300, PaidCredits: 100, LifetimeNetCredits: 1650, CentsPerCredit: 8, Currency: "USD"})
	if v.AvailableCents != 10000 || v.Currency != "USD" || v.AvailableCredits != 1250 || v.ReservedCredits != 300 || v.LifetimeNetCredits != 1650 {
		t.Fatalf("balance view: %+v", v)
	}
	if z := balanceView(&PayoutView{CentsPerCredit: 8, Currency: "EUR"}); z.AvailableCents != 0 || z.Currency != "EUR" {
		t.Fatalf("empty balance: %+v", z)
	}
}
