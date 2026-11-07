package service

// revisions 3.0: chat unread

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/cache"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

type unreadChatRepo struct {
	repository.ChatRepository
	mu      sync.Mutex
	totals  map[uuid.UUID][2]int64
	calls   int
	conv    *domain.Conversation
	failing bool
}

func (r *unreadChatRepo) UnreadTotals(_ context.Context, id uuid.UUID, _ bool) (int64, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.failing {
		return 0, 0, errors.New("db down")
	}
	t := r.totals[id]
	return t[0], t[1], nil
}

func (r *unreadChatRepo) GetConversation(context.Context, uuid.UUID) (*domain.Conversation, error) {
	return r.conv, nil
}

func (r *unreadChatRepo) set(id uuid.UUID, n, c int64) {
	r.mu.Lock()
	r.totals[id] = [2]int64{n, c}
	r.mu.Unlock()
}

type sentEvent struct {
	typ  string
	data map[string]any
	to   []uuid.UUID
}

type recordingEvents struct {
	NoopPublisher
	mu     sync.Mutex
	events []sentEvent
}

func (e *recordingEvents) Ephemeral(_ context.Context, typ string, data any, to ...uuid.UUID) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	m, _ := data.(map[string]any)
	e.events = append(e.events, sentEvent{typ, m, to})
	return nil
}

func (e *recordingEvents) all() []sentEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]sentEvent(nil), e.events...)
}

type unreadUsers struct{ repository.UserRepository }

func (unreadUsers) GetProfiles(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.Profile, error) {
	return map[uuid.UUID]domain.Profile{ids[0]: {Nickname: "Moonchild"}}, nil
}

func (unreadUsers) GetByIDs(_ context.Context, ids []uuid.UUID) ([]domain.User, error) {
	return []domain.User{{ID: ids[0], DisplayName: "Real Name"}}, nil
}

type unreadPsychics struct{ repository.PsychicRepository }

func (unreadPsychics) GetByUserID(_ context.Context, id uuid.UUID) (*domain.Psychic, error) {
	return &domain.Psychic{UserID: id, Name: "Ramone", Slug: "ramone"}, nil
}

type unreadContacts struct {
	repository.ContactRepository
	c repository.Contact
}

func (u *unreadContacts) Contacts(context.Context, []uuid.UUID) ([]repository.Contact, error) {
	return []repository.Contact{u.c}, nil
}

type fakePusher struct {
	mu      sync.Mutex
	enabled bool
	sent    []PushPayload
	to      []uuid.UUID
}

func (f *fakePusher) Enabled() bool { return f.enabled }
func (f *fakePusher) QueueChatMessage(id uuid.UUID, p PushPayload) {
	f.mu.Lock()
	f.sent, f.to = append(f.sent, p), append(f.to, id)
	f.mu.Unlock()
}

type unreadRig struct {
	svc      *ChatService
	repo     *unreadChatRepo
	events   *recordingEvents
	kv       *cache.Memory
	presence *realtime.MemoryPresence
	contacts *unreadContacts
	push     *fakePusher
	client   uuid.UUID
	psychic  uuid.UUID
	conv     uuid.UUID
}

func newUnreadRig(t *testing.T) *unreadRig {
	t.Helper()
	r := &unreadRig{client: uuid.New(), psychic: uuid.New(), conv: uuid.New(),
		events: &recordingEvents{}, kv: cache.NewMemory(nil), presence: realtime.NewMemoryPresence(time.Minute, nil),
		push: &fakePusher{enabled: true}}
	r.repo = &unreadChatRepo{totals: map[uuid.UUID][2]int64{}}
	r.repo.conv = &domain.Conversation{ID: r.conv, ClientID: r.client, PsychicID: r.psychic, ClientUnread: 3, PsychicUnread: 2}
	r.contacts = &unreadContacts{c: repository.Contact{UserID: r.psychic, Locale: "fr", Status: domain.UserActive, NotifyPush: true}}
	r.svc = &ChatService{chat: r.repo, users: unreadUsers{}, psychicsRepo: unreadPsychics{}, presence: r.presence, events: r.events}
	r.svc.WithUnread(r.kv, r.contacts, r.push, "https://app.test")
	return r
}

func (r *unreadRig) textFrom(sender uuid.UUID) MessageView {
	return MessageView{ID: uuid.New(), ConversationID: r.conv, SenderID: &sender, Kind: string(domain.MessageText), Body: "my secret question about my marriage"}
}

