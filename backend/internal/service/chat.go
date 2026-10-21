package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/ratelimit"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// ChatSettings are the live chat business parameters.
type ChatSettings struct {
	RequestTimeout    time.Duration // BR-04
	AcceptedTimeout   time.Duration
	GracePeriod       time.Duration // BR-05
	BillingInterval   time.Duration // one billed minute (BR-01/BR-02)
	MaxMessageLength  int
	HistoryPageSize   int
	LowBalanceMinutes int
	SharePercent      int // psychic share of gross credits (BR-08)
	MessageRule       ratelimit.Rule
	TypingRule        ratelimit.Rule
}

// ChatService implements live chat (spec §5, §6, FR-09–FR-12).
type ChatService struct {
	tx           database.TxManager
	chat         repository.ChatRepository
	users        repository.UserRepository
	psychicsRepo repository.PsychicRepository
	psychics     *PsychicService
	wallet       *WalletService
	wallets      repository.WalletRepository
	earnings     repository.EarningRepository
	presence     realtime.Presence
	events       EventPublisher
	tracker      analytics.Tracker
	limiter      ratelimit.Limiter
	clock        clock.Clock
	cfg          ChatSettings
	extras       ChatExtras
}

// NewChatService wires the chat service.
func NewChatService(tx database.TxManager, chat repository.ChatRepository, users repository.UserRepository,
	psychicsRepo repository.PsychicRepository, psychics *PsychicService, wallet *WalletService, wallets repository.WalletRepository,
	earnings repository.EarningRepository, presence realtime.Presence, events EventPublisher, tracker analytics.Tracker,
	limiter ratelimit.Limiter, clk clock.Clock, cfg ChatSettings) *ChatService {
	return &ChatService{tx: tx, chat: chat, users: users, psychicsRepo: psychicsRepo, psychics: psychics, wallet: wallet,
		wallets: wallets, earnings: earnings, presence: presence, events: events, tracker: tracker, limiter: limiter, clock: clk, cfg: cfg}
}

// System message bodies are stable codes the frontend localises.
const (
	sysSessionStarted = "session.started"
	sysSessionEnded   = "session.ended"
)

// track records a chat analytics event attributed to the session's client.
func (c *ChatService) track(ctx context.Context, name string, s *domain.Session, props map[string]any) {
	if props == nil {
		props = map[string]any{}
	}
	props["session_id"] = s.ID
	props["psychic_id"] = s.PsychicID
	props["rate"] = s.RatePerMinute
	if _, ok := reqctx.PrincipalFrom(ctx); !ok {
		ctx = reqctx.WithPrincipal(ctx, reqctx.Principal{UserID: s.ClientID, Role: domain.RoleClient})
	}
	c.tracker.Track(ctx, name, props)
}

// ---------------------------------------------------------------------------
// Requesting and answering (sequence steps 1–7)
// ---------------------------------------------------------------------------

