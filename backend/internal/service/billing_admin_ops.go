package service

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 6.0 (§26.5, §26.7): what the admin console shows and does about
// card payments: the processor's status, the webhook log (and replay),
// disputes (with the evidence draft) and the extra columns of the payments list.

// ------------------------------------------------------------- status page

// stripeSubscribedEvents are the events the webhook endpoint must be
// subscribed to (the contract's table).
var stripeSubscribedEvents = []string{
	"payment_intent.succeeded", "payment_intent.payment_failed", "payment_intent.canceled", "payment_intent.processing",
	"refund.created", "refund.updated", "refund.failed",
	"charge.dispute.created", "charge.dispute.updated", "charge.dispute.closed", "charge.dispute.funds_withdrawn", "charge.dispute.funds_reinstated",
	"radar.early_fraud_warning.created", "radar.early_fraud_warning.updated",
	"payment_method.detached", "customer.deleted", "review.opened", "review.closed",
}

// providerProbe is the cached answer of the processor's account call.
type providerProbe struct {
	at     time.Time
	status *payment.AccountStatus
	err    error
}

// accountStatus returns the processor's account status (cached for 60 s,
// failures included, so a down processor is not hammered by the console).
func (s *BillingService) accountStatus(ctx context.Context, provider string) (*payment.AccountStatus, error) {
	p, ok := s.registry.Get(provider)
	if !ok {
		return nil, payment.ErrNotFound
	}
	inspector, ok := p.(payment.AccountInspector)
	if !ok {
		return nil, payment.ErrNotFound
	}
	now := s.clock.Now()
	s.ops.mu.Lock()
	if c, ok := s.ops.status[provider]; ok && now.Sub(c.at) < statusCacheTTL {
		s.ops.mu.Unlock()
		return c.status, c.err
	}
	s.ops.mu.Unlock()
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	st, err := inspector.AccountStatus(cctx)
	probe := &providerProbe{at: now, err: err}
	if err == nil {
		probe.status = &st
	} else {
		logger.FromContext(ctx).Warn("billing: provider account status unavailable", zap.Error(err), zap.String("provider", provider))
	}
	s.ops.mu.Lock()
	s.ops.status[provider] = probe
	s.ops.mu.Unlock()
	return probe.status, probe.err
}

// DescriptorCheck compares the statement descriptor the account uses with the
// one Staryield expects (the processor ignores per-charge descriptors).
type DescriptorCheck struct {
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Matches  bool   `json:"matches"`
}

// WebhookStatus is the state of the webhook endpoint.
type WebhookStatus struct {
	ExpectedURL   string                        `json:"expected_url"`
	Endpoints     []payment.WebhookEndpointInfo `json:"endpoints"`
	MissingEvents []string                      `json:"missing_events"`
	LastEventAt   *time.Time                    `json:"last_event_at,omitempty"`
	Received24h   int64                         `json:"received_24h"`
	Failed24h     int64                         `json:"failed_24h"`
}

// ProviderStatusView is GET /admin/billing/stripe/status.
type ProviderStatusView struct {
	Configured bool                   `json:"configured"`
	Livemode   bool                   `json:"livemode"`
	Account    *payment.AccountStatus `json:"account,omitempty"`
	Descriptor DescriptorCheck        `json:"descriptor"`
	Webhook    WebhookStatus          `json:"webhook"`
	Warnings   []string               `json:"warnings"`
}

