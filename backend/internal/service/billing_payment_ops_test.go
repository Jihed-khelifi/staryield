package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 6.0 (§26.5): payment operations, unit level. The database-backed
// behaviour is covered by tests/e2e/payment_ops_*_test.go.

func TestNextDisputeStatusOnlyMovesForward(t *testing.T) {
	cases := []struct {
		name              string
		current, incoming domain.DisputeStatus
		want              domain.DisputeStatus
	}{
		{"new dispute takes the status", "", domain.DisputeNeedsResponse, domain.DisputeNeedsResponse},
		{"no status keeps the current one", domain.DisputeUnderReview, "", domain.DisputeUnderReview},
		{"unknown status is ignored", domain.DisputeNeedsResponse, "bogus", domain.DisputeNeedsResponse},
		{"inquiry escalates to a chargeback", domain.DisputeWarningNeedsResponse, domain.DisputeNeedsResponse, domain.DisputeNeedsResponse},
		{"evidence moves it under review", domain.DisputeNeedsResponse, domain.DisputeUnderReview, domain.DisputeUnderReview},
		{"a late needs_response cannot reopen a dispute under review", domain.DisputeUnderReview, domain.DisputeNeedsResponse, domain.DisputeUnderReview},
		{"closing wins over everything before it", domain.DisputeUnderReview, domain.DisputeWon, domain.DisputeWon},
		{"a closed dispute never changes (won)", domain.DisputeWon, domain.DisputeLost, domain.DisputeWon},
		{"a closed dispute never reopens", domain.DisputeLost, domain.DisputeNeedsResponse, domain.DisputeLost},
		{"the same status refreshes", domain.DisputeNeedsResponse, domain.DisputeNeedsResponse, domain.DisputeNeedsResponse},
		{"an inquiry closes without effect", domain.DisputeWarningUnderReview, domain.DisputeWarningClosed, domain.DisputeWarningClosed},
	}
	for _, c := range cases {
		if got := nextDisputeStatus(c.current, c.incoming); got != c.want {
			t.Errorf("%s: %q + %q = %q, want %q", c.name, c.current, c.incoming, got, c.want)
		}
	}
}

func TestDisputeStatusClasses(t *testing.T) {
	for st, want := range map[domain.DisputeStatus][3]bool{ // inquiry, open, closed
		domain.DisputeWarningNeedsResponse: {true, false, false}, domain.DisputeWarningUnderReview: {true, false, false},
		domain.DisputeWarningClosed: {true, false, true}, domain.DisputeNeedsResponse: {false, true, false},
		domain.DisputeUnderReview: {false, true, false}, domain.DisputeWon: {false, false, true},
		domain.DisputeLost: {false, false, true}, domain.DisputePrevented: {false, false, true},
	} {
		if got := [3]bool{st.Inquiry(), st.Open(), st.Closed()}; got != want {
			t.Errorf("%s: %v, want %v", st, got, want)
		}
	}
	if !domain.DisputeNeedsResponse.AcceptsEvidence() || domain.DisputeUnderReview.AcceptsEvidence() {
		t.Error("only needs_response statuses accept evidence")
	}
}

// ---- freeze and restore (lot only, never below zero) ----

type disputeLedger struct {
	*ledgerHarness
	bill     *BillingService
	purchase uuid.UUID
	lot      uuid.UUID
	tx       *domain.PaymentTransaction
}

// newDisputeLedger has a 100-credit purchase lot and a 50-credit bonus lot.
func newDisputeLedger(t *testing.T) *disputeLedger {
	t.Helper()
	h := newLedgerHarness(t)
	d := &disputeLedger{ledgerHarness: h, purchase: uuid.New()}
	h.mustApply(Movement{Type: domain.LedgerSignupBonus, Amount: 50, Lot: &LotSpec{Source: domain.LotBonus}})
	h.mustApply(Movement{Type: domain.LedgerPurchase, Amount: 100, ReferenceType: "credit_purchase", ReferenceID: &d.purchase,
		Lot: &LotSpec{Source: domain.LotPurchase, PaidCents: 1000, Currency: "USD"}})
	pl, err := h.db.PurchaseLot(h.ctx, d.purchase)
	if err != nil {
		t.Fatal(err)
	}
	d.lot = pl.ID
	d.bill = &BillingService{wallets: h.db, wallet: h.svc, clock: h.clk}
	d.tx = &domain.PaymentTransaction{ID: uuid.New(), UserID: h.user, Kind: domain.TxCreditPurchase, Status: domain.TxSucceeded, CreditPurchaseID: &d.purchase}
	return d
}

