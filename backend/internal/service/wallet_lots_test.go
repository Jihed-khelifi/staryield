package service

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// memLedger is an in-memory wallet + lot store with the semantics of the SQL
// repositories (constraints included), so the tests exercise WalletService
// itself. It is one "database": a failed movement is undone by restoring a
// snapshot, like a rolled-back transaction.
type memLedger struct {
	wallets map[uuid.UUID]*domain.Wallet
	ledger  []domain.LedgerEntry
	lots    []domain.CreditLot
	allocs  []domain.CreditLotAllocation

	lockReads      int // SELECT … FOR UPDATE of a wallet
	spendableReads int // SELECTs of candidate lots
	applyCalls     int // lot update + allocation statements
}

func newMemLedger() *memLedger { return &memLedger{wallets: map[uuid.UUID]*domain.Wallet{}} }

func (m *memLedger) snapshot() *memLedger {
	c := &memLedger{wallets: map[uuid.UUID]*domain.Wallet{}, spendableReads: m.spendableReads, applyCalls: m.applyCalls}
	for k, w := range m.wallets {
		cp := *w
		c.wallets[k] = &cp
	}
	c.ledger = append([]domain.LedgerEntry(nil), m.ledger...)
	c.lots = append([]domain.CreditLot(nil), m.lots...)
	c.allocs = append([]domain.CreditLotAllocation(nil), m.allocs...)
	return c
}

// ---- WalletRepository ----

func (m *memLedger) Create(_ context.Context, w *domain.Wallet) error {
	cp := *w
	m.wallets[w.UserID] = &cp
	return nil
}
func (m *memLedger) Get(_ context.Context, id uuid.UUID) (*domain.Wallet, error) {
	if w, ok := m.wallets[id]; ok {
		cp := *w
		return &cp, nil
	}
	return nil, repository.ErrNotFound
}
func (m *memLedger) GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.Wallet, error) {
	m.lockReads++
	return m.Get(ctx, id)
}
func (m *memLedger) Save(_ context.Context, w *domain.Wallet) error {
	cp := *w
	m.wallets[w.UserID] = &cp
	return nil
}
func (m *memLedger) AppendLedger(_ context.Context, e *domain.LedgerEntry) error {
	ok, _ := m.InsertLedgerIfAbsent(context.Background(), e)
	if !ok {
		return repository.ErrDuplicate
	}
	return nil
}
func (m *memLedger) InsertLedgerIfAbsent(_ context.Context, e *domain.LedgerEntry) (bool, error) {
	for _, x := range m.ledger {
		if x.IdempotencyKey == e.IdempotencyKey {
			return false, nil
		}
	}
	m.ledger = append(m.ledger, *e)
	return true, nil
}
func (m *memLedger) LedgerByKey(_ context.Context, key string) (*domain.LedgerEntry, error) {
	for _, x := range m.ledger {
		if x.IdempotencyKey == key {
			cp := x
			return &cp, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (m *memLedger) ListLedger(context.Context, uuid.UUID, *repository.Cursor, int) ([]domain.LedgerEntry, error) {
	return nil, nil
}

// ---- LotRepository ----

func (m *memLedger) CreateLot(_ context.Context, l *domain.CreditLot) error {
	for _, x := range m.lots {
		if l.LedgerEntryID != nil && x.LedgerEntryID != nil && *x.LedgerEntryID == *l.LedgerEntryID {
			return repository.ErrDuplicate
		}
		if l.ReferenceID != nil && x.ReferenceID != nil && *x.ReferenceID == *l.ReferenceID &&
			x.ReferenceType == l.ReferenceType && (l.ReferenceType == domain.LotRefCreditPurchase || l.ReferenceType == domain.LotRefSubscriptionGrant) {
			return repository.ErrDuplicate
		}
	}
	m.lots = append(m.lots, *l)
	return nil
}

func (m *memLedger) SpendableLots(_ context.Context, userID uuid.UUID, limit int) ([]domain.LotStock, error) {
	m.spendableReads++
	var rows []domain.CreditLot
	for _, l := range m.lots {
		if l.UserID == userID && l.CreditsRemaining > 0 {
			rows = append(rows, l)
		}
	}
	sortLots(rows)
	var out []domain.LotStock
	for _, l := range rows {
		if len(out) == limit {
			break
		}
		out = append(out, domain.LotStock{ID: l.ID, Remaining: l.CreditsRemaining, Value: l.ValueRemainingMillicents})
	}
	return out, nil
}

func sortLots(rows []domain.CreditLot) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		return a.ID.String() < b.ID.String()
	})
}

