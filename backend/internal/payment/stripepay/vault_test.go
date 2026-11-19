package stripepay_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment/stripepay"
)

func TestEnsureCustomerCreatesConfirmsAndRecreates(t *testing.T) {
	f := newFixture(t)
	uid := uuid.New()
	ref, err := f.p.EnsureCustomer(ctx(), payment.CustomerRequest{UserID: uid, IdempotencyKey: "cus:u:stripe", Email: "ada@example.com", Name: "Ada L"})
	if err != nil || !strings.HasPrefix(ref, "cus_") {
		t.Fatalf("create: %q %v", ref, err)
	}
	r := f.lastPost("/v1/customers")
	if r.IdempotencyKey() != "cus:u:stripe" || r.Form.Get("email") != "ada@example.com" || r.Form.Get("name") != "Ada L" ||
		r.Form.Get("metadata[sy_user_id]") != uid.String() {
		t.Fatalf("customer form: %v key %q", r.Form, r.IdempotencyKey())
	}
	// A known reference is confirmed without creating another customer.
	f.fake.ResetRequests()
	again, err := f.p.EnsureCustomer(ctx(), payment.CustomerRequest{UserID: uid, ExistingRef: ref, Email: "ada@example.com", Name: "Ada L"})
	if err != nil || again != ref || len(f.fake.RequestsTo("POST", "/v1/customers")) != 0 || len(f.fake.RequestsTo("POST", "/v1/customers/*")) != 0 {
		t.Fatalf("confirm: %q %v %d requests", again, err, len(f.fake.Requests()))
	}
	// A changed e-mail is pushed to Stripe.
	if _, err = f.p.EnsureCustomer(ctx(), payment.CustomerRequest{UserID: uid, ExistingRef: ref, Email: "new@example.com", Name: "Ada L"}); err != nil {
		t.Fatal(err)
	}
	if got := f.lastPost("/v1/customers/" + ref).Form.Get("email"); got != "new@example.com" {
		t.Fatalf("e-mail update: %q", got)
	}
	// Deleted at Stripe: a new customer replaces it (and never replays the old key).
	if err = f.p.DeleteCustomer(ctx(), ref); err != nil {
		t.Fatal(err)
	}
	fresh, err := f.p.EnsureCustomer(ctx(), payment.CustomerRequest{UserID: uid, ExistingRef: ref, IdempotencyKey: "cus:u:stripe", Email: "ada@example.com"})
	if err != nil || fresh == ref {
		t.Fatalf("recreate: %q %v", fresh, err)
	}
	// Unknown at Stripe likewise.
	if other, err := f.p.EnsureCustomer(ctx(), payment.CustomerRequest{UserID: uid, ExistingRef: "cus_missing"}); err != nil || other == "cus_missing" {
		t.Fatalf("unknown customer: %q %v", other, err)
	}
}

func TestDeleteCustomerIsIdempotent(t *testing.T) {
	f := newFixture(t)
	ref := f.customer()
	if err := f.p.DeleteCustomer(ctx(), ref); err != nil {
		t.Fatal(err)
	}
	if err := f.p.DeleteCustomer(ctx(), ref); err != nil {
		t.Fatalf("second delete: %v", err)
	}
	if cs := f.fake.Customers(); len(cs) != 1 || !cs[0].Deleted {
		t.Fatalf("customers: %+v", cs)
	}
	if err := f.p.DeleteCustomer(ctx(), "pm_nope"); !errors.Is(err, payment.ErrNotFound) {
		t.Fatalf("a non-customer reference: %v", err)
	}
}

