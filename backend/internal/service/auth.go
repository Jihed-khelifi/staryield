package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
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

// AuthSettings are the auth-related configuration values.
type AuthSettings struct {
	RefreshTTL         time.Duration
	ResetTTL           time.Duration
	MaxFailures        int
	Lockout            time.Duration
	MinimumAge         int
	SignupFreeMinutes  int
	SignupBonusCredits int64
	StatusCacheTTL     time.Duration
	TicketTTL          time.Duration
	FrontendURL        string
	IPHashSecret       string
}

// AuthResult carries freshly issued credentials.
type AuthResult struct {
	UserID         uuid.UUID
	Role           domain.Role
	AccessToken    string
	AccessExpires  time.Time
	RefreshToken   string
	RefreshExpires time.Time
}

// RegisterInput is the sign-up payload.
type RegisterInput struct {
	Email       string
	Password    string
	DisplayName string
	AcceptTerms bool
	BirthDate   string
	Locale      string
}

// AuthService handles identity and credentials.
type AuthService struct {
	tx      database.TxManager
	users   repository.UserRepository
	tokens  repository.TokenRepository
	wallet  *WalletService
	wallets repository.WalletRepository
	hasher  *security.PasswordHasher
	issuer  *security.TokenIssuer
	kv      cache.Store
	tickets realtime.TicketStore
	mail    EmailQueue
	events  EventPublisher
	tracker analytics.Tracker
	clock   clock.Clock
	cfg     AuthSettings
	// ident enables signed-in devices (identity module; see devices.go).
	ident *authIdentity
	// revisions 3.0: site settings. signupMinutes returns the sign-up offer's
	// free minutes (SiteService.SignupFreeMinutes); nil falls back to
	// cfg.SignupFreeMinutes.
	signupMinutes func(ctx context.Context) int
	// revisions 4.0: Sign in with Google (set by NewGoogleAuth); nil when absent.
	google *GoogleAuth
	// push removes browser push subscriptions when sessions end (auth_push.go).
	push sessionPush
}

// SetSignupOffer installs the sign-up offer lookup (revisions 3.0).
func (s *AuthService) SetSignupOffer(fn func(ctx context.Context) int) { s.signupMinutes = fn }

// NewAuthService wires the auth service. E-mail (password resets) goes
// through the outbox (EmailQueue).
func NewAuthService(tx database.TxManager, users repository.UserRepository, tokens repository.TokenRepository,
	wallet *WalletService, wallets repository.WalletRepository, hasher *security.PasswordHasher, issuer *security.TokenIssuer,
	kv cache.Store, tickets realtime.TicketStore, m EmailQueue, events EventPublisher, tracker analytics.Tracker,
	clk clock.Clock, cfg AuthSettings) *AuthService {
	return &AuthService{tx: tx, users: users, tokens: tokens, wallet: wallet, wallets: wallets, hasher: hasher, issuer: issuer,
		kv: kv, tickets: tickets, mail: m, events: events, tracker: tracker, clock: clk, cfg: cfg}
}

// NormalizeEmail lowercases and validates an e-mail address.
func NormalizeEmail(raw string) (string, bool) {
	e := strings.ToLower(strings.TrimSpace(raw))
	if len(e) < 3 || len(e) > 254 || strings.ContainsAny(e, " \t\r\n<>\"'") {
		return "", false
	}
	addr, err := mail.ParseAddress(e)
	if err != nil || addr.Address != e {
		return "", false
	}
	at := strings.LastIndexByte(e, '@')
	if at < 1 || at > 64 || !strings.Contains(e[at+1:], ".") || strings.HasSuffix(e, ".") {
		return "", false
	}
	return e, true
}

// ParseBirthDate validates YYYY-MM-DD and the minimum age.
func ParseBirthDate(s string, now time.Time, minimumAge int) (*time.Time, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, ""
	}
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, "Enter a valid date (YYYY-MM-DD)."
	}
	if d.Year() < 1900 || d.After(now) {
		return nil, "Enter a valid birth date that is not in the future."
	}
	if minimumAge > 0 && d.AddDate(minimumAge, 0, 0).After(now) {
		return nil, fmt.Sprintf("You must be at least %d years old.", minimumAge)
	}
	return &d, ""
}

func normalizeLocale(l string) string {
	switch strings.ToLower(strings.TrimSpace(l)) {
	case "fr":
		return "fr"
	case "es":
		return "es"
	}
	return "en"
}

