package realtime

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// revisions 6.0 (docs/platform-extensions.md §26.5): a filtered
// text/event-stream for one short-lived purpose (the purchase stream).
//
// It uses the same machinery as ServeSSE (hub registration, broker
// subscription so durable events published on any instance reach it, the same
// headers, keep-alive comments) but speaks only about what the caller asked
// for: an initial frame, the relayed events that pass Relay, optional polling
// and a hard lifetime. Frames are the realtime envelope without an id:
//
//	data: {"type":"purchase.updated","ts":"…","data":{…}}

// StreamEvent is one frame of a filtered stream.
type StreamEvent struct {
	Type string
	// Data is marshalled into the frame (json.RawMessage is written as is).
	Data any
}

// StreamSpec describes a filtered stream.
type StreamSpec struct {
	// Initial is the frame sent as soon as the connection is subscribed
	// (nil sends none).
	Initial *StreamEvent
	// Relay decides, for every durable or broadcast event delivered to the
	// user, whether and how it reaches the stream: it returns the frame to
	// write, or false to drop the event. auth.revoked and server.shutdown
	// always end the stream (they are never offered to Relay).
	Relay func(eventType string, data json.RawMessage) (StreamEvent, bool)
	// Terminal reports that the stream ends after this frame was written.
	Terminal func(StreamEvent) bool
	// Poll, when set, runs every PollEvery (n counts from 1) and may return a
	// frame to write (nil writes none). An error is logged and the stream
	// carries on.
	Poll      func(ctx context.Context, n int) (*StreamEvent, error)
	PollEvery time.Duration
	// MaxLifetime ends the stream after this long (zero: no limit) by writing
	// Timeout, when set.
	MaxLifetime time.Duration
	Timeout     *StreamEvent
}

// ServeStream serves a filtered stream to the authenticated user. Like
// ServeSSE it must be called from a route that is exempt from the request
// timeout.
func (s *Server) ServeStream(w http.ResponseWriter, r *http.Request, userID uuid.UUID, role string, spec StreamSpec) {
	s.serveSSE(w, r, userID, role, &spec)
}

// runStream subscribes the connection and runs the stream until a terminal
// frame, the lifetime, a close of the client or the server, or a failed write.
func (s *Server) runStream(ctx context.Context, c *Client, spec *StreamSpec, writeRaw func(string) error) {
	// The subscription is confirmed before anything is read or sent, so every
	// event published afterwards reaches this connection.
	c.subscribed = true // Subscribe takes a reference even when it times out
	if err := s.broker.Subscribe(ctx, c.UserID); err != nil {
		c.Log.Warn("realtime: stream subscription not confirmed; relying on polling", zap.Error(err))
	}
	var last []byte
	write := func(ev StreamEvent) (terminal bool, err error) {
		frame, payload, err := encodeStreamEvent(ev)
		if err != nil {
			c.Log.Error("realtime: marshal stream event", zap.Error(err), zap.String("type", ev.Type))
			return false, nil
		}
		terminal = spec.Terminal != nil && spec.Terminal(ev)
		if bytes.Equal(payload, last) && !terminal {
			return false, nil // the same state again (a relay after a poll)
		}
		last = payload
		return terminal, writeRaw("data: " + string(frame) + "\n\n")
	}
	if spec.Initial != nil {
		if terminal, err := write(*spec.Initial); err != nil || terminal {
			return
		}
	}
	keepAlive := time.NewTicker(s.cfg.PingInterval)
	defer keepAlive.Stop()
	var poll <-chan time.Time
	if spec.Poll != nil && spec.PollEvery > 0 {
		t := time.NewTicker(spec.PollEvery)
		defer t.Stop()
		poll = t.C
	}
	var lifetime <-chan time.Time
	if spec.MaxLifetime > 0 {
		t := time.NewTimer(spec.MaxLifetime)
		defer t.Stop()
		lifetime = t.C
	}
	polls := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.Done():
			// Tell the browser why it was closed (auth.revoked, server.shutdown).
			for {
				select {
				case f := <-c.send:
					var env struct {
						Type string `json:"type"`
					}
					if json.Unmarshal(f.Data, &env) == nil && (env.Type == EventAuthRevoked || env.Type == EventServerShutdown) {
						_ = writeRaw("data: " + string(f.Data) + "\n\n")
					}
					continue
				default:
				}
				return
			}
		case f := <-c.send:
			var env struct {
				Type string          `json:"type"`
				Data json.RawMessage `json:"data"`
			}
			if json.Unmarshal(f.Data, &env) != nil {
				continue
			}
			if env.Type == EventAuthRevoked || env.Type == EventServerShutdown {
				_ = writeRaw("data: " + string(f.Data) + "\n\n")
				return
			}
			if spec.Relay == nil {
				continue
			}
			ev, ok := spec.Relay(env.Type, env.Data)
			if !ok {
				continue
			}
			if terminal, err := write(ev); err != nil || terminal {
				return
			}
		case <-poll:
			polls++
			ev, err := spec.Poll(ctx, polls)
			if err != nil {
				c.Log.Warn("realtime: stream poll failed", zap.Error(err), zap.Int("n", polls))
				continue
			}
			if ev == nil {
				continue
			}
			if terminal, err := write(*ev); err != nil || terminal {
				return
			}
		case <-lifetime:
			if spec.Timeout != nil {
				_, _ = write(*spec.Timeout)
			}
			return
		case <-keepAlive.C:
			if err := writeRaw(": keep-alive\n\n"); err != nil {
				return
			}
		}
	}
}

// encodeStreamEvent returns the wire frame of an event and the bytes that
// identify its state (the frame without its timestamp), used to skip
// consecutive duplicates.
func encodeStreamEvent(ev StreamEvent) (frame, payload []byte, err error) {
	var raw json.RawMessage
	if ev.Data != nil {
		if r, ok := ev.Data.(json.RawMessage); ok {
			raw = r
		} else if raw, err = json.Marshal(ev.Data); err != nil {
			return nil, nil, err
		}
	}
	payload, err = json.Marshal(Envelope{Type: ev.Type, Data: raw})
	if err != nil {
		return nil, nil, err
	}
	frame, err = json.Marshal(Envelope{Type: ev.Type, TS: time.Now().UTC(), Data: raw})
	return frame, payload, err
}

// closeStream leaves the hub and the broker subscription. A filtered stream
// has no presence and no business hook to run.
func (s *Server) closeStream(c *Client) {
	s.hub.Unregister(c)
	if c.subscribed {
		s.broker.Unsubscribe(c.UserID)
	}
	code, reason := c.closeInfo()
	c.Log.Debug("realtime: stream closed", zap.Int("code", code), zap.String("reason", reason))
}
