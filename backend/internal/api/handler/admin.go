package handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// AdminUsers lists accounts.
func (h *Handler) AdminUsers(c echo.Context) error {
	limit, err := limitParam(c, 50, 200)
	if err != nil {
		return err
	}
	out, err := h.svc.Admin.Users(c.Request().Context(), c.QueryParam("q"), c.QueryParam("role"), c.QueryParam("status"), c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminUser returns one account.
func (h *Handler) AdminUser(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Admin.User(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminUpdateUser changes status/role.
func (h *Handler) AdminUpdateUser(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req service.UserUpdate
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Admin.UpdateUser(c.Request().Context(), principal(c).UserID, id, req)
	if err != nil {
		return err
	}
	return ok(c, out)
}

type adjustRequest struct {
	Amount         int64  `json:"amount"`
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotency_key"`
}

// AdminAdjustCredits applies a manual credit correction.
func (h *Handler) AdminAdjustCredits(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req adjustRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Admin.AdjustCredits(c.Request().Context(), principal(c).UserID, id, req.Amount, req.Reason, req.IdempotencyKey)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminPsychics lists advisors including unlisted ones.
func (h *Handler) AdminPsychics(c echo.Context) error {
	out, err := h.svc.Admin.Psychics(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"items": out})
}

// AdminCreatePsychic creates an advisor.
func (h *Handler) AdminCreatePsychic(c echo.Context) error {
	var req service.PsychicInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Admin.CreatePsychic(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return created(c, out)
}

// AdminUpdatePsychic edits an advisor.
func (h *Handler) AdminUpdatePsychic(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req service.PsychicInput
	if err := bind(c, &req); err != nil {
		return err
	}
	if req.Email != nil || req.Password != nil {
		return apperror.Validation(map[string]string{"email": "Credentials cannot be changed here."})
	}
	out, err := h.svc.Admin.UpdatePsychic(c.Request().Context(), id, req)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminPsychicEarnings shows one advisor's earnings.
func (h *Handler) AdminPsychicEarnings(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	return h.earningsFor(c, id)
}

// AdminCreateService adds a listed service.
func (h *Handler) AdminCreateService(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req service.ServiceInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Admin.CreateService(c.Request().Context(), id, req)
	if err != nil {
		return err
	}
	return created(c, out)
}

// AdminUpdateService edits a service.
func (h *Handler) AdminUpdateService(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req service.ServiceInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Admin.UpdateService(c.Request().Context(), id, req)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminPacks lists credit packs.
func (h *Handler) AdminPacks(c echo.Context) error {
	out, err := h.svc.Admin.Packs(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"items": out})
}

// AdminCreatePack adds a credit pack.
func (h *Handler) AdminCreatePack(c echo.Context) error {
	var req service.PackInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Admin.CreatePack(c.Request().Context(), req, h.defaultCurrency)
	if err != nil {
		return err
	}
	return created(c, out)
}

// AdminUpdatePack edits a credit pack.
func (h *Handler) AdminUpdatePack(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req service.PackInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Admin.UpdatePack(c.Request().Context(), id, req)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminRequests lists custom requests platform-wide.
func (h *Handler) AdminRequests(c echo.Context) error {
	statuses, err := service.ParseRequestStatuses(c.QueryParam("status"))
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 20, 100)
	if err != nil {
		return err
	}
	out, err := h.svc.Requests.AdminList(c.Request().Context(), statuses, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

type resolveRequest struct {
	Refund bool   `json:"refund"`
	Note   string `json:"note"`
}

// AdminResolveRequest refunds or upholds a request.
func (h *Handler) AdminResolveRequest(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req resolveRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Requests.Resolve(c.Request().Context(), principal(c).UserID, id, req.Refund, req.Note)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminSessions lists chat sessions.
func (h *Handler) AdminSessions(c echo.Context) error {
	statuses, err := service.ParseSessionStatuses(c.QueryParam("status"))
	if err != nil {
		return err
	}
	clientID, err := optionalUUID(c, "client_id")
	if err != nil {
		return err
	}
	psychicID, err := optionalUUID(c, "psychic_id")
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 20, 100)
	if err != nil {
		return err
	}
	out, err := h.svc.Admin.Sessions(c.Request().Context(), statuses, clientID, psychicID, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminSession returns one session.
func (h *Handler) AdminSession(c echo.Context) error {
	return h.Session(c)
}

// AdminTranscript returns a session's conversation (audited).
func (h *Handler) AdminTranscript(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 100, 200)
	if err != nil {
		return err
	}
	out, err := h.svc.Admin.Transcript(c.Request().Context(), id, c.QueryParam("reason"), c.QueryParam("before"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminTerminateSession ends a live session.
func (h *Handler) AdminTerminateSession(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req reasonRequest
	if c.Request().ContentLength != 0 {
		if err := bind(c, &req); err != nil {
			return err
		}
	}
	out, err := h.svc.Admin.TerminateSession(c.Request().Context(), id, req.Reason)
	if err != nil {
		return err
	}
	return ok(c, out)
}

func (h *Handler) reportRange(c echo.Context) (service.Range, error) {
	return service.ParseRange(c.QueryParam("from"), c.QueryParam("to"), time.Now().UTC())
}

// AdminOverview returns the headline business report (FR-14).
func (h *Handler) AdminOverview(c echo.Context) error {
	r, err := h.reportRange(c)
	if err != nil {
		return err
	}
	out, err := h.svc.Analytics.Overview(c.Request().Context(), r)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminDaily returns per-day metrics.
func (h *Handler) AdminDaily(c echo.Context) error {
	r, err := h.reportRange(c)
	if err != nil {
		return err
	}
	out, err := h.svc.Analytics.Daily(c.Request().Context(), r)
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"items": out})
}

// AdminTimeseries counts one event over time.
func (h *Handler) AdminTimeseries(c echo.Context) error {
	r, err := h.reportRange(c)
	if err != nil {
		return err
	}
	out, err := h.svc.Analytics.Timeseries(c.Request().Context(), c.QueryParam("name"), c.QueryParam("interval"), r)
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"items": out})
}

// AdminFunnel computes an ordered conversion funnel.
func (h *Handler) AdminFunnel(c echo.Context) error {
	r, err := h.reportRange(c)
	if err != nil {
		return err
	}
	var steps []string
	for _, s := range strings.Split(c.QueryParam("steps"), ",") {
		if s = strings.TrimSpace(s); s != "" {
			steps = append(steps, s)
		}
	}
	out, err := h.svc.Analytics.Funnel(c.Request().Context(), steps, r)
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"items": out})
}

// AdminTopEvents ranks events by volume.
func (h *Handler) AdminTopEvents(c echo.Context) error {
	r, err := h.reportRange(c)
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 50, 200)
	if err != nil {
		return err
	}
	out, err := h.svc.Analytics.TopEvents(c.Request().Context(), r, limit)
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"items": out})
}

// AdminEvents lists raw analytics events.
func (h *Handler) AdminEvents(c echo.Context) error {
	r, err := h.reportRange(c)
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 100, 1000)
	if err != nil {
		return err
	}
	userID, err := optionalUUID(c, "user_id")
	if err != nil {
		return err
	}
	var before int64
	if raw := c.QueryParam("before_id"); raw != "" {
		if before, err = strconv.ParseInt(raw, 10, 64); err != nil || before < 0 {
			return apperror.Validation(map[string]string{"before_id": "Invalid cursor."})
		}
	}
	out, err := h.svc.Analytics.Events(c.Request().Context(), repository.EventFilter{
		Name: c.QueryParam("name"), Source: c.QueryParam("source"), UserID: userID, From: r.From, To: r.To, AfterID: before, Limit: limit,
	})
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminAudit lists audit log entries.
func (h *Handler) AdminAudit(c echo.Context) error {
	actor, err := optionalUUID(c, "actor_id")
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 50, 200)
	if err != nil {
		return err
	}
	out, err := h.svc.Admin.Audit(c.Request().Context(), actor, c.QueryParam("target_type"), c.QueryParam("target_id"), c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

var _ = uuid.Nil
