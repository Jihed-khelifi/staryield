package service

import (
	"context"
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 5.0: refund requests (§24.4).

func creditTx(status domain.TransactionStatus, paid, refunded int64) *domain.PaymentTransaction {
	pid := uuid.New()
	return &domain.PaymentTransaction{ID: uuid.New(), Kind: domain.TxCreditPurchase, Status: status, AmountCents: paid,
		RefundedAmountCents: refunded, CreditPurchaseID: &pid}
}

func TestRefundEligibility(t *testing.T) {
	inWindow, outside := domain.RefundQuote{WithinWindow: true}, domain.RefundQuote{WithinWindow: false}
	sub := creditTx(domain.TxSucceeded, 100, 0)
	sub.Kind = domain.TxSubscriptionRenewal
	noPurchase := creditTx(domain.TxSucceeded, 100, 0)
	noPurchase.CreditPurchaseID = nil
	cases := []struct {
		name       string
		tx         *domain.PaymentTransaction
		reason     domain.RefundReason
		quote      domain.RefundQuote
		withWindow bool
		want       string // refund_not_eligible reason, "" for eligible
	}{
		{"paid, in the window", creditTx(domain.TxSucceeded, 100, 0), domain.RefundUnusedCredits, inWindow, true, ""},
		{"partially refunded still eligible", creditTx(domain.TxPartiallyRefunded, 100, 40), domain.RefundUnusedCredits, inWindow, true, ""},
		{"membership payment", sub, domain.RefundOther, inWindow, true, apperror.RefundReasonNotCreditPurchase},
		{"credit purchase kind without a purchase", noPurchase, domain.RefundOther, inWindow, true, apperror.RefundReasonNotCreditPurchase},
		{"pending", creditTx(domain.TxPending, 100, 0), domain.RefundOther, inWindow, true, apperror.RefundReasonNotPaid},
		{"failed", creditTx(domain.TxFailed, 100, 0), domain.RefundOther, inWindow, true, apperror.RefundReasonNotPaid},
		{"disputed", creditTx(domain.TxDisputed, 100, 0), domain.RefundOther, inWindow, true, apperror.RefundReasonNotPaid},
		{"refunded", creditTx(domain.TxRefunded, 100, 100), domain.RefundOther, inWindow, true, apperror.RefundReasonFullyRefunded},
		{"partially refunded up to the full amount", creditTx(domain.TxPartiallyRefunded, 100, 100), domain.RefundOther, inWindow, true, apperror.RefundReasonFullyRefunded},
		{"unused credits outside the window", creditTx(domain.TxSucceeded, 100, 0), domain.RefundUnusedCredits, outside, true, apperror.RefundReasonOutsideWindow},
		{"technical problem any time", creditTx(domain.TxSucceeded, 100, 0), domain.RefundTechnicalProblem, outside, true, ""},
		{"incorrect charge any time", creditTx(domain.TxSucceeded, 100, 0), domain.RefundIncorrectCharge, outside, true, ""},
		{"unauthorized charge any time", creditTx(domain.TxSucceeded, 100, 0), domain.RefundUnauthorizedCharge, outside, true, ""},
		{"reading concern any time", creditTx(domain.TxSucceeded, 100, 0), domain.RefundReadingConcern, outside, true, ""},
		{"other any time", creditTx(domain.TxSucceeded, 100, 0), domain.RefundOther, outside, true, ""},
		{"an administrator skips the window", creditTx(domain.TxSucceeded, 100, 0), domain.RefundUnusedCredits, outside, false, ""},
		{"an administrator still cannot refund a refunded payment", creditTx(domain.TxRefunded, 100, 100), domain.RefundUnusedCredits, outside, false, apperror.RefundReasonFullyRefunded},
	}
	for _, c := range cases {
		err := refundEligibility(c.tx, c.reason, c.quote, c.withWindow)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: refused: %v", c.name, err)
		case c.want != "" && err == nil:
			t.Errorf("%s: accepted, want %s", c.name, c.want)
		case c.want != "":
			if err.Code != apperror.CodeRefundNotEligible || err.Details["reason"] != c.want {
				t.Errorf("%s: got %s %v, want %s", c.name, err.Code, err.Details, c.want)
			}
		}
	}
}