// RequestSession starts a live chat request from a client to a psychic
// (FR-09). requestKey makes retries idempotent.
func (c *ChatService) RequestSession(ctx context.Context, clientID uuid.UUID, psychicSlug, requestKey string) (*SessionView, error) {
	if !ValidIdempotencyKey(requestKey) {
		return nil, apperror.Validation(map[string]string{"request_key": "A valid request key is required."})
	}
	if existing, err := c.chat.GetSessionByRequestKey(ctx, clientID, requestKey); err == nil {
		v := c.sessionView(ctx, c.newParticipants(), existing, viewer{ID: clientID})
		return &v, nil
	} else if !isNotFound(err) {
		return nil, apperror.Internal(err)
	}
	psychic, err := c.psychicsRepo.GetBySlug(ctx, psychicSlug)
	if err != nil || !psychic.Listed {
		return nil, apperror.NotFound("Advisor")
	}
	if psychic.UserID == clientID {
		return nil, apperror.Forbidden("You cannot start a chat with yourself.")
	}
	if open, err := c.chat.OpenSessionForClient(ctx, clientID); err == nil {
		return nil, apperror.Conflict(apperror.CodeSessionOpen, "You already have a live chat in progress.").
			WithDetails(map[string]any{"session_id": open.ID, "status": open.Status})
	} else if !isNotFound(err) {
		return nil, apperror.Internal(err)
	}
	status, _, err := c.psychics.Status(ctx, psychic.UserID)
	if err != nil {
		return nil, err
	}
	if status != domain.PresenceOnline {
		return nil, apperror.Conflict(apperror.CodePsychicUnavailable, unavailableMessage(status)).
			WithDetails(map[string]any{"status": status})
	}
	w, err := c.wallets.Get(ctx, clientID)
	if err != nil {
		return nil, mapRepoErr(err, "Wallet")
	}
	if w.FreeMinutes <= 0 && w.Balance < psychic.RatePerMinute {
		return nil, apperror.InsufficientCredits(psychic.RatePerMinute, w.Balance)
	}

	now := c.clock.Now()
	s := &domain.Session{
		ID: domain.NewID(), ClientID: clientID, PsychicID: psychic.UserID, Status: domain.SessionRequested,
		RatePerMinute: psychic.RatePerMinute, RequestKey: requestKey, RequestedAt: now,
		ExpiresAt: now.Add(c.cfg.RequestTimeout), CreatedAt: now, UpdatedAt: now,
	}
	err = c.tx.WithinTx(ctx, func(ctx context.Context) error {
		conv, err := c.chat.GetOrCreateConversation(ctx, clientID, psychic.UserID)
		if err != nil {
			return err
		}
		s.ConversationID = conv.ID
		return c.chat.CreateSession(ctx, s)
	})
	switch {
	case errors.Is(err, repository.ErrDuplicate):
		existing, err2 := c.chat.GetSessionByRequestKey(ctx, clientID, requestKey)
		if err2 != nil {
			return nil, apperror.Internal(err2)
		}
		s = existing
	case errors.Is(err, repository.ErrClientBusy):
		return nil, apperror.Conflict(apperror.CodeSessionOpen, "You already have a live chat in progress.")
	case errors.Is(err, repository.ErrPsychicBusy):
		return nil, apperror.Conflict(apperror.CodePsychicUnavailable, unavailableMessage(domain.PresenceBusy))
	case err != nil:
		return nil, apperror.Internal(err)
	default:
		c.publishSession(ctx, realtime.EventSessionRequested, s, nil)
		c.psychics.PublishStatus(ctx, s.PsychicID)
		c.track(ctx, "chat.requested", s, nil)
		client := c.newParticipants().client(ctx, clientID)
		c.notifier().Notify(ctx, NotificationSpec{Kind: NotifyChatRequested,
			Data: map[string]any{"session_id": s.ID, "client_name": client.Name}, Link: "/chatroom"}, s.PsychicID)
	}
	v := c.sessionView(ctx, c.newParticipants(), s, viewer{ID: clientID})
	return &v, nil
}

func unavailableMessage(st domain.PresenceStatus) string {
	switch st {
	case domain.PresenceBusy:
		return "This advisor is in another reading right now."
	case domain.PresenceAway:
		return "This advisor is away right now."
	}
	return "This advisor is offline right now."
}

// AcceptSession is the psychic accepting a request (sequence step 5).
func (c *ChatService) AcceptSession(ctx context.Context, psychicID, sessionID uuid.UUID) (*SessionView, error) {
	now := c.clock.Now()
	var s *domain.Session
	var accepted, expired bool
	err := c.tx.WithinTx(ctx, func(ctx context.Context) error {
		accepted, expired = false, false
		var err error
		s, err = c.chat.GetSessionForUpdate(ctx, sessionID)
		if err != nil {
			return err
		}
		if s.PsychicID != psychicID {
			return repository.ErrNotFound
		}
		if s.Status != domain.SessionRequested {
			return nil // idempotent: already answered
		}
		if !now.Before(s.ExpiresAt) {
			s.Status, s.EndReason, s.EndedAt = domain.SessionExpired, domain.EndRequestTimeout, &now
			expired = true
			return c.chat.SaveSession(ctx, s)
		}
		s.Status, s.AcceptedAt, s.ExpiresAt = domain.SessionAccepted, &now, now.Add(c.cfg.AcceptedTimeout)
		accepted = true
		return c.chat.SaveSession(ctx, s)
	})
	if err != nil {
		return nil, mapRepoErr(err, "Session")
	}
	if expired {
		c.publishSession(ctx, realtime.EventSessionExpired, s, nil)
		c.psychics.PublishStatus(ctx, s.PsychicID)
		c.track(ctx, "chat.expired", s, map[string]any{"reason": string(domain.EndRequestTimeout)})
		return nil, apperror.InvalidState("This request has expired.")
	}
	if accepted {
		c.publishSession(ctx, realtime.EventSessionAccepted, s, nil)
		c.track(ctx, "chat.accepted", s, map[string]any{"response_ms": now.Sub(s.RequestedAt).Milliseconds()})
		c.tryActivate(ctx, s.ID)
		if fresh, err := c.chat.GetSession(ctx, s.ID); err == nil {
			s = fresh
		}
	} else if s.Status == domain.SessionDeclined || s.Status == domain.SessionExpired || s.Status == domain.SessionCancelled {
		return nil, apperror.InvalidState("This request can no longer be accepted.").WithDetails(map[string]any{"status": s.Status})
	}
	v := c.sessionView(ctx, c.newParticipants(), s, viewer{ID: psychicID})
	return &v, nil
}

