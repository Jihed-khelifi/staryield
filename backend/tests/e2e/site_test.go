package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 3.0: site settings (§15): promo banners, the sign-up offer and
// free mode. The database is shared by the whole suite, so every test starts
// from (and restores) the migration's seed state.

const seedBannerID = "5a1f0c3e-6b2d-4f7a-9c1e-3d8b7a4e2f10"

// resetSite puts the site configuration back to the migration's seed state.
func (e *env) resetSite() {
	e.t.Helper()
	reset := func() {
		db := e.app.DB
		must := func(err error) {
			if err != nil {
				e.t.Errorf("reset site settings: %v", err)
			}
		}
		must(db.Exec("DELETE FROM site_banners WHERE id <> ?", seedBannerID).Error)
		must(db.Exec(`UPDATE site_banners SET active = true, audience = 'anonymous', action_type = 'link', action_href = '/signup',
			starts_at = NULL, ends_at = NULL, sort_order = 0,
			text = jsonb_build_object('en', '{free_minutes} MINUTES FREE + 80% OFF FOR NEW CUSTOMERS',
				'fr', '{free_minutes} MINUTES OFFERTES + 80 % DE RÉDUCTION POUR LES NOUVEAUX CLIENTS',
				'es', '{free_minutes} MINUTOS GRATIS + 80 % DE DESCUENTO PARA NUEVOS CLIENTES'),
			cta_label = jsonb_build_object('en', 'CLAIM INTRO OFFER →', 'fr', 'PROFITEZ DE L''OFFRE DE BIENVENUE →',
				'es', 'APROVECHA LA OFERTA DE BIENVENIDA →')
			WHERE id = ?`, seedBannerID).Error)
		must(db.Exec(`UPDATE site_settings SET value = '{"enabled": true, "rotation_seconds": 6}', version = version + 1 WHERE key = 'banners'`).Error)
		must(db.Exec(`UPDATE site_settings SET value = '{"enabled": true, "free_minutes": 5}', version = version + 1 WHERE key = 'signup_offer'`).Error)
		must(db.Exec(`UPDATE site_settings SET value = '{"enabled": false, "reason": ""}', version = version + 1 WHERE key = 'free_mode'`).Error)
	}
	reset()
	e.t.Cleanup(reset)
}

// pinSignupFreeMinutes sets the sign-up offer for the test (restored at its
// end). Tests written for the former 3-minute offer use it.
func (e *env) pinSignupFreeMinutes(n int) {
	e.t.Helper()
	var before string
	if err := e.app.DB.Raw(`SELECT value::text FROM site_settings WHERE key = 'signup_offer'`).Scan(&before).Error; err != nil {
		e.t.Fatal(err)
	}
	set := func(v string) {
		if err := e.app.DB.Exec(`UPDATE site_settings SET value = ?::jsonb, version = version + 1 WHERE key = 'signup_offer'`, v).Error; err != nil {
			e.t.Errorf("pin signup offer: %v", err)
		}
	}
	set(fmt.Sprintf(`{"enabled": true, "free_minutes": %d}`, n))
	e.t.Cleanup(func() { set(before) })
}

// ---- types mirroring the contract -----------------------------------------

type siteConfig struct {
	Banners struct {
		RotationSeconds int          `json:"rotation_seconds"`
		Items           []siteBanner `json:"items"`
	} `json:"banners"`
	SignupOffer struct {
		Enabled     bool `json:"enabled"`
		FreeMinutes int  `json:"free_minutes"`
	} `json:"signup_offer"`
	FreeMode struct {
		Active bool    `json:"active"`
		EndsAt *string `json:"ends_at"`
	} `json:"free_mode"`
	GeneratedAt string `json:"generated_at"`
}

type siteBanner struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	CTALabel string `json:"cta_label"`
	Action   struct {
		Type string `json:"type"`
		Href string `json:"href"`
	} `json:"action"`
	Audience string `json:"audience"`
}

func (e *env) siteConfig(locale string) (siteConfig, response) {
	e.t.Helper()
	c := e.client()
	c.origin = ""
	c.locale = locale
	r := c.raw(http.MethodGet, "/api/v1/site/config", nil, nil)
	if r.status != http.StatusOK {
		e.t.Fatalf("GET /site/config: %d %s", r.status, r.body)
	}
	var out siteConfig
	r.decode(e.t, &out)
	return out, r
}

func bannerBody(text string) map[string]any {
	return map[string]any{"text": map[string]any{"en": text}, "action": map[string]any{"type": "none"}}
}

func (e *env) adminAudit(actor uuid.UUID, action string) []domain.AuditLog {
	var rows []domain.AuditLog
	if err := e.app.DB.Where("action = ? AND actor_id = ?", action, actor).Order("created_at").Find(&rows).Error; err != nil {
		e.t.Fatal(err)
	}
	return rows
}

func fieldMap(t *testing.T, r response) map[string]string {
	t.Helper()
	var body struct {
		Error struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	r.decode(t, &body)
	out := map[string]string{}
	if f, ok := body.Error.Details["fields"].(map[string]any); ok {
		for k, v := range f {
			out[k], _ = v.(string)
		}
	}
	return out
}

// ---- public configuration -------------------------------------------------

func TestSiteConfigPublicShapeLocalesAndETag(t *testing.T) {
	e := newEnv(t)
	e.resetSite()

	cfg, r := e.siteConfig("")
	if cfg.SignupOffer.FreeMinutes != 5 || !cfg.SignupOffer.Enabled || cfg.FreeMode.Active || cfg.Banners.RotationSeconds != 6 {
		t.Fatalf("seed configuration: %+v", cfg)
	}
	if len(cfg.Banners.Items) != 1 {
		t.Fatalf("the seed banner is missing: %s", r.body)
	}
	b := cfg.Banners.Items[0]
	if b.ID != seedBannerID || b.Text != "5 MINUTES FREE + 80% OFF FOR NEW CUSTOMERS" || b.CTALabel != "CLAIM INTRO OFFER →" ||
		b.Action.Type != "link" || b.Action.Href != "/signup" || b.Audience != "anonymous" {
		t.Fatalf("seed banner: %+v", b)
	}
	if _, err := time.Parse(time.RFC3339Nano, cfg.GeneratedAt); err != nil {
		t.Fatalf("generated_at: %v", err)
	}
	if strings.Contains(string(r.body), "{free_minutes}") {
		t.Fatal("the placeholder must be replaced")
	}
	// Caching headers.
	if cc := r.header.Get("Cache-Control"); cc != "public, max-age=30, stale-while-revalidate=60" {
		t.Fatalf("Cache-Control: %q", cc)
	}
	vary := strings.Join(r.header.Values("Vary"), ",")
	if !strings.Contains(vary, "Accept-Language") || !strings.Contains(vary, "X-Locale") {
		t.Fatalf("Vary: %q", vary)
	}
	etag := r.header.Get("ETag")
	if !strings.HasPrefix(etag, `W/"`) {
		t.Fatalf("ETag: %q", etag)
	}
	if r.header.Get("RateLimit-Policy") == "" {
		t.Fatal("the site rate-limit bucket is not applied")
	}

	// Conditional request: 304 without a body, same validators.
	c := e.client()
	c.origin = ""
	nm := c.raw(http.MethodGet, "/api/v1/site/config", nil, map[string]string{"If-None-Match": etag})
	if nm.status != http.StatusNotModified || len(nm.body) != 0 || nm.header.Get("ETag") != etag ||
		nm.header.Get("Cache-Control") == "" {
		t.Fatalf("revalidation: %d %q %v", nm.status, nm.body, nm.header)
	}
	if st := c.raw(http.MethodGet, "/api/v1/site/config", nil, map[string]string{"If-None-Match": `W/"other"`}).status; st != http.StatusOK {
		t.Fatalf("other validator: %d", st)
	}
	if st := c.raw(http.MethodGet, "/api/v1/site/config", nil, map[string]string{"If-None-Match": `W/"zzz", ` + etag}).status; st != http.StatusNotModified {
		t.Fatalf("validator list: %d", st)
	}

	// Locales: X-Locale, Accept-Language, ?locale=; unknown falls back to English.
	fr, frResp := e.siteConfig("fr")
	if got := fr.Banners.Items[0]; got.Text != "5 MINUTES OFFERTES + 80 % DE RÉDUCTION POUR LES NOUVEAUX CLIENTS" || got.CTALabel != "PROFITEZ DE L'OFFRE DE BIENVENUE →" {
		t.Fatalf("fr: %+v", got)
	}
	if frResp.header.Get("ETag") == etag {
		t.Fatal("languages share a validator")
	}
	if es, _ := e.siteConfig("es"); es.Banners.Items[0].Text != "5 MINUTOS GRATIS + 80 % DE DESCUENTO PARA NUEVOS CLIENTES" {
		t.Fatalf("es: %+v", es.Banners.Items[0])
	}
	if de, _ := e.siteConfig("de"); de.Banners.Items[0].Text != b.Text {
		t.Fatalf("de falls back to en: %+v", de.Banners.Items[0])
	}
	q := e.client()
	q.origin = ""
	var viaQuery siteConfig
	q.must(http.StatusOK, http.MethodGet, "/api/v1/site/config?locale=es", nil, &viaQuery)
	if !strings.Contains(viaQuery.Banners.Items[0].Text, "MINUTOS GRATIS") {
		t.Fatalf("?locale=es: %+v", viaQuery.Banners.Items[0])
	}
	var viaHeader siteConfig
	q.raw(http.MethodGet, "/api/v1/site/config", nil, map[string]string{"Accept-Language": "fr-CA,fr;q=0.9"}).decode(t, &viaHeader)
	if !strings.Contains(viaHeader.Banners.Items[0].Text, "MINUTES OFFERTES") {
		t.Fatalf("Accept-Language: %+v", viaHeader.Banners.Items[0])
	}

	// No authentication and no CSRF are needed, also from a cross-site Origin
	// with no cookies, and the response is never personalised.
	for _, c := range []*client{e.client(), e.registerClient(), e.adminClient()} {
		if r := c.raw(http.MethodGet, "/api/v1/site/config", nil, nil); r.status != http.StatusOK || r.header.Get("Set-Cookie") != "" {
			t.Fatalf("anonymous/signed-in/admin origin: %d", r.status)
		}
	}
	// The endpoint is read-only.
	if st := e.client().call(http.MethodPost, "/api/v1/site/config", map[string]any{}).status; st != http.StatusMethodNotAllowed && st != http.StatusNotFound {
		t.Fatalf("POST /site/config: %d", st)
	}
}

func TestSiteConfigRateLimitBucketPerIP(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.RateLimit.Scale = 1 })
	e.resetSite()
	c := e.client()
	c.origin = ""
	limited := 0
	for i := 0; i < 700; i++ {
		switch r := c.raw(http.MethodGet, "/api/v1/site/config", nil, nil); r.status {
		case http.StatusOK:
		case http.StatusTooManyRequests:
			limited++
			if r.errCode(t) != "rate_limited" || r.header.Get("Retry-After") == "" {
				t.Fatalf("429 shape: %s %v", r.body, r.header)
			}
		default:
			t.Fatalf("request %d: %d %s", i, r.status, r.body)
		}
	}
	if limited == 0 {
		t.Fatal("the burst of 600 per IP was never exceeded")
	}
}

