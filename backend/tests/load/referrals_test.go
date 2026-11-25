//go:build load

package load

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Growth: the referral loop. GET /me/referrals is read by the account
// overview, the invite page and after every referral notification or wallet
// update; its first call per client creates the code (with collision retry).
// TestLoadReferrals:
//
//  1. every virtual user asks for its code at the same moment (concurrent
//     first calls: one code each, stable afterwards);
//  2. steady GET /me/referrals traffic while friends sign up with the invite
//     links (POST /auth/register with attribution.referral_code: the sign-up
//     path plus the referral's anti-abuse checks);
//  3. checks that every accepted sign-up is counted once by its inviter.
//
// Registration is limited per IP: run the API with STARYIELD_RATE_LIMIT_SCALE=1000.
func TestLoadReferrals(t *testing.T) {
	cfg := LoadConfig()
	RequireAPI(t, cfg)
	ctx := context.Background()
	pool := signedInClients(t, ctx, cfg, cfg.Users)

	type mine struct {
		Code    string `json:"code"`
		Program struct {
			Enabled bool `json:"enabled"`
		} `json:"program"`
		Stats struct {
			Joined int64 `json:"joined"`
		} `json:"stats"`
	}
	read := func(c *Client) (mine, error) {
		var out mine
		_, err := c.Do(ctx, http.MethodGet, "/api/v1/me/referrals", nil, &out)
		return out, err
	}

	// 1. Concurrent first calls.
	codes := make([]string, len(pool))
	before := make([]int64, len(pool))
	var wg sync.WaitGroup
	for i, c := range pool {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m, err := read(c)
			if err != nil {
				t.Errorf("first GET /me/referrals: %v", err)
				return
			}
			codes[i], before[i] = m.Code, m.Stats.Joined
		}()
	}
	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}
	seen := map[string]bool{}
	for i, code := range codes {
		if len(code) != 8 || seen[code] {
			t.Fatalf("user %d: code %q (empty, malformed or shared)", i, code)
		}
		seen[code] = true
	}
	m, _ := read(pool[0])
	if !m.Program.Enabled {
		t.Skip("the referral program is disabled on this API")
	}

	// 2. Steady reads while friends sign up with the links.
	run := time.Now().UnixNano()
	var signups sync.Map // inviter index → *atomic.Int64
	for i := range pool {
		signups.Store(i, new(atomic.Int64))
	}
	reads := SLO{P95: 100 * time.Millisecond, P99: 250 * time.Millisecond, MaxErrRate: 0.001}
	writes := SLO{P95: 400 * time.Millisecond, P99: 800 * time.Millisecond, MaxErrRate: 0.001}
	scenarios := []Scenario{
		{Name: "GET /me/referrals", Rate: cfg.Rate(150), Duration: cfg.Duration, Do: get("/api/v1/me/referrals"), SLO: reads},
		{Name: "POST /auth/register (invited)", Rate: cfg.Rate(10), Duration: cfg.Duration, MaxInFlight: 32, SLO: writes,
			Do: func(ctx context.Context, _ *Client, i int64) (int, error) {
				inviter := int(i % int64(len(codes)))
				friend := NewClient(cfg.API, cfg.Origin)
				status, err := friend.Do(ctx, http.MethodPost, "/api/v1/auth/register", map[string]any{
					"email": fmt.Sprintf("load-ref-%d-%d@example.test", run, i), "password": "L0ad!Test#Passw0rd", "accept_terms": true,
					"birth_date": "1992-02-11", "locale": "en", "attribution": map[string]any{"referral_code": codes[inviter], "landing_path": "/en"},
				}, nil)
				if err == nil {
					v, _ := signups.Load(inviter)
					v.(*atomic.Int64).Add(1)
				}
				return status, err
			}},
	}
	results := runAll(ctx, scenarios, pool)
	Report(t, cfg, "referrals", results)

	// 3. Every accepted sign-up is counted once by its inviter.
	for i, c := range pool {
		m, err := read(c)
		if err != nil {
			t.Fatalf("GET /me/referrals: %v", err)
		}
		v, _ := signups.Load(i)
		if want := before[i] + v.(*atomic.Int64).Load(); m.Code != codes[i] || m.Stats.Joined != want {
			t.Errorf("inviter %d: code %q (was %q), %d friends joined, want %d", i, m.Code, codes[i], m.Stats.Joined, want)
		}
	}
}
