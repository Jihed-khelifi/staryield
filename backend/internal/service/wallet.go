package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// Movement is one credit movement applied to a wallet.
type Movement struct {
	UserID         uuid.UUID
	Type           domain.LedgerEntryType
	Amount         int64 // change to available balance
	HeldAmount     int64 // change to held balance
	ReferenceType  string
	ReferenceID    *uuid.UUID
	IdempotencyKey string
	Description    string
	ActorID        *uuid.UUID

	// revisions 5.0: credit lots. WalletService.Apply owns every lot change;
	// a movement says what to do with the lots through exactly one of these.
	//
	// Lot (Amount > 0): the credits form a new lot.
	// Restore or RestoreKey (Amount > 0): return the credits to the lots the
	// named movement consumed (by ledger entry id, or by idempotency key when
	// the caller only knows that).
	// ConsumeLot (Amount < 0): take the credits from this lot only; without it
	// a debit consumes lots by priority, created_at, id.
	Lot        *LotSpec
	Restore    *uuid.UUID
	RestoreKey string
	ConsumeLot *uuid.UUID

	// Locked is the wallet of UserID as the caller just read it with its row
	// lock held in this transaction (WalletService.UseFreeMinute returns one).
	// Apply then skips locking it again, saving a round trip on the hot
	// billing path. Never pass a wallet that was not locked in this transaction.
	Locked *domain.Wallet
}

// LotSpec describes the lot a credit-adding movement creates.
type LotSpec struct {
	Source    domain.LotSource
	PaidCents int64 // what was paid for the whole lot (purchase only)
	Currency  string
	// ReferenceType and ReferenceID default to the movement's own.
	ReferenceType string
	ReferenceID   *uuid.UUID
}

// MovementResult is the full outcome of a movement.
type MovementResult struct {
	Wallet  *domain.Wallet
	Applied bool
	// EntryID is the new ledger entry (zero on a replay).
	EntryID uuid.UUID
	// ValueMillicents is the revenue basis of the credits the movement
	// consumed from lots (negative when it restored credits).
	ValueMillicents int64
}

// Errors of the wallet.
var (
	// ErrInsufficientFunds means a movement would make a balance negative.
	ErrInsufficientFunds = errors.New("service: insufficient funds")
	// ErrLotDirective is a programming error: a credit-adding movement
	// without Lot or Restore (or a directive that does not fit the movement).
	// Money fails closed: nothing is written, the transaction rolls back.
	ErrLotDirective = errors.New("service: wallet movement has no valid lot directive")
	// ErrLotUnavailable means ConsumeLot names a lot that does not exist for
	// the user or holds fewer credits than the movement takes.
	ErrLotUnavailable = errors.New("service: lot cannot supply the movement")
)

// spendableLotsBatch is how many lots one consumption reads; a debit that
// needs more reads the rest in a second query.
const spendableLotsBatch = 16

// WalletView is the client-facing wallet.
type WalletView struct {
	Balance     int64 `json:"balance"`
	Held        int64 `json:"held"`
	FreeMinutes int   `json:"free_minutes"`
}

// LedgerView is a client-facing ledger entry.
type LedgerView struct {
	ID            uuid.UUID  `json:"id"`
	Type          string     `json:"type"`
	Amount        int64      `json:"amount"`
	HeldAmount    int64      `json:"held_amount"`
	BalanceAfter  int64      `json:"balance_after"`
	HeldAfter     int64      `json:"held_after"`
	ReferenceType string     `json:"reference_type,omitempty"`
	ReferenceID   *uuid.UUID `json:"reference_id,omitempty"`
	Description   string     `json:"description"`
	CreatedAt     time.Time  `json:"created_at"`
}

// WalletService applies credit movements atomically with their ledger rows.
type WalletService struct {
	tx      database.TxManager
	wallets repository.WalletRepository
	lots    repository.LotRepository
	clock   clock.Clock
	// inTx reports whether ctx carries a transaction (database.InTx; unit tests
	// with in-memory repositories replace it).
	inTx func(context.Context) bool
}

// NewWalletService returns a wallet service.
func NewWalletService(tx database.TxManager, wallets repository.WalletRepository, lots repository.LotRepository, clk clock.Clock) *WalletService {
	return &WalletService{tx: tx, wallets: wallets, lots: lots, clock: clk, inTx: database.InTx}
}

