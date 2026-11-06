package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// WaitlistJob notifies waiting clients when their advisor is online.
const WaitlistJob = "waitlist"

// Waitlist timing and sizes.
const (
	// waitlistPriorityLead is how long members with the waitlist-priority
	// benefit have an advisor to themselves before everyone else waiting is
	// told the advisor is online.
	waitlistPriorityLead = 2 * time.Minute
	waitlistClaimBatch   = 200
	waitlistPageDefault  = 20
	waitlistPageMax      = 50
)

// Waitlist messages (translated at the API edge).
const (
	msgWaitlistLimit = "You can wait for up to 20 advisors at a time. Leave a waiting list to join another."
	msgWaitlistOwn   = "Only clients can join a waiting list."
	msgWaitlistNow   = "This advisor is online now. Start a chat instead."
)

// WaitlistView is one advisor the signed-in client is waiting for.
type WaitlistView struct {
	Slug     string    `json:"slug"`
	Name     string    `json:"name"`
	ImageURL string    `json:"image_url"`
	Status   string    `json:"status"`
	JoinedAt time.Time `json:"joined_at"`
}

// WaitlistPriority reports whether a client is told first (membership
// benefit). A nil hook gives nobody priority.
type WaitlistPriority func(ctx context.Context, userID uuid.UUID) bool

// AdvisorStatus returns one advisor's effective status.
type AdvisorStatus func(ctx context.Context, psychicID uuid.UUID) (domain.PresenceStatus, error)

// WaitlistService lets clients wait for an advisor to come online and tells
// them, once, when the advisor is available.
type WaitlistService struct {
	tx       database.TxManager
	repo     repository.WaitlistRepository
	psychics repository.PsychicRepository
	statuses StatusLookup
	status   AdvisorStatus
	priority WaitlistPriority
	notify   Notifier
	tracker  analytics.Tracker
	clock    clock.Clock
	lead     time.Duration
	wake     chan struct{}
}

// NewWaitlistService wires the waitlist.
func NewWaitlistService(tx database.TxManager, repo repository.WaitlistRepository, psychics repository.PsychicRepository,
	statuses StatusLookup, status AdvisorStatus, notify Notifier, tracker analytics.Tracker, clk clock.Clock) *WaitlistService {
	if notify == nil {
		notify = NoopNotifier{}
	}
	return &WaitlistService{tx: tx, repo: repo, psychics: psychics, statuses: statuses, status: status, notify: notify,
		tracker: tracker, clock: clk, lead: waitlistPriorityLead, wake: make(chan struct{}, 1)}
}

// SetPriority installs the membership priority hook.
func (s *WaitlistService) SetPriority(p WaitlistPriority) { s.priority = p }

// Wake is signalled when notifications may be due (the job runs early).
func (s *WaitlistService) Wake() <-chan struct{} { return s.wake }

func (s *WaitlistService) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// publicAdvisor resolves a slug to an advisor who may appear publicly.
func (s *WaitlistService) publicAdvisor(ctx context.Context, slug string) (*domain.Psychic, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" || len(slug) > 80 {
		return nil, apperror.NotFound("Advisor")
	}
	p, err := s.psychics.GetBySlug(ctx, slug)
	if err != nil {
		return nil, mapRepoErr(err, "Advisor")
	}
	public, err := s.psychics.IsPublic(ctx, p.UserID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if !public {
		return nil, apperror.NotFound("Advisor")
	}
	return p, nil
}

func waitlistView(p *domain.Psychic, st domain.PresenceStatus, joined time.Time) WaitlistView {
	return WaitlistView{Slug: p.Slug, Name: p.Name, ImageURL: p.ImageURL, Status: string(st), JoinedAt: joined}
}

