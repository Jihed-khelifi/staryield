package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/push"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// PushSettings tune Web Push.
type PushSettings struct {
	// PublicKey is the VAPID public key served to browsers; empty disables
	// push (subscription endpoints answer feature_unavailable).
	PublicKey   string
	FrontendURL string
	// HashKey keys the endpoint hash.
	HashKey    string
	MaxPerUser int
	Workers    int
	QueueSize  int
	// MaxFailures drops a subscription after that many consecutive errors.
	MaxFailures int
	// ChatRequestTTL is how long a chat.requested push lives on the push
	// service: the request timeout, after which the request is moot. 0 uses
	// the sender's default TTL.
	ChatRequestTTL time.Duration
}

// PushService manages browser subscriptions and delivers notifications to
// them asynchronously (best effort: the in-app notification is the record).
type PushService struct {
	repo    repository.PushRepository
	sealer  *security.Sealer
	sender  push.Sender
	clock   clock.Clock
	log     *zap.Logger
	cfg     PushSettings
	queue   chan pushJob
	stop    chan struct{}
	once    sync.Once
	wg      sync.WaitGroup
	dropped atomic.Int64
	sent    atomic.Int64
	// paused holds push-service hosts that answered 429, until when.
	pmu    sync.Mutex
	paused map[string]time.Time
}

type pushJob struct {
	userID  uuid.UUID
	payload []byte
	topic   string
	urgency string
	ttl     time.Duration
}

// NewPushService wires Web Push; sender nil (or an empty public key)
// disables it.
func NewPushService(repo repository.PushRepository, sealer *security.Sealer, sender push.Sender, clk clock.Clock, log *zap.Logger,
	cfg PushSettings) *PushService {
	if cfg.MaxPerUser <= 0 {
		cfg.MaxPerUser = 10
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 10000
	}
	if cfg.MaxFailures <= 0 {
		cfg.MaxFailures = 10
	}
	return &PushService{repo: repo, sealer: sealer, sender: sender, clock: clk, log: log, cfg: cfg,
		queue: make(chan pushJob, cfg.QueueSize), stop: make(chan struct{}), paused: map[string]time.Time{}}
}

// Enabled reports whether push is configured.
func (s *PushService) Enabled() bool { return s.sender != nil && s.cfg.PublicKey != "" }

// PushConfigView is what browsers need to subscribe.
type PushConfigView struct {
	Enabled   bool   `json:"enabled"`
	PublicKey string `json:"public_key,omitempty"`
}

// Config returns the public push configuration.
func (s *PushService) Config() PushConfigView {
	if !s.Enabled() {
		return PushConfigView{Enabled: false}
	}
	return PushConfigView{Enabled: true, PublicKey: s.cfg.PublicKey}
}

// PushSubscriptionInput is a browser PushSubscription (toJSON()).
type PushSubscriptionInput struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
	// ExpirationTime is part of PushSubscription.toJSON(); ignored.
	ExpirationTime *int64 `json:"expirationTime"`
}

// PushSubscriptionView is a registered subscription (no endpoint or keys).
type PushSubscriptionView struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *PushService) endpointHash(endpoint string) string {
	return security.Keyed(s.cfg.HashKey, endpoint, 0)
}

func pushAD(hash string) string { return "push:" + hash }

