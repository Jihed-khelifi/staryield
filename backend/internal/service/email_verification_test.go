package service

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

type verifyFixture struct {
	db     *memDB
	clock  *clock.Fake
	mail   *memMail
	events *eventLog
	notes  *recordingNotifier
	svc    *EmailVerificationService
	user   domain.User
	hooked []uuid.UUID
}

func newVerifyFixture(t *testing.T) *verifyFixture {
	t.Helper()
	db := newMemDB()
	clk := clock.NewFake(time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC))
	sealer, err := security.NewSealer(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	f := &verifyFixture{db: db, clock: clk, mail: &memMail{}, events: &eventLog{}, notes: &recordingNotifier{}}
	f.svc = NewEmailVerificationService(EmailVerificationDeps{Tx: memTx{db: db}, Users: memUsers{db: db}, Actions: memActions{db: db},
		Mail: f.mail, Sealer: sealer, Events: f.events, Notifier: f.notes, Audit: memAudit{db: db}, Clock: clk,
		Settings: EmailVerificationSettings{FrontendURL: "https://staryield.test", IPHashSecret: "secret"}})
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
	f.svc.OnVerified(func(context.Context, uuid.UUID) { order = append(order, "second") })
	f.svc.OnVerified(nil)
	f.svc.OnRegistered(context.Background(), &f.user)
	if _, err := f.svc.Verify(context.Background(), f.lastToken(t)); err != nil {
		t.Fatal(err)
	}
	if len(f.hooked) != 1 || len(order) != 1 {
		t.Fatalf("hooks: %v %v", f.hooked, order)
	}
}
