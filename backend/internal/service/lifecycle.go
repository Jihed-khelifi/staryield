package service

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// LifecycleJob is the worker job that sends the lifecycle messages.
const LifecycleJob = "lifecycle"

// Notification kinds of the lifecycle journeys (rendered by the web app from
// Kind + Data; e-mail and push text in internal/mail/lifecycle.go).
const (
	NotifyLifecycleVerifyReminder = "lifecycle.verify_reminder"
	NotifyLifecycleFirstReading   = "lifecycle.first_reading"
	NotifyLifecycleInviteFriends  = "lifecycle.invite_friends"
	NotifyLifecycleWinBack        = "lifecycle.win_back"
)

// The nudges are promotional: their e-mail and push go to the people who opted
// in to marketing (and the outbox cancels the e-mail while the provider only
// sends transactional mail). The verification reminder is transactional: its
// e-mail is the fresh verification link, sent by the e-mail verification
// feature through SetVerificationReminder, so the kind itself is in-app only.
func init() {
	marketing := ChannelPolicy{Email: true, Category: domain.EmailMarketing, Push: true}
	channelPolicies[NotifyLifecycleFirstReading] = marketing
	channelPolicies[NotifyLifecycleInviteFriends] = marketing
	channelPolicies[NotifyLifecycleWinBack] = marketing
}

// Lifecycle tuning. Delays and the attribution window are admin settings; the
// rest are product rules.
const (
	// lifecycleCap: at most one lifecycle message per user per this long.
	lifecycleCap = 48 * time.Hour
	// lifecycleCatchUp: a sign-up or reading older than the delay by this much
	// is no longer nudged (turning a journey on, or a long outage, never
	// messages the whole history).
	lifecycleCatchUp = 72 * time.Hour
	// Win-back: at most one message per 30-day cycle and three per lapse (the
	// last reading). Cycle k is due when the last reading is k-1 cycles older
	// than the delay; each cycle is its own catch-up band, so a reader lapsed
	// for longer than the three cycles is left alone.
	lifecycleWinBackCycle  = 30 * 24 * time.Hour
	lifecycleWinBackCycles = 3
	// lifecycleReconcile: readings settled in this long are re-read each run, so
	// a failed settlement hook never leaves a reader missing.
	lifecycleReconcile = 3 * time.Hour
	lifecycleBatch     = 500
	// lifecycleMaxBatches bounds one journey's work per run; lifecycleBudget is
	// the wall-clock budget of a whole run (the next run continues).
	lifecycleMaxBatches = 40
	lifecycleBudget     = 3 * time.Minute
	lifecycleHookTime   = 10 * time.Second
)

// LifecycleDeps wire the lifecycle service.
type LifecycleDeps struct {
	Tx       database.TxManager
	Repo     repository.LifecycleRepository
	Site     repository.SiteRepository
	Contacts repository.ContactRepository
	Audit    repository.AuditRepository
	Notifier Notifier
	// Events publishes onboarding.updated to the client's tabs and devices.
	Events EventPublisher
	// Cache (optional) keeps the activation funnel for a minute.
	Cache cache.Store
	Clock clock.Clock
	Log   *zap.Logger
	// IPHashSecret keys the IP hash of audit entries.
	IPHashSecret string
}

// LifecycleService sends the lifecycle messages (verification reminder, first
// reading, after the first reading, win-back), measures their conversion and
// serves the onboarding checklist and the admin activation reports.
type LifecycleService struct {
	d LifecycleDeps

	mu               sync.RWMutex
	sendVerification func(ctx context.Context, userID uuid.UUID) error
	referralsEnabled func(ctx context.Context) bool
	invitedFriends   func(ctx context.Context, userID uuid.UUID) (int, error)
	invitedInCohort  func(ctx context.Context, from, to time.Time) (int64, error)
}

// NewLifecycleService wires the service.
func NewLifecycleService(d LifecycleDeps) *LifecycleService {
	if d.Notifier == nil {
		d.Notifier = NoopNotifier{}
	}
	if d.Events == nil {
		d.Events = NoopPublisher{}
	}
	if d.Log == nil {
		d.Log = zap.NewNop()
	}
	return &LifecycleService{d: d}
}

