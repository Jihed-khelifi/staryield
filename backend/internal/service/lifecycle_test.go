package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// --- fakes ----------------------------------------------------------------

type fakeLifecycleRepo struct {
	repository.LifecycleRepository
	mu        sync.Mutex
	due       map[string][]repository.LifecycleDue // journey -> users in keyset order (a claimed user is no longer due)
	queries   []repository.LifecycleDueQuery
	claimed   []domain.LifecycleMessage
	conflicts map[uuid.UUID]bool // users whose claim another instance won
	released  []uuid.UUID
	online    int
	facts     repository.OnboardingFacts
	dismissed *time.Time
	converts  []convertCall
	readers   []uuid.UUID
	locked    bool // another instance holds the run lock
}

type convertCall struct {
	user     uuid.UUID
	from, to time.Time
}

func (f *fakeLifecycleRepo) WithRunLock(ctx context.Context, fn func(context.Context) error) (bool, error) {
	if f.locked {
		return false, nil
	}
	return true, fn(ctx)
}
func (f *fakeLifecycleRepo) ReconcileReaders(context.Context, time.Time) (int, error) { return 0, nil }

// Due returns the journey's users after the cursor, like the keyset query:
// users already claimed for the journey are not due any more.
func (f *fakeLifecycleRepo) Due(_ context.Context, q repository.LifecycleDueQuery) ([]repository.LifecycleDue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, q)
	claimed := map[uuid.UUID]bool{}
	for _, c := range f.claimed {
		if c.Journey == q.Journey {
			claimed[c.UserID] = true
		}
	}
	all := f.due[q.Journey]
	start := 0
	if q.After != nil {
		for i, d := range all {
			if d.UserID == q.After.UserID {
				start = i + 1
			}
		}
	}
	var out []repository.LifecycleDue
	for _, d := range all[start:] {
		if len(out) == q.Limit {
			break
		}
		if !claimed[d.UserID] {
			out = append(out, d)
		}
	}
	return out, nil
}
func (f *fakeLifecycleRepo) Claim(_ context.Context, rows []domain.LifecycleMessage) (map[uuid.UUID]bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	got := map[uuid.UUID]bool{}
	for _, r := range rows {
		if f.conflicts[r.UserID] {
			continue
		}
		got[r.ID] = true
		f.claimed = append(f.claimed, r)
	}
	return got, nil
}
func (f *fakeLifecycleRepo) Release(_ context.Context, ids []uuid.UUID) error {
	f.released = append(f.released, ids...)
	return nil
}
func (f *fakeLifecycleRepo) Stats(context.Context, time.Time, int) ([]repository.LifecycleStatRow, error) {
	return nil, nil
}
func (f *fakeLifecycleRepo) OnlineAdvisors(context.Context) (int, error) { return f.online, nil }
func (f *fakeLifecycleRepo) RecordReaders(_ context.Context, ids []uuid.UUID) error {
	f.readers = append(f.readers, ids...)
	return nil
}
func (f *fakeLifecycleRepo) Convert(_ context.Context, u uuid.UUID, from, to time.Time) (int64, error) {
	f.converts = append(f.converts, convertCall{u, from, to})
	return 1, nil
}
func (f *fakeLifecycleRepo) Onboarding(context.Context, uuid.UUID) (repository.OnboardingFacts, error) {
	facts := f.facts
	facts.Dismissed = f.dismissed != nil
	return facts, nil
}
func (f *fakeLifecycleRepo) SetOnboardingDismissed(_ context.Context, _ uuid.UUID, at *time.Time) error {
	f.dismissed = at
	return nil
}

type fakeLifecycleSite struct {
	repository.SiteRepository
	st      *domain.SiteSetting
	saved   domain.JSONMap
	version int
}

