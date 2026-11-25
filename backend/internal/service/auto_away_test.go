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

func TestAutoAwaySettingsParsing(t *testing.T) {
	st := func(v domain.JSONMap) *domain.SiteSetting { return &domain.SiteSetting{Value: v} }
	cases := []struct {
		name string
		in   *domain.SiteSetting
		want AutoAwaySettings
	}{
		{"missing document", nil, AutoAwaySettings{true, 3}},
		{"empty document", st(domain.JSONMap{}), AutoAwaySettings{true, 3}},
		{"stored values", st(domain.JSONMap{"enabled": false, "missed_requests": float64(5)}), AutoAwaySettings{false, 5}},
		{"lower bound", st(domain.JSONMap{"enabled": true, "missed_requests": float64(2)}), AutoAwaySettings{true, 2}},
		{"upper bound", st(domain.JSONMap{"enabled": true, "missed_requests": float64(10)}), AutoAwaySettings{true, 10}},
		{"below range", st(domain.JSONMap{"enabled": true, "missed_requests": float64(1)}), AutoAwaySettings{true, 3}},
		{"above range", st(domain.JSONMap{"enabled": false, "missed_requests": float64(11)}), AutoAwaySettings{false, 3}},
		{"wrong types", st(domain.JSONMap{"enabled": "yes", "missed_requests": "4"}), AutoAwaySettings{true, 3}},
	}
	for _, c := range cases {
		if got := autoAwayFrom(c.in); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
	if got := autoAwayFrom(&domain.SiteSetting{Value: defaultAutoAway().toMap()}); got != defaultAutoAway() {
		t.Errorf("round trip: %+v", got)
	}
}

// aaPsychics is an in-memory advisor table with the auto-away statements.
type aaPsychics struct {
	repository.PsychicRepository
	mu           sync.Mutex
	missed       int
	availability domain.Availability
}

func (p *aaPsychics) IncrementMissed(context.Context, uuid.UUID) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.missed++
	return p.missed, nil
}
func (p *aaPsychics) ResetMissed(context.Context, uuid.UUID) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.missed = 0
	return nil
}
func (p *aaPsychics) ResetMissedAtLeast(_ context.Context, _ uuid.UUID, min int) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.missed < min {
		return false, nil
	}
	p.missed = 0
	return true, nil
}
func (p *aaPsychics) SetAwayIfOnline(context.Context, uuid.UUID) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.availability != domain.AvailabilityOnline {
		return false, nil
	}
	p.availability = domain.AvailabilityAway
	return true, nil
}

type aaChat struct {
	repository.ChatRepository
	open *domain.Session
}

func (c aaChat) OpenSessionForPsychic(context.Context, uuid.UUID) (*domain.Session, error) {
	if c.open == nil {
		return nil, repository.ErrNotFound
	}
	return c.open, nil
}

type aaPublisher struct{ n int }

func (p *aaPublisher) PublishStatus(context.Context, uuid.UUID) { p.n++ }

type aaFixture struct {
	svc    *AutoAwayService
	psy    *aaPsychics
	site   *fakeLifecycleSite
	notes  *recordingNotifier
	status *aaPublisher
	audit  *auditSink
	id     uuid.UUID
}

func newAutoAwayFixture(t *testing.T, open *domain.Session) *aaFixture {
	t.Helper()
	f := &aaFixture{psy: &aaPsychics{availability: domain.AvailabilityOnline}, site: &fakeLifecycleSite{}, notes: &recordingNotifier{},
		status: &aaPublisher{}, audit: &auditSink{}, id: uuid.New()}
	f.svc = NewAutoAwayService(AutoAwayDeps{Tx: lcTx{}, Psychics: f.psy, Chat: aaChat{open: open}, Site: f.site, Audit: f.audit,
		Advisors: f.status, Notifier: f.notes, Tracker: &analytics.Recorder{}, Clock: clock.NewFake(time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC))})
	return f
}

func (f *aaFixture) miss(n int) {
	for i := 0; i < n; i++ {
		f.svc.OnRequestMissed(context.Background(), f.id)
	}
}

func TestAutoAwayThreshold(t *testing.T) {
	f := newAutoAwayFixture(t, nil)
	f.miss(2)
	if f.psy.availability != domain.AvailabilityOnline || f.psy.missed != 2 || len(f.notes.spec) != 0 {
		t.Fatalf("below the threshold: %s, %d, %d notices", f.psy.availability, f.psy.missed, len(f.notes.spec))
	}
	f.miss(1)
	if f.psy.availability != domain.AvailabilityAway || f.psy.missed != 0 || f.status.n != 1 {
		t.Fatalf("at the threshold: %s, counter %d, %d status publications", f.psy.availability, f.psy.missed, f.status.n)
	}
	if len(f.notes.spec) != 1 || f.notes.spec[0].Kind != NotifyAutoAway || f.notes.spec[0].Data["missed"] != 3 || f.notes.spec[0].Link != "/chatroom" {
		t.Fatalf("notification: %+v", f.notes.spec)
	}
	// Further misses on an Away advisor count again but change nothing.
	f.miss(3)
	if len(f.notes.spec) != 1 || f.psy.missed != 0 {
		t.Fatalf("after going away: %d notices, counter %d", len(f.notes.spec), f.psy.missed)
	}
}

