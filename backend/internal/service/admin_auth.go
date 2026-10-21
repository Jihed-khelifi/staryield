package service

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// Admin console error codes.
const (
	CodeStepUpRequired apperror.Code = "step_up_required"
	CodeInvalidCode    apperror.Code = "invalid_code"
)

// ErrStepUpRequired asks the admin to confirm a sensitive action with a
// fresh TOTP code.
func ErrStepUpRequired() *apperror.Error {
	return apperror.New(CodeStepUpRequired, http.StatusForbidden, "Confirm this action with a code from your authenticator app.")
}

func invalidCode(attemptsLeft int) *apperror.Error {
	e := apperror.New(CodeInvalidCode, http.StatusUnauthorized, "The code is incorrect or has expired.")
	if attemptsLeft >= 0 {
		e = e.WithDetails(map[string]any{"attempts_left": attemptsLeft})
	}
	return e
}

// AdminAuthSettings are the admin console session policies.
type AdminAuthSettings struct {
	SessionTTL   time.Duration
	IdleTimeout  time.Duration
	StepUpWindow time.Duration
	ChallengeTTL time.Duration
	Issuer       string
	IPHashSecret string
}

// Challenge purposes.
const (
	challengeMFA    = "mfa"
	challengeEnroll = "enroll"
	maxCodeAttempts = 5
	recoveryCodes   = 10
	// touchEvery throttles last_seen_at writes for active sessions.
	touchEvery = time.Minute
)

