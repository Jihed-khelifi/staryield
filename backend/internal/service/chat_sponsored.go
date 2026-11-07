package service

import (
	"context"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
)

// revisions 3.0: site settings (§15.4). Billing of a sponsored chat: a chat
// requested while free mode was active.

// SetFreeMode installs the free-mode lookup (SiteService.FreeModeActive).
func (c *ChatService) SetFreeMode(fn func(ctx context.Context) bool) { c.freeModeActive = fn }

// billSponsoredMinuteLocked records the next started minute of a sponsored
// session (locked, Active). The platform pays: nothing is taken from the
// client's wallet, the client's own free minutes are kept, no low-balance
// warning is sent and the session never ends for lack of credits. The
// minute is recorded as usual (billed minutes, next billing time,
// session.billed) and counted as sponsored at the rate the client would
// have paid; the advisor earns at the list rate (domain.AdvisorGross).
func (c *ChatService) billSponsoredMinuteLocked(ctx context.Context, s *domain.Session, due time.Time, post *afterCommit) error {
	minute := s.BilledMinutes + 1
	s.BilledMinutes = minute
	s.SponsoredMinutes++
	s.SponsoredCredits += s.RatePerMinute
	next := due.Add(c.cfg.BillingInterval)
	s.NextBillingAt = &next
	if err := c.chat.SaveSession(ctx, s); err != nil {
		return err
	}
	snapshot := *s
	// The wallet is only read, so the client's screen stays in step.
	var wallet *WalletView
	if w, err := c.wallets.Get(ctx, s.ClientID); err == nil {
		v := walletView(w)
		wallet = &v
	}
	post.add(func(ctx context.Context) {
		billed := map[string]any{"session_id": snapshot.ID, "minute": minute, "free": true, "charged": 0, "sponsored": true,
			"next_billing_at": next, "active_seconds": int64(snapshot.ElapsedActive(c.clock.Now()) / time.Second)}
		clientPayload := make(map[string]any, len(billed)+2)
		for k, v := range billed {
			clientPayload[k] = v
		}
		if wallet != nil {
			clientPayload["wallet"] = *wallet
			clientPayload["minutes_remaining"] = int64(wallet.FreeMinutes) + wallet.Balance/max(snapshot.RatePerMinute, 1)
		}
		warnErr(ctx, "chat: publish billed", c.events.Publish(ctx, realtime.EventSessionBilled, clientPayload, snapshot.ClientID))

		// The advisor sees the list rate as the minute's value and their
		// live earnings, exactly as for a paid minute.
		psychicPayload := make(map[string]any, len(billed)+3)
		for k, v := range billed {
			psychicPayload[k] = v
		}
		psychicPayload["charged"] = max(snapshot.ListRatePerMinute, snapshot.RatePerMinute)
		earned, _ := domain.SplitEarnings(snapshot.AdvisorGross(), c.cfg.SharePercent)
		psychicPayload["psychic_earnings"] = earned
		if c.cfg.PayoutCentsPerCredit > 0 {
			psychicPayload["psychic_earnings_cents"], psychicPayload["currency"] = earned*c.cfg.PayoutCentsPerCredit, c.cfg.Currency
		}
		warnErr(ctx, "chat: publish billed", c.events.Publish(ctx, realtime.EventSessionBilled, psychicPayload, snapshot.PsychicID))
		c.track(ctx, "chat.minute_billed", &snapshot, map[string]any{"minute": minute, "free": true, "credits": 0, "sponsored": true})
	})
	return nil
}
