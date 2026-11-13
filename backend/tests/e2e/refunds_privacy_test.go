package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 5.0 (§24.2–24.4): the account's credit lots, welcome-offer record
// and refund requests are in the personal-data export (never the card key),
// and erasing the account removes the free text of its refund requests while
// the requests stay as financial records.
func TestRefundRequestsAreExportedAndTheirTextIsErasedWithTheAccount(t *testing.T) {
	w := newRefundWorld(t)
	const written = "My sister Jane Doe used my card without asking."
	r := w.request(w.txA, "unauthorized_charge", written, uniq("refund-key-"))
	if r.status != http.StatusCreated {
		t.Fatalf("request: %d %s", r.status, r.body)
	}
	var req service.RefundRequestView
	if err := json.Unmarshal(r.body, &req); err != nil {
		t.Fatal(err)
	}
	w.admin.stepUp()
	w.admin.must(http.StatusOK, http.MethodPost, adminRefunds+"/"+req.ID.String()+"/deny",
		map[string]any{"note": "Jane Doe is a family member; the card holder authorised it."}, nil)

	files := exportFiles(t, w.e, w.c)
	var refunds []struct {
		Reference      string `json:"reference"`
		Status         string `json:"status"`
		Details        string `json:"details"`
		ResolutionNote string `json:"resolution_note"`
		Quote          struct {
			AmountPaidCents int64 `json:"amount_paid_cents"`
		} `json:"quote"`
	}
	if err := json.Unmarshal(files["billing/refund-requests.json"], &refunds); err != nil || len(refunds) != 1 {
		t.Fatalf("export refund requests: %v %s", err, files["billing/refund-requests.json"])
	}
	if got := refunds[0]; got.Reference != req.Reference || got.Status != "denied" || got.Details != written ||
		got.ResolutionNote == "" || got.Quote.AmountPaidCents != 999 {
		t.Fatalf("exported request: %+v", got)
	}
	var lots []struct {
		Source       string `json:"source"`
		CreditsTotal int64  `json:"credits_total"`
		PaidCents    int64  `json:"paid_cents"`
	}
	if err := json.Unmarshal(files["credits/lots.json"], &lots); err != nil {
		t.Fatalf("export lots: %v %s", err, files["credits/lots.json"])
	}
	purchased := 0
	for _, l := range lots {
		if l.Source == "purchase" {
			purchased++
		}
	}
	if purchased != 2 {
		t.Fatalf("export lots: want the two purchase lots, got %+v", lots)
	}
	if _, ok := files["billing/welcome-offer.json"]; !ok {
		t.Fatal("export: billing/welcome-offer.json is missing")
	}
	for path, body := range files {
		if bytes.Contains(body, []byte("card_key")) {
			t.Fatalf("export: %s carries the welcome-offer card key", path)
		}
	}

	// Closing and erasing the account keeps the request (amounts, status) without its text.
	w.c.must(http.StatusAccepted, http.MethodPost, "/api/v1/me/close", map[string]any{"password": strongPassword, "confirmation": "DELETE"}, nil)
	w.e.clock.Advance(31*24*time.Hour + time.Minute)
	runClosure(t, w.e)
	var row struct {
		Status               string
		Details              string
		ResolutionNote       string
		QuoteAmountPaidCents int64
	}
	if err := w.e.app.DB.Raw(`SELECT status, details, resolution_note, quote_amount_paid_cents FROM refund_requests WHERE id = ?`, req.ID).
		Row().Scan(&row.Status, &row.Details, &row.ResolutionNote, &row.QuoteAmountPaidCents); err != nil {
		t.Fatal(err)
	}
	if row.Status != "denied" || row.Details != "" || row.ResolutionNote != "" || row.QuoteAmountPaidCents != 999 {
		t.Fatalf("after erasure: %+v", row)
	}
}
