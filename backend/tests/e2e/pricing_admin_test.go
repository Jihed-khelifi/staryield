package e2e

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 5.0: pricing administration (§24.1).

// restoreCatalogFlags puts the canonical flags back after a test that moved
// the reference or the intro pack.
func (e *env) restoreCatalogFlags() {
	e.t.Cleanup(func() {
		e.app.DB.Exec("UPDATE credit_packs SET reference = false WHERE code <> 'quick_150' AND reference")
		e.app.DB.Exec("UPDATE credit_packs SET reference = true WHERE code = 'quick_150' AND active")
		e.app.DB.Exec("UPDATE credit_packs SET active = false WHERE kind = 'intro' AND code <> 'welcome_600'")
		e.app.DB.Exec("UPDATE credit_packs SET active = true WHERE code = 'welcome_600'")
	})
}

func adminPackBody(code string, extra map[string]any) map[string]any {
	body := map[string]any{"code": code, "name": map[string]string{"en": "Promo " + code}, "credits": 400, "price_cents": 3000}
	for k, v := range extra {
		body[k] = v
	}
	return body
}

func fieldsOf(t *testing.T, r response) map[string]string {
	t.Helper()
	var body struct {
		Error struct {
			Details struct {
				Fields map[string]string `json:"fields"`
			} `json:"details"`
		} `json:"error"`
	}
	r.decode(t, &body)
	return body.Error.Details.Fields
}

