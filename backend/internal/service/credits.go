package service

import (
	"context"
	"errors"
	"net/http"
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
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// PackView is a purchasable credit pack.
type PackView struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Credits    int64     `json:"credits"`
	PriceCents int64     `json:"price_cents"`
	Currency   string    `json:"currency"`
	Featured   bool      `json:"featured"`
	Active     bool      `json:"active"`
	SortOrder  int       `json:"sort_order"`
}

// PurchaseView is a credit purchase.
type PurchaseView struct {
	ID            uuid.UUID   `json:"id"`
	PackID        uuid.UUID   `json:"pack_id"`
	Credits       int64       `json:"credits"`
	PriceCents    int64       `json:"price_cents"`
	Currency      string      `json:"currency"`
	Status        string      `json:"status"`
	Provider      string      `json:"provider"`
	FailureReason string      `json:"failure_reason,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	CompletedAt   *time.Time  `json:"completed_at,omitempty"`
	Wallet        *WalletView `json:"wallet,omitempty"`
}

// PurchaseInput starts a purchase.
type PurchaseInput struct {
	PackID         uuid.UUID
	IdempotencyKey string
	ReceiptEmail   string
}

// CreditService sells credit packs (FR-05).
type CreditService struct {
	tx       database.TxManager
	credits  repository.CreditRepository
	wallet   *WalletService
	wallets  repository.WalletRepository
	provider payment.Provider
	events   EventPublisher
	tracker  analytics.Tracker
	clock    clock.Clock
}

// NewCreditService wires the credit service.
func NewCreditService(tx database.TxManager, credits repository.CreditRepository, wallet *WalletService, wallets repository.WalletRepository,
	provider payment.Provider, events EventPublisher, tracker analytics.Tracker, clk clock.Clock) *CreditService {
	return &CreditService{tx: tx, credits: credits, wallet: wallet, wallets: wallets, provider: provider, events: events, tracker: tracker, clock: clk}
}

func packView(p *domain.CreditPack) PackView {
	return PackView{ID: p.ID, Name: p.Name, Credits: p.Credits, PriceCents: p.PriceCents, Currency: p.Currency, Featured: p.Featured, Active: p.Active, SortOrder: p.SortOrder}
}

func purchaseView(p *domain.CreditPurchase) PurchaseView {
	return PurchaseView{ID: p.ID, PackID: p.PackID, Credits: p.Credits, PriceCents: p.PriceCents, Currency: p.Currency,
		Status: string(p.Status), Provider: p.Provider, FailureReason: p.FailureReason, CreatedAt: p.CreatedAt, CompletedAt: p.CompletedAt}
}

// Packs lists purchasable packs.
func (s *CreditService) Packs(ctx context.Context) ([]PackView, error) {
	packs, err := s.credits.ListPacks(ctx, true)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]PackView, 0, len(packs))
	for i := range packs {
		out = append(out, packView(&packs[i]))
	}
	return out, nil
}

// Purchase charges the provider and credits the wallet on success. Replays
// with the same idempotency key return the original purchase.
func (s *CreditService) Purchase(ctx context.Context, userID uuid.UUID, in PurchaseInput) (*PurchaseView, error) {
	if !ValidIdempotencyKey(in.IdempotencyKey) {
		return nil, apperror.Validation(map[string]string{"idempotency_key": "A valid idempotency key is required."})
	}
	email := ""
	if strings.TrimSpace(in.ReceiptEmail) != "" {
		e, ok := NormalizeEmail(in.ReceiptEmail)
		if !ok {
			return nil, apperror.Validation(map[string]string{"receipt_email": "Enter a valid email address."})
		}
		email = e
	}
	if existing, err := s.credits.GetPurchaseByKey(ctx, userID, in.IdempotencyKey); err == nil {
		return s.withWallet(ctx, existing)
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, apperror.Internal(err)
	}
	pack, err := s.credits.GetPack(ctx, in.PackID)
	if err != nil || !pack.Active {
		return nil, apperror.NotFound("Credit pack")
	}
	now := s.clock.Now()
	p := &domain.CreditPurchase{
		ID: domain.NewID(), UserID: userID, PackID: pack.ID, Credits: pack.Credits, PriceCents: pack.PriceCents,
		Currency: pack.Currency, Status: domain.PurchasePending, Provider: s.provider.Name(),
		IdempotencyKey: in.IdempotencyKey, ReceiptEmail: email, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.credits.CreatePurchase(ctx, p); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			if existing, err2 := s.credits.GetPurchaseByKey(ctx, userID, in.IdempotencyKey); err2 == nil {
				return s.withWallet(ctx, existing)
			}
		}
		return nil, apperror.Internal(err)
	}
	s.tracker.Track(ctx, "credits.purchase_started", map[string]any{"pack_id": pack.ID, "credits": pack.Credits, "amount_cents": pack.PriceCents, "currency": pack.Currency})

	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	res, err := s.provider.Checkout(cctx, payment.CheckoutRequest{
		PurchaseID: p.ID, UserID: userID, AmountCents: p.PriceCents, Currency: p.Currency,
		Description: pack.Name, Email: email,
	})
	cancel()
	if err != nil {
		logger.FromContext(ctx).Error("payment: checkout failed", zap.Error(err), zap.String("purchase_id", p.ID.String()))
		_ = s.credits.UpdatePurchase(ctx, p.ID, map[string]any{"status": domain.PurchaseFailed, "failure_reason": "provider_error"})
		s.tracker.Track(ctx, "credits.purchase_failed", map[string]any{"reason": "provider_error"})
		return nil, apperror.Unavailable(err)
	}
	switch res.Status {
	case payment.CheckoutSucceeded:
		if _, err := s.complete(ctx, p.ID, res.ProviderRef); err != nil {
			return nil, err
		}
	case payment.CheckoutFailed:
		_ = s.credits.UpdatePurchase(ctx, p.ID, map[string]any{"status": domain.PurchaseFailed, "failure_reason": res.FailureCode, "provider_ref": res.ProviderRef})
		s.tracker.Track(ctx, "credits.purchase_failed", map[string]any{"reason": res.FailureCode})
		return nil, (&apperror.Error{Code: apperror.CodePaymentFailed, Status: http.StatusPaymentRequired,
			Message: "Your payment was declined. No credits were added."}).WithDetails(map[string]any{"purchase_id": p.ID, "reason": res.FailureCode})
	case payment.CheckoutPending:
		_ = s.credits.UpdatePurchase(ctx, p.ID, map[string]any{"provider_ref": res.ProviderRef})
	}
	final, err := s.credits.GetPurchaseByKey(ctx, userID, in.IdempotencyKey)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return s.withWallet(ctx, final)
}

func (s *CreditService) withWallet(ctx context.Context, p *domain.CreditPurchase) (*PurchaseView, error) {
	v := purchaseView(p)
	if w, err := s.wallets.Get(ctx, p.UserID); err == nil {
		wv := walletView(w)
		v.Wallet = &wv
	}
	return &v, nil
}

// complete marks a purchase succeeded and credits the wallet exactly once.
func (s *CreditService) complete(ctx context.Context, purchaseID uuid.UUID, providerRef string) (bool, error) {
	var credited *domain.Wallet
	var purchase *domain.CreditPurchase
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		p, err := s.credits.GetPurchaseForUpdate(ctx, purchaseID)
		if err != nil {
			return err
		}
		if p.Status != domain.PurchasePending {
			purchase = p
			return nil
		}
		now := s.clock.Now()
		if err := s.credits.UpdatePurchase(ctx, p.ID, map[string]any{"status": domain.PurchaseSucceeded, "provider_ref": providerRef, "completed_at": now}); err != nil {
			return err
		}
		w, _, err := s.wallet.Apply(ctx, Movement{
			UserID: p.UserID, Type: domain.LedgerPurchase, Amount: p.Credits, ReferenceType: "credit_purchase",
			ReferenceID: &p.ID, IdempotencyKey: "purchase:" + p.ID.String(), Description: "Credit pack purchase",
		})
		if err != nil {
			return err
		}
		credited, purchase = w, p
		return nil
	})
	if err != nil {
		return false, mapRepoErr(err, "Purchase")
	}
	if credited != nil {
		ctx = reqctx.WithPrincipal(ctx, reqctx.Principal{UserID: purchase.UserID, Role: domain.RoleClient})
		warnErr(ctx, "credits: publish wallet", s.events.Publish(ctx, realtime.EventWalletUpdated, walletView(credited), purchase.UserID))
		s.tracker.Track(ctx, "credits.purchase_succeeded", map[string]any{
			"pack_id": purchase.PackID, "credits": purchase.Credits, "amount_cents": purchase.PriceCents, "currency": purchase.Currency,
		})
		return true, nil
	}
	return false, nil
}

// HandleWebhook processes a verified provider notification idempotently.
func (s *CreditService) HandleWebhook(ctx context.Context, signature string, body []byte) error {
	evt, err := s.provider.VerifyWebhook(signature, body, s.clock.Now())
	if err != nil {
		logger.FromContext(ctx).Warn("payment: webhook rejected", zap.Error(err))
		return apperror.New(apperror.CodeBadRequest, http.StatusBadRequest, "Invalid webhook signature.")
	}
	p, err := s.credits.GetPurchaseByProviderRef(ctx, s.provider.Name(), evt.ProviderRef)
	if err != nil && evt.PurchaseID != "" {
		if id, perr := uuid.Parse(evt.PurchaseID); perr == nil {
			var p2 *domain.CreditPurchase
			err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
				var e error
				p2, e = s.credits.GetPurchaseForUpdate(ctx, id)
				return e
			})
			p = p2
		}
	}
	if err != nil {
		logger.FromContext(ctx).Warn("payment: webhook for unknown purchase", zap.String("provider_ref", evt.ProviderRef))
		return nil // acknowledge so the provider stops retrying
	}
	switch evt.Type {
	case "payment.succeeded":
		_, err = s.complete(ctx, p.ID, evt.ProviderRef)
		return err
	case "payment.failed":
		if p.Status == domain.PurchasePending {
			return mapRepoErr(s.credits.UpdatePurchase(ctx, p.ID, map[string]any{"status": domain.PurchaseFailed, "failure_reason": evt.FailureCode}), "Purchase")
		}
		return nil
	case "payment.refunded":
		return s.refund(ctx, p.ID)
	}
	logger.FromContext(ctx).Info("payment: webhook type ignored", zap.String("type", evt.Type))
	return nil
}

// refund reverses a succeeded purchase, removing at most the credits still
// available (spent credits cannot be clawed back below zero).
func (s *CreditService) refund(ctx context.Context, purchaseID uuid.UUID) error {
	return mapRepoErr(s.tx.WithinTx(ctx, func(ctx context.Context) error {
		p, err := s.credits.GetPurchaseForUpdate(ctx, purchaseID)
		if err != nil {
			return err
		}
		if p.Status != domain.PurchaseSucceeded {
			return nil
		}
		w, err := s.wallets.GetForUpdate(ctx, p.UserID)
		if err != nil {
			return err
		}
		amount := min(p.Credits, w.Balance)
		if amount < p.Credits {
			logger.FromContext(ctx).Warn("payment: refund exceeds available balance; partial clawback",
				zap.String("purchase_id", p.ID.String()), zap.Int64("credits", p.Credits), zap.Int64("clawed_back", amount))
		}
		if amount > 0 {
			if _, _, err := s.wallet.Apply(ctx, Movement{UserID: p.UserID, Type: domain.LedgerRefund, Amount: -amount,
				ReferenceType: "credit_purchase", ReferenceID: &p.ID, IdempotencyKey: "purchase_refund:" + p.ID.String(),
				Description: "Payment refunded"}); err != nil {
				return err
			}
		}
		return s.credits.UpdatePurchase(ctx, p.ID, map[string]any{"status": domain.PurchaseRefunded})
	}), "Purchase")
}

// Purchases lists the caller's purchases.
func (s *CreditService) Purchases(ctx context.Context, userID uuid.UUID, cursor string, limit int) (Page[PurchaseView], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[PurchaseView]{}, err
	}
	rows, err := s.credits.ListPurchases(ctx, userID, c, limit)
	if err != nil {
		return Page[PurchaseView]{}, apperror.Internal(err)
	}
	page := Page[PurchaseView]{Items: make([]PurchaseView, 0, len(rows))}
	for i := range rows {
		page.Items = append(page.Items, purchaseView(&rows[i]))
	}
	if len(rows) > 0 && len(rows) == clampPage(limit, 20, 100) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return page, nil
}
