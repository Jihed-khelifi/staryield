package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
)

// CloudflareConfig configures the Cloudflare Email Service REST driver.
type CloudflareConfig struct {
	AccountID string
	APIToken  string
	// From is the sender, "Name <address>" or a bare address on a domain
	// onboarded to Email Sending. FromName, when set, overrides the name.
	From     string
	FromName string
	ReplyTo  string
	// BaseURL defaults to https://api.cloudflare.com/client/v4 (tests point
	// it at a fake server).
	BaseURL string
	Timeout time.Duration
}

// Cloudflare sends through POST /accounts/{account_id}/email/sending/send.
// Email Sending is a transactional-only service: callers must never route
// marketing e-mail through it (the outbox cancels such messages).
type Cloudflare struct {
	endpoint string
	token    string
	from     address
	replyTo  *address
	client   *http.Client
}

type address struct {
	Address string `json:"address"`
	Name    string `json:"name,omitempty"`
}

// NewCloudflare validates cfg and returns the driver.
func NewCloudflare(cfg CloudflareConfig) (*Cloudflare, error) {
	if strings.TrimSpace(cfg.AccountID) == "" || strings.TrimSpace(cfg.APIToken) == "" {
		return nil, errors.New("mail: cloudflare driver needs an account id and an API token")
	}
	from, err := parseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("mail: from address: %w", err)
	}
	if n := strings.TrimSpace(cfg.FromName); n != "" {
		from.Name = n
	}
	c := &Cloudflare{from: from, token: strings.TrimSpace(cfg.APIToken)}
	if strings.TrimSpace(cfg.ReplyTo) != "" {
		rt, err := parseAddress(cfg.ReplyTo)
		if err != nil {
			return nil, fmt.Errorf("mail: reply-to address: %w", err)
		}
		c.replyTo = &rt
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = "https://api.cloudflare.com/client/v4"
	}
	c.endpoint = base + "/accounts/" + strings.TrimSpace(cfg.AccountID) + "/email/sending/send"
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	c.client = &http.Client{
		Timeout: timeout,
		// The API never redirects; refusing keeps the bearer token on its host.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return c, nil
}

// Name implements the provider name convention.
func (*Cloudflare) Name() string { return "cloudflare" }

func parseAddress(s string) (address, error) {
	a, err := mail.ParseAddress(strings.TrimSpace(s))
	if err != nil {
		return address{}, err
	}
	return address{Address: a.Address, Name: a.Name}, nil
}

type cfRequest struct {
	To      string            `json:"to"`
	From    any               `json:"from"`
	ReplyTo any               `json:"reply_to,omitempty"`
	Subject string            `json:"subject"`
	Text    string            `json:"text,omitempty"`
	HTML    string            `json:"html,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

type cfResponse struct {
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	Result *struct {
		Delivered            []string `json:"delivered"`
		Queued               []string `json:"queued"`
		PermanentBounces     []string `json:"permanent_bounces"`
		SuppressedRecipients []string `json:"suppressed_recipients"`
		MessageID            string   `json:"message_id"`
	} `json:"result"`
}

func (a address) wire() any {
	if a.Name == "" {
		return a.Address
	}
	return a
}

// Send implements Mailer.
func (c *Cloudflare) Send(ctx context.Context, m Message) (Result, error) {
	body := cfRequest{To: m.To, From: c.from.wire(), Subject: m.Subject, Text: m.Text, HTML: m.HTML, Headers: m.Headers}
	if c.replyTo != nil {
		body.ReplyTo = c.replyTo.wire()
	}
	if m.Category != "" {
		if body.Headers == nil {
			body.Headers = map[string]string{}
		}
		body.Headers["X-Staryield-Category"] = m.Category
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return Result{}, &SendError{Reason: "encode", Retryable: false}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(raw))
	if err != nil {
		return Result{}, &SendError{Reason: "request", Retryable: false}
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := c.client.Do(req)
	if err != nil {
		// Network failure or timeout: the message may or may not have been
		// accepted; retrying risks a duplicate, never a loss.
		return Result{}, &SendError{Reason: "transport", Retryable: true}
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	var parsed cfResponse
	_ = json.Unmarshal(data, &parsed)

	if res.StatusCode >= 200 && res.StatusCode < 300 && parsed.Success {
		out := Result{}
		if r := parsed.Result; r != nil {
			out.MessageID = r.MessageID
			out.Suppressed = append(append([]string{}, r.PermanentBounces...), r.SuppressedRecipients...)
			out.Queued = len(r.Queued) > 0
		}
		return out, nil
	}
	se := &SendError{Status: res.StatusCode, Reason: "rejected"}
	if len(parsed.Errors) > 0 {
		se.Code = parsed.Errors[0].Code
		se.Reason = shortReason(parsed.Errors[0].Message)
	}
	switch {
	case res.StatusCode == http.StatusTooManyRequests || se.Code == 10004:
		se.Retryable = true
		se.RetryAfter = retryAfter(res.Header.Get("Retry-After"))
	case res.StatusCode >= 500:
		se.Retryable = true
	case res.StatusCode >= 200 && res.StatusCode < 300:
		// 2xx without success: treat as a transient provider error.
		se.Retryable = true
	}
	return Result{}, se
}

// shortReason keeps provider error identifiers (e.g.
// "email.sending.error.throttled") and drops anything else.
func shortReason(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 80 {
		s = s[:80]
	}
	for _, r := range s {
		if !(r == '.' || r == '_' || r == '-' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return "rejected"
		}
	}
	if s == "" {
		return "rejected"
	}
	return s
}

func retryAfter(h string) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0
	}
	if n, err := strconv.Atoi(h); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}
