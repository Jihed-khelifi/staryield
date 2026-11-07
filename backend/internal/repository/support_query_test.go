package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 3.0: support desk. The SQL of the admin queue keeps its shape:
// keyset predicates, no OFFSET, search through the GIN expression and the
// ranked "waiting" sort split into two index-friendly ranges.

func supportSQL(t *testing.T, f SupportAdminFilter) string {
	t.Helper()
	var last string
	db := dryRunDB(t)
	capture := func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.SQL.Len() > 0 {
			last = tx.Statement.SQL.String()
		}
	}
	_ = db.Callback().Query().After("gorm:query").Register("support:capture_query", capture)
	_ = db.Callback().Row().After("gorm:row").Register("support:capture_row", capture)
	f.Limit = 26
	_, _ = NewSupportRepository(db).AdminList(context.Background(), f)
	return last
}

func TestSupportQueueQueryShapes(t *testing.T) {
	at, id := time.Now(), uuid.New()
	cases := map[string]struct {
		f    SupportAdminFilter
		want []string
	}{
		"newest": {SupportAdminFilter{Sort: SupportSortNewest, Cursor: &SupportCursor{At: at, ID: id}},
			[]string{"(t.created_at, t.id) < ($", "ORDER BY t.created_at DESC, t.id DESC", "t.deleted_at IS NULL"}},
		"oldest": {SupportAdminFilter{Sort: SupportSortOldest, Cursor: &SupportCursor{At: at, ID: id}},
			[]string{"(t.created_at, t.id) > ($", "ORDER BY t.created_at ASC, t.id ASC"}},
		"waiting open": {SupportAdminFilter{Sort: SupportSortWaiting, Statuses: []domain.SupportStatus{domain.SupportOpen}},
			[]string{"ORDER BY t.last_customer_message_at ASC, t.id ASC", "t.status IN"}},
		"waiting rest (rank 1)": {SupportAdminFilter{Sort: SupportSortWaiting, Statuses: []domain.SupportStatus{domain.SupportPending},
			Cursor: &SupportCursor{At: at, ID: id, Rank: 1}},
			[]string{"t.status <> 'open'", "(t.last_customer_message_at, t.id) > ($", "ORDER BY t.last_customer_message_at ASC, t.id ASC"}},
		"search": {SupportAdminFilter{Words: "refund", Reference: "SY-ABCDEF", EmailHash: "h", Priority: "high", Topic: "billing", Unassigned: true},
			[]string{"to_tsvector('simple', t.subject) @@ plainto_tsquery('simple', $", "t.reference = $", "t.email_hash = $", "t.priority = $",
				"t.topic = $", "t.assignee_id IS NULL"}},
	}
	for name, tc := range cases {
		sql := supportSQL(t, tc.f)
		for _, want := range tc.want {
			if !strings.Contains(sql, want) {
				t.Errorf("%s: %q missing from %s", name, want, sql)
			}
		}
		for _, banned := range []string{"OFFSET", "CASE WHEN", "email_enc", "message_enc", "access_token"} {
			if strings.Contains(sql, banned) {
				t.Errorf("%s: %q must not appear: %s", name, banned, sql)
			}
		}
	}
}
