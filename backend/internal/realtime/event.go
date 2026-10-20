// Package realtime delivers server events to browsers over WebSocket (primary)
// and Server-Sent Events (fallback).
//
// Delivery model:
//   - Durable events are appended to a per-user Redis Stream (bounded length
//     and TTL) and fanned out through Redis Pub/Sub to every API instance.
//     The stream entry ID doubles as the event ID, so a reconnecting client
//     resumes exactly after the last event it processed (WebSocket "resume"
//     query parameter or the SSE Last-Event-ID header).
//   - Ephemeral events (typing, presence) skip the stream.
//   - The server is authoritative: clients send intents (frames); state
//     changes are decided by services and then published as events.
package realtime

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// Server → client event types.
const (
	EventHello           = "hello"
	EventResyncRequired  = "resync_required"
	EventAck             = "ack"
	EventError           = "error"
	EventPong            = "pong"
	EventServerShutdown  = "server.shutdown"
	EventAuthRevoked     = "auth.revoked"
	EventPresenceUpdated = "presence.updated"

	EventSessionRequested    = "session.requested"
	EventSessionAccepted     = "session.accepted"
	EventSessionDeclined     = "session.declined"
	EventSessionExpired      = "session.expired"
	EventSessionCancelled    = "session.cancelled"
	EventSessionStarted      = "session.started"
	EventSessionBilled       = "session.billed"
	EventSessionLowBalance   = "session.low_balance"
	EventSessionDisconnected = "session.disconnected"
	EventSessionResumed      = "session.resumed"
	EventSessionEnded        = "session.ended"
	EventSessionSettled      = "session.settled"
	EventParticipant         = "session.participant"

	EventMessageCreated = "message.created"
	EventMessageRead    = "message.read"
	EventTyping         = "typing"

	EventWalletUpdated = "wallet.updated"

	EventCustomRequestCreated = "custom_request.created"
	EventCustomRequestUpdated = "custom_request.updated"
	EventPsychicStatusChanged = "psychic.status_changed"
)

// Client → server frame types.
const (
	FramePing         = "ping"
	FrameSessionJoin  = "session.join"
	FrameSessionLeave = "session.leave"
	FrameMessageSend  = "message.send"
	FrameTyping       = "typing"
	FrameMessageRead  = "message.read"
)

// Envelope is the wire format of every server → client event.
type Envelope struct {
	ID   string          `json:"id,omitempty"`
	Type string          `json:"type"`
	TS   time.Time       `json:"ts"`
	Ref  string          `json:"ref,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
	Err  *FrameError     `json:"error,omitempty"`
}

// FrameError is the error payload of an "error" event.
type FrameError struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	Details    map[string]any `json:"details,omitempty"`
	RetryAfter int            `json:"retry_after,omitempty"`
}

// ClientFrame is the wire format of every client → server frame.
type ClientFrame struct {
	Type string          `json:"type"`
	Ref  string          `json:"ref,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

// encode marshals an envelope without an ID (the ID is injected later).
func encode(eventType string, data any, ts time.Time) ([]byte, error) {
	var raw json.RawMessage
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		raw = b
	}
	return json.Marshal(Envelope{Type: eventType, TS: ts.UTC(), Data: raw})
}

// withID injects "id" as the first property of a JSON object produced by
// encode. Avoids re-marshalling on every fan-out.
func withID(id string, payload []byte) []byte {
	if id == "" || len(payload) < 2 || payload[0] != '{' {
		return payload
	}
	out := make([]byte, 0, len(payload)+len(id)+8)
	out = append(out, `{"id":"`...)
	out = append(out, id...)
	out = append(out, `",`...)
	return append(out, payload[1:]...)
}

// StreamID is a parsed Redis stream ID ("<ms>-<seq>").
type StreamID struct{ Ms, Seq uint64 }

// ParseStreamID parses a stream ID; ok is false when malformed.
func ParseStreamID(s string) (StreamID, bool) {
	msPart, seqPart, found := strings.Cut(s, "-")
	if !found || len(s) > 41 {
		return StreamID{}, false
	}
	ms, err := strconv.ParseUint(msPart, 10, 64)
	if err != nil {
		return StreamID{}, false
	}
	seq, err := strconv.ParseUint(seqPart, 10, 64)
	if err != nil {
		return StreamID{}, false
	}
	return StreamID{Ms: ms, Seq: seq}, true
}

// Less reports whether a precedes b.
func (a StreamID) Less(b StreamID) bool {
	return a.Ms < b.Ms || (a.Ms == b.Ms && a.Seq < b.Seq)
}

// String formats the ID.
func (a StreamID) String() string {
	return strconv.FormatUint(a.Ms, 10) + "-" + strconv.FormatUint(a.Seq, 10)
}

// IsAfter reports whether stream ID a is strictly after b. Invalid IDs are
// treated as "not after" so they are never delivered twice.
func IsAfter(a, b string) bool {
	if b == "" {
		return true
	}
	pa, okA := ParseStreamID(a)
	pb, okB := ParseStreamID(b)
	if !okA || !okB {
		return false
	}
	return pb.Less(pa)
}