// SetVerificationReminder installs the function that sends a user a fresh
// e-mail verification link (transactional). Without it the verification
// reminder journey does not run.
func (s *LifecycleService) SetVerificationReminder(fn func(ctx context.Context, userID uuid.UUID) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sendVerification = fn
}

// SetReferralsEnabled installs the referral program's switch. Without it the
// program counts as off: the invite journey records its messages as skipped.
func (s *LifecycleService) SetReferralsEnabled(fn func(ctx context.Context) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.referralsEnabled = fn
}

// SetInvitedFriends installs the number of friends a user invited (the
// onboarding checklist's last step). Without it the count is 0.
func (s *LifecycleService) SetInvitedFriends(fn func(ctx context.Context, userID uuid.UUID) (int, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invitedFriends = fn
}

// SetInvitedCohort installs the number of clients, among those who signed up in
// [from, to), who invited at least one friend (the funnel's invite step).
// Without it the step is reported as unavailable.
func (s *LifecycleService) SetInvitedCohort(fn func(ctx context.Context, from, to time.Time) (int64, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invitedInCohort = fn
}

func (s *LifecycleService) verificationSender() func(context.Context, uuid.UUID) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sendVerification
}

func (s *LifecycleService) referralsOn(ctx context.Context) bool {
	s.mu.RLock()
	fn := s.referralsEnabled
	s.mu.RUnlock()
	return fn != nil && fn(ctx)
}

// journeyAvailable says whether a journey can run in this deployment.
func (s *LifecycleService) journeyAvailable(key string) bool {
	if key == domain.JourneyVerifyReminder {
		return s.verificationSender() != nil
	}
	return true
}

// ---------------------------------------------------------------------------
// Conversion
// ---------------------------------------------------------------------------

// Attach registers the settlement observer on the chat service.
func (s *LifecycleService) Attach(chat *ChatService) { chat.OnSessionSettled(s.OnSessionSettled) }

// OnSessionSettled records a completed reading: the client's reader row is
// refreshed and the lifecycle messages that preceded the reading within the
// attribution window are marked as converted (one UPDATE). It runs after the
// settlement committed and logs its own errors; the job's reconciliation
// repairs a reader row this missed.
func (s *LifecycleService) OnSessionSettled(ctx context.Context, sess domain.Session) {
	if sess.BilledMinutes <= 0 || sess.SettledAt == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), lifecycleHookTime)
	defer cancel()
	log := s.d.Log.With(zap.String("component", "lifecycle"))
	if err := s.d.Repo.RecordReaders(ctx, []uuid.UUID{sess.ClientID}); err != nil {
		log.Warn("lifecycle: record reader failed", zap.Error(err))
	}
	set := s.settings(ctx)
	from := sess.SettledAt.AddDate(0, 0, -set.AttributionDays)
	if _, err := s.d.Repo.Convert(ctx, sess.ClientID, from, *sess.SettledAt); err != nil {
		log.Warn("lifecycle: conversion update failed", zap.Error(err))
	}
}

// ---------------------------------------------------------------------------
// The job
// ---------------------------------------------------------------------------

// lifecycleCounts are one run's results, logged when anything happened.
type lifecycleCounts struct {
	Sent, Skipped, Failed, Conflicts int
}

// RunDue sends what is due. Only one instance runs it at a time (advisory
// lock); the claim insert makes each message exactly-once even so. It is cheap
// when nothing is due: every journey is one indexed query returning no rows.
func (s *LifecycleService) RunDue(ctx context.Context) error {
	_, err := s.d.Repo.WithRunLock(ctx, s.run)
	return err
}

