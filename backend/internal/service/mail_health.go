package service

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// growth: e-mail delivery. The admin console's view of the outbox — which
// provider is configured, what happened to the e-mail of each template, why
// messages did not go out, and a synchronous test send — so a deployment that
// sends nothing (no provider, a plan restricted to verified recipients, a
// revoked token) is visible at a glance instead of only in the logs.

// Delivery states reported by MailHealth.Delivery.
const (
	// DeliveryOff: the no-op driver is configured; nothing is sent.
	DeliveryOff = "off"
	// DeliveryLogOnly: the development driver writes e-mails to the log.
	DeliveryLogOnly = "log_only"
	// DeliveryProvider: a real provider delivers e-mail.
	DeliveryProvider = "provider"
)

// MailHealth is GET /admin/mail/health.
type MailHealth struct {
	Driver         string `json:"driver"`
	Delivery       string `json:"delivery"`
	From           string `json:"from"`
	AllowMarketing bool   `json:"allow_marketing"`
	// Days and Since describe the window of the counts.
	Days      int                   `json:"days"`
	Since     time.Time             `json:"since"`
	Totals    MailStatusCounts      `json:"totals"`
	Templates []MailTemplateHealth  `json:"templates"`
	Reasons   []MailReasonCount     `json:"reasons"`
	Backlog   MailBacklogView       `json:"backlog"`
	Problems  []MailProblemView     `json:"problems"`
	Alerts    []MailHealthAlertView `json:"alerts"`
}

// MailStatusCounts counts outbox rows by status.
type MailStatusCounts struct {
	Queued     int64 `json:"queued"`
	Sending    int64 `json:"sending"`
	Sent       int64 `json:"sent"`
	Failed     int64 `json:"failed"`
	Dead       int64 `json:"dead"`
	Suppressed int64 `json:"suppressed"`
	Cancelled  int64 `json:"cancelled"`
	Total      int64 `json:"total"`
}

func (c *MailStatusCounts) add(status domain.EmailStatus, n int64) {
	switch status {
	case domain.EmailQueued:
		c.Queued += n
	case domain.EmailSending:
		c.Sending += n
	case domain.EmailSent:
		c.Sent += n
	case domain.EmailFailed:
		c.Failed += n
	case domain.EmailDead:
		c.Dead += n
	case domain.EmailSuppressed:
		c.Suppressed += n
	case domain.EmailCancelled:
		c.Cancelled += n
	}
	c.Total += n
}

// MailTemplateHealth is the outcome of one template's e-mail.
type MailTemplateHealth struct {
	Template string `json:"template"`
	MailStatusCounts
}

// MailReasonCount counts the rows that did not go out for one reason.
type MailReasonCount struct {
	Status domain.EmailStatus `json:"status"`
	Reason string             `json:"reason"`
	Count  int64              `json:"count"`
}

// MailBacklogView summarises the e-mail waiting to be sent.
type MailBacklogView struct {
	Queued      int64      `json:"queued"`
	Retrying    int64      `json:"retrying"`
	OldestDueAt *time.Time `json:"oldest_due_at,omitempty"`
}

