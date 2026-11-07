package service

// revisions 3.0: support desk, administrator side (section 16.1). The routes
// are admin-only; reading a ticket and replying need a fresh step-up.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// SupportQuery is the admin queue's filter, as sent in the query string.
type SupportQuery struct {
	Status   string // comma list
	Topic    string
	Priority string
	Assignee string // me | unassigned | <uuid>
	UserID   string
	Q        string
	Sort     string // newest | oldest | waiting
	Cursor   string
	Limit    int
}

// AdminReplyInput is a reply or an internal note.
type AdminReplyInput struct {
	Body       string  `json:"body"`
	Visibility string  `json:"visibility"`
	Status     *string `json:"status"`
}

// AdminUpdateInput changes a ticket's workflow fields. AssigneeID null
// unassigns; absent leaves it alone.
type AdminUpdateInput struct {
	Status          *string          `json:"status"`
	Priority        *string          `json:"priority"`
	AssigneeID      Optional[string] `json:"assignee_id"`
	ExpectedVersion *int             `json:"expected_version"`
}

func (s *SupportService) record(ctx context.Context, action string, t *domain.SupportTicket, meta map[string]any) {
	if s.desk.Audit == nil {
		return
	}
	if meta == nil {
		meta = map[string]any{}
	}
	meta["reference"] = t.Reference
	recordAudit(ctx, s.desk.Audit, s.clock, s.ipSecret, action, "support_ticket", t.ID.String(), meta)
}

// ---------------------------------------------------------------------------
// Queue
// ---------------------------------------------------------------------------

func parseStatusList(raw string) ([]domain.SupportStatus, error) {
	var out []domain.SupportStatus
	seen := map[domain.SupportStatus]bool{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		st := domain.SupportStatus(part)
		if !domain.ValidSupportStatus(st) {
			return nil, apperror.Validation(map[string]string{"status": msgSupportStatus})
		}
		if !seen[st] {
			seen[st] = true
			out = append(out, st)
		}
	}
	return out, nil
}

func onlyOpenStatuses(st []domain.SupportStatus) bool {
	if len(st) == 0 {
		return false
	}
	for _, s := range st {
		if s != domain.SupportOpen {
			return false
		}
	}
	return true
}

// buildFilter validates a query and turns it into a repository filter. q is a
// reference (exact, any case), an e-mail address (exact, through the keyed
// hash) or words of the subject.
func (s *SupportService) buildFilter(adminID uuid.UUID, q SupportQuery) (repository.SupportAdminFilter, error) {
	fields := map[string]string{}
	var f repository.SupportAdminFilter
	var err error
	if f.Statuses, err = parseStatusList(q.Status); err != nil {
		fields["status"] = msgSupportStatus
	}
	if t := strings.ToLower(strings.TrimSpace(q.Topic)); t != "" {
		if !domain.ValidSupportTopic(domain.SupportTopic(t)) {
			fields["topic"] = msgSupportTopic
		}
		f.Topic = t
	}
	if p := strings.ToLower(strings.TrimSpace(q.Priority)); p != "" {
		if !domain.ValidSupportPriority(domain.SupportPriority(p)) {
			fields["priority"] = msgSupportPriority
		}
		f.Priority = p
	}
	switch a := strings.ToLower(strings.TrimSpace(q.Assignee)); a {
	case "":
	case "me":
		f.AssigneeID = &adminID
	case "unassigned":
		f.Unassigned = true
	default:
		if id, perr := uuid.Parse(a); perr == nil {
			f.AssigneeID = &id
		} else {
			fields["assignee"] = msgSupportAssigneeList
		}
	}
	if u := strings.TrimSpace(q.UserID); u != "" {
		if id, perr := uuid.Parse(u); perr == nil {
			f.UserID = &id
		} else {
			fields["user_id"] = "Invalid identifier."
		}
	}
	switch sort := strings.ToLower(strings.TrimSpace(q.Sort)); sort {
	case "", repository.SupportSortNewest:
		f.Sort = repository.SupportSortNewest
	case repository.SupportSortOldest, repository.SupportSortWaiting:
		f.Sort = sort
	default:
		fields["sort"] = msgSupportSort
	}
	if text := CleanLine(q.Q); text != "" {
		if utf8.RuneCountInString(text) > supportMaxQuery {
			fields["q"] = msgSupportQuery
		} else {
			f.Reference, f.EmailHash, f.Words = s.classifyQuery(text)
		}
	}
	if len(fields) > 0 {
		return f, apperror.Validation(fields)
	}
	return f, nil
}

