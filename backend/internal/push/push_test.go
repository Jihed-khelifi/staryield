package push

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func testConfig(t *testing.T) Config {
	t.Helper()
	pub, priv, err := GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}
	return Config{PublicKey: pub, PrivateKey: priv, Subject: "mailto:support@staryield.net", TTL: time.Hour}
}

// browserKeys returns a subscription key pair like PushSubscription.getKey.
func browserKeys(t *testing.T) (p256dh, auth string) {
	t.Helper()
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, 16)
	_, _ = rand.Read(secret)
	return base64.RawURLEncoding.EncodeToString(k.PublicKey().Bytes()), base64.RawURLEncoding.EncodeToString(secret)
}

func TestValidateEndpoint(t *testing.T) {
	ok := []string{
		"https://fcm.googleapis.com/fcm/send/abc:def",
		"https://updates.push.services.mozilla.com/wpush/v2/gAAAA",
		"https://web.push.apple.com/QGuQyavXutnMH",
		"https://wns2-par02p.notify.windows.com/w/?token=BQYAAA",
		"https://fcm.googleapis.com:443/fcm/send/x",
	}
	for _, e := range ok {
		if _, err := ValidateEndpoint(e); err != nil {
			t.Errorf("%s rejected: %v", e, err)
		}
	}
	bad := map[string]error{
		"http://fcm.googleapis.com/fcm/send/x":                    ErrEndpoint,
		"https://fcm.googleapis.com:8443/x":                       ErrEndpoint,
		"https://user:pw@fcm.googleapis.com/x":                    ErrEndpoint,
		"https://127.0.0.1/x":                                     ErrUnsupported,
		"https://[::1]/x":                                         ErrUnsupported,
		"https://169.254.169.254/latest/meta-data":                ErrUnsupported,
		"https://evil.example/fcm.googleapis.com":                 ErrUnsupported,
		"https://fcm.googleapis.com.evil.example/x":               ErrUnsupported,
		"https://notify.windows.com/x":                            ErrUnsupported,
		"https://push.apple.com/x":                                ErrUnsupported,
		"fcm.googleapis.com/fcm/send/x":                           ErrEndpoint,
		"":                                                        ErrEndpoint,
		"https://fcm.googleapis.com/" + strings.Repeat("a", 1100): ErrEndpoint,
	}
	for e, want := range bad {
		if _, err := ValidateEndpoint(e); err != want {
			t.Errorf("%q: got %v, want %v", e, err, want)
		}
	}
}

