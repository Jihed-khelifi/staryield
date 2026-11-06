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

// TicketClaims is what a consumed ticket authenticates.
type TicketClaims struct {
	UserID uuid.UUID
	Role   string
	// Family is the refresh-token family (signed-in device) that asked for
	// the ticket; zero when unknown.
	Family uuid.UUID
}

// FamilyTickets is implemented by ticket stores that remember the session
// family, so signing one device out closes only that device's connections.
type FamilyTickets interface {
	IssueFamily(ctx context.Context, userID uuid.UUID, role string, family uuid.UUID, ttl time.Duration) (string, error)
	ConsumeClaims(ctx context.Context, ticket string) (TicketClaims, bool, error)
}

func ticketValue(userID uuid.UUID, role string, family uuid.UUID) string {
	v := userID.String() + "|" + role
	if family != uuid.Nil {
		v += "|" + family.String()
	}
	return v
}

// Issue implements TicketStore.
func (t *RedisTickets) Issue(ctx context.Context, userID uuid.UUID, role string, ttl time.Duration) (string, error) {
	return t.IssueFamily(ctx, userID, role, uuid.Nil, ttl)
}

// IssueFamily implements FamilyTickets.
func (t *RedisTickets) IssueFamily(ctx context.Context, userID uuid.UUID, role string, family uuid.UUID, ttl time.Duration) (string, error) {
	ticket, err := security.RandomToken(32)
	if err != nil {
		return "", err
	}
	if err := t.client.Set(ctx, t.key(ticket), ticketValue(userID, role, family), ttl).Err(); err != nil {
		return "", err
	}
	return ticket, nil
}

// Consume implements TicketStore.
func (t *RedisTickets) Consume(ctx context.Context, ticket string) (uuid.UUID, string, bool, error) {
	c, ok, err := t.ConsumeClaims(ctx, ticket)
	return c.UserID, c.Role, ok, err
}

// ConsumeClaims implements FamilyTickets.
func (t *RedisTickets) ConsumeClaims(ctx context.Context, ticket string) (TicketClaims, bool, error) {
	if ticket == "" || len(ticket) > 128 {
		return TicketClaims{}, false, nil
	}
	v, err := t.client.GetDel(ctx, t.key(ticket)).Result()
	if err == redis.Nil {
		return TicketClaims{}, false, nil
	}
	if err != nil {
		return TicketClaims{}, false, err
	}
	c, ok := parseTicketValue(v)
	return c, ok, nil
}

// parseTicketValue reads "user|role" or "user|role|family".
func parseTicketValue(v string) (TicketClaims, bool) {
	parts := strings.Split(v, "|")
	if len(parts) < 2 || len(parts) > 3 {
		return TicketClaims{}, false
	}
	id, err := uuid.Parse(parts[0])
	if err != nil {
		return TicketClaims{}, false
	}
	c := TicketClaims{UserID: id, Role: parts[1]}
	if len(parts) == 3 {
		if c.Family, err = uuid.Parse(parts[2]); err != nil {
			return TicketClaims{}, false
		}
	}
	return c, true
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
func (m *MemoryTickets) Issue(ctx context.Context, userID uuid.UUID, role string, ttl time.Duration) (string, error) {
	return m.IssueFamily(ctx, userID, role, uuid.Nil, ttl)
}

// IssueFamily implements FamilyTickets.
func (m *MemoryTickets) IssueFamily(_ context.Context, userID uuid.UUID, role string, family uuid.UUID, ttl time.Duration) (string, error) {
	ticket, err := security.RandomToken(32)
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	m.tickets[security.HashToken(ticket)] = memTicket{value: ticketValue(userID, role, family), expires: m.now().Add(ttl)}
	m.mu.Unlock()
	return ticket, nil
}

// Consume implements TicketStore.
func (m *MemoryTickets) Consume(ctx context.Context, ticket string) (uuid.UUID, string, bool, error) {
	c, ok, err := m.ConsumeClaims(ctx, ticket)
	return c.UserID, c.Role, ok, err
}

// ConsumeClaims implements FamilyTickets.
func (m *MemoryTickets) ConsumeClaims(_ context.Context, ticket string) (TicketClaims, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h := security.HashToken(ticket)
	t, ok := m.tickets[h]
	delete(m.tickets, h)
	if !ok || m.now().After(t.expires) {
		return TicketClaims{}, false, nil
	}
	c, valid := parseTicketValue(t.value)
	return c, valid, nil
}