// ProviderStatus reports the processor account, descriptor and webhook
// endpoint of a provider (Stripe on the console). origin is the public origin
// of this API (the webhook URL the endpoint must point at). It works when the
// provider is absent: configured is false and only the log is summarised.
func (s *BillingService) ProviderStatus(ctx context.Context, provider, origin string) (*ProviderStatusView, error) {
	if s.ops == nil {
		return nil, apperror.FeatureUnavailable("Payment operations are not available.")
	}
	out := &ProviderStatusView{Warnings: []string{}, Descriptor: DescriptorCheck{Expected: s.cfg.StatementDescriptor},
		Webhook: WebhookStatus{ExpectedURL: strings.TrimRight(origin, "/") + "/api/v1/webhooks/payments/" + provider,
			Endpoints: []payment.WebhookEndpointInfo{}, MissingEvents: []string{}}}
	if counts, err := s.ops.repo.WebhookCounts(ctx, provider, s.clock.Now().Add(-24*time.Hour)); err == nil {
		out.Webhook.Received24h, out.Webhook.Failed24h, out.Webhook.LastEventAt = counts.Received, counts.Failed, counts.LastAt
	} else {
		return nil, apperror.Internal(err)
	}
	p, registered := s.registry.Get(provider)
	if !registered {
		out.Warnings = append(out.Warnings, "This payment provider is not configured.")
		return out, nil
	}
	out.Configured = true
	if _, ok := p.(payment.AccountInspector); !ok {
		out.Warnings = append(out.Warnings, "This payment provider cannot report its account status.")
		return out, nil
	}
	st, err := s.accountStatus(ctx, provider)
	if err != nil {
		out.Warnings = append(out.Warnings, "The payment provider did not answer. Try again in a minute.")
		return out, nil
	}
	out.Account, out.Livemode = st, st.Livemode
	out.Descriptor.Actual = st.StatementDescriptor
	out.Descriptor.Matches = st.StatementDescriptor != "" && strings.EqualFold(strings.TrimSpace(st.StatementDescriptor), strings.TrimSpace(out.Descriptor.Expected))
	if st.WebhookEndpoints != nil {
		out.Webhook.Endpoints = st.WebhookEndpoints
	}
	out.Webhook.MissingEvents = missingEvents(st.WebhookEndpoints, out.Webhook.ExpectedURL)
	if !st.ChargesEnabled {
		out.Warnings = append(out.Warnings, "The account cannot take charges yet.")
	}
	if !st.PayoutsEnabled {
		out.Warnings = append(out.Warnings, "Payouts are not enabled on the account.")
	}
	if len(st.RequirementsDue) > 0 {
		out.Warnings = append(out.Warnings, "The account has information due: "+strings.Join(st.RequirementsDue, ", ")+".")
	}
	if !out.Descriptor.Matches {
		out.Warnings = append(out.Warnings, "The account's statement descriptor does not match the expected one.")
	}
	switch endpoint := endpointFor(st.WebhookEndpoints, out.Webhook.ExpectedURL); {
	case endpoint == nil:
		out.Warnings = append(out.Warnings, "No webhook endpoint points at this API.")
	case endpoint.Status != "enabled":
		out.Warnings = append(out.Warnings, "The webhook endpoint is disabled.")
	case len(out.Webhook.MissingEvents) > 0:
		out.Warnings = append(out.Warnings, "The webhook endpoint is missing events: "+strings.Join(out.Webhook.MissingEvents, ", ")+".")
	}
	if out.Webhook.Failed24h > 0 {
		out.Warnings = append(out.Warnings, "Some webhook deliveries failed in the last 24 hours.")
	}
	return out, nil
}

// endpointFor finds the endpoint that points at url.
func endpointFor(endpoints []payment.WebhookEndpointInfo, url string) *payment.WebhookEndpointInfo {
	want := strings.TrimRight(url, "/")
	for i := range endpoints {
		if strings.TrimRight(endpoints[i].URL, "/") == want {
			return &endpoints[i]
		}
	}
	return nil
}

// missingEvents lists the events the endpoint at url is not subscribed to
// (every one of them when there is no such endpoint; "*" subscribes to all).
func missingEvents(endpoints []payment.WebhookEndpointInfo, url string) []string {
	e := endpointFor(endpoints, url)
	if e == nil {
		return slices.Clone(stripeSubscribedEvents)
	}
	if slices.Contains(e.EnabledEvents, "*") {
		return []string{}
	}
	missing := []string{}
	for _, ev := range stripeSubscribedEvents {
		if !slices.Contains(e.EnabledEvents, ev) {
			missing = append(missing, ev)
		}
	}
	return missing
}

// ------------------------------------------------------------- webhook log

// WebhookEventRow is a delivery in the webhook log.
type WebhookEventRow struct {
	ID           uuid.UUID  `json:"id"`
	Provider     string     `json:"provider"`
	EventID      string     `json:"event_id"`
	EventType    string     `json:"event_type"`
	ProviderType string     `json:"provider_type"`
	ObjectRef    string     `json:"object_ref"`
	Livemode     bool       `json:"livemode"`
	Status       string     `json:"status"`
	Error        string     `json:"error"`
	Attempts     int        `json:"attempts"`
	CreatedAt    time.Time  `json:"created_at"`
	ProcessedAt  *time.Time `json:"processed_at,omitempty"`
}

