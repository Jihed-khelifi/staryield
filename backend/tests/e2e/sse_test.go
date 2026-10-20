package e2e

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
)

func websocketDial(u string, h http.Header) (*websocket.Conn, *http.Response, error) {
	d := websocket.Dialer{Subprotocols: []string{realtime.Subprotocol}, HandshakeTimeout: 5 * time.Second}
	return d.Dial(u, h)
}

type sseStream struct {
	events chan realtime.Envelope
}

// openSSE connects to the event stream with the client's cookies.
func openSSE(t *testing.T, ctx context.Context, c *client) *sseStream {
	t.Helper()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.e.srv.URL+"/api/v1/realtime/stream", nil)
	req.Header.Set("Origin", c.origin)
	req.Header.Set("Accept", "text/event-stream")
	httpClient := &http.Client{Jar: c.http.Jar}
	res, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("sse: %v", err)
	}
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("sse status %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	s := &sseStream{events: make(chan realtime.Envelope, 256)}
	go func() {
		defer res.Body.Close()
		sc := bufio.NewScanner(res.Body)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		var id string
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "id: "):
				id = strings.TrimPrefix(line, "id: ")
			case strings.HasPrefix(line, "data: "):
				var env realtime.Envelope
				if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &env) == nil {
					if env.ID == "" {
						env.ID = id
					}
					s.events <- env
				}
				id = ""
			}
		}
	}()
	return s
}

func (s *sseStream) next(t *testing.T, typ string) realtime.Envelope {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case e := <-s.events:
			if e.Type == typ {
				return e
			}
		case <-deadline:
			t.Fatalf("sse: timed out waiting for %s", typ)
		}
	}
}
