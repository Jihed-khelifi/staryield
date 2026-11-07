//go:build load

package load

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

// revisions 3.0: site settings. GET /site/config is the first request of
// every page view, so it must stay cheap: cached reads (in-process copy +
// Redis), conditional requests answered with 304, a few locales. The
// per-IP "site" bucket (300/min, burst 100) is lifted with
// STARYIELD_RATE_LIMIT_SCALE as for every load test.

// conditionalGet sends If-None-Match and treats 200 and 304 as success.
func conditionalGet(etags map[string]string) Request {
	locales := []string{"en", "fr", "es"}
	return func(ctx context.Context, c *Client, i int64) (int, error) {
		loc := locales[i%int64(len(locales))]
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/v1/site/config", nil)
		if err != nil {
			return 0, err
		}
		req.Header.Set("Origin", c.origin)
		req.Header.Set("X-Locale", loc)
		if tag := etags[loc]; tag != "" {
			req.Header.Set("If-None-Match", tag)
		}
		res, err := c.http.Do(req)
		if err != nil {
			return 0, err
		}
		defer res.Body.Close()
		_, _ = io.Copy(io.Discard, res.Body)
		if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNotModified {
			return res.StatusCode, fmt.Errorf("GET /site/config (%s): status %d", loc, res.StatusCode)
		}
		return res.StatusCode, nil
	}
}

func TestLoadSiteConfig(t *testing.T) {
	cfg := LoadConfig()
	RequireAPI(t, cfg)
	ctx := context.Background()
	pool := make([]*Client, 32)
	for i := range pool {
		pool[i] = NewClient(cfg.API, cfg.Origin)
	}

	// Sanity: the payload has the contract's shape and a validator per locale.
	etags := map[string]string{}
	for _, loc := range []string{"en", "fr", "es"} {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, cfg.API+"/api/v1/site/config?locale="+loc, nil)
		res, err := pool[0].http.Do(req)
		if err != nil {
			t.Fatalf("site config: %v", err)
		}
		var body struct {
			Banners struct {
				Items []json.RawMessage `json:"items"`
			} `json:"banners"`
			SignupOffer struct {
				Enabled bool `json:"enabled"`
			} `json:"signup_offer"`
		}
		err = json.NewDecoder(res.Body).Decode(&body)
		res.Body.Close()
		if err != nil || res.StatusCode != http.StatusOK || res.Header.Get("ETag") == "" {
			t.Fatalf("site config (%s): status %d etag %q: %v", loc, res.StatusCode, res.Header.Get("ETag"), err)
		}
		etags[loc] = res.Header.Get("ETag")
	}

	// Cached reads: the in-process copy answers; PostgreSQL is not touched
	// per request, so the budget is tight.
	fast := SLO{P95: 40 * time.Millisecond, P99: 120 * time.Millisecond, MaxErrRate: 0.001}
	scenarios := []Scenario{
		{Name: "GET /site/config", Rate: cfg.Rate(600), Duration: cfg.Duration, Do: get("/api/v1/site/config"), SLO: fast},
		{Name: "GET /site/config (locales)", Rate: cfg.Rate(400), Duration: cfg.Duration, SLO: fast,
			Do: func(ctx context.Context, c *Client, i int64) (int, error) {
				return c.Get(ctx, "/api/v1/site/config?locale="+[]string{"en", "fr", "es"}[i%3])
			}},
		{Name: "GET /site/config (If-None-Match → 304)", Rate: cfg.Rate(600), Duration: cfg.Duration, Do: conditionalGet(etags), SLO: fast},
	}
	results := runAll(ctx, scenarios, pool)
	Report(t, cfg, "site-config", results)
}