func TestAttachChecksOwnership(t *testing.T) {
	f := newFixture(t)
	me, other := f.customer(), f.customer()
	// A payment method a confirmed SetupIntent attached to me.
	owned := f.fake.AddPaymentMethod(me, "ok_visa_4242_FR")
	f.fake.ResetRequests()
	ref, meta, err := f.p.Attach(ctx(), payment.AttachRequest{CustomerRef: me, ClientToken: owned, IdempotencyKey: "pm:1"})
	if err != nil || ref != owned || meta.Brand != "visa" || meta.Last4 != "4242" || meta.Country != "FR" || meta.Fingerprint == "" {
		t.Fatalf("owned: %q %+v %v", ref, meta, err)
	}
	if len(f.fake.RequestsTo("POST", "/v1/payment_methods/*")) != 0 {
		t.Fatal("an owned payment method must not be attached again")
	}
	// Unattached: attached now, with our idempotency key.
	loose := f.fake.AddPaymentMethod("", "ok_mastercard_4444")
	if ref, _, err = f.p.Attach(ctx(), payment.AttachRequest{CustomerRef: me, ClientToken: loose, IdempotencyKey: "pm:2"}); err != nil || ref != loose {
		t.Fatalf("unattached: %q %v", ref, err)
	}
	if r := f.lastPost("/v1/payment_methods/" + loose + "/attach"); r.IdempotencyKey() != "pm:2" || r.Form.Get("customer") != me {
		t.Fatalf("attach request: %v %q", r.Form, r.IdempotencyKey())
	}
	if owner, _ := f.fake.PaymentMethodCustomer(loose); owner != me {
		t.Fatalf("owner: %q", owner)
	}
	// Someone else's card, unknown ids and wrong shapes are invalid tokens.
	theirs := f.fake.AddPaymentMethod(other, "ok_visa_4242")
	for _, tok := range []string{theirs, "pm_missing", "ctoken_ok_visa_4242", "tok_visa", ""} {
		if _, _, err = f.p.Attach(ctx(), payment.AttachRequest{CustomerRef: me, ClientToken: tok}); !errors.Is(err, payment.ErrInvalidToken) {
			t.Errorf("token %q: %v", tok, err)
		}
	}
	if owner, _ := f.fake.PaymentMethodCustomer(theirs); owner != other {
		t.Fatal("another customer's card must stay theirs")
	}
}

func TestAttachAndInspectReportTheSameFingerprint(t *testing.T) {
	f := newFixture(t)
	me := f.customer()
	pm := f.fake.AddPaymentMethod(me, "ok_visa_4242")
	_, attached, err := f.p.Attach(ctx(), payment.AttachRequest{CustomerRef: me, ClientToken: pm})
	if err != nil {
		t.Fatal(err)
	}
	inspected, err := f.p.InspectToken(ctx(), "ctoken_ok_visa_4242")
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Fingerprint == "" || inspected.Fingerprint != attached.Fingerprint || inspected.Last4 != "4242" {
		t.Fatalf("inspect %+v attach %+v", inspected, attached)
	}
	if other, _ := f.p.InspectToken(ctx(), "ctoken_ok_visa_4444"); other.Fingerprint == inspected.Fingerprint {
		t.Fatal("different cards must have different fingerprints")
	}
	if viaPM, err := f.p.InspectToken(ctx(), pm); err != nil || viaPM.Fingerprint != attached.Fingerprint {
		t.Fatalf("inspect a payment method: %+v %v", viaPM, err)
	}
	// Inspecting never charges.
	if len(f.fake.Intents()) != 0 {
		t.Fatal("inspection created a payment")
	}
}

func TestInspectTokenRefusesUsedExpiredAndForeignTokens(t *testing.T) {
	f := newFixture(t)
	if _, err := f.p.Checkout(ctx(), checkoutRequest("ctoken_ok_visa_4242")); err != nil {
		t.Fatal(err)
	}
	for _, tok := range []string{"ctoken_ok_visa_4242", "ctoken_expired", "tok_x", "ctoken_nonsense", ""} {
		if _, err := f.p.InspectToken(ctx(), tok); err == nil {
			t.Errorf("token %q was accepted", tok)
		} else if tok != "ctoken_nonsense" && !errors.Is(err, payment.ErrInvalidToken) {
			t.Errorf("token %q: %v", tok, err)
		}
	}
}

func TestDetach(t *testing.T) {
	f := newFixture(t)
	me := f.customer()
	pm := f.fake.AddPaymentMethod(me, "ok_visa_4242")
	if err := f.p.Detach(ctx(), me, pm); err != nil {
		t.Fatal(err)
	}
	if owner, _ := f.fake.PaymentMethodCustomer(pm); owner != "" {
		t.Fatalf("still attached to %q", owner)
	}
	if err := f.p.Detach(ctx(), me, pm); err != nil {
		t.Fatalf("detaching twice: %v", err)
	}
	if err := f.p.Detach(ctx(), me, "pm_missing"); err != nil {
		t.Fatalf("a missing payment method counts as removed: %v", err)
	}
	if err := f.p.Detach(ctx(), me, "card_123"); !errors.Is(err, payment.ErrNotFound) {
		t.Fatalf("wrong shape: %v", err)
	}
}