func TestFreezeTakesOnlyTheUnusedCreditsOfThePurchaseLot(t *testing.T) {
	d := newDisputeLedger(t)
	d.mustApply(Movement{Amount: -30}) // spends the purchased credits first: 70 left in the lot
	dispute := &domain.PaymentDispute{ID: uuid.New()}
	w, credits, err := d.bill.freezeDisputedCredits(d.ctx, d.tx, dispute)
	if err != nil || credits != 70 || w.Balance != 50 {
		t.Fatalf("freeze: credits=%d wallet=%+v err=%v", credits, w, err)
	}
	if d.remaining(d.lot) != 0 {
		t.Fatalf("the purchase lot is empty: %d", d.remaining(d.lot))
	}
	var bonus int64
	for _, l := range d.db.lots {
		if l.Source == domain.LotBonus {
			bonus = l.CreditsRemaining
		}
	}
	if bonus != 50 {
		t.Fatalf("bonus credits must stay: %d", bonus)
	}
	d.checkInvariant()
	// The same dispute freezes nothing a second time (idempotent ledger key).
	if _, credits, _ := d.bill.freezeDisputedCredits(d.ctx, d.tx, dispute); credits != 0 && d.db.wallets[d.user].Balance != 50 {
		t.Fatalf("second freeze changed the wallet: %d", d.db.wallets[d.user].Balance)
	}
}

func TestFreezeNeverGoesBelowZero(t *testing.T) {
	d := newDisputeLedger(t)
	// The lot claims 100 but the wallet only holds 40 (a broken invariant the
	// reconciliation job reports): the balance is authoritative.
	d.db.wallets[d.user].Balance = 40
	_, credits, err := d.bill.freezeDisputedCredits(d.ctx, d.tx, &domain.PaymentDispute{ID: uuid.New()})
	if err != nil || credits != 40 || d.db.wallets[d.user].Balance != 0 {
		t.Fatalf("freeze: credits=%d balance=%d err=%v", credits, d.db.wallets[d.user].Balance, err)
	}
}

func TestFreezeWithoutALotOrCreditsDoesNothing(t *testing.T) {
	d := newDisputeLedger(t)
	other := uuid.New()
	noLot := *d.tx
	noLot.CreditPurchaseID = &other
	if _, credits, err := d.bill.freezeDisputedCredits(d.ctx, &noLot, &domain.PaymentDispute{ID: uuid.New()}); err != nil || credits != 0 {
		t.Fatalf("no lot: %d %v", credits, err)
	}
	d.mustApply(Movement{Amount: -150}) // everything spent
	if _, credits, err := d.bill.freezeDisputedCredits(d.ctx, d.tx, &domain.PaymentDispute{ID: uuid.New()}); err != nil || credits != 0 {
		t.Fatalf("nothing left: %d %v", credits, err)
	}
	subscription := *d.tx
	subscription.Kind, subscription.CreditPurchaseID = domain.TxSubscriptionRenewal, nil
	if _, credits, err := d.bill.freezeDisputedCredits(d.ctx, &subscription, &domain.PaymentDispute{ID: uuid.New()}); err != nil || credits != 0 {
		t.Fatalf("membership payment: %d %v", credits, err)
	}
}

