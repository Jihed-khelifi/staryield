package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// ErrNotInTransaction is a programming error: a grant must share the
// transaction that records why it was made, or a crash could grant twice.
var ErrNotInTransaction = errors.New("service: free minutes must be granted inside a transaction")

// GrantFreeMinutes adds promotional chat minutes to a wallet. It locks the
// wallet row and must run inside the caller's transaction (the one that also
// records the reason, so a retry can never grant twice). It returns the
// updated wallet; publish wallet.updated after the commit.
func (s *WalletService) GrantFreeMinutes(ctx context.Context, userID uuid.UUID, minutes int) (*domain.Wallet, error) {
	if minutes <= 0 {
		return nil, fmt.Errorf("service: grant of %d free minutes", minutes)
	}
	if !s.inTx(ctx) {
		return nil, ErrNotInTransaction
	}
	w, err := s.wallets.GetForUpdate(ctx, userID)
	if err != nil {
		return nil, err
	}
	w.FreeMinutes += minutes
	w.UpdatedAt = s.clock.Now()
	if err := s.wallets.Save(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}
