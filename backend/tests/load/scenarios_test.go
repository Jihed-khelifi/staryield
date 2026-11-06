//go:build load

package load

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

var signs = []string{"aries", "taurus", "gemini", "cancer", "leo", "virgo", "libra", "scorpio", "sagittarius", "capricorn", "aquarius", "pisces"}

// natalPlaces are GeoNames ids of birthplaces (Paris, London, New York,
// Tokyo, Sydney, Cairo, Mexico City, São Paulo).
var natalPlaces = []string{"gn:2988507", "gn:2643743", "gn:5128581", "gn:1850147", "gn:2147714", "gn:360630", "gn:3530597", "gn:3448439"}

var placeQueries = []string{"par", "lon", "new", "tok", "ber", "mad", "rom", "san", "mex", "syd", "cai", "lis"}

func get(path string) Request {
	return func(ctx context.Context, c *Client, _ int64) (int, error) { return c.Get(ctx, path) }
}

// TestLoadPublicCatalog: anonymous traffic on the public catalogue.
func TestLoadPublicCatalog(t *testing.T) {
	cfg := LoadConfig()
	RequireAPI(t, cfg)
	ctx := context.Background()
	pool := make([]*Client, 32)
	for i := range pool {
		pool[i] = NewClient(cfg.API, cfg.Origin)
	}
	fast := SLO{P95: 100 * time.Millisecond, P99: 250 * time.Millisecond, MaxErrRate: 0.001}
	compute := SLO{P95: 300 * time.Millisecond, P99: 600 * time.Millisecond, MaxErrRate: 0.001}
	scenarios := []Scenario{
		{Name: "GET /psychics", Rate: cfg.Rate(150), Duration: cfg.Duration, Do: get("/api/v1/psychics"), SLO: fast},
		// Deep pages, filters and search hit the same snapshot.
		{Name: "GET /psychics (paged, filtered)", Rate: cfg.Rate(100), Duration: cfg.Duration, SLO: fast,
			Do: func(ctx context.Context, c *Client, i int64) (int, error) {
				q := []string{"?page=%d", "?page=%d&status=offline", "?page=%d&q=a", "?page=%d&skill=Tarot"}[i%4]
				return c.Get(ctx, "/api/v1/psychics"+fmt.Sprintf(q, 1+i%40))
			}},
		{Name: "GET /psychics/featured", Rate: cfg.Rate(200), Duration: cfg.Duration, Do: get("/api/v1/psychics/featured"), SLO: fast},
		{Name: "GET /astrology/horoscope", Rate: cfg.Rate(150), Duration: cfg.Duration, SLO: fast,
			Do: func(ctx context.Context, c *Client, i int64) (int, error) {
				return c.Get(ctx, "/api/v1/astrology/horoscope/"+signs[i%int64(len(signs))])
			}},
		{Name: "GET /astrology/places", Rate: cfg.Rate(100), Duration: cfg.Duration, SLO: fast,
			Do: func(ctx context.Context, c *Client, i int64) (int, error) {
				return c.Get(ctx, "/api/v1/astrology/places?q="+placeQueries[i%int64(len(placeQueries))])
			}},
		{Name: "GET /credit-packs", Rate: cfg.Rate(100), Duration: cfg.Duration, Do: get("/api/v1/credit-packs"), SLO: fast},
		{Name: "GET /aztro", Rate: cfg.Rate(100), Duration: cfg.Duration, SLO: fast,
			Do: func(ctx context.Context, c *Client, i int64) (int, error) {
				day := []string{"yesterday", "today", "tomorrow"}[i%3]
				return c.Get(ctx, "/api/v1/aztro?sign="+signs[i%int64(len(signs))]+"&day="+day)
			}},
		// The natal calculator computes a full chart, spread and numerology
		// per request (no caching: birth data is never stored).
		{Name: "POST /astrology/natal", Rate: cfg.Rate(40), Duration: cfg.Duration, SLO: compute,
			Do: func(ctx context.Context, c *Client, i int64) (int, error) {
				body := map[string]any{"birth_date": fmt.Sprintf("19%02d-%02d-%02d", 50+i%50, 1+i%12, 1+i%28),
					"birth_time": fmt.Sprintf("%02d:%02d", i%24, (i*7)%60), "place_id": natalPlaces[i%int64(len(natalPlaces))],
					"house_system": "placidus", "include": []string{"aspects", "tarot", "numerology"}}
				return c.Do(ctx, http.MethodPost, "/api/v1/astrology/natal", body, nil)
			}},
	}
	results := runAll(ctx, scenarios, pool)
	Report(t, cfg, "public-catalog", results)
}

