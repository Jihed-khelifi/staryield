package service

// revisions 3.3: messages outside a live chat (§22.3). In-memory doubles with
// a per-conversation row lock and transaction rollback, so the gates, the cap
// arithmetic and the concurrency rules can be tested without infrastructure.

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/ratelimit"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
)

type osTxKey struct{}

// osTx is one fake transaction: row locks released at the end, undo log run
// on failure.
type osTx struct {
	locks []*sync.Mutex
	undo  []func()
}

type osTxManager struct{}

func (osTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(osTxKey{}).(*osTx); ok {
		return fn(ctx)
	}
	tx := &osTx{}
	err := fn(context.WithValue(ctx, osTxKey{}, tx))
	if err != nil {
		for i := len(tx.undo) - 1; i >= 0; i-- {
			tx.undo[i]()
		}
	}
	for i := len(tx.locks) - 1; i >= 0; i-- {
		tx.locks[i].Unlock()
	}
	return err
}

func txOf(ctx context.Context) *osTx {
	tx, _ := ctx.Value(osTxKey{}).(*osTx)
	return tx
}

// osChat is an in-memory ChatRepository for the send path.
type osChat struct {
	repository.ChatRepository
	mu       sync.Mutex
	conv     *domain.Conversation
	convLock sync.Mutex
	sessions []domain.Session
	started  bool // the conversation had a started live chat
	messages []domain.Message
	// order records "session" / "conversation" lock acquisitions.
	order []string
	// failAppend makes AppendMessage fail once with the error.
	appendCalls atomic.Int64
}

func (r *osChat) GetConversation(context.Context, uuid.UUID) (*domain.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := *r.conv
	return &c, nil
}