// Register creates a client account (FR-02) and signs it in.
func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*AuthResult, error) {
	fields := map[string]string{}
	email, ok := NormalizeEmail(in.Email)
	if !ok {
		fields["email"] = "Enter a valid email address."
	}
	if msg := security.ValidatePassword(in.Password, email); msg != "" {
		fields["password"] = msg
	}
	if !in.AcceptTerms {
		fields["accept_terms"] = "Please accept the Terms of Service and Privacy Policy."
	}
	now := s.clock.Now()
	birth, msg := ParseBirthDate(in.BirthDate, now, s.cfg.MinimumAge)
	if msg != "" {
		fields["birth_date"] = msg
	}
	name := CleanLine(in.DisplayName)
	if len([]rune(name)) > 80 {
		fields["display_name"] = "Use at most 80 characters."
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	if name == "" {
		name = defaultDisplayName(email)
	}
	hash, err := s.hasher.HashContext(ctx, in.Password)
	if err != nil {
		return nil, hashError(err)
	}
	acct, err := s.createAccount(ctx, newAccount{Email: email, PasswordHash: hash, DisplayName: name, Locale: in.Locale, Birth: birth})
	if err != nil {
		return nil, mapRepoErr(err, "User")
	}
	return acct.Result, nil
}

// defaultDisplayName is the e-mail's local part (at most 40 characters).
func defaultDisplayName(email string) string {
	local := email[:strings.IndexByte(email, '@')]
	if len(local) > 40 {
		local = local[:40]
	}
	return local
}

// newAccount describes a client account to create. Sign-up with an e-mail
// and password and sign-up with Google share createAccount, so the wallet,
// the sign-up offer, the bonus credits, the profile and the terms timestamp
// can never differ between them.
type newAccount struct {
	Email string
	// PasswordHash is '' for an account created with Google (no password).
	PasswordHash string
	DisplayName  string
	Locale       string
	Birth        *time.Time
	// EmailVerified stamps email_verified_at (Google vouches for the address).
	EmailVerified bool
	// Provider names the sign-up method for analytics ("" = password).
	Provider string
	// InTx runs inside the creating transaction after the wallet exists and
	// before the session is issued (e.g. linking the Google identity).
	InTx func(ctx context.Context, user *domain.User) error
}

// createdAccount is the outcome of createAccount.
type createdAccount struct {
	User   *domain.User
	Result *AuthResult
}

// createAccount creates a client account (terms accepted now) with its
// profile, wallet, sign-up offer and bonus, and signs it in, in one
// transaction. A taken address is the apperror email_taken; errors from InTx
// are returned as they are.
func (s *AuthService) createAccount(ctx context.Context, in newAccount) (*createdAccount, error) {
	now := s.clock.Now()
	user := &domain.User{
		ID: domain.NewID(), Email: in.Email, PasswordHash: in.PasswordHash, Role: domain.RoleClient, Status: domain.UserActive,
		DisplayName: in.DisplayName, Locale: normalizeLocale(in.Locale), TermsAcceptedAt: now,
		CreatedAt: now, UpdatedAt: now,
	}
	if in.EmailVerified {
		user.EmailVerifiedAt = &now
	}
	var result *AuthResult
	signupMinutes := s.cfg.SignupFreeMinutes // revisions 3.0: site settings
	if s.signupMinutes != nil {
		signupMinutes = max(s.signupMinutes(ctx), 0)
	}
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.users.Create(ctx, user); err != nil {
			if errors.Is(err, repository.ErrDuplicate) {
				return apperror.Conflict(apperror.CodeEmailTaken, "An account with this email already exists.")
			}
			return err
		}
		if err := s.users.CreateProfile(ctx, newProfile(user.ID, in.Birth, now)); err != nil {
			return err
		}
		if err := s.wallets.Create(ctx, &domain.Wallet{UserID: user.ID, FreeMinutes: signupMinutes, UpdatedAt: now}); err != nil {
			return err
		}
		if s.cfg.SignupBonusCredits > 0 {
			if _, _, err := s.wallet.Apply(ctx, Movement{
				UserID: user.ID, Type: domain.LedgerSignupBonus, Amount: s.cfg.SignupBonusCredits,
				ReferenceType: "user", ReferenceID: &user.ID, IdempotencyKey: "signup_bonus:" + user.ID.String(),
				Description: "Welcome bonus",
			}); err != nil {
				return err
			}
		}
		if in.InTx != nil {
			if err := in.InTx(ctx, user); err != nil {
				return err
			}
		}
		var err error
		result, err = s.issueSession(ctx, user, uuid.Nil)
		return err
	})
	if err != nil {
		return nil, err
	}
	ctx = reqctx.WithPrincipal(ctx, reqctx.Principal{UserID: user.ID, Role: user.Role})
	props := map[string]any{"role": string(user.Role), "locale": user.Locale, "has_birth_date": in.Birth != nil,
		"signup_free_minutes": signupMinutes}
	if in.Provider != "" {
		props["provider"] = in.Provider
	}
	s.tracker.Track(ctx, "user.registered", props)
	logger.FromContext(ctx).Info("user registered", zap.String("user_id", user.ID.String()))
	return &createdAccount{User: user, Result: result}, nil
}

