package service

// revisions 3.0: support desk (section 16). This file holds the shared pieces:
// dependencies, view types, validation, e-mail masking, private-link tokens
// and cursors. The admin queue, the requester views and the maintenance job
// are in support_admin.go, support_customer.go and support_maintenance.go.

import (
	"context"
	"crypto/subtle"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// Notification kind for a staff reply (in-app and push; the reply e-mail is
// sent separately).
const NotifySupportReplied = "support.replied"

// SupportJob is the name of the hourly maintenance job.
const SupportJob = "support-maintenance"

// Support desk messages (translated at the API edge).
const (
	msgSupportReplyBody    = "Write a message between 1 and 5,000 characters."
	msgSupportVisibility   = "Choose public or internal."
	msgSupportStatus       = "Choose open, pending, resolved or closed."
	msgSupportPriority     = "Choose low, normal, high or urgent."
	msgSupportAssignee     = "Choose an active administrator."
	msgSupportAssigneeID   = "Invalid administrator."
	msgSupportSort         = "Choose newest, oldest or waiting."
	msgSupportAssigneeList = "Use me, unassigned or an administrator's identifier."
	msgSupportQuery        = "Use at most 100 characters."
	msgSupportClosed       = "This request is closed. Please send a new request to contact us again."
	msgSupportLimit        = "This request has reached the maximum number of messages."
	msgSupportVersion      = "This request was changed by someone else. Reload it and try again."
	msgSupportKind         = "Choose contact or chat report."
	msgSupportAutoClose    = "Closed automatically after 7 days without a reply."
)

const (
	supportMaxBody       = 5000
	supportMaxQuery      = 100
	supportSummaryTTL    = 15 * time.Second
	supportViewedTTL     = 10 * time.Minute
	supportAutoCloseDays = 7
	supportAdminPage     = 25
	supportAdminPageMax  = 100
	supportMyPage        = 20
	supportMyPageMax     = 50
	supportTokenBytes    = 32
	supportTokenLen      = 43 // 32 bytes, base64url without padding
	supportBrand         = "Staryield Support"
)

var (
	supportRefRE   = regexp.MustCompile(`^SY-[A-HJ-NP-Z2-9]{6}$`)
	supportTokenRE = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
)

// SupportSettings configure the support desk.
type SupportSettings struct {
	// FrontendURL builds private links; ConsoleURL the links sent to the team.
	FrontendURL string
	ConsoleURL  string
	// Inbox receives requester replies of unassigned tickets.
	Inbox string
	// LinkTTL is the validity of a private link (each e-mailed link extends it).
	LinkTTL time.Duration
	// Retention is how long closed tickets are kept.
	Retention time.Duration
}

// SupportDeskDeps are the collaborators of the support desk.
type SupportDeskDeps struct {
	Tx    database.TxManager
	Users repository.UserRepository
	// revisions 3.1: conversation reports (see support_report.go).
	Chat     repository.ChatRepository
	Psychics repository.PsychicRepository
	Audit    repository.AuditRepository
	Cache    cache.Store // optional: summary cache and throttled audit
	Notifier Notifier    // in-app + push notification for staff replies
	Settings SupportSettings
}

// WithDesk wires the support desk. Without it only the contact form works.
func (s *SupportService) WithDesk(d SupportDeskDeps) *SupportService {
	if d.Notifier == nil {
		d.Notifier = NoopNotifier{}
	}
	if d.Settings.LinkTTL <= 0 {
		d.Settings.LinkTTL = 720 * time.Hour
	}
	if d.Settings.Retention <= 0 {
		d.Settings.Retention = 17520 * time.Hour
	}
	d.Settings.FrontendURL = strings.TrimRight(d.Settings.FrontendURL, "/")
	d.Settings.ConsoleURL = strings.TrimRight(d.Settings.ConsoleURL, "/")
	s.desk = d
	return s
}

func (s *SupportService) enabled() bool { return s.desk.Tx != nil && s.desk.Users != nil }

// ---------------------------------------------------------------------------
// Views
// ---------------------------------------------------------------------------

// AdminRequester identifies who wrote, without the address.
type AdminRequester struct {
	UserID      *uuid.UUID `json:"user_id,omitempty"`
	Name        string     `json:"name"`
	EmailMasked string     `json:"email_masked"`
}

// AdminRequesterFull adds the decrypted address (ticket detail only).
type AdminRequesterFull struct {
	AdminRequester
	Email string `json:"email"`
}

// AdminAssignee is the administrator a ticket is assigned to.
type AdminAssignee struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type adminTicketCommon struct {
	ID              uuid.UUID      `json:"id"`
	Reference       string         `json:"reference"`
	Subject         string         `json:"subject"`
	Topic           string         `json:"topic"`
	Status          string         `json:"status"`
	Priority        string         `json:"priority"`
	Kind            string         `json:"kind"` // contact | chat_report (revisions 3.1)
	Locale          string         `json:"locale"`
	Assignee        *AdminAssignee `json:"assignee,omitempty"`
	MessageCount    int            `json:"message_count"`
	NeedsReply      bool           `json:"needs_reply"`
	Unread          bool           `json:"unread"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	LastMessageAt   time.Time      `json:"last_message_at"`
	FirstResponseAt *time.Time     `json:"first_response_at,omitempty"`
	ResolvedAt      *time.Time     `json:"resolved_at,omitempty"`
	Version         int            `json:"version"`
}

// AdminTicketRow is a queue row (metadata only).
type AdminTicketRow struct {
	adminTicketCommon
	Requester AdminRequester      `json:"requester"`
	Report    *AdminReportContext `json:"report,omitempty"` // chat reports only
}

// AdminTicketUser is the account a ticket is linked to.
type AdminTicketUser struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	Status      string    `json:"status"`
}

// MessageAuthor names the author of a message.
type MessageAuthor struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// SupportMessageView is a message in the admin thread.
type SupportMessageView struct {
	ID          uuid.UUID     `json:"id"`
	Author      MessageAuthor `json:"author"`
	Visibility  string        `json:"visibility"`
	Body        string        `json:"body"`
	CreatedAt   time.Time     `json:"created_at"`
	EmailStatus string        `json:"email_status,omitempty"`
}

// AdminTicket is the decrypted ticket detail.
type AdminTicket struct {
	adminTicketCommon
	Requester AdminRequesterFull   `json:"requester"`
	Report    *AdminReportDetail   `json:"report,omitempty"` // chat reports only
	User      *AdminTicketUser     `json:"user,omitempty"`
	Messages  []SupportMessageView `json:"messages"`
}

// SupportSummary drives the console's navigation badge.
type SupportSummary struct {
	Open       int64 `json:"open"`
	Pending    int64 `json:"pending"`
	Resolved   int64 `json:"resolved"`
	Closed     int64 `json:"closed"`
	Unassigned int64 `json:"unassigned"`
	Mine       int64 `json:"mine"`
	NeedsReply int64 `json:"needs_reply"`
	// ReportsOpen counts chat reports still being worked (open or pending).
	ReportsOpen int64 `json:"reports_open"`
}

// CustomerTicketRow is a request as its requester sees it in a list.
type CustomerTicketRow struct {
	Reference     string    `json:"reference"`
	Subject       string    `json:"subject"`
	Topic         string    `json:"topic"`
	Status        string    `json:"status"`
	Awaiting      string    `json:"awaiting"` // support | you | none
	CreatedAt     time.Time `json:"created_at"`
	LastMessageAt time.Time `json:"last_message_at"`
	// revisions 3.1: conversation reports.
	Kind   string             `json:"kind"`
	Report *ChatReportContext `json:"report,omitempty"`
}

// CustomerMessage is a message visible to the requester.
type CustomerMessage struct {
	ID        uuid.UUID     `json:"id"`
	Author    MessageAuthor `json:"author"`
	Body      string        `json:"body"`
	CreatedAt time.Time     `json:"created_at"`
}

// CustomerTicket is a request with its public thread.
type CustomerTicket struct {
	CustomerTicketRow
	Messages []CustomerMessage `json:"messages"`
	CanReply bool              `json:"can_reply"`
}

// ---------------------------------------------------------------------------
// Validation and helpers
// ---------------------------------------------------------------------------

// validateSupportBody normalises a reply or note (1–5000 characters).
func validateSupportBody(raw string) (string, error) {
	body := strings.TrimSpace(CleanText(raw))
	if n := utf8.RuneCountInString(body); n < 1 || n > supportMaxBody {
		return "", apperror.Validation(map[string]string{"body": msgSupportReplyBody})
	}
	return body, nil
}

// awaitingOf says who the next move belongs to.
func awaitingOf(s domain.SupportStatus) string {
	switch s {
	case domain.SupportOpen:
		return "support"
	case domain.SupportPending:
		return "you"
	}
	return "none"
}

// unreadTicket reports whether the requester wrote since staff last looked (a
// tie counts as unread: better a ticket shown twice than one missed).
func unreadTicket(t *domain.SupportTicket) bool {
	return t.StaffSeenAt == nil || !t.StaffSeenAt.After(t.LastCustomerMessageAt)
}

// statusFields returns the column changes of moving a ticket from cur to next.
// Resolving stamps resolved_at, closing stamps closed_at (the retention clock);
// moving back to open or pending clears both. Closed tickets only leave
// "closed" through an administrator's PATCH (replies are refused earlier).
func statusFields(cur, next domain.SupportStatus, now time.Time) map[string]any {
	if cur == next {
		return nil
	}
	f := map[string]any{"status": next}
	switch next {
	case domain.SupportResolved:
		f["resolved_at"], f["closed_at"] = now, nil
	case domain.SupportClosed:
		f["closed_at"] = now
	default:
		f["resolved_at"], f["closed_at"] = nil, nil
	}
	return f
}

// MaskEmail hides most of an address: "jane@gmail.com" becomes "j•••@g•••.com".
func MaskEmail(email string) string {
	email = strings.TrimSpace(email)
	local, domainPart, ok := strings.Cut(email, "@")
	if !ok || local == "" || domainPart == "" {
		return "•••"
	}
	first := func(s string) string { r, _ := utf8.DecodeRuneInString(s); return string(r) }
	host, tld := domainPart, ""
	if i := strings.LastIndex(domainPart, "."); i > 0 && i < len(domainPart)-1 {
		host, tld = domainPart[:i], domainPart[i:]
	}
	return first(local) + "•••@" + first(host) + "•••" + tld
}

// staffDisplay is how requesters see staff: "Staryield Support (Maria)".
func staffDisplay(name string) string {
	first := staffFirstName(name)
	if first == "" {
		return supportBrand
	}
	return supportBrand + " (" + first + ")"
}

// staffFirstName returns the first word of a display name (never an address).
func staffFirstName(name string) string {
	f := strings.Fields(CleanLine(name))
	if len(f) == 0 || strings.Contains(f[0], "@") {
		return ""
	}
	if utf8.RuneCountInString(f[0]) > 30 {
		return string([]rune(f[0])[:30])
	}
	return f[0]
}

// parseTicketKey accepts a uuid or a reference ("SY-…", any case).
func parseTicketKey(key string) (id uuid.UUID, ref string, err error) {
	key = strings.TrimSpace(key)
	if u, perr := uuid.Parse(key); perr == nil {
		return u, "", nil
	}
	if r := strings.ToUpper(key); supportRefRE.MatchString(r) {
		return uuid.Nil, r, nil
	}
	return uuid.Nil, "", apperror.NotFound("Support request")
}

// ---------------------------------------------------------------------------
// Private-link tokens
// ---------------------------------------------------------------------------

func supportTokenAD(id uuid.UUID) string { return supportAD(id, "token") }

// tokenHash is the keyed hash stored instead of the token.
func (s *SupportService) tokenHash(token string) string {
	return security.Keyed(s.ipSecret, "support-token:"+token, 0)
}

// issueToken creates a fresh token for t: its keyed hash and a sealed copy are
// stored on the ticket (the raw token is returned once, for the e-mail link).
func (s *SupportService) issueToken(t *domain.SupportTicket, now time.Time) (string, error) {
	token, err := security.RandomToken(supportTokenBytes)
	if err != nil {
		return "", err
	}
	sealed, err := s.sealer.Seal(token, supportTokenAD(t.ID))
	if err != nil {
		return "", err
	}
	hash := s.tokenHash(token)
	exp := now.Add(s.desk.ttl())
	t.AccessTokenHash, t.AccessTokenEnc, t.AccessTokenExpiresAt = &hash, &sealed, &exp
	return token, nil
}

func (d SupportDeskDeps) ttl() time.Duration {
	if d.Settings.LinkTTL > 0 {
		return d.Settings.LinkTTL
	}
	return 720 * time.Hour
}

// verifyToken checks a presented token in constant time. Every failure
// (unknown ticket, malformed, wrong, expired) is the same "false"; the caller
// answers one 404 for all of them. t may be nil: the comparison still runs.
func (s *SupportService) verifyToken(t *domain.SupportTicket, presented string, now time.Time) bool {
	wellFormed := supportTokenRE.MatchString(presented)
	stored := strings.Repeat("0", 64)
	live := false
	if t != nil && t.AccessTokenHash != nil && t.AccessTokenExpiresAt != nil {
		stored = *t.AccessTokenHash
		live = now.Before(*t.AccessTokenExpiresAt)
	}
	if !wellFormed {
		presented = strings.Repeat("A", supportTokenLen)
	}
	match := subtle.ConstantTimeCompare([]byte(s.tokenHash(presented)), []byte(stored)) == 1
	return wellFormed && live && match && t != nil
}

// linkFor returns the private link of a ticket for an e-mail: the stored token
// is extended, not rotated, and a new one is only issued when there is none, it
// expired or it cannot be read. Failures return an empty link.
func (s *SupportService) linkFor(ctx context.Context, t *domain.SupportTicket) string {
	now := s.clock.Now()
	if t.AccessTokenHash != nil && t.AccessTokenEnc != nil && t.AccessTokenExpiresAt != nil && now.Before(*t.AccessTokenExpiresAt) {
		if token, err := s.sealer.Open(*t.AccessTokenEnc, supportTokenAD(t.ID)); err == nil && supportTokenRE.MatchString(token) {
			exp := now.Add(s.desk.ttl())
			if err := s.repo.Update(ctx, t.ID, map[string]any{"access_token_expires_at": exp}); err != nil {
				warnErr(ctx, "support: extend link", err)
				return ""
			}
			t.AccessTokenExpiresAt = &exp
			return s.ticketLink(t.Locale, t.Reference, token)
		}
	}
	token, err := s.issueToken(t, now)
	if err != nil {
		warnErr(ctx, "support: issue link", err)
		return ""
	}
	if err := s.repo.Update(ctx, t.ID, map[string]any{"access_token_hash": *t.AccessTokenHash, "access_token_enc": *t.AccessTokenEnc,
		"access_token_expires_at": *t.AccessTokenExpiresAt}); err != nil {
		warnErr(ctx, "support: store link", err)
		return ""
	}
	return s.ticketLink(t.Locale, t.Reference, token)
}

// ticketLink is the private link; the token travels in the fragment, which
// browsers never send to servers or put in referrers.
func (s *SupportService) ticketLink(locale, reference, token string) string {
	if s.desk.Settings.FrontendURL == "" {
		return ""
	}
	return mail.AppURL(s.desk.Settings.FrontendURL, locale, "/support/"+reference) + "#t=" + token
}

// consoleLink points the team at a ticket in the admin console.
func (s *SupportService) consoleLink(id uuid.UUID) string {
	if s.desk.Settings.ConsoleURL == "" {
		return ""
	}
	return s.desk.Settings.ConsoleURL + "/support/" + id.String()
}

// ---------------------------------------------------------------------------
// Cursors
// ---------------------------------------------------------------------------

// encodeSupportCursor builds the opaque cursor after the last row of a page.
// The waiting sort across mixed statuses also encodes the row's rank.
func encodeSupportCursor(sort string, t *domain.SupportTicket, mixedWaiting bool) string {
	switch {
	case sort == repository.SupportSortWaiting && mixedWaiting:
		rank := 1
		if t.Status == domain.SupportOpen {
			rank = 0
		}
		return repository.EncodeCursor(t.LastCustomerMessageAt, fmt.Sprintf("%d|%s", rank, t.ID))
	case sort == repository.SupportSortWaiting:
		return repository.EncodeCursor(t.LastCustomerMessageAt, t.ID.String())
	}
	return repository.EncodeCursor(t.CreatedAt, t.ID.String())
}

// decodeSupportCursor parses a cursor produced by encodeSupportCursor.
func decodeSupportCursor(raw, sort string, mixedWaiting bool) (*repository.SupportCursor, error) {
	c, err := decodeCursor(raw)
	if err != nil || c == nil {
		return nil, err
	}
	bad := apperror.Validation(map[string]string{"cursor": "Invalid pagination cursor."})
	idPart, out := c.ID, &repository.SupportCursor{At: c.At}
	if sort == repository.SupportSortWaiting && mixedWaiting {
		rank, rest, ok := strings.Cut(c.ID, "|")
		if !ok || (rank != "0" && rank != "1") {
			return nil, bad
		}
		out.Rank = int(rank[0] - '0')
		idPart = rest
	}
	id, perr := uuid.Parse(idPart)
	if perr != nil {
		return nil, bad
	}
	out.ID = id
	return out, nil
}