// Subscribe registers (or refreshes) the caller's browser subscription.
func (s *PushService) Subscribe(ctx context.Context, userID uuid.UUID, in PushSubscriptionInput, userAgent string) (*PushSubscriptionView, error) {
	if !s.Enabled() {
		return nil, apperror.FeatureUnavailable("Push notifications are not available.")
	}
	endpoint, err := push.ValidateEndpoint(in.Endpoint)
	switch {
	case errors.Is(err, push.ErrUnsupported):
		return nil, apperror.Validation(map[string]string{"endpoint": "This push service is not supported."})
	case err != nil:
		return nil, apperror.Validation(map[string]string{"endpoint": "Use the push endpoint provided by your browser."})
	}
	if push.ValidateKeys(in.Keys.P256dh, in.Keys.Auth) != nil {
		return nil, apperror.Validation(map[string]string{"keys": "Invalid push subscription keys."})
	}
	hash := s.endpointHash(endpoint)
	existing, err := s.repo.Active(ctx, hash)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, apperror.Internal(err)
	}
	// Re-subscribing the same browser does not count towards the limit.
	if existing == nil || existing.UserID != userID {
		n, err := s.repo.CountActive(ctx, userID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if n >= int64(s.cfg.MaxPerUser) {
			return nil, apperror.Validation(map[string]string{"endpoint": fmt.Sprintf("You can enable notifications on at most %d devices.", s.cfg.MaxPerUser)})
		}
	}
	ad := pushAD(hash)
	now := s.clock.Now()
	row := &domain.PushSubscription{ID: domain.NewID(), UserID: userID, EndpointHash: hash, UserAgent: truncateString(CleanLine(userAgent), 256),
		SoftDeleteTimestamps: domain.SoftDeleteTimestamps{CreatedAt: now, UpdatedAt: now}}
	if row.EndpointSealed, err = s.sealer.Seal(endpoint, ad); err != nil {
		return nil, apperror.Internal(err)
	}
	if row.P256dhSealed, err = s.sealer.Seal(strings.TrimSpace(in.Keys.P256dh), ad); err != nil {
		return nil, apperror.Internal(err)
	}
	if row.AuthSealed, err = s.sealer.Seal(strings.TrimSpace(in.Keys.Auth), ad); err != nil {
		return nil, apperror.Internal(err)
	}
	if err := s.repo.Upsert(ctx, row); err != nil {
		return nil, apperror.Internal(err)
	}
	saved, err := s.repo.Active(ctx, hash)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &PushSubscriptionView{ID: saved.ID, CreatedAt: saved.CreatedAt}, nil
}

// Unsubscribe removes the caller's subscription for an endpoint. It is
// idempotent and never reveals whether the endpoint belongs to someone else.
func (s *PushService) Unsubscribe(ctx context.Context, userID uuid.UUID, endpoint string) error {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" || len(endpoint) > 1024 {
		return apperror.Validation(map[string]string{"endpoint": "Use the push endpoint provided by your browser."})
	}
	if u, err := url.Parse(endpoint); err == nil {
		endpoint = u.String()
	}
	if _, err := s.repo.DeleteForUser(ctx, userID, s.endpointHash(endpoint)); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

// PushPayload is what the service worker receives.
type PushPayload struct {
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	Tag   string `json:"tag"`
}

// NotifyUser queues a push for every browser of a user (non-blocking; the
// job is dropped, with a log, when the queue is full).
func (s *PushService) NotifyUser(userID uuid.UUID, locale string, n *domain.Notification) {
	if !s.Enabled() || n == nil {
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
	payload, _ := json.Marshal(PushPayload{Kind: n.Kind, Title: title, Body: body, URL: mail.AppURL(s.cfg.FrontendURL, locale, n.Link), Tag: tag})
	urgency := "normal"
	var ttl time.Duration
	if n.Kind == NotifyChatRequested {
		urgency, ttl = "high", s.cfg.ChatRequestTTL
	}
	select {
	case s.queue <- pushJob{userID: userID, payload: payload, topic: strings.NewReplacer(".", "", ":", "", "-", "").Replace(tag), urgency: urgency, ttl: ttl}:
	default:
		if c := s.dropped.Add(1); c == 1 || c%1000 == 0 {
			s.log.Error("push: queue full, dropping", zap.Int64("dropped_total", c))
		}
	}
}

// Start launches the delivery workers.
func (s *PushService) Start() {
	if !s.Enabled() {
		return
	}
	for i := 0; i < s.cfg.Workers; i++ {
		s.wg.Add(1)
		go s.work()
	}
}

// Stop drains the queue and waits for the workers (bounded by ctx).
func (s *PushService) Stop(ctx context.Context) {
	s.once.Do(func() { close(s.stop) })
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		s.log.Warn("push: shutdown before drain completed", zap.Int("queued", len(s.queue)))
	}
}