func newProfile(userID uuid.UUID, birth *time.Time, now time.Time) *domain.Profile {
	return &domain.Profile{UserID: userID, BirthDate: birth, Mood: "Empowered", Theme: "Warm Beige", NotifyHoroscope: true, UpdatedAt: now}
}

func (s *AuthService) throttleKey(email string) string {
	return "auth:fail:" + security.HashToken(email)[:32]
}

// Login verifies credentials (FR-02) with per-account lockout after
// repeated failures. Unknown e-mails and wrong passwords are
// indistinguishable in both response and timing.
func (s *AuthService) Login(ctx context.Context, rawEmail, password string) (*AuthResult, error) {
	res, _, err := s.LoginLinking(ctx, rawEmail, password, "")
	return res, err
}

// LoginLinking is Login for a browser that may hold a Google sign-in cookie
// (revisions 4.0, §23.1): when the cookie waits for a link_password whose
// e-mail is this account's, and the password was right, the Google identity
// is linked in the same transaction as the session. linked is "google" then.
func (s *AuthService) LoginLinking(ctx context.Context, rawEmail, password, googleCookie string) (res *AuthResult, linked string, err error) {
	user, err := s.VerifyCredentials(ctx, rawEmail, password)
	if err != nil {
		return nil, "", err
	}
	if !user.Active() {
		s.tracker.Track(ctx, "user.login_failed", map[string]any{"reason": string(user.Status)})
		if err := s.offerReactivation(ctx, user); err != nil {
			return nil, "", err
		}
		return nil, "", apperror.AccountDisabled()
	}
	var link *googleLink
	if googleCookie != "" && s.google != nil {
		link = s.google.takeLink(ctx, user, googleCookie)
	}
	var result *AuthResult
	if link == nil {
		result, err = s.issueSession(ctx, user, uuid.Nil)
	} else {
		err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
			if err := s.google.linkInTx(ctx, user, link); err != nil {
				return err
			}
			var err error
			result, err = s.issueSession(ctx, user, uuid.Nil)
			return err
		})
	}
	if errors.Is(err, repository.ErrIdentityTaken) {
		// The Google identity was linked elsewhere in the meantime (the
		// transaction rolled back): the password was right, so sign in
		// without linking.
		link = nil
		result, err = s.issueSession(ctx, user, uuid.Nil)
	}
	if err != nil {
		return nil, "", apperror.Internal(err)
	}
	ctx = reqctx.WithPrincipal(ctx, reqctx.Principal{UserID: user.ID, Role: user.Role})
	s.tracker.Track(ctx, "user.logged_in", map[string]any{"role": string(user.Role)})
	if link != nil {
		s.google.afterLinked(ctx, user, link.Email, "password_login")
		linked = domain.ProviderGoogle
	}
	return result, linked, nil
}

