package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	mailer "github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// Public frontend pages that receive the one-time links.
const (
	pathConfirmEmail = "/confirm-email"
	pathCancelEmail  = "/cancel-email-change"
	pathKeepAccount  = "/keep-account"
	pathLogin        = "/login"
	pathResetPw      = "/reset-password"
)

// ClosureConfirmation must be typed to close an account.
const ClosureConfirmation = "DELETE"

func maskEmail(e string) string { return logger.RedactEmail(e) }

// emailChangePayload is sealed into e-mail change tokens.
type emailChangePayload struct {
	Old string `json:"old,omitempty"`
	New string `json:"new"`
}

func tokenAD(t *domain.ActionToken) string {
	return "action:" + string(t.Purpose) + ":" + t.ID.String() + ":" + t.UserID.String()
}

func (s *AccountService) sealPayload(t *domain.ActionToken, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	t.PayloadEnc, err = s.d.Sealer.Seal(string(b), tokenAD(t))
	return err
}

func (s *AccountService) openPayload(t *domain.ActionToken, v any) error {
	plain, err := s.d.Sealer.Open(t.PayloadEnc, tokenAD(t))
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(plain), v)
}

// newActionToken creates a single-use token row and returns the raw token.
func (s *AccountService) newActionToken(ctx context.Context, userID uuid.UUID, purpose domain.ActionPurpose, ttl time.Duration,
	subject *uuid.UUID, payload any) (string, *domain.ActionToken, error) {
	raw, err := security.RandomToken(32)
	if err != nil {
		return "", nil, err
	}
	now := s.d.Clock.Now()
	t := &domain.ActionToken{ID: domain.NewID(), UserID: userID, Purpose: purpose, TokenHash: security.HashToken(raw), SubjectID: subject,
		ExpiresAt: now.Add(ttl), CreatedAt: now, UpdatedAt: now}
	if payload != nil {
		if err := s.sealPayload(t, payload); err != nil {
			return "", nil, err
		}
	}
	if err := s.d.Actions.Create(ctx, t); err != nil {
		return "", nil, err
	}
	return raw, t, nil
}

func validRawToken(raw string) bool { return raw != "" && len(raw) <= 128 }

func (s *AccountService) link(locale, path, token string) string {
	u := mailer.AppURL(s.cfg.FrontendURL, locale, path)
	if token != "" {
		u += "?token=" + token
	}
	return u
}

func (s *AccountService) record(ctx context.Context, action string, userID uuid.UUID, meta map[string]any) {
	recordAudit(ctx, s.d.Audit, s.d.Clock, s.cfg.IPHashSecret, action, "user", userID.String(), meta)
}

// checkPassword verifies the caller's current password (re-authentication
// for sensitive actions).
func (s *AccountService) checkPassword(ctx context.Context, u *domain.User, password string) error {
	if !u.HasPassword() { // created with Google: set a password first
		return apperror.PasswordNotSet()
	}
	if password == "" || len(password) > security.PasswordMaxLength*4 {
		if err := s.d.Hasher.VerifyDummyContext(ctx, password); err != nil {
			return hashError(err)
		}
		return apperror.Validation(map[string]string{"password": "The password is incorrect."})
	}
	if _, err := s.d.Hasher.VerifyContext(ctx, password, u.PasswordHash); err != nil {
		if hasherBusy(err) {
			return hashError(err)
		}
		return apperror.Validation(map[string]string{"password": "The password is incorrect."})
	}
	return nil
}

// afterSignOutEverywhere drops cached state and live connections once every
// session of a user was revoked.
func (s *AccountService) afterSignOutEverywhere(ctx context.Context, userID uuid.UUID) {
	s.d.Auth.InvalidateStatus(ctx, userID)
	warnErr(ctx, "account: realtime revoke", s.d.Events.Revoke(ctx, userID))
}

// ---------------------------------------------------------------------------
// E-mail change
// ---------------------------------------------------------------------------

