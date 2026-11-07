package cache

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Bus is a fire-and-forget broadcast channel between instances (Redis
// Pub/Sub in production), used to tell every instance to drop an
// in-process copy. Delivery is best effort: copies also expire by TTL.
type Bus interface {
	// Publish sends msg to every subscriber of channel (this instance's
	// included).
	Publish(ctx context.Context, channel, msg string) error
	// Subscribe delivers messages of channel to handle until ctx ends. It
	// reconnects on its own; handle must not block.
	Subscribe(ctx context.Context, channel string, handle func(msg string))
}

// RedisBus implements Bus over Redis Pub/Sub. Channels carry the key prefix
// so isolated deployments (and tests) sharing a Redis never hear each other.
type RedisBus struct {
	client *redis.Client
	prefix string
}

// NewRedisBus returns a Pub/Sub bus.
func NewRedisBus(client *redis.Client, prefix string) *RedisBus {
	return &RedisBus{client: client, prefix: prefix}
}

// Publish implements Bus.
func (b *RedisBus) Publish(ctx context.Context, channel, msg string) error {
	return b.client.Publish(ctx, b.prefix+channel, msg).Err()
}

// Subscribe implements Bus. A dropped subscription is re-established after a
// short pause; messages sent meanwhile are lost (callers rely on TTLs).
func (b *RedisBus) Subscribe(ctx context.Context, channel string, handle func(msg string)) {
	for ctx.Err() == nil {
		sub := b.client.Subscribe(ctx, b.prefix+channel)
		ch := sub.Channel()
	loop:
		for {
			select {
			case <-ctx.Done():
				break loop
			case m, ok := <-ch:
				if !ok {
					break loop
				}
				handle(m.Payload)
			}
		}
		_ = sub.Close()
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
		}
	}
}

// MemoryBus implements Bus in-process (tests, and instances sharing one
// process).
type MemoryBus struct {
	mu   sync.Mutex
	subs map[string][]chan string
}

// NewMemoryBus returns an in-process bus.
func NewMemoryBus() *MemoryBus { return &MemoryBus{subs: map[string][]chan string{}} }

// Publish implements Bus.
func (b *MemoryBus) Publish(_ context.Context, channel, msg string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs[channel] {
		select {
		case ch <- msg:
		default: // a slow subscriber misses it, like Redis
		}
	}
	return nil
}

// Subscribe implements Bus.
func (b *MemoryBus) Subscribe(ctx context.Context, channel string, handle func(msg string)) {
	ch := make(chan string, 16)
	b.mu.Lock()
	b.subs[channel] = append(b.subs[channel], ch)
	b.mu.Unlock()
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-ch:
			handle(m)
		}
	}
}
