package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/i18n"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// EmailRequest asks for one transactional e-mail.
type EmailRequest struct {
	// UserID is the account the e-mail concerns (nil for addresses not yet
	// attached to an account, e.g. a new address awaiting confirmation).
	UserID   *uuid.UUID
	To       string
	Locale   string
	Category domain.EmailCategory
	// Template names the message for records and metrics ("password_reset",
	// "notification:chat.missed").
	Template string
	Content  mail.Content
	// DedupeKey makes the enqueue idempotent (e.g. "notification:<id>").
	DedupeKey string
}

// EmailQueue enqueues transactional e-mail (implemented by MailService).
type EmailQueue interface {
	Enqueue(ctx context.Context, reqs ...EmailRequest) error
}

// MailSettings tune the outbox.
type MailSettings struct {
	FrontendURL string
	// MaxAttempts bounds delivery tries before a row is dead.
	MaxAttempts int
	// DailyCap bounds e-mails per user per 24h (security e-mail is exempt).
	DailyCap int
	// AllowMarketing must stay false while the provider is
	// transactional-only (Cloudflare Email Service).
	AllowMarketing bool
	// Lease is how long a claimed row is reserved for one delivery attempt.
	Lease time.Duration
	// BatchSize rows are claimed per round.
	BatchSize int
	// BaseBackoff doubles per attempt (±20% jitter) up to MaxBackoff.
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	// SendTimeout bounds one provider call.
	SendTimeout time.Duration
	// HashKey keys the HMAC of addresses (to_hash, suppressions).
	HashKey string
	// SupportEmail is printed in the footer of every e-mail (revisions 5.1,
	// STARYIELD_MAIL_SUPPORT_INBOX; empty: the built-in support address).
	SupportEmail string
}

func (s *MailSettings) defaults() {
	if s.MaxAttempts <= 0 {
		s.MaxAttempts = 8
	}
	if s.DailyCap <= 0 {
		s.DailyCap = 20
	}
	if s.Lease <= 0 {
		s.Lease = 2 * time.Minute
	}
	if s.BatchSize <= 0 {
		s.BatchSize = 50
	}
	if s.BaseBackoff <= 0 {
		s.BaseBackoff = 30 * time.Second
	}
	if s.MaxBackoff <= 0 {
		s.MaxBackoff = time.Hour
	}
	if s.SendTimeout <= 0 {
		s.SendTimeout = 30 * time.Second
	}
}

// Outbox status reasons (email_outbox.status_reason).
const (
	reasonNoMarketingProvider = "no_marketing_provider"
	reasonDailyCap            = "daily_cap"
	reasonSuppressed          = "recipient_suppressed"
	reasonBounced             = "permanent_bounce"
	reasonMaxAttempts         = "max_attempts"
	reasonRejected            = "rejected"
	reasonUnreadable          = "unreadable"
	reasonInvalidRecipient    = "invalid_recipient"
)

// MailService is the e-mail outbox: Enqueue renders and stores messages
// (sealed) in the caller's transaction; DeliverDue, run by the worker,
// delivers them through the provider with retries, backoff and suppression.
type MailService struct {
	tx       database.TxManager
	repo     repository.EmailRepository
	provider mail.Mailer
	sealer   *security.Sealer
	clock    clock.Clock
	log      *zap.Logger
	cfg      MailSettings
	wake     chan struct{}
	jitter   func() float64
}

// NewMailService wires the outbox.
func NewMailService(tx database.TxManager, repo repository.EmailRepository, provider mail.Mailer, sealer *security.Sealer,
	clk clock.Clock, log *zap.Logger, cfg MailSettings) *MailService {
	cfg.defaults()
	return &MailService{tx: tx, repo: repo, provider: provider, sealer: sealer, clock: clk, log: log, cfg: cfg,
		wake: make(chan struct{}, 1), jitter: rand.Float64}
}

// Wake returns a channel signalled when new e-mail was enqueued, so the
// delivery loop can run without waiting for its next tick.
func (s *MailService) Wake() <-chan struct{} { return s.wake }

// AddressHash returns the keyed hash identifying an address in the outbox
// and suppression list.
func (s *MailService) AddressHash(email string) string {
	return security.Keyed(s.cfg.HashKey, strings.ToLower(strings.TrimSpace(email)), 0)
}

type emailPayload struct {
	Subject string            `json:"s"`
	Text    string            `json:"t"`
	HTML    string            `json:"h"`
	Headers map[string]string `json:"x,omitempty"`
}

func outboxAD(id uuid.UUID) string { return "email_outbox:" + id.String() }

