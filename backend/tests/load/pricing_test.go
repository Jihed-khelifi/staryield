//go:build load

package load

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/seed"
)

// revisions 5.0: pricing catalog (§24.1) and the top-up catalog. GET /pricing
// is the first request of the public pricing page and is served from memory
// (in-process copy + Redis snapshot, ETag/304); GET /credits/catalog is one
// statement per request (wallet + welcome-offer eligibility as EXISTS
// subqueries) next to the same cached packs.
//
// Dataset: cmd/seed -profile=large -extend "credit purchases" gives the
// popularity aggregation and the eligibility checks real volume (about 390k
// purchases, 8k redeemed offers).

type pricingSample struct {
	// redeemed has a redeemed offer and purchases: the eligibility check's
	// worst case. fresh has neither.
	redeemed, fresh uuid.UUID
}

func pickPricingSample(db *gorm.DB) pricingSample {
	s := pricingSample{redeemed: uuid.New(), fresh: uuid.New()}
	var one struct{ A uuid.UUID }
	if db.Table("offer_redemptions").Select("user_id AS a").Limit(1).Scan(&one).Error == nil && one.A != uuid.Nil {
		s.redeemed = one.A
	}
	one = struct{ A uuid.UUID }{}
	if db.Raw(`SELECT w.user_id AS a FROM wallets w WHERE NOT EXISTS (SELECT 1 FROM credit_purchases p WHERE p.user_id = w.user_id) LIMIT 1`).
		Scan(&one).Error == nil && one.A != uuid.Nil {
		s.fresh = one.A
	}
	return s
}

// pricingChecks are the EXPLAIN checks of the pricing module.
func pricingChecks(s pricingSample) []queryCheck {
	return []queryCheck{
		// The "most popular" aggregation over the default 90-day window: an index-only scan
		// (cached 5 minutes, recomputed on any pack write). A 365-day window covers most of
		// the table and is a sequential scan by design (~120 ms at 390k purchases), still cached.
		{"pricing: purchases per pack (90 d)", 150 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewPricingRepository(d).PurchaseCounts(ctx, time.Now().AddDate(0, 0, -90))
		}},
		{"pricing: packs", 10 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewPricingRepository(d).Packs(ctx)
		}},
		// Top-up catalog: wallet + eligibility in one statement, per request.
		{"pricing: catalog state (offer redeemed)", 10 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewPricingRepository(d).CatalogState(ctx, s.redeemed)
		}},
		{"pricing: catalog state (no purchases)", 10 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewPricingRepository(d).CatalogState(ctx, s.fresh)
		}},
		{"offers: redemption of a purchase", 10 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewPricingRepository(d).OfferByPurchase(ctx, uuid.New())
		}},
	}
}

// pricingGet is a conditional GET of /pricing in a few locales.
func pricingGet(etags map[string]string, conditional bool) Request {
	locales := []string{"en", "fr", "es"}
	return func(ctx context.Context, c *Client, i int64) (int, error) {
		loc := locales[i%int64(len(locales))]
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/v1/pricing", nil)
		if err != nil {
			return 0, err
		}
		req.Header.Set("Origin", c.origin)
		req.Header.Set("X-Locale", loc)
		if conditional && etags[loc] != "" {
			req.Header.Set("If-None-Match", etags[loc])
		}
		res, err := c.http.Do(req)
		if err != nil {
			return 0, err
		}
		defer res.Body.Close()
		_, _ = io.Copy(io.Discard, res.Body)
		if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNotModified {
			return res.StatusCode, fmt.Errorf("GET /pricing (%s): status %d", loc, res.StatusCode)
		}
		return res.StatusCode, nil
	}
}

