package realtime

import (
	"context"
	"errors"
	"sort"
	"strconv"
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

// OrderedPublish is a durable event that belongs to a conversation's
// sequence. The broker delivers a conversation's events in Seq order: an
// event whose predecessor has not been published yet is held back until the
// predecessor arrives or the gap is flushed (see FlushGaps).
type OrderedPublish struct {
	ConversationID uuid.UUID
	Seq            int64
	UserIDs        []uuid.UUID
	Payload        []byte
	Now            time.Time
}

// Ordered-delivery and subscription tuning.
const (
	// GateTTL bounds how long a conversation's ordering state survives
	// without traffic. An expired gate accepts the next event as-is.
	GateTTL = 7 * 24 * time.Hour
	// GapFlushAge is how long an out-of-order event waits for its
	// predecessor before it is published anyway (clients then fill the hole
	// through the messages "since" endpoint).
	GapFlushAge = 2 * time.Second
	// SubscribeTimeout bounds how long a new connection waits for this
	// instance's subscription to its user channel to be confirmed.
	SubscribeTimeout = 2 * time.Second
)

// ErrSubscribeTimeout means the user channel subscription was not confirmed
// in time; the connection still works but may have missed events.
var ErrSubscribeTimeout = errors.New("realtime: subscription not confirmed in time")

// Broker moves events between instances and keeps a replay buffer.
type Broker interface {
	// Publish appends a durable event to the user's stream and fans it out.
	Publish(ctx context.Context, userID uuid.UUID, payload []byte) (string, error)
	// PublishOrdered publishes a conversation event to every user in seq
	// order (see OrderedPublish). It returns how many events were published
	// now (0 when the event was held back or was a duplicate).
	PublishOrdered(ctx context.Context, op OrderedPublish) (int, error)
	// FlushGaps publishes events held back for longer than cutoff allows,
	// accepting the gap. It returns how many events were published.
	FlushGaps(ctx context.Context, cutoff time.Time) (int, error)
	// PublishEphemeral fans out without persisting.
	PublishEphemeral(ctx context.Context, userID uuid.UUID, payload []byte) error
	// Broadcast fans out to every connected user (ephemeral).
	Broadcast(ctx context.Context, payload []byte) error
	// Control sends a control instruction (e.g. close connections) for a user.
	Control(ctx context.Context, userID uuid.UUID, instruction string) error
	// Replay returns durable frames after afterID. gap is true when events
	// may have been lost (trimmed/expired), so the client must resync.
	Replay(ctx context.Context, userID uuid.UUID, afterID string, limit int64) (frames []Delivery, gap bool, err error)
	// Tail returns the ID of the user's newest durable event ("" if none).
	Tail(ctx context.Context, userID uuid.UUID) (string, error)
	// Subscribe makes this instance receive the user's events. Calls are
	// reference counted (one per local connection); the first one subscribes
	// and waits up to SubscribeTimeout for confirmation.
	Subscribe(ctx context.Context, userID uuid.UUID) error
	// Unsubscribe releases one Subscribe; the last one unsubscribes.
	Unsubscribe(userID uuid.UUID)
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

// gateEmit is shared by the ordered scripts: it appends payload to each
// recipient stream and publishes it. keys alternates stream and channel.
const gateEmit = `
local function emit(payload, keys, maxlen, ttl)
  for i = 1, #keys, 2 do
    local id = redis.call('XADD', keys[i], 'MAXLEN', '~', maxlen, '*', 'p', payload)
    redis.call('PEXPIRE', keys[i], ttl)
    redis.call('PUBLISH', keys[i + 1], 'E' .. id .. '|' .. payload)
  end
end
`

// orderedScript is the per-conversation ordering gate.
//
//	KEYS[1] gate hash: last (last published seq), n (held-back count),
//	        keys (recipient keys), b<seq> (held-back payloads)
//	KEYS[2] gaps zset: conversation id scored by when its gap began
//	KEYS[3..] recipient stream/channel key pairs
//	ARGV: seq, payload, stream maxlen, stream ttl ms, now ms, gate ttl ms, conversation id
//
// An unknown gate (new conversation, or state expired/lost) accepts the
// event as the next one. Events at or below the last published seq are
// duplicates. Events past a gap are held back; publishing the missing
// predecessor releases every consecutive held-back event in order.
var orderedScript = redis.NewScript(gateEmit + `
local seq = tonumber(ARGV[1])
local keys = {}
for i = 3, #KEYS do keys[#keys + 1] = KEYS[i] end
local last = tonumber(redis.call('HGET', KEYS[1], 'last'))
if not last then last = seq - 1 end
redis.call('HSET', KEYS[1], 'keys', table.concat(keys, ' '))
redis.call('PEXPIRE', KEYS[1], ARGV[6])
if seq <= last then
  return 0
end
if seq > last + 1 then
  if redis.call('HSETNX', KEYS[1], 'b' .. seq, ARGV[2]) == 1 then
    redis.call('HINCRBY', KEYS[1], 'n', 1)
  end
  redis.call('HSET', KEYS[1], 'last', last)
  redis.call('ZADD', KEYS[2], 'NX', ARGV[5], ARGV[7])
  return 0
end
emit(ARGV[2], keys, ARGV[3], ARGV[4])
local published = 1
last = seq
while true do
  local field = 'b' .. (last + 1)
  local held = redis.call('HGET', KEYS[1], field)
  if not held then break end
  emit(held, keys, ARGV[3], ARGV[4])
  redis.call('HDEL', KEYS[1], field)
  redis.call('HINCRBY', KEYS[1], 'n', -1)
  last = last + 1
  published = published + 1
end
redis.call('HSET', KEYS[1], 'last', last)
if tonumber(redis.call('HGET', KEYS[1], 'n') or '0') > 0 then
  -- Another gap remains: restart its timer.
  redis.call('ZADD', KEYS[2], ARGV[5], ARGV[7])
else
  redis.call('HSET', KEYS[1], 'n', 0)
  redis.call('ZREM', KEYS[2], ARGV[7])
end
return published
`)

// flushScript publishes a conversation's held-back events in seq order,
// accepting the gaps, when its gap is older than the cutoff.
//
//	KEYS[1] gate hash, KEYS[2] gaps zset
//	ARGV: conversation id, cutoff ms, stream maxlen, stream ttl ms, gate ttl ms
//
// Recipient keys come from the gate (single-node Redis assumption).
var flushScript = redis.NewScript(gateEmit + `
local score = redis.call('ZSCORE', KEYS[2], ARGV[1])
if not score or tonumber(score) > tonumber(ARGV[2]) then return 0 end
local keyList = redis.call('HGET', KEYS[1], 'keys')
local all = redis.call('HGETALL', KEYS[1])
local seqs, held = {}, {}
for i = 1, #all, 2 do
  local f = all[i]
  if string.sub(f, 1, 1) == 'b' then
    local s = tonumber(string.sub(f, 2))
    seqs[#seqs + 1] = s
    held[s] = all[i + 1]
  end
end
table.sort(seqs)
local keys = {}
if keyList then
  for k in string.gmatch(keyList, '%S+') do keys[#keys + 1] = k end
end
local last = tonumber(redis.call('HGET', KEYS[1], 'last') or '0')
local published = 0
for _, s in ipairs(seqs) do
  if s > last then
    emit(held[s], keys, ARGV[3], ARGV[4])
    last = s
    published = published + 1
  end
  redis.call('HDEL', KEYS[1], 'b' .. s)
end
redis.call('HSET', KEYS[1], 'last', last, 'n', 0)
redis.call('PEXPIRE', KEYS[1], ARGV[5])
redis.call('ZREM', KEYS[2], ARGV[1])
return published
`)

// userSub is this instance's interest in one user's channel.
type userSub struct {
	refs      int             // local connections holding the subscription
	pending   int             // SUBSCRIBE commands awaiting confirmation
	confirmed bool            // Redis confirmed the subscription
	waiters   []chan struct{} // closed when confirmed
}

// RedisBroker implements Broker on Redis Streams + Pub/Sub. Each instance
// subscribes only to the channels of users connected to it (plus the
// broadcast channel), so fan-out traffic scales with local connections
// rather than with every user on the platform.
type RedisBroker struct {
	client    *redis.Client
	prefix    string
	maxLen    int64
	streamTTL time.Duration
	log       *zap.Logger

	mu    sync.Mutex
	users map[uuid.UUID]*userSub
	ps    *redis.PubSub // active subscription connection (nil while down)
	runs  int
}

// NewRedisBroker returns a Redis broker.
func NewRedisBroker(client *redis.Client, prefix string, maxLen int64, streamTTL time.Duration, log *zap.Logger) *RedisBroker {
	return &RedisBroker{client: client, prefix: prefix, maxLen: maxLen, streamTTL: streamTTL, log: log, users: map[uuid.UUID]*userSub{}}
}

func (b *RedisBroker) streamKey(userID uuid.UUID) string { return b.prefix + "rt:s:" + userID.String() }
func (b *RedisBroker) userChan(userID uuid.UUID) string  { return b.prefix + "rt:u:" + userID.String() }
func (b *RedisBroker) broadcastChan() string             { return b.prefix + "rt:bc" }
func (b *RedisBroker) gateKey(conv uuid.UUID) string     { return b.prefix + "rt:g:" + conv.String() }
func (b *RedisBroker) gapsKey() string                   { return b.prefix + "rt:gaps" }

// Publish implements Broker.
func (b *RedisBroker) Publish(ctx context.Context, userID uuid.UUID, payload []byte) (string, error) {
	return publishScript.Run(ctx, b.client, []string{b.streamKey(userID), b.userChan(userID)},
		b.maxLen, string(payload), b.streamTTL.Milliseconds()).Text()
}

// PublishOrdered implements Broker.
func (b *RedisBroker) PublishOrdered(ctx context.Context, op OrderedPublish) (int, error) {
	if op.Seq <= 0 {
		return 0, errors.New("realtime: ordered publish needs a positive seq")
	}
	keys := make([]string, 0, 2+2*len(op.UserIDs))
	keys = append(keys, b.gateKey(op.ConversationID), b.gapsKey())
	for _, u := range op.UserIDs {
		keys = append(keys, b.streamKey(u), b.userChan(u))
	}
	n, err := orderedScript.Run(ctx, b.client, keys, op.Seq, string(op.Payload), b.maxLen, b.streamTTL.Milliseconds(),
		op.Now.UnixMilli(), GateTTL.Milliseconds(), op.ConversationID.String()).Int()
	return n, err
}

// FlushGaps implements Broker.
func (b *RedisBroker) FlushGaps(ctx context.Context, cutoff time.Time) (int, error) {
	ids, err := b.client.ZRangeByScore(ctx, b.gapsKey(), &redis.ZRangeBy{
		Min: "-inf", Max: strconv.FormatInt(cutoff.UnixMilli(), 10), Count: 200,
	}).Result()
	if err != nil {
		return 0, err
	}
	total := 0
	for _, id := range ids {
		conv, err := uuid.Parse(id)
		if err != nil {
			b.client.ZRem(ctx, b.gapsKey(), id)
			continue
		}
		n, err := flushScript.Run(ctx, b.client, []string{b.gateKey(conv), b.gapsKey()}, id, cutoff.UnixMilli(),
			b.maxLen, b.streamTTL.Milliseconds(), GateTTL.Milliseconds()).Int()
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
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

// Tail implements Broker.
func (b *RedisBroker) Tail(ctx context.Context, userID uuid.UUID) (string, error) {
	entries, err := b.client.XRevRangeN(ctx, b.streamKey(userID), "+", "-", 1).Result()
	if err != nil || len(entries) == 0 {
		return "", err
	}
	return entries[0].ID, nil
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

// Subscribe implements Broker.
func (b *RedisBroker) Subscribe(ctx context.Context, userID uuid.UUID) error {
	b.mu.Lock()
	st := b.users[userID]
	if st == nil {
		st = &userSub{}
		b.users[userID] = st
	}
	st.refs++
	if st.refs == 1 {
		st.confirmed = false
		// Commands are written under b.mu, so SUBSCRIBE/UNSUBSCRIBE for a
		// user reach Redis in the order of the reference-count transitions.
		if b.ps != nil {
			st.pending++
			b.command(ctx, true, userID)
		}
	}
	if st.confirmed {
		b.mu.Unlock()
		return nil
	}
	w := make(chan struct{})
	st.waiters = append(st.waiters, w)
	b.mu.Unlock()

	t := time.NewTimer(SubscribeTimeout)
	defer t.Stop()
	select {
	case <-w:
		return nil
	case <-t.C:
		return ErrSubscribeTimeout
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Unsubscribe implements Broker.
func (b *RedisBroker) Unsubscribe(userID uuid.UUID) {
	b.mu.Lock()
	defer b.mu.Unlock()
	st := b.users[userID]
	if st == nil || st.refs == 0 {
		return
	}
	st.refs--
	if st.refs > 0 {
		return
	}
	st.confirmed = false
	for _, w := range st.waiters {
		close(w) // only timed-out callers remain; release them
	}
	st.waiters = nil
	if b.ps != nil {
		b.command(context.Background(), false, userID)
	}
	if st.pending == 0 {
		delete(b.users, userID)
	}
}

// command writes SUBSCRIBE/UNSUBSCRIBE for a user channel. Caller holds
// b.mu. Failures are logged: go-redis remembers the channel set and
// re-subscribes after reconnecting, and a restarted Run subscribes every
// referenced user again.
func (b *RedisBroker) command(ctx context.Context, subscribe bool, userID uuid.UUID) {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	var err error
	if subscribe {
		err = b.ps.Subscribe(wctx, b.userChan(userID))
	} else {
		err = b.ps.Unsubscribe(wctx, b.userChan(userID))
	}
	if err != nil {
		b.log.Warn("realtime broker: (un)subscribe failed; will retry on reconnect", zap.Error(err), zap.Bool("subscribe", subscribe))
	}
}

// confirm records a subscription confirmation for a user channel.
func (b *RedisBroker) confirm(userID uuid.UUID) {
	b.mu.Lock()
	defer b.mu.Unlock()
	st := b.users[userID]
	if st == nil {
		return
	}
	if st.pending > 0 {
		st.pending--
	}
	if st.pending == 0 && st.refs > 0 && !st.confirmed {
		st.confirmed = true
		for _, w := range st.waiters {
			close(w)
		}
		st.waiters = nil
	}
	if st.refs == 0 && st.pending == 0 {
		delete(b.users, userID)
	}
}

// SubscribedUsers returns how many users this instance subscribes to
// (observability and tests).
func (b *RedisBroker) SubscribedUsers() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := 0
	for _, st := range b.users {
		if st.refs > 0 {
			n++
		}
	}
	return n
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
	ps := b.client.Subscribe(ctx)
	defer ps.Close()

	// Install the connection and subscribe the broadcast channel plus every
	// user currently connected here, atomically with respect to Subscribe
	// and Unsubscribe.
	b.mu.Lock()
	b.ps = ps
	restarted := b.runs > 0
	b.runs++
	channels := []string{b.broadcastChan()}
	for id, st := range b.users {
		if st.refs == 0 {
			delete(b.users, id)
			continue
		}
		st.pending, st.confirmed = 1, false
		channels = append(channels, b.userChan(id))
	}
	err := ps.Subscribe(ctx, channels...)
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		if b.ps == ps {
			b.ps = nil
		}
		b.mu.Unlock()
	}()
	if err != nil {
		return err
	}
	b.log.Info("realtime broker subscribing", zap.Int("users", len(channels)-1))

	// ChannelWithSubscriptions surfaces re-subscriptions after go-redis
	// transparently reconnects; Pub/Sub messages published during an outage
	// are lost, so connected clients are told to resync from the API.
	ch := ps.ChannelWithSubscriptions(redis.WithChannelSize(4096))
	userPrefix := b.prefix + "rt:u:"
	broadcastConfirmations := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		case raw, ok := <-ch:
			if !ok {
				return errors.New("realtime: pubsub channel closed")
			}
			if sub, isSub := raw.(*redis.Subscription); isSub {
				if sub.Kind != "subscribe" {
					continue
				}
				if sub.Channel == b.broadcastChan() {
					broadcastConfirmations++
					// The first confirmation of the first run is the initial
					// subscription; any other means events may have been lost.
					if broadcastConfirmations > 1 || restarted {
						b.log.Warn("realtime broker resubscribed; requesting client resync")
						deliver(Delivery{Broadcast: true, Control: ControlResync})
					}
					continue
				}
				if strings.HasPrefix(sub.Channel, userPrefix) {
					if id, err := uuid.Parse(strings.TrimPrefix(sub.Channel, userPrefix)); err == nil {
						b.confirm(id)
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

// memGate is the in-memory ordering state of one conversation.
type memGate struct {
	last     int64
	held     map[int64][]byte
	users    []uuid.UUID
	gapSince time.Time
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

	refs  map[uuid.UUID]int
	gmu   sync.Mutex // serialises ordered publishing (held across Publish)
	gates map[uuid.UUID]*memGate
}

// NewMemoryBroker returns an in-memory broker keeping maxLen events per user.
func NewMemoryBroker(maxLen int) *MemoryBroker {
	return &MemoryBroker{streams: map[uuid.UUID][]Delivery{}, maxLen: maxLen, now: time.Now,
		refs: map[uuid.UUID]int{}, gates: map[uuid.UUID]*memGate{}}
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

func (b *MemoryBroker) emitAll(ctx context.Context, users []uuid.UUID, payload []byte) {
	for _, u := range users {
		_, _ = b.Publish(ctx, u, payload)
	}
}

// PublishOrdered implements Broker with the same semantics as the Redis gate.
func (b *MemoryBroker) PublishOrdered(ctx context.Context, op OrderedPublish) (int, error) {
	if op.Seq <= 0 {
		return 0, errors.New("realtime: ordered publish needs a positive seq")
	}
	b.gmu.Lock()
	defer b.gmu.Unlock()
	g := b.gates[op.ConversationID]
	if g == nil {
		g = &memGate{last: op.Seq - 1, held: map[int64][]byte{}}
		b.gates[op.ConversationID] = g
	}
	g.users = op.UserIDs
	if op.Seq <= g.last {
		return 0, nil
	}
	if op.Seq > g.last+1 {
		if _, dup := g.held[op.Seq]; !dup {
			g.held[op.Seq] = op.Payload
		}
		if g.gapSince.IsZero() {
			g.gapSince = op.Now
		}
		return 0, nil
	}
	b.emitAll(ctx, g.users, op.Payload)
	published := 1
	g.last = op.Seq
	for {
		p, ok := g.held[g.last+1]
		if !ok {
			break
		}
		b.emitAll(ctx, g.users, p)
		delete(g.held, g.last+1)
		g.last++
		published++
	}
	if len(g.held) > 0 {
		g.gapSince = op.Now
	} else {
		g.gapSince = time.Time{}
	}
	return published, nil
}

// FlushGaps implements Broker.
func (b *MemoryBroker) FlushGaps(ctx context.Context, cutoff time.Time) (int, error) {
	b.gmu.Lock()
	defer b.gmu.Unlock()
	total := 0
	for _, g := range b.gates {
		if g.gapSince.IsZero() || g.gapSince.After(cutoff) {
			continue
		}
		seqs := make([]int64, 0, len(g.held))
		for s := range g.held {
			seqs = append(seqs, s)
		}
		sort.Slice(seqs, func(i, j int) bool { return seqs[i] < seqs[j] })
		for _, s := range seqs {
			if s > g.last {
				b.emitAll(ctx, g.users, g.held[s])
				g.last = s
				total++
			}
			delete(g.held, s)
		}
		g.gapSince = time.Time{}
	}
	return total, nil
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

// Tail implements Broker.
func (b *MemoryBroker) Tail(_ context.Context, userID uuid.UUID) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if s := b.streams[userID]; len(s) > 0 {
		return s[len(s)-1].ID, nil
	}
	return "", nil
}

// Subscribe implements Broker. The in-process broker delivers every event
// to its subscribers, so this only keeps the reference count.
func (b *MemoryBroker) Subscribe(_ context.Context, userID uuid.UUID) error {
	b.mu.Lock()
	b.refs[userID]++
	b.mu.Unlock()
	return nil
}

// Unsubscribe implements Broker.
func (b *MemoryBroker) Unsubscribe(userID uuid.UUID) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.refs[userID] <= 1 {
		delete(b.refs, userID)
		return
	}
	b.refs[userID]--
}

// SubscribedUsers returns how many users hold a subscription.
func (b *MemoryBroker) SubscribedUsers() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.refs)
}

// Run implements Broker.
func (b *MemoryBroker) Run(ctx context.Context, deliver func(Delivery)) error {
	b.mu.Lock()
	b.subs = append(b.subs, deliver)
	b.mu.Unlock()
	<-ctx.Done()
	return nil
}

// SubscribeAll registers deliver immediately (tests that do not call Run).
func (b *MemoryBroker) SubscribeAll(deliver func(Delivery)) {
	b.mu.Lock()
	b.subs = append(b.subs, deliver)
	b.mu.Unlock()
}
