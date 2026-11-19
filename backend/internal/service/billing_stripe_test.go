package service

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// revisions 6.0 (§26.5): Stripe-facing helpers of billing.

// userStub serves one account (the embedded interface is nil: any other call panics).
type userStub struct {
	repository.UserRepository
	user *domain.User
}

func (u userStub) GetByID(context.Context, uuid.UUID) (*domain.User, error) {
	if u.user == nil {
		return nil, repository.ErrNotFound
	}
	return u.user, nil
}

func TestReturnURLUsesTheRequestLocaleThenTheAccountThenEnglish(t *testing.T) {
	purchase := uuid.New()
	build := func(front string, user *domain.User) *BillingService {
		return &BillingService{cfg: BillingSettings{FrontendURL: front}, users: userStub{user: user}}
	}
	withLocale := func(l string) context.Context { return reqctx.WithMeta(context.Background(), reqctx.Meta{Locale: l}) }
	want := func(locale string) string {
		return "https://staryield.net/" + locale + "/credits/return?purchase=" + purchase.String()
	}
	s := build("https://staryield.net", &domain.User{Locale: "es"})
	if got := s.returnURL(withLocale("fr"), uuid.New(), purchase); got != want("fr") {
		t.Errorf("request locale: %s", got)
	}
	if got := s.returnURL(context.Background(), uuid.New(), purchase); got != want("es") {
		t.Errorf("account locale: %s", got)
	}
	if got := s.returnURL(withLocale("de"), uuid.New(), purchase); got != want("es") {
		t.Errorf("an unsupported request locale falls back to the account's: %s", got)
	}
	s = build("https://staryield.net", &domain.User{Locale: "xx"})
	if got := s.returnURL(context.Background(), uuid.New(), purchase); got != want("en") {
		t.Errorf("default locale: %s", got)
	}
	s = build("https://staryield.net", nil)
	if got := s.returnURL(context.Background(), uuid.New(), purchase); got != want("en") {
		t.Errorf("unknown account: %s", got)
	}
	if got := build("", nil).returnURL(withLocale("en"), uuid.New(), purchase); got != "" {
		t.Errorf("no frontend URL: %q", got)
	}
}

func TestChargeMetadataCarriesOurIDs(t *testing.T) {
	tx := &domain.PaymentTransaction{ID: uuid.New(), UserID: uuid.New()}
	m := chargeMetadata(tx)
	if len(m) != 2 || m["sy_tx_id"] != tx.ID.String() || m["sy_user_id"] != tx.UserID.String() {
		t.Fatalf("membership charge: %v", m)
	}
	pid := uuid.New()
	tx.CreditPurchaseID = &pid
	if m = chargeMetadata(tx); m["sy_purchase_id"] != pid.String() || len(m) != 3 {
		t.Fatalf("purchase charge: %v", m)
	}
}

func TestProviderErrorIsA503WithoutTheCause(t *testing.T) {
	s := &BillingService{}
	for _, cause := range []error{payment.ErrConfiguration, payment.ErrUnknownOutcome, errors.New("boom")} {
		err := s.providerError(context.Background(), "test", cause)
		if err == nil {
			t.Fatal("a provider failure must be an error")
		}
		e := processorUnavailable(cause)
		if e.Status != http.StatusServiceUnavailable || e.Code != "provider_unavailable" || e.Err != cause || e.RetryAfter == 0 {
			t.Fatalf("processorUnavailable: %+v", e)
		}
		if e.Message == cause.Error() {
			t.Fatal("the cause must never be the customer's message")
		}
	}
}
