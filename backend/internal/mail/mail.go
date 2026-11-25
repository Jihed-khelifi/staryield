// Package mail renders and sends transactional e-mail. Providers implement
// Mailer: Cloudflare Email Service in production, a log driver for local
// development, a no-op driver and a recorder for tests. Messages are never
// sent from request handlers directly: services enqueue them in the e-mail
// outbox (service.MailService), whose workers deliver with retries.
package mail

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
)

// Message is an outgoing e-mail.
type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
	// Headers are extra headers (List-Unsubscribe, Auto-Submitted, X-*).
	Headers map[string]string
	// Category is the delivery category ("security", "billing", …); some
	// providers tag messages with it.
	Category string
	// Attachments are files sent with the message (revisions 6.0, §26.6: the
	// PDF of an invoice or credit note).
	Attachments []Attachment
}

// Attachment is a file sent with a message.
type Attachment struct {
	Filename    string
	ContentType string
	Content     []byte
}

// AttachmentRef names an attachment without carrying it: the e-mail outbox
// stores references (so nothing binary is queued) and the sending service
// resolves them to files when the message is delivered.
type AttachmentRef struct {
	// Kind selects the resolver ("invoice").
	Kind string `json:"kind"`
	// ID identifies the document for that resolver.
	ID string `json:"id"`
}

// attachmentSummary is how drivers that do not deliver (log, no-op) describe
// attachments: names and sizes, never contents.
func attachmentSummary(atts []Attachment) []zap.Field {
	if len(atts) == 0 {
		return nil
	}
	names := make([]string, 0, len(atts))
	sizes := make([]int, 0, len(atts))
	for _, a := range atts {
		names = append(names, a.Filename)
		sizes = append(sizes, len(a.Content))
	}
	return []zap.Field{zap.Strings("attachments", names), zap.Ints("attachment_bytes", sizes)}
}

// Result is a provider's per-message outcome.
type Result struct {
	// MessageID is the provider's identifier, when it returns one.
	MessageID string
	// Suppressed lists recipients the provider refused permanently (hard
	// bounces, suppression list). They must not be retried.
	Suppressed []string
	// Queued reports the provider accepted the message for later delivery.
	Queued bool
	// Discarded reports a driver that accepted the message without delivering
	// it (the no-op driver): the outbox records it as cancelled with reason
	// no_provider, never as sent.
	Discarded bool
}

// Mailer sends one message.
type Mailer interface {
	Send(ctx context.Context, m Message) (Result, error)
}

// Name returns a short provider name for records ("cloudflare", "log"…).
func Name(m Mailer) string {
	if n, ok := m.(interface{ Name() string }); ok {
		return n.Name()
	}
	return "custom"
}

// SendError is a provider failure. Retryable errors (rate limits, provider
// outages, timeouts) are retried with backoff; others fail the message.
type SendError struct {
	Status     int           // HTTP status (0 for transport errors)
	Code       int           // provider error code, when known
	Reason     string        // short machine-readable reason, never secrets
	Retryable  bool          // safe to retry
	RetryAfter time.Duration // provider-requested delay (0 = unknown)
	// Config marks a failure of the provider account rather than of the
	// message (credentials, entitlement, plan restrictions): the operator must
	// act, and the message is retried so it goes out once that is fixed.
	Config bool
	// Hint tells the operator what to check (logged and shown in the admin
	// console; never contains secrets).
	Hint string
}

// IsConfigError reports whether err is a provider account or configuration
// failure (see SendError.Config).
func IsConfigError(err error) bool {
	var se *SendError
	return errors.As(err, &se) && se.Config
}

// HintOf returns the operator hint carried by err, if any.
func HintOf(err error) string {
	var se *SendError
	if errors.As(err, &se) {
		return se.Hint
	}
	return ""
}

func (e *SendError) Error() string {
	if e.Code != 0 {
		return fmt.Sprintf("mail: send failed (status %d, code %d): %s", e.Status, e.Code, e.Reason)
	}
	return fmt.Sprintf("mail: send failed (status %d): %s", e.Status, e.Reason)
}

// IsRetryable reports whether err is worth retrying. Unknown errors are
// treated as transient (network, context deadline).
func IsRetryable(err error) bool {
	var se *SendError
	if errors.As(err, &se) {
		return se.Retryable
	}
	return err != nil
}

// RetryAfter returns the provider-requested delay carried by err, if any.
func RetryAfter(err error) time.Duration {
	var se *SendError
	if errors.As(err, &se) {
		return se.RetryAfter
	}
	return 0
}

// LogMailer writes e-mails to the log (development only; refused in
// production because bodies contain one-time links).
type LogMailer struct{ From string }

// Name implements the provider name convention.
func (LogMailer) Name() string { return "log" }

// Send implements Mailer.
func (l LogMailer) Send(ctx context.Context, m Message) (Result, error) {
	fields := append([]zap.Field{zap.String("from", l.From), zap.String("to", m.To),
		zap.String("subject", m.Subject), zap.String("category", m.Category), zap.String("body", m.Text)}, attachmentSummary(m.Attachments)...)
	logger.FromContext(ctx).Info("mail (log driver)", fields...)
	return Result{}, nil
}

// NoopMailer discards e-mails. The outbox records each one as cancelled
// (reason no_provider), so a deployment without a provider is visible in the
// outbox and in the admin console instead of looking delivered.
type NoopMailer struct{}

// Name implements the provider name convention.
func (NoopMailer) Name() string { return "noop" }

// Send implements Mailer.
func (NoopMailer) Send(ctx context.Context, m Message) (Result, error) {
	logger.FromContext(ctx).Warn("mail: no provider configured; message discarded",
		append([]zap.Field{zap.String("category", m.Category)}, attachmentSummary(m.Attachments)...)...)
	return Result{Discarded: true}, nil
}

// Recorder captures messages (tests). Fail, when set, decides the outcome
// for a message (provider failure simulation).
type Recorder struct {
	mu   sync.Mutex
	sent []Message
	Fail func(Message) (Result, error)
}

// Name implements the provider name convention.
func (*Recorder) Name() string { return "recorder" }

// Send implements Mailer.
func (r *Recorder) Send(_ context.Context, m Message) (Result, error) {
	r.mu.Lock()
	fail := r.Fail
	r.mu.Unlock()
	if fail != nil {
		if res, err := fail(m); err != nil || len(res.Suppressed) > 0 {
			return res, err
		}
	}
	r.mu.Lock()
	r.sent = append(r.sent, m)
	r.mu.Unlock()
	return Result{MessageID: fmt.Sprintf("rec-%d", len(r.sent))}, nil
}

// SetFail installs (or clears, with nil) a failure simulation.
func (r *Recorder) SetFail(fn func(Message) (Result, error)) {
	r.mu.Lock()
	r.Fail = fn
	r.mu.Unlock()
}

// Sent returns a copy of the captured messages.
func (r *Recorder) Sent() []Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Message(nil), r.sent...)
}

// Reset forgets captured messages.
func (r *Recorder) Reset() {
	r.mu.Lock()
	r.sent = nil
	r.mu.Unlock()
}
