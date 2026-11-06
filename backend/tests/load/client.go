//go:build load

package load

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"time"
)

// sharedTransport keeps connections warm across all virtual users so the test
// measures the API, not TCP and TLS handshakes or ephemeral port exhaustion.
var sharedTransport = &http.Transport{
	Proxy:               nil,
	DialContext:         (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
	MaxIdleConns:        2048,
	MaxIdleConnsPerHost: 1024,
	MaxConnsPerHost:     0,
	IdleConnTimeout:     90 * time.Second,
	ForceAttemptHTTP2:   false,
}

// Client is one virtual user: its own cookie jar (session) and CSRF token,
// sending the headers the web app sends.
type Client struct {
	base   string
	origin string
	http   *http.Client

	mu   sync.Mutex
	csrf string
}

// NewClient returns an anonymous virtual user.
func NewClient(base, origin string) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{base: base, origin: origin, http: &http.Client{Jar: jar, Timeout: 15 * time.Second, Transport: sharedTransport}}
}

// APIError is a non-2xx answer.
type APIError struct {
	Status int
	Code   string
	Body   string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("HTTP %d %s", e.Status, e.Code)
	}
	return fmt.Sprintf("HTTP %d %.120s", e.Status, e.Body)
}

func (c *Client) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	tok := c.csrf
	c.mu.Unlock()
	if tok != "" {
		return tok, nil
	}
	var out struct {
		Token string `json:"csrf_token"`
	}
	if _, err := c.send(ctx, http.MethodGet, "/api/v1/auth/csrf", nil, &out, ""); err != nil {
		return "", err
	}
	c.mu.Lock()
	if c.csrf == "" {
		c.csrf = out.Token
	}
	tok = c.csrf
	c.mu.Unlock()
	return tok, nil
}

// Do sends a JSON request like the frontend (CSRF on unsafe methods, one
// retry after a rotated CSRF token) and decodes a 2xx body into out.
func (c *Client) Do(ctx context.Context, method, path string, payload, out any) (int, error) {
	csrf := ""
	if method != http.MethodGet && method != http.MethodHead {
		tok, err := c.token(ctx)
		if err != nil {
			return 0, err
		}
		csrf = tok
	}
	status, err := c.send(ctx, method, path, payload, out, csrf)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Code == "csrf_failed" {
		c.mu.Lock()
		c.csrf = ""
		c.mu.Unlock()
		tok, terr := c.token(ctx)
		if terr != nil {
			return status, terr
		}
		return c.send(ctx, method, path, payload, out, tok)
	}
	return status, err
}

func (c *Client) send(ctx context.Context, method, path string, payload, out any, csrf string) (int, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Origin", c.origin)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if tok := res.Header.Get("X-CSRF-Token"); tok != "" {
		c.mu.Lock()
		c.csrf = tok
		c.mu.Unlock()
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return res.StatusCode, err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		var env struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &env)
		return res.StatusCode, &APIError{Status: res.StatusCode, Code: env.Error.Code, Body: string(raw)}
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return res.StatusCode, fmt.Errorf("decode %s: %w", path, err)
		}
	}
	return res.StatusCode, nil
}

// Login signs the virtual user in. Like the web app, it retries the
// retryable answers (503 while password hashing is saturated, 429) with
// jittered backoff, so a burst of sign-ins is admitted gradually.
func (c *Client) Login(ctx context.Context, email, password string) error {
	backoff := 250 * time.Millisecond
	for attempt := 0; ; attempt++ {
		_, err := c.Do(ctx, http.MethodPost, "/api/v1/auth/login", map[string]any{"email": email, "password": password}, nil)
		var apiErr *APIError
		if err == nil || attempt >= 8 || !errors.As(err, &apiErr) ||
			(apiErr.Status != http.StatusServiceUnavailable && apiErr.Status != http.StatusTooManyRequests) {
			return err
		}
		wait := backoff + time.Duration(rand.Int64N(int64(backoff)))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		backoff = min(2*backoff, 4*time.Second)
	}
}

// Get is a GET that only checks the status.
func (c *Client) Get(ctx context.Context, path string) (int, error) {
	return c.Do(ctx, http.MethodGet, path, nil, nil)
}
