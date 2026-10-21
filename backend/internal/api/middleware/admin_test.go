package middleware

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

func TestAdminOriginOnlyAllowsTheConsole(t *testing.T) {
	mw := AdminOrigin(map[string]bool{"https://admin.staryield.net": true})
	for origin, ok := range map[string]bool{
		"https://admin.staryield.net": true,
		"HTTPS://ADMIN.STARYIELD.NET": true,
		"https://staryield.net":       false,
		"https://evil.example":        false,
		"null":                        false,
		"":                            false,
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		_, err := run(t, req, okHandler, mw)
		if (err == nil) != ok {
			t.Errorf("origin %q: err=%v", origin, err)
		}
		if err != nil && code(err) != apperror.CodeCSRF {
			t.Errorf("origin %q: code %s", origin, code(err))
		}
	}
	// CORS preflight passes through to the CORS middleware.
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/admin/users", nil)
	if _, err := run(t, req, okHandler, mw); err != nil {
		t.Fatalf("preflight: %v", err)
	}
}

func TestIPAllowlist(t *testing.T) {
	_, office, _ := net.ParseCIDR("203.0.113.0/24")
	mw := IPAllowlist([]*net.IPNet{office})
	for ip, ok := range map[string]bool{"203.0.113.7": true, "198.51.100.1": false} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":1234"
		_, err := run(t, req, okHandler, mw)
		if (err == nil) != ok {
			t.Errorf("%s: %v", ip, err)
		}
	}
	// No allow-list: everyone passes.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, err := run(t, req, okHandler, IPAllowlist(nil)); err != nil {
		t.Fatal(err)
	}
}

type fakeAdminAuth struct {
	err  error
	user uuid.UUID
	got  string
}

func (f *fakeAdminAuth) AuthenticateAdmin(ctx context.Context, token string) (context.Context, error) {
	f.got = token
	if f.err != nil {
		return nil, f.err
	}
	return reqctx.WithPrincipal(ctx, reqctx.Principal{UserID: f.user, Role: domain.RoleAdmin}), nil
}

func TestAdminSessionAndStepUp(t *testing.T) {
	auth := &fakeAdminAuth{user: uuid.New()}
	mw := AdminSession(auth, "sy_adm")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, err := run(t, req, okHandler, mw); code(err) != apperror.CodeUnauthenticated {
		t.Fatalf("no cookie: %v", err)
	}
	// The regular access cookie is not an admin session.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "sy_at", Value: "web-token"})
	if _, err := run(t, req, okHandler, mw); code(err) != apperror.CodeUnauthenticated {
		t.Fatalf("web cookie: %v", err)
	}
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "sy_adm", Value: "admin-token"})
	var seen reqctx.Principal
	_, err := run(t, req, func(c echo.Context) error {
		seen, _ = reqctx.PrincipalFrom(c.Request().Context())
		return nil
	}, mw)
	if err != nil || seen.UserID != auth.user || seen.Role != domain.RoleAdmin || auth.got != "admin-token" {
		t.Fatalf("session: %v %+v", err, seen)
	}
	auth.err = apperror.SessionExpired()
	if _, err := run(t, req, okHandler, mw); code(err) != apperror.CodeSessionExpired {
		t.Fatalf("expired: %v", err)
	}

	denied := StepUp(func(context.Context) error { return apperror.New("step_up_required", http.StatusForbidden, "x") })
	if _, err := run(t, httptest.NewRequest(http.MethodPost, "/", nil), okHandler, denied); code(err) != "step_up_required" {
		t.Fatalf("step-up: %v", err)
	}
	allowed := StepUp(func(context.Context) error { return nil })
	if _, err := run(t, httptest.NewRequest(http.MethodPost, "/", nil), okHandler, allowed); err != nil {
		t.Fatal(err)
	}
}

func TestCSRFBearerBypassDoesNotCoverAdminCookie(t *testing.T) {
	csrf, err := security.NewCSRF("0123456789abcdef0123456789abcdef-csrf")
	if err != nil {
		t.Fatal(err)
	}
	names := CookieNames{Access: "sy_at", Refresh: "sy_rt", CSRF: "sy_csrf", Admin: "sy_adm"}
	mw := CSRF(csrf, names, map[string]bool{"https://admin.test": true}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", nil)
	req.Header.Set("Authorization", "Bearer forged")
	req.AddCookie(&http.Cookie{Name: "sy_adm", Value: "admin-session"})
	if _, err := run(t, req, okHandler, mw); code(err) != apperror.CodeCSRF {
		t.Fatalf("bearer header must not bypass CSRF when an admin cookie is sent: %v", err)
	}
	// A pure token client (no cookies at all) is not exposed to CSRF.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", nil)
	req.Header.Set("Authorization", "Bearer token")
	if _, err := run(t, req, okHandler, mw); err != nil {
		t.Fatalf("token client: %v", err)
	}
}