// TestLoadSignedInClient: dashboard reads by many signed-in clients.
func TestLoadSignedInClient(t *testing.T) {
	cfg := LoadConfig()
	RequireAPI(t, cfg)
	ctx := context.Background()
	pool := signedInClients(t, ctx, cfg, cfg.Users)
	fast := SLO{P95: 100 * time.Millisecond, P99: 250 * time.Millisecond, MaxErrRate: 0.001}
	heavy := SLO{P95: 150 * time.Millisecond, P99: 400 * time.Millisecond, MaxErrRate: 0.001}
	scenarios := []Scenario{
		{Name: "GET /me", Rate: cfg.Rate(100), Duration: cfg.Duration, Do: get("/api/v1/me"), SLO: fast},
		{Name: "GET /wallet", Rate: cfg.Rate(80), Duration: cfg.Duration, Do: get("/api/v1/wallet"), SLO: fast},
		{Name: "GET /wallet/ledger", Rate: cfg.Rate(60), Duration: cfg.Duration, Do: get("/api/v1/wallet/ledger?limit=20"), SLO: heavy},
		{Name: "GET /chat/conversations", Rate: cfg.Rate(60), Duration: cfg.Duration, Do: get("/api/v1/chat/conversations"), SLO: heavy},
		{Name: "GET /me/notifications", Rate: cfg.Rate(60), Duration: cfg.Duration, Do: get("/api/v1/me/notifications?limit=20"), SLO: heavy},
		{Name: "GET /me/horoscope", Rate: cfg.Rate(60), Duration: cfg.Duration, Do: get("/api/v1/me/horoscope"), SLO: fast},
		{Name: "GET /me/settings", Rate: cfg.Rate(60), Duration: cfg.Duration, Do: get("/api/v1/me/settings"), SLO: fast},
		{Name: "GET /me/sessions", Rate: cfg.Rate(40), Duration: cfg.Duration, Do: get("/api/v1/me/sessions?limit=10"), SLO: fast},
		{Name: "GET /me/waitlist", Rate: cfg.Rate(40), Duration: cfg.Duration, Do: get("/api/v1/me/waitlist?limit=20"), SLO: fast},
		{Name: "GET /me/chart", Rate: cfg.Rate(40), Duration: cfg.Duration, Do: get("/api/v1/me/chart"), SLO: heavy},
		{Name: "GET /billing/transactions", Rate: cfg.Rate(30), Duration: cfg.Duration, Do: get("/api/v1/billing/transactions?limit=20"), SLO: heavy},
	}
	results := runAll(ctx, scenarios, pool)
	Report(t, cfg, "signed-in-client", results)
}

