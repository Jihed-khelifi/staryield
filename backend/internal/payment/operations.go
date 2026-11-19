package payment

import (
	"context"
	"errors"
	"time"
)

// revisions 6.0 (docs/platform-extensions.md §26.5): card-payment
// operations a real processor (Stripe) adds on top of the saved-card
// capabilities: retrieving a charge after the customer finished a 3-D Secure
// step, managing disputes and reporting the account's status. Webhook events
// of every provider are normalised to the Event* types below.

// Normalised webhook event types (WebhookEvent.Type). The legacy hosted
// checkout keeps payment.succeeded | payment.failed | payment.refunded.
const (
	EventChargeSucceeded        = "charge.succeeded"
	EventChargeFailed           = "charge.failed"
	EventChargeProcessing       = "charge.processing"
	EventChargeRefunded         = "charge.refunded" // one refund (RefundRef, AmountCents)
	EventRefundFailed           = "refund.failed"   // a refund the processor could not complete
	EventChargeDisputed         = "charge.disputed" // a dispute (or inquiry) opened
	EventDisputeUpdated         = "charge.dispute_updated"
	EventDisputeClosed          = "charge.dispute_closed" // DisputeStatus: won | lost | warning_closed | prevented
	EventDisputeFundsWithdrawn  = "charge.dispute_funds_withdrawn"
	EventDisputeFundsReinstated = "charge.dispute_funds_reinstated"
	EventFraudWarning           = "fraud.early_warning" // ObjectRef, Actionable, FraudType
	EventMethodDetached         = "method.detached"     // ObjectRef = the payment method
	EventCustomerDeleted        = "customer.deleted"    // ObjectRef = the customer
	EventReviewOpened           = "review.opened"       // the processor's manual fraud review
	EventReviewClosed           = "review.closed"
	// EventIgnored marks a verified event the platform does not act on; it
	// is still recorded in the webhook log.
	EventIgnored = "ignored"
)

// Risk levels (RiskInfo.Level).
const (
	RiskNormal      = "normal"
	RiskElevated    = "elevated"
	RiskHighest     = "highest"
	RiskNotAssessed = "not_assessed"
	RiskUnknown     = "unknown"
)

// 3-D Secure policies (CheckoutRequest.ThreeDSecure, site setting
// payment_policies.three_d_secure).
const (
	ThreeDSAutomatic = "automatic"
	ThreeDSAny       = "any"
)

// ErrChargeDisputed means the processor refuses to refund a charge that is
// disputed: the dispute must be answered (or accepted) instead.
var ErrChargeDisputed = errors.New("payment: the charge is disputed")

// RiskInfo is the processor's fraud assessment of a charge.
type RiskInfo struct {
	Level string `json:"level"`
	// Score is 0–99 (higher is riskier); -1 when the processor gives none.
	Score int `json:"score"`
}

// ChargeRetriever reads the current state of a charge by its provider
// reference, e.g. after the customer completed (or abandoned) 3-D Secure in
// the browser. A declined or abandoned charge is reported as a DeclineError
// (with a nil error); a charge still awaiting the customer has status
// ChargeRequiresAction and its NextAction.
type ChargeRetriever interface {
	RetrieveCharge(ctx context.Context, providerRef string) (ChargeResult, *DeclineError, error)
}

// DisputeEvidence is the text evidence submitted for a dispute. Empty
// fields are left out. ReceiptPDF, when set, is uploaded as the receipt.
type DisputeEvidence struct {
	ProductDescription     string
	CustomerName           string
	CustomerEmail          string
	BillingCountry         string
	ServiceDate            string // YYYY-MM-DD
	AccessActivityLog      string
	RefundPolicyDisclosure string
	CancellationRebuttal   string
	UncategorizedText      string
	ReceiptPDF             []byte
	ReceiptFilename        string
}

// DisputeManager answers disputes at the processor.
type DisputeManager interface {
	// SubmitDisputeEvidence saves the evidence; submit=true sends it to the
	// card network (final: it cannot be edited afterwards).
	SubmitDisputeEvidence(ctx context.Context, disputeRef string, ev DisputeEvidence, submit bool) error
	// AcceptDispute concedes the dispute (it closes as lost).
	AcceptDispute(ctx context.Context, disputeRef string) error
}

// WebhookEndpointInfo is a webhook endpoint registered at the processor.
type WebhookEndpointInfo struct {
	ID            string   `json:"id"`
	URL           string   `json:"url"`
	Status        string   `json:"status"` // enabled | disabled
	EnabledEvents []string `json:"enabled_events"`
	APIVersion    string   `json:"api_version,omitempty"`
}

// AccountStatus describes the processor account (admin status page).
type AccountStatus struct {
	AccountID           string                `json:"account_id"`
	Livemode            bool                  `json:"livemode"`
	Country             string                `json:"country,omitempty"`
	DefaultCurrency     string                `json:"default_currency,omitempty"`
	ChargesEnabled      bool                  `json:"charges_enabled"`
	PayoutsEnabled      bool                  `json:"payouts_enabled"`
	DetailsSubmitted    bool                  `json:"details_submitted"`
	StatementDescriptor string                `json:"statement_descriptor,omitempty"`
	RequirementsDue     []string              `json:"requirements_due"`
	WebhookEndpoints    []WebhookEndpointInfo `json:"webhook_endpoints"`
	APIVersion          string                `json:"api_version,omitempty"`
	CheckedAt           time.Time             `json:"checked_at"`
}

// AccountInspector reports the processor account's status.
type AccountInspector interface {
	AccountStatus(ctx context.Context) (AccountStatus, error)
}

// EventFetcher re-reads a webhook event from the provider by its id and
// normalises it like VerifyWebhookRequest does (admin replay of a failed
// delivery, §26.5). Signature checks do not apply: the event comes straight
// from the provider's API.
type EventFetcher interface {
	FetchEvent(ctx context.Context, eventID string) (*WebhookEvent, error)
}
