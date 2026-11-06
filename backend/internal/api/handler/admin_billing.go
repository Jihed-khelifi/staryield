package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// AdminBillingPlans lists every membership plan with its versions.
func (h *Handler) AdminBillingPlans(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	out, err := b.AdminPlans(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"items": orEmpty(out)})
}

// AdminCreateBillingPlan creates a plan and its first version.
func (h *Handler) AdminCreateBillingPlan(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	var req service.PlanCreateInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := b.AdminCreatePlan(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return created(c, out)
}

// AdminUpdateBillingPlan edits a plan's presentation and visibility.
func (h *Handler) AdminUpdateBillingPlan(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req service.PlanUpdateInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := b.AdminUpdatePlan(c.Request().Context(), id, req)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminDeleteBillingPlan retires a plan without members.
func (h *Handler) AdminDeleteBillingPlan(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	if err := b.AdminDeletePlan(c.Request().Context(), id); err != nil {
		return err
	}
	return noContent(c)
}

// AdminCreateBillingPlanVersion adds an immutable price/benefit version.
func (h *Handler) AdminCreateBillingPlanVersion(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req service.PlanVersionInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := b.AdminCreatePlanVersion(c.Request().Context(), id, req)
	if err != nil {
		return err
	}
	return created(c, out)
}

// AdminRoutingRules lists payment routing rules.
func (h *Handler) AdminRoutingRules(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	out, err := b.RoutingRules(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"items": orEmpty(out)})
}

// AdminCreateRoutingRule adds a routing rule.
func (h *Handler) AdminCreateRoutingRule(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	var req service.RoutingRuleInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := b.CreateRoutingRule(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return created(c, out)
}

// AdminUpdateRoutingRule edits a routing rule.
func (h *Handler) AdminUpdateRoutingRule(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req service.RoutingRuleInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := b.UpdateRoutingRule(c.Request().Context(), id, req)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminDeleteRoutingRule removes a routing rule.
func (h *Handler) AdminDeleteRoutingRule(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	if err := b.DeleteRoutingRule(c.Request().Context(), id); err != nil {
		return err
	}
	return noContent(c)
}

// AdminSimulateRoute shows which providers a payment would use.
func (h *Handler) AdminSimulateRoute(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	var req service.SimulateRouteInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := b.SimulateRoute(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminPaymentProviders lists configured providers and capabilities.
func (h *Handler) AdminPaymentProviders(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"items": b.Providers()})
}

// AdminBillingSubscriptions lists memberships.
func (h *Handler) AdminBillingSubscriptions(c echo.Context) error {
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
	out, err := b.AdminSubscriptions(c.Request().Context(), userID, c.QueryParam("status"), c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminCancelBillingSubscription cancels a membership now or at period end.
func (h *Handler) AdminCancelBillingSubscription(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req service.AdminCancelInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := b.AdminCancelSubscription(c.Request().Context(), id, req)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminBillingTransactions lists money movements.
func (h *Handler) AdminBillingTransactions(c echo.Context) error {
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
	out, err := b.AdminTransactions(c.Request().Context(), userID, c.QueryParam("status"), c.QueryParam("kind"), c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminRefundTransaction refunds (part of) a payment.
func (h *Handler) AdminRefundTransaction(c echo.Context) error {
	b, err := h.billing()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req service.RefundInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := b.RefundTransaction(c.Request().Context(), id, req)
	if err != nil {
		return err
	}
	return ok(c, out)
}