// EmailChangeInput starts an e-mail change.
type EmailChangeInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RequestEmailChange e-mails a confirmation link to the new address and a
// notice (with a cancel link) to the current one. It reports success even
// when the new address belongs to another account — that address receives
// a notice instead — so the endpoint cannot reveal which addresses exist.
func (s *AccountService) RequestEmailChange(ctx context.Context, userID uuid.UUID, in EmailChangeInput) error {
	email, ok := NormalizeEmail(in.Email)
	if !ok {
		return apperror.Validation(map[string]string{"email": "Enter a valid email address."})
	}
	u, err := s.d.Users.GetByID(ctx, userID)
	if err != nil {
		return mapRepoErr(err, "User")
	}
	if u.Role == domain.RoleAdmin {
		return apperror.Forbidden("Administrator accounts are managed by the team.")
	}
	if err := s.checkPassword(ctx, u, in.Password); err != nil {
		return err
	}
	if email == u.Email {
		return apperror.Validation(map[string]string{"email": "This is already your email address."})
	}
	lang := normalizeLocale(u.Locale)
	other, err := s.d.Users.GetByEmail(ctx, email)
	switch {
	case err == nil:
		uid := other.ID
		if err := s.d.Mail.Enqueue(ctx, EmailRequest{UserID: &uid, To: email, Locale: normalizeLocale(other.Locale),
			Category: domain.EmailSecurity, Template: "email_change_attempt",
			Content: mailer.EmailChangeAttempt(normalizeLocale(other.Locale), s.link(other.Locale, pathLogin, ""))}); err != nil {
			logger.FromContext(ctx).Error("account: enqueue e-mail change attempt notice", zap.Error(err))
		}
		s.record(ctx, "email_change.requested", userID, map[string]any{"to": maskEmail(email), "address_in_use": true})
		return nil
	case !errors.Is(err, repository.ErrNotFound):
		return apperror.Internal(err)
	}
	now := s.d.Clock.Now()
	err = s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		// The latest request wins; earlier restore links stay valid.
		if err := s.d.Actions.InvalidateOpen(ctx, userID, domain.ActionEmailChange, now); err != nil {
			return err
		}
		confirm, ct, err := s.newActionToken(ctx, userID, domain.ActionEmailChange, s.cfg.EmailChangeTTL, nil, emailChangePayload{New: email})
		if err != nil {
			return err
		}
		cancel, _, err := s.newActionToken(ctx, userID, domain.ActionEmailChangeCancel, s.cfg.RevertWindow, &ct.ID,
			emailChangePayload{Old: u.Email, New: email})
		if err != nil {
			return err
		}
		minutes := int(s.cfg.EmailChangeTTL / time.Minute)
		return s.d.Mail.Enqueue(ctx,
			// The new address is not attached to the account yet.
			EmailRequest{To: email, Locale: lang, Category: domain.EmailSecurity, Template: "email_change_confirm",
				Content: mailer.EmailChangeRequested(lang, s.link(lang, pathConfirmEmail, confirm), minutes)},
			EmailRequest{UserID: &u.ID, To: u.Email, Locale: lang, Category: domain.EmailSecurity, Template: "email_change_notice",
				Content: mailer.EmailChangeNotice(lang, maskEmail(email), s.link(lang, pathCancelEmail, cancel))})
	})
	if err != nil {
		return apperror.Internal(err)
	}
	s.d.Notifier.Notify(ctx, NotificationSpec{Kind: NotifyAccountEmailChangeRequested, Data: map[string]any{"email": maskEmail(email)},
		Link: "/profile/settings#account"}, userID)
	s.record(ctx, "email_change.requested", userID, map[string]any{"to": maskEmail(email)})
	s.d.Tracker.Track(ctx, "account.email_change_requested", nil)
	return nil
}

// EmailChangeResult reports what a link did.
type EmailChangeResult struct {
	// Status is confirmed, cancelled or restored.
	Status string `json:"status"`
}