// VerifyCredentials checks an e-mail and password with lockout, constant
// time dummy verification for unknown accounts and transparent rehashing.
// It records last_login_at; callers decide whether the (possibly inactive)
// account may proceed.
func (s *AuthService) VerifyCredentials(ctx context.Context, rawEmail, password string) (*domain.User, error) {
	email, ok := NormalizeEmail(rawEmail)
	if !ok || password == "" || len(password) > security.PasswordMaxLength*4 {
		if err := s.hasher.VerifyDummyContext(ctx, password); err != nil {
			return nil, hashError(err)
		}
		return nil, apperror.InvalidCredentials()
	}
	key := s.throttleKey(email)
	if n, found, err := s.kv.Get(ctx, key); err != nil {
		logger.FromContext(ctx).Error("auth: throttle lookup failed", zap.Error(err))
	} else if found {
		if count, _ := strconv.Atoi(n); count >= s.cfg.MaxFailures {
			ttl, _ := s.kv.TTL(ctx, key)
			s.tracker.Track(ctx, "user.login_locked", nil)
			return nil, apperror.AccountLocked(ttl)
		}
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			return nil, apperror.Internal(err)
		}
		// A busy hasher answers like it would for a real account, and is not a failed attempt.
		if err := s.hasher.VerifyDummyContext(ctx, password); err != nil {
			return nil, hashError(err)
		}
		s.recordFailure(ctx, key, "unknown_email")
		return nil, apperror.InvalidCredentials()
	}
	if !user.HasPassword() {
		// An account created with Google has no password: nothing can match
		// it. Same work, same answer and same lockout budget as a wrong
		// password, so the response and its timing reveal nothing.
		if err := s.hasher.VerifyDummyContext(ctx, password); err != nil {
			return nil, hashError(err)
		}
		s.recordFailure(ctx, key, "no_password")
		return nil, apperror.InvalidCredentials()
	}
	needsRehash, err := s.hasher.VerifyContext(ctx, password, user.PasswordHash)
	if hasherBusy(err) {
		return nil, hashError(err)
	}
	if err != nil {
		if !errors.Is(err, security.ErrMismatch) {
			logger.FromContext(ctx).Error("auth: stored hash unreadable", zap.String("user_id", user.ID.String()), zap.Error(err))
		}
		s.recordFailure(ctx, key, "bad_password")
		return nil, apperror.InvalidCredentials()
	}
	if !user.Active() {
		return user, nil
	}
	_ = s.kv.Del(ctx, key)

	now := s.clock.Now()
	fields := map[string]any{"last_login_at": now}
	if needsRehash {
		if h, err := s.hasher.HashContext(ctx, password); err == nil {
			fields["password_hash"] = h
		}
	}
	if err := s.users.Updates(ctx, user.ID, fields); err != nil {
		warnErr(ctx, "auth: update last login", err)
	}
	user.LastLoginAt = &now
	return user, nil
}

// RecordFailedLogin counts a failed attempt against an account's lockout
// budget (used when credentials are right but the account may not sign in
// to a privileged surface).
func (s *AuthService) RecordFailedLogin(ctx context.Context, rawEmail, reason string) {
	if email, ok := NormalizeEmail(rawEmail); ok {
		s.recordFailure(ctx, s.throttleKey(email), reason)
	}
}

func (s *AuthService) recordFailure(ctx context.Context, key, reason string) {
	n, err := s.kv.Incr(ctx, key, s.cfg.Lockout)
	if err != nil {
		logger.FromContext(ctx).Error("auth: throttle increment failed", zap.Error(err))
	}
	s.tracker.Track(ctx, "user.login_failed", map[string]any{"reason": reason, "attempt": n})
}

// issueSession creates a refresh token (new family unless given) and an
// access token.
func (s *AuthService) issueSession(ctx context.Context, user *domain.User, family uuid.UUID) (*AuthResult, error) {
	newFamily := family == uuid.Nil
	if newFamily {
		family = domain.NewID()
	}
	raw, err := security.RandomToken(32)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	meta := reqctx.MetaFrom(ctx)
	rt := &domain.RefreshToken{
		ID: domain.NewID(), UserID: user.ID, FamilyID: family, TokenHash: security.HashToken(raw),
		ExpiresAt: now.Add(s.cfg.RefreshTTL), UserAgent: truncateString(meta.UserAgent, 256),
		IPHash: security.Keyed(s.cfg.IPHashSecret, meta.IP, 32), CreatedAt: now,
	}
	if err := s.recordDevice(ctx, rt, newFamily); err != nil {
		return nil, err
	}
	if err := s.tokens.CreateRefresh(ctx, rt); err != nil {
		return nil, err
	}
	access, exp, err := s.issuer.Issue(user.ID, string(user.Role), family, user.TokenVersion)
	if err != nil {
		return nil, err
	}
	return &AuthResult{UserID: user.ID, Role: user.Role, AccessToken: access, AccessExpires: exp,
		RefreshToken: raw, RefreshExpires: rt.ExpiresAt}, nil
}

