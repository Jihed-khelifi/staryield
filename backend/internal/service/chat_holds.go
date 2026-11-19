package service

// revisions 6.0 (docs/platform-extensions.md §26.1): the free-trial hold.
//
// When a reading's next minute cannot be paid it is held for hold_seconds
// instead of ending: the advisor keeps writing, the client sees those replies
// locked and tops up. A top-up in time converts the hold (one minute is
// charged, the reading continues, the replies are revealed); otherwise the
// reading ends with hold_expired and the replies stay hidden until the client
// pays one minute to reveal them.
//
// Locks are taken in one order everywhere: session, hold, wallet.

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// holdWiring is the hold's collaborators.
type holdWiring struct {
	// settings reads the hold settings (SiteService.TrialHold); nil means holds
	// are off.
	settings func(ctx context.Context) TrialHoldSettings
}

// SetTrialHold installs the hold settings reader (SiteService.TrialHold).
func (c *ChatService) SetTrialHold(fn func(ctx context.Context) TrialHoldSettings) {
	c.holds.settings = fn
}

// trialHoldSettings reads the settings outside any transaction (a cache miss
// must never run inside, or poison, the billing transaction). Without a reader
// they are the zero value: no hold.
func (c *ChatService) trialHoldSettings(ctx context.Context) TrialHoldSettings {
	if c.holds.settings == nil {
		return TrialHoldSettings{}
	}
	return c.holds.settings(database.Detach(ctx))
}

// ---------------------------------------------------------------------------
// Entering a hold
// ---------------------------------------------------------------------------

// holdOrEndLocked is what billing does when the client cannot pay the next
// minute of s (locked, active): hold the reading when the settings say so,
// else end it (BR-03).
func (c *ChatService) holdOrEndLocked(ctx context.Context, s *domain.Session, now time.Time, post *afterCommit) error {
	trialEnded := domain.TrialEnded(s.FreeMinutesUsed, s.CreditsCharged)
	cfg := c.trialHoldSettings(ctx)
	if !s.Sponsored && cfg.Applies(trialEnded) {
		return c.enterHoldLocked(ctx, s, cfg, trialEnded, now, post)
	}
	return c.endLocked(ctx, s, domain.EndCreditsExhausted, now, post)
}

// enterHoldLocked holds s: active time is accrued up to now, the session
// becomes held (not billed, not active time) and a hold row is created, all in
// the caller's transaction.
func (c *ChatService) enterHoldLocked(ctx context.Context, s *domain.Session, cfg TrialHoldSettings, trialEnded bool, now time.Time, post *afterCommit) error {
	c.accrueActive(s, now)
	expires := now.Add(cfg.HoldDuration())
	hold := &domain.ChatHold{ID: domain.NewID(), SessionID: s.ID, ConversationID: s.ConversationID, ClientID: s.ClientID, PsychicID: s.PsychicID,
		Reason: domain.HoldCreditsExhausted, Status: domain.HoldOpen, StartedAt: now, ExpiresAt: expires,
		MaxExpiresAt: expires.Add(cfg.PaymentGrace()), PriceCredits: s.RatePerMinute,
		ListRateCredits: max(s.ListRatePerMinute, s.RatePerMinute), CreatedAt: now, UpdatedAt: now}
	if trialEnded {
		hold.Reason = domain.HoldTrialEnded
	}
	if err := c.chat.CreateHold(ctx, hold); err != nil {
		return err
	}
	s.Status, s.HoldExpiresAt, s.LastHoldID = domain.SessionHeld, &expires, &hold.ID
	s.NextBillingAt, s.LastResumedAt = nil, nil
	if err := c.chat.SaveSession(ctx, s); err != nil {
		return err
	}
	snap, h := *s, *hold
	post.add(func(ctx context.Context) {
		c.publishSessionHold(ctx, realtime.EventSessionHeld, &snap, &h)
		c.track(ctx, "chat.hold_started", &snap, map[string]any{"hold_id": h.ID, "reason": string(h.Reason), "price": h.PriceCredits})
	})
	return nil
}

