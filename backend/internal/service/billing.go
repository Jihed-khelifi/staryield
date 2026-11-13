package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// Billing error codes (stable; kept in sync with the frontend catalogue).
const (
	CodeSubscriptionExists    apperror.Code = "subscription_exists"
	CodePaymentMethodRequired apperror.Code = "payment_method_required"
	CodeLimitReached          apperror.Code = "limit_reached"
)

// BillingSettings configure memberships and saved cards.
type BillingSettings struct {
	Currency          string
	RenewalLease      time.Duration // claim lease of a charge attempt in flight
	GracePeriod       time.Duration // past-due membership expires after this
	IncompleteTTL     time.Duration // unresolved first charge expires after this
	MaxPaymentMethods int
	IPHashSecret      string
	// revisions 5.1: the memberships switch (§25.1) and the statement
	// descriptor sent with every charge (§25.3).
	MembershipsEnabled  bool
	StatementDescriptor string
}

// PurchaseSettler finishes credit purchases whose charge resolved
// asynchronously (reconciliation, webhooks). CreditService implements it.
type PurchaseSettler interface {
	CompletePurchase(ctx context.Context, purchaseID uuid.UUID, providerRef string) error
	FailPurchase(ctx context.Context, purchaseID uuid.UUID, code string) error
}

// BillingService sells the Premium Membership and manages saved payment
// methods, provider routing and the money-movement ledger. Card data never
// reaches it: cards arrive as provider tokens and are stored as sealed
// provider references.
type BillingService struct {
	tx       database.TxManager
	repo     repository.BillingRepository
	users    repository.UserRepository
	wallets  repository.WalletRepository
	wallet   *WalletService
	credits  repository.CreditRepository
	registry *payment.Registry
	legacy   payment.Provider
	sealer   *security.Sealer
	audit    repository.AuditRepository
	events   EventPublisher
	notify   Notifier
	tracker  analytics.Tracker
	clock    clock.Clock
	cfg      BillingSettings
	settler  PurchaseSettler
	// revisions 5.0: refund requests. Loads the refund info of a page of
	// payments (RefundService.TransactionInfo); nil leaves the rows without it.
	refundInfo RefundInfoLoader
}

// BillingDeps are the billing service's collaborators.
type BillingDeps struct {
	Tx       database.TxManager
	Repo     repository.BillingRepository
	Users    repository.UserRepository
	Wallets  repository.WalletRepository
	Wallet   *WalletService
	Credits  repository.CreditRepository
	Registry *payment.Registry
	// Legacy is the hosted-checkout provider of plain credit purchases; it
	// is consulted for refunds of purchases it processed.
	Legacy  payment.Provider
	Sealer  *security.Sealer
	Audit   repository.AuditRepository
	Events  EventPublisher
	Notify  Notifier
	Tracker analytics.Tracker
	Clock   clock.Clock
}

// NewBillingService wires the billing service.
func NewBillingService(d BillingDeps, cfg BillingSettings) *BillingService {
	if cfg.RenewalLease <= 0 {
		cfg.RenewalLease = 5 * time.Minute
	}
	if cfg.GracePeriod <= 0 {
		cfg.GracePeriod = 7 * 24 * time.Hour
	}
	if cfg.IncompleteTTL <= 0 {
		cfg.IncompleteTTL = 24 * time.Hour
	}
	if cfg.MaxPaymentMethods <= 0 {
		cfg.MaxPaymentMethods = 5
	}
	if cfg.Currency == "" {
		cfg.Currency = "USD"
	}
	notify := d.Notify
	if notify == nil {
		notify = NoopNotifier{}
	}
	events := d.Events
	if events == nil {
		events = NoopPublisher{}
	}
	return &BillingService{tx: d.Tx, repo: d.Repo, users: d.Users, wallets: d.Wallets, wallet: d.Wallet, credits: d.Credits,
		registry: d.Registry, legacy: d.Legacy, sealer: d.Sealer, audit: d.Audit, events: events, notify: notify, tracker: d.Tracker,
		clock: d.Clock, cfg: cfg}
}

