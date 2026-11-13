package service

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
)

// LotReconcileJob is the daily scheduler job that checks the lot invariant.
const LotReconcileJob = "credit-lot-reconcile"

// lotReconcileChunk is how many wallets one reconciliation page scans.
const lotReconcileChunk = 5000

// lotReconcileLogCap bounds the per-user error lines of one run (the summary
// line always carries the full count).
const lotReconcileLogCap = 50

// LotReconcileReport is the outcome of one reconciliation run.
type LotReconcileReport struct {
	Wallets    int // wallets scanned
	Mismatched int // wallets whose balance differs from Σ credits_remaining
	// Drift is Σ (balance − lots) over the mismatched wallets: positive when
	// wallets hold credits no lot accounts for.
	Drift int64
}

// ReconcileLots verifies, for every wallet, that wallets.balance equals the
// sum of the credits left in its lots (docs/platform-extensions.md §24.3).
// It pages through the wallets by user id and only reads: a mismatch is logged
// at error level and counted, never repaired, because the balance is the
// authority and a repair is a decision for a person.
func (s *WalletService) ReconcileLots(ctx context.Context) (LotReconcileReport, error) {
	log := logger.FromContext(ctx)
	var rep LotReconcileReport
	after := uuid.Nil
	for {
		chunk, err := s.lots.Reconcile(ctx, after, lotReconcileChunk)
		if err != nil {
			return rep, err
		}
		rep.Wallets += chunk.Scanned
		for _, m := range chunk.Mismatches {
			rep.Mismatched++
			rep.Drift += m.Balance - m.LotsRemaining
			if rep.Mismatched <= lotReconcileLogCap {
				log.Error("wallet: credit lots do not add up to the balance",
					zap.String("user_id", m.UserID.String()), zap.Int64("balance", m.Balance), zap.Int64("lots_remaining", m.LotsRemaining))
			}
		}
		if chunk.Scanned < lotReconcileChunk || chunk.Last == uuid.Nil {
			break
		}
		after = chunk.Last
		if err := ctx.Err(); err != nil {
			return rep, err
		}
	}
	if rep.Mismatched > 0 {
		log.Error("wallet: credit lot reconciliation found mismatches",
			zap.Int("wallets", rep.Wallets), zap.Int("mismatched", rep.Mismatched), zap.Int64("drift_credits", rep.Drift))
	} else {
		log.Info("wallet: credit lot reconciliation clean", zap.Int("wallets", rep.Wallets))
	}
	return rep, nil
}