// Refresh-token rotation tolerance (revisions 3.3). A rotated token is not
// proof of theft by itself: several tabs, a retried request or a lost
// response all present a token that was just rotated.
const (
	// refreshReuseGrace: within this time of its rotation, a token is
	// honoured again and the successor stays valid (concurrent refreshes).
	refreshReuseGrace = 2 * time.Minute
	// refreshRecoveryWindow: after the grace and up to this time, a token
	// is honoured once more when no token of its family was rotated since
	// (the responses carrying its successors were lost); those successors
	// are revoked ("superseded"). Anything else is replay of a stolen token.
	// A signed-out family is never reopened, in either window.
	refreshRecoveryWindow = 10 * time.Minute

	revokeRotated    = "rotated"
	revokeSuperseded = "superseded"
)

// ErrRefreshConflict is no longer returned by Refresh (concurrent refreshes
// succeed within refreshReuseGrace); the code is kept for old clients.
var ErrRefreshConflict = apperror.New("refresh_conflict", 409, "Your session was refreshed in another tab. Please retry.")

// Refresh rotates a refresh token and issues a new access token.
func (s *AuthService) Refresh(ctx context.Context, raw string) (*AuthResult, error) {
	if raw == "" || len(raw) > 128 {
		return nil, apperror.SessionExpired()
	}
	now := s.clock.Now()
	var result *AuthResult
	var reused *domain.RefreshToken
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		rt, err := s.tokens.GetRefreshByHashForUpdate(ctx, security.HashToken(raw))
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return apperror.SessionExpired()
			}
			return err
		}
		if rt.RevokedAt != nil {
			if rt.RevokeReason != revokeRotated && rt.RevokeReason != revokeSuperseded {
				return apperror.SessionExpired()
			}
			honour, err := s.honourRotated(ctx, rt, now)
			if errors.Is(err, errRefreshFamilyEnded) {
				// Signed out (logout, password reset, device removed…): a
				// recently rotated token must never reopen the session.
				return apperror.SessionExpired()
			}
			if err != nil {
				return err
			}
			if !honour {
				reused = rt
				_, err := s.tokens.RevokeFamily(ctx, rt.FamilyID, "reuse_detected", now)
				return err
			}
		}
		if !now.Before(rt.ExpiresAt) {
			return apperror.SessionExpired()
		}
		user, err := s.users.GetByID(ctx, rt.UserID)
		if err != nil {
			return err
		}
		if !user.Active() {
			_, _ = s.tokens.RevokeFamily(ctx, rt.FamilyID, "account_disabled", now)
			return apperror.AccountDisabled()
		}
		result, err = s.issueSession(ctx, user, rt.FamilyID)
		if err != nil {
			return err
		}
		newID, err := s.tokenIDByRaw(ctx, result.RefreshToken)
		if err != nil {
			return err
		}
		if rt.RevokedAt != nil {
			return nil // already rotated: this refresh adds a sibling successor
		}
		return s.tokens.RevokeRefresh(ctx, rt.ID, revokeRotated, &newID, now)
	})
	if reused != nil {
		// Token theft suspected: the family is revoked; also invalidate
		// outstanding access tokens and close realtime connections.
		logger.FromContext(ctx).Warn("auth: refresh token reuse detected; family revoked",
			zap.String("user_id", reused.UserID.String()), zap.String("family_id", reused.FamilyID.String()))
		s.revokeAccess(ctx, reused.UserID)
		s.tracker.Track(ctx, "security.refresh_reuse", map[string]any{"user_ref": reused.UserID.String()})
		return nil, apperror.SessionExpired()
	}
	if err != nil {
		return nil, mapRepoErr(err, "Session")
	}
	return result, nil
}

// errRefreshFamilyEnded: the token's family was signed out; the token is
// simply expired (no reuse detection, never a new token).
var errRefreshFamilyEnded = errors.New("auth: refresh family ended")

// honourRotated decides whether a refresh token that was already rotated is
// still honoured (see refreshReuseGrace / refreshRecoveryWindow). The
// family's open tokens are locked (after the presented row) first:
//   - a family that was signed out is never reopened (errRefreshFamilyEnded);
//   - within the grace the token is honoured (its successors stay valid);
//   - within the recovery window it is honoured only when no token of the
//     family was rotated since (the browser never received the successors,
//     siblings included); those open successors are then superseded.
//
// false means replay (reuse detection).
func (s *AuthService) honourRotated(ctx context.Context, rt *domain.RefreshToken, now time.Time) (bool, error) {
	if rt.RevokeReason != revokeRotated {
		return false, nil // a superseded token is never honoured again
	}
	age := now.Sub(*rt.RevokedAt)
	if age >= refreshRecoveryWindow {
		return false, nil
	}
	fam, err := s.tokens.RefreshFamilyState(ctx, rt.FamilyID, rt.ID, *rt.RevokedAt)
	if err != nil {
		return false, err
	}
	if !fam.Live {
		return false, errRefreshFamilyEnded
	}
	if age < refreshReuseGrace {
		return true, nil
	}
	if fam.Advanced {
		return false, nil // the chain moved on without this token: replay
	}
	if _, err := s.tokens.SupersedeOpenRefresh(ctx, rt.FamilyID, now); err != nil {
		return false, err
	}
	return true, nil
}

