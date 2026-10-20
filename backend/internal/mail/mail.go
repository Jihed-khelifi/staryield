// Package mail sends transactional e-mail. The log driver prints messages
// for local development; production requires a real provider adapter.
package mail

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
)

// Message is an outgoing e-mail.
type Message struct {
	To      string
	Subject string
	Text    string
}

// Mailer sends messages.
type Mailer interface {
	Send(ctx context.Context, m Message) error
}

// LogMailer writes e-mails to the log (development only; refused in
// production because bodies contain one-time links).
type LogMailer struct{ From string }

// Send implements Mailer.
func (l LogMailer) Send(ctx context.Context, m Message) error {
	logger.FromContext(ctx).Info("mail (log driver)", zap.String("from", l.From), zap.String("to", m.To),
		zap.String("subject", m.Subject), zap.String("body", m.Text))
	return nil
}

// NoopMailer discards e-mails.
type NoopMailer struct{}

// Send implements Mailer.
func (NoopMailer) Send(ctx context.Context, m Message) error {
	logger.FromContext(ctx).Warn("mail: no provider configured; message discarded", zap.String("subject", m.Subject))
	return nil
}

// Recorder captures messages (tests).
type Recorder struct {
	mu   sync.Mutex
	sent []Message
}

// Send implements Mailer.
func (r *Recorder) Send(_ context.Context, m Message) error {
	r.mu.Lock()
	r.sent = append(r.sent, m)
	r.mu.Unlock()
	return nil
}

// Sent returns a copy of the captured messages.
func (r *Recorder) Sent() []Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Message(nil), r.sent...)
}
