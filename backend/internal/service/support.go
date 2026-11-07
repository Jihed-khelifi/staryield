package service

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// Contact-form validation messages (translated at the API edge).
const (
	msgSupportTopic   = "Choose a topic from the list."
	msgSupportSubject = "Write a subject between 3 and 120 characters."
	msgSupportMessage = "Describe your request in 10 to 5,000 characters."
	msgSupportConsent = "Please accept the terms so we can process your request."
	msgSupportName    = "Use at most 80 characters."
	msgSupportEmail   = "Enter a valid email address."
)

// referenceAlphabet avoids look-alike characters (0/O, 1/I) so references
// can be read out over the phone.
const referenceAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// TicketInput is a contact-form submission.
type TicketInput struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Topic   string `json:"topic"`
	Subject string `json:"subject"`
	Message string `json:"message"`
	Consent bool   `json:"consent"`
}

// TicketReceipt is what the sender gets back.
type TicketReceipt struct {
	Reference string    `json:"reference"`
	Topic     string    `json:"topic"`
	CreatedAt time.Time `json:"created_at"`
}

// SupportNotifier is told about ticket events: MailSupportNotifier e-mails
// the support inbox and acknowledges the request to the sender (with the
// private link), e-mails staff replies to the requester and requester replies
// to the team.
type SupportNotifier interface {
	// TicketCreated acknowledges a new request; link is the private link.
	TicketCreated(ctx context.Context, t *domain.SupportTicket, email, message, link string)
	// StaffReplied e-mails a public staff reply to the requester. It returns
	// an error when the e-mail could not be queued.
	StaffReplied(ctx context.Context, t *domain.SupportTicket, email, body, link string, messageID uuid.UUID) error
	// CustomerReplied e-mails a requester's reply to the team (to).
	CustomerReplied(ctx context.Context, t *domain.SupportTicket, to, body, consoleLink string, messageID uuid.UUID, reopened bool)
}

// NoopSupportNotifier discards notifications.
type NoopSupportNotifier struct{}

// TicketCreated implements SupportNotifier.
func (NoopSupportNotifier) TicketCreated(context.Context, *domain.SupportTicket, string, string, string) {
}

// StaffReplied implements SupportNotifier.
func (NoopSupportNotifier) StaffReplied(context.Context, *domain.SupportTicket, string, string, string, uuid.UUID) error {
	return nil
}

// CustomerReplied implements SupportNotifier.
func (NoopSupportNotifier) CustomerReplied(context.Context, *domain.SupportTicket, string, string, string, uuid.UUID, bool) {
}

// SupportService stores contact-form requests and runs the support desk
// (admin queue, requester views, replies, maintenance).
type SupportService struct {
	repo     repository.SupportRepository
	sealer   *security.Sealer
	notify   SupportNotifier
	clock    clock.Clock
	ipSecret string
	desk     SupportDeskDeps // revisions 3.0: support desk (see WithDesk)
}

// NewSupportService wires the support service.
func NewSupportService(repo repository.SupportRepository, sealer *security.Sealer, notify SupportNotifier, clk clock.Clock, ipSecret string) *SupportService {
	if notify == nil {
		notify = NoopSupportNotifier{}
	}
	return &SupportService{repo: repo, sealer: sealer, notify: notify, clock: clk, ipSecret: ipSecret}
}

// supportAD binds sealed ticket fields to their row.
func supportAD(id uuid.UUID, field string) string { return "support:" + id.String() + ":" + field }

// newSupportReference returns "SY-" and six unambiguous characters.
func newSupportReference() (string, error) {
	var b strings.Builder
	b.WriteString("SY-")
	max := big.NewInt(int64(len(referenceAlphabet)))
	for i := 0; i < 6; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b.WriteByte(referenceAlphabet[n.Int64()])
	}
	return b.String(), nil
}

type cleanTicket struct {
	name, email, subject, message string
	topic                         domain.SupportTopic
}