func unreadPrincipal(id uuid.UUID, role domain.Role) reqctx.Principal {
	return reqctx.Principal{UserID: id, Role: role}
}

func TestUnreadCountCacheHitMissAndInvalidation(t *testing.T) {
	r := newUnreadRig(t)
	ctx := context.Background()
	p := unreadPrincipal(r.client, domain.RoleClient)
	r.repo.set(r.client, 7, 2)

	v, err := r.svc.UnreadCount(ctx, p)
	if err != nil || v.Count != 7 || v.Conversations != 2 {
		t.Fatalf("miss: %+v %v", v, err)
	}
	// The DB changes, but the cached value is served (hit).
	r.repo.set(r.client, 9, 3)
	if v, _ = r.svc.UnreadCount(ctx, p); v.Count != 7 || r.repo.calls != 1 {
		t.Fatalf("hit: %+v calls=%d", v, r.repo.calls)
	}
	// Invalidation forces a recount.
	r.svc.invalidateUnread(ctx, r.client)
	if v, _ = r.svc.UnreadCount(ctx, p); v.Count != 9 || v.Conversations != 3 || r.repo.calls != 2 {
		t.Fatalf("after invalidation: %+v calls=%d", v, r.repo.calls)
	}
	// Other users have their own entry.
	other := unreadPrincipal(uuid.New(), domain.RoleClient)
	if v, _ = r.svc.UnreadCount(ctx, other); v.Count != 0 {
		t.Fatalf("other user: %+v", v)
	}
	// A corrupt cache entry is ignored.
	_ = r.kv.Set(ctx, unreadKey(r.client), "garbage", time.Minute)
	if v, _ = r.svc.UnreadCount(ctx, p); v.Count != 9 {
		t.Fatalf("corrupt entry: %+v", v)
	}
	// Roles other than client/advisor are refused; database errors surface.
	if _, err := r.svc.UnreadCount(ctx, unreadPrincipal(uuid.New(), domain.RoleAdmin)); err == nil {
		t.Fatal("admin allowed")
	}
	r.repo.failing = true
	r.svc.invalidateUnread(ctx, r.client)
	if _, err := r.svc.UnreadCount(ctx, p); err == nil {
		t.Fatal("db error swallowed")
	}
}

func TestUnreadCountWorksWithoutCache(t *testing.T) {
	r := newUnreadRig(t)
	r.svc.unread = nil
	r.repo.set(r.psychic, 4, 1)
	v, err := r.svc.UnreadCount(context.Background(), unreadPrincipal(r.psychic, domain.RolePsychic))
	if err != nil || v.Count != 4 {
		t.Fatalf("%+v %v", v, err)
	}
	// Invalidation and storage hooks are no-ops without the feature wired.
	r.svc.invalidateUnread(context.Background(), r.psychic)
	r.svc.onMessageStored(context.Background(), r.textFrom(r.client), r.client, r.psychic)
	r.svc.FlushUnread()
}

