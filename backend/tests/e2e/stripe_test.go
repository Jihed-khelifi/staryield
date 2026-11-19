package e2e

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment/stripepay/stripetest"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 6.0 (§26.5): the Stripe adapter behind the real API, against the
// fake Stripe (internal/payment/stripepay/stripetest).

const (
	e2eStripeSecret      = "sk_test_e2e_secret_0123456789abcdef"
	e2eStripePublishable = "pk_test_e2e_publishable_0123456789abcdef"
	e2eStripeWebhook     = "whsec_e2e_webhook_secret_0123456789abcdef"
)

// withStripe registers only Stripe, routed by default, against the fake.
func withStripe(apiBase string) option {
	return func(c *config.Config) {
		c.Payments.Providers = []string{"stripe"}
		c.Payments.DefaultProvider = "stripe"
		c.Stripe = config.StripeConfig{SecretKey: e2eStripeSecret, PublishableKey: e2eStripePublishable, WebhookSecret: e2eStripeWebhook,
			APIBase: apiBase, Timeout: 5 * time.Second}
	}
}

// stripeEnv starts a fake Stripe and an app that uses it.
func stripeEnv(t *testing.T) (*env, *stripetest.Server) {
	t.Helper()
	fake := stripetest.New()
	t.Cleanup(fake.Close)
	e := newEnv(t, withStripe(fake.URL))
	e.ensurePacks()
	// Routing rules choose the provider (§12.2); the seeded default rule points at sandbox_a.
	// The database is shared by the suite, so the rule is removed when the test ends.
	name, prio := "Stripe e2e", 1
	rule, err := e.app.Services.Billing.CreateRoutingRule(context.Background(), service.RoutingRuleInput{Name: &name, Priority: &prio,
		Providers: &[]string{"stripe"}})
	if err != nil {
		t.Fatalf("routing rule: %v", err)
	}
	t.Cleanup(func() { _ = e.app.Services.Billing.DeleteRoutingRule(context.Background(), rule.ID) })
	return e, fake
}

