package domain

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// revisions 5.0: refund requests (§24.4).

// The state machine: only open moves, to each of the four resolutions; every
// other pair (self-moves and anything out of a terminal state) is refused.
func TestRefundTransitionTable(t *testing.T) {
	allowed := map[[2]RefundRequestStatus]bool{
		{RefundOpen, RefundApproved}:  true,
		{RefundOpen, RefundDenied}:    true,
		{RefundOpen, RefundCredited}:  true,
		{RefundOpen, RefundWithdrawn}: true,
	}
	for _, from := range RefundStatuses {
		for _, to := range RefundStatuses {
			want := allowed[[2]RefundRequestStatus{from, to}]
			if got := from.CanTransition(to); got != want {
				t.Errorf("%s → %s: got %v, want %v", from, to, got, want)
			}
		}
		if got, want := from.Terminal(), from != RefundOpen; got != want {
			t.Errorf("%s.Terminal() = %v, want %v", from, got, want)
		}
	}
	if RefundRequestStatus("bogus").CanTransition(RefundDenied) || RefundRequestStatus("bogus").Valid() {
		t.Error("an unknown status must not move")
	}
}

func TestRefundReasonsAreValidated(t *testing.T) {
	for _, r := range RefundReasons {
		if !r.Valid() {
			t.Errorf("%s must be valid", r)
		}
	}
	for _, r := range []RefundReason{"", "UNUSED_CREDITS", "chargeback", "unused_credits "} {
		if r.Valid() {
			t.Errorf("%q must be refused", r)
		}
	}
}

func TestRefundReferenceFormat(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 2000; i++ {
		ref, err := NewRefundReference()
		if err != nil {
			t.Fatal(err)
		}
		if !RefundReferencePattern.MatchString(ref) {
			t.Fatalf("malformed reference %q", ref)
		}
		if strings.ContainsAny(ref[3:], "ILOU") {
			t.Fatalf("reference %q uses a letter Crockford base32 leaves out", ref)
		}
		seen[ref] = true
	}
	if len(seen) < 1990 { // 40 random bits: a collision in 2000 draws is already a miracle
		t.Fatalf("references repeat: %d distinct of 2000", len(seen))
	}
}

// Every one of the 32 symbols can come out, and masking is uniform: byte b maps to alphabet[b&31].
func TestRefundReferenceUsesTheWholeAlphabet(t *testing.T) {
	var all []byte
	for i := 0; i < 256; i++ {
		all = append(all, byte(i))
	}
	got := map[byte]int{}
	for off := 0; off+8 <= len(all); off += 8 {
		ref, err := refundReferenceFrom(bytes.NewReader(all[off : off+8]))
		if err != nil {
			t.Fatal(err)
		}
		for i := 3; i < len(ref); i++ {
			got[ref[i]]++
		}
	}
	if len(got) != 32 {
		t.Fatalf("only %d of 32 symbols reachable", len(got))
	}
	for sym, n := range got {
		if n != 8 { // 256 bytes ÷ 32 symbols
			t.Errorf("symbol %c drawn %d times, want 8: masking is biased", sym, n)
		}
	}
	if _, err := refundReferenceFrom(bytes.NewReader([]byte{1, 2, 3})); err == nil {
		t.Fatal("a short random source must fail, not yield a weak reference")
	}
}

func lot(total, remaining int64) *CreditLot {
	return &CreditLot{CreditsTotal: total, CreditsRemaining: remaining}
}

func TestRefundQuoteMath(t *testing.T) {
	bought := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name           string
		in             QuoteInput
		wantRefundable int64
		wantUsed       int64
		wantRemaining  int64
	}{
		{"unused pack refunds everything", QuoteInput{AmountPaid: 999, Credits: 100, Lot: lot(100, 100), Refundable: true}, 999, 0, 100},
		{"fully used pack refunds nothing", QuoteInput{AmountPaid: 999, Credits: 100, Lot: lot(100, 0), Refundable: true}, 0, 100, 0},
		{"partly used: the unused share, rounded down", QuoteInput{AmountPaid: 999, Credits: 100, Lot: lot(100, 37), Refundable: true}, 369, 63, 37},
		{"partly refunded already", QuoteInput{AmountPaid: 1000, Refunded: 400, Credits: 100, Lot: lot(100, 100), Refundable: true}, 600, 0, 100},
		{"never more than what is left of the payment", QuoteInput{AmountPaid: 1000, Refunded: 900, Credits: 100, Lot: lot(100, 100), Refundable: true}, 100, 0, 100},
		{"held credits already left the lot (counted as used)", QuoteInput{AmountPaid: 1000, Credits: 100, Lot: lot(100, 60), Refundable: true}, 600, 40, 60},
		{"rounding: 1 credit of 3 for $1.00", QuoteInput{AmountPaid: 100, Credits: 3, Lot: lot(3, 1), Refundable: true}, 33, 2, 1},
		{"no lot (a spent legacy purchase): nothing left", QuoteInput{AmountPaid: 500, Credits: 50, Refundable: true}, 0, 50, 0},
		{"a lot that claims more than it was: clamped", QuoteInput{AmountPaid: 500, Credits: 50, Lot: lot(50, 80), Refundable: true}, 500, 0, 50},
		{"not in a refundable state: nothing", QuoteInput{AmountPaid: 500, Credits: 50, Lot: lot(50, 50), Refundable: false}, 0, 0, 50},
		{"a huge pack does not overflow", QuoteInput{AmountPaid: 10_000_000, Credits: 1_000_000, Lot: lot(1_000_000, 999_999), Refundable: true}, 9_999_990, 1, 999_999},
	}
	for _, c := range cases {
		in := c.in
		in.PurchasedAt, in.WindowDays, in.Now = bought, 14, bought.AddDate(0, 0, 1)
		q := NewRefundQuote(in)
		if q.RefundableCents != c.wantRefundable || q.CreditsUsed != c.wantUsed || q.CreditsRemaining != c.wantRemaining {
			t.Errorf("%s: refundable %d (want %d), used %d (want %d), remaining %d (want %d)", c.name,
				q.RefundableCents, c.wantRefundable, q.CreditsUsed, c.wantUsed, q.CreditsRemaining, c.wantRemaining)
		}
		if q.CreditsPurchased != q.CreditsUsed+q.CreditsRemaining {
			t.Errorf("%s: purchased %d != used %d + remaining %d", c.name, q.CreditsPurchased, q.CreditsUsed, q.CreditsRemaining)
		}
	}
}

func TestRefundQuoteWindow(t *testing.T) {
	bought := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	end := bought.AddDate(0, 0, 14)
	in := QuoteInput{TransactionID: uuid.New(), AmountPaid: 100, Credits: 10, Lot: lot(10, 10), PurchasedAt: bought, WindowDays: 14, Refundable: true}
	for _, c := range []struct {
		name string
		now  time.Time
		want bool
	}{
		{"on the day", bought, true},
		{"the last instant", end, true},
		{"one microsecond later", end.Add(time.Microsecond), false},
		{"long after", end.AddDate(1, 0, 0), false},
	} {
		in.Now = c.now
		q := NewRefundQuote(in)
		if q.WithinWindow != c.want || !q.WindowEndsAt.Equal(end) {
			t.Errorf("%s: within=%v ends=%s", c.name, q.WithinWindow, q.WindowEndsAt)
		}
	}
	in.WindowDays = 30
	in.Now = end.AddDate(0, 0, 5)
	if q := NewRefundQuote(in); !q.WithinWindow || !q.WindowEndsAt.Equal(bought.AddDate(0, 0, 30)) {
		t.Errorf("a 30 day window: %+v", q)
	}
}
