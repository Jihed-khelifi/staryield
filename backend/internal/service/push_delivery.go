package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/push"
)

// pushLane is a priority class of push jobs. Workers always take urgent work
// first, then normal, then bulk; a full lane never blocks or evicts another.
type pushLane int

// Lanes, highest priority first.
const (
	laneUrgent pushLane = iota // chat.requested, chat.missed, chat messages, waitlist.available
	laneNormal
	laneBulk // broadcasts
	laneCount
)

func (l pushLane) String() string { return [...]string{"urgent", "normal", "bulk"}[l] }

// laneFor picks the lane of a notification kind.
func laneFor(kind string) pushLane {
	switch kind {
	case NotifyChatRequested, NotifyChatMissed, NotifyWaitlistAvailable:
		return laneUrgent
	case NotifyBroadcast:
		return laneBulk
	}
	return laneNormal
}

const (
	// pushMaxAttempts is how many times one device is tried for transient
	// failures (network error, 5xx, 429).
	pushMaxAttempts = 3
	// pushDeviceParallel bounds the devices of one user sent in parallel.
	pushDeviceParallel = 4
	pushMaxRetryAfter  = time.Hour
)

// defaultPushBackoff is the wait before attempt n+2: 1 s, then 5 s (capped
// at 30 s).
func defaultPushBackoff(attempt int) time.Duration {
	d := time.Second
	for i := 0; i < attempt; i++ {
		d *= 5
	}
	return min(d, 30*time.Second)
}

type pushJob struct {
	userID  uuid.UUID
	payload []byte
	topic   string
	urgency string
	ttl     time.Duration
	lane    pushLane
	// expires is when the push service would drop the message anyway; a job
	// older than that is discarded before sending.
	expires time.Time
	attempt int
	// only restricts a retry to one subscription (uuid.Nil: all devices).
	only uuid.UUID
}

type pushCounters struct {
	sent, retried, expired, pruned, dropped, rejected, gaveUp atomic.Int64
}

// PushStats are the push service counters (since start).
type PushStats struct {
	Sent     int64 `json:"sent"`
	Retried  int64 `json:"retried"`
	Expired  int64 `json:"expired"`
	Pruned   int64 `json:"pruned"`
	Dropped  int64 `json:"dropped"`
	Rejected int64 `json:"rejected"`
	// GaveUp counts transient failures that ran out of attempts (or were
	// abandoned at shutdown).
	GaveUp int64          `json:"gave_up"`
	Depth  map[string]int `json:"depth"`
}

// Snapshot returns the counters and the per-lane queue depth.
func (s *PushService) Snapshot() PushStats {
	st := PushStats{Sent: s.stats.sent.Load(), Retried: s.stats.retried.Load(), Expired: s.stats.expired.Load(),
		Pruned: s.stats.pruned.Load(), Dropped: s.stats.dropped.Load(), Rejected: s.stats.rejected.Load(),
		GaveUp: s.stats.gaveUp.Load(), Depth: map[string]int{}}
	for l := range s.lanes {
		st.Depth[pushLane(l).String()] = len(s.lanes[l])
	}
	return st
}

// Stats returns sent and dropped counters.
func (s *PushService) Stats() (sent, dropped int64) {
	return s.stats.sent.Load(), s.stats.dropped.Load()
}

// PushPayload is what the service worker receives.
type PushPayload struct {
	// ID is the notification id (absent for chat-message pushes): the web app
	// marks that notification read when the push is opened.
	ID    string `json:"id,omitempty"`
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	Tag   string `json:"tag"`
}

var topicReplacer = strings.NewReplacer(".", "", ":", "", "-", "")

// NotifyUser queues a push for every browser of a user (non-blocking; the
// job is dropped, with a log and a counter, when its lane is full).
func (s *PushService) NotifyUser(userID uuid.UUID, locale string, n *domain.Notification) {
	s.queueNotification(nil, userID, locale, n)
}

