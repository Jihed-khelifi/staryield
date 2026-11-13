package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/migrations"
	"github.com/Jihed-khelifi/staryield/backend/internal/seed"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 5.0: pricing catalog and welcome offer (§24.1, §24.2).

var canonicalOrder = "welcome_600,quick_150,starter_315,popular_675,deep_1350,premium_2400"

// isolateCatalog parks every active pack that is not part of the canonical
// catalog (other tests create reference packs in the shared database), so the
// badges are those of the canonical catalog. They are re-activated after the
// test.
func (e *env) isolateCatalog() {
	e.t.Helper()
	var ids []uuid.UUID
	if err := e.app.DB.Raw(`UPDATE credit_packs SET active = false
		WHERE active AND code NOT IN ('welcome_600','quick_150','starter_315','popular_675','deep_1350','premium_2400')
		RETURNING id`).Scan(&ids).Error; err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() {
		if len(ids) > 0 {
			e.app.DB.Exec("UPDATE credit_packs SET active = true WHERE id IN ?", ids)
		}
	})
}

func (e *env) pricing(c *client, query string) (service.PricingSummary, response) {
	e.t.Helper()
	r := c.call(http.MethodGet, "/api/v1/pricing"+query, nil)
	var out service.PricingSummary
	if r.status == http.StatusOK {
		r.decode(e.t, &out)
	}
	return out, r
}

func packByCode(t *testing.T, packs []service.PricingPack, code string) service.PricingPack {
	t.Helper()
	for _, p := range packs {
		if p.Code == code {
			return p
		}
	}
	t.Fatalf("no pack %s in %+v", code, packs)
	return service.PricingPack{}
}

