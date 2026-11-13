package service

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

type inlineTx struct{}

func (inlineTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// memEmailRepo is an in-memory EmailRepository for policy tests.
type memEmailRepo struct {
	mu   sync.Mutex
	rows map[uuid.UUID]*domain.EmailOutbox
	supp map[string]domain.EmailSuppression
}

func newMemEmailRepo() *memEmailRepo {
	return &memEmailRepo{rows: map[uuid.UUID]*domain.EmailOutbox{}, supp: map[string]domain.EmailSuppression{}}
}

func (m *memEmailRepo) Enqueue(_ context.Context, rows []domain.EmailOutbox) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range rows {
		dup := false
		for _, r := range m.rows {
			if rows[i].DedupeKey != "" && r.DedupeKey == rows[i].DedupeKey {
				dup = true
			}
		}
		if !dup {
			r := rows[i]
			m.rows[r.ID] = &r
		}
	}
	return nil
}

func (m *memEmailRepo) CountRecent(_ context.Context, userID uuid.UUID, since time.Time, exclude domain.EmailCategory) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var n int64
	for _, r := range m.rows {
		if r.UserID != nil && *r.UserID == userID && !r.CreatedAt.Before(since) && r.Category != exclude &&
			r.Status != domain.EmailCancelled && r.Status != domain.EmailSuppressed {
			n++
		}
	}
	return n, nil
}

func (m *memEmailRepo) Suppressed(_ context.Context, hashes []string, now time.Time) (map[string]bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]bool{}
	for _, h := range hashes {
		if s, ok := m.supp[h]; ok && (s.ExpiresAt == nil || s.ExpiresAt.After(now)) {
			out[h] = true
		}
	}
	return out, nil
}

func (m *memEmailRepo) Suppress(_ context.Context, s *domain.EmailSuppression) error {
	m.mu.Lock()
	m.supp[s.EmailHash] = *s
	m.mu.Unlock()
	return nil
}

func (m *memEmailRepo) Claim(_ context.Context, now time.Time, lease time.Duration, limit int) ([]domain.EmailOutbox, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.EmailOutbox
	for _, r := range m.rows {
		due := (r.Status == domain.EmailQueued && !r.NextAttemptAt.After(now)) ||
			(r.Status == domain.EmailSending && r.LockedUntil != nil && !r.LockedUntil.After(now))
		if due && len(out) < limit {
			until := now.Add(lease)
			r.Status, r.LockedUntil = domain.EmailSending, &until
			r.Attempts++
			out = append(out, *r)
		}
	}
	return out, nil
}

func (m *memEmailRepo) Finish(_ context.Context, id uuid.UUID, f map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.rows[id]
	for k, v := range f {
		switch k {
		case "status":
			r.Status = v.(domain.EmailStatus)
		case "status_reason":
			r.StatusReason = v.(string)
		case "last_error":
			r.LastError = v.(string)
		case "to_sealed":
			r.ToSealed = v.(string)
		case "payload_sealed":
			r.PayloadSealed = v.(string)
		case "next_attempt_at":
			r.NextAttemptAt = v.(time.Time)
		case "locked_until":
			r.LockedUntil = nil
		case "provider_message_id":
			r.ProviderMessageID = v.(string)
		case "sent_at":
			t := v.(time.Time)
			r.SentAt = &t
		}
	}
	return nil
}

func (m *memEmailRepo) PurgeFinished(context.Context, time.Time) (int64, error) { return 0, nil }
func (m *memEmailRepo) Get(_ context.Context, id uuid.UUID) (*domain.EmailOutbox, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *r
	return &cp, nil
}
func (m *memEmailRepo) List(context.Context, *uuid.UUID, string, int) ([]domain.EmailOutbox, error) {
	return nil, nil
}

func (m *memEmailRepo) byTemplate(t string) []domain.EmailOutbox {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.EmailOutbox
	for _, r := range m.rows {
		if r.Template == t {
			out = append(out, *r)
		}
	}
	return out
}