func TestRefundPayable(t *testing.T) {
	for _, c := range []struct {
		tx   *domain.PaymentTransaction
		want bool
	}{
		{creditTx(domain.TxSucceeded, 100, 0), true},
		{creditTx(domain.TxPartiallyRefunded, 100, 50), true},
		{creditTx(domain.TxPartiallyRefunded, 100, 100), false},
		{creditTx(domain.TxRefunded, 100, 100), false},
		{creditTx(domain.TxPending, 100, 0), false},
		{creditTx(domain.TxDisputed, 100, 0), false},
	} {
		if got := refundPayable(c.tx); got != c.want {
			t.Errorf("%s %d/%d: %v", c.tx.Status, c.tx.RefundedAmountCents, c.tx.AmountCents, got)
		}
	}
}

// ---------------------------------------------------------------- the state machine

// fakeRefunds is an in-memory RefundRepository with failure injection.
type fakeRefunds struct {
	rows        map[uuid.UUID]domain.RefundRequest
	failUpdates int // the next n Update calls fail
	updates     int
}

func (f *fakeRefunds) Create(context.Context, *domain.RefundRequest) error { panic("unused") }
func (f *fakeRefunds) Get(_ context.Context, id uuid.UUID) (*domain.RefundRequest, error) {
	return f.GetForUpdate(nil, id)
}
func (f *fakeRefunds) GetForUpdate(_ context.Context, id uuid.UUID) (*domain.RefundRequest, error) {
	r, ok := f.rows[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &r, nil
}
func (f *fakeRefunds) Update(_ context.Context, id uuid.UUID, fields map[string]any) error {
	f.updates++
	if f.failUpdates > 0 {
		f.failUpdates--
		return errors.New("connection lost")
	}
	r := f.rows[id]
	r.Status = fields["status"].(domain.RefundRequestStatus)
	r.ResolutionNote = fields["resolution_note"].(string)
	r.RefundedCents = fields["refunded_cents"].(*int64)
	f.rows[id] = r
	return nil
}
func (f *fakeRefunds) ByKey(context.Context, uuid.UUID, string) (*domain.RefundRequest, error) {
	panic("unused")
}
func (f *fakeRefunds) OpenFor(context.Context, uuid.UUID) (*domain.RefundRequest, error) {
	panic("unused")
}
func (f *fakeRefunds) Latest(context.Context, []uuid.UUID) (map[uuid.UUID]repository.RefundSummary, error) {
	panic("unused")
}
func (f *fakeRefunds) ListForUser(context.Context, uuid.UUID, *repository.Cursor, int) ([]domain.RefundRequest, error) {
	panic("unused")
}
func (f *fakeRefunds) List(context.Context, repository.RefundFilter) ([]domain.RefundRequest, error) {
	panic("unused")
}
func (f *fakeRefunds) Packs(context.Context, []uuid.UUID) (map[uuid.UUID]repository.PackRef, error) {
	panic("unused")
}

// rollbackTx commits on success and restores the rows on error, like a database.
type rollbackTx struct{ f *fakeRefunds }

func (t rollbackTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	snap := maps.Clone(t.f.rows)
	if err := fn(ctx); err != nil {
		t.f.rows = snap
		return err
	}
	return nil
}

func refundMachine(status domain.RefundRequestStatus) (*RefundService, *fakeRefunds, uuid.UUID) {
	f := &fakeRefunds{rows: map[uuid.UUID]domain.RefundRequest{}}
	id := uuid.New()
	f.rows[id] = domain.RefundRequest{ID: id, UserID: uuid.New(), Status: status}
	s := &RefundService{d: RefundDeps{Tx: rollbackTx{f}, Repo: f, Clock: clock.NewFake(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))}}
	return s, f, id
}

