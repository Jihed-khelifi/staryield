package realtime

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// RedisTickets implements TicketStore. Only the ticket hash is stored and it
// is deleted atomically on first use (GETDEL), so a leaked or replayed
// ticket cannot open a second connection.
type RedisTickets struct {
	client *redis.Client
	prefix string
}

// NewRedisTickets returns a Redis ticket store.
func NewRedisTickets(client *redis.Client, prefix string) *RedisTickets {
	return &RedisTickets{client: client, prefix: prefix}
}

func (t *RedisTickets) key(ticket string) string {
	return t.prefix + "rt:t:" + security.HashToken(ticket)
}

// Issue implements TicketStore.
func (t *RedisTickets) Issue(ctx context.Context, userID uuid.UUID, role string, ttl time.Duration) (string, error) {
	ticket, err := security.RandomToken(32)
	if err != nil {
		return "", err
	}
	if err := t.client.Set(ctx, t.key(ticket), userID.String()+"|"+role, ttl).Err(); err != nil {
		return "", err
	}
	return ticket, nil
}

// Consume implements TicketStore.
func (t *RedisTickets) Consume(ctx context.Context, ticket string) (uuid.UUID, string, bool, error) {
	if ticket == "" || len(ticket) > 128 {
		return uuid.Nil, "", false, nil
	}
	v, err := t.client.GetDel(ctx, t.key(ticket)).Result()
	if err == redis.Nil {
		return uuid.Nil, "", false, nil
	}
	if err != nil {
		return uuid.Nil, "", false, err
	}
	return parseTicketValue(v)
}

func parseTicketValue(v string) (uuid.UUID, string, bool, error) {
	idStr, role, ok := strings.Cut(v, "|")
	if !ok {
		return uuid.Nil, "", false, nil
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, "", false, nil
	}
	return id, role, true, nil
}

// MemoryTickets implements TicketStore in-process.
type MemoryTickets struct {
	mu      sync.Mutex
	tickets map[string]memTicket
	now     func() time.Time
}

type memTicket struct {
	value   string
	expires time.Time
}

// NewMemoryTickets returns an in-memory ticket store.
func NewMemoryTickets(now func() time.Time) *MemoryTickets {
	if now == nil {
		now = time.Now
	}
	return &MemoryTickets{tickets: map[string]memTicket{}, now: now}
}

// Issue implements TicketStore.
func (m *MemoryTickets) Issue(_ context.Context, userID uuid.UUID, role string, ttl time.Duration) (string, error) {
	ticket, err := security.RandomToken(32)
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	m.tickets[security.HashToken(ticket)] = memTicket{value: userID.String() + "|" + role, expires: m.now().Add(ttl)}
	m.mu.Unlock()
	return ticket, nil
}

// Consume implements TicketStore.
func (m *MemoryTickets) Consume(_ context.Context, ticket string) (uuid.UUID, string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h := security.HashToken(ticket)
	t, ok := m.tickets[h]
	delete(m.tickets, h)
	if !ok || m.now().After(t.expires) {
		return uuid.Nil, "", false, nil
	}
	return parseTicketValue(t.value)
}