// publishSessionHold sends event with the per-viewer session and hold views to
// both participants.
func (c *ChatService) publishSessionHold(ctx context.Context, event string, s *domain.Session, h *domain.ChatHold) {
	ps := c.newParticipants()
	ps.holds[h.ID] = h
	for _, uid := range []uuid.UUID{s.ClientID, s.PsychicID} {
		w := viewer{ID: uid}
		payload := map[string]any{"session": c.sessionView(ctx, ps, s, w), "hold": holdView(h, w)}
		warnErr(ctx, "chat: publish "+event, c.events.Publish(ctx, event, payload, uid))
	}
}

// holdForMessage returns the open hold a message is written under: the
// advisor's text and images during a hold (never a client's message, never a
// system notice). s is locked.
func (c *ChatService) holdForMessage(ctx context.Context, s *domain.Session, sender uuid.UUID, kind domain.MessageKind) (*domain.ChatHold, error) {
	if s.Status != domain.SessionHeld || sender != s.PsychicID || (kind != domain.MessageText && kind != domain.MessageImage) {
		return nil, nil
	}
	h, err := c.chat.OpenHoldForSessionForUpdate(ctx, s.ID)
	if err != nil {
		return nil, err
	}
	return h, nil
}

// ---------------------------------------------------------------------------
// Conversion: a top-up in time
// ---------------------------------------------------------------------------

// convertState is the outcome of trying to convert one held reading.
type convertState int

const (
	// convertedHold: one minute was charged and the reading continues.
	convertedHold convertState = iota + 1
	// notHeldAnymore: the session is not held (already converted, ended, ...).
	notHeldAnymore
	// notEnoughCredits: the client cannot pay the minute; nothing changed.
	notEnoughCredits
)

type convertResult struct {
	state  convertState
	wallet *domain.Wallet
	price  int64
}

// ResumeHolds tries to convert every held reading of a client. It runs after
// a credit purchase succeeded and after an admin credit grant (§26.1); a
// reading that cannot be paid yet stays held. Errors are logged, never
// returned: the caller has already done its own work.
func (c *ChatService) ResumeHolds(ctx context.Context, clientID uuid.UUID) {
	// The work belongs to the client (analytics attribution), not to whoever
	// triggered it, and must outlive a cancelled request.
	ctx = reqctx.WithPrincipal(context.WithoutCancel(database.Detach(ctx)), reqctx.Principal{UserID: clientID, Role: domain.RoleClient})
	ids, err := c.chat.HeldSessionIDsForClient(ctx, clientID)
	if err != nil {
		logger.FromContext(ctx).Error("chat: look up held readings", zap.Error(err), zap.String("client_id", clientID.String()))
		return
	}
	for _, id := range ids {
		if _, err := c.convertHold(ctx, id); err != nil {
			logger.FromContext(ctx).Error("chat: convert hold", zap.Error(err), zap.String("session_id", id.String()))
		}
	}
}

// ContinueHold is POST /chat/sessions/:id/hold/continue: convert the client's
// held reading now.
func (c *ChatService) ContinueHold(ctx context.Context, clientID, sessionID uuid.UUID, key string) (*HoldContinued, error) {
	if !ValidIdempotencyKey(key) {
		return nil, apperror.Validation(map[string]string{"idempotency_key": "A valid idempotency key is required."})
	}
	s, err := c.chat.GetSession(ctx, sessionID)
	if err != nil || s.ClientID != clientID {
		return nil, mapRepoErr(repository.ErrNotFound, "Session")
	}
	res, err := c.convertHold(ctx, sessionID)
	if err != nil {
		return nil, mapRepoErr(err, "Session")
	}
	switch res.state {
	case notHeldAnymore:
		return nil, apperror.InvalidState("This reading is not waiting for a top-up.")
	case notEnoughCredits:
		return nil, apperror.InsufficientCreditsFor(res.price, res.wallet.Balance)
	}
	fresh, err := c.chat.GetSession(ctx, sessionID)
	if err != nil {
		return nil, mapRepoErr(err, "Session")
	}
	out := &HoldContinued{Session: c.sessionView(ctx, c.newParticipants(), fresh, viewer{ID: clientID}), Wallet: walletView(res.wallet)}
	if fresh.LastHoldID != nil {
		if h, err := c.chat.GetHold(ctx, *fresh.LastHoldID); err == nil {
			out.Hold = holdView(h, viewer{ID: clientID})
		}
	}
	return out, nil
}