func approveMove(applied *int, amount int64) refundMove {
	return refundMove{to: domain.RefundApproved, apply: func(context.Context, *domain.RefundRequest) (refundOutcome, error) {
		*applied++
		tx := uuid.New()
		return refundOutcome{note: "ok", refundedCents: &amount, refundTxID: &tx, resolvedBy: uuid.New(), irreversible: true}, nil
	}}
}

// Every transition goes through the one function: allowed from open, refused (and never run) from any
// terminal state.
func TestRefundTransitionOnlyMovesOpenRequests(t *testing.T) {
	for _, from := range domain.RefundStatuses {
		for _, to := range domain.RefundStatuses {
			s, f, id := refundMachine(from)
			ran := 0
			_, _, err := s.transition(context.Background(), id, refundMove{to: to, apply: func(context.Context, *domain.RefundRequest) (refundOutcome, error) {
				ran++
				return refundOutcome{note: "n", resolvedBy: uuid.New()}, nil
			}})
			if from.CanTransition(to) {
				if err != nil || ran != 1 || f.rows[id].Status != to {
					t.Errorf("%s → %s: err %v, ran %d, now %s", from, to, err, ran, f.rows[id].Status)
				}
				continue
			}
			var ae *apperror.Error
			if !errors.As(err, &ae) || ae.Code != apperror.CodeInvalidState || ran != 0 || f.rows[id].Status != from {
				t.Errorf("%s → %s: err %v, ran %d, now %s: a forbidden move must be refused before its side effect", from, to, err, ran, f.rows[id].Status)
			}
		}
	}
}

func TestRefundTransitionGuardRunsBeforeAnything(t *testing.T) {
	s, f, id := refundMachine(domain.RefundOpen)
	ran := 0
	_, _, err := s.transition(context.Background(), id, refundMove{to: domain.RefundWithdrawn,
		guard: func(*domain.RefundRequest) error { return repository.ErrNotFound },
		apply: func(context.Context, *domain.RefundRequest) (refundOutcome, error) {
			ran++
			return refundOutcome{}, nil
		}})
	var ae *apperror.Error
	if !errors.As(err, &ae) || ae.Code != apperror.CodeNotFound || ran != 0 || f.rows[id].Status != domain.RefundOpen {
		t.Fatalf("guard: err %v, ran %d, status %s", err, ran, f.rows[id].Status)
	}
	// Even a resolved request is "not found" to someone who does not own it.
	s, _, id = refundMachine(domain.RefundDenied)
	_, _, err = s.transition(context.Background(), id, refundMove{to: domain.RefundWithdrawn,
		guard: func(*domain.RefundRequest) error { return repository.ErrNotFound },
		apply: func(context.Context, *domain.RefundRequest) (refundOutcome, error) { return refundOutcome{}, nil }})
	if !errors.As(err, &ae) || ae.Code != apperror.CodeNotFound {
		t.Fatalf("a stranger must not learn the state of a request: %v", err)
	}
}

func TestRefundTransitionFailedSideEffectLeavesTheRequestOpen(t *testing.T) {
	s, f, id := refundMachine(domain.RefundOpen)
	boom := apperror.RefundRefused()
	_, _, err := s.transition(context.Background(), id, refundMove{to: domain.RefundApproved,
		apply: func(context.Context, *domain.RefundRequest) (refundOutcome, error) { return refundOutcome{}, boom }})
	var ae *apperror.Error
	if !errors.As(err, &ae) || ae.Code != apperror.CodeRefundRefused || f.rows[id].Status != domain.RefundOpen || f.updates != 0 {
		t.Fatalf("err %v, status %s, updates %d", err, f.rows[id].Status, f.updates)
	}
}

