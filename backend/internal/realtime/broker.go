package realtime

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Delivery is one fan-out message received by an instance.
type Delivery struct {
	UserID    uuid.UUID // zero for broadcasts
	Broadcast bool
	Control   string // non-empty for control messages (e.g. "revoke")
	ID        string
	Frame     []byte // client-ready JSON (ID injected)
}

// Broker moves events between instances and keeps a replay buffer.
type Broker interface {
	// Publish appends a durable event to the user's stream and fans it out.
	Publish(ctx context.Context, userID uuid.UUID, payload []byte) (string, error)
	// PublishEphemeral fans out without persisting.
	PublishEphemeral(ctx context.Context, userID uuid.UUID, payload []byte) error
	// Broadcast fans out to every connected user (ephemeral).
	Broadcast(ctx context.Context, payload []byte) error
	// Control sends a control instruction (e.g. close connections) for a user.
	Control(ctx context.Context, userID uuid.UUID, instruction string) error
	// Replay returns durable frames after afterID. gap is true when events
	// may have been lost (trimmed/expired), so the client must resync.
	Replay(ctx context.Context, userID uuid.UUID, afterID string, limit int64) (frames []Delivery, gap bool, err error)
	// Run receives deliveries until ctx is cancelled.
	Run(ctx context.Context, deliver func(Delivery)) error
}

// Message kinds on the pub/sub wire: "E<id>|<json>" event, "X<instruction>".
const (
	kindEvent   = 'E'
	kindControl = 'X'
)

// publishScript appends to the stream, bounds it, refreshes its TTL and
// publishes atomically, returning the new stream ID.
var publishScript = redis.NewScript(`
local id = redis.call('XADD', KEYS[1], 'MAXLEN', '~', ARGV[1], '*', 'p', ARGV[2])
redis.call('PEXPIRE', KEYS[1], ARGV[3])
redis.call('PUBLISH', KEYS[2], 'E' .. id .. '|' .. ARGV[2])
return id
`)

// RedisBroker implements Broker on Redis Streams + Pub/Sub.
type RedisBroker struct {
	client    *redis.Client
	prefix    string
	maxLen    int64
	streamTTL time.Duration
	log       *zap.Logger
}

// NewRedisBroker returns a Redis broker.
func NewRedisBroker(client *redis.Client, prefix string, maxLen int64, streamTTL time.Duration, log *zap.Logger) *RedisBroker {
	return &RedisBroker{client: client, prefix: prefix, maxLen: maxLen, streamTTL: streamTTL, log: log}
}

func (b *RedisBroker) streamKey(userID uuid.UUID) string { return b.prefix + "rt:s:" + userID.String() }
func (b *RedisBroker) userChan(userID uuid.UUID) string  { return b.prefix + "rt:u:" + userID.String() }
func (b *RedisBroker) broadcastChan() string             { return b.prefix + "rt:bc" }

// Publish implements Broker.
func (b *RedisBroker) Publish(ctx context.Context, userID uuid.UUID, payload []byte) (string, error) {
	return publishScript.Run(ctx, b.client, []string{b.streamKey(userID), b.userChan(userID)},
		b.maxLen, string(payload), b.streamTTL.Milliseconds()).Text()
}

// PublishEphemeral implements Broker.
func (b *RedisBroker) PublishEphemeral(ctx context.Context, userID uuid.UUID, payload []byte) error {
	return b.client.Publish(ctx, b.userChan(userID), string(kindEvent)+"|"+string(payload)).Err()
}

// Broadcast implements Broker.
func (b *RedisBroker) Broadcast(ctx context.Context, payload []byte) error {
	return b.client.Publish(ctx, b.broadcastChan(), string(kindEvent)+"|"+string(payload)).Err()
}

// Control implements Broker.
func (b *RedisBroker) Control(ctx context.Context, userID uuid.UUID, instruction string) error {
	return b.client.Publish(ctx, b.userChan(userID), string(kindControl)+instruction).Err()
}

