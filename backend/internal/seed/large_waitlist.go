package seed

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// Load-test data for advisor waiting lists and card payments, so the query
// plans of those pages are checked at realistic sizes.
func init() {
	RegisterLarge(Generator{Name: "waiting lists", Run: func(_ context.Context, d *Dataset) error { return d.waitingLists() }})
	RegisterLarge(Generator{Name: "payment transactions", Run: func(_ context.Context, d *Dataset) error { return d.paymentTransactions() }})
}

// waitingLists puts ~8% of clients on 1–5 advisors' lists. About half of
// the entries are finished (notified or left); a few open ones are stamped
// as due, as if their advisor just came online.
func (d *Dataset) waitingLists() error {
	if len(d.Psychics) == 0 {
		return nil
	}
	batch := make([]domain.WaitlistEntry, 0, d.Opts.BatchSize)
	flush := func() error {
		err := d.insert("advisor_waitlist", &batch, len(batch), true)
		batch = batch[:0]
		return err
	}
	for _, client := range d.Clients {
		if d.Rand.Float64() >= 0.08 {
			continue
		}
		seen := map[int]bool{}
		for k, n := 0, 1+d.Rand.IntN(5); k < n; k++ {
			pi := d.Rand.IntN(len(d.Psychics))
			if seen[pi] {
				continue
			}
			seen[pi] = true
			at := d.Now.Add(-time.Duration(d.Rand.IntN(6*24*60)) * time.Minute)
			e := domain.WaitlistEntry{ID: d.ID(at), UserID: client, PsychicID: d.Psychics[pi], CreatedAt: at, UpdatedAt: at}
			switch r := d.Rand.Float64(); {
			case r < 0.35:
				done := at.Add(time.Duration(1+d.Rand.IntN(48)) * time.Hour)
				e.AvailableAt, e.NotifiedAt, e.UpdatedAt = &done, &done, done
			case r < 0.5:
				left := at.Add(time.Duration(1+d.Rand.IntN(48)) * time.Hour)
				e.DeletedAt.Time, e.DeletedAt.Valid, e.UpdatedAt = left, true, left
			case r < 0.53:
				due := d.Now.Add(-time.Duration(d.Rand.IntN(120)) * time.Second)
				e.AvailableAt = &due
			}
			batch = append(batch, e)
			if len(batch) == cap(batch) {
				if err := flush(); err != nil {
					return err
				}
			}
		}
	}
	if err := flush(); err != nil {
		return err
	}
	d.Log.Info("waiting lists generated", zap.Int("rows", d.Counts["advisor_waitlist"]))
	return nil
}

// paymentTransactions gives ~30% of clients 1–12 card payments for credit
// packs over the history window (~8% declined, a few refunded).
func (d *Dataset) paymentTransactions() error {
	batch := make([]domain.PaymentTransaction, 0, d.Opts.BatchSize)
	flush := func() error {
		err := d.insert("payment_transactions", &batch, len(batch), true)
		batch = batch[:0]
		return err
	}
	packs := []struct{ cents, credits int64 }{{499, 50}, {999, 110}, {1999, 250}, {4999, 700}}
	for ci, client := range d.Clients {
		if d.Rand.Float64() >= 0.3 {
			continue
		}
		for k, n := 0, 1+d.Rand.IntN(12); k < n; k++ {
			at := d.Now.Add(-time.Duration(d.Rand.Int64N(int64(historyWindow))))
			p := packs[d.Rand.IntN(len(packs))]
			tx := domain.PaymentTransaction{ID: d.ID(at), UserID: client, Kind: domain.TxCreditPurchase,
				Status: domain.TxSucceeded, AmountCents: p.cents, Currency: "USD", Credits: p.credits,
				Provider: "sandbox_a", Description: fmt.Sprintf("%d credits", p.credits),
				IdempotencyKey: fmt.Sprintf("load:%s:pay:%d:%d", d.Tag, ci, k), AttemptNo: 1, CreatedAt: at, UpdatedAt: at}
			switch r := d.Rand.Float64(); {
			case r < 0.08:
				tx.Status, tx.FailureCode, tx.Credits = domain.TxFailed, "card_declined", 0
			case r < 0.1:
				tx.Status, tx.RefundedAmountCents = domain.TxRefunded, p.cents
			}
			batch = append(batch, tx)
			if len(batch) == cap(batch) {
				if err := flush(); err != nil {
					return err
				}
			}
		}
	}
	if err := flush(); err != nil {
		return err
	}
	d.Log.Info("payment transactions generated", zap.Int("rows", d.Counts["payment_transactions"]))
	return nil
}