func TestAutoAwayUsesTheConfiguredThreshold(t *testing.T) {
	f := newAutoAwayFixture(t, nil)
	f.site.st = &domain.SiteSetting{Key: domain.SiteSettingAdvisorAutoAway, Version: 1, Value: domain.JSONMap{"enabled": true, "missed_requests": float64(2)}}
	f.miss(2)
	if f.psy.availability != domain.AvailabilityAway || f.notes.spec[0].Data["missed"] != 2 {
		t.Fatalf("threshold 2: %s, %+v", f.psy.availability, f.notes.spec)
	}
}

func TestAutoAwayDisabledCountsNothing(t *testing.T) {
	f := newAutoAwayFixture(t, nil)
	f.site.st = &domain.SiteSetting{Key: domain.SiteSettingAdvisorAutoAway, Version: 1, Value: domain.JSONMap{"enabled": false, "missed_requests": float64(3)}}
	f.psy.missed = 2
	f.miss(5)
	if f.psy.availability != domain.AvailabilityOnline || f.psy.missed != 0 || len(f.notes.spec) != 0 {
		t.Fatalf("disabled: %s, counter %d, %d notices", f.psy.availability, f.psy.missed, len(f.notes.spec))
	}
}

func TestAutoAwayNeverMovesAnAdvisorWhoIsNotOnline(t *testing.T) {
	for _, av := range []domain.Availability{domain.AvailabilityAway, domain.AvailabilityOffline} {
		f := newAutoAwayFixture(t, nil)
		f.psy.availability = av
		f.miss(3)
		if f.psy.availability != av || f.psy.missed != 0 || len(f.notes.spec) != 0 || f.status.n != 0 {
			t.Errorf("%s: now %s, counter %d, %d notices, %d publications", av, f.psy.availability, f.psy.missed, len(f.notes.spec), f.status.n)
		}
	}
}

func TestAutoAwaySkipsAnAdvisorInAReading(t *testing.T) {
	f := newAutoAwayFixture(t, &domain.Session{Status: domain.SessionActive})
	f.miss(3)
	if f.psy.availability != domain.AvailabilityOnline || f.psy.missed != 0 || len(f.notes.spec) != 0 {
		t.Fatalf("in a reading: %s, counter %d, %d notices", f.psy.availability, f.psy.missed, len(f.notes.spec))
	}
	// A pending request from another client is not a reading.
	f = newAutoAwayFixture(t, &domain.Session{Status: domain.SessionRequested})
	f.miss(3)
	if f.psy.availability != domain.AvailabilityAway {
		t.Fatalf("with a pending request: %s", f.psy.availability)
	}
}

func TestAutoAwayAnswersResetTheCount(t *testing.T) {
	f := newAutoAwayFixture(t, nil)
	f.miss(2)
	f.svc.OnRequestAnswered(context.Background(), f.id)
	f.miss(2)
	if f.psy.availability != domain.AvailabilityOnline || f.psy.missed != 2 {
		t.Fatalf("miss, miss, answer, miss, miss: %s, counter %d", f.psy.availability, f.psy.missed)
	}
}

func TestAutoAwayConcurrentMissesNotifyOnce(t *testing.T) {
	f := newAutoAwayFixture(t, nil)
	f.psy.missed = 1
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f.svc.OnRequestMissed(context.Background(), f.id)
		}()
	}
	wg.Wait()
	if len(f.notes.spec) != 1 || f.status.n != 1 {
		t.Fatalf("%d notifications, %d status publications", len(f.notes.spec), f.status.n)
	}
}

func TestAutoAwayUpdateValidation(t *testing.T) {
	f := newAutoAwayFixture(t, nil)
	f.site.st = &domain.SiteSetting{Key: domain.SiteSettingAdvisorAutoAway, Version: 4, Value: domain.JSONMap{"enabled": true, "missed_requests": float64(3)}}
	ctx := context.Background()
	five, one, eleven, zero := 5, 1, 11, 0
	for name, in := range map[string]AutoAwayInput{
		"too low":     {MissedRequests: &one},
		"too high":    {MissedRequests: &eleven},
		"zero":        {MissedRequests: &zero},
		"empty":       {},
		"bad version": {MissedRequests: &five, Version: &zero},
	} {
		if _, err := f.svc.UpdateSettings(ctx, in); errCode(err) != apperror.CodeValidation {
			t.Errorf("%s: %v", name, err)
		}
	}
	if f.site.version != 0 || len(f.audit.rows) != 0 {
		t.Fatal("an invalid update was saved")
	}
	// Stale version.
	stale := 3
	if _, err := f.svc.UpdateSettings(ctx, AutoAwayInput{MissedRequests: &five, Version: &stale}); errCode(err) != apperror.CodeVersionConflict {
		t.Fatalf("stale: %v", err)
	}
	// A valid change is saved with the next version and audited; the same value again is not.
	cur := 4
	out, err := f.svc.UpdateSettings(ctx, AutoAwayInput{MissedRequests: &five, Version: &cur})
	if err != nil || out.MissedRequests != 5 || out.Version != 5 || !out.Enabled || len(f.audit.rows) != 1 || f.audit.rows[0].Action != "advisors.auto_away.updated" {
		t.Fatalf("update: %+v, %v, audit %+v", out, err, f.audit.rows)
	}
	if out, err = f.svc.UpdateSettings(ctx, AutoAwayInput{MissedRequests: &five}); err != nil || out.Version != 5 || len(f.audit.rows) != 1 {
		t.Fatalf("unchanged: %+v, %v", out, err)
	}
}
