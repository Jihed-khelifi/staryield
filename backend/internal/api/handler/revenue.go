package handler

import (
	"errors"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// AdminRevenue returns the revenue report (revisions 5.0, §24.5): at most 366
// days, interval day (default), week or month.
func (h *Handler) AdminRevenue(c echo.Context) error {
	if h.svc.Revenue == nil {
		return apperror.Internal(errors.New("handler: revenue service not wired"))
	}
	r, interval, err := service.ParseRevenueRange(c.QueryParam("from"), c.QueryParam("to"), c.QueryParam("interval"), time.Now().UTC())
	if err != nil {
		return err
	}
	out, err := h.svc.Revenue.Report(c.Request().Context(), r, interval)
	if err != nil {
		return err
	}
	return ok(c, out)
}