func (f *fakeLifecycleSite) Setting(context.Context, string) (*domain.SiteSetting, error) {
	if f.st == nil {
		return nil, repository.ErrNotFound
	}
	return f.st, nil
}
func (f *fakeLifecycleSite) SettingForUpdate(ctx context.Context, key string) (*domain.SiteSetting, error) {
	return f.Setting(ctx, key)
}
func (f *fakeLifecycleSite) SaveSetting(_ context.Context, _ string, v domain.JSONMap, version int, _ *uuid.UUID, _ time.Time) error {
	b, _ := json.Marshal(v) // as stored: numbers come back as float64
	var stored domain.JSONMap
	_ = json.Unmarshal(b, &stored)
	f.saved, f.version = stored, version
	f.st = &domain.SiteSetting{Key: domain.SiteSettingLifecycle, Value: stored, Version: version}
	return nil
}
func (f *fakeLifecycleSite) UserNames(context.Context, []uuid.UUID) (map[uuid.UUID]string, error) {
	return map[uuid.UUID]string{}, nil
}

type fakeContacts map[uuid.UUID]repository.Contact

func (f fakeContacts) Contacts(_ context.Context, ids []uuid.UUID) ([]repository.Contact, error) {
	var out []repository.Contact
	for _, id := range ids {
		if c, ok := f[id]; ok {
			out = append(out, c)
		}
	}
	return out, nil
}

type lcEvent struct {
	kind  string
	data  any
	users []uuid.UUID
}

type lcEvents struct {
	NoopPublisher
	got []lcEvent
}

func (p *lcEvents) Publish(_ context.Context, kind string, data any, ids ...uuid.UUID) error {
	p.got = append(p.got, lcEvent{kind, data, ids})
	return nil
}

type lcTx struct{}

func (lcTx) WithinTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type lifecycleFixture struct {
	svc    *LifecycleService
	repo   *fakeLifecycleRepo
	site   *fakeLifecycleSite
	notes  *recordingNotifier
	events *lcEvents
	clk    *clock.Fake
	users  map[uuid.UUID]repository.Contact
}

func newLifecycleFixture(t *testing.T) *lifecycleFixture {
	t.Helper()
	f := &lifecycleFixture{repo: &fakeLifecycleRepo{due: map[string][]repository.LifecycleDue{}}, site: &fakeLifecycleSite{},
		notes: &recordingNotifier{}, events: &lcEvents{}, clk: clock.NewFake(time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)),
		users: map[uuid.UUID]repository.Contact{}}
	f.svc = NewLifecycleService(LifecycleDeps{Tx: lcTx{}, Repo: f.repo, Site: f.site, Contacts: fakeContacts(f.users), Notifier: f.notes,
		Events: f.events, Clock: f.clk})
	return f
}

func (f *lifecycleFixture) user(marketing, push bool) uuid.UUID {
	id := uuid.New()
	f.users[id] = repository.Contact{UserID: id, Email: "x@example.com", Locale: "en", Status: domain.UserActive,
		NotifyEmail: true, NotifyPush: push, NotifyMarketing: marketing}
	return id
}

// --- tests ----------------------------------------------------------------

func TestLifecycleDefaultsWhenSettingMissing(t *testing.T) {
	f := newLifecycleFixture(t)
	set := f.svc.settings(context.Background())
	if set.AttributionDays != 7 || !set.Journeys[domain.JourneyFirstReading].Enabled || set.Journeys[domain.JourneyWinBack].Enabled ||
		set.Journeys[domain.JourneyWinBack].DelayHours != 336 || set.Journeys[domain.JourneyVerifyReminder].DelayHours != 24 {
		t.Fatalf("defaults: %+v", set)
	}
}

func TestLifecycleVerifyJourneyNeedsTheVerificationSeam(t *testing.T) {
	f := newLifecycleFixture(t)
	u := f.user(true, true)
	f.repo.due[domain.JourneyVerifyReminder] = []repository.LifecycleDue{{UserID: u, Cycle: 1}}
	if err := f.svc.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.repo.claimed) != 0 || len(f.notes.spec) != 0 {
		t.Fatalf("the journey ran without the seam: %d claims, %d notifications", len(f.repo.claimed), len(f.notes.spec))
	}
	if f.svc.journeyAvailable(domain.JourneyVerifyReminder) || !f.svc.journeyAvailable(domain.JourneyFirstReading) {
		t.Fatal("availability")
	}
}