func (s *AuthService) tokenIDByRaw(ctx context.Context, raw string) (uuid.UUID, error) {
	rt, err := s.tokens.GetRefreshByHashForUpdate(ctx, security.HashToken(raw))
	if err != nil {
		return uuid.Nil, err
	}
	return rt.ID, nil
}

// revokeAccess invalidates every access token issued so far for userID and
// drops its realtime connections.
func (s *AuthService) revokeAccess(ctx context.Context, userID uuid.UUID) {
	if err := s.users.Updates(ctx, userID, map[string]any{"token_version": gorm.Expr("token_version + 1")}); err != nil {
		warnErr(ctx, "auth: bump token_version", err)
	}
	s.InvalidateStatus(ctx, userID)
	warnErr(ctx, "auth: realtime revoke", s.events.Revoke(ctx, userID))
}

// Logout signs this device out: its refresh-token family is revoked, its
// access token stops working at once and its realtime connections close.
func (s *AuthService) logout(ctx context.Context, raw, pushEndpoint string) error {
	if raw == "" || len(raw) > 128 {
		return nil
	}
	var signedOut *domain.RefreshToken
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		rt, err := s.tokens.GetRefreshByHashForUpdate(ctx, security.HashToken(raw))
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		if err != nil {
			return apperror.Internal(err)
		}
		if _, err = s.tokens.RevokeFamily(ctx, rt.FamilyID, "logout", s.clock.Now()); err != nil {
			return err
		}
		signedOut = rt
		return nil
	})
	if err != nil {
		return err
	}
	if signedOut != nil {
		s.afterFamilyRevoked(ctx, signedOut.UserID, signedOut.FamilyID)
		s.endPushSubscription(ctx, signedOut.UserID, pushEndpoint)
	}
	return nil
}

// LogoutAll signs the user out of every device.
func (s *AuthService) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	if _, err := s.tokens.RevokeAllForUser(ctx, userID, "logout_all", s.clock.Now()); err != nil {
		return apperror.Internal(err)
	}
	s.endPushSessions(ctx, userID)
	s.revokeAccess(ctx, userID)
	s.tracker.Track(ctx, "user.logged_out_all", nil)
	return nil
}

// ForgotPassword e-mails a single-use reset link. It always succeeds so the
// response never reveals whether an account exists.
func (s *AuthService) ForgotPassword(ctx context.Context, rawEmail, locale string) error {
	email, ok := NormalizeEmail(rawEmail)
	if !ok {
		return apperror.Validation(map[string]string{"email": "Enter a valid email address."})
	}
	n, err := s.kv.Incr(ctx, "auth:reset:"+security.HashToken(email)[:32], time.Hour)
	if err == nil && n > 5 {
		logger.FromContext(ctx).Warn("auth: password reset throttled", zap.String("email", logger.RedactEmail(email)))
		return nil
	}
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			logger.FromContext(ctx).Error("auth: reset lookup failed", zap.Error(err))
		}
		return nil
	}
	if !user.Active() {
		return nil
	}
	raw, err := security.RandomToken(32)
	if err != nil {
		return apperror.Internal(err)
	}
	now := s.clock.Now()
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.tokens.InvalidateResets(ctx, user.ID, now); err != nil {
			return err
		}
		return s.tokens.CreateReset(ctx, &domain.PasswordResetToken{
			ID: domain.NewID(), UserID: user.ID, TokenHash: security.HashToken(raw), ExpiresAt: now.Add(s.cfg.ResetTTL), CreatedAt: now,
		})
	})
	if err != nil {
		return apperror.Internal(err)
	}
	// The e-mail follows the language chosen on the page (else the
	// account's language).
	lang := normalizeLocale(locale)
	if strings.TrimSpace(locale) == "" {
		lang = normalizeLocale(user.Locale)
	}
	link := fmt.Sprintf("%s/%s/reset-password?token=%s", s.cfg.FrontendURL, lang, raw)
	minutes := int(s.cfg.ResetTTL / time.Minute)
	// The outbox seals the link at rest and retries delivery; a failure to
	// enqueue is logged, never revealed (the response is always the same).
	uid := user.ID
	if err := s.mail.Enqueue(ctx, EmailRequest{UserID: &uid, To: user.Email, Locale: lang, Category: domain.EmailSecurity,
		Template: "password_reset", Content: mailer.PasswordReset(lang, link, minutes)}); err != nil {
		logger.FromContext(ctx).Error("auth: enqueue reset email failed", zap.Error(err))
	}
	s.tracker.Track(reqctx.WithPrincipal(ctx, reqctx.Principal{UserID: user.ID, Role: user.Role}), "password.reset_requested", nil)
	return nil
}