func TestMessageStoredPublishTargets(t *testing.T) {
	ctx := context.Background()

	// A client's message goes to the advisor only.
	r := newUnreadRig(t)
	r.repo.set(r.psychic, 5, 2)
	r.svc.onMessageStored(ctx, r.textFrom(r.client), r.client, r.psychic)
	r.svc.FlushUnread()
	evs := r.events.all()
	// The advisor (recipient) first; the sender's own total may have dropped
	// (nothing cached for them, so their event is sent too).
	if len(evs) != 2 || evs[0].typ != realtime.EventChatUnread || len(evs[0].to) != 1 || evs[0].to[0] != r.psychic || evs[1].to[0] != r.client {
		t.Fatalf("client message targets: %+v", evs)
	}
	d := evs[0].data
	if d["count"] != int64(5) || d["conversations"] != int64(2) || d["conversation_id"] != r.conv || d["conversation_unread"] != int64(2) {
		t.Fatalf("payload: %+v", d)
	}

	// An advisor's message goes to the client; the advisor's event is
	// skipped when their cached total did not change.
	r = newUnreadRig(t)
	_ = r.kv.Set(ctx, unreadKey(r.psychic), "0:0", time.Minute)
	r.svc.onMessageStored(ctx, r.textFrom(r.psychic), r.client, r.psychic)
	r.svc.FlushUnread()
	evs = r.events.all()
	if len(evs) != 1 || evs[0].to[0] != r.client || evs[0].data["conversation_unread"] != int64(3) {
		t.Fatalf("advisor message targets: %+v", evs)
	}
	// ... and sent when it did (the advisor read the thread by replying).
	r = newUnreadRig(t)
	_ = r.kv.Set(ctx, unreadKey(r.psychic), "4:1", time.Minute)
	r.svc.onMessageStored(ctx, r.textFrom(r.psychic), r.client, r.psychic)
	r.svc.FlushUnread()
	if evs = r.events.all(); len(evs) != 2 || evs[1].to[0] != r.psychic || evs[1].data["count"] != int64(0) {
		t.Fatalf("sender drop: %+v", evs)
	}

	// A system notice is unread for both.
	r = newUnreadRig(t)
	r.svc.onMessageStored(ctx, MessageView{ID: uuid.New(), ConversationID: r.conv, Kind: string(domain.MessageSystem), Body: "session.started"}, r.client, r.psychic)
	r.svc.FlushUnread()
	evs = r.events.all()
	if len(evs) != 2 || evs[0].to[0] != r.client || evs[1].to[0] != r.psychic {
		t.Fatalf("system targets: %+v", evs)
	}

	// A sender who is not a participant produces nothing.
	r = newUnreadRig(t)
	r.svc.onMessageStored(ctx, r.textFrom(uuid.New()), r.client, r.psychic)
	r.svc.FlushUnread()
	if len(r.events.all()) != 0 {
		t.Fatal("stranger produced events")
	}
}

func TestMessageStoredInvalidatesBothCaches(t *testing.T) {
	r := newUnreadRig(t)
	ctx := context.Background()
	_ = r.kv.Set(ctx, unreadKey(r.psychic), "1:1", time.Minute)
	_ = r.kv.Set(ctx, unreadKey(r.client), "0:0", time.Minute)
	r.svc.onMessageStored(ctx, r.textFrom(r.client), r.client, r.psychic)
	if _, ok, _ := r.kv.Get(ctx, unreadKey(r.psychic)); ok {
		t.Fatal("recipient cache survived")
	}
	if _, ok, _ := r.kv.Get(ctx, unreadKey(r.client)); ok {
		t.Fatal("sender cache survived (their own total may have dropped)")
	}
	r.svc.FlushUnread()
}

func TestAfterReadPublishesToTheReaderAndInvalidates(t *testing.T) {
	r := newUnreadRig(t)
	ctx := context.Background()
	_ = r.kv.Set(ctx, unreadKey(r.client), "9:3", time.Minute)
	r.repo.set(r.client, 6, 2)
	r.svc.afterRead(ctx, r.client, false, r.conv, 0)
	if _, ok, _ := r.kv.Get(ctx, unreadKey(r.client)); ok {
		t.Fatal("reader cache survived")
	}
	evs := r.events.all()
	if len(evs) != 1 || evs[0].to[0] != r.client || evs[0].data["count"] != int64(6) || evs[0].data["conversation_unread"] != int64(0) {
		t.Fatalf("reader event: %+v", evs)
	}
}

