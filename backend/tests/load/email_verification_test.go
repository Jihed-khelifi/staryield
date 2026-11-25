//go:build load

package load

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
)

// Growth: e-mail verification under concurrency, against a running API.
//
// The links travel only by e-mail, so the API must use the log mail driver
// (STARYIELD_MAIL_DRIVER=log) and LOAD_API_LOG must name its log file, where
// every e-mail body is printed; without it the test is skipped. It also needs
// LOAD_DATABASE_DSN (the API's database) for the final checks.
//
//  1. LOAD_VERIFY_USERS accounts (default 300) sign up at a constant rate
//     and receive their welcome link; one in four listens on a WebSocket.
//  2. Once the 60 s resend cooldown has passed, every account's link is
//     opened from two devices at the same moment as a resend is requested
//     for it (a person double-clicking while another tab asks for a new
//     link): the three requests contend for the same user row and token.
//  3. Accounts whose link the resend replaced open their newest link.
//
// Objectives: no server error in any phase, latency within the SLOs, every
// account verified exactly once (one audit row, one realtime event per
// listening account), no open link left behind.
//
//	LOAD_API_LOG=/path/api.log go test -tags load ./tests/load/ -run TestLoadEmailVerification -v -timeout 20m
func TestLoadEmailVerification(t *testing.T) {
	cfg := LoadConfig()
	logPath := env("LOAD_API_LOG", "")
	if logPath == "" {
		t.Skip("needs LOAD_API_LOG: the log of an API using the log mail driver (the links are only in the e-mails)")
	}
	RequireAPI(t, cfg)
	ctx := context.Background()
	live, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("database %s: %v", cfg.DSN, err)
	}
	n := envInt("LOAD_VERIFY_USERS", 300)
	run := time.Now().UnixNano() % 1_000_000

	// 1. Sign-ups (each queues its welcome link).
	accounts := make([]*verifyAccount, n)
	signup := func(ctx context.Context, _ *Client, i int64) (int, error) {
		if int(i) >= n {
			return http.StatusOK, nil
		}
		a := &verifyAccount{email: fmt.Sprintf("load-verify-%06d-%05d@example.test", run, i), c: NewClient(cfg.API, cfg.Origin)}
		var out struct {
			User struct {
				ID uuid.UUID `json:"id"`
			} `json:"user"`
		}
		status, err := withBackoff(ctx, func() (int, error) {
			return a.c.Do(ctx, http.MethodPost, "/api/v1/auth/register", map[string]any{
				"email": a.email, "password": "L0ad!Test#Passw0rd", "accept_terms": true, "birth_date": "1990-05-17", "locale": "en",
			}, &out)
		})
		if err == nil {
			a.id = out.User.ID
			accounts[i] = a
		}
		return status, err
	}
	// Sign-ups are bounded by password hashing (argon2 admission answers 503
	// when saturated), not by this feature: a fixed rate, retried like the
	// web app does.
	const signupRate = 10
	signups := Run(ctx, Scenario{Name: "POST register (welcome link)", Rate: signupRate, Duration: durationFor(n, signupRate),
		SLO: SLO{P95: 1500 * time.Millisecond, P99: 3 * time.Second, MaxErrRate: 0.001}, Do: signup}, []*Client{NewClient(cfg.API, cfg.Origin)})
	signedUp := time.Now()
	var ready []*verifyAccount
	for _, a := range accounts {
		if a != nil {
			ready = append(ready, a)
		}
	}
	if len(ready) == 0 {
		t.Fatalf("no account could sign up: %v", signups.ErrorSamples)
	}

	// Welcome links, from the log (the outbox delivers within seconds).
	links := waitForLinks(t, logPath, ready, 1, 90*time.Second)
	for _, a := range ready {
		a.token = links[a.email][0]
	}
	// One account in four listens for the live update.
	var listeners []*verifyListener
	for i, a := range ready {
		if i%4 != 0 {
			continue
		}
		l, err := listen(ctx, a.c, realtime.EventEmailVerified)
		if err != nil {
			t.Fatalf("websocket for %s: %v", a.email, err)
		}
		a.listener = l
		listeners = append(listeners, l)
	}
	defer func() {
		for _, l := range listeners {
			l.close()
		}
	}()

	// 2. Past the cooldown, open each link twice while a resend runs.
	if wait := time.Until(signedUp.Add(61 * time.Second)); wait > 0 {
		t.Logf("waiting %s for the resend cooldown of the last sign-ups", wait.Round(time.Second))
		time.Sleep(wait)
	}
	anon := make([]*Client, 64)
	for i := range anon {
		anon[i] = NewClient(cfg.API, cfg.Origin)
	}
	var serverErrors atomic.Int64
	refusedOK := func(status int, err error, allowed ...string) error {
		if status >= 500 || status == 0 {
			serverErrors.Add(1)
			return err
		}
		if apiErr, ok := err.(*APIError); ok {
			for _, code := range allowed {
				if apiErr.Code == code {
					return nil // an expected answer of the race (another request won)
				}
			}
		}
		return err
	}
	openLink := func(ctx context.Context, c *Client, i int64) (int, error) {
		a := ready[int(i)%len(ready)]
		status, err := c.Do(ctx, http.MethodPost, "/api/v1/auth/email/verify", map[string]any{"token": a.token}, nil)
		return status, refusedOK(status, err, "invalid_token")
	}
	resend := func(ctx context.Context, _ *Client, i int64) (int, error) {
		a := ready[int(i)%len(ready)]
		status, err := a.c.Do(ctx, http.MethodPost, "/api/v1/me/email/verification", nil, nil)
		return status, refusedOK(status, err, "already_verified", "rate_limited")
	}
	rate := cfg.Rate(20)
	dur := durationFor(len(ready), rate)
	slo := SLO{P95: 250 * time.Millisecond, P99: 750 * time.Millisecond, MaxErrRate: 0}
	results := runAll(ctx, []Scenario{
		{Name: "POST email/verify (device 1)", Rate: rate, Duration: dur, SLO: slo, Do: openLink},
		{Name: "POST email/verify (device 2)", Rate: rate, Duration: dur, SLO: slo, Do: openLink},
		{Name: "POST me/email/verification", Rate: rate, Duration: dur, SLO: slo, Do: resend},
	}, anon)

	// 3. Accounts whose link the resend replaced open the newest one.
	var replaced []*verifyAccount
	for _, a := range ready {
		var me struct {
			EmailVerified bool `json:"email_verified"`
		}
		if status, err := a.c.Do(ctx, http.MethodGet, "/api/v1/me", nil, &me); err != nil {
			if status >= 500 {
				serverErrors.Add(1)
			}
			t.Fatalf("GET /me for %s: %v", a.email, err)
		}
		if !me.EmailVerified {
			replaced = append(replaced, a)
		}
	}
	if len(replaced) > 0 {
		newest := waitForLinks(t, logPath, replaced, 2, 90*time.Second)
		for _, a := range replaced {
			tokens := newest[a.email]
			status, err := anon[0].Do(ctx, http.MethodPost, "/api/v1/auth/email/verify", map[string]any{"token": tokens[len(tokens)-1]}, nil)
			if err != nil {
				if status >= 500 {
					serverErrors.Add(1)
				}
				t.Errorf("the newest link of %s: %d %v", a.email, status, err)
			}
		}
	}
	Report(t, cfg, "email-verification", append([]Result{signups}, results...))
	t.Logf("%d accounts; %d links replaced by a concurrent resend and opened afterwards", len(ready), len(replaced))

	if n := serverErrors.Load(); n > 0 {
		t.Errorf("%d server errors under concurrency", n)
	}
	ids := make([]uuid.UUID, len(ready))
	for i, a := range ready {
		ids[i] = a.id
	}
	type count struct {
		TargetID string
		N        int64
	}
	var audited []count
	live.Raw(`SELECT target_id, count(*) AS n FROM audit_logs WHERE action = 'account.email_verified' AND target_id IN ? GROUP BY target_id`,
		uuidStrings(ids)).Scan(&audited)
	if len(audited) != len(ready) {
		t.Errorf("%d of %d accounts have a verification audit row", len(audited), len(ready))
	}
	for _, c := range audited {
		if c.N != 1 {
			t.Errorf("account %s verified %d times", c.TargetID, c.N)
		}
	}
	var stillOpen int64
	live.Raw(`SELECT count(*) FROM user_action_tokens WHERE purpose = 'email_verify' AND used_at IS NULL AND user_id IN ?`, ids).Scan(&stillOpen)
	if stillOpen != 0 {
		t.Errorf("%d links still open after every account was verified", stillOpen)
	}
	deadline := time.Now().Add(10 * time.Second)
	for _, a := range ready {
		if a.listener == nil {
			continue
		}
		for a.listener.count() == 0 && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
		}
	}
	time.Sleep(500 * time.Millisecond) // a duplicate would arrive right after the first event
	for _, a := range ready {
		if a.listener != nil && a.listener.count() != 1 {
			t.Errorf("%s heard the verification %d times on its open tab", a.email, a.listener.count())
		}
	}
}

