package service

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/i18n"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	mailer "github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/oidc"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// Sign in with Google (revisions 4.0, docs/platform-extensions.md §23.1).
//
// The flow is the server-side authorization-code flow with PKCE. A sign-in
// attempt is a single-use Redis record (state), bound to the browser that
// started it through the HttpOnly cookie sy_oa. A callback that needs the
// person to do something more (accept the terms, prove the password of an
// existing account, reactivate) parks a short-lived "pending" record under
// the hash of that same cookie. Google is never trusted to link accounts
// by e-mail: an existing account is linked only after its password is
// proven, or from a signed-in session.

// Notification kinds raised by linking and unlinking Google (in-app only).
const (
	NotifyAccountGoogleLinked   = "account.google_linked"
	NotifyAccountGoogleUnlinked = "account.google_unlinked"
)

// Sign-in intents (POST /auth/google/start).
const (
	GoogleIntentSignin = "signin"
	GoogleIntentSignup = "signup"
	GoogleIntentLink   = "link"
)

// Callback results (the result= parameter of the redirect).
const (
	GoogleResultSignedIn         = "signed_in"
	GoogleResultSignedUp         = "signed_up"
	GoogleResultLinked           = "linked"
	GoogleResultSignupConsent    = "signup_consent"
	GoogleResultLinkPassword     = "link_password"
	GoogleResultReactivation     = "reactivation"
	GoogleResultAccountDisabled  = "account_disabled"
	GoogleResultCancelled        = "google_cancelled"
	GoogleResultStateInvalid     = "google_state_invalid"
	GoogleResultEmailUnverified  = "google_email_unverified"
	GoogleResultAccountInUse     = "google_account_in_use"
	GoogleResultNotAllowed       = "google_not_allowed"
	GoogleResultUnavailable      = "google_unavailable"
	googlePendingConsent         = "signup_consent"
	googlePendingLinkPassword    = "link_password"
	googlePendingReactivation    = "reactivation"
	googleStatePrefix            = "google:state:"
	googlePendingPrefix          = "google:pending:"
	googleMaxCookieLen           = 128
	googleMaxStateLen            = 128
	googleMaxAttributionJSONSize = 4 << 10
)

// GoogleSettings configure the service.
type GoogleSettings struct {
	// Enabled mirrors the configuration (client id, secret and redirect URL).
	Enabled bool
	// StateTTL is the lifetime of an attempt and of a pending record.
	StateTTL     time.Duration
	FrontendURL  string
	IPHashSecret string
}

// GoogleDeps are the service's collaborators.
type GoogleDeps struct {
	Auth       *AuthService
	Provider   oidc.Provider // nil: unavailable
	Identities repository.IdentityRepository
	Users      repository.UserRepository
	Audit      repository.AuditRepository
	KV         cache.Store
	Notifier   Notifier
	Mail       EmailQueue
	Clock      clock.Clock
	Settings   GoogleSettings
}

// GoogleAuth implements Sign in with Google.
type GoogleAuth struct {
	d   GoogleDeps
	cfg GoogleSettings
}

// NewGoogleAuth wires the service and registers it with the auth service
// (password sign-in links a pending Google identity).
func NewGoogleAuth(d GoogleDeps) *GoogleAuth {
	if d.Notifier == nil {
		d.Notifier = NoopNotifier{}
	}
	if d.Settings.StateTTL <= 0 {
		d.Settings.StateTTL = 10 * time.Minute
	}
	g := &GoogleAuth{d: d, cfg: d.Settings}
	d.Auth.google = g
	return g
}

// Enabled reports whether Google sign-in is configured and usable.
func (g *GoogleAuth) Enabled() bool { return g != nil && g.cfg.Enabled && g.d.Provider != nil }

// StateTTL is the lifetime of a sign-in attempt (also the cookie's).
func (g *GoogleAuth) StateTTL() time.Duration { return g.cfg.StateTTL }

// ---------------------------------------------------------------------------
// Records
// ---------------------------------------------------------------------------

// googleState is a sign-in attempt, stored under the hash of its state.
type googleState struct {
	Nonce       string            `json:"nonce"`
	Verifier    string            `json:"verifier"`
	Intent      string            `json:"intent"`
	Locale      string            `json:"locale"`
	Next        string            `json:"next,omitempty"`
	AcceptTerms bool              `json:"accept_terms,omitempty"`
	Attribution *AttributionInput `json:"attribution,omitempty"`
	UserID      *uuid.UUID        `json:"user_id,omitempty"` // intent link
	// Family is the device (session) that started a link: it must still be
	// signed in when Google calls back.
	Family     *uuid.UUID `json:"family,omitempty"`
	CookieHash string     `json:"cookie_hash"`
	Expires    int64      `json:"exp"`
}