// MembershipsEnabled reports whether memberships are on sale (revisions 5.1,
// STARYIELD_BILLING_MEMBERSHIPS_ENABLED). While off nobody can subscribe, change
// the membership card or resume; viewing and cancelling always work.
func (s *BillingService) MembershipsEnabled() bool { return s.cfg.MembershipsEnabled }

// errMembershipsOff is the 503 of the membership actions while the switch is off.
func errMembershipsOff() error { return apperror.FeatureUnavailable("Memberships are not available.") }

// WithSettler lets the reconciliation worker finish credit purchases.
func (s *BillingService) WithSettler(p PurchaseSettler) *BillingService {
	s.settler = p
	return s
}

// WithRefundInfo lets the transactions list carry the refund info of credit
// purchases (revisions 5.0, §24.4).
func (s *BillingService) WithRefundInfo(l RefundInfoLoader) *BillingService {
	s.refundInfo = l
	return s
}

// ------------------------------------------------------------------ views

// PlanView is a membership plan as sold today (its current version).
type PlanView struct {
	ID                 uuid.UUID `json:"id"`
	Code               string    `json:"code"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	VersionID          uuid.UUID `json:"version_id"`
	Version            int       `json:"version"`
	PriceCents         int64     `json:"price_cents"`
	Currency           string    `json:"currency"`
	Interval           string    `json:"interval"`
	IntervalCount      int       `json:"interval_count"`
	MonthlyCreditGrant int64     `json:"monthly_credit_grant"`
	ChatDiscountBps    int       `json:"chat_discount_bps"`
	ChatDiscountPct    float64   `json:"chat_discount_pct"`
	MonthlyFreeMinutes int       `json:"monthly_free_minutes"`
	WaitlistPriority   bool      `json:"waitlist_priority"`
	Benefits           []string  `json:"benefits"`
}

func planView(p *domain.SubscriptionPlan, v *domain.PlanVersion, locale string) PlanView {
	return PlanView{ID: p.ID, Code: p.Code, Name: p.Name.For(locale), Description: p.Description.For(locale), VersionID: v.ID,
		Version: v.Version, PriceCents: v.PriceCents, Currency: v.Currency, Interval: string(v.BillingInterval),
		IntervalCount: v.IntervalCount, MonthlyCreditGrant: v.MonthlyCreditGrant, ChatDiscountBps: v.ChatDiscountBps,
		ChatDiscountPct: float64(v.ChatDiscountBps) / 100, MonthlyFreeMinutes: v.MonthlyFreeMinutes,
		WaitlistPriority: v.WaitlistPriority, Benefits: v.Benefits.For(locale)}
}

// PaymentMethodView is a saved card (display data only).
type PaymentMethodView struct {
	ID        uuid.UUID `json:"id"`
	Provider  string    `json:"provider"`
	Brand     string    `json:"brand"`
	Last4     string    `json:"last4"`
	ExpMonth  int       `json:"exp_month"`
	ExpYear   int       `json:"exp_year"`
	IsDefault bool      `json:"is_default"`
	Status    string    `json:"status"`
	Expired   bool      `json:"expired"`
	CreatedAt time.Time `json:"created_at"`
}

func methodView(m *domain.PaymentMethod, now time.Time) PaymentMethodView {
	status := m.Status
	if status == domain.MethodActive && m.ExpiredAt(now) {
		status = domain.MethodExpired
	}
	return PaymentMethodView{ID: m.ID, Provider: m.Provider, Brand: m.Brand, Last4: m.Last4, ExpMonth: m.ExpMonth, ExpYear: m.ExpYear,
		IsDefault: m.IsDefault, Status: string(status), Expired: m.ExpiredAt(now), CreatedAt: m.CreatedAt}
}

// CardSummary identifies the card of a transaction.
type CardSummary struct {
	Brand string `json:"brand"`
	Last4 string `json:"last4"`
}

// SubscriptionView is a membership.
type SubscriptionView struct {
	ID                 uuid.UUID          `json:"id"`
	Status             string             `json:"status"`
	Plan               PlanView           `json:"plan"`
	CancelAtPeriodEnd  bool               `json:"cancel_at_period_end"`
	CurrentPeriodStart *time.Time         `json:"current_period_start,omitempty"`
	CurrentPeriodEnd   *time.Time         `json:"current_period_end,omitempty"`
	RenewsAt           *time.Time         `json:"renews_at,omitempty"`
	NextRetryAt        *time.Time         `json:"next_retry_at,omitempty"`
	GraceUntil         *time.Time         `json:"grace_until,omitempty"`
	CanceledAt         *time.Time         `json:"canceled_at,omitempty"`
	EndedAt            *time.Time         `json:"ended_at,omitempty"`
	PaymentMethod      *PaymentMethodView `json:"payment_method,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
}

