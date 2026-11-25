package handler

import (
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// Growth: the referral loop.

func (h *Handler) referrals() (*service.ReferralService, error) {
	if h.svc.Referrals == nil {
		return nil, apperror.FeatureUnavailable("Referrals are not available right now.")
	}
	return h.svc.Referrals, nil
}

// MyReferrals returns the caller's invite code and link, the program terms and
// counters (GET /me/referrals). Counts only: never who joined.
func (h *Handler) MyReferrals(c echo.Context) error {
	s, err := h.referrals()
	if err != nil {
		return err
	}
	out, err := s.Mine(c.Request().Context(), principal(c).UserID)
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "private, no-store")
	return ok(c, out)
}

// AdminReferrals lists referrals (status and flagged filters, keyset pages).
func (h *Handler) AdminReferrals(c echo.Context) error {
	s, err := h.referrals()
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 20, 100)
	if err != nil {
		return err
	}
	var flagged *bool
	if raw := c.QueryParam("flagged"); raw != "" {
		v, perr := strconv.ParseBool(raw)
		if perr != nil {
			return apperror.Validation(map[string]string{"flagged": "Use true or false."})
		}
		flagged = &v
	}
	out, err := s.List(c.Request().Context(), c.QueryParam("status"), flagged, c.QueryParam("cursor"), limit)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminReferralSummary returns the program's counters and top inviters.
func (h *Handler) AdminReferralSummary(c echo.Context) error {
	s, err := h.referrals()
	if err != nil {
		return err
	}
	out, err := s.Summary(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminApproveReferral releases a referral held for review.
func (h *Handler) AdminApproveReferral(c echo.Context) error {
	s, err := h.referrals()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	out, err := s.Approve(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminRejectReferral closes a referral with a reason.
func (h *Handler) AdminRejectReferral(c echo.Context) error {
	s, err := h.referrals()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := s.Reject(c.Request().Context(), id, req.Reason)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminLaunchReferrals launches the rewards (once; typed confirmation).
func (h *Handler) AdminLaunchReferrals(c echo.Context) error {
	s, err := h.referrals()
	if err != nil {
		return err
	}
	var req struct {
		Confirmation string `json:"confirmation"`
	}
	if err := bind(c, &req); err != nil {
		return err
	}
	out, err := s.Launch(c.Request().Context(), req.Confirmation)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminReferralProgram returns the program setting.
func (h *Handler) AdminReferralProgram(c echo.Context) error {
	out, err := h.svc.Site.ReferralProgramSetting(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminUpdateReferralProgram replaces the program setting.
func (h *Handler) AdminUpdateReferralProgram(c echo.Context) error {
	var in service.ReferralProgramInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Site.UpdateReferralProgram(c.Request().Context(), in)
	if err != nil {
		return err
	}
	return ok(c, out)
}
