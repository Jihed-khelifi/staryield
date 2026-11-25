package e2e

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Funnel visit stitching (conversion follow-ups, §27.5): the actor of an
// event is its account when the event (or the browser that sent it, through
// web_visitors) has one, else the anonymous browser id.

type funnelOut struct {
	Items []struct {
		Name   string `json:"name"`
		Actors int64  `json:"actors"`
	} `json:"items"`
}

// funnelActors returns the actor count of each step.
func funnelActors(t *testing.T, admin *client, steps []string, from, to time.Time) []int64 {
	t.Helper()
	var out funnelOut
	q := url.Values{"steps": {strings.Join(steps, ",")}, "from": {from.Format(time.RFC3339)}, "to": {to.Format(time.RFC3339)}}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/analytics/funnel?"+q.Encode(), nil, &out)
	if len(out.Items) != len(steps) {
		t.Fatalf("funnel: %+v", out)
	}
	counts := make([]int64, len(steps))
	for i, it := range out.Items {
		counts[i] = it.Actors
	}
	return counts
}

// waitEvents waits until the analytics writer has stored n events matching the condition.
func (e *env) waitEvents(where string, n int64, args ...any) {
	e.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var got int64
	for time.Now().Before(deadline) {
		e.app.DB.Raw("SELECT count(*) FROM analytics_events WHERE "+where, args...).Row().Scan(&got)
		if got >= n {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	e.t.Fatalf("expected %d analytics events (%s), stored %d", n, where, got)
}

func TestFunnelStitchesVisitorsToAccounts(t *testing.T) {
	e := newEnv(t)
	e.calcStart()
	admin := e.createAdmin()
	steps := []string{"web.page_view", "user.registered"}
	from, to := e.clock.Now().Add(-time.Hour), e.clock.Now().Add(6*time.Hour)
	// Other tests share the database: count only what this one adds.
	base := funnelActors(t, admin, steps, from, to)

	a, b, c := "stitch-browser-a", "stitch-browser-b", "stitch-browser-c"
	// Browsers A, B and C browse anonymously (C on a calculator page, so the
	// server creates its visitor row).
	e.postEvents(e.client(), e.calcEvent("page_view", a, "/en/psychics", "en", nil))
	e.postEvents(e.client(), e.calcEvent("page_view", b, "/en/psychics", "en", nil))
	e.postEvents(e.client(), e.calcEvent("page_view", c, "/en/calculators/astrology", "en", nil))
	e.waitEvents("anonymous_id IN (?, ?, ?)", 3, a, b, c)

	// A signs up: the register body links the browser; the server event
	// user.registered carries the account only.
	e.clock.Advance(time.Minute)
	user := e.registerWith(e.client(), uniq("stitch")+"@example.com", map[string]any{"anonymous_id": a}, "en")
	e.waitEvents("name = 'user.registered' AND user_id = ?", 1, user.ID)

	// The same person signs in from browser C: an authed event links it.
	e.clock.Advance(time.Minute)
	cc := e.login(user.Email, strongPassword)
	e.postEvents(cc, e.calcEvent("page_view", c, "/en/calculators/astrology", "en", nil))
	e.waitEvents("anonymous_id = ? AND user_id IS NOT NULL", 1, c)

	got := funnelActors(t, admin, steps, from, to)
	// Step 1: the account once (browsers A and C) and browser B on its own.
	if d := got[0] - base[0]; d != 2 {
		t.Fatalf("page_view actors grew by %d, want 2 (the account once, the unlinked browser once)", d)
	}
	// Step 2: the pre-sign-up visit and the sign-up are one actor.
	if d := got[1] - base[1]; d != 1 {
		t.Fatalf("registered actors grew by %d, want 1", d)
	}

	// History is stitched too: events stored before the link resolve at query time.
	var stitched int64
	e.app.DB.Raw(`SELECT count(*) FROM analytics_events WHERE anonymous_id = ? AND user_id IS NULL`, a).Row().Scan(&stitched)
	if stitched != 1 {
		t.Fatalf("the pre-sign-up event must stay anonymous (append-only), found %d", stitched)
	}
}
