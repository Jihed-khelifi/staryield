// Package e2e exercises the full API — HTTP, WebSocket, SSE, PostgreSQL and
// Redis — exactly as the frontend uses it. Time-dependent business rules
// (per-minute billing, request expiry, grace periods) are driven with a fake
// clock and explicit scheduler ticks, so tests are fast and deterministic.
//
// Requirements: PostgreSQL and Redis. Configure with
// STARYIELD_TEST_DATABASE_DSN and STARYIELD_TEST_REDIS_URL. When they are
// unreachable the suite is skipped unless STARYIELD_E2E=1 (then it fails).
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/app"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/migrations"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
)

const origin = "http://localhost:3000"

var (
	testDSN   = envOr("STARYIELD_TEST_DATABASE_DSN", "postgres://postgres@127.0.0.1:55432/staryield_test?sslmode=disable")
	testRedis = envOr("STARYIELD_TEST_REDIS_URL", "redis://127.0.0.1:56379/15")
	available bool
)

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func TestMain(m *testing.M) {
	time.Local = time.UTC
	if err := resetDatabase(); err != nil {
		if os.Getenv("STARYIELD_E2E") == "1" {
			fmt.Fprintln(os.Stderr, "e2e: infrastructure unavailable:", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "e2e: skipping (PostgreSQL/Redis unavailable):", err)
		os.Exit(0)
	}
	available = true
	os.Exit(m.Run())
}

// resetDatabase recreates the public schema and applies migrations once.
func resetDatabase() error {
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return err
	}
	if err := db.WithContext(ctx).Exec("DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public;").Error; err != nil {
		return err
	}
	_, err = migrations.Up(ctx, db, zap.NewNop())
	return err
}

// env is one isolated application instance (own Redis key prefix).
type env struct {
	t       *testing.T
	app     *app.App
	srv     *httptest.Server
	clock   *clock.Fake
	mail    *mail.Recorder
	store   *storage.Memory
	tracker *analytics.Recorder
}

type option func(*config.Config)

func newEnv(t *testing.T, opts ...option) *env {
	t.Helper()
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	cfg.App.Env = config.EnvTest
	cfg.Database.DSN = testDSN
	cfg.Database.AutoMigrate = false
	cfg.Redis.URL = testRedis
	cfg.Redis.KeyPrefix = "e2e:" + uuid.NewString()[:8] + ":"
	cfg.Auth.Argon2Memory, cfg.Auth.Argon2Iterations, cfg.Auth.Argon2Parallelism = 1024, 1, 1
	cfg.Worker.Enabled = false // tests drive the scheduler explicitly
	cfg.RateLimit.Scale = 1000
	cfg.Security.AllowedOrigins = []string{origin}
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
	fake := clock.NewFake(time.Now())
	rec := &mail.Recorder{}
	store := storage.NewMemory("https://assets.test")
	ctx := context.Background()
	a, err := app.New(ctx, cfg, log, app.Overrides{Clock: fake, Mailer: rec, Storage: store})
	if err != nil {
		t.Fatalf("app: %v", err)
	}
	a.Start(ctx)
	srv := httptest.NewServer(a.Router)
	e := &env{t: t, app: a, srv: srv, clock: fake, mail: rec, store: store}
	t.Cleanup(func() {
		srv.CloseClientConnections()
		srv.Close()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		a.Shutdown(sctx)
	})
	return e
}

// tick advances the fake clock and runs the session scheduler once.
func (e *env) tick(d time.Duration) {
	e.t.Helper()
	e.clock.Advance(d)
	if err := e.app.Scheduler.Tick(context.Background()); err != nil {
		e.t.Fatalf("scheduler tick: %v", err)
	}
}

func (e *env) maintenance() {
	e.t.Helper()
	if err := e.app.Scheduler.Maintenance(context.Background()); err != nil {
		e.t.Fatalf("maintenance: %v", err)
	}
}

// client is a browser-like API client with a cookie jar and CSRF handling.
type client struct {
	e      *env
	http   *http.Client
	csrf   string
	origin string
	user   *service.UserView
	pw     string
	// wsOrigin overrides the Origin header for WebSocket handshakes only.
	wsOrigin string
}

func (e *env) client() *client {
	jar, _ := cookiejar.New(nil)
	return &client{e: e, http: &http.Client{Jar: jar, Timeout: 15 * time.Second}, origin: origin}
}

type response struct {
	status int
	body   []byte
	header http.Header
}

func (r response) decode(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.body, v); err != nil {
		t.Fatalf("decode %s: %v", r.body, err)
	}
}

func (r response) errCode(t *testing.T) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(r.body, &body)
	return body.Error.Code
}