func TestMessagePushDecisions(t *testing.T) {
	ctx := context.Background()
	store := func(r *unreadRig, v MessageView) {
		r.svc.onMessageStored(ctx, v, r.client, r.psychic)
		r.svc.FlushUnread()
	}

	// Offline recipient, switch on: one push, localized, no message text.
	r := newUnreadRig(t)
	store(r, r.textFrom(r.client))
	if len(r.push.sent) != 1 || r.push.to[0] != r.psychic {
		t.Fatalf("offline push: %+v", r.push.sent)
	}
	p := r.push.sent[0]
	if p.Kind != "chat.message" || p.Title != "Nouveau message de Moonchild" || p.Body != "Ouvrez le salon de chat pour le lire." ||
		p.URL != "https://app.test/fr/chatroom" || p.Tag != "chat:"+r.conv.String() {
		t.Fatalf("payload: %+v", p)
	}

	// Online recipient: no push, and the throttle slot stays free.
	r = newUnreadRig(t)
	_ = r.presence.Connect(ctx, "c1", r.psychic)
	store(r, r.textFrom(r.client))
	if len(r.push.sent) != 0 {
		t.Fatal("pushed to a connected user")
	}
	_, _ = r.presence.Disconnect(ctx, "c1", r.psychic)
	store(r, r.textFrom(r.client))
	if len(r.push.sent) != 1 {
		t.Fatalf("offline after online: %d", len(r.push.sent))
	}

	// Switch off, closed account, unknown contact, push disabled.
	r = newUnreadRig(t)
	r.contacts.c.NotifyPush = false
	store(r, r.textFrom(r.client))
	r.contacts.c.NotifyPush, r.contacts.c.Status = true, domain.UserDeactivated
	store(r, r.textFrom(r.client))
	r.contacts.c.Status = domain.UserActive
	r.push.enabled = false
	store(r, r.textFrom(r.client))
	if len(r.push.sent) != 0 {
		t.Fatalf("pushed although not allowed: %+v", r.push.sent)
	}

	// System messages never push.
	r = newUnreadRig(t)
	store(r, MessageView{ID: uuid.New(), ConversationID: r.conv, Kind: string(domain.MessageSystem), Body: "session.ended"})
	if len(r.push.sent) != 0 {
		t.Fatal("system message pushed")
	}

	// A client recipient gets the advisor's public name and the reader link.
	r = newUnreadRig(t)
	r.contacts.c = repository.Contact{UserID: r.client, Locale: "es", Status: domain.UserActive, NotifyPush: true}
	store(r, r.textFrom(r.psychic))
	if len(r.push.sent) != 1 || r.push.sent[0].Title != "Nuevo mensaje de Ramone" || r.push.sent[0].URL != "https://app.test/es/chatroom?reader=ramone" {
		t.Fatalf("client push: %+v", r.push.sent)
	}
}

func TestMessagePushThrottleWindow(t *testing.T) {
	r := newUnreadRig(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		r.svc.onMessageStored(ctx, r.textFrom(r.client), r.client, r.psychic)
		r.svc.FlushUnread()
	}
	if len(r.push.sent) != 1 {
		t.Fatalf("burst produced %d pushes", len(r.push.sent))
	}
	// Another conversation (or recipient) has its own window.
	other := r.textFrom(r.client)
	other.ConversationID = uuid.New()
	r.svc.onMessageStored(ctx, other, r.client, r.psychic)
	r.svc.FlushUnread()
	if len(r.push.sent) != 2 {
		t.Fatalf("second conversation: %d", len(r.push.sent))
	}
	// The window ends with the key's TTL.
	if err := r.kv.Del(ctx, chatPushKey(r.conv, r.psychic)); err != nil {
		t.Fatal(err)
	}
	r.svc.onMessageStored(ctx, r.textFrom(r.client), r.client, r.psychic)
	r.svc.FlushUnread()
	if len(r.push.sent) != 3 {
		t.Fatalf("after the window: %d", len(r.push.sent))
	}
	if ttl, _ := r.kv.TTL(ctx, chatPushKey(r.conv, r.psychic)); ttl <= 0 || ttl > ChatPushWindow {
		t.Fatalf("throttle ttl %v", ttl)
	}
}

func TestChatMessagePushNeverCarriesTheMessageText(t *testing.T) {
	conv := uuid.New()
	for _, loc := range []string{"en", "fr", "es", "de", ""} {
		for _, name := range []string{"", "Ramone", "Anna\nBob", strings.Repeat("x", 300)} {
			p := ChatMessagePush("https://app.test", loc, name, "ramone", false, conv)
			blob := p.Kind + p.Title + p.Body + p.URL + p.Tag
			if strings.Contains(blob, "secret") || strings.Contains(p.Title, "\n") || len(p.Title) > 140 {
				t.Fatalf("%s/%q: %+v", loc, name, p)
			}
			if p.Body == "" || p.Title == "" || !strings.HasPrefix(p.URL, "https://app.test/") {
				t.Fatalf("%s/%q: %+v", loc, name, p)
			}
		}
	}
	// The slug is query-escaped; advisors never get a reader link.
	if p := ChatMessagePush("https://app.test", "en", "A", "a&b=c", false, conv); !strings.HasSuffix(p.URL, "/chatroom?reader=a%26b%3Dc") {
		t.Fatalf("slug not escaped: %s", p.URL)
	}
	if p := ChatMessagePush("https://app.test", "en", "A", "ramone", true, conv); strings.Contains(p.URL, "reader") {
		t.Fatalf("advisor link: %s", p.URL)
	}
	if p := ChatMessagePush("https://app.test", "en", "", "", false, conv); p.Title != "New message" || strings.HasSuffix(p.URL, "reader=") {
		t.Fatalf("anonymous sender: %+v", p)
	}
}
