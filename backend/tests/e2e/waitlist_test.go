package e2e

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

func waitlistPath(slug string) string { return "/api/v1/psychics/" + slug + "/waitlist" }

func (c *client) waitlist(query string) service.Page[service.WaitlistView] {
	c.e.t.Helper()
	var page service.Page[service.WaitlistView]
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/waitlist"+query, nil, &page)
	return page
}

func (e *env) runWaitlist() {
	e.t.Helper()
	if err := e.app.Scheduler.RunJob(context.Background(), service.WaitlistJob); err != nil {
		e.t.Fatalf("waitlist job: %v", err)
	}
}

func TestWaitlistJoinLeaveAndLimits(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	c := e.registerClient()
	advisor := e.createPsychic(30)

	var joined service.WaitlistView
	c.must(http.StatusOK, http.MethodPost, waitlistPath(advisor.slug()), nil, &joined)
	if joined.Slug != advisor.slug() || joined.Status != "offline" || joined.JoinedAt.IsZero() {
		t.Fatalf("joined: %+v", joined)
	}
	var again service.WaitlistView
	c.must(http.StatusOK, http.MethodPost, waitlistPath(advisor.slug()), nil, &again)
	if !again.JoinedAt.Equal(joined.JoinedAt) {
		t.Fatalf("joining twice must keep the first entry: %v vs %v", again.JoinedAt, joined.JoinedAt)
	}
	if page := c.waitlist(""); len(page.Items) != 1 || page.Items[0].Slug != advisor.slug() {
		t.Fatalf("my waitlist: %+v", page.Items)
	}

	// Leaving is idempotent.
	c.must(http.StatusNoContent, http.MethodDelete, waitlistPath(advisor.slug()), nil, nil)
	c.must(http.StatusNoContent, http.MethodDelete, waitlistPath(advisor.slug()), nil, nil)
	if page := c.waitlist(""); len(page.Items) != 0 {
		t.Fatalf("left list still shown: %+v", page.Items)
	}

	// An advisor who is online now cannot be waited for.
	online := e.createPsychic(30)
	online.goOnline()
	if r := c.call(http.MethodPost, waitlistPath(online.slug()), nil); r.status != http.StatusConflict || r.errCode(t) != "advisor_available" {
		t.Fatalf("online advisor: %d %s", r.status, r.body)
	}

	// Unknown, hidden and banned advisors look the same: not found.
	hidden := e.createPsychic(30)
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/psychics/"+hidden.user.ID.String(), map[string]any{"listed": false}, nil)
	for _, slug := range []string{"no-such-advisor", hidden.slug(), strings.Repeat("x", 90)} {
		if r := c.call(http.MethodPost, "/api/v1/psychics/"+slug+"/waitlist", nil); r.status != http.StatusNotFound {
			t.Errorf("join %q: %d %s", slug, r.status, r.body)
		}
	}

	// Only signed-in clients may join; CSRF applies.
	if r := e.client().call(http.MethodPost, waitlistPath(advisor.slug()), nil); r.status != http.StatusUnauthorized {
		t.Errorf("anonymous join: %d", r.status)
	}
	if r := online.call(http.MethodPost, waitlistPath(advisor.slug()), nil); r.status != http.StatusForbidden {
		t.Errorf("advisor join: %d %s", r.status, r.body)
	}
	if r := c.raw(http.MethodPost, waitlistPath(advisor.slug()), nil, nil); r.status != http.StatusForbidden || r.errCode(t) != "csrf_failed" {
		t.Errorf("join without CSRF: %d %s", r.status, r.body)
	}

	// At most 20 open entries, paged newest first.
	var slugs []string
	for i := 0; i < 20; i++ {
		p := e.createPsychic(30)
		c.must(http.StatusOK, http.MethodPost, waitlistPath(p.slug()), nil, nil)
		slugs = append(slugs, p.slug())
		e.clock.Advance(time.Second) // distinct join times keep the order stable
	}
	if r := c.call(http.MethodPost, waitlistPath(advisor.slug()), nil); r.status != http.StatusConflict || r.errCode(t) != "limit_reached" {
		t.Fatalf("21st list: %d %s", r.status, r.body)
	}
	first := c.waitlist("?limit=5")
	if len(first.Items) != 5 || first.NextCursor == "" || first.Items[0].Slug != slugs[19] {
		t.Fatalf("first page: %+v", first)
	}
	var seen []string
	cursor := ""
	for {
		page := c.waitlist("?limit=5&cursor=" + cursor)
		for _, it := range page.Items {
			seen = append(seen, it.Slug)
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if len(seen) != 20 || seen[19] != slugs[0] {
		t.Fatalf("paged %d entries: %v", len(seen), seen)
	}
	if r := c.call(http.MethodGet, "/api/v1/me/waitlist?cursor=not-a-cursor", nil); r.status != http.StatusUnprocessableEntity {
		t.Errorf("bad cursor: %d", r.status)
	}
	// Hiding an advisor frees the slot (hidden lists neither show nor count).
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/users/"+e.psychicID(t, slugs[0]).String(),
		map[string]any{"status": "banned", "status_reason": "Policy"}, nil)
	c.must(http.StatusOK, http.MethodPost, waitlistPath(advisor.slug()), nil, nil)
}

func (e *env) psychicID(t *testing.T, slug string) uuid.UUID {
	t.Helper()
	p, err := e.app.Services.Psychics.Get(context.Background(), slug)
	if err != nil {
		t.Fatal(err)
	}
	return p.ID
}

func TestWaitlistNotifiesMembersFirst(t *testing.T) {
	e := newEnv(t)
	_, version := e.billingPlan(999, func(v *service.PlanVersionInput) { v.WaitlistPriority = true })
	member, regular := e.registerClient(), e.registerClient()
	member.subscribe(version, member.attachCard("sbx_a_visa_4242_12_2035").ID)
	advisor := e.createPsychic(30)
	for _, c := range []*client{member, regular} {
		c.must(http.StatusOK, http.MethodPost, waitlistPath(advisor.slug()), nil, nil)
	}

	// Nothing happens while the advisor is offline.
	e.runWaitlist()
	ws := advisor.goOnline()
	e.runWaitlist()
	eventually(t, 5*time.Second, "member notified", func() bool {
		return countNotifications(t, member, service.NotifyWaitlistAvailable) == 1
	})
	if n := countNotifications(t, regular, service.NotifyWaitlistAvailable); n != 0 {
		t.Fatalf("non-member notified during the priority lead: %d", n)
	}
	if len(member.waitlist("").Items) != 0 || len(regular.waitlist("").Items) != 1 {
		t.Fatal("the member's entry is finished once notified; the other is still open")
	}

	// After the lead, everyone else is told if the advisor is still online.
	e.clock.Advance(2*time.Minute + time.Second)
	regular.refresh()
	member.refresh()
	e.runWaitlist()
	eventually(t, 5*time.Second, "regular client notified", func() bool {
		return countNotifications(t, regular, service.NotifyWaitlistAvailable) == 1
	})
	var page service.Page[service.NotificationView]
	regular.must(http.StatusOK, http.MethodGet, "/api/v1/me/notifications?limit=20", nil, &page)
	for _, n := range page.Items {
		if n.Kind == service.NotifyWaitlistAvailable && (n.Link != "/chatroom?reader="+advisor.slug() || n.Data["psychic_name"] == "") {
			t.Errorf("notification: %+v", n)
		}
	}

	// Each entry is told once.
	e.runWaitlist()
	time.Sleep(300 * time.Millisecond)
	if countNotifications(t, member, service.NotifyWaitlistAvailable) != 1 || countNotifications(t, regular, service.NotifyWaitlistAvailable) != 1 {
		t.Fatal("a finished entry was notified again")
	}
	_ = ws
}

func TestWaitlistWaitsForTheNextTimeIfTheAdvisorLeaves(t *testing.T) {
	e := newEnv(t)
	c := e.registerClient()
	advisor := e.createPsychic(30)
	c.must(http.StatusOK, http.MethodPost, waitlistPath(advisor.slug()), nil, nil)

	advisor.goOnline()
	advisor.must(http.StatusOK, http.MethodPatch, "/api/v1/psychic/availability", map[string]any{"availability": "away"}, nil)
	e.clock.Advance(3 * time.Minute)
	c.refresh()
	advisor.refresh()
	e.runWaitlist()
	time.Sleep(300 * time.Millisecond)
	if n := countNotifications(t, c, service.NotifyWaitlistAvailable); n != 0 {
		t.Fatalf("notified about an advisor who already left: %d", n)
	}
	if page := c.waitlist(""); len(page.Items) != 1 || page.Items[0].Status != "away" {
		t.Fatalf("entry should still be open: %+v", page.Items)
	}

	// Back online: a fresh lead starts, then the client is told.
	advisor.must(http.StatusOK, http.MethodPatch, "/api/v1/psychic/availability", map[string]any{"availability": "online"}, nil)
	e.runWaitlist()
	time.Sleep(200 * time.Millisecond)
	if n := countNotifications(t, c, service.NotifyWaitlistAvailable); n != 0 {
		t.Fatalf("notified before the lead passed: %d", n)
	}
	e.clock.Advance(2*time.Minute + time.Second)
	c.refresh()
	e.runWaitlist()
	eventually(t, 5*time.Second, "client notified", func() bool {
		return countNotifications(t, c, service.NotifyWaitlistAvailable) == 1
	})
}

func TestWaitlistErasureAndExpiry(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	gone, stays := e.registerClient(), e.registerClient()
	advisor, closing := e.createPsychic(30), e.createPsychic(30)
	for _, c := range []*client{gone, stays} {
		c.must(http.StatusOK, http.MethodPost, waitlistPath(advisor.slug()), nil, nil)
		c.must(http.StatusOK, http.MethodPost, waitlistPath(closing.slug()), nil, nil)
	}
	count := func(where string, args ...any) int64 {
		t.Helper()
		var n int64
		if err := e.app.DB.Table("advisor_waitlist").Where(where, args...).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		return n
	}
	admin.stepUp()
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/users/"+gone.user.ID.String(), map[string]any{"reason": "Erasure request"}, nil)
	if n := count("user_id = ?", gone.user.ID); n != 0 {
		t.Fatalf("erased client's entries kept: %d", n)
	}
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/users/"+closing.user.ID.String(), map[string]any{"reason": "Erasure request"}, nil)
	if n := count("psychic_id = ?", closing.user.ID); n != 0 {
		t.Fatalf("entries waiting for an erased advisor kept: %d", n)
	}
	if len(stays.waitlist("").Items) != 1 {
		t.Fatal("other clients' entries must stay")
	}

	// Open entries expire after 7 days; finished ones are purged later.
	e.clock.Advance(8 * 24 * time.Hour)
	e.maintenance()
	stays.relogin()
	if page := stays.waitlist(""); len(page.Items) != 0 {
		t.Fatalf("stale entry still open: %+v", page.Items)
	}
	if n := count("user_id = ? AND deleted_at IS NOT NULL", stays.user.ID); n != 1 {
		t.Fatalf("expired entry should be kept (soft-deleted) until retention: %d", n)
	}
	e.clock.Advance(91 * 24 * time.Hour)
	e.maintenance()
	if n := count("user_id = ?", stays.user.ID); n != 0 {
		t.Fatalf("finished entry outlived retention: %d", n)
	}
}
