package payment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// Saved-card sandboxes for development and automated tests. They simulate
// a processor with a customer vault and idempotent charges; configuration
// refuses them in production unless explicitly allowed.
//
// Client tokens (what the provider's hosted fields would return):
//
//	sbx_a_<brand>_<last4>_<mm>_<yyyy>[_<country>]   e.g. sbx_a_visa_4242_12_2030_fr
//
// Card behaviour by last4: 0002 hard decline, 9995 soft decline (insufficient
// funds), anything else approves. Amount behaviour (cents % 100): .13 hard
// decline, .14 soft decline, .15 timeout — the charge is made but the
// response is lost, so the caller must reconcile (Lookup) or retry with the
// same idempotency key.
//
// revisions 6.0 (§26.5), test support for card payment operations:
//
//   - Card last4 3220 asks for a 3-D Secure step: the charge answers pending with
//     next_action {type: "sandbox_3ds"} (a saved card answers requires_action) and
//     completes as succeeded (3-D Secure "authenticated", the card, a normal risk
//     level) on the first RetrieveCharge made at least 2 s after the charge. Last4
//     3221 completes as a decline, authentication_failed. Lookup and replays keep
//     answering "needs the customer" until then.
//   - Every sandbox provider implements ChargeRetriever. SandboxA also implements
//     DisputeManager (evidence and accept are recorded on the processor, see
//     SandboxProcessor.DisputeEvidence), AccountInspector (a fake account the test
//     configures with SetAccount and SetWebhookEndpoints) and EventFetcher (the
//     events RecordEvent stored, and the refunds the processor made).
//   - Signed generic webhook events (payment.WebhookEvent as JSON) already carry
//     every revisions 6.0 field: dispute, fraud warning, refund, card, risk and
//     3-D Secure.

// SandboxProcessor is the simulated processor state. Several provider
// instances (API replicas) may share one processor.
type SandboxProcessor struct {
	mu        sync.Mutex
	seq       int
	customers map[string]bool
	methods   map[string]*sbxMethod
	charges   map[string]*sbxCharge // by idempotency key
	byRef     map[string]*sbxCharge // by provider reference
	refunds   map[string]RefundResult
	attaches  map[string]sbxAttach
	calls     map[string]int // charge requests per idempotency key
	// descriptors records the statement descriptor of each charge request by
	// idempotency key (revisions 5.1): the sandboxes do not use it, tests read it.
	descriptors map[string]string
	// revisions 6.0 (§26.5): see sandbox_ops.go.
	ops                        sandboxOps
	customerOrder, methodOrder []string // creation order, for tests that craft provider events
}

type sbxMethod struct {
	ref, customer string
	meta          CardMeta
	detached      bool
}

type sbxCharge struct {
	key, ref, method string
	amount, refunded int64
	currency         string
	decline          *DeclineError
	networkRef       string
	// revisions 6.0: a 3-D Secure challenge (last4 3220 / 3221) waits for the
	// first RetrieveCharge at least sandboxChallengeDelay after created.
	challenge bool
	done      bool
	created   time.Time
	card      CardMeta
	// disputed: the cardholder disputed the charge (SandboxProcessor.DisputeCharge);
	// refunding it is refused, like a real processor does.
	disputed bool
}

type sbxAttach struct {
	ref  string
	meta CardMeta
}

// NewSandboxProcessor returns an empty simulated processor.
func NewSandboxProcessor() *SandboxProcessor {
	return &SandboxProcessor{customers: map[string]bool{}, methods: map[string]*sbxMethod{}, charges: map[string]*sbxCharge{},
		byRef: map[string]*sbxCharge{}, refunds: map[string]RefundResult{}, attaches: map[string]sbxAttach{}, calls: map[string]int{},
		descriptors: map[string]string{}}
}

// ChargeRequests returns how many times a charge with this idempotency key
// was requested (replays included).
func (p *SandboxProcessor) ChargeRequests(key string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls[key]
}

// StatementDescriptor returns the statement descriptor the last charge request
// with this idempotency key carried (revisions 5.1, §25.3).
func (p *SandboxProcessor) StatementDescriptor(key string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.descriptors[key]
}