func TestClientConfigPaymentAndSetupModes(t *testing.T) {
	f := newFixture(t)
	for _, purpose := range []string{payment.PurposeCredits, payment.PurposeSubscription} {
		cfg, err := f.p.ClientConfig(ctx(), payment.ClientConfigRequest{UserID: uuid.New(), Purpose: purpose, AmountCents: 999, Currency: "USD"})
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Provider != "stripe" || cfg.Type != "stripe_elements" || cfg.PublishableKey != testPublishableKey || cfg.Mode != "payment" ||
			cfg.AmountCents != 999 || cfg.Currency != "usd" || len(cfg.PaymentMethodTypes) != 1 || cfg.PaymentMethodTypes[0] != "card" ||
			cfg.Livemode == nil || *cfg.Livemode || cfg.ClientSecret != "" {
			t.Fatalf("%s config: %+v", purpose, cfg)
		}
	}
	if len(f.fake.Requests()) != 0 {
		t.Fatal("the payment mode needs no Stripe call")
	}
	// Setup mode: a SetupIntent for the user's customer.
	me := f.customer()
	uid := uuid.New()
	cfg, err := f.p.ClientConfig(ctx(), payment.ClientConfigRequest{UserID: uid, Purpose: payment.PurposeMethod, CustomerRef: me,
		ThreeDSecure: payment.ThreeDSAny})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != "setup" || !strings.Contains(cfg.ClientSecret, "_secret_") || cfg.PublishableKey != testPublishableKey || cfg.Livemode == nil {
		t.Fatalf("setup config: %+v", cfg)
	}
	r := f.lastPost("/v1/setup_intents")
	for k, want := range map[string]string{"customer": me, "usage": "on_session", "metadata[sy_user_id]": uid.String(),
		"payment_method_options[card][request_three_d_secure]": "any"} {
		if got := r.Form.Get(k); got != want {
			t.Errorf("setup form %s = %q, want %q", k, got, want)
		}
	}
	if !hasValue(r.Form, "card") {
		t.Errorf("setup intents are card only: %v", r.Form)
	}
	// What the browser then confirms attaches to the same customer.
	pm := f.fake.ConfirmSetupIntent(cfg.ClientSecret, "ok_visa_4242")
	if _, _, err := f.p.Attach(ctx(), payment.AttachRequest{CustomerRef: me, ClientToken: pm}); err != nil {
		t.Fatalf("attach the confirmed card: %v", err)
	}
	if _, err := f.p.ClientConfig(ctx(), payment.ClientConfigRequest{Purpose: payment.PurposeMethod}); err == nil {
		t.Fatal("saving a card without a customer must fail")
	}
}

func TestLiveKeysReportLivemode(t *testing.T) {
	f := newFixture(t, func(o *stripepayOptions) {
		o.SecretKey, o.PublishableKey = "sk_live_unit_secret_0123456789", "pk_live_unit_publishable_0123456789"
	})
	cfg, _ := f.p.ClientConfig(ctx(), payment.ClientConfigRequest{Purpose: payment.PurposeCredits, Currency: "USD"})
	if cfg.Livemode == nil || !*cfg.Livemode || !f.p.Livemode() {
		t.Fatalf("live keys: %+v", cfg)
	}
}

func TestNewValidatesKeys(t *testing.T) {
	ok := stripepay.Options{SecretKey: testSecretKey, PublishableKey: testPublishableKey, WebhookSecret: testWebhookSecret}
	if _, err := stripepay.New(ok); err != nil {
		t.Fatal(err)
	}
	for name, mod := range map[string]func(*stripepay.Options){
		"no secret":      func(o *stripepay.Options) { o.SecretKey = "" },
		"no publishable": func(o *stripepay.Options) { o.PublishableKey = "" },
		"no webhook":     func(o *stripepay.Options) { o.WebhookSecret = "" },
		"mixed modes":    func(o *stripepay.Options) { o.SecretKey = "sk_live_0123456789abcdef" },
	} {
		o := ok
		mod(&o)
		if _, err := stripepay.New(o); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