func invalidLink() error { return apperror.InvalidToken("This link is invalid or has expired.") }

// ConfirmEmailChange completes a change from the link sent to the new
// address. Every session is revoked (the user signs in with the new
// address) and the previous address receives a restore link.
func (s *AccountService) ConfirmEmailChange(ctx context.Context, raw string) (*EmailChangeResult, error) {
	if !validRawToken(raw) {
		return nil, invalidLink()
	}
	now := s.d.Clock.Now()
	var user *domain.User
	var newEmail string
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		t, err := s.d.Actions.Consume(ctx, security.HashToken(raw), domain.ActionEmailChange, now)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return invalidLink()
			}
			return err
		}
		var pl emailChangePayload
		if err := s.openPayload(t, &pl); err != nil {
			return invalidLink()
		}
		u, err := s.d.Users.GetForUpdate(ctx, t.UserID)
		if err != nil {
			return err
		}
		if !u.Active() {
			return invalidLink()
		}
		if err := s.d.Users.Updates(ctx, u.ID, map[string]any{"email": pl.New, "email_verified_at": now,
			"token_version": gorm.Expr("token_version + 1")}); err != nil {
			if database.IsUniqueViolation(err, "users_email_key") {
				return apperror.Conflict(apperror.CodeEmailTaken, "An account with this email already exists.")
			}
			return err
		}
		if _, err := s.d.Tokens.RevokeAllForUser(ctx, u.ID, "email_changed", now); err != nil {
			return err
		}
		restore, _, err := s.newActionToken(ctx, u.ID, domain.ActionEmailChangeCancel, s.cfg.RevertWindow, &t.ID,
			emailChangePayload{Old: u.Email, New: pl.New})
		if err != nil {
			return err
		}
		lang := normalizeLocale(u.Locale)
		user, newEmail = u, pl.New
		return s.d.Mail.Enqueue(ctx, EmailRequest{UserID: &u.ID, To: u.Email, Locale: lang, Category: domain.EmailSecurity,
			Template: "email_changed", Content: mailer.EmailChangedRevertible(lang, maskEmail(pl.New), s.link(lang, pathCancelEmail, restore))})
	})
	if err != nil {
		if _, ok := apperror.As(err); ok {
			return nil, err
		}
		return nil, apperror.Internal(err)
	}
	s.afterSignOutEverywhere(ctx, user.ID)
	s.d.Notifier.Notify(ctx, NotificationSpec{Kind: NotifyAccountEmailChanged, Data: map[string]any{"email": maskEmail(newEmail)},
		Link: "/profile/settings#account"}, user.ID)
	ctx = reqctx.WithPrincipal(ctx, reqctx.Principal{UserID: user.ID, Role: user.Role})
	s.record(ctx, "email_change.confirmed", user.ID, map[string]any{"from": maskEmail(user.Email), "to": maskEmail(newEmail)})
	s.d.Tracker.Track(ctx, "account.email_changed", nil)
	return &EmailChangeResult{Status: "confirmed"}, nil
}

