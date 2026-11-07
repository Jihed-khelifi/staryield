package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// ErrDuplicateReference reports a support reference that is already taken
// (the service retries with a fresh one).
var ErrDuplicateReference = errors.New("repository: support reference already used")

// ErrDuplicateReport reports that the reporter already has an open or pending
// report about the conversation (the unique index decides, so it is race-safe).
var ErrDuplicateReport = errors.New("repository: chat report already open")

// supportReportIndex is the partial unique index behind ErrDuplicateReport.
const supportReportIndex = "support_tickets_one_open_report_idx"

// SupportRepository persists support tickets and their messages.
type SupportRepository interface {
	Create(ctx context.Context, t *domain.SupportTicket) error
	// PurgeForUser hard-deletes the tickets linked to a user (erasure);
	// messages go with them (ON DELETE CASCADE).
	PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error)

	// revisions 3.0: support desk
	Get(ctx context.Context, id uuid.UUID) (*domain.SupportTicket, error)
	GetByReference(ctx context.Context, reference string) (*domain.SupportTicket, error)
	GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.SupportTicket, error)
	// GetForUser returns a ticket only when it is linked to userID.
	GetForUser(ctx context.Context, userID uuid.UUID, reference string) (*domain.SupportTicket, error)
	Update(ctx context.Context, id uuid.UUID, fields map[string]any) error
	AddMessage(ctx context.Context, m *domain.SupportMessage) error
	SetMessageEmailStatus(ctx context.Context, id uuid.UUID, status domain.SupportEmailStatus) error
	// Messages returns a ticket's thread (oldest first, at most limit). With
	// publicOnly, internal notes and system entries are excluded in SQL.
	Messages(ctx context.Context, ticketID uuid.UUID, publicOnly bool, limit int) ([]domain.SupportMessage, error)
	AdminList(ctx context.Context, f SupportAdminFilter) ([]SupportAdminRow, error)
	Summary(ctx context.Context, adminID uuid.UUID) (SupportCounts, error)
	ListForUser(ctx context.Context, userID uuid.UUID, cursor *Cursor, limit int) ([]domain.SupportTicket, error)
	// OpenReport returns the user's open or pending report about a conversation
	// (served by the partial unique index); ErrNotFound when there is none.
	OpenReport(ctx context.Context, userID, conversationID uuid.UUID) (*domain.SupportTicket, error)
	// ClaimResolved locks resolved tickets whose resolution is older than
	// before (several instances may run the job: SKIP LOCKED).
	ClaimResolved(ctx context.Context, before time.Time, limit int) ([]domain.SupportTicket, error)
	// DeleteClosed hard-deletes closed tickets closed before t (messages cascade).
	DeleteClosed(ctx context.Context, before time.Time, limit int) (int64, error)
	// Unmasked returns tickets whose masked address is not filled in yet.
	Unmasked(ctx context.Context, limit int) ([]domain.SupportTicket, error)
}

// Support list sorts.
const (
	SupportSortNewest  = "newest"
	SupportSortOldest  = "oldest"
	SupportSortWaiting = "waiting"
)

// SupportCursor is a keyset position of the admin queue. Rank is only used by
// the "waiting" sort across mixed statuses (open tickets rank first).
type SupportCursor struct {
	At   time.Time
	ID   uuid.UUID
	Rank int
}

// SupportAdminFilter narrows the admin queue. Reference and EmailHash are
// exact matches; Words is a subject full-text search.
type SupportAdminFilter struct {
	Statuses   []domain.SupportStatus
	Topic      string
	Priority   string
	Kind       domain.TicketKind // "" = every kind
	AssigneeID *uuid.UUID
	Unassigned bool
	UserID     *uuid.UUID
	Reference  string
	EmailHash  string
	Words      string
	Sort       string
	Cursor     *SupportCursor
	Limit      int // rows to fetch (callers ask for one extra)
}

// SupportAdminRow is a queue row: ticket metadata (nothing sealed) with the
// assignee's and the linked account's display names.
type SupportAdminRow struct {
	domain.SupportTicket
	AssigneeName string
	UserName     string
}

// SupportCounts feeds the console's navigation badge.
type SupportCounts struct {
	Open, Pending, Resolved, Closed int64
	// Unassigned and Mine count tickets still being worked (open or pending).
	Unassigned, Mine int64
	// ReportsOpen counts chat reports still being worked (open or pending).
	ReportsOpen int64
}

// GormSupportRepository implements SupportRepository.
type GormSupportRepository struct{ db *gorm.DB }

// NewSupportRepository returns a GORM support repository.
func NewSupportRepository(db *gorm.DB) *GormSupportRepository { return &GormSupportRepository{db: db} }

