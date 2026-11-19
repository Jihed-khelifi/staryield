package service

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// revisions 6.0 (docs/platform-extensions.md §26.5): the parts of billing that
// talk to a real processor (Stripe): customers with the account identity,
// return URLs, charge metadata, the 3-D Secure policy and error mapping.

// codeProviderUnavailable is the 503 of a processor that did not answer
// (§26.9): the customer retries.
const codeProviderUnavailable apperror.Code = "provider_unavailable"

// processorUnavailable is the error the customer sees when the processor
// failed or refused our credentials; the cause is logged, never shown.
func processorUnavailable(cause error) *apperror.Error {
	return &apperror.Error{Code: codeProviderUnavailable, Status: http.StatusServiceUnavailable,
		Message: "The payment processor did not answer. Please try again in a moment.", Err: cause, RetryAfter: 5 * time.Second}
}

// providerError turns a provider call failure into the error the API returns.
// A rejected processor key is a configuration error (logged as such).
func (s *BillingService) providerError(ctx context.Context, op string, err error) error {
	if errors.Is(err, payment.ErrConfiguration) {
		logger.FromContext(ctx).Error("billing: the payment processor refused our credentials (configuration error)", zap.String("op", op), zap.Error(err))
	}
	return processorUnavailable(err)
}

// customerIdentity is the account e-mail and display name given to the
// provider's customer (fraud screening and dispute evidence). Best effort: a
// failed lookup sends none.
func (s *BillingService) customerIdentity(ctx context.Context, userID uuid.UUID) (email, name string) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		logger.FromContext(ctx).Warn("billing: account identity unavailable for the provider customer", zap.Error(err))
		return "", ""
	}
	return u.Email, u.DisplayName
}

// customerRefFor returns the provider customer a charge belongs to, or "" when
// the provider has no customer vault. A customer already linked locally is
// used as is (no provider round trip); the first purchase creates it.
func (s *BillingService) customerRefFor(ctx context.Context, userID uuid.UUID, p payment.Named) (string, error) {
	if _, ok := p.(payment.CustomerVault); !ok {
		return "", nil
	}
	if c, err := s.repo.GetCustomer(ctx, userID, p.Name()); err == nil {
		ref, err := s.sealer.Open(c.ProviderCustomerRefEnc, customerAD(c))
		if err != nil {
			return "", apperror.Internal(err)
		}
		return ref, nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return "", apperror.Internal(err)
	}
	_, ref, err := s.ensureCustomer(ctx, userID, p)
	return ref, err
}

// siteLocales are the locales the web app serves.
var siteLocales = map[string]bool{"en": true, "fr": true, "es": true}

// returnURL is where the browser returns after a redirect-based authentication
// step of a credit purchase: {frontend}/{locale}/credits/return?purchase=<id>.
// The locale is the request's, else the account's, else "en". Empty when the
// frontend URL is unknown.
func (s *BillingService) returnURL(ctx context.Context, userID, purchaseID uuid.UUID) string {
	if s.cfg.FrontendURL == "" {
		return ""
	}
	locale := reqctx.MetaFrom(ctx).Locale
	if !siteLocales[locale] {
		locale = "en"
		if u, err := s.users.GetByID(ctx, userID); err == nil && siteLocales[u.Locale] {
			locale = u.Locale
		}
	}
	return s.cfg.FrontendURL + "/" + locale + "/credits/return?purchase=" + url.QueryEscape(purchaseID.String())
}

// chargeMetadata tags a charge at the provider with our ids (the adapter adds
// the idempotency key): the ledger row, the user and, for a credit purchase,
// the purchase.
func chargeMetadata(t *domain.PaymentTransaction) map[string]string {
	m := map[string]string{"sy_tx_id": t.ID.String(), "sy_user_id": t.UserID.String()}
	if t.CreditPurchaseID != nil {
		m["sy_purchase_id"] = t.CreditPurchaseID.String()
	}
	return m
}

// recordPendingRef stores the provider reference of a charge whose outcome is
// still open on its ledger row, so webhooks and refunds find it by reference.
func (s *BillingService) recordPendingRef(ctx context.Context, t *domain.PaymentTransaction, ref string) {
	if ref == "" || (t.ProviderRef != nil && *t.ProviderRef != "") {
		return
	}
	warnErr(ctx, "billing: record provider reference", s.repo.UpdateTransaction(ctx, t.ID,
		map[string]any{"provider_ref": ref, "updated_at": s.clock.Now()}))
}