func (c *client) ensureCSRF() {
	if c.csrf != "" {
		return
	}
	r := c.raw(http.MethodGet, "/api/v1/auth/csrf", nil, nil)
	var out struct {
		Token string `json:"csrf_token"`
	}
	r.decode(c.e.t, &out)
	c.csrf = out.Token
}

func (c *client) raw(method, path string, body io.Reader, headers map[string]string) response {
	c.e.t.Helper()
	req, err := http.NewRequest(method, c.e.srv.URL+path, body)
	if err != nil {
		c.e.t.Fatalf("request: %v", err)
	}
	if c.origin != "" {
		req.Header.Set("Origin", c.origin)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if tok := res.Header.Get("X-CSRF-Token"); tok != "" {
		c.csrf = tok
	}
	return response{status: res.StatusCode, body: b, header: res.Header}
}

// call sends JSON with CSRF protection like the frontend does.
func (c *client) call(method, path string, payload any) response {
	c.e.t.Helper()
	headers := map[string]string{}
	var body io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		body = bytes.NewReader(b)
		headers["Content-Type"] = "application/json"
	}
	if method != http.MethodGet {
		c.ensureCSRF()
		headers["X-CSRF-Token"] = c.csrf
	}
	return c.raw(method, path, body, headers)
}

// must asserts the status code and decodes the body into out (optional).
func (c *client) must(status int, method, path string, payload, out any) response {
	c.e.t.Helper()
	r := c.call(method, path, payload)
	if r.status != status {
		c.e.t.Fatalf("%s %s: want %d, got %d: %s", method, path, status, r.status, r.body)
	}
	if out != nil {
		r.decode(c.e.t, out)
	}
	return r
}

var seq struct {
	sync.Mutex
	n int
}

func uniq(prefix string) string {
	seq.Lock()
	defer seq.Unlock()
	seq.n++
	return fmt.Sprintf("%s%d%s", prefix, seq.n, strings.ReplaceAll(uuid.NewString()[:6], "-", ""))
}

const strongPassword = "Str0ng!Passw0rd#"

// registerClient creates and signs in a client account.
func (e *env) registerClient() *client {
	e.t.Helper()
	c := e.client()
	email := uniq("client") + "@example.com"
	var out struct {
		User *service.UserView `json:"user"`
	}
	c.must(http.StatusCreated, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email": email, "password": strongPassword, "accept_terms": true, "birth_date": "1990-05-17", "locale": "en",
	}, &out)
	c.user, c.pw = out.User, strongPassword
	return c
}

// createPsychic creates an advisor via the admin service and signs in.
func (e *env) createPsychic(rate int64) *client {
	e.t.Helper()
	slug := uniq("psy")
	email := slug + "@advisors.example.com"
	_, err := e.app.Services.Admin.CreatePsychic(context.Background(), service.PsychicInput{
		Email: &email, Password: ptr(strongPassword), Name: ptr("Advisor " + slug), Slug: &slug, RatePerMinute: &rate,
	})
	if err != nil {
		e.t.Fatalf("create psychic: %v", err)
	}
	return e.login(email, strongPassword)
}

// createAdmin inserts an admin account directly and signs in.
func (e *env) createAdmin() *client {
	e.t.Helper()
	email := uniq("admin") + "@example.com"
	hasher := security.NewPasswordHasher(1024, 1, 1)
	hash, _ := hasher.Hash(strongPassword)
	now := e.clock.Now()
	u := domain.User{ID: domain.NewID(), Email: email, PasswordHash: hash, Role: domain.RoleAdmin, Status: domain.UserActive,
		DisplayName: "Admin", Locale: "en", TermsAcceptedAt: now, CreatedAt: now, UpdatedAt: now}
	if err := e.app.DB.Create(&u).Error; err != nil {
		e.t.Fatalf("create admin: %v", err)
	}
	if err := e.app.DB.Create(&domain.Profile{UserID: u.ID, Mood: "Calm", Theme: "Warm Beige", UpdatedAt: now}).Error; err != nil {
		e.t.Fatalf("admin profile: %v", err)
	}
	if err := e.app.DB.Create(&domain.Wallet{UserID: u.ID, UpdatedAt: now}).Error; err != nil {
		e.t.Fatalf("admin wallet: %v", err)
	}
	return e.login(email, strongPassword)
}

func (e *env) login(email, password string) *client {
	e.t.Helper()
	c := e.client()
	var out struct {
		User *service.UserView `json:"user"`
	}
	c.must(http.StatusOK, http.MethodPost, "/api/v1/auth/login", map[string]any{"email": email, "password": password}, &out)
	c.user, c.pw = out.User, password
	return c
}

