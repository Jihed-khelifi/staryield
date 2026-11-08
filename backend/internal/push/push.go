// Package push delivers Web Push notifications (RFC 8030 with VAPID,
// RFC 8292, and aes128gcm payload encryption, RFC 8291) to browsers.
//
// Browser endpoints are capability URLs chosen by the client, so they are an
// SSRF vector: only HTTPS endpoints on the known push services are accepted,
// redirects are never followed and connections to private networks are
// refused at dial time.
package push

import (
	"context"
	"crypto/ecdh"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Config holds the VAPID identity and sending limits.
type Config struct {
	// PublicKey and PrivateKey are the VAPID key pair, base64url encoded (an
	// uncompressed P-256 point and a 32-byte scalar). Generate with cmd/vapid.
	PublicKey  string
	PrivateKey string
	// Subject identifies the sender to push services: "mailto:…" or an
	// https URL.
	Subject string
	// TTL is how long push services keep an undelivered message.
	TTL time.Duration
	// Timeout bounds one push request.
	Timeout time.Duration
}

// Validate checks the key pair and subject.
func (c Config) Validate() error {
	pub, err := decodeKey(c.PublicKey)
	if err != nil || len(pub) != 65 || pub[0] != 0x04 {
		return errors.New("push: VAPID public key must be a base64url uncompressed P-256 point (65 bytes)")
	}
	if _, err := ecdh.P256().NewPublicKey(pub); err != nil {
		return errors.New("push: VAPID public key is not on the P-256 curve")
	}
	priv, err := decodeKey(c.PrivateKey)
	if err != nil || len(priv) != 32 {
		return errors.New("push: VAPID private key must be 32 base64url bytes")
	}
	key, err := ecdh.P256().NewPrivateKey(priv)
	if err != nil {
		return errors.New("push: VAPID private key is invalid")
	}
	if string(key.PublicKey().Bytes()) != string(pub) {
		return errors.New("push: VAPID public and private keys do not match")
	}
	if !strings.HasPrefix(c.Subject, "mailto:") && !strings.HasPrefix(c.Subject, "https://") {
		return errors.New("push: VAPID subject must be a mailto: or https:// URL")
	}
	return nil
}

func decodeKey(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "=")); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(s)
}

// allowedHosts are the push services of the major browsers; wildcard
// entries match sub-domains.
var allowedHosts = []string{
	"fcm.googleapis.com",                // Chrome, Edge (FCM), Opera, Samsung
	"updates.push.services.mozilla.com", // Firefox
	"web.push.apple.com",                // Safari
	"*.notify.windows.com",              // Edge (WNS)
}

// Validation errors.
var (
	ErrEndpoint    = errors.New("push: endpoint is not a supported push service URL")
	ErrUnsupported = errors.New("push: push service not supported")
	ErrKeys        = errors.New("push: invalid subscription keys")
)