func badges(p service.PricingPack) string {
	var parts []string
	for _, b := range p.Badges {
		if b.Type == service.BadgeSave {
			parts = append(parts, "save:"+itoa(b.Percent))
		} else {
			parts = append(parts, b.Type)
		}
	}
	return strings.Join(parts, ",")
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestPublicPricing(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	e.isolateCatalog()
	c := e.client()

	sum, r := e.pricing(c, "")
	if r.status != http.StatusOK {
		t.Fatalf("GET /pricing: %d %s", r.status, r.body)
	}
	if cc := r.header.Get("Cache-Control"); cc != "public, max-age=60, stale-while-revalidate=300" {
		t.Fatalf("Cache-Control: %q", cc)
	}
	if v := strings.Join(r.header.Values("Vary"), ","); !strings.Contains(v, "Accept-Language") || !strings.Contains(v, "X-Locale") {
		t.Fatalf("Vary: %q", v)
	}
	etag := r.header.Get("ETag")
	if !strings.HasPrefix(etag, `W/"`) {
		t.Fatalf("ETag: %q", etag)
	}
	var codes []string
	for _, p := range sum.Packs {
		codes = append(codes, p.Code)
	}
	if strings.Join(codes, ",") != canonicalOrder {
		t.Fatalf("canonical order: %v", codes)
	}
	want := map[string]string{"welcome_600": "new_client_offer", "quick_150": "", "starter_315": "save:4", "popular_675": "save:11",
		"deep_1350": "save:16", "premium_2400": "best_value,save:21"}
	for code, w := range want {
		if got := badges(packByCode(t, sum.Packs, code)); got != w {
			t.Errorf("%s: badges %q, want %q", code, got, w)
		}
	}
	if sum.Reference == nil || sum.Reference.Credits != 150 || sum.Reference.PriceCents != 2000 || sum.Currency != "USD" ||
		sum.RateRange.Min != 30 || sum.RateRange.Max != 60 || !sum.FreeTrial.Enabled || sum.FreeTrial.Minutes != 4 {
		t.Fatalf("summary: %+v", sum)
	}
	pop := packByCode(t, sum.Packs, "popular_675")
	if !pop.Featured || pop.Tagline == "" || packByCode(t, sum.Packs, "quick_150").Label != "One focused question" {
		t.Fatalf("featured / tagline / label: %+v", pop)
	}
	for _, internal := range []string{"processing", "advisor_share", "contribution", "popular_min"} {
		if strings.Contains(string(r.body), internal) {
			t.Fatalf("internal economics leaked (%s): %s", internal, r.body)
		}
	}

	// Conditional requests.
	if nm := c.raw(http.MethodGet, "/api/v1/pricing", nil, map[string]string{"If-None-Match": etag}); nm.status != http.StatusNotModified || len(nm.body) != 0 {
		t.Fatalf("If-None-Match: %d %s", nm.status, nm.body)
	}
	if st := c.raw(http.MethodGet, "/api/v1/pricing", nil, map[string]string{"If-None-Match": `W/"other"`}).status; st != http.StatusOK {
		t.Fatalf("other validator: %d", st)
	}

	// Locale: X-Locale, ?locale= and Accept-Language; unknown falls back to English.
	fr, rf := e.pricing(c, "?locale=fr")
	if packByCode(t, fr.Packs, "welcome_600").Name != "Pack de bienvenue" || rf.header.Get("ETag") == etag {
		t.Fatalf("?locale=fr: %+v %s", fr.Packs[0], rf.header.Get("ETag"))
	}
	es := e.client()
	es.locale = "es"
	if s, _ := e.pricing(es, ""); packByCode(t, s.Packs, "quick_150").Label != "Una pregunta concreta" {
		t.Fatalf("X-Locale es: %+v", s.Packs[1])
	}
	al := c.raw(http.MethodGet, "/api/v1/pricing", nil, map[string]string{"Accept-Language": "fr-FR,fr;q=0.9"})
	var viaHeader service.PricingSummary
	al.decode(t, &viaHeader)
	if packByCode(t, viaHeader.Packs, "popular_675").Name != "Le plus populaire" {
		t.Fatalf("Accept-Language: %+v", viaHeader.Packs[3])
	}
	if s, _ := e.pricing(c, "?locale=de"); packByCode(t, s.Packs, "popular_675").Name != "Most Popular" {
		t.Fatal("an unknown locale reads as English")
	}

	// The legacy endpoint keeps working and now carries the code.
	var legacy struct {
		Items []service.PackView `json:"items"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/credit-packs", nil, &legacy)
	found := false
	for _, p := range legacy.Items {
		found = found || (p.Code == "starter_315" && p.Credits == 315 && p.PriceCents == 4000)
	}
	if !found {
		t.Fatalf("legacy packs: %+v", legacy.Items)
	}
}

func TestPricingFollowsTheSignupOfferAndAdminWrites(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	e.isolateCatalog()
	c := e.client()
	admin := e.createAdmin()

	// free_trial is the live sign-up offer (§15.3).
	var offer service.SignupOffer
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/site/signup-offer", nil, &offer)
	t.Cleanup(func() {
		e.app.DB.Exec(`UPDATE site_settings SET value = '{"enabled": true, "free_minutes": 4}', version = version + 1 WHERE key = 'signup_offer'`)
	})
	admin.stepUp()
	admin.must(http.StatusOK, http.MethodPut, "/api/v1/admin/site/signup-offer", map[string]any{"enabled": true, "free_minutes": 4}, nil)
	if s, _ := e.pricing(c, ""); !s.FreeTrial.Enabled || s.FreeTrial.Minutes != 4 {
		t.Fatalf("free trial: %+v", s.FreeTrial)
	}
	admin.must(http.StatusOK, http.MethodPut, "/api/v1/admin/site/signup-offer", map[string]any{"enabled": false, "free_minutes": 4}, nil)
	if s, _ := e.pricing(c, ""); s.FreeTrial.Enabled || s.FreeTrial.Minutes != 0 {
		t.Fatalf("free trial off: %+v", s.FreeTrial)
	}

	// A pack write is visible at once (the snapshot is invalidated), and the
	// savings are recomputed from the live prices.
	_, r := e.pricing(c, "")
	before := r.header.Get("ETag")
	var created service.AdminPack
	code := uniq("promo_")
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/credit-packs", map[string]any{"code": code,
		"name": map[string]string{"en": "Promo", "fr": "Promo FR"}, "credits": 500, "price_cents": 5000}, &created)
	after, r2 := e.pricing(c, "")
	if r2.header.Get("ETag") == before {
		t.Fatal("the validator must change with the catalog")
	}
	promo := packByCode(t, after.Packs, code)
	// 5000/500 = 10.0 c per credit vs 13.33: floor(100 x (2000x500 - 5000x150) / (2000x500)) = 25.
	if badges(promo) != "best_value,save:25" || badges(packByCode(t, after.Packs, "premium_2400")) != "save:21" {
		t.Fatalf("badges after the write: promo %q, premium %q", badges(promo), badges(packByCode(t, after.Packs, "premium_2400")))
	}
	if after.Packs[len(after.Packs)-1].Code != code {
		t.Fatalf("a new pack is appended last: %+v", after.Packs)
	}
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/credit-packs/"+created.ID.String(), map[string]any{"active": false}, nil)
	if s, _ := e.pricing(c, ""); len(s.Packs) != 6 || badges(packByCode(t, s.Packs, "premium_2400")) != "best_value,save:21" {
		t.Fatalf("deactivating the pack restores the catalog: %+v", s.Packs)
	}
	// A second instance hears about changes over Pub/Sub (tested through Redis
	// directly: the cached snapshot is gone after a write).
	if n, err := e.app.Redis.Exists(t.Context(), e.app.Config.Redis.KeyPrefix+"pricing:v1").Result(); err != nil || n != 1 {
		t.Fatalf("the snapshot is cached in Redis after a read: %d %v", n, err)
	}
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/credit-packs/"+created.ID.String(), map[string]any{"featured": true}, nil)
	if n, _ := e.app.Redis.Exists(t.Context(), e.app.Config.Redis.KeyPrefix+"pricing:v1").Result(); n != 0 {
		t.Fatal("a write drops the Redis snapshot")
	}
}

func TestMostPopularNeedsEvidence(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	e.isolateCatalog()
	c := e.client()
	admin := e.createAdmin()
	t.Cleanup(func() {
		e.app.DB.Exec(`UPDATE site_settings SET value = '{"rate_min": 30, "rate_max": 60, "popular_min_purchases": 25, "popular_window_days": 90, "processing_fee_bps": 800, "processing_fixed_cents": 30}', version = version + 1 WHERE key = 'pricing'`)
	})
	if s, _ := e.pricing(c, ""); strings.Contains(badges(packByCode(t, s.Packs, "popular_675")), "most_popular") {
		t.Fatal("no purchases: nobody is most popular")
	}
	// Seed succeeded purchases of one standard pack directly (history), then lower the threshold.
	buyer := e.registerClient()
	pack := packByCode(t, mustPricing(e, c).Packs, "popular_675")
	for i := 0; i < 3; i++ {
		if err := e.app.DB.Exec(`INSERT INTO credit_purchases (id, user_id, pack_id, credits, price_cents, currency, status, provider, idempotency_key, completed_at)
			VALUES (?, ?, ?, 675, 8000, 'USD', 'succeeded', 'sandbox', ?, ?)`, uuid.New(), buyer.user.ID, pack.ID, uniq("hist-"), e.clock.Now()).Error; err != nil {
			t.Fatal(err)
		}
	}
	var settings service.PricingSettings
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/pricing/settings", nil, &settings)
	admin.stepUp()
	put := func(minPurchases int) {
		admin.must(http.StatusOK, http.MethodPut, "/api/v1/admin/pricing/settings", map[string]any{"rate_min": 30, "rate_max": 60,
			"popular_min_purchases": minPurchases, "popular_window_days": 90, "processing_fee_bps": 800, "processing_fixed_cents": 30,
			"expected_version": settings.Version}, &settings)
	}
	put(25)
	if s, _ := e.pricing(c, ""); strings.Contains(badges(packByCode(t, s.Packs, "popular_675")), "most_popular") {
		t.Fatal("3 purchases are below a threshold of 25")
	}
	put(3)
	if got := badges(packByCode(t, mustPricing(e, c).Packs, "popular_675")); got != "most_popular,save:11" {
		t.Fatalf("at the threshold with a strict leader: %q", got)
	}
	// The admin list reports the window count.
	var list struct {
		Items []service.AdminPack `json:"items"`
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/credit-packs", nil, &list)
	for _, p := range list.Items {
		if p.Code == "popular_675" && (p.PurchasesWindow != 3 || badges2(p.Badges) != "most_popular,save:11") {
			t.Fatalf("admin view: %+v", p)
		}
	}
}

func badges2(bs []service.PackBadge) string {
	return badges(service.PricingPack{Badges: bs})
}

func mustPricing(e *env, c *client) service.PricingSummary {
	e.t.Helper()
	s, r := e.pricing(c, "")
	if r.status != http.StatusOK {
		e.t.Fatalf("pricing: %d", r.status)
	}
	return s
}

func TestCanonicalCatalogMatchesTheSeed(t *testing.T) {
	e := newEnv(t)
	var packs []struct {
		Code, Kind, Name                            string
		Credits, PriceCents                         int64
		Active, Featured, Reference, NewClientsOnly bool
		SortOrder                                   int
		NameL10n, TaglineL10n, LabelL10n            string
	}
	if err := e.app.DB.Raw(`SELECT code, kind, name, credits, price_cents, active, featured, reference, new_clients_only, sort_order,
		name_l10n::text AS name_l10n, tagline_l10n::text AS tagline_l10n, label_l10n::text AS label_l10n
		FROM credit_packs WHERE code IN ('welcome_600','quick_150','starter_315','popular_675','deep_1350','premium_2400') ORDER BY sort_order`).Scan(&packs).Error; err != nil {
		t.Fatal(err)
	}
	canon := seed.CanonicalPacks()
	if len(packs) != len(canon) {
		t.Fatalf("migration inserted %d canonical packs, seed defines %d", len(packs), len(canon))
	}
	same := func(raw string, want map[string]string) bool {
		var got map[string]string
		_ = json.Unmarshal([]byte(raw), &got)
		if len(got) != len(want) {
			return false
		}
		for k, v := range want {
			if got[k] != v {
				return false
			}
		}
		return true
	}
	for i, m := range packs {
		s := canon[i]
		if m.Code != s.Code || m.Kind != s.Kind || m.Name != s.Name || m.Credits != s.Credits || m.PriceCents != s.PriceCents ||
			m.Active != s.Active || m.Featured != s.Featured || m.Reference != s.Reference || m.NewClientsOnly != s.NewClientsOnly ||
			m.SortOrder != s.SortOrder || !same(m.NameL10n, s.NameL10n) || !same(m.TaglineL10n, s.TaglineL10n) || !same(m.LabelL10n, s.LabelL10n) {
			t.Errorf("migration and seed disagree on %s:\n migration %+v\n seed      %+v", s.Code, m, s)
		}
	}
}

// ---------------------------------------------------------------------------
// top-up catalog and the welcome offer
// ---------------------------------------------------------------------------

func (e *env) catalog(c *client) service.TopUpCatalog {
	e.t.Helper()
	var out service.TopUpCatalog
	c.must(http.StatusOK, http.MethodGet, "/api/v1/credits/catalog", nil, &out)
	return out
}

func catalogCodes(cat service.TopUpCatalog) string {
	var out []string
	for _, p := range cat.Packs {
		out = append(out, p.Code)
	}
	return strings.Join(out, ",")
}

func (c *client) buy(packID uuid.UUID, extra map[string]any) response {
	c.e.t.Helper()
	body := map[string]any{"pack_id": packID, "idempotency_key": uniq("purchase-key-"), "consent": purchaseConsent()}
	for k, v := range extra {
		body[k] = v
	}
	return c.call(http.MethodPost, "/api/v1/credits/purchases", body)
}

func (e *env) offerStatuses(userID uuid.UUID) []string {
	e.t.Helper()
	var out []string
	if err := e.app.DB.Raw("SELECT status FROM offer_redemptions WHERE user_id = ? ORDER BY created_at", userID).Scan(&out).Error; err != nil {
		e.t.Fatal(err)
	}
	return out
}

func offerReason(t *testing.T, r response) string {
	t.Helper()
	var body struct {
		Error struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal(r.body, &body)
	if r.status != http.StatusConflict || body.Error.Code != "offer_unavailable" {
		t.Fatalf("expected offer_unavailable, got %d %s", r.status, r.body)
	}
	reason, _ := body.Error.Details["reason"].(string)
	return reason
}

func TestTopUpCatalogRequiresAClient(t *testing.T) {
	e := newEnv(t)
	if r := e.client().call(http.MethodGet, "/api/v1/credits/catalog", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", r.status)
	}
	if r := e.createPsychic(30).call(http.MethodGet, "/api/v1/credits/catalog", nil); r.status != http.StatusForbidden {
		t.Fatalf("advisor: %d", r.status)
	}
}

func TestWelcomeOfferLifecycle(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	e.isolateCatalog()
	c := e.registerClient()
	ids := map[string]uuid.UUID{}
	for _, p := range mustPricing(e, e.client()).Packs {
		ids[p.Code] = p.ID
	}

	// A new client: the welcome pack first, then the standard packs.
	cat := e.catalog(c)
	if !cat.WelcomeOffer.Eligible || cat.WelcomeOffer.PackID == nil || *cat.WelcomeOffer.PackID != ids["welcome_600"] ||
		catalogCodes(cat) != canonicalOrder || cat.RateRange.Min != 30 || cat.RateRange.Max != 60 || cat.Wallet.FreeMinutes != 4 {
		t.Fatalf("new client catalog: %+v", cat)
	}
	if cat.Packs[0].Name != "Welcome Pack" || badges(cat.Packs[0]) != "new_client_offer" {
		t.Fatalf("welcome pack: %+v", cat.Packs[0])
	}
	if cc := c.call(http.MethodGet, "/api/v1/credits/catalog", nil).header.Get("Cache-Control"); !strings.Contains(cc, "no-store") && !strings.Contains(cc, "private") {
		t.Fatalf("the catalog is per account: %q", cc)
	}
	fr := e.login(c.user.Email, strongPassword)
	fr.locale = "fr"
	if got := e.catalog(fr).Packs[0].Name; got != "Pack de bienvenue" {
		t.Fatalf("catalog locale: %q", got)
	}

	// The hosted checkout has no card to check.
	if reason := offerReason(t, c.buy(ids["welcome_600"], nil)); reason != "card_check_unavailable" {
		t.Fatalf("hosted checkout: %s", reason)
	}
	if len(e.offerStatuses(c.user.ID)) != 0 {
		t.Fatal("a refused purchase leaves no reservation")
	}

	// A declined card releases the offer: the client may retry.
	bad := c.attachCard("sbx_a_visa_0002_12_2035")
	if r := c.buy(ids["welcome_600"], map[string]any{"payment_method_id": bad.ID}); r.status != http.StatusPaymentRequired || r.errCode(t) != "payment_failed" {
		t.Fatalf("declined: %d %s", r.status, r.body)
	}
	if st := e.offerStatuses(c.user.ID); len(st) != 1 || st[0] != "released" {
		t.Fatalf("decline releases: %v", st)
	}
	if cat := e.catalog(c); !cat.WelcomeOffer.Eligible || catalogCodes(cat) != canonicalOrder {
		t.Fatalf("still eligible after a decline: %+v", cat)
	}

	// The retry succeeds; the offer is redeemed and gone from the catalog.
	good := c.attachCard("sbx_a_visa_4242_12_2035")
	var p service.PurchaseView
	r := c.buy(ids["welcome_600"], map[string]any{"payment_method_id": good.ID})
	if r.status != http.StatusCreated {
		t.Fatalf("welcome purchase: %d %s", r.status, r.body)
	}
	r.decode(t, &p)
	if p.Status != "succeeded" || p.Credits != 600 || p.PriceCents != 1000 || p.Wallet.Balance != 600 {
		t.Fatalf("purchase: %+v", p)
	}
	if st := e.offerStatuses(c.user.ID); len(st) != 2 || st[0] != "released" || st[1] != "redeemed" {
		t.Fatalf("redeemed: %v", st)
	}
	cat = e.catalog(c)
	if cat.WelcomeOffer.Eligible || cat.WelcomeOffer.PackID != nil || strings.Contains(catalogCodes(cat), "welcome") || cat.Wallet.Balance != 600 {
		t.Fatalf("redeemed offer is absent from the catalog: %+v", cat)
	}
	if reason := offerReason(t, c.buy(ids["welcome_600"], map[string]any{"payment_method_id": good.ID})); reason != "already_redeemed" {
		t.Fatalf("second purchase: %s", reason)
	}
	// The raw card fingerprint is never stored: the key is a keyed hash.
	var keys []string
	e.app.DB.Raw("SELECT card_key FROM offer_redemptions WHERE user_id = ? AND status = 'redeemed'", c.user.ID).Scan(&keys)
	if len(keys) != 1 || len(keys[0]) != 64 || strings.Contains(keys[0], "fp_") {
		t.Fatalf("card key: %v", keys)
	}

	// One per payment method: the same card on another account, saved or fresh.
	d := e.registerClient()
	if reason := offerReason(t, d.buy(ids["welcome_600"], map[string]any{"payment_token": "sbx_a_visa_4242_12_2035", "provider": "sandbox_a"})); reason != "payment_method_used" {
		t.Fatalf("same card, fresh token: %s", reason)
	}
	same := d.attachCard("sbx_a_visa_4242_12_2035")
	if reason := offerReason(t, d.buy(ids["welcome_600"], map[string]any{"payment_method_id": same.ID})); reason != "payment_method_used" {
		t.Fatalf("same card, saved: %s", reason)
	}
	if len(e.offerStatuses(d.user.ID)) != 0 || !e.catalog(d).WelcomeOffer.Eligible {
		t.Fatal("a refused purchase leaves the other account eligible")
	}
	if w := d.wallet(); w.Balance != 0 {
		t.Fatalf("nothing was charged: %+v", w)
	}
	other := d.attachCard("sbx_a_mastercard_4444_12_2035")
	if r := d.buy(ids["welcome_600"], map[string]any{"payment_method_id": other.ID}); r.status != http.StatusCreated {
		t.Fatalf("another card on the other account: %d %s", r.status, r.body)
	}

	// A refund keeps the offer redeemed (Refund Policy §5).
	admin := e.createAdmin()
	var txs service.Page[service.AdminTransactionRow]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/billing/transactions?kind=credit_purchase&user_id="+c.user.ID.String()+"&status=succeeded", nil, &txs)
	if len(txs.Items) != 1 {
		t.Fatalf("transactions: %+v", txs.Items)
	}
	admin.stepUp()
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/billing/transactions/"+txs.Items[0].ID.String()+"/refund",
		map[string]any{"amount_cents": 1000, "reason": "Goodwill gesture"}, nil)
	if cat := e.catalog(c); cat.WelcomeOffer.Eligible || strings.Contains(catalogCodes(cat), "welcome") {
		t.Fatalf("a refund must not restore eligibility: %+v", cat)
	}
	if st := e.offerStatuses(c.user.ID); st[len(st)-1] != "redeemed" {
		t.Fatalf("still redeemed after the refund: %v", st)
	}
	if reason := offerReason(t, c.buy(ids["welcome_600"], map[string]any{"payment_method_id": good.ID})); reason != "already_redeemed" {
		t.Fatalf("after the refund: %s", reason)
	}
}

func TestWelcomeOfferIsForNewClients(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	e.isolateCatalog()
	c := e.registerClient()
	ids := map[string]uuid.UUID{}
	for _, p := range mustPricing(e, e.client()).Packs {
		ids[p.Code] = p.ID
	}
	// A first purchase of a standard pack (hosted checkout).
	if r := c.buy(ids["quick_150"], nil); r.status != http.StatusCreated {
		t.Fatalf("standard purchase: %d %s", r.status, r.body)
	}
	cat := e.catalog(c)
	if cat.WelcomeOffer.Eligible || catalogCodes(cat) != "quick_150,starter_315,popular_675,deep_1350,premium_2400" || cat.Wallet.Balance != 150 {
		t.Fatalf("after a purchase: %+v", cat)
	}
	card := c.attachCard("sbx_a_visa_4242_12_2035")
	if reason := offerReason(t, c.buy(ids["welcome_600"], map[string]any{"payment_method_id": card.ID})); reason != "not_new_client" {
		t.Fatalf("not a new client: %s", reason)
	}
	// An inactive welcome pack is no offer at all.
	e.app.DB.Exec("UPDATE credit_packs SET active = false WHERE code = 'welcome_600'")
	t.Cleanup(func() { e.app.DB.Exec("UPDATE credit_packs SET active = true WHERE code = 'welcome_600'") })
	if reason := offerReason(t, c.buy(ids["welcome_600"], map[string]any{"payment_method_id": card.ID})); reason != "no_offer" {
		t.Fatalf("no intro pack: %s", reason)
	}
}

func TestWelcomeOfferCardCheckSwitch(t *testing.T) {
	setup := func(e *env) (*client, uuid.UUID) {
		e.ensurePacks()
		e.isolateCatalog()
		c := e.registerClient()
		var id uuid.UUID
		for _, p := range mustPricing(e, e.client()).Packs {
			if p.Code == "welcome_600" {
				id = p.ID
			}
		}
		card := c.attachCard("sbx_a_visa_4242_12_2035")
		// A provider that cannot tell cards apart: no fingerprint on the saved card.
		e.app.DB.Exec("UPDATE payment_methods SET fingerprint_hash = '' WHERE id = ?", card.ID)
		return c, id
	}
	e := newEnv(t)
	c, id := setup(e)
	cards := c.cards()
	if reason := offerReason(t, c.buy(id, map[string]any{"payment_method_id": cards[0].ID})); reason != "card_check_unavailable" {
		t.Fatalf("default (card check required): %s", reason)
	}
	if len(e.offerStatuses(c.user.ID)) != 0 {
		t.Fatal("no reservation without a card check")
	}

	off := newEnv(t, func(cfg *config.Config) { cfg.Pricing.OfferRequireCardCheck = false })
	c2, id2 := setup(off)
	if r := c2.buy(id2, map[string]any{"payment_method_id": c2.cards()[0].ID}); r.status != http.StatusCreated {
		t.Fatalf("card check off: %d %s", r.status, r.body)
	}
	var keys []string
	off.app.DB.Raw("SELECT card_key FROM offer_redemptions WHERE user_id = ?", c2.user.ID).Scan(&keys)
	if len(keys) != 1 || keys[0] != "" {
		t.Fatalf("stored without a card key: %v", keys)
	}
	// Still one per account.
	if reason := offerReason(t, c2.buy(id2, map[string]any{"payment_method_id": c2.cards()[0].ID})); reason != "already_redeemed" {
		t.Fatalf("per account: %s", reason)
	}
}

func TestWelcomeOfferPendingChargeAndReconciliation(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	e.isolateCatalog()
	admin := e.createAdmin()
	// A test intro pack priced so the sandbox loses the response (.15): the
	// charge succeeds at the processor but the outcome is unknown.
	e.app.DB.Exec("UPDATE credit_packs SET active = false WHERE code = 'welcome_600'")
	t.Cleanup(func() {
		// Two statements: the unique index allows one active intro pack at any time.
		e.app.DB.Exec("UPDATE credit_packs SET active = false WHERE kind = 'intro' AND code <> 'welcome_600'")
		e.app.DB.Exec("UPDATE credit_packs SET active = true WHERE code = 'welcome_600'")
	})
	var intro service.AdminPack
	admin.stepUp()
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/credit-packs", map[string]any{"code": uniq("intro_"), "kind": "intro",
		"name": map[string]string{"en": "Timeout offer"}, "credits": 100, "price_cents": 1015}, &intro)
	if !intro.NewClientsOnly || intro.Kind != "intro" {
		t.Fatalf("an intro pack defaults to new clients only: %+v", intro)
	}

	buyer := func(token string) (*client, service.PurchaseView) {
		c := e.registerClient()
		card := c.attachCard(token)
		var p service.PurchaseView
		r := c.buy(intro.ID, map[string]any{"payment_method_id": card.ID})
		if r.status != http.StatusCreated {
			t.Fatalf("intro purchase: %d %s", r.status, r.body)
		}
		r.decode(t, &p)
		return c, p
	}
	// Unknown outcome: the purchase is pending and the offer reserved (it blocks a second try).
	c1, p1 := buyer("sbx_a_visa_4242_01_2036")
	if p1.Status != "pending" {
		t.Fatalf("purchase: %+v", p1)
	}
	if st := e.offerStatuses(c1.user.ID); len(st) != 1 || st[0] != "reserved" {
		t.Fatalf("reserved while pending: %v", st)
	}
	if cat := e.catalog(c1); cat.WelcomeOffer.Eligible {
		t.Fatalf("a reserved offer is not offered again: %+v", cat)
	}
	// Reconciliation resolves it: the charge went through, so the offer is redeemed.
	e.tick(6 * time.Minute)
	if st := e.offerStatuses(c1.user.ID); len(st) != 1 || st[0] != "redeemed" {
		t.Fatalf("redeemed by reconciliation: %v", st)
	}
	if w := c1.wallet(); w.Balance != 100 {
		t.Fatalf("credited once: %+v", w)
	}

	// A reconciliation that fails releases the offer and the client may retry.
	c2, p2 := buyer("sbx_a_visa_4242_02_2036")
	if err := e.app.Services.Credits.FailPurchase(t.Context(), p2.ID, "card_declined"); err != nil {
		t.Fatal(err)
	}
	if st := e.offerStatuses(c2.user.ID); len(st) != 1 || st[0] != "released" {
		t.Fatalf("released after a failed reconciliation: %v", st)
	}
	if cat := e.catalog(c2); !cat.WelcomeOffer.Eligible {
		t.Fatalf("eligible again: %+v", cat)
	}
	card := c2.attachCard("sbx_a_visa_4242_03_2036")
	if r := c2.buy(intro.ID, map[string]any{"payment_method_id": card.ID}); r.status != http.StatusCreated {
		t.Fatalf("retry: %d %s", r.status, r.body)
	}
	if st := e.offerStatuses(c2.user.ID); len(st) != 2 || st[0] != "released" || st[1] != "reserved" {
		t.Fatalf("retry reserves again: %v", st)
	}
}

var cardSeq struct {
	sync.Mutex
	n int
}

// freshCardToken is a sandbox token for a card no other test has used (the
// welcome offer is one per card and the suite shares one database).
func freshCardToken() string {
	cardSeq.Lock()
	defer cardSeq.Unlock()
	cardSeq.n++
	return "sbx_a_visa_" + fmt.Sprintf("%04d", 5000+cardSeq.n) + "_12_2037"
}

// post sends a purchase without touching the shared client state, so several
// goroutines may use one signed-in browser.
func (c *client) post(path string, payload any) (int, []byte) {
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, c.e.srv.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", c.origin)
	req.Header.Set("X-CSRF-Token", c.csrf)
	res, err := c.http.Do(req)
	if err != nil {
		return 0, []byte(err.Error())
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, body
}

func TestWelcomeOfferIsReservedExactlyOnceUnderConcurrency(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	e.isolateCatalog()
	var welcome uuid.UUID
	for _, p := range mustPricing(e, e.client()).Packs {
		if p.Code == "welcome_600" {
			welcome = p.ID
		}
	}

	run := func(attempts []func() (int, []byte)) (created int, reasons []string) {
		var wg sync.WaitGroup
		var mu sync.Mutex
		start := make(chan struct{})
		for _, a := range attempts {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				status, body := a()
				mu.Lock()
				defer mu.Unlock()
				if status == http.StatusCreated {
					created++
					return
				}
				var out struct {
					Error struct {
						Code    string         `json:"code"`
						Details map[string]any `json:"details"`
					} `json:"error"`
				}
				_ = json.Unmarshal(body, &out)
				reason, _ := out.Error.Details["reason"].(string)
				reasons = append(reasons, out.Error.Code+":"+reason)
			}()
		}
		close(start)
		wg.Wait()
		return created, reasons
	}

	// One account, several simultaneous purchases (different keys, same card).
	c := e.registerClient()
	card := c.attachCard(freshCardToken())
	c.ensureCSRF()
	var attempts []func() (int, []byte)
	for i := 0; i < 6; i++ {
		key := uniq("purchase-key-")
		attempts = append(attempts, func() (int, []byte) {
			return c.post("/api/v1/credits/purchases", map[string]any{"pack_id": welcome, "idempotency_key": key, "payment_method_id": card.ID, "consent": purchaseConsent()})
		})
	}
	created, reasons := run(attempts)
	if created != 1 {
		t.Fatalf("exactly one purchase may win, got %d (others: %v)", created, reasons)
	}
	for _, r := range reasons {
		if r != "offer_unavailable:already_redeemed" {
			t.Fatalf("the losers are told the offer is used: %v", reasons)
		}
	}
	var active int64
	e.app.DB.Raw("SELECT count(*) FROM offer_redemptions WHERE user_id = ? AND status IN ('reserved','redeemed')", c.user.ID).Scan(&active)
	if w := c.wallet(); active != 1 || w.Balance != 600 {
		t.Fatalf("one reservation, one credit: %d active, wallet %+v", active, w)
	}

	// Several accounts with the same card: exactly one reservation for that card.
	const accounts = 6
	shared := freshCardToken()
	var cardAttempts []func() (int, []byte)
	var users []uuid.UUID
	for i := 0; i < accounts; i++ {
		u := e.registerClient()
		m := u.attachCard(shared)
		u.ensureCSRF()
		users = append(users, u.user.ID)
		key := uniq("purchase-key-")
		cardAttempts = append(cardAttempts, func() (int, []byte) {
			return u.post("/api/v1/credits/purchases", map[string]any{"pack_id": welcome, "idempotency_key": key, "payment_method_id": m.ID, "consent": purchaseConsent()})
		})
	}
	created, reasons = run(cardAttempts)
	if created != 1 {
		t.Fatalf("one card, one offer: %d winners (others: %v)", created, reasons)
	}
	for _, r := range reasons {
		if r != "offer_unavailable:payment_method_used" {
			t.Fatalf("the losers are told the card was used: %v", reasons)
		}
	}
	var perCard int64
	e.app.DB.Raw("SELECT count(*) FROM offer_redemptions WHERE user_id IN ? AND status IN ('reserved','redeemed')", users).Scan(&perCard)
	if perCard != 1 {
		t.Fatalf("active redemptions for the card: %d", perCard)
	}
	var charged int
	e.app.DB.Raw("SELECT count(*) FROM credit_purchases WHERE user_id IN ? AND status = 'succeeded'", users).Scan(&charged)
	if charged != 1 {
		t.Fatalf("only the winner was charged: %d", charged)
	}
}

// Migration 0026 keeps existing packs (purchases reference them): they get
// legacy_<8 hex> codes, an English l10n name, are deactivated and sorted after
// the canonical catalog, and the six canonical packs take their place.
func TestPricingMigrationRetiresLegacyPacks(t *testing.T) {
	admin, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := admin.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	name := "staryield_mig26_" + uuid.NewString()[:8]
	if err := admin.Exec("CREATE DATABASE " + name).Error; err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() { admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)") })
	u, _ := url.Parse(testDSN)
	u.Path = "/" + name
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	sqlConn, _ := db.DB()
	t.Cleanup(func() { _ = sqlConn.Close() })

	all, err := migrations.Load()
	if err != nil {
		t.Fatal(err)
	}
	var pricing string
	for _, m := range all {
		if m.Version == "0026_pricing_catalog" {
			pricing = m.SQL
			continue
		}
		if m.Version < "0026" {
			if err := db.Exec(m.SQL).Error; err != nil {
				t.Fatalf("migration %s: %v", m.Version, err)
			}
		}
	}
	if pricing == "" {
		t.Fatal("0026_pricing_catalog is not registered")
	}
	// Three legacy packs, one of them bought.
	// Starter and Seeker share the 8-hex prefix (time-ordered ids created in the
	// same instant do); Devotee does not.
	legacy := map[string]uuid.UUID{"Starter": uuid.MustParse("019b1234-0000-7000-8000-" + "00000000aaaa"),
		"Seeker": uuid.MustParse("019b1234-0000-7000-8000-" + "00000000bbbb"), "Devotee": uuid.New()}
	for pname, id := range legacy {
		if err := db.Exec(`INSERT INTO credit_packs (id, name, credits, price_cents, currency, active, featured, sort_order)
			VALUES (?, ?, 250, 999, 'USD', true, ?, 2)`, id, pname, pname == "Seeker").Error; err != nil {
			t.Fatal(err)
		}
	}
	user := uuid.New()
	if err := db.Exec(`INSERT INTO users (id, email, password_hash, role, status, display_name, locale, terms_accepted_at)
		VALUES (?, 'legacy@example.com', 'x', 'client', 'active', 'L', 'en', now())`, user).Error; err != nil {
		t.Skipf("cannot seed a legacy purchase with this schema: %v", err)
	}
	if err := db.Exec(`INSERT INTO credit_purchases (id, user_id, pack_id, credits, price_cents, currency, status, provider, idempotency_key)
		VALUES (?, ?, ?, 250, 999, 'USD', 'succeeded', 'sandbox', 'k1')`, uuid.New(), user, legacy["Seeker"]).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Exec(pricing).Error; err != nil {
		t.Fatalf("0026: %v", err)
	}
	var rows []struct {
		ID                      uuid.UUID
		Code, Name, NameEN      string
		Active, Featured, Intro bool
		SortOrder               int
	}
	if err := db.Raw(`SELECT id, code, name, name_l10n ->> 'en' AS name_en, active, featured, kind = 'intro' AS intro, sort_order
		FROM credit_packs ORDER BY sort_order, code`).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 9 {
		t.Fatalf("3 legacy + 6 canonical packs, got %d: %+v", len(rows), rows)
	}
	for i, want := range strings.Split(canonicalOrder, ",") {
		if rows[i].Code != want || !rows[i].Active {
			t.Fatalf("canonical pack %d: %+v", i, rows[i])
		}
	}
	for _, r := range rows[6:] {
		if r.Active || r.Featured || r.SortOrder < 1000 || r.NameEN != r.Name ||
			r.Code != wantLegacyCode(r.ID, legacy) {
			t.Fatalf("legacy pack: %+v", r)
		}
	}
	// The constraints hold: one active intro, one reference, well-formed codes.
	for sql, what := range map[string]string{
		`INSERT INTO credit_packs (id, code, kind, name, name_l10n, credits, price_cents) VALUES (gen_random_uuid(), 'second_intro', 'intro', 'x', '{"en":"x"}', 10, 100)`:                  "a second active intro pack",
		`INSERT INTO credit_packs (id, code, name, name_l10n, credits, price_cents, reference) VALUES (gen_random_uuid(), 'second_ref', 'x', '{"en":"x"}', 10, 100, true)`:                  "a second reference pack",
		`INSERT INTO credit_packs (id, code, name, name_l10n, credits, price_cents) VALUES (gen_random_uuid(), 'Bad Code', 'x', '{"en":"x"}', 10, 100)`:                                     "a malformed code",
		`INSERT INTO credit_packs (id, code, name, name_l10n, credits, price_cents) VALUES (gen_random_uuid(), 'quick_150', 'x', '{"en":"x"}', 10, 100)`:                                    "a duplicate code",
		`INSERT INTO credit_packs (id, code, name, name_l10n, credits, price_cents, active, reference) VALUES (gen_random_uuid(), 'inactive_ref', 'x', '{"en":"x"}', 10, 100, false, true)`: "an inactive reference",
	} {
		if err := db.Exec(sql).Error; err == nil {
			t.Errorf("the database accepted %s", what)
		}
	}
	// offer_redemptions: one active per user, one per card; released ones do not count.
	insert := func(purchase uuid.UUID, user uuid.UUID, card, status string) error {
		return db.Exec(`INSERT INTO offer_redemptions (id, user_id, pack_id, purchase_id, provider, card_key, status, redeemed_at, released_at)
			VALUES (gen_random_uuid(), ?, ?, ?, 'sandbox_a', ?, ?, CASE WHEN ? = 'redeemed' THEN now() END, CASE WHEN ? = 'released' THEN now() END)`,
			user, rows[0].ID, purchase, card, status, status, status).Error
	}
	mkPurchase := func(key string) uuid.UUID {
		id := uuid.New()
		if err := db.Exec(`INSERT INTO credit_purchases (id, user_id, pack_id, credits, price_cents, currency, status, provider, idempotency_key)
			VALUES (?, ?, ?, 600, 1000, 'USD', 'pending', 'sandbox', ?)`, id, user, rows[0].ID, key).Error; err != nil {
			t.Fatal(err)
		}
		return id
	}
	if err := insert(mkPurchase("a"), user, "card1", "released"); err != nil {
		t.Fatal(err)
	}
	if err := insert(mkPurchase("b"), user, "card1", "reserved"); err != nil {
		t.Fatalf("a released offer does not block: %v", err)
	}
	if err := insert(mkPurchase("c"), user, "card2", "reserved"); err == nil {
		t.Error("two active redemptions for one user")
	}
	if err := insert(mkPurchase("d"), user, "card1", "redeemed"); err == nil {
		t.Error("two active redemptions for one card")
	}
}

// wantLegacyCode is legacy_<first 8 hex of the id>, or the last 8 hex when two
// legacy ids share their prefix.
func wantLegacyCode(id uuid.UUID, all map[string]uuid.UUID) string {
	hex := strings.ReplaceAll(id.String(), "-", "")
	n := 0
	for _, other := range all {
		if strings.ReplaceAll(other.String(), "-", "")[:8] == hex[:8] {
			n++
		}
	}
	if n > 1 {
		return "legacy_" + hex[24:]
	}
	return "legacy_" + hex[:8]
}
