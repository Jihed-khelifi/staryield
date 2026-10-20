package realtime

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Control instructions carried by Broker.Control / Delivery.Control.
const (
	ControlRevoke = "revoke"
	ControlResync = "resync"
)

// Presence tracks which connections are alive and which chat sessions each
// connection has joined. Entries carry an expiry score and are refreshed by
// heartbeats, so a crashed instance's connections age out on their own.
type Presence interface {
	// Connect registers a live connection for userID.
	Connect(ctx context.Context, connID string, userID uuid.UUID) error
	// Heartbeat refreshes the connection and every session it joined.
	Heartbeat(ctx context.Context, connID string, userID uuid.UUID) error
	// Disconnect removes the connection and returns the sessions it had joined.
	Disconnect(ctx context.Context, connID string, userID uuid.UUID) ([]uuid.UUID, error)
	// ConnectionOwner returns the user owning a live connection.
	ConnectionOwner(ctx context.Context, connID string) (uuid.UUID, bool, error)
	// Join marks the connection as present in a session.
	Join(ctx context.Context, connID string, userID, sessionID uuid.UUID) error
	// Leave removes the connection from a session.
	Leave(ctx context.Context, connID string, userID, sessionID uuid.UUID) error
	// InSession reports whether userID has any live connection in sessionID.
	InSession(ctx context.Context, sessionID, userID uuid.UUID) (bool, error)
	// Online reports which users have at least one live connection.
	Online(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]bool, error)
}

// RedisPresence implements Presence with sorted sets scored by expiry time.
type RedisPresence struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
	now    func() time.Time
}

// NewRedisPresence returns a Redis presence tracker.
func NewRedisPresence(client *redis.Client, prefix string, ttl time.Duration) *RedisPresence {
	return &RedisPresence{client: client, prefix: prefix, ttl: ttl, now: time.Now}
}

func (p *RedisPresence) userKey(u uuid.UUID) string      { return p.prefix + "rt:up:" + u.String() }
func (p *RedisPresence) connKey(c string) string         { return p.prefix + "rt:c:" + c }
func (p *RedisPresence) connSessionsKey(c string) string { return p.prefix + "rt:cs:" + c }
func (p *RedisPresence) sessionKey(s, u uuid.UUID) string {
	return p.prefix + "rt:sp:" + s.String() + ":" + u.String()
}

func (p *RedisPresence) expiry() float64 { return float64(p.now().Add(p.ttl).UnixMilli()) }