// LoginChallenge is returned after a correct password.
type LoginChallenge struct {
	Status    string    `json:"status"`
	Challenge string    `json:"challenge"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Enrollment carries a new TOTP secret.
type Enrollment struct {
	Secret     string `json:"secret"`
	OTPAuthURL string `json:"otpauth_url"`
}

// AdminIdentity names the signed-in administrator.
type AdminIdentity struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
}

// AdminSessionView describes the current admin session.
type AdminSessionView struct {
	Admin              AdminIdentity `json:"admin"`
	CSRFToken          string        `json:"csrf_token,omitempty"`
	SessionID          uuid.UUID     `json:"session_id"`
	ExpiresAt          time.Time     `json:"expires_at"`
	IdleExpiresAt      time.Time     `json:"idle_expires_at"`
	IdleTimeoutSeconds int           `json:"idle_timeout_seconds"`
	StepUpUntil        time.Time     `json:"step_up_until"`
	RecoveryCodesLeft  int           `json:"recovery_codes_left"`
	RecoveryCodes      []string      `json:"recovery_codes,omitempty"`
}

// AdminSessionRow lists an admin's active sessions.
type AdminSessionRow struct {
	ID         uuid.UUID `json:"id"`
	Current    bool      `json:"current"`
	UserAgent  string    `json:"user_agent"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// IssuedAdminSession is a new session with its raw cookie token.
type IssuedAdminSession struct {
	Token   string
	View    *AdminSessionView
	Expires time.Time
}

// AdminContext is the authenticated admin session attached to a request.
type AdminContext struct {
	Session *domain.AdminSession
	User    *domain.User
}

type adminCtxKey struct{}

// WithAdmin stores the admin session in ctx.
func WithAdmin(ctx context.Context, a *AdminContext) context.Context {
	return context.WithValue(ctx, adminCtxKey{}, a)
}

// AdminFrom returns the admin session of the request, if any.
func AdminFrom(ctx context.Context) (*AdminContext, bool) {
	a, ok := ctx.Value(adminCtxKey{}).(*AdminContext)
	return a, ok && a != nil
}

// AdminAuthService authenticates administrators with password + TOTP and
// manages server-side admin console sessions.
type AdminAuthService struct {
	tx      database.TxManager
	auth    *AuthService
	users   repository.UserRepository
	admins  repository.AdminRepository
	audit   repository.AuditRepository
	kv      cache.Store
	sealer  *security.Sealer
	tracker analytics.Tracker
	clock   clock.Clock
	cfg     AdminAuthSettings
}

// NewAdminAuthService wires the admin authentication service.
func NewAdminAuthService(tx database.TxManager, auth *AuthService, users repository.UserRepository, admins repository.AdminRepository,
	audit repository.AuditRepository, kv cache.Store, sealer *security.Sealer, tracker analytics.Tracker, clk clock.Clock,
	cfg AdminAuthSettings) *AdminAuthService {
	return &AdminAuthService{tx: tx, auth: auth, users: users, admins: admins, audit: audit, kv: kv, sealer: sealer,
		tracker: tracker, clock: clk, cfg: cfg}
}

func (s *AdminAuthService) recordAudit(ctx context.Context, actor uuid.UUID, action string, meta map[string]any) {
	m := reqctx.MetaFrom(ctx)
	if meta == nil {
		meta = map[string]any{}
	}
	if m.UserAgent != "" {
		meta["user_agent"] = truncateString(m.UserAgent, 120)
	}
	if err := s.audit.Create(ctx, &domain.AuditLog{ID: domain.NewID(), ActorID: &actor, Action: action, TargetType: "admin",
		TargetID: actor.String(), Metadata: domain.JSONMap(meta), IPHash: security.Keyed(s.cfg.IPHashSecret, m.IP, 32),
		RequestID: m.RequestID, CreatedAt: s.clock.Now()}); err != nil {
		logger.FromContext(ctx).Error("admin auth: audit write failed", zap.Error(err), zap.String("action", action))
	}
}

func challengeKey(token string) string { return "adm:ch:" + security.HashToken(token) }
func enrollKey(token string) string    { return "adm:enr:" + security.HashToken(token) }

// Login verifies the password of an administrator and returns a second
// factor challenge. Any other account gets the generic credentials error.
func (s *AdminAuthService) Login(ctx context.Context, email, password string) (*LoginChallenge, error) {
	user, err := s.auth.VerifyCredentials(ctx, email, password)
	if err != nil {
		return nil, err
	}
	if user.Role != domain.RoleAdmin || !user.Active() {
		// Correct password on a non-admin or disabled account: count it as a
		// failure so the admin surface cannot be used to probe accounts.
		s.auth.RecordFailedLogin(ctx, email, "admin_not_allowed")
		s.tracker.Track(ctx, "admin.login_denied", map[string]any{"role": string(user.Role), "status": string(user.Status)})
		return nil, apperror.InvalidCredentials()
	}
	purpose := challengeMFA
	if _, err := s.admins.GetMFA(ctx, user.ID); errors.Is(err, repository.ErrNotFound) {
		purpose = challengeEnroll
	} else if err != nil {
		return nil, apperror.Internal(err)
	}
	token, err := security.RandomToken(32)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if err := s.kv.Set(ctx, challengeKey(token), user.ID.String()+"|"+purpose+"|0", s.cfg.ChallengeTTL); err != nil {
		return nil, apperror.Unavailable(err)
	}
	status := "mfa_required"
	if purpose == challengeEnroll {
		status = "enrollment_required"
	}
	s.recordAudit(ctx, user.ID, "admin.password_verified", map[string]any{"next": status})
	return &LoginChallenge{Status: status, Challenge: token, ExpiresAt: s.clock.Now().Add(s.cfg.ChallengeTTL)}, nil
}

type challenge struct {
	userID   uuid.UUID
	purpose  string
	attempts int
}

func (s *AdminAuthService) loadChallenge(ctx context.Context, token, purpose string) (*challenge, error) {
	if token == "" || len(token) > 128 {
		return nil, apperror.InvalidToken("Your sign-in attempt expired. Please sign in again.")
	}
	raw, ok, err := s.kv.Get(ctx, challengeKey(token))
	if err != nil {
		return nil, apperror.Unavailable(err)
	}
	if !ok {
		return nil, apperror.InvalidToken("Your sign-in attempt expired. Please sign in again.")
	}
	parts := strings.Split(raw, "|")
	if len(parts) != 3 {
		return nil, apperror.InvalidToken("Your sign-in attempt expired. Please sign in again.")
	}
	id, err := uuid.Parse(parts[0])
	if err != nil || parts[1] != purpose {
		return nil, apperror.InvalidToken("Your sign-in attempt expired. Please sign in again.")
	}
	n, _ := strconv.Atoi(parts[2])
	return &challenge{userID: id, purpose: parts[1], attempts: n}, nil
}

// failChallenge counts a wrong code; the challenge dies after the limit.
func (s *AdminAuthService) failChallenge(ctx context.Context, token string, ch *challenge) error {
	ch.attempts++
	left := maxCodeAttempts - ch.attempts
	if left <= 0 {
		_ = s.kv.Del(ctx, challengeKey(token), enrollKey(token))
		s.recordAudit(ctx, ch.userID, "admin.mfa_locked", nil)
		return apperror.InvalidToken("Too many incorrect codes. Please sign in again.")
	}
	ttl, err := s.kv.TTL(ctx, challengeKey(token))
	if err != nil || ttl <= 0 {
		ttl = s.cfg.ChallengeTTL
	}
	warnErr(ctx, "admin auth: update challenge", s.kv.Set(ctx, challengeKey(token),
		ch.userID.String()+"|"+ch.purpose+"|"+strconv.Itoa(ch.attempts), ttl))
	s.recordAudit(ctx, ch.userID, "admin.mfa_failed", map[string]any{"attempts_left": left})
	return invalidCode(left)
}

func (s *AdminAuthService) activeAdmin(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperror.InvalidToken("Your sign-in attempt expired. Please sign in again.")
		}
		return nil, apperror.Internal(err)
	}
	if u.Role != domain.RoleAdmin || !u.Active() {
		return nil, apperror.InvalidCredentials()
	}
	return u, nil
}

