package service

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 5.0: refund requests (docs/platform-extensions.md §24.4).
//
// A client asks for money back for one credit-pack payment. The request is a
// small state machine (domain.RefundRequestStatus) whose every move goes
// through RefundService.transition (refunds_admin.go). Money only ever moves
// through BillingService.RefundTransaction, the provider-agnostic path, which
// also claws the refunded credits back from that purchase's own lot.

// Notification kinds of refund requests.
const (
	NotifyRefundReceived = "refund_request.received"
	NotifyRefundResolved = "refund_request.resolved"
)

// RefundSettings configure refund requests.
type RefundSettings struct {
	// WindowDays is the refund window of unused credits (STARYIELD_REFUNDS_WINDOW_DAYS).
	WindowDays int
	// RequestsEnabled is the in-app kill switch (STARYIELD_REFUNDS_REQUESTS_ENABLED).
	RequestsEnabled bool
	// MaxCredit bounds courtesy credits (STARYIELD_BILLING_MAX_ADMIN_ADJUSTMENT).
	MaxCredit    int64
	IPHashSecret string
	// StatementDescriptor is named in the approval e-mail (revisions 5.1, §25.5).
	StatementDescriptor string
}

// RefundDeps are the refund service's collaborators.
type RefundDeps struct {
	Tx      database.TxManager
	Repo    repository.RefundRepository
	Billing *BillingService
	Users   repository.UserRepository
	Audit   repository.AuditRepository
	Events  EventPublisher
	Notify  Notifier
	Clock   clock.Clock
}

// RefundService serves refund requests to clients and administrators.
type RefundService struct {
	d   RefundDeps
	cfg RefundSettings
}

// NewRefundService wires the refund service and plugs its payment info into
// the billing service's transactions list.
func NewRefundService(d RefundDeps, cfg RefundSettings) *RefundService {
	if cfg.WindowDays <= 0 {
		cfg.WindowDays = 14
	}
	if d.Events == nil {
		d.Events = NoopPublisher{}
	}
	if d.Notify == nil {
		d.Notify = NoopNotifier{}
	}
	s := &RefundService{d: d, cfg: cfg}
	if d.Billing != nil {
		d.Billing.WithRefundInfo(s.TransactionInfo)
	}
	return s
}

// ------------------------------------------------------------------ views

// RefundResolutionView is how a request ended.
type RefundResolutionView struct {
	Note           string    `json:"note"`
	RefundedCents  *int64    `json:"refunded_cents,omitempty"`
	CreditsGranted *int64    `json:"credits_granted,omitempty"`
	ResolvedAt     time.Time `json:"resolved_at"`
}