// googlePending is what the sy_oa cookie is waiting for.
type googlePending struct {
	Kind        string            `json:"kind"`
	Subject     string            `json:"sub,omitempty"`
	Email       string            `json:"email,omitempty"`
	UserID      *uuid.UUID        `json:"user_id,omitempty"`
	Locale      string            `json:"locale"`
	Next        string            `json:"next,omitempty"`
	Attribution *AttributionInput `json:"attribution,omitempty"`
	Expires     int64             `json:"exp"`
}

func stateKey(state string) string           { return googleStatePrefix + security.HashToken(state) }
func pendingKey(cookie string) string        { return googlePendingPrefix + security.HashToken(cookie) }
func validCookieValue(v string) bool         { return v != "" && len(v) <= googleMaxCookieLen }
func validStateValue(v string) bool          { return v != "" && len(v) <= googleMaxStateLen }
func (g *GoogleAuth) now() time.Time         { return g.d.Clock.Now() }
func (g *GoogleAuth) expiry() int64          { return g.now().Add(g.cfg.StateTTL).Unix() }
func (g *GoogleAuth) expired(exp int64) bool { return g.now().Unix() > exp }

// take reads and deletes a key atomically (GETDEL).
func (g *GoogleAuth) take(ctx context.Context, key string) (string, bool, error) {
	t, ok := g.d.KV.(cache.Taker)
	if !ok {
		return "", false, errors.New("google: the cache cannot delete atomically")
	}
	return t.GetDel(ctx, key)
}

func (g *GoogleAuth) putPending(ctx context.Context, cookie string, p *googlePending) error {
	p.Expires = g.expiry()
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return g.d.KV.Set(ctx, pendingKey(cookie), string(b), g.cfg.StateTTL)
}

// peekPending reads the pending record of a browser without consuming it.
func (g *GoogleAuth) peekPending(ctx context.Context, cookie string) (*googlePending, error) {
	if !validCookieValue(cookie) {
		return nil, nil
	}
	raw, found, err := g.d.KV.Get(ctx, pendingKey(cookie))
	if err != nil || !found {
		return nil, err
	}
	return g.decodePending(raw), nil
}

func (g *GoogleAuth) decodePending(raw string) *googlePending {
	var p googlePending
	if json.Unmarshal([]byte(raw), &p) != nil || g.expired(p.Expires) {
		return nil
	}
	return &p
}

// ---------------------------------------------------------------------------
// Start
// ---------------------------------------------------------------------------

// GoogleStartInput starts a sign-in attempt.
type GoogleStartInput struct {
	Intent      string
	AcceptTerms bool
	Next        string
	Locale      string
	Attribution *AttributionInput
	// Principal is the signed-in caller (intent link only).
	Principal *reqctx.Principal
}

// GoogleStartResult is where to send the browser, and the cookie that binds
// the attempt to it.
type GoogleStartResult struct {
	AuthorizationURL string
	// Cookie is the value of sy_oa; the service keeps only its hash.
	Cookie string
	TTL    time.Duration
}

