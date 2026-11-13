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
	Code       string    `json:"code"` // revisions 5.0
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
	// NextAction asks the client to complete a provider step (3-D Secure).
	NextAction *payment.NextAction `json:"next_action,omitempty"`
}

// PurchaseInput starts a purchase. Without a payment method or token the
// legacy hosted checkout is used.
type PurchaseInput struct {
	PackID         uuid.UUID
	IdempotencyKey string
	ReceiptEmail   string
	// Billing: pay with a saved card, or a fresh provider token.
	PaymentMethodID *uuid.UUID
	PaymentToken    string
	Provider        string
	SaveMethod      bool
	TrustedCountry  string
	// Consent is the version of the Terms, Privacy Policy and Refund &
	// Cancellation Policy the buyer agreed to (revisions 5.1, §25.2). Required
	// for a new purchase; a replay of an earlier request does not need it.
	Consent *PurchaseConsent
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
	notify   Notifier
	billing  *BillingService
	offers   *OfferService // revisions 5.0: welcome offer (offers.go)
	// revisions 5.1: consent evidence, statement descriptor and receipts.
	settings CreditSettings
	contacts repository.ContactRepository
	mailer   EmailQueue
}

// CreditSettings are the credit service's configuration values (revisions 5.1).
type CreditSettings struct {
	// IPHashSecret keys the hash of the buyer's IP stored with the consent
	// (the analytics secret, as refund requests use).
	IPHashSecret string
	// StatementDescriptor is what the buyer's card statement shows (§25.3).
	StatementDescriptor string
	// FrontendURL is the site origin (links in the receipt copy for receipt_email).
	FrontendURL string
}

// WithSettings sets the consent, descriptor and receipt settings.
func (s *CreditService) WithSettings(c CreditSettings) *CreditService {
	s.settings = c
	return s
}

// WithReceiptMail enables the receipt e-mail to a purchase's receipt_email
// address (the account's own receipt goes through the notification channels).
func (s *CreditService) WithReceiptMail(contacts repository.ContactRepository, q EmailQueue) *CreditService {
	s.contacts, s.mailer = contacts, q
	return s
}

// WithNotifier enables purchase notifications.
func (s *CreditService) WithNotifier(n Notifier) *CreditService {
	s.notify = n
	return s
}

// WithBilling enables saved-card / token purchases and mirrors every
// purchase into the money ledger (payment_transactions).
func (s *CreditService) WithBilling(b *BillingService) *CreditService {
	s.billing = b
	return s
}

// settle mirrors a purchase's final state in the money ledger.
func (s *CreditService) settle(ctx context.Context, purchaseID uuid.UUID, status domain.TransactionStatus, providerRef, code string) {
	if s.billing == nil {
		return
	}
	warnErr(ctx, "credits: mirror purchase in ledger", s.billing.SettlePurchaseTransaction(ctx, purchaseID, status, providerRef, code))
}

// CompletePurchase credits a purchase whose charge succeeded asynchronously
// (billing reconciliation or webhook). Implements PurchaseSettler.
func (s *CreditService) CompletePurchase(ctx context.Context, purchaseID uuid.UUID, providerRef string) error {
	_, err := s.complete(ctx, purchaseID, providerRef)
	return err
}

// FailPurchase marks a pending purchase failed. Implements PurchaseSettler.
func (s *CreditService) FailPurchase(ctx context.Context, purchaseID uuid.UUID, code string) error {
	return mapRepoErr(s.tx.WithinTx(ctx, func(ctx context.Context) error {
		p, err := s.credits.GetPurchaseForUpdate(ctx, purchaseID)
		if err != nil {
			return err
		}
		if p.Status != domain.PurchasePending {
			return nil
		}
		// revisions 5.0: a failed welcome-pack purchase frees the offer.
		if err := s.offers.Release(ctx, p.ID); err != nil {
			return err
		}
		return s.credits.UpdatePurchase(ctx, p.ID, map[string]any{"status": domain.PurchaseFailed, "failure_reason": truncateString(code, 64)})
	}), "Purchase")
}

