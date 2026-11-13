package service

import (
	"context"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 5.0: refund requests, state transitions and the admin console (§24.4).

// ------------------------------------------------------- the state machine

// refundOutcome is what a transition decided: it becomes the resolution
// columns of the row.
type refundOutcome struct {
	note           string
	refundedCents  *int64
	creditsGranted *int64
	refundTxID     *uuid.UUID
	resolvedBy     uuid.UUID
	// irreversible is set once money or credits moved: if writing the row
	// then fails, the write is retried (the side effect is never repeated).
	irreversible bool
	// wallet is the wallet a credit grant changed (published after commit).
	wallet *domain.Wallet
}

// refundMove is one requested transition. apply runs inside the transaction,
// after the row is locked and the move validated, and performs the side
// effect (refund through the provider, courtesy credits, nothing).
type refundMove struct {
	to domain.RefundRequestStatus
	// guard runs first, on the locked row, before the move is validated (who
	// may touch this request); nil allows everyone the route admitted.
	guard func(r *domain.RefundRequest) error
	apply func(ctx context.Context, r *domain.RefundRequest) (refundOutcome, error)
	// external: apply talks to a payment provider. The transaction then runs
	// on a context the HTTP deadline cannot cancel half-way (bounded by
	// externalTimeout), so a refund the provider executed is never left
	// without its row.
	external bool
}

// externalTimeout bounds a transition that calls the payment provider (the
// provider call itself has its own 30 s limit).
const externalTimeout = 45 * time.Second

// transition is the ONLY code that changes the status of a refund request.
// It locks the row (SELECT … FOR UPDATE) inside a transaction, checks the
// move against the transition table (domain.RefundRequestStatus.CanTransition),
// runs the side effect and writes the new status and the resolution in the
// same transaction. Two administrators acting on the same request serialise
// on the row lock: the second one finds it resolved (409 invalid_state).
func (s *RefundService) transition(ctx context.Context, id uuid.UUID, mv refundMove) (*domain.RefundRequest, refundOutcome, error) {
	if mv.external {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.WithoutCancel(ctx), externalTimeout)
		defer cancel()
	}
	var out refundOutcome
	var done *domain.RefundRequest
	run := func(resume bool, apply func(context.Context, *domain.RefundRequest) (refundOutcome, error)) error {
		return s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
			r, err := s.d.Repo.GetForUpdate(ctx, id)
			if err != nil {
				return err
			}
			if mv.guard != nil {
				if err := mv.guard(r); err != nil {
					return err
				}
			}
			if resume && r.Status == mv.to {
				// The first attempt did commit after all (only its answer was lost).
				done = r
				return nil
			}
			if !r.Status.CanTransition(mv.to) {
				return apperror.InvalidState("This refund request has already been resolved.")
			}
			res, err := apply(ctx, r)
			if err != nil {
				return err
			}
			out = res
			now := s.d.Clock.Now()
			fields := map[string]any{"status": mv.to, "resolution_note": res.note, "refunded_cents": res.refundedCents,
				"credits_granted": res.creditsGranted, "refund_transaction_id": res.refundTxID, "resolved_by": res.resolvedBy,
				"resolved_at": now, "updated_at": now}
			if err := s.d.Repo.Update(ctx, id, fields); err != nil {
				return err
			}
			r.Status, r.ResolutionNote, r.RefundedCents, r.CreditsGranted = mv.to, res.note, res.refundedCents, res.creditsGranted
			r.RefundTransactionID, r.ResolvedBy, r.ResolvedAt, r.UpdatedAt = res.refundTxID, &res.resolvedBy, &now, now
			done = r
			return nil
		})
	}
	err := run(false, mv.apply)
	if err != nil && out.irreversible {
		// The provider refunded and the ledger booked it, but writing the
		// request failed (lost connection, say). Retry the write alone, in a
		// fresh transaction, without refunding again.
		s.logger(ctx).Error("refund request: refund booked but the request was not updated; retrying the write",
			zap.String("request_id", id.String()), zap.Error(err))
		saved := out
		done = nil
		err = run(true, func(context.Context, *domain.RefundRequest) (refundOutcome, error) { return saved, nil })
		if err != nil {
			// The row is still open although the money moved: needs a human.
			s.logger(ctx).Error("refund request: MANUAL RECONCILIATION NEEDED: refund booked, request still open",
				zap.String("request_id", id.String()), zap.String("refund_transaction_id", fmt.Sprint(saved.refundTxID)), zap.Error(err))
		}
	}
	if err != nil {
		return nil, refundOutcome{}, mapRepoErr(err, "Refund request")
	}
	return done, out, nil
}

// ------------------------------------------------------------ admin views

// RefundUserRef is the person behind a request.
type RefundUserRef struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
}

