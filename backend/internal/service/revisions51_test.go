package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// revisions 5.1: merchant-account readiness (§25).

func TestConsentDatesMustBeRealAndNotInTheFuture(t *testing.T) {
	now := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	good := []string{"2026-11-13", "2026-01-01", "2025-02-28", " 2026-11-12 "}
	bad := []string{"", "2026-11-14", "2999-01-01", "2026-02-30", "2025-02-29", "2026-13-01", "2026-1-5", "26-11-13", "2026/11/13",
		"13-11-2026", "2026-11-13T00:00:00Z", "yesterday", "0000-00-00"}
	for _, v := range good {
		if _, ok := consentDate(v, now); !ok {
			t.Errorf("%q must be accepted", v)
		}
	}
	for _, v := range bad {
		if _, ok := consentDate(v, now); ok {
			t.Errorf("%q must be refused", v)
		}
	}
}

func TestPurchaseConsentValidationAndRecord(t *testing.T) {
	s := &CreditService{settings: CreditSettings{IPHashSecret: "secret-for-tests-0123456789"}}
	now := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	for _, c := range []*PurchaseConsent{nil, {}, {Terms: "2026-11-13", Privacy: "2026-11-13"}, {Terms: "2026-11-13", Privacy: "2999-01-01", Refund: "2026-11-13"},
		{Terms: "nope", Privacy: "2026-11-13", Refund: "2026-11-13"}} {
		_, err := s.validatePurchaseConsent(context.Background(), c, now)
		e, ok := apperror.As(err)
		if !ok || e.Status != 422 || e.Details == nil {
			t.Fatalf("consent %+v: %v", c, err)
		}
	}
	ctx := reqctx.WithMeta(context.Background(), reqctx.Meta{IP: "203.0.113.9", UserAgent: "  Mozilla/5.0\t(X11)  " + strings.Repeat("é", 400)})
	rec, err := s.validatePurchaseConsent(ctx, &PurchaseConsent{Terms: "2026-11-01", Privacy: "2026-11-02", Refund: "2026-11-03"}, now)
	if err != nil {
		t.Fatal(err)
	}
	p := &domain.CreditPurchase{}
	applyConsent(p, rec)
	got := purchaseConsentOf(p)
	if got == nil || got.Terms != "2026-11-01" || got.Privacy != "2026-11-02" || got.Refund != "2026-11-03" || !got.At.Equal(now) {
		t.Fatalf("record: %+v", got)
	}
	if p.ConsentIPHash == nil || *p.ConsentIPHash != securityKeyed("secret-for-tests-0123456789", "203.0.113.9") || strings.Contains(*p.ConsentIPHash, "203.0.113.9") {
		t.Fatalf("ip hash: %v", p.ConsentIPHash)
	}
	if p.ConsentUserAgent == nil || !strings.HasPrefix(*p.ConsentUserAgent, "Mozilla/5.0 (X11) é") || len([]rune(*p.ConsentUserAgent)) != consentUserAgentMax {
		t.Fatalf("user agent: %v (%d runes)", p.ConsentUserAgent, len([]rune(*p.ConsentUserAgent)))
	}
	// Purchases from before revisions 5.1 carry no record.
	if purchaseConsentOf(&domain.CreditPurchase{}) != nil {
		t.Fatal("an old purchase has no consent")
	}
}

// Every charge path carries the statement descriptor (§25.3): the hosted
// checkout request is built in one place.
func TestCheckoutRequestCarriesTheStatementDescriptor(t *testing.T) {
	s := &CreditService{settings: CreditSettings{StatementDescriptor: "STARYIELD MEDIA LLC"}}
	p := &domain.CreditPurchase{ID: uuid.New(), UserID: uuid.New(), PriceCents: 1999, Currency: "USD"}
	req := s.checkoutRequest(p, "Starter", "a@example.com")
	if req.StatementDescriptor != "STARYIELD MEDIA LLC" || req.AmountCents != 1999 || req.PurchaseID != p.ID || req.Description != "Starter" {
		t.Fatalf("checkout request: %+v", req)
	}
	var _ payment.CheckoutRequest = req
}