func walletView(w *domain.Wallet) WalletView {
	return WalletView{Balance: w.Balance, Held: w.Held, FreeMinutes: w.FreeMinutes}
}

// Get returns the caller's wallet.
func (s *WalletService) Get(ctx context.Context, userID uuid.UUID) (WalletView, error) {
	w, err := s.wallets.Get(ctx, userID)
	if err != nil {
		return WalletView{}, mapRepoErr(err, "Wallet")
	}
	return walletView(w), nil
}

// Ledger lists the caller's credit movements, newest first.
func (s *WalletService) Ledger(ctx context.Context, userID uuid.UUID, cursor string, limit int) (Page[LedgerView], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[LedgerView]{}, err
	}
	rows, err := s.wallets.ListLedger(ctx, userID, c, limit)
	if err != nil {
		return Page[LedgerView]{}, apperror.Internal(err)
	}
	out := Page[LedgerView]{Items: make([]LedgerView, 0, len(rows))}
	for _, e := range rows {
		out.Items = append(out.Items, LedgerView{
			ID: e.ID, Type: string(e.EntryType), Amount: e.Amount, HeldAmount: e.HeldAmount,
			BalanceAfter: e.BalanceAfter, HeldAfter: e.HeldAfter, ReferenceType: e.ReferenceType,
			ReferenceID: e.ReferenceID, Description: e.Description, CreatedAt: e.CreatedAt,
		})
	}
	if len(rows) > 0 && len(rows) == clampPage(limit, 50, 100) {
		last := rows[len(rows)-1]
		out.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return out, nil
}

// Apply locks the wallet, appends the ledger entry, moves the lots and
// updates balances. It must run inside a transaction. A repeated
// IdempotencyKey is a no-op that returns applied=false, which makes retries
// and worker re-runs safe.
func (s *WalletService) Apply(ctx context.Context, m Movement) (w *domain.Wallet, applied bool, err error) {
	res, err := s.Execute(ctx, m)
	return res.Wallet, res.Applied, err
}

// Execute is Apply with the full result (the new ledger entry and the
// revenue basis the movement moved).
//
// Round trips: the wallet lock, one INSERT … ON CONFLICT DO NOTHING on the
// ledger (its row count tells a replay from a fresh movement, so no separate
// key lookup), the lot work (one SELECT of candidate lots plus one statement
// that updates them and appends the allocations) and the wallet save. A
// movement that would overdraw is looked up by key first, because a replay
// must return applied=false even when the balance could no longer pay it.
//
// Lots need no row locks of their own: every change to a user's lots happens
// here, after the user's wallet row was locked, so two movements of one user
// never interleave.
func (s *WalletService) Execute(ctx context.Context, m Movement) (MovementResult, error) {
	if !s.inTx(ctx) {
		return MovementResult{}, errors.New("service: wallet movement outside transaction")
	}
	if m.IdempotencyKey == "" {
		return MovementResult{}, errors.New("service: wallet movement requires an idempotency key")
	}
	if err := m.validateLots(); err != nil {
		return MovementResult{}, err
	}
	var w *domain.Wallet
	if m.Locked != nil && m.Locked.UserID == m.UserID {
		cp := *m.Locked
		w = &cp
	} else {
		var err error
		if w, err = s.wallets.GetForUpdate(ctx, m.UserID); err != nil {
			return MovementResult{}, mapRepoErr(err, "Wallet")
		}
	}
	balance, held := w.Balance+m.Amount, w.Held+m.HeldAmount
	if balance < 0 || held < 0 {
		if _, err := s.wallets.LedgerByKey(ctx, m.IdempotencyKey); err == nil {
			return MovementResult{Wallet: w}, nil
		} else if !errors.Is(err, repository.ErrNotFound) {
			return MovementResult{}, err
		}
		return MovementResult{Wallet: w}, ErrInsufficientFunds
	}
	now := s.clock.Now()
	entry := &domain.LedgerEntry{
		ID: domain.NewID(), UserID: m.UserID, EntryType: m.Type, Amount: m.Amount, HeldAmount: m.HeldAmount,
		BalanceAfter: balance, HeldAfter: held, ReferenceType: m.ReferenceType, ReferenceID: m.ReferenceID,
		IdempotencyKey: m.IdempotencyKey, Description: m.Description, ActorID: m.ActorID, CreatedAt: now,
	}
	fresh, err := s.wallets.InsertLedgerIfAbsent(ctx, entry)
	if err != nil {
		return MovementResult{}, err
	}
	if !fresh {
		return MovementResult{Wallet: w}, nil
	}
	value, err := s.moveLots(ctx, m, entry, now)
	if err != nil {
		return MovementResult{}, err
	}
	w.Balance, w.Held, w.UpdatedAt = balance, held, now
	if err := s.wallets.Save(ctx, w); err != nil {
		return MovementResult{}, err
	}
	return MovementResult{Wallet: w, Applied: true, EntryID: entry.ID, ValueMillicents: value}, nil
}