// Replay implements Broker.
func (b *RedisBroker) Replay(ctx context.Context, userID uuid.UUID, afterID string, limit int64) ([]Delivery, bool, error) {
	if _, ok := ParseStreamID(afterID); !ok {
		return nil, true, nil
	}
	key := b.streamKey(userID)
	first, err := b.client.XRangeN(ctx, key, "-", "+", 1).Result()
	if err != nil {
		return nil, false, err
	}
	if len(first) == 0 {
		// Stream expired or never existed: nothing to replay, but events
		// older than the TTL may have been missed.
		return nil, true, nil
	}
	gap := IsAfter(first[0].ID, afterID) && !isSuccessor(afterID, first[0].ID)
	entries, err := b.client.XRangeN(ctx, key, "("+afterID, "+", limit).Result()
	if err != nil {
		return nil, false, err
	}
	if int64(len(entries)) >= limit {
		gap = true
	}
	out := make([]Delivery, 0, len(entries))
	for _, e := range entries {
		p, _ := e.Values["p"].(string)
		if p == "" {
			continue
		}
		out = append(out, Delivery{UserID: userID, ID: e.ID, Frame: withID(e.ID, []byte(p))})
	}
	return out, gap, nil
}

// isSuccessor reports whether next is the stream entry immediately after
// prev when both were created in the same millisecond. Conservative: across
// milliseconds we cannot prove continuity, so a gap is assumed only when the
// oldest retained entry is newer than the client's position.
func isSuccessor(prev, next string) bool {
	p, ok1 := ParseStreamID(prev)
	n, ok2 := ParseStreamID(next)
	return ok1 && ok2 && p.Ms == n.Ms && p.Seq+1 == n.Seq
}

// Run implements Broker. It reconnects with backoff when Redis drops.
func (b *RedisBroker) Run(ctx context.Context, deliver func(Delivery)) error {
	backoff := 250 * time.Millisecond
	for {
		err := b.runOnce(ctx, deliver)
		if ctx.Err() != nil {
			return nil
		}
		b.log.Error("realtime broker subscription lost; reconnecting", zap.Error(err), zap.Duration("backoff", backoff))
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 10*time.Second)
	}
}

func (b *RedisBroker) runOnce(ctx context.Context, deliver func(Delivery)) error {
	ps := b.client.PSubscribe(ctx, b.prefix+"rt:u:*")
	defer ps.Close()
	if err := ps.Subscribe(ctx, b.broadcastChan()); err != nil {
		return err
	}
	b.log.Info("realtime broker subscribing")
	// ChannelWithSubscriptions surfaces re-subscriptions after go-redis
	// transparently reconnects; Pub/Sub messages published during the outage
	// are lost, so connected clients are told to resync from the API.
	ch := ps.ChannelWithSubscriptions(redis.WithChannelSize(4096))
	userPrefix := b.prefix + "rt:u:"
	confirmations := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		case raw, ok := <-ch:
			if !ok {
				return errors.New("realtime: pubsub channel closed")
			}
			if sub, isSub := raw.(*redis.Subscription); isSub {
				if sub.Kind == "psubscribe" || sub.Kind == "subscribe" {
					confirmations++
					// Two confirmations (psubscribe + subscribe) belong to the
					// initial subscription; later ones mean a reconnect.
					if confirmations > 2 {
						b.log.Warn("realtime broker resubscribed after reconnect; requesting client resync")
						deliver(Delivery{Broadcast: true, Control: ControlResync})
					}
				}
				continue
			}
			msg, isMsg := raw.(*redis.Message)
			if !isMsg {
				continue
			}
			d, ok := decodeWire(msg.Payload)
			if !ok {
				b.log.Warn("realtime: malformed pubsub payload", zap.String("channel", msg.Channel))
				continue
			}
			if msg.Channel == b.broadcastChan() {
				d.Broadcast = true
			} else if strings.HasPrefix(msg.Channel, userPrefix) {
				id, err := uuid.Parse(strings.TrimPrefix(msg.Channel, userPrefix))
				if err != nil {
					continue
				}
				d.UserID = id
			} else {
				continue
			}
			deliver(d)
		}
	}
}