// classifyQuery decides what a search box entry is.
func (s *SupportService) classifyQuery(text string) (reference, emailHash, words string) {
	if ref := strings.ToUpper(text); supportRefRE.MatchString(ref) {
		return ref, "", ""
	}
	if strings.Contains(text, "@") {
		if email, ok := NormalizeEmail(text); ok {
			return "", security.Keyed(s.ipSecret, "support-email:"+email, 0), ""
		}
	}
	return "", "", text
}

// AdminTickets pages the queue (keyset, newest first unless sorted).
func (s *SupportService) AdminTickets(ctx context.Context, adminID uuid.UUID, q SupportQuery) (Page[AdminTicketRow], error) {
	empty := Page[AdminTicketRow]{Items: []AdminTicketRow{}}
	if !s.enabled() {
		return empty, nil
	}
	f, err := s.buildFilter(adminID, q)
	if err != nil {
		return empty, err
	}
	mixed := f.Sort == repository.SupportSortWaiting && !onlyOpenStatuses(f.Statuses)
	if f.Cursor, err = decodeSupportCursor(q.Cursor, f.Sort, mixed); err != nil {
		return empty, err
	}
	size := clampPage(q.Limit, supportAdminPage, supportAdminPageMax)
	f.Limit = size + 1
	rows, err := s.repo.AdminList(ctx, f)
	if err != nil {
		return empty, apperror.Internal(err)
	}
	page := empty
	for i := range rows {
		if i == size {
			page.NextCursor = encodeSupportCursor(f.Sort, &rows[size-1].SupportTicket, mixed)
			break
		}
		page.Items = append(page.Items, adminRow(&rows[i]))
	}
	return page, nil
}

func adminCommon(t *domain.SupportTicket, assigneeName string) adminTicketCommon {
	c := adminTicketCommon{ID: t.ID, Reference: t.Reference, Subject: t.Subject, Topic: string(t.Topic), Status: string(t.Status),
		Priority: string(t.Priority), Locale: t.Locale, MessageCount: t.MessageCount, NeedsReply: t.Status == domain.SupportOpen,
		Unread: unreadTicket(t), CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, LastMessageAt: t.LastMessageAt,
		FirstResponseAt: t.FirstResponseAt, ResolvedAt: t.ResolvedAt, Version: t.Version}
	if t.AssigneeID != nil {
		c.Assignee = &AdminAssignee{ID: *t.AssigneeID, Name: assigneeName}
	}
	return c
}

func maskedOf(t *domain.SupportTicket) string {
	if t.EmailMasked == "" {
		return "•••"
	}
	return t.EmailMasked
}

func adminRow(r *repository.SupportAdminRow) AdminTicketRow {
	name := r.Name
	if name == "" {
		name = r.UserName
	}
	return AdminTicketRow{adminTicketCommon: adminCommon(&r.SupportTicket, r.AssigneeName),
		Requester: AdminRequester{UserID: r.UserID, Name: name, EmailMasked: maskedOf(&r.SupportTicket)}}
}

// ---------------------------------------------------------------------------
// Summary (cached 15 s, invalidated on every write)
// ---------------------------------------------------------------------------

const supportGenKey = "support:generation"

// invalidateSummary makes the next summary read fresh: the cache key carries a
// generation number that every write increments.
func (s *SupportService) invalidateSummary(ctx context.Context) {
	if s.desk.Cache == nil {
		return
	}
	if _, err := s.desk.Cache.Incr(database.Detach(ctx), supportGenKey, 24*3600*1e9); err != nil {
		warnErr(ctx, "support: invalidate summary", err)
	}
}

