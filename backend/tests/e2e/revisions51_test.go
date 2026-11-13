package e2e

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 5.1: merchant-account readiness (docs/platform-extensions.md §25):
// the memberships switch, consent evidence, the statement descriptor and the
// confirmation e-mails.

const (
	purchasesPath = "/api/v1/credits/purchases"
	descriptor    = "STARYIELD MEDIA LLC"
)

func pack(t *testing.T, c *client, credits int64) service.PackView {
	t.Helper()
	var packs struct {
		Items []service.PackView `json:"items"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/credit-packs", nil, &packs)
	for _, p := range packs.Items {
		if p.Credits == credits && strings.HasPrefix(p.Code, "test_") {
			return p
		}
	}
	t.Fatalf("no test pack with %d credits", credits)
	return service.PackView{}
}

// waitMail runs the outbox until a message to `to` whose subject contains `subject` arrived.
func waitMail(t *testing.T, e *env, to, subject string) mail.Message {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		for _, m := range sentTo(e.flushMail(), to) {
			if strings.Contains(m.Subject, subject) {
				return m
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no e-mail %q to %s; sent: %+v", subject, to, e.mail.Sent())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func countMail(e *env, to, subject string) int {
	n := 0
	for _, m := range sentTo(e.mail.Sent(), to) {
		if strings.Contains(m.Subject, subject) {
			n++
		}
	}
	return n
}

// ------------------------------------------------------------ §25.2 consent

func TestPurchaseRequiresConsentAndStoresTheEvidence(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	c := e.registerClient()
	p := pack(t, c, 250)
	body := func(key string, consent any) map[string]any {
		m := map[string]any{"pack_id": p.ID, "idempotency_key": key}
		if consent != nil {
			m["consent"] = consent
		}
		return m
	}
	// Missing or invalid consent: 422 fields.consent, nothing charged or created.
	key := uniq("purchase-key-")
	for name, consent := range map[string]any{
		"missing":      nil,
		"empty":        map[string]any{},
		"no refund":    map[string]any{"terms": "2026-11-13", "privacy": "2026-11-13"},
		"bad format":   map[string]any{"terms": "13/11/2026", "privacy": "2026-11-13", "refund": "2026-11-13"},
		"not a date":   map[string]any{"terms": "2026-02-30", "privacy": "2026-11-13", "refund": "2026-11-13"},
		"future":       map[string]any{"terms": "2026-11-13", "privacy": "2999-01-01", "refund": "2026-11-13"},
		"blank string": map[string]any{"terms": "", "privacy": "", "refund": ""},
	} {
		r := c.call(http.MethodPost, purchasesPath, body(key, consent))
		if r.status != http.StatusUnprocessableEntity || errField(t, r, "consent") != "Accept the Terms of Use, Privacy Policy and Refund & Cancellation Policy to continue." {
			t.Fatalf("%s: %d %s", name, r.status, r.body)
		}
	}
	// Unknown fields stay strict.
	if r := c.call(http.MethodPost, purchasesPath, map[string]any{"pack_id": p.ID, "idempotency_key": key, "consent": map[string]any{
		"terms": "2026-11-13", "privacy": "2026-11-13", "refund": "2026-11-13", "marketing": true}}); r.status != http.StatusUnprocessableEntity ||
		errField(t, r, "marketing") == "" {
		t.Fatalf("unknown consent field: %d %s", r.status, r.body)
	}
	var n int64
	e.app.DB.Raw("SELECT count(*) FROM credit_purchases WHERE user_id = ?", c.user.ID).Scan(&n)
	if n != 0 || c.wallet().Balance != 0 {
		t.Fatalf("a refused purchase left %d purchases, balance %d", n, c.wallet().Balance)
	}
	// The French message.
	r := c.raw(http.MethodPost, purchasesPath, strings.NewReader(`{"pack_id":"`+p.ID.String()+`","idempotency_key":"`+key+`"}`),
		map[string]string{"Content-Type": "application/json", "X-Locale": "fr", "X-CSRF-Token": c.csrf})
	if r.status != http.StatusUnprocessableEntity || !strings.Contains(string(r.body), "Acceptez les Conditions d'utilisation") {
		t.Fatalf("french message: %d %s", r.status, r.body)
	}

	// A valid purchase stores the evidence.
	var first service.PurchaseView
	ua := "  Mozilla/5.0\t(X11; Linux)   " + strings.Repeat("x", 400)
	r = c.raw(http.MethodPost, purchasesPath, strings.NewReader(string(mustJSON(t, body(key, map[string]any{"terms": "2026-11-10", "privacy": "2026-11-11", "refund": "2026-11-12"})))),
		map[string]string{"Content-Type": "application/json", "X-CSRF-Token": c.csrf, "User-Agent": ua})
	if r.status != http.StatusCreated {
		t.Fatalf("purchase: %d %s", r.status, r.body)
	}
	r.decode(t, &first)
	var row struct {
		Terms, Privacy, Refund, IPHash, UserAgent string
		At                                        time.Time
	}
	if err := e.app.DB.Raw(`SELECT consent_terms_version, consent_privacy_version, consent_refund_version, consent_ip_hash, consent_user_agent, consent_at
		FROM credit_purchases WHERE id = ?`, first.ID).Row().Scan(&row.Terms, &row.Privacy, &row.Refund, &row.IPHash, &row.UserAgent, &row.At); err != nil {
		t.Fatal(err)
	}
	if row.Terms != "2026-11-10" || row.Privacy != "2026-11-11" || row.Refund != "2026-11-12" {
		t.Fatalf("versions: %+v", row)
	}
	if want := security.Keyed(e.app.Config.Analytics.IPHashSecret, "127.0.0.1", 32); row.IPHash != want {
		t.Fatalf("ip hash %q, want the keyed hash of the client IP %q", row.IPHash, want)
	}
	// (The HTTP edge keeps 256 bytes of the header; the 300-rune bound is checked in the service tests.)
	if runes := len([]rune(row.UserAgent)); !strings.HasPrefix(row.UserAgent, "Mozilla/5.0 (X11; Linux) xxx") || runes < 200 || runes > 300 {
		t.Fatalf("user agent: %q (%d runes)", row.UserAgent, len([]rune(row.UserAgent)))
	}
	if d := e.clock.Now().Sub(row.At); d < -time.Minute || d > time.Minute {
		t.Fatalf("consent_at is server time: %s vs %s", row.At, e.clock.Now())
	}

	// A replay returns the original purchase without repeating the consent.
	var replay service.PurchaseView
	c.must(http.StatusCreated, http.MethodPost, purchasesPath, body(key, nil), &replay)
	if replay.ID != first.ID || c.wallet().Balance != 250 {
		t.Fatalf("replay: %+v balance %d", replay, c.wallet().Balance)
	}

	// The purchase history does not echo the evidence.
	if r := c.call(http.MethodGet, purchasesPath, nil); strings.Contains(string(r.body), "consent") {
		t.Fatalf("purchase history leaks the consent record: %s", r.body)
	}

	// Admin: the transaction row carries {terms, privacy, refund, at}.
	admin := e.createAdmin()
	r = admin.call(http.MethodGet, "/api/v1/admin/billing/transactions?kind=credit_purchase&user_id="+c.user.ID.String(), nil)
	if r.status != http.StatusOK {
		t.Fatalf("admin transactions: %d %s", r.status, r.body)
	}
	var page struct {
		Items []map[string]any `json:"items"`
	}
	r.decode(t, &page)
	if len(page.Items) != 1 {
		t.Fatalf("rows: %s", r.body)
	}
	consent, _ := page.Items[0]["consent"].(map[string]any)
	if consent["terms"] != "2026-11-10" || consent["privacy"] != "2026-11-11" || consent["refund"] != "2026-11-12" || consent["at"] == nil || len(consent) != 4 {
		t.Fatalf("admin consent: %v", page.Items[0]["consent"])
	}
	if _, leaked := page.Items[0]["consent_ip_hash"]; leaked || strings.Contains(string(r.body), row.IPHash) || strings.Contains(string(r.body), "Mozilla") {
		t.Fatalf("admin row leaks the IP hash or user agent: %s", r.body)
	}
	// Rows without a record (older purchases, other kinds) have no consent key.
	e.app.DB.Exec("UPDATE credit_purchases SET consent_at = NULL, consent_terms_version = NULL WHERE id = ?", first.ID)
	r = admin.call(http.MethodGet, "/api/v1/admin/billing/transactions?kind=credit_purchase&user_id="+c.user.ID.String(), nil)
	if strings.Contains(string(r.body), `"consent"`) {
		t.Fatalf("a purchase without a record shows consent: %s", r.body)
	}
	e.app.DB.Exec("UPDATE credit_purchases SET consent_at = ?, consent_terms_version = '2026-11-10' WHERE id = ?", row.At, first.ID)

	// The export lists the versions and the time, never the IP hash or the user agent.
	files := exportFiles(t, e, c)
	var exported []struct {
		ID      uuid.UUID `json:"id"`
		Consent *struct {
			Terms   string    `json:"terms"`
			Privacy string    `json:"privacy"`
			Refund  string    `json:"refund"`
			At      time.Time `json:"at"`
		} `json:"consent"`
	}
	if err := json.Unmarshal(files["credits/purchases.json"], &exported); err != nil || len(exported) != 1 {
		t.Fatalf("export purchases: %v %s", err, files["credits/purchases.json"])
	}
	if x := exported[0]; x.ID != first.ID || x.Consent == nil || x.Consent.Terms != "2026-11-10" || x.Consent.Privacy != "2026-11-11" ||
		x.Consent.Refund != "2026-11-12" || x.Consent.At.IsZero() {
		t.Fatalf("exported consent: %+v", exported[0])
	}
	for path, b := range files {
		if strings.Contains(string(b), row.IPHash) || strings.Contains(string(b), "Mozilla/5.0") {
			t.Fatalf("export %s carries the IP hash or the user agent", path)
		}
	}
}

// ------------------------------------------------- §25.1 memberships switch

func TestMembershipsSwitchedOff(t *testing.T) {
	// A membership that exists while the switch is on...
	on := newEnv(t)
	plan, version := on.billingPlan(999)
	c1 := on.registerClient()
	card := c1.attachCard("sbx_a_visa_4242_12_2035")
	c1.subscribe(version, card.ID)
	var live struct {
		Items []service.PlanView `json:"items"`
	}
	c1.must(http.StatusOK, http.MethodGet, "/api/v1/billing/plans", nil, &live)
	if len(live.Items) == 0 {
		t.Fatal("with memberships on the plans are listed")
	}
	var cfgOn struct {
		Features struct {
			Memberships bool `json:"memberships"`
		} `json:"features"`
	}
	on.client().must(http.StatusOK, http.MethodGet, "/api/v1/site/config", nil, &cfgOn)
	if !cfgOn.Features.Memberships {
		t.Fatal("site config: features.memberships must be true while memberships are on")
	}
	admin := on.createAdmin()
	var adminOn struct {
		Items       []service.AdminPlanView `json:"items"`
		Memberships bool                    `json:"memberships_enabled"`
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/billing/plans", nil, &adminOn)
	if !adminOn.Memberships || len(adminOn.Items) == 0 {
		t.Fatalf("admin plans while on: %+v", adminOn)
	}

	// ...keeps working after the switch goes off: it can be seen and cancelled.
	off := newEnv(t, withMemberships(false))
	c := off.login(c1.user.Email, c1.pw)
	var plans map[string]json.RawMessage
	c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/plans", nil, &plans)
	if string(plans["items"]) != "[]" {
		t.Fatalf("plans while off: %s", plans["items"])
	}
	refused := func(name string, r response) {
		t.Helper()
		if r.status != http.StatusServiceUnavailable || r.errCode(t) != "feature_unavailable" || !strings.Contains(string(r.body), "Memberships are not available.") {
			t.Fatalf("%s while off: %d %s", name, r.status, r.body)
		}
	}
	other := c.attachCard("sbx_a_mastercard_4444_01_2034")
	refused("subscribe", c.call(http.MethodPost, "/api/v1/billing/subscriptions", map[string]any{"plan_version_id": version, "payment_method_id": other.ID,
		"idempotency_key": uniq("sub-key-")}))
	refused("change card", c.call(http.MethodPatch, "/api/v1/billing/subscription", map[string]any{"payment_method_id": other.ID}))
	refused("resume", c.call(http.MethodPost, "/api/v1/billing/subscription/resume", nil))
	fr := c.raw(http.MethodPost, "/api/v1/billing/subscription/resume", strings.NewReader("{}"),
		map[string]string{"Content-Type": "application/json", "X-Locale": "es", "X-CSRF-Token": c.csrf})
	if !strings.Contains(string(fr.body), "Las membresías no están disponibles.") {
		t.Fatalf("spanish message: %s", fr.body)
	}
	if s := c.subscription(); s == nil || s.Status != "active" {
		t.Fatalf("an existing membership can always be seen: %+v", s)
	}
	var cancelled struct {
		Subscription *service.SubscriptionView `json:"subscription"`
	}
	c.must(http.StatusOK, http.MethodPost, "/api/v1/billing/subscription/cancel", map[string]any{"reason": "Switching off"}, &cancelled)
	if cancelled.Subscription == nil || !cancelled.Subscription.CancelAtPeriodEnd {
		t.Fatalf("an existing membership can always be cancelled: %+v", cancelled)
	}
	// Cards and everything else are unaffected.
	if len(c.cards()) < 2 {
		t.Fatal("saved cards still work")
	}

	// The site config says so, and the admin plan endpoints stay usable.
	var cfgOff struct {
		Features map[string]bool `json:"features"`
	}
	off.client().must(http.StatusOK, http.MethodGet, "/api/v1/site/config", nil, &cfgOff)
	if v, ok := cfgOff.Features["memberships"]; !ok || v {
		t.Fatalf("site config features while off: %v", cfgOff.Features)
	}
	// The validator changes with the flag.
	etagOn := on.client().raw(http.MethodGet, "/api/v1/site/config", nil, nil).header.Get("ETag")
	etagOff := off.client().raw(http.MethodGet, "/api/v1/site/config", nil, nil).header.Get("ETag")
	if etagOn == "" || etagOn == etagOff {
		t.Fatalf("site config ETag must change with features.memberships: %q vs %q", etagOn, etagOff)
	}
	offAdmin := off.createAdmin()
	var adminOff struct {
		Items       []service.AdminPlanView `json:"items"`
		Memberships bool                    `json:"memberships_enabled"`
	}
	offAdmin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/billing/plans", nil, &adminOff)
	found := false
	for _, p := range adminOff.Items {
		found = found || p.ID == plan.ID
	}
	if adminOff.Memberships || !found {
		t.Fatalf("admin plans while off: memberships_enabled=%v, plan listed=%v", adminOff.Memberships, found)
	}
	offAdmin.stepUp()
	offAdmin.must(http.StatusCreated, http.MethodPost, "/api/v1/admin/billing/plans", map[string]any{"code": uniq("vip"), "name": map[string]string{"en": "VIP"},
		"version": map[string]any{"price_cents": 999, "interval": "month"}}, nil)
}

// -------------------------------------------------- §25.3 statement descriptor

func TestStatementDescriptorInCatalogAndOnEveryCharge(t *testing.T) {
	proc := payment.NewSandboxProcessor()
	e := newEnvWithProcessor(t, proc)
	e.ensurePacks()
	c := e.registerClient()
	var cat service.TopUpCatalog
	c.must(http.StatusOK, http.MethodGet, "/api/v1/credits/catalog", nil, &cat)
	if cat.StatementDescriptor != descriptor {
		t.Fatalf("catalog statement_descriptor: %q", cat.StatementDescriptor)
	}
	p := pack(t, c, 250)
	// Saved card.
	card := c.attachCard("sbx_a_visa_4242_12_2035")
	var saved service.PurchaseView
	c.must(http.StatusCreated, http.MethodPost, purchasesPath, map[string]any{"pack_id": p.ID, "payment_method_id": card.ID,
		"idempotency_key": uniq("purchase-key-"), "consent": purchaseConsent()}, &saved)
	if got := proc.StatementDescriptor("purchase:" + saved.ID.String()); got != descriptor {
		t.Fatalf("saved-card charge descriptor %q", got)
	}
	// Single-use token (token checkout).
	var token service.PurchaseView
	c.must(http.StatusCreated, http.MethodPost, purchasesPath, map[string]any{"pack_id": p.ID, "payment_token": "sbx_a_mastercard_4444_01_2034",
		"provider": "sandbox_a", "idempotency_key": uniq("purchase-key-"), "consent": purchaseConsent()}, &token)
	if got := proc.StatementDescriptor("purchase:" + token.ID.String()); got != descriptor {
		t.Fatalf("token checkout descriptor %q", got)
	}
	// Membership charge.
	_, version := e.billingPlan(999)
	res := c.subscribe(version, card.ID)
	if got := proc.StatementDescriptor("sub:" + res.Subscription.ID.String() + ":p1:a1"); got != descriptor {
		t.Fatalf("membership charge descriptor %q", got)
	}
	// A configured value is the one sent and shown.
	custom := newEnvWithProcessor(t, proc, func(cfg *configAlias) { cfg.Payments.StatementDescriptor = "STARYIELD READINGS" })
	custom.ensurePacks()
	cc := custom.registerClient()
	cc.must(http.StatusOK, http.MethodGet, "/api/v1/credits/catalog", nil, &cat)
	if cat.StatementDescriptor != "STARYIELD READINGS" {
		t.Fatalf("configured descriptor: %q", cat.StatementDescriptor)
	}
	var bought service.PurchaseView
	cc.must(http.StatusCreated, http.MethodPost, purchasesPath, map[string]any{"pack_id": pack(t, cc, 250).ID, "payment_method_id": cc.attachCard("sbx_a_visa_4242_12_2035").ID,
		"idempotency_key": uniq("purchase-key-"), "consent": purchaseConsent()}, &bought)
	if got := proc.StatementDescriptor("purchase:" + bought.ID.String()); got != "STARYIELD READINGS" {
		t.Fatalf("configured descriptor on the charge: %q", got)
	}
}

// ------------------------------------------------------ §25.5 e-mails

func TestPurchaseReceiptEmail(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	c := e.registerClient()
	card := c.attachCard("sbx_a_visa_4242_12_2035")
	p := pack(t, c, 250)
	// Even with every notification e-mail switched off the receipt is sent.
	e.setPrefs(c.user.ID, map[string]any{"notify_email": false})

	var out service.PurchaseView
	c.must(http.StatusCreated, http.MethodPost, purchasesPath, map[string]any{"pack_id": p.ID, "payment_method_id": card.ID, "receipt_email": "Accounting@Example.org",
		"idempotency_key": uniq("purchase-key-"), "consent": purchaseConsent()}, &out)
	m := waitMail(t, e, c.user.Email, "Your receipt: 250 credits")
	for _, want := range []string{"Pack 250", "9.99 USD", "Visa •••• 4242", out.ID.String(), "UTC", descriptor, "One-time payment. No subscription or automatic top-up.",
		"/en/refund-policy", "/en/terms", "Staryield Media LLC", "30 N Gould St, Ste N, Sheridan, WY 82801, United States", "support@staryield.net",
		"Support is available 24/7. We reply within 1–2 days."} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("receipt text lacks %q:\n%s", want, m.Text)
		}
		if !strings.Contains(m.HTML, strings.ReplaceAll(want, "&", "&amp;")) {
			t.Errorf("receipt HTML lacks %q", want)
		}
	}
	if m.Category != "billing" {
		t.Fatalf("category %q", m.Category)
	}
	// The receipt_email copy: another address gets it too, without an account link.
	copyMsg := waitMail(t, e, "accounting@example.org", "Your receipt: 250 credits")
	if strings.Contains(copyMsg.Text, "View your balance") || !strings.Contains(copyMsg.Text, "9.99 USD") || !strings.Contains(copyMsg.Text, descriptor) {
		t.Fatalf("receipt copy:\n%s", copyMsg.Text)
	}
	// The in-app notification still carries the credits and now the receipt facts.
	var page service.Page[service.NotificationView]
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/notifications", nil, &page)
	var data map[string]any
	for _, n := range page.Items {
		if n.Kind == service.NotifyPurchase {
			data = n.Data
		}
	}
	if data == nil || data["credits"] != float64(250) || data["price_cents"] != float64(999) || data["currency"] != "USD" || data["pack_name"] != "Pack 250" ||
		data["card_brand"] != "visa" || data["card_last4"] != "4242" || data["statement_descriptor"] != descriptor || data["completed_at"] == nil {
		t.Fatalf("notification data: %v", data)
	}

	// The account's own address as receipt_email: one e-mail, not two. French account: French receipt.
	e.mail.Reset()
	if err := e.app.DB.Model(&domain.User{}).Where("id = ?", c.user.ID).Update("locale", "fr").Error; err != nil {
		t.Fatal(err)
	}
	c.must(http.StatusCreated, http.MethodPost, purchasesPath, map[string]any{"pack_id": p.ID, "receipt_email": strings.ToUpper(c.user.Email),
		"idempotency_key": uniq("purchase-key-"), "consent": purchaseConsent()}, &out)
	fr := waitMail(t, e, c.user.Email, "Votre reçu : 250 crédits")
	if !strings.Contains(fr.Text, "Ce débit apparaît sur votre relevé de carte sous le nom "+descriptor+".") || strings.Contains(fr.Text, "Visa") {
		t.Fatalf("french receipt (hosted checkout has no card):\n%s", fr.Text)
	}
	time.Sleep(200 * time.Millisecond)
	e.flushMail()
	if n := countMail(e, c.user.Email, "Votre reçu"); n != 1 {
		t.Fatalf("one receipt expected, got %d", n)
	}
}

func TestRefundEmails(t *testing.T) {
	w := newRefundWorld(t)
	e, c, admin := w.e, w.c, w.admin
	e.flushMail()
	e.mail.Reset()

	// A refund request approved: one e-mail (the approval), not a second one for the refund itself.
	req := w.mustRequest(w.txA, "other")
	if r := w.approve(req.ID, 599); r.status != http.StatusOK {
		t.Fatalf("approve: %d %s", r.status, r.body)
	}
	m := waitMail(t, e, c.user.Email, "Update on your refund request")
	for _, want := range []string{"5.99 USD", "Pack 250", "250", descriptor, "5–10 business days", "support@staryield.net", "Staryield Media LLC"} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("approval e-mail lacks %q:\n%s", want, m.Text)
		}
	}
	time.Sleep(300 * time.Millisecond)
	e.flushMail()
	if n := countMail(e, c.user.Email, "refund was issued"); n != 0 {
		t.Fatalf("a refund request approval must not also send the direct-refund e-mail (%d)", n)
	}
	if n := countMail(e, c.user.Email, "Update on your refund request"); n != 1 {
		t.Fatalf("approval e-mails: %d", n)
	}

	// A refund issued from the admin console tells the customer.
	e.mail.Reset()
	admin.stepUp()
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/billing/transactions/"+w.txB.String()+"/refund", map[string]any{"amount_cents": 500, "reason": "Goodwill refund"}, nil)
	m = waitMail(t, e, c.user.Email, "Your refund was issued")
	for _, want := range []string{"5.00 USD", "Pack 600", descriptor, "5–10 business days", "support@staryield.net", "Staryield Media LLC"} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("refund e-mail lacks %q:\n%s", want, m.Text)
		}
	}
	if m.Category != "billing" {
		t.Fatalf("category %q", m.Category)
	}

	// A refund the provider reports (charge.refunded) tells the customer once, even when redelivered.
	e.mail.Reset()
	var ref string
	e.app.DB.Raw("SELECT provider_ref FROM payment_transactions WHERE id = ?", w.txB).Scan(&ref)
	evt := payment.WebhookEvent{ID: "evt_" + uniq("r"), Type: "charge.refunded", ProviderRef: ref, AmountCents: 300}
	for i := 0; i < 2; i++ {
		if r := signedWebhook(e, "sandbox_a", evt, ""); r.status != http.StatusOK {
			t.Fatalf("webhook %d: %d %s", i, r.status, r.body)
		}
	}
	m = waitMail(t, e, c.user.Email, "Your refund was issued")
	if !strings.Contains(m.Text, "3.00 USD") {
		t.Fatalf("provider refund e-mail:\n%s", m.Text)
	}
	time.Sleep(300 * time.Millisecond)
	e.flushMail()
	if n := countMail(e, c.user.Email, "Your refund was issued"); n != 1 {
		t.Fatalf("a redelivered webhook sent %d e-mails", n)
	}
	// In-app too.
	var page service.Page[service.NotificationView]
	c.must(http.StatusOK, http.MethodGet, "/api/v1/me/notifications?limit=50", nil, &page)
	kinds := 0
	for _, n := range page.Items {
		if n.Kind == service.NotifyPaymentRefunded {
			kinds++
		}
	}
	if kinds != 2 {
		t.Fatalf("payment.refunded notifications: %d", kinds)
	}
}

// A membership's receipts are sent for every real charge. The memberships
// switch (§25.1) only stops new memberships: an existing one that renews while
// it is off still gets its renewal e-mail (card-scheme rules require a receipt).
func TestMembershipReceiptsAreSentEvenWhileMembershipsAreOff(t *testing.T) {
	proc := payment.NewSandboxProcessor() // both instances charge the same simulated processor
	e := newEnvWithProcessor(t, proc)
	plan, version := e.billingPlan(999)
	_ = plan
	c := e.registerClient()
	card := c.attachCard("sbx_a_visa_4242_12_2035")
	res := c.subscribe(version, card.ID)

	started := waitMail(t, e, c.user.Email, "Your membership has started")
	for _, want := range []string{"Test Membership", "9.99 USD", descriptor, "Cancel membership", "/en/profile/settings#billing", "Next renewal date",
		res.Subscription.CurrentPeriodEnd.UTC().Format("2006-01-02"), "Support is available 24/7. We reply within 1–2 days."} {
		if !strings.Contains(started.Text, want) {
			t.Errorf("subscription.started lacks %q:\n%s", want, started.Text)
		}
	}
	e.renewAt(*res.Subscription.CurrentPeriodEnd, c)
	renewed := waitMail(t, e, c.user.Email, "Your membership was renewed")
	for _, want := range []string{"Test Membership", "9.99 USD", descriptor, "Cancel membership", "/en/profile/settings#billing", "Billing period", "Next renewal date"} {
		if !strings.Contains(renewed.Text, want) {
			t.Errorf("subscription.renewed lacks %q:\n%s", want, renewed.Text)
		}
	}

	// Switched off: the existing membership still renews, and its receipt is still e-mailed.
	off := newEnvWithProcessor(t, proc, withMemberships(false))
	oc := off.login(c.user.Email, c.pw)
	renewedAgain := *res.Subscription.CurrentPeriodEnd
	off.mail.Reset()
	off.clock.Set(renewedAgain.AddDate(0, 1, 1))
	off.tick(time.Second)
	receipt := waitMail(t, off, c.user.Email, "Your membership was renewed")
	for _, want := range []string{"9.99 USD", descriptor, "Cancel membership"} {
		if !strings.Contains(receipt.Text, want) {
			t.Errorf("renewal receipt while memberships are off lacks %q:\n%s", want, receipt.Text)
		}
	}
	oc.relogin() // a month passed on this instance's clock
	if n := countNotifications(t, oc, service.NotifySubscriptionRenewed); n == 0 {
		t.Fatal("the in-app renewal notification is created too")
	}
}
