package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// SetCountryResolver installs how the trusted edge country of a request is
// read (e.g. Cloudflare's CF-IPCountry, honoured only from trusted proxies).
func (h *Handler) SetCountryResolver(fn func(*http.Request) string) { h.countryOf = fn }

func (h *Handler) trustedCountry(c echo.Context) string {
	if h.countryOf == nil {
		return ""
	}
	return h.countryOf(c.Request())
}

func locale(c echo.Context) string {
	if l := reqctx.MetaFrom(c.Request().Context()).Locale; l != "" {
		return l
	}
	return "en"
}

func (h *Handler) billing() (*service.BillingService, error) {
	if h.svc.Billing == nil {
		return nil, apperror.FeatureUnavailable("Memberships are not available yet.")
	}
	return h.svc.Billing, nil
}

// BillingPlans lists the memberships on sale (public).
func (h *Handler) BillingPlans(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	out, err := b.Plans(c.Request().Context(), locale(c))
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=300")
	c.Response().Header().Set("Vary", "X-Locale, Accept-Language")
	return ok(c, map[string]any{"items": orEmpty(out)})
}

// BillingClientConfig tells the browser which provider's card fields to load.
func (h *Handler) BillingClientConfig(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	planVersion, err := optionalUUID(c, "plan_version_id")
	if err != nil {
		return err
	}
	out, err := b.ClientConfig(c.Request().Context(), principal(c).UserID, service.ClientConfigInput{Purpose: c.QueryParam("purpose"),
		PlanVersionID: planVersion, TrustedCountry: h.trustedCountry(c)})
	if err != nil {
		return err
	}
	return ok(c, out)
}

// PaymentMethods lists saved cards.
func (h *Handler) PaymentMethods(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	out, err := b.PaymentMethods(c.Request().Context(), principal(c).UserID)
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"items": orEmpty(out)})
}

type attachMethodRequest struct {
	Provider       string `json:"provider"`
	Token          string `json:"token"`
	BillingCountry string `json:"billing_country"`
	MakeDefault    bool   `json:"make_default"`
	IdempotencyKey string `json:"idempotency_key"`
}

// AttachPaymentMethod saves a card tokenised by the provider (never raw
// card numbers: unknown fields such as "number" or "cvc" are rejected).
func (h *Handler) AttachPaymentMethod(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	var req attachMethodRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = c.Request().Header.Get("Idempotency-Key")
	}
	out, isNew, err := b.AttachPaymentMethod(c.Request().Context(), principal(c).UserID, service.AttachMethodInput{
		Provider: req.Provider, Token: req.Token, BillingCountry: req.BillingCountry, MakeDefault: req.MakeDefault,
		IdempotencyKey: req.IdempotencyKey, TrustedCountry: h.trustedCountry(c)})
	if err != nil {
		return err
	}
	if isNew {
		return created(c, out)
	}
	return ok(c, out)
}

// DeletePaymentMethod removes a saved card.
func (h *Handler) DeletePaymentMethod(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	if err := b.DeletePaymentMethod(c.Request().Context(), principal(c).UserID, id); err != nil {
		return err
	}
	return noContent(c)
}

// SetDefaultPaymentMethod makes a saved card the default.
func (h *Handler) SetDefaultPaymentMethod(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := b.SetDefaultPaymentMethod(c.Request().Context(), principal(c).UserID, id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// Subscription returns the caller's membership (null when none).
func (h *Handler) Subscription(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	out, err := b.Subscription(c.Request().Context(), principal(c).UserID, locale(c))
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"subscription": out})
}

type subscribeRequest struct {
	PlanVersionID   uuid.UUID `json:"plan_version_id"`
	PaymentMethodID uuid.UUID `json:"payment_method_id"`
	IdempotencyKey  string    `json:"idempotency_key"`
}

// Subscribe starts a membership charged to a saved card.
func (h *Handler) Subscribe(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	var req subscribeRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = c.Request().Header.Get("Idempotency-Key")
	}
	out, err := b.Subscribe(c.Request().Context(), principal(c).UserID, service.SubscribeInput{PlanVersionID: req.PlanVersionID,
		PaymentMethodID: req.PaymentMethodID, IdempotencyKey: req.IdempotencyKey, TrustedCountry: h.trustedCountry(c), Locale: locale(c)})
	if err != nil {
		return err
	}
	return created(c, out)
}

type cancelSubscriptionRequest struct {
	AtPeriodEnd *bool  `json:"at_period_end"`
	Reason      string `json:"reason"`
}

// CancelSubscription stops auto-renewal (an unpaid membership ends now).
func (h *Handler) CancelSubscription(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	var req cancelSubscriptionRequest
	if c.Request().ContentLength != 0 {
		if err := bind(c, &req); err != nil {
			return err
		}
	}
	if req.AtPeriodEnd != nil && !*req.AtPeriodEnd {
		return apperror.Validation(map[string]string{"at_period_end": "Memberships end at the end of the paid period."})
	}
	out, err := b.CancelSubscription(c.Request().Context(), principal(c).UserID, req.Reason, locale(c))
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"subscription": out})
}

// ResumeSubscription undoes a cancellation before the period ends.
func (h *Handler) ResumeSubscription(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	out, err := b.ResumeSubscription(c.Request().Context(), principal(c).UserID, locale(c))
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"subscription": out})
}

type updateSubscriptionRequest struct {
	PaymentMethodID uuid.UUID `json:"payment_method_id"`
}

// UpdateSubscription changes the membership's card (a past-due membership
// is charged right away).
func (h *Handler) UpdateSubscription(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	var req updateSubscriptionRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := b.UpdateSubscription(c.Request().Context(), principal(c).UserID, service.UpdateSubscriptionInput{
		PaymentMethodID: req.PaymentMethodID, TrustedCountry: h.trustedCountry(c), Locale: locale(c)})
	if err != nil {
		return err
	}
	return ok(c, out)
}

// Transactions lists the caller's payments.
func (h *Handler) Transactions(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 20, 100)
	if err != nil {
		return err
	}
	out, err := b.Transactions(c.Request().Context(), principal(c).UserID, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// ProviderWebhook receives signed notifications from a saved-card provider.
func (h *Handler) ProviderWebhook(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	body, err := readAll(c, 64*1024)
	if err != nil {
		return err
	}
	if err := b.HandleProviderWebhook(c.Request().Context(), c.Param("provider"), c.Request().Header, body); err != nil {
		return err
	}
	return ok(c, map[string]bool{"received": true})
}