// TransactionView is a money movement shown to its owner.
type TransactionView struct {
	ID                  uuid.UUID    `json:"id"`
	Kind                string       `json:"kind"`
	Status              string       `json:"status"`
	AmountCents         int64        `json:"amount_cents"`
	RefundedAmountCents int64        `json:"refunded_amount_cents"`
	Currency            string       `json:"currency"`
	Description         string       `json:"description"`
	Credits             int64        `json:"credits,omitempty"`
	PaymentMethod       *CardSummary `json:"payment_method,omitempty"`
	FailureCode         string       `json:"failure_code,omitempty"`
	CreatedAt           time.Time    `json:"created_at"`
	// Refund (revisions 5.0, §24.4) is set on the caller's own credit-purchase
	// rows by GET /billing/transactions only.
	Refund *TransactionRefundInfo `json:"refund,omitempty"`
}

func transactionView(t *domain.PaymentTransaction, methods map[uuid.UUID]domain.PaymentMethod) TransactionView {
	v := TransactionView{ID: t.ID, Kind: string(t.Kind), Status: string(t.Status), AmountCents: t.AmountCents,
		RefundedAmountCents: t.RefundedAmountCents, Currency: t.Currency, Description: t.Description, Credits: t.Credits,
		FailureCode: t.FailureCode, CreatedAt: t.CreatedAt}
	if t.PaymentMethodID != nil {
		if m, ok := methods[*t.PaymentMethodID]; ok {
			v.PaymentMethod = &CardSummary{Brand: m.Brand, Last4: m.Last4}
		}
	}
	return v
}

// SubscriptionResult is the outcome of a membership change that charged.
type SubscriptionResult struct {
	Subscription SubscriptionView    `json:"subscription"`
	Transaction  *TransactionView    `json:"transaction,omitempty"`
	NextAction   *payment.NextAction `json:"next_action,omitempty"`
}

// ---------------------------------------------------------------- helpers

func (s *BillingService) disabled() bool { return s.registry.Empty() }

func featureOff(what string) error {
	switch what {
	case "methods":
		return apperror.FeatureUnavailable("Saved cards are not available yet.")
	default:
		return apperror.FeatureUnavailable("Memberships are not available yet.")
	}
}

func customerAD(c *domain.PaymentCustomer) string {
	return "pcus:" + c.ID.String() + ":" + c.UserID.String()
}
func methodAD(m *domain.PaymentMethod) string { return "pm:" + m.ID.String() + ":" + m.UserID.String() }
func networkAD(m *domain.PaymentMethod) string {
	return "pmnet:" + m.ID.String() + ":" + m.UserID.String()
}

// refs decrypts the provider references needed to charge a saved card.
func (s *BillingService) refs(ctx context.Context, m *domain.PaymentMethod) (customerRef, methodRef, networkRef string, err error) {
	c, err := s.repo.GetCustomerByID(ctx, m.CustomerID)
	if err != nil {
		return "", "", "", err
	}
	if customerRef, err = s.sealer.Open(c.ProviderCustomerRefEnc, customerAD(c)); err != nil {
		return "", "", "", err
	}
	if methodRef, err = s.sealer.Open(m.ProviderMethodRefEnc, methodAD(m)); err != nil {
		return "", "", "", err
	}
	if m.NetworkTxnRefEnc != "" {
		networkRef, _ = s.sealer.Open(m.NetworkTxnRefEnc, networkAD(m))
	}
	return customerRef, methodRef, networkRef, nil
}

// rules loads the routing rules for the registry.
func (s *BillingService) rules(ctx context.Context) ([]payment.Rule, error) {
	rows, err := s.repo.ListRules(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]payment.Rule, 0, len(rows))
	for _, r := range rows {
		out = append(out, payment.Rule{ID: r.ID.String(), Priority: r.Priority, Enabled: r.Enabled, Purpose: string(r.Purpose),
			Providers: r.Providers, Countries: r.MatchCountries, Currencies: r.MatchCurrencies, PlanIDs: r.MatchPlanIDs, UserIDs: r.MatchUserIDs})
	}
	return out, nil
}

