//go:build load

package load

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 3.0: support desk. EXPLAIN checks for the admin queue (every
// filter, search and sort), the requester's list and the summary. The
// dataset comes from `go run ./cmd/seed -profile=large` (generator "support
// tickets"; on an existing population: -extend "support tickets") or from the
// bulk test in tests/e2e (SUPPORT_LARGE_KEEP=1).

// seqScanAllowed lists checks where a sequential scan of a big table is the
// right plan: the summary counts the whole table (cached for 15 s, one pass).
var seqScanAllowed = map[string]bool{"support: summary (cached 15 s)": true}

type supportSample struct {
	admin, user uuid.UUID
	reference   string
	emailHash   string
	cursorAt    time.Time
	cursorID    uuid.UUID
	ticket      uuid.UUID
	// revisions 3.1: a client with an open chat report and its conversation.
	reportUser, reportConv uuid.UUID
}

// pickSupportIDs finds the busiest administrator and requester so each query
// is checked on its worst case; random values keep the checks running on an
// empty table.
func pickSupportIDs(db *gorm.DB) supportSample {
	s := supportSample{admin: uuid.New(), user: uuid.New(), reference: "SY-AAAAAA", emailHash: "0000000000000000000000000000000000000000000000000000000000000000",
		cursorAt: time.Now().Add(-30 * 24 * time.Hour), cursorID: uuid.New(), ticket: uuid.New()}
	var one struct{ A uuid.UUID }
	if db.Table("support_tickets").Select("assignee_id AS a").Where("assignee_id IS NOT NULL").Group("assignee_id").Order("count(*) DESC").Limit(1).Scan(&one).Error == nil && one.A != uuid.Nil {
		s.admin = one.A
	}
	one = struct{ A uuid.UUID }{}
	if db.Table("support_tickets").Select("user_id AS a").Where("user_id IS NOT NULL").Group("user_id").Order("count(*) DESC").Limit(1).Scan(&one).Error == nil && one.A != uuid.Nil {
		s.user = one.A
	}
	var row struct {
		ID        uuid.UUID
		Reference string
		EmailHash string
		CreatedAt time.Time
	}
	if db.Table("support_tickets").Select("id, reference, email_hash, created_at").Order("created_at DESC").Offset(50000).Limit(1).Scan(&row).Error == nil && row.ID != uuid.Nil {
		s.reference, s.emailHash, s.cursorAt, s.cursorID, s.ticket = row.Reference, row.EmailHash, row.CreatedAt, row.ID, row.ID
	}
	var rep struct{ U, C uuid.UUID }
	if db.Table("support_tickets").Select("user_id AS u, conversation_id AS c").
		Where("kind = 'chat_report' AND status = 'open' AND conversation_id IS NOT NULL").Limit(1).Scan(&rep).Error == nil && rep.U != uuid.Nil {
		s.reportUser, s.reportConv = rep.U, rep.C
	} else {
		s.reportUser, s.reportConv = uuid.New(), uuid.New()
	}
	return s
}

func supportChecks(s supportSample) []queryCheck {
	open, pending := domain.SupportOpen, domain.SupportPending
	list := func(name string, budget time.Duration, f repository.SupportAdminFilter) queryCheck {
		f.Limit = 26
		return queryCheck{name, budget, func(ctx context.Context, d *gorm.DB) { _, _ = repository.NewSupportRepository(d).AdminList(ctx, f) }}
	}
	const fast = 50 * time.Millisecond
	cursor := &repository.SupportCursor{At: s.cursorAt, ID: s.cursorID}
	return []queryCheck{
		list("support: queue newest", fast, repository.SupportAdminFilter{Sort: repository.SupportSortNewest}),
		list("support: queue newest (deep page)", fast, repository.SupportAdminFilter{Sort: repository.SupportSortNewest, Cursor: cursor}),
		list("support: queue oldest", fast, repository.SupportAdminFilter{Sort: repository.SupportSortOldest}),
		list("support: queue oldest (deep page)", fast, repository.SupportAdminFilter{Sort: repository.SupportSortOldest, Cursor: cursor}),
		list("support: queue waiting (open)", fast, repository.SupportAdminFilter{Sort: repository.SupportSortWaiting, Statuses: []domain.SupportStatus{open}}),
		list("support: queue waiting (open, deep)", fast, repository.SupportAdminFilter{Sort: repository.SupportSortWaiting,
			Statuses: []domain.SupportStatus{open}, Cursor: cursor}),
		list("support: queue waiting (ranked)", 150*time.Millisecond, repository.SupportAdminFilter{Sort: repository.SupportSortWaiting,
			Statuses: []domain.SupportStatus{open, pending}}),
		// The second range of the ranked sort: every status but open.
		list("support: queue waiting (pending, resolved)", fast, repository.SupportAdminFilter{Sort: repository.SupportSortWaiting,
			Statuses: []domain.SupportStatus{pending, domain.SupportResolved}, Cursor: &repository.SupportCursor{At: s.cursorAt, ID: s.cursorID, Rank: 1}}),
		// revisions 3.1: conversation reports (kind=chat_report): one partial index per sort.
		list("support: reports newest", fast, repository.SupportAdminFilter{Kind: domain.TicketChatReport, Sort: repository.SupportSortNewest}),
		list("support: reports newest (deep page)", fast, repository.SupportAdminFilter{Kind: domain.TicketChatReport, Sort: repository.SupportSortNewest, Cursor: cursor}),
		list("support: reports oldest", fast, repository.SupportAdminFilter{Kind: domain.TicketChatReport, Sort: repository.SupportSortOldest}),
		list("support: reports oldest (deep page)", fast, repository.SupportAdminFilter{Kind: domain.TicketChatReport, Sort: repository.SupportSortOldest, Cursor: cursor}),
		list("support: reports waiting (open)", fast, repository.SupportAdminFilter{Kind: domain.TicketChatReport, Sort: repository.SupportSortWaiting,
			Statuses: []domain.SupportStatus{open}}),
		list("support: reports waiting (ranked)", 150*time.Millisecond, repository.SupportAdminFilter{Kind: domain.TicketChatReport,
			Sort: repository.SupportSortWaiting, Statuses: []domain.SupportStatus{open, pending}}),
		list("support: reports waiting (rest, deep)", fast, repository.SupportAdminFilter{Kind: domain.TicketChatReport, Sort: repository.SupportSortWaiting,
			Statuses: []domain.SupportStatus{pending, domain.SupportResolved}, Cursor: &repository.SupportCursor{At: s.cursorAt, ID: s.cursorID, Rank: 1}}),
		list("support: reports filter status", fast, repository.SupportAdminFilter{Kind: domain.TicketChatReport, Statuses: []domain.SupportStatus{pending}}),
		list("support: reports filter open+pending", fast, repository.SupportAdminFilter{Kind: domain.TicketChatReport,
			Statuses: []domain.SupportStatus{open, pending}, Sort: repository.SupportSortNewest}),
		list("support: contacts only newest", fast, repository.SupportAdminFilter{Kind: domain.TicketContact, Sort: repository.SupportSortNewest}),
		{"support: open report lookup", fast, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewSupportRepository(d).OpenReport(ctx, s.reportUser, s.reportConv)
		}},
		{"chat: latest session of a conversation", fast, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewChatRepository(d).LatestSession(ctx, s.reportConv)
		}},
		list("support: filter status", fast, repository.SupportAdminFilter{Statuses: []domain.SupportStatus{pending}}),
		list("support: filter priority", fast, repository.SupportAdminFilter{Priority: "urgent"}),
		list("support: filter topic + status", fast, repository.SupportAdminFilter{Topic: "billing", Statuses: []domain.SupportStatus{open}}),
		list("support: filter assignee", fast, repository.SupportAdminFilter{AssigneeID: &s.admin}),
		list("support: filter assignee + status", fast, repository.SupportAdminFilter{AssigneeID: &s.admin, Statuses: []domain.SupportStatus{open, pending}}),
		list("support: filter unassigned", fast, repository.SupportAdminFilter{Unassigned: true, Statuses: []domain.SupportStatus{open}}),
		list("support: filter user", fast, repository.SupportAdminFilter{UserID: &s.user}),
		list("support: search reference", fast, repository.SupportAdminFilter{Reference: s.reference}),
		list("support: search e-mail (keyed hash)", fast, repository.SupportAdminFilter{EmailHash: s.emailHash}),
		list("support: search subject words", fast, repository.SupportAdminFilter{Words: "refund charge"}),
		list("support: search subject + filters", fast, repository.SupportAdminFilter{Words: "login", Statuses: []domain.SupportStatus{open}, Priority: "high"}),
		{"support: summary (cached 15 s)", 250 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewSupportRepository(d).Summary(ctx, s.admin)
		}},
		{"support: my tickets", fast, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewSupportRepository(d).ListForUser(ctx, s.user, nil, 21)
		}},
		{"support: my tickets (deep)", fast, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewSupportRepository(d).ListForUser(ctx, s.user, &repository.Cursor{At: s.cursorAt, ID: s.cursorID.String()}, 21)
		}},
		{"support: ticket by reference", fast, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewSupportRepository(d).GetByReference(ctx, s.reference)
		}},
		{"support: thread", fast, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewSupportRepository(d).Messages(ctx, s.ticket, false, 200)
		}},
		{"support: maintenance claim resolved", 100 * time.Millisecond, func(ctx context.Context, d *gorm.DB) {
			_, _ = repository.NewSupportRepository(d).ClaimResolved(ctx, time.Now().Add(-7*24*time.Hour), 200)
		}},
	}
}
