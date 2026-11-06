package e2e

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

type directoryPage struct {
	Items      []map[string]any `json:"items"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	Total      int              `json:"total"`
	TotalPages int              `json:"total_pages"`
}

func (p directoryPage) slugs() []string {
	out := make([]string, len(p.Items))
	for i, it := range p.Items {
		out[i], _ = it["slug"].(string)
	}
	return out
}

// Advisor cards are public: they must never carry account identifiers,
// contact details, long-form profile text or exact presence timestamps.
var cardForbidden = []string{"id", "user_id", "email", "phone", "last_online_at", "about", "philosophy", "services", "availability", "listed"}

func assertPublicCard(t *testing.T, card map[string]any) {
	t.Helper()
	for _, k := range cardForbidden {
		if _, ok := card[k]; ok {
			t.Errorf("public card exposes %q: %v", k, card)
		}
	}
}

func TestDirectoryBrowse(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	tag := uniq("seer")
	var advisors []*client
	for i := 0; i < 7; i++ {
		p := e.createPsychic(30)
		admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/psychics/"+p.user.ID.String(),
			map[string]any{"name": fmt.Sprintf("Seer %s %d", tag, i), "sort_order": i}, nil)
		advisors = append(advisors, p)
	}
	visitor := e.client()
	browse := func(query string) directoryPage {
		t.Helper()
		var out directoryPage
		visitor.must(http.StatusOK, http.MethodGet, "/api/v1/psychics?q="+url.QueryEscape(tag)+query, nil, &out)
		return out
	}

	// Numbered pages in the curated order, with totals.
	p1 := browse("&page_size=3")
	if p1.Total != 7 || p1.TotalPages != 3 || p1.Page != 1 || p1.PageSize != 3 || len(p1.Items) != 3 {
		t.Fatalf("page 1: %+v", p1)
	}
	if got := p1.slugs(); got[0] != advisors[0].slug() || got[2] != advisors[2].slug() {
		t.Fatalf("page 1 order: %v", got)
	}
	for _, card := range p1.Items {
		assertPublicCard(t, card)
		if card["status"] != "offline" {
			t.Errorf("never-connected advisor shown %v", card["status"])
		}
	}
	if p3 := browse("&page_size=3&page=3"); len(p3.Items) != 1 || p3.Items[0]["slug"] != advisors[6].slug() {
		t.Fatalf("last page: %+v", p3.slugs())
	}
	if past := browse("&page_size=3&page=4"); len(past.Items) != 0 || past.Total != 7 {
		t.Fatalf("past the end: %+v", past)
	}
	if def := browse(""); def.PageSize != 24 || len(def.Items) != 7 {
		t.Fatalf("default page size: %+v", def)
	}

	// Going online shows immediately (the snapshot is patched, not rebuilt).
	advisors[4].goOnline()
	if on := browse("&status=online"); on.Total != 1 || on.Items[0]["slug"] != advisors[4].slug() {
		t.Fatalf("online filter: %+v", on.slugs())
	}
	if off := browse("&status=offline"); off.Total != 6 {
		t.Fatalf("offline filter: %d", off.Total)
	}

	// A profile edit is searchable right away.
	advisors[2].must(http.StatusOK, http.MethodPatch, "/api/v1/psychic/profile", map[string]any{"skills": []string{"Crystal Gazing"}}, nil)
	if sk := browse("&skill=" + url.QueryEscape("crystal gazing")); sk.Total != 1 || sk.Items[0]["slug"] != advisors[2].slug() {
		t.Fatalf("skill filter: %+v", sk.slugs())
	}
	if none := browse("&skill=Necromancy"); none.Total != 0 || none.TotalPages != 0 || none.Items == nil {
		t.Fatalf("empty result must be an empty list: %+v", none)
	}

	// Hiding and banning take effect immediately.
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/psychics/"+advisors[0].user.ID.String(), map[string]any{"listed": false}, nil)
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/users/"+advisors[1].user.ID.String(),
		map[string]any{"status": "banned", "status_reason": "Policy"}, nil)
	if after := browse(""); after.Total != 5 {
		t.Fatalf("hidden or banned advisor still listed: %v", after.slugs())
	}

	r := visitor.must(http.StatusOK, http.MethodGet, "/api/v1/psychics", nil, nil)
	if cc := r.header.Get("Cache-Control"); cc != "public, max-age=10" {
		t.Errorf("Cache-Control = %q", cc)
	}

	for _, q := range []string{"page=-1", "page=abc", "page=10001", "page_size=49", "page_size=-2", "status=sleeping",
		"q=" + strings.Repeat("a", 61), "skill=" + strings.Repeat("b", 41)} {
		r := visitor.call(http.MethodGet, "/api/v1/psychics?"+q, nil)
		if r.status != http.StatusUnprocessableEntity || r.errCode(t) != "validation_failed" {
			t.Errorf("%s: %d %s", q, r.status, r.body)
		}
	}
}

func TestFeaturedAdvisors(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	set := func(p *client, body map[string]any) {
		t.Helper()
		admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/psychics/"+p.user.ID.String(), body, nil)
	}
	// top: online and rated well by many; rising: online, fewer reviews;
	// resting: the best rated of all but offline.
	top, rising, resting := e.createPsychic(30), e.createPsychic(45), e.createPsychic(30)
	set(top, map[string]any{"rating": 5.0, "reviews_count": 50000, "skills": []string{"Tarot", "Love", "Career", "Dreams"}})
	set(rising, map[string]any{"rating": 4.6, "reviews_count": 50})
	set(resting, map[string]any{"rating": 5.0, "reviews_count": 90000})
	top.goOnline()
	risingWS := rising.goOnline()

	type featured struct {
		Items []struct {
			Slug          string   `json:"slug"`
			Name          string   `json:"name"`
			Status        string   `json:"status"`
			CTA           string   `json:"cta"`
			Skills        []string `json:"skills"`
			RatePerMinute int64    `json:"rate_per_minute"`
		} `json:"items"`
		GeneratedAt time.Time `json:"generated_at"`
	}
	visitor := e.client()
	var out featured
	r := visitor.must(http.StatusOK, http.MethodGet, "/api/v1/psychics/featured", nil, &out)
	if len(out.Items) != 3 {
		t.Fatalf("default selection size: %d", len(out.Items))
	}
	if out.Items[0].Slug != top.slug() || out.Items[1].Slug != rising.slug() {
		t.Fatalf("online advisors first, best rated first: %+v", out.Items)
	}
	if out.Items[0].CTA != "chat" || out.Items[1].CTA != "chat" || out.Items[2].CTA != "waitlist" || out.Items[2].Status != "offline" {
		t.Fatalf("calls to action: %+v", out.Items)
	}
	if out.Items[2].Slug != resting.slug() {
		t.Fatalf("best rated offline advisor should fill the last card, got %s", out.Items[2].Slug)
	}
	if len(out.Items[0].Skills) != 3 || out.Items[1].RatePerMinute != 45 || out.GeneratedAt.IsZero() {
		t.Fatalf("card fields: %+v", out)
	}
	var rawCards struct {
		Items []map[string]any `json:"items"`
	}
	r.decode(t, &rawCards)
	for _, c := range rawCards.Items {
		assertPublicCard(t, c)
	}

	// Caching: public with revalidation; an unchanged selection is a 304.
	if cc := r.header.Get("Cache-Control"); cc != "public, max-age=10, stale-while-revalidate=30" {
		t.Errorf("Cache-Control = %q", cc)
	}
	etag := r.header.Get("ETag")
	if !strings.HasPrefix(etag, `W/"`) {
		t.Fatalf("ETag = %q", etag)
	}
	if nm := visitor.raw(http.MethodGet, "/api/v1/psychics/featured", nil, map[string]string{"If-None-Match": etag}); nm.status != http.StatusNotModified || len(nm.body) != 0 {
		t.Fatalf("revalidation: %d %s", nm.status, nm.body)
	}

	// The selection follows status changes at once: rising goes away.
	rising.must(http.StatusOK, http.MethodPatch, "/api/v1/psychic/availability", map[string]any{"availability": "away"}, nil)
	changed := visitor.raw(http.MethodGet, "/api/v1/psychics/featured", nil, map[string]string{"If-None-Match": etag})
	if changed.status != http.StatusOK || changed.header.Get("ETag") == etag {
		t.Fatalf("changed selection revalidated as unchanged: %d", changed.status)
	}
	changed.decode(t, &out)
	if out.Items[1].Slug != rising.slug() || out.Items[1].Status != "away" || out.Items[1].CTA != "waitlist" {
		t.Fatalf("away advisor: %+v", out.Items[1])
	}
	_ = risingWS

	var six featured
	visitor.must(http.StatusOK, http.MethodGet, "/api/v1/psychics/featured?limit=6", nil, &six)
	if len(six.Items) != 6 {
		t.Fatalf("limit=6: %d", len(six.Items))
	}
	for _, q := range []string{"limit=0", "limit=7", "limit=-1", "limit=three", "limit=99999999999999999999"} {
		r := visitor.call(http.MethodGet, "/api/v1/psychics/featured?"+q, nil)
		if q == "limit=0" {
			// 0 means "default", like an absent limit.
			if r.status != http.StatusOK {
				t.Errorf("%s: %d", q, r.status)
			}
			continue
		}
		if r.status != http.StatusUnprocessableEntity || r.errCode(t) != "validation_failed" {
			t.Errorf("%s: %d %s", q, r.status, r.body)
		}
	}
}

