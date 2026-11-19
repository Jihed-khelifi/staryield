package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 6.0 (docs/platform-extensions.md §26.5): payment operations of
// the billing service. Everything here is provider-agnostic: it consumes the
// normalised webhook events and the optional provider capabilities
// (ChargeRetriever, DisputeManager, AccountInspector, EventFetcher).
//
//   - payment_ops.go            wiring, alerts, the purchase gate, charge details
//   - billing_webhooks.go       the webhook log and the event dispatcher
//   - billing_disputes.go       disputes (webhooks, evidence, accept)
//   - billing_fraud.go          early fraud warnings
//   - billing_admin_ops.go      the admin console's views of all of it
//   - billing_policies.go       the payment policies admin
//   - credits_stream.go         the purchase stream

// InvoicePDFs renders the Staryield invoice of a purchase (the invoices
// service implements it). Dispute evidence attaches it when it is wired.
type InvoicePDFs interface {
	PurchaseInvoicePDF(ctx context.Context, purchaseID uuid.UUID) (pdf []byte, filename string, err error)
}

// PaymentOpsSettings configure the payment operations.
type PaymentOpsSettings struct {
	// FrontendURL is the site origin (the refund policy link of dispute evidence).
	FrontendURL string
	// ConsoleURL is the admin console origin (links in alert e-mails).
	ConsoleURL string
	// SupportInbox receives the alert e-mails (empty: audit only).
	SupportInbox string
}

// paymentOps is what the billing service needs for payment operations.
type paymentOps struct {
	repo     repository.PaymentOpsRepository
	cfg      PaymentOpsSettings
	mailer   EmailQueue
	invoices InvoicePDFs

	mu     sync.Mutex
	status map[string]*providerProbe // account status per provider, cached
	mode   map[string]*modeProbe     // live or test mode per provider, cached
}

// statusCacheTTL is how long the processor's answers on the admin status page
// are kept (§26.5: "Stripe calls cached 60 s").
const statusCacheTTL = 60 * time.Second

// WithPaymentOps enables dispute, fraud-warning and webhook-log handling. q
// delivers the alert e-mails to the support inbox (nil: audit only).
func (s *BillingService) WithPaymentOps(repo repository.PaymentOpsRepository, cfg PaymentOpsSettings, q EmailQueue) *BillingService {
	s.ops = &paymentOps{repo: repo, cfg: cfg, mailer: q, status: map[string]*providerProbe{}, mode: map[string]*modeProbe{}}
	return s
}

// WithInvoicePDFs lets dispute evidence attach the invoice PDF.
func (s *BillingService) WithInvoicePDFs(i InvoicePDFs) *BillingService {
	if s.ops != nil {
		s.ops.invoices = i
	}
	return s
}

// PurchaseGate refuses a new credit purchase while the buyer has a payment
// dispute in progress and the policy says so (one indexed query). Install it
// with CreditService.WithPurchaseGate.
func (s *BillingService) PurchaseGate(ctx context.Context, userID uuid.UUID) error {
	if s.ops == nil || !s.policies(ctx).BlockPurchasesOnDispute {
		return nil
	}
	open, err := s.ops.repo.HasOpenDispute(ctx, userID)
	if err != nil {
		return apperror.Internal(err)
	}
	if open {
		return apperror.PurchasesBlocked()
	}
	return nil
}

// ----------------------------------------------------------------- helpers

// refHash is what the database keeps to find the sealed row of a provider
// reference (payment methods and customers).
func refHash(ref string) string { return sha256Hex([]byte("provider-ref:" + ref)) }

// chargeDetails are what a processor reports about a charge besides its outcome.
type chargeDetails struct {
	Card    *payment.CardMeta
	Risk    *payment.RiskInfo
	ThreeDS string
}

func (d chargeDetails) empty() bool { return d.Card == nil && d.Risk == nil && d.ThreeDS == "" }

// fields returns the columns to write for the details (only what is known).
func (d chargeDetails) fields(now time.Time) map[string]any {
	f := map[string]any{}
	if c := d.Card; c != nil {
		if country := strings.ToUpper(strings.TrimSpace(c.Country)); countryCode.MatchString(country) {
			f["card_country"] = country
		}
		if c.Brand != "" {
			f["card_brand"] = normalizeBrand(c.Brand)
		}
		if len(c.Last4) == 4 && strings.Trim(c.Last4, "0123456789") == "" {
			f["card_last4"] = c.Last4
		}
	}
	if r := d.Risk; r != nil {
		switch r.Level {
		case payment.RiskNormal, payment.RiskElevated, payment.RiskHighest, payment.RiskNotAssessed:
			f["risk_level"] = r.Level
		case "":
		default:
			f["risk_level"] = payment.RiskUnknown
		}
		if r.Score >= 0 && r.Score <= 100 {
			f["risk_score"] = r.Score
		}
	}
	if d.ThreeDS != "" {
		f["three_ds"] = truncateString(d.ThreeDS, 32)
	}
	if len(f) > 0 {
		f["updated_at"] = now
	}
	return f
}

