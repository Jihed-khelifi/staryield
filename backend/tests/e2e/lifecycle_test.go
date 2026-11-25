package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// Lifecycle messaging (growth): the journeys, their eligibility, consent and
// frequency cap, exactly-once delivery, conversion, the admin settings and
// reports, and the onboarding checklist. The database is shared by the suite:
// every test starts from (and restores) the seed state of the lifecycle tables
// and setting.

const (
	lifecyclePath = "/api/v1/admin/lifecycle"
	funnelPath    = "/api/v1/admin/lifecycle/funnel"
)

func (e *env) lcStart() {
	e.t.Helper()
	reset := func() {
		// Clients of the tests that ran before (aged, or created under a moved
		// clock) must not be due now: they leave every sign-up window.
		e.app.DB.Exec(`UPDATE users SET created_at = now() - interval '2000 days' WHERE role = 'client'`)
		e.app.DB.Exec(`DELETE FROM lifecycle_messages`)
		e.app.DB.Exec(`DELETE FROM lifecycle_readers`)
		e.app.DB.Exec(`UPDATE site_settings SET version = version + 1, value = '{"journeys": {
			"verify_reminder": {"enabled": true, "delay_hours": 24}, "first_reading": {"enabled": true, "delay_hours": 24},
			"after_first_reading": {"enabled": true, "delay_hours": 24}, "win_back": {"enabled": false, "delay_hours": 336}},
			"attribution_days": 7}' WHERE key = 'lifecycle'`)
	}
	reset()
	e.t.Cleanup(reset)
}

