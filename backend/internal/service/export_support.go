package service

// revisions 3.0: support desk. The personal-data export includes the user's
// support requests with the public messages only: internal notes and system
// entries are the team's working notes, not data the user provided.

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// collectSupport adds support.json to an export.
func (s *ExportService) collectSupport(ctx context.Context, b *ExportBundle, userID uuid.UUID, rows exportRows, capped func(int, string)) error {
	var tickets []domain.SupportTicket
	if err := rows(&tickets, "user_id"); err != nil {
		return err
	}
	capped(len(tickets), "support requests")
	out := make([]map[string]any, 0, len(tickets))
	for i := range tickets {
		t := &tickets[i]
		email, err := s.d.Sealer.Open(t.EmailEnc, supportAD(t.ID, "email"))
		if err != nil {
			return fmt.Errorf("export: open support address: %w", err)
		}
		first, err := s.d.Sealer.Open(t.MessageEnc, supportAD(t.ID, "message"))
		if err != nil {
			return fmt.Errorf("export: open support request: %w", err)
		}
		thread := []map[string]any{{"from": "you", "body": first, "at": t.CreatedAt}}
		var msgs []domain.SupportMessage
		if err := s.d.Data.Rows(ctx, &msgs, "ticket_id", t.ID, domain.MaxSupportMessages); err != nil {
			return err
		}
		for _, m := range msgs {
			if m.Visibility != domain.SupportPublic || m.AuthorType == domain.SupportAuthorSystem {
				continue
			}
			body, err := s.d.Sealer.Open(m.BodyEnc, supportMessageAD(m.ID))
			if err != nil {
				return fmt.Errorf("export: open support message: %w", err)
			}
			from := "you"
			if m.AuthorType == domain.SupportAuthorStaff {
				from = "support team"
			}
			thread = append(thread, map[string]any{"from": from, "body": body, "at": m.CreatedAt})
		}
		entry := map[string]any{"reference": t.Reference, "topic": t.Topic, "subject": t.Subject, "status": t.Status,
			"kind": kindOf(t), "email": email, "name": t.Name, "created_at": t.CreatedAt, "messages": thread}
		if isReport(t) { // revisions 3.1: conversation reports
			entry["reason"], entry["conversation_id"], entry["session_id"] = *t.ReportReason, t.ConversationID, t.SessionID
		}
		out = append(out, entry)
	}
	b.Documents = append(b.Documents, ExportDocument{Path: "support.json", Data: out})
	return nil
}