func TestLifecycleVerifyReminderSendsOnceAndGivesTheClaimBackOnFailure(t *testing.T) {
	f := newLifecycleFixture(t)
	ok, bad := f.user(false, false), f.user(false, false)
	f.repo.due[domain.JourneyVerifyReminder] = []repository.LifecycleDue{{UserID: ok, Cycle: 1}, {UserID: bad, Cycle: 1}}
	var sent []uuid.UUID
	f.svc.SetVerificationReminder(func(_ context.Context, id uuid.UUID) error {
		if id == bad {
			return errors.New("mail down")
		}
		sent = append(sent, id)
		return nil
	})
	if err := f.svc.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 1 || sent[0] != ok {
		t.Fatalf("verification links sent to %v", sent)
	}
	if len(f.notes.spec) != 1 || f.notes.spec[0].Kind != NotifyLifecycleVerifyReminder || f.notes.sent[ok] != 1 || f.notes.sent[bad] != 0 {
		t.Fatalf("in-app notifications: %+v %v", f.notes.spec, f.notes.sent)
	}
	if len(f.repo.released) != 1 {
		t.Fatalf("claims given back: %d", len(f.repo.released))
	}
}

func TestLifecycleChannelsFollowConsent(t *testing.T) {
	f := newLifecycleFixture(t)
	opted, silent, noPush := f.user(true, true), f.user(false, true), f.user(true, false)
	f.repo.due[domain.JourneyFirstReading] = []repository.LifecycleDue{{UserID: opted, Cycle: 1, FreeMinutes: 5}, {UserID: silent, Cycle: 1}, {UserID: noPush, Cycle: 1}}
	if err := f.svc.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := map[uuid.UUID]domain.JSONMap{}
	for _, r := range f.repo.claimed {
		got[r.UserID] = r.Channels
	}
	// In-app is always attempted; marketing e-mail and push need the opt-in.
	if c := got[opted]; c["in_app"] != true || c["email"] != true || c["push"] != true {
		t.Errorf("opted in: %v", c)
	}
	if c := got[silent]; c["in_app"] != true || c["email"] != false || c["push"] != false {
		t.Errorf("not opted in (push switch on): %v", c)
	}
	if c := got[noPush]; c["email"] != true || c["push"] != false {
		t.Errorf("opted in, push off: %v", c)
	}
	// The kind's policy really is the marketing category.
	if p := PolicyFor(NotifyLifecycleFirstReading); p.Category != domain.EmailMarketing || !p.Email || !p.Push {
		t.Errorf("first_reading policy: %+v", p)
	}
	for _, kind := range []string{NotifyLifecycleInviteFriends, NotifyLifecycleWinBack} {
		if PolicyFor(kind).Category != domain.EmailMarketing {
			t.Errorf("%s is promotional: marketing category expected", kind)
		}
	}
	if p := PolicyFor(NotifyLifecycleVerifyReminder); p.Email || p.Push {
		t.Errorf("the verification reminder's e-mail is the verification link itself: %+v", p)
	}
	// The data states the wallet's free minutes.
	for _, s := range f.notes.spec {
		if s.Kind != NotifyLifecycleFirstReading || s.Link != "/psychics" {
			t.Errorf("spec: %+v", s)
		}
	}
}

func TestLifecycleLostClaimIsNotSent(t *testing.T) {
	f := newLifecycleFixture(t)
	mine, theirs := f.user(true, true), f.user(true, true)
	f.repo.conflicts = map[uuid.UUID]bool{theirs: true}
	f.repo.due[domain.JourneyFirstReading] = []repository.LifecycleDue{{UserID: mine, Cycle: 1}, {UserID: theirs, Cycle: 1}}
	if err := f.svc.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.notes.sent[mine] != 1 || f.notes.sent[theirs] != 0 {
		t.Fatalf("notifications: %v", f.notes.sent)
	}
}

func TestLifecycleAnotherInstanceHoldingTheLockSkipsTheRun(t *testing.T) {
	f := newLifecycleFixture(t)
	u := f.user(true, true)
	f.repo.due[domain.JourneyFirstReading] = []repository.LifecycleDue{{UserID: u, Cycle: 1}}
	f.repo.locked = true
	if err := f.svc.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.repo.claimed) != 0 {
		t.Fatal("ran without the lock")
	}
}

