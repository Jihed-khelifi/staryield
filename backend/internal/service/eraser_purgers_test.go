package service

import (
	"context"
	"testing"

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
	e.SetCalculators(namedPurger{"calculators", &calls})
	e.AddPurger(namedPurger{"referrals", &calls})
	e.AddPurger(namedPurger{"another feature", &calls})
	n, err := e.d.Calculators.PurgeForUser(context.Background(), uuid.New())
	if err != nil || n != 3 {
		t.Fatalf("purged %d, err %v", n, err)
	}
	if len(calls) != 3 || calls[0] != "calculators" || calls[1] != "referrals" || calls[2] != "another feature" {
		t.Fatalf("calls: %v", calls)
	}

	// Without a slot already wired, the first purger takes it.
	calls = nil
	e = NewAccountEraser(EraserDeps{})
	e.AddPurger(namedPurger{"referrals", &calls})
	if _, err := e.d.Calculators.PurgeForUser(context.Background(), uuid.New()); err != nil || len(calls) != 1 {
		t.Fatalf("calls: %v %v", calls, err)
	}
}