// AdminRefundRequestView is a request in the console: the live quote is what
// the payment can be refunded for right now (a refund issued from the provider
// meanwhile, a webhook-driven charge.refunded included, shows in it while the
// request stays open).
type AdminRefundRequestView struct {
	RefundRequestView
	User      RefundUserRef      `json:"user"`
	LiveQuote domain.RefundQuote `json:"live_quote"`
}

// RefundLotView is one lot of the user (the console shows where credits came from).
type RefundLotView struct {
	Source           string    `json:"source"`
	CreditsTotal     int64     `json:"credits_total"`
	CreditsRemaining int64     `json:"credits_remaining"`
	PaidCents        int64     `json:"paid_cents"`
	CreatedAt        time.Time `json:"created_at"`
}

// AdminRefundRequestDetail adds the user's lots.
type AdminRefundRequestDetail struct {
	AdminRefundRequestView
	Lots []RefundLotView `json:"lots"`
}

// adminViews enriches requests for the console with one query each for the
// users, the payments, the lots and the packs (no per-row queries).
func (s *RefundService) adminViews(ctx context.Context, rows []domain.RefundRequest) ([]AdminRefundRequestView, error) {
	userIDs := make([]uuid.UUID, 0, len(rows))
	txIDs := make([]uuid.UUID, 0, len(rows))
	seenUser, seenTx := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
	for _, r := range rows {
		if !seenUser[r.UserID] {
			seenUser[r.UserID] = true
			userIDs = append(userIDs, r.UserID)
		}
		if !seenTx[r.TransactionID] {
			seenTx[r.TransactionID] = true
			txIDs = append(txIDs, r.TransactionID)
		}
	}
	users, err := s.d.Users.GetByIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	byUser := make(map[uuid.UUID]domain.User, len(users))
	for _, u := range users {
		byUser[u.ID] = u
	}
	txs, err := s.d.Billing.repo.TransactionsByIDs(ctx, txIDs)
	if err != nil {
		return nil, err
	}
	data, err := s.loadQuoteData(ctx, txs)
	if err != nil {
		return nil, err
	}
	byTx := make(map[uuid.UUID]*domain.PaymentTransaction, len(txs))
	for i := range txs {
		byTx[txs[i].ID] = &txs[i]
	}
	now := s.d.Clock.Now()
	out := make([]AdminRefundRequestView, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		v := AdminRefundRequestView{RefundRequestView: refundRequestView(r), LiveQuote: r.Quote}
		v.LiveQuote.TransactionID = r.TransactionID
		if t, ok := byTx[r.TransactionID]; ok {
			v.LiveQuote = s.quoteOf(t, data, now)
		}
		u := byUser[r.UserID]
		v.User = RefundUserRef{ID: r.UserID, DisplayName: u.DisplayName, Email: u.Email}
		out = append(out, v)
	}
	return out, nil
}

// AdminList pages requests (status and user filters), newest first.
func (s *RefundService) AdminList(ctx context.Context, status string, userID *uuid.UUID, cursor string, limit int) (Page[AdminRefundRequestView], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[AdminRefundRequestView]{}, err
	}
	st := domain.RefundRequestStatus(status)
	if st != "" && !st.Valid() {
		return Page[AdminRefundRequestView]{}, apperror.Validation(map[string]string{"status": "Unknown status."})
	}
	rows, err := s.d.Repo.List(ctx, repository.RefundFilter{Status: st, UserID: userID, Cursor: c, Limit: limit})
	if err != nil {
		return Page[AdminRefundRequestView]{}, apperror.Internal(err)
	}
	items, err := s.adminViews(ctx, rows)
	if err != nil {
		return Page[AdminRefundRequestView]{}, apperror.Internal(err)
	}
	page := Page[AdminRefundRequestView]{Items: items}
	if page.Items == nil {
		page.Items = []AdminRefundRequestView{}
	}
	if len(rows) > 0 && len(rows) == clampPage(limit, 20, 100) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return page, nil
}

// AdminGet returns one request with the user's lots.
func (s *RefundService) AdminGet(ctx context.Context, id uuid.UUID) (*AdminRefundRequestDetail, error) {
	r, err := s.d.Repo.Get(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Refund request")
	}
	views, err := s.adminViews(ctx, []domain.RefundRequest{*r})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	lots, err := s.d.Billing.wallet.LotsForUser(ctx, r.UserID)
	if err != nil {
		return nil, err
	}
	out := &AdminRefundRequestDetail{AdminRefundRequestView: views[0], Lots: make([]RefundLotView, 0, len(lots.Lots))}
	for _, l := range lots.Lots {
		out.Lots = append(out.Lots, RefundLotView{Source: l.Source, CreditsTotal: l.CreditsTotal, CreditsRemaining: l.CreditsRemaining,
			PaidCents: l.PaidCents, CreatedAt: l.CreatedAt})
	}
	return out, nil
}