// CancelEmailChange handles the link sent to the previous address: before
// confirmation it cancels the pending change; afterwards it restores the
// previous address, signs every device out and e-mails a password reset
// link (the account may be compromised).
func (s *AccountService) CancelEmailChange(ctx context.Context, raw string) (*EmailChangeResult, error) {
	if !validRawToken(raw) {
		return nil, invalidLink()
	}
	now := s.d.Clock.Now()
	var user *domain.User
	var restoredTo string
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		t, err := s.d.Actions.Consume(ctx, security.HashToken(raw), domain.ActionEmailChangeCancel, now)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return invalidLink()
			}
			return err
		}
		var pl emailChangePayload
		if err := s.openPayload(t, &pl); err != nil || pl.Old == "" {
			return invalidLink()
		}
		u, err := s.d.Users.GetForUpdate(ctx, t.UserID)
		if err != nil {
			return err
		}
		if u.Status == domain.UserDeleted {
			return invalidLink()
		}
		user = u
		if err := s.d.Actions.InvalidateOpen(ctx, u.ID, domain.ActionEmailChange, now); err != nil {
			return err
		}
		if u.Email == pl.Old {
			return nil // not confirmed yet: the pending change is cancelled
		}
		if err := s.d.Users.Updates(ctx, u.ID, map[string]any{"email": pl.Old, "token_version": gorm.Expr("token_version + 1")}); err != nil {
			if database.IsUniqueViolation(err, "users_email_key") {
				return apperror.Conflict(apperror.CodeEmailTaken, "This email address now belongs to another account. Contact our support team.")
			}
			return err
		}
		if _, err := s.d.Tokens.RevokeAllForUser(ctx, u.ID, "email_change_reverted", now); err != nil {
			return err
		}
		reset, err := security.RandomToken(32)
		if err != nil {
			return err
		}
		if err := s.d.Tokens.InvalidateResets(ctx, u.ID, now); err != nil {
			return err
		}
		if err := s.d.Tokens.CreateReset(ctx, &domain.PasswordResetToken{ID: domain.NewID(), UserID: u.ID,
			TokenHash: security.HashToken(reset), ExpiresAt: now.Add(s.cfg.ResetTTL), CreatedAt: now}); err != nil {
			return err
		}
		restoredTo = pl.Old
		lang := normalizeLocale(u.Locale)
		return s.d.Mail.Enqueue(ctx, EmailRequest{UserID: &u.ID, To: pl.Old, Locale: lang, Category: domain.EmailSecurity,
			Template: "email_restored", Content: mailer.EmailRestored(lang, s.link(lang, pathResetPw, reset), int(s.cfg.ResetTTL/time.Minute))})
	})
	if err != nil {
		if _, ok := apperror.As(err); ok {
			return nil, err
		}
		return nil, apperror.Internal(err)
	}
	ctx = reqctx.WithPrincipal(ctx, reqctx.Principal{UserID: user.ID, Role: user.Role})
	if restoredTo == "" {
		s.record(ctx, "email_change.cancelled", user.ID, nil)
		return &EmailChangeResult{Status: "cancelled"}, nil
	}
	s.afterSignOutEverywhere(ctx, user.ID)
	s.record(ctx, "email_change.restored", user.ID, map[string]any{"from": maskEmail(user.Email), "to": maskEmail(restoredTo)})
	s.d.Tracker.Track(ctx, "account.email_restored", nil)
	return &EmailChangeResult{Status: "restored"}, nil
}

// ---------------------------------------------------------------------------
// Deactivation, closure and reactivation
// ---------------------------------------------------------------------------

// PasswordInput re-authenticates a sensitive action.
type PasswordInput struct {
	Password string `json:"password"`
}

// endPresence ends live chats and takes an advisor offline (their public
// profile is hidden while the account is not active).
func (s *AccountService) endPresence(ctx context.Context, u *domain.User) {
	if s.d.Chat != nil {
		s.d.Chat.EndAllForUser(ctx, u.ID)
	}
	if u.Role == domain.RolePsychic && s.d.PsychicSvc != nil {
		s.d.PsychicSvc.PublishStatus(ctx, u.ID)
	}
}

func (s *AccountService) offline(ctx context.Context, u *domain.User) error {
	if u.Role != domain.RolePsychic || s.d.Psychics == nil {
		return nil
	}
	err := s.d.Psychics.Updates(ctx, u.ID, map[string]any{"availability": domain.AvailabilityOffline})
	if errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	return err
}