// Start records a sign-in attempt and returns Google's authorization URL.
func (g *GoogleAuth) Start(ctx context.Context, in GoogleStartInput) (*GoogleStartResult, error) {
	if !g.Enabled() {
		return nil, apperror.GoogleUnavailable(errors.New("google sign-in is not configured"))
	}
	switch in.Intent {
	case GoogleIntentSignin, GoogleIntentSignup, GoogleIntentLink:
	default:
		return nil, apperror.Validation(map[string]string{"intent": "Choose signin, signup or link."})
	}
	rec := googleState{Intent: in.Intent, Locale: normalizeLocale(in.Locale), Next: SanitizeNext(in.Next),
		AcceptTerms: in.AcceptTerms && in.Intent == GoogleIntentSignup, Expires: g.expiry()}
	if in.Intent == GoogleIntentLink {
		if in.Principal == nil || in.Principal.UserID == uuid.Nil {
			return nil, apperror.Unauthenticated("")
		}
		u, err := g.d.Users.GetByID(ctx, in.Principal.UserID)
		if err != nil {
			return nil, mapRepoErr(err, "User")
		}
		if u.Role == domain.RoleAdmin {
			return nil, apperror.GoogleNotAllowed()
		}
		if !u.Active() {
			return nil, apperror.AccountDisabled()
		}
		rec.UserID = &u.ID
		if in.Principal.FamilyID != uuid.Nil {
			fam := in.Principal.FamilyID
			rec.Family = &fam
		}
	}
	if in.Attribution != nil {
		if b, err := json.Marshal(in.Attribution); err == nil && len(b) <= googleMaxAttributionJSONSize {
			rec.Attribution = in.Attribution
		}
	}
	state, err1 := oidc.RandomToken(32)
	nonce, err2 := oidc.RandomToken(32)
	verifier, err3 := oidc.NewVerifier()
	cookie, err4 := oidc.RandomToken(32)
	if err := errors.Join(err1, err2, err3, err4); err != nil {
		return nil, apperror.Internal(err)
	}
	rec.Nonce, rec.Verifier, rec.CookieHash = nonce, verifier, security.HashToken(cookie)
	authURL, err := g.d.Provider.AuthorizationURL(ctx, oidc.AuthParams{State: state, Nonce: nonce, CodeChallenge: oidc.Challenge(verifier),
		Locale: rec.Locale})
	if err != nil {
		return nil, apperror.GoogleUnavailable(err)
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if err := g.d.KV.Set(ctx, stateKey(state), string(b), g.cfg.StateTTL); err != nil {
		return nil, apperror.Unavailable(err)
	}
	g.d.Auth.tracker.Track(ctx, "auth.google_started", map[string]any{"intent": in.Intent})
	return &GoogleStartResult{AuthorizationURL: authURL, Cookie: cookie, TTL: g.cfg.StateTTL}, nil
}

// ---------------------------------------------------------------------------
// Callback
// ---------------------------------------------------------------------------

// GoogleCallbackInput is what the browser brings back from Google.
type GoogleCallbackInput struct {
	Code  string
	State string
	// Error is Google's error parameter (access_denied…).
	Error string
	// Cookie is the value of sy_oa.
	Cookie string
	// Locale is the request's language, used when no attempt is found.
	Locale string
}

// GoogleCallbackOutcome says where the browser goes next. The callback
// itself never fails: every problem is a result.
type GoogleCallbackOutcome struct {
	Result string
	Locale string
	Next   string
	// Auth is set for signed_in and signed_up: the session to install.
	Auth *AuthResult
	// Created reports a new account; the caller runs the post-registration
	// steps (attribution, calculator link) with Attribution.
	Created     bool
	UserID      uuid.UUID
	Attribution *AttributionInput
	// KeepCookie keeps sy_oa: a pending step (consent, link, reactivation) waits
	// for it, or the callback consumed nothing of this browser's.
	KeepCookie bool
}

// googleIdentity is a validated Google identity within its attempt.
type googleFlow struct {
	Subject     string
	Email       string
	Intent      string
	AcceptTerms bool
	Locale      string
	Next        string
	Attribution *AttributionInput
	UserID      *uuid.UUID
	Family      *uuid.UUID
	Cookie      string
}

// takeState validates and consumes a sign-in attempt. The cookie check comes
// first and does not consume: a request without the right cookie (a forged
// callback, a link scanner) must not burn the genuine attempt.
func (g *GoogleAuth) takeState(ctx context.Context, state, cookie string) *googleState {
	if !validStateValue(state) || !validCookieValue(cookie) {
		return nil
	}
	key := stateKey(state)
	raw, found, err := g.d.KV.Get(ctx, key)
	if err != nil || !found {
		return nil
	}
	var peek googleState
	if json.Unmarshal([]byte(raw), &peek) != nil ||
		subtle.ConstantTimeCompare([]byte(peek.CookieHash), []byte(security.HashToken(cookie))) != 1 {
		return nil
	}
	taken, ok, err := g.take(ctx, key) // single use: exactly one caller wins
	if err != nil {
		logger.FromContext(ctx).Error("google: take state", zap.Error(err))
		return nil
	}
	if !ok {
		return nil
	}
	var rec googleState
	if json.Unmarshal([]byte(taken), &rec) != nil || g.expired(rec.Expires) {
		return nil
	}
	return &rec
}

// Callback completes a sign-in attempt. It never returns an error.
func (g *GoogleAuth) Callback(ctx context.Context, in GoogleCallbackInput) *GoogleCallbackOutcome {
	out := &GoogleCallbackOutcome{Locale: normalizeLocale(in.Locale)}
	fail := func(result string) *GoogleCallbackOutcome { out.Result = result; return out }
	if !g.Enabled() {
		return fail(GoogleResultUnavailable)
	}
	rec := g.takeState(ctx, in.State, in.Cookie)
	if rec == nil {
		// Nothing of this browser's was consumed: keep its sy_oa, so a
		// forged callback cannot wipe the cookie of an attempt in progress.
		out.KeepCookie = true
		return fail(GoogleResultStateInvalid)
	}
	out.Locale, out.Next, out.Attribution = rec.Locale, rec.Next, rec.Attribution
	if in.Error != "" {
		switch in.Error {
		case "temporarily_unavailable", "server_error":
			return fail(GoogleResultUnavailable)
		}
		return fail(GoogleResultCancelled)
	}
	if in.Code == "" {
		return fail(GoogleResultStateInvalid)
	}
	idn, err := g.d.Provider.Exchange(ctx, oidc.ExchangeParams{Code: in.Code, CodeVerifier: rec.Verifier, Nonce: rec.Nonce})
	switch {
	case errors.Is(err, oidc.ErrEmailUnverified):
		return fail(GoogleResultEmailUnverified)
	case errors.Is(err, oidc.ErrUnavailable):
		logger.FromContext(ctx).Warn("google: provider unavailable", zap.Error(err))
		return fail(GoogleResultUnavailable)
	case err != nil:
		logger.FromContext(ctx).Warn("google: sign-in rejected", zap.Error(err))
		return fail(GoogleResultStateInvalid)
	}
	email, ok := NormalizeEmail(idn.Email)
	if !ok || idn.Subject == "" {
		logger.FromContext(ctx).Warn("google: unusable identity")
		return fail(GoogleResultStateInvalid)
	}
	f := &googleFlow{Subject: idn.Subject, Email: email, Intent: rec.Intent, AcceptTerms: rec.AcceptTerms, Locale: rec.Locale,
		Next: rec.Next, Attribution: rec.Attribution, UserID: rec.UserID, Family: rec.Family, Cookie: in.Cookie}
	if rec.Intent == GoogleIntentLink {
		g.link(ctx, f, out)
	} else {
		g.signInOrUp(ctx, f, out)
	}
	return out
}

// errRaced reports that a concurrent request created the same account or
// identity first; the caller resolves the identity again.
var errRaced = errors.New("google: concurrent sign-up")

func (g *GoogleAuth) internalFailure(ctx context.Context, out *GoogleCallbackOutcome, err error) {
	logger.FromContext(ctx).Error("google: sign-in failed", zap.Error(err))
	out.Result = GoogleResultUnavailable
}

// signInOrUp resolves a Google identity for the sign-in and sign-up intents.
func (g *GoogleAuth) signInOrUp(ctx context.Context, f *googleFlow, out *GoogleCallbackOutcome) {
	for attempt := 0; attempt < 2; attempt++ {
		err := g.resolve(ctx, f, out)
		if errors.Is(err, errRaced) {
			continue // the identity (or the address) now exists: look again
		}
		if err != nil {
			g.internalFailure(ctx, out, err)
		}
		return
	}
	g.internalFailure(ctx, out, errors.New("google: sign-up kept racing"))
}

func (g *GoogleAuth) resolve(ctx context.Context, f *googleFlow, out *GoogleCallbackOutcome) error {
	ident, err := g.d.Identities.GetBySubject(ctx, domain.ProviderGoogle, f.Subject)
	if err == nil {
		return g.signInKnown(ctx, ident, f, out)
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	// Unknown Google identity. The address may belong to an existing
	// account, but it is never linked on that basis alone.
	existing, err := g.d.Users.GetByEmail(ctx, f.Email)
	switch {
	case err == nil:
		// The account may have been created by this very identity a moment
		// ago (a concurrent callback): its identity row commits together
		// with the user, so it is visible now.
		if ident, err := g.d.Identities.GetBySubject(ctx, domain.ProviderGoogle, f.Subject); err == nil {
			return g.signInKnown(ctx, ident, f, out)
		} else if !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		if existing.Role == domain.RoleAdmin {
			out.Result = GoogleResultNotAllowed
			return nil
		}
		return g.park(ctx, f, out, &googlePending{Kind: googlePendingLinkPassword, Subject: f.Subject, Email: f.Email, UserID: &existing.ID},
			GoogleResultLinkPassword)
	case !errors.Is(err, repository.ErrNotFound):
		return err
	}
	if f.Intent != GoogleIntentSignup || !f.AcceptTerms {
		return g.park(ctx, f, out, &googlePending{Kind: googlePendingConsent, Subject: f.Subject, Email: f.Email}, GoogleResultSignupConsent)
	}
	acct, err := g.createFromGoogle(ctx, f)
	if err != nil {
		return err
	}
	out.Result, out.Auth, out.Created, out.UserID = GoogleResultSignedUp, acct.Result, true, acct.User.ID
	return nil
}

// park stores what the browser must do next and answers with result.
func (g *GoogleAuth) park(ctx context.Context, f *googleFlow, out *GoogleCallbackOutcome, p *googlePending, result string) error {
	p.Locale, p.Next, p.Attribution = f.Locale, f.Next, f.Attribution
	if err := g.putPending(ctx, f.Cookie, p); err != nil {
		return err
	}
	out.Result, out.KeepCookie = result, true
	return nil
}

// signInKnown signs in the owner of a linked Google identity.
func (g *GoogleAuth) signInKnown(ctx context.Context, ident *domain.UserIdentity, f *googleFlow, out *GoogleCallbackOutcome) error {
	u, err := g.d.Users.GetByID(ctx, ident.UserID)
	if err != nil {
		return err
	}
	if u.Role == domain.RoleAdmin {
		out.Result = GoogleResultNotAllowed
		return nil
	}
	if !u.Active() {
		g.d.Auth.tracker.Track(ctx, "user.login_failed", map[string]any{"reason": string(u.Status), "method": "google"})
		if g.canReactivate(u) {
			return g.park(ctx, f, out, &googlePending{Kind: googlePendingReactivation, UserID: &u.ID}, GoogleResultReactivation)
		}
		out.Result = GoogleResultAccountDisabled
		return nil
	}
	now := g.now()
	var res *AuthResult
	err = g.d.Auth.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := g.d.Users.Updates(ctx, u.ID, map[string]any{"last_login_at": now}); err != nil {
			return err
		}
		if err := g.d.Identities.Touch(ctx, ident.ID, f.Email, now); err != nil {
			return err
		}
		var err error
		res, err = g.d.Auth.issueSession(ctx, u, uuid.Nil)
		return err
	})
	if err != nil {
		return err
	}
	out.Result, out.Auth, out.UserID = GoogleResultSignedIn, res, u.ID
	g.d.Auth.tracker.Track(reqctx.WithPrincipal(ctx, reqctx.Principal{UserID: u.ID, Role: u.Role}), "user.logged_in",
		map[string]any{"role": string(u.Role), "method": "google"})
	return nil
}