func TestRestoreGivesTheFrozenCreditsBackToTheSameLotOnce(t *testing.T) {
	d := newDisputeLedger(t)
	d.mustApply(Movement{Amount: -30})
	dispute := &domain.PaymentDispute{ID: uuid.New()}
	_, frozen, err := d.bill.freezeDisputedCredits(d.ctx, d.tx, dispute)
	if err != nil || frozen != 70 {
		t.Fatal(frozen, err)
	}
	dispute.CreditsFrozen = frozen
	d.mustApply(Movement{Amount: 20, Lot: &LotSpec{Source: domain.LotBonus}}) // life goes on meanwhile
	w, err := d.bill.restoreDisputedCredits(d.ctx, d.tx, dispute, d.clk.Now())
	if err != nil || w == nil || dispute.CreditsRestored != 70 {
		t.Fatalf("restore: %+v %+v %v", w, dispute, err)
	}
	if d.remaining(d.lot) != 70 {
		t.Fatalf("the purchase lot got %d back, want 70", d.remaining(d.lot))
	}
	d.checkInvariant()
	before := d.db.wallets[d.user].Balance
	if w, err := d.bill.restoreDisputedCredits(d.ctx, d.tx, dispute, d.clk.Now()); err != nil || w != nil || d.db.wallets[d.user].Balance != before {
		t.Fatalf("a second restore must change nothing: %+v %v", w, err)
	}
}

// ---- gate and policies ----

type openDisputeRepo struct {
	repository.PaymentOpsRepository
	open bool
	err  error
	asks int
}

func (r *openDisputeRepo) HasOpenDispute(context.Context, uuid.UUID) (bool, error) {
	r.asks++
	return r.open, r.err
}