func TestSiteConfigSurvivesRedisOutage(t *testing.T) {
	e := newEnv(t)
	e.resetSite()
	if _, r := e.siteConfig(""); r.status != http.StatusOK {
		t.Fatal("warm-up")
	}
	// Redis gone (cache and limiter): the public endpoint fails open and
	// serves from the in-process copy or the database, never an error.
	_ = e.app.Redis.Close()
	for i := 0; i < 3; i++ {
		cfg, _ := e.siteConfig("")
		if len(cfg.Banners.Items) != 1 || cfg.SignupOffer.FreeMinutes != 5 {
			t.Fatalf("outage: %+v", cfg)
		}
	}
}

// ---- admin: banners -------------------------------------------------------

func TestAdminBannersRequireAdminAndStepUp(t *testing.T) {
	e := newEnv(t)
	e.resetSite()
	admin := e.createAdmin()

	// Reads need an admin session only.
	var list service.AdminBanners
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/site/banners", nil, &list)
	if len(list.Items) != 1 || list.Items[0].ID.String() != seedBannerID || list.Items[0].Status != "live" || !list.Settings.Enabled ||
		list.Settings.RotationSeconds != 6 || list.Items[0].Text["fr"] == "" || list.Items[0].Text["en"] != "{free_minutes} MINUTES FREE + 80% OFF FOR NEW CUSTOMERS" {
		t.Fatalf("admin list: %+v", list)
	}

	// Every write needs a fresh step-up: 403 step_up_required without it.
	id := seedBannerID
	writes := []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/api/v1/admin/site/banners", bannerBody("Hello")},
		{http.MethodPatch, "/api/v1/admin/site/banners/" + id, map[string]any{"active": false}},
		{http.MethodDelete, "/api/v1/admin/site/banners/" + id, nil},
		{http.MethodPut, "/api/v1/admin/site/banners/order", map[string]any{"ids": []string{id}}},
		{http.MethodPatch, "/api/v1/admin/site/banners/settings", map[string]any{"enabled": false}},
		{http.MethodPut, "/api/v1/admin/site/signup-offer", map[string]any{"enabled": true, "free_minutes": 9}},
		{http.MethodPut, "/api/v1/admin/site/free-mode", map[string]any{"enabled": false}},
	}
	// The sign-in itself counts as a recent TOTP; let the 10-minute window pass.
	e.clock.Advance(11 * time.Minute)
	for _, w := range writes {
		r := admin.call(w.method, w.path, w.body)
		if r.status != http.StatusForbidden || r.errCode(t) != "step_up_required" {
			t.Errorf("%s %s without step-up: %d %s", w.method, w.path, r.status, r.body)
		}
	}
	for _, p := range []string{"/api/v1/admin/site/banners", "/api/v1/admin/site/signup-offer", "/api/v1/admin/site/free-mode"} {
		admin.must(http.StatusOK, http.MethodGet, p, nil, nil) // reads stay open
	}
	// Nothing changed.
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/site/banners", nil, &list)
	if len(list.Items) != 1 || list.Items[0].Version != 1 {
		t.Fatalf("a refused write changed state: %+v", list)
	}

	// Non-admins and anonymous callers never reach the routes.
	member := e.registerClient()
	for _, c := range []*client{member, e.client(), e.createPsychic(30)} {
		for _, p := range []string{"/api/v1/admin/site/banners", "/api/v1/admin/site/signup-offer", "/api/v1/admin/site/free-mode"} {
			if st := c.call(http.MethodGet, p, nil).status; st != http.StatusUnauthorized && st != http.StatusForbidden {
				t.Errorf("GET %s as a non-admin: %d", p, st)
			}
		}
		if st := c.call(http.MethodPost, "/api/v1/admin/site/banners", bannerBody("x")).status; st != http.StatusUnauthorized && st != http.StatusForbidden {
			t.Errorf("POST as a non-admin: %d", st)
		}
	}
	// A client session replayed on the admin origin is not an admin session.
	member.origin = adminOrigin
	if st := member.call(http.MethodGet, "/api/v1/admin/site/banners", nil).status; st != http.StatusUnauthorized && st != http.StatusForbidden {
		t.Errorf("client cookies on the admin origin: %d", st)
	}

	// After the step-up the same writes succeed.
	admin.stepUp()
	var created service.AdminBanner
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/site/banners", bannerBody("Hello"), &created)
	if created.Version != 1 || created.SortOrder != 1 || created.Status != "live" || created.UpdatedBy == nil || created.UpdatedBy.ID != admin.adminID {
		t.Fatalf("created: %+v", created)
	}
}