func TestLifecycleInviteJourneyIsSkippedWhileReferralsAreOff(t *testing.T) {
	f := newLifecycleFixture(t)
	u := f.user(true, true)
	f.repo.due[domain.JourneyAfterFirstReading] = []repository.LifecycleDue{{UserID: u, Cycle: 1}}
	// No seam: the program counts as off.
	if err := f.svc.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.repo.claimed) != 1 || f.repo.claimed[0].Status != domain.LifecycleSkipped || f.repo.claimed[0].SkipReason != domain.LifecycleSkipReferralsOff {
		t.Fatalf("claims: %+v", f.repo.claimed)
	}
	if len(f.notes.spec) != 0 {
		t.Fatalf("notified while the program is off: %+v", f.notes.spec)
	}
	// Turned on: the message goes out and links to the invite page.
	f2 := newLifecycleFixture(t)
	u2 := f2.user(true, true)
	f2.svc.SetReferralsEnabled(func(context.Context) bool { return true })
	f2.repo.due[domain.JourneyAfterFirstReading] = []repository.LifecycleDue{{UserID: u2, Cycle: 1}}
	if err := f2.svc.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f2.notes.spec) != 1 || f2.notes.spec[0].Kind != NotifyLifecycleInviteFriends || f2.notes.spec[0].Link != "/profile/invite" {
		t.Fatalf("spec: %+v", f2.notes.spec)
	}
}

func TestLifecycleWinBackStatesOnlyWhatIsTrue(t *testing.T) {
	f := newLifecycleFixture(t)
	f.site.st = &domain.SiteSetting{Key: domain.SiteSettingLifecycle, Version: 1, Value: defaultLifecycle().withEnabled(domain.JourneyWinBack, true).toMap()}
	u := f.user(true, false)
	f.repo.online = 0
	f.repo.due[domain.JourneyWinBack] = []repository.LifecycleDue{{UserID: u, Cycle: 2}}
	if err := f.svc.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.notes.spec) != 1 || f.notes.spec[0].Data["online_advisors"] != 0 || f.notes.spec[0].Link != "/profile" {
		t.Fatalf("nobody online: %+v", f.notes.spec)
	}
	if f.repo.claimed[0].Cycle != 2 {
		t.Fatalf("cycle: %d", f.repo.claimed[0].Cycle)
	}
	f = newLifecycleFixture(t)
	f.site.st = &domain.SiteSetting{Key: domain.SiteSettingLifecycle, Version: 1, Value: defaultLifecycle().withEnabled(domain.JourneyWinBack, true).toMap()}
	u = f.user(true, false)
	f.repo.online = 3
	f.repo.due[domain.JourneyWinBack] = []repository.LifecycleDue{{UserID: u, Cycle: 1}}
	_ = f.svc.RunDue(context.Background())
	if f.notes.spec[0].Data["online_advisors"] != 3 || f.notes.spec[0].Link != "/psychics" {
		t.Fatalf("advisors online: %+v", f.notes.spec)
	}
}

func (s LifecycleSettings) withEnabled(key string, on bool) LifecycleSettings {
	j := s.Journeys[key]
	j.Enabled = on
	s.Journeys[key] = j
	return s
}

func TestLifecycleDisabledJourneyDoesNotRun(t *testing.T) {
	f := newLifecycleFixture(t)
	f.site.st = &domain.SiteSetting{Key: domain.SiteSettingLifecycle, Version: 1, Value: defaultLifecycle().withEnabled(domain.JourneyFirstReading, false).toMap()}
	u := f.user(true, true)
	f.repo.due[domain.JourneyFirstReading] = []repository.LifecycleDue{{UserID: u, Cycle: 1}}
	_ = f.svc.RunDue(context.Background())
	if len(f.repo.claimed) != 0 {
		t.Fatal("disabled journey ran")
	}
}

