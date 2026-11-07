package cache

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestMemoryBusDeliversToEverySubscriberOfAChannel(t *testing.T) {
	bus := NewMemoryBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	got := map[string][]string{}
	sub := func(name, channel string) {
		go bus.Subscribe(ctx, channel, func(m string) {
			mu.Lock()
			got[name] = append(got[name], m)
			mu.Unlock()
		})
	}
	sub("a", "site:config:changed")
	sub("b", "site:config:changed")
	sub("other", "something:else")
	time.Sleep(50 * time.Millisecond)
	if err := bus.Publish(ctx, "site:config:changed", "changed"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		n := len(got["a"]) + len(got["b"])
		mu.Unlock()
		if n == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("delivered %v", got)
		}
		time.Sleep(5 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got["other"]) != 0 {
		t.Fatalf("a subscriber of another channel received %v", got["other"])
	}
}