// StartEnrollment creates (or returns the pending) TOTP secret for a
// first-time admin.
func (s *AdminAuthService) StartEnrollment(ctx context.Context, token string) (*Enrollment, error) {
	ch, err := s.loadChallenge(ctx, token, challengeEnroll)
	if err != nil {
		return nil, err
	}
	u, err := s.activeAdmin(ctx, ch.userID)
	if err != nil {
		return nil, err
	}
	var secret string
	if sealed, ok, err := s.kv.Get(ctx, enrollKey(token)); err == nil && ok {
		secret, _ = s.sealer.Open(sealed, u.ID.String())
	}
	if secret == "" {
		if secret, err = security.NewTOTPSecret(); err != nil {
			return nil, apperror.Internal(err)
		}
		sealed, err := s.sealer.Seal(secret, u.ID.String())
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if err := s.kv.Set(ctx, enrollKey(token), sealed, s.cfg.ChallengeTTL); err != nil {
			return nil, apperror.Unavailable(err)
		}
	}
	return &Enrollment{Secret: secret, OTPAuthURL: security.TOTPURI(s.cfg.Issuer, u.Email, secret)}, nil
}

// ConfirmEnrollment verifies the first code, stores the encrypted secret
// and recovery codes, and opens a session.
func (s *AdminAuthService) ConfirmEnrollment(ctx context.Context, token, code string) (*IssuedAdminSession, error) {
	ch, err := s.loadChallenge(ctx, token, challengeEnroll)
	if err != nil {
		return nil, err
	}
	u, err := s.activeAdmin(ctx, ch.userID)
	if err != nil {
		return nil, err
	}
	sealed, ok, err := s.kv.Get(ctx, enrollKey(token))
	if err != nil {
		return nil, apperror.Unavailable(err)
	}
	if !ok {
		return nil, apperror.InvalidState("Start the authenticator setup first.")
	}
	secret, err := s.sealer.Open(sealed, u.ID.String())
	if err != nil {
		return nil, apperror.Internal(err)
	}
	now := s.clock.Now()
	step, valid := security.VerifyTOTP(secret, code, now, 0)
	if !valid {
		return nil, s.failChallenge(ctx, token, ch)
	}
	codes, hashes, err := newRecoverySet()
	if err != nil {
		return nil, apperror.Internal(err)
	}
	stored, err := s.sealer.Seal(secret, u.ID.String())
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var issued *IssuedAdminSession
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if existing, err := s.admins.GetMFAForUpdate(ctx, u.ID); err == nil && existing != nil {
			return apperror.Conflict(apperror.CodeConflict, "Two-step verification is already set up for this account.")
		} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		if err := s.admins.SaveMFA(ctx, &domain.AdminMFA{UserID: u.ID, SecretEnc: stored, LastUsedStep: step,
			RecoveryHashes: hashes, EnabledAt: now}); err != nil {
			return err
		}
		var err error
		issued, err = s.openSession(ctx, u, len(hashes))
		return err
	})
	if err != nil {
		return nil, mapRepoErr(err, "Admin")
	}
	_ = s.kv.Del(ctx, challengeKey(token), enrollKey(token))
	issued.View.RecoveryCodes = codes
	s.recordAudit(ctx, u.ID, "admin.mfa_enrolled", nil)
	s.recordAudit(ctx, u.ID, "admin.logged_in", map[string]any{"method": "totp_enrollment"})
	return issued, nil
}