// AdminSummary returns the queue counters ("mine" is per administrator).
func (s *SupportService) AdminSummary(ctx context.Context, adminID uuid.UUID) (*SupportSummary, error) {
	if !s.enabled() {
		return &SupportSummary{}, nil
	}
	key := ""
	if s.desk.Cache != nil {
		gen, _, err := s.desk.Cache.Get(ctx, supportGenKey)
		if err == nil {
			key = "support:summary:" + gen + ":" + adminID.String()
			if raw, ok, err := s.desk.Cache.Get(ctx, key); err == nil && ok {
				var out SupportSummary
				if json.Unmarshal([]byte(raw), &out) == nil {
					return &out, nil
				}
			}
		}
	}
	c, err := s.repo.Summary(ctx, adminID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := &SupportSummary{Open: c.Open, Pending: c.Pending, Resolved: c.Resolved, Closed: c.Closed,
		Unassigned: c.Unassigned, Mine: c.Mine, NeedsReply: c.Open}
	if key != "" {
		if b, err := json.Marshal(out); err == nil {
			warnErr(ctx, "support: cache summary", s.desk.Cache.Set(ctx, key, string(b), supportSummaryTTL))
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Detail
// ---------------------------------------------------------------------------

func (s *SupportService) resolveTicket(ctx context.Context, key string) (*domain.SupportTicket, error) {
	id, ref, err := parseTicketKey(key)
	if err != nil {
		return nil, err
	}
	var t *domain.SupportTicket
	if ref != "" {
		t, err = s.repo.GetByReference(ctx, ref)
	} else {
		t, err = s.repo.Get(ctx, id)
	}
	if err != nil {
		return nil, mapRepoErr(err, "Support request")
	}
	return t, nil
}

// AdminTicketDetail decrypts a ticket and its thread. The route requires a
// fresh step-up; viewing is audited at most once per administrator and ticket
// every 10 minutes, and marks the ticket as seen by the team.
func (s *SupportService) AdminTicketDetail(ctx context.Context, adminID uuid.UUID, key string) (*AdminTicket, error) {
	if !s.enabled() {
		return nil, apperror.NotFound("Support request")
	}
	t, err := s.resolveTicket(ctx, key)
	if err != nil {
		return nil, err
	}
	s.auditViewed(ctx, adminID, t)
	if unreadTicket(t) {
		now := s.clock.Now()
		if err := s.repo.Update(ctx, t.ID, map[string]any{"staff_seen_at": now}); err != nil {
			return nil, mapRepoErr(err, "Support request")
		}
		t.StaffSeenAt = &now
		s.invalidateSummary(ctx)
	}
	return s.adminView(ctx, t)
}

// auditViewed records support_ticket.viewed, throttled with a Redis key. If
// the throttle cannot be read the view is audited (more audit, never less).
func (s *SupportService) auditViewed(ctx context.Context, adminID uuid.UUID, t *domain.SupportTicket) {
	if s.desk.Cache != nil {
		n, err := s.desk.Cache.Incr(ctx, "support:viewed:"+adminID.String()+":"+t.ID.String(), supportViewedTTL)
		if err == nil && n > 1 {
			return
		}
	}
	s.record(ctx, "support_ticket.viewed", t, nil)
}

// adminView builds the decrypted detail of t.
func (s *SupportService) adminView(ctx context.Context, t *domain.SupportTicket) (*AdminTicket, error) {
	email, err := s.sealer.Open(t.EmailEnc, supportAD(t.ID, "email"))
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("support: open address: %w", err))
	}
	first, err := s.sealer.Open(t.MessageEnc, supportAD(t.ID, "message"))
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("support: open request: %w", err))
	}
	msgs, err := s.repo.Messages(ctx, t.ID, false, domain.MaxSupportMessages)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	ids := make([]uuid.UUID, 0, len(msgs)+2)
	for _, m := range msgs {
		if m.AuthorID != nil {
			ids = append(ids, *m.AuthorID)
		}
	}
	if t.AssigneeID != nil {
		ids = append(ids, *t.AssigneeID)
	}
	if t.UserID != nil {
		ids = append(ids, *t.UserID)
	}
	people := map[uuid.UUID]domain.User{}
	if len(ids) > 0 {
		users, err := s.desk.Users.GetByIDs(ctx, ids)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		for _, u := range users {
			people[u.ID] = u
		}
	}
	customer := t.Name
	if customer == "" && t.UserID != nil {
		customer = people[*t.UserID].DisplayName
	}
	if customer == "" {
		customer = "Customer"
	}
	out := &AdminTicket{adminTicketCommon: adminCommon(t, people[derefID(t.AssigneeID)].DisplayName),
		Requester: AdminRequesterFull{AdminRequester: AdminRequester{UserID: t.UserID, Name: t.Name, EmailMasked: maskedOf(t)}, Email: email},
		Messages:  make([]SupportMessageView, 0, len(msgs)+1)}
	if out.Requester.Name == "" && t.UserID != nil {
		out.Requester.Name = people[*t.UserID].DisplayName
	}
	if t.UserID != nil {
		if u, ok := people[*t.UserID]; ok {
			out.User = &AdminTicketUser{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, Role: string(u.Role), Status: string(u.Status)}
		}
	}
	out.Messages = append(out.Messages, SupportMessageView{ID: t.ID, Author: MessageAuthor{Type: "customer", Name: customer},
		Visibility: "public", Body: first, CreatedAt: t.CreatedAt})
	for _, m := range msgs {
		body, err := s.sealer.Open(m.BodyEnc, supportMessageAD(m.ID))
		if err != nil {
			return nil, apperror.Internal(fmt.Errorf("support: open message: %w", err))
		}
		v := SupportMessageView{ID: m.ID, Visibility: string(m.Visibility), Body: body, CreatedAt: m.CreatedAt}
		switch m.AuthorType {
		case domain.SupportAuthorCustomer:
			v.Author = MessageAuthor{Type: "customer", Name: customer}
		case domain.SupportAuthorStaff:
			name := "Staff"
			if m.AuthorID != nil {
				if u, ok := people[*m.AuthorID]; ok && u.DisplayName != "" {
					name = u.DisplayName
				}
			}
			v.Author = MessageAuthor{Type: "staff", Name: name}
			if m.EmailStatus != domain.SupportEmailNone {
				v.EmailStatus = string(m.EmailStatus)
			}
		default:
			v.Author = MessageAuthor{Type: "system", Name: supportBrand}
		}
		out.Messages = append(out.Messages, v)
	}
	return out, nil
}

