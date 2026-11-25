package service

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// The verification flow's token queries on the in-memory store.

func (r memActions) FindOpen(_ context.Context, hash string, p domain.ActionPurpose, now time.Time) (*domain.ActionToken, error) {
	for _, t := range r.db.actions {
		if t.TokenHash == hash && t.Purpose == p && t.UsedAt == nil && t.ExpiresAt.After(now) {
			return &t, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r memActions) Use(_ context.Context, id uuid.UUID, now time.Time) error {
	t, ok := r.db.actions[id]
	if !ok || t.UsedAt != nil || !t.ExpiresAt.After(now) {
		return repository.ErrNotFound
	}
	t.UsedAt = &now
	r.db.actions[id] = t
	return nil
}

func (r memActions) IssuedSince(_ context.Context, userID uuid.UUID, p domain.ActionPurpose, since time.Time, limit int) ([]time.Time, error) {
	var out []time.Time
	for _, t := range r.db.actions {
		if t.UserID == userID && t.Purpose == p && t.CreatedAt.After(since) {
			out = append(out, t.CreatedAt)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].After(out[j]) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// memSuppressions is the outbox's suppression list (addresses as their own hash).
type memSuppressions map[string]bool

func (m memSuppressions) AddressHash(email string) string { return strings.ToLower(email) }

func (m memSuppressions) Suppressed(_ context.Context, hash string) (bool, error) {
	return m[hash], nil
}

// lockLog records the order in which the flow locks users and writes tokens.
type lockLog struct{ calls []string }

type loggedUsers struct {
	memUsers
	log *lockLog
}

func (r loggedUsers) GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	r.log.calls = append(r.log.calls, "lock user")
	return r.memUsers.GetForUpdate(ctx, id)
}

type loggedActions struct {
	memActions
	log *lockLog
	// failInvalidate makes InvalidateOpen fail (a broken transaction).
	failInvalidate bool
}

func (r loggedActions) Use(ctx context.Context, id uuid.UUID, now time.Time) error {
	r.log.calls = append(r.log.calls, "write token")
	return r.memActions.Use(ctx, id, now)
}

func (r loggedActions) Consume(ctx context.Context, hash string, p domain.ActionPurpose, now time.Time) (*domain.ActionToken, error) {
	r.log.calls = append(r.log.calls, "write token")
	return r.memActions.Consume(ctx, hash, p, now)
}

func (r loggedActions) InvalidateOpen(ctx context.Context, userID uuid.UUID, p domain.ActionPurpose, at time.Time) error {
	r.log.calls = append(r.log.calls, "write token")
	if r.failInvalidate {
		return errors.New("connection reset")
	}
	return r.memActions.InvalidateOpen(ctx, userID, p, at)
}

func (r loggedActions) Create(ctx context.Context, t *domain.ActionToken) error {
	r.log.calls = append(r.log.calls, "write token")
	return r.memActions.Create(ctx, t)
}

type verifyFixture struct {
	db         *memDB
	clock      *clock.Fake
	mail       *memMail
	events     *eventLog
	notes      *recordingNotifier
	suppressed memSuppressions
	locks      *lockLog
	actions    *loggedActions
	svc        *EmailVerificationService
	user       domain.User
	hooked     []uuid.UUID
}

func newVerifyFixture(t *testing.T) *verifyFixture {
	t.Helper()
	db := newMemDB()
	clk := clock.NewFake(time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC))
	sealer, err := security.NewSealer(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	locks := &lockLog{}
	f := &verifyFixture{db: db, clock: clk, mail: &memMail{}, events: &eventLog{}, notes: &recordingNotifier{}, suppressed: memSuppressions{},
		locks: locks, actions: &loggedActions{memActions: memActions{db: db}, log: locks}}
	f.svc = NewEmailVerificationService(EmailVerificationDeps{Tx: memTx{db: db}, Users: loggedUsers{memUsers: memUsers{db: db}, log: locks},
		Actions: f.actions, Mail: f.mail, Suppressions: f.suppressed, Sealer: sealer, Events: f.events, Notifier: f.notes, Audit: memAudit{db: db},
		Clock: clk, Settings: EmailVerificationSettings{FrontendURL: "https://staryield.test", IPHashSecret: "secret"}})
	f.svc.OnVerified(func(_ context.Context, id uuid.UUID) { f.hooked = append(f.hooked, id) })
	f.user = f.addUser("moon@example.com", domain.RoleClient)
	return f
}

func (f *verifyFixture) addUser(email string, role domain.Role) domain.User {
	now := f.clock.Now()
	u := domain.User{ID: domain.NewID(), Email: email, Role: role, Status: domain.UserActive, DisplayName: "Moon", Locale: "fr",
		TermsAcceptedAt: now, CreatedAt: now, UpdatedAt: now}
	f.db.users[u.ID] = u
	return u
}

// rawToken extracts the token from the fragment of the e-mailed link.
func rawToken(t *testing.T, r EmailRequest) string {
	t.Helper()
	if r.Content.Action == nil {
		t.Fatal("no call to action")
	}
	_, tok, ok := strings.Cut(r.Content.Action.URL, "#token=")
	if !ok || tok == "" {
		t.Fatalf("link without a fragment token: %s", r.Content.Action.URL)
	}
	return tok
}

func (f *verifyFixture) lastToken(t *testing.T) string {
	t.Helper()
	sent := f.mail.byTemplate("email_verify")
	if len(sent) == 0 {
		t.Fatal("no verification e-mail was queued")
	}
	return rawToken(t, sent[len(sent)-1])
}

func wantCode(t *testing.T, err error, code apperror.Code) *apperror.Error {
	t.Helper()
	e, ok := apperror.As(err)
	if !ok || e.Code != code {
		t.Fatalf("want error %s, got %v", code, err)
	}
	return e
}

func TestRegistrationSendsTheLinkInTheFragment(t *testing.T) {
	f := newVerifyFixture(t)
	f.svc.OnRegistered(context.Background(), &f.user)
	sent := f.mail.byTemplate("email_verify")
	if len(sent) != 1 {
		t.Fatalf("want one e-mail, got %d", len(sent))
	}
	r := sent[0]
	if r.Category != domain.EmailSecurity || r.To != f.user.Email || r.Locale != "fr" || r.UserID == nil || *r.UserID != f.user.ID {
		t.Fatalf("unexpected request: %+v", r)
	}
	url := r.Content.Action.URL
	if !strings.HasPrefix(url, "https://staryield.test/fr/verify-email#token=") || strings.Contains(url, "?") {
		t.Fatalf("the token must travel in the fragment only: %s", url)
	}
	open := memActions{db: f.db}.open(f.user.ID, domain.ActionEmailVerify)
	if len(open) != 1 || r.DedupeKey != "email_verify:"+open[0].ID.String() {
		t.Fatalf("dedupe key %q does not name token %v", r.DedupeKey, open)
	}
	raw := rawToken(t, r)
	if open[0].TokenHash == raw || open[0].TokenHash != security.HashToken(raw) {
		t.Fatal("only the hash of the token may be stored")
	}
	if strings.Contains(open[0].PayloadEnc, f.user.Email) {
		t.Fatal("the payload must be sealed")
	}
	if open[0].ExpiresAt.Sub(open[0].CreatedAt) != 48*time.Hour {
		t.Fatalf("the link lives %v", open[0].ExpiresAt.Sub(open[0].CreatedAt))
	}
}

func TestRegistrationSkipsVerifiedAndNonClientAccounts(t *testing.T) {
	f := newVerifyFixture(t)
	now := f.clock.Now()
	verified := f.user
	verified.EmailVerifiedAt = &now
	f.svc.OnRegistered(context.Background(), &verified)
	advisor := f.addUser("seer@example.com", domain.RolePsychic)
	f.svc.OnRegistered(context.Background(), &advisor)
	f.svc.OnRegistered(context.Background(), nil)
	if n := len(f.mail.sent); n != 0 {
		t.Fatalf("no e-mail expected, got %d", n)
	}
}

func TestVerifyMarksTheAddressAndAnnouncesIt(t *testing.T) {
	f := newVerifyFixture(t)
	ctx := context.Background()
	f.svc.OnRegistered(ctx, &f.user)
	raw := f.lastToken(t)

	res, err := f.svc.Verify(ctx, raw)
	if err != nil || res.Status != VerificationVerified || !res.EmailVerified {
		t.Fatalf("verify: %+v %v", res, err)
	}
	if u := f.db.users[f.user.ID]; u.EmailVerifiedAt == nil || !u.EmailVerifiedAt.Equal(f.clock.Now()) {
		t.Fatalf("the address was not stamped: %+v", u.EmailVerifiedAt)
	}
	ev := f.events.of(realtime.EventEmailVerified, f.user.ID)
	if len(ev) != 1 || ev[0].data["email_verified"] != true || ev[0].data["email_verified_at"] == nil {
		t.Fatalf("realtime event: %+v", f.events.events)
	}
	if len(f.notes.spec) != 1 || f.notes.spec[0].Kind != NotifyAccountEmailVerified || f.notes.sent[f.user.ID] != 1 {
		t.Fatalf("notification: %+v", f.notes.spec)
	}
	if len(f.hooked) != 1 || f.hooked[0] != f.user.ID {
		t.Fatalf("OnVerified hooks: %v", f.hooked)
	}
	var audited bool
	for _, a := range f.db.audit {
		if a.Action == "account.email_verified" && a.TargetID == f.user.ID.String() {
			audited = true
			if a.ActorID == nil || *a.ActorID != f.user.ID {
				t.Fatalf("the user is the actor: %+v", a.ActorID)
			}
		}
	}
	if !audited {
		t.Fatal("verification not audited")
	}
}

func TestATokenWorksOnlyOnce(t *testing.T) {
	f := newVerifyFixture(t)
	ctx := context.Background()
	f.svc.OnRegistered(ctx, &f.user)
	raw := f.lastToken(t)
	if _, err := f.svc.Verify(ctx, raw); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.Verify(ctx, raw)
	wantCode(t, err, apperror.CodeInvalidToken)
	if n := len(f.events.events); n != 1 {
		t.Fatalf("the replay must not announce anything: %d events", n)
	}
}

func TestUnknownMalformedAndExpiredLinksAnswerTheSame(t *testing.T) {
	f := newVerifyFixture(t)
	ctx := context.Background()
	f.svc.OnRegistered(ctx, &f.user)
	raw := f.lastToken(t)
	f.clock.Advance(48*time.Hour + time.Second)
	var msgs []string
	for _, tok := range []string{"", "nope", strings.Repeat("a", 500), raw} {
		_, err := f.svc.Verify(ctx, tok)
		msgs = append(msgs, wantCode(t, err, apperror.CodeInvalidToken).Message)
	}
	for _, m := range msgs[1:] {
		if m != msgs[0] {
			t.Fatalf("answers differ: %q", msgs)
		}
	}
	if f.db.users[f.user.ID].EmailVerifiedAt != nil || len(f.events.events) != 0 {
		t.Fatal("a failed verification changed something")
	}
}

func TestALinkForAnOldAddressIsInvalidAndBurnt(t *testing.T) {
	f := newVerifyFixture(t)
	ctx := context.Background()
	f.svc.OnRegistered(ctx, &f.user)
	raw := f.lastToken(t)
	u := f.db.users[f.user.ID]
	u.Email = "new@example.com"
	f.db.users[u.ID] = u

	_, err := f.svc.Verify(ctx, raw)
	wantCode(t, err, apperror.CodeInvalidToken)
	if f.db.users[f.user.ID].EmailVerifiedAt != nil || len(f.events.events) != 0 {
		t.Fatal("the new address must not be verified by a link for the old one")
	}
	if n := len(memActions{db: f.db}.open(f.user.ID, domain.ActionEmailVerify)); n != 0 {
		t.Fatalf("the unusable link must be burnt, %d still open", n)
	}
}

func TestALinkIsBoundToItsTokenRow(t *testing.T) {
	f := newVerifyFixture(t)
	ctx := context.Background()
	f.svc.OnRegistered(ctx, &f.user)
	other := f.addUser("sun@example.com", domain.RoleClient)
	f.svc.OnRegistered(ctx, &other)
	// Move the payload of one token onto the other's row: the associated data differs.
	var a, b domain.ActionToken
	for _, t := range f.db.actions {
		if t.UserID == f.user.ID {
			a = t
		} else {
			b = t
		}
	}
	a.PayloadEnc = b.PayloadEnc
	f.db.actions[a.ID] = a
	_, err := f.svc.Verify(ctx, f.mail.sent[0].Content.Action.URL[strings.Index(f.mail.sent[0].Content.Action.URL, "#token=")+7:])
	wantCode(t, err, apperror.CodeInvalidToken)
}

func TestVerifyRefusesInactiveAccounts(t *testing.T) {
	f := newVerifyFixture(t)
	ctx := context.Background()
	f.svc.OnRegistered(ctx, &f.user)
	raw := f.lastToken(t)
	u := f.db.users[f.user.ID]
	u.Status = domain.UserDeactivated
	f.db.users[u.ID] = u
	_, err := f.svc.Verify(ctx, raw)
	wantCode(t, err, apperror.CodeInvalidToken)
}

func TestVerifyingAnAlreadyVerifiedAddressChangesNothing(t *testing.T) {
	f := newVerifyFixture(t)
	ctx := context.Background()
	f.svc.OnRegistered(ctx, &f.user)
	raw := f.lastToken(t)
	at := f.clock.Now().Add(-time.Hour)
	u := f.db.users[f.user.ID]
	u.EmailVerifiedAt = &at
	f.db.users[u.ID] = u

	res, err := f.svc.Verify(ctx, raw)
	if err != nil || res.Status != VerificationAlreadyVerified || !res.EmailVerified {
		t.Fatalf("verify: %+v %v", res, err)
	}
	if got := f.db.users[f.user.ID].EmailVerifiedAt; !got.Equal(at) {
		t.Fatalf("the original date must stay: %v", got)
	}
	if len(f.events.events) != 0 || len(f.notes.spec) != 0 || len(f.hooked) != 0 {
		t.Fatal("nothing may be announced twice")
	}
}

func TestResendHonoursTheCooldownAndReplacesTheOldLink(t *testing.T) {
	f := newVerifyFixture(t)
	ctx := context.Background()
	f.svc.OnRegistered(ctx, &f.user)
	first := f.lastToken(t)

	_, err := f.svc.Resend(ctx, f.user.ID)
	e := wantCode(t, err, apperror.CodeRateLimited)
	if e.RetryAfter <= 0 || e.RetryAfter > time.Minute {
		t.Fatalf("Retry-After %v", e.RetryAfter)
	}
	if len(f.mail.sent) != 1 {
		t.Fatal("no e-mail inside the cooldown")
	}

	f.clock.Advance(61 * time.Second)
	sent, err := f.svc.Resend(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sent.Status != "sent" || sent.Email != "m***@example.com" || !sent.ResendAvailableAt.Equal(f.clock.Now().Add(time.Minute)) {
		t.Fatalf("answer: %+v", sent)
	}
	if n := len(memActions{db: f.db}.open(f.user.ID, domain.ActionEmailVerify)); n != 1 {
		t.Fatalf("exactly one open link expected, got %d", n)
	}
	_, err = f.svc.Verify(ctx, first)
	wantCode(t, err, apperror.CodeInvalidToken)
	if _, err := f.svc.Verify(ctx, f.lastToken(t)); err != nil {
		t.Fatalf("the newest link must work: %v", err)
	}
}

func TestResendAfterVerificationIsAConflict(t *testing.T) {
	f := newVerifyFixture(t)
	ctx := context.Background()
	f.svc.OnRegistered(ctx, &f.user)
	if _, err := f.svc.Verify(ctx, f.lastToken(t)); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.Resend(ctx, f.user.ID)
	wantCode(t, err, apperror.CodeAlreadyVerified)
	_, err = f.svc.AdminResend(ctx, f.user.ID)
	wantCode(t, err, apperror.CodeAlreadyVerified)
	_, err = f.svc.Resend(ctx, uuid.New())
	wantCode(t, err, apperror.CodeNotFound)
}

func TestAnyRoleWithAnUnverifiedAddressMayResend(t *testing.T) {
	f := newVerifyFixture(t)
	for _, role := range []domain.Role{domain.RolePsychic, domain.RoleAdmin} {
		u := f.addUser(string(role)+"@example.com", role)
		if _, err := f.svc.Resend(context.Background(), u.ID); err != nil {
			t.Fatalf("%s: %v", role, err)
		}
	}
	if len(f.mail.sent) != 2 {
		t.Fatalf("e-mails: %d", len(f.mail.sent))
	}
}

func TestReminderIsSilentWhenThereIsNothingToSend(t *testing.T) {
	f := newVerifyFixture(t)
	ctx := context.Background()
	if err := f.svc.SendReminder(ctx, f.user.ID); err != nil || len(f.mail.sent) != 1 {
		t.Fatalf("first reminder: %v, %d e-mails", err, len(f.mail.sent))
	}
	if err := f.svc.SendReminder(ctx, f.user.ID); err != nil || len(f.mail.sent) != 1 {
		t.Fatalf("a reminder inside the cooldown must be silent: %v, %d e-mails", err, len(f.mail.sent))
	}
	f.clock.Advance(2 * time.Minute)
	if _, err := f.svc.Verify(ctx, f.lastToken(t)); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SendReminder(ctx, f.user.ID); err != nil || len(f.mail.sent) != 1 {
		t.Fatalf("a verified address gets no reminder: %v", err)
	}
	if err := f.svc.SendReminder(ctx, uuid.New()); err == nil {
		t.Fatal("an unknown user is an error, not a silent skip")
	}
}

func TestAdminResendIsAuditedAndNeedsAnActiveAccount(t *testing.T) {
	f := newVerifyFixture(t)
	ctx := context.Background()
	if _, err := f.svc.AdminResend(ctx, f.user.ID); err != nil {
		t.Fatal(err)
	}
	var audited bool
	for _, a := range f.db.audit {
		audited = audited || a.Action == "admin.email_verification_resent" && a.TargetID == f.user.ID.String()
	}
	if !audited {
		t.Fatal("the resend must be audited")
	}
	u := f.db.users[f.user.ID]
	u.Status = domain.UserDeactivated
	f.db.users[u.ID] = u
	f.clock.Advance(2 * time.Minute)
	_, err := f.svc.AdminResend(ctx, f.user.ID)
	wantCode(t, err, apperror.CodeInvalidState)
}

func TestVerifiedHooksRunInOrder(t *testing.T) {
	f := newVerifyFixture(t)
	var order []string
	f.svc.OnVerified(func(context.Context, uuid.UUID) { order = append(order, "first") })
	f.svc.OnVerified(nil)
	f.svc.OnVerified(func(context.Context, uuid.UUID) { order = append(order, "second") })
	f.svc.OnVerified(func(context.Context, uuid.UUID) { order = append(order, "third") })
	f.svc.OnRegistered(context.Background(), &f.user)
	if _, err := f.svc.Verify(context.Background(), f.lastToken(t)); err != nil {
		t.Fatal(err)
	}
	if len(f.hooked) != 1 || strings.Join(order, ",") != "first,second,third" {
		t.Fatalf("hooks: %v %v", f.hooked, order)
	}
}

// Opening a link and sending one lock the same rows in the same order (the
// user, then the tokens): the opposite order deadlocks a link opened while a
// resend, a console resend or a reminder runs for the same account.
func TestEveryPathLocksTheUserBeforeWritingTokens(t *testing.T) {
	f := newVerifyFixture(t)
	ctx := context.Background()
	check := func(name string) {
		t.Helper()
		if len(f.locks.calls) == 0 || f.locks.calls[0] != "lock user" {
			t.Fatalf("%s: a token was written before the user was locked: %v", name, f.locks.calls)
		}
		f.locks.calls = nil
	}
	f.svc.OnRegistered(ctx, &f.user)
	check("welcome link")
	f.clock.Advance(2 * time.Minute)
	if _, err := f.svc.Resend(ctx, f.user.ID); err != nil {
		t.Fatal(err)
	}
	check("resend")
	f.clock.Advance(2 * time.Minute)
	if _, err := f.svc.AdminResend(ctx, f.user.ID); err != nil {
		t.Fatal(err)
	}
	check("console resend")
	f.clock.Advance(2 * time.Minute)
	if err := f.svc.SendReminder(ctx, f.user.ID); err != nil {
		t.Fatal(err)
	}
	check("reminder")
	if _, err := f.svc.Verify(ctx, f.lastToken(t)); err != nil {
		t.Fatal(err)
	}
	check("verify")
}

// The side effects of a verification belong to the committed transaction: a
// failure after the address was stamped rolls everything back and announces
// nothing (no event, notification, audit, analytics or hook).
func TestAFailedVerificationAnnouncesNothing(t *testing.T) {
	f := newVerifyFixture(t)
	ctx := context.Background()
	f.svc.OnRegistered(ctx, &f.user)
	raw := f.lastToken(t)
	f.actions.failInvalidate = true

	_, err := f.svc.Verify(ctx, raw)
	wantCode(t, err, apperror.CodeInternal)
	if f.db.users[f.user.ID].EmailVerifiedAt != nil {
		t.Fatal("the rolled back transaction left the address verified")
	}
	if len(f.events.events) != 0 || len(f.notes.spec) != 0 || len(f.hooked) != 0 {
		t.Fatalf("a failed verification announced something: %d events, %d notifications, %d hooks", len(f.events.events), len(f.notes.spec), len(f.hooked))
	}
	for _, a := range f.db.audit {
		if a.Action == "account.email_verified" {
			t.Fatal("a failed verification was audited")
		}
	}
	// The link survived the rollback: it still works once the database does.
	f.actions.failInvalidate = false
	if _, err := f.svc.Verify(ctx, raw); err != nil {
		t.Fatalf("the link must survive a rolled back attempt: %v", err)
	}
	if len(f.events.events) != 1 || len(f.hooked) != 1 {
		t.Fatalf("the retry must announce once: %d events, %d hooks", len(f.events.events), len(f.hooked))
	}
}

// A link that was replaced while the request waited for the user row is
// refused like any used link, and changes nothing.
func TestALinkReplacedMeanwhileIsRefused(t *testing.T) {
	f := newVerifyFixture(t)
	ctx := context.Background()
	f.svc.OnRegistered(ctx, &f.user)
	raw := f.lastToken(t)
	tok, err := f.actions.FindOpen(ctx, security.HashToken(raw), domain.ActionEmailVerify, f.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	// What a concurrent resend does between the lookup and the user lock.
	if err := f.actions.memActions.InvalidateOpen(ctx, f.user.ID, domain.ActionEmailVerify, f.clock.Now()); err != nil {
		t.Fatal(err)
	}
	if err := f.actions.Use(ctx, tok.ID, f.clock.Now()); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("using a replaced token: %v", err)
	}
	_, err = f.svc.Verify(ctx, raw)
	wantCode(t, err, apperror.CodeInvalidToken)
	if f.db.users[f.user.ID].EmailVerifiedAt != nil || len(f.events.events) != 0 {
		t.Fatal("a replaced link verified the address")
	}
}

// An account receives at most DailyLimit links in any 24 hours, by every
// path: the address is unproven, and security e-mail escapes the outbox's
// daily cap.
func TestLinksAreCappedPerDay(t *testing.T) {
	f := newVerifyFixture(t)
	ctx := context.Background()
	f.svc.OnRegistered(ctx, &f.user) // 1
	for i := 2; i <= 5; i++ {
		f.clock.Advance(2 * time.Minute)
		if _, err := f.svc.Resend(ctx, f.user.ID); err != nil {
			t.Fatalf("link %d: %v", i, err)
		}
	}
	f.clock.Advance(2 * time.Minute)
	_, err := f.svc.Resend(ctx, f.user.ID)
	e := wantCode(t, err, apperror.CodeRateLimited)
	// The welcome link was sent 10 minutes ago: it leaves the window in 23 h 50 min.
	if want := 24*time.Hour - 10*time.Minute; e.RetryAfter != want {
		t.Fatalf("Retry-After %v, want %v", e.RetryAfter, want)
	}
	if !strings.Contains(e.Message, "maximum number of verification emails") {
		t.Fatalf("message %q", e.Message)
	}
	_, err = f.svc.AdminResend(ctx, f.user.ID)
	wantCode(t, err, apperror.CodeRateLimited)
	if err := f.svc.SendReminder(ctx, f.user.ID); err != nil {
		t.Fatalf("a capped reminder must be silent: %v", err)
	}
	if n := len(f.mail.byTemplate("email_verify")); n != 5 {
		t.Fatalf("%d links sent, want 5", n)
	}
	// Once the oldest link leaves the window, one more may go.
	f.clock.Advance(e.RetryAfter)
	if _, err := f.svc.Resend(ctx, f.user.ID); err != nil {
		t.Fatalf("after the window: %v", err)
	}
}

// No link is "sent" to an address that refuses e-mail (a permanent bounce or
// a complaint): no token, no outbox row, and a clear answer.
func TestNoLinkForAnAddressThatRefusesEmail(t *testing.T) {
	f := newVerifyFixture(t)
	ctx := context.Background()
	f.suppressed[f.user.Email] = true
	f.svc.OnRegistered(ctx, &f.user)
	if err := f.svc.SendReminder(ctx, f.user.ID); err != nil {
		t.Fatalf("a reminder to a refusing address must be silent: %v", err)
	}
	_, err := f.svc.Resend(ctx, f.user.ID)
	wantCode(t, err, apperror.CodeEmailUndeliverable)
	_, err = f.svc.AdminResend(ctx, f.user.ID)
	wantCode(t, err, apperror.CodeEmailUndeliverable)
	if len(f.mail.sent) != 0 || len(f.db.actions) != 0 {
		t.Fatalf("%d e-mails and %d tokens for a refusing address", len(f.mail.sent), len(f.db.actions))
	}
	if p, err := f.svc.Pending(ctx, f.user.ID); err != nil || p != nil {
		t.Fatalf("no link may be pending: %+v %v", p, err)
	}
}

// Pending tells the clients whether a link is waiting in the inbox: accounts
// created before verification existed (or by the team) have none.
func TestPendingDescribesTheOpenLink(t *testing.T) {
	f := newVerifyFixture(t)
	ctx := context.Background()
	if p, err := f.svc.Pending(ctx, f.user.ID); err != nil || p != nil {
		t.Fatalf("an account that never received a link: %+v %v", p, err)
	}
	sentAt := f.clock.Now()
	f.svc.OnRegistered(ctx, &f.user)
	f.clock.Advance(10 * time.Second)
	p, err := f.svc.Pending(ctx, f.user.ID)
	if err != nil || p == nil || !p.SentAt.Equal(sentAt) || !p.ResendAvailableAt.Equal(sentAt.Add(time.Minute)) {
		t.Fatalf("pending: %+v %v", p, err)
	}
	f.clock.Advance(48 * time.Hour)
	if p, err := f.svc.Pending(ctx, f.user.ID); err != nil || p != nil {
		t.Fatalf("an expired link is not pending: %+v %v", p, err)
	}
}
