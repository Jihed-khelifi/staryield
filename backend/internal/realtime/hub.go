package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
)

// WebSocket close codes (4000–4999 are application-defined).
const (
	CloseNormal          = 1000
	CloseGoingAway       = 1001
	CloseUnsupported     = 1003
	CloseSlowConsumer    = 4000
	CloseUnauthorized    = 4001
	ClosePolicyViolation = 4008
	CloseTooManyConns    = 4009
)

// Hub errors.
var (
	ErrTooManyConnections = errors.New("realtime: too many connections for user")
	ErrShuttingDown       = errors.New("realtime: server shutting down")
)

// outFrame is one queued frame; ID is set for durable events (SSE id: line).
type outFrame struct {
	ID   string
	Data []byte
}

// Client is one live realtime connection (WebSocket or SSE).
type Client struct {
	ID        string
	UserID    uuid.UUID
	Role      string
	Transport string
	Log       *zap.Logger

	send      chan outFrame
	done      chan struct{}
	closeOnce sync.Once
	closeCode int
	closeMsg  string

	mu        sync.Mutex
	replaying bool
	pending   []Delivery
	lastID    string
	maxPend   int
	dropped   atomic.Int64
}

func newClient(userID uuid.UUID, role, transport string, buffer int, replaying bool, log *zap.Logger) *Client {
	id := "c_" + uuid.NewString()
	return &Client{
		ID: id, UserID: userID, Role: role, Transport: transport,
		Log:       log.With(zap.String("conn_id", id), zap.String("user_id", userID.String()), zap.String("transport", transport)),
		send:      make(chan outFrame, buffer),
		done:      make(chan struct{}),
		replaying: replaying,
		maxPend:   buffer * 4,
	}
}

// Done is closed when the client is closing.
func (c *Client) Done() <-chan struct{} { return c.done }

// Close marks the client for closing with a WebSocket close code. Safe to
// call many times; only the first code is kept.
func (c *Client) Close(code int, reason string) {
	c.closeOnce.Do(func() {
		c.closeCode, c.closeMsg = code, reason
		close(c.done)
	})
}

// closeInfo returns the close code and reason.
func (c *Client) closeInfo() (int, string) {
	select {
	case <-c.done:
		return c.closeCode, c.closeMsg
	default:
		return CloseNormal, ""
	}
}

// deliver routes a broker delivery, buffering it while a replay is running.
func (c *Client) deliver(d Delivery) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.replaying {
		if len(c.pending) >= c.maxPend {
			c.Log.Warn("realtime: replay buffer overflow; closing slow client")
			c.Close(CloseSlowConsumer, "replay buffer overflow")
			return
		}
		c.pending = append(c.pending, d)
		return
	}
	c.enqueueLocked(d)
}

// enqueueLocked queues a delivery, dropping duplicates of already-sent
// durable events. Caller holds c.mu.
func (c *Client) enqueueLocked(d Delivery) {
	if d.ID != "" {
		if c.lastID != "" && !IsAfter(d.ID, c.lastID) {
			return
		}
	}
	select {
	case c.send <- outFrame{ID: d.ID, Data: d.Frame}:
		if d.ID != "" {
			c.lastID = d.ID
		}
	case <-c.done:
	default:
		c.dropped.Add(1)
		c.Log.Warn("realtime: send buffer full; closing slow client", zap.Int("buffer", cap(c.send)))
		c.Close(CloseSlowConsumer, "client too slow")
	}
}

// finishReplay sends replayed frames then the live frames buffered meanwhile,
// skipping duplicates by stream ID, and switches the client to live mode.
func (c *Client) finishReplay(replayed []Delivery) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, d := range replayed {
		c.enqueueLocked(d)
	}
	for _, d := range c.pending {
		c.enqueueLocked(d)
	}
	c.pending = nil
	c.replaying = false
}

// SendEvent queues a locally-generated event (hello, ack, error, pong).
func (c *Client) SendEvent(eventType, ref string, data any, frameErr *FrameError) {
	var raw json.RawMessage
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			c.Log.Error("realtime: marshal local event", zap.Error(err), zap.String("type", eventType))
			return
		}
		raw = b
	}
	b, err := json.Marshal(Envelope{Type: eventType, TS: time.Now().UTC(), Ref: ref, Data: raw, Err: frameErr})
	if err != nil {
		c.Log.Error("realtime: marshal envelope", zap.Error(err))
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enqueueLocked(Delivery{Frame: b})
}

// SendError queues an error event translated from err.
func (c *Client) SendError(ref string, err error) {
	c.SendEvent(EventError, ref, nil, ToFrameError(err, c.Log))
}

// ToFrameError converts an error into a safe client payload, logging
// unexpected failures.
func ToFrameError(err error, log *zap.Logger) *FrameError {
	appErr := apperror.From(err)
	if appErr.Status >= 500 {
		log.Error("realtime: frame failed", zap.Error(err), zap.String("code", string(appErr.Code)))
	} else {
		log.Debug("realtime: frame rejected", zap.String("code", string(appErr.Code)), zap.String("reason", appErr.Message))
	}
	fe := &FrameError{Code: string(appErr.Code), Message: appErr.Message, Details: appErr.Details}
	if appErr.RetryAfter > 0 {
		fe.RetryAfter = int(appErr.RetryAfter.Seconds() + 0.999)
	}
	return fe
}