// canReactivate: the owner paused (or scheduled the closure of) the account
// and the reactivation offer is available.
func (g *GoogleAuth) canReactivate(u *domain.User) bool {
	return g.d.Auth.ident != nil && u.Status == domain.UserDeactivated &&
		(u.DeactivatedBy == domain.DeactivatedBySelf || u.DeactivatedBy == domain.DeactivatedByClosure)
}

// createFromGoogle creates the account of a new Google identity through the
// same code path as POST /auth/register (no password, verified address).
func (g *GoogleAuth) createFromGoogle(ctx context.Context, f *googleFlow) (*createdAccount, error) {
	now := g.now()
	acct, err := g.d.Auth.createAccount(ctx, newAccount{
		Email: f.Email, PasswordHash: "", DisplayName: defaultDisplayName(f.Email), Locale: f.Locale, EmailVerified: true,
		Provider: domain.ProviderGoogle,
		InTx: func(ctx context.Context, user *domain.User) error {
			return g.d.Identities.Create(ctx, &domain.UserIdentity{ID: domain.NewID(), UserID: user.ID, Provider: domain.ProviderGoogle,
				Subject: f.Subject, Email: f.Email, LastLoginAt: &now, CreatedAt: now, UpdatedAt: now})
		},
	})
	if err != nil {
		if errors.Is(err, repository.ErrIdentityTaken) {
			return nil, errRaced
		}
		if ae, ok := apperror.As(err); ok && ae.Code == apperror.CodeEmailTaken {
			return nil, errRaced
		}
		return nil, err
	}
	g.record(reqctx.WithPrincipal(ctx, reqctx.Principal{UserID: acct.User.ID, Role: acct.User.Role}), "auth.google.signup", acct.User.ID,
		map[string]any{"email": maskGoogleEmail(f.Email)})
	return acct, nil
}