// validateTicket normalises a submission and reports every problem at once.
func validateTicket(in TicketInput) (cleanTicket, error) {
	fields := map[string]string{}
	var out cleanTicket
	out.name = CleanLine(in.Name)
	if utf8.RuneCountInString(out.name) > 80 {
		fields["name"] = msgSupportName
	}
	if email, ok := NormalizeEmail(in.Email); ok {
		out.email = email
	} else {
		fields["email"] = msgSupportEmail
	}
	out.topic = domain.SupportTopic(strings.ToLower(strings.TrimSpace(in.Topic)))
	if out.topic == "" {
		out.topic = domain.SupportGeneral
	}
	if !domain.ValidSupportTopic(out.topic) {
		fields["topic"] = msgSupportTopic
	}
	out.subject = CleanLine(in.Subject)
	if n := utf8.RuneCountInString(out.subject); n < 3 || n > 120 {
		fields["subject"] = msgSupportSubject
	}
	out.message = strings.TrimSpace(CleanText(in.Message))
	if n := utf8.RuneCountInString(out.message); n < 10 || n > 5000 {
		fields["message"] = msgSupportMessage
	}
	if !in.Consent {
		fields["consent"] = msgSupportConsent
	}
	if len(fields) > 0 {
		return out, apperror.Validation(fields)
	}
	return out, nil
}

// ticketDraft is what a new ticket is built from: the contact form and the
// conversation reports share the construction (sealing, hashes, mask, link).
type ticketDraft struct {
	name, email, subject, message string
	topic                         domain.SupportTopic
	priority                      domain.SupportPriority
	kind                          domain.TicketKind
}

// buildTicket assembles a new open ticket from a draft: the address and the
// message are sealed, the address and the IP keyed-hashed, the address masked
// for the queue, and a private-link token issued (returned once, for the link).
func (s *SupportService) buildTicket(ctx context.Context, p reqctx.Principal, d ticketDraft) (*domain.SupportTicket, string, error) {
	meta := reqctx.MetaFrom(ctx)
	now := s.clock.Now()
	if d.priority == "" {
		d.priority = domain.SupportNormal
	}
	if d.kind == "" {
		d.kind = domain.TicketContact
	}
	t := &domain.SupportTicket{ID: domain.NewID(), Name: d.name, Topic: d.topic, Subject: d.subject,
		MessageChars: utf8.RuneCountInString(d.message), Locale: normalizeLocale(meta.Locale), Status: domain.SupportOpen,
		EmailHash: security.Keyed(s.ipSecret, "support-email:"+d.email, 0), IPHash: security.Keyed(s.ipSecret, meta.IP, 32),
		CreatedAt: now, UpdatedAt: now,
		// revisions 3.0: support desk
		Priority: d.priority, Version: 1, MessageCount: 1, LastMessageAt: now, LastCustomerMessageAt: now,
		EmailMasked: MaskEmail(d.email), Kind: d.kind}
	token, err := s.issueToken(t, now)
	if err != nil {
		return nil, "", err
	}
	if p.UserID != uuid.Nil {
		id := p.UserID
		t.UserID = &id
	}
	if t.EmailEnc, err = s.sealer.Seal(d.email, supportAD(t.ID, "email")); err != nil {
		return nil, "", err
	}
	if t.MessageEnc, err = s.sealer.Seal(d.message, supportAD(t.ID, "message")); err != nil {
		return nil, "", err
	}
	return t, token, nil
}

// insertTicket stores t under a fresh reference. A reference collision is
// astronomically rare; it retries a few times. Other repository errors (such
// as repository.ErrDuplicateReport) are returned as they are.
func (s *SupportService) insertTicket(ctx context.Context, t *domain.SupportTicket) error {
	var err error
	for attempt := 0; ; attempt++ {
		if t.Reference, err = newSupportReference(); err != nil {
			return err
		}
		err = s.repo.Create(ctx, t)
		if !errors.Is(err, repository.ErrDuplicateReference) || attempt == 4 {
			return err
		}
	}
}

// Create stores a ticket. Signed-in senders are linked to their account.
func (s *SupportService) Create(ctx context.Context, p reqctx.Principal, in TicketInput) (*TicketReceipt, error) {
	clean, err := validateTicket(in)
	if err != nil {
		return nil, err
	}
	t, token, err := s.buildTicket(ctx, p, ticketDraft{name: clean.name, email: clean.email, subject: clean.subject,
		message: clean.message, topic: clean.topic})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if err := s.insertTicket(ctx, t); err != nil {
		return nil, apperror.Internal(err)
	}
	s.invalidateSummary(ctx) // revisions 3.0: support desk
	s.notify.TicketCreated(ctx, t, clean.email, clean.message, s.ticketLink(t.Locale, t.Reference, token))
	return &TicketReceipt{Reference: t.Reference, Topic: string(t.Topic), CreatedAt: t.CreatedAt}, nil
}
