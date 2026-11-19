package e2e

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 6.0 (docs/platform-extensions.md §26.5): helpers of the payment
// operations tests (disputes, fraud warnings, webhook log, policies, admin
// pages, purchase stream). Provider events are the signed generic events of
// the sandbox_a provider.

// opsWorld is a client with a sign-up bonus and two card purchases (the first
// partly used), an administrator and the shared sandbox processor.
type opsWorld struct {
	e                    *env
	proc                 *payment.SandboxProcessor
	c, admin             *client
	card                 service.PaymentMethodView
	purchaseA, purchaseB uuid.UUID
	txA, txB             uuid.UUID
	refA, refB           string
}

// newOpsWorld buys a 250-credit pack (A, 999 cents) and a 600-credit pack (B,
// 1999 cents) and spends 100 credits: A keeps 150, B 600, the bonus 50.
func newOpsWorld(t *testing.T, opts ...option) *opsWorld {
	t.Helper()
	proc := payment.NewSandboxProcessor()
	e := newEnvWithProcessor(t, proc, append([]option{withSignupBonus(50)}, opts...)...)
	e.ensurePacks()
	e.disableSignupTrial()
	w := &opsWorld{e: e, proc: proc, c: e.registerClient(), admin: e.createAdmin()}
	w.card = w.c.attachCard("sbx_a_visa_4242_12_2035")
	w.purchaseA, w.txA = w.c.buyWithCard(250, w.card)
	w.purchaseB, w.txB = w.c.buyWithCard(600, w.card)
	if _, err := e.app.Services.Admin.AdjustCredits(context.Background(), w.admin.user.ID, w.c.user.ID, -100, "use", uniq("adj-key-")); err != nil {
		t.Fatal(err)
	}
	w.refA, w.refB = w.providerRef(w.txA), w.providerRef(w.txB)
	if w.refA == "" || w.refB == "" {
		t.Fatalf("payments have no provider reference: %q %q", w.refA, w.refB)
	}
	w.restorePoliciesAtEnd()
	return w
}

func (w *opsWorld) providerRef(tx uuid.UUID) string {
	var ref string
	w.e.app.DB.Raw("SELECT coalesce(provider_ref, '') FROM payment_transactions WHERE id = ?", tx).Scan(&ref)
	return ref
}

func (w *opsWorld) count(query string, args ...any) int64 {
	w.e.t.Helper()
	var n int64
	if err := w.e.app.DB.Raw(query, args...).Scan(&n).Error; err != nil {
		w.e.t.Fatalf("%s: %v", query, err)
	}
	return n
}

func (w *opsWorld) text(query string, args ...any) string {
	w.e.t.Helper()
	var s string
	if err := w.e.app.DB.Raw(query, args...).Scan(&s).Error; err != nil {
		w.e.t.Fatalf("%s: %v", query, err)
	}
	return s
}

// txStatus is the status of a payment.
func (w *opsWorld) txStatus(tx uuid.UUID) string {
	return w.text("SELECT status FROM payment_transactions WHERE id = ?", tx)
}