func countries(values ...string) []string {
	var out []string
	seen := map[string]bool{}
	for _, v := range values {
		v = strings.ToUpper(strings.TrimSpace(v))
		if countryCode.MatchString(v) && !seen[v] && v != "XX" && v != "T1" {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// route returns the providers allowed for a payment.
func (s *BillingService) route(ctx context.Context, in payment.RouteInput) ([]payment.Named, string, error) {
	rules, err := s.rules(ctx)
	if err != nil {
		return nil, "", err
	}
	return s.registry.Route(in, rules)
}

// providerAllowed reports whether provider may serve the payment for every
// purpose given (any of them is enough when anyPurpose).
func (s *BillingService) providerAllowed(ctx context.Context, provider string, in payment.RouteInput, purposes ...string) (bool, error) {
	for _, purpose := range purposes {
		in.Purpose = purpose
		ps, _, err := s.route(ctx, in)
		if errors.Is(err, payment.ErrNoProvider) {
			continue
		}
		if err != nil {
			return false, err
		}
		for _, p := range ps {
			if p.Name() == provider {
				return true, nil
			}
		}
	}
	return false, nil
}

// storedCharger returns the provider of a saved card if it can charge it.
func (s *BillingService) storedCharger(name string) (payment.StoredCharger, bool) {
	p, ok := s.registry.Get(name)
	if !ok {
		return nil, false
	}
	c, ok := p.(payment.StoredCharger)
	return c, ok
}

// refunder returns the provider able to refund a transaction it processed.
func (s *BillingService) refunder(name string) (payment.Refunder, bool) {
	if p, ok := s.registry.Get(name); ok {
		r, ok := p.(payment.Refunder)
		return r, ok
	}
	if s.legacy != nil && s.legacy.Name() == name {
		r, ok := s.legacy.(payment.Refunder)
		return r, ok
	}
	return nil, false
}

func (s *BillingService) record(ctx context.Context, action, targetType, targetID string, meta map[string]any) {
	recordAudit(ctx, s.audit, s.clock, s.cfg.IPHashSecret, action, targetType, targetID, meta)
}

func (s *BillingService) track(ctx context.Context, name string, props map[string]any) {
	if s.tracker != nil {
		s.tracker.Track(ctx, name, props)
	}
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func paymentFailed(code string, details map[string]any) error {
	d := map[string]any{"reason": code}
	for k, v := range details {
		d[k] = v
	}
	return (&apperror.Error{Code: apperror.CodePaymentFailed, Status: http.StatusPaymentRequired,
		Message: "Your payment was declined. Please use another card."}).WithDetails(d)
}

// panRun flags a 13–19 digit run: card numbers must never be sent to us.
var panRun = regexp.MustCompile(`[0-9]{13,19}`)

// clientToken validates a provider token from the browser.
var clientToken = regexp.MustCompile(`^[A-Za-z0-9_\-.:]{8,512}$`)

func validateToken(token string) map[string]string {
	switch {
	case panRun.MatchString(strings.ReplaceAll(strings.ReplaceAll(token, " ", ""), "-", "")):
		return map[string]string{"token": "Card numbers must be entered in the secure payment form, never sent directly."}
	case !clientToken.MatchString(token):
		return map[string]string{"token": "The card details could not be verified. Please enter them again."}
	}
	return nil
}

var providerName = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)
var regexpBrand = regexp.MustCompile(`^[a-z][a-z_]{1,19}$`)
var countryCode = regexp.MustCompile(`^[A-Z]{2}$`)
var currencyCode = regexp.MustCompile(`^[A-Z]{3}$`)

// ------------------------------------------------------- member benefits

// activeSubscription returns the user's membership if it currently gives
// benefits (active, inside a paid period).
func (s *BillingService) activeSubscription(ctx context.Context, userID uuid.UUID) (*domain.Subscription, *domain.PlanVersion, error) {
	sub, err := s.repo.GetOpenSubscription(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	now := s.clock.Now()
	if sub.Status != domain.SubActive || sub.CurrentPeriodEnd == nil || !sub.CurrentPeriodEnd.After(now) {
		return nil, nil, repository.ErrNotFound
	}
	v, err := s.repo.GetVersion(ctx, sub.PlanVersionID)
	if err != nil {
		return nil, nil, err
	}
	return sub, v, nil
}

// IsMember reports whether the user currently has an active membership
// (used for waitlist priority). Errors count as "not a member".
func (s *BillingService) IsMember(ctx context.Context, userID uuid.UUID) bool {
	_, v, err := s.activeSubscription(ctx, userID)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			warnErr(ctx, "billing: membership lookup", err)
		}
		return false
	}
	return v != nil
}

// HasWaitlistPriority reports whether the user's membership includes
// waitlist priority.
func (s *BillingService) HasWaitlistPriority(ctx context.Context, userID uuid.UUID) bool {
	_, v, err := s.activeSubscription(ctx, userID)
	return err == nil && v.WaitlistPriority
}

// DiscountFor returns the member chat discount in basis points (0 when the
// user has no active membership). The chat flow snapshots it into
// chat_sessions.discount_bps when a session starts and bills
// domain.ApplyDiscount(list rate, bps); advisor earnings stay on the list
// rate (platform-funded discount).
func (s *BillingService) DiscountFor(ctx context.Context, userID uuid.UUID) int {
	_, v, err := s.activeSubscription(ctx, userID)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			warnErr(ctx, "billing: discount lookup", err)
		}
		return 0
	}
	return v.ChatDiscountBps
}

