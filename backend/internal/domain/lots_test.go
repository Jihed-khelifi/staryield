package domain

import (
	"math"
	"math/big"
	"math/rand"
	"testing"

	"github.com/google/uuid"
)

func TestRefundableCents(t *testing.T) {
	cases := []struct {
		name                                     string
		total, remaining, paid, refunded, expect int64
	}{
		{"unused pack refunds everything", 600, 600, 1000, 0, 1000},
		{"half used", 600, 300, 1000, 0, 500},
		{"floor of the unused share", 600, 1, 1000, 0, 1}, // 1.666 -> 1
		{"floor never rounds up", 700, 699, 999, 0, 997},  // 997.57 -> 997
		{"fully used", 600, 0, 1000, 0, 0},                // nothing left
		{"capped by what is left of the payment", 600, 600, 1000, 400, 600},
		{"already refunded fully", 600, 600, 1000, 1000, 0},
		{"refunded more than paid (corrupt) is zero", 600, 600, 1000, 1500, 0},
		{"remaining above total is clamped", 600, 900, 1000, 0, 1000},
		{"negative remaining is zero", 600, -5, 1000, 0, 0},
		{"negative refunded is ignored", 600, 600, 1000, -50, 1000},
		{"zero total", 0, 0, 1000, 0, 0},
		{"zero paid (bonus lot)", 600, 600, 0, 0, 0},
		{"partial refund already made, unused share is the cap", 600, 300, 1000, 400, 500}, // min(600 left, 500 unused)
		{"partial refund already made, payment left is the cap", 600, 300, 1000, 700, 300}, // min(300 left, 500 unused)
		{"largest prices do not overflow", 1_000_000, 999_999, 10_000_000, 0, 9_999_990},
	}
	for _, c := range cases {
		if got := RefundableCents(c.total, c.remaining, c.paid, c.refunded); got != c.expect {
			t.Errorf("%s: RefundableCents(%d,%d,%d,%d) = %d, want %d", c.name, c.total, c.remaining, c.paid, c.refunded, got, c.expect)
		}
	}
}

func TestClawbackCredits(t *testing.T) {
	cases := []struct {
		name                                   string
		total, remaining, paid, refund, expect int64
	}{
		{"full refund of an unused pack", 600, 600, 1000, 1000, 600},
		{"half refund", 600, 600, 1000, 500, 300},
		{"ceil keeps the customer from keeping paid-back credits", 600, 600, 1000, 1, 1}, // 0.6 -> 1
		{"ceil on a fraction", 700, 700, 999, 500, 351},                                  // 350.35 -> 351
		{"refund above the unused value takes only what is left", 600, 100, 1000, 1000, 100},
		{"nothing left, nothing taken", 600, 0, 1000, 1000, 0},
		{"zero refund", 600, 600, 1000, 0, 0},
		{"negative refund", 600, 600, 1000, -10, 0},
		{"zero paid", 600, 600, 0, 100, 0},
		{"zero total", 0, 0, 1000, 100, 0},
		{"refund larger than paid is capped by remaining", 600, 600, 1000, 5000, 600},
		{"largest values do not overflow", 1_000_000, 1_000_000, 10_000_000, 9_999_999, 1_000_000},
		{"huge refund saturates and is capped", 1_000_000, 400_000, 1, math.MaxInt64, 400_000},
	}
	for _, c := range cases {
		if got := ClawbackCredits(c.total, c.remaining, c.paid, c.refund); got != c.expect {
			t.Errorf("%s: ClawbackCredits(%d,%d,%d,%d) = %d, want %d", c.name, c.total, c.remaining, c.paid, c.refund, got, c.expect)
		}
	}
}

// bigFloorDiv is the reference implementation of mulDiv.
func bigMulDiv(a, b, c int64, up bool) *big.Int {
	n := new(big.Int).Mul(big.NewInt(a), big.NewInt(b))
	q, r := new(big.Int).QuoRem(n, big.NewInt(c), new(big.Int))
	if up && r.Sign() != 0 {
		q.Add(q, big.NewInt(1))
	}
	return q
}