func derefID(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return *id
}

func supportMessageAD(id uuid.UUID) string { return "support-message:" + id.String() }

// ---------------------------------------------------------------------------
// Reply and note
// ---------------------------------------------------------------------------

// AdminReply adds a public reply (e-mailed to the requester with the private
// link) or an internal note. Closed tickets refuse it.
func (s *SupportService) AdminReply(ctx context.Context, adminID uuid.UUID, key string, in AdminReplyInput) (*AdminTicket, error) {
	if !s.enabled() {
		return nil, apperror.NotFound("Support request")
	}
	fields := map[string]string{}
	body, berr := validateSupportBody(in.Body)
	if berr != nil {
		fields["body"] = msgSupportReplyBody
	}
	vis := domain.SupportVisibility(strings.ToLower(strings.TrimSpace(in.Visibility)))
	if vis != domain.SupportPublic && vis != domain.SupportInternal {
		fields["visibility"] = msgSupportVisibility
	}
	var target *domain.SupportStatus
	if in.Status != nil {
		st := domain.SupportStatus(strings.ToLower(strings.TrimSpace(*in.Status)))
		if !domain.ValidSupportStatus(st) {
			fields["status"] = msgSupportStatus
		}
		target = &st
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	t0, err := s.resolveTicket(ctx, key)
	if err != nil {
		return nil, err
	}
	sealedID := domain.NewID()
	sealed, err := s.sealer.Seal(body, supportMessageAD(sealedID))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	now := s.clock.Now()
	msg := &domain.SupportMessage{ID: sealedID, TicketID: t0.ID, AuthorType: domain.SupportAuthorStaff, AuthorID: &adminID,
		Visibility: vis, BodyEnc: sealed, BodyChars: utf8.RuneCountInString(body), EmailStatus: domain.SupportEmailNone, CreatedAt: now}
	var t *domain.SupportTicket
	var changed bool
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
		next := locked.Status
		switch {
		case target != nil:
			next = *target
		case vis == domain.SupportPublic:
			next = domain.SupportPending
		}
		upd := map[string]any{"message_count": locked.MessageCount + 1, "last_message_at": now, "staff_seen_at": now,
			"updated_at": now, "version": locked.Version + 1}
		for k, v := range statusFields(locked.Status, next, now) {
			upd[k] = v
		}
		if vis == domain.SupportPublic {
			upd["last_staff_message_at"] = now
			if locked.FirstResponseAt == nil {
				upd["first_response_at"] = now
			}
			if next == domain.SupportResolved && locked.Status == domain.SupportResolved {
				upd["resolved_at"] = now // answering a resolved request restarts the auto-close window
			}
		}
		if err := s.repo.AddMessage(ctx, msg); err != nil {
			return err
		}
		if err := s.repo.Update(ctx, locked.ID, upd); err != nil {
			return err
		}
		changed = next != locked.Status
		t = locked
		return nil
	})
	if err != nil {
		return nil, mapRepoErr(err, "Support request")
	}
	s.invalidateSummary(ctx)
	// Reload for the committed values (and the token columns).
	if fresh, err := s.repo.Get(ctx, t.ID); err == nil {
		t = fresh
	}
	action := "support_ticket.noted"
	if vis == domain.SupportPublic {
		action = "support_ticket.replied"
		s.deliverReply(ctx, t, msg, body)
	}
	s.record(ctx, action, t, map[string]any{"message_id": msg.ID.String(), "visibility": string(vis), "status": string(t.Status),
		"status_changed": changed})
	return s.adminView(ctx, t)
}