func newRecoverySet() ([]string, domain.StringList, error) {
	codes, err := security.NewRecoveryCodes(recoveryCodes)
	if err != nil {
		return nil, nil, err
	}
	hashes := make(domain.StringList, 0, len(codes))
	for _, c := range codes {
		hashes = append(hashes, security.HashToken("recovery:"+c))
	}
	return codes, hashes, nil
}

// Verify completes sign-in with a TOTP code or a single-use recovery code.
func (s *AdminAuthService) Verify(ctx context.Context, token, code, recovery string) (*IssuedAdminSession, error) {
	ch, err := s.loadChallenge(ctx, token, challengeMFA)
	if err != nil {
		return nil, err
	}
	u, err := s.activeAdmin(ctx, ch.userID)
	if err != nil {
		return nil, err
	}
	if (code == "") == (recovery == "") {
		return nil, apperror.Validation(map[string]string{"code": "Enter the 6-digit code or a recovery code."})
	}
	var issued *IssuedAdminSession
	method := "totp"
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		mfa, err := s.admins.GetMFAForUpdate(ctx, u.ID)
		if err != nil {
			return err
		}
		if recovery != "" {
			method = "recovery_code"
			if !consumeRecovery(mfa, recovery) {
				return errInvalidSecondFactor
			}
		} else {
			secret, err := s.sealer.Open(mfa.SecretEnc, u.ID.String())
			if err != nil {
				return err
			}
			step, ok := security.VerifyTOTP(secret, code, s.clock.Now(), mfa.LastUsedStep)
			if !ok {
				return errInvalidSecondFactor
			}
			mfa.LastUsedStep = step
		}
		if err := s.admins.SaveMFA(ctx, mfa); err != nil {
			return err
		}
		issued, err = s.openSession(ctx, u, len(mfa.RecoveryHashes))
		return err
	})
	if errors.Is(err, errInvalidSecondFactor) {
		return nil, s.failChallenge(ctx, token, ch)
	}
	if err != nil {
		return nil, mapRepoErr(err, "Admin")
	}
	_ = s.kv.Del(ctx, challengeKey(token))
	s.recordAudit(ctx, u.ID, "admin.logged_in", map[string]any{"method": method})
	return issued, nil
}

var errInvalidSecondFactor = errors.New("admin auth: invalid second factor")

// consumeRecovery removes a matching recovery code hash (single use).
func consumeRecovery(mfa *domain.AdminMFA, code string) bool {
	c := security.NormalizeRecoveryCode(code)
	if c == "" {
		return false
	}
	want := security.HashToken("recovery:" + c)
	for i, h := range mfa.RecoveryHashes {
		if h == want {
			mfa.RecoveryHashes = append(mfa.RecoveryHashes[:i:i], mfa.RecoveryHashes[i+1:]...)
			return true
		}
	}
	return false
}

// openSession creates a server-side session (MFA verified now).
func (s *AdminAuthService) openSession(ctx context.Context, u *domain.User, recoveryLeft int) (*IssuedAdminSession, error) {
	raw, err := security.RandomToken(32)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	meta := reqctx.MetaFrom(ctx)
	sess := &domain.AdminSession{ID: domain.NewID(), UserID: u.ID, TokenHash: security.HashToken(raw),
		IPHash: security.Keyed(s.cfg.IPHashSecret, meta.IP, 32), UserAgent: truncateString(meta.UserAgent, 256),
		CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(s.cfg.SessionTTL), MFAAt: now}
	if err := s.admins.CreateSession(ctx, sess); err != nil {
		return nil, err
	}
	return &IssuedAdminSession{Token: raw, View: s.view(u, sess, recoveryLeft), Expires: sess.ExpiresAt}, nil
}

func (s *AdminAuthService) view(u *domain.User, sess *domain.AdminSession, recoveryLeft int) *AdminSessionView {
	idle := sess.LastSeenAt.Add(s.cfg.IdleTimeout)
	if idle.After(sess.ExpiresAt) {
		idle = sess.ExpiresAt
	}
	return &AdminSessionView{
		Admin:     AdminIdentity{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName},
		SessionID: sess.ID, ExpiresAt: sess.ExpiresAt, IdleExpiresAt: idle,
		IdleTimeoutSeconds: int(s.cfg.IdleTimeout / time.Second), StepUpUntil: sess.MFAAt.Add(s.cfg.StepUpWindow),
		RecoveryCodesLeft: recoveryLeft,
	}
}

