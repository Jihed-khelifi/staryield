package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 6.0 (§26.6): invoices and credit notes.

func (h *Handler) invoices() (*service.InvoiceService, error) {
	if h.svc.Invoices == nil {
		return nil, apperror.FeatureUnavailable("Invoices are not available yet.")
	}
	return h.svc.Invoices, nil
}

// pdf writes a rendered document as a download. Documents carry personal
// data: never cached, never sniffed.
func pdf(c echo.Context, body []byte, filename string) error {
	hdr := c.Response().Header()
	hdr.Set("Cache-Control", "private, no-store")
	hdr.Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	hdr.Set("X-Content-Type-Options", "nosniff")
	return c.Blob(http.StatusOK, "application/pdf", body)
}

// Invoice returns one of the caller's invoices or credit notes (404 for
// anyone else's).
func (h *Handler) Invoice(c echo.Context) error {
	s, err := h.invoices()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return apperror.NotFound("Invoice")
	}
	out, err := s.Get(c.Request().Context(), principal(c).UserID, id)
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "private, no-store")
	return ok(c, out)
}

// InvoicePDF downloads the PDF of one of the caller's documents.
func (h *Handler) InvoicePDF(c echo.Context) error {
	s, err := h.invoices()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return apperror.NotFound("Invoice")
	}
	body, name, err := s.PDF(c.Request().Context(), principal(c).UserID, id)
	if err != nil {
		return err
	}
	return pdf(c, body, name)
}

// AdminInvoices lists documents of every account.
func (h *Handler) AdminInvoices(c echo.Context) error {
	s, err := h.invoices()
	if err != nil {
		return err
	}
	limit, err := limitParam(c, 20, 100)
	if err != nil {
		return err
	}
	userID, err := optionalUUID(c, "user_id")
	if err != nil {
		return err
	}
	out, err := s.AdminList(c.Request().Context(), service.AdminInvoiceQuery{UserID: userID, Kind: c.QueryParam("kind"), Q: c.QueryParam("q"),
		From: c.QueryParam("from"), To: c.QueryParam("to"), Cursor: c.QueryParam("cursor"), Limit: limit})
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminInvoice returns any document.
func (h *Handler) AdminInvoice(c echo.Context) error {
	s, err := h.invoices()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return apperror.NotFound("Invoice")
	}
	out, err := s.AdminGet(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return ok(c, out)
}

// AdminInvoicePDF downloads the PDF of any document.
func (h *Handler) AdminInvoicePDF(c echo.Context) error {
	s, err := h.invoices()
	if err != nil {
		return err
	}
	id, err := uuidParam(c, "id")
	if err != nil {
		return apperror.NotFound("Invoice")
	}
	body, name, err := s.AdminPDF(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return pdf(c, body, name)
}

// AdminExportInvoices streams the accounting export (CSV) of a date range of
// at most 366 days. Step-up protected and audited.
func (h *Handler) AdminExportInvoices(c echo.Context) error {
	s, err := h.invoices()
	if err != nil {
		return err
	}
	from, to := c.QueryParam("from"), c.QueryParam("to")
	if _, _, err := service.ExportRange(from, to); err != nil { // refuse before any byte is written
		return err
	}
	// A large range may take longer than the API's usual write deadline.
	_ = http.NewResponseController(c.Response().Writer).SetWriteDeadline(time.Now().Add(10 * time.Minute))
	hdr := c.Response().Header()
	hdr.Set("Cache-Control", "private, no-store")
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Content-Disposition", `attachment; filename="invoices-`+safeDate(from)+`-`+safeDate(to)+`.csv"`)
	hdr.Set(echo.HeaderContentType, "text/csv; charset=utf-8")
	c.Response().WriteHeader(http.StatusOK)
	_, err = s.ExportCSV(c.Request().Context(), from, to, c.Response())
	return err
}

// safeDate keeps the date part of an export bound for the file name.
func safeDate(s string) string {
	if len(s) >= 10 {
		s = s[:10]
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= '0' && c <= '9' || c == '-' {
			out = append(out, c)
		}
	}
	return string(out)
}