// ------------------------------------------------------------ admin actions

// AdminCreateRefundInput records a request received outside the app.
type AdminCreateRefundInput struct {
	TransactionID uuid.UUID `json:"transaction_id"`
	Reason        string    `json:"reason"`
	Details       string    `json:"details"`
	Channel       string    `json:"channel"`
}

// AdminCreate records a request that reached support by e-mail (or that an
// administrator opens on a client's behalf). The kill switch does not apply
// and neither does the unused-credits window: the administrator decides.
func (s *RefundService) AdminCreate(ctx context.Context, in AdminCreateRefundInput) (*AdminRefundRequestView, error) {
	reason, details, err := validateRefundInput(in.TransactionID, in.Reason, in.Details)
	if err != nil {
		return nil, err
	}
	channel := domain.RefundChannel(in.Channel)
	if channel != domain.RefundChannelEmail && channel != domain.RefundChannelAdmin {
		return nil, apperror.Validation(map[string]string{"channel": "Choose how the request was received."})
	}
	t, err := s.d.Billing.repo.GetTransaction(ctx, in.TransactionID)
	if err != nil {
		return nil, mapRepoErr(err, "Payment")
	}
	r, err := s.open(ctx, t, reason, details, channel, "admin:"+uuid.NewString(), false)
	if err != nil {
		return nil, err
	}
	views, err := s.adminViews(ctx, []domain.RefundRequest{*r})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &views[0], nil
}

// ApproveRefundInput approves a request with a refund.
type ApproveRefundInput struct {
	AmountCents int64  `json:"amount_cents"`
	Note        string `json:"note"`
}

func cleanRefundNote(raw string) (string, error) {
	note := CleanText(raw)
	if n := utf8.RuneCountInString(note); n < 5 || n > domain.RefundNoteMax {
		return "", apperror.Validation(map[string]string{"note": "Write a note of 5 to 1000 characters."})
	}
	return note, nil
}

// Approve refunds the payment through the provider (BillingService.RefundTransaction:
// the provider-agnostic path, with the lot clawback) and marks the request
// approved. On any failure the request stays open and the error is returned:
// 409 invalid_state (not open), 422 amount_cents (too large), 409
// refund_refused (the provider definitively refused), 503 service_unavailable
// (outcome unknown: repeat with the same amount to complete it).
func (s *RefundService) Approve(ctx context.Context, adminID, id uuid.UUID, in ApproveRefundInput) (*AdminRefundRequestView, error) {
	note, err := cleanRefundNote(in.Note)
	if err != nil {
		return nil, err
	}
	if in.AmountCents <= 0 {
		return nil, apperror.Validation(map[string]string{"amount_cents": "Enter an amount to refund."})
	}
	mv := refundMove{to: domain.RefundApproved, external: true, apply: func(ctx context.Context, r *domain.RefundRequest) (refundOutcome, error) {
		// The provider call and the booking run outside this transaction
		// (their own, committed units); this one only holds the request's row lock.
		_, refundID, err := s.d.Billing.refundTransaction(withQuietRefund(database.Detach(ctx)), r.TransactionID, // revisions 5.1: the approval sends the notice
			RefundInput{AmountCents: in.AmountCents, Reason: truncateRunes(note, 300)})
		if err != nil {
			return refundOutcome{}, err
		}
		amount := in.AmountCents
		return refundOutcome{note: note, refundedCents: &amount, refundTxID: &refundID, resolvedBy: adminID, irreversible: true}, nil
	}}
	r, out, err := s.transition(ctx, id, mv)
	if err != nil {
		return nil, err
	}
	ctx = context.WithoutCancel(ctx) // the refund happened: its audit record and notices must not be lost to a request deadline
	s.record(ctx, "refund_request.approved", id.String(), map[string]any{"reference": r.Reference, "amount_cents": *out.refundedCents,
		"currency": r.Quote.Currency, "transaction_id": r.TransactionID, "refund_id": out.refundTxID, "user_id": r.UserID})
	s.resolved(ctx, r, out)
	return s.adminView(ctx, r)
}

// DenyRefundInput denies a request.
type DenyRefundInput struct {
	Note string `json:"note"`
}

// Deny closes a request without paying anything.
func (s *RefundService) Deny(ctx context.Context, adminID, id uuid.UUID, in DenyRefundInput) (*AdminRefundRequestView, error) {
	note, err := cleanRefundNote(in.Note)
	if err != nil {
		return nil, err
	}
	r, out, err := s.transition(ctx, id, refundMove{to: domain.RefundDenied, apply: func(context.Context, *domain.RefundRequest) (refundOutcome, error) {
		return refundOutcome{note: note, resolvedBy: adminID}, nil
	}})
	if err != nil {
		return nil, err
	}
	ctx = context.WithoutCancel(ctx)
	s.record(ctx, "refund_request.denied", id.String(), map[string]any{"reference": r.Reference, "transaction_id": r.TransactionID, "user_id": r.UserID})
	s.resolved(ctx, r, out)
	return s.adminView(ctx, r)
}

