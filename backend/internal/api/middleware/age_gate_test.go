package middleware

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// revisions 5.0: 18+ age gate (§24.6).
func TestAgeGate(t *testing.T) {
	mw := AgeGate()
	for _, tc := range []struct {
		name string
		p    reqctx.Principal
		want apperror.Code // "" = passes
	}{
		{"adult client", reqctx.Principal{Role: domain.RoleClient, Age: domain.AgeAdult}, ""},
		{"no date", reqctx.Principal{Role: domain.RoleClient, Age: domain.AgeBirthDateRequired}, apperror.CodeBirthDateRequired},
		{"restricted", reqctx.Principal{Role: domain.RoleClient, Age: domain.AgeRestricted}, apperror.CodeAgeRestricted},
		{"status not loaded", reqctx.Principal{Role: domain.RoleClient}, apperror.CodeBirthDateRequired},
		{"advisor", reqctx.Principal{Role: domain.RolePsychic}, ""},
		{"admin", reqctx.Principal{Role: domain.RoleAdmin}, ""},
	} {
		tc.p.UserID = uuid.New()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/sessions", nil)
		req = req.WithContext(reqctx.WithPrincipal(req.Context(), tc.p))
		_, err := run(t, req, okHandler, mw)
		if tc.want == "" {
			if err != nil {
				t.Errorf("%s: %v", tc.name, err)
			}
			continue
		}
		e, ok := apperror.As(err)
		if !ok || e.Code != tc.want || e.Status != http.StatusForbidden {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
	// Without a principal the gate is not the place to answer: 401.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, err := run(t, req, okHandler, mw); code(err) != apperror.CodeUnauthenticated {
		t.Errorf("anonymous: %v", err)
	}
}

// The gate reads a field of the principal the authenticator already loaded:
// no query, no allocation on the allowed path.
func BenchmarkAgeGateAdult(b *testing.B) {
	h := AgeGate()(func(c echo.Context) error { return nil })
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(reqctx.WithPrincipal(req.Context(), reqctx.Principal{UserID: uuid.New(), Role: domain.RoleClient, Age: domain.AgeAdult}))
	c := e.NewContext(req, httptest.NewRecorder())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := h(c); err != nil {
			b.Fatal(err)
		}
	}
}
