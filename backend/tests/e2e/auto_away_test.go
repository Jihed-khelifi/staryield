package e2e

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// Follow-ups: auto-away. An Online advisor whose live-chat requests expire
// unanswered N times in a row (default 3) is set Away, told once, and the
// counter starts again. Declines, accepts and the advisor's own availability
// changes reset the count; only Online advisors are moved; the admin console
// edits the threshold.

const autoAwayPath = "/api/v1/admin/advisors/auto-away"

// aaRequest has a fresh client request the advisor and returns the session.
func (e *env) aaRequest(psychic *client) service.SessionView {
	e.t.Helper()
	s, _ := e.aaRequestBy(psychic)
	return s
}

// aaRequestBy is aaRequest that also returns the requesting client.
func (e *env) aaRequestBy(psychic *client) (service.SessionView, *client) {
	e.t.Helper()
	var s service.SessionView
	c := e.registerClient()
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/chat/sessions",
		map[string]any{"psychic_slug": psychic.slug(), "request_key": uniq("req-key-")}, &s)
	return s, c
}

// aaMiss makes one request expire unanswered.
func (e *env) aaMiss(psychic *client) {
	e.t.Helper()
	e.aaRequest(psychic)
	e.tick(91 * time.Second)
}

// aaSettle waits until the notification dispatcher has flushed and handled
// everything queued so far (it batches for a flush interval first).
func (e *env) aaSettle() {
	e.t.Helper()
	time.Sleep(e.app.Config.Notifications.FlushInterval + 100*time.Millisecond)
	last, stable := int64(-1), 0
	deadline := time.Now().Add(5 * time.Second)
	for stable < 4 {
		n := e.app.Dispatcher.Processed()
		if n == last {
			stable++
		} else {
			last, stable = n, 0
		}
		if time.Now().After(deadline) {
			e.t.Fatal("notifications did not settle")
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func (e *env) aaCounter(psychic *client) int {
	e.t.Helper()
	var n int
	if err := e.app.DB.Raw(`SELECT missed_requests_in_row FROM psychics WHERE user_id = ?`, psychic.user.ID).Scan(&n).Error; err != nil {
		e.t.Fatal(err)
	}
	return n
}

func (e *env) aaSetCounter(psychic *client, n int) {
	e.t.Helper()
	if err := e.app.DB.Exec(`UPDATE psychics SET missed_requests_in_row = ? WHERE user_id = ?`, n, psychic.user.ID).Error; err != nil {
		e.t.Fatal(err)
	}
}

// aaStatus is the advisor's status as clients see it.
func (e *env) aaStatus(psychic *client) string {
	e.t.Helper()
	var v service.PsychicView
	e.client().must(http.StatusOK, http.MethodGet, "/api/v1/psychics/"+psychic.slug(), nil, &v)
	return v.Status
}

// aaNotices counts the advisor's auto-away notifications.
func (e *env) aaNotices(psychic *client) int {
	e.t.Helper()
	e.aaSettle()
	var n int64
	if err := e.app.DB.Table("notifications").Where("user_id = ? AND kind = ?", psychic.user.ID, service.NotifyAutoAway).Count(&n).Error; err != nil {
		e.t.Fatal(err)
	}
	return int(n)
}

func TestAutoAwayAfterThreeMissedRequests(t *testing.T) {
	e := newEnv(t)
	psychic := e.createPsychic(30)
	psychic.goOnline()
	e.skipWelcomeMail()

	e.aaMiss(psychic)
	e.aaMiss(psychic)
	if st := e.aaStatus(psychic); st != "online" || e.aaCounter(psychic) != 2 {
		t.Fatalf("after two misses: %s, counter %d", st, e.aaCounter(psychic))
	}
	if e.aaNotices(psychic) != 0 {
		t.Fatal("notified before the threshold")
	}
	e.aaMiss(psychic)
	if st := e.aaStatus(psychic); st != "away" {
		t.Fatalf("after three misses: %s", st)
	}
	if n := e.aaNotices(psychic); n != 1 {
		t.Fatalf("auto-away notifications: %d", n)
	}
	if c := e.aaCounter(psychic); c != 0 {
		t.Fatalf("counter after auto-away: %d", c)
	}
	// In-app: the kind and the count; link to the chatroom's availability control.
	var page service.Page[service.NotificationView]
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/me/notifications", nil, &page)
	found := false
	for _, n := range page.Items {
		if n.Kind == service.NotifyAutoAway {
			found = true
			if n.Data["missed"] != float64(3) || n.Link != "/chatroom" {
				t.Fatalf("notification: %+v", n)
			}
		}
	}
	if !found {
		t.Fatalf("no auto-away notification in %+v", page.Items)
	}
	// E-mail: exactly one message about it (the missed-request e-mails are separate).
	var away []string
	for _, m := range sentTo(e.flushMail(), psychic.user.Email) {
		if m.Subject == "You were set to Away" {
			away = append(away, m.Text)
		}
	}
	if len(away) != 1 || !strings.Contains(away[0], "after 3 missed requests") {
		t.Fatalf("auto-away e-mails: %q", away)
	}
	// The advisor comes back Online by choice: a single new miss does not send them away again.
	psychic.must(http.StatusOK, http.MethodPatch, "/api/v1/psychic/availability", map[string]any{"availability": "online"}, nil)
	e.aaMiss(psychic)
	if st := e.aaStatus(psychic); st != "online" || e.aaCounter(psychic) != 1 {
		t.Fatalf("after coming back: %s, counter %d", st, e.aaCounter(psychic))
	}
}

func TestAutoAwayCountsOnlyMissesInARow(t *testing.T) {
	e := newEnv(t)
	psychic := e.createPsychic(30)
	pws := psychic.goOnline()

	e.aaMiss(psychic)
	e.aaMiss(psychic)
	// A decline is an answer, not a miss.
	s := e.aaRequest(psychic)
	pws.waitFor(realtime.EventSessionRequested, 5*time.Second)
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/decline", nil, nil)
	if c := e.aaCounter(psychic); c != 0 {
		t.Fatalf("counter after a decline: %d", c)
	}
	e.aaMiss(psychic)
	e.aaMiss(psychic)
	if st := e.aaStatus(psychic); st != "online" || e.aaCounter(psychic) != 2 {
		t.Fatalf("miss, miss, decline, miss, miss: %s, counter %d", st, e.aaCounter(psychic))
	}
	// A client cancelling a pending request says nothing about the advisor.
	s, owner := e.aaRequestBy(psychic)
	owner.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/cancel", nil, nil)
	if c := e.aaCounter(psychic); c != 2 {
		t.Fatalf("counter after a cancel: %d", c)
	}
}

func TestAutoAwayAcceptResetsTheCount(t *testing.T) {
	e := newEnv(t)
	psychic := e.createPsychic(30)
	pws := psychic.goOnline()
	e.aaMiss(psychic)
	e.aaMiss(psychic)
	s := e.aaRequest(psychic)
	pws.waitFor(realtime.EventSessionRequested, 5*time.Second)
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+s.ID.String()+"/accept", nil, nil)
	if c := e.aaCounter(psychic); c != 0 {
		t.Fatalf("counter after an accept: %d", c)
	}
	if st := e.aaStatus(psychic); st == "away" {
		t.Fatal("set away after an accept")
	}
}

func TestAutoAwayResetsWhenAdvisorSetsAvailability(t *testing.T) {
	e := newEnv(t)
	psychic := e.createPsychic(30)
	psychic.goOnline()
	e.aaMiss(psychic)
	e.aaMiss(psychic)
	psychic.must(http.StatusOK, http.MethodPatch, "/api/v1/psychic/availability", map[string]any{"availability": "online"}, nil)
	if c := e.aaCounter(psychic); c != 0 {
		t.Fatalf("counter after the advisor set availability: %d", c)
	}
	e.aaMiss(psychic)
	if st := e.aaStatus(psychic); st != "online" || e.aaNotices(psychic) != 0 {
		t.Fatalf("one miss after the reset: %s", st)
	}
}

func TestAutoAwayOnlyFromOnline(t *testing.T) {
	e := newEnv(t)
	psychic := e.createPsychic(30)
	psychic.goOnline()
	e.aaMiss(psychic)
	e.aaMiss(psychic)
	// A request is pending, then the advisor stops being Online by a path that
	// keeps the counter (an admin or account change), and it expires: the third
	// miss lands on an advisor who is not Online.
	e.aaRequest(psychic)
	if err := e.app.DB.Exec(`UPDATE psychics SET availability = 'away' WHERE user_id = ?`, psychic.user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if c := e.aaCounter(psychic); c != 2 {
		t.Fatalf("counter before the expiry: %d", c)
	}
	e.tick(91 * time.Second)
	if st := e.aaStatus(psychic); st != "away" {
		t.Fatalf("status: %s", st)
	}
	if n := e.aaNotices(psychic); n != 0 {
		t.Fatalf("notified an advisor who was already Away: %d", n)
	}
	if c := e.aaCounter(psychic); c != 0 {
		t.Fatalf("counter must be reset even when nothing changed: %d", c)
	}
	// Offline is never touched either.
	if err := e.app.DB.Exec(`UPDATE psychics SET availability = 'offline', missed_requests_in_row = 2 WHERE user_id = ?`, psychic.user.ID).Error; err != nil {
		t.Fatal(err)
	}
	e.app.Services.AutoAway.OnRequestMissed(context.Background(), psychic.user.ID)
	var av string
	e.app.DB.Raw(`SELECT availability FROM psychics WHERE user_id = ?`, psychic.user.ID).Scan(&av)
	if av != "offline" || e.aaNotices(psychic) != 0 || e.aaCounter(psychic) != 0 {
		t.Fatalf("offline advisor: %s, %d notices, counter %d", av, e.aaNotices(psychic), e.aaCounter(psychic))
	}
}

func TestAutoAwayNotWhileInAReading(t *testing.T) {
	e := newEnv(t)
	client, psychic, _, _, _ := startedSession(t, e, 30, 250)
	_ = client
	e.aaSetCounter(psychic, 2)
	e.app.Services.AutoAway.OnRequestMissed(context.Background(), psychic.user.ID)
	if c := e.aaCounter(psychic); c != 0 {
		t.Fatalf("counter: %d", c)
	}
	var av string
	e.app.DB.Raw(`SELECT availability FROM psychics WHERE user_id = ?`, psychic.user.ID).Scan(&av)
	if av != "online" || e.aaNotices(psychic) != 0 {
		t.Fatalf("advisor in a reading: %s, %d notices", av, e.aaNotices(psychic))
	}
}

func TestAutoAwayOnceUnderConcurrentExpiry(t *testing.T) {
	e := newEnv(t)
	psychic := e.createPsychic(30)
	psychic.goOnline()
	e.aaSetCounter(psychic, 1)
	// Several expiries of the same advisor race (two instances, or one tick):
	// the counter takes every increment and exactly one caller claims the reset.
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.app.Services.AutoAway.OnRequestMissed(context.Background(), psychic.user.ID)
		}()
	}
	wg.Wait()
	if n := e.aaNotices(psychic); n != 1 {
		t.Fatalf("auto-away notifications: %d", n)
	}
	if st := e.aaStatus(psychic); st != "away" {
		t.Fatalf("status: %s", st)
	}
	if got := len(sentTo(filterSubject(e.flushMail(), "You were set to Away"), psychic.user.Email)); got != 1 {
		t.Fatalf("auto-away e-mails: %d", got)
	}
}

func filterSubject(msgs []mail.Message, subject string) []mail.Message {
	var out []mail.Message
	for _, m := range msgs {
		if m.Subject == subject {
			out = append(out, m)
		}
	}
	return out
}

func TestAutoAwayDisabledBySetting(t *testing.T) {
	e := newEnv(t)
	t.Cleanup(func() { // the database is shared by the package's tests
		e.app.DB.Exec(`UPDATE site_settings SET version = version + 1, value = '{"enabled": true, "missed_requests": 3}' WHERE key = 'advisor_auto_away'`)
	})
	if err := e.app.DB.Exec(`UPDATE site_settings SET version = version + 1, value = '{"enabled": false, "missed_requests": 3}' WHERE key = 'advisor_auto_away'`).Error; err != nil {
		t.Fatal(err)
	}
	psychic := e.createPsychic(30)
	psychic.goOnline()
	for i := 0; i < 4; i++ {
		e.aaMiss(psychic)
	}
	if st := e.aaStatus(psychic); st != "online" || e.aaNotices(psychic) != 0 || e.aaCounter(psychic) != 0 {
		t.Fatalf("disabled: %s, %d notices, counter %d", st, e.aaNotices(psychic), e.aaCounter(psychic))
	}
}

func TestAutoAwayAdminSettings(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	var audit struct {
		Items []struct {
			Action string `json:"action"`
		} `json:"items"`
	}
	const auditPath = "/api/v1/admin/audit-logs?action=advisors.auto_away.updated"
	admin.must(http.StatusOK, http.MethodGet, auditPath, nil, &audit)
	auditedBefore := len(audit.Items)

	var cur service.AutoAwayView
	admin.must(http.StatusOK, http.MethodGet, autoAwayPath, nil, &cur)
	if !cur.Enabled || cur.MissedRequests != 3 || cur.Version < 1 {
		t.Fatalf("seeded setting: %+v", cur)
	}
	// Writes need a fresh step-up.
	e.clock.Advance(11 * time.Minute)
	body := map[string]any{"version": cur.Version, "missed_requests": 2}
	if r := admin.call(http.MethodPut, autoAwayPath, body); r.status != http.StatusForbidden || r.errCode(t) != "step_up_required" {
		t.Fatalf("put without step-up: %d %s", r.status, r.body)
	}
	admin.stepUp()
	var out service.AutoAwayView
	admin.must(http.StatusOK, http.MethodPut, autoAwayPath, body, &out)
	if out.MissedRequests != 2 || !out.Enabled || out.Version != cur.Version+1 || out.UpdatedBy == nil {
		t.Fatalf("updated: %+v", out)
	}
	// A stale version conflicts; invalid bodies are validation errors on the field.
	if r := admin.call(http.MethodPut, autoAwayPath, body); r.status != http.StatusConflict || r.errCode(t) != "version_conflict" {
		t.Fatalf("stale version: %d %s", r.status, r.body)
	}
	for _, bad := range []map[string]any{{"missed_requests": 11}, {"missed_requests": 1}, {"missed_requests": 0}, {"missed_requests": -3}} {
		r := admin.call(http.MethodPut, autoAwayPath, bad)
		if r.status != http.StatusUnprocessableEntity || !strings.Contains(string(r.body), "missed_requests") || !strings.Contains(string(r.body), "Choose between 2 and 10.") {
			t.Errorf("%v: %d %s", bad, r.status, r.body)
		}
	}
	if r := admin.call(http.MethodPut, autoAwayPath, map[string]any{}); r.status != http.StatusUnprocessableEntity {
		t.Errorf("empty body: %d %s", r.status, r.body)
	}
	// The same value again is not a new version and not audited.
	var again service.AutoAwayView
	admin.must(http.StatusOK, http.MethodPut, autoAwayPath, map[string]any{"version": out.Version, "missed_requests": 2}, &again)
	if again.Version != out.Version {
		t.Fatalf("unchanged update bumped the version: %d", again.Version)
	}
	admin.must(http.StatusOK, http.MethodGet, auditPath, nil, &audit)
	if len(audit.Items) != auditedBefore+1 {
		t.Fatalf("audit: %d entries, want %d", len(audit.Items), auditedBefore+1)
	}
	// The new threshold applies: two misses are enough.
	psychic := e.createPsychic(30)
	psychic.goOnline()
	e.aaMiss(psychic)
	if st := e.aaStatus(psychic); st != "online" {
		t.Fatalf("after one miss: %s", st)
	}
	e.aaMiss(psychic)
	if st := e.aaStatus(psychic); st != "away" || e.aaNotices(psychic) != 1 {
		t.Fatalf("after two misses: %s", st)
	}
	// Disabling it through the API stops the counting.
	var off service.AutoAwayView
	admin.must(http.StatusOK, http.MethodPut, autoAwayPath, map[string]any{"version": again.Version, "enabled": false}, &off)
	if off.Enabled || off.MissedRequests != 2 {
		t.Fatalf("disabled: %+v", off)
	}
	// Clients and advisors cannot read or write it.
	for _, who := range []*client{e.registerClient(), psychic} {
		if r := who.call(http.MethodGet, autoAwayPath, nil); r.status != http.StatusForbidden && r.status != http.StatusUnauthorized && r.status != http.StatusNotFound {
			t.Fatalf("non-admin read: %d", r.status)
		}
		if r := who.call(http.MethodPut, autoAwayPath, map[string]any{"enabled": true}); r.status == http.StatusOK {
			t.Fatal("non-admin write")
		}
	}
}
