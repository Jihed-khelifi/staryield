package service

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// PayoutSettings configure advisor withdrawals.
type PayoutSettings struct {
	CentsPerCredit int64
	MinCredits     int64
	Currency       string
	IPHashSecret   string
}

// WithdrawalView is a payout request as seen by its advisor.
type WithdrawalView struct {
	ID              uuid.UUID  `json:"id"`
	AmountCredits   int64      `json:"amount_credits"`
	AmountCents     int64      `json:"amount_cents"`
	Currency        string     `json:"currency"`
	Method          string     `json:"method"`
	DestinationHint string     `json:"destination_hint"`
	Status          string     `json:"status"`
	Note            string     `json:"note"`
	RequestedAt     time.Time  `json:"requested_at"`
	ProcessedAt     *time.Time `json:"processed_at,omitempty"`
}

// AdminWithdrawalView adds processing context for administrators.
type AdminWithdrawalView struct {
	WithdrawalView
	Psychic     ParticipantRef `json:"psychic"`
	ProcessedBy *uuid.UUID     `json:"processed_by,omitempty"`
	Reference   string         `json:"reference,omitempty"`
}

// PayoutView is an advisor's withdrawable balance.
type PayoutView struct {
	AvailableCredits   int64  `json:"available_credits"`
	ReservedCredits    int64  `json:"reserved_credits"`
	PaidCredits        int64  `json:"paid_credits"`
	LifetimeNetCredits int64  `json:"lifetime_net_credits"`
	CentsPerCredit     int64  `json:"cents_per_credit"`
	Currency           string `json:"currency"`
	MinWithdrawal      int64  `json:"min_withdrawal_credits"`
}

// WithdrawalInput requests a payout.
type WithdrawalInput struct {
	AmountCredits  int64  `json:"amount_credits"`
	Method         string `json:"method"`
	Destination    string `json:"destination"`
	IdempotencyKey string `json:"idempotency_key"`
}

// WithdrawalService handles advisor payouts of net earnings.
type WithdrawalService struct {
	tx       database.TxManager
	repo     repository.WithdrawalRepository
	psychics repository.PsychicRepository
	audit    repository.AuditRepository
	sealer   *security.Sealer
	notify   Notifier
	tracker  analytics.Tracker
	clock    clock.Clock
	cfg      PayoutSettings
}

// NewWithdrawalService wires the withdrawal service.
func NewWithdrawalService(tx database.TxManager, repo repository.WithdrawalRepository, psychics repository.PsychicRepository,
	audit repository.AuditRepository, sealer *security.Sealer, notify Notifier, tracker analytics.Tracker, clk clock.Clock, cfg PayoutSettings) *WithdrawalService {
	if notify == nil {
		notify = NoopNotifier{}
	}
	return &WithdrawalService{tx: tx, repo: repo, psychics: psychics, audit: audit, sealer: sealer, notify: notify, tracker: tracker, clock: clk, cfg: cfg}
}

func withdrawalView(w *domain.Withdrawal) WithdrawalView {
	return WithdrawalView{ID: w.ID, AmountCredits: w.AmountCredits, AmountCents: w.AmountCents, Currency: w.Currency, Method: w.Method,
		DestinationHint: w.DestinationHint, Status: string(w.Status), Note: w.Note, RequestedAt: w.RequestedAt, ProcessedAt: w.ProcessedAt}
}