// HoldContinued is the answer of POST /chat/sessions/:id/hold/continue.
type HoldContinued struct {
	Session SessionView `json:"session"`
	Hold    HoldView    `json:"hold"`
	Wallet  WalletView  `json:"wallet"`
}

// convertHold converts one held reading under the session lock: one minute is
// charged exactly like a billed minute, the hold is resolved, its messages are
// revealed and the reading continues (or waits for a party to come back).
func (c *ChatService) convertHold(ctx context.Context, sessionID uuid.UUID) (convertResult, error) {
	var post afterCommit
	var res convertResult
	err := c.tx.WithinTx(ctx, func(ctx context.Context) error {
		post, res = nil, convertResult{}
		s, err := c.chat.GetSessionForUpdate(ctx, sessionID)
		if err != nil {
			return err
		}
		if s.Status != domain.SessionHeld {
			res.state = notHeldAnymore
			return nil
		}
		hold, err := c.chat.OpenHoldForSessionForUpdate(ctx, s.ID)
		if err != nil {
			return err
		}
		mc, err := c.chargeMinuteLocked(ctx, s)
		if errors.Is(err, ErrInsufficientFunds) {
			res.state, res.wallet, res.price = notEnoughCredits, mc.wallet, s.RatePerMinute
			return nil // nothing was written
		}
		if err != nil {
			return err
		}
		res.state, res.wallet = convertedHold, mc.wallet
		now := c.clock.Now()
		hold.Status, hold.ResolvedAt, hold.ConvertedCredits = domain.HoldConverted, &now, mc.charged
		if err := c.chat.SaveHold(ctx, hold); err != nil {
			return err
		}
		if _, err := c.chat.UnlockHoldMessages(ctx, hold); err != nil {
			return err
		}
		clientIn, psychicIn, err := c.bothPresent(ctx, s)
		if err != nil {
			return err
		}
		var next *time.Time
		s.HoldExpiresAt = nil
		if clientIn && psychicIn {
			n := now.Add(c.cfg.BillingInterval)
			s.Status, s.LastResumedAt, next = domain.SessionActive, &now, &n
		} else {
			// Someone left while the reading was held: the paid minute's
			// coverage is kept for the resume, the grace period starts now.
			party := "both"
			switch {
			case clientIn:
				party = "psychic"
			case psychicIn:
				party = "client"
			}
			grace := now.Add(c.cfg.GracePeriod)
			s.Status, s.DisconnectedAt, s.DisconnectedParty, s.GraceExpiresAt = domain.SessionDisconnected, &now, party, &grace
			s.LastResumedAt = nil
		}
		// The events read the session as it is once the minute is booked (the
		// snapshot is taken below, before the transaction commits and they run).
		var snap domain.Session
		h := *hold
		post.add(func(ctx context.Context) {
			c.publishSessionHold(ctx, realtime.EventSessionHoldEnded, &snap, &h)
			c.announceRevealed(ctx, &snap, &h)
			c.track(ctx, "chat.hold_converted", &snap, map[string]any{"hold_id": h.ID, "free": mc.free, "credits": mc.charged,
				"held_messages": h.HeldMessages, "held_s": int64(now.Sub(h.StartedAt) / time.Second)})
		})
		if err := c.finishMinuteLocked(ctx, s, mc, next, &post); err != nil {
			return err
		}
		snap = *s
		post.add(func(ctx context.Context) {
			if snap.Status == domain.SessionActive {
				c.publishSession(ctx, realtime.EventSessionResumed, &snap, nil)
				return
			}
			c.publishSession(ctx, realtime.EventSessionDisconnected, &snap, map[string]any{"party": snap.DisconnectedParty, "grace_expires_at": snap.GraceExpiresAt})
			c.track(ctx, "chat.disconnected", &snap, map[string]any{"party": snap.DisconnectedParty})
		})
		return nil
	})
	if err != nil {
		return res, err
	}
	post.run(ctx)
	return res, nil
}

