package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/config"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/ratelimit"
)

// Subprotocol is the negotiated WebSocket subprotocol.
const Subprotocol = "staryield.v1"

// FrameHandler implements the business side of realtime connections. It is
// provided by the service layer, keeping this package transport-only.
type FrameHandler interface {
	// OnConnect runs after the connection is registered.
	OnConnect(ctx context.Context, c *Client)
	// HandleFrame executes a client intent and returns the ack payload.
	HandleFrame(ctx context.Context, c *Client, frameType string, data json.RawMessage) (any, error)
	// OnDisconnect runs after the connection is gone, with the sessions it
	// had joined.
	OnDisconnect(ctx context.Context, c *Client, joined []uuid.UUID)
}

// Authorizer re-validates the user when a connection opens (status may
// have changed since the ticket was issued).
type Authorizer func(ctx context.Context, userID uuid.UUID) (role string, err error)

// Server serves WebSocket and SSE connections.
type Server struct {
	hub       *Hub
	broker    Broker
	presence  Presence
	tickets   TicketStore
	handler   FrameHandler
	authorize Authorizer
	cfg       config.RealtimeConfig
	origins   map[string]bool
	log       *zap.Logger
	upgrader  websocket.Upgrader
	wg        sync.WaitGroup
}

// NewServer wires the transport.
func NewServer(hub *Hub, broker Broker, presence Presence, tickets TicketStore, handler FrameHandler,
	authorize Authorizer, cfg config.RealtimeConfig, allowedOrigins []string, log *zap.Logger) *Server {
	s := &Server{hub: hub, broker: broker, presence: presence, tickets: tickets, handler: handler,
		authorize: authorize, cfg: cfg, origins: map[string]bool{}, log: log}
	for _, o := range allowedOrigins {
		s.origins[strings.ToLower(o)] = true
	}
	s.upgrader = websocket.Upgrader{
		HandshakeTimeout:  10 * time.Second,
		ReadBufferSize:    4096,
		WriteBufferSize:   4096,
		Subprotocols:      []string{Subprotocol},
		CheckOrigin:       s.checkOrigin,
		EnableCompression: false,
		Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {
			writeJSONError(w, status, apperror.New(apperror.CodeBadRequest, status, "WebSocket handshake failed."))
		},
	}
	return s
}

// checkOrigin blocks Cross-Site WebSocket Hijacking: browsers always send
// Origin, so a page on another site cannot open a socket with our tickets.
func (s *Server) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // non-browser client; the single-use ticket still authenticates
	}
	return s.origins[strings.ToLower(origin)]
}

// Wait blocks until all connection goroutines have exited.
func (s *Server) Wait() { s.wg.Wait() }

// ServeWS authenticates with a one-time ticket and upgrades the connection.
func (s *Server) ServeWS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	if !s.checkOrigin(r) {
		log.Warn("realtime: websocket origin rejected", zap.String("origin", r.Header.Get("Origin")))
		writeJSONError(w, http.StatusForbidden, apperror.Forbidden("Origin not allowed."))
		return
	}
	userID, role, ok, err := s.tickets.Consume(ctx, r.URL.Query().Get("ticket"))
	if err != nil {
		log.Error("realtime: ticket lookup failed", zap.Error(err))
		writeJSONError(w, http.StatusServiceUnavailable, apperror.Unavailable(err))
		return
	}
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, apperror.Unauthenticated("Realtime ticket is invalid or expired."))
		return
	}
	if s.authorize != nil {
		if role, err = s.authorize(ctx, userID); err != nil {
			appErr := apperror.From(err)
			writeJSONError(w, appErr.Status, appErr)
			return
		}
	}
	resume := r.URL.Query().Get("resume")
	if _, valid := ParseStreamID(resume); !valid {
		resume = ""
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Warn("realtime: upgrade failed", zap.Error(err))
		return
	}
	client := newClient(userID, role, "ws", s.cfg.SendBuffer, resume != "", s.log.With(zap.String("request_id", r.Header.Get("X-Request-ID"))))
	if err := s.hub.Register(client); err != nil {
		code := CloseTooManyConns
		msg := "too many connections"
		if errors.Is(err, ErrShuttingDown) {
			code, msg = CloseGoingAway, "server restarting"
		}
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, msg), time.Now().Add(time.Second))
		_ = conn.Close()
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runWS(conn, client, resume)
	}()
}

func (s *Server) runWS(conn *websocket.Conn, c *Client, resume string) {
	connCtx, cancel := context.WithCancel(logger.WithContext(context.Background(), c.Log))
	defer cancel()
	defer func() {
		if rec := recover(); rec != nil {
			c.Log.Error("realtime: websocket panic", zap.Any("panic", rec), zap.Stack("stack"))
		}
	}()

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		s.writeWS(conn, c)
	}()

	s.open(connCtx, c, resume)
	go s.heartbeat(connCtx, c)
	s.readWS(connCtx, conn, c)

	c.Close(CloseNormal, "")
	<-writerDone
	_ = conn.Close()
	s.close(c)
}