func TestAdminBannerCRUDAuditAndVersions(t *testing.T) {
	e := newEnv(t)
	e.resetSite()
	admin := e.createAdmin()
	admin.stepUp()

	var b service.AdminBanner
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/site/banners", map[string]any{
		"text":      map[string]any{"en": "  Spring offer  ", "fr": "Offre de printemps"},
		"cta_label": map[string]any{"en": "Go", "fr": "Allez"},
		"action":    map[string]any{"type": "link", "href": "/psychics?sort=rating"},
		"audience":  "signed_in",
	}, &b)
	if b.Text["en"] != "Spring offer" || b.Action.Href != "/psychics?sort=rating" || b.Audience != "signed_in" || !b.Active || b.Version != 1 {
		t.Fatalf("created: %+v", b)
	}
	// The public site shows it, after the seed banner, with its audience.
	cfg, _ := e.siteConfig("fr")
	if len(cfg.Banners.Items) != 2 || cfg.Banners.Items[1].Text != "Offre de printemps" || cfg.Banners.Items[1].CTALabel != "Allez" ||
		cfg.Banners.Items[1].Audience != "signed_in" || cfg.Banners.Items[1].Action.Href != "/psychics?sort=rating" {
		t.Fatalf("public: %+v", cfg.Banners.Items)
	}

	// Partial update; the whole text object is replaced.
	var u service.AdminBanner
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/site/banners/"+b.ID.String(), map[string]any{
		"text": map[string]any{"en": "Summer offer"}, "audience": "all", "expected_version": 1}, &u)
	if u.Version != 2 || u.Text["en"] != "Summer offer" || u.Text["fr"] != "" || u.Audience != "all" || u.Action.Href != "/psychics?sort=rating" {
		t.Fatalf("updated: %+v", u)
	}
	cfg, _ = e.siteConfig("fr")
	if cfg.Banners.Items[1].Text != "Summer offer" {
		t.Fatalf("the public copy follows at once, with the English fallback: %+v", cfg.Banners.Items[1])
	}

	// Stale version: 409 version_conflict with the current version.
	r := admin.call(http.MethodPatch, "/api/v1/admin/site/banners/"+b.ID.String(), map[string]any{"active": false, "expected_version": 1})
	var conflict struct {
		Error struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	r.decode(t, &conflict)
	if r.status != http.StatusConflict || conflict.Error.Code != "version_conflict" || conflict.Error.Details["current_version"] != float64(2) {
		t.Fatalf("conflict: %d %s", r.status, r.body)
	}
	// Unknown ids and malformed bodies.
	if r := admin.call(http.MethodPatch, "/api/v1/admin/site/banners/"+uuid.NewString(), map[string]any{"active": true}); r.status != http.StatusNotFound {
		t.Fatalf("unknown id: %d", r.status)
	}
	if r := admin.call(http.MethodPatch, "/api/v1/admin/site/banners/not-a-uuid", map[string]any{"active": true}); r.status != http.StatusNotFound {
		t.Fatalf("bad id: %d", r.status)
	}
	if r := admin.call(http.MethodPatch, "/api/v1/admin/site/banners/"+b.ID.String(), map[string]any{"colour": "red"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown field: %d %s", r.status, r.body)
	}
	if r := admin.call(http.MethodPost, "/api/v1/admin/site/banners", map[string]any{"text": "plain string", "action": map[string]any{"type": "none"}}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("wrong type: %d %s", r.status, r.body)
	}

	// Delete.
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/site/banners/"+b.ID.String(), nil, nil)
	if r := admin.call(http.MethodDelete, "/api/v1/admin/site/banners/"+b.ID.String(), nil); r.status != http.StatusNotFound {
		t.Fatalf("second delete: %d", r.status)
	}
	if cfg, _ = e.siteConfig("en"); len(cfg.Banners.Items) != 1 {
		t.Fatalf("after delete: %+v", cfg.Banners.Items)
	}

	// Audit: one row per write, by this admin, naming fields and never copy.
	for action, want := range map[string]int{"site_banner.created": 1, "site_banner.updated": 1, "site_banner.deleted": 1} {
		if got := len(e.adminAudit(admin.adminID, action)); got != want {
			t.Errorf("audit %s: %d rows, want %d", action, got, want)
		}
	}
	upd := e.adminAudit(admin.adminID, "site_banner.updated")[0]
	if upd.TargetID != b.ID.String() || upd.Metadata["fields"] != "audience,text" && upd.Metadata["fields"] != "text,audience" {
		t.Fatalf("update audit: %+v", upd)
	}
	for _, row := range e.adminAudit(admin.adminID, "site_banner.created") {
		if strings.Contains(fmt.Sprint(row.Metadata), "Spring offer") {
			t.Fatalf("copy leaked into the audit trail: %v", row.Metadata)
		}
	}
}

func TestAdminBannerValidationFieldKeysAndLocale(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.Links.AllowedHosts = []string{"partner.example.com"} })
	e.resetSite()
	admin := e.createAdmin()
	admin.stepUp()

	bad := map[string]any{
		"text":      map[string]any{"en": "<script>alert(1)</script>", "fr": strings.Repeat("é", 141), "es": "{name}", "de": "Hallo"},
		"cta_label": map[string]any{"en": strings.Repeat("c", 41)},
		"action":    map[string]any{"type": "link", "href": "javascript:alert(1)"},
		"audience":  "everybody",
		"starts_at": e.clock.Now().Add(2 * time.Hour).Format(time.RFC3339),
		"ends_at":   e.clock.Now().Add(time.Hour).Format(time.RFC3339),
	}
	r := admin.call(http.MethodPost, "/api/v1/admin/site/banners", bad)
	if r.status != http.StatusUnprocessableEntity || r.errCode(t) != "validation_failed" {
		t.Fatalf("validation: %d %s", r.status, r.body)
	}
	fields := fieldMap(t, r)
	for _, key := range []string{"text.en", "text.fr", "text.es", "text.de", "cta_label.en", "action.href", "audience", "ends_at"} {
		if fields[key] == "" {
			t.Errorf("missing field key %q in %v", key, fields)
		}
	}
	// Messages follow the console language.
	admin.locale = "fr"
	frFields := fieldMap(t, admin.call(http.MethodPost, "/api/v1/admin/site/banners", bad))
	if frFields["text.fr"] != "Utilisez entre 1 et 140 caractères." || frFields["text.es"] != "Le seul champ variable autorisé est {free_minutes}." {
		t.Fatalf("fr messages: %v", frFields)
	}
	admin.locale = "es"
	if esFields := fieldMap(t, admin.call(http.MethodPost, "/api/v1/admin/site/banners", bad)); esFields["text.fr"] != "Usa entre 1 y 140 caracteres." {
		t.Fatalf("es messages: %v", esFields)
	}
	admin.locale = ""

	// Dangerous destinations are all refused; nothing was stored.
	for _, href := range []string{"javascript:alert(1)", "JAVASCRIPT:alert(1)", "data:text/html,<script>alert(1)</script>", "//evil.example.com/x",
		"http://partner.example.com/", "https://evil.example.com/", "https://partner.example.com.evil.com/", "https://user@partner.example.com/",
		"vbscript:x", "/\\evil.example.com", "/en/signup", "signup", ""} {
		body := bannerBody("Hello")
		body["action"] = map[string]any{"type": "link", "href": href}
		if r := admin.call(http.MethodPost, "/api/v1/admin/site/banners", body); r.status != http.StatusUnprocessableEntity || fieldMap(t, r)["action.href"] == "" {
			t.Errorf("href %q: %d %s", href, r.status, r.body)
		}
	}
	for _, href := range []string{"/signup", "/psychics?sort=rating#top", "https://partner.example.com/promo"} {
		body := bannerBody("Hello " + href)
		body["action"] = map[string]any{"type": "link", "href": href}
		if r := admin.call(http.MethodPost, "/api/v1/admin/site/banners", body); r.status != http.StatusCreated {
			t.Errorf("href %q refused: %d %s", href, r.status, r.body)
		}
	}
	// Script-looking text is refused rather than stored; invisible characters are stripped.
	for _, text := range []string{"<img src=x onerror=alert(1)>", "Hello <b>there</b>", "{{7*7}}", "{free_minutes} {price}"} {
		if r := admin.call(http.MethodPost, "/api/v1/admin/site/banners", bannerBody(text)); r.status != http.StatusUnprocessableEntity {
			t.Errorf("text %q: %d", text, r.status)
		}
	}
	var b service.AdminBanner
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/site/banners", bannerBody("Cle‮an​ text"), &b)
	if b.Text["en"] != "Clean text" {
		t.Fatalf("stripped text: %q", b.Text["en"])
	}
	// A link needs a destination, "none" never keeps one.
	body := bannerBody("No destination")
	body["action"] = map[string]any{"type": "link"}
	if r := admin.call(http.MethodPost, "/api/v1/admin/site/banners", body); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("link without href: %d", r.status)
	}
	body["action"] = map[string]any{"type": "none", "href": "https://partner.example.com/"}
	var none service.AdminBanner
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/site/banners", body, &none)
	if none.Action.Type != "none" || none.Action.Href != "" {
		t.Fatalf("none action: %+v", none.Action)
	}
	// Only the three seeded languages exist; unknown placeholders never reach the DB.
	var n int64
	e.app.DB.Raw(`SELECT count(*) FROM site_banners WHERE text::text LIKE '%<%' OR text::text LIKE '%{name}%'`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d unsafe banners stored", n)
	}
}

