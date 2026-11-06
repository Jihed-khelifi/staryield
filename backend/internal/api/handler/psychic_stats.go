package handler

import "github.com/labstack/echo/v4"

// MyStatsOverview returns the signed-in advisor's KPI cards
// (?from&to as RFC 3339 or YYYY-MM-DD in tz; default the last 30 days).
func (h *Handler) MyStatsOverview(c echo.Context) error {
	out, err := h.svc.PsychicStats.Overview(c.Request().Context(), principal(c).UserID,
		c.QueryParam("from"), c.QueryParam("to"), c.QueryParam("tz"))
	if err != nil {
		return err
	}
	return ok(c, out)
}