func TestMulDivMatchesBigInt(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 20000; i++ {
		a, b, c := rng.Int63n(1<<40), rng.Int63n(1<<40), rng.Int63n(1<<40)+1
		for _, up := range []bool{false, true} {
			want := bigMulDiv(a, b, c, up)
			got := mulDiv(a, b, c, up)
			if want.IsInt64() {
				if got != want.Int64() {
					t.Fatalf("mulDiv(%d,%d,%d,%v) = %d, want %s", a, b, c, up, got, want)
				}
			} else if got != math.MaxInt64 {
				t.Fatalf("mulDiv(%d,%d,%d,%v) = %d, want saturation", a, b, c, up, got)
			}
		}
	}
	// The case the 64-bit arithmetic gets wrong: credits × value ≈ 1e22.
	if got := mulDiv(1_000_000, 10_000_000_000_000_000, 1_000_000, false); got != 10_000_000_000_000_000 {
		t.Fatalf("1e22 intermediate: %d", got)
	}
}

func ids(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = NewID()
	}
	return out
}

func TestPlanConsumption(t *testing.T) {
	id := ids(3)
	cases := []struct {
		name      string
		lots      []LotStock
		credits   int64
		want      []LotDelta
		uncovered int64
	}{
		{"one lot, partial, value floored", []LotStock{{id[0], 600, 1_000_000}}, 30, []LotDelta{{id[0], 30, 50_000}}, 0},
		{"value floors", []LotStock{{id[0], 700, 999_000}}, 1, []LotDelta{{id[0], 1, 1_427}}, 0}, // 1427.14
		{"emptying a lot takes all its value", []LotStock{{id[0], 700, 999_000}}, 700, []LotDelta{{id[0], 700, 999_000}}, 0},
		{"spans lots in the order given", []LotStock{{id[0], 10, 100}, {id[1], 5, 0}, {id[2], 50, 0}}, 30,
			[]LotDelta{{id[0], 10, 100}, {id[1], 5, 0}, {id[2], 15, 0}}, 0},
		{"stops when covered", []LotStock{{id[0], 10, 100}, {id[1], 5, 0}}, 10, []LotDelta{{id[0], 10, 100}}, 0},
		{"uncovered remainder is reported", []LotStock{{id[0], 10, 100}}, 25, []LotDelta{{id[0], 10, 100}}, 15},
		{"no lots at all", nil, 7, nil, 7},
		{"empty lots are skipped", []LotStock{{id[0], 0, 0}, {id[1], 4, 40}}, 4, []LotDelta{{id[1], 4, 40}}, 0},
		{"zero credits", []LotStock{{id[0], 10, 100}}, 0, nil, 0},
	}
	for _, c := range cases {
		got, unc := PlanConsumption(c.lots, c.credits)
		if len(got) != len(c.want) || unc != c.uncovered {
			t.Errorf("%s: got %v uncovered %d, want %v uncovered %d", c.name, got, unc, c.want, c.uncovered)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: delta %d = %+v, want %+v", c.name, i, got[i], c.want[i])
			}
		}
	}
}

