package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

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
func (n *MailSupportNotifier) TicketCreated(ctx context.Context, t *domain.SupportTicket, email, message, link string) {
	if n == nil || n.queue == nil {
		return
	}
	reqs := []EmailRequest{{UserID: t.UserID, To: email, Locale: t.Locale, Category: domain.EmailAccount,
		Template: "support_ack", Content: mail.SupportAcknowledgement(t.Locale, t.Reference, link),
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

// StaffReplied implements SupportNotifier: the reply text and the private link
// go to the address given in the request, in the ticket's language.
func (n *MailSupportNotifier) StaffReplied(ctx context.Context, t *domain.SupportTicket, email, body, link string, messageID uuid.UUID) error {
	if n == nil || n.queue == nil {
		return fmt.Errorf("support: no mail queue")
	}
	return n.queue.Enqueue(database.Detach(ctx), EmailRequest{UserID: t.UserID, To: email, Locale: t.Locale, Category: domain.EmailAccount,
		Template: "support_reply", Content: mail.SupportReply(t.Locale, t.Reference, body, link),
		DedupeKey: "support:" + t.ID.String() + ":reply:" + messageID.String()})
}

// CustomerReplied implements SupportNotifier: the team (the assignee or the
// support inbox) is told with the reply and a console link.
func (n *MailSupportNotifier) CustomerReplied(ctx context.Context, t *domain.SupportTicket, to, body, consoleLink string,
	messageID uuid.UUID, reopened bool) {
	if n == nil || n.queue == nil {
		return
	}
	if to = strings.TrimSpace(to); to == "" {
		to = n.inbox
	}
	if to == "" {
		return
	}
	warnErr(ctx, "support: e-mail requester reply", n.queue.Enqueue(database.Detach(ctx), EmailRequest{To: to, Locale: "en",
		Category: domain.EmailAccount, Template: "support_customer_reply",
		Content:   mail.SupportCustomerReply(t.Reference, t.Name, body, consoleLink, reopened),
		DedupeKey: "support:" + t.ID.String() + ":customer:" + messageID.String()}))
}