// ResetPassword consumes a reset token and sets a new password, signing the
// user out everywhere.
func (s *AuthService) ResetPassword(ctx context.Context, raw, password string) error {
	if raw == "" || len(raw) > 128 {
		return apperror.InvalidToken("This reset link is invalid or has expired.")
	}
	now := s.clock.Now()
	var userID uuid.UUID
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		t, err := s.tokens.GetResetByHashForUpdate(ctx, security.HashToken(raw))
		if err != nil || t.UsedAt != nil || !now.Before(t.ExpiresAt) {
			if err != nil && !errors.Is(err, repository.ErrNotFound) {
				return err
			}
			return apperror.InvalidToken("This reset link is invalid or has expired.")
		}
		user, err := s.users.GetForUpdate(ctx, t.UserID)
		if err != nil {
			return err
		}
		if msg := security.ValidatePassword(password, user.Email); msg != "" {
			return apperror.Validation(map[string]string{"password": msg})
		}
		hash, err := s.hasher.HashContext(ctx, password)
		if err != nil {
			return hashError(err)
		}
		if err := s.users.Updates(ctx, user.ID, map[string]any{
			"password_hash": hash, "token_version": gorm.Expr("token_version + 1"),
		}); err != nil {
			return err
		}
		if err := s.tokens.MarkResetUsed(ctx, t.ID, now); err != nil {
			return err
		}
		if _, err := s.tokens.RevokeAllForUser(ctx, user.ID, "password_reset", now); err != nil {
			return err
		}
		userID = user.ID
		return nil
	})
	if err != nil {
		return mapRepoErr(err, "Reset token")
	}
	s.endPushSessions(ctx, userID)
	s.InvalidateStatus(ctx, userID)
	warnErr(ctx, "auth: realtime revoke", s.events.Revoke(ctx, userID))
	s.tracker.Track(reqctx.WithPrincipal(ctx, reqctx.Principal{UserID: userID}), "password.reset_completed", nil)
	return nil
}

// ChangePassword updates the password for a signed-in user and returns a
// fresh session (all other sessions are revoked).
func (s *AuthService) ChangePassword(ctx context.Context, userID uuid.UUID, current, next string) (*AuthResult, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, mapRepoErr(err, "User")
	}
	if !user.HasPassword() {
		return nil, apperror.PasswordNotSet()
	}
	if _, err := s.hasher.VerifyContext(ctx, current, user.PasswordHash); err != nil {
		if hasherBusy(err) {
			return nil, hashError(err)
		}
		return nil, apperror.Validation(map[string]string{"current_password": "The current password is incorrect."})
	}
	if msg := security.ValidatePassword(next, user.Email); msg != "" {
		return nil, apperror.Validation(map[string]string{"new_password": msg})
	}
	if current == next {
		return nil, apperror.Validation(map[string]string{"new_password": "Choose a password different from the current one."})
	}
	hash, err := s.hasher.HashContext(ctx, next)
	if err != nil {
		return nil, hashError(err)
	}
	now := s.clock.Now()
	var result *AuthResult
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.users.Updates(ctx, user.ID, map[string]any{"password_hash": hash, "token_version": gorm.Expr("token_version + 1")}); err != nil {
			return err
		}
		if _, err := s.tokens.RevokeAllForUser(ctx, user.ID, "password_changed", now); err != nil {
			return err
		}
		fresh, err := s.users.GetByID(ctx, user.ID)
		if err != nil {
			return err
		}
		// Issued with the new token version, so only this session survives.
		result, err = s.issueSession(ctx, fresh, uuid.Nil)
		return err
	})
	if err != nil {
		return nil, mapRepoErr(err, "User")
	}
	s.endPushSessions(ctx, user.ID)
	s.InvalidateStatus(ctx, user.ID)
	warnErr(ctx, "auth: realtime revoke", s.events.Revoke(ctx, user.ID))
	s.tracker.Track(ctx, "password.changed", nil)
	return result, nil
}