// Charged reports whether a charge exists for the key, and its amount.
func (p *SandboxProcessor) Charged(key string) (int64, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.charges[key]
	if !ok || c.decline != nil || c.waiting() {
		return 0, false
	}
	return c.amount, true
}

// SuccessfulCharges counts approved charges.
func (p *SandboxProcessor) SuccessfulCharges() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, c := range p.charges {
		if c.decline == nil && !c.waiting() {
			n++
		}
	}
	return n
}

func (p *SandboxProcessor) next(prefix string) string {
	p.seq++
	return fmt.Sprintf("%s_%d%s", prefix, p.seq, security.MustRandomToken(6))
}

var sandboxToken = regexp.MustCompile(`^sbx_([ab])_(visa|mastercard|amex|discover)_([0-9]{4})_([0-9]{1,2})_([0-9]{4})(?:_([a-z]{2}))?$`)

func parseSandboxToken(letter, token string, now time.Time) (CardMeta, error) {
	m := sandboxToken.FindStringSubmatch(token)
	if m == nil || m[1] != letter {
		return CardMeta{}, ErrInvalidToken
	}
	month, _ := strconv.Atoi(m[4])
	year, _ := strconv.Atoi(m[5])
	if month < 1 || month > 12 || year < 2000 || year > 2100 {
		return CardMeta{}, ErrInvalidToken
	}
	meta := CardMeta{Brand: m[2], Last4: m[3], ExpMonth: month, ExpYear: year, Country: strings.ToUpper(m[6])}
	now = now.UTC()
	if now.Year() > year || (now.Year() == year && int(now.Month()) > month) {
		return CardMeta{}, &DeclineError{Code: "expired_card"}
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%02d|%d", meta.Brand, meta.Last4, meta.ExpMonth, meta.ExpYear)))
	meta.Fingerprint = "fp_" + hex.EncodeToString(sum[:8])
	return meta, nil
}

func outcomeFor(last4 string, amountCents int64) (decline *DeclineError, timeout bool) {
	switch amountCents % 100 {
	case 13:
		return &DeclineError{Code: "card_declined"}, false
	case 14:
		return &DeclineError{Code: "processing_error", Soft: true}, false
	case 15:
		return nil, true
	}
	switch last4 {
	case "0002":
		return &DeclineError{Code: "card_declined"}, false
	case "9995":
		return &DeclineError{Code: "insufficient_funds", Soft: true}, false
	}
	return nil, false
}

// sandboxCore implements the shared behaviour; the exported provider types
// expose different capability sets.
type sandboxCore struct {
	name      string
	letter    string
	proc      *SandboxProcessor
	secret    []byte
	tolerance time.Duration
}

func (s *sandboxCore) Name() string { return s.name }

func (s *sandboxCore) ensureCustomer(ctx context.Context, req CustomerRequest) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	p := s.proc
	p.mu.Lock()
	defer p.mu.Unlock()
	if req.ExistingRef != "" && p.customers[req.ExistingRef] {
		return req.ExistingRef, nil
	}
	ref := p.next("cus_sbx_" + s.letter)
	p.customers[ref] = true
	p.customerOrder = append(p.customerOrder, ref)
	return ref, nil
}

func (s *sandboxCore) deleteCustomer(_ context.Context, ref string) error {
	p := s.proc
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.customers[ref] {
		return ErrNotFound
	}
	delete(p.customers, ref)
	for _, m := range p.methods {
		if m.customer == ref {
			m.detached = true
		}
	}
	return nil
}