// Authenticate resolves an admin session cookie. Sessions expire after the
// idle timeout or the absolute lifetime; the account must still be an
// active administrator.
func (s *AdminAuthService) Authenticate(ctx context.Context, token string) (*AdminContext, error) {
	if token == "" || len(token) > 128 {
		return nil, apperror.Unauthenticated("")
	}
	sess, err := s.admins.GetSessionByHash(ctx, security.HashToken(token))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperror.Unauthenticated("")
		}
		return nil, apperror.Internal(err)
	}
	now := s.clock.Now()
	switch {
	case sess.RevokedAt != nil:
		return nil, apperror.SessionExpired()
	case !now.Before(sess.ExpiresAt):
		return nil, apperror.SessionExpired()
	case now.Sub(sess.LastSeenAt) >= s.cfg.IdleTimeout:
		warnErr(ctx, "admin auth: revoke idle session", s.admins.RevokeSession(ctx, sess.ID, "idle_timeout", now))
		return nil, apperror.SessionExpired()
	}
	u, err := s.users.GetByID(ctx, sess.UserID)
	if err != nil {
		return nil, mapRepoErr(err, "Admin")
	}
	if u.Role != domain.RoleAdmin || !u.Active() {
		warnErr(ctx, "admin auth: revoke session", s.admins.RevokeSession(ctx, sess.ID, "account_changed", now))
		return nil, apperror.SessionExpired()
	}
	if now.Sub(sess.LastSeenAt) >= touchEvery {
		warnErr(ctx, "admin auth: touch session", s.admins.TouchSession(ctx, sess.ID, now))
		sess.LastSeenAt = now
	}
	return &AdminContext{Session: sess, User: u}, nil
}

// AuthenticateAdmin implements middleware.AdminAuthenticator: it returns a
// context carrying the admin session and principal.
func (s *AdminAuthService) AuthenticateAdmin(ctx context.Context, token string) (context.Context, error) {
	a, err := s.Authenticate(ctx, token)
	if err != nil {
		return nil, err
	}
	ctx = WithAdmin(ctx, a)
	return reqctx.WithPrincipal(ctx, reqctx.Principal{UserID: a.User.ID, Role: domain.RoleAdmin}), nil
}

// RequireStepUp enforces a recent second-factor verification.
func (s *AdminAuthService) RequireStepUp(ctx context.Context) error {
	a, ok := AdminFrom(ctx)
	if !ok {
		return apperror.Unauthenticated("")
	}
	if s.clock.Now().Sub(a.Session.MFAAt) > s.cfg.StepUpWindow {
		return ErrStepUpRequired()
	}
	return nil
}

// Session returns the current session view.
func (s *AdminAuthService) Session(ctx context.Context) (*AdminSessionView, error) {
	a, ok := AdminFrom(ctx)
	if !ok {
		return nil, apperror.Unauthenticated("")
	}
	left := 0
	if mfa, err := s.admins.GetMFA(ctx, a.User.ID); err == nil {
		left = len(mfa.RecoveryHashes)
	}
	return s.view(a.User, a.Session, left), nil
}

// StepUp re-verifies a TOTP code for sensitive actions.
func (s *AdminAuthService) StepUp(ctx context.Context, code string) (*AdminSessionView, error) {
	a, ok := AdminFrom(ctx)
	if !ok {
		return nil, apperror.Unauthenticated("")
	}
	key := "adm:stepup:" + a.Session.ID.String()
	if n, found, err := s.kv.Get(ctx, key); err == nil && found {
		if count, _ := strconv.Atoi(n); count >= maxCodeAttempts {
			ttl, _ := s.kv.TTL(ctx, key)
			return nil, apperror.RateLimited(ttl)
		}
	}
	now := s.clock.Now()
	left := 0
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		mfa, err := s.admins.GetMFAForUpdate(ctx, a.User.ID)
		if err != nil {
			return err
		}
		secret, err := s.sealer.Open(mfa.SecretEnc, a.User.ID.String())
		if err != nil {
			return err
		}
		step, ok := security.VerifyTOTP(secret, code, now, mfa.LastUsedStep)
		if !ok {
			return errInvalidSecondFactor
		}
		mfa.LastUsedStep = step
		left = len(mfa.RecoveryHashes)
		if err := s.admins.SaveMFA(ctx, mfa); err != nil {
			return err
		}
		return s.admins.SetSessionMFA(ctx, a.Session.ID, now)
	})
	if errors.Is(err, errInvalidSecondFactor) {
		n, _ := s.kv.Incr(ctx, key, 15*time.Minute)
		s.recordAudit(ctx, a.User.ID, "admin.step_up_failed", nil)
		return nil, invalidCode(maxCodeAttempts - int(n))
	}
	if err != nil {
		return nil, mapRepoErr(err, "Admin")
	}
	_ = s.kv.Del(ctx, key)
	a.Session.MFAAt, a.Session.LastSeenAt = now, now
	s.recordAudit(ctx, a.User.ID, "admin.step_up", nil)
	return s.view(a.User, a.Session, left), nil
}

