package e2e

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
)

// A realtime ticket fetched by a device moments before that device is
// signed out (from another device) must not open a connection afterwards.
func TestRealtimeTicketOfRevokedDeviceIsRefused(t *testing.T) {
	e := newEnv(t)
	a := e.registerClient()
	b := e.loginAs(a.user.Email, strongPassword, uaIPhone)
	bDev := b.currentDevice()
	ticket := b.ticket() // issued while device B is still signed in

	a.must(http.StatusNoContent, http.MethodDelete, "/api/v1/me/sessions/"+bDev.ID.String(), nil, nil)

	u := "ws" + strings.TrimPrefix(e.srv.URL, "http") + "/api/v1/realtime/ws?ticket=" + url.QueryEscape(ticket)
	h := http.Header{}
	h.Set("Origin", origin)
	d := websocket.Dialer{Subprotocols: []string{realtime.Subprotocol}, HandshakeTimeout: 5 * time.Second}
	conn, res, err := d.Dial(u, h)
	if err == nil {
		_ = conn.Close()
		t.Fatal("a revoked device's ticket opened a connection")
	}
	if res == nil || res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("handshake status: %v %v", res, err)
	}

	// Device A's own tickets still work.
	w := a.connect("")
	if w.hello.Type != realtime.EventHello {
		t.Fatalf("hello: %+v", w.hello)
	}
}
