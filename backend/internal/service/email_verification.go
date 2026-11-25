package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	mailer "github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// Growth: e-mail verification by secure link.
//
// A new password account gets a welcome e-mail with a single-use link
// ({frontend}/{locale}/verify-email#token=...; the fragment never reaches a
// server, a log or a referrer). The token is 32 random bytes, only its hash
// is stored, it expires, it is consumed atomically, and its sealed payload
// binds it to the address it was issued for. Opening the link (in any
// browser, signed in or not) verifies the address and tells every tab and
// device of the user through the realtime connection.

// NotifyAccountEmailVerified is the in-app notification sent once the
// address was verified (the clients announce the realtime event themselves
// and do not toast the notification a second time).
const NotifyAccountEmailVerified = "account.email_verified"

const (
	pathVerifyEmail = "/verify-email"

	// VerificationVerified and VerificationAlreadyVerified are the statuses of
	// VerificationResult.
	VerificationVerified        = "verified"
	VerificationAlreadyVerified = "already_verified"
)

// EmailVerificationSettings tune the verification flow.
type EmailVerificationSettings struct {
	FrontendURL  string
	IPHashSecret string
	// TTL bounds the link (default 48 hours).
	TTL time.Duration
	// ResendCooldown is the minimum time between two links (default 60 s).
	ResendCooldown time.Duration
}

func (c EmailVerificationSettings) withDefaults() EmailVerificationSettings {
	if c.TTL <= 0 {
		c.TTL = 48 * time.Hour
	}
	if c.ResendCooldown <= 0 {
		c.ResendCooldown = time.Minute
	}
	return c
}

// EmailVerificationDeps are the collaborators of EmailVerificationService.
// Events, Notifier and Tracker are optional.
type EmailVerificationDeps struct {
	Tx       database.TxManager
	Users    repository.UserRepository
	Actions  repository.ActionTokenRepository
	Mail     EmailQueue
	Sealer   *security.Sealer
	Events   EventPublisher
	Notifier Notifier
	Audit    repository.AuditRepository
	Tracker  analytics.Tracker
	Clock    clock.Clock
	Settings EmailVerificationSettings
}

// EmailVerificationService verifies account addresses by e-mailed link.
type EmailVerificationService struct {
	d   EmailVerificationDeps
	cfg EmailVerificationSettings

	mu         sync.RWMutex
	onVerified []func(ctx context.Context, userID uuid.UUID)
}

// NewEmailVerificationService wires the service.
func NewEmailVerificationService(d EmailVerificationDeps) *EmailVerificationService {
	if d.Events == nil {
		d.Events = NoopPublisher{}
	}
	if d.Notifier == nil {
		d.Notifier = NoopNotifier{}
	}
	if d.Tracker == nil {
		d.Tracker = analytics.Noop{}
	}
	return &EmailVerificationService{d: d, cfg: d.Settings.withDefaults()}
}

// OnVerified registers a callback run after an address was verified for the
// first time (once the transaction committed; the referral rewards use it).
// Callbacks run in registration order and must log their own errors.
func (s *EmailVerificationService) OnVerified(fn func(ctx context.Context, userID uuid.UUID)) {
	if fn == nil {
		return
	}
	s.mu.Lock()
	s.onVerified = append(s.onVerified, fn)
	s.mu.Unlock()
}

// verificationPayload is sealed into the token: the address it vouches for.
type verificationPayload struct {
	Email string `json:"email"`
}

// VerificationSent is the answer to a resend.
type VerificationSent struct {
	Status string `json:"status"` // sent
	// Email is the masked address the link was sent to.
	Email string `json:"email"`
	// ResendAvailableAt is when another link can be requested.
	ResendAvailableAt time.Time `json:"resend_available_at"`
}

// VerificationResult is the answer to opening a link.
type VerificationResult struct {
	// Status is verified or already_verified.
	Status        string `json:"status"`
	EmailVerified bool   `json:"email_verified"`
}

func (s *EmailVerificationService) link(locale, raw string) string {
	return mailer.AppURL(s.cfg.FrontendURL, locale, pathVerifyEmail) + "#token=" + raw
}

func (s *EmailVerificationService) record(ctx context.Context, action string, userID uuid.UUID, meta map[string]any) {
	if s.d.Audit == nil {
		return
	}
	recordAudit(ctx, s.d.Audit, s.d.Clock, s.cfg.IPHashSecret, action, "user", userID.String(), meta)
}