func TestAdminBannerLimitReorderAndScheduling(t *testing.T) {
	e := newEnv(t)
	e.resetSite()
	admin := e.createAdmin()
	admin.stepUp()

	ids := []string{seedBannerID}
	for i := 1; i < domain.MaxSiteBanners; i++ {
		var b service.AdminBanner
		admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/site/banners", bannerBody(fmt.Sprintf("Banner %d", i)), &b)
		ids = append(ids, b.ID.String())
	}
	r := admin.call(http.MethodPost, "/api/v1/admin/site/banners", bannerBody("The eleventh"))
	if r.status != http.StatusConflict || r.errCode(t) != "limit_reached" {
		t.Fatalf("11th banner: %d %s", r.status, r.body)
	}
	if cfg, _ := e.siteConfig("en"); len(cfg.Banners.Items) != 10 {
		t.Fatalf("public items: %d", len(cfg.Banners.Items))
	}

	// Reorder: every id exactly once, in the new order.
	rev := make([]string, len(ids))
	for i, id := range ids {
		rev[len(ids)-1-i] = id
	}
	var out struct {
		Items []service.AdminBanner `json:"items"`
	}
	admin.must(http.StatusOK, http.MethodPut, "/api/v1/admin/site/banners/order", map[string]any{"ids": rev}, &out)
	for i, item := range out.Items {
		if item.ID.String() != rev[i] || item.SortOrder != i {
			t.Fatalf("order[%d] = %s/%d", i, item.ID, item.SortOrder)
		}
	}
	if cfg, _ := e.siteConfig("en"); cfg.Banners.Items[0].ID != rev[0] || cfg.Banners.Items[9].ID != rev[9] {
		t.Fatalf("public order follows: %+v", cfg.Banners.Items)
	}
	for name, body := range map[string]any{
		"missing":   map[string]any{"ids": ids[:9]},
		"duplicate": map[string]any{"ids": append(append([]string{}, ids[:9]...), ids[0])},
		"unknown":   map[string]any{"ids": append(append([]string{}, ids[:9]...), uuid.NewString())},
		"extra":     map[string]any{"ids": append(append([]string{}, ids...), uuid.NewString())},
		"empty":     map[string]any{"ids": []string{}},
		"not uuids": map[string]any{"ids": []string{"a", "b"}},
		"no ids":    map[string]any{},
	} {
		r := admin.call(http.MethodPut, "/api/v1/admin/site/banners/order", body)
		if r.status != http.StatusUnprocessableEntity || fieldMap(t, r)["ids"] == "" {
			t.Errorf("reorder %s: %d %s", name, r.status, r.body)
		}
	}

	// Scheduling: the status is computed and only live banners are public.
	now := e.clock.Now()
	var sched, ended service.AdminBanner
	body := bannerBody("Starts later")
	body["starts_at"] = now.Add(20 * time.Minute).Format(time.RFC3339)
	// Make room: delete two banners.
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/site/banners/"+ids[1], nil, nil)
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/site/banners/"+ids[2], nil, nil)
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/site/banners", body, &sched)
	body = bannerBody("Already over")
	body["starts_at"] = now.Add(-2 * time.Hour).Format(time.RFC3339)
	body["ends_at"] = now.Add(-time.Hour).Format(time.RFC3339)
	admin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/site/banners", body, &ended)
	if sched.Status != "scheduled" || ended.Status != "expired" {
		t.Fatalf("statuses: %s / %s", sched.Status, ended.Status)
	}
	public := func() map[string]bool {
		cfg, _ := e.siteConfig("en")
		m := map[string]bool{}
		for _, it := range cfg.Banners.Items {
			m[it.ID] = true
		}
		return m
	}
	if p := public(); p[sched.ID.String()] || p[ended.ID.String()] {
		t.Fatal("a scheduled or expired banner is public")
	}
	var list service.AdminBanners
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/site/banners", nil, &list)
	statuses := map[string]string{}
	for _, it := range list.Items {
		statuses[it.ID.String()] = it.Status
	}
	if statuses[sched.ID.String()] != "scheduled" || statuses[ended.ID.String()] != "expired" || len(list.Items) != 10 {
		t.Fatalf("admin list keeps every banner: %v", statuses)
	}
	// The schedule is applied to the request time without any job: after the
	// start the banner is public (the cached copy only holds the rows).
	e.clock.Advance(21 * time.Minute)
	if p := public(); !p[sched.ID.String()] {
		t.Fatal("the scheduled banner did not go live")
	}
	// Deactivating hides it; the schedule can be cleared with null.
	admin.stepUp()
	var off service.AdminBanner
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/site/banners/"+sched.ID.String(), map[string]any{"active": false}, &off)
	if off.Status != "inactive" || public()[sched.ID.String()] {
		t.Fatalf("inactive: %+v", off)
	}
	var cleared service.AdminBanner
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/site/banners/"+ended.ID.String(), map[string]any{"ends_at": nil, "starts_at": nil}, &cleared)
	if cleared.Status != "live" || cleared.EndsAt != nil || !public()[ended.ID.String()] {
		t.Fatalf("cleared schedule: %+v", cleared)
	}
	if r := admin.call(http.MethodPatch, "/api/v1/admin/site/banners/"+ended.ID.String(), map[string]any{
		"starts_at": e.clock.Now().Add(time.Hour).Format(time.RFC3339), "ends_at": e.clock.Now().Format(time.RFC3339)}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("end before start: %d %s", r.status, r.body)
	}
	if got := len(e.adminAudit(admin.adminID, "site_banner.reordered")); got != 1 {
		t.Fatalf("reorder audit rows: %d", got)
	}
}

func TestAdminBannerSettingsHideAllAndRotation(t *testing.T) {
	e := newEnv(t)
	e.resetSite()
	admin := e.createAdmin()
	admin.stepUp()

	var s service.BannerSettings
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/site/banners/settings", map[string]any{"rotation_seconds": 12}, &s)
	if !s.Enabled || s.RotationSeconds != 12 {
		t.Fatalf("rotation: %+v", s)
	}
	if cfg, _ := e.siteConfig("en"); cfg.Banners.RotationSeconds != 12 {
		t.Fatalf("public rotation: %d", cfg.Banners.RotationSeconds)
	}
	for _, n := range []int{3, 31, 0, -1} {
		if r := admin.call(http.MethodPatch, "/api/v1/admin/site/banners/settings", map[string]any{"rotation_seconds": n}); r.status != http.StatusUnprocessableEntity ||
			fieldMap(t, r)["rotation_seconds"] == "" {
			t.Errorf("rotation %d: %d", n, r.status)
		}
	}
	if r := admin.call(http.MethodPatch, "/api/v1/admin/site/banners/settings", map[string]any{"enabled": false, "expected_version": s.Version + 3}); r.status != http.StatusConflict {
		t.Fatalf("stale settings version: %d", r.status)
	}
	// Hiding the bar: items is an empty list, not null and not missing.
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/site/banners/settings", map[string]any{"enabled": false, "expected_version": s.Version}, &s)
	cfg, r := e.siteConfig("en")
	if len(cfg.Banners.Items) != 0 || !strings.Contains(string(r.body), `"items":[]`) {
		t.Fatalf("hidden bar: %s", r.body)
	}
	// The banners stay for the admin; switching back restores the bar.
	var list service.AdminBanners
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/site/banners", nil, &list)
	if len(list.Items) != 1 || list.Settings.Enabled {
		t.Fatalf("admin list while hidden: %+v", list)
	}
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/site/banners/settings", map[string]any{"enabled": true}, &s)
	if cfg, _ = e.siteConfig("en"); len(cfg.Banners.Items) != 1 {
		t.Fatalf("restored: %+v", cfg.Banners.Items)
	}
	if got := len(e.adminAudit(admin.adminID, "site_banner.settings_updated")); got != 3 {
		t.Fatalf("settings audit rows: %d", got)
	}
}

// ---- sign-up offer --------------------------------------------------------

func TestSignupOfferControlsNewWalletsOnly(t *testing.T) {
	e := newEnv(t)
	e.resetSite()
	admin := e.createAdmin()

	first := e.registerClient()
	if w := first.wallet(); w.FreeMinutes != 5 {
		t.Fatalf("seed offer: %+v", w)
	}
	// The registration event records the minutes granted.
	waitSignupEvent := func(userID uuid.UUID, want int) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			var props string
			e.app.DB.Raw(`SELECT properties::text FROM analytics_events WHERE name = 'user.registered' AND user_id = ? LIMIT 1`, userID).Scan(&props)
			if strings.Contains(props, fmt.Sprintf(`"signup_free_minutes": %d`, want)) || strings.Contains(props, fmt.Sprintf(`"signup_free_minutes":%d`, want)) {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("user.registered for %s lacks signup_free_minutes=%d (got %q)", userID, want, props)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	waitSignupEvent(first.user.ID, 5)

	// Without a step-up the offer cannot change (the sign-in's window has passed).
	e.clock.Advance(11 * time.Minute)
	r := admin.call(http.MethodPut, "/api/v1/admin/site/signup-offer", map[string]any{"enabled": true, "free_minutes": 7})
	if r.status != http.StatusForbidden || r.errCode(t) != "step_up_required" {
		t.Fatalf("no step-up: %d %s", r.status, r.body)
	}
	admin.stepUp()
	var offer service.SignupOffer
	admin.must(http.StatusOK, http.MethodPut, "/api/v1/admin/site/signup-offer", map[string]any{"enabled": true, "free_minutes": 7}, &offer)
	if offer.FreeMinutes != 7 || !offer.Enabled || offer.Version < 2 || offer.UpdatedBy == nil || offer.UpdatedBy.ID != admin.adminID {
		t.Fatalf("offer: %+v", offer)
	}
	second := e.registerClient()
	if w := second.wallet(); w.FreeMinutes != 7 {
		t.Fatalf("after the change: %+v", w)
	}
	waitSignupEvent(second.user.ID, 7)
	if w := first.wallet(); w.FreeMinutes != 5 {
		t.Fatalf("existing wallets must not change: %+v", w)
	}
	if cfg, _ := e.siteConfig("en"); cfg.SignupOffer.FreeMinutes != 7 || cfg.Banners.Items[0].Text != "7 MINUTES FREE + 80% OFF FOR NEW CUSTOMERS" {
		t.Fatalf("public offer and banner copy follow: %+v", cfg)
	}

	// Disabled: new accounts get nothing, the public config reports 0, and
	// the banner that promises free minutes is not shown.
	admin.must(http.StatusOK, http.MethodPut, "/api/v1/admin/site/signup-offer", map[string]any{"enabled": false, "free_minutes": 7, "expected_version": offer.Version}, &offer)
	third := e.registerClient()
	if w := third.wallet(); w.FreeMinutes != 0 {
		t.Fatalf("disabled offer: %+v", w)
	}
	waitSignupEvent(third.user.ID, 0)
	cfg, _ := e.siteConfig("en")
	if cfg.SignupOffer.Enabled || cfg.SignupOffer.FreeMinutes != 0 || len(cfg.Banners.Items) != 0 {
		t.Fatalf("public config while disabled: %+v", cfg)
	}
	var stored service.SignupOffer
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/site/signup-offer", nil, &stored)
	if stored.Enabled || stored.FreeMinutes != 7 {
		t.Fatalf("the stored minutes are kept: %+v", stored)
	}
	// Switching back on restores them.
	admin.must(http.StatusOK, http.MethodPut, "/api/v1/admin/site/signup-offer", map[string]any{"enabled": true, "free_minutes": 7}, &offer)
	if w := e.registerClient().wallet(); w.FreeMinutes != 7 {
		t.Fatalf("re-enabled: %+v", w)
	}

	// Validation, versions and audit.
	for name, body := range map[string]any{
		"zero":         map[string]any{"enabled": true, "free_minutes": 0},
		"too many":     map[string]any{"enabled": true, "free_minutes": 61},
		"negative":     map[string]any{"enabled": true, "free_minutes": -3},
		"missing":      map[string]any{"enabled": true},
		"no flag":      map[string]any{"free_minutes": 5},
		"wrong type":   map[string]any{"enabled": "yes", "free_minutes": 5},
		"fractional":   map[string]any{"enabled": true, "free_minutes": 2.5},
		"unknown":      map[string]any{"enabled": true, "free_minutes": 5, "bonus": 1},
		"string value": map[string]any{"enabled": true, "free_minutes": "5"},
	} {
		if r := admin.call(http.MethodPut, "/api/v1/admin/site/signup-offer", body); r.status != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %s", name, r.status, r.body)
		}
	}
	if fields := fieldMap(t, admin.call(http.MethodPut, "/api/v1/admin/site/signup-offer", map[string]any{"enabled": true, "free_minutes": 61})); fields["free_minutes"] != "Use between 1 and 60 minutes." {
		t.Fatalf("message: %v", fields)
	}
	for _, minutes := range []int{1, 60} {
		admin.must(http.StatusOK, http.MethodPut, "/api/v1/admin/site/signup-offer", map[string]any{"enabled": true, "free_minutes": minutes}, &offer)
	}
	if r := admin.call(http.MethodPut, "/api/v1/admin/site/signup-offer", map[string]any{"enabled": true, "free_minutes": 5, "expected_version": 1}); r.status != http.StatusConflict {
		t.Fatalf("stale version: %d", r.status)
	}
	rows := e.adminAudit(admin.adminID, "site.signup_offer_updated")
	if len(rows) != 5 {
		t.Fatalf("audit rows: %d", len(rows))
	}
	// The fake clock stamps rows of one instant alike: look the row up by content.
	found := false
	for _, row := range rows {
		before, _ := row.Metadata["before"].(map[string]any)
		after, _ := row.Metadata["after"].(map[string]any)
		if before["free_minutes"] == float64(5) && after["free_minutes"] == float64(7) && after["enabled"] == true {
			found = true
		}
	}
	if !found {
		t.Fatalf("no audit row records the change from 5 to 7 minutes: %v", rows)
	}
}

