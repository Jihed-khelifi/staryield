package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/ratelimit"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// run executes mws around a handler and returns the recorder and the error
// the chain produced.
func run(t *testing.T, req *http.Request, h echo.HandlerFunc, mws ...echo.MiddlewareFunc) (*httptest.ResponseRecorder, error) {
	t.Helper()
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return rec, h(c)
}

func okHandler(c echo.Context) error { return c.String(http.StatusOK, "ok") }

func code(err error) apperror.Code {
	if e, ok := apperror.As(err); ok {
		return e.Code
	}
	return ""
}

func TestRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec, _ := run(t, req, okHandler, RequestID())
	if _, err := uuid.Parse(rec.Header().Get(HeaderRequestID)); err != nil {
		t.Fatal("generated id must be a uuid")
	}
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderRequestID, "client-trace-0001")
	rec, _ = run(t, req, okHandler, RequestID())
	if rec.Header().Get(HeaderRequestID) != "client-trace-0001" {
		t.Fatal("valid inbound id must be kept")
	}
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderRequestID, "bad id\r\ninjected")
	rec, _ = run(t, req, okHandler, RequestID())
	if strings.Contains(rec.Header().Get(HeaderRequestID), "injected") {
		t.Fatal("header injection")
	}
}

func TestRequestContextCarriesLoggerAndMeta(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "fr-FR,fr;q=0.9")
	_, err := run(t, req, func(c echo.Context) error {
		meta := reqctx.MetaFrom(c.Request().Context())
		if meta.Locale != "fr" || logger.FromContext(c.Request().Context()) == zap.L() {
			return errors.New("context not populated")
		}
		return nil
	}, RequestID(), RequestContext(zap.NewNop(), "secret"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestAnonymizeIP(t *testing.T) {
	if AnonymizeIP("203.0.113.77") != "203.0.113.0" || AnonymizeIP("2001:db8:1234:5678::1") != "2001:db8:1234::" || AnonymizeIP("x") != "" {
		t.Fatal("anonymize")
	}
}

func TestRecover(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := run(t, req, func(echo.Context) error { panic("boom") }, Recover())
	if code(err) != apperror.CodeInternal {
		t.Fatalf("panic must map to internal error: %v", err)
	}
}

func TestSecurityHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	rec, _ := run(t, req, okHandler, SecurityHeaders(true))
	for _, h := range []string{"X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy", "Strict-Transport-Security", "Referrer-Policy"} {
		if rec.Header().Get(h) == "" {
			t.Errorf("missing %s", h)
		}
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("API responses must not be cached")
	}
}

func TestBodyLimit(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("a", 100)))
	_, err := run(t, req, okHandler, BodyLimit(10, nil))
	if code(err) != apperror.CodePayloadTooLarge {
		t.Fatalf("content-length over limit: %v", err)
	}
}