// validateLots checks that the lot directive fits the movement. A credit-adding
// movement without one is refused in production too: silently minting a lot
// would hide a mispriced purchase (paid credits treated as free, or free
// credits as refundable), which is worse than a failed request that the tests
// of every call site would have caught.
func (m Movement) validateLots() error {
	directives := 0
	for _, set := range []bool{m.Lot != nil, m.Restore != nil || m.RestoreKey != "", m.ConsumeLot != nil} {
		if set {
			directives++
		}
	}
	switch {
	case m.Amount > 0:
		if m.ConsumeLot != nil || (m.Lot != nil) == (m.Restore != nil || m.RestoreKey != "") {
			return fmt.Errorf("%w: credits added by %q need exactly one of Lot or Restore", ErrLotDirective, m.IdempotencyKey)
		}
		if m.Restore != nil && m.RestoreKey != "" {
			return fmt.Errorf("%w: %q names the movement to restore twice", ErrLotDirective, m.IdempotencyKey)
		}
		if l := m.Lot; l != nil {
			if !l.Source.Valid() || l.PaidCents < 0 || (l.Source != domain.LotPurchase && l.PaidCents != 0) {
				return fmt.Errorf("%w: %q has an invalid lot (%s, %d cents)", ErrLotDirective, m.IdempotencyKey, l.Source, l.PaidCents)
			}
		}
	case m.Amount < 0:
		if m.Lot != nil || m.Restore != nil || m.RestoreKey != "" {
			return fmt.Errorf("%w: a debit (%q) cannot add or restore lots", ErrLotDirective, m.IdempotencyKey)
		}
	default:
		if directives > 0 {
			return fmt.Errorf("%w: %q moves no credits but names a lot directive", ErrLotDirective, m.IdempotencyKey)
		}
	}
	return nil
}

// moveLots applies the lot side of a movement that was just appended to the
// ledger and returns the value (revenue basis) it consumed or restored.
func (s *WalletService) moveLots(ctx context.Context, m Movement, entry *domain.LedgerEntry, now time.Time) (int64, error) {
	switch {
	case m.Amount > 0 && m.Lot != nil:
		return 0, s.createLot(ctx, m, entry, now, *m.Lot, m.Amount, string(m.Type))
	case m.Amount > 0:
		return s.restore(ctx, m, entry, now)
	case m.Amount < 0:
		return s.consume(ctx, m, entry, now)
	}
	return 0, nil
}

func (s *WalletService) createLot(ctx context.Context, m Movement, entry *domain.LedgerEntry, now time.Time, spec LotSpec, credits int64, origin string) error {
	refType, refID := spec.ReferenceType, spec.ReferenceID
	if refType == "" {
		refType, refID = m.ReferenceType, m.ReferenceID
	}
	return s.lots.CreateLot(ctx, &domain.CreditLot{
		ID: domain.NewID(), UserID: m.UserID, Source: spec.Source, Priority: spec.Source.Priority(), Origin: origin,
		LedgerEntryID: &entry.ID, ReferenceType: refType, ReferenceID: refID, CreditsTotal: credits, CreditsRemaining: credits,
		PaidCents: spec.PaidCents, Currency: spec.Currency, ValueRemainingMillicents: spec.PaidCents * 1000,
		CreatedAt: now, UpdatedAt: now,
	})
}

