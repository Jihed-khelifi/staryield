package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/app"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
)

// shared is what several application instances of one cluster have in
// common: the database (always), the Redis key prefix, the clock, mail and
// object storage — exactly like API replicas behind a load balancer.
type shared struct {
	prefix   string
	clock    *clock.Fake
	mail     *mail.Recorder
	store    *storage.Memory
	advisors *storage.Memory
	chat     *storage.Memory
}

// newCluster starts n application instances sharing PostgreSQL, Redis (one
// key prefix) and a fake clock. A client created on one instance can talk
// to another with client.on(otherEnv).
func newCluster(t *testing.T, n int, opts ...option) []*env {
	t.Helper()
	sh := shared{
		prefix: "e2e:" + uuid.NewString()[:8] + ":", clock: clock.NewFake(time.Now()), mail: &mail.Recorder{},
		store: storage.NewMemory("https://assets.test"), advisors: storage.NewMemory("https://advisors.test"), chat: storage.NewMemory(""),
	}
	out := make([]*env, n)
	for i := range out {
		out[i] = newSharedEnv(t, sh, opts...)
	}
	return out
}

// newSharedEnv is newEnv with the cluster's shared parts.
func newSharedEnv(t *testing.T, sh shared, opts ...option) *env {
	t.Helper()
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	cfg.App.Env = config.EnvTest
	cfg.Database.DSN = testDSN
	cfg.Redis.URL = testRedis
	cfg.Redis.KeyPrefix = sh.prefix
	cfg.Auth.Argon2Memory, cfg.Auth.Argon2Iterations, cfg.Auth.Argon2Parallelism = 1024, 1, 1
	cfg.Worker.Enabled = false
	cfg.RateLimit.Scale = 1000
	cfg.Security.AllowedOrigins = []string{origin}
	cfg.Security.AdminOrigins = []string{adminOrigin}
	cfg.Log.Level = envOr("STARYIELD_TEST_LOG_LEVEL", "error")
	cfg.Analytics.FlushInterval = 50 * time.Millisecond
	for _, o := range opts {
		o(cfg)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("config invalid: %v", err)
	}
	log := zap.NewNop()
	if cfg.Log.Level != "error" {
		log, _ = zap.NewDevelopment()
	}
	ctx := context.Background()
	a, err := app.New(ctx, cfg, log, app.Overrides{Clock: sh.clock, Mailer: sh.mail, Storage: sh.store, AdvisorStorage: sh.advisors, ChatStorage: sh.chat})
	if err != nil {
		t.Fatalf("app: %v", err)
	}
	a.Start(ctx)
	srv := httptest.NewServer(a.Router)
	e := &env{t: t, app: a, srv: srv, clock: sh.clock, mail: sh.mail, store: sh.store, advisors: sh.advisors, chat: sh.chat}
	t.Cleanup(func() {
		srv.CloseClientConnections()
		srv.Close()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		a.Shutdown(sctx)
	})
	return e
}

// on returns the same browser (cookies, CSRF token) talking to another
// instance of the cluster.
func (c *client) on(e *env) *client {
	cp := *c
	cp.e = e
	return &cp
}

// postMessage sends a chat message over REST without failing the test from
// a goroutine; it returns an error instead.
func postMessage(c *client, sessionID uuid.UUID, body, clientMsgID string) error {
	payload, _ := json.Marshal(map[string]any{"body": body, "client_msg_id": clientMsgID})
	req, err := http.NewRequest(http.MethodPost, c.e.srv.URL+"/api/v1/chat/sessions/"+sessionID.String()+"/messages", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", c.origin)
	req.Header.Set("X-CSRF-Token", c.csrf)
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("send %q: %d %s", body, res.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}