// open registers presence, greets the client and replays missed events.
func (s *Server) open(ctx context.Context, c *Client, resume string) {
	if err := s.presence.Connect(ctx, c.ID, c.UserID); err != nil {
		c.Log.Error("realtime: presence connect failed", zap.Error(err))
	}
	c.SendEvent(EventHello, "", map[string]any{
		"connection_id":   c.ID,
		"user_id":         c.UserID,
		"role":            c.Role,
		"server_time":     time.Now().UTC(),
		"heartbeat_ms":    s.cfg.PingInterval.Milliseconds(),
		"protocol":        Subprotocol,
		"resumed":         resume != "",
		"max_frame_bytes": s.cfg.MaxFrameBytes,
	}, nil)
	if resume != "" {
		frames, gap, err := s.broker.Replay(ctx, c.UserID, resume, s.cfg.ReplayLimit)
		if err != nil {
			c.Log.Error("realtime: replay failed", zap.Error(err))
			gap = true
		}
		c.finishReplay(frames)
		if gap {
			c.SendEvent(EventResyncRequired, "", map[string]string{"reason": "events_unavailable"}, nil)
		}
		c.Log.Debug("realtime: replayed events", zap.Int("count", len(frames)), zap.Bool("gap", gap))
	}
	s.handler.OnConnect(ctx, c)
}

// close removes presence and notifies the service layer.
func (s *Server) close(c *Client) {
	s.hub.Unregister(c)
	ctx, cancel := context.WithTimeout(logger.WithContext(context.Background(), c.Log), 10*time.Second)
	defer cancel()
	joined, err := s.presence.Disconnect(ctx, c.ID, c.UserID)
	if err != nil {
		c.Log.Error("realtime: presence disconnect failed", zap.Error(err))
	}
	s.handler.OnDisconnect(ctx, c, joined)
	code, reason := c.closeInfo()
	c.Log.Info("realtime: connection closed", zap.Int("code", code), zap.String("reason", reason), zap.Int64("dropped", c.dropped.Load()))
}

// heartbeat refreshes presence so it never expires while the client lives.
func (s *Server) heartbeat(ctx context.Context, c *Client) {
	interval := s.cfg.PresenceTTL / 3
	if interval <= 0 {
		interval = 15 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-c.Done():
			return
		case <-ctx.Done():
			return
		case <-t.C:
			hctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			if err := s.presence.Heartbeat(hctx, c.ID, c.UserID); err != nil {
				c.Log.Warn("realtime: presence heartbeat failed", zap.Error(err))
			}
			cancel()
		}
	}
}

func (s *Server) writeWS(conn *websocket.Conn, c *Client) {
	ping := time.NewTicker(s.cfg.PingInterval)
	defer ping.Stop()
	write := func(frame []byte) error {
		_ = conn.SetWriteDeadline(time.Now().Add(s.cfg.WriteWait))
		return conn.WriteMessage(websocket.TextMessage, frame)
	}
	for {
		select {
		case f := <-c.send:
			if err := write(f.Data); err != nil {
				c.Log.Debug("realtime: write failed", zap.Error(err))
				c.Close(CloseNormal, "write failed")
				return
			}
		case <-ping.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(s.cfg.WriteWait)); err != nil {
				c.Log.Debug("realtime: ping failed", zap.Error(err))
				c.Close(CloseNormal, "ping failed")
				return
			}
		case <-c.Done():
			// Flush frames queued before closing (e.g. auth.revoked).
			for {
				select {
				case f := <-c.send:
					if write(f.Data) != nil {
						return
					}
					continue
				default:
				}
				break
			}
			code, reason := c.closeInfo()
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(time.Second))
			return
		}
	}
}

func (s *Server) readWS(ctx context.Context, conn *websocket.Conn, c *Client) {
	conn.SetReadLimit(s.cfg.MaxFrameBytes)
	extend := func() { _ = conn.SetReadDeadline(time.Now().Add(s.cfg.PongWait)) }
	extend()
	conn.SetPongHandler(func(string) error { extend(); return nil })

	limiter := ratelimit.NewLocal(s.cfg.FrameRate, s.cfg.FrameBurst)
	violations := 0
	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			switch {
			case websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived):
				c.Log.Debug("realtime: client closed", zap.Error(err))
			case errors.Is(err, websocket.ErrReadLimit):
				// gorilla already sent close 1009 (message too big).
				c.Log.Warn("realtime: frame too large")
				c.Close(CloseMessageTooBig, "frame too large")
			default:
				c.Log.Debug("realtime: read ended", zap.Error(err))
			}
			return
		}
		extend()
		select {
		case <-c.Done():
			return
		default:
		}
		if mt != websocket.TextMessage {
			c.Close(CloseUnsupported, "text frames only")
			return
		}
		if !limiter.Allow(time.Now()) {
			violations++
			c.SendError("", apperror.RateLimited(time.Second))
			if violations > 25 {
				c.Log.Warn("realtime: closing abusive connection (rate limit)")
				c.Close(ClosePolicyViolation, "rate limit exceeded")
				return
			}
			continue
		}
		s.dispatch(ctx, c, data)
	}
}