func (s *LifecycleService) run(ctx context.Context) error {
	started := time.Now()
	log := s.d.Log.With(zap.String("component", "lifecycle"))
	if n, err := s.d.Repo.ReconcileReaders(ctx, s.d.Clock.Now().Add(-lifecycleReconcile)); err != nil {
		log.Warn("lifecycle: reader reconciliation failed", zap.Error(err))
	} else if n > 0 {
		log.Debug("lifecycle: readers reconciled", zap.Int("clients", n))
	}
	set := s.settings(ctx)
	var online *int // counted once per run, when a journey needs it
	var firstErr error
	for _, key := range domain.LifecycleJourneys {
		cfg := set.Journeys[key]
		if !cfg.Enabled || !s.journeyAvailable(key) {
			continue
		}
		if time.Since(started) > lifecycleBudget {
			log.Info("lifecycle: time budget used, continuing next run")
			break
		}
		counts, err := s.runJourney(ctx, key, cfg, started, &online)
		if counts != (lifecycleCounts{}) {
			log.Info("lifecycle: journey run", zap.String("journey", key), zap.Int("sent", counts.Sent), zap.Int("skipped", counts.Skipped),
				zap.Int("failed", counts.Failed), zap.Int("conflicts", counts.Conflicts))
		}
		if err != nil {
			log.Error("lifecycle: journey failed", zap.String("journey", key), zap.Error(err))
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// dueWindows are the event-time bands a journey looks at in one run: the
// users whose event (sign-up, first or last reading) passed the delay within
// the catch-up window; for win-back one band per cycle.
func dueWindows(key string, cfg LifecycleJourney, now time.Time) []repository.LifecycleDueQuery {
	delay := time.Duration(cfg.DelayHours) * time.Hour
	base := repository.LifecycleDueQuery{Journey: key, CapSince: now.Add(-lifecycleCap), Limit: lifecycleBatch}
	bands := 1
	if key == domain.JourneyWinBack {
		bands = lifecycleWinBackCycles
		base.InactiveSince = now.Add(-delay)
		base.CycleSince = now.Add(-lifecycleWinBackCycle)
		base.MaxCycles = lifecycleWinBackCycles
	}
	out := make([]repository.LifecycleDueQuery, 0, bands)
	for k := 0; k < bands; k++ {
		q := base
		q.Cutoff = now.Add(-delay - time.Duration(k)*lifecycleWinBackCycle)
		q.Floor = q.Cutoff.Add(-lifecycleCatchUp)
		out = append(out, q)
	}
	return out
}

func (s *LifecycleService) runJourney(ctx context.Context, key string, cfg LifecycleJourney, started time.Time, online **int) (lifecycleCounts, error) {
	var total lifecycleCounts
	// Each band is fixed for the whole run and walked in keyset order, so every
	// due user is read once per run whatever happens to its claim.
	batches := 0
	for _, q := range dueWindows(key, cfg, s.d.Clock.Now()) {
		for ; batches < lifecycleMaxBatches && time.Since(started) <= lifecycleBudget; batches++ {
			if ctx.Err() != nil {
				return total, ctx.Err()
			}
			due, err := s.d.Repo.Due(ctx, q)
			if err != nil {
				return total, err
			}
			if len(due) == 0 {
				break
			}
			counts, err := s.process(ctx, key, due, s.d.Clock.Now(), online)
			total.Sent += counts.Sent
			total.Skipped += counts.Skipped
			total.Failed += counts.Failed
			total.Conflicts += counts.Conflicts
			if err != nil {
				return total, err
			}
			if len(due) < lifecycleBatch {
				break
			}
			last := due[len(due)-1]
			q.After = &repository.LifecycleCursor{At: last.EventAt, UserID: last.UserID}
		}
	}
	return total, nil
}

// process claims one batch and delivers the claims this instance won.
func (s *LifecycleService) process(ctx context.Context, key string, due []repository.LifecycleDue, now time.Time, online **int) (lifecycleCounts, error) {
	var counts lifecycleCounts
	ids := make([]uuid.UUID, len(due))
	for i := range due {
		ids[i] = due[i].UserID
	}
	contacts, err := s.d.Contacts.Contacts(ctx, ids)
	if err != nil {
		return counts, err
	}
	byID := make(map[uuid.UUID]repository.Contact, len(contacts))
	for _, c := range contacts {
		byID[c.UserID] = c
	}
	referralsOn := key != domain.JourneyAfterFirstReading || s.referralsOn(ctx)
	policy := PolicyFor(journeyKind(key))
	rows := make([]domain.LifecycleMessage, 0, len(due))
	dueOf := make(map[uuid.UUID]repository.LifecycleDue, len(due))
	for _, d := range due {
		c, found := byID[d.UserID]
		if !found || c.Status != domain.UserActive {
			continue
		}
		row := domain.LifecycleMessage{ID: domain.NewID(), UserID: d.UserID, Journey: key, Cycle: max(d.Cycle, 1), Status: domain.LifecycleSent,
			CreatedAt: now}
		if !referralsOn {
			row.Status, row.SkipReason = domain.LifecycleSkipped, domain.LifecycleSkipReferralsOff
			row.Channels = domain.JSONMap{}
		} else {
			// What is attempted: the in-app notification always; e-mail and push
			// only for people whose settings allow this category (the outbox may
			// still cancel marketing e-mail while no marketing provider exists).
			row.Channels = domain.JSONMap{"in_app": true, "email": policy.wantsEmail(c), "push": policy.wantsPush(c)}
			if key == domain.JourneyVerifyReminder {
				row.Channels["email"] = true // the verification link, a transactional message
			}
		}
		rows = append(rows, row)
		dueOf[d.UserID] = d
	}
	inserted, err := s.d.Repo.Claim(ctx, rows)
	if err != nil {
		return counts, err
	}
	var release []uuid.UUID
	for _, row := range rows {
		if !inserted[row.ID] {
			counts.Conflicts++ // another instance (or run) claimed it first
			continue
		}
		if row.Status == domain.LifecycleSkipped {
			counts.Skipped++
			continue
		}
		spec, ok := s.spec(ctx, key, dueOf[row.UserID], online)
		if key == domain.JourneyVerifyReminder {
			send := s.verificationSender()
			if send == nil {
				release = append(release, row.ID)
				continue
			}
			if err := send(ctx, row.UserID); err != nil {
				// Nothing went out: give the claim back so the next run retries
				// (the catch-up window bounds the retries).
				s.d.Log.Warn("lifecycle: verification reminder failed", zap.Error(err))
				release = append(release, row.ID)
				counts.Failed++
				continue
			}
		}
		if ok {
			s.d.Notifier.Notify(ctx, spec, row.UserID)
		}
		counts.Sent++
	}
	if len(release) > 0 {
		if err := s.d.Repo.Release(ctx, release); err != nil {
			return counts, err
		}
	}
	return counts, nil
}

func journeyKind(key string) string {
	switch key {
	case domain.JourneyVerifyReminder:
		return NotifyLifecycleVerifyReminder
	case domain.JourneyFirstReading:
		return NotifyLifecycleFirstReading
	case domain.JourneyAfterFirstReading:
		return NotifyLifecycleInviteFriends
	case domain.JourneyWinBack:
		return NotifyLifecycleWinBack
	}
	return ""
}

// spec builds the notification of a journey. Its data only states facts true
// now: the wallet's free minutes, the advisors online at this moment.
func (s *LifecycleService) spec(ctx context.Context, key string, d repository.LifecycleDue, online **int) (NotificationSpec, bool) {
	switch key {
	case domain.JourneyVerifyReminder:
		return NotificationSpec{Kind: NotifyLifecycleVerifyReminder, Link: "/profile/settings#account"}, true
	case domain.JourneyFirstReading:
		return NotificationSpec{Kind: NotifyLifecycleFirstReading, Link: "/psychics",
			Data: map[string]any{"free_minutes": d.FreeMinutes, "online_advisors": s.onlineAdvisors(ctx, online)}}, true
	case domain.JourneyAfterFirstReading:
		return NotificationSpec{Kind: NotifyLifecycleInviteFriends, Link: "/profile/invite"}, true
	case domain.JourneyWinBack:
		n := s.onlineAdvisors(ctx, online)
		link := "/psychics"
		if n == 0 {
			link = "/profile" // the daily horoscope is on the overview
		}
		return NotificationSpec{Kind: NotifyLifecycleWinBack, Link: link, Data: map[string]any{"online_advisors": n}}, true
	}
	return NotificationSpec{}, false
}

// onlineAdvisors counts the advisors online, once per run.
func (s *LifecycleService) onlineAdvisors(ctx context.Context, cache **int) int {
	if *cache != nil {
		return **cache
	}
	n, err := s.d.Repo.OnlineAdvisors(ctx)
	if err != nil {
		warnErr(ctx, "lifecycle: count online advisors", err)
		n = 0 // claim nothing we cannot show
	}
	*cache = &n
	return n
}
