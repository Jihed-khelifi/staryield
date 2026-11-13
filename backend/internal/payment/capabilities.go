package payment

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Providers differ in what they can do (hosted checkout, a customer vault,
// stored-credential charges, refunds, webhooks…). Each capability is an
// optional interface discovered by type assertion, so an adapter only
// implements what its processor supports and routing can require a
// capability before choosing it. Planned adapters: Stripe (Payment Element +
// SetupIntent, off-session PaymentIntent), NMI-style high-risk gateways
// (PayKings / QuadraPay: Collect.js iframes, Customer Vault, stored-credential
// flags) — all keep Staryield at PCI SAQ-A because card data is entered in
// the provider's fields and only tokens reach our API.

// Named identifies a provider (every provider implements it).
type Named interface {
	Name() string
}

// Checkouter performs a one-off charge (hosted page or a single-use token).
type Checkouter interface {
	Checkout(ctx context.Context, req CheckoutRequest) (CheckoutResult, error)
}

// WebhookRequest is an inbound provider notification.
type WebhookRequest struct {
	Header http.Header
	Body   []byte
	Now    time.Time
}

// WebhookVerifier authenticates provider notifications (signature, replay
// window) and decodes them.
type WebhookVerifier interface {
	VerifyWebhookRequest(req WebhookRequest) (*WebhookEvent, error)
}

// CustomerRequest creates (or confirms) the provider-side customer.
type CustomerRequest struct {
	UserID         uuid.UUID
	ExistingRef    string
	IdempotencyKey string
}

// CustomerVault manages provider customers that own saved cards.
type CustomerVault interface {
	EnsureCustomer(ctx context.Context, req CustomerRequest) (ref string, err error)
	DeleteCustomer(ctx context.Context, ref string) error
}

// AttachRequest saves a card tokenised by the provider's client-side fields.
type AttachRequest struct {
	CustomerRef    string
	ClientToken    string
	IdempotencyKey string
	Now            time.Time
}

// CardMeta is the displayable, non-sensitive description of a card.
type CardMeta struct {
	Brand       string
	Last4       string
	ExpMonth    int
	ExpYear     int
	Country     string // ISO 3166-1 alpha-2 issuing country, when known
	Fingerprint string // provider fingerprint (same card → same value)
}

// MethodAttacher stores and removes cards in the vault.
type MethodAttacher interface {
	Attach(ctx context.Context, req AttachRequest) (methodRef string, meta CardMeta, err error)
	Detach(ctx context.Context, customerRef, methodRef string) error
}

// ChargeReason tells the network who initiated a stored-credential charge.
type ChargeReason string

// Stored-credential reasons.
const (
	// ReasonCustomerInitiated: the customer is present (first membership
	// charge, buying credits with a saved card).
	ReasonCustomerInitiated ChargeReason = "cit"
	// ReasonRecurring: merchant-initiated renewal of a subscription.
	ReasonRecurring ChargeReason = "recurring"
	// ReasonUnscheduled: merchant-initiated, not on a fixed schedule.
	ReasonUnscheduled ChargeReason = "unscheduled"
)

// StoredCharge charges a saved card.
type StoredCharge struct {
	CustomerRef    string
	MethodRef      string
	AmountCents    int64
	Currency       string
	IdempotencyKey string
	Reason         ChargeReason
	// NetworkTxnRef is the network transaction ID of the initial
	// customer-initiated charge (required by some networks for renewals).
	NetworkTxnRef string
	Description   string
	// StatementDescriptor is what the customer's card statement shows
	// (revisions 5.1, §25.3); the sandboxes ignore it.
	StatementDescriptor string
}

// ChargeStatus is a charge outcome reported by a provider.
type ChargeStatus string

// Charge outcomes (declines are DeclineError values).
const (
	ChargeSucceeded      ChargeStatus = "succeeded"
	ChargePending        ChargeStatus = "pending"
	ChargeRequiresAction ChargeStatus = "requires_action"
)

// NextAction asks the customer to complete a step (3-D Secure) in the
// provider's UI.
type NextAction struct {
	Type         string `json:"type"`
	URL          string `json:"url,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
}

// ChargeResult is a successful or pending charge.
type ChargeResult struct {
	Status        ChargeStatus
	ProviderRef   string
	NetworkTxnRef string
	NextAction    *NextAction
}

// StoredCharger charges saved cards. Providers must de-duplicate on
// IdempotencyKey: replaying a key returns the original outcome.
type StoredCharger interface {
	ChargeStored(ctx context.Context, req StoredCharge) (ChargeResult, error)
}

// LookupResult is what the provider knows about an idempotency key.
type LookupResult struct {
	Found   bool
	Result  ChargeResult
	Decline *DeclineError
}

// ChargeLookup finds the outcome of a charge whose response was lost
// (timeout, crash), so it can be reconciled without charging twice.
type ChargeLookup interface {
	Lookup(ctx context.Context, idempotencyKey string) (LookupResult, error)
}

// RefundRequest refunds (part of) a charge.
type RefundRequest struct {
	ProviderRef    string
	AmountCents    int64
	Currency       string
	IdempotencyKey string
	Reason         string
}

// RefundResult identifies the provider-side refund.
type RefundResult struct {
	ProviderRef string
}

// Refunder returns money to the customer.
type Refunder interface {
	Refund(ctx context.Context, req RefundRequest) (RefundResult, error)
}

// ClientConfigRequest asks what the browser needs to tokenise a card.
type ClientConfigRequest struct {
	UserID      uuid.UUID
	Purpose     string
	AmountCents int64
	Currency    string
	Now         time.Time
}

// TestCard is a sandbox card the UI may offer (never real card data).
type TestCard struct {
	Brand     string `json:"brand"`
	Last4     string `json:"last4"`
	Behaviour string `json:"behaviour"`
}

// ClientConfig configures the provider's hosted fields in the browser.
type ClientConfig struct {
	Provider        string     `json:"provider"`
	Type            string     `json:"type"` // sandbox | stripe_elements | nmi_collectjs | authnet_accept_hosted
	PublishableKey  string     `json:"publishable_key,omitempty"`
	TokenizationKey string     `json:"tokenization_key,omitempty"`
	HostedFormURL   string     `json:"hosted_form_url,omitempty"`
	ClientSecret    string     `json:"client_secret,omitempty"`
	TokenFormat     string     `json:"token_format,omitempty"`
	TestCards       []TestCard `json:"test_cards,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
}

// TokenInspector reads the card behind a single-use client token without
// charging it (revisions 5.0, §24.2): the welcome offer needs the card's
// fingerprint before the charge to enforce one offer per payment method.
// Real adapters implement it when the processor's API allows it (Stripe:
// PaymentMethod retrieval); the fingerprint must be the same value Attach
// reports for the same card.
type TokenInspector interface {
	InspectToken(ctx context.Context, token string) (CardMeta, error)
}

// ClientTokenizer configures client-side card entry.
type ClientTokenizer interface {
	ClientConfig(ctx context.Context, req ClientConfigRequest) (ClientConfig, error)
}

// DeclineError is a card decline. Soft declines (insufficient funds,
// processing errors) may succeed when retried later; hard declines (stolen,
// closed, do not honour) need another payment method.
type DeclineError struct {
	Code string
	Soft bool
}

func (e *DeclineError) Error() string {
	kind := "hard"
	if e.Soft {
		kind = "soft"
	}
	return fmt.Sprintf("payment: card declined (%s: %s)", kind, e.Code)
}

// AsDecline extracts a decline from err.
func AsDecline(err error) (*DeclineError, bool) {
	var d *DeclineError
	ok := errors.As(err, &d)
	return d, ok
}

// Provider-side errors.
var (
	// ErrUnknownOutcome means the provider may or may not have charged
	// (timeout, connection reset, 5xx). The charge must be reconciled with
	// Lookup or retried with the same idempotency key.
	ErrUnknownOutcome = errors.New("payment: charge outcome unknown")
	// ErrInvalidToken rejects a malformed or foreign client token.
	ErrInvalidToken = errors.New("payment: invalid payment token")
	// ErrNotFound means the provider has no such customer, card or charge.
	ErrNotFound = errors.New("payment: not found at provider")
	// ErrNoProvider means no configured provider can serve the request.
	ErrNoProvider = errors.New("payment: no provider available for this payment")
	// ErrRefundTooLarge rejects a refund above the remaining amount.
	ErrRefundTooLarge = errors.New("payment: refund exceeds the charge")
)

// Capability is a bit set of provider features.
type Capability uint16

// Capabilities.
const (
	CapCheckout Capability = 1 << iota
	CapWebhooks
	CapVault
	CapAttach
	CapStoredCharge
	CapLookup
	CapRefund
	CapClientConfig
	// CapInspect: the provider can read a token's card without charging it.
	CapInspect
)

var capabilityNames = []struct {
	c    Capability
	name string
}{
	{CapCheckout, "checkout"}, {CapWebhooks, "webhooks"}, {CapVault, "vault"}, {CapAttach, "attach"},
	{CapStoredCharge, "stored_charge"}, {CapLookup, "lookup"}, {CapRefund, "refund"}, {CapClientConfig, "client_config"},
	{CapInspect, "inspect"},
}

// Has reports whether c includes every capability in want.
func (c Capability) Has(want Capability) bool { return c&want == want }

// Names lists the capabilities in c.
func (c Capability) Names() []string {
	out := []string{}
	for _, n := range capabilityNames {
		if c.Has(n.c) {
			out = append(out, n.name)
		}
	}
	return out
}

// CapabilitiesOf discovers what p implements.
func CapabilitiesOf(p Named) Capability {
	var c Capability
	if _, ok := p.(Checkouter); ok {
		c |= CapCheckout
	}
	if _, ok := p.(WebhookVerifier); ok {
		c |= CapWebhooks
	}
	if _, ok := p.(CustomerVault); ok {
		c |= CapVault
	}
	if _, ok := p.(MethodAttacher); ok {
		c |= CapAttach
	}
	if _, ok := p.(StoredCharger); ok {
		c |= CapStoredCharge
	}
	if _, ok := p.(ChargeLookup); ok {
		c |= CapLookup
	}
	if _, ok := p.(Refunder); ok {
		c |= CapRefund
	}
	if _, ok := p.(ClientTokenizer); ok {
		c |= CapClientConfig
	}
	if _, ok := p.(TokenInspector); ok {
		c |= CapInspect
	}
	return c
}

// Purposes a payment can be routed for.
const (
	PurposeAny          = "any"
	PurposeCredits      = "credits"
	PurposeSubscription = "subscription"
)

// Rule is a routing rule (see domain.RoutingRule).
type Rule struct {
	ID         string
	Priority   int
	Enabled    bool
	Purpose    string
	Providers  []string
	Countries  []string
	Currencies []string
	PlanIDs    []string
	UserIDs    []string
}

// RouteInput describes a payment to route.
type RouteInput struct {
	UserID string
	// Countries are the trusted country signals (edge country, card issuing
	// country). When several are known, a provider must be allowed for each
	// of them: the stricter outcome wins.
	Countries []string
	Currency  string
	PlanID    string
	Purpose   string // credits | subscription | any
	Needs     Capability
}

// Registry holds the configured providers and routes payments to them.
type Registry struct {
	providers   map[string]Named
	order       []string
	defaultName string
}

// NewRegistry registers providers; defaultName is used when no rule
// matches (it must be one of them, or empty for none).
func NewRegistry(defaultName string, providers ...Named) *Registry {
	r := &Registry{providers: map[string]Named{}, defaultName: defaultName}
	for _, p := range providers {
		if p == nil {
			continue
		}
		if _, dup := r.providers[p.Name()]; dup {
			continue
		}
		r.providers[p.Name()] = p
		r.order = append(r.order, p.Name())
	}
	return r
}

// Get returns a registered provider.
func (r *Registry) Get(name string) (Named, bool) {
	if r == nil {
		return nil, false
	}
	p, ok := r.providers[name]
	return p, ok
}

// Names lists registered providers in registration order.
func (r *Registry) Names() []string {
	if r == nil {
		return nil
	}
	return append([]string(nil), r.order...)
}

// Empty reports whether no provider is configured (payments disabled).
func (r *Registry) Empty() bool { return r == nil || len(r.providers) == 0 }

// Route returns the providers to try, in order, and the ID of the rule
// that selected them ("" for the default). The first enabled rule (lowest
// priority, then ID) whose purpose and non-empty match lists all apply
// decides: its registered providers having the needed capabilities are
// returned; when none qualifies the payment is refused (a rule never falls
// through to a less specific one).
func (r *Registry) Route(in RouteInput, rules []Rule) ([]Named, string, error) {
	if r.Empty() {
		return nil, "", ErrNoProvider
	}
	countries := in.Countries
	if len(countries) == 0 {
		countries = []string{""}
	}
	var result []Named
	ruleID := ""
	for i, country := range countries {
		names, id := r.routeOne(in, country, rules)
		var usable []Named
		for _, n := range names {
			if p, ok := r.providers[n]; ok && CapabilitiesOf(p).Has(in.Needs) {
				usable = append(usable, p)
			}
		}
		if i == 0 {
			result, ruleID = usable, id
			continue
		}
		result = intersect(result, usable)
		if id != ruleID {
			ruleID = ruleID + "+" + id
		}
	}
	if len(result) == 0 {
		return nil, ruleID, ErrNoProvider
	}
	return result, ruleID, nil
}

func (r *Registry) routeOne(in RouteInput, country string, rules []Rule) ([]string, string) {
	sorted := append([]Rule(nil), rules...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Priority != sorted[j].Priority {
			return sorted[i].Priority < sorted[j].Priority
		}
		return sorted[i].ID < sorted[j].ID
	})
	for _, rule := range sorted {
		if !rule.Enabled || !purposeMatches(rule.Purpose, in.Purpose) {
			continue
		}
		if !listMatches(rule.Countries, country) || !listMatches(rule.Currencies, in.Currency) ||
			!listMatches(rule.PlanIDs, in.PlanID) || !listMatches(rule.UserIDs, in.UserID) {
			continue
		}
		return rule.Providers, rule.ID
	}
	if r.defaultName == "" {
		return nil, ""
	}
	return []string{r.defaultName}, ""
}

func purposeMatches(rule, want string) bool {
	return rule == "" || rule == PurposeAny || want == "" || want == PurposeAny || rule == want
}

// listMatches: an empty list matches everything; otherwise the value must
// be present (an unknown value never matches a restricted list).
func listMatches(list []string, v string) bool {
	if len(list) == 0 {
		return true
	}
	if v == "" {
		return false
	}
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func intersect(a, b []Named) []Named {
	in := map[string]bool{}
	for _, p := range b {
		in[p.Name()] = true
	}
	var out []Named
	for _, p := range a {
		if in[p.Name()] {
			out = append(out, p)
		}
	}
	return out
}