// The provider refunded and the ledger booked it, but the request row could not be written: the
// write is retried in a fresh transaction and the refund is never repeated.
func TestRefundTransitionRetriesTheWriteWithoutRefundingTwice(t *testing.T) {
	s, f, id := refundMachine(domain.RefundOpen)
	f.failUpdates = 1
	applied := 0
	r, out, err := s.transition(context.Background(), id, approveMove(&applied, 1500))
	if err != nil || applied != 1 || r.Status != domain.RefundApproved || f.rows[id].Status != domain.RefundApproved || *out.refundedCents != 1500 {
		t.Fatalf("err %v, applied %d, status %s", err, applied, f.rows[id].Status)
	}
	if f.updates != 2 {
		t.Fatalf("expected one failed and one successful write, got %d", f.updates)
	}

	// Both writes fail: the error surfaces (the money moved; a human reconciles), still one refund.
	s, f, id = refundMachine(domain.RefundOpen)
	f.failUpdates = 2
	applied = 0
	if _, _, err := s.transition(context.Background(), id, approveMove(&applied, 1500)); err == nil || applied != 1 {
		t.Fatalf("err %v, applied %d", err, applied)
	}
	if f.rows[id].Status != domain.RefundOpen {
		t.Fatalf("status %s", f.rows[id].Status)
	}
}

// A later retry of a request that was resolved meanwhile is refused, not repeated.
func TestRefundTransitionSecondApprovalIsRefused(t *testing.T) {
	s, f, id := refundMachine(domain.RefundOpen)
	applied := 0
	if _, _, err := s.transition(context.Background(), id, approveMove(&applied, 100)); err != nil {
		t.Fatal(err)
	}
	_, _, err := s.transition(context.Background(), id, approveMove(&applied, 100))
	var ae *apperror.Error
	if !errors.As(err, &ae) || ae.Code != apperror.CodeInvalidState || applied != 1 || f.rows[id].Status != domain.RefundApproved {
		t.Fatalf("err %v, applied %d", err, applied)
	}
}

func TestRefundNoteValidation(t *testing.T) {
	for _, c := range []struct {
		in string
		ok bool
	}{{"", false}, {"abcd", false}, {"  abc  ", false}, {"abcde", true}, {string(make([]rune, 1001)), false}} {
		_, err := cleanRefundNote(c.in)
		if (err == nil) != c.ok {
			t.Errorf("%q: %v", c.in, err)
		}
	}
	long := make([]rune, 1000)
	for i := range long {
		long[i] = 'é'
	}
	if _, err := cleanRefundNote(string(long)); err != nil {
		t.Errorf("1000 characters (multi-byte) must be accepted: %v", err)
	}
}

func TestRevenueRangeAndBuckets(t *testing.T) {
	now := time.Date(2026, 11, 12, 9, 30, 0, 0, time.UTC)
	if _, _, err := ParseRevenueRange("2025-11-11", "2026-11-12", "", now); err == nil {
		t.Error("367 days (the end date is inclusive) must be refused")
	}
	if _, _, err := ParseRevenueRange("2025-11-12", "2026-11-12", "", now); err != nil {
		t.Errorf("366 days must be accepted: %v", err)
	}
	if r, in, err := ParseRevenueRange("2025-11-13", "2026-11-12", "week", now); err != nil || in != "week" || r.To.Sub(r.From) != 365*24*time.Hour {
		t.Errorf("365 days: %v %s %v", r, in, err)
	}
	if _, _, err := ParseRevenueRange("2026-11-01", "2026-11-10", "hour", now); err == nil {
		t.Error("hourly buckets do not exist")
	}
	if _, in, err := ParseRevenueRange("", "", "", now); err != nil || in != "day" {
		t.Errorf("defaults: %s %v", in, err)
	}
	// Buckets start where date_trunc starts them: weeks on Monday, months on the 1st.
	wed := time.Date(2026, 11, 11, 15, 0, 0, 0, time.UTC)
	if got := revenueBucketStart(wed, "week"); !got.Equal(time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("week: %s", got)
	}
	sun := time.Date(2026, 11, 15, 23, 0, 0, 0, time.UTC)
	if got := revenueBucketStart(sun, "week"); !got.Equal(time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("sunday belongs to the week that started on Monday: %s", got)
	}
	if got := revenueBucketStart(wed, "month"); !got.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("month: %s", got)
	}
	if bps(1, 3) != 3333 || bps(0, 5) != 0 || bps(5, 0) != 0 || bps(5, 5) != 10000 {
		t.Error("bps")
	}
}
