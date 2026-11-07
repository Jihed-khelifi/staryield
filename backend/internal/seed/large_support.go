package seed

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// revisions 3.0: support desk. Load-test tickets (about one for every two
// clients, 100k with the default profile) with a realistic mix of statuses,
// priorities, assignees, topics and subjects, plus a short sealed thread on
// the busier ones, so the admin queue, its filters, search and the
// requester's own list are exercised at size.
func init() {
	RegisterLarge(Generator{Name: "support tickets", Run: func(_ context.Context, d *Dataset) error { return d.supportTickets() }})
}

const supportRefAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

var supportSubjects = []string{"Refund for a double charge", "Cannot log in to my account", "Question about my credits", "Advisor did not reply",
	"Export my personal data", "Delete my account", "Chat disconnected during my reading", "Update my payment method", "Membership renewal question",
	"Privacy question about my birth data", "Technical problem with the chat", "Invoice for my last purchase", "Change my e-mail address",
	"Review was not published", "Promo code does not work"}

var supportBodies = []string{"Hello, I need help with this as soon as possible.", "Thank you for looking into it, I will wait for your answer.",
	"This happened again today, can you check what is going on?", "We checked your account and everything looks fine on our side.",
	"Could you send a screenshot of the error you are seeing?", "I have refunded the amount, it will appear within a few days."}

// supportTickets generates the tickets. They need the sealer (addresses and
// bodies are sealed like the API does), so the step is skipped without one.
func (d *Dataset) supportTickets() error {
	if d.Sealer == nil {
		d.Log.Warn("support tickets skipped: no sealer configured")
		return nil
	}
	if len(d.Clients) == 0 {
		return nil
	}
	var admins []uuid.UUID
	if err := d.DB.Model(&domain.User{}).Where("role = ? AND status = ?", domain.RoleAdmin, domain.UserActive).Limit(10).Pluck("id", &admins).Error; err != nil {
		return err
	}
	total := len(d.Clients) / 2
	if total < 100 {
		total = 100
	}
	if total > 250_000 {
		total = 250_000
	}
	refs := make(map[string]bool, total)
	tickets := make([]domain.SupportTicket, 0, d.Opts.BatchSize)
	messages := make([]domain.SupportMessage, 0, d.Opts.BatchSize)
	flush := func() error {
		if err := d.insert("support_tickets", &tickets, len(tickets), true); err != nil {
			return err
		}
		tickets = tickets[:0]
		err := d.insert("support_messages", &messages, len(messages), true)
		messages = messages[:0]
		return err
	}
	topics := []domain.SupportTopic{domain.SupportGeneral, domain.SupportPrivacy, domain.SupportBilling, domain.SupportAdvisor, domain.SupportTechnical}
	locales := []string{"en", "fr", "es"}
	for i := 0; i < total; i++ {
		created := d.Past()
		id := d.ID(created)
		var ref string
		for ref == "" || refs[ref] {
			var b strings.Builder
			b.WriteString("SY-")
			for k := 0; k < 6; k++ {
				b.WriteByte(supportRefAlphabet[d.Rand.IntN(len(supportRefAlphabet))])
			}
			ref = b.String()
		}
		refs[ref] = true
		email := "ticket-" + d.Tag + "-" + id.String()[:8] + "@" + LoadEmailDomain
		subject := supportSubjects[d.Rand.IntN(len(supportSubjects))]
		body := supportBodies[d.Rand.IntN(len(supportBodies))] + " Reference " + ref + "."
		t := domain.SupportTicket{ID: id, Reference: ref, Name: firstNames[d.Rand.IntN(len(firstNames))], Topic: topics[d.Rand.IntN(len(topics))],
			Subject: subject, MessageChars: utf8.RuneCountInString(body), Locale: locales[d.pick(70, 15, 15)],
			EmailHash: security.Keyed(LoadPassword, "support-email:"+email, 0), IPHash: security.Keyed(LoadPassword, id.String(), 32),
			CreatedAt: created, UpdatedAt: created, Priority: domain.SupportPriority([]string{"low", "normal", "high", "urgent"}[d.pick(15, 60, 18, 7)]),
			Version: 1, MessageCount: 1, LastMessageAt: created, LastCustomerMessageAt: created, EmailMasked: "t•••@l•••.test"}
		t.EmailEnc, _ = d.Sealer.Seal(email, "support:"+id.String()+":email")
		t.MessageEnc, _ = d.Sealer.Seal(body, "support:"+id.String()+":message")
		if d.Rand.Float64() < 0.2 { // linked to an account
			u := d.Clients[d.Rand.IntN(len(d.Clients))]
			t.UserID = &u
		}
		status := []domain.SupportStatus{domain.SupportOpen, domain.SupportPending, domain.SupportResolved, domain.SupportClosed}[d.pick(30, 20, 20, 30)]
		t.Status = status
		if status != domain.SupportOpen && d.Rand.Float64() < 0.8 && len(admins) > 0 {
			a := admins[d.Rand.IntN(len(admins))]
			t.AssigneeID = &a
		}
		d.supportThread(&t, status, admins, &messages)
		if d.Rand.Float64() < 0.6 {
			seen := d.After(created, 48*time.Hour)
			t.StaffSeenAt = &seen
		}
		tickets = append(tickets, t)
		if len(tickets) == cap(tickets) {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := flush(); err != nil {
		return err
	}
	d.Log.Info("support tickets generated", zap.Int("tickets", d.Counts["support_tickets"]), zap.Int("messages", d.Counts["support_messages"]))
	return nil
}

// supportThread gives a ticket that was answered a short sealed thread and
// stamps its resolution or closing time (open tickets stay as created).
func (d *Dataset) supportThread(t *domain.SupportTicket, status domain.SupportStatus, admins []uuid.UUID, out *[]domain.SupportMessage) {
	created, id := t.CreatedAt, t.ID
	// A short thread on tickets that were answered.
	last := created
	if status != domain.SupportOpen {
		for n, replies := 0, 1+d.Rand.IntN(3); n < replies && t.MessageCount < domain.MaxSupportMessages; n++ {
			last = d.After(last, 72*time.Hour)
			staff := n%2 == 0
			m := domain.SupportMessage{ID: d.ID(last), TicketID: id, AuthorType: domain.SupportAuthorCustomer, Visibility: domain.SupportPublic,
				EmailStatus: domain.SupportEmailNone, CreatedAt: last}
			text := supportBodies[d.Rand.IntN(len(supportBodies))]
			if staff {
				m.AuthorType, m.EmailStatus = domain.SupportAuthorStaff, domain.SupportEmailSent
				if len(admins) > 0 {
					m.AuthorID = &admins[d.Rand.IntN(len(admins))]
				}
				t.LastStaffMessageAt = &last
				if t.FirstResponseAt == nil {
					first := last
					t.FirstResponseAt = &first
				}
			} else {
				t.LastCustomerMessageAt = last
			}
			m.BodyEnc, _ = d.Sealer.Seal(text, "support-message:"+m.ID.String())
			m.BodyChars = utf8.RuneCountInString(text)
			*out = append(*out, m)
			t.MessageCount++
			t.LastMessageAt = last
		}
		t.UpdatedAt = t.LastMessageAt
	}
	switch status {
	case domain.SupportResolved:
		at := d.After(last, 0)
		t.ResolvedAt = &at
		t.UpdatedAt = at
	case domain.SupportClosed:
		at := d.After(last, 0)
		t.ClosedAt = &at
		t.UpdatedAt = at
	}
}
