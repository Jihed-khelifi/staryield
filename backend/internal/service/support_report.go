package service

// revisions 3.1: conversation reports (section 16.4). A client reports a
// problem with a chat conversation; the report is a support ticket of kind
// "chat_report" linked to the conversation, one of its sessions and the
// advisor's user. It then lives in the ordinary desk (queue, replies, e-mails,
// private link, auto-close, retention, erasure). The advisor is never told.

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// Report validation messages (translated at the API edge).
const (
	msgReportReason  = "Choose what went wrong."
	msgReportOther   = "Describe the problem in 10 to 2,000 characters."
	msgReportDetails = "Use at most 2,000 characters."
	msgReportSession = "This session is not part of the conversation."
	msgReportID      = "Invalid identifier."
	msgReportOnlyCl  = "Only clients can report a conversation."
)

const (
	reportMaxDetails = 2000
	reportMinOther   = 10
	reportMaxSubject = 120
	reportMaxName    = 80
)

// ChatReportInput is a conversation report as sent by the chatroom.
type ChatReportInput struct {
	ConversationID string `json:"conversation_id"`
	SessionID      string `json:"session_id"`
	Reason         string `json:"reason"`
	Details        string `json:"details"`
	// Seq is the newest message the reporter had seen; it is clamped to the
	// conversation's last sequence number.
	Seq int64 `json:"seq"`
}

// reportLabels are the English reason labels of the queue subject and the team
// e-mail (interfaces show their own localized labels).
var reportLabels = map[domain.ChatReportReason]string{
	domain.ReportAdvisorUnresponsive: "Advisor unresponsive",
	domain.ReportInappropriate:       "Inappropriate behavior",
	domain.ReportBilling:             "Billing issue",
	domain.ReportTechnical:           "Technical problem",
	domain.ReportOther:               "Other",
}

// reportSubject is "Chat report · <reason label> · <advisor name>", at most 120
// characters: the advisor's name is shortened first, then the whole line.
func reportSubject(r domain.ChatReportReason, advisor string) string {
	prefix := "Chat report · " + reportLabels[r]
	advisor = CleanLine(advisor)
	if advisor == "" {
		return ellipsize(prefix, reportMaxSubject)
	}
	prefix += " · "
	if room := reportMaxSubject - utf8.RuneCountInString(prefix); room > 0 {
		return prefix + ellipsize(advisor, room)
	}
	return ellipsize(prefix, reportMaxSubject)
}

// ellipsize cuts s to n characters, ending in "…" when it was shortened.
func ellipsize(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return strings.TrimSpace(string(r[:n-1])) + "…"
}

type cleanReport struct {
	conversationID uuid.UUID
	sessionID      *uuid.UUID
	reason         domain.ChatReportReason
	details        string
	seq            int64
}

// validateReport normalises a report and reports every problem at once.
func validateReport(in ChatReportInput) (cleanReport, error) {
	fields := map[string]string{}
	var out cleanReport
	id, err := uuid.Parse(strings.TrimSpace(in.ConversationID))
	if err != nil {
		fields["conversation_id"] = msgReportID
	}
	out.conversationID = id
	if raw := strings.TrimSpace(in.SessionID); raw != "" {
		sid, err := uuid.Parse(raw)
		if err != nil {
			fields["session_id"] = msgReportID
		}
		out.sessionID = &sid
	}
	out.reason = domain.ChatReportReason(strings.ToLower(strings.TrimSpace(in.Reason)))
	if !domain.ValidChatReportReason(out.reason) {
		fields["reason"] = msgReportReason
	}
	out.details = strings.TrimSpace(CleanText(in.Details))
	switch n := utf8.RuneCountInString(out.details); {
	case n > reportMaxDetails:
		fields["details"] = msgReportDetails
		if out.reason == domain.ReportOther {
			fields["details"] = msgReportOther
		}
	case out.reason == domain.ReportOther && n < reportMinOther:
		fields["details"] = msgReportOther
	}
	if in.Seq > 0 {
		out.seq = in.Seq
	}
	if len(fields) > 0 {
		return out, apperror.Validation(fields)
	}
	return out, nil
}

