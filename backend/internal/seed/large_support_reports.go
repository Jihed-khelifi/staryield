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

// revisions 3.1: conversation reports. About one chat report for every ten
// support tickets, each linked to a real conversation of a synthetic client,
// its latest session and the advisor. Statuses are realistic and respect the
// "one open or pending report per client and conversation" unique index
// (conversations are drawn without repetition for the current report; some
// also get an older, closed one). The admin queue's kind filter, the open
// report lookup and the latest-session lookup are exercised at size.
func init() {
	RegisterLarge(Generator{Name: "chat reports", Run: func(_ context.Context, d *Dataset) error { return d.chatReports() }})
}

type reportConv struct {
	ID, ClientID, PsychicID uuid.UUID
	LastSeq                 int64
}

var reportReasons = []domain.ChatReportReason{domain.ReportAdvisorUnresponsive, domain.ReportInappropriate, domain.ReportBilling,
	domain.ReportTechnical, domain.ReportOther}

var reportLabels = map[domain.ChatReportReason]string{domain.ReportAdvisorUnresponsive: "Advisor unresponsive",
	domain.ReportInappropriate: "Inappropriate behavior", domain.ReportBilling: "Billing issue", domain.ReportTechnical: "Technical problem",
	domain.ReportOther: "Other"}