// lcRun runs the job once and waits until the notifications it sent are persisted.
func (e *env) lcRun() {
	e.t.Helper()
	before := e.app.Dispatcher.Processed()
	var sentBefore, sentAfter int64
	e.app.DB.Model(&domain.LifecycleMessage{}).Where("status = 'sent'").Count(&sentBefore)
	if err := e.app.Scheduler.RunJob(context.Background(), service.LifecycleJob); err != nil {
		e.t.Fatalf("lifecycle job: %v", err)
	}
	e.app.DB.Model(&domain.LifecycleMessage{}).Where("status = 'sent'").Count(&sentAfter)
	want := before + (sentAfter - sentBefore)
	deadline := time.Now().Add(5 * time.Second)
	for e.app.Dispatcher.Processed() < want {
		if time.Now().After(deadline) {
			e.t.Fatalf("notifications not delivered: %d of %d", e.app.Dispatcher.Processed()-before, sentAfter-sentBefore)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// lcAge makes an account older (sign-up time moves into the past).
func (e *env) lcAge(id uuid.UUID, d time.Duration) {
	e.t.Helper()
	at := e.clock.Now().Add(-d)
	if err := e.app.DB.Exec(`UPDATE users SET created_at = ?, updated_at = ? WHERE id = ?`, at, at, id).Error; err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) lcFreeMinutes(id uuid.UUID, n int) {
	e.t.Helper()
	if err := e.app.DB.Exec(`INSERT INTO wallets (user_id, free_minutes) VALUES (?, ?)
		ON CONFLICT (user_id) DO UPDATE SET free_minutes = EXCLUDED.free_minutes`, id, n).Error; err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) lcMessages(id uuid.UUID, journey string) []domain.LifecycleMessage {
	e.t.Helper()
	var out []domain.LifecycleMessage
	q := e.app.DB.Where("user_id = ?", id)
	if journey != "" {
		q = q.Where("journey = ?", journey)
	}
	if err := q.Order("created_at, cycle").Find(&out).Error; err != nil {
		e.t.Fatal(err)
	}
	return out
}

func (e *env) lcNotifications(id uuid.UUID, kind string) []domain.Notification {
	e.t.Helper()
	var out []domain.Notification
	if err := e.app.DB.Where("user_id = ? AND kind = ?", id, kind).Order("created_at").Find(&out).Error; err != nil {
		e.t.Fatal(err)
	}
	return out
}

// lcReader gives a client a completed-readings record without running a chat.
func (e *env) lcReader(id uuid.UUID, first, last time.Time, readings int) {
	e.t.Helper()
	if err := e.app.DB.Exec(`INSERT INTO lifecycle_readers (user_id, first_reading_at, last_reading_at, readings) VALUES (?, ?, ?, ?)
		ON CONFLICT (user_id) DO UPDATE SET first_reading_at = EXCLUDED.first_reading_at, last_reading_at = EXCLUDED.last_reading_at,
		readings = EXCLUDED.readings`, id, first, last, readings).Error; err != nil {
		e.t.Fatal(err)
	}
}

// lcLapse makes a reader inactive since d ago: sign-in, sessions and reading.
func (e *env) lcLapse(id uuid.UUID, d time.Duration) {
	e.t.Helper()
	at := e.clock.Now().Add(-d)
	e.lcAge(id, d+24*time.Hour)
	for _, q := range []string{`UPDATE users SET last_login_at = ? WHERE id = ?`,
		`UPDATE refresh_tokens SET created_at = ?, last_used_at = ? WHERE user_id = ?`} {
		args := []any{at, id}
		if q[:20] == `UPDATE refresh_token` {
			args = []any{at, at, id}
		}
		if err := e.app.DB.Exec(q, args...).Error; err != nil {
			e.t.Fatal(err)
		}
	}
	e.lcReader(id, at.Add(-48*time.Hour), at, 2)
}

func (e *env) lcOptIn(id uuid.UUID) {
	e.setPrefs(id, map[string]any{"notify_marketing": true, "notify_push": true})
}

func TestLifecycleFirstReadingNudge(t *testing.T) {
	e := newEnv(t)
	e.lcStart()
	due, optedIn := e.registerClient(), e.registerClient()
	young, old, advisor, deactivated, restricted, noBirth := e.registerClient(), e.registerClient(), e.createPsychic(40), e.registerClient(), e.registerClient(), e.registerClient()
	for _, c := range []*client{due, optedIn, deactivated, restricted, noBirth} {
		e.lcAge(c.user.ID, 25*time.Hour)
	}
	e.lcAge(young.user.ID, 2*time.Hour)
	e.lcAge(old.user.ID, 10*24*time.Hour) // beyond the catch-up window
	e.lcAge(advisor.user.ID, 25*time.Hour)
	e.app.DB.Exec(`UPDATE users SET status = 'deactivated' WHERE id = ?`, deactivated.user.ID)
	e.app.DB.Exec(`UPDATE users SET age_restricted_at = now(), adult_confirmed_at = NULL WHERE id = ?`, restricted.user.ID)
	e.app.DB.Exec(`UPDATE users SET adult_confirmed_at = NULL WHERE id = ?`, noBirth.user.ID)
	e.lcFreeMinutes(optedIn.user.ID, 5)
	e.lcFreeMinutes(due.user.ID, 0)
	e.lcOptIn(optedIn.user.ID)

	e.lcRun()
	for _, c := range []*client{due, optedIn} {
		rows := e.lcMessages(c.user.ID, domain.JourneyFirstReading)
		if len(rows) != 1 || rows[0].Status != domain.LifecycleSent || rows[0].Cycle != 1 || rows[0].Channels["in_app"] != true {
			t.Fatalf("first_reading rows of %s: %+v", c.user.Email, rows)
		}
	}
	for name, c := range map[string]*client{"too young": young, "too old": old, "an advisor": advisor, "deactivated": deactivated,
		"age restricted": restricted, "no adult confirmation": noBirth} {
		if rows := e.lcMessages(c.user.ID, ""); len(rows) != 0 {
			t.Errorf("%s was messaged: %+v", name, rows)
		}
	}
	// Truthful copy: the free minutes the wallet holds, a link to the advisors.
	n := e.lcNotifications(optedIn.user.ID, service.NotifyLifecycleFirstReading)
	if len(n) != 1 || n[0].Link != "/psychics" || n[0].Data["free_minutes"] != float64(5) {
		t.Fatalf("opted-in notification: %+v", n)
	}
	if n := e.lcNotifications(due.user.ID, service.NotifyLifecycleFirstReading); len(n) != 1 || n[0].Data["free_minutes"] != float64(0) {
		t.Fatalf("neutral notification: %+v", n)
	}
	// Consent: promotional e-mail only for the opted-in client (and the outbox
	// cancels it while the provider is transactional-only); nothing is queued
	// for the client who never opted in.
	if rows := e.outbox(optedIn.user.ID); len(rows) != 1 || rows[0].Category != domain.EmailMarketing || rows[0].Status != domain.EmailCancelled {
		t.Fatalf("opted-in e-mail: %+v", rows)
	}
	if rows := e.outbox(due.user.ID); len(rows) != 0 {
		t.Fatalf("e-mail queued without the marketing opt-in: %+v", rows)
	}
	if got := e.lcMessages(optedIn.user.ID, "")[0].Channels; got["email"] != true || got["push"] != true {
		t.Fatalf("recorded channels: %v", got)
	}
	if got := e.lcMessages(due.user.ID, "")[0].Channels; got["email"] != false || got["push"] != false {
		t.Fatalf("recorded channels without consent: %v", got)
	}
	if msgs := sentTo(e.flushMail(), optedIn.user.Email); len(msgs) != 0 {
		t.Fatalf("marketing e-mail reached the transactional provider: %+v", msgs)
	}

	// Exactly once: running again, even much later, sends nothing more.
	e.lcRun()
	e.clock.Advance(26 * time.Hour)
	e.lcRun()
	for _, c := range []*client{due, optedIn} {
		if n := e.lcNotifications(c.user.ID, service.NotifyLifecycleFirstReading); len(n) != 1 {
			t.Fatalf("%d notifications after re-runs", len(n))
		}
	}
}

func TestLifecycleMarketingEmailIsSentWhenAProviderExists(t *testing.T) {
	e := newEnv(t, func(c *configAlias) { c.Mail.AllowMarketing = true })
	e.lcStart()
	optedIn, silent := e.registerClient(), e.registerClient()
	for _, c := range []*client{optedIn, silent} {
		e.lcAge(c.user.ID, 25*time.Hour)
	}
	e.lcOptIn(optedIn.user.ID)
	e.lcFreeMinutes(optedIn.user.ID, 1)
	e.lcRun()
	msgs := e.flushMail()
	if got := sentTo(msgs, silent.user.Email); len(got) != 0 {
		t.Fatalf("e-mail without the opt-in: %+v", got)
	}
	got := sentTo(msgs, optedIn.user.Email)
	if len(got) != 1 || got[0].Category != "marketing" || got[0].Subject != "Your free minute is waiting" || !strings.Contains(got[0].Text, "tips, offers and reminders") {
		t.Fatalf("opted-in e-mail: %+v", got)
	}
}

func TestLifecycleVerifyReminder(t *testing.T) {
	e := newEnv(t)
	e.lcStart()
	e.app.DB.Exec(`UPDATE site_settings SET version = version + 1, value = jsonb_set(value, '{journeys,first_reading,enabled}', 'false') WHERE key = 'lifecycle'`)
	unverified, verified, young := e.registerClient(), e.registerClient(), e.registerClient()
	for _, c := range []*client{unverified, verified} {
		e.lcAge(c.user.ID, 25*time.Hour)
	}
	e.lcAge(young.user.ID, time.Hour)
	e.app.DB.Exec(`UPDATE users SET email_verified_at = now() WHERE id = ?`, verified.user.ID)
	e.app.DB.Exec(`UPDATE users SET email_verified_at = NULL WHERE id IN (?, ?)`, unverified.user.ID, young.user.ID)

	// Without the e-mail verification feature the journey does not run.
	e.lcRun()
	if rows := e.lcMessages(unverified.user.ID, domain.JourneyVerifyReminder); len(rows) != 0 {
		t.Fatalf("ran without the verification seam: %+v", rows)
	}
	var mu sync.Mutex
	var links []uuid.UUID
	e.app.Services.Lifecycle.SetVerificationReminder(func(_ context.Context, id uuid.UUID) error {
		mu.Lock()
		defer mu.Unlock()
		links = append(links, id)
		return nil
	})
	e.lcRun()
	e.lcRun()
	mu.Lock()
	defer mu.Unlock()
	if len(links) != 1 || links[0] != unverified.user.ID {
		t.Fatalf("verification links sent to %v", links)
	}
	if n := e.lcNotifications(unverified.user.ID, service.NotifyLifecycleVerifyReminder); len(n) != 1 || n[0].Link != "/profile/settings#account" {
		t.Fatalf("in-app reminder: %+v", n)
	}
	if rows := e.lcMessages(verified.user.ID, domain.JourneyVerifyReminder); len(rows) != 0 {
		t.Fatalf("a verified client was reminded: %+v", rows)
	}
	if rows := e.lcMessages(young.user.ID, domain.JourneyVerifyReminder); len(rows) != 0 {
		t.Fatalf("a new client was reminded: %+v", rows)
	}
	// The reminder is transactional: no promotional e-mail row is queued for it.
	if rows := e.outbox(unverified.user.ID); len(rows) != 0 {
		t.Fatalf("outbox: %+v", rows)
	}
}

func TestLifecycleFailedVerificationReminderIsRetried(t *testing.T) {
	e := newEnv(t)
	e.lcStart()
	e.app.DB.Exec(`UPDATE site_settings SET version = version + 1, value = jsonb_set(value, '{journeys,first_reading,enabled}', 'false') WHERE key = 'lifecycle'`)
	c := e.registerClient()
	e.lcAge(c.user.ID, 25*time.Hour)
	fail := true
	var calls int
	e.app.Services.Lifecycle.SetVerificationReminder(func(context.Context, uuid.UUID) error {
		calls++
		if fail {
			return context.DeadlineExceeded
		}
		return nil
	})
	e.lcRun()
	if rows := e.lcMessages(c.user.ID, ""); len(rows) != 0 || len(e.lcNotifications(c.user.ID, service.NotifyLifecycleVerifyReminder)) != 0 {
		t.Fatalf("a failed send left a claim or a notification: %+v", rows)
	}
	fail = false
	e.lcRun()
	if rows := e.lcMessages(c.user.ID, domain.JourneyVerifyReminder); len(rows) != 1 || calls != 2 {
		t.Fatalf("retry: %+v after %d calls", rows, calls)
	}
}

// One message per user per 48 hours: a user due for two journeys gets the
// first now and the second once the cap has passed (never both at once).
func TestLifecycleFrequencyCap(t *testing.T) {
	e := newEnv(t)
	e.lcStart()
	e.app.Services.Lifecycle.SetVerificationReminder(func(context.Context, uuid.UUID) error { return nil })
	c := e.registerClient()
	e.lcAge(c.user.ID, 25*time.Hour)
	e.app.DB.Exec(`UPDATE users SET email_verified_at = NULL WHERE id = ?`, c.user.ID)
	e.lcRun()
	rows := e.lcMessages(c.user.ID, "")
	if len(rows) != 1 || rows[0].Journey != domain.JourneyVerifyReminder {
		t.Fatalf("first run: %+v", rows)
	}
	e.clock.Advance(47 * time.Hour)
	e.lcAge(c.user.ID, 25*time.Hour+47*time.Hour) // the client is 72 hours old at this point
	e.lcRun()
	if rows := e.lcMessages(c.user.ID, ""); len(rows) != 1 {
		t.Fatalf("a second message inside the cap: %+v", rows)
	}
	e.clock.Advance(2 * time.Hour)
	e.lcAge(c.user.ID, 74*time.Hour)
	e.lcRun()
	rows = e.lcMessages(c.user.ID, "")
	if len(rows) != 2 || rows[1].Journey != domain.JourneyFirstReading {
		t.Fatalf("after the cap: %+v", rows)
	}
}

func TestLifecycleAfterFirstReadingNeedsTheReferralProgram(t *testing.T) {
	e := newEnv(t)
	e.lcStart()
	skipped, sent, young := e.registerClient(), e.registerClient(), e.registerClient()
	now := e.clock.Now()
	e.lcReader(skipped.user.ID, now.Add(-25*time.Hour), now.Add(-25*time.Hour), 1)
	e.lcReader(sent.user.ID, now.Add(-30*time.Hour), now.Add(-30*time.Hour), 1)
	e.lcReader(young.user.ID, now.Add(-2*time.Hour), now.Add(-2*time.Hour), 1)

	// The program is off (no seam): the due messages are recorded as skipped.
	e.lcRun()
	for _, c := range []*client{skipped, sent} {
		rows := e.lcMessages(c.user.ID, domain.JourneyAfterFirstReading)
		if len(rows) != 1 || rows[0].Status != domain.LifecycleSkipped || rows[0].SkipReason != domain.LifecycleSkipReferralsOff {
			t.Fatalf("referrals off: %+v", rows)
		}
		if n := e.lcNotifications(c.user.ID, service.NotifyLifecycleInviteFriends); len(n) != 0 {
			t.Fatalf("notified while the program is off: %+v", n)
		}
	}
	// A reading 24 hours ago, with the program on.
	e.app.Services.Lifecycle.SetReferralsEnabled(func(context.Context) bool { return true })
	e.app.DB.Exec(`DELETE FROM lifecycle_messages WHERE user_id = ?`, sent.user.ID)
	e.lcRun()
	n := e.lcNotifications(sent.user.ID, service.NotifyLifecycleInviteFriends)
	if len(n) != 1 || n[0].Link != "/profile/invite" {
		t.Fatalf("invite notification: %+v", n)
	}
	if rows := e.lcMessages(young.user.ID, ""); len(rows) != 0 {
		t.Fatalf("a reading two hours ago: %+v", rows)
	}
}

func TestLifecycleWinBackCycles(t *testing.T) {
	e := newEnv(t)
	e.lcStart()
	admin := e.createAdmin()
	admin.stepUp()
	var cur service.LifecycleView
	admin.must(http.StatusOK, http.MethodGet, lifecyclePath, nil, &cur)
	on := true
	admin.must(http.StatusOK, http.MethodPut, lifecyclePath, map[string]any{"version": cur.Version,
		"journeys": map[string]any{domain.JourneyWinBack: map[string]any{"enabled": on}}}, nil)

	lapsed, active, tokenActive, neverRead := e.registerClient(), e.registerClient(), e.registerClient(), e.registerClient()
	e.lcLapse(lapsed.user.ID, 15*24*time.Hour)
	e.lcLapse(active.user.ID, 15*24*time.Hour)
	e.lcLapse(tokenActive.user.ID, 15*24*time.Hour)
	e.lcLapse(neverRead.user.ID, 15*24*time.Hour)
	e.app.DB.Exec(`DELETE FROM lifecycle_readers WHERE user_id = ?`, neverRead.user.ID)
	e.app.DB.Exec(`UPDATE users SET last_login_at = ? WHERE id = ?`, e.clock.Now().Add(-2*24*time.Hour), active.user.ID)
	e.app.DB.Exec(`UPDATE refresh_tokens SET last_used_at = ? WHERE user_id = ?`, e.clock.Now().Add(-time.Hour), tokenActive.user.ID)
	e.app.DB.Exec(`UPDATE psychics SET availability = 'offline'`)

	e.lcRun()
	rows := e.lcMessages(lapsed.user.ID, domain.JourneyWinBack)
	if len(rows) != 1 || rows[0].Cycle != 1 {
		t.Fatalf("lapsed reader: %+v", rows)
	}
	for name, c := range map[string]*client{"a recent sign-in": active, "recent session activity": tokenActive, "no reading": neverRead} {
		if rows := e.lcMessages(c.user.ID, domain.JourneyWinBack); len(rows) != 0 {
			t.Errorf("%s was messaged: %+v", name, rows)
		}
	}
	// Nobody is online: the message says the horoscope is ready, not that
	// advisors are online.
	n := e.lcNotifications(lapsed.user.ID, service.NotifyLifecycleWinBack)
	if len(n) != 1 || n[0].Data["online_advisors"] != float64(0) || n[0].Link != "/profile" {
		t.Fatalf("win-back notification: %+v", n)
	}
	// At most one per 30-day cycle, three per lapse: the next ones come a cycle
	// apart, each while the last reading is in its cycle's 72-hour band.
	e.clock.Advance(10 * 24 * time.Hour)
	e.lcRun()
	if rows := e.lcMessages(lapsed.user.ID, domain.JourneyWinBack); len(rows) != 1 {
		t.Fatalf("a second win-back inside the cycle: %+v", rows)
	}
	// 45 days since the reading (the second band), then 75 (the third).
	for _, step := range []struct {
		cycle int
		wait  time.Duration
	}{{2, 20 * 24 * time.Hour}, {3, 30 * 24 * time.Hour}} {
		e.clock.Advance(step.wait)
		e.lcRun()
		rows := e.lcMessages(lapsed.user.ID, domain.JourneyWinBack)
		if len(rows) != step.cycle || rows[step.cycle-1].Cycle != step.cycle {
			t.Fatalf("cycle %d: %+v", step.cycle, rows)
		}
	}
	// Three per lapse: a fourth cycle later, the reader is left alone.
	e.clock.Advance(30 * 24 * time.Hour)
	e.lcRun()
	if rows := e.lcMessages(lapsed.user.ID, domain.JourneyWinBack); len(rows) != 3 {
		t.Fatalf("a fourth win-back for one lapse: %d rows", len(rows))
	}
	// A new reading and a new lapse start the count again (cycle 4).
	e.app.DB.Exec(`UPDATE lifecycle_readers SET last_reading_at = ?, readings = readings + 1 WHERE user_id = ?`, e.clock.Now().Add(-15*24*time.Hour), lapsed.user.ID)
	e.lcRun()
	if rows := e.lcMessages(lapsed.user.ID, domain.JourneyWinBack); len(rows) != 4 || rows[3].Cycle != 4 {
		t.Fatalf("after a new lapse: %+v", rows)
	}
	// Outside every band (lapsed 20 days, the job was down when the first band
	// passed): nothing is sent; the next cycle's band picks the reader up.
	late := e.registerClient()
	e.lcLapse(late.user.ID, 20*24*time.Hour)
	e.lcRun()
	if rows := e.lcMessages(late.user.ID, domain.JourneyWinBack); len(rows) != 0 {
		t.Fatalf("a reader between two bands: %+v", rows)
	}
	e.clock.Advance(25 * 24 * time.Hour) // 45 days since the reading: the second band
	e.lcRun()
	if rows := e.lcMessages(late.user.ID, domain.JourneyWinBack); len(rows) != 1 || rows[0].Cycle != 1 {
		t.Fatalf("the next band: %+v", rows)
	}
}

func TestLifecycleWinBackNamesTheAdvisorsOnline(t *testing.T) {
	e := newEnv(t)
	e.lcStart()
	e.app.DB.Exec(`UPDATE site_settings SET version = version + 1, value = jsonb_set(value, '{journeys,win_back,enabled}', 'true') WHERE key = 'lifecycle'`)
	e.createPsychic(40)
	e.app.DB.Exec(`UPDATE psychics SET availability = 'online' WHERE listed`)
	c := e.registerClient()
	e.lcLapse(c.user.ID, 15*24*time.Hour)
	e.lcRun()
	n := e.lcNotifications(c.user.ID, service.NotifyLifecycleWinBack)
	if len(n) != 1 || n[0].Data["online_advisors"].(float64) < 1 || n[0].Link != "/psychics" {
		t.Fatalf("win-back notification: %+v", n)
	}
}

// A reading completed within the attribution window after a message converts
// it (one UPDATE, once); older messages, later ones and the verification
// reminder (converted by verifying the address, not by a reading) do not.
func TestLifecycleConversionOnSettledReading(t *testing.T) {
	e := newEnv(t)
	e.lcStart()
	e.ensurePacks()
	psychic := e.createPsychic(30)
	c := e.registerClient()
	e.lcAge(c.user.ID, 25*time.Hour)
	e.lcRun()
	if rows := e.lcMessages(c.user.ID, domain.JourneyFirstReading); len(rows) != 1 || rows[0].ConvertedAt != nil {
		t.Fatalf("before the reading: %+v", rows)
	}
	// Other messages of the same user: too old, after the reading, a verification reminder.
	old, future, verify := uuid.New(), uuid.New(), uuid.New()
	e.app.DB.Exec(`INSERT INTO lifecycle_messages (id, user_id, journey, cycle, status, channels, created_at) VALUES
		(?, ?, 'win_back', 1, 'sent', '{}', ?), (?, ?, 'win_back', 2, 'sent', '{}', ?), (?, ?, 'verify_reminder', 1, 'sent', '{}', ?)`,
		old, c.user.ID, e.clock.Now().Add(-10*24*time.Hour), future, c.user.ID, e.clock.Now().Add(24*time.Hour), verify, c.user.ID, e.clock.Now().Add(-time.Hour))

	c.buyCredits(250)
	_, _, _, _, s := startedSessionOn(t, e, psychic, c)
	e.tick(time.Minute + time.Second)
	endSession(t, c, s.ID)
	deadline := time.Now().Add(5 * time.Second)
	for {
		var n int64
		e.app.DB.Model(&domain.LifecycleMessage{}).Where("user_id = ? AND converted_at IS NOT NULL", c.user.ID).Count(&n)
		if n > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	converted := map[uuid.UUID]bool{}
	for _, m := range e.lcMessages(c.user.ID, "") {
		converted[m.ID] = m.ConvertedAt != nil
	}
	if converted[old] || converted[future] || converted[verify] {
		t.Fatalf("converted out of the window: %v", converted)
	}
	first := e.lcMessages(c.user.ID, domain.JourneyFirstReading)[0]
	if first.ConvertedAt == nil {
		t.Fatal("the first-reading nudge did not convert")
	}
	// The reader row exists and counts the reading.
	var readings int
	e.app.DB.Raw(`SELECT readings FROM lifecycle_readers WHERE user_id = ?`, c.user.ID).Scan(&readings)
	if readings != 1 {
		t.Fatalf("reader row: %d readings", readings)
	}
	// The statistics show the conversion.
	admin := e.createAdmin()
	var view service.LifecycleView
	admin.must(http.StatusOK, http.MethodGet, lifecyclePath, nil, &view)
	for _, j := range view.Journeys {
		if j.Key == domain.JourneyFirstReading {
			if st := j.Stats["7"]; st.Sent != 1 || st.Converted != 1 || st.ConversionRate != 1 {
				t.Fatalf("first_reading stats: %+v", j.Stats)
			}
		}
	}
	// And a reader no longer gets the "first reading" nudge.
	e.app.DB.Exec(`DELETE FROM lifecycle_messages WHERE user_id = ?`, c.user.ID)
	e.lcRun()
	if rows := e.lcMessages(c.user.ID, domain.JourneyFirstReading); len(rows) != 0 {
		t.Fatalf("a reader was nudged to take a first reading: %+v", rows)
	}
}

// Two API instances running the job at the same time still send once: the
// advisory lock serialises the runs and the claim insert is the guard if the
// lock were ever lost.
func TestLifecycleExactlyOnceAcrossInstances(t *testing.T) {
	e1, e2 := newEnv(t), newEnv(t)
	e1.lcStart()
	const n = 60
	ids := make([]uuid.UUID, 0, n)
	for i := 0; i < n; i++ {
		c := e1.registerClient()
		e1.lcAge(c.user.ID, 25*time.Hour)
		ids = append(ids, c.user.ID)
	}
	var wg sync.WaitGroup
	for _, e := range []*env{e1, e2} {
		wg.Add(1)
		go func(e *env) {
			defer wg.Done()
			if err := e.app.Services.Lifecycle.RunDue(context.Background()); err != nil {
				t.Errorf("run: %v", err)
			}
		}(e)
	}
	wg.Wait()
	deadline := time.Now().Add(5 * time.Second)
	for e1.app.Dispatcher.Processed()+e2.app.Dispatcher.Processed() < n && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	for _, id := range ids {
		if rows := e1.lcMessages(id, ""); len(rows) != 1 {
			t.Fatalf("user %s has %d messages", id, len(rows))
		}
	}
	var notes int64
	e1.app.DB.Model(&domain.Notification{}).Where("kind = ? AND user_id IN ?", service.NotifyLifecycleFirstReading, ids).Count(&notes)
	if notes != n {
		t.Fatalf("%d notifications for %d users", notes, n)
	}
	// The claim itself is idempotent: a second claim of the same (user, journey,
	// cycle) inserts nothing, so only the first claimer would send.
	repo := repository.NewLifecycleRepository(e1.app.DB)
	row := domain.LifecycleMessage{ID: domain.NewID(), UserID: ids[0], Journey: domain.JourneyFirstReading, Cycle: 1, Status: domain.LifecycleSent,
		Channels: domain.JSONMap{}, CreatedAt: e1.clock.Now()}
	got, err := repo.Claim(context.Background(), []domain.LifecycleMessage{row})
	if err != nil || len(got) != 0 {
		t.Fatalf("a duplicate claim won: %v %v", got, err)
	}
	// While another instance holds the lock the run does nothing.
	held := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_, _ = repo.WithRunLock(context.Background(), func(context.Context) error {
			close(held)
			<-release
			return nil
		})
	}()
	<-held
	acquired, err := repo.WithRunLock(context.Background(), func(context.Context) error { return nil })
	close(release)
	if err != nil || acquired {
		t.Fatalf("second lock holder: acquired=%v err=%v", acquired, err)
	}
}

// At size: 1,500 due clients (three keyset batches of 500, with sign-up times
// shared by several accounts so the cursor's tie-break on the id matters) and
// two instances running the job at once: every client gets exactly one
// message and one notification.
func TestLifecycleAtSizeKeysetBatchesAcrossInstances(t *testing.T) {
	e1, e2 := newEnv(t), newEnv(t)
	e1.lcStart()
	const n = 1500
	tag := strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	at := e1.clock.Now().Add(-30 * time.Hour)
	var ids []uuid.UUID
	if err := e1.app.DB.Raw(`INSERT INTO users (id, email, password_hash, role, status, terms_accepted_at, adult_confirmed_at, created_at, updated_at)
		SELECT gen_random_uuid(), 'lc-size-' || ? || '-' || i || '@example.test', 'x', 'client', 'active', ?, ?,
			?::timestamptz + (i / 3) * interval '1 second', ?::timestamptz
		FROM generate_series(1, ?) AS i RETURNING id`, tag, at, at, at, at, n).Scan(&ids).Error; err != nil || len(ids) != n {
		t.Fatalf("insert %d clients: %v", len(ids), err)
	}
	var wg sync.WaitGroup
	for _, e := range []*env{e1, e2} {
		wg.Add(1)
		go func(e *env) {
			defer wg.Done()
			if err := e.app.Services.Lifecycle.RunDue(context.Background()); err != nil {
				t.Errorf("run: %v", err)
			}
		}(e)
	}
	wg.Wait()
	var notes int64
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		e1.app.DB.Model(&domain.Notification{}).Where("kind = ? AND user_id IN ?", service.NotifyLifecycleFirstReading, ids).Count(&notes)
		if notes >= n {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	var rows, users int64
	e1.app.DB.Model(&domain.LifecycleMessage{}).Where("user_id IN ?", ids).Count(&rows)
	e1.app.DB.Model(&domain.LifecycleMessage{}).Where("user_id IN ? AND journey = ? AND status = 'sent'", ids, domain.JourneyFirstReading).
		Distinct("user_id").Count(&users)
	if rows != n || users != n || notes != n {
		t.Fatalf("%d clients: %d messages for %d users, %d notifications", n, rows, users, notes)
	}
	// Once more: nothing is due any more.
	e1.lcRun()
	e1.app.DB.Model(&domain.LifecycleMessage{}).Where("user_id IN ?", ids).Count(&rows)
	if rows != n {
		t.Fatalf("a second run added messages: %d", rows)
	}
}

func TestLifecycleReadersAreRecordedAndReconciled(t *testing.T) {
	e := newEnv(t)
	e.lcStart()
	e.ensurePacks()
	psychic := e.createPsychic(30)
	c := e.registerClient()
	c.buyCredits(250)
	_, _, _, _, s := startedSessionOn(t, e, psychic, c)
	e.tick(time.Minute + time.Second)
	endSession(t, c, s.ID)
	deadline := time.Now().Add(5 * time.Second)
	var readings int
	for readings == 0 && time.Now().Before(deadline) {
		e.app.DB.Raw(`SELECT COALESCE(max(readings), 0) FROM lifecycle_readers WHERE user_id = ?`, c.user.ID).Scan(&readings)
		time.Sleep(20 * time.Millisecond)
	}
	if readings != 1 {
		t.Fatalf("hook: %d readings", readings)
	}
	// A lost hook is repaired by the job's reconciliation of recent readings.
	e.app.DB.Exec(`DELETE FROM lifecycle_readers WHERE user_id = ?`, c.user.ID)
	e.lcRun()
	e.app.DB.Raw(`SELECT COALESCE(max(readings), 0) FROM lifecycle_readers WHERE user_id = ?`, c.user.ID).Scan(&readings)
	if readings != 1 {
		t.Fatalf("reconciliation: %d readings", readings)
	}
}

func TestLifecycleAdminSettings(t *testing.T) {
	e := newEnv(t)
	e.lcStart()
	admin := e.createAdmin()
	var audit struct {
		Items []struct {
			Action string `json:"action"`
		} `json:"items"`
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/audit-logs?action=lifecycle.settings.updated", nil, &audit)
	auditedBefore := len(audit.Items)

	var cur service.LifecycleView
	admin.must(http.StatusOK, http.MethodGet, lifecyclePath, nil, &cur)
	if len(cur.Journeys) != 4 || cur.AttributionDays != 7 || cur.Version < 1 {
		t.Fatalf("seeded view: %+v", cur)
	}
	byKey := map[string]service.LifecycleJourneyView{}
	for _, j := range cur.Journeys {
		byKey[j.Key] = j
		if len(j.Channels) == 0 || j.Stats["7"].SkippedReasons == nil || j.Stats["30"].SkippedReasons == nil {
			t.Fatalf("journey %s: %+v", j.Key, j)
		}
	}
	if !byKey["first_reading"].Enabled || byKey["win_back"].Enabled || byKey["win_back"].DelayHours != 336 || byKey["first_reading"].DelayHours != 24 ||
		!byKey["first_reading"].Promotional || byKey["verify_reminder"].Promotional || byKey["verify_reminder"].Available {
		t.Fatalf("seeded journeys: %+v", byKey)
	}
	// Writes need a fresh step-up.
	e.clock.Advance(11 * time.Minute)
	delay := 48
	body := map[string]any{"version": cur.Version, "attribution_days": 14,
		"journeys": map[string]any{"first_reading": map[string]any{"delay_hours": delay, "enabled": false}}}
	if r := admin.call(http.MethodPut, lifecyclePath, body); r.status != http.StatusForbidden || r.errCode(t) != "step_up_required" {
		t.Fatalf("put without step-up: %d %s", r.status, r.body)
	}
	admin.stepUp()
	var out service.LifecycleView
	admin.must(http.StatusOK, http.MethodPut, lifecyclePath, body, &out)
	got := map[string]service.LifecycleJourneyView{}
	for _, j := range out.Journeys {
		got[j.Key] = j
	}
	if out.Version != cur.Version+1 || out.AttributionDays != 14 || got["first_reading"].Enabled || got["first_reading"].DelayHours != 48 ||
		!got["after_first_reading"].Enabled || out.UpdatedBy == nil {
		t.Fatalf("updated: %+v", out)
	}
	// A stale version conflicts; invalid bodies are validation errors.
	if r := admin.call(http.MethodPut, lifecyclePath, body); r.status != http.StatusConflict || r.errCode(t) != "version_conflict" {
		t.Fatalf("stale version: %d %s", r.status, r.body)
	}
	for _, bad := range []map[string]any{
		{},
		{"attribution_days": 0},
		{"attribution_days": 31},
		{"journeys": map[string]any{"nope": map[string]any{"enabled": true}}},
		{"journeys": map[string]any{"win_back": map[string]any{"delay_hours": 0}}},
		{"journeys": map[string]any{"win_back": map[string]any{"delay_hours": 24*90 + 1}}},
		{"journeys": map[string]any{"win_back": map[string]any{"enabled": true}}, "version": 0},
	} {
		if r := admin.call(http.MethodPut, lifecyclePath, bad); r.status != http.StatusUnprocessableEntity {
			t.Errorf("%v: %d %s", bad, r.status, r.body)
		}
	}
	// The same value again is not a new version and not audited.
	same := map[string]any{"version": out.Version, "attribution_days": 14}
	var again service.LifecycleView
	admin.must(http.StatusOK, http.MethodPut, lifecyclePath, same, &again)
	if again.Version != out.Version {
		t.Fatalf("unchanged update bumped the version: %d", again.Version)
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/audit-logs?action=lifecycle.settings.updated", nil, &audit)
	if len(audit.Items) != auditedBefore+1 {
		t.Fatalf("audit: %d entries, want %d", len(audit.Items), auditedBefore+1)
	}
	// A disabled journey does not run, a changed delay applies.
	c := e.registerClient()
	e.lcAge(c.user.ID, 25*time.Hour)
	e.lcRun()
	if rows := e.lcMessages(c.user.ID, ""); len(rows) != 0 {
		t.Fatalf("a disabled journey ran: %+v", rows)
	}
	// Clients cannot read or write it.
	me := e.registerClient()
	if r := me.call(http.MethodGet, lifecyclePath, nil); r.status == http.StatusOK {
		t.Fatal("a client read the admin setting")
	}
}

func TestLifecycleDelayFromSettingsApplies(t *testing.T) {
	e := newEnv(t)
	e.lcStart()
	e.app.DB.Exec(`UPDATE site_settings SET version = version + 1, value = jsonb_set(value, '{journeys,first_reading,delay_hours}', '48') WHERE key = 'lifecycle'`)
	c := e.registerClient()
	e.lcAge(c.user.ID, 25*time.Hour)
	e.lcRun()
	if rows := e.lcMessages(c.user.ID, ""); len(rows) != 0 {
		t.Fatalf("messaged before the 48 hour delay: %+v", rows)
	}
	e.lcAge(c.user.ID, 49*time.Hour)
	e.lcRun()
	if rows := e.lcMessages(c.user.ID, ""); len(rows) != 1 {
		t.Fatalf("after the delay: %+v", rows)
	}
}

func TestLifecycleStatsAndFunnel(t *testing.T) {
	e := newEnv(t)
	e.lcStart()
	admin := e.createAdmin()
	now := e.clock.Now()
	// The cohort: five clients that signed up 2-5 days ago, one 40 days ago.
	var cohort []*client
	for i := 0; i < 5; i++ {
		c := e.registerClient()
		e.lcAge(c.user.ID, time.Duration(2+i)*24*time.Hour)
		cohort = append(cohort, c)
	}
	outside := e.registerClient()
	e.lcAge(outside.user.ID, 40*24*time.Hour)
	e.app.DB.Exec(`UPDATE users SET email_verified_at = NULL WHERE id IN ?`, []uuid.UUID{cohort[0].user.ID, cohort[1].user.ID, cohort[2].user.ID, cohort[3].user.ID, cohort[4].user.ID, outside.user.ID})
	e.app.DB.Exec(`UPDATE users SET email_verified_at = now() WHERE id IN ?`, []uuid.UUID{cohort[0].user.ID, cohort[1].user.ID, cohort[2].user.ID})
	for _, c := range cohort[:2] {
		c.must(http.StatusOK, http.MethodPatch, "/api/v1/me/profile", map[string]any{"birth_time": "07:45", "birth_place_id": "gn:2988507"}, nil)
	}
	e.lcReader(cohort[0].user.ID, now.Add(-48*time.Hour), now.Add(-47*time.Hour), 1)
	e.lcReader(cohort[1].user.ID, now.Add(-72*time.Hour), now.Add(-10*time.Hour), 2)
	e.lcReader(outside.user.ID, now.Add(-24*time.Hour), now.Add(-10*time.Hour), 3)

	var f service.LifecycleFunnelView
	admin.must(http.StatusOK, http.MethodGet, funnelPath, nil, &f)
	counts := map[string]*int64{}
	for _, s := range f.Steps {
		counts[s.Key] = s.Count
	}
	// Other tests' clients may exist in the shared database; they were created
	// moments ago, so only compare the differences the cohort makes: the
	// signed-up total is at least the cohort and each step is at most the
	// previous cohort numbers plus those clients. The numbers of this cohort:
	var signed, verified, birth, first, second int64
	e.app.DB.Raw(`SELECT count(*), count(u.email_verified_at), count(*) FILTER (WHERE p.birth_date IS NOT NULL AND p.birth_latitude IS NOT NULL),
		count(r.user_id), count(*) FILTER (WHERE r.readings >= 2)
		FROM users u LEFT JOIN user_profiles p ON p.user_id = u.id LEFT JOIN lifecycle_readers r ON r.user_id = u.id
		WHERE u.role = 'client' AND u.created_at >= ? AND u.created_at < ?`, f.From, f.To).Row().Scan(&signed, &verified, &birth, &first, &second)
	if *counts["signed_up"] != signed || *counts["verified_email"] != verified || *counts["birth_details"] != birth ||
		*counts["first_reading"] != first || *counts["second_reading"] != second {
		t.Fatalf("funnel %v differs from the definition: %d %d %d %d %d", f.Steps, signed, verified, birth, first, second)
	}
	// The cohort's own contribution.
	if first < 2 || second < 1 || birth < 2 || verified < 3 || signed < 5 {
		t.Fatalf("funnel does not include the cohort: %+v", f.Steps)
	}
	if f.Steps[4].Key != "invited_friend" || f.Steps[4].Count != nil {
		t.Fatalf("the invite step without the referral program: %+v", f.Steps[4])
	}
	// The referral program's count fills the invite step.
	e.app.Services.Lifecycle.SetInvitedCohort(func(_ context.Context, from, to time.Time) (int64, error) { return 3, nil })
	admin.must(http.StatusOK, http.MethodGet, funnelPath+"?days=7", nil, &f)
	if f.Days != 7 || f.Steps[4].Count == nil || *f.Steps[4].Count != 3 {
		t.Fatalf("invite step: %+v", f.Steps[4])
	}
	// The 7-day window excludes the 40 day old client; the 90-day window includes it.
	var wide service.LifecycleFunnelView
	admin.must(http.StatusOK, http.MethodGet, funnelPath+"?days=90", nil, &wide)
	if *wide.Steps[0].Count < *f.Steps[0].Count+1 {
		t.Fatalf("90 days: %d, 7 days: %d", *wide.Steps[0].Count, *f.Steps[0].Count)
	}
	for _, q := range []string{"?days=0x", "?days=91", "?days=-1"} {
		if r := admin.call(http.MethodGet, funnelPath+q, nil); r.status != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %s", q, r.status, r.body)
		}
	}

	// Statistics: sent, skipped by reason, converted over 7 and 30 days.
	e.app.DB.Exec(`INSERT INTO lifecycle_messages (id, user_id, journey, cycle, status, skip_reason, channels, converted_at, created_at) VALUES
		(?, ?, 'first_reading', 1, 'sent', '', '{}', ?, ?), (?, ?, 'first_reading', 1, 'sent', '', '{}', NULL, ?),
		(?, ?, 'first_reading', 1, 'sent', '', '{}', ?, ?), (?, ?, 'after_first_reading', 1, 'skipped', 'referrals_disabled', '{}', NULL, ?)`,
		uuid.New(), cohort[0].user.ID, now, now.Add(-time.Hour),
		uuid.New(), cohort[1].user.ID, now.Add(-2*time.Hour),
		uuid.New(), cohort[2].user.ID, now, now.Add(-20*24*time.Hour),
		uuid.New(), cohort[3].user.ID, now.Add(-3*time.Hour))
	var view service.LifecycleView
	admin.must(http.StatusOK, http.MethodGet, lifecyclePath, nil, &view)
	stats := map[string]map[string]service.LifecycleJourneyStats{}
	for _, j := range view.Journeys {
		stats[j.Key] = j.Stats
	}
	if s := stats["first_reading"]["7"]; s.Sent != 2 || s.Converted != 1 || s.ConversionRate != 0.5 {
		t.Fatalf("7 days: %+v", s)
	}
	if s := stats["first_reading"]["30"]; s.Sent != 3 || s.Converted != 2 {
		t.Fatalf("30 days: %+v", s)
	}
	if s := stats["after_first_reading"]["7"]; s.Sent != 0 || s.Skipped != 1 || s.SkippedReasons["referrals_disabled"] != 1 || s.ConversionRate != 0 {
		t.Fatalf("skipped: %+v", s)
	}
	// A verification reminder converts when the address was confirmed in the window after it.
	e.app.DB.Exec(`INSERT INTO lifecycle_messages (id, user_id, journey, cycle, status, channels, created_at) VALUES (?, ?, 'verify_reminder', 1, 'sent', '{}', ?)`,
		uuid.New(), cohort[0].user.ID, now.Add(-5*time.Hour))
	e.app.DB.Exec(`UPDATE users SET email_verified_at = ? WHERE id = ?`, now.Add(-4*time.Hour), cohort[0].user.ID)
	admin.must(http.StatusOK, http.MethodGet, lifecyclePath, nil, &view)
	for _, j := range view.Journeys {
		if j.Key == domain.JourneyVerifyReminder {
			if s := j.Stats["7"]; s.Sent != 1 || s.Converted != 1 {
				t.Fatalf("verify_reminder: %+v", s)
			}
		}
	}
}

func TestOnboardingChecklist(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	e.app.DB.Exec(`UPDATE users SET email_verified_at = NULL WHERE id = ?`, c.user.ID)
	var got service.OnboardingView
	// Without e-mail verification and the referral program, only the steps
	// that can be completed are listed.
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/onboarding", nil, &got)
	if got.Total != 2 || got.Steps[0].Key != "birth_details" || got.Steps[1].Key != "first_reading" {
		t.Fatalf("without the seams: %+v", got)
	}
	e.app.Services.Lifecycle.SetVerificationReminder(func(context.Context, uuid.UUID) error { return nil })
	e.app.Services.Lifecycle.SetReferralsEnabled(func(context.Context) bool { return true })
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/onboarding", nil, &got)
	keys := []string{"verify_email", "birth_details", "first_reading", "invite_friend"}
	if got.Total != 4 || got.Completed != 0 || got.Dismissed || len(got.Steps) != 4 {
		t.Fatalf("fresh client: %+v", got)
	}
	for i, k := range keys {
		if got.Steps[i].Key != k || got.Steps[i].Done {
			t.Fatalf("step %d: %+v", i, got.Steps[i])
		}
	}
	// Steps complete as the account facts change.
	e.app.DB.Exec(`UPDATE users SET email_verified_at = now() WHERE id = ?`, c.user.ID)
	c.must(http.StatusOK, http.MethodPatch, "/api/v1/me/profile", map[string]any{"birth_time": "07:45", "birth_place_id": "gn:2988507"}, nil)
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/onboarding", nil, &got)
	if got.Completed != 2 || !got.Steps[0].Done || !got.Steps[1].Done || got.Steps[2].Done {
		t.Fatalf("after verifying and adding birth details: %+v", got)
	}
	// The referral program's count is the last step; without it the step stays open.
	e.app.Services.Lifecycle.SetInvitedFriends(func(_ context.Context, id uuid.UUID) (int, error) {
		if id == c.user.ID {
			return 1, nil
		}
		return 0, nil
	})
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/onboarding", nil, &got)
	if got.Completed != 3 || !got.Steps[3].Done {
		t.Fatalf("with an invited friend: %+v", got)
	}
	e.app.Services.Lifecycle.SetInvitedFriends(nil)
	// Dismiss and undo; another tab of the client follows over the WebSocket.
	tab := c.connect("")
	c.must(http.StatusOK, http.MethodPost, "/api/v1/me/onboarding/dismiss", nil, &got)
	if !got.Dismissed {
		t.Fatalf("dismiss: %+v", got)
	}
	var pushed service.OnboardingView
	if ev := tab.waitFor("onboarding.updated", 5*time.Second); json.Unmarshal(ev.Data, &pushed) != nil || !pushed.Dismissed || pushed.Total != 4 {
		t.Fatalf("onboarding.updated: %s", ev.Data)
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/onboarding", nil, &got)
	if !got.Dismissed {
		t.Fatalf("dismissal not kept: %+v", got)
	}
	c.must(http.StatusOK, http.MethodPost, "/api/v1/me/onboarding/dismiss", nil, &got) // idempotent
	c.must(http.StatusOK, http.MethodDelete, "/api/v1/me/onboarding/dismiss", nil, &got)
	if got.Dismissed {
		t.Fatalf("undo: %+v", got)
	}
	if ev := tab.waitMatch("onboarding.updated", `"dismissed":false`, 5*time.Second); json.Unmarshal(ev.Data, &pushed) != nil || pushed.Dismissed {
		t.Fatalf("onboarding.updated after the undo: %s", ev.Data)
	}
	// The profile row is not clobbered by profile edits.
	c.must(http.StatusOK, http.MethodPost, "/api/v1/me/onboarding/dismiss", nil, &got)
	c.must(http.StatusOK, http.MethodPatch, "/api/v1/me/profile", map[string]any{"birth_time": "08:00"}, nil)
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/onboarding", nil, &got)
	if !got.Dismissed {
		t.Fatal("a profile edit cleared the dismissal")
	}
	// A completed reading completes the third step at once.
	e.app.DB.Exec(`INSERT INTO lifecycle_readers (user_id, first_reading_at, last_reading_at, readings) VALUES (?, now(), now(), 1)`, c.user.ID)
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/onboarding", nil, &got)
	if !got.Steps[2].Done {
		t.Fatalf("after a reading: %+v", got)
	}
	// Clients only; signed-in only.
	advisor := e.createPsychic(40)
	if r := advisor.call(http.MethodGet, "/api/v1/me/onboarding", nil); r.status != http.StatusForbidden {
		t.Fatalf("advisor: %d %s", r.status, r.body)
	}
	if r := e.client().call(http.MethodGet, "/api/v1/me/onboarding", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d %s", r.status, r.body)
	}
	if r := advisor.call(http.MethodPost, "/api/v1/me/onboarding/dismiss", nil); r.status != http.StatusForbidden {
		t.Fatalf("advisor dismiss: %d %s", r.status, r.body)
	}
}

// The checklist is the client's own data: an account without an adult birth
// date can still read and dismiss it (the age gate is for client actions).
func TestOnboardingIsAvailableBeforeTheAgeCheck(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	e.app.DB.Exec(`UPDATE users SET adult_confirmed_at = NULL WHERE id = ?`, c.user.ID)
	var got service.OnboardingView
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/onboarding", nil, &got)
	c.must(http.StatusOK, http.MethodPost, "/api/v1/me/onboarding/dismiss", nil, &got)
}