// Enqueue implements EmailQueue. Invalid requests are logged and skipped:
// e-mail is a side effect and never fails the caller's operation.
func (s *MailService) Enqueue(ctx context.Context, reqs ...EmailRequest) error {
	if len(reqs) == 0 {
		return nil
	}
	now := s.clock.Now()
	rows := make([]domain.EmailOutbox, 0, len(reqs))
	hashes := make([]string, 0, len(reqs))
	type prepared struct {
		req   EmailRequest
		to    string
		hash  string
		valid bool
	}
	prep := make([]prepared, 0, len(reqs))
	for _, r := range reqs {
		to, ok := NormalizeEmail(r.To)
		p := prepared{req: r, to: to, valid: ok && r.Category.Valid() && r.Template != "" && len(r.Template) <= 64 && len(r.DedupeKey) <= 128}
		if p.valid {
			p.hash = s.AddressHash(to)
			hashes = append(hashes, p.hash)
		} else {
			logger.FromContext(ctx).Warn("mail: invalid e-mail request skipped", zap.String("template", r.Template),
				zap.String("category", string(r.Category)))
		}
		prep = append(prep, p)
	}
	suppressed, err := s.repo.Suppressed(ctx, hashes, now)
	if err != nil {
		return fmt.Errorf("mail: suppression lookup: %w", err)
	}
	capped := map[uuid.UUID]int64{}
	for _, p := range prep {
		if !p.valid {
			continue
		}
		r := p.req
		row := domain.EmailOutbox{ID: domain.NewID(), UserID: r.UserID, Category: r.Category, Template: r.Template,
			Locale: i18n.Normalize(r.Locale), ToHash: p.hash, DedupeKey: r.DedupeKey, Status: domain.EmailQueued,
			NextAttemptAt: now, Timestamps: domain.Timestamps{CreatedAt: now, UpdatedAt: now}}
		switch {
		case r.Category == domain.EmailMarketing && !s.cfg.AllowMarketing:
			row.Status, row.StatusReason = domain.EmailCancelled, reasonNoMarketingProvider
		case suppressed[p.hash]:
			row.Status, row.StatusReason = domain.EmailSuppressed, reasonSuppressed
		case r.Category != domain.EmailSecurity && r.UserID != nil:
			n, ok := capped[*r.UserID]
			if !ok {
				if n, err = s.repo.CountRecent(ctx, *r.UserID, now.Add(-24*time.Hour), domain.EmailSecurity); err != nil {
					return fmt.Errorf("mail: daily cap: %w", err)
				}
			}
			if n >= int64(s.cfg.DailyCap) {
				row.Status, row.StatusReason = domain.EmailCancelled, reasonDailyCap
			}
			capped[*r.UserID] = n + 1
		}
		if row.Status == domain.EmailQueued {
			rendered, err := mail.Render(r.Content, mail.Layout{Locale: row.Locale, FrontendURL: s.cfg.FrontendURL, Category: string(r.Category),
				SupportEmail: s.cfg.SupportEmail})
			if err != nil {
				return err
			}
			body, _ := json.Marshal(emailPayload{Subject: rendered.Subject, Text: rendered.Text, HTML: rendered.HTML, Headers: rendered.Headers})
			if row.ToSealed, err = s.sealer.Seal(p.to, outboxAD(row.ID)); err != nil {
				return err
			}
			if row.PayloadSealed, err = s.sealer.Seal(string(body), outboxAD(row.ID)); err != nil {
				return err
			}
		} else {
			logger.FromContext(ctx).Info("mail: e-mail not sent", zap.String("template", r.Template), zap.String("reason", row.StatusReason))
		}
		rows = append(rows, row)
	}
	if err := s.repo.Enqueue(ctx, rows); err != nil {
		return fmt.Errorf("mail: enqueue: %w", err)
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return nil
}

// DeliverDue claims due rows and delivers them; it returns how many rows it
// processed. Several instances may run it concurrently (SKIP LOCKED leases).
func (s *MailService) DeliverDue(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = s.cfg.BatchSize
	}
	var claimed []domain.EmailOutbox
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		claimed, err = s.repo.Claim(ctx, s.clock.Now(), s.cfg.Lease, limit)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("mail: claim: %w", err)
	}
	for i := range claimed {
		s.deliver(ctx, &claimed[i])
	}
	return len(claimed), nil
}

// Drain delivers until nothing is due (tests and the wake path).
func (s *MailService) Drain(ctx context.Context) (int, error) {
	total := 0
	for {
		n, err := s.DeliverDue(ctx, s.cfg.BatchSize)
		total += n
		if err != nil || n < s.cfg.BatchSize {
			return total, err
		}
	}
}