func TestTimeoutSetsDeadline(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := run(t, req, func(c echo.Context) error {
		if _, ok := c.Request().Context().Deadline(); !ok {
			return errors.New("no deadline")
		}
		<-c.Request().Context().Done()
		return nil
	}, Timeout(20*time.Millisecond, nil))
	if code(err) != apperror.CodeTimeout {
		t.Fatalf("timeout: %v", err)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	l := ratelimit.NewMemoryLimiter(nil)
	rule := ratelimit.Rule{Name: "t", Rate: 1, Period: time.Minute, Burst: 1}
	mw := RateLimit(l, rule, ByIP, false)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, err := run(t, req, okHandler, mw); err != nil {
		t.Fatal(err)
	}
	_, err := run(t, httptest.NewRequest(http.MethodGet, "/", nil), okHandler, mw)
	if e, ok := apperror.As(err); !ok || e.Code != apperror.CodeRateLimited || e.RetryAfter <= 0 {
		t.Fatalf("second request: %v", err)
	}
}

type failingLimiter struct{}

func (failingLimiter) Allow(context.Context, string, ratelimit.Rule) (ratelimit.Result, error) {
	return ratelimit.Result{}, errors.New("redis down")
}

func TestRateLimitFailModes(t *testing.T) {
	rule := ratelimit.Rule{Name: "t", Rate: 1, Period: time.Minute}
	if _, err := run(t, httptest.NewRequest(http.MethodGet, "/", nil), okHandler, RateLimit(failingLimiter{}, rule, ByIP, true)); err != nil {
		t.Fatalf("fail-open must allow: %v", err)
	}
	_, err := run(t, httptest.NewRequest(http.MethodGet, "/", nil), okHandler, RateLimit(failingLimiter{}, rule, ByIP, false))
	if code(err) != apperror.CodeUnavailable {
		t.Fatalf("fail-closed must reject: %v", err)
	}
}

func TestCSRFMiddleware(t *testing.T) {
	csrf, _ := security.NewCSRF(strings.Repeat("c", 32))
	names := CookieNames{Access: "sy_at", Refresh: "sy_rt", CSRF: "sy_csrf"}
	mw := CSRF(csrf, names, map[string]bool{"https://app.test": true}, nil)
	token := csrf.Issue()
	build := func(method, origin, header, cookie string) *http.Request {
		r := httptest.NewRequest(method, "/", nil)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if header != "" {
			r.Header.Set(CSRFHeader, header)
		}
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: "sy_csrf", Value: cookie})
		}
		return r
	}
	cases := []struct {
		name string
		req  *http.Request
		ok   bool
	}{
		{"safe method", build(http.MethodGet, "", "", ""), true},
		{"valid", build(http.MethodPost, "https://app.test", token, token), true},
		{"missing header", build(http.MethodPost, "https://app.test", "", token), false},
		{"mismatch", build(http.MethodPost, "https://app.test", token, csrf.Issue()), false},
		{"foreign origin", build(http.MethodPost, "https://evil.test", token, token), false},
	}
	bearer := build(http.MethodPost, "", "", "")
	bearer.Header.Set("Authorization", "Bearer abc")
	cases = append(cases, struct {
		name string
		req  *http.Request
		ok   bool
	}{"bearer without cookies", bearer, true})
	for _, tc := range cases {
		_, err := run(t, tc.req, okHandler, mw)
		if (err == nil) != tc.ok {
			t.Errorf("%s: err=%v", tc.name, err)
		}
	}
}

type fakeAuth struct{ p reqctx.Principal }

func (f fakeAuth) Authenticate(_ context.Context, token string) (reqctx.Principal, error) {
	if token == "good" {
		return f.p, nil
	}
	return reqctx.Principal{}, apperror.SessionExpired()
}

func TestAuthenticateAndRBAC(t *testing.T) {
	names := CookieNames{Access: "sy_at"}
	auth := fakeAuth{p: reqctx.Principal{UserID: uuid.New(), Role: domain.RolePsychic}}
	withCookie := func(v string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.AddCookie(&http.Cookie{Name: "sy_at", Value: v})
		return r
	}
	if _, err := run(t, httptest.NewRequest(http.MethodGet, "/", nil), okHandler, Authenticate(auth, names, true)); code(err) != apperror.CodeUnauthenticated {
		t.Fatalf("missing token: %v", err)
	}
	if _, err := run(t, withCookie("bad"), okHandler, Authenticate(auth, names, true)); code(err) != apperror.CodeSessionExpired {
		t.Fatalf("bad token: %v", err)
	}
	if _, err := run(t, withCookie("bad"), okHandler, Authenticate(auth, names, false)); err != nil {
		t.Fatalf("optional auth must pass anonymous: %v", err)
	}
	if _, err := run(t, withCookie("good"), okHandler, Authenticate(auth, names, true), RequireRole(domain.RolePsychic)); err != nil {
		t.Fatalf("authorized: %v", err)
	}
	if _, err := run(t, withCookie("good"), okHandler, Authenticate(auth, names, true), RequireRole(domain.RoleAdmin)); code(err) != apperror.CodeForbidden {
		t.Fatalf("rbac: %v", err)
	}
	bearer := httptest.NewRequest(http.MethodGet, "/", nil)
	bearer.Header.Set("Authorization", "Bearer good")
	if _, err := run(t, bearer, okHandler, Authenticate(auth, names, true)); err != nil {
		t.Fatalf("bearer: %v", err)
	}
}