func TestLifecycleInactiveRecipientIsNotMessaged(t *testing.T) {
	f := newLifecycleFixture(t)
	u := f.user(true, true)
	c := f.users[u]
	c.Status = domain.UserDeactivated
	f.users[u] = c
	f.repo.due[domain.JourneyFirstReading] = []repository.LifecycleDue{{UserID: u, Cycle: 1}}
	_ = f.svc.RunDue(context.Background())
	if len(f.repo.claimed) != 0 || len(f.notes.spec) != 0 {
		t.Fatal("deactivated account was messaged")
	}
}

func TestLifecycleConversionWindow(t *testing.T) {
	f := newLifecycleFixture(t)
	settled := f.clk.Now()
	client := uuid.New()
	f.svc.OnSessionSettled(context.Background(), domain.Session{ClientID: client, BilledMinutes: 3, SettledAt: &settled})
	if len(f.repo.readers) != 1 || f.repo.readers[0] != client {
		t.Fatalf("reader rows: %v", f.repo.readers)
	}
	if len(f.repo.converts) != 1 {
		t.Fatalf("conversion calls: %d", len(f.repo.converts))
	}
	c := f.repo.converts[0]
	if c.user != client || !c.to.Equal(settled) || !c.from.Equal(settled.AddDate(0, 0, -7)) {
		t.Fatalf("window: %+v", c)
	}
	// A longer attribution window from the settings applies.
	f.site.st = &domain.SiteSetting{Key: domain.SiteSettingLifecycle, Version: 1, Value: domain.JSONMap{"journeys": map[string]any{}, "attribution_days": float64(14)}}
	f.svc.OnSessionSettled(context.Background(), domain.Session{ClientID: client, BilledMinutes: 1, SettledAt: &settled})
	if got := f.repo.converts[1]; !got.from.Equal(settled.AddDate(0, 0, -14)) {
		t.Fatalf("window: %+v", got)
	}
	// A session without a billed minute is not a completed reading.
	f.svc.OnSessionSettled(context.Background(), domain.Session{ClientID: client, BilledMinutes: 0, SettledAt: &settled})
	if len(f.repo.converts) != 2 {
		t.Fatal("an unbilled session converted")
	}
}

func TestLifecycleUpdateValidatesAndKeepsVersions(t *testing.T) {
	f := newLifecycleFixture(t)
	f.site.st = &domain.SiteSetting{Key: domain.SiteSettingLifecycle, Version: 3, Value: defaultLifecycle().toMap()}
	ctx := context.Background()
	bad := 0
	_, err := f.svc.UpdateLifecycle(ctx, LifecycleInput{Journeys: map[string]LifecycleJourneyInput{"nope": {}, domain.JourneyWinBack: {DelayHours: &bad}}, AttributionDays: &bad})
	var ae *apperror.Error
	if !errors.As(err, &ae) || ae.Code != "validation_failed" {
		t.Fatalf("validation: %v", err)
	}
	if _, err := f.svc.UpdateLifecycle(ctx, LifecycleInput{}); err == nil {
		t.Fatal("an empty update must be refused")
	}
	stale := 2
	on := true
	if _, err := f.svc.UpdateLifecycle(ctx, LifecycleInput{Version: &stale, Journeys: map[string]LifecycleJourneyInput{domain.JourneyWinBack: {Enabled: &on}}}); err == nil {
		t.Fatal("a stale version must conflict")
	}
	if f.site.saved != nil {
		t.Fatal("a refused update was saved")
	}
	// An unchanged value is not a new version.
	fresh := 3
	if _, err := f.svc.UpdateLifecycle(ctx, LifecycleInput{Version: &fresh, Journeys: map[string]LifecycleJourneyInput{domain.JourneyFirstReading: {Enabled: &on}}}); err == nil && f.site.saved != nil {
		t.Fatal("an unchanged setting was saved")
	}
}

