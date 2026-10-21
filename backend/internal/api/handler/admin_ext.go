package handler

import (
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// AdminCreateUser creates a client or administrator.
func (h *Handler) AdminCreateUser(c echo.Context) error {
	var req service.CreateUserInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Admin.CreateUser(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return created(c, out)
}

// AdminDeleteUser soft-deletes and anonymises an account.
func (h *Handler) AdminDeleteUser(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req reasonRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	if err := h.svc.Admin.DeleteUser(c.Request().Context(), principal(c).UserID, id, req.Reason); err != nil {
		return err
	}
	return noContent(c)
}

// AdminLogoutUser revokes all main-site sessions of a user.
func (h *Handler) AdminLogoutUser(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	if err := h.svc.Admin.LogoutEverywhere(c.Request().Context(), id); err != nil {
		return err
	}
	return noContent(c)
}

// AdminPsychic returns one advisor.
func (h *Handler) AdminPsychic(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Admin.Psychic(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminDeletePsychic soft-deletes an advisor.
func (h *Handler) AdminDeletePsychic(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req reasonRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	if err := h.svc.Admin.DeletePsychic(c.Request().Context(), principal(c).UserID, id, req.Reason); err != nil {
		return err
	}
	return noContent(c)
}

// AdminUploadPsychicImage stores a new advisor portrait.
func (h *Handler) AdminUploadPsychicImage(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	data, _, err := readUpload(c, "image", h.limits.AdvisorImageBytes)
	if err != nil {
		return err
	}
	out, err := h.svc.Admin.UploadPsychicImage(c.Request().Context(), id, data)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminDeletePsychicImage removes an advisor portrait.
func (h *Handler) AdminDeletePsychicImage(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Admin.DeletePsychicImage(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminPsychicAnalytics shows an advisor's dashboard.
func (h *Handler) AdminPsychicAnalytics(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	return h.psychicAnalytics(c, id)
}

// AdminDeleteService deletes an unused service.
func (h *Handler) AdminDeleteService(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	if err := h.svc.Admin.DeleteService(c.Request().Context(), id); err != nil {
		return err
	}
	return noContent(c)
}

// AdminDeletePack deletes an unused credit pack.
func (h *Handler) AdminDeletePack(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	if err := h.svc.Admin.DeletePack(c.Request().Context(), id); err != nil {
		return err
	}
	return noContent(c)
}

// AdminPresence lists connected users.
func (h *Handler) AdminPresence(c echo.Context) error {
	role := c.QueryParam("role")
	if role != "" && !domain.Role(role).Valid() {
		return apperror.Validation(map[string]string{"role": "Unknown role."})
	}
	out, err := h.svc.Admin.Presence(c.Request().Context(), role)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// --- Reviews moderation ---------------------------------------------------------

// AdminReviews lists reviews for moderation.
func (h *Handler) AdminReviews(c echo.Context) error {
	psychicID, err := optionalUUID(c, "psychic_id")
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 50, 200)
	if err != nil {
		return err
	}
	rating := 0
	if raw := c.QueryParam("rating"); raw != "" {
		if rating, err = strconv.Atoi(raw); err != nil || rating < 1 || rating > 5 {
			return apperror.Validation(map[string]string{"rating": "Use a number between 1 and 5."})
		}
	}
	out, err := h.svc.Reviews.AdminList(c.Request().Context(), repository.ReviewFilter{PsychicID: psychicID,
		Status: domain.ReviewStatus(c.QueryParam("status")), Rating: rating, Limit: limit}, c.QueryParam("cursor"))
	if err != nil {
		return err
	}
	return ok(c, out)
}

type moderateRequest struct {
	Status domain.ReviewStatus `json:"status"`
	Note   string              `json:"note"`
}

// AdminModerateReview hides or restores a review.
func (h *Handler) AdminModerateReview(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req moderateRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Reviews.Moderate(c.Request().Context(), id, req.Status, req.Note)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// --- Withdrawals ------------------------------------------------------------------

// AdminWithdrawals lists payout requests.
func (h *Handler) AdminWithdrawals(c echo.Context) error {
	statuses, err := service.ParseWithdrawalStatuses(c.QueryParam("status"))
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
	out, err := h.svc.Withdrawals.AdminList(c.Request().Context(), statuses, psychicID, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminWithdrawalDestination reveals payout details (audited).
func (h *Handler) AdminWithdrawalDestination(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	dest, err := h.svc.Withdrawals.Destination(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return ok(c, map[string]string{"destination": dest})
}

type withdrawalActionRequest struct {
	Note      string `json:"note"`
	Reference string `json:"reference"`
}

func (h *Handler) withdrawalTransition(c echo.Context, to domain.WithdrawalStatus) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req withdrawalActionRequest
	if c.Request().ContentLength != 0 {
		if err := bind(c, &req); err != nil {
			return err
		}
	}
	out, err := h.svc.Withdrawals.Transition(c.Request().Context(), principal(c).UserID, id, to, req.Note, req.Reference)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminApproveWithdrawal approves a pending withdrawal.
func (h *Handler) AdminApproveWithdrawal(c echo.Context) error {
	return h.withdrawalTransition(c, domain.WithdrawalApproved)
}

// AdminPayWithdrawal marks an approved withdrawal as paid.
func (h *Handler) AdminPayWithdrawal(c echo.Context) error {
	return h.withdrawalTransition(c, domain.WithdrawalPaid)
}

// AdminRejectWithdrawal rejects a withdrawal.
func (h *Handler) AdminRejectWithdrawal(c echo.Context) error {
	return h.withdrawalTransition(c, domain.WithdrawalRejected)
}

// --- Broadcasts ----------------------------------------------------------------------

// AdminBroadcast queues a broadcast (202).
func (h *Handler) AdminBroadcast(c echo.Context) error {
	var req service.BroadcastInput
	if err := bind(c, &req); err != nil {
		return err
	}
	ctx := c.Request().Context()
	if req.Audience == "all" {
		if err := h.svc.AdminAuth.RequireStepUp(ctx); err != nil {
			return err
		}
	}
	out, err := h.svc.Notifications.Broadcast(ctx, principal(c).UserID, req)
	if err != nil {
		return err
	}
	return c.JSON(202, out)
}

// AdminBroadcasts lists broadcasts.
func (h *Handler) AdminBroadcasts(c echo.Context) error {
	limit, err := limitParam(c, 20, 100)
	if err != nil {
		return err
	}
	out, err := h.svc.Notifications.Broadcasts(c.Request().Context(), c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminBroadcastDetail returns one broadcast.
func (h *Handler) AdminBroadcastDetail(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Notifications.GetBroadcast(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// --- Campaigns and links ---------------------------------------------------------------

// AdminCampaigns lists campaigns.
func (h *Handler) AdminCampaigns(c echo.Context) error {
	out, err := h.svc.Growth.Campaigns(c.Request().Context(), c.QueryParam("status"))
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"items": orEmpty(out)})
}

// AdminCreateCampaign adds a campaign.
func (h *Handler) AdminCreateCampaign(c echo.Context) error {
	var req service.CampaignInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Growth.CreateCampaign(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return created(c, out)
}

// AdminCampaign returns one campaign.
func (h *Handler) AdminCampaign(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Growth.Campaign(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminUpdateCampaign edits a campaign.
func (h *Handler) AdminUpdateCampaign(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req service.CampaignInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Growth.UpdateCampaign(c.Request().Context(), id, req)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminDeleteCampaign deletes or archives a campaign.
func (h *Handler) AdminDeleteCampaign(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	if err := h.svc.Growth.DeleteCampaign(c.Request().Context(), id); err != nil {
		return err
	}
	return noContent(c)
}

// AdminCampaignReport returns a campaign's funnel.
func (h *Handler) AdminCampaignReport(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	r, err := h.reportRange(c)
	if err != nil {
		return err
	}
	out, err := h.svc.Growth.CampaignReport(c.Request().Context(), id, r, c.QueryParam("interval"))
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminLinks lists short links.
func (h *Handler) AdminLinks(c echo.Context) error {
	campaignID, err := optionalUUID(c, "campaign_id")
	if err != nil {
		return err
	}
	active, err := optionalBool(c, "active")
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 50, 200)
	if err != nil {
		return err
	}
	out, err := h.svc.Growth.Links(c.Request().Context(), repository.LinkFilter{CampaignID: campaignID, Query: c.QueryParam("q"),
		Active: active, Limit: limit}, c.QueryParam("cursor"))
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminLinkSettings returns how short links are built.
func (h *Handler) AdminLinkSettings(c echo.Context) error {
	return ok(c, h.svc.Growth.Settings())
}

// AdminCreateLink adds a short link.
func (h *Handler) AdminCreateLink(c echo.Context) error {
	var req service.LinkInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Growth.CreateLink(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return created(c, out)
}

// AdminLink returns one short link.
func (h *Handler) AdminLink(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Growth.Link(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminUpdateLink edits a short link.
func (h *Handler) AdminUpdateLink(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req service.LinkInput
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Growth.UpdateLink(c.Request().Context(), id, req)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminDeleteLink deletes or deactivates a short link.
func (h *Handler) AdminDeleteLink(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	if err := h.svc.Growth.DeleteLink(c.Request().Context(), id); err != nil {
		return err
	}
	return noContent(c)
}

// AdminLinkReport returns a link's funnel.
func (h *Handler) AdminLinkReport(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	r, err := h.reportRange(c)
	if err != nil {
		return err
	}
	out, err := h.svc.Growth.LinkReport(c.Request().Context(), id, r, c.QueryParam("interval"))
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminAcquisition returns the platform acquisition overview.
func (h *Handler) AdminAcquisition(c echo.Context) error {
	r, err := h.reportRange(c)
	if err != nil {
		return err
	}
	out, err := h.svc.Growth.Acquisition(c.Request().Context(), r, c.QueryParam("interval"))
	if err != nil {
		return err
	}
	return ok(c, out)
}

// --- Insights ----------------------------------------------------------------------------

// AdminAudience returns audience breakdowns.
func (h *Handler) AdminAudience(c echo.Context) error {
	r, err := h.reportRange(c)
	if err != nil {
		return err
	}
	out, err := h.svc.Insights.Audience(c.Request().Context(), r)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminEngagement returns engagement metrics.
func (h *Handler) AdminEngagement(c echo.Context) error {
	r, err := h.reportRange(c)
	if err != nil {
		return err
	}
	out, err := h.svc.Insights.Engagement(c.Request().Context(), r, c.QueryParam("interval"))
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminRetention returns weekly cohort retention.
func (h *Handler) AdminRetention(c echo.Context) error {
	weeks := 8
	if raw := c.QueryParam("weeks"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return apperror.Validation(map[string]string{"weeks": "Use a positive number of weeks."})
		}
		weeks = n
	}
	out, err := h.svc.Insights.Retention(c.Request().Context(), weeks)
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"cohorts": out})
}

// AdminTopUsers ranks clients.
func (h *Handler) AdminTopUsers(c echo.Context) error {
	r, err := h.reportRange(c)
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 20, 100)
	if err != nil {
		return err
	}
	out, err := h.svc.Insights.TopUsers(c.Request().Context(), c.QueryParam("metric"), r, limit)
	if err != nil {
		return err
	}
	return ok(c, map[string]any{"items": orEmpty(out)})
}

// AdminUserInsights returns one account's drill-down.
func (h *Handler) AdminUserInsights(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Insights.User(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return ok(c, out)
}
