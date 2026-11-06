package service

import (
	"context"
	"strings"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
)

// MailSupportNotifier e-mails new contact-form requests to the support inbox
// and acknowledges them to the requester, through the mail outbox (sealed
// at rest, retried, wiped once sent).
type MailSupportNotifier struct {
	queue EmailQueue
	inbox string
}

// NewMailSupportNotifier returns a notifier; with an empty inbox only the
// acknowledgement is sent.
func NewMailSupportNotifier(queue EmailQueue, inbox string) *MailSupportNotifier {
	return &MailSupportNotifier{queue: queue, inbox: strings.TrimSpace(inbox)}
}

// TicketCreated implements SupportNotifier. Failures are logged: the ticket
// itself is already stored.
func (n *MailSupportNotifier) TicketCreated(ctx context.Context, t *domain.SupportTicket, email, message string) {
	if n == nil || n.queue == nil {
		return
	}
	reqs := []EmailRequest{{UserID: t.UserID, To: email, Locale: t.Locale, Category: domain.EmailAccount,
		Template: "support_ack", Content: mail.SupportAcknowledgement(t.Locale, t.Reference),
		DedupeKey: "support:" + t.ID.String() + ":ack"}}
	if n.inbox != "" {
		name := t.Name
		if name == "" {
			name = "(no name)"
		}
		reqs = append(reqs, EmailRequest{To: n.inbox, Locale: "en", Category: domain.EmailAccount, Template: "support_ticket",
			Content:   mail.SupportTicketForTeam(t.Reference, string(t.Topic), name, email, t.Subject, message, t.UserID != nil),
			DedupeKey: "support:" + t.ID.String() + ":team"})
	}
	warnErr(ctx, "support: e-mail ticket", n.queue.Enqueue(database.Detach(ctx), reqs...))
}