// deliverReply e-mails a public reply and notifies a signed-in requester.
func (s *SupportService) deliverReply(ctx context.Context, t *domain.SupportTicket, msg *domain.SupportMessage, body string) {
	status := domain.SupportEmailFailed
	if email, err := s.sealer.Open(t.EmailEnc, supportAD(t.ID, "email")); err == nil {
		link := s.linkFor(ctx, t)
		if link != "" {
			if err := s.notify.StaffReplied(ctx, t, email, body, link, msg.ID); err == nil {
				status = domain.SupportEmailQueued
			} else {
				warnErr(ctx, "support: e-mail reply", err)
			}
		}
	}
	warnErr(ctx, "support: e-mail status", s.repo.SetMessageEmailStatus(database.Detach(ctx), msg.ID, status))
	if t.UserID != nil {
		s.desk.Notifier.Notify(database.Detach(ctx), NotificationSpec{Kind: NotifySupportReplied,
			Data: map[string]any{"reference": t.Reference, "subject": t.Subject}, Link: "/support/requests/" + t.Reference}, *t.UserID)
	}
}

// ---------------------------------------------------------------------------
// Update (status, priority, assignee)
// ---------------------------------------------------------------------------

func supportVersionConflict(current int) *apperror.Error {
	return apperror.New(apperror.CodeVersionConflict, 409, msgSupportVersion).WithDetails(map[string]any{"current_version": current})
}

// checkAssignee makes sure the assignee is an active administrator.
func (s *SupportService) checkAssignee(ctx context.Context, raw string) (uuid.UUID, error) {
	bad := apperror.Validation(map[string]string{"assignee_id": msgSupportAssignee})
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return uuid.Nil, apperror.Validation(map[string]string{"assignee_id": msgSupportAssigneeID})
	}
	u, err := s.desk.Users.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return uuid.Nil, bad
		}
		return uuid.Nil, apperror.Internal(err)
	}
	if u.Role != domain.RoleAdmin || u.Status != domain.UserActive {
		return uuid.Nil, bad
	}
	return id, nil
}