func (s *WalletService) consume(ctx context.Context, m Movement, entry *domain.LedgerEntry, now time.Time) (int64, error) {
	need := -m.Amount
	var deltas []domain.LotDelta
	if m.ConsumeLot != nil {
		lot, err := s.lots.LotOf(ctx, m.UserID, *m.ConsumeLot)
		if errors.Is(err, repository.ErrNotFound) {
			return 0, fmt.Errorf("%w: lot %s", ErrLotUnavailable, *m.ConsumeLot)
		}
		if err != nil {
			return 0, err
		}
		if lot.CreditsRemaining < need {
			return 0, fmt.Errorf("%w: lot %s holds %d of %d credits", ErrLotUnavailable, lot.ID, lot.CreditsRemaining, need)
		}
		deltas, _ = domain.PlanConsumption([]domain.LotStock{{ID: lot.ID, Remaining: lot.CreditsRemaining, Value: lot.ValueRemainingMillicents}}, need)
	} else {
		stock, err := s.lots.SpendableLots(ctx, m.UserID, spendableLotsBatch)
		if err != nil {
			return 0, err
		}
		var uncovered int64
		deltas, uncovered = domain.PlanConsumption(stock, need)
		if uncovered > 0 && len(stock) == spendableLotsBatch {
			if stock, err = s.lots.SpendableLots(ctx, m.UserID, 1000); err != nil {
				return 0, err
			}
			deltas, uncovered = domain.PlanConsumption(stock, need)
		}
		if uncovered > 0 {
			// The invariant Σ credits_remaining = balance is broken for this user.
			// The balance is authoritative (the client really had the credits), so
			// the debit stands; the reconciliation job keeps reporting the user.
			logger.FromContext(ctx).Error("wallet: debit exceeds the user's lots; lots and balance disagree",
				zap.String("user_id", m.UserID.String()), zap.String("key", m.IdempotencyKey),
				zap.Int64("debit", need), zap.Int64("uncovered", uncovered))
		}
	}
	return sumValue(deltas), s.lots.ApplyDeltas(ctx, m.UserID, entry.ID, nil, deltas, now)
}

func (s *WalletService) restore(ctx context.Context, m Movement, entry *domain.LedgerEntry, now time.Time) (int64, error) {
	rows, err := s.lots.Allocations(ctx, m.UserID, m.Restore, m.RestoreKey)
	if err != nil {
		return 0, err
	}
	var original *uuid.UUID
	var sources []domain.LotRestoreSource
	index := map[uuid.UUID]int{}
	for _, r := range rows {
		i, ok := index[r.LotID]
		if !ok {
			i = len(sources)
			index[r.LotID] = i
			sources = append(sources, domain.LotRestoreSource{LotID: r.LotID, Room: r.LotTotal - r.LotRemaining, ValueRoom: r.LotPaidCents*1000 - r.LotValue})
		}
		// Consumed credits count up, credits an earlier restore returned count down.
		sources[i].Credits += r.Credits
		sources[i].Value += r.ValueMillicents
		if r.Credits > 0 {
			id := r.LedgerEntryID
			original = &id
		}
	}
	kept := sources[:0]
	for _, src := range sources {
		if src.Credits > 0 {
			kept = append(kept, src)
		}
	}
	deltas, leftover := domain.PlanRestore(kept, m.Amount)
	if len(deltas) > 0 {
		if err := s.lots.ApplyDeltas(ctx, m.UserID, entry.ID, original, deltas, now); err != nil {
			return 0, err
		}
	}
	if leftover > 0 {
		// Nothing (or not enough) to restore: movements from before lots existed.
		// The credits are real and were spent from the balance, but their origin
		// is unknown, so they come back as free credits.
		logger.FromContext(ctx).Warn("wallet: restoring credits without lot allocations; creating a legacy_return bonus lot",
			zap.String("user_id", m.UserID.String()), zap.String("key", m.IdempotencyKey), zap.Int64("credits", leftover))
		if err := s.createLot(ctx, m, entry, now, LotSpec{Source: domain.LotBonus}, leftover, domain.LotOriginLegacyReturn); err != nil {
			return 0, err
		}
	}
	return sumValue(deltas), nil
}

func sumValue(deltas []domain.LotDelta) int64 {
	var v int64
	for _, d := range deltas {
		v += d.ValueMillicents
	}
	return v
}