// NotifyUserWait is NotifyUser for broadcast fan-out: when the (bulk) lane is
// full it waits, bounded by ctx, instead of dropping, so a large broadcast
// is paced by the push workers.
func (s *PushService) NotifyUserWait(ctx context.Context, userID uuid.UUID, locale string, n *domain.Notification) {
	s.queueNotification(ctx, userID, locale, n)
}

func (s *PushService) queueNotification(wait context.Context, userID uuid.UUID, locale string, n *domain.Notification) {
	if s == nil || !s.Enabled() || n == nil {
		return
	}
	title, body, ok := mail.PushText(locale, n.Kind, n.Data)
	if !ok {
		return
	}
	tag := n.Kind
	for _, k := range []string{"session_id", "request_id", "review_id", "withdrawal_id", "purchase_id", "broadcast_id"} {
		if v, ok := n.Data[k]; ok {
			tag = n.Kind + ":" + fmt.Sprint(v)
			break
		}
	}
	// A missed request replaces the stale request on the device (and
	// collapses with it on the push service) through a shared tag.
	if n.Kind == NotifyChatRequested || n.Kind == NotifyChatMissed {
		if v, ok := n.Data["session_id"]; ok {
			tag = "chat.request:" + fmt.Sprint(v)
		}
	}
	payload, _ := json.Marshal(PushPayload{ID: n.ID.String(), Kind: n.Kind, Title: title, Body: body, URL: mail.AppURL(s.cfg.FrontendURL, locale, n.Link), Tag: tag})
	job := pushJob{userID: userID, payload: payload, topic: topicReplacer.Replace(tag), urgency: "normal", lane: laneFor(n.Kind)}
	if n.Kind == NotifyChatRequested {
		job.urgency, job.ttl = "high", s.cfg.ChatRequestTTL
	}
	s.enqueue(wait, job)
}

// QueueChatMessage queues a prebuilt push (chat message notifications) for
// every browser of a user, with high urgency on the urgent lane.
// Non-blocking, like NotifyUser.
func (s *PushService) QueueChatMessage(userID uuid.UUID, p PushPayload) {
	if s == nil || !s.Enabled() {
		return
	}
	payload, _ := json.Marshal(p)
	s.enqueue(nil, pushJob{userID: userID, payload: payload, topic: topicReplacer.Replace(p.Tag), urgency: "high", lane: laneUrgent})
}

// enqueue puts a job on its lane. With wait == nil a full lane drops the job;
// otherwise it waits for room (bounded by wait, shutdown and one minute).
func (s *PushService) enqueue(wait context.Context, job pushJob) bool {
	ttl := job.ttl
	if ttl <= 0 {
		ttl = s.cfg.TTL
	}
	if job.expires.IsZero() {
		job.expires = s.now().Add(ttl)
	}
	lane := s.lanes[job.lane]
	if wait == nil || job.lane != laneBulk {
		select {
		case lane <- job:
			return true
		default:
			s.drop(job)
			return false
		}
	}
	select {
	case lane <- job:
		return true
	case <-wait.Done():
	case <-s.stop:
	case <-time.After(time.Minute):
	}
	s.drop(job)
	return false
}

func (s *PushService) drop(job pushJob) {
	if c := s.stats.dropped.Add(1); c == 1 || c%1000 == 0 {
		s.log.Error("push: lane full, dropping", zap.Int64("dropped_total", c), zap.String("lane", job.lane.String()))
	}
}

// Start launches the delivery workers. One worker never takes bulk work, so
// a flood of broadcast pushes cannot occupy every worker while an urgent job
// waits.
func (s *PushService) Start() {
	if !s.Enabled() {
		return
	}
	for i := 0; i < s.cfg.Workers; i++ {
		s.wg.Add(1)
		go s.work(i != 0 || s.cfg.Workers == 1)
	}
}

