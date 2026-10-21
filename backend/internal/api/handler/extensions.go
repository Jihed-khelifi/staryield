package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// --- Notifications --------------------------------------------------------------

// Notifications lists the caller's notifications.
func (h *Handler) Notifications(c echo.Context) error {
	limit, err := limitParam(c, 20, 100)
	if err != nil {
		return err
	}
	out, err := h.svc.Notifications.List(c.Request().Context(), principal(c).UserID, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// UnreadNotifications returns the unread count.
func (h *Handler) UnreadNotifications(c echo.Context) error {
	n, err := h.svc.Notifications.UnreadCount(c.Request().Context(), principal(c).UserID)
	if err != nil {
		return err
	}
	return ok(c, map[string]int64{"count": n})
}

type markReadRequest struct {
	IDs []string `json:"ids"`
	All bool     `json:"all"`
}

// MarkNotificationsRead marks notifications as read.
func (h *Handler) MarkNotificationsRead(c echo.Context) error {
	var req markReadRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	if len(req.IDs) > 100 {
		return apperror.Validation(map[string]string{"ids": "Mark at most 100 notifications at once."})
	}
	ids := make([]uuid.UUID, 0, len(req.IDs))
	for _, raw := range req.IDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return apperror.Validation(map[string]string{"ids": "Invalid identifier."})
		}
		ids = append(ids, id)
	}
	n, err := h.svc.Notifications.MarkRead(c.Request().Context(), principal(c).UserID, ids, req.All)
	if err != nil {
		return err
	}
	return ok(c, map[string]int64{"unread": n})
}

// --- Reviews ---------------------------------------------------------------------

type reviewRequest struct {
	Rating  int    `json:"rating"`
	Comment string `json:"comment"`
}

// SubmitReview rates a finished session (client).
func (h *Handler) SubmitReview(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req reviewRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Reviews.Submit(c.Request().Context(), principal(c).UserID, id, req.Rating, req.Comment)
	if err != nil {
		return err
	}
	return created(c, out)
}

// PsychicReviews lists an advisor's public reviews.
func (h *Handler) PsychicReviews(c echo.Context) error {
	limit, err := limitParam(c, 10, 50)
	if err != nil {
		return err
	}
	out, err := h.svc.Reviews.Public(c.Request().Context(), c.Param("slug"), c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// MyReviews lists the signed-in advisor's reviews.
func (h *Handler) MyReviews(c echo.Context) error {
	limit, err := limitParam(c, 20, 100)
	if err != nil {
		return err
	}
	out, err := h.svc.Reviews.ForPsychic(c.Request().Context(), principal(c).UserID, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// --- Chat images and client profile -----------------------------------------------

// SendImage uploads a private image message.
func (h *Handler) SendImage(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	data, values, err := readUpload(c, "image", h.limits.ChatImageBytes)
	if err != nil {
		return err
	}
	out, err := h.svc.Chat.SendImage(c.Request().Context(), principal(c), service.UploadImageInput{
		SessionID: id, Data: data, Caption: values["caption"], ClientMsgID: values["client_msg_id"]})
	if err != nil {
		return err
	}
	return created(c, map[string]any{"message": out})
}

// Attachment streams a private chat image to a participant.
func (h *Handler) Attachment(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	body, ct, err := h.svc.Chat.Attachment(c.Request().Context(), principal(c), id)
	if err != nil {
		return err
	}
	hd := c.Response().Header()
	// Private to the participant's browser; never cached by shared caches.
	hd.Set("Cache-Control", "private, max-age=86400")
	hd.Del("Pragma")
	hd.Set("Content-Disposition", "inline")
	hd.Set("Cross-Origin-Resource-Policy", "same-site")
	hd.Set("Vary", "Cookie")
	return c.Blob(http.StatusOK, ct, body)
}

// ClientProfile returns the client card for the advisor of a conversation.
func (h *Handler) ClientProfile(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Chat.ClientProfile(c.Request().Context(), principal(c), id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// --- Psychic analytics and withdrawals -------------------------------------------------

// MyAnalytics returns the signed-in advisor's dashboard.
func (h *Handler) MyAnalytics(c echo.Context) error {
	return h.psychicAnalytics(c, principal(c).UserID)
}

func (h *Handler) psychicAnalytics(c echo.Context, id uuid.UUID) error {
	r, err := h.reportRange(c)
	if err != nil {
		return err
	}
	out, err := h.svc.PsychicAnalytics.Analytics(c.Request().Context(), id, r, c.QueryParam("tz"), c.QueryParam("interval"))
	if err != nil {
		return err
	}
	return ok(c, out)
}

// MyWithdrawals lists the advisor's payout requests.
func (h *Handler) MyWithdrawals(c echo.Context) error {
	limit, err := limitParam(c, 20, 100)
	if err != nil {
		return err
	}
	out, err := h.svc.Withdrawals.List(c.Request().Context(), principal(c).UserID, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// RequestWithdrawal creates a payout request.
func (h *Handler) RequestWithdrawal(c echo.Context) error {
	var req service.WithdrawalInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Withdrawals.Request(c.Request().Context(), principal(c).UserID, req)
	if err != nil {
		return err
	}
	return created(c, out)
}

// CancelWithdrawal cancels a pending payout request.
func (h *Handler) CancelWithdrawal(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Withdrawals.Cancel(c.Request().Context(), principal(c).UserID, id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// --- Short links -----------------------------------------------------------------------

// ShortLink records a marketing click and redirects to the landing page.
func (h *Handler) ShortLink(c echo.Context) error {
	r := c.Request()
	target := h.svc.Growth.Click(r.Context(), c.Param("code"), c.RealIP(), r.UserAgent(), r.Referer())
	hd := c.Response().Header()
	hd.Set("Cache-Control", "no-store")
	hd.Set("X-Robots-Tag", "noindex")
	return c.Redirect(http.StatusFound, target)
}
