package seed

import (
	mrand "math/rand/v2"
	"testing"
	"time"
)

func TestLargeOptionsValidate(t *testing.T) {
	ok := DefaultLargeOptions()
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*LargeOptions){
		"no users":          func(o *LargeOptions) { o.Users = 0 },
		"no advisors":       func(o *LargeOptions) { o.Psychics = 0 },
		"too many pairs":    func(o *LargeOptions) { o.Users, o.Psychics, o.Conversations = 2, 2, 5 },
		"negative messages": func(o *LargeOptions) { o.MessagesPerConversation = -1 },
		"zero batch":        func(o *LargeOptions) { o.BatchSize = 0 },
		"batch too big":     func(o *LargeOptions) { o.BatchSize = 10000 },
		"negative ledger":   func(o *LargeOptions) { o.LedgerRows = -5 },
		"negative reviews":  func(o *LargeOptions) { o.Reviews = -1 },
		"negative chats":    func(o *LargeOptions) { o.Conversations = -1 },
	} {
		o := DefaultLargeOptions()
		mutate(&o)
		if o.Validate() == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestDatasetIDsAreTimeOrderedAndReproducible(t *testing.T) {
	newDS := func() *Dataset { return &Dataset{Rand: mrand.New(mrand.NewPCG(7, 9))} }
	a, b := newDS(), newDS()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	prev := a.ID(t0)
	if prev != b.ID(t0) {
		t.Fatal("same seed must give the same ID")
	}
	if prev.Version() != 7 || prev.Variant().String() != "RFC4122" {
		t.Fatalf("not a v7 UUID: %v %v", prev.Version(), prev.Variant())
	}
	for i := 1; i < 100; i++ {
		next := a.ID(t0.Add(time.Duration(i) * time.Millisecond))
		if next.String() <= prev.String() {
			t.Fatalf("IDs not ordered by time: %s then %s", prev, next)
		}
		prev = next
	}
}

func TestPickFollowsWeights(t *testing.T) {
	d := &Dataset{Rand: mrand.New(mrand.NewPCG(1, 2))}
	counts := make([]int, 3)
	for i := 0; i < 10000; i++ {
		counts[d.pick(70, 20, 10)]++
	}
	if counts[0] < 6500 || counts[0] > 7500 || counts[2] < 700 || counts[2] > 1300 {
		t.Fatalf("weights not respected: %v", counts)
	}
}