// Hub tracks the connections served by this instance.
type Hub struct {
	log        *zap.Logger
	maxPerUser int

	mu       sync.RWMutex
	byUser   map[uuid.UUID]map[*Client]struct{}
	total    int
	shutdown bool
	drained  chan struct{}
}

// NewHub returns an empty hub.
func NewHub(log *zap.Logger, maxPerUser int) *Hub {
	return &Hub{log: log, maxPerUser: maxPerUser, byUser: map[uuid.UUID]map[*Client]struct{}{}}
}

// Register adds a client.
func (h *Hub) Register(c *Client) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.shutdown {
		return ErrShuttingDown
	}
	set := h.byUser[c.UserID]
	if h.maxPerUser > 0 && len(set) >= h.maxPerUser {
		return ErrTooManyConnections
	}
	if set == nil {
		set = map[*Client]struct{}{}
		h.byUser[c.UserID] = set
	}
	set[c] = struct{}{}
	h.total++
	return nil
}

// Unregister removes a client.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.byUser[c.UserID]
	if _, ok := set[c]; !ok {
		return
	}
	delete(set, c)
	if len(set) == 0 {
		delete(h.byUser, c.UserID)
	}
	h.total--
	if h.shutdown && h.total == 0 && h.drained != nil {
		close(h.drained)
		h.drained = nil
	}
}

// Count returns the number of local connections.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.total
}

// UserConnections returns the number of local connections for userID.
func (h *Hub) UserConnections(userID uuid.UUID) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.byUser[userID])
}

func (h *Hub) clientsFor(userID uuid.UUID) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	set := h.byUser[userID]
	out := make([]*Client, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	return out
}

func (h *Hub) allClients() []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]*Client, 0, h.total)
	for _, set := range h.byUser {
		for c := range set {
			out = append(out, c)
		}
	}
	return out
}

// Dispatch delivers a broker message to local clients.
func (h *Hub) Dispatch(d Delivery) {
	if d.Broadcast {
		if d.Control == ControlResync {
			for _, c := range h.allClients() {
				c.SendEvent(EventResyncRequired, "", map[string]string{"reason": "server_reconnected"}, nil)
			}
			return
		}
		for _, c := range h.allClients() {
			c.deliver(d)
		}
		return
	}
	clients := h.clientsFor(d.UserID)
	if d.Control == ControlRevoke {
		for _, c := range clients {
			c.SendEvent(EventAuthRevoked, "", map[string]string{"reason": "session_revoked"}, nil)
			c.Close(CloseUnauthorized, "session revoked")
		}
		return
	}
	for _, c := range clients {
		c.deliver(d)
	}
}

// Shutdown asks every client to reconnect elsewhere and waits until they
// are gone or ctx expires.
func (h *Hub) Shutdown(ctx context.Context) {
	h.mu.Lock()
	h.shutdown = true
	if h.total == 0 {
		h.mu.Unlock()
		return
	}
	drained := make(chan struct{})
	h.drained = drained
	h.mu.Unlock()

	for _, c := range h.allClients() {
		c.SendEvent(EventServerShutdown, "", map[string]any{"reconnect_after_ms": 500}, nil)
		c.Close(CloseGoingAway, "server restarting")
	}
	select {
	case <-drained:
	case <-ctx.Done():
		h.log.Warn("realtime: shutdown deadline reached with open connections", zap.Int("open", h.Count()))
	}
}

// Publisher is the service-facing API for emitting events.
type Publisher struct {
	broker Broker
	now    func() time.Time
}

// NewPublisher returns a publisher over broker.
func NewPublisher(broker Broker, now func() time.Time) *Publisher {
	if now == nil {
		now = time.Now
	}
	return &Publisher{broker: broker, now: now}
}

// Publish sends a durable event to each user. Failures are logged and
// returned; the database remains the source of truth and clients resync.
func (p *Publisher) Publish(ctx context.Context, eventType string, data any, userIDs ...uuid.UUID) error {
	payload, err := encode(eventType, data, p.now())
	if err != nil {
		return err
	}
	var errs []error
	for _, u := range userIDs {
		if u == uuid.Nil {
			continue
		}
		if _, err := p.broker.Publish(ctx, u, payload); err != nil {
			logger.FromContext(ctx).Error("realtime: publish failed", zap.Error(err), zap.String("type", eventType), zap.String("to", u.String()))
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Ephemeral sends a non-durable event (typing, presence) to each user.
func (p *Publisher) Ephemeral(ctx context.Context, eventType string, data any, userIDs ...uuid.UUID) error {
	payload, err := encode(eventType, data, p.now())
	if err != nil {
		return err
	}
	var errs []error
	for _, u := range userIDs {
		if err := p.broker.PublishEphemeral(ctx, u, payload); err != nil {
			logger.FromContext(ctx).Warn("realtime: ephemeral publish failed", zap.Error(err), zap.String("type", eventType))
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Broadcast sends an ephemeral event to every connected user.
func (p *Publisher) Broadcast(ctx context.Context, eventType string, data any) error {
	payload, err := encode(eventType, data, p.now())
	if err != nil {
		return err
	}
	if err := p.broker.Broadcast(ctx, payload); err != nil {
		logger.FromContext(ctx).Warn("realtime: broadcast failed", zap.Error(err), zap.String("type", eventType))
		return err
	}
	return nil
}

// Revoke closes every realtime connection of userID on all instances.
func (p *Publisher) Revoke(ctx context.Context, userID uuid.UUID) error {
	return p.broker.Control(ctx, userID, ControlRevoke)
}