func (m *memLedger) lot(id uuid.UUID) *domain.CreditLot {
	for i := range m.lots {
		if m.lots[i].ID == id {
			return &m.lots[i]
		}
	}
	return nil
}

func (m *memLedger) LotOf(_ context.Context, userID, lotID uuid.UUID) (*domain.CreditLot, error) {
	if l := m.lot(lotID); l != nil && l.UserID == userID {
		cp := *l
		return &cp, nil
	}
	return nil, repository.ErrNotFound
}

func (m *memLedger) ApplyDeltas(_ context.Context, userID, entryID uuid.UUID, restores *uuid.UUID, deltas []domain.LotDelta, now time.Time) error {
	m.applyCalls++
	for _, d := range deltas {
		l := m.lot(d.LotID)
		if l == nil || l.UserID != userID {
			return errors.New("lot not found for user")
		}
		rem, val := l.CreditsRemaining-d.Credits, l.ValueRemainingMillicents-d.ValueMillicents
		if rem < 0 || rem > l.CreditsTotal || val < 0 || val > l.PaidCents*1000 { // the table's CHECK constraints
			return errors.New("lot constraint violated")
		}
		l.CreditsRemaining, l.ValueRemainingMillicents, l.UpdatedAt = rem, val, now
		m.allocs = append(m.allocs, domain.CreditLotAllocation{ID: domain.NewID(), LedgerEntryID: entryID, LotID: d.LotID, UserID: userID,
			Credits: d.Credits, ValueMillicents: d.ValueMillicents, RestoresEntryID: restores, CreatedAt: now})
	}
	return nil
}

func (m *memLedger) Allocations(_ context.Context, userID uuid.UUID, entryID *uuid.UUID, key string) ([]repository.LotAllocationRow, error) {
	id := uuid.Nil
	if entryID != nil {
		id = *entryID
	} else {
		for _, e := range m.ledger {
			if e.IdempotencyKey == key {
				id = e.ID
			}
		}
	}
	var out []repository.LotAllocationRow
	for _, a := range m.allocs {
		if a.UserID != userID || !((a.LedgerEntryID == id && a.Credits > 0) || (a.RestoresEntryID != nil && *a.RestoresEntryID == id)) {
			continue
		}
		l := m.lot(a.LotID)
		out = append(out, repository.LotAllocationRow{LotID: a.LotID, LedgerEntryID: a.LedgerEntryID, RestoresEntryID: a.RestoresEntryID,
			Credits: a.Credits, ValueMillicents: a.ValueMillicents, LotPriority: l.Priority, LotCreatedAt: l.CreatedAt, LotTotal: l.CreditsTotal,
			LotRemaining: l.CreditsRemaining, LotPaidCents: l.PaidCents, LotValue: l.ValueRemainingMillicents})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].LotPriority != out[j].LotPriority {
			return out[i].LotPriority < out[j].LotPriority
		}
		return out[i].LotCreatedAt.Before(out[j].LotCreatedAt)
	})
	return out, nil
}