// reportConversation returns a conversation the caller is the client of. An
// unknown conversation and somebody else's answer the same 404 (no existence
// leak; advisors never reach it either).
func (s *SupportService) reportConversation(ctx context.Context, userID, id uuid.UUID) (*domain.Conversation, error) {
	if !s.reportsEnabled() || userID == uuid.Nil {
		return nil, apperror.NotFound("Conversation")
	}
	conv, err := s.desk.Chat.GetConversation(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Conversation")
	}
	if conv.ClientID != userID {
		return nil, apperror.NotFound("Conversation")
	}
	return conv, nil
}

func (s *SupportService) reportsEnabled() bool {
	return s.enabled() && s.desk.Chat != nil && s.desk.Psychics != nil
}

// CreateChatReport stores a conversation report (clients only) and tells the
// team. A second open or pending report about the same conversation answers 409
// report_exists with the existing reference, also when two requests race (the
// partial unique index decides).
func (s *SupportService) CreateChatReport(ctx context.Context, p reqctx.Principal, in ChatReportInput) (*CustomerTicket, error) {
	if p.UserID == uuid.Nil {
		return nil, apperror.Unauthenticated("")
	}
	if p.Role != domain.RoleClient {
		return nil, apperror.Forbidden(msgReportOnlyCl)
	}
	clean, err := validateReport(in)
	if err != nil {
		return nil, err
	}
	conv, err := s.reportConversation(ctx, p.UserID, clean.conversationID)
	if err != nil {
		return nil, err
	}
	sess, err := s.reportSession(ctx, conv, clean.sessionID)
	if err != nil {
		return nil, err
	}
	if existing, err := s.repo.OpenReport(ctx, p.UserID, conv.ID); err == nil {
		return nil, apperror.ReportExists(existing.Reference)
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, apperror.Internal(err)
	}

	account, err := s.desk.Users.GetByID(ctx, p.UserID)
	if err != nil {
		return nil, mapRepoErr(err, "Account")
	}
	email, ok := NormalizeEmail(account.Email)
	if !ok {
		email = strings.ToLower(strings.TrimSpace(account.Email))
	}
	advisorName := s.advisorName(ctx, conv.PsychicID)
	name := truncateRunes(CleanLine(account.DisplayName), reportMaxName)
	t, token, err := s.buildTicket(ctx, p, ticketDraft{name: name, email: email, subject: reportSubject(clean.reason, advisorName),
		message: clean.details, topic: clean.reason.Topic(), priority: clean.reason.Priority(), kind: domain.TicketChatReport})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	reason := clean.reason
	reportedUser, convID := conv.PsychicID, conv.ID
	t.ConversationID, t.ReportedUserID, t.ReportReason = &convID, &reportedUser, &reason
	t.ReportedSeq = min(clean.seq, max(conv.LastSeq, 0))
	if sess != nil {
		sid := sess.ID
		t.SessionID = &sid
	}
	if err := s.insertTicket(ctx, t); err != nil {
		if errors.Is(err, repository.ErrDuplicateReport) {
			if existing, lerr := s.repo.OpenReport(ctx, p.UserID, conv.ID); lerr == nil {
				return nil, apperror.ReportExists(existing.Reference)
			}
		}
		return nil, apperror.Internal(err)
	}
	s.invalidateSummary(ctx)
	message := clean.details
	s.notify.TicketCreated(ctx, t, email, message, s.ticketLink(t.Locale, t.Reference, token))
	return s.customerView(ctx, t)
}

// reportSession resolves the session a report is about: the one given (it must
// belong to the conversation) or the conversation's latest. nil when the
// conversation has no session at all.
func (s *SupportService) reportSession(ctx context.Context, conv *domain.Conversation, given *uuid.UUID) (*domain.Session, error) {
	bad := apperror.Validation(map[string]string{"session_id": msgReportSession})
	if given != nil {
		sess, err := s.desk.Chat.GetSession(ctx, *given)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, bad
			}
			return nil, apperror.Internal(err)
		}
		if sess.ConversationID != conv.ID {
			return nil, bad
		}
		return sess, nil
	}
	sess, err := s.desk.Chat.LatestSession(ctx, conv.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, apperror.Internal(err)
	}
	return sess, nil
}

// advisorName is the advisor's public name (the account's display name when
// there is no profile), "" when neither can be read.
func (s *SupportService) advisorName(ctx context.Context, userID uuid.UUID) string {
	if ps, err := s.desk.Psychics.GetByUserIDs(ctx, []uuid.UUID{userID}); err == nil && len(ps) > 0 && ps[0].Name != "" {
		return ps[0].Name
	}
	if u, err := s.desk.Users.GetByID(ctx, userID); err == nil {
		return u.DisplayName
	}
	return ""
}

