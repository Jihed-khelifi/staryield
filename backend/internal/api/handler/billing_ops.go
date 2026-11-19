package handler

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 6.0 (docs/platform-extensions.md §26.5): the purchase stream and
// the admin endpoints of card-payment operations.

// PurchaseStream serves GET /credits/purchases/:id/stream: the purchase's
// current state at once, then its updates until it is final (or 120 s).
func (h *Handler) PurchaseStream(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	p := principal(c)
	credits := h.svc.Credits
	// The first provider check happens now (it settles a payment the customer
	// already finished); the poll below makes the later ones.
	view, err := credits.SyncPurchase(c.Request().Context(), p.UserID, id, true)
	if err != nil {
		return err
	}
	cfg := credits.StreamSettings()
	spec := realtime.StreamSpec{
		Initial: &realtime.StreamEvent{Type: realtime.EventPurchaseUpdated, Data: view},
		Relay: func(eventType string, data json.RawMessage) (realtime.StreamEvent, bool) {
			if eventType != realtime.EventPurchaseUpdated {
				return realtime.StreamEvent{}, false
			}
			var env struct {
				Purchase json.RawMessage `json:"purchase"`
			}
			var head struct {
				ID uuid.UUID `json:"id"`
			}
			if json.Unmarshal(data, &env) != nil || json.Unmarshal(env.Purchase, &head) != nil || head.ID != id {
				return realtime.StreamEvent{}, false
			}
			return realtime.StreamEvent{Type: realtime.EventPurchaseUpdated, Data: env.Purchase}, true
		},
		Terminal: purchaseStreamTerminal,
		Poll: func(ctx context.Context, n int) (*realtime.StreamEvent, error) {
			v, err := credits.SyncPurchase(ctx, p.UserID, id, n < cfg.MaxChecks)
			if err != nil {
				return nil, err
			}
			return &realtime.StreamEvent{Type: realtime.EventPurchaseUpdated, Data: v}, nil
		},
		PollEvery:   cfg.PollEvery,
		MaxLifetime: cfg.MaxLifetime,
		Timeout:     &realtime.StreamEvent{Type: realtime.EventPurchaseTimeout},
	}
	h.realtime.ServeStream(c.Response(), c.Request(), p.UserID, string(p.Role), spec)
	return nil
}

// purchaseStreamTerminal ends the stream after a final purchase state or the timeout.
func purchaseStreamTerminal(ev realtime.StreamEvent) bool {
	switch ev.Type {
	case realtime.EventPurchaseTimeout:
		return true
	case realtime.EventPurchaseUpdated:
		switch d := ev.Data.(type) {
		case *service.PurchaseView:
			return service.FinalPurchaseStatus(d.Status)
		case json.RawMessage:
			var v struct {
				Status string `json:"status"`
			}
			return json.Unmarshal(d, &v) == nil && service.FinalPurchaseStatus(v.Status)
		}
	}
	return false
}

// requestOrigin is the public origin of this API as the request reached it.
func requestOrigin(c echo.Context) string {
	return c.Scheme() + "://" + c.Request().Host
}

// AdminProviderStatus serves GET /admin/billing/stripe/status. The optional
// provider query parameter (default stripe) selects another registered
// provider that can report its account (the sandboxes, in tests).
func (h *Handler) AdminProviderStatus(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	provider := c.QueryParam("provider")
	if provider == "" {
		provider = "stripe"
	}
	out, err := b.ProviderStatus(c.Request().Context(), provider, requestOrigin(c))
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return ok(c, out)
}

// AdminWebhookEvents lists the webhook log.
func (h *Handler) AdminWebhookEvents(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 20, 100)
	if err != nil {
		return err
	}
	out, err := b.WebhookEvents(c.Request().Context(), c.QueryParam("provider"), c.QueryParam("status"), c.QueryParam("type"),
		c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminReplayWebhook re-fetches an event from its provider and re-applies it.
func (h *Handler) AdminReplayWebhook(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := b.ReplayWebhook(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminDisputes lists disputes.
func (h *Handler) AdminDisputes(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 20, 100)
	if err != nil {
		return err
	}
	userID, err := optionalUUID(c, "user_id")
	if err != nil {
		return err
	}
	out, err := b.Disputes(c.Request().Context(), c.QueryParam("status"), userID, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminDispute returns a dispute with its payment, customer and evidence draft.
func (h *Handler) AdminDispute(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := b.Dispute(c.Request().Context(), id)
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return ok(c, out)
}

// AdminDisputeEvidence saves or submits (final) the evidence of a dispute.
func (h *Handler) AdminDisputeEvidence(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var in service.DisputeEvidenceInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := b.SubmitDisputeEvidence(c.Request().Context(), id, in)
	if err != nil {
		return err
	}
	return ok(c, out)
}

type noteRequest struct {
	Note string `json:"note"`
}

// AdminDisputeAccept concedes a dispute.
func (h *Handler) AdminDisputeAccept(c echo.Context) error {
	return h.adminNoted(c, func(b *service.BillingService, c echo.Context, id uuid.UUID, note string) (any, error) {
		return b.AcceptDispute(c.Request().Context(), id, note)
	})
}

// AdminFraudWarnings lists early fraud warnings.
func (h *Handler) AdminFraudWarnings(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 20, 100)
	if err != nil {
		return err
	}
	out, err := b.FraudWarnings(c.Request().Context(), c.QueryParam("status"), c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminFraudWarningRefund refunds the warned payment.
func (h *Handler) AdminFraudWarningRefund(c echo.Context) error {
	return h.adminNoted(c, func(b *service.BillingService, c echo.Context, id uuid.UUID, note string) (any, error) {
		return b.RefundFraudWarning(c.Request().Context(), id, note)
	})
}

// AdminFraudWarningDismiss dismisses a warning.
func (h *Handler) AdminFraudWarningDismiss(c echo.Context) error {
	return h.adminNoted(c, func(b *service.BillingService, c echo.Context, id uuid.UUID, note string) (any, error) {
		return b.DismissFraudWarning(c.Request().Context(), id, note)
	})
}

// adminNoted runs an admin action addressed by :id that takes a {note} body.
func (h *Handler) adminNoted(c echo.Context, run func(*service.BillingService, echo.Context, uuid.UUID, string) (any, error)) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var in noteRequest
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := run(b, c, id, in.Note)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminPaymentPolicies returns the payment policies.
func (h *Handler) AdminPaymentPolicies(c echo.Context) error {
	out, err := h.svc.Site.PaymentPoliciesSetting(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminUpdatePaymentPolicies replaces the payment policies.
func (h *Handler) AdminUpdatePaymentPolicies(c echo.Context) error {
	var in service.PaymentPoliciesInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Site.UpdatePaymentPolicies(c.Request().Context(), in)
	if err != nil {
		return err
	}
	return ok(c, out)
}