func TestSignupOfferIsSharedAcrossInstances(t *testing.T) {
	e := newEnv(t)
	e.resetSite()
	// The admin write reaches the in-process copy through the Redis
	// invalidation and the Pub/Sub announcement, not only through the TTL.
	admin := e.createAdmin()
	other := e.registerClient()
	if other.wallet().FreeMinutes != 5 {
		t.Fatal("seed")
	}
	// Warm the in-process copy of this instance, change the offer, and read again at once.
	if cfg, _ := e.siteConfig("en"); cfg.SignupOffer.FreeMinutes != 5 {
		t.Fatal("warm-up")
	}
	admin.stepUp()
	admin.must(http.StatusOK, http.MethodPut, "/api/v1/admin/site/signup-offer", map[string]any{"enabled": true, "free_minutes": 9}, nil)
	if cfg, _ := e.siteConfig("en"); cfg.SignupOffer.FreeMinutes != 9 {
		t.Fatalf("the writing instance serves a stale copy: %+v", cfg.SignupOffer)
	}
	// A message on the channel from "another instance" drops this one's copy
	// even though nothing was written through it.
	if err := e.app.DB.Exec(`UPDATE site_settings SET value = '{"enabled": true, "free_minutes": 11}', version = version + 1 WHERE key = 'signup_offer'`).Error; err != nil {
		t.Fatal(err)
	}
	if cfg, _ := e.siteConfig("en"); cfg.SignupOffer.FreeMinutes != 9 {
		t.Fatalf("expected the cached copy until invalidated: %+v", cfg.SignupOffer)
	}
	bus := cache.NewRedisBus(e.app.Redis, e.app.Config.Redis.KeyPrefix)
	if err := bus.Publish(context.Background(), service.SiteChangedChannel, "changed"); err != nil {
		t.Fatal(err)
	}
	// The Redis snapshot is dropped by writers; emulate that too.
	kv := cache.NewRedis(e.app.Redis, e.app.Config.Redis.KeyPrefix)
	_ = kv.Del(context.Background(), "site:config:v1")
	deadline := time.Now().Add(3 * time.Second)
	for {
		cfg, _ := e.siteConfig("en")
		if cfg.SignupOffer.FreeMinutes == 11 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the announcement did not drop the copy: %+v", cfg.SignupOffer)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// ---- free mode ------------------------------------------------------------

func TestFreeModeGuardsConfirmationAndAudit(t *testing.T) {
	e := newEnv(t)
	e.resetSite()
	admin := e.createAdmin()
	admin.stepUp()
	now := e.clock.Now()
	on := func(mod func(map[string]any)) map[string]any {
		b := map[string]any{"enabled": true, "confirmation": "ENABLE FREE MODE", "reason": "Launch weekend",
			"ends_at": now.Add(24 * time.Hour).Format(time.RFC3339)}
		mod(b)
		return b
	}
	for name, body := range map[string]struct {
		body map[string]any
		key  string
	}{
		"wrong phrase":   {on(func(b map[string]any) { b["confirmation"] = "enable free mode" }), "confirmation"},
		"no phrase":      {on(func(b map[string]any) { delete(b, "confirmation") }), "confirmation"},
		"extra space":    {on(func(b map[string]any) { b["confirmation"] = " ENABLE FREE MODE" }), "confirmation"},
		"no reason":      {on(func(b map[string]any) { delete(b, "reason") }), "reason"},
		"short reason":   {on(func(b map[string]any) { b["reason"] = "four" }), "reason"},
		"long reason":    {on(func(b map[string]any) { b["reason"] = strings.Repeat("r", 501) }), "reason"},
		"past end":       {on(func(b map[string]any) { b["ends_at"] = now.Add(-time.Minute).Format(time.RFC3339) }), "ends_at"},
		"beyond 90 days": {on(func(b map[string]any) { b["ends_at"] = now.Add(91 * 24 * time.Hour).Format(time.RFC3339) }), "ends_at"},
		"bad end":        {on(func(b map[string]any) { b["ends_at"] = "soon" }), "ends_at"},
	} {
		r := admin.call(http.MethodPut, "/api/v1/admin/site/free-mode", body.body)
		if name == "bad end" { // not a timestamp: refused while decoding
			if r.status != http.StatusBadRequest && r.status != http.StatusUnprocessableEntity {
				t.Errorf("bad end: %d %s", r.status, r.body)
			}
			continue
		}
		if r.status != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %s", name, r.status, r.body)
			continue
		}
		if fieldMap(t, r)[body.key] == "" {
			t.Errorf("%s: no %q field in %s", name, body.key, r.body)
		}
	}
	// The phrase message is localized.
	admin.locale = "fr"
	if f := fieldMap(t, admin.call(http.MethodPut, "/api/v1/admin/site/free-mode", on(func(b map[string]any) { b["confirmation"] = "oui" }))); f["confirmation"] != "Saisissez exactement ENABLE FREE MODE pour confirmer." {
		t.Fatalf("fr: %v", f)
	}
	admin.locale = ""
	if cfg, _ := e.siteConfig("en"); cfg.FreeMode.Active {
		t.Fatal("a refused request switched free mode on")
	}
	if got := len(e.adminAudit(admin.adminID, "site.free_mode_enabled")); got != 0 {
		t.Fatalf("refused requests were audited as enabled: %d", got)
	}

	// Accepted: exactly 5 characters and exactly 90 days are fine.
	var st service.FreeModeState
	admin.must(http.StatusOK, http.MethodPut, "/api/v1/admin/site/free-mode", on(func(b map[string]any) {
		b["reason"] = "12345"
		b["ends_at"] = now.Add(90 * 24 * time.Hour).Format(time.RFC3339)
	}), &st)
	if !st.Enabled || !st.Active || st.EnabledAt == nil || st.EnabledBy == nil || st.EnabledBy.ID != admin.adminID || st.EndsAt == nil || st.Reason != "12345" {
		t.Fatalf("enabled: %+v", st)
	}
	cfg, _ := e.siteConfig("en")
	if !cfg.FreeMode.Active || cfg.FreeMode.EndsAt == nil {
		t.Fatalf("public: %+v", cfg.FreeMode)
	}
	// Concurrency token.
	if r := admin.call(http.MethodPut, "/api/v1/admin/site/free-mode", map[string]any{"enabled": false, "expected_version": st.Version - 1}); r.status != http.StatusConflict {
		t.Fatalf("stale version: %d", r.status)
	}
	var off service.FreeModeState
	admin.must(http.StatusOK, http.MethodPut, "/api/v1/admin/site/free-mode", map[string]any{"enabled": false, "reason": "Promotion over", "expected_version": st.Version}, &off)
	if off.Enabled || off.Active || off.DisabledAt == nil || off.DisabledBy == nil || off.Reason != "12345" {
		t.Fatalf("disabled: %+v", off)
	}
	if cfg, _ = e.siteConfig("en"); cfg.FreeMode.Active {
		t.Fatalf("public after disabling: %+v", cfg.FreeMode)
	}
	enabled, disabled := e.adminAudit(admin.adminID, "site.free_mode_enabled"), e.adminAudit(admin.adminID, "site.free_mode_disabled")
	if len(enabled) != 1 || len(disabled) != 1 || enabled[0].Metadata["reason"] != "12345" || disabled[0].Metadata["reason"] != "Promotion over" {
		t.Fatalf("audit: %+v / %+v", enabled, disabled)
	}
	if prev, _ := disabled[0].Metadata["previous"].(map[string]any); prev["enabled"] != true {
		t.Fatalf("audit previous state: %v", disabled[0].Metadata)
	}
}

// freeModeFixture is an online advisor, an admin and a client with no
// credits and no free minutes (the sign-up offer is switched off).
type freeModeFixture struct {
	e       *env
	admin   *client
	psychic *client
	pws     *ws
}

func newFreeModeFixture(t *testing.T) *freeModeFixture {
	t.Helper()
	e := newEnv(t)
	e.resetSite()
	f := &freeModeFixture{e: e, admin: e.createAdmin(), psychic: e.createPsychic(40)}
	f.pws = f.psychic.goOnline()
	return f
}

func (f *freeModeFixture) set(body map[string]any, out any) {
	f.admin.stepUp()
	f.admin.must(http.StatusOK, http.MethodPut, "/api/v1/admin/site/free-mode", body, out)
}

func (f *freeModeFixture) enable(hours int) service.FreeModeState {
	var st service.FreeModeState
	f.set(map[string]any{"enabled": true, "confirmation": "ENABLE FREE MODE", "reason": "Launch weekend",
		"ends_at": f.e.clock.Now().Add(time.Duration(hours) * time.Hour).Format(time.RFC3339)}, &st)
	return st
}

func (f *freeModeFixture) disable() {
	f.set(map[string]any{"enabled": false}, nil)
}

func (f *freeModeFixture) overview() repository.Overview {
	var o repository.Overview
	from, to := f.e.clock.Now().AddDate(0, 0, -7).Format("2006-01-02"), f.e.clock.Now().AddDate(0, 0, 7).Format("2006-01-02")
	f.admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/reports/overview?from="+from+"&to="+to, nil, &o)
	return o
}

// requestSession asks for a chat; it returns the raw response.
func (f *freeModeFixture) request(c *client) response {
	return c.call(http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": f.psychic.slug(), "request_key": uniq("req-key-")})
}

func TestFreeModeSponsoredChatEndToEnd(t *testing.T) {
	f := newFreeModeFixture(t)
	e, admin, psychic := f.e, f.admin, f.psychic
	// New accounts get no free minutes, so this client has no way to pay.
	admin.stepUp()
	admin.must(http.StatusOK, http.MethodPut, "/api/v1/admin/site/signup-offer", map[string]any{"enabled": false, "free_minutes": 5}, nil)
	client := e.registerClient()
	if w := client.wallet(); w.Balance != 0 || w.FreeMinutes != 0 {
		t.Fatalf("wallet: %+v", w)
	}
	cws := client.connect("")

	// Without free mode: no credits, no chat.
	if r := f.request(client); r.status != http.StatusPaymentRequired || r.errCode(t) != "insufficient_credits" {
		t.Fatalf("request without free mode: %d %s", r.status, r.body)
	}
	before := f.overview()

	st := f.enable(24)
	if !st.Active || st.Stats.SponsoredSessions < 0 {
		t.Fatalf("enabled: %+v", st)
	}
	baseline := st.Stats

	var requested service.SessionView
	r := f.request(client)
	if r.status != http.StatusCreated {
		t.Fatalf("request in free mode: %d %s", r.status, r.body)
	}
	r.decode(t, &requested)
	if !requested.Sponsored || requested.SponsoredMinutes != 0 {
		t.Fatalf("requested: %+v", requested)
	}
	sid := requested.ID.String()
	// The same request key stays idempotent.
	pws := f.pws
	pws.waitFor(realtime.EventSessionRequested, 5*time.Second)
	cws.mustAck(realtime.FrameSessionJoin, map[string]any{"session_id": requested.ID})
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+sid+"/accept", nil, nil)
	pws.mustAck(realtime.FrameSessionJoin, map[string]any{"session_id": requested.ID})
	started := sessionFrom(t, cws.waitFor(realtime.EventSessionStarted, 5*time.Second).Data)
	pws.waitFor(realtime.EventSessionStarted, 5*time.Second)
	if !started.Sponsored || started.BilledMinutes != 1 || started.SponsoredMinutes != 1 || started.FreeMinutesUsed != 0 || *started.CreditsCharged != 0 || started.SponsoredCredits != 40 {
		t.Fatalf("started: %+v", started)
	}

	// The first minute: client sees a free, zero-charge, sponsored minute;
	// the advisor sees the list rate and live earnings.
	var cb, pb map[string]any
	_ = json.Unmarshal(cws.waitFor(realtime.EventSessionBilled, 5*time.Second).Data, &cb)
	_ = json.Unmarshal(pws.waitFor(realtime.EventSessionBilled, 5*time.Second).Data, &pb)
	if cb["sponsored"] != true || cb["free"] != true || cb["charged"] != float64(0) || cb["minute"] != float64(1) {
		t.Fatalf("client billed payload: %v", cb)
	}
	if w, _ := cb["wallet"].(map[string]any); w["balance"] != float64(0) || w["free_minutes"] != float64(0) {
		t.Fatalf("client wallet in the billed event: %v", cb["wallet"])
	}
	if pb["sponsored"] != true || pb["charged"] != float64(40) || pb["psychic_earnings"] != float64(24) {
		t.Fatalf("advisor billed payload: %v", pb)
	}

	// Views: conversation open_session, session, admin view and the summary.
	var inbox service.InboxPage
	client.must(http.StatusOK, http.MethodGet, "/api/v1/chat/conversations", nil, &inbox)
	if len(inbox.Items) == 0 || inbox.Items[0].OpenSession == nil || !inbox.Items[0].OpenSession.Sponsored {
		t.Fatalf("open_session: %+v", inbox.Items)
	}
	var adv service.SessionView
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/chat/sessions/"+sid, nil, &adv)
	if !adv.Sponsored || adv.CreditsCharged != nil || adv.PsychicEarnings == nil || *adv.PsychicEarnings != 24 {
		t.Fatalf("advisor session view: %+v", adv)
	}
	var adm service.SessionView
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/chat/sessions/"+sid, nil, &adm)
	if !adm.Sponsored || adm.SponsoredMinutes != 1 {
		t.Fatalf("admin session view: %+v", adm)
	}

	// Minute two, then free mode is switched off while the chat is running.
	e.tick(time.Minute + time.Second)
	cws.waitMatch(realtime.EventSessionBilled, `"minute":2`, 5*time.Second)
	f.disable()
	if cfg, _ := e.siteConfig("en"); cfg.FreeMode.Active {
		t.Fatal("public config still says free mode is active")
	}
	// The running session stays sponsored: minute three is still free, there
	// is no low-balance warning and the chat does not end for lack of credits.
	e.tick(time.Minute)
	cws.waitMatch(realtime.EventSessionBilled, `"minute":3`, 5*time.Second)
	cws.expectNone(realtime.EventSessionLowBalance, 300*time.Millisecond)
	cws.expectNone(realtime.EventSessionEnded, 100*time.Millisecond)
	var live service.SessionView
	client.must(http.StatusOK, http.MethodGet, "/api/v1/chat/sessions/"+sid, nil, &live)
	if live.Status != "active" || !live.Sponsored || live.SponsoredMinutes != 3 || live.BilledMinutes != 3 {
		t.Fatalf("running session after free mode ended: %+v", live)
	}
	// Wallet and ledger untouched.
	if w := client.wallet(); w.Balance != 0 || w.FreeMinutes != 0 {
		t.Fatalf("wallet after sponsored minutes: %+v", w)
	}
	var ledger struct {
		Items []map[string]any `json:"items"`
	}
	client.must(http.StatusOK, http.MethodGet, "/api/v1/wallet/ledger", nil, &ledger)
	for _, it := range ledger.Items {
		if it["entry_type"] == "chat_charge" {
			t.Fatalf("a sponsored minute was charged: %v", it)
		}
	}

	// End the chat: summary for the client, settlement for the advisor.
	var sum service.SessionSummary
	client.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+sid+"/end", nil, &sum)
	if !sum.Session.Sponsored || sum.Session.SponsoredMinutes != 3 || sum.Session.SponsoredCredits != 120 || *sum.Session.CreditsCharged != 0 || sum.Session.EndReason != "client_ended" {
		t.Fatalf("summary: %+v", sum.Session)
	}
	pws.waitFor(realtime.EventSessionSettled, 5*time.Second)
	var earnings service.EarningsReport
	psychic.must(http.StatusOK, http.MethodGet, "/api/v1/psychic/earnings", nil, &earnings)
	if earnings.Summary.GrossCredits != 120 || earnings.Summary.NetCredits != 72 {
		t.Fatalf("the advisor earns 60%% of 3 × 40 at the list rate: %+v", earnings.Summary)
	}
	if w := client.wallet(); w.Balance != 0 || w.FreeMinutes != 0 {
		t.Fatalf("wallet at the end: %+v", w)
	}

	// Free mode is off again: a new request needs credits.
	if r := f.request(client); r.status != http.StatusPaymentRequired || r.errCode(t) != "insufficient_credits" {
		t.Fatalf("request after free mode: %d %s", r.status, r.body)
	}

	// Statistics and the overview (the shared database holds other tests'
	// data, so both are checked as differences).
	// The statistics are cached for 15 s (the response that switched free mode
	// off holds the numbers of that moment): read the cached copy, then drop it.
	var state service.FreeModeState
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/site/free-mode", nil, &state)
	if state.Stats.SponsoredMinutes-baseline.SponsoredMinutes != 2 {
		t.Fatalf("expected the cached statistics of the moment free mode was switched off: %+v", state.Stats)
	}
	kv := cache.NewRedis(e.app.Redis, e.app.Config.Redis.KeyPrefix)
	if err := kv.Del(context.Background(), "site:free-mode:stats:v1"); err != nil {
		t.Fatal(err)
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/site/free-mode", nil, &state)
	got := state.Stats
	if state.Enabled || got.SponsoredSessions-baseline.SponsoredSessions != 1 || got.SponsoredMinutes-baseline.SponsoredMinutes != 3 ||
		got.SponsoredCredits-baseline.SponsoredCredits != 120 || got.AdvisorEarnings-baseline.AdvisorEarnings != 72 || got.LiveSponsoredSessions != 0 || got.Since == nil {
		t.Fatalf("free mode stats: %+v (baseline %+v)", got, baseline)
	}
	after := f.overview()
	if after.Sponsored.Sessions-before.Sponsored.Sessions != 1 || after.Sponsored.Minutes-before.Sponsored.Minutes != 3 ||
		after.Sponsored.Credits-before.Sponsored.Credits != 120 {
		t.Fatalf("overview sponsored: %+v → %+v", before.Sponsored, after.Sponsored)
	}
	if after.CreditsSpentChat != before.CreditsSpentChat || after.CreditsPurchased != before.CreditsPurchased {
		t.Fatalf("sponsored minutes leaked into credits spent: %+v → %+v", before, after)
	}
	if after.PsychicNetCredits-before.PsychicNetCredits != 72 {
		t.Fatalf("advisor net credits in the overview: %d", after.PsychicNetCredits-before.PsychicNetCredits)
	}
}

func TestFreeModeKeepsTheClientsOwnFreeMinutes(t *testing.T) {
	f := newFreeModeFixture(t)
	e, psychic := f.e, f.psychic
	client := e.registerClient() // 5 promotional minutes, no credits
	f.enable(24)
	cws := client.connect("")
	var requested service.SessionView
	f.request(client).decode(t, &requested)
	if !requested.Sponsored {
		t.Fatalf("requested: %+v", requested)
	}
	f.pws.waitFor(realtime.EventSessionRequested, 5*time.Second)
	cws.mustAck(realtime.FrameSessionJoin, map[string]any{"session_id": requested.ID})
	psychic.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+requested.ID.String()+"/accept", nil, nil)
	f.pws.mustAck(realtime.FrameSessionJoin, map[string]any{"session_id": requested.ID})
	cws.waitFor(realtime.EventSessionStarted, 5*time.Second)
	e.tick(time.Minute + time.Second)
	e.tick(time.Minute)
	if w := client.wallet(); w.FreeMinutes != 5 || w.Balance != 0 {
		t.Fatalf("the client's own free minutes must be kept: %+v", w)
	}
	var sum service.SessionSummary
	client.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+requested.ID.String()+"/end", nil, &sum)
	if sum.Session.FreeMinutesUsed != 0 || sum.Session.SponsoredMinutes != 3 || sum.Session.BilledMinutes != 3 {
		t.Fatalf("summary: %+v", sum.Session)
	}
	f.disable()
	// Afterwards the free minutes are used as before.
	var again service.SessionView
	f.request(client).decode(t, &again)
	if again.Sponsored {
		t.Fatalf("a chat requested after free mode is not sponsored: %+v", again)
	}
}