func (s *MailService) deliver(ctx context.Context, row *domain.EmailOutbox) {
	log := logger.FromContext(ctx).With(zap.String("email_id", row.ID.String()), zap.String("template", row.Template))
	finish := func(fields map[string]any) {
		fields["updated_at"] = s.clock.Now()
		if err := s.repo.Finish(ctx, row.ID, fields); err != nil {
			log.Error("mail: update outbox row failed", zap.Error(err))
		}
	}
	final := func(status domain.EmailStatus, reason, lastErr string) {
		// Final rows keep no recipient or body.
		finish(map[string]any{"status": status, "status_reason": reason, "last_error": lastErr, "locked_until": nil,
			"to_sealed": "", "payload_sealed": ""})
	}
	to, err1 := s.sealer.Open(row.ToSealed, outboxAD(row.ID))
	raw, err2 := s.sealer.Open(row.PayloadSealed, outboxAD(row.ID))
	var payload emailPayload
	if err1 != nil || err2 != nil || json.Unmarshal([]byte(raw), &payload) != nil {
		log.Error("mail: outbox row cannot be decrypted (key rotated or tampered); dropping")
		final(domain.EmailFailed, reasonUnreadable, "")
		return
	}
	if _, ok := NormalizeEmail(to); !ok {
		final(domain.EmailFailed, reasonInvalidRecipient, "")
		return
	}
	sendCtx, cancel := context.WithTimeout(ctx, s.cfg.SendTimeout)
	res, err := s.provider.Send(sendCtx, mail.Message{To: to, Subject: payload.Subject, Text: payload.Text, HTML: payload.HTML,
		Headers: payload.Headers, Category: string(row.Category)})
	cancel()
	now := s.clock.Now()
	switch {
	case err == nil && bounced(res, to):
		s.suppress(ctx, row.ToHash, mail.Name(s.provider))
		final(domain.EmailSuppressed, reasonBounced, "")
		log.Warn("mail: recipient bounced permanently; address suppressed")
	case err == nil:
		finish(map[string]any{"status": domain.EmailSent, "status_reason": "", "last_error": "", "locked_until": nil,
			"sent_at": now, "provider": truncateString(mail.Name(s.provider), 32), "provider_message_id": truncateString(res.MessageID, 200),
			"to_sealed": "", "payload_sealed": ""})
	case mail.IsRetryable(err) && row.Attempts < s.cfg.MaxAttempts:
		next := now.Add(s.backoff(row.Attempts, mail.RetryAfter(err)))
		finish(map[string]any{"status": domain.EmailQueued, "locked_until": nil, "next_attempt_at": next,
			"last_error": describeMailErr(err), "provider": truncateString(mail.Name(s.provider), 32)})
		log.Warn("mail: delivery failed; will retry", zap.Int("attempt", row.Attempts), zap.Time("next_attempt_at", next),
			zap.String("error", describeMailErr(err)))
	case mail.IsRetryable(err):
		final(domain.EmailDead, reasonMaxAttempts, describeMailErr(err))
		log.Error("mail: delivery abandoned after max attempts", zap.Int("attempts", row.Attempts), zap.String("error", describeMailErr(err)))
	default:
		final(domain.EmailFailed, reasonRejected, describeMailErr(err))
		log.Error("mail: provider rejected the message", zap.String("error", describeMailErr(err)))
	}
}

func bounced(res mail.Result, to string) bool {
	for _, a := range res.Suppressed {
		if strings.EqualFold(strings.TrimSpace(a), to) {
			return true
		}
	}
	return false
}

func (s *MailService) suppress(ctx context.Context, hash, source string) {
	now := s.clock.Now()
	if err := s.repo.Suppress(ctx, &domain.EmailSuppression{EmailHash: hash, Reason: domain.SuppressHardBounce, Source: truncateString(source, 40),
		Timestamps: domain.Timestamps{CreatedAt: now, UpdatedAt: now}}); err != nil {
		logger.FromContext(ctx).Error("mail: record suppression failed", zap.Error(err))
	}
}

// backoff is BaseBackoff·2^(attempt-1) ±20%, capped at MaxBackoff; a longer
// provider Retry-After wins (capped at 24h).
func (s *MailService) backoff(attempt int, retryAfter time.Duration) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := float64(s.cfg.BaseBackoff) * math.Pow(2, float64(attempt-1))
	d = math.Min(d, float64(s.cfg.MaxBackoff))
	d *= 0.8 + 0.4*s.jitter()
	out := time.Duration(d)
	if retryAfter > out {
		out = min(retryAfter, 24*time.Hour)
	}
	return out
}

func describeMailErr(err error) string {
	var se *mail.SendError
	if errors.As(err, &se) {
		if se.Code != 0 {
			return truncateString(fmt.Sprintf("status %d code %d %s", se.Status, se.Code, se.Reason), 300)
		}
		return truncateString(fmt.Sprintf("status %d %s", se.Status, se.Reason), 300)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "transport"
}

// PurgeFinished deletes final outbox rows older than retention.
func (s *MailService) PurgeFinished(ctx context.Context, now time.Time, retention time.Duration) (int64, error) {
	return s.repo.PurgeFinished(ctx, now.Add(-retention))
}