// Deactivate pauses the caller's account: every session is revoked, live
// chats end and an advisor's profile is hidden. Signing in again offers
// reactivation.
func (s *AccountService) Deactivate(ctx context.Context, userID uuid.UUID, in PasswordInput) error {
	u, err := s.d.Users.GetByID(ctx, userID)
	if err != nil {
		return mapRepoErr(err, "User")
	}
	if u.Role == domain.RoleAdmin {
		return apperror.Forbidden("Administrator accounts are managed by the team.")
	}
	if err := s.checkPassword(ctx, u, in.Password); err != nil {
		return err
	}
	now := s.d.Clock.Now()
	lang := normalizeLocale(u.Locale)
	err = s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.d.Users.Updates(ctx, u.ID, map[string]any{"status": domain.UserDeactivated, "deactivated_by": domain.DeactivatedBySelf,
			"status_reason": "", "status_changed_at": now, "closure_scheduled_at": nil, "token_version": gorm.Expr("token_version + 1")}); err != nil {
			return err
		}
		if _, err := s.d.Tokens.RevokeAllForUser(ctx, u.ID, "account_deactivated", now); err != nil {
			return err
		}
		if err := s.offline(ctx, u); err != nil {
			return err
		}
		return s.d.Mail.Enqueue(ctx, EmailRequest{UserID: &u.ID, To: u.Email, Locale: lang, Category: domain.EmailSecurity,
			Template: "account_deactivated", Content: mailer.AccountDeactivated(lang, s.link(lang, pathLogin, ""))})
	})
	if err != nil {
		return apperror.Internal(err)
	}
	s.d.Auth.endPushSessions(ctx, u.ID)
	s.afterSignOutEverywhere(ctx, u.ID)
	s.endPresence(ctx, u)
	s.record(ctx, "user.self_deactivated", u.ID, map[string]any{"role": string(u.Role)})
	s.d.Tracker.Track(ctx, "account.deactivated", nil)
	return nil
}

// CloseInput requests the permanent closure of the caller's account.
type CloseInput struct {
	Password     string `json:"password"`
	Confirmation string `json:"confirmation"`
	Reason       string `json:"reason"`
}

// ClosureView answers a closure request.
type ClosureView struct {
	Status              string    `json:"status"`
	DeletionScheduledAt time.Time `json:"deletion_scheduled_at"`
}

// closureBlockers lists what prevents an advisor from leaving: credits not
// yet paid out, payouts in progress and orders still open.
func (s *AccountService) closureBlockers(ctx context.Context, u *domain.User) ([]string, error) {
	if u.Role != domain.RolePsychic {
		return nil, nil
	}
	var reasons []string
	if s.d.Payouts != nil {
		b, err := s.d.Payouts.Balance(ctx, u.ID)
		if err != nil {
			return nil, err
		}
		// Credits below the minimum payout cannot be withdrawn, so they do
		// not hold the account back.
		if b.AvailableCredits > 0 && b.AvailableCredits >= b.MinWithdrawal {
			reasons = append(reasons, "earnings_balance")
		}
		if b.ReservedCredits > 0 {
			reasons = append(reasons, "pending_withdrawals")
		}
	}
	if s.d.Requests != nil {
		id := u.ID
		open, err := s.d.Requests.List(ctx, repository.RequestFilter{PsychicID: &id,
			Statuses: []domain.CustomRequestStatus{domain.RequestPending, domain.RequestAccepted, domain.RequestDelivered, domain.RequestDisputed}, Limit: 1})
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if len(open) > 0 {
			reasons = append(reasons, "open_requests")
		}
	}
	return reasons, nil
}

