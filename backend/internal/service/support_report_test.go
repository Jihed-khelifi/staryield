package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// revisions 3.1: conversation reports. In-memory fakes: just enough of the
// repositories for the rules (authorization, session resolution, duplicates,
// views, summary, export).

type reportRepo struct {
	*fakeSupportRepo
	hideOpenOnce bool // simulate a race: the pre-check misses, the unique index fires
	adminRows    []repository.SupportAdminRow
	lastFilter   repository.SupportAdminFilter
	counts       repository.SupportCounts
}

func (r *reportRepo) Create(_ context.Context, t *domain.SupportTicket) error {
	if t.Kind == domain.TicketChatReport && (t.Status == domain.SupportOpen || t.Status == domain.SupportPending) {
		for _, o := range r.tickets {
			if o.Kind == domain.TicketChatReport && o.UserID != nil && t.UserID != nil && *o.UserID == *t.UserID &&
				derefID(o.ConversationID) == derefID(t.ConversationID) && (o.Status == domain.SupportOpen || o.Status == domain.SupportPending) {
				return repository.ErrDuplicateReport
			}
		}
	}
	cp := *t
	r.tickets[t.ID] = &cp
	return nil
}

func (r *reportRepo) OpenReport(_ context.Context, userID, conv uuid.UUID) (*domain.SupportTicket, error) {
	if r.hideOpenOnce {
		r.hideOpenOnce = false
		return nil, repository.ErrNotFound
	}
	for _, o := range r.tickets {
		if o.Kind == domain.TicketChatReport && o.UserID != nil && *o.UserID == userID && derefID(o.ConversationID) == conv &&
			(o.Status == domain.SupportOpen || o.Status == domain.SupportPending) {
			cp := *o
			return &cp, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *reportRepo) Get(_ context.Context, id uuid.UUID) (*domain.SupportTicket, error) {
	if t, ok := r.tickets[id]; ok {
		cp := *t
		return &cp, nil
	}
	return nil, repository.ErrNotFound
}

func (r *reportRepo) AdminList(_ context.Context, f repository.SupportAdminFilter) ([]repository.SupportAdminRow, error) {
	r.lastFilter = f
	return r.adminRows, nil
}

func (r *reportRepo) Summary(context.Context, uuid.UUID) (repository.SupportCounts, error) {
	return r.counts, nil
}

type fakeChat struct {
	repository.ChatRepository
	convs    map[uuid.UUID]*domain.Conversation
	sessions map[uuid.UUID]*domain.Session
	batches  int
}

func (f *fakeChat) GetConversation(_ context.Context, id uuid.UUID) (*domain.Conversation, error) {
	if c, ok := f.convs[id]; ok {
		return c, nil
	}
	return nil, repository.ErrNotFound
}

func (f *fakeChat) GetSession(_ context.Context, id uuid.UUID) (*domain.Session, error) {
	if s, ok := f.sessions[id]; ok {
		return s, nil
	}
	return nil, repository.ErrNotFound
}

func (f *fakeChat) LatestSession(_ context.Context, conv uuid.UUID) (*domain.Session, error) {
	var best *domain.Session
	for _, s := range f.sessions {
		if s.ConversationID == conv && (best == nil || s.RequestedAt.After(best.RequestedAt)) {
			best = s
		}
	}
	if best == nil {
		return nil, repository.ErrNotFound
	}
	return best, nil
}

func (f *fakeChat) GetSessions(_ context.Context, ids []uuid.UUID) ([]domain.Session, error) {
	f.batches++
	var out []domain.Session
	for _, id := range ids {
		if s, ok := f.sessions[id]; ok {
			out = append(out, *s)
		}
	}
	return out, nil
}

type fakePsychics struct {
	repository.PsychicRepository
	byUser map[uuid.UUID]domain.Psychic
	calls  int
}

func (f *fakePsychics) GetByUserIDs(_ context.Context, ids []uuid.UUID) ([]domain.Psychic, error) {
	f.calls++
	var out []domain.Psychic
	for _, id := range ids {
		if p, ok := f.byUser[id]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

type captureNotifier struct {
	NoopSupportNotifier
	created []*domain.SupportTicket
	message []string
}

func (c *captureNotifier) TicketCreated(_ context.Context, t *domain.SupportTicket, _ string, message, _ string) {
	cp := *t
	c.created = append(c.created, &cp)
	c.message = append(c.message, message)
}

type reportEnv struct {
	svc      *SupportService
	repo     *reportRepo
	chat     *fakeChat
	psychics *fakePsychics
	notify   *captureNotifier
	clk      *clock.Fake
	client   reqctx.Principal
	other    reqctx.Principal
	advisor  reqctx.Principal
	conv     *domain.Conversation
	sessOld  *domain.Session
	sessNew  *domain.Session
	foreign  *domain.Session // a session of another conversation
}

func newReportEnv(t *testing.T) *reportEnv {
	t.Helper()
	sealer, err := security.NewSealer([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	clk := clock.NewFake(testT0)
	e := &reportEnv{clk: clk, repo: &reportRepo{fakeSupportRepo: newFakeSupportRepo()}, notify: &captureNotifier{}}
	e.client = reqctx.Principal{UserID: uuid.New(), Role: domain.RoleClient}
	e.other = reqctx.Principal{UserID: uuid.New(), Role: domain.RoleClient}
	e.advisor = reqctx.Principal{UserID: uuid.New(), Role: domain.RolePsychic}
	e.conv = &domain.Conversation{ID: uuid.New(), ClientID: e.client.UserID, PsychicID: e.advisor.UserID, LastSeq: 42}
	otherConv := &domain.Conversation{ID: uuid.New(), ClientID: e.other.UserID, PsychicID: e.advisor.UserID, LastSeq: 5}
	started := testT0.Add(-2 * time.Hour)
	e.sessOld = &domain.Session{ID: uuid.New(), ConversationID: e.conv.ID, Status: domain.SessionSettled, RequestedAt: testT0.Add(-48 * time.Hour)}
	e.sessNew = &domain.Session{ID: uuid.New(), ConversationID: e.conv.ID, Status: domain.SessionSettled, RequestedAt: testT0.Add(-3 * time.Hour),
		StartedAt: &started, EndReason: domain.EndClientEnded, ActiveSeconds: 300, BilledMinutes: 5, CreditsCharged: 150, RatePerMinute: 30, Sponsored: true}
	e.foreign = &domain.Session{ID: uuid.New(), ConversationID: otherConv.ID, RequestedAt: testT0}
	e.chat = &fakeChat{convs: map[uuid.UUID]*domain.Conversation{e.conv.ID: e.conv, otherConv.ID: otherConv},
		sessions: map[uuid.UUID]*domain.Session{e.sessOld.ID: e.sessOld, e.sessNew.ID: e.sessNew, e.foreign.ID: e.foreign}}
	e.psychics = &fakePsychics{byUser: map[uuid.UUID]domain.Psychic{e.advisor.UserID: {UserID: e.advisor.UserID, Slug: "luna-vale", Name: "Luna Vale",
		ImageURL: "https://img.test/luna.jpg"}}}
	users := fakeStaff{users: map[uuid.UUID]domain.User{
		e.client.UserID:  {ID: e.client.UserID, Email: "Jane.Doe@Example.com", DisplayName: "Jane Doe", Role: domain.RoleClient, Status: domain.UserActive},
		e.advisor.UserID: {ID: e.advisor.UserID, Email: "luna@example.com", DisplayName: "Luna", Role: domain.RolePsychic, Status: domain.UserActive},
	}}
	e.svc = NewSupportService(e.repo, sealer, e.notify, clk, "ip-secret").WithDesk(SupportDeskDeps{Tx: inlineTx{}, Users: users,
		Cache: cache.NewMemory(clk.Now), Chat: e.chat, Psychics: e.psychics,
		Settings: SupportSettings{FrontendURL: "https://staryield.test", LinkTTL: 720 * time.Hour, Retention: 720 * time.Hour}})
	return e
}

func (e *reportEnv) ctx() context.Context {
	return reqctx.WithMeta(context.Background(), reqctx.Meta{Locale: "fr", IP: "203.0.113.9"})
}

func (e *reportEnv) report(reason string) ChatReportInput {
	return ChatReportInput{ConversationID: e.conv.ID.String(), Reason: reason}
}

func appErr(t *testing.T, err error) *apperror.Error {
	t.Helper()
	ae, ok := apperror.As(err)
	if !ok {
		t.Fatalf("want an application error, got %v", err)
	}
	return ae
}

func TestValidateReportTable(t *testing.T) {
	conv := uuid.New().String()
	long := strings.Repeat("é", 2001)
	cases := []struct {
		name  string
		in    ChatReportInput
		field string // "" = valid
		msg   string
	}{
		{"valid without details", ChatReportInput{ConversationID: conv, Reason: "billing"}, "", ""},
		{"reason is case-insensitive", ChatReportInput{ConversationID: conv, Reason: " Technical "}, "", ""},
		{"unknown reason", ChatReportInput{ConversationID: conv, Reason: "spam"}, "reason", msgReportReason},
		{"missing reason", ChatReportInput{ConversationID: conv}, "reason", msgReportReason},
		{"bad conversation id", ChatReportInput{ConversationID: "nope", Reason: "billing"}, "conversation_id", msgReportID},
		{"bad session id", ChatReportInput{ConversationID: conv, Reason: "billing", SessionID: "x"}, "session_id", msgReportID},
		{"details at the limit", ChatReportInput{ConversationID: conv, Reason: "billing", Details: strings.Repeat("é", 2000)}, "", ""},
		{"details over the limit", ChatReportInput{ConversationID: conv, Reason: "billing", Details: long}, "details", msgReportDetails},
		{"other needs 10 characters", ChatReportInput{ConversationID: conv, Reason: "other", Details: "too short"}, "details", msgReportOther},
		{"other without details", ChatReportInput{ConversationID: conv, Reason: "other"}, "details", msgReportOther},
		{"other with exactly 10", ChatReportInput{ConversationID: conv, Reason: "other", Details: "0123456789"}, "", ""},
		{"other over the limit", ChatReportInput{ConversationID: conv, Reason: "other", Details: long}, "details", msgReportOther},
		{"short details are fine for other reasons", ChatReportInput{ConversationID: conv, Reason: "inappropriate", Details: "rude"}, "", ""},
		{"whitespace-only details count as empty", ChatReportInput{ConversationID: conv, Reason: "other", Details: "   \n  "}, "details", msgReportOther},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateReport(tc.in)
			if tc.field == "" {
				if err != nil {
					t.Fatalf("want valid, got %v", err)
				}
				return
			}
			f := deskFields(appErr(t, err))
			if f[tc.field] != tc.msg {
				t.Fatalf("fields = %v, want %s: %q", f, tc.field, tc.msg)
			}
		})
	}
	got, _ := validateReport(ChatReportInput{ConversationID: conv, Reason: "billing", Details: "Hello\u0007 there  ", Seq: -4})
	if got.details != "Hello there" || got.seq != 0 {
		t.Fatalf("details are sanitized and a negative seq becomes 0: %+v", got)
	}
}

func TestReportTopicPriorityAndSubject(t *testing.T) {
	topics := map[domain.ChatReportReason]domain.SupportTopic{domain.ReportAdvisorUnresponsive: domain.SupportAdvisor,
		domain.ReportInappropriate: domain.SupportAdvisor, domain.ReportBilling: domain.SupportBilling,
		domain.ReportTechnical: domain.SupportTechnical, domain.ReportOther: domain.SupportGeneral}
	for r, want := range topics {
		if got := r.Topic(); got != want {
			t.Errorf("topic(%s) = %s, want %s", r, got, want)
		}
		wantP := domain.SupportHigh
		if r == domain.ReportInappropriate {
			wantP = domain.SupportUrgent
		}
		if got := r.Priority(); got != wantP {
			t.Errorf("priority(%s) = %s, want %s", r, got, wantP)
		}
	}
	if got := reportSubject(domain.ReportBilling, "Luna Vale"); got != "Chat report · Billing issue · Luna Vale" {
		t.Fatalf("subject = %q", got)
	}
	long := reportSubject(domain.ReportInappropriate, strings.Repeat("Ñ", 300))
	if n := utf8Len(long); n > 120 || !strings.HasPrefix(long, "Chat report · Inappropriate behavior · ÑÑ") || !strings.HasSuffix(long, "…") {
		t.Fatalf("subject must be truncated to 120 characters (%d): %q", n, long)
	}
	if got := reportSubject(domain.ReportOther, ""); got != "Chat report · Other" {
		t.Fatalf("subject without advisor = %q", got)
	}
	if n := utf8Len(reportSubject(domain.ReportAdvisorUnresponsive, strings.Repeat("x", 120))); n != 120 {
		t.Fatalf("subject length = %d, want exactly 120", n)
	}
}

func TestCreateChatReportAuthorization(t *testing.T) {
	e := newReportEnv(t)
	adminP := reqctx.Principal{UserID: uuid.New(), Role: domain.RoleAdmin}
	for name, p := range map[string]reqctx.Principal{"advisor": e.advisor, "admin": adminP} {
		_, err := e.svc.CreateChatReport(e.ctx(), p, e.report("billing"))
		if ae := appErr(t, err); ae.Status != 403 || ae.Code != apperror.CodeForbidden {
			t.Errorf("%s: want 403 forbidden, got %v", name, err)
		}
	}
	if _, err := e.svc.CreateChatReport(e.ctx(), reqctx.Principal{}, e.report("billing")); appErr(t, err).Status != 401 {
		t.Errorf("anonymous must be 401: %v", err)
	}
	// Another client's conversation and an unknown one answer the same 404.
	_, errOther := e.svc.CreateChatReport(e.ctx(), e.other, e.report("billing"))
	unknown := e.report("billing")
	unknown.ConversationID = uuid.New().String()
	_, errUnknown := e.svc.CreateChatReport(e.ctx(), e.client, unknown)
	a, b := appErr(t, errOther), appErr(t, errUnknown)
	if a.Status != 404 || b.Status != 404 || a.Code != b.Code || a.Message != b.Message {
		t.Fatalf("foreign and unknown conversations must be indistinguishable: %v / %v", errOther, errUnknown)
	}
	if len(e.repo.tickets) != 0 || len(e.notify.created) != 0 {
		t.Fatal("nothing may be stored or sent when authorization fails")
	}
	// The open-report lookup follows the same rule.
	if _, err := e.svc.OpenChatReport(e.ctx(), e.other.UserID, e.conv.ID); appErr(t, err).Status != 404 {
		t.Fatalf("lookup of a foreign conversation must be 404: %v", err)
	}
	if _, err := e.svc.OpenChatReport(e.ctx(), e.advisor.UserID, e.conv.ID); appErr(t, err).Status != 404 {
		t.Fatalf("an advisor never sees whether a report exists: %v", err)
	}
}

func TestCreateChatReportStoresTicket(t *testing.T) {
	e := newReportEnv(t)
	in := e.report("inappropriate")
	in.Details = "  They were rude.\u0007 "
	in.Seq = 9999 // clamped to the conversation's last_seq
	out, err := e.svc.CreateChatReport(e.ctx(), e.client, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.repo.tickets) != 1 {
		t.Fatalf("tickets: %d", len(e.repo.tickets))
	}
	var tk *domain.SupportTicket
	for _, x := range e.repo.tickets {
		tk = x
	}
	if tk.Kind != domain.TicketChatReport || tk.Topic != domain.SupportAdvisor || tk.Priority != domain.SupportUrgent || tk.Status != domain.SupportOpen {
		t.Fatalf("derived fields: %+v", tk)
	}
	if tk.Subject != "Chat report · Inappropriate behavior · Luna Vale" || tk.Name != "Jane Doe" || tk.Locale != "fr" {
		t.Fatalf("subject/name/locale: %q %q %q", tk.Subject, tk.Name, tk.Locale)
	}
	if *tk.UserID != e.client.UserID || *tk.ConversationID != e.conv.ID || *tk.ReportedUserID != e.advisor.UserID || *tk.ReportReason != domain.ReportInappropriate {
		t.Fatalf("links: %+v", tk)
	}
	if *tk.SessionID != e.sessNew.ID {
		t.Fatalf("no session_id: the latest session by requested_at must be used, got %v", *tk.SessionID)
	}
	if tk.ReportedSeq != 42 {
		t.Fatalf("seq must be clamped to last_seq: %d", tk.ReportedSeq)
	}
	// Sealed like a contact ticket: address and details are never stored in clear.
	if strings.Contains(tk.EmailEnc, "example.com") || strings.Contains(tk.MessageEnc, "rude") {
		t.Fatal("address and details must be sealed")
	}
	if got, _ := e.svc.sealer.Open(tk.EmailEnc, supportAD(tk.ID, "email")); got != "jane.doe@example.com" {
		t.Fatalf("sealed address = %q", got)
	}
	if got, _ := e.svc.sealer.Open(tk.MessageEnc, supportAD(tk.ID, "message")); got != "They were rude." {
		t.Fatalf("sealed details = %q", got)
	}
	if tk.EmailHash != security.Keyed("ip-secret", "support-email:jane.doe@example.com", 0) || tk.EmailMasked != "j•••@e•••.com" {
		t.Fatalf("hash/mask: %q %q", tk.EmailHash, tk.EmailMasked)
	}
	if tk.AccessTokenHash == nil || tk.AccessTokenEnc == nil || tk.AccessTokenExpiresAt == nil || tk.MessageCount != 1 || tk.Version != 1 {
		t.Fatalf("private link / desk fields: %+v", tk)
	}
	if len(e.notify.created) != 1 || e.notify.message[0] != "They were rude." {
		t.Fatalf("the notifier must be told once: %d", len(e.notify.created))
	}
	if g, _, _ := e.svc.desk.Cache.Get(context.Background(), supportGenKey); g == "" {
		t.Fatal("the summary cache must be invalidated")
	}
	// The receipt is a CustomerTicket with the report context.
	if out.Kind != "chat_report" || out.Report == nil || out.Report.Reason != "inappropriate" || out.Report.Advisor == nil ||
		out.Report.Advisor.Name != "Luna Vale" || out.Report.Advisor.Slug != "luna-vale" || out.Report.Advisor.AvatarURL != "https://img.test/luna.jpg" {
		t.Fatalf("view: %+v", out)
	}
	if out.Report.SessionStartedAt == nil || !out.Report.SessionStartedAt.Equal(*e.sessNew.StartedAt) {
		t.Fatalf("session_started_at is started_at: %v", out.Report.SessionStartedAt)
	}
	if out.Awaiting != "support" || !out.CanReply || len(out.Messages) != 1 || out.Messages[0].Body != "They were rude." {
		t.Fatalf("thread: %+v", out)
	}
}

func TestCreateChatReportSessionRules(t *testing.T) {
	e := newReportEnv(t)
	// A given session of the conversation is used (not the latest).
	in := e.report("billing")
	in.SessionID = e.sessOld.ID.String()
	out, err := e.svc.CreateChatReport(e.ctx(), e.client, in)
	if err != nil {
		t.Fatal(err)
	}
	if *out.Report.SessionID != e.sessOld.ID || out.Report.SessionStartedAt == nil || !out.Report.SessionStartedAt.Equal(e.sessOld.RequestedAt) {
		t.Fatalf("session_started_at falls back to requested_at: %+v", out.Report)
	}
	// Sessions of another conversation, and unknown ones, are a field error (no leak).
	e2 := newReportEnv(t)
	for name, id := range map[string]string{"foreign": e2.foreign.ID.String(), "unknown": uuid.New().String()} {
		in := e2.report("billing")
		in.SessionID = id
		_, err := e2.svc.CreateChatReport(e2.ctx(), e2.client, in)
		ae := appErr(t, err)
		if ae.Status != 422 || deskFields(ae)["session_id"] != msgReportSession {
			t.Errorf("%s session: %v", name, err)
		}
	}
	if len(e2.repo.tickets) != 0 {
		t.Fatal("nothing is stored on a session mismatch")
	}
	// A conversation without any session still accepts a report.
	e3 := newReportEnv(t)
	e3.chat.sessions = map[uuid.UUID]*domain.Session{}
	out, err = e3.svc.CreateChatReport(e3.ctx(), e3.client, e3.report("technical"))
	if err != nil || out.Report.SessionID != nil || out.Report.SessionStartedAt != nil {
		t.Fatalf("no session: %+v %v", out, err)
	}
	// seq 0 / negative stays 0; in-range is kept.
	e4 := newReportEnv(t)
	in = e4.report("technical")
	in.Seq = 7
	if _, err := e4.svc.CreateChatReport(e4.ctx(), e4.client, in); err != nil {
		t.Fatal(err)
	}
	for _, tk := range e4.repo.tickets {
		if tk.ReportedSeq != 7 {
			t.Fatalf("reported_seq = %d", tk.ReportedSeq)
		}
	}
}

func TestCreateChatReportWithoutDetailsHasNoOriginalMessage(t *testing.T) {
	e := newReportEnv(t)
	out, err := e.svc.CreateChatReport(e.ctx(), e.client, e.report("advisor_unresponsive"))
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Messages) != 0 {
		t.Fatalf("an empty report has no original text: %+v", out.Messages)
	}
	for _, tk := range e.repo.tickets {
		if tk.MessageChars != 0 || tk.Priority != domain.SupportHigh || tk.Topic != domain.SupportAdvisor {
			t.Fatalf("%+v", tk)
		}
	}
}

func TestCreateChatReportDuplicates(t *testing.T) {
	e := newReportEnv(t)
	first, err := e.svc.CreateChatReport(e.ctx(), e.client, e.report("billing"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.CreateChatReport(e.ctx(), e.client, e.report("technical"))
	ae := appErr(t, err)
	if ae.Status != 409 || ae.Code != apperror.CodeReportExists || ae.Details["reference"] != first.Reference {
		t.Fatalf("want 409 report_exists with the reference, got %v (%v)", err, ae.Details)
	}
	if len(e.repo.tickets) != 1 {
		t.Fatalf("a duplicate must not create a ticket: %d", len(e.repo.tickets))
	}
	// Pending still counts as open.
	for _, tk := range e.repo.tickets {
		tk.Status = domain.SupportPending
	}
	if _, err := e.svc.CreateChatReport(e.ctx(), e.client, e.report("billing")); appErr(t, err).Code != apperror.CodeReportExists {
		t.Fatalf("pending must block a new report: %v", err)
	}
	// Race: the pre-check misses, the unique index fires; the same error comes back.
	e.repo.hideOpenOnce = true
	_, err = e.svc.CreateChatReport(e.ctx(), e.client, e.report("billing"))
	ae = appErr(t, err)
	if ae.Code != apperror.CodeReportExists || ae.Details["reference"] != first.Reference {
		t.Fatalf("race path: %v (%v)", err, ae.Details)
	}
	// After the previous report is resolved (or closed) a new one is allowed.
	for _, tk := range e.repo.tickets {
		tk.Status = domain.SupportResolved
	}
	if _, err := e.svc.CreateChatReport(e.ctx(), e.client, e.report("billing")); err != nil {
		t.Fatalf("a new report after resolution: %v", err)
	}
	// The open lookup returns the new one only.
	row, err := e.svc.OpenChatReport(e.ctx(), e.client.UserID, e.conv.ID)
	if err != nil || row == nil || row.Reference == first.Reference || row.Kind != "chat_report" || row.Report == nil {
		t.Fatalf("open report: %+v %v", row, err)
	}
	for _, tk := range e.repo.tickets {
		tk.Status = domain.SupportClosed
	}
	if row, err := e.svc.OpenChatReport(e.ctx(), e.client.UserID, e.conv.ID); err != nil || row != nil {
		t.Fatalf("closed reports are not open: %+v %v", row, err)
	}
}

func TestContactTicketsKeepTheirView(t *testing.T) {
	e := newReportEnv(t)
	tk := &domain.SupportTicket{ID: uuid.New(), Reference: "SY-AAAAAA", Topic: domain.SupportGeneral, Status: domain.SupportOpen}
	row := customerRow(tk)
	if row.Kind != "contact" || row.Report != nil {
		t.Fatalf("contact row: %+v", row)
	}
	if ctxs := e.svc.customerReports(context.Background(), []*domain.SupportTicket{tk}); len(ctxs) != 0 {
		t.Fatal("a contact ticket has no report context")
	}
	if c := adminCommon(tk, ""); c.Kind != "contact" {
		t.Fatalf("admin kind = %q", c.Kind)
	}
	if e.psychics.calls != 0 {
		t.Fatal("contact tickets must not trigger advisor lookups")
	}
}

func TestAdminQueueReportsAreBatchedAndKindIsValidated(t *testing.T) {
	e := newReportEnv(t)
	admin := uuid.New()
	mk := func(withSession bool) repository.SupportAdminRow {
		reason := domain.ReportBilling
		tk := domain.SupportTicket{ID: uuid.New(), Reference: "SY-BBBBBB", Kind: domain.TicketChatReport, ReportReason: &reason, ReportedSeq: 7,
			ReportedUserID: &e.advisor.UserID, ConversationID: &e.conv.ID, Status: domain.SupportOpen, EmailMasked: "j•••@e•••.com"}
		if withSession {
			tk.SessionID = &e.sessNew.ID
		}
		return repository.SupportAdminRow{SupportTicket: tk}
	}
	contact := repository.SupportAdminRow{SupportTicket: domain.SupportTicket{ID: uuid.New(), Reference: "SY-CCCCCC", Kind: domain.TicketContact}}
	e.repo.adminRows = []repository.SupportAdminRow{mk(true), mk(false), contact, mk(true)}
	page, err := e.svc.AdminTickets(e.ctx(), admin, SupportQuery{Kind: "chat_report"})
	if err != nil {
		t.Fatal(err)
	}
	if e.repo.lastFilter.Kind != domain.TicketChatReport {
		t.Fatalf("filter: %+v", e.repo.lastFilter)
	}
	if e.psychics.calls != 1 || e.chat.batches != 1 {
		t.Fatalf("advisors and sessions must be loaded once per page: %d / %d", e.psychics.calls, e.chat.batches)
	}
	r0 := page.Items[0]
	if r0.Kind != "chat_report" || r0.Report == nil || r0.Report.ReportedSeq != 7 || r0.Report.Reason != "billing" ||
		r0.Report.Advisor == nil || r0.Report.Advisor.UserID != e.advisor.UserID || r0.Report.Advisor.Name != "Luna Vale" ||
		r0.Report.Advisor.Slug != "luna-vale" || r0.Report.Advisor.PsychicID == nil {
		t.Fatalf("report context: %+v", r0.Report)
	}
	if page.Items[2].Kind != "contact" || page.Items[2].Report != nil {
		t.Fatalf("contact row must be unchanged: %+v", page.Items[2])
	}
	_, err = e.svc.AdminTickets(e.ctx(), admin, SupportQuery{Kind: "spam"})
	if ae := appErr(t, err); ae.Status != 422 || deskFields(ae)["kind"] != msgSupportKind {
		t.Fatalf("invalid kind must be 422: %v", err)
	}
	for _, k := range []string{"", "contact", "CHAT_REPORT"} {
		if _, err := e.svc.AdminTickets(e.ctx(), admin, SupportQuery{Kind: k}); err != nil {
			t.Errorf("kind %q: %v", k, err)
		}
	}
}

func TestAdminDetailCarriesReportAndSession(t *testing.T) {
	e := newReportEnv(t)
	in := e.report("billing")
	in.Details = "I was charged twice."
	created, err := e.svc.CreateChatReport(e.ctx(), e.client, in)
	if err != nil {
		t.Fatal(err)
	}
	var id uuid.UUID
	for k := range e.repo.tickets {
		id = k
	}
	tk, _ := e.repo.Get(context.Background(), id)
	d, err := e.svc.adminView(context.Background(), tk)
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != "chat_report" || d.Report == nil || d.Report.Session == nil {
		t.Fatalf("detail: %+v", d.Report)
	}
	s := d.Report.Session
	if s.ID != e.sessNew.ID || s.Status != "settled" || s.ActiveSeconds != 300 || s.BilledMinutes != 5 || s.CreditsCharged != 150 ||
		s.RatePerMinute != 30 || !s.Sponsored || s.EndReason != "client_ended" || s.StartedAt == nil {
		t.Fatalf("session summary: %+v", s)
	}
	if len(d.Messages) != 1 || d.Messages[0].Body != "I was charged twice." || created.Reference != d.Reference {
		t.Fatalf("thread: %+v", d.Messages)
	}
}

func TestSummaryCountsOpenReports(t *testing.T) {
	e := newReportEnv(t)
	e.repo.counts = repository.SupportCounts{Open: 5, Pending: 2, ReportsOpen: 3}
	sum, err := e.svc.AdminSummary(e.ctx(), uuid.New())
	if err != nil || sum.ReportsOpen != 3 || sum.Open != 5 {
		t.Fatalf("%+v %v", sum, err)
	}
}

func TestExportListsKindAndReportFields(t *testing.T) {
	e := newReportEnv(t)
	if _, err := e.svc.CreateChatReport(e.ctx(), e.client, e.report("technical")); err != nil {
		t.Fatal(err)
	}
	contact := &domain.SupportTicket{ID: uuid.New(), Reference: "SY-DDDDDD", Kind: domain.TicketContact, Topic: domain.SupportGeneral,
		Status: domain.SupportOpen, CreatedAt: testT0}
	contact.EmailEnc, _ = e.svc.sealer.Seal("jane@example.com", supportAD(contact.ID, "email"))
	contact.MessageEnc, _ = e.svc.sealer.Seal("hello there", supportAD(contact.ID, "message"))
	var all []domain.SupportTicket
	for _, tk := range e.repo.tickets {
		all = append(all, *tk)
	}
	all = append(all, *contact)
	ex := &ExportService{d: ExportDeps{Sealer: e.svc.sealer, Data: noRowsData{}}}
	b := &ExportBundle{}
	err := ex.collectSupport(context.Background(), b, e.client.UserID, func(dest any, _ string) error {
		*(dest.(*[]domain.SupportTicket)) = all
		return nil
	}, func(int, string) {})
	if err != nil {
		t.Fatal(err)
	}
	items := b.Documents[0].Data.([]map[string]any)
	byKind := map[string]map[string]any{}
	for _, it := range items {
		byKind[it["kind"].(string)] = it
	}
	rep, con := byKind["chat_report"], byKind["contact"]
	if rep == nil || con == nil {
		t.Fatalf("both kinds are exported: %v", items)
	}
	if rep["reason"] != domain.ReportTechnical || rep["conversation_id"] == nil || rep["session_id"] == nil {
		t.Fatalf("report fields: %v", rep)
	}
	if _, has := con["reason"]; has {
		t.Fatalf("contact entries have no report fields: %v", con)
	}
}

type noRowsData struct {
	repository.ExportDataRepository
}

func (noRowsData) Rows(context.Context, any, string, uuid.UUID, int) error { return nil }

func TestChatReportSubmittedIsAnAllowedClientEvent(t *testing.T) {
	if !ClientEvents["chat_report_submitted"] {
		t.Fatal("chat_report_submitted must be accepted from browsers")
	}
}
