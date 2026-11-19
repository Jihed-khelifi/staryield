package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 6.0 (§26.4): the calculators' e-mail report.

// CalculatorReportConfig tells the dialog whether the report is on and whether
// its marketing box starts ticked for the request's trusted country.
func (h *Handler) CalculatorReportConfig(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "private, no-store")
	return ok(c, h.svc.CalculatorReport.Config(c.Request().Context(), h.trustedCountry(c)))
}

// RequestReportEmail queues the e-mailed report (public, optionally signed in).
func (h *Handler) RequestReportEmail(c echo.Context) error {
	var in service.ReportEmailInput
	if err := bind(c, &in); err != nil {
		return err
	}
	in.Country = h.trustedCountry(c)
	if err := h.svc.CalculatorReport.Request(c.Request().Context(), in); err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "private, no-store")
	return c.JSON(http.StatusAccepted, map[string]string{"status": "queued"})
}

// MarketingUnsubscribe applies the unsubscribe link of a marketing e-mail.
func (h *Handler) MarketingUnsubscribe(c echo.Context) error {
	var in struct {
		Token string `json:"token"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	if err := h.svc.CalculatorReport.Unsubscribe(c.Request().Context(), in.Token); err != nil {
		return err
	}
	return ok(c, map[string]string{"status": "unsubscribed"})
}

// AdminMarketingLeads lists the e-mail leads, newest first.
func (h *Handler) AdminMarketingLeads(c echo.Context) error {
	limit, err := limitParam(c, 25, 100)
	if err != nil {
		return err
	}
	out, err := h.svc.CalculatorReport.Leads(c.Request().Context(), service.LeadQuery{Status: c.QueryParam("status"),
		Kind: c.QueryParam("kind"), Q: c.QueryParam("q"), Cursor: c.QueryParam("cursor"), Limit: limit})
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminExportMarketingLeads streams the subscribed leads (or another status)
// as CSV for the marketing provider (step-up, audited with the row count).
func (h *Handler) AdminExportMarketingLeads(c echo.Context) error {
	f, err := h.svc.CalculatorReport.LeadExportFilter(c.QueryParam("status"), c.QueryParam("kind"))
	if err != nil {
		return err
	}
	res := c.Response()
	res.Header().Set(echo.HeaderContentType, "text/csv; charset=utf-8")
	res.Header().Set(echo.HeaderContentDisposition, `attachment; filename="staryield-leads-`+time.Now().UTC().Format("20060102")+`.csv"`)
	res.Header().Set("Cache-Control", "no-store")
	res.WriteHeader(http.StatusOK)
	if _, err := h.svc.CalculatorReport.ExportLeads(c.Request().Context(), f, res); err != nil {
		// The status line is already sent: the file ends early and the audit
		// trail records the rows written.
		logger.FromContext(c.Request().Context()).Error("marketing leads export failed", zap.Error(err))
	}
	return nil
}

// AdminDeleteMarketingLead erases one lead.
func (h *Handler) AdminDeleteMarketingLead(c echo.Context) error {
	id, err := uuidParam(c, "id")
	if err != nil {
		return err
	}
	if err := h.svc.CalculatorReport.DeleteLead(c.Request().Context(), id); err != nil {
		return err
	}
	return noContent(c)
}

// AdminCalculatorReport returns the e-mail report setting.
func (h *Handler) AdminCalculatorReport(c echo.Context) error {
	out, err := h.svc.Site.CalculatorReportSetting(c.Request().Context())
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminUpdateCalculatorReport replaces the e-mail report setting.
func (h *Handler) AdminUpdateCalculatorReport(c echo.Context) error {
	var in service.CalculatorReportInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Site.UpdateCalculatorReport(c.Request().Context(), in)
	if err != nil {
		return err
	}
	return ok(c, out)
}
