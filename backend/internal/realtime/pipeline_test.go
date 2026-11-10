package realtime

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// PublishMany sends one durable event per user, in a few round trips on
// Redis, and every user's stream holds exactly its own event.
func TestPublisherPublishManyPipelinesOnRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	b := NewRedisBroker(client, "t:", 100, time.Hour, zap.NewNop())
	pub := NewPublisher(b, nil)
	ctx := context.Background()

	const n = 1200 // more than one pipeline chunk
	events := make([]UserEvent, n)
	for i := range events {
		events[i] = UserEvent{UserID: uuid.New(), Data: map[string]int{"i": i}}
	}
	events = append(events, UserEvent{UserID: uuid.Nil, Data: 0}) // skipped
	failed, err := pub.PublishMany(ctx, "notification.created", events)
	if err != nil || len(failed) != 0 {
		t.Fatalf("publish many: failed=%v err=%v", failed, err)
	}
	for _, i := range []int{0, 499, 500, n - 1} {
		msgs, err := client.XRange(ctx, b.streamKey(events[i].UserID), "-", "+").Result()
		if err != nil || len(msgs) != 1 || !containsInt([]byte(msgs[0].Values["p"].(string)), i) {
			t.Fatalf("user %d stream: %v %+v", i, err, msgs)
		}
	}
}

func containsInt(frame []byte, i int) bool {
	return strings.Contains(string(frame), `"i":`+strconv.Itoa(i)+`}`)
}

func TestPublisherPublishManyOnMemoryBroker(t *testing.T) {
	b := NewMemoryBroker(10)
	pub := NewPublisher(b, nil)
	users := []uuid.UUID{uuid.New(), uuid.New()}
	failed, err := pub.PublishMany(context.Background(), "x", []UserEvent{{users[0], 1}, {users[1], 2}})
	if err != nil || len(failed) != 0 {
		t.Fatalf("%v %v", failed, err)
	}
	for _, u := range users {
		if tail, _ := b.Tail(context.Background(), u); tail == "" {
			t.Fatalf("user %s has no event", u)
		}
	}
}
