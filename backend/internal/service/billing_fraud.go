package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 6.0 (§26.5): early fraud warnings. An issuer tells the processor a
// card payment looks fraudulent: refunding it before the cardholder disputes it
// avoids the chargeback and its fee. With the efw_auto_refund policy an
// actionable warning on an undisputed payment is refunded at once (reason
// "fraudulent", the system as actor, lot clawback and credit note like every
// refund); otherwise the team decides (refund or dismiss).

// fraudRefundReason is the reason a fraud-warning refund carries to the processor.
const fraudRefundReason = "fraudulent"

// webhookFraudWarning records a warning and, when the policy says so, refunds
// the payment.
func (s *BillingService) webhookFraudWarning(ctx context.Context, t *domain.PaymentTransaction, evt *payment.WebhookEvent) (webhookResult, error) {
	if s.ops == nil {
		return ignoredEvent("fraud warnings are not enabled"), nil
	}
	ref := truncateString(firstNonBlank(evt.ObjectRef, "payment:"+evt.ProviderRef), 200)
	now := s.clock.Now()
	w, err := s.ops.repo.GetFraudWarningByRef(ctx, t.Provider, ref)
	created := false
	switch {
	case errors.Is(err, repository.ErrNotFound):
		w = &domain.PaymentFraudWarning{ID: domain.NewID(), Provider: t.Provider, WarningRef: ref, TransactionID: t.ID, UserID: t.UserID,
			FraudType: truncateString(evt.FraudType, 100), Actionable: evt.Actionable, Action: domain.FraudActionNone, CreatedAt: now, UpdatedAt: now}
		if err = s.ops.repo.CreateFraudWarning(ctx, w); errors.Is(err, repository.ErrDuplicate) { // a parallel delivery created it
			w, err = s.ops.repo.GetFraudWarningByRef(ctx, t.Provider, ref)
		} else if err == nil {
			created = true
		}
		if err != nil {
			return webhookResult{}, apperror.Internal(err)
		}
	case err != nil:
		return webhookResult{}, apperror.Internal(err)
	case w.Action == domain.FraudActionNone && (w.Actionable != evt.Actionable || (evt.FraudType != "" && w.FraudType != evt.FraudType)):
		fields := map[string]any{"actionable": evt.Actionable, "updated_at": now}
		if evt.FraudType != "" {
			fields["fraud_type"] = truncateString(evt.FraudType, 100)
		}
		if err := s.ops.repo.UpdateFraudWarning(ctx, w.ID, fields); err != nil {
			return webhookResult{}, apperror.Internal(err)
		}
		w.Actionable = evt.Actionable
	}
	if created {
		s.record(ctx, "fraud_warning.received", "payment_fraud_warning", w.ID.String(), map[string]any{"payment_id": t.ID,
			"fraud_type": w.FraudType, "actionable": w.Actionable, "user_id": t.UserID})
	}
	if w.Action != domain.FraudActionNone || !w.Actionable {
		return appliedEvent(), nil
	}
	if s.policies(ctx).EFWAutoRefund {
		refunded, err := s.autoRefundFraudulent(ctx, t, w)
		if err != nil {
			return webhookResult{}, err // a transient failure: the provider retries
		}
		if refunded {
			return appliedEvent(), nil
		}
	}
	if created {
		s.alert(ctx, "fraud_warning:"+w.ID.String(), "Early fraud warning on a payment", []string{"Payment: " + t.ID.String(),
			"Customer: " + t.UserID.String(), "Amount: " + money(t.AmountCents, t.Currency), "Type: " + w.FraudType,
			"Refund the payment before it is disputed, or dismiss the warning."}, "/payments/fraud-warnings")
	}
	return appliedEvent(), nil
}