// Join puts a client on an advisor's waiting list. Joining a list the client
// is already on returns the existing entry. An advisor who is online now
// cannot be waited for (the client should start a chat instead).
func (s *WaitlistService) Join(ctx context.Context, userID uuid.UUID, role domain.Role, slug string) (*WaitlistView, error) {
	if role != domain.RoleClient {
		return nil, apperror.Forbidden(msgWaitlistOwn)
	}
	p, err := s.publicAdvisor(ctx, slug)
	if err != nil {
		return nil, err
	}
	st, err := s.status(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	if st == domain.PresenceOnline {
		return nil, apperror.Conflict(apperror.CodeAdvisorAvailable, msgWaitlistNow)
	}
	if e, err := s.repo.Open(ctx, userID, p.UserID); err == nil {
		v := waitlistView(p, st, e.CreatedAt)
		return &v, nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, apperror.Internal(err)
	}
	n, err := s.repo.CountOpen(ctx, userID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if n >= domain.WaitlistMaxOpen {
		return nil, apperror.Conflict(apperror.CodeLimitReached, msgWaitlistLimit)
	}
	now := s.clock.Now()
	e := &domain.WaitlistEntry{ID: domain.NewID(), UserID: userID, PsychicID: p.UserID, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.Create(ctx, e); err != nil {
		if !errors.Is(err, repository.ErrDuplicate) {
			return nil, apperror.Internal(err)
		}
		// A concurrent join won: return that entry.
		if e, err = s.repo.Open(ctx, userID, p.UserID); err != nil {
			return nil, mapRepoErr(err, "Waiting list entry")
		}
	}
	s.tracker.Track(ctx, "waitlist_joined", map[string]any{"psychic_slug": p.Slug})
	v := waitlistView(p, st, e.CreatedAt)
	return &v, nil
}

// Leave takes the client off an advisor's waiting list. It succeeds whether
// or not the client was on it.
func (s *WaitlistService) Leave(ctx context.Context, userID uuid.UUID, slug string) error {
	slug = strings.TrimSpace(slug)
	if slug == "" || len(slug) > 80 {
		return apperror.NotFound("Advisor")
	}
	p, err := s.psychics.GetBySlug(ctx, slug)
	if err != nil {
		return mapRepoErr(err, "Advisor")
	}
	left, err := s.repo.Leave(ctx, userID, p.UserID, s.clock.Now())
	if err != nil {
		return apperror.Internal(err)
	}
	if left {
		s.tracker.Track(ctx, "waitlist_left", map[string]any{"psychic_slug": p.Slug})
	}
	return nil
}

// List pages the advisors the client is waiting for, newest first, with
// their live status.
func (s *WaitlistService) List(ctx context.Context, userID uuid.UUID, cursor string, limit int) (Page[WaitlistView], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[WaitlistView]{}, err
	}
	size := limit
	if size <= 0 {
		size = waitlistPageDefault
	}
	size = min(size, waitlistPageMax)
	rows, err := s.repo.ListOpen(ctx, userID, c, size+1)
	if err != nil {
		return Page[WaitlistView]{}, apperror.Internal(err)
	}
	page := Page[WaitlistView]{Items: make([]WaitlistView, 0, min(len(rows), size))}
	if len(rows) > size {
		last := rows[size-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
		rows = rows[:size]
	}
	list := make([]domain.Psychic, len(rows))
	for i, r := range rows {
		list[i] = domain.Psychic{UserID: r.PsychicID, Availability: r.Availability}
	}
	st, err := s.statuses(ctx, list)
	if err != nil {
		return Page[WaitlistView]{}, err
	}
	for _, r := range rows {
		status := st[r.PsychicID]
		if status == "" {
			status = domain.PresenceOffline
		}
		page.Items = append(page.Items, WaitlistView{Slug: r.PsychicSlug, Name: r.PsychicName, ImageURL: r.PsychicImage,
			Status: string(status), JoinedAt: r.CreatedAt})
	}
	return page, nil
}

// AdvisorOnline records that an advisor came online: their waiting clients
// are due a notification. It is called on every published "online" status,
// so it is idempotent (already stamped entries are left alone) and never
// fails the caller.
func (s *WaitlistService) AdvisorOnline(ctx context.Context, psychicID uuid.UUID) {
	n, err := s.repo.MarkAvailable(database.Detach(ctx), psychicID, s.clock.Now())
	if err != nil {
		warnErr(ctx, "waitlist: mark advisor available", err)
		return
	}
	if n > 0 {
		s.signal()
	}
}

type waitlistSend struct {
	psychicID uuid.UUID
	users     []uuid.UUID
}

// RunDue sends the notifications that are due. An
// entry is told once, while its advisor is online: members with priority
// straight away, everyone else once the priority lead has passed. Entries
// whose advisor went away first wait for the next time the advisor is
// online. Safe to run on several instances at once.
func (s *WaitlistService) RunDue(ctx context.Context) error {
	now := s.clock.Now()
	var sends []waitlistSend
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		due, err := s.repo.ClaimDue(ctx, waitlistClaimBatch)
		if err != nil || len(due) == 0 {
			return err
		}
		status := map[uuid.UUID]domain.PresenceStatus{}
		for _, e := range due {
			if _, ok := status[e.PsychicID]; ok {
				continue
			}
			st, err := s.status(ctx, e.PsychicID)
			if err != nil && !errors.Is(err, apperror.ErrNotFound) {
				return err
			}
			status[e.PsychicID] = st
		}
		var notified, reset []uuid.UUID
		byAdvisor := map[uuid.UUID][]uuid.UUID{}
		var order []uuid.UUID
		for _, e := range due {
			if status[e.PsychicID] != domain.PresenceOnline {
				reset = append(reset, e.ID)
				continue
			}
			if now.Sub(*e.AvailableAt) < s.lead && (s.priority == nil || !s.priority(ctx, e.UserID)) {
				continue // not due yet: a later run tells them if the advisor is still online
			}
			notified = append(notified, e.ID)
			if _, ok := byAdvisor[e.PsychicID]; !ok {
				order = append(order, e.PsychicID)
			}
			byAdvisor[e.PsychicID] = append(byAdvisor[e.PsychicID], e.UserID)
		}
		if err := s.repo.MarkNotified(ctx, notified, now); err != nil {
			return err
		}
		if err := s.repo.ResetAvailable(ctx, reset, now); err != nil {
			return err
		}
		for _, id := range order {
			sends = append(sends, waitlistSend{psychicID: id, users: byAdvisor[id]})
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, snd := range sends {
		p, err := s.psychics.GetByUserID(ctx, snd.psychicID)
		if err != nil {
			warnErr(ctx, "waitlist: advisor for notification", err)
			continue
		}
		s.notify.Notify(ctx, NotificationSpec{Kind: NotifyWaitlistAvailable,
			Data: map[string]any{"psychic_name": p.Name, "psychic_slug": p.Slug},
			Link: "/chatroom?reader=" + p.Slug}, snd.users...)
	}
	return nil
}

// ExpireStale closes open entries older than domain.WaitlistOpenTTL
// (maintenance cleanup).
func (s *WaitlistService) ExpireStale(ctx context.Context, now time.Time) (int64, error) {
	return s.repo.ExpireOpen(ctx, now.Add(-domain.WaitlistOpenTTL), now, 1000)
}

// PurgeFinished deletes notified and left entries past retention
// (maintenance cleanup).
func (s *WaitlistService) PurgeFinished(ctx context.Context, now time.Time) (int64, error) {
	return s.repo.PurgeFinished(ctx, now.Add(-domain.WaitlistRetention), 1000)
}