// NewCreditService wires the credit service.
func NewCreditService(tx database.TxManager, credits repository.CreditRepository, wallet *WalletService, wallets repository.WalletRepository,
	provider payment.Provider, events EventPublisher, tracker analytics.Tracker, clk clock.Clock) *CreditService {
	return &CreditService{tx: tx, credits: credits, wallet: wallet, wallets: wallets, provider: provider, events: events, tracker: tracker,
		clock: clk, notify: NoopNotifier{}}
}

func packView(p *domain.CreditPack) PackView {
	return PackView{ID: p.ID, Code: p.Code, Name: p.Name, Credits: p.Credits, PriceCents: p.PriceCents, Currency: p.Currency, Featured: p.Featured, Active: p.Active, SortOrder: p.SortOrder}
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
	pay := PurchasePayment{PaymentMethodID: in.PaymentMethodID, Token: strings.TrimSpace(in.PaymentToken), Provider: in.Provider,
		SaveMethod: in.SaveMethod, IdempotencyKey: in.IdempotencyKey, TrustedCountry: in.TrustedCountry}
	if pay.UsesBilling() {
		if s.billing == nil {
			return nil, apperror.FeatureUnavailable("Saved cards are not available yet.")
		}
	} else if _, off := s.provider.(payment.Disabled); off {
		return nil, apperror.FeatureUnavailable("Credit purchases are not available yet.")
	}
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
	// revisions 5.1: a new purchase needs the buyer's consent (§25.2); replays above do not.
	consent, err := s.validatePurchaseConsent(ctx, in.Consent, s.clock.Now())
	if err != nil {
		return nil, err
	}
	pack, err := s.credits.GetPack(ctx, in.PackID)
	if err == nil && !pack.Active && pack.Kind == domain.PackIntro { // revisions 5.0: a stale catalog still showing a withdrawn welcome pack
		return nil, apperror.OfferUnavailable(apperror.OfferReasonNoOffer)
	}
	if err != nil || !pack.Active {
		return nil, apperror.NotFound("Credit pack")
	}
	var src *PurchaseSource
	providerName := s.provider.Name()
	if pay.UsesBilling() {
		pay.Currency = pack.Currency
		if src, err = s.billing.ResolvePurchaseSource(ctx, userID, pay); err != nil {
			return nil, err
		}
		providerName = src.Provider
	}
	// revisions 5.0: welcome offer. Eligibility and the card's fingerprint are
	// checked before anything is created (offers.go, credits_offers.go).
	var offer *OfferPlan
	if pack.Kind == domain.PackIntro {
		if offer, err = s.planOffer(ctx, userID, pack, src); err != nil {
			return nil, err
		}
	}
	// end revisions 5.0
	now := s.clock.Now()
	p := &domain.CreditPurchase{
		ID: domain.NewID(), UserID: userID, PackID: pack.ID, Credits: pack.Credits, PriceCents: pack.PriceCents,
		Currency: pack.Currency, Status: domain.PurchasePending, Provider: providerName,
		IdempotencyKey: in.IdempotencyKey, ReceiptEmail: email, CreatedAt: now, UpdatedAt: now,
	}
	applyConsent(p, consent)                                // revisions 5.1
	if err := s.createPurchase(ctx, p, offer); err != nil { // revisions 5.0: reserves the welcome offer in the same transaction
		if _, ok := apperror.As(err); ok {
			return nil, err // offer_unavailable
		}
		if errors.Is(err, repository.ErrDuplicate) {
			if existing, err2 := s.credits.GetPurchaseByKey(ctx, userID, in.IdempotencyKey); err2 == nil {
				return s.withWallet(ctx, existing)
			}
		}
		return nil, apperror.Internal(err)
	}
	if s.billing != nil {
		if err := s.billing.RecordPurchaseTransaction(ctx, p, pack.Name, src); err != nil {
			_ = s.credits.UpdatePurchase(ctx, p.ID, map[string]any{"status": domain.PurchaseFailed, "failure_reason": "ledger_error"})
			s.releaseOffer(ctx, p.ID) // revisions 5.0
			return nil, apperror.Internal(err)
		}
	}
	s.tracker.Track(ctx, "credits.purchase_started", map[string]any{"pack_id": pack.ID, "credits": pack.Credits, "amount_cents": pack.PriceCents, "currency": pack.Currency})

	var res payment.CheckoutResult
	var next *payment.NextAction
	if src != nil {
		charge, err := s.billing.ChargePurchase(ctx, p, src)
		if err != nil {
			_ = s.credits.UpdatePurchase(ctx, p.ID, map[string]any{"status": domain.PurchaseFailed, "failure_reason": "provider_error"})
			s.releaseOffer(ctx, p.ID) // revisions 5.0
			s.settle(ctx, p.ID, domain.TxFailed, "", "provider_error")
			return nil, err
		}
		res = payment.CheckoutResult{Status: charge.Status, ProviderRef: charge.ProviderRef, FailureCode: charge.FailureCode}
		next = charge.NextAction
	} else {
		cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		res, err = s.provider.Checkout(cctx, s.checkoutRequest(p, pack.Name, email))
		cancel()
		if err != nil {
			logger.FromContext(ctx).Error("payment: checkout failed", zap.Error(err), zap.String("purchase_id", p.ID.String()))
			_ = s.credits.UpdatePurchase(ctx, p.ID, map[string]any{"status": domain.PurchaseFailed, "failure_reason": "provider_error"})
			s.releaseOffer(ctx, p.ID) // revisions 5.0
			s.settle(ctx, p.ID, domain.TxFailed, "", "provider_error")
			s.tracker.Track(ctx, "credits.purchase_failed", map[string]any{"reason": "provider_error"})
			return nil, apperror.Unavailable(err)
		}
	}
	switch res.Status {
	case payment.CheckoutSucceeded:
		if _, err := s.complete(ctx, p.ID, res.ProviderRef); err != nil {
			return nil, err
		}
	case payment.CheckoutFailed:
		upd := map[string]any{"status": domain.PurchaseFailed, "failure_reason": truncateString(res.FailureCode, 64)}
		if res.ProviderRef != "" {
			upd["provider_ref"] = res.ProviderRef
		}
		_ = s.credits.UpdatePurchase(ctx, p.ID, upd)
		s.releaseOffer(ctx, p.ID) // revisions 5.0
		s.settle(ctx, p.ID, domain.TxFailed, res.ProviderRef, res.FailureCode)
		s.tracker.Track(ctx, "credits.purchase_failed", map[string]any{"reason": res.FailureCode})
		return nil, (&apperror.Error{Code: apperror.CodePaymentFailed, Status: http.StatusPaymentRequired,
			Message: "Your payment was declined. No credits were added."}).WithDetails(map[string]any{"purchase_id": p.ID, "reason": res.FailureCode})
	case payment.CheckoutPending:
		if res.ProviderRef != "" {
			_ = s.credits.UpdatePurchase(ctx, p.ID, map[string]any{"provider_ref": res.ProviderRef})
		}
	}
	final, err := s.credits.GetPurchaseByKey(ctx, userID, in.IdempotencyKey)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	v, err := s.withWallet(ctx, final)
	if v != nil {
		v.NextAction = next
	}
	return v, err
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
		// revisions 5.0: the charge succeeded, so the welcome offer is redeemed.
		if err := s.offers.Redeem(ctx, p.ID); err != nil {
			return err
		}
		now := s.clock.Now()
		if err := s.credits.UpdatePurchase(ctx, p.ID, map[string]any{"status": domain.PurchaseSucceeded, "provider_ref": providerRef, "completed_at": now}); err != nil {
			return err
		}
		w, _, err := s.wallet.Apply(ctx, Movement{
			UserID: p.UserID, Type: domain.LedgerPurchase, Amount: p.Credits, ReferenceType: "credit_purchase",
			ReferenceID: &p.ID, IdempotencyKey: "purchase:" + p.ID.String(), Description: "Credit pack purchase",
			// revisions 5.0: credit lots. The lot is priced at what was paid.
			Lot: &LotSpec{Source: domain.LotPurchase, PaidCents: p.PriceCents, Currency: p.Currency},
		})
		if err != nil {
			return err
		}
		p.Status, p.CompletedAt = domain.PurchaseSucceeded, &now // revisions 5.1: the receipt shows the completion time
		credited, purchase = w, p
		return nil
	})
	if err != nil {
		return false, mapRepoErr(err, "Purchase")
	}
	if credited != nil || (purchase != nil && purchase.Status == domain.PurchaseSucceeded) {
		s.settle(ctx, purchaseID, domain.TxSucceeded, providerRef, "")
	}
	if credited != nil {
		ctx = reqctx.WithPrincipal(ctx, reqctx.Principal{UserID: purchase.UserID, Role: domain.RoleClient})
		warnErr(ctx, "credits: publish wallet", s.events.Publish(ctx, realtime.EventWalletUpdated, walletView(credited), purchase.UserID))
		s.tracker.Track(ctx, "credits.purchase_succeeded", map[string]any{
			"pack_id": purchase.PackID, "credits": purchase.Credits, "amount_cents": purchase.PriceCents, "currency": purchase.Currency,
		})
		// revisions 5.1: the receipt (§25.5): pack, amount, time, card and descriptor.
		data := s.receiptData(ctx, purchase)
		s.notify.Notify(ctx, NotificationSpec{Kind: NotifyPurchase, Data: data, Link: "/profile"}, purchase.UserID)
		s.mailReceiptCopy(ctx, purchase, data)
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
			if err := s.credits.UpdatePurchase(ctx, p.ID, map[string]any{"status": domain.PurchaseFailed, "failure_reason": evt.FailureCode}); err != nil {
				return mapRepoErr(err, "Purchase")
			}
			s.releaseOffer(ctx, p.ID) // revisions 5.0
			s.settle(ctx, p.ID, domain.TxFailed, evt.ProviderRef, evt.FailureCode)
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
	refunded := false
	var bought *domain.CreditPurchase // revisions 5.1
	err := mapRepoErr(s.tx.WithinTx(ctx, func(ctx context.Context) error {
		p, err := s.credits.GetPurchaseForUpdate(ctx, purchaseID)
		if err != nil {
			return err
		}
		if p.Status != domain.PurchaseSucceeded {
			return nil
		}
		// Lock the wallet first: the lot read below must not change before Apply.
		w, err := s.wallets.GetForUpdate(ctx, p.UserID)
		if err != nil {
			return err
		}
		// revisions 5.0: credit lots. A refund takes back what is left of this
		// purchase's own lot (the wallet is locked, so the lot cannot change
		// before Apply), never credits of other purchases or bonus credits.
		var lotID *uuid.UUID
		amount := int64(0)
		if lot, err := s.wallet.PurchaseLot(ctx, p.ID); err == nil {
			// The balance is authoritative: a lot claiming more than the wallet holds
			// (invariant broken, reported by the reconciliation job) is capped by it.
			amount, lotID = min(lot.CreditsRemaining, w.Balance), &lot.ID
		} else if !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		if amount < p.Credits {
			logger.FromContext(ctx).Warn("payment: refund exceeds the purchase's remaining credits; partial clawback",
				zap.String("purchase_id", p.ID.String()), zap.Int64("credits", p.Credits), zap.Int64("clawed_back", amount))
		}
		if amount > 0 {
			if _, _, err := s.wallet.Apply(ctx, Movement{UserID: p.UserID, Type: domain.LedgerRefund, Amount: -amount,
				ReferenceType: "credit_purchase", ReferenceID: &p.ID, IdempotencyKey: "purchase_refund:" + p.ID.String(),
				Description: "Payment refunded", ConsumeLot: lotID}); err != nil {
				return err
			}
		}
		refunded, bought = true, p
		return s.credits.UpdatePurchase(ctx, p.ID, map[string]any{"status": domain.PurchaseRefunded})
	}), "Purchase")
	if err == nil && refunded {
		s.settle(ctx, purchaseID, domain.TxRefunded, "", "")
		s.notifyRefunded(ctx, bought) // revisions 5.1 (§25.5)
	}
	return err
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

// checkoutRequest is the hosted-checkout charge of a purchase; it carries the
// statement descriptor like every other charge (revisions 5.1, §25.3).
func (s *CreditService) checkoutRequest(p *domain.CreditPurchase, description, email string) payment.CheckoutRequest {
	return payment.CheckoutRequest{PurchaseID: p.ID, UserID: p.UserID, AmountCents: p.PriceCents, Currency: p.Currency,
		Description: description, Email: email, StatementDescriptor: s.settings.StatementDescriptor}
}
