package service

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/push"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// fakePushRepo keeps subscriptions in memory with the failure-counter
// semantics of the SQL repository.
type fakePushRepo struct {
	repository.PushRepository
	mu       sync.Mutex
	subs     map[uuid.UUID]*domain.PushSubscription // live subscriptions
	byUser   map[uuid.UUID][]uuid.UUID
	deletes  int
	successN int
}

func newFakePushRepo() *fakePushRepo {
	return &fakePushRepo{subs: map[uuid.UUID]*domain.PushSubscription{}, byUser: map[uuid.UUID][]uuid.UUID{}}
}

func (r *fakePushRepo) add(s *domain.PushSubscription) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subs[s.ID] = s
	r.byUser[s.UserID] = append(r.byUser[s.UserID], s.ID)
}

func (r *fakePushRepo) ListActive(_ context.Context, users []uuid.UUID) ([]domain.PushSubscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.PushSubscription
	for _, u := range users {
		for _, id := range r.byUser[u] {
			if s, ok := r.subs[id]; ok {
				out = append(out, *s)
			}
		}
	}
	return out, nil
}

func (r *fakePushRepo) MarkSuccess(_ context.Context, id uuid.UUID, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.subs[id]; ok {
		s.FailureCount = 0
		r.successN++
	}
	return nil
}

func (r *fakePushRepo) MarkFailure(_ context.Context, id uuid.UUID) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.subs[id]; ok {
		s.FailureCount++
		return s.FailureCount, nil
	}
	return 0, nil
}

func (r *fakePushRepo) SoftDelete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.subs, id)
	r.deletes++
	return nil
}

func (r *fakePushRepo) failures(id uuid.UUID) (int, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.subs[id]
	if !ok {
		return 0, false
	}
	return s.FailureCount, true
}

// scriptedSender answers each send through fn and records it.
type scriptedSender struct {
	mu    sync.Mutex
	calls []sentCall
	fn    func(n int, sub push.Subscription, m push.Message) push.Result
}

type sentCall struct {
	endpoint string
	msg      push.Message
	at       time.Time
}

func (s *scriptedSender) Send(_ context.Context, sub push.Subscription, m push.Message) push.Result {
	s.mu.Lock()
	s.calls = append(s.calls, sentCall{sub.Endpoint, m, time.Now()})
	n := len(s.calls)
	s.mu.Unlock()
	if s.fn == nil {
		return push.Result{Outcome: push.Delivered}
	}
	return s.fn(n, sub, m)
}

func (s *scriptedSender) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func (s *scriptedSender) snapshot() []sentCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]sentCall(nil), s.calls...)
}

type pushRig struct {
	svc    *PushService
	repo   *fakePushRepo
	sender *scriptedSender
	user   uuid.UUID
}

func newPushRig(t *testing.T, cfg PushSettings, devices int) *pushRig {
	t.Helper()
	sealer, err := security.NewSealer(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	cfg.PublicKey, cfg.FrontendURL, cfg.HashKey = "k", "https://app.test", "hash-key"
	sender := &scriptedSender{}
	repo := newFakePushRepo()
	svc := NewPushService(repo, sealer, sender, clock.Real{}, zap.NewNop(), cfg)
	svc.backoff = func(int) time.Duration { return time.Millisecond }
	user := uuid.New()
	for i := 0; i < devices; i++ {
		endpoint := "https://fcm.googleapis.com/fcm/send/" + uuid.NewString()
		hash := svc.endpointHash(endpoint)
		ad := pushAD(hash)
		sub := &domain.PushSubscription{ID: uuid.New(), UserID: user, EndpointHash: hash}
		sub.EndpointSealed, _ = sealer.Seal(endpoint, ad)
		sub.P256dhSealed, _ = sealer.Seal("p", ad)
		sub.AuthSealed, _ = sealer.Seal("a", ad)
		repo.add(sub)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		svc.Stop(ctx)
	})
	return &pushRig{svc: svc, repo: repo, sender: sender, user: user}
}