// Create inserts a ticket.
func (r *GormSupportRepository) Create(ctx context.Context, t *domain.SupportTicket) error {
	err := database.Conn(ctx, r.db).Create(t).Error
	if isUnique(err, "support_tickets_reference_key") {
		return ErrDuplicateReference
	}
	if isUnique(err, supportReportIndex) {
		return ErrDuplicateReport
	}
	return err
}

// PurgeForUser implements SupportRepository.
func (r *GormSupportRepository) PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	res := database.Conn(ctx, r.db).Unscoped().Where("user_id = ?", userID).Delete(&domain.SupportTicket{})
	return res.RowsAffected, res.Error
}

// Get implements SupportRepository.
func (r *GormSupportRepository) Get(ctx context.Context, id uuid.UUID) (*domain.SupportTicket, error) {
	var t domain.SupportTicket
	if err := database.Conn(ctx, r.db).First(&t, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &t, nil
}

// GetByReference implements SupportRepository.
func (r *GormSupportRepository) GetByReference(ctx context.Context, reference string) (*domain.SupportTicket, error) {
	var t domain.SupportTicket
	if err := database.Conn(ctx, r.db).First(&t, "reference = ?", reference).Error; err != nil {
		return nil, notFound(err)
	}
	return &t, nil
}

// GetForUpdate implements SupportRepository.
func (r *GormSupportRepository) GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.SupportTicket, error) {
	var t domain.SupportTicket
	if err := database.Conn(ctx, r.db).Clauses(forUpdate).First(&t, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &t, nil
}

// GetForUser implements SupportRepository. Matching on user_id only means
// another account's reference answers not-found, never forbidden.
func (r *GormSupportRepository) GetForUser(ctx context.Context, userID uuid.UUID, reference string) (*domain.SupportTicket, error) {
	var t domain.SupportTicket
	if err := database.Conn(ctx, r.db).First(&t, "user_id = ? AND reference = ?", userID, reference).Error; err != nil {
		return nil, notFound(err)
	}
	return &t, nil
}

// Update implements SupportRepository.
func (r *GormSupportRepository) Update(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	// UpdateColumns: updated_at only moves when the caller sets it (viewing a
	// ticket or masking an address is not an edit).
	res := database.Conn(ctx, r.db).Model(&domain.SupportTicket{}).Where("id = ?", id).UpdateColumns(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// AddMessage implements SupportRepository.
func (r *GormSupportRepository) AddMessage(ctx context.Context, m *domain.SupportMessage) error {
	return database.Conn(ctx, r.db).Create(m).Error
}

// SetMessageEmailStatus implements SupportRepository.
func (r *GormSupportRepository) SetMessageEmailStatus(ctx context.Context, id uuid.UUID, status domain.SupportEmailStatus) error {
	return database.Conn(ctx, r.db).Model(&domain.SupportMessage{}).Where("id = ?", id).Update("email_status", status).Error
}

// Messages implements SupportRepository.
func (r *GormSupportRepository) Messages(ctx context.Context, ticketID uuid.UUID, publicOnly bool, limit int) ([]domain.SupportMessage, error) {
	q := database.Conn(ctx, r.db).Where("ticket_id = ?", ticketID)
	if publicOnly {
		q = q.Where("visibility = ? AND author_type IN ?", domain.SupportPublic,
			[]domain.SupportAuthor{domain.SupportAuthorCustomer, domain.SupportAuthorStaff})
	}
	var out []domain.SupportMessage
	err := q.Order("created_at ASC, id ASC").Limit(clampLimit(limit, domain.MaxSupportMessages, domain.MaxSupportMessages)).Find(&out).Error
	return out, err
}

// supportRow lists the columns a queue row needs: nothing sealed.
const supportRowColumns = "t.id, t.reference, t.user_id, t.name, t.topic, t.subject, t.message_chars, t.locale, t.status, " +
	"t.priority, t.assignee_id, t.version, t.message_count, t.created_at, t.updated_at, t.last_message_at, " +
	"t.last_customer_message_at, t.last_staff_message_at, t.first_response_at, t.resolved_at, t.closed_at, t.staff_seen_at, " +
	"t.email_masked, t.kind, t.conversation_id, t.session_id, t.reported_user_id, t.report_reason, t.reported_seq, " +
	"a.display_name AS assignee_name, u.display_name AS user_name"

// adminBase applies the filters shared by every sort (no order, no cursor).
func (r *GormSupportRepository) adminBase(ctx context.Context, f SupportAdminFilter) *gorm.DB {
	q := database.Conn(ctx, r.db).Table("support_tickets AS t").Select(supportRowColumns).
		Joins("LEFT JOIN users a ON a.id = t.assignee_id").
		Joins("LEFT JOIN users u ON u.id = t.user_id").
		Where("t.deleted_at IS NULL")
	if len(f.Statuses) > 0 {
		q = q.Where("t.status IN ?", f.Statuses)
	}
	if f.Topic != "" {
		q = q.Where("t.topic = ?", f.Topic)
	}
	if f.Priority != "" {
		q = q.Where("t.priority = ?", f.Priority)
	}
	// Literals (from the validated enum), not parameters: the partial indexes of
	// the report queue are only chosen when the predicate is provable.
	switch f.Kind {
	case domain.TicketChatReport:
		q = q.Where("t.kind = 'chat_report'")
	case domain.TicketContact:
		q = q.Where("t.kind = 'contact'")
	}
	switch {
	case f.Unassigned:
		q = q.Where("t.assignee_id IS NULL")
	case f.AssigneeID != nil:
		q = q.Where("t.assignee_id = ?", *f.AssigneeID)
	}
	if f.UserID != nil {
		q = q.Where("t.user_id = ?", *f.UserID)
	}
	if f.Reference != "" {
		q = q.Where("t.reference = ?", f.Reference)
	}
	if f.EmailHash != "" {
		q = q.Where("t.email_hash = ?", f.EmailHash)
	}
	if f.Words != "" {
		q = q.Where("to_tsvector('simple', t.subject) @@ plainto_tsquery('simple', ?)", f.Words)
	}
	return q
}

// AdminList implements SupportRepository.
func (r *GormSupportRepository) AdminList(ctx context.Context, f SupportAdminFilter) ([]SupportAdminRow, error) {
	limit := clampLimit(f.Limit, 26, 101)
	c := f.Cursor
	switch f.Sort {
	case SupportSortOldest:
		q := r.adminBase(ctx, f)
		if c != nil {
			q = q.Where("(t.created_at, t.id) > (?, ?)", c.At, c.ID)
		}
		var out []SupportAdminRow
		err := q.Order("t.created_at ASC, t.id ASC").Limit(limit).Scan(&out).Error
		return out, err
	case SupportSortWaiting:
		// The queue: open tickets first, the oldest unanswered customer
		// message at the top.
		if onlyOpen(f.Statuses) {
			q := r.adminBase(ctx, f)
			if c != nil {
				q = q.Where("(t.last_customer_message_at, t.id) > (?, ?)", c.At, c.ID)
			}
			var out []SupportAdminRow
			err := q.Order("t.last_customer_message_at ASC, t.id ASC").Limit(limit).Scan(&out).Error
			return out, err
		}
		return r.waitingRanked(ctx, f, limit)
	}
	q := r.adminBase(ctx, f)
	if c != nil {
		q = q.Where("(t.created_at, t.id) < (?, ?)", c.At, c.ID)
	}
	var out []SupportAdminRow
	err := q.Order("t.created_at DESC, t.id DESC").Limit(limit).Scan(&out).Error
	return out, err
}

// waitingRanked serves the waiting sort across mixed statuses as two keyset
// ranges, each on its own partial index: rank 0 (open tickets, by oldest
// customer message) then rank 1 (every other status). A single ORDER BY on a
// CASE expression could not use an index at all.
func (r *GormSupportRepository) waitingRanked(ctx context.Context, f SupportAdminFilter, limit int) ([]SupportAdminRow, error) {
	c := f.Cursor
	var out []SupportAdminRow
	wantOpen := len(f.Statuses) == 0
	var rest []domain.SupportStatus
	for _, s := range f.Statuses {
		if s == domain.SupportOpen {
			wantOpen = true
		} else {
			rest = append(rest, s)
		}
	}
	if wantOpen && (c == nil || c.Rank == 0) {
		of := f
		of.Statuses = []domain.SupportStatus{domain.SupportOpen}
		q := r.adminBase(ctx, of)
		if c != nil {
			q = q.Where("(t.last_customer_message_at, t.id) > (?, ?)", c.At, c.ID)
		}
		if err := q.Order("t.last_customer_message_at ASC, t.id ASC").Limit(limit).Scan(&out).Error; err != nil {
			return nil, err
		}
	}
	if len(out) >= limit || (len(f.Statuses) > 0 && len(rest) == 0) {
		return out, nil
	}
	rf := f
	rf.Statuses = rest
	q := r.adminBase(ctx, rf).Where("t.status <> 'open'")
	if c != nil && c.Rank == 1 {
		q = q.Where("(t.last_customer_message_at, t.id) > (?, ?)", c.At, c.ID)
	}
	var tail []SupportAdminRow
	if err := q.Order("t.last_customer_message_at ASC, t.id ASC").Limit(limit - len(out)).Scan(&tail).Error; err != nil {
		return nil, err
	}
	return append(out, tail...), nil
}

func onlyOpen(statuses []domain.SupportStatus) bool {
	if len(statuses) == 0 {
		return false
	}
	for _, s := range statuses {
		if s != domain.SupportOpen {
			return false
		}
	}
	return true
}

// Summary implements SupportRepository: one statement of index-only counts.
func (r *GormSupportRepository) Summary(ctx context.Context, adminID uuid.UUID) (SupportCounts, error) {
	var row struct {
		Open, Pending, Resolved, Closed, Unassigned, Mine, ReportsOpen int64
	}
	err := database.Conn(ctx, r.db).Raw(`SELECT
		(SELECT count(*) FROM support_tickets WHERE deleted_at IS NULL AND status = 'open') AS open,
		(SELECT count(*) FROM support_tickets WHERE deleted_at IS NULL AND status = 'pending') AS pending,
		(SELECT count(*) FROM support_tickets WHERE deleted_at IS NULL AND status = 'resolved') AS resolved,
		(SELECT count(*) FROM support_tickets WHERE deleted_at IS NULL AND status = 'closed') AS closed,
		(SELECT count(*) FROM support_tickets WHERE deleted_at IS NULL AND assignee_id IS NULL AND status IN ('open', 'pending')) AS unassigned,
		(SELECT count(*) FROM support_tickets WHERE deleted_at IS NULL AND assignee_id = ? AND status IN ('open', 'pending')) AS mine,
		(SELECT count(*) FROM support_tickets WHERE deleted_at IS NULL AND kind = 'chat_report' AND status IN ('open', 'pending')) AS reports_open`,
		adminID).Scan(&row).Error
	return SupportCounts{Open: row.Open, Pending: row.Pending, Resolved: row.Resolved, Closed: row.Closed,
		Unassigned: row.Unassigned, Mine: row.Mine, ReportsOpen: row.ReportsOpen}, err
}

// ListForUser implements SupportRepository.
func (r *GormSupportRepository) ListForUser(ctx context.Context, userID uuid.UUID, cursor *Cursor, limit int) ([]domain.SupportTicket, error) {
	q := database.Conn(ctx, r.db).
		Select("id, reference, user_id, name, topic, subject, locale, status, created_at, updated_at, last_message_at, "+
			"kind, conversation_id, session_id, reported_user_id, report_reason").
		Where("user_id = ?", userID)
	q = applyCursor(q, cursor, "last_message_at", "id")
	var out []domain.SupportTicket
	err := q.Order("last_message_at DESC, id DESC").Limit(clampLimit(limit, 21, 51)).Find(&out).Error
	return out, err
}

// OpenReport implements SupportRepository.
func (r *GormSupportRepository) OpenReport(ctx context.Context, userID, conversationID uuid.UUID) (*domain.SupportTicket, error) {
	var t domain.SupportTicket
	err := database.Conn(ctx, r.db).
		Where("user_id = ? AND conversation_id = ? AND kind = 'chat_report' AND status IN ('open', 'pending') AND deleted_at IS NULL",
			userID, conversationID).Take(&t).Error
	if err != nil {
		return nil, notFound(err)
	}
	return &t, nil
}

// ClaimResolved implements SupportRepository.
func (r *GormSupportRepository) ClaimResolved(ctx context.Context, before time.Time, limit int) ([]domain.SupportTicket, error) {
	var out []domain.SupportTicket
	err := database.Conn(ctx, r.db).Clauses(forUpdateSkipLocked).
		Where("status = ? AND resolved_at < ?", domain.SupportResolved, before).
		Order("resolved_at ASC").Limit(clampLimit(limit, 200, 500)).Find(&out).Error
	return out, err
}

// DeleteClosed implements SupportRepository.
func (r *GormSupportRepository) DeleteClosed(ctx context.Context, before time.Time, limit int) (int64, error) {
	res := database.Conn(ctx, r.db).Exec(`DELETE FROM support_tickets WHERE id IN (
		SELECT id FROM support_tickets WHERE status = 'closed' AND closed_at < ? ORDER BY closed_at LIMIT ? FOR UPDATE SKIP LOCKED)`,
		before, clampLimit(limit, 500, 2000))
	return res.RowsAffected, res.Error
}

// Unmasked implements SupportRepository.
func (r *GormSupportRepository) Unmasked(ctx context.Context, limit int) ([]domain.SupportTicket, error) {
	var out []domain.SupportTicket
	err := database.Conn(ctx, r.db).Select("id, email_enc").Where("email_masked = ''").
		Order("created_at ASC").Limit(clampLimit(limit, 200, 500)).Find(&out).Error
	return out, err
}
