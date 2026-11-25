package e2e

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// growth: e-mail delivery. The admin console sees what happened to the
// calculator reports (and every other e-mail) and can prove the provider
// delivers with a test send; nothing of it exposes recipients or bodies.
func TestAdminMailHealthAndTestSend(t *testing.T) {
	e := newEnv(t)
	e.reportStart()
	e.flushMail()
	visitor := e.client()
	for _, email := range []string{"health-ok@example.com", "health-refused@example.com"} {
		if r := visitor.postReport(reportBody("astrology", reportSun, email, nil), ""); r.status != http.StatusAccepted {
			t.Fatalf("report: %d %s", r.status, r.body)
		}
	}
	// The provider refuses one recipient the way Cloudflare's Workers Free plan
	// does for unverified addresses: a configuration problem, retried later.
	e.mail.SetFail(func(m mail.Message) (mail.Result, error) {
		if m.To == "health-refused@example.com" {
			return mail.Result{}, &mail.SendError{Status: 403, Code: 10208, Reason: "email.sending.error.email.recipient_plan_restricted",
				Retryable: true, Config: true}
		}
		return mail.Result{}, nil
	})
	t.Cleanup(func() { e.mail.SetFail(nil) })
	e.flushMail()

	admin := e.createAdmin()
	var h service.MailHealth
	admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/mail/health?days=7", nil, &h)
	if h.Driver != "recorder" || h.Delivery != service.DeliveryProvider || h.Days != 7 {
		t.Fatalf("header: %+v", h)
	}
	var report *service.MailTemplateHealth
	for i := range h.Templates {
		if h.Templates[i].Template == service.ReportEmailTemplate {
			report = &h.Templates[i]
		}
	}
	if report == nil || report.Sent < 1 || report.Queued < 1 {
		t.Fatalf("calculator report outcomes: %+v", h.Templates)
	}
	found := false
	for _, p := range h.Problems {
		if p.Template == service.ReportEmailTemplate && p.ProviderCode == 10208 && p.Status == domain.EmailQueued {
			found = true
		}
	}
	if !found {
		t.Fatalf("the refused report is listed with the provider code: %+v", h.Problems)
	}
	body := string(mustJSON(t, h))
	for _, secret := range []string{"health-ok@example.com", "health-refused@example.com", "1990-08-23"} {
		if strings.Contains(body, secret) {
			t.Fatalf("the delivery view leaks %q", secret)
		}
	}
	if r := admin.call(http.MethodGet, "/api/v1/admin/mail/health?days=90", nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("days=90: %d", r.status)
	}

	// The test send needs a fresh step-up (the sign-in's own TOTP counts for
	// 10 minutes), then reports the provider's answer.
	e.clock.Advance(11 * time.Minute)
	path := "/api/v1/admin/mail/test"
	if r := admin.call(http.MethodPost, path, map[string]any{"to": "ops@example.com"}); r.errCode(t) != "step_up_required" {
		t.Fatalf("test send without step-up: %d %s", r.status, r.body)
	}
	admin.stepUp()
	var res service.MailTestResult
	admin.must(http.StatusOK, http.MethodPost, path, map[string]any{"to": "Ops@Example.com"}, &res)
	if !res.Delivered || res.Driver != "recorder" {
		t.Fatalf("test send: %+v", res)
	}
	if got := e.mail.Sent(); got[len(got)-1].To != "ops@example.com" {
		t.Fatalf("the test e-mail did not reach the provider: %+v", got[len(got)-1])
	}
	admin.must(http.StatusOK, http.MethodPost, path, map[string]any{"to": "health-refused@example.com"}, &res)
	if res.Delivered || res.ProviderCode != 10208 || !res.Retryable {
		t.Fatalf("refused test send: %+v", res)
	}
	if r := admin.call(http.MethodPost, path, map[string]any{"to": "nope"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("invalid address: %d", r.status)
	}
	var audited int64
	e.app.DB.Model(&domain.AuditLog{}).Where("action = ? AND target_id = ?", "admin.mail_test_sent", "o***@example.com").Count(&audited)
	if n := audited; n < 1 {
		t.Fatalf("test sends audited: %d", n)
	}
	// Clients never reach the view.
	if r := e.registerClient().call(http.MethodGet, "/api/v1/admin/mail/health", nil); r.status == http.StatusOK {
		t.Fatal("a client read the delivery view")
	}
}