// sendMode says how issue treats a user who cannot receive a link.
type sendMode int

const (
	sendStrict sendMode = iota // report already_verified and the cooldown
	sendQuiet                  // skip silently (reminders)
	sendFirst                  // first link of a new account: no cooldown
)

// errSkipped aborts a quiet send without an error for the caller.
var errSkipped = errors.New("verification: skipped")

// issue sends a new link to the user, invalidating the open ones. The user
// row is locked, so concurrent requests are serialised and the cooldown
// cannot be raced.
func (s *EmailVerificationService) issue(ctx context.Context, userID uuid.UUID, mode sendMode, requireActive bool) (*VerificationSent, error) {
	now := s.d.Clock.Now()
	var out *VerificationSent
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		u, err := s.d.Users.GetForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		if u.EmailVerifiedAt != nil {
			if mode == sendQuiet {
				return errSkipped
			}
			return apperror.AlreadyVerified()
		}
		if requireActive && !u.Active() {
			if mode == sendQuiet {
				return errSkipped
			}
			return apperror.InvalidState("Only active accounts can receive a verification email.")
		}
		if mode != sendFirst {
			latest, err := s.d.Actions.LatestOpen(ctx, u.ID, domain.ActionEmailVerify, now)
			switch {
			case err == nil:
				if wait := latest.CreatedAt.Add(s.cfg.ResendCooldown).Sub(now); wait > 0 {
					if mode == sendQuiet {
						return errSkipped
					}
					return apperror.RateLimited(wait)
				}
			case !errors.Is(err, repository.ErrNotFound):
				return err
			}
		}
		if err := s.d.Actions.InvalidateOpen(ctx, u.ID, domain.ActionEmailVerify, now); err != nil {
			return err
		}
		raw, t, err := s.newToken(ctx, u, now)
		if err != nil {
			return err
		}
		lang := normalizeLocale(u.Locale)
		if err := s.d.Mail.Enqueue(ctx, EmailRequest{UserID: &u.ID, To: u.Email, Locale: lang, Category: domain.EmailSecurity,
			Template: "email_verify", DedupeKey: "email_verify:" + t.ID.String(),
			Content: mailer.EmailVerification(lang, s.link(lang, raw), int(s.cfg.TTL/time.Hour))}); err != nil {
			return err
		}
		out = &VerificationSent{Status: "sent", Email: maskEmail(u.Email), ResendAvailableAt: now.Add(s.cfg.ResendCooldown)}
		return nil
	})
	if err != nil {
		if errors.Is(err, errSkipped) {
			return nil, nil
		}
		if _, ok := apperror.As(err); ok {
			return nil, err
		}
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// newToken creates the single-use token row for the user's current address.
func (s *EmailVerificationService) newToken(ctx context.Context, u *domain.User, now time.Time) (string, *domain.ActionToken, error) {
	raw, err := security.RandomToken(32)
	if err != nil {
		return "", nil, err
	}
	t := &domain.ActionToken{ID: domain.NewID(), UserID: u.ID, Purpose: domain.ActionEmailVerify, TokenHash: security.HashToken(raw),
		ExpiresAt: now.Add(s.cfg.TTL), CreatedAt: now, UpdatedAt: now}
	b, err := json.Marshal(verificationPayload{Email: u.Email})
	if err != nil {
		return "", nil, err
	}
	if t.PayloadEnc, err = s.d.Sealer.Seal(string(b), tokenAD(t)); err != nil {
		return "", nil, err
	}
	if err := s.d.Actions.Create(ctx, t); err != nil {
		return "", nil, err
	}
	return raw, t, nil
}

// OnRegistered is the registration hook: a new client account with an
// unverified address gets the welcome e-mail with its link. Failures are
// logged; they never fail the registration (the user can resend).
func (s *EmailVerificationService) OnRegistered(ctx context.Context, u *domain.User) {
	if u == nil || u.Role != domain.RoleClient || u.EmailVerifiedAt != nil {
		return
	}
	if _, err := s.issue(ctx, u.ID, sendFirst, true); err != nil {
		logger.FromContext(ctx).Error("email verification: send welcome link", zap.String("user_id", u.ID.String()), zap.Error(err))
	}
}

// Resend sends a new link to the caller. 409 already_verified when the
// address is verified; 429 (with Retry-After) inside the cooldown.
func (s *EmailVerificationService) Resend(ctx context.Context, userID uuid.UUID) (*VerificationSent, error) {
	out, err := s.issue(ctx, userID, sendStrict, true)
	if err != nil {
		return nil, mapRepoErrIssue(err)
	}
	s.d.Tracker.Track(ctx, "user.email_verification_resent", nil)
	return out, nil
}

// SendReminder sends a link like Resend but stays silent when the address is
// verified, the account is not active or a link was sent within the cooldown
// (the lifecycle "verify reminder").
func (s *EmailVerificationService) SendReminder(ctx context.Context, userID uuid.UUID) error {
	_, err := s.issue(ctx, userID, sendQuiet, true)
	return err
}

// AdminResend sends a new link on behalf of an administrator (audited).
func (s *EmailVerificationService) AdminResend(ctx context.Context, userID uuid.UUID) (*VerificationSent, error) {
	out, err := s.issue(ctx, userID, sendStrict, true)
	if err != nil {
		return nil, mapRepoErrIssue(err)
	}
	s.record(ctx, "admin.email_verification_resent", userID, nil)
	return out, nil
}

// mapRepoErrIssue turns a missing user into a 404 and keeps other errors.
func mapRepoErrIssue(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return mapRepoErr(err, "User")
	}
	return err
}