// TestLoadAdvisorNotes: an advisor's notes panel (reads) plus note writes.
func TestLoadAdvisorNotes(t *testing.T) {
	cfg := LoadConfig()
	RequireAPI(t, cfg)
	ctx := context.Background()
	psychic := NewClient(cfg.API, cfg.Origin)
	if err := psychic.Login(ctx, cfg.PsychicEmail, cfg.PsychicPassword); err != nil {
		t.Fatalf("advisor login (%s): %v", cfg.PsychicEmail, err)
	}
	conv := metConversation(t, ctx, psychic)
	notes := "/api/v1/chat/conversations/" + conv + "/notes"
	reads := SLO{P95: 100 * time.Millisecond, P99: 250 * time.Millisecond, MaxErrRate: 0.001}
	writes := SLO{P95: 200 * time.Millisecond, P99: 500 * time.Millisecond, MaxErrRate: 0.001}
	scenarios := []Scenario{
		{Name: "GET notes (own)", Rate: cfg.Rate(80), Duration: cfg.Duration, Do: get(notes + "?limit=5"), SLO: reads},
		{Name: "GET notes (shared)", Rate: cfg.Rate(60), Duration: cfg.Duration, Do: get(notes + "/shared?limit=5"), SLO: reads},
		// Writes stay well under the per-advisor bucket (30/h × STARYIELD_RATE_LIMIT_SCALE).
		// Each arrival creates a note and deletes it again, so long or scaled
		// runs never reach the per-client note cap (409 limit_reached).
		{Name: "POST + DELETE note", Rate: cfg.Rate(5), Duration: cfg.Duration, SLO: writes,
			Do: func(ctx context.Context, c *Client, i int64) (int, error) {
				var created struct {
					ID string `json:"id"`
				}
				status, err := c.Do(ctx, http.MethodPost, notes, map[string]any{"body": fmt.Sprintf("Load note %d — %s", i, time.Now().Format(time.RFC3339Nano))}, &created)
				if err != nil || created.ID == "" {
					return status, err
				}
				return c.Do(ctx, http.MethodDelete, "/api/v1/chat/notes/"+created.ID, nil, nil)
			}},
	}
	results := runAll(ctx, scenarios, []*Client{psychic})
	Report(t, cfg, "advisor-notes", results)
}

// runAll runs scenarios concurrently, as real traffic mixes them.
func runAll(ctx context.Context, scenarios []Scenario, pool []*Client) []Result {
	results := make([]Result, len(scenarios))
	var wg sync.WaitGroup
	for i := range scenarios {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = Run(ctx, scenarios[i], pool)
		}(i)
	}
	wg.Wait()
	return results
}

// signedInClients registers n fresh clients (falls back to the seed client).
func signedInClients(t *testing.T, ctx context.Context, cfg Config, n int) []*Client {
	t.Helper()
	var pool []*Client
	run := time.Now().UnixNano()
	for i := 0; i < n; i++ {
		c := NewClient(cfg.API, cfg.Origin)
		email := fmt.Sprintf("load-%d-%d@example.test", run, i)
		_, err := c.Do(ctx, http.MethodPost, "/api/v1/auth/register", map[string]any{
			"email": email, "password": "L0ad!Test#Passw0rd", "accept_terms": true, "birth_date": "1990-05-17", "locale": "en",
		}, nil)
		if err != nil {
			t.Logf("register virtual user %d: %v (continuing with %d users)", i, err, len(pool))
			break
		}
		pool = append(pool, c)
	}
	if len(pool) == 0 {
		c := NewClient(cfg.API, cfg.Origin)
		if err := c.Login(ctx, cfg.ClientEmail, cfg.ClientPassword); err != nil {
			t.Fatalf("no virtual users: register failed and seed login failed: %v", err)
		}
		pool = append(pool, c)
	}
	return pool
}

// metConversation finds a conversation where the advisor may write notes
// (a started session exists). The large seed provides many.
func metConversation(t *testing.T, ctx context.Context, psychic *Client) string {
	t.Helper()
	var page struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if _, err := psychic.Do(ctx, http.MethodGet, "/api/v1/chat/conversations", nil, &page); err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	for _, conv := range page.Items {
		_, err := psychic.Do(ctx, http.MethodPost, "/api/v1/chat/conversations/"+conv.ID+"/notes",
			map[string]any{"body": "Load test setup note"}, nil)
		if err == nil {
			return conv.ID
		}
		// Skip conversations the advisor may not write in, and ones whose
		// note cap an earlier run already filled.
		if apiErr, ok := err.(*APIError); ok && (apiErr.Status == http.StatusForbidden || strings.Contains(apiErr.Code, "not_found") ||
			apiErr.Code == "limit_reached") {
			continue
		}
		t.Fatalf("probe conversation %s: %v", conv.ID, err)
	}
	t.Skipf("advisor has no conversation with a started session; run `go run ./cmd/seed -profile=large` first")
	return ""
}