func (c *client) packID(credits int64) uuid.UUID {
	c.e.t.Helper()
	var packs struct {
		Items []service.PackView `json:"items"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/credit-packs", nil, &packs)
	for _, p := range packs.Items {
		if p.Credits == credits && !strings.HasPrefix(p.Code, "welcome_") {
			return p.ID
		}
	}
	c.e.t.Fatalf("no %d-credit pack", credits)
	return uuid.Nil
}

// payWith buys a pack with a Stripe token or saved card.
func (c *client) payWith(pack uuid.UUID, extra map[string]any) response {
	c.e.t.Helper()
	body := map[string]any{"provider": "stripe"}
	for k, v := range extra {
		body[k] = v
	}
	return c.buy(pack, body)
}

func (e *env) stripeWebhook(payload []byte, header string) response {
	if header == "" {
		header = stripetest.Sign(e2eStripeWebhook, payload, e.clock.Now())
	}
	return e.client().raw(http.MethodPost, "/api/v1/webhooks/payments/stripe", bytes.NewReader(payload),
		map[string]string{"Content-Type": "application/json", "Stripe-Signature": header})
}

func (e *env) txOf(purchase uuid.UUID) (provider, ref, status string) {
	e.t.Helper()
	var row struct {
		Provider    string
		ProviderRef *string
		Status      string
	}
	if err := e.app.DB.Raw("SELECT provider, provider_ref, status FROM payment_transactions WHERE credit_purchase_id = ?", purchase).Scan(&row).Error; err != nil {
		e.t.Fatal(err)
	}
	if row.ProviderRef != nil {
		ref = *row.ProviderRef
	}
	return row.Provider, ref, row.Status
}

func (c *client) purchaseView(r response) service.PurchaseView {
	c.e.t.Helper()
	var v service.PurchaseView
	r.decode(c.e.t, &v)
	return v
}

func TestStripePurchaseWithAConfirmationTokenCreditsTheWalletOnce(t *testing.T) {
	e, fake := stripeEnv(t)
	c := e.registerClient()
	pack := c.packID(250)
	body := map[string]any{"pack_id": pack, "idempotency_key": uniq("stripe-key-"), "consent": purchaseConsent(),
		"provider": "stripe", "payment_token": "ctoken_ok_visa_4242_FR"}
	r := c.call(http.MethodPost, "/api/v1/credits/purchases", body)
	if r.status != http.StatusCreated {
		t.Fatalf("purchase: %d %s", r.status, r.body)
	}
	p := c.purchaseView(r)
	if p.Status != "succeeded" || p.Provider != "stripe" || p.Wallet == nil || p.Wallet.Balance != 250 || p.NextAction != nil {
		t.Fatalf("purchase: %+v", p)
	}
	provider, ref, status := e.txOf(p.ID)
	if provider != "stripe" || !strings.HasPrefix(ref, "pi_") || status != "succeeded" {
		t.Fatalf("ledger row: %s %s %s", provider, ref, status)
	}

	// What reached Stripe: the contract of §26.5.
	pis := fake.RequestsTo(http.MethodPost, "/v1/payment_intents")
	if len(pis) != 1 {
		t.Fatalf("%d payment intents", len(pis))
	}
	form := pis[0].Form
	txKey := "purchase:" + p.ID.String()
	for k, want := range map[string]string{"amount": "999", "currency": "usd", "confirm": "true", "confirmation_token": "ctoken_ok_visa_4242_FR",
		"metadata[sy_key]": txKey, "metadata[sy_purchase_id]": p.ID.String(), "metadata[sy_user_id]": c.user.ID.String(),
		"payment_method_options[card][request_three_d_secure]": "automatic",
		"return_url": e.app.Config.App.FrontendURL + "/en/credits/return?purchase=" + p.ID.String()} {
		if got := form.Get(k); got != want {
			t.Errorf("payment intent %s = %q, want %q", k, got, want)
		}
	}
	if pis[0].IdempotencyKey() != txKey || form.Get("metadata[sy_tx_id]") == "" || form.Get("customer") == "" || form.Get("receipt_email") != "" {
		t.Errorf("idempotency key %q, tx %q, customer %q, receipt %q", pis[0].IdempotencyKey(), form.Get("metadata[sy_tx_id]"),
			form.Get("customer"), form.Get("receipt_email"))
	}
	// The buyer's customer carries the account e-mail and name.
	customers := fake.Customers()
	if len(customers) != 1 || customers[0].Email != c.user.Email || customers[0].Name != c.user.DisplayName ||
		customers[0].Metadata["sy_user_id"] != c.user.ID.String() || customers[0].ID != form.Get("customer") {
		t.Fatalf("customers: %+v (user %s %q)", customers, c.user.Email, c.user.DisplayName)
	}

	// A replay with the same key returns the purchase and charges nothing more.
	if r = c.call(http.MethodPost, "/api/v1/credits/purchases", body); r.status != http.StatusCreated && r.status != http.StatusOK {
		t.Fatalf("replay: %d %s", r.status, r.body)
	}
	if again := c.purchaseView(r); again.ID != p.ID {
		t.Fatalf("replay created another purchase: %+v", again)
	}
	if len(fake.Intents()) != 1 || c.wallet().Balance != 250 {
		t.Fatalf("a replay must not charge twice: %d intents, balance %d", len(fake.Intents()), c.wallet().Balance)
	}
	// A second purchase reuses the same customer.
	if r = c.payWith(pack, map[string]any{"payment_token": "ctoken_ok_visa_4242_DE"}); r.status != http.StatusCreated {
		t.Fatalf("second purchase: %d %s", r.status, r.body)
	}
	if len(fake.Customers()) != 1 || c.wallet().Balance != 500 {
		t.Fatalf("customers %d, balance %d", len(fake.Customers()), c.wallet().Balance)
	}
}

func TestStripeDeclinesAndInvalidTokens(t *testing.T) {
	e, fake := stripeEnv(t)
	c := e.registerClient()
	pack := c.packID(250)
	for _, tc := range []struct{ token, reason string }{
		{"ctoken_decline_insufficient_funds", "insufficient_funds"},
		{"ctoken_decline_card_declined", "card_declined"},
		{"ctoken_decline_expired_card", "expired_card"},
		{"ctoken_decline_lost_card", "lost_card"},
	} {
		r := c.payWith(pack, map[string]any{"payment_token": tc.token})
		if r.status != http.StatusPaymentRequired || r.errCode(t) != "payment_failed" {
			t.Fatalf("%s: %d %s", tc.token, r.status, r.body)
		}
		if !strings.Contains(string(r.body), `"reason":"`+tc.reason+`"`) {
			t.Fatalf("%s: the decline reason %s must reach the client: %s", tc.token, tc.reason, r.body)
		}
	}
	if w := c.wallet(); w.Balance != 0 {
		t.Fatalf("declines credit nothing: %+v", w)
	}
	var failed int64
	e.app.DB.Raw("SELECT count(*) FROM credit_purchases WHERE user_id = ? AND status = 'failed'", c.user.ID).Scan(&failed)
	if failed != 4 {
		t.Fatalf("failed purchases: %d", failed)
	}
	var ledgerFailed int64
	e.app.DB.Raw("SELECT count(*) FROM payment_transactions WHERE user_id = ? AND status = 'failed' AND provider = 'stripe'", c.user.ID).Scan(&ledgerFailed)
	if ledgerFailed != 4 {
		t.Fatalf("failed ledger rows: %d", ledgerFailed)
	}

	// An expired or unknown ConfirmationToken is the buyer's problem: 422, nothing charged.
	for _, token := range []string{"ctoken_expired", "ctoken_nonsense"} {
		if r := c.payWith(pack, map[string]any{"payment_token": token}); r.status != http.StatusUnprocessableEntity {
			t.Fatalf("%s: %d %s", token, r.status, r.body)
		}
	}
	// A card number is refused before Stripe is called.
	before := len(fake.Requests())
	if r := c.payWith(pack, map[string]any{"payment_token": "4242424242424242"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("card number: %d", r.status)
	}
	if len(fake.Requests()) != before {
		t.Fatal("a card number must be refused without calling Stripe")
	}
	// A rejected processor key is a configuration error: 503, nothing credited, no customer data leaks.
	r := c.payWith(pack, map[string]any{"payment_token": "ctoken_auth"})
	if r.status != http.StatusServiceUnavailable || r.errCode(t) != "provider_unavailable" || strings.Contains(string(r.body), e2eStripeSecret) {
		t.Fatalf("rejected key: %d %s", r.status, r.body)
	}
	// A Stripe outage on a card the processor never saw leaves the purchase pending, never charged twice.
	r = c.payWith(pack, map[string]any{"payment_token": "ctoken_error500"})
	if r.status != http.StatusCreated || c.purchaseView(r).Status != "pending" || c.wallet().Balance != 0 {
		t.Fatalf("outage: %d %s", r.status, r.body)
	}
}

func TestStripeThreeDSecurePendsThenTheWebhookCompletesOnce(t *testing.T) {
	e, fake := stripeEnv(t)
	c := e.registerClient()
	r := c.payWith(c.packID(250), map[string]any{"payment_token": "ctoken_3ds_visa_3155"})
	if r.status != http.StatusCreated {
		t.Fatalf("3-D Secure purchase: %d %s", r.status, r.body)
	}
	p := c.purchaseView(r)
	if p.Status != "pending" || p.NextAction == nil || p.NextAction.Type != "use_stripe_sdk" || !strings.Contains(p.NextAction.ClientSecret, "_secret_") ||
		(p.Wallet != nil && p.Wallet.Balance != 0) {
		t.Fatalf("pending purchase: %+v", p)
	}
	pis := fake.Intents()
	if len(pis) != 1 || pis[0].Status != "requires_action" || !strings.HasPrefix(p.NextAction.ClientSecret, pis[0].ID) {
		t.Fatalf("intents: %+v", pis)
	}
	// The pending ledger row already knows its Stripe payment (webhooks and refunds find it by reference).
	if _, ref, status := e.txOf(p.ID); ref != pis[0].ID || status != "pending" {
		t.Fatalf("pending ledger row: %s %s", ref, status)
	}

	// The browser finishes 3-D Secure; Stripe tells us.
	fake.CompleteAction(pis[0].ID, true)
	event := stripetest.PaymentIntentSucceeded("evt_3ds_"+uuid.NewString()[:8], pis[0].ID, 999, "usd",
		map[string]string{"sy_key": "purchase:" + p.ID.String(), "sy_purchase_id": p.ID.String()})
	if r = e.stripeWebhook(event, ""); r.status != http.StatusOK {
		t.Fatalf("webhook: %d %s", r.status, r.body)
	}
	if w := c.wallet(); w.Balance != 250 {
		t.Fatalf("credited: %+v", w)
	}
	// Redelivered: acknowledged, never applied twice.
	for i := 0; i < 2; i++ {
		if r = e.stripeWebhook(event, ""); r.status != http.StatusOK {
			t.Fatalf("redelivery: %d %s", r.status, r.body)
		}
	}
	// The same payment through a second event id (Stripe resends under a new id): still once.
	again := stripetest.PaymentIntentSucceeded("evt_3ds_other", pis[0].ID, 999, "usd", map[string]string{"sy_key": "purchase:" + p.ID.String()})
	if r = e.stripeWebhook(again, ""); r.status != http.StatusOK {
		t.Fatalf("second event: %d %s", r.status, r.body)
	}
	if w := c.wallet(); w.Balance != 250 {
		t.Fatalf("credited more than once: %+v", w)
	}
	if _, _, status := e.txOf(p.ID); status != "succeeded" {
		t.Fatalf("ledger row: %s", status)
	}
	var credited int64
	e.app.DB.Raw("SELECT count(*) FROM credit_ledger WHERE user_id = ? AND entry_type = 'purchase'", c.user.ID).Scan(&credited)
	if credited != 1 {
		t.Fatalf("purchase ledger entries: %d", credited)
	}
	// A failed payment event for another pending purchase fails it (and credits nothing).
	r = c.payWith(c.packID(250), map[string]any{"payment_token": "ctoken_3ds_mastercard_3155"})
	q := c.purchaseView(r)
	qi, _ := fake.IntentByKey("purchase:" + q.ID.String())
	fail := stripetest.PaymentIntentFailed("evt_fail_1", qi.ID, 999, "usd", map[string]string{"sy_key": "purchase:" + q.ID.String()}, "insufficient_funds")
	if r = e.stripeWebhook(fail, ""); r.status != http.StatusOK {
		t.Fatalf("failed webhook: %d %s", r.status, r.body)
	}
	var st, reason string
	e.app.DB.Raw("SELECT status, failure_reason FROM credit_purchases WHERE id = ?", q.ID).Row().Scan(&st, &reason)
	if st != "failed" || reason != "insufficient_funds" || c.wallet().Balance != 250 {
		t.Fatalf("failed purchase: %s %s balance %d", st, reason, c.wallet().Balance)
	}
}

func TestStripeWebhookRejectsBadSignatures(t *testing.T) {
	e, _ := stripeEnv(t)
	payload := stripetest.PaymentIntentSucceeded("evt_sig", "pi_nothing", 999, "usd", nil)
	now := e.clock.Now()
	for name, header := range map[string]string{
		"wrong secret": stripetest.Sign("whsec_someone_else_0123456789", payload, now),
		"stale":        stripetest.Sign(e2eStripeWebhook, payload, now.Add(-10*time.Minute)),
		"junk":         "t=1,v1=00",
		"scheme only":  "v0=abc",
	} {
		if r := e.stripeWebhook(payload, header); r.status != http.StatusBadRequest {
			t.Errorf("%s: %d %s", name, r.status, r.body)
		}
	}
	// No header at all, and a body changed after signing.
	if r := e.client().raw(http.MethodPost, "/api/v1/webhooks/payments/stripe", bytes.NewReader(payload), map[string]string{"Content-Type": "application/json"}); r.status != http.StatusBadRequest {
		t.Errorf("unsigned: %d", r.status)
	}
	signed := stripetest.Sign(e2eStripeWebhook, payload, now)
	tampered := bytes.Replace(payload, []byte("999"), []byte("1"), 1)
	if r := e.stripeWebhook(tampered, signed); r.status != http.StatusBadRequest {
		t.Errorf("tampered: %d", r.status)
	}
	// A good signature for an unknown payment is acknowledged (Stripe stops retrying) and logged.
	if r := e.stripeWebhook(payload, ""); r.status != http.StatusOK {
		t.Errorf("unknown payment: %d %s", r.status, r.body)
	}
	var rows int64
	e.app.DB.Raw("SELECT count(*) FROM webhook_events WHERE provider = 'stripe' AND event_id = 'evt_sig'").Scan(&rows)
	if rows != 1 {
		t.Fatalf("webhook log rows: %d", rows)
	}
}

func TestStripeLostResponseIsReconciledByLookup(t *testing.T) {
	e, fake := stripeEnv(t)
	c := e.registerClient()
	tick := func(d time.Duration) { e.tick(d); c.relogin() } // minutes pass: the access token expires
	r := c.payWith(c.packID(250), map[string]any{"payment_token": "ctoken_lost_visa_4242"})
	if r.status != http.StatusCreated {
		t.Fatalf("purchase: %d %s", r.status, r.body)
	}
	p := c.purchaseView(r)
	if p.Status != "pending" || p.NextAction != nil {
		t.Fatalf("a lost response stays pending: %+v", p)
	}
	if n := len(fake.Intents()); n != 1 || fake.Intents()[0].Status != "succeeded" {
		t.Fatalf("the money moved at Stripe: %+v", fake.Intents())
	}
	if c.wallet().Balance != 0 {
		t.Fatal("nothing is credited before the outcome is known")
	}
	tick(time.Minute) // the lease is still running
	if c.wallet().Balance != 0 {
		t.Fatal("reconciled before the lease expired")
	}
	// Stripe's search index lags: not found yet, still pending, retried.
	fake.SetSearchLag(true)
	tick(6 * time.Minute)
	if c.wallet().Balance != 0 {
		t.Fatal("credited without knowing the outcome")
	}
	fake.SetSearchLag(false)
	tick(6 * time.Minute)
	if w := c.wallet(); w.Balance != 250 {
		t.Fatalf("reconciled wallet: %+v", w)
	}
	if _, ref, status := e.txOf(p.ID); status != "succeeded" || ref == "" {
		t.Fatalf("ledger row: %s %s", ref, status)
	}
	if len(fake.Intents()) != 1 {
		t.Fatalf("reconciliation must not charge again: %d intents", len(fake.Intents()))
	}
	tick(6 * time.Minute) // nothing more happens
	if c.wallet().Balance != 250 {
		t.Fatalf("credited again: %d", c.wallet().Balance)
	}

	// The same lost response settled by its webhook instead (matched by the idempotency key).
	r = c.payWith(c.packID(250), map[string]any{"payment_token": "ctoken_lost_mastercard_4444"})
	q := c.purchaseView(r)
	qi, found := fake.IntentByKey("purchase:" + q.ID.String())
	if q.Status != "pending" || !found {
		t.Fatalf("second lost response: %+v %v", q, found)
	}
	ev := stripetest.PaymentIntentSucceeded("evt_lost_"+uuid.NewString()[:8], qi.ID, 999, "usd", map[string]string{"sy_key": "purchase:" + q.ID.String()})
	if r = e.stripeWebhook(ev, ""); r.status != http.StatusOK {
		t.Fatalf("webhook: %d %s", r.status, r.body)
	}
	if c.wallet().Balance != 500 {
		t.Fatalf("webhook settlement: %d", c.wallet().Balance)
	}
	tick(6 * time.Minute)
	if c.wallet().Balance != 500 {
		t.Fatalf("webhook and reconciliation must not both credit: %d", c.wallet().Balance)
	}
}

func TestStripeSavedCardsAndPurchasesWithThem(t *testing.T) {
	e, fake := stripeEnv(t)
	c := e.registerClient()
	pack := c.packID(250)

	// Settings → "Add a card": a SetupIntent for the user's customer.
	var cfg struct {
		Provider           string   `json:"provider"`
		Type               string   `json:"type"`
		Mode               string   `json:"mode"`
		PublishableKey     string   `json:"publishable_key"`
		ClientSecret       string   `json:"client_secret"`
		PaymentMethodTypes []string `json:"payment_method_types"`
		Livemode           *bool    `json:"livemode"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/client-config?purpose=method", nil, &cfg)
	if cfg.Provider != "stripe" || cfg.Type != "stripe_elements" || cfg.Mode != "setup" || cfg.PublishableKey != e2eStripePublishable ||
		!strings.Contains(cfg.ClientSecret, "_secret_") || len(cfg.PaymentMethodTypes) != 1 || cfg.Livemode == nil || *cfg.Livemode {
		t.Fatalf("method config: %+v", cfg)
	}
	customers := fake.Customers()
	if len(customers) != 1 || customers[0].Email != c.user.Email {
		t.Fatalf("customers: %+v", customers)
	}
	si := fake.SetupIntentRequests()
	if len(si) != 1 || si[0].Form.Get("customer") != customers[0].ID || si[0].Form.Get("usage") != "on_session" ||
		si[0].Form.Get("payment_method_options[card][request_three_d_secure]") != "automatic" {
		t.Fatalf("setup intent request: %v", si[0].Form)
	}
	// The payment mode (credits) needs no SetupIntent; unknown purposes are refused.
	var pay struct {
		Mode string `json:"mode"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/client-config?purpose=credits", nil, &pay)
	if pay.Mode != "payment" || len(fake.SetupIntentRequests()) != 1 {
		t.Fatalf("credits config: %+v", pay)
	}
	if r := c.call(http.MethodGet, "/api/v1/billing/client-config?purpose=nonsense", nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown purpose: %d", r.status)
	}

	// The browser confirms the SetupIntent; the id of the payment method is posted.
	pm := fake.ConfirmSetupIntent(cfg.ClientSecret, "ok_visa_4242_FR")
	var card service.PaymentMethodView
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/billing/payment-methods", map[string]any{
		"provider": "stripe", "token": pm, "idempotency_key": uniq("card-key-")}, &card)
	if card.Provider != "stripe" || card.Brand != "visa" || card.Last4 != "4242" || !card.IsDefault {
		t.Fatalf("saved card: %+v", card)
	}
	// Someone else's payment method cannot be saved.
	other := e.registerClient()
	var ocfg struct {
		ClientSecret string `json:"client_secret"`
	}
	other.must(http.StatusOK, http.MethodGet, "/api/v1/billing/client-config?purpose=method", nil, &ocfg)
	theirs := fake.ConfirmSetupIntent(ocfg.ClientSecret, "ok_mastercard_4444")
	if r := c.call(http.MethodPost, "/api/v1/billing/payment-methods", map[string]any{"provider": "stripe", "token": theirs, "idempotency_key": uniq("card-key-")}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("another customer's card: %d %s", r.status, r.body)
	}
	if owner, _ := fake.PaymentMethodCustomer(theirs); owner == customers[0].ID {
		t.Fatal("another customer's card must not be attached")
	}
	if len(c.cards()) != 1 {
		t.Fatalf("cards: %+v", c.cards())
	}

	// A purchase with the saved card: customer-initiated, on session.
	r := c.payWith(pack, map[string]any{"payment_method_id": card.ID})
	if r.status != http.StatusCreated {
		t.Fatalf("saved-card purchase: %d %s", r.status, r.body)
	}
	p := c.purchaseView(r)
	if p.Status != "succeeded" || p.Wallet.Balance != 250 {
		t.Fatalf("purchase: %+v", p)
	}
	pi := fake.RequestsTo(http.MethodPost, "/v1/payment_intents")
	last := pi[len(pi)-1].Form
	for k, want := range map[string]string{"customer": customers[0].ID, "payment_method": pm, "off_session": "false", "confirm": "true",
		"metadata[sy_purchase_id]": p.ID.String(), "metadata[sy_user_id]": c.user.ID.String(),
		"return_url": e.app.Config.App.FrontendURL + "/en/credits/return?purchase=" + p.ID.String()} {
		if got := last.Get(k); got != want {
			t.Errorf("stored charge %s = %q, want %q", k, got, want)
		}
	}
	if last.Get("metadata[sy_key]") != "purchase:"+p.ID.String() {
		t.Errorf("sy_key: %q", last.Get("metadata[sy_key]"))
	}

	// A saved card that needs 3-D Secure returns the next action, like a token.
	card3 := fake.ConfirmSetupIntent(cfg.ClientSecret, "3ds_visa_3155")
	var saved3 service.PaymentMethodView
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/billing/payment-methods", map[string]any{
		"provider": "stripe", "token": card3, "idempotency_key": uniq("card-key-")}, &saved3)
	r = c.payWith(pack, map[string]any{"payment_method_id": saved3.ID})
	p3 := c.purchaseView(r)
	if r.status != http.StatusCreated || p3.Status != "pending" || p3.NextAction == nil || p3.NextAction.Type != "use_stripe_sdk" || p3.NextAction.ClientSecret == "" {
		t.Fatalf("saved 3-D Secure card: %d %s", r.status, r.body)
	}
	if c.wallet().Balance != 250 {
		t.Fatalf("nothing is credited before authentication: %d", c.wallet().Balance)
	}
	// A saved card the issuer declines.
	declined := fake.ConfirmSetupIntent(cfg.ClientSecret, "decline_insufficient_funds")
	var savedD service.PaymentMethodView
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/billing/payment-methods", map[string]any{
		"provider": "stripe", "token": declined, "idempotency_key": uniq("card-key-")}, &savedD)
	if r = c.payWith(pack, map[string]any{"payment_method_id": savedD.ID}); r.status != http.StatusPaymentRequired || !strings.Contains(string(r.body), "insufficient_funds") {
		t.Fatalf("declined saved card: %d %s", r.status, r.body)
	}

	// Removing a card detaches it at Stripe.
	c.must(http.StatusNoContent, http.MethodDelete, "/api/v1/billing/payment-methods/"+card.ID.String(), nil, nil)
	if owner, _ := fake.PaymentMethodCustomer(pm); owner != "" {
		t.Fatalf("the removed card is still attached to %q at Stripe", owner)
	}
}

func TestStripeWelcomeOfferIsRefusedForAReusedCard(t *testing.T) {
	e, fake := stripeEnv(t)
	e.isolateCatalog()
	var welcome uuid.UUID
	for _, p := range mustPricing(e, e.client()).Packs {
		if p.Code == "welcome_600" {
			welcome = p.ID
		}
	}
	a := e.registerClient()
	r := a.payWith(welcome, map[string]any{"payment_token": "ctoken_ok_visa_4242_FR"})
	if r.status != http.StatusCreated || a.purchaseView(r).Status != "succeeded" || a.wallet().Balance != 600 {
		t.Fatalf("welcome purchase: %d %s", r.status, r.body)
	}
	// The card was read from the ConfirmationToken before it was charged.
	if len(fake.RequestsTo(http.MethodGet, "/v1/confirmation_tokens/ctoken_ok_visa_4242_FR")) == 0 {
		t.Fatal("the welcome offer must inspect the ConfirmationToken's card")
	}
	// Another account, another token, the same card: refused, nothing charged.
	b := e.registerClient()
	before := len(fake.Intents())
	if reason := offerReason(t, b.payWith(welcome, map[string]any{"payment_token": "ctoken_ok_visa_4242_DE"})); reason != "payment_method_used" {
		t.Fatalf("reused card: %s", reason)
	}
	if len(fake.Intents()) != before || b.wallet().Balance != 0 || !e.catalog(b).WelcomeOffer.Eligible {
		t.Fatalf("a refused offer must charge nothing and leave the account eligible (%d intents)", len(fake.Intents()))
	}
	// A different card on that account is fine.
	if r = b.payWith(welcome, map[string]any{"payment_token": "ctoken_ok_mastercard_4444_US"}); r.status != http.StatusCreated {
		t.Fatalf("another card: %d %s", r.status, r.body)
	}
	// A declined welcome purchase releases the offer.
	c := e.registerClient()
	if r = c.payWith(welcome, map[string]any{"payment_token": "ctoken_decline_card_declined"}); r.status == http.StatusCreated {
		t.Fatalf("a declined card: %d", r.status)
	}
	if !e.catalog(c).WelcomeOffer.Eligible {
		t.Fatal("a declined welcome purchase must leave the offer available")
	}
}

func TestStripeCustomerIsDeletedAtAccountClosure(t *testing.T) {
	e, fake := stripeEnv(t)
	c := e.registerClient()
	c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/client-config?purpose=method", nil, &struct{}{})
	if cs := fake.Customers(); len(cs) != 1 || cs[0].Deleted {
		t.Fatalf("customers: %+v", cs)
	}
	for i := 0; i < 2; i++ { // closure may be retried
		if err := e.app.Services.Billing.CancelForClosure(context.Background(), c.user.ID); err != nil {
			t.Fatalf("closure %d: %v", i, err)
		}
	}
	if cs := fake.Customers(); len(cs) != 1 || !cs[0].Deleted {
		t.Fatalf("the Stripe customer must be deleted at closure: %+v", cs)
	}
}

func TestStripeRoutingAndRegistration(t *testing.T) {
	e, _ := stripeEnv(t)
	c := e.registerClient()
	var cfg struct {
		Provider string `json:"provider"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/client-config", nil, &cfg)
	if cfg.Provider != "stripe" {
		t.Fatalf("default routing: %+v", cfg)
	}
	admin := e.createAdmin()
	var providers struct {
		Items []service.ProviderView `json:"items"`
	}
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/billing/providers", nil, &providers)
	found := false
	for _, p := range providers.Items {
		if p.Name == "stripe" {
			found = true
			if p.Sandbox || len(p.Capabilities) < 10 {
				t.Fatalf("stripe provider: %+v", p)
			}
		}
	}
	if !found {
		t.Fatalf("stripe is not registered: %+v", providers.Items)
	}
	// Sandbox providers are not registered when only stripe is listed.
	if r := c.call(http.MethodPost, "/api/v1/billing/payment-methods", map[string]any{"provider": "sandbox_a", "token": "sbx_a_visa_4242_12_2035",
		"idempotency_key": uniq("card-key-")}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("sandbox provider: %d %s", r.status, r.body)
	}
}