func (m *memLedger) byRef(refType string, id uuid.UUID) (*domain.CreditLot, error) {
	for _, l := range m.lots {
		if l.ReferenceType == refType && l.ReferenceID != nil && *l.ReferenceID == id {
			cp := l
			return &cp, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (m *memLedger) PurchaseLot(_ context.Context, id uuid.UUID) (*domain.CreditLot, error) {
	return m.byRef(domain.LotRefCreditPurchase, id)
}
func (m *memLedger) GrantLot(_ context.Context, id uuid.UUID) (*domain.CreditLot, error) {
	return m.byRef(domain.LotRefSubscriptionGrant, id)
}
func (m *memLedger) PurchaseLots(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.CreditLot, error) {
	out := map[uuid.UUID]domain.CreditLot{}
	for _, id := range ids {
		if l, err := m.byRef(domain.LotRefCreditPurchase, id); err == nil {
			out[id] = *l
		}
	}
	return out, nil
}
func (m *memLedger) UserLots(_ context.Context, userID uuid.UUID, _ int) ([]domain.CreditLot, error) {
	var out []domain.CreditLot
	for _, l := range m.lots {
		if l.UserID == userID {
			out = append(out, l)
		}
	}
	return out, nil
}
func (m *memLedger) Reconcile(_ context.Context, after uuid.UUID, size int) (repository.LotChunk, error) {
	var ids []uuid.UUID
	for id := range m.wallets {
		if id.String() > after.String() {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	if len(ids) > size {
		ids = ids[:size]
	}
	out := repository.LotChunk{Scanned: len(ids)}
	for _, id := range ids {
		var sum int64
		for _, l := range m.lots {
			if l.UserID == id {
				sum += l.CreditsRemaining
			}
		}
		if sum != m.wallets[id].Balance {
			out.Mismatches = append(out.Mismatches, repository.LotMismatch{UserID: id, Balance: m.wallets[id].Balance, LotsRemaining: sum})
		}
		out.Last = id
	}
	return out, nil
}

// ---- harness ----

type ledgerHarness struct {
	t    *testing.T
	db   *memLedger
	svc  *WalletService
	clk  *clock.Fake
	user uuid.UUID
	seq  int
	logs *observer.ObservedLogs
	ctx  context.Context
}

func newLedgerHarness(t *testing.T) *ledgerHarness {
	t.Helper()
	db := newMemLedger()
	clk := clock.NewFake(time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC))
	svc := NewWalletService(nil, db, db, clk)
	svc.inTx = func(context.Context) bool { return true }
	core, logs := observer.New(zapcore.DebugLevel)
	h := &ledgerHarness{t: t, db: db, svc: svc, clk: clk, user: domain.NewID(), logs: logs,
		ctx: logger.WithContext(context.Background(), zap.New(core))}
	db.wallets[h.user] = &domain.Wallet{UserID: h.user}
	return h
}

// apply runs a movement the way a transaction would: a failure undoes everything.
func (h *ledgerHarness) apply(m Movement) (MovementResult, error) {
	h.t.Helper()
	m.UserID = h.user
	if m.IdempotencyKey == "" {
		h.seq++
		m.IdempotencyKey = "k" + string(rune('a'+h.seq))
	}
	if m.Type == "" {
		m.Type = domain.LedgerAdminAdjustment
	}
	snap := h.db.snapshot()
	res, err := h.svc.Execute(h.ctx, m)
	if err != nil {
		*h.db = *snap
	}
	return res, err
}

func (h *ledgerHarness) mustApply(m Movement) MovementResult {
	h.t.Helper()
	res, err := h.apply(m)
	if err != nil {
		h.t.Fatalf("apply %+v: %v", m, err)
	}
	return res
}

// lot adds a lot `ago` minutes in the past (so creation order is controllable).
func (h *ledgerHarness) addLot(src domain.LotSource, credits, paidCents int64, ageMinutes int) uuid.UUID {
	h.t.Helper()
	h.clk.Set(h.clk.Now().Add(-time.Duration(ageMinutes) * time.Minute))
	res := h.mustApply(Movement{Amount: credits, Lot: &LotSpec{Source: src, PaidCents: paidCents, Currency: "USD"}})
	h.clk.Set(h.clk.Now().Add(time.Duration(ageMinutes) * time.Minute))
	for _, l := range h.db.lots {
		if l.LedgerEntryID != nil && *l.LedgerEntryID == res.EntryID {
			return l.ID
		}
	}
	h.t.Fatal("lot not created")
	return uuid.Nil
}

func (h *ledgerHarness) remaining(id uuid.UUID) int64 { return h.db.lot(id).CreditsRemaining }
func (h *ledgerHarness) value(id uuid.UUID) int64     { return h.db.lot(id).ValueRemainingMillicents }

// invariant: Σ credits_remaining = balance.
func (h *ledgerHarness) checkInvariant() {
	h.t.Helper()
	var sum int64
	for _, l := range h.db.lots {
		sum += l.CreditsRemaining
		if l.CreditsRemaining < 0 || l.CreditsRemaining > l.CreditsTotal {
			h.t.Fatalf("lot %s out of bounds: %+v", l.ID, l)
		}
	}
	if b := h.db.wallets[h.user].Balance; sum != b {
		h.t.Fatalf("Σ remaining = %d, balance = %d", sum, b)
	}
}

func (h *ledgerHarness) errorLogs() int { return h.logs.FilterLevelExact(zapcore.ErrorLevel).Len() }

// ---- tests ----

func TestApplyConsumesBySourcePriorityThenAge(t *testing.T) {
	h := newLedgerHarness(t)
	// Created out of consumption order on purpose: bonus first, then a newer
	// purchase, a membership grant and an older purchase.
	bonus := h.addLot(domain.LotBonus, 100, 0, 50)
	newPurchase := h.addLot(domain.LotPurchase, 200, 1000, 20)
	member := h.addLot(domain.LotMembership, 50, 0, 40)
	oldPurchase := h.addLot(domain.LotPurchase, 300, 2000, 30)
	h.checkInvariant()

	h.mustApply(Movement{Type: domain.LedgerChatCharge, Amount: -250}) // all of the older purchase's lot is spent first
	if got := h.remaining(oldPurchase); got != 50 {
		t.Fatalf("older purchase remaining %d, want 50", got)
	}
	if h.remaining(newPurchase) != 200 || h.remaining(member) != 50 || h.remaining(bonus) != 100 {
		t.Fatal("only the oldest purchase lot may be touched")
	}
	h.mustApply(Movement{Type: domain.LedgerChatCharge, Amount: -100}) // 50 old purchase + 50 of the newer
	if h.remaining(oldPurchase) != 0 || h.remaining(newPurchase) != 150 {
		t.Fatalf("FIFO inside purchases broken: %d %d", h.remaining(oldPurchase), h.remaining(newPurchase))
	}
	h.mustApply(Movement{Type: domain.LedgerChatCharge, Amount: -200}) // 150 purchase, then 50 membership
	if h.remaining(newPurchase) != 0 || h.remaining(member) != 0 || h.remaining(bonus) != 100 {
		t.Fatalf("membership must be spent before bonus: purchase %d member %d bonus %d", h.remaining(newPurchase), h.remaining(member), h.remaining(bonus))
	}
	h.mustApply(Movement{Type: domain.LedgerChatCharge, Amount: -100})
	if h.remaining(bonus) != 0 {
		t.Fatal("bonus is spent last")
	}
	h.checkInvariant()
	if h.errorLogs() != 0 {
		t.Fatal("no uncovered debit expected")
	}
}

func TestApplySpansLotsAndValueIsExact(t *testing.T) {
	h := newLedgerHarness(t)
	a := h.addLot(domain.LotPurchase, 600, 999, 30) // 999 000 millicents
	b := h.addLot(domain.LotPurchase, 700, 1000, 20)
	c := h.addLot(domain.LotBonus, 10, 0, 10)

	res := h.mustApply(Movement{Type: domain.LedgerChatCharge, Amount: -1300 + 7}) // empties a, takes 601... see below
	// 1293 credits: all 600 of a (999 000), 693 of b: floor(693 × 1 000 000 / 700) = 990 000
	if h.remaining(a) != 0 || h.value(a) != 0 || h.remaining(b) != 7 || h.value(b) != 10_000 || h.remaining(c) != 10 {
		t.Fatalf("a %d/%d b %d/%d c %d", h.remaining(a), h.value(a), h.remaining(b), h.value(b), h.remaining(c))
	}
	if res.ValueMillicents != 999_000+990_000 {
		t.Fatalf("value %d, want %d", res.ValueMillicents, 999_000+990_000)
	}
	// Spending the rest of b takes exactly the value left, not a re-rounded share.
	res = h.mustApply(Movement{Type: domain.LedgerChatCharge, Amount: -7})
	if res.ValueMillicents != 10_000 || h.value(b) != 0 {
		t.Fatalf("emptying allocation took %d, lot keeps %d", res.ValueMillicents, h.value(b))
	}
	// Over a lot's whole life the allocations add up to exactly what was paid.
	var sum int64
	for _, al := range h.db.allocs {
		if al.LotID == b {
			sum += al.ValueMillicents
		}
	}
	if sum != 1_000_000 {
		t.Fatalf("allocations of lot b add up to %d, want 1000000", sum)
	}
	h.checkInvariant()
}

func TestApplyOneMinuteAtATimeConservesValue(t *testing.T) {
	h := newLedgerHarness(t)
	lot := h.addLot(domain.LotPurchase, 700, 999, 5)
	var total int64
	for i := 0; i < 700; i++ {
		total += h.mustApply(Movement{Type: domain.LedgerChatCharge, Amount: -1}).ValueMillicents
	}
	if total != 999_000 || h.remaining(lot) != 0 || h.value(lot) != 0 {
		t.Fatalf("700 single-credit debits recognised %d (want 999000); lot %d/%d", total, h.remaining(lot), h.value(lot))
	}
	h.checkInvariant()
}

func TestRestoreReturnsCreditsToTheSameLots(t *testing.T) {
	h := newLedgerHarness(t)
	p := h.addLot(domain.LotPurchase, 100, 500, 30)
	m := h.addLot(domain.LotMembership, 50, 0, 20)
	b := h.addLot(domain.LotBonus, 40, 0, 10)

	hold := h.mustApply(Movement{Type: domain.LedgerHold, Amount: -160, HeldAmount: 160, IdempotencyKey: "request_hold:r1"})
	if hold.ValueMillicents != 500_000 || h.remaining(p)+h.remaining(m)+h.remaining(b) != 30 {
		t.Fatalf("hold took value %d", hold.ValueMillicents)
	}
	// Meanwhile a chat spends some of what is left.
	h.mustApply(Movement{Type: domain.LedgerChatCharge, Amount: -10})

	rest := h.mustApply(Movement{Type: domain.LedgerHoldRefund, Amount: 160, HeldAmount: -160, RestoreKey: "request_hold:r1"})
	if rest.ValueMillicents != -500_000 {
		t.Fatalf("restored value %d, want -500000", rest.ValueMillicents)
	}
	// Exactly the allocations of the hold are back: purchase 100, membership 50, bonus 10 of 40.
	if h.remaining(p) != 100 || h.value(p) != 500_000 || h.remaining(m) != 50 || h.remaining(b) != 30 {
		t.Fatalf("lots after restore: p %d/%d m %d b %d", h.remaining(p), h.value(p), h.remaining(m), h.remaining(b))
	}
	if len(h.db.lots) != 3 {
		t.Fatal("a restore must not create lots when allocations exist")
	}
	if w := h.db.wallets[h.user]; w.Balance != 180 || w.Held != 0 {
		t.Fatalf("wallet %+v", w)
	}
	h.checkInvariant()
}

func TestRestoreByLedgerEntryID(t *testing.T) {
	h := newLedgerHarness(t)
	p := h.addLot(domain.LotPurchase, 100, 500, 30)
	hold := h.mustApply(Movement{Type: domain.LedgerHold, Amount: -60, HeldAmount: 60})
	h.mustApply(Movement{Type: domain.LedgerHoldRefund, Amount: 60, HeldAmount: -60, Restore: &hold.EntryID})
	if h.remaining(p) != 100 || h.value(p) != 500_000 {
		t.Fatalf("lot %d/%d", h.remaining(p), h.value(p))
	}
	h.checkInvariant()
}

func TestRestoreIsNotRepeatable(t *testing.T) {
	h := newLedgerHarness(t)
	p := h.addLot(domain.LotPurchase, 100, 500, 30)
	h.mustApply(Movement{Type: domain.LedgerHold, Amount: -60, HeldAmount: 60, IdempotencyKey: "hold"})
	h.mustApply(Movement{Type: domain.LedgerHoldRefund, Amount: 60, HeldAmount: -60, RestoreKey: "hold", IdempotencyKey: "back1"})
	// Held credits only exist once; a second restore cannot give purchase credits again.
	h.mustApply(Movement{Type: domain.LedgerAdminAdjustment, Amount: 5, Lot: &LotSpec{Source: domain.LotBonus}}) // balance for the next debit
	h.mustApply(Movement{Type: domain.LedgerRefund, Amount: 20, RestoreKey: "hold", IdempotencyKey: "back2"})
	if h.remaining(p) != 100 {
		t.Fatalf("purchase lot refilled twice: %d", h.remaining(p))
	}
	legacy := 0
	for _, l := range h.db.lots {
		if l.Origin == domain.LotOriginLegacyReturn {
			legacy++
			if l.Source != domain.LotBonus || l.PaidCents != 0 || l.CreditsTotal != 20 {
				t.Fatalf("legacy_return lot %+v", l)
			}
		}
	}
	if legacy != 1 {
		t.Fatalf("want one legacy_return bonus lot, got %d", legacy)
	}
	h.checkInvariant()
}

func TestRestoreWithoutAllocationsCreatesLegacyReturnLot(t *testing.T) {
	h := newLedgerHarness(t)
	h.mustApply(Movement{Type: domain.LedgerHoldRefund, Amount: 25, RestoreKey: "request_hold:old-order"})
	if len(h.db.lots) != 1 {
		t.Fatalf("lots %d", len(h.db.lots))
	}
	l := h.db.lots[0]
	if l.Source != domain.LotBonus || l.Origin != "legacy_return" || l.CreditsRemaining != 25 || l.PaidCents != 0 || l.LedgerEntryID == nil {
		t.Fatalf("lot %+v", l)
	}
	h.checkInvariant()
}

func TestPartialRestore(t *testing.T) {
	h := newLedgerHarness(t)
	p := h.addLot(domain.LotPurchase, 100, 999, 30)
	h.mustApply(Movement{Type: domain.LedgerHold, Amount: -100, HeldAmount: 100, IdempotencyKey: "hold"})
	res := h.mustApply(Movement{Type: domain.LedgerHoldRefund, Amount: 40, HeldAmount: -40, RestoreKey: "hold", IdempotencyKey: "part1"})
	if h.remaining(p) != 40 || res.ValueMillicents != -399_600 { // floor(40 × 999000 / 100)
		t.Fatalf("remaining %d value %d", h.remaining(p), res.ValueMillicents)
	}
	res = h.mustApply(Movement{Type: domain.LedgerHoldRefund, Amount: 60, HeldAmount: -60, RestoreKey: "hold", IdempotencyKey: "part2"})
	if h.remaining(p) != 100 || h.value(p) != 999_000 || res.ValueMillicents != -599_400 {
		t.Fatalf("after both parts: %d/%d (second part %d)", h.remaining(p), h.value(p), res.ValueMillicents)
	}
	h.checkInvariant()
}

func TestConsumeLotOnlyTouchesThatLot(t *testing.T) {
	h := newLedgerHarness(t)
	target := h.addLot(domain.LotPurchase, 100, 1000, 30)
	other := h.addLot(domain.LotPurchase, 100, 1000, 40) // older: priority consumption would take it first
	bonus := h.addLot(domain.LotBonus, 100, 0, 50)

	res := h.mustApply(Movement{Type: domain.LedgerRefund, Amount: -30, ConsumeLot: &target})
	if h.remaining(target) != 70 || h.remaining(other) != 100 || h.remaining(bonus) != 100 {
		t.Fatalf("target %d other %d bonus %d", h.remaining(target), h.remaining(other), h.remaining(bonus))
	}
	if res.ValueMillicents != 300_000 {
		t.Fatalf("value %d", res.ValueMillicents)
	}
	// More than the lot holds: refused, nothing written, balance untouched.
	before := h.db.wallets[h.user].Balance
	nLedger := len(h.db.ledger)
	if _, err := h.apply(Movement{Type: domain.LedgerRefund, Amount: -71, ConsumeLot: &target}); !errors.Is(err, ErrLotUnavailable) {
		t.Fatalf("want ErrLotUnavailable, got %v", err)
	}
	if h.db.wallets[h.user].Balance != before || len(h.db.ledger) != nLedger || h.remaining(target) != 70 {
		t.Fatal("a refused movement must leave no trace")
	}
	// A lot of someone else is refused too.
	foreign := uuid.New()
	if _, err := h.apply(Movement{Type: domain.LedgerRefund, Amount: -1, ConsumeLot: &foreign}); !errors.Is(err, ErrLotUnavailable) {
		t.Fatalf("unknown lot: %v", err)
	}
	h.checkInvariant()
}

func TestApplyReplayIsANoOp(t *testing.T) {
	h := newLedgerHarness(t)
	h.addLot(domain.LotPurchase, 100, 1000, 30)
	m := Movement{Type: domain.LedgerChatCharge, Amount: -40, IdempotencyKey: "chat:s1:1"}
	first := h.mustApply(m)
	if !first.Applied {
		t.Fatal("first apply")
	}
	again := h.mustApply(m)
	if again.Applied || again.ValueMillicents != 0 || again.Wallet.Balance != 60 {
		t.Fatalf("replay: %+v", again)
	}
	if len(h.db.allocs) != 1 {
		t.Fatalf("replay wrote allocations: %d", len(h.db.allocs))
	}
	h.checkInvariant()
}

func TestInsufficientFundsReplayStillReportsNotApplied(t *testing.T) {
	h := newLedgerHarness(t)
	h.addLot(domain.LotBonus, 10, 0, 5)
	m := Movement{Type: domain.LedgerChatCharge, Amount: -10, IdempotencyKey: "chat:s1:1"}
	if res := h.mustApply(m); !res.Applied {
		t.Fatal("first apply")
	}
	// The balance is now 0: the same movement again must be a no-op, not insufficient funds.
	res, err := h.apply(m)
	if err != nil || res.Applied {
		t.Fatalf("replay with an empty wallet: applied=%v err=%v", res.Applied, err)
	}
	// A different key that cannot be paid is insufficient funds, wallet returned, nothing written.
	n := len(h.db.ledger)
	res, err = h.apply(Movement{Type: domain.LedgerChatCharge, Amount: -1, IdempotencyKey: "chat:s1:2"})
	if !errors.Is(err, ErrInsufficientFunds) || res.Wallet == nil || len(h.db.ledger) != n {
		t.Fatalf("insufficient: %v %+v", err, res)
	}
	// Held cannot go negative either.
	h.addLot(domain.LotBonus, 10, 0, 1)
	if _, err := h.apply(Movement{Type: domain.LedgerHoldRelease, HeldAmount: -1}); !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("held underflow: %v", err)
	}
}

func TestUncoveredDebitStillAppliesAndIsLoggedAtErrorLevel(t *testing.T) {
	h := newLedgerHarness(t)
	h.db.wallets[h.user].Balance = 10 // a balance no lot accounts for (invariant broken)
	h.addLot(domain.LotBonus, 5, 0, 5)
	res, err := h.apply(Movement{Type: domain.LedgerChatCharge, Amount: -12, IdempotencyKey: "chat:s2:1"})
	if err != nil || !res.Applied {
		t.Fatalf("balance is authoritative: %v %+v", err, res)
	}
	if res.Wallet.Balance != 3 {
		t.Fatalf("balance %d, want 3", res.Wallet.Balance)
	}
	if h.errorLogs() != 1 {
		t.Fatalf("want one error log, got %d", h.errorLogs())
	}
	entry := h.logs.FilterLevelExact(zapcore.ErrorLevel).All()[0]
	if entry.ContextMap()["uncovered"] != int64(7) {
		t.Fatalf("logged %v", entry.ContextMap())
	}
}

func TestMovementsWithoutAValidLotDirectiveFailClosed(t *testing.T) {
	h := newLedgerHarness(t)
	h.addLot(domain.LotBonus, 50, 0, 5)
	entry := uuid.New()
	lot := uuid.New()
	cases := map[string]Movement{
		"credits added with no directive":       {Amount: 10},
		"lot and restore together":              {Amount: 10, Lot: &LotSpec{Source: domain.LotBonus}, RestoreKey: "x"},
		"restore by id and key":                 {Amount: 10, Restore: &entry, RestoreKey: "x"},
		"credits added consuming a lot":         {Amount: 10, ConsumeLot: &lot},
		"debit with a lot spec":                 {Amount: -1, Lot: &LotSpec{Source: domain.LotBonus}},
		"debit that restores":                   {Amount: -1, RestoreKey: "x"},
		"unknown source":                        {Amount: 5, Lot: &LotSpec{Source: "gift"}},
		"paid bonus lot":                        {Amount: 5, Lot: &LotSpec{Source: domain.LotBonus, PaidCents: 100}},
		"negative price":                        {Amount: 5, Lot: &LotSpec{Source: domain.LotPurchase, PaidCents: -1}},
		"held-only movement naming a directive": {HeldAmount: -1, RestoreKey: "x"},
	}
	for name, m := range cases {
		n, bal := len(h.db.ledger), h.db.wallets[h.user].Balance
		if _, err := h.apply(m); !errors.Is(err, ErrLotDirective) {
			t.Errorf("%s: want ErrLotDirective, got %v", name, err)
		}
		if len(h.db.ledger) != n || h.db.wallets[h.user].Balance != bal {
			t.Errorf("%s: wrote something", name)
		}
	}
	h.checkInvariant()
}

func TestHoldReleaseTouchesNoLots(t *testing.T) {
	h := newLedgerHarness(t)
	h.addLot(domain.LotPurchase, 100, 1000, 5)
	h.mustApply(Movement{Type: domain.LedgerHold, Amount: -60, HeldAmount: 60})
	n := len(h.db.allocs)
	res := h.mustApply(Movement{Type: domain.LedgerHoldRelease, HeldAmount: -60})
	if len(h.db.allocs) != n || res.ValueMillicents != 0 || h.db.applyCalls != 1 {
		t.Fatal("releasing a hold moves no credits and must not touch lots")
	}
	h.checkInvariant()
}

func TestConsumptionRoundTrips(t *testing.T) {
	h := newLedgerHarness(t)
	h.addLot(domain.LotPurchase, 1000, 1000, 5)
	h.db.spendableReads, h.db.applyCalls = 0, 0
	h.mustApply(Movement{Type: domain.LedgerChatCharge, Amount: -20})
	if h.db.spendableReads != 1 || h.db.applyCalls != 1 {
		t.Fatalf("one minute = one lot SELECT and one lot statement, got %d and %d", h.db.spendableReads, h.db.applyCalls)
	}
}

func TestManyLotsNeedASecondRead(t *testing.T) {
	h := newLedgerHarness(t)
	for i := 0; i < spendableLotsBatch+4; i++ {
		h.addLot(domain.LotBonus, 1, 0, 100-i)
	}
	h.db.spendableReads = 0
	h.mustApply(Movement{Type: domain.LedgerChatCharge, Amount: -(spendableLotsBatch + 2)})
	if h.db.spendableReads != 2 || h.db.wallets[h.user].Balance != 2 {
		t.Fatalf("reads %d balance %d", h.db.spendableReads, h.db.wallets[h.user].Balance)
	}
	h.checkInvariant()
}

func TestSignupMembershipAndPurchaseLotsCarryTheirReferences(t *testing.T) {
	h := newLedgerHarness(t)
	purchase, grant := uuid.New(), uuid.New()
	h.mustApply(Movement{Type: domain.LedgerPurchase, Amount: 600, ReferenceType: "credit_purchase", ReferenceID: &purchase,
		Lot: &LotSpec{Source: domain.LotPurchase, PaidCents: 1000, Currency: "USD"}})
	h.mustApply(Movement{Type: domain.LedgerSubscriptionGrant, Amount: 100, ReferenceType: "subscription", ReferenceID: &purchase,
		Lot: &LotSpec{Source: domain.LotMembership, ReferenceType: domain.LotRefSubscriptionGrant, ReferenceID: &grant}})
	pl, err := h.svc.PurchaseLot(h.ctx, purchase)
	if err != nil || pl.Source != domain.LotPurchase || pl.PaidCents != 1000 || pl.ValueRemainingMillicents != 1_000_000 || pl.Origin != "purchase" {
		t.Fatalf("purchase lot %+v %v", pl, err)
	}
	gl, err := h.svc.GrantLot(h.ctx, grant)
	if err != nil || gl.Source != domain.LotMembership || gl.PaidCents != 0 || gl.Priority != 1 {
		t.Fatalf("grant lot %+v %v", gl, err)
	}
	if _, err := h.svc.PurchaseLot(h.ctx, uuid.New()); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("unknown purchase: %v", err)
	}
	m, err := h.svc.PurchaseLots(h.ctx, []uuid.UUID{purchase, uuid.New()})
	if err != nil || len(m) != 1 || m[purchase].CreditsTotal != 600 {
		t.Fatalf("PurchaseLots %v %v", m, err)
	}
	sum, err := h.svc.LotsForUser(h.ctx, h.user)
	if err != nil || sum.PurchasedRemaining != 600 || sum.MembershipRemaining != 100 || sum.BonusRemaining != 0 || len(sum.Lots) != 2 {
		t.Fatalf("summary %+v %v", sum, err)
	}
}

func TestReconcileLotsReportsAndNeverChangesBalances(t *testing.T) {
	h := newLedgerHarness(t)
	h.addLot(domain.LotBonus, 100, 0, 5)
	// Two more wallets: one consistent, one whose balance no lot explains.
	good, bad := domain.NewID(), domain.NewID()
	h.db.wallets[good] = &domain.Wallet{UserID: good}
	h.db.wallets[bad] = &domain.Wallet{UserID: bad, Balance: 40}
	h.db.lots = append(h.db.lots, domain.CreditLot{ID: domain.NewID(), UserID: good, Source: domain.LotBonus, Priority: 2, CreditsTotal: 5, CreditsRemaining: 0})
	h.db.wallets[good].Balance = 0

	rep, err := h.svc.ReconcileLots(h.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Wallets != 3 || rep.Mismatched != 1 || rep.Drift != 40 {
		t.Fatalf("report %+v", rep)
	}
	if h.errorLogs() != 2 { // one line for the user, one summary with the counts
		t.Fatalf("error logs %d", h.errorLogs())
	}
	if h.db.wallets[bad].Balance != 40 || h.db.wallets[h.user].Balance != 100 {
		t.Fatal("reconciliation must never change balances")
	}
	// A clean ledger logs no error.
	h.db.wallets[bad].Balance = 0
	h.logs.TakeAll()
	if rep, _ := h.svc.ReconcileLots(h.ctx); rep.Mismatched != 0 || h.errorLogs() != 0 {
		t.Fatalf("clean run: %+v, %d errors", rep, h.errorLogs())
	}
}

// A caller that already holds the wallet lock (the billing path, after
// UseFreeMinute) hands the wallet over and Apply does not lock it again.
func TestApplyWithLockedWalletSkipsTheSecondLock(t *testing.T) {
	h := newLedgerHarness(t)
	h.addLot(domain.LotPurchase, 100, 1000, 5)
	locked, err := h.db.GetForUpdate(h.ctx, h.user)
	if err != nil {
		t.Fatal(err)
	}
	h.db.lockReads = 0
	res := h.mustApply(Movement{Type: domain.LedgerChatCharge, Amount: -20, Locked: locked})
	if h.db.lockReads != 0 || res.Wallet.Balance != 80 || h.db.wallets[h.user].Balance != 80 {
		t.Fatalf("lock reads %d, balance %d", h.db.lockReads, res.Wallet.Balance)
	}
	if locked.Balance != 100 {
		t.Fatal("the caller's wallet must not be mutated")
	}
	// Without it the wallet is locked once.
	h.db.lockReads = 0
	h.mustApply(Movement{Type: domain.LedgerChatCharge, Amount: -20})
	if h.db.lockReads != 1 {
		t.Fatalf("lock reads %d, want 1", h.db.lockReads)
	}
	h.checkInvariant()
}