// DeclineSession is the psychic declining a request (BR-04).
func (c *ChatService) DeclineSession(ctx context.Context, psychicID, sessionID uuid.UUID) (*SessionView, error) {
	return c.closeRequest(ctx, sessionID, func(s *domain.Session) error {
		if s.PsychicID != psychicID {
			return repository.ErrNotFound
		}
		return nil
	}, domain.SessionDeclined, domain.EndDeclined, realtime.EventSessionDeclined, "chat.declined", psychicID)
}

// CancelSession is the client withdrawing a pending request.
func (c *ChatService) CancelSession(ctx context.Context, clientID, sessionID uuid.UUID) (*SessionView, error) {
	return c.closeRequest(ctx, sessionID, func(s *domain.Session) error {
		if s.ClientID != clientID {
			return repository.ErrNotFound
		}
		return nil
	}, domain.SessionCancelled, domain.EndCancelled, realtime.EventSessionCancelled, "chat.cancelled", clientID)
}

func (c *ChatService) closeRequest(ctx context.Context, sessionID uuid.UUID, authorize func(*domain.Session) error,
	to domain.SessionStatus, reason domain.EndReason, event, metric string, viewerID uuid.UUID) (*SessionView, error) {
	now := c.clock.Now()
	var s *domain.Session
	changed := false
	err := c.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if s, err = c.chat.GetSessionForUpdate(ctx, sessionID); err != nil {
			return err
		}
		if err := authorize(s); err != nil {
			return err
		}
		if s.Status == to {
			return nil
		}
		if s.Status != domain.SessionRequested {
			return apperror.InvalidState("This request has already been answered.").WithDetails(map[string]any{"status": s.Status})
		}
		s.Status, s.EndReason, s.EndedAt = to, reason, &now
		changed = true
		return c.chat.SaveSession(ctx, s)
	})
	if err != nil {
		return nil, mapRepoErr(err, "Session")
	}
	if changed {
		c.publishSession(ctx, event, s, nil)
		c.psychics.PublishStatus(ctx, s.PsychicID)
		c.track(ctx, metric, s, map[string]any{"after_ms": now.Sub(s.RequestedAt).Milliseconds()})
	}
	v := c.sessionView(ctx, c.newParticipants(), s, viewer{ID: viewerID})
	return &v, nil
}

// ---------------------------------------------------------------------------
// Presence-driven transitions: Accepted→Active, Active⇄Disconnected (BR-05)
// ---------------------------------------------------------------------------

// bothPresent reports whether both participants are connected to s.
func (c *ChatService) bothPresent(ctx context.Context, s *domain.Session) (clientIn, psychicIn bool, err error) {
	if clientIn, err = c.presence.InSession(ctx, s.ID, s.ClientID); err != nil {
		return false, false, err
	}
	if psychicIn, err = c.presence.InSession(ctx, s.ID, s.PsychicID); err != nil {
		return false, false, err
	}
	return clientIn, psychicIn, nil
}

// Join marks a connection as present in a session and advances the state
// machine when both parties are connected.
func (c *ChatService) Join(ctx context.Context, p reqctx.Principal, connID string, sessionID uuid.UUID) (*SessionView, error) {
	s, err := c.loadParticipantSession(ctx, p.UserID, false, sessionID)
	if err != nil {
		return nil, err
	}
	if !s.Status.IsOpen() {
		v := c.sessionView(ctx, c.newParticipants(), s, viewer{ID: p.UserID})
		return &v, nil
	}
	if err := c.presence.Join(ctx, connID, p.UserID, sessionID); err != nil {
		return nil, apperror.Unavailable(err)
	}
	warnErr(ctx, "chat: participant event", c.events.Ephemeral(ctx, realtime.EventParticipant,
		map[string]any{"session_id": s.ID, "user_id": p.UserID, "present": true}, s.Counterpart(p.UserID)))
	switch s.Status {
	case domain.SessionAccepted:
		c.tryActivate(ctx, s.ID)
	case domain.SessionDisconnected:
		c.tryResume(ctx, s.ID)
	}
	if fresh, err := c.chat.GetSession(ctx, s.ID); err == nil {
		s = fresh
	}
	v := c.sessionView(ctx, c.newParticipants(), s, viewer{ID: p.UserID})
	return &v, nil
}

// Leave removes a connection from a session (user navigated away).
func (c *ChatService) Leave(ctx context.Context, p reqctx.Principal, connID string, sessionID uuid.UUID) error {
	s, err := c.loadParticipantSession(ctx, p.UserID, false, sessionID)
	if err != nil {
		return err
	}
	if err := c.presence.Leave(ctx, connID, p.UserID, sessionID); err != nil {
		return apperror.Unavailable(err)
	}
	c.handlePresenceLoss(ctx, s.ID, p.UserID)
	return nil
}

