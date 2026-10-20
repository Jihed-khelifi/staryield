package handler

import (
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

type orderRequest struct {
	ServiceID      uuid.UUID  `json:"service_id"`
	Note           string     `json:"note"`
	PreferredAt    *time.Time `json:"preferred_at"`
	IdempotencyKey string     `json:"idempotency_key"`
}

// OrderRequest places a custom request (FR-07).
func (h *Handler) OrderRequest(c echo.Context) error {
	var req orderRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	if req.ServiceID == uuid.Nil {
		return apperror.Validation(map[string]string{"service_id": "Choose a service."})
	}
	out, err := h.svc.Requests.Order(c.Request().Context(), principal(c).UserID, service.OrderInput{
		ServiceID: req.ServiceID, Note: req.Note, PreferredAt: req.PreferredAt, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return err
	}
	return created(c, out)
}

// ListRequests lists the caller's custom requests.
func (h *Handler) ListRequests(c echo.Context) error {
	statuses, err := service.ParseRequestStatuses(c.QueryParam("status"))
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 20, 100)
	if err != nil {
		return err
	}
	out, err := h.svc.Requests.List(c.Request().Context(), principal(c), statuses, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// GetRequest returns one custom request.
func (h *Handler) GetRequest(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Requests.Get(c.Request().Context(), principal(c), id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

func (h *Handler) requestAction(c echo.Context, fn func(id uuid.UUID) (*service.CustomRequestView, error)) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := fn(id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// CancelRequest is the client cancelling.
func (h *Handler) CancelRequest(c echo.Context) error {
	return h.requestAction(c, func(id uuid.UUID) (*service.CustomRequestView, error) {
		return h.svc.Requests.Cancel(c.Request().Context(), principal(c).UserID, id)
	})
}

// AcceptRequest is the psychic accepting.
func (h *Handler) AcceptRequest(c echo.Context) error {
	return h.requestAction(c, func(id uuid.UUID) (*service.CustomRequestView, error) {
		return h.svc.Requests.Accept(c.Request().Context(), principal(c).UserID, id)
	})
}

type reasonRequest struct {
	Reason string `json:"reason"`
}

// DeclineRequest is the psychic declining.
func (h *Handler) DeclineRequest(c echo.Context) error {
	var req reasonRequest
	if c.Request().ContentLength != 0 {
		if err := bind(c, &req); err != nil {
			return err
		}
	}
	return h.requestAction(c, func(id uuid.UUID) (*service.CustomRequestView, error) {
		return h.svc.Requests.Decline(c.Request().Context(), principal(c).UserID, id, req.Reason)
	})
}

type deliverRequest struct {
	Delivery string `json:"delivery"`
}

// DeliverRequest is the psychic fulfilling.
func (h *Handler) DeliverRequest(c echo.Context) error {
	var req deliverRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	return h.requestAction(c, func(id uuid.UUID) (*service.CustomRequestView, error) {
		return h.svc.Requests.Deliver(c.Request().Context(), principal(c).UserID, id, req.Delivery)
	})
}

// DisputeRequest is the client disputing a delivery.
func (h *Handler) DisputeRequest(c echo.Context) error {
	var req reasonRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	return h.requestAction(c, func(id uuid.UUID) (*service.CustomRequestView, error) {
		return h.svc.Requests.Dispute(c.Request().Context(), principal(c).UserID, id, req.Reason)
	})
}

// readAll reads a bounded raw body.
func readAll(c echo.Context, max int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, max+1))
	if err != nil {
		return nil, decodeError(err)
	}
	if int64(len(body)) > max {
		return nil, apperror.PayloadTooLarge("The request body is too large.")
	}
	return body, nil
}
