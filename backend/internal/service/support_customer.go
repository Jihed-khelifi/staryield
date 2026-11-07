package service

// revisions 3.0: support desk, requester side (section 16.2). A signed-in
// requester reaches their tickets by user_id only; an anonymous one through the
// private link (header X-Support-Token). Internal notes and system entries are
// never returned: the SQL itself excludes them.

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// CustomerReplyInput is a requester's reply.
type CustomerReplyInput struct {
	Body string `json:"body"`
}

func customerRow(t *domain.SupportTicket) CustomerTicketRow {
	return CustomerTicketRow{Reference: t.Reference, Subject: t.Subject, Topic: string(t.Topic), Status: string(t.Status),
		Awaiting: awaitingOf(t.Status), CreatedAt: t.CreatedAt, LastMessageAt: t.LastMessageAt, Kind: kindOf(t)}
}

// MyTickets lists the signed-in user's requests, latest activity first.
func (s *SupportService) MyTickets(ctx context.Context, userID uuid.UUID, cursor string, limit int) (Page[CustomerTicketRow], error) {
	page := Page[CustomerTicketRow]{Items: []CustomerTicketRow{}}
	c, err := decodeCursor(cursor)
	if err != nil {
		return page, err
	}
	size := clampPage(limit, supportMyPage, supportMyPageMax)
	rows, err := s.repo.ListForUser(ctx, userID, c, size+1)
	if err != nil {
		return page, apperror.Internal(err)
	}
	var reports []*domain.SupportTicket
	for i := range rows {
		if i == size {
			page.NextCursor = repository.EncodeCursor(rows[size-1].LastMessageAt, rows[size-1].ID.String())
			break
		}
		page.Items = append(page.Items, customerRow(&rows[i]))
		reports = append(reports, &rows[i])
	}
	if ctxs := s.customerReports(ctx, reports); len(ctxs) > 0 { // one batch per page
		for i := range page.Items {
			page.Items[i].Report = ctxs[reports[i].ID]
		}
	}
	return page, nil
}

// MyTicket returns one of the signed-in user's requests; another user's
// reference answers 404.
func (s *SupportService) MyTicket(ctx context.Context, userID uuid.UUID, reference string) (*CustomerTicket, error) {
	t, err := s.ownTicket(ctx, userID, reference)
	if err != nil {
		return nil, err
	}
	return s.customerView(ctx, t)
}

// MyReply adds a reply to one of the signed-in user's requests.
func (s *SupportService) MyReply(ctx context.Context, userID uuid.UUID, reference string, in CustomerReplyInput) (*CustomerTicket, error) {
	t, err := s.ownTicket(ctx, userID, reference)
	if err != nil {
		return nil, err
	}
	return s.customerReply(ctx, t, in)
}

func (s *SupportService) ownTicket(ctx context.Context, userID uuid.UUID, reference string) (*domain.SupportTicket, error) {
	ref := strings.ToUpper(strings.TrimSpace(reference))
	if !s.enabled() || userID == uuid.Nil || !supportRefRE.MatchString(ref) {
		return nil, apperror.NotFound("Support request")
	}
	t, err := s.repo.GetForUser(ctx, userID, ref)
	if err != nil {
		return nil, mapRepoErr(err, "Support request")
	}
	return t, nil
}

// linkTicket authenticates the private link. Every failure (unknown
// reference, missing, malformed, wrong or expired token) is the same 404, and
// the token comparison runs even when the reference is unknown.
func (s *SupportService) linkTicket(ctx context.Context, reference, token string) (*domain.SupportTicket, error) {
	notFound := apperror.NotFound("Support request")
	if !s.enabled() {
		return nil, notFound
	}
	ref := strings.ToUpper(strings.TrimSpace(reference))
	var t *domain.SupportTicket
	if supportRefRE.MatchString(ref) {
		if found, err := s.repo.GetByReference(ctx, ref); err == nil {
			t = found
		}
	}
	if !s.verifyToken(t, strings.TrimSpace(token), s.clock.Now()) {
		return nil, notFound
	}
	return t, nil
}

// LinkTicket returns a request through its private link.
func (s *SupportService) LinkTicket(ctx context.Context, reference, token string) (*CustomerTicket, error) {
	t, err := s.linkTicket(ctx, reference, token)
	if err != nil {
		return nil, err
	}
	return s.customerView(ctx, t)
}

// LinkReply adds a reply through the private link.
func (s *SupportService) LinkReply(ctx context.Context, reference, token string, in CustomerReplyInput) (*CustomerTicket, error) {
	t, err := s.linkTicket(ctx, reference, token)
	if err != nil {
		return nil, err
	}
	return s.customerReply(ctx, t, in)
}