func TestFreeModeEndsByItselfAtEndsAt(t *testing.T) {
	e := newEnv(t)
	e.resetSite()
	admin := e.createAdmin()
	admin.stepUp()
	admin.must(http.StatusOK, http.MethodPut, "/api/v1/admin/site/signup-offer", map[string]any{"enabled": false, "free_minutes": 5}, nil)
	admin.must(http.StatusOK, http.MethodPut, "/api/v1/admin/site/free-mode", map[string]any{"enabled": true, "confirmation": "ENABLE FREE MODE",
		"reason": "One hour test", "ends_at": e.clock.Now().Add(time.Hour).Format(time.RFC3339)}, nil)
	if cfg, _ := e.siteConfig("en"); !cfg.FreeMode.Active {
		t.Fatal("free mode should be active")
	}
	// Two hours later nothing has been switched off, yet it no longer applies.
	e.clock.Advance(2 * time.Hour)
	cfg, _ := e.siteConfig("en")
	if cfg.FreeMode.Active || cfg.FreeMode.EndsAt != nil {
		t.Fatalf("public config after ends_at: %+v", cfg.FreeMode)
	}
	state, err := e.app.Services.Site.FreeMode(context.Background())
	if err != nil || !state.Enabled || state.Active {
		t.Fatalf("state: enabled stays true until an admin switches it off: %+v %v", state, err)
	}
	psychic := e.createPsychic(40)
	psychic.goOnline()
	client := e.registerClient()
	r := client.call(http.MethodPost, "/api/v1/chat/sessions", map[string]any{"psychic_slug": psychic.slug(), "request_key": uniq("req-key-")})
	if r.status != http.StatusPaymentRequired || r.errCode(t) != "insufficient_credits" {
		t.Fatalf("a chat after ends_at needs credits: %d %s", r.status, r.body)
	}
}

