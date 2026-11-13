package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 5.0: refund requests (§24.4).

func (h *Handler) refunds() (*service.RefundService, error) {
	if h.svc.Refunds == nil {
		return nil, apperror.FeatureUnavailable("Refund requests are not available right now. Please contact support.")
	}
	return h.svc.Refunds, nil
}

// RefundQuote tells the caller what one of their payments can be refunded for.
func (h *Handler) RefundQuote(c echo.Context) error {
	s, err := h.refunds()
	if err != nil {
		return err
	}
	id, err := optionalUUID(c, "transaction_id")
	if err != nil {
		return err
	}
	if id == nil {
		return apperror.Validation(map[string]string{"transaction_id": "Choose a payment."})
	}
	out, err := s.Quote(c.Request().Context(), principal(c).UserID, *id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// CreateRefundRequest records a refund request (201; 200 on an idempotent replay).
func (h *Handler) CreateRefundRequest(c echo.Context) error {
	s, err := h.refunds()
	if err != nil {
		return err
	}
	var req service.CreateRefundInput
	if err := bind(c, &req); err != nil {
		return err
	}
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = c.Request().Header.Get("Idempotency-Key")
	}
	out, isNew, err := s.Create(c.Request().Context(), principal(c).UserID, req)
	if err != nil {
		return err
	}
	if isNew {
		return created(c, out)
	}
	return ok(c, out)
}

// MyRefundRequests lists the caller's refund requests, newest first.
func (h *Handler) MyRefundRequests(c echo.Context) error {
	s, err := h.refunds()
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 20, 100)
	if err != nil {
		return err
	}
	out, err := s.ListMine(c.Request().Context(), principal(c).UserID, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// WithdrawRefundRequest takes back an open request.
func (h *Handler) WithdrawRefundRequest(c echo.Context) error {
	s, err := h.refunds()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := s.Withdraw(c.Request().Context(), principal(c).UserID, id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminRefundRequests lists refund requests (status and user filters).
func (h *Handler) AdminRefundRequests(c echo.Context) error {
	s, err := h.refunds()
	if err != nil {
		return err
	}
	userID, err := optionalUUID(c, "user_id")
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 20, 100)
	if err != nil {
		return err
	}
	out, err := s.AdminList(c.Request().Context(), c.QueryParam("status"), userID, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminRefundRequest returns one request with the user's lots.
func (h *Handler) AdminRefundRequest(c echo.Context) error {
	s, err := h.refunds()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := s.AdminGet(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminCreateRefundRequest records a request received by e-mail (step-up).
func (h *Handler) AdminCreateRefundRequest(c echo.Context) error {
	s, err := h.refunds()
	if err != nil {
		return err
	}
	var req service.AdminCreateRefundInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := s.AdminCreate(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return created(c, out)
}

// AdminApproveRefundRequest refunds the payment and approves the request (step-up).
func (h *Handler) AdminApproveRefundRequest(c echo.Context) error {
	s, err := h.refunds()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req service.ApproveRefundInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := s.Approve(c.Request().Context(), principal(c).UserID, id, req)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminDenyRefundRequest denies the request (step-up).
func (h *Handler) AdminDenyRefundRequest(c echo.Context) error {
	s, err := h.refunds()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req service.DenyRefundInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := s.Deny(c.Request().Context(), principal(c).UserID, id, req)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminCreditRefundRequest answers the request with courtesy credits (step-up).
func (h *Handler) AdminCreditRefundRequest(c echo.Context) error {
	s, err := h.refunds()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req service.CreditRefundInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := s.Credit(c.Request().Context(), principal(c).UserID, id, req)
	if err != nil {
		return err
	}
	return ok(c, out)
}
