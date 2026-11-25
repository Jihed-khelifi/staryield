package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
)

func healthOf(t *testing.T, svc *MailService) *MailHealth {
	t.Helper()
	h, err := svc.Health(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func hasAlert(h *MailHealth, code string) *MailHealthAlertView {
	for i := range h.Alerts {
		if h.Alerts[i].Code == code {
			return &h.Alerts[i]
		}
	}
	return nil
}

// The delivery view counts each template's e-mail by outcome and explains why
// messages did not go out, without recipients or bodies.
func TestMailHealthCountsOutcomesPerTemplate(t *testing.T) {
	svc, _, rec, _ := newTestMail(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := svc.Enqueue(ctx, req(uuid.New(), domain.EmailAccount, "calculator_report")); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.Enqueue(ctx, req(uuid.New(), domain.EmailMarketing, "notification:broadcast")); err != nil {
		t.Fatal(err)
	}
	rec.SetFail(func(m mail.Message) (mail.Result, error) {
		if len(rec.Sent()) >= 2 { // the third message is refused
			return mail.Result{}, &mail.SendError{Status: 400, Code: 10202, Reason: "email.sending.error.email.invalid"}
		}
		return mail.Result{}, nil
	})
	if _, err := svc.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	h := healthOf(t, svc)
	if h.Driver != "recorder" || h.Delivery != DeliveryProvider || h.From != "" || h.Days != 7 {
		t.Fatalf("header: %+v", h)
	}
	if h.Totals.Total != 4 || h.Totals.Sent != 2 || h.Totals.Failed != 1 || h.Totals.Cancelled != 1 {
		t.Fatalf("totals: %+v", h.Totals)
	}
	if len(h.Templates) != 2 || h.Templates[0].Template != "calculator_report" || h.Templates[0].Sent != 2 || h.Templates[0].Failed != 1 {
		t.Fatalf("templates: %+v", h.Templates)
	}
	reasons := map[string]int64{}
	for _, r := range h.Reasons {
		reasons[string(r.Status)+":"+r.Reason] = r.Count
	}
	if reasons["failed:rejected"] != 1 || reasons["cancelled:no_marketing_provider"] != 1 {
		t.Fatalf("reasons: %+v", h.Reasons)
	}
	if len(h.Problems) != 2 {
		t.Fatalf("problems: %+v", h.Problems)
	}
	for _, p := range h.Problems {
		if p.Template == "calculator_report" && (p.ProviderCode != 10202 || !strings.Contains(p.Error, "10202") || p.Hint != "") {
			t.Fatalf("message problem: %+v", p)
		}
	}
	if len(h.Alerts) != 0 {
		t.Fatalf("no operator alert expected: %+v", h.Alerts)
	}
}

// A deployment without a provider, or a provider account problem, is an
// operator alert with what to fix.
func TestMailHealthAlerts(t *testing.T) {
	ctx := context.Background()
	t.Run("no provider", func(t *testing.T) {
		svc, _, _, _ := newTestMail(t)
		svc.provider = mail.NoopMailer{}
		_ = svc.Enqueue(ctx, req(uuid.New(), domain.EmailAccount, "calculator_report"))
		_, _ = svc.Drain(ctx)
		h := healthOf(t, svc)
		a := hasAlert(h, "no_provider")
		if h.Delivery != DeliveryOff || a == nil || a.Count != 1 || !strings.Contains(a.Hint, "STARYIELD_MAIL_DRIVER=cloudflare") {
			t.Fatalf("alerts: %+v", h.Alerts)
		}
	})
	t.Run("provider configuration", func(t *testing.T) {
		svc, repo, _, _ := newTestMail(t)
		cf := &namedFail{name: "cloudflare", err: &mail.SendError{Status: 403, Code: 10208, Reason: "email.sending.error.email.recipient_plan_restricted",
			Retryable: true, Config: true}}
		svc.provider = cf
		_ = svc.Enqueue(ctx, req(uuid.New(), domain.EmailAccount, "calculator_report"))
		_, _ = svc.Drain(ctx)
		row := repo.byTemplate("calculator_report")[0]
		if row.Status != domain.EmailQueued {
			t.Fatalf("a configuration failure is retried: %+v", row)
		}
		h := healthOf(t, svc)
		a := hasAlert(h, "provider_config")
		if a == nil || a.Count != 1 || !strings.Contains(a.Hint, "Workers Paid") {
			t.Fatalf("alerts: %+v", h.Alerts)
		}
		if len(h.Problems) != 1 || h.Problems[0].ProviderCode != 10208 || h.Problems[0].NextAttemptAt == nil || h.Backlog.Retrying != 1 {
			t.Fatalf("problems/backlog: %+v %+v", h.Problems, h.Backlog)
		}
	})
	t.Run("late backlog", func(t *testing.T) {
		svc, _, _, clk := newTestMail(t)
		_ = svc.Enqueue(ctx, req(uuid.New(), domain.EmailAccount, "calculator_report"))
		clk.Advance(time.Hour) // nothing delivered it
		if a := hasAlert(healthOf(t, svc), "backlog"); a == nil || a.Count != 1 {
			t.Fatal("a late queue must raise the backlog alert")
		}
	})
	t.Run("window", func(t *testing.T) {
		svc, _, _, _ := newTestMail(t)
		for _, days := range []int{0, 31, -1} {
			if _, err := svc.Health(ctx, days); err == nil {
				t.Fatalf("days=%d accepted", days)
			}
		}
	})
}

// namedFail is a provider that always fails with err, under a given name.
type namedFail struct {
	name string
	err  error
}

func (n *namedFail) Name() string { return n.name }
func (n *namedFail) Send(context.Context, mail.Message) (mail.Result, error) {
	return mail.Result{}, n.err
}

// The test send reaches the provider at once and reports its answer.
func TestMailSendTestReportsTheProviderAnswer(t *testing.T) {
	ctx := context.Background()
	svc, repo, rec, _ := newTestMail(t)
	out, err := svc.SendTest(ctx, MailTestInput{To: "Ops@Example.com", Locale: "fr"})
	if err != nil || !out.Delivered || out.Driver != "recorder" || out.MessageID == "" {
		t.Fatalf("delivered: %+v %v", out, err)
	}
	sent := rec.Sent()
	if len(sent) != 1 || sent[0].To != "ops@example.com" || !strings.Contains(sent[0].Subject, "Staryield") {
		t.Fatalf("sent: %+v", sent)
	}
	if len(repo.rows) != 0 {
		t.Fatal("a test send must not touch the outbox")
	}
	svc.provider = &namedFail{name: "cloudflare", err: &mail.SendError{Status: 401, Code: 10101, Reason: "email.sending.error.authentication.unauthorized",
		Retryable: true, Config: true, Hint: "check the token"}}
	out, err = svc.SendTest(ctx, MailTestInput{To: "ops@example.com"})
	if err != nil || out.Delivered || out.ProviderCode != 10101 || out.Hint != "check the token" || !out.Retryable {
		t.Fatalf("refused: %+v %v", out, err)
	}
	svc.provider = mail.NoopMailer{}
	if out, _ := svc.SendTest(ctx, MailTestInput{To: "ops@example.com"}); out.Delivered || !out.Discarded {
		t.Fatalf("noop: %+v", out)
	}
	if _, err := svc.SendTest(ctx, MailTestInput{To: "not-an-address"}); err == nil {
		t.Fatal("invalid address accepted")
	}
}