func TestFreeModeDoesNotAffectCustomRequestsOrPurchases(t *testing.T) {
	f := newFreeModeFixture(t)
	e := f.e
	f.enable(24)
	e.ensurePacks()
	client := e.registerClient()
	// Credit purchases are unaffected and still charge the wallet.
	if w := client.buyCredits(250); w.Balance != 250 {
		t.Fatalf("purchase: %+v", w)
	}
	// A paid chat is also possible in free mode, but it is sponsored: the
	// balance is not touched.
	cws := client.connect("")
	var requested service.SessionView
	f.request(client).decode(t, &requested)
	f.pws.waitFor(realtime.EventSessionRequested, 5*time.Second)
	cws.mustAck(realtime.FrameSessionJoin, map[string]any{"session_id": requested.ID})
	f.psychic.must(http.StatusOK, http.MethodPost, "/api/v1/chat/sessions/"+requested.ID.String()+"/accept", nil, nil)
	f.pws.mustAck(realtime.FrameSessionJoin, map[string]any{"session_id": requested.ID})
	cws.waitFor(realtime.EventSessionStarted, 5*time.Second)
	e.tick(time.Minute + time.Second)
	if w := client.wallet(); w.Balance != 250 || w.FreeMinutes != 5 {
		t.Fatalf("a sponsored chat touched the wallet: %+v", w)
	}
}

// ---- large dataset ----------------------------------------------------------

