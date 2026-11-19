package service

// revisions 6.0 (§26.4): the calculators' e-mail report. The personal-data
// export includes the lead linked to the account (what was asked for, how
// often, and the marketing consent with its evidence). The address is the
// account's own and the IP hash never leaves the database; birth details were
// never stored.

import (
	"context"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// collectCalculatorReports adds calculator-reports.json to an export when the
// account has e-mailed reports.
func (s *ExportService) collectCalculatorReports(ctx context.Context, b *ExportBundle, userID uuid.UUID, rows exportRows) error {
	var leads []domain.EmailLead
	if err := rows(&leads, "user_id"); err != nil {
		return err
	}
	if len(leads) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(leads))
	for i := range leads {
		l := &leads[i]
		entry := map[string]any{"calculators": []string(l.Kinds), "first_calculator": l.Source, "reports_sent": l.ReportsSent, "locale": l.Locale,
			"country": l.Country, "marketing_status": l.MarketingStatus, "first_requested_at": l.CreatedAt, "last_requested_at": l.LastRequestAt}
		if l.ConsentAt != nil {
			entry["marketing_consent"] = map[string]any{"at": l.ConsentAt, "box_started_ticked": l.ConsentPrechecked, "consent_text_version": l.ConsentTextVersion,
				"browser": l.ConsentUserAgent}
		}
		if l.UnsubscribedAt != nil {
			entry["unsubscribed_at"] = l.UnsubscribedAt
		}
		out = append(out, entry)
	}
	b.Documents = append(b.Documents, ExportDocument{Path: "calculator-reports.json", Data: out})
	return nil
}