// Stop drains the lanes and waits for the workers (bounded by ctx). Retries
// still waiting for their timer are abandoned.
func (s *PushService) Stop(ctx context.Context) {
	s.stopped.Store(true)
	s.once.Do(func() { close(s.stop) })
	s.tmu.Lock()
	for t := range s.timers {
		if t.Stop() {
			s.stats.gaveUp.Add(1)
		}
	}
	s.timers = map[*time.Timer]struct{}{}
	s.tmu.Unlock()
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		snap := s.Snapshot()
		s.log.Warn("push: shutdown before drain completed", zap.Any("queued", snap.Depth))
	}
}

// take returns the next job in strict priority order (urgent, normal, bulk
// when allowed). With block it waits for work; after Stop it keeps returning
// queued jobs until the lanes are empty.
func (s *PushService) take(allowBulk, block bool) (pushJob, bool) {
	for l := laneUrgent; l < laneCount; l++ {
		if l == laneBulk && !allowBulk {
			continue
		}
		select {
		case j := <-s.lanes[l]:
			return j, true
		default:
		}
	}
	if !block {
		return pushJob{}, false
	}
	var bulk chan pushJob // nil blocks forever: this worker skips bulk
	if allowBulk {
		bulk = s.lanes[laneBulk]
	}
	select {
	case j := <-s.lanes[laneUrgent]:
		return j, true
	case j := <-s.lanes[laneNormal]:
		return j, true
	case j := <-bulk:
		return j, true
	case <-s.stop:
		return s.take(allowBulk, false)
	}
}

func (s *PushService) work(allowBulk bool) {
	defer s.wg.Done()
	for {
		job, ok := s.take(allowBulk, true)
		if !ok {
			return
		}
		s.deliver(job)
	}
}

// Drain delivers queued jobs synchronously (tests).
func (s *PushService) Drain() {
	for {
		job, ok := s.take(true, false)
		if !ok {
			return
		}
		s.deliver(job)
	}
}

func (s *PushService) deliver(job pushJob) {
	if s.now().After(job.expires) {
		s.stats.expired.Add(1)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	subs, err := s.repo.ListActive(ctx, []uuid.UUID{job.userID})
	if err != nil {
		s.log.Warn("push: load subscriptions failed", zap.Error(err))
		return
	}
	sem := make(chan struct{}, pushDeviceParallel)
	var wg sync.WaitGroup
	for i := range subs {
		sub := subs[i]
		if job.only != uuid.Nil && sub.ID != job.only {
			continue
		}
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() { <-sem; wg.Done() }()
			s.deliverOne(ctx, job, &sub)
		}()
	}
	wg.Wait()
}

func (s *PushService) deliverOne(ctx context.Context, job pushJob, sub *domain.PushSubscription) {
	ad := pushAD(sub.EndpointHash)
	endpoint, e1 := s.sealer.Open(sub.EndpointSealed, ad)
	p256dh, e2 := s.sealer.Open(sub.P256dhSealed, ad)
	auth, e3 := s.sealer.Open(sub.AuthSealed, ad)
	if e1 != nil || e2 != nil || e3 != nil {
		_ = s.repo.SoftDelete(ctx, sub.ID)
		s.stats.pruned.Add(1)
		return
	}
	host := hostOf(endpoint)
	var res push.Result
	if until, paused := s.pausedUntil(host); paused {
		res = push.Result{Outcome: push.Throttled, RetryAfter: until.Sub(s.now())}
	} else {
		msg := push.Message{Payload: job.payload, Topic: job.topic, Urgency: job.urgency, TTL: job.ttl}
		if job.attempt > 0 {
			// A retry must not outlive the original message.
			msg.TTL = max(time.Second, job.expires.Sub(s.now()))
		}
		res = s.sender.Send(ctx, push.Subscription{Endpoint: endpoint, P256dh: p256dh, Auth: auth}, msg)
		if res.Outcome == push.Throttled {
			wait := res.RetryAfter
			if wait <= 0 {
				wait = time.Minute
			}
			s.pause(host, min(wait, pushMaxRetryAfter))
		}
	}
	switch res.Outcome {
	case push.Delivered:
		s.stats.sent.Add(1)
		warnErr(ctx, "push: mark success", s.repo.MarkSuccess(ctx, sub.ID, s.clock.Now()))
	case push.Gone:
		s.stats.pruned.Add(1)
		warnErr(ctx, "push: prune subscription", s.repo.SoftDelete(ctx, sub.ID))
	case push.Throttled, push.Transient:
		// Never counts towards removal.
		s.retryLater(job, sub.ID, res)
	default:
		s.stats.rejected.Add(1)
		n, err := s.repo.MarkFailure(ctx, sub.ID)
		if err == nil && n >= s.cfg.MaxFailures {
			s.stats.pruned.Add(1)
			warnErr(ctx, "push: drop failing subscription", s.repo.SoftDelete(ctx, sub.ID))
		}
	}
}