// customerView builds the requester's view: the original request, then the
// public messages. Staff appear as "Staryield Support" plus their first name.
func (s *SupportService) customerView(ctx context.Context, t *domain.SupportTicket) (*CustomerTicket, error) {
	first, err := s.sealer.Open(t.MessageEnc, supportAD(t.ID, "message"))
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("support: open request: %w", err))
	}
	msgs, err := s.repo.Messages(ctx, t.ID, true, domain.MaxSupportMessages)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	staff := make([]uuid.UUID, 0, len(msgs))
	for _, m := range msgs {
		if m.AuthorType == domain.SupportAuthorStaff && m.AuthorID != nil {
			staff = append(staff, *m.AuthorID)
		}
	}
	names := map[uuid.UUID]string{}
	if len(staff) > 0 {
		if users, err := s.desk.Users.GetByIDs(ctx, staff); err == nil {
			for _, u := range users {
				names[u.ID] = u.DisplayName
			}
		}
	}
	out := &CustomerTicket{CustomerTicketRow: customerRow(t), CanReply: t.Status != domain.SupportClosed,
		Messages: make([]CustomerMessage, 0, len(msgs)+1)}
	out.Report = s.customerReports(ctx, []*domain.SupportTicket{t})[t.ID]
	// A report without details has no original text: the thread starts at the first reply.
	if first != "" || !isReport(t) {
		out.Messages = append(out.Messages, CustomerMessage{ID: t.ID, Author: MessageAuthor{Type: "customer", Name: t.Name},
			Body: first, CreatedAt: t.CreatedAt})
	}
	for _, m := range msgs {
		// Defence in depth: the query already excludes everything else.
		if m.Visibility != domain.SupportPublic || m.AuthorType == domain.SupportAuthorSystem {
			continue
		}
		body, err := s.sealer.Open(m.BodyEnc, supportMessageAD(m.ID))
		if err != nil {
			return nil, apperror.Internal(fmt.Errorf("support: open message: %w", err))
		}
		author := MessageAuthor{Type: "customer", Name: t.Name}
		if m.AuthorType == domain.SupportAuthorStaff {
			author = MessageAuthor{Type: "staff", Name: staffDisplay(names[derefID(m.AuthorID)])}
		}
		out.Messages = append(out.Messages, CustomerMessage{ID: m.ID, Author: author, Body: body, CreatedAt: m.CreatedAt})
	}
	return out, nil
}

// customerReply stores a requester's reply. It re-opens pending and resolved
// requests, marks them unread and e-mails the team. Closed requests refuse it.
func (s *SupportService) customerReply(ctx context.Context, t0 *domain.SupportTicket, in CustomerReplyInput) (*CustomerTicket, error) {
	body, err := validateSupportBody(in.Body)
	if err != nil {
		return nil, err
	}
	id := domain.NewID()
	sealed, err := s.sealer.Seal(body, supportMessageAD(id))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	now := s.clock.Now()
	msg := &domain.SupportMessage{ID: id, TicketID: t0.ID, AuthorType: domain.SupportAuthorCustomer, Visibility: domain.SupportPublic,
		BodyEnc: sealed, BodyChars: utf8.RuneCountInString(body), EmailStatus: domain.SupportEmailNone, CreatedAt: now}
	var t *domain.SupportTicket
	reopened := false
	err = s.desk.Tx.WithinTx(ctx, func(ctx context.Context) error {
		locked, err := s.repo.GetForUpdate(ctx, t0.ID)
		if err != nil {
			return err
		}
		if locked.Status == domain.SupportClosed {
			return apperror.InvalidState(msgSupportClosed)
		}
		if locked.MessageCount >= domain.MaxSupportMessages {
			return apperror.Conflict(apperror.CodeLimitReached, msgSupportLimit)
		}
		upd := map[string]any{"message_count": locked.MessageCount + 1, "last_message_at": now,
			"last_customer_message_at": now, "updated_at": now, "version": locked.Version + 1}
		if locked.Status != domain.SupportOpen {
			reopened = true
			for k, v := range statusFields(locked.Status, domain.SupportOpen, now) {
				upd[k] = v
			}
		}
		if err := s.repo.AddMessage(ctx, msg); err != nil {
			return err
		}
		if err := s.repo.Update(ctx, locked.ID, upd); err != nil {
			return err
		}
		t = locked
		return nil
	})
	if err != nil {
		return nil, mapRepoErr(err, "Support request")
	}
	s.invalidateSummary(ctx)
	if fresh, err := s.repo.Get(ctx, t.ID); err == nil {
		t = fresh
	}
	s.notifyTeam(ctx, t, msg.ID, body, reopened)
	return s.customerView(ctx, t)
}

// notifyTeam e-mails the assignee (or the support inbox) about a requester's
// reply, with the reference, the reply and a console link.
func (s *SupportService) notifyTeam(ctx context.Context, t *domain.SupportTicket, msgID uuid.UUID, body string, reopened bool) {
	to := s.desk.Settings.Inbox
	if t.AssigneeID != nil {
		if u, err := s.desk.Users.GetByID(ctx, *t.AssigneeID); err == nil && u.Status == domain.UserActive && u.Email != "" {
			to = u.Email
		}
	}
	s.notify.CustomerReplied(ctx, t, to, body, s.consoleLink(t.ID), msgID, reopened)
}