// autoRefundFraudulent refunds the refundable amount of an undisputed payment.
// It returns whether the payment was refunded; a refund the processor refuses
// for good (the payment is disputed, was refunded elsewhere) leaves the warning
// to the team, anything else is returned for a retry.
func (s *BillingService) autoRefundFraudulent(ctx context.Context, t *domain.PaymentTransaction, w *domain.PaymentFraudWarning) (bool, error) {
	fresh, err := s.repo.GetTransaction(ctx, t.ID)
	if err != nil {
		return false, mapRepoErr(err, "Payment")
	}
	disputes, err := s.ops.repo.DisputesByTransactions(ctx, []uuid.UUID{t.ID})
	if err != nil {
		return false, apperror.Internal(err)
	}
	if _, disputed := disputes[t.ID]; disputed || fresh.Status == domain.TxDisputed ||
		(fresh.Status != domain.TxSucceeded && fresh.Status != domain.TxPartiallyRefunded) || fresh.Refundable() <= 0 {
		return false, nil
	}
	amount := fresh.Refundable()
	if _, _, err := s.refundTransaction(ctx, t.ID, RefundInput{AmountCents: amount, Reason: fraudRefundReason}); err != nil {
		if e, ok := apperror.As(err); ok && e.Status < 500 {
			logger.FromContext(ctx).Warn("billing: automatic fraud-warning refund refused", zap.Error(err), zap.String("payment_id", t.ID.String()))
			s.alert(ctx, "fraud_refund_refused:"+w.ID.String(), "Automatic refund of a fraud warning was refused", []string{
				"Payment: " + t.ID.String(), "Amount: " + money(amount, t.Currency), "Reason: " + e.Message,
				"Decide from the console: refund by hand or dismiss the warning."}, "/payments/fraud-warnings")
			return false, nil
		}
		return false, err
	}
	now := s.clock.Now()
	if err := s.ops.repo.UpdateFraudWarning(ctx, w.ID, map[string]any{"action": domain.FraudActionRefunded, "actioned_at": now,
		"note": "Refunded automatically.", "updated_at": now}); err != nil {
		return false, apperror.Internal(err)
	}
	s.record(ctx, "fraud_warning.refunded", "payment_fraud_warning", w.ID.String(), map[string]any{"payment_id": t.ID, "automatic": true,
		"amount_cents": amount, "currency": t.Currency})
	return true, nil
}

