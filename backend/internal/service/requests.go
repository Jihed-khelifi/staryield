package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// RequestSettings are the custom request business parameters.
type RequestSettings struct {
	AcceptTimeout   time.Duration
	DeliveryTimeout time.Duration
	DisputeWindow   time.Duration
	MaxNoteLength   int
	MaxDelivery     int
	SharePercent    int
}

// CustomRequestView is an order as seen by its client, psychic or an admin.
type CustomRequestView struct {
	ID             uuid.UUID       `json:"id"`
	Client         ParticipantView `json:"client"`
	Psychic        ParticipantView `json:"psychic"`
	ServiceID      uuid.UUID       `json:"service_id"`
	ServiceName    string          `json:"service_name"`
	PriceCredits   int64           `json:"price_credits"`
	Status         string          `json:"status"`
	ClientNote     string          `json:"client_note"`
	PreferredAt    *time.Time      `json:"preferred_at,omitempty"`
	Delivery       string          `json:"delivery,omitempty"`
	DeclineReason  string          `json:"decline_reason,omitempty"`
	DisputeReason  string          `json:"dispute_reason,omitempty"`
	ResolutionNote string          `json:"resolution_note,omitempty"`
	AcceptBy       time.Time       `json:"accept_by"`
	DeliverBy      *time.Time      `json:"deliver_by,omitempty"`
	AcceptedAt     *time.Time      `json:"accepted_at,omitempty"`
	DeliveredAt    *time.Time      `json:"delivered_at,omitempty"`
	ClosedAt       *time.Time      `json:"closed_at,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	Wallet         *WalletView     `json:"wallet,omitempty"`
}

// OrderInput is a custom request order.
type OrderInput struct {
	ServiceID      uuid.UUID
	Note           string
	PreferredAt    *time.Time
	IdempotencyKey string
}

// CustomRequestService implements the custom request flow (§5.1, FR-07,
// BR-06, BR-07).
type CustomRequestService struct {
	tx       database.TxManager
	requests repository.CustomRequestRepository
	psychics repository.PsychicRepository
	wallet   *WalletService
	wallets  repository.WalletRepository
	earnings repository.EarningRepository
	chat     *ChatService
	events   EventPublisher
	tracker  analytics.Tracker
	clock    clock.Clock
	cfg      RequestSettings
}

// NewCustomRequestService wires the service.
func NewCustomRequestService(tx database.TxManager, requests repository.CustomRequestRepository, psychics repository.PsychicRepository,
	wallet *WalletService, wallets repository.WalletRepository, earnings repository.EarningRepository, chat *ChatService,
	events EventPublisher, tracker analytics.Tracker, clk clock.Clock, cfg RequestSettings) *CustomRequestService {
	return &CustomRequestService{tx: tx, requests: requests, psychics: psychics, wallet: wallet, wallets: wallets,
		earnings: earnings, chat: chat, events: events, tracker: tracker, clock: clk, cfg: cfg}
}

func (s *CustomRequestService) view(ctx context.Context, r *domain.CustomRequest, ps *participants) CustomRequestView {
	return CustomRequestView{
		ID: r.ID, Client: ps.client(ctx, r.ClientID), Psychic: ps.psychic(ctx, r.PsychicID), ServiceID: r.ServiceID,
		ServiceName: r.ServiceName, PriceCredits: r.PriceCredits, Status: string(r.Status), ClientNote: r.ClientNote,
		PreferredAt: r.PreferredAt, Delivery: r.Delivery, DeclineReason: r.DeclineReason, DisputeReason: r.DisputeReason,
		ResolutionNote: r.ResolutionNote, AcceptBy: r.AcceptBy, DeliverBy: r.DeliverBy, AcceptedAt: r.AcceptedAt,
		DeliveredAt: r.DeliveredAt, ClosedAt: r.ClosedAt, CreatedAt: r.CreatedAt,
	}
}

func (s *CustomRequestService) publish(ctx context.Context, r *domain.CustomRequest, event string) {
	ps := s.chat.newParticipants()
	v := s.view(ctx, r, ps)
	warnErr(ctx, "requests: publish", s.events.Publish(ctx, event, map[string]any{"request": v}, r.ClientID, r.PsychicID))
}

func (s *CustomRequestService) track(ctx context.Context, name string, r *domain.CustomRequest, props map[string]any) {
	if props == nil {
		props = map[string]any{}
	}
	props["request_id"], props["psychic_id"], props["service_id"], props["credits"] = r.ID, r.PsychicID, r.ServiceID, r.PriceCredits
	if _, ok := reqctx.PrincipalFrom(ctx); !ok {
		ctx = reqctx.WithPrincipal(ctx, reqctx.Principal{UserID: r.ClientID, Role: domain.RoleClient})
	}
	s.tracker.Track(ctx, name, props)
}

// Order places a custom request and holds its price (FR-07, BR-06).
func (s *CustomRequestService) Order(ctx context.Context, clientID uuid.UUID, in OrderInput) (*CustomRequestView, error) {
	if !ValidIdempotencyKey(in.IdempotencyKey) {
		return nil, apperror.Validation(map[string]string{"idempotency_key": "A valid idempotency key is required."})
	}
	note := CleanText(in.Note)
	if utf8.RuneCountInString(note) > s.cfg.MaxNoteLength {
		return nil, apperror.Validation(map[string]string{"note": "Your note is too long."})
	}
	now := s.clock.Now()
	if in.PreferredAt != nil && (in.PreferredAt.Before(now.Add(-time.Hour)) || in.PreferredAt.After(now.AddDate(1, 0, 0))) {
		return nil, apperror.Validation(map[string]string{"preferred_at": "Choose a date within the next year."})
	}
	if existing, err := s.requests.GetByKey(ctx, clientID, in.IdempotencyKey); err == nil {
		v := s.view(ctx, existing, s.chat.newParticipants())
		return &v, nil
	} else if !isNotFound(err) {
		return nil, apperror.Internal(err)
	}
	svc, err := s.psychics.GetService(ctx, in.ServiceID)
	if err != nil || !svc.Active {
		return nil, apperror.NotFound("Service")
	}
	psychic, err := s.psychics.GetByUserID(ctx, svc.PsychicID)
	if err != nil || !psychic.Listed {
		return nil, apperror.NotFound("Advisor")
	}
	if psychic.UserID == clientID {
		return nil, apperror.Forbidden("You cannot order your own service.")
	}
	r := &domain.CustomRequest{
		ID: domain.NewID(), ClientID: clientID, PsychicID: psychic.UserID, ServiceID: svc.ID, ServiceName: svc.Name,
		PriceCredits: svc.PriceCredits, Status: domain.RequestPending, ClientNote: note, PreferredAt: in.PreferredAt,
		IdempotencyKey: in.IdempotencyKey, AcceptBy: now.Add(s.cfg.AcceptTimeout), CreatedAt: now, UpdatedAt: now,
	}
	var wallet *domain.Wallet
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.requests.Create(ctx, r); err != nil {
			return err
		}
		w, _, err := s.wallet.Apply(ctx, Movement{
			UserID: clientID, Type: domain.LedgerHold, Amount: -r.PriceCredits, HeldAmount: r.PriceCredits,
			ReferenceType: "custom_request", ReferenceID: &r.ID, IdempotencyKey: "request_hold:" + r.ID.String(),
			Description: "Held for " + r.ServiceName,
		})
		wallet = w
		return err
	})
	if errors.Is(err, ErrInsufficientFunds) {
		available := int64(0)
		if wallet != nil {
			available = wallet.Balance
		}
		return nil, apperror.InsufficientCredits(r.PriceCredits, available)
	}
	if errors.Is(err, repository.ErrDuplicate) {
		existing, err2 := s.requests.GetByKey(ctx, clientID, in.IdempotencyKey)
		if err2 != nil {
			return nil, apperror.Internal(err2)
		}
		v := s.view(ctx, existing, s.chat.newParticipants())
		return &v, nil
	}
	if err != nil {
		return nil, mapRepoErr(err, "Request")
	}
	s.publish(ctx, r, realtime.EventCustomRequestCreated)
	warnErr(ctx, "requests: publish wallet", s.events.Publish(ctx, realtime.EventWalletUpdated, walletView(wallet), clientID))
	s.track(ctx, "custom_request.ordered", r, nil)
	v := s.view(ctx, r, s.chat.newParticipants())
	wv := walletView(wallet)
	v.Wallet = &wv
	return &v, nil
}

// transition locks a request, checks the actor, applies mutate, and runs
// credit movements inside one transaction.
func (s *CustomRequestService) transition(ctx context.Context, id uuid.UUID, authorize func(*domain.CustomRequest) error,
	mutate func(ctx context.Context, r *domain.CustomRequest, now time.Time) (changed bool, err error)) (*domain.CustomRequest, bool, error) {
	var out *domain.CustomRequest
	var changed bool
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		r, err := s.requests.GetForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if err := authorize(r); err != nil {
			return err
		}
		changed, err = mutate(ctx, r, s.clock.Now())
		if err != nil {
			return err
		}
		out = r
		if changed {
			return s.requests.Save(ctx, r)
		}
		return nil
	})
	if err != nil {
		return nil, false, mapRepoErr(err, "Request")
	}
	return out, changed, nil
}

func (s *CustomRequestService) refundHold(ctx context.Context, r *domain.CustomRequest, key, description string) error {
	_, _, err := s.wallet.Apply(ctx, Movement{
		UserID: r.ClientID, Type: domain.LedgerHoldRefund, Amount: r.PriceCredits, HeldAmount: -r.PriceCredits,
		ReferenceType: "custom_request", ReferenceID: &r.ID, IdempotencyKey: key + ":" + r.ID.String(), Description: description,
	})
	return err
}

func asPsychic(id uuid.UUID) func(*domain.CustomRequest) error {
	return func(r *domain.CustomRequest) error {
		if r.PsychicID != id {
			return repository.ErrNotFound
		}
		return nil
	}
}

func asClient(id uuid.UUID) func(*domain.CustomRequest) error {
	return func(r *domain.CustomRequest) error {
		if r.ClientID != id {
			return repository.ErrNotFound
		}
		return nil
	}
}

func invalidRequestState(r *domain.CustomRequest) error {
	return apperror.InvalidState("This request cannot be changed in its current state.").WithDetails(map[string]any{"status": r.Status})
}

func (s *CustomRequestService) finish(ctx context.Context, r *domain.CustomRequest, changed bool, metric string) (*CustomRequestView, error) {
	if changed {
		s.publish(ctx, r, realtime.EventCustomRequestUpdated)
		if w, err := s.wallets.Get(ctx, r.ClientID); err == nil {
			warnErr(ctx, "requests: publish wallet", s.events.Publish(ctx, realtime.EventWalletUpdated, walletView(w), r.ClientID))
		}
		s.track(ctx, metric, r, nil)
	}
	v := s.view(ctx, r, s.chat.newParticipants())
	return &v, nil
}

// Accept is the psychic accepting an order.
func (s *CustomRequestService) Accept(ctx context.Context, psychicID, id uuid.UUID) (*CustomRequestView, error) {
	r, changed, err := s.transition(ctx, id, asPsychic(psychicID), func(ctx context.Context, r *domain.CustomRequest, now time.Time) (bool, error) {
		if r.Status == domain.RequestAccepted {
			return false, nil
		}
		if r.Status != domain.RequestPending {
			return false, invalidRequestState(r)
		}
		if !now.Before(r.AcceptBy) {
			return false, apperror.InvalidState("This request has expired.")
		}
		deliverBy := now.Add(s.cfg.DeliveryTimeout)
		r.Status, r.AcceptedAt, r.DeliverBy = domain.RequestAccepted, &now, &deliverBy
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	return s.finish(ctx, r, changed, "custom_request.accepted")
}

// Decline is the psychic declining; held credits return to the client (BR-07).
func (s *CustomRequestService) Decline(ctx context.Context, psychicID, id uuid.UUID, reason string) (*CustomRequestView, error) {
	reason = CleanLine(reason)
	if utf8.RuneCountInString(reason) > 500 {
		return nil, apperror.Validation(map[string]string{"reason": "Use at most 500 characters."})
	}
	r, changed, err := s.transition(ctx, id, asPsychic(psychicID), func(ctx context.Context, r *domain.CustomRequest, now time.Time) (bool, error) {
		if r.Status == domain.RequestDeclined {
			return false, nil
		}
		if r.Status != domain.RequestPending {
			return false, invalidRequestState(r)
		}
		if err := s.refundHold(ctx, r, "request_refund", "Refund: request declined"); err != nil {
			return false, err
		}
		r.Status, r.DeclineReason, r.ClosedAt = domain.RequestDeclined, reason, &now
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	return s.finish(ctx, r, changed, "custom_request.declined")
}

// Cancel is the client cancelling a pending order (full refund).
func (s *CustomRequestService) Cancel(ctx context.Context, clientID, id uuid.UUID) (*CustomRequestView, error) {
	r, changed, err := s.transition(ctx, id, asClient(clientID), func(ctx context.Context, r *domain.CustomRequest, now time.Time) (bool, error) {
		if r.Status == domain.RequestCancelled {
			return false, nil
		}
		if r.Status != domain.RequestPending {
			return false, apperror.InvalidState("Accepted requests can no longer be cancelled.")
		}
		if err := s.refundHold(ctx, r, "request_refund", "Refund: request cancelled"); err != nil {
			return false, err
		}
		r.Status, r.ClosedAt = domain.RequestCancelled, &now
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	return s.finish(ctx, r, changed, "custom_request.cancelled")
}

// Deliver is the psychic fulfilling an order: the client receives the
// delivery and held credits are released to the psychic's earnings (BR-06).
func (s *CustomRequestService) Deliver(ctx context.Context, psychicID, id uuid.UUID, delivery string) (*CustomRequestView, error) {
	delivery = CleanText(delivery)
	if delivery == "" || utf8.RuneCountInString(delivery) > s.cfg.MaxDelivery {
		return nil, apperror.Validation(map[string]string{"delivery": "Write the delivery (up to the length limit)."})
	}
	r, changed, err := s.transition(ctx, id, asPsychic(psychicID), func(ctx context.Context, r *domain.CustomRequest, now time.Time) (bool, error) {
		if r.Status == domain.RequestDelivered {
			return false, nil
		}
		if r.Status != domain.RequestAccepted {
			return false, invalidRequestState(r)
		}
		if _, _, err := s.wallet.Apply(ctx, Movement{
			UserID: r.ClientID, Type: domain.LedgerHoldRelease, HeldAmount: -r.PriceCredits,
			ReferenceType: "custom_request", ReferenceID: &r.ID, IdempotencyKey: "request_release:" + r.ID.String(),
			Description: "Released to advisor: " + r.ServiceName,
		}); err != nil {
			return false, err
		}
		net, commission := domain.SplitEarnings(r.PriceCredits, s.cfg.SharePercent)
		if err := s.earnings.Create(ctx, &domain.Earning{ID: domain.NewID(), PsychicID: r.PsychicID, SourceType: domain.EarningCustomRequest,
			SourceID: r.ID, EntryKind: domain.EarningKindEarning, GrossCredits: r.PriceCredits, CommissionCredits: commission,
			NetCredits: net, SharePercent: s.cfg.SharePercent, CreatedAt: now}); err != nil && !errors.Is(err, repository.ErrDuplicate) {
			return false, err
		}
		r.Status, r.Delivery, r.DeliveredAt = domain.RequestDelivered, delivery, &now
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	return s.finish(ctx, r, changed, "custom_request.delivered")
}

// Dispute lets the client contest a delivery within the dispute window.
func (s *CustomRequestService) Dispute(ctx context.Context, clientID, id uuid.UUID, reason string) (*CustomRequestView, error) {
	reason = CleanText(reason)
	if reason == "" || utf8.RuneCountInString(reason) > 2000 {
		return nil, apperror.Validation(map[string]string{"reason": "Tell us what went wrong (up to 2000 characters)."})
	}
	r, changed, err := s.transition(ctx, id, asClient(clientID), func(ctx context.Context, r *domain.CustomRequest, now time.Time) (bool, error) {
		if r.Status != domain.RequestDelivered {
			return false, invalidRequestState(r)
		}
		if r.DeliveredAt != nil && now.Sub(*r.DeliveredAt) > s.cfg.DisputeWindow {
			return false, apperror.InvalidState("The dispute window for this request has closed.")
		}
		r.Status, r.DisputeReason = domain.RequestDisputed, reason
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	return s.finish(ctx, r, changed, "custom_request.disputed")
}

// Resolve is an admin decision on a request (§5.1 "Admin can audit/dispute"):
// refund returns the credits to the client (reversing earnings when the
// request was delivered); reject keeps the delivery in place.
func (s *CustomRequestService) Resolve(ctx context.Context, adminID, id uuid.UUID, refund bool, note string) (*CustomRequestView, error) {
	note = CleanText(note)
	r, changed, err := s.transition(ctx, id, func(*domain.CustomRequest) error { return nil },
		func(ctx context.Context, r *domain.CustomRequest, now time.Time) (bool, error) {
			if !refund {
				if r.Status != domain.RequestDisputed {
					return false, invalidRequestState(r)
				}
				r.Status, r.ResolutionNote, r.ResolvedBy = domain.RequestDelivered, note, &adminID
				return true, nil
			}
			switch r.Status {
			case domain.RequestPending, domain.RequestAccepted:
				if err := s.refundHold(ctx, r, "request_admin_refund", "Refund issued by support"); err != nil {
					return false, err
				}
			case domain.RequestDelivered, domain.RequestDisputed:
				// Credits were released; return them from the platform and
				// reverse the psychic's earning.
				if _, _, err := s.wallet.Apply(ctx, Movement{UserID: r.ClientID, Type: domain.LedgerRefund, Amount: r.PriceCredits,
					ReferenceType: "custom_request", ReferenceID: &r.ID, IdempotencyKey: "request_dispute_refund:" + r.ID.String(),
					Description: "Refund issued by support", ActorID: &adminID}); err != nil {
					return false, err
				}
				net, commission := domain.SplitEarnings(r.PriceCredits, s.cfg.SharePercent)
				if err := s.earnings.Create(ctx, &domain.Earning{ID: domain.NewID(), PsychicID: r.PsychicID, SourceType: domain.EarningCustomRequest,
					SourceID: r.ID, EntryKind: domain.EarningKindReversal, GrossCredits: -r.PriceCredits, CommissionCredits: -commission,
					NetCredits: -net, SharePercent: s.cfg.SharePercent, CreatedAt: now}); err != nil && !errors.Is(err, repository.ErrDuplicate) {
					return false, err
				}
			default:
				return false, invalidRequestState(r)
			}
			r.Status, r.ResolutionNote, r.ResolvedBy, r.ClosedAt = domain.RequestRefunded, note, &adminID, &now
			return true, nil
		})
	if err != nil {
		return nil, err
	}
	return s.finish(ctx, r, changed, "custom_request.resolved")
}

// Get returns one request for its client, psychic or an admin.
func (s *CustomRequestService) Get(ctx context.Context, p reqctx.Principal, id uuid.UUID) (*CustomRequestView, error) {
	r, err := s.requests.Get(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Request")
	}
	if !p.IsAdmin() && r.ClientID != p.UserID && r.PsychicID != p.UserID {
		return nil, apperror.NotFound("Request")
	}
	v := s.view(ctx, r, s.chat.newParticipants())
	return &v, nil
}

// List returns the caller's requests (as client or psychic).
func (s *CustomRequestService) List(ctx context.Context, p reqctx.Principal, statuses []domain.CustomRequestStatus, cursor string, limit int) (Page[CustomRequestView], error) {
	cur, err := decodeCursor(cursor)
	if err != nil {
		return Page[CustomRequestView]{}, err
	}
	f := repository.RequestFilter{Statuses: statuses, Cursor: cur, Limit: limit}
	switch p.Role {
	case domain.RolePsychic:
		f.PsychicID = &p.UserID
	case domain.RoleAdmin:
	default:
		f.ClientID = &p.UserID
	}
	return s.page(ctx, f)
}

// AdminList lists requests across the platform.
func (s *CustomRequestService) AdminList(ctx context.Context, statuses []domain.CustomRequestStatus, cursor string, limit int) (Page[CustomRequestView], error) {
	cur, err := decodeCursor(cursor)
	if err != nil {
		return Page[CustomRequestView]{}, err
	}
	return s.page(ctx, repository.RequestFilter{Statuses: statuses, Cursor: cur, Limit: limit})
}

func (s *CustomRequestService) page(ctx context.Context, f repository.RequestFilter) (Page[CustomRequestView], error) {
	rows, err := s.requests.List(ctx, f)
	if err != nil {
		return Page[CustomRequestView]{}, apperror.Internal(err)
	}
	ps := s.chat.newParticipants()
	page := Page[CustomRequestView]{Items: make([]CustomRequestView, 0, len(rows))}
	for i := range rows {
		page.Items = append(page.Items, s.view(ctx, &rows[i], ps))
	}
	if len(rows) > 0 && len(rows) == clampPage(f.Limit, 20, 100) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return page, nil
}

// ExpireDue refunds pending orders not accepted in time and accepted orders
// not delivered in time (BR-07).
func (s *CustomRequestService) ExpireDue(ctx context.Context, batch int) (int, error) {
	var expired []domain.CustomRequest
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		expired = expired[:0]
		now := s.clock.Now()
		pending, err := s.requests.ClaimExpiredPending(ctx, now, batch)
		if err != nil {
			return err
		}
		overdue, err := s.requests.ClaimOverdueDelivery(ctx, now, batch)
		if err != nil {
			return err
		}
		for _, list := range [][]domain.CustomRequest{pending, overdue} {
			for i := range list {
				r := &list[i]
				if err := s.refundHold(ctx, r, "request_expired_refund", "Refund: request expired"); err != nil {
					return err
				}
				r.Status, r.ClosedAt = domain.RequestExpired, &now
				if err := s.requests.Save(ctx, r); err != nil {
					return err
				}
				expired = append(expired, *r)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	for i := range expired {
		r := &expired[i]
		s.publish(ctx, r, realtime.EventCustomRequestUpdated)
		if w, err := s.wallets.Get(ctx, r.ClientID); err == nil {
			warnErr(ctx, "requests: publish wallet", s.events.Publish(ctx, realtime.EventWalletUpdated, walletView(w), r.ClientID))
		}
		s.track(ctx, "custom_request.expired", r, nil)
	}
	if len(expired) > 0 {
		logger.FromContext(ctx).Info("custom requests expired", zap.Int("count", len(expired)))
	}
	return len(expired), nil
}

// ParseRequestStatuses parses a comma-separated status filter.
func ParseRequestStatuses(raw string) ([]domain.CustomRequestStatus, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	valid := map[domain.CustomRequestStatus]bool{
		domain.RequestPending: true, domain.RequestAccepted: true, domain.RequestDelivered: true, domain.RequestDeclined: true,
		domain.RequestCancelled: true, domain.RequestExpired: true, domain.RequestDisputed: true, domain.RequestRefunded: true,
	}
	var out []domain.CustomRequestStatus
	for _, part := range strings.Split(raw, ",") {
		st := domain.CustomRequestStatus(strings.TrimSpace(part))
		if !valid[st] {
			return nil, apperror.Validation(map[string]string{"status": "Unknown status filter."})
		}
		out = append(out, st)
	}
	return out, nil
}

// ParseSessionStatuses parses a comma-separated session status filter.
func ParseSessionStatuses(raw string) ([]domain.SessionStatus, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	valid := map[domain.SessionStatus]bool{}
	for _, st := range []domain.SessionStatus{domain.SessionRequested, domain.SessionAccepted, domain.SessionActive, domain.SessionDisconnected,
		domain.SessionEnded, domain.SessionSettled, domain.SessionDeclined, domain.SessionExpired, domain.SessionCancelled} {
		valid[st] = true
	}
	var out []domain.SessionStatus
	for _, part := range strings.Split(raw, ",") {
		st := domain.SessionStatus(strings.TrimSpace(part))
		if !valid[st] {
			return nil, apperror.Validation(map[string]string{"status": "Unknown status filter."})
		}
		out = append(out, st)
	}
	return out, nil
}