func (g *GoogleAuth) record(ctx context.Context, action string, userID uuid.UUID, meta map[string]any) {
	recordAudit(ctx, g.d.Audit, g.d.Clock, g.cfg.IPHashSecret, action, "user", userID.String(), meta)
}

// link connects the Google identity to the signed-in account of the attempt.
func (g *GoogleAuth) link(ctx context.Context, f *googleFlow, out *GoogleCallbackOutcome) {
	if f.UserID == nil {
		out.Result = GoogleResultStateInvalid
		return
	}
	u, err := g.d.Users.GetByID(ctx, *f.UserID)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		out.Result = GoogleResultStateInvalid
		return
	case err != nil:
		g.internalFailure(ctx, out, err)
		return
	case u.Role == domain.RoleAdmin:
		out.Result = GoogleResultNotAllowed
		return
	case !u.Active():
		out.Result = GoogleResultAccountDisabled
		return
	}
	// Whoever completes Google's screen must not be able to attach their
	// Google account to a user who signed out meanwhile: the device that
	// started the link has to be still signed in.
	if f.Family != nil {
		st, err := g.d.Auth.authState(ctx, u.ID, *f.Family)
		if err != nil {
			g.internalFailure(ctx, out, err)
			return
		}
		if st.revoked {
			out.Result = GoogleResultStateInvalid
			return
		}
	}
	result, err := g.linkIdentity(ctx, u, f.Subject, f.Email)
	if err != nil {
		g.internalFailure(ctx, out, err)
		return
	}
	out.Result, out.UserID = result, u.ID
	if result == GoogleResultLinked {
		g.afterLinked(ctx, u, f.Email, "settings")
	}
}

// linkIdentity adds the identity to u unless it is already linked
// (idempotent for the same subject) or taken (replaces nothing). It returns
// the callback result.
func (g *GoogleAuth) linkIdentity(ctx context.Context, u *domain.User, subject, email string) (string, error) {
	if other, err := g.d.Identities.GetBySubject(ctx, domain.ProviderGoogle, subject); err == nil {
		if other.UserID != u.ID {
			return GoogleResultAccountInUse, nil
		}
		return GoogleResultLinked, g.d.Identities.Touch(ctx, other.ID, email, g.now())
	} else if !errors.Is(err, repository.ErrNotFound) {
		return "", err
	}
	if _, err := g.d.Identities.GetForUser(ctx, u.ID, domain.ProviderGoogle); err == nil {
		return GoogleResultAccountInUse, nil // a different Google account is connected: nothing is replaced
	} else if !errors.Is(err, repository.ErrNotFound) {
		return "", err
	}
	now := g.now()
	err := g.d.Identities.Create(ctx, &domain.UserIdentity{ID: domain.NewID(), UserID: u.ID, Provider: domain.ProviderGoogle,
		Subject: subject, Email: email, LastLoginAt: &now, CreatedAt: now, UpdatedAt: now})
	if errors.Is(err, repository.ErrIdentityTaken) {
		return GoogleResultAccountInUse, nil
	}
	if err != nil {
		return "", err
	}
	return GoogleResultLinked, nil
}

// afterLinked raises the audit entry, the in-app notification and the
// security e-mail of a new Google connection.
func (g *GoogleAuth) afterLinked(ctx context.Context, u *domain.User, googleEmail, via string) {
	ctx = reqctx.WithPrincipal(ctx, reqctx.Principal{UserID: u.ID, Role: u.Role})
	g.record(ctx, "auth.google.linked", u.ID, map[string]any{"via": via, "email": maskGoogleEmail(googleEmail)})
	g.d.Notifier.Notify(ctx, NotificationSpec{Kind: NotifyAccountGoogleLinked, Data: map[string]any{"email": maskGoogleEmail(googleEmail)},
		Link: "/profile/settings#account"}, u.ID)
	lang := normalizeLocale(u.Locale)
	g.sendSecurityMail(ctx, u, "google_linked", mailer.GoogleLinked(lang, maskGoogleEmail(googleEmail), g.settingsLink(lang)))
	g.d.Auth.tracker.Track(ctx, "account.google_linked", map[string]any{"via": via})
}

func (g *GoogleAuth) afterUnlinked(ctx context.Context, u *domain.User) {
	g.record(ctx, "auth.google.unlinked", u.ID, nil)
	g.d.Notifier.Notify(ctx, NotificationSpec{Kind: NotifyAccountGoogleUnlinked, Data: map[string]any{"email": maskGoogleEmail(u.Email)},
		Link: "/profile/settings#account"}, u.ID)
	lang := normalizeLocale(u.Locale)
	g.sendSecurityMail(ctx, u, "google_unlinked", mailer.GoogleUnlinked(lang, g.settingsLink(lang)))
	g.d.Auth.tracker.Track(ctx, "account.google_unlinked", nil)
}

func (g *GoogleAuth) settingsLink(lang string) string {
	return mailer.AppURL(g.cfg.FrontendURL, lang, "/profile/settings#account")
}

func (g *GoogleAuth) sendSecurityMail(ctx context.Context, u *domain.User, template string, c mailer.Content) {
	if g.d.Mail == nil {
		return
	}
	uid := u.ID
	if err := g.d.Mail.Enqueue(ctx, EmailRequest{UserID: &uid, To: u.Email, Locale: normalizeLocale(u.Locale),
		Category: domain.EmailSecurity, Template: template, Content: c}); err != nil {
		logger.FromContext(ctx).Error("google: enqueue security e-mail", zap.String("template", template), zap.Error(err))
	}
}

// RedirectURL is where the callback sends the browser:
// <frontend>/<locale>/auth/google?result=<result>[&next=<path>].
func (g *GoogleAuth) RedirectURL(out *GoogleCallbackOutcome) string {
	u := strings.TrimRight(g.cfg.FrontendURL, "/") + "/" + i18n.Normalize(out.Locale) + "/auth/google?result=" + url.QueryEscape(out.Result)
	if next := SanitizeNext(out.Next); next != "" {
		u += "&next=" + url.QueryEscape(next)
	}
	return u
}

// ---------------------------------------------------------------------------
// Pending step: view, consent
// ---------------------------------------------------------------------------

// GooglePendingView is what the sy_oa cookie is waiting for.
type GooglePendingView struct {
	Kind  string `json:"kind"`
	Email string `json:"email,omitempty"`
	// Reactivation only.
	Code                  string     `json:"code,omitempty"`
	ReactivationToken     string     `json:"reactivation_token,omitempty"`
	ReactivationExpiresAt *time.Time `json:"reactivation_expires_at,omitempty"`
	DeletionScheduledAt   *time.Time `json:"deletion_scheduled_at,omitempty"`
}

// Pending describes the pending step of the browser holding cookie; 404
// when there is none.
func (g *GoogleAuth) Pending(ctx context.Context, cookie string) (*GooglePendingView, error) {
	p, err := g.peekPending(ctx, cookie)
	if err != nil {
		return nil, apperror.Unavailable(err)
	}
	if p == nil {
		return nil, apperror.NotFound("Pending sign-in")
	}
	switch p.Kind {
	case googlePendingConsent, googlePendingLinkPassword:
		return &GooglePendingView{Kind: p.Kind, Email: p.Email}, nil
	case googlePendingReactivation:
		if p.UserID == nil {
			return nil, apperror.NotFound("Pending sign-in")
		}
		u, err := g.d.Users.GetByID(ctx, *p.UserID)
		if err != nil {
			return nil, mapRepoErr(err, "Pending sign-in")
		}
		// A fresh offer on every read (the token lives only minutes).
		offer := g.d.Auth.offerReactivation(ctx, u)
		if offer == nil {
			_ = g.d.KV.Del(ctx, pendingKey(cookie))
			return nil, apperror.NotFound("Pending sign-in") // reactivated meanwhile
		}
		ae, ok := apperror.As(offer)
		if !ok || (ae.Code != apperror.CodeAccountDeactivated && ae.Code != apperror.CodeClosurePending) {
			return nil, offer
		}
		v := &GooglePendingView{Kind: p.Kind, Code: string(ae.Code)}
		v.ReactivationToken, _ = ae.Details["reactivation_token"].(string)
		if t, ok := ae.Details["reactivation_expires_at"].(time.Time); ok {
			v.ReactivationExpiresAt = &t
		}
		if t, ok := ae.Details["deletion_scheduled_at"].(time.Time); ok {
			v.DeletionScheduledAt = &t
		}
		return v, nil
	}
	return nil, apperror.NotFound("Pending sign-in")
}

// GoogleCompleteInput accepts the terms for a pending sign-up.
type GoogleCompleteInput struct {
	AcceptTerms bool
	Locale      string
	Attribution *AttributionInput
	Cookie      string
}

// GoogleCompleteResult is the new account's session.
type GoogleCompleteResult struct {
	Auth        *AuthResult
	UserID      uuid.UUID
	Locale      string
	Attribution *AttributionInput
	// Created is false when the identity already had an account (a
	// concurrent request created it) and the person was simply signed in.
	Created bool
}

// Complete creates the account of a pending signup_consent (single use).
func (g *GoogleAuth) Complete(ctx context.Context, in GoogleCompleteInput) (*GoogleCompleteResult, error) {
	if !g.Enabled() {
		return nil, apperror.GoogleUnavailable(errors.New("google sign-in is not configured"))
	}
	p, err := g.peekPending(ctx, in.Cookie)
	if err != nil {
		return nil, apperror.Unavailable(err)
	}
	if p == nil || p.Kind != googlePendingConsent {
		return nil, apperror.GoogleStateInvalid()
	}
	if !in.AcceptTerms {
		return nil, apperror.Validation(map[string]string{"accept_terms": "Please accept the Terms of Service and Privacy Policy."})
	}
	raw, ok, err := g.take(ctx, pendingKey(in.Cookie)) // single use
	if err != nil {
		return nil, apperror.Unavailable(err)
	}
	if !ok {
		return nil, apperror.GoogleStateInvalid()
	}
	if p = g.decodePending(raw); p == nil || p.Kind != googlePendingConsent {
		return nil, apperror.GoogleStateInvalid()
	}
	locale := p.Locale
	if strings.TrimSpace(in.Locale) != "" {
		locale = normalizeLocale(in.Locale)
	}
	attribution := p.Attribution
	if in.Attribution != nil {
		attribution = in.Attribution
	}
	f := &googleFlow{Subject: p.Subject, Email: p.Email, Locale: locale, Attribution: attribution}
	acct, err := g.createFromGoogle(ctx, f)
	if errors.Is(err, errRaced) {
		// Another request created this identity (or the address was
		// registered meanwhile): sign the owner in, or report the address.
		if ident, gerr := g.d.Identities.GetBySubject(ctx, domain.ProviderGoogle, p.Subject); gerr == nil {
			out := &GoogleCallbackOutcome{}
			if err := g.signInKnown(ctx, ident, f, out); err != nil {
				return nil, apperror.Internal(err)
			}
			if out.Auth == nil {
				return nil, apperror.GoogleStateInvalid()
			}
			return &GoogleCompleteResult{Auth: out.Auth, UserID: out.UserID, Locale: locale, Attribution: attribution}, nil
		}
		return nil, apperror.Conflict(apperror.CodeEmailTaken, "An account with this email already exists.")
	}
	if err != nil {
		return nil, mapRepoErr(err, "User")
	}
	return &GoogleCompleteResult{Auth: acct.Result, UserID: acct.User.ID, Locale: locale, Attribution: attribution, Created: true}, nil
}

// ---------------------------------------------------------------------------
// Link on password sign-in
// ---------------------------------------------------------------------------