func (s *sandboxCore) attach(ctx context.Context, req AttachRequest) (string, CardMeta, error) {
	if err := ctx.Err(); err != nil {
		return "", CardMeta{}, err
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	meta, err := parseSandboxToken(s.letter, req.ClientToken, now)
	if err != nil {
		return "", CardMeta{}, err
	}
	p := s.proc
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.customers[req.CustomerRef] {
		return "", CardMeta{}, ErrNotFound
	}
	if req.IdempotencyKey != "" {
		if a, ok := p.attaches[req.IdempotencyKey]; ok {
			return a.ref, a.meta, nil
		}
	}
	ref := p.next("pm_sbx_" + s.letter)
	p.methods[ref] = &sbxMethod{ref: ref, customer: req.CustomerRef, meta: meta}
	p.methodOrder = append(p.methodOrder, ref)
	if req.IdempotencyKey != "" {
		p.attaches[req.IdempotencyKey] = sbxAttach{ref: ref, meta: meta}
	}
	return ref, meta, nil
}

// inspectToken reads a token's card: the same parsing and fingerprint
// derivation as attach, with no vault or processor state touched.
func (s *sandboxCore) inspectToken(ctx context.Context, token string) (CardMeta, error) {
	if err := ctx.Err(); err != nil {
		return CardMeta{}, err
	}
	return parseSandboxToken(s.letter, token, time.Now())
}

func (s *sandboxCore) detach(_ context.Context, customerRef, methodRef string) error {
	p := s.proc
	p.mu.Lock()
	defer p.mu.Unlock()
	m, ok := p.methods[methodRef]
	if !ok || m.customer != customerRef {
		return ErrNotFound
	}
	m.detached = true
	return nil
}

func (s *sandboxCore) chargeStored(ctx context.Context, req StoredCharge) (ChargeResult, error) {
	if err := ctx.Err(); err != nil {
		return ChargeResult{}, err
	}
	if req.IdempotencyKey == "" || req.AmountCents <= 0 {
		return ChargeResult{}, fmt.Errorf("payment: sandbox: invalid charge request")
	}
	p := s.proc
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls[req.IdempotencyKey]++
	p.descriptors[req.IdempotencyKey] = req.StatementDescriptor
	if c, ok := p.charges[req.IdempotencyKey]; ok { // idempotent replay
		res, decline := c.current()
		if decline != nil {
			return ChargeResult{}, decline
		}
		return res, nil
	}
	m, ok := p.methods[req.MethodRef]
	if !ok || m.detached || m.customer != req.CustomerRef {
		return ChargeResult{}, &DeclineError{Code: "payment_method_unavailable"}
	}
	decline, timeout := outcomeFor(m.meta.Last4, req.AmountCents)
	c := &sbxCharge{key: req.IdempotencyKey, ref: p.next("ch_sbx_" + s.letter), method: m.ref, amount: req.AmountCents,
		currency: req.Currency, decline: decline, created: time.Now(), card: m.meta}
	if decline == nil {
		c.networkRef = p.next("ntx")
		c.challenge = isChallengeCard(m.meta.Last4)
	}
	p.charges[req.IdempotencyKey] = c
	p.byRef[c.ref] = c
	if timeout {
		return ChargeResult{}, ErrUnknownOutcome
	}
	res, decline := c.current()
	if decline != nil {
		return ChargeResult{}, decline
	}
	return res, nil
}

func (s *sandboxCore) lookup(_ context.Context, key string) (LookupResult, error) {
	p := s.proc
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.charges[key]
	if !ok {
		return LookupResult{}, nil
	}
	res, decline := c.current()
	if decline != nil {
		return LookupResult{Found: true, Decline: decline}, nil
	}
	return LookupResult{Found: true, Result: res}, nil
}

func (s *sandboxCore) refund(_ context.Context, req RefundRequest) (RefundResult, error) {
	p := s.proc
	p.mu.Lock()
	defer p.mu.Unlock()
	if r, ok := p.refunds[req.IdempotencyKey]; ok && req.IdempotencyKey != "" {
		return r, nil
	}
	c, ok := p.byRef[req.ProviderRef]
	if !ok || c.decline != nil || c.waiting() {
		return RefundResult{}, ErrNotFound
	}
	if c.disputed {
		return RefundResult{}, ErrChargeDisputed
	}
	if req.AmountCents <= 0 || req.AmountCents > c.amount-c.refunded {
		return RefundResult{}, ErrRefundTooLarge
	}
	c.refunded += req.AmountCents
	r := RefundResult{ProviderRef: p.next("re_sbx_" + s.letter)}
	if req.IdempotencyKey != "" {
		p.refunds[req.IdempotencyKey] = r
	}
	p.ops.recordRefund(r.ProviderRef, c.ref, req.AmountCents, req.Currency) // revisions 6.0: the event an admin replay can fetch
	return r, nil
}

func (s *sandboxCore) clientConfig(_ context.Context, _ ClientConfigRequest) (ClientConfig, error) {
	return ClientConfig{
		Provider: s.name, Type: "sandbox", PublishableKey: "pk_test_" + s.name,
		TokenFormat: "sbx_" + s.letter + "_<brand>_<last4>_<mm>_<yyyy>[_<country>]",
		TestCards: []TestCard{
			{Brand: "visa", Last4: "4242", Behaviour: "approved"},
			{Brand: "mastercard", Last4: "4444", Behaviour: "approved"},
			{Brand: "visa", Last4: "9995", Behaviour: "soft_decline"},
			{Brand: "visa", Last4: "0002", Behaviour: "hard_decline"},
			{Brand: "visa", Last4: "3220", Behaviour: "3ds_success"}, // revisions 6.0
			{Brand: "visa", Last4: "3221", Behaviour: "3ds_failure"},
		},
	}, nil
}

func (s *sandboxCore) checkout(ctx context.Context, req CheckoutRequest) (CheckoutResult, error) {
	if err := ctx.Err(); err != nil {
		return CheckoutResult{}, err
	}
	last4 := ""
	var card CardMeta
	if req.PaymentToken != "" {
		meta, err := parseSandboxToken(s.letter, req.PaymentToken, time.Now())
		if err != nil {
			if d, ok := AsDecline(err); ok {
				return CheckoutResult{Status: CheckoutFailed, FailureCode: d.Code}, nil
			}
			return CheckoutResult{}, err
		}
		last4, card = meta.Last4, meta
	}
	key := req.IdempotencyKey
	if key == "" {
		key = "checkout:" + req.PurchaseID.String()
	}
	p := s.proc
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls[key]++
	p.descriptors[key] = req.StatementDescriptor
	if c, ok := p.charges[key]; ok {
		return c.checkoutResult(), nil
	}
	decline, timeout := outcomeFor(last4, req.AmountCents)
	c := &sbxCharge{key: key, ref: p.next("ch_sbx_" + s.letter), amount: req.AmountCents, currency: req.Currency, decline: decline,
		created: time.Now(), card: card}
	c.challenge = decline == nil && req.PaymentToken != "" && isChallengeCard(last4)
	p.charges[key] = c
	p.byRef[c.ref] = c
	if timeout {
		return CheckoutResult{}, ErrUnknownOutcome
	}
	return c.checkoutResult(), nil
}

// SandboxA is a full-featured sandbox (hosted checkout, vault, stored
// charges, lookup, refunds, client config and signed webhooks).
type SandboxA struct{ core sandboxCore }

// NewSandboxA returns the sandbox_a provider.
func NewSandboxA(proc *SandboxProcessor, webhookSecret string, tolerance time.Duration) *SandboxA {
	return &SandboxA{core: sandboxCore{name: "sandbox_a", letter: "a", proc: proc, secret: []byte(webhookSecret), tolerance: tolerance}}
}

// Name implements Named.
func (s *SandboxA) Name() string { return s.core.Name() }

// Checkout implements Checkouter.
func (s *SandboxA) Checkout(ctx context.Context, req CheckoutRequest) (CheckoutResult, error) {
	return s.core.checkout(ctx, req)
}

// VerifyWebhookRequest implements WebhookVerifier (header
// X-Staryield-Signature: t=<unix>,v1=<hmac-sha256>).
func (s *SandboxA) VerifyWebhookRequest(req WebhookRequest) (*WebhookEvent, error) {
	return VerifySignedPayload(s.core.secret, s.core.tolerance, req.Header.Get("X-Staryield-Signature"), req.Body, req.Now)
}

// EnsureCustomer implements CustomerVault.
func (s *SandboxA) EnsureCustomer(ctx context.Context, req CustomerRequest) (string, error) {
	return s.core.ensureCustomer(ctx, req)
}

// DeleteCustomer implements CustomerVault.
func (s *SandboxA) DeleteCustomer(ctx context.Context, ref string) error {
	return s.core.deleteCustomer(ctx, ref)
}

// Attach implements MethodAttacher.
func (s *SandboxA) Attach(ctx context.Context, req AttachRequest) (string, CardMeta, error) {
	return s.core.attach(ctx, req)
}

// Detach implements MethodAttacher.
func (s *SandboxA) Detach(ctx context.Context, customerRef, methodRef string) error {
	return s.core.detach(ctx, customerRef, methodRef)
}

// ChargeStored implements StoredCharger.
func (s *SandboxA) ChargeStored(ctx context.Context, req StoredCharge) (ChargeResult, error) {
	return s.core.chargeStored(ctx, req)
}

// Lookup implements ChargeLookup.
func (s *SandboxA) Lookup(ctx context.Context, key string) (LookupResult, error) {
	return s.core.lookup(ctx, key)
}

// Refund implements Refunder.
func (s *SandboxA) Refund(ctx context.Context, req RefundRequest) (RefundResult, error) {
	return s.core.refund(ctx, req)
}

// ClientConfig implements ClientTokenizer.
func (s *SandboxA) ClientConfig(ctx context.Context, req ClientConfigRequest) (ClientConfig, error) {
	return s.core.clientConfig(ctx, req)
}

// InspectToken implements TokenInspector.
func (s *SandboxA) InspectToken(ctx context.Context, token string) (CardMeta, error) {
	return s.core.inspectToken(ctx, token)
}

// SandboxB is a vault-only sandbox (saved cards, stored charges, refunds and
// client config; no hosted checkout, lookup or webhooks) — like a gateway
// reached only through its customer vault.
type SandboxB struct{ core sandboxCore }

// NewSandboxB returns the sandbox_b provider.
func NewSandboxB(proc *SandboxProcessor) *SandboxB {
	return &SandboxB{core: sandboxCore{name: "sandbox_b", letter: "b", proc: proc}}
}

// Name implements Named.
func (s *SandboxB) Name() string { return s.core.Name() }

// EnsureCustomer implements CustomerVault.
func (s *SandboxB) EnsureCustomer(ctx context.Context, req CustomerRequest) (string, error) {
	return s.core.ensureCustomer(ctx, req)
}

// DeleteCustomer implements CustomerVault.
func (s *SandboxB) DeleteCustomer(ctx context.Context, ref string) error {
	return s.core.deleteCustomer(ctx, ref)
}

// Attach implements MethodAttacher.
func (s *SandboxB) Attach(ctx context.Context, req AttachRequest) (string, CardMeta, error) {
	return s.core.attach(ctx, req)
}

// Detach implements MethodAttacher.
func (s *SandboxB) Detach(ctx context.Context, customerRef, methodRef string) error {
	return s.core.detach(ctx, customerRef, methodRef)
}

// ChargeStored implements StoredCharger.
func (s *SandboxB) ChargeStored(ctx context.Context, req StoredCharge) (ChargeResult, error) {
	return s.core.chargeStored(ctx, req)
}

// Refund implements Refunder.
func (s *SandboxB) Refund(ctx context.Context, req RefundRequest) (RefundResult, error) {
	return s.core.refund(ctx, req)
}

// ClientConfig implements ClientTokenizer.
func (s *SandboxB) ClientConfig(ctx context.Context, req ClientConfigRequest) (ClientConfig, error) {
	return s.core.clientConfig(ctx, req)
}

// InspectToken implements TokenInspector.
func (s *SandboxB) InspectToken(ctx context.Context, token string) (CardMeta, error) {
	return s.core.inspectToken(ctx, token)
}

// RetrieveCharge implements ChargeRetriever (revisions 6.0).
func (s *SandboxA) RetrieveCharge(ctx context.Context, ref string) (ChargeResult, *DeclineError, error) {
	return s.core.retrieveCharge(ctx, ref)
}

// RetrieveCharge implements ChargeRetriever (revisions 6.0).
func (s *SandboxB) RetrieveCharge(ctx context.Context, ref string) (ChargeResult, *DeclineError, error) {
	return s.core.retrieveCharge(ctx, ref)
}

// KnownProviders are the provider names configuration accepts.
var KnownProviders = map[string]bool{"sandbox_a": true, "sandbox_b": true}

// IsSandbox reports whether name is a sandbox provider.
func IsSandbox(name string) bool { return strings.HasPrefix(name, "sandbox") }