// ConnectionClosed handles a dropped realtime connection.
func (c *ChatService) ConnectionClosed(ctx context.Context, userID uuid.UUID, sessions []uuid.UUID) {
	for _, sid := range sessions {
		c.handlePresenceLoss(ctx, sid, userID)
	}
}

// tryActivate moves Accepted→Active when both parties are connected and
// charges the first started minute immediately (BR-02).
func (c *ChatService) tryActivate(ctx context.Context, sessionID uuid.UUID) {
	var post afterCommit
	err := c.tx.WithinTx(ctx, func(ctx context.Context) error {
		post = nil
		s, err := c.chat.GetSessionForUpdate(ctx, sessionID)
		if err != nil {
			return err
		}
		if s.Status != domain.SessionAccepted {
			return nil
		}
		clientIn, psychicIn, err := c.bothPresent(ctx, s)
		if err != nil || !clientIn || !psychicIn {
			return err
		}
		now := c.clock.Now()
		s.Status, s.StartedAt, s.LastResumedAt, s.ActiveSeconds = domain.SessionActive, &now, &now, 0
		if err := c.addSystemMessage(ctx, s, sysSessionStarted, now, &post); err != nil {
			return err
		}
		if err := c.billMinuteLocked(ctx, s, now, now, &post); err != nil {
			return err
		}
		snap := *s
		// "started" must reach clients before the billing/ended events.
		post = append(afterCommit{func(ctx context.Context) {
			c.publishSession(ctx, realtime.EventSessionStarted, &snap, nil)
			c.track(ctx, "chat.started", &snap, nil)
		}}, post...)
		return nil
	})
	if err != nil {
		logger.FromContext(ctx).Error("chat: activation failed", zap.Error(err), zap.String("session_id", sessionID.String()))
		return
	}
	post.run(ctx)
}

// handlePresenceLoss moves Active→Disconnected when a participant has no
// live connection left, opening the reconnection grace period.
func (c *ChatService) handlePresenceLoss(ctx context.Context, sessionID, leaver uuid.UUID) {
	s, err := c.chat.GetSession(ctx, sessionID)
	if err != nil {
		return
	}
	if s.Status.IsOpen() {
		warnErr(ctx, "chat: participant event", c.events.Ephemeral(ctx, realtime.EventParticipant,
			map[string]any{"session_id": s.ID, "user_id": leaver, "present": false}, s.Counterpart(leaver)))
	}
	if s.Status != domain.SessionActive {
		return
	}
	var post afterCommit
	err = c.tx.WithinTx(ctx, func(ctx context.Context) error {
		post = nil
		s, err := c.chat.GetSessionForUpdate(ctx, sessionID)
		if err != nil || s.Status != domain.SessionActive {
			return err
		}
		clientIn, psychicIn, err := c.bothPresent(ctx, s)
		if err != nil || (clientIn && psychicIn) {
			return err
		}
		now := c.clock.Now()
		c.accrueActive(s, now)
		party := "both"
		switch {
		case clientIn:
			party = "psychic"
		case psychicIn:
			party = "client"
		}
		grace := now.Add(c.cfg.GracePeriod)
		s.Status, s.DisconnectedAt, s.DisconnectedParty, s.GraceExpiresAt = domain.SessionDisconnected, &now, party, &grace
		s.NextBillingAt, s.LastResumedAt = nil, nil
		if err := c.chat.SaveSession(ctx, s); err != nil {
			return err
		}
		post.add(func(ctx context.Context) {
			c.publishSession(ctx, realtime.EventSessionDisconnected, s, map[string]any{"party": party, "grace_expires_at": grace})
			c.track(ctx, "chat.disconnected", s, map[string]any{"party": party})
		})
		return nil
	})
	if err != nil {
		logger.FromContext(ctx).Error("chat: disconnect transition failed", zap.Error(err), zap.String("session_id", sessionID.String()))
		return
	}
	post.run(ctx)
}