// OpenChatReport returns the caller's open or pending report about a
// conversation, nil when there is none. Somebody else's conversation answers
// 404 like an unknown one.
func (s *SupportService) OpenChatReport(ctx context.Context, userID, conversationID uuid.UUID) (*CustomerTicketRow, error) {
	if _, err := s.reportConversation(ctx, userID, conversationID); err != nil {
		return nil, err
	}
	t, err := s.repo.OpenReport(ctx, userID, conversationID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, apperror.Internal(err)
	}
	row := customerRow(t)
	row.Report = s.customerReports(ctx, []*domain.SupportTicket{t})[t.ID]
	return &row, nil
}

// ---------------------------------------------------------------------------
// Views
// ---------------------------------------------------------------------------

// ReportAdvisorPublic is the advisor as clients see them: public profile fields.
type ReportAdvisorPublic struct {
	Name      string `json:"name"`
	Slug      string `json:"slug,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// ChatReportContext is what the requester sees about a chat report.
type ChatReportContext struct {
	Reason           string               `json:"reason"`
	ConversationID   *uuid.UUID           `json:"conversation_id,omitempty"`
	SessionID        *uuid.UUID           `json:"session_id,omitempty"`
	Advisor          *ReportAdvisorPublic `json:"advisor,omitempty"`
	SessionStartedAt *time.Time           `json:"session_started_at,omitempty"`
}

// ReportAdvisorAdmin identifies the reported advisor for the team.
type ReportAdvisorAdmin struct {
	UserID    uuid.UUID  `json:"user_id"`
	PsychicID *uuid.UUID `json:"psychic_id,omitempty"`
	Name      string     `json:"name"`
	Slug      string     `json:"slug,omitempty"`
}

// AdminReportContext is a chat report as the queue shows it.
type AdminReportContext struct {
	Reason         string              `json:"reason"`
	ConversationID *uuid.UUID          `json:"conversation_id,omitempty"`
	SessionID      *uuid.UUID          `json:"session_id,omitempty"`
	ReportedSeq    int64               `json:"reported_seq"`
	Advisor        *ReportAdvisorAdmin `json:"advisor,omitempty"`
}

// AdminReportSession summarises the linked session (the transcript itself is
// read through the audited transcript endpoint).
type AdminReportSession struct {
	ID             uuid.UUID  `json:"id"`
	Status         string     `json:"status"`
	RequestedAt    time.Time  `json:"requested_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
	EndReason      string     `json:"end_reason,omitempty"`
	ActiveSeconds  int64      `json:"active_seconds"`
	BilledMinutes  int        `json:"billed_minutes"`
	CreditsCharged int64      `json:"credits_charged"`
	RatePerMinute  int64      `json:"rate_per_minute"`
	Sponsored      bool       `json:"sponsored"`
}

// AdminReportDetail is AdminTicket.report: the context and the linked session.
type AdminReportDetail struct {
	AdminReportContext
	Session *AdminReportSession `json:"session,omitempty"`
}

func isReport(t *domain.SupportTicket) bool {
	return t.Kind == domain.TicketChatReport && t.ReportReason != nil
}

func kindOf(t *domain.SupportTicket) string {
	if t.Kind == domain.TicketChatReport {
		return string(domain.TicketChatReport)
	}
	return string(domain.TicketContact)
}

// reportRefs are the advisors and sessions a page of reports points to, loaded
// in one query each (never per ticket).
type reportRefs struct {
	psychics map[uuid.UUID]domain.Psychic
	users    map[uuid.UUID]domain.User // advisors without a public profile (admin only)
	sessions map[uuid.UUID]domain.Session
}