func (d *Dataset) chatReports() error {
	if d.Sealer == nil {
		d.Log.Warn("chat reports skipped: no sealer configured")
		return nil
	}
	var existing, tickets int64
	if err := d.DB.Table("support_tickets").Where("kind = 'chat_report'").Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 {
		d.Log.Info("chat reports skipped: already generated", zap.Int64("reports", existing))
		return nil
	}
	if err := d.DB.Table("support_tickets").Count(&tickets).Error; err != nil {
		return err
	}
	target := int(tickets / 10)
	if target < 100 {
		target = 100
	}
	if target > 100_000 {
		target = 100_000
	}
	var admins []uuid.UUID
	if err := d.DB.Model(&domain.User{}).Where("role = ? AND status = ?", domain.RoleAdmin, domain.UserActive).Limit(10).Pluck("id", &admins).Error; err != nil {
		return err
	}
	// Conversations of the synthetic clients, read from the database (an
	// extension run has no in-memory population).
	var convs []reportConv
	if err := d.DB.Raw(`SELECT c.id, c.client_id, c.psychic_id, c.last_seq FROM conversations c
		JOIN users u ON u.id = c.client_id WHERE u.email LIKE ? AND u.role = 'client' ORDER BY c.id LIMIT ?`,
		"%@"+LoadEmailDomain, 4*target).Scan(&convs).Error; err != nil {
		return err
	}
	if len(convs) == 0 {
		d.Log.Warn("chat reports skipped: no conversations of synthetic clients")
		return nil
	}
	d.Rand.Shuffle(len(convs), func(i, j int) { convs[i], convs[j] = convs[j], convs[i] })
	if target > len(convs) {
		target = len(convs)
	}
	convs = convs[:target]
	latest, err := d.latestSessions(convs)
	if err != nil {
		return err
	}

	rows := make([]domain.SupportTicket, 0, min(d.Opts.BatchSize, 1500)) // 37 columns: stay under 65535 bind parameters
	messages := make([]domain.SupportMessage, 0, d.Opts.BatchSize)
	flush := func() error {
		if err := d.insert("support_tickets", &rows, len(rows), true); err != nil {
			return err
		}
		rows = rows[:0]
		err := d.insert("support_messages", &messages, len(messages), true)
		messages = messages[:0]
		return err
	}
	// References must not collide with the existing tickets (100k tickets and
	// 10k reports in a 32^6 space do collide now and then).
	var taken []string
	if err := d.DB.Table("support_tickets").Pluck("reference", &taken).Error; err != nil {
		return err
	}
	refs := make(map[string]bool, len(taken)+2*target)
	for _, r := range taken {
		refs[r] = true
	}
	build := func(c reportConv, status domain.SupportStatus, notBefore time.Time) domain.SupportTicket {
		created := d.After(notBefore, 90*24*time.Hour)
		if created.After(d.Now) {
			created = d.Now.Add(-time.Duration(d.Rand.IntN(3600)) * time.Second)
		}
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
		reason := reportReasons[d.pick(20, 15, 25, 25, 15)]
		email := "report-" + d.Tag + "-" + id.String()[:8] + "@" + LoadEmailDomain
		details := ""
		if reason == domain.ReportOther || d.Rand.Float64() < 0.6 {
			details = supportBodies[d.Rand.IntN(len(supportBodies))] + " Reference " + ref + "."
		}
		clientID, advisorID, convID := c.ClientID, c.PsychicID, c.ID
		seq := int64(0)
		if c.LastSeq > 0 {
			seq = int64(d.Rand.IntN(int(c.LastSeq) + 1))
		}
		t := domain.SupportTicket{ID: id, Reference: ref, UserID: &clientID, Name: firstNames[d.Rand.IntN(len(firstNames))], Topic: reason.Topic(),
			Subject: "Chat report · " + reportLabels[reason] + " · Advisor", MessageChars: utf8.RuneCountInString(details),
			Locale: []string{"en", "fr", "es"}[d.pick(70, 15, 15)], Status: status,
			EmailHash: security.Keyed(LoadPassword, "support-email:"+email, 0), IPHash: security.Keyed(LoadPassword, id.String(), 32),
			CreatedAt: created, UpdatedAt: created, Priority: reason.Priority(), Version: 1, MessageCount: 1, LastMessageAt: created,
			LastCustomerMessageAt: created, EmailMasked: "r•••@l•••.test", Kind: domain.TicketChatReport, ConversationID: &convID,
			ReportedUserID: &advisorID, ReportReason: &reason, ReportedSeq: seq}
		if sid, ok := latest[c.ID]; ok {
			t.SessionID = &sid
		}
		t.EmailEnc, _ = d.Sealer.Seal(email, "support:"+id.String()+":email")
		t.MessageEnc, _ = d.Sealer.Seal(details, "support:"+id.String()+":message")
		if status != domain.SupportOpen && d.Rand.Float64() < 0.8 && len(admins) > 0 {
			a := admins[d.Rand.IntN(len(admins))]
			t.AssigneeID = &a
		}
		d.supportThread(&t, status, admins, &messages)
		if d.Rand.Float64() < 0.6 {
			seen := d.After(created, 48*time.Hour)
			t.StaffSeenAt = &seen
		}
		return t
	}
	add := func(t domain.SupportTicket) error {
		rows = append(rows, t)
		return nil
	}
	for _, c := range convs {
		// Flush before building: a ticket's messages must never be inserted ahead of it.
		if len(rows)+2 > cap(rows) {
			if err := flush(); err != nil {
				return err
			}
		}
		// The current report: one conversation, at most one open or pending report.
		status := []domain.SupportStatus{domain.SupportOpen, domain.SupportPending, domain.SupportResolved, domain.SupportClosed}[d.pick(30, 15, 20, 35)]
		t := build(c, status, d.Past())
		if d.Rand.Float64() < 0.1 { // an older, closed report of the same conversation
			old := build(c, domain.SupportClosed, t.CreatedAt.Add(-120*24*time.Hour))
			if old.CreatedAt.Before(t.CreatedAt) {
				if err := add(old); err != nil {
					return err
				}
			}
		}
		if err := add(t); err != nil {
			return err
		}
	}
	if err := flush(); err != nil {
		return err
	}
	d.Log.Info("chat reports generated", zap.Int("reports", d.Counts["support_tickets"]), zap.Int("messages", d.Counts["support_messages"]))
	return nil
}

// latestSessions maps each conversation to its newest session (by requested_at).
func (d *Dataset) latestSessions(convs []reportConv) (map[uuid.UUID]uuid.UUID, error) {
	out := make(map[uuid.UUID]uuid.UUID, len(convs))
	const chunk = 5000
	for start := 0; start < len(convs); start += chunk {
		ids := make([]uuid.UUID, 0, chunk)
		for _, c := range convs[start:min(start+chunk, len(convs))] {
			ids = append(ids, c.ID)
		}
		var rows []struct{ ConversationID, ID uuid.UUID }
		if err := d.DB.Raw(`SELECT DISTINCT ON (conversation_id) conversation_id, id FROM chat_sessions WHERE conversation_id IN ?
			ORDER BY conversation_id, requested_at DESC, id DESC`, ids).Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			out[r.ConversationID] = r.ID
		}
	}
	return out, nil
}