func newTestMail(t *testing.T) (*MailService, *memEmailRepo, *mail.Recorder, *clock.Fake) {
	t.Helper()
	sealer, err := security.NewSealer([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	repo := newMemEmailRepo()
	rec := &mail.Recorder{}
	clk := clock.NewFake(time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC))
	svc := NewMailService(inlineTx{}, repo, rec, sealer, clk, zap.NewNop(), MailSettings{FrontendURL: "https://staryield.net",
		DailyCap: 3, MaxAttempts: 3, HashKey: "test-key"})
	svc.jitter = func() float64 { return 0.5 } // no jitter: deterministic backoff
	return svc, repo, rec, clk
}

func req(user uuid.UUID, cat domain.EmailCategory, tmpl string) EmailRequest {
	return EmailRequest{UserID: &user, To: "Nova@Example.com", Locale: "fr", Category: cat, Template: tmpl,
		Content: mail.PasswordReset("fr", "https://staryield.net/fr/reset-password?token=SECRET", 30)}
}

func TestMailEnqueueSealsAndDelivers(t *testing.T) {
	svc, repo, rec, _ := newTestMail(t)
	user := uuid.New()
	if err := svc.Enqueue(context.Background(), req(user, domain.EmailSecurity, "password_reset")); err != nil {
		t.Fatal(err)
	}
	rows := repo.byTemplate("password_reset")
	if len(rows) != 1 || rows[0].Status != domain.EmailQueued || rows[0].Locale != "fr" {
		t.Fatalf("rows: %+v", rows)
	}
	if strings.Contains(rows[0].ToSealed, "example.com") || strings.Contains(rows[0].PayloadSealed, "SECRET") {
		t.Fatal("recipient or one-time link stored in clear")
	}
	if rows[0].ToHash != svc.AddressHash("nova@example.com") {
		t.Fatal("address hash not normalised")
	}
	select {
	case <-svc.Wake():
	default:
		t.Fatal("enqueue must wake the delivery loop")
	}
	if n, err := svc.Drain(context.Background()); err != nil || n != 1 {
		t.Fatalf("drain: %d %v", n, err)
	}
	sent := rec.Sent()
	if len(sent) != 1 || sent[0].To != "nova@example.com" || !strings.Contains(sent[0].Text, "token=SECRET") || sent[0].Category != "security" {
		t.Fatalf("sent: %+v", sent)
	}
	got, _ := repo.Get(context.Background(), rows[0].ID)
	if got.Status != domain.EmailSent || got.SentAt == nil || got.PayloadSealed != "" || got.ToSealed != "" || got.ProviderMessageID == "" {
		t.Fatalf("after send: %+v", got)
	}
	// Nothing left to deliver.
	if n, _ := svc.DeliverDue(context.Background(), 10); n != 0 {
		t.Fatalf("redelivered %d", n)
	}
}

func TestMailMarketingIsNeverSentThroughTransactionalProvider(t *testing.T) {
	svc, repo, rec, _ := newTestMail(t)
	if err := svc.Enqueue(context.Background(), req(uuid.New(), domain.EmailMarketing, "notification:broadcast")); err != nil {
		t.Fatal(err)
	}
	rows := repo.byTemplate("notification:broadcast")
	if len(rows) != 1 || rows[0].Status != domain.EmailCancelled || rows[0].StatusReason != reasonNoMarketingProvider || rows[0].PayloadSealed != "" {
		t.Fatalf("marketing row: %+v", rows)
	}
	_, _ = svc.Drain(context.Background())
	if len(rec.Sent()) != 0 {
		t.Fatal("marketing e-mail sent")
	}
}

func TestMailDailyCapSparesSecurityMail(t *testing.T) {
	svc, repo, _, _ := newTestMail(t)
	user := uuid.New()
	for i := 0; i < 5; i++ {
		if err := svc.Enqueue(context.Background(), req(user, domain.EmailNotification, "notification:chat.missed")); err != nil {
			t.Fatal(err)
		}
	}
	var queued, capped int
	for _, r := range repo.byTemplate("notification:chat.missed") {
		switch {
		case r.Status == domain.EmailQueued:
			queued++
		case r.StatusReason == reasonDailyCap:
			capped++
		}
	}
	if queued != 3 || capped != 2 {
		t.Fatalf("cap: queued=%d capped=%d", queued, capped)
	}
	// Security mail is exempt from the cap.
	if err := svc.Enqueue(context.Background(), req(user, domain.EmailSecurity, "password_reset")); err != nil {
		t.Fatal(err)
	}
	if r := repo.byTemplate("password_reset"); len(r) != 1 || r[0].Status != domain.EmailQueued {
		t.Fatalf("security mail capped: %+v", r)
	}
	// A batch counts towards the cap too.
	other := uuid.New()
	batch := []EmailRequest{req(other, domain.EmailBilling, "b1"), req(other, domain.EmailBilling, "b2"), req(other, domain.EmailBilling, "b3"),
		req(other, domain.EmailBilling, "b4")}
	if err := svc.Enqueue(context.Background(), batch...); err != nil {
		t.Fatal(err)
	}
	if r := repo.byTemplate("b4"); len(r) != 1 || r[0].StatusReason != reasonDailyCap {
		t.Fatalf("batch cap: %+v", r)
	}
}

func TestMailRetriesThenDies(t *testing.T) {
	svc, repo, rec, clk := newTestMail(t)
	rec.SetFail(func(mail.Message) (mail.Result, error) {
		return mail.Result{}, &mail.SendError{Status: 503, Code: 10100, Reason: "upstream", Retryable: true}
	})
	_ = svc.Enqueue(context.Background(), req(uuid.New(), domain.EmailSecurity, "password_reset"))
	id := repo.byTemplate("password_reset")[0].ID

	_, _ = svc.Drain(context.Background())
	r, _ := repo.Get(context.Background(), id)
	if r.Status != domain.EmailQueued || r.Attempts != 1 || !strings.Contains(r.LastError, "10100") {
		t.Fatalf("after first failure: %+v", r)
	}
	if got := r.NextAttemptAt.Sub(clk.Now()); got != 30*time.Second {
		t.Fatalf("first backoff %v, want 30s", got)
	}
	// Not due yet: no attempt.
	if n, _ := svc.DeliverDue(context.Background(), 10); n != 0 {
		t.Fatal("retried before the backoff elapsed")
	}
	clk.Advance(31 * time.Second)
	_, _ = svc.Drain(context.Background())
	r, _ = repo.Get(context.Background(), id)
	if r.Attempts != 2 || r.NextAttemptAt.Sub(clk.Now()) != time.Minute {
		t.Fatalf("second backoff: %+v (%v)", r, r.NextAttemptAt.Sub(clk.Now()))
	}
	clk.Advance(2 * time.Minute)
	_, _ = svc.Drain(context.Background())
	r, _ = repo.Get(context.Background(), id)
	if r.Status != domain.EmailDead || r.StatusReason != reasonMaxAttempts || r.PayloadSealed != "" || r.ToSealed != "" {
		t.Fatalf("after max attempts: %+v", r)
	}
}

func TestMailPermanentFailureAndBounce(t *testing.T) {
	svc, repo, rec, _ := newTestMail(t)
	rec.SetFail(func(mail.Message) (mail.Result, error) {
		return mail.Result{}, &mail.SendError{Status: 400, Code: 10001, Reason: "email.sending.error.invalid_request_schema"}
	})
	_ = svc.Enqueue(context.Background(), req(uuid.New(), domain.EmailSecurity, "rejected"))
	_, _ = svc.Drain(context.Background())
	if r := repo.byTemplate("rejected")[0]; r.Status != domain.EmailFailed || r.Attempts != 1 {
		t.Fatalf("4xx must fail without retry: %+v", r)
	}

	rec.SetFail(func(m mail.Message) (mail.Result, error) { return mail.Result{Suppressed: []string{m.To}}, nil })
	_ = svc.Enqueue(context.Background(), req(uuid.New(), domain.EmailSecurity, "bounce"))
	_, _ = svc.Drain(context.Background())
	if r := repo.byTemplate("bounce")[0]; r.Status != domain.EmailSuppressed || r.StatusReason != reasonBounced {
		t.Fatalf("bounce: %+v", r)
	}
	// The address is now suppressed: new mail is not even queued.
	rec.SetFail(nil)
	_ = svc.Enqueue(context.Background(), req(uuid.New(), domain.EmailSecurity, "after_bounce"))
	if r := repo.byTemplate("after_bounce")[0]; r.Status != domain.EmailSuppressed || r.StatusReason != reasonSuppressed {
		t.Fatalf("suppressed address queued: %+v", r)
	}
	if len(rec.Sent()) != 0 {
		t.Fatal("mail sent to a suppressed address")
	}
}

func TestMailRetryAfterAndUnreadableRows(t *testing.T) {
	svc, repo, rec, clk := newTestMail(t)
	rec.SetFail(func(mail.Message) (mail.Result, error) {
		return mail.Result{}, &mail.SendError{Status: 429, Code: 10004, Reason: "throttled", Retryable: true, RetryAfter: 10 * time.Minute}
	})
	_ = svc.Enqueue(context.Background(), req(uuid.New(), domain.EmailSecurity, "throttled"))
	_, _ = svc.Drain(context.Background())
	r := repo.byTemplate("throttled")[0]
	if r.NextAttemptAt.Sub(clk.Now()) != 10*time.Minute {
		t.Fatalf("retry-after not honoured: %v", r.NextAttemptAt.Sub(clk.Now()))
	}
	// A row sealed with another key (or tampered) is dropped, not retried forever.
	rec.SetFail(nil)
	_ = svc.Enqueue(context.Background(), req(uuid.New(), domain.EmailSecurity, "tampered"))
	tampered := repo.byTemplate("tampered")[0]
	repo.mu.Lock()
	repo.rows[tampered.ID].PayloadSealed = "v1:AAAA"
	repo.mu.Unlock()
	_, _ = svc.Drain(context.Background())
	if r, _ := repo.Get(context.Background(), tampered.ID); r.Status != domain.EmailFailed || r.StatusReason != reasonUnreadable {
		t.Fatalf("tampered row: %+v", r)
	}
}

func TestMailSkipsInvalidRequestsAndDedupes(t *testing.T) {
	svc, repo, _, _ := newTestMail(t)
	user := uuid.New()
	bad := req(user, domain.EmailSecurity, "invalid")
	bad.To = "not-an-email"
	if err := svc.Enqueue(context.Background(), bad); err != nil {
		t.Fatal(err)
	}
	if len(repo.byTemplate("invalid")) != 0 {
		t.Fatal("invalid recipient queued")
	}
	d := req(user, domain.EmailNotification, "dedupe")
	d.DedupeKey = "notification:1"
	_ = svc.Enqueue(context.Background(), d)
	_ = svc.Enqueue(context.Background(), d)
	if n := len(repo.byTemplate("dedupe")); n != 1 {
		t.Fatalf("dedupe: %d rows", n)
	}
}

func TestMailBackoffBounds(t *testing.T) {
	svc, _, _, _ := newTestMail(t)
	svc.jitter = func() float64 { return 1 } // +20%
	if got := svc.backoff(1, 0); got != 36*time.Second {
		t.Fatalf("attempt 1 +20%%: %v", got)
	}
	svc.jitter = func() float64 { return 0 } // -20%
	if got := svc.backoff(20, 0); got != 48*time.Minute {
		t.Fatalf("cap -20%%: %v", got)
	}
	if got := svc.backoff(1, 48*time.Hour); got != 24*time.Hour {
		t.Fatalf("retry-after cap: %v", got)
	}
}

func TestChannelPolicies(t *testing.T) {
	on := repository.Contact{NotifyEmail: true, NotifyPush: true, NotifyMarketing: true}
	off := repository.Contact{}
	cases := []struct {
		kind              string
		emailOn, emailOff bool
		pushOn            bool
	}{
		{NotifyChatRequested, false, false, true},
		{NotifyChatMissed, true, false, true},
		{NotifyWithdrawal, true, true, true}, // billing: always e-mailed
		{NotifyExportReady, true, true, true},
		{NotifyBroadcast, true, false, true},             // marketing: opt-in only
		{NotifyPurchase, true, true, true},               // revisions 5.1: the receipt is billing mail, always e-mailed (§25.5)
		{NotifySubscriptionStarted, true, true, true},    // revisions 5.1
		{NotifyPaymentRefunded, true, true, true},        // revisions 5.1
		{NotifyAccountEmailChanged, false, false, false}, // sent directly, in-app only here
	}
	for _, c := range cases {
		p := PolicyFor(c.kind)
		if p.wantsEmail(on) != c.emailOn || p.wantsEmail(off) != c.emailOff || p.wantsPush(on) != c.pushOn || p.wantsPush(off) {
			t.Errorf("%s: email(on)=%v email(off)=%v push(on)=%v push(off)=%v", c.kind, p.wantsEmail(on), p.wantsEmail(off),
				p.wantsPush(on), p.wantsPush(off))
		}
	}
	// Marketing push needs both switches.
	if PolicyFor(NotifyBroadcast).wantsPush(repository.Contact{NotifyPush: true}) {
		t.Fatal("marketing push without marketing opt-in")
	}
}