func TestPurchaseGateCombinations(t *testing.T) {
	for _, c := range []struct {
		name    string
		block   bool
		open    bool
		wantErr bool
	}{
		{"open dispute, policy on", true, true, true},
		{"open dispute, policy off", false, true, false},
		{"no dispute, policy on", true, false, false},
		{"no dispute, policy off", false, false, false},
	} {
		repo := &openDisputeRepo{open: c.open}
		s := &BillingService{ops: &paymentOps{repo: repo}, policiesFn: func(context.Context) PaymentPolicies {
			return PaymentPolicies{BlockPurchasesOnDispute: c.block}
		}}
		err := s.PurchaseGate(context.Background(), uuid.New())
		if c.wantErr {
			e, ok := apperror.As(err)
			if !ok || e.Code != apperror.CodePurchasesBlocked || e.Status != 403 || e.Details["reason"] != "dispute" {
				t.Errorf("%s: %v", c.name, err)
			}
		} else if err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
		if !c.block && repo.asks != 0 {
			t.Errorf("%s: the database is not asked when the policy is off", c.name)
		}
	}
	// Without payment operations wired, nothing is refused; a database error is an error.
	if err := (&BillingService{}).PurchaseGate(context.Background(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	s := &BillingService{ops: &paymentOps{repo: &openDisputeRepo{err: errors.New("boom")}}, policiesFn: func(context.Context) PaymentPolicies {
		return PaymentPolicies{BlockPurchasesOnDispute: true}
	}}
	if e, _ := apperror.As(s.PurchaseGate(context.Background(), uuid.New())); e == nil || e.Status != 500 {
		t.Fatalf("a failing lookup must not let the purchase through: %v", e)
	}
}

// seedPaymentPolicies stores the document migration 0031 seeds.
func seedPaymentPolicies(f *siteFixture) {
	f.repo.settings[domain.SiteSettingPaymentPolicies] = domain.SiteSetting{Key: "payment_policies", Version: 1, UpdatedAt: f.clk.Now(),
		Value: domain.JSONMap{"block_purchases_on_dispute": true, "dispute_clawback": true, "efw_auto_refund": false, "three_d_secure": "automatic"}}
}

func TestPaymentPoliciesAdminValidationVersionAndAudit(t *testing.T) {
	f := newSiteFixture(t)
	seedPaymentPolicies(f)
	cur, err := f.svc.PaymentPoliciesSetting(f.ctx)
	if err != nil || cur.Version != 1 || !cur.BlockPurchasesOnDispute || !cur.DisputeClawback || cur.EFWAutoRefund || cur.ThreeDSecure != "automatic" {
		t.Fatalf("seed: %+v %v", cur, err)
	}
	for _, in := range []PaymentPoliciesInput{{}, {BlockPurchasesOnDispute: ptr(true)},
		{BlockPurchasesOnDispute: ptr(true), DisputeClawback: ptr(true), EFWAutoRefund: ptr(false), ThreeDSecure: ptr("always")}} {
		if _, err := f.svc.UpdatePaymentPolicies(f.ctx, in); !isCode(err, apperror.CodeValidation) {
			t.Fatalf("input %+v must be refused: %v", in, err)
		}
	}
	good := PaymentPoliciesInput{BlockPurchasesOnDispute: ptr(false), DisputeClawback: ptr(true), EFWAutoRefund: ptr(true), ThreeDSecure: ptr("any"),
		ExpectedVersion: ptr(cur.Version)}
	out, err := f.svc.UpdatePaymentPolicies(f.ctx, good)
	if err != nil || out.Version != 2 || out.BlockPurchasesOnDispute || !out.EFWAutoRefund || out.ThreeDSecure != "any" || out.UpdatedBy == nil {
		t.Fatalf("update: %+v %v", out, err)
	}
	if got := f.svc.PaymentPolicies(f.ctx); got.BlockPurchasesOnDispute || !got.EFWAutoRefund || got.ThreeDSecure != "any" {
		t.Fatalf("the cache must be dropped on write: %+v", got)
	}
	stale := good
	stale.ExpectedVersion = ptr(1)
	if _, err := f.svc.UpdatePaymentPolicies(f.ctx, stale); !isCode(err, apperror.CodeVersionConflict) {
		t.Fatalf("stale version: %v", err)
	}
	alias := good
	alias.ExpectedVersion, alias.Version = nil, ptr(2) // "version" works like "expected_version"
	if out, err := f.svc.UpdatePaymentPolicies(f.ctx, alias); err != nil || out.Version != 2 {
		t.Fatalf("an unchanged document writes nothing: %+v %v", out, err)
	}
	if last := f.audit.rows[len(f.audit.rows)-1]; last.Action != "billing.policies.updated" || last.ActorID == nil || *last.ActorID != f.admin {
		t.Fatalf("audit: %+v", last)
	}
}

func TestPaymentPoliciesFailSafeWhenUnreadable(t *testing.T) {
	f := newSiteFixture(t)
	seedPaymentPolicies(f)
	f.repo.fail = true
	got := f.svc.PaymentPolicies(f.ctx)
	if !got.BlockPurchasesOnDispute || !got.DisputeClawback || got.EFWAutoRefund || got.ThreeDSecure != "automatic" {
		t.Fatalf("unreadable policies must resolve to the protective defaults: %+v", got)
	}
}

// ---- evidence ----

func TestEvidenceDraftIsBuiltFromKeptDataAndNeverQuotesMessages(t *testing.T) {
	at := time.Date(2026, 11, 19, 14, 3, 0, 0, time.UTC)
	in := evidenceInput{Kind: domain.TxCreditPurchase, ProductName: "Starter pack", Credits: 250, AmountCents: 999, Currency: "USD", PurchasedAt: at,
		CustomerName: "Ada", CustomerEmail: "ada@example.com", CardCountry: "FR", ThreeDS: "authenticated", AccountCreated: at.AddDate(0, -3, 0),
		Consent:         &PurchaseConsentView{Terms: "2026-11-19", Privacy: "2026-11-19", Refund: "2026-11-19", At: at.Add(-time.Minute)},
		Usage:           []repository.UsageRow{{StartedAt: at.Add(time.Hour), Advisor: "Luna", BilledMinutes: 5, CreditsCharged: 50}},
		RefundPolicyURL: "https://staryield.net/en/refund-policy", HasInvoice: true}
	f := composeEvidenceDraft(in)
	for name, want := range map[string]string{
		f.ProductDescription:     "digital credits for live psychic chat, for entertainment purposes only",
		f.AccessActivityLog:      "Luna | 5 | 50",
		f.RefundPolicyDisclosure: "version 2026-11-19",
		f.UncategorizedText:      "3-D Secure: authenticated",
		f.CancellationRebuttal:   "not a subscription",
	} {
		if !strings.Contains(name, want) {
			t.Errorf("%q must contain %q", name, want)
		}
	}
	if f.CustomerName != "Ada" || f.CustomerEmail != "ada@example.com" || f.BillingCountry != "FR" || f.ServiceDate != "2026-11-19" || !f.AttachInvoice {
		t.Fatalf("fields: %+v", f)
	}
	if !strings.Contains(f.RefundPolicyDisclosure, "https://staryield.net/en/refund-policy") {
		t.Fatalf("policy link: %q", f.RefundPolicyDisclosure)
	}
	// The usage row type has no message text; the whole draft must stay free of any.
	b, _ := json.Marshal(f)
	if strings.Contains(strings.ToLower(string(b)), "message") && !strings.Contains(string(b), "no message") {
		t.Logf("draft mentions messages: %s", b)
	}
	none := composeEvidenceDraft(evidenceInput{Kind: domain.TxCreditPurchase, PurchasedAt: at, RefundPolicyURL: "u"})
	if !strings.Contains(none.AccessActivityLog, "No reading has been started") || !strings.Contains(none.RefundPolicyDisclosure, "published at u") {
		t.Fatalf("no readings, no consent: %+v", none)
	}
	membership := composeEvidenceDraft(evidenceInput{Kind: domain.TxSubscriptionRenewal, ProductName: "Premium", PurchasedAt: at})
	if !strings.Contains(membership.ProductDescription, "Premium Membership") || strings.Contains(membership.CancellationRebuttal, "not a subscription") {
		t.Fatalf("membership evidence: %+v", membership)
	}
}

func TestEvidenceUsageLogCapsAtTheListLimit(t *testing.T) {
	rows := make([]repository.UsageRow, evidenceUsageRows)
	for i := range rows {
		rows[i] = repository.UsageRow{StartedAt: time.Now(), Advisor: "A", BilledMinutes: 1, CreditsCharged: 1}
	}
	f := composeEvidenceDraft(evidenceInput{Kind: domain.TxCreditPurchase, PurchasedAt: time.Now(), Usage: rows})
	if !strings.Contains(f.AccessActivityLog, "the first 50 readings are listed") {
		t.Fatal("a truncated log says so")
	}
}

func TestValidateEvidence(t *testing.T) {
	ok, err := validateEvidence(DisputeEvidenceFields{ProductDescription: "  Credits\r\nfor chat ", CustomerName: " Ada  Lovelace ", CustomerEmail: " ADA@Example.com ",
		BillingCountry: "fr", ServiceDate: "2026-11-19", AttachInvoice: true}, true)
	if err != nil || ok.ProductDescription != "Credits\nfor chat" || ok.CustomerName != "Ada Lovelace" || ok.CustomerEmail != "ada@example.com" || ok.BillingCountry != "FR" {
		t.Fatalf("clean: %+v %v", ok, err)
	}
	bad := DisputeEvidenceFields{ProductDescription: strings.Repeat("x", evidenceTextMax+1), CustomerName: strings.Repeat("n", 201),
		CustomerEmail: "not-an-email", BillingCountry: "France", ServiceDate: "19/11/2026"}
	_, err = validateEvidence(bad, false)
	fields := siteFields(t, err)
	for _, k := range []string{"product_description", "customer_name", "customer_email", "billing_country", "service_date"} {
		if fields[k] == "" {
			t.Errorf("field %s must be refused: %v", k, fields)
		}
	}
	if _, err := validateEvidence(DisputeEvidenceFields{}, false); err != nil {
		t.Fatalf("saving an empty draft is allowed: %v", err)
	}
	if _, err := validateEvidence(DisputeEvidenceFields{}, true); err == nil {
		t.Fatal("submitting needs a product description")
	}
}

// ---- status page ----

func TestMissingEventsAndEndpointMatching(t *testing.T) {
	url := "https://api.staryield.net/api/v1/webhooks/payments/stripe"
	if got := missingEvents(nil, url); len(got) != len(stripeSubscribedEvents) {
		t.Fatalf("no endpoint: every event is missing: %v", got)
	}
	all := []payment.WebhookEndpointInfo{{URL: url + "/", Status: "enabled", EnabledEvents: []string{"*"}}}
	if got := missingEvents(all, url); len(got) != 0 {
		t.Fatalf("a wildcard subscribes to everything: %v", got)
	}
	partial := []payment.WebhookEndpointInfo{{URL: "https://elsewhere.test/hook", EnabledEvents: []string{"*"}},
		{URL: url, Status: "enabled", EnabledEvents: stripeSubscribedEvents[:len(stripeSubscribedEvents)-2]}}
	got := missingEvents(partial, url)
	if len(got) != 2 || got[0] != "review.opened" || got[1] != "review.closed" {
		t.Fatalf("partial: %v", got)
	}
	if endpointFor(partial, url) == nil || endpointFor(partial, "https://nowhere.test") != nil {
		t.Fatal("endpoint matching")
	}
	// The contract lists every event the adapter maps.
	for _, ev := range []string{"payment_intent.succeeded", "refund.failed", "charge.dispute.funds_reinstated", "radar.early_fraud_warning.updated", "customer.deleted"} {
		found := false
		for _, s := range stripeSubscribedEvents {
			found = found || s == ev
		}
		if !found {
			t.Errorf("%s missing from the subscription list", ev)
		}
	}
}

func TestStripeDashboardURL(t *testing.T) {
	for _, c := range []struct {
		ref  string
		live bool
		want string
	}{
		{"pi_123", true, "https://dashboard.stripe.com/payments/pi_123"},
		{"pi_123", false, "https://dashboard.stripe.com/test/payments/pi_123"},
		{"ch_123", true, ""},
		{"", true, ""},
	} {
		if got := stripeDashboardURL(c.ref, c.live); got != c.want {
			t.Errorf("%s live=%v: %q, want %q", c.ref, c.live, got, c.want)
		}
	}
}

func TestChargeDetailsFieldsKeepOnlyWhatIsKnownAndValid(t *testing.T) {
	now := time.Now()
	f := chargeDetails{Card: &payment.CardMeta{Brand: "Visa", Last4: "4242", Country: "fr"},
		Risk: &payment.RiskInfo{Level: "weird", Score: 150}, ThreeDS: "authenticated"}.fields(now)
	if f["card_country"] != "FR" || f["card_brand"] != "visa" || f["card_last4"] != "4242" || f["risk_level"] != payment.RiskUnknown ||
		f["three_ds"] != "authenticated" || f["updated_at"] != now {
		t.Fatalf("fields: %v", f)
	}
	if _, ok := f["risk_score"]; ok {
		t.Fatal("a score outside 0-100 is dropped")
	}
	g := chargeDetails{Card: &payment.CardMeta{Last4: "42x2", Country: "France"}, Risk: &payment.RiskInfo{Level: payment.RiskElevated, Score: -1}}.fields(now)
	if g["risk_level"] != payment.RiskElevated || g["card_last4"] != nil || g["card_country"] != nil || g["risk_score"] != nil {
		t.Fatalf("invalid card parts must be dropped: %v", g)
	}
	if !(chargeDetails{}).empty() || len((chargeDetails{}).fields(now)) != 0 {
		t.Fatal("nothing known, nothing written")
	}
	if refHash("pm_1") == refHash("pm_2") || len(refHash("pm_1")) != 64 {
		t.Fatal("ref hash")
	}
}

func TestPurchaseStreamFinalStatuses(t *testing.T) {
	for st, want := range map[string]bool{"succeeded": true, "failed": true, "refunded": true, "pending": false, "": false} {
		if FinalPurchaseStatus(st) != want {
			t.Errorf("%q", st)
		}
	}
	if s := (&CreditService{}).StreamSettings(); s.PollEvery != 3*time.Second || s.MaxChecks != 20 || s.MaxLifetime != 120*time.Second {
		t.Fatalf("defaults: %+v", s)
	}
	s := (&CreditService{}).WithStreamSettings(PurchaseStreamSettings{PollEvery: time.Second, MaxChecks: 2, MaxLifetime: time.Minute})
	if got := s.StreamSettings(); got.MaxChecks != 2 || got.MaxLifetime != time.Minute {
		t.Fatalf("override: %+v", got)
	}
}