// CancelForErasure implements BillingCanceler (account erasure and closure).
func (s *BillingService) CancelForErasure(ctx context.Context, userID uuid.UUID) error {
	return s.CancelForClosure(ctx, userID)
}

// CancelForClosure ends the user's membership immediately and removes
// their saved cards (account closure / erasure). Provider calls are best
// effort; the local records are authoritative.
func (s *BillingService) CancelForClosure(ctx context.Context, userID uuid.UUID) error {
	now := s.clock.Now()
	var canceled *domain.Subscription
	var methods []domain.PaymentMethod
	var customers []domain.PaymentCustomer
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if sub, err := s.repo.GetOpenSubscription(ctx, userID); err == nil {
			locked, err := s.repo.GetSubscriptionForUpdate(ctx, sub.ID)
			if err != nil {
				return err
			}
			if locked.Status.Open() {
				if err := s.repo.UpdateSubscription(ctx, locked.ID, map[string]any{"status": domain.SubCanceled, "ended_at": now,
					"canceled_at": now, "cancel_reason": "account_closed", "next_attempt_at": nil, "next_grant_at": nil,
					"locked_until": nil, "updated_at": now}); err != nil {
					return err
				}
				canceled = locked
			}
		} else if !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		var err error
		if methods, err = s.repo.ListMethods(ctx, userID); err != nil {
			return err
		}
		for _, m := range methods {
			if err := s.repo.SoftDeleteMethod(ctx, m.ID, now); err != nil {
				return err
			}
		}
		if customers, err = s.repo.ListCustomers(ctx, userID); err != nil {
			return err
		}
		for _, c := range customers {
			if err := s.repo.SoftDeleteCustomer(ctx, c.ID, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return mapRepoErr(err, "Membership")
	}
	ctx = database.Detach(ctx)
	for _, c := range customers {
		p, ok := s.registry.Get(c.Provider)
		if !ok {
			continue
		}
		vault, ok := p.(payment.CustomerVault)
		if !ok {
			continue
		}
		if ref, err := s.sealer.Open(c.ProviderCustomerRefEnc, customerAD(&c)); err == nil {
			warnErr(ctx, "billing: delete provider customer", vault.DeleteCustomer(ctx, ref), zap.String("provider", c.Provider))
		}
	}
	if canceled != nil {
		s.notify.Notify(ctx, NotificationSpec{Kind: NotifySubscriptionCanceled, Data: map[string]any{"subscription_id": canceled.ID,
			"reason": "account_closed"}, Link: "/profile/settings"}, userID)
	}
	logger.FromContext(ctx).Info("billing: membership closed for account", zap.Int("cards", len(methods)))
	return nil
}