func TestAdminPackWritesNeedStepUp(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	e.clock.Advance(11 * time.Minute) // the sign-in step-up window has passed
	var list struct {
		Items []service.AdminPack `json:"items"`
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/credit-packs", nil, &list) // reads need none
	if len(list.Items) < 6 {
		t.Fatalf("admin packs: %+v", list.Items)
	}
	id := list.Items[0].ID.String()
	ids := make([]string, len(list.Items))
	for i, p := range list.Items {
		ids[i] = p.ID.String()
	}
	for name, r := range map[string]response{
		"create":   admin.call(http.MethodPost, "/api/v1/admin/credit-packs", adminPackBody(uniq("nostep_"), nil)),
		"update":   admin.call(http.MethodPatch, "/api/v1/admin/credit-packs/"+id, map[string]any{"featured": true}),
		"delete":   admin.call(http.MethodDelete, "/api/v1/admin/credit-packs/"+id, nil),
		"reorder":  admin.call(http.MethodPut, "/api/v1/admin/credit-packs/order", map[string]any{"ids": ids}),
		"settings": admin.call(http.MethodPut, "/api/v1/admin/pricing/settings", map[string]any{}),
	} {
		if r.status != http.StatusForbidden || r.errCode(t) != "step_up_required" {
			t.Errorf("%s without step-up: %d %s", name, r.status, r.body)
		}
	}
	// Reads of the settings need none.
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/pricing/settings", nil, nil)
	// Pack writes are admin-only.
	if r := e.registerClient().call(http.MethodGet, "/api/v1/admin/credit-packs", nil); r.status == http.StatusOK {
		t.Fatal("clients must not read the admin catalog")
	}
}

func TestAdminPackCatalogRules(t *testing.T) {
	e := newEnv(t)
	e.restoreCatalogFlags()
	admin := e.createAdmin()
	admin.stepUp()
	const packs = "/api/v1/admin/credit-packs"

	// Validation (422, dotted field keys).
	for name, c := range map[string]struct {
		body map[string]any
		key  string
	}{
		"no code":       {map[string]any{"name": map[string]string{"en": "x"}, "credits": 10, "price_cents": 100}, "code"},
		"bad code":      {adminPackBody("Bad Code", nil), "code"},
		"short code":    {adminPackBody("a", nil), "code"},
		"no english":    {adminPackBody(uniq("v_"), map[string]any{"name": map[string]string{"fr": "Seulement"}}), "name.en"},
		"unknown lang":  {adminPackBody(uniq("v_"), map[string]any{"name": map[string]string{"en": "x", "de": "x"}}), "name.de"},
		"markup":        {adminPackBody(uniq("v_"), map[string]any{"tagline": map[string]string{"en": "<b>Best</b>"}}), "tagline.en"},
		"long tagline":  {adminPackBody(uniq("v_"), map[string]any{"tagline": map[string]string{"en": strings.Repeat("a", 121)}}), "tagline.en"},
		"long label":    {adminPackBody(uniq("v_"), map[string]any{"label": map[string]string{"en": strings.Repeat("a", 41)}}), "label.en"},
		"long name":     {adminPackBody(uniq("v_"), map[string]any{"name": map[string]string{"en": strings.Repeat("a", 61)}}), "name.en"},
		"zero credits":  {adminPackBody(uniq("v_"), map[string]any{"credits": 0}), "credits"},
		"cheap":         {adminPackBody(uniq("v_"), map[string]any{"price_cents": 49}), "price_cents"},
		"bad kind":      {adminPackBody(uniq("v_"), map[string]any{"kind": "gift"}), "kind"},
		"standard only": {adminPackBody(uniq("v_"), map[string]any{"new_clients_only": true}), "new_clients_only"},
		"intro ref":     {adminPackBody(uniq("v_"), map[string]any{"kind": "intro", "active": false, "reference": true}), "reference"},
		"inactive ref":  {adminPackBody(uniq("v_"), map[string]any{"active": false, "reference": true}), "reference"},
	} {
		r := admin.call(http.MethodPost, packs, c.body)
		if r.status != http.StatusUnprocessableEntity || fieldsOf(t, r)[c.key] == "" {
			t.Errorf("%s: want 422 with %s, got %d %s", name, c.key, r.status, r.body)
		}
	}
	// Messages follow the administrator's language.
	fr := admin
	fr.locale = "fr"
	if r := fr.call(http.MethodPost, packs, adminPackBody("Bad Code", nil)); !strings.Contains(fieldsOf(t, r)["code"], "minuscules") {
		t.Fatalf("localized validation: %s", r.body)
	}
	fr.locale = ""

	// Create: the shape of AdminPack, appended last, audited.
	code := uniq("promo_")
	var p service.AdminPack
	admin.must(http.StatusCreated, http.MethodPost, packs, adminPackBody(code, map[string]any{"tagline": map[string]string{"en": "Fresh", "fr": "Frais"}}), &p)
	if p.Code != code || p.Kind != "standard" || p.Name["en"] != "Promo "+code || p.Tagline["fr"] != "Frais" || !p.Active || p.Currency != "USD" ||
		p.PricePerCreditMillicents != 7500 || p.NewClientsOnly || p.Reference || p.Badges == nil {
		t.Fatalf("created: %+v", p)
	}
	if r := admin.call(http.MethodPost, packs, adminPackBody(code, nil)); r.status != http.StatusUnprocessableEntity || fieldsOf(t, r)["code"] == "" {
		t.Fatalf("duplicate code: %d %s", r.status, r.body)
	}

	// The code and kind are immutable (the same value is accepted).
	if r := admin.call(http.MethodPatch, packs+"/"+p.ID.String(), map[string]any{"code": "other_code"}); r.status != http.StatusUnprocessableEntity || fieldsOf(t, r)["code"] == "" {
		t.Fatalf("code change: %d %s", r.status, r.body)
	}
	if r := admin.call(http.MethodPatch, packs+"/"+p.ID.String(), map[string]any{"kind": "intro"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("kind change: %d %s", r.status, r.body)
	}
	if r := admin.call(http.MethodPatch, packs+"/"+p.ID.String(), map[string]any{}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("empty patch: %d", r.status)
	}

	// Price and credit edits are audited with before and after.
	admin.must(http.StatusOK, http.MethodPatch, packs+"/"+p.ID.String(), map[string]any{"price_cents": 3500, "credits": 450,
		"name": map[string]string{"en": "Promo 2", "es": "Promo ES"}}, &p)
	if p.PriceCents != 3500 || p.Credits != 450 || p.Name["es"] != "Promo ES" || p.Name["en"] != "Promo 2" {
		t.Fatalf("updated: %+v", p)
	}
	var audit service.Page[service.AuditView]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/audit-logs?actor_id="+admin.adminID.String(), nil, &audit)
	found := false
	for _, a := range audit.Items {
		if a.Action == "pack.updated" && a.TargetID == p.ID.String() {
			found = true
			if a.Metadata["price_before"] != float64(3000) || a.Metadata["price_after"] != float64(3500) ||
				a.Metadata["credits_before"] != float64(400) || a.Metadata["credits_after"] != float64(450) {
				t.Fatalf("audit before/after: %v", a.Metadata)
			}
		}
	}
	if !found {
		t.Fatalf("pack.updated not audited: %+v", audit.Items)
	}

	// The reference: setting it clears the previous one in the same transaction.
	admin.must(http.StatusOK, http.MethodPatch, packs+"/"+p.ID.String(), map[string]any{"reference": true}, &p)
	var list struct {
		Items []service.AdminPack `json:"items"`
	}
	admin.must(http.StatusOK, http.MethodGet, packs, nil, &list)
	refs := 0
	for _, it := range list.Items {
		if it.Reference {
			refs++
			if it.ID != p.ID {
				t.Fatalf("the previous reference must be cleared: %+v", it)
			}
		}
	}
	if refs != 1 {
		t.Fatalf("exactly one reference: %d", refs)
	}
	// The reference cannot be deactivated (422), unless it stops being the reference.
	if r := admin.call(http.MethodPatch, packs+"/"+p.ID.String(), map[string]any{"active": false}); r.status != http.StatusUnprocessableEntity || fieldsOf(t, r)["active"] == "" {
		t.Fatalf("deactivating the reference: %d %s", r.status, r.body)
	}
	if r := admin.call(http.MethodPatch, packs+"/"+p.ID.String(), map[string]any{"active": false, "reference": false}); r.status != http.StatusOK {
		t.Fatalf("deactivating and un-referencing: %d %s", r.status, r.body)
	}

	// Only one active intro pack (409); an inactive one can be created.
	r := admin.call(http.MethodPost, packs, adminPackBody(uniq("intro_"), map[string]any{"kind": "intro"}))
	if r.status != http.StatusConflict || r.errCode(t) != "conflict" {
		t.Fatalf("a second active intro pack: %d %s", r.status, r.body)
	}
	var second service.AdminPack
	admin.must(http.StatusCreated, http.MethodPost, packs, adminPackBody(uniq("intro_"), map[string]any{"kind": "intro", "active": false}), &second)
	if !second.NewClientsOnly {
		t.Fatalf("intro packs default to new clients only: %+v", second)
	}
	if r := admin.call(http.MethodPatch, packs+"/"+second.ID.String(), map[string]any{"active": true}); r.status != http.StatusConflict {
		t.Fatalf("activating a second intro pack: %d %s", r.status, r.body)
	}
	// An intro pack cannot be the reference.
	if r := admin.call(http.MethodPatch, packs+"/"+second.ID.String(), map[string]any{"reference": true}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("intro reference: %d", r.status)
	}

	// Delete: an unused pack goes; one with purchases is a 409 (deactivate instead).
	admin.must(http.StatusNoContent, http.MethodDelete, packs+"/"+second.ID.String(), nil, nil)
	if r := admin.call(http.MethodDelete, packs+"/"+second.ID.String(), nil); r.status != http.StatusNotFound {
		t.Fatalf("deleted twice: %d", r.status)
	}
	var starter service.AdminPack
	for _, it := range list.Items {
		if it.Code == "starter_315" {
			starter = it
		}
	}
	buyer := e.registerClient()
	if r := buyer.buy(starter.ID, nil); r.status != http.StatusCreated {
		t.Fatalf("purchase: %d %s", r.status, r.body)
	}
	if r := admin.call(http.MethodDelete, packs+"/"+starter.ID.String(), nil); r.status != http.StatusConflict {
		t.Fatalf("a pack with purchases: %d %s", r.status, r.body)
	}
	// A purchase snapshots credits and price: later edits only affect new ones.
	admin.must(http.StatusOK, http.MethodPatch, packs+"/"+starter.ID.String(), map[string]any{"price_cents": 4100}, nil)
	t.Cleanup(func() { e.app.DB.Exec("UPDATE credit_packs SET price_cents = 4000 WHERE code = 'starter_315'") })
	var history service.Page[service.PurchaseView]
	buyer.must(http.StatusOK, http.MethodGet, "/api/v1/credits/purchases", nil, &history)
	if len(history.Items) != 1 || history.Items[0].PriceCents != 4000 {
		t.Fatalf("purchase snapshot: %+v", history.Items)
	}
}

func TestAdminPackOrder(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	e.isolateCatalog()
	admin := e.createAdmin()
	admin.stepUp()
	const order = "/api/v1/admin/credit-packs/order"
	var list struct {
		Items []service.AdminPack `json:"items"`
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/credit-packs", nil, &list)
	var ids []string
	for _, p := range list.Items {
		ids = append(ids, p.ID.String())
	}
	// Reverse the whole list (inactive packs included), restore afterwards.
	e.t.Cleanup(func() {
		for _, p := range list.Items {
			e.app.DB.Exec("UPDATE credit_packs SET sort_order = ? WHERE id = ?", p.SortOrder, p.ID)
		}
	})
	rev := make([]string, len(ids))
	for i := range ids {
		rev[len(ids)-1-i] = ids[i]
	}
	var out struct {
		Items []service.AdminPack `json:"items"`
	}
	admin.must(http.StatusOK, http.MethodPut, order, map[string]any{"ids": rev}, &out)
	for i, p := range out.Items {
		if p.ID.String() != rev[i] {
			t.Fatalf("position %d: %s, want %s", i, p.ID, rev[i])
		}
	}
	// The public catalog follows (active packs only, same relative order).
	sum, _ := e.pricing(e.client(), "")
	if sum.Packs[0].Code != "premium_2400" || sum.Packs[len(sum.Packs)-1].Code != "welcome_600" {
		t.Fatalf("public order: %+v", sum.Packs)
	}
	// Every pack exactly once.
	for name, body := range map[string]any{
		"missing":   map[string]any{"ids": rev[1:]},
		"duplicate": map[string]any{"ids": append([]string{rev[0]}, rev...)[:len(rev)]},
		"unknown":   map[string]any{"ids": append(append([]string{}, rev[1:]...), "7c4a6e0e-0000-4000-8000-000000000000")},
		"not uuid":  map[string]any{"ids": []string{"x"}},
		"absent":    map[string]any{},
	} {
		if r := admin.call(http.MethodPut, order, body); r.status != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %s", name, r.status, r.body)
		}
	}
}

func TestAdminPricingSettings(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	t.Cleanup(func() {
		e.app.DB.Exec(`UPDATE site_settings SET value = '{"rate_min": 30, "rate_max": 60, "popular_min_purchases": 25, "popular_window_days": 90, "processing_fee_bps": 800, "processing_fixed_cents": 30}', version = version + 1 WHERE key = 'pricing'`)
	})
	var s service.PricingSettings
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/pricing/settings", nil, &s)
	if s.RateMin != 30 || s.RateMax != 60 || s.PopularMinPurchases != 25 || s.PopularWindowDays != 90 || s.ProcessingFeeBps != 800 || s.ProcessingFixedCents != 30 || s.Version < 1 {
		t.Fatalf("defaults: %+v", s)
	}
	admin.stepUp()
	body := func(extra map[string]any) map[string]any {
		b := map[string]any{"rate_min": 25, "rate_max": 70, "popular_min_purchases": 10, "popular_window_days": 60, "processing_fee_bps": 750,
			"processing_fixed_cents": 25, "expected_version": s.Version}
		for k, v := range extra {
			b[k] = v
		}
		return b
	}
	for name, c := range map[string]struct {
		extra map[string]any
		key   string
	}{
		"rate":      {map[string]any{"rate_min": 0}, "rate_min"},
		"order":     {map[string]any{"rate_min": 50, "rate_max": 40}, "rate_max"},
		"threshold": {map[string]any{"popular_min_purchases": 0}, "popular_min_purchases"},
		"window":    {map[string]any{"popular_window_days": 3}, "popular_window_days"},
		"fee":       {map[string]any{"processing_fee_bps": 3001}, "processing_fee_bps"},
		"fixed":     {map[string]any{"processing_fixed_cents": 501}, "processing_fixed_cents"},
	} {
		if r := admin.call(http.MethodPut, "/api/v1/admin/pricing/settings", body(c.extra)); r.status != http.StatusUnprocessableEntity || fieldsOf(t, r)[c.key] == "" {
			t.Errorf("%s: %d %s", name, r.status, r.body)
		}
	}
	var saved service.PricingSettings
	admin.must(http.StatusOK, http.MethodPut, "/api/v1/admin/pricing/settings", body(nil), &saved)
	if saved.RateMin != 25 || saved.RateMax != 70 || saved.Version != s.Version+1 || saved.UpdatedBy == nil || saved.UpdatedBy.ID != admin.adminID {
		t.Fatalf("saved: %+v", saved)
	}
	// A stale version is a conflict that names the current one.
	r := admin.call(http.MethodPut, "/api/v1/admin/pricing/settings", body(map[string]any{"expected_version": s.Version}))
	if r.status != http.StatusConflict || r.errCode(t) != "version_conflict" || !strings.Contains(string(r.body), `"current_version":`) {
		t.Fatalf("stale version: %d %s", r.status, r.body)
	}
	// The public summary reports the new range at once; the economics stay private.
	if sum, _ := e.pricing(e.client(), ""); sum.RateRange.Min != 25 || sum.RateRange.Max != 70 {
		t.Fatalf("public range: %+v", sum.RateRange)
	}
	// Audited with before and after.
	var audit service.Page[service.AuditView]
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/audit-logs?actor_id="+admin.adminID.String(), nil, &audit)
	found := false
	for _, a := range audit.Items {
		if a.Action == "pricing.settings_updated" {
			found = true
			before, _ := a.Metadata["before"].(map[string]any)
			after, _ := a.Metadata["after"].(map[string]any)
			if before["rate_min"] != float64(30) || after["rate_min"] != float64(25) {
				t.Fatalf("audit: %v", a.Metadata)
			}
		}
	}
	if !found {
		t.Fatal("pricing.settings_updated not audited")
	}
	// Saving the same values again changes nothing (no version bump).
	var same service.PricingSettings
	admin.must(http.StatusOK, http.MethodPut, "/api/v1/admin/pricing/settings", body(map[string]any{"expected_version": saved.Version}), &same)
	if same.Version != saved.Version {
		t.Fatalf("unchanged save bumped the version: %d -> %d", saved.Version, same.Version)
	}
}

func TestAdvisorRateRange(t *testing.T) {
	e := newEnv(t)
	admin := e.createAdmin()
	t.Cleanup(func() {
		e.app.DB.Exec(`UPDATE site_settings SET value = '{"rate_min": 30, "rate_max": 60, "popular_min_purchases": 25, "popular_window_days": 90, "processing_fee_bps": 800, "processing_fixed_cents": 30}', version = version + 1 WHERE key = 'pricing'`)
	})
	create := func(rate int) response {
		s := uniq("rate")
		return admin.call(http.MethodPost, "/api/v1/admin/psychics", map[string]any{"email": s + "@example.com", "password": strongPassword,
			"name": "Advisor " + s, "slug": s, "rate_per_minute": rate})
	}
	for _, rate := range []int{29, 61, 100} {
		if r := create(rate); r.status != http.StatusUnprocessableEntity || !strings.Contains(fieldsOf(t, r)["rate_per_minute"], "30") {
			t.Fatalf("rate %d: %d %s", rate, r.status, r.body)
		}
	}
	admin.locale = "es"
	if r := create(10); !strings.Contains(fieldsOf(t, r)["rate_per_minute"], "créditos") {
		t.Fatalf("localized: %s", r.body)
	}
	admin.locale = ""
	var ok service.AdminPsychicView
	if r := create(30); r.status != http.StatusCreated {
		t.Fatalf("lower bound: %d %s", r.status, r.body)
	} else {
		r.decode(t, &ok)
	}
	if r := create(60); r.status != http.StatusCreated {
		t.Fatalf("upper bound: %d %s", r.status, r.body)
	}
	if ok.RateOutOfRange {
		t.Fatalf("in range: %+v", ok)
	}
	// Updates obey the range; other fields of an out-of-range advisor stay editable.
	if r := admin.call(http.MethodPatch, "/api/v1/admin/psychics/"+ok.ID.String(), map[string]any{"rate_per_minute": 100}); r.status != http.StatusUnprocessableEntity || fieldsOf(t, r)["rate_per_minute"] == "" {
		t.Fatalf("update out of range: %d %s", r.status, r.body)
	}
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/psychics/"+ok.ID.String(), map[string]any{"rate_per_minute": 45}, &ok)
	// An advisor set before the range existed is left alone and flagged.
	legacy := e.createPsychic(10)
	var flagged service.AdminPsychicView
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/psychics/"+legacy.user.ID.String(), nil, &flagged)
	if flagged.RatePerMinute != 10 || !flagged.RateOutOfRange {
		t.Fatalf("detail: %+v", flagged)
	}
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/psychics/"+legacy.user.ID.String(), map[string]any{"headline": "Still here"}, &flagged)
	if flagged.RatePerMinute != 10 || !flagged.RateOutOfRange {
		t.Fatalf("an unrelated edit leaves the rate alone: %+v", flagged)
	}
	var list struct {
		Items []service.AdminPsychicView `json:"items"`
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/psychics?q="+strings.TrimSuffix(legacy.user.Email, "@advisors.example.com"), nil, &list)
	if len(list.Items) != 1 || !list.Items[0].RateOutOfRange {
		t.Fatalf("list: %+v", list.Items)
	}
	// Widening the range takes effect at once and clears the flag.
	admin.stepUp()
	var s service.PricingSettings
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/pricing/settings", nil, &s)
	admin.must(http.StatusOK, http.MethodPut, "/api/v1/admin/pricing/settings", map[string]any{"rate_min": 5, "rate_max": 200, "popular_min_purchases": 25,
		"popular_window_days": 90, "processing_fee_bps": 800, "processing_fixed_cents": 30, "expected_version": s.Version}, nil)
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/psychics/"+legacy.user.ID.String(), nil, &flagged)
	if flagged.RateOutOfRange {
		t.Fatalf("flag after widening: %+v", flagged)
	}
	admin.must(http.StatusOK, http.MethodPatch, "/api/v1/admin/psychics/"+ok.ID.String(), map[string]any{"rate_per_minute": 150}, nil)
}