// CreditRefundInput answers a request with courtesy credits.
type CreditRefundInput struct {
	Credits int64  `json:"credits"`
	Note    string `json:"note"`
}

// Credit gives courtesy credits (a bonus lot, through the same movement as an
// administrator's credit adjustment) instead of a refund. The grant and the
// status change commit together; the movement's idempotency key is the
// request's, so it can never be granted twice.
func (s *RefundService) Credit(ctx context.Context, adminID, id uuid.UUID, in CreditRefundInput) (*AdminRefundRequestView, error) {
	note, err := cleanRefundNote(in.Note)
	if err != nil {
		return nil, err
	}
	if in.Credits < 1 || (s.cfg.MaxCredit > 0 && in.Credits > s.cfg.MaxCredit) {
		return nil, apperror.Validation(map[string]string{"credits": fmt.Sprintf("Use a whole number of credits between 1 and %d.", s.cfg.MaxCredit)})
	}
	mv := refundMove{to: domain.RefundCredited, apply: func(ctx context.Context, r *domain.RefundRequest) (refundOutcome, error) {
		w, _, err := s.d.Billing.wallet.Apply(ctx, adminAdjustmentMovement(adminID, r.UserID, in.Credits,
			"Courtesy credits for refund request "+r.Reference, "refund_request:"+r.ID.String()))
		if err != nil {
			return refundOutcome{}, err
		}
		credits := in.Credits
		return refundOutcome{note: note, creditsGranted: &credits, resolvedBy: adminID, wallet: w}, nil
	}}
	r, out, err := s.transition(ctx, id, mv)
	if err != nil {
		return nil, err
	}
	ctx = context.WithoutCancel(ctx)
	if out.wallet != nil {
		dctx := database.Detach(ctx)
		warnErr(dctx, "refunds: publish wallet", s.d.Events.Publish(dctx, realtime.EventWalletUpdated, walletView(out.wallet), r.UserID))
	}
	s.record(ctx, "refund_request.credited", id.String(), map[string]any{"reference": r.Reference, "credits": in.Credits,
		"transaction_id": r.TransactionID, "user_id": r.UserID})
	s.resolved(ctx, r, out)
	return s.adminView(ctx, r)
}

// Withdraw lets the client take back an open request of theirs.
func (s *RefundService) Withdraw(ctx context.Context, userID, id uuid.UUID) (*RefundRequestView, error) {
	r, _, err := s.transition(ctx, id, refundMove{to: domain.RefundWithdrawn,
		guard: func(r *domain.RefundRequest) error {
			if r.UserID != userID { // someone else's request is "not found", never revealed
				return repository.ErrNotFound
			}
			return nil
		},
		apply: func(context.Context, *domain.RefundRequest) (refundOutcome, error) {
			return refundOutcome{resolvedBy: userID}, nil
		}})
	if err != nil {
		return nil, err
	}
	s.record(ctx, "refund_request.withdrawn", id.String(), map[string]any{"reference": r.Reference, "transaction_id": r.TransactionID})
	v := refundRequestView(r)
	return &v, nil
}

func (s *RefundService) adminView(ctx context.Context, r *domain.RefundRequest) (*AdminRefundRequestView, error) {
	views, err := s.adminViews(ctx, []domain.RefundRequest{*r})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &views[0], nil
}

// resolved tells the client how their request ended (in-app, and e-mail and
// push through the notification channels).
func (s *RefundService) resolved(ctx context.Context, r *domain.RefundRequest, out refundOutcome) {
	data := map[string]any{"request_id": r.ID, "reference": r.Reference, "status": string(r.Status), "currency": r.Quote.Currency}
	if out.refundedCents != nil {
		data["refunded_cents"] = *out.refundedCents
		// revisions 5.1: the approval e-mail names the original purchase and the descriptor (§25.5).
		data["pack_name"], data["credits"], data["purchased_at"] = r.Quote.PackName, r.Quote.CreditsPurchased, r.Quote.PurchasedAt
		if s.cfg.StatementDescriptor != "" {
			data["statement_descriptor"] = s.cfg.StatementDescriptor
		}
	}
	if out.creditsGranted != nil {
		data["credits_granted"] = *out.creditsGranted
	}
	s.d.Notify.Notify(ctx, NotificationSpec{Kind: NotifyRefundResolved, Data: data, Link: "/profile/settings"}, r.UserID)
}