// MailProblemView is one message that did not go out or is being retried. It
// never carries the recipient or the body.
type MailProblemView struct {
	ID            uuid.UUID          `json:"id"`
	Template      string             `json:"template"`
	Category      string             `json:"category"`
	Status        domain.EmailStatus `json:"status"`
	Reason        string             `json:"reason,omitempty"`
	Error         string             `json:"error,omitempty"`
	ProviderCode  int                `json:"provider_code,omitempty"`
	Hint          string             `json:"hint,omitempty"`
	Attempts      int                `json:"attempts"`
	NextAttemptAt *time.Time         `json:"next_attempt_at,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
}

// MailHealthAlertView is a problem the operator must act on.
type MailHealthAlertView struct {
	// Code is no_provider, log_only, provider_config or backlog.
	Code string `json:"code"`
	// Hint is what the operator should check (English operator guidance, like
	// the provider hints of the mail package; shown as is by the console).
	Hint  string `json:"hint,omitempty"`
	Count int64  `json:"count,omitempty"`
}

// providerError reads "status 403 code 10208 …" back from last_error.
var providerError = regexp.MustCompile(`^status (\d+)(?: code (\d+))?`)

func parseProviderError(lastError string) (status, code int) {
	m := providerError.FindStringSubmatch(lastError)
	if m == nil {
		return 0, 0
	}
	status, _ = strconv.Atoi(m[1])
	if m[2] != "" {
		code, _ = strconv.Atoi(m[2])
	}
	return status, code
}

// deliveryOf names how the configured driver delivers.
func deliveryOf(driver string) string {
	switch driver {
	case "noop":
		return DeliveryOff
	case "log":
		return DeliveryLogOnly
	}
	return DeliveryProvider
}

// backlogAlertAge is how late the oldest queued message may be before the
// view raises an alert (the worker polls every few seconds).
const backlogAlertAge = 15 * time.Minute

// Health reports the outbox of the last `days` days (1..30).
func (s *MailService) Health(ctx context.Context, days int) (*MailHealth, error) {
	if days < 1 || days > 30 {
		return nil, apperror.Validation(map[string]string{"days": "Use a number of days between 1 and 30."})
	}
	now := s.clock.Now()
	since := now.Add(-time.Duration(days) * 24 * time.Hour)
	stats, err := s.repo.DeliveryStats(ctx, since)
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("mail health: stats: %w", err))
	}
	problems, err := s.repo.Problems(ctx, since, 30)
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("mail health: problems: %w", err))
	}
	backlog, err := s.repo.Backlog(ctx)
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("mail health: backlog: %w", err))
	}
	driver := mail.Name(s.provider)
	out := &MailHealth{Driver: driver, Delivery: deliveryOf(driver), From: s.cfg.From, AllowMarketing: s.cfg.AllowMarketing,
		Days: days, Since: since, Templates: []MailTemplateHealth{}, Reasons: []MailReasonCount{}, Problems: []MailProblemView{},
		Alerts:  []MailHealthAlertView{},
		Backlog: MailBacklogView{Queued: backlog.Queued, Retrying: backlog.Retrying, OldestDueAt: backlog.OldestDue}}
	byTemplate := map[string]*MailTemplateHealth{}
	reasons := map[MailReasonCount]int64{}
	for _, st := range stats {
		t := byTemplate[st.Template]
		if t == nil {
			t = &MailTemplateHealth{Template: st.Template}
			byTemplate[st.Template] = t
		}
		t.add(st.Status, st.Count)
		out.Totals.add(st.Status, st.Count)
		if st.Status != domain.EmailSent && st.Status != domain.EmailQueued && st.Status != domain.EmailSending {
			reasons[MailReasonCount{Status: st.Status, Reason: st.Reason}] += st.Count
		}
	}
	for _, t := range byTemplate {
		out.Templates = append(out.Templates, *t)
	}
	sort.Slice(out.Templates, func(i, j int) bool {
		if out.Templates[i].Total != out.Templates[j].Total {
			return out.Templates[i].Total > out.Templates[j].Total
		}
		return out.Templates[i].Template < out.Templates[j].Template
	})
	for k, n := range reasons {
		k.Count = n
		out.Reasons = append(out.Reasons, k)
	}
	sort.Slice(out.Reasons, func(i, j int) bool {
		if out.Reasons[i].Count != out.Reasons[j].Count {
			return out.Reasons[i].Count > out.Reasons[j].Count
		}
		return out.Reasons[i].Reason < out.Reasons[j].Reason
	})
	configHints := map[string]int64{}
	for _, p := range problems {
		v := MailProblemView{ID: p.ID, Template: p.Template, Category: string(p.Category), Status: p.Status, Reason: p.StatusReason,
			Error: p.LastError, Attempts: p.Attempts, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
		if p.Status == domain.EmailQueued {
			due := p.NextAttemptAt
			v.NextAttemptAt = &due
		}
		if status, code := parseProviderError(p.LastError); status != 0 {
			v.ProviderCode = code
			if hint, ok := mail.ConfigHint(status, code); ok && strings.EqualFold(p.Provider, "cloudflare") {
				v.Hint = hint
				configHints[hint]++
			}
		}
		out.Problems = append(out.Problems, v)
	}
	switch out.Delivery {
	case DeliveryOff:
		out.Alerts = append(out.Alerts, MailHealthAlertView{Code: "no_provider", Count: reasons[MailReasonCount{Status: domain.EmailCancelled, Reason: reasonNoProvider}],
			Hint: "set STARYIELD_MAIL_DRIVER=cloudflare with STARYIELD_MAIL_CLOUDFLARE_ACCOUNT_ID and STARYIELD_MAIL_CLOUDFLARE_API_TOKEN"})
	case DeliveryLogOnly:
		out.Alerts = append(out.Alerts, MailHealthAlertView{Code: "log_only", Hint: "development driver: e-mails are written to the API log"})
	}
	hints := make([]string, 0, len(configHints))
	for h := range configHints {
		hints = append(hints, h)
	}
	sort.Strings(hints)
	for _, h := range hints {
		out.Alerts = append(out.Alerts, MailHealthAlertView{Code: "provider_config", Hint: h, Count: configHints[h]})
	}
	if backlog.OldestDue != nil && now.Sub(*backlog.OldestDue) > backlogAlertAge {
		out.Alerts = append(out.Alerts, MailHealthAlertView{Code: "backlog", Count: backlog.Queued,
			Hint: "queued e-mail is late: check that the API workers run (STARYIELD_WORKER_ENABLED) and that the provider answers"})
	}
	return out, nil
}

// mailAudit records admin actions of the delivery view (test sends).
type mailAudit struct {
	repo     repository.AuditRepository
	ipSecret string
}

// WithAudit installs the audit log used by SendTest (wired at start-up).
func (s *MailService) WithAudit(repo repository.AuditRepository, ipHashSecret string) *MailService {
	s.audit = &mailAudit{repo: repo, ipSecret: ipHashSecret}
	return s
}

// MailTestInput is POST /admin/mail/test.
type MailTestInput struct {
	To     string `json:"to"`
	Locale string `json:"locale"`
}

// MailTestResult reports the provider's answer to a test e-mail.
type MailTestResult struct {
	Delivered bool   `json:"delivered"`
	Driver    string `json:"driver"`
	// Queued: the provider accepted the message for later delivery.
	Queued    bool   `json:"queued,omitempty"`
	MessageID string `json:"message_id,omitempty"`
	// Discarded: the no-op driver accepted and dropped it.
	Discarded bool `json:"discarded,omitempty"`
	// Error describes a refusal ("status 403 code 10208 …"), with the
	// provider's code, an operator hint and whether a retry may help.
	Error        string `json:"error,omitempty"`
	ProviderCode int    `json:"provider_code,omitempty"`
	Hint         string `json:"hint,omitempty"`
	Retryable    bool   `json:"retryable,omitempty"`
	Suppressed   bool   `json:"suppressed,omitempty"`
}

// SendTest sends a short test e-mail through the provider right away
// (bypassing the outbox, so the answer comes back to the console) and reports
// what the provider said. The recipient is never stored or logged in clear.
func (s *MailService) SendTest(ctx context.Context, in MailTestInput) (*MailTestResult, error) {
	to, ok := NormalizeEmail(in.To)
	if !ok {
		return nil, apperror.Validation(map[string]string{"to": "Enter a valid email address."})
	}
	locale := strings.TrimSpace(in.Locale)
	if locale == "" {
		locale = "en"
	}
	rendered, err := mail.Render(mail.MailTest(locale), mail.Layout{Locale: locale, FrontendURL: s.cfg.FrontendURL,
		Category: string(domain.EmailAccount), SupportEmail: s.cfg.SupportEmail})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	sendCtx, cancel := context.WithTimeout(ctx, s.cfg.SendTimeout)
	defer cancel()
	res, err := s.provider.Send(sendCtx, mail.Message{To: to, Subject: rendered.Subject, Text: rendered.Text, HTML: rendered.HTML,
		Headers: rendered.Headers, Category: string(domain.EmailAccount)})
	out := &MailTestResult{Driver: mail.Name(s.provider)}
	switch {
	case err != nil:
		out.Error, out.Retryable, out.Hint = describeMailErr(err), mail.IsRetryable(err), mail.HintOf(err)
		_, out.ProviderCode = parseProviderError(out.Error)
		logger.FromContext(ctx).Warn("mail: test e-mail refused", zap.String("error", out.Error), zap.String("hint", out.Hint))
	case res.Discarded:
		out.Discarded = true
	case bounced(res, to):
		out.Suppressed = true
	default:
		out.Delivered, out.Queued, out.MessageID = true, res.Queued, truncateString(res.MessageID, 200)
	}
	if s.audit != nil {
		recordAudit(ctx, s.audit.repo, s.clock, s.audit.ipSecret, "admin.mail_test_sent", "email", maskEmail(to),
			map[string]any{"driver": out.Driver, "delivered": out.Delivered, "discarded": out.Discarded, "suppressed": out.Suppressed,
				"error": out.Error, "provider_code": out.ProviderCode})
	}
	return out, nil
}