func (r *pushRig) job(lane pushLane, payload string) pushJob {
	return pushJob{userID: r.user, payload: []byte(payload), lane: lane, urgency: "normal"}
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestPushLanesAreServedByStrictPriority(t *testing.T) {
	r := newPushRig(t, PushSettings{QueueSize: 10}, 1)
	for _, j := range []pushJob{r.job(laneBulk, "bulk"), r.job(laneNormal, "normal"), r.job(laneUrgent, "urgent"), r.job(laneBulk, "bulk2")} {
		r.svc.enqueue(nil, j)
	}
	var order []string
	for {
		j, ok := r.svc.take(true, false)
		if !ok {
			break
		}
		order = append(order, string(j.payload))
	}
	if len(order) != 4 || order[0] != "urgent" || order[1] != "normal" || order[2] != "bulk" || order[3] != "bulk2" {
		t.Fatalf("order: %v", order)
	}
}

func TestLaneForKinds(t *testing.T) {
	want := map[string]pushLane{NotifyChatRequested: laneUrgent, NotifyChatMissed: laneUrgent, NotifyWaitlistAvailable: laneUrgent,
		NotifyBroadcast: laneBulk, NotifyPurchase: laneNormal, NotifyReviewReceived: laneNormal, NotifyExportReady: laneNormal}
	for kind, lane := range want {
		if got := laneFor(kind); got != lane {
			t.Errorf("%s: lane %v, want %v", kind, got, lane)
		}
	}
}

// A flood of bulk jobs never delays an urgent job: one worker never takes
// bulk work and every pick point serves urgent first.
func TestBulkFloodNeverDelaysUrgent(t *testing.T) {
	r := newPushRig(t, PushSettings{QueueSize: 100, Workers: 2}, 1)
	release := make(chan struct{})
	var bulkStarted atomic.Int32
	r.sender.fn = func(_ int, _ push.Subscription, m push.Message) push.Result {
		if string(m.Payload) == "bulk" {
			bulkStarted.Add(1)
			<-release
		}
		return push.Result{Outcome: push.Delivered}
	}
	r.svc.Start()
	for i := 0; i < 50; i++ {
		r.svc.enqueue(nil, r.job(laneBulk, "bulk"))
	}
	eventually(t, "a bulk send in flight", func() bool { return bulkStarted.Load() == 1 })
	start := time.Now()
	r.svc.enqueue(nil, r.job(laneUrgent, "urgent"))
	eventually(t, "the urgent send", func() bool {
		for _, c := range r.sender.snapshot() {
			if string(c.msg.Payload) == "urgent" {
				return true
			}
		}
		return false
	})
	if time.Since(start) > time.Second {
		t.Fatalf("urgent job waited %v behind bulk", time.Since(start))
	}
	if bulkStarted.Load() != 1 {
		t.Fatalf("the bulk-free worker took bulk work: %d", bulkStarted.Load())
	}
	close(release)
}

func TestFullBulkLaneNeverBlocksOrEvictsOthers(t *testing.T) {
	r := newPushRig(t, PushSettings{QueueSize: 2}, 1)
	for i := 0; i < 5; i++ {
		r.svc.enqueue(nil, r.job(laneBulk, "bulk"))
	}
	if !r.svc.enqueue(nil, r.job(laneNormal, "normal")) || !r.svc.enqueue(nil, r.job(laneUrgent, "urgent")) {
		t.Fatal("a full bulk lane refused another lane's job")
	}
	st := r.svc.Snapshot()
	if st.Depth["bulk"] != 2 || st.Depth["normal"] != 1 || st.Depth["urgent"] != 1 || st.Dropped != 3 {
		t.Fatalf("snapshot: %+v", st)
	}
	// Broadcast fan-out waits for room instead of dropping.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if r.svc.enqueue(ctx, r.job(laneBulk, "bulk")) {
		t.Fatal("enqueue into a full bulk lane succeeded")
	}
}

func TestExpiredJobIsDiscardedBeforeSending(t *testing.T) {
	r := newPushRig(t, PushSettings{QueueSize: 10, TTL: time.Minute}, 1)
	base := time.Now()
	r.svc.now = func() time.Time { return base }
	j := r.job(laneNormal, "stale")
	r.svc.enqueue(nil, j) // expires base+1m
	fresh := r.job(laneNormal, "fresh")
	fresh.ttl = time.Hour
	r.svc.enqueue(nil, fresh)
	r.svc.now = func() time.Time { return base.Add(2 * time.Minute) }
	r.svc.Drain()
	calls := r.sender.snapshot()
	if len(calls) != 1 || string(calls[0].msg.Payload) != "fresh" {
		t.Fatalf("sent: %+v", calls)
	}
	if st := r.svc.Snapshot(); st.Expired != 1 || st.Sent != 1 {
		t.Fatalf("stats: %+v", st)
	}
}