// tryResume moves Disconnected→Active when both parties are back within the
// grace period. Billing resumes where the paid minute left off.
func (c *ChatService) tryResume(ctx context.Context, sessionID uuid.UUID) {
	var post afterCommit
	err := c.tx.WithinTx(ctx, func(ctx context.Context) error {
		post = nil
		s, err := c.chat.GetSessionForUpdate(ctx, sessionID)
		if err != nil || s.Status != domain.SessionDisconnected {
			return err
		}
		now := c.clock.Now()
		if s.GraceExpiresAt != nil && !now.Before(*s.GraceExpiresAt) {
			return nil // the worker ends it (grace expired)
		}
		clientIn, psychicIn, err := c.bothPresent(ctx, s)
		if err != nil || !clientIn || !psychicIn {
			return err
		}
		s.Status, s.LastResumedAt, s.DisconnectedAt, s.GraceExpiresAt, s.DisconnectedParty = domain.SessionActive, &now, nil, nil, ""
		s.Reconnects++
		covered := s.PaidCoverage(c.cfg.BillingInterval) - time.Duration(s.ActiveSeconds)*time.Second
		if covered > 0 {
			next := now.Add(covered)
			s.NextBillingAt = &next
			if err := c.chat.SaveSession(ctx, s); err != nil {
				return err
			}
		} else if err := c.billMinuteLocked(ctx, s, now, now, &post); err != nil {
			return err
		}
		snap := *s
		post = append(afterCommit{func(ctx context.Context) {
			c.publishSession(ctx, realtime.EventSessionResumed, &snap, nil)
			c.track(ctx, "chat.reconnected", &snap, map[string]any{"reconnects": snap.Reconnects})
		}}, post...)
		return nil
	})
	if err != nil {
		logger.FromContext(ctx).Error("chat: resume failed", zap.Error(err), zap.String("session_id", sessionID.String()))
		return
	}
	post.run(ctx)
}

// accrueActive adds the current active stretch to ActiveSeconds.
func (c *ChatService) accrueActive(s *domain.Session, now time.Time) {
	if s.Status == domain.SessionActive && s.LastResumedAt != nil && now.After(*s.LastResumedAt) {
		s.ActiveSeconds += int64(now.Sub(*s.LastResumedAt) / time.Second)
	}
	// Active time never exceeds what was paid for: billing is per started
	// minute, so the paid coverage always runs ahead of the clock.
	if limit := int64(s.PaidCoverage(c.cfg.BillingInterval) / time.Second); s.ActiveSeconds > limit {
		s.ActiveSeconds = limit
	}
}

// ---------------------------------------------------------------------------
// Billing (sequence loop steps 8–14; BR-01, BR-02, BR-03)
// ---------------------------------------------------------------------------

// billMinuteLocked charges the next started minute for s (locked, Active)
// at time due, saving s. When the client cannot pay, the session ends
// immediately (BR-03). Free promotional minutes are used first.
func (c *ChatService) billMinuteLocked(ctx context.Context, s *domain.Session, due, now time.Time, post *afterCommit) error {
	minute := s.BilledMinutes + 1
	w, free, err := c.wallet.UseFreeMinute(ctx, s.ClientID)
	if err != nil {
		return err
	}
	charged := int64(0)
	if !free {
		w, _, err = c.wallet.Apply(ctx, Movement{
			UserID: s.ClientID, Type: domain.LedgerChatCharge, Amount: -s.RatePerMinute,
			ReferenceType: "chat_session", ReferenceID: &s.ID,
			IdempotencyKey: fmt.Sprintf("chat:%s:%d", s.ID, minute),
			Description:    fmt.Sprintf("Live chat minute %d", minute),
		})
		if errors.Is(err, ErrInsufficientFunds) {
			return c.endLocked(ctx, s, domain.EndCreditsExhausted, now, post)
		}
		if err != nil {
			return err
		}
		charged = s.RatePerMinute
		s.CreditsCharged += charged
	} else {
		s.FreeMinutesUsed++
	}
	s.BilledMinutes = minute
	next := due.Add(c.cfg.BillingInterval)
	s.NextBillingAt = &next
	if err := c.chat.SaveSession(ctx, s); err != nil {
		return err
	}
	snapshot := *s
	wallet := walletView(w)
	post.add(func(ctx context.Context) {
		remaining := int64(wallet.FreeMinutes) + wallet.Balance/max(snapshot.RatePerMinute, 1)
		billed := map[string]any{"session_id": snapshot.ID, "minute": minute, "free": free, "charged": charged,
			"next_billing_at": next, "active_seconds": int64(snapshot.ElapsedActive(c.clock.Now()) / time.Second)}
		clientPayload := map[string]any{"wallet": wallet, "minutes_remaining": remaining}
		for k, v := range billed {
			clientPayload[k] = v
		}
		warnErr(ctx, "chat: publish billed", c.events.Publish(ctx, realtime.EventSessionBilled, clientPayload, snapshot.ClientID))
		warnErr(ctx, "chat: publish billed", c.events.Publish(ctx, realtime.EventSessionBilled, billed, snapshot.PsychicID))
		warnErr(ctx, "chat: publish wallet", c.events.Publish(ctx, realtime.EventWalletUpdated, wallet, snapshot.ClientID))
		if remaining < int64(c.cfg.LowBalanceMinutes) {
			warnErr(ctx, "chat: publish low balance", c.events.Publish(ctx, realtime.EventSessionLowBalance,
				map[string]any{"session_id": snapshot.ID, "minutes_remaining": remaining, "wallet": wallet}, snapshot.ClientID))
		}
		c.track(ctx, "chat.minute_billed", &snapshot, map[string]any{"minute": minute, "free": free, "credits": charged})
	})
	return nil
}