// Logout revokes the current session.
func (s *AdminAuthService) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	sess, err := s.admins.GetSessionByHash(ctx, security.HashToken(token))
	if errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	if err != nil {
		return apperror.Internal(err)
	}
	if err := s.admins.RevokeSession(ctx, sess.ID, "logout", s.clock.Now()); err != nil {
		return apperror.Internal(err)
	}
	s.recordAudit(ctx, sess.UserID, "admin.logged_out", nil)
	return nil
}

// Sessions lists the admin's active sessions.
func (s *AdminAuthService) Sessions(ctx context.Context) ([]AdminSessionRow, error) {
	a, ok := AdminFrom(ctx)
	if !ok {
		return nil, apperror.Unauthenticated("")
	}
	rows, err := s.admins.ListActiveSessions(ctx, a.User.ID, s.clock.Now())
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]AdminSessionRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, AdminSessionRow{ID: r.ID, Current: r.ID == a.Session.ID, UserAgent: r.UserAgent,
			CreatedAt: r.CreatedAt, LastSeenAt: r.LastSeenAt, ExpiresAt: r.ExpiresAt})
	}
	return out, nil
}

// RevokeSession ends one of the admin's own sessions.
func (s *AdminAuthService) RevokeSession(ctx context.Context, id uuid.UUID) error {
	a, ok := AdminFrom(ctx)
	if !ok {
		return apperror.Unauthenticated("")
	}
	rows, err := s.admins.ListActiveSessions(ctx, a.User.ID, s.clock.Now())
	if err != nil {
		return apperror.Internal(err)
	}
	for _, r := range rows {
		if r.ID == id {
			if err := s.admins.RevokeSession(ctx, id, "revoked_by_owner", s.clock.Now()); err != nil {
				return apperror.Internal(err)
			}
			s.recordAudit(ctx, a.User.ID, "admin.session_revoked", map[string]any{"session_id": id.String()})
			return nil
		}
	}
	return apperror.NotFound("Session")
}

// RegenerateRecoveryCodes replaces the recovery codes (step-up required).
func (s *AdminAuthService) RegenerateRecoveryCodes(ctx context.Context) ([]string, error) {
	a, ok := AdminFrom(ctx)
	if !ok {
		return nil, apperror.Unauthenticated("")
	}
	codes, hashes, err := newRecoverySet()
	if err != nil {
		return nil, apperror.Internal(err)
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		mfa, err := s.admins.GetMFAForUpdate(ctx, a.User.ID)
		if err != nil {
			return err
		}
		mfa.RecoveryHashes = hashes
		return s.admins.SaveMFA(ctx, mfa)
	})
	if err != nil {
		return nil, mapRepoErr(err, "Admin")
	}
	s.recordAudit(ctx, a.User.ID, "admin.recovery_codes_regenerated", nil)
	return codes, nil
}

// RevokeAllFor ends every admin session of a user (role change, ban,
// deactivation, deletion).
func (s *AdminAuthService) RevokeAllFor(ctx context.Context, userID uuid.UUID, reason string) {
	if _, err := s.admins.RevokeUserSessions(ctx, userID, reason, s.clock.Now()); err != nil {
		warnErr(ctx, "admin auth: revoke sessions", err)
	}
}

// ResetMFA removes an admin's second factor so they re-enrol at next
// sign-in (used when an account stops being an admin).
func (s *AdminAuthService) ResetMFA(ctx context.Context, userID uuid.UUID) {
	warnErr(ctx, "admin auth: reset mfa", s.admins.DeleteMFA(ctx, userID))
}

// HasMFA reports whether the user enrolled a second factor.
func (s *AdminAuthService) HasMFA(ctx context.Context, userID uuid.UUID) bool {
	_, err := s.admins.GetMFA(ctx, userID)
	return err == nil
}
