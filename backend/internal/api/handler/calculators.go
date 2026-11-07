package handler

import (
	"net/url"

	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 3.0: calculator analytics

func calculatorQuery(c echo.Context) (service.CalculatorQuery, error) {
	q := service.CalculatorQuery{From: c.QueryParam("from"), To: c.QueryParam("to"), Interval: c.QueryParam("interval"),
		Kind: c.QueryParam("kind"), Locale: c.QueryParam("locale"), Source: c.QueryParam("source"), Cursor: c.QueryParam("cursor")}
	q.SourceSet = hasQueryParam(c.QueryParams(), "source")
	var err error
	if q.CampaignID, err = optionalUUID(c, "campaign_id"); err != nil {
		return q, err
	}
	if q.LinkID, err = optionalUUID(c, "link_id"); err != nil {
		return q, err
	}
	return q, nil
}

func hasQueryParam(v url.Values, name string) bool {
	_, ok := v[name]
	return ok
}

// AdminCalculators returns the calculator usage report.
func (h *Handler) AdminCalculators(c echo.Context) error {
	q, err := calculatorQuery(c)
	if err != nil {
		return err
	}
	out, err := h.svc.Calculators.Report(c.Request().Context(), q)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminCalculatorConversions lists sign-ups of calculator visitors.
func (h *Handler) AdminCalculatorConversions(c echo.Context) error {
	q, err := calculatorQuery(c)
	if err != nil {
		return err
	}
	if q.Limit, err = limitParam(c, 25, 100); err != nil {
		return err
	}
	out, err := h.svc.Calculators.Conversions(c.Request().Context(), q)
	if err != nil {
		return err
	}
	return ok(c, out)
}
