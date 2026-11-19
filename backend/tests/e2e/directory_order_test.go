package e2e

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"
)

// TestDirectoryOrder (revisions 6.0, §26.3): the directory and the home
// selection share one order: online, away, busy, offline; inside a tier the
// review-weighted rating, then review count, then the admin's featured flag,
// featured rank, last time online, sort order and name.
func TestDirectoryOrder(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	admin := e.createAdmin()
	tag := uniq("ord")
	named := func(label string, body map[string]any) *client {
		t.Helper()
		p := e.createPsychic(30)
		body["name"] = "Order " + tag + " " + label
		admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/psychics/"+p.user.ID.String(), body, nil)
		return p
	}
	// Same rating and reviews inside the groups below, so the admin's fields
	// and the last time online decide. (The platform mean depends on every
	// advisor in the shared database: the assertions only compare advisors
	// whose weighted rating order does not depend on it.)
	top := named("top", map[string]any{"rating": 4.9, "reviews_count": 400})
	rank2 := named("rank2", map[string]any{"rating": 4.5, "reviews_count": 100, "featured": true, "featured_rank": 2})
	rank1 := named("rank1", map[string]any{"rating": 4.5, "reviews_count": 100, "featured": true, "featured_rank": 1})
	plain := named("plain", map[string]any{"rating": 4.5, "reviews_count": 100, "sort_order": 5})
	away := named("away", map[string]any{"rating": 3.0, "reviews_count": 5})
	busy := named("busy", map[string]any{"rating": 3.0, "reviews_count": 5})
	star := named("star", map[string]any{"rating": 5.0, "reviews_count": 90000}) // best rated, but offline
	// Offline, equal stats: last online, then sort order, then name decide.
	oldest := named("zz-old", map[string]any{"rating": 4.0, "reviews_count": 10, "sort_order": 9})
	newest := named("zz-new", map[string]any{"rating": 4.0, "reviews_count": 10, "sort_order": 9})
	neverB := named("b-never", map[string]any{"rating": 4.0, "reviews_count": 10, "sort_order": 1})
	neverA := named("a-never", map[string]any{"rating": 4.0, "reviews_count": 10, "sort_order": 1})
	neverLate := named("c-never", map[string]any{"rating": 4.0, "reviews_count": 10, "sort_order": 2})

	// Offline advisors that were online once: the one online later goes first.
	w := oldest.goOnline()
	e.clock.Advance(2 * time.Minute)
	oldest.must(http.StatusOK, http.MethodPatch, "/api/v1/psychic/availability", map[string]any{"availability": "offline"}, nil)
	_ = w.conn.Close()
	e.clock.Advance(2 * time.Minute)
	w = newest.goOnline()
	e.clock.Advance(2 * time.Minute)
	newest.must(http.StatusOK, http.MethodPatch, "/api/v1/psychic/availability", map[string]any{"availability": "offline"}, nil)
	_ = w.conn.Close()

	for _, p := range []*client{top, rank2, rank1, plain} {
		p.goOnline()
	}
	away.goOnline()
	away.must(http.StatusOK, http.MethodPatch, "/api/v1/psychic/availability", map[string]any{"availability": "away"}, nil)
	startedSessionWith(t, e, busy, 250)

	visitor := e.client()
	browse := func(query string) directoryPage {
		t.Helper()
		var out directoryPage
		visitor.must(http.StatusOK, http.MethodGet, "/api/v1/psychics?page_size=48&q="+url.QueryEscape("Order "+tag)+query, nil, &out)
		return out
	}
	slugsOf := func(ps ...*client) []string {
		out := make([]string, len(ps))
		for i, p := range ps {
			out[i] = p.slug()
		}
		return out
	}
	want := slugsOf(top, rank1, rank2, plain, away, busy, star, newest, oldest, neverA, neverB, neverLate)
	if got := browse("").slugs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("directory order\n got  %v\n want %v", got, want)
	}

	// Filters keep the order.
	if got := browse("&status=online").slugs(); !reflect.DeepEqual(got, want[:4]) {
		t.Errorf("status=online: %v, want %v", got, want[:4])
	}
	if got := browse("&status=offline").slugs(); !reflect.DeepEqual(got, want[6:]) {
		t.Errorf("status=offline: %v, want %v", got, want[6:])
	}
	// Pages cut the ordered list.
	var p2 directoryPage
	visitor.must(http.StatusOK, http.MethodGet, "/api/v1/psychics?page_size=5&page=2&q="+url.QueryEscape("Order "+tag), nil, &p2)
	if got := p2.slugs(); !reflect.DeepEqual(got, want[5:10]) {
		t.Errorf("page 2: %v, want %v", got, want[5:10])
	}

	// The home selection is the head of the same order (the whole database,
	// so compare with the unfiltered directory rather than with this test's
	// advisors).
	var head directoryPage
	visitor.must(http.StatusOK, http.MethodGet, "/api/v1/psychics?page_size=6", nil, &head)
	var featured struct {
		Items []struct {
			Slug string `json:"slug"`
		} `json:"items"`
	}
	visitor.must(http.StatusOK, http.MethodGet, "/api/v1/psychics/featured?limit=6", nil, &featured)
	got := make([]string, len(featured.Items))
	for i, it := range featured.Items {
		got[i] = it.Slug
	}
	if !reflect.DeepEqual(got, head.slugs()) {
		t.Errorf("featured %v differs from the directory head %v", got, head.slugs())
	}
	if !reflect.DeepEqual(got, want[:6]) {
		t.Errorf("featured %v, want %v", got, want[:6])
	}

	// A live status change moves the advisor between tiers at once: the
	// away advisor comes online and now outranks the busier-looking ones by
	// rating, then the best advisor leaves.
	away.must(http.StatusOK, http.MethodPatch, "/api/v1/psychic/availability", map[string]any{"availability": "online"}, nil)
	after := browse("").slugs()
	if idx := indexOf(after, away.slug()); idx < 0 || idx > 4 {
		t.Errorf("an advisor who came online should join the online tier, got position %d in %v", idx, after)
	}
	top.must(http.StatusOK, http.MethodPatch, "/api/v1/psychic/availability", map[string]any{"availability": "away"}, nil)
	if idx := indexOf(browse("").slugs(), top.slug()); idx != 4 {
		t.Errorf("an advisor who went away should follow the online tier (position 4), got %d", idx)
	}
}

func indexOf(list []string, want string) int {
	for i, s := range list {
		if s == want {
			return i
		}
	}
	return -1
}