func ptr[T any](v T) *T { return &v }

// buyCredits purchases the pack with the given credit amount.
func (c *client) buyCredits(credits int64) service.WalletView {
	c.e.t.Helper()
	var packs struct {
		Items []service.PackView `json:"items"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/credit-packs", nil, &packs)
	for _, p := range packs.Items {
		if p.Credits == credits {
			var out service.PurchaseView
			c.must(http.StatusCreated, http.MethodPost, "/api/v1/credits/purchases", map[string]any{
				"pack_id": p.ID, "idempotency_key": uniq("purchase-key-"),
			}, &out)
			return *out.Wallet
		}
	}
	c.e.t.Fatalf("no pack with %d credits", credits)
	return service.WalletView{}
}

// ensurePacks makes sure the reference packs exist in the shared database.
func (e *env) ensurePacks() {
	e.t.Helper()
	packs, err := e.app.Services.Credits.Packs(context.Background())
	if err != nil {
		e.t.Fatal(err)
	}
	have := map[int64]bool{}
	for _, p := range packs {
		have[p.Credits] = true
	}
	for i, p := range []struct {
		credits, cents int64
	}{{250, 999}, {600, 1999}, {1500, 3999}, {40, 113}} {
		if have[p.credits] {
			continue
		}
		name := fmt.Sprintf("Pack %d", p.credits)
		if _, err := e.app.Services.Admin.CreatePack(context.Background(), service.PackInput{
			Name: &name, Credits: ptr(p.credits), PriceCents: ptr(p.cents), SortOrder: ptr(i),
		}, "USD"); err != nil {
			e.t.Fatal(err)
		}
	}
}

// wallet returns the client's wallet.
func (c *client) wallet() service.WalletView {
	c.e.t.Helper()
	var w service.WalletView
	c.must(http.StatusOK, http.MethodGet, "/api/v1/wallet", nil, &w)
	return w
}

// ws is a realtime connection with an event inbox.
type ws struct {
	t      *testing.T
	conn   *websocket.Conn
	events chan realtime.Envelope
	closed chan struct{}
	code   int
	hello  realtime.Envelope
	mu     sync.Mutex
	refN   int
	lastID string
	// backlog keeps events received while waiting for something else, so
	// no event is lost between helper calls.
	backlog []realtime.Envelope
}

// next returns the next event (backlog first) or false on timeout/close.
func (w *ws) next(deadline <-chan time.Time) (realtime.Envelope, bool) {
	w.mu.Lock()
	if len(w.backlog) > 0 {
		e := w.backlog[0]
		w.backlog = w.backlog[1:]
		w.mu.Unlock()
		return e, true
	}
	w.mu.Unlock()
	select {
	case e := <-w.events:
		return e, true
	case <-deadline:
		return realtime.Envelope{}, false
	case <-w.closed:
		select {
		case e := <-w.events:
			return e, true
		default:
			return realtime.Envelope{}, false
		}
	}
}

func (w *ws) stash(events []realtime.Envelope) {
	w.mu.Lock()
	w.backlog = append(events, w.backlog...)
	w.mu.Unlock()
}

// connect opens a WebSocket using a fresh single-use ticket.
func (c *client) connect(resume string) *ws {
	c.e.t.Helper()
	conn, _, err := c.dial(resume)
	if err != nil {
		c.e.t.Fatalf("websocket dial: %v", err)
	}
	w := &ws{t: c.e.t, conn: conn, events: make(chan realtime.Envelope, 512), closed: make(chan struct{})}
	go w.read()
	w.hello = w.waitFor(realtime.EventHello, 5*time.Second)
	c.e.t.Cleanup(func() { _ = w.conn.Close() })
	return w
}

func (c *client) ticket() string {
	c.e.t.Helper()
	var out struct {
		Ticket string `json:"ticket"`
	}
	c.must(http.StatusOK, http.MethodPost, "/api/v1/realtime/ticket", nil, &out)
	return out.Ticket
}

func (c *client) dial(resume string) (*websocket.Conn, *http.Response, error) {
	u := "ws" + strings.TrimPrefix(c.e.srv.URL, "http") + "/api/v1/realtime/ws?ticket=" + url.QueryEscape(c.ticket())
	if resume != "" {
		u += "&resume=" + url.QueryEscape(resume)
	}
	h := http.Header{}
	h.Set("Origin", c.origin)
	if c.wsOrigin != "" {
		h.Set("Origin", c.wsOrigin)
	}
	d := websocket.Dialer{Subprotocols: []string{realtime.Subprotocol}, HandshakeTimeout: 5 * time.Second}
	return d.Dial(u, h)
}

func (w *ws) read() {
	defer close(w.closed)
	for {
		_, data, err := w.conn.ReadMessage()
		if err != nil {
			if ce, ok := err.(*websocket.CloseError); ok {
				w.code = ce.Code
			}
			return
		}
		var env realtime.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}
		if env.ID != "" {
			w.mu.Lock()
			w.lastID = env.ID
			w.mu.Unlock()
		}
		w.events <- env
	}
}

// waitFor returns the next event of type typ; other events stay queued.
func (w *ws) waitFor(typ string, timeout time.Duration) realtime.Envelope {
	w.t.Helper()
	deadline := time.After(timeout)
	var skipped []realtime.Envelope
	defer func() { w.stash(skipped) }()
	for {
		e, ok := w.next(deadline)
		if !ok {
			w.t.Fatalf("timed out (or closed, code %d) waiting for %q; saw %d other events", w.code, typ, len(skipped))
		}
		if e.Type == typ {
			return e
		}
		skipped = append(skipped, e)
	}
}

// waitMatch returns the next event of type typ whose payload contains sub.
func (w *ws) waitMatch(typ, sub string, timeout time.Duration) realtime.Envelope {
	w.t.Helper()
	deadline := time.After(timeout)
	var skipped []realtime.Envelope
	defer func() { w.stash(skipped) }()
	for {
		e, ok := w.next(deadline)
		if !ok {
			w.t.Fatalf("timed out waiting for %q containing %q", typ, sub)
		}
		if e.Type == typ && strings.Contains(string(e.Data), sub) {
			return e
		}
		skipped = append(skipped, e)
	}
}

// drain discards every queued event.
func (w *ws) drain() {
	w.mu.Lock()
	w.backlog = nil
	w.mu.Unlock()
	for {
		select {
		case <-w.events:
		case <-time.After(150 * time.Millisecond):
			return
		}
	}
}

// expectNone asserts no event of type typ arrives within d.
func (w *ws) expectNone(typ string, d time.Duration) {
	w.t.Helper()
	deadline := time.After(d)
	var skipped []realtime.Envelope
	defer func() { w.stash(skipped) }()
	for {
		e, ok := w.next(deadline)
		if !ok {
			return
		}
		if e.Type == typ {
			w.t.Fatalf("unexpected %q event: %s", typ, e.Data)
		}
		skipped = append(skipped, e)
	}
}

// request sends a frame and waits for its ack (or error).
func (w *ws) request(typ string, data any) realtime.Envelope {
	w.t.Helper()
	w.mu.Lock()
	w.refN++
	ref := fmt.Sprintf("r%d", w.refN)
	w.mu.Unlock()
	raw, _ := json.Marshal(data)
	if err := w.conn.WriteJSON(realtime.ClientFrame{Type: typ, Ref: ref, Data: raw}); err != nil {
		w.t.Fatalf("write frame: %v", err)
	}
	deadline := time.After(5 * time.Second)
	var skipped []realtime.Envelope
	defer func() { w.stash(skipped) }()
	for {
		e, ok := w.next(deadline)
		if !ok {
			w.t.Fatalf("no ack for %s", typ)
		}
		if e.Ref == ref && (e.Type == realtime.EventAck || e.Type == realtime.EventError) {
			return e
		}
		skipped = append(skipped, e)
	}
}

// mustAck sends a frame and requires a successful ack.
func (w *ws) mustAck(typ string, data any) json.RawMessage {
	w.t.Helper()
	e := w.request(typ, data)
	if e.Type != realtime.EventAck {
		w.t.Fatalf("%s failed: %+v", typ, e.Err)
	}
	return e.Data
}

func (w *ws) last() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastID
}

func (w *ws) waitClosed(timeout time.Duration) int {
	w.t.Helper()
	select {
	case <-w.closed:
		return w.code
	case <-time.After(timeout):
		w.t.Fatalf("connection not closed")
	}
	return 0
}

// sessionFrom decodes the "session" field of an event payload.
func sessionFrom(t *testing.T, data json.RawMessage) service.SessionView {
	t.Helper()
	var out struct {
		Session service.SessionView `json:"session"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode session: %v (%s)", err, data)
	}
	return out.Session
}

// goOnline connects the psychic and marks them available.
func (p *client) goOnline() *ws {
	p.e.t.Helper()
	w := p.connect("")
	p.must(http.StatusOK, http.MethodPatch, "/api/v1/psychic/availability", map[string]any{"availability": "online"}, nil)
	return w
}

func (p *client) slug() string { return p.user.Psychic.Slug }