func (s *SupportService) loadReportRefs(ctx context.Context, tickets []*domain.SupportTicket, withUsers bool) reportRefs {
	refs := reportRefs{psychics: map[uuid.UUID]domain.Psychic{}, users: map[uuid.UUID]domain.User{}, sessions: map[uuid.UUID]domain.Session{}}
	if s.desk.Chat == nil || s.desk.Psychics == nil {
		return refs
	}
	var advisors, sessions []uuid.UUID
	seenA, seenS := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
	for _, t := range tickets {
		if !isReport(t) {
			continue
		}
		if t.ReportedUserID != nil && !seenA[*t.ReportedUserID] {
			seenA[*t.ReportedUserID] = true
			advisors = append(advisors, *t.ReportedUserID)
		}
		if t.SessionID != nil && !seenS[*t.SessionID] {
			seenS[*t.SessionID] = true
			sessions = append(sessions, *t.SessionID)
		}
	}
	if len(advisors) > 0 {
		if ps, err := s.desk.Psychics.GetByUserIDs(ctx, advisors); err != nil {
			warnErr(ctx, "support: load report advisors", err)
		} else {
			for _, p := range ps {
				refs.psychics[p.UserID] = p
			}
		}
		if withUsers {
			var missing []uuid.UUID
			for _, id := range advisors {
				if _, ok := refs.psychics[id]; !ok {
					missing = append(missing, id)
				}
			}
			if len(missing) > 0 {
				if us, err := s.desk.Users.GetByIDs(ctx, missing); err == nil {
					for _, u := range us {
						refs.users[u.ID] = u
					}
				}
			}
		}
	}
	if len(sessions) > 0 {
		if ss, err := s.desk.Chat.GetSessions(ctx, sessions); err != nil {
			warnErr(ctx, "support: load report sessions", err)
		} else {
			for _, x := range ss {
				refs.sessions[x.ID] = x
			}
		}
	}
	return refs
}

// customerReports builds the requester's report context of each chat report.
func (s *SupportService) customerReports(ctx context.Context, tickets []*domain.SupportTicket) map[uuid.UUID]*ChatReportContext {
	out := map[uuid.UUID]*ChatReportContext{}
	refs := s.loadReportRefs(ctx, tickets, false)
	for _, t := range tickets {
		if !isReport(t) {
			continue
		}
		c := &ChatReportContext{Reason: string(*t.ReportReason), ConversationID: t.ConversationID, SessionID: t.SessionID}
		if t.ReportedUserID != nil {
			if p, ok := refs.psychics[*t.ReportedUserID]; ok {
				c.Advisor = &ReportAdvisorPublic{Name: p.Name, Slug: p.Slug, AvatarURL: p.ImageURL}
			}
		}
		if t.SessionID != nil {
			if sess, ok := refs.sessions[*t.SessionID]; ok {
				at := sess.RequestedAt
				if sess.StartedAt != nil {
					at = *sess.StartedAt
				}
				c.SessionStartedAt = &at
			}
		}
		out[t.ID] = c
	}
	return out
}

// adminReports builds the queue context of each chat report; with sessions it
// also summarises the linked session (ticket detail).
func (s *SupportService) adminReports(ctx context.Context, tickets []*domain.SupportTicket) (map[uuid.UUID]*AdminReportContext, reportRefs) {
	out := map[uuid.UUID]*AdminReportContext{}
	refs := s.loadReportRefs(ctx, tickets, true)
	for _, t := range tickets {
		if !isReport(t) {
			continue
		}
		c := &AdminReportContext{Reason: string(*t.ReportReason), ConversationID: t.ConversationID, SessionID: t.SessionID,
			ReportedSeq: t.ReportedSeq}
		if id := t.ReportedUserID; id != nil {
			if p, ok := refs.psychics[*id]; ok {
				pid := p.UserID
				c.Advisor = &ReportAdvisorAdmin{UserID: *id, PsychicID: &pid, Name: p.Name, Slug: p.Slug}
			} else if u, ok := refs.users[*id]; ok {
				c.Advisor = &ReportAdvisorAdmin{UserID: *id, Name: u.DisplayName}
			} else {
				c.Advisor = &ReportAdvisorAdmin{UserID: *id, Name: ""}
			}
		}
		out[t.ID] = c
	}
	return out, refs
}

func adminReportSession(x domain.Session) *AdminReportSession {
	return &AdminReportSession{ID: x.ID, Status: string(x.Status), RequestedAt: x.RequestedAt, StartedAt: x.StartedAt, EndedAt: x.EndedAt,
		EndReason: string(x.EndReason), ActiveSeconds: x.ActiveSeconds, BilledMinutes: x.BilledMinutes, CreditsCharged: x.CreditsCharged,
		RatePerMinute: x.RatePerMinute, Sponsored: x.Sponsored}
}