func TestOutcomesRemoveCountOrRetry(t *testing.T) {
	cases := []struct {
		name       string
		result     push.Result
		wantGone   bool
		wantFails  int
		wantRetry  bool
		wantReject bool
	}{
		{"delivered", push.Result{Outcome: push.Delivered, Status: 201}, false, 0, false, false},
		{"gone 410", push.Result{Outcome: push.Gone, Status: 410}, true, 0, false, false},
		{"gone 404", push.Result{Outcome: push.Gone, Status: 404}, true, 0, false, false},
		{"network error", push.Result{Outcome: push.Transient}, false, 0, true, false},
		{"server error", push.Result{Outcome: push.Transient, Status: 503}, false, 0, true, false},
		{"throttled", push.Result{Outcome: push.Throttled, Status: 429}, false, 0, true, false},
		{"bad request", push.Result{Outcome: push.Failed, Status: 400}, false, 1, false, true},
		{"unauthorized", push.Result{Outcome: push.Failed, Status: 401}, false, 1, false, true},
		{"forbidden", push.Result{Outcome: push.Failed, Status: 403}, false, 1, false, true},
		{"too large", push.Result{Outcome: push.Failed, Status: 413}, false, 1, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newPushRig(t, PushSettings{QueueSize: 10, MaxFailures: 10}, 1)
			r.sender.fn = func(int, push.Subscription, push.Message) push.Result { return c.result }
			subID := r.repo.byUser[r.user][0]
			r.svc.enqueue(nil, r.job(laneNormal, "x"))
			r.svc.Drain()
			fails, alive := r.repo.failures(subID)
			if alive == c.wantGone || (alive && fails != c.wantFails) {
				t.Fatalf("alive=%v failures=%d, want gone=%v failures=%d", alive, fails, c.wantGone, c.wantFails)
			}
			st := r.svc.Snapshot()
			if (st.Retried == 1) != c.wantRetry || (st.Rejected == 1) != c.wantReject {
				t.Fatalf("stats: %+v", st)
			}
		})
	}
}

func TestRejectedSendsRemoveAtLimitAndSuccessResets(t *testing.T) {
	r := newPushRig(t, PushSettings{QueueSize: 10, MaxFailures: 3}, 1)
	subID := r.repo.byUser[r.user][0]
	results := []push.Outcome{push.Failed, push.Failed, push.Delivered, push.Failed, push.Failed, push.Failed}
	var i int
	r.sender.fn = func(int, push.Subscription, push.Message) push.Result {
		o := results[i]
		i++
		return push.Result{Outcome: o}
	}
	for k := 0; k < len(results); k++ {
		r.svc.enqueue(nil, r.job(laneNormal, "x"))
		r.svc.Drain()
		fails, alive := r.repo.failures(subID)
		switch k {
		case 1:
			if !alive || fails != 2 {
				t.Fatalf("after two failures: alive=%v fails=%d", alive, fails)
			}
		case 2:
			if !alive || fails != 0 {
				t.Fatalf("success must reset the counter: alive=%v fails=%d", alive, fails)
			}
		case 4:
			if !alive || fails != 2 {
				t.Fatalf("after reset and two failures: alive=%v fails=%d", alive, fails)
			}
		case 5:
			if alive {
				t.Fatal("not removed after three consecutive rejections")
			}
		}
	}
}

// Transient failures retry (3 attempts, never more) and never count towards
// removal, however low the failure limit is.
func TestTransientFailuresRetryThreeTimesAndNeverPrune(t *testing.T) {
	r := newPushRig(t, PushSettings{QueueSize: 10, Workers: 2, MaxFailures: 1}, 1)
	subID := r.repo.byUser[r.user][0]
	r.sender.fn = func(int, push.Subscription, push.Message) push.Result {
		return push.Result{Outcome: push.Transient, Status: 503}
	}
	r.svc.Start()
	r.svc.enqueue(nil, r.job(laneNormal, "x"))
	eventually(t, "three attempts", func() bool { return r.svc.Snapshot().GaveUp == 1 })
	time.Sleep(50 * time.Millisecond)
	if n := r.sender.count(); n != 3 {
		t.Fatalf("attempts: %d, want 3", n)
	}
	if fails, alive := r.repo.failures(subID); !alive || fails != 0 {
		t.Fatalf("transient failures counted: alive=%v fails=%d", alive, fails)
	}
	if st := r.svc.Snapshot(); st.Retried != 2 || st.Rejected != 0 || st.Pruned != 0 {
		t.Fatalf("stats: %+v", st)
	}
}

