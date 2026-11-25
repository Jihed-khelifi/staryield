package service

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// Job names (the worker logs and tests drive them by name).
const (
	ReferralGrantJob = "referral-grants"
	ReferralSweepJob = "referral-sweep"
)

const (
	// grantBatch is how many referrals one transaction claims.
	grantBatch = 200
	// grantBudget bounds one run of the grant job; the next run (or the wake
	// signal) continues where it stopped.
	grantBudget = 45 * time.Second
	// rejectNoWallet is the reason of a referral whose inviter has no wallet
	// (never expected; the row must not block the queue).
	rejectNoWallet = "wallet_missing"
)

// RunGrants adds the earned minutes to the inviters' wallets once the rewards
// are launched, in batches of grantBatch until nothing is left or the time
// budget is spent. Job referral-grants (every minute, and woken by the launch,
// by an approval and by a referral that qualifies after launch).
//
// Safe on several instances and idempotent: a batch claims its rows with
// FOR UPDATE SKIP LOCKED, adds the minutes and flips the rows to granted in
// one transaction, so a row is granted exactly once and a crash grants
// nothing. Rows are claimed oldest first; wallets are locked in id order so
// two instances never wait on each other in a cycle.
func (s *ReferralService) RunGrants(ctx context.Context) error {
	if !s.d.Program(ctx).Launched() {
		return nil
	}
	deadline := time.Now().Add(grantBudget)
	for time.Now().Before(deadline) && ctx.Err() == nil {
		n, err := s.grantBatch(ctx)
		if err != nil {
			return err
		}
		if n < grantBatch {
			return nil
		}
	}
	return nil
}

// inviterGrant is what one batch adds to one inviter.
type inviterGrant struct {
	minutes int
	friends int
	ids     []uuid.UUID
	wallet  *domain.Wallet
}

// grantBatch runs one batch and returns how many rows it claimed.
func (s *ReferralService) grantBatch(ctx context.Context) (int, error) {
	var claimed int
	var grants map[uuid.UUID]*inviterGrant
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		claimed, grants = 0, nil
		rows, err := s.d.Repo.ClaimGrantable(ctx, grantBatch)
		if err != nil {
			return err
		}
		claimed = len(rows)
		if claimed == 0 {
			return nil
		}
		byInviter := map[uuid.UUID]*inviterGrant{}
		for i := range rows {
			g := byInviter[rows[i].ReferrerID]
			if g == nil {
				g = &inviterGrant{}
				byInviter[rows[i].ReferrerID] = g
			}
			g.minutes += rows[i].RewardMinutes
			g.friends++
			g.ids = append(g.ids, rows[i].ID)
		}
		order := make([]uuid.UUID, 0, len(byInviter))
		for id := range byInviter {
			order = append(order, id)
		}
		sort.Slice(order, func(i, j int) bool { return order[i].String() < order[j].String() })
		now := s.d.Clock.Now()
		var granted []uuid.UUID
		grants = map[uuid.UUID]*inviterGrant{}
		for _, inviter := range order {
			g := byInviter[inviter]
			w, err := s.d.Wallet.GrantFreeMinutes(ctx, inviter, g.minutes)
			switch {
			case errors.Is(err, repository.ErrNotFound):
				if err := s.rejectAll(ctx, rows, g.ids, now); err != nil {
					return err
				}
				continue
			case err != nil:
				return err
			}
			g.wallet = w
			grants[inviter] = g
			granted = append(granted, g.ids...)
		}
		if _, err := s.d.Repo.MarkGranted(ctx, granted, now); err != nil {
			return err
		}
		database.AfterCommit(ctx, func(ctx context.Context) { s.announceGrants(ctx, grants) })
		return nil
	})
	if err != nil {
		return 0, err
	}
	return claimed, nil
}

// rejectAll closes the claimed rows of an inviter whose wallet is missing so
// they cannot block the queue.
func (s *ReferralService) rejectAll(ctx context.Context, rows []domain.Referral, ids []uuid.UUID, now time.Time) error {
	want := map[uuid.UUID]bool{}
	for _, id := range ids {
		want[id] = true
	}
	for i := range rows {
		if !want[rows[i].ID] {
			continue
		}
		r := rows[i]
		r.Status, r.RejectReason, r.DecidedAt = domain.ReferralRejected, rejectNoWallet, &now
		if err := s.d.Repo.Save(ctx, &r); err != nil {
			return err
		}
	}
	return nil
}

// announceGrants publishes each inviter's new balance and tells them how many
// minutes were added (one notification per inviter per batch).
func (s *ReferralService) announceGrants(ctx context.Context, grants map[uuid.UUID]*inviterGrant) {
	for inviter, g := range grants {
		warnErr(ctx, "referrals: publish wallet", s.d.Events.Publish(ctx, realtime.EventWalletUpdated, walletView(g.wallet), inviter))
		s.d.Notifier.Notify(ctx, NotificationSpec{Kind: NotifyReferralRewardGranted,
			Data: map[string]any{"minutes": g.minutes, "friends": g.friends}, Link: referralLink}, inviter)
		s.d.Tracker.Track(ctx, "referral.granted", map[string]any{"referrer_id": inviter, "minutes": g.minutes, "friends": g.friends})
	}
}