func (r *osChat) GetConversationForUpdate(ctx context.Context, _ uuid.UUID) (*domain.Conversation, error) {
	r.convLock.Lock()
	if tx := txOf(ctx); tx != nil {
		tx.locks = append(tx.locks, &r.convLock)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.order = append(r.order, "conversation")
	c := *r.conv
	return &c, nil
}

func (r *osChat) OpenSessionsForConversations(context.Context, []uuid.UUID) ([]domain.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Session
	for _, s := range r.sessions {
		for _, st := range domain.OpenSessionStatuses {
			if s.Status == st {
				out = append(out, s)
			}
		}
	}
	return out, nil
}

func (r *osChat) GetSession(_ context.Context, id uuid.UUID) (*domain.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.sessions {
		if s.ID == id {
			c := s
			return &c, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *osChat) GetSessionForUpdate(ctx context.Context, id uuid.UUID) (*domain.Session, error) {
	r.mu.Lock()
	r.order = append(r.order, "session")
	r.mu.Unlock()
	return r.GetSession(ctx, id)
}

func (r *osChat) HasStartedSession(context.Context, uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.started, nil
}

func (r *osChat) GetMessageByClientID(_ context.Context, sender uuid.UUID, cmid string) (*domain.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range r.messages {
		if m.SenderID != nil && *m.SenderID == sender && m.ClientMsgID != nil && *m.ClientMsgID == cmid {
			c := m
			return &c, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *osChat) CountOffSession(_ context.Context, _, sender uuid.UUID, since time.Time) (repository.OffSessionCount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out repository.OffSessionCount
	for _, m := range r.messages {
		if m.SessionID == nil && m.SenderID != nil && *m.SenderID == sender && m.CreatedAt.After(since) {
			out.Count++
			if out.Oldest == nil || m.CreatedAt.Before(*out.Oldest) {
				t := m.CreatedAt
				out.Oldest = &t
			}
		}
	}
	return out, nil
}

func (r *osChat) AppendMessage(ctx context.Context, m *domain.Message) (*domain.Conversation, error) {
	r.appendCalls.Add(1)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.conv.LastSeq++
	m.Seq = r.conv.LastSeq
	r.messages = append(r.messages, *m)
	if tx := txOf(ctx); tx != nil {
		id := m.ID
		tx.undo = append(tx.undo, func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			for i := range r.messages {
				if r.messages[i].ID == id {
					r.messages = append(r.messages[:i], r.messages[i+1:]...)
					break
				}
			}
			r.conv.LastSeq--
		})
	}
	c := *r.conv
	return &c, nil
}

func (r *osChat) offSessionRows() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, m := range r.messages {
		if m.SessionID == nil {
			n++
		}
	}
	return n
}

// osUsers serves accounts by id.
type osUsers struct {
	repository.UserRepository
	users map[uuid.UUID]domain.User
}

func (u osUsers) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	if x, ok := u.users[id]; ok {
		return &x, nil
	}
	return nil, repository.ErrNotFound
}

// osPsychics reports which advisors exist (soft-deleted ones are absent).
type osPsychics struct {
	repository.PsychicRepository
	present map[uuid.UUID]bool
}

func (p osPsychics) GetByUserIDs(_ context.Context, ids []uuid.UUID) ([]domain.Psychic, error) {
	var out []domain.Psychic
	for _, id := range ids {
		if p.present[id] {
			out = append(out, domain.Psychic{UserID: id})
		}
	}
	return out, nil
}

type osAttachments struct {
	repository.AttachmentRepository
	fail    error
	created atomic.Int64
}

func (a *osAttachments) Create(context.Context, *domain.Attachment) error {
	if a.fail != nil {
		return a.fail
	}
	a.created.Add(1)
	return nil
}

func (a *osAttachments) ForMessages(context.Context, []uuid.UUID) (map[uuid.UUID][]domain.Attachment, error) {
	return map[uuid.UUID][]domain.Attachment{}, nil
}

type allowAll struct{}

func (allowAll) Allow(context.Context, string, ratelimit.Rule) (ratelimit.Result, error) {
	return ratelimit.Result{Allowed: true}, nil
}

type osRig struct {
	svc      *ChatService
	repo     *osChat
	users    osUsers
	psychics osPsychics
	rec      *analytics.Recorder
	clk      *clock.Fake
	store    *storage.Memory
	atts     *osAttachments
	client   uuid.UUID
	psychic  uuid.UUID
	conv     uuid.UUID
	cfg      OffSessionSettings
	free     bool
	cfgOK    bool
}

func newOSRig(t *testing.T) *osRig {
	t.Helper()
	r := &osRig{client: uuid.New(), psychic: uuid.New(), conv: uuid.New(), rec: &analytics.Recorder{},
		clk: clock.NewFake(time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)), store: storage.NewMemory("http://store.test"),
		atts: &osAttachments{}, cfg: OffSessionSettings{Enabled: true, ClientDailyLimit: 3, AdvisorDailyLimit: 5}, cfgOK: true}
	r.repo = &osChat{conv: &domain.Conversation{ID: r.conv, ClientID: r.client, PsychicID: r.psychic}, started: true}
	r.users = osUsers{users: map[uuid.UUID]domain.User{
		r.client:  {ID: r.client, Role: domain.RoleClient, Status: domain.UserActive},
		r.psychic: {ID: r.psychic, Role: domain.RolePsychic, Status: domain.UserActive},
	}}
	r.psychics = osPsychics{present: map[uuid.UUID]bool{r.psychic: true}}
	r.svc = &ChatService{tx: osTxManager{}, chat: r.repo, users: r.users, psychicsRepo: r.psychics, events: NoopPublisher{},
		tracker: r.rec, limiter: allowAll{}, clock: r.clk, cfg: ChatSettings{MaxMessageLength: 2000}}
	r.svc.WithExtras(ChatExtras{Attachments: r.atts, Store: r.store,
		Images: ImagePolicy{MaxBytes: 5 << 20, MaxPixels: 10_000_000, MaxSide: 2048, AllowGIF: true, KeepTransparency: true, Field: "image"}})
	r.svc.SetOffSessionSettings(func(context.Context) OffSessionSettings {
		if !r.cfgOK {
			return OffSessionSettings{}
		}
		return r.cfg
	})
	r.svc.SetFreeMode(func(context.Context) bool { return r.free })
	return r
}

func (r *osRig) asClient() reqctx.Principal {
	return reqctx.Principal{UserID: r.client, Role: domain.RoleClient}
}
func (r *osRig) asPsychic() reqctx.Principal {
	return reqctx.Principal{UserID: r.psychic, Role: domain.RolePsychic}
}

func (r *osRig) setSession(status domain.SessionStatus) uuid.UUID {
	id := uuid.New()
	started := time.Now()
	r.repo.mu.Lock()
	r.repo.sessions = []domain.Session{{ID: id, ConversationID: r.conv, ClientID: r.client, PsychicID: r.psychic, Status: status, StartedAt: &started}}
	r.repo.mu.Unlock()
	return id
}

func (r *osRig) send(p reqctx.Principal, body, cmid string) (*SendResult, error) {
	return r.svc.SendToConversation(context.Background(), p, ConversationSendInput{ConversationID: r.conv, Body: body, ClientMsgID: cmid})
}

func code(t *testing.T, err error) apperror.Code {
	t.Helper()
	var ae *apperror.Error
	if !errors.As(err, &ae) {
		t.Fatalf("want an apperror, got %v", err)
	}
	return ae.Code
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for x := 0; x < 32; x++ {
		for y := 0; y < 32; y++ {
			img.Set(x, y, color.RGBA{uint8(x * 8), uint8(y * 8), 90, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRoutingLiveStatusesGoIntoTheSession(t *testing.T) {
	for _, st := range []domain.SessionStatus{domain.SessionActive, domain.SessionDisconnected} {
		r := newOSRig(t)
		r.cfg = OffSessionSettings{} // the feature off: live routing must not care
		sid := r.setSession(st)
		for i := 0; i < 30; i++ { // far over any cap: live messages are never capped
			res, err := r.send(r.asClient(), "hello", "")
			if err != nil {
				t.Fatalf("%s: send %d: %v", st, i, err)
			}
			if res.Message.SessionID == nil || *res.Message.SessionID != sid || res.Message.OffSession {
				t.Fatalf("%s: message not attached to the session: %+v", st, res.Message)
			}
			if res.Allowance == nil || !res.Allowance.Live {
				t.Fatalf("%s: allowance %+v", st, res.Allowance)
			}
		}
		if r.repo.offSessionRows() != 0 {
			t.Fatalf("%s: off-session rows stored", st)
		}
		if names := r.rec.Names(); names[0] != "chat.message_sent" {
			t.Fatalf("%s: events %v", st, names)
		}
	}
}

func TestRoutingOtherStatusesGoOffSession(t *testing.T) {
	for _, st := range []domain.SessionStatus{domain.SessionRequested, domain.SessionAccepted, domain.SessionEnded, ""} {
		r := newOSRig(t)
		if st != "" {
			r.setSession(st)
		}
		res, err := r.send(r.asClient(), "hello", "")
		if err != nil {
			t.Fatalf("%q: %v", st, err)
		}
		if res.Message.SessionID != nil || !res.Message.OffSession || res.Allowance == nil || res.Allowance.Live {
			t.Fatalf("%q: %+v %+v", st, res.Message, res.Allowance)
		}
		if res.Allowance.Used != 1 || res.Allowance.Remaining != 2 || res.Allowance.Limit != 3 {
			t.Fatalf("%q: allowance %+v", st, res.Allowance)
		}
	}
}

func TestLiveRoutingLocksSessionBeforeConversation(t *testing.T) {
	r := newOSRig(t)
	r.setSession(domain.SessionActive)
	if _, err := r.send(r.asClient(), "hi", ""); err != nil {
		t.Fatal(err)
	}
	r.repo.mu.Lock()
	order := append([]string(nil), r.repo.order...)
	r.repo.mu.Unlock()
	if len(order) != 1 || order[0] != "session" {
		t.Fatalf("lock order %v", order)
	}
	// Off-session: only the conversation row is locked.
	r2 := newOSRig(t)
	if _, err := r2.send(r2.asClient(), "hi", ""); err != nil {
		t.Fatal(err)
	}
	if len(r2.repo.order) != 1 || r2.repo.order[0] != "conversation" {
		t.Fatalf("off-session lock order %v", r2.repo.order)
	}
}

func TestSessionEndedAfterRoutingFallsBackOffSession(t *testing.T) {
	r := newOSRig(t)
	// The routing read sees an active session, the lock read sees it ended.
	sid := r.setSession(domain.SessionActive)
	svc := r.svc
	svc.chat = &endedOnLock{osChat: r.repo, id: sid}
	res, err := r.send(r.asClient(), "late", "")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Message.OffSession {
		t.Fatalf("not off-session: %+v", res.Message)
	}
	if got := r.repo.order; len(got) != 2 || got[0] != "session" || got[1] != "conversation" {
		t.Fatalf("lock order %v", got)
	}
}

type endedOnLock struct {
	*osChat
	id uuid.UUID
}

func (e *endedOnLock) GetSessionForUpdate(ctx context.Context, id uuid.UUID) (*domain.Session, error) {
	s, err := e.osChat.GetSessionForUpdate(ctx, id)
	if err == nil {
		s.Status = domain.SessionEnded
	}
	return s, err
}

func (e *endedOnLock) OpenSessionsForConversations(ctx context.Context, ids []uuid.UUID) ([]domain.Session, error) {
	return e.osChat.OpenSessionsForConversations(ctx, ids)
}

func TestRefusalsFollowTheRuleOrder(t *testing.T) {
	r := newOSRig(t)
	ctx := context.Background()
	// Everything wrong at once: not a participant wins.
	stranger := reqctx.Principal{UserID: uuid.New(), Role: domain.RoleClient}
	r.cfg.Enabled = false
	r.repo.started = false
	r.users.users[r.psychic] = domain.User{ID: r.psychic, Status: domain.UserBanned}
	if _, err := r.send(stranger, "x", ""); code(t, err) != apperror.CodeNotFound {
		t.Fatalf("participant first: %v", err)
	}
	// Then the kill switch.
	if _, err := r.send(r.asClient(), "x", ""); code(t, err) != apperror.CodeOffSessionDisabled {
		t.Fatalf("disabled: %v", err)
	}
	// Then eligibility.
	r.cfg.Enabled = true
	if _, err := r.send(r.asClient(), "x", ""); code(t, err) != apperror.CodeOffSessionNotEligible {
		t.Fatalf("not eligible: %v", err)
	}
	// Then the recipient.
	r.repo.started = true
	if _, err := r.send(r.asClient(), "x", ""); code(t, err) != apperror.CodeRecipientUnavailable {
		t.Fatalf("recipient: %v", err)
	}
	// Then the cap (client limit 3).
	r.users.users[r.psychic] = domain.User{ID: r.psychic, Status: domain.UserActive}
	for i := 0; i < 3; i++ {
		if _, err := r.send(r.asClient(), "x", ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.send(r.asClient(), "x", ""); code(t, err) != apperror.CodeOffSessionLimit {
		t.Fatalf("limit: %v", err)
	}
	if r.repo.offSessionRows() != 3 {
		t.Fatalf("rows %d", r.repo.offSessionRows())
	}
	_ = ctx
	want := []string{"disabled", "not_eligible", "recipient", "limit"}
	var got []string
	for _, e := range r.rec.Events {
		if e.Name == "chat.off_session_refused" {
			got = append(got, e.Properties["reason"].(string))
			if _, hasBody := e.Properties["body"]; hasBody {
				t.Fatal("message text in analytics")
			}
		}
	}
	if len(got) != len(want) {
		t.Fatalf("refusal events %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("refusal events %v want %v", got, want)
		}
	}
}

func TestRecipientStatesRefused(t *testing.T) {
	for name, mutate := range map[string]func(r *osRig){
		"deleted":     func(r *osRig) { r.users.users[r.psychic] = domain.User{ID: r.psychic, Status: domain.UserDeleted} },
		"banned":      func(r *osRig) { r.users.users[r.psychic] = domain.User{ID: r.psychic, Status: domain.UserBanned} },
		"deactivated": func(r *osRig) { r.users.users[r.psychic] = domain.User{ID: r.psychic, Status: domain.UserDeactivated} },
		"gone":        func(r *osRig) { delete(r.users.users, r.psychic) },
		"removed advisor": func(r *osRig) {
			r.psychics.present[r.psychic] = false
		},
	} {
		r := newOSRig(t)
		mutate(r)
		if _, err := r.send(r.asClient(), "x", ""); code(t, err) != apperror.CodeRecipientUnavailable {
			t.Fatalf("%s: %v", name, err)
		}
	}
	// An advisor writing to a deleted client is refused too.
	r := newOSRig(t)
	r.users.users[r.client] = domain.User{ID: r.client, Status: domain.UserDeleted}
	if _, err := r.send(r.asPsychic(), "x", ""); code(t, err) != apperror.CodeRecipientUnavailable {
		t.Fatalf("client deleted: %v", err)
	}
	// An advisor-side check does not require the advisor row for a client recipient.
	r2 := newOSRig(t)
	r2.psychics.present[r2.psychic] = false
	if _, err := r2.send(r2.asPsychic(), "x", ""); err != nil {
		t.Fatalf("advisor to client: %v", err)
	}
}

func TestPerRoleCapBoundaries(t *testing.T) {
	for _, tc := range []struct {
		role  string
		limit int
	}{{"client", 3}, {"advisor", 5}} {
		r := newOSRig(t)
		p := r.asClient()
		if tc.role == "advisor" {
			p = r.asPsychic()
		}
		for i := 1; i <= tc.limit; i++ {
			res, err := r.send(p, "msg", "")
			if err != nil {
				t.Fatalf("%s message %d: %v", tc.role, i, err)
			}
			if res.Allowance.Used != i || res.Allowance.Remaining != tc.limit-i {
				t.Fatalf("%s %d: %+v", tc.role, i, res.Allowance)
			}
		}
		_, err := r.send(p, "one too many", "")
		var ae *apperror.Error
		if !errors.As(err, &ae) || ae.Code != apperror.CodeOffSessionLimit || ae.Status != 429 {
			t.Fatalf("%s: want 429 off_session_limit_reached, got %v", tc.role, err)
		}
		if ae.Details["limit"] != tc.limit || ae.Details["used"] != tc.limit {
			t.Fatalf("%s: details %v", tc.role, ae.Details)
		}
		wantReset := r.clk.Now().Add(24 * time.Hour)
		if got := ae.Details["resets_at"].(time.Time); !got.Equal(wantReset) {
			t.Fatalf("%s: resets_at %v want %v", tc.role, got, wantReset)
		}
		if ae.RetryAfter != 24*time.Hour {
			t.Fatalf("%s: retry after %v", tc.role, ae.RetryAfter)
		}
		if r.repo.offSessionRows() != tc.limit {
			t.Fatalf("%s: rows %d", tc.role, r.repo.offSessionRows())
		}
	}
}

func TestCapsAreIndependentPerSenderAndRole(t *testing.T) {
	r := newOSRig(t)
	for i := 0; i < 3; i++ {
		if _, err := r.send(r.asClient(), "x", ""); err != nil {
			t.Fatal(err)
		}
	}
	// The client is capped; the advisor is not affected.
	if _, err := r.send(r.asClient(), "x", ""); code(t, err) != apperror.CodeOffSessionLimit {
		t.Fatalf("client: %v", err)
	}
	if _, err := r.send(r.asPsychic(), "reply", ""); err != nil {
		t.Fatalf("advisor: %v", err)
	}
}

func TestRollingWindowWithFakeClock(t *testing.T) {
	r := newOSRig(t)
	start := r.clk.Now()
	if _, err := r.send(r.asClient(), "a", ""); err != nil {
		t.Fatal(err)
	}
	r.clk.Advance(time.Hour)
	for i := 0; i < 2; i++ {
		if _, err := r.send(r.asClient(), "b", ""); err != nil {
			t.Fatal(err)
		}
	}
	_, err := r.send(r.asClient(), "c", "")
	var ae *apperror.Error
	if !errors.As(err, &ae) || ae.Code != apperror.CodeOffSessionLimit {
		t.Fatalf("want limit, got %v", err)
	}
	// resets_at is when the OLDEST counted message leaves the window.
	if got := ae.Details["resets_at"].(time.Time); !got.Equal(start.Add(24 * time.Hour)) {
		t.Fatalf("resets_at %v want %v", got, start.Add(24*time.Hour))
	}
	if ae.RetryAfter != 23*time.Hour {
		t.Fatalf("retry after %v", ae.RetryAfter)
	}
	// Exactly at resets_at the first message no longer counts.
	r.clk.Set(start.Add(24 * time.Hour))
	res, err := r.send(r.asClient(), "d", "")
	if err != nil {
		t.Fatalf("at the boundary: %v", err)
	}
	if res.Allowance.Used != 3 {
		t.Fatalf("used %d", res.Allowance.Used)
	}
	// 24 h + 1 s after the first message: it is gone (used 3 -> still full
	// with the two from +1h and the new one; one second before it leaves the
	// window it still counted).
	r2 := newOSRig(t)
	if _, err := r2.send(r2.asClient(), "old", ""); err != nil {
		t.Fatal(err)
	}
	r2.clk.Advance(24*time.Hour - time.Second)
	if a, _ := r2.svc.OffSessionAllowanceFor(context.Background(), r2.asClient(), r2.conv); a.Used != 1 {
		t.Fatalf("23h59m59s old must count: %+v", a)
	}
	r2.clk.Advance(2 * time.Second)
	a, err := r2.svc.OffSessionAllowanceFor(context.Background(), r2.asClient(), r2.conv)
	if err != nil || a.Used != 0 || a.Remaining != 3 || a.ResetsAt != nil {
		t.Fatalf("24h+1s old must not count: %+v %v", a, err)
	}
}

func TestFreeModeBypassesOnlyTheCap(t *testing.T) {
	r := newOSRig(t)
	r.free = true
	for i := 0; i < 12; i++ { // far over the client cap of 3
		res, err := r.send(r.asClient(), "free", "")
		if err != nil {
			t.Fatalf("free mode send %d: %v", i, err)
		}
		if !res.Allowance.Unlimited || res.Allowance.Remaining != res.Allowance.Limit {
			t.Fatalf("allowance %+v", res.Allowance)
		}
	}
	// The kill switch still applies.
	r.cfg.Enabled = false
	if _, err := r.send(r.asClient(), "x", ""); code(t, err) != apperror.CodeOffSessionDisabled {
		t.Fatalf("kill switch: %v", err)
	}
	r.cfg.Enabled = true
	// So does a role limit of 0.
	r.cfg.ClientDailyLimit = 0
	if _, err := r.send(r.asClient(), "x", ""); code(t, err) != apperror.CodeOffSessionDisabled {
		t.Fatalf("limit 0: %v", err)
	}
	r.cfg.ClientDailyLimit = 3
	// And eligibility and the recipient.
	r.repo.started = false
	if _, err := r.send(r.asClient(), "x", ""); code(t, err) != apperror.CodeOffSessionNotEligible {
		t.Fatalf("eligibility: %v", err)
	}
	r.repo.started = true
	r.users.users[r.psychic] = domain.User{ID: r.psychic, Status: domain.UserBanned}
	if _, err := r.send(r.asClient(), "x", ""); code(t, err) != apperror.CodeRecipientUnavailable {
		t.Fatalf("recipient: %v", err)
	}
}

func TestLimitZeroForOneRoleDisablesOnlyThatRole(t *testing.T) {
	r := newOSRig(t)
	r.cfg.AdvisorDailyLimit = 0
	if _, err := r.send(r.asPsychic(), "x", ""); code(t, err) != apperror.CodeOffSessionDisabled {
		t.Fatalf("advisor: %v", err)
	}
	if _, err := r.send(r.asClient(), "x", ""); err != nil {
		t.Fatalf("client: %v", err)
	}
	a, err := r.svc.OffSessionAllowanceFor(context.Background(), r.asPsychic(), r.conv)
	if err != nil || a.Enabled || a.Limit != 0 || a.Remaining != 0 {
		t.Fatalf("advisor allowance %+v %v", a, err)
	}
}

func TestUnreadableSettingsDisableTheFeature(t *testing.T) {
	r := newOSRig(t)
	r.cfgOK = false
	if _, err := r.send(r.asClient(), "x", ""); code(t, err) != apperror.CodeOffSessionDisabled {
		t.Fatalf("unreadable: %v", err)
	}
	// And with nothing wired at all.
	r.svc.offSession = nil
	if _, err := r.send(r.asClient(), "x", ""); code(t, err) != apperror.CodeOffSessionDisabled {
		t.Fatalf("unwired: %v", err)
	}
	// Live chats are unaffected by the setting.
	r.setSession(domain.SessionActive)
	if _, err := r.send(r.asClient(), "x", ""); err != nil {
		t.Fatalf("live: %v", err)
	}
}

func TestIdempotentReplayNeverCountsAndSkipsEveryGate(t *testing.T) {
	r := newOSRig(t)
	first, err := r.send(r.asClient(), "once", "cmid-aaaa-1")
	if err != nil || first.Replayed {
		t.Fatalf("first: %v %+v", err, first)
	}
	for i := 0; i < 2; i++ {
		if _, err := r.send(r.asClient(), "fill", ""); err != nil {
			t.Fatal(err)
		}
	}
	// The cap is reached; a new message is refused, the replay is not.
	if _, err := r.send(r.asClient(), "new", "cmid-aaaa-2"); code(t, err) != apperror.CodeOffSessionLimit {
		t.Fatalf("new: %v", err)
	}
	for i := 0; i < 5; i++ {
		res, err := r.send(r.asClient(), "once", "cmid-aaaa-1")
		if err != nil || !res.Replayed || res.Message.ID != first.Message.ID {
			t.Fatalf("replay %d: %v %+v", i, err, res)
		}
		if res.Allowance == nil || res.Allowance.Used != 3 {
			t.Fatalf("replay allowance %+v", res.Allowance)
		}
	}
	if r.repo.offSessionRows() != 3 {
		t.Fatalf("rows %d", r.repo.offSessionRows())
	}
	// Even with the feature switched off and the recipient gone.
	r.cfg.Enabled = false
	r.users.users[r.psychic] = domain.User{ID: r.psychic, Status: domain.UserDeleted}
	if res, err := r.send(r.asClient(), "once", "cmid-aaaa-1"); err != nil || !res.Replayed {
		t.Fatalf("replay after disable: %v", err)
	}
	// A bad identifier is a validation error, never a replay.
	if _, err := r.send(r.asClient(), "x", "no"); code(t, err) != apperror.CodeValidation {
		t.Fatalf("bad id: %v", err)
	}
}

func TestConcurrentSendsNeverExceedTheCap(t *testing.T) {
	r := newOSRig(t)
	r.cfg.ClientDailyLimit = 7
	var ok, limited atomic.Int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := r.send(r.asClient(), "burst", "burst-msg-"+uuid.NewString()[:8])
			switch {
			case err == nil:
				ok.Add(1)
			case code(t, err) == apperror.CodeOffSessionLimit:
				limited.Add(1)
			default:
				t.Errorf("unexpected: %v", err)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	if ok.Load() != 7 || limited.Load() != 33 || r.repo.offSessionRows() != 7 {
		t.Fatalf("ok=%d limited=%d rows=%d", ok.Load(), limited.Load(), r.repo.offSessionRows())
	}
}

func TestConcurrentIdenticalClientMsgIDStoresOnce(t *testing.T) {
	r := newOSRig(t)
	var wg sync.WaitGroup
	var replays atomic.Int64
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := r.send(r.asClient(), "same", "same-msg-id")
			if err != nil {
				t.Errorf("send: %v", err)
				return
			}
			if res.Replayed {
				replays.Add(1)
			}
		}()
	}
	wg.Wait()
	if r.repo.offSessionRows() != 1 || replays.Load() != 9 {
		t.Fatalf("rows=%d replays=%d", r.repo.offSessionRows(), replays.Load())
	}
}

func TestAllowanceComputation(t *testing.T) {
	r := newOSRig(t)
	ctx := context.Background()
	a, err := r.svc.OffSessionAllowanceFor(ctx, r.asClient(), r.conv)
	if err != nil || !a.Enabled || !a.Eligible || a.Live || a.Unlimited || a.Limit != 3 || a.Used != 0 || a.Remaining != 3 || a.ResetsAt != nil {
		t.Fatalf("fresh: %+v %v", a, err)
	}
	if _, err := r.send(r.asClient(), "x", ""); err != nil {
		t.Fatal(err)
	}
	a, _ = r.svc.OffSessionAllowanceFor(ctx, r.asClient(), r.conv)
	if a.Used != 1 || a.Remaining != 2 || a.ResetsAt == nil || !a.ResetsAt.Equal(r.clk.Now().Add(24*time.Hour)) {
		t.Fatalf("after one: %+v", a)
	}
	// The advisor has an allowance of its own.
	if a, _ = r.svc.OffSessionAllowanceFor(ctx, r.asPsychic(), r.conv); a.Limit != 5 || a.Used != 0 {
		t.Fatalf("advisor: %+v", a)
	}
	r.free = true
	if a, _ = r.svc.OffSessionAllowanceFor(ctx, r.asClient(), r.conv); !a.Unlimited || a.Remaining != 3 || a.Used != 1 {
		t.Fatalf("free: %+v", a)
	}
	r.free = false
	r.repo.started = false
	if a, _ = r.svc.OffSessionAllowanceFor(ctx, r.asClient(), r.conv); a.Eligible {
		t.Fatalf("not eligible: %+v", a)
	}
	r.setSession(domain.SessionDisconnected)
	if a, _ = r.svc.OffSessionAllowanceFor(ctx, r.asClient(), r.conv); !a.Live {
		t.Fatalf("live: %+v", a)
	}
	r.cfg.Enabled = false
	if a, _ = r.svc.OffSessionAllowanceFor(ctx, r.asClient(), r.conv); a.Enabled || a.Limit != 0 {
		t.Fatalf("disabled: %+v", a)
	}
	// Strangers get a 404.
	if _, err := r.svc.OffSessionAllowanceFor(ctx, reqctx.Principal{UserID: uuid.New(), Role: domain.RoleClient}, r.conv); code(t, err) != apperror.CodeNotFound {
		t.Fatalf("stranger: %v", err)
	}
}

func TestLegacySessionSendRoutesOffSessionAfterTheChatEnded(t *testing.T) {
	r := newOSRig(t)
	sid := r.setSession(domain.SessionEnded)
	v, err := r.svc.SendMessage(context.Background(), r.asClient(), SendMessageInput{SessionID: sid, Body: "after the end"})
	if err != nil {
		t.Fatal(err)
	}
	if !v.OffSession || v.SessionID != nil {
		t.Fatalf("%+v", v)
	}
	if _, err := r.svc.SendMessage(context.Background(), reqctx.Principal{UserID: uuid.New(), Role: domain.RoleClient}, SendMessageInput{SessionID: sid, Body: "x"}); code(t, err) != apperror.CodeNotFound {
		t.Fatalf("stranger: %v", err)
	}
}

func TestMessageViewLabelsOffSession(t *testing.T) {
	sid := uuid.New()
	off := messageView(&domain.Message{ID: uuid.New(), Kind: domain.MessageText})
	live := messageView(&domain.Message{ID: uuid.New(), Kind: domain.MessageText, SessionID: &sid})
	sys := messageView(&domain.Message{ID: uuid.New(), Kind: domain.MessageSystem, SessionID: &sid})
	if !off.OffSession || live.OffSession || sys.OffSession {
		t.Fatalf("off=%v live=%v sys=%v", off.OffSession, live.OffSession, sys.OffSession)
	}
}

func TestOffSessionImagePrecheckRefusesBeforeDecodeAndStorage(t *testing.T) {
	for name, setup := range map[string]func(r *osRig){
		"disabled":     func(r *osRig) { r.cfg.Enabled = false },
		"not eligible": func(r *osRig) { r.repo.started = false },
		"recipient":    func(r *osRig) { r.users.users[r.psychic] = domain.User{ID: r.psychic, Status: domain.UserBanned} },
		"limit": func(r *osRig) {
			for i := 0; i < 3; i++ {
				if _, err := r.send(r.asClient(), "x", ""); err != nil {
					panic(err)
				}
			}
		},
	} {
		r := newOSRig(t)
		setup(r)
		// Garbage bytes: had the image been decoded, the error would be a
		// validation error, not the gate's.
		_, err := r.svc.SendImageToConversation(context.Background(), r.asClient(), UploadImageInput{ConversationID: r.conv, Data: []byte("not an image at all")})
		var ae *apperror.Error
		if !errors.As(err, &ae) || ae.Status != 409 && ae.Status != 429 {
			t.Fatalf("%s: want a gate refusal, got %v", name, err)
		}
		if len(r.store.Keys()) != 0 || r.atts.created.Load() != 0 {
			t.Fatalf("%s: stored something: %v", name, r.store.Keys())
		}
	}
}

func TestOffSessionImageCountsTowardTheCap(t *testing.T) {
	r := newOSRig(t)
	data := pngBytes(t)
	for i := 0; i < 2; i++ {
		res, err := r.svc.SendImageToConversation(context.Background(), r.asClient(), UploadImageInput{ConversationID: r.conv, Data: data, Caption: "pic"})
		if err != nil {
			t.Fatal(err)
		}
		if !res.Message.OffSession || res.Message.Kind != "image" || len(res.Message.Attachments) != 1 || res.Allowance.Used != i+1 {
			t.Fatalf("%+v %+v", res.Message, res.Allowance)
		}
	}
	if _, err := r.send(r.asClient(), "text", ""); err != nil { // third, text
		t.Fatal(err)
	}
	if len(r.store.Keys()) != 2 {
		t.Fatalf("objects %v", r.store.Keys())
	}
	// The cap is shared by text and images: the next image is refused and
	// nothing new is stored.
	_, err := r.svc.SendImageToConversation(context.Background(), r.asClient(), UploadImageInput{ConversationID: r.conv, Data: data})
	if code(t, err) != apperror.CodeOffSessionLimit {
		t.Fatalf("image over cap: %v", err)
	}
	if len(r.store.Keys()) != 2 {
		t.Fatalf("objects after refusal %v", r.store.Keys())
	}
	found := false
	for _, e := range r.rec.Events {
		if e.Name == "chat.off_session_message_sent" && e.Properties["kind"] == "image" {
			found = true
			if e.Properties["conversation_id"] == nil || e.Properties["role"] != "client" {
				t.Fatalf("event %+v", e.Properties)
			}
		}
	}
	if !found {
		t.Fatalf("events %v", r.rec.Names())
	}
}

func TestStoredImageIsDeletedWhenTheTransactionFails(t *testing.T) {
	r := newOSRig(t)
	r.atts.fail = errors.New("attachments table is down")
	_, err := r.svc.SendImageToConversation(context.Background(), r.asClient(), UploadImageInput{ConversationID: r.conv, Data: pngBytes(t)})
	if err == nil {
		t.Fatal("want an error")
	}
	if len(r.store.Keys()) != 0 {
		t.Fatalf("orphaned object %v", r.store.Keys())
	}
	if r.repo.offSessionRows() != 0 {
		t.Fatal("the message survived the rollback")
	}
	// A refused send after the upload (cap reached between the pre-check and
	// the lock) also removes the object.
	r2 := newOSRig(t)
	svc := r2.svc
	svc.chat = &fillOnLock{osChat: r2.repo, rig: r2}
	_, err = svc.SendImageToConversation(context.Background(), r2.asClient(), UploadImageInput{ConversationID: r2.conv, Data: pngBytes(t)})
	if code(t, err) != apperror.CodeOffSessionLimit {
		t.Fatalf("want the cap inside the tx, got %v", err)
	}
	if len(r2.store.Keys()) != 0 {
		t.Fatalf("orphaned object %v", r2.store.Keys())
	}
}

// fillOnLock simulates concurrent sends that exhaust the cap while this
// request waits for the conversation lock: the unlocked pre-check passed, the
// authoritative check inside the transaction refuses.
type fillOnLock struct {
	*osChat
	rig *osRig
}

func (f *fillOnLock) GetConversationForUpdate(ctx context.Context, id uuid.UUID) (*domain.Conversation, error) {
	f.osChat.mu.Lock()
	for i := 0; i < 3; i++ {
		sender := f.rig.client
		f.osChat.messages = append(f.osChat.messages, domain.Message{ID: uuid.New(), SenderID: &sender, Kind: domain.MessageText, CreatedAt: f.rig.clk.Now()})
	}
	f.osChat.mu.Unlock()
	return f.osChat.GetConversationForUpdate(ctx, id)
}

func TestSendingToAnUnknownConversationIs404(t *testing.T) {
	r := newOSRig(t)
	r.svc.chat = &missingConv{osChat: r.repo}
	if _, err := r.send(r.asClient(), "x", ""); code(t, err) != apperror.CodeNotFound {
		t.Fatalf("%v", err)
	}
}

type missingConv struct{ *osChat }

func (missingConv) GetConversation(context.Context, uuid.UUID) (*domain.Conversation, error) {
	return nil, repository.ErrNotFound
}