func TestMembershipsSwitchRefusesNewSubscriptionsOnly(t *testing.T) {
	off := &BillingService{cfg: BillingSettings{Currency: "USD"}}
	plans, err := off.Plans(context.Background(), "en")
	if err != nil || plans == nil || len(plans) != 0 {
		t.Fatalf("plans while off: %v %v", plans, err)
	}
	if off.MembershipsEnabled() {
		t.Fatal("off")
	}
	check := func(name string, err error) {
		t.Helper()
		e, ok := apperror.As(err)
		if !ok || e.Code != apperror.CodeFeatureUnavailable || e.Status != 503 || e.Message != "Memberships are not available." {
			t.Errorf("%s while off: %v", name, err)
		}
	}
	_, err = off.Subscribe(context.Background(), uuid.New(), SubscribeInput{})
	check("subscribe", err)
	_, err = off.UpdateSubscription(context.Background(), uuid.New(), UpdateSubscriptionInput{})
	check("change card", err)
	_, err = off.ResumeSubscription(context.Background(), uuid.New(), "en")
	check("resume", err)
}

func TestRefundAndMembershipNotices(t *testing.T) {
	parent := &domain.PaymentTransaction{ID: uuid.New(), Kind: domain.TxCreditPurchase, Currency: "USD", Credits: 600, Description: "Starter",
		CreditPurchaseID: ptr(uuid.New()), CreatedAt: time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)}
	n := refundNotice(parent, uuid.New(), 500, "STARYIELD MEDIA LLC")
	if n["amount_cents"] != int64(500) || n["currency"] != "USD" || n["pack_name"] != "Starter" || n["credits"] != int64(600) ||
		n["statement_descriptor"] != "STARYIELD MEDIA LLC" || n["purchase_id"] != *parent.CreditPurchaseID {
		t.Fatalf("refund notice: %v", n)
	}
	if _, ok := refundNotice(parent, uuid.New(), 500, "")["statement_descriptor"]; ok {
		t.Fatal("no descriptor configured: none in the notice")
	}
	start := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	m := membershipNotice(&domain.Subscription{ID: uuid.New()}, &domain.SubscriptionPlan{Name: domain.LocalizedText{"en": "Premium", "fr": "Prime"}},
		&domain.PlanVersion{IntervalCount: 1, BillingInterval: domain.IntervalMonth},
		&domain.SubscriptionInvoice{PeriodStart: start, PeriodEnd: start.AddDate(0, 1, 0)},
		&domain.PaymentTransaction{AmountCents: 999, Currency: "USD"}, "STARYIELD MEDIA LLC")
	if m["plan_name"] != "Premium" || m["amount_cents"] != int64(999) || m["period_months"] != 1 || m["statement_descriptor"] != "STARYIELD MEDIA LLC" ||
		m["next_renewal_at"] != start.AddDate(0, 1, 0) || m["plan_names"].(map[string]any)["fr"] != "Prime" {
		t.Fatalf("membership notice: %v", m)
	}
	// quietRefund is carried by the context only when set.
	if quietRefund(context.Background()) || !quietRefund(withQuietRefund(context.Background())) {
		t.Fatal("quiet refund flag")
	}
}

type recordingContacts struct{ c repository.Contact }

func (r recordingContacts) Contacts(context.Context, []uuid.UUID) ([]repository.Contact, error) {
	return []repository.Contact{r.c}, nil
}

type recordingQueue struct{ reqs []EmailRequest }

func (q *recordingQueue) Enqueue(_ context.Context, reqs ...EmailRequest) error {
	q.reqs = append(q.reqs, reqs...)
	return nil
}