// Balance returns the advisor's payout balance.
func (s *WithdrawalService) Balance(ctx context.Context, psychicID uuid.UUID) (*PayoutView, error) {
	b, err := s.repo.Balance(ctx, psychicID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &PayoutView{AvailableCredits: b.Available(), ReservedCredits: b.Reserved, PaidCredits: b.Paid, LifetimeNetCredits: b.LifetimeNet,
		CentsPerCredit: s.cfg.CentsPerCredit, Currency: s.cfg.Currency, MinWithdrawal: s.cfg.MinCredits}, nil
}

// maskDestination keeps only enough to recognise the destination.
func maskDestination(method, dest string) string {
	if method == "paypal" {
		at := strings.LastIndexByte(dest, '@')
		if at <= 0 {
			return "•••"
		}
		local := dest[:at]
		return string([]rune(local)[0]) + "•••" + dest[at:]
	}
	compact := strings.ReplaceAll(dest, " ", "")
	if len(compact) <= 4 {
		return "••••"
	}
	return "•••• " + compact[len(compact)-4:]
}

func validDestination(method, dest string) (string, string) {
	dest = CleanLine(dest)
	if dest == "" || utf8.RuneCountInString(dest) > 200 {
		return "", "Enter where we should send the money (up to 200 characters)."
	}
	switch method {
	case "paypal":
		addr, err := mail.ParseAddress(dest)
		if err != nil || addr.Address != dest {
			return "", "Enter the email address of your PayPal account."
		}
	case "bank_transfer":
		compact := strings.ReplaceAll(dest, " ", "")
		if len(compact) < 8 {
			return "", "Enter your full IBAN or account details."
		}
	default:
		return "", ""
	}
	return dest, ""
}

// Request creates a payout request for available credits.
func (s *WithdrawalService) Request(ctx context.Context, psychicID uuid.UUID, in WithdrawalInput) (*WithdrawalView, error) {
	fields := map[string]string{}
	if in.Method != "bank_transfer" && in.Method != "paypal" {
		fields["method"] = "Choose bank transfer or PayPal."
	}
	dest, msg := validDestination(in.Method, in.Destination)
	if msg != "" {
		fields["destination"] = msg
	}
	if in.AmountCredits < s.cfg.MinCredits {
		fields["amount_credits"] = "The amount is below the minimum withdrawal."
	}
	if !ValidIdempotencyKey(in.IdempotencyKey) {
		fields["idempotency_key"] = "A valid idempotency key is required."
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	if existing, err := s.repo.GetByKey(ctx, psychicID, in.IdempotencyKey); err == nil {
		v := withdrawalView(existing)
		return &v, nil
	}
	sealed, err := s.sealer.Seal(dest, psychicID.String())
	if err != nil {
		return nil, apperror.Internal(err)
	}
	now := s.clock.Now()
	w := &domain.Withdrawal{ID: domain.NewID(), PsychicID: psychicID, AmountCredits: in.AmountCredits,
		AmountCents: in.AmountCredits * s.cfg.CentsPerCredit, Currency: s.cfg.Currency, Method: in.Method, DestinationEnc: sealed,
		DestinationHint: maskDestination(in.Method, dest), Status: domain.WithdrawalPending, IdempotencyKey: in.IdempotencyKey,
		RequestedAt: now, CreatedAt: now, UpdatedAt: now}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		// Serialise withdrawals per advisor so two requests cannot both
		// spend the same balance.
		if err := s.repo.LockPsychic(ctx, psychicID); err != nil {
			return err
		}
		b, err := s.repo.Balance(ctx, psychicID)
		if err != nil {
			return err
		}
		if w.AmountCredits > b.Available() {
			return apperror.Validation(map[string]string{"amount_credits": "The amount is more than your available balance."}).
				WithDetails(map[string]any{"available_credits": b.Available()})
		}
		return s.repo.Create(ctx, w)
	})
	if errors.Is(err, repository.ErrDuplicate) {
		return nil, apperror.Conflict(apperror.CodeConflict, "You already have a withdrawal in progress.")
	}
	if err != nil {
		return nil, mapRepoErr(err, "Advisor")
	}
	s.tracker.Track(ctx, "withdrawal.requested", map[string]any{"amount_credits": w.AmountCredits, "method": w.Method})
	v := withdrawalView(w)
	return &v, nil
}

// Cancel withdraws a pending request.
func (s *WithdrawalService) Cancel(ctx context.Context, psychicID, id uuid.UUID) (*WithdrawalView, error) {
	var w *domain.Withdrawal
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if w, err = s.repo.GetForUpdate(ctx, id); err != nil {
			return err
		}
		if w.PsychicID != psychicID {
			return repository.ErrNotFound
		}
		if w.Status != domain.WithdrawalPending {
			return apperror.InvalidState("Only pending withdrawals can be cancelled.")
		}
		now := s.clock.Now()
		w.Status, w.ProcessedAt = domain.WithdrawalCancelled, &now
		return s.repo.Save(ctx, w)
	})
	if err != nil {
		return nil, mapRepoErr(err, "Withdrawal")
	}
	v := withdrawalView(w)
	return &v, nil
}

// List returns an advisor's withdrawals.
func (s *WithdrawalService) List(ctx context.Context, psychicID uuid.UUID, cursor string, limit int) (Page[WithdrawalView], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[WithdrawalView]{}, err
	}
	rows, err := s.repo.List(ctx, repository.WithdrawalFilter{PsychicID: &psychicID, Cursor: c, Limit: limit})
	if err != nil {
		return Page[WithdrawalView]{}, apperror.Internal(err)
	}
	page := Page[WithdrawalView]{Items: make([]WithdrawalView, 0, len(rows))}
	for i := range rows {
		page.Items = append(page.Items, withdrawalView(&rows[i]))
	}
	if len(rows) > 0 && len(rows) == clampPage(limit, 20, 100) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.RequestedAt, last.ID.String())
	}
	return page, nil
}

func (s *WithdrawalService) adminView(ctx context.Context, w *domain.Withdrawal) AdminWithdrawalView {
	v := AdminWithdrawalView{WithdrawalView: withdrawalView(w), Psychic: ParticipantRef{ID: w.PsychicID}, ProcessedBy: w.ProcessedBy, Reference: w.Reference}
	if p, err := s.psychics.GetByUserID(ctx, w.PsychicID); err == nil {
		v.Psychic.Name, v.Psychic.Slug = p.Name, p.Slug
	}
	return v
}