// Connect implements Presence.
func (p *RedisPresence) Connect(ctx context.Context, connID string, userID uuid.UUID) error {
	pipe := p.client.TxPipeline()
	pipe.Set(ctx, p.connKey(connID), userID.String(), p.ttl)
	pipe.ZAdd(ctx, p.userKey(userID), redis.Z{Score: p.expiry(), Member: connID})
	pipe.PExpire(ctx, p.userKey(userID), 2*p.ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// Heartbeat implements Presence.
func (p *RedisPresence) Heartbeat(ctx context.Context, connID string, userID uuid.UUID) error {
	sessions, err := p.client.SMembers(ctx, p.connSessionsKey(connID)).Result()
	if err != nil {
		return err
	}
	exp := p.expiry()
	pipe := p.client.TxPipeline()
	pipe.Set(ctx, p.connKey(connID), userID.String(), p.ttl)
	pipe.ZAdd(ctx, p.userKey(userID), redis.Z{Score: exp, Member: connID})
	pipe.PExpire(ctx, p.userKey(userID), 2*p.ttl)
	if len(sessions) > 0 {
		pipe.PExpire(ctx, p.connSessionsKey(connID), 2*p.ttl)
	}
	for _, s := range sessions {
		sid, err := uuid.Parse(s)
		if err != nil {
			continue
		}
		pipe.ZAdd(ctx, p.sessionKey(sid, userID), redis.Z{Score: exp, Member: connID})
		pipe.PExpire(ctx, p.sessionKey(sid, userID), 2*p.ttl)
	}
	_, err = pipe.Exec(ctx)
	return err
}

// Disconnect implements Presence.
func (p *RedisPresence) Disconnect(ctx context.Context, connID string, userID uuid.UUID) ([]uuid.UUID, error) {
	members, err := p.client.SMembers(ctx, p.connSessionsKey(connID)).Result()
	if err != nil {
		return nil, err
	}
	pipe := p.client.TxPipeline()
	pipe.Del(ctx, p.connKey(connID), p.connSessionsKey(connID))
	pipe.ZRem(ctx, p.userKey(userID), connID)
	var out []uuid.UUID
	for _, s := range members {
		sid, err := uuid.Parse(s)
		if err != nil {
			continue
		}
		pipe.ZRem(ctx, p.sessionKey(sid, userID), connID)
		out = append(out, sid)
	}
	_, err = pipe.Exec(ctx)
	return out, err
}

// ConnectionOwner implements Presence.
func (p *RedisPresence) ConnectionOwner(ctx context.Context, connID string) (uuid.UUID, bool, error) {
	v, err := p.client.Get(ctx, p.connKey(connID)).Result()
	if err == redis.Nil {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil, false, nil
	}
	return id, true, nil
}

// Join implements Presence.
func (p *RedisPresence) Join(ctx context.Context, connID string, userID, sessionID uuid.UUID) error {
	pipe := p.client.TxPipeline()
	pipe.SAdd(ctx, p.connSessionsKey(connID), sessionID.String())
	pipe.PExpire(ctx, p.connSessionsKey(connID), 2*p.ttl)
	pipe.ZAdd(ctx, p.sessionKey(sessionID, userID), redis.Z{Score: p.expiry(), Member: connID})
	pipe.PExpire(ctx, p.sessionKey(sessionID, userID), 2*p.ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// Leave implements Presence.
func (p *RedisPresence) Leave(ctx context.Context, connID string, userID, sessionID uuid.UUID) error {
	pipe := p.client.TxPipeline()
	pipe.SRem(ctx, p.connSessionsKey(connID), sessionID.String())
	pipe.ZRem(ctx, p.sessionKey(sessionID, userID), connID)
	_, err := pipe.Exec(ctx)
	return err
}

// InSession implements Presence.
func (p *RedisPresence) InSession(ctx context.Context, sessionID, userID uuid.UUID) (bool, error) {
	n, err := p.client.ZCount(ctx, p.sessionKey(sessionID, userID), strconv.FormatInt(p.now().UnixMilli(), 10), "+inf").Result()
	return n > 0, err
}

// Online implements Presence.
func (p *RedisPresence) Online(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := make(map[uuid.UUID]bool, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	now := strconv.FormatInt(p.now().UnixMilli(), 10)
	pipe := p.client.Pipeline()
	cmds := make([]*redis.IntCmd, len(userIDs))
	for i, u := range userIDs {
		cmds[i] = pipe.ZCount(ctx, p.userKey(u), now, "+inf")
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}
	for i, u := range userIDs {
		out[u] = cmds[i].Val() > 0
	}
	return out, nil
}

// MemoryPresence implements Presence in-process (tests, single instance).
type MemoryPresence struct {
	mu       sync.Mutex
	ttl      time.Duration
	now      func() time.Time
	conns    map[string]memConn
	sessions map[string]map[string]time.Time // "sid:uid" -> connID -> expiry
}

type memConn struct {
	user     uuid.UUID
	expires  time.Time
	sessions map[uuid.UUID]struct{}
}

// NewMemoryPresence returns an in-memory presence tracker.
func NewMemoryPresence(ttl time.Duration, now func() time.Time) *MemoryPresence {
	if now == nil {
		now = time.Now
	}
	return &MemoryPresence{ttl: ttl, now: now, conns: map[string]memConn{}, sessions: map[string]map[string]time.Time{}}
}

func skey(s, u uuid.UUID) string { return s.String() + ":" + u.String() }

// Connect implements Presence.
func (m *MemoryPresence) Connect(_ context.Context, connID string, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.conns[connID] = memConn{user: userID, expires: m.now().Add(m.ttl), sessions: map[uuid.UUID]struct{}{}}
	return nil
}

// Heartbeat implements Presence.
func (m *MemoryPresence) Heartbeat(_ context.Context, connID string, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.conns[connID]
	if !ok {
		c = memConn{user: userID, sessions: map[uuid.UUID]struct{}{}}
	}
	exp := m.now().Add(m.ttl)
	c.expires = exp
	m.conns[connID] = c
	for sid := range c.sessions {
		if set := m.sessions[skey(sid, userID)]; set != nil {
			set[connID] = exp
		}
	}
	return nil
}

// Disconnect implements Presence.
func (m *MemoryPresence) Disconnect(_ context.Context, connID string, userID uuid.UUID) ([]uuid.UUID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.conns[connID]
	if !ok {
		return nil, nil
	}
	delete(m.conns, connID)
	out := make([]uuid.UUID, 0, len(c.sessions))
	for sid := range c.sessions {
		delete(m.sessions[skey(sid, userID)], connID)
		out = append(out, sid)
	}
	return out, nil
}

// ConnectionOwner implements Presence.
func (m *MemoryPresence) ConnectionOwner(_ context.Context, connID string) (uuid.UUID, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.conns[connID]
	if !ok || m.now().After(c.expires) {
		return uuid.Nil, false, nil
	}
	return c.user, true, nil
}

// Join implements Presence.
func (m *MemoryPresence) Join(_ context.Context, connID string, userID, sessionID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.conns[connID]
	if !ok {
		c = memConn{user: userID, expires: m.now().Add(m.ttl), sessions: map[uuid.UUID]struct{}{}}
	}
	c.sessions[sessionID] = struct{}{}
	m.conns[connID] = c
	k := skey(sessionID, userID)
	if m.sessions[k] == nil {
		m.sessions[k] = map[string]time.Time{}
	}
	m.sessions[k][connID] = m.now().Add(m.ttl)
	return nil
}

// Leave implements Presence.
func (m *MemoryPresence) Leave(_ context.Context, connID string, userID, sessionID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.conns[connID]; ok {
		delete(c.sessions, sessionID)
	}
	delete(m.sessions[skey(sessionID, userID)], connID)
	return nil
}

// InSession implements Presence.
func (m *MemoryPresence) InSession(_ context.Context, sessionID, userID uuid.UUID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	for _, exp := range m.sessions[skey(sessionID, userID)] {
		if exp.After(now) {
			return true, nil
		}
	}
	return false, nil
}

// Online implements Presence.
func (m *MemoryPresence) Online(_ context.Context, userIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	out := make(map[uuid.UUID]bool, len(userIDs))
	want := make(map[uuid.UUID]bool, len(userIDs))
	for _, u := range userIDs {
		want[u] = true
		out[u] = false
	}
	for _, c := range m.conns {
		if want[c.user] && c.expires.After(now) {
			out[c.user] = true
		}
	}
	return out, nil
}

// TicketStore issues single-use WebSocket handshake tickets so long-lived
// credentials never appear in URLs.
type TicketStore interface {
	Issue(ctx context.Context, userID uuid.UUID, role string, ttl time.Duration) (string, error)
	Consume(ctx context.Context, ticket string) (uuid.UUID, string, bool, error)
}