// A lot consumed in many random-sized steps hands out exactly what was paid:
// the allocations add up to the lot's value and the last one empties it.
func TestPlanConsumptionConservesValue(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	for trial := 0; trial < 2000; trial++ {
		total := rng.Int63n(1_000_000) + 1
		paid := (rng.Int63n(10_000_000) + 1) * 1000
		lot := LotStock{ID: NewID(), Remaining: total, Value: paid}
		var sumValue, sumCredits int64
		for lot.Remaining > 0 {
			step := rng.Int63n(min(lot.Remaining, 5000)) + 1
			d, unc := PlanConsumption([]LotStock{lot}, step)
			if unc != 0 || len(d) != 1 {
				t.Fatalf("unexpected plan %v %d", d, unc)
			}
			lot.Remaining -= d[0].Credits
			lot.Value -= d[0].ValueMillicents
			if lot.Value < 0 {
				t.Fatalf("value went negative")
			}
			sumValue += d[0].ValueMillicents
			sumCredits += d[0].Credits
		}
		if sumCredits != total || sumValue != paid || lot.Value != 0 {
			t.Fatalf("trial %d: credits %d/%d value %d/%d left %d", trial, sumCredits, total, sumValue, paid, lot.Value)
		}
	}
}

func TestPlanRestore(t *testing.T) {
	id := ids(3)
	src := []LotRestoreSource{
		{LotID: id[0], Credits: 10, Value: 100, Room: 10, ValueRoom: 100},
		{LotID: id[1], Credits: 5, Value: 0, Room: 5, ValueRoom: 0},
	}
	cases := []struct {
		name     string
		src      []LotRestoreSource
		amount   int64
		want     []LotDelta
		leftover int64
	}{
		{"whole movement", src, 15, []LotDelta{{id[0], -10, -100}, {id[1], -5, 0}}, 0},
		{"partial restore floors the value", src, 3, []LotDelta{{id[0], -3, -30}}, 0},
		{"partial spanning lots", src, 12, []LotDelta{{id[0], -10, -100}, {id[1], -2, 0}}, 0},
		{"more than was consumed leaves a leftover", src, 20, []LotDelta{{id[0], -10, -100}, {id[1], -5, 0}}, 5},
		{"nothing consumed (legacy)", nil, 8, nil, 8},
		{"a lot with no room is skipped", []LotRestoreSource{{LotID: id[0], Credits: 10, Value: 100, Room: 4, ValueRoom: 40}}, 10,
			[]LotDelta{{id[0], -4, -40}}, 6},
		{"value is capped by the room", []LotRestoreSource{{LotID: id[0], Credits: 10, Value: 100, Room: 10, ValueRoom: 60}}, 10,
			[]LotDelta{{id[0], -10, -60}}, 0},
		{"zero amount", src, 0, nil, 0},
	}
	for _, c := range cases {
		got, left := PlanRestore(c.src, c.amount)
		if len(got) != len(c.want) || left != c.leftover {
			t.Errorf("%s: got %v leftover %d, want %v leftover %d", c.name, got, left, c.want, c.leftover)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: delta %d = %+v, want %+v", c.name, i, got[i], c.want[i])
			}
		}
	}
}

func TestLotSourcePriority(t *testing.T) {
	if LotPurchase.Priority() != 0 || LotMembership.Priority() != 1 || LotBonus.Priority() != 2 {
		t.Fatal("consumption order is purchase, membership, bonus")
	}
	if LotSource("nope").Valid() || !LotBonus.Valid() {
		t.Fatal("Valid")
	}
}

// The refund quote and the clawback agree: refunding what the quote offers
// takes back at most the credits that are left, and refunding the whole
// payment of an unused lot takes back every credit.
func TestRefundQuoteAndClawbackAgree(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	for i := 0; i < 5000; i++ {
		total := rng.Int63n(1_000_000) + 1
		paid := rng.Int63n(10_000_000) + 1
		remaining := rng.Int63n(total + 1)
		quote := RefundableCents(total, remaining, paid, 0)
		if quote > paid {
			t.Fatalf("quote %d above paid %d", quote, paid)
		}
		claw := ClawbackCredits(total, remaining, paid, quote)
		if claw > remaining {
			t.Fatalf("clawback %d above remaining %d", claw, remaining)
		}
	}
	if got := ClawbackCredits(600, 600, 1000, RefundableCents(600, 600, 1000, 0)); got != 600 {
		t.Fatalf("full refund of an unused pack took %d credits", got)
	}
}