// AdminList lists withdrawals for processing.
func (s *WithdrawalService) AdminList(ctx context.Context, statuses []domain.WithdrawalStatus, psychicID *uuid.UUID, cursor string, limit int) (Page[AdminWithdrawalView], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[AdminWithdrawalView]{}, err
	}
	rows, err := s.repo.List(ctx, repository.WithdrawalFilter{PsychicID: psychicID, Statuses: statuses, Cursor: c, Limit: limit})
	if err != nil {
		return Page[AdminWithdrawalView]{}, apperror.Internal(err)
	}
	page := Page[AdminWithdrawalView]{Items: make([]AdminWithdrawalView, 0, len(rows))}
	for i := range rows {
		page.Items = append(page.Items, s.adminView(ctx, &rows[i]))
	}
	if len(rows) > 0 && len(rows) == clampPage(limit, 20, 100) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.RequestedAt, last.ID.String())
	}
	return page, nil
}

// ParseWithdrawalStatuses parses a comma-separated status filter.
func ParseWithdrawalStatuses(raw string) ([]domain.WithdrawalStatus, error) {
	var out []domain.WithdrawalStatus
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		switch domain.WithdrawalStatus(part) {
		case domain.WithdrawalPending, domain.WithdrawalApproved, domain.WithdrawalPaid, domain.WithdrawalRejected, domain.WithdrawalCancelled:
			out = append(out, domain.WithdrawalStatus(part))
		default:
			return nil, apperror.Validation(map[string]string{"status": "Unknown status."})
		}
	}
	return out, nil
}

// Destination decrypts the payout details (audited; step-up enforced by
// the route).
func (s *WithdrawalService) Destination(ctx context.Context, id uuid.UUID) (string, error) {
	w, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", mapRepoErr(err, "Withdrawal")
	}
	dest, err := s.sealer.Open(w.DestinationEnc, w.PsychicID.String())
	if err != nil {
		return "", apperror.Internal(err)
	}
	recordAudit(ctx, s.audit, s.clock, s.cfg.IPHashSecret, "withdrawal.destination_viewed", "withdrawal", id.String(), nil)
	return dest, nil
}

// Transition moves a withdrawal through approve / pay / reject.
func (s *WithdrawalService) Transition(ctx context.Context, adminID, id uuid.UUID, to domain.WithdrawalStatus, note, reference string) (*AdminWithdrawalView, error) {
	note = CleanLine(note)
	reference = CleanLine(reference)
	if utf8.RuneCountInString(note) > 500 {
		return nil, apperror.Validation(map[string]string{"note": "Use at most 500 characters."})
	}
	if utf8.RuneCountInString(reference) > 120 {
		return nil, apperror.Validation(map[string]string{"reference": "Use at most 120 characters."})
	}
	if to == domain.WithdrawalRejected && note == "" {
		return nil, apperror.Validation(map[string]string{"note": "Explain why the withdrawal is rejected."})
	}
	var w *domain.Withdrawal
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if w, err = s.repo.GetForUpdate(ctx, id); err != nil {
			return err
		}
		allowed := map[domain.WithdrawalStatus][]domain.WithdrawalStatus{
			domain.WithdrawalApproved: {domain.WithdrawalPending},
			domain.WithdrawalPaid:     {domain.WithdrawalApproved},
			domain.WithdrawalRejected: {domain.WithdrawalPending, domain.WithdrawalApproved},
		}
		ok := false
		for _, from := range allowed[to] {
			ok = ok || w.Status == from
		}
		if !ok {
			return apperror.InvalidState("This withdrawal cannot move to that status.").WithDetails(map[string]any{"status": w.Status})
		}
		now := s.clock.Now()
		w.Status, w.ProcessedAt, w.ProcessedBy = to, &now, &adminID
		if note != "" {
			w.Note = note
		}
		if reference != "" {
			w.Reference = reference
		}
		return s.repo.Save(ctx, w)
	})
	if err != nil {
		return nil, mapRepoErr(err, "Withdrawal")
	}
	recordAudit(ctx, s.audit, s.clock, s.cfg.IPHashSecret, "withdrawal."+string(to), "withdrawal", id.String(), map[string]any{
		"amount_credits": w.AmountCredits, "psychic_id": w.PsychicID.String(), "reference": reference})
	s.notify.Notify(ctx, NotificationSpec{Kind: NotifyWithdrawal, Data: map[string]any{"withdrawal_id": w.ID, "status": string(to),
		"amount_credits": w.AmountCredits}, Link: "/profile/analytics"}, w.PsychicID)
	v := s.adminView(ctx, w)
	return &v, nil
}
