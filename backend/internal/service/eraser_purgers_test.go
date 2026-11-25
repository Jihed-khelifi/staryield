package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type namedPurger struct {
	name  string
	calls *[]string
}

func (p namedPurger) PurgeForUser(context.Context, uuid.UUID) (int64, error) {
	*p.calls = append(*p.calls, p.name)
	return 1, nil
}

func TestAddPurgerKeepsEveryPurgerInOrder(t *testing.T) {
	var calls []string
	e := NewAccountEraser(EraserDeps{})
	e.AddPurger(nil) // ignored
	e.AddPurger(namedPurger{"referrals", &calls})
	e.AddPurger(namedPurger{"another feature", &calls})
	// Wiring a slot afterwards never drops the added purgers.
	e.SetCalculators(namedPurger{"calculators", &calls})
	e.SetCalculators(namedPurger{"calculators v2", &calls})
	if err := e.eraseExtras(context.Background(), uuid.New(), time.Now()); err != nil {
		t.Fatal(err)
	}
	want := []string{"calculators v2", "referrals", "another feature"}
	if len(calls) != len(want) {
		t.Fatalf("calls: %v", calls)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("calls: %v, want %v", calls, want)
		}
	}
}
