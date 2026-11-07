package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// fakeSupportRepo is an in-memory support repository: just enough of the
// interface for the token, visibility and maintenance rules.
type fakeSupportRepo struct {
	repository.SupportRepository // unimplemented methods panic: tests only use what is below
	tickets                      map[uuid.UUID]*domain.SupportTicket
	messages                     []domain.SupportMessage
	deletedBefore                []time.Time
}

func newFakeSupportRepo() *fakeSupportRepo {
	return &fakeSupportRepo{tickets: map[uuid.UUID]*domain.SupportTicket{}}
}

func (f *fakeSupportRepo) Update(_ context.Context, id uuid.UUID, fields map[string]any) error {
	t, ok := f.tickets[id]
	if !ok {
		return repository.ErrNotFound
	}
	for k, v := range fields {
		switch k {
		case "status":
			t.Status = v.(domain.SupportStatus)
		case "closed_at":
			if v == nil {
				t.ClosedAt = nil
			} else {
				at := v.(time.Time)
				t.ClosedAt = &at
			}
		case "resolved_at":
			if v == nil {
				t.ResolvedAt = nil
			} else {
				at := v.(time.Time)
				t.ResolvedAt = &at
			}
		case "version":
			t.Version = v.(int)
		case "message_count":
			t.MessageCount = v.(int)
		case "last_message_at":
			t.LastMessageAt = v.(time.Time)
		case "email_masked":
			t.EmailMasked = v.(string)
		case "access_token_expires_at":
			at := v.(time.Time)
			t.AccessTokenExpiresAt = &at
		case "access_token_hash":
			h := v.(string)
			t.AccessTokenHash = &h
		case "access_token_enc":
			e := v.(string)
			t.AccessTokenEnc = &e
		}
	}
	return nil
}

func (f *fakeSupportRepo) ClaimResolved(_ context.Context, before time.Time, limit int) ([]domain.SupportTicket, error) {
	var out []domain.SupportTicket
	for _, t := range f.tickets {
		if t.Status == domain.SupportResolved && t.ResolvedAt != nil && t.ResolvedAt.Before(before) && len(out) < limit {
			out = append(out, *t)
		}
	}
	return out, nil
}

func (f *fakeSupportRepo) AddMessage(_ context.Context, m *domain.SupportMessage) error {
	f.messages = append(f.messages, *m)
	return nil
}

func (f *fakeSupportRepo) DeleteClosed(_ context.Context, before time.Time, limit int) (int64, error) {
	f.deletedBefore = append(f.deletedBefore, before)
	var n int64
	for id, t := range f.tickets {
		if t.Status == domain.SupportClosed && t.ClosedAt != nil && t.ClosedAt.Before(before) && n < int64(limit) {
			delete(f.tickets, id)
			n++
		}
	}
	return n, nil
}

func (f *fakeSupportRepo) Unmasked(_ context.Context, limit int) ([]domain.SupportTicket, error) {
	var out []domain.SupportTicket
	for _, t := range f.tickets {
		if t.EmailMasked == "" && len(out) < limit {
			out = append(out, *t)
		}
	}
	return out, nil
}

func (f *fakeSupportRepo) Messages(_ context.Context, ticketID uuid.UUID, publicOnly bool, _ int) ([]domain.SupportMessage, error) {
	var out []domain.SupportMessage
	for _, m := range f.messages {
		if m.TicketID != ticketID {
			continue
		}
		// The real repository filters in SQL when publicOnly; this fake returns
		// EVERYTHING so the service's own defence in depth is what gets tested.
		out = append(out, m)
	}
	return out, nil
}

// fakeStaff resolves staff display names.
type fakeStaff struct {
	repository.UserRepository
	users map[uuid.UUID]domain.User
}

func (f fakeStaff) GetByIDs(_ context.Context, ids []uuid.UUID) ([]domain.User, error) {
	var out []domain.User
	for _, id := range ids {
		if u, ok := f.users[id]; ok {
			out = append(out, u)
		}
	}
	return out, nil
}

func (f fakeStaff) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	if u, ok := f.users[id]; ok {
		return &u, nil
	}
	return nil, repository.ErrNotFound
}

type auditSink struct {
	repository.AuditRepository
	rows []domain.AuditLog
}

func (a *auditSink) Create(_ context.Context, l *domain.AuditLog) error {
	a.rows = append(a.rows, *l)
	return nil
}