func webhookEventRow(e *domain.ProviderWebhookEvent) WebhookEventRow {
	return WebhookEventRow{ID: e.ID, Provider: e.Provider, EventID: e.EventID, EventType: e.EventType, ProviderType: e.ProviderType,
		ObjectRef: e.ObjectRef, Livemode: e.Livemode, Status: string(e.Status), Error: e.Error, Attempts: e.Attempts,
		CreatedAt: e.CreatedAt, ProcessedAt: e.ProcessedAt}
}

// WebhookEvents lists the webhook log, newest first.
func (s *BillingService) WebhookEvents(ctx context.Context, provider, status, eventType, cursor string, limit int) (Page[WebhookEventRow], error) {
	if s.ops == nil {
		return Page[WebhookEventRow]{Items: []WebhookEventRow{}}, nil
	}
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[WebhookEventRow]{}, err
	}
	st := domain.WebhookStatus(status)
	if st != "" && !st.Valid() {
		return Page[WebhookEventRow]{}, apperror.Validation(map[string]string{"status": "Unknown status."})
	}
	if provider != "" && !providerName.MatchString(provider) {
		return Page[WebhookEventRow]{}, apperror.Validation(map[string]string{"provider": "Choose a supported payment provider."})
	}
	rows, err := s.ops.repo.ListWebhooks(ctx, repository.WebhookFilter{Provider: provider, Status: st,
		ProviderType: truncateString(strings.TrimSpace(eventType), 100), Cursor: c, Limit: limit})
	if err != nil {
		return Page[WebhookEventRow]{}, apperror.Internal(err)
	}
	page := Page[WebhookEventRow]{Items: make([]WebhookEventRow, 0, len(rows))}
	for i := range rows {
		page.Items = append(page.Items, webhookEventRow(&rows[i]))
	}
	if len(rows) > 0 && len(rows) == clampPage(limit, 20, 100) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return page, nil
}

// ReplayWebhook fetches an event from the provider again and re-applies it
// (handlers are idempotent): the way to finish an event whose delivery failed.
func (s *BillingService) ReplayWebhook(ctx context.Context, id uuid.UUID) (*WebhookEventRow, error) {
	if s.ops == nil {
		return nil, apperror.NotFound("Webhook event")
	}
	rec, err := s.ops.repo.GetWebhookByID(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Webhook event")
	}
	p, ok := s.registry.Get(rec.Provider)
	if !ok {
		return nil, apperror.ProviderUnavailable(nil)
	}
	fetcher, ok := p.(payment.EventFetcher)
	if !ok {
		return nil, apperror.InvalidState("This payment provider cannot replay events.")
	}
	if strings.HasPrefix(rec.EventID, "sha256:") {
		return nil, apperror.InvalidState("This event has no provider id and cannot be fetched again.")
	}
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	evt, err := fetcher.FetchEvent(cctx, rec.EventID)
	cancel()
	if errors.Is(err, payment.ErrNotFound) || (err == nil && evt == nil) {
		return nil, apperror.InvalidState("The provider no longer has this event.")
	}
	if err != nil {
		logger.FromContext(ctx).Warn("billing: fetch webhook event failed", zap.Error(err), zap.String("provider", rec.Provider))
		return nil, apperror.ProviderUnavailable(err)
	}
	applyErr := s.processWebhook(ctx, rec, evt)
	s.record(ctx, "billing.webhook_replayed", "webhook_event", rec.ID.String(), map[string]any{"provider": rec.Provider,
		"event_id": rec.EventID, "event_type": rec.EventType, "ok": applyErr == nil})
	if applyErr != nil {
		return nil, applyErr
	}
	updated, err := s.ops.repo.GetWebhookByID(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Webhook event")
	}
	row := webhookEventRow(updated)
	return &row, nil
}

// --------------------------------------------------------------- disputes

