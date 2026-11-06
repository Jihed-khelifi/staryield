package e2e

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/app"
	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/push"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// configAlias lets option literals in this package name the config type.
type configAlias = config.Config

// fakePush records Web Push deliveries; Outcome decides the push service's
// answer (default: delivered).
type fakePush struct {
	mu      sync.Mutex
	sent    []pushSend
	Outcome push.Outcome
}

type pushSend struct {
	Endpoint string
	Payload  service.PushPayload
	Topic    string
	Urgency  string
}

func (f *fakePush) Send(_ context.Context, sub push.Subscription, m push.Message) push.Result {
	var p service.PushPayload
	_ = json.Unmarshal(m.Payload, &p)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, pushSend{Endpoint: sub.Endpoint, Payload: p, Topic: m.Topic, Urgency: m.Urgency})
	return push.Result{Outcome: f.Outcome}
}

func (f *fakePush) Sent() []pushSend {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]pushSend(nil), f.sent...)
}

func (f *fakePush) setOutcome(o push.Outcome) {
	f.mu.Lock()
	f.Outcome = o
	f.mu.Unlock()
}

// withPush enables Web Push with a fresh VAPID key pair.
func withPush(t *testing.T) option {
	t.Helper()
	pub, priv, err := push.GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}
	return func(c *config.Config) {
		c.Push.Enabled = true
		c.Push.VAPIDPublicKey, c.Push.VAPIDPrivateKey = pub, priv
	}
}

// flushMail runs the e-mail outbox job (the worker is off in tests) and
// returns everything the provider received so far.
func (e *env) flushMail() []mail.Message {
	e.t.Helper()
	if err := e.app.Scheduler.RunJob(context.Background(), app.MailJob); err != nil {
		e.t.Fatalf("mail job: %v", err)
	}
	return e.mail.Sent()
}

// notify sends a notification through the real dispatcher and waits until it
// is persisted and fanned out (e-mail enqueued, pushes queued).
func (e *env) notify(spec service.NotificationSpec, userIDs ...uuid.UUID) {
	e.t.Helper()
	before := e.app.Dispatcher.Processed()
	e.app.Dispatcher.Notify(context.Background(), spec, userIDs...)
	want := before + int64(len(userIDs))
	deadline := time.Now().Add(5 * time.Second)
	for {
		if e.app.Dispatcher.Processed() >= want {
			return
		}
		if time.Now().After(deadline) {
			e.t.Fatalf("notification %s not delivered", spec.Kind)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// waitPushes waits until the fake push service received at least n
// messages (delivery runs on background workers).
func (e *env) waitPushes(n int) []pushSend {
	e.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if sent := e.push.Sent(); len(sent) >= n {
			return sent
		}
		if time.Now().After(deadline) {
			e.t.Fatalf("want %d pushes, got %d", n, len(e.push.Sent()))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// browserSubscription returns a PushSubscription.toJSON() body for an
// allow-listed push service.
func browserSubscription(t *testing.T, endpoint string) map[string]any {
	t.Helper()
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, 16)
	_, _ = rand.Read(secret)
	return map[string]any{"endpoint": endpoint, "expirationTime": nil, "keys": map[string]any{
		"p256dh": base64.RawURLEncoding.EncodeToString(k.PublicKey().Bytes()),
		"auth":   base64.RawURLEncoding.EncodeToString(secret),
	}}
}