// loginSyntheticClients signs in n synthetic clients (some with purchases and
// a redeemed offer, some without) and returns a cleanup that removes the
// sessions and refresh tokens the sign-ins wrote. It writes nothing else.
func loginSyntheticClients(t *testing.T, ctx context.Context, cfg Config, db *gorm.DB, n int) []*Client {
	t.Helper()
	var emails []struct {
		ID    uuid.UUID
		Email string
	}
	db.Raw(`(SELECT u.id, u.email FROM users u JOIN offer_redemptions o ON o.user_id = u.id WHERE u.email LIKE ? LIMIT ?)
		UNION ALL (SELECT u.id, u.email FROM users u WHERE u.email LIKE ? AND u.role = 'client' AND NOT EXISTS
		  (SELECT 1 FROM credit_purchases p WHERE p.user_id = u.id) LIMIT ?)`,
		"%@"+seed.LoadEmailDomain, n/2, "%@"+seed.LoadEmailDomain, n-n/2).Scan(&emails)
	if len(emails) == 0 {
		t.Skip("no synthetic clients (run cmd/seed -profile=large)")
	}
	started := time.Now().Add(-time.Second)
	ids := make([]uuid.UUID, 0, len(emails))
	var (
		mu   sync.Mutex
		pool []*Client
		wg   sync.WaitGroup
		sem  = make(chan struct{}, 16)
	)
	for _, e := range emails {
		ids = append(ids, e.ID)
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			c := NewClient(cfg.API, cfg.Origin)
			if err := c.Login(ctx, e.Email, seed.LoadPassword); err != nil {
				return
			}
			mu.Lock()
			pool = append(pool, c)
			mu.Unlock()
		}()
	}
	wg.Wait()
	t.Cleanup(func() {
		for _, table := range []string{"refresh_tokens", "user_sessions"} {
			if err := db.Exec("DELETE FROM "+table+" WHERE user_id IN ? AND created_at >= ?", ids, started).Error; err != nil {
				t.Logf("cleanup %s: %v", table, err)
			}
		}
	})
	if len(pool) == 0 {
		t.Fatal("no synthetic client could sign in")
	}
	t.Logf("%d of %d synthetic clients signed in", len(pool), len(emails))
	return pool
}

func TestLoadPricing(t *testing.T) {
	cfg := LoadConfig()
	RequireAPI(t, cfg)
	ctx := context.Background()
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("database: %v", err)
	}
	anon := make([]*Client, 64)
	for i := range anon {
		anon[i] = NewClient(cfg.API, cfg.Origin)
	}
	etags := map[string]string{}
	for _, loc := range []string{"en", "fr", "es"} {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, cfg.API+"/api/v1/pricing?locale="+loc, nil)
		res, err := anon[0].http.Do(req)
		if err != nil {
			t.Fatalf("pricing: %v", err)
		}
		_, _ = io.Copy(io.Discard, res.Body)
		res.Body.Close()
		if res.StatusCode != http.StatusOK || res.Header.Get("ETag") == "" {
			t.Fatalf("pricing (%s): status %d etag %q", loc, res.StatusCode, res.Header.Get("ETag"))
		}
		etags[loc] = res.Header.Get("ETag")
	}
	signed := loginSyntheticClients(t, ctx, cfg, db, cfg.Users)
	pool := append(append([]*Client{}, anon...), signed...)

	// Cached reads: the in-process copy answers, PostgreSQL is not touched per request.
	cached := SLO{P95: 40 * time.Millisecond, P99: 120 * time.Millisecond, MaxErrRate: 0.001}
	// The catalog adds one indexed statement per request.
	catalog := SLO{P95: 60 * time.Millisecond, P99: 200 * time.Millisecond, MaxErrRate: 0.001}
	scenarios := []Scenario{
		{Name: "GET /pricing (locales)", Rate: cfg.Rate(1500), Duration: cfg.Duration, Do: pricingGet(etags, false), SLO: cached},
		{Name: "GET /pricing (If-None-Match → 304)", Rate: cfg.Rate(1500), Duration: cfg.Duration, Do: pricingGet(etags, true), SLO: cached},
		{Name: "GET /credits/catalog", Rate: cfg.Rate(600), Duration: cfg.Duration, SLO: catalog,
			Do: func(ctx context.Context, c *Client, i int64) (int, error) {
				return signed[int(i)%len(signed)].Get(ctx, "/api/v1/credits/catalog")
			}},
	}
	results := runAll(ctx, scenarios, pool)
	Report(t, cfg, "pricing", results)
}