// announceRevealed tells both parties that the messages of hold h are visible
// now: the client gets them in full, the advisor the ids (messages.revealed).
func (c *ChatService) announceRevealed(ctx context.Context, s *domain.Session, h *domain.ChatHold) {
	rows, err := c.chat.ListHoldMessages(ctx, h)
	if err != nil {
		warnErr(ctx, "chat: load revealed messages", err)
		return
	}
	views := c.holdMessageViews(ctx, rows, viewer{ID: s.ClientID})
	ids := make([]uuid.UUID, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	warnErr(ctx, "chat: publish revealed", c.events.Publish(ctx, realtime.EventMessagesRevealed,
		map[string]any{"conversation_id": s.ConversationID, "session_id": s.ID, "hold_id": h.ID, "messages": views}, s.ClientID))
	warnErr(ctx, "chat: publish revealed", c.events.Publish(ctx, realtime.EventMessagesRevealed,
		map[string]any{"conversation_id": s.ConversationID, "session_id": s.ID, "hold_id": h.ID, "message_ids": ids}, s.PsychicID))
}

// holdMessageViews renders the messages a hold covered for w (visible now).
func (c *ChatService) holdMessageViews(ctx context.Context, rows []domain.Message, w viewer) []MessageView {
	out := make([]MessageView, len(rows))
	for i := range rows {
		out[i] = messageView(&rows[i])
	}
	out = c.withAttachments(ctx, out)
	c.decorateBestEffort(ctx, out, true)
	redactMessages(out, w)
	return out
}

// ---------------------------------------------------------------------------
// Expiry (worker)
// ---------------------------------------------------------------------------

// ExpireHolds ends held readings whose hold ran out. While a credit purchase
// the client started during the hold is still processing, the hold is extended
// by HoldExtension at a time, up to the payment grace; otherwise the reading
// ends with hold_expired. Readings are claimed with SKIP LOCKED, so several
// workers can run.
func (c *ChatService) ExpireHolds(ctx context.Context, batch int) (int, error) {
	count := 0
	for count < batch {
		var post afterCommit
		found := false
		err := c.tx.WithinTx(ctx, func(ctx context.Context) error {
			post, found = nil, false
			now := c.clock.Now()
			rows, err := c.chat.ClaimExpiredHolds(ctx, now, 1)
			if err != nil || len(rows) == 0 {
				return err
			}
			found = true
			return c.expireOrExtendLocked(ctx, &rows[0], now, &post)
		})
		if err != nil {
			return count, err
		}
		if !found {
			break
		}
		count++
		post.run(ctx)
	}
	return count, nil
}

func (c *ChatService) expireOrExtendLocked(ctx context.Context, s *domain.Session, now time.Time, post *afterCommit) error {
	h, err := c.chat.OpenHoldForSessionForUpdate(ctx, s.ID)
	if err != nil && !isNotFound(err) {
		return err
	}
	if h != nil && now.Before(h.MaxExpiresAt) {
		pending, err := c.chat.HasPendingPurchaseSince(ctx, s.ClientID, h.StartedAt)
		if err != nil {
			return err
		}
		if pending {
			next := now.Add(domain.HoldExtension)
			if next.After(h.MaxExpiresAt) {
				next = h.MaxExpiresAt
			}
			h.ExpiresAt, s.HoldExpiresAt = next, &next
			if err := c.chat.SaveHold(ctx, h); err != nil {
				return err
			}
			if err := c.chat.SaveSession(ctx, s); err != nil {
				return err
			}
			sessionID, holdID, client, psychic := s.ID, h.ID, s.ClientID, s.PsychicID
			post.add(func(ctx context.Context) {
				warnErr(ctx, "chat: publish hold extended", c.events.Publish(ctx, realtime.EventSessionHoldExtended,
					map[string]any{"session_id": sessionID, "hold_id": holdID, "expires_at": next}, client, psychic))
			})
			return nil
		}
	}
	return c.endLocked(ctx, s, domain.EndHoldExpired, now, post)
}

// expireHoldLocked resolves the open hold of a session that is ending (called
// by endLocked, whatever the end reason): the hold becomes expired and its
// replies stay hidden until revealed.
func (c *ChatService) expireHoldLocked(ctx context.Context, s *domain.Session, now time.Time, post *afterCommit) error {
	h, err := c.chat.OpenHoldForSessionForUpdate(ctx, s.ID)
	if isNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	h.Status, h.ResolvedAt = domain.HoldExpired, &now
	if err := c.chat.SaveHold(ctx, h); err != nil {
		return err
	}
	snap, hold := *s, *h
	post.add(func(ctx context.Context) {
		c.publishSessionHold(ctx, realtime.EventSessionHoldEnded, &snap, &hold)
		c.track(ctx, "chat.hold_expired", &snap, map[string]any{"hold_id": hold.ID, "held_messages": hold.HeldMessages,
			"held_s": int64(now.Sub(hold.StartedAt) / time.Second), "end_reason": string(snap.EndReason)})
	})
	return nil
}

// ---------------------------------------------------------------------------
// Reveal (after expiry)
// ---------------------------------------------------------------------------

// HoldRevealed is the answer of POST /chat/holds/:id/reveal.
type HoldRevealed struct {
	Hold     HoldView      `json:"hold"`
	Messages []MessageView `json:"messages"`
	Wallet   WalletView    `json:"wallet"`
}

// GetHold is GET /chat/holds/:id: a participant's view of a hold.
func (c *ChatService) GetHold(ctx context.Context, p reqctx.Principal, id uuid.UUID) (*HoldView, error) {
	h, err := c.chat.GetHold(ctx, id)
	if err != nil || (h.ClientID != p.UserID && h.PsychicID != p.UserID) {
		return nil, mapRepoErr(repository.ErrNotFound, "Hold")
	}
	v := holdView(h, viewer{ID: p.UserID})
	return &v, nil
}

// RevealHold is POST /chat/holds/:id/reveal: after the reading ended without
// a top-up, the client pays one minute at the reading's rate to read the
// replies it hid. Charged once whatever the retries (the ledger key is the
// hold's), and the advisor earns their share of that minute at the list rate.
func (c *ChatService) RevealHold(ctx context.Context, clientID, holdID uuid.UUID, key string) (*HoldRevealed, error) {
	if !ValidIdempotencyKey(key) {
		return nil, apperror.Validation(map[string]string{"idempotency_key": "A valid idempotency key is required."})
	}
	var hold *domain.ChatHold
	var wallet *domain.Wallet
	var revealed bool
	err := c.tx.WithinTx(ctx, func(ctx context.Context) error {
		hold, wallet, revealed = nil, nil, false
		h, err := c.chat.GetHoldForUpdate(ctx, holdID)
		if err != nil {
			return err
		}
		if h.ClientID != clientID {
			return repository.ErrNotFound // someone else's hold is "not found"
		}
		hold = h
		switch h.Status {
		case domain.HoldOpen:
			return apperror.InvalidState("These hidden messages can only be revealed after the reading ended.")
		case domain.HoldConverted, domain.HoldRevealed:
			return nil // a replay, or nothing was hidden: the current state
		}
		if h.HeldMessages == 0 {
			// The advisor wrote nothing during the hold: there is nothing to pay for.
			return apperror.InvalidState("There are no hidden messages to reveal.")
		}
		res, err := c.wallet.Execute(ctx, Movement{UserID: clientID, Type: domain.LedgerChatReveal, Amount: -h.PriceCredits,
			ReferenceType: "chat_hold", ReferenceID: &h.ID, IdempotencyKey: "chat-hold-reveal:" + h.ID.String(),
			Description: "Replies read after a reading ended"})
		if errors.Is(err, ErrInsufficientFunds) {
			return apperror.InsufficientCreditsFor(h.PriceCredits, res.Wallet.Balance)
		}
		if err != nil {
			return err
		}
		wallet = res.Wallet
		now := c.clock.Now()
		h.Status, h.RevealedAt, h.RevealCredits, h.RevealRevenueMillicents = domain.HoldRevealed, &now, h.PriceCredits, res.ValueMillicents
		if err := c.chat.SaveHold(ctx, h); err != nil {
			return err
		}
		if _, err := c.chat.UnlockHoldMessages(ctx, h); err != nil {
			return err
		}
		net, commission := domain.SplitEarnings(h.ListRateCredits, c.cfg.SharePercent)
		if err := c.earnings.Create(ctx, &domain.Earning{ID: domain.NewID(), PsychicID: h.PsychicID, SourceType: domain.EarningChatHoldReveal,
			SourceID: h.ID, EntryKind: domain.EarningKindEarning, GrossCredits: h.ListRateCredits, CommissionCredits: commission,
			NetCredits: net, SharePercent: c.cfg.SharePercent, CreatedAt: now}); err != nil && !errors.Is(err, repository.ErrDuplicate) {
			return err
		}
		revealed = true
		return nil
	})
	if err != nil {
		return nil, mapRepoErr(err, "Hold")
	}
	rows, err := c.chat.ListHoldMessages(ctx, hold)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if wallet == nil {
		w, err := c.wallets.Get(ctx, clientID)
		if err != nil {
			return nil, mapRepoErr(err, "Wallet")
		}
		wallet = w
	}
	out := &HoldRevealed{Hold: holdView(hold, viewer{ID: clientID}), Messages: c.holdMessageViews(ctx, rows, viewer{ID: clientID}),
		Wallet: walletView(wallet)}
	if revealed {
		c.afterReveal(ctx, hold, wallet)
	}
	return out, nil
}

// afterReveal publishes and tracks a reveal that was just committed.
func (c *ChatService) afterReveal(ctx context.Context, h *domain.ChatHold, wallet *domain.Wallet) {
	ctx = reqctx.WithPrincipal(database.Detach(ctx), reqctx.Principal{UserID: h.ClientID, Role: domain.RoleClient})
	if s, err := c.chat.GetSession(ctx, h.SessionID); err == nil {
		c.announceRevealed(ctx, s, h)
		c.track(ctx, "chat.hold_revealed", s, map[string]any{"hold_id": h.ID, "credits": h.RevealCredits, "held_messages": h.HeldMessages})
	} else {
		warnErr(ctx, "chat: load session of a revealed hold", err)
	}
	warnErr(ctx, "chat: publish wallet", c.events.Publish(ctx, realtime.EventWalletUpdated, walletView(wallet), h.ClientID))
}

// CreditsGrantedHook is told, after commit, that credits were added to a
// user's wallet outside a purchase (an administrator's grant, courtesy credits
// answering a refund request): a held reading may be payable now.
type CreditsGrantedHook func(ctx context.Context, userID uuid.UUID)