// AdminUpdate changes status, priority and assignee under optimistic
// concurrency (expected_version) and audits every change with before/after.
func (s *SupportService) AdminUpdate(ctx context.Context, adminID uuid.UUID, key string, in AdminUpdateInput) (*AdminTicketRow, error) {
	if !s.enabled() {
		return nil, apperror.NotFound("Support request")
	}
	fields := map[string]string{}
	var status *domain.SupportStatus
	if in.Status != nil {
		st := domain.SupportStatus(strings.ToLower(strings.TrimSpace(*in.Status)))
		if !domain.ValidSupportStatus(st) {
			fields["status"] = msgSupportStatus
		}
		status = &st
	}
	var priority *domain.SupportPriority
	if in.Priority != nil {
		p := domain.SupportPriority(strings.ToLower(strings.TrimSpace(*in.Priority)))
		if !domain.ValidSupportPriority(p) {
			fields["priority"] = msgSupportPriority
		}
		priority = &p
	}
	var assignee *uuid.UUID
	if in.AssigneeID.Set && in.AssigneeID.Value != nil {
		id, err := s.checkAssignee(ctx, *in.AssigneeID.Value)
		if err != nil {
			if ae, ok := apperror.As(err); ok && ae.Code == apperror.CodeValidation {
				fields["assignee_id"] = msgSupportAssignee
				if _, perr := uuid.Parse(strings.TrimSpace(*in.AssigneeID.Value)); perr != nil {
					fields["assignee_id"] = msgSupportAssigneeID
				}
			} else {
				return nil, err
			}
		}
		assignee = &id
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	t0, err := s.resolveTicket(ctx, key)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	changes := map[string]any{}
	var t *domain.SupportTicket
	err = s.desk.Tx.WithinTx(ctx, func(ctx context.Context) error {
		locked, err := s.repo.GetForUpdate(ctx, t0.ID)
		if err != nil {
			return err
		}
		if in.ExpectedVersion != nil && *in.ExpectedVersion != locked.Version {
			return supportVersionConflict(locked.Version)
		}
		upd := map[string]any{}
		if status != nil && *status != locked.Status {
			changes["status"] = map[string]any{"from": locked.Status, "to": *status}
			for k, v := range statusFields(locked.Status, *status, now) {
				upd[k] = v
			}
		}
		if priority != nil && *priority != locked.Priority {
			changes["priority"] = map[string]any{"from": locked.Priority, "to": *priority}
			upd["priority"] = *priority
		}
		if in.AssigneeID.Set && !sameID(locked.AssigneeID, assignee) {
			changes["assignee_id"] = map[string]any{"from": idString(locked.AssigneeID), "to": idString(assignee)}
			upd["assignee_id"] = assignee
		}
		t = locked
		if len(upd) == 0 {
			return nil
		}
		upd["version"], upd["updated_at"] = locked.Version+1, now
		return s.repo.Update(ctx, locked.ID, upd)
	})
	if err != nil {
		return nil, mapRepoErr(err, "Support request")
	}
	if len(changes) > 0 {
		s.invalidateSummary(ctx)
		s.record(ctx, "support_ticket.updated", t, map[string]any{"changes": changes})
	}
	fresh, err := s.repo.Get(ctx, t.ID)
	if err != nil {
		return nil, mapRepoErr(err, "Support request")
	}
	row := AdminTicketRow{adminTicketCommon: adminCommon(fresh, ""),
		Requester: AdminRequester{UserID: fresh.UserID, Name: fresh.Name, EmailMasked: maskedOf(fresh)}}
	ids := []uuid.UUID{}
	if fresh.AssigneeID != nil {
		ids = append(ids, *fresh.AssigneeID)
	}
	if fresh.UserID != nil {
		ids = append(ids, *fresh.UserID)
	}
	if len(ids) > 0 {
		if users, err := s.desk.Users.GetByIDs(ctx, ids); err == nil {
			for _, u := range users {
				if fresh.AssigneeID != nil && u.ID == *fresh.AssigneeID {
					row.Assignee = &AdminAssignee{ID: u.ID, Name: u.DisplayName}
				}
				if fresh.UserID != nil && u.ID == *fresh.UserID && row.Requester.Name == "" {
					row.Requester.Name = u.DisplayName
				}
			}
		}
	}
	return &row, nil
}

func sameID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func idString(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return id.String()
}