// Every billing e-mail (receipt, refund, membership started/renewed) is sent:
// a real charge always gets its receipt, whatever the memberships switch says.
func TestBillingEmailsAreAlwaysSent(t *testing.T) {
	uid := uuid.New()
	contact := repository.Contact{UserID: uid, Email: "a@example.com", Locale: "en", Status: domain.UserActive}
	rows := []domain.Notification{
		{ID: uuid.New(), UserID: uid, Kind: NotifySubscriptionStarted, Data: domain.JSONMap{"plan_name": "Premium"}, Link: "/profile/settings#billing"},
		{ID: uuid.New(), UserID: uid, Kind: NotifySubscriptionRenewed, Data: domain.JSONMap{"plan_name": "Premium"}, Link: "/profile/settings#billing"},
		{ID: uuid.New(), UserID: uid, Kind: NotifyPurchase, Data: domain.JSONMap{"credits": int64(50)}, Link: "/profile"},
		{ID: uuid.New(), UserID: uid, Kind: NotifyPaymentRefunded, Data: domain.JSONMap{"amount_cents": int64(500), "currency": "USD"}, Link: "/profile/settings#billing"},
	}
	// Billing e-mails go out for every real charge or refund. The memberships
	// switch only stops new memberships (§25.1); a membership that still
	// renews must get its receipt, so membership e-mails are never held back.
	{
		q := &recordingQueue{}
		ch := NewNotificationChannels(recordingContacts{contact}, q, nil, "https://staryield.net", zap.NewNop())
		ch.Deliver(context.Background(), rows)
		got := map[string]bool{}
		for _, r := range q.reqs {
			got[strings.TrimPrefix(r.Template, "notification:")] = true
			if r.Category != domain.EmailBilling {
				t.Errorf("%s is billing mail, got %s", r.Template, r.Category)
			}
		}
		if !got[NotifyPurchase] || !got[NotifyPaymentRefunded] || !got[NotifySubscriptionStarted] || !got[NotifySubscriptionRenewed] {
			t.Errorf("every billing e-mail is sent: %v", got)
		}
	}
}

// The receipt copy to receipt_email is sent only to a different address.
func TestReceiptCopyGoesToAnotherAddressOnly(t *testing.T) {
	uid := uuid.New()
	q := &recordingQueue{}
	s := &CreditService{clock: clock.NewFake(time.Now()), mailer: q,
		contacts: recordingContacts{repository.Contact{UserID: uid, Email: "Owner@Example.com", Locale: "fr", Status: domain.UserActive}}}
	data := map[string]any{"credits": int64(600), "price_cents": int64(1999), "currency": "USD", "purchase_id": uuid.New(), "pack_name": "Starter"}
	buy := func(receipt string) *domain.CreditPurchase {
		return &domain.CreditPurchase{ID: uuid.New(), UserID: uid, ReceiptEmail: receipt}
	}
	s.mailReceiptCopy(context.Background(), buy(""), data)
	s.mailReceiptCopy(context.Background(), buy("owner@example.com"), data) // the account's own address (any case)
	if len(q.reqs) != 0 {
		t.Fatalf("no copy expected: %+v", q.reqs)
	}
	s.mailReceiptCopy(context.Background(), buy("accounting@example.org"), data)
	if len(q.reqs) != 1 {
		t.Fatalf("one copy expected: %+v", q.reqs)
	}
	r := q.reqs[0]
	if r.To != "accounting@example.org" || r.Locale != "fr" || r.Category != domain.EmailBilling || r.Content.Action != nil ||
		!strings.Contains(r.Content.Subject, "600") || !strings.HasPrefix(r.DedupeKey, "purchase_receipt_email:") {
		t.Fatalf("copy: %+v", r)
	}
}

func TestSiteConfigAdvertisesMembershipsOnlyWhenOn(t *testing.T) {
	snap := &siteSnapshot{}
	snap.Signup.Enabled, snap.Signup.FreeMinutes = true, 4
	now := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	off, err := renderSiteConfig(snap, "en", now, false, false)
	if err != nil {
		t.Fatal(err)
	}
	on, _ := renderSiteConfig(snap, "en", now, false, true)
	if !strings.Contains(string(off.Body), `"features":{"memberships":false}`) || !strings.Contains(string(on.Body), `"features":{"memberships":true}`) {
		t.Fatalf("features: %s / %s", off.Body, on.Body)
	}
	if off.ETag == on.ETag {
		t.Fatal("the validator must change when features.memberships does")
	}
}
