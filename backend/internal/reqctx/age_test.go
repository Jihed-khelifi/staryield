package reqctx

import (
	"net/http"
	"testing"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// revisions 5.0: 18+ age gate (§24.6).
func TestRequireAdult(t *testing.T) {
	for _, tc := range []struct {
		name string
		p    Principal
		code apperror.Code // "" = allowed
	}{
		{"adult client", Principal{Role: domain.RoleClient, Age: domain.AgeAdult}, ""},
		{"client without a date", Principal{Role: domain.RoleClient, Age: domain.AgeBirthDateRequired}, apperror.CodeBirthDateRequired},
		{"restricted client", Principal{Role: domain.RoleClient, Age: domain.AgeRestricted}, apperror.CodeAgeRestricted},
		{"client with no status loaded fails closed", Principal{Role: domain.RoleClient}, apperror.CodeBirthDateRequired},
		{"advisor", Principal{Role: domain.RolePsychic}, ""},
		{"admin", Principal{Role: domain.RoleAdmin}, ""},
	} {
		err := tc.p.RequireAdult()
		if tc.code == "" {
			if err != nil {
				t.Errorf("%s: %v", tc.name, err)
			}
			continue
		}
		e, ok := apperror.As(err)
		if !ok || e.Code != tc.code || e.Status != http.StatusForbidden {
			t.Errorf("%s: %v", tc.name, err)
		}
		if tc.code == apperror.CodeBirthDateRequired && e.Details["next"] != "/signup/birth" {
			t.Errorf("%s: details %v", tc.name, e.Details)
		}
	}
}