func TestValidateKeys(t *testing.T) {
	p, a := browserKeys(t)
	if err := ValidateKeys(p, a); err != nil {
		t.Fatal(err)
	}
	if ValidateKeys(p, base64.RawURLEncoding.EncodeToString(make([]byte, 8))) == nil {
		t.Fatal("short auth secret accepted")
	}
	// A 65-byte value that is not a point on the curve.
	notOnCurve := make([]byte, 65)
	notOnCurve[0] = 0x04
	if ValidateKeys(base64.RawURLEncoding.EncodeToString(notOnCurve), a) == nil {
		t.Fatal("off-curve key accepted")
	}
	if ValidateKeys("%%%", a) == nil {
		t.Fatal("garbage accepted")
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := testConfig(t)
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	other := testConfig(t)
	mismatch := cfg
	mismatch.PrivateKey = other.PrivateKey
	if mismatch.Validate() == nil {
		t.Fatal("mismatched key pair accepted")
	}
	bad := cfg
	bad.Subject = "support@staryield.net"
	if bad.Validate() == nil {
		t.Fatal("subject without scheme accepted")
	}
}

type captureTransport struct {
	mu       sync.Mutex
	requests []*http.Request
	bodies   [][]byte
	status   int
	header   http.Header
}

func (c *captureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(r.Body)
	c.mu.Lock()
	c.requests = append(c.requests, r)
	c.bodies = append(c.bodies, body)
	status, header := c.status, c.header
	c.mu.Unlock()
	if header == nil {
		header = http.Header{}
	}
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
}

func TestSendEncryptsAndSignsAndClassifies(t *testing.T) {
	cfg := testConfig(t)
	tr := &captureTransport{status: http.StatusCreated}
	w, err := NewWebPush(cfg, tr)
	if err != nil {
		t.Fatal(err)
	}
	p, a := browserKeys(t)
	sub := Subscription{Endpoint: "https://fcm.googleapis.com/fcm/send/abc", P256dh: p, Auth: a}
	payload := []byte(`{"title":"Hello","body":"World"}`)
	res := w.Send(context.Background(), sub, Message{Payload: payload, Topic: "chat.requested:1234", Urgency: "high"})
	if res.Outcome != Delivered {
		t.Fatalf("outcome: %+v", res)
	}
	req := tr.requests[0]
	if req.URL.String() != sub.Endpoint || req.Method != http.MethodPost {
		t.Fatalf("request: %s %s", req.Method, req.URL)
	}
	if !strings.HasPrefix(req.Header.Get("Authorization"), "vapid t=") || req.Header.Get("Content-Encoding") != "aes128gcm" {
		t.Fatalf("headers: %v", req.Header)
	}
	if req.Header.Get("TTL") != "3600" || req.Header.Get("Urgency") != "high" || req.Header.Get("Topic") != "chatrequested1234" {
		t.Fatalf("delivery headers: ttl=%q urgency=%q topic=%q", req.Header.Get("TTL"), req.Header.Get("Urgency"), req.Header.Get("Topic"))
	}
	if strings.Contains(string(tr.bodies[0]), "Hello") {
		t.Fatal("payload sent in clear")
	}
	for status, want := range map[int]Outcome{http.StatusGone: Gone, http.StatusNotFound: Gone, http.StatusTooManyRequests: Throttled,
		http.StatusBadRequest: Failed, http.StatusInternalServerError: Failed} {
		tr.status = status
		tr.header = http.Header{"Retry-After": []string{"30"}}
		res := w.Send(context.Background(), sub, Message{Payload: payload})
		if res.Outcome != want {
			t.Errorf("status %d → %v, want %v", status, res.Outcome, want)
		}
		if status == http.StatusTooManyRequests && res.RetryAfter != 30*time.Second {
			t.Errorf("retry-after: %v", res.RetryAfter)
		}
	}
	// An endpoint that is (no longer) allowed is treated as gone, never sent.
	before := len(tr.requests)
	if res := w.Send(context.Background(), Subscription{Endpoint: "https://127.0.0.1/x", P256dh: p, Auth: a}, Message{Payload: payload}); res.Outcome != Gone {
		t.Fatalf("disallowed endpoint: %+v", res)
	}
	if len(tr.requests) != before {
		t.Fatal("request sent to a disallowed endpoint")
	}
}

func TestSafeTransportRefusesPrivateAddresses(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, err = SafeTransport().DialContext(context.Background(), "tcp", ln.Addr().String())
	if err == nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("loopback dial allowed: %v", err)
	}
	for _, ip := range []string{"10.0.0.1", "192.168.1.1", "172.16.0.1", "169.254.169.254", "100.64.0.1", "::1", "fe80::1", "0.0.0.0"} {
		if PublicIP(net.ParseIP(ip)) {
			t.Errorf("%s considered public", ip)
		}
	}
	if !PublicIP(net.ParseIP("142.250.74.110")) {
		t.Fatal("public address rejected")
	}
}

// A message TTL (chat requests live only as long as the request) overrides
// the configured default; it is rounded up so a short TTL never means "none".
func TestSendTTLFromMessageOrDefault(t *testing.T) {
	cfg := testConfig(t)
	tr := &captureTransport{status: http.StatusCreated}
	w, err := NewWebPush(cfg, tr)
	if err != nil {
		t.Fatal(err)
	}
	p, a := browserKeys(t)
	sub := Subscription{Endpoint: "https://fcm.googleapis.com/fcm/send/ttl", P256dh: p, Auth: a}
	cases := []struct {
		ttl  time.Duration
		want string
	}{{0, "3600"}, {90 * time.Second, "90"}, {1500 * time.Millisecond, "2"}, {time.Millisecond, "1"}}
	for i, c := range cases {
		if res := w.Send(context.Background(), sub, Message{Payload: []byte(`{}`), TTL: c.ttl}); res.Outcome != Delivered {
			t.Fatalf("ttl %v: %+v", c.ttl, res)
		}
		if got := tr.requests[i].Header.Get("TTL"); got != c.want {
			t.Errorf("ttl %v: header %q, want %q", c.ttl, got, c.want)
		}
	}
}