func TestOnboardingChecklist(t *testing.T) {
	f := newLifecycleFixture(t)
	ctx := context.Background()
	user := uuid.New()
	// Without e-mail verification and the referral program, their steps are
	// left out: every step shown can be completed.
	got, err := f.svc.Onboarding(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 2 || got.Completed != 0 || got.Dismissed || got.Steps[0].Key != OnboardingBirthDetails || got.Steps[1].Key != OnboardingFirstReading {
		t.Fatalf("without the seams: %+v", got)
	}
	f.svc.SetVerificationReminder(func(context.Context, uuid.UUID) error { return nil })
	f.svc.SetReferralsEnabled(func(context.Context) bool { return true })
	got, _ = f.svc.Onboarding(ctx, user)
	if got.Total != 4 || got.Completed != 0 || got.Steps[0].Key != OnboardingVerifyEmail || got.Steps[3].Key != OnboardingInviteFriend {
		t.Fatalf("fresh client: %+v", got)
	}
	f.repo.facts = repository.OnboardingFacts{Verified: true, BirthDetails: true}
	f.svc.SetInvitedFriends(func(context.Context, uuid.UUID) (int, error) { return 2, nil })
	got, _ = f.svc.Onboarding(ctx, user)
	if got.Completed != 3 || !got.Steps[3].Done || got.Steps[2].Done {
		t.Fatalf("progress: %+v", got)
	}
	// The referral seam failing leaves the step open, not the whole checklist broken.
	f.svc.SetInvitedFriends(func(context.Context, uuid.UUID) (int, error) { return 0, errors.New("down") })
	got, err = f.svc.Onboarding(ctx, user)
	if err != nil || got.Steps[3].Done {
		t.Fatalf("seam failure: %+v %v", got, err)
	}
	// The program switched off: the step goes away.
	f.svc.SetReferralsEnabled(func(context.Context) bool { return false })
	if got, _ = f.svc.Onboarding(ctx, user); got.Total != 3 || got.Steps[2].Key != OnboardingFirstReading {
		t.Fatalf("referrals off: %+v", got)
	}
	got, _ = f.svc.DismissOnboarding(ctx, user, true)
	if !got.Dismissed {
		t.Fatal("dismiss")
	}
	got, _ = f.svc.DismissOnboarding(ctx, user, false)
	if got.Dismissed {
		t.Fatal("undo")
	}
	// Every tab and device of the client follows: one durable event per change,
	// carrying the new checklist, to that client only.
	if len(f.events.got) != 2 {
		t.Fatalf("events: %+v", f.events.got)
	}
	for i, want := range []bool{true, false} {
		ev := f.events.got[i]
		view, ok := ev.data.(*OnboardingView)
		if ev.kind != "onboarding.updated" || !ok || view.Dismissed != want || len(ev.users) != 1 || ev.users[0] != user {
			t.Fatalf("event %d: %+v", i, ev)
		}
	}
}

func TestLifecycleWalksTheDueUsersInKeysetBatches(t *testing.T) {
	f := newLifecycleFixture(t)
	var all []repository.LifecycleDue
	at := f.clk.Now().Add(-30 * time.Hour)
	for i := 0; i < 2*lifecycleBatch+200; i++ {
		u := f.user(true, true)
		all = append(all, repository.LifecycleDue{UserID: u, EventAt: at.Add(time.Duration(i) * time.Second), Cycle: 1})
	}
	f.repo.due[domain.JourneyFirstReading] = all
	if err := f.svc.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.repo.claimed) != len(all) || len(f.notes.spec) != len(all) {
		t.Fatalf("claimed %d, notified %d of %d", len(f.repo.claimed), len(f.notes.spec), len(all))
	}
	var qs []repository.LifecycleDueQuery
	for _, q := range f.repo.queries {
		if q.Journey == domain.JourneyFirstReading {
			qs = append(qs, q)
		}
	}
	// Three batches; each continues after the last user of the previous one,
	// over the same window (fixed for the run).
	if len(qs) != 3 || qs[0].After != nil {
		t.Fatalf("queries: %+v", qs)
	}
	for i, q := range qs[1:] {
		last := all[(i+1)*lifecycleBatch-1]
		if q.After == nil || q.After.UserID != last.UserID || !q.After.At.Equal(last.EventAt) || !q.Cutoff.Equal(qs[0].Cutoff) || !q.Floor.Equal(qs[0].Floor) {
			t.Fatalf("batch %d: %+v, want after %v", i+2, q, last)
		}
	}
	// The window: delay 24 h, catch-up 72 h, cap 48 h.
	now := f.clk.Now()
	if q := qs[0]; !q.Cutoff.Equal(now.Add(-24*time.Hour)) || !q.Floor.Equal(now.Add(-96*time.Hour)) || !q.CapSince.Equal(now.Add(-48*time.Hour)) {
		t.Fatalf("window: %+v", q)
	}
}

