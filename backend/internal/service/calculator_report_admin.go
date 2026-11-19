package service

import (
	"context"
	"encoding/csv"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 6.0 (§26.4): the admin's view of the e-mail leads.

const (
	// leadExportBatch is the page size of the CSV export's keyset reads.
	leadExportBatch = 500
	// leadExportMax bounds one export's duration (the request's own deadline
	// is too short for a large list).
	leadExportMax = 10 * time.Minute
)

// LeadQuery is GET /admin/marketing/leads.
type LeadQuery struct {
	Status string
	Kind   string
	// Q is the exact address (matched through its keyed hash).
	Q      string
	Cursor string
	Limit  int
}

// EmailLeadView is an e-mail lead as the admin console shows it (EmailLead of §26.4).
type EmailLeadView struct {
	ID              uuid.UUID  `json:"id"`
	Email           string     `json:"email"`
	Locale          string     `json:"locale"`
	Country         string     `json:"country"`
	Source          string     `json:"source"`
	Kinds           []string   `json:"kinds"`
	ReportsSent     int        `json:"reports_sent"`
	MarketingStatus string     `json:"marketing_status"`
	SubscribedAt    *time.Time `json:"subscribed_at,omitempty"`
	Prechecked      *bool      `json:"prechecked,omitempty"`
	UnsubscribedAt  *time.Time `json:"unsubscribed_at,omitempty"`
	UserID          *uuid.UUID `json:"user_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (s *CalculatorReportService) address(ctx context.Context, l *domain.EmailLead) string {
	email, err := s.d.Sealer.Open(l.EmailSealed, leadAD(l.ID))
	if err != nil {
		logger.FromContext(ctx).Warn("report: e-mail lead address cannot be decrypted", zap.String("lead_id", l.ID.String()))
		return ""
	}
	return email
}

func (s *CalculatorReportService) view(ctx context.Context, l *domain.EmailLead) EmailLeadView {
	v := EmailLeadView{ID: l.ID, Email: s.address(ctx, l), Locale: l.Locale, Country: l.Country, Source: l.Source,
		Kinds: append([]string{}, l.Kinds...), ReportsSent: l.ReportsSent, MarketingStatus: string(l.MarketingStatus),
		UnsubscribedAt: l.UnsubscribedAt, UserID: l.UserID, CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt}
	if l.ConsentAt != nil {
		v.SubscribedAt, v.Prechecked = l.ConsentAt, l.ConsentPrechecked
	}
	return v
}

// leadFilter validates the list and export filters.
func (s *CalculatorReportService) leadFilter(q LeadQuery) (repository.LeadFilter, error) {
	fields := map[string]string{}
	f := repository.LeadFilter{}
	if q.Status != "" {
		if f.Status = domain.MarketingStatus(q.Status); !f.Status.Valid() {
			fields["status"] = "Use none, subscribed or unsubscribed."
		}
	}
	if q.Kind != "" {
		if f.Kind = domain.ReportKind(q.Kind); !f.Kind.Valid() {
			fields["kind"] = "Use natal-chart, path-of-life or astrology."
		}
	}
	if q := strings.TrimSpace(q.Q); q != "" {
		email, ok := NormalizeEmail(q)
		if !ok {
			fields["q"] = "Enter the full address."
		} else {
			f.EmailHash = s.d.Mail.AddressHash(email)
		}
	}
	if len(fields) > 0 {
		return f, apperror.Validation(fields)
	}
	return f, nil
}

// Leads lists the leads, newest first.
func (s *CalculatorReportService) Leads(ctx context.Context, q LeadQuery) (*Page[EmailLeadView], error) {
	f, err := s.leadFilter(q)
	if err != nil {
		return nil, err
	}
	if f.Cursor, err = decodeCursor(q.Cursor); err != nil {
		return nil, err
	}
	limit := q.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	f.Limit = limit + 1
	rows, err := s.d.Leads.List(ctx, f)
	if err != nil {
		return nil, mapRepoErr(err, "Leads")
	}
	page := &Page[EmailLeadView]{Items: []EmailLeadView{}}
	if len(rows) > limit {
		last := rows[limit-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
		rows = rows[:limit]
	}
	for i := range rows {
		page.Items = append(page.Items, s.view(ctx, &rows[i]))
	}
	return page, nil
}

// LeadExportFilter validates the CSV export's filters; the status defaults to
// subscribed (the list the marketing provider imports).
func (s *CalculatorReportService) LeadExportFilter(status, kind string) (repository.LeadFilter, error) {
	if status == "" {
		status = string(domain.MarketingSubscribed)
	}
	return s.leadFilter(LeadQuery{Status: status, Kind: kind})
}

// ExportLeads streams the leads of f as CSV (address, locale, country,
// subscribed at, source), newest first, in keyset batches, and records the
// export with its row count in the audit trail. The caller has validated f
// with LeadExportFilter and checked the step-up.
func (s *CalculatorReportService) ExportLeads(ctx context.Context, f repository.LeadFilter, w io.Writer) (int, error) {
	ctx, cancel := leadExportContext(ctx)
	defer cancel()
	out := csv.NewWriter(w)
	rows := 0
	defer func() {
		s.record(ctx, "marketing.leads.exported", "email_lead", "csv", map[string]any{"rows": rows, "status": string(f.Status),
			"kind": string(f.Kind)})
	}()
	if err := out.Write([]string{"email", "locale", "country", "subscribed_at", "source"}); err != nil {
		return rows, err
	}
	f.Limit = leadExportBatch
	for {
		batch, err := s.d.Leads.List(ctx, f)
		if err != nil {
			return rows, err
		}
		for i := range batch {
			l := &batch[i]
			subscribed := ""
			if l.ConsentAt != nil {
				subscribed = l.ConsentAt.UTC().Format(time.RFC3339)
			}
			if err := out.Write([]string{leadCSVCell(s.address(ctx, l)), l.Locale, l.Country, subscribed, l.Source}); err != nil {
				return rows, err
			}
			rows++
		}
		out.Flush()
		if err := out.Error(); err != nil {
			return rows, err
		}
		if len(batch) < leadExportBatch {
			return rows, nil
		}
		last := batch[len(batch)-1]
		f.Cursor = &repository.Cursor{At: last.CreatedAt, ID: last.ID.String()}
	}
}

// leadCSVCell keeps a spreadsheet from running a cell as a formula (CSV injection):
// a value that starts with =, +, - or @ (or a control character) is prefixed
// with an apostrophe. Addresses may legitimately start with + or -.
func leadCSVCell(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return v
}

// DeleteLead erases one lead (an erasure request): the row is deleted and the
// deletion audited without the address.
func (s *CalculatorReportService) DeleteLead(ctx context.Context, id uuid.UUID) error {
	var lead *domain.EmailLead
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if lead, err = s.d.Leads.ForUpdate(ctx, id); err != nil {
			return err
		}
		return s.d.Leads.Delete(ctx, id)
	})
	if err != nil {
		return mapRepoErr(err, "Lead")
	}
	s.record(ctx, "marketing.lead.deleted", "email_lead", id.String(), map[string]any{"marketing_status": string(lead.MarketingStatus),
		"kinds": []string(lead.Kinds), "linked_account": lead.UserID != nil, "reports_sent": lead.ReportsSent})
	return nil
}

func (s *CalculatorReportService) record(ctx context.Context, action, targetType, targetID string, meta map[string]any) {
	if s.d.Audit == nil {
		return
	}
	recordAudit(ctx, s.d.Audit, s.d.Clock, s.d.Cfg.IPHashSecret, action, targetType, targetID, meta)
}

// leadExportContext detaches the export from the request's short deadline
// (a large list takes longer) while keeping its values (the admin principal for
// the audit); the export ends when it finishes, fails to write (the client
// left) or leadExportMax passes.
func leadExportContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), leadExportMax)
}