func TestRetryHonoursRetryAfterAndNeverGoesPastTheTTL(t *testing.T) {
	r := newPushRig(t, PushSettings{QueueSize: 10, Workers: 2, TTL: time.Hour}, 1)
	r.sender.fn = func(n int, _ push.Subscription, _ push.Message) push.Result {
		if n == 1 {
			return push.Result{Outcome: push.Throttled, Status: 429, RetryAfter: 200 * time.Millisecond}
		}
		return push.Result{Outcome: push.Delivered}
	}
	r.svc.Start()
	r.svc.enqueue(nil, r.job(laneNormal, "x"))
	eventually(t, "the retry", func() bool { return r.sender.count() == 2 })
	calls := r.sender.snapshot()
	if gap := calls[1].at.Sub(calls[0].at); gap < 190*time.Millisecond {
		t.Fatalf("retry after %v, Retry-After was 200ms", gap)
	}
	if calls[1].msg.TTL <= 0 || calls[1].msg.TTL > time.Hour {
		t.Fatalf("retry TTL must be the remaining life: %v", calls[1].msg.TTL)
	}

	// A Retry-After beyond the message's life is not retried at all.
	r2 := newPushRig(t, PushSettings{QueueSize: 10, TTL: time.Hour}, 1)
	r2.sender.fn = func(int, push.Subscription, push.Message) push.Result {
		return push.Result{Outcome: push.Throttled, Status: 429, RetryAfter: 30 * time.Minute}
	}
	short := r2.job(laneUrgent, "short")
	short.ttl = 90 * time.Second
	r2.svc.enqueue(nil, short)
	r2.svc.Drain()
	if st := r2.svc.Snapshot(); st.Retried != 0 || st.Expired != 1 || r2.sender.count() != 1 {
		t.Fatalf("retry past the TTL: %+v sends=%d", st, r2.sender.count())
	}
}

func TestDefaultPushBackoff(t *testing.T) {
	want := []time.Duration{time.Second, 5 * time.Second, 25 * time.Second, 30 * time.Second, 30 * time.Second}
	for i, w := range want {
		if got := defaultPushBackoff(i); got != w {
			t.Errorf("attempt %d: %v, want %v", i, got, w)
		}
	}
}

func TestDevicesOfOneUserAreSentInParallel(t *testing.T) {
	r := newPushRig(t, PushSettings{QueueSize: 10}, 3)
	var inflight, peak atomic.Int32
	r.sender.fn = func(int, push.Subscription, push.Message) push.Result {
		n := inflight.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(80 * time.Millisecond)
		inflight.Add(-1)
		return push.Result{Outcome: push.Delivered}
	}
	r.svc.enqueue(nil, r.job(laneNormal, "x"))
	start := time.Now()
	r.svc.Drain()
	if peak.Load() != 3 || time.Since(start) > 200*time.Millisecond {
		t.Fatalf("peak concurrency %d in %v: devices were sent one after another", peak.Load(), time.Since(start))
	}
}

func TestPushPayloadCarriesTheNotificationId(t *testing.T) {
	r := newPushRig(t, PushSettings{QueueSize: 10}, 1)
	n := &domain.Notification{ID: uuid.New(), Kind: NotifyReviewReceived, Data: map[string]any{"review_id": "r1", "rating": 5}, Link: "/profile"}
	r.svc.NotifyUser(r.user, "en", n)
	job, ok := r.svc.take(true, false)
	var p PushPayload
	if err := json.Unmarshal(job.payload, &p); !ok || err != nil || p.ID != n.ID.String() {
		t.Fatalf("payload id: %s %v", job.payload, err)
	}
	r.svc.QueueChatMessage(r.user, PushPayload{Kind: "chat.message", Title: "t", Body: "b", URL: "/x", Tag: "chat:1"})
	job, _ = r.svc.take(true, false)
	if bytes.Contains(job.payload, []byte(`"id"`)) || job.lane != laneUrgent {
		t.Fatalf("chat message push: lane %v payload %s", job.lane, job.payload)
	}
}

func TestRemoveSubscriptionsAreNilSafe(t *testing.T) {
	var s *PushService
	s.RemoveAllForUser(context.Background(), uuid.New())
	s.RemoveSubscription(context.Background(), uuid.New(), "https://fcm.googleapis.com/x")
}