func TestSiteStatsAndOverviewOnALargeDataset(t *testing.T) {
	if !available {
		t.Skip("infrastructure unavailable")
	}
	if testing.Short() {
		t.Skip("large dataset")
	}
	const n = 200_000
	db := isolatedSchema(t)
	ctx := context.Background()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	mkUser := func(role domain.Role, label string) uuid.UUID {
		u := domain.User{ID: domain.NewID(), Email: label + "@large.test", PasswordHash: "x", Role: role, Status: domain.UserActive,
			DisplayName: label, Locale: "en", TermsAcceptedAt: t0, CreatedAt: t0, UpdatedAt: t0}
		if err := db.Create(&u).Error; err != nil {
			t.Fatal(err)
		}
		return u.ID
	}
	mkAdvisor := func(label string) uuid.UUID {
		id := mkUser(domain.RolePsychic, label)
		p := domain.Psychic{UserID: id, Slug: label, Name: label, About: domain.StringList{}, Skills: domain.StringList{}, RatePerMinute: 40,
			Availability: domain.AvailabilityOffline, Listed: true}
		if err := db.Create(&p).Error; err != nil {
			t.Fatal(err)
		}
		return id
	}
	mkConversation := func(client, advisor uuid.UUID) uuid.UUID {
		c := domain.Conversation{ID: domain.NewID(), ClientID: client, PsychicID: advisor, LastActivityAt: t0, CreatedAt: t0}
		if err := db.Create(&c).Error; err != nil {
			t.Fatal(err)
		}
		return c.ID
	}
	client, advisor := mkUser(domain.RoleClient, "bulk-client"), mkAdvisor("bulk-advisor")
	conv := mkConversation(client, advisor)

	// 200k finished chats: every 4th is sponsored, with 1-7 minutes, and
	// the advisor's earning is recorded for the sponsored chats of odd length.
	start := time.Now()
	err := db.Exec(`
		WITH s AS (SELECT i, gen_random_uuid() AS id, (i % 7) + 1 AS minutes, (i % 4 = 0) AS sponsored FROM generate_series(1, ?::int) AS i),
		ins AS (
			INSERT INTO chat_sessions (id, conversation_id, client_id, psychic_id, status, end_reason, rate_per_minute, list_rate_per_minute,
				request_key, requested_at, expires_at, started_at, ended_at, settled_at, active_seconds, billed_minutes, credits_charged,
				sponsored, sponsored_minutes, sponsored_credits)
			SELECT id, ?::uuid, ?::uuid, ?::uuid, 'settled', 'client_ended', 40, 40, 'bulk-' || i,
				?::timestamptz + i * interval '10 seconds', ?::timestamptz + i * interval '10 seconds' + interval '2 minutes',
				?::timestamptz + i * interval '10 seconds', ?::timestamptz + i * interval '10 seconds' + minutes * interval '1 minute',
				?::timestamptz + i * interval '10 seconds' + minutes * interval '1 minute', minutes * 60, minutes,
				CASE WHEN sponsored THEN 0 ELSE minutes * 40 END, sponsored, CASE WHEN sponsored THEN minutes ELSE 0 END,
				CASE WHEN sponsored THEN minutes * 40 ELSE 0 END
			FROM s
			RETURNING id, sponsored, sponsored_minutes, requested_at
		)
		INSERT INTO psychic_earnings (id, psychic_id, source_type, source_id, entry_kind, gross_credits, commission_credits, net_credits, share_percent, created_at)
		SELECT gen_random_uuid(), ?::uuid, 'chat_session', id, 'earning', sponsored_minutes * 40,
			sponsored_minutes * 40 - (sponsored_minutes * 40 * 60) / 100, (sponsored_minutes * 40 * 60) / 100, 60, requested_at
		FROM ins WHERE sponsored AND sponsored_minutes % 2 = 1`,
		n, conv, client, advisor, t0, t0, t0, t0, t0, advisor).Error
	if err != nil {
		t.Fatalf("bulk insert: %v", err)
	}
	t.Logf("inserted %d sessions in %s", n, time.Since(start).Round(time.Millisecond))

	// Three live sponsored chats, each with its own pair (one open chat per client and per advisor).
	for i, status := range []string{"accepted", "active", "disconnected"} {
		c, a := mkUser(domain.RoleClient, fmt.Sprintf("live-client-%d", i)), mkAdvisor(fmt.Sprintf("live-advisor-%d", i))
		cv := mkConversation(c, a)
		s := domain.Session{ID: domain.NewID(), ConversationID: cv, ClientID: c, PsychicID: a, Status: domain.SessionStatus(status), RatePerMinute: 40,
			ListRatePerMinute: 40, RequestKey: fmt.Sprintf("live-%d", i), RequestedAt: t0.Add(100 * 24 * time.Hour), ExpiresAt: t0.Add(101 * 24 * time.Hour),
			Sponsored: true, SponsoredMinutes: 2, SponsoredCredits: 80, BilledMinutes: 2, CreatedAt: t0, UpdatedAt: t0}
		if err := db.Create(&s).Error; err != nil {
			t.Fatalf("live session: %v", err)
		}
	}
	// A sponsored request that never started is not counted.
	{
		c, a := mkUser(domain.RoleClient, "idle-client"), mkAdvisor("idle-advisor")
		s := domain.Session{ID: domain.NewID(), ConversationID: mkConversation(c, a), ClientID: c, PsychicID: a, Status: domain.SessionRequested,
			RatePerMinute: 40, ListRatePerMinute: 40, RequestKey: "idle", RequestedAt: t0.Add(100 * 24 * time.Hour), ExpiresAt: t0.Add(101 * 24 * time.Hour),
			Sponsored: true, CreatedAt: t0, UpdatedAt: t0}
		if err := db.Create(&s).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec("ANALYZE chat_sessions").Error; err != nil {
		t.Fatal(err)
	}

	// Expected values, computed independently of SQL.
	var wantSessions, wantMinutes, wantCredits, wantEarn, wantSettled int64
	for i := 1; i <= n; i++ {
		wantSettled++
		if i%4 != 0 {
			continue
		}
		m := int64(i%7) + 1
		wantSessions++
		wantMinutes += m
		wantCredits += m * 40
		if m%2 == 1 {
			wantEarn += m * 40 * 60 / 100
		}
	}
	const liveSessions, liveMinutes = 3, 6

	timed := func(name string, budget time.Duration, fn func()) {
		t.Helper()
		begin := time.Now()
		fn()
		if d := time.Since(begin); d > budget {
			t.Errorf("%s took %s, budget %s", name, d, budget)
		} else {
			t.Logf("%s: %s", name, d.Round(time.Millisecond))
		}
	}

	site := repository.NewSiteRepository(db)
	timed("sponsored totals", 2*time.Second, func() {
		got, err := site.SponsoredTotals(ctx, t0)
		if err != nil {
			t.Fatal(err)
		}
		if got.Sessions != wantSessions+liveSessions || got.Minutes != wantMinutes+liveMinutes || got.Credits != (wantMinutes+liveMinutes)*40 || got.Live != liveSessions {
			t.Fatalf("totals: %+v want sessions=%d minutes=%d credits=%d live=%d", got, wantSessions+liveSessions, wantMinutes+liveMinutes,
				(wantMinutes+liveMinutes)*40, liveSessions)
		}
	})
	timed("sponsored earnings", 2*time.Second, func() {
		got, err := site.SponsoredEarnings(ctx, t0)
		if err != nil {
			t.Fatal(err)
		}
		if got != wantEarn {
			t.Fatalf("earnings: %d want %d", got, wantEarn)
		}
	})
	// A narrower period (the last 50,000 chats) gives smaller, still exact, numbers.
	since := t0.Add(time.Duration(n-50_000) * 10 * time.Second)
	var narrow int64
	for i := n - 50_000; i <= n; i++ {
		if i%4 == 0 {
			narrow++
		}
	}
	if got, err := site.SponsoredTotals(ctx, since); err != nil || got.Sessions != narrow+liveSessions {
		t.Fatalf("narrow totals: %+v %v (want %d sessions)", got, err, narrow+liveSessions)
	}

	reports := repository.NewReportRepository(db)
	timed("admin overview", 6*time.Second, func() {
		o, err := reports.Overview(ctx, t0, t0.Add(400*24*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		if o.Sponsored.Sessions != wantSessions+liveSessions || o.Sponsored.Minutes != wantMinutes+liveMinutes || o.Sponsored.Credits != (wantMinutes+liveMinutes)*40 {
			t.Fatalf("overview sponsored: %+v", o.Sponsored)
		}
		if o.Sessions["settled"] != wantSettled || o.CreditsSpentChat != 0 {
			t.Fatalf("overview sessions/credits: settled=%d spent=%d", o.Sessions["settled"], o.CreditsSpentChat)
		}
	})

	// The same numbers through the service (free-mode state, with the cache).
	clk := clock.NewFake(t0.Add(time.Hour))
	svc := service.NewSiteService(service.SiteDeps{Tx: noTx{}, Repo: site, Cache: cache.NewMemory(nil), Clock: clk, Log: zap.NewNop(),
		Cfg: service.SiteSettings{SignupFallbackMinutes: 5}})
	if err := db.Exec(`UPDATE site_settings SET value = jsonb_build_object('enabled', true, 'reason', 'bulk', 'enabled_at', ?::text), version = version + 1 WHERE key = 'free_mode'`,
		t0.Format(time.RFC3339)).Error; err != nil {
		t.Fatal(err)
	}
	timed("free mode state", 3*time.Second, func() {
		st, err := svc.FreeMode(ctx)
		if err != nil {
			t.Fatal(err)
		}
		s := st.Stats
		if s.SponsoredSessions != wantSessions+liveSessions || s.LiveSponsoredSessions != liveSessions || s.AdvisorEarnings != wantEarn ||
			s.SponsoredMinutes != wantMinutes+liveMinutes {
			t.Fatalf("state: %+v", s)
		}
	})
	timed("free mode state (cached)", 50*time.Millisecond, func() {
		if _, err := svc.FreeMode(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

// noTx runs functions without a transaction (read-only service calls).
type noTx struct{}

func (noTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }
