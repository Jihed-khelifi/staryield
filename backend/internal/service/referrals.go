package service

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// Growth: the referral loop. Every client can invite friends with a personal
// link. For each invited friend who signs up with it, verifies the e-mail
// address and completes a reading, the inviter earns free minutes, added by
// the grant job (referrals_grants.go). The rules, in order:
//
//   - a code is 8 characters of an unambiguous alphabet, created on first use;
//   - Attribute (called after every sign-up) records the friend as "joined"
//     when the code belongs to an active client other than the new account and
//     the program is enabled; sign-ups that look suspicious are flagged for an
//     administrator instead of being rewarded automatically;
//   - Evaluate (called when a reading settles and when an address is verified,
//     in either order) moves a joined friend to "qualified" once both are true,
//     or to "rejected" (cap_reached) when the inviter's cap is used up;
//   - the grant job adds the minutes of a qualified, unheld referral within a
//     minute (it is woken by the qualification) and tells the inviter then.
//
// Privacy: an inviter only ever sees counts and notifications that name nobody.

const (
	// referralCodeLength and referralAlphabet: 30 symbols, no 0, O, 1, I, L or U.
	referralCodeLength = 8
	referralAlphabet   = "ABCDEFGHJKMNPQRSTVWXYZ23456789"

	// referralBurstLimit is how many friends may join one inviter in
	// referralBurstWindow before further sign-ups are held for review.
	referralBurstLimit  = 10
	referralBurstWindow = 24 * time.Hour
	// referralNetworkWindow is how far back an inviter's device counts as
	// "the same network" as a new sign-up.
	referralNetworkWindow = 30 * 24 * time.Hour

	// referralCodeAttempts bounds the retries of a code collision.
	referralCodeAttempts = 8
)

var referralCodePattern = regexp.MustCompile(`^[` + referralAlphabet + `]{8}$`)

// ReferralSettings configure the referral service.
type ReferralSettings struct {
	FrontendURL  string
	IPHashSecret string
}

// ReferralDeps are the referral service's collaborators.
type ReferralDeps struct {
	Tx    database.TxManager
	Repo  repository.ReferralRepository
	Audit repository.AuditRepository
	// Wallet grants the free minutes.
	Wallet *WalletService
	// Program reads the program setting (SiteService.ReferralProgram).
	Program  func(ctx context.Context) ReferralProgram
	Notifier Notifier
	Events   EventPublisher
	Tracker  analytics.Tracker
	Clock    clock.Clock
	Cfg      ReferralSettings
	// NewCode returns a candidate code; tests inject collisions. Default: random.
	NewCode func() string
}

// ReferralService owns the referral rules.
type ReferralService struct {
	d    ReferralDeps
	wake chan struct{}
}

// NewReferralService wires the referral service.
func NewReferralService(d ReferralDeps) *ReferralService {
	if d.Notifier == nil {
		d.Notifier = NoopNotifier{}
	}
	if d.Events == nil {
		d.Events = NoopPublisher{}
	}
	if d.Tracker == nil {
		d.Tracker = analytics.Noop{}
	}
	if d.NewCode == nil {
		d.NewCode = randomReferralCode
	}
	return &ReferralService{d: d, wake: make(chan struct{}, 1)}
}

// Wake is signalled when rewards may be ready to grant (the job runs early).
func (s *ReferralService) Wake() <-chan struct{} { return s.wake }

func (s *ReferralService) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *ReferralService) record(ctx context.Context, action, targetType, targetID string, meta map[string]any) {
	if s.d.Audit == nil {
		return
	}
	recordAudit(ctx, s.d.Audit, s.d.Clock, s.d.Cfg.IPHashSecret, action, targetType, targetID, meta)
}

// randomReferralCode draws a code uniformly from the alphabet.
func randomReferralCode() string {
	max := big.NewInt(int64(len(referralAlphabet)))
	b := make([]byte, referralCodeLength)
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic("referrals: no randomness: " + err.Error())
		}
		b[i] = referralAlphabet[n.Int64()]
	}
	return string(b)
}

// NormalizeReferralCode trims and upper-cases a code typed or pasted by a
// person; ok is false when it cannot be a code.
func NormalizeReferralCode(in string) (string, bool) {
	code := strings.ToUpper(strings.TrimSpace(in))
	return code, referralCodePattern.MatchString(code)
}

// EnsureCode returns the user's code, creating it on first use. A collision
// with another user's code retries with a new one; two concurrent first calls
// of the same user end with the same code.
func (s *ReferralService) EnsureCode(ctx context.Context, userID uuid.UUID) (string, error) {
	code, err := s.d.Repo.CodeOf(ctx, userID)
	if err == nil {
		return code, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return "", apperror.Internal(err)
	}
	for range referralCodeAttempts {
		candidate := s.d.NewCode()
		created, err := s.d.Repo.CreateCode(ctx, userID, candidate, s.d.Clock.Now())
		if err != nil {
			return "", apperror.Internal(err)
		}
		if created {
			return candidate, nil
		}
		// Nothing stored: either this user got a code meanwhile, or the
		// candidate belongs to someone else.
		if code, err := s.d.Repo.CodeOf(ctx, userID); err == nil {
			return code, nil
		}
	}
	return "", apperror.Internal(errors.New("referrals: no free code after retries"))
}

// ---------------------------------------------------------------------------
// Joining
// ---------------------------------------------------------------------------

// Attribute records a new account as invited when the sign-up carried a valid
// referral code. It never fails the sign-up: invalid codes, a disabled
// program or any error are logged at debug or warn level and ignored.
func (s *ReferralService) Attribute(ctx context.Context, userID uuid.UUID, in *AttributionInput) {
	if s == nil || in == nil || strings.TrimSpace(in.ReferralCode) == "" {
		return
	}
	log := logger.FromContext(ctx)
	code, ok := NormalizeReferralCode(in.ReferralCode)
	if !ok {
		log.Debug("referrals: ignored a malformed code")
		return
	}
	if !s.d.Program(ctx).Enabled {
		log.Debug("referrals: program disabled; sign-up not credited")
		return
	}
	inviter, err := s.d.Repo.InviterByCode(ctx, code)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			warnErr(ctx, "referrals: look up the code", err)
		} else {
			log.Debug("referrals: ignored an unknown code")
		}
		return
	}
	if inviter == userID {
		return
	}
	now := s.d.Clock.Now()
	ipHash := security.Keyed(s.d.Cfg.IPHashSecret, reqctx.MetaFrom(ctx).IP, 32)
	ref := &domain.Referral{ID: domain.NewID(), ReferrerID: inviter, RefereeID: &userID, Code: code, Status: domain.ReferralJoined,
		Flags: domain.StringList{}, SignupIPHash: ipHash, JoinedAt: now, CreatedAt: now, UpdatedAt: now}

	// The browser the account was created in: first linked to the inviter
	// (self-referral), or to another account (one person, several accounts).
	// The sign-up itself links a fresh browser to the new account first.
	owner := uuid.Nil
	if ValidAnonymousID(in.AnonymousID) {
		owner, err = s.d.Repo.VisitorOwner(ctx, in.AnonymousID)
		warnErr(ctx, "referrals: check the browser link", err)
	}
	if owner == inviter {
		ref.Status, ref.RejectReason = domain.ReferralRejected, domain.ReferralRejectSelf
	} else {
		if owner != uuid.Nil && owner != userID {
			ref.Flags = append(ref.Flags, domain.ReferralFlagSharedBrowser)
		}
		shared, err := s.d.Repo.SharesSessionNetwork(ctx, inviter, ipHash, now.Add(-referralNetworkWindow))
		warnErr(ctx, "referrals: check the network", err)
		if shared {
			ref.Flags = append(ref.Flags, domain.ReferralFlagSameNetwork)
		}
		recent, err := s.d.Repo.JoinedSince(ctx, inviter, now.Add(-referralBurstWindow))
		warnErr(ctx, "referrals: count recent sign-ups", err)
		if recent >= referralBurstLimit {
			ref.Flags = append(ref.Flags, domain.ReferralFlagSignupBurst)
		}
		ref.Flagged = len(ref.Flags) > 0
	}
	created, err := s.d.Repo.Create(ctx, ref)
	if err != nil {
		warnErr(ctx, "referrals: record the sign-up", err)
		return
	}
	if !created {
		return
	}
	s.d.Tracker.Track(ctx, "referral.joined", map[string]any{"flagged": ref.Flagged, "status": string(ref.Status)})
	if ref.Status == domain.ReferralJoined && !ref.Flagged {
		s.d.Notifier.Notify(ctx, NotificationSpec{Kind: NotifyReferralJoined, Data: map[string]any{}, Link: referralLink}, inviter)
	}
	// A friend who is already verified (Google) and somehow already has a
	// reading is evaluated by the sweep; nothing to do here.
}

// ---------------------------------------------------------------------------
// Qualifying
// ---------------------------------------------------------------------------

// OnSessionSettled is the ChatService.OnSessionSettled hook: a settled
// reading with at least one billed or free minute may complete a referral.
func (s *ReferralService) OnSessionSettled(ctx context.Context, sess domain.Session) {
	if sess.BilledMinutes <= 0 && sess.FreeMinutesUsed <= 0 {
		return
	}
	s.Evaluate(ctx, sess.ClientID)
}

// OnEmailVerified is called when a user's e-mail address becomes verified
// (the e-mail verification stream wires it): the referral may now qualify.
func (s *ReferralService) OnEmailVerified(ctx context.Context, userID uuid.UUID) {
	s.Evaluate(ctx, userID)
}

// Evaluate moves the friend's referral from joined to qualified (or rejected
// when the inviter's cap is used up) when the friend's address is verified and
// they have a qualifying reading. It reads both conditions from the database,
// so the order of the two events does not matter, and it is idempotent: the
// referral row is locked and only a joined row changes. Errors are logged.
func (s *ReferralService) Evaluate(ctx context.Context, refereeID uuid.UUID) {
	if s == nil {
		return
	}
	// One indexed read keeps the common case (a reading by someone nobody
	// invited) at a single cheap query.
	pre, err := s.d.Repo.ByReferee(ctx, refereeID)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			warnErr(ctx, "referrals: look up the friend", err)
		}
		return
	}
	if pre.Status != domain.ReferralJoined {
		return
	}
	prog := s.d.Program(ctx)
	if !prog.Enabled {
		return // rewards pause with the program; the sweep resumes them
	}
	var after *domain.Referral
	err = s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		after = nil
		r, err := s.d.Repo.ByRefereeForUpdate(ctx, refereeID)
		if err != nil {
			return err
		}
		if r.Status != domain.ReferralJoined {
			return nil
		}
		verified, err := s.d.Repo.EmailVerified(ctx, refereeID)
		if err != nil || !verified {
			return err
		}
		session, err := s.d.Repo.QualifyingSession(ctx, refereeID)
		if err != nil || session == nil {
			return err
		}
		// Serialize the inviter's decisions so two friends qualifying at the
		// same moment cannot both take the last place under the cap.
		if err := s.d.Repo.LockInviter(ctx, r.ReferrerID); err != nil {
			return err
		}
		rewarded, err := s.d.Repo.Rewarded(ctx, r.ReferrerID)
		if err != nil {
			return err
		}
		now := s.d.Clock.Now()
		if rewarded >= int64(prog.MaxRewardsPerReferrer) {
			// Closed: no longer waits for a review (the flags stay as history).
			r.Status, r.RejectReason, r.Flagged = domain.ReferralRejected, domain.ReferralRejectCap, false
		} else {
			r.Status, r.RewardMinutes, r.QualifiedAt, r.FirstSessionID = domain.ReferralQualified, prog.RewardMinutes, &now, session
		}
		if err := s.d.Repo.Save(ctx, r); err != nil {
			return err
		}
		after = r
		return nil
	})
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			warnErr(ctx, "referrals: evaluate", err, zap.String("referee_id", refereeID.String()))
		}
		return
	}
	if after != nil && after.Status == domain.ReferralQualified {
		s.afterQualified(ctx, after)
	}
}

// afterQualified records a qualified referral (an analytics event, not a
// notification) and wakes the grant job, which adds the minutes within a
// minute and tells the inviter then. A flagged referral waits for an
// administrator's decision: the grant job skips it until it is approved.
func (s *ReferralService) afterQualified(ctx context.Context, r *domain.Referral) {
	s.d.Tracker.Track(ctx, "referral.qualified", map[string]any{"minutes": r.RewardMinutes, "flagged": r.Flagged})
	if r.Flagged {
		return
	}
	s.signal()
}

// sweepBatch bounds one reconciliation pass.
const sweepBatch = 200

// RunSweep evaluates friends who are verified and have a qualifying reading
// but are still "joined": a hook that did not run (a restart between the
// commit and the hook, rewards paused while the program was disabled). Job
// referral-sweep, every few minutes, safe on several instances (Evaluate
// locks each row).
func (s *ReferralService) RunSweep(ctx context.Context) error {
	if !s.d.Program(ctx).Enabled {
		return nil
	}
	ids, err := s.d.Repo.ReadyToQualify(ctx, sweepBatch)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		s.Evaluate(ctx, id)
	}
	return nil
}

// ---------------------------------------------------------------------------
// The inviter's own view
// ---------------------------------------------------------------------------

// ReferralProgramView is the program as the inviter sees it.
type ReferralProgramView struct {
	Enabled       bool `json:"enabled"`
	RewardMinutes int  `json:"reward_minutes"`
	MaxRewards    int  `json:"max_rewards"`
}

// ReferralStatsView are the inviter's counters: counts and minutes only.
type ReferralStatsView struct {
	// Joined are the friends who signed up with the link, Qualified those who
	// completed a reading (their reward is earned).
	Joined           int64 `json:"joined"`
	Qualified        int64 `json:"qualified"`
	PendingMinutes   int64 `json:"pending_minutes"`
	GrantedMinutes   int64 `json:"granted_minutes"`
	RemainingRewards int64 `json:"remaining_rewards"`
}

// MyReferrals is GET /me/referrals.
type MyReferrals struct {
	Code    string              `json:"code"`
	Link    string              `json:"link"`
	Program ReferralProgramView `json:"program"`
	Stats   ReferralStatsView   `json:"stats"`
}

// Mine returns the caller's code (created on first call), invite link,
// program terms and counters. Only clients invite friends.
func (s *ReferralService) Mine(ctx context.Context, userID uuid.UUID) (*MyReferrals, error) {
	if p, ok := reqctx.PrincipalFrom(ctx); ok && p.Role != domain.RoleClient {
		return nil, apperror.Forbidden("Only clients can invite friends.")
	}
	code, err := s.EnsureCode(ctx, userID)
	if err != nil {
		return nil, err
	}
	stats, err := s.d.Repo.Stats(ctx, userID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	prog := s.d.Program(ctx)
	locale := reqctx.MetaFrom(ctx).Locale
	out := &MyReferrals{Code: code, Link: mail.AppURL(s.d.Cfg.FrontendURL, locale, "") + "?ref=" + code,
		Program: ReferralProgramView{Enabled: prog.Enabled, RewardMinutes: prog.RewardMinutes, MaxRewards: prog.MaxRewardsPerReferrer},
		Stats: ReferralStatsView{Joined: stats.Joined, Qualified: stats.Qualified, PendingMinutes: stats.PendingMinutes,
			GrantedMinutes: stats.GrantedMinutes, RemainingRewards: max(int64(prog.MaxRewardsPerReferrer)-stats.Qualified, 0)}}
	return out, nil
}

// Enabled reports whether the program is on (the lifecycle invite journey
// only runs while it is).
func (s *ReferralService) Enabled(ctx context.Context) bool { return s.d.Program(ctx).Enabled }

// InvitedFriends returns 1 when at least one friend joined with the user's
// code, else 0 (the onboarding checklist only asks whether the client
// invited someone, so the count stops at the first row).
func (s *ReferralService) InvitedFriends(ctx context.Context, userID uuid.UUID) (int, error) {
	found, err := s.d.Repo.HasInvited(ctx, userID)
	if err != nil || !found {
		return 0, err
	}
	return 1, nil
}

// InvitersAmong counts the clients who signed up in [from, to) and invited
// at least one friend (the lifecycle funnel's invite step).
func (s *ReferralService) InvitersAmong(ctx context.Context, from, to time.Time) (int64, error) {
	return s.d.Repo.InvitersAmong(ctx, from, to)
}

// PurgeForUser implements UserDataPurger (account erasure).
func (s *ReferralService) PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.d.Repo.PurgeForUser(ctx, userID)
}

var _ UserDataPurger = (*ReferralService)(nil)