func decodeWire(payload string) (Delivery, bool) {
	if payload == "" {
		return Delivery{}, false
	}
	switch payload[0] {
	case kindControl:
		return Delivery{Control: payload[1:]}, true
	case kindEvent:
		id, body, ok := strings.Cut(payload[1:], "|")
		if !ok {
			return Delivery{}, false
		}
		return Delivery{ID: id, Frame: withID(id, []byte(body))}, true
	}
	return Delivery{}, false
}

// MemoryBroker is an in-process Broker for tests and single-instance use.
type MemoryBroker struct {
	mu      sync.Mutex
	streams map[uuid.UUID][]Delivery
	maxLen  int
	subs    []func(Delivery)
	lastMs  uint64
	seq     uint64
	now     func() time.Time
}

// NewMemoryBroker returns an in-memory broker keeping maxLen events per user.
func NewMemoryBroker(maxLen int) *MemoryBroker {
	return &MemoryBroker{streams: map[uuid.UUID][]Delivery{}, maxLen: maxLen, now: time.Now}
}

func (b *MemoryBroker) nextID() string {
	ms := uint64(b.now().UnixMilli())
	if ms <= b.lastMs {
		ms = b.lastMs
		b.seq++
	} else {
		b.lastMs, b.seq = ms, 0
	}
	return StreamID{Ms: ms, Seq: b.seq}.String()
}

func (b *MemoryBroker) fanout(d Delivery) {
	b.mu.Lock()
	subs := append([]func(Delivery){}, b.subs...)
	b.mu.Unlock()
	for _, s := range subs {
		s(d)
	}
}

// Publish implements Broker.
func (b *MemoryBroker) Publish(_ context.Context, userID uuid.UUID, payload []byte) (string, error) {
	b.mu.Lock()
	id := b.nextID()
	d := Delivery{UserID: userID, ID: id, Frame: withID(id, payload)}
	s := append(b.streams[userID], d)
	if b.maxLen > 0 && len(s) > b.maxLen {
		s = s[len(s)-b.maxLen:]
	}
	b.streams[userID] = s
	b.mu.Unlock()
	b.fanout(d)
	return id, nil
}

// PublishEphemeral implements Broker.
func (b *MemoryBroker) PublishEphemeral(_ context.Context, userID uuid.UUID, payload []byte) error {
	b.fanout(Delivery{UserID: userID, Frame: payload})
	return nil
}

// Broadcast implements Broker.
func (b *MemoryBroker) Broadcast(_ context.Context, payload []byte) error {
	b.fanout(Delivery{Broadcast: true, Frame: payload})
	return nil
}

// Control implements Broker.
func (b *MemoryBroker) Control(_ context.Context, userID uuid.UUID, instruction string) error {
	b.fanout(Delivery{UserID: userID, Control: instruction})
	return nil
}

// Replay implements Broker.
func (b *MemoryBroker) Replay(_ context.Context, userID uuid.UUID, afterID string, limit int64) ([]Delivery, bool, error) {
	if _, ok := ParseStreamID(afterID); !ok {
		return nil, true, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.streams[userID]
	if len(s) == 0 {
		return nil, true, nil
	}
	gap := IsAfter(s[0].ID, afterID) && !isSuccessor(afterID, s[0].ID)
	var out []Delivery
	for _, d := range s {
		if IsAfter(d.ID, afterID) {
			out = append(out, d)
			if int64(len(out)) >= limit {
				gap = true
				break
			}
		}
	}
	return out, gap, nil
}

// Run implements Broker.
func (b *MemoryBroker) Run(ctx context.Context, deliver func(Delivery)) error {
	b.mu.Lock()
	b.subs = append(b.subs, deliver)
	b.mu.Unlock()
	<-ctx.Done()
	return nil
}

// Subscribe registers deliver immediately (tests that do not call Run).
func (b *MemoryBroker) Subscribe(deliver func(Delivery)) {
	b.mu.Lock()
	b.subs = append(b.subs, deliver)
	b.mu.Unlock()
}
