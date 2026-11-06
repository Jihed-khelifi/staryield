package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

type fakeWaitlistRepo struct {
	mu      sync.Mutex
	entries []domain.WaitlistEntry
}

func (f *fakeWaitlistRepo) isOpen(e domain.WaitlistEntry) bool {
	return !e.DeletedAt.Valid && e.NotifiedAt == nil
}

func (f *fakeWaitlistRepo) Open(_ context.Context, userID, psychicID uuid.UUID) (*domain.WaitlistEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.entries {
		if f.isOpen(e) && e.UserID == userID && e.PsychicID == psychicID {
			return &e, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (f *fakeWaitlistRepo) CountOpen(_ context.Context, userID uuid.UUID) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for _, e := range f.entries {
		if f.isOpen(e) && e.UserID == userID {
			n++
		}
	}
	return n, nil
}

func (f *fakeWaitlistRepo) Create(_ context.Context, e *domain.WaitlistEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = append(f.entries, *e)
	return nil
}

func (f *fakeWaitlistRepo) Leave(_ context.Context, userID, psychicID uuid.UUID, at time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, e := range f.entries {
		if f.isOpen(e) && e.UserID == userID && e.PsychicID == psychicID {
			f.entries[i].DeletedAt.Time, f.entries[i].DeletedAt.Valid = at, true
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeWaitlistRepo) ListOpen(context.Context, uuid.UUID, *repository.Cursor, int) ([]repository.WaitlistRow, error) {
	return nil, nil
}

func (f *fakeWaitlistRepo) MarkAvailable(_ context.Context, psychicID uuid.UUID, at time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for i, e := range f.entries {
		if f.isOpen(e) && e.PsychicID == psychicID && e.AvailableAt == nil {
			t := at
			f.entries[i].AvailableAt = &t
			n++
		}
	}
	return n, nil
}

func (f *fakeWaitlistRepo) ClaimDue(context.Context, int) ([]domain.WaitlistEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.WaitlistEntry
	for _, e := range f.entries {
		if f.isOpen(e) && e.AvailableAt != nil {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeWaitlistRepo) set(ids []uuid.UUID, apply func(*domain.WaitlistEntry)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range ids {
		for i := range f.entries {
			if f.entries[i].ID == id {
				apply(&f.entries[i])
			}
		}
	}
}

func (f *fakeWaitlistRepo) MarkNotified(_ context.Context, ids []uuid.UUID, at time.Time) error {
	f.set(ids, func(e *domain.WaitlistEntry) { t := at; e.NotifiedAt = &t })
	return nil
}

func (f *fakeWaitlistRepo) ResetAvailable(_ context.Context, ids []uuid.UUID, _ time.Time) error {
	f.set(ids, func(e *domain.WaitlistEntry) { e.AvailableAt = nil })
	return nil
}

func (f *fakeWaitlistRepo) ExpireOpen(context.Context, time.Time, time.Time, int) (int64, error) {
	return 0, nil
}
func (f *fakeWaitlistRepo) PurgeFinished(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}
func (f *fakeWaitlistRepo) PurgeForUser(context.Context, uuid.UUID) (int64, error) { return 0, nil }

// fakeAdvisors serves the few PsychicRepository reads the waitlist uses.
type fakeAdvisors struct {
	repository.PsychicRepository
	bySlug map[string]*domain.Psychic
	hidden map[uuid.UUID]bool
}

func (f *fakeAdvisors) GetBySlug(_ context.Context, slug string) (*domain.Psychic, error) {
	if p, ok := f.bySlug[slug]; ok {
		return p, nil
	}
	return nil, repository.ErrNotFound
}

func (f *fakeAdvisors) IsPublic(_ context.Context, id uuid.UUID) (bool, error) {
	return !f.hidden[id], nil
}

func (f *fakeAdvisors) GetByUserID(_ context.Context, id uuid.UUID) (*domain.Psychic, error) {
	for _, p := range f.bySlug {
		if p.UserID == id {
			return p, nil
		}
	}
	return nil, repository.ErrNotFound
}

type recordingNotifier struct {
	mu   sync.Mutex
	sent map[uuid.UUID]int
	spec []NotificationSpec
}

func (r *recordingNotifier) Notify(_ context.Context, spec NotificationSpec, ids ...uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sent == nil {
		r.sent = map[uuid.UUID]int{}
	}
	for _, id := range ids {
		r.sent[id]++
	}
	r.spec = append(r.spec, spec)
}

type waitlistFixture struct {
	svc      *WaitlistService
	repo     *fakeWaitlistRepo
	notes    *recordingNotifier
	clk      *clock.Fake
	advisor  *domain.Psychic
	status   map[uuid.UUID]domain.PresenceStatus
	advisors *fakeAdvisors
}

func newWaitlistFixture(members ...uuid.UUID) *waitlistFixture {
	f := &waitlistFixture{repo: &fakeWaitlistRepo{}, notes: &recordingNotifier{}, clk: clock.NewFake(time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)),
		advisor: &domain.Psychic{UserID: uuid.New(), Slug: "ramone", Name: "Ramone"}, status: map[uuid.UUID]domain.PresenceStatus{}}
	f.advisors = &fakeAdvisors{bySlug: map[string]*domain.Psychic{"ramone": f.advisor}, hidden: map[uuid.UUID]bool{}}
	status := func(_ context.Context, id uuid.UUID) (domain.PresenceStatus, error) {
		if st, ok := f.status[id]; ok {
			return st, nil
		}
		return domain.PresenceOffline, nil
	}
	f.svc = NewWaitlistService(inlineTx{}, f.repo, f.advisors, statusFromAvailability, status, f.notes, analytics.Noop{}, f.clk)
	isMember := map[uuid.UUID]bool{}
	for _, m := range members {
		isMember[m] = true
	}
	f.svc.SetPriority(func(_ context.Context, id uuid.UUID) bool { return isMember[id] })
	return f
}

func TestWaitlistJoinRules(t *testing.T) {
	f := newWaitlistFixture()
	ctx := context.Background()
	client := uuid.New()
	if _, err := f.svc.Join(ctx, client, domain.RolePsychic, "ramone"); !isCode(err, apperror.CodeForbidden) {
		t.Fatalf("advisor joined: %v", err)
	}
	if _, err := f.svc.Join(ctx, client, domain.RoleClient, "nobody"); !isCode(err, apperror.CodeNotFound) {
		t.Fatalf("unknown advisor: %v", err)
	}
	f.status[f.advisor.UserID] = domain.PresenceOnline
	if _, err := f.svc.Join(ctx, client, domain.RoleClient, "ramone"); !isCode(err, apperror.CodeAdvisorAvailable) {
		t.Fatalf("online advisor: %v", err)
	}
	f.status[f.advisor.UserID] = domain.PresenceBusy
	first, err := f.svc.Join(ctx, client, domain.RoleClient, " ramone ")
	if err != nil || first.Status != "busy" {
		t.Fatalf("join busy advisor: %+v %v", first, err)
	}
	f.clk.Advance(time.Minute)
	again, err := f.svc.Join(ctx, client, domain.RoleClient, "ramone")
	if err != nil || !again.JoinedAt.Equal(first.JoinedAt) || len(f.repo.entries) != 1 {
		t.Fatalf("second join must be idempotent: %+v %v (%d rows)", again, err, len(f.repo.entries))
	}
	f.advisors.hidden[f.advisor.UserID] = true
	if _, err := f.svc.Join(ctx, uuid.New(), domain.RoleClient, "ramone"); !isCode(err, apperror.CodeNotFound) {
		t.Fatalf("hidden advisor: %v", err)
	}
	f.advisors.hidden[f.advisor.UserID] = false

	// The cap counts open entries only.
	capped := uuid.New()
	for i := 0; i < domain.WaitlistMaxOpen; i++ {
		f.repo.entries = append(f.repo.entries, domain.WaitlistEntry{ID: uuid.New(), UserID: capped, PsychicID: uuid.New()})
	}
	if _, err := f.svc.Join(ctx, capped, domain.RoleClient, "ramone"); !isCode(err, apperror.CodeLimitReached) {
		t.Fatalf("cap: %v", err)
	}
	f.repo.entries[len(f.repo.entries)-1].NotifiedAt = ptrTo(f.clk.Now())
	if _, err := f.svc.Join(ctx, capped, domain.RoleClient, "ramone"); err != nil {
		t.Fatalf("a finished entry must free its slot: %v", err)
	}
}

func ptrTo[T any](v T) *T { return &v }

func isCode(err error, code apperror.Code) bool {
	e, ok := apperror.As(err)
	return ok && e.Code == code
}

func TestWaitlistRunDuePriorityLeadAndReset(t *testing.T) {
	member, regular := uuid.New(), uuid.New()
	f := newWaitlistFixture(member)
	ctx := context.Background()
	f.status[f.advisor.UserID] = domain.PresenceAway
	for _, u := range []uuid.UUID{member, regular} {
		if _, err := f.svc.Join(ctx, u, domain.RoleClient, "ramone"); err != nil {
			t.Fatal(err)
		}
	}
	mustRun := func() {
		t.Helper()
		if err := f.svc.RunDue(ctx); err != nil {
			t.Fatal(err)
		}
	}
	mustRun() // nothing stamped yet
	if len(f.notes.sent) != 0 {
		t.Fatal("notified before the advisor came online")
	}

	f.status[f.advisor.UserID] = domain.PresenceOnline
	f.svc.AdvisorOnline(ctx, f.advisor.UserID)
	select {
	case <-f.svc.Wake():
	default:
		t.Fatal("stamping due entries must wake the job")
	}
	mustRun()
	if f.notes.sent[member] != 1 || f.notes.sent[regular] != 0 {
		t.Fatalf("priority: %+v", f.notes.sent)
	}
	if s := f.notes.spec[0]; s.Kind != NotifyWaitlistAvailable || s.Link != "/chatroom?reader=ramone" || s.Data["psychic_name"] != "Ramone" {
		t.Fatalf("spec: %+v", s)
	}

	// The advisor leaves before the lead passes: the entry waits for next time.
	f.clk.Advance(waitlistPriorityLead + time.Second)
	f.status[f.advisor.UserID] = domain.PresenceBusy
	mustRun()
	if f.notes.sent[regular] != 0 {
		t.Fatal("told about an advisor who is no longer available")
	}
	if e, _ := f.repo.Open(ctx, regular, f.advisor.UserID); e == nil || e.AvailableAt != nil {
		t.Fatalf("entry should be open and unstamped: %+v", e)
	}

	// Next time online, the lead starts again.
	f.status[f.advisor.UserID] = domain.PresenceOnline
	f.svc.AdvisorOnline(ctx, f.advisor.UserID)
	<-f.svc.Wake() // the scheduler consumes the wake-up
	mustRun()
	if f.notes.sent[regular] != 0 {
		t.Fatal("told before the new lead passed")
	}
	f.clk.Advance(waitlistPriorityLead)
	mustRun()
	mustRun()
	if f.notes.sent[regular] != 1 || f.notes.sent[member] != 1 {
		t.Fatalf("each entry is told exactly once: %+v", f.notes.sent)
	}
	f.svc.AdvisorOnline(ctx, f.advisor.UserID) // nothing open: no wake-up
	select {
	case <-f.svc.Wake():
		t.Fatal("woke with nothing due")
	default:
	}
}