// Verify consumes a link: the address becomes verified (once), whatever
// browser opens it. Unknown, expired, used, mismatching and inactive-account
// links all answer the same invalid_token error; a link that can no longer
// be used is burnt.
func (s *EmailVerificationService) Verify(ctx context.Context, raw string) (*VerificationResult, error) {
	if !validRawToken(raw) {
		return nil, invalidLink()
	}
	now := s.d.Clock.Now()
	var (
		result  *VerificationResult
		invalid bool
	)
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		t, err := s.d.Actions.Consume(ctx, security.HashToken(raw), domain.ActionEmailVerify, now)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				invalid = true
				return nil
			}
			return err
		}
		u, err := s.d.Users.GetForUpdate(ctx, t.UserID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				invalid = true
				return nil
			}
			return err
		}
		var pl verificationPayload
		plain, openErr := s.d.Sealer.Open(t.PayloadEnc, tokenAD(t))
		if openErr != nil || json.Unmarshal([]byte(plain), &pl) != nil || pl.Email != u.Email || !u.Active() {
			invalid = true // committed: the consumed token stays burnt
			return nil
		}
		if u.EmailVerifiedAt != nil {
			result = &VerificationResult{Status: VerificationAlreadyVerified, EmailVerified: true}
			return nil
		}
		if err := s.d.Users.Updates(ctx, u.ID, map[string]any{"email_verified_at": now}); err != nil {
			return err
		}
		if err := s.d.Actions.InvalidateOpen(ctx, u.ID, domain.ActionEmailVerify, now); err != nil {
			return err
		}
		result = &VerificationResult{Status: VerificationVerified, EmailVerified: true}
		user := *u
		database.AfterCommit(ctx, func(ctx context.Context) { s.afterVerified(ctx, &user, now) })
		return nil
	})
	if err != nil {
		if _, ok := apperror.As(err); ok {
			return nil, err
		}
		return nil, apperror.Internal(err)
	}
	if invalid {
		return nil, invalidLink()
	}
	return result, nil
}

// afterVerified runs once the verification committed: realtime event to every
// tab, in-app notification, audit, analytics and the OnVerified hooks.
func (s *EmailVerificationService) afterVerified(ctx context.Context, u *domain.User, at time.Time) {
	ctx = reqctx.WithPrincipal(ctx, reqctx.Principal{UserID: u.ID, Role: u.Role})
	warnErr(ctx, "email verification: realtime event", s.d.Events.Publish(ctx, realtime.EventEmailVerified,
		map[string]any{"email_verified": true, "email_verified_at": at}, u.ID))
	s.d.Notifier.Notify(ctx, NotificationSpec{Kind: NotifyAccountEmailVerified, Data: map[string]any{}, Link: "/profile/settings#account"}, u.ID)
	s.record(ctx, "account.email_verified", u.ID, nil)
	s.d.Tracker.Track(ctx, "user.email_verified", map[string]any{"role": string(u.Role)})
	s.mu.RLock()
	hooks := append([]func(context.Context, uuid.UUID){}, s.onVerified...)
	s.mu.RUnlock()
	for _, fn := range hooks {
		fn(ctx, u.ID)
	}
}