// Close schedules the caller's account for erasure after the grace period.
// The account is paused at once (sessions revoked, profile hidden); the
// link e-mailed now, or signing in again, keeps it.
func (s *AccountService) Close(ctx context.Context, userID uuid.UUID, in CloseInput) (*ClosureView, error) {
	fields := map[string]string{}
	if in.Confirmation != ClosureConfirmation {
		fields["confirmation"] = "Type DELETE to confirm."
	}
	reason := CleanText(in.Reason)
	if utf8.RuneCountInString(reason) > 500 {
		fields["reason"] = "Use at most 500 characters."
	}
	u, err := s.d.Users.GetByID(ctx, userID)
	if err != nil {
		return nil, mapRepoErr(err, "User")
	}
	if u.Role == domain.RoleAdmin {
		return nil, apperror.Forbidden("Administrator accounts are managed by the team.")
	}
	if err := s.checkPassword(ctx, u, in.Password); err != nil {
		var ae *apperror.Error
		if errors.As(err, &ae) && ae.Code == apperror.CodeValidation {
			fields["password"] = "The password is incorrect."
		} else {
			return nil, err
		}
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	reasons, err := s.closureBlockers(ctx, u)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if len(reasons) > 0 {
		return nil, apperror.ClosureBlocked(reasons)
	}
	now := s.d.Clock.Now()
	at := now.Add(s.cfg.ClosureGrace)
	lang := normalizeLocale(u.Locale)
	err = s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.d.Users.Updates(ctx, u.ID, map[string]any{"status": domain.UserDeactivated, "deactivated_by": domain.DeactivatedByClosure,
			"status_reason": "", "status_changed_at": now, "closure_scheduled_at": at, "token_version": gorm.Expr("token_version + 1")}); err != nil {
			return err
		}
		if _, err := s.d.Tokens.RevokeAllForUser(ctx, u.ID, "account_closure_scheduled", now); err != nil {
			return err
		}
		if err := s.offline(ctx, u); err != nil {
			return err
		}
		if err := s.d.Actions.InvalidateOpen(ctx, u.ID, domain.ActionClosureCancel, now); err != nil {
			return err
		}
		keep, _, err := s.newActionToken(ctx, u.ID, domain.ActionClosureCancel, s.cfg.ClosureGrace, nil, nil)
		if err != nil {
			return err
		}
		return s.d.Mail.Enqueue(ctx, EmailRequest{UserID: &u.ID, To: u.Email, Locale: lang, Category: domain.EmailSecurity,
			Template: "closure_scheduled", Content: mailer.ClosureScheduled(lang, formatDate(lang, at), s.link(lang, pathKeepAccount, keep))})
	})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	s.d.Auth.endPushSessions(ctx, u.ID)
	s.afterSignOutEverywhere(ctx, u.ID)
	s.endPresence(ctx, u)
	meta := map[string]any{"role": string(u.Role), "deletion_scheduled_at": at.Format(time.RFC3339)}
	if reason != "" {
		meta["reason"] = reason
	}
	s.record(ctx, "user.closure_scheduled", u.ID, meta)
	s.d.Tracker.Track(ctx, "account.closure_scheduled", nil)
	return &ClosureView{Status: "closure_scheduled", DeletionScheduledAt: at}, nil
}

// formatDate renders a date for e-mail copy in the recipient's language.
func formatDate(locale string, t time.Time) string {
	t = t.UTC()
	switch locale {
	case "fr", "es":
		return t.Format("02/01/2006")
	}
	return t.Format("January 2, 2006")
}