// persistChargeDetails stores the card, risk and 3-D Secure details of a
// charge on its payment (best effort: details are evidence, not money).
func (s *BillingService) persistChargeDetails(ctx context.Context, transactionID uuid.UUID, d chargeDetails) {
	if d.empty() {
		return
	}
	f := d.fields(s.clock.Now())
	if len(f) == 0 {
		return
	}
	warnErr(ctx, "billing: store charge details", s.repo.UpdateTransaction(ctx, transactionID, f),
		zap.String("transaction_id", transactionID.String()))
}

// The provider reference of a charge that is not settled yet is stored by
// recordPendingRef (billing_stripe.go), so webhooks and the console find it.

// detailsMissing reports whether the payment has none of the processor's
// details yet (so one retrieval is worth making).
func detailsMissing(t *domain.PaymentTransaction) bool {
	return t.RiskLevel == "" && t.CardCountry == "" && t.ThreeDS == ""
}

// retrieveCharge asks the provider for the current state of a charge (nil
// when the provider cannot or the call failed; errors are logged).
func (s *BillingService) retrieveCharge(ctx context.Context, provider, ref string) (*payment.ChargeResult, *payment.DeclineError, bool) {
	p, ok := s.registry.Get(provider)
	if !ok {
		return nil, nil, false
	}
	r, ok := p.(payment.ChargeRetriever)
	if !ok || ref == "" {
		return nil, nil, false
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	res, decline, err := r.RetrieveCharge(cctx, ref)
	if err != nil {
		logger.FromContext(ctx).Warn("billing: retrieve charge failed", zap.Error(err), zap.String("provider", provider))
		return nil, nil, false
	}
	return &res, decline, true
}

// ------------------------------------------------------------------ alerts

// alert tells the team about something that needs a human (a dispute, a fraud
// warning, a failed refund): an e-mail to the support inbox, deduplicated by
// key. The audit record of the event is written by the caller.
func (s *BillingService) alert(ctx context.Context, key, heading string, lines []string, consolePath string) {
	if s.ops == nil || s.ops.mailer == nil || strings.TrimSpace(s.ops.cfg.SupportInbox) == "" {
		return
	}
	link := ""
	if base := strings.TrimRight(s.ops.cfg.ConsoleURL, "/"); base != "" {
		link = base + consolePath
	}
	err := s.ops.mailer.Enqueue(database.Detach(ctx), EmailRequest{To: strings.TrimSpace(s.ops.cfg.SupportInbox), Locale: "en",
		Category: domain.EmailAccount, Template: "payment_alert", Content: mail.PaymentAlertForTeam(heading, lines, link),
		DedupeKey: "payment_alert:" + key})
	warnErr(ctx, "billing: enqueue payment alert", err, zap.String("key", key))
}

// money formats an amount for alert lines ("USD 9.99").
func money(cents int64, currency string) string {
	return fmt.Sprintf("%s %d.%02d", currency, cents/100, cents%100)
}

// --------------------------------------------------------- dashboard links

// stripeProvider is the registered name of the Stripe adapter.
const stripeProvider = "stripe"

// stripeDashboardURL is the Stripe Dashboard page of a payment ("" when the
// reference is not a PaymentIntent).
func stripeDashboardURL(ref string, live bool) string {
	if !strings.HasPrefix(ref, "pi_") {
		return ""
	}
	if live {
		return "https://dashboard.stripe.com/payments/" + ref
	}
	return "https://dashboard.stripe.com/test/payments/" + ref
}

// modeProbe is the cached live/test mode of a provider.
type modeProbe struct {
	at   time.Time
	live bool
}

// providerLive reports whether a provider processes real money. The account
// status is the authority; when it cannot be read, the mode of the last
// webhook the provider sent is used (cached, so an admin page costs at most
// one query a minute); unknown counts as live.
func (s *BillingService) providerLive(ctx context.Context, provider string) bool {
	if s.ops == nil {
		return true
	}
	now := s.clock.Now()
	s.ops.mu.Lock()
	if m, ok := s.ops.mode[provider]; ok && now.Sub(m.at) < statusCacheTTL {
		s.ops.mu.Unlock()
		return m.live
	}
	s.ops.mu.Unlock()
	live := true
	if st, err := s.accountStatus(ctx, provider); err == nil && st != nil {
		live = st.Livemode
	} else if last, err := s.ops.repo.LastWebhook(ctx, provider); err == nil {
		live = last.Livemode
	}
	s.ops.mu.Lock()
	s.ops.mode[provider] = &modeProbe{at: now, live: live}
	s.ops.mu.Unlock()
	return live
}

// providerURL is the provider's own page of a payment (the console links it).
func (s *BillingService) providerURL(ctx context.Context, provider, ref string, lives map[string]bool) string {
	if provider != stripeProvider || ref == "" {
		return ""
	}
	live, ok := lives[provider]
	if !ok {
		live = s.providerLive(ctx, provider)
		lives[provider] = live
	}
	return stripeDashboardURL(ref, live)
}