type intervalRow struct {
	Status    string
	StartedAt time.Time
	EndedAt   *time.Time
}

func presenceHistory(t *testing.T, e *env, psychicID uuid.UUID) []intervalRow {
	t.Helper()
	var rows []intervalRow
	if err := e.app.DB.Raw("SELECT status, started_at, ended_at FROM psychic_presence_intervals WHERE psychic_id = ? ORDER BY started_at, created_at",
		psychicID).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	return rows
}

func lastOnline(t *testing.T, e *env, psychicID uuid.UUID) *time.Time {
	t.Helper()
	var row struct{ LastOnlineAt *time.Time }
	if err := e.app.DB.Raw("SELECT last_online_at FROM psychics WHERE user_id = ?", psychicID).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row.LastOnlineAt
}

func TestPresenceHistory(t *testing.T) {
	e := newEnv(t)
	p := e.createPsychic(30)
	id := p.user.ID
	if lastOnline(t, e, id) != nil {
		t.Fatal("a new advisor has never been online")
	}

	w := p.goOnline()
	start := e.clock.Now()
	eventually(t, 2*time.Second, "online interval", func() bool {
		h := presenceHistory(t, e, id)
		return len(h) == 1 && h[0].Status == "online" && h[0].EndedAt == nil
	})
	e.clock.Advance(30 * time.Minute)
	p.refresh() // the access token outlived its TTL
	p.must(http.StatusOK, http.MethodPatch, "/api/v1/psychic/availability", map[string]any{"availability": "away"}, nil)
	wentAway := e.clock.Now()
	e.clock.Advance(10 * time.Minute)
	_ = w.conn.Close()
	eventually(t, 5*time.Second, "history closed on disconnect", func() bool {
		h := presenceHistory(t, e, id)
		return len(h) == 2 && h[1].EndedAt != nil
	})
	h := presenceHistory(t, e, id)
	if h[0].Status != "online" || h[0].EndedAt.Sub(h[0].StartedAt) != 30*time.Minute || h[0].StartedAt.Sub(start).Abs() > time.Second {
		t.Errorf("online interval: %+v", h[0])
	}
	if h[1].Status != "away" || h[1].EndedAt.Sub(h[1].StartedAt) != 10*time.Minute {
		t.Errorf("away interval: %+v", h[1])
	}
	// Away does not count as online: last online is when the advisor left.
	if at := lastOnline(t, e, id); at == nil || at.Sub(wentAway).Abs() > time.Millisecond {
		t.Errorf("last_online_at = %v, want %v", at, wentAway)
	}

	// Reconciliation closes intervals left open by a crash (no event was
	// published) and leaves live ones alone.
	crashed := e.createPsychic(30)
	live := e.createPsychic(30)
	live.goOnline()
	if err := e.app.DB.Exec(`INSERT INTO psychic_presence_intervals (id, psychic_id, status, started_at, created_at, updated_at)
		VALUES (?, ?, 'online', ?, ?, ?)`, uuid.New(), crashed.user.ID, e.clock.Now().Add(-time.Hour), e.clock.Now(), e.clock.Now()).Error; err != nil {
		t.Fatal(err)
	}
	if err := e.app.Scheduler.RunJob(context.Background(), service.PresenceJob); err != nil {
		t.Fatal(err)
	}
	if ch := presenceHistory(t, e, crashed.user.ID); len(ch) != 1 || ch[0].EndedAt == nil {
		t.Errorf("crashed advisor's interval left open: %+v", ch)
	}
	if lh := presenceHistory(t, e, live.user.ID); len(lh) != 1 || lh[0].EndedAt != nil || lh[0].Status != "online" {
		t.Errorf("live advisor's interval changed: %+v", lh)
	}
}