type verifyAccount struct {
	email    string
	id       uuid.UUID
	c        *Client
	token    string
	listener *verifyListener
}

// withBackoff retries the answers a browser retries (503 while password
// hashing is saturated, 429) with jittered backoff, like Client.Login.
func withBackoff(ctx context.Context, do func() (int, error)) (int, error) {
	backoff := 250 * time.Millisecond
	for attempt := 0; ; attempt++ {
		status, err := do()
		if err == nil || attempt >= 8 || (status != http.StatusServiceUnavailable && status != http.StatusTooManyRequests) {
			return status, err
		}
		select {
		case <-ctx.Done():
			return status, ctx.Err()
		case <-time.After(backoff + time.Duration(rand.Int64N(int64(backoff)))):
		}
		backoff = min(2*backoff, 4*time.Second)
	}
}

// durationFor is how long a constant rate takes to reach every account once.
func durationFor(accounts, rate int) time.Duration {
	return time.Duration(accounts)*time.Second/time.Duration(rate) - time.Millisecond
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

var verifyLinkRE = regexp.MustCompile(`/verify-email#token=([A-Za-z0-9_-]+)`)

// waitForLinks reads the verification links of the accounts from the API log
// (oldest first) until each has at least min of them.
func waitForLinks(t *testing.T, path string, accounts []*verifyAccount, min int, timeout time.Duration) map[string][]string {
	t.Helper()
	want := make(map[string]bool, len(accounts))
	for _, a := range accounts {
		want[a.email] = true
	}
	deadline := time.Now().Add(timeout)
	for {
		links, err := scanVerificationLinks(path, want)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		missing := 0
		for email := range want {
			if len(links[email]) < min {
				missing++
			}
		}
		if missing == 0 {
			return links
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d of %d accounts have fewer than %d verification e-mails in %s", missing, len(want), min, path)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// scanVerificationLinks: the log mail driver prints one line per e-mail with
// its recipient ("to") and its body.
func scanVerificationLinks(path string, want map[string]bool) (map[string][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string][]string{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 8<<20)
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, "/verify-email#token=") {
			continue
		}
		i := strings.Index(line, `"to": "`)
		if i < 0 {
			continue
		}
		to := line[i+len(`"to": "`):]
		to = to[:strings.IndexByte(to, '"')]
		if !want[to] {
			continue
		}
		if m := verifyLinkRE.FindStringSubmatch(line); m != nil {
			out[to] = append(out[to], m[1])
		}
	}
	return out, sc.Err()
}

// verifyListener counts the events of one type a signed-in tab receives.
type verifyListener struct {
	conn *websocket.Conn
	n    atomic.Int64
	done chan struct{}
	once sync.Once
}

func (l *verifyListener) count() int64 { return l.n.Load() }

func (l *verifyListener) close() {
	l.once.Do(func() { _ = l.conn.Close(); <-l.done })
}

func listen(ctx context.Context, c *Client, typ string) (*verifyListener, error) {
	var tk struct {
		Ticket string `json:"ticket"`
	}
	if _, err := c.Do(ctx, http.MethodPost, "/api/v1/realtime/ticket", nil, &tk); err != nil {
		return nil, fmt.Errorf("ticket: %w", err)
	}
	u := "ws" + strings.TrimPrefix(c.base, "http") + "/api/v1/realtime/ws?ticket=" + url.QueryEscape(tk.Ticket)
	h := http.Header{}
	h.Set("Origin", c.origin)
	d := websocket.Dialer{Subprotocols: []string{realtime.Subprotocol}, HandshakeTimeout: 10 * time.Second}
	conn, _, err := d.DialContext(ctx, u, h)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	l := &verifyListener{conn: conn, done: make(chan struct{})}
	hello := make(chan struct{})
	go func() {
		defer close(l.done)
		var greeted bool
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var env realtime.Envelope
			if json.Unmarshal(data, &env) != nil {
				continue
			}
			switch env.Type {
			case realtime.EventHello:
				if !greeted {
					greeted = true
					close(hello)
				}
			case typ:
				l.n.Add(1)
			}
		}
	}()
	select {
	case <-hello:
		return l, nil
	case <-time.After(10 * time.Second):
		l.close()
		return nil, fmt.Errorf("no hello")
	}
}
