package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/config"
)

func render(t *testing.T, err error) (*httptest.ResponseRecorder, ErrorBody) {
	t.Helper()
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
	c.Response().Header().Set("X-Request-ID", "req-1")
	ErrorHandler(err, c)
	var body ErrorBody
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec, body
}

func TestErrorHandlerShapes(t *testing.T) {
	rec, body := render(t, apperror.Validation(map[string]string{"email": "bad"}))
	if rec.Code != http.StatusUnprocessableEntity || body.Error.Code != apperror.CodeValidation || body.Error.RequestID != "req-1" || body.Error.Details == nil {
		t.Fatalf("validation: %d %+v", rec.Code, body)
	}
	rec, body = render(t, errors.New("database password=hunter2 leaked"))
	if rec.Code != http.StatusInternalServerError || body.Error.Code != apperror.CodeInternal || body.Error.Message == "database password=hunter2 leaked" {
		t.Fatalf("internal errors must be opaque: %+v", body)
	}
	rec, _ = render(t, apperror.RateLimited(1500*time.Millisecond))
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "2" {
		t.Fatalf("retry-after: %d %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	rec, body = render(t, echo.ErrNotFound)
	if rec.Code != http.StatusNotFound || body.Error.Code != apperror.CodeNotFound {
		t.Fatalf("echo 404: %+v", body)
	}
	rec, _ = render(t, echo.ErrMethodNotAllowed)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("405: %d", rec.Code)
	}
}

func TestCookieNames(t *testing.T) {
	if n := CookieNamesFor(config.SecurityConfig{CookieSecure: true}); n.Access != "__Host-sy_at" || n.Refresh != "__Secure-sy_rt" {
		t.Fatalf("secure names: %+v", n)
	}
	if n := CookieNamesFor(config.SecurityConfig{CookieSecure: true, CookieDomain: ".staryield.net"}); n.Access != "__Secure-sy_at" {
		t.Fatalf("domain cookies cannot use __Host-: %+v", n)
	}
	if n := CookieNamesFor(config.SecurityConfig{}); n.Access != "sy_at" {
		t.Fatalf("dev names: %+v", n)
	}
}

func TestParseSize(t *testing.T) {
	for in, want := range map[string]int64{"64KB": 64 << 10, "2MB": 2 << 20, "512B": 512, "bogus": 7, "": 7} {
		if got := parseSize(in, 7); got != want {
			t.Errorf("%q: %d", in, got)
		}
	}
}

func TestIPExtractor(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.5:1234"
	r.Header.Set("CF-Connecting-IP", "198.51.100.7")
	r.Header.Set("X-Forwarded-For", "203.0.113.1")
	if ip := ipExtractor(config.HTTPConfig{IPSource: "direct"})(r); ip != "10.0.0.5" {
		t.Fatalf("direct: %s", ip)
	}
	if ip := ipExtractor(config.HTTPConfig{IPSource: "cloudflare", TrustedProxies: []string{"10.0.0.0/8"}})(r); ip != "198.51.100.7" {
		t.Fatalf("trusted cloudflare: %s", ip)
	}
	if ip := ipExtractor(config.HTTPConfig{IPSource: "cloudflare", TrustedProxies: []string{"192.168.0.0/16"}})(r); ip != "10.0.0.5" {
		t.Fatalf("untrusted proxy header must be ignored: %s", ip)
	}

	// Railway's edge sets X-Real-IP; a client-supplied value from an
	// untrusted peer is ignored, as is garbage.
	r.Header.Set("X-Real-IP", "198.51.100.9")
	if ip := ipExtractor(config.HTTPConfig{IPSource: "x-real-ip", TrustedProxies: []string{"10.0.0.0/8"}})(r); ip != "198.51.100.9" {
		t.Fatalf("trusted x-real-ip: %s", ip)
	}
	r.RemoteAddr = "203.0.113.50:4321"
	if ip := ipExtractor(config.HTTPConfig{IPSource: "x-real-ip", TrustedProxies: []string{"10.0.0.0/8"}})(r); ip != "203.0.113.50" {
		t.Fatalf("x-real-ip from an untrusted peer must be ignored: %s", ip)
	}
	r.RemoteAddr = "10.0.0.5:1234"
	r.Header.Set("X-Real-IP", "not-an-ip")
	if ip := ipExtractor(config.HTTPConfig{IPSource: "x-real-ip", TrustedProxies: []string{"10.0.0.0/8"}})(r); ip != "10.0.0.5" {
		t.Fatalf("invalid x-real-ip must be ignored: %s", ip)
	}
}
