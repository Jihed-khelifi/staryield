package mail

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeCloudflare struct {
	mu       sync.Mutex
	requests []map[string]any
	auth     []string
	paths    []string
	status   int
	body     string
	header   map[string]string
	delay    time.Duration
}

func (f *fakeCloudflare) handler(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	f.mu.Lock()
	f.requests = append(f.requests, body)
	f.auth = append(f.auth, r.Header.Get("Authorization"))
	f.paths = append(f.paths, r.Method+" "+r.URL.Path)
	status, resp, delay := f.status, f.body, f.delay
	for k, v := range f.header {
		w.Header().Set(k, v)
	}
	f.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, resp)
}

func newFake(t *testing.T, status int, body string) (*fakeCloudflare, *Cloudflare) {
	t.Helper()
	f := &fakeCloudflare{status: status, body: body}
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	t.Cleanup(srv.Close)
	c, err := NewCloudflare(CloudflareConfig{AccountID: "acct123", APIToken: "secret-token", From: "Staryield <no-reply@staryield.net>",
		ReplyTo: "support@staryield.net", BaseURL: srv.URL, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return f, c
}

const okBody = `{"success":true,"errors":[],"messages":[],"result":{"delivered":["a@example.com"],"permanent_bounces":[],"queued":[],"message_id":"m-1"}}`

func TestCloudflareSendsDocumentedRequest(t *testing.T) {
	f, c := newFake(t, http.StatusOK, okBody)
	res, err := c.Send(context.Background(), Message{To: "a@example.com", Subject: "Hello", Text: "Hi", HTML: "<p>Hi</p>",
		Headers: map[string]string{"List-Unsubscribe": "<https://staryield.net/en/profile/settings>"}, Category: "notification"})
	if err != nil {
		t.Fatal(err)
	}
	if res.MessageID != "m-1" || len(res.Suppressed) != 0 {
		t.Fatalf("result: %+v", res)
	}
	if f.paths[0] != "POST /accounts/acct123/email/sending/send" {
		t.Fatalf("path: %s", f.paths[0])
	}
	if f.auth[0] != "Bearer secret-token" {
		t.Fatalf("auth header: %q", f.auth[0])
	}
	body := f.requests[0]
	if body["to"] != "a@example.com" || body["subject"] != "Hello" || body["text"] != "Hi" || body["html"] != "<p>Hi</p>" {
		t.Fatalf("body: %v", body)
	}
	from, _ := body["from"].(map[string]any)
	if from["address"] != "no-reply@staryield.net" || from["name"] != "Staryield" {
		t.Fatalf("from: %v", body["from"])
	}
	if body["reply_to"] != "support@staryield.net" {
		t.Fatalf("reply_to: %v", body["reply_to"])
	}
	headers, _ := body["headers"].(map[string]any)
	if headers["List-Unsubscribe"] == nil || headers["X-Staryield-Category"] != "notification" {
		t.Fatalf("headers: %v", headers)
	}
}

func TestCloudflareReportsPermanentBounces(t *testing.T) {
	_, c := newFake(t, http.StatusOK, `{"success":true,"errors":[],"result":{"delivered":[],"permanent_bounces":["a@example.com"],"queued":[]}}`)
	res, err := c.Send(context.Background(), Message{To: "a@example.com", Subject: "s", Text: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Suppressed) != 1 || res.Suppressed[0] != "a@example.com" {
		t.Fatalf("suppressed: %+v", res)
	}
	_, c = newFake(t, http.StatusOK, `{"success":true,"errors":[],"result":{"delivered":[],"permanent_bounces":[],"queued":[],"suppressed_recipients":["a@example.com"]}}`)
	if res, _ := c.Send(context.Background(), Message{To: "a@example.com", Subject: "s", Text: "t"}); len(res.Suppressed) != 1 {
		t.Fatalf("suppressed recipients not reported: %+v", res)
	}
}

func TestCloudflareClassifiesErrors(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		body      string
		retryable bool
		code      int
	}{
		{"throttled", http.StatusTooManyRequests, `{"success":false,"errors":[{"code":10004,"message":"email.sending.error.throttled"}]}`, true, 10004},
		{"internal", http.StatusInternalServerError, `{"success":false,"errors":[{"code":10002,"message":"email.sending.error.internal_server"}]}`, true, 10002},
		{"upstream", http.StatusServiceUnavailable, `{"success":false,"errors":[{"code":10100,"message":"email.sending.error.authentication.upstream"}]}`, true, 10100},
		{"schema", http.StatusBadRequest, `{"success":false,"errors":[{"code":10001,"message":"email.sending.error.invalid_request_schema"}]}`, false, 10001},
		{"unauthorized", http.StatusUnauthorized, `{"success":false,"errors":[{"code":10101,"message":"email.sending.error.authentication.unauthorized"}]}`, false, 10101},
		{"not entitled", http.StatusForbidden, `{"success":false,"errors":[{"code":10105,"message":"email.sending.error.authentication.not_entitled"}]}`, false, 10105},
		{"garbage", http.StatusBadGateway, `<html>bad gateway</html>`, true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, c := newFake(t, tc.status, tc.body)
			_, err := c.Send(context.Background(), Message{To: "a@example.com", Subject: "s", Text: "t"})
			var se *SendError
			if !errors.As(err, &se) {
				t.Fatalf("want SendError, got %v", err)
			}
			if se.Retryable != tc.retryable || se.Code != tc.code || se.Status != tc.status {
				t.Fatalf("classification: %+v", se)
			}
			if IsRetryable(err) != tc.retryable {
				t.Fatal("IsRetryable disagrees")
			}
			if strings.Contains(err.Error(), "secret-token") {
				t.Fatal("error leaks the API token")
			}
		})
	}
}

func TestCloudflareHonoursRetryAfter(t *testing.T) {
	f, c := newFake(t, http.StatusTooManyRequests, `{"success":false,"errors":[{"code":10004,"message":"email.sending.error.throttled"}]}`)
	f.header = map[string]string{"Retry-After": "42"}
	_, err := c.Send(context.Background(), Message{To: "a@example.com", Subject: "s", Text: "t"})
	if RetryAfter(err) != 42*time.Second {
		t.Fatalf("retry after: %v", RetryAfter(err))
	}
}

func TestCloudflareTimeoutIsRetryable(t *testing.T) {
	f, _ := newFake(t, http.StatusOK, okBody)
	f.delay = 300 * time.Millisecond
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	defer srv.Close()
	c, err := NewCloudflare(CloudflareConfig{AccountID: "a", APIToken: "t", From: "no-reply@staryield.net", BaseURL: srv.URL, Timeout: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Send(context.Background(), Message{To: "a@example.com", Subject: "s", Text: "t"})
	var se *SendError
	if !errors.As(err, &se) || !se.Retryable || se.Reason != "transport" {
		t.Fatalf("timeout classification: %v", err)
	}
}

func TestCloudflareDoesNotFollowRedirects(t *testing.T) {
	var hits int
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()
	c, err := NewCloudflare(CloudflareConfig{AccountID: "a", APIToken: "t", From: "no-reply@staryield.net", BaseURL: redirector.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Send(context.Background(), Message{To: "a@example.com", Subject: "s", Text: "t"}); err == nil {
		t.Fatal("a redirect must not count as success")
	}
	if hits != 0 {
		t.Fatal("redirect followed: the bearer token could leak to another host")
	}
}

func TestNewCloudflareValidatesConfig(t *testing.T) {
	if _, err := NewCloudflare(CloudflareConfig{APIToken: "t", From: "a@b.c"}); err == nil {
		t.Fatal("missing account id accepted")
	}
	if _, err := NewCloudflare(CloudflareConfig{AccountID: "a", APIToken: "t", From: "not an address"}); err == nil {
		t.Fatal("bad from accepted")
	}
	c, err := NewCloudflare(CloudflareConfig{AccountID: "a", APIToken: "t", From: "no-reply@staryield.net", FromName: "Staryield"})
	if err != nil || c.from.Name != "Staryield" {
		t.Fatalf("from name override: %v %+v", err, c)
	}
	if Name(c) != "cloudflare" || Name(NoopMailer{}) != "noop" {
		t.Fatal("provider names")
	}
}