// googleLink is a Google identity ready to be linked by a password sign-in.
type googleLink struct {
	Subject string
	Email   string
}

// takeLink consumes the pending link_password of the browser when it is for
// this account (same e-mail) and the Google identity is still free. The
// password was already verified by the caller. It returns nil when the
// sign-in has nothing to link.
func (g *GoogleAuth) takeLink(ctx context.Context, u *domain.User, cookie string) *googleLink {
	if u.Role == domain.RoleAdmin || !g.Enabled() {
		return nil
	}
	p, err := g.peekPending(ctx, cookie)
	if err != nil || p == nil || p.Kind != googlePendingLinkPassword || p.Email != u.Email || p.Subject == "" {
		return nil
	}
	raw, ok, err := g.take(ctx, pendingKey(cookie)) // single use
	if err != nil || !ok {
		return nil
	}
	if p = g.decodePending(raw); p == nil || p.Kind != googlePendingLinkPassword || p.Email != u.Email {
		return nil
	}
	// A Google identity linked elsewhere meanwhile, or a Google account
	// already on this account, is not replaced: the sign-in just proceeds.
	if _, err := g.d.Identities.GetBySubject(ctx, domain.ProviderGoogle, p.Subject); !errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	if _, err := g.d.Identities.GetForUser(ctx, u.ID, domain.ProviderGoogle); !errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	return &googleLink{Subject: p.Subject, Email: p.Email}
}

// linkInTx links the identity inside the sign-in transaction.
func (g *GoogleAuth) linkInTx(ctx context.Context, u *domain.User, l *googleLink) error {
	now := g.now()
	return g.d.Identities.Create(ctx, &domain.UserIdentity{ID: domain.NewID(), UserID: u.ID, Provider: domain.ProviderGoogle,
		Subject: l.Subject, Email: l.Email, LastLoginAt: &now, CreatedAt: now, UpdatedAt: now})
}

// ---------------------------------------------------------------------------
// Sign-in methods
// ---------------------------------------------------------------------------

// GoogleMethodView is the connected Google account.
type GoogleMethodView struct {
	Email    string    `json:"email"`
	LinkedAt time.Time `json:"linked_at"`
}

// SignInMethodsView lists how an account can sign in.
type SignInMethodsView struct {
	Password        bool              `json:"password"`
	Google          *GoogleMethodView `json:"google,omitempty"`
	GoogleAvailable bool              `json:"google_available"`
}

// SignInMethods returns the caller's sign-in methods.
func (g *GoogleAuth) SignInMethods(ctx context.Context, userID uuid.UUID) (*SignInMethodsView, error) {
	u, err := g.d.Users.GetByID(ctx, userID)
	if err != nil {
		return nil, mapRepoErr(err, "User")
	}
	v := &SignInMethodsView{Password: u.HasPassword(), GoogleAvailable: g.Enabled() && u.Role != domain.RoleAdmin}
	switch ident, err := g.d.Identities.GetForUser(ctx, userID, domain.ProviderGoogle); {
	case err == nil:
		v.Google = &GoogleMethodView{Email: ident.Email, LinkedAt: ident.CreatedAt}
	case !errors.Is(err, repository.ErrNotFound):
		return nil, apperror.Internal(err)
	}
	return v, nil
}

// Disconnect removes the caller's Google identity after the password is
// confirmed. Without a password Google is the only way in: it stays.
func (g *GoogleAuth) Disconnect(ctx context.Context, userID uuid.UUID, password string) error {
	u, err := g.d.Users.GetByID(ctx, userID)
	if err != nil {
		return mapRepoErr(err, "User")
	}
	if _, err := g.d.Identities.GetForUser(ctx, userID, domain.ProviderGoogle); err != nil {
		return mapRepoErr(err, "Sign-in method")
	}
	if !u.HasPassword() {
		return apperror.LastSignInMethod()
	}
	if password == "" || len(password) > security.PasswordMaxLength*4 {
		if err := g.d.Auth.hasher.VerifyDummyContext(ctx, password); err != nil {
			return hashError(err)
		}
		return apperror.InvalidCredentials()
	}
	if _, err := g.d.Auth.hasher.VerifyContext(ctx, password, u.PasswordHash); err != nil {
		if hasherBusy(err) {
			return hashError(err)
		}
		return apperror.InvalidCredentials()
	}
	if err := g.d.Identities.Delete(ctx, userID, domain.ProviderGoogle); err != nil {
		return mapRepoErr(err, "Sign-in method")
	}
	g.afterUnlinked(ctx, u)
	return nil
}

// SignInMethodNames lists the methods of an account for the admin console.
func SignInMethodNames(hasPassword bool, identities []domain.UserIdentity) []string {
	out := []string{}
	if hasPassword {
		out = append(out, "password")
	}
	for _, i := range identities {
		out = append(out, i.Provider)
	}
	return out
}