// event builds a signed-event body about payment A.
func (w *opsWorld) event(typ string, edit ...func(*payment.WebhookEvent)) payment.WebhookEvent {
	evt := payment.WebhookEvent{ID: "evt_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16], Type: typ, ProviderRef: w.refA}
	for _, f := range edit {
		f(&evt)
	}
	return evt
}

func (w *opsWorld) send(evt payment.WebhookEvent) response {
	w.e.t.Helper()
	return signedWebhook(w.e, "sandbox_a", evt, "")
}

func (w *opsWorld) mustSend(evt payment.WebhookEvent) {
	w.e.t.Helper()
	if r := w.send(evt); r.status != http.StatusOK {
		w.e.t.Fatalf("webhook %s: %d %s", evt.Type, r.status, r.body)
	}
}

// disputeOf is the dispute of a provider reference as the database holds it.
type disputeRow struct {
	ID              uuid.UUID
	Status          string
	CreditsFrozen   int64
	CreditsRestored int64
	FundsWithdrawn  bool
	PriorStatus     string
	Reason          string
	AmountCents     int64
	ClosedAt        *time.Time
	ActivatedAt     *time.Time
}

func (w *opsWorld) dispute(ref string) disputeRow {
	w.e.t.Helper()
	var d disputeRow
	err := w.e.app.DB.Raw(`SELECT id, status, credits_frozen, credits_restored, funds_withdrawn, prior_status, reason, amount_cents, closed_at, activated_at
		FROM payment_disputes WHERE provider = 'sandbox_a' AND dispute_ref = ?`, ref).Scan(&d).Error
	if err != nil || d.ID == uuid.Nil {
		w.e.t.Fatalf("dispute %s: %+v %v", ref, d, err)
	}
	return d
}

func openDispute(ref string, status string) func(*payment.WebhookEvent) {
	due := time.Now().Add(7 * 24 * time.Hour).UTC().Truncate(time.Second)
	return func(e *payment.WebhookEvent) {
		e.DisputeRef, e.DisputeStatus, e.DisputeReason, e.AmountCents, e.Currency, e.EvidenceDueBy = ref, status, "fraudulent", 999, "USD", &due
		e.ObjectRef, e.ProviderType = ref, "charge.dispute.created"
	}
}

func closedDispute(ref, status string) func(*payment.WebhookEvent) {
	return func(e *payment.WebhookEvent) {
		e.DisputeRef, e.DisputeStatus, e.ObjectRef, e.ProviderType = ref, status, ref, "charge.dispute.closed"
	}
}

// tryBuy attempts a 250-credit purchase with the saved card.
func (w *opsWorld) tryBuy(c *client) response {
	w.e.t.Helper()
	var packs struct {
		Items []service.PackView `json:"items"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/credit-packs", nil, &packs)
	for _, p := range packs.Items {
		if p.Code == "test_250" {
			return c.call(http.MethodPost, "/api/v1/credits/purchases", map[string]any{"pack_id": p.ID, "payment_method_id": w.card.ID,
				"idempotency_key": uniq("purchase-key-"), "consent": purchaseConsent()})
		}
	}
	w.e.t.Fatal("no test_250 pack")
	return response{}
}

// setPolicies writes the payment policies (the cache of this instance is dropped).
func (w *opsWorld) setPolicies(block, clawback, efw bool, threeDS string) {
	w.e.t.Helper()
	cur, err := w.e.app.Services.Site.PaymentPoliciesSetting(context.Background())
	if err != nil {
		w.e.t.Fatal(err)
	}
	if _, err := w.e.app.Services.Site.UpdatePaymentPolicies(context.Background(), service.PaymentPoliciesInput{BlockPurchasesOnDispute: &block,
		DisputeClawback: &clawback, EFWAutoRefund: &efw, ThreeDSecure: &threeDS, ExpectedVersion: &cur.Version}); err != nil {
		w.e.t.Fatal(err)
	}
}

// restorePoliciesAtEnd puts the seeded policies back when the test is over (the
// database is shared by every test of the package).
func (w *opsWorld) restorePoliciesAtEnd() {
	w.e.t.Cleanup(func() { w.setPolicies(true, true, false, "automatic") })
}

// supportMail returns the subjects of the alert e-mails the support inbox
// received that mention needle (the outbox is shared by every test of the
// package, so alerts of other tests may be delivered here).
func (w *opsWorld) supportMail(needle string) []string {
	w.e.t.Helper()
	var out []string
	for _, m := range w.e.flushMail() {
		if m.To == w.e.app.Config.Mail.SupportInbox && strings.HasPrefix(m.Subject, "[Payments]") && strings.Contains(m.Text, needle) {
			out = append(out, m.Subject)
		}
	}
	return out
}

func hasAudit(w *opsWorld, action string) bool {
	return w.count("SELECT count(*) FROM audit_logs WHERE action = ?", action) > 0
}

func (w *opsWorld) lotRemaining(purchase uuid.UUID) int64 {
	return w.e.purchaseLot(purchase).CreditsRemaining
}

func (w *opsWorld) bonusRemaining() int64 {
	return w.e.lotOf(w.c.user.ID, domain.LotBonus).CreditsRemaining
}
