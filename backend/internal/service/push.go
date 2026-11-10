package service

import (
	"context"
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
	// Workers is the number of delivery workers (default 8).
	Workers int
	// QueueSize bounds each of the three lanes (urgent, normal, bulk).
	QueueSize int
	// MaxFailures drops a subscription after that many consecutive rejected
	// sends (400, 401, 403, 413…). Network errors, 5xx and 429 never count.
	MaxFailures int
	// ChatRequestTTL is how long a chat.requested push lives on the push
	// service: the request timeout, after which the request is moot. 0 uses
	// the sender's default TTL.
	ChatRequestTTL time.Duration
	// TTL is the default message lifetime (STARYIELD_PUSH_TTL); a queued job
	// older than its TTL is discarded before sending. 0 means one hour.
	TTL time.Duration
}

// PushService manages browser subscriptions and delivers notifications to
// them asynchronously (best effort: the in-app notification is the record).
// Delivery runs on three bounded lanes served by strict priority (see
// push_delivery.go).
type PushService struct {
	repo    repository.PushRepository
	sealer  *security.Sealer
	sender  push.Sender
	clock   clock.Clock
	log     *zap.Logger
	cfg     PushSettings
	lanes   [laneCount]chan pushJob
	stop    chan struct{}
	once    sync.Once
	wg      sync.WaitGroup
	stopped atomic.Bool
	// timers are the delayed retries waiting for their timer; Stop cancels them.
	tmu    sync.Mutex
	timers map[*time.Timer]struct{}
	// now and backoff are replaced by tests.
	now     func() time.Time
	backoff func(attempt int) time.Duration
	stats   pushCounters
	// paused holds push-service hosts that answered 429, until when.
	pmu    sync.Mutex
	paused map[string]time.Time
}

// NewPushService wires Web Push; sender nil (or an empty public key)
// disables it.
func NewPushService(repo repository.PushRepository, sealer *security.Sealer, sender push.Sender, clk clock.Clock, log *zap.Logger,
	cfg PushSettings) *PushService {
	if cfg.MaxPerUser <= 0 {
		cfg.MaxPerUser = 10
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 8
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 10000
	}
	if cfg.MaxFailures <= 0 {
		cfg.MaxFailures = 10
	}
	if cfg.TTL <= 0 {
		cfg.TTL = time.Hour
	}
	s := &PushService{repo: repo, sealer: sealer, sender: sender, clock: clk, log: log, cfg: cfg,
		stop: make(chan struct{}), paused: map[string]time.Time{}, timers: map[*time.Timer]struct{}{}, now: time.Now, backoff: defaultPushBackoff}
	for i := range s.lanes {
		s.lanes[i] = make(chan pushJob, cfg.QueueSize)
	}
	return s
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
