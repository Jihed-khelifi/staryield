package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
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
}

// ErrInsufficientFunds means a movement would make a balance negative.
var ErrInsufficientFunds = errors.New("service: insufficient funds")

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
	clock   clock.Clock
}

// NewWalletService returns a wallet service.
func NewWalletService(tx database.TxManager, wallets repository.WalletRepository, clk clock.Clock) *WalletService {
	return &WalletService{tx: tx, wallets: wallets, clock: clk}
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

// Apply locks the wallet, appends the ledger entry and updates balances.
// It must run inside a transaction. A repeated IdempotencyKey is a no-op that
// returns applied=false, which makes retries and worker re-runs safe.
func (s *WalletService) Apply(ctx context.Context, m Movement) (w *domain.Wallet, applied bool, err error) {
	if !database.InTx(ctx) {
		return nil, false, errors.New("service: wallet movement outside transaction")
	}
	if m.IdempotencyKey == "" {
		return nil, false, errors.New("service: wallet movement requires an idempotency key")
	}
	w, err = s.wallets.GetForUpdate(ctx, m.UserID)
	if err != nil {
		return nil, false, mapRepoErr(err, "Wallet")
	}
	if _, err := s.wallets.LedgerByKey(ctx, m.IdempotencyKey); err == nil {
		return w, false, nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, false, err
	}
	balance, held := w.Balance+m.Amount, w.Held+m.HeldAmount
	if balance < 0 || held < 0 {
		return w, false, ErrInsufficientFunds
	}
	now := s.clock.Now()
	entry := &domain.LedgerEntry{
		ID: domain.NewID(), UserID: m.UserID, EntryType: m.Type, Amount: m.Amount, HeldAmount: m.HeldAmount,
		BalanceAfter: balance, HeldAfter: held, ReferenceType: m.ReferenceType, ReferenceID: m.ReferenceID,
		IdempotencyKey: m.IdempotencyKey, Description: m.Description, ActorID: m.ActorID, CreatedAt: now,
	}
	if err := s.wallets.AppendLedger(ctx, entry); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return w, false, nil
		}
		return nil, false, err
	}
	w.Balance, w.Held, w.UpdatedAt = balance, held, now
	if err := s.wallets.Save(ctx, w); err != nil {
		return nil, false, err
	}
	return w, true, nil
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