// LotView is the summary of one lot (admin statements, refund requests).
type LotView struct {
	ID                       uuid.UUID  `json:"id"`
	Source                   string     `json:"source"`
	Origin                   string     `json:"origin"`
	CreditsTotal             int64      `json:"credits_total"`
	CreditsRemaining         int64      `json:"credits_remaining"`
	PaidCents                int64      `json:"paid_cents"`
	Currency                 string     `json:"currency"`
	ValueRemainingMillicents int64      `json:"value_remaining_millicents"`
	ReferenceType            string     `json:"reference_type,omitempty"`
	ReferenceID              *uuid.UUID `json:"reference_id,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
}

// LotsSummary is a user's lots, newest first, with what is left per source.
type LotsSummary struct {
	Lots                []LotView `json:"lots"`
	PurchasedRemaining  int64     `json:"purchased_remaining"`
	MembershipRemaining int64     `json:"membership_remaining"`
	BonusRemaining      int64     `json:"bonus_remaining"`
}

// LotsForUser lists the user's lots (up to 500, newest first) and the credits
// left in each source. Read-only.
func (s *WalletService) LotsForUser(ctx context.Context, userID uuid.UUID) (LotsSummary, error) {
	rows, err := s.lots.UserLots(ctx, userID, 500)
	if err != nil {
		return LotsSummary{}, apperror.Internal(err)
	}
	out := LotsSummary{Lots: make([]LotView, 0, len(rows))}
	for _, l := range rows {
		out.Lots = append(out.Lots, LotView{ID: l.ID, Source: string(l.Source), Origin: l.Origin, CreditsTotal: l.CreditsTotal,
			CreditsRemaining: l.CreditsRemaining, PaidCents: l.PaidCents, Currency: l.Currency,
			ValueRemainingMillicents: l.ValueRemainingMillicents, ReferenceType: l.ReferenceType, ReferenceID: l.ReferenceID, CreatedAt: l.CreatedAt})
		switch l.Source {
		case domain.LotPurchase:
			out.PurchasedRemaining += l.CreditsRemaining
		case domain.LotMembership:
			out.MembershipRemaining += l.CreditsRemaining
		default:
			out.BonusRemaining += l.CreditsRemaining
		}
	}
	return out, nil
}

// PurchaseLot returns the lot a credit purchase created, or
// repository.ErrNotFound when it has none (a purchase that never completed).
// Read it inside the transaction that books a refund, after locking the
// wallet, so the remaining credits cannot change under the caller.
func (s *WalletService) PurchaseLot(ctx context.Context, purchaseID uuid.UUID) (*domain.CreditLot, error) {
	return s.lots.PurchaseLot(ctx, purchaseID)
}

// GrantLot returns the membership lot of a subscription grant, or
// repository.ErrNotFound (a grant from before lots existed, or one with no
// credits).
func (s *WalletService) GrantLot(ctx context.Context, grantID uuid.UUID) (*domain.CreditLot, error) {
	return s.lots.GrantLot(ctx, grantID)
}

// PurchaseLots returns the lots of several purchases with one query, keyed by
// purchase id (purchases without a lot are absent).
func (s *WalletService) PurchaseLots(ctx context.Context, purchaseIDs []uuid.UUID) (map[uuid.UUID]domain.CreditLot, error) {
	m, err := s.lots.PurchaseLots(ctx, purchaseIDs)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return m, nil
}

// UseFreeMinute consumes one promotional minute if available (inside a tx).
func (s *WalletService) UseFreeMinute(ctx context.Context, userID uuid.UUID) (*domain.Wallet, bool, error) {
	w, err := s.wallets.GetForUpdate(ctx, userID)
	if err != nil {
		return nil, false, mapRepoErr(err, "Wallet")
	}
	if w.FreeMinutes <= 0 {
		return w, false, nil
	}
	w.FreeMinutes--
	w.UpdatedAt = s.clock.Now()
	if err := s.wallets.Save(ctx, w); err != nil {
		return nil, false, err
	}
	return w, true, nil
}

func clampPage(limit, def, max int) int {
	if limit <= 0 {
		return def
	}
	if limit > max {
		return max
	}
	return limit
}