// Stats returns sent and dropped counters.
func (s *PushService) Stats() (sent, dropped int64) { return s.sent.Load(), s.dropped.Load() }

func (s *PushService) work() {
	defer s.wg.Done()
	for {
		select {
		case job := <-s.queue:
			s.deliver(job)
		case <-s.stop:
			for {
				select {
				case job := <-s.queue:
					s.deliver(job)
				default:
					return
				}
			}
		}
	}
}

// Drain delivers queued jobs synchronously (tests).
func (s *PushService) Drain() {
	for {
		select {
		case job := <-s.queue:
			s.deliver(job)
		default:
			return
		}
	}
}

func (s *PushService) deliver(job pushJob) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	subs, err := s.repo.ListActive(ctx, []uuid.UUID{job.userID})
	if err != nil {
		s.log.Warn("push: load subscriptions failed", zap.Error(err))
		return
	}
	for i := range subs {
		sub := &subs[i]
		ad := pushAD(sub.EndpointHash)
		endpoint, e1 := s.sealer.Open(sub.EndpointSealed, ad)
		p256dh, e2 := s.sealer.Open(sub.P256dhSealed, ad)
		auth, e3 := s.sealer.Open(sub.AuthSealed, ad)
		if e1 != nil || e2 != nil || e3 != nil {
			_ = s.repo.SoftDelete(ctx, sub.ID)
			continue
		}
		host := hostOf(endpoint)
		if s.isPaused(host) {
			continue
		}
		res := s.sender.Send(ctx, push.Subscription{Endpoint: endpoint, P256dh: p256dh, Auth: auth},
			push.Message{Payload: job.payload, Topic: job.topic, Urgency: job.urgency, TTL: job.ttl})
		switch res.Outcome {
		case push.Delivered:
			s.sent.Add(1)
			warnErr(ctx, "push: mark success", s.repo.MarkSuccess(ctx, sub.ID, s.clock.Now()))
		case push.Gone:
			warnErr(ctx, "push: prune subscription", s.repo.SoftDelete(ctx, sub.ID))
		case push.Throttled:
			wait := res.RetryAfter
			if wait <= 0 {
				wait = time.Minute
			}
			s.pause(host, min(wait, time.Hour))
		default:
			n, err := s.repo.MarkFailure(ctx, sub.ID)
			if err == nil && n >= s.cfg.MaxFailures {
				warnErr(ctx, "push: drop failing subscription", s.repo.SoftDelete(ctx, sub.ID))
			}
		}
	}
}

func hostOf(endpoint string) string {
	if u, err := url.Parse(endpoint); err == nil {
		return strings.ToLower(u.Hostname())
	}
	return ""
}

func (s *PushService) isPaused(host string) bool {
	s.pmu.Lock()
	defer s.pmu.Unlock()
	until, ok := s.paused[host]
	if !ok {
		return false
	}
	if s.clock.Now().After(until) {
		delete(s.paused, host)
		return false
	}
	return true
}

func (s *PushService) pause(host string, d time.Duration) {
	s.pmu.Lock()
	s.paused[host] = s.clock.Now().Add(d)
	s.pmu.Unlock()
	s.log.Warn("push: service throttled us; pausing", zap.String("host", host), zap.Duration("for", d))
}

// revisions 3.0: chat unread

// QueueChatMessage queues a prebuilt push (chat message notifications) for
// every browser of a user, with high urgency. Non-blocking, like NotifyUser.
func (s *PushService) QueueChatMessage(userID uuid.UUID, p PushPayload) {
	if !s.Enabled() {
		return
	}
	payload, _ := json.Marshal(p)
	select {
	case s.queue <- pushJob{userID: userID, payload: payload, topic: strings.NewReplacer(".", "", ":", "", "-", "").Replace(p.Tag), urgency: "high"}:
	default:
		if c := s.dropped.Add(1); c == 1 || c%1000 == 0 {
			s.log.Error("push: queue full, dropping", zap.Int64("dropped_total", c))
		}
	}
}