// retryLater schedules another attempt for one device: backoff 1 s → 30 s,
// at least Retry-After, never past the message's lifetime, at most
// pushMaxAttempts attempts in all.
func (s *PushService) retryLater(job pushJob, sub uuid.UUID, res push.Result) {
	if job.attempt+1 >= pushMaxAttempts || s.stopped.Load() {
		s.stats.gaveUp.Add(1)
		return
	}
	wait := s.backoff(job.attempt)
	if res.RetryAfter > wait {
		wait = min(res.RetryAfter, pushMaxRetryAfter)
	}
	if !s.now().Add(wait).Before(job.expires) {
		s.stats.expired.Add(1)
		return
	}
	job.attempt++
	job.only = sub
	s.stats.retried.Add(1)
	s.tmu.Lock()
	defer s.tmu.Unlock()
	var t *time.Timer
	t = time.AfterFunc(wait, func() {
		s.tmu.Lock()
		delete(s.timers, t)
		s.tmu.Unlock()
		if s.stopped.Load() || !s.enqueue(nil, job) {
			s.stats.gaveUp.Add(1)
		}
	})
	s.timers[t] = struct{}{}
}

func hostOf(endpoint string) string {
	if u, err := url.Parse(endpoint); err == nil {
		return strings.ToLower(u.Hostname())
	}
	return ""
}

func (s *PushService) pausedUntil(host string) (time.Time, bool) {
	s.pmu.Lock()
	defer s.pmu.Unlock()
	until, ok := s.paused[host]
	if !ok {
		return time.Time{}, false
	}
	if s.now().After(until) {
		delete(s.paused, host)
		return time.Time{}, false
	}
	return until, true
}

func (s *PushService) pause(host string, d time.Duration) {
	s.pmu.Lock()
	s.paused[host] = s.now().Add(d)
	s.pmu.Unlock()
	s.log.Warn("push: service throttled us; pausing", zap.String("host", host), zap.Duration("for", d))
}

// RemoveSubscription deletes the user's subscription for one browser
// endpoint (sign-out): idempotent, silent when the endpoint is unknown or
// belongs to someone else. Safe on a nil service.
func (s *PushService) RemoveSubscription(ctx context.Context, userID uuid.UUID, endpoint string) {
	if s == nil || s.repo == nil {
		return
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" || len(endpoint) > 1024 {
		return
	}
	if u, err := url.Parse(endpoint); err == nil {
		endpoint = u.String()
	}
	_, err := s.repo.DeleteForUser(ctx, userID, s.endpointHash(endpoint))
	warnErr(ctx, "push: remove subscription", err)
}

// RemoveAllForUser deletes every push subscription of a user (sessions
// ended: logout everywhere, password change or reset, deactivation, closure,
// ban). Safe on a nil service.
func (s *PushService) RemoveAllForUser(ctx context.Context, userID uuid.UUID) {
	if s == nil || s.repo == nil {
		return
	}
	_, err := s.repo.DeleteAllForUser(ctx, userID)
	warnErr(ctx, "push: remove subscriptions", err)
}