// RefundRequestView is a refund request (the §24.4 shape).
type RefundRequestView struct {
	ID            uuid.UUID             `json:"id"`
	Reference     string                `json:"reference"`
	TransactionID uuid.UUID             `json:"transaction_id"`
	Status        string                `json:"status"`
	Reason        string                `json:"reason"`
	Details       string                `json:"details"`
	Channel       string                `json:"channel"`
	Quote         domain.RefundQuote    `json:"quote"`
	Resolution    *RefundResolutionView `json:"resolution,omitempty"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
}

func refundRequestView(r *domain.RefundRequest) RefundRequestView {
	q := r.Quote
	q.TransactionID = r.TransactionID
	v := RefundRequestView{ID: r.ID, Reference: r.Reference, TransactionID: r.TransactionID, Status: string(r.Status),
		Reason: string(r.Reason), Details: r.Details, Channel: string(r.Channel), Quote: q, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
	if r.Status != domain.RefundOpen && r.ResolvedAt != nil {
		v.Resolution = &RefundResolutionView{Note: r.ResolutionNote, RefundedCents: r.RefundedCents, CreditsGranted: r.CreditsGranted,
			ResolvedAt: *r.ResolvedAt}
	}
	return v
}

// TransactionRefundInfo is the refund state of a credit-purchase row of
// GET /billing/transactions.
type TransactionRefundInfo struct {
	RefundableCents  int64                 `json:"refundable_cents"`
	CreditsRemaining int64                 `json:"credits_remaining"`
	WindowEndsAt     time.Time             `json:"window_ends_at"`
	CanRequest       bool                  `json:"can_request"`
	Request          *TransactionRefundRef `json:"request,omitempty"`
}

// TransactionRefundRef points at the open (else newest) request of a payment.
type TransactionRefundRef struct {
	ID        uuid.UUID `json:"id"`
	Reference string    `json:"reference"`
	Status    string    `json:"status"`
}

// RefundInfoLoader loads the refund info of a page of payments, keyed by
// payment id (rows that are not credit purchases are absent).
type RefundInfoLoader func(ctx context.Context, rows []domain.PaymentTransaction) (map[uuid.UUID]*TransactionRefundInfo, error)

// --------------------------------------------------------- quote and rules

// refundPayable reports whether a payment is in a state that can be refunded
// (paid, and not refunded in full).
func refundPayable(t *domain.PaymentTransaction) bool {
	return (t.Status == domain.TxSucceeded || t.Status == domain.TxPartiallyRefunded) && t.RefundedAmountCents < t.AmountCents
}

// refundEligibility applies the rules of §24.4 to a payment and a reason. It
// returns nil when a request may be made. withWindow=false skips the window
// (an administrator recording a request received by e-mail decides that
// herself).
func refundEligibility(t *domain.PaymentTransaction, reason domain.RefundReason, q domain.RefundQuote, withWindow bool) *apperror.Error {
	switch {
	case t.Kind != domain.TxCreditPurchase || t.CreditPurchaseID == nil:
		return apperror.RefundNotEligible(apperror.RefundReasonNotCreditPurchase)
	case t.Status == domain.TxRefunded || (t.Status == domain.TxPartiallyRefunded && t.RefundedAmountCents >= t.AmountCents):
		return apperror.RefundNotEligible(apperror.RefundReasonFullyRefunded)
	case t.Status != domain.TxSucceeded && t.Status != domain.TxPartiallyRefunded:
		return apperror.RefundNotEligible(apperror.RefundReasonNotPaid)
	case withWindow && reason == domain.RefundUnusedCredits && !q.WithinWindow:
		// Other reasons (a technical or billing problem, an unauthorized
		// charge...) may be raised at any time.
		return apperror.RefundNotEligible(apperror.RefundReasonOutsideWindow)
	}
	return nil
}

// quoteData is what quotes are computed from, loaded for a set of payments
// with one query per table.
type quoteData struct {
	lots  map[uuid.UUID]domain.CreditLot
	packs map[uuid.UUID]repository.PackRef
}

func (s *RefundService) loadQuoteData(ctx context.Context, txs []domain.PaymentTransaction) (quoteData, error) {
	ids := make([]uuid.UUID, 0, len(txs))
	seen := map[uuid.UUID]bool{}
	for _, t := range txs {
		if t.CreditPurchaseID != nil && !seen[*t.CreditPurchaseID] {
			seen[*t.CreditPurchaseID] = true
			ids = append(ids, *t.CreditPurchaseID)
		}
	}
	lots, err := s.d.Billing.wallet.PurchaseLots(ctx, ids)
	if err != nil {
		return quoteData{}, err
	}
	packs, err := s.d.Repo.Packs(ctx, ids)
	if err != nil {
		return quoteData{}, err
	}
	return quoteData{lots: lots, packs: packs}, nil
}

func (s *RefundService) quoteOf(t *domain.PaymentTransaction, d quoteData, now time.Time) domain.RefundQuote {
	in := domain.QuoteInput{TransactionID: t.ID, PackName: t.Description, Currency: t.Currency, AmountPaid: t.AmountCents,
		Refunded: t.RefundedAmountCents, Credits: t.Credits, PurchasedAt: t.CreatedAt, WindowDays: s.cfg.WindowDays, Now: now,
		Refundable: refundPayable(t)}
	if t.CreditPurchaseID != nil {
		if l, ok := d.lots[*t.CreditPurchaseID]; ok {
			in.Lot = &l
		}
		if p, ok := d.packs[*t.CreditPurchaseID]; ok {
			in.PackName = p.Name
		}
	}
	return domain.NewRefundQuote(in)
}

// ownPayment loads a payment of the caller (someone else's is "not found").
func (s *RefundService) ownPayment(ctx context.Context, userID, id uuid.UUID) (*domain.PaymentTransaction, error) {
	t, err := s.d.Billing.repo.GetTransaction(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Payment")
	}
	if t.UserID != userID {
		return nil, apperror.NotFound("Payment")
	}
	return t, nil
}

// Quote tells the caller what one of their payments can be refunded for now.
func (s *RefundService) Quote(ctx context.Context, userID, transactionID uuid.UUID) (*domain.RefundQuote, error) {
	t, err := s.ownPayment(ctx, userID, transactionID)
	if err != nil {
		return nil, err
	}
	switch {
	case t.Kind != domain.TxCreditPurchase || t.CreditPurchaseID == nil:
		return nil, apperror.RefundNotEligible(apperror.RefundReasonNotCreditPurchase)
	case t.Status != domain.TxSucceeded && t.Status != domain.TxPartiallyRefunded && t.Status != domain.TxRefunded:
		return nil, apperror.RefundNotEligible(apperror.RefundReasonNotPaid)
	}
	d, err := s.loadQuoteData(ctx, []domain.PaymentTransaction{*t})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	q := s.quoteOf(t, d, s.d.Clock.Now())
	return &q, nil
}

// TransactionInfo is the RefundInfoLoader of GET /billing/transactions: the
// refund state of a page's credit purchases from ONE query on the lots and ONE
// on the requests, however many rows the page has.
func (s *RefundService) TransactionInfo(ctx context.Context, rows []domain.PaymentTransaction) (map[uuid.UUID]*TransactionRefundInfo, error) {
	var purchases []domain.PaymentTransaction
	ids := make([]uuid.UUID, 0, len(rows))
	purchaseIDs := make([]uuid.UUID, 0, len(rows))
	for _, t := range rows {
		if t.Kind == domain.TxCreditPurchase && t.CreditPurchaseID != nil {
			purchases = append(purchases, t)
			ids = append(ids, t.ID)
			purchaseIDs = append(purchaseIDs, *t.CreditPurchaseID)
		}
	}
	out := make(map[uuid.UUID]*TransactionRefundInfo, len(purchases))
	if len(purchases) == 0 {
		return out, nil
	}
	lots, err := s.d.Billing.wallet.PurchaseLots(ctx, purchaseIDs)
	if err != nil {
		return nil, err
	}
	latest, err := s.d.Repo.Latest(ctx, ids)
	if err != nil {
		return nil, err
	}
	d := quoteData{lots: lots}
	now := s.d.Clock.Now()
	for i := range purchases {
		t := &purchases[i]
		q := s.quoteOf(t, d, now)
		info := &TransactionRefundInfo{RefundableCents: q.RefundableCents, CreditsRemaining: q.CreditsRemaining, WindowEndsAt: q.WindowEndsAt}
		latestReq, has := latest[t.ID]
		if has {
			info.Request = &TransactionRefundRef{ID: latestReq.ID, Reference: latestReq.Reference, Status: string(latestReq.Status)}
		}
		open := has && latestReq.Status == domain.RefundOpen
		info.CanRequest = s.cfg.RequestsEnabled && refundPayable(t) && !open
		out[t.ID] = info
	}
	return out, nil
}

// --------------------------------------------------------------- creation

// CreateRefundInput is a client's request.
type CreateRefundInput struct {
	TransactionID  uuid.UUID `json:"transaction_id"`
	Reason         string    `json:"reason"`
	Details        string    `json:"details"`
	IdempotencyKey string    `json:"idempotency_key"`
}

// referenceAttempts bounds how often a colliding reference is regenerated.
const referenceAttempts = 6

// Create records a client's refund request: 201 for a new one, and the same
// request again (created=false) when the idempotency key was used before.
func (s *RefundService) Create(ctx context.Context, userID uuid.UUID, in CreateRefundInput) (*RefundRequestView, bool, error) {
	if !s.cfg.RequestsEnabled {
		return nil, false, apperror.FeatureUnavailable("Refund requests are not available right now. Please contact support.")
	}
	reason, details, err := validateRefundInput(in.TransactionID, in.Reason, in.Details)
	if err != nil {
		return nil, false, err
	}
	if !ValidIdempotencyKey(in.IdempotencyKey) {
		return nil, false, apperror.Validation(map[string]string{"idempotency_key": "A valid idempotency key is required."})
	}
	// A replay answers with the request already made.
	if prior, err := s.d.Repo.ByKey(ctx, userID, in.IdempotencyKey); err == nil {
		if prior.TransactionID != in.TransactionID {
			return nil, false, apperror.Validation(map[string]string{"idempotency_key": "This key was already used for another payment."})
		}
		v := refundRequestView(prior)
		return &v, false, nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, false, apperror.Internal(err)
	}
	t, err := s.ownPayment(ctx, userID, in.TransactionID)
	if err != nil {
		return nil, false, err
	}
	r, err := s.open(ctx, t, reason, details, domain.RefundChannelWeb, in.IdempotencyKey, true)
	if errors.Is(err, errRefundReplay) {
		prior, err := s.d.Repo.ByKey(ctx, userID, in.IdempotencyKey)
		if err != nil {
			return nil, false, apperror.Internal(err)
		}
		v := refundRequestView(prior)
		return &v, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	v := refundRequestView(r)
	return &v, true, nil
}

func validateRefundInput(transactionID uuid.UUID, reasonRaw, detailsRaw string) (domain.RefundReason, string, error) {
	fields := map[string]string{}
	if transactionID == uuid.Nil {
		fields["transaction_id"] = "Choose a payment."
	}
	reason := domain.RefundReason(reasonRaw)
	if !reason.Valid() {
		fields["reason"] = "Choose a reason."
	}
	details := CleanText(detailsRaw)
	if utf8.RuneCountInString(details) > domain.RefundDetailsMax {
		fields["details"] = "Details can be at most 2000 characters."
	}
	if len(fields) > 0 {
		return "", "", apperror.Validation(fields)
	}
	return reason, details, nil
}

// errRefundReplay says a concurrent request with the same key won the race.
var errRefundReplay = errors.New("service: refund request replay")

// open validates the payment, snapshots its quote and inserts the request.
// Shared by the client (web) and the console (email, admin).
func (s *RefundService) open(ctx context.Context, t *domain.PaymentTransaction, reason domain.RefundReason, details string,
	channel domain.RefundChannel, key string, withWindow bool) (*domain.RefundRequest, error) {
	if t.Kind != domain.TxCreditPurchase || t.CreditPurchaseID == nil {
		return nil, apperror.RefundNotEligible(apperror.RefundReasonNotCreditPurchase)
	}
	if existing, err := s.d.Repo.OpenFor(ctx, t.ID); err == nil {
		return nil, apperror.RefundRequestExists(existing.ID.String())
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, apperror.Internal(err)
	}
	d, err := s.loadQuoteData(ctx, []domain.PaymentTransaction{*t})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	now := s.d.Clock.Now()
	q := s.quoteOf(t, d, now)
	if e := refundEligibility(t, reason, q, withWindow); e != nil {
		return nil, e
	}
	pack, ok := d.packs[*t.CreditPurchaseID]
	if !ok {
		return nil, apperror.Internal(errors.New("service: refund request: purchase without a pack"))
	}
	r := &domain.RefundRequest{ID: domain.NewID(), UserID: t.UserID, TransactionID: t.ID, CreditPurchaseID: *t.CreditPurchaseID,
		PackID: pack.PackID, Status: domain.RefundOpen, Reason: reason, Details: details, Channel: channel, IdempotencyKey: key,
		Quote: q, CreatedAt: now, UpdatedAt: now}
	for attempt := 0; ; attempt++ {
		if r.Reference, err = domain.NewRefundReference(); err != nil {
			return nil, apperror.Internal(err)
		}
		err = s.d.Repo.Create(ctx, r)
		if !errors.Is(err, repository.ErrRefundReferenceTaken) || attempt >= referenceAttempts {
			break
		}
	}
	switch {
	case err == nil:
	case errors.Is(err, repository.ErrRefundOpenExists):
		// A concurrent submission won: tell the caller which request it is.
		existing, lookup := s.d.Repo.OpenFor(ctx, t.ID)
		if lookup != nil {
			return nil, apperror.Internal(lookup)
		}
		return nil, apperror.RefundRequestExists(existing.ID.String())
	case errors.Is(err, repository.ErrRefundKeyTaken):
		return nil, errRefundReplay
	default:
		return nil, apperror.Internal(err)
	}
	s.record(ctx, "refund_request.created", r.ID.String(), map[string]any{"reference": r.Reference, "channel": string(channel),
		"reason": string(reason), "transaction_id": t.ID, "user_id": t.UserID})
	s.d.Notify.Notify(ctx, NotificationSpec{Kind: NotifyRefundReceived, Data: map[string]any{"request_id": r.ID, "reference": r.Reference},
		Link: "/profile/settings"}, r.UserID)
	return r, nil
}

// ListMine pages the caller's own requests, newest first.
func (s *RefundService) ListMine(ctx context.Context, userID uuid.UUID, cursor string, limit int) (Page[RefundRequestView], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[RefundRequestView]{}, err
	}
	rows, err := s.d.Repo.ListForUser(ctx, userID, c, limit)
	if err != nil {
		return Page[RefundRequestView]{}, apperror.Internal(err)
	}
	page := Page[RefundRequestView]{Items: make([]RefundRequestView, 0, len(rows))}
	for i := range rows {
		page.Items = append(page.Items, refundRequestView(&rows[i]))
	}
	if len(rows) > 0 && len(rows) == clampPage(limit, 20, 100) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return page, nil
}

func (s *RefundService) record(ctx context.Context, action, id string, meta map[string]any) {
	if s.d.Audit == nil {
		return
	}
	recordAudit(ctx, s.d.Audit, s.d.Clock, s.cfg.IPHashSecret, action, "refund_request", id, meta)
}

func (s *RefundService) logger(ctx context.Context) *zap.Logger { return logger.FromContext(ctx) }
