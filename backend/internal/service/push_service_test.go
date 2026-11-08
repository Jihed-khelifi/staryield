package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/push"
)

type noopSender struct{}

func (noopSender) Send(context.Context, push.Subscription, push.Message) push.Result {
	return push.Result{Outcome: push.Delivered}
}

func queuedPush(t *testing.T, s *PushService, kind string, data map[string]any) (pushJob, PushPayload) {
	t.Helper()
	s.NotifyUser(uuid.New(), "en", &domain.Notification{Kind: kind, Data: data, Link: "/chatroom"})
	select {
	case job := <-s.queue:
		var p PushPayload
		if err := json.Unmarshal(job.payload, &p); err != nil {
			t.Fatal(err)
		}
		return job, p
	default:
		t.Fatalf("%s: nothing queued", kind)
		return pushJob{}, PushPayload{}
	}
}

func TestNotifyUserChatRequestTagTTLAndUrgency(t *testing.T) {
	s := NewPushService(nil, nil, noopSender{}, clock.Real{}, zap.NewNop(),
		PushSettings{PublicKey: "k", FrontendURL: "https://app.test", ChatRequestTTL: 90 * time.Second})
	data := map[string]any{"session_id": "abc-123", "client_name": "Ana"}

	job, p := queuedPush(t, s, NotifyChatRequested, data)
	if p.Kind != NotifyChatRequested || p.Tag != "chat.request:abc-123" {
		t.Fatalf("requested payload: %+v", p)
	}
	if job.ttl != 90*time.Second || job.urgency != "high" || job.topic != "chatrequestabc123" {
		t.Fatalf("requested job: ttl=%v urgency=%q topic=%q", job.ttl, job.urgency, job.topic)
	}

	// The missed push shares the tag (and so the topic) with the request,
	// keeps its own kind, and uses the default TTL and urgency.
	mjob, mp := queuedPush(t, s, NotifyChatMissed, data)
	if mp.Kind != NotifyChatMissed || mp.Tag != p.Tag || mjob.topic != job.topic {
		t.Fatalf("missed payload: %+v topic=%q", mp, mjob.topic)
	}
	if mjob.ttl != 0 || mjob.urgency != "normal" {
		t.Fatalf("missed job: ttl=%v urgency=%q", mjob.ttl, mjob.urgency)
	}
}

func TestNotifyUserOtherKindsKeepTheirTags(t *testing.T) {
	s := NewPushService(nil, nil, noopSender{}, clock.Real{}, zap.NewNop(),
		PushSettings{PublicKey: "k", FrontendURL: "https://app.test", ChatRequestTTL: 90 * time.Second})
	job, p := queuedPush(t, s, NotifyReviewReceived, map[string]any{"review_id": "r-1"})
	if p.Tag != NotifyReviewReceived+":r-1" || job.ttl != 0 || job.urgency != "normal" {
		t.Fatalf("review push: %+v ttl=%v urgency=%q", p, job.ttl, job.urgency)
	}
}

func TestChatRequestTTLUnsetFallsBackToDefault(t *testing.T) {
	s := NewPushService(nil, nil, noopSender{}, clock.Real{}, zap.NewNop(), PushSettings{PublicKey: "k", FrontendURL: "https://app.test"})
	a, _ := queuedPush(t, s, NotifyChatRequested, map[string]any{"session_id": uuid.NewString()})
	if a.ttl != 0 {
		t.Fatalf("unset ChatRequestTTL must fall back to the default: %v", a.ttl)
	}
}