// BillDue charges every Active session whose next minute has started.
// Sessions are claimed with SKIP LOCKED, so several workers can run.
func (c *ChatService) BillDue(ctx context.Context, batch int) (int, error) {
	processed := 0
	for processed < batch {
		var post afterCommit
		found := false
		err := c.tx.WithinTx(ctx, func(ctx context.Context) error {
			post = nil
			now := c.clock.Now()
			rows, err := c.chat.ClaimDueBilling(ctx, now, 1)
			if err != nil || len(rows) == 0 {
				return err
			}
			found = true
			s := &rows[0]
			// Catch up minute by minute if the worker fell behind.
			for s.Status == domain.SessionActive && s.NextBillingAt != nil && !s.NextBillingAt.After(now) {
				if err := c.billMinuteLocked(ctx, s, *s.NextBillingAt, now, &post); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return processed, err
		}
		if !found {
			break
		}
		processed++
		post.run(ctx)
	}
	return processed, nil
}

// ---------------------------------------------------------------------------
// Ending and settlement (sequence steps 15–18; §6 Ended→Settled)
// ---------------------------------------------------------------------------

// endLocked transitions a locked live session to Ended.
func (c *ChatService) endLocked(ctx context.Context, s *domain.Session, reason domain.EndReason, now time.Time, post *afterCommit) error {
	c.accrueActive(s, now)
	s.Status, s.EndReason, s.EndedAt = domain.SessionEnded, reason, &now
	s.NextBillingAt, s.GraceExpiresAt, s.LastResumedAt = nil, nil, nil
	if err := c.chat.SaveSession(ctx, s); err != nil {
		return err
	}
	if s.StartedAt != nil {
		if err := c.addSystemMessage(ctx, s, sysSessionEnded+":"+string(reason), now, post); err != nil {
			return err
		}
	}
	snapshot := *s
	post.add(func(ctx context.Context) {
		c.track(ctx, "chat.ended", &snapshot, map[string]any{"reason": string(reason), "duration_s": snapshot.ActiveSeconds,
			"billed_minutes": snapshot.BilledMinutes, "free_minutes": snapshot.FreeMinutesUsed, "credits": snapshot.CreditsCharged})
		c.settleAndAnnounce(ctx, snapshot.ID, reason)
	})
	return nil
}

// settleAndAnnounce settles an ended session and notifies both parties
// with the summary.
func (c *ChatService) settleAndAnnounce(ctx context.Context, sessionID uuid.UUID, reason domain.EndReason) {
	s, err := c.settle(ctx, sessionID)
	if err != nil {
		logger.FromContext(ctx).Error("chat: settlement failed; worker will retry", zap.Error(err), zap.String("session_id", sessionID.String()))
		if s, err = c.chat.GetSession(ctx, sessionID); err != nil {
			return
		}
	}
	for _, uid := range []uuid.UUID{s.ClientID, s.PsychicID} {
		sum := c.summary(ctx, s, viewer{ID: uid})
		warnErr(ctx, "chat: publish ended", c.events.Publish(ctx, realtime.EventSessionEnded,
			map[string]any{"session": sum.Session, "summary": sum, "reason": reason}, uid))
	}
	if s.Status == domain.SessionSettled {
		warnErr(ctx, "chat: publish settled", c.events.Publish(ctx, realtime.EventSessionSettled,
			map[string]any{"session_id": s.ID, "net_credits": s.PsychicEarnings, "gross_credits": s.CreditsCharged}, s.PsychicID))
	}
	c.psychics.PublishStatus(ctx, s.PsychicID)
	// Settlement retries (reason "") must not prompt twice.
	if reason != "" {
		c.notifyReviewPrompt(ctx, s)
	}
}

// settle computes psychic earnings for an Ended session (BR-08).
func (c *ChatService) settle(ctx context.Context, sessionID uuid.UUID) (*domain.Session, error) {
	var out *domain.Session
	err := c.tx.WithinTx(ctx, func(ctx context.Context) error {
		s, err := c.chat.GetSessionForUpdate(ctx, sessionID)
		if err != nil {
			return err
		}
		out = s
		if s.Status != domain.SessionEnded {
			return nil
		}
		net, commission := domain.SplitEarnings(s.CreditsCharged, c.cfg.SharePercent)
		if s.CreditsCharged > 0 {
			if err := c.earnings.Create(ctx, &domain.Earning{
				ID: domain.NewID(), PsychicID: s.PsychicID, SourceType: domain.EarningChatSession, SourceID: s.ID,
				EntryKind: domain.EarningKindEarning, GrossCredits: s.CreditsCharged, CommissionCredits: commission,
				NetCredits: net, SharePercent: c.cfg.SharePercent, CreatedAt: c.clock.Now(),
			}); err != nil && !errors.Is(err, repository.ErrDuplicate) {
				return err
			}
		}
		if s.BilledMinutes > 0 {
			if err := c.psychicsRepo.IncrementReadings(ctx, s.PsychicID); err != nil {
				return err
			}
		}
		now := c.clock.Now()
		s.Status, s.SettledAt, s.PsychicEarnings = domain.SessionSettled, &now, net
		return c.chat.SaveSession(ctx, s)
	})
	if err != nil {
		return out, err
	}
	if out != nil && out.Status == domain.SessionSettled {
		c.track(ctx, "chat.settled", out, map[string]any{"gross": out.CreditsCharged, "net": out.PsychicEarnings})
	}
	return out, nil
}

// EndSession ends a session at a participant's request (sequence step 15).
// Ending a pending request cancels (client) or declines (psychic) it.
func (c *ChatService) EndSession(ctx context.Context, p reqctx.Principal, sessionID uuid.UUID) (*SessionSummary, error) {
	s, err := c.loadParticipantSession(ctx, p.UserID, p.IsAdmin(), sessionID)
	if err != nil {
		return nil, err
	}
	if s.Status == domain.SessionRequested {
		if p.UserID == s.ClientID {
			if _, err := c.CancelSession(ctx, p.UserID, sessionID); err != nil {
				return nil, err
			}
		} else if p.UserID == s.PsychicID {
			if _, err := c.DeclineSession(ctx, p.UserID, sessionID); err != nil {
				return nil, err
			}
		}
		s, _ = c.chat.GetSession(ctx, sessionID)
		sum := c.summary(ctx, s, viewer{ID: p.UserID, Admin: p.IsAdmin()})
		return &sum, nil
	}
	reason := domain.EndClientEnded
	switch {
	case p.IsAdmin() && !s.IsParticipant(p.UserID):
		reason = domain.EndAdminTerminated
	case p.UserID == s.PsychicID:
		reason = domain.EndPsychicEnded
	}
	return c.end(ctx, sessionID, reason, viewer{ID: p.UserID, Admin: p.IsAdmin()})
}

// end transitions a live session to Ended and settles it.
func (c *ChatService) end(ctx context.Context, sessionID uuid.UUID, reason domain.EndReason, w viewer) (*SessionSummary, error) {
	var post afterCommit
	err := c.tx.WithinTx(ctx, func(ctx context.Context) error {
		post = nil
		s, err := c.chat.GetSessionForUpdate(ctx, sessionID)
		if err != nil {
			return err
		}
		if !s.Status.IsLive() {
			return nil // already ended: idempotent
		}
		now := c.clock.Now()
		// Bill minutes that already started but the worker has not charged
		// yet (BR-02), so ending a chat never skips a started minute.
		for s.Status == domain.SessionActive && s.NextBillingAt != nil && s.NextBillingAt.Before(now) {
			if err := c.billMinuteLocked(ctx, s, *s.NextBillingAt, now, &post); err != nil {
				return err
			}
		}
		if !s.Status.IsLive() {
			return nil // the catch-up ended it (credits exhausted)
		}
		return c.endLocked(ctx, s, reason, now, &post)
	})
	if err != nil {
		return nil, mapRepoErr(err, "Session")
	}
	post.run(ctx)
	s, err := c.chat.GetSession(ctx, sessionID)
	if err != nil {
		return nil, mapRepoErr(err, "Session")
	}
	sum := c.summary(ctx, s, w)
	return &sum, nil
}

// EndAllForUser ends every live session of a user (account deactivation).
func (c *ChatService) EndAllForUser(ctx context.Context, userID uuid.UUID) {
	for _, find := range []func(context.Context, uuid.UUID) (*domain.Session, error){c.chat.OpenSessionForClient, c.chat.OpenSessionForPsychic} {
		s, err := find(ctx, userID)
		if err != nil {
			continue
		}
		if s.Status == domain.SessionRequested {
			if s.ClientID == userID {
				_, _ = c.CancelSession(ctx, userID, s.ID)
			} else {
				_, _ = c.DeclineSession(ctx, userID, s.ID)
			}
			continue
		}
		_, err = c.end(ctx, s.ID, domain.EndAccountDisabled, viewer{Admin: true})
		warnErr(ctx, "chat: end session for deactivated user", err)
	}
}

// ---------------------------------------------------------------------------
// Background sweeps (worker)
// ---------------------------------------------------------------------------

// ExpireDue expires unanswered requests (BR-04) and accepted sessions that
// never started.
func (c *ChatService) ExpireDue(ctx context.Context, batch int) (int, error) {
	var expired []domain.Session
	err := c.tx.WithinTx(ctx, func(ctx context.Context) error {
		now := c.clock.Now()
		rows, err := c.chat.ClaimDueExpiry(ctx, now, batch)
		if err != nil {
			return err
		}
		expired = expired[:0]
		for i := range rows {
			s := &rows[i]
			reason := domain.EndRequestTimeout
			if s.Status == domain.SessionAccepted {
				reason = domain.EndNotStarted
			}
			s.Status, s.EndReason, s.EndedAt = domain.SessionExpired, reason, &now
			if err := c.chat.SaveSession(ctx, s); err != nil {
				return err
			}
			expired = append(expired, *s)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	for i := range expired {
		s := &expired[i]
		c.publishSession(ctx, realtime.EventSessionExpired, s, nil)
		c.psychics.PublishStatus(ctx, s.PsychicID)
		c.track(ctx, "chat.expired", s, map[string]any{"reason": string(s.EndReason)})
		if s.EndReason == domain.EndRequestTimeout {
			client := c.newParticipants().client(ctx, s.ClientID)
			c.notifier().Notify(ctx, NotificationSpec{Kind: NotifyChatMissed,
				Data: map[string]any{"session_id": s.ID, "client_name": client.Name}, Link: "/chatroom"}, s.PsychicID)
		}
	}
	return len(expired), nil
}

// EndExpiredGrace ends Disconnected sessions whose grace period elapsed.
func (c *ChatService) EndExpiredGrace(ctx context.Context, batch int) (int, error) {
	count := 0
	for count < batch {
		var post afterCommit
		found := false
		err := c.tx.WithinTx(ctx, func(ctx context.Context) error {
			post = nil
			now := c.clock.Now()
			rows, err := c.chat.ClaimDueGrace(ctx, now, 1)
			if err != nil || len(rows) == 0 {
				return err
			}
			found = true
			return c.endLocked(ctx, &rows[0], domain.EndGraceExpired, now, &post)
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

// SettlePending settles Ended sessions whose inline settlement failed.
func (c *ChatService) SettlePending(ctx context.Context, batch int) (int, error) {
	var ids []uuid.UUID
	err := c.tx.WithinTx(ctx, func(ctx context.Context) error {
		rows, err := c.chat.ClaimUnsettled(ctx, c.clock.Now().Add(-5*time.Second), batch)
		for _, r := range rows {
			ids = append(ids, r.ID)
		}
		return err
	})
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		c.settleAndAnnounce(ctx, id, "")
	}
	return len(ids), nil
}

// SweepPresence reconciles live sessions with presence, catching
// connections that vanished without a close (e.g. a crashed instance).
func (c *ChatService) SweepPresence(ctx context.Context, limit int) error {
	sessions, err := c.chat.ListLiveSessions(ctx, limit)
	if err != nil {
		return err
	}
	for i := range sessions {
		s := &sessions[i]
		clientIn, psychicIn, err := c.bothPresent(ctx, s)
		if err != nil {
			return err
		}
		switch {
		case s.Status == domain.SessionAccepted && clientIn && psychicIn:
			c.tryActivate(ctx, s.ID)
		case s.Status == domain.SessionActive && (!clientIn || !psychicIn):
			leaver := s.ClientID
			if clientIn {
				leaver = s.PsychicID
			}
			c.handlePresenceLoss(ctx, s.ID, leaver)
		}
	}
	return nil
}

// addSystemMessage records a system notice in the transcript.
func (c *ChatService) addSystemMessage(ctx context.Context, s *domain.Session, body string, at time.Time, post *afterCommit) error {
	m := &domain.Message{ID: domain.NewID(), ConversationID: s.ConversationID, SessionID: s.ID, Kind: domain.MessageSystem, Body: body, CreatedAt: at}
	if _, err := c.chat.CreateMessage(ctx, m); err != nil {
		return err
	}
	if err := c.chat.TouchConversation(ctx, s.ConversationID, m.ID, at); err != nil {
		return err
	}
	view := messageView(m)
	client, psychic := s.ClientID, s.PsychicID
	post.add(func(ctx context.Context) {
		warnErr(ctx, "chat: publish system message", c.events.Publish(ctx, realtime.EventMessageCreated, map[string]any{"message": view}, client, psychic))
	})
	return nil
}