// DisputeView is a dispute in the admin console.
type DisputeView struct {
	ID                  uuid.UUID  `json:"id"`
	Provider            string     `json:"provider"`
	DisputeRef          string     `json:"dispute_ref"`
	TransactionID       uuid.UUID  `json:"transaction_id"`
	UserID              uuid.UUID  `json:"user_id"`
	AmountCents         int64      `json:"amount_cents"`
	Currency            string     `json:"currency"`
	Reason              string     `json:"reason"`
	Status              string     `json:"status"`
	EvidenceDueBy       *time.Time `json:"evidence_due_by,omitempty"`
	EvidenceSubmittedAt *time.Time `json:"evidence_submitted_at,omitempty"`
	CreditsFrozen       int64      `json:"credits_frozen"`
	CreditsRestored     int64      `json:"credits_restored"`
	FundsWithdrawn      bool       `json:"funds_withdrawn"`
	ClosedAt            *time.Time `json:"closed_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func disputeView(d *domain.PaymentDispute) DisputeView {
	return DisputeView{ID: d.ID, Provider: d.Provider, DisputeRef: d.DisputeRef, TransactionID: d.TransactionID, UserID: d.UserID,
		AmountCents: d.AmountCents, Currency: d.Currency, Reason: d.Reason, Status: string(d.Status), EvidenceDueBy: d.EvidenceDueBy,
		EvidenceSubmittedAt: d.EvidenceSubmittedAt, CreditsFrozen: d.CreditsFrozen, CreditsRestored: d.CreditsRestored,
		FundsWithdrawn: d.FundsWithdrawn, ClosedAt: d.ClosedAt, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt}
}

// DisputeUser is the customer of a dispute.
type DisputeUser struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
}

// DisputeDetail is GET /admin/billing/disputes/:id.
type DisputeDetail struct {
	DisputeView
	Transaction   *AdminTransactionRow  `json:"transaction,omitempty"`
	User          *DisputeUser          `json:"user,omitempty"`
	EvidenceDraft DisputeEvidenceFields `json:"evidence_draft"`
}

// Disputes lists disputes, newest first.
func (s *BillingService) Disputes(ctx context.Context, status string, userID *uuid.UUID, cursor string, limit int) (Page[DisputeView], error) {
	if s.ops == nil {
		return Page[DisputeView]{Items: []DisputeView{}}, nil
	}
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[DisputeView]{}, err
	}
	st := domain.DisputeStatus(status)
	if st != "" && !st.Valid() {
		return Page[DisputeView]{}, apperror.Validation(map[string]string{"status": "Unknown status."})
	}
	rows, err := s.ops.repo.ListDisputes(ctx, repository.DisputeFilter{Status: st, UserID: userID, Cursor: c, Limit: limit})
	if err != nil {
		return Page[DisputeView]{}, apperror.Internal(err)
	}
	page := Page[DisputeView]{Items: make([]DisputeView, 0, len(rows))}
	for i := range rows {
		page.Items = append(page.Items, disputeView(&rows[i]))
	}
	if len(rows) > 0 && len(rows) == clampPage(limit, 20, 100) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return page, nil
}

// Dispute returns a dispute with its payment, its customer and the evidence
// draft composed from the data already kept.
func (s *BillingService) Dispute(ctx context.Context, id uuid.UUID) (*DisputeDetail, error) {
	d, t, err := s.disputeAndPayment(ctx, id)
	if err != nil {
		return nil, err
	}
	out := &DisputeDetail{DisputeView: disputeView(d)}
	u, err := s.users.GetByID(ctx, d.UserID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, apperror.Internal(err)
	}
	if u != nil {
		out.User = &DisputeUser{ID: u.ID, DisplayName: u.DisplayName, Email: u.Email}
	}
	rows, err := s.adminRows(ctx, []domain.PaymentTransaction{*t})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if len(rows) == 1 {
		out.Transaction = &rows[0]
	}
	if out.EvidenceDraft, err = s.evidenceDraft(ctx, t, u); err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

func (s *BillingService) disputeAndPayment(ctx context.Context, id uuid.UUID) (*domain.PaymentDispute, *domain.PaymentTransaction, error) {
	if s.ops == nil {
		return nil, nil, apperror.NotFound("Dispute")
	}
	d, err := s.ops.repo.GetDispute(ctx, id)
	if err != nil {
		return nil, nil, mapRepoErr(err, "Dispute")
	}
	t, err := s.repo.GetTransaction(ctx, d.TransactionID)
	if err != nil {
		return nil, nil, mapRepoErr(err, "Payment")
	}
	return d, t, nil
}

// evidenceDraft composes the evidence of a payment.
func (s *BillingService) evidenceDraft(ctx context.Context, t *domain.PaymentTransaction, u *domain.User) (DisputeEvidenceFields, error) {
	in := evidenceInput{Kind: t.Kind, ProductName: t.Description, Credits: t.Credits, AmountCents: t.AmountCents, Currency: t.Currency,
		PurchasedAt: t.CreatedAt, CardCountry: t.CardCountry, ThreeDS: t.ThreeDS,
		RefundPolicyURL: strings.TrimRight(s.ops.cfg.FrontendURL, "/") + "/en/refund-policy",
		HasInvoice:      s.ops.invoices != nil && t.CreditPurchaseID != nil}
	if u != nil {
		in.CustomerName, in.CustomerEmail, in.AccountCreated = u.DisplayName, u.Email, u.CreatedAt
	}
	if t.CreditPurchaseID != nil {
		purchases, err := s.credits.PurchasesByIDs(ctx, []uuid.UUID{*t.CreditPurchaseID})
		if err != nil {
			return DisputeEvidenceFields{}, err
		}
		if len(purchases) == 1 {
			in.Consent = purchaseConsentOf(&purchases[0])
			in.PurchasedAt = purchases[0].CreatedAt
		}
		if r, err := s.credits.PurchaseReceipt(ctx, *t.CreditPurchaseID); err == nil {
			in.ProductName = r.PackName
		} else if !errors.Is(err, repository.ErrNotFound) {
			return DisputeEvidenceFields{}, err
		}
	}
	usage, err := s.ops.repo.UsageLog(ctx, t.UserID, in.PurchasedAt, evidenceUsageRows)
	if err != nil {
		return DisputeEvidenceFields{}, err
	}
	in.Usage = usage
	return composeEvidenceDraft(in), nil
}

// DisputeEvidenceInput is POST /admin/billing/disputes/:id/evidence.
type DisputeEvidenceInput struct {
	Fields DisputeEvidenceFields `json:"fields"`
	Submit bool                  `json:"submit"`
}

// SubmitDisputeEvidence saves the evidence at the processor; submit=true sends
// it to the card network, which is final.
func (s *BillingService) SubmitDisputeEvidence(ctx context.Context, id uuid.UUID, in DisputeEvidenceInput) (*DisputeView, error) {
	fields, err := validateEvidence(in.Fields, in.Submit)
	if err != nil {
		return nil, err
	}
	d, t, err := s.disputeAndPayment(ctx, id)
	if err != nil {
		return nil, err
	}
	if d.EvidenceSubmittedAt != nil {
		return nil, apperror.InvalidState("Evidence was already submitted for this dispute. It cannot be changed.")
	}
	if !d.Status.AcceptsEvidence() {
		return nil, apperror.InvalidState("This dispute no longer accepts evidence.")
	}
	manager, err := s.disputeManager(d.Provider)
	if err != nil {
		return nil, err
	}
	var pdf []byte
	var filename string
	if fields.AttachInvoice && s.ops.invoices != nil && t.CreditPurchaseID != nil {
		if pdf, filename, err = s.ops.invoices.PurchaseInvoicePDF(ctx, *t.CreditPurchaseID); err != nil {
			logger.FromContext(ctx).Warn("billing: invoice not attached to dispute evidence", zap.Error(err), zap.String("dispute_id", id.String()))
			pdf, filename = nil, ""
		}
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	err = manager.SubmitDisputeEvidence(cctx, d.DisputeRef, providerEvidence(fields, pdf, filename), in.Submit)
	cancel()
	if err != nil {
		if errors.Is(err, payment.ErrNotFound) {
			return nil, apperror.InvalidState("The provider does not know this dispute.")
		}
		logger.FromContext(ctx).Warn("billing: dispute evidence failed", zap.Error(err), zap.String("dispute_id", id.String()))
		return nil, apperror.ProviderUnavailable(err)
	}
	now := s.clock.Now()
	action := "dispute.evidence_saved"
	if in.Submit {
		action = "dispute.evidence_submitted"
		if err := s.ops.repo.UpdateDispute(ctx, id, map[string]any{"evidence_submitted_at": now, "updated_at": now}); err != nil {
			return nil, apperror.Internal(err)
		}
	}
	s.record(ctx, action, "payment_dispute", id.String(), map[string]any{"dispute_ref": d.DisputeRef, "attached_invoice": len(pdf) > 0})
	return s.disputeViewOf(ctx, id)
}

// AcceptDispute concedes a dispute: it closes as lost.
func (s *BillingService) AcceptDispute(ctx context.Context, id uuid.UUID, note string) (*DisputeView, error) {
	n, err := cleanNote(note)
	if err != nil {
		return nil, err
	}
	d, t, err := s.disputeAndPayment(ctx, id)
	if err != nil {
		return nil, err
	}
	if d.Status.Closed() {
		return nil, apperror.InvalidState("This dispute is already closed.")
	}
	manager, err := s.disputeManager(d.Provider)
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	err = manager.AcceptDispute(cctx, d.DisputeRef)
	cancel()
	if err != nil && !errors.Is(err, payment.ErrNotFound) {
		logger.FromContext(ctx).Warn("billing: accept dispute failed", zap.Error(err), zap.String("dispute_id", id.String()))
		return nil, apperror.ProviderUnavailable(err)
	}
	if _, err := s.advanceDispute(ctx, t, disputeUpdate{Ref: d.DisputeRef, Status: domain.DisputeLost}); err != nil {
		return nil, err
	}
	s.record(ctx, "dispute.accepted", "payment_dispute", id.String(), map[string]any{"dispute_ref": d.DisputeRef, "note": n})
	return s.disputeViewOf(ctx, id)
}

func (s *BillingService) disputeViewOf(ctx context.Context, id uuid.UUID) (*DisputeView, error) {
	d, err := s.ops.repo.GetDispute(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Dispute")
	}
	v := disputeView(d)
	return &v, nil
}

// ------------------------------------------------------ payments list rows

// TransactionDispute is the dispute summary of an admin payment row.
type TransactionDispute struct {
	ID     uuid.UUID `json:"id"`
	Status string    `json:"status"`
}

// decorateRows adds what revisions 6.0 shows next to a payment: the card
// country, risk, 3-D Secure result, review flag, dispute and the provider's own
// page. The disputes of a page are one query.
func (s *BillingService) decorateRows(ctx context.Context, rows []AdminTransactionRow, txs []domain.PaymentTransaction) error {
	var disputes map[uuid.UUID]domain.PaymentDispute
	if s.ops != nil {
		ids := make([]uuid.UUID, 0, len(txs))
		for i := range txs {
			ids = append(ids, txs[i].ID)
		}
		var err error
		if disputes, err = s.ops.repo.DisputesByTransactions(ctx, ids); err != nil {
			return err
		}
	}
	lives := map[string]bool{}
	for i := range rows {
		t := &txs[i]
		rows[i].CardCountry, rows[i].RiskLevel, rows[i].RiskScore = t.CardCountry, t.RiskLevel, t.RiskScore
		rows[i].ThreeDS, rows[i].InReview = t.ThreeDS, t.InReview
		if d, ok := disputes[t.ID]; ok {
			rows[i].Dispute = &TransactionDispute{ID: d.ID, Status: string(d.Status)}
		}
		if t.ProviderRef != nil && s.ops != nil {
			rows[i].ProviderURL = s.providerURL(ctx, t.Provider, *t.ProviderRef, lives)
		}
	}
	return nil
}

// adminRows builds the admin rows of payments (cards, consent and the
// revisions 6.0 columns): the same decoration for every console endpoint.
func (s *BillingService) adminRows(ctx context.Context, txs []domain.PaymentTransaction) ([]AdminTransactionRow, error) {
	methods, err := s.methodsOf(ctx, txs)
	if err != nil {
		return nil, err
	}
	consents, err := s.consentsOf(ctx, txs)
	if err != nil {
		return nil, err
	}
	docs, err := s.invoiceRefsOf(ctx, txs) // revisions 6.0 (§26.6): invoices and credit notes, one batched query
	if err != nil {
		return nil, err
	}
	rows := make([]AdminTransactionRow, 0, len(txs))
	for i := range txs {
		row := adminTransactionRow(&txs[i], methods)
		if txs[i].CreditPurchaseID != nil {
			row.Consent = consents[*txs[i].CreditPurchaseID]
		}
		row.Invoice, row.CreditNote = docs[txs[i].ID].Invoice, docs[txs[i].ID].CreditNote
		rows = append(rows, row)
	}
	return rows, s.decorateRows(ctx, rows, txs)
}