var testT0 = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func newDeskForTest(t *testing.T) (*SupportService, *fakeSupportRepo, *clock.Fake, *auditSink) {
	t.Helper()
	sealer, err := security.NewSealer([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	clk := clock.NewFake(testT0)
	repo := newFakeSupportRepo()
	audit := &auditSink{}
	svc := NewSupportService(repo, sealer, nil, clk, "ip-secret").WithDesk(SupportDeskDeps{Tx: inlineTx{}, Users: fakeStaff{users: map[uuid.UUID]domain.User{}},
		Audit: audit, Cache: cache.NewMemory(clk.Now),
		Settings: SupportSettings{FrontendURL: "https://staryield.test", ConsoleURL: "https://admin.staryield.test",
			LinkTTL: 720 * time.Hour, Retention: 24 * 30 * time.Hour}})
	return svc, repo, clk, audit
}

func TestMaskEmail(t *testing.T) {
	for in, want := range map[string]string{
		"jane@gmail.com":              "j•••@g•••.com",
		"Jane.Doe@mail.example.co.uk": "J•••@m•••.uk",
		"x@y.io":                      "x•••@y•••.io",
		"élodie@société.fr":           "é•••@s•••.fr",
		"a@localhost":                 "a•••@l•••",
		"":                            "•••",
		"no-at-sign":                  "•••",
		"@nolocal.com":                "•••",
		"nodomain@":                   "•••",
		" spaced@Example.org ":        "s•••@E•••.org",
		"user@sub.domain.example.com": "u•••@s•••.com",
		"weird@.com":                  "w•••@.•••",
		"trailing@dot.":               "t•••@d•••",
	} {
		if got := MaskEmail(in); got != want {
			t.Errorf("MaskEmail(%q) = %q, want %q", in, got, want)
		}
	}
	// Whatever the input, the local part and the domain are never revealed.
	if got := MaskEmail("secret.person@private-company.com"); strings.Contains(got, "ecret") || strings.Contains(got, "rivate") {
		t.Fatalf("leaks: %q", got)
	}
}

func TestValidateSupportBody(t *testing.T) {
	for _, tc := range []struct {
		in string
		ok bool
	}{
		{"x", true}, {"  hello  ", true}, {"", false}, {"   \n\t ", false}, {strings.Repeat("a", 5000), true},
		{strings.Repeat("a", 5001), false}, {strings.Repeat("é", 5000), true}, {strings.Repeat("é", 5001), false},
		{"bell\x07\x00 only", true}, {"\x07\x00", false},
	} {
		got, err := validateSupportBody(tc.in)
		if (err == nil) != tc.ok {
			t.Errorf("validateSupportBody(%.20q): err=%v want ok=%v", tc.in, err, tc.ok)
		}
		if err != nil {
			ae, _ := apperror.As(err)
			if ae == nil || ae.Code != apperror.CodeValidation || deskFields(ae)["body"] != msgSupportReplyBody {
				t.Errorf("error shape: %v", err)
			}
		} else if got != strings.TrimSpace(got) {
			t.Errorf("not trimmed: %q", got)
		}
	}
}

func TestStatusTransitions(t *testing.T) {
	now := testT0
	all := []domain.SupportStatus{domain.SupportOpen, domain.SupportPending, domain.SupportResolved, domain.SupportClosed}
	for _, from := range all {
		for _, to := range all {
			f := statusFields(from, to, now)
			if from == to {
				if f != nil {
					t.Errorf("%s→%s should be a no-op: %v", from, to, f)
				}
				continue
			}
			if f["status"] != to {
				t.Errorf("%s→%s: status %v", from, to, f["status"])
			}
			switch to {
			case domain.SupportResolved:
				if f["resolved_at"] != now || f["closed_at"] != nil {
					t.Errorf("%s→resolved stamps: %v", from, f)
				}
			case domain.SupportClosed:
				if f["closed_at"] != now {
					t.Errorf("%s→closed must stamp closed_at (the retention clock): %v", from, f)
				}
				if _, touched := f["resolved_at"]; touched {
					t.Errorf("closing must keep resolved_at: %v", f)
				}
			default:
				if f["resolved_at"] != nil || f["closed_at"] != nil {
					t.Errorf("%s→%s must clear resolved_at and closed_at: %v", from, to, f)
				}
			}
		}
	}
	for st, want := range map[domain.SupportStatus]string{domain.SupportOpen: "support", domain.SupportPending: "you",
		domain.SupportResolved: "none", domain.SupportClosed: "none"} {
		if awaitingOf(st) != want {
			t.Errorf("awaiting(%s) = %s", st, awaitingOf(st))
		}
	}
	for _, s := range []string{"open", "pending", "resolved", "closed"} {
		if !domain.ValidSupportStatus(domain.SupportStatus(s)) {
			t.Errorf("%s should be valid", s)
		}
	}
	for _, s := range []string{"", "OPEN", "done", "open,pending", "closed "} {
		if domain.ValidSupportStatus(domain.SupportStatus(s)) {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestParseStatusList(t *testing.T) {
	got, err := parseStatusList(" Open, pending ,open,")
	if err != nil || len(got) != 2 || got[0] != domain.SupportOpen || got[1] != domain.SupportPending {
		t.Fatalf("list: %v %v", got, err)
	}
	if got, err := parseStatusList(""); err != nil || len(got) != 0 {
		t.Fatalf("empty: %v %v", got, err)
	}
	if _, err := parseStatusList("open,bogus"); err == nil {
		t.Fatal("unknown status must be rejected")
	}
	if !onlyOpenStatuses([]domain.SupportStatus{domain.SupportOpen}) || onlyOpenStatuses(nil) ||
		onlyOpenStatuses([]domain.SupportStatus{domain.SupportOpen, domain.SupportPending}) {
		t.Fatal("onlyOpenStatuses")
	}
}

func TestSupportQueryClassification(t *testing.T) {
	svc, _, _, _ := newDeskForTest(t)
	admin := uuid.New()
	ref, hash, words := svc.classifyQuery("sy-ab2cd3")
	if ref != "SY-AB2CD3" || hash != "" || words != "" {
		t.Fatalf("reference: %q %q %q", ref, hash, words)
	}
	ref, hash, words = svc.classifyQuery("Visitor@Example.com")
	want := security.Keyed("ip-secret", "support-email:visitor@example.com", 0)
	if ref != "" || hash != want || words != "" || len(hash) != 64 {
		t.Fatalf("email must go through the keyed hash of the normalised address: %q %q %q", ref, hash, words)
	}
	ref, hash, words = svc.classifyQuery("refund for double charge")
	if ref != "" || hash != "" || words != "refund for double charge" {
		t.Fatalf("words: %q %q %q", ref, hash, words)
	}
	// "SY-" followed by something that is not a reference is a subject search.
	if _, _, w := svc.classifyQuery("SY-12"); w != "SY-12" {
		t.Fatalf("partial reference: %q", w)
	}
	// An address that does not validate is searched as words, never hashed.
	if _, h, w := svc.classifyQuery("not an@email"); h != "" || w == "" {
		t.Fatalf("invalid address: %q %q", h, w)
	}

	f, err := svc.buildFilter(admin, SupportQuery{Status: "open,pending", Topic: "Billing", Priority: "HIGH", Assignee: "me",
		Sort: "waiting", Q: "  invoice  "})
	if err != nil || len(f.Statuses) != 2 || f.Topic != "billing" || f.Priority != "high" || f.AssigneeID == nil ||
		*f.AssigneeID != admin || f.Sort != "waiting" || f.Words != "invoice" {
		t.Fatalf("filter: %+v %v", f, err)
	}
	f, err = svc.buildFilter(admin, SupportQuery{Assignee: "unassigned"})
	if err != nil || !f.Unassigned || f.Sort != "newest" {
		t.Fatalf("unassigned/default sort: %+v %v", f, err)
	}
	other := uuid.New()
	if f, err = svc.buildFilter(admin, SupportQuery{Assignee: other.String(), UserID: other.String()}); err != nil ||
		*f.AssigneeID != other || *f.UserID != other {
		t.Fatalf("uuid filters: %+v %v", f, err)
	}
	// Every problem is reported at once, with translatable messages.
	_, err = svc.buildFilter(admin, SupportQuery{Status: "x", Topic: "y", Priority: "z", Assignee: "nobody", UserID: "nope",
		Sort: "random", Q: strings.Repeat("a", 101)})
	ae, _ := apperror.As(err)
	if ae == nil {
		t.Fatal("expected validation error")
	}
	fields := deskFields(ae)
	for _, k := range []string{"status", "topic", "priority", "assignee", "user_id", "sort", "q"} {
		if fields[k] == "" {
			t.Errorf("missing field error %q in %v", k, fields)
		}
	}
}

func TestCursorEncodeDecode(t *testing.T) {
	tk := &domain.SupportTicket{ID: uuid.New(), CreatedAt: testT0.Add(-time.Hour), LastCustomerMessageAt: testT0.Add(-2 * time.Hour), Status: domain.SupportOpen}
	// newest / oldest: (created_at, id)
	for _, sort := range []string{repository.SupportSortNewest, repository.SupportSortOldest} {
		raw := encodeSupportCursor(sort, tk, false)
		c, err := decodeSupportCursor(raw, sort, false)
		if err != nil || c == nil || !c.At.Equal(tk.CreatedAt) || c.ID != tk.ID || c.Rank != 0 {
			t.Fatalf("%s: %+v %v", sort, c, err)
		}
	}
	// waiting, only open tickets: (last_customer_message_at, id)
	raw := encodeSupportCursor(repository.SupportSortWaiting, tk, false)
	c, err := decodeSupportCursor(raw, repository.SupportSortWaiting, false)
	if err != nil || !c.At.Equal(tk.LastCustomerMessageAt) || c.ID != tk.ID {
		t.Fatalf("waiting: %+v %v", c, err)
	}
	// waiting across statuses carries the rank (open first)
	raw = encodeSupportCursor(repository.SupportSortWaiting, tk, true)
	if c, err = decodeSupportCursor(raw, repository.SupportSortWaiting, true); err != nil || c.Rank != 0 || c.ID != tk.ID {
		t.Fatalf("waiting rank 0: %+v %v", c, err)
	}
	pending := *tk
	pending.Status = domain.SupportPending
	raw = encodeSupportCursor(repository.SupportSortWaiting, &pending, true)
	if c, err = decodeSupportCursor(raw, repository.SupportSortWaiting, true); err != nil || c.Rank != 1 {
		t.Fatalf("waiting rank 1: %+v %v", c, err)
	}
	// Empty cursor = first page.
	if c, err := decodeSupportCursor("", repository.SupportSortNewest, false); err != nil || c != nil {
		t.Fatalf("empty: %v %v", c, err)
	}
	// Garbage, wrong shapes and cursors of another sort are rejected as validation errors.
	bad := []string{"%%%", "e30", repository.EncodeCursor(testT0, "not-a-uuid"), strings.Repeat("A", 300)}
	for _, raw := range bad {
		if _, err := decodeSupportCursor(raw, repository.SupportSortNewest, false); err == nil {
			t.Errorf("cursor %.20q accepted", raw)
		}
	}
	if _, err := decodeSupportCursor(repository.EncodeCursor(testT0, uuid.NewString()), repository.SupportSortWaiting, true); err == nil {
		t.Error("a plain cursor must not be accepted by the ranked waiting sort")
	}
	if _, err := decodeSupportCursor(repository.EncodeCursor(testT0, "2|"+uuid.NewString()), repository.SupportSortWaiting, true); err == nil {
		t.Error("rank must be 0 or 1")
	}
	ae, _ := apperror.As(func() error { _, err := decodeSupportCursor("!!", "newest", false); return err }())
	if ae == nil || ae.Code != apperror.CodeValidation {
		t.Errorf("error code: %v", ae)
	}
}

func TestParseTicketKey(t *testing.T) {
	id := uuid.New()
	if got, ref, err := parseTicketKey(" " + id.String() + " "); err != nil || got != id || ref != "" {
		t.Fatalf("uuid: %v %q %v", got, ref, err)
	}
	if got, ref, err := parseTicketKey("sy-ab2cd3"); err != nil || got != uuid.Nil || ref != "SY-AB2CD3" {
		t.Fatalf("reference: %v %q %v", got, ref, err)
	}
	for _, bad := range []string{"", "SY-", "SY-OOOOOO", "SY-AB2CD34", "1; DROP TABLE support_tickets", "../etc"} {
		if _, _, err := parseTicketKey(bad); !errors.Is(err, apperror.ErrNotFound) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func TestStaffDisplayName(t *testing.T) {
	for in, want := range map[string]string{
		"Maria Lopez":         "Staryield Support (Maria)",
		"  maria  ":           "Staryield Support (maria)",
		"":                    "Staryield Support",
		"admin@staryield.net": "Staryield Support",
		"Jean-Pierre Martin":  "Staryield Support (Jean-Pierre)",
	} {
		if got := staffDisplay(in); got != want {
			t.Errorf("staffDisplay(%q) = %q, want %q", in, got, want)
		}
	}
	if got := staffDisplay(strings.Repeat("x", 200) + " Y"); utf8Len(got) > len(supportBrand)+35 {
		t.Errorf("first name not bounded: %d", utf8Len(got))
	}
}

func utf8Len(s string) int { return len([]rune(s)) }

// --- private-link token ------------------------------------------------------

func TestSupportTokenIssueVerifyExpiry(t *testing.T) {
	svc, _, clk, _ := newDeskForTest(t)
	tk := &domain.SupportTicket{ID: uuid.New()}
	token, err := svc.issueToken(tk, clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !supportTokenRE.MatchString(token) {
		t.Fatalf("token shape (32 random bytes, base64url): %q", token)
	}
	// Only a keyed hash and a sealed copy are stored, never the token.
	if tk.AccessTokenHash == nil || len(*tk.AccessTokenHash) != 64 || *tk.AccessTokenHash == token ||
		strings.Contains(*tk.AccessTokenHash, token) || strings.Contains(*tk.AccessTokenEnc, token) {
		t.Fatalf("stored token material: %v / %v", tk.AccessTokenHash, tk.AccessTokenEnc)
	}
	if want := security.Keyed("ip-secret", "support-token:"+token, 0); *tk.AccessTokenHash != want {
		t.Fatal("the stored hash must be the keyed hash (not a plain SHA-256)")
	}
	if *tk.AccessTokenHash == security.HashToken(token) {
		t.Fatal("hash must be keyed")
	}
	if plain, err := svc.sealer.Open(*tk.AccessTokenEnc, supportTokenAD(tk.ID)); err != nil || plain != token {
		t.Fatalf("sealed copy: %q %v", plain, err)
	}
	if _, err := svc.sealer.Open(*tk.AccessTokenEnc, supportTokenAD(uuid.New())); err == nil {
		t.Fatal("sealed copy must be bound to its ticket")
	}
	if !svc.tokenOK(tk, token) {
		t.Fatal("valid token rejected")
	}
	// Wrong, near-miss, malformed, empty, other ticket's token.
	other := &domain.SupportTicket{ID: uuid.New()}
	otherTok, _ := svc.issueToken(other, clk.Now())
	flip := token[:len(token)-1] + map[bool]string{true: "A", false: "B"}[token[len(token)-1] != 'A']
	for _, bad := range []string{"", "short", token + "x", flip, strings.ToUpper(token), otherTok, strings.Repeat("A", 43), token[:42] + "!"} {
		if svc.tokenOK(tk, bad) {
			t.Errorf("token %q accepted", bad)
		}
	}
	// No ticket at all: same answer (and the comparison still runs).
	if svc.verifyToken(nil, token, clk.Now()) {
		t.Fatal("unknown ticket accepted")
	}
	// A ticket without a token never matches, not even the all-zero hash.
	if svc.verifyToken(&domain.SupportTicket{ID: uuid.New()}, strings.Repeat("A", 43), clk.Now()) {
		t.Fatal("ticket without a token accepted")
	}
	// Expiry: valid until the TTL, rejected at and after it.
	clk.Advance(720*time.Hour - time.Second)
	if !svc.tokenOK(tk, token) {
		t.Fatal("token expired too early")
	}
	clk.Advance(time.Second)
	if svc.tokenOK(tk, token) {
		t.Fatal("expired token accepted")
	}
	// Tokens are unique.
	seen := map[string]bool{token: true}
	for i := 0; i < 200; i++ {
		x := &domain.SupportTicket{ID: uuid.New()}
		tok, _ := svc.issueToken(x, clk.Now())
		if seen[tok] {
			t.Fatal("token repeated")
		}
		seen[tok] = true
	}
}

// tokenOK verifies at the fake clock's time.
func (s *SupportService) tokenOK(t *domain.SupportTicket, token string) bool {
	return s.verifyToken(t, token, s.clock.Now())
}

func TestSupportLinkIsExtendedNotRotated(t *testing.T) {
	svc, repo, clk, _ := newDeskForTest(t)
	tk := &domain.SupportTicket{ID: uuid.New(), Reference: "SY-AB2CD3", Locale: "fr"}
	token, _ := svc.issueToken(tk, clk.Now())
	repo.tickets[tk.ID] = tk
	firstExp := *tk.AccessTokenExpiresAt

	clk.Advance(10 * 24 * time.Hour)
	link := svc.linkFor(context.Background(), tk)
	wantPrefix := "https://staryield.test/fr/support/SY-AB2CD3#t="
	if !strings.HasPrefix(link, wantPrefix) || strings.TrimPrefix(link, wantPrefix) != token {
		t.Fatalf("the same token must be re-sent: %q (token %q)", link, token)
	}
	if !tk.AccessTokenExpiresAt.After(firstExp) || !tk.AccessTokenExpiresAt.Equal(clk.Now().Add(720*time.Hour)) {
		t.Fatalf("validity not extended: %v → %v", firstExp, tk.AccessTokenExpiresAt)
	}
	if !svc.tokenOK(tk, token) {
		t.Fatal("the original token must still work after extension")
	}
	// Once expired, a new token is issued and the old one dies.
	clk.Advance(721 * time.Hour)
	link2 := svc.linkFor(context.Background(), tk)
	fresh := strings.TrimPrefix(link2, wantPrefix)
	if fresh == token || !supportTokenRE.MatchString(fresh) {
		t.Fatalf("expired link must be replaced: %q", link2)
	}
	if !svc.tokenOK(tk, fresh) || svc.tokenOK(tk, token) {
		t.Fatal("new token must work and the old one must not")
	}
	// The fragment (not the query string) carries the token.
	if strings.Contains(strings.SplitN(link2, "#", 2)[0], fresh) {
		t.Fatal("token must only be in the URL fragment")
	}
}

// --- visibility ----------------------------------------------------------------

func TestCustomerViewNeverShowsInternalNotesOrSystemEntries(t *testing.T) {
	svc, repo, clk, _ := newDeskForTest(t)
	staff := domain.User{ID: uuid.New(), DisplayName: "Maria Lopez", Email: "maria@staryield.net"}
	svc.desk.Users = fakeStaff{users: map[uuid.UUID]domain.User{staff.ID: staff}}
	tk := &domain.SupportTicket{ID: uuid.New(), Reference: "SY-AB2CD3", Name: "Visitor", Subject: "Help", Topic: domain.SupportGeneral,
		Status: domain.SupportPending, CreatedAt: testT0, LastMessageAt: testT0}
	tk.MessageEnc, _ = svc.sealer.Seal("original request", supportAD(tk.ID, "message"))
	repo.tickets[tk.ID] = tk
	add := func(author domain.SupportAuthor, vis domain.SupportVisibility, body string, by *uuid.UUID) {
		id := domain.NewID()
		sealed, _ := svc.sealer.Seal(body, supportMessageAD(id))
		repo.messages = append(repo.messages, domain.SupportMessage{ID: id, TicketID: tk.ID, AuthorType: author, AuthorID: by,
			Visibility: vis, BodyEnc: sealed, CreatedAt: clk.Now()})
		clk.Advance(time.Minute)
	}
	add(domain.SupportAuthorStaff, domain.SupportInternal, "INTERNAL: customer seems to be a fraudster", &staff.ID)
	add(domain.SupportAuthorStaff, domain.SupportPublic, "Hello, we are looking into it", &staff.ID)
	add(domain.SupportAuthorSystem, domain.SupportInternal, "SYSTEM: closed automatically", nil)
	add(domain.SupportAuthorCustomer, domain.SupportPublic, "Thanks!", nil)
	// A malformed row that is not "internal" but is a system entry must not leak either.
	add(domain.SupportAuthorSystem, domain.SupportPublic, "SYSTEM-PUBLIC leak attempt", nil)

	view, err := svc.customerView(context.Background(), tk)
	if err != nil {
		t.Fatal(err)
	}
	var bodies []string
	for _, m := range view.Messages {
		bodies = append(bodies, m.Body)
		if m.Author.Type != "customer" && m.Author.Type != "staff" {
			t.Errorf("author type %q must never reach a requester", m.Author.Type)
		}
	}
	if strings.Join(bodies, "|") != "original request|Hello, we are looking into it|Thanks!" {
		t.Fatalf("thread: %v", bodies)
	}
	for _, m := range view.Messages {
		if m.Author.Type == "staff" && m.Author.Name != "Staryield Support (Maria)" {
			t.Errorf("staff name: %q", m.Author.Name)
		}
		if strings.Contains(m.Author.Name, "Lopez") || strings.Contains(m.Author.Name, "@") {
			t.Errorf("staff identity leaked: %q", m.Author.Name)
		}
	}
	if view.Awaiting != "you" || !view.CanReply {
		t.Fatalf("view: %+v", view.CustomerTicketRow)
	}
	tk.Status = domain.SupportClosed
	if view, _ = svc.customerView(context.Background(), tk); view.CanReply || view.Awaiting != "none" {
		t.Fatalf("closed view: %+v", view)
	}
}

// --- audit throttle ----------------------------------------------------------

func TestViewedAuditIsThrottled(t *testing.T) {
	svc, _, clk, audit := newDeskForTest(t)
	adminA, adminB := uuid.New(), uuid.New()
	t1 := &domain.SupportTicket{ID: uuid.New(), Reference: "SY-AB2CD3"}
	t2 := &domain.SupportTicket{ID: uuid.New(), Reference: "SY-EF4GH5"}
	ctx := context.Background()

	svc.auditViewed(ctx, adminA, t1)
	svc.auditViewed(ctx, adminA, t1)
	clk.Advance(9*time.Minute + 59*time.Second)
	svc.auditViewed(ctx, adminA, t1)
	if len(audit.rows) != 1 || audit.rows[0].Action != "support_ticket.viewed" || audit.rows[0].TargetID != t1.ID.String() {
		t.Fatalf("one audit row expected within 10 minutes: %+v", audit.rows)
	}
	// Per administrator and per ticket.
	svc.auditViewed(ctx, adminB, t1)
	svc.auditViewed(ctx, adminA, t2)
	if len(audit.rows) != 3 {
		t.Fatalf("other admin / ticket must be audited: %d rows", len(audit.rows))
	}
	// After the window the next view is audited again.
	clk.Advance(2 * time.Second)
	svc.auditViewed(ctx, adminA, t1)
	if len(audit.rows) != 4 {
		t.Fatalf("after 10 minutes: %d rows", len(audit.rows))
	}
	if audit.rows[0].Metadata["reference"] != "SY-AB2CD3" {
		t.Fatalf("metadata: %v", audit.rows[0].Metadata)
	}
	// Without a cache every view is audited (more audit, never less).
	svc.desk.Cache = nil
	before := len(audit.rows)
	svc.auditViewed(ctx, adminA, t1)
	svc.auditViewed(ctx, adminA, t1)
	if len(audit.rows) != before+2 {
		t.Fatal("no cache: every view must be audited")
	}
}

// --- maintenance ---------------------------------------------------------------

func TestMaintenanceAutoClosesResolvedTicketsAfterSevenDays(t *testing.T) {
	svc, repo, clk, _ := newDeskForTest(t)
	mk := func(status domain.SupportStatus, resolvedAgo time.Duration) *domain.SupportTicket {
		tk := &domain.SupportTicket{ID: uuid.New(), Status: status, Version: 1, MessageCount: 1, EmailMasked: "a•••@b•••.com"}
		if status == domain.SupportResolved {
			at := clk.Now().Add(-resolvedAgo)
			tk.ResolvedAt = &at
		}
		repo.tickets[tk.ID] = tk
		return tk
	}
	old := mk(domain.SupportResolved, 7*24*time.Hour+time.Minute)
	edge := mk(domain.SupportResolved, 7*24*time.Hour-time.Minute)
	pending := mk(domain.SupportPending, 30*24*time.Hour)
	open := mk(domain.SupportOpen, 0)
	full := mk(domain.SupportResolved, 8*24*time.Hour)
	full.MessageCount = domain.MaxSupportMessages

	rep, err := svc.Maintain(context.Background(), clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	if rep.AutoClosed != 2 {
		t.Fatalf("auto-closed %d, want 2", rep.AutoClosed)
	}
	if old.Status != domain.SupportClosed || old.ClosedAt == nil || !old.ClosedAt.Equal(clk.Now()) || old.Version != 2 {
		t.Fatalf("old ticket: %+v", old)
	}
	if edge.Status != domain.SupportResolved || pending.Status != domain.SupportPending || open.Status != domain.SupportOpen {
		t.Fatal("only resolved tickets older than 7 days may be closed")
	}
	// A system entry records why; a full thread is closed without one.
	sys := 0
	for _, m := range repo.messages {
		if m.AuthorType != domain.SupportAuthorSystem || m.Visibility != domain.SupportInternal || m.TicketID != old.ID {
			t.Errorf("unexpected message: %+v", m)
		}
		if body, err := svc.sealer.Open(m.BodyEnc, supportMessageAD(m.ID)); err != nil || body != msgSupportAutoClose {
			t.Errorf("system body: %q %v", body, err)
		}
		sys++
	}
	if sys != 1 || old.MessageCount != 2 || full.MessageCount != domain.MaxSupportMessages || full.Status != domain.SupportClosed {
		t.Fatalf("system entries: %d, counts %d/%d, full status %s", sys, old.MessageCount, full.MessageCount, full.Status)
	}
	// Running again changes nothing (idempotent).
	if rep, _ = svc.Maintain(context.Background(), clk.Now()); rep.AutoClosed != 0 {
		t.Fatalf("second run closed %d", rep.AutoClosed)
	}
	// The edge ticket closes once a minute more has passed.
	clk.Advance(2 * time.Minute)
	if rep, _ = svc.Maintain(context.Background(), clk.Now()); rep.AutoClosed != 1 || edge.Status != domain.SupportClosed {
		t.Fatalf("edge: %+v %s", rep, edge.Status)
	}
}

func TestMaintenanceRetentionDeletesOldClosedTickets(t *testing.T) {
	svc, repo, clk, _ := newDeskForTest(t) // retention: 30 days
	mk := func(closedAgo time.Duration) *domain.SupportTicket {
		at := clk.Now().Add(-closedAgo)
		tk := &domain.SupportTicket{ID: uuid.New(), Status: domain.SupportClosed, ClosedAt: &at, EmailMasked: "x"}
		repo.tickets[tk.ID] = tk
		return tk
	}
	gone := mk(31 * 24 * time.Hour)
	kept := mk(29 * 24 * time.Hour)
	open := &domain.SupportTicket{ID: uuid.New(), Status: domain.SupportOpen, EmailMasked: "x"}
	repo.tickets[open.ID] = open
	rep, err := svc.Maintain(context.Background(), clk.Now())
	if err != nil || rep.Deleted != 1 {
		t.Fatalf("report %+v err %v", rep, err)
	}
	if _, ok := repo.tickets[gone.ID]; ok {
		t.Fatal("ticket past retention must be deleted")
	}
	if _, ok := repo.tickets[kept.ID]; !ok {
		t.Fatal("ticket within retention must stay")
	}
	if _, ok := repo.tickets[open.ID]; !ok {
		t.Fatal("open ticket must stay")
	}
	if want := clk.Now().Add(-30 * 24 * time.Hour); !repo.deletedBefore[0].Equal(want) {
		t.Fatalf("retention cut-off %v, want %v", repo.deletedBefore[0], want)
	}
	// A moved clock moves the cut-off.
	clk.Advance(2 * 24 * time.Hour)
	if rep, _ = svc.Maintain(context.Background(), clk.Now()); rep.Deleted != 1 {
		t.Fatalf("after 2 more days: %+v", rep)
	}
}

func TestMaintenanceBackfillsMaskedAddresses(t *testing.T) {
	svc, repo, clk, _ := newDeskForTest(t)
	old := &domain.SupportTicket{ID: uuid.New(), Status: domain.SupportOpen}
	old.EmailEnc, _ = svc.sealer.Seal("jane@gmail.com", supportAD(old.ID, "email"))
	broken := &domain.SupportTicket{ID: uuid.New(), Status: domain.SupportOpen, EmailEnc: "garbage"}
	repo.tickets[old.ID], repo.tickets[broken.ID] = old, broken
	rep, err := svc.Maintain(context.Background(), clk.Now())
	if err != nil || rep.Masked != 2 {
		t.Fatalf("%+v %v", rep, err)
	}
	if old.EmailMasked != "j•••@g•••.com" || broken.EmailMasked != "•••" {
		t.Fatalf("masks: %q %q", old.EmailMasked, broken.EmailMasked)
	}
	if rep, _ = svc.Maintain(context.Background(), clk.Now()); rep.Masked != 0 {
		t.Fatal("backfill must not repeat")
	}
}

func TestSummaryCacheGeneration(t *testing.T) {
	svc, _, _, _ := newDeskForTest(t)
	ctx := context.Background()
	svc.invalidateSummary(ctx)
	g1, _, _ := svc.desk.Cache.Get(ctx, supportGenKey)
	svc.invalidateSummary(ctx)
	g2, _, _ := svc.desk.Cache.Get(ctx, supportGenKey)
	if g1 == "" || g1 == g2 {
		t.Fatalf("every write must move the generation: %q → %q", g1, g2)
	}
}

func deskFields(e *apperror.Error) map[string]string {
	f, _ := e.Details["fields"].(map[string]any)
	out := map[string]string{}
	for k, v := range f {
		out[k], _ = v.(string)
	}
	return out
}