// ValidateEndpoint checks a browser endpoint: an absolute https URL on an
// allow-listed push service, default port, no credentials, bounded length.
func ValidateEndpoint(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 1024 {
		return "", ErrEndpoint
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Opaque != "" {
		return "", ErrEndpoint
	}
	if p := u.Port(); p != "" && p != "443" {
		return "", ErrEndpoint
	}
	host := strings.ToLower(u.Hostname())
	if net.ParseIP(host) != nil {
		return "", ErrUnsupported
	}
	if !HostAllowed(host) {
		return "", ErrUnsupported
	}
	return u.String(), nil
}

// HostAllowed reports whether host is a supported push service.
func HostAllowed(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	for _, h := range allowedHosts {
		if strings.HasPrefix(h, "*.") {
			suffix := h[1:] // ".notify.windows.com"
			if strings.HasSuffix(host, suffix) && len(host) > len(suffix) {
				return true
			}
			continue
		}
		if host == h {
			return true
		}
	}
	return false
}

// ValidateKeys checks the subscription's ECDH public key (65-byte
// uncompressed P-256 point on the curve) and auth secret (16 bytes).
func ValidateKeys(p256dh, auth string) error {
	pk, err := decodeKey(p256dh)
	if err != nil || len(pk) != 65 || pk[0] != 0x04 {
		return ErrKeys
	}
	if _, err := ecdh.P256().NewPublicKey(pk); err != nil {
		return ErrKeys
	}
	a, err := decodeKey(auth)
	if err != nil || len(a) != 16 {
		return ErrKeys
	}
	return nil
}

// Subscription is a decrypted browser registration.
type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// Message is one notification for a subscription.
type Message struct {
	Payload []byte
	// Topic collapses pending messages with the same topic (≤ 32 URL-safe
	// characters).
	Topic   string
	Urgency string // very-low | low | normal | high
	// TTL is how long the push service keeps the message if the device is
	// offline; 0 uses the sender's configured TTL.
	TTL time.Duration
}

// Outcome classifies a push service response.
type Outcome int

// Outcomes.
const (
	Delivered Outcome = iota
	// Gone: the subscription expired or was revoked (404/410); delete it.
	Gone
	// Throttled: the push service asked to slow down (429).
	Throttled
	// Failed: any other error (counted towards dropping the subscription).
	Failed
)

// Result is the outcome of one send.
type Result struct {
	Outcome    Outcome
	Status     int
	RetryAfter time.Duration
}

// Sender sends Web Push messages.
type Sender interface {
	Send(ctx context.Context, sub Subscription, m Message) Result
}

// WebPush is the VAPID sender.
type WebPush struct {
	cfg     Config
	subject string
	client  *http.Client
}

// NewWebPush validates cfg and returns a sender. transport may be nil (a
// hardened default dialler) or a test double.
func NewWebPush(cfg Config, transport http.RoundTripper) (*WebPush, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.TTL <= 0 {
		cfg.TTL = time.Hour
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if transport == nil {
		transport = SafeTransport()
	}
	// webpush-go prefixes "mailto:" unless the subject is an https URL.
	subject := strings.TrimPrefix(cfg.Subject, "mailto:")
	return &WebPush{cfg: cfg, subject: subject, client: &http.Client{
		Timeout:   cfg.Timeout,
		Transport: transport,
		// Push services answer directly; a redirect could point anywhere.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

// PublicKey returns the VAPID public key for browsers (applicationServerKey).
func (w *WebPush) PublicKey() string { return w.cfg.PublicKey }

// Send implements Sender.
func (w *WebPush) Send(ctx context.Context, sub Subscription, m Message) Result {
	if _, err := ValidateEndpoint(sub.Endpoint); err != nil {
		return Result{Outcome: Gone}
	}
	urgency := webpush.UrgencyNormal
	switch m.Urgency {
	case "very-low":
		urgency = webpush.UrgencyVeryLow
	case "low":
		urgency = webpush.UrgencyLow
	case "high":
		urgency = webpush.UrgencyHigh
	}
	res, err := webpush.SendNotificationWithContext(ctx, m.Payload, &webpush.Subscription{
		Endpoint: sub.Endpoint, Keys: webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &webpush.Options{
		HTTPClient: w.client, Subscriber: w.subject, VAPIDPublicKey: w.cfg.PublicKey, VAPIDPrivateKey: w.cfg.PrivateKey,
		TTL: ttlSeconds(m.TTL, w.cfg.TTL), Urgency: urgency, Topic: topic(m.Topic),
	})
	if err != nil {
		return Result{Outcome: Failed}
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 16<<10))
	switch {
	case res.StatusCode >= 200 && res.StatusCode < 300:
		return Result{Outcome: Delivered, Status: res.StatusCode}
	case res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusGone:
		return Result{Outcome: Gone, Status: res.StatusCode}
	case res.StatusCode == http.StatusTooManyRequests:
		return Result{Outcome: Throttled, Status: res.StatusCode, RetryAfter: retryAfter(res.Header.Get("Retry-After"))}
	}
	return Result{Outcome: Failed, Status: res.StatusCode}
}

// ttlSeconds picks the message TTL (or the default when 0), rounded up to
// whole seconds and never below 1 s so a short TTL is not read as "none".
func ttlSeconds(msg, def time.Duration) int {
	d := msg
	if d <= 0 {
		d = def
	}
	return int((d + time.Second - 1) / time.Second)
}

// topic keeps a header-safe topic: URL-safe base64 alphabet, ≤ 32 chars.
func topic(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
		if b.Len() == 32 {
			break
		}
	}
	return b.String()
}

func retryAfter(h string) time.Duration {
	h = strings.TrimSpace(h)
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

// SafeTransport is an HTTP transport that refuses to connect to loopback,
// private, link-local or unspecified addresses (DNS rebinding defence on top
// of the host allow-list).
func SafeTransport() *http.Transport {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second, Control: func(_, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		ip := net.ParseIP(host)
		if ip == nil || !PublicIP(ip) {
			return fmt.Errorf("push: refusing to connect to non-public address %s", host)
		}
		return nil
	}}
	return &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}
}

// PublicIP reports whether ip is a globally routable unicast address.
func PublicIP(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() || isCGNAT(ip))
}

func isCGNAT(ip net.IP) bool {
	v4 := ip.To4()
	return v4 != nil && v4[0] == 100 && v4[1]&0xc0 == 64
}

// GenerateKeys returns a new VAPID key pair (base64url, unpadded).
func GenerateKeys() (publicKey, privateKey string, err error) {
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return "", "", err
	}
	return pub, priv, nil
}