// dispatch decodes and executes one client frame; frames from a single
// connection are processed sequentially, preserving their order.
func (s *Server) dispatch(ctx context.Context, c *Client, data []byte) {
	var f ClientFrame
	if err := json.Unmarshal(data, &f); err != nil || f.Type == "" || len(f.Type) > 40 || len(f.Ref) > 64 {
		c.SendError(f.Ref, apperror.BadRequest("Malformed frame."))
		return
	}
	if f.Type == FramePing {
		c.SendEvent(EventPong, f.Ref, map[string]any{"server_time": time.Now().UTC()}, nil)
		return
	}
	fctx, cancel := context.WithTimeout(logger.With(ctx, zap.String("frame", f.Type)), 10*time.Second)
	defer cancel()
	result, err := s.safeHandle(fctx, c, f)
	if err != nil {
		c.SendError(f.Ref, err)
		return
	}
	if f.Ref != "" {
		c.SendEvent(EventAck, f.Ref, result, nil)
	}
}

func (s *Server) safeHandle(ctx context.Context, c *Client, f ClientFrame) (result any, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			c.Log.Error("realtime: frame handler panic", zap.Any("panic", rec), zap.Stack("stack"))
			err = apperror.Internal(fmt.Errorf("panic: %v", rec))
		}
	}()
	return s.handler.HandleFrame(ctx, c, f.Type, f.Data)
}

// ServeSSE streams events as text/event-stream for the authenticated user.
// SSE is the fallback when WebSockets are blocked; clients send intents over
// REST and resume with Last-Event-ID.
func (s *Server) ServeSSE(w http.ResponseWriter, r *http.Request, userID uuid.UUID, role string) {
	log := logger.FromContext(r.Context())
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, apperror.Internal(errors.New("streaming unsupported")))
		return
	}
	resume := r.Header.Get("Last-Event-ID")
	if resume == "" {
		resume = r.URL.Query().Get("resume")
	}
	if _, valid := ParseStreamID(resume); !valid {
		resume = ""
	}
	client := newClient(userID, role, "sse", s.cfg.SendBuffer, resume != "", log)
	if err := s.hub.Register(client); err != nil {
		status := http.StatusTooManyRequests
		if errors.Is(err, ErrShuttingDown) {
			status = http.StatusServiceUnavailable
		}
		writeJSONError(w, status, apperror.New(apperror.CodeRateLimited, status, "Too many open connections."))
		return
	}
	s.wg.Add(1)
	defer s.wg.Done()

	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	rc := http.NewResponseController(w)
	// The server-wide ReadTimeout would otherwise cancel this long-lived
	// request; liveness is enforced by keep-alive writes instead.
	_ = rc.SetReadDeadline(time.Time{})
	w.WriteHeader(http.StatusOK)

	ctx, cancel := context.WithCancel(logger.WithContext(r.Context(), client.Log))
	defer cancel()
	writeRaw := func(s string) error {
		_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if _, err := io.WriteString(w, s); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	if writeRaw("retry: 3000\n\n") != nil {
		s.close(client)
		return
	}

	s.open(ctx, client, resume)
	go s.heartbeat(ctx, client)

	keepAlive := time.NewTicker(s.cfg.PingInterval)
	defer keepAlive.Stop()
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-client.Done():
			// Flush remaining frames (e.g. server.shutdown, auth.revoked).
			for {
				select {
				case f := <-client.send:
					_ = writeRaw(formatSSE(f))
					continue
				default:
				}
				break
			}
			break loop
		case f := <-client.send:
			if err := writeRaw(formatSSE(f)); err != nil {
				break loop
			}
		case <-keepAlive.C:
			if err := writeRaw(": keep-alive\n\n"); err != nil {
				break loop
			}
		}
	}
	client.Close(CloseNormal, "")
	s.close(client)
}

func formatSSE(f outFrame) string {
	var b strings.Builder
	if f.ID != "" {
		b.WriteString("id: ")
		b.WriteString(f.ID)
		b.WriteByte('\n')
	}
	// Frames are single-line JSON, so one data: line suffices.
	b.WriteString("data: ")
	b.Write(f.Data)
	b.WriteString("\n\n")
	return b.String()
}

// RunBroker pumps broker deliveries into the hub until ctx ends.
func (s *Server) RunBroker(ctx context.Context) error {
	return s.broker.Run(ctx, s.hub.Dispatch)
}

func writeJSONError(w http.ResponseWriter, status int, e *apperror.Error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": e.Code, "message": e.Message}})
}