// reactivate restores a self-deactivated (or closing) account inside a
// transaction and returns it.
func (s *AccountService) reactivate(ctx context.Context, userID uuid.UUID, now time.Time) (*domain.User, error) {
	u, err := s.d.Users.GetForUpdate(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u.Status != domain.UserDeactivated || (u.DeactivatedBy != domain.DeactivatedBySelf && u.DeactivatedBy != domain.DeactivatedByClosure) {
		return nil, invalidLink()
	}
	if err := s.d.Users.Updates(ctx, u.ID, map[string]any{"status": domain.UserActive, "deactivated_by": "", "status_reason": "",
		"status_changed_at": now, "closure_scheduled_at": nil}); err != nil {
		return nil, err
	}
	for _, p := range []domain.ActionPurpose{domain.ActionClosureCancel, domain.ActionReactivate} {
		if err := s.d.Actions.InvalidateOpen(ctx, u.ID, p, now); err != nil {
			return nil, err
		}
	}
	return s.d.Users.GetByID(ctx, u.ID)
}

func (s *AccountService) afterReactivation(ctx context.Context, u *domain.User, how string) {
	s.d.Auth.InvalidateStatus(ctx, u.ID)
	if u.Role == domain.RolePsychic && s.d.PsychicSvc != nil {
		s.d.PsychicSvc.PublishStatus(ctx, u.ID)
	}
	lang := normalizeLocale(u.Locale)
	if err := s.d.Mail.Enqueue(ctx, EmailRequest{UserID: &u.ID, To: u.Email, Locale: lang, Category: domain.EmailSecurity,
		Template: "account_reactivated", Content: mailer.Reactivated(lang, mailer.AppURL(s.cfg.FrontendURL, lang, "/"))}); err != nil {
		logger.FromContext(ctx).Error("account: enqueue reactivation notice", zap.Error(err))
	}
	s.d.Notifier.Notify(ctx, NotificationSpec{Kind: NotifyAccountReactivated, Data: map[string]any{"via": how}}, u.ID)
	ctx = reqctx.WithPrincipal(ctx, reqctx.Principal{UserID: u.ID, Role: u.Role})
	s.record(ctx, "user.reactivated_self", u.ID, map[string]any{"via": how})
	s.d.Tracker.Track(ctx, "account.reactivated", map[string]any{"via": how})
}

// TokenInput carries a one-time token.
type TokenInput struct {
	Token string `json:"token"`
}

// Reactivate consumes the token offered at sign-in (after the password was
// checked) and signs the user in.
func (s *AccountService) Reactivate(ctx context.Context, raw string) (*AuthResult, error) {
	if !validRawToken(raw) {
		return nil, invalidLink()
	}
	now := s.d.Clock.Now()
	var user *domain.User
	var result *AuthResult
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		t, err := s.d.Actions.Consume(ctx, security.HashToken(raw), domain.ActionReactivate, now)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return invalidLink()
			}
			return err
		}
		if user, err = s.reactivate(ctx, t.UserID, now); err != nil {
			return err
		}
		result, err = s.d.Auth.issueSession(ctx, user, uuid.Nil)
		return err
	})
	if err != nil {
		if _, ok := apperror.As(err); ok {
			return nil, err
		}
		return nil, apperror.Internal(err)
	}
	s.afterReactivation(ctx, user, "sign_in")
	return result, nil
}

// CancelClosure keeps an account from the link e-mailed when its closure
// was requested. It does not sign the user in.
func (s *AccountService) CancelClosure(ctx context.Context, raw string) error {
	if !validRawToken(raw) {
		return invalidLink()
	}
	now := s.d.Clock.Now()
	var user *domain.User
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		t, err := s.d.Actions.Consume(ctx, security.HashToken(raw), domain.ActionClosureCancel, now)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return invalidLink()
			}
			return err
		}
		user, err = s.reactivate(ctx, t.UserID, now)
		return err
	})
	if err != nil {
		if _, ok := apperror.As(err); ok {
			return err
		}
		return apperror.Internal(err)
	}
	s.afterReactivation(ctx, user, "email_link")
	return nil
}

// ProcessClosures erases accounts whose closure date has passed (worker).
func (s *AccountService) ProcessClosures(ctx context.Context, limit int) (int, error) {
	if s.d.Eraser == nil {
		return 0, nil
	}
	ids, err := s.d.Accounts.DueClosures(ctx, s.d.Clock.Now(), limit)
	if err != nil {
		return 0, err
	}
	done := 0
	var errs []error
	for _, id := range ids {
		u, err := s.d.Eraser.Erase(ctx, id, EraseRequest{Reason: "self_closure", Notify: true, OnlyClosureDue: true})
		if err != nil {
			if errors.Is(err, ErrNotErasable) {
				continue
			}
			errs = append(errs, fmt.Errorf("erase %s: %w", id, err))
			continue
		}
		done++
		s.record(database.Detach(ctx), "user.erased", id, map[string]any{"reason": "self_closure", "role": string(u.Role)})
	}
	return done, errors.Join(errs...)
}