// FraudWarningView is an early fraud warning in the admin console.
type FraudWarningView struct {
	ID            uuid.UUID  `json:"id"`
	Provider      string     `json:"provider"`
	WarningRef    string     `json:"warning_ref"`
	TransactionID uuid.UUID  `json:"transaction_id"`
	UserID        uuid.UUID  `json:"user_id"`
	FraudType     string     `json:"fraud_type"`
	Actionable    bool       `json:"actionable"`
	Action        string     `json:"action"`
	ActionedAt    *time.Time `json:"actioned_at,omitempty"`
	Note          string     `json:"note,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

func fraudWarningView(w *domain.PaymentFraudWarning) FraudWarningView {
	return FraudWarningView{ID: w.ID, Provider: w.Provider, WarningRef: w.WarningRef, TransactionID: w.TransactionID, UserID: w.UserID,
		FraudType: w.FraudType, Actionable: w.Actionable, Action: string(w.Action), ActionedAt: w.ActionedAt, Note: w.Note, CreatedAt: w.CreatedAt}
}

// FraudWarnings lists early fraud warnings, newest first.
func (s *BillingService) FraudWarnings(ctx context.Context, action, cursor string, limit int) (Page[FraudWarningView], error) {
	if s.ops == nil {
		return Page[FraudWarningView]{Items: []FraudWarningView{}}, nil
	}
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[FraudWarningView]{}, err
	}
	act := domain.FraudAction(action)
	switch act {
	case "", domain.FraudActionNone, domain.FraudActionRefunded, domain.FraudActionDismissed:
	default:
		return Page[FraudWarningView]{}, apperror.Validation(map[string]string{"status": "Unknown status."})
	}
	rows, err := s.ops.repo.ListFraudWarnings(ctx, repository.FraudWarningFilter{Action: act, Cursor: c, Limit: limit})
	if err != nil {
		return Page[FraudWarningView]{}, apperror.Internal(err)
	}
	page := Page[FraudWarningView]{Items: make([]FraudWarningView, 0, len(rows))}
	for i := range rows {
		page.Items = append(page.Items, fraudWarningView(&rows[i]))
	}
	if len(rows) > 0 && len(rows) == clampPage(limit, 20, 100) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return page, nil
}

// cleanNote validates the note of an administrator's decision.
func cleanNote(note string) (string, error) {
	n := CleanLine(note)
	if n == "" || len([]rune(n)) > 500 {
		return "", apperror.Validation(map[string]string{"note": "A note (up to 500 characters) is required."})
	}
	return n, nil
}

// RefundFraudWarning refunds the refundable amount of the warned payment on an
// administrator's decision (reason "fraudulent").
func (s *BillingService) RefundFraudWarning(ctx context.Context, id uuid.UUID, note string) (*FraudWarningView, error) {
	n, err := cleanNote(note)
	if err != nil {
		return nil, err
	}
	w, err := s.openFraudWarning(ctx, id)
	if err != nil {
		return nil, err
	}
	t, err := s.repo.GetTransaction(ctx, w.TransactionID)
	if err != nil {
		return nil, mapRepoErr(err, "Payment")
	}
	if t.Refundable() <= 0 {
		return nil, apperror.InvalidState("This payment has nothing left to refund.")
	}
	amount := t.Refundable()
	if _, _, err := s.refundTransaction(ctx, t.ID, RefundInput{AmountCents: amount, Reason: fraudRefundReason}); err != nil {
		return nil, err
	}
	out, err := s.closeFraudWarning(ctx, id, domain.FraudActionRefunded, n)
	if err != nil {
		return nil, err
	}
	s.record(ctx, "fraud_warning.refunded", "payment_fraud_warning", id.String(), map[string]any{"payment_id": t.ID, "automatic": false,
		"amount_cents": amount, "currency": t.Currency, "note": n})
	return out, nil
}

// DismissFraudWarning marks a warning as looked at: the payment stays.
func (s *BillingService) DismissFraudWarning(ctx context.Context, id uuid.UUID, note string) (*FraudWarningView, error) {
	n, err := cleanNote(note)
	if err != nil {
		return nil, err
	}
	if _, err := s.openFraudWarning(ctx, id); err != nil {
		return nil, err
	}
	out, err := s.closeFraudWarning(ctx, id, domain.FraudActionDismissed, n)
	if err != nil {
		return nil, err
	}
	s.record(ctx, "fraud_warning.dismissed", "payment_fraud_warning", id.String(), map[string]any{"note": n})
	return out, nil
}

// openFraudWarning loads a warning nobody has acted on yet.
func (s *BillingService) openFraudWarning(ctx context.Context, id uuid.UUID) (*domain.PaymentFraudWarning, error) {
	if s.ops == nil {
		return nil, apperror.NotFound("Fraud warning")
	}
	w, err := s.ops.repo.GetFraudWarning(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Fraud warning")
	}
	if w.Action != domain.FraudActionNone {
		return nil, apperror.InvalidState("This warning has already been handled.")
	}
	return w, nil
}

// closeFraudWarning records the decision on a warning (once: a concurrent
// decision wins).
func (s *BillingService) closeFraudWarning(ctx context.Context, id uuid.UUID, action domain.FraudAction, note string) (*FraudWarningView, error) {
	now := s.clock.Now()
	var out *domain.PaymentFraudWarning
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		w, err := s.ops.repo.GetFraudWarningForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if w.Action != domain.FraudActionNone {
			out = w
			return nil
		}
		if err := s.ops.repo.UpdateFraudWarning(ctx, id, map[string]any{"action": action, "actioned_at": now, "note": note, "updated_at": now}); err != nil {
			return err
		}
		w.Action, w.ActionedAt, w.Note = action, &now, note
		out = w
		return nil
	})
	if err != nil {
		return nil, mapRepoErr(err, "Fraud warning")
	}
	v := fraudWarningView(out)
	return &v, nil
}