// Win-back looks at one catch-up band per cycle: readers whose last reading
// passed the delay (cycle 1), or the delay plus one or two 30-day cycles, in
// the last 72 hours; inactivity is always measured from the delay.
func TestLifecycleWinBackLooksAtOneBandPerCycle(t *testing.T) {
	now := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	qs := dueWindows(domain.JourneyWinBack, LifecycleJourney{Enabled: true, DelayHours: 336}, now)
	if len(qs) != 3 {
		t.Fatalf("bands: %d", len(qs))
	}
	day := 24 * time.Hour
	for k, q := range qs {
		cutoff := now.Add(-14*day - time.Duration(k)*30*day)
		if !q.Cutoff.Equal(cutoff) || !q.Floor.Equal(cutoff.Add(-72*time.Hour)) || !q.InactiveSince.Equal(now.Add(-14*day)) ||
			!q.CycleSince.Equal(now.Add(-30*day)) || q.MaxCycles != 3 || !q.CapSince.Equal(now.Add(-48*time.Hour)) || q.After != nil {
			t.Fatalf("band %d: %+v", k+1, q)
		}
	}
	// The other journeys: one band, no inactivity condition.
	for _, key := range []string{domain.JourneyVerifyReminder, domain.JourneyFirstReading, domain.JourneyAfterFirstReading} {
		qs := dueWindows(key, LifecycleJourney{Enabled: true, DelayHours: 24}, now)
		if len(qs) != 1 || !qs[0].Cutoff.Equal(now.Add(-day)) || !qs[0].Floor.Equal(now.Add(-4*day)) || !qs[0].InactiveSince.IsZero() {
			t.Fatalf("%s: %+v", key, qs)
		}
	}
}

func TestLifecycleAdminViewExplainsJourneysThatReachNobody(t *testing.T) {
	f := newLifecycleFixture(t)
	notes := func() map[string]LifecycleJourneyView {
		view, err := f.svc.Lifecycle(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]LifecycleJourneyView{}
		for _, j := range view.Journeys {
			out[j.Key] = j
		}
		return out
	}
	got := notes()
	if j := got[domain.JourneyVerifyReminder]; j.Available || j.Note != "verification_unavailable" {
		t.Fatalf("verify reminder without the feature: %+v", j)
	}
	if j := got[domain.JourneyAfterFirstReading]; !j.Available || j.Note != "referrals_disabled" {
		t.Fatalf("invite journey with the program off: %+v", j)
	}
	if j := got[domain.JourneyFirstReading]; j.Note != "" || !j.Promotional {
		t.Fatalf("first reading: %+v", j)
	}
	f.svc.SetVerificationReminder(func(context.Context, uuid.UUID) error { return nil })
	f.svc.SetReferralsEnabled(func(context.Context) bool { return true })
	for key, j := range notes() {
		if !j.Available || j.Note != "" {
			t.Fatalf("%s with every feature on: %+v", key, j)
		}
	}
}

func TestLifecycleFunnelWithoutTheReferralSeam(t *testing.T) {
	f := newLifecycleFixture(t)
	f.repo.LifecycleRepository = funnelRepo{}
	out, err := f.svc.Funnel(context.Background(), 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Steps) != 6 || out.Steps[4].Key != "invited_friend" || out.Steps[4].Count != nil || *out.Steps[0].Count != 10 || out.Steps[3].Rate != 0.2 {
		t.Fatalf("funnel: %+v", out)
	}
	if _, err := f.svc.Funnel(context.Background(), 400); err == nil {
		t.Fatal("window too long")
	}
}

type funnelRepo struct{ repository.LifecycleRepository }

func (funnelRepo) Funnel(context.Context, time.Time, time.Time) (repository.LifecycleFunnel, error) {
	return repository.LifecycleFunnel{SignedUp: 10, Verified: 6, BirthDetails: 4, FirstReading: 2, SecondReading: 1}, nil
}