// statusEntry is the cached authorization state of a user.
type statusEntry struct {
	Status       domain.UserStatus
	Role         domain.Role
	TokenVersion int
}

func statusKey(id uuid.UUID) string { return "auth:status:" + id.String() }

func (e statusEntry) encode() string {
	return string(e.Status) + "|" + string(e.Role) + "|" + strconv.Itoa(e.TokenVersion)
}

func decodeStatus(v string) (statusEntry, bool) {
	parts := strings.Split(v, "|")
	if len(parts) != 3 {
		return statusEntry{}, false
	}
	ver, err := strconv.Atoi(parts[2])
	if err != nil {
		return statusEntry{}, false
	}
	return statusEntry{Status: domain.UserStatus(parts[0]), Role: domain.Role(parts[1]), TokenVersion: ver}, true
}

// InvalidateStatus drops the cached authorization state of a user.
func (s *AuthService) InvalidateStatus(ctx context.Context, userID uuid.UUID) {
	warnErr(ctx, "auth: invalidate status cache", s.kv.Del(ctx, statusKey(userID)))
}

func (s *AuthService) status(ctx context.Context, userID uuid.UUID) (statusEntry, error) {
	if v, ok, err := s.kv.Get(ctx, statusKey(userID)); err == nil && ok {
		if e, valid := decodeStatus(v); valid {
			return e, nil
		}
	} else if err != nil {
		logger.FromContext(ctx).Warn("auth: status cache unavailable; using database", zap.Error(err))
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return statusEntry{}, apperror.Unauthenticated("")
		}
		return statusEntry{}, apperror.Internal(err)
	}
	e := statusEntry{Status: user.Status, Role: user.Role, TokenVersion: user.TokenVersion}
	warnErr(ctx, "auth: status cache set", s.kv.Set(ctx, statusKey(userID), e.encode(), s.cfg.StatusCacheTTL))
	return e, nil
}

// Authenticate validates an access token and the account's current state.
func (s *AuthService) Authenticate(ctx context.Context, token string) (reqctx.Principal, error) {
	claims, err := s.issuer.Parse(token)
	if err != nil {
		if errors.Is(err, security.ErrTokenExpired) {
			return reqctx.Principal{}, apperror.SessionExpired()
		}
		return reqctx.Principal{}, apperror.Unauthenticated("")
	}
	userID := uuid.MustParse(claims.Subject)
	family, _ := uuid.Parse(claims.FamilyID)
	st, err := s.authState(ctx, userID, family)
	if err != nil {
		return reqctx.Principal{}, err
	}
	// A signed-out device's access token dies immediately.
	if st.revoked {
		return reqctx.Principal{}, apperror.SessionExpired()
	}
	if st.Status != domain.UserActive {
		return reqctx.Principal{}, apperror.AccountDisabled()
	}
	// Revocation is exact: any bump of token_version invalidates the token.
	if claims.Version != st.TokenVersion {
		return reqctx.Principal{}, apperror.SessionExpired()
	}
	if !st.seen {
		s.markSeen(ctx, family)
	}
	return reqctx.Principal{UserID: userID, Role: st.Role, FamilyID: family}, nil
}

// AuthorizeRealtime checks that a user may (still) hold realtime
// connections from the device the ticket was issued to: a ticket fetched
// moments before that device was signed out must not open a connection.
func (s *AuthService) AuthorizeRealtime(ctx context.Context, userID, family uuid.UUID) (string, error) {
	st, err := s.authState(ctx, userID, family)
	if err != nil {
		return "", err
	}
	if st.revoked {
		return "", apperror.SessionExpired()
	}
	if st.Status != domain.UserActive {
		return "", apperror.AccountDisabled()
	}
	return string(st.Role), nil
}

// IssueRealtimeTicket returns a single-use WebSocket handshake ticket.
func (s *AuthService) IssueRealtimeTicket(ctx context.Context, p reqctx.Principal) (string, time.Duration, error) {
	ticket, err := s.realtimeTicket(ctx, p)
	if err != nil {
		return "", 0, apperror.Unavailable(err)
	}
	return ticket, s.cfg.TicketTTL, nil
}

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
